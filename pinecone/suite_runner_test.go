// This file is used to run all the test suites in the package pinecone
package pinecone

import (
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

	sourceTag := "pinecone_test_go_sdk"
	client, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: sourceTag})
	require.NotNil(t, client, "Client should not be nil after creation")
	require.NoError(t, err)
	indexTags := IndexTags{"test1": "test-tag-1", "test2": "test-tag-2"}

	serverlessIdx := BuildServerlessTestIndex(client, "serverless-"+GenerateTestIndexName(), indexTags)
	podIdx := BuildPodTestIndex(client, "pods-"+GenerateTestIndexName(), indexTags)

	podTestSuite := &IntegrationTests{
		apiKey:    apiKey,
		indexType: "pods",
		host:      podIdx.Host,
		dimension: podIdx.Dimension,
		client:    client,
		sourceTag: sourceTag,
		idxName:   podIdx.Name,
		indexTags: &indexTags,
	}

	serverlessTestSuite := &IntegrationTests{
		apiKey:    apiKey,
		indexType: "serverless",
		host:      serverlessIdx.Host,
		dimension: serverlessIdx.Dimension,
		client:    client,
		sourceTag: sourceTag,
		idxName:   serverlessIdx.Name,
		indexTags: &indexTags,
	}

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}
