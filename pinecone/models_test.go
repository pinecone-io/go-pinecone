package pinecone

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestMarshalIndexStatusUnit(t *testing.T) {
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

func TestMarshalServerlessSpecUnit(t *testing.T) {
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

func TestMarshalPodSpecUnit(t *testing.T) {
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

func TestMarshalIndexSpecUnit(t *testing.T) {
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

func TestMarshalIndexUnit(t *testing.T) {
	dimension := int32(128)

	tests := []struct {
		name  string
		input Index
		want  string
	}{
		{
			name: "All fields present",
			input: Index{
				Name:               "test-index",
				Dimension:          &dimension,
				Host:               "index-host-1.io",
				Metric:             "cosine",
				VectorType:         "sparse",
				DeletionProtection: "enabled",
				Embed: &IndexEmbed{
					Model: "multilingual-e5-large",
				},
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
				Tags: &IndexTags{
					"test1": "test-tag-1",
				},
			},
			want: `{"name":"test-index","host":"index-host-1.io","metric":"cosine","vector_type":"sparse","deletion_protection":"enabled","dimension":128,"spec":{"serverless":{"cloud":"aws","region":"us-west-2"}},"status":{"ready":true,"state":"Ready"},"tags":{"test1":"test-tag-1"},"embed":{"model":"multilingual-e5-large"}}`,
		},
		{
			name:  "Fields omitted",
			input: Index{},
			want:  `{"name":"","host":"","metric":"","vector_type":""}`,
		},
		{
			name: "Fields empty",
			input: Index{
				Name:      "",
				Dimension: nil,
				Host:      "",
				Metric:    "",
				Spec:      nil,
				Status:    nil,
			},
			want: `{"name":"","host":"","metric":"","vector_type":""}`,
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

func TestMarshalCollectionUnit(t *testing.T) {
	tests := []struct {
		name  string
		input Collection
		want  string
	}{
		{
			name: "All fields present",
			input: Collection{
				Name:        "test-collection",
				Size:        15328,
				Status:      "Ready",
				Dimension:   132,
				VectorCount: 15000,
				Environment: "us-west-2",
			},
			want: `{"name":"test-collection","size":15328,"status":"Ready","dimension":132,"vector_count":15000,"environment":"us-west-2"}`,
		},
		{
			name:  "Fields omitted",
			input: Collection{},
			want:  `{"name":"","size":0,"status":"","dimension":0,"vector_count":0,"environment":""}`,
		},
		{
			name: "Fields empty",
			input: Collection{
				Name:        "",
				Size:        0,
				Status:      "",
				Dimension:   0,
				VectorCount: 0,
				Environment: "",
			},
			want: `{"name":"","size":0,"status":"","dimension":0,"vector_count":0,"environment":""}`,
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

func TestMarshalPodSpecMetadataConfigUnit(t *testing.T) {
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

func TestMarshalVectorUnit(t *testing.T) {
	metadata, err := structpb.NewStruct(map[string]interface{}{"genre": "rock"})
	if err != nil {
		t.Fatalf("Failed to create metadata: %v", err)
	}
	vecValues := []float32{0.1, 0.2, 0.3}

	tests := []struct {
		name  string
		input Vector
		want  string
	}{
		{
			name: "All fields present",
			input: Vector{
				Id:       "vector-1",
				Values:   &vecValues,
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

func TestMarshalScoredVectorUnit(t *testing.T) {
	metadata, err := structpb.NewStruct(map[string]interface{}{"genre": "rock"})
	if err != nil {
		t.Fatalf("Failed to create metadata: %v", err)
	}
	vecValues := []float32{0.1, 0.2, 0.3}

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
					Values:   &vecValues,
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

func TestMarshalSparseValuesUnit(t *testing.T) {
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

func TestMarshalNamespaceSummaryUnit(t *testing.T) {
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

func TestMarshalUsageUnit(t *testing.T) {
	tests := []struct {
		name  string
		input Usage
		want  string
	}{
		{
			name:  "All fields present",
			input: Usage{ReadUnits: 100},
			want:  `{"read_units":100}`,
		},
		{
			name:  "Fields omitted",
			input: Usage{},
			want:  `{"read_units":0}`,
		},
		{
			name:  "Fields empty",
			input: Usage{ReadUnits: 0},
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

func TestMarshalIndexEmbedUnit(t *testing.T) {
	dimension := int32(128)
	metric := IndexMetric("cosine")
	vectorType := "sparse"
	fieldMap := map[string]interface{}{
		"text-field": "my-text-field",
	}
	readParameters := map[string]interface{}{
		"readParam": "readParamValue",
	}
	writeParameters := map[string]interface{}{
		"writeParam": "writeParamValue",
	}

	tests := []struct {
		name  string
		input IndexEmbed
		want  string
	}{
		{
			name: "All fields present",
			input: IndexEmbed{
				Model:           "multilingual-e5-large",
				Dimension:       &dimension,
				Metric:          &metric,
				VectorType:      &vectorType,
				FieldMap:        &fieldMap,
				ReadParameters:  &readParameters,
				WriteParameters: &writeParameters,
			},
			want: `{"model":"multilingual-e5-large","dimension":128,"metric":"cosine","vector_type":"sparse","field_map":{"text-field":"my-text-field"},"read_parameters":{"readParam":"readParamValue"},"write_parameters":{"writeParam":"writeParamValue"}}`,
		},
		{
			name:  "Fields omitted",
			input: IndexEmbed{},
			want:  `{"model":""}`,
		},
		{
			name: "Fields empty",
			input: IndexEmbed{
				Model:           "",
				Dimension:       nil,
				Metric:          nil,
				VectorType:      nil,
				FieldMap:        nil,
				ReadParameters:  nil,
				WriteParameters: nil,
			},
			want: `{"model":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(c *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				c.Errorf("Failed to marshal IndexEmbed: %v", err)
			}
			if string(got) != tt.want {
				c.Errorf("Marshal IndexEmbed got = %s, want = %s", string(got), tt.want)
			}
		})
	}
}
