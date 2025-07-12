package pinecone

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (ts *AdminIntegrationTests) TestOrganizations() {
	var originalOrgName string
	var orgId string

	ts.T().Run("ListOrganizations", func(t *testing.T) {
		orgs, err := ts.adminClient.Organization.List(context.Background())
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), orgs, "Expected organizations to be non-nil")
		require.Greater(ts.T(), len(orgs), 0, "Expected at least one organization in list")
		originalOrgName = orgs[0].Name
		orgId = orgs[0].Id
	})

	ts.T().Run("DescribeOrganization", func(t *testing.T) {
		descOrg, err := ts.adminClient.Organization.Describe(context.Background(), orgId)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), descOrg, "Expected organization to be non-nil")
		require.Equal(ts.T(), orgId, descOrg.Id, "Expected organization ID to match")
	})

	ts.T().Run("UpdateOrganization", func(t *testing.T) {
		newName := originalOrgName + "-updated"
		updatedOrg, err := ts.adminClient.Organization.Update(context.Background(), orgId, &UpdateOrganizationParams{
			Name: &newName,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), updatedOrg, "Expected organization to be non-nil")
		require.Equal(ts.T(), newName, updatedOrg.Name, "Expected organization name to match")

		_, err = ts.adminClient.Organization.Update(context.Background(), orgId, &UpdateOrganizationParams{
			Name: &originalOrgName,
		})
		require.NoError(ts.T(), err)
	})

	// Service accounts are associated with a single organization, and cannot create new ones currently.
	// Skip explicitly testing DeleteOrganization for now.
}

func (ts *AdminIntegrationTests) TestProjectsAndAPIKeys() {
	// Test project operations
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String()[:6])
	var newProject *Project
	var err error
	ts.T().Run("CreateProject", func(t *testing.T) {
		maxPods := 8
		forceEncryptionWithCmek := true
		newProject, err = ts.adminClient.Project.Create(context.Background(), &CreateProjectParams{
			Name:                    projectName,
			MaxPods:                 &maxPods,
			ForceEncryptionWithCmek: &forceEncryptionWithCmek,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), newProject, "Expected project to be non-nil")
		require.Equal(ts.T(), projectName, newProject.Name, "Expected project name to match")
		require.Equal(ts.T(), maxPods, newProject.MaxPods, "Expected max pods to match")
		require.Equal(ts.T(), forceEncryptionWithCmek, newProject.ForceEncryptionWithCmek, "Expected force encryption with CMEK to match")
	})

	ts.T().Run("DescribeProject", func(t *testing.T) {
		descProject, err := ts.adminClient.Project.Describe(context.Background(), newProject.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), descProject, "Expected project to be non-nil")
		require.Equal(ts.T(), descProject.Id, descProject.Id, "Expected project ID to match")
	})

	ts.T().Run("UpdateProject", func(t *testing.T) {
		newName := projectName + "-updated"
		newMaxPods := 10
		updatedProject, err := ts.adminClient.Project.Update(context.Background(), newProject.Id, &UpdateProjectParams{
			Name:    &newName,
			MaxPods: &newMaxPods,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), updatedProject, "Expected project to be non-nil")
		require.Equal(ts.T(), newName, updatedProject.Name, "Expected project name to match")

		updatedProject, err = ts.adminClient.Project.Describe(context.Background(), updatedProject.Id)
		require.Equal(ts.T(), newMaxPods, updatedProject.MaxPods, "Expected max pods to match")
	})

	ts.T().Run("ListProjects", func(t *testing.T) {
		projects, err := ts.adminClient.Project.List(context.Background())
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), projects, "Expected projects to be non-nil")
		require.Greater(ts.T(), len(projects), 0, "Expected at least one project in list")
		found := false
		for _, project := range projects {
			if project.Id == newProject.Id {
				found = true
				break
			}
		}
		require.True(ts.T(), found, "Expected project to be in project list")
	})

	// Test API key operations using project
	apiKeyName := fmt.Sprintf("test-api-key-%s", uuid.New().String()[:6])
	var newAPIKey *APIKey

	ts.T().Run("CreateAPIKey", func(t *testing.T) {
		roles := []string{"ProjectEditor", "ProjectViewer", "ControlPlaneEditor", "ControlPlaneViewer"}
		apiKeyWithSecret, err := ts.adminClient.APIKey.Create(context.Background(), newProject.Id, &CreateAPIKeyParams{
			Name:  apiKeyName,
			Roles: &roles,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), apiKeyWithSecret, "Expected API key to be non-nil")
		require.Equal(ts.T(), apiKeyName, apiKeyWithSecret.Key.Name, "Expected API key name to match")
		require.ElementsMatch(ts.T(), roles, apiKeyWithSecret.Key.Roles, "Expected API key roles to match")
		newAPIKey = &apiKeyWithSecret.Key
	})

	ts.T().Run("DescribeAPIKey", func(t *testing.T) {
		descAPIKey, err := ts.adminClient.APIKey.Describe(context.Background(), newAPIKey.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), descAPIKey, "Expected API key to be non-nil")
		require.Equal(ts.T(), newAPIKey.Id, descAPIKey.Id, "Expected API key ID to match")
	})

	ts.T().Run("ListAPIKeys", func(t *testing.T) {
		apiKeys, err := ts.adminClient.APIKey.List(context.Background(), newProject.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), apiKeys, "Expected API keys to be non-nil")
		require.Greater(ts.T(), len(apiKeys), 0, "Expected at least one API key in list")
		found := false
		for _, apiKey := range apiKeys {
			if apiKey.Id == newAPIKey.Id {
				found = true
				break
			}
		}
		require.True(ts.T(), found, "Expected API key to be in API key list")
	})

	ts.T().Run("UpdateAPIKey", func(t *testing.T) {
		newName := apiKeyName + "-updated"
		newRoles := []string{"ProjectEditor", "ProjectViewer", "ControlPlaneEditor", "ControlPlaneViewer"}
		updatedAPIKey, err := ts.adminClient.APIKey.Update(context.Background(), newAPIKey.Id, &UpdateAPIKeyParams{
			Name: &newName,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), updatedAPIKey, "Expected API key to be non-nil")
		require.Equal(ts.T(), newName, updatedAPIKey.Name, "Expected API key name to match")
		require.ElementsMatch(ts.T(), newRoles, updatedAPIKey.Roles, "Expected API key roles to match")
	})

	ts.T().Run("DeleteAPIKey", func(t *testing.T) {
		err := ts.adminClient.APIKey.Delete(context.Background(), newAPIKey.Id)
		require.NoError(ts.T(), err)
		_, err = ts.adminClient.APIKey.Describe(context.Background(), newAPIKey.Id)
		require.Error(ts.T(), err)
		require.Contains(ts.T(), err.Error(), "API Key")
		require.Contains(ts.T(), err.Error(), "not found")
	})

	// Clean up project
	ts.T().Run("DeleteProject", func(t *testing.T) {
		err := ts.adminClient.Project.Delete(context.Background(), newProject.Id)
		require.NoError(ts.T(), err)
		_, err = ts.adminClient.Project.Describe(context.Background(), newProject.Id)
		require.Error(ts.T(), err)
		require.Contains(ts.T(), err.Error(), "Project")
		require.Contains(ts.T(), err.Error(), "not found")
	})
}
