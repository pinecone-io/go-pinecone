package pinecone

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// IndexMetric is the [similarity metric] to be used by similarity search against a Pinecone [Index].
//
// [similarity metric]: https://docs.pinecone.io/guides/index-data/create-an-index#similarity-metrics
type IndexMetric string

const (
	// IndexMetricCosine is cosine similarity. [Client.CreateServerlessIndex] and
	// [Client.CreateBYOCIndex] use it for an index that stores dense vectors when Metric is unset.
	IndexMetricCosine IndexMetric = "cosine"
	// IndexMetricDotproduct is the dot product. It is required for sparse vectors, and for hybrid
	// queries on a vector index that stores both dense and sparse vectors.
	IndexMetricDotproduct IndexMetric = "dotproduct"
	// IndexMetricEuclidean is Euclidean (L2) distance.
	IndexMetricEuclidean IndexMetric = "euclidean"
)

// IndexStatusState is the state of a Pinecone [Index], as reported in IndexStatus.State. The API
// may add states, so don't assume the constants below are exhaustive.
type IndexStatusState string

const (
	// IndexStatusStateDisabled means the index is disabled.
	IndexStatusStateDisabled IndexStatusState = "Disabled"
	// IndexStatusStateFailed means the index is in a failed state.
	IndexStatusStateFailed IndexStatusState = "Failed"
	// IndexStatusStateInitializationFailed means the index failed to initialize.
	IndexStatusStateInitializationFailed IndexStatusState = "InitializationFailed"
	// IndexStatusStateInitializing means the index is being created or upgraded.
	IndexStatusStateInitializing IndexStatusState = "Initializing"
	// IndexStatusStateReady means the index is ready to serve requests.
	IndexStatusStateReady IndexStatusState = "Ready"
	// IndexStatusStateScalingDown means the index is scaling down.
	IndexStatusStateScalingDown IndexStatusState = "ScalingDown"
	// IndexStatusStateScalingUp means the index is scaling up.
	IndexStatusStateScalingUp IndexStatusState = "ScalingUp"
	// IndexStatusStateScalingUpPodSize means a pod-based index is being scaled to a larger pod size.
	IndexStatusStateScalingUpPodSize IndexStatusState = "ScalingUpPodSize"
	// IndexStatusStateTerminating means the index is being deleted.
	IndexStatusStateTerminating IndexStatusState = "Terminating"
)

// DeletionProtection determines whether [deletion protection] is "enabled" or "disabled" for the [Index].
// When "enabled", the [Index] cannot be deleted. Defaults to "disabled".
//
// [deletion protection]: https://docs.pinecone.io/guides/manage-data/manage-indexes#configure-deletion-protection
type DeletionProtection string

const (
	// DeletionProtectionEnabled prevents the index from being deleted.
	DeletionProtectionEnabled DeletionProtection = "enabled"
	// DeletionProtectionDisabled allows the index to be deleted.
	DeletionProtectionDisabled DeletionProtection = "disabled"
)

// Cloud is the [cloud provider] hosting a Pinecone [Index].
//
// [cloud provider]: https://docs.pinecone.io/guides/index-data/create-an-index#cloud-regions
type Cloud string

const (
	// CloudAWS is Amazon Web Services.
	CloudAWS Cloud = "aws"
	// CloudAzure is Microsoft Azure.
	CloudAzure Cloud = "azure"
	// CloudGCP is Google Cloud Platform.
	CloudGCP Cloud = "gcp"
)

// IndexStatus is the status of a Pinecone [Index].
type IndexStatus struct {
	// Ready reports whether the index can serve requests. It is true when State is
	// IndexStatusStateReady or IndexStatusStateDisabled, and while an existing index is being upgraded;
	// it is false while the index is scaling. Check State to tell a disabled index from a ready one.
	Ready bool `json:"ready"`
	// State is the current [IndexStatusState] of the index.
	State IndexStatusState `json:"state"`
}

// IndexSpec is the infrastructure specification (serverless, pod-based, or BYOC) of a Pinecone [Index].
// Only one of the following fields will be present: Pod, Serverless, BYOC.
type IndexSpec struct {
	// Pod is the [PodSpec] of a pod-based index.
	Pod *PodSpec `json:"pod,omitempty"`
	// Serverless is the [ServerlessSpec] of a serverless index.
	Serverless *ServerlessSpec `json:"serverless,omitempty"`
	// BYOC is the [BYOCSpec] of a BYOC index.
	BYOC *BYOCSpec `json:"byoc,omitempty"`
}

// IndexEmbed represents the embedding model configured for an index,
// including document fields mapped to embedding inputs.
type IndexEmbed struct {
	// Model is the name of the embedding model used to create the index (e.g., "multilingual-e5-large").
	Model string `json:"model"`
	// Dimension is the dimension of the embedding model, specifying the size of the output vector.
	Dimension *int32 `json:"dimension,omitempty"`
	// Metric is the distance metric used by the embedding model. If VectorType is "sparse", the
	// metric must be "dotproduct". If VectorType is "dense", the metric defaults to "cosine".
	Metric *IndexMetric `json:"metric,omitempty"`
	// VectorType is always nil: Pinecone API version 2026-07 doesn't report it. A nil Dimension
	// means the model produces sparse vectors.
	VectorType *string `json:"vector_type,omitempty"`
	// FieldMap identifies the name of the text field from your document model that is embedded.
	FieldMap *map[string]interface{} `json:"field_map,omitempty"`
	// ReadParameters are the read parameters for the embedding model.
	ReadParameters *map[string]interface{} `json:"read_parameters,omitempty"`
	// WriteParameters are the write parameters for the embedding model.
	WriteParameters *map[string]interface{} `json:"write_parameters,omitempty"`
}

// IndexTags is a set of key-value pairs attached to a Pinecone [Index]. An index can have at most 20
// tags. Keys must be 1-80 characters of ASCII letters, digits, '_', or '-'. Values must be 120
// characters or less of printable ASCII characters or spaces.
type IndexTags map[string]string

// IndexSchema is the schema of a Pinecone [Index]. The schema defines the typed fields that records
// in the index can contain, including vector fields, semantic text fields, and metadata fields.
//
// Vector indexes, including indexes created from a dimension and metric, report their vectors under
// the reserved field names "_values" (dense) and "_sparse_values" (sparse). Metadata fields
// configured for filtering on older indexes are reported as [LegacyMetadataField] entries.
type IndexSchema struct {
	// Fields maps each field name to its [IndexSchemaField] configuration.
	Fields map[string]IndexSchemaField `json:"fields"`
}

// IndexSchemaField is the configuration of a single field in an [IndexSchema]. Exactly one of the
// pointer fields is non-nil, identifying the field's type.
type IndexSchemaField struct {
	// DenseVector is set when the field is a [DenseVectorField].
	DenseVector *DenseVectorField `json:"dense_vector,omitempty"`
	// SparseVector is set when the field is a [SparseVectorField].
	SparseVector *SparseVectorField `json:"sparse_vector,omitempty"`
	// SemanticText is set when the field is a [SemanticTextField]. Reported on responses only.
	SemanticText *SemanticTextField `json:"semantic_text,omitempty"`
	// String is set when the field is a [StringField].
	String *StringField `json:"string,omitempty"`
	// StringList is set when the field is a [StringListField]. Reported on responses only.
	StringList *StringListField `json:"string_list,omitempty"`
	// Boolean is set when the field is a [BooleanField]. Reported on responses only.
	Boolean *BooleanField `json:"boolean,omitempty"`
	// Float is set when the field is a [FloatField]. Reported on responses only.
	Float *FloatField `json:"float,omitempty"`
	// Integer is set when the field is an [IntegerField]. Reported on responses only.
	Integer *IntegerField `json:"integer,omitempty"`
	// LegacyMetadata is set when the field is a [LegacyMetadataField]. Reported on responses only.
	LegacyMetadata *LegacyMetadataField `json:"legacy_metadata,omitempty"`
}

// DenseVectorField is a dense vector field configuration. Stores fixed-dimension floating-point
// vectors for approximate nearest-neighbor (ANN) search.
type DenseVectorField struct {
	// Dimension is the number of dimensions in the dense vectors stored in this field.
	Dimension int32 `json:"dimension"`
	// Metric is the distance metric used for similarity search.
	Metric IndexMetric `json:"metric"`
	// Description (Optional) describes the field, at most 256 bytes.
	Description *string `json:"description,omitempty"`
}

// SparseVectorField is a sparse vector field configuration. Sparse fields take no dimension and no
// metric; sparse scoring is not configurable.
type SparseVectorField struct {
	// Description (Optional) describes the field, at most 256 bytes.
	Description *string `json:"description,omitempty"`
}

// SemanticTextField is a semantic text field backed by an integrated embedding model, as returned
// when describing an index created with [Client.CreateIndexForModel]. It cannot be declared when
// creating an index directly.
type SemanticTextField struct {
	// Model is the name of the integrated embedding model used for this field.
	Model string `json:"model"`
	// Dimension is the dimension of the vectors the model produces. Nil for models that produce
	// sparse vectors.
	Dimension *int32 `json:"dimension,omitempty"`
	// Metric is the distance metric used for similarity search.
	Metric *IndexMetric `json:"metric,omitempty"`
	// ReadParameters are model-specific parameters applied at query time, such as "input_type".
	ReadParameters *map[string]interface{} `json:"read_parameters,omitempty"`
	// WriteParameters are model-specific parameters applied at write time, such as "input_type".
	WriteParameters *map[string]interface{} `json:"write_parameters,omitempty"`
	// Description is the field's description, if one was set.
	Description *string `json:"description,omitempty"`
}

// StringField is a string field configuration. When FullTextSearch is present the field is indexed
// for full-text search; Filterable is reported on responses when the field is indexed for metadata
// filtering.
type StringField struct {
	// FullTextSearch is the [FullTextSearchConfig] for the field. Required when declaring a
	// StringField at index creation.
	FullTextSearch *FullTextSearchConfig `json:"full_text_search,omitempty"`
	// Filterable reports whether the field is indexed for metadata filtering. Reported on
	// responses only; ignored when creating an index.
	Filterable *bool `json:"filterable,omitempty"`
	// Description (Optional) describes the field, at most 256 bytes.
	Description *string `json:"description,omitempty"`
}

// FullTextSearchConfig configures full-text search on a [StringField]. StopWords requires
// Stemming; Ngram cannot be combined with Stemming or StopWords.
type FullTextSearchConfig struct {
	// Language (Optional) is the language for text analysis. Defaults to "en".
	Language *string `json:"language,omitempty"`
	// Stemming (Optional) determines whether stemming is applied during text analysis. Defaults to false.
	Stemming *bool `json:"stemming,omitempty"`
	// StopWords (Optional) determines whether stop words are filtered during text analysis.
	// Requires Stemming. Defaults to false.
	StopWords *bool `json:"stop_words,omitempty"`
	// Ngram (Optional) tokenizes the field into character n-grams instead of words. See [NgramConfig].
	Ngram *NgramConfig `json:"ngram,omitempty"`
}

// NgramConfig configures character n-gram tokenization for substring matching on a full-text-search
// [StringField].
type NgramConfig struct {
	// MinGram is the minimum n-gram length. Must be at least 1 and no greater than MaxGram.
	MinGram int `json:"min_gram"`
	// MaxGram is the maximum n-gram length. Must be no less than MinGram and at most 10.
	MaxGram int `json:"max_gram"`
	// PrefixOnly (Optional) generates only prefix n-grams anchored at the start of each token
	// (e.g. for autocomplete). Defaults to false.
	PrefixOnly *bool `json:"prefix_only,omitempty"`
}

// StringListField is a string array field configuration reported in index schemas. String array
// values are indexed automatically at upsert time and cannot be declared at index creation.
type StringListField struct {
	// Filterable reports whether the field is indexed for metadata filtering.
	Filterable *bool `json:"filterable,omitempty"`
	// Description is the field's description, if one was set.
	Description *string `json:"description,omitempty"`
}

// BooleanField is a boolean field configuration reported in index schemas. Boolean values are
// indexed automatically at upsert time and cannot be declared at index creation.
type BooleanField struct {
	// Filterable reports whether the field is indexed for metadata filtering.
	Filterable *bool `json:"filterable,omitempty"`
	// Description is the field's description, if one was set.
	Description *string `json:"description,omitempty"`
}

// FloatField is a numeric (floating-point) field configuration reported in index schemas. Numeric
// values are indexed automatically at upsert time and cannot be declared at index creation.
type FloatField struct {
	// Filterable reports whether the field is indexed for metadata filtering.
	Filterable *bool `json:"filterable,omitempty"`
	// Description is the field's description, if one was set.
	Description *string `json:"description,omitempty"`
}

// IntegerField is an integer field configuration reported in index schemas. Integer values are
// indexed automatically at upsert time and cannot be declared at index creation.
type IntegerField struct {
	// Filterable reports whether the field is indexed for metadata filtering.
	Filterable *bool `json:"filterable,omitempty"`
	// Description is the field's description, if one was set.
	Description *string `json:"description,omitempty"`
}

// LegacyMetadataField is a metadata field from an index that pre-dates typed schemas, carrying only
// whether the field is indexed for filtering.
type LegacyMetadataField struct {
	// Filterable reports whether the field is indexed for metadata filtering.
	Filterable bool `json:"filterable"`
}

// IndexDeployment describes where a Pinecone [Index] runs. Exactly one of the pointer fields is
// non-nil, identifying the deployment type.
type IndexDeployment struct {
	// Managed is set for a serverless index. See [ManagedDeployment].
	Managed *ManagedDeployment `json:"managed,omitempty"`
	// Pod is set for a pod-based index. See [PodDeployment]. Pod-based indexes can't be created
	// on API version 2026-07, but existing ones are still reported.
	Pod *PodDeployment `json:"pod,omitempty"`
	// Byoc is set for a BYOC index. See [ByocDeployment].
	Byoc *ByocDeployment `json:"byoc,omitempty"`
}

// ManagedDeployment is the deployment configuration for a serverless (managed) index. Environment
// is returned in responses and must not be set when creating an index.
type ManagedDeployment struct {
	// Cloud is the public cloud where the index is hosted.
	Cloud Cloud `json:"cloud"`
	// Region is the cloud region where the index is hosted.
	Region string `json:"region"`
	// Environment is the Pinecone environment hosting the index. Reported on responses only.
	Environment *string `json:"environment,omitempty"`
}

// PodDeployment is the deployment configuration of a pod-based index.
type PodDeployment struct {
	// Environment is the environment where the index is hosted.
	Environment string `json:"environment"`
	// PodType is the pod type of the index: one of "s1", "p1", or "p2", followed by ".x1", ".x2",
	// ".x4", or ".x8".
	PodType string `json:"pod_type"`
	// Replicas is the number of replicas. Replicas duplicate the index for higher availability
	// and throughput.
	Replicas *int32 `json:"replicas,omitempty"`
	// Shards is the number of shards. Shards split the data across multiple pods.
	Shards *int32 `json:"shards,omitempty"`
}

// ByocDeployment is the deployment configuration of a BYOC (bring-your-own-cloud) index.
type ByocDeployment struct {
	// Environment is the BYOC environment where the index is hosted.
	Environment string `json:"environment"`
}

// Index is a Pinecone [Index] object.
//
// An index is described by its [IndexSchema] and [IndexDeployment]. The Metric, VectorType,
// Dimension, Spec, and Embed fields are derived from them for backward compatibility and are
// deprecated; see the field docs.
type Index struct {
	// Name is the name of the index.
	Name string `json:"name"`
	// Host is the URL address where the index is hosted.
	Host string `json:"host"`
	// Schema is the [IndexSchema] of the index, defining its typed fields.
	Schema *IndexSchema `json:"schema,omitempty"`
	// Deployment is the [IndexDeployment] of the index (managed, pod, or BYOC).
	Deployment *IndexDeployment `json:"deployment,omitempty"`
	// ReadCapacity is the [ReadCapacity] configuration of the index, if any.
	ReadCapacity *ReadCapacity `json:"read_capacity,omitempty"`
	// DeletionProtection is whether deletion protection is "enabled" or "disabled" for the index.
	DeletionProtection DeletionProtection `json:"deletion_protection,omitempty"`
	// PrivateHost is the private endpoint URL of the index, if any.
	PrivateHost *string `json:"private_host,omitempty"`
	// SourceCollection is the name of the collection this index was created from, if any.
	SourceCollection *string `json:"source_collection,omitempty"`
	// SourceBackupId is the ID of the backup this index was restored from, if any.
	SourceBackupId *string `json:"source_backup_id,omitempty"`
	// CmekId is the ID of the customer-managed encryption key (CMEK) used to encrypt the index, if any.
	CmekId *string `json:"cmek_id,omitempty"`
	// Status is the [IndexStatus] of the index, which includes index state information.
	Status *IndexStatus `json:"status,omitempty"`
	// Tags are the custom [IndexTags] added to the index.
	Tags *IndexTags `json:"tags,omitempty"`

	// Metric is the distance metric of the index's dense vector field.
	//
	// Deprecated: Derived from Schema for backward compatibility. Use the [DenseVectorField] in Schema instead.
	Metric IndexMetric `json:"metric"`
	// VectorType is the index's vector type, "dense" or "sparse".
	//
	// Deprecated: Derived from Schema for backward compatibility. Use the vector fields in Schema instead.
	VectorType string `json:"vector_type"`
	// Dimension is the dimension of the index's dense vector field.
	//
	// Deprecated: Derived from Schema for backward compatibility. Use the [DenseVectorField] in Schema instead.
	Dimension *int32 `json:"dimension,omitempty"`
	// Spec is the index's [IndexSpec]: a [PodSpec], [ServerlessSpec], or [BYOCSpec].
	//
	// Deprecated: Derived from Deployment for backward compatibility. Use Deployment instead.
	Spec *IndexSpec `json:"spec,omitempty"`
	// Embed is the [IndexEmbed] model configured for the index, if applicable.
	//
	// Deprecated: Derived from Schema for backward compatibility. Use the [SemanticTextField] in Schema instead.
	Embed *IndexEmbed `json:"embed,omitempty"`
}

// Collection is a Pinecone [collection entity]. Only available for pod-based Indexes.
//
// [collection entity]: https://docs.pinecone.io/guides/indexes/pods/understanding-collections
type Collection struct {
	// Name is the name of the collection.
	Name string `json:"name"`
	// Size is the total size of the collection in bytes.
	Size int64 `json:"size"`
	// Status is the [CollectionStatus] of the collection.
	Status CollectionStatus `json:"status"`
	// Dimension is the dimensionality of the vectors for each record stored in the collection.
	Dimension int32 `json:"dimension"`
	// VectorCount is the number of records (vectors) stored in the collection.
	VectorCount int32 `json:"vector_count"`
	// Environment is the environment where the collection is hosted.
	Environment string `json:"environment"`
}

// CollectionStatus is the status of a Pinecone [Collection].
type CollectionStatus string

const (
	// CollectionStatusInitializing means the collection is being created.
	CollectionStatusInitializing CollectionStatus = "Initializing"
	// CollectionStatusReady means the collection is ready to use.
	CollectionStatusReady CollectionStatus = "Ready"
	// CollectionStatusTerminating means the collection is being deleted.
	CollectionStatusTerminating CollectionStatus = "Terminating"
	// CollectionStatusTerminated means the collection has been deleted.
	CollectionStatusTerminated CollectionStatus = "Terminated"
)

// PodSpecMetadataConfig represents the metadata fields to be indexed when a Pinecone [Index] is created.
type PodSpecMetadataConfig struct {
	// Indexed (Optional) lists the metadata fields to index. When nil, all metadata fields are indexed.
	Indexed *[]string `json:"indexed,omitempty"`
}

// PodSpec is the infrastructure specification of a pod-based Pinecone [Index]. Only available for pod-based Indexes.
type PodSpec struct {
	// Environment is the environment where the index is hosted.
	Environment string `json:"environment"`
	// PodType is the pod type used for the index. Must be one of "s1", "p1", or "p2" followed by
	// ".x1", ".x2", ".x4", or ".x8".
	PodType string `json:"pod_type"`
	// PodCount is the number of pods used for the index. Should equal ShardCount × Replicas.
	PodCount int `json:"pod_count"`
	// Replicas is the number of replicas. Replicas duplicate the index. They provide higher
	// availability and throughput.
	Replicas int32 `json:"replicas"`
	// ShardCount is the number of shards. Shards split your data across multiple pods so you can
	// fit more data into an index.
	ShardCount int32 `json:"shard_count"`
	// SourceCollection is the name of the [Collection] used as a source for the index.
	SourceCollection *string `json:"source_collection,omitempty"`
	// MetadataConfig is always nil: Pinecone API version 2026-07 doesn't report it. Metadata
	// fields configured for filtering are reported in Index.Schema.
	MetadataConfig *PodSpecMetadataConfig `json:"metadata_config,omitempty"`
}

// ServerlessSpec is the infrastructure specification of a serverless Pinecone [Index]. Only available for serverless Indexes.
type ServerlessSpec struct {
	// Cloud is the public cloud provider where the index is hosted.
	Cloud Cloud `json:"cloud"`
	// Region is the region where the index is hosted.
	Region string `json:"region"`
	// Schema is always nil: Pinecone API version 2026-07 doesn't report it. Metadata
	// fields configured for filtering are reported in Index.Schema.
	Schema *MetadataSchema `json:"schema,omitempty"`
	// SourceCollection is the name of the [Collection] used as a source for the index.
	SourceCollection *string `json:"source_collection,omitempty"`
	// ReadCapacity is the [ReadCapacity] configuration of the serverless index.
	ReadCapacity *ReadCapacity `json:"read_capacity,omitempty"`
}

// BYOCSpec is the infrastructure specification of a BYOC Pinecone [Index].
type BYOCSpec struct {
	// Environment is the environment where the index is hosted.
	Environment string `json:"environment"`
	// Schema is always nil: Pinecone API version 2026-07 doesn't report it. Metadata
	// fields configured for filtering are reported in Index.Schema.
	Schema *MetadataSchema `json:"schema,omitempty"`
	// ReadCapacity is the [ReadCapacity] configuration of the index.
	ReadCapacity *ReadCapacity `json:"read_capacity,omitempty"`
}

// ReadCapacity represents the read capacity configuration returned from the API.
// [ReadCapacity] is a tagged union which can have either [ReadCapacityOnDemand] or [ReadCapacityDedicated].
type ReadCapacity struct {
	// OnDemand is set when the index uses OnDemand read capacity, with its current status.
	OnDemand *ReadCapacityOnDemand `json:"on_demand,omitempty"`
	// Dedicated is set when the index uses Dedicated read capacity, with its current status.
	Dedicated *ReadCapacityDedicated `json:"dedicated,omitempty"`
}

// ReadCapacityOnDemand represents OnDemand read capacity mode with status information.
type ReadCapacityOnDemand struct {
	// Status is the current status of the read capacity configuration.
	Status ReadCapacityStatus `json:"status"`
}

// ReadCapacityDedicated represents Dedicated read capacity configuration with status information.
type ReadCapacityDedicated struct {
	// NodeType is the type of machines in use.
	NodeType *string `json:"node_type"`
	// Scaling is the [ReadCapacityScaling] strategy configuration.
	Scaling *ReadCapacityScaling `json:"scaling,omitempty"`
	// Status is the current status of the read capacity configuration.
	Status ReadCapacityStatus `json:"status"`
}

// ReadCapacityScaling represents the scaling configuration for dedicated read capacity.
type ReadCapacityScaling struct {
	// Manual is the manual scaling configuration, with fixed replicas and shards.
	Manual *ReadCapacityManualScaling `json:"manual,omitempty"`
}

// ReadCapacityManualScaling represents manual scaling configuration.
type ReadCapacityManualScaling struct {
	// Replicas is the number of replicas to use. Replicas duplicate the compute resources and data
	// of an index, allowing higher query throughput and availability. Setting replicas to 0
	// disables the index but can be used to reduce costs while usage is paused.
	Replicas *int32 `json:"replicas"`
	// Shards is the number of shards to use. Shards determine the storage capacity of an index,
	// with each shard providing 250 GB of storage.
	Shards *int32 `json:"shards"`
}

// ReadCapacityStatus represents the current status of factors affecting the read capacity of a serverless index.
type ReadCapacityStatus struct {
	// State is the overall status state: "Ready", "Scaling", "Migrating", or "Error".
	State string `json:"state"`
	// CurrentReplicas is the current number of replicas.
	CurrentReplicas *int32 `json:"current_replicas,omitempty"`
	// CurrentShards is the current number of shards.
	CurrentShards *int32 `json:"current_shards,omitempty"`
	// ErrorMessage describes any issues with the read capacity configuration.
	ErrorMessage *string `json:"error_message,omitempty"`
}

// Vector is a [dense or sparse vector object] with optional metadata.
//
// [dense or sparse vector object]: https://docs.pinecone.io/guides/core-concepts/key-terms#dense-vector
type Vector struct {
	// Id is the unique ID of the vector.
	Id string `json:"id"`
	// Values are the dense vector values. On a request, set Values, SparseValues, or both. On a
	// response, Values is nil when the index stores only sparse vectors or values were not requested.
	Values *[]float32 `json:"values,omitempty"`
	// SparseValues are the [SparseValues] of the vector.
	SparseValues *SparseValues `json:"sparse_values,omitempty"`
	// Metadata is the record's [Metadata].
	Metadata *Metadata `json:"metadata,omitempty"`
}

// ScoredVector is a vector with an associated similarity score calculated according to the distance metric of the
// [Index].
type ScoredVector struct {
	// Vector is the matched [Vector]. Its Values and Metadata are populated only when IncludeValues
	// and IncludeMetadata were set on the query.
	Vector *Vector `json:"vector,omitempty"`
	// Score is the similarity score of the vector.
	Score float32 `json:"score"`
}

// SparseValues is a sparse vector object, used on its own in an index that stores only sparse
// vectors, or together with dense values for [hybrid search].
//
// [hybrid search]: https://docs.pinecone.io/guides/search/hybrid-search
type SparseValues struct {
	// Indices are the positions of the non-zero values. Must be the same length as Values.
	Indices []uint32 `json:"indices,omitempty"`
	// Values are the non-zero values, one for each entry in Indices.
	Values []float32 `json:"values,omitempty"`
}

// NamespaceSummary is a summary of stats for a Pinecone [namespace].
//
// [namespace]: https://docs.pinecone.io/guides/index-data/indexing-overview#namespaces
type NamespaceSummary struct {
	// VectorCount is the number of vectors in the namespace.
	VectorCount uint32 `json:"vector_count"`
}

// NamespaceDescription is a description of a Pinecone [namespace].
//
// [namespace]: https://docs.pinecone.io/guides/index-data/indexing-overview#namespaces
type NamespaceDescription struct {
	// Name is the name of the namespace.
	Name string `json:"name"`
	// RecordCount is the number of records in the namespace.
	RecordCount uint64 `json:"record_count"`
	// IndexedFields are the [IndexedFields] of the namespace.
	IndexedFields *IndexedFields `json:"indexed_fields,omitempty"`
	// Schema is the [MetadataSchema] for Pinecone's internal metadata index.
	Schema *MetadataSchema `json:"schema,omitempty"`
	// SizeBytes is the approximate storage size of the namespace in bytes. Data written before
	// size tracking reads as 0, and recently deleted data may still be counted until compaction.
	SizeBytes uint64 `json:"size_bytes"`
}

// IndexedFields is a list of all indexed metadata fields in the namespace.
type IndexedFields struct {
	// Fields are the names of all indexed metadata fields in the namespace.
	Fields []string `json:"fields,omitempty"`
}

// Usage is the read-unit usage ([Read Units]) of a single data-plane operation.
//
// [Read Units]: https://docs.pinecone.io/guides/manage-cost/understanding-cost#read-units
type Usage struct {
	// ReadUnits is the number of read units consumed by the operation.
	ReadUnits uint32 `json:"read_units"`
}

// RerankUsage is the usage stats ([Rerank Units]) for a reranking request.
//
// [Rerank Units]: https://docs.pinecone.io/guides/manage-cost/understanding-cost#reranking
type RerankUsage struct {
	// RerankUnits is the number of rerank units consumed by the request.
	RerankUnits *int `json:"rerank_units,omitempty"`
}

// MetadataFilter represents the [metadata filters] attached to a Pinecone request. It is used by
// query, delete, fetch-by-metadata, update-by-metadata, and filtered stats requests.
//
// [metadata filters]: https://docs.pinecone.io/guides/search/filter-by-metadata
type MetadataFilter = structpb.Struct

// Metadata represents optional, additional information stored as part of a record, which you can
// [filter by]. It can be set on upsert and changed on update.
//
// [filter by]: https://docs.pinecone.io/guides/index-data/indexing-overview#metadata
type Metadata = structpb.Struct

// NewMetadataFilter creates a [MetadataFilter] from a map of key-value pairs representing metadata filter expressions.
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
// [MetadataFilter]: https://docs.pinecone.io/guides/search/filter-by-metadata
func NewMetadataFilter(m map[string]interface{}) (*MetadataFilter, error) {
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// NewMetadata creates [Metadata] from a map of key-value pairs representing metadata fields.
// This helper function eliminates the need to import and use [structpb.Struct] directly.
//
// The input map should contain flat key-value pairs where:
//   - Keys must be strings and must not start with a $
//   - Values must be one of: string, integer, floating point, boolean, or a list of strings given as
//     []interface{} (a []string is rejected when the map is converted)
//   - Nested JSON objects are not supported
//   - Null values are not supported (remove keys instead)
//
// Example:
//
//	metadataMap := map[string]interface{}{
//		"genre":        "classical",
//		"year":         2020,
//		"is_public":    true,
//		"tags":         []interface{}{"beginner", "database"},
//	}
//	metadata, err := pinecone.NewMetadata(metadataMap)
//
// [Metadata]: https://docs.pinecone.io/guides/index-data/indexing-overview#metadata
func NewMetadata(m map[string]interface{}) (*Metadata, error) {
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Embedding represents the embedding of a single input which is returned after [generating embeddings].
// [Embedding] is a tagged union which can have either a [SparseEmbedding] or a [DenseEmbedding].
//
// [generating embeddings]: https://docs.pinecone.io/reference/api/latest/inference/generate-vectors
type Embedding struct {
	// SparseEmbedding is the [SparseEmbedding] representation of the input.
	SparseEmbedding *SparseEmbedding `json:"sparse_embedding,omitempty"`
	// DenseEmbedding is the [DenseEmbedding] representation of the input.
	DenseEmbedding *DenseEmbedding `json:"dense_embedding,omitempty"`
}

// DenseEmbedding represents a dense numerical embedding of the input.
type DenseEmbedding struct {
	// VectorType is the type of vector embedding ("dense").
	VectorType string `json:"vector_type"`
	// Values are the float32 values of the dense embedding.
	Values []float32 `json:"values"`
}

// SparseEmbedding represents a sparse embedding of the input, where only selected dimensions are populated.
type SparseEmbedding struct {
	// VectorType is the type of vector embedding ("sparse").
	VectorType string `json:"vector_type"`
	// SparseValues are the float32 values of the sparse embedding.
	SparseValues []float32 `json:"sparse_values"`
	// SparseIndices are the embedding indices, one for each entry in SparseValues.
	SparseIndices []int64 `json:"sparse_indices"`
	// SparseTokens are the normalized tokens used to create the sparse embedding, if requested.
	SparseTokens *[]string `json:"sparse_tokens,omitempty"`
}

// Pagination represents the pagination information for a list of resources.
type Pagination struct {
	// Next is the token to pass to retrieve the next page of results.
	Next string `json:"next"`
}

// ImportStatus represents the status of an [Import] operation.
type ImportStatus string

const (
	// ImportStatusCancelled means the [Import] was canceled.
	ImportStatusCancelled ImportStatus = "Cancelled"
	// ImportStatusCompleted means the [Import] completed successfully.
	ImportStatusCompleted ImportStatus = "Completed"
	// ImportStatusFailed means the [Import] encountered an error and did not complete.
	ImportStatusFailed ImportStatus = "Failed"
	// ImportStatusInProgress means the [Import] is in progress.
	ImportStatusInProgress ImportStatus = "InProgress"
	// ImportStatusPending means the [Import] is pending and has not yet started.
	ImportStatusPending ImportStatus = "Pending"
)

// ImportErrorMode specifies how errors are handled during an [Import]. Pass it to
// [IndexConnection.StartImport] as a string, e.g. string(ImportErrorModeContinue).
type ImportErrorMode string

const (
	// ImportErrorModeAbort stops the [Import] at the first record that fails. This is the default.
	ImportErrorModeAbort ImportErrorMode = "abort"
	// ImportErrorModeContinue skips records that fail and continues the [Import].
	ImportErrorModeContinue ImportErrorMode = "continue"
)

// Import represents the details and status of an import process.
type Import struct {
	// Id is the unique identifier of the import process.
	Id string `json:"id,omitempty"`
	// PercentComplete is the percentage of the import process that has been completed.
	PercentComplete float32 `json:"percent_complete,omitempty"`
	// RecordsImported is the total number of records successfully imported.
	RecordsImported int64 `json:"records_imported,omitempty"`
	// Status is the current [ImportStatus] of the import.
	Status ImportStatus `json:"status,omitempty"`
	// Uri is the URI of the source data for the import.
	Uri string `json:"uri,omitempty"`
	// CreatedAt is the time at which the import process was initiated.
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// FinishedAt is the time at which the import process finished (either successfully or with an error).
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	// Error is the error message associated with the failure, if the import failed.
	Error *string `json:"error,omitempty"`
}

// IntegratedRecord is a record to upsert into an index with integrated embedding via
// [IndexConnection.UpsertRecords]. It must carry exactly one of an "_id" or "id" field plus the
// index's field_map text field; other fields are stored as metadata.
type IntegratedRecord map[string]interface{}

// DocumentUsage reports the read units consumed by a document read operation.
type DocumentUsage struct {
	// ReadUnits is the number of read units consumed by the operation.
	ReadUnits int32 `json:"read_units"`
}

// UpsertDocumentsRequest holds the parameters for [IndexConnection.UpsertDocuments].
type UpsertDocumentsRequest struct {
	// Documents (Required) are the documents to upsert. Each [Document] must carry an "_id" field
	// and at least one field declared in the index schema; other fields are stored as filterable
	// metadata.
	Documents []Document `json:"documents"`
}

// UpsertDocumentsResponse is returned by [IndexConnection.UpsertDocuments].
type UpsertDocumentsResponse struct {
	// UpsertedCount is the number of documents successfully upserted.
	UpsertedCount int32 `json:"upserted_count"`
}

// DocumentScoringMethod defines how documents are scored against a query in
// [IndexConnection.SearchDocuments]. The Type field determines which other fields are used:
//   - "dense_vector": score by dense vector similarity. Requires Fields naming exactly one field, and Values.
//   - "sparse_vector": score by sparse vector similarity. Requires Fields naming exactly one field, and SparseValues.
//   - "text": score by BM25 text similarity. Requires Fields naming one or more fields, and Query.
//   - "query_string": score using a Lucene query string. Requires Query; Fields must be empty
//     (use field qualifiers inside the query string to target fields).
type DocumentScoringMethod struct {
	// Type is the scoring method type: "dense_vector", "sparse_vector", "text", or "query_string".
	Type string `json:"type"`
	// Fields are the names of the fields to score against.
	Fields []string `json:"fields,omitempty"`
	// Query is the text query for the "text" and "query_string" types. At most 10 KB, and must
	// not be empty after trimming whitespace.
	Query *string `json:"query,omitempty"`
	// Values are the dense vector values for the "dense_vector" type.
	Values *[]float32 `json:"values,omitempty"`
	// SparseValues are the [SparseValues] for the "sparse_vector" type.
	SparseValues *SparseValues `json:"sparse_values,omitempty"`
}

// SearchDocumentsRequest holds the parameters for [IndexConnection.SearchDocuments].
type SearchDocumentsRequest struct {
	// TopK (Required) is the number of top-ranked documents to return.
	TopK int32 `json:"top_k"`
	// ScoreBy (Required) are the scoring methods to rank documents by. A single method of any type
	// is valid; several methods may be combined only when every one is "text" or "query_string".
	ScoreBy []DocumentScoringMethod `json:"score_by"`
	// Filter (Optional) is a metadata filter expression restricting the documents searched.
	Filter map[string]interface{} `json:"filter,omitempty"`
	// IncludeFields (Optional) are the document fields to return on each match alongside "_id" and
	// "_score". When empty, no fields are returned; pass []string{"*"} to return every field.
	IncludeFields []string `json:"include_fields,omitempty"`
}

// DocumentMatch is a document returned from [IndexConnection.SearchDocuments], including the
// document ID, similarity score, and any requested fields. Score is nil when the score is not a
// finite number.
type DocumentMatch struct {
	// Id is the ID of the matched document.
	Id string `json:"_id"`
	// Score is the similarity score of the matched document, or nil when it is not a finite number.
	Score *float32 `json:"_score"`
	// Fields are the requested document fields, keyed by field name.
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

// SearchDocumentsResponse is returned by [IndexConnection.SearchDocuments].
type SearchDocumentsResponse struct {
	// Matches are the matching documents, ordered from most to least similar.
	Matches []DocumentMatch `json:"matches"`
	// Namespace is the namespace that was searched.
	Namespace string `json:"namespace"`
	// Usage is the [DocumentUsage] consumed by the search.
	Usage DocumentUsage `json:"usage"`
}

// FetchDocumentsRequest holds the parameters for [IndexConnection.FetchDocuments]. Exactly one of
// Ids or Filter must be provided.
type FetchDocumentsRequest struct {
	// Ids is a list of document IDs to fetch. Mutually exclusive with Filter.
	Ids []string `json:"ids,omitempty"`
	// Filter is a metadata filter expression selecting the documents to fetch. Must not be empty.
	// Mutually exclusive with Ids.
	Filter map[string]interface{} `json:"filter,omitempty"`
	// IncludeFields (Optional) are the document fields to return. When empty, all fields are returned.
	IncludeFields []string `json:"include_fields,omitempty"`
	// Limit (Optional) is the maximum number of documents per page for a fetch by Filter. Defaults to 100.
	Limit *int32 `json:"limit,omitempty"`
	// PaginationToken (Optional) is a token from a previous response to retrieve the next page.
	// Only valid together with Filter.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// FetchDocumentsResponse is returned by [IndexConnection.FetchDocuments]. Each fetched [Document]
// carries its "_id" alongside its field values.
type FetchDocumentsResponse struct {
	// Documents maps each document ID to the fetched [Document].
	Documents map[string]Document `json:"documents"`
	// Namespace is the namespace the documents were fetched from.
	Namespace string `json:"namespace"`
	// Pagination holds the token for the next page of a fetch by Filter, if more results remain.
	Pagination *Pagination `json:"pagination,omitempty"`
	// Usage is the [DocumentUsage] consumed by the fetch.
	Usage DocumentUsage `json:"usage"`
}

// DeleteDocumentsRequest holds the parameters for [IndexConnection.DeleteDocuments]. Exactly one of
// Ids, Filter, or DeleteAll must be provided.
type DeleteDocumentsRequest struct {
	// Ids is a list of document IDs to delete.
	Ids []string `json:"ids,omitempty"`
	// Filter is a metadata filter expression selecting the documents to delete. Must not be empty;
	// to delete every document in the namespace, set DeleteAll. Text-match operators
	// ($match_phrase, $match_all, $match_any) are not supported here.
	Filter map[string]interface{} `json:"filter,omitempty"`
	// DeleteAll deletes all documents in the namespace when true.
	DeleteAll bool `json:"delete_all,omitempty"`
}

// DeleteDocumentsResponse is returned by [IndexConnection.DeleteDocuments].
type DeleteDocumentsResponse struct {
	// MatchedRecords is the point-in-time number of documents that matched Filter when the delete
	// was accepted. It is nil for deletes by Ids or DeleteAll, and also when the count could not be
	// read in time; 0 means the filter matched no documents. Because the delete is applied
	// asynchronously, it doesn't guarantee how many documents are ultimately deleted.
	MatchedRecords *int32 `json:"matched_records,omitempty"`
}

// UpdateDocumentsRequest holds the parameters for [IndexConnection.UpdateDocuments]. Either
// Documents (per-document updates) or Filter with SetFields and/or RemoveFields (a filtered patch)
// must be provided; the two forms are mutually exclusive.
type UpdateDocumentsRequest struct {
	// Documents are partial document updates. Each [Document] must carry "_id"; other entries set
	// the named fields, and an optional "_remove_fields" entry ([]string) deletes fields. Null
	// values are rejected; list the field in "_remove_fields" to delete it.
	Documents []Document `json:"documents,omitempty"`
	// Filter is a metadata filter expression selecting the documents to patch. Must not be empty.
	// Text-match operators ($match_phrase, $match_all, $match_any) are not supported here.
	Filter map[string]interface{} `json:"filter,omitempty"`
	// SetFields are the fields to set on every document matching Filter.
	SetFields map[string]interface{} `json:"set_fields,omitempty"`
	// RemoveFields are the names of the fields to remove from every document matching Filter.
	RemoveFields []string `json:"remove_fields,omitempty"`
}

// UpdateDocumentsResponse is returned by [IndexConnection.UpdateDocuments].
type UpdateDocumentsResponse struct {
	// MatchedRecords is the point-in-time number of documents that matched Filter when the update
	// was accepted. It is nil for per-document updates (Documents), and also when the count could
	// not be read in time; 0 means the filter matched no documents. Because the update is applied
	// asynchronously, it doesn't guarantee how many documents are ultimately updated.
	MatchedRecords *int32 `json:"matched_records,omitempty"`
}

// ListDocumentsRequest holds the parameters for [IndexConnection.ListDocuments].
type ListDocumentsRequest struct {
	// Prefix (Optional) is a prefix to filter document IDs.
	Prefix *string `json:"prefix,omitempty"`
	// Limit (Optional) is the maximum number of documents per page. Defaults to 100.
	Limit *int32 `json:"limit,omitempty"`
	// PaginationToken (Optional) is a token from a previous response to retrieve the next page.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// ListedDocument identifies a document returned by [IndexConnection.ListDocuments].
type ListedDocument struct {
	// Id is the ID of the document.
	Id string `json:"_id"`
}

// ListDocumentsResponse is returned by [IndexConnection.ListDocuments].
type ListDocumentsResponse struct {
	// Documents are the listed documents, in sorted order by ID.
	Documents []ListedDocument `json:"documents"`
	// Namespace is the namespace the documents were listed from.
	Namespace string `json:"namespace"`
	// Pagination holds the token for the next page, if more results remain.
	Pagination *Pagination `json:"pagination,omitempty"`
	// Usage is the [DocumentUsage] consumed by the list operation.
	Usage DocumentUsage `json:"usage"`
}

// SearchRecordsRequest represents a search request for records in a specific namespace.
type SearchRecordsRequest struct {
	// Query is the [SearchRecordsQuery] to search with.
	Query SearchRecordsQuery `json:"query"`
	// Fields are the fields to return in the search results. If not specified, all fields are returned.
	Fields *[]string `json:"fields,omitempty"`
	// Rerank holds the [SearchRecordsRerank] parameters for reranking the initial search results.
	Rerank *SearchRecordsRerank `json:"rerank,omitempty"`
}

// SearchRecordsQuery represents the query parameters for searching records.
type SearchRecordsQuery struct {
	// TopK (Required) is the number of similar records to return, from 1 to 10000.
	TopK int32 `json:"top_k"`
	// Filter is the metadata filter to apply.
	Filter *map[string]interface{} `json:"filter,omitempty"`
	// Id is the unique ID of the vector to be used as a query vector.
	Id *string `json:"id,omitempty"`
	// Inputs is the query input to embed with the index's integrated embedding model, e.g.
	// {"text": "query text"}. Only supported on indexes with integrated embedding.
	Inputs *map[string]interface{} `json:"inputs,omitempty"`
	// Vector is the [SearchRecordsVector] representation of the query.
	Vector *SearchRecordsVector `json:"vector,omitempty"`
	// MatchTerms are the [SearchMatchTerms] that must be present in the text of each search hit.
	MatchTerms *SearchMatchTerms `json:"match_terms,omitempty"`
}

// SearchMatchTerms represents the terms to match in the text of each search hit.
type SearchMatchTerms struct {
	// Strategy is the strategy for matching terms in the text. Currently, only "all" is supported,
	// which means all specified terms must be present. Defaults to "all" when nil.
	Strategy *string `json:"strategy,omitempty"`
	// Terms is a list of terms that must be present in the text of each search hit, based on Strategy.
	Terms *[]string `json:"terms,omitempty"`
}

// SearchRecordsRerank represents the parameters for reranking search results.
// See [reranking models] for the available models, their parameters, and supported rank fields.
//
// [reranking models]: https://docs.pinecone.io/guides/search/rerank-results#reranking-models
type SearchRecordsRerank struct {
	// Model is the name of the reranking model to use.
	Model string `json:"model"`
	// RankFields (Required) are the field(s) to consider for reranking. Must contain at least one
	// field. The number of fields supported is model-specific.
	RankFields []string `json:"rank_fields"`
	// Parameters are additional model-specific parameters.
	Parameters *map[string]interface{} `json:"parameters,omitempty"`
	// Query is the query to rerank documents against. If set, it overrides the query input
	// provided at the top level. Required when the search uses Vector or Id, since there is no
	// query text to rerank against.
	Query *string `json:"query,omitempty"`
	// TopN is the number of top results to return after reranking. Defaults to TopK.
	TopN *int32 `json:"top_n,omitempty"`
}

// Hit represents a record whose vector values are similar to the provided search query.
type Hit struct {
	// Id is the record ID of the search hit.
	Id string `json:"_id"`
	// Score is the similarity score of the returned record.
	Score float32 `json:"_score"`
	// Fields are the selected record fields associated with the search hit.
	Fields map[string]interface{} `json:"fields"`
}

// SearchRecordsResponse represents the response of a records search.
type SearchRecordsResponse struct {
	// Result contains the [Hit] responses for the search.
	Result struct {
		// Hits are the matching records, ordered from most to least similar.
		Hits []Hit `json:"hits"`
	} `json:"result"`
	// Usage is the [SearchUsage] consumed by the search operation.
	Usage SearchUsage `json:"usage"`
}

// SearchRecordsVector represents the vector data used in a search request.
type SearchRecordsVector struct {
	// SparseIndices are the sparse embedding indices.
	SparseIndices *[]int32 `json:"sparse_indices,omitempty"`
	// SparseValues are the sparse embedding values.
	SparseValues *[]float32 `json:"sparse_values,omitempty"`
	// Values are the dense vector values.
	Values *[]float32 `json:"values,omitempty"`
}

// SearchUsage represents the resource usage details of a search operation.
type SearchUsage struct {
	// ReadUnits is the number of read units consumed by the operation.
	ReadUnits int32 `json:"read_units"`
	// EmbedTotalTokens is the number of embedding tokens consumed by the operation.
	EmbedTotalTokens *int32 `json:"embed_total_tokens,omitempty"`
	// RerankUnits is the number of rerank units consumed by the operation.
	RerankUnits *int32 `json:"rerank_units,omitempty"`
}

// ModelInfoList represents a list of [ModelInfo] objects describing the models hosted by Pinecone.
type ModelInfoList struct {
	// Models are the [ModelInfo] objects describing each model.
	Models *[]ModelInfo `json:"models,omitempty"`
}

// ModelInfo describes a model hosted by Pinecone, including its type, supported parameters, and other details.
type ModelInfo struct {
	// DefaultDimension is the default embedding model dimension (dense embedding models only).
	DefaultDimension *int32 `json:"default_dimension,omitempty"`
	// MaxBatchSize is the maximum batch size (number of sequences) supported by the model.
	MaxBatchSize *int32 `json:"max_batch_size,omitempty"`
	// MaxSequenceLength is the maximum tokens per sequence supported by the model.
	MaxSequenceLength *int32 `json:"max_sequence_length,omitempty"`
	// Modality is the modality of the model (e.g. "text").
	Modality *string `json:"modality,omitempty"`
	// Model is the name of the model.
	Model string `json:"model"`
	// ProviderName is the name of the provider of the model (e.g. "Pinecone", "NVIDIA").
	ProviderName *string `json:"provider_name,omitempty"`
	// ShortDescription is a summary of the model.
	ShortDescription string `json:"short_description"`
	// SupportedDimensions are the dimensions supported by the model (dense embedding models only).
	SupportedDimensions *[]int32 `json:"supported_dimensions,omitempty"`
	// SupportedMetrics are the distance metrics supported by the model for similarity search.
	SupportedMetrics *[]IndexMetric `json:"supported_metrics,omitempty"`
	// SupportedParameters are the parameters supported by the model, including value constraints.
	SupportedParameters *[]SupportedParameter `json:"supported_parameters,omitempty"`
	// Type is the type of model (e.g. "embed" or "rerank").
	Type string `json:"type"`
	// VectorType is whether the embedding model produces "dense" or "sparse" embeddings.
	VectorType *string `json:"vector_type,omitempty"`
}

// SupportedParameter describes a parameter supported by the model, including parameter value constraints.
type SupportedParameter struct {
	// AllowedValues are the allowed parameter values when Type is "one_of".
	AllowedValues *[]SupportedParameterValue `json:"allowed_values,omitempty"`
	// Default is the default value for the parameter when the parameter is optional.
	Default *SupportedParameterValue `json:"default,omitempty"`
	// Max is the maximum allowed value (inclusive) when Type is "numeric_range".
	Max *float32 `json:"max,omitempty"`
	// Min is the minimum allowed value (inclusive) when Type is "numeric_range".
	Min *float32 `json:"min,omitempty"`
	// Parameter is the name of the parameter.
	Parameter string `json:"parameter"`
	// Required indicates whether the parameter is required.
	Required bool `json:"required"`
	// Type is the parameter type: "one_of", "numeric_range", or "any". For "one_of", AllowedValues
	// is set and the value must be one of them; "one_of" is only compatible with ValueType "string"
	// or "integer". For "numeric_range", Min and Max are set and the value must adhere to ValueType
	// and fall within [Min, Max]. For "any", any value adhering to ValueType is allowed.
	Type string `json:"type"`
	// ValueType is the type of value the parameter accepts: "string", "integer", "float", or "boolean".
	ValueType string `json:"value_type"`
}

// SupportedParameterValue is a tagged union type representing the value of a [SupportedParameter].
type SupportedParameterValue struct {
	// StringValue is set if the parameter accepts strings.
	StringValue *string
	// IntValue is set if the parameter accepts integers.
	IntValue *int32
	// FloatValue is set if the parameter accepts floating point numbers.
	FloatValue *float32
	// BoolValue is set if the parameter accepts true/false input.
	BoolValue *bool
}

// UnmarshalJSON decodes a JSON string, number, or boolean into the matching field of
// [SupportedParameterValue]. It returns an error for any other JSON type.
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

// Backup describes the configuration and status of a Pinecone backup.
//
// The Dimension and Metric fields are derived from Schema for backward compatibility and are
// deprecated; see the field docs.
type Backup struct {
	// BackupId is the unique identifier of the backup.
	BackupId string `json:"backup_id"`
	// Cloud is the cloud provider where the backup is stored.
	Cloud string `json:"cloud"`
	// CreatedAt is the timestamp when the backup was created.
	CreatedAt *string `json:"created_at,omitempty"`
	// Description is an optional description providing context for the backup.
	Description *string `json:"description,omitempty"`
	// Name is an optional user-defined name for the backup.
	Name *string `json:"name,omitempty"`
	// NamespaceCount is the number of namespaces in the backup.
	NamespaceCount *int64 `json:"namespace_count,omitempty"`
	// RecordCount is the total number of records in the backup.
	RecordCount *int64 `json:"record_count,omitempty"`
	// Region is the cloud region where the backup is stored.
	Region string `json:"region"`
	// Schema is the typed [IndexSchema] of the source index, when reported.
	Schema *IndexSchema `json:"schema,omitempty"`
	// SizeBytes is the size of the backup in bytes.
	SizeBytes *int64 `json:"size_bytes,omitempty"`
	// SourceIndexDeletedAt is the deletion timestamp of the source index, or nil while that index
	// is still active.
	SourceIndexDeletedAt *time.Time `json:"source_index_deleted_at,omitempty"`
	// SourceIndexId is the ID of the index from which the backup was taken.
	SourceIndexId string `json:"source_index_id"`
	// SourceIndexName is the name of the index from which the backup was taken.
	SourceIndexName string `json:"source_index_name"`
	// Status is the current status of the backup: "Initializing", "Ready", or "InitializationFailed".
	Status string `json:"status"`
	// Tags are the custom user [IndexTags] of the backup.
	Tags *IndexTags `json:"tags,omitempty"`

	// Dimension is the dimension of the source index's dense vector field.
	//
	// Deprecated: Derived from Schema for backward compatibility. Use the [DenseVectorField] in Schema instead.
	Dimension *int32 `json:"dimension,omitempty"`
	// Metric is the distance metric of the source index's dense vector field.
	//
	// Deprecated: Derived from Schema for backward compatibility. Use the [DenseVectorField] in Schema instead.
	Metric *IndexMetric `json:"metric,omitempty"`
}

// BackupList contains a paginated list of backups.
type BackupList struct {
	// Data is the list of [Backup] records.
	Data []*Backup `json:"data"`
	// Pagination holds the token for fetching the next page of results. It is nil when there are
	// no more results; otherwise pass Pagination.Next as the next request's PaginationToken.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// RestoreJob describes the status of a restore job.
type RestoreJob struct {
	// BackupId is the ID of the backup used for the restore.
	BackupId string `json:"backup_id"`
	// CompletedAt is the timestamp when the restore job finished; nil while the job is pending.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// CreatedAt is the timestamp when the restore job started.
	CreatedAt time.Time `json:"created_at"`
	// PercentComplete is 100 once the restore job has completed, and nil otherwise. Intermediate
	// progress is not reported; use Status to track the job.
	PercentComplete *float32 `json:"percent_complete,omitempty"`
	// RestoreJobId is the unique identifier of the restore job.
	RestoreJobId string `json:"restore_job_id"`
	// Status is the status of the restore job: "Pending", "Completed", "Failed", or "Cancelled".
	Status string `json:"status"`
	// TargetIndexId is the ID of the index into which data is being restored.
	TargetIndexId string `json:"target_index_id"`
	// TargetIndexName is the name of the index into which data is being restored.
	TargetIndexName string `json:"target_index_name"`
}

// RestoreJobList contains a paginated list of restore jobs.
type RestoreJobList struct {
	// Data is the list of [RestoreJob] records.
	Data []*RestoreJob `json:"data"`
	// Pagination holds the token for fetching the next page of results. It is nil when there are
	// no more results; otherwise pass Pagination.Next as the next request's PaginationToken.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Project represents the details of a project.
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

// Organization represents the details of an organization.
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

// APIKey represents the details of an API key without the secret.
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

// APIKeyWithSecret represents an API key together with its secret value. The secret is returned only
// when the key is created and cannot be retrieved later; store it securely and never log it.
type APIKeyWithSecret struct {
	// The details of an [APIKey], without the secret.
	Key APIKey `json:"key"`

	// The value to use as an API key. New keys will have the format `"pckey_<public-label>_<unique-key>"`.
	// The entire string should be used when authenticating.
	Value string `json:"value"`
}

// PrincipalType is the kind of principal that receives permissions from a [RoleBinding].
type PrincipalType string

const (
	// PrincipalTypeUser is a user who is a member of the organization.
	PrincipalTypeUser PrincipalType = "user"
	// PrincipalTypeServiceAccount is a service account.
	PrincipalTypeServiceAccount PrincipalType = "service_account"
	// PrincipalTypeAPIKey is a project API key.
	PrincipalTypeAPIKey PrincipalType = "api_key"
	// PrincipalTypeInvite is a pending invite; its bindings apply once the invite is accepted.
	PrincipalTypeInvite PrincipalType = "invite"
)

// ResourceType is the kind of resource scope a [RoleBinding] applies to.
type ResourceType string

const (
	// ResourceTypeOrganization scopes a binding to the organization.
	ResourceTypeOrganization ResourceType = "organization"
	// ResourceTypeProject scopes a binding to a single project.
	ResourceTypeProject ResourceType = "project"
)

// RoleBinding grants a role to a principal (a user, service account, API key,
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

// RoleBindingInput describes a role to grant to a principal when creating a
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

// RoleBindingList contains a paginated list of role bindings.
type RoleBindingList struct {
	// Data is the list of [RoleBinding] records.
	Data []*RoleBinding `json:"data"`
	// Pagination holds the token for fetching the next page of results. It is nil when there are
	// no more results; otherwise pass Pagination.Next as the next request's PaginationToken.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// ServiceAccount represents a service account. The OAuth client secret is not included;
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

// ServiceAccountWithSecret represents a service account together with a newly issued
// OAuth client secret. The secret is returned only once — at creation or secret
// rotation — and cannot be retrieved later. Treat ClientSecret as a credential: store
// it securely and never log it.
type ServiceAccountWithSecret struct {
	// The details of the service account, without the secret.
	ServiceAccount ServiceAccount `json:"service_account"`

	// The OAuth client secret. Returned exactly once. Store it securely and never log it.
	ClientSecret string `json:"client_secret"`
}

// ServiceAccountList contains a paginated list of service accounts.
type ServiceAccountList struct {
	// Data is the list of [ServiceAccount] records.
	Data []*ServiceAccount `json:"data"`
	// Pagination holds the token for fetching the next page of results. It is nil when there are
	// no more results; otherwise pass Pagination.Next as the next request's PaginationToken.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// InviteStatus is the lifecycle status of an [Invite].
type InviteStatus string

const (
	// InviteStatusPending means the invite has not been accepted and has not expired.
	InviteStatusPending InviteStatus = "pending"
	// InviteStatusExpired means the invite expired without being accepted; it can be resent.
	InviteStatusExpired InviteStatus = "expired"
	// InviteStatusProcessed means the invite has been accepted.
	InviteStatusProcessed InviteStatus = "processed"
)

// Invite represents an invitation to join the organization.
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

// InviteList contains a paginated list of invites.
type InviteList struct {
	// Data is the list of [Invite] records.
	Data []*Invite `json:"data"`
	// Pagination holds the token for fetching the next page of results. It is nil when there are
	// no more results; otherwise pass Pagination.Next as the next request's PaginationToken.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// User represents a user who is a member of the organization.
type User struct {
	// The unique ID of the user. Use this as the principal ID when creating or
	// querying role bindings for the user.
	Id string `json:"id"`

	// The user's email address.
	Email string `json:"email"`

	// (Optional) The user's display name. Nil if the user has not set one.
	Name *string `json:"name,omitempty"`
}

// UserList contains a paginated list of users.
type UserList struct {
	// Data is the list of [User] records.
	Data []*User `json:"data"`
	// Pagination holds the token for fetching the next page of results. It is nil when there are
	// no more results; otherwise pass Pagination.Next as the next request's PaginationToken.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// MetadataSchema configures which metadata fields Pinecone's metadata index covers: when a schema is
// present, only the fields in Fields (each with Filterable true) are indexed. What happens when no
// schema is given depends on where it is used; see the field that takes it. Filterable false is not
// supported.
type MetadataSchema struct {
	// Fields maps each metadata field name to its [MetadataSchemaField] configuration. Each name
	// must be a valid metadata field name.
	Fields map[string]MetadataSchemaField `json:"fields"`
}

// MetadataSchemaField is the configuration of a single metadata field in a [MetadataSchema].
type MetadataSchemaField struct {
	// Filterable reports whether the field is indexed and can be used in filters. Only true is
	// supported.
	Filterable bool `json:"filterable,omitempty"`
}
