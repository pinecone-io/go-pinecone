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
	"google.golang.org/protobuf/types/known/structpb"
)

type LocalIntegrationTests struct {
	suite.Suite
	client    *Client
	host      string
	dimension int32
	indexType string
	namespace string
	metadata  *Metadata
	vectorIds []string
	idxConns  []*IndexConnection
}

func (ts *LocalIntegrationTests) SetupSuite() {
	ctx := context.Background()

	// Deterministically create vectors
	vectors := GenerateVectors(100, ts.dimension, true, ts.metadata)

	// Get vector ids for the suite
	vectorIds := make([]string, len(vectors))
	for i, v := range vectors {
		vectorIds[i] = v.Id
	}

	// Upsert vectors into each index connection
	for _, idxConn := range ts.idxConns {
		upsertedVectors, err := idxConn.UpsertVectors(ctx, vectors)
		require.NoError(ts.T(), err)
		fmt.Printf("Upserted vectors: %v into host: %s in namespace: %s \n", upsertedVectors, ts.host, idxConn.Namespace)
	}

	ts.vectorIds = append(ts.vectorIds, vectorIds...)
}

func (ts *LocalIntegrationTests) TearDownSuite() {
	// test deleting vectors as a part of cleanup for each index connection
	for _, idxConn := range ts.idxConns {
		// Delete a slice of vectors by id
		err := idxConn.DeleteVectorsById(context.Background(), ts.vectorIds[10:20])
		require.NoError(ts.T(), err)

		// Delete vectors by filter
		if ts.indexType == "pods" {
			err = idxConn.DeleteVectorsByFilter(context.Background(), ts.metadata)
			require.NoError(ts.T(), err)
		}

		// Delete all remaining vectors
		err = idxConn.DeleteAllVectorsInNamespace(context.Background())
		require.NoError(ts.T(), err)
	}

	description, err := ts.idxConns[0].DescribeIndexStats(context.Background())
	require.NoError(ts.T(), err)
	assert.NotNil(ts.T(), description, "Index description should not be nil")
	assert.Equal(ts.T(), uint32(0), description.TotalVectorCount, "Total vector count should be 0 after deleting")
}

// This is the entry point for all local integration tests
// This test function is picked up by go test and triggers the suite runs when
// the build tag localServer is set
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

	namespace := "test-namespace"
	metadata := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"genre": {Kind: &structpb.Value_StringValue{StringValue: "classical"}},
		},
	}

	client, err := NewClientBase(NewClientBaseParams{})
	require.NotNil(t, client, "Client should not be nil after creation")
	require.NoError(t, err)

	// Create index connections for pod and serverless indexes with both default namespace
	// and a custom namespace
	var podIdxConns []*IndexConnection
	idxConnPod, err := client.Index(NewIndexConnParams{Host: localHostPod})
	require.NoError(t, err)
	podIdxConns = append(podIdxConns, idxConnPod)

	idxConnPodNamespace, err := client.Index(NewIndexConnParams{Host: localHostPod, Namespace: namespace})
	require.NoError(t, err)
	podIdxConns = append(podIdxConns, idxConnPodNamespace)

	var serverlessIdxConns []*IndexConnection
	idxConnServerless, err := client.Index(NewIndexConnParams{Host: localHostServerless},
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	serverlessIdxConns = append(serverlessIdxConns, idxConnServerless)

	idxConnServerless, err = client.Index(NewIndexConnParams{Host: localHostServerless, Namespace: namespace})
	require.NoError(t, err)
	serverlessIdxConns = append(serverlessIdxConns, idxConnServerless)

	localHostPodSuite := &LocalIntegrationTests{
		client:    client,
		idxConns:  podIdxConns,
		indexType: "pods",
		host:      localHostPod,
		namespace: namespace,
		metadata:  metadata,
		dimension: int32(parsedDimension),
	}

	localHostSuiteServerless := &LocalIntegrationTests{
		client:    client,
		idxConns:  serverlessIdxConns,
		indexType: "serverless",
		host:      localHostServerless,
		namespace: namespace,
		metadata:  metadata,
		dimension: int32(parsedDimension),
	}

	suite.Run(t, localHostPodSuite)
	suite.Run(t, localHostSuiteServerless)
}

func (ts *LocalIntegrationTests) TestFetchVectors() {
	fetchVectorId := ts.vectorIds[0]

	for _, idxConn := range ts.idxConns {
		fetchVectorsResponse, err := idxConn.FetchVectors(context.Background(), []string{fetchVectorId})
		require.NoError(ts.T(), err)

		assert.NotNil(ts.T(), fetchVectorsResponse, "Fetch vectors response should not be nil")
		assert.Equal(ts.T(), 1, len(fetchVectorsResponse.Vectors), "Fetch vectors response should have 1 vector")
		assert.Equal(ts.T(), fetchVectorId, fetchVectorsResponse.Vectors[fetchVectorId].Id, "Fetched vector id should match")
	}
}

func (ts *LocalIntegrationTests) TestQueryVectors() {
	queryVectorId := ts.vectorIds[0]
	topK := 10

	for _, idxConn := range ts.idxConns {
		queryVectorsByIdResponse, err := idxConn.QueryByVectorId(context.Background(), &QueryByVectorIdRequest{VectorId: queryVectorId, TopK: uint32(topK)})
		require.NoError(ts.T(), err)

		assert.NotNil(ts.T(), queryVectorsByIdResponse, "Query results should not be nil")
		assert.Equal(ts.T(), topK, len(queryVectorsByIdResponse.Matches), "Query results should have 10 matches")
		assert.Equal(ts.T(), queryVectorId, queryVectorsByIdResponse.Matches[0].Vector.Id, "Top query result vector id should match queryVectorId")
	}
}

func (ts *LocalIntegrationTests) TestUpdateVectors() {
	updateVectorId := ts.vectorIds[0]
	newValues := generateVectorValues(ts.dimension)

	for _, idxConn := range ts.idxConns {
		err := idxConn.UpdateVector(context.Background(), &UpdateVectorRequest{Id: updateVectorId, Values: newValues})
		require.NoError(ts.T(), err)

		fetchVectorsResponse, err := idxConn.FetchVectors(context.Background(), []string{updateVectorId})
		require.NoError(ts.T(), err)
		assert.Equal(ts.T(), newValues, fetchVectorsResponse.Vectors[updateVectorId].Values, "Updated vector values should match")
	}
}

func (ts *LocalIntegrationTests) TestDescribeIndexStats() {
	for _, idxConn := range ts.idxConns {
		description, err := idxConn.DescribeIndexStats(context.Background())
		require.NoError(ts.T(), err)

		assert.NotNil(ts.T(), description, "Index description should not be nil")
		assert.Equal(ts.T(), description.TotalVectorCount, uint32(len(ts.vectorIds)*2), "Index host should match")
	}
}

func (ts *LocalIntegrationTests) TestListVectorIds() {
	limit := uint32(25)
	// Listing vector ids is only available for serverless indexes
	if ts.indexType == "serverless" {
		for _, idxConn := range ts.idxConns {
			listVectorIdsResponse, err := idxConn.ListVectors(context.Background(), &ListVectorsRequest{
				Limit: &limit,
			})
			require.NoError(ts.T(), err)

			assert.NotNil(ts.T(), listVectorIdsResponse, "ListVectors response should not be nil")
			assert.Equal(ts.T(), limit, uint32(len(listVectorIdsResponse.VectorIds)), "ListVectors response should have %d vector ids", limit)
		}
	}
}
