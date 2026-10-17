//go:build integration

package pinecone

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pinecone-io/go-pinecone/v7/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (ts *integrationTests) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *integrationTests) TestQueryByVector() {
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

func (ts *integrationTests) TestQueryById() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *integrationTests) TestDeleteVectorsById() {
	ctx := context.Background()

	// Use second namespace to isolate deletions
	idxConn := ts.idxConn.WithNamespace(ts.namespaces[1])
	vecIds := ts.vectorIds[len(ts.vectorIds)-2:]

	err := idxConn.DeleteVectorsById(ctx, vecIds)
	assert.NoError(ts.T(), err)

	// validate the vectors were deleted
	retryAssertionsWithDefaults(ts.T(), func() error {
		res, err := idxConn.FetchVectors(ctx, vecIds)
		if err != nil {
			return fmt.Errorf("Failed to fetch vectors: %v", err)
		}
		if len(res.Vectors) > 0 {
			return fmt.Errorf("Vectors were not deleted")
		}
		return nil
	})
}

func (ts *integrationTests) TestDeleteVectorsByFilter() {
	metadataFilter := map[string]interface{}{
		"genre": "classical",
	}
	filter, err := structpb.NewStruct(metadataFilter)
	if err != nil {
		ts.FailNow(fmt.Sprintf("Failed to create metadata filter: %v", err))
	}

	ctx := context.Background()

	idxConn := ts.idxConn.WithNamespace(ts.namespaces[1])
	err = idxConn.DeleteVectorsByFilter(ctx, filter)
	assert.NoError(ts.T(), err)

	// validate the vectors were deleted if not pod index. Delete-by-filter reconciles
	// asynchronously and can take longer than the default window to converge on
	// serverless, so use the extended retry budget to avoid spurious failures.
	if ts.indexType != "pods" {
		retryAssertionsWithExtendedTimeout(ts.T(), func() error {
			res, err := idxConn.FetchVectorsByMetadata(ctx, &FetchVectorsByMetadataRequest{
				Filter: filter,
			})
			if err != nil {
				return fmt.Errorf("Failed to fetch vectors: %v", err)
			}
			if len(res.Vectors) > 0 {
				return fmt.Errorf("Vectors were not deleted")
			}
			return nil
		})
	}
}

func (ts *integrationTests) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	idxConn := ts.idxConn.WithNamespace(ts.namespaces[2])
	err := idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)

	// validate the namespace is empty
	retryAssertionsWithDefaults(ts.T(), func() error {
		res, err := idxConn.FetchVectors(ctx, ts.vectorIds)
		if err != nil {
			return fmt.Errorf("Failed to list vectors: %v", err)
		}
		assert.NotNil(ts.T(), res)
		if len(res.Vectors) > 0 {
			return fmt.Errorf("Vectors were not deleted")
		}
		return nil
	})
}

func (ts *integrationTests) TestDescribeIndexStats() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStats(ctx)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

// MetadataFilter on this operation isPods-based indexes only.
// This primarily just validates passing the MetadataFilter to the method.
func (ts *integrationTests) TestDescribeIndexStatsFiltered() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStatsFiltered(ctx, &MetadataFilter{})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *integrationTests) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{}

	ctx := context.Background()
	res, err := ts.idxConn.ListVectors(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *integrationTests) TestMetadataAppliedToRequests() {
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

func (ts *integrationTests) TestUpdateVectorValues() {
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

func (ts *integrationTests) TestUpdateVectorMetadata() {
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

func (ts *integrationTests) TestUpdateVectorSparseValues() {
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

func (ts *integrationTests) TestFetchVectorsByMetadata() {
	if ts.indexType == "pods" {
		ts.T().Skip("Skipping fetch vectors by metadata test for pod index")
	}

	ctx := context.Background()

	// Create filter to match vectors with genre="classical"
	filterMap := map[string]interface{}{
		"genre": map[string]interface{}{
			"$eq": "classical",
		},
	}
	filter, err := NewMetadataFilter(filterMap)
	assert.NoError(ts.T(), err)

	limit := uint32(10)

	// Use pre-existing vectors with classical metadata from SetupSuite
	retryAssertionsWithDefaults(ts.T(), func() error {
		res, err := ts.idxConn.FetchVectorsByMetadata(ctx, &FetchVectorsByMetadataRequest{
			Filter: filter,
			Limit:  &limit,
		})
		assert.NoError(ts.T(), err, "Failed to fetch vectors by metadata")
		assert.NotNil(ts.T(), res, "FetchVectorsByMetadata response is nil")

		// Verify response structure
		assert.NotNil(ts.T(), res.Vectors, "Vectors map is nil")
		assert.Greater(ts.T(), len(res.Vectors), 0, "No vectors found matching filter")

		// Verify all returned vectors have the expected metadata
		for id, vector := range res.Vectors {
			assert.NotNil(ts.T(), vector, "Vector %s is nil", id)
			assert.NotNil(ts.T(), vector.Metadata, "Vector %s has nil metadata", id)

			genreField, ok := vector.Metadata.Fields["genre"]
			assert.True(ts.T(), ok, "Vector %s missing genre field", id)

			genre := genreField.GetStringValue()
			assert.Equal(ts.T(), "classical", genre, "Vector %s has incorrect genre", id)
		}

		// Verify namespace is set
		assert.NotEmpty(ts.T(), res.Namespace, "Namespace is empty")
		assert.NotNil(ts.T(), res.Usage, "Usage is nil")

		return nil
	})
}

func (ts *integrationTests) TestUpdateVectorsByMetadata() {
	if ts.indexType == "pods" {
		ts.T().Skip("Skipping update vectors by metadata test for non-serverless index")
	}

	ctx := context.Background()

	// Use pre-existing vectors with rock metadata from SetupSuite
	filterMap := map[string]interface{}{
		"genre": map[string]interface{}{
			"$eq": "rock",
		},
	}
	filter, err := NewMetadataFilter(filterMap)
	require.NoError(ts.T(), err)

	// Create metadata for DryRun test (required by validation)
	dryRunMetadataMap := map[string]interface{}{
		"genre": "rock",
		"year":  2021,
	}
	dryRunMetadata, err := NewMetadata(dryRunMetadataMap)
	require.NoError(ts.T(), err)

	// DryRun with Filter
	dryRun := true
	dryRunRes, err := ts.idxConn.UpdateVectorsByMetadata(ctx, &UpdateVectorsByMetadataRequest{
		Filter:   filter,
		Metadata: dryRunMetadata,
		DryRun:   &dryRun,
	})

	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), dryRunRes)
	assert.Greater(ts.T(), dryRunRes.MatchedRecords, int32(0), "DryRun should return matched records count")

	// Update metadata with Filter
	updateMetadataMap := map[string]interface{}{
		"genre":   "rock",
		"year":    2021,
		"updated": true,
	}
	updateMetadata, err := NewMetadata(updateMetadataMap)
	require.NoError(ts.T(), err)

	dryRun = false
	updateRes, err := ts.idxConn.UpdateVectorsByMetadata(ctx, &UpdateVectorsByMetadataRequest{
		Filter:   filter,
		Metadata: updateMetadata,
		DryRun:   &dryRun,
	})

	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), updateRes)
	assert.Greater(ts.T(), updateRes.MatchedRecords, int32(0), "Update should return matched records count")

	// Verify the update worked by fetching vectors and checking their metadata
	retryAssertionsWithDefaults(ts.T(), func() error {
		// Fetch the vectors that should have been updated
		fetched, err := ts.idxConn.FetchVectors(ctx, ts.vectorsWithRockMetadata)
		if err != nil {
			return fmt.Errorf("Failed to fetch vectors: %v", err)
		}

		if len(fetched.Vectors) == 0 {
			return fmt.Errorf("No vectors found")
		}

		// Verify all vectors have the updated metadata
		for id, vector := range fetched.Vectors {
			assert.NotNil(ts.T(), vector, "Vector %s is nil", id)
			assert.NotNil(ts.T(), vector.Metadata, "Vector %s has nil metadata", id)

			// Check that the updated field is present
			updatedField, ok := vector.Metadata.Fields["updated"]
			if !ok {
				return fmt.Errorf("Vector did not contain updated field: %s", id)
			}
			if ok {
				updated := updatedField.GetBoolValue()
				assert.True(ts.T(), updated, "Vector %s updated field should be true", id)
			}

			// Verify genre is still rock
			genreField, ok := vector.Metadata.Fields["genre"]
			assert.True(ts.T(), ok, "Vector %s missing genre field", id)
			if ok {
				genre := genreField.GetStringValue()
				assert.Equal(ts.T(), "rock", genre, "Vector %s genre should still be rock", id)
			}
		}

		return nil
	})
}

func (ts *integrationTests) TestImportFlowHappyPath() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Skipping import flow test for non-serverless index")
	}

	testImportUri := "s3://dev-bulk-import-datasets-pub/10-records-dim-10/"
	ctx := context.Background()
	errorMode := "continue"

	startRes, err := ts.idxConn.StartImport(ctx, testImportUri, nil, &errorMode)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), startRes)

	assert.NotNil(ts.T(), startRes.Id)
	describeRes, err := ts.idxConn.DescribeImport(ctx, startRes.Id)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), describeRes)
	assert.Equal(ts.T(), startRes.Id, describeRes.Id)

	limit := int32(10)
	listRes, err := ts.idxConn.ListImports(ctx, &ListImportsRequest{Limit: &limit})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), listRes)

	err = ts.idxConn.CancelImport(ctx, startRes.Id)
	assert.NoError(ts.T(), err)
}

func (ts *integrationTests) TestImportFlowNoUriError() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Skipping import flow test for non-serverless index")
	}

	ctx := context.Background()
	_, err := ts.idxConn.StartImport(ctx, "", nil, nil)
	assert.Error(ts.T(), err)
	assert.Contains(ts.T(), err.Error(), "must specify a uri")
}

func (ts *integrationTests) TestIntegratedInference() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Running TestIntegratedInference once")
	}
	indexName := "test-integrated-" + generateTestIndexName()

	// create integrated index with Schema
	ctx := context.Background()
	testSchema := &MetadataSchema{
		Fields: map[string]MetadataSchemaField{
			"category": {Filterable: true},
		},
	}
	index, err := ts.client.CreateIndexForModel(ctx, &CreateIndexForModelRequest{
		Name:   indexName,
		Cloud:  "aws",
		Region: "us-east-1",
		Embed: CreateIndexForModelEmbed{
			Model:    "multilingual-e5-large",
			FieldMap: map[string]interface{}{"text": "chunk_text"},
		},
		Schema: testSchema,
	})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), index)

	// Wait for the index to be fully ready before attempting data plane operations
	_, err = waitUntilIndexReady(ts, ctx, indexName)
	require.NoError(ts.T(), err)

	// Verify the typed schema is returned when describing the index: the declared metadata
	// field is reported as a schema field under 2026-07.
	retryAssertionsWithDefaults(ts.T(), func() error {
		describedIndex, err := ts.client.DescribeIndex(ctx, indexName)
		if err != nil {
			return fmt.Errorf("DescribeIndex failed: %v", err)
		}
		assert.NotNil(ts.T(), describedIndex.Spec, "Index.Spec should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless, "Index.Spec.Serverless should not be nil")
		if describedIndex.Schema == nil {
			return fmt.Errorf("Schema not yet set on described index")
		}
		if _, ok := describedIndex.Schema.Fields["category"]; !ok {
			return fmt.Errorf("declared metadata field %q not yet reported in index schema", "category")
		}
		return nil
	})

	defer func(ts *integrationTests, name string) {
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

	idxConn, err := ts.client.Index(NewIndexConnParams{Host: index.Host, Namespace: ts.namespaces[0]})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), idxConn)
	defer idxConn.Close()

	err = idxConn.UpsertRecords(ctx, records)
	require.NoError(ts.T(), err)

	retryAssertionsWithDefaults(ts.T(), func() error {
		res, err := idxConn.SearchRecords(ctx, &SearchRecordsRequest{
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

func (ts *integrationTests) TestIntegratedEmbeddingDocuments() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Running TestIntegratedEmbeddingDocuments once")
	}
	ctx := context.Background()
	indexName := "test-embed-" + generateTestIndexName()
	defaultDenseModel := "llama-text-embed-v2"
	defualtSparseModel := "pinecone-sparse-english-v0"

	_, err := ts.client.CreateIndex(ctx, &CreateIndexRequest{
		Name: indexName,
		Schema: IndexSchema{Fields: map[string]IndexSchemaField{
			"body":    {String: &StringField{Embed: &EmbedConfig{}}},
			"summary": {String: &StringField{SparseEmbed: &SparseEmbedConfig{}, StoreText: ptr(false)}},
		}},
		Tags: ts.indexTags,
	})
	require.NoError(ts.T(), err)
	defer func(ts *integrationTests, name string) {
		require.NoError(ts.T(), ts.deleteIndex(name))
	}(ts, indexName)

	_, err = waitUntilIndexReady(ts, ctx, indexName)
	require.NoError(ts.T(), err)

	// Describe reports the configuration resolved from the model's defaults.
	index, err := ts.client.DescribeIndex(ctx, indexName)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), index.Schema)

	// Check dense embedding
	body := index.Schema.Fields["body"].String
	require.NotNil(ts.T(), body)
	require.NotNil(ts.T(), body.Embed)
	assert.Equal(ts.T(), defaultDenseModel, derefOrDefault(body.Embed.Model, ""))
	assert.Equal(ts.T(), int32(1024), derefOrDefault(body.Embed.Dimension, 0))
	assert.NotNil(ts.T(), body.Embed.Metric)
	assert.NotNil(ts.T(), body.FullTextSearch, "a stored string field is indexed for full-text search")

	// Check sparse embedding
	summary := index.Schema.Fields["summary"].String
	require.NotNil(ts.T(), summary)
	require.NotNil(ts.T(), summary.SparseEmbed)
	assert.Equal(ts.T(), defualtSparseModel, derefOrDefault(summary.SparseEmbed.Model, ""))
	assert.Equal(ts.T(), ptr(false), summary.StoreText)

	// Connect to the index
	idxConn, err := ts.client.Index(NewIndexConnParams{Host: index.Host, Namespace: ts.namespaces[0]})
	require.NoError(ts.T(), err)
	defer idxConn.Close()

	upsertRes, err := idxConn.UpsertDocuments(ctx, &UpsertDocumentsRequest{Documents: []Document{
		{"_id": "shoes", "body": "Lightweight trail running shoes with a grippy outsole.", "summary": "trail running shoes", "category": "footwear"},
		{"_id": "tent", "body": "A two-person backpacking tent that pitches in minutes.", "summary": "backpacking tent", "category": "shelter"},
		{"_id": "stove", "body": "A compact camping stove that boils water fast.", "summary": "camping stove", "category": "cooking"},
	}})
	require.NoError(ts.T(), err)
	assert.Equal(ts.T(), int32(3), upsertRes.UpsertedCount)
	require.NotNil(ts.T(), upsertRes.Usage)
	assert.Greater(ts.T(), upsertRes.Usage.EmbedTotalTokens, int32(0))

	query := "running shoes"
	tests := []struct {
		name    string
		scoreBy DocumentScoringMethod
	}{
		{name: "embed", scoreBy: DocumentScoringMethod{Type: DocumentScoringTypeEmbed, Fields: []string{"body"}, Query: &query}},
		{name: "sparse_embed", scoreBy: DocumentScoringMethod{Type: DocumentScoringTypeSparseEmbed, Fields: []string{"summary"}, Query: &query}},
		{name: "text", scoreBy: DocumentScoringMethod{Type: DocumentScoringTypeText, Fields: []string{"body"}, Query: &query}},
	}
	for _, tt := range tests {
		// Retry the search until data is properly indexed
		retryAssertionsWithDefaults(ts.T(), func() error {
			res, err := idxConn.SearchDocuments(ctx, &SearchDocumentsRequest{
				TopK:          3,
				ScoreBy:       []DocumentScoringMethod{tt.scoreBy},
				IncludeFields: []string{"body"},
			})
			if err != nil {
				return fmt.Errorf("%s search failed: %w", tt.name, err)
			}
			if len(res.Matches) == 0 {
				return fmt.Errorf("%s search returned no matches yet", tt.name)
			}
			if res.Matches[0].Id != "shoes" {
				return fmt.Errorf("%s search ranked %q first, want %q", tt.name, res.Matches[0].Id, "shoes")
			}
			return nil
		})
	}

	// Error case - A field whose text isn't stored can't be returned.
	_, err = idxConn.SearchDocuments(ctx, &SearchDocumentsRequest{
		TopK:          3,
		ScoreBy:       []DocumentScoringMethod{tests[1].scoreBy},
		IncludeFields: []string{"summary"},
	})
	var pcErr *PineconeError
	require.ErrorAs(ts.T(), err, &pcErr)
	assert.Equal(ts.T(), http.StatusBadRequest, pcErr.Code)
}

func (ts *integrationTests) TestDescribeNamespace() {
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

func (ts *integrationTests) TestListNamespaces() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Namespace operations are only supported in serverless indexes")
	}

	ctx := context.Background()

	// Get all namespaces to determine total count
	allNamespaces, err := ts.idxConn.ListNamespaces(ctx, nil)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), allNamespaces, "ListNamespaces response should not be nil")
	expectedTotalCount := allNamespaces.TotalCount
	require.Greater(ts.T(), expectedTotalCount, int32(0), "TotalCount should be greater than 0")

	// List one namespace with limit
	limit := uint32(1)
	namespaces, err := ts.idxConn.ListNamespaces(ctx, &ListNamespacesParams{
		Limit: &limit,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), namespaces, "ListNamespaces response should not be nil")
	require.Equal(ts.T(), limit, uint32(len(namespaces.Namespaces)))
	require.Equal(ts.T(), expectedTotalCount, namespaces.TotalCount, "TotalCount should be consistent across paginated requests")

	// List remaining
	if namespaces.Pagination != nil && namespaces.Pagination.Next != "" {
		namespaces, err = ts.idxConn.ListNamespaces(ctx, &ListNamespacesParams{
			PaginationToken: &namespaces.Pagination.Next,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), namespaces, "ListNamespaces response should not be nil")
		require.Greater(ts.T(), len(namespaces.Namespaces), 0, "ListNamespaces should return the second page of results")
		require.Equal(ts.T(), expectedTotalCount, namespaces.TotalCount, "TotalCount should be consistent across paginated requests")
	}
}

func (ts *integrationTests) TestCreateNamespace() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Namespace operations are only supported in serverless indexes")
	}

	ctx := context.Background()

	// Test creating namespace without Schema
	namespaceName1 := "test-namespace-" + generateTestIndexName()
	namespace1, err := ts.idxConn.CreateNamespace(ctx, &CreateNamespaceParams{
		Name: namespaceName1,
	})
	require.NoError(ts.T(), err, "CreateNamespace should not return an error")
	require.NotNil(ts.T(), namespace1, "Namespace description should not be nil")
	require.Equal(ts.T(), namespaceName1, namespace1.Name, "Namespace name should match")

	// Verify namespace can be described
	retryAssertionsWithDefaults(ts.T(), func() error {
		namespaceDesc, err := ts.idxConn.DescribeNamespace(ctx, namespaceName1)
		if err != nil {
			return fmt.Errorf("DescribeNamespace failed: %v", err)
		}
		assert.NotNil(ts.T(), namespaceDesc, "Namespace description should not be nil")
		assert.Equal(ts.T(), namespaceName1, namespaceDesc.Name, "Namespace name should match")
		return nil
	})

	// Test creating namespace with Schema
	namespaceName2 := "test-namespace-" + generateTestIndexName()
	testSchema := &MetadataSchema{
		Fields: map[string]MetadataSchemaField{
			"category": {Filterable: true},
			"rating":   {Filterable: true},
		},
	}
	namespace2, err := ts.idxConn.CreateNamespace(ctx, &CreateNamespaceParams{
		Name:   namespaceName2,
		Schema: testSchema,
	})
	require.NoError(ts.T(), err, "CreateNamespace with Schema should not return an error")
	require.NotNil(ts.T(), namespace2, "Namespace description should not be nil")
	require.Equal(ts.T(), namespaceName2, namespace2.Name, "Namespace name should match")

	// Verify namespace with schema can be described
	retryAssertionsWithDefaults(ts.T(), func() error {
		namespaceDesc, err := ts.idxConn.DescribeNamespace(ctx, namespaceName2)
		if err != nil {
			return fmt.Errorf("DescribeNamespace failed: %v", err)
		}
		assert.NotNil(ts.T(), namespaceDesc, "Namespace description should not be nil")
		assert.Equal(ts.T(), namespaceName2, namespaceDesc.Name, "Namespace name should match")
		if namespaceDesc.Schema != nil {
			assert.Equal(ts.T(), len(testSchema.Fields), len(namespaceDesc.Schema.Fields), "Schema fields count should match")
		}
		return nil
	})

	// Clean up created namespaces
	err = ts.idxConn.DeleteNamespace(ctx, namespaceName1)
	if err != nil {
		fmt.Printf("Failed to delete namespace %s: %v\n", namespaceName1, err)
	}
	err = ts.idxConn.DeleteNamespace(ctx, namespaceName2)
	if err != nil {
		fmt.Printf("Failed to delete namespace %s: %v\n", namespaceName2, err)
	}
}

func (ts *integrationTests) TestDeleteNamespace() {
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
