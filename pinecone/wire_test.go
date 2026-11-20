package pinecone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pinecone-io/go-pinecone/v8/internal/gen"
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
	// semanticTextFieldsJSON is the schema of a legacy integrated index.
	semanticTextFieldsJSON = `{"chunk_text": {"type": "semantic_text", "model": "multilingual-e5-large", "dimension": 1024, "metric": "cosine"}}`

	autoscaledReadCapacityJSON      = `{"mode": "Dedicated", "dedicated": {"node_type": "b1", "shards": 2, "scaling": "Auto", "auto": {"min_replicas": 2, "max_replicas": 8}}, "status": {"state": "Ready", "current_replicas": 5, "current_shards": 2}}`
	dedicatedManualReadCapacityJSON = `{"mode": "Dedicated", "dedicated": {"node_type": "b1", "shards": 1, "scaling": "Manual", "manual": {"replicas": 2}}, "status": {"state": "Ready", "current_replicas": 2, "current_shards": 1}}`
)

// indexModelFixture builds an IndexModel response body. Empty fields fall back to a ready managed
// index named "test-index" with a single dense field.
type indexModelFixture struct {
	name       string
	state      IndexStatusState
	deployment string
	fields     string
	tags       string
	// readCapacity is the raw read_capacity JSON; empty omits the key.
	readCapacity string
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
	readCapacity := ""
	if f.readCapacity != "" {
		readCapacity = `"read_capacity": ` + f.readCapacity + `,`
	}
	return fmt.Sprintf(`{
		%s
		"name": %q,
		"host": "https://test-host.pinecone.io",
		"deletion_protection": "disabled",
		"status": {"ready": %t, "state": %q},
		"deployment": %s,
		"schema": {"fields": %s},
		"tags": %s
	}`, readCapacity, f.name, f.state == IndexStatusStateReady, f.state, f.deployment, f.fields, f.tags)
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
	idx, err := newWireClient(t, host).Index(NewIndexConnParams{Host: host, Namespace: namespace})
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })
	return idx
}

// An explicit http:// index host reaches a plain-HTTP server on every REST data-plane surface:
// documents, records, and imports.
func TestIndexRESTHonorsHTTPHost(t *testing.T) {
	server := newWireServer(t, http.StatusOK, `{}`)
	require.Contains(t, server.URL, "http://")
	idx := newWireIndexConnection(t, server.URL, "movies")
	ctx := context.Background()

	_, err := idx.ListDocuments(ctx, nil)
	require.NoError(t, err)
	_, err = idx.SearchRecords(ctx, &SearchRecordsRequest{
		Query: SearchRecordsQuery{TopK: 1, Inputs: &map[string]interface{}{"text": "q"}},
	})
	require.NoError(t, err)
	_, err = idx.ListImports(ctx, nil)
	require.NoError(t, err)

	got := server.requests()
	require.Len(t, got, 3)
	assert.Equal(t, "/namespaces/movies/documents/list", got[0].path)
	assert.Equal(t, "/records/namespaces/movies/search", got[1].path)
	assert.Equal(t, "/bulk/imports", got[2].path)
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
	assert.Equal(t, "2026-10", gen.PineconeApiVersion)
}

// openResponseIndexJSON is an index whose schema has a field type this SDK doesn't know, whose
// deployment has a deployment type it doesn't know, and which carries properties it doesn't know at
// every level.
const openResponseIndexJSON = `{
	"name": "future-index",
	"host": "https://future-index.pinecone.io",
	"deletion_protection": "disabled",
	"status": {"ready": true, "state": "Ready", "progress": 1},
	"deployment": {"deployment_type": "edge", "environment": "somewhere", "zone": "z1"},
	"schema": {"fields": {
		"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine", "quantization": "int8"},
		"location": {"type": "geo_point", "precision": 7},
		"genre": {"type": "string", "filterable": true, "collation": "en-US"}
	}, "version": 2},
	"tags": null,
	"embed_cache": {"enabled": true}
}`

// assertOpenResponseIndex checks that openResponseIndexJSON decodes with its known parts intact.
func assertOpenResponseIndex(t *testing.T, idx *Index) {
	t.Helper()
	require.NotNil(t, idx)
	assert.Equal(t, "future-index", idx.Name)
	assert.Nil(t, idx.Deployment)
	assert.Nil(t, idx.Spec)
	require.NotNil(t, idx.Schema)
	assert.Equal(t, map[string]IndexSchemaField{
		"_values": {DenseVector: &DenseVectorField{Dimension: 3, Metric: IndexMetricCosine}},
		"genre":   {String: &StringField{Filterable: ptr(true)}},
	}, idx.Schema.Fields)
	assert.Equal(t, ptr(int32(3)), idx.Dimension)
	assert.Equal(t, IndexMetricCosine, idx.Metric)
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
			name:     "CreateBYOCIndex with OnDemand read capacity",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "byoc-index", deployment: byocDeploymentJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateBYOCIndex(context.Background(), &CreateBYOCIndexRequest{
					Name:         "byoc-index",
					Environment:  "test-environ",
					Dimension:    ptr(int32(3)),
					ReadCapacity: &ReadCapacityParams{OnDemand: &ReadCapacityOnDemandConfig{}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "byoc-index",
					"schema": {"fields": {"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"}}},
					"deployment": ` + byocDeploymentJSON + `,
					"read_capacity": {"mode": "OnDemand"},
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
					"your_vector": {"type": "dense_vector", "dimension": 1536, "metric": "dotproduct"},
					"sparse_terms": {"type": "sparse_vector"},
					"title": {"type": "string", "full_text_search": {"language": "en"}}
				}`,
			}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateIndex(context.Background(), &CreateIndexRequest{
					Name: "hybrid",
					Schema: IndexSchema{
						Fields: map[string]IndexSchemaField{
							"your_vector":  {DenseVector: &DenseVectorField{Dimension: 1536, Metric: IndexMetricDotproduct}},
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
						"your_vector": {"type": "dense_vector", "dimension": 1536, "metric": "dotproduct"},
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
					"body": {"type": "string", "embed": {"model": "llama-text-embed-v2", "dimension": 1024, "metric": "cosine", "processing_location": "us"}},
					"summary": {"type": "string", "sparse_embed": {}, "store_text": false}
				}`,
			}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateIndex(context.Background(), &CreateIndexRequest{
					Name: "articles",
					Schema: IndexSchema{
						Fields: map[string]IndexSchemaField{
							"body": {String: &StringField{Embed: &EmbedConfig{
								Model:              ptr("llama-text-embed-v2"),
								Dimension:          ptr(int32(1024)),
								ProcessingLocation: ptr("us"),
								ReadParameters:     &map[string]interface{}{"truncate": "END"},
							}}},
							"summary": {String: &StringField{SparseEmbed: &SparseEmbedConfig{}, StoreText: ptr(false)}},
						},
					},
				})
				require.NoError(t, err)
				assert.Equal(t, "articles", idx.Name)
				require.NotNil(t, idx.Schema)
				body := idx.Schema.Fields["body"].String
				require.NotNil(t, body)
				require.NotNil(t, body.Embed)
				assert.Equal(t, ptr("us"), body.Embed.ProcessingLocation)
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
							"processing_location": "us",
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
			response: indexModelFixture{name: "integrated-index", fields: semanticTextFieldsJSON}.JSON(),
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
			// 2026-10 describes schema fields and deployments as open objects: a field or deployment
			// type this SDK doesn't know, and properties it doesn't know, must not fail the response.
			name:     "DescribeIndex tolerates unknown schema field types, deployment types, and properties",
			status:   http.StatusOK,
			response: openResponseIndexJSON,
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.DescribeIndex(context.Background(), "future-index")
				require.NoError(t, err)
				assertOpenResponseIndex(t, idx)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/future-index"}},
		},
		{
			name:     "ListIndexes tolerates unknown schema field types, deployment types, and properties",
			status:   http.StatusOK,
			response: `{"indexes": [` + openResponseIndexJSON + `], "incompatible_indexes": []}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIndexes(context.Background())
				require.NoError(t, err)
				require.Len(t, indexes, 1)
				assertOpenResponseIndex(t, indexes[0])
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes"}},
		},
		{
			name:   "ListBackups tolerates unknown schema field types and reads incompatible_data",
			status: http.StatusOK,
			response: `{"data": [{"backup_id": "bkp-1", "source_index_name": "docs", "source_index_id": "idx-1", "status": "Ready",
					"cloud": "aws", "region": "us-east-1", "created_at": "2026-10-01T00:00:00Z", "storage_tier": "cold",
					"schema": {"fields": {
						"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine", "quantization": "int8"},
						"location": {"type": "geo_point", "precision": 7}}}}],
				"pagination": null,
				"incompatible_data": [{"backup_id": "bkp-2", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				require.Len(t, backups.Data, 1)
				backup := backups.Data[0]
				require.NotNil(t, backup.Schema)
				assert.Equal(t, map[string]IndexSchemaField{
					"_values": {DenseVector: &DenseVectorField{Dimension: 3, Metric: IndexMetricCosine}},
				}, backup.Schema.Fields)
				assert.Equal(t, ptr(int32(3)), backup.Dimension)
				assert.Equal(t, []*IncompatibleBackup{{BackupId: "bkp-2", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, backups.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups"}},
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
						"shards": 1,
						"scaling": "Manual",
						"manual": {"replicas": 1}
					}},
					"tags": null
				}`,
			}},
		},
		{
			name:     "CreateIndexFromBackup into another region",
			status:   http.StatusAccepted,
			response: `{"index_id": "idx-1", "restore_job_id": "rj-1"}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{
					BackupId:   "backup-1",
					Name:       "restored-us-west-2",
					Deployment: &IndexDeployment{Managed: &ManagedDeployment{Cloud: CloudAWS, Region: "us-west-2"}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/backups/backup-1/create-index",
				body: `{
					"name": "restored-us-west-2",
					"deployment": {"deployment_type": "managed", "cloud": "aws", "region": "us-west-2"},
					"tags": null
				}`,
			}},
		},
		{
			name:     "CreateIndexFromBackup into a BYOC environment",
			status:   http.StatusAccepted,
			response: `{"index_id": "idx-1", "restore_job_id": "rj-1"}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{
					BackupId:   "backup-1",
					Name:       "restored-byoc-index",
					Deployment: &IndexDeployment{Byoc: &ByocDeployment{Environment: "aws-us-east-1-b921"}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/backups/backup-1/create-index",
				body: `{
					"name": "restored-byoc-index",
					"deployment": {"deployment_type": "byoc", "environment": "aws-us-east-1-b921"},
					"tags": null
				}`,
			}},
		},
		{
			// 2026-10 sends modalities and no modality; the deprecated Modality is filled from it.
			name:   "DescribeModel decodes modalities and fills the deprecated Modality",
			status: http.StatusOK,
			response: `{
				"model": "example-multimodal-embedding-model",
				"short_description": "An example multimodal embedding model.",
				"type": "embed",
				"modalities": ["image", "text", "pdf"],
				"supported_parameters": []
			}`,
			call: func(t *testing.T, pc *Client) {
				model, err := pc.Inference.DescribeModel(context.Background(), "example-multimodal-embedding-model")
				require.NoError(t, err)
				assert.Equal(t, &[]string{"image", "text", "pdf"}, model.Modalities)
				require.NotNil(t, model.Modality)
				assert.Equal(t, "text", *model.Modality)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/models/example-multimodal-embedding-model"}},
		},
		{
			name:   "ListModels sends the modality filter and fills the deprecated Modality on each model",
			status: http.StatusOK,
			response: `{"models": [
				{"model": "text-model", "short_description": "d", "type": "embed", "modalities": ["text"], "supported_parameters": []},
				{"model": "image-model", "short_description": "d", "type": "embed", "modalities": ["image"], "supported_parameters": []},
				{"model": "image-pdf-model", "short_description": "d", "type": "embed", "modalities": ["image", "pdf"], "supported_parameters": []},
				{"model": "legacy-model", "short_description": "d", "type": "embed", "modality": "text", "supported_parameters": []}]}`,
			call: func(t *testing.T, pc *Client) {
				list, err := pc.Inference.ListModels(context.Background(), &ListModelsParams{
					Type:       ptr("embed"),
					VectorType: ptr("dense"),
					Modality:   ptr("image"),
				})
				require.NoError(t, err)
				require.NotNil(t, list.Models)
				models := *list.Models
				require.Len(t, models, 4)
				assert.Equal(t, ptr("text"), models[0].Modality)
				assert.Equal(t, ptr("image"), models[1].Modality)
				assert.Nil(t, models[2].Modality, "no single modality describes an image and pdf model")
				assert.Equal(t, &[]string{"image", "pdf"}, models[2].Modalities)
				assert.Equal(t, ptr("text"), models[3].Modality, "a response's own modality is kept")
				assert.Nil(t, models[3].Modalities)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/models", query: "modality=image&type=embed&vector_type=dense"}},
		},
		{
			// A v7 call with no Modality sends no modality parameter.
			name:     "ListModels without Modality sends only the filters that are set",
			status:   http.StatusOK,
			response: `{"models": []}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.Inference.ListModels(context.Background(), &ListModelsParams{Type: ptr("rerank")})
				require.NoError(t, err)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/models", query: "type=rerank"}},
		},
		{
			name:     "ListModels returns the 400 for an unknown modality as a PineconeError",
			status:   http.StatusBadRequest,
			response: `{"error": {"code": "INVALID_ARGUMENT", "message": "Invalid modality 'video'."}, "status": 400}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.Inference.ListModels(context.Background(), &ListModelsParams{Modality: ptr("video")})
				var pcErr *PineconeError
				require.ErrorAs(t, err, &pcErr)
				assert.Equal(t, http.StatusBadRequest, pcErr.Code)
				assert.Equal(t, "INVALID_ARGUMENT", pcErr.ErrorCode)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/models", query: "modality=video"}},
		},
		{
			name:   "DescribeModel decodes supported_on",
			status: http.StatusOK,
			response: `{
				"model": "gemini-embedding-2",
				"short_description": "A dense embedding model",
				"type": "embed",
				"supported_on": ["standalone_inference", "document_index"]
			}`,
			call: func(t *testing.T, pc *Client) {
				model, err := pc.Inference.DescribeModel(context.Background(), "gemini-embedding-2")
				require.NoError(t, err)
				require.NotNil(t, model.SupportedOn)
				assert.Equal(t, []string{"standalone_inference", "document_index"}, *model.SupportedOn)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/models/gemini-embedding-2"}},
		},
		{
			// The v7.2.0-documented Manual shape: shards move from manual to dedicated on the wire.
			name:     "CreateServerlessIndex with Dedicated Manual read capacity",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "dedicated-index"}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
					Name:      "dedicated-index",
					Cloud:     CloudAWS,
					Region:    "us-east-1",
					Dimension: ptr(int32(3)),
					ReadCapacity: &ReadCapacityParams{
						Dedicated: &ReadCapacityDedicatedConfig{
							NodeType: ptr("b1"),
							Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(1)), Shards: ptr(int32(1))}},
						},
					},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "dedicated-index",
					"schema": {"fields": {"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"}}},
					"deployment": ` + managedDeploymentJSON + `,
					"read_capacity": {"mode": "Dedicated", "dedicated": {"node_type": "b1", "shards": 1, "scaling": "Manual", "manual": {"replicas": 1}}},
					"tags": null
				}`,
			}},
		},
		{
			name:     "ConfigureIndex Manual replicas and shards",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(3)), Shards: ptr(int32(2))}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/dedicated-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/dedicated-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"shards": 2, "scaling": "Manual", "manual": {"replicas": 3}}}, "tags": null}`,
				},
			},
		},
		{
			// A Manual block with only Shards changes the shard count and keeps the current replicas.
			name:     "ConfigureIndex Manual shards only",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Shards: ptr(int32(2))}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/dedicated-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/dedicated-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"shards": 2}}, "tags": null}`,
				},
			},
		},
		{
			// An empty Manual block can't be sent: 2026-10 requires manual.replicas, and 0 would pause
			// the index.
			name:     "ConfigureIndex empty Manual returns an error",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{}},
					}},
				})
				require.ErrorContains(t, err, "Scaling.Manual must set Replicas")
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/dedicated-index"}},
		},
		{
			// Creating needs replicas; the error lists what a create requires instead of pointing at
			// keeping the current scaling.
			name:     "CreateServerlessIndex empty Manual returns the incomplete-configuration error",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "dedicated-index"}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
					Name:      "dedicated-index",
					Cloud:     CloudAWS,
					Region:    "us-east-1",
					Dimension: ptr(int32(3)),
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						NodeType: ptr("b1"),
						Shards:   ptr(int32(1)),
						Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{}},
					}},
				})
				require.ErrorIs(t, err, errDedicatedReadCapacityIncomplete)
			},
			want: []wantRequest{},
		},
		{
			// Shards wins over the deprecated Manual.Shards from a scaling block copied from describe.
			name:     "ConfigureIndex with conflicting shard counts sends Dedicated.Shards",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Shards:  ptr(int32(3)),
						Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(2)), Shards: ptr(int32(1))}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/dedicated-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/dedicated-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"shards": 3, "scaling": "Manual", "manual": {"replicas": 2}}}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex changes only the node type of an autoscaled index",
			status:   http.StatusOK,
			response: indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "autoscaling-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{NodeType: ptr("t1")}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/autoscaling-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/autoscaling-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"node_type": "t1"}}, "tags": null}`,
				},
			},
		},
		{
			name:     "DescribeIndex Dedicated Manual read capacity",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.DescribeIndex(context.Background(), "dedicated-index")
				require.NoError(t, err)
				require.NotNil(t, idx.ReadCapacity)
				dedicated := idx.ReadCapacity.Dedicated
				require.NotNil(t, dedicated)
				assert.Equal(t, "b1", *dedicated.NodeType)
				require.NotNil(t, dedicated.Scaling)
				require.NotNil(t, dedicated.Scaling.Manual)
				assert.Equal(t, int32(2), *dedicated.Scaling.Manual.Replicas)
				assert.Equal(t, int32(1), *dedicated.Scaling.Manual.Shards)
				assert.Equal(t, "Ready", dedicated.Status.State)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/dedicated-index"}},
		},
		{
			name:     "CreateIndex with replica autoscaling",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateIndex(context.Background(), &CreateIndexRequest{
					Name: "autoscaling-index",
					Schema: IndexSchema{Fields: map[string]IndexSchemaField{
						"your_vector": {DenseVector: &DenseVectorField{Dimension: 1536, Metric: IndexMetricCosine}},
					}},
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						NodeType: ptr("b1"),
						Shards:   ptr(int32(2)),
						Scaling:  &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 2, MaxReplicas: 8}},
					}},
				})
				require.NoError(t, err)
				require.NotNil(t, idx.ReadCapacity)
				dedicated := idx.ReadCapacity.Dedicated
				require.NotNil(t, dedicated)
				assert.Equal(t, ReadCapacityScalingStrategyAuto, *dedicated.ScalingStrategy)
				assert.Equal(t, int32(2), *dedicated.Shards)
				assert.Nil(t, dedicated.Scaling.Manual)
				assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 2, MaxReplicas: 8}, dedicated.Scaling.Auto)
				assert.Equal(t, int32(5), *dedicated.Status.CurrentReplicas)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "autoscaling-index",
					"schema": {"fields": {"your_vector": {"type": "dense_vector", "dimension": 1536, "metric": "cosine"}}},
					"read_capacity": {"mode": "Dedicated", "dedicated": {
						"node_type": "b1",
						"shards": 2,
						"scaling": "Auto",
						"auto": {"min_replicas": 2, "max_replicas": 8}
					}},
					"tags": null
				}`,
			}},
		},
		{
			name:     "CreateIndexForModel with replica autoscaling",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "model-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.CreateIndexForModel(context.Background(), &CreateIndexForModelRequest{
					Name:   "model-index",
					Cloud:  CloudAWS,
					Region: "us-east-1",
					Embed: CreateIndexForModelEmbed{
						Model:    "llama-text-embed-v2",
						FieldMap: map[string]interface{}{"text": "chunk_text"},
					},
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						NodeType: ptr("b1"),
						Shards:   ptr(int32(2)),
						Scaling:  &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 2, MaxReplicas: 8}},
					}},
				})
				require.NoError(t, err)
				require.NotNil(t, idx.ReadCapacity)
				require.NotNil(t, idx.ReadCapacity.Dedicated)
				assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 2, MaxReplicas: 8}, idx.ReadCapacity.Dedicated.Scaling.Auto)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes/create-for-model",
				body: `{
					"name": "model-index",
					"cloud": "aws",
					"region": "us-east-1",
					"deletion_protection": "disabled",
					"embed": {"model": "llama-text-embed-v2", "field_map": {"text": "chunk_text"}},
					"read_capacity": {"mode": "Dedicated", "dedicated": {
						"node_type": "b1",
						"shards": 2,
						"scaling": "Auto",
						"auto": {"min_replicas": 2, "max_replicas": 8}
					}},
					"tags": null
				}`,
			}},
		},
		{
			name:     "ListIndexes reports each index's read capacity and live status",
			status:   http.StatusOK,
			response: `{"indexes": [` + indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON() + `], "incompatible_indexes": []}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIndexes(context.Background())
				require.NoError(t, err)
				require.Len(t, indexes, 1)
				dedicated := indexes[0].ReadCapacity.Dedicated
				require.NotNil(t, dedicated)
				assert.Equal(t, ReadCapacityScalingStrategyAuto, *dedicated.ScalingStrategy)
				assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 2, MaxReplicas: 8}, dedicated.Scaling.Auto)
				assert.Equal(t, ReadCapacityStatus{State: "Ready", CurrentReplicas: ptr(int32(5)), CurrentShards: ptr(int32(2))}, dedicated.Status)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes"}},
		},
		{
			name:   "DescribeIndex reports a read capacity Error state with its message",
			status: http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: `{"mode": "Dedicated",
				"dedicated": {"node_type": "b1", "shards": 1, "scaling": "Manual", "manual": {"replicas": 2}},
				"status": {"state": "Error", "current_replicas": null, "current_shards": null, "error_message": "Read node quota exceeded"}}`}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.DescribeIndex(context.Background(), "dedicated-index")
				require.NoError(t, err)
				assert.Equal(t, ReadCapacityStatus{State: "Error", ErrorMessage: ptr("Read node quota exceeded")}, idx.ReadCapacity.Dedicated.Status)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/dedicated-index"}},
		},
		{
			name:     "DescribeIndex reports an index migrating from OnDemand to Dedicated",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: `{"mode": "Dedicated", "dedicated": {"node_type": "t1", "shards": 1, "scaling": "Manual", "manual": {"replicas": 1}}, "status": {"state": "Migrating", "current_replicas": null, "current_shards": null}}`}.JSON(),
			call: func(t *testing.T, pc *Client) {
				idx, err := pc.DescribeIndex(context.Background(), "dedicated-index")
				require.NoError(t, err)
				dedicated := idx.ReadCapacity.Dedicated
				assert.Equal(t, "Migrating", dedicated.Status.State)
				assert.Nil(t, dedicated.Status.CurrentReplicas)
				assert.Equal(t, ptr("t1"), dedicated.NodeType)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/dedicated-index"}},
		},
		{
			// Replicas 0 pauses a manually scaled index; the zero must reach the wire.
			name:     "ConfigureIndex pauses a manually scaled index with zero replicas",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(0))}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/dedicated-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/dedicated-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"scaling": "Manual", "manual": {"replicas": 0}}}, "tags": null}`,
				},
			},
		},
		{
			// The SDK sends OnDemand as given; the API refuses it for an index with Dedicated read
			// capacity (400 INVALID_ARGUMENT), which the SDK doesn't pre-judge.
			name:     "ConfigureIndex sends OnDemand for an autoscaled index as given",
			status:   http.StatusOK,
			response: indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "autoscaling-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{OnDemand: &ReadCapacityOnDemandConfig{}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/autoscaling-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/autoscaling-index",
					body:   `{"read_capacity": {"mode": "OnDemand"}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex turns on replica autoscaling",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 2, MaxReplicas: 8}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/dedicated-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/dedicated-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"scaling": "Auto", "auto": {"min_replicas": 2, "max_replicas": 8}}}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex changes the bounds of an autoscaled index",
			status:   http.StatusOK,
			response: indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "autoscaling-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 4}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/autoscaling-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/autoscaling-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"scaling": "Auto", "auto": {"min_replicas": 1, "max_replicas": 4}}}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex changes only shards of an autoscaled index",
			status:   http.StatusOK,
			response: indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "autoscaling-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{Shards: ptr(int32(3))}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/autoscaling-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/autoscaling-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"shards": 3}}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex switches an autoscaled index back to a fixed replica count",
			status:   http.StatusOK,
			response: indexModelFixture{name: "autoscaling-index", readCapacity: autoscaledReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "autoscaling-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(4))}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/autoscaling-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/autoscaling-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"scaling": "Manual", "manual": {"replicas": 4}}}, "tags": null}`,
				},
			},
		},
		{
			// Manual and Auto can't become one scaling value, so nothing is sent after the describe.
			name:     "ConfigureIndex with both Manual and Auto returns an error",
			status:   http.StatusOK,
			response: indexModelFixture{name: "dedicated-index", readCapacity: dedicatedManualReadCapacityJSON}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "dedicated-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{
							Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(2))},
							Auto:   &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2},
						},
					}},
				})
				require.ErrorContains(t, err, "both Scaling.Manual and Scaling.Auto are set")
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/dedicated-index"}},
		},
		{
			// Shards wins over a different deprecated Manual.Shards.
			name:     "CreateServerlessIndex with conflicting shard counts sends Dedicated.Shards",
			status:   http.StatusCreated,
			response: indexModelFixture{name: "dedicated-index"}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
					Name:      "dedicated-index",
					Cloud:     CloudAWS,
					Region:    "us-east-1",
					Dimension: ptr(int32(3)),
					ReadCapacity: &ReadCapacityParams{
						Dedicated: &ReadCapacityDedicatedConfig{
							NodeType: ptr("b1"),
							Shards:   ptr(int32(2)),
							Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(1)), Shards: ptr(int32(1))}},
						},
					},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/indexes",
				body: `{
					"name": "dedicated-index",
					"schema": {"fields": {"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"}}},
					"deployment": ` + managedDeploymentJSON + `,
					"read_capacity": {"mode": "Dedicated", "dedicated": {"node_type": "b1", "shards": 2, "scaling": "Manual", "manual": {"replicas": 1}}},
					"tags": null
				}`,
			}},
		},
		{
			// Describe may omit read_capacity, so the SDK can't tell whether the index is OnDemand; the
			// API judges a partial Dedicated change instead of the SDK refusing it.
			name:     "ConfigureIndex shards only when describe omits read capacity",
			status:   http.StatusOK,
			response: indexModelFixture{name: "unclassified-index"}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "unclassified-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{Shards: ptr(int32(2))}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/unclassified-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/unclassified-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"shards": 2}}, "tags": null}`,
				},
			},
		},
		{
			name:     "ConfigureIndex Auto bounds when describe reports an unlisted mode",
			status:   http.StatusOK,
			response: indexModelFixture{name: "unclassified-index", readCapacity: `{"mode": "Burst", "status": {"state": "Ready"}}`}.JSON(),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "unclassified-index", ConfigureIndexParams{
					ReadCapacity: &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
						Scaling: &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 4}},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes/unclassified-index"},
				{
					method: http.MethodPatch,
					path:   "/indexes/unclassified-index",
					body:   `{"read_capacity": {"mode": "Dedicated", "dedicated": {"scaling": "Auto", "auto": {"min_replicas": 1, "max_replicas": 4}}}, "tags": null}`,
				},
			},
		},
		{
			// Indexes 2026-10 can't describe come back in their own array; ListIndexes keeps returning
			// only full indexes, so existing code never sees a partial Index.
			name:   "ListIndexes and ListIncompatibleIndexes with a mixed list",
			status: http.StatusOK,
			response: `{"indexes": [` + indexModelFixture{name: "docs"}.JSON() + `],
				"incompatible_indexes": [{"name": "docs-v2", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "Index 'docs-v2' uses features that API version 2026-10 can't represent."}]}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIndexes(context.Background())
				require.NoError(t, err)
				require.Len(t, indexes, 1)
				assert.Equal(t, "docs", indexes[0].Name)
				require.NotNil(t, indexes[0].Status)

				incompatible, err := pc.ListIncompatibleIndexes(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleIndex{{
					Name:              "docs-v2",
					MinimumAPIVersion: ptr("2027-04"),
					Message:           "Index 'docs-v2' uses features that API version 2026-10 can't represent.",
				}}, incompatible)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes"},
				{method: http.MethodGet, path: "/indexes"},
			},
		},
		{
			// Each version key is null when there's no such version, and both can be set. A key that
			// a server leaves out reads as nil too.
			name:   "ListIncompatibleIndexes when every index is incompatible",
			status: http.StatusOK,
			response: `{"indexes": [], "incompatible_indexes": [
				{"name": "ga", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m1"},
				{"name": "preview", "minimum_api_version": null, "preview_api_version": "2027-07.alpha", "message": "m2"},
				{"name": "both", "minimum_api_version": "2027-04", "preview_api_version": "2027-07.alpha", "message": "m3"},
				{"name": "neither", "minimum_api_version": null, "preview_api_version": null, "message": "m4"},
				{"name": "absent", "message": "m5"}]}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIndexes(context.Background())
				require.NoError(t, err)
				assert.Empty(t, indexes)

				incompatible, err := pc.ListIncompatibleIndexes(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleIndex{
					{Name: "ga", MinimumAPIVersion: ptr("2027-04"), Message: "m1"},
					{Name: "preview", PreviewAPIVersion: ptr("2027-07.alpha"), Message: "m2"},
					{Name: "both", MinimumAPIVersion: ptr("2027-04"), PreviewAPIVersion: ptr("2027-07.alpha"), Message: "m3"},
					{Name: "neither", Message: "m4"},
					{Name: "absent", Message: "m5"},
				}, incompatible)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes"},
				{method: http.MethodGet, path: "/indexes"},
			},
		},
		{
			// Servers deployed before the arrays became required omit the key.
			name:     "ListIncompatibleIndexes when the key is absent",
			status:   http.StatusOK,
			response: `{"indexes": [` + indexModelFixture{name: "docs"}.JSON() + `]}`,
			call: func(t *testing.T, pc *Client) {
				incompatible, err := pc.ListIncompatibleIndexes(context.Background())
				require.NoError(t, err)
				assert.NotNil(t, incompatible)
				assert.Empty(t, incompatible)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes"}},
		},
		{
			// A malformed incompatible_indexes must not stop ListIndexes from returning the full
			// indexes; only ListIncompatibleIndexes reports it.
			name:     "ListIndexes ignores a malformed incompatible_indexes",
			status:   http.StatusOK,
			response: `{"indexes": [` + indexModelFixture{name: "docs"}.JSON() + `], "incompatible_indexes": {"name": 7}}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIndexes(context.Background())
				require.NoError(t, err)
				require.Len(t, indexes, 1)
				assert.Equal(t, "docs", indexes[0].Name)

				_, err = pc.ListIncompatibleIndexes(context.Background())
				require.Error(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes"},
				{method: http.MethodGet, path: "/indexes"},
			},
		},
		{
			// Entries are decoded one at a time: a wrong-typed entry, null, or one without a name is
			// left out, and the rest are still returned.
			name:   "ListIncompatibleIndexes skips malformed and unnamed entries",
			status: http.StatusOK,
			response: `{"indexes": [],
				"incompatible_indexes": [{"name": "bad", "minimum_api_version": 2027}, "docs-v3", null, {}, {"minimum_api_version": "2027-04", "preview_api_version": null}, {"name": "docs-v2", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}]}`,
			call: func(t *testing.T, pc *Client) {
				incompatible, err := pc.ListIncompatibleIndexes(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleIndex{{Name: "docs-v2", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, incompatible)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes"}},
		},
		{
			name:     "DescribeIndex of an incompatible index returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatibleIndex412JSON,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.DescribeIndex(context.Background(), "docs-v2")
				requireIncompatibleIndexError(t, err)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/docs-v2"}},
		},
		{
			// Only an early-access API version can act on the index, so details has no minimum.
			name:   "DescribeIndex of an index only an early-access API version can describe",
			status: http.StatusPreconditionFailed,
			response: incompatible412JSON(incompatiblePreviewMessage,
				map[string]any{"api_version": "2026-10", "preview_api_version": "2027-07.alpha"}),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.DescribeIndex(context.Background(), "docs-preview")
				requireIncompatibleError(t, err, incompatiblePreviewMessage,
					map[string]any{"api_version": "2026-10", "preview_api_version": "2027-07.alpha"})
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/docs-preview"}},
		},
		{
			// No public API version can act on the index, so details has only the request's version.
			name:     "DescribeIndex of an index no public API version can describe",
			status:   http.StatusPreconditionFailed,
			response: incompatible412JSON(incompatibleNoPublicMessage, map[string]any{"api_version": "2026-10"}),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.DescribeIndex(context.Background(), "docs-next")
				requireIncompatibleError(t, err, incompatibleNoPublicMessage, map[string]any{"api_version": "2026-10"})
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/docs-next"}},
		},
		{
			name:     "CreateCollection from an incompatible index returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatibleIndex412JSON,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateCollection(context.Background(), &CreateCollectionRequest{Name: "archive", Source: "docs-v2"})
				requireIncompatibleIndexError(t, err)
			},
			want: []wantRequest{{method: http.MethodPost, path: "/collections", body: `{"name":"archive","source":"docs-v2"}`}},
		},
		{
			name:     "CreateBackup of an incompatible index returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatibleIndex412JSON,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateBackup(context.Background(), &CreateBackupParams{IndexName: "docs-v2"})
				requireIncompatibleIndexError(t, err)
			},
			want: []wantRequest{{method: http.MethodPost, path: "/indexes/docs-v2/backups", body: `{}`}},
		},
		{
			// ConfigureIndex describes the index first, so the refusal arrives before any PATCH. The
			// error still reads as a configure failure.
			name:     "ConfigureIndex of an incompatible index returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatibleIndex412JSON,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "docs-v2", ConfigureIndexParams{Tags: IndexTags{"team": "search"}})
				requireIncompatibleIndexError(t, err)
				var pcErr *PineconeError
				require.ErrorAs(t, err, &pcErr)
				assert.Equal(t, "failed to configure index: "+incompatibleIndexMessage, pcErr.Message)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/docs-v2"}},
		},
		{
			name:     "ConfigureIndex of a missing index reports a configure failure",
			status:   http.StatusNotFound,
			response: `{"status": 404, "error": {"code": "NOT_FOUND", "message": "Resource missing-index not found"}}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ConfigureIndex(context.Background(), "missing-index", ConfigureIndexParams{DeletionProtection: DeletionProtectionDisabled})
				var pcErr *PineconeError
				require.ErrorAs(t, err, &pcErr)
				assert.Equal(t, http.StatusNotFound, pcErr.Code)
				assert.Equal(t, "NOT_FOUND", pcErr.ErrorCode)
				assert.Equal(t, "failed to configure index: Resource missing-index not found", pcErr.Message)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/missing-index"}},
		},
		{
			name:   "ListCollections and ListIncompatibleCollections with a mixed list",
			status: http.StatusOK,
			response: `{"collections": [{"name": "small-collection", "status": "Ready", "environment": "us-east1-gcp"}],
				"incompatible_collections": [{"name": "archive-2025", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "Collection 'archive-2025' uses features that API version 2026-10 can't represent."},
					{"name": "archive-preview", "minimum_api_version": null, "preview_api_version": "2027-07.alpha", "message": "m2"}]}`,
			call: func(t *testing.T, pc *Client) {
				collections, err := pc.ListCollections(context.Background())
				require.NoError(t, err)
				require.Len(t, collections, 1)
				assert.Equal(t, "small-collection", collections[0].Name)

				incompatible, err := pc.ListIncompatibleCollections(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleCollection{{
					Name:              "archive-2025",
					MinimumAPIVersion: ptr("2027-04"),
					Message:           "Collection 'archive-2025' uses features that API version 2026-10 can't represent.",
				}, {
					Name:              "archive-preview",
					PreviewAPIVersion: ptr("2027-07.alpha"),
					Message:           "m2",
				}}, incompatible)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/collections"},
				{method: http.MethodGet, path: "/collections"},
			},
		},
		{
			name:     "ListIncompatibleCollections when the key is absent",
			status:   http.StatusOK,
			response: `{"collections": []}`,
			call: func(t *testing.T, pc *Client) {
				incompatible, err := pc.ListIncompatibleCollections(context.Background())
				require.NoError(t, err)
				assert.NotNil(t, incompatible)
				assert.Empty(t, incompatible)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/collections"}},
		},
		{
			name:     "ListCollections ignores a malformed incompatible_collections",
			status:   http.StatusOK,
			response: `{"collections": [{"name": "small-collection", "status": "Ready", "environment": "us-east1-gcp"}], "incompatible_collections": {"name": 7}}`,
			call: func(t *testing.T, pc *Client) {
				collections, err := pc.ListCollections(context.Background())
				require.NoError(t, err)
				require.Len(t, collections, 1)

				_, err = pc.ListIncompatibleCollections(context.Background())
				require.Error(t, err)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/collections"},
				{method: http.MethodGet, path: "/collections"},
			},
		},
		{
			name:   "ListIncompatibleCollections skips malformed and unnamed entries",
			status: http.StatusOK,
			response: `{"collections": [],
				"incompatible_collections": [{"name": "bad", "minimum_api_version": 2027}, "archive-2024", null, {}, {"name": "archive-2025", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}]}`,
			call: func(t *testing.T, pc *Client) {
				incompatible, err := pc.ListIncompatibleCollections(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleCollection{{Name: "archive-2025", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, incompatible)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/collections"}},
		},
		{
			name:     "DescribeCollection of an incompatible collection returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatible412JSON(incompatibleCollectionMessage, incompatibleMinimumDetails),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.DescribeCollection(context.Background(), "archive-2025")
				requireIncompatibleError(t, err, incompatibleCollectionMessage, incompatibleMinimumDetails)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/collections/archive-2025"}},
		},
		{
			// Incompatible entries count toward the page size, so Pagination can point to more
			// pages while Data holds fewer than Limit backups.
			name:   "ListBackups for the project with incompatible backups",
			status: http.StatusOK,
			response: `{"data": [{"backup_id": "bkp-1", "source_index_name": "docs", "source_index_id": "idx-1", "status": "Ready", "cloud": "aws", "region": "us-east-1"}],
				"pagination": {"next": "dG9rZW4="},
				"incompatible_data": [
					{"backup_id": "bkp-2", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m1"},
					{"backup_id": "bkp-3", "minimum_api_version": null, "preview_api_version": "2027-07.alpha", "message": "m2"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), &ListBackupsParams{Limit: ptr(3)})
				require.NoError(t, err)
				require.Len(t, backups.Data, 1)
				assert.Equal(t, "bkp-1", backups.Data[0].BackupId)
				require.NotNil(t, backups.Pagination)
				assert.Equal(t, "dG9rZW4=", backups.Pagination.Next)
				assert.Equal(t, []*IncompatibleBackup{
					{BackupId: "bkp-2", MinimumAPIVersion: ptr("2027-04"), Message: "m1"},
					{BackupId: "bkp-3", PreviewAPIVersion: ptr("2027-07.alpha"), Message: "m2"},
				}, backups.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups", query: "limit=3"}},
		},
		{
			name:   "ListBackups for an index with an incompatible backup",
			status: http.StatusOK,
			response: `{"data": [],
				"incompatible_data": [{"backup_id": "bkp-2", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m1"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), &ListBackupsParams{IndexName: ptr("docs")})
				require.NoError(t, err)
				assert.Empty(t, backups.Data)
				assert.Equal(t, []*IncompatibleBackup{{BackupId: "bkp-2", MinimumAPIVersion: ptr("2027-04"), Message: "m1"}}, backups.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/indexes/docs/backups"}},
		},
		{
			// Production sends the pre-rename key until the server-side rename deploys.
			name:   "ListBackups and ListRestoreJobs fall back to the pre-rename keys",
			status: http.StatusOK,
			response: `{"data": [], "pagination": null,
				"incompatible_backups": [{"backup_id": "b", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}],
				"incompatible_restore_jobs": [{"restore_job_id": "r", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleBackup{{BackupId: "b", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, backups.IncompatibleData)

				jobs, err := pc.ListRestoreJobs(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleRestoreJob{{RestoreJobId: "r", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, jobs.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups"}, {method: http.MethodGet, path: "/restore-jobs"}},
		},
		{
			name:   "ListBackups prefers incompatible_data over the pre-rename key",
			status: http.StatusOK,
			response: `{"data": [], "pagination": null,
				"incompatible_data": [{"backup_id": "new", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}],
				"incompatible_backups": [{"backup_id": "old", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleBackup{{BackupId: "new", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, backups.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups"}},
		},
		{
			name:     "ListBackups without incompatible_data",
			status:   http.StatusOK,
			response: `{"data": [{"backup_id": "bkp-1", "source_index_name": "docs", "source_index_id": "idx-1", "status": "Ready", "cloud": "aws", "region": "us-east-1"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				require.Len(t, backups.Data, 1)
				assert.NotNil(t, backups.IncompatibleData)
				assert.Empty(t, backups.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups"}},
		},
		{
			// A malformed incompatible_data entry must not stop ListBackups from returning the full
			// backups. An entry that doesn't decode is left out; the others are kept.
			name:   "ListBackups tolerates malformed incompatible_data entries",
			status: http.StatusOK,
			response: `{"data": [{"backup_id": "bkp-1", "source_index_name": "docs", "source_index_id": "idx-1", "status": "Ready", "cloud": "aws", "region": "us-east-1"}],
				"incompatible_data": [{"backup_id": "bkp-2", "minimum_api_version": 2027, "message": "m1"}, "bkp-3", null, {}, {"backup_id": "bkp-4", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m2"}]}`,
			call: func(t *testing.T, pc *Client) {
				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				require.Len(t, backups.Data, 1)
				assert.Equal(t, []*IncompatibleBackup{{BackupId: "bkp-4", MinimumAPIVersion: ptr("2027-04"), Message: "m2"}}, backups.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups"}},
		},
		{
			// Like ListIncompatibleIndexes, a list that isn't an array is reported, not read as empty.
			name:     "ListBackups reports an incompatible_data that isn't an array",
			status:   http.StatusOK,
			response: `{"data": [{"backup_id": "bkp-1", "source_index_name": "docs", "source_index_id": "idx-1", "status": "Ready", "cloud": "aws", "region": "us-east-1"}], "incompatible_data": {"backup_id": "bkp-2"}}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ListBackups(context.Background(), nil)
				require.ErrorContains(t, err, "failed to decode incompatible_data")
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups"}},
		},
		{
			name:     "DescribeBackup of an incompatible backup returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatible412JSON(incompatibleBackupMessage, incompatibleMinimumDetails),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.DescribeBackup(context.Background(), "bkp-2")
				requireIncompatibleError(t, err, incompatibleBackupMessage, incompatibleMinimumDetails)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/backups/bkp-2"}},
		},
		{
			name:     "CreateIndexFromBackup of an incompatible backup returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatible412JSON(incompatibleBackupMessage, incompatibleMinimumDetails),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{BackupId: "bkp-2", Name: "restored"})
				requireIncompatibleError(t, err, incompatibleBackupMessage, incompatibleMinimumDetails)
			},
			want: []wantRequest{{method: http.MethodPost, path: "/backups/bkp-2/create-index", body: `{"name": "restored", "tags": null}`}},
		},
		{
			name:   "ListRestoreJobs with incompatible restore jobs",
			status: http.StatusOK,
			response: `{"data": [{"restore_job_id": "rj-1", "backup_id": "bkp-1", "target_index_name": "docs", "target_index_id": "idx-1", "status": "Completed", "created_at": "2026-03-15T10:30:00Z", "percent_complete": 100}],
				"pagination": {"next": "dG9rZW4="},
				"incompatible_data": [{"restore_job_id": "rj-2", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m1"},
					{"restore_job_id": "rj-3", "minimum_api_version": null, "preview_api_version": "2027-07.alpha", "message": "m2"}]}`,
			call: func(t *testing.T, pc *Client) {
				jobs, err := pc.ListRestoreJobs(context.Background(), &ListRestoreJobsParams{Limit: ptr(2)})
				require.NoError(t, err)
				require.Len(t, jobs.Data, 1)
				assert.Equal(t, "rj-1", jobs.Data[0].RestoreJobId)
				require.NotNil(t, jobs.Pagination)
				assert.Equal(t, "dG9rZW4=", jobs.Pagination.Next)
				assert.Equal(t, []*IncompatibleRestoreJob{
					{RestoreJobId: "rj-2", MinimumAPIVersion: ptr("2027-04"), Message: "m1"},
					{RestoreJobId: "rj-3", PreviewAPIVersion: ptr("2027-07.alpha"), Message: "m2"},
				}, jobs.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/restore-jobs", query: "limit=2"}},
		},
		{
			name:   "ListRestoreJobs tolerates a malformed incompatible_data",
			status: http.StatusOK,
			response: `{"data": [{"restore_job_id": "rj-1", "backup_id": "bkp-1", "target_index_name": "docs", "target_index_id": "idx-1", "status": "Completed", "created_at": "2026-03-15T10:30:00Z", "percent_complete": 100}],
				"incompatible_data": [{"restore_job_id": 7}, null, {"minimum_api_version": "2027-04", "preview_api_version": null}, {"restore_job_id": "rj-3", "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m2"}]}`,
			call: func(t *testing.T, pc *Client) {
				jobs, err := pc.ListRestoreJobs(context.Background(), nil)
				require.NoError(t, err)
				require.Len(t, jobs.Data, 1)
				assert.Equal(t, []*IncompatibleRestoreJob{{RestoreJobId: "rj-3", MinimumAPIVersion: ptr("2027-04"), Message: "m2"}}, jobs.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/restore-jobs"}},
		},
		{
			name:     "ListRestoreJobs reports a non-array incompatible_data",
			status:   http.StatusOK,
			response: `{"data": [{"restore_job_id": "rj-1", "backup_id": "bkp-1", "target_index_name": "docs", "target_index_id": "idx-1", "status": "Completed", "created_at": "2026-03-15T10:30:00Z", "percent_complete": 100}], "incompatible_data": {"restore_job_id": "rj-2"}}`,
			call: func(t *testing.T, pc *Client) {
				_, err := pc.ListRestoreJobs(context.Background(), nil)
				require.ErrorContains(t, err, "failed to decode incompatible_data")
			},
			want: []wantRequest{{method: http.MethodGet, path: "/restore-jobs"}},
		},
		{
			name:     "ListRestoreJobs without incompatible_data",
			status:   http.StatusOK,
			response: `{"data": []}`,
			call: func(t *testing.T, pc *Client) {
				jobs, err := pc.ListRestoreJobs(context.Background(), nil)
				require.NoError(t, err)
				assert.NotNil(t, jobs.IncompatibleData)
				assert.Empty(t, jobs.IncompatibleData)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/restore-jobs"}},
		},
		{
			name:     "DescribeRestoreJob of an incompatible restore job returns the 412 as a PineconeError",
			status:   http.StatusPreconditionFailed,
			response: incompatible412JSON(incompatibleRestoreJobMessage, incompatibleMinimumDetails),
			call: func(t *testing.T, pc *Client) {
				_, err := pc.DescribeRestoreJob(context.Background(), "rj-2")
				requireIncompatibleError(t, err, incompatibleRestoreJobMessage, incompatibleMinimumDetails)
			},
			want: []wantRequest{{method: http.MethodGet, path: "/restore-jobs/rj-2"}},
		},
		{
			// 2026-10 always sends the incompatible_* arrays, empty when there are none (apis 95b17e0e).
			name:   "Empty incompatible arrays",
			status: http.StatusOK,
			response: `{"indexes": [], "collections": [], "data": [],
				"incompatible_indexes": [], "incompatible_collections": [], "incompatible_data": []}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIncompatibleIndexes(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleIndex{}, indexes)

				collections, err := pc.ListIncompatibleCollections(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleCollection{}, collections)

				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleBackup{}, backups.IncompatibleData)

				jobs, err := pc.ListRestoreJobs(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleRestoreJob{}, jobs.IncompatibleData)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes"},
				{method: http.MethodGet, path: "/collections"},
				{method: http.MethodGet, path: "/backups"},
				{method: http.MethodGet, path: "/restore-jobs"},
			},
		},
		{
			// Entry models may gain fields in later releases (apis a101b112); unknown ones are ignored.
			// Backups and restore jobs share the incompatible_data key, so one array serves both lists:
			// each list drops entries without its own identifier.
			name:   "Incompatible entries with fields the SDK doesn't know",
			status: http.StatusOK,
			response: `{"indexes": [], "collections": [], "data": [],
				"incompatible_indexes": [{"name": "i", "minimum_api_version": "2027-04", "preview_api_version": null, "docs_url": "https://docs.pinecone.io", "message": "m"}],
				"incompatible_collections": [{"name": "c", "minimum_api_version": "2027-04", "preview_api_version": null, "docs_url": "https://docs.pinecone.io", "message": "m"}],
				"incompatible_data": [{"backup_id": "b", "minimum_api_version": "2027-04", "preview_api_version": null, "docs_url": "https://docs.pinecone.io", "message": "m"},
					{"restore_job_id": "r", "minimum_api_version": "2027-04", "preview_api_version": null, "docs_url": "https://docs.pinecone.io", "message": "m"}]}`,
			call: func(t *testing.T, pc *Client) {
				indexes, err := pc.ListIncompatibleIndexes(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleIndex{{Name: "i", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, indexes)

				collections, err := pc.ListIncompatibleCollections(context.Background())
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleCollection{{Name: "c", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, collections)

				backups, err := pc.ListBackups(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleBackup{{BackupId: "b", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, backups.IncompatibleData)

				jobs, err := pc.ListRestoreJobs(context.Background(), nil)
				require.NoError(t, err)
				assert.Equal(t, []*IncompatibleRestoreJob{{RestoreJobId: "r", MinimumAPIVersion: ptr("2027-04"), Message: "m"}}, jobs.IncompatibleData)
			},
			want: []wantRequest{
				{method: http.MethodGet, path: "/indexes"},
				{method: http.MethodGet, path: "/collections"},
				{method: http.MethodGet, path: "/backups"},
				{method: http.MethodGet, path: "/restore-jobs"},
			},
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
			response:  `{"matches": [{"_id": "doc-1", "_score": 0.9, "genre": "drama"}], "namespace": "movies", "usage": {"read_units": 5, "egress_bytes": 2048}}`,
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
				require.NotNil(t, res.Usage.EgressBytes)
				assert.Equal(t, int64(2048), *res.Usage.EgressBytes)
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
				assert.Nil(t, res.Usage.EgressBytes, "egress_bytes is absent from the response")
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/articles/documents/search",
				body:   `{"top_k": 5, "score_by": [{"type": "embed", "fields": ["body"], "query": "trail running shoes"}]}`,
			}},
		},
		{
			name:      "SearchDocuments embed with parameters",
			namespace: "claims-en",
			status:    http.StatusOK,
			response:  `{"matches": [], "namespace": "claims-en", "usage": {"read_units": 1}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				_, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
					TopK: 5,
					ScoreBy: []DocumentScoringMethod{{
						Type:       DocumentScoringTypeEmbed,
						Fields:     []string{"body"},
						Query:      ptr("Is the Eiffel Tower taller than 300 m?"),
						Parameters: map[string]interface{}{"task_type": "fact_verification"},
					}},
				})
				require.NoError(t, err)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/claims-en/documents/search",
				body: `{"top_k": 5, "score_by": [{
					"type": "embed",
					"fields": ["body"],
					"query": "Is the Eiffel Tower taller than 300 m?",
					"parameters": {"task_type": "fact_verification"}
				}]}`,
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
			response:  `{"documents": {"doc-1": {"_id": "doc-1", "genre": "drama"}}, "namespace": "movies", "usage": {"read_units": 1, "egress_bytes": 187}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.FetchDocuments(context.Background(), &FetchDocumentsRequest{Ids: []string{"doc-1"}})
				require.NoError(t, err)
				require.Contains(t, res.Documents, "doc-1")
				assert.Equal(t, "drama", res.Documents["doc-1"]["genre"])
				assert.Equal(t, int32(1), res.Usage.ReadUnits)
				require.NotNil(t, res.Usage.EgressBytes)
				assert.Equal(t, int64(187), *res.Usage.EgressBytes)
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
			response:  `{"documents": [{"_id": "doc-1"}], "namespace": "movies", "pagination": {"next": "token-1"}, "usage": {"read_units": 1, "egress_bytes": 96}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.ListDocuments(context.Background(), &ListDocumentsRequest{Prefix: ptr("doc-")})
				require.NoError(t, err)
				require.Len(t, res.Documents, 1)
				assert.Equal(t, "doc-1", res.Documents[0].Id)
				require.NotNil(t, res.Pagination)
				assert.Equal(t, "token-1", res.Pagination.Next)
				require.NotNil(t, res.Usage.EgressBytes)
				assert.Equal(t, int64(96), *res.Usage.EgressBytes)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/namespaces/movies/documents/list",
				body:   `{"prefix": "doc-"}`,
			}},
		},
		{
			name:      "SearchRecords",
			namespace: "articles",
			status:    http.StatusOK,
			response:  `{"namespace": "articles", "result": {"hits": [{"_id": "rec-1", "_score": 0.7, "fields": {"chunk_text": "apples"}}]}, "usage": {"read_units": 2, "embed_total_tokens": 3, "egress_bytes": 512}}`,
			call: func(t *testing.T, idx *IndexConnection) {
				res, err := idx.SearchRecords(context.Background(), &SearchRecordsRequest{
					Query: SearchRecordsQuery{TopK: 5, Inputs: &map[string]interface{}{"text": "apples"}},
				})
				require.NoError(t, err)
				assert.Equal(t, "articles", res.Namespace)
				require.Len(t, res.Result.Hits, 1)
				assert.Equal(t, "rec-1", res.Result.Hits[0].Id)
				assert.Equal(t, int32(2), res.Usage.ReadUnits)
				require.NotNil(t, res.Usage.EmbedTotalTokens)
				assert.Equal(t, int32(3), *res.Usage.EmbedTotalTokens)
				require.NotNil(t, res.Usage.EgressBytes)
				assert.Equal(t, int64(512), *res.Usage.EgressBytes)
			},
			want: []wantRequest{{
				method: http.MethodPost,
				path:   "/records/namespaces/articles/search",
				body:   `{"query": {"top_k": 5, "inputs": {"text": "apples"}}}`,
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

// The v7.2.0 shape (shards inside Scaling.Manual) and the 2026-10 shape (Dedicated.Shards) must put
// the same bytes on the wire, for create and for configure.
func TestReadCapacityDeprecatedShardsWireIdenticalUnit(t *testing.T) {
	oldShape := &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		NodeType: ptr("b1"),
		Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(1)), Shards: ptr(int32(2))}},
	}}
	newShape := &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		NodeType: ptr("b1"),
		Shards:   ptr(int32(2)),
		Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(1))}},
	}}

	oldCreate, err := readCapacityParamsToReadCapacity(oldShape)
	require.NoError(t, err)
	newCreate, err := readCapacityParamsToReadCapacity(newShape)
	require.NoError(t, err)
	oldBytes, err := json.Marshal(oldCreate)
	require.NoError(t, err)
	newBytes, err := json.Marshal(newCreate)
	require.NoError(t, err)
	assert.Equal(t, string(newBytes), string(oldBytes), "create")

	current := &ReadCapacity{Dedicated: &ReadCapacityDedicated{NodeType: ptr("b1")}}
	oldPatch, err := patchReadCapacity(oldShape, current)
	require.NoError(t, err)
	newPatch, err := patchReadCapacity(newShape, current)
	require.NoError(t, err)
	oldBytes, err = json.Marshal(oldPatch)
	require.NoError(t, err)
	newBytes, err = json.Marshal(newPatch)
	require.NoError(t, err)
	assert.Equal(t, string(newBytes), string(oldBytes), "configure")
}

// Messages and details from the apis 4fcdbf87 412 examples, with this file's resource names.
const (
	incompatibleIndexMessage      = "Index 'docs-v2' uses features that API version 2026-10 can't represent. To view or configure it, upgrade to a Pinecone SDK that supports API version 2027-04 or later. If you call the API directly, set the X-Pinecone-Api-Version header to 2027-04 or later."
	incompatibleCollectionMessage = "Collection 'archive-2025' uses features that API version 2026-10 can't represent. To view it, upgrade to a Pinecone SDK that supports API version 2027-04 or later. If you call the API directly, set the X-Pinecone-Api-Version header to 2027-04 or later."
	incompatibleBackupMessage     = "Backup 'bkp-2' uses features that API version 2026-10 can't represent. To view or restore it, upgrade to a Pinecone SDK that supports API version 2027-04 or later. If you call the API directly, set the X-Pinecone-Api-Version header to 2027-04 or later."
	incompatibleRestoreJobMessage = "Restore job 'rj-2' uses features that API version 2026-10 can't represent. To view it, upgrade to a Pinecone SDK that supports API version 2027-04 or later. If you call the API directly, set the X-Pinecone-Api-Version header to 2027-04 or later."
	incompatiblePreviewMessage    = "Index 'docs-preview' uses features that API version 2026-10 can't represent; it is only available on the early-access API version 2027-07.alpha. To view or configure it, set the X-Pinecone-Api-Version header to 2027-07.alpha."
	incompatibleNoPublicMessage   = "Index 'docs-next' uses features that aren't available in any public API version yet. It can still be deleted."
)

// incompatibleMinimumDetails are the 412 details when a generally available API version can act on
// the resource. A version key with no value is left out, never null.
var incompatibleMinimumDetails = map[string]any{"api_version": "2026-10", "minimum_api_version": "2027-04"}

var incompatibleIndex412JSON = incompatible412JSON(incompatibleIndexMessage, incompatibleMinimumDetails)

func incompatible412JSON(message string, details map[string]any) string {
	body, err := json.Marshal(map[string]any{
		"status": 412,
		"error": map[string]any{
			"code":    "FAILED_PRECONDITION",
			"reason":  "RESOURCE_INCOMPATIBLE_WITH_API_VERSION",
			"message": message,
			"details": details,
		},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func requireIncompatibleError(t *testing.T, err error, message string, details map[string]any) {
	t.Helper()
	var pcErr *PineconeError
	require.ErrorAs(t, err, &pcErr)
	assert.Equal(t, http.StatusPreconditionFailed, pcErr.Code)
	assert.Equal(t, "FAILED_PRECONDITION", pcErr.ErrorCode)
	assert.Equal(t, ReasonResourceIncompatibleWithAPIVersion, pcErr.Reason)
	assert.Contains(t, pcErr.Message, message)
	assert.Equal(t, details, pcErr.Details())
	typed := pcErr.TypedDetails()
	require.NotNil(t, typed)
	assert.Equal(t, details["minimum_api_version"], derefOrNil(typed.MinimumAPIVersion))
	assert.Equal(t, details["preview_api_version"], derefOrNil(typed.PreviewAPIVersion))
}

func derefOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func requireIncompatibleIndexError(t *testing.T, err error) {
	t.Helper()
	requireIncompatibleError(t, err, incompatibleIndexMessage, incompatibleMinimumDetails)
}

func TestLegacyIntegratedIndexDimensionWireUnit(t *testing.T) {
	sparseEmbed := func(dimension string) string {
		return `{"chunk_text": {"type": "semantic_text", "model": "pinecone-sparse-english-v0"` + dimension +
			`, "metric": "dotproduct", "write_parameters": {"input_type": "passage"}}}`
	}
	sparse := &IndexEmbed{
		Model:           "pinecone-sparse-english-v0",
		Metric:          ptr(IndexMetricDotproduct),
		FieldMap:        &map[string]interface{}{"text": "chunk_text"},
		WriteParameters: &map[string]interface{}{"input_type": "passage"},
	}
	dense := &IndexEmbed{
		Model:     "multilingual-e5-large",
		Dimension: ptr(int32(1024)),
		Metric:    ptr(IndexMetricCosine),
		FieldMap:  &map[string]interface{}{"text": "chunk_text"},
	}

	fieldsTests := []struct {
		name       string
		fields     string
		vectorType string
		metric     IndexMetric
		embed      *IndexEmbed
	}{
		{name: "sparse model with a zero dimension", fields: sparseEmbed(`, "dimension": 0`), vectorType: "sparse", metric: IndexMetricDotproduct, embed: sparse},
		{name: "sparse model with a null dimension", fields: sparseEmbed(`, "dimension": null`), vectorType: "sparse", metric: IndexMetricDotproduct, embed: sparse},
		{name: "sparse model without a dimension", fields: sparseEmbed(""), vectorType: "sparse", metric: IndexMetricDotproduct, embed: sparse},
		{
			name:       "dense model",
			fields:     `{"chunk_text": {"type": "semantic_text", "model": "multilingual-e5-large", "dimension": 1024, "metric": "cosine"}}`,
			vectorType: "dense",
			metric:     IndexMetricCosine,
			embed:      dense,
		},
	}

	assertIndex := func(t *testing.T, idx *Index, vectorType string, metric IndexMetric, embed *IndexEmbed) {
		t.Helper()
		require.NotNil(t, idx)
		assert.Equal(t, vectorType, idx.VectorType)
		assert.Equal(t, metric, idx.Metric)
		assert.Equal(t, embed.Dimension, idx.Dimension)
		assert.Equal(t, embed, idx.Embed)
		require.NotNil(t, idx.Schema)
		field := idx.Schema.Fields["chunk_text"]
		require.NotNil(t, field.SemanticText)
		assert.Equal(t, embed.Dimension, field.SemanticText.Dimension)
	}

	for _, ft := range fieldsTests {
		t.Run(ft.name, func(t *testing.T) {
			index := indexModelFixture{name: "model-index", fields: ft.fields}.JSON()

			t.Run("CreateIndexForModel", func(t *testing.T) {
				server := newWireServer(t, http.StatusCreated, index)
				idx, err := newWireClient(t, server.URL).CreateIndexForModel(context.Background(), &CreateIndexForModelRequest{
					Name:   "model-index",
					Cloud:  CloudAWS,
					Region: "us-east-1",
					Embed: CreateIndexForModelEmbed{
						Model:    ft.embed.Model,
						FieldMap: map[string]interface{}{"text": "chunk_text"},
					},
				})
				require.NoError(t, err)
				assertIndex(t, idx, ft.vectorType, ft.metric, ft.embed)
			})

			t.Run("DescribeIndex", func(t *testing.T) {
				server := newWireServer(t, http.StatusOK, index)
				idx, err := newWireClient(t, server.URL).DescribeIndex(context.Background(), "model-index")
				require.NoError(t, err)
				assertIndex(t, idx, ft.vectorType, ft.metric, ft.embed)
			})

			t.Run("ListIndexes", func(t *testing.T) {
				server := newWireServer(t, http.StatusOK, `{"indexes": [`+index+`], "incompatible_indexes": []}`)
				indexes, err := newWireClient(t, server.URL).ListIndexes(context.Background())
				require.NoError(t, err)
				require.Len(t, indexes, 1)
				assertIndex(t, indexes[0], ft.vectorType, ft.metric, ft.embed)
			})
		})
	}
}

func TestDocumentIndexDenseSlotWireUnit(t *testing.T) {
	const embedFields = `{
		"body": {"type": "string", "store_text": false, "embed": {"model": "llama-text-embed-v2", "dimension": 1024, "metric": "cosine"}},
		"terms": {"type": "sparse_vector"}
	}`

	t.Run("DescribeIndex", func(t *testing.T) {
		server := newWireServer(t, http.StatusOK, indexModelFixture{name: "docs", fields: embedFields}.JSON())
		idx, err := newWireClient(t, server.URL).DescribeIndex(context.Background(), "docs")
		require.NoError(t, err)
		assert.Equal(t, "dense", idx.VectorType)
		assert.Equal(t, ptr(int32(1024)), idx.Dimension)
		assert.Equal(t, IndexMetricCosine, idx.Metric)
		assert.Nil(t, idx.Embed)
	})

	t.Run("DescribeBackup", func(t *testing.T) {
		server := newWireServer(t, http.StatusOK, `{"backup_id": "bkp-1", "source_index_name": "docs", "source_index_id": "idx-1",
			"status": "Ready", "cloud": "aws", "region": "us-east-1", "schema": {"fields": `+embedFields+`}}`)
		backup, err := newWireClient(t, server.URL).DescribeBackup(context.Background(), "bkp-1")
		require.NoError(t, err)
		assert.Equal(t, ptr(int32(1024)), backup.Dimension)
		assert.Equal(t, ptr(IndexMetricCosine), backup.Metric)
	})
}

func TestUnknownTypesReusingKnownPropertiesWireUnit(t *testing.T) {
	const knownField = `"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"}`
	knownFields := map[string]IndexSchemaField{
		"_values": {DenseVector: &DenseVectorField{Dimension: 3, Metric: IndexMetricCosine}},
	}
	managed := &IndexDeployment{Managed: &ManagedDeployment{Cloud: CloudAWS, Region: "us-east-1"}}

	cases := []struct {
		name       string
		fields     string
		deployment string
		want       *IndexDeployment
	}{
		{name: "field type reusing metric as an object", fields: `{` + knownField + `, "loc": {"type": "geo_point", "metric": {"kind": "haversine"}}}`, want: managed},
		{name: "field type reusing dimension as a string", fields: `{` + knownField + `, "loc": {"type": "geo_point", "dimension": "2d"}}`, want: managed},
		{name: "field type reusing embed as a string", fields: `{` + knownField + `, "loc": {"type": "geo_point", "embed": "auto"}}`, want: managed},
		{name: "field type that isn't a string", fields: `{` + knownField + `, "loc": {"type": {"name": "geo_point"}, "metric": {"kind": "haversine"}}}`, want: managed},
		{name: "deployment type reusing replicas as a string", fields: `{` + knownField + `}`, deployment: `{"deployment_type": "edge", "environment": "edge-1", "replicas": "auto", "shards": [1]}`},
		{name: "deployment type that isn't a string", fields: `{` + knownField + `}`, deployment: `{"deployment_type": 7, "replicas": "auto"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := indexModelFixture{name: "future-index", fields: tc.fields, deployment: tc.deployment}.JSON()
			backup := `{"backup_id": "bkp-1", "source_index_name": "future-index", "source_index_id": "idx-1", "status": "Ready",
				"cloud": "aws", "region": "us-east-1", "schema": {"fields": ` + tc.fields + `}}`

			assertIndex := func(t *testing.T, idx *Index) {
				t.Helper()
				require.NotNil(t, idx)
				require.NotNil(t, idx.Schema)
				assert.Equal(t, knownFields, idx.Schema.Fields)
				assert.Equal(t, tc.want, idx.Deployment)
				assert.Equal(t, ptr(int32(3)), idx.Dimension)
				if tc.want == nil {
					assert.Nil(t, idx.Spec)
				}
			}

			indexCalls := []struct {
				name     string
				status   int
				response string
				call     func(pc *Client) (*Index, error)
			}{
				{"DescribeIndex", http.StatusOK, index, func(pc *Client) (*Index, error) {
					return pc.DescribeIndex(context.Background(), "future-index")
				}},
				{"CreateIndex", http.StatusCreated, index, func(pc *Client) (*Index, error) {
					return pc.CreateIndex(context.Background(), &CreateIndexRequest{
						Name:   "future-index",
						Schema: IndexSchema{Fields: map[string]IndexSchemaField{"_values": knownFields["_values"]}},
					})
				}},
				{"CreateIndexForModel", http.StatusCreated, index, func(pc *Client) (*Index, error) {
					return pc.CreateIndexForModel(context.Background(), &CreateIndexForModelRequest{
						Name:   "future-index",
						Cloud:  CloudAWS,
						Region: "us-east-1",
						Embed:  CreateIndexForModelEmbed{Model: "multilingual-e5-large", FieldMap: map[string]interface{}{"text": "chunk_text"}},
					})
				}},
				{"ConfigureIndex", http.StatusOK, index, func(pc *Client) (*Index, error) {
					return pc.ConfigureIndex(context.Background(), "future-index", ConfigureIndexParams{Tags: IndexTags{"env": "dev"}})
				}},
				{"ListIndexes", http.StatusOK, `{"indexes": [` + index + `, ` + indexModelFixture{}.JSON() + `], "incompatible_indexes": []}`, func(pc *Client) (*Index, error) {
					indexes, err := pc.ListIndexes(context.Background())
					if err != nil {
						return nil, err
					}
					require.Len(t, indexes, 2)
					assert.Equal(t, "test-index", indexes[1].Name)
					return indexes[0], nil
				}},
			}
			for _, call := range indexCalls {
				t.Run(call.name, func(t *testing.T) {
					server := newWireServer(t, call.status, call.response)
					idx, err := call.call(newWireClient(t, server.URL))
					require.NoError(t, err)
					assertIndex(t, idx)
				})
			}

			t.Run("DescribeBackup", func(t *testing.T) {
				server := newWireServer(t, http.StatusOK, backup)
				got, err := newWireClient(t, server.URL).DescribeBackup(context.Background(), "bkp-1")
				require.NoError(t, err)
				require.NotNil(t, got.Schema)
				assert.Equal(t, knownFields, got.Schema.Fields)
			})

			t.Run("ListBackups", func(t *testing.T) {
				server := newWireServer(t, http.StatusOK, `{"data": [`+backup+`], "pagination": null, "incompatible_data": []}`)
				got, err := newWireClient(t, server.URL).ListBackups(context.Background(), nil)
				require.NoError(t, err)
				require.Len(t, got.Data, 1)
				require.NotNil(t, got.Data[0].Schema)
				assert.Equal(t, knownFields, got.Data[0].Schema.Fields)
			})
		})
	}
}

func TestMistypedKnownPropertyFailsWireUnit(t *testing.T) {
	cases := []struct {
		name       string
		fields     string
		deployment string
	}{
		{name: "dense vector dimension as a string", fields: `{"_values": {"type": "dense_vector", "dimension": "1536", "metric": "cosine"}}`},
		{name: "managed deployment cloud as a number", deployment: `{"deployment_type": "managed", "cloud": 123, "region": "us-east-1"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := indexModelFixture{fields: tc.fields, deployment: tc.deployment}.JSON()

			t.Run("DescribeIndex", func(t *testing.T) {
				server := newWireServer(t, http.StatusOK, index)
				_, err := newWireClient(t, server.URL).DescribeIndex(context.Background(), "test-index")
				require.Error(t, err)
			})

			t.Run("ListIndexes", func(t *testing.T) {
				server := newWireServer(t, http.StatusOK, `{"indexes": [`+index+`], "incompatible_indexes": []}`)
				_, err := newWireClient(t, server.URL).ListIndexes(context.Background())
				require.Error(t, err)
			})
		})
	}
}

func TestImportWireUnit(t *testing.T) {
	const importJSON = `{"id": "101", "uri": "s3://b/d", "status": "Completed", "createdAt": "2026-10-01T12:00:00Z",
		"finishedAt": "2026-10-01T13:00:00Z", "percentComplete": 100, "recordsImported": 999990,
		"error": "some rows failed", "validationResult": {"importSuccesses": 999990, "importErrors": 10}}`
	wantValidation := &ImportValidationResult{ImportSuccesses: ptr(int64(999990)), ImportErrors: ptr(int64(10))}

	t.Run("DescribeImport validation result", func(t *testing.T) {
		server := newWireServer(t, http.StatusOK, importJSON)
		res, err := newWireIndexConnection(t, server.URL, "").DescribeImport(context.Background(), "101")
		require.NoError(t, err)
		assert.Equal(t, wantValidation, res.ValidationResult)
		assertRequests(t, server.requests(), []wantRequest{{method: http.MethodGet, path: "/bulk/imports/101"}})
	})

	t.Run("ListImports validation result", func(t *testing.T) {
		server := newWireServer(t, http.StatusOK, `{"data": [`+importJSON+`]}`)
		res, err := newWireIndexConnection(t, server.URL, "").ListImports(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, res.Imports, 1)
		assert.Equal(t, wantValidation, res.Imports[0].ValidationResult)
	})

	t.Run("DescribeImport before validation", func(t *testing.T) {
		server := newWireServer(t, http.StatusOK, `{"id": "102", "uri": "s3://b/d", "status": "Pending", "percentComplete": 0, "recordsImported": 0}`)
		res, err := newWireIndexConnection(t, server.URL, "").DescribeImport(context.Background(), "102")
		require.NoError(t, err)
		assert.Nil(t, res.ValidationResult)
		assert.Nil(t, res.CreatedAt, "a body without createdAt leaves CreatedAt nil")
	})

	t.Run("empty id is rejected before sending", func(t *testing.T) {
		server := newWireServer(t, http.StatusOK, `{}`)
		idx := newWireIndexConnection(t, server.URL, "")

		res, err := idx.DescribeImport(context.Background(), "")
		require.ErrorContains(t, err, "id")
		assert.Nil(t, res)

		err = idx.CancelImport(context.Background(), "")
		require.ErrorContains(t, err, "id")

		assert.Empty(t, server.requests())
	})
}

// A "." or ".." namespace must reach the server as a literal path segment. Unencoded, URL
// resolution removes it ("/namespaces/../documents/upsert" becomes "/documents/upsert").
func TestDotNamespaceRestPathUnit(t *testing.T) {
	calls := []struct {
		name     string
		status   int
		response string
		path     string
		call     func(idx *IndexConnection) error
	}{
		{"UpsertDocuments", http.StatusAccepted, `{"upserted_count": 1}`, "/namespaces/%s/documents/upsert", func(idx *IndexConnection) error {
			_, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{Documents: []Document{{"_id": "doc-1"}}})
			return err
		}},
		{"SearchDocuments", http.StatusOK, `{"matches": [], "namespace": "x", "usage": {"read_units": 1}}`, "/namespaces/%s/documents/search", func(idx *IndexConnection) error {
			_, err := idx.SearchDocuments(context.Background(), &SearchDocumentsRequest{
				TopK:    1,
				ScoreBy: []DocumentScoringMethod{{Type: DocumentScoringTypeText, Fields: []string{"title"}, Query: ptr("q")}},
			})
			return err
		}},
		{"FetchDocuments", http.StatusOK, `{"documents": {}, "namespace": "x", "usage": {"read_units": 1}}`, "/namespaces/%s/documents/fetch", func(idx *IndexConnection) error {
			_, err := idx.FetchDocuments(context.Background(), &FetchDocumentsRequest{Ids: []string{"doc-1"}})
			return err
		}},
		{"DeleteDocuments", http.StatusAccepted, `{}`, "/namespaces/%s/documents/delete", func(idx *IndexConnection) error {
			_, err := idx.DeleteDocuments(context.Background(), &DeleteDocumentsRequest{Ids: []string{"doc-1"}})
			return err
		}},
		{"UpdateDocuments", http.StatusAccepted, `{"matched_records": 1}`, "/namespaces/%s/documents/update", func(idx *IndexConnection) error {
			_, err := idx.UpdateDocuments(context.Background(), &UpdateDocumentsRequest{Documents: []Document{{"_id": "doc-1"}}})
			return err
		}},
		{"ListDocuments", http.StatusOK, `{"documents": [], "namespace": "x", "usage": {"read_units": 1}}`, "/namespaces/%s/documents/list", func(idx *IndexConnection) error {
			_, err := idx.ListDocuments(context.Background(), &ListDocumentsRequest{})
			return err
		}},
		{"UpsertRecords", http.StatusCreated, ``, "/records/namespaces/%s/upsert", func(idx *IndexConnection) error {
			return idx.UpsertRecords(context.Background(), []*IntegratedRecord{{"_id": "rec-1", "chunk_text": "apples"}})
		}},
		{"SearchRecords", http.StatusOK, `{"namespace": "x", "result": {"hits": []}, "usage": {"read_units": 1}}`, "/records/namespaces/%s/search", func(idx *IndexConnection) error {
			_, err := idx.SearchRecords(context.Background(), &SearchRecordsRequest{
				Query: SearchRecordsQuery{TopK: 1, Inputs: &map[string]interface{}{"text": "apples"}},
			})
			return err
		}},
	}
	namespaces := []struct{ namespace, segment string }{
		{".", "%2E"},
		{"..", "%2E%2E"},
		{"...", "..."},
		{"a/..", "a%2F.."},
	}

	for _, c := range calls {
		for _, ns := range namespaces {
			t.Run(c.name+" "+ns.namespace, func(t *testing.T) {
				var mu sync.Mutex
				var got []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					got = append(got, r.URL.EscapedPath())
					mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(c.status)
					_, _ = w.Write([]byte(c.response))
				}))
				t.Cleanup(server.Close)

				require.NoError(t, c.call(newWireIndexConnection(t, server.URL, ns.namespace)))
				mu.Lock()
				defer mu.Unlock()
				assert.Equal(t, []string{fmt.Sprintf(c.path, ns.segment)}, got)
			})
		}
	}
}

// Bulk import errors use the rpcStatus shape ({"code": <gRPC code>, "message": ...}); documents
// and records errors use ErrorResponse ({"error": {"code": ..., "message": ...}}). Both decode into
// PineconeError's ErrorCode and Message.
func TestRestDataPlaneErrorShapesUnit(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		response      string
		call          func(idx *IndexConnection) error
		wantErrorCode string
		wantMessage   string
	}{
		{
			name:     "StartImport rpcStatus",
			status:   http.StatusBadRequest,
			response: `{"code": 3, "message": "Invalid URI: bucket s3://nope does not exist", "details": []}`,
			call: func(idx *IndexConnection) error {
				_, err := idx.StartImport(context.Background(), "s3://nope/data", nil, nil)
				return err
			},
			wantErrorCode: "INVALID_ARGUMENT",
			wantMessage:   "failed to start import: Invalid URI: bucket s3://nope does not exist",
		},
		{
			name:     "DescribeImport rpcStatus",
			status:   http.StatusNotFound,
			response: `{"code": 5, "message": "Import 101 not found", "details": []}`,
			call: func(idx *IndexConnection) error {
				_, err := idx.DescribeImport(context.Background(), "101")
				return err
			},
			wantErrorCode: "NOT_FOUND",
			wantMessage:   "failed to describe import: Import 101 not found",
		},
		{
			name:     "ListImports rpcStatus",
			status:   http.StatusBadRequest,
			response: `{"code": 3, "message": "Invalid pagination token"}`,
			call: func(idx *IndexConnection) error {
				_, err := idx.ListImports(context.Background(), &ListImportsRequest{PaginationToken: ptr("bad")})
				return err
			},
			wantErrorCode: "INVALID_ARGUMENT",
			wantMessage:   "failed to list imports: Invalid pagination token",
		},
		{
			name:     "CancelImport rpcStatus",
			status:   http.StatusPreconditionFailed,
			response: `{"code": 9, "message": "Import 101 has already completed", "details": []}`,
			call: func(idx *IndexConnection) error {
				return idx.CancelImport(context.Background(), "101")
			},
			wantErrorCode: "FAILED_PRECONDITION",
			wantMessage:   "failed to cancel import: Import 101 has already completed",
		},
		{
			name:     "StartImport rpcStatus unrecognized code",
			status:   http.StatusInternalServerError,
			response: `{"code": 99, "message": "something new"}`,
			call: func(idx *IndexConnection) error {
				_, err := idx.StartImport(context.Background(), "s3://b/d", nil, nil)
				return err
			},
			wantErrorCode: "UNKNOWN",
			wantMessage:   "failed to start import: something new",
		},
		{
			name:     "UpsertDocuments ErrorResponse",
			status:   http.StatusBadRequest,
			response: `{"error": {"code": "INVALID_ARGUMENT", "message": "Document is missing _id"}, "status": 400}`,
			call: func(idx *IndexConnection) error {
				_, err := idx.UpsertDocuments(context.Background(), &UpsertDocumentsRequest{Documents: []Document{{"_id": "doc-1"}}})
				return err
			},
			wantErrorCode: "INVALID_ARGUMENT",
			wantMessage:   "failed to upsert documents: Document is missing _id",
		},
		{
			name:     "SearchRecords ErrorResponse",
			status:   http.StatusBadRequest,
			response: `{"error": {"code": "INVALID_ARGUMENT", "message": "Index does not support integrated inference"}, "status": 400}`,
			call: func(idx *IndexConnection) error {
				_, err := idx.SearchRecords(context.Background(), &SearchRecordsRequest{
					Query: SearchRecordsQuery{TopK: 1, Inputs: &map[string]interface{}{"text": "apples"}},
				})
				return err
			},
			wantErrorCode: "INVALID_ARGUMENT",
			wantMessage:   "failed to search records: Index does not support integrated inference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWireServer(t, tt.status, tt.response)
			err := tt.call(newWireIndexConnection(t, server.URL, "ns"))
			var pcErr *PineconeError
			require.ErrorAs(t, err, &pcErr)
			assert.Equal(t, tt.status, pcErr.Code)
			assert.Equal(t, tt.wantErrorCode, pcErr.ErrorCode)
			assert.Equal(t, tt.wantMessage, pcErr.Message)
			assert.Contains(t, pcErr.Error(), tt.wantMessage)
		})
	}
}

// TestIncompatibleListKeysWireUnit pins how ListBackups and ListRestoreJobs choose between
// incompatible_data and the pre-rename key, and that all four incompatible lists report a value
// that isn't an array as an error instead of reading it as empty.
func TestIncompatibleListKeysWireUnit(t *testing.T) {
	backup := func(id string) string {
		return fmt.Sprintf(`{"backup_id": %q, "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}`, id)
	}
	job := func(id string) string {
		return fmt.Sprintf(`{"restore_job_id": %q, "minimum_api_version": "2027-04", "preview_api_version": null, "message": "m"}`, id)
	}
	lists := []struct {
		name     string
		oldKey   string
		entry    func(id string) string
		call     func(pc *Client) ([]string, error)
		errorKey string
	}{
		{
			name:   "ListBackups",
			oldKey: "incompatible_backups",
			entry:  backup,
			call: func(pc *Client) ([]string, error) {
				res, err := pc.ListBackups(context.Background(), nil)
				if err != nil {
					return nil, err
				}
				ids := []string{}
				for _, b := range res.IncompatibleData {
					ids = append(ids, b.BackupId)
				}
				return ids, nil
			},
		},
		{
			name:   "ListRestoreJobs",
			oldKey: "incompatible_restore_jobs",
			entry:  job,
			call: func(pc *Client) ([]string, error) {
				res, err := pc.ListRestoreJobs(context.Background(), nil)
				if err != nil {
					return nil, err
				}
				ids := []string{}
				for _, j := range res.IncompatibleData {
					ids = append(ids, j.RestoreJobId)
				}
				return ids, nil
			},
		},
	}
	cases := []struct {
		name string
		// newValue and oldValue are raw JSON for incompatible_data and the pre-rename key; empty
		// leaves the key out. %s in them is replaced with an entry for the given id.
		newValue string
		oldValue string
		want     []string
		wantErr  string
	}{
		{name: "new key only", newValue: "[NEW]", want: []string{"new"}},
		{name: "old key only", oldValue: "[OLD]", want: []string{"old"}},
		{name: "both keys: new wins", newValue: "[NEW]", oldValue: "[OLD]", want: []string{"new"}},
		{name: "new null falls back to old", newValue: "null", oldValue: "[OLD]", want: []string{"old"}},
		{name: "new empty array wins over old", newValue: "[]", oldValue: "[OLD]", want: []string{}},
		{name: "neither key", want: []string{}},
		{name: "entries with an empty id are skipped", newValue: `[EMPTY, NEW]`, want: []string{"new"}},
		{name: "new is an object", newValue: `{"id": "x"}`, oldValue: "[OLD]", wantErr: "failed to decode incompatible_data"},
		{name: "new is a string", newValue: `"oops"`, oldValue: "[OLD]", wantErr: "failed to decode incompatible_data"},
		{name: "new is a number", newValue: `7`, wantErr: "failed to decode incompatible_data"},
		{name: "old is an object and new is absent", oldValue: `{"id": "x"}`, wantErr: "failed to decode OLDKEY"},
		{name: "old is malformed but new is valid", newValue: "[NEW]", oldValue: `"oops"`, want: []string{"new"}},
	}
	for _, list := range lists {
		for _, tc := range cases {
			t.Run(list.name+"/"+tc.name, func(t *testing.T) {
				expand := func(v string) string {
					v = strings.ReplaceAll(v, "NEW", list.entry("new"))
					v = strings.ReplaceAll(v, "OLD", list.entry("old"))
					return strings.ReplaceAll(v, "EMPTY", list.entry(""))
				}
				body := `{"data": [], "pagination": null`
				if tc.newValue != "" {
					body += `, "incompatible_data": ` + expand(tc.newValue)
				}
				if tc.oldValue != "" {
					body += fmt.Sprintf(`, %q: %s`, list.oldKey, expand(tc.oldValue))
				}
				body += "}"
				server := newWireServer(t, http.StatusOK, body)
				ids, err := list.call(newWireClient(t, server.URL))
				if tc.wantErr != "" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), strings.ReplaceAll(tc.wantErr, "OLDKEY", list.oldKey))
					var pcErr *PineconeError
					assert.False(t, errors.As(err, &pcErr), "a malformed list is a decode error, not a PineconeError")
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tc.want, ids)
			})
		}
	}

	nonArrays := []struct {
		name  string
		value string
	}{
		{"object", `{"name": "x"}`},
		{"string", `"oops"`},
	}
	for _, v := range nonArrays {
		t.Run("ListIncompatibleIndexes/"+v.name, func(t *testing.T) {
			server := newWireServer(t, http.StatusOK, `{"indexes": [], "incompatible_indexes": `+v.value+`}`)
			_, err := newWireClient(t, server.URL).ListIncompatibleIndexes(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to decode incompatible_indexes")
		})
		t.Run("ListIncompatibleCollections/"+v.name, func(t *testing.T) {
			server := newWireServer(t, http.StatusOK, `{"collections": [], "incompatible_collections": `+v.value+`}`)
			_, err := newWireClient(t, server.URL).ListIncompatibleCollections(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to decode incompatible_collections")
		})
	}
}

// TestControlPlaneValidationSendsNoRequestUnit pins client-side validation that must return a
// plain error before any request is sent.
func TestControlPlaneValidationSendsNoRequestUnit(t *testing.T) {
	tests := []struct {
		name    string
		call    func(pc *Client) error
		wantErr string
	}{
		{"DescribeIndex with an empty name", func(pc *Client) error {
			_, err := pc.DescribeIndex(context.Background(), "")
			return err
		}, "you must provide an idxName to describe an index"},
		{"DeleteIndex with an empty name", func(pc *Client) error {
			return pc.DeleteIndex(context.Background(), "")
		}, "you must provide an idxName to delete an index"},
		{"ConfigureIndex with an empty name", func(pc *Client) error {
			_, err := pc.ConfigureIndex(context.Background(), "", ConfigureIndexParams{DeletionProtection: DeletionProtectionEnabled})
			return err
		}, "you must provide a name to configure an index"},
		{"ConfigureIndex with an empty ReadCapacityParams", func(pc *Client) error {
			_, err := pc.ConfigureIndex(context.Background(), "docs", ConfigureIndexParams{ReadCapacity: &ReadCapacityParams{}})
			return err
		}, "ReadCapacity must set Dedicated or OnDemand when configuring an index"},
		{"DescribeCollection with an empty name", func(pc *Client) error {
			_, err := pc.DescribeCollection(context.Background(), "")
			return err
		}, "you must provide a collectionName to describe a collection"},
		{"DeleteCollection with an empty name", func(pc *Client) error {
			return pc.DeleteCollection(context.Background(), "")
		}, "you must provide a collectionName to delete a collection"},
		{"ListBackups with an empty IndexName", func(pc *Client) error {
			_, err := pc.ListBackups(context.Background(), &ListBackupsParams{IndexName: ptr("")})
			return err
		}, "IndexName must not be empty; leave it nil to list the project's backups"},
		{"DescribeBackup with an empty id", func(pc *Client) error {
			_, err := pc.DescribeBackup(context.Background(), "")
			return err
		}, "you must provide a backupId to describe a backup"},
		{"DeleteBackup with an empty id", func(pc *Client) error {
			return pc.DeleteBackup(context.Background(), "")
		}, "you must provide a backupId to delete a backup"},
		{"DescribeRestoreJob with an empty id", func(pc *Client) error {
			_, err := pc.DescribeRestoreJob(context.Background(), "")
			return err
		}, "you must provide a restoreJobId to describe a restore job"},
		{"CreateBackup with nil params", func(pc *Client) error {
			_, err := pc.CreateBackup(context.Background(), nil)
			return err
		}, "in (*CreateBackupParams) cannot be nil"},
		{"CreateBackup with an empty IndexName", func(pc *Client) error {
			_, err := pc.CreateBackup(context.Background(), &CreateBackupParams{})
			return err
		}, "IndexName must be included in CreateBackupParams"},
		{"CreateIndexFromBackup with nil params", func(pc *Client) error {
			_, err := pc.CreateIndexFromBackup(context.Background(), nil)
			return err
		}, "in (*CreateIndexFromBackupParams) cannot be nil"},
		{"CreateIndexFromBackup with an empty BackupId", func(pc *Client) error {
			_, err := pc.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{Name: "restored"})
			return err
		}, "BackupId must be included in CreateIndexFromBackupParams"},
		{"CreateIndexFromBackup with an empty Name", func(pc *Client) error {
			_, err := pc.CreateIndexFromBackup(context.Background(), &CreateIndexFromBackupParams{BackupId: "bkp-1"})
			return err
		}, "Name must be included in CreateIndexFromBackupParams"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWireServer(t, http.StatusOK, `{}`)
			err := tt.call(newWireClient(t, server.URL))
			require.EqualError(t, err, tt.wantErr)
			var pcErr *PineconeError
			assert.False(t, errors.As(err, &pcErr), "validation errors are plain errors")
			assert.Empty(t, server.requests(), "no request may be sent")
		})
	}
}

// ConfigureIndex rejects a Schema key that doesn't name a semantic_text field of the index, because
// the API renames the field (and the legacy field map) to the key instead of rejecting it.
func TestConfigureIndexSchemaKeyMustNameSemanticTextFieldUnit(t *testing.T) {
	readParams := &map[string]interface{}{"input_type": "query"}
	tests := []struct {
		name    string
		fields  string
		key     string
		wantErr string
	}{
		{
			name:    "misspelled key",
			fields:  semanticTextFieldsJSON,
			key:     "chunk_txt",
			wantErr: `Schema field "chunk_txt" isn't a semantic_text field of index "integrated-index"; its semantic_text field is "chunk_text"`,
		},
		{
			name:    "index without a semantic_text field",
			fields:  denseFieldsJSON,
			key:     "_values",
			wantErr: `Schema field "_values" isn't a semantic_text field of index "integrated-index"; Schema applies only to the semantic_text field of a legacy integrated index`,
		},
		{
			name:   "key names the semantic_text field",
			fields: semanticTextFieldsJSON,
			key:    "chunk_text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWireServer(t, http.StatusOK, indexModelFixture{name: "integrated-index", fields: tt.fields}.JSON())
			_, err := newWireClient(t, server.URL).ConfigureIndex(context.Background(), "integrated-index", ConfigureIndexParams{
				Schema: &ConfigureIndexSchema{Fields: map[string]ConfigureSemanticTextField{tt.key: {ReadParameters: readParams}}},
			})
			got := server.requests()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				var pcErr *PineconeError
				assert.False(t, errors.As(err, &pcErr), "validation errors are plain errors")
				require.Len(t, got, 1, "only the describe request may be sent")
				assert.Equal(t, http.MethodGet, got[0].method)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, 2)
			assert.Equal(t, http.MethodPatch, got[1].method)
			assert.JSONEq(t, `{"schema": {"fields": {"chunk_text": {"type": "semantic_text", "read_parameters": {"input_type": "query"}}}}, "tags": null}`, got[1].body)
		})
	}
}
