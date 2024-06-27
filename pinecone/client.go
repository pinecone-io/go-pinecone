package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pinecone-io/go-pinecone/internal/gen/control"
	"github.com/pinecone-io/go-pinecone/internal/provider"
	"github.com/pinecone-io/go-pinecone/internal/useragent"
)


// Client holds the parameters for connecting to the Pinecone service. It is returned by the NewClient and NewClientBase
// functions. To use Client, first build the parameters of the request using NewClientParams (or NewClientBaseParams).
// Then, pass those parameters into the NewClient (or NewBaseClient) function to create a new Client object.
// Once instantiated, you can use Client to execute control plane API requests (e.g. create an index, list indexes,
// etc.). Read more about different control plane API routes at [pinecone.io/docs].
//
// Note: Client methods are safe for concurrent use.
//
// Fields:
//   - headers: An optional map of additional HTTP headers to include in each API request to the control plane,
//    Provided through NewClientParams.Headers or NewClientBaseParams.Headers.
//   - restClient: The underlying REST client (managed internally) used to communicate with the Pinecone control plane API.
//   - sourceTag: An optional string used to help Pinecone attribute API activity, provided through NewClientParams.SourceTag
//  or NewClientBaseParams.SourceTag.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams) // --> This creates a new Client object.
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.DescribeIndex(ctx, "your-index-name")
//  if err != nil {
//	  log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//  }
//
// 	idxConnection, err := pc.Index(idx.Host)
//	if err != nil {
//	  log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	}
//
// [pinecone.io/docs]: https://docs.pinecone.io/reference/api/control-plane/list_indexes
type Client struct {
	headers    map[string]string
	restClient *control.Client
	sourceTag  string
}

// NewClientParams holds the parameters for creating a new Client while authenticating with an API key.
//
// Fields:
//   - ApiKey: The API key used to authenticate with the Pinecone control plane API.
//   - Headers: An optional map of additional HTTP headers to include in each API request to the control plane.
//   - Host: The host URL of the Pinecone control plane API. If not provided,
//   the default value is "https://api.pinecone.io".
//   - RestClient: An optional HTTP client to use for communication with the control plane API.
//   - SourceTag: An optional string used to help Pinecone attribute API activity.
//
// See Client for code example.
type NewClientParams struct {
	ApiKey     string            // required - provide through NewClientParams or environment variable PINECONE_API_KEY
	Headers    map[string]string // optional
	Host       string            // optional
	RestClient *http.Client      // optional
	SourceTag  string            // optional
}

// NewClientBaseParams holds the parameters for creating a new Client while passing custom authentication headers.
//
// Fields:
//   - Headers: An optional map of additional HTTP headers to include in each API request to the control plane,
//   provided through NewClientParams.Headers or NewClientBaseParams.Headers.
//   "Authorization" and "X-Project-Id" headers are required if authenticating using a JWT.
//   - Host: The host URL of the Pinecone control plane API. If not provided,
//   the default value is "https://api.pinecone.io".
//   - RestClient: An optional HTTP client to use for communication with the control plane API.
//   - SourceTag: An optional string used to help Pinecone attribute API activity.
//
// See Client for code example.
type NewClientBaseParams struct {
	Headers    map[string]string
	Host       string
	RestClient *http.Client
	SourceTag  string
}

// NewClient creates and initializes a new instance of Client.
// This function sets up the control plane client with the necessary configuration for authentication and communication
// with the control plane API.
//
// This function requires an input parameter of type NewClientParams, which includes:
//   - ApiKey: The API key used to authenticate with the Pinecone control plane API.
//   - Headers: An optional map of additional HTTP headers to include in each API request to the control plane.
//   Provided through NewClientParams.Headers or NewClientBaseParams.Headers.
//   - Host: The host URL of the Pinecone control plane API. If not provided,
//     the default value is "https://api.pinecone.io".
//   - RestClient: An optional custom HTTP client to use for communication with the control plane API.
//   - SourceTag: An optional string used to help Pinecone attribute API activity to our partners.
//
// Note: It is important to handle the error returned by this function to ensure that the
// control plane client has been created successfully before attempting to make API calls.
//
// Returns a pointer to an initialized Client instance on success. In case of
// failure, it returns nil and an error describing the issue encountered. Possible errors
// include issues with setting up the API key provider or problems initializing the
// underlying REST client.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
func NewClient(in NewClientParams) (*Client, error) {
	osApiKey := os.Getenv("PINECONE_API_KEY")
	hasApiKey := (valueOrFallback(in.ApiKey, osApiKey) != "")

	if !hasApiKey {
		return nil, fmt.Errorf("no API key provided, please pass an API key for authorization through NewClientParams or set the PINECONE_API_KEY environment variable")
	}

	apiKeyHeader := struct{ Key, Value string }{"Api-Key", valueOrFallback(in.ApiKey, osApiKey)}

	clientHeaders := in.Headers
	if clientHeaders == nil {
		clientHeaders = make(map[string]string)
		clientHeaders[apiKeyHeader.Key] = apiKeyHeader.Value

	} else {
		clientHeaders[apiKeyHeader.Key] = apiKeyHeader.Value
	}

	return NewClientBase(NewClientBaseParams{Headers: clientHeaders, Host: in.Host, RestClient: in.RestClient, SourceTag: in.SourceTag})
}

// NewClientBase creates and initializes a new instance of Client with custom authentication headers,
// allowing users to authenticate in ways other than passing an API key.
//
// This function requires an input parameter of type NewClientBaseParams, which includes:
//   - Headers: An optional map of additional HTTP headers to include in each API request to the control plane.
//    Provided through NewClientParams.Headers or NewClientBaseParams.Headers.
//    "Authorization" and "X-Project-Id" headers are required.
//   - Host: The host URL of the Pinecone control plane API. If not provided,
//     the default value is "https://api.pinecone.io".
//   - RestClient: An optional custom HTTP client to use for communication with the control plane API.
//   - SourceTag: An optional string used to help Pinecone attribute API activity to our partners.
//
// Note: It is important to handle the error returned by this function to ensure that the
// control plane client has been created successfully before attempting to make API calls.
//
// Returns a pointer to an initialized Client instance on success. In case of
// failure, it returns nil and an error describing the issue encountered. Possible errors
// include issues with setting up the headers or problems initializing the
// underlying REST client.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientBaseParams{
//    Headers: map[string]string{
//     "Authorization": "Bearer " + "<your JWT token>"
//     "X-Project-Id": "<Your Pinecone project ID>"
//    },
//    SourceTag: "your_source_identifier", // optional
//  }
//
//  pc, err := pinecone.NewClientBase(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
func NewClientBase(in NewClientBaseParams) (*Client, error) {
	clientOptions := buildClientBaseOptions(in)
	var err error

	controlHostOverride := valueOrFallback(in.Host, os.Getenv("PINECONE_CONTROLLER_HOST"))
	if controlHostOverride != "" {
		controlHostOverride, err = ensureURLScheme(controlHostOverride)
		if err != nil {
			return nil, err
		}
	}

	client, err := control.NewClient(valueOrFallback(controlHostOverride, "https://api.pinecone.io"), clientOptions...)
	if err != nil {
		return nil, err
	}

	c := Client{restClient: client, sourceTag: in.SourceTag, headers: in.Headers}
	return &c, nil
}

// Index creates an IndexConnection to the specified host.
//
// Parameters:
//   - host: The host URL of your Pinecone index.
//
// Returns a pointer to an IndexConnection instance on success. In case of failure, it returns nil and an error.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.DescribeIndex(ctx, "your-index-name")
//  if err != nil {
//	  fmt.Printf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//  }
//
// 	idxConnection, err := pc.Index(idx.Host)
//	if err != nil {
//	  fmt.Println("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	}
 func (c *Client) Index(host string) (*IndexConnection, error) {
	return c.IndexWithAdditionalMetadata(host, "", nil)
}

// IndexWithNamespace creates an IndexConnection to the specified host within the specified namespace.
//
// Parameters:
//   - host: The host URL of your Pinecone index.
//   - namespace: The target namespace.
//
// Returns a pointer to an IndexConnection instance on success. In case of failure, it returns nil and an error.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.DescribeIndex(ctx, "your-index-name")
//  if err != nil {
//	  log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//  }
//
// 	idxConnection, err := pc.IndexWithNamespace(idx.Host, "custom-namespace")
//	if err != nil {
//	  log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	}
func (c *Client) IndexWithNamespace(host string, namespace string) (*IndexConnection, error) {
	return c.IndexWithAdditionalMetadata(host, namespace, nil)
}

// IndexWithAdditionalMetadata creates an IndexConnection to the specified host within the specified namespace,
// with the addition of custom metadata.
//
// Parameters:
//   - host: The host URL of your Pinecone index.
//   - namespace: The target namespace.
//   - additionalMetadata: A map of additional metadata fields to include in the API request.
//
// Returns a pointer to an IndexConnection instance on success. In case of failure,
// it returns nil and the error encountered.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.DescribeIndex(ctx, "your-index-name")
//  if err != nil {
//	  log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//  }
//
//  indexMetadata := map[string]string{
//	  "indexMetadata": "custom-index-level-metadata",
//	}
//
//  idxConnection, err := pc.IndexWithAdditionalMetadata(idx.Host, "custom-namespace", indexMetadata)
//	if err != nil {
//	  log.Fatalf("Failed to create IndexConnection: %v", err)
//	} else {
//    fmt.Printf("IndexConnection created successfully for index: %s , with namespace \"%s\"\n", idx.Name,
//    idxConnection.Namespace)
//	}
func (c *Client) IndexWithAdditionalMetadata(host string, namespace string, additionalMetadata map[string]string) (*IndexConnection, error) {
	authHeader := c.extractAuthHeader()

	// merge additionalMetadata with authHeader
	if additionalMetadata != nil {
		for _, key := range authHeader {
			additionalMetadata[key] = authHeader[key]
		}
	} else {
		additionalMetadata = authHeader
	}

	idx, err := newIndexConnection(newIndexParameters{host: host, namespace: namespace, sourceTag: c.sourceTag, additionalMetadata: additionalMetadata})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

// ListIndexes retrieves a list of all indexes in a Pinecone project.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//   to be canceled or to timeout according to the context's deadline.
//
// Returns a slice of pointers to Index objects on success. In case of failure,
// it returns nil and the error encountered.
//
// Example:
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idxs, err := pc.ListIndexes(ctx)
//	if err != nil {
//	  log.Fatalf("Failed to list indexes:", err)
//  } else {
//	  fmt.Println("Your project has the following indexes:")
//	  for _, idx := range idxs {
//	    fmt.Printf("- \"%s\"\n", idx.Name)
//	  }
//  }
func (c *Client) ListIndexes(ctx context.Context) ([]*Index, error) {
	res, err := c.restClient.ListIndexes(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list indexes: ")
	}

	var indexList control.IndexList
	err = json.NewDecoder(res.Body).Decode(&indexList)
	if err != nil {
		return nil, err
	}

	indexes := make([]*Index, len(*indexList.Indexes))
	for i, idx := range *indexList.Indexes {
		indexes[i] = toIndex(&idx)
	}

	return indexes, nil
}

// CreatePodIndexRequest holds the parameters for creating a new Pods-based index.
//
// Fields:
//   - Name: The name of the index.
//   - Dimension: The dimension of the index.
//   - Metric: The metric used to measure the similarity between vectors ("Cosine", "Euclidean", or "Dotproduct").
//   - Environment: The cloud environment in which the index will be created.
//   - PodType: The type of pod to use for the index ("p1", "p2", or "s2").
//   - Shards: The number of shards to use for the index (defaults to 1).
//   - Replicas: The number of replicas to use for the index (defaults to 1).
//   - SourceCollection: The Collection from which to create the index.
//   - MetadataConfig: The metadata configuration for the index.
//
// To create a new pods-based index, use the CreatePodIndex method on the Client object.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.CreatePodIndex(ctx, &pinecone.CreatePodIndexRequest{
//    Name:        "my-pod-index",
//    Dimension:   3,
//    Metric:      pinecone.Cosine,
//    Environment: "us-west1-gcp",
//    PodType:     "s1",
//    },
//  )
//
//  if err != nil {
//	  log.Fatalf("Failed to create pod index:", err)
//	} else {
//	  fmt.Printf("Successfully created pod index: %s", idx.Name)
//  }
type CreatePodIndexRequest struct {
	Name             string
	Dimension        int32
	Metric           IndexMetric
	Environment      string
	PodType          string
	Shards           int32
	Replicas         int32
	SourceCollection *string
	MetadataConfig   *PodSpecMetadataConfig
}

// ReplicaCount ensures the replica count is >1 and returns a pointer to the number of replicas on the
// CreatePodIndexRequest object.
func (req CreatePodIndexRequest) ReplicaCount() *int32 {
	x := minOne(req.Replicas)
	return &x
}

// ShardCount ensures the number of shards is >1 and returns a pointer to the number of shards on the
// CreatePodIndexRequest object.
func (req CreatePodIndexRequest) ShardCount() *int32 {
	x := minOne(req.Shards)
	return &x
}

// TotalCount calculates and returns the total number of pods (replicas*shards) on the CreatePodIndexRequest object.
func (req CreatePodIndexRequest) TotalCount() *int {
	x := int(*req.ReplicaCount() * *req.ShardCount())
	return &x
}

// CreatePodIndex creates and initializes a new pods-based index via the specified Client.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - in: A pointer to a CreatePodIndexRequest object.
//
// Returns a pointer to an Index object. In case of failure, it returns nil and the error encountered.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.CreatePodIndex(ctx, &pinecone.CreatePodIndexRequest{
//    Name:        "my-pod-index",
//    Dimension:   3,
//    Metric:      pinecone.Cosine,
//    Environment: "us-west1-gcp",
//    PodType:     "s1",
//    }
//  )
//
// 	if err != nil {
//	  log.Fatalf("Failed to create pod index:", err)
//	} else {
//	  fmt.Printf("Successfully created pod index: %s", idx.Name)
//	}
func (c *Client) CreatePodIndex(ctx context.Context, in *CreatePodIndexRequest) (*Index, error) {
	metric := control.IndexMetric(in.Metric)
	req := control.CreateIndexRequest{
		Name:      in.Name,
		Dimension: in.Dimension,
		Metric:    &metric,
	}

	//add the spec to req.
	//because this is defined as an anon struct in the generated code, it must match exactly here.
	req.Spec = control.CreateIndexRequest_Spec{
		Pod: &struct {
			Environment    string `json:"environment"`
			MetadataConfig *struct {
				Indexed *[]string `json:"indexed,omitempty"`
			} `json:"metadata_config,omitempty"`
			PodType          control.PodSpecPodType   `json:"pod_type"`
			Pods             *int                     `json:"pods,omitempty"`
			Replicas         *control.PodSpecReplicas `json:"replicas,omitempty"`
			Shards           *control.PodSpecShards   `json:"shards,omitempty"`
			SourceCollection *string                  `json:"source_collection,omitempty"`
		}{
			Environment:      in.Environment,
			PodType:          in.PodType,
			Pods:             in.TotalCount(),
			Replicas:         in.ReplicaCount(),
			Shards:           in.ShardCount(),
			SourceCollection: in.SourceCollection,
		},
	}
	if in.MetadataConfig != nil {
		req.Spec.Pod.MetadataConfig = &struct {
			Indexed *[]string `json:"indexed,omitempty"`
		}{
			Indexed: in.MetadataConfig.Indexed,
		}
	}

	res, err := c.restClient.CreateIndex(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, handleErrorResponseBody(res, "failed to create index: ")
	}

	return decodeIndex(res.Body)
}

// CreateServerlessIndexRequest holds the parameters for creating a new Serverless index.
//
// Fields:
//   - Name: The name of the index.
//   - Dimension: The dimension of the index.
//   - Metric: The metric used to measure the similarity between vectors ("Cosine", "Euclidean", or "Dotproduct").
//   - Cloud: The cloud provider in which the index will be created.
//   - Region: The region in which the index will be created.
//
// To create a new Serverless index, use the CreateServerlessIndex method on the Client object.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//    Name:    "my-serverless-index",
//    Dimension: 3,
//    Metric:  pinecone.Cosine,
//    Cloud:   pinecone.Aws,
//    Region:  "us-east-1",
//    }
//  )
//
//  if err != nil {
//    log.Fatalf("Failed to create serverless index: %s", idx.Name)
//  } else {
//    fmt.Printf("Successfully created serverless index: %s", idx.Name)
// }
type CreateServerlessIndexRequest struct {
	Name      string
	Dimension int32
	Metric    IndexMetric
	Cloud     Cloud
	Region    string
}

// CreateServerlessIndex creates and initializes a new serverless index via the specified Client.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - in: A pointer to a CreateServerlessIndexRequest object.
//
// Returns a pointer to an Index object. In case of failure, it returns nil and the error encountered.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//    Name:    "my-serverless-index",
//    Dimension: 3,
//    Metric:  pinecone.Cosine,
//    Cloud:   pinecone.Aws,
//    Region:  "us-east-1",
//  }
//)
//
//  if err != nil {
//    log.Fatalf("Failed to create serverless index: %s", idx.Name)
//  } else {
//    fmt.Printf("Successfully created serverless index: %s", idx.Name)
// }
func (c *Client) CreateServerlessIndex(ctx context.Context, in *CreateServerlessIndexRequest) (*Index, error) {
	metric := control.IndexMetric(in.Metric)
	req := control.CreateIndexRequest{
		Name:      in.Name,
		Dimension: in.Dimension,
		Metric:    &metric,
		Spec: control.CreateIndexRequest_Spec{
			Serverless: &control.ServerlessSpec{
				Cloud:  control.ServerlessSpecCloud(in.Cloud),
				Region: in.Region,
			},
		},
	}

	res, err := c.restClient.CreateIndex(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, handleErrorResponseBody(res, "failed to create index: ")
	}

	return decodeIndex(res.Body)
}

// DescribeIndex retrieves information about a specific index.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - idxName: The name of the index to describe.
//
// Returns a pointer to an Index object. In case of failure, it returns nil and the error encountered.
//
// Since the returned value is a pointer to an Index object, it will have the following fields:
//   - Name: The name of the index.
//   - Dimension: The dimension of the index.
//   - Host: The host URL of the index.
//   - Metric: The metric used to measure the similarity between vectors.
//   - Spec: The specification of the index.
//   - Status: The status of the index.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  idx, err := pc.DescribeIndex(ctx, "the-name-of-my-index")
//  if err != nil {
//    log.Fatalf("Failed to describe index: %s", err)
//  } else {
//    fmt.Printf("%+v", *idx)
// }
func (c *Client) DescribeIndex(ctx context.Context, idxName string) (*Index, error) {
	res, err := c.restClient.DescribeIndex(ctx, idxName)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe index: ")
	}

	return decodeIndex(res.Body)
}

// DeleteIndex deletes a specific index.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - idxName: The name of the index to delete.
//
// Returns an error if the deletion fails.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
// 	err = pc.DeleteIndex(ctx, "the-name-of-my-index")
//	if err != nil {
//	  fmt.Println("Error:", err)
//	} else {
//	  fmt.Printf("Index \"%s\" deleted successfully", idx.Name)
//	}
func (c *Client) DeleteIndex(ctx context.Context, idxName string) error {
	res, err := c.restClient.DeleteIndex(ctx, idxName)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete index: ")
	}

	return nil
}

// ListCollections retrieves a list of all collections in a Pinecone project.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//
// Returns a slice of pointers to Collection objects on success. In case of failure,
// it returns nil and the error encountered.
//
// Note: Collections are only available for pods-based indexes.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
// pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  collections, err := pc.ListCollections(ctx)
//	if err != nil {
//	  fmt.Println("Failed to list collections:", err)
//	} else {
//	  if len(collections) == 0 {
//	    fmt.Printf("No collections found in project")
//	  } else {
//	    fmt.Println("Collections in project:")
//		for _, collection := range collections {
//		  fmt.Printf("Collection: %s\n", collection.Name)
//		}
//	  }
//	}
func (c *Client) ListCollections(ctx context.Context) ([]*Collection, error) {
	res, err := c.restClient.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list collections: ")
	}

	var collectionsResponse control.CollectionList
	if err := json.NewDecoder(res.Body).Decode(&collectionsResponse); err != nil {
		return nil, err
	}

	var collections []*Collection
	for _, collectionModel := range *collectionsResponse.Collections {
		collections = append(collections, toCollection(&collectionModel))
	}

	return collections, nil
}

// DescribeCollection retrieves information about a specific collection.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - collectionName: The name of the collection to describe.
//
// Returns a pointer to a Collection object. In case of failure, it returns nil and the error encountered.
//
// Note: Collections are only available for pods-based indexes.
//
// Since the returned value is a pointer to a Collection object, it will have the following fields:
//   - Name: The name of the collection.
//   - Size: The size of the collection.
//   - Status: The status of the collection.
//   - Dimension: The dimension of the collection.
//   - VectorCount: The number of vectors in the collection.
//   - Environment: The cloud environment in which the collection resides.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
//  pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  collection, err := pc.DescribeCollection(ctx, "my-collection")
//  if err != nil {
//	  fmt.Println("Error describing collection:", err)
//	} else {
//	  fmt.Printf("Collection: %+v\n", *collection)
//	}
func (c *Client) DescribeCollection(ctx context.Context, collectionName string) (*Collection, error) {
	res, err := c.restClient.DescribeCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe collection: ")
	}

	return decodeCollection(res.Body)
}

// CreateCollectionRequest holds the parameters for creating a new collection.
//
// Fields:
//   - Name: The name of the collection.
//   - Source: The source index from which the collection will be made.
//
// To create a new collection, use the CreateCollection method on the Client object.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
// pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  collection, err := pc.CreateCollection(ctx, &pinecone.CreateCollectionRequest{
//    Name:   "my-collection",
//    Source: "my-source-pods-based-index",
//    },
//  )
//  if err != nil {
//	  log.Fatalf("Failed to create collection: %s", err)
//	} else {
//	  fmt.Printf("Successfully created collection \"%s\".", collection.Name)
//	}
type CreateCollectionRequest struct {
	Name   string
	Source string
}

// CreateCollection creates and initializes a new collection via the specified Client.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - in: A pointer to a CreateCollectionRequest object.
//
// Returns a pointer to a Collection object. In case of failure, it returns nil and the error encountered.
//
// Note: Collections are only available for pods-based indexes.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
// pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  collection, err := pc.CreateCollection(ctx, &pinecone.CreateCollectionRequest{
//    Name:   "my-collection",
//    Source: "my-source-pods-based-index",
//    }
//  )
// 	if err != nil {
//	  log.Fatalf("Failed to create collection: %s", err)
//	} else {
//	  fmt.Printf("Successfully created collection \"%s\".", collection.Name)
//	}
func (c *Client) CreateCollection(ctx context.Context, in *CreateCollectionRequest) (*Collection, error) {
	req := control.CreateCollectionRequest{
		Name:   in.Name,
		Source: in.Source,
	}
	res, err := c.restClient.CreateCollection(ctx, req)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, handleErrorResponseBody(res, "failed to create collection: ")
	}

	return decodeCollection(res.Body)
}

// DeleteCollection deletes a specific collection.
//
// Parameters:
// 	- ctx: A context.Context object controls the request's lifetime, allowing for the request
//  to be canceled or to timeout according to the context's deadline.
//  - collectionName: The name of the collection to delete.
//
// Returns an error if the deletion fails.
//
// Note: Collections are only available for pods-based indexes.
//
// Example:
//  ctx := context.Background()
//
//  clientParams := pinecone.NewClientParams{
//	  ApiKey:    "YOUR_API_KEY",
//	  SourceTag: "your_source_identifier", // optional
//	}
//
// pc, err := pinecone.NewClient(clientParams)
//  if err != nil {
//    log.Fatalf("Failed to create Client: %v", err)
//  }
//
//  collectionName := "my-collection"
//
//  err = pc.DeleteCollection(ctx, collectionName)
//	if err != nil {
//	  log.Fatalf("Failed to create collection: %s\n", err)
//	} else {
//	  log.Printf("Successfully deleted collection \"%s\"\n", collectionName)
//	}
func (c *Client) DeleteCollection(ctx context.Context, collectionName string) error {
	res, err := c.restClient.DeleteCollection(ctx, collectionName)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete collection: ")
	}

	return nil
}

func (c *Client) extractAuthHeader() map[string]string {
	possibleAuthKeys := []string{
		"api-key",
		"authorization",
		"access_token",
	}

	for key, value := range c.headers {
		for _, checkKey := range possibleAuthKeys {
			if strings.ToLower(key) == checkKey {
				//fmt.Println("!! key in extractAuthHeader here", key)
				//fmt.Println("!! Return value from extractAuthHeader looks like this: ", map[string]string{key: value})
				return map[string]string{key: value}
			}
		}
	}

	return nil
}

func toIndex(idx *control.IndexModel) *Index {
	if idx == nil {
		return nil
	}

	spec := &IndexSpec{}
	if idx.Spec.Pod != nil {
		spec.Pod = &PodSpec{
			Environment:      idx.Spec.Pod.Environment,
			PodType:          idx.Spec.Pod.PodType,
			PodCount:         int32(idx.Spec.Pod.Pods),
			Replicas:         idx.Spec.Pod.Replicas,
			ShardCount:       idx.Spec.Pod.Shards,
			SourceCollection: idx.Spec.Pod.SourceCollection,
		}
		if idx.Spec.Pod.MetadataConfig != nil {
			spec.Pod.MetadataConfig = &PodSpecMetadataConfig{Indexed: idx.Spec.Pod.MetadataConfig.Indexed}
		}
	}
	if idx.Spec.Serverless != nil {
		spec.Serverless = &ServerlessSpec{
			Cloud:  Cloud(idx.Spec.Serverless.Cloud),
			Region: idx.Spec.Serverless.Region,
		}
	}
	status := &IndexStatus{
		Ready: idx.Status.Ready,
		State: IndexStatusState(idx.Status.State),
	}
	return &Index{
		Name:      idx.Name,
		Dimension: idx.Dimension,
		Host:      idx.Host,
		Metric:    IndexMetric(idx.Metric),
		Spec:      spec,
		Status:    status,
	}
}

func decodeIndex(resBody io.ReadCloser) (*Index, error) {
	var idx control.IndexModel
	err := json.NewDecoder(resBody).Decode(&idx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode idx response: %w", err)
	}

	return toIndex(&idx), nil
}

func toCollection(cm *control.CollectionModel) *Collection {
	if cm == nil {
		return nil
	}

	return &Collection{
		Name:        cm.Name,
		Size:        derefOrDefault(cm.Size, 0),
		Status:      CollectionStatus(cm.Status),
		Dimension:   derefOrDefault(cm.Dimension, 0),
		VectorCount: derefOrDefault(cm.VectorCount, 0),
		Environment: cm.Environment,
	}
}

func decodeCollection(resBody io.ReadCloser) (*Collection, error) {
	var collectionModel control.CollectionModel
	err := json.NewDecoder(resBody).Decode(&collectionModel)
	if err != nil {
		return nil, fmt.Errorf("failed to decode collection response: %w", err)
	}

	return toCollection(&collectionModel), nil
}

func decodeErrorResponse(resBodyBytes []byte) (*control.ErrorResponse, error) {
	var errorResponse control.ErrorResponse
	err := json.Unmarshal(resBodyBytes, &errorResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode error response: %w", err)
	}

	if errorResponse.Status == 0 {
		return nil, fmt.Errorf("unable to parse ErrorResponse: %v", string(resBodyBytes))
	}

	return &errorResponse, nil
}

type errorResponseMap struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
	Message    string `json:"message,omitempty"`
	Details    string `json:"details,omitempty"`
}

func handleErrorResponseBody(response *http.Response, errMsgPrefix string) error {
	resBodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var errMap errorResponseMap
	errMap.StatusCode = response.StatusCode

	// try and decode ErrorResponse
	if json.Valid(resBodyBytes) {
		errorResponse, err := decodeErrorResponse(resBodyBytes)
		if err == nil {
			errMap.Message = errorResponse.Error.Message
			errMap.ErrorCode = string(errorResponse.Error.Code)

			if errorResponse.Error.Details != nil {
				errMap.Details = fmt.Sprintf("%+v", errorResponse.Error.Details)
			}
		}
	}

	errMap.Body = string(resBodyBytes)

	if errMap.Message != "" {
		errMap.Message = errMsgPrefix + errMap.Message
	}

	return formatError(errMap)
}

func formatError(errMap errorResponseMap) error {
	jsonString, err := json.Marshal(errMap)
	if err != nil {
		return err
	}
	baseError := fmt.Errorf(string(jsonString))

	return &PineconeError{Code: errMap.StatusCode, Msg: baseError}
}

func buildClientBaseOptions(in NewClientBaseParams) []control.ClientOption {
	clientOptions := []control.ClientOption{}

	// build and apply user agent
	userAgentProvider := provider.NewHeaderProvider("User-Agent", useragent.BuildUserAgent(in.SourceTag))
	clientOptions = append(clientOptions, control.WithRequestEditorFn(userAgentProvider.Intercept))

	envAdditionalHeaders, hasEnvAdditionalHeaders := os.LookupEnv("PINECONE_ADDITIONAL_HEADERS")
	additionalHeaders := make(map[string]string)

	// add headers from environment
	if hasEnvAdditionalHeaders {
		err := json.Unmarshal([]byte(envAdditionalHeaders), &additionalHeaders)
		if err != nil {
			log.Printf("failed to parse PINECONE_ADDITIONAL_HEADERS: %v", err)
		}
	}

	// merge headers from parameters if passed
	if in.Headers != nil {
		for key, value := range in.Headers {
			additionalHeaders[key] = value
		}
	}

	// add headers to client options
	for key, value := range additionalHeaders {
		headerProvider := provider.NewHeaderProvider(key, value)
		clientOptions = append(clientOptions, control.WithRequestEditorFn(headerProvider.Intercept))
	}

	// apply custom http client if provided
	if in.RestClient != nil {
		clientOptions = append(clientOptions, control.WithHTTPClient(in.RestClient))
	}

	return clientOptions
}

func ensureURLScheme(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Scheme == "" {
		return "https://" + inputURL, nil
	}
	return inputURL, nil
}

func valueOrFallback[T comparable](value, fallback T) T {
	var zero T // set to zero-value of generic type T
	if value != zero {
		return value
	} else {
		return fallback
	}
}

func derefOrDefault[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func minOne(x int32) int32 {
	if x < 1 { // ensure x is at least 1
		return 1
	}
	return x
}
