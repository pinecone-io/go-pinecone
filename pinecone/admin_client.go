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

	return &AdminClient{restClient: adminClient}, nil
}

// Project The details of a project.
type Project struct {
	// CreatedAt The date and time when the project was created.
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// ForceEncryptionWithCmek Whether to force encryption with a customer-managed encryption key (CMEK).
	ForceEncryptionWithCmek bool `json:"force_encryption_with_cmek"`

	// Id The unique ID of the project.
	Id uuid.UUID `json:"id"`

	// MaxPods The maximum number of Pods that can be created in the project.
	MaxPods int `json:"max_pods"`

	// Name The name of the project.
	Name string `json:"name"`

	// OrganizationId The unique ID of the organization that the project belongs to.
	OrganizationId string `json:"organization_id"`
}

func (a *AdminClient) ListProjects(ctx context.Context) ([]*Project, error) {
	res, err := a.restClient.ListProjects(ctx)
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
			projects[i] = &Project{
				CreatedAt:               project.CreatedAt,
				ForceEncryptionWithCmek: project.ForceEncryptionWithCmek,
				Id:                      project.Id,
				MaxPods:                 project.MaxPods,
				Name:                    project.Name,
				OrganizationId:          project.OrganizationId,
			}
		}
	} else {
		projects = make([]*Project, 0)
	}

	return projects, nil
}

// Organization The details of an organization.
type Organization struct {
	// CreatedAt The date and time when the organization was created.
	CreatedAt time.Time `json:"created_at"`

	// Id The unique ID of the organization.
	Id uuid.UUID `json:"id"`

	// Name The name of the organization.
	Name string `json:"name"`

	// PaymentStatus The current payment status of the organization.
	PaymentStatus string `json:"payment_status"`

	// Plan The current plan the organization is on.
	Plan string `json:"plan"`

	// SupportTier The support tier of the organization.
	SupportTier string `json:"support_tier"`
}

func (a *AdminClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	res, err := a.restClient.ListOrganizations(ctx)
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
			organizations[i] = &Organization{
				CreatedAt:     org.CreatedAt,
				Id:            org.Id,
				Name:          org.Name,
				PaymentStatus: org.PaymentStatus,
				Plan:          org.Plan,
				SupportTier:   org.SupportTier,
			}
		}
	} else {
		organizations = make([]*Organization, 0)
	}

	return organizations, nil
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

	operationPath := "/oauth2/token"
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
		"audience":      "https://api.pinecone.io",
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
	if in.SourceTag == nil {
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
