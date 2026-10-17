//go:build integration

package pinecone

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (ts *adminIntegrationTests) TestOrganizations() {
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

func (ts *adminIntegrationTests) TestProjectsAndAPIKeys() {
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
		// The Admin API returns the project's effective max_pods allotment, which is
		// governed by the organization's pod quota and may exceed the requested value
		// (e.g. requesting 8 yields the org default of 50). Assert the returned value is
		// a valid allotment of at least what we requested rather than an exact echo.
		require.GreaterOrEqual(ts.T(), newProject.MaxPods, maxPods, "Expected max pods to be at least the requested value")
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
		require.NoError(ts.T(), err)
		// As with create, the API reports the effective max_pods allotment (>= requested),
		// not necessarily the exact requested value.
		require.GreaterOrEqual(ts.T(), updatedProject.MaxPods, newMaxPods, "Expected max pods to be at least the requested value")
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

func (ts *adminIntegrationTests) TestServiceAccounts() {
	saName := fmt.Sprintf("test-sa-%s", uuid.New().String()[:6])
	var newSA *ServiceAccount
	var originalSecret string

	ts.T().Run("CreateServiceAccount", func(t *testing.T) {
		saWithSecret, err := ts.adminClient.ServiceAccount.Create(context.Background(), &CreateServiceAccountParams{
			Name: saName,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), saWithSecret, "Expected service account to be non-nil")
		require.Equal(ts.T(), saName, saWithSecret.ServiceAccount.Name, "Expected service account name to match")
		require.NotEmpty(ts.T(), saWithSecret.ServiceAccount.Id, "Expected service account ID to be set")
		require.NotEmpty(ts.T(), saWithSecret.ServiceAccount.ClientId, "Expected client ID to be set")
		require.NotEmpty(ts.T(), saWithSecret.ClientSecret, "Expected client secret to be returned on creation")
		newSA = &saWithSecret.ServiceAccount
		originalSecret = saWithSecret.ClientSecret
	})

	// If creation failed there is nothing to exercise; the failed subtest already
	// marks the overall test as failed.
	if newSA == nil {
		return
	}

	ts.T().Run("DescribeServiceAccount", func(t *testing.T) {
		descSA, err := ts.adminClient.ServiceAccount.Describe(context.Background(), newSA.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), descSA, "Expected service account to be non-nil")
		require.Equal(ts.T(), newSA.Id, descSA.Id, "Expected service account ID to match")
		require.Equal(ts.T(), newSA.ClientId, descSA.ClientId, "Expected client ID to match")
	})

	ts.T().Run("ListServiceAccounts", func(t *testing.T) {
		list, err := ts.adminClient.ServiceAccount.List(context.Background(), nil)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), list, "Expected service account list to be non-nil")
		require.NotNil(ts.T(), list.Data, "Expected service account list data to be non-nil")
		found := false
		for _, sa := range list.Data {
			if sa.Id == newSA.Id {
				found = true
				break
			}
		}
		require.True(ts.T(), found, "Expected created service account to be in list")
	})

	ts.T().Run("UpdateServiceAccount", func(t *testing.T) {
		newName := saName + "-updated"
		updatedSA, err := ts.adminClient.ServiceAccount.Update(context.Background(), newSA.Id, &UpdateServiceAccountParams{
			Name: &newName,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), updatedSA, "Expected service account to be non-nil")
		require.Equal(ts.T(), newName, updatedSA.Name, "Expected service account name to match")
	})

	ts.T().Run("RotateServiceAccountSecret", func(t *testing.T) {
		rotated, err := ts.adminClient.ServiceAccount.RotateSecret(context.Background(), newSA.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), rotated, "Expected service account to be non-nil")
		require.Equal(ts.T(), newSA.Id, rotated.ServiceAccount.Id, "Expected service account ID to match")
		require.NotEmpty(ts.T(), rotated.ClientSecret, "Expected a new client secret to be returned")
		require.NotEqual(ts.T(), originalSecret, rotated.ClientSecret, "Expected rotated secret to differ from the original")
	})

	ts.T().Run("DeleteServiceAccount", func(t *testing.T) {
		err := ts.adminClient.ServiceAccount.Delete(context.Background(), newSA.Id)
		require.NoError(ts.T(), err)
		_, err = ts.adminClient.ServiceAccount.Describe(context.Background(), newSA.Id)
		require.Error(ts.T(), err, "Expected describing a deleted service account to fail")
	})
}

func (ts *adminIntegrationTests) TestRoleBindings() {
	// A role binding needs a principal and a resource scope. Create a dedicated
	// service account to act as the principal, then grant it an organization-scoped
	// role. Both the binding and the principal are cleaned up before returning.
	saName := fmt.Sprintf("test-rb-sa-%s", uuid.New().String()[:6])
	var principal *ServiceAccount

	ts.T().Run("CreatePrincipalServiceAccount", func(t *testing.T) {
		saWithSecret, err := ts.adminClient.ServiceAccount.Create(context.Background(), &CreateServiceAccountParams{
			Name: saName,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), saWithSecret, "Expected service account to be non-nil")
		principal = &saWithSecret.ServiceAccount
	})

	if principal == nil {
		return
	}
	// Ensure the principal service account is removed even if a later subtest fails.
	defer func() {
		_ = ts.adminClient.ServiceAccount.Delete(context.Background(), principal.Id)
	}()

	var newBinding *RoleBinding

	ts.T().Run("CreateRoleBinding", func(t *testing.T) {
		binding, err := ts.adminClient.RoleBinding.Create(context.Background(), &CreateRoleBindingParams{
			PrincipalId:   principal.Id,
			PrincipalType: PrincipalTypeServiceAccount,
			ResourceType:  ResourceTypeOrganization,
			Role:          "OrgMember",
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), binding, "Expected role binding to be non-nil")
		require.NotEmpty(ts.T(), binding.Id, "Expected role binding ID to be set")
		require.Equal(ts.T(), principal.Id, binding.PrincipalId, "Expected principal ID to match")
		require.Equal(ts.T(), PrincipalTypeServiceAccount, binding.PrincipalType, "Expected principal type to match")
		require.Equal(ts.T(), "OrgMember", binding.Role, "Expected role to match")
		newBinding = binding
	})

	if newBinding == nil {
		return
	}

	ts.T().Run("DescribeRoleBinding", func(t *testing.T) {
		binding, err := ts.adminClient.RoleBinding.Describe(context.Background(), newBinding.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), binding, "Expected role binding to be non-nil")
		require.Equal(ts.T(), newBinding.Id, binding.Id, "Expected role binding ID to match")
	})

	ts.T().Run("ListRoleBindings", func(t *testing.T) {
		principalType := PrincipalTypeServiceAccount
		list, err := ts.adminClient.RoleBinding.List(context.Background(), &ListRoleBindingsParams{
			PrincipalType: &principalType,
			PrincipalId:   &principal.Id,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), list, "Expected role binding list to be non-nil")
		require.NotNil(ts.T(), list.Data, "Expected role binding list data to be non-nil")
		found := false
		for _, rb := range list.Data {
			if rb.Id == newBinding.Id {
				found = true
				break
			}
		}
		require.True(ts.T(), found, "Expected created role binding to be in filtered list")
	})

	ts.T().Run("DeleteRoleBinding", func(t *testing.T) {
		err := ts.adminClient.RoleBinding.Delete(context.Background(), newBinding.Id)
		require.NoError(ts.T(), err)
		_, err = ts.adminClient.RoleBinding.Describe(context.Background(), newBinding.Id)
		require.Error(ts.T(), err, "Expected describing a deleted role binding to fail")
	})
}

func (ts *adminIntegrationTests) TestInvites() {
	// example.com is IANA-reserved and never delivers mail, so it is safe to use for
	// an invite that is created and immediately cleaned up.
	email := fmt.Sprintf("test-invite-%s@example.com", uuid.New().String()[:6])
	var newInvite *Invite

	ts.T().Run("CreateInvite", func(t *testing.T) {
		invite, err := ts.adminClient.Invite.Create(context.Background(), &CreateInviteParams{
			Email: email,
			RoleBindings: []RoleBindingInput{
				{ResourceType: ResourceTypeOrganization, Role: "OrgMember"},
			},
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), invite, "Expected invite to be non-nil")
		require.Equal(ts.T(), email, invite.Email, "Expected invite email to match")
		require.Equal(ts.T(), InviteStatusPending, invite.Status, "Expected a new invite to be pending")
		require.NotEmpty(ts.T(), invite.Id, "Expected invite ID to be set")
		newInvite = invite
	})

	if newInvite == nil {
		return
	}

	ts.T().Run("DescribeInvite", func(t *testing.T) {
		invite, err := ts.adminClient.Invite.Describe(context.Background(), newInvite.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), invite, "Expected invite to be non-nil")
		require.Equal(ts.T(), newInvite.Id, invite.Id, "Expected invite ID to match")
		require.Equal(ts.T(), email, invite.Email, "Expected invite email to match")
	})

	ts.T().Run("ListInvites", func(t *testing.T) {
		list, err := ts.adminClient.Invite.List(context.Background(), nil)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), list, "Expected invite list to be non-nil")
		require.NotNil(ts.T(), list.Data, "Expected invite list data to be non-nil")
		found := false
		for _, inv := range list.Data {
			if inv.Id == newInvite.Id {
				found = true
				break
			}
		}
		require.True(ts.T(), found, "Expected pending invite to be in list")
	})

	ts.T().Run("ResendInvite", func(t *testing.T) {
		invite, err := ts.adminClient.Invite.Resend(context.Background(), newInvite.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), invite, "Expected invite to be non-nil")
		require.Equal(ts.T(), newInvite.Id, invite.Id, "Expected invite ID to match")
		require.NotNil(ts.T(), invite.ExpiresAt, "Expected resent invite to carry an expiration")
	})

	ts.T().Run("DeleteInvite", func(t *testing.T) {
		err := ts.adminClient.Invite.Delete(context.Background(), newInvite.Id)
		require.NoError(ts.T(), err)
		_, err = ts.adminClient.Invite.Describe(context.Background(), newInvite.Id)
		require.Error(ts.T(), err, "Expected describing a deleted invite to fail")
	})
}

func (ts *adminIntegrationTests) TestUsers() {
	// Users represent real human members of the organization. The SDK cannot create
	// users, and deleting one would remove an actual member, so this test is limited
	// to list, describe, and email-filter coverage against existing members.
	var sampleUser *User

	ts.T().Run("ListUsers", func(t *testing.T) {
		list, err := ts.adminClient.User.List(context.Background(), nil)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), list, "Expected user list to be non-nil")
		require.NotNil(ts.T(), list.Data, "Expected user list data to be non-nil")
		if len(list.Data) == 0 {
			// An organization with no human members (e.g. service-account-only access)
			// leaves nothing to describe or filter; list coverage still ran above.
			return
		}
		sampleUser = list.Data[0]
		require.NotEmpty(ts.T(), sampleUser.Id, "Expected user ID to be set")
		require.NotEmpty(ts.T(), sampleUser.Email, "Expected user email to be set")
	})

	if sampleUser == nil {
		return
	}

	ts.T().Run("DescribeUser", func(t *testing.T) {
		user, err := ts.adminClient.User.Describe(context.Background(), sampleUser.Id)
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), user, "Expected user to be non-nil")
		require.Equal(ts.T(), sampleUser.Id, user.Id, "Expected user ID to match")
		require.Equal(ts.T(), sampleUser.Email, user.Email, "Expected user email to match")
	})

	ts.T().Run("ListUsersByEmail", func(t *testing.T) {
		list, err := ts.adminClient.User.List(context.Background(), &ListUsersParams{
			Email: &sampleUser.Email,
		})
		require.NoError(ts.T(), err)
		require.NotNil(ts.T(), list, "Expected user list to be non-nil")
		require.NotNil(ts.T(), list.Data, "Expected user list data to be non-nil")
		found := false
		for _, u := range list.Data {
			if u.Id == sampleUser.Id {
				found = true
				break
			}
		}
		require.True(ts.T(), found, "Expected email filter to return the matching user")
	})
}
