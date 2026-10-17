package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pinecone-io/go-pinecone/v7/internal/gen"
	"github.com/pinecone-io/go-pinecone/v7/internal/gen/db_control"
	"github.com/pinecone-io/go-pinecone/v7/internal/gen/inference"
	"github.com/pinecone-io/go-pinecone/v7/internal/provider"
	"github.com/pinecone-io/go-pinecone/v7/internal/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestCreateServerlessIndexMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{})
	require.Error(t, err)
	require.ErrorContainsf(t, err, "fields Name, Cloud, and Region must be included in CreateServerlessIndexRequest", err.Error())
}

func TestCreateServerlessIndexInvalidSparseDimensionUnit(t *testing.T) {
	vectorType := "sparse"
	dimension := int32(1)
	metric := IndexMetricDotproduct
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
	metric := IndexMetricCosine
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
	metric := IndexMetricCosine
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

func TestCreateCollectionMissingReqdFieldsUnit(t *testing.T) {
	client := &Client{}
	_, err := client.CreateCollection(context.Background(), &CreateCollectionRequest{})
	require.Error(t, err)
	require.ErrorContains(t, err, "fields Name and Source must be included in CreateCollectionRequest")
}

func TestCreateIndexSchemaValidationUnit(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	_, err := client.CreateIndex(ctx, &CreateIndexRequest{Name: "x"})
	require.ErrorContains(t, err, "Schema must contain at least one field")

	_, err = client.CreateIndex(ctx, &CreateIndexRequest{Name: "x", Schema: IndexSchema{
		Fields: map[string]IndexSchemaField{"year": {Integer: &IntegerField{}}},
	}})
	require.ErrorContains(t, err, "metadata fields don't need to be declared")

	_, err = client.CreateIndex(ctx, &CreateIndexRequest{Name: "x", Schema: IndexSchema{
		Fields: map[string]IndexSchemaField{"title": {String: &StringField{}}},
	}})
	require.ErrorContains(t, err, "can only be declared with at least one of FullTextSearch, Embed, or SparseEmbed set")

	_, err = client.CreateIndex(ctx, &CreateIndexRequest{Name: "x", Schema: IndexSchema{
		Fields: map[string]IndexSchemaField{"text": {SemanticText: &SemanticTextField{Model: "multilingual-e5-large"}}},
	}})
	require.ErrorContains(t, err, "declare a StringField with Embed or SparseEmbed")

	_, err = client.CreateIndex(ctx, &CreateIndexRequest{
		Name: "x",
		Schema: IndexSchema{
			Fields: map[string]IndexSchemaField{"_values": {DenseVector: &DenseVectorField{Dimension: 3, Metric: IndexMetricCosine}}},
		},
		Deployment: &IndexDeployment{Pod: &PodDeployment{Environment: "us-east1-gcp", PodType: "p1.x1"}},
	})
	require.ErrorContains(t, err, "creating pod-based indexes is not supported by Pinecone API version 2026-07")
}

func TestLegacyCreateGuidedErrorsUnit(t *testing.T) {
	client := &Client{}
	ctx := context.Background()
	dimension := int32(3)

	_, err := client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name: "x", Cloud: CloudAWS, Region: "us-east-1", Dimension: &dimension, SourceCollection: ptr("my-collection"),
	})
	require.ErrorContains(t, err, "SourceCollection is not supported by Pinecone API version 2026-07")

	_, err = client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name: "x", Cloud: CloudAWS, Region: "us-east-1", Dimension: &dimension, Schema: &MetadataSchema{},
	})
	require.ErrorContains(t, err, "Schema is not supported by Pinecone API version 2026-07")

	_, err = client.CreateBYOCIndex(ctx, &CreateBYOCIndexRequest{
		Name: "x", Environment: "e", Dimension: &dimension, Schema: &MetadataSchema{},
	})
	require.ErrorContains(t, err, "Schema is not supported by Pinecone API version 2026-07")
}

func TestConfigureIndexSchemaValidationUnit(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	_, err := client.ConfigureIndex(ctx, "x", ConfigureIndexParams{Schema: &ConfigureIndexSchema{}})
	require.ErrorContains(t, err, "Schema must contain exactly one field to update")

	_, err = client.ConfigureIndex(ctx, "x", ConfigureIndexParams{Schema: &ConfigureIndexSchema{
		Fields: map[string]ConfigureSemanticTextField{
			"title": {ReadParameters: &map[string]interface{}{"input_type": "query"}},
			"body":  {ReadParameters: &map[string]interface{}{"input_type": "query"}},
		},
	}})
	require.ErrorContains(t, err, "Schema must contain exactly one field to update")
}

func TestListBackupsIncludeDeletedRequiresIndexNameUnit(t *testing.T) {
	client := &Client{}
	_, err := client.ListBackups(context.Background(), &ListBackupsParams{IncludeDeleted: ptr(true)})
	require.ErrorContains(t, err, "IncludeDeleted requires IndexName")
}

func TestDescribeModelRejectsEmptyNameUnit(t *testing.T) {
	inference := &InferenceService{}
	model, err := inference.DescribeModel(context.Background(), "")
	require.ErrorContains(t, err, "modelName must not be empty")
	assert.Nil(t, model)
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
	dimension := int32(128)
	replicas := int32(1)
	shards := int32(1)

	// classicSchema is the wire schema of a classic dense vectors-API index and the public
	// IndexSchema it converts to.
	classicSchemaJSON := `{"fields":{"_values":{"type":"dense_vector","dimension":128,"metric":"cosine"},"_sparse_values":{"type":"sparse_vector"}}}`
	classicSchema := &IndexSchema{
		Fields: map[string]IndexSchemaField{
			"_values":        {DenseVector: &DenseVectorField{Dimension: 128, Metric: IndexMetricCosine}},
			"_sparse_values": {SparseVector: &SparseVectorField{}},
		},
	}

	mustIndexModel := func(raw string) *db_control.IndexModel {
		var idx db_control.IndexModel
		require.NoError(t, json.Unmarshal([]byte(raw), &idx))
		return &idx
	}

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
			originalInput: mustIndexModel(`{
				"name": "testIndex",
				"host": "test-host",
				"deletion_protection": "disabled",
				"status": {"ready": true, "state": "Ready"},
				"deployment": {"deployment_type": "pod", "environment": "test-environ", "pod_type": "p1.x2", "replicas": 1, "shards": 1},
				"schema": ` + classicSchemaJSON + `
			}`),
			expectedOutput: &Index{
				Name:               "testIndex",
				Host:               "test-host",
				DeletionProtection: "disabled",
				Schema:             classicSchema,
				Deployment: &IndexDeployment{
					Pod: &PodDeployment{
						Environment: "test-environ",
						PodType:     "p1.x2",
						Replicas:    &replicas,
						Shards:      &shards,
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("Ready"),
				},
				// Deprecated compat fields computed from Schema and Deployment.
				Dimension:  &dimension,
				Metric:     IndexMetricCosine,
				VectorType: "dense",
				Spec: &IndexSpec{
					Pod: &PodSpec{
						Environment: "test-environ",
						PodType:     "p1.x2",
						PodCount:    1,
						Replicas:    1,
						ShardCount:  1,
					},
				},
			},
		},
		{
			name: "serverless index input",
			originalInput: mustIndexModel(`{
				"name": "testIndex",
				"host": "test-host",
				"deletion_protection": "enabled",
				"source_collection": "my-collection",
				"status": {"ready": true, "state": "Ready"},
				"deployment": {"deployment_type": "managed", "cloud": "aws", "region": "us-east-1"},
				"read_capacity": {"mode": "OnDemand", "status": {"state": "Ready"}},
				"schema": ` + classicSchemaJSON + `
			}`),
			expectedOutput: &Index{
				Name:               "testIndex",
				Host:               "test-host",
				DeletionProtection: "enabled",
				Schema:             classicSchema,
				SourceCollection:   ptr("my-collection"),
				Deployment: &IndexDeployment{
					Managed: &ManagedDeployment{
						Cloud:  Cloud("aws"),
						Region: "us-east-1",
					},
				},
				ReadCapacity: &ReadCapacity{
					OnDemand: &ReadCapacityOnDemand{
						Status: ReadCapacityStatus{State: "Ready"},
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("Ready"),
				},
				// Deprecated compat fields computed from Schema and Deployment.
				Dimension:  &dimension,
				Metric:     IndexMetricCosine,
				VectorType: "dense",
				Spec: &IndexSpec{
					Serverless: &ServerlessSpec{
						Cloud:            Cloud("aws"),
						Region:           "us-east-1",
						SourceCollection: ptr("my-collection"),
						ReadCapacity: &ReadCapacity{
							OnDemand: &ReadCapacityOnDemand{
								Status: ReadCapacityStatus{State: "Ready"},
							},
						},
					},
				},
			},
		},
		{
			name: "sparse serverless index input",
			originalInput: mustIndexModel(`{
				"name": "testIndex",
				"host": "test-host",
				"deletion_protection": "disabled",
				"status": {"ready": true, "state": "Ready"},
				"deployment": {"deployment_type": "managed", "cloud": "aws", "region": "us-east-1"},
				"schema": {"fields": {"_sparse_values": {"type": "sparse_vector"}}}
			}`),
			expectedOutput: &Index{
				Name:               "testIndex",
				Host:               "test-host",
				DeletionProtection: "disabled",
				Schema: &IndexSchema{
					Fields: map[string]IndexSchemaField{
						"_sparse_values": {SparseVector: &SparseVectorField{}},
					},
				},
				Deployment: &IndexDeployment{
					Managed: &ManagedDeployment{
						Cloud:  Cloud("aws"),
						Region: "us-east-1",
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("Ready"),
				},
				// Deprecated compat fields: sparse-only schema implies dotproduct.
				Metric:     IndexMetricDotproduct,
				VectorType: "sparse",
				Spec: &IndexSpec{
					Serverless: &ServerlessSpec{
						Cloud:  Cloud("aws"),
						Region: "us-east-1",
					},
				},
			},
		},
		{
			name: "byoc index input",
			originalInput: mustIndexModel(`{
				"name": "testIndex",
				"host": "test-host",
				"private_host": "test-private-host",
				"deletion_protection": "enabled",
				"status": {"ready": true, "state": "Ready"},
				"deployment": {"deployment_type": "byoc", "environment": "test-environ"},
				"schema": ` + classicSchemaJSON + `
			}`),
			expectedOutput: &Index{
				Name:               "testIndex",
				Host:               "test-host",
				PrivateHost:        ptr("test-private-host"),
				DeletionProtection: "enabled",
				Schema:             classicSchema,
				Deployment: &IndexDeployment{
					Byoc: &ByocDeployment{Environment: "test-environ"},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("Ready"),
				},
				// Deprecated compat fields computed from Schema and Deployment.
				Dimension:  &dimension,
				Metric:     IndexMetricCosine,
				VectorType: "dense",
				Spec: &IndexSpec{
					BYOC: &BYOCSpec{Environment: "test-environ"},
				},
			},
		},
		{
			name: "integrated index input (semantic_text field)",
			originalInput: mustIndexModel(`{
				"name": "testIndex",
				"host": "test-host",
				"deletion_protection": "disabled",
				"status": {"ready": true, "state": "Ready"},
				"deployment": {"deployment_type": "managed", "cloud": "aws", "region": "us-east-1"},
				"schema": {"fields": {"chunk_text": {"type": "semantic_text", "model": "multilingual-e5-large", "dimension": 1024, "metric": "cosine"}}}
			}`),
			expectedOutput: &Index{
				Name:               "testIndex",
				Host:               "test-host",
				DeletionProtection: "disabled",
				Schema: &IndexSchema{
					Fields: map[string]IndexSchemaField{
						"chunk_text": {SemanticText: &SemanticTextField{
							Model:     "multilingual-e5-large",
							Dimension: ptr(int32(1024)),
							Metric:    ptr(IndexMetricCosine),
						}},
					},
				},
				Deployment: &IndexDeployment{
					Managed: &ManagedDeployment{
						Cloud:  Cloud("aws"),
						Region: "us-east-1",
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: IndexStatusState("Ready"),
				},
				// Deprecated compat fields: Embed is computed from the semantic_text field.
				Embed: &IndexEmbed{
					Model:     "multilingual-e5-large",
					Dimension: ptr(int32(1024)),
					Metric:    ptr(IndexMetricCosine),
					FieldMap:  &map[string]interface{}{"text": "chunk_text"},
				},
				Dimension:  ptr(int32(1024)),
				Metric:     IndexMetricCosine,
				VectorType: "dense",
				Spec: &IndexSpec{
					Serverless: &ServerlessSpec{
						Cloud:  Cloud("aws"),
						Region: "us-east-1",
					},
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

func TestToBackupUnit(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		require.Nil(t, toBackup(nil))
	})

	t.Run("maps all fields", func(t *testing.T) {
		createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		sourceIndexDeletedAt := time.Date(2024, 6, 1, 12, 30, 0, 0, time.UTC)
		description := "test backup"
		name := "backup-name"
		namespaceCount := int64(3)
		recordCount := int64(42)
		sizeBytes := int64(2048)
		tags := db_control.IndexTags{"env": "dev"}

		// A classic dense index schema plus a legacy metadata field.
		var schema db_control.IndexSchema
		require.NoError(t, json.Unmarshal([]byte(`{"fields":{"_values":{"type":"dense_vector","dimension":1536,"metric":"cosine"},"genre":{"filterable":true}}}`), &schema))

		model := &db_control.BackupModel{
			BackupId:             "backup-1",
			Cloud:                "aws",
			CreatedAt:            &createdAt,
			Description:          &description,
			Name:                 &name,
			NamespaceCount:       &namespaceCount,
			RecordCount:          &recordCount,
			Region:               "us-east-1",
			Schema:               &schema,
			SizeBytes:            &sizeBytes,
			SourceIndexDeletedAt: &sourceIndexDeletedAt,
			SourceIndexId:        "idx-id",
			SourceIndexName:      "idx-name",
			Status:               "Ready",
			Tags:                 &tags,
		}

		result := toBackup(model)
		require.NotNil(t, result)

		require.Equal(t, "backup-1", result.BackupId)
		require.Equal(t, "aws", result.Cloud)
		require.NotNil(t, result.CreatedAt)
		require.Equal(t, "2024-01-01T00:00:00Z", *result.CreatedAt)
		require.Equal(t, &description, result.Description)
		require.Equal(t, &name, result.Name)
		require.Equal(t, &namespaceCount, result.NamespaceCount)
		require.Equal(t, &recordCount, result.RecordCount)
		require.Equal(t, "us-east-1", result.Region)
		require.Equal(t, &sizeBytes, result.SizeBytes)
		require.Equal(t, &sourceIndexDeletedAt, result.SourceIndexDeletedAt)
		require.Equal(t, "idx-id", result.SourceIndexId)
		require.Equal(t, "idx-name", result.SourceIndexName)
		require.Equal(t, "Ready", result.Status)
		require.Equal(t, IndexTags(tags), *result.Tags)

		// The typed schema is converted field by field.
		require.NotNil(t, result.Schema)
		dense := result.Schema.Fields["_values"].DenseVector
		require.NotNil(t, dense)
		require.Equal(t, int32(1536), dense.Dimension)
		require.Equal(t, IndexMetricCosine, dense.Metric)
		legacy := result.Schema.Fields["genre"].LegacyMetadata
		require.NotNil(t, legacy)
		require.True(t, legacy.Filterable)

		// Deprecated Dimension/Metric are computed from the schema's dense vector field.
		require.NotNil(t, result.Dimension)
		require.Equal(t, int32(1536), *result.Dimension)
		require.NotNil(t, result.Metric)
		require.Equal(t, IndexMetricCosine, *result.Metric)
	})

	t.Run("preserves fractional seconds in CreatedAt", func(t *testing.T) {
		tests := []struct {
			name      string
			createdAt time.Time
			expected  string
		}{
			{"milliseconds", time.Date(2024, 1, 1, 12, 30, 45, 123000000, time.UTC), "2024-01-01T12:30:45.123Z"},
			{"nanoseconds", time.Date(2024, 1, 1, 12, 30, 45, 123456789, time.UTC), "2024-01-01T12:30:45.123456789Z"},
			{"whole seconds", time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC), "2024-01-01T12:30:45Z"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := toBackup(&db_control.BackupModel{CreatedAt: &tt.createdAt})
				require.NotNil(t, result.CreatedAt)
				require.Equal(t, tt.expected, *result.CreatedAt)

				parsed, err := time.Parse(time.RFC3339Nano, *result.CreatedAt)
				require.NoError(t, err)
				require.True(t, tt.createdAt.Equal(parsed))
			})
		}
	})
}

func TestDecodeEmbedResponseUnit(t *testing.T) {
	dense := inference.DenseEmbedding{
		Values: []float32{0.1, 0.2},
	}
	var embedding inference.Embedding
	require.NoError(t, embedding.FromDenseEmbedding(dense))

	totalTokens := int32(10)
	raw := inference.EmbeddingsList{
		Data:       []inference.Embedding{embedding},
		Model:      "test-model",
		VectorType: "dense",
	}
	raw.Usage.TotalTokens = &totalTokens

	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(raw))

	result, err := decodeEmbedResponse(io.NopCloser(bytes.NewReader(buf.Bytes())))
	require.NoError(t, err)
	require.Equal(t, "dense", result.VectorType)
	require.Equal(t, "test-model", result.Model)
	require.Equal(t, totalTokens, derefOrDefault(result.Usage.TotalTokens, int32(0)))
	require.Len(t, result.Data, 1)
	require.NotNil(t, result.Data[0].DenseEmbedding)
	require.Equal(t, []float32{0.1, 0.2}, result.Data[0].DenseEmbedding.Values)
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

func Test_fromMetadataSchemaToRest_Unit(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, fromMetadataSchemaToRest(nil))
	})

	t.Run("empty fields map", func(t *testing.T) {
		result := fromMetadataSchemaToRest(&MetadataSchema{Fields: map[string]MetadataSchemaField{}})
		require.NotNil(t, result)
		require.NotNil(t, result.Fields)
		assert.Empty(t, *result.Fields)
	})

	t.Run("fields keep their filterable flag", func(t *testing.T) {
		result := fromMetadataSchemaToRest(&MetadataSchema{
			Fields: map[string]MetadataSchemaField{
				"genre":  {Filterable: true},
				"year":   {Filterable: true},
				"rating": {Filterable: false},
			},
		})
		require.NotNil(t, result)
		require.NotNil(t, result.Fields)
		fields := *result.Fields
		require.Len(t, fields, 3)
		assert.Equal(t, db_control.MetadataSchemaFieldsFilterable(true), fields["genre"].Filterable)
		assert.Equal(t, db_control.MetadataSchemaFieldsFilterable(true), fields["year"].Filterable)
		assert.Equal(t, db_control.MetadataSchemaFieldsFilterable(false), fields["rating"].Filterable)
	})
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
			name:      "empty ReadCapacityParams should return nil (same as nil)",
			input:     &ReadCapacityParams{},
			wantError: false,
			validate: func(t *testing.T, result *db_control.ReadCapacity) {
				require.Nil(t, result)
			},
		},
		{
			name: "both Dedicated and OnDemand specified should error",
			input: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("t1"),
				},
				OnDemand: &ReadCapacityOnDemandConfig{},
			},
			wantError: true,
			validate: func(t *testing.T, result *db_control.ReadCapacity) {
				require.Nil(t, result)
			},
		},
		{
			name: "OnDemand with empty config should succeed",
			input: &ReadCapacityParams{
				OnDemand: &ReadCapacityOnDemandConfig{},
			},
			wantError: false,
			validate: func(t *testing.T, result *db_control.ReadCapacity) {
				require.NotNil(t, result)
				mode, err := result.Discriminator()

				require.NoError(t, err)
				assert.Equal(t, "OnDemand", mode)

				onDemandSpec, err := result.AsReadCapacityOnDemandSpec()
				require.NoError(t, err)
				assert.Equal(t, "OnDemand", onDemandSpec.Mode)
			},
		},
		{
			name: "Dedicated with NodeType only should error (2026-07 create requires full manual scaling config)",
			input: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("t1"),
				},
			},
			wantError: true,
		},
		{
			name: "Dedicated with partial manual scaling should error",
			input: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("b1"),
					Scaling: &ReadCapacityScaling{
						Manual: &ReadCapacityManualScaling{
							Replicas: ptr(int32(2)),
						},
					},
				},
			},
			wantError: true,
		},
		{
			name: "Dedicated with NodeType and full Manual scaling",
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
				assert.Equal(t, "b1", dedicatedSpec.Dedicated.NodeType)
				assert.Equal(t, "Manual", dedicatedSpec.Dedicated.Scaling)
				assert.Equal(t, int32(2), dedicatedSpec.Dedicated.Manual.Replicas)
				assert.Equal(t, int32(3), dedicatedSpec.Dedicated.Manual.Shards)
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
		validate  func(t *testing.T, result *db_control.ReadCapacityPatch, err error)
	}{
		{
			name:      "nil new params should return nil",
			newParams: nil,
			oldConfig: onDemandOld,
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
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
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
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
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				mode, modeErr := result.Discriminator()
				require.NoError(t, modeErr)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, specErr := result.AsReadCapacityDedicatedPatchSpec()
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
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				mode, modeErr := result.Discriminator()
				require.NoError(t, modeErr)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, specErr := result.AsReadCapacityDedicatedPatchSpec()
				require.NoError(t, specErr)
				assert.Nil(t, dedicatedSpec.Dedicated.NodeType)
				require.NotNil(t, dedicatedSpec.Dedicated.Manual)
				assert.Equal(t, int32(4), *dedicatedSpec.Dedicated.Manual.Replicas)
				assert.Nil(t, dedicatedSpec.Dedicated.Manual.Shards)
			},
		},
		{
			name: "nil old to on-demand should succeed",
			newParams: &ReadCapacityParams{
				OnDemand: &ReadCapacityOnDemandConfig{},
			},
			oldConfig: nil,
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				mode, modeErr := result.Discriminator()
				require.NoError(t, modeErr)
				assert.Equal(t, "OnDemand", mode)

				onDemandSpec, specErr := result.AsReadCapacityOnDemandSpec()
				require.NoError(t, specErr)
				assert.Equal(t, "OnDemand", onDemandSpec.Mode)
			},
		},
		{
			name: "nil old to dedicated without manual scaling should error",
			newParams: &ReadCapacityParams{
				Dedicated: &ReadCapacityDedicatedConfig{
					NodeType: ptr("b1"),
				},
			},
			oldConfig: nil,
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		},
		{
			name: "nil old to dedicated with full manual scaling succeeds",
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
			oldConfig: nil,
			validate: func(t *testing.T, result *db_control.ReadCapacityPatch, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				mode, modeErr := result.Discriminator()
				require.NoError(t, modeErr)
				assert.Equal(t, "Dedicated", mode)

				dedicatedSpec, specErr := result.AsReadCapacityDedicatedPatchSpec()
				require.NoError(t, specErr)
				require.NotNil(t, dedicatedSpec.Dedicated.NodeType)
				assert.Equal(t, "t1", *dedicatedSpec.Dedicated.NodeType)
				require.NotNil(t, dedicatedSpec.Dedicated.Manual)
				assert.Equal(t, int32(2), *dedicatedSpec.Dedicated.Manual.Replicas)
				assert.Equal(t, int32(3), *dedicatedSpec.Dedicated.Manual.Shards)
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
				NodeType: "t1",
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
				NodeType: "b1",
				Scaling:  "Manual",
				Manual: db_control.ScalingConfigManual{
					Replicas: 2,
					Shards:   3,
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

	emptyInput := func() *db_control.ReadCapacityResponse {
		// Create an empty ReadCapacityResponse (simulates missing read_capacity in API response)
		var result db_control.ReadCapacityResponse
		return &result
	}()

	malformedInput := func() *db_control.ReadCapacityResponse {
		// Create a malformed ReadCapacityResponse with invalid JSON
		var result db_control.ReadCapacityResponse
		// Inject malformed JSON that isn't "unexpected end of JSON input"
		_ = result.UnmarshalJSON([]byte(`{"mode": "OnDemand", "status": {"state": "Ready"`)) // missing closing braces
		return &result
	}()

	unknownModeInput := func() *db_control.ReadCapacityResponse {
		// Create a ReadCapacityResponse with an unknown mode
		var result db_control.ReadCapacityResponse
		_ = result.UnmarshalJSON([]byte(`{"mode": "FutureMode", "status": {"state": "Ready"}}`))
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
			name:      "empty input (no discriminator)",
			input:     emptyInput,
			wantError: false,
			expected:  nil,
		},
		{
			name:      "malformed JSON returns nil (permissive)",
			input:     malformedInput,
			wantError: false,
			expected:  nil,
		},
		{
			name:      "unknown mode returns nil (permissive for future compatibility)",
			input:     unknownModeInput,
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

func Test_ConfigureIndex_ValidationErrors_Unit(t *testing.T) {
	tests := []struct {
		name           string
		deploymentJSON string
		configParams   ConfigureIndexParams
		expectedError  string
	}{
		{
			name:           "Pod index with ReadCapacity should error",
			deploymentJSON: podDeploymentJSON,
			configParams: ConfigureIndexParams{
				ReadCapacity: &ReadCapacityParams{
					Dedicated: &ReadCapacityDedicatedConfig{
						NodeType: ptr("t1"),
					},
				},
			},
			expectedError: "cannot configure ReadCapacity on a pod index; ReadCapacity is only supported for serverless and BYOC indexes",
		},
		{
			name:           "Serverless index with PodType should error",
			deploymentJSON: managedDeploymentJSON,
			configParams: ConfigureIndexParams{
				PodType: "p1.x1",
			},
			expectedError: "cannot configure PodType or Replicas on a non-pod index; these parameters are only supported for pod indexes",
		},
		{
			name:           "Serverless index with Replicas should error",
			deploymentJSON: managedDeploymentJSON,
			configParams: ConfigureIndexParams{
				Replicas: 4,
			},
			expectedError: "cannot configure PodType or Replicas on a non-pod index; these parameters are only supported for pod indexes",
		},
		{
			name:           "Serverless index with both PodType and Replicas should error",
			deploymentJSON: managedDeploymentJSON,
			configParams: ConfigureIndexParams{
				PodType:  "p1.x1",
				Replicas: 4,
			},
			expectedError: "cannot configure PodType or Replicas on a non-pod index; these parameters are only supported for pod indexes",
		},
		{
			name:           "BYOC index with PodType should error",
			deploymentJSON: byocDeploymentJSON,
			configParams: ConfigureIndexParams{
				PodType: "p1.x1",
			},
			expectedError: "cannot configure PodType or Replicas on a non-pod index; these parameters are only supported for pod indexes",
		},
		{
			name:           "BYOC index with Replicas should error",
			deploymentJSON: byocDeploymentJSON,
			configParams: ConfigureIndexParams{
				Replicas: 2,
			},
			expectedError: "cannot configure PodType or Replicas on a non-pod index; these parameters are only supported for pod indexes",
		},
		{
			name:           "Embed always errors under the 2026-07 API",
			deploymentJSON: managedDeploymentJSON,
			configParams: ConfigureIndexParams{
				Embed: &ConfigureIndexEmbed{
					Model: ptr("multilingual-e5-large"),
				},
			},
			expectedError: "Embed is not supported by Pinecone API version 2026-07: to update the read or write parameters of an index's embedding model, use Schema",
		},
		{
			name:           "BYOC index switching ReadCapacity to OnDemand should succeed",
			deploymentJSON: byocDeploymentJSON,
			configParams: ConfigureIndexParams{
				ReadCapacity: &ReadCapacityParams{
					OnDemand: &ReadCapacityOnDemandConfig{},
				},
			},
			expectedError: "", // should succeed without panic
		},
		{
			name:           "Pod index scaling PodType and Replicas should succeed",
			deploymentJSON: podDeploymentJSON,
			configParams: ConfigureIndexParams{
				PodType:  "p1.x2",
				Replicas: 2,
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWireServer(t, http.StatusOK, indexModelFixture{deployment: tt.deploymentJSON}.JSON())
			client := newWireClient(t, server.URL)

			_, err := client.ConfigureIndex(context.Background(), "test-index", tt.configParams)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
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
