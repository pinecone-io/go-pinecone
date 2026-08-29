package pinecone

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// [IndexMetric] is the [similarity metric] to be used by similarity search against a Pinecone [Index].
//
// [similarity metric]: https://docs.pinecone.io/guides/indexes/understanding-indexes#similarity-metrics
type IndexMetric string

const (
	Cosine     IndexMetric = "cosine"     // Default distance metric, ideal for textual data
	Dotproduct IndexMetric = "dotproduct" // Ideal for hybrid search
	Euclidean  IndexMetric = "euclidean"  // Ideal for distance-based data (e.g. lat/long points)
)

// [IndexStatusState] is the state of a Pinecone [Index].
type IndexStatusState string

const (
	InitializationFailed IndexStatusState = "InitializationFailed"
	Initializing         IndexStatusState = "Initializing"
	Ready                IndexStatusState = "Ready"
	ScalingDown          IndexStatusState = "ScalingDown"
	ScalingDownPodSize   IndexStatusState = "ScalingDownPodSize"
	ScalingUp            IndexStatusState = "ScalingUp"
	ScalingUpPodSize     IndexStatusState = "ScalingUpPodSize"
	Terminating          IndexStatusState = "Terminating"
)

// [DeletionProtection] determines whether [deletion protection] is "enabled" or "disabled" for the [Index].
// When "enabled", the [Index] cannot be deleted. Defaults to "disabled".
//
// [deletion protection]: http://docs.pinecone.io/guides/indexes/prevent-index-deletion
type DeletionProtection string

const (
	DeletionProtectionEnabled  DeletionProtection = "enabled"
	DeletionProtectionDisabled DeletionProtection = "disabled"
)

// [Cloud] is the [cloud provider] to be used for a Pinecone serverless [Index].
//
// [cloud provider]: https://docs.pinecone.io/troubleshooting/available-cloud-regions
type Cloud string

const (
	Aws   Cloud = "aws"
	Azure Cloud = "azure"
	Gcp   Cloud = "gcp"
)

// [IndexStatus] is the status of a Pinecone [Index].
type IndexStatus struct {
	Ready bool             `json:"ready"`
	State IndexStatusState `json:"state"`
}

// [IndexSpec] is the infrastructure specification (serverless, pod-based, or BYOC) of a Pinecone [Index].
// Only one of the following fields will be present: Pod, Serverless, BYOC.
type IndexSpec struct {
	Pod        *PodSpec        `json:"pod,omitempty"`
	Serverless *ServerlessSpec `json:"serverless,omitempty"`
	BYOC       *BYOCSpec       `json:"byoc,omitempty"`
}

// [IndexEmbed] represents the embedding model configured for an index,
// including document fields mapped to embedding inputs.
//
// Fields:
//   - Model: The name of the embedding model used to create the index (e.g., "multilingual-e5-large").
//   - Dimension: The dimension of the embedding model, specifying the size of the output vector.
//   - Metric: The distance metric used by the embedding model. If the 'vector_type' is 'sparse',
//     the metric must be 'dotproduct'. If the `vector_type` is `dense`, the metric
//     defaults to 'cosine'.
//   - VectorType: The index vector type associated with the model. If 'dense', the vector dimension must be specified.
//     If 'sparse', the vector dimension will be nil.
//   - FieldMap: Identifies the name of the text field from your document model that is embedded.
//   - ReadParameters: The read parameters for the embedding model.
//   - WriteParameters: The write parameters for the embedding model.
type IndexEmbed struct {
	Model           string                  `json:"model"`
	Dimension       *int32                  `json:"dimension,omitempty"`
	Metric          *IndexMetric            `json:"metric,omitempty"`
	VectorType      *string                 `json:"vector_type,omitempty"`
	FieldMap        *map[string]interface{} `json:"field_map,omitempty"`
	ReadParameters  *map[string]interface{} `json:"read_parameters,omitempty"`
	WriteParameters *map[string]interface{} `json:"write_parameters,omitempty"`
}

// [IndexTags] is a set of key-value pairs that can be attached to a Pinecone [Index].
type IndexTags map[string]string

// [IndexSchema] is the schema of a Pinecone [Index] under the 2026-07 API. The schema defines the typed
// fields that documents in the index can contain, including vector fields, semantic text fields, and
// metadata fields.
//
// Indexes served by the vectors API (created by earlier API versions from dimension/metric/vector_type,
// or created with the reserved "_values"/"_sparse_values" schema) report their vector fields under those
// reserved names, and any metadata fields configured for filtering as [LegacyMetadataField] entries.
type IndexSchema struct {
	Fields map[string]IndexSchemaField `json:"fields"`
}

// [IndexSchemaField] is the configuration of a single field in an [IndexSchema]. Exactly one of the
// pointer fields is non-nil, identifying the field's type.
type IndexSchemaField struct {
	DenseVector    *DenseVectorField    `json:"dense_vector,omitempty"`
	SparseVector   *SparseVectorField   `json:"sparse_vector,omitempty"`
	SemanticText   *SemanticTextField   `json:"semantic_text,omitempty"`
	String         *StringField         `json:"string,omitempty"`
	StringList     *StringListField     `json:"string_list,omitempty"`
	Boolean        *BooleanField        `json:"boolean,omitempty"`
	Float          *FloatField          `json:"float,omitempty"`
	Integer        *IntegerField        `json:"integer,omitempty"`
	LegacyMetadata *LegacyMetadataField `json:"legacy_metadata,omitempty"`
}

// [DenseVectorField] is a dense vector field configuration. Stores fixed-dimension floating-point
// vectors for approximate nearest-neighbor (ANN) search.
type DenseVectorField struct {
	Dimension   int32       `json:"dimension"`
	Metric      IndexMetric `json:"metric"`
	Description *string     `json:"description,omitempty"`
}

// [SparseVectorField] is a sparse vector field configuration. Sparse fields take no dimension and no
// metric; sparse scoring is not configurable.
type SparseVectorField struct {
	Description *string `json:"description,omitempty"`
}

// [SemanticTextField] is a semantic text field backed by an integrated embedding model, as returned
// when describing an index created with [Client.CreateIndexForModel]. It cannot be declared when
// creating an index directly.
type SemanticTextField struct {
	Model           string                  `json:"model"`
	Dimension       *int32                  `json:"dimension,omitempty"`
	Metric          *IndexMetric            `json:"metric,omitempty"`
	ReadParameters  *map[string]interface{} `json:"read_parameters,omitempty"`
	WriteParameters *map[string]interface{} `json:"write_parameters,omitempty"`
	Description     *string                 `json:"description,omitempty"`
}

// [StringField] is a string field configuration. When FullTextSearch is present the field is indexed
// for full-text search; Filterable is reported on responses when the field is indexed for metadata
// filtering.
type StringField struct {
	FullTextSearch *FullTextSearchConfig `json:"full_text_search,omitempty"`
	Filterable     *bool                 `json:"filterable,omitempty"`
	Description    *string               `json:"description,omitempty"`
}

// [FullTextSearchConfig] configures full-text search on a [StringField]. StopWords requires
// Stemming; Ngram cannot be combined with Stemming or StopWords.
type FullTextSearchConfig struct {
	Language  *string      `json:"language,omitempty"`
	Stemming  *bool        `json:"stemming,omitempty"`
	StopWords *bool        `json:"stop_words,omitempty"`
	Ngram     *NgramConfig `json:"ngram,omitempty"`
}

// [NgramConfig] configures character n-gram tokenization for substring matching on a full-text-search
// [StringField].
type NgramConfig struct {
	MinGram    int   `json:"min_gram"`
	MaxGram    int   `json:"max_gram"`
	PrefixOnly *bool `json:"prefix_only,omitempty"`
}

// [StringListField] is a string array field configuration reported in index schemas. String array
// values are indexed automatically at upsert time and cannot be declared at index creation.
type StringListField struct {
	Filterable  *bool   `json:"filterable,omitempty"`
	Description *string `json:"description,omitempty"`
}

// [BooleanField] is a boolean field configuration reported in index schemas. Boolean values are
// indexed automatically at upsert time and cannot be declared at index creation.
type BooleanField struct {
	Filterable  *bool   `json:"filterable,omitempty"`
	Description *string `json:"description,omitempty"`
}

// [FloatField] is a numeric (floating-point) field configuration reported in index schemas. Numeric
// values are indexed automatically at upsert time and cannot be declared at index creation.
type FloatField struct {
	Filterable  *bool   `json:"filterable,omitempty"`
	Description *string `json:"description,omitempty"`
}

// [IntegerField] is an integer field configuration reported in index schemas. Integer values are
// indexed automatically at upsert time and cannot be declared at index creation.
type IntegerField struct {
	Filterable  *bool   `json:"filterable,omitempty"`
	Description *string `json:"description,omitempty"`
}

// [LegacyMetadataField] is a metadata field from an index that pre-dates typed schemas, carrying only
// whether the field is indexed for filtering.
type LegacyMetadataField struct {
	Filterable bool `json:"filterable"`
}

// [IndexDeployment] is the deployment configuration of a Pinecone [Index] under the 2026-07 API.
// Exactly one of the pointer fields is non-nil, identifying the deployment type.
type IndexDeployment struct {
	Managed *ManagedDeployment `json:"managed,omitempty"`
	Pod     *PodDeployment     `json:"pod,omitempty"`
	Byoc    *ByocDeployment    `json:"byoc,omitempty"`
}

// [ManagedDeployment] is the deployment configuration for a serverless (managed) index. Environment
// is returned in responses and must not be set when creating an index.
type ManagedDeployment struct {
	Cloud       Cloud   `json:"cloud"`
	Region      string  `json:"region"`
	Environment *string `json:"environment,omitempty"`
}

// [PodDeployment] is the deployment configuration of a pod-based index.
type PodDeployment struct {
	Environment string `json:"environment"`
	PodType     string `json:"pod_type"`
	Replicas    *int32 `json:"replicas,omitempty"`
	Shards      *int32 `json:"shards,omitempty"`
}

// [ByocDeployment] is the deployment configuration of a BYOC (bring-your-own-cloud) index.
type ByocDeployment struct {
	Environment string `json:"environment"`
}

// [Index] is a Pinecone [Index] object.
//
// Under the 2026-07 API every index carries a persisted [IndexSchema] and an [IndexDeployment].
// The Metric, VectorType, Dimension, Spec, and Embed fields are computed from Schema and Deployment
// for backward compatibility and are deprecated; new code should read Schema, Deployment, and
// ReadCapacity directly.
//
// Fields:
//   - Name: The name of the index.
//   - Host: The URL address where the index is hosted.
//   - Schema: The [IndexSchema] of the index, defining its typed fields.
//   - Deployment: The [IndexDeployment] of the index (managed, pod, or byoc).
//   - ReadCapacity: The [ReadCapacity] configuration of the index, if any.
//   - DeletionProtection: Whether deletion protection is configured for the index. Can be 'enabled' or 'disabled'.
//   - PrivateHost: The private endpoint URL of an index.
//   - SourceCollection: The name of the collection this index was created from, if any.
//   - SourceBackupId: The ID of the backup this index was restored from, if any.
//   - CmekId: The ID of the customer-managed encryption key (CMEK) used to encrypt this index, if any.
//   - Status: The [IndexStatus] of the index, which includes index state information.
//   - Tags: Custom [IndexTags] added to an index.
//   - Metric: Deprecated: computed from Schema. The distance metric of the index's vector field.
//   - VectorType: Deprecated: computed from Schema. One of 'sparse' or 'dense'.
//   - Dimension: Deprecated: computed from Schema. The dimension of the index's dense vector field.
//   - Spec: Deprecated: computed from Deployment. Contains [PodSpec], [ServerlessSpec], or [BYOCSpec].
//   - Embed: Deprecated: computed from the Schema's [SemanticTextField]. The [IndexEmbed] model configured for the index, if applicable.
type Index struct {
	Name               string             `json:"name"`
	Host               string             `json:"host"`
	Schema             *IndexSchema       `json:"schema,omitempty"`
	Deployment         *IndexDeployment   `json:"deployment,omitempty"`
	ReadCapacity       *ReadCapacity      `json:"read_capacity,omitempty"`
	DeletionProtection DeletionProtection `json:"deletion_protection,omitempty"`
	PrivateHost        *string            `json:"private_host,omitempty"`
	SourceCollection   *string            `json:"source_collection,omitempty"`
	SourceBackupId     *string            `json:"source_backup_id,omitempty"`
	CmekId             *string            `json:"cmek_id,omitempty"`
	Status             *IndexStatus       `json:"status,omitempty"`
	Tags               *IndexTags         `json:"tags,omitempty"`

	// Deprecated: computed from Schema for backward compatibility; read Schema directly in new code.
	Metric IndexMetric `json:"metric"`
	// Deprecated: computed from Schema for backward compatibility; read Schema directly in new code.
	VectorType string `json:"vector_type"`
	// Deprecated: computed from Schema for backward compatibility; read Schema directly in new code.
	Dimension *int32 `json:"dimension,omitempty"`
	// Deprecated: computed from Deployment for backward compatibility; read Deployment directly in new code.
	Spec *IndexSpec `json:"spec,omitempty"`
	// Deprecated: computed from the Schema's [SemanticTextField] for backward compatibility.
	Embed *IndexEmbed `json:"embed,omitempty"`
}

// [Collection] is a Pinecone [collection entity]. Only available for pod-based Indexes.
//
// Fields:
//   - Name: The name of the collection.
//   - Size: The total size of the collection in bytes.
//   - Status: The [CollectionStatus] of the collection.
//   - Dimension: The dimensionality of the vectors for each record stored in the collection.
//   - VectorCount: The number of records (vectors) stored in the collection.
//   - Environment: The environment where the collection is hosted.
//
// [collection entity]: https://docs.pinecone.io/guides/indexes/understanding-collections
type Collection struct {
	Name        string           `json:"name"`
	Size        int64            `json:"size"`
	Status      CollectionStatus `json:"status"`
	Dimension   int32            `json:"dimension"`
	VectorCount int32            `json:"vector_count"`
	Environment string           `json:"environment"`
}

// [CollectionStatus] is the status of a Pinecone [Collection].
type CollectionStatus string

const (
	CollectionStatusInitializing CollectionStatus = "Initializing"
	CollectionStatusReady        CollectionStatus = "Ready"
	CollectionStatusTerminating  CollectionStatus = "Terminating"
	CollectionStatusTerminated   CollectionStatus = "Terminated"
)

// [PodSpecMetadataConfig] represents the metadata fields to be indexed when a Pinecone [Index] is created.
type PodSpecMetadataConfig struct {
	Indexed *[]string `json:"indexed,omitempty"`
}

// [PodSpec] is the infrastructure specification of a pod-based Pinecone [Index]. Only available for pod-based Indexes.
//
// Fields:
//   - Environment: The environment where the index is hosted.
//   - PodType: The pod type used for the index. Must be one of "s1", "p1", or "p2"
//     followed by ".x1", ".x2", ".x4", or ".x8".
//   - PodCount: The number of pods used for the index. Should equal `shards` × `replicas`.
//   - Replicas: The number of replicas. Replicas duplicate the index. They provide higher availability and throughput.
//   - ShardCount: The number of shards. Shards split your data across multiple pods so you can fit more data into an index.
//   - SourceCollection: The name of the [Collection] used as a source for the index.
//   - MetadataConfig: Configuration for the behavior of Pinecone's internal metadata index. By default, all metadata is indexed;
//     when `metadata_config` is present, only specified metadata fields are indexed.
type PodSpec struct {
	Environment      string                 `json:"environment"`
	PodType          string                 `json:"pod_type"`
	PodCount         int                    `json:"pod_count"`
	Replicas         int32                  `json:"replicas"`
	ShardCount       int32                  `json:"shard_count"`
	SourceCollection *string                `json:"source_collection,omitempty"`
	MetadataConfig   *PodSpecMetadataConfig `json:"metadata_config,omitempty"`
}

// [ServerlessSpec] is the infrastructure specification of a serverless Pinecone [Index]. Only available for serverless Indexes.
//
// Fields:
//   - Cloud: The public cloud provider where the index is hosted.
//   - Region: The region where the index is hosted.
//   - Schema: (Optional) Schema for the behavior of Pinecone's internal metadata index. By default, all metadata is indexed.
//   - SourceCollection: (Optional) The name of the [Collection] used as a source for the index.
//   - ReadCapacity: (Optional) The read capacity configuration for the serverless index. Used to configure dedicated read capacity
//     with specific node types and scaling strategies.
type ServerlessSpec struct {
	Cloud            Cloud           `json:"cloud"`
	Region           string          `json:"region"`
	Schema           *MetadataSchema `json:"schema,omitempty"`
	SourceCollection *string         `json:"source_collection,omitempty"`
	ReadCapacity     *ReadCapacity   `json:"read_capacity,omitempty"`
}

// [BYOCSpec] is the infrastructure specification of a BYOC Pinecone [Index].
//
// Fields:
//   - Environment: The environment where the index is hosted.
//   - Schema: Schema for the behavior of Pinecone's internal metadata index.
//   - ReadCapacity: (Optional) The read capacity configuration for the serverless index. Used to configure dedicated read capacity
//     with specific node types and scaling strategies.
type BYOCSpec struct {
	Environment  string          `json:"environment"`
	Schema       *MetadataSchema `json:"schema,omitempty"`
	ReadCapacity *ReadCapacity   `json:"read_capacity,omitempty"`
}

// [ReadCapacity] represents the read capacity configuration returned from the API.
// [ReadCapacity] is a tagged union which can have either [ReadCapacityOnDemand] or [ReadCapacityDedicated].
//
// Fields:
//   - OnDemand: OnDemand read capacity mode with current status.
//   - Dedicated: Dedicated read capacity mode with current status.
type ReadCapacity struct {
	OnDemand  *ReadCapacityOnDemand  `json:"on_demand,omitempty"`
	Dedicated *ReadCapacityDedicated `json:"dedicated,omitempty"`
}

// [ReadCapacityOnDemand] represents OnDemand read capacity mode with status information.
//
// Fields:
//   - Status: The current status of the read capacity configuration.
type ReadCapacityOnDemand struct {
	Status ReadCapacityStatus `json:"status"`
}

// [ReadCapacityDedicated] represents Dedicated read capacity configuration with status information.
//
// Fields:
//   - NodeType: The type of machines in use.
//   - Scaling: The scaling strategy configuration.
//   - Status: The current status of the read capacity configuration.
type ReadCapacityDedicated struct {
	NodeType *string              `json:"node_type"`
	Scaling  *ReadCapacityScaling `json:"scaling,omitempty"`
	Status   ReadCapacityStatus   `json:"status"`
}

// [ReadCapacityScaling] represents the scaling configuration for dedicated read capacity.
//
// Fields:
//   - Manual: Manual scaling configuration with fixed replicas and shards.
type ReadCapacityScaling struct {
	Manual *ReadCapacityManualScaling `json:"manual,omitempty"`
}

// [ReadCapacityManualScaling] represents manual scaling configuration.
//
// Fields:
//   - Replicas: The number of replicas to use. Replicas duplicate the compute resources
//     and data of an index, allowing higher query throughput and availability.
//     Setting replicas to 0 disables the index but can be used to reduce costs while usage is paused.
//   - Shards: The number of shards to use. Shards determine the storage capacity of an index,
//     with each shard providing 250 GB of storage.
type ReadCapacityManualScaling struct {
	Replicas *int32 `json:"replicas"`
	Shards   *int32 `json:"shards"`
}

// [ReadCapacityStatus] represents the current status of factors affecting the read capacity of a serverless index.
//
// Fields:
//   - State: The overall status state. Available values: "Ready", "Scaling", "Migrating", or "Error".
//   - CurrentReplicas: The current number of replicas.
//   - CurrentShards: The current number of shards.
//   - ErrorMessage: An optional error message if there are issues with the read capacity configuration.
type ReadCapacityStatus struct {
	State           string  `json:"state"`
	CurrentReplicas *int32  `json:"current_replicas,omitempty"`
	CurrentShards   *int32  `json:"current_shards,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
}

// [Vector] is a [dense or sparse vector object] with optional metadata.
//
// [dense or sparse vector object]: https://docs.pinecone.io/guides/get-started/key-concepts#dense-vector
type Vector struct {
	Id           string        `json:"id"`
	Values       *[]float32    `json:"values,omitempty"`
	SparseValues *SparseValues `json:"sparse_values,omitempty"`
	Metadata     *Metadata     `json:"metadata,omitempty"`
}

// [ScoredVector] is a vector with an associated similarity score calculated according to the distance metric of the
// [Index].
type ScoredVector struct {
	Vector *Vector `json:"vector,omitempty"`
	Score  float32 `json:"score"`
}

// [SparseValues] is a sparse vector object, most commonly used for [hybrid search].
//
// [hybrid search]: https://docs.pinecone.io/guides/data/understanding-hybrid-search#hybrid-search-in-pinecone
type SparseValues struct {
	Indices []uint32  `json:"indices,omitempty"`
	Values  []float32 `json:"values,omitempty"`
}

// [NamespaceSummary] is a summary of stats for a Pinecone [namespace].
//
// [namespace]: https://docs.pinecone.io/guides/indexes/use-namespaces
// Fields:
//   - VectorCount: The number of vectors in the namespace.
type NamespaceSummary struct {
	VectorCount uint32 `json:"vector_count"`
}

// [NamespaceDescription] is a description of a Pinecone [namespace].
//
// [namespace]: https://docs.pinecone.io/guides/indexes/use-namespaces
// Fields:
//   - Name: The name of the namespace.
//   - RecordCount: The number of records in the namespace.
//   - TotalCount: The total number of namespaces in the index matching the prefix
//   - IndexedFields: A list of all indexed metadata fields in the namespace
//   - Schema: Schema for the behavior of Pinecone's internal metadata index.
type NamespaceDescription struct {
	Name          string          `json:"name"`
	RecordCount   uint64          `json:"record_count"`
	IndexedFields *IndexedFields  `json:"indexed_fields,omitempty"`
	Schema        *MetadataSchema `json:"schema,omitempty"`
}

// [IndexedFields] is a list of all indexed metadata fields in the namespace
// Fields:
//   - Fields: A list of all indexed metadata fields in the namespace
type IndexedFields struct {
	Fields []string `json:"fields,omitempty"`
}

// [Usage] is the usage stats ([Read Units]) for a Pinecone [Index].
//
// [Read Units]: https://docs.pinecone.io/guides/organizations/manage-cost/understanding-cost#serverless-indexes
type Usage struct {
	ReadUnits uint32 `json:"read_units"`
}

// [RerankUsage] is the usage stats ([Rerank Units]) for a reranking request.
//
// [Rerank Units]: https://docs.pinecone.io/guides/organizations/manage-cost/understanding-cost#rerank
type RerankUsage struct {
	RerankUnits *int `json:"rerank_units,omitempty"`
}

// [MetadataFilter] represents the [metadata filters] attached to a Pinecone request.
// These optional metadata filters are applied to query and deletion requests.
//
// [metadata filters]: https://docs.pinecone.io/guides/data/filter-with-metadata#querying-an-index-with-metadata-filters
type MetadataFilter = structpb.Struct

// [Metadata] represents optional,
// additional information that can be [attached to, or updated for, a vector] in a Pinecone Index.
//
// [attached to, or updated for, a vector]: https://docs.pinecone.io/guides/data/filter-with-metadata#inserting-metadata-into-an-index
type Metadata = structpb.Struct

// [NewMetadataFilter] creates a [MetadataFilter] from a map of key-value pairs representing metadata filter expressions.
// This helper function eliminates the need to import and use [structpb.Struct] directly.
//
// The input map should contain metadata filter expressions using Pinecone's filtering operators
// (e.g., $eq, $ne, $gt, $gte, $lt, $lte, $in, $nin, $exists, $and, $or).
//
// Example:
//
//	filterMap := map[string]interface{}{
//		"genre": map[string]interface{}{
//			"$eq": "documentary",
//		},
//		"year": map[string]interface{}{
//			"$gte": 2020,
//		},
//	}
//	filter, err := pinecone.NewMetadataFilter(filterMap)
//
// [MetadataFilter]: https://docs.pinecone.io/guides/data/filter-with-metadata#querying-an-index-with-metadata-filters
func NewMetadataFilter(m map[string]interface{}) (*MetadataFilter, error) {
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// [NewMetadata] creates [Metadata] from a map of key-value pairs representing metadata fields.
// This helper function eliminates the need to import and use [structpb.Struct] directly.
//
// The input map should contain flat key-value pairs where:
//   - Keys must be strings and must not start with a $
//   - Values must be one of: string, integer, floating point, boolean, or list of strings
//   - Nested JSON objects are not supported
//   - Null values are not supported (remove keys instead)
//
// Example:
//
//	metadataMap := map[string]interface{}{
//		"genre":        "classical",
//		"year":         2020,
//		"is_public":    true,
//		"tags":         []string{"beginner", "database"},
//	}
//	metadata, err := pinecone.NewMetadata(metadataMap)
//
// [Metadata]: https://docs.pinecone.io/guides/data/filter-with-metadata#inserting-metadata-into-an-index
func NewMetadata(m map[string]interface{}) (*Metadata, error) {
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// [Embedding] represents the embedding of a single input which is returned after [generating embeddings].
// [Embedding] is a tagged union which can have either a [SparseEmbedding] or a [DenseEmbedding].
//
// [generating embeddings]: https://docs.pinecone.io/guides/inference/generate-embeddings#3-generate-embeddings
// Fields:
//   - SparseEmbedding: The [SparseEmbedding] representation of the input.
//   - DenseEmbedding: The [DenseEmbedding] representation of the input.
type Embedding struct {
	SparseEmbedding *SparseEmbedding `json:"sparse_embedding,omitempty"`
	DenseEmbedding  *DenseEmbedding  `json:"dense_embedding,omitempty"`
}

// [DenseEmbedding] represents a dense numerical embedding of the input.
//
// Fields:
//   - VectorType: A string indicating the type of vector embedding ("dense").
//   - Values: A slice of float32 values representing the dense embedding.
type DenseEmbedding struct {
	VectorType string    `json:"vector_type"`
	Values     []float32 `json:"values"`
}

// [SparseEmbedding] represents a sparse embedding of the input, where only selected dimensions are populated.
//
// Fields:
//   - VectorType: A string indicating the type of vector embedding ("sparse").
//   - SparseValues: A slice of float32 values representing the sparse embedding value.
//   - SparseIndices: A slice of int64 values representing the embedding indices.
//   - SparseTokens: The normalized tokens used to create the sparse embedding, if requested.
type SparseEmbedding struct {
	VectorType    string    `json:"vector_type"`
	SparseValues  []float32 `json:"sparse_values"`
	SparseIndices []int64   `json:"sparse_indices"`
	SparseTokens  *[]string `json:"sparse_tokens,omitempty"`
}

// [Pagination] represents the pagination information for a list of resources.
type Pagination struct {
	Next string `json:"next"`
}

// [ImportStatus] represents the status of an [Import] operation.
//
// Values:
//   - Cancelled: The [Import] was canceled.
//   - Completed: The [Import] completed successfully.
//   - Failed: The [Import] encountered an error and did not complete successfully.
//   - InProgress: The [Import] is currently in progress.
//   - Pending: The [Import] is pending and has not yet started.
type ImportStatus string

const (
	Cancelled  ImportStatus = "Cancelled"
	Completed  ImportStatus = "Completed"
	Failed     ImportStatus = "Failed"
	InProgress ImportStatus = "InProgress"
	Pending    ImportStatus = "Pending"
)

// [ImportErrorMode] specifies how errors are handled during an [Import].
//
// Values:
//   - Abort: The [Import] process will abort upon encountering an error.
//   - Continue: The [Import] process will continue, skipping over records that produce errors.
type ImportErrorMode string

const (
	Abort    ImportErrorMode = "abort"
	Continue ImportErrorMode = "continue"
)

// [Import] represents the details and status of an import process.
//
// Fields:
//   - Id: The unique identifier of the [Import] process.
//   - PercentComplete: The percentage of the [Import] process that has been completed.
//   - RecordsImported: The total number of records successfully imported.
//   - Status: The current status of the [Import] (e.g., "InProgress", "Completed", "Failed").
//   - Uri: The URI of the source data for the [Import].
//   - CreatedAt: The time at which the [Import] process was initiated.
//   - FinishedAt: The time at which the [Import] process finished (either successfully or with an error).
//   - Error: If the [Import] failed, contains the error message associated with the failure.
type Import struct {
	Id              string       `json:"id,omitempty"`
	PercentComplete float32      `json:"percent_complete,omitempty"`
	RecordsImported int64        `json:"records_imported,omitempty"`
	Status          ImportStatus `json:"status,omitempty"`
	Uri             string       `json:"uri,omitempty"`
	CreatedAt       *time.Time   `json:"created_at,omitempty"`
	FinishedAt      *time.Time   `json:"finished_at,omitempty"`
	Error           *string      `json:"error,omitempty"`
}

type IntegratedRecord map[string]interface{}

// [DocumentUsage] reports the read units consumed by a documents API read operation.
type DocumentUsage struct {
	ReadUnits int32 `json:"read_units"`
}

// [UpsertDocumentsRequest] holds the parameters for [IndexConnection.UpsertDocuments].
//
// Fields:
//   - Documents: (Required) The documents to upsert. Each [Document] must carry an "_id" field and at
//     least one field declared in the index schema; other fields are stored as filterable metadata.
type UpsertDocumentsRequest struct {
	Documents []Document `json:"documents"`
}

// [UpsertDocumentsResponse] is returned by [IndexConnection.UpsertDocuments].
type UpsertDocumentsResponse struct {
	UpsertedCount int32 `json:"upserted_count"`
}

// [DocumentScoringMethod] defines how documents are scored against a query in
// [IndexConnection.SearchDocuments]. The Type field determines which other fields are used:
//   - "dense_vector": score by dense vector similarity. Requires Fields naming exactly one field, and Values.
//   - "sparse_vector": score by sparse vector similarity. Requires Fields naming exactly one field, and SparseValues.
//   - "text": score by BM25 text similarity. Requires Fields naming one or more fields, and Query.
//   - "query_string": score using a Lucene query string. Requires Query; Fields must be empty
//     (use field qualifiers inside the query string to target fields).
type DocumentScoringMethod struct {
	Type         string        `json:"type"`
	Fields       []string      `json:"fields,omitempty"`
	Query        *string       `json:"query,omitempty"`
	Values       *[]float32    `json:"values,omitempty"`
	SparseValues *SparseValues `json:"sparse_values,omitempty"`
}

// [SearchDocumentsRequest] holds the parameters for [IndexConnection.SearchDocuments].
//
// Fields:
//   - TopK: (Required) The number of top-ranked documents to return.
//   - ScoreBy: (Required) The scoring methods to rank documents by. A single method of any type is
//     valid; several methods may be combined only when every one is "text" or "query_string".
//   - Filter: (Optional) A metadata filter expression restricting the documents searched.
//   - IncludeFields: (Optional) The document fields to return on each match alongside "_id" and
//     "_score". When empty, no fields are returned; pass []string{"*"} to return every field.
type SearchDocumentsRequest struct {
	TopK          int32                   `json:"top_k"`
	ScoreBy       []DocumentScoringMethod `json:"score_by"`
	Filter        map[string]interface{}  `json:"filter,omitempty"`
	IncludeFields []string                `json:"include_fields,omitempty"`
}

// [DocumentMatch] is a document returned from [IndexConnection.SearchDocuments], including the
// document ID, similarity score, and any requested fields. Score is nil when the score is not a
// finite number.
type DocumentMatch struct {
	Id     string                 `json:"_id"`
	Score  *float32               `json:"_score"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler. The wire format carries the requested fields flattened
// alongside "_id" and "_score"; they are collected into Fields.
func (m *DocumentMatch) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if id, ok := raw["_id"]; ok {
		if err := json.Unmarshal(id, &m.Id); err != nil {
			return err
		}
		delete(raw, "_id")
	}
	if score, ok := raw["_score"]; ok {
		if err := json.Unmarshal(score, &m.Score); err != nil {
			return err
		}
		delete(raw, "_score")
	}
	if len(raw) > 0 {
		m.Fields = make(map[string]interface{}, len(raw))
		for key, value := range raw {
			var decoded interface{}
			if err := json.Unmarshal(value, &decoded); err != nil {
				return err
			}
			m.Fields[key] = decoded
		}
	}
	return nil
}

// [SearchDocumentsResponse] is returned by [IndexConnection.SearchDocuments].
type SearchDocumentsResponse struct {
	Matches   []DocumentMatch `json:"matches"`
	Namespace string          `json:"namespace"`
	Usage     DocumentUsage   `json:"usage"`
}

// [FetchDocumentsRequest] holds the parameters for [IndexConnection.FetchDocuments]. Exactly one of
// Ids or Filter must be provided.
//
// Fields:
//   - Ids: A list of document IDs to fetch. Mutually exclusive with Filter.
//   - Filter: A metadata filter expression selecting the documents to fetch. Must not be empty.
//     Mutually exclusive with Ids.
//   - IncludeFields: (Optional) The document fields to return. When empty, all fields are returned.
//   - Limit: (Optional) The maximum number of documents per page for a fetch by Filter. Defaults to 100.
//   - PaginationToken: (Optional) A token from a previous response to retrieve the next page. Only
//     valid together with Filter.
type FetchDocumentsRequest struct {
	Ids             []string               `json:"ids,omitempty"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeFields   []string               `json:"include_fields,omitempty"`
	Limit           *int32                 `json:"limit,omitempty"`
	PaginationToken *string                `json:"pagination_token,omitempty"`
}

// [FetchDocumentsResponse] is returned by [IndexConnection.FetchDocuments]. Each fetched [Document]
// carries its "_id" alongside its field values.
type FetchDocumentsResponse struct {
	Documents  map[string]Document `json:"documents"`
	Namespace  string              `json:"namespace"`
	Pagination *Pagination         `json:"pagination,omitempty"`
	Usage      DocumentUsage       `json:"usage"`
}

// [DeleteDocumentsRequest] holds the parameters for [IndexConnection.DeleteDocuments]. Exactly one of
// Ids, Filter, or DeleteAll must be provided.
//
// Fields:
//   - Ids: A list of document IDs to delete.
//   - Filter: A metadata filter expression selecting the documents to delete. Must not be empty; to
//     delete every document in the namespace, set DeleteAll.
//   - DeleteAll: If true, delete all documents in the namespace.
type DeleteDocumentsRequest struct {
	Ids       []string               `json:"ids,omitempty"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
	DeleteAll bool                   `json:"delete_all,omitempty"`
}

// [DeleteDocumentsResponse] is returned by [IndexConnection.DeleteDocuments]. MatchedRecords is the
// point-in-time number of documents that matched Filter when the delete was accepted; it is only
// returned for a filtered delete.
type DeleteDocumentsResponse struct {
	MatchedRecords *int32 `json:"matched_records,omitempty"`
}

// [UpdateDocumentsRequest] holds the parameters for [IndexConnection.UpdateDocuments]. Either
// Documents (per-document updates) or Filter with SetFields and/or RemoveFields (a filtered patch)
// must be provided; the two forms are mutually exclusive.
//
// Fields:
//   - Documents: Partial document updates. Each [Document] must carry "_id"; other entries set the
//     named fields, and an optional "_remove_fields" entry ([]string) deletes fields.
//   - Filter: A metadata filter expression selecting the documents to patch. Must not be empty.
//   - SetFields: The fields to set on every document matching Filter.
//   - RemoveFields: The names of the fields to remove from every document matching Filter.
type UpdateDocumentsRequest struct {
	Documents    []Document             `json:"documents,omitempty"`
	Filter       map[string]interface{} `json:"filter,omitempty"`
	SetFields    map[string]interface{} `json:"set_fields,omitempty"`
	RemoveFields []string               `json:"remove_fields,omitempty"`
}

// [UpdateDocumentsResponse] is returned by [IndexConnection.UpdateDocuments]. MatchedRecords is the
// point-in-time number of documents that matched Filter when the update was accepted; it is only
// returned for a filtered update.
type UpdateDocumentsResponse struct {
	MatchedRecords *int32 `json:"matched_records,omitempty"`
}

// [ListDocumentsRequest] holds the parameters for [IndexConnection.ListDocuments].
//
// Fields:
//   - Prefix: (Optional) A prefix to filter document IDs.
//   - Limit: (Optional) The maximum number of documents per page. Defaults to 100.
//   - PaginationToken: (Optional) A token from a previous response to retrieve the next page.
type ListDocumentsRequest struct {
	Prefix          *string `json:"prefix,omitempty"`
	Limit           *int32  `json:"limit,omitempty"`
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// [ListedDocument] identifies a document returned by [IndexConnection.ListDocuments].
type ListedDocument struct {
	Id string `json:"_id"`
}

// [ListDocumentsResponse] is returned by [IndexConnection.ListDocuments]. Documents are in sorted
// order by ID.
type ListDocumentsResponse struct {
	Documents  []ListedDocument `json:"documents"`
	Namespace  string           `json:"namespace"`
	Pagination *Pagination      `json:"pagination,omitempty"`
	Usage      DocumentUsage    `json:"usage"`
}

// [SearchRecordsRequest] represents a search request for records in a specific namespace.
//
// Fields:
//   - Query: The query inputs to search with.
//   - Fields: The fields to return in the search results.
//   - Rerank: Parameters for reranking the initial search results.
type SearchRecordsRequest struct {
	Query  SearchRecordsQuery   `json:"query"`
	Fields *[]string            `json:"fields,omitempty"`
	Rerank *SearchRecordsRerank `json:"rerank,omitempty"`
}

// [SearchRecordsQuery] represents the query parameters for searching records.
//
// Fields:
//   - TopK: The number of results to return for each search.
//   - Filter: The filter to apply.
//   - Id: The unique ID of the vector to be used as a query vector.
//   - Inputs: Additional input parameters for the query.
//   - Vector: The vector representation of the query.
type SearchRecordsQuery struct {
	TopK       int32                   `json:"top_k"`
	Filter     *map[string]interface{} `json:"filter,omitempty"`
	Id         *string                 `json:"id,omitempty"`
	Inputs     *map[string]interface{} `json:"inputs,omitempty"`
	Vector     *SearchRecordsVector    `json:"vector,omitempty"`
	MatchTerms *SearchMatchTerms       `json:"match_terms,omitempty"`
}

// [SearchMatchTerms] represents the terms to match in the text of each search hit.
//
// Fields:
//   - Strategy: The strategy for matching terms in the text. Currently, only `all` is supported, which means all specified terms must be present.
//     Leaving this empty will default to 'all'.
//   - Terms: A list of terms that must be present in the text of each search hit based on the specified strategy.
type SearchMatchTerms struct {
	Strategy *string   `json:"strategy,omitempty"`
	Terms    *[]string `json:"terms,omitempty"`
}

// [SearchRecordsRerank] represents the parameters for reranking search results.
//
// Fields:
//   - Model: The name of the [reranking model](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models) to use.
//   - RankFields: The field(s) to consider for reranking. Defaults to `["text"]`. The number of fields supported is [model-specific](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models).
//   - Parameters: Additional model-specific parameters. Refer to the [model guide](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models) for available model parameters.
//   - Query: The query to rerank documents against. If a specific rerank query is specified,  it overwrites the query input that was provided at the top level.
//   - TopN: The number of top results to return after reranking. Defaults to top_k.
type SearchRecordsRerank struct {
	Model      string                  `json:"model"`
	RankFields []string                `json:"rank_fields"`
	Parameters *map[string]interface{} `json:"parameters,omitempty"`
	Query      *string                 `json:"query,omitempty"`
	TopN       *int32                  `json:"top_n,omitempty"`
}

// [Hit] represents a record whose vector values are similar to the provided search query.
//
// Fields:
//   - Id: The record ID of the search hit.
//   - Score: The similarity score of the returned record.
//   - Fields: The selected record fields associated with the search hit.
type Hit struct {
	Id     string                 `json:"_id"`
	Score  float32                `json:"_score"`
	Fields map[string]interface{} `json:"fields"`
}

// [SearchRecordsResponse] represents the response of a records search.
//
// Fields:
//   - Result: The result object containing the [Hit] responses for the search.
//   - Usage: The resource usage details for the search operation.
type SearchRecordsResponse struct {
	Result struct {
		Hits []Hit `json:"hits"`
	} `json:"result"`
	Usage SearchUsage `json:"usage"`
}

// [SearchRecordsVector] represents the vector data used in a search request.
//
// Fields:
//   - SparseIndices: The sparse embedding indices.
//   - SparseValues: The sparse embedding values.
//   - Values: The dense vector data included in the request.
type SearchRecordsVector struct {
	SparseIndices *[]int32   `json:"sparse_indices,omitempty"`
	SparseValues  *[]float32 `json:"sparse_values,omitempty"`
	Values        *[]float32 `json:"values,omitempty"`
}

// [SearchUsage] represents the resource usage details of a search operation.
//
// Fields:
//   - ReadUnits: The number of read units consumed by this operation.
//   - EmbedTotalTokens: The number of embedding tokens consumed by this operation.
//   - RerankUnits: The number of rerank units consumed by this operation.
type SearchUsage struct {
	ReadUnits        int32  `json:"read_units"`
	EmbedTotalTokens *int32 `json:"embed_total_tokens,omitempty"`
	RerankUnits      *int32 `json:"rerank_units,omitempty"`
}

// [ModelInfoList] represents a list of [ModelInfo] objects describing the models hosted by Pinecone.
//
// Fields:
//   - Models: A slice of [ModelInfo] objects.
type ModelInfoList struct {
	Models *[]ModelInfo `json:"models,omitempty"`
}

// [ModelInfo] represents the model configuration include model type, supported parameters, and other model details.
//
// Fields:
//   - DefaultDimension: The default embedding model dimension (applies to dense embedding models only).
//   - MaxBatchSize: The maximum batch size (number of sequences) supported by the model.
//   - MaxSequenceLength: The maximum tokens per sequence supported by the model.
//   - Modality: The modality of the model (e.g. "text").
//   - Model: The name of the model.
//   - ProviderName: The name of the provider of the model. (e.g. "Pinecone", "NVIDIA").
//   - ShortDescription: A summary of the model.
//   - SupportedDimensions: The list of supported dimensions for the model (applies to dense embedding models only).
//   - SupportedMetrics: The distance metrics supported by the model for similarity search (e.g. "cosine", "dotproduct", "euclidean").
//   - SupportedParameters: A list of parameters supported by the model, including parameter value constraints.
//   - Type: The type of model (e.g. "embed" or "rerank").
//   - VectorType: Whether the embedding model produces "dense" or "sparse" embeddings.
type ModelInfo struct {
	DefaultDimension    *int32                `json:"default_dimension,omitempty"`
	MaxBatchSize        *int32                `json:"max_batch_size,omitempty"`
	MaxSequenceLength   *int32                `json:"max_sequence_length,omitempty"`
	Modality            *string               `json:"modality,omitempty"`
	Model               string                `json:"model"`
	ProviderName        *string               `json:"provider_name,omitempty"`
	ShortDescription    string                `json:"short_description"`
	SupportedDimensions *[]int32              `json:"supported_dimensions,omitempty"`
	SupportedMetrics    *[]IndexMetric        `json:"supported_metrics,omitempty"`
	SupportedParameters *[]SupportedParameter `json:"supported_parameters,omitempty"`
	Type                string                `json:"type"`
	VectorType          *string               `json:"vector_type,omitempty"`
}

// [SupportedParameter] describes a parameter supported by the model, including parameter value constraints.
//
// Fields:
//   - AllowedValues: The allowed parameter values when the type is "one_of".
//   - Default: The default value for the parameter when a parameter is optional.
//   - Max: The maximum allowed value (inclusive) when the type is "numeric_range".
//   - Min: The minimum allowed value (inclusive) when the type is "numeric_range".
//   - Parameter: The name of the parameter.
//   - Required: Indicates whether this parameter is required or optional.
//   - Type: The parameter type e.g. "one_of", "numeric_range", or "any". If the type is "one_of", then "allowed_values" will be set,
//     and the value specified must be one of the allowed values. "one_of" is only compatible with ValueType "string" or "integer".
//     If "numeric_range", then "min" and "max" will be set, then the value specified must adhere to the ValueType and must fall within
//     the `[Min, Max]` range. If "any" then any value is allowed, as long as it adheres to the ValueType.
//   - ValueType: The type of value the parameter accepts, e.g. "string", "integer", "float", or "boolean".
type SupportedParameter struct {
	AllowedValues *[]SupportedParameterValue `json:"allowed_values,omitempty"`
	Default       *SupportedParameterValue   `json:"default,omitempty"`
	Max           *float32                   `json:"max,omitempty"`
	Min           *float32                   `json:"min,omitempty"`
	Parameter     string                     `json:"parameter"`
	Required      bool                       `json:"required"`
	Type          string                     `json:"type"`
	ValueType     string                     `json:"value_type"`
}

// [SupportedParameterValue] is a tagged union type representing the value of a [SupportedParameter].
//
// Fields:
//   - StringValue: A string-based value, if the parameter accepts strings.
//   - IntValue: An integer-based value, if the parameter accepts integers.
//   - FloatValue: A float-based value, if the parameter accepts floating point numbers.
//   - BoolValue: A boolean value, if the parameter accepts true/false input.
type SupportedParameterValue struct {
	StringValue *string
	IntValue    *int32
	FloatValue  *float32
	BoolValue   *bool
}

func (spv *SupportedParameterValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		spv.StringValue = &s
		return nil
	}

	var i int32
	if err := json.Unmarshal(data, &i); err == nil {
		spv.IntValue = &i
		return nil
	}

	var f float32
	if err := json.Unmarshal(data, &f); err == nil {
		spv.FloatValue = &f
		return nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		spv.BoolValue = &b
		return nil
	}
	return fmt.Errorf("unsupported type for SupportedParameterValue: %s", data)
}

// [Backup] describes the configuration and status of a Pinecone backup.
//
// Fields:
//   - BackupId: Unique identifier for the backup.
//   - Cloud: Cloud provider where the backup is stored.
//   - CreatedAt: Timestamp when the backup was created.
//   - Description: Optional description providing context for the backup.
//   - Dimension: The dimensions of the vectors to be inserted in the index.
//   - Metric: The distance metric to be used for similarity search. You can use 'euclidean', 'cosine', or 'dotproduct'. If the 'vector_type' is 'sparse', the metric must be 'dotproduct'. If the `vector_type` is `dense`, the metric defaults to 'cosine'.
//   - Name: Optional user-defined name for the backup.
//   - NamespaceCount: Number of namespaces in the backup.
//   - RecordCount: Total number of records in the backup.
//   - Region: Cloud region where the backup is stored.
//   - Schema: The typed [IndexSchema] of the source index, when reported.
//   - SizeBytes: Size of the backup in bytes.
//   - SourceIndexDeletedAt: Deletion timestamp of the source index, or nil while that index is still active.
//   - SourceIndexId: ID of the index.
//   - SourceIndexName: Name of the index from which the backup was taken.
//   - Status: Current status of the backup (e.g., Initializing, Ready, Failed).
//   - Tags: Custom user tags added to an index. Keys must be 80 characters or less. Values must be 120 characters or less. Keys must be alphanumeric, '_', or '-'. Values must be alphanumeric, ';', '@', '_', '-', '.', '+', or ' '. To unset a key, set the value to an empty string.
//   - Dimension, Metric: Deprecated: computed from Schema's dense vector field for backward compatibility.
type Backup struct {
	BackupId             string       `json:"backup_id"`
	Cloud                string       `json:"cloud"`
	CreatedAt            *string      `json:"created_at,omitempty"`
	Description          *string      `json:"description,omitempty"`
	Name                 *string      `json:"name,omitempty"`
	NamespaceCount       *int64       `json:"namespace_count,omitempty"`
	RecordCount          *int64       `json:"record_count,omitempty"`
	Region               string       `json:"region"`
	Schema               *IndexSchema `json:"schema,omitempty"`
	SizeBytes            *int64       `json:"size_bytes,omitempty"`
	SourceIndexDeletedAt *time.Time   `json:"source_index_deleted_at,omitempty"`
	SourceIndexId        string       `json:"source_index_id"`
	SourceIndexName      string       `json:"source_index_name"`
	Status               string       `json:"status"`
	Tags                 *IndexTags   `json:"tags,omitempty"`

	// Deprecated: computed from Schema's dense vector field for backward compatibility.
	Dimension *int32 `json:"dimension,omitempty"`
	// Deprecated: computed from Schema's dense vector field for backward compatibility.
	Metric *IndexMetric `json:"metric,omitempty"`
}

// [BackupList] contains a paginated list of backups.
//
// Fields:
//   - Data: A list of [Backup] records.
//   - Pagination: Pagination token for fetching the next page of results.
type BackupList struct {
	Data       []*Backup   `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// [RestoreJob] describes the status of a restore job.
//
// Fields:
//   - BackupId: Backup used for the restore.
//   - CompletedAt: Timestamp when the restore job finished.
//   - CreatedAt: Timestamp when the restore job started.
//   - PercentComplete: The progress made by the restore job out of 100.
//   - RestoreJobId: Unique identifier for the restore job.
//   - Status: Status of the restore job.
//   - TargetIndexId: ID of the index.
//   - TargetIndexName: Name of the index into which data is being restored.
type RestoreJob struct {
	BackupId        string     `json:"backup_id"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	PercentComplete *float32   `json:"percent_complete,omitempty"`
	RestoreJobId    string     `json:"restore_job_id"`
	Status          string     `json:"status"`
	TargetIndexId   string     `json:"target_index_id"`
	TargetIndexName string     `json:"target_index_name"`
}

// [RestoreJobList] contains a paginated list of restore jobs.
//
// Fields:
//   - Data: A list of [RestoreJob] records.
//   - Pagination: Pagination token for fetching the next page of results.
type RestoreJobList struct {
	Data       []*RestoreJob `json:"data"`
	Pagination *Pagination   `json:"pagination,omitempty"`
}

// [Project] represents the details of a project.
type Project struct {
	// The name of the project.
	Name string `json:"name"`

	// The unique ID of the project.
	Id string `json:"id"`

	// The unique ID of the organization that the project belongs to.
	OrganizationId string `json:"organization_id"`

	// The date and time when the project was created.
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// Whether to force encryption with a customer-managed encryption key (CMEK).
	ForceEncryptionWithCmek bool `json:"force_encryption_with_cmek"`

	// The maximum number of Pods that can be created in the project.
	MaxPods int `json:"max_pods"`
}

// [Organization] represents the details of an organization.
type Organization struct {
	// The name of the organization.
	Name string `json:"name"`

	// The unique ID of the organization.
	Id string `json:"id"`

	// The date and time when the organization was created.
	CreatedAt time.Time `json:"created_at"`

	// The current payment status of the organization.
	PaymentStatus string `json:"payment_status"`

	// The current plan the organization is on.
	Plan string `json:"plan"`

	// The support tier of the organization.
	SupportTier string `json:"support_tier"`
}

// [APIKey] represents the details of an API key without the secret.
type APIKey struct {
	// The name of the API key.
	Name string `json:"name"`

	// The unique ID of the API key.
	Id string `json:"id"`

	// The ID of the project containing the API key.
	ProjectId string `json:"project_id"`

	// The roles assigned to the API key.
	Roles []string `json:"roles"`
}

// [APIKeyWithSecret] represents the details of an API key with the secret.
type APIKeyWithSecret struct {
	// The details of an [APIKey], without the secret.
	Key APIKey `json:"key"`

	// The value to use as an API key. New keys will have the format `"pckey_<public-label>_<unique-key>"`.
	// The entire string should be used when authenticating.
	Value string `json:"value"`
}

// [PrincipalType] is the kind of principal that receives permissions from a [RoleBinding].
type PrincipalType string

const (
	PrincipalTypeUser           PrincipalType = "user"
	PrincipalTypeServiceAccount PrincipalType = "service_account"
	PrincipalTypeAPIKey         PrincipalType = "api_key"
	PrincipalTypeInvite         PrincipalType = "invite"
)

// [ResourceType] is the kind of resource scope a [RoleBinding] applies to.
type ResourceType string

const (
	ResourceTypeOrganization ResourceType = "organization"
	ResourceTypeProject      ResourceType = "project"
)

// [RoleBinding] grants a [Role] to a principal (a user, service account, API key,
// or invite) at an organization or project scope.
type RoleBinding struct {
	// The unique ID of the role binding.
	Id string `json:"id"`

	// The principal's ID. A UUID for all principal types.
	PrincipalId string `json:"principal_id"`

	// The kind of principal that receives permissions from the role binding.
	PrincipalType PrincipalType `json:"principal_type"`

	// The unique ID of the organization or project that the binding is scoped to.
	ResourceId string `json:"resource_id"`

	// The kind of resource scope the role binding applies to.
	ResourceType ResourceType `json:"resource_type"`

	// The role assigned to the principal at the resource scope.
	Role string `json:"role"`

	// The date and time when the role binding was created.
	CreatedAt time.Time `json:"created_at"`
}

// [RoleBindingInput] describes a role to grant to a principal when creating a
// resource such as an invite or service account. ResourceType selects the binding
// scope: for "organization" scope, omit ResourceId; for "project" scope, ResourceId
// is required and must be the project's ID.
type RoleBindingInput struct {
	// The kind of resource scope the role binding applies to.
	ResourceType ResourceType `json:"resource_type"`

	// The role to assign to the principal at the resource scope.
	// Expected "organization"-scoped values: "OrgOwner", "OrgManager", "OrgBillingAdmin", "OrgMember".
	// Expected "project"-scoped values: "ProjectOwner", "ProjectManager", "ProjectMember", "ProjectEditor", "ProjectViewer", "ControlPlaneEditor", "ControlPlaneViewer", "DataPlaneEditor", "DataPlaneViewer".
	Role string `json:"role"`

	// (Optional) The ID of the project the binding applies to. Required when
	// ResourceType is "project"; omit for "organization" scope.
	ResourceId *string `json:"resource_id,omitempty"`
}

// [RoleBindingList] contains a paginated list of role bindings.
//
// Fields:
//   - Data: A list of [RoleBinding] records.
//   - Pagination: Pagination token for fetching the next page of results.
type RoleBindingList struct {
	Data       []*RoleBinding `json:"data"`
	Pagination *Pagination    `json:"pagination,omitempty"`
}

// [ServiceAccount] represents a service account. The OAuth client secret is not included;
// it is returned only once, at creation or secret rotation, as a [ServiceAccountWithSecret].
type ServiceAccount struct {
	// The unique ID of the service account. Use this as the principal ID when
	// creating or querying role bindings for the service account.
	Id string `json:"id"`

	// A short human-readable name, set by the caller at creation time.
	Name string `json:"name"`

	// The OAuth client ID used by the service account to obtain access tokens.
	ClientId string `json:"client_id"`

	// The date and time the service account was created.
	CreatedAt time.Time `json:"created_at"`

	// The date and time of the service account's most recent metadata update.
	UpdatedAt time.Time `json:"updated_at"`
}

// [ServiceAccountWithSecret] represents a service account together with a newly issued
// OAuth client secret. The secret is returned only once — at creation or secret
// rotation — and cannot be retrieved later. Treat ClientSecret as a credential: store
// it securely and never log it.
type ServiceAccountWithSecret struct {
	// The details of the service account, without the secret.
	ServiceAccount ServiceAccount `json:"service_account"`

	// The OAuth client secret. Returned exactly once. Store it securely and never log it.
	ClientSecret string `json:"client_secret"`
}

// [ServiceAccountList] contains a paginated list of service accounts.
//
// Fields:
//   - Data: A list of [ServiceAccount] records.
//   - Pagination: Pagination token for fetching the next page of results.
type ServiceAccountList struct {
	Data       []*ServiceAccount `json:"data"`
	Pagination *Pagination       `json:"pagination,omitempty"`
}

// [InviteStatus] is the lifecycle status of an [Invite].
type InviteStatus string

const (
	InviteStatusPending   InviteStatus = "pending"
	InviteStatusExpired   InviteStatus = "expired"
	InviteStatusProcessed InviteStatus = "processed"
)

// [Invite] represents an invitation to join the organization.
type Invite struct {
	// The unique ID of the invite.
	Id string `json:"id"`

	// The email address the invite was sent to.
	Email string `json:"email"`

	// The lifecycle status of the invite. List endpoints return only "pending" and
	// "expired" invites; "processed" is returned only when fetching a single invite by ID.
	Status InviteStatus `json:"status"`

	// The date and time the invite was created.
	CreatedAt time.Time `json:"created_at"`

	// (Optional) When the invite expires if not accepted. The default TTL is 7 days,
	// and resending the invite extends it. Nil if the invite does not expire.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// (Optional) The date and time the invite was accepted. Nil while the invite is
	// still pending or expired.
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// [InviteList] contains a paginated list of invites.
//
// Fields:
//   - Data: A list of [Invite] records.
//   - Pagination: Pagination token for fetching the next page of results.
type InviteList struct {
	Data       []*Invite   `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// [User] represents a user who is a member of the organization.
type User struct {
	// The unique ID of the user. Use this as the principal ID when creating or
	// querying role bindings for the user.
	Id string `json:"id"`

	// The user's email address.
	Email string `json:"email"`

	// (Optional) The user's display name. Nil if the user has not set one.
	Name *string `json:"name,omitempty"`
}

// [UserList] contains a paginated list of users.
//
// Fields:
//   - Data: A list of [User] records.
//   - Pagination: Pagination token for fetching the next page of results.
type UserList struct {
	Data       []*User     `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Schema for the behavior of Pinecone's internal metadata index. By default, all metadata is indexed; when `schema` is present, only fields which are present in the `fields` object with a `filterable: true` are indexed. Note that `filterable: false` is not currently supported.
type MetadataSchema struct {
	Fields map[string]MetadataSchemaField `json:"fields"`
}

// A map of metadata field names to their configuration. The field name must be a valid metadata field name. The field name must be unique.
type MetadataSchemaField struct {
	// Whether the field is filterable. If true, the field is indexed and can be used in filters. Only true values are allowed.
	Filterable bool `json:"filterable,omitempty"`
}
