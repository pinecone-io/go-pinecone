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
	"github.com/pinecone-io/go-pinecone/internal/gen"
	"github.com/pinecone-io/go-pinecone/internal/gen/db_control"
	"github.com/pinecone-io/go-pinecone/internal/provider"

	"github.com/pinecone-io/go-pinecone/internal/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests:
func (ts *IntegrationTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *IntegrationTests) TestCreatePodIndex() {
	name := uuid.New().String()

	defer func(ts *IntegrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
}

func (ts *IntegrationTests) TestCreatePodIndexInvalidDimension() {
	name := uuid.New().String()

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   -1,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *IntegrationTests) TestCreateServerlessIndexInvalidDimension() {
	name := uuid.New().String()

	_, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: -1,
		Metric:    Cosine,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *IntegrationTests) TestCreateServerlessIndex() {
	name := uuid.New().String()

	defer func(ts *IntegrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: 10,
		Metric:    Cosine,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
}

func (ts *IntegrationTests) TestDescribeIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.idxName)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.idxName, index.Name, "Index name does not match")
}

func (ts *IntegrationTests) TestDescribeNonExistentIndex() {
	_, err := ts.client.DescribeIndex(context.Background(), "non-existent-index")
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *IntegrationTests) TestListCollections() {
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

func (ts *IntegrationTests) TestDescribeCollection() {
	if ts.indexType == "serverless" {
		ts.T().Skip("No pod index to test")
	}
	ctx := context.Background()

	collection, err := ts.client.DescribeCollection(ctx, ts.collectionName)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.collectionName, collection.Name, "Collection name does not match")
}

func (ts *IntegrationTests) TestDeletionProtection() {
	// configure index to enable deletion protection
	index, err := ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{DeletionProtection: "enabled"})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), DeletionProtectionEnabled, index.DeletionProtection, "Expected deletion protection to be 'enabled'")

	// validate we cannot delete the index
	err = ts.client.DeleteIndex(context.Background(), ts.idxName)
	require.ErrorContainsf(ts.T(), err, "failed to delete index: Deletion protection is enabled for this index", err.Error())

	// if testing a pods index, make sure configuring the index without specifying DeletionProtection maintains "enabled" state
	if ts.indexType == "pods" {
		index, err := ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{PodType: "p1.x2"})
		require.NoError(ts.T(), err)
		require.Equal(ts.T(), DeletionProtectionEnabled, index.DeletionProtection, "Expected deletion protection to be 'enabled'")

		// validate we cannot delete the index
		err = ts.client.DeleteIndex(context.Background(), ts.idxName)
		require.ErrorContainsf(ts.T(), err, "failed to delete index: Deletion protection is enabled for this index", err.Error())
	}

	// disable deletion protection so the index can be cleaned up during integration teardown
	_, err = ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{DeletionProtection: "disabled"})
	require.NoError(ts.T(), err)

	// Before moving on to another test, wait for the index to be done upgrading
	_, err = WaitUntilIndexReady(ts, context.Background())
	require.NoError(ts.T(), err)
}

func (ts *IntegrationTests) TestConfigureIndexIllegalScaleDown() {
	name := uuid.New().String()

	defer func(ts *IntegrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{PodType: "p1.x1"})
	require.ErrorContainsf(ts.T(), err, "Cannot scale down", err.Error())
}

func (ts *IntegrationTests) TestConfigureIndexScaleUpNoPods() {
	name := uuid.New().String()

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{Replicas: 2})
	require.NoError(ts.T(), err)

	// give index a bit of time to start upgrading before we poll
	time.Sleep(500 * time.Millisecond)

	isReady, _ := WaitUntilIndexReady(ts, context.Background())
	require.True(ts.T(), isReady, "Expected index to be ready")

	time.Sleep(500 * time.Millisecond)

	err = ts.client.DeleteIndex(context.Background(), name)
	require.NoError(ts.T(), err)
}

func (ts *IntegrationTests) TestConfigureIndexScaleUpNoReplicas() {
	name := uuid.New().String()

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{PodType: "p1.x4"})
	require.NoError(ts.T(), err)

	// give index a bit of time to start upgrading before we poll
	time.Sleep(500 * time.Millisecond)

	isReady, _ := WaitUntilIndexReady(ts, context.Background())
	require.True(ts.T(), isReady, "Expected index to be ready")

	time.Sleep(500 * time.Millisecond)

	err = ts.client.DeleteIndex(context.Background(), name)
	require.NoError(ts.T(), err)
}

func (ts *IntegrationTests) TestConfigureIndexIllegalNoPodsOrReplicasOrDeletionProtection() {
	_, err := ts.client.ConfigureIndex(context.Background(), ts.idxName, ConfigureIndexParams{})
	require.ErrorContainsf(ts.T(), err, "must specify PodType, Replicas, DeletionProtection, or Tags", err.Error())
}

func (ts *IntegrationTests) TestConfigureIndexHitPodLimit() {
	name := uuid.New().String()

	defer func(ts *IntegrationTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   2,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, ConfigureIndexParams{Replicas: 30000})
	require.ErrorContainsf(ts.T(), err, "You've reached the max pods allowed", err.Error())
}

func (ts *IntegrationTests) TestGenerateEmbeddings() {
	// Run Embed tests once rather than duplicating across serverless & pods
	if ts.indexType == "pod" {
		ts.T().Skip("Skipping Embed tests for pods")
	}

	ctx := context.Background()
	embeddingModel := "multilingual-e5-large"
	embeddings, err := ts.client.Inference.Embed(ctx, &EmbedRequest{
		Model: embeddingModel,
		TextInputs: []string{
			"The quick brown fox jumps over the lazy dog",
			"Lorem ipsum",
		},
		Parameters: EmbedParameters{
			InputType: "query",
			Truncate:  "END",
		},
	})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), embeddings, "Expected embedding to be non-nil")
	require.Equal(ts.T(), embeddingModel, embeddings.Model, "Expected model to be '%s', but got '%s'", embeddingModel, embeddings.Model)
	require.Equal(ts.T(), 2, len(embeddings.Data), "Expected 2 embeddings")
	require.Equal(ts.T(), 1024, len(*embeddings.Data[0].Values), "Expected embeddings to have length 1024")
}

func (ts *IntegrationTests) TestGenerateEmbeddingsInvalidInputs() {
	ctx := context.Background()
	embeddingModel := "multilingual-e5-large"
	_, err := ts.client.Inference.Embed(ctx, &EmbedRequest{
		Model: embeddingModel,
		Parameters: EmbedParameters{
			InputType: "query",
			Truncate:  "END",
		},
	})

	require.Error(ts.T(), err)
	require.Contains(ts.T(), err.Error(), "TextInputs must contain at least one value")
}

func (ts *IntegrationTests) TestRerankDocumentDefaultField() {
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

func (ts *IntegrationTests) TestRerankDocumentCustomField() {
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

func (ts *IntegrationTests) TestRerankDocumentAllDefaults() {
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

func (ts *IntegrationTests) TestRerankDocumentsMultipleRankFields() {
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

func (ts *IntegrationTests) TestRerankDocumentFieldError() {
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

func (ts *IntegrationTests) TestIndexTags() {
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
		t.Errorf(fmt.Sprintf("Expected error to contain 'no API key provided, please pass an API key for authorization', but got '%s'", err.Error()))
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
	require.ErrorContainsf(t, err, "fields Name, Dimension, Metric, Environment, and Podtype must be included in CreatePodIndexRequest", err.Error()) //_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{})
}

func TestCreateServerlessIndexMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{})
	require.Error(t, err)
	require.ErrorContainsf(t, err, "fields Name, Dimension, Metric, Cloud, and Region must be included in CreateServerlessIndexRequest", err.Error())
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
	deletionProtectionEnabled := db_control.Enabled
	deletionProtectionDisabled := db_control.Disabled
	pods := 1
	replicas := int32(1)
	shards := int32(1)

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
				Dimension:          128,
				Host:               "test-host",
				Metric:             "cosine",
				DeletionProtection: &deletionProtectionDisabled,
				Spec: struct {
					Pod        *db_control.PodSpec        `json:"pod,omitempty"`
					Serverless *db_control.ServerlessSpec `json:"serverless,omitempty"`
				}(struct {
					Pod        *db_control.PodSpec
					Serverless *db_control.ServerlessSpec
				}{Pod: &db_control.PodSpec{
					Environment:      "test-environ",
					PodType:          "p1.x2",
					Pods:             &pods,
					Replicas:         &replicas,
					Shards:           &shards,
					SourceCollection: nil,
					MetadataConfig:   nil,
				}}),
				Status: struct {
					Ready bool                             `json:"ready"`
					State db_control.IndexModelStatusState `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:               "testIndex",
				Dimension:          128,
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
				Dimension:          128,
				Host:               "test-host",
				Metric:             "cosine",
				DeletionProtection: &deletionProtectionEnabled,
				Spec: struct {
					Pod        *db_control.PodSpec        `json:"pod,omitempty"`
					Serverless *db_control.ServerlessSpec `json:"serverless,omitempty"`
				}(struct {
					Pod        *db_control.PodSpec
					Serverless *db_control.ServerlessSpec
				}{Serverless: &db_control.ServerlessSpec{
					Cloud:  "test-environ",
					Region: "test-region",
				}}),
				Status: struct {
					Ready bool                             `json:"ready"`
					State db_control.IndexModelStatusState `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:               "testIndex",
				Dimension:          128,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := toIndex(tt.originalInput)
			if diff := cmp.Diff(tt.expectedOutput, input); diff != "" {
				t.Errorf("toIndex() mismatch (-expectedOutput +input):\n%s", diff)
			}
			assert.EqualValues(t, tt.expectedOutput, input)
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

// Helper functions:
func (ts *IntegrationTests) deleteIndex(name string) error {
	_, err := WaitUntilIndexReady(ts, context.Background())
	require.NoError(ts.T(), err)

	return ts.client.DeleteIndex(context.Background(), name)
}

func mockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
