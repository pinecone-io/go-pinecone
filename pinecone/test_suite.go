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

type integrationTests struct {
	suite.Suite
	apiKey                       string
	client                       *Client
	host                         string
	dimension                    *int32
	indexType                    string
	vectorIds                    []string
	idxName                      string
	backupId                     string
	idxConn                      *IndexConnection
	collectionName               string
	sourceTag                    string
	indexTags                    *IndexTags
	schema                       *MetadataSchema
	namespaces                   []string
	vectorsWithClassicalMetadata []string
	vectorsWithRockMetadata      []string
}

type adminIntegrationTests struct {
	suite.Suite
	clientId     string
	clientSecret string
	adminClient  *AdminClient
}

func (ts *integrationTests) SetupSuite() {
	ctx := context.Background()

	_, err := waitUntilIndexReady(ts, ctx)
	require.NoError(ts.T(), err)

	namespace1 := uuid.New().String()
	namespace2 := uuid.New().String()
	namespace3 := uuid.New().String()
	namespace4 := uuid.New().String()
	namespaces := append([]string{}, namespace1, namespace2, namespace3, namespace4)

	idxConn, err := ts.client.Index(NewIndexConnParams{
		Host:      ts.host,
		Namespace: namespace1,
	})

	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), idxConn, "Failed to create idxConn")

	ts.idxConn = idxConn
	ts.namespaces = namespaces
	dim := int32(0)
	if ts.dimension != nil {
		dim = *ts.dimension
	}

	// Deterministically create vectors
	vectors := generateVectors(10, dim, false, nil)

	// Create vectors with classical metadata for testing
	classicalVectors := make([]*Vector, 5)
	classicalVectorIds := make([]string, 5)
	for i := 0; i < 5; i++ {
		metadataMap := map[string]interface{}{
			"genre": "classical",
			"year":  2020 + i,
		}
		metadata, err := NewMetadata(metadataMap)
		if err != nil {
			log.Fatalf("Failed to create classical metadata in SetupSuite: %v", err)
		}

		values := generateVectorValues(dim)
		classicalVectors[i] = &Vector{
			Id:       fmt.Sprintf("classical-vector-%d", i),
			Values:   values,
			Metadata: metadata,
		}
		classicalVectorIds[i] = classicalVectors[i].Id
	}

	// Create vectors with rock metadata for testing
	rockVectors := make([]*Vector, 3)
	rockVectorIds := make([]string, 3)
	for i := 0; i < 3; i++ {
		metadataMap := map[string]interface{}{
			"genre": "rock",
			"year":  2021,
		}
		metadata, err := NewMetadata(metadataMap)
		if err != nil {
			log.Fatalf("Failed to create rock metadata in SetupSuite: %v", err)
		}

		values := generateVectorValues(dim)
		rockVectors[i] = &Vector{
			Id:       fmt.Sprintf("rock-vector-%d", i),
			Values:   values,
			Metadata: metadata,
		}
		rockVectorIds[i] = rockVectors[i].Id
	}

	// Combine all vectors
	allVectors := append(vectors, classicalVectors...)
	allVectors = append(allVectors, rockVectors...)

	// Add vector ids to the suite
	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}
	ts.vectorIds = vectorIds

	// Upsert all vectors into each namespace
	for _, ns := range namespaces {
		idxConnNamespaced := ts.idxConn.WithNamespace(ns)
		_, err = idxConnNamespaced.UpsertVectors(ctx, allVectors)
		if err != nil {
			log.Fatalf("Failed to upsert vectors in SetupSuite: %v to namespace: %v", err, idxConnNamespaced.namespace)
		}
	}

	// Store metadata vector IDs
	ts.vectorsWithClassicalMetadata = classicalVectorIds
	ts.vectorsWithRockMetadata = rockVectorIds

	// Wait for vector freshness
	err = pollIndexForFreshness(ts, ctx, vectorIds[0])
	if err != nil {
		log.Fatalf("Vector freshness failed in SetupSuite: %v", err)
	}

	// Create collection for pod index suite
	if ts.indexType == "pods" {
		createCollection(ts, ctx)
	}

	// Create backup for serverless index suite
	if ts.indexType == "serverless" {
		createBackup(ts, ctx)
	}

	fmt.Printf("\n %s set up suite completed successfully\n", ts.indexType)
}

func (ts *integrationTests) TearDownSuite() {
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
	// Sometimes indexes are stuck upgrading, or have pending collections
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

	// Delete backup
	if ts.backupId != "" {
		err = ts.client.DeleteBackup(ctx, ts.backupId)
		if err != nil {
			fmt.Printf("Failed to delete backup \"%s\": %v\n", ts.backupId, err)
		}
	}

	fmt.Printf("\n %s setup suite torn down successfully\n", ts.indexType)
}

// Helper funcs
func generateTestIndexName() string {
	return fmt.Sprintf("index-%d", time.Now().UnixMilli())
}

func createCollection(ts *integrationTests, ctx context.Context) {
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

func createBackup(ts *integrationTests, ctx context.Context) {
	backupName := fmt.Sprintf("backup-%s", uuid.New().String())
	backupDesc := fmt.Sprintf("Backup created for index %s for Pinecone integration tests", ts.idxName)
	fmt.Printf("Creating backup: %s for index: %s\n", backupName, ts.idxName)

	backup, err := ts.client.CreateBackup(ctx, &CreateBackupParams{
		IndexName:   ts.idxName,
		Name:        &backupName,
		Description: &backupDesc,
	})

	require.NoError(ts.T(), err)
	require.Equal(ts.T(), backupName, *backup.Name)
	require.Equal(ts.T(), backupDesc, *backup.Description)
	ts.backupId = backup.BackupId

	fmt.Printf("Successfully created backup with ID: %s\n", ts.backupId)
	fmt.Printf("Waiting for backup to complete...\n")
	retries := 5
	for retries > 0 {
		time.Sleep(2 * time.Second)
		backupDesc, err := ts.client.DescribeBackup(ctx, ts.backupId)
		require.NoError(ts.T(), err)

		if backupDesc.Status == "Ready" || backupDesc.Status == "Failed" {
			fmt.Printf("Backup \"%s\" is ready with status: %s\n", ts.backupId, backupDesc.Status)
			return
		}

		fmt.Printf("Backup \"%s\" not ready yet, retrying... (%d retries left)\n", ts.backupId, retries-1)
		retries--
	}
}

func waitUntilIndexReady(ts *integrationTests, ctx context.Context) (bool, error) {
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

func buildServerlessTestIndex(in *Client, idxName string, tags IndexTags, schema *MetadataSchema, readCapacity *ReadCapacityParams) *Index {
	ctx := context.Background()
	dimension := int32(setDimensionsForTestIndexes())
	metric := Cosine

	fmt.Printf("Creating Serverless index: %s\n", idxName)
	serverlessIdx, err := in.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:         idxName,
		Dimension:    &dimension,
		Metric:       &metric,
		Region:       "us-east-1",
		Cloud:        "aws",
		Tags:         &tags,
		Schema:       schema,
		ReadCapacity: readCapacity,
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
			t.Logf("Attempt %d/%d failed: %+v. Retrying in %f...", attempt, maxRetries, err, delay.Seconds())
			time.Sleep(delay)
		} else {
			t.Fatalf("Test failed after %d attempts: %+v", maxRetries, err)
		}
	}
}

func retryAssertionsWithDefaults(t *testing.T, fn func() error) {
	retryAssertions(t, 30, 5*time.Second, fn)
}

func pollIndexForFreshness(ts *integrationTests, ctx context.Context, sampleId string) error {
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
