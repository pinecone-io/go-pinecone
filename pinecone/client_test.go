package pinecone

import (
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/internal/provider"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pinecone-io/go-pinecone/internal/gen/control"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Integration tests:
type ClientTestsIntegration struct {
	suite.Suite
	client          Client
	clientSourceTag Client
	sourceTag       string
	podIndex        string
	serverlessIndex string
}

func TestIntegrationClient(t *testing.T) {
	suite.Run(t, new(ClientTestsIntegration))
}

func (ts *ClientTestsIntegration) SetupSuite() {
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

func (ts *ClientTestsIntegration) TestNewClientParamsSetIntegration() {
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

func (ts *ClientTestsIntegration) TestNewClientParamsSetSourceTag() {
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

func (ts *ClientTestsIntegration) TestNewClientParamsSetHeaders() {
	apiKey := "test-api-key"
	headers := map[string]string{"test-header": "test-ptr"}
	client, err := NewClient(NewClientParams{ApiKey: apiKey, Headers: headers})

	require.NoError(ts.T(), err)
	apiKeyHeader, ok := client.headers["Api-Key"]
	require.True(ts.T(), ok, "Expected client to have an 'Api-Key' header")
	require.Equal(ts.T(), apiKey, apiKeyHeader, "Expected 'Api-Key' header to match provided ApiKey")
	require.Equal(ts.T(), client.headers, headers, "Expected client to have headers '%+v', but got '%+v'", headers, client.headers)
	require.Equal(ts.T(), 3, len(client.restClient.RequestEditors), "Expected client to have %s request editors, but got %s", 3, len(client.restClient.RequestEditors))
}

func (ts *ClientTestsIntegration) TestNewClientParamsNoApiKeyNoAuthorizationHeader() {
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

func (ts *ClientTestsIntegration) TestHeadersAppliedToRequests() {
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
	assert.Equal(ts.T(), "123456", testHeaderValue, "Expected request to have header ptr '123456', but got '%s'", testHeaderValue)
}

func (ts *ClientTestsIntegration) TestAdditionalHeadersAppliedToRequest() {
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
	assert.Equal(ts.T(), "environment-header", testHeaderValue, "Expected request to have header ptr 'environment-header', but got '%s'", testHeaderValue)

	os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
}

func (ts *ClientTestsIntegration) TestHeadersOverrideAdditionalHeaders() {
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
	assert.Equal(ts.T(), "param-header", testHeaderValue, "Expected request to have header ptr 'param-header', but got '%s'", testHeaderValue)

	os.Unsetenv("PINECONE_ADDITIONAL_HEADERS")
}

func (ts *ClientTestsIntegration) TestControllerHostOverride() {
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

func (ts *ClientTestsIntegration) TestControllerHostOverrideFromEnv() {
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

func (ts *ClientTestsIntegration) TestControllerHostNormalization() {
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

func (ts *ClientTestsIntegration) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *ClientTestsIntegration) TestListIndexesSourceTag() {
	indexes, err := ts.clientSourceTag.ListIndexes(context.Background())
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *ClientTestsIntegration) TestCreatePodIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
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

func (ts *ClientTestsIntegration) TestCreatePodIndexInvalidDimension() {
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

func (ts *ClientTestsIntegration) TestCreateServerlessIndexInvalidDimension() {
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

func (ts *ClientTestsIntegration) TestCreateServerlessIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
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

func (ts *ClientTestsIntegration) TestDescribeServerlessIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.serverlessIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.serverlessIndex, index.Name, "Index name does not match")
}

func (ts *ClientTestsIntegration) TestDescribeNonExistentIndex() {
	_, err := ts.client.DescribeIndex(context.Background(), "non-existent-index")
	require.Error(ts.T(), err)
	require.Equal(ts.T(), reflect.TypeOf(err), reflect.TypeOf(&PineconeError{}), "Expected error to be of type PineconeError")
}

func (ts *ClientTestsIntegration) TestDescribeServerlessIndexSourceTag() {
	index, err := ts.clientSourceTag.DescribeIndex(context.Background(), ts.serverlessIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.serverlessIndex, index.Name, "Index name does not match")
}

func (ts *ClientTestsIntegration) TestDescribePodIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.podIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.podIndex, index.Name, "Index name does not match")
}

func (ts *ClientTestsIntegration) TestDescribePodIndexSourceTag() {
	index, err := ts.clientSourceTag.DescribeIndex(context.Background(), ts.podIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.podIndex, index.Name, "Index name does not match")
}

func (ts *ClientTestsIntegration) TestListCollections() {
	ctx := context.Background()

	var collectionNames []string
	for i := 0; i < 3; i++ {
		collectionName := uuid.New().String()
		collectionNames = append(collectionNames, collectionName)
	}

	defer func(ts *ClientTestsIntegration, collectionNames []string) {
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

func (ts *ClientTestsIntegration) TestDescribeCollection() {
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

func (ts *ClientTestsIntegration) TestCreateCollection() {
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

func (ts *ClientTestsIntegration) TestDeleteCollection() {
	collectionName := uuid.New().String()
	_, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	require.NoError(ts.T(), err)

	err = ts.client.DeleteCollection(context.Background(), collectionName)
	require.NoError(ts.T(), err)
}

func (ts *ClientTestsIntegration) TestConfigureIndexIllegalScaleDown() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	pods := "p1.x1"      // test index originally created with "p1.x2" pods
	replicas := int32(1) // could be nil, but do not want to test nil case here
	_, err = ts.client.ConfigureIndex(context.Background(), name, &pods, &replicas)
	require.ErrorContainsf(ts.T(), err, "Cannot scale down", err.Error())
}

func (ts *ClientTestsIntegration) TestConfigureIndexScaleUpNoPods() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	replicas := int32(2)
	_, err = ts.client.ConfigureIndex(context.Background(), name, nil, &replicas)
	require.NoError(ts.T(), err)
}

func (ts *ClientTestsIntegration) TestConfigureIndexScaleUpNoReplicas() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	pods := "p1.x4"
	_, err = ts.client.ConfigureIndex(context.Background(), name, &pods, nil)
	require.NoError(ts.T(), err)
}

func (ts *ClientTestsIntegration) TestConfigureIndexIllegalNoPodsOrReplicas() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	_, err = ts.client.ConfigureIndex(context.Background(), name, nil, nil)
	require.ErrorContainsf(ts.T(), err, "must specify either podType or replicas", err.Error())
}

func (ts *ClientTestsIntegration) TestConfigureIndexHitPodLimit() {
	name := uuid.New().String()

	defer func(ts *ClientTestsIntegration, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	_, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x2",
	})
	if err != nil {
		log.Fatalf("Error creating index %s: %v", name, err)
	}

	replicas := int32(30000)
	_, err = ts.client.ConfigureIndex(context.Background(), name, nil, &replicas)
	require.ErrorContainsf(ts.T(), err, "You've reached the max pods allowed", err.Error())
}

func (ts *ClientTestsIntegration) deleteIndex(name string) error {
	return ts.client.DeleteIndex(context.Background(), name)
}

// Unit tests:
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
		{ // TODO: should really add a negative case here, but triggering a marshalling error is hard
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
			assert.Equal(t, tt.expectedResult, *result, "Expected result to be '%d', but got '%d'", tt.expectedResult, *result)
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
			expected: "http://pinecone-api.io", // TODO: should this be https?
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
	tests := []struct {
		name           string
		originalInput  *control.IndexModel
		expectedOutput *Index
	}{
		{
			name:           "nil input",
			originalInput:  nil,
			expectedOutput: nil,
		},
		{
			name: "pod index input",
			originalInput: &control.IndexModel{
				Name:      "testIndex",
				Dimension: 128,
				Host:      "test-host",
				Metric:    "cosine",
				Spec: struct {
					Pod        *control.PodSpec        `json:"pod,omitempty"`
					Serverless *control.ServerlessSpec `json:"serverless,omitempty"`
				}(struct {
					Pod        *control.PodSpec
					Serverless *control.ServerlessSpec
				}{Pod: &control.PodSpec{
					Environment:      "test-environ",
					PodType:          "p1.x2",
					Pods:             1,
					Replicas:         1,
					Shards:           1,
					SourceCollection: nil,
					MetadataConfig:   nil,
				}}),
				Status: struct {
					Ready bool                          `json:"ready"`
					State control.IndexModelStatusState `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:      "testIndex",
				Dimension: 128,
				Host:      "test-host",
				Metric:    "cosine",
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
			originalInput: &control.IndexModel{
				Name:      "testIndex",
				Dimension: 128,
				Host:      "test-host",
				Metric:    "cosine",
				Spec: struct {
					Pod        *control.PodSpec        `json:"pod,omitempty"`
					Serverless *control.ServerlessSpec `json:"serverless,omitempty"`
				}(struct {
					Pod        *control.PodSpec
					Serverless *control.ServerlessSpec
				}{Serverless: &control.ServerlessSpec{
					Cloud:  "test-environ",
					Region: "test-region",
				}}),
				Status: struct {
					Ready bool                          `json:"ready"`
					State control.IndexModelStatusState `json:"state"`
				}{
					Ready: true,
					State: "active",
				},
			},
			expectedOutput: &Index{
				Name:      "testIndex",
				Dimension: 128,
				Host:      "test-host",
				Metric:    "cosine",
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
		originalInput  *control.CollectionModel
		expectedOutput *Collection
	}{
		{
			name:           "nil input",
			originalInput:  nil,
			expectedOutput: nil,
		},
		{
			name: "collection input",
			originalInput: &control.CollectionModel{
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
			originalInput: &control.CollectionModel{
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
				assert.Equal(t, tc.expectedHeaders, client.headers, "Expected headers to be '%v', but got '%v'", tc.expectedHeaders, client.headers)
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
			name:         "Host provided in parameters",
			host:         "https://custom-host.com/",
			envHost:      "",
			expectedHost: "https://custom-host.com/",
			expectedErr:  false,
		},
		{
			name:         "Host from environment variable",
			host:         "",
			envHost:      "https://env-host.com/",
			expectedHost: "https://env-host.com/",
			expectedErr:  false,
		},
		{
			name:         "Default host",
			host:         "",
			envHost:      "",
			expectedHost: "https://api.pinecone.io/",
			expectedErr:  false,
		},
		{
			name:         "Invalid URL scheme",
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

func TestBuildClientBaseOptions(t *testing.T) {
	tests := []struct {
		name           string
		params         NewClientBaseParams
		envHeaders     string
		expect         []control.ClientOption
		expectEnvUnset bool
	}{
		{
			name: "Construct base params without additional env headers present",
			params: NewClientBaseParams{
				SourceTag: "source-tag",
				Headers:   map[string]string{"Param-Header": "param-value"},
			},
			expect: []control.ClientOption{
				control.WithRequestEditorFn(provider.NewHeaderProvider("User-Agent", "test-user-agent").Intercept),
				control.WithRequestEditorFn(provider.NewHeaderProvider("Param-Header", "param-value").Intercept),
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
			expect: []control.ClientOption{
				control.WithRequestEditorFn(provider.NewHeaderProvider("Env-Header", "env-value").Intercept),
				control.WithRequestEditorFn(provider.NewHeaderProvider("User-Agent", "test-user-agent").Intercept),
				control.WithRequestEditorFn(provider.NewHeaderProvider("Param-Header", "param-value").Intercept),
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
