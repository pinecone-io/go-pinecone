// This file is used to run all the test suites in the package pinecone
package pinecone

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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
		clientSourceTag: *clientSourceTag,
		sourceTag:       sourceTag,
		idxName:         podIdx.Name,
	}

	serverlessTestSuite := &IntegrationTests{
		host:            serverlessIdx.Host,
		dimension:       serverlessIdx.Dimension,
		apiKey:          apiKey,
		indexType:       "serverless",
		client:          client,
		clientSourceTag: *clientSourceTag,
		sourceTag:       sourceTag,
		idxName:         serverlessIdx.Name,
	}

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)

}

func TestRunSuites(t *testing.T) {
	RunSuites(t)
}
