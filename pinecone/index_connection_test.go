package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type IndexConnectionTests struct {
	suite.Suite
	host             string
	dimension        int32
	apiKey           string
	indexType        string
	idxConn          *IndexConnection
	sourceTag        string
	idxConnSourceTag *IndexConnection
	vectorIds        []string
}

// Runs the test suite with `go test`
func TestIndexConnection(t *testing.T) {
	apiKey := os.Getenv("PINECONE_API_KEY")
	assert.NotEmptyf(t, apiKey, "PINECONE_API_KEY env variable not set")

	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	if err != nil {
		t.FailNow()
	}

	podIndexName := os.Getenv("TEST_POD_INDEX_NAME")
	assert.NotEmptyf(t, podIndexName, "TEST_POD_INDEX_NAME env variable not set")

	podIdx, err := client.DescribeIndex(context.Background(), podIndexName)
	if err != nil {
		t.FailNow()
	}

	podTestSuite := new(IndexConnectionTests)
	podTestSuite.indexType = "pod"
	podTestSuite.host = podIdx.Host
	podTestSuite.dimension = podIdx.Dimension
	podTestSuite.apiKey = apiKey

	serverlessIndexName := os.Getenv("TEST_SERVERLESS_INDEX_NAME")
	assert.NotEmptyf(t, serverlessIndexName, "TEST_SERVERLESS_INDEX_NAME env variable not set")

	serverlessIdx, err := client.DescribeIndex(context.Background(), serverlessIndexName)
	if err != nil {
		t.FailNow()
	}

	serverlessTestSuite := new(IndexConnectionTests)
	serverlessTestSuite.indexType = "serverless"
	serverlessTestSuite.host = serverlessIdx.Host
	serverlessTestSuite.dimension = serverlessIdx.Dimension
	serverlessTestSuite.apiKey = apiKey

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}

func (ts *IndexConnectionTests) SetupSuite() {
	assert.NotEmptyf(ts.T(), ts.host, "HOST env variable not set")
	assert.NotEmptyf(ts.T(), ts.apiKey, "API_KEY env variable not set")
	additionalMetadata := map[string]string{"api-key": ts.apiKey}

	namespace, err := uuid.NewV7()
	assert.NoError(ts.T(), err)

	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ""})
	assert.NoError(ts.T(), err)
	ts.idxConn = idxConn

	ts.sourceTag = "test_source_tag"
	idxConnSourceTag, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ts.sourceTag})
	assert.NoError(ts.T(), err)
	ts.idxConnSourceTag = idxConnSourceTag

	ts.loadData()
}

func (ts *IndexConnectionTests) TearDownSuite() {
	ts.truncateData()

	err := ts.idxConn.Close()
	assert.NoError(ts.T(), err)

	err = ts.idxConnSourceTag.Close()
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) TestNewIndexConnection() {
	apiKey := "test-api-key"
	namespace := ""
	sourceTag := ""
	additionalMetadata := map[string]string{"api-key": apiKey}
	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace,
		sourceTag:          sourceTag})

	require.NoError(ts.T(), err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(ts.T(), ok, "Expected client to have an 'api-key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Empty(ts.T(), idxConn.Namespace, "Expected idxConn to have empty namespace, but got '%s'", idxConn.Namespace)
	require.NotNil(ts.T(), idxConn.dataClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(ts.T(), idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")
}

func (ts *IndexConnectionTests) TestNewIndexConnectionNamespace() {
	apiKey := "test-api-key"
	namespace := "test-namespace"
	sourceTag := "test-source-tag"
	additionalMetadata := map[string]string{"api-key": apiKey}
	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace,
		sourceTag:          sourceTag})

	require.NoError(ts.T(), err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(ts.T(), ok, "Expected client to have an 'api-key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Equal(ts.T(), namespace, idxConn.Namespace, "Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.Namespace)
	require.NotNil(ts.T(), idxConn.dataClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(ts.T(), idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")
}

func (ts *IndexConnectionTests) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestFetchVectorsSourceTag() {
	ctx := context.Background()
	res, err := ts.idxConnSourceTag.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByVector() {
	vec := make([]float32, ts.dimension)
	for i := range vec {
		vec[i] = 0.01
	}

	req := &QueryByVectorValuesRequest{
		Vector: vec,
		TopK:   5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorValues(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByVectorSourceTag() {
	vec := make([]float32, ts.dimension)
	for i := range vec {
		vec[i] = 0.01
	}

	req := &QueryByVectorValuesRequest{
		Vector: vec,
		TopK:   5,
	}

	ctx := context.Background()
	res, err := ts.idxConnSourceTag.QueryByVectorValues(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryById() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByIdSourceTag() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConnSourceTag.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDeleteVectorsById() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsById(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDeleteVectorsByFilter() {
	metadataFilter := map[string]interface{}{
		"genre": "classical",
	}
	filter, err := structpb.NewStruct(metadataFilter)
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to create metadata filter: %v", err))
	}

	ctx := context.Background()
	err = ts.idxConn.DeleteVectorsByFilter(ctx, filter)

	if ts.indexType == "serverless" {
		assert.Error(ts.T(), err)
		assert.Containsf(ts.T(), err.Error(), "Serverless and Starter indexes do not support deleting with metadata filtering", "Expected error message to contain 'Serverless and Starter indexes do not support deleting with metadata filtering'")
	} else {
		assert.NoError(ts.T(), err)
	}

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDescribeIndexStats() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStats(ctx)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDescribeIndexStatsFiltered() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStatsFiltered(ctx, &MetadataFilter{})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{}

	ctx := context.Background()
	res, err := ts.idxConn.ListVectors(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) loadData() {
	vals := []float32{0.01, 0.02, 0.03, 0.04, 0.05}
	vectors := make([]*Vector, len(vals))
	ts.vectorIds = make([]string, len(vals))

	for i, val := range vals {
		vec := make([]float32, ts.dimension)
		for i := range vec {
			vec[i] = val
		}

		id := fmt.Sprintf("vec-%d", i+1)
		ts.vectorIds[i] = id

		vectors[i] = &Vector{
			Id:     id,
			Values: vec,
		}
	}

	ctx := context.Background()
	_, err := ts.idxConn.UpsertVectors(ctx, vectors)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) loadDataSourceTag() {
	vals := []float32{0.01, 0.02, 0.03, 0.04, 0.05}
	vectors := make([]*Vector, len(vals))
	ts.vectorIds = make([]string, len(vals))

	for i, val := range vals {
		vec := make([]float32, ts.dimension)
		for i := range vec {
			vec[i] = val
		}

		id := fmt.Sprintf("vec-%d", i+1)
		ts.vectorIds[i] = id

		vectors[i] = &Vector{
			Id:     id,
			Values: vec,
		}
	}

	ctx := context.Background()
	_, err := ts.idxConnSourceTag.UpsertVectors(ctx, vectors)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) truncateData() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) TestMetadataAppliedToRequests() {
	apiKey := "test-api-key"
	namespace := "test-namespace"
	sourceTag := "test-source-tag"
	additionalMetadata := map[string]string{"api-key": apiKey, "test-meta": "test-value"}

	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace,
		sourceTag:          sourceTag,
	},
		grpc.WithUnaryInterceptor(utils.MetadataInterceptor(ts.T(), additionalMetadata)),
	)

	require.NoError(ts.T(), err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(ts.T(), ok, "Expected client to have an 'api-key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Equal(ts.T(), namespace, idxConn.Namespace, "Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.Namespace)
	require.NotNil(ts.T(), idxConn.dataClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(ts.T(), idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")

	// initiate request to trigger the MetadataInterceptor
	stats, err := idxConn.DescribeIndexStats(context.Background())
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to describe index stats: %v", err))
	}

	require.NotNil(ts.T(), stats)
}

func TestMarshalFetchVectorsResponse(t *testing.T) {
	tests := []struct {
		name  string
		input FetchVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: FetchVectorsResponse{
				Vectors: map[string]*Vector{
					"vec-1": {Id: "vec-1", Values: []float32{0.01, 0.01, 0.01}},
					"vec-2": {Id: "vec-2", Values: []float32{0.02, 0.02, 0.02}},
				},
				Usage: &Usage{ReadUnits: 5},
			},
			want: `{"vectors":{"vec-1":{"id":"vec-1","values":[0.01,0.01,0.01]},"vec-2":{"id":"vec-2","values":[0.02,0.02,0.02]}},"usage":{"read_units":5}}`,
		},
		{
			name:  "Fields omitted",
			input: FetchVectorsResponse{},
			want:  `{}`,
		},
		{
			name: "Fields empty",
			input: FetchVectorsResponse{
				Vectors: nil,
				Usage:   nil,
			},
			want: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal FetchVectorsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal FetchVectorsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestMarshalListVectorsResponse(t *testing.T) {
	vectorId1 := "vec-1"
	vectorId2 := "vec-2"
	paginationToken := "next-token"
	tests := []struct {
		name  string
		input ListVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: ListVectorsResponse{
				VectorIds:           []*string{&vectorId1, &vectorId2},
				Usage:               &Usage{ReadUnits: 5},
				NextPaginationToken: &paginationToken,
			},
			want: `{"vector_ids":["vec-1","vec-2"],"usage":{"read_units":5},"next_pagination_token":"next-token"}`,
		},
		{
			name:  "Fields omitted",
			input: ListVectorsResponse{},
			want:  `{}`,
		},
		{
			name: "Fields empty",
			input: ListVectorsResponse{
				VectorIds:           nil,
				Usage:               nil,
				NextPaginationToken: nil,
			},
			want: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal ListVectorsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal ListVectorsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestMarshalQueryVectorsResponse(t *testing.T) {
	tests := []struct {
		name  string
		input QueryVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: QueryVectorsResponse{
				Matches: []*ScoredVector{
					{Vector: &Vector{Id: "vec-1", Values: []float32{0.01, 0.01, 0.01}}, Score: 0.1},
					{Vector: &Vector{Id: "vec-2", Values: []float32{0.02, 0.02, 0.02}}, Score: 0.2},
				},
				Usage: &Usage{ReadUnits: 5},
			},
			want: `{"matches":[{"vector":{"id":"vec-1","values":[0.01,0.01,0.01]},"score":0.1},{"vector":{"id":"vec-2","values":[0.02,0.02,0.02]},"score":0.2}],"usage":{"read_units":5}}`,
		},
		{
			name:  "Fields omitted",
			input: QueryVectorsResponse{},
			want:  `{}`,
		},
		{
			name:  "Fields empty",
			input: QueryVectorsResponse{Matches: nil, Usage: nil},
			want:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal QueryVectorsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal QueryVectorsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestMarshalDescribeIndexStatsResponse(t *testing.T) {
	tests := []struct {
		name  string
		input DescribeIndexStatsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: DescribeIndexStatsResponse{
				Dimension:        3,
				IndexFullness:    0.5,
				TotalVectorCount: 100,
				Namespaces: map[string]*NamespaceSummary{
					"namespace-1": {VectorCount: 50},
				},
			},
			want: `{"dimension":3,"index_fullness":0.5,"total_vector_count":100,"namespaces":{"namespace-1":{"vector_count":50}}}`,
		},
		{
			name:  "Fields omitted",
			input: DescribeIndexStatsResponse{},
			want:  `{"dimension":0,"index_fullness":0,"total_vector_count":0}`,
		},
		{
			name: "Fields empty",
			input: DescribeIndexStatsResponse{
				Dimension:        0,
				IndexFullness:    0,
				TotalVectorCount: 0,
				Namespaces:       nil,
			},
			want: `{"dimension":0,"index_fullness":0,"total_vector_count":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal DescribeIndexStatsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal DescribeIndexStatsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}
