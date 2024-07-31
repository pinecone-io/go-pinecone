package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/pinecone-io/go-pinecone/internal/gen/data"
	"github.com/pinecone-io/go-pinecone/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests
func (ts *IntegrationTests) TestNewIndexConnection() {
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

func (ts *IntegrationTests) TestNewIndexConnectionNamespace() {
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

func (ts *IntegrationTests) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestFetchVectorsSourceTag() {
	ctx := context.Background()
	res, err := ts.idxConnSourceTag.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestQueryByVector() {
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

func (ts *IntegrationTests) TestQueryByVectorSourceTag() {
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

func (ts *IntegrationTests) TestQueryById() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestQueryByIdSourceTag() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConnSourceTag.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestDeleteVectorsById() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsById(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)

	_, err = ts.idxConn.UpsertVectors(ctx, createVectorsForUpsert())
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}
}

func (ts *IntegrationTests) TestDeleteVectorsByFilter() {
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

func (ts *IntegrationTests) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)

	_, err = ts.idxConn.UpsertVectors(ctx, createVectorsForUpsert())
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}

}

func (ts *IntegrationTests) TestDescribeIndexStats() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStats(ctx)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestDescribeIndexStatsFiltered() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStatsFiltered(ctx, &MetadataFilter{})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestUpsertVectorsMissingReqdFields() {
	ctx := context.Background()
	vectors := []*Vector{
		{},
	}
	_, err := ts.idxConn.UpsertVectors(ctx, vectors)
	assert.Error(ts.T(), err)
	assert.Contains(ts.T(), err.Error(), "vectors must have at least ID and Values fields in order to be upserted")
}

func (ts *IntegrationTests) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{}

	ctx := context.Background()
	res, err := ts.idxConn.ListVectors(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestMetadataAppliedToRequests() {
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

func (ts *IntegrationTests) TestUpdateVectorValues() {
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

func (ts *IntegrationTests) TestUpdateVectorMissingReqdFields() {
	ctx := context.Background()
	err := ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{})
	assert.Error(ts.T(), err)
	assert.Contains(ts.T(), err.Error(), "a vector ID plus at least one of Values, SparseValues, or Metadata must be provided to update a vector")
}

func (ts *IntegrationTests) TestUpdateVectorMetadata() {
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

func (ts *IntegrationTests) TestUpdateVectorSparseValues() error {
	ctx := context.Background()

	dims := int(ts.dimension)
	indices := generateUint32Array(dims)
	vals := generateFloat32Array(dims)
	expectedSparseValues := SparseValues{
		Indices: indices,
		Values:  vals,
	}

	fmt.Printf("Updating sparse values in host \"%s\"...\n", ts.host)
	err := ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:           ts.vectorIds[0],
		SparseValues: &expectedSparseValues,
	})
	require.NoError(ts.T(), err)

	// Wait for updates to propagate
	time.Sleep(5 * time.Second)

	// Fetch updated vector and verify sparse values
	vector, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to fetch vector: %v", err))
	}
	actualSparseValues := vector.Vectors[ts.vectorIds[0]].SparseValues.Values

	assert.ElementsMatch(ts.T(), expectedSparseValues.Values, actualSparseValues, "Sparse values do not match")

	return nil
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
				Usage:     &Usage{ReadUnits: 5},
				Namespace: "test-namespace",
			},
			want: `{"vectors":{"vec-1":{"id":"vec-1","values":[0.01,0.01,0.01]},"vec-2":{"id":"vec-2","values":[0.02,0.02,0.02]}},"usage":{"read_units":5},"namespace":"test-namespace"}`,
		},
		{
			name:  "Fields omitted",
			input: FetchVectorsResponse{},
			want:  `{"namespace":""}`,
		},
		{
			name: "Fields empty",
			input: FetchVectorsResponse{
				Vectors:   nil,
				Usage:     nil,
				Namespace: "",
			},
			want: `{"namespace":""}`,
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
				Namespace:           "test-namespace",
			},
			want: `{"vector_ids":["vec-1","vec-2"],"usage":{"read_units":5},"next_pagination_token":"next-token","namespace":"test-namespace"}`,
		},
		{
			name:  "Fields omitted",
			input: ListVectorsResponse{},
			want:  `{"namespace":""}`,
		},
		{
			name: "Fields empty",
			input: ListVectorsResponse{
				VectorIds:           nil,
				Usage:               nil,
				NextPaginationToken: nil,
				Namespace:           "",
			},
			want: `{"namespace":""}`,
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
				Usage:     &Usage{ReadUnits: 5},
				Namespace: "test-namespace",
			},
			want: `{"matches":[{"vector":{"id":"vec-1","values":[0.01,0.01,0.01]},"score":0.1},{"vector":{"id":"vec-2","values":[0.02,0.02,0.02]},"score":0.2}],"usage":{"read_units":5},"namespace":"test-namespace"}`,
		},
		{
			name:  "Fields omitted",
			input: QueryVectorsResponse{},
			want:  `{"namespace":""}`,
		},
		{
			name:  "Fields empty",
			input: QueryVectorsResponse{Matches: nil, Usage: nil},
			want:  `{"namespace":""}`,
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

// Helper funcs
func generateFloat32Array(n int) []float32 {
	array := make([]float32, n)
	for i := 0; i < n; i++ {
		array[i] = float32(i)
	}
	return array
}

func generateUint32Array(n int) []uint32 {
	array := make([]uint32, n)
	for i := 0; i < n; i++ {
		array[i] = uint32(i)
	}
	return array
}
