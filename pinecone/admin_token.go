package pinecone

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pinecone-io/go-pinecone/v6/internal/gen/admin"
)

// tokenRefreshMargin keeps an in-flight request from outliving the token it was sent with.
const tokenRefreshMargin = 5 * time.Minute

type mintTokenFunc func(ctx context.Context) (*authTokenResponse, error)

type refreshingTokenSource struct {
	mu       sync.Mutex
	mint     mintTokenFunc
	now      func() time.Time
	token    string
	deadline time.Time // zero when the token endpoint reported no usable expires_in
}

func newRefreshingTokenSource(initial *authTokenResponse, mint mintTokenFunc) *refreshingTokenSource {
	ts := &refreshingTokenSource{mint: mint, now: time.Now}
	ts.set(initial)
	return ts
}

// refreshDeadline caps the margin at half the lifetime so a short-lived token isn't stale on arrival.
func refreshDeadline(now time.Time, expiresIn int) time.Time {
	if expiresIn <= 0 {
		return time.Time{}
	}
	lifetime := time.Duration(expiresIn) * time.Second
	return now.Add(lifetime - min(tokenRefreshMargin, lifetime/2))
}

func (ts *refreshingTokenSource) set(res *authTokenResponse) {
	ts.token = res.AccessToken
	ts.deadline = refreshDeadline(ts.now(), res.ExpiresIn)
}

func (ts *refreshingTokenSource) mintLocked(ctx context.Context) (string, error) {
	res, err := ts.mint(ctx)
	if err != nil {
		return "", err
	}
	ts.set(res)
	return ts.token, nil
}

func (ts *refreshingTokenSource) current(ctx context.Context) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if !ts.deadline.IsZero() && !ts.now().Before(ts.deadline) {
		return ts.mintLocked(ctx)
	}
	return ts.token, nil
}

// replace mints only if stale is still current, so concurrent 401s cost one token exchange.
func (ts *refreshingTokenSource) replace(ctx context.Context, stale string) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.token != stale {
		return ts.token, nil
	}
	return ts.mintLocked(ctx)
}

func (ts *refreshingTokenSource) Intercept(ctx context.Context, req *http.Request) error {
	token, err := ts.current(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

type tokenRefreshingDoer struct {
	base   admin.HttpRequestDoer
	tokens *refreshingTokenSource
}

func (d *tokenRefreshingDoer) Do(req *http.Request) (*http.Response, error) {
	res, err := d.base.Do(req)
	if err != nil || res.StatusCode != http.StatusUnauthorized {
		return res, err
	}
	if req.Body != nil && req.GetBody == nil {
		return res, nil
	}

	stale := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	token, err := d.tokens.replace(req.Context(), stale)
	if err != nil || token == stale {
		return res, nil
	}

	retry := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return res, nil
		}
		retry.Body = body
	}
	retry.Header.Set("Authorization", "Bearer "+token)

	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return d.base.Do(retry)
}
