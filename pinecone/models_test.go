package pinecone

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestMarshalingIndexStatus(t *testing.T) {
	tests := []struct {
		name  string
		input IndexStatus
		want  string
	}{
		{
			name:  "All fields present",
			input: IndexStatus{Ready: true, State: "Ready"},
			want:  `{"ready":true,"state":"Ready"}`,
		},
		{
			name:  "Fields omitted",
			input: IndexStatus{},
			want:  `{"ready":false,"state":""}`,
		},
		{
			name:  "Fields empty",
			input: IndexStatus{Ready: false, State: ""},
			want:  `{"ready":false,"state":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal IndexStatus: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal IndexStatus got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalingServerlessSpec(t *testing.T) {
	tests := []struct {
		name  string
		input ServerlessSpec
		want  string
	}{
		{
			name:  "All fields present",
			input: ServerlessSpec{Cloud: "aws", Region: "us-west-"},
			want:  `{"cloud":"aws","region":"us-west-"}`,
		},
		{
			name:  "Fields omitted",
			input: ServerlessSpec{},
			want:  `{"cloud":"","region":""}`,
		},
		{
			name:  "Fields empty",
			input: ServerlessSpec{Cloud: "", Region: ""},
			want:  `{"cloud":"","region":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal ServerlessSpec: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal ServerlessSpec got = %s, want = %s", string(got), tt.want)
			}

		})
	}
}

func TestMarshalingPodSpec(t *testing.T) {
	sourceCollection := "source-collection"
	tests := []struct {
		name  string
		input PodSpec
		want  string
	}{
		{
			name: "All fields present",
			input: PodSpec{
				Environment:      "us-west2-gcp",
				PodType:          "p1.x1",
				PodCount:         1,
				Replicas:         1,
				ShardCount:       1,
				SourceCollection: &sourceCollection,
				MetadataConfig: &PodSpecMetadataConfig{
					Indexed: &[]string{"genre"},
				},
			},
			want: `{"environment":"us-west2-gcp","pod_type":"p1.x1","pod_count":1,"replicas":1,"shard_count":1,"source_collection":"source-collection","metadata_config":{"indexed":["genre"]}}`,
		},
		{
			name:  "Fields omitted",
			input: PodSpec{},
			want:  `{"environment":"","pod_type":"","pod_count":0,"replicas":0,"shard_count":0}`,
		},
		{
			name: "Fields empty",
			input: PodSpec{
				Environment:      "",
				PodType:          "",
				PodCount:         0,
				Replicas:         0,
				ShardCount:       0,
				SourceCollection: nil,
				MetadataConfig:   nil,
			},
			want: `{"environment":"","pod_type":"","pod_count":0,"replicas":0,"shard_count":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal PodSpec: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal PodSpec got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalingIndexSpec(t *testing.T) {
	sourceCollection := "source-collection"
	tests := []struct {
		name  string
		input IndexSpec
		want  string
	}{
		{
			name: "Pod spec",
			input: IndexSpec{Pod: &PodSpec{
				Environment:      "us-west2-gcp",
				PodType:          "p1.x1",
				PodCount:         1,
				Replicas:         1,
				ShardCount:       1,
				SourceCollection: &sourceCollection,
				MetadataConfig: &PodSpecMetadataConfig{
					Indexed: &[]string{"genre"},
				},
			}},
			want: `{"pod":{"environment":"us-west2-gcp","pod_type":"p1.x1","pod_count":1,"replicas":1,"shard_count":1,"source_collection":"source-collection","metadata_config":{"indexed":["genre"]}}}`,
		},
		{
			name:  "Serverless spec",
			input: IndexSpec{Serverless: &ServerlessSpec{Cloud: "aws", Region: "us-west-"}},
			want:  `{"serverless":{"cloud":"aws","region":"us-west-"}}`,
		},
		{
			name:  "Fields omitted",
			input: IndexSpec{},
			want:  `{}`,
		},
		{
			name:  "Fields empty",
			input: IndexSpec{Pod: nil, Serverless: nil},
			want:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal IndexSpec: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal IndexSpec got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalIndex(t *testing.T) {
	tests := []struct {
		name  string
		input Index
		want  string
	}{
		{
			name: "All fields present",
			input: Index{
				Name:      "test-index",
				Dimension: 128,
				Host:      "index-host-1.io",
				Metric:    "cosine",
				Spec: &IndexSpec{
					Serverless: &ServerlessSpec{
						Cloud:  "aws",
						Region: "us-west-2",
					},
				},
				Status: &IndexStatus{
					Ready: true,
					State: "Ready",
				},
			},
			want: `{"name":"test-index","dimension":128,"host":"index-host-1.io","metric":"cosine","spec":{"serverless":{"cloud":"aws","region":"us-west-2"}},"status":{"ready":true,"state":"Ready"}}`,
		},
		{
			name:  "Fields omitted",
			input: Index{},
			want:  `{"name":"","dimension":0,"host":"","metric":""}`,
		},
		{
			name: "Fields empty",
			input: Index{
				Name:      "",
				Dimension: 0,
				Host:      "",
				Metric:    "",
				Spec:      nil,
				Status:    nil,
			},
			want: `{"name":"","dimension":0,"host":"","metric":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal Index: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal Index got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalCollection(t *testing.T) {
	tests := []struct {
		name  string
		input Collection
		want  string
	}{
		{
			name: "All fields present",
			input: Collection{
				Name:        "test-collection",
				Size:        toInt64(15328),
				Status:      "Ready",
				Dimension:   toInt32(132),
				VectorCount: toInt32(15000),
				Environment: "us-west-2",
			},
			want: `{"name":"test-collection","size":15328,"status":"Ready","dimension":132,"vector_count":15000,"environment":"us-west-2"}`,
		},
		{
			name:  "Fields omitted",
			input: Collection{},
			want:  `{"name":"","status":"","environment":""}`,
		},
		{
			name: "Fields empty",
			input: Collection{
				Name:        "",
				Size:        nil,
				Status:      "",
				Dimension:   nil,
				VectorCount: nil,
				Environment: "",
			},
			want: `{"name":"","status":"","environment":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal Collection: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal Collection got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalPodSpecMetadataConfig(t *testing.T) {
	tests := []struct {
		name  string
		input PodSpecMetadataConfig
		want  string
	}{
		{
			name:  "All fields present",
			input: PodSpecMetadataConfig{Indexed: &[]string{"genre", "artist"}},
			want:  `{"indexed":["genre","artist"]}`,
		},
		{
			name:  "Fields omitted",
			input: PodSpecMetadataConfig{},
			want:  `{}`,
		},
		{
			name:  "Fields empty",
			input: PodSpecMetadataConfig{Indexed: nil},
			want:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal PodSpecMetadataConfig: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal PodSpecMetadataConfig got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalVector(t *testing.T) {
	metadata, err := structpb.NewStruct(map[string]interface{}{"genre": "rock"})
	if err != nil {
		t.Fatalf("Failed to create metadata: %v", err)
	}

	tests := []struct {
		name  string
		input Vector
		want  string
	}{
		{
			name: "All fields present",
			input: Vector{
				Id:       "vector-1",
				Values:   []float32{0.1, 0.2, 0.3},
				Metadata: metadata,
				SparseValues: &SparseValues{
					Indices: []uint32{1, 2, 3},
					Values:  []float32{0.1, 0.2, 0.3},
				},
			},
			want: `{"id":"vector-1","values":[0.1,0.2,0.3],"sparse_values":{"indices":[1,2,3],"values":[0.1,0.2,0.3]},"metadata":{"genre":"rock"}}`,
		},
		{
			name:  "Fields omitted",
			input: Vector{},
			want:  `{"id":""}`,
		},
		{
			name:  "Fields empty",
			input: Vector{Id: "", Values: nil, SparseValues: nil, Metadata: nil},
			want:  `{"id":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal Vector: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal Vector got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalScoredVector(t *testing.T) {
	metadata, err := structpb.NewStruct(map[string]interface{}{"genre": "rock"})
	if err != nil {
		t.Fatalf("Failed to create metadata: %v", err)
	}

	tests := []struct {
		name  string
		input ScoredVector
		want  string
	}{
		{
			name: "All fields present",
			input: ScoredVector{
				Vector: &Vector{
					Id:       "vector-1",
					Values:   []float32{0.1, 0.2, 0.3},
					Metadata: metadata,
					SparseValues: &SparseValues{
						Indices: []uint32{1, 2, 3},
						Values:  []float32{0.1, 0.2, 0.3},
					},
				},
				Score: 0.9,
			},
			want: `{"vector":{"id":"vector-1","values":[0.1,0.2,0.3],"sparse_values":{"indices":[1,2,3],"values":[0.1,0.2,0.3]},"metadata":{"genre":"rock"}},"score":0.9}`,
		},
		{
			name:  "Fields omitted",
			input: ScoredVector{},
			want:  `{"score":0}`,
		},
		{
			name:  "Fields empty",
			input: ScoredVector{Vector: nil, Score: 0},
			want:  `{"score":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal ScoredVector: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal ScoredVector got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalSparseValues(t *testing.T) {
	tests := []struct {
		name  string
		input SparseValues
		want  string
	}{
		{
			name: "All fields present",
			input: SparseValues{
				Indices: []uint32{1, 2, 3},
				Values:  []float32{0.1, 0.2, 0.3},
			},
			want: `{"indices":[1,2,3],"values":[0.1,0.2,0.3]}`,
		},
		{
			name:  "Fields omitted",
			input: SparseValues{},
			want:  `{}`,
		},
		{
			name:  "Fields empty",
			input: SparseValues{Indices: nil, Values: nil},
			want:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal SparseValues: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal SparseValues got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalNamespaceSummary(t *testing.T) {
	tests := []struct {
		name  string
		input NamespaceSummary
		want  string
	}{
		{
			name:  "All fields present",
			input: NamespaceSummary{VectorCount: 15000},
			want:  `{"vector_count":15000}`,
		},
		{
			name:  "Fields omitted",
			input: NamespaceSummary{},
			want:  `{"vector_count":0}`,
		},
		{
			name:  "Fields empty",
			input: NamespaceSummary{VectorCount: 0},
			want:  `{"vector_count":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal NamespaceSummary: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal NamespaceSummary got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}

func TestMarshalUsage(t *testing.T) {
	tests := []struct {
		name  string
		input Usage
		want  string
	}{
		{
			name:  "All fields present",
			input: Usage{ReadUnits: toUInt32(100)},
			want:  `{"read_units":100}`,
		},
		{
			name:  "Fields omitted",
			input: Usage{},
			want:  `{}`,
		},
		{
			name:  "Fields empty",
			input: Usage{ReadUnits: toUInt32(0)},
			want:  `{"read_units":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal Usage: %v", err)
				return
			}
			if string(got) != tt.want {
				c.Errorf("Marshal Usage got = %s, want = %s", string(got), tt.want)
			}
		})
	}

}

func toInt64(i int64) *int64 {
	return &i
}

func toInt32(i int32) *int32 {
	return &i
}
