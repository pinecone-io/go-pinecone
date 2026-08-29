package pinecone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/pinecone-io/go-pinecone/v6/internal/gen"
	db_data_rest "github.com/pinecone-io/go-pinecone/v6/internal/gen/db_data/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the 2026-07 wire conformance of every request the SDK builds: the
// X-Pinecone-Api-Version header value, the schema/deployment translation the legacy create shims
// produce, and the request shapes of the documents API.

const conformanceIndexJSON = `{
	"name": "test-index",
	"host": "https://test-host.pinecone.io",
	"deletion_protection": "disabled",
	"status": {"ready": true, "state": "Ready"},
	"deployment": {"deployment_type": "managed", "cloud": "aws", "region": "us-east-1"},
	"schema": {"fields": {
		"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine", "description": null},
		"_sparse_values": {"type": "sparse_vector", "description": null}
	}}
}`

const conformancePodIndexJSON = `{
	"name": "test-index",
	"host": "https://test-host.pinecone.io",
	"deletion_protection": "disabled",
	"status": {"ready": true, "state": "Ready"},
	"deployment": {"deployment_type": "pod", "environment": "us-west1-gcp", "pod_type": "p1.x2", "replicas": 2, "shards": 1},
	"schema": {"fields": {
		"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine", "description": null}
	}}
}`

type conformanceCapture struct {
	mu       sync.Mutex
	requests []conformanceRequest
}

type conformanceRequest struct {
	method     string
	path       string
	apiVersion string
	body       map[string]interface{}
}

func (c *conformanceCapture) last(t *testing.T) conformanceRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	require.NotEmpty(t, c.requests, "expected at least one captured request")
	return c.requests[len(c.requests)-1]
}

// newConformanceServer records every request and answers with the response handler's choice of
// body and status per method+path.
func newConformanceServer(t *testing.T, respond func(r *http.Request) (int, string)) (*httptest.Server, *conformanceCapture) {
	capture := &conformanceCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		capture.mu.Lock()
		capture.requests = append(capture.requests, conformanceRequest{
			method:     r.Method,
			path:       r.URL.Path,
			apiVersion: r.Header.Get("X-Pinecone-Api-Version"),
			body:       body,
		})
		capture.mu.Unlock()

		status, response := respond(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server, capture
}

func newConformanceClient(t *testing.T, host string) *Client {
	client, err := NewClientBase(NewClientBaseParams{
		Host:    host,
		Headers: map[string]string{"Api-Key": "test-api-key"},
	})
	require.NoError(t, err)
	return client
}

func TestApiVersionConstantIs202607Unit(t *testing.T) {
	assert.Equal(t, "2026-07", gen.PineconeApiVersion)
}

func TestCreateServerlessIndexConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusCreated, conformanceIndexJSON
	})
	client := newConformanceClient(t, server.URL)

	dimension := int32(1536)
	metric := Dotproduct
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      "dense-index",
		Cloud:     Aws,
		Region:    "us-east-1",
		Dimension: &dimension,
		Metric:    &metric,
	})
	require.NoError(t, err)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/indexes", request.path)

	// The legacy dense declaration translates to the reserved "_values" schema field.
	fields := request.body["schema"].(map[string]interface{})["fields"].(map[string]interface{})
	denseField := fields["_values"].(map[string]interface{})
	assert.Equal(t, "dense_vector", denseField["type"])
	assert.Equal(t, float64(1536), denseField["dimension"])
	assert.Equal(t, "dotproduct", denseField["metric"])
	assert.NotContains(t, denseField, "description")

	deployment := request.body["deployment"].(map[string]interface{})
	assert.Equal(t, "managed", deployment["deployment_type"])
	assert.Equal(t, "aws", deployment["cloud"])
	assert.Equal(t, "us-east-1", deployment["region"])

	// The 2026-04 top-level keys must be gone.
	assert.NotContains(t, request.body, "dimension")
	assert.NotContains(t, request.body, "metric")
	assert.NotContains(t, request.body, "vector_type")
	assert.NotContains(t, request.body, "spec")
}

func TestCreateServerlessSparseIndexConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusCreated, conformanceIndexJSON
	})
	client := newConformanceClient(t, server.URL)

	vectorType := "sparse"
	metric := Dotproduct
	_, err := client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:       "sparse-index",
		Cloud:      Aws,
		Region:     "us-east-1",
		VectorType: &vectorType,
		Metric:     &metric,
	})
	require.NoError(t, err)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)

	// A sparse index translates to the reserved "_sparse_values" field with no dimension and no
	// metric; sparse scoring is not configurable in 2026-07.
	fields := request.body["schema"].(map[string]interface{})["fields"].(map[string]interface{})
	require.Contains(t, fields, "_sparse_values")
	sparseField := fields["_sparse_values"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"type": "sparse_vector"}, sparseField)
	assert.NotContains(t, fields, "_values")
}

func TestCreatePodIndexUnsupportedUnit(t *testing.T) {
	// The 2026-07 backend rejects deployment_type "pod" at creation; the SDK fails fast with a
	// guided error before any request is sent.
	client := newConformanceClient(t, "http://localhost:1")

	_, err := client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        "pod-index",
		Dimension:   3,
		Environment: "us-west1-gcp",
		PodType:     "p1.x2",
		Replicas:    2,
		Shards:      1,
	})
	require.ErrorContains(t, err, "creating pod indexes is not supported by the 2026-07 API")
}

func TestCreateBYOCIndexConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusCreated, conformanceIndexJSON
	})
	client := newConformanceClient(t, server.URL)

	dimension := int32(3)
	_, err := client.CreateBYOCIndex(context.Background(), &CreateBYOCIndexRequest{
		Name:        "byoc-index",
		Environment: "my-environment",
		Dimension:   &dimension,
	})
	require.NoError(t, err)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)

	deployment := request.body["deployment"].(map[string]interface{})
	assert.Equal(t, "byoc", deployment["deployment_type"])
	assert.Equal(t, "my-environment", deployment["environment"])
}

func TestCreateIndexSchemaFirstConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusCreated, conformanceIndexJSON
	})
	client := newConformanceClient(t, server.URL)

	language := "en"
	_, err := client.CreateIndex(context.Background(), &CreateIndexRequest{
		Name: "hybrid",
		Schema: IndexSchema{
			Fields: map[string]IndexSchemaField{
				"embedding":    {DenseVector: &DenseVectorField{Dimension: 1536, Metric: Dotproduct}},
				"sparse_terms": {SparseVector: &SparseVectorField{}},
				"title":        {String: &StringField{FullTextSearch: &FullTextSearchConfig{Language: &language}}},
			},
		},
		Deployment: &IndexDeployment{Managed: &ManagedDeployment{Cloud: Aws, Region: "us-east-1"}},
	})
	require.NoError(t, err)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)

	fields := request.body["schema"].(map[string]interface{})["fields"].(map[string]interface{})
	dense := fields["embedding"].(map[string]interface{})
	assert.Equal(t, "dense_vector", dense["type"])
	assert.Equal(t, float64(1536), dense["dimension"])
	assert.Equal(t, "dotproduct", dense["metric"])
	// A sparse field carries neither dimension nor metric.
	assert.Equal(t, map[string]interface{}{"type": "sparse_vector"}, fields["sparse_terms"])
	title := fields["title"].(map[string]interface{})
	assert.Equal(t, "string", title["type"])
	assert.Equal(t, map[string]interface{}{"language": "en"}, title["full_text_search"])

	deployment := request.body["deployment"].(map[string]interface{})
	assert.Equal(t, "managed", deployment["deployment_type"])
}

func TestCreateIndexSchemaValidationUnit(t *testing.T) {
	client := newConformanceClient(t, "http://localhost:1")
	ctx := context.Background()

	_, err := client.CreateIndex(ctx, &CreateIndexRequest{Name: "x"})
	require.ErrorContains(t, err, "Schema must contain at least one field")

	// Metadata fields are not declared at creation time.
	_, err = client.CreateIndex(ctx, &CreateIndexRequest{Name: "x", Schema: IndexSchema{
		Fields: map[string]IndexSchemaField{"year": {Integer: &IntegerField{}}},
	}})
	require.ErrorContains(t, err, "metadata fields are not declared in the schema")

	// A string field without full-text search has no create-time meaning.
	_, err = client.CreateIndex(ctx, &CreateIndexRequest{Name: "x", Schema: IndexSchema{
		Fields: map[string]IndexSchemaField{"title": {String: &StringField{}}},
	}})
	require.ErrorContains(t, err, "only with a full_text_search configuration")

	// Semantic text fields come from CreateIndexForModel.
	_, err = client.CreateIndex(ctx, &CreateIndexRequest{Name: "x", Schema: IndexSchema{
		Fields: map[string]IndexSchemaField{"text": {SemanticText: &SemanticTextField{Model: "multilingual-e5-large"}}},
	}})
	require.ErrorContains(t, err, "use CreateIndexForModel")
}

func TestConfigureIndexConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		if r.Method == http.MethodGet {
			return http.StatusOK, conformancePodIndexJSON
		}
		return http.StatusOK, conformancePodIndexJSON
	})
	client := newConformanceClient(t, server.URL)

	_, err := client.ConfigureIndex(context.Background(), "pod-index", ConfigureIndexParams{
		PodType:  "p1.x4",
		Replicas: 4,
	})
	require.NoError(t, err)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, http.MethodPatch, request.method)

	// Pod scaling nests under deployment (no deployment_type key), replacing the 2026-04 spec.pod.
	deployment := request.body["deployment"].(map[string]interface{})
	assert.Equal(t, "p1.x4", deployment["pod_type"])
	assert.Equal(t, float64(4), deployment["replicas"])
	assert.NotContains(t, deployment, "deployment_type")
	assert.NotContains(t, request.body, "spec")
	assert.NotContains(t, request.body, "embed")
}

func TestLegacyCreateGuidedErrorsUnit(t *testing.T) {
	// Guided errors fire before any HTTP request, so no server is needed.
	client := newConformanceClient(t, "http://localhost:1")
	ctx := context.Background()
	sourceCollection := "my-collection"
	dimension := int32(3)

	_, err := client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name: "x", Cloud: Aws, Region: "us-east-1", Dimension: &dimension, SourceCollection: &sourceCollection,
	})
	require.ErrorContains(t, err, "SourceCollection is not supported by the 2026-07 API")

	_, err = client.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name: "x", Cloud: Aws, Region: "us-east-1", Dimension: &dimension, Schema: &MetadataSchema{},
	})
	require.ErrorContains(t, err, "Schema (metadata schema) is not supported by the 2026-07 API")

	_, err = client.CreatePodIndex(ctx, &CreatePodIndexRequest{
		Name: "x", Dimension: 3, Environment: "e", PodType: "p1.x1",
	})
	require.ErrorContains(t, err, "creating pod indexes is not supported by the 2026-07 API")

	_, err = client.CreateBYOCIndex(ctx, &CreateBYOCIndexRequest{
		Name: "x", Environment: "e", Dimension: &dimension, Schema: &MetadataSchema{},
	})
	require.ErrorContains(t, err, "Schema (metadata schema) is not supported by the 2026-07 API")

	_, err = client.ConfigureIndex(ctx, "x", ConfigureIndexParams{Embed: &ConfigureIndexEmbed{}})
	require.ErrorContains(t, err, "Embed is not supported by the 2026-07 API")
}

func newConformanceIndexConnection(t *testing.T, host string, namespace string) *IndexConnection {
	// Client.Index forces https on the REST host, so build the connection directly to keep the
	// httptest server's plain-http URL.
	dbDataClient, err := db_data_rest.NewClient(host)
	require.NoError(t, err)
	idx, err := newIndexConnection(newIndexParameters{
		host:               host,
		namespace:          namespace,
		additionalMetadata: map[string]string{"Api-Key": "test-api-key"},
		dbDataClient:       dbDataClient,
	})
	require.NoError(t, err)
	return idx
}

func TestUpsertDocumentsConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusAccepted, `{"upserted_count": 1}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "movies")

	res, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{
		Documents: []Document{{"_id": "doc-1", "genre": "drama"}},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), res.UpsertedCount)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/namespaces/movies/documents/upsert", request.path)
	documents := request.body["documents"].([]interface{})
	require.Len(t, documents, 1)
	assert.Equal(t, "doc-1", documents[0].(map[string]interface{})["_id"])
}

func TestUpsertDocumentsDefaultNamespaceConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusAccepted, `{"upserted_count": 1}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "")

	_, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{
		Documents: []Document{{"_id": "doc-1"}},
	})
	require.NoError(t, err)

	// The default namespace is addressed as "__default__" in REST paths.
	assert.Equal(t, "/namespaces/__default__/documents/upsert", capture.last(t).path)
}

func TestSearchDocumentsConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusOK, `{"matches": [{"_id": "doc-1", "_score": 0.9, "genre": "drama"}], "namespace": "movies", "usage": {"read_units": 5}}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "movies")

	query := "family movie"
	res, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
		TopK: 10,
		ScoreBy: []DocumentScoringMethod{
			{Type: "text", Fields: []string{"title", "plot"}, Query: &query},
		},
		IncludeFields: []string{"*"},
	})
	require.NoError(t, err)
	require.Len(t, res.Matches, 1)
	assert.Equal(t, "doc-1", res.Matches[0].Id)
	require.NotNil(t, res.Matches[0].Score)
	assert.InDelta(t, 0.9, float64(*res.Matches[0].Score), 1e-6)
	// Requested fields arrive flattened on the wire and are collected into Fields.
	assert.Equal(t, "drama", res.Matches[0].Fields["genre"])

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, "/namespaces/movies/documents/search", request.path)
	assert.Equal(t, float64(10), request.body["top_k"])
	scoreBy := request.body["score_by"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "text", scoreBy["type"])
	assert.Equal(t, "family movie", scoreBy["query"])
}

func TestSearchDocumentsScoreByValidationUnit(t *testing.T) {
	idx := newConformanceIndexConnection(t, "http://localhost:1", "movies")
	ctx := context.Background()
	query := "q"

	_, err := idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 0, ScoreBy: []DocumentScoringMethod{{Type: "text", Query: &query}}})
	require.ErrorContains(t, err, "TopK must be at least 1")

	_, err = idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 10})
	require.ErrorContains(t, err, "ScoreBy must contain at least one")

	// A dense_vector clause must appear on its own.
	values := []float32{0.1}
	_, err = idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 10, ScoreBy: []DocumentScoringMethod{
		{Type: "text", Fields: []string{"plot"}, Query: &query},
		{Type: "dense_vector", Fields: []string{"embedding"}, Values: &values},
	}})
	require.ErrorContains(t, err, `a "dense_vector" method must appear on its own`)
}

func TestFetchDocumentsConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusOK, `{"documents": {"doc-1": {"_id": "doc-1", "genre": "drama"}}, "namespace": "movies", "usage": {"read_units": 1}}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "movies")

	res, err := idx.FetchDocuments(context.Background(), &FetchDocumentsRequest{Ids: []string{"doc-1"}})
	require.NoError(t, err)
	require.Contains(t, res.Documents, "doc-1")
	assert.Equal(t, "drama", res.Documents["doc-1"]["genre"])

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, "/namespaces/movies/documents/fetch", request.path)
	assert.Equal(t, []interface{}{"doc-1"}, request.body["ids"])
}

func TestFetchDocumentsSelectorValidationUnit(t *testing.T) {
	idx := newConformanceIndexConnection(t, "http://localhost:1", "movies")
	ctx := context.Background()

	_, err := idx.FetchDocuments(ctx, &FetchDocumentsRequest{})
	require.ErrorContains(t, err, "exactly one of Ids or Filter")

	_, err = idx.FetchDocuments(ctx, &FetchDocumentsRequest{Ids: []string{"a"}, Filter: map[string]interface{}{"genre": "drama"}})
	require.ErrorContains(t, err, "exactly one of Ids or Filter")

	_, err = idx.FetchDocuments(ctx, &FetchDocumentsRequest{Filter: map[string]interface{}{}})
	require.ErrorContains(t, err, "Filter must not be empty")
}

func TestDeleteDocumentsConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusAccepted, `{"matched_records": 3}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "movies")

	res, err := idx.DeleteDocuments(context.Background(), &DeleteDocumentsRequest{
		Filter: map[string]interface{}{"genre": map[string]interface{}{"$eq": "drama"}},
	})
	require.NoError(t, err)
	require.NotNil(t, res.MatchedRecords)
	assert.Equal(t, int32(3), *res.MatchedRecords)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, "/namespaces/movies/documents/delete", request.path)
	assert.Contains(t, request.body, "filter")
	assert.NotContains(t, request.body, "ids")
	assert.NotContains(t, request.body, "delete_all")
}

func TestDeleteDocumentsSelectorValidationUnit(t *testing.T) {
	idx := newConformanceIndexConnection(t, "http://localhost:1", "movies")
	ctx := context.Background()

	_, err := idx.DeleteDocuments(ctx, &DeleteDocumentsRequest{})
	require.ErrorContains(t, err, "exactly one of Ids, Filter, or DeleteAll")

	_, err = idx.DeleteDocuments(ctx, &DeleteDocumentsRequest{Ids: []string{"a"}, DeleteAll: true})
	require.ErrorContains(t, err, "exactly one of Ids, Filter, or DeleteAll")

	_, err = idx.DeleteDocuments(ctx, &DeleteDocumentsRequest{Filter: map[string]interface{}{}})
	require.ErrorContains(t, err, "Filter must not be empty")
}

func TestUpdateDocumentsConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusAccepted, `{"matched_records": 2}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "movies")

	res, err := idx.UpdateDocuments(context.Background(), &UpdateDocumentsRequest{
		Filter:       map[string]interface{}{"genre": "drama"},
		SetFields:    map[string]interface{}{"reviewed": true},
		RemoveFields: []string{"draft"},
	})
	require.NoError(t, err)
	require.NotNil(t, res.MatchedRecords)
	assert.Equal(t, int32(2), *res.MatchedRecords)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, "/namespaces/movies/documents/update", request.path)
	assert.Contains(t, request.body, "filter")
	assert.Equal(t, map[string]interface{}{"reviewed": true}, request.body["set_fields"])
	assert.Equal(t, []interface{}{"draft"}, request.body["remove_fields"])
}

func TestUpdateDocumentsSelectorValidationUnit(t *testing.T) {
	idx := newConformanceIndexConnection(t, "http://localhost:1", "movies")
	ctx := context.Background()

	_, err := idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{})
	require.ErrorContains(t, err, "either Documents or a filtered patch")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{
		Documents: []Document{{"_id": "a"}},
		Filter:    map[string]interface{}{"genre": "drama"},
	})
	require.ErrorContains(t, err, "either Documents or a filtered patch")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{Filter: map[string]interface{}{"genre": "drama"}})
	require.ErrorContains(t, err, "must set SetFields and/or RemoveFields")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{SetFields: map[string]interface{}{"a": 1}})
	require.ErrorContains(t, err, "only valid together with Filter")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{Documents: []Document{{"genre": "drama"}}})
	require.ErrorContains(t, err, `must have an "_id" field`)
}

func TestListDocumentsConformanceUnit(t *testing.T) {
	server, capture := newConformanceServer(t, func(r *http.Request) (int, string) {
		return http.StatusOK, `{"documents": [{"_id": "doc-1"}], "namespace": "movies", "pagination": {"next": "token-1"}, "usage": {"read_units": 1}}`
	})
	idx := newConformanceIndexConnection(t, server.URL, "movies")

	prefix := "doc-"
	res, err := idx.ListDocuments(context.Background(), &ListDocumentsRequest{Prefix: &prefix})
	require.NoError(t, err)
	require.Len(t, res.Documents, 1)
	assert.Equal(t, "doc-1", res.Documents[0].Id)
	require.NotNil(t, res.Pagination)
	assert.Equal(t, "token-1", res.Pagination.Next)

	request := capture.last(t)
	assert.Equal(t, "2026-07", request.apiVersion)
	assert.Equal(t, "/namespaces/movies/documents/list", request.path)
	assert.Equal(t, "doc-", request.body["prefix"])
}

func TestVectorOpValidationParityUnit(t *testing.T) {
	idx := newConformanceIndexConnection(t, "http://localhost:1", "movies")
	ctx := context.Background()

	_, err := idx.QueryByVectorValues(ctx, &QueryByVectorValuesRequest{Vector: []float32{0.1}, TopK: 20000})
	require.ErrorContains(t, err, "TopK must be between 1 and 10000")

	_, err = idx.QueryByVectorId(ctx, &QueryByVectorIdRequest{VectorId: "a", TopK: 0})
	require.ErrorContains(t, err, "TopK must be between 1 and 10000")

	_, err = idx.FetchVectors(ctx, []string{})
	require.ErrorContains(t, err, "at least one vector ID")

	longId := make([]byte, 600)
	for i := range longId {
		longId[i] = 'a'
	}
	_, err = idx.FetchVectors(ctx, []string{string(longId)})
	require.ErrorContains(t, err, "exceeds the maximum length of 512")

	err = idx.DeleteVectorsByFilter(ctx, &MetadataFilter{})
	require.ErrorContains(t, err, "empty metadata filter is not allowed")

	limit := uint32(500)
	_, err = idx.ListVectors(ctx, &ListVectorsRequest{Limit: &limit})
	require.ErrorContains(t, err, "Limit must be between 1 and 100")
}
