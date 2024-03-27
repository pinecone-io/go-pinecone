package pinecone

import (
	"context"
	"fmt"
	"github.com/deepmap/oapi-codegen/v2/pkg/securityprovider"
	"github.com/google/uuid"
	"github.com/pinecone-io/go-pinecone/internal/gen/management"
	"net/http"
)

// ManagementClient provides a high-level interface for interacting with the
// Pinecone management plane API. It encapsulates the necessary authentication,
// request creation, and response handling for the API's operations.
//
// The ManagementClient is designed to simplify the management of projects and
// their API keys by abstracting away the direct handling of HTTP requests and
// responses. It leverages a generated low-level client (`restClient`) for
// communication with the API, ensuring that requests are properly authenticated
// and formatted according to the API's specification.
//
// Fields:
//   - apiKey: The API key used for authenticating requests to the management API.
//     This key should have the necessary permissions for the operations you intend to perform.
//   - restClient: An instance of the generated low-level client that actually performs
//     HTTP requests to the management API. This field is internal and managed
//     by the ManagementClient.
//
// To use ManagementClient, first instantiate it using the NewManagementClient function,
// providing it with the necessary configuration. Once instantiated, you can call its
// methods to perform actions such as listing, creating, getting, and deleting projects
// and project API keys.
//
// Example:
//
//	clientParams := NewManagementClientParams{ApiKey: "your_api_key_here"}
//	managementClient, err := NewManagementClient(clientParams)
//	if err != nil {
//	    log.Fatalf("Failed to create management client: %v", err)
//	}
//	// Now you can use managementClient to interact with the API
//
// Note that ManagementClient methods are designed to be safe for concurrent use by multiple
// goroutines, assuming that its configuration (e.g., the API key) is not modified after
// initialization.
type ManagementClient struct {
	apiKey     string
	restClient *management.ClientWithResponses
}

// NewManagementClientParams holds the parameters for creating a new ManagementClient.
type NewManagementClientParams struct {
	ApiKey string
}

// NewManagementClient creates and initializes a new instance of ManagementClient.
// This method sets up the management plane client with the necessary configuration for
// authentication and communication with the management API.
//
// The method requires an input parameter of type NewManagementClientParams, which includes:
// - ApiKey: A string representing the API key used for authentication against the management API.
//
// The API key is used to configure the underlying HTTP client with the appropriate
// authentication headers for all requests made to the management API.
//
// Returns a pointer to an initialized ManagementClient instance on success. In case of
// failure, it returns nil and an error describing the issue encountered. Possible errors
// include issues with setting up the API key provider or problems initializing the
// underlying REST client.
//
// Example:
//
//	clientParams := NewManagementClientParams{
//	    ApiKey: "your_api_key_here",
//	}
//	managementClient, err := NewManagementClient(clientParams)
//	if err != nil {
//	    log.Fatalf("Failed to create management client: %v", err)
//	}
//	// Use managementClient to interact with the management API
//
// It is important to handle the error returned by this method to ensure that the
// management client has been created successfully before attempting to make API calls.
func NewManagementClient(in NewManagementClientParams) (*ManagementClient, error) {
	apiKeyProvider, err := securityprovider.NewSecurityProviderApiKey("header", "Api-Key", in.ApiKey)
	if err != nil {
		return nil, err
	}

	client, err := management.NewClientWithResponses("https://api.pinecone.io/management/v1alpha", management.WithRequestEditorFn(apiKeyProvider.Intercept))
	if err != nil {
		return nil, err
	}

	c := ManagementClient{apiKey: in.ApiKey, restClient: client}
	return &c, nil
}

// ListProjects retrieves all projects from the management API. It makes a call to the
// management plane's ListProjects endpoint and returns a slice of Project pointers.
//
// The method handles various HTTP response codes from the API, including:
// - 401 Unauthorized: Returned if the API key is invalid or missing.
// - 500 Internal Server Error: Indicates a server-side error. It might be temporary.
// - 4XX: Covers other client-side errors not explicitly handled by other conditions.
//
// In case of a successful response but with unexpected format or empty data,
// it returns an error indicating the unexpected response format.
//
// Context (ctx) is used to control the request's lifetime. It allows for the request
// to be canceled or to timeout according to the context's deadline.
//
// Returns a slice of pointers to Project structs populated with the project data
// on success. In case of failure, it returns an error describing the issue encountered.
// It's important to check the returned error to understand the outcome of the request.
//
// Example:
//
//	projects, err := managementClient.ListProjects(ctx)
//	if err != nil {
//	    log.Fatalf("Failed to list projects: %v", err)
//	}
//	for _, project := range projects {
//	    fmt.Printf("Project ID: %s, Name: %s\n", project.Id, project.Name)
//	}
func (c *ManagementClient) ListProjects(ctx context.Context) ([]*Project, error) {
	res, err := c.restClient.ListProjectsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	if res.JSON401 != nil {
		return nil, fmt.Errorf("unauthorized: %s", res.JSON401.Error.Message)
	}

	if res.JSON500 != nil {
		return nil, fmt.Errorf("internal server error: %s", res.JSON500.Error.Message)
	}

	if res.JSON4XX != nil {
		return nil, fmt.Errorf("unknown error: %s", res.JSON4XX.Error.Message)
	}

	if res.JSON200 == nil || res.JSON200.Data == nil {
		return nil, fmt.Errorf("unexpected response format or empty data")
	}

	projectList := make([]*Project, len(*res.JSON200.Data))
	for i, p := range *res.JSON200.Data {
		project := Project{
			Id:   p.Id,
			Name: p.Name,
		}
		projectList[i] = &project
	}
	return projectList, nil
}

// FetchProject retrieves a project by its ID from the management API. It makes a call to the
// management plane's FetchProject endpoint and returns the project details.
//
// Parameters:
// - ctx: A context.Context to control the request's lifetime.
// - projectId: A string representing the unique identifier of the project to retrieve.
//
// Returns the project details on success or an error if the operation fails. Possible errors
// include unauthorized access, project not found, internal server errors, or other HTTP client
// errors.
//
// Example:
//
//	project, err := managementClient.FetchProject(ctx, "your_project_id_here")
//	if err != nil {
//	    log.Fatalf("Failed to fetch project: %v", err)
//	}
//	fmt.Printf("Project ID: %s, Name: %s\n", project.Id, project.Name)
func (c *ManagementClient) FetchProject(ctx context.Context, projectId uuid.UUID) (*Project, error) {
	resp, err := c.restClient.FetchProjectWithResponse(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		if resp.JSON200 != nil {
			return &Project{
				Id:   resp.JSON200.Id,
				Name: resp.JSON200.Name,
			}, nil
		}
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized: %v", resp.JSON401)
	case http.StatusNotFound:
		return nil, fmt.Errorf("project not found: %v", resp.JSON404)
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("internal server error: %v", resp.JSON500)
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return nil, fmt.Errorf("unexpected response format or empty data")
}

// CreateProject creates a new project in the management API. It sends a request to the
// management plane's CreateProject endpoint with the project details.
//
// Parameters:
// - ctx: A context.Context to control the request's lifetime.
// - projectName: A string representing the name of the project to create.
//
// Returns the created project's details on success or an error if the creation fails.
// Possible errors include unauthorized access, validation errors, internal server errors,
// or other HTTP client errors.
//
// Example:
//
//	project, err := managementClient.CreateProject(ctx, "New Project Name")
//	if err != nil {
//	    log.Fatalf("Failed to create project: %v", err)
//	}
//	fmt.Printf("Created Project ID: %s, Name: %s\n", project.Id, project.Name)
func (c *ManagementClient) CreateProject(ctx context.Context, projectName string) (*Project, error) {
	body := management.CreateProjectJSONRequestBody{
		Name: projectName,
	}

	resp, err := c.restClient.CreateProjectWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusCreated:
		if resp.JSON201 != nil {
			return &Project{
				Id:   resp.JSON201.Id,
				Name: resp.JSON201.Name,
			}, nil
		}
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized: %v", resp.JSON401)
	case http.StatusBadRequest:
		return nil, fmt.Errorf("bad request: %v", resp.JSON400)
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return nil, fmt.Errorf("unexpected response format or empty data")
}

// DeleteProject deletes a project by its ID from the management API. It makes a call to the
// management plane's DeleteProject endpoint.
//
// Parameters:
// - ctx: A context.Context to control the request's lifetime.
// - projectId: A string representing the unique identifier of the project to delete.
//
// Returns an error if the deletion fails. Possible errors include unauthorized access,
// project not found, internal server errors, or other HTTP client errors.
//
// Example:
//
//	err := managementClient.DeleteProject(ctx, "your_project_id_here")
//	if err != nil {
//	    log.Fatalf("Failed to delete project: %v", err)
//	}
func (c *ManagementClient) DeleteProject(ctx context.Context, projectId uuid.UUID) error {
	resp, err := c.restClient.DeleteProjectWithResponse(ctx, projectId)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK, http.StatusAccepted, http.StatusNoContent:
		return nil // Success case
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: %v", resp.JSON401)
	case http.StatusNotFound:
		return fmt.Errorf("project not found: %v", resp.JSON404)
	case http.StatusInternalServerError:
		return fmt.Errorf("internal server error: %v", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
}
