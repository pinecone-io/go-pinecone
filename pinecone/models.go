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

// [IndexSpec] is the infrastructure specification (pods vs serverless) of a Pinecone [Index].
type IndexSpec struct {
	Pod        *PodSpec        `json:"pod,omitempty"`
	Serverless *ServerlessSpec `json:"serverless,omitempty"`
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
//   - VectorType:  The index vector type associated with the model. If 'dense', the vector dimension must be specified.
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

// [Index] is a Pinecone [Index] object. Can be either a pod-based or a serverless [Index], depending on the [IndexSpec].
type Index struct {
	Name               string             `json:"name"`
	Host               string             `json:"host"`
	Metric             IndexMetric        `json:"metric"`
	VectorType         string             `json:"vector_type"`
	DeletionProtection DeletionProtection `json:"deletion_protection,omitempty"`
	Dimension          *int32             `json:"dimension"`
	Spec               *IndexSpec         `json:"spec,omitempty"`
	Status             *IndexStatus       `json:"status,omitempty"`
	Tags               *IndexTags         `json:"tags,omitempty"`
	Embed              *IndexEmbed        `json:"embed,omitempty"`
}

// [Collection] is a Pinecone [collection entity]. Only available for pod-based Indexes.
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
)

// [PodSpecMetadataConfig] represents the metadata fields to be indexed when a Pinecone [Index] is created.
type PodSpecMetadataConfig struct {
	Indexed *[]string `json:"indexed,omitempty"`
}

// [PodSpec] is the infrastructure specification of a pod-based Pinecone [Index]. Only available for pod-based Indexes.
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
type ServerlessSpec struct {
	Cloud  Cloud  `json:"cloud"`
	Region string `json:"region"`
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
type NamespaceDescription struct {
	Name        string `json:"name"`
	RecordCount uint64 `json:"record_count"`
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
	TopK   int32                   `json:"top_k"`
	Filter *map[string]interface{} `json:"filter,omitempty"`
	Id     *string                 `json:"id,omitempty"`
	Inputs *map[string]interface{} `json:"inputs,omitempty"`
	Vector *SearchRecordsVector    `json:"vector,omitempty"`
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
//   - SizeBytes: Size of the backup in bytes.
//   - SourceIndexId: ID of the index.
//   - SourceIndexName: Name of the index from which the backup was taken.
//   - Status: Current status of the backup (e.g., Initializing, Ready, Failed).
//   - Tags: Custom user tags added to an index. Keys must be 80 characters or less. Values must be 120 characters or less. Keys must be alphanumeric, '_', or '-'. Values must be alphanumeric, ';', '@', '_', '-', '.', '+', or ' '. To unset a key, set the value to an empty string.
type Backup struct {
	BackupId        string       `json:"backup_id"`
	Cloud           string       `json:"cloud"`
	CreatedAt       *string      `json:"created_at,omitempty"`
	Description     *string      `json:"description,omitempty"`
	Dimension       *int32       `json:"dimension,omitempty"`
	Metric          *IndexMetric `json:"metric,omitempty"`
	Name            *string      `json:"name,omitempty"`
	NamespaceCount  *int         `json:"namespace_count,omitempty"`
	RecordCount     *int         `json:"record_count,omitempty"`
	Region          string       `json:"region"`
	SizeBytes       *int         `json:"size_bytes,omitempty"`
	SourceIndexId   string       `json:"source_index_id"`
	SourceIndexName string       `json:"source_index_name"`
	Status          string       `json:"status"`
	Tags            *IndexTags   `json:"tags,omitempty"`
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
