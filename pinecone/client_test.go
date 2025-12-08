package pinecone

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/v5/internal/gen"
	"github.com/pinecone-io/go-pinecone/v5/internal/gen/db_control"
	"github.com/pinecone-io/go-pinecone/v5/internal/provider"
	"github.com/pinecone-io/go-pinecone/v5/internal/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests:
func (ts *integrationTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *integrationTests) TestCreatePodIndexDense() {
	if ts.indexType == "serverless" {
		ts.T().Skip("Skipping pod index tests for serverless suite")
	}

	name := uuid.New().String()
	metric := Cosine

	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      &metric,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
	// create index should default to "dense" if no VectorType is specified
	require.Equal(ts.T(), "dense", idx.VectorType, "Index vector type does not match")
}

func (ts *integrationTests) TestCreateServerlessIndexDense() {
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping serverless index tests for pod suite")
	}

	name := uuid.New().String()
	dimension := int32(10)
	metric := Cosine

	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: &dimension,
		Metric:    &metric,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
	// create index should default to "dense" if no VectorType is specified
	require.Equal(ts.T(), "dense", idx.VectorType, "Index vector type does not match")
}

func (ts *integrationTests) TestCreateServerlessIndexSparse() {
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping serverless index tests for pod suite")
	}

	name := uuid.New().String()
	vectorType := "sparse"
	metric := Dotproduct

	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:       name,
		Metric:     &metric,
		Cloud:      Aws,
		Region:     "us-west-2",
		VectorType: &vectorType,
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
	require.Equal(ts.T(), vectorType, idx.VectorType, "Index vector type does not match")
}

func (ts *integrationTests) TestCreateServerlessIndexInvalidDimension() {
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping serverless index tests for pod suite")
	}

	name := uuid.New().String()
	dimension := int32(-1)
	metric := Cosine

	_, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: &dimension,
		Metric:    &metric,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *integrationTests) TestDescribeIndex() {
	ctx := context.Background()
	index, err := ts.client.DescribeIndex(ctx, ts.idxName)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), index)

	// Assert index name
	assert.Equal(ts.T(), ts.idxName, index.Name, "Index name should match")

	// Assert index tags
	if ts.indexTags != nil {
		assert.Equal(ts.T(), ts.indexTags, index.Tags, "Index tags should match")
	}

	// Assert Schema for serverless indexes
	if ts.indexType == "serverless" {
		assert.NotNil(ts.T(), index.Spec, "Index.Spec should not be nil")
		assert.NotNil(ts.T(), index.Spec.Serverless, "Index.Spec.Serverless should not be nil")
		assert.NotNil(ts.T(), index.Spec.Serverless.Schema, "Schema should be set on the test index")

		if ts.schema != nil {
			expectedFields := ts.schema.Fields
			actualFields := index.Spec.Serverless.Schema.Fields

			// Assert field count matches
			assert.Equal(ts.T(), len(expectedFields), len(actualFields), "Schema field count should match")

			// Assert each field matches
			for fieldName, expectedField := range expectedFields {
				actualField, exists := actualFields[fieldName]
				assert.True(ts.T(), exists, "Field %s should exist in schema", fieldName)
				if exists {
					assert.Equal(ts.T(), expectedField.Filterable, actualField.Filterable, "Field %s Filterable property should match", fieldName)
				}
			}
		}
	}
}

func (ts *integrationTests) TestDescribeNonExistentIndex() {
	_, err := ts.client.DescribeIndex(context.Background(), "non-existent-index")
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *integrationTests) TestListCollections() {
	if ts.indexType == "serverless" {
		ts.T().Skip("No pod index to test")
	}
	ctx := context.Background()

	// Call the method under test to list all collections
	collections, err := ts.client.ListCollections(ctx)
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(collections), 0, "Expected at least one collection to exist")

	// Check that the created collection is returned in the list
	found := false
	for _, collection := range collections {
		if collection.Name == ts.collectionName {
			found = true
		}
	}
	require.True(ts.T(), found, "Collection %v not found in list of collections", ts.collectionName)
}

func (ts *integrationTests) TestDescribeCollection() {
	if ts.indexType == "serverless" {
		ts.T().Skip("No pod index to test")
	}
	ctx := context.Background()

	collection, err := ts.client.DescribeCollection(ctx, ts.collectionName)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.collectionName, collection.Name, "Collection name does not match")
}

func (ts *integrationTests) TestDeletionProtection() {
	// configure index to enable deletion protection
	index, err := ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{DeletionProtection: "enabled"})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), DeletionProtectionEnabled, index.DeletionProtection, "Expected deletion protection to be 'enabled'")

	// validate we cannot delete the index
	err = ts.client.DeleteIndex(context.Background(), ts.idxName)
	require.ErrorContainsf(ts.T(), err, "failed to delete index: Deletion protection is enabled for this index", err.Error())

	// disable deletion protection so the index can be cleaned up during integration teardown
	_, err = ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{DeletionProtection: "disabled"})
	require.NoError(ts.T(), err)

	// Before moving on to another test, wait for the index to be done upgrading
	_, err = waitUntilIndexReady(ts, context.Background())
	require.NoError(ts.T(), err)
}

func (ts *integrationTests) TestConfigureIndexIllegalScaleDown() {
	name := uuid.New().String()
	metric := Cosine

	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      &metric,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{PodType: "p1.x1"})
	require.ErrorContainsf(ts.T(), err, "Cannot scale down", err.Error())
}

func (ts *integrationTests) TestConfigureIndexScaleUpNoPods() {
	name := uuid.New().String()
	metric := Cosine

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      &metric,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{Replicas: 2})
	require.NoError(ts.T(), err)

	// give index a bit of time to upgrade
	time.Sleep(20 * time.Second)

	err = ts.client.DeleteIndex(context.Background(), name)
	require.NoError(ts.T(), err)
}

func (ts *integrationTests) TestConfigureIndexScaleUpNoReplicas() {
	name := uuid.New().String()
	metric := Cosine

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      &metric,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{PodType: "p1.x4"})
	require.NoError(ts.T(), err)

	// give index a bit of time to upgrade
	time.Sleep(20 * time.Second)

	err = ts.client.DeleteIndex(context.Background(), name)
	require.NoError(ts.T(), err)
}

func (ts *integrationTests) TestConfigureIndexIllegalNoPodsOrReplicasOrDeletionProtection() {
	_, err := ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{})
	require.ErrorContainsf(ts.T(), err, "must specify PodType, Replicas, DeletionProtection, or Tags", err.Error())
}

func (ts *integrationTests) TestConfigureIndexHitPodLimit() {
	name := uuid.New().String()
	metric := Cosine

	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      &metric,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{Replicas: 30000})
	require.ErrorContainsf(ts.T(), err, "You've reached the max pods allowed", err.Error())
}

func (ts *integrationTests) TestDescribeEmbedModel() {
	ctx := context.Background()
	modelName := "multilingual-e5-large"
	paramQuery := "query"
	paramPassage := "passage"
	paramEND := "END"
	paramNONE := "NONE"
	supportedDimensions := []int32{1024}
	supportedMetrics := []IndexMetric{"cosine", "euclidean"}
	allowedValuesInputType := []SupportedParameterValue{{StringValue: &paramQuery}, {StringValue: &paramPassage}}
	allowedValuesTruncate := []SupportedParameterValue{{StringValue: &paramEND}, {StringValue: &paramNONE}}
	supportedParameters := []SupportedParameter{
		{
			Type:          "one_of",
			Required:      true,
			Parameter:     "input_type",
			ValueType:     "string",
			AllowedValues: &allowedValuesInputType,
		},
		{
			Type:          "one_of",
			Required:      false,
			Parameter:     "truncate",
			ValueType:     "string",
			Default:       &SupportedParameterValue{StringValue: &paramEND},
			AllowedValues: &allowedValuesTruncate,
		},
	}

	model, err := ts.client.Inference.DescribeModel(ctx, modelName)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), model, "Expected model to be non-nil")
	require.Equal(ts.T(), "embed", model.Type, "Expected model type to be 'embed'")
	require.Equal(ts.T(), modelName, model.Model, "Expected model name to match")
	require.Equal(ts.T(), model.ShortDescription, "A high-performance dense embedding model trained on a mixture of multilingual datasets. It works well on messy data and short queries expected to return medium-length passages of text (1-2 paragraphs)")
	require.Equal(ts.T(), "dense", *model.VectorType, "Expected model vector type to be 'dense'")
	require.Equal(ts.T(), "text", *model.Modality, "Expected model modality to be 'text'")
	require.Equal(ts.T(), int32(1024), *model.DefaultDimension, "Expected model default dimension to be 1024")
	require.Equal(ts.T(), int32(507), *model.MaxSequenceLength, "Expected model max sequence length to be 507")
	require.Equal(ts.T(), int32(96), *model.MaxBatchSize, "Expected model max batch size to be 96")
	require.Equal(ts.T(), "Microsoft", *model.ProviderName, "Expected model provider name to be 'Microsoft'")
	require.Equal(ts.T(), supportedDimensions, *model.SupportedDimensions, "Expected model supported dimensions to match")
	require.Equal(ts.T(), supportedMetrics, *model.SupportedMetrics, "Expected model supported metrics to match")
	require.Equal(ts.T(), supportedParameters, *model.SupportedParameters, "Expected model supported parameters to match")
}

func (ts *integrationTests) TestDescribeRerankModel() {
	ctx := context.Background()
	modelName := "pinecone-rerank-v0"
	paramEND := "END"
	paramNONE := "NONE"
	defaultParam := SupportedParameterValue{StringValue: &paramEND}
	allowedValues := []SupportedParameterValue{{StringValue: &paramEND}, {StringValue: &paramNONE}}
	supportedParameters := []SupportedParameter{
		{
			Type:          "one_of",
			Default:       &defaultParam,
			Required:      false,
			Parameter:     "truncate",
			ValueType:     "string",
			AllowedValues: &allowedValues,
		},
	}

	model, err := ts.client.Inference.DescribeModel(ctx, modelName)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), model, "Expected model to be non-nil")
	require.Equal(ts.T(), "rerank", model.Type, "Expected model type to be 'rerank'")
	require.Equal(ts.T(), modelName, model.Model, "Expected model name to match")
	require.Equal(ts.T(), model.ShortDescription, "A state of the art reranking model that out-performs competitors on widely accepted benchmarks. It can handle chunks up to 512 tokens (1-2 paragraphs)")
	require.Equal(ts.T(), "text", *model.Modality, "Expected model modality to be 'text'")
	require.Equal(ts.T(), int32(512), *model.MaxSequenceLength, "Expected model max sequence length to be 512")
	require.Equal(ts.T(), int32(100), *model.MaxBatchSize, "Expected model max batch size to be 100")
	require.Equal(ts.T(), "Pinecone", *model.ProviderName, "Expected model provider name to be 'Pinecone'")
	require.Equal(ts.T(), supportedParameters, *model.SupportedParameters, "Expected model supported parameters to match")
	require.Nil(ts.T(), model.VectorType, "Expected model vector type to be nil")
	require.Nil(ts.T(), model.SupportedMetrics, "Expected model supported metrics to be nil")
	require.Nil(ts.T(), model.SupportedDimensions, "Expected model supported dimensions to be nil")
	require.Nil(ts.T(), model.DefaultDimension, "Expected model default dimension to be nil")
}

func (ts *integrationTests) TestListAllModels() {
	ctx := context.Background()

	allModels, err := ts.client.Inference.ListModels(ctx, nil)
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), allModels, "Expected model results to be non-nil")
	require.Greater(ts.T(), len(*allModels.Models), 0, "Expected at least one model to be listed")

	returnsRerank := false
	returnsEmbed := false
	returnsSparse := false
	returnsDense := false
	for _, model := range *allModels.Models {
		if model.Type == "rerank" {
			returnsRerank = true
		}
		if model.Type == "embed" {
			returnsEmbed = true
			if *model.VectorType == "sparse" {
				returnsSparse = true
			} else if *model.VectorType == "dense" {
				returnsDense = true
			}
		}
	}
	require.True(ts.T(), returnsRerank, "Expected at least one rerank model to be listed")
	require.True(ts.T(), returnsEmbed, "Expected at least one embed model to be listed")
	require.True(ts.T(), returnsSparse, "Expected at least one sparse embed model to be listed")
	require.True(ts.T(), returnsDense, "Expected at least one dense embed model to be listed")
}

func (ts *integrationTests) TestListRerankModels() {
	ctx := context.Background()
	rerank := "rerank"

	rerankModels, err := ts.client.Inference.ListModels(ctx, &ListModelsParams{
		Type: &rerank,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), rerankModels, "Expected model results to be non-nil")
	require.Greater(ts.T(), len(*rerankModels.Models), 0, "Expected at least one model to be listed")

	returnsOnlyRerank := true
	returnsEmbed := false

	for _, model := range *rerankModels.Models {
		if model.Type != "rerank" {
			returnsOnlyRerank = false
		}
		if model.Type == "embed" {
			returnsEmbed = true
		}
	}

	require.Equal(ts.T(), true, returnsOnlyRerank, "Expected all models to be of type 'rerank'")
	require.Equal(ts.T(), false, returnsEmbed, "Expected no embed models to be listed in rerank models")
}

func (ts *integrationTests) TestListEmbeddingModels() {
	ctx := context.Background()
	embed := "embed"
	sparse := "sparse"
	dense := "dense"
	returnsOnlyEmbed := true

	// List embed models (sparse)
	sparseEmbedModels, err := ts.client.Inference.ListModels(ctx, &ListModelsParams{
		Type:       &embed,
		VectorType: &sparse,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), sparseEmbedModels, "Expected model results to be non-nil")
	require.Greater(ts.T(), len(*sparseEmbedModels.Models), 0, "Expected at least one model to be listed")

	allSparseModels := true
	for _, model := range *sparseEmbedModels.Models {
		if model.Type != "embed" {
			returnsOnlyEmbed = false
		}
		if *model.VectorType != "sparse" {
			allSparseModels = false
		}
	}
	require.Equal(ts.T(), true, returnsOnlyEmbed, "Expected all models to be of type 'embed'")
	require.Equal(ts.T(), true, allSparseModels, "Expected all listed models to be of vector type 'sparse'")

	// List embed models (dense)
	denseEmbedModels, err := ts.client.Inference.ListModels(ctx, &ListModelsParams{
		Type:       &embed,
		VectorType: &dense,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), denseEmbedModels, "Expected model results to be non-nil")
	require.Greater(ts.T(), len(*denseEmbedModels.Models), 0, "Expected at least one model to be listed")

	allDenseModels := true
	for _, model := range *denseEmbedModels.Models {
		if model.Type != "embed" {
			returnsOnlyEmbed = false
		}
		if *model.VectorType != "dense" {
			allDenseModels = false
		}
	}
	require.Equal(ts.T(), true, returnsOnlyEmbed, "Expected all models to be of type 'embed'")
	require.Equal(ts.T(), true, allDenseModels, "Expected all listed models to be of vector type 'dense'")
}

func (ts *integrationTests) TestGenerateEmbeddingsDense() {
	// Run Embed tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Embed tests for pods")
	}

	ctx := context.Background()
	embeddingModel := "multilingual-e5-large"
	denseEmbeddings, err := ts.client.Inference.Embed(ctx, &EmbedRequest{
		Model: embeddingModel,
		TextInputs: []string{
			"The quick brown fox jumps over the lazy dog",
			"Lorem ipsum",
		},
		Parameters: map[string]interface{}{
			"input_type": "query",
			"truncate":   "END",
		},
	})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), denseEmbeddings, "Expected embedding to be non-nil")
	require.Equal(ts.T(), embeddingModel, denseEmbeddings.Model, "Expected model to be '%s', but got '%s'", embeddingModel, denseEmbeddings.Model)
	require.Equal(ts.T(), 2, len(denseEmbeddings.Data), "Expected 2 embeddings")
	require.NotNil(ts.T(), denseEmbeddings.Data[0].DenseEmbedding, "Expected DenseEmbedding to be non-nil")
	require.Equal(ts.T(), 1024, len(denseEmbeddings.Data[0].DenseEmbedding.Values), "Expected embeddings to have length 1024")
}

func (ts *integrationTests) TestGenerateEmbeddingsSparse() {
	// Run Embed tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Embed tests for pods")
	}

	ctx := context.Background()
	embeddingModel := "pinecone-sparse-english-v0"
	sparseEmbeddings, err := ts.client.Inference.Embed(ctx, &EmbedRequest{
		Model: embeddingModel,
		TextInputs: []string{
			"The quick brown fox jumps over the lazy dog",
			"Lorem ipsum",
		},
		Parameters: map[string]interface{}{
			"input_type":    "passage",
			"return_tokens": true,
		},
	})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), sparseEmbeddings, "Expected embedding to be non-nil")
	require.Equal(ts.T(), embeddingModel, sparseEmbeddings.Model, "Expected model to be '%s', but got '%s'", embeddingModel, sparseEmbeddings.Model)
	require.Equal(ts.T(), 2, len(sparseEmbeddings.Data), "Expected 2 embeddings")
	require.NotNil(ts.T(), sparseEmbeddings.Data[0].SparseEmbedding, "Expected SparseEmbedding to be non-nil")
	require.NotNil(ts.T(), sparseEmbeddings.Data[0].SparseEmbedding.SparseTokens, "Expected SparseTokens to be non-nil")
	require.NotNil(ts.T(), sparseEmbeddings.Data[0].SparseEmbedding.SparseIndices, "Expected SparseIndices to be non-nil")
	require.NotNil(ts.T(), sparseEmbeddings.Data[0].SparseEmbedding.SparseValues, "Expected SparseValues to be non-nil")
}

func (ts *integrationTests) TestGenerateEmbeddingsInvalidInputs() {
	ctx := context.Background()
	embeddingModel := "multilingual-e5-large"
	_, err := ts.client.Inference.Embed(ctx, &EmbedRequest{
		Model: embeddingModel,
		Parameters: map[string]interface{}{
			"input_type": "query",
			"truncate":   "END",
		},
	})

	require.Error(ts.T(), err)
	require.Contains(ts.T(), err.Error(), "TextInputs must contain at least one value")
}

func (ts *integrationTests) TestRerankDocumentDefaultField() {
	// Run Rerank tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Rerank tests for pods")
	}

	ctx := context.Background()
	rerankModel := "bge-reranker-v2-m3"
	topN := 2
	retunDocuments := true
	ranking, err := ts.client.Inference.Rerank(ctx, &RerankRequest{
		Model:           rerankModel,
		Query:           "i love apples",
		ReturnDocuments: &retunDocuments,
		TopN:            &topN,
		Documents: []Document{
			{"id": "vec1", "text": "Apple is a popular fruit known for its sweetness and crisp texture."},
			{"id": "vec2", "text": "Many people enjoy eating apples as a healthy snack."},
			{"id": "vec3", "text": "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces."},
			{"id": "vec4", "text": "An apple a day keeps the doctor away, as the saying goes."},
		}})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), ranking, "Expected reranking result to be non-nil")
	require.Equal(ts.T(), topN, len(ranking.Data), "Expected %v rankings", topN)

	doc := *ranking.Data[0].Document
	_, exists := doc["text"]
	require.True(ts.T(), exists, "Expected '%s' to exist in Document map", "text")
	_, exists = doc["id"]
	require.True(ts.T(), exists, "Expected '%s' to exist in Document map", "id")
}

func (ts *integrationTests) TestRerankDocumentCustomField() {
	// Run Rerank tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Rerank tests for pods")
	}

	ctx := context.Background()
	rerankModel := "bge-reranker-v2-m3"
	topN := 2
	retunDocuments := true
	ranking, err := ts.client.Inference.Rerank(ctx, &RerankRequest{
		Model:           rerankModel,
		Query:           "i love apples",
		ReturnDocuments: &retunDocuments,
		TopN:            &topN,
		RankFields:      &[]string{"customField"},
		Documents: []Document{
			{"id": "vec1", "customField": "Apple is a popular fruit known for its sweetness and crisp texture."},
			{"id": "vec2", "customField": "Many people enjoy eating apples as a healthy snack."},
			{"id": "vec3", "customField": "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces."},
			{"id": "vec4", "customField": "An apple a day keeps the doctor away, as the saying goes."},
		}})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), ranking, "Expected reranking result to be non-nil")
	require.Equal(ts.T(), topN, len(ranking.Data), "Expected %v rankings", topN)

	doc := *ranking.Data[0].Document
	_, exists := doc["customField"]
	require.True(ts.T(), exists, "Expected '%s' to exist in Document map", "customField")
	_, exists = doc["id"]
	require.True(ts.T(), exists, "Expected '%s' to exist in Document map", "id")
}

func (ts *integrationTests) TestRerankDocumentAllDefaults() {
	// Run Rerank tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Rerank tests for pods")
	}

	ctx := context.Background()
	rerankModel := "bge-reranker-v2-m3"
	ranking, err := ts.client.Inference.Rerank(ctx, &RerankRequest{
		Model: rerankModel,
		Query: "i love apples",
		Documents: []Document{
			{"id": "vec1", "text": "Apple is a popular fruit known for its sweetness and crisp texture."},
			{"id": "vec2", "text": "Many people enjoy eating apples as a healthy snack."},
			{"id": "vec3", "text": "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces."},
			{"id": "vec4", "text": "An apple a day keeps the doctor away, as the saying goes."},
		}})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), ranking, "Expected reranking result to be non-nil")
	require.Equal(ts.T(), 4, len(ranking.Data), "Expected %v rankings", 4)

	doc := *ranking.Data[0].Document
	_, exists := doc["text"]
	require.True(ts.T(), exists, "Expected '%s' to exist in Document map", "text")
	_, exists = doc["id"]
	require.True(ts.T(), exists, "Expected '%s' to exist in Document map", "id")
}

func (ts *integrationTests) TestRerankDocumentsMultipleRankFields() {
	// Run Rerank tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Rerank tests for pods")
	}

	ctx := context.Background()
	rerankModel := "bge-reranker-v2-m3"
	_, err := ts.client.Inference.Rerank(ctx, &RerankRequest{
		Model:      rerankModel,
		Query:      "i love apples",
		RankFields: &[]string{"text", "custom-field"},
		Documents: []Document{
			{
				"id":           "vec1",
				"text":         "Apple is a popular fruit known for its sweetness and crisp texture.",
				"custom-field": "another field",
			},
			{
				"id":           "vec2",
				"text":         "Many people enjoy eating apples as a healthy snack.",
				"custom-field": "another field",
			},
			{
				"id":           "vec3",
				"text":         "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces.",
				"custom-field": "another field",
			},
			{
				"id":           "vec4",
				"text":         "An apple a day keeps the doctor away, as the saying goes.",
				"custom-field": "another field",
			},
		}})

	require.Error(ts.T(), err)
	require.Contains(ts.T(), err.Error(), "Only one rank field is supported for model")
}

func (ts *integrationTests) TestRerankDocumentFieldError() {
	// Run Rerank tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Rerank tests for pods")
	}

	ctx := context.Background()
	rerankModel := "bge-reranker-v2-m3"
	_, err := ts.client.Inference.Rerank(ctx, &RerankRequest{
		Model:      rerankModel,
		Query:      "i love apples",
		RankFields: &[]string{"custom-field"},
		Documents: []Document{
			{"id": "vec1", "text": "Apple is a popular fruit known for its sweetness and crisp texture."},
			{"id": "vec2", "text": "Many people enjoy eating apples as a healthy snack."},
			{"id": "vec3", "text": "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces."},
			{"id": "vec4", "text": "An apple a day keeps the doctor away, as the saying goes."},
		}})

	require.Error(ts.T(), err)
	require.Contains(ts.T(), err.Error(), "field 'custom-field' not found in document")
}

func (ts *integrationTests) TestIndexTags() {
	// Validate that index tags are set
	index, err := ts.client.DescribeIndex(context.Background(), ts.idxName)
	require.NoError(ts.T(), err)

	assert.Equal(ts.T(), ts.indexTags, index.Tags, "Expected index tags to match")

	// Update first tag, and clear the second
	counter := 0
	updatedTags := make(IndexTags)
	deletedTag := ""
	for key := range *ts.indexTags {
		if counter == 0 {
			updatedTags[key] = "updated-tag"
		} else {
			deletedTag = key
			updatedTags[key] = ""
		}
		counter++
	}

	index, err = ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{Tags: updatedTags})
	require.NoError(ts.T(), err)

	// Remove empty tag from the map
	delete(updatedTags, deletedTag)

	assert.Equal(ts.T(), &updatedTags, index.Tags, "Expected index tags to match")
	ts.indexTags = &updatedTags
}

func (ts *integrationTests) TestCreateServerlessIndexWithReadCapacity() {
	if ts.indexType != "serverless" {
		ts.T().Skip("ReadCapacity is only supported in serverless indexes")
	}

	ctx := context.Background()

	// Test creating index with OnDemand ReadCapacity (default)
	indexName1 := "rc-ondemand-" + generateTestIndexName()
	dimension := int32(setDimensionsForTestIndexes())
	metric := Cosine

	index1, err := ts.client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:      indexName1,
		Dimension: &dimension,
		Metric:    &metric,
		Region:    "us-east-1",
		Cloud:     "aws",
		// ReadCapacity is nil, which should default to OnDemand
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), index1)

	// Clean up index after test run
	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, indexName1)

	// Verify OnDemand ReadCapacity is returned
	retryAssertionsWithDefaults(ts.T(), func() error {
		describedIndex, err := ts.client.DescribeIndex(ctx, indexName1)
		if err != nil {
			return fmt.Errorf("DescribeIndex failed: %v", err)
		}
		assert.NotNil(ts.T(), describedIndex.Spec, "Index.Spec should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless, "Index.Spec.Serverless should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity, "ReadCapacity should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity.OnDemand, "ReadCapacity should be OnDemand by default")
		return nil
	})

	// Test creating index with Dedicated ReadCapacity
	indexName2 := "rc-dedicated-" + generateTestIndexName()
	readCapacity := &ReadCapacityParams{
		Dedicated: &ReadCapacityDedicatedConfig{
			NodeType: ptr("t1"),
			Scaling: &ReadCapacityScaling{
				Manual: &ReadCapacityManualScaling{
					Replicas: ptr(int32(1)),
					Shards:   ptr(int32(1)),
				},
			},
		},
	}

	index2, err := ts.client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:         indexName2,
		Dimension:    &dimension,
		Metric:       &metric,
		Region:       "us-east-1",
		Cloud:        "aws",
		ReadCapacity: readCapacity,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), index2)

	// Clean up index after test run
	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, indexName2)

	// Verify Dedicated ReadCapacity is returned
	retryAssertionsWithDefaults(ts.T(), func() error {
		describedIndex, err := ts.client.DescribeIndex(ctx, indexName2)
		if err != nil {
			return fmt.Errorf("DescribeIndex failed: %v", err)
		}
		assert.NotNil(ts.T(), describedIndex.Spec, "Index.Spec should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless, "Index.Spec.Serverless should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity, "ReadCapacity should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity.Dedicated, "ReadCapacity should be Dedicated")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity.Dedicated.NodeType, "NodeType should not be nil")
		assert.Equal(ts.T(), "t1", *describedIndex.Spec.Serverless.ReadCapacity.Dedicated.NodeType, "NodeType should be t1")
		return nil
	})
}

func (ts *integrationTests) TestConfigureIndexReadCapacity() {
	if ts.indexType != "serverless" {
		ts.T().Skip("ReadCapacity is only supported in serverless indexes")
	}

	ctx := context.Background()

	// Create a test index with OnDemand ReadCapacity
	indexName := "configure-rc-" + generateTestIndexName()
	dimension := int32(setDimensionsForTestIndexes())
	metric := Cosine

	index, err := ts.client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:      indexName,
		Dimension: &dimension,
		Metric:    &metric,
		Region:    "us-east-1",
		Cloud:     "aws",
		ReadCapacity: &ReadCapacityParams{
			OnDemand: &ReadCapacityOnDemandConfig{},
		},
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), index)

	defer func(ts *integrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, indexName)

	// Wait for index to be ready
	_, err = waitUntilIndexReady(ts, ctx)
	require.NoError(ts.T(), err)

	// Configure index to use Dedicated ReadCapacity
	readCapacity := &ReadCapacityParams{
		Dedicated: &ReadCapacityDedicatedConfig{
			NodeType: ptr("t1"),
			Scaling: &ReadCapacityScaling{
				Manual: &ReadCapacityManualScaling{
					Replicas: ptr(int32(1)),
					Shards:   ptr(int32(1)),
				},
			},
		},
	}

	// Verify ReadCapacity was updated to dedicated
	retryAssertionsWithDefaults(ts.T(), func() error {
		configuredIndex, err := ts.client.ConfigureIndex(ctx, indexName, ConfigureIndexParams{
			ReadCapacity: readCapacity,
		})
		if err != nil {
			return fmt.Errorf("ConfigureIndex failed: %v", err)
		}
		assert.NoError(ts.T(), err)
		assert.NotNil(ts.T(), configuredIndex)

		describedIndex, err := ts.client.DescribeIndex(ctx, indexName)
		if err != nil {
			return fmt.Errorf("DescribeIndex failed: %v", err)
		}
		assert.NotNil(ts.T(), describedIndex.Spec, "Index.Spec should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless, "Index.Spec.Serverless should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity, "ReadCapacity should not be nil")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity.Dedicated, "ReadCapacity should be Dedicated after configuration")
		assert.NotNil(ts.T(), describedIndex.Spec.Serverless.ReadCapacity.Dedicated.NodeType, "NodeType should not be nil")
		assert.Equal(ts.T(), "t1", *describedIndex.Spec.Serverless.ReadCapacity.Dedicated.NodeType, "NodeType should be t1")
		// Check status - it may be Ready, Scaling, or Migrating
		status := describedIndex.Spec.Serverless.ReadCapacity.Dedicated.Status.State
		assert.Contains(ts.T(), []string{"Ready", "Scaling", "Migrating"}, status, "ReadCapacity status should be Ready, Scaling, or Migrating")
		return nil
	})

	// Configure dedicated configuration - bump shards to 2
	readCapacity = &ReadCapacityParams{
		Dedicated: &ReadCapacityDedicatedConfig{
			Scaling: &ReadCapacityScaling{
				Manual: &ReadCapacityManualScaling{
					Shards: ptr(int32(2)),
				},
			},
		},
	}

	// Verify ReadCapacity was updated to dedicated with new shards
	retryAssertionsWithDefaults(ts.T(), func() error {
		configuredIndex, err := ts.client.ConfigureIndex(ctx, indexName, ConfigureIndexParams{
			ReadCapacity: readCapacity,
		})
		if err != nil {
			return fmt.Errorf("ConfigureIndex failed: %v", err)
		}
		assert.NoError(ts.T(), err)
		assert.NotNil(ts.T(), configuredIndex)
		assert.Equal(ts.T(), ptr(int32(2)), configuredIndex.Spec.Serverless.ReadCapacity.Dedicated.Scaling.Manual.Shards)
		return nil
	})
}

func (ts *integrationTests) TestListAndDescribeIndexBackups() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Skipping backup tests for non-serverless indexes")
	}
	// CreateBackup and DeleteBackup are tested as a part of integrationTests.SetupSuite(), so not explicitly tested here
	limit := 5

	// list project backups
	backups, err := ts.client.ListBackups(context.Background(), &ListBackupsParams{Limit: &limit})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), backups, "Expected backups to be non-nil")

	// list index backups
	indexBackups, err := ts.client.ListBackups(context.Background(), &ListBackupsParams{
		IndexName: &ts.idxName,
		Limit:     &limit,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), indexBackups, "Expected index backups to be non-nil")
	if len(indexBackups.Data) > 0 {
		require.Equal(ts.T(), ts.idxName, indexBackups.Data[0].SourceIndexName, "Expected index backup to match index name")
	}
}

func (ts *integrationTests) TestCreateIndexFromBackupViaRestore() {
	if ts.indexType != "serverless" {
		ts.T().Skip("Skipping backup tests for non-serverless indexes")
	}
	limit := 5
	restoredIndexName := ts.idxName + "-from-backup"
	restoredIndexTags := IndexTags{"status": "integration-test", "type": "backup-restore"}
	createIndexFromBackupResp, err := ts.client.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{
		BackupId: ts.backupId,
		Name:     restoredIndexName,
		Tags:     &restoredIndexTags,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), createIndexFromBackupResp, "Expected CreateIndexFromBackup response to be non-nil")

	// validate describing restore job
	restoreJob, err := ts.client.DescribeRestoreJob(context.Background(), createIndexFromBackupResp.RestoreJobId)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), createIndexFromBackupResp.RestoreJobId, restoreJob.RestoreJobId, "Expected restore job ID to match")
	require.Equal(ts.T(), restoredIndexName, restoreJob.TargetIndexName)

	// validate listing restore jobs
	restoreJobs, err := ts.client.ListRestoreJobs(context.Background(), &ListRestoreJobsParams{Limit: &limit})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), restoreJobs, "Expected restore jobs to be non-nil")

	// wait until restore job completes
	maxRetries := 5
	for restoreJob.CompletedAt != nil || maxRetries > 0 {
		time.Sleep(2 * time.Second)
		restoreJob, err = ts.client.DescribeRestoreJob(context.Background(), createIndexFromBackupResp.RestoreJobId)
		require.NoError(ts.T(), err)
		maxRetries--
	}

	// validate describing the restored index
	index, err := ts.client.DescribeIndex(context.Background(), restoredIndexName)
	defer func(ts *integrationTests, name string) {
		err := ts.client.DeleteIndex(context.Background(), name)
		require.NoError(ts.T(), err)
	}(ts, restoredIndexName)

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), index, "Expected restored index to be non-nil")
	require.Equal(ts.T(), restoredIndexName, index.Name, "Expected restored index name to match")
	require.Equal(ts.T(), restoredIndexTags, *index.Tags, "Expected restored index tags to match")
}

// Unit tests:
func TestExtractAuthHeaderUnit(t *testing.T) {
	globalApiKey := os.Getenv("PINECONE_API_KEY")
	os.Unsetenv("PINECONE_API_KEY")

	// Passing an API key should result in an 'Api-Key' header
	apiKey := "test-api-key"
	expectedHeader := map[string]string{"Api-Key": apiKey}
	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	if err != nil {
		t.Error(err.Error())
	}
	assert.Equal(t,
		expectedHeader,
		client.extractAuthHeader(),
		"Expected client.extractAuthHeader to return %v but got '%s'", expectedHeader, client.extractAuthHeader(),
	)

	// Passing a custom auth header with "authorization" should be returned as is
	expectedHeader = map[string]string{"Authorization": "Bearer test-token-123456"}
	client, err = NewClientBase(NewClientBaseParams{Headers: expectedHeader})
	if err != nil {
		t.Error(err.Error())
	}
	assert.Equal(t,
		expectedHeader,
		client.extractAuthHeader(),
		"Expected client.extractAuthHeader to return %v but got '%s'", expectedHeader, client.extractAuthHeader(),
	)

	// Passing a custom auth header with "access_token" should be returned as is
	expectedHeader = map[string]string{"access_token": "test-token-123456"}
	client, err = NewClientBase(NewClientBaseParams{Headers: expectedHeader})
	if err != nil {
		t.Error(err.Error())
	}
	assert.Equal(t,
		expectedHeader,
		client.extractAuthHeader(),
		"Expected client.extractAuthHeader to return %v but got '%s'", expectedHeader, client.extractAuthHeader(),
	)

	os.Setenv("PINECONE_API_KEY", globalApiKey)
}

func TestApiKeyPassedToIndexConnectionUnit(t *testing.T) {
	apiKey := "test-api-key"

	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	if err != nil {
		t.Error(err.Error())
	}

	indexConn, err := client.Index(NewIndexConnParams{Host: "my-index-host.io"})
	if err != nil {
		t.Error(err.Error())
	}

	indexMetadata := indexConn.additionalMetadata
	metadataHasApiKey := false
	for key, value := range indexMetadata {
		if key == "Api-Key" && value == apiKey {
			metadataHasApiKey = true
			break
		}
	}

	assert.True(t, metadataHasApiKey, "Expected IndexConnection metadata to contain 'Api-Key' with value '%s'", apiKey)
}

func TestNewClientParamsSetUnit(t *testing.T) {
	apiKey := "test-api-key"
	client, err := NewClient(NewClientParams{ApiKey: apiKey})

	require.NoError(t, err)
	require.Empty(t, client.baseParams.SourceTag, "Expected client to have empty sourceTag")
	require.NotNil(t, client.baseParams.Headers, "Expected client headers to not be nil")
	apiKeyHeader, ok := client.baseParams.Headers["Api-Key"]
	require.True(t, ok, "Expected client to have an 'Api-Key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(t, 3, len(client.restClient.RequestEditors), "Expected client to have correct number of request editors")
}

func TestNewClientParamsSetSourceTagUnit(t *testing.T) {
	apiKey := "test-api-key"
	sourceTag := "test-source-tag"
	client, err := NewClient(NewClientParams{
		ApiKey:    apiKey,
		SourceTag: sourceTag,
	})

	require.NoError(t, err)
	apiKeyHeader, ok := client.baseParams.Headers["Api-Key"]
	require.True(t, ok, "Expected client to have an 'Api-Key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(t, sourceTag, client.baseParams.SourceTag, "Expected client to have sourceTag '%s', but got '%s'", sourceTag, client.baseParams.SourceTag)
	require.Equal(t, 3, len(client.restClient.RequestEditors), "Expected client to have %s request editors, but got %s", 2, len(client.restClient.RequestEditors))
}

func TestNewClientParamsSetHeadersUnit(t *testing.T) {
	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "test-ptr"}
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers})

	require.NoError(t, err)
	apiKeyHeader, ok := client.baseParams.Headers["Api-Key"]
	require.True(t, ok, "Expected client to have an 'Api-Key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(t, client.baseParams.Headers, headers, "Expected client to have headers '%+v', but got '%+v'", headers, client.baseParams.Headers)
	require.Equal(t, 4, len(client.restClient.RequestEditors), "Expected client to have %s request editors, but got %s", 3, len(client.restClient.RequestEditors))
}

func TestNewClientParamsNoApiKeyNoAuthorizationHeaderUnit(t *testing.T) {
	apiKey := os.Getenv("PINECONE_API_KEY")
	os.Unsetenv("PINECONE_API_KEY")

	client, err := NewClient(NewClientParams{})
	require.NotNil(t, err, "Expected error when creating client without an API key or Authorization header")
	if !strings.Contains(err.Error(), "no API key provided, please pass an API key for authorization") {
		t.Errorf("Expected error to contain 'no API key provided, please pass an API key for authorization', but got '%s'", err.Error())
	}

	require.Nil(t, client, "Expected client to be nil when creating client without an API key or Authorization header")

	os.Setenv("PINECONE_API_KEY", apiKey)
}

func TestHeadersAppliedToRequestsUnit(t *testing.T) {
	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "123456"}

	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers, RestClient: httpClient})
	if err != nil {
		t.Error(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(t, err)
	require.NotNil(t, mockTransport.Req, "Expected request to be made")

	testHeaderValue := mockTransport.Req.Header.Get("test-header")
	assert.Equal(t, "123456", testHeaderValue, "Expected request to have header ptr '123456', but got '%s'", testHeaderValue)
}

func TestAdditionalHeadersAppliedToRequestUnit(t *testing.T) {
	os.Setenv("PINECONE_ADDITIONAL_HEADERS", `{"test-header": "environment-header"}`)

	apiKey := "test-api-key"

	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, RestClient: httpClient})
	if err != nil {
		t.Error(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(t, err)
	require.NotNil(t, mockTransport.Req, "Expected request to be made")

	testHeaderValue := mockTransport.Req.Header.Get("test-header")
	assert.Equal(t, "environment-header", testHeaderValue, "Expected request to have header ptr 'environment-header', but got '%s'", testHeaderValue)

	os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
}

func TestHeadersOverrideAdditionalHeadersUnit(t *testing.T) {
	os.Setenv("PINECONE_ADDITIONAL_HEADERS", `{"test-header": "environment-header"}`)

	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "param-header"}

	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers, RestClient: httpClient})
	if err != nil {
		t.Error(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(t, err)
	require.NotNil(t, mockTransport.Req, "Expected request to be made")

	testHeaderValue := mockTransport.Req.Header.Get("test-header")
	assert.Equal(t, "param-header", testHeaderValue, "Expected request to have header ptr 'param-header', but got '%s'", testHeaderValue)

	os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
}

func TestControllerHostOverrideUnit(t *testing.T) {
	apiKey := "test-api-key"
	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Host: "https://test-controller-host.io", RestClient: httpClient})
	if err != nil {
		t.Error(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(t, err)
	require.NotNil(t, mockTransport.Req, "Expected request to be made")
	assert.Equal(t, "test-controller-host.io", mockTransport.Req.Host, "Expected request to be made to 'test-controller-host.io', but got '%s'", mockTransport.Req.URL.Host)
}

func TestControllerHostOverrideFromEnvUnit(t *testing.T) {
	os.Setenv("PINECONE_CONTROLLER_HOST", "https://env-controller-host.io")

	apiKey := "test-api-key"
	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, RestClient: httpClient})
	if err != nil {
		t.Error(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(t, err)
	require.NotNil(t, mockTransport.Req, "Expected request to be made")
	assert.Equal(t, "env-controller-host.io", mockTransport.Req.Host, "Expected request to be made to 'env-controller-host.io', but got '%s'", mockTransport.Req.URL.Host)

	os.Unsetenv("PINECONE_CONTROLLER_HOST")
}

func TestControllerHostNormalizationUnit(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		wantHost   string
		wantScheme string
	}{
		{
			name:       "Test with https prefix",
			host:       "https://pinecone-api.io",
			wantHost:   "pinecone-api.io",
			wantScheme: "https",
		}, {
			name:       "Test with http prefix",
			host:       "http://pinecone-api.io",
			wantHost:   "pinecone-api.io",
			wantScheme: "http",
		}, {
			name:       "Test without prefix",
			host:       "pinecone-api.io",
			wantHost:   "pinecone-api.io",
			wantScheme: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey := "test-api-key"
			httpClient := utils.CreateMockClient(`{"indexes": []}`)
			client, err := NewClient(NewClientParams{ApiKey: apiKey, Host: tt.host, RestClient: httpClient})
			if err != nil {
				t.Error(err.Error())
			}
			mockTransport := httpClient.Transport.(*utils.MockTransport)

			_, err = client.ListIndexes(context.Background())
			require.NoError(t, err)
			require.NotNil(t, mockTransport.Req, "Expected request to be made")

			assert.Equal(t, tt.wantHost, mockTransport.Req.URL.Host, "Expected request to be made to host '%s', but got '%s'", tt.wantHost, mockTransport.Req.URL.Host)
			assert.Equal(t, tt.wantScheme, mockTransport.Req.URL.Scheme, "Expected request to be made to host '%s, but got '%s'", tt.wantScheme, mockTransport.Req.URL.Host)
		})
	}
}

func TestIndexConnectionMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.Index(NewIndexConnParams{})
	require.ErrorContainsf(t, err, "field Host is required", err.Error())
}

func TestCreatePodIndexMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{})
	require.Error(t, err)
	require.ErrorContainsf(t, err, "fields Name, positive Dimension, Environment, and Podtype must be included in CreatePodIndexRequest", err.Error())
}

func TestCreateServerlessIndexMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{})
	require.Error(t, err)
	require.ErrorContainsf(t, err, "fields Name, Cloud, and Region must be included in CreateServerlessIndexRequest", err.Error())
}

func TestCreateServerlessIndexInvalidSparseDimensionUnit(t *testing.T) {
	vectorType := "sparse"
	dimension := int32(1)
	metric := Dotproduct
	client := &Client{}
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:       "test-invalid-dimension",
		Metric:     &metric,
		Cloud:      "aws",
		Region:     "us-east-1",
		Dimension:  &dimension,
		VectorType: &vectorType,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "Dimension should not be specified when VectorType is 'sparse'")
}

func TestCreateServerlessIndexInvalidSparseMetricUnit(t *testing.T) {
	vectorType := "sparse"
	metric := Cosine
	client := &Client{}
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:       "test-invalid-dimension",
		Metric:     &metric,
		Cloud:      "aws",
		Region:     "us-east-1",
		VectorType: &vectorType,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "Metric should be 'dotproduct' when VectorType is 'sparse'")
}

func TestCreateServerlessIndexInvalidDenseDimensionUnit(t *testing.T) {
	vectorType := "dense"
	metric := Cosine
	client := &Client{}
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:       "test-invalid-dimension",
		Metric:     &metric,
		Cloud:      "aws",
		Region:     "us-east-1",
		VectorType: &vectorType,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "Dimension should be specified when VectorType is 'dense'")
}

func TestCreatePodIndexInvalidDimensionUnit(t *testing.T) {
	metric := Cosine
	client := &Client{}
	_, err := client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        "test-invalid-dimension",
		Dimension:   -1,
		Metric:      &metric,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "fields Name, positive Dimension, Environment, and Podtype must be included in CreatePodIndexRequest")
}

func TestCreateCollectionMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.CreateCollection(context.Background(), &CreateCollectionRequest{})
	require.Error(t, err)
	require.ErrorContains(t, err, "fields Name and Source must be included in CreateCollectionRequest")
}

func TestHandleErrorResponseBodyUnit(t *testing.T) {
	tests := []struct {
		name         string
		responseBody *http.Response
		statusCode   int
		prefix       string
		errorOutput  string
	}{
		{
			name:         "test ErrorResponse body",
			responseBody: mockResponse(`{"error": { "code": "INVALID_ARGUMENT", "message": "test error message"}, "status": 400}`, http.StatusBadRequest),
			statusCode:   http.StatusBadRequest,
			errorOutput:  `{"status_code":400,"body":"{\"error\": { \"code\": \"INVALID_ARGUMENT\", \"message\": \"test error message\"}, \"status\": 400}","error_code":"INVALID_ARGUMENT","message":"test error message"}`,
		}, {
			name:         "test JSON body",
			responseBody: mockResponse(`{"message": "test error message", "extraCode": 665}`, http.StatusBadRequest),
			statusCode:   http.StatusBadRequest,
			errorOutput:  `{"status_code":400,"body":"{\"message\": \"test error message\", \"extraCode\": 665}"}`,
		}, {
			name:         "test string body",
			responseBody: mockResponse(`test error message`, http.StatusBadRequest),
			statusCode:   http.StatusBadRequest,
			errorOutput:  `{"status_code":400,"body":"test error message"}`,
		}, {
			name:         "Test error response with empty response",
			responseBody: mockResponse(`{}`, http.StatusBadRequest),
			statusCode:   http.StatusBadRequest,
			prefix:       "test prefix",
			errorOutput:  `{"status_code":400,"body":"{}"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleErrorResponseBody(tt.responseBody, tt.prefix)
			assert.Equal(t, err.Error(), tt.errorOutput, "Expected error to be '%s', but got '%s'", tt.errorOutput, err.Error())

		})
	}
}

func TestFormatErrorUnit(t *testing.T) {
	tests := []struct {
		name     string
		err      int
		expected *PineconeError
	}{
		{
			name: "Confirm error message is formatted as expected",
			err:  202,
			expected: &PineconeError{
				Code: 202,
				Msg:  fmt.Errorf(`{"status_code":202}`)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := errorResponseMap{
				StatusCode: tt.err,
			}
			result := formatError(req)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}

}

func TestValueOrFallBackUnit(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		expected string
	}{
		{
			name:     "Confirm ptr is returned",
			value:    "test-ptr",
			fallback: "fallback-ptr",
			expected: "test-ptr",
		}, {
			name:     "Confirm fallback is returned",
			value:    "",
			fallback: "fallback-ptr",
			expected: "fallback-ptr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueOrFallback(tt.value, tt.fallback)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestMinOneUnit(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{
			name:     "Confirm positive ptr if input is positive",
			value:    5,
			expected: 5,
		}, {
			name:     "Confirm coercion to 1 if input is zero",
			value:    0,
			expected: 1,
		}, {
			name:     "Confirm coercion to 1 if input is negative",
			value:    -5,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := minOne(int32(tt.value))
			assert.Equal(t, int32(tt.expected), result, "Expected result to be '%d', but got '%d'", tt.expected, result)
		})
	}

}

func TestTotalCountUnit(t *testing.T) {
	tests := []struct {
		name           string
		replicaCount   int32
		shardCount     int32
		expectedResult int
	}{
		{
			name:           "Confirm correct multiplication if all values are >0",
			replicaCount:   2,
			shardCount:     3,
			expectedResult: 6,
		}, {
			name:           "Confirm ptr of 0 get ignored in calculation",
			replicaCount:   0,
			shardCount:     5,
			expectedResult: 5,
		},
		{
			name:           "Confirm negative ptr gets ignored in calculation",
			replicaCount:   -2,
			shardCount:     3,
			expectedResult: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreatePodIndexRequest{
				Replicas: tt.replicaCount,
				Shards:   tt.shardCount,
			}
			result := req.TotalCount()
			assert.Equal(t, tt.expectedResult, result, "Expected result to be '%d', but got '%d'", tt.expectedResult, result)
		})
	}
}

func TestEnsureURLSchemeUnit(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Confirm https prefix is added",
			url:      "pinecone-api.io",
			expected: "https://pinecone-api.io",
		}, {
			name:     "Confirm http prefix is added",
			url:      "http://pinecone-api.io",
			expected: "http://pinecone-api.io",
		},
		{
			name:     "Confirm https prefix is added",
			url:      "https://pinecone-api.io",
			expected: "https://pinecone-api.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ensureURLScheme(tt.url)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}

}

func TestToIndexUnit(t *testing.T) {
	deletionProtectionEnabled := "enabled"
	deletionProtectionDisabled := "disabled"
	pods := 1
	replicas := int32(1)
	shards := int32(1)
	dimension := int32(128)

	tests := []struct {
		name           string
		originalInput  *db_control.IndexModel
		expectedOutput *Index
	}{
		{
			name:           "nil input",
			originalInput:  nil,
			expectedOutput: nil,
		},
		{
			name: "pod index input",
			originalInput: &db_control.IndexModel{
				Name:               "testIndex",
				Dimension:          &dimension,
				Host:               "test-host",
				Metric:             "cosine",
				DeletionProtection: &deletionProtectionDisabled,
				Spec: newPodIndexModelSpec(t, db_control.IndexModelSpec1{
					Pod: db_control.PodSpec{
						Environment:      "test-environ",
						PodType:          "p1.x2",
						Pods:             &pods,
						Replicas:         &replicas,
						Shards:           &shards,
						SourceCollection: nil,
						MetadataConfig:   nil,
					},
				}),
				Status: struct {
					Ready bool   `json:"ready"`
					State string `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:               "testIndex",
				Dimension:          &dimension,
				Host:               "test-host",
				Metric:             "cosine",
				DeletionProtection: "disabled",
				Spec: &IndexSpec{
					Pod: &PodSpec{
						Environment:      "test-environ",
						PodType:          "p1.x2",
						PodCount:         1,
						Replicas:         1,
						ShardCount:       1,
						SourceCollection: nil,
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("active"),
				},
			},
		},
		{
			name: "serverless index input",
			originalInput: &db_control.IndexModel{
				Name:               "testIndex",
				Dimension:          &dimension,
				Host:               "test-host",
				Metric:             "cosine",
				DeletionProtection: &deletionProtectionEnabled,
				Spec: newServerlessIndexModelSpec(t, db_control.IndexModelSpec0{
					Serverless: db_control.ServerlessSpecResponse{
						Cloud:  "test-environ",
						Region: "test-region",
					},
				}),
				Status: struct {
					Ready bool   `json:"ready"`
					State string `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:               "testIndex",
				Dimension:          &dimension,
				Host:               "test-host",
				Metric:             "cosine",
				DeletionProtection: "enabled",
				Spec: &IndexSpec{
					Serverless: &ServerlessSpec{
						Cloud:  Cloud("test-environ"),
						Region: "test-region",
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("active"),
				},
			},
		},
		{
			name: "byoc index input",
			originalInput: &db_control.IndexModel{
				Name:               "testIndex",
				Dimension:          &dimension,
				Host:               "test-host",
				PrivateHost:        ptr("test-private-host"),
				Metric:             "cosine",
				DeletionProtection: &deletionProtectionEnabled,
				Spec: newByocIndexModelSpec(t, db_control.IndexModelSpec2{
					Byoc: db_control.ByocSpec{
						Environment: "test-environ",
						Schema:      nil,
					},
				}),
				Status: struct {
					Ready bool   `json:"ready"`
					State string `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:               "testIndex",
				Dimension:          &dimension,
				Host:               "test-host",
				PrivateHost:        ptr("test-private-host"),
				Metric:             "cosine",
				DeletionProtection: "enabled",
				Spec: &IndexSpec{
					BYOC: &BYOCSpec{
						Environment: "test-environ",
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("active"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := toIndex(tt.originalInput)
			if err != nil {
				t.Errorf("toIndex() error: %v", err)
			}
			if diff := cmp.Diff(tt.expectedOutput, output); diff != "" {
				t.Errorf("toIndex() mismatch (-expectedOutput +input):\n%s", diff)
			}
			assert.EqualValues(t, tt.expectedOutput, output)
		})
	}
}

func TestToCollectionUnit(t *testing.T) {
	size := int64(100)
	dimension := int32(128)
	vectorCount := int32(1000)

	tests := []struct {
		name           string
		originalInput  *db_control.CollectionModel
		expectedOutput *Collection
	}{
		{
			name:           "nil input",
			originalInput:  nil,
			expectedOutput: nil,
		},
		{
			name: "collection input",
			originalInput: &db_control.CollectionModel{
				Dimension:   &dimension,
				Name:        "testCollection",
				Environment: "test-environ",
				Size:        &size,
				VectorCount: &vectorCount,
				Status:      "active",
			},
			expectedOutput: &Collection{
				Name:        "testCollection",
				Size:        size,
				Status:      "active",
				Dimension:   128,
				VectorCount: vectorCount,
				Environment: "test-environ",
			},
		},
		{
			name: "collection input",
			originalInput: &db_control.CollectionModel{
				Dimension:   &dimension,
				Name:        "testCollection",
				Environment: "test-environ",
				Size:        &size,
				VectorCount: &vectorCount,
				Status:      "active",
			},
			expectedOutput: &Collection{
				Name:        "testCollection",
				Size:        size,
				Status:      "active",
				Dimension:   128,
				VectorCount: vectorCount,
				Environment: "test-environ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := toCollection(tt.originalInput)
			if diff := cmp.Diff(tt.expectedOutput, input); diff != "" {
				t.Errorf("toCollection() mismatch (-expectedOutput +input):\n%s", diff)
			}
			assert.EqualValues(t, tt.expectedOutput, input)
		})
	}
}

func TestDerefOrDefaultUnit(t *testing.T) {
	tests := []struct {
		name         string
		ptr          any
		defaultValue any
		expected     any
	}{
		{
			name:         "Confirm defaultValue is returned when ptr is nil",
			ptr:          nil,
			defaultValue: "fallback-ptr",
			expected:     "fallback-ptr",
		}, {
			name:         "Confirm ptr is returned when provided (string)",
			ptr:          "some provided ptr",
			defaultValue: "fallback-ptr",
			expected:     "some provided ptr",
		},
		{
			name:         "Confirm ptr is returned when provided (int)",
			ptr:          78,
			defaultValue: 92,
			expected:     78,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueOrFallback(tt.ptr, tt.defaultValue)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestNewClientUnit(t *testing.T) {
	testCases := []struct {
		name            string
		apiKey          string
		headers         map[string]string
		expectedHeaders map[string]string
		expectedErr     bool
	}{
		{
			name:   "Custom headers provided",
			apiKey: "test-api-key",
			headers: map[string]string{
				"Test-Header": "custom-header-value",
			},
			expectedHeaders: map[string]string{
				"Api-Key":     "test-api-key",
				"Test-Header": "custom-header-value",
			},
			expectedErr: false,
		},
		{
			name:            "No headers provided",
			apiKey:          "test-api-key",
			headers:         nil,
			expectedHeaders: map[string]string{"Api-Key": "test-api-key"},
			expectedErr:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockNewClientParams := NewClientParams{
				ApiKey:  tc.apiKey,
				Headers: tc.headers,
			}

			client, err := NewClient(mockNewClientParams)

			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tc.expectedHeaders, client.baseParams.Headers, "Expected headers to be '%v', but got '%v'", tc.expectedHeaders, client.baseParams.Headers)
			}
		})
	}
}

func TestNewClientBaseUnit(t *testing.T) {
	// Save the current environment variable value and defer restoring it
	originalHostEnv := os.Getenv("PINECONE_CONTROLLER_HOST")
	defer os.Setenv("PINECONE_CONTROLLER_HOST", originalHostEnv)

	testCases := []struct {
		name         string
		host         string
		envHost      string
		expectedHost string
		expectedErr  bool
	}{
		{
			name:         "Host passed in explicitly",
			host:         "https://custom-host.com/",
			envHost:      "",
			expectedHost: "https://custom-host.com/",
			expectedErr:  false,
		},
		{
			name:         "Host taken from environment variable",
			host:         "",
			envHost:      "https://env-host.com/",
			expectedHost: "https://env-host.com/",
			expectedErr:  false,
		},
		{
			name: "Host is not passed explicitly nor is it stored as an environment variable, " +
				"so default host is used",
			host:         "",
			envHost:      "",
			expectedHost: "https://api.pinecone.io/",
			expectedErr:  false,
		},
		{
			name:         "Pass an invalid URL scheme",
			host:         "invalid-host			", // invalid b/c tab chars in url
			envHost:      "",
			expectedHost: "",
			expectedErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable for the test case
			os.Setenv("PINECONE_CONTROLLER_HOST", tc.envHost)

			params := NewClientBaseParams{
				Host: tc.host,
			}
			client, err := NewClientBase(params)

			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				if tc.expectedHost != "" {
					assert.Equal(t, tc.expectedHost, client.restClient.Server)
				}
			}
		})
	}
}

func TestBuildClientBaseOptionsUnit(t *testing.T) {
	tests := []struct {
		name           string
		params         NewClientBaseParams
		envHeaders     string
		expect         []db_control.ClientOption
		expectEnvUnset bool
	}{
		{
			name: "Construct base params without additional env headers present",
			params: NewClientBaseParams{
				SourceTag: "source-tag",
				Headers:   map[string]string{"Param-Header": "param-value"},
			},
			expect: []db_control.ClientOption{
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("User-Agent", "test-user-agent").Intercept),
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("X-Pinecone-Api-Version", gen.PineconeApiVersion).Intercept),
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("Param-Header", "param-value").Intercept),
			},
			expectEnvUnset: true,
		},
		{
			name: "Construct base params with additional env headers present",
			params: NewClientBaseParams{
				SourceTag: "source-tag",
				Headers:   map[string]string{"Param-Header": "param-value"},
			},
			envHeaders: `{"Env-Header": "env-value"}`,
			expect: []db_control.ClientOption{
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("Env-Header", "env-value").Intercept),
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("X-Pinecone-Api-Version", gen.PineconeApiVersion).Intercept),
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("User-Agent", "test-user-agent").Intercept),
				db_control.WithRequestEditorFn(provider.NewHeaderProvider("Param-Header", "param-value").Intercept),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envHeaders != "" {
				os.Setenv("PINECONE_ADDITIONAL_HEADERS", tt.envHeaders)
				defer os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
			}

			clientOptions := buildClientBaseOptions(tt.params)
			assert.Equal(t, len(tt.expect), len(clientOptions))

			for i, opt := range tt.expect {
				assert.IsType(t, opt, clientOptions[i])
			}
		})
	}
}

func TestEnsureHostHasHttpsUnit(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "Host with https prefix should remain unchanged",
			host:     "https://test-abcd.pinecone.io",
			expected: "https://test-abcd.pinecone.io",
		},
		{
			name:     "Host with http prefix should be converted to https",
			host:     "http://test-abcd.pinecone.io",
			expected: "https://test-abcd.pinecone.io",
		},
		{
			name:     "Host without prefix should get https prefix",
			host:     "test-abcd.pinecone.io",
			expected: "https://test-abcd.pinecone.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureHostHasHttps(tt.host)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func Test_toMetadataSchemaFromRest_Unit(t *testing.T) {
	// utility type for the inline representation of MetadataSchema in the REST API
	type restMetadataSchemaInput = struct {
		Fields map[string]struct {
			Filterable *bool `json:"filterable,omitempty"`
		} `json:"fields"`
	}

	tests := []struct {
		name     string
		input    *restMetadataSchemaInput
		expected *MetadataSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty fields map",
			input: &restMetadataSchemaInput{
				Fields: make(map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}),
			},
			expected: &MetadataSchema{
				Fields: make(map[string]MetadataSchemaField),
			},
		},
		{
			name: "fields with filterable true",
			input: &restMetadataSchemaInput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre": {Filterable: ptr(true)},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: true},
				},
			},
		},
		{
			name: "fields with filterable false",
			input: &restMetadataSchemaInput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre": {Filterable: ptr(false)},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: false},
				},
			},
		},
		{
			name: "fields with filterable nil (defaults to false)",
			input: &restMetadataSchemaInput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre": {Filterable: nil},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: false},
				},
			},
		},
		{
			name: "multiple fields",
			input: &restMetadataSchemaInput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre":  {Filterable: ptr(true)},
					"year":   {Filterable: ptr(true)},
					"rating": {Filterable: ptr(false)},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre":  {Filterable: true},
					"year":   {Filterable: true},
					"rating": {Filterable: false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toMetadataSchemaFromRest(tt.input)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("toMetadataSchemaRest() mismatch (-expected +result):\n%s", diff)
			}
		})
	}
}

func Test_fromMetadataSchemaToRest_Unit(t *testing.T) {
	// utility type for the inline representation of MetadataSchema in the REST API
	type restMetadataSchemaOutput = struct {
		Fields map[string]struct {
			Filterable *bool `json:"filterable,omitempty"`
		} `json:"fields"`
	}

	tests := []struct {
		name     string
		input    *MetadataSchema
		expected *restMetadataSchemaOutput
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty fields map",
			input: &MetadataSchema{
				Fields: make(map[string]MetadataSchemaField),
			},
			expected: &restMetadataSchemaOutput{
				Fields: make(map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}),
			},
		},
		{
			name: "fields with filterable true",
			input: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: true},
				},
			},
			expected: &restMetadataSchemaOutput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre": {Filterable: ptr(true)},
				},
			},
		},
		{
			name: "fields with filterable false",
			input: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: false},
				},
			},
			expected: &restMetadataSchemaOutput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre": {Filterable: ptr(false)},
				},
			},
		},
		{
			name: "multiple fields",
			input: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre":  {Filterable: true},
					"year":   {Filterable: true},
					"rating": {Filterable: false},
				},
			},
			expected: &restMetadataSchemaOutput{
				Fields: map[string]struct {
					Filterable *bool `json:"filterable,omitempty"`
				}{
					"genre":  {Filterable: ptr(true)},
					"year":   {Filterable: ptr(true)},
					"rating": {Filterable: ptr(false)},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromMetadataSchemaToRest(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, len(tt.expected.Fields), len(result.Fields))

			for key, expectedField := range tt.expected.Fields {
				actualField, ok := result.Fields[key]
				require.True(t, ok, "Field %s should exist", key)

				if expectedField.Filterable == nil {
					assert.Nil(t, actualField.Filterable)
				} else {
					require.NotNil(t, actualField.Filterable)
					assert.Equal(t, *expectedField.Filterable, *actualField.Filterable)
				}
			}
		})
	}
}

func Test_readCapacityParamsToReadCapacity_Unit(t *testing.T) {
	tests := []struct {
		name      string
		input     *ReadCapacityParams
		wantError bool
		validate  func(t *testing.T, result *db_control.ReadCapacity)
	}{
		{
			name:      "nil request should return nil (defaults to OnDemand)",
			input:     nil,
			wantError: false,
			validate: func(t *testing.T, result *db_control.ReadCapacity) {
				require.Nil(t, result)
			},
		},
		{
			name: "Dedicated with NodeType only",
			input: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("t1"),
				},
			},
			wantError: false,
			validate: func(t *testing.T, result *db_control.ReadCapacity) {
				require.NotNil(t, result)
				mode, err := result.Discriminator()

				require.NoError(t, err)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, err := result.AsReadCapacityDedicatedSpec()
				require.NoError(t, err)
				assert.Equal(t, "t1", *dedicatedSpec.Dedicated.NodeType)
			},
		},
		{
			name: "Dedicated with NodeType and Manual scaling",
			input: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("b1"),
					Scaling: &ReadCapacityScaling{
						Manual: &ReadCapacityManualScaling{
							Replicas: ptr(int32(2)),
							Shards:   ptr(int32(3)),
						},
					},
				},
			},
			wantError: false,
			validate: func(t *testing.T, result *db_control.ReadCapacity) {
				require.NotNil(t, result)
				mode, err := result.Discriminator()

				require.NoError(t, err)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, err := result.AsReadCapacityDedicatedSpec()
				require.NoError(t, err)
				assert.Equal(t, "b1", *dedicatedSpec.Dedicated.NodeType)
				require.NotNil(t, dedicatedSpec.Dedicated.Manual)
				assert.Equal(t, int32(2), *dedicatedSpec.Dedicated.Manual.Replicas)
				assert.Equal(t, int32(3), *dedicatedSpec.Dedicated.Manual.Shards)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := readCapacityParamsToReadCapacity(tt.input)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				tt.validate(t, result)
			}
		})
	}
}

func Test_patchReadCapacity_Unit(t *testing.T) {
	onDemandOld := &ReadCapacity{
		OnDemand: &ReadCapacityOnDemand{
			Status: ReadCapacityStatus{State: "Ready"},
		},
	}
	dedicatedOld := &ReadCapacity{
		Dedicated: &ReadCapacityDedicated{
			NodeType: ptr("t1"),
			Scaling: &ReadCapacityScaling{
				Manual: &ReadCapacityManualScaling{
					Replicas: ptr(int32(1)),
					Shards:   ptr(int32(1)),
				},
			},
			Status: ReadCapacityStatus{State: "Ready"},
		},
	}

	tests := []struct {
		name      string
		newParams *ReadCapacityParams
		oldConfig *ReadCapacity
		validate  func(t *testing.T, result *db_control.ReadCapacity, err error)
	}{
		{
			name:      "nil new params should return nil",
			newParams: nil,
			oldConfig: onDemandOld,
			validate: func(t *testing.T, result *db_control.ReadCapacity, err error) {
				require.NoError(t, err)
				assert.Nil(t, result)
			},
		},
		{
			name: "on-demand to dedicated without manual scaling should error",
			newParams: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("b1"),
				},
			},
			oldConfig: onDemandOld,
			validate: func(t *testing.T, result *db_control.ReadCapacity, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		},
		{
			name: "on-demand to dedicated with full manual scaling succeeds",
			newParams: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("t1"),
					Scaling: &ReadCapacityScaling{
						Manual: &ReadCapacityManualScaling{
							Replicas: ptr(int32(2)),
							Shards:   ptr(int32(3)),
						},
					},
				},
			},
			oldConfig: onDemandOld,
			validate: func(t *testing.T, result *db_control.ReadCapacity, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				mode, modeErr := result.Discriminator()
				require.NoError(t, modeErr)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, specErr := result.AsReadCapacityDedicatedSpec()
				require.NoError(t, specErr)
				require.NotNil(t, dedicatedSpec.Dedicated.NodeType)
				assert.Equal(t, "t1", *dedicatedSpec.Dedicated.NodeType)
				require.NotNil(t, dedicatedSpec.Dedicated.Manual)
				assert.Equal(t, int32(2), *dedicatedSpec.Dedicated.Manual.Replicas)
				assert.Equal(t, int32(3), *dedicatedSpec.Dedicated.Manual.Shards)
			},
		},
		{
			name: "dedicated to dedicated should succeed with single value patch",
			newParams: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					Scaling: &ReadCapacityScaling{
						Manual: &ReadCapacityManualScaling{
							Replicas: ptr(int32(4)),
						},
					},
				},
			},
			oldConfig: dedicatedOld,
			validate: func(t *testing.T, result *db_control.ReadCapacity, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				mode, modeErr := result.Discriminator()
				require.NoError(t, modeErr)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, specErr := result.AsReadCapacityDedicatedSpec()
				require.NoError(t, specErr)
				assert.Nil(t, dedicatedSpec.Dedicated.NodeType)
				require.NotNil(t, dedicatedSpec.Dedicated.Manual)
				assert.Equal(t, int32(4), *dedicatedSpec.Dedicated.Manual.Replicas)
				assert.Nil(t, dedicatedSpec.Dedicated.Manual.Shards)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := patchReadCapacity(tt.newParams, tt.oldConfig)
			tt.validate(t, result, err)
		})
	}
}

func Test_toReadCapacity_Unit(t *testing.T) {
	replicas := int32(2)
	shards := int32(3)
	errorMsg := "test error"

	// utility funcs for creating inputs from generated code types
	onDemandInput := func() *db_control.ReadCapacityResponse {
		var result db_control.ReadCapacityResponse
		onDemandSpec := db_control.ReadCapacityOnDemandSpecResponse{
			Mode: "OnDemand",
			Status: db_control.ReadCapacityStatus{
				State:           "Ready",
				CurrentReplicas: &replicas,
				CurrentShards:   &shards,
				ErrorMessage:    nil,
			},
		}
		err := result.FromReadCapacityOnDemandSpecResponse(onDemandSpec)
		if err != nil {
			panic(err)
		}
		return &result
	}()

	dedicatedNoScalingInput := func() *db_control.ReadCapacityResponse {
		var result db_control.ReadCapacityResponse
		dedicatedSpec := db_control.ReadCapacityDedicatedSpecResponse{
			Mode: "Dedicated",
			Dedicated: db_control.ReadCapacityDedicatedConfig{
				NodeType: ptr("t1"),
				Scaling:  ptr(""),
			},
			Status: db_control.ReadCapacityStatus{
				State:           "Ready",
				CurrentReplicas: &replicas,
				CurrentShards:   &shards,
				ErrorMessage:    nil,
			},
		}
		err := result.FromReadCapacityDedicatedSpecResponse(dedicatedSpec)
		if err != nil {
			panic(err)
		}
		return &result
	}()

	dedicatedWithScalingInput := func() *db_control.ReadCapacityResponse {
		var result db_control.ReadCapacityResponse
		dedicatedSpec := db_control.ReadCapacityDedicatedSpecResponse{
			Mode: "Dedicated",
			Dedicated: db_control.ReadCapacityDedicatedConfig{
				NodeType: ptr("b1"),
				Scaling:  ptr("Manual"),
				Manual: &db_control.ScalingConfigManual{
					Replicas: ptr(int32(2)),
					Shards:   ptr(int32(3)),
				},
			},
			Status: db_control.ReadCapacityStatus{
				State:           "Scaling",
				CurrentReplicas: &replicas,
				CurrentShards:   &shards,
				ErrorMessage:    &errorMsg,
			},
		}
		err := result.FromReadCapacityDedicatedSpecResponse(dedicatedSpec)
		if err != nil {
			panic(err)
		}
		return &result
	}()

	tests := []struct {
		name      string
		input     *db_control.ReadCapacityResponse
		wantError bool
		expected  *ReadCapacity
	}{
		{
			name:      "nil input",
			input:     nil,
			wantError: false,
			expected:  nil,
		},
		{
			name:      "OnDemand mode",
			input:     onDemandInput,
			wantError: false,
			expected: &ReadCapacity{
				OnDemand: &ReadCapacityOnDemand{
					Status: ReadCapacityStatus{
						State:           "Ready",
						CurrentReplicas: &replicas,
						CurrentShards:   &shards,
						ErrorMessage:    nil,
					},
				},
			},
		},
		{
			name:      "Dedicated mode without scaling",
			input:     dedicatedNoScalingInput,
			wantError: false,
			expected: &ReadCapacity{
				Dedicated: &ReadCapacityDedicated{
					NodeType: ptr("t1"),
					Scaling:  nil,
					Status: ReadCapacityStatus{
						State:           "Ready",
						CurrentReplicas: &replicas,
						CurrentShards:   &shards,
						ErrorMessage:    nil,
					},
				},
			},
		},
		{
			name:      "Dedicated mode with manual scaling",
			input:     dedicatedWithScalingInput,
			wantError: false,
			expected: &ReadCapacity{
				Dedicated: &ReadCapacityDedicated{
					NodeType: ptr("b1"),
					Scaling: &ReadCapacityScaling{
						Manual: &ReadCapacityManualScaling{
							Replicas: ptr(int32(2)),
							Shards:   ptr(int32(3)),
						},
					},
					Status: ReadCapacityStatus{
						State:           "Scaling",
						CurrentReplicas: &replicas,
						CurrentShards:   &shards,
						ErrorMessage:    &errorMsg,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toReadCapacity(tt.input)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if diff := cmp.Diff(tt.expected, result); diff != "" {
					t.Errorf("toReadCapacity() mismatch (-expected +result):\n%s", diff)
				}
			}
		})
	}
}

// Helper functions:
func (ts *integrationTests) deleteIndex(name string) error {
	_, err := waitUntilIndexReady(ts, context.Background())
	require.NoError(ts.T(), err)

	return ts.client.DeleteIndex(context.Background(), name)
}

func newServerlessIndexModelSpec(t *testing.T, in db_control.IndexModelSpec0) db_control.IndexModel_Spec {
	spec := db_control.IndexModel_Spec{}
	err := spec.FromIndexModelSpec0(in)
	if err != nil {
		t.Fatalf("Failed to convert serverless IndexModelSpec0 to IndexModel_Spec: %v", err)
	}
	return spec
}

func newPodIndexModelSpec(t *testing.T, in db_control.IndexModelSpec1) db_control.IndexModel_Spec {
	spec := db_control.IndexModel_Spec{}
	err := spec.FromIndexModelSpec1(in)
	if err != nil {
		t.Fatalf("Failed to convert pod IndexModelSpec1 to IndexModel_Spec: %v", err)
	}
	return spec
}

func newByocIndexModelSpec(t *testing.T, in db_control.IndexModelSpec2) db_control.IndexModel_Spec {
	spec := db_control.IndexModel_Spec{}
	err := spec.FromIndexModelSpec2(in)
	if err != nil {
		t.Fatalf("Failed to convert byoc IndexModelSpec2 to IndexModel_Spec: %v", err)
	}
	return spec
}

func ptr[T any](v T) *T { return &v }

func mockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
