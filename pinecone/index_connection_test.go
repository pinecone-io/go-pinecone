package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"

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
	host             string
	dimension        int32
	apiKey           string
	indexType        string
	idxConn          *IndexConnection
	sourceTag        string
	idxConnSourceTag *IndexConnection
	vectorIds        []string
}

func TestIntegrationIndexConnection(t *testing.T) {
	apiKey := os.Getenv("PINECONE_API_KEY")
	assert.NotEmptyf(t, apiKey, "PINECONE_API_KEY env variable not set")

	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: map[string]string{"content-type": "application/json"}})
	if err != nil {
		t.FailNow()
	}
	fmt.Printf("Headers: %+v\n", client.headers)

	serverlessIdx := buildServerlessTestIndex(t, client)
	podIdx := buildPodTestIndex(t, client)

	// TODO: make it so that the test indexes are deleted before running the tests,
	//  but for now this is a workaround
	if serverlessIdx == nil || podIdx == nil {
		log.Fatalf("Delete test indexes and rerun testing suite")
	}

	podTestSuite := &IndexConnectionTestsIntegration{
		host:      podIdx.Host,
		dimension: podIdx.Dimension,
		apiKey:    apiKey,
		indexType: "pod",
	}

	serverlessTestSuite := &IndexConnectionTestsIntegration{
		host:      serverlessIdx.Host,
		dimension: serverlessIdx.Dimension,
		apiKey:    apiKey,
		indexType: "serverless",
	}
	fmt.Printf("pods ts.host : %v\n", podTestSuite.host)
	fmt.Printf("serverless ts.host : %v\n", serverlessTestSuite.host)

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}

func (ts *IndexConnectionTestsIntegration) SetupSuite() {
	fmt.Printf("Host in setup suite: %v\n", ts.host)
	ctx := context.Background()

	assert.NotEmptyf(ts.T(), ts.host, "HOST env variable not set")
	assert.NotEmptyf(ts.T(), ts.apiKey, "API_KEY env variable not set")
	additionalMetadata := map[string]string{"api-key": ts.apiKey, "content-type": "application/json"}

	namespace, err := uuid.NewV7()
	assert.NoError(ts.T(), err)

	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ""})
	assert.NoError(ts.T(), err)
	ts.idxConn = idxConn

	vectors := setVectorsForUpsert()
	//for _, vector := range vectors {
	//	fmt.Printf("%+v\n", vector)
	//}
	//fmt.Printf("Dimension of index: %v\n", ts.dimension)
	fmt.Printf("host: %+v\n", ts.host)
	upsertVectors, err := idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors: %v", err)
		return
	}
	fmt.Printf("Upserted vectors: %v into host: %s\n", upsertVectors, ts.host)
	//podTestSuite.idxConn.UpsertVectors(context.Background(), vectors)

	ts.sourceTag = "test_source_tag"
	idxConnSourceTag, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ts.sourceTag})
	assert.NoError(ts.T(), err)
	ts.idxConnSourceTag = idxConnSourceTag

	//ts.loadData()
	fmt.Println("\nSetupSuite completed successfully")

}

func (ts *IndexConnectionTestsIntegration) TearDownSuite() {
	//ts.truncateData()

	err := ts.idxConn.Close()
	assert.NoError(ts.T(), err)

	err = ts.idxConnSourceTag.Close()
	assert.NoError(ts.T(), err)
	fmt.Println("\nSetup suite torn down successfullly")
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

	//ts.loadData() //reload deleted data
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

	//ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTestsIntegration) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)

	//ts.loadData() //reload deleted data
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
	dims := int(ts.dimension)

	podsConn := ts.idxConn
	fmt.Printf("\nPodsConn... %+v", podsConn)
	fmt.Printf("\nidxConn... %+v", ts.idxConn)

	idxStats, _ := ts.idxConn.DescribeIndexStats(ctx)
	//serverlessConn := *ts.idxConn
	fmt.Printf("\nIndex stats: %+v", idxStats)
	//fmt.Printf("\nserverlessConn Namespaces... %+v", serverlessConn.Namespace)

	//idx, _ := ts.idxConn.DescribeIndexStats(ctx)
	//fmt.Printf("\nidxConn... global? %+v", idx.Namespaces)

	fmt.Printf("\nHost: %s", ts.host)

	err := podsConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:     ts.vectorIds[0],
		Values: generateFloat32Array(dims),
	})
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTestsIntegration) TestUpdateVectorMetadata() {
	ctx := context.Background()

	metadataMap := map[string]interface{}{
		"genre": "classical",
	}

	metadata, err := structpb.NewStruct(metadataMap)

	err = ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id:       ts.vectorIds[0],
		Metadata: metadata,
	})
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTestsIntegration) TestUpdateVectorSparseValues() {
	ctx := context.Background()
	dims := int(ts.dimension)
	generatedSparseIndices := generateUint32Array(dims)
	generatedSparseValues := generateFloat32Array(dims)

	err := ts.idxConn.UpdateVector(ctx, &UpdateVectorRequest{
		Id: ts.vectorIds[0],
		SparseValues: &SparseValues{
			Indices: generatedSparseIndices,
			Values:  generatedSparseValues,
		},
	})
	assert.NoError(ts.T(), err)

	fmt.Printf("Vector ID is: %v\n", ts.vectorIds[0])

	vector, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
	fmt.Printf("Ignore me %v", &vector)
	//fmt.Printf("Vector sparse valuse: %v\n", vector.Vectors[ts.vectorIds[0]].SparseValues.Values)
	//fmt.Printf("Generated sparse values: %v\n", generatedSparseValues)
	//assert.Equal(ts.T(), vector.Vectors[ts.vectorIds[0]].SparseValues.Values, generatedValues)
}

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

func setIdsForUpsert() []string {
	return []string{"vec-1", "vec-2", "vec-3", "vec-4", "vec-5"}
}

func setVectorsForUpsert() []*Vector {
	//values := []float32{0.01, 0.02, 0.03, 0.04, 0.05}
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

func listAndDeleteIndexes(in *Client, context context.Context, indexNameToDelete string) bool {
	var deletedIndex bool

	indexes, err := in.ListIndexes(context)
	if err != nil {
		log.Fatalf("Failed to list indexes in Integration Tests: %v", err)
	}

	for _, v := range indexes {
		if v.Name == indexNameToDelete {
			err := in.DeleteIndex(context, v.Name)
			if err != nil {
				log.Fatalf("Failed to delete Pod index in Integration Tests: %v", err)
			}
			deletedIndex = true
		}
	}
	return deletedIndex

}

func buildServerlessTestIndex(t *testing.T, in *Client) *Index {
	ctx := context.Background()

	serverlessIndexName := os.Getenv("TEST_SERVERLESS_INDEX_NAME")
	assert.NotEmptyf(t, serverlessIndexName, "TEST_SERVERLESS_INDEX_NAME env variable not set")

	needToDeleteIndex := listAndDeleteIndexes(in, ctx, serverlessIndexName)
	if !needToDeleteIndex {
		serverlessIdx, err := in.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
			Name:      serverlessIndexName,
			Dimension: int32(setDimensionsForTestIndexes()),
			Metric:    Cosine,
			Region:    "us-east-1",
			Cloud:     "aws",
		})
		if err != nil {
			log.Fatalf("Failed to create Serverless index in Integration Tests: %v", err)
		} else {
			fmt.Printf("Successfully created a new Serverless index: %s!\n", serverlessIndexName)
		}

		return serverlessIdx
	} // TODO: when index does NOT exist, get this error: "malformed header: missing HTTP content-type"
	// TODO: but now when I add it in TestIntegrationIndexConnection under client, err (ln40), it says rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: read tcp 192.168.7.21:49936->52.6.114.50:443: read: connection reset by peer"
	// TODO: worked for pods, but not for serverless

	fmt.Printf("Index %s already exists. Delete!\n", serverlessIndexName)
	return nil
	//idxExists := &Index{
	//	Name:      serverlessIndexName,
	//	Dimension: int32(setDimensionsForTestIndexes()),
	//}
	//fmt.Printf("Index obj: %+v\n", idxExists)
	//return idxExists // TODO: when index DOES exist, host is blank and that seems to be fucking stuff up
}

func buildPodTestIndex(t *testing.T, in *Client) *Index {
	ctx := context.Background()

	podIndexName := os.Getenv("TEST_POD_INDEX_NAME")
	assert.NotEmptyf(t, podIndexName, "TEST_POD_INDEX_NAME env variable not set")

	needToDeleteIndex := listAndDeleteIndexes(in, ctx, podIndexName)

	if !needToDeleteIndex {
		podIdx, err := in.CreatePodIndex(ctx, &CreatePodIndexRequest{
			Name:        podIndexName,
			Dimension:   int32(setDimensionsForTestIndexes()),
			Metric:      Cosine,
			Environment: "us-east-1-aws",
			PodType:     "p1",
		})
		if err != nil {
			log.Fatalf("Failed to create Pod index in Integration Tests: %v", err)
		} else {
			fmt.Printf("Successfully created a new Pod index: %s!\n", podIndexName)
		}

		return podIdx
	}

	fmt.Printf("Index %s already exists. Delete!\n", podIndexName)
	//return
	//idxExists, _ := in.restClient.ListIndexes(ctx)
	//idxs, err := in.restClient.ListIndexes(ctx)
	//var idxToReturn Index
	//for _, idx := range idxs.Body. {
	//	fmt.Printf("- \"%s\"\n", idx.Name)
	//	if idx == podIndexName {
	//		idxToReturn = idx
	//	}
	//}

	//fmt.Printf("Index obj: %+v\n", idxExists)
	//return idxExists
	return nil
}

func (ts *IndexConnectionTestsIntegration) loadData(indexName string) {

	//vals := []float32{0.01, 0.02, 0.03, 0.04, 0.05}
	//vectors := writeVectorsForUpsert()
	//ts.vectorIds = writeIdsForUpsert()
	//
	//for i, v := range vectors {
	//	vec := make([]float32, ts.dimension)
	//	for i := range vec {
	//		vec[i] = val
	//	}
	//
	//	id := fmt.Sprintf("vec-%d", i+1)
	//	ts.vectorIds[i] = id
	//
	//	vectors[i] = &Vector{
	//		Id:     id,
	//		Values: vec,
	//	}
	//}

	//ctx := context.Background()
	//_, err := ts.idxConn.UpsertVectors(ctx, vectors)
	//assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTestsIntegration) loadDataSourceTag() {
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
