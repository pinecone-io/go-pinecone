package pinecone

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/suite"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ManagementClientTests struct {
	suite.Suite
	client  ManagementClient
	project Project
	apiKey  APIKeyWithoutSecret
}

func TestManagementClient(t *testing.T) {
	suite.Run(t, new(ManagementClientTests))
}

func (ts *ManagementClientTests) SetupSuite() {
	apiKey := os.Getenv("ORG_API_KEY")
	require.NotEmpty(ts.T(), apiKey, "ORG_API_KEY env variable not set")

	client, err := NewManagementClient(NewManagementClientParams{ApiKey: apiKey})
	if err != nil {
		ts.FailNow(err.Error())
	}
	ts.client = *client

	projects, err := ts.client.ListProjects(context.Background())
	require.NoError(ts.T(), err, "Failed to list projects for test setup")
	require.Greater(ts.T(), len(projects), 0, "Projects list should not be empty")
	ts.project = *projects[0]

	apiKeys, err := ts.client.ListApiKeys(context.Background(), ts.project.Id)
	require.NoError(ts.T(), err, "Failed to list API keys for test setup")
	require.NotNil(ts.T(), apiKeys, "API keys in test setup should not be nil")
	require.Greater(ts.T(), len(apiKeys), 0, "API key list in test setup should not be empty")
	ts.apiKey = *apiKeys[0]
}

func (ts *ManagementClientTests) TestListProjects() {
	projects, err := ts.client.ListProjects(context.Background())
	require.NoError(ts.T(), err, "Failed to list projects")
	require.Greater(ts.T(), len(projects), 0, "Projects list should not be empty")
}

func (ts *ManagementClientTests) TestFetchProject() {
	testProjectID := ts.project.Id

	project, err := ts.client.FetchProject(context.Background(), testProjectID)
	require.NoError(ts.T(), err, "Failed to fetch project")

	require.NotNil(ts.T(), project, "Fetched project should not be nil")
	require.Equal(ts.T(), testProjectID, project.Id, "Fetched project ID should match the requested ID")
	require.Equal(ts.T(), ts.project.Name, project.Name, "Fetched project name should match the expected name")
}

func (ts *ManagementClientTests) TestCreateProject() {
	projectName := "TestProject_" + fmt.Sprint(time.Now().UnixNano())

	createdProject, err := ts.client.CreateProject(context.Background(), projectName)
	defer func() {
		if createdProject != nil {
			delErr := ts.client.DeleteProject(context.Background(), createdProject.Id)
			require.NoError(ts.T(), delErr, "Failed to clean up project")
		}
	}()

	require.NoError(ts.T(), err, "Failed to create project")
	require.NotNil(ts.T(), createdProject, "Created project should not be nil")
	require.Equal(ts.T(), projectName, createdProject.Name, "Created project name should match")
}

func (ts *ManagementClientTests) TestDeleteProject() {
	// Create a project to delete
	projectName := "TestProjectForDeletion_" + fmt.Sprint(time.Now().UnixNano())
	projectToDelete, err := ts.client.CreateProject(context.Background(), projectName)
	require.NoError(ts.T(), err, "Failed to create project for deletion")
	require.NotNil(ts.T(), projectToDelete, "Project for deletion should not be nil")

	// Attempt to delete the project
	err = ts.client.DeleteProject(context.Background(), projectToDelete.Id)
	require.NoError(ts.T(), err, "Failed to delete project")

	// Verify deletion by attempting to fetch the deleted project
	_, fetchErr := ts.client.FetchProject(context.Background(), projectToDelete.Id)
	require.Error(ts.T(), fetchErr, "Expected an error when fetching a deleted project")
}

func (ts *ManagementClientTests) TestListApiKeys() {
	apiKeys, err := ts.client.ListApiKeys(context.Background(), ts.project.Id)
	require.NoError(ts.T(), err, "Failed to list API keys")
	require.NotNil(ts.T(), apiKeys, "API keys should not be nil")
	require.Greater(ts.T(), len(apiKeys), 0, "API key list should not be empty")
}

func (ts *ManagementClientTests) TestFetchApiKey() {
	apiKeyDetails, err := ts.client.FetchApiKey(context.Background(), ts.apiKey.Id)
	require.NoError(ts.T(), err, "Failed to fetch API key details")
	require.NotNil(ts.T(), apiKeyDetails, "API key details should not be nil")
	require.Equal(ts.T(), apiKeyDetails.Id, ts.apiKey.Id, "API key ID should match")
	require.Equal(ts.T(), apiKeyDetails.Name, ts.apiKey.Name, "API key Name should match")
	require.Equal(ts.T(), apiKeyDetails.ProjectId, ts.apiKey.ProjectId, "API key's Project ID should match")
}

func (ts *ManagementClientTests) TestCreateApiKey() {
	apiKeyName := generateRandomString(6) // current limitation of Alpha-release
	newApiKey, err := ts.client.CreateApiKey(context.Background(), ts.project.Id, apiKeyName)
	defer func() {
		if newApiKey != nil {
			delErr := ts.client.DeleteApiKey(context.Background(), newApiKey.Id)
			require.NoError(ts.T(), delErr, "Failed to clean up api key")
		}
	}()

	require.NoError(ts.T(), err, "Failed to create API key")
	require.NotNil(ts.T(), newApiKey, "Newly created API key should not be nil")
	require.Equal(ts.T(), apiKeyName, newApiKey.Name, "API key name should match")
	require.NotEmpty(ts.T(), newApiKey.Secret, "Newly created API key should have a secret")
}

func (ts *ManagementClientTests) TestDeleteApiKey() {
	// Create an API key to delete
	apiKeyName := generateRandomString(6) // current limitation of Alpha-release
	apiKey, _ := ts.client.CreateApiKey(context.Background(), ts.project.Id, apiKeyName)
	err := ts.client.DeleteApiKey(context.Background(), apiKey.Id)
	require.NoError(ts.T(), err, "Failed to delete API key")
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
