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
	Ready bool
	State IndexStatusState
}

type IndexSpec struct {
	Pod        *PodSpec
	Serverless *ServerlessSpec
}

type Index struct {
	Name      string
	Dimension int32
	Host      string
	Metric    IndexMetric
	Spec      *IndexSpec
	Status    *IndexStatus
}

type PodSpecMetadataConfig struct {
	Indexed *[]string
}

type PodSpec struct {
	Environment      string
	PodType          string
	PodCount         int32
	Replicas         int32
	ShardCount       int32
	SourceCollection *string
	MetadataConfig   *PodSpecMetadataConfig
}

type ServerlessSpec struct {
	Cloud  Cloud
	Region string
}

type Vector struct {
	Id           string
	Values       []float32
	SparseValues *SparseValues
	Metadata     *Metadata
}

type ScoredVector struct {
	Vector *Vector
	Score  float32
}

type SparseValues struct {
	Indices []uint32
	Values  []float32
}

type NamespaceSummary struct {
	VectorCount uint32
}

type Usage struct {
	ReadUnits *uint32
}

type Filter = structpb.Struct
type Metadata = structpb.Struct
