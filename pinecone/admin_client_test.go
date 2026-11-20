package pinecone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/v8/internal/gen/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		getAuthTokenFunc = func(ctx context.Context, id, secret string, opts ...admin.ClientOption) (*authTokenResponse, error) {
			assert.Equal(t, clientId, id)
			assert.Equal(t, clientSecret, secret)
			return &authTokenResponse{AccessToken: "mock-token", ExpiresIn: 1800}, nil
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
			Pagination: &admin.PaginationResponse{Next: &next},
		}

		list := toRoleBindingList(adminList)

		require.NotNil(t, list)
		require.Len(t, list.Data, 2)
		assert.Equal(t, PrincipalTypeUser, list.Data[0].PrincipalType)
		assert.Equal(t, PrincipalTypeAPIKey, list.Data[1].PrincipalType)
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
			Data:       []admin.RoleBinding{},
			Pagination: &admin.PaginationResponse{Next: nil},
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
			Pagination: &admin.PaginationResponse{Next: &next},
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
			Pagination: &admin.PaginationResponse{Next: &next},
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

func TestToUserUnit(t *testing.T) {
	t.Run("user with display name", func(t *testing.T) {
		id := uuid.New()
		name := "Ada Lovelace"
		adminUser := admin.User{
			Id:    id,
			Email: "ada@example.com",
			Name:  &name,
		}

		user := toUser(adminUser)

		require.NotNil(t, user)
		assert.Equal(t, id.String(), user.Id, "expected UUID to be stringified")
		assert.Equal(t, "ada@example.com", user.Email, "expected email to be stringified")
		require.NotNil(t, user.Name)
		assert.Equal(t, name, *user.Name)
	})

	t.Run("user with SSO and personal accounts", func(t *testing.T) {
		var adminUser admin.User
		require.NoError(t, json.Unmarshal([]byte(`{
			"id": "`+uuid.New().String()+`",
			"email": "ada@example.com",
			"accounts": [{"type": "sso", "name_id": "ada@corp.example"}, {"type": "personal"}]
		}`), &adminUser))

		user := toUser(adminUser)

		require.NotNil(t, user)
		assert.Equal(t, &[]UserAccount{
			{Type: "sso", NameId: ptr("ada@corp.example")},
			{Type: "personal"},
		}, user.Accounts)
	})

	t.Run("user in an organization without SSO", func(t *testing.T) {
		var adminUser admin.User
		require.NoError(t, json.Unmarshal([]byte(`{"id": "`+uuid.New().String()+`", "email": "ada@example.com"}`), &adminUser))

		user := toUser(adminUser)

		require.NotNil(t, user)
		assert.Nil(t, user.Accounts)
	})

	t.Run("user without display name", func(t *testing.T) {
		adminUser := admin.User{
			Id:    uuid.New(),
			Email: "noname@example.com",
			Name:  nil,
		}

		user := toUser(adminUser)

		require.NotNil(t, user)
		assert.Nil(t, user.Name, "expected nil Name when the user has not set one")
	})
}

func TestToUserListUnit(t *testing.T) {
	t.Run("populated data with pagination", func(t *testing.T) {
		next := "next-token"
		name := "Ada Lovelace"
		adminList := admin.UserList{
			Data: []admin.User{
				{Id: uuid.New(), Email: "ada@example.com", Name: &name},
				{Id: uuid.New(), Email: "noname@example.com"},
			},
			Pagination: &admin.PaginationResponse{Next: &next},
		}

		list := toUserList(adminList)

		require.NotNil(t, list)
		require.Len(t, list.Data, 2)
		require.NotNil(t, list.Data[0].Name)
		assert.Equal(t, name, *list.Data[0].Name)
		assert.Nil(t, list.Data[1].Name)
		require.NotNil(t, list.Pagination)
		assert.Equal(t, next, list.Pagination.Next)
	})

	t.Run("nil pagination envelope yields nil pagination", func(t *testing.T) {
		adminList := admin.UserList{
			Data:       []admin.User{},
			Pagination: nil,
		}

		list := toUserList(adminList)

		require.NotNil(t, list)
		assert.NotNil(t, list.Data, "expected non-nil (empty) data slice")
		assert.Len(t, list.Data, 0)
		assert.Nil(t, list.Pagination, "expected nil pagination when envelope is absent")
	})
}

func TestUserInvalidUUIDUnit(t *testing.T) {
	client := &DefaultUserClient{}

	t.Run("Describe", func(t *testing.T) {
		user, err := client.Describe(context.Background(), "not-a-uuid")
		assert.Nil(t, user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid userId")
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(context.Background(), "not-a-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid userId")
	})
}

func TestOrganizationDeleteStatusCodesUnit(t *testing.T) {
	t.Setenv("PINECONE_CLIENT_ID", "")
	t.Setenv("PINECONE_CLIENT_SECRET", "")
	t.Setenv("PINECONE_ACCESS_TOKEN", "")

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "202 accepted", status: http.StatusAccepted},
		{name: "200 ok", status: http.StatusOK},
		{name: "412 precondition failed", status: http.StatusPreconditionFailed, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			client, err := NewAdminClientWithContext(context.Background(), NewAdminClientParams{
				AccessToken: "test-token",
				Host:        server.URL,
			})
			require.NoError(t, err)

			err = client.Organization.Delete(context.Background(), "org-id")
			assert.Equal(t, http.MethodDelete, gotMethod)
			assert.Equal(t, "/admin/organizations/org-id", gotPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

const (
	testAPIKeySecret         = "pckey_secret_value_do_not_log"
	testServiceAccountSecret = "sa_client_secret_do_not_log"
	testProjectId            = "4ba3ad9f-2bbd-4c2b-a0c2-d2f3a7c2c9a1"
	testResourceId           = "0f8e7d6c-5b4a-4392-8170-6e5d4c3b2a19"
)

var (
	testAPIKeyWithSecretBody = `{"key":{"id":"` + testResourceId + `","name":"k","project_id":"` + testProjectId + `","roles":["ProjectEditor"]},"value":"` + testAPIKeySecret + `"}`
	testSAWithSecretBody     = `{"client_secret":"` + testServiceAccountSecret + `","service_account":{"client_id":"cid","created_at":"2026-10-01T00:00:00Z","id":"` + testResourceId + `","name":"sa","updated_at":"2026-10-01T00:00:00Z"}}`
)

var testInviteRoleBindings = []RoleBindingInput{{ResourceType: ResourceTypeOrganization, Role: "OrgMember"}}

func newAdminStatusTestClient(t *testing.T, status int, body string) (*AdminClient, *int32) {
	t.Helper()
	t.Setenv("PINECONE_CLIENT_ID", "")
	t.Setenv("PINECONE_CLIENT_SECRET", "")
	t.Setenv("PINECONE_ACCESS_TOKEN", "")
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	client, err := NewAdminClientWithContext(context.Background(), NewAdminClientParams{
		AccessToken: "test-token",
		Host:        server.URL,
	})
	require.NoError(t, err)
	return client, &calls
}

func TestSecretBearingAdminCallsAcceptAnySuccessStatus(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(fmt.Sprintf("APIKey.Create %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, testAPIKeyWithSecretBody)
			key, err := client.APIKey.Create(context.Background(), testProjectId, &CreateAPIKeyParams{Name: "k"})
			require.NoError(t, err)
			assert.Equal(t, testAPIKeySecret, key.Value)
			assert.Equal(t, testResourceId, key.Key.Id)
		})
		t.Run(fmt.Sprintf("ServiceAccount.Create %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, testSAWithSecretBody)
			sa, err := client.ServiceAccount.Create(context.Background(), &CreateServiceAccountParams{Name: "sa"})
			require.NoError(t, err)
			assert.Equal(t, testServiceAccountSecret, sa.ClientSecret)
			assert.Equal(t, testResourceId, sa.ServiceAccount.Id)
		})
		t.Run(fmt.Sprintf("ServiceAccount.RotateSecret %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, testSAWithSecretBody)
			sa, err := client.ServiceAccount.RotateSecret(context.Background(), testResourceId)
			require.NoError(t, err)
			assert.Equal(t, testServiceAccountSecret, sa.ClientSecret)
		})
	}
}

func TestSecretBearingAdminCallsNeverPutSecretInError(t *testing.T) {
	malformedKey := `{"key":{"id":"not-a-uuid","name":"k","project_id":"` + testProjectId + `","roles":[]},"value":"` + testAPIKeySecret + `"}`
	malformedSA := `{"client_secret":"` + testServiceAccountSecret + `","service_account":{"id":"not-a-uuid","created_at":"yesterday"}}`
	secrets := []string{testAPIKeySecret, testServiceAccountSecret}

	calls := map[string]func(*AdminClient) error{
		"APIKey.Create": func(c *AdminClient) error {
			_, err := c.APIKey.Create(context.Background(), testProjectId, &CreateAPIKeyParams{Name: "k"})
			return err
		},
		"ServiceAccount.Create": func(c *AdminClient) error {
			_, err := c.ServiceAccount.Create(context.Background(), &CreateServiceAccountParams{Name: "sa"})
			return err
		},
		"ServiceAccount.RotateSecret": func(c *AdminClient) error {
			_, err := c.ServiceAccount.RotateSecret(context.Background(), testResourceId)
			return err
		},
	}
	bodies := []string{
		testAPIKeyWithSecretBody,
		testSAWithSecretBody,
		malformedKey,
		malformedSA,
		testAPIKeyWithSecretBody[:len(testAPIKeyWithSecretBody)-2],
		testSAWithSecretBody[:len(testSAWithSecretBody)-2],
	}
	for name, call := range calls {
		for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNonAuthoritativeInfo} {
			for i, body := range bodies {
				t.Run(fmt.Sprintf("%s/%d/body%d", name, status, i), func(t *testing.T) {
					client, _ := newAdminStatusTestClient(t, status, body)
					err := call(client)
					if err == nil {
						return
					}
					var pcErr *PineconeError
					assert.False(t, errors.As(err, &pcErr), "a 2xx response must not become a PineconeError: %v", err)
					for _, secret := range secrets {
						assert.NotContains(t, err.Error(), secret)
					}
				})
			}
		}
	}
}

func TestAdminCallsAcceptPlausibleSuccessStatuses(t *testing.T) {
	projectBody := `{"id":"` + testProjectId + `","name":"p","max_pods":0,"force_encryption_with_cmek":false,"organization_id":"org"}`
	inviteBody := `{"id":"` + testResourceId + `","email":"a@example.com","status":"pending","created_at":"2026-10-01T00:00:00Z","expires_at":null,"processed_at":null}`
	roleBindingBody := `{"id":"` + testResourceId + `","principal_id":"p","principal_type":"service_account","resource_id":"org","resource_type":"organization","role":"OrgMember","created_at":"2026-10-01T00:00:00Z"}`

	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(fmt.Sprintf("Project.Create %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, projectBody)
			_, err := client.Project.Create(context.Background(), &CreateProjectParams{Name: "p"})
			require.NoError(t, err)
		})
		t.Run(fmt.Sprintf("Invite.Create %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, inviteBody)
			_, err := client.Invite.Create(context.Background(), &CreateInviteParams{Email: "a@example.com", RoleBindings: testInviteRoleBindings})
			require.NoError(t, err)
		})
		t.Run(fmt.Sprintf("RoleBinding.Create %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, roleBindingBody)
			_, err := client.RoleBinding.Create(context.Background(), &CreateRoleBindingParams{
				PrincipalId: "p", PrincipalType: PrincipalTypeServiceAccount, ResourceType: ResourceTypeOrganization, Role: "OrgMember",
			})
			require.NoError(t, err)
		})
	}
	for _, status := range []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent} {
		t.Run(fmt.Sprintf("deletes %d", status), func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, status, "")
			ctx := context.Background()
			require.NoError(t, client.Project.Delete(ctx, testProjectId))
			require.NoError(t, client.APIKey.Delete(ctx, testResourceId))
			require.NoError(t, client.ServiceAccount.Delete(ctx, testResourceId))
			require.NoError(t, client.RoleBinding.Delete(ctx, testResourceId))
			require.NoError(t, client.Invite.Delete(ctx, testResourceId))
			require.NoError(t, client.User.Delete(ctx, testResourceId))
			require.NoError(t, client.Organization.Delete(ctx, "org-id"))
		})
	}
}

func TestOrganizationMethodsRejectEmptyId(t *testing.T) {
	client, calls := newAdminStatusTestClient(t, http.StatusOK, `{"data":[]}`)
	ctx := context.Background()

	org, err := client.Organization.Describe(ctx, "")
	assert.EqualError(t, err, "you must provide an organizationId to describe an organization")
	assert.Nil(t, org)

	name := "n"
	org, err = client.Organization.Update(ctx, "", &UpdateOrganizationParams{Name: &name})
	assert.EqualError(t, err, "you must provide an organizationId to update an organization")
	assert.Nil(t, org)

	assert.EqualError(t, client.Organization.Delete(ctx, ""), "you must provide an organizationId to delete an organization")
	assert.Equal(t, int32(0), atomic.LoadInt32(calls), "no request should be sent")
}

func TestInviteCreateRequiresRoleBindings(t *testing.T) {
	client, calls := newAdminStatusTestClient(t, http.StatusCreated, `{}`)
	ctx := context.Background()

	for name, bindings := range map[string][]RoleBindingInput{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			invite, err := client.Invite.Create(ctx, &CreateInviteParams{Email: "a@example.com", RoleBindings: bindings})
			assert.EqualError(t, err, "you must provide at least one role binding in RoleBindings to create an invite")
			assert.Nil(t, invite)
		})
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(calls), "no request should be sent")
}

func TestInviteCreateSendsRoleBindings(t *testing.T) {
	t.Setenv("PINECONE_CLIENT_ID", "")
	t.Setenv("PINECONE_CLIENT_SECRET", "")
	t.Setenv("PINECONE_ACCESS_TOKEN", "")
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + testResourceId + `","email":"a@example.com","status":"pending","created_at":"2026-10-01T00:00:00Z","expires_at":null,"processed_at":null}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewAdminClientWithContext(context.Background(), NewAdminClientParams{AccessToken: "test-token", Host: server.URL})
	require.NoError(t, err)

	_, err = client.Invite.Create(context.Background(), &CreateInviteParams{
		Email:        "a@example.com",
		RoleBindings: []RoleBindingInput{{ResourceType: ResourceTypeOrganization, Role: "OrgMember"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "a@example.com", got["email"])
	assert.Equal(t, []any{map[string]any{"resource_type": "organization", "role": "OrgMember"}}, got["role_bindings"])
}

func TestAdminDecodeErrorsNameTheOperation(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(*AdminClient) error{
		"create project": func(c *AdminClient) error {
			_, err := c.Project.Create(ctx, &CreateProjectParams{Name: "p"})
			return err
		},
		"update project": func(c *AdminClient) error {
			_, err := c.Project.Update(ctx, testProjectId, &UpdateProjectParams{})
			return err
		},
		"list projects": func(c *AdminClient) error {
			_, err := c.Project.List(ctx)
			return err
		},
		"describe project": func(c *AdminClient) error {
			_, err := c.Project.Describe(ctx, testProjectId)
			return err
		},
		"list organizations": func(c *AdminClient) error {
			_, err := c.Organization.List(ctx)
			return err
		},
		"describe organization": func(c *AdminClient) error {
			_, err := c.Organization.Describe(ctx, "org")
			return err
		},
		"update organization": func(c *AdminClient) error {
			_, err := c.Organization.Update(ctx, "org", &UpdateOrganizationParams{})
			return err
		},
		"create api key": func(c *AdminClient) error {
			_, err := c.APIKey.Create(ctx, testProjectId, &CreateAPIKeyParams{Name: "k"})
			return err
		},
		"update api key": func(c *AdminClient) error {
			_, err := c.APIKey.Update(ctx, testResourceId, &UpdateAPIKeyParams{})
			return err
		},
		"list api keys": func(c *AdminClient) error {
			_, err := c.APIKey.List(ctx, testProjectId)
			return err
		},
		"describe api key": func(c *AdminClient) error {
			_, err := c.APIKey.Describe(ctx, testResourceId)
			return err
		},
		"create role binding": func(c *AdminClient) error {
			_, err := c.RoleBinding.Create(ctx, &CreateRoleBindingParams{})
			return err
		},
		"list role bindings": func(c *AdminClient) error {
			_, err := c.RoleBinding.List(ctx, nil)
			return err
		},
		"describe role binding": func(c *AdminClient) error {
			_, err := c.RoleBinding.Describe(ctx, testResourceId)
			return err
		},
		"create service account": func(c *AdminClient) error {
			_, err := c.ServiceAccount.Create(ctx, &CreateServiceAccountParams{Name: "sa"})
			return err
		},
		"update service account": func(c *AdminClient) error {
			_, err := c.ServiceAccount.Update(ctx, testResourceId, &UpdateServiceAccountParams{})
			return err
		},
		"list service accounts": func(c *AdminClient) error {
			_, err := c.ServiceAccount.List(ctx, nil)
			return err
		},
		"describe service account": func(c *AdminClient) error {
			_, err := c.ServiceAccount.Describe(ctx, testResourceId)
			return err
		},
		"rotate service account secret": func(c *AdminClient) error {
			_, err := c.ServiceAccount.RotateSecret(ctx, testResourceId)
			return err
		},
		"create invite": func(c *AdminClient) error {
			_, err := c.Invite.Create(ctx, &CreateInviteParams{Email: "a@example.com", RoleBindings: testInviteRoleBindings})
			return err
		},
		"list invites": func(c *AdminClient) error {
			_, err := c.Invite.List(ctx, nil)
			return err
		},
		"describe invite": func(c *AdminClient) error {
			_, err := c.Invite.Describe(ctx, testResourceId)
			return err
		},
		"resend invite": func(c *AdminClient) error {
			_, err := c.Invite.Resend(ctx, testResourceId)
			return err
		},
		"list users": func(c *AdminClient) error {
			_, err := c.User.List(ctx, nil)
			return err
		},
		"describe user": func(c *AdminClient) error {
			_, err := c.User.Describe(ctx, testResourceId)
			return err
		},
	}
	for op, call := range calls {
		t.Run(op, func(t *testing.T) {
			client, _ := newAdminStatusTestClient(t, http.StatusOK, `{"id":`)
			err := call(client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to decode "+op+" response: ")
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		})
	}
}
