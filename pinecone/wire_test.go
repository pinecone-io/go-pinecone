package pinecone

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/pinecone-io/go-pinecone/v7/internal/gen"
	db_data_rest "github.com/pinecone-io/go-pinecone/v7/internal/gen/db_data/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run public SDK methods against an httptest server and pin what goes over the wire:
// the method, path, query, X-Pinecone-Api-Version header, and full JSON body of every request, plus
// how responses decode into SDK types.

const (
	managedDeploymentJSON = `{"deployment_type": "managed", "cloud": "aws", "region": "us-east-1"}`
	podDeploymentJSON     = `{"deployment_type": "pod", "environment": "us-east1-gcp", "pod_type": "p1.x1", "replicas": 1, "shards": 1}`
	byocDeploymentJSON    = `{"deployment_type": "byoc", "environment": "test-environ"}`
	denseFieldsJSON       = `{"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"}}`
)

// indexModelFixture builds an IndexModel response body. Empty fields fall back to a ready managed
// index named "test-index" with a single dense field.
type indexModelFixture struct {
	name       string
	state      IndexStatusState
	deployment string
	fields     string
	tags       string
}

func (f indexModelFixture) JSON() string {
	if f.name == "" {
		f.name = "test-index"
	}
	if f.state == "" {
		f.state = IndexStatusStateReady
	}
	if f.deployment == "" {
		f.deployment = managedDeploymentJSON
	}
	if f.fields == "" {
		f.fields = denseFieldsJSON
	}
	if f.tags == "" {
		f.tags = "null"
	}
	return fmt.Sprintf(`{
		"name": %q,
		"host": "https://test-host.pinecone.io",
		"deletion_protection": "disabled",
		"status": {"ready": %t, "state": %q},
		"deployment": %s,
		"schema": {"fields": %s},
		"tags": %s
	}`, f.name, f.state == IndexStatusStateReady, f.state, f.deployment, f.fields, f.tags)
}

type wireRequest struct {
	method     string
	path       string
	query      string
	apiVersion string
	body       string
}

// wireServer answers every request with a fixed status and body, recording each request.
type wireServer struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []wireRequest
}

func newWireServer(t *testing.T, status int, response string) *wireServer {
	t.Helper()
	s := &wireServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		s.mu.Lock()
		s.reqs = append(s.reqs, wireRequest{
			method:     r.Method,
			path:       r.URL.Path,
			query:      r.URL.RawQuery,
			apiVersion: r.Header.Get("X-Pinecone-Api-Version"),
			body:       string(body),
		})
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *wireServer) requests() []wireRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]wireRequest(nil), s.reqs...)
}

func newWireClient(t *testing.T, host string) *Client {
	t.Helper()
	pc, err := NewClientBase(NewClientBaseParams{
		Host:    host,
		Headers: map[string]string{"Api-Key": "test-api-key"},
	})
	require.NoError(t, err)
	return pc
}

func newWireIndexConnection(t *testing.T, host string, namespace string) *IndexConnection {
	t.Helper()
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

// wantRequest is an expected request. An empty body means the request must have no body; a
// non-empty body is compared as JSON.
type wantRequest struct {
	method string
	path   string
	query  string
	body   string
}

func assertRequests(t *testing.T, got []wireRequest, want []wantRequest) {
	t.Helper()
	require.Len(t, got, len(want), "number of requests sent")
	for i, w := range want {
		assert.Equal(t, gen.PineconeApiVersion, got[i].apiVersion, "request %d api version", i)
		assert.Equal(t, w.method, got[i].method, "request %d method", i)
		assert.Equal(t, w.path, got[i].path, "request %d path", i)
		assert.Equal(t, w.query, got[i].query, "request %d query", i)
		if w.body == "" {
			assert.Empty(t, got[i].body, "request %d body", i)
		} else {
			assert.JSONEq(t, w.body, got[i].body, "request %d body", i)
		}
	}
}

func TestPineconeApiVersionUnit(t *testing.T) {
	assert.Equal(t, "2026-07", gen.PineconeApiVersion)
}

func TestControlPlaneWireUnit(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		response string
		call     func(t *testing.T, pc *Client)
		want     []wantRequest
	}{
		{
			// Dimension and Metric map to the reserved "_values" schema field.
			name:     "CreateServerlessIndex dense",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "dense-index"}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
					Name:      "dense-index",
					Cloud:     CloudAWS,
					Region:    "us-east-1",
					Dimension: ptr(int32(1536)),
					Metric:    ptr(IndexMetricDotproduct),
				})
				require.NoError(t, err)
				assert.Equal(t, "dense-index", idx.Name)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "dense-index",
					"schema": {"fields": {"_values": {"type": "dense_vector", "dimension": 1536, "metric": "dotproduct"}}},
					"deployment": ` + managedDeploymentJSON + `,
					"tags": null
				}`,
			}},
		},
		{
			// A sparse index maps to "_sparse_values" with no dimension and no metric; sparse scoring
			// is not configurable.
			name:   "CreateServerlessIndex sparse",
			status: http.StatusCreated,
			response: indexModelFixture{
				name:   "sparse-index",
				fields: `{"_sparse_values": {"type": "sparse_vector"}}`,
			}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
					Name:       "sparse-index",
					Cloud:      CloudAWS,
					Region:     "us-east-1",
					VectorType: ptr("sparse"),
					Metric:     ptr(IndexMetricDotproduct),
				})
				require.NoError(t, err)
				assert.Equal(t, "sparse-index", idx.Name)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "sparse-index",
					"schema": {"fields": {"_sparse_values": {"type": "sparse_vector"}}},
					"deployment": ` + managedDeploymentJSON + `,
					"tags": null
				}`,
			}},
		},
		{
			name:     "CreateBYOCIndex",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "byoc-index", deployment: byocDeploymentJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateBYOCIndex(context.Background(), &CreateBYOCIndexRequest{
					Name:        "byoc-index",
					Environment: "test-environ",
					Dimension:   ptr(int32(3)),
				})
				require.NoError(t, err)
				assert.Equal(t, "byoc-index", idx.Name)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "byoc-index",
					"schema": {"fields": {"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"}}},
					"deployment": ` + byocDeploymentJSON + `,
					"deletion_protection": "disabled",
					"tags": null
				}`,
			}},
		},
		{
			name:   "CreateIndex with dense, sparse, and full-text search fields",
			status: http.StatusCreated,
			response: indexModelFixture{
				name: "hybrid",
				fields: `{
					"embedding": {"type": "dense_vector", "dimension": 1536, "metric": "dotproduct"},
					"sparse_terms": {"type": "sparse_vector"},
					"title": {"type": "string", "full_text_search": {"language": "en"}}
				}`,
			}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateIndex(context.Background(), &CreateIndexRequest{
					Name: "hybrid",
					Schema: IndexSchema{
						Fields: map[string]IndexSchemaField{
							"embedding":    {DenseVector: &DenseVectorField{Dimension: 1536, Metric: IndexMetricDotproduct}},
							"sparse_terms": {SparseVector: &SparseVectorField{}},
							"title":        {String: &StringField{FullTextSearch: &FullTextSearchConfig{Language: ptr("en")}}},
						},
					},
					Deployment: &IndexDeployment{Managed: &ManagedDeployment{Cloud: CloudAWS, Region: "us-east-1"}},
				})
				require.NoError(t, err)
				assert.Equal(t, "hybrid", idx.Name)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "hybrid",
					"schema": {"fields": {
						"embedding": {"type": "dense_vector", "dimension": 1536, "metric": "dotproduct"},
						"sparse_terms": {"type": "sparse_vector"},
						"title": {"type": "string", "full_text_search": {"language": "en"}}
					}},
					"deployment": ` + managedDeploymentJSON + `,
					"tags": null
				}`,
			}},
		},
		{
			name:   "CreateIndex with integrated embedding",
			status: http.StatusCreated,
			response: indexModelFixture{
				name: "articles",
				fields: `{
					"body": {"type": "string", "embed": {"model": "llama-text-embed-v2", "dimension": 1024}},
					"summary": {"type": "string", "sparse_embed": {}, "store_text": false}
				}`,
			}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateIndex(context.Background(), &CreateIndexRequest{
					Name: "articles",
					Schema: IndexSchema{
						Fields: map[string]IndexSchemaField{
							"body": {String: &StringField{Embed: &EmbedConfig{
								Model:          ptr("llama-text-embed-v2"),
								Dimension:      ptr(int32(1024)),
								ReadParameters: &map[string]interface{}{"truncate": "END"},
							}}},
							"summary": {String: &StringField{SparseEmbed: &SparseEmbedConfig{}, StoreText: ptr(false)}},
						},
					},
				})
				require.NoError(t, err)
				assert.Equal(t, "articles", idx.Name)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "articles",
					"schema": {"fields": {
						"body": {"type": "string", "embed": {
							"model": "llama-text-embed-v2",
							"dimension": 1024,
							"read_parameters": {"truncate": "END"}
						}},
						"summary": {"type": "string", "sparse_embed": {}, "store_text": false}
					}},
					"tags": null
				}`,
			}},
		},
		{
			// ConfigureIndex describes the index first: PodType and Replicas are only accepted because
			// the GET response reports a pod deployment. Pod scaling nests under deployment with no
			// deployment_type key.
			name:     "ConfigureIndex pod scaling",
			status:   http.StatusOK,
			response: indexModelFixture{name: "pod-index", deployment: podDeploymentJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "pod-index", ConfigureIndexParams{
					PodType:  "p1.x4",
					Replicas: 4,
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/pod-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/pod-index",
					body:   `{"deployment": {"pod_type": "p1.x4", "replicas": 4}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex schema",
			status:   http.StatusOK,
			response: indexModelFixture{name: "integrated-index"}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "integrated-index", ConfigureIndexParams{
					Schema: &ConfigureIndexSchema{Fields: map[string]ConfigureSemanticTextField{
						"chunk_text": {
							Model:          ptr("multilingual-e5-large"),
							ReadParameters: &map[string]interface{}{"input_type": "query", "truncate": "NONE"},
						},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/integrated-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/integrated-index",
					body: `{"schema": {"fields": {"chunk_text": {
						"type": "semantic_text",
						"model": "multilingual-e5-large",
						"read_parameters": {"input_type": "query", "truncate": "NONE"}
					}}}, "tags": null}`,
				},
			},
		},
		{
			// Tags already on the index are not echoed back; the API merges the sent tags into them.
			name:   "ConfigureIndex tags",
			status: http.StatusOK,
			response: indexModelFixture{
				name: "tagged-index",
				tags: `{"env": "dev", "team": "search", "owner": "alice"}`,
			}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "tagged-index", ConfigureIndexParams{
					Tags: IndexTags{"env": "prod", "owner": ""},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/tagged-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/tagged-index",
					body:   `{"tags": {"env": "prod", "owner": ""}}`,
				},
			},
		},
		{
			name:     "DescribeIndex Failed state",
			status:   http.StatusOK,
			response: indexModelFixture{state: IndexStatusStateFailed}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.DescribeIndex(context.Background(), "test-index")
				require.NoError(t, err)
				assert.Equal(t, IndexStatusStateFailed, idx.Status.State)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/test-index"}},
		},
		{
			name:     "DescribeIndex Disabled state",
			status:   http.StatusOK,
			response: indexModelFixture{state: IndexStatusStateDisabled}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.DescribeIndex(context.Background(), "test-index")
				require.NoError(t, err)
				assert.Equal(t, IndexStatusStateDisabled, idx.Status.State)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/test-index"}},
		},
		{
			name:     "ListCollections with no collections key",
			status:   http.StatusOK,
			response: `{}`,
			call: func(t *testing.T, pc *Client) {
				collections, err := pc.ListCollections(context.Background())
				require.NoError(t, err)
				assert.Empty(t, collections)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/collections"}},
		},
		{
			name:     "CreateIndexFromBackup with read capacity",
			status:   http.StatusAccepted,
			response: `{"index_id": "idx-1", "restore_job_id": "rj-1"}`,
			call: func(t *testing.T, pc *Client) {
				res, err := pc.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{
					BackupId: "backup-1",
					Name:     "restored-index",
					ReadCapacity: &ReadCapacityParams{
						Dedicated: &ReadCapacityDedicatedConfig{
							NodeType: ptr("t1"),
							Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(1)), Shards: ptr(int32(1))}},
						},
					},
				})
				require.NoError(t, err)
				assert.Equal(t, "idx-1", res.IndexId)
				assert.Equal(t, "rj-1", res.RestoreJobId)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/backups/backup-1/create-index",
				body: `{
					"name": "restored-index",
					"read_capacity": {"mode": "Dedicated", "dedicated": {
						"node_type": "t1",
						"scaling": "Manual",
						"manual": {"replicas": 1, "shards": 1}
					}},
					"tags": null
				}`,
			}},
		},
		{
			name:     "ListBackups for an index including deleted",
			status:   http.StatusOK,
			response: `{"data": []}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ListBackups(context.Background(), &ListBackupsParams{
					IndexName:      ptr("my-index"),
					IncludeDeleted: ptr(true),
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodGet,
				path:   "/indexes/my-index/backups",
				query:  "include_deleted=true",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWireServer(t, tt.status, tt.response)
			tt.call(t, newWireClient(t, server.URL))
			assertRequests(t, server.requests(), tt.want)
		})
	}
}

func TestDataPlaneWireUnit(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		status    int
		response  string
		call      func(t *testing.T, idx *IndexConnection)
		want      []wantRequest
	}{
		{
			name:      "UpsertDocuments",
			namespace: "movies",
			status:    http.StatusAccepted,
			response:  `{"upserted_count": 1}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{
					Documents: []Document{{"_id": "doc-1", "genre": "drama"}},
				})
				require.NoError(t, err)
				assert.Equal(t, int32(1), res.UpsertedCount)
				assert.Nil(t, res.Usage)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/upsert",
				body:   `{"documents": [{"_id": "doc-1", "genre": "drama"}]}`,
			}},
		},
		{
			// The default namespace is addressed as "__default__" in REST paths.
			name:     "UpsertDocuments default namespace",
			status:   http.StatusAccepted,
			response: `{"upserted_count": 1}`,
			call: func(t *testing.T, idx *IndexConnection) {
				_, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{
					Documents: []Document{{"_id": "doc-1"}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/__default__/documents/upsert",
				body:   `{"documents": [{"_id": "doc-1"}]}`,
			}},
		},
		{
			name:      "UpsertDocuments with embedding usage",
			namespace: "articles",
			status:    http.StatusAccepted,
			response:  `{"upserted_count": 1, "usage": {"embed_total_tokens": 12}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{
					Documents: []Document{{"_id": "doc-1", "body": "Lightweight trail running shoes"}},
				})
				require.NoError(t, err)
				assert.Equal(t, &DocumentWriteUsage{EmbedTotalTokens: 12}, res.Usage)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/articles/documents/upsert",
				body:   `{"documents": [{"_id": "doc-1", "body": "Lightweight trail running shoes"}]}`,
			}},
		},
		{
			// Requested fields arrive flattened on the wire and are collected into Fields.
			name:      "SearchDocuments text",
			namespace: "movies",
			status:    http.StatusOK,
			response:  `{"matches": [{"_id": "doc-1", "_score": 0.9, "genre": "drama"}], "namespace": "movies", "usage": {"read_units": 5}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
					TopK: 10,
					ScoreBy: []DocumentScoringMethod{
						{Type: DocumentScoringTypeText, Fields: []string{"title", "plot"}, Query: ptr("family movie")},
					},
					IncludeFields: []string{"*"},
				})
				require.NoError(t, err)
				require.Len(t, res.Matches, 1)
				assert.Equal(t, "doc-1", res.Matches[0].Id)
				require.NotNil(t, res.Matches[0].Score)
				assert.InDelta(t, 0.9, float64(*res.Matches[0].Score), 1e-6)
				assert.Equal(t, map[string]interface{}{"genre": "drama"}, res.Matches[0].Fields)
				assert.Equal(t, int32(5), res.Usage.ReadUnits)
				assert.Nil(t, res.Usage.EmbedTotalTokens)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/search",
				body: `{
					"top_k": 10,
					"score_by": [{"type": "text", "fields": ["title", "plot"], "query": "family movie"}],
					"include_fields": ["*"]
				}`,
			}},
		},
		{
			name:      "SearchDocuments embed",
			namespace: "articles",
			status:    http.StatusOK,
			response:  `{"matches": [{"_id": "doc-1", "_score": 0.8}], "namespace": "articles", "usage": {"read_units": 3, "embed_total_tokens": 4}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
					TopK:    5,
					ScoreBy: []DocumentScoringMethod{{Type: DocumentScoringTypeEmbed, Fields: []string{"body"}, Query: ptr("trail running shoes")}},
				})
				require.NoError(t, err)
				assert.Equal(t, int32(3), res.Usage.ReadUnits)
				require.NotNil(t, res.Usage.EmbedTotalTokens)
				assert.Equal(t, int32(4), *res.Usage.EmbedTotalTokens)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/articles/documents/search",
				body:   `{"top_k": 5, "score_by": [{"type": "embed", "fields": ["body"], "query": "trail running shoes"}]}`,
			}},
		},
		{
			name:      "SearchDocuments sparse_embed",
			namespace: "articles",
			status:    http.StatusOK,
			response:  `{"matches": [{"_id": "doc-1", "_score": 0.8}], "namespace": "articles", "usage": {"read_units": 3, "embed_total_tokens": 4}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
					TopK:    5,
					ScoreBy: []DocumentScoringMethod{{Type: DocumentScoringTypeSparseEmbed, Fields: []string{"body"}, Query: ptr("trail running shoes")}},
				})
				require.NoError(t, err)
				require.NotNil(t, res.Usage.EmbedTotalTokens)
				assert.Equal(t, int32(4), *res.Usage.EmbedTotalTokens)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/articles/documents/search",
				body:   `{"top_k": 5, "score_by": [{"type": "sparse_embed", "fields": ["body"], "query": "trail running shoes"}]}`,
			}},
		},
		{
			name:      "SearchDocuments API error",
			namespace: "articles",
			status:    http.StatusBadRequest,
			response:  `{"error": {"code": "INVALID_ARGUMENT", "message": "include_fields names field 'summary', which does not store its text"}, "status": 400}`,
			call: func(t *testing.T, idx *IndexConnection) {
				_, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
					TopK:          5,
					ScoreBy:       []DocumentScoringMethod{{Type: DocumentScoringTypeSparseEmbed, Fields: []string{"summary"}, Query: ptr("shoes")}},
					IncludeFields: []string{"summary"},
				})
				var pcErr *PineconeError
				require.ErrorAs(t, err, &pcErr)
				assert.Equal(t, http.StatusBadRequest, pcErr.Code)
				assert.Contains(t, pcErr.Error(), "does not store its text")
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/articles/documents/search",
				body: `{
					"top_k": 5,
					"score_by": [{"type": "sparse_embed", "fields": ["summary"], "query": "shoes"}],
					"include_fields": ["summary"]
				}`,
			}},
		},
		{
			name:      "FetchDocuments",
			namespace: "movies",
			status:    http.StatusOK,
			response:  `{"documents": {"doc-1": {"_id": "doc-1", "genre": "drama"}}, "namespace": "movies", "usage": {"read_units": 1}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.FetchDocuments(context.Background(), &FetchDocumentsRequest{Ids: []string{"doc-1"}})
				require.NoError(t, err)
				require.Contains(t, res.Documents, "doc-1")
				assert.Equal(t, "drama", res.Documents["doc-1"]["genre"])
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/fetch",
				body:   `{"ids": ["doc-1"]}`,
			}},
		},
		{
			name:      "DeleteDocuments by filter",
			namespace: "movies",
			status:    http.StatusAccepted,
			response:  `{"matched_records": 3}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.DeleteDocuments(context.Background(), &DeleteDocumentsRequest{
					Filter: map[string]interface{}{"genre": map[string]interface{}{"$eq": "drama"}},
				})
				require.NoError(t, err)
				require.NotNil(t, res.MatchedRecords)
				assert.Equal(t, int32(3), *res.MatchedRecords)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/delete",
				body:   `{"filter": {"genre": {"$eq": "drama"}}}`,
			}},
		},
		{
			name:      "UpdateDocuments filtered patch",
			namespace: "movies",
			status:    http.StatusAccepted,
			response:  `{"matched_records": 2}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.UpdateDocuments(context.Background(), &UpdateDocumentsRequest{
					Filter:       map[string]interface{}{"genre": "drama"},
					SetFields:    map[string]interface{}{"reviewed": true},
					RemoveFields: []string{"draft"},
				})
				require.NoError(t, err)
				require.NotNil(t, res.MatchedRecords)
				assert.Equal(t, int32(2), *res.MatchedRecords)
				assert.Nil(t, res.Usage)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/update",
				body:   `{"filter": {"genre": "drama"}, "set_fields": {"reviewed": true}, "remove_fields": ["draft"]}`,
			}},
		},
		{
			name:      "UpdateDocuments with embedding usage",
			namespace: "articles",
			status:    http.StatusAccepted,
			response:  `{"matched_records": 1, "usage": {"embed_total_tokens": 12}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.UpdateDocuments(context.Background(), &UpdateDocumentsRequest{
					Documents: []Document{{"_id": "doc-1", "body": "Waterproof trail running shoes"}},
				})
				require.NoError(t, err)
				assert.Equal(t, &DocumentWriteUsage{EmbedTotalTokens: 12}, res.Usage)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/articles/documents/update",
				body:   `{"documents": [{"_id": "doc-1", "body": "Waterproof trail running shoes"}]}`,
			}},
		},
		{
			name:      "ListDocuments",
			namespace: "movies",
			status:    http.StatusOK,
			response:  `{"documents": [{"_id": "doc-1"}], "namespace": "movies", "pagination": {"next": "token-1"}, "usage": {"read_units": 1}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.ListDocuments(context.Background(), &ListDocumentsRequest{Prefix: ptr("doc-")})
				require.NoError(t, err)
				require.Len(t, res.Documents, 1)
				assert.Equal(t, "doc-1", res.Documents[0].Id)
				require.NotNil(t, res.Pagination)
				assert.Equal(t, "token-1", res.Pagination.Next)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/list",
				body:   `{"prefix": "doc-"}`,
			}},
		},
		{
			name:     "ListImports with no data key",
			status:   http.StatusOK,
			response: `{}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.ListImports(context.Background(), nil)
				require.NoError(t, err)
				assert.Empty(t, res.Imports)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/bulk/imports"}},
		},
		{
			name:     "ListImports with pagination",
			status:   http.StatusOK,
			response: `{}`,
			call: func(t *testing.T, idx *IndexConnection) {
				_, err := idx.ListImports(context.Background(), &ListImportsRequest{
					Limit:           ptr(int32(10)),
					PaginationToken: ptr("next-page"),
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodGet,
				path:   "/bulk/imports",
				query:  "limit=10&paginationToken=next-page",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWireServer(t, tt.status, tt.response)
			tt.call(t, newWireIndexConnection(t, server.URL, tt.namespace))
			assertRequests(t, server.requests(), tt.want)
		})
	}
}
