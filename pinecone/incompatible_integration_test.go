//go:build integration

package pinecone

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/pinecone-io/go-pinecone/v8/internal/gen"
	"github.com/pinecone-io/go-pinecone/v8/internal/gen/db_control"
	"github.com/stretchr/testify/require"
)

// An SDK reads a missing incompatible_* key as an empty list, so this checks the raw responses:
// every list endpoint sends the key, even when no resource is incompatible.
func TestIncompatibleListKeysIntegration(t *testing.T) {
	apiKey := os.Getenv("PINECONE_API_KEY")
	require.NotEmpty(t, apiKey, "PINECONE_API_KEY is required")
	pc, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: "pinecone_test_go_sdk"})
	require.NoError(t, err)
	version := gen.PineconeApiVersion

	cases := []struct {
		key  string
		list func(context.Context) (*http.Response, error)
	}{
		{"incompatible_indexes", func(ctx context.Context) (*http.Response, error) {
			return pc.restClient.ListIndexes(ctx, &db_control.ListIndexesParams{XPineconeApiVersion: version})
		}},
		{"incompatible_collections", func(ctx context.Context) (*http.Response, error) {
			return pc.restClient.ListCollections(ctx, &db_control.ListCollectionsParams{XPineconeApiVersion: version})
		}},
		{"incompatible_backups", func(ctx context.Context) (*http.Response, error) {
			return pc.restClient.ListProjectBackups(ctx, nil)
		}},
		{"incompatible_restore_jobs", func(ctx context.Context) (*http.Response, error) {
			return pc.restClient.ListRestoreJobs(ctx, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			res, err := tc.list(t.Context())
			require.NoError(t, err)
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, res.StatusCode, "body: %s", body)

			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &fields))
			raw, ok := fields[tc.key]
			require.True(t, ok, "response has no %q key: %s", tc.key, body)
			var entries []json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &entries), "%q is not an array: %s", tc.key, raw)
		})
	}
}
