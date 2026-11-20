//go:build integration

package pinecone

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestIndexCRUDIntegration exercises the 2026-10 control plane on empty, short-lived indexes.
// Run with PINECONE_API_KEY and go test -count=1 -tags integration -run '^TestIndexCRUDIntegration$' ./pinecone.
func TestIndexCRUDIntegration(t *testing.T) {
	apiKey := os.Getenv("PINECONE_API_KEY")
	require.NotEmpty(t, apiKey, "PINECONE_API_KEY is required")
	pc, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: "pinecone_test_go_sdk"})
	require.NoError(t, err)

	dense := DenseVectorField{Dimension: 5, Metric: IndexMetricCosine}
	if !t.Run("document", func(t *testing.T) {
		testIndexCRUD(t, pc, "your_vector", dense, nil, func(ctx context.Context, name string) (*Index, error) {
			return pc.CreateIndex(ctx, &CreateIndexRequest{
				Name: name,
				Schema: IndexSchema{Fields: map[string]IndexSchemaField{
					"your_vector": {DenseVector: &dense},
				}},
			})
		})
	}) {
		return
	}

	t.Run("vector", func(t *testing.T) {
		testIndexCRUD(t, pc, "_values", dense, nil, func(ctx context.Context, name string) (*Index, error) {
			return pc.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
				Name:      name,
				Dimension: &dense.Dimension,
				Metric:    &dense.Metric,
				Cloud:     CloudAWS,
				Region:    "us-east-1",
			})
		})
	})
}

func testIndexCRUD(t *testing.T, pc *Client, field string, dense DenseVectorField, auto *ReadCapacityAutoScaling, create func(context.Context, string) (*Index, error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	name := "go2610-" + uuid.NewString()
	t.Logf("creating index %s", name)
	created, err := create(ctx, name)
	// A failed response can still follow a successful create. Clean up the unique name
	// even on a transport or decoding failure. A conflict means the name belongs to someone else.
	if !hasIntegrationHTTPStatus(err, http.StatusConflict) {
		t.Cleanup(func() { deleteIntegrationIndex(t, pc, name) })
	}
	require.NoError(t, err, "create index %s", name)
	require.NotNil(t, created)
	require.Equal(t, name, created.Name)
	require.NotEmpty(t, created.Host)

	tags := IndexTags{"sdk-integration": "updated"}
	configured, err := pc.ConfigureIndex(ctx, name, ConfigureIndexParams{Tags: tags})
	require.NoError(t, err, "configure index %s", name)
	require.NotNil(t, configured)
	require.Equal(t, name, configured.Name)

	described, err := pc.DescribeIndex(ctx, name)
	require.NoError(t, err, "describe index %s", name)
	checkIntegrationIndex(t, described, name, field, dense, tags, auto)

	indexes, err := pc.ListIndexes(ctx)
	require.NoError(t, err, "list indexes")
	checkIntegrationIndex(t, findIntegrationIndex(indexes, name), name, field, dense, tags, auto)
}

func checkIntegrationIndex(t *testing.T, index *Index, name, field string, dense DenseVectorField, tags IndexTags, auto *ReadCapacityAutoScaling) {
	t.Helper()
	require.NotNil(t, index, "index %s", name)
	require.Equal(t, name, index.Name)
	require.NotNil(t, index.Schema)
	require.Equal(t, &dense, index.Schema.Fields[field].DenseVector)
	require.NotNil(t, index.Tags)
	for key, value := range tags {
		require.Equal(t, value, (*index.Tags)[key], "tag %s", key)
	}
	require.NotNil(t, index.ReadCapacity)
	if auto == nil {
		require.NotNil(t, index.ReadCapacity.OnDemand)
		require.Nil(t, index.ReadCapacity.Dedicated)
		return
	}
	dedicated := index.ReadCapacity.Dedicated
	require.NotNil(t, dedicated)
	require.Equal(t, ptr(ReadCapacityScalingStrategyAuto), dedicated.ScalingStrategy)
	require.Equal(t, ptr(int32(1)), dedicated.Shards)
	require.NotNil(t, dedicated.Scaling)
	require.Equal(t, auto, dedicated.Scaling.Auto)
	require.Nil(t, dedicated.Scaling.Manual)
}

func deleteIntegrationIndex(t *testing.T, pc *Client, name string) {
	t.Helper()
	// Use a fresh context so cleanup still runs if an operation timed out or failed.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err := pc.DeleteIndex(ctx, name)
	if !hasIntegrationHTTPStatus(err, http.StatusNotFound) {
		require.NoError(t, err, "delete index %s", name)
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		_, err := pc.DescribeIndex(ctx, name)
		if hasIntegrationHTTPStatus(err, http.StatusNotFound) {
			indexes, err := pc.ListIndexes(ctx)
			require.NoError(t, err, "list indexes after deleting %s", name)
			if findIntegrationIndex(indexes, name) == nil {
				t.Logf("confirmed deleted: %s (describe returned 404 and list omitted it)", name)
				return
			}
		} else {
			require.NoError(t, err, "confirm deletion of %s", name)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("index %s is still visible after deletion: %v", name, ctx.Err())
		case <-ticker.C:
		}
	}
}

func findIntegrationIndex(indexes []*Index, name string) *Index {
	for _, index := range indexes {
		if index.Name == name {
			return index
		}
	}
	return nil
}

func hasIntegrationHTTPStatus(err error, status int) bool {
	var apiErr *PineconeError
	return errors.As(err, &apiErr) && apiErr.Code == status
}
