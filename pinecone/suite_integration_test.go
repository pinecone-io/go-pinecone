//go:build integration

package pinecone

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// This is the entry point for all integration tests
// This test function is picked up by go test and triggers the suite runs
func TestRunSuites(t *testing.T) {
	RunSuites(t)
}

func RunSuites(t *testing.T) {
	apiKey, apiKeyPresent := os.LookupEnv("PINECONE_API_KEY")
	clientId, clientIdPresent := os.LookupEnv("PINECONE_CLIENT_ID")
	clientSecret, clientSecretPresent := os.LookupEnv("PINECONE_CLIENT_SECRET")
	assert.True(t, apiKeyPresent, "PINECONE_API_KEY env variable not set")
	assert.True(t, clientIdPresent, "PINECONE_CLIENT_ID env variable not set")
	assert.True(t, clientSecretPresent, "PINECONE_CLIENT_SECRET env variable not set")

	// Check if we're skipping suites via environment variables
	skipAdminSuite, skipAdminSuitePresent := os.LookupEnv("PINECONE_SKIP_ADMIN")
	skipServerlessSuite, skipServerlessSuitePresent := os.LookupEnv("PINECONE_SKIP_SERVERLESS")

	skipAdmin := false
	skipServerless := false
	if skipAdminSuitePresent && skipAdminSuite == "true" {
		skipAdmin = true
	}
	if skipServerlessSuitePresent && skipServerlessSuite == "true" {
		skipServerless = true
	}

	sourceTag := "pinecone_test_go_sdk"
	client, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: sourceTag})
	require.NotNil(t, client, "Client should not be nil after creation")
	require.NoError(t, err)
	indexTags := IndexTags{"test1": "test-tag-1", "test2": "test-tag-2"}

	adminClient, err := NewAdminClient(NewAdminClientParams{
		ClientId:     clientId,
		ClientSecret: clientSecret,
	})
	require.NoError(t, err)
	require.NotNil(t, adminClient, "AdminClient should not be nil after creation")

	// Metadata schemas are no longer declared at index creation under 2026-07;
	// metadata fields are indexed automatically at upsert.
	// The pod suite is gone: the 2026-07 API rejects pod index creation, so there is no pod
	// index to run it against. Existing pod indexes remain served, which unit tests cover.
	serverlessIdx := buildServerlessTestIndex(client, "serverless-"+generateTestIndexName(), indexTags, nil)

	serverlessTestSuite := &integrationTests{
		apiKey:    apiKey,
		indexType: "serverless",
		host:      serverlessIdx.Host,
		dimension: serverlessIdx.Dimension,
		client:    client,
		sourceTag: sourceTag,
		idxName:   serverlessIdx.Name,
		indexTags: &indexTags,
	}

	adminTestSuite := &adminIntegrationTests{
		clientId:     clientId,
		clientSecret: clientSecret,
		adminClient:  adminClient,
	}

	if !skipAdmin {
		suite.Run(t, adminTestSuite)
	} else {
		fmt.Printf("Skipping admin suite. PINECONE_SKIP_ADMIN is set to %v\n", skipAdmin)
	}
	if !skipServerless {
		suite.Run(t, serverlessTestSuite)
	} else {
		fmt.Printf("Skipping serverless suite. PINECONE_SKIP_SERVERLESS is set to %v\n", skipServerless)
	}
}

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

	_, err := waitUntilIndexReady(ts, ctx, ts.idxName)
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
	retries := 30
	for retries > 0 {
		time.Sleep(2 * time.Second)
		backupDesc, err := ts.client.DescribeBackup(ctx, ts.backupId)
		require.NoError(ts.T(), err)

		if backupDesc.Status == "Ready" || backupDesc.Status == "Completed" || backupDesc.Status == "Failed" {
			fmt.Printf("Backup \"%s\" is ready with status: %s\n", ts.backupId, backupDesc.Status)
			return
		}

		fmt.Printf("Backup \"%s\" not ready yet, retrying... (%d retries left)\n", ts.backupId, retries-1)
		retries--
	}
}

func waitUntilIndexReady(ts *integrationTests, ctx context.Context, indexName string) (bool, error) {
	start := time.Now()
	delay := 5 * time.Second
	maxWaitTimeSeconds := 280 * time.Second

	for {
		index, err := ts.client.DescribeIndex(ctx, indexName)
		require.NoError(ts.T(), err)

		if index.Status.Ready && index.Status.State == IndexStatusStateReady {
			fmt.Printf("Index \"%s\" is ready after %f seconds\n", indexName, time.Since(start).Seconds())
			return true, err
		}

		totalSeconds := time.Since(start)

		if totalSeconds >= maxWaitTimeSeconds {
			return false, fmt.Errorf("Index \"%s\" not ready after %f seconds", indexName, totalSeconds.Seconds())
		}

		fmt.Printf("Index \"%s\" not ready yet, retrying... (%f/%f)\n", indexName, totalSeconds.Seconds(), maxWaitTimeSeconds.Seconds())
		time.Sleep(delay)
	}
}

func buildServerlessTestIndex(in *Client, idxName string, tags IndexTags, readCapacity *ReadCapacityParams) *Index {
	ctx := context.Background()
	dimension := int32(setDimensionsForTestIndexes())
	metric := IndexMetricCosine

	fmt.Printf("Creating Serverless index: %s\n", idxName)
	serverlessIdx, err := in.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:         idxName,
		Dimension:    &dimension,
		Metric:       &metric,
		Region:       "us-east-1",
		Cloud:        "aws",
		Tags:         &tags,
		ReadCapacity: readCapacity,
	})
	if err != nil {
		log.Fatalf("Failed to create Serverless index \"%s\" in integration test: %v", err, idxName)
	} else {
		fmt.Printf("Successfully created a new Serverless index: %s!\n", idxName)
	}
	return serverlessIdx
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
	retryAssertions(t, 30, 3*time.Second, fn)
}

// retryAssertionsWithExtendedTimeout allows a longer convergence window for
// operations whose effects propagate slowly under eventual consistency (e.g.
// delete-by-filter, which must reconcile a metadata query against the freshly
// mutated index). 60 attempts * 3s = up to 180s.
func retryAssertionsWithExtendedTimeout(t *testing.T, fn func() error) {
	retryAssertions(t, 60, 3*time.Second, fn)
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
