package pinecone

import (
	"context"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type ManagementClientTests struct {
	suite.Suite
	client ManagementClient
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
}

func (ts *ManagementClientTests) TestListProjects() {
	projects, err := ts.client.ListProjects(context.Background())
	require.NoError(ts.T(), err, "Failed to list projects")
	require.Greater(ts.T(), len(projects), 0, "Projects list should not be empty")
}
