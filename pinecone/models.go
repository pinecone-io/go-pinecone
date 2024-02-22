package pinecone

import "google.golang.org/protobuf/types/known/structpb"

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
