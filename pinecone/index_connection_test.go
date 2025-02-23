package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	db_data_grpc "github.com/pinecone-io/go-pinecone/v3/internal/gen/db_data/grpc"
	"github.com/pinecone-io/go-pinecone/v3/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests
func (ts *IntegrationTests) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IntegrationTests) TestQueryByVector() {
	vec := make([]float32, derefOrDefault(ts.dimension, 0))
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

func (ts *IntegrationTests) TestDeleteVectorsById() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsById(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	ts.vectorIds = []string{}

	vectors := GenerateVectors(5, derefOrDefault(ts.dimension, 0), true, nil)

	_, err = ts.idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}

	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}

	ts.vectorIds = append(ts.vectorIds, vectorIds...)
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
	} else {
		assert.NoError(ts.T(), err)
	}
	ts.vectorIds = []string{}

	vectors := GenerateVectors(5, derefOrDefault(ts.dimension, 0), true, nil)

	_, err = ts.idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}

	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}

	ts.vectorIds = append(ts.vectorIds, vectorIds...)
}

func (ts *IntegrationTests) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)
	ts.vectorIds = []string{}

	vectors := GenerateVectors(5, derefOrDefault(ts.dimension, 0), true, nil)

	_, err = ts.idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsById test. Error: %v", err)
	}

	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}

	ts.vectorIds = append(ts.vectorIds, vectorIds...)

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
	require.NotNil(ts.T(), idxConn.grpcClient, "Expected idxConn to have non-nil dataClient")
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

	if actualVals != nil {
		assert.ElementsMatch(ts.T(), expectedVals, *actualVals, "Values do not match")
	}
}

func (ts *IntegrationTests) TestUpdateVectorMetadata() {
	ctx := context.Background()

	expectedMetadata := map[string]interface{}{
		"genre": "death-metal",
	}
	expectedMetadataMap, err := structpb.NewStruct(expectedMetadata)
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to create metadata map: %v", err))
	}

	err = ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:       ts.vectorIds[0],
		Metadata: expectedMetadataMap,
	})
	assert.NoError(ts.T(), err)

	time.Sleep(10 * time.Second)

	vectors, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to fetch vector: %v", err))
	}
	vector := vectors.Vectors[ts.vectorIds[0]]

	if vector != nil {
		assert.NotNil(ts.T(), vector.Metadata, "Metadata is nil after update")

		expectedGenre := expectedMetadataMap.Fields["genre"].GetStringValue()
		actualGenre := vector.Metadata.Fields["genre"].GetStringValue()

		assert.Equal(ts.T(), expectedGenre, actualGenre, "Metadata does not match")
	}
}

func (ts *IntegrationTests) TestUpdateVectorSparseValues() {
	ctx := context.Background()

	dims := int32(derefOrDefault(ts.dimension, 0))
	indices := generateUint32Array(int(dims))
	vals := generateFloat32Array(int(dims))
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
	vectors, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to fetch vector: %v", err))
	}
	vector := vectors.Vectors[ts.vectorIds[0]]

	if vector != nil {
		actualSparseValues := vector.SparseValues.Values

		assert.ElementsMatch(ts.T(), expectedSparseValues.Values, actualSparseValues, "Sparse values do not match")
	}
}

func (ts *IntegrationTests) TestImportFlowHappyPath() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Skipping import flow test for non-serverless index")
	}

	testImportUri := "s3://dev-bulk-import-datasets-pub/10-records-dim-10/"
	ctx := context.Background()
	errorMode := "continue"

	startRes, err := ts.idxConn.StartImport(ctx, testImportUri, nil, (*ImportErrorMode)(&errorMode))
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), startRes)

	assert.NotNil(ts.T(), startRes.Id)
	describeRes, err := ts.idxConn.DescribeImport(ctx, startRes.Id)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), describeRes)
	assert.Equal(ts.T(), startRes.Id, describeRes.Id)

	limit := int32(10)
	listRes, err := ts.idxConn.ListImports(ctx, &limit, nil)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), listRes)

	err = ts.idxConn.CancelImport(ctx, startRes.Id)
	assert.NoError(ts.T(), err)
}

func (ts *IntegrationTests) TestImportFlowNoUriError() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Skipping import flow test for non-serverless index")
	}

	ctx := context.Background()
	_, err := ts.idxConn.StartImport(ctx, "", nil, nil)
	assert.Error(ts.T(), err)
	assert.Contains(ts.T(), err.Error(), "must specify a uri")
}

// Unit tests:
func TestUpdateVectorMissingReqdFieldsUnit(t *testing.T) {
	ctx := context.Background()
	idxConn := &IndexConnection{}
	err := idxConn.UpdateVector(ctx, &UpdateVectorRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a vector ID plus at least one of Values, SparseValues, or Metadata must be provided to update a vector")
}

func TestNewIndexConnection(t *testing.T) {
	apiKey := "test-api-key"
	host := "test-host.io"
	namespace := ""
	sourceTag := ""
	additionalMetadata := map[string]string{"api-key": apiKey}
	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               host,
		namespace:          namespace,
		sourceTag:          sourceTag})

	require.NoError(t, err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(t, ok, "Expected client to have an 'api-key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Empty(t, idxConn.Namespace, "Expected idxConn to have empty namespace, but got '%s'", idxConn.Namespace)
	require.NotNil(t, idxConn.grpcClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(t, idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")
}

func TestNewIndexConnectionNamespace(t *testing.T) {
	apiKey := "test-api-key"
	host := "test-host.io"
	namespace := "test-namespace"
	sourceTag := "test-source-tag"
	additionalMetadata := map[string]string{"api-key": apiKey}
	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               host,
		namespace:          namespace,
		sourceTag:          sourceTag})

	require.NoError(t, err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(t, ok, "Expected client to have an 'api-key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Equal(t, namespace, idxConn.Namespace, "Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.Namespace)
	require.NotNil(t, idxConn.grpcClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(t, idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")
}

func TestMarshalFetchVectorsResponseUnit(t *testing.T) {
	vec1Values := []float32{0.01, 0.01, 0.01}
	vec2Values := []float32{0.02, 0.02, 0.02}

	tests := []struct {
		name  string
		input FetchVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: FetchVectorsResponse{
				Vectors: map[string]*Vector{
					"vec-1": {Id: "vec-1", Values: &vec1Values},
					"vec-2": {Id: "vec-2", Values: &vec2Values},
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
	vec1Values := []float32{0.01, 0.01, 0.01}
	vec2Values := []float32{0.02, 0.02, 0.02}
	tests := []struct {
		name  string
		input QueryVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: QueryVectorsResponse{
				Matches: []*ScoredVector{
					{Vector: &Vector{Id: "vec-1", Values: &vec1Values}, Score: 0.1},
					{Vector: &Vector{Id: "vec-2", Values: &vec2Values}, Score: 0.2},
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
				Dimension:        uint32Pointer(3),
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
			want:  `{"dimension":null,"index_fullness":0,"total_vector_count":0}`,
		},
		{
			name: "Fields empty",
			input: DescribeIndexStatsResponse{
				Dimension:        uint32Pointer(0),
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
	vecValues := []float32{0.01, 0.02, 0.03}

	tests := []struct {
		name     string
		vector   *db_data_grpc.Vector
		expected *Vector
	}{
		{
			name:     "Pass nil vector, expect nil to be returned",
			vector:   nil,
			expected: nil,
		},
		{
			name: "Pass dense vector",
			vector: &db_data_grpc.Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
			expected: &Vector{
				Id:     "dense-1",
				Values: &vecValues,
			},
		},
		{
			name: "Pass sparse vector",
			vector: &db_data_grpc.Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &db_data_grpc.SparseValues{
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
			vector: &db_data_grpc.Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},

			expected: &Vector{
				Id:     "hybrid-1",
				Values: &vecValues,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector with metadata",
			vector: &db_data_grpc.Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
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
				Values: &vecValues,
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
		sparseValues *db_data_grpc.SparseValues
		expected     *SparseValues
	}{
		{
			name:         "Pass nil sparse values, expect nil to be returned",
			sparseValues: nil,
			expected:     nil,
		},
		{
			name: "Pass sparse values",
			sparseValues: &db_data_grpc.SparseValues{
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
	vecValues := []float32{0.01, 0.02, 0.03}

	tests := []struct {
		name         string
		scoredVector *db_data_grpc.ScoredVector
		expected     *ScoredVector
	}{
		{
			name:         "Pass nil scored vector, expect nil to be returned",
			scoredVector: nil,
			expected:     nil,
		},
		{
			name: "Pass scored dense vector",
			scoredVector: &db_data_grpc.ScoredVector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
				Score:  0.1,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "dense-1",
					Values: &vecValues,
				},
				Score: 0.1,
			},
		},
		{
			name: "Pass scored sparse vector",
			scoredVector: &db_data_grpc.ScoredVector{
				Id: "sparse-1",
				SparseValues: &db_data_grpc.SparseValues{
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
			scoredVector: &db_data_grpc.ScoredVector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Score: 0.3,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "hybrid-1",
					Values: &vecValues,
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
			scoredVector: &db_data_grpc.ScoredVector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
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
					Values: &vecValues,
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
	vecValues := []float32{0.01, 0.02, 0.03}

	tests := []struct {
		name     string
		vector   *Vector
		expected *db_data_grpc.Vector
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
				Values: &vecValues,
			},
			expected: &db_data_grpc.Vector{
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
			expected: &db_data_grpc.Vector{
				Id: "sparse-1",
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector",
			vector: &Vector{
				Id:     "hybrid-1",
				Values: &vecValues,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &db_data_grpc.Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector with metadata",
			vector: &Vector{
				Id:     "hybrid-metadata-1",
				Values: &vecValues,
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
			expected: &db_data_grpc.Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
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
		expected     *db_data_grpc.SparseValues
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
			expected: &db_data_grpc.SparseValues{
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
			expected: &db_data_grpc.SparseValues{
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
		usage    *db_data_grpc.Usage
		expected *Usage
	}{
		{
			name:     "Pass nil usage, expect nil to be returned",
			usage:    nil,
			expected: nil,
		},
		{
			name: "Pass usage",
			usage: &db_data_grpc.Usage{
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

func TestNormalizeHostUnit(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		expectedHost     string
		expectedIsSecure bool
	}{
		{
			name:             "https:// scheme should be removed",
			host:             "https://this-is-my-host.io",
			expectedHost:     "this-is-my-host.io",
			expectedIsSecure: true,
		}, {
			name:             "https:// scheme should be removed",
			host:             "https://this-is-my-host.io:33445",
			expectedHost:     "this-is-my-host.io:33445",
			expectedIsSecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, isSecure := normalizeHost(tt.host)
			assert.Equal(t, tt.expectedHost, result, "Expected result to be '%s', but got '%s'", tt.expectedHost, result)
			assert.Equal(t, tt.expectedIsSecure, isSecure, "Expected isSecure to be '%t', but got '%t'", tt.expectedIsSecure, isSecure)
		})
	}
}

func TestToPaginationTokenGrpc(t *testing.T) {
	tokenForNilCase := ""
	tokenForPositiveCase := "next-token"

	tests := []struct {
		name     string
		token    *db_data_grpc.Pagination
		expected *string
	}{
		{
			name:     "Pass empty token, expect empty string to be returned",
			token:    &db_data_grpc.Pagination{},
			expected: &tokenForNilCase,
		},
		{
			name: "Pass token",
			token: &db_data_grpc.Pagination{
				Next: "next-token",
			},
			expected: &tokenForPositiveCase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toPaginationTokenGrpc(tt.token)
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

func uint32Pointer(i uint32) *uint32 {
	return &i
}
