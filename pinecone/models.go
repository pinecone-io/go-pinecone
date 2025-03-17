package pinecone

import (
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

// [SparseValues] is a sparse vector objects, most commonly used for [hybrid search].
//
// [hybrid search]: https://docs.pinecone.io/guides/data/understanding-hybrid-search#hybrid-search-in-pinecone
type SparseValues struct {
	Indices []uint32  `json:"indices,omitempty"`
	Values  []float32 `json:"values,omitempty"`
}

// [NamespaceSummary] is a summary of stats for a Pinecone [namespace].
//
// [namespace]: https://docs.pinecone.io/guides/indexes/use-namespaces
type NamespaceSummary struct {
	VectorCount uint32 `json:"vector_count"`
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
//
// [generating embeddings]: https://docs.pinecone.io/guides/inference/generate-embeddings#3-generate-embeddings
type Embedding struct {
	Values *[]float32 `json:"values,omitempty"`
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

// ImportErrorMode specifies how errors are handled during an [Import].
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

// SearchRecordsRequest A search request for records in a specific namespace.
type SearchRecordsRequest struct {
	// Fields The fields to return in the search results.
	Fields *[]string `json:"fields,omitempty"`

	// Query The query inputs to search with.
	Query SearchRecordsQuery `json:"query"`

	// Rerank Parameters for reranking the initial search results.
	Rerank *SearchRecordsRerank `json:"rerank,omitempty"`
}

type SearchRecordsQuery struct {
	// Filter The filter to apply.
	Filter *map[string]interface{} `json:"filter,omitempty"`

	// Id The unique ID of the vector to be used as a query vector.
	Id     *string                 `json:"id,omitempty"`
	Inputs *map[string]interface{} `json:"inputs,omitempty"`

	// TopK The number of results to return for each search.
	TopK   int32                `json:"top_k"`
	Vector *SearchRecordsVector `json:"vector,omitempty"`
}

type SearchRecordsRerank struct {
	// Model The name of the [reranking model](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models) to use.
	Model string `json:"model"`

	// Parameters Additional model-specific parameters. Refer to the [model guide](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models) for available model parameters.
	Parameters *map[string]interface{} `json:"parameters,omitempty"`

	// Query The query to rerank documents against. If a specific rerank query is specified,  it overwrites the query input that was provided at the top level.
	Query *string `json:"query,omitempty"`

	// RankFields The field(s) to consider for reranking. If not provided, the default is `["text"]`.
	//
	// The number of fields supported is [model-specific](https://docs.pinecone.io/guides/inference/understanding-inference#reranking-models).
	RankFields []string `json:"rank_fields"`

	// TopN The number of top results to return after reranking. Defaults to top_k.
	TopN *int32 `json:"top_n,omitempty"`
}

// Hit A record whose vector values are similar to the provided search query.
type Hit struct {
	// Id The record id of the search hit.
	Id string `json:"_id"`

	// Score The similarity score of the returned record.
	Score float32 `json:"_score"`

	// Fields The selected record fields associated with the search hit.
	Fields map[string]interface{} `json:"fields"`
}

// SearchRecordsResponse The records search response.
type SearchRecordsResponse struct {
	Result struct {
		// Hits The hits for the search document request.
		Hits []Hit `json:"hits"`
	} `json:"result"`
	Usage SearchUsage `json:"usage"`
}

// SearchRecordsVector defines model for SearchRecordsVector.
type SearchRecordsVector struct {
	// SparseIndices The sparse embedding indices.
	SparseIndices *[]int32 `json:"sparse_indices,omitempty"`

	// SparseValues The sparse embedding values.
	SparseValues *[]float32 `json:"sparse_values,omitempty"`

	// Values This is the vector data included in the request.
	Values *[]float32 `json:"values,omitempty"`
}

// SearchUsage defines model for SearchUsage.
type SearchUsage struct {
	// EmbedTotalTokens The number of embedding tokens consumed by this operation.
	EmbedTotalTokens *int32 `json:"embed_total_tokens,omitempty"`

	// ReadUnits The number of read units consumed by this operation.
	ReadUnits int32 `json:"read_units"`

	// RerankUnits The number of rerank units consumed by this operation.
	RerankUnits *int32 `json:"rerank_units,omitempty"`
}
