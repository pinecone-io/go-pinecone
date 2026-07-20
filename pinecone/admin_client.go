package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/pinecone-io/go-pinecone/v5/internal/gen"
	"github.com/pinecone-io/go-pinecone/v5/internal/gen/admin"
	"github.com/pinecone-io/go-pinecone/v5/internal/provider"
	"github.com/pinecone-io/go-pinecone/v5/internal/useragent"
)

// [AdminClient] provides access to Pinecone's administrative APIs, which supports
// managing projects, organizations, API keys, role bindings, service accounts,
// invites, and users. It is constructed using [NewAdminClient] or
// [NewAdminClientWithContext].
type AdminClient struct {
	// Project provides methods for creating, updating, listing, describing,
	// and deleting projects.
	Project ProjectClient

	// Organization provides methods for listing, describing, updating,
	// and deleting organizations.
	Organization OrganizationClient

	// APIKey provides methods for creating, updating, listing, describing,
	// and deleting API keys within a project.
	APIKey APIKeyClient

	// RoleBinding provides methods for creating, listing, describing, and
	// deleting role bindings, which grant roles to principals (users, service
	// accounts, API keys, and invites) at an organization or project scope.
	RoleBinding RoleBindingClient

	// ServiceAccount provides methods for creating, updating, listing, describing,
	// deleting, and rotating the secret of service accounts within an organization.
	ServiceAccount ServiceAccountClient

	// Invite provides methods for creating, listing, describing, resending, and
	// deleting invitations to join the organization.
	Invite InviteClient

	// User provides methods for listing, describing, and deleting users within
	// the organization.
	User UserClient
}

// [ProjectClient] provides an interface for managing Pinecone projects.
type ProjectClient interface {
	// Create a new project.
	Create(ctx context.Context, in *CreateProjectParams) (*Project, error)

	// Update an existing project by ID.
	Update(ctx context.Context, projectId string, in *UpdateProjectParams) (*Project, error)

	// List all projects available to the authenticated service account.
	List(ctx context.Context) ([]*Project, error)

	// Describe an existing project by ID.
	Describe(ctx context.Context, projectId string) (*Project, error)

	// Delete a project by ID.
	Delete(ctx context.Context, projectId string) error
}

// [OrganizationClient] provides an interface for managing organizations.
type OrganizationClient interface {
	// List all organizations available to the authenticated service account.
	List(ctx context.Context) ([]*Organization, error)

	// Describe an organization by ID.
	Describe(ctx context.Context, organizationId string) (*Organization, error)

	// Update an existing organization by ID.
	Update(ctx context.Context, organizationId string, in *UpdateOrganizationParams) (*Organization, error)

	// Delete an organization by ID. All projects within the organization must be deleted first.
	Delete(ctx context.Context, organizationId string) error
}

// [APIKeyClient] provides an interface for managing API keys within a project.
type APIKeyClient interface {
	// Create a new API key.
	Create(ctx context.Context, projectId string, in *CreateAPIKeyParams) (*APIKeyWithSecret, error)

	// Update an existing API key by ID.
	Update(ctx context.Context, apiKeyId string, in *UpdateAPIKeyParams) (*APIKey, error)

	// List all API keys within a project by project ID.
	List(ctx context.Context, projectId string) ([]*APIKey, error)

	// Describe an API key by ID.
	Describe(ctx context.Context, apiKeyId string) (*APIKey, error)

	// Delete an API key by ID.
	Delete(ctx context.Context, apiKeyId string) error
}

// [RoleBindingClient] provides an interface for managing role bindings, which grant
// roles to principals (users, service accounts, API keys, and invites) at an
// organization or project scope.
type RoleBindingClient interface {
	// Create a new role binding.
	Create(ctx context.Context, in *CreateRoleBindingParams) (*RoleBinding, error)

	// List role bindings, optionally filtered by principal, resource, or role.
	List(ctx context.Context, in *ListRoleBindingsParams) (*RoleBindingList, error)

	// Describe a role binding by ID.
	Describe(ctx context.Context, roleBindingId string) (*RoleBinding, error)

	// Delete a role binding by ID.
	Delete(ctx context.Context, roleBindingId string) error
}

// [ServiceAccountClient] provides an interface for managing service accounts within
// an organization.
type ServiceAccountClient interface {
	// Create a new service account. The returned [ServiceAccountWithSecret] contains
	// the OAuth client secret, which is returned only once.
	Create(ctx context.Context, in *CreateServiceAccountParams) (*ServiceAccountWithSecret, error)

	// Update an existing service account by ID.
	Update(ctx context.Context, serviceAccountId string, in *UpdateServiceAccountParams) (*ServiceAccount, error)

	// List all service accounts within the organization.
	List(ctx context.Context, in *ListServiceAccountsParams) (*ServiceAccountList, error)

	// Describe a service account by ID.
	Describe(ctx context.Context, serviceAccountId string) (*ServiceAccount, error)

	// RotateSecret issues a new OAuth client secret for a service account by ID. The
	// returned [ServiceAccountWithSecret] contains the new secret, which is returned
	// only once; the previous secret is invalidated.
	RotateSecret(ctx context.Context, serviceAccountId string) (*ServiceAccountWithSecret, error)

	// Delete a service account by ID.
	Delete(ctx context.Context, serviceAccountId string) error
}

// [InviteClient] provides an interface for managing invitations to join the organization.
type InviteClient interface {
	// Create and send a new invite. The role bindings must include at least one
	// organization-scoped binding that grants organization membership.
	Create(ctx context.Context, in *CreateInviteParams) (*Invite, error)

	// List invites in the organization.
	List(ctx context.Context, in *ListInvitesParams) (*InviteList, error)

	// Describe an invite by ID.
	Describe(ctx context.Context, inviteId string) (*Invite, error)

	// Resend an existing invite by ID, extending its expiration.
	Resend(ctx context.Context, inviteId string) (*Invite, error)

	// Delete an invite by ID.
	Delete(ctx context.Context, inviteId string) error
}

// [UserClient] provides an interface for managing users within the organization.
type UserClient interface {
	// List users in the organization, optionally filtered by email.
	List(ctx context.Context, in *ListUsersParams) (*UserList, error)

	// Describe a user by ID.
	Describe(ctx context.Context, userId string) (*User, error)

	// Delete a user by ID, removing them from the organization.
	Delete(ctx context.Context, userId string) error
}

// [DefaultProjectClient] is the default implementation of [ProjectClient].
type DefaultProjectClient struct {
	restClient *admin.Client
}

// [DefaultOrganizationClient] is the default implementation of [OrganizationClient].
type DefaultOrganizationClient struct {
	restClient *admin.Client
}

// [DefaultApiKeyClient] is the default implementation of [APIKeyClient].
type DefaultApiKeyClient struct {
	restClient *admin.Client
}

// [DefaultRoleBindingClient] is the default implementation of [RoleBindingClient].
type DefaultRoleBindingClient struct {
	restClient *admin.Client
}

// [DefaultServiceAccountClient] is the default implementation of [ServiceAccountClient].
type DefaultServiceAccountClient struct {
	restClient *admin.Client
}

// [DefaultInviteClient] is the default implementation of [InviteClient].
type DefaultInviteClient struct {
	restClient *admin.Client
}

// [DefaultUserClient] is the default implementation of [UserClient].
type DefaultUserClient struct {
	restClient *admin.Client
}

// [NewAdminClientParams] contains parameters used to configure the [AdminClient].
// You must provide either a client ID and secret, or an access token, either directly or via environment
// variables (PINECONE_CLIENT_ID, PINECONE_CLIENT_SECRET, PINECONE_ACCESS_TOKEN).
type NewAdminClientParams struct {
	// The OAuth client ID used for authentication.
	ClientId string

	// The OAuth client secret used for authentication.
	ClientSecret string

	// The OAuth access token used for authentication.
	AccessToken string

	// The host URL of the Pinecone API. If not provided, the default value is "https://api.pinecone.io".
	Host string

	// (Optional) Additional headers to include in the request.
	Headers *map[string]string

	// (Optional) The HTTP client to use for the request.
	RestClient *http.Client

	// (Optional) The source tag to include in the request.
	SourceTag *string
}

// [NewAdminClient] returns a new [AdminClient] using the given parameters,
// using context.Background as the default context. It validates the client ID and secret
// from the input or environment, authenticates, and constructs an authorized [AdminClient].
func NewAdminClient(in NewAdminClientParams) (*AdminClient, error) {
	return NewAdminClientWithContext(context.Background(), in)
}

// [NewAdminClientWithContext] returns a new [AdminClient] using the provided
// context and parameters. This function allows for finer control over timeout, and
// cancellation of the authentication request. It validates the client ID and secret
// from the input or environment, authenticates, and constructs an authorized [AdminClient].
func NewAdminClientWithContext(ctx context.Context, in NewAdminClientParams) (*AdminClient, error) {
	var authHeader string
	clientOptions := buildAdminClientOptions(in)

	accessToken := valueOrFallback(in.AccessToken, os.Getenv("PINECONE_ACCESS_TOKEN"))
	if accessToken != "" {
		// Use access token directly if provided
		authHeader = fmt.Sprintf("Bearer %s", accessToken)
	} else {
		// Fall back to client ID and secret if access token is not provided
		clientId := valueOrFallback(in.ClientId, os.Getenv("PINECONE_CLIENT_ID"))
		clientSecret := valueOrFallback(in.ClientSecret, os.Getenv("PINECONE_CLIENT_SECRET"))
		if clientId == "" {
			return nil, fmt.Errorf("no ClientId provided, please pass an ClientId for authorization through NewAdminClientParams or set the PINECONE_CLIENT_ID environment variable")
		}
		if clientSecret == "" {
			return nil, fmt.Errorf("no ClientSecret provided, please pass an ClientSecret for authorization through NewAdminClientParams or set the PINECONE_CLIENT_SECRET environment variable")
		}

		authToken, err := getAuthTokenFunc(ctx, clientId, clientSecret, clientOptions...)
		if err != nil {
			return nil, err
		}
		authHeader = fmt.Sprintf("Bearer %s", authToken)
	}

	hostOverride := valueOrFallback(in.Host, os.Getenv("PINECONE_CONTROLLER_HOST"))
	if hostOverride != "" {
		var err error
		hostOverride, err = ensureURLScheme(hostOverride)
		if err != nil {
			return nil, err
		}
	}

	authProvider := provider.NewHeaderProvider("Authorization", authHeader)
	clientOptions = append(clientOptions, admin.WithRequestEditorFn(authProvider.Intercept))

	adminClient, err := newAdminClient(valueOrFallback(hostOverride, "https://api.pinecone.io"), clientOptions...)
	if err != nil {
		return nil, err
	}

	return &AdminClient{
		Project: &DefaultProjectClient{
			restClient: adminClient,
		},
		Organization: &DefaultOrganizationClient{
			restClient: adminClient,
		},
		APIKey: &DefaultApiKeyClient{
			restClient: adminClient,
		},
		RoleBinding: &DefaultRoleBindingClient{
			restClient: adminClient,
		},
		ServiceAccount: &DefaultServiceAccountClient{
			restClient: adminClient,
		},
		Invite: &DefaultInviteClient{
			restClient: adminClient,
		},
		User: &DefaultUserClient{
			restClient: adminClient,
		},
	}, nil
}

// testing abstractions
var (
	getAuthTokenFunc = getAuthToken
	newAdminClient   = admin.NewClient
)

// [CreateProjectParams] contains parameters for creating a new project.
type CreateProjectParams struct {
	// The name of the new project.
	Name string `json:"name"`

	// (Optional) Whether to force encryption with a customer-managed encryption key (CMEK). Default is `false`.
	// Once enabled, CMEK encryption cannot be disabled.
	ForceEncryptionWithCmek *bool `json:"force_encryption_with_cmek,omitempty"`

	// (Optional) The maximum number of Pods that can be created in the project. Default is `0` (serverless only).
	MaxPods *int `json:"max_pods,omitempty"`
}

// Creates a new project.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [CreateProjectParams] containing the new project's configuration.
//
// Returns a pointer to a [Project] or an error.
//
// Example:
//
//	ctx := context.Background()
//	project, err := adminClient.Project.Create(ctx, &pinecone.CreateProjectParams{
//		Name: "example-project",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (p *DefaultProjectClient) Create(ctx context.Context, in *CreateProjectParams) (*Project, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateProjectParams) cannot be nil")
	}

	request := admin.CreateProjectRequest{
		ForceEncryptionWithCmek: in.ForceEncryptionWithCmek,
		MaxPods:                 in.MaxPods,
		Name:                    in.Name,
	}

	res, err := p.restClient.CreateProject(ctx, &admin.CreateProjectParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to create project: ")
	}

	var adminProject admin.Project
	err = json.NewDecoder(res.Body).Decode(&adminProject)
	if err != nil {
		return nil, err
	}

	return toProject(adminProject), nil
}

// [UpdateProjectParams] contains parameters for updating an existing project.
type UpdateProjectParams struct {
	// (Optional) The name of the new project.
	Name *string `json:"name,omitempty"`

	// (Optional) Whether to force encryption with a customer-managed encryption key (CMEK).
	// Once enabled, CMEK encryption cannot be disabled.
	ForceEncryptionWithCmek *bool `json:"force_encryption_with_cmek,omitempty"`

	// (Optional) The maximum number of Pods that can be created in the project.
	MaxPods *int `json:"max_pods,omitempty"`
}

// Updates an existing project by ID.
//
// Parameters:
//   - ctx: The request context.
//   - projectId: The ID of the project to update.
//   - in: A pointer to [UpdateProjectParams] containing the updated project configuration.
//
// Returns the updated [Project] or an error.
//
// Example:
//
//	project, err := adminClient.Project.Update(ctx, "project-id", &pinecone.UpdateProjectParams{
//		Name: "renamed-project",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (p *DefaultProjectClient) Update(ctx context.Context, projectId string, in *UpdateProjectParams) (*Project, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateProjectParams) cannot be nil")
	}

	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}

	request := admin.UpdateProjectRequest{
		Name:                    in.Name,
		MaxPods:                 in.MaxPods,
		ForceEncryptionWithCmek: in.ForceEncryptionWithCmek,
	}

	res, err := p.restClient.UpdateProject(ctx, projectIdUUID, &admin.UpdateProjectParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to update project: ")
	}

	var adminProject admin.Project
	err = json.NewDecoder(res.Body).Decode(&adminProject)
	if err != nil {
		return nil, err
	}

	return toProject(adminProject), nil
}

// Lists all projects available to the authenticated service account.
//
// Parameters:
//   - ctx: The request context.
//
// Returns a slice of [Project] objects or an error.
//
// Example:
//
//	projects, err := adminClient.Project.List(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
func (p *DefaultProjectClient) List(ctx context.Context) ([]*Project, error) {
	res, err := p.restClient.ListProjects(ctx, &admin.ListProjectsParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list projects: ")
	}

	var listResp struct {
		Data *[]admin.Project `json:"data,omitempty"`
	}
	err = json.NewDecoder(res.Body).Decode(&listResp)
	if err != nil {
		return nil, err
	}

	var projects []*Project
	if listResp.Data != nil {
		projects = make([]*Project, len(*listResp.Data))
		for i, project := range *listResp.Data {
			projects[i] = toProject(project)
		}
	} else {
		projects = make([]*Project, 0)
	}

	return projects, nil
}

// Describes an existing project by ID.
//
// Parameters:
//   - ctx: The request context.
//   - projectId: The ID of the project to describe.
//
// Returns a pointer to a [Project] or an error.
//
// Example:
//
//	project, err := adminClient.Project.Describe(ctx, "project-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (p *DefaultProjectClient) Describe(ctx context.Context, projectId string) (*Project, error) {
	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := p.restClient.FetchProject(ctx, projectIdUUID, &admin.FetchProjectParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe project: ")
	}

	var adminProject admin.Project
	err = json.NewDecoder(res.Body).Decode(&adminProject)
	if err != nil {
		return nil, err
	}

	return toProject(adminProject), nil
}

// Deletes a project by ID.
//
// Parameters:
//   - ctx: The request context.
//   - projectId: The ID of the project to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.Project.Delete(ctx, "project-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (p *DefaultProjectClient) Delete(ctx context.Context, projectId string) error {
	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := p.restClient.DeleteProject(ctx, projectIdUUID, &admin.DeleteProjectParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete project: ")
	}

	return nil
}

// Lists all organizations available to the authenticated service account.
//
// Parameters:
//   - ctx: The request context.
//
// Returns a slice of [Organization] objects or an error.
//
// Example:
//
//	orgs, err := adminClient.Organization.List(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
func (o *DefaultOrganizationClient) List(ctx context.Context) ([]*Organization, error) {
	res, err := o.restClient.ListOrganizations(ctx, &admin.ListOrganizationsParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list organizations: ")
	}

	var listResp struct {
		Data *[]admin.Organization `json:"data,omitempty"`
	}
	err = json.NewDecoder(res.Body).Decode(&listResp)
	if err != nil {
		return nil, err
	}

	var organizations []*Organization
	if listResp.Data != nil {
		organizations = make([]*Organization, len(*listResp.Data))
		for i, org := range *listResp.Data {
			organizations[i] = toOrganization(org)
		}
	} else {
		organizations = make([]*Organization, 0)
	}

	return organizations, nil
}

// Describes an organization by ID.
//
// Parameters:
//   - ctx: The request context.
//   - organizationId: The ID of the organization to describe.
//
// Returns a pointer to an [Organization] or an error.
//
// Example:
//
//	org, err := adminClient.Organization.Describe(ctx, "organization-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (o *DefaultOrganizationClient) Describe(ctx context.Context, organizationId string) (*Organization, error) {
	res, err := o.restClient.FetchOrganization(ctx, organizationId, &admin.FetchOrganizationParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe organization: ")
	}

	var adminOrganization admin.Organization
	err = json.NewDecoder(res.Body).Decode(&adminOrganization)
	if err != nil {
		return nil, err
	}

	return toOrganization(adminOrganization), nil
}

// [UpdateOrganizationParams] contains parameters for updating an existing organization.
type UpdateOrganizationParams struct {
	// (Optional) The new name of the organization.
	Name *string `json:"name"`
}

// Updates an existing organization by ID.
//
// Parameters:
//   - ctx: The request context.
//   - organizationId: The ID of the organization to update.
//   - in: A pointer to [UpdateOrganizationParams] containing updated fields.
//
// Returns the updated [Organization] or an error.
//
// Example:
//
//	org, err := adminClient.Organization.Update(ctx, "organization-id", &pinecone.UpdateOrganizationParams{
//		Name: "Renamed Org",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (o *DefaultOrganizationClient) Update(ctx context.Context, organizationId string, in *UpdateOrganizationParams) (*Organization, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateOrganizationParams) cannot be nil")
	}

	request := admin.UpdateOrganizationRequest{
		Name: in.Name,
	}

	res, err := o.restClient.UpdateOrganization(ctx, organizationId, &admin.UpdateOrganizationParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to update organization: ")
	}

	var adminOrganization admin.Organization
	err = json.NewDecoder(res.Body).Decode(&adminOrganization)
	if err != nil {
		return nil, err
	}

	return toOrganization(adminOrganization), nil
}

// Deletes an organization by ID. All projects within the organization must be deleted first.
//
// Parameters:
//   - ctx: The request context.
//   - organizationId: The ID of the organization to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.Organization.Delete(ctx, "organization-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (o *DefaultOrganizationClient) Delete(ctx context.Context, organizationId string) error {
	res, err := o.restClient.DeleteOrganization(ctx, organizationId, &admin.DeleteOrganizationParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return handleErrorResponseBody(res, "failed to delete organization: ")
	}
	return nil
}

// [CreateAPIKeyParams] contains parameters for creating a new API key.
type CreateAPIKeyParams struct {
	// The name of the API key. The name must be 1-80 characters long.
	Name string `json:"name"`

	// (Optional) The roles to create the API key with.
	// Expected values: "ProjectEditor", "ProjectViewer", "ControlPlaneEditor", "ControlPlaneViewer", "DataPlaneEditor", "DataPlaneViewer"
	// Default is `["ProjectEditor"]`.
	Roles *[]string `json:"roles,omitempty"`
}

// Creates a new API key.
//
// Parameters:
//   - ctx: The request context.
//   - projectId: The ID of the project in which to create the API key.
//   - in: A pointer to [CreateAPIKeyParams] containing the API key configuration.
//
// Returns a pointer to an [APIKeyWithSecret] or an error.
//
// Example:
//
//	apiKeyWithSecret, err := adminClient.APIKey.Create(ctx, "project-id", &pinecone.CreateAPIKeyParams{
//		Name: "my-api-key",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (a *DefaultApiKeyClient) Create(ctx context.Context, projectId string, in *CreateAPIKeyParams) (*APIKeyWithSecret, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateAPIKeyParams) cannot be nil")
	}

	request := admin.CreateAPIKeyRequest{
		Name:  in.Name,
		Roles: in.Roles,
	}

	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := a.restClient.CreateApiKey(ctx, projectIdUUID, &admin.CreateApiKeyParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to create api key: ")
	}

	var adminApiKey admin.APIKeyWithSecret
	err = json.NewDecoder(res.Body).Decode(&adminApiKey)
	if err != nil {
		return nil, err
	}

	return toAPIKeyWithSecret(adminApiKey), nil
}

// [UpdateAPIKeyParams] contains parameters for updating an existing API key.
type UpdateAPIKeyParams struct {
	// (Optional) A new name for the API key. The name must be 1-80 characters long. If omitted, the name will not be updated.
	Name *string `json:"name,omitempty"`

	// (Optional) A new set of roles for the API key. Existing roles will be removed if not included.
	// Expected values:ProjectEditor, ProjectViewer, ControlPlaneEditor, ControlPlaneViewer, DataPlaneEditor, DataPlaneViewer
	// If this field is omitted, the roles will not be updated.
	Roles *[]string `json:"roles,omitempty"`
}

// Updates an existing API key by ID.
//
// Parameters:
//   - ctx: The request context.
//   - apiKeyId: The ID of the API key to update.
//   - in: A pointer to [UpdateAPIKeyParams] containing updated fields.
//
// Returns the updated [APIKey] or an error.
//
// Example:
//
//	apiKey, err := adminClient.APIKey.Update(ctx, "api-key-id", &pinecone.UpdateAPIKeyParams{
//		Name: "updated-name",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (a *DefaultApiKeyClient) Update(ctx context.Context, apiKeyId string, in *UpdateAPIKeyParams) (*APIKey, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateAPIKeyParams) cannot be nil")
	}

	apiKeyIdUUID, err := uuid.Parse(apiKeyId)
	if err != nil {
		return nil, fmt.Errorf("invalid apiKeyId: %w", err)
	}

	request := admin.UpdateAPIKeyRequest{
		Name:  in.Name,
		Roles: in.Roles,
	}

	res, err := a.restClient.UpdateApiKey(ctx, apiKeyIdUUID, &admin.UpdateApiKeyParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to update api key: ")
	}

	var adminApiKey admin.APIKey
	err = json.NewDecoder(res.Body).Decode(&adminApiKey)
	if err != nil {
		return nil, err
	}

	return toAPIKey(adminApiKey), nil
}

// Lists all API keys within a project by project ID.
//
// Parameters:
//   - ctx: The request context.
//   - projectId: The ID of the project to list API keys for.
//
// Returns a slice of [APIKey] objects or an error.
//
// Example:
//
//	apiKeys, err := adminClient.APIKey.List(ctx, "project-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (a *DefaultApiKeyClient) List(ctx context.Context, projectId string) ([]*APIKey, error) {
	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := a.restClient.ListProjectApiKeys(ctx, projectIdUUID, &admin.ListProjectApiKeysParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list api keys: ")
	}

	var listResp struct {
		Data *[]admin.APIKey `json:"data,omitempty"`
	}
	err = json.NewDecoder(res.Body).Decode(&listResp)
	if err != nil {
		return nil, err
	}

	var apiKeys []*APIKey
	if listResp.Data != nil {
		apiKeys = make([]*APIKey, len(*listResp.Data))
		for i, apiKey := range *listResp.Data {
			apiKeys[i] = toAPIKey(apiKey)
		}
	} else {
		apiKeys = make([]*APIKey, 0)
	}

	return apiKeys, nil
}

// Describes an API key by ID.
//
// Parameters:
//   - ctx: The request context.
//   - apiKeyId: The ID of the API key to describe.
//
// Returns a pointer to an [APIKey] or an error.
//
// Example:
//
//	apiKey, err := adminClient.APIKey.Describe(ctx, "api-key-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (a *DefaultApiKeyClient) Describe(ctx context.Context, apiKeyId string) (*APIKey, error) {
	apiKeyIdUUID, err := uuid.Parse(apiKeyId)
	if err != nil {
		return nil, fmt.Errorf("invalid apiKeyId: %w", err)
	}

	res, err := a.restClient.FetchApiKey(ctx, apiKeyIdUUID, &admin.FetchApiKeyParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe api key: ")
	}

	var adminApiKey admin.APIKey
	err = json.NewDecoder(res.Body).Decode(&adminApiKey)
	if err != nil {
		return nil, err
	}

	return toAPIKey(adminApiKey), nil
}

// Deletes an API key by ID.
//
// Parameters:
//   - ctx: The request context.
//   - apiKeyId: The ID of the API key to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.APIKey.Delete(ctx, "api-key-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (a *DefaultApiKeyClient) Delete(ctx context.Context, apiKeyId string) error {
	apiKeyIdUUID, err := uuid.Parse(apiKeyId)
	if err != nil {
		return fmt.Errorf("invalid apiKeyId: %w", err)
	}

	res, err := a.restClient.DeleteApiKey(ctx, apiKeyIdUUID, &admin.DeleteApiKeyParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete api key: ")
	}

	return nil
}

// [CreateRoleBindingParams] contains parameters for creating a new role binding.
type CreateRoleBindingParams struct {
	// The ID of the principal to grant the role to. The format depends on PrincipalType.
	PrincipalId string `json:"principal_id"`

	// The kind of principal that receives permissions from the role binding.
	PrincipalType PrincipalType `json:"principal_type"`

	// The kind of resource scope the role binding applies to.
	ResourceType ResourceType `json:"resource_type"`

	// The role to assign to the principal at the resource scope.
	Role string `json:"role"`

	// (Optional) The ID of the project the binding applies to. Required when
	// ResourceType is "project"; omit for "organization" scope.
	ResourceId *string `json:"resource_id,omitempty"`
}

// Creates a new role binding, granting a role to a principal at an organization or project scope.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [CreateRoleBindingParams] containing the role binding configuration.
//
// Returns a pointer to a [RoleBinding] or an error.
//
// Example:
//
//	roleBinding, err := adminClient.RoleBinding.Create(ctx, &pinecone.CreateRoleBindingParams{
//		PrincipalId:   "service-account-id",
//		PrincipalType: pinecone.PrincipalTypeServiceAccount,
//		ResourceType:  pinecone.ResourceTypeOrganization,
//		Role:          "OrgMember",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (r *DefaultRoleBindingClient) Create(ctx context.Context, in *CreateRoleBindingParams) (*RoleBinding, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateRoleBindingParams) cannot be nil")
	}

	request := admin.CreateRoleBindingRequest{
		PrincipalId:   in.PrincipalId,
		PrincipalType: string(in.PrincipalType),
		ResourceType:  string(in.ResourceType),
		ResourceId:    in.ResourceId,
		Role:          in.Role,
	}

	res, err := r.restClient.CreateRoleBinding(ctx, &admin.CreateRoleBindingParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to create role binding: ")
	}

	var adminRoleBinding admin.RoleBinding
	err = json.NewDecoder(res.Body).Decode(&adminRoleBinding)
	if err != nil {
		return nil, err
	}

	return toRoleBinding(adminRoleBinding), nil
}

// [ListRoleBindingsParams] contains the query parameters used when listing role bindings.
// All fields are optional filters. PrincipalType is required when PrincipalId is set,
// and ResourceType is required when ResourceId is set.
type ListRoleBindingsParams struct {
	// (Optional) Filter by principal type. Required when PrincipalId is set.
	PrincipalType *PrincipalType `json:"principal_type,omitempty"`

	// (Optional) Filter by principal ID. Requires PrincipalType.
	PrincipalId *string `json:"principal_id,omitempty"`

	// (Optional) Filter by resource type. Required when ResourceId is set.
	ResourceType *ResourceType `json:"resource_type,omitempty"`

	// (Optional) Filter by resource ID. Requires ResourceType.
	ResourceId *string `json:"resource_id,omitempty"`

	// (Optional) Filter by role.
	Role *string `json:"role,omitempty"`

	// (Optional) The maximum number of role bindings to return per page.
	Limit *int `json:"limit,omitempty"`

	// (Optional) Token to retrieve the next page of results. Will be nil if there are no more results.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// Lists role bindings, optionally filtered by principal, resource, or role.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [ListRoleBindingsParams] containing optional filters. May be nil to list with defaults.
//
// Returns a pointer to a [RoleBindingList] or an error.
//
// Example:
//
//	principalType := pinecone.PrincipalTypeServiceAccount
//	principalId := "service-account-id"
//	roleBindings, err := adminClient.RoleBinding.List(ctx, &pinecone.ListRoleBindingsParams{
//		PrincipalType: &principalType,
//		PrincipalId:   &principalId,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (r *DefaultRoleBindingClient) List(ctx context.Context, in *ListRoleBindingsParams) (*RoleBindingList, error) {
	params := &admin.ListRoleBindingsParams{XPineconeApiVersion: gen.PineconeApiVersion}
	if in != nil {
		if in.PrincipalType != nil {
			principalType := string(*in.PrincipalType)
			params.PrincipalType = &principalType
		}
		if in.ResourceType != nil {
			resourceType := string(*in.ResourceType)
			params.ResourceType = &resourceType
		}
		params.PrincipalId = in.PrincipalId
		params.ResourceId = in.ResourceId
		params.Role = in.Role
		params.Limit = in.Limit
		params.PaginationToken = in.PaginationToken
	}

	res, err := r.restClient.ListRoleBindings(ctx, params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list role bindings: ")
	}

	var adminRoleBindingList admin.RoleBindingList
	err = json.NewDecoder(res.Body).Decode(&adminRoleBindingList)
	if err != nil {
		return nil, err
	}

	return toRoleBindingList(adminRoleBindingList), nil
}

// Describes a role binding by ID.
//
// Parameters:
//   - ctx: The request context.
//   - roleBindingId: The ID of the role binding to describe.
//
// Returns a pointer to a [RoleBinding] or an error.
//
// Example:
//
//	roleBinding, err := adminClient.RoleBinding.Describe(ctx, "role-binding-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (r *DefaultRoleBindingClient) Describe(ctx context.Context, roleBindingId string) (*RoleBinding, error) {
	roleBindingIdUUID, err := uuid.Parse(roleBindingId)
	if err != nil {
		return nil, fmt.Errorf("invalid roleBindingId: %w", err)
	}

	res, err := r.restClient.FetchRoleBinding(ctx, roleBindingIdUUID, &admin.FetchRoleBindingParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe role binding: ")
	}

	var adminRoleBinding admin.RoleBinding
	err = json.NewDecoder(res.Body).Decode(&adminRoleBinding)
	if err != nil {
		return nil, err
	}

	return toRoleBinding(adminRoleBinding), nil
}

// Deletes a role binding by ID.
//
// Parameters:
//   - ctx: The request context.
//   - roleBindingId: The ID of the role binding to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.RoleBinding.Delete(ctx, "role-binding-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (r *DefaultRoleBindingClient) Delete(ctx context.Context, roleBindingId string) error {
	roleBindingIdUUID, err := uuid.Parse(roleBindingId)
	if err != nil {
		return fmt.Errorf("invalid roleBindingId: %w", err)
	}

	res, err := r.restClient.DeleteRoleBinding(ctx, roleBindingIdUUID, &admin.DeleteRoleBindingParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete role binding: ")
	}

	return nil
}

// [CreateServiceAccountParams] contains parameters for creating a new service account.
type CreateServiceAccountParams struct {
	// The human-readable name of the service account.
	Name string `json:"name"`

	// (Optional) Initial role bindings for the service account. Omitting the field or
	// passing an empty slice creates the service account with no role bindings; roles
	// can be added later via [RoleBindingClient].
	RoleBindings []RoleBindingInput `json:"role_bindings,omitempty"`
}

// Creates a new service account.
//
// The returned [ServiceAccountWithSecret] contains the OAuth client secret, which is
// returned only once and cannot be retrieved later. Store it securely and never log it.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [CreateServiceAccountParams] containing the service account configuration.
//
// Returns a pointer to a [ServiceAccountWithSecret] or an error.
//
// Example:
//
//	serviceAccountWithSecret, err := adminClient.ServiceAccount.Create(ctx, &pinecone.CreateServiceAccountParams{
//		Name: "my-service-account",
//		RoleBindings: []pinecone.RoleBindingInput{
//			{ResourceType: pinecone.ResourceTypeOrganization, Role: "OrgMember"},
//		},
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *DefaultServiceAccountClient) Create(ctx context.Context, in *CreateServiceAccountParams) (*ServiceAccountWithSecret, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateServiceAccountParams) cannot be nil")
	}

	request := admin.CreateServiceAccountRequest{
		Name: in.Name,
	}
	if in.RoleBindings != nil {
		roleBindings := make([]admin.RoleBindingInput, len(in.RoleBindings))
		for i, roleBinding := range in.RoleBindings {
			roleBindings[i] = toRoleBindingInput(roleBinding)
		}
		request.RoleBindings = &roleBindings
	}

	res, err := s.restClient.CreateServiceAccount(ctx, &admin.CreateServiceAccountParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, handleErrorResponseBody(res, "failed to create service account: ")
	}

	var adminServiceAccount admin.ServiceAccountWithSecret
	err = json.NewDecoder(res.Body).Decode(&adminServiceAccount)
	if err != nil {
		return nil, err
	}

	return toServiceAccountWithSecret(adminServiceAccount), nil
}

// [UpdateServiceAccountParams] contains parameters for updating an existing service account.
type UpdateServiceAccountParams struct {
	// (Optional) A new name for the service account. If omitted, the name is unchanged.
	Name *string `json:"name,omitempty"`
}

// Updates an existing service account by ID.
//
// Parameters:
//   - ctx: The request context.
//   - serviceAccountId: The ID of the service account to update.
//   - in: A pointer to [UpdateServiceAccountParams] containing updated fields.
//
// Returns the updated [ServiceAccount] or an error.
//
// Example:
//
//	newName := "renamed-service-account"
//	serviceAccount, err := adminClient.ServiceAccount.Update(ctx, "service-account-id", &pinecone.UpdateServiceAccountParams{
//		Name: &newName,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *DefaultServiceAccountClient) Update(ctx context.Context, serviceAccountId string, in *UpdateServiceAccountParams) (*ServiceAccount, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateServiceAccountParams) cannot be nil")
	}

	serviceAccountIdUUID, err := uuid.Parse(serviceAccountId)
	if err != nil {
		return nil, fmt.Errorf("invalid serviceAccountId: %w", err)
	}

	request := admin.UpdateServiceAccountRequest{
		Name: in.Name,
	}

	res, err := s.restClient.UpdateServiceAccount(ctx, serviceAccountIdUUID, &admin.UpdateServiceAccountParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to update service account: ")
	}

	var adminServiceAccount admin.ServiceAccount
	err = json.NewDecoder(res.Body).Decode(&adminServiceAccount)
	if err != nil {
		return nil, err
	}

	return toServiceAccount(adminServiceAccount), nil
}

// [ListServiceAccountsParams] contains the query parameters used when listing service accounts.
type ListServiceAccountsParams struct {
	// (Optional) The maximum number of service accounts to return per page.
	Limit *int `json:"limit,omitempty"`

	// (Optional) Token to retrieve the next page of results. Will be nil if there are no more results.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// Lists all service accounts within the organization.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [ListServiceAccountsParams] containing pagination options. May be nil to list with defaults.
//
// Returns a pointer to a [ServiceAccountList] or an error.
//
// Example:
//
//	serviceAccounts, err := adminClient.ServiceAccount.List(ctx, nil)
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *DefaultServiceAccountClient) List(ctx context.Context, in *ListServiceAccountsParams) (*ServiceAccountList, error) {
	params := &admin.ListServiceAccountsParams{XPineconeApiVersion: gen.PineconeApiVersion}
	if in != nil {
		params.Limit = in.Limit
		params.PaginationToken = in.PaginationToken
	}

	res, err := s.restClient.ListServiceAccounts(ctx, params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list service accounts: ")
	}

	var adminServiceAccountList admin.ServiceAccountList
	err = json.NewDecoder(res.Body).Decode(&adminServiceAccountList)
	if err != nil {
		return nil, err
	}

	return toServiceAccountList(adminServiceAccountList), nil
}

// Describes a service account by ID.
//
// Parameters:
//   - ctx: The request context.
//   - serviceAccountId: The ID of the service account to describe.
//
// Returns a pointer to a [ServiceAccount] or an error.
//
// Example:
//
//	serviceAccount, err := adminClient.ServiceAccount.Describe(ctx, "service-account-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *DefaultServiceAccountClient) Describe(ctx context.Context, serviceAccountId string) (*ServiceAccount, error) {
	serviceAccountIdUUID, err := uuid.Parse(serviceAccountId)
	if err != nil {
		return nil, fmt.Errorf("invalid serviceAccountId: %w", err)
	}

	res, err := s.restClient.FetchServiceAccount(ctx, serviceAccountIdUUID, &admin.FetchServiceAccountParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe service account: ")
	}

	var adminServiceAccount admin.ServiceAccount
	err = json.NewDecoder(res.Body).Decode(&adminServiceAccount)
	if err != nil {
		return nil, err
	}

	return toServiceAccount(adminServiceAccount), nil
}

// Rotates the OAuth client secret for a service account by ID.
//
// The returned [ServiceAccountWithSecret] contains the new secret, which is returned
// only once and cannot be retrieved later. Store it securely and never log it. The
// previous secret is invalidated.
//
// Parameters:
//   - ctx: The request context.
//   - serviceAccountId: The ID of the service account whose secret should be rotated.
//
// Returns a pointer to a [ServiceAccountWithSecret] or an error.
//
// Example:
//
//	serviceAccountWithSecret, err := adminClient.ServiceAccount.RotateSecret(ctx, "service-account-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *DefaultServiceAccountClient) RotateSecret(ctx context.Context, serviceAccountId string) (*ServiceAccountWithSecret, error) {
	serviceAccountIdUUID, err := uuid.Parse(serviceAccountId)
	if err != nil {
		return nil, fmt.Errorf("invalid serviceAccountId: %w", err)
	}

	res, err := s.restClient.RotateServiceAccountSecret(ctx, serviceAccountIdUUID, &admin.RotateServiceAccountSecretParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to rotate service account secret: ")
	}

	var adminServiceAccount admin.ServiceAccountWithSecret
	err = json.NewDecoder(res.Body).Decode(&adminServiceAccount)
	if err != nil {
		return nil, err
	}

	return toServiceAccountWithSecret(adminServiceAccount), nil
}

// Deletes a service account by ID.
//
// Parameters:
//   - ctx: The request context.
//   - serviceAccountId: The ID of the service account to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.ServiceAccount.Delete(ctx, "service-account-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *DefaultServiceAccountClient) Delete(ctx context.Context, serviceAccountId string) error {
	serviceAccountIdUUID, err := uuid.Parse(serviceAccountId)
	if err != nil {
		return fmt.Errorf("invalid serviceAccountId: %w", err)
	}

	res, err := s.restClient.DeleteServiceAccount(ctx, serviceAccountIdUUID, &admin.DeleteServiceAccountParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete service account: ")
	}

	return nil
}

// [CreateInviteParams] contains parameters for creating and sending a new invite.
type CreateInviteParams struct {
	// The email address to invite.
	Email string `json:"email"`

	// Role bindings for the invitee. Must include at least one organization-scoped
	// binding that grants organization membership (e.g. "OrgOwner", "OrgManager",
	// "OrgBillingAdmin", or "OrgMember"); project-scoped bindings are optional.
	RoleBindings []RoleBindingInput `json:"role_bindings"`
}

// Creates and sends a new invite to join the organization.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [CreateInviteParams] containing the invite configuration.
//
// Returns a pointer to an [Invite] or an error.
//
// Example:
//
//	invite, err := adminClient.Invite.Create(ctx, &pinecone.CreateInviteParams{
//		Email: "teammate@example.com",
//		RoleBindings: []pinecone.RoleBindingInput{
//			{ResourceType: pinecone.ResourceTypeOrganization, Role: "OrgMember"},
//		},
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (i *DefaultInviteClient) Create(ctx context.Context, in *CreateInviteParams) (*Invite, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateInviteParams) cannot be nil")
	}

	roleBindings := make([]admin.RoleBindingInput, len(in.RoleBindings))
	for idx, roleBinding := range in.RoleBindings {
		roleBindings[idx] = toRoleBindingInput(roleBinding)
	}

	request := admin.CreateInviteRequest{
		Email:        openapi_types.Email(in.Email),
		RoleBindings: roleBindings,
	}

	res, err := i.restClient.CreateInvite(ctx, &admin.CreateInviteParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to create invite: ")
	}

	var adminInvite admin.Invite
	err = json.NewDecoder(res.Body).Decode(&adminInvite)
	if err != nil {
		return nil, err
	}

	return toInvite(adminInvite), nil
}

// [ListInvitesParams] contains the query parameters used when listing invites.
type ListInvitesParams struct {
	// (Optional) The maximum number of invites to return per page.
	Limit *int `json:"limit,omitempty"`

	// (Optional) Token to retrieve the next page of results. Will be nil if there are no more results.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// Lists invites in the organization.
//
// List returns only "pending" and "expired" invites; a "processed" invite is
// returned only when fetching a single invite by ID with [DefaultInviteClient.Describe].
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [ListInvitesParams] containing pagination options. May be nil to list with defaults.
//
// Returns a pointer to an [InviteList] or an error.
//
// Example:
//
//	invites, err := adminClient.Invite.List(ctx, nil)
//	if err != nil {
//		log.Fatal(err)
//	}
func (i *DefaultInviteClient) List(ctx context.Context, in *ListInvitesParams) (*InviteList, error) {
	params := &admin.ListInvitesParams{XPineconeApiVersion: gen.PineconeApiVersion}
	if in != nil {
		params.Limit = in.Limit
		params.PaginationToken = in.PaginationToken
	}

	res, err := i.restClient.ListInvites(ctx, params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list invites: ")
	}

	var adminInviteList admin.InviteList
	err = json.NewDecoder(res.Body).Decode(&adminInviteList)
	if err != nil {
		return nil, err
	}

	return toInviteList(adminInviteList), nil
}

// Describes an invite by ID.
//
// Parameters:
//   - ctx: The request context.
//   - inviteId: The ID of the invite to describe.
//
// Returns a pointer to an [Invite] or an error.
//
// Example:
//
//	invite, err := adminClient.Invite.Describe(ctx, "invite-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (i *DefaultInviteClient) Describe(ctx context.Context, inviteId string) (*Invite, error) {
	inviteIdUUID, err := uuid.Parse(inviteId)
	if err != nil {
		return nil, fmt.Errorf("invalid inviteId: %w", err)
	}

	res, err := i.restClient.FetchInvite(ctx, inviteIdUUID, &admin.FetchInviteParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe invite: ")
	}

	var adminInvite admin.Invite
	err = json.NewDecoder(res.Body).Decode(&adminInvite)
	if err != nil {
		return nil, err
	}

	return toInvite(adminInvite), nil
}

// Resends an existing invite by ID, resending the invite email and extending the
// invite's expiration to seven days from now.
//
// Parameters:
//   - ctx: The request context.
//   - inviteId: The ID of the invite to resend.
//
// Returns the updated [Invite] or an error.
//
// Example:
//
//	invite, err := adminClient.Invite.Resend(ctx, "invite-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (i *DefaultInviteClient) Resend(ctx context.Context, inviteId string) (*Invite, error) {
	inviteIdUUID, err := uuid.Parse(inviteId)
	if err != nil {
		return nil, fmt.Errorf("invalid inviteId: %w", err)
	}

	res, err := i.restClient.ResendInvite(ctx, inviteIdUUID, &admin.ResendInviteParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to resend invite: ")
	}

	var adminInvite admin.Invite
	err = json.NewDecoder(res.Body).Decode(&adminInvite)
	if err != nil {
		return nil, err
	}

	return toInvite(adminInvite), nil
}

// Deletes an invite by ID.
//
// Parameters:
//   - ctx: The request context.
//   - inviteId: The ID of the invite to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.Invite.Delete(ctx, "invite-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (i *DefaultInviteClient) Delete(ctx context.Context, inviteId string) error {
	inviteIdUUID, err := uuid.Parse(inviteId)
	if err != nil {
		return fmt.Errorf("invalid inviteId: %w", err)
	}

	res, err := i.restClient.DeleteInvite(ctx, inviteIdUUID, &admin.DeleteInviteParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete invite: ")
	}

	return nil
}

// [ListUsersParams] contains the query parameters used when listing users.
type ListUsersParams struct {
	// (Optional) Case-insensitive filter on the user's email address.
	Email *string `json:"email,omitempty"`

	// (Optional) The maximum number of users to return per page.
	Limit *int `json:"limit,omitempty"`

	// (Optional) Token to retrieve the next page of results. Will be nil if there are no more results.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// Lists users in the organization, optionally filtered by email.
//
// Parameters:
//   - ctx: The request context.
//   - in: A pointer to [ListUsersParams] containing optional filters and pagination options. May be nil to list with defaults.
//
// Returns a pointer to a [UserList] or an error.
//
// Example:
//
//	users, err := adminClient.User.List(ctx, nil)
//	if err != nil {
//		log.Fatal(err)
//	}
func (u *DefaultUserClient) List(ctx context.Context, in *ListUsersParams) (*UserList, error) {
	params := &admin.ListUsersParams{XPineconeApiVersion: gen.PineconeApiVersion}
	if in != nil {
		if in.Email != nil {
			email := openapi_types.Email(*in.Email)
			params.Email = &email
		}
		params.Limit = in.Limit
		params.PaginationToken = in.PaginationToken
	}

	res, err := u.restClient.ListUsers(ctx, params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list users: ")
	}

	var adminUserList admin.UserList
	err = json.NewDecoder(res.Body).Decode(&adminUserList)
	if err != nil {
		return nil, err
	}

	return toUserList(adminUserList), nil
}

// Describes a user by ID.
//
// Parameters:
//   - ctx: The request context.
//   - userId: The ID of the user to describe.
//
// Returns a pointer to a [User] or an error.
//
// Example:
//
//	user, err := adminClient.User.Describe(ctx, "user-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (u *DefaultUserClient) Describe(ctx context.Context, userId string) (*User, error) {
	userIdUUID, err := uuid.Parse(userId)
	if err != nil {
		return nil, fmt.Errorf("invalid userId: %w", err)
	}

	res, err := u.restClient.FetchUser(ctx, userIdUUID, &admin.FetchUserParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe user: ")
	}

	var adminUser admin.User
	err = json.NewDecoder(res.Body).Decode(&adminUser)
	if err != nil {
		return nil, err
	}

	return toUser(adminUser), nil
}

// Deletes a user by ID, removing them from the organization.
//
// Parameters:
//   - ctx: The request context.
//   - userId: The ID of the user to delete.
//
// Returns an error if deletion fails.
//
// Example:
//
//	err := adminClient.User.Delete(ctx, "user-id")
//	if err != nil {
//		log.Fatal(err)
//	}
func (u *DefaultUserClient) Delete(ctx context.Context, userId string) error {
	userIdUUID, err := uuid.Parse(userId)
	if err != nil {
		return fmt.Errorf("invalid userId: %w", err)
	}

	res, err := u.restClient.DeleteUser(ctx, userIdUUID, &admin.DeleteUserParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete user: ")
	}

	return nil
}

type authTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

func getAuthToken(ctx context.Context, clientId string, clientSecret string, opts ...admin.ClientOption) (string, error) {
	// build REST client for retrieving token
	authServer := "https://login.pinecone.io"
	tokenClient, err := admin.NewClient(authServer, opts...)
	if err != nil {
		return "", err
	}

	// build authentication request
	serverURL, err := url.Parse(authServer)
	if err != nil {
		return "", err
	}

	operationPath := "/oauth/token"
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return "", err
	}

	bodyMap := map[string]string{
		"client_id":     clientId,
		"client_secret": clientSecret,
		"grant_type":    "client_credentials",
		"audience":      "https://api.pinecone.io/",
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return "", err
	}

	bodyReader := bytes.NewReader(body)
	req, err := http.NewRequest("POST", queryURL.String(), bodyReader)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	res, err := tokenClient.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", handleErrorResponseBody(res, "failed to get auth token: %s")
	}

	var tokenResponse authTokenResponse
	err = json.NewDecoder(res.Body).Decode(&tokenResponse)
	if err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}

func buildAdminClientOptions(in NewAdminClientParams) []admin.ClientOption {
	clientOptions := []admin.ClientOption{}
	headerProviders := buildAdminClientProviderHeaders(in)
	for _, provider := range headerProviders {
		clientOptions = append(clientOptions, admin.WithRequestEditorFn(provider.Intercept))
	}

	// apply custom http client if provided
	if in.RestClient != nil {
		clientOptions = append(clientOptions, admin.WithHTTPClient(in.RestClient))
	}

	return clientOptions
}

func buildAdminClientProviderHeaders(in NewAdminClientParams) []*provider.CustomHeader {
	providers := []*provider.CustomHeader{}

	sourceTag := ""
	if in.SourceTag != nil {
		sourceTag = *in.SourceTag
	}

	// build and apply user agent header
	providers = append(providers, provider.NewHeaderProvider("User-Agent", useragent.BuildUserAgent(sourceTag)))
	// build and apply api version header
	providers = append(providers, provider.NewHeaderProvider("X-Pinecone-Api-Version", gen.PineconeApiVersion))

	// get headers from environment
	envAdditionalHeaders, hasEnvAdditionalHeaders := os.LookupEnv("PINECONE_ADDITIONAL_HEADERS")
	additionalHeaders := make(map[string]string)
	if hasEnvAdditionalHeaders {
		err := json.Unmarshal([]byte(envAdditionalHeaders), &additionalHeaders)
		if err != nil {
			log.Printf("failed to parse PINECONE_ADDITIONAL_HEADERS: %v", err)
		}
	}
	// merge headers from parameters if passed with additionalHeaders from environment
	if in.Headers != nil {
		for key, value := range *in.Headers {
			additionalHeaders[key] = value
		}
	}
	// create header providers
	for key, value := range additionalHeaders {
		providers = append(providers, provider.NewHeaderProvider(key, value))
	}

	return providers
}

func toProject(project admin.Project) *Project {
	return &Project{
		CreatedAt:               project.CreatedAt,
		ForceEncryptionWithCmek: project.ForceEncryptionWithCmek,
		Id:                      project.Id.String(),
		MaxPods:                 project.MaxPods,
		Name:                    project.Name,
		OrganizationId:          project.OrganizationId,
	}
}

func toOrganization(organization admin.Organization) *Organization {
	return &Organization{
		CreatedAt:     organization.CreatedAt,
		Id:            organization.Id,
		Name:          organization.Name,
		PaymentStatus: organization.PaymentStatus,
		Plan:          organization.Plan,
		SupportTier:   organization.SupportTier,
	}
}

func toAPIKey(apiKey admin.APIKey) *APIKey {
	return &APIKey{
		Id:        apiKey.Id.String(),
		Name:      apiKey.Name,
		ProjectId: apiKey.ProjectId.String(),
		Roles:     apiKey.Roles,
	}
}

func toAPIKeyWithSecret(apiKey admin.APIKeyWithSecret) *APIKeyWithSecret {
	return &APIKeyWithSecret{
		Key:   *toAPIKey(apiKey.Key),
		Value: apiKey.Value,
	}
}

func toRoleBinding(roleBinding admin.RoleBinding) *RoleBinding {
	return &RoleBinding{
		Id:            roleBinding.Id.String(),
		PrincipalId:   roleBinding.PrincipalId,
		PrincipalType: PrincipalType(roleBinding.PrincipalType),
		ResourceId:    roleBinding.ResourceId,
		ResourceType:  ResourceType(roleBinding.ResourceType),
		Role:          roleBinding.Role,
		CreatedAt:     roleBinding.CreatedAt,
	}
}

func toRoleBindingList(roleBindingList admin.RoleBindingList) *RoleBindingList {
	list := &RoleBindingList{
		Data: make([]*RoleBinding, len(roleBindingList.Data)),
	}
	for i, roleBinding := range roleBindingList.Data {
		list.Data[i] = toRoleBinding(roleBinding)
	}
	if roleBindingList.Pagination != nil && roleBindingList.Pagination.Next != nil {
		list.Pagination = &Pagination{Next: *roleBindingList.Pagination.Next}
	}
	return list
}

func toRoleBindingInput(roleBindingInput RoleBindingInput) admin.RoleBindingInput {
	return admin.RoleBindingInput{
		ResourceType: string(roleBindingInput.ResourceType),
		Role:         roleBindingInput.Role,
		ResourceId:   roleBindingInput.ResourceId,
	}
}

func toServiceAccount(serviceAccount admin.ServiceAccount) *ServiceAccount {
	return &ServiceAccount{
		Id:        serviceAccount.Id.String(),
		Name:      serviceAccount.Name,
		ClientId:  serviceAccount.ClientId,
		CreatedAt: serviceAccount.CreatedAt,
		UpdatedAt: serviceAccount.UpdatedAt,
	}
}

func toServiceAccountWithSecret(serviceAccount admin.ServiceAccountWithSecret) *ServiceAccountWithSecret {
	return &ServiceAccountWithSecret{
		ServiceAccount: *toServiceAccount(serviceAccount.ServiceAccount),
		ClientSecret:   serviceAccount.ClientSecret,
	}
}

func toServiceAccountList(serviceAccountList admin.ServiceAccountList) *ServiceAccountList {
	list := &ServiceAccountList{
		Data: make([]*ServiceAccount, len(serviceAccountList.Data)),
	}
	for i, serviceAccount := range serviceAccountList.Data {
		list.Data[i] = toServiceAccount(serviceAccount)
	}
	if serviceAccountList.Pagination != nil && serviceAccountList.Pagination.Next != nil {
		list.Pagination = &Pagination{Next: *serviceAccountList.Pagination.Next}
	}
	return list
}

func toInvite(invite admin.Invite) *Invite {
	return &Invite{
		Id:          invite.Id.String(),
		Email:       string(invite.Email),
		Status:      InviteStatus(invite.Status),
		CreatedAt:   invite.CreatedAt,
		ExpiresAt:   invite.ExpiresAt,
		ProcessedAt: invite.ProcessedAt,
	}
}

func toInviteList(inviteList admin.InviteList) *InviteList {
	list := &InviteList{
		Data: make([]*Invite, len(inviteList.Data)),
	}
	for i, invite := range inviteList.Data {
		list.Data[i] = toInvite(invite)
	}
	if inviteList.Pagination != nil && inviteList.Pagination.Next != nil {
		list.Pagination = &Pagination{Next: *inviteList.Pagination.Next}
	}
	return list
}

func toUser(user admin.User) *User {
	return &User{
		Id:    user.Id.String(),
		Email: string(user.Email),
		Name:  user.Name,
	}
}

func toUserList(userList admin.UserList) *UserList {
	list := &UserList{
		Data: make([]*User, len(userList.Data)),
	}
	for i, user := range userList.Data {
		list.Data[i] = toUser(user)
	}
	if userList.Pagination != nil && userList.Pagination.Next != nil {
		list.Pagination = &Pagination{Next: *userList.Pagination.Next}
	}
	return list
}
