package pinecone

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ManagementClientTests struct {
	suite.Suite
	client  ManagementClient
	project Project
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
