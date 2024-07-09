package pinecone

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClientTests struct {
	suite.Suite
	client          Client
	clientSourceTag Client
	sourceTag       string
	podIndex        string
	serverlessIndex string
}

func TestClient(t *testing.T) {
	suite.Run(t, new(ClientTests))
}

func TestHandleErrorResponseBody(t *testing.T) {
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

func (ts *ClientTests) SetupSuite() {
	apiKey := os.Getenv("PINECONE_API_KEY")
	require.NotEmpty(ts.T(), apiKey, "PINECONE_API_KEY env variable not set")

	ts.podIndex = os.Getenv("TEST_POD_INDEX_NAME")
	require.NotEmpty(ts.T(), ts.podIndex, "TEST_POD_INDEX_NAME env variable not set")

	ts.serverlessIndex = os.Getenv("TEST_SERVERLESS_INDEX_NAME")
	require.NotEmpty(ts.T(), ts.serverlessIndex, "TEST_SERVERLESS_INDEX_NAME env variable not set")

	client, err := NewClient(NewClientParams{})
	require.NoError(ts.T(), err)

	ts.client = *client

	ts.sourceTag = "test_source_tag"
	clientSourceTag, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: ts.sourceTag})
	require.NoError(ts.T(), err)
	ts.clientSourceTag = *clientSourceTag

	// this will clean up the project deleting all indexes and collections that are
	// named a UUID. Generally not needed as all tests are cleaning up after themselves
	// Left here as a convenience during active development.
	//deleteUUIDNamedResources(context.Background(), &ts.client)
}

func (ts *ClientTests) TestNewClientParamsSet() {
	apiKey := "test-api-key"
	client, err := NewClient(NewClientParams{ApiKey: apiKey})

	require.NoError(ts.T(), err)
	require.Empty(ts.T(), client.sourceTag, "Expected client to have empty sourceTag")
	require.NotNil(ts.T(), client.headers, "Expected client headers to not be nil")
	apiKeyHeader, ok := client.headers["Api-Key"]
	require.True(ts.T(), ok, "Expected client to have an 'Api-Key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(ts.T(), 2, len(client.restClient.RequestEditors), "Expected client to have correct number of require editors")
}

func (ts *ClientTests) TestNewClientParamsSetSourceTag() {
	apiKey := "test-api-key"
	sourceTag := "test-source-tag"
	client, err := NewClient(NewClientParams{
		ApiKey:    apiKey,
		SourceTag: sourceTag,
	})

	require.NoError(ts.T(), err)
	apiKeyHeader, ok := client.headers["Api-Key"]
	require.True(ts.T(), ok, "Expected client to have an 'Api-Key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(ts.T(), sourceTag, client.sourceTag, "Expected client to have sourceTag '%s', but got '%s'", sourceTag, client.sourceTag)
	require.Equal(ts.T(), 2, len(client.restClient.RequestEditors), "Expected client to have %s request editors, but got %s", 2, len(client.restClient.RequestEditors))
}

func (ts *ClientTests) TestNewClientParamsSetHeaders() {
	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "test-value"}
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers})

	require.NoError(ts.T(), err)
	apiKeyHeader, ok := client.headers["Api-Key"]
	require.True(ts.T(), ok, "Expected client to have an 'Api-Key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(ts.T(), client.headers, headers, "Expected client to have headers '%+v', but got '%+v'", headers, client.headers)
	require.Equal(ts.T(), 3, len(client.restClient.RequestEditors), "Expected client to have %s request editors, but got %s", 3, len(client.restClient.RequestEditors))
}

func (ts *ClientTests) TestNewClientParamsNoApiKeyNoAuthorizationHeader() {
	apiKey := os.Getenv("PINECONE_API_KEY")
	os.Unsetenv("PINECONE_API_KEY")

	client, err := NewClient(NewClientParams{})
	require.NotNil(ts.T(), err, "Expected error when creating client without an API key or Authorization header")
	if !strings.Contains(err.Error(), "no API key provided, please pass an API key for authorization") {
		ts.FailNow(fmt.Sprintf("Expected error to contain 'no API key provided, please pass an API key for authorization', but got '%s'", err.Error()))
	}

	require.Nil(ts.T(), client, "Expected client to be nil when creating client without an API key or Authorization header")

	os.Setenv("PINECONE_API_KEY", apiKey)
}

func (ts *ClientTests) TestHeadersAppliedToRequests() {
	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "123456"}

	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers, RestClient: httpClient})
	if err != nil {
		ts.FailNow(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), mockTransport.Req, "Expected request to be made")

	testHeaderValue := mockTransport.Req.Header.Get("test-header")
	assert.Equal(ts.T(), "123456", testHeaderValue, "Expected request to have header value '123456', but got '%s'", testHeaderValue)
}

func (ts *ClientTests) TestAdditionalHeadersAppliedToRequest() {
	os.Setenv("PINECONE_ADDITIONAL_HEADERS", `{"test-header": "environment-header"}`)

	apiKey := "test-api-key"

	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, RestClient: httpClient})
	if err != nil {
		ts.FailNow(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), mockTransport.Req, "Expected request to be made")

	testHeaderValue := mockTransport.Req.Header.Get("test-header")
	assert.Equal(ts.T(), "environment-header", testHeaderValue, "Expected request to have header value 'environment-header', but got '%s'", testHeaderValue)

	os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
}

func (ts *ClientTests) TestHeadersOverrideAdditionalHeaders() {
	os.Setenv("PINECONE_ADDITIONAL_HEADERS", `{"test-header": "environment-header"}`)

	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "param-header"}

	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers, RestClient: httpClient})
	if err != nil {
		ts.FailNow(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), mockTransport.Req, "Expected request to be made")

	testHeaderValue := mockTransport.Req.Header.Get("test-header")
	assert.Equal(ts.T(), "param-header", testHeaderValue, "Expected request to have header value 'param-header', but got '%s'", testHeaderValue)

	os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
}

func (ts *ClientTests) TestControllerHostOverride() {
	apiKey := "test-api-key"
	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Host: "https://test-controller-host.io", RestClient: httpClient})
	if err != nil {
		ts.FailNow(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), mockTransport.Req, "Expected request to be made")
	assert.Equal(ts.T(), "test-controller-host.io", mockTransport.Req.Host, "Expected request to be made to 'test-controller-host.io', but got '%s'", mockTransport.Req.URL.Host)
}

func (ts *ClientTests) TestControllerHostOverrideFromEnv() {
	os.Setenv("PINECONE_CONTROLLER_HOST", "https://env-controller-host.io")

	apiKey := "test-api-key"
	httpClient := utils.CreateMockClient(`{"indexes": []}`)
	client, err := NewClient(NewClientParams{ApiKey: apiKey, RestClient: httpClient})
	if err != nil {
		ts.FailNow(err.Error())
	}
	mockTransport := httpClient.Transport.(*utils.MockTransport)

	_, err = client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), mockTransport.Req, "Expected request to be made")
	assert.Equal(ts.T(), "env-controller-host.io", mockTransport.Req.Host, "Expected request to be made to 'env-controller-host.io', but got '%s'", mockTransport.Req.URL.Host)

	os.Unsetenv("PINECONE_CONTROLLER_HOST")
}

func (ts *ClientTests) TestExtractAuthHeader() {
	globalApiKey := os.Getenv("PINECONE_API_KEY")
	os.Unsetenv("PINECONE_API_KEY")

	// Passing an API key should result in an 'Api-Key' header
	apiKey := "test-api-key"
	expectedHeader := map[string]string{"Api-Key": apiKey}
	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	if err != nil {
		ts.FailNow(err.Error())
	}
	assert.Equal(ts.T(),
		expectedHeader,
		client.extractAuthHeader(),
		"Expected client.extractAuthHeader to return %v but got '%s'", expectedHeader, client.extractAuthHeader(),
	)

	// Passing a custom auth header with "authorization" should be returned as is
	expectedHeader = map[string]string{"Authorization": "Bearer test-token-123456"}
	client, err = NewClientBase(NewClientBaseParams{Headers: expectedHeader})
	if err != nil {
		ts.FailNow(err.Error())
	}
	assert.Equal(ts.T(),
		expectedHeader,
		client.extractAuthHeader(),
		"Expected client.extractAuthHeader to return %v but got '%s'", expectedHeader, client.extractAuthHeader(),
	)

	// Passing a custom auth header with "access_token" should be returned as is
	expectedHeader = map[string]string{"access_token": "test-token-123456"}
	client, err = NewClientBase(NewClientBaseParams{Headers: expectedHeader})
	if err != nil {
		ts.FailNow(err.Error())
	}
	assert.Equal(ts.T(),
		expectedHeader,
		client.extractAuthHeader(),
		"Expected client.extractAuthHeader to return %v but got '%s'", expectedHeader, client.extractAuthHeader(),
	)

	os.Setenv("PINECONE_API_KEY", globalApiKey)
}

func (ts *ClientTests) TestApiKeyPassedToIndexConnection() {
	apiKey := "test-api-key"

	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	if err != nil {
		ts.FailNow(err.Error())
	}

	indexConn, err := client.Index(NewIndexConnParams{Host: "my-index-host.io"})
	if err != nil {
		ts.FailNow(err.Error())
	}

	indexMetadata := indexConn.additionalMetadata
	metadataHasApiKey := false
	for key, value := range indexMetadata {
		if key == "Api-Key" && value == apiKey {
			metadataHasApiKey = true
			break
		}
	}

	assert.True(ts.T(), metadataHasApiKey, "Expected IndexConnection metadata to contain 'Api-Key' with value '%s'", apiKey)
}

func (ts *ClientTests) TestControllerHostNormalization() {
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
		ts.Run(tt.name, func() {
			apiKey := "test-api-key"
			httpClient := utils.CreateMockClient(`{"indexes": []}`)
			client, err := NewClient(NewClientParams{ApiKey: apiKey, Host: tt.host, RestClient: httpClient})
			if err != nil {
				ts.FailNow(err.Error())
			}
			mockTransport := httpClient.Transport.(*utils.MockTransport)

			_, err = client.ListIndexes(context.Background())
			require.NoError(ts.T(), err)
			require.NotNil(ts.T(), mockTransport.Req, "Expected request to be made")

			assert.Equal(ts.T(), tt.wantHost, mockTransport.Req.URL.Host, "Expected request to be made to host '%s', but got '%s'", tt.wantHost, mockTransport.Req.URL.Host)
			assert.Equal(ts.T(), tt.wantScheme, mockTransport.Req.URL.Scheme, "Expected request to be made to host '%s, but got '%s'", tt.wantScheme, mockTransport.Req.URL.Host)
		})
	}
}

func (ts *ClientTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *ClientTests) TestListIndexesSourceTag() {
	indexes, err := ts.clientSourceTag.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *ClientTests) TestCreatePodIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
}

func (ts *ClientTests) TestCreatePodIndexInvalidDimension() {
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

func (ts *ClientTests) TestCreateServerlessIndexInvalidDimension() {
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

func (ts *ClientTests) TestCreateServerlessIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTests, name string) {
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

func (ts *ClientTests) TestDescribeServerlessIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.serverlessIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.serverlessIndex, index.Name, "Index name does not match")
}

func (ts *ClientTests) TestDescribeNonExistentIndex() {
	_, err := ts.client.DescribeIndex(context.Background(), "non-existent-index")
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *ClientTests) TestDescribeServerlessIndexSourceTag() {
	index, err := ts.clientSourceTag.DescribeIndex(context.Background(), ts.serverlessIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.serverlessIndex, index.Name, "Index name does not match")
}

func (ts *ClientTests) TestDescribePodIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.podIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.podIndex, index.Name, "Index name does not match")
}

func (ts *ClientTests) TestDescribePodIndexSourceTag() {
	index, err := ts.clientSourceTag.DescribeIndex(context.Background(), ts.podIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.podIndex, index.Name, "Index name does not match")
}

func (ts *ClientTests) TestListCollections() {
	ctx := context.Background()

	var collectionNames []string
	for i := 0; i < 3; i++ {
		collectionName := uuid.New().String()
		collectionNames = append(collectionNames, collectionName)
	}

	defer func(ts *ClientTests, collectionNames []string) {
		for _, name := range collectionNames {
			err := ts.client.DeleteCollection(ctx, name)
			require.NoError(ts.T(), err, "Error deleting collection")
		}
	}(ts, collectionNames)

	for _, name := range collectionNames {
		_, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
			Name:   name,
			Source: ts.podIndex,
		})
		require.NoError(ts.T(), err, "Error creating collection")
	}

	// Call the method under test to list all collections
	collections, err := ts.client.ListCollections(ctx)
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(collections), 2, "Expected at least three collections to exist")

	// Check that the created collections are in the returned list
	found := 0
	for _, collection := range collections {
		for _, name := range collectionNames {
			if collection.Name == name {
				found++
				break
			}
		}
	}
	require.Equal(ts.T(), len(collectionNames), found, "Not all created collections were listed")
}

func (ts *ClientTests) TestDescribeCollection() {
	ctx := context.Background()
	collectionName := uuid.New().String()

	defer func(client *Client, ctx context.Context, collectionName string) {
		err := client.DeleteCollection(ctx, collectionName)
		require.NoError(ts.T(), err)
	}(&ts.client, ctx, collectionName)

	_, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	require.NoError(ts.T(), err)

	collection, err := ts.client.DescribeCollection(ctx, collectionName)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), collectionName, collection.Name, "Collection name does not match")
}

func (ts *ClientTests) TestCreateCollection() {
	name := uuid.New().String()
	sourceIndex := ts.podIndex

	defer func() {
		err := ts.client.DeleteCollection(context.Background(), name)
		require.NoError(ts.T(), err)
	}()

	collection, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   name,
		Source: sourceIndex,
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, collection.Name, "Collection name does not match")
}

func (ts *ClientTests) TestDeleteCollection() {
	collectionName := uuid.New().String()
	_, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	require.NoError(ts.T(), err)

	err = ts.client.DeleteCollection(context.Background(), collectionName)
	require.NoError(ts.T(), err)
}

func (ts *ClientTests) deleteIndex(name string) error {
	return ts.client.DeleteIndex(context.Background(), name)
}

// Helper function to check if a name is a valid UUID
func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func deleteUUIDNamedResources(ctx context.Context, c *Client) error {
	// Delete UUID-named indexes
	indexes, err := c.ListIndexes(ctx)
	if err != nil {
		return err
	}

	for _, index := range indexes {
		if isValidUUID(index.Name) {
			err := c.DeleteIndex(ctx, index.Name)
			if err != nil {
				return err
			}
		}
	}

	// Delete UUID-named collections
	collections, err := c.ListCollections(ctx)
	if err != nil {
		return err
	}

	for _, collection := range collections {
		if isValidUUID(collection.Name) {
			err := c.DeleteCollection(ctx, collection.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func mockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
