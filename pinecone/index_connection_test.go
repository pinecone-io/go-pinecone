package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"

	db_data_grpc "github.com/pinecone-io/go-pinecone/v4/internal/gen/db_data/grpc"
	"github.com/pinecone-io/go-pinecone/v4/internal/utils"
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
		Vector:          vec,
		TopK:            5,
		IncludeValues:   true,
		IncludeMetadata: true,
	}

	retryAssertionsWithDefaults(ts.T(), func() error {
		ctx := context.Background()
		res, err := ts.idxConn.QueryByVectorValues(ctx, req)
		if err != nil {
			return fmt.Errorf("QueryByVectorValues failed: %v", err)
		}
		if res == nil {
			return fmt.Errorf("QueryByVectorValues response is nil")
		}

		assert.NoError(ts.T(), err)
		assert.NotNil(ts.T(), res)
		return nil
	})
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

	vectors := generateVectors(5, derefOrDefault(ts.dimension, 0), false, nil)

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
	_ = ts.idxConn.DeleteVectorsByFilter(ctx, filter)

	ts.vectorIds = []string{}

	vectors := generateVectors(5, derefOrDefault(ts.dimension, 0), false, nil)

	_, err = ts.idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteVectorsByFilter test. Error: %v", err)
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

	vectors := generateVectors(5, derefOrDefault(ts.dimension, 0), false, nil)

	_, err = ts.idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in TestDeleteAllVectorsInNamespace test. Error: %v", err)
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
	require.Equal(ts.T(), namespace, idxConn.namespace, "Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.namespace)
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

	retryAssertionsWithDefaults(ts.T(), func() error {
		vector, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
		if err != nil {
			return fmt.Errorf("Failed to fetch vector: %v", err)
		}

		if len(vector.Vectors) > 0 {
			actualVals := vector.Vectors[ts.vectorIds[0]].Values
			if actualVals != nil {
				if !slicesEqual[float32](expectedVals, *actualVals) {
					return fmt.Errorf("Values do not match")
				} else {
					return nil // Test passed
				}
			} else {
				return fmt.Errorf("Values are nil after UpdateVector->FetchVector")
			}
		} else {
			return fmt.Errorf("No vectors found after UpdateVector->FetchVector")
		}
	})
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

	retryAssertionsWithDefaults(ts.T(), func() error {
		vectors, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
		if err != nil {
			return fmt.Errorf("Failed to fetch vector: %v", err)
		}

		if vectors != nil && len(vectors.Vectors) > 0 {
			vector := vectors.Vectors[ts.vectorIds[0]]
			if vector == nil {
				return fmt.Errorf("Fetched vector is nil after UpdateVector->FetchVector")
			}
			if vector.Metadata == nil {
				return fmt.Errorf("Metadata is nil after update")
			}

			expectedGenre := expectedMetadataMap.Fields["genre"].GetStringValue()
			actualGenre := vector.Metadata.Fields["genre"].GetStringValue()

			if expectedGenre != actualGenre {
				return fmt.Errorf("Metadata does not match")
			}
		} else {
			return fmt.Errorf("No vectors found after update")
		}
		return nil // Test passed
	})
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

	// Fetch updated vector and verify sparse values
	retryAssertionsWithDefaults(ts.T(), func() error {
		vectors, err := ts.idxConn.FetchVectors(ctx, []string{ts.vectorIds[0]})
		if err != nil {
			return fmt.Errorf("Failed to fetch vector: %v", err)
		}

		vector := vectors.Vectors[ts.vectorIds[0]]

		if vector == nil {
			return fmt.Errorf("Fetched vector is nil after UpdateVector->FetchVector")
		}
		if vector.SparseValues == nil {
			return fmt.Errorf("Sparse values are nil after UpdateVector->FetchVector")
		}
		actualSparseValues := vector.SparseValues.Values

		if !slicesEqual[float32](expectedSparseValues.Values, actualSparseValues) {
			return fmt.Errorf("Sparse values do not match")
		}
		return nil // Test passed
	})
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

func (ts *IntegrationTests) TestIntegratedInference() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Running TestIntegratedInference once")
	}
	indexName := "test-integrated-" + generateTestIndexName()

	// create integrated index
	ctx := context.Background()
	index, err := ts.client.CreateIndexForModel(ctx, &CreateIndexForModelRequest{
		Name:   indexName,
		Cloud:  "aws",
		Region: "us-east-1",
		Embed: CreateIndexForModelEmbed{
			Model:    "multilingual-e5-large",
			FieldMap: map[string]interface{}{"text": "chunk_text"},
		},
	})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), index)

	defer func(ts *IntegrationTests, name string) {
		err := ts.deleteIndex(name)

		require.NoError(ts.T(), err)
	}(ts, indexName)

	// upsert records/documents
	records := []*IntegratedRecord{
		{
			"_id":        "rec1",
			"chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
			"category":   "product",
		},
		{
			"_id":        "rec2",
			"chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
			"category":   "nutrition",
		},
		{
			"_id":        "rec3",
			"chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
			"category":   "cultivation",
		},
		{
			"_id":        "rec4",
			"chunk_text": "In 2001, Apple released the iPod, which transformed the music industry by making portable music widely accessible.",
			"category":   "product",
		},
		{
			"_id":        "rec5",
			"chunk_text": "Apple went public in 1980, making history with one of the largest IPOs at that time.",
			"category":   "milestone",
		},
		{
			"_id":        "rec6",
			"chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
			"category":   "nutrition",
		},
		{
			"_id":        "rec7",
			"chunk_text": "Known for its design-forward products, Apple's branding and market strategy have greatly influenced the technology sector and popularized minimalist design worldwide.",
			"category":   "influence",
		},
		{
			"_id":        "rec8",
			"chunk_text": "The high fiber content in apples can also help regulate blood sugar levels, making them a favorable snack for people with diabetes.",
			"category":   "nutrition",
		},
	}
	err = ts.idxConn.UpsertRecords(ctx, records)
	assert.NoError(ts.T(), err)

	retryAssertionsWithDefaults(ts.T(), func() error {
		res, err := ts.idxConn.SearchRecords(ctx, &SearchRecordsRequest{
			Query: SearchRecordsQuery{
				TopK: 5,
				Inputs: &map[string]interface{}{
					"text": "Disease prevention",
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to search records: %v", err)
		}
		if res == nil {
			return fmt.Errorf("result is nil")
		}
		return nil
	})
}

func (ts *IntegrationTests) TestDescribeNamespace() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Namespace operations are only supported in serverless indexes")
	}

	ctx := context.Background()
	namespace := ts.namespaces[0]

	namespaceDesc, err := ts.idxConn.DescribeNamespace(ctx, namespace)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), namespaceDesc, "Namespace description should not be nil")
	require.Equal(ts.T(), namespace, namespaceDesc.Name, "Namespace name should match the requested namespace")
}

func (ts *IntegrationTests) TestListNamespaces() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Namespace operations are only supported in serverless indexes")
	}

	// List one namespace with limit
	limit := uint32(1)
	ctx := context.Background()
	namespaces, err := ts.idxConn.ListNamespaces(ctx, &ListNamespacesParams{
		Limit: &limit,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), namespaces, "ListNamespaces response should not be nil")
	require.Equal(ts.T(), limit, uint32(len(namespaces.Namespaces)))

	// List remaining
	namespaces, err = ts.idxConn.ListNamespaces(ctx, &ListNamespacesParams{
		PaginationToken: &namespaces.Pagination.Next,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), namespaces, "ListNamespaces response should not be nil")
	require.Greater(ts.T(), len(namespaces.Namespaces), 0, "ListNamespaces should return the second page of results")
}

func (ts *IntegrationTests) TestDeleteNamespace() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Namespace operations are only supported in serverless indexes")
	}

	deletedNamespace := ts.namespaces[len(ts.namespaces)-1]
	ctx := context.Background()
	err := ts.idxConn.DeleteNamespace(ctx, deletedNamespace)
	require.NoError(ts.T(), err, "DeleteNamespace should not return an error")

	// Verify the namespace is deleted, which may take some time
	retryAssertionsWithDefaults(ts.T(), func() error {
		namespaces, err := ts.idxConn.ListNamespaces(ctx, nil)
		if err != nil {
			return fmt.Errorf("ListNamespaces failed: %v", err)
		}
		for _, ns := range namespaces.Namespaces {
			if ns.Name == deletedNamespace {
				return fmt.Errorf("Namespace %s was not deleted", deletedNamespace)
			}
		}
		return nil // Namespace successfully deleted
	})
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
	require.Empty(t, idxConn.namespace, "Expected idxConn to have empty namespace, but got '%s'", idxConn.namespace)
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
	require.Equal(t, namespace, idxConn.namespace, "Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.namespace)
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

func slicesEqual[T comparable](a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
