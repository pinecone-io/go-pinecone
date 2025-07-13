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
	"github.com/pinecone-io/go-pinecone/v4/internal/gen"
	"github.com/pinecone-io/go-pinecone/v4/internal/gen/admin"
	"github.com/pinecone-io/go-pinecone/v4/internal/provider"
	"github.com/pinecone-io/go-pinecone/v4/internal/useragent"
)

// [AdminClient] provides access to Pinecone's administrative APIs, which supports
// managing projects, organizations, and API keys. It is constructed using
// [NewAdminClient] or [NewAdminClientWithContext].
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
}

// [ProjectClient] defines operations for managing Pinecone projects.
type ProjectClient interface {
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
	Create(ctx context.Context, in *CreateProjectParams) (*Project, error)

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
	Update(ctx context.Context, projectId string, in *UpdateProjectParams) (*Project, error)

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
	List(ctx context.Context) ([]*Project, error)

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
	Describe(ctx context.Context, projectId string) (*Project, error)

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
	Delete(ctx context.Context, projectId string) error
}

// [OrganizationClient] defines operations for managing organizations.
type OrganizationClient interface {
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
	List(ctx context.Context) ([]*Organization, error)

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
	Describe(ctx context.Context, organizationId string) (*Organization, error)

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
	Update(ctx context.Context, organizationId string, in *UpdateOrganizationParams) (*Organization, error)

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
	Delete(ctx context.Context, organizationId string) error
}

// [APIKeyClient] defines operations for managing API keys within a project.
type APIKeyClient interface {
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
	Create(ctx context.Context, projectId string, in *CreateAPIKeyParams) (*APIKeyWithSecret, error)

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
	Update(ctx context.Context, apiKeyId string, in *UpdateAPIKeyParams) (*APIKey, error)

	// Lists all API keys within a project.
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
	List(ctx context.Context, projectId string) ([]*APIKey, error)

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
	Describe(ctx context.Context, apiKeyId string) (*APIKey, error)

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
	Delete(ctx context.Context, apiKeyId string) error
}

// [NewAdminClientParams] contains parameters used to configure the [AdminClient].
// You must provide a client ID and secret either directly or via environment
// variables (PINECONE_CLIENT_ID and PINECONE_CLIENT_SECRET).
type NewAdminClientParams struct {
	// The OAuth client ID used for authentication.
	ClientId string

	// The OAuth client secret used for authentication.
	ClientSecret string

	// (Optional) Additional headers to include in the request.
	Headers *map[string]string

	// (Optional)The HTTP client to use for the request.
	RestClient *http.Client

	// (Optional)The source tag to include in the request.
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
	osClientId := os.Getenv("PINECONE_CLIENT_ID")
	osClientSecret := os.Getenv("PINECONE_CLIENT_SECRET")
	hasClientId := valueOrFallback(in.ClientId, osClientId) != ""
	hasClientSecret := valueOrFallback(in.ClientSecret, osClientSecret) != ""

	if !hasClientId {
		return nil, fmt.Errorf("no ClientId provided, please pass an ClientId for authorization through NewAdminClientParams or set the PINECONE_CLIENT_ID environment variable")
	}
	if !hasClientSecret {
		return nil, fmt.Errorf("no ClientSecret provided, please pass an ClientSecret for authorization through NewAdminClientParams or set the PINECONE_CLIENT_SECRET environment variable")
	}

	clientOptions := buildAdminClientOptions(in)

	authToken, err := getAuthToken(ctx, in.ClientId, in.ClientSecret, clientOptions...)
	if err != nil {
		return nil, err
	}

	authProvider := provider.NewHeaderProvider("Authorization", fmt.Sprintf("Bearer %s", authToken))
	clientOptions = append(clientOptions, admin.WithRequestEditorFn(authProvider.Intercept))

	adminClient, err := admin.NewClient("https://api.pinecone.io", clientOptions...)
	if err != nil {
		return nil, err
	}

	return &AdminClient{
		Project: &projectClient{
			restClient: adminClient,
		},
		Organization: &organizationClient{
			restClient: adminClient,
		},
		APIKey: &apiKeyClient{
			restClient: adminClient,
		},
	}, nil
}

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

func (p *projectClient) Create(ctx context.Context, in *CreateProjectParams) (*Project, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateProjectParams) cannot be nil")
	}

	request := admin.CreateProjectRequest{
		ForceEncryptionWithCmek: in.ForceEncryptionWithCmek,
		MaxPods:                 in.MaxPods,
		Name:                    in.Name,
	}

	res, err := p.restClient.CreateProject(ctx, request)
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

func (p *projectClient) Update(ctx context.Context, projectId string, in *UpdateProjectParams) (*Project, error) {
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

	res, err := p.restClient.UpdateProject(ctx, projectIdUUID, request)
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

func (p *projectClient) List(ctx context.Context) ([]*Project, error) {
	res, err := p.restClient.ListProjects(ctx)
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

func (p *projectClient) Describe(ctx context.Context, projectId string) (*Project, error) {
	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := p.restClient.FetchProject(ctx, projectIdUUID)
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

func (p *projectClient) Delete(ctx context.Context, projectId string) error {
	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := p.restClient.DeleteProject(ctx, projectIdUUID)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete project: ")
	}

	return nil
}

func (o *organizationClient) List(ctx context.Context) ([]*Organization, error) {
	res, err := o.restClient.ListOrganizations(ctx)
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

func (o *organizationClient) Describe(ctx context.Context, organizationId string) (*Organization, error) {
	res, err := o.restClient.FetchOrganization(ctx, organizationId)
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

func (o *organizationClient) Update(ctx context.Context, organizationId string, in *UpdateOrganizationParams) (*Organization, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateOrganizationParams) cannot be nil")
	}

	request := admin.UpdateOrganizationRequest{
		Name: in.Name,
	}

	res, err := o.restClient.UpdateOrganization(ctx, organizationId, request)
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

func (o *organizationClient) Delete(ctx context.Context, organizationId string) error {
	res, err := o.restClient.DeleteOrganization(ctx, organizationId)
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

func (a *apiKeyClient) Create(ctx context.Context, projectId string, in *CreateAPIKeyParams) (*APIKeyWithSecret, error) {
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

	res, err := a.restClient.CreateApiKey(ctx, projectIdUUID, request)
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

func (a *apiKeyClient) Update(ctx context.Context, apiKeyId string, in *UpdateAPIKeyParams) (*APIKey, error) {
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

	res, err := a.restClient.UpdateApiKey(ctx, apiKeyIdUUID, request)
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

func (a *apiKeyClient) List(ctx context.Context, projectId string) ([]*APIKey, error) {
	projectIdUUID, err := uuid.Parse(projectId)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}

	res, err := a.restClient.ListApiKeys(ctx, projectIdUUID)
	if err != nil {
		return nil, err
	}

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

func (a *apiKeyClient) Describe(ctx context.Context, apiKeyId string) (*APIKey, error) {
	apiKeyIdUUID, err := uuid.Parse(apiKeyId)
	if err != nil {
		return nil, fmt.Errorf("invalid apiKeyId: %w", err)
	}

	res, err := a.restClient.FetchApiKey(ctx, apiKeyIdUUID)
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

func (a *apiKeyClient) Delete(ctx context.Context, apiKeyId string) error {
	apiKeyIdUUID, err := uuid.Parse(apiKeyId)
	if err != nil {
		return fmt.Errorf("invalid apiKeyId: %w", err)
	}

	res, err := a.restClient.DeleteApiKey(ctx, apiKeyIdUUID)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete api key: ")
	}

	return nil
}

type projectClient struct {
	restClient *admin.Client
}

type organizationClient struct {
	restClient *admin.Client
}

type apiKeyClient struct {
	restClient *admin.Client
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
