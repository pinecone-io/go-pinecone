package pinecone

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func fastPolicy(maxRetries int) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        maxRetries,
		BaseDelay:         time.Millisecond,
		MaxDelay:          5 * time.Millisecond,
		BackoffMultiplier: 2,
	}
}

func TestRetryPolicyValidateUnit(t *testing.T) {
	tests := []struct {
		name    string
		policy  *RetryPolicy
		wantErr bool
	}{
		{"nil is valid", nil, false},
		{"default is valid", DefaultRetryPolicy(), false},
		{"zero retries valid", &RetryPolicy{}, false},
		{"negative retries", &RetryPolicy{MaxRetries: -1}, true},
		{"negative delay", &RetryPolicy{MaxRetries: 1, BaseDelay: -1, BackoffMultiplier: 2}, true},
		{"zero base delay with retries", &RetryPolicy{MaxRetries: 1, MaxDelay: 5, BackoffMultiplier: 2}, true},
		{"zero max delay with retries", &RetryPolicy{MaxRetries: 1, BaseDelay: 1, BackoffMultiplier: 2}, true},
		{"base > max", &RetryPolicy{MaxRetries: 1, BaseDelay: 10, MaxDelay: 5, BackoffMultiplier: 2}, true},
		{"multiplier < 1", &RetryPolicy{MaxRetries: 1, BaseDelay: 1, MaxDelay: 5, BackoffMultiplier: 0.5}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// serverReturning responds with `status` for the first `failures` calls, then 200.
// It records the number of calls and each request body.
func serverReturning(status, failures int) (*httptest.Server, *int32, *[]string) {
	var calls int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if int(n) <= failures {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	return srv, &calls, &bodies
}

func TestRetryTransportRetriesOnRateLimitUnit(t *testing.T) {
	srv, calls, _ := serverReturning(http.StatusTooManyRequests, 2)
	defer srv.Close()

	client := NewRetryHTTPClient(fastPolicy(3), nil)
	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(calls), "expected 2 retries then success")
}

func TestRetryTransportTransient5xxUnit(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		srv, calls, _ := serverReturning(status, 1)
		client := NewRetryHTTPClient(fastPolicy(3), nil)
		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(2), atomic.LoadInt32(calls), "status %d should retry once", status)
		srv.Close()
	}
}

func TestRetryTransportNoRetryOn4xxUnit(t *testing.T) {
	srv, calls, _ := serverReturning(http.StatusBadRequest, 5)
	defer srv.Close()

	client := NewRetryHTTPClient(fastPolicy(3), nil)
	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(calls), "4xx (non-429) must not retry")
}

func TestRetryTransportExhaustsRetriesUnit(t *testing.T) {
	srv, calls, _ := serverReturning(http.StatusTooManyRequests, 100)
	defer srv.Close()

	client := NewRetryHTTPClient(fastPolicy(2), nil)
	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "returns last response after exhausting retries")
	assert.Equal(t, int32(3), atomic.LoadInt32(calls), "1 initial + 2 retries")
}

func TestRetryTransportReplaysBodyUnit(t *testing.T) {
	srv, calls, bodies := serverReturning(http.StatusTooManyRequests, 2)
	defer srv.Close()

	client := NewRetryHTTPClient(fastPolicy(3), nil)
	resp, err := client.Post(srv.URL, "text/plain", strings.NewReader("payload"))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, int32(3), atomic.LoadInt32(calls))
	for i, b := range *bodies {
		assert.Equal(t, "payload", b, "attempt %d should replay the full body", i)
	}
}

func TestRetryTransportContextCancelUnit(t *testing.T) {
	srv, _, _ := serverReturning(http.StatusTooManyRequests, 100)
	defer srv.Close()

	// Large delay so the wait dominates; cancel should short-circuit it.
	policy := &RetryPolicy{MaxRetries: 5, BaseDelay: 10 * time.Second, MaxDelay: 10 * time.Second, BackoffMultiplier: 2}
	client := NewRetryHTTPClient(policy, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	start := time.Now()
	_, err := client.Do(req)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, time.Second, "cancellation should abort the backoff wait promptly")
}

func TestRetryTransportRetryAfterHeaderUnit(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "2")
	assert.Equal(t, 2*time.Second, retryAfterDelay(resp))

	resp.Header.Set("Retry-After", "not-a-number")
	assert.Equal(t, time.Duration(0), retryAfterDelay(resp))

	assert.Equal(t, time.Duration(0), retryAfterDelay(&http.Response{Header: http.Header{}}))
	assert.Equal(t, time.Duration(0), retryAfterDelay(nil))
}

func TestRetryBackoffBoundsUnit(t *testing.T) {
	tr := &retryTransport{policy: &RetryPolicy{MaxRetries: 10, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second, BackoffMultiplier: 2}}

	// Retry-After hint (already bounded by the caller) is honored as-is.
	assert.Equal(t, 500*time.Millisecond, tr.backoff(0, 500*time.Millisecond))

	// Full jitter: 0 <= delay <= min(cap, base*mult^attempt), across many draws.
	for attempt := 0; attempt < 8; attempt++ {
		ceiling := 100 * time.Millisecond << attempt // base * 2^attempt
		if ceiling > time.Second {
			ceiling = time.Second
		}
		for i := 0; i < 200; i++ {
			d := tr.backoff(attempt, 0)
			assert.GreaterOrEqual(t, d, time.Duration(0))
			assert.LessOrEqual(t, d, ceiling, "attempt %d exceeded ceiling", attempt)
		}
	}
}

func TestNewRetryHTTPClientPreservesBaseUnit(t *testing.T) {
	base := &http.Client{Timeout: 42 * time.Second}
	client := NewRetryHTTPClient(fastPolicy(1), base)

	assert.Equal(t, 42*time.Second, client.Timeout, "base client settings should be preserved")
	assert.NotSame(t, base, client, "should not mutate the provided client")
	_, ok := client.Transport.(*retryTransport)
	assert.True(t, ok, "transport should be wrapped")
}

func TestBuildRetryServiceConfigUnit(t *testing.T) {
	// Disabled when fewer than 2 attempts.
	assert.Empty(t, buildRetryServiceConfig(&RetryPolicy{MaxRetries: 0}))

	cfg := buildRetryServiceConfig(&RetryPolicy{MaxRetries: 3, BaseDelay: 500 * time.Millisecond, MaxDelay: 30 * time.Second, BackoffMultiplier: 2})
	require.True(t, json.Valid([]byte(cfg)), "service config must be valid JSON")
	assert.Contains(t, cfg, `"MaxAttempts": 4`)
	assert.Contains(t, cfg, `"InitialBackoff": "0.5s"`)
	assert.Contains(t, cfg, `"MaxBackoff": "30s"`)
	assert.Contains(t, cfg, "RESOURCE_EXHAUSTED")
	assert.Contains(t, cfg, "UNAVAILABLE")
	assert.Contains(t, cfg, `"service": "VectorService"`)
}

func TestRetryDialOptionsUnit(t *testing.T) {
	// Disabled policy → no options.
	assert.Nil(t, RetryDialOptions(&RetryPolicy{MaxRetries: 0}))

	// Enabled policy: service config + raised call-attempt limit.
	assert.Len(t, RetryDialOptions(fastPolicy(3)), 2)
	assert.Len(t, RetryDialOptions(nil), 2) // nil → default policy
}

func TestGrpcDurationUnit(t *testing.T) {
	assert.Equal(t, "0.5s", grpcDuration(500*time.Millisecond))
	assert.Equal(t, "30s", grpcDuration(30*time.Second))
	assert.Equal(t, "0s", grpcDuration(0))
}

func TestRetryTransportNoRetryPostOn5xxUnit(t *testing.T) {
	// 5xx on a non-idempotent method must not retry (could duplicate a mutation).
	srv, calls, _ := serverReturning(http.StatusServiceUnavailable, 5)
	defer srv.Close()

	client := NewRetryHTTPClient(fastPolicy(3), nil)
	resp, err := client.Post(srv.URL, "text/plain", strings.NewReader("x"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(calls), "POST 5xx must not be retried")
}

// countingRoundTripper fails with an error for the first `failures` calls, then succeeds.
type countingRoundTripper struct {
	failures int
	calls    int32
}

func (rt *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	if int(atomic.AddInt32(&rt.calls, 1)) <= rt.failures {
		return nil, assertErr
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

var assertErr = &url.Error{Op: "Get", URL: "x", Err: io.ErrUnexpectedEOF}

func TestRetryTransportRetriesTransportErrorUnit(t *testing.T) {
	// Idempotent GET: transport errors are retried.
	rt := &countingRoundTripper{failures: 2}
	client := NewRetryHTTPClient(fastPolicy(3), &http.Client{Transport: rt})
	resp, err := client.Get("http://example.invalid")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, int32(3), atomic.LoadInt32(&rt.calls))
}

func TestRetryTransportNoRetryPostTransportErrorUnit(t *testing.T) {
	// Non-idempotent POST: transport errors are not retried.
	rt := &countingRoundTripper{failures: 5}
	client := NewRetryHTTPClient(fastPolicy(3), &http.Client{Transport: rt})
	_, err := client.Post("http://example.invalid", "text/plain", strings.NewReader("x"))
	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&rt.calls), "POST transport error must not be retried")
}

func TestRetryTransportRetryAfterExceedsMaxDelayUnit(t *testing.T) {
	// Server asks to wait longer than we're willing → stop, return the response.
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := NewRetryHTTPClient(fastPolicy(3), nil) // MaxDelay 5ms
	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "Retry-After beyond MaxDelay should stop retries")
}

func TestRetryAfterHTTPDateUnit(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", time.Now().Add(2*time.Second).UTC().Format(http.TimeFormat))
	d := retryAfterDelay(resp)
	assert.Greater(t, d, time.Duration(0))
	assert.LessOrEqual(t, d, 2*time.Second)

	// A past date yields no delay.
	resp.Header.Set("Retry-After", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
	assert.Equal(t, time.Duration(0), retryAfterDelay(resp))
}

func TestNewRetryHTTPClientInvokesBaseTransportUnit(t *testing.T) {
	rt := &countingRoundTripper{failures: 0}
	client := NewRetryHTTPClient(nil, &http.Client{Transport: rt}) // nil policy → default
	resp, err := client.Get("http://example.invalid")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, int32(1), atomic.LoadInt32(&rt.calls), "wrapped base transport must be invoked")
}

func TestRetryServiceConfigAcceptedByGrpcUnit(t *testing.T) {
	// The rendered config must be accepted by grpc-go, not merely valid JSON.
	// A high MaxRetries (> gRPC's default 5-attempt cap) must still be accepted,
	// since RetryDialOptions raises the per-call attempt limit to match.
	for _, policy := range []*RetryPolicy{DefaultRetryPolicy(), fastPolicy(9)} {
		opts := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
			RetryDialOptions(policy)...)
		conn, err := grpc.NewClient("passthrough:///localhost:0", opts...)
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	}
}

func TestNewClientRetryPolicyWiringUnit(t *testing.T) {
	pc, err := NewClient(NewClientParams{ApiKey: "test-key", RetryPolicy: DefaultRetryPolicy()})
	require.NoError(t, err)
	_, ok := pc.baseParams.RestClient.Transport.(*retryTransport)
	assert.True(t, ok, "REST client transport should be wrapped with retries")
}

func TestNewClientInvalidRetryPolicyUnit(t *testing.T) {
	_, err := NewClient(NewClientParams{ApiKey: "test-key", RetryPolicy: &RetryPolicy{MaxRetries: -1}})
	require.Error(t, err)
}
