//go:build localServer

package pinecone

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type LocalIntegrationTests struct {
	suite.Suite
	client    *Client
	host      string
	dimension int32
	indexType string
	vectorIds []string
	idxConn   *IndexConnection
}

func (ts *LocalIntegrationTests) SetupSuite() {
	fmt.Printf("Local Integration Suite Setup")
	ctx := context.Background()

	// Deterministically create vectors
	vectors := GenerateVectors(10, ts.dimension, false)

	// Upsert vectors
	upsertedVectors, err := ts.idxConn.UpsertVectors(ctx, vectors)
	require.NoError(ts.T(), err)
	fmt.Printf("Upserted vectors: %v into host: %s\n", upsertedVectors, ts.host)

	// Add vector ids to the suite
	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}
	ts.vectorIds = append(ts.vectorIds, vectorIds...)
}

func (ts *LocalIntegrationTests) TearDownSuite() {
	fmt.Printf("Local Integration Suite Teardown")
}

// This is the entry point for all local integration tests
// This test function is picked up by go test and triggers the suite runs when
// the
func TestRunLocalIntegrationSuite(t *testing.T) {
	fmt.Println("Running local integration tests")
	RunLocalSuite(t)
}

func RunLocalSuite(t *testing.T) {
	fmt.Println("Running local integration tests")
	localHostPod, present := os.LookupEnv("PINECONE_INDEX_URL_POD")
	assert.True(t, present, "PINECONE_INDEX_URL_POD env variable not set")

	localHostServerless, present := os.LookupEnv("PINECONE_INDEX_URL_SERVERLESS")
	assert.True(t, present, "PINECONE_INDEX_URL_SERVERLESS env variable not set")

	dimension, present := os.LookupEnv("PINECONE_DIMENSION")
	assert.True(t, present, "PINECONE_DIMENSION env variable not set")

	parsedDimension, err := strconv.ParseInt(dimension, 10, 32)
	require.NoError(t, err)

	client, err := NewClientBase(NewClientBaseParams{})
	require.NotNil(t, client, "Client should not be nil after creation")
	require.NoError(t, err)

	idxConnPod, err := client.Index(NewIndexConnParams{Host: localHostPod})
	require.NoError(t, err)

	idxConnServerless, err := client.Index(NewIndexConnParams{Host: localHostServerless},
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	localHostPodSuite := &LocalIntegrationTests{
		client:    client,
		idxConn:   idxConnPod,
		indexType: "pods",
		host:      localHostPod,
		dimension: int32(parsedDimension),
	}

	localHostSuiteServerless := &LocalIntegrationTests{
		client:    client,
		idxConn:   idxConnServerless,
		indexType: "serverless",
		host:      localHostServerless,
		dimension: int32(parsedDimension),
	}

	suite.Run(t, localHostPodSuite)
	suite.Run(t, localHostSuiteServerless)
}

func (ts *LocalIntegrationTests) TestFetchVectors() {
	fetchVectorId := ts.vectorIds[0]

	fetchVectorsResponse, err := ts.idxConn.FetchVectors(context.Background(), []string{fetchVectorId})
	require.NoError(ts.T(), err)

	assert.NotNil(ts.T(), fetchVectorsResponse, "Fetch vectors response should not be nil")
	assert.Equal(ts.T(), 1, len(fetchVectorsResponse.Vectors), "Fetch vectors response should have 1 vector")
	assert.Equal(ts.T(), fetchVectorId, fetchVectorsResponse.Vectors[fetchVectorId].Id, "Fetched vector id should match")
}

func (ts *LocalIntegrationTests) TestQueryVectors() {
	queryVectorId := ts.vectorIds[0]
	topK := 10

	queryVectorsByIdResponse, err := ts.idxConn.QueryByVectorId(context.Background(), &QueryByVectorIdRequest{VectorId: queryVectorId, TopK: uint32(topK)})
	require.NoError(ts.T(), err)

	assert.NotNil(ts.T(), queryVectorsByIdResponse, "Query results should not be nil")
	assert.Equal(ts.T(), 1, len(queryVectorsByIdResponse.Matches), "Query results should have 10 matches")
	assert.Equal(ts.T(), queryVectorId, queryVectorsByIdResponse.Matches[0].Vector.Id, "Top query result vector id should match queryVectorId")
}

// func (ts *LocalIntegrationTests) TestUpdateVectors() {

// }

// func (ts *LocalIntegrationTests) TestDeleteVectors() {

// }
