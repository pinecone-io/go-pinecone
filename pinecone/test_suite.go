package pinecone

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IntegrationTests struct {
	suite.Suite
	apiKey         string
	client         *Client
	host           string
	dimension      *int32
	indexType      string
	vectorIds      []string
	idxName        string
	idxConn        *IndexConnection
	collectionName string
	sourceTag      string
	indexTags      *IndexTags
}

func (ts *IntegrationTests) SetupSuite() {
	ctx := context.Background()

	_, err := waitUntilIndexReady(ts, ctx)
	require.NoError(ts.T(), err)

	namespace := uuid.New().String()

	idxConn, err := ts.client.Index(NewIndexConnParams{
		Host:      ts.host,
		Namespace: namespace,
	})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), idxConn, "Failed to create idxConn")

	ts.idxConn = idxConn
	dim := int32(0)
	if ts.dimension != nil {
		dim = *ts.dimension
	}

	// Deterministically create vectors
	vectors := generateVectors(10, dim, false, nil)

	// Add vector ids to the suite
	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}

	// Upsert vectors
	err = upsertVectors(ts, ctx, vectors)
	if err != nil {
		log.Fatalf("Failed to upsert vectors in SetupSuite: %v", err)
	}

	// Wait for vector freshness
	err = pollIndexForFreshness(ts, ctx, vectorIds[0])
	if err != nil {
		log.Fatalf("Vector freshness failed in SetupSuite: %v", err)
	}

	// Create collection for pod index suite
	if ts.indexType == "pods" {
		createCollection(ts, ctx)
	}

	fmt.Printf("\n %s set up suite completed successfully\n", ts.indexType)

	// Poll for data freshness
}

func (ts *IntegrationTests) TearDownSuite() {
	ctx := context.Background()

	// Close index connection
	err := ts.idxConn.Close()
	require.NoError(ts.T(), err)

	// Delete collection
	if ts.collectionName != "" {
		err = ts.client.DeleteCollection(ctx, ts.collectionName)
		require.NoError(ts.T(), err)

		// Before moving on to deleting the index, wait for collection to be cleaned up
		time.Sleep(3 * time.Second)
	}

	// Delete test index
	err = ts.client.DeleteIndex(ctx, ts.idxName)

	// If the index failed to delete, wait a bit and retry cleaning up
	// Somtimes indexes are stuck upgrading, or have pending collections
	retry := 4
	for err != nil && retry > 0 {
		time.Sleep(5 * time.Second)
		fmt.Printf("Failed to delete index \"%s\". Retrying... (%d/4)\n", ts.idxName, 5-retry)
		err = ts.client.DeleteIndex(ctx, ts.idxName)
		retry--
	}

	if err != nil {
		fmt.Printf("Failed to delete index \"%s\" after 4 retries: %v\n", ts.idxName, err)
	}

	fmt.Printf("\n %s setup suite torn down successfully\n", ts.indexType)
}

// Helper funcs
func generateTestIndexName() string {
	return fmt.Sprintf("index-%d", time.Now().UnixMilli())
}

func upsertVectors(ts *IntegrationTests, ctx context.Context, vectors []*Vector) error {
	_, err := waitUntilIndexReady(ts, ctx)
	require.NoError(ts.T(), err)

	ids := make([]string, len(vectors))
	for i, v := range vectors {
		ids[i] = v.Id
	}

	upsertVectors, err := ts.idxConn.UpsertVectors(ctx, vectors)
	require.NoError(ts.T(), err)
	fmt.Printf("Upserted vectors: %v into host: %s\n", upsertVectors, ts.host)

	ts.vectorIds = append(ts.vectorIds, ids...)

	return nil
}

func createCollection(ts *IntegrationTests, ctx context.Context) {
	name := uuid.New().String()
	sourceIndex := ts.idxName

	ts.collectionName = name

	collection, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
		Name:   name,
		Source: sourceIndex,
	})

	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, collection.Name)
}

func waitUntilIndexReady(ts *IntegrationTests, ctx context.Context) (bool, error) {
	start := time.Now()
	delay := 5 * time.Second
	maxWaitTimeSeconds := 280 * time.Second

	for {
		index, err := ts.client.DescribeIndex(ctx, ts.idxName)
		require.NoError(ts.T(), err)

		if index.Status.Ready && index.Status.State == Ready {
			fmt.Printf("Index \"%s\" is ready after %f seconds\n", ts.idxName, time.Since(start).Seconds())
			return true, err
		}

		totalSeconds := time.Since(start)

		if totalSeconds >= maxWaitTimeSeconds {
			return false, fmt.Errorf("Index \"%s\" not ready after %f seconds", ts.idxName, totalSeconds.Seconds())
		}

		fmt.Printf("Index \"%s\" not ready yet, retrying... (%f/%f)\n", ts.idxName, totalSeconds.Seconds(), maxWaitTimeSeconds.Seconds())
		time.Sleep(delay)
	}
}

func generateVectors(numOfVectors int, dimension int32, isSparse bool, metadata *Metadata) []*Vector {
	vectors := make([]*Vector, numOfVectors)

	for i := 0; i < int(numOfVectors); i++ {
		vectors[i] = &Vector{
			Id: fmt.Sprintf("vector-%d", i),
		}

		if isSparse {
			var sparseValues SparseValues
			for j := 0; j < int(dimension); j++ {
				sparseValues.Indices = append(sparseValues.Indices, uint32(j))
			}
			values := generateVectorValues(dimension)
			sparseValues.Values = *values
			vectors[i].SparseValues = &sparseValues
		} else {
			values := generateVectorValues(dimension)
			vectors[i].Values = values
		}

		if metadata != nil {
			vectors[i].Metadata = metadata
		}
	}

	return vectors
}

func generateVectorValues(dimension int32) *[]float32 {
	maxInt := 1000000 // A large integer to normalize the float values
	values := make([]float32, dimension)

	for i := int32(0); i < dimension; i++ {
		// Generate a random integer and normalize it to the range [0, 1)
		values[i] = float32(rand.Intn(maxInt)) / float32(maxInt)
	}

	return &values
}

func buildServerlessTestIndex(in *Client, idxName string, tags IndexTags) *Index {
	ctx := context.Background()
	dimension := int32(setDimensionsForTestIndexes())
	metric := Cosine

	fmt.Printf("Creating Serverless index: %s\n", idxName)
	serverlessIdx, err := in.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:      idxName,
		Dimension: &dimension,
		Metric:    &metric,
		Region:    "us-east-1",
		Cloud:     "aws",
		Tags:      &tags,
	})
	if err != nil {
		log.Fatalf("Failed to create Serverless index \"%s\" in integration test: %v", err, idxName)
	} else {
		fmt.Printf("Successfully created a new Serverless index: %s!\n", idxName)
	}
	return serverlessIdx
}

func buildPodTestIndex(in *Client, name string, tags IndexTags) *Index {
	ctx := context.Background()
	metric := Cosine

	fmt.Printf("Creating pod index: %s\n", name)
	podIdx, err := in.CreatePodIndex(ctx, &CreatePodIndexRequest{
		Name:        name,
		Dimension:   int32(setDimensionsForTestIndexes()),
		Metric:      &metric,
		Environment: "us-east-1-aws",
		PodType:     "p1",
		Tags:        &tags,
	})
	if err != nil {
		log.Fatalf("Failed to create pod index in buildPodTestIndex test: %v", err)
	} else {
		fmt.Printf("Successfully created a new pod index: %s!\n", name)
	}
	return podIdx
}

func retryAssertions(t *testing.T, maxRetries int, delay time.Duration, fn func() error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// function call passed, we return
		if err := fn(); err == nil {
			return
		} else if attempt < maxRetries {
			t.Logf("Attempt %d/%d failed: %+v. Retrying in %d...", attempt, maxRetries, err, delay)
			time.Sleep(delay)
		} else {
			t.Fatalf("Test failed after %d attempts: %+v", maxRetries, err)
		}
	}
}

func retryAssertionsWithDefaults(t *testing.T, fn func() error) {
	retryAssertions(t, 30, 5*time.Second, fn)
}

func pollIndexForFreshness(ts *IntegrationTests, ctx context.Context, sampleId string) error {
	maxSleep := 240 * time.Second
	delay := 5 * time.Second
	totalWait := 0 * time.Second

	fetchResp, _ := ts.idxConn.FetchVectors(ctx, []string{sampleId})
	queryResp, _ := ts.idxConn.QueryByVectorId(ctx, &QueryByVectorIdRequest{VectorId: sampleId, TopK: 1})
	for len(fetchResp.Vectors) == 0 && len(queryResp.Matches) == 0 {
		if totalWait >= maxSleep {
			return fmt.Errorf("timed out waiting for vector freshness")
		}
		fmt.Printf("Vector not fresh for id: %s, waiting %+v seconds...\n", sampleId, delay.Seconds())
		time.Sleep(delay)
		totalWait += delay

		fetchResp, _ = ts.idxConn.FetchVectors(ctx, []string{sampleId})
		queryResp, _ = ts.idxConn.QueryByVectorId(ctx, &QueryByVectorIdRequest{VectorId: sampleId, TopK: 1})
	}
	return nil
}

func setDimensionsForTestIndexes() uint32 {
	return uint32(5)
}
