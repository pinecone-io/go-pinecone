package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	db_data_grpc "github.com/pinecone-io/go-pinecone/v7/internal/gen/db_data/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateVectorMissingReqdFieldsUnit(t *testing.T) {
	ctx := context.Background()
	idxConn := &IndexConnection{}
	err := idxConn.UpdateVector(ctx, &UpdateVectorRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "an Id value must be provided to update a vector")
}

func TestNewIndexConnectionUnit(t *testing.T) {
	apiKey := "test-api-key"
	host := "test-host.io"
	namespace := ""
	sourceTag := ""
	additionalMetadata := map[string]string{"api-key": apiKey}
	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               host,
		namespace:          namespace,
		sourceTag:          sourceTag})

	require.NoError(t, err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(t, ok, "Expected client to have an 'api-key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Empty(t, idxConn.namespace, "Expected idxConn to have empty namespace, but got '%s'", idxConn.namespace)
	require.NotNil(t, idxConn.grpcClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(t, idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")
}

func TestNewIndexConnectionNamespaceUnit(t *testing.T) {
	apiKey := "test-api-key"
	host := "test-host.io"
	namespace := "test-namespace"
	sourceTag := "test-source-tag"
	additionalMetadata := map[string]string{"api-key": apiKey}
	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               host,
		namespace:          namespace,
		sourceTag:          sourceTag})

	require.NoError(t, err)
	apiKeyHeader, ok := idxConn.additionalMetadata["api-key"]
	require.True(t, ok, "Expected client to have an 'api-key' header")
	require.Equal(t, apiKey, apiKeyHeader, "Expected 'api-key' header to equal %s", apiKey)
	require.Equal(t, namespace, idxConn.namespace, "Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.namespace)
	require.NotNil(t, idxConn.grpcClient, "Expected idxConn to have non-nil dataClient")
	require.NotNil(t, idxConn.grpcConn, "Expected idxConn to have non-nil grpcConn")
}

func TestMarshalFetchVectorsResponseUnit(t *testing.T) {
	vec1Values := []float32{0.01, 0.01, 0.01}
	vec2Values := []float32{0.02, 0.02, 0.02}

	tests := []struct {
		name  string
		input FetchVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: FetchVectorsResponse{
				Vectors: map[string]*Vector{
					"vec-1": {Id: "vec-1", Values: &vec1Values},
					"vec-2": {Id: "vec-2", Values: &vec2Values},
				},
				Usage:     &Usage{ReadUnits: 5},
				Namespace: "test-namespace",
			},
			want: `{"vectors":{"vec-1":{"id":"vec-1","values":[0.01,0.01,0.01]},"vec-2":{"id":"vec-2","values":[0.02,0.02,0.02]}},"usage":{"read_units":5},"namespace":"test-namespace"}`,
		},
		{
			name:  "Fields omitted",
			input: FetchVectorsResponse{},
			want:  `{"namespace":""}`,
		},
		{
			name: "Fields empty",
			input: FetchVectorsResponse{
				Vectors:   nil,
				Usage:     nil,
				Namespace: "",
			},
			want: `{"namespace":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal FetchVectorsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal FetchVectorsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestMarshalListVectorsResponseUnit(t *testing.T) {
	vectorId1 := "vec-1"
	vectorId2 := "vec-2"
	paginationToken := "next-token"
	tests := []struct {
		name  string
		input ListVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: ListVectorsResponse{
				VectorIds:           []*string{&vectorId1, &vectorId2},
				Usage:               &Usage{ReadUnits: 5},
				NextPaginationToken: &paginationToken,
				Namespace:           "test-namespace",
			},
			want: `{"vector_ids":["vec-1","vec-2"],"usage":{"read_units":5},"next_pagination_token":"next-token","namespace":"test-namespace"}`,
		},
		{
			name:  "Fields omitted",
			input: ListVectorsResponse{},
			want:  `{"namespace":""}`,
		},
		{
			name: "Fields empty",
			input: ListVectorsResponse{
				VectorIds:           nil,
				Usage:               nil,
				NextPaginationToken: nil,
				Namespace:           "",
			},
			want: `{"namespace":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal ListVectorsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal ListVectorsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestMarshalQueryVectorsResponseUnit(t *testing.T) {
	vec1Values := []float32{0.01, 0.01, 0.01}
	vec2Values := []float32{0.02, 0.02, 0.02}
	tests := []struct {
		name  string
		input QueryVectorsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: QueryVectorsResponse{
				Matches: []*ScoredVector{
					{Vector: &Vector{Id: "vec-1", Values: &vec1Values}, Score: 0.1},
					{Vector: &Vector{Id: "vec-2", Values: &vec2Values}, Score: 0.2},
				},
				Usage:     &Usage{ReadUnits: 5},
				Namespace: "test-namespace",
			},
			want: `{"matches":[{"vector":{"id":"vec-1","values":[0.01,0.01,0.01]},"score":0.1},{"vector":{"id":"vec-2","values":[0.02,0.02,0.02]},"score":0.2}],"usage":{"read_units":5},"namespace":"test-namespace"}`,
		},
		{
			name:  "Fields omitted",
			input: QueryVectorsResponse{},
			want:  `{"namespace":""}`,
		},
		{
			name:  "Fields empty",
			input: QueryVectorsResponse{Matches: nil, Usage: nil},
			want:  `{"namespace":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal QueryVectorsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal QueryVectorsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestMarshalDescribeIndexStatsResponseUnit(t *testing.T) {
	tests := []struct {
		name  string
		input DescribeIndexStatsResponse
		want  string
	}{
		{
			name: "All fields present",
			input: DescribeIndexStatsResponse{
				Dimension:        uint32Pointer(3),
				IndexFullness:    0.5,
				TotalVectorCount: 100,
				Metric:           indexMetricPointer(IndexMetricCosine),
				VectorType:       pointerOrNil("dense"),
				MemoryFullness:   float32Pointer(0.25),
				StorageFullness:  float32Pointer(0.75),
				Namespaces: map[string]*NamespaceSummary{
					"namespace-1": {VectorCount: 50},
				},
			},
			want: `{"dimension":3,"index_fullness":0.5,"total_vector_count":100,"metric":"cosine","vector_type":"dense","memory_fullness":0.25,"storage_fullness":0.75,"namespaces":{"namespace-1":{"vector_count":50}}}`,
		},
		{
			name:  "Fields omitted",
			input: DescribeIndexStatsResponse{},
			want:  `{"dimension":null,"index_fullness":0,"total_vector_count":0}`,
		},
		{
			name: "Fields empty",
			input: DescribeIndexStatsResponse{
				Dimension:        uint32Pointer(0),
				IndexFullness:    0,
				TotalVectorCount: 0,
				Namespaces:       nil,
			},
			want: `{"dimension":0,"index_fullness":0,"total_vector_count":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal DescribeIndexStatsResponse: %v", err)
			}

			if got := string(bytes); got != tt.want {
				t.Errorf("Marshal DescribeIndexStatsResponse got = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestToVectorUnit(t *testing.T) {
	vecValues := []float32{0.01, 0.02, 0.03}

	tests := []struct {
		name     string
		vector   *db_data_grpc.Vector
		expected *Vector
	}{
		{
			name:     "Pass nil vector, expect nil to be returned",
			vector:   nil,
			expected: nil,
		},
		{
			name: "Pass dense vector",
			vector: &db_data_grpc.Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
			expected: &Vector{
				Id:     "dense-1",
				Values: &vecValues,
			},
		},
		{
			name: "Pass sparse vector",
			vector: &db_data_grpc.Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector",
			vector: &db_data_grpc.Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},

			expected: &Vector{
				Id:     "hybrid-1",
				Values: &vecValues,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector with metadata",
			vector: &db_data_grpc.Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					}},
			},
			expected: &Vector{
				Id:     "hybrid-metadata-1",
				Values: &vecValues,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toVector(tt.vector)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestToSparseValuesUnit(t *testing.T) {
	tests := []struct {
		name         string
		sparseValues *db_data_grpc.SparseValues
		expected     *SparseValues
	}{
		{
			name:         "Pass nil sparse values, expect nil to be returned",
			sparseValues: nil,
			expected:     nil,
		},
		{
			name: "Pass sparse values",
			sparseValues: &db_data_grpc.SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
			expected: &SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toSparseValues(tt.sparseValues)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestToScoredVectorUnit(t *testing.T) {
	vecValues := []float32{0.01, 0.02, 0.03}

	tests := []struct {
		name         string
		scoredVector *db_data_grpc.ScoredVector
		expected     *ScoredVector
	}{
		{
			name:         "Pass nil scored vector, expect nil to be returned",
			scoredVector: nil,
			expected:     nil,
		},
		{
			name: "Pass scored dense vector",
			scoredVector: &db_data_grpc.ScoredVector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
				Score:  0.1,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "dense-1",
					Values: &vecValues,
				},
				Score: 0.1,
			},
		},
		{
			name: "Pass scored sparse vector",
			scoredVector: &db_data_grpc.ScoredVector{
				Id: "sparse-1",
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Score: 0.2,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id: "sparse-1",
					SparseValues: &SparseValues{
						Indices: []uint32{0, 2},
						Values:  []float32{0.01, 0.03},
					},
				},
				Score: 0.2,
			},
		},
		{
			name: "Pass scored hybrid vector",
			scoredVector: &db_data_grpc.ScoredVector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Score: 0.3,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "hybrid-1",
					Values: &vecValues,
					SparseValues: &SparseValues{
						Indices: []uint32{0, 2},
						Values:  []float32{0.01, 0.03},
					},
				},
				Score: 0.3,
			},
		},
		{
			name: "Pass scored hybrid vector with metadata",
			scoredVector: &db_data_grpc.ScoredVector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					},
				},
				Score: 0.4,
			},
			expected: &ScoredVector{
				Vector: &Vector{
					Id:     "hybrid-metadata-1",
					Values: &vecValues,
					SparseValues: &SparseValues{
						Indices: []uint32{0, 2},
						Values:  []float32{0.01, 0.03},
					},
					Metadata: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
						},
					},
				},
				Score: 0.4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toScoredVector(tt.scoredVector)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestVecToGrpcUnit(t *testing.T) {
	vecValues := []float32{0.01, 0.02, 0.03}

	tests := []struct {
		name     string
		vector   *Vector
		expected *db_data_grpc.Vector
	}{
		{
			name:     "Pass nil vector, expect nil to be returned",
			vector:   nil,
			expected: nil,
		},
		{
			name: "Pass dense vector",
			vector: &Vector{
				Id:     "dense-1",
				Values: &vecValues,
			},
			expected: &db_data_grpc.Vector{
				Id:     "dense-1",
				Values: []float32{0.01, 0.02, 0.03},
			},
		},
		{
			name: "Pass sparse vector",
			vector: &Vector{
				Id:     "sparse-1",
				Values: nil,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &db_data_grpc.Vector{
				Id: "sparse-1",
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector",
			vector: &Vector{
				Id:     "hybrid-1",
				Values: &vecValues,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
			expected: &db_data_grpc.Vector{
				Id:     "hybrid-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
			},
		},
		{
			name: "Pass hybrid vector with metadata",
			vector: &Vector{
				Id:     "hybrid-metadata-1",
				Values: &vecValues,
				SparseValues: &SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					},
				},
			},
			expected: &db_data_grpc.Vector{
				Id:     "hybrid-metadata-1",
				Values: []float32{0.01, 0.02, 0.03},
				SparseValues: &db_data_grpc.SparseValues{
					Indices: []uint32{0, 2},
					Values:  []float32{0.01, 0.03},
				},
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vecToGrpc(tt.vector)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestSparseValToGrpcUnit(t *testing.T) {
	tests := []struct {
		name         string
		sparseValues *SparseValues
		metadata     *structpb.Struct
		expected     *db_data_grpc.SparseValues
	}{
		{
			name:         "Pass nil sparse values, expect nil to be returned",
			sparseValues: nil,
			expected:     nil,
		},
		{
			name: "Pass sparse values",
			sparseValues: &SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
			expected: &db_data_grpc.SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
		},
		{
			name: "Pass sparse values with metadata (metadata is ignored)",
			sparseValues: &SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
			metadata: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
				},
			},
			expected: &db_data_grpc.SparseValues{
				Indices: []uint32{0, 2},
				Values:  []float32{0.01, 0.03},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sparseValToGrpc(tt.sparseValues)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestAkCtxUnit(t *testing.T) {
	tests := []struct {
		name               string
		additionalMetadata map[string]string
		initialMetadata    map[string]string
		expectedMetadata   map[string]string
	}{
		{
			name:               "No additional metadata in IndexConnection obj",
			additionalMetadata: nil,
			initialMetadata:    map[string]string{"initial-key": "initial-value"},
			expectedMetadata:   map[string]string{"initial-key": "initial-value"},
		},
		{
			name:               "With additional metadata in IndexConnection obj",
			additionalMetadata: map[string]string{"addtl-key1": "addtl-value1", "addtl-key2": "addtl-value2"},
			initialMetadata:    map[string]string{"initial-key": "initial-value"},
			expectedMetadata: map[string]string{
				"initial-key": "initial-value",
				"addtl-key1":  "addtl-value1",
				"addtl-key2":  "addtl-value2",
			},
		},
		{
			name: "Only additional metadata",
			additionalMetadata: map[string]string{
				"addtl-key1": "addtl-value1",
				"addtl-key2": "addtl-value2",
			},
			initialMetadata: nil,
			expectedMetadata: map[string]string{
				"addtl-key1": "addtl-value1",
				"addtl-key2": "addtl-value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &IndexConnection{additionalMetadata: tt.additionalMetadata}
			ctx := context.Background()

			// Add initial metadata to the context if provided
			if tt.initialMetadata != nil {
				md := metadata.New(tt.initialMetadata)
				ctx = metadata.NewOutgoingContext(ctx, md)
			}

			// Call the method
			newCtx := idx.akCtx(ctx)

			// Retrieve metadata from the new context
			md, ok := metadata.FromOutgoingContext(newCtx)
			assert.True(t, ok)

			// Check that the metadata matches the expected metadata
			for key, expectedValue := range tt.expectedMetadata {
				values := md[key]
				assert.Contains(t, values, expectedValue)
			}
		})
	}
}

func TestToUsageUnit(t *testing.T) {
	u5 := uint32(5)
	egress := uint32(1432)

	tests := []struct {
		name     string
		usage    *db_data_grpc.Usage
		expected *Usage
	}{
		{
			name:     "Pass nil usage, expect nil to be returned",
			usage:    nil,
			expected: nil,
		},
		{
			name: "Pass usage without egress, expect nil EgressBytes",
			usage: &db_data_grpc.Usage{
				ReadUnits: &u5,
			},
			expected: &Usage{
				ReadUnits: 5,
			},
		},
		{
			name: "Pass usage with egress",
			usage: &db_data_grpc.Usage{
				ReadUnits:   &u5,
				EgressBytes: &egress,
			},
			expected: &Usage{
				ReadUnits:   5,
				EgressBytes: &egress,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toUsage(tt.usage)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func TestNormalizeHostUnit(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		expectedHost     string
		expectedIsSecure bool
	}{
		{
			name:             "https:// scheme should be removed",
			host:             "https://this-is-my-host.io",
			expectedHost:     "this-is-my-host.io",
			expectedIsSecure: true,
		}, {
			name:             "https:// scheme should be removed",
			host:             "https://this-is-my-host.io:33445",
			expectedHost:     "this-is-my-host.io:33445",
			expectedIsSecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, isSecure := normalizeHost(tt.host)
			assert.Equal(t, tt.expectedHost, result, "Expected result to be '%s', but got '%s'", tt.expectedHost, result)
			assert.Equal(t, tt.expectedIsSecure, isSecure, "Expected isSecure to be '%t', but got '%t'", tt.expectedIsSecure, isSecure)
		})
	}
}

func TestToPaginationTokenGrpcUnit(t *testing.T) {
	tokenForNilCase := ""
	tokenForPositiveCase := "next-token"

	tests := []struct {
		name     string
		token    *db_data_grpc.Pagination
		expected *string
	}{
		{
			name:     "Pass empty token, expect empty string to be returned",
			token:    &db_data_grpc.Pagination{},
			expected: &tokenForNilCase,
		},
		{
			name: "Pass token",
			token: &db_data_grpc.Pagination{
				Next: "next-token",
			},
			expected: &tokenForPositiveCase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toPaginationTokenGrpc(tt.token)
			assert.Equal(t, tt.expected, result, "Expected result to be '%s', but got '%s'", tt.expected, result)
		})
	}
}

func Test_fromMetadataSchemaToGrpc_Unit(t *testing.T) {
	tests := []struct {
		name     string
		input    *MetadataSchema
		expected *db_data_grpc.MetadataSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty fields map",
			input: &MetadataSchema{
				Fields: make(map[string]MetadataSchemaField),
			},
			expected: &db_data_grpc.MetadataSchema{
				Fields: make(map[string]*db_data_grpc.MetadataFieldProperties),
			},
		},
		{
			name: "fields with filterable true",
			input: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: true},
				},
			},
			expected: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre": {Filterable: true},
				},
			},
		},
		{
			name: "fields with filterable false",
			input: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: false},
				},
			},
			expected: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre": {Filterable: false},
				},
			},
		},
		{
			name: "multiple fields",
			input: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre":  {Filterable: true},
					"year":   {Filterable: true},
					"rating": {Filterable: false},
				},
			},
			expected: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre":  {Filterable: true},
					"year":   {Filterable: true},
					"rating": {Filterable: false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromMetadataSchemaToGrpc(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, len(tt.expected.Fields), len(result.Fields))

			for key, expectedField := range tt.expected.Fields {
				actualField, ok := result.Fields[key]
				require.True(t, ok, "Field %s should exist", key)
				require.NotNil(t, actualField)
				assert.Equal(t, expectedField.Filterable, actualField.Filterable)
			}
		})
	}
}

func Test_toMetadataSchemaGrpc_Unit(t *testing.T) {
	tests := []struct {
		name     string
		input    *db_data_grpc.MetadataSchema
		expected *MetadataSchema
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty fields map",
			input: &db_data_grpc.MetadataSchema{
				Fields: make(map[string]*db_data_grpc.MetadataFieldProperties),
			},
			expected: &MetadataSchema{
				Fields: make(map[string]MetadataSchemaField),
			},
		},
		{
			name: "fields with filterable true",
			input: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre": {Filterable: true},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: true},
				},
			},
		},
		{
			name: "fields with filterable false",
			input: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre": {Filterable: false},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre": {Filterable: false},
				},
			},
		},
		{
			name: "multiple fields",
			input: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre":  {Filterable: true},
					"year":   {Filterable: true},
					"rating": {Filterable: false},
				},
			},
			expected: &MetadataSchema{
				Fields: map[string]MetadataSchemaField{
					"genre":  {Filterable: true},
					"year":   {Filterable: true},
					"rating": {Filterable: false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toMetadataSchemaGrpc(tt.input)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("toMetadataSchemaGrpc() mismatch (-expected +result):\n%s", diff)
			}
		})
	}
}

func TestToNamespaceDescriptionUnit(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		require.Nil(t, toNamespaceDescription(nil))
	})

	t.Run("maps schema and indexed fields", func(t *testing.T) {
		ns := &db_data_grpc.NamespaceDescription{
			Name:        "namespace-1",
			RecordCount: 42,
			Schema: &db_data_grpc.MetadataSchema{
				Fields: map[string]*db_data_grpc.MetadataFieldProperties{
					"genre": {
						Filterable: true,
					},
				},
			},
			IndexedFields: &db_data_grpc.IndexedFields{
				Fields: []string{"genre"},
			},
			SizeBytes: 2048,
		}

		result := toNamespaceDescription(ns)
		require.NotNil(t, result)
		require.Equal(t, "namespace-1", result.Name)
		require.EqualValues(t, 42, result.RecordCount)
		require.NotNil(t, result.Schema)
		require.True(t, result.Schema.Fields["genre"].Filterable)
		require.NotNil(t, result.IndexedFields)
		require.Equal(t, []string{"genre"}, result.IndexedFields.Fields)
		require.EqualValues(t, 2048, result.SizeBytes)
	})
}

func uint32Pointer(i uint32) *uint32 {
	return &i
}

func float32Pointer(v float32) *float32 {
	return &v
}

func indexMetricPointer(metric IndexMetric) *IndexMetric {
	return &metric
}

func TestRestNamespaceUnit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string maps to __default__",
			input:    "",
			expected: "__default__",
		},
		{
			name:     "non-empty namespace is returned unchanged",
			input:    "my-namespace",
			expected: "my-namespace",
		},
		{
			name:     "__default__ is returned unchanged",
			input:    "__default__",
			expected: "__default__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveNamespace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type requireSecurityCreds struct{}

func (requireSecurityCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return nil, nil
}

func (requireSecurityCreds) RequireTransportSecurity() bool { return true }

func TestNewIndexConnectionReturnsGrpcClientErrorUnit(t *testing.T) {
	// Per-RPC credentials that require transport security are rejected by grpc.NewClient on a
	// plaintext host, so the constructor must return that error rather than exit the process.
	idxConn, err := newIndexConnection(newIndexParameters{host: "http://localhost:5081"},
		grpc.WithPerRPCCredentials(requireSecurityCreds{}))
	require.Error(t, err)
	assert.Nil(t, idxConn)
	assert.Contains(t, err.Error(), "failed to create grpc client")
}

func TestUpsertRecordsRejectsBothIdFieldsUnit(t *testing.T) {
	idxConn := &IndexConnection{}
	err := idxConn.UpsertRecords(context.Background(), []*IntegratedRecord{
		{"_id": "rec1", "id": "rec1", "text": "hello"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestSearchRecordsRequiresRerankFieldsUnit(t *testing.T) {
	idxConn := &IndexConnection{}
	_, err := idxConn.SearchRecords(context.Background(), &SearchRecordsRequest{
		Query:  SearchRecordsQuery{TopK: 5, Inputs: &map[string]interface{}{"text": "hello"}},
		Rerank: &SearchRecordsRerank{Model: "bge-reranker-v2-m3"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RankFields")
}

func TestValidateMetadataRejectsNullUnit(t *testing.T) {
	tests := []struct {
		name    string
		value   *structpb.Value
		wantErr bool
	}{
		{name: "string", value: structpb.NewStringValue("a")},
		{name: "number", value: structpb.NewNumberValue(1)},
		{name: "bool", value: structpb.NewBoolValue(true)},
		{name: "string list", value: structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{structpb.NewStringValue("a")}})},
		{name: "null", value: structpb.NewNullValue(), wantErr: true},
		{name: "unset kind", value: &structpb.Value{}, wantErr: true},
		{name: "number list", value: structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{structpb.NewNumberValue(1)}}), wantErr: true},
		{name: "struct", value: structpb.NewStructValue(&structpb.Struct{}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetadata(&Metadata{Fields: map[string]*structpb.Value{"field": tt.value}})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type namespaceCapturingClient struct {
	db_data_grpc.VectorServiceClient
	namespaces []string
}

func (c *namespaceCapturingClient) DescribeNamespace(ctx context.Context, in *db_data_grpc.DescribeNamespaceRequest, opts ...grpc.CallOption) (*db_data_grpc.NamespaceDescription, error) {
	c.namespaces = append(c.namespaces, in.Namespace)
	return &db_data_grpc.NamespaceDescription{Name: in.Namespace}, nil
}

func (c *namespaceCapturingClient) DeleteNamespace(ctx context.Context, in *db_data_grpc.DeleteNamespaceRequest, opts ...grpc.CallOption) (*db_data_grpc.DeleteResponse, error) {
	c.namespaces = append(c.namespaces, in.Namespace)
	return &db_data_grpc.DeleteResponse{}, nil
}

func TestNamespaceOperationsResolveDefaultNamespaceUnit(t *testing.T) {
	fake := &namespaceCapturingClient{}
	var client db_data_grpc.VectorServiceClient = fake
	idxConn := &IndexConnection{grpcClient: &client}
	ctx := context.Background()

	_, err := idxConn.DescribeNamespace(ctx, "")
	require.NoError(t, err)
	_, err = idxConn.DescribeNamespace(ctx, "__default__")
	require.NoError(t, err)
	_, err = idxConn.DescribeNamespace(ctx, "movies")
	require.NoError(t, err)
	require.NoError(t, idxConn.DeleteNamespace(ctx, ""))

	assert.Equal(t, []string{"__default__", "__default__", "movies", "__default__"}, fake.namespaces)
}

func TestSearchDocumentsScoreByValidationUnit(t *testing.T) {
	idx := &IndexConnection{}
	ctx := context.Background()
	query := "q"

	_, err := idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 0, ScoreBy: []DocumentScoringMethod{{Type: "text", Query: &query}}})
	require.ErrorContains(t, err, "TopK must be at least 1")

	_, err = idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 10})
	require.ErrorContains(t, err, "ScoreBy must contain at least one")

	values := []float32{0.1}
	_, err = idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 10, ScoreBy: []DocumentScoringMethod{
		{Type: "text", Fields: []string{"plot"}, Query: &query},
		{Type: "dense_vector", Fields: []string{"embedding"}, Values: &values},
	}})
	require.ErrorContains(t, err, `a "dense_vector" method must appear on its own`)

	for _, scoringType := range []string{DocumentScoringTypeEmbed, DocumentScoringTypeSparseEmbed} {
		embed := DocumentScoringMethod{Type: scoringType, Fields: []string{"body"}, Query: &query}
		text := DocumentScoringMethod{Type: DocumentScoringTypeText, Fields: []string{"plot"}, Query: &query}
		for _, scoreBy := range [][]DocumentScoringMethod{{embed, text}, {text, embed}} {
			_, err = idx.SearchDocuments(ctx, &SearchDocumentsRequest{TopK: 10, ScoreBy: scoreBy})
			require.ErrorContains(t, err, fmt.Sprintf("a %q method must appear on its own", scoringType))
		}
	}
}

func TestFetchDocumentsSelectorValidationUnit(t *testing.T) {
	idx := &IndexConnection{}
	ctx := context.Background()

	_, err := idx.FetchDocuments(ctx, &FetchDocumentsRequest{})
	require.ErrorContains(t, err, "exactly one of Ids or Filter")

	_, err = idx.FetchDocuments(ctx, &FetchDocumentsRequest{Ids: []string{"a"}, Filter: map[string]interface{}{"genre": "drama"}})
	require.ErrorContains(t, err, "exactly one of Ids or Filter")

	_, err = idx.FetchDocuments(ctx, &FetchDocumentsRequest{Filter: map[string]interface{}{}})
	require.ErrorContains(t, err, "Filter must not be empty")
}

func TestDeleteDocumentsSelectorValidationUnit(t *testing.T) {
	idx := &IndexConnection{}
	ctx := context.Background()

	_, err := idx.DeleteDocuments(ctx, &DeleteDocumentsRequest{})
	require.ErrorContains(t, err, "exactly one of Ids, Filter, or DeleteAll")

	_, err = idx.DeleteDocuments(ctx, &DeleteDocumentsRequest{Ids: []string{"a"}, DeleteAll: true})
	require.ErrorContains(t, err, "exactly one of Ids, Filter, or DeleteAll")

	_, err = idx.DeleteDocuments(ctx, &DeleteDocumentsRequest{Filter: map[string]interface{}{}})
	require.ErrorContains(t, err, "Filter must not be empty")
}

func TestUpdateDocumentsSelectorValidationUnit(t *testing.T) {
	idx := &IndexConnection{}
	ctx := context.Background()

	_, err := idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{})
	require.ErrorContains(t, err, "either Documents or a filtered patch")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{
		Documents: []Document{{"_id": "a"}},
		Filter:    map[string]interface{}{"genre": "drama"},
	})
	require.ErrorContains(t, err, "either Documents or a filtered patch")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{Filter: map[string]interface{}{"genre": "drama"}})
	require.ErrorContains(t, err, "must set SetFields and/or RemoveFields")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{SetFields: map[string]interface{}{"a": 1}})
	require.ErrorContains(t, err, "only valid together with Filter")

	_, err = idx.UpdateDocuments(ctx, &UpdateDocumentsRequest{Documents: []Document{{"genre": "drama"}}})
	require.ErrorContains(t, err, `must have an "_id" field`)
}

func TestVectorOpValidationUnit(t *testing.T) {
	idx := &IndexConnection{}
	ctx := context.Background()

	_, err := idx.QueryByVectorValues(ctx, &QueryByVectorValuesRequest{Vector: []float32{0.1}, TopK: 20000})
	require.ErrorContains(t, err, "TopK must be between 1 and 10000")

	_, err = idx.QueryByVectorId(ctx, &QueryByVectorIdRequest{VectorId: "a", TopK: 0})
	require.ErrorContains(t, err, "TopK must be between 1 and 10000")

	_, err = idx.FetchVectors(ctx, []string{})
	require.ErrorContains(t, err, "at least one vector ID")

	_, err = idx.FetchVectors(ctx, []string{strings.Repeat("a", 600)})
	require.ErrorContains(t, err, "exceeds the maximum length of 512")

	err = idx.DeleteVectorsByFilter(ctx, &MetadataFilter{})
	require.ErrorContains(t, err, "empty metadata filter is not allowed")

	_, err = idx.ListVectors(ctx, &ListVectorsRequest{Limit: ptr(uint32(500))})
	require.ErrorContains(t, err, "Limit must be between 1 and 100")
}

func TestListImportsLimitValidationUnit(t *testing.T) {
	idx := &IndexConnection{}
	_, err := idx.ListImports(context.Background(), &ListImportsRequest{Limit: ptr(int32(0))})
	require.ErrorContains(t, err, "Limit must be between")
}
