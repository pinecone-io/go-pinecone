package pinecone

import (
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// IndexMetric is the [distance metric] to be used by similarity search against a Pinecone Index.
//
// [distance metric]: https://docs.pinecone.io/guides/indexes/understanding-indexes#distance-metrics
type IndexMetric string

const (
	Cosine     IndexMetric = "cosine"     // Default distance metric, ideal for textual data
	Dotproduct IndexMetric = "dotproduct" // Ideal for hybrid search
	Euclidean  IndexMetric = "euclidean"  // Ideal for distance-based data (e.g. lat/long points)
)

// IndexStatusState is the state of a Pinecone Index.
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

// DeletionProtection determines whether [deletion protection] is "enabled" or "disabled" for the index.
// When "enabled", the index cannot be deleted. Defaults to "disabled".
//
// [deletion protection]: http://docs.pinecone.io/guides/indexes/prevent-index-deletion
type DeletionProtection string

const (
	DeletionProtectionEnabled  DeletionProtection = "enabled"
	DeletionProtectionDisabled DeletionProtection = "disabled"
)

// Cloud is the [cloud provider] to be used for a Pinecone serverless Index.
//
// [cloud provider]: https://docs.pinecone.io/troubleshooting/available-cloud-regions
type Cloud string

const (
	Aws   Cloud = "aws"
	Azure Cloud = "azure"
	Gcp   Cloud = "gcp"
)

// IndexStatus is the status of a Pinecone Index.
type IndexStatus struct {
	Ready bool             `json:"ready"`
	State IndexStatusState `json:"state"`
}

// IndexSpec is the infrastructure specification (pods vs serverless) of a Pinecone Index.
type IndexSpec struct {
	Pod        *PodSpec        `json:"pod,omitempty"`
	Serverless *ServerlessSpec `json:"serverless,omitempty"`
}

// Index is a Pinecone Index object. Can be either a pod-based or a serverless Index, depending on the IndexSpec.
type Index struct {
	Name               string             `json:"name"`
	Dimension          int32              `json:"dimension"`
	Host               string             `json:"host"`
	Metric             IndexMetric        `json:"metric"`
	DeletionProtection DeletionProtection `json:"deletion_protection,omitempty"`
	Spec               *IndexSpec         `json:"spec,omitempty"`
	Status             *IndexStatus       `json:"status,omitempty"`
}

// Collection is a Pinecone [Collection object]. Only available for pod-based Indexes.
//
// [Collection object]: https://docs.pinecone.io/guides/indexes/understanding-collections
type Collection struct {
	Name        string           `json:"name"`
	Size        int64            `json:"size"`
	Status      CollectionStatus `json:"status"`
	Dimension   int32            `json:"dimension"`
	VectorCount int32            `json:"vector_count"`
	Environment string           `json:"environment"`
}

// CollectionStatus is the status of a Pinecone Collection.
type CollectionStatus string

const (
	CollectionStatusInitializing CollectionStatus = "Initializing"
	CollectionStatusReady        CollectionStatus = "Ready"
	CollectionStatusTerminating  CollectionStatus = "Terminating"
)

// PodSpecMetadataConfig represents the metadata fields to be indexed when a Pinecone Index is created.
type PodSpecMetadataConfig struct {
	Indexed *[]string `json:"indexed,omitempty"`
}

// PodSpec is the infrastructure specification of a pod-based Pinecone Index. Only available for pod-based Indexes.
type PodSpec struct {
	Environment      string                 `json:"environment"`
	PodType          string                 `json:"pod_type"`
	PodCount         int32                  `json:"pod_count"`
	Replicas         int32                  `json:"replicas"`
	ShardCount       int32                  `json:"shard_count"`
	SourceCollection *string                `json:"source_collection,omitempty"`
	MetadataConfig   *PodSpecMetadataConfig `json:"metadata_config,omitempty"`
}

// ServerlessSpec is the infrastructure specification of a serverless Pinecone Index. Only available for serverless Indexes.
type ServerlessSpec struct {
	Cloud  Cloud  `json:"cloud"`
	Region string `json:"region"`
}

// Vector is a [dense or sparse vector object] with optional metadata.
//
// [dense or sparse vector object]: https://docs.pinecone.io/guides/get-started/key-concepts#dense-vector
type Vector struct {
	Id           string        `json:"id"`
	Values       []float32     `json:"values,omitempty"`
	SparseValues *SparseValues `json:"sparse_values,omitempty"`
	Metadata     *Metadata     `json:"metadata,omitempty"`
}

// ScoredVector is a vector with an associated similarity score calculated according to the distance metric of the
// Index.
type ScoredVector struct {
	Vector *Vector `json:"vector,omitempty"`
	Score  float32 `json:"score"`
}

// SparseValues is a sparse vector objects, most commonly used for [hybrid search].
//
// [hybrid search]: https://docs.pinecone.io/guides/data/understanding-hybrid-search#hybrid-search-in-pinecone
type SparseValues struct {
	Indices []uint32  `json:"indices,omitempty"`
	Values  []float32 `json:"values,omitempty"`
}

// NamespaceSummary is a summary of stats for a Pinecone [namespace].
//
// [namespace]: https://docs.pinecone.io/guides/indexes/use-namespaces
type NamespaceSummary struct {
	VectorCount uint32 `json:"vector_count"`
}

// Usage is the usage stats ([Read Units]) for a Pinecone Index.
//
// [Read Units]: https://docs.pinecone.io/guides/organizations/manage-cost/understanding-cost#serverless-indexes
type Usage struct {
	ReadUnits uint32 `json:"read_units"`
}

// RerankUsage is the usage stats ([Rerank Units]) for a reranking request.
//
// [Rerank Units]: https://docs.pinecone.io/guides/organizations/manage-cost/understanding-cost#rerank
type RerankUsage struct {
	RerankUnits *int `json:"rerank_units,omitempty"`
}

// MetadataFilter represents the [metadata filters] attached to a Pinecone request.
// These optional metadata filters are applied to query and deletion requests.
//
// [metadata filters]: https://docs.pinecone.io/guides/data/filter-with-metadata#querying-an-index-with-metadata-filters
type MetadataFilter = structpb.Struct

// Metadata represents optional,
// additional information that can be [attached to, or updated for, a vector] in a Pinecone Index.
//
// [attached to, or updated for, a vector]: https://docs.pinecone.io/guides/data/filter-with-metadata#inserting-metadata-into-an-index
type Metadata = structpb.Struct

// ImportStatus represents the status of an import operation.
//
// Values:
//   - Cancelled: The import was canceled.
//   - Completed: The import completed successfully.
//   - Failed: The import encountered an error and did not complete successfully.
//   - InProgress: The import is currently in progress.
//   - Pending: The import is pending and has not yet started.
type ImportStatus string

const (
	Cancelled  ImportStatus = "Cancelled"
	Completed  ImportStatus = "Completed"
	Failed     ImportStatus = "Failed"
	InProgress ImportStatus = "InProgress"
	Pending    ImportStatus = "Pending"
)

// ImportErrorMode specifies how errors are handled during an import.
//
// Values:
//   - Abort: The import process will abort upon encountering an error.
//   - Continue: The import process will continue, skipping over records that produce errors.
type ImportErrorMode string

const (
	Abort    ImportErrorMode = "abort"
	Continue ImportErrorMode = "continue"
)

// Import represents the details and status of a bulk import process.
//
// Fields:
//   - Id: The unique identifier of the import process.
//   - PercentComplete: The percentage of the import process that has been completed.
//   - RecordsImported: The total number of records successfully imported.
//   - Status: The current status of the import (e.g., "InProgress", "Completed", "Failed").
//   - Uri: The URI of the source data for the import.
//   - CreatedAt: The time at which the import process was initiated.
//   - FinishedAt: The time at which the import process finished (either successfully or with an error).
//   - Error: If the import failed, contains the error message associated with the failure.
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
