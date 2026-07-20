package pinecone

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/v5/internal/gen/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests:
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

// Unit tests:
func TestNewAdminClientWithContextUnit(t *testing.T) {
	// grab global env vars, and unset so they don't interfere with unit tests
	osClientId := os.Getenv("PINECONE_CLIENT_ID")
	osClientSecret := os.Getenv("PINECONE_CLIENT_SECRET")
	osAccessToken := os.Getenv("PINECONE_ACCESS_TOKEN")
	os.Unsetenv("PINECONE_CLIENT_ID")
	os.Unsetenv("PINECONE_CLIENT_SECRET")
	os.Unsetenv("PINECONE_ACCESS_TOKEN")

	ctx := context.Background()

	t.Run("access token provided", func(t *testing.T) {
		// mock admin.NewClient
		called := false
		newAdminClient = func(url string, opts ...admin.ClientOption) (*admin.Client, error) {
			called = true
			return &admin.Client{}, nil
		}
		defer func() { newAdminClient = admin.NewClient }()

		in := NewAdminClientParams{
			AccessToken: "test-token",
		}
		client, err := NewAdminClientWithContext(ctx, in)
		assert.NoError(t, err)
		assert.True(t, called)
		assert.NotNil(t, client)
	})

	t.Run("client ID and secret provided", func(t *testing.T) {
		clientId := "test-client-id"
		clientSecret := "test-client-secret"

		// mock admin.NewClient
		newAdminClient = func(url string, opts ...admin.ClientOption) (*admin.Client, error) {
			return &admin.Client{}, nil
		}
		defer func() { newAdminClient = admin.NewClient }()

		// mock getAuthToken
		getAuthTokenFunc = func(ctx context.Context, id, secret string, opts ...admin.ClientOption) (string, error) {
			assert.Equal(t, clientId, id)
			assert.Equal(t, clientSecret, secret)
			return "mock-token", nil
		}
		defer func() { getAuthTokenFunc = getAuthToken }()

		in := NewAdminClientParams{
			ClientId:     clientId,
			ClientSecret: clientSecret,
		}
		client, err := NewAdminClientWithContext(ctx, in)
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("missing client ID, secret, and access token", func(t *testing.T) {
		in := NewAdminClientParams{}
		client, err := NewAdminClientWithContext(ctx, in)
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no ClientId provided")

		in = NewAdminClientParams{
			ClientId: "test-client-id",
		}
		client, err = NewAdminClientWithContext(ctx, in)
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no ClientSecret provided")
	})

	// restore global env vars
	os.Setenv("PINECONE_CLIENT_ID", osClientId)
	os.Setenv("PINECONE_CLIENT_SECRET", osClientSecret)
	os.Setenv("PINECONE_ACCESS_TOKEN", osAccessToken)
}

func TestToRoleBindingUnit(t *testing.T) {
	id := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	adminRoleBinding := admin.RoleBinding{
		Id:            id,
		PrincipalId:   "principal-id",
		PrincipalType: "service_account",
		ResourceId:    "resource-id",
		ResourceType:  "organization",
		Role:          "OrgMember",
		CreatedAt:     createdAt,
	}

	roleBinding := toRoleBinding(adminRoleBinding)

	require.NotNil(t, roleBinding)
	assert.Equal(t, id.String(), roleBinding.Id, "expected UUID to be stringified")
	assert.Equal(t, "principal-id", roleBinding.PrincipalId)
	assert.Equal(t, PrincipalTypeServiceAccount, roleBinding.PrincipalType)
	assert.Equal(t, "resource-id", roleBinding.ResourceId)
	assert.Equal(t, ResourceTypeOrganization, roleBinding.ResourceType)
	assert.Equal(t, "OrgMember", roleBinding.Role)
	assert.Equal(t, createdAt, roleBinding.CreatedAt)
}

func TestToRoleBindingListUnit(t *testing.T) {
	t.Run("populated data with pagination", func(t *testing.T) {
		next := "next-token"
		adminList := admin.RoleBindingList{
			Data: []admin.RoleBinding{
				{Id: uuid.New(), PrincipalType: "user", ResourceType: "project", Role: "ProjectEditor"},
				{Id: uuid.New(), PrincipalType: "api_key", ResourceType: "organization", Role: "OrgOwner"},
			},
			Pagination: &struct {
				Next *string `json:"next,omitempty"`
			}{Next: &next},
		}

		list := toRoleBindingList(adminList)

		require.NotNil(t, list)
		require.Len(t, list.Data, 2)
		assert.Equal(t, PrincipalTypeUser, list.Data[0].PrincipalType)
		assert.Equal(t, PrincipalTypeApiKey, list.Data[1].PrincipalType)
		require.NotNil(t, list.Pagination)
		assert.Equal(t, next, list.Pagination.Next)
	})

	t.Run("nil pagination envelope yields nil pagination", func(t *testing.T) {
		adminList := admin.RoleBindingList{
			Data:       []admin.RoleBinding{{Id: uuid.New()}},
			Pagination: nil,
		}

		list := toRoleBindingList(adminList)

		require.NotNil(t, list)
		require.Len(t, list.Data, 1)
		assert.Nil(t, list.Pagination, "expected nil pagination when envelope is absent")
	})

	t.Run("pagination envelope with nil Next yields nil pagination", func(t *testing.T) {
		adminList := admin.RoleBindingList{
			Data: []admin.RoleBinding{},
			Pagination: &struct {
				Next *string `json:"next,omitempty"`
			}{Next: nil},
		}

		list := toRoleBindingList(adminList)

		require.NotNil(t, list)
		assert.NotNil(t, list.Data, "expected non-nil (empty) data slice")
		assert.Len(t, list.Data, 0)
		assert.Nil(t, list.Pagination, "expected nil pagination when Next is nil")
	})
}

func TestRoleBindingCreateNilParamsUnit(t *testing.T) {
	client := &DefaultRoleBindingClient{}
	roleBinding, err := client.Create(context.Background(), nil)
	assert.Nil(t, roleBinding)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CreateRoleBindingParams")
}

func TestRoleBindingInvalidUUIDUnit(t *testing.T) {
	client := &DefaultRoleBindingClient{}

	t.Run("Describe", func(t *testing.T) {
		roleBinding, err := client.Describe(context.Background(), "not-a-uuid")
		assert.Nil(t, roleBinding)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid roleBindingId")
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(context.Background(), "not-a-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid roleBindingId")
	})
}

func TestToRoleBindingInputUnit(t *testing.T) {
	t.Run("project scope with resource ID", func(t *testing.T) {
		resourceId := "project-id"
		input := toRoleBindingInput(RoleBindingInput{
			ResourceType: ResourceTypeProject,
			Role:         "ProjectEditor",
			ResourceId:   &resourceId,
		})
		assert.Equal(t, "project", input.ResourceType)
		assert.Equal(t, "ProjectEditor", input.Role)
		require.NotNil(t, input.ResourceId)
		assert.Equal(t, resourceId, *input.ResourceId)
	})

	t.Run("organization scope omits resource ID", func(t *testing.T) {
		input := toRoleBindingInput(RoleBindingInput{
			ResourceType: ResourceTypeOrganization,
			Role:         "OrgMember",
		})
		assert.Equal(t, "organization", input.ResourceType)
		assert.Equal(t, "OrgMember", input.Role)
		assert.Nil(t, input.ResourceId)
	})
}

func TestToServiceAccountUnit(t *testing.T) {
	id := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	updatedAt := createdAt.Add(time.Hour)
	adminServiceAccount := admin.ServiceAccount{
		Id:        id,
		Name:      "my-service-account",
		ClientId:  "client-id",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	serviceAccount := toServiceAccount(adminServiceAccount)

	require.NotNil(t, serviceAccount)
	assert.Equal(t, id.String(), serviceAccount.Id, "expected UUID to be stringified")
	assert.Equal(t, "my-service-account", serviceAccount.Name)
	assert.Equal(t, "client-id", serviceAccount.ClientId)
	assert.Equal(t, createdAt, serviceAccount.CreatedAt)
	assert.Equal(t, updatedAt, serviceAccount.UpdatedAt)
}

func TestToServiceAccountWithSecretUnit(t *testing.T) {
	id := uuid.New()
	adminServiceAccount := admin.ServiceAccountWithSecret{
		ServiceAccount: admin.ServiceAccount{
			Id:   id,
			Name: "my-service-account",
		},
		ClientSecret: "super-secret-value",
	}

	serviceAccount := toServiceAccountWithSecret(adminServiceAccount)

	require.NotNil(t, serviceAccount)
	assert.Equal(t, id.String(), serviceAccount.ServiceAccount.Id)
	assert.Equal(t, "my-service-account", serviceAccount.ServiceAccount.Name)
	assert.Equal(t, "super-secret-value", serviceAccount.ClientSecret)
}

func TestToServiceAccountListUnit(t *testing.T) {
	t.Run("populated data with pagination", func(t *testing.T) {
		next := "next-token"
		adminList := admin.ServiceAccountList{
			Data: []admin.ServiceAccount{
				{Id: uuid.New(), Name: "sa-1"},
				{Id: uuid.New(), Name: "sa-2"},
			},
			Pagination: &struct {
				Next *string `json:"next,omitempty"`
			}{Next: &next},
		}

		list := toServiceAccountList(adminList)

		require.NotNil(t, list)
		require.Len(t, list.Data, 2)
		assert.Equal(t, "sa-1", list.Data[0].Name)
		assert.Equal(t, "sa-2", list.Data[1].Name)
		require.NotNil(t, list.Pagination)
		assert.Equal(t, next, list.Pagination.Next)
	})

	t.Run("nil pagination envelope yields nil pagination", func(t *testing.T) {
		adminList := admin.ServiceAccountList{
			Data:       []admin.ServiceAccount{},
			Pagination: nil,
		}

		list := toServiceAccountList(adminList)

		require.NotNil(t, list)
		assert.NotNil(t, list.Data, "expected non-nil (empty) data slice")
		assert.Len(t, list.Data, 0)
		assert.Nil(t, list.Pagination, "expected nil pagination when envelope is absent")
	})
}

func TestServiceAccountNilParamsUnit(t *testing.T) {
	client := &DefaultServiceAccountClient{}

	t.Run("Create", func(t *testing.T) {
		serviceAccount, err := client.Create(context.Background(), nil)
		assert.Nil(t, serviceAccount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CreateServiceAccountParams")
	})

	t.Run("Update", func(t *testing.T) {
		serviceAccount, err := client.Update(context.Background(), uuid.New().String(), nil)
		assert.Nil(t, serviceAccount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UpdateServiceAccountParams")
	})
}

func TestServiceAccountInvalidUUIDUnit(t *testing.T) {
	client := &DefaultServiceAccountClient{}
	name := "renamed"

	t.Run("Describe", func(t *testing.T) {
		serviceAccount, err := client.Describe(context.Background(), "not-a-uuid")
		assert.Nil(t, serviceAccount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid serviceAccountId")
	})

	t.Run("Update", func(t *testing.T) {
		serviceAccount, err := client.Update(context.Background(), "not-a-uuid", &UpdateServiceAccountParams{Name: &name})
		assert.Nil(t, serviceAccount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid serviceAccountId")
	})

	t.Run("RotateSecret", func(t *testing.T) {
		serviceAccount, err := client.RotateSecret(context.Background(), "not-a-uuid")
		assert.Nil(t, serviceAccount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid serviceAccountId")
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(context.Background(), "not-a-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid serviceAccountId")
	})
}

func TestToInviteUnit(t *testing.T) {
	t.Run("pending invite with expiry and no processed time", func(t *testing.T) {
		id := uuid.New()
		createdAt := time.Now().UTC().Truncate(time.Second)
		expiresAt := createdAt.Add(7 * 24 * time.Hour)
		adminInvite := admin.Invite{
			Id:          id,
			Email:       "teammate@example.com",
			Status:      "pending",
			CreatedAt:   createdAt,
			ExpiresAt:   &expiresAt,
			ProcessedAt: nil,
		}

		invite := toInvite(adminInvite)

		require.NotNil(t, invite)
		assert.Equal(t, id.String(), invite.Id, "expected UUID to be stringified")
		assert.Equal(t, "teammate@example.com", invite.Email, "expected email to be stringified")
		assert.Equal(t, InviteStatusPending, invite.Status)
		assert.Equal(t, createdAt, invite.CreatedAt)
		require.NotNil(t, invite.ExpiresAt)
		assert.Equal(t, expiresAt, *invite.ExpiresAt)
		assert.Nil(t, invite.ProcessedAt, "expected nil ProcessedAt for a pending invite")
	})

	t.Run("processed invite carries processed time and nil expiry", func(t *testing.T) {
		processedAt := time.Now().UTC().Truncate(time.Second)
		adminInvite := admin.Invite{
			Id:          uuid.New(),
			Email:       "teammate@example.com",
			Status:      "processed",
			ProcessedAt: &processedAt,
			ExpiresAt:   nil,
		}

		invite := toInvite(adminInvite)

		require.NotNil(t, invite)
		assert.Equal(t, InviteStatusProcessed, invite.Status)
		assert.Nil(t, invite.ExpiresAt, "expected nil ExpiresAt when the invite does not expire")
		require.NotNil(t, invite.ProcessedAt)
		assert.Equal(t, processedAt, *invite.ProcessedAt)
	})
}

func TestToInviteListUnit(t *testing.T) {
	t.Run("populated data with pagination", func(t *testing.T) {
		next := "next-token"
		adminList := admin.InviteList{
			Data: []admin.Invite{
				{Id: uuid.New(), Email: "a@example.com", Status: "pending"},
				{Id: uuid.New(), Email: "b@example.com", Status: "expired"},
			},
			Pagination: &struct {
				Next *string `json:"next,omitempty"`
			}{Next: &next},
		}

		list := toInviteList(adminList)

		require.NotNil(t, list)
		require.Len(t, list.Data, 2)
		assert.Equal(t, "a@example.com", list.Data[0].Email)
		assert.Equal(t, InviteStatusExpired, list.Data[1].Status)
		require.NotNil(t, list.Pagination)
		assert.Equal(t, next, list.Pagination.Next)
	})

	t.Run("nil pagination envelope yields nil pagination", func(t *testing.T) {
		adminList := admin.InviteList{
			Data:       []admin.Invite{},
			Pagination: nil,
		}

		list := toInviteList(adminList)

		require.NotNil(t, list)
		assert.NotNil(t, list.Data, "expected non-nil (empty) data slice")
		assert.Len(t, list.Data, 0)
		assert.Nil(t, list.Pagination, "expected nil pagination when envelope is absent")
	})
}

func TestInviteCreateNilParamsUnit(t *testing.T) {
	client := &DefaultInviteClient{}
	invite, err := client.Create(context.Background(), nil)
	assert.Nil(t, invite)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CreateInviteParams")
}

func TestInviteInvalidUUIDUnit(t *testing.T) {
	client := &DefaultInviteClient{}

	t.Run("Describe", func(t *testing.T) {
		invite, err := client.Describe(context.Background(), "not-a-uuid")
		assert.Nil(t, invite)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid inviteId")
	})

	t.Run("Resend", func(t *testing.T) {
		invite, err := client.Resend(context.Background(), "not-a-uuid")
		assert.Nil(t, invite)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid inviteId")
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(context.Background(), "not-a-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid inviteId")
	})
}
