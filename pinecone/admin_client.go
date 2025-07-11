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
	"time"

	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/v4/internal/gen"
	"github.com/pinecone-io/go-pinecone/v4/internal/gen/admin"
	"github.com/pinecone-io/go-pinecone/v4/internal/provider"
	"github.com/pinecone-io/go-pinecone/v4/internal/useragent"
)

type AdminClient struct {
	Project      ProjectClient
	Organization OrganizationClient
	ApiKey       ApiKeyClient
}

type ProjectClient interface {
	Create(ctx context.Context, in *CreateProjectParams) (*Project, error)
	Update(ctx context.Context, projectId string, in *UpdateProjectParams) (*Project, error)
	List(ctx context.Context) ([]*Project, error)
	Describe(ctx context.Context, projectId string) (*Project, error)
	Delete(ctx context.Context, projectId string) error
}

type OrganizationClient interface {
	List(ctx context.Context) ([]*Organization, error)
	Describe(ctx context.Context, organizationId string) (*Organization, error)
	Update(ctx context.Context, organizationId string, in *UpdateOrganizationParams) (*Organization, error)
	Delete(ctx context.Context, organizationId string) error
}

type ApiKeyClient interface {
	Create(ctx context.Context, projectId string, in *CreateApiKeyParams) (*ApiKeyWithSecret, error)
	Update(ctx context.Context, apiKeyId string, in *UpdateApiKeyParams) (*ApiKey, error)
	List(ctx context.Context, projectId string) ([]*ApiKey, error)
	Describe(ctx context.Context, apiKeyId string) (*ApiKey, error)
	Delete(ctx context.Context, apiKeyId string) error
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

type NewAdminClientParams struct {
	ClientId     string
	ClientSecret string
	Headers      *map[string]string
	RestClient   *http.Client
	SourceTag    *string
}

func NewAdminClient(in NewAdminClientParams) (*AdminClient, error) {
	return NewAdminClientWithContext(context.Background(), in)
}

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
		ApiKey: &apiKeyClient{
			restClient: adminClient,
		},
	}, nil
}

// Project The details of a project.
type Project struct {
	// CreatedAt The date and time when the project was created.
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// ForceEncryptionWithCmek Whether to force encryption with a customer-managed encryption key (CMEK).
	ForceEncryptionWithCmek bool `json:"force_encryption_with_cmek"`

	// Id The unique ID of the project.
	Id string `json:"id"`

	// MaxPods The maximum number of Pods that can be created in the project.
	MaxPods int `json:"max_pods"`

	// Name The name of the project.
	Name string `json:"name"`

	// OrganizationId The unique ID of the organization that the project belongs to.
	OrganizationId string `json:"organization_id"`
}

type CreateProjectParams struct {
	// ForceEncryptionWithCmek Whether to force encryption with a customer-managed encryption key (CMEK). Default is `false`.
	ForceEncryptionWithCmek *bool `json:"force_encryption_with_cmek,omitempty"`

	// MaxPods The maximum number of Pods that can be created in the project. Default is `0` (serverless only).
	MaxPods *int `json:"max_pods,omitempty"`

	// Name The name of the new project.
	Name string `json:"name"`
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

type UpdateProjectParams struct {
	// ForceEncryptionWithCmek Whether to force encryption with a customer-managed encryption key (CMEK). Once enabled, CMEK encryption cannot be disabled.
	ForceEncryptionWithCmek *bool `json:"force_encryption_with_cmek,omitempty"`

	// MaxPods The maximum number of Pods that can be created in the project.
	MaxPods *int `json:"max_pods,omitempty"`

	// Name The name of the new project.
	Name *string `json:"name,omitempty"`
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
		Name: in.Name,
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

// Organization The details of an organization.
type Organization struct {
	// CreatedAt The date and time when the organization was created.
	CreatedAt time.Time `json:"created_at"`

	// Id The unique ID of the organization.
	Id string `json:"id"`

	// Name The name of the organization.
	Name string `json:"name"`

	// PaymentStatus The current payment status of the organization.
	PaymentStatus string `json:"payment_status"`

	// Plan The current plan the organization is on.
	Plan string `json:"plan"`

	// SupportTier The support tier of the organization.
	SupportTier string `json:"support_tier"`
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

type UpdateOrganizationParams struct {
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

type CreateApiKeyParams struct {
	// Name The name of the API key. The name must be 1-80 characters long.
	Name string `json:"name"`

	// Roles The roles to create the API key with. Default is `["ProjectEditor"]`.
	Roles *[]string `json:"roles,omitempty"`
}

type ApiKeyWithSecret struct {
	// Key The details of an API key, without the secret.
	Key ApiKey `json:"key"`

	// Value The value to use as an API key. New keys will have the format `"pckey_<public-label>_<unique-key>"`. The entire string should be used when authenticating.
	Value string `json:"value"`
}

func (a *apiKeyClient) Create(ctx context.Context, projectId string, in *CreateApiKeyParams) (*ApiKeyWithSecret, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateApiKeyParams) cannot be nil")
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

	return toApiKeyWithSecret(adminApiKey), nil
}

type UpdateApiKeyParams struct {
	// Name A new name for the API key. The name must be 1-80 characters long. If omitted, the name will not be updated.
	Name *string `json:"name,omitempty"`

	// Roles A new set of roles for the API key. Existing roles will be removed if not included.
	// If this field is omitted, the roles will not be updated.
	Roles *[]string `json:"roles,omitempty"`
}

func (a *apiKeyClient) Update(ctx context.Context, apiKeyId string, in *UpdateApiKeyParams) (*ApiKey, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateApiKeyParams) cannot be nil")
	}

	apiKeyIdUUID, err := uuid.Parse(apiKeyId)
	if err != nil {
		return nil, fmt.Errorf("invalid apiKeyId: %w", err)
	}

	request := admin.UpdateAPIKeyRequest{
		Name: in.Name,
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

	return toApiKey(adminApiKey), nil
}

type ApiKey struct {
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	ProjectId string   `json:"project_id"`
	Roles     []string `json:"roles"`
}

func (a *apiKeyClient) List(ctx context.Context, projectId string) ([]*ApiKey, error) {
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

	var apiKeys []*ApiKey
	if listResp.Data != nil {
		apiKeys = make([]*ApiKey, len(*listResp.Data))
		for i, apiKey := range *listResp.Data {
			apiKeys[i] = toApiKey(apiKey)
		}
	} else {
		apiKeys = make([]*ApiKey, 0)
	}

	return apiKeys, nil
}

func (a *apiKeyClient) Describe(ctx context.Context, apiKeyId string) (*ApiKey, error) {
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

	return toApiKey(adminApiKey), nil
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

func toApiKey(apiKey admin.APIKey) *ApiKey {
	return &ApiKey{
		Id:        apiKey.Id.String(),
		Name:      apiKey.Name,
		ProjectId: apiKey.ProjectId.String(),
		Roles:     apiKey.Roles,
	}
}

func toApiKeyWithSecret(apiKey admin.APIKeyWithSecret) *ApiKeyWithSecret {
	return &ApiKeyWithSecret{
		Key:   *toApiKey(apiKey.Key),
		Value: apiKey.Value,
	}
}
