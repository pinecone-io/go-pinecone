//go:build smoke

// Package smoke holds the mocked critical-path smoke gate — the key-free half
// of go-pinecone's smoke-test coverage. Unlike the keyed suites under
// ./pinecone (which hit a real backend and require PINECONE_API_KEY) and the
// localServer suite (which needs the Dockerized mock index), this test stands
// up its own in-process mocks and runs on every pull request with no API key.
//
// It guards the three most critical use cases against a regression in the
// request/response plumbing:
//
//  1. connect — construct a client and resolve an index host via DescribeIndex
//  2. upsert  — write vectors to the data plane
//  3. query   — read them back by vector similarity
//
// The mocks are injected at the transport layer, not at the SDK's public API:
//
//   - The control plane (DescribeIndex) is REST, so it is mocked with an
//     httptest.Server pointed at via NewClientParams.Host.
//   - The data plane (UpsertVectors / QueryByVectorValues) is gRPC, so it is
//     mocked with a real in-process grpc.Server implementing VectorServiceServer.
//
// Everything above the wire — config resolution, request building, gRPC
// marshaling, and response deserialization — is the real code path, so this
// catches wiring regressions that an SDK-object-level mock would paper over.
// This is the Go analogue of the Python SDK's respx-backed gate
// (tests/smoke/test_mocked_critical_path_*.py) and the TS SDK's fetchApi-injected
// gate (pinecone-ts-client src/smoke/mockedCriticalPath.test.ts).
//
// The build tag `smoke` isolates this from the default `go test ./pinecone`
// run; CI invokes it explicitly with `-tags smoke -run '^TestMockedCriticalPath$'`
// and asserts the test actually ran (see .github/workflows/ci.yaml), so the job
// fails rather than silently passing if this file is ever moved or emptied.
package smoke

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db_data_grpc "github.com/pinecone-io/go-pinecone/v6/internal/gen/db_data/grpc"
	"github.com/pinecone-io/go-pinecone/v6/pinecone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const (
	indexName = "mocked-critical-path"
	namespace = "smoke-ns"
)

// mockVectorService is an in-process implementation of the data-plane gRPC
// service. It records which legs of the critical path were exercised and the
// requests it received, so the test can assert the real request-building path
// produced what we expect on the wire.
type mockVectorService struct {
	db_data_grpc.UnimplementedVectorServiceServer

	upsertCalls int
	queryCalls  int
	lastUpsert  *db_data_grpc.UpsertRequest
	lastQuery   *db_data_grpc.QueryRequest
}

func (m *mockVectorService) Upsert(_ context.Context, req *db_data_grpc.UpsertRequest) (*db_data_grpc.UpsertResponse, error) {
	m.upsertCalls++
	m.lastUpsert = req
	return &db_data_grpc.UpsertResponse{UpsertedCount: uint32(len(req.Vectors))}, nil
}

func (m *mockVectorService) Query(_ context.Context, req *db_data_grpc.QueryRequest) (*db_data_grpc.QueryResponse, error) {
	m.queryCalls++
	m.lastQuery = req
	return &db_data_grpc.QueryResponse{
		Matches: []*db_data_grpc.ScoredVector{
			{Id: "v1", Score: 0.99},
			{Id: "v2", Score: 0.87},
		},
	}, nil
}

// startMockDataPlane stands up the in-process gRPC data-plane server on a
// loopback port and returns its "http://host:port" address (the http scheme
// tells the SDK to dial insecurely) plus the recording service.
func startMockDataPlane(t *testing.T) (string, *mockVectorService) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to listen for mock data plane")

	svc := &mockVectorService{}
	srv := grpc.NewServer()
	db_data_grpc.RegisterVectorServiceServer(srv, svc)

	go func() {
		// Serve returns when srv.Stop() is called during cleanup.
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)

	return "http://" + lis.Addr().String(), svc
}

// startMockControlPlane stands up the httptest-backed REST control plane. Its
// GET /indexes/{name} handler returns a describe-index body whose host points
// at the gRPC data plane, so the resolved host flows into the data-plane
// connection exactly as it would in production. Any unexpected request fails
// the test loudly rather than passing silently.
func startMockControlPlane(t *testing.T, dataPlaneHost string) (string, *int) {
	t.Helper()

	describeHits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/indexes/"+indexName, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s on describe-index route", r.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		describeHits++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":        indexName,
			"dimension":   3,
			"metric":      "cosine",
			"host":        dataPlaneHost,
			"spec":        map[string]any{"serverless": map[string]any{"cloud": "gcp", "region": "us-east1"}},
			"status":      map[string]any{"ready": true, "state": "Ready"},
			"vector_type": "dense",
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to mocked control plane: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected request", http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv.URL, &describeHits
}

func floats(vals ...float32) *[]float32 {
	return &vals
}

// TestMockedCriticalPath drives connect -> upsert -> query against fully
// in-process mocks with no PINECONE_API_KEY. The dummy API key below is only to
// satisfy NewClient's constructor; the mocks never validate it.
func TestMockedCriticalPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dataPlaneHost, dataPlane := startMockDataPlane(t)
	controlPlaneURL, describeHits := startMockControlPlane(t, dataPlaneHost)

	// 1. connect: build the client against the mocked control plane and resolve
	//    the data-plane host via DescribeIndex.
	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: "mocked-key",
		Host:   controlPlaneURL,
	})
	require.NoError(t, err, "NewClient should succeed against the mocked control plane")

	idx, err := pc.DescribeIndex(ctx, indexName)
	require.NoError(t, err, "DescribeIndex should succeed against the mocked control plane")
	require.Equal(t, indexName, idx.Name)
	require.Equal(t, dataPlaneHost, idx.Host, "resolved host should be the mocked data plane")

	idxConn, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: namespace})
	require.NoError(t, err, "Index connection should be created from the resolved host")
	t.Cleanup(func() { _ = idxConn.Close() })

	// 2. upsert: write vectors to the mocked data plane.
	upsertCount, err := idxConn.UpsertVectors(ctx, []*pinecone.Vector{
		{Id: "v1", Values: floats(0.1, 0.2, 0.3)},
		{Id: "v2", Values: floats(0.4, 0.5, 0.6)},
		{Id: "v3", Values: floats(0.7, 0.8, 0.9)},
	})
	require.NoError(t, err, "UpsertVectors should succeed")
	assert.Equal(t, uint32(3), upsertCount, "mock echoes back the number of vectors upserted")

	// 3. query: read them back by vector similarity.
	queryResp, err := idxConn.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:        []float32{0.1, 0.2, 0.3},
		TopK:          2,
		IncludeValues: false,
	})
	require.NoError(t, err, "QueryByVectorValues should succeed")
	require.Len(t, queryResp.Matches, 2, "mock returns two matches")
	assert.Equal(t, "v1", queryResp.Matches[0].Vector.Id)
	assert.InDelta(t, 0.99, queryResp.Matches[0].Score, 1e-6)
	assert.Equal(t, "v2", queryResp.Matches[1].Vector.Id)

	// Every leg of the critical path was actually exercised over the wire, and
	// the real request-building path put our inputs on the wire correctly.
	assert.Equal(t, 1, *describeHits, "describe-index should be called exactly once")

	require.Equal(t, 1, dataPlane.upsertCalls, "upsert should hit the data plane exactly once")
	assert.Equal(t, namespace, dataPlane.lastUpsert.Namespace, "namespace should be propagated to upsert")
	assert.Len(t, dataPlane.lastUpsert.Vectors, 3, "all three vectors should reach the wire")

	require.Equal(t, 1, dataPlane.queryCalls, "query should hit the data plane exactly once")
	assert.Equal(t, namespace, dataPlane.lastQuery.Namespace, "namespace should be propagated to query")
	assert.Equal(t, uint32(2), dataPlane.lastQuery.TopK, "topK should be propagated to query")
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, dataPlane.lastQuery.Vector, "query vector should reach the wire")
}
