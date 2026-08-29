// This file is used to run all the test suites in the package pinecone
package pinecone

import (
	"fmt"
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
