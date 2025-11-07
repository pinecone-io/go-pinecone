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
	apiKey, apiKeyPresent := os.LookupEnv("PINECONE_API_KEY")
	clientId, clientIdPresent := os.LookupEnv("PINECONE_CLIENT_ID")
	clientSecret, clientSecretPresent := os.LookupEnv("PINECONE_CLIENT_SECRET")
	assert.True(t, apiKeyPresent, "PINECONE_API_KEY env variable not set")
	assert.True(t, clientIdPresent, "PINECONE_CLIENT_ID env variable not set")
	assert.True(t, clientSecretPresent, "PINECONE_CLIENT_SECRET env variable not set")

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

	// Create a test schema with filterable fields
	testSchema := &MetadataSchema{
		Fields: map[string]MetadataSchemaField{
			"genre": {Filterable: true},
			"year":  {Filterable: true},
		},
	}
	serverlessIdx := buildServerlessTestIndex(client, "serverless-"+generateTestIndexName(), indexTags, testSchema, nil)
	podIdx := buildPodTestIndex(client, "pods-"+generateTestIndexName(), indexTags)

	podTestSuite := &integrationTests{
		apiKey:    apiKey,
		indexType: "pods",
		host:      podIdx.Host,
		dimension: podIdx.Dimension,
		client:    client,
		sourceTag: sourceTag,
		idxName:   podIdx.Name,
		indexTags: &indexTags,
	}

	serverlessTestSuite := &integrationTests{
		apiKey:    apiKey,
		indexType: "serverless",
		host:      serverlessIdx.Host,
		dimension: serverlessIdx.Dimension,
		client:    client,
		sourceTag: sourceTag,
		idxName:   serverlessIdx.Name,
		indexTags: &indexTags,
		schema:    testSchema,
	}

	adminTestSuite := &adminIntegrationTests{
		clientId:     clientId,
		clientSecret: clientSecret,
		adminClient:  adminClient,
	}

	suite.Run(t, adminTestSuite)
	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}
