// This file is used to run all the test suites in the package pinecone
package pinecone

import (
	"context"
	"os"
	"testing"

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
	apiKey, present := os.LookupEnv("PINECONE_API_KEY")
	assert.True(t, present, "PINECONE_API_KEY env variable not set")

	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	require.NotNil(t, client, "Client should not be nil after creation")
	require.NoError(t, err)

	sourceTag := "test_source_tag"
	clientSourceTag, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: sourceTag})
	require.NoError(t, err)

	serverlessIdx := BuildServerlessTestIndex(client, "serverless-"+GenerateTestIndexName())
	podIdx := BuildPodTestIndex(client, "pods-"+GenerateTestIndexName())

	podTestSuite := &IntegrationTests{
		apiKey:          apiKey,
		indexType:       "pods",
		host:            podIdx.Host,
		dimension:       podIdx.Dimension,
		client:          client,
		clientSourceTag: clientSourceTag,
		sourceTag:       sourceTag,
		idxName:         podIdx.Name,
	}

	serverlessTestSuite := &IntegrationTests{
		host:            serverlessIdx.Host,
		dimension:       serverlessIdx.Dimension,
		apiKey:          apiKey,
		indexType:       "serverless",
		client:          client,
		clientSourceTag: clientSourceTag,
		sourceTag:       sourceTag,
		idxName:         serverlessIdx.Name,
	}

	ctx := context.Background()
	done := make(chan indexReadyResponse, 2)

	// spawn goroutines to wait until indexes are ready
	go waitUntilIndexReadyWithChannel(podTestSuite, ctx, done)
	go waitUntilIndexReadyWithChannel(serverlessTestSuite, ctx, done)

	// wait until indexes are ready before proceeding
	for i := 0; i < 2; i++ {
		result := <-done
		require.True(t, result.ready, "Index %s is not ready", result.indexName)
	}

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}

func waitUntilIndexReadyWithChannel(ts *IntegrationTests, ctx context.Context, done chan<- indexReadyResponse) {
	ready, err := WaitUntilIndexReady(ts, ctx)
	if err != nil {
		require.NoError(ts.T(), err)
	}

	done <- indexReadyResponse{indexName: ts.idxName, ready: ready}
}

type indexReadyResponse struct {
	indexName string
	ready     bool
}
