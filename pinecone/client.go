// Package pinecone provides a client for the [Pinecone managed vector database].
//
// [Pinecone managed vector database]: https://www.pinecone.io/
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

	"github.com/pinecone-io/go-pinecone/v3/internal/gen"
	"github.com/pinecone-io/go-pinecone/v3/internal/gen/db_control"
	db_data_rest "github.com/pinecone-io/go-pinecone/v3/internal/gen/db_data/rest"
	"github.com/pinecone-io/go-pinecone/v3/internal/gen/inference"
	"github.com/pinecone-io/go-pinecone/v3/internal/provider"
	"github.com/pinecone-io/go-pinecone/v3/internal/useragent"
	"google.golang.org/grpc"
)

// [Client] holds the parameters for connecting to the Pinecone service. It is returned by the [NewClient] and [NewClientBase]
// functions. To use Client, first build the parameters of the request using [NewClientParams] (or [NewClientBaseParams]).
// Then, pass those parameters into the [NewClient] (or [NewClientBase]) function to create a new [Client] object.
// Once instantiated, you can use [Client] to execute Pinecone API requests (e.g. create an [Index], list Indexes,
// etc.), and Inference API requests. Read more about different Pinecone API routes [here].
//
// Note: Client methods are safe for concurrent use.
//
// Fields:
//   - Inference: An [InferenceService] object that exposes methods for interacting with the Pinecone [Inference API].
//   - restClient: Optional underlying *http.Client object used to communicate with the Pinecone API,
//     provided through [NewClientParams.RestClient] or [NewClientBaseParams.RestClient]. If not provided,
//     a default client is created for you.
//   - baseParams: A [NewClientBaseParams] object that holds the configuration for the Pinecone client.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams) // --> This creates a new Client object.
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    } else {
//		       fmt.Printf("Successfully found the \"%s\" index!\n", idx.Name)
//	    }
//
//	    idxConnection, err := pc.Index(idx.Host)
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    } else {
//		       log.Println("IndexConnection created successfully!")
//	    }
//
// [here]: https://docs.pinecone.io/reference/api/control-plane/list_indexes
// [Inference API]: https://docs.pinecone.io/reference/api/2024-07/inference/generate-embeddings
type Client struct {
	Inference  *InferenceService
	restClient *db_control.Client
	baseParams *NewClientBaseParams
}

// [NewClientParams] holds the parameters for creating a new [Client] instance while authenticating via an API key.
//
// Fields:
//   - ApiKey: (Required) The API key used to authenticate with the Pinecone API.
//     This value must be passed by the user unless it is set as an environment variable ("PINECONE_API_KEY").
//   - Headers: (Optional) An optional map of HTTP headers to include in each API request.
//   - Host: (Optional) The host URL of the Pinecone API. If not provided, the default value is "https://api.pinecone.io".
//   - RestClient: An optional HTTP client to use for communication with the Pinecone API.
//   - SourceTag: An optional string used to help Pinecone attribute API activity.
//
// See [Client] for code example.
type NewClientParams struct {
	ApiKey     string            // required - provide through NewClientParams or environment variable PINECONE_API_KEY
	Headers    map[string]string // optional
	Host       string            // optional
	RestClient *http.Client      // optional
	SourceTag  string            // optional
}

// [NewClientBaseParams] holds the parameters for creating a new [Client] instance while passing custom authentication
// headers. If there is no API key or authentication provided through Headers, API calls will fail.
//
// Fields:
//   - Headers: (Optional) A map of HTTP headers to include in each API request.
//     "Authorization" and "X-Project-Id" headers are required if authenticating using a JWT.
//   - Host: (Optional) The host URL of the Pinecone API. If not provided,
//     the default value is "https://api.pinecone.io".
//   - RestClient: (Optional) An *http.Client object to use for communication with the Pinecone API.
//   - SourceTag: (Optional) A string used to help Pinecone attribute API activity.
//
// See [Client] for code example.
type NewClientBaseParams struct {
	Headers    map[string]string
	Host       string
	RestClient *http.Client
	SourceTag  string
}

// [NewIndexConnParams] holds the parameters for creating an [IndexConnection] to a Pinecone index.
//
// Fields:
//   - Host: (Required) The host URL of the Pinecone index. To find your host url use the [Client.DescribeIndex] or [Client.ListIndexes] methods.
//     Alternatively, the host is displayed in the Pinecone web console.
//   - Namespace: (Optional) The index namespace to use for operations. If not provided, the default namespace of "" will be used.
//   - AdditionalMetadata: (Optional) Metadata to be sent with each RPC request.
//
// See [Client.Index] for code example.
type NewIndexConnParams struct {
	Host               string            // required - obtained through DescribeIndex or ListIndexes
	Namespace          string            // optional - if not provided the default namespace of "" will be used
	AdditionalMetadata map[string]string // optional
}

// [NewClient] creates and initializes a new instance of [Client].
// This function sets up the Pinecone client with the necessary configuration for authentication and communication.
//
// Parameters:
//   - in: A [NewClientParams] object. See [NewClientParams] for more information.
//
// Note: It is important to handle the error returned by this function to ensure that the
// Pinecone client has been created successfully before attempting to make API calls.
//
// Returns a pointer to an initialized [Client] instance or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
func NewClient(in NewClientParams) (*Client, error) {
	osApiKey := os.Getenv("PINECONE_API_KEY")
	hasApiKey := valueOrFallback(in.ApiKey, osApiKey) != ""

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

// [NewClientBase] creates and initializes a new instance of [Client] with custom authentication headers.
//
// Parameters:
//   - in: A [NewClientBaseParams] object that includes the necessary configuration for the Pinecone client. See
//     [NewClientBaseParams] for more information.
//
// Notes:
//   - It is important to handle the error returned by this function to ensure that the
//     Pinecone client has been created successfully before attempting to make API calls.
//   - A Pinecone API key is not required when using [NewClientBase].
//
// Returns a pointer to an initialized [Client] instance or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientBaseParams{
//	        Headers: map[string]string{
//	            "Authorization": "Bearer " + "<your OAuth token>"
//	            "X-Project-Id": "<Your Pinecone project ID>"
//	        },
//	        SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClientBase(clientParams)
//		       if err != nil {
//	            log.Fatalf("Failed to create Client: %v", err)
//	        } else {
//		           fmt.Println("Successfully created a new Client object!")
//	    }
func NewClientBase(in NewClientBaseParams) (*Client, error) {
	controlOptions := buildClientBaseOptions(in)
	inferenceOptions := buildInferenceBaseOptions(in)
	var err error

	controlHostOverride := valueOrFallback(in.Host, os.Getenv("PINECONE_CONTROLLER_HOST"))
	if controlHostOverride != "" {
		controlHostOverride, err = ensureURLScheme(controlHostOverride)
		if err != nil {
			return nil, err
		}
	}

	dbControlClient, err := db_control.NewClient(valueOrFallback(controlHostOverride, "https://api.pinecone.io"), controlOptions...)
	if err != nil {
		return nil, err
	}
	inferenceClient, err := inference.NewClient(valueOrFallback(controlHostOverride, "https://api.pinecone.io"), inferenceOptions...)
	if err != nil {
		return nil, err
	}

	c := Client{
		Inference:  &InferenceService{client: inferenceClient},
		restClient: dbControlClient,
		baseParams: &in,
	}
	return &c, nil
}

// [Client.Index] creates an [IndexConnection] to a specified host.
//
// Parameters:
//   - in: A [NewIndexConnParams] object that includes the necessary configuration to create an [IndexConnection].
//     See NewIndexConnParams for more information.
//
// Note: It is important to handle the error returned by this method to ensure that the [IndexConnection] is created
// successfully before making data plane calls.
//
// Returns a pointer to an [IndexConnection] instance or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    } else {
//		       fmt.Printf("Successfully found the \"%s\" index!\n", idx.Name)
//	    }
//
//	    indexConnParams := pinecone.NewIndexConnParams{
//		       Host: idx.Host,
//		       Namespace: "your-namespace",
//		       AdditionalMetadata: map[string]string{
//			       "your-metadata-key": "your-metadata-value",
//		       },
//	    }
//
//	    idxConnection, err := pc.Index(indexConnParams)
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    } else {
//		       log.Println("IndexConnection created successfully!")
//	    }
func (c *Client) Index(in NewIndexConnParams, dialOpts ...grpc.DialOption) (*IndexConnection, error) {
	if in.AdditionalMetadata == nil {
		in.AdditionalMetadata = make(map[string]string)
	}

	if in.Host == "" {
		return nil, fmt.Errorf("field Host is required to create an IndexConnection. Find your Host from calling DescribeIndex or via the Pinecone console")
	}

	// add api version header if not provided
	if _, ok := in.AdditionalMetadata["X-Pinecone-Api-Version"]; !ok {
		in.AdditionalMetadata["X-Pinecone-Api-Version"] = gen.PineconeApiVersion
	}

	// extract authHeader from Client which is used to authenticate the IndexConnection
	// merge authHeader with additionalMetadata provided in NewIndexConnParams
	authHeader := c.extractAuthHeader()
	for key, value := range authHeader {
		in.AdditionalMetadata[key] = value
	}

	dbDataOptions := buildDataClientBaseOptions(*c.baseParams)
	dbDataClient, err := db_data_rest.NewClient(ensureHostHasHttps(in.Host), dbDataOptions...)
	if err != nil {
		return nil, err
	}

	idx, err := newIndexConnection(newIndexParameters{
		host:               in.Host,
		namespace:          in.Namespace,
		sourceTag:          c.baseParams.SourceTag,
		additionalMetadata: in.AdditionalMetadata,
		dbDataClient:       dbDataClient,
	}, dialOpts...)
	if err != nil {
		return nil, err
	}
	return idx, nil
}

func ensureHostHasHttps(host string) string {
	if strings.HasPrefix("http://", host) {
		return strings.Replace(host, "http://", "https://", 1)
	} else if !strings.HasPrefix("https://", host) {
		return "https://" + host
	}

	return host
}

// [Client.ListIndexes] retrieves a list of all Indexes in a Pinecone [project].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//
// Returns a slice of pointers to Index objects or an error.
//
// Example:
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    idxs, err := pc.ListIndexes(ctx)
//	    if err != nil {
//		       log.Fatalf("Failed to list indexes: %v", err)
//	    } else {
//		       fmt.Println("Your project has the following indexes:")
//		       for _, idx := range idxs {
//			       fmt.Printf("- \"%s\"\n", idx.Name)
//		       }
//	    }
//
// [project]: https://docs.pinecone.io/guides/projects/understanding-projects
func (c *Client) ListIndexes(ctx context.Context) ([]*Index, error) {
	res, err := c.restClient.ListIndexes(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list indexes: ")
	}

	var indexList db_control.IndexList
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

// [CreatePodIndexRequest] holds the parameters for creating a new pods-based Index.
//
// Fields:
//   - Name: (Required) The name of the [Index]. Resource name must be 1-45 characters long,
//     start and end with an alphanumeric character,
//     and consist only of lower case alphanumeric characters or '-'.
//   - Dimension: (Required) The [dimensionality] of the vectors to be inserted in the Index.
//   - Metric: (Required) The distance metric to be used for [similarity] search. You can use
//     'euclidean', 'cosine', or 'dotproduct'. Defaults to 'cosine'.
//   - DeletionProtection: (Optional) determines whether [deletion protection] is "enabled" or "disabled" for the index.
//     When "enabled", the index cannot be deleted. Defaults to "disabled".
//   - Environment: (Required) The [cloud environment] where the Index will be hosted.
//   - PodType: (Required) The [type of pod] to use for the [Index]. One of `s1`, `p1`, or `p2` appended with `.` and
//     one of `x1`, `x2`, `x4`, or `x8`.
//   - Shards: (Optional) The number of shards to use for the Index (defaults to 1).
//     Shards split your data across multiple pods, so you can fit more data into an Index.
//   - Replicas: (Optional) The number of [replicas] to use for the Index (defaults to 1). Replicas duplicate your Index.
//     They provide higher availability and throughput. Replicas can be scaled up or down as your needs change.
//   - SourceCollection: (Optional) The name of the [Collection] to be used as the source for the Index.
//   - MetadataConfig: (Optional) The [metadata configuration] for the behavior of Pinecone's internal metadata Index. By
//     default, all metadata is indexed; when `metadata_config` is present,
//     only specified metadata fields are indexed. These configurations are
//     only valid for use with pod-based Indexes.
//   - Tags: (Optional) A map of tags to associate with the Index.
//
// To create a new pods-based Index, use the [Client.CreatePodIndex] method.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    podIndexMetadata := &pinecone.PodSpecMetadataConfig{
//		       Indexed: &[]string{"title", "description"},
//	    }
//
//	    indexName := "my-pod-index"
//
//	    idx, err := pc.CreatePodIndex(ctx, &pinecone.CreatePodIndexRequest{
//	        Name:        indexName,
//	        Dimension:   3,
//	        Metric:      pinecone.Cosine,
//	        Environment: "us-west1-gcp",
//	        PodType:     "s1",
//	        MetadataConfig: podIndexMetadata,
//	        })
//
//	    if err != nil {
//		       log.Fatalf("Failed to create pod index: %v", err)
//	    } else {
//		       fmt.Printf("Successfully created pod index: %s", idx.Name)
//	    }
//
// [dimensionality]: https://docs.pinecone.io/guides/indexes/choose-a-pod-type-and-size#dimensionality-of-vectors
// [similarity]: https://docs.pinecone.io/guides/indexes/understanding-indexes#distance-metrics
// [metadata configuration]: https://docs.pinecone.io/guides/indexes/configure-pod-based-indexes#selective-metadata-indexing
// [cloud environment]: https://docs.pinecone.io/guides/indexes/understanding-indexes#pod-environments
// [replicas]: https://docs.pinecone.io/guides/indexes/configure-pod-based-indexes#add-replicas
// [type of pod]: https://docs.pinecone.io/guides/indexes/choose-a-pod-type-and-size
// [deletion protection]: https://docs.pinecone.io/guides/indexes/prevent-index-deletion#enable-deletion-protection
type CreatePodIndexRequest struct {
	Name               string
	Dimension          int32
	Environment        string
	PodType            string
	Shards             int32
	Replicas           int32
	Metric             *IndexMetric
	DeletionProtection *DeletionProtection
	SourceCollection   *string
	MetadataConfig     *PodSpecMetadataConfig
	Tags               *IndexTags
}

// [CreatePodIndexRequestReplicaCount] ensures the replica count of a pods-based Index is >1.
// It returns a pointer to the number of replicas on a [CreatePodIndexRequest] object.
func (req CreatePodIndexRequest) ReplicaCount() int32 {
	return minOne(req.Replicas)
}

// [CreatePodIndexRequestShardCount] ensures the number of shards on a pods-based Index is >1. It returns a pointer to the number of shards on
// a [CreatePodIndexRequest] object.
func (req CreatePodIndexRequest) ShardCount() int32 {
	return minOne(req.Shards)
}

// [CreatePodIndexRequest.TotalCount] calculates and returns the total number of pods (replicas*shards) on a [CreatePodIndexRequest] object.
func (req CreatePodIndexRequest) TotalCount() int {
	return int(req.ReplicaCount() * req.ShardCount())
}

// [Client.CreatePodIndex] creates and initializes a new pods-based Index via the specified [Client].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreatePodIndexRequest] object. See [CreatePodIndexRequest] for more information.
//
// Returns a pointer to an [Index] object or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    podIndexMetadata := &pinecone.PodSpecMetadataConfig{
//		       Indexed: &[]string{"title", "description"},
//	    }
//
//	    indexName := "my-pod-index"
//
//		idx, err := pc.CreatePodIndex(ctx, &pinecone.CreatePodIndexRequest{
//		    Name:        indexName,
//		    Dimension:   3,
//		    Metric:      pinecone.Cosine,
//		    Environment: "us-west1-gcp",
//		    PodType:     "s1",
//		    MetadataConfig: podIndexMetadata,
//		})
//
//		if err != nil {
//	    	log.Fatalf("Failed to create pod index:", err)
//		} else {
//			   fmt.Printf("Successfully created pod index: %s", idx.Name)
//		}
func (c *Client) CreatePodIndex(ctx context.Context, in *CreatePodIndexRequest) (*Index, error) {
	if in.Name == "" || in.Dimension <= 0 || in.Environment == "" || in.PodType == "" {
		return nil, fmt.Errorf("fields Name, positive Dimension, Environment, and Podtype must be included in CreatePodIndexRequest")
	}

	var deletionProtection *db_control.DeletionProtection
	if in.DeletionProtection != nil {
		deletionProtection = pointerOrNil(db_control.DeletionProtection(*in.DeletionProtection))
	}

	var metric *db_control.CreateIndexRequestMetric
	if in.Metric != nil {
		metric = pointerOrNil(db_control.CreateIndexRequestMetric(*in.Metric))
	}

	pods := in.TotalCount()
	replicas := in.ReplicaCount()
	shards := in.ShardCount()
	vectorType := "dense"

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	req := db_control.CreateIndexRequest{
		Name:               in.Name,
		Dimension:          &in.Dimension,
		Metric:             metric,
		DeletionProtection: deletionProtection,
		Tags:               tags,
		VectorType:         &vectorType,
	}

	req.Spec = db_control.IndexSpec{
		Pod: &db_control.PodSpec{
			Environment:      in.Environment,
			PodType:          in.PodType,
			Pods:             &pods,
			Replicas:         &replicas,
			Shards:           &shards,
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

// [CreateServerlessIndexRequest] holds the parameters for creating a new [Serverless] Index.
//
// Fields:
//   - Name: (Required) The name of the [Index]. Resource name must be 1-45 characters long,
//     start and end with an alphanumeric character,
//     and consist only of lower case alphanumeric characters or '-'.
//   - Cloud: (Required) The public [cloud provider] where you would like your [Index] hosted.
//     For serverless Indexes, you define only the cloud and region where the [Index] should be hosted.
//   - Region: (Required) The [region] where you would like your [Index] to be created.
//   - Metric: (Optional) The metric used to measure the [similarity] between vectors ('euclidean', 'cosine', or 'dotproduct'). Defaults
//     to `cosine` or `dotproduct` depending on the VectorType.
//   - DeletionProtection: (Optional) Determines whether [deletion protection] is "enabled" or "disabled" for the index.
//     When "enabled", the index cannot be deleted. Defaults to "disabled".
//   - Dimension: (Optional) The [dimensionality] of the vectors to be inserted in the [Index].
//   - VectorType: (Optional) The index vector type. You can use `dense` or `sparse`. If `dense`, the vector dimension must be specified.
//     If `sparse`, the vector dimension should not be specified, and the Metric must be set to `dotproduct`. Defaults to `dense`.
//   - Tags: (Optional) A map of tags to associate with the Index.
//
// To create a new Serverless Index, use the [Client.CreateServerlessIndex] method.
//
// Example:
//
//	    ctx := context.Background()
//
//		clientParams := pinecone.NewClientParams{
//		    ApiKey:    "YOUR_API_KEY",
//			SourceTag: "your_source_identifier", // optional
//	    }
//
//		pc, err := pinecone.NewClient(clientParams)
//		if err != nil {
//		    log.Fatalf("Failed to create Client: %v", err)
//		} else {
//		    fmt.Println("Successfully created a new Client object!")
//		}
//
//		indexName := "my-serverless-index"
//
//		idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//		    Name:      indexName,
//			Dimension: 3,
//			Metric:  pinecone.Cosine,
//			Cloud:   pinecone.Aws,
//			Region:  "us-east-1",
//	    })
//
//		if err != nil {
//		    log.Fatalf("Failed to create serverless index: %s", indexName)
//		} else {
//		    fmt.Printf("Successfully created serverless index: %s", idx.Name)
//		}
//
// [dimensionality]: https://docs.pinecone.io/guides/indexes/choose-a-pod-type-and-size#dimensionality-of-vectors
// [Serverless]: https://docs.pinecone.io/guides/indexes/understanding-indexes#serverless-indexes
// [similarity]: https://docs.pinecone.io/guides/indexes/understanding-indexes#distance-metrics
// [region]: https://docs.pinecone.io/troubleshooting/available-cloud-regions
// [cloud provider]: https://docs.pinecone.io/troubleshooting/available-cloud-regions#regions-available-for-serverless-indexes
// [deletion protection]: https://docs.pinecone.io/guides/indexes/prevent-index-deletion#enable-deletion-protection
type CreateServerlessIndexRequest struct {
	Name               string
	Cloud              Cloud
	Region             string
	Metric             *IndexMetric
	DeletionProtection *DeletionProtection
	Dimension          *int32
	VectorType         *string
	Tags               *IndexTags
}

// [Client.CreateServerlessIndex] creates and initializes a new serverless Index via the specified [Client].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateServerlessIndexRequest] object. See [CreateServerlessIndexRequest] for more information.
//
// Returns a pointer to an [Index] object or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    indexName := "my-serverless-index"
//
//	    idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//		    Name:    indexName,
//		    Dimension: 3,
//		    Metric:  pinecone.Cosine,
//		    Cloud:   pinecone.Aws,
//		    Region:  "us-east-1",
//		})
//
//		if err != nil {
//		    log.Fatalf("Failed to create serverless index: %s", indexName)
//		} else {
//		    fmt.Printf("Successfully created serverless index: %s", idx.Name)
//		}
func (c *Client) CreateServerlessIndex(ctx context.Context, in *CreateServerlessIndexRequest) (*Index, error) {
	if in.Name == "" || in.Cloud == "" || in.Region == "" {
		return nil, fmt.Errorf("fields Name, Cloud, and Region must be included in CreateServerlessIndexRequest")
	}

	// default to "dense" if VectorType is not specified
	vectorType := in.VectorType

	// validate VectorType options
	if in.VectorType != nil {
		switch *in.VectorType {
		case "sparse":
			if in.Dimension != nil {
				return nil, fmt.Errorf("Dimension should not be specified when VectorType is 'sparse'")
			} else if in.Metric != nil && *in.Metric != Dotproduct {
				return nil, fmt.Errorf("Metric should be 'dotproduct' when VectorType is 'sparse'")
			}
		case "dense":
			if in.Dimension == nil {
				return nil, fmt.Errorf("Dimension should be specified when VectorType is 'dense'")
			}
		default:
			return nil, fmt.Errorf("unsupported VectorType: %s", *in.VectorType)
		}
	}

	var deletionProtection *db_control.DeletionProtection
	if in.DeletionProtection != nil {
		deletionProtection = pointerOrNil(db_control.DeletionProtection(*in.DeletionProtection))
	}

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	var metric *db_control.CreateIndexRequestMetric
	if in.Metric != nil {
		metric = pointerOrNil(db_control.CreateIndexRequestMetric(*in.Metric))
	}

	req := db_control.CreateIndexRequest{
		Name:               in.Name,
		Dimension:          in.Dimension,
		Metric:             metric,
		DeletionProtection: deletionProtection,
		VectorType:         vectorType,
		Spec: db_control.IndexSpec{
			Serverless: &db_control.ServerlessSpec{
				Cloud:  db_control.ServerlessSpecCloud(in.Cloud),
				Region: in.Region,
			},
		},
		Tags: tags,
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

// [CreateIndexForModelRequest] defines the desired configuration for creating an index with an associated embedding model.
//
// Fields:
//   - Name: (Required) The name of the [Index]. Resource name must be 1-45 characters long,
//     start and end with an alphanumeric character, and consist only of lower case alphanumeric characters or '-'.
//   - Cloud: (Required) The public [cloud provider] where you would like your [Index] hosted.
//   - Region: (Required) The [region] where you would like your [Index] to be created.
//   - DeletionProtection: (Optional) Whether [deletion protection] is enabled or disabled for the index.
//     When enabled, the index cannot be deleted. Defaults to disabled.
//   - Embed: (Required) The [CreateIndexForModelEmbed] object for embedding model configuration.
//     Once set, the model cannot be changed, but embedding configurations such as field map, read parameters,
//     or write parameters can be updated.
//   - FieldMap: Identifies the name of the text field from your document model that will be embedded.
//   - Metric: The [similarity metric] to be used for similarity search. Options: 'euclidean', 'cosine', or 'dotproduct'.
//     If not specified, the metric will default according to the model and cannot be updated once set.
//   - Model: The name of the embedding model to use for the index.
//   - ReadParameters: The read parameters for the embedding model.
//   - WriteParameters: The write parameters for the embedding model.
//   - Tags: (Optional) Custom user tags added to an index.
//     Keys must be 80 characters or less, values must be 120 characters or less.
//     Keys must be alphanumeric, '_', or '-'. Values must be alphanumeric, ';', '@', '_', '-', '.', '+', or ' '.
//     To unset a key, set the value to be an empty string.
//
// To create an index with an associated embedding model, use the [Client.CreateIndexForModel] method.
//
// Example:
//
//	ctx := context.Background()
//
//	clientParams := pinecone.NewClientParams{
//	     ApiKey:    "YOUR_API_KEY",
//	     SourceTag: "your_source_identifier", // optional
//	}
//
//	pc, err := pinecone.NewClient(clientParams)
//	if err != nil {
//	     log.Fatalf("Failed to create Client: %v", err)
//	}
//
//	request := &pinecone.CreateIndexForModelRequest{
//	     Name:   "my-index",
//	     Cloud:  pinecone.Aws,
//	     Region: "us-east-1",
//	     Embed: CreateIndexForModelEmbed{
//			Model:    "multilingual-e5-large",
//			FieldMap: map[string]interface{}{"text": "chunk_text"},
//		 },
//	}
//
//	idx, err := pc.CreateIndexForModel(ctx, request)
//	if err != nil {
//	     log.Fatalf("Failed to create index: %v", err)
//	} else {
//	     fmt.Printf("Successfully created index: %s", idx.Name)
//	}
//
// [Index]: https://docs.pinecone.io/guides/indexes/understanding-indexes
// [region]: https://docs.pinecone.io/troubleshooting/available-cloud-regions
// [cloud provider]: https://docs.pinecone.io/troubleshooting/available-cloud-regions#regions-available-for-serverless-indexes
// [deletion protection]: https://docs.pinecone.io/guides/indexes/manage-indexes#enable-deletion-protection
// [similarity metric]: https://docs.pinecone.io/guides/indexes/understanding-indexes#similarity-metrics
type CreateIndexForModelRequest struct {
	Name               string
	Cloud              Cloud
	Region             string
	DeletionProtection *DeletionProtection
	Embed              CreateIndexForModelEmbed
	Tags               *IndexTags
}

// [CreateIndexForModelEmbed] defines the embedding model configuration for an index.
//
// Fields:
//   - Model: (Required) The name of the embedding model to use for the index.
//   - FieldMap: (Required) Identifies the name of the text field from your document model that will be embedded.
//   - Dimension: (Optional) The dimensionality of the vectors to be inserted in the Index. If not specified, the dimension
//     will be defaulted according to the model.
//   - Metric: (Optional) The [similarity metric] to be used for similarity search. You can use 'euclidean', 'cosine', or 'dotproduct'.
//     If not specified, the metric will be defaulted according to the model. Cannot be updated once set.
//   - ReadParameters: (Optional) Read parameters for the embedding model.
//   - WriteParameters: (Optional) Write parameters for the embedding model.
//
// The `CreateIndexForModelEmbed` struct is used as part of the [CreateIndexForModelRequest] when creating an index
// with an associated embedding model. Once an index is created, the `model` field cannot be changed, but other
// configurations such as `field_map`, `read_parameters`, and `write_parameters` can be updated.
//
// [similarity metric]: https://docs.pinecone.io/guides/indexes/understanding-indexes#similarity-metrics
type CreateIndexForModelEmbed struct {
	Model           string
	FieldMap        map[string]interface{}
	Dimension       *int
	Metric          *IndexMetric
	ReadParameters  *map[string]interface{}
	WriteParameters *map[string]interface{}
}

// [Client.CreateIndexForModel] creates and initializes a new serverless Index via the specified [Client] that is configured
// for use with one of Pinecone's integrated inference models. After the index is created, you can upsert and search for records
// using the [IndexConnection.UpsertRecords] and [IndexConnection.SearchRecords] methods.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateIndexForModelRequest] object. See [CreateIndexForModelRequest] for more information.
//
// Returns a pointer to an [Index] object or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    indexName := "my-serverless-index"
//
//	    idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//		    Name:    indexName,
//		    Dimension: 3,
//		    Metric:  pinecone.Cosine,
//		    Cloud:   pinecone.Aws,
//		    Region:  "us-east-1",
//		})
//
//		if err != nil {
//		    log.Fatalf("Failed to create serverless index: %s", indexName)
//		} else {
//		    fmt.Printf("Successfully created serverless index: %s", idx.Name)
//		}
func (c *Client) CreateIndexForModel(ctx context.Context, in *CreateIndexForModelRequest) (*Index, error) {
	if in.Name == "" || in.Cloud == "" || in.Region == "" || in.Embed.Model == "" {
		return nil, fmt.Errorf("fields Name, Cloud, and Region, and Embed.Model must be included in CreateServerlessIndexRequest")
	}

	deletionProtection := derefOrDefault(in.DeletionProtection, "disabled")

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	req := db_control.CreateIndexForModelRequest{
		Name:   in.Name,
		Region: in.Region,
		Cloud:  db_control.CreateIndexForModelRequestCloud(in.Cloud),
		Embed: struct {
			Dimension       *int                                              `json:"dimension,omitempty"`
			FieldMap        map[string]interface{}                            `json:"field_map"`
			Metric          *db_control.CreateIndexForModelRequestEmbedMetric `json:"metric,omitempty"`
			Model           string                                            `json:"model"`
			ReadParameters  *map[string]interface{}                           `json:"read_parameters,omitempty"`
			WriteParameters *map[string]interface{}                           `json:"write_parameters,omitempty"`
		}{
			Dimension:       in.Embed.Dimension,
			FieldMap:        in.Embed.FieldMap,
			Metric:          (*db_control.CreateIndexForModelRequestEmbedMetric)(in.Embed.Metric),
			Model:           in.Embed.Model,
			ReadParameters:  in.Embed.ReadParameters,
			WriteParameters: in.Embed.WriteParameters,
		},
		DeletionProtection: (*db_control.DeletionProtection)(&deletionProtection),
		Tags:               tags,
	}

	res, err := c.restClient.CreateIndexForModel(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, handleErrorResponseBody(res, "failed to create index: ")
	}

	return decodeIndex(res.Body)
}

// [Client.DescribeIndex] retrieves information about a specific [Index]. See [Index] for more information.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - idxName: The name of the [Index] to describe.
//
// Returns a pointer to an [Index] object or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "the-name-of-my-index")
//	    if err != nil {
//	        log.Fatalf("Failed to describe index: %s", err)
//	    } else {
//	        desc := fmt.Sprintf("Description: \n  Name: %s\n  Dimension: %d\n  Host: %s\n  Metric: %s\n"+
//			"  DeletionProtection"+
//			": %s\n"+
//			"  Spec: %+v"+
//			"\n  Status: %+v\n",
//			idx.Name, idx.Dimension, idx.Host, idx.Metric, idx.DeletionProtection, idx.Spec, idx.Status)
//
//		    fmt.Println(desc)
//	    }
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

// [Client.DeleteIndex] deletes a specific [Index].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - idxName: The name of the [Index] to delete.
//
// Returns an error if the deletion fails.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    indexName := "the-name-of-my-index"
//
//	    err = pc.DeleteIndex(ctx, indexName)
//	    if err != nil {
//		       log.Fatalf("Error: %v", err)
//	    } else {
//	        fmt.Printf("Index \"%s\" deleted successfully", indexName)
//	    }
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

// [ConfigureIndexParams] contains parameters for configuring an [Index]. For both pod-based
// and serverless indexes you can configure the DeletionProtection status for an [Index].
// For pod-based indexes you can also configure the number of Replicas and the PodType.
// Each of the fields is optional, but at least one field must be set.
// See [scale a pods-based index] for more information.
//
// Fields:
//   - PodType: (Optional) The pod size to scale the index to. For a "p1" pod type,
//     you could pass "p1.x2" to scale your index to the "x2" size, or you could pass "p1.x4"
//     to scale your index to the "x4" size, and so forth. Only applies to pod-based indexes.
//   - Replicas: (Optional) The number of replicas to scale the index to. This is capped by
//     the maximum number of replicas allowed in your Pinecone project. To configure this number,
//     go to [app.pinecone.io], select your project, and configure the maximum number of pods.
//   - DeletionProtection: (Optional) DeletionProtection determines whether [deletion protection]
//     is "enabled" or "disabled" for the index. When "enabled", the index cannot be deleted. Defaults to "disabled".
//   - Tags: (Optional) A map of tags to associate with the Index.
//   - Embed: (Optional) The [ConfigureIndexEmbed] object for integrated index configuration.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    idx, err := pc.ConfigureIndex(ctx, "my-index", ConfigureIndexParams{ DeletionProtection: "enabled", Replicas: 4 })
//
// [app.pinecone.io]: https://app.pinecone.io
// [scale a pods-based index]: https://docs.pinecone.io/guides/indexes/configure-pod-based-indexes
// [deletion protection]: https://docs.pinecone.io/guides/indexes/prevent-index-deletion#enable-deletion-protection
type ConfigureIndexParams struct {
	PodType            string
	Replicas           int32
	DeletionProtection DeletionProtection
	Tags               IndexTags
	Embed              *ConfigureIndexEmbed
}

// [ConfigureIndexEmbed] contains parameters for configuring the integrated inference embedding settings for an [Index].
// You can convert an existing serverless index to an integrated index by specifying the Model and FieldMap.
// The index vector type and dimension must match the model vector type and dimension, and the index similarity metric must be supported by the model.
// Refer to the [model guide](https://docs.pinecone.io/guides/inference/understanding-inference#embedding-models) for available models and model details.
//
// You can later change the embedding configuration to update the field map, read parameters, or write parameters. Once set, the model cannot be changed.
//
// Fields:
//   - FieldMap: (Optional) Identifies the name of the text field from your document model that will be embedded.
//   - Model: (Optional) The name of the embedding model to use with the index.
//   - ReadParameters: (Optional) The read parameters for the embedding model.
//   - WriteParameters: (Optional) The write parameters for the embedding model.
type ConfigureIndexEmbed struct {
	FieldMap        *map[string]interface{}
	Model           *string
	ReadParameters  *map[string]interface{}
	WriteParameters *map[string]interface{}
}

// [Client.ConfigureIndex] is used to [scale a pods-based index] up or down by changing the size of the pods or the number of
// replicas, or to enable and disable deletion protection for an [Index].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - name: The name of the [Index] to configure.
//   - in: A pointer to a ConfigureIndexParams object that contains the parameters for configuring the [Index].
//
// Note: You can only scale an [Index] up, not down. If you want to scale an [Index] down,
// you must create a new index with the desired configuration.
//
// Returns a pointer to a configured [Index] object or an error.
//
// Example:
//
//		// To scale the size of your pods-based index from "x2" to "x4":
//		 _, err := pc.ConfigureIndex(ctx, "my-pod-index", ConfigureIndexParams{PodType: "p1.x4"})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
//		// To scale the number of replicas:
//		 _, err := pc.ConfigureIndex(ctx, "my-pod-index", ConfigureIndexParams{Replicas: 4})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
//		// To scale both the size of your pods and the number of replicas to 4:
//		 _, err := pc.ConfigureIndex(ctx, "my-pod-index", ConfigureIndexParams{PodType: "p1.x4", Replicas: 4})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
//	    // To enable deletion protection:
//		 _, err := pc.ConfigureIndex(ctx, "my-index", ConfigureIndexParams{DeletionProtection: "enabled"})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
// [scale a pods-based index]: https://docs.pinecone.io/guides/indexes/configure-pod-based-indexes
func (c *Client) ConfigureIndex(ctx context.Context, name string, in ConfigureIndexParams) (*Index, error) {
	if in.PodType == "" && in.Replicas == 0 && in.DeletionProtection == "" && in.Tags == nil {
		return nil, fmt.Errorf("must specify PodType, Replicas, DeletionProtection, or Tags when configuring an index")
	}

	podType := pointerOrNil(in.PodType)
	replicas := pointerOrNil(in.Replicas)
	deletionProtection := pointerOrNil(in.DeletionProtection)

	// Describe index in order to merge existing tags with incoming tags
	idxDesc, err := c.DescribeIndex(ctx, name)
	if err != nil {
		return nil, err
	}
	existingTags := idxDesc.Tags

	var request db_control.ConfigureIndexRequest
	if podType != nil || replicas != nil {
		request.Spec =
			&struct {
				Pod struct {
					PodType  *string `json:"pod_type,omitempty"`
					Replicas *int32  `json:"replicas,omitempty"`
				} `json:"pod"`
			}{
				Pod: struct {
					PodType  *string `json:"pod_type,omitempty"`
					Replicas *int32  `json:"replicas,omitempty"`
				}{
					PodType:  podType,
					Replicas: replicas,
				},
			}
	}
	if in.Embed != nil {
		request.Embed =
			&struct {
				FieldMap        *map[string]interface{} `json:"field_map,omitempty"`
				Model           *string                 `json:"model,omitempty"`
				ReadParameters  *map[string]interface{} `json:"read_parameters,omitempty"`
				WriteParameters *map[string]interface{} `json:"write_parameters,omitempty"`
			}{
				FieldMap:        in.Embed.FieldMap,
				Model:           in.Embed.Model,
				ReadParameters:  in.Embed.ReadParameters,
				WriteParameters: in.Embed.WriteParameters,
			}
	}

	request.DeletionProtection = (*db_control.DeletionProtection)(deletionProtection)
	request.Tags = (*db_control.IndexTags)(mergeIndexTags(existingTags, in.Tags))

	res, err := c.restClient.ConfigureIndex(ctx, name, request)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to configure index: ")
	}

	return decodeIndex(res.Body)
}

// [Client.ListCollections] retrieves a list of all Collections in a Pinecone [project]. See [understanding collections] for more information.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//
// Returns a slice of pointers to [Collection] objects or an error.
//
// Note: Collections are only available for pods-based Indexes.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    collections, err := pc.ListCollections(ctx)
//	    if err != nil {
//		       log.Fatalf("Failed to list collections: %v", err)
//	    } else {
//		       if len(collections) == 0 {
//		           fmt.Printf("No collections found in project")
//		       } else {
//		           fmt.Println("Collections in project:")
//		           for _, collection := range collections {
//			           fmt.Printf("- %s\n", collection.Name)
//		           }
//		       }
//	    }
//
// [project]: https://docs.pinecone.io/guides/projects/understanding-projects
// [understanding collections]: https://docs.pinecone.io/guides/indexes/understanding-collections
func (c *Client) ListCollections(ctx context.Context) ([]*Collection, error) {
	res, err := c.restClient.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list collections: ")
	}

	var collectionsResponse db_control.CollectionList
	if err := json.NewDecoder(res.Body).Decode(&collectionsResponse); err != nil {
		return nil, err
	}

	var collections []*Collection
	for _, collectionModel := range *collectionsResponse.Collections {
		collections = append(collections, toCollection(&collectionModel))
	}

	return collections, nil
}

// [Client.DescribeCollection] retrieves information about a specific [Collection]. See [understanding collections]
// for more information.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - collectionName: The name of the [Collection] to describe.
//
// Returns a pointer to a [Collection] object or an error.
//
// Note: Collections are only available for pods-based Indexes.
//
// Since the returned value is a pointer to a [Collection] object, it will have the following fields:
//   - Name: The name of the [Collection].
//   - Size: The size of the [Collection] in bytes.
//   - Status: The status of the [Collection].
//   - Dimension: The [dimensionality] of the vectors stored in each record held in the [Collection].
//   - VectorCount: The number of records stored in the [Collection].
//   - Environment: The cloud environment where the [Collection] is hosted.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    collection, err := pc.DescribeCollection(ctx, "my-collection")
//	    if err != nil {
//		       log.Fatalf("Error describing collection: %v", err)
//	    } else {
//		       fmt.Printf("Collection: %+v\n", *collection)
//	    }
//
// [dimensionality]: https://docs.pinecone.io/guides/indexes/choose-a-pod-type-and-size#dimensionality-of-vectors
// [understanding collections]: https://docs.pinecone.io/guides/indexes/understanding-collections
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

// [CreateCollectionRequest] holds the parameters for creating a new [Collection].
//
// Fields:
//   - Name: (Required) The name of the [Collection].
//   - Source: (Required) The name of the Index to be used as the source for the [Collection].
//
// To create a new [Collection], use the [Client.CreateCollection] method.
//
// Note: Collections are only available for pods-based Indexes.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    collection, err := pc.CreateCollection(ctx, &pinecone.CreateCollectionRequest{
//	        Name:   "my-collection",
//	        Source: "my-source-index",
//	     })
//	    if err != nil {
//		       log.Fatalf("Failed to create collection: %v", err)
//	    } else {
//		       fmt.Printf("Successfully created collection \"%s\".", collection.Name)
//	    }
type CreateCollectionRequest struct {
	Name   string
	Source string
}

// [Client.CreateCollection] creates and initializes a new [Collection] via the specified [Client].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateCollectionRequest] object.
//
// Note: Collections are only available for pods-based Indexes.
//
// Returns a pointer to a [Collection] object or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    collection, err := pc.CreateCollection(ctx, &pinecone.CreateCollectionRequest{
//	        Name:   "my-collection",
//	        Source: "my-source-index",
//	    })
//	    if err != nil {
//		       log.Fatalf("Failed to create collection: %v", err)
//	    } else {
//		       fmt.Printf("Successfully created collection \"%s\".", collection.Name)
//	    }
func (c *Client) CreateCollection(ctx context.Context, in *CreateCollectionRequest) (*Collection, error) {
	if in.Source == "" || in.Name == "" {
		return nil, fmt.Errorf("fields Name and Source must be included in CreateCollectionRequest")
	}

	req := db_control.CreateCollectionRequest{
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

// [Client.DeleteCollection] deletes a specific [Collection]
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - collectionName: The name of the [Collection] to delete.
//
// Note: Collections are only available for pods-based Indexes.
//
// Returns an error if the deletion fails.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    collectionName := "my-collection"
//
//	    err = pc.DeleteCollection(ctx, collectionName)
//	    if err != nil {
//		       log.Fatalf("Failed to create collection: %s\n", err)
//	    } else {
//		       log.Printf("Successfully deleted collection \"%s\"\n", collectionName)
//	    }
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

// [InferenceService] is a struct which exposes methods for interacting with the Pinecone Inference API. [InferenceService]
// can be accessed via the Client object through the Client.Inference namespace.
//
// [Pinecone Inference API]: https://docs.pinecone.io/guides/inference/understanding-inference#embedding-models
type InferenceService struct {
	client *inference.Client
}

// [EmbedRequest] holds the parameters for generating embeddings for a list of input strings.
//
// Fields:
//   - Model: (Required) The model to use for generating embeddings.
//   - TextInputs: (Required) A list of strings to generate embeddings for.
//   - Parameters: (Optional) EmbedParameters object that contains additional parameters to use when generating embeddings.
type EmbedRequest struct {
	Model      string
	TextInputs []string
	Parameters EmbedParameters
}

// [EmbedParameters] contains model-specific parameters that can be used for generating embeddings.
//
// Fields:
//
//   - InputType: (Optional) A common property used to distinguish between different types of data. For example, "passage", or "query".
//
//   - Truncate: (Optional) How to handle inputs longer than those supported by the model. if "NONE", when the input exceeds
//     the maximum input token length, an error will be returned.
//
//     type EmbedParameters struct {
//     InputType string
//     Truncate  string
//     }
type EmbedParameters map[string]interface{}

// [EmbedResponse] represents holds the embeddings generated for a single input.
//
// Fields:
//   - Data: A list of [Embedding] objects containing the embeddings generated for the input.
//   - Model: The model used to generate the embeddings.
//   - Usage: Usage statistics ([Total Tokens]) for the request.
//
// [Total Tokens]: https://docs.pinecone.io/guides/organizations/manage-cost/understanding-cost#embed
type EmbedResponse struct {
	Data  []Embedding `json:"data"`
	Model string      `json:"model"`
	Usage struct {
		TotalTokens *int `json:"total_tokens,omitempty"`
	} `json:"usage"`
}

// [InferenceService.Embed] generates embeddings for a list of inputs using the specified model and (optional) parameters.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to an EmbedRequest object that contains the model to use for embedding generation, the
//     list of input strings to generate embeddings for, and any additional parameters to use for generation.
//
// Returns a pointer to an [EmbeddingsList] object or an error.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//		       ApiKey: "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err !=  nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    } else {
//		       fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    in := &pinecone.EmbedRequest{
//		       Model: "multilingual-e5-large",
//		       TextInputs: []string{"Who created the first computer?"},
//		       Parameters: pinecone.EmbedParameters{
//			       InputType: "passage",
//			       Truncate: "END",
//		       },
//	    }
//
//	    res, err := pc.Inference.Embed(ctx, in)
//	    if err != nil {
//		       log.Fatalf("Failed to embed: %v", err)
//	    } else {
//		       fmt.Printf("Successfull generated embeddings: %+v", res)
//	    }
func (i *InferenceService) Embed(ctx context.Context, in *EmbedRequest) (*EmbedResponse, error) {
	if len(in.TextInputs) == 0 {
		return nil, fmt.Errorf("TextInputs must contain at least one value")
	}

	// Convert text inputs to the expected type
	convertedInputs := make([]struct {
		Text *string `json:"text,omitempty"`
	}, len(in.TextInputs))
	for i, input := range in.TextInputs {
		convertedInputs[i] = struct {
			Text *string `json:"text,omitempty"`
		}{Text: &input}
	}

	req := inference.EmbedRequest{
		Model:  in.Model,
		Inputs: convertedInputs,
	}

	// convert embedding parameters to expected type
	if &in.Parameters != nil {
		params := map[string]interface{}(in.Parameters)
		req.Parameters = &params
	}

	res, err := i.client.Embed(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to embed: ")
	}

	return decodeEmbedResponse(res.Body)
}

// [Document] is a map representing the document to be reranked.
type Document map[string]interface{}

// [RerankRequest] holds the parameters for calling [InferenceService.Rerank] and reranking documents
// by a specified query and model.
//
// Fields:
//   - Model: "The [model] to use for reranking.
//   - Query: (Required) The query to rerank Documents against.
//   - Documents: (Required) A list of Document objects to be reranked. The default is "text", but you can
//     specify this behavior with [RerankRequest.RankFields].
//   - RankFields: (Optional) The fields to rank the Documents by. If not provided, the default is "text".
//   - ReturnDocuments: (Optional) Whether to include Documents in the response. Defaults to true.
//   - TopN: (Optional) How many Documents to return. Defaults to the length of input Documents.
//   - Parameters: (Optional) Additional model-specific parameters for the reranker
//
// [model]: https://docs.pinecone.io/guides/inference/understanding-inference#models
type RerankRequest struct {
	Model           string
	Query           string
	Documents       []Document
	RankFields      *[]string
	ReturnDocuments *bool
	TopN            *int
	Parameters      *map[string]interface{}
}

// Represents a ranked document with a relevance score and an index position.
//
// Fields:
//   - Document: The [Document].
//   - Index: The index position of the Document from the original request. This can be used
//     to locate the position of the document relative to others described in the request.
//   - Score: The relevance score of the Document indicating how closely it matches the query.
type RankedDocument struct {
	Document *Document `json:"document,omitempty"`
	Index    int       `json:"index"`
	Score    float32   `json:"score"`
}

// [RerankResponse] is the result of a reranking operation.
//
// Fields:
//   - Data: A list of [RankedDocument] objects which have been reranked. The RankedDocuments are sorted in order of relevance,
//     with the first being the most relevant.
//   - Model: The model used to rerank documents.
//   - Usage: Usage statistics ([Rerank Units]) for the reranking operation.
//
// [Read Units]: https://docs.pinecone.io/guides/organizations/manage-cost/understanding-cost#rerank
type RerankResponse struct {
	Data  []RankedDocument `json:"data,omitempty"`
	Model string           `json:"model"`
	Usage RerankUsage      `json:"usage"`
}

// [InferenceService.Rerank] reranks documents with associated relevance scores that represent the relevance of each [Document]
// to the provided query using the specified model.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [RerankRequest] object that contains the model, query, and documents to use for reranking.
//
// Example:
//
//	     ctx := context.Background()
//
//	     clientParams := pinecone.NewClientParams{
//		        ApiKey:    "YOUR_API_KEY",
//		        SourceTag: "your_source_identifier", // optional
//	     }
//
//	     pc, err := pinecone.NewClient(clientParams)
//	     if err != nil {
//		        log.Fatalf("Failed to create Client: %v", err)
//	     }
//
//	     rerankModel := "bge-reranker-v2-m3"
//	     topN := 2
//	     retunDocuments := true
//	     documents := []pinecone.Document{
//		        {"id": "doc1", "text": "Apple is a popular fruit known for its sweetness and crisp texture."},
//		        {"id": "doc2", "text": "Many people enjoy eating apples as a healthy snack."},
//		        {"id": "doc3", "text": "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces."},
//		        {"id": "doc4", "text": "An apple a day keeps the doctor away, as the saying goes."},
//	     }
//
//	     ranking, err := pc.Inference.Rerank(ctx, &pinecone.RerankRequest{
//		        Model:           rerankModel,
//		        Query:           "i love to eat apples",
//		        ReturnDocuments: &retunDocuments,
//		        TopN:            &topN,
//		        RankFields:      &[]string{"text"},
//		        Documents:       documents,
//	     })
//	     if err != nil {
//		        log.Fatalf("Failed to rerank: %v", err)
//	     }
//	     fmt.Printf("Rerank result: %+v\n", ranking)
func (i *InferenceService) Rerank(ctx context.Context, in *RerankRequest) (*RerankResponse, error) {
	convertedDocuments := make([]inference.Document, len(in.Documents))
	for i, doc := range in.Documents {
		convertedDocuments[i] = inference.Document(doc)
	}
	req := inference.RerankJSONRequestBody{
		Model:           in.Model,
		Query:           in.Query,
		Documents:       convertedDocuments,
		RankFields:      in.RankFields,
		ReturnDocuments: in.ReturnDocuments,
		TopN:            in.TopN,
		Parameters:      in.Parameters,
	}
	res, err := i.client.Rerank(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to rerank: ")
	}
	return decodeRerankResponse(res.Body)
}

func (i *InferenceService) GetModel(ctx context.Context, modelName string) (*inference.ModelInfo, error) {
	res, err := i.client.GetModel(ctx, modelName)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to get model: ")
	}
	var modelInfo inference.ModelInfo
	err = json.NewDecoder(res.Body).Decode(&modelInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to decode model info response: %w", err)
	}
	return &modelInfo, nil
}

func (c *Client) extractAuthHeader() map[string]string {
	possibleAuthKeys := []string{
		"api-key",
		"authorization",
		"access_token",
	}

	for key, value := range c.baseParams.Headers {
		for _, checkKey := range possibleAuthKeys {
			if strings.ToLower(key) == checkKey {
				return map[string]string{key: value}
			}
		}
	}

	return nil
}

func toIndex(idx *db_control.IndexModel) *Index {
	if idx == nil {
		return nil
	}

	spec := &IndexSpec{}
	if idx.Spec.Pod != nil {
		spec.Pod = &PodSpec{
			Environment:      idx.Spec.Pod.Environment,
			PodType:          idx.Spec.Pod.PodType,
			PodCount:         derefOrDefault(idx.Spec.Pod.Pods, 1),
			Replicas:         derefOrDefault(idx.Spec.Pod.Replicas, 1),
			ShardCount:       derefOrDefault(idx.Spec.Pod.Shards, 1),
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
	var embed *IndexEmbed
	if idx.Embed != nil {
		var metric *IndexMetric
		if idx.Embed.Metric != nil {
			convertedMetric := IndexMetric(*idx.Embed.Metric)
			metric = &convertedMetric
		}

		embed = &IndexEmbed{
			Dimension:       idx.Embed.Dimension,
			FieldMap:        idx.Embed.FieldMap,
			Metric:          metric,
			Model:           idx.Embed.Model,
			ReadParameters:  idx.Embed.ReadParameters,
			VectorType:      idx.Embed.VectorType,
			WriteParameters: idx.Embed.WriteParameters,
		}
	}

	tags := (*IndexTags)(idx.Tags)
	deletionProtection := derefOrDefault(idx.DeletionProtection, "disabled")

	return &Index{
		Name:               idx.Name,
		Host:               idx.Host,
		Metric:             IndexMetric(idx.Metric),
		VectorType:         idx.VectorType,
		DeletionProtection: DeletionProtection(deletionProtection),
		Dimension:          idx.Dimension,
		Spec:               spec,
		Status:             status,
		Tags:               tags,
		Embed:              embed,
	}
}

func decodeIndex(resBody io.ReadCloser) (*Index, error) {
	var idx db_control.IndexModel
	err := json.NewDecoder(resBody).Decode(&idx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode idx response: %w", err)
	}

	return toIndex(&idx), nil
}

func decodeEmbedResponse(resBody io.ReadCloser) (*EmbedResponse, error) {
	var rawEmbedResponse struct {
		Data  []json.RawMessage `json:"data"`
		Model string            `json:"model"`
		Usage struct {
			TotalTokens *int `json:"total_tokens,omitempty"`
		}
	}
	if err := json.NewDecoder(resBody).Decode(&rawEmbedResponse); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	decodedEmbeddings := make([]Embedding, len(rawEmbedResponse.Data))
	for i, embedding := range rawEmbedResponse.Data {
		var vectorTypeCheck struct {
			VectorType string `json:"vector_type"`
		}
		if err := json.Unmarshal(embedding, &vectorTypeCheck); err != nil {
			return nil, fmt.Errorf("failed to decode VectorType check: %w", err)
		}

		switch vectorTypeCheck.VectorType {
		case "sparse":
			var sparseEmbedding SparseEmbedding
			if err := json.Unmarshal(embedding, &sparseEmbedding); err != nil {
				return nil, fmt.Errorf("failed to decode sparse embedding: %w", err)
			}
			decodedEmbeddings[i] = Embedding{SparseEmbedding: &sparseEmbedding}
		case "dense":
			var denseEmbedding DenseEmbedding
			if err := json.Unmarshal(embedding, &denseEmbedding); err != nil {
				return nil, fmt.Errorf("failed to decode dense embedding: %w", err)
			}
			decodedEmbeddings[i] = Embedding{DenseEmbedding: &denseEmbedding}
		default:
			return nil, fmt.Errorf("unsupported VectorType: %s", vectorTypeCheck.VectorType)
		}
	}

	return &EmbedResponse{
		Data:  decodedEmbeddings,
		Model: rawEmbedResponse.Model,
		Usage: rawEmbedResponse.Usage,
	}, nil
}

func decodeRerankResponse(resBody io.ReadCloser) (*RerankResponse, error) {
	var rerankResponse RerankResponse
	err := json.NewDecoder(resBody).Decode(&rerankResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode rerank response: %w", err)
	}

	return &rerankResponse, nil
}

func toCollection(cm *db_control.CollectionModel) *Collection {
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
	var collectionModel db_control.CollectionModel
	err := json.NewDecoder(resBody).Decode(&collectionModel)
	if err != nil {
		return nil, fmt.Errorf("failed to decode collection response: %w", err)
	}

	return toCollection(&collectionModel), nil
}

func decodeErrorResponse(resBodyBytes []byte) (*db_control.ErrorResponse, error) {
	var errorResponse db_control.ErrorResponse
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

func buildClientBaseOptions(in NewClientBaseParams) []db_control.ClientOption {
	clientOptions := []db_control.ClientOption{}
	headerProviders := buildSharedProviderHeaders(in)

	for _, provider := range headerProviders {
		clientOptions = append(clientOptions, db_control.WithRequestEditorFn(provider.Intercept))
	}

	// apply custom http client if provided
	if in.RestClient != nil {
		clientOptions = append(clientOptions, db_control.WithHTTPClient(in.RestClient))
	}

	return clientOptions
}

func buildInferenceBaseOptions(in NewClientBaseParams) []inference.ClientOption {
	clientOptions := []inference.ClientOption{}
	headerProviders := buildSharedProviderHeaders(in)

	for _, provider := range headerProviders {
		clientOptions = append(clientOptions, inference.WithRequestEditorFn(provider.Intercept))
	}

	// apply custom http client if provided
	if in.RestClient != nil {
		clientOptions = append(clientOptions, inference.WithHTTPClient(in.RestClient))
	}

	return clientOptions
}

func buildDataClientBaseOptions(in NewClientBaseParams) []db_data_rest.ClientOption {
	clientOptions := []db_data_rest.ClientOption{}
	headerProviders := buildSharedProviderHeaders(in)

	for _, provider := range headerProviders {
		clientOptions = append(clientOptions, db_data_rest.WithRequestEditorFn(provider.Intercept))
	}

	// apply custom http client if provided
	if in.RestClient != nil {
		clientOptions = append(clientOptions, db_data_rest.WithHTTPClient(in.RestClient))
	}

	return clientOptions
}

func buildSharedProviderHeaders(in NewClientBaseParams) []*provider.CustomHeader {
	providers := []*provider.CustomHeader{}

	// build and apply user agent header
	providers = append(providers, provider.NewHeaderProvider("User-Agent", useragent.BuildUserAgent(in.SourceTag)))
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
		for key, value := range in.Headers {
			additionalHeaders[key] = value
		}
	}
	// create header providers
	for key, value := range additionalHeaders {
		providers = append(providers, provider.NewHeaderProvider(key, value))
	}

	return providers
}

func mergeIndexTags(existingTags *IndexTags, newTags IndexTags) *IndexTags {
	if existingTags == nil || *existingTags == nil {
		existingTags = &IndexTags{}
	}
	merged := make(IndexTags)

	// Copy existing tags
	for key, value := range *existingTags {
		merged[key] = value
	}

	// Merge new tags
	for key, value := range newTags {
		merged[key] = value
	}

	return &merged
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

func pointerOrNil[T comparable](value T) *T {
	var zero T // set to zero-value of generic type T
	if value == zero {
		return nil
	}
	return &value
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
