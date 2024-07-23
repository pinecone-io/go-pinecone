package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc/status"

	"google.golang.org/grpc/codes"

	"github.com/pinecone-io/go-pinecone/internal/gen/data"
	"google.golang.org/grpc/metadata"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

// Integration tests:
type IndexConnectionTestsIntegration struct {
	suite.Suite
	host              string
	dimension         int32
	apiKey            string
	indexType         string
	idxConn           *IndexConnection
	sourceTag         string
	idxConnSourceTag  *IndexConnection
	vectorIds         []string
	client            *Client
	podIdxName        string
	serverlessIdxName string
}

func TestIntegrationIndexConnection(t *testing.T) {
	apiKey := os.Getenv("PINECONE_API_KEY")
	assert.NotEmptyf(t, apiKey, "PINECONE_API_KEY env variable not set")

	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: map[string]string{"content-type": "application/json"}})
	require.NotNil(t, client, "Client should not be nil after creation")
	require.NoError(t, err)

	podIndexName := os.Getenv("TEST_PODS_INDEX_NAME")
	assert.NotEmptyf(t, podIndexName, "TEST_PODS_INDEX_NAME env variable not set")

	serverlessIndexName := os.Getenv("TEST_SERVERLESS_INDEX_NAME")
	assert.NotEmptyf(t, serverlessIndexName, "TEST_SERVERLESS_INDEX_NAME env variable not set")

	serverlessIdx := buildServerlessTestIndex(client, serverlessIndexName)
	podIdx := buildPodTestIndex(client, podIndexName)

	podTestSuite := &IndexConnectionTestsIntegration{
		host:       podIdx.Host,
		dimension:  podIdx.Dimension,
		apiKey:     apiKey,
		indexType:  "pods",
		client:     client,
		podIdxName: podIdx.Name,
	}

	serverlessTestSuite := &IndexConnectionTestsIntegration{
		host:              serverlessIdx.Host,
		dimension:         serverlessIdx.Dimension,
		apiKey:            apiKey,
		indexType:         "serverless",
		client:            client,
		serverlessIdxName: serverlessIdx.Name,
	}

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}

func (ts *IndexConnectionTestsIntegration) SetupSuite() {
	ctx := context.Background()

	assert.NotEmptyf(ts.T(), ts.host, "HOST env variable not set")
	assert.NotEmptyf(ts.T(), ts.apiKey, "API_KEY env variable not set")
	additionalMetadata := map[string]string{"api-key": ts.apiKey, "content-type": "application/json"}

	namespace, err := uuid.NewUUID()
	require.NoError(ts.T(), err)

	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ""})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), idxConn, "Failed to create idxConn")

	ts.idxConn = idxConn

	// Deterministically create vectors
	vectors := createVectorsForUpsert()

	// Set vector IDs
	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}
	ts.vectorIds = vectorIds

	// Upsert vectors
	err = upsert(ts, ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in SetupSuite: %v", err)
	}

	ts.sourceTag = "test_source_tag"
	idxConnSourceTag, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ts.sourceTag})
	require.NoError(ts.T(), err)
	ts.idxConnSourceTag = idxConnSourceTag

	fmt.Printf("\n %s set up suite completed successfully\n", ts.indexType)
}

func (ts *IndexConnectionTestsIntegration) TearDownSuite() {
	ctx := context.Background()

	// Delete test indexes
	err := ts.client.DeleteIndex(ctx, ts.serverlessIdxName)
	err = ts.client.DeleteIndex(ctx, ts.podIdxName)

	// TODO Delete test collections?

	err = ts.idxConn.Close()
	require.NoError(ts.T(), err)

	err = ts.idxConnSourceTag.Close()
	require.NoError(ts.T(), err)
	fmt.Printf("\n %s setup suite torn down successfully\n", ts.indexType)
}

func (ts *IndexConnectionTestsIntegration) TestNewIndexConnection() {
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

func (ts *IndexConnectionTestsIntegration) TestNewIndexConnectionNamespace() {
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

func (ts *IndexConnectionTestsIntegration) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestFetchVectorsSourceTag() {
	ctx := context.Background()
	res, err := ts.idxConnSourceTag.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestQueryByVector() {
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

func (ts *IndexConnectionTestsIntegration) TestQueryByVectorSourceTag() {
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

func (ts *IndexConnectionTestsIntegration) TestQueryById() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestQueryByIdSourceTag() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConnSourceTag.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestDeleteVectorsById() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsById(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)

	_, err = ts.idxConn.UpsertVectors(ctx, createVectorsForUpsert())
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}
}

func (ts *IndexConnectionTestsIntegration) TestDeleteVectorsByFilter() {
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

	_, err = ts.idxConn.UpsertVectors(ctx, createVectorsForUpsert())
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}
}

func (ts *IndexConnectionTestsIntegration) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)

	_, err = ts.idxConn.UpsertVectors(ctx, createVectorsForUpsert())
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}

}

func (ts *IndexConnectionTestsIntegration) TestDescribeIndexStats() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStats(ctx)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestDescribeIndexStatsFiltered() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStatsFiltered(ctx, &MetadataFilter{})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{}

	ctx := context.Background()
	res, err := ts.idxConn.ListVectors(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTestsIntegration) TestMetadataAppliedToRequests() {
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

func (ts *IndexConnectionTestsIntegration) TestUpdateVectorValues() {
	ctx := context.Background()

	expectedVals := []float32{7.2, 7.2, 7.2, 7.2, 7.2}
	err := ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:     ts.vectorIds[0],
		Values: expectedVals,
	})
	assert.NoError(ts.T(), err)

	time.Sleep(5 * time.Second)

	vector, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to fetch vector: %v", err))
	}
	actualVals := vector.Vectors[ts.vectorIds[0]].Values

	assert.ElementsMatch(ts.T(), expectedVals, actualVals, "Values do not match")
}

func (ts *IndexConnectionTestsIntegration) TestUpdateVectorMetadata() {
	ctx := context.Background()

	expectedMetadata := map[string]interface{}{
		"genre": "death-metal",
	}
	expectedMetadataMap, err := structpb.NewStruct(expectedMetadata)

	err = ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:       ts.vectorIds[0],
		Metadata: expectedMetadataMap,
	})
	assert.NoError(ts.T(), err)

	time.Sleep(5 * time.Second)

	vector, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to fetch vector: %v", err))
	}

	expectedGenre := expectedMetadataMap.Fields["genre"].GetStringValue()
	actualGenre := vector.Vectors[ts.vectorIds[0]].Metadata.Fields["genre"].GetStringValue()

	assert.Equal(ts.T(), expectedGenre, actualGenre, "Metadata does not match")
}

func (ts *IndexConnectionTestsIntegration) TestUpdateVectorSparseValues() {
	ctx := context.Background()

	dims := int(ts.dimension)
	indices := generateUint32Array(dims)
	vals := generateFloat32Array(dims)
	expectedSparseValues := SparseValues{
		Indices: indices,
		Values:  vals,
	}

	err := ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:           ts.vectorIds[0],
		SparseValues: &expectedSparseValues,
	})
	assert.NoError(ts.T(), err)

	time.Sleep(5 * time.Second)

	vector, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to fetch vector: %v", err))
	}
	actualSparseValues := vector.Vectors[ts.vectorIds[0]].SparseValues.Values

	assert.ElementsMatch(ts.T(), expectedSparseValues.Values, actualSparseValues, "Sparse values do not match")
}

// Unit tests:
func TestMarshalFetchVectorsResponseUnit(t *testing.T) {
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

func TestMarshalListVectorsResponseUnit(t *testing.T) {
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

func TestMarshalQueryVectorsResponseUnit(t *testing.T) {
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

func TestMarshalDescribeIndexStatsResponseUnit(t *testing.T) {
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

func TestToVectorUnit(t *testing.T) {
	tests := []struct {
		name     string
		vector   *data.Vector
		expected *Vector
	}{
		{
			name:     "Pass nil vector, expect nil to be returned",
			vector:   nil,
			expected: nil,
		},
		{
			name: "Pass dense vector",
			vector: &data.Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
			expected: &Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
		},
		{
			name: "Pass sparse vector",
			vector: &data.Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector",
			vector: &data.Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},

			expected: &Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector with metadata",
			vector: &data.Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					}},
			},
			expected: &Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toVector(tt.vector)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestToSparseValuesUnit(t *testing.T) {
	tests := []struct {
		name         string
		sparseValues *data.SparseValues
		expected     *SparseValues
	}{
		{
			name:         "Pass nil sparse values, expect nil to be returned",
			sparseValues: nil,
			expected:     nil,
		},
		{
			name: "Pass sparse values",
			sparseValues: &data.SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
			expected: &SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toSparseValues(tt.sparseValues)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestToScoredVectorUnit(t *testing.T) {
	tests := []struct {
		name         string
		scoredVector *data.ScoredVector
		expected     *ScoredVector
	}{
		{
			name:         "Pass nil scored vector, expect nil to be returned",
			scoredVector: nil,
			expected:     nil,
		},
		{
			name: "Pass scored dense vector",
			scoredVector: &data.ScoredVector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.01, 0.01},
				Score:  0.1,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "dense-1",
					Values: []float32{0.01, 0.01, 0.01},
				},
				Score: 0.1,
			},
		},
		{
			name: "Pass scored sparse vector",
			scoredVector: &data.ScoredVector{
				Id: "sparse-1",
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Score: 0.2,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id: "sparse-1",
					SparseValues: &SparseValues{
						Indices: []uint32{0, 2},
						Values:  []float32{0.01, 0.03},
					},
				},
				Score: 0.2,
			},
		},
		{
			name: "Pass scored hybrid vector",
			scoredVector: &data.ScoredVector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Score: 0.3,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "hybrid-1",
					Values: []float32{0.01, 0.02, 0.03},
					SparseValues: &SparseValues{
						Indices: []uint32{0, 2},
						Values:  []float32{0.01, 0.03},
					},
				},
				Score: 0.3,
			},
		},
		{
			name: "Pass scored hybrid vector with metadata",
			scoredVector: &data.ScoredVector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					},
				},
				Score: 0.4,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "hybrid-metadata-1",
					Values: []float32{0.01, 0.02, 0.03},
					SparseValues: &SparseValues{
						Indices: []uint32{0, 2},
						Values:  []float32{0.01, 0.03},
					},
					Metadata: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
						},
					},
				},
				Score: 0.4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toScoredVector(tt.scoredVector)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestVecToGrpcUnit(t *testing.T) {
	tests := []struct {
		name     string
		vector   *Vector
		expected *data.Vector
	}{
		{
			name:     "Pass nil vector, expect nil to be returned",
			vector:   nil,
			expected: nil,
		},
		{
			name: "Pass dense vector",
			vector: &Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
			expected: &data.Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
		},
		{
			name: "Pass sparse vector",
			vector: &Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &data.Vector{
				Id: "sparse-1",
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector",
			vector: &Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &data.Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector with metadata",
			vector: &Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					},
				},
			},
			expected: &data.Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &data.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vecToGrpc(tt.vector)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestSparseValToGrpcUnit(t *testing.T) {
	tests := []struct {
		name         string
		sparseValues *SparseValues
		metadata     *structpb.Struct
		expected     *data.SparseValues
	}{
		{
			name:         "Pass nil sparse values, expect nil to be returned",
			sparseValues: nil,
			expected:     nil,
		},
		{
			name: "Pass sparse values",
			sparseValues: &SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
			expected: &data.SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
		},
		{
			name: "Pass sparse values with metadata (metadata is ignored)",
			sparseValues: &SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
			metadata: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
				},
			},
			expected: &data.SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sparseValToGrpc(tt.sparseValues)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestAkCtxUnit(t *testing.T) {
	tests := []struct {
		name               string
		additionalMetadata map[string]string
		initialMetadata    map[string]string
		expectedMetadata   map[string]string
	}{
		{
			name:               "No additional metadata in IndexConnection obj",
			additionalMetadata: nil,
			initialMetadata:    map[string]string{"initial-key": "initial-value"},
			expectedMetadata:   map[string]string{"initial-key": "initial-value"},
		},
		{
			name:               "With additional metadata in IndexConnection obj",
			additionalMetadata: map[string]string{"addtl-key1": "addtl-value1", "addtl-key2": "addtl-value2"},
			initialMetadata:    map[string]string{"initial-key": "initial-value"},
			expectedMetadata: map[string]string{
				"initial-key": "initial-value",
				"addtl-key1":  "addtl-value1",
				"addtl-key2":  "addtl-value2",
			},
		},
		{
			name: "Only additional metadata",
			additionalMetadata: map[string]string{
				"addtl-key1": "addtl-value1",
				"addtl-key2": "addtl-value2",
			},
			initialMetadata: nil,
			expectedMetadata: map[string]string{
				"addtl-key1": "addtl-value1",
				"addtl-key2": "addtl-value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &IndexConnection{additionalMetadata: tt.additionalMetadata}
			ctx := context.Background()

			// Add initial metadata to the context if provided
			if tt.initialMetadata != nil {
				md := metadata.New(tt.initialMetadata)
				ctx = metadata.NewOutgoingContext(ctx, md)
			}

			// Call the method
			newCtx := idx.akCtx(ctx)

			// Retrieve metadata from the new context
			md, ok := metadata.FromOutgoingContext(newCtx)
			assert.True(t, ok)

			// Check that the metadata matches the expected metadata
			for key, expectedValue := range tt.expectedMetadata {
				values := md[key]
				assert.Contains(t, values, expectedValue)
			}
		})
	}
}

func TestToUsageUnit(t *testing.T) {
	u5 := uint32(5)

	tests := []struct {
		name     string
		usage    *data.Usage
		expected *Usage
	}{
		{
			name:     "Pass nil usage, expect nil to be returned",
			usage:    nil,
			expected: nil,
		},
		{
			name: "Pass usage",
			usage: &data.Usage{
				ReadUnits: &u5,
			},
			expected: &Usage{
				ReadUnits: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toUsage(tt.usage)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestToPaginationToken(t *testing.T) {
	tokenForNilCase := ""
	tokenForPositiveCase := "next-token"

	tests := []struct {
		name     string
		token    *data.Pagination
		expected *string
	}{
		{
			name:     "Pass empty token, expect empty string to be returned",
			token:    &data.Pagination{},
			expected: &tokenForNilCase,
		},
		{
			name: "Pass token",
			token: &data.Pagination{
				Next: "next-token",
			},
			expected: &tokenForPositiveCase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toPaginationToken(tt.token)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

// Helper functions:
func (ts *IndexConnectionTestsIntegration) truncateData() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)
}

func createVectorsForUpsert() []*Vector {
	vectors := make([]*Vector, 5)
	for i := 0; i < 5; i++ {
		vectors[i] = &Vector{
			Id:     fmt.Sprintf("vector-%d", i+1),
			Values: []float32{float32(i), float32(i) + 0.1, float32(i) + 0.2, float32(i) + 0.3, float32(i) + 0.4},
			SparseValues: &SparseValues{
				Indices: []uint32{0, 1, 2, 3, 4},
				Values:  []float32{float32(i), float32(i) + 0.1, float32(i) + 0.2, float32(i) + 0.3, float32(i) + 0.4},
			},
			Metadata: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
				},
			},
		}
	}
	return vectors
}

func setDimensionsForTestIndexes() uint32 {
	return uint32(5)
}

func buildServerlessTestIndex(in *Client, idxName string) *Index {
	ctx := context.Background()
	//serverlessIndexName := retrieveServerlessIndexName(t)
	//indexes, err := in.ListIndexes(ctx)
	//if err != nil {
	//	log.Fatalf("Could not list indexes in buildServerlessTestIndex: %v", err)
	//}
	//for _, v := range indexes {
	//	if v.Name == serverlessIndexName {
	//		fmt.Printf("Found existing Serverless index: %s, deleting.\n", serverlessIndexName)
	//		err := in.DeleteIndex(ctx, v.Name)
	//		time.Sleep(5 * time.Second)
	//		if err != nil {
	//			log.Fatalf("Failed to delete Serverless index \"%s\" in buildServerlessTestIndex Tests: %v", err, serverlessIndexName)
	//		}
	//	}
	//}

	fmt.Printf("Creating Serverless index: %s\n", idxName)
	serverlessIdx, err := in.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:      idxName,
		Dimension: int32(setDimensionsForTestIndexes()),
		Metric:    Cosine,
		Region:    "us-east-1",
		Cloud:     "aws",
	})
	if err != nil {
		log.Fatalf("Failed to create Serverless index \"%s\" in integration test: %v", err, idxName)
	} else {
		fmt.Printf("Successfully created a new Serverless index: %s!\n", idxName)
	}
	return serverlessIdx
}

func buildPodTestIndex(in *Client, name string) *Index {
	ctx := context.Background()
	//podIndexName := retrievePodIndexName(t)

	//indexes, err := in.ListIndexes(ctx)
	//if err != nil {
	//	log.Fatalf("Could not list indexes in buildPodTestIndex: %v", err)
	//}
	//for _, v := range indexes {
	//	if v.Name == podIndexName {
	//		fmt.Printf("Found existing pod index: %s, deleting.\n", podIndexName)
	//		err := in.DeleteIndex(ctx, podIndexName)
	//		time.Sleep(5 * time.Second)
	//		if err != nil {
	//			log.Fatalf("Failed to delete pod index in buildPodTestIndex test: %v", err)
	//		}
	//	}
	//}

	fmt.Printf("Creating pod index: %s\n", name)
	podIdx, err := in.CreatePodIndex(ctx, &CreatePodIndexRequest{
		Name:        name,
		Dimension:   int32(setDimensionsForTestIndexes()),
		Metric:      Cosine,
		Environment: "us-east-1-aws",
		PodType:     "p1",
	})
	if err != nil {
		log.Fatalf("Failed to create pod index in buildPodTestIndex test: %v", err)
	} else {
		fmt.Printf("Successfully created a new pod index: %s!\n", name)
	}
	return podIdx
}

// TODO: necessary?
func generateFloat32Array(n int) []float32 {
	array := make([]float32, n)
	for i := 0; i < n; i++ {
		array[i] = float32(i)
	}
	return array
}

// TODO: necessary?
func generateUint32Array(n int) []uint32 {
	array := make([]uint32, n)
	for i := 0; i < n; i++ {
		array[i] = uint32(i)
	}
	return array
}

func getStatus(ts *IndexConnectionTestsIntegration, ctx context.Context) (bool, error) {
	var indexName string
	if ts.indexType == "serverless" {
		indexName = ts.serverlessIdxName
	} else if ts.indexType == "pods" {
		indexName = ts.podIdxName
	}
	if ts.client == nil {
		return false, fmt.Errorf("client is nil")
	}

	var desc *Index
	var err error
	maxRetries := 12
	delay := 12 * time.Second
	for i := 0; i < maxRetries; i++ {
		desc, err = ts.client.DescribeIndex(ctx, indexName)
		if err == nil {
			break
		}
		if status.Code(err) == codes.Unknown {
			fmt.Printf("Index \"%s\" not found, retrying... (%d/%d)\n", indexName, i+1, maxRetries)
			time.Sleep(delay)
		} else {
			fmt.Printf("Status code = %v\n", status.Code(err))
			return false, err
		}
	}
	if err != nil {
		return false, fmt.Errorf("failed to describe index \"%s\" after retries: %v", err, indexName)
	}
	return desc.Status.Ready, nil
}

func upsert(ts *IndexConnectionTestsIntegration, ctx context.Context, vectors []*Vector) error {
	maxRetries := 12
	delay := 12 * time.Second
	fmt.Printf("Attempting to upsert vectors into host \"%s\"...\n", ts.host)
	for i := 0; i < maxRetries; i++ {
		ready, err := getStatus(ts, ctx)
		if err != nil {
			fmt.Printf("Error getting index ready: %v\n", err)
			return err
		}
		if ready {
			upsertVectors, err := ts.idxConn.UpsertVectors(ctx, vectors)
			require.NoError(ts.T(), err)
			fmt.Printf("Upserted vectors: %v into host: %s\n", upsertVectors, ts.host)
			break
		} else {
			time.Sleep(delay)
			fmt.Printf("Host \"%s\" not ready for upserting yet, retrying... (%d/%d)\n", ts.host, i, maxRetries)
		}
	}
	return nil
}
