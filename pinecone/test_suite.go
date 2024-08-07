package pinecone

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IntegrationTests struct {
	suite.Suite
	apiKey           string
	client           *Client
	clientSourceTag  *Client
	host             string
	dimension        int32
	indexType        string
	vectorIds        []string
	idxName          string
	idxConn          *IndexConnection
	idxConnSourceTag *IndexConnection
	sourceTag        string
}

func (ts *IntegrationTests) SetupSuite() {
	ctx := context.Background()

	namespace, err := uuid.NewUUID()
	require.NoError(ts.T(), err)

	idxConn, err := ts.client.Index(NewIndexConnParams{
		Host:      ts.host,
		Namespace: namespace.String(),
	})

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

	idxConnSourceTag, err := ts.clientSourceTag.Index(NewIndexConnParams{
		Host:      ts.host,
		Namespace: namespace.String(),
	})
	require.NoError(ts.T(), err)
	ts.idxConnSourceTag = idxConnSourceTag

	fmt.Printf("\n %s set up suite completed successfully\n", ts.indexType)
}

func (ts *IntegrationTests) TearDownSuite() {
	ctx := context.Background()

	err := ts.idxConn.Close()
	require.NoError(ts.T(), err)

	err = ts.idxConnSourceTag.Close()
	require.NoError(ts.T(), err)

	// Delete test index
	_, err = WaitUntilIndexReady(ts, ctx)
	require.NoError(ts.T(), err)

	err = ts.client.DeleteIndex(ctx, ts.idxName)
	require.NoError(ts.T(), err)

	fmt.Printf("\n %s setup suite torn down successfully\n", ts.indexType)
}

// Helper funcs
func GenerateTestIndexName() string {
	return fmt.Sprintf("index-%d", time.Now().UnixMilli())
}

func upsertVectors(ts *IntegrationTests, ctx context.Context, vectors []*Vector) error {
	_, err := WaitUntilIndexReady(ts, ctx)
	require.NoError(ts.T(), err)

	upsertVectors, err := ts.idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		buf := make([]byte, 1<<16)
		runtime.Stack(buf, true)
		fmt.Printf("Stack trace: %s\n", buf)
		return err
	}
	fmt.Printf("Upserted vectors: %v into host: %s\n", upsertVectors, ts.host)

	return nil
}

func WaitUntilIndexReady(ts *IntegrationTests, ctx context.Context) (bool, error) {
	maxRetries := 24
	delay := 5 * time.Second
	totalSeconds := 0

	for i := 0; i < maxRetries; i++ {
		index, err := ts.client.DescribeIndex(ctx, ts.idxName)
		if err != nil {
			fmt.Printf("Error describing index: %v\n", err)
		}
		if index.Status.State == Ready && index.Status.Ready {
			fmt.Printf("Index \"%s\" is ready!\n", ts.idxName)
			return true, nil
		} else {
			fmt.Printf("Index \"%s\" not ready yet, retrying... (%d/%d)\n", ts.idxName, i, maxRetries)
			time.Sleep(delay)
			totalSeconds += int(delay.Seconds())
		}
	}

	fmt.Printf("Index \"%s\" not ready after %d seconds\n", ts.idxName, totalSeconds)
	return false, nil
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
