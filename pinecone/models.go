package pinecone

import "google.golang.org/protobuf/types/known/structpb"

// IndexMetric is the [distance metric] to be used by similarity search against a Pinecone Index.
//
// [distance metric]: https://docs.pinecone.io/guides/indexes/understanding-indexes#distance-metrics
type IndexMetric string

const (
	Cosine     IndexMetric = "cosine" // Default similarity, ideal for textual data
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

// Index is a Pinecone Index object. Can be either a pod-based or a serverless Index, depending on passed IndexSpec.
type Index struct {
	Name      string       `json:"name"`
	Dimension int32        `json:"dimension"`
	Host      string       `json:"host"`
	Metric    IndexMetric  `json:"metric"`
	Spec      *IndexSpec   `json:"spec,omitempty"`
	Status    *IndexStatus `json:"status,omitempty"`
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

// PodSpecMetadataConfig is the metadata fields to be indexed when a Pinecone Index is created.
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

// Filter is the structure that holds the conditions for a query sent to a Pinecone Index,
// e.g. 'give me all vectors where "author" is "John"'.
type Filter = structpb.Struct

// Metadata defines [the conditions for a query] sent to a Pinecone Index, or the additional metadata [to be indexed with a vector].
//
// [the conditions for a query]: https://docs.pinecone.io/guides/data/filter-with-metadata#querying-an-index-with-metadata-filters
// [to be indexed with a vector]: https://docs.pinecone.io/guides/data/filter-with-metadata#inserting-metadata-into-an-index
type Metadata = structpb.Struct
