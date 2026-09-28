package pinecone

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pinecone-io/go-pinecone/v7/internal/gen/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshDeadlineUnit(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	assert.Equal(t, now.Add(1500*time.Second), refreshDeadline(now, 1800))
	assert.Equal(t, now.Add(150*time.Second), refreshDeadline(now, 300), "short-lived tokens use half their lifetime as the margin")
	assert.True(t, refreshDeadline(now, 0).IsZero())
	assert.True(t, refreshDeadline(now, -1).IsZero())
}

func TestRefreshingTokenSourceRefreshesAheadOfExpiryUnit(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	ts, mints := newTestTokenSource(t, &now, 1800)

	token, err := ts.current(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token-0", token)

	now = now.Add(1499 * time.Second)
	token, _ = ts.current(context.Background())
	assert.Equal(t, "token-0", token)
	assert.Equal(t, int32(0), atomic.LoadInt32(mints))

	now = now.Add(time.Second)
	token, _ = ts.current(context.Background())
	assert.Equal(t, "token-1", token)
	assert.Equal(t, int32(1), atomic.LoadInt32(mints))
}

func TestRefreshingTokenSourceWithoutExpiryNeverRefreshesUnit(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	ts, mints := newTestTokenSource(t, &now, 0)

	now = now.Add(24 * time.Hour)
	token, _ := ts.current(context.Background())
	assert.Equal(t, "token-0", token)
	assert.Equal(t, int32(0), atomic.LoadInt32(mints))
}

func TestRefreshingTokenSourceConcurrentRefreshMintsOnceUnit(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	ts, mints := newTestTokenSource(t, &now, 1800)
	now = now.Add(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = ts.current(context.Background())
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), atomic.LoadInt32(mints))

	// A 401 on a token that was already replaced reuses the replacement.
	token, _ := ts.replace(context.Background(), "token-0")
	assert.Equal(t, "token-1", token)
	assert.Equal(t, int32(1), atomic.LoadInt32(mints))
}

func TestTokenRefreshingDoerReplaysBodyOn401Unit(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, r.Header.Get("Authorization")+" "+string(body))
		if r.Header.Get("Authorization") != "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	now := time.Unix(1_000_000, 0)
	ts, mints := newTestTokenSource(t, &now, 1800)
	doer := &tokenRefreshingDoer{base: http.DefaultClient, tokens: ts}

	req, err := http.NewRequest(http.MethodPatch, server.URL, bytes.NewReader([]byte(`{"name":"x"}`)))
	require.NoError(t, err)
	require.NoError(t, ts.Intercept(context.Background(), req))

	res, err := doer.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(mints))
	assert.Equal(t, []string{`Bearer token-0 {"name":"x"}`, `Bearer token-1 {"name":"x"}`}, bodies)
}

func TestAdminClientRetriesOnceWithNewTokenAfter401Unit(t *testing.T) {
	t.Setenv("PINECONE_ACCESS_TOKEN", "")
	mints := mockAuthTokens(t)
	server, calls := newAdminTestServer(t, "token-2")

	client, err := NewAdminClientWithContext(context.Background(), NewAdminClientParams{
		ClientId: "id", ClientSecret: "secret", Host: server.URL,
	})
	require.NoError(t, err)

	require.NoError(t, client.Organization.Delete(context.Background(), "org-id"))
	assert.Equal(t, int32(2), atomic.LoadInt32(mints), "initial token plus one replacement")
	assert.Equal(t, int32(2), atomic.LoadInt32(calls))
}

func TestAdminClientPersistent401RetriesOnlyOnceUnit(t *testing.T) {
	t.Setenv("PINECONE_ACCESS_TOKEN", "")
	mints := mockAuthTokens(t)
	server, calls := newAdminTestServer(t, "never")

	client, err := NewAdminClientWithContext(context.Background(), NewAdminClientParams{
		ClientId: "id", ClientSecret: "secret", Host: server.URL,
	})
	require.NoError(t, err)

	require.Error(t, client.Organization.Delete(context.Background(), "org-id"))
	assert.Equal(t, int32(2), atomic.LoadInt32(mints))
	assert.Equal(t, int32(2), atomic.LoadInt32(calls))
}

func TestAdminClientAccessTokenIsNotRefreshedUnit(t *testing.T) {
	mints := mockAuthTokens(t)
	server, calls := newAdminTestServer(t, "never")

	client, err := NewAdminClientWithContext(context.Background(), NewAdminClientParams{
		AccessToken: "caller-token", Host: server.URL,
	})
	require.NoError(t, err)

	require.Error(t, client.Organization.Delete(context.Background(), "org-id"))
	assert.Equal(t, int32(0), atomic.LoadInt32(mints))
	assert.Equal(t, int32(1), atomic.LoadInt32(calls))
}

func newTestTokenSource(t *testing.T, now *time.Time, expiresIn int) (*refreshingTokenSource, *int32) {
	t.Helper()
	var mints int32
	mint := func(ctx context.Context) (*authTokenResponse, error) {
		n := atomic.AddInt32(&mints, 1)
		return &authTokenResponse{AccessToken: "token-" + string(rune('0'+n)), ExpiresIn: expiresIn}, nil
	}
	ts := &refreshingTokenSource{mint: mint, now: func() time.Time { return *now }}
	ts.set(&authTokenResponse{AccessToken: "token-0", ExpiresIn: expiresIn})
	return ts, &mints
}

func newAdminTestServer(t *testing.T, accept string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("Authorization") != "Bearer "+accept {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func mockAuthTokens(t *testing.T) *int32 {
	t.Helper()
	var mints int32
	getAuthTokenFunc = func(ctx context.Context, id, secret string, opts ...admin.ClientOption) (*authTokenResponse, error) {
		n := atomic.AddInt32(&mints, 1)
		return &authTokenResponse{AccessToken: "token-" + string(rune('0'+n)), ExpiresIn: 1800}, nil
	}
	t.Cleanup(func() { getAuthTokenFunc = getAuthToken })
	return &mints
}
