package pinecone

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/grpc"
)

// [RetryPolicy] configures exponential-backoff retries for rate-limited (HTTP 429 /
// gRPC RESOURCE_EXHAUSTED) and transient (5xx / gRPC UNAVAILABLE) responses. Other
// 4xx errors are never retried. Pass it via [NewClientParams.RetryPolicy] to enable
// retries on both the REST (control/data/inference) and gRPC (data plane) clients.
//
// For REST, 429 is always retried (the request was rejected, not processed), while
// 5xx and transport errors are retried only for idempotent HTTP methods to avoid
// duplicating non-idempotent operations.
//
// Fields:
//   - MaxRetries: Number of retries after the initial attempt. 0 disables retries.
//   - BaseDelay: Initial backoff before the first retry. Required when MaxRetries > 0.
//   - MaxDelay: Upper bound on any single backoff. Required when MaxRetries > 0.
//   - BackoffMultiplier: Growth factor applied to the delay each attempt (e.g. 2.0).
type RetryPolicy struct {
	MaxRetries        int
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
}

// [DefaultRetryPolicy] returns a sensible default: 3 retries, 500ms base delay,
// 30s cap, doubling each attempt.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        3,
		BaseDelay:         500 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2,
	}
}

func (p *RetryPolicy) validate() error {
	if p == nil {
		return nil
	}
	if p.MaxRetries < 0 {
		return fmt.Errorf("RetryPolicy.MaxRetries must be >= 0, got %d", p.MaxRetries)
	}
	if p.MaxRetries == 0 {
		return nil
	}
	if p.BaseDelay <= 0 {
		return fmt.Errorf("RetryPolicy.BaseDelay must be > 0 when MaxRetries > 0")
	}
	if p.MaxDelay <= 0 {
		return fmt.Errorf("RetryPolicy.MaxDelay must be > 0 when MaxRetries > 0")
	}
	if p.BaseDelay > p.MaxDelay {
		return fmt.Errorf("RetryPolicy.BaseDelay (%s) must be <= MaxDelay (%s)", p.BaseDelay, p.MaxDelay)
	}
	if p.BackoffMultiplier < 1 {
		return fmt.Errorf("RetryPolicy.BackoffMultiplier must be >= 1, got %v", p.BackoffMultiplier)
	}
	return nil
}

// [NewRetryHTTPClient] returns an *http.Client that retries per policy. If base is
// provided its settings are preserved and its transport is wrapped; otherwise a new
// client is created. A nil policy uses [DefaultRetryPolicy].
func NewRetryHTTPClient(policy *RetryPolicy, base *http.Client) *http.Client {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	client := &http.Client{}
	if base != nil {
		*client = *base
	}
	client.Transport = &retryTransport{policy: policy, base: client.Transport}
	return client
}

// retryTransport wraps an http.RoundTripper, retrying rate-limit and transient responses.
type retryTransport struct {
	policy *RetryPolicy
	base   http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	// Buffer the body once so it can be replayed on each attempt.
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	var resp *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		attemptReq := req.Clone(req.Context())
		if body != nil {
			attemptReq.Body = io.NopCloser(bytes.NewReader(body))
			attemptReq.ContentLength = int64(len(body))
		}

		resp, err = base.RoundTrip(attemptReq)

		// Stop on cancellation regardless of the error.
		if req.Context().Err() != nil {
			return resp, err
		}
		if !t.shouldRetry(attempt, req.Method, resp, err) {
			return resp, err
		}

		retryAfter := retryAfterDelay(resp)
		if retryAfter > t.policy.MaxDelay {
			return resp, err // honor the server's hint over our budget: stop retrying
		}
		drainResponse(resp)
		if !wait(req.Context(), t.backoff(attempt, retryAfter)) {
			return nil, req.Context().Err()
		}
	}
}

func (t *retryTransport) shouldRetry(attempt int, method string, resp *http.Response, err error) bool {
	if attempt >= t.policy.MaxRetries {
		return false
	}
	if err != nil {
		return isIdempotent(method) // transport error: only safe to replay idempotent requests
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true // rate-limited: request was rejected, safe to retry
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return isIdempotent(method)
	}
	return false
}

// isIdempotent reports whether an HTTP method is safe to retry after the server may
// have processed the request.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete, http.MethodTrace:
		return true
	}
	return false
}

// backoff returns the wait before the next attempt: the Retry-After hint if present
// (already bounded by MaxDelay by the caller), else exponential growth with full
// jitter, capped at MaxDelay.
func (t *retryTransport) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	d := float64(t.policy.BaseDelay) * math.Pow(t.policy.BackoffMultiplier, float64(attempt))
	if d > float64(t.policy.MaxDelay) {
		d = float64(t.policy.MaxDelay)
	}
	if d <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(d) + 1))
}

// retryAfterDelay parses a Retry-After header in either delta-seconds or HTTP-date
// form; 0 if absent, unparseable, or in the past.
func retryAfterDelay(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

func drainResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// wait sleeps for d or until ctx is cancelled; returns false if cancelled.
func wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// [RetryDialOption] returns a gRPC dial option enabling built-in retries for the data
// plane per policy, keyed on RESOURCE_EXHAUSTED and UNAVAILABLE. A nil policy uses
// [DefaultRetryPolicy]. Returns a no-op option when the policy disables retries.
// Note: gRPC caps built-in retries at 5 attempts by default.
func RetryDialOption(policy *RetryPolicy) grpc.DialOption {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	cfg := buildRetryServiceConfig(policy)
	if cfg == "" {
		return grpc.EmptyDialOption{}
	}
	return grpc.WithDefaultServiceConfig(cfg)
}

// buildRetryServiceConfig renders a gRPC service config for the VectorService, or ""
// if the policy allows fewer than 2 attempts (retries disabled).
func buildRetryServiceConfig(policy *RetryPolicy) string {
	attempts := policy.MaxRetries + 1
	if attempts < 2 {
		return ""
	}
	return fmt.Sprintf(`{
  "methodConfig": [{
    "name": [{"service": "VectorService"}],
    "retryPolicy": {
      "MaxAttempts": %d,
      "InitialBackoff": "%s",
      "MaxBackoff": "%s",
      "BackoffMultiplier": %v,
      "RetryableStatusCodes": ["RESOURCE_EXHAUSTED", "UNAVAILABLE"]
    }
  }]
}`, attempts, grpcDuration(policy.BaseDelay), grpcDuration(policy.MaxDelay), policy.BackoffMultiplier)
}

// grpcDuration formats a duration as protobuf JSON seconds (e.g. "0.5s", "30s").
func grpcDuration(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}
