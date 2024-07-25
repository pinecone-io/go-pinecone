package pinecone

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IntegrationTests struct {
	suite.Suite
	apiKey            string
	client            *Client
	host              string
	dimension         int32
	indexType         string
	vectorIds         []string
	podIdxName        string
	serverlessIdxName string
	idxConn           *IndexConnection
	sourceTag         string
	clientSourceTag   Client
	idxConnSourceTag  *IndexConnection
}

func (ts *IntegrationTests) SetupSuite() {
	ctx := context.Background()

	additionalMetadata := map[string]string{"api-key": ts.apiKey}

	namespace, err := uuid.NewUUID()
	require.NoError(ts.T(), err)

	idxConn, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ""})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), idxConn, "Failed to create idxConn")

	ts.idxConn = idxConn

	// Deterministically create vectors
	vectors := createVectorsForUpsert()

	// Set vector IDs
	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}
	ts.vectorIds = vectorIds

	// Upsert vectors
	err = upsertVectors(ts, ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in SetupSuite: %v", err)
	}

	ts.sourceTag = "test_source_tag"
	idxConnSourceTag, err := newIndexConnection(newIndexParameters{
		additionalMetadata: additionalMetadata,
		host:               ts.host,
		namespace:          namespace.String(),
		sourceTag:          ts.sourceTag})
	require.NoError(ts.T(), err)
	ts.idxConnSourceTag = idxConnSourceTag

	fmt.Printf("\n %s set up suite completed successfully\n", ts.indexType)
}

func (ts *IntegrationTests) TearDownSuite() {
	ctx := context.Background()

	// Delete test indexes
	err := ts.client.DeleteIndex(ctx, ts.serverlessIdxName)
	err = ts.client.DeleteIndex(ctx, ts.podIdxName)

	err = ts.idxConn.Close()
	require.NoError(ts.T(), err)

	err = ts.idxConnSourceTag.Close()
	require.NoError(ts.T(), err)
	fmt.Printf("\n %s setup suite torn down successfully\n", ts.indexType)
}

// Helper funcs
func GenerateTestIndexName() string {
	return fmt.Sprintf("index-%d", time.Now().UnixMilli())
}

func upsertVectors(ts *IntegrationTests, ctx context.Context, vectors []*Vector) error {
	maxRetries := 12
	delay := 12 * time.Second
	fmt.Printf("Attempting to upsert vectors into host \"%s\"...\n", ts.host)
	for i := 0; i < maxRetries; i++ {
		ready, err := GetIndexStatus(ts, ctx)
		if err != nil {
			fmt.Printf("Error getting index ready: %v\n", err)
			return err
		}
		if ready {
			upsertVectors, err := ts.idxConn.UpsertVectors(ctx, vectors)
			require.NoError(ts.T(), err)
			fmt.Printf("Upserted vectors: %v into host: %s\n", upsertVectors, ts.host)
			break
		} else {
			time.Sleep(delay)
			fmt.Printf("Host \"%s\" not ready for upserting yet, retrying... (%d/%d)\n", ts.host, i, maxRetries)
		}
	}
	return nil
}

// TODO: how to get this func to work for client tests too
func GetIndexStatus(ts *IntegrationTests, ctx context.Context) (bool, error) {
	var indexName string
	if ts.indexType == "serverless" {
		indexName = ts.serverlessIdxName
	} else if ts.indexType == "pods" {
		indexName = ts.podIdxName
	}
	if ts.client == nil {
		return false, fmt.Errorf("client is nil")
	}

	var desc *Index
	var err error
	maxRetries := 12
	delay := 12 * time.Second
	for i := 0; i < maxRetries; i++ {
		desc, err = ts.client.DescribeIndex(ctx, indexName)
		if err == nil {
			break
		}
		if status.Code(err) == codes.Unknown {
			fmt.Printf("Index \"%s\" not found, retrying... (%d/%d)\n", indexName, i+1, maxRetries)
			time.Sleep(delay)
		} else {
			fmt.Printf("Status code = %v\n", status.Code(err))
			return false, err
		}
	}
	if err != nil {
		return false, fmt.Errorf("failed to describe index \"%s\" after retries: %v", err, indexName)
	}
	return desc.Status.Ready, nil
}

func createVectorsForUpsert() []*Vector {
	vectors := make([]*Vector, 5)
	for i := 0; i < 5; i++ {
		vectors[i] = &Vector{
			Id:     fmt.Sprintf("vector-%d", i+1),
			Values: []float32{float32(i), float32(i) + 0.1, float32(i) + 0.2, float32(i) + 0.3, float32(i) + 0.4},
			SparseValues: &SparseValues{
				Indices: []uint32{0, 1, 2, 3, 4},
				Values:  []float32{float32(i), float32(i) + 0.1, float32(i) + 0.2, float32(i) + 0.3, float32(i) + 0.4},
			},
			Metadata: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
				},
			},
		}
	}
	return vectors
}

func BuildServerlessTestIndex(in *Client, idxName string) *Index {
	ctx := context.Background()

	fmt.Printf("Creating Serverless index: %s\n", idxName)
	serverlessIdx, err := in.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:      idxName,
		Dimension: int32(setDimensionsForTestIndexes()),
		Metric:    Cosine,
		Region:    "us-east-1",
		Cloud:     "aws",
	})
	if err != nil {
		log.Fatalf("Failed to create Serverless index \"%s\" in integration test: %v", err, idxName)
	} else {
		fmt.Printf("Successfully created a new Serverless index: %s!\n", idxName)
	}
	return serverlessIdx
}

func BuildPodTestIndex(in *Client, name string) *Index {
	ctx := context.Background()

	fmt.Printf("Creating pod index: %s\n", name)
	podIdx, err := in.CreatePodIndex(ctx, &CreatePodIndexRequest{
		Name:        name,
		Dimension:   int32(setDimensionsForTestIndexes()),
		Metric:      Cosine,
		Environment: "us-east-1-aws",
		PodType:     "p1",
	})
	if err != nil {
		log.Fatalf("Failed to create pod index in buildPodTestIndex test: %v", err)
	} else {
		fmt.Printf("Successfully created a new pod index: %s!\n", name)
	}
	return podIdx
}

func setDimensionsForTestIndexes() uint32 {
	return uint32(5)
}
