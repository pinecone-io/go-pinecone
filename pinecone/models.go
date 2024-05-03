package pinecone

import "google.golang.org/protobuf/types/known/structpb"

type IndexMetric string

const (
	Cosine     IndexMetric = "cosine"
	Dotproduct IndexMetric = "dotproduct"
	Euclidean  IndexMetric = "euclidean"
)

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

type Cloud string

const (
	Aws   Cloud = "aws"
	Azure Cloud = "azure"
	Gcp   Cloud = "gcp"
)

type IndexStatus struct {
	Ready bool             `json:"ready"`
	State IndexStatusState `json:"state"`
}

type IndexSpec struct {
	Pod        *PodSpec        `json:"pod,omitempty"`
	Serverless *ServerlessSpec `json:"serverless,omitempty"`
}

type Index struct {
	Name      string       `json:"name"`
	Dimension int32        `json:"dimension"`
	Host      string       `json:"host"`
	Metric    IndexMetric  `json:"metric"`
	Spec      *IndexSpec   `json:"spec,omitempty"`
	Status    *IndexStatus `json:"status,omitempty"`
}

type Collection struct {
	Name        string           `json:"name"`
	Size        *int64           `json:"size,omitempty"`
	Status      CollectionStatus `json:"status"`
	Dimension   *int32           `json:"dimension,omitempty"`
	VectorCount *int32           `json:"vector_count,omitempty"`
	Environment string           `json:"environment"`
}

type CollectionStatus string

const (
	CollectionStatusInitializing CollectionStatus = "Initializing"
	CollectionStatusReady        CollectionStatus = "Ready"
	CollectionStatusTerminating  CollectionStatus = "Terminating"
)

type PodSpecMetadataConfig struct {
	Indexed *[]string `json:"indexed,omitempty"`
}

type PodSpec struct {
	Environment      string                 `json:"environment"`
	PodType          string                 `json:"pod_type"`
	PodCount         int32                  `json:"pod_count"`
	Replicas         int32                  `json:"replicas"`
	ShardCount       int32                  `json:"shard_count"`
	SourceCollection *string                `json:"source_collection,omitempty"`
	MetadataConfig   *PodSpecMetadataConfig `json:"metadata_config,omitempty"`
}

type ServerlessSpec struct {
	Cloud  Cloud  `json:"cloud"`
	Region string `json:"region"`
}

type Vector struct {
	Id           string        `json:"id"`
	Values       []float32     `json:"values,omitempty"`
	SparseValues *SparseValues `json:"sparse_values,omitempty"`
	Metadata     *Metadata     `json:"metadata,omitempty"`
}

type ScoredVector struct {
	Vector *Vector `json:"vector,omitempty"`
	Score  float32 `json:"score"`
}

type SparseValues struct {
	Indices []uint32  `json:"indices,omitempty"`
	Values  []float32 `json:"values,omitempty"`
}

type NamespaceSummary struct {
	VectorCount uint32 `json:"vector_count"`
}

type Usage struct {
	ReadUnits *uint32 `json:"read_units,omitempty"`
}

type Filter = structpb.Struct
type Metadata = structpb.Struct