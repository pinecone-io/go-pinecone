package pinecone

import (
	"encoding/json"
	"testing"

	"github.com/pinecone-io/go-pinecone/v7/internal/gen/db_control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToIndexSchemaUnit(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected IndexSchemaField
		skipped  bool
	}{
		{
			name:  "dense vector",
			field: `{"type": "dense_vector", "dimension": 1536, "metric": "dotproduct", "description": "Embeddings"}`,
			expected: IndexSchemaField{DenseVector: &DenseVectorField{
				Dimension:   1536,
				Metric:      IndexMetricDotproduct,
				Description: ptr("Embeddings"),
			}},
		},
		{
			name:     "sparse vector",
			field:    `{"type": "sparse_vector", "description": null}`,
			expected: IndexSchemaField{SparseVector: &SparseVectorField{}},
		},
		{
			name: "semantic text",
			field: `{"type": "semantic_text", "model": "multilingual-e5-large", "dimension": 1024, "metric": "cosine",
				"read_parameters": {"input_type": "query"}, "write_parameters": {"input_type": "passage"}}`,
			expected: IndexSchemaField{SemanticText: &SemanticTextField{
				Model:           "multilingual-e5-large",
				Dimension:       ptr(int32(1024)),
				Metric:          ptr(IndexMetricCosine),
				ReadParameters:  &map[string]interface{}{"input_type": "query"},
				WriteParameters: &map[string]interface{}{"input_type": "passage"},
			}},
		},
		{
			name: "string with full-text search",
			field: `{"type": "string", "description": "Title", "filterable": true,
				"full_text_search": {"language": "en", "stemming": true, "stop_words": true}}`,
			expected: IndexSchemaField{String: &StringField{
				Description: ptr("Title"),
				Filterable:  ptr(true),
				FullTextSearch: &FullTextSearchConfig{
					Language:  ptr("en"),
					Stemming:  ptr(true),
					StopWords: ptr(true),
				},
			}},
		},
		{
			name: "string with n-gram full-text search",
			field: `{"type": "string",
				"full_text_search": {"language": "en", "stemming": false, "stop_words": false,
					"ngram": {"min_gram": 2, "max_gram": 4, "prefix_only": true}}}`,
			expected: IndexSchemaField{String: &StringField{
				FullTextSearch: &FullTextSearchConfig{
					Language:  ptr("en"),
					Stemming:  ptr(false),
					StopWords: ptr(false),
					Ngram:     &NgramConfig{MinGram: 2, MaxGram: 4, PrefixOnly: ptr(true)},
				},
			}},
		},
		{
			name:     "string without full-text search",
			field:    `{"type": "string", "filterable": true}`,
			expected: IndexSchemaField{String: &StringField{Filterable: ptr(true)}},
		},
		{
			name:     "string list",
			field:    `{"type": "string_list", "filterable": true, "description": "Tags"}`,
			expected: IndexSchemaField{StringList: &StringListField{Filterable: ptr(true), Description: ptr("Tags")}},
		},
		{
			name:     "boolean",
			field:    `{"type": "boolean", "filterable": false}`,
			expected: IndexSchemaField{Boolean: &BooleanField{Filterable: ptr(false)}},
		},
		{
			name:     "float",
			field:    `{"type": "float", "filterable": true}`,
			expected: IndexSchemaField{Float: &FloatField{Filterable: ptr(true)}},
		},
		{
			name:     "integer",
			field:    `{"type": "integer", "filterable": true}`,
			expected: IndexSchemaField{Integer: &IntegerField{Filterable: ptr(true)}},
		},
		{
			name:     "legacy metadata field without a type",
			field:    `{"filterable": true}`,
			expected: IndexSchemaField{LegacyMetadata: &LegacyMetadataField{Filterable: true}},
		},
		{
			name:    "unknown field type is skipped",
			field:   `{"type": "geo_point", "filterable": true}`,
			skipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema db_control.IndexSchema
			require.NoError(t, json.Unmarshal([]byte(`{"fields": {"f": `+tt.field+`}}`), &schema))

			result := toIndexSchema(&schema)
			require.NotNil(t, result)

			if tt.skipped {
				assert.Empty(t, result.Fields)
				return
			}
			require.Contains(t, result.Fields, "f")
			assert.Equal(t, tt.expected, result.Fields["f"])
		})
	}

	t.Run("nil schema", func(t *testing.T) {
		assert.Nil(t, toIndexSchema(nil))
	})

	t.Run("unknown field types do not drop known fields", func(t *testing.T) {
		var schema db_control.IndexSchema
		require.NoError(t, json.Unmarshal([]byte(`{"fields": {
			"_values": {"type": "dense_vector", "dimension": 3, "metric": "cosine"},
			"location": {"type": "geo_point"}
		}}`), &schema))

		result := toIndexSchema(&schema)
		require.NotNil(t, result)
		assert.Len(t, result.Fields, 1)
		assert.Contains(t, result.Fields, "_values")
	})
}

func TestToDbCreateIndexSchemaUnit(t *testing.T) {
	tests := []struct {
		name     string
		field    IndexSchemaField
		expected string
	}{
		{
			name:     "dense vector",
			field:    IndexSchemaField{DenseVector: &DenseVectorField{Dimension: 1536, Metric: IndexMetricCosine}},
			expected: `{"type": "dense_vector", "dimension": 1536, "metric": "cosine"}`,
		},
		{
			name:     "dense vector with description",
			field:    IndexSchemaField{DenseVector: &DenseVectorField{Dimension: 3, Metric: IndexMetricEuclidean, Description: ptr("Embeddings")}},
			expected: `{"type": "dense_vector", "dimension": 3, "metric": "euclidean", "description": "Embeddings"}`,
		},
		{
			name:     "sparse vector",
			field:    IndexSchemaField{SparseVector: &SparseVectorField{}},
			expected: `{"type": "sparse_vector"}`,
		},
		{
			name:     "sparse vector with description",
			field:    IndexSchemaField{SparseVector: &SparseVectorField{Description: ptr("Keywords")}},
			expected: `{"type": "sparse_vector", "description": "Keywords"}`,
		},
		{
			name: "string with full-text search",
			field: IndexSchemaField{String: &StringField{
				Description: ptr("Title"),
				FullTextSearch: &FullTextSearchConfig{
					Language:  ptr("en"),
					Stemming:  ptr(true),
					StopWords: ptr(true),
				},
			}},
			expected: `{"type": "string", "description": "Title",
				"full_text_search": {"language": "en", "stemming": true, "stop_words": true}}`,
		},
		{
			name: "string with n-gram full-text search",
			field: IndexSchemaField{String: &StringField{
				FullTextSearch: &FullTextSearchConfig{
					Ngram: &NgramConfig{MinGram: 2, MaxGram: 4, PrefixOnly: ptr(true)},
				},
			}},
			expected: `{"type": "string", "full_text_search": {"ngram": {"min_gram": 2, "max_gram": 4, "prefix_only": true}}}`,
		},
		{
			name: "n-gram without prefix_only",
			field: IndexSchemaField{String: &StringField{
				FullTextSearch: &FullTextSearchConfig{Ngram: &NgramConfig{MinGram: 1, MaxGram: 3}},
			}},
			expected: `{"type": "string", "full_text_search": {"ngram": {"min_gram": 1, "max_gram": 3}}}`,
		},
		{
			name:     "string with empty full-text search config",
			field:    IndexSchemaField{String: &StringField{FullTextSearch: &FullTextSearchConfig{}}},
			expected: `{"type": "string", "full_text_search": {}}`,
		},
		{
			name: "string Filterable is not sent",
			field: IndexSchemaField{String: &StringField{
				Filterable:     ptr(true),
				FullTextSearch: &FullTextSearchConfig{Language: ptr("en")},
			}},
			expected: `{"type": "string", "full_text_search": {"language": "en"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toDbCreateIndexSchema(IndexSchema{Fields: map[string]IndexSchemaField{"f": tt.field}})
			require.NoError(t, err)
			assert.JSONEq(t, `{"fields": {"f": `+tt.expected+`}}`, marshalJSON(t, result))
		})
	}

	errorTests := []struct {
		name     string
		field    IndexSchemaField
		expected string
	}{
		{
			name:     "string without full-text search",
			field:    IndexSchemaField{String: &StringField{Filterable: ptr(true)}},
			expected: `field "f": a StringField can only be declared with FullTextSearch set`,
		},
		{
			name:     "semantic text",
			field:    IndexSchemaField{SemanticText: &SemanticTextField{Model: "multilingual-e5-large"}},
			expected: `field "f": SemanticText fields can't be declared with CreateIndex`,
		},
		{
			name:     "string list",
			field:    IndexSchemaField{StringList: &StringListField{}},
			expected: `field "f": metadata fields don't need to be declared`,
		},
		{
			name:     "boolean",
			field:    IndexSchemaField{Boolean: &BooleanField{}},
			expected: `field "f": metadata fields don't need to be declared`,
		},
		{
			name:     "float",
			field:    IndexSchemaField{Float: &FloatField{}},
			expected: `field "f": metadata fields don't need to be declared`,
		},
		{
			name:     "integer",
			field:    IndexSchemaField{Integer: &IntegerField{}},
			expected: `field "f": metadata fields don't need to be declared`,
		},
		{
			name:     "legacy metadata",
			field:    IndexSchemaField{LegacyMetadata: &LegacyMetadataField{Filterable: true}},
			expected: `field "f": metadata fields don't need to be declared`,
		},
		{
			name:     "no field type set",
			field:    IndexSchemaField{},
			expected: `field "f": exactly one field type must be set on IndexSchemaField`,
		},
		{
			name: "more than one field type set",
			field: IndexSchemaField{
				DenseVector:  &DenseVectorField{Dimension: 3, Metric: IndexMetricCosine},
				SparseVector: &SparseVectorField{},
			},
			expected: `field "f": exactly one field type must be set on IndexSchemaField`,
		},
	}

	for _, tt := range errorTests {
		t.Run("error: "+tt.name, func(t *testing.T) {
			_, err := toDbCreateIndexSchema(IndexSchema{Fields: map[string]IndexSchemaField{"f": tt.field}})
			require.ErrorContains(t, err, tt.expected)
		})
	}
}

func TestClassicVectorSchemaUnit(t *testing.T) {
	tests := []struct {
		name       string
		vectorType string
		dimension  *int32
		metric     *IndexMetric
		expected   string
	}{
		{
			name:       "dense defaults to cosine",
			vectorType: "dense",
			dimension:  ptr(int32(1536)),
			expected:   `{"fields": {"_values": {"type": "dense_vector", "dimension": 1536, "metric": "cosine"}}}`,
		},
		{
			name:       "dense with explicit metric",
			vectorType: "dense",
			dimension:  ptr(int32(3)),
			metric:     ptr(IndexMetricDotproduct),
			expected:   `{"fields": {"_values": {"type": "dense_vector", "dimension": 3, "metric": "dotproduct"}}}`,
		},
		{
			name:       "sparse ignores dimension and metric",
			vectorType: "sparse",
			dimension:  ptr(int32(3)),
			metric:     ptr(IndexMetricDotproduct),
			expected:   `{"fields": {"_sparse_values": {"type": "sparse_vector"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := classicVectorSchema(tt.vectorType, tt.dimension, tt.metric)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, marshalJSON(t, result))
		})
	}

	t.Run("dense requires a dimension", func(t *testing.T) {
		_, err := classicVectorSchema("dense", nil, nil)
		require.ErrorContains(t, err, "dimension is required for dense indexes")
	})

	t.Run("matches the equivalent explicit schema", func(t *testing.T) {
		classicDense, err := classicVectorSchema("dense", ptr(int32(1536)), ptr(IndexMetricEuclidean))
		require.NoError(t, err)
		explicitDense, err := toDbCreateIndexSchema(IndexSchema{Fields: map[string]IndexSchemaField{
			reservedDenseFieldName: {DenseVector: &DenseVectorField{Dimension: 1536, Metric: IndexMetricEuclidean}},
		}})
		require.NoError(t, err)
		assert.JSONEq(t, marshalJSON(t, explicitDense), marshalJSON(t, classicDense))

		classicSparse, err := classicVectorSchema("sparse", nil, nil)
		require.NoError(t, err)
		explicitSparse, err := toDbCreateIndexSchema(IndexSchema{Fields: map[string]IndexSchemaField{
			reservedSparseFieldName: {SparseVector: &SparseVectorField{}},
		}})
		require.NoError(t, err)
		assert.JSONEq(t, marshalJSON(t, explicitSparse), marshalJSON(t, classicSparse))
	})
}

func TestToDbDeploymentRequestUnit(t *testing.T) {
	t.Run("nil deployment", func(t *testing.T) {
		result, err := toDbDeploymentRequest(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("managed", func(t *testing.T) {
		result, err := toDbDeploymentRequest(&IndexDeployment{Managed: &ManagedDeployment{Cloud: CloudAWS, Region: "us-east-1"}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"deployment_type": "managed", "cloud": "aws", "region": "us-east-1"}`, marshalJSON(t, result))
	})

	t.Run("managed Environment is not sent", func(t *testing.T) {
		result, err := toDbDeploymentRequest(&IndexDeployment{Managed: &ManagedDeployment{
			Cloud:       CloudGCP,
			Region:      "us-central1",
			Environment: ptr("us-central1-gcp"),
		}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"deployment_type": "managed", "cloud": "gcp", "region": "us-central1"}`, marshalJSON(t, result))
	})

	t.Run("byoc", func(t *testing.T) {
		result, err := toDbDeploymentRequest(&IndexDeployment{Byoc: &ByocDeployment{Environment: "aws-us-east-1-b921"}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"deployment_type": "byoc", "environment": "aws-us-east-1-b921"}`, marshalJSON(t, result))
	})

	t.Run("pod is rejected", func(t *testing.T) {
		_, err := toDbDeploymentRequest(&IndexDeployment{Pod: &PodDeployment{Environment: "us-east1-gcp", PodType: "p1.x1"}})
		require.ErrorContains(t, err, "creating pod-based indexes is not supported by Pinecone API version 2026-07")
	})

	t.Run("no deployment type set", func(t *testing.T) {
		_, err := toDbDeploymentRequest(&IndexDeployment{})
		require.ErrorContains(t, err, "exactly one of Managed, Pod, or Byoc must be set on IndexDeployment")
	})

	t.Run("more than one deployment type set", func(t *testing.T) {
		_, err := toDbDeploymentRequest(&IndexDeployment{
			Managed: &ManagedDeployment{Cloud: CloudAWS, Region: "us-east-1"},
			Byoc:    &ByocDeployment{Environment: "aws-us-east-1-b921"},
		})
		require.ErrorContains(t, err, "exactly one of Managed, Pod, or Byoc must be set on IndexDeployment")
	})
}

func TestToIndexDeploymentUnit(t *testing.T) {
	tests := []struct {
		name       string
		deployment string
		expected   *IndexDeployment
	}{
		{
			name:       "managed",
			deployment: `{"deployment_type": "managed", "cloud": "aws", "region": "us-east-1", "environment": "us-east-1-aws"}`,
			expected: &IndexDeployment{Managed: &ManagedDeployment{
				Cloud:       CloudAWS,
				Region:      "us-east-1",
				Environment: ptr("us-east-1-aws"),
			}},
		},
		{
			name:       "pod",
			deployment: `{"deployment_type": "pod", "environment": "us-east1-gcp", "pod_type": "p1.x2", "replicas": 2, "shards": 3}`,
			expected: &IndexDeployment{Pod: &PodDeployment{
				Environment: "us-east1-gcp",
				PodType:     "p1.x2",
				Replicas:    ptr(int32(2)),
				Shards:      ptr(int32(3)),
			}},
		},
		{
			name:       "byoc",
			deployment: `{"deployment_type": "byoc", "environment": "aws-us-east-1-b921"}`,
			expected:   &IndexDeployment{Byoc: &ByocDeployment{Environment: "aws-us-east-1-b921"}},
		},
		{
			name:       "unknown deployment type",
			deployment: `{"deployment_type": "edge", "environment": "somewhere"}`,
			expected:   nil,
		},
		{
			name:       "malformed deployment is skipped",
			deployment: `{"deployment_type": "managed", "cloud": 123, "region": "us-east-1"}`,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deployment db_control.IndexDeployment
			require.NoError(t, json.Unmarshal([]byte(tt.deployment), &deployment))

			assert.Equal(t, tt.expected, toIndexDeployment(deployment))
		})
	}
}

func TestApplyIndexCompatFieldsUnit(t *testing.T) {
	dense := func(dimension int32, metric IndexMetric) IndexSchemaField {
		return IndexSchemaField{DenseVector: &DenseVectorField{Dimension: dimension, Metric: metric}}
	}
	sparse := IndexSchemaField{SparseVector: &SparseVectorField{}}
	fullTextSearch := IndexSchemaField{String: &StringField{FullTextSearch: &FullTextSearchConfig{Language: ptr("en")}}}

	vectorTests := []struct {
		name               string
		fields             map[string]IndexSchemaField
		expectedVectorType string
		expectedDimension  *int32
		expectedMetric     IndexMetric
	}{
		{
			name:               "dense vector index reports both reserved fields",
			fields:             map[string]IndexSchemaField{"_values": dense(1536, IndexMetricCosine), "_sparse_values": sparse},
			expectedVectorType: "dense",
			expectedDimension:  ptr(int32(1536)),
			expectedMetric:     IndexMetricCosine,
		},
		{
			name:               "sparse-only index",
			fields:             map[string]IndexSchemaField{"_sparse_values": sparse},
			expectedVectorType: "sparse",
			expectedMetric:     IndexMetricDotproduct,
		},
		{
			name:               "single named dense field",
			fields:             map[string]IndexSchemaField{"embedding": dense(768, IndexMetricEuclidean), "title": fullTextSearch},
			expectedVectorType: "dense",
			expectedDimension:  ptr(int32(768)),
			expectedMetric:     IndexMetricEuclidean,
		},
		{
			name:               "reserved dense field wins over named dense fields",
			fields:             map[string]IndexSchemaField{"_values": dense(3, IndexMetricDotproduct), "other": dense(8, IndexMetricCosine)},
			expectedVectorType: "dense",
			expectedDimension:  ptr(int32(3)),
			expectedMetric:     IndexMetricDotproduct,
		},
		{
			name:   "several named dense fields leave the fields unset",
			fields: map[string]IndexSchemaField{"a": dense(3, IndexMetricCosine), "b": dense(8, IndexMetricCosine)},
		},
		{
			name:   "full-text-search index with no vector field",
			fields: map[string]IndexSchemaField{"title": fullTextSearch},
		},
	}

	for _, tt := range vectorTests {
		t.Run(tt.name, func(t *testing.T) {
			index := &Index{Schema: &IndexSchema{Fields: tt.fields}}
			applyIndexCompatFields(index)

			assert.Equal(t, tt.expectedVectorType, index.VectorType)
			assert.Equal(t, tt.expectedDimension, index.Dimension)
			assert.Equal(t, tt.expectedMetric, index.Metric)
			assert.Nil(t, index.Embed)
		})
	}

	t.Run("semantic text field populates Embed", func(t *testing.T) {
		index := &Index{Schema: &IndexSchema{Fields: map[string]IndexSchemaField{
			"chunk_text": {SemanticText: &SemanticTextField{
				Model:          "multilingual-e5-large",
				Dimension:      ptr(int32(1024)),
				Metric:         ptr(IndexMetricCosine),
				ReadParameters: &map[string]interface{}{"input_type": "query"},
			}},
		}}}
		applyIndexCompatFields(index)

		require.NotNil(t, index.Embed)
		assert.Equal(t, &IndexEmbed{
			Model:          "multilingual-e5-large",
			Dimension:      ptr(int32(1024)),
			Metric:         ptr(IndexMetricCosine),
			FieldMap:       &map[string]interface{}{"text": "chunk_text"},
			ReadParameters: &map[string]interface{}{"input_type": "query"},
		}, index.Embed)
		assert.Equal(t, "dense", index.VectorType)
		assert.Equal(t, ptr(int32(1024)), index.Dimension)
		assert.Equal(t, IndexMetricCosine, index.Metric)
	})

	t.Run("semantic text field without a dimension is sparse", func(t *testing.T) {
		index := &Index{Schema: &IndexSchema{Fields: map[string]IndexSchemaField{
			"chunk_text": {SemanticText: &SemanticTextField{Model: "pinecone-sparse-english-v0", Metric: ptr(IndexMetricDotproduct)}},
		}}}
		applyIndexCompatFields(index)

		require.NotNil(t, index.Embed)
		assert.Equal(t, "sparse", index.VectorType)
		assert.Nil(t, index.Dimension)
		assert.Equal(t, IndexMetricDotproduct, index.Metric)
	})

	t.Run("nil schema and deployment leave everything unset", func(t *testing.T) {
		index := &Index{}
		applyIndexCompatFields(index)
		assert.Equal(t, &Index{}, index)
	})

	sourceCollection := ptr("movie-embeddings")
	readCapacity := &ReadCapacity{}

	specTests := []struct {
		name       string
		deployment *IndexDeployment
		expected   *IndexSpec
	}{
		{
			name:       "managed deployment becomes a serverless spec",
			deployment: &IndexDeployment{Managed: &ManagedDeployment{Cloud: CloudAWS, Region: "us-east-1"}},
			expected: &IndexSpec{Serverless: &ServerlessSpec{
				Cloud:            CloudAWS,
				Region:           "us-east-1",
				SourceCollection: sourceCollection,
				ReadCapacity:     readCapacity,
			}},
		},
		{
			name: "pod deployment becomes a pod spec",
			deployment: &IndexDeployment{Pod: &PodDeployment{
				Environment: "us-east1-gcp",
				PodType:     "p1.x2",
				Replicas:    ptr(int32(2)),
				Shards:      ptr(int32(3)),
			}},
			expected: &IndexSpec{Pod: &PodSpec{
				Environment:      "us-east1-gcp",
				PodType:          "p1.x2",
				PodCount:         6,
				Replicas:         2,
				ShardCount:       3,
				SourceCollection: sourceCollection,
			}},
		},
		{
			name:       "pod replicas and shards default to 1",
			deployment: &IndexDeployment{Pod: &PodDeployment{Environment: "us-east1-gcp", PodType: "s1.x1"}},
			expected: &IndexSpec{Pod: &PodSpec{
				Environment:      "us-east1-gcp",
				PodType:          "s1.x1",
				PodCount:         1,
				Replicas:         1,
				ShardCount:       1,
				SourceCollection: sourceCollection,
			}},
		},
		{
			name:       "byoc deployment becomes a BYOC spec",
			deployment: &IndexDeployment{Byoc: &ByocDeployment{Environment: "aws-us-east-1-b921"}},
			expected: &IndexSpec{BYOC: &BYOCSpec{
				Environment:  "aws-us-east-1-b921",
				ReadCapacity: readCapacity,
			}},
		},
	}

	for _, tt := range specTests {
		t.Run(tt.name, func(t *testing.T) {
			index := &Index{Deployment: tt.deployment, SourceCollection: sourceCollection, ReadCapacity: readCapacity}
			applyIndexCompatFields(index)
			assert.Equal(t, tt.expected, index.Spec)
		})
	}
}

func marshalJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
