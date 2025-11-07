// Package pinecone provides a client for the [Pinecone managed vector database].
//
// [Pinecone managed vector database]: https://www.pinecone.io/
package pinecone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pinecone-io/go-pinecone/v4/internal/gen"
	"github.com/pinecone-io/go-pinecone/v4/internal/gen/db_control"
	db_data_rest "github.com/pinecone-io/go-pinecone/v4/internal/gen/db_data/rest"
	"github.com/pinecone-io/go-pinecone/v4/internal/gen/inference"
	"github.com/pinecone-io/go-pinecone/v4/internal/provider"
	"github.com/pinecone-io/go-pinecone/v4/internal/useragent"
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
	Namespace          string            // optional - if not provided the default namespace of "__default__" will be used
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

	if in.Namespace == "" {
		in.Namespace = "__default__"
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
	if strings.HasPrefix(host, "http://") {
		return strings.Replace(host, "http://", "https://", 1)
	} else if !strings.HasPrefix(host, "https://") {
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
// Returns a slice of pointers to [Index] objects or an error.
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
	res, err := c.restClient.ListIndexes(ctx, &db_control.ListIndexesParams{XPineconeApiVersion: gen.PineconeApiVersion})
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

	var indexes []*Index
	if indexList.Indexes != nil {
		indexes = make([]*Index, len(*indexList.Indexes))
		for i, idx := range *indexList.Indexes {
			index, err := toIndex(&idx)
			if err != nil {
				return nil, err
			}
			indexes[i] = index
		}
	} else {
		indexes = make([]*Index, 0)
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
		Metric:             (*string)(in.Metric),
		DeletionProtection: deletionProtection,
		Tags:               tags,
		VectorType:         &vectorType,
	}

	podSpec := db_control.IndexSpec1{
		Pod: db_control.PodSpec{
			Environment:      in.Environment,
			PodType:          in.PodType,
			Pods:             &pods,
			Replicas:         &replicas,
			Shards:           &shards,
			SourceCollection: in.SourceCollection,
		}}

	if in.MetadataConfig != nil {
		podSpec.Pod.MetadataConfig = &struct {
			Indexed *[]string `json:"indexed,omitempty"`
		}{
			Indexed: in.MetadataConfig.Indexed,
		}
	}

	err := req.Spec.FromIndexSpec1(podSpec)
	if err != nil {
		return nil, err
	}

	res, err := c.restClient.CreateIndex(ctx, &db_control.CreateIndexParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
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
//   - ReadCapacity: (Optional) The read capacity configuration for the serverless index. Used to configure dedicated read capacity
//     with specific node types and scaling strategies.
//   - Schema: (Optional) Schema for the behavior of Pinecone's internal metadata index. By default, all metadata is indexed.
//   - Tags: (Optional) A map of tags to associate with the Index.
//   - SourceCollection: (Optional) The name of the [Collection] to use as the source for the index. NOTE: Collections can only be created
//     from pods-based indexes.
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
	ReadCapacity       *ReadCapacityRequest
	Schema             *MetadataSchema
	Tags               *IndexTags
	SourceCollection   *string
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

	readCapacity, err := readCapacityRequestToReadCapacity(in.ReadCapacity)
	if err != nil {
		return nil, err
	}

	serverlessSpec := db_control.IndexSpec0{
		Serverless: db_control.ServerlessSpec{
			Cloud:            string(in.Cloud),
			Region:           in.Region,
			SourceCollection: in.SourceCollection,
			Schema:           fromMetadataSchemaToRest(in.Schema),
			ReadCapacity:     readCapacity,
		},
	}

	req := db_control.CreateIndexRequest{
		Name:               in.Name,
		Dimension:          in.Dimension,
		Metric:             (*string)(in.Metric),
		DeletionProtection: deletionProtection,
		VectorType:         vectorType,
		Tags:               tags,
	}

	err = req.Spec.FromIndexSpec0(serverlessSpec)
	if err != nil {
		return nil, err
	}

	res, err := c.restClient.CreateIndex(ctx, &db_control.CreateIndexParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
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
//   - ReadCapacity: (Optional) The read capacity configuration for the serverless index. Used to configure dedicated read capacity
//     with specific node types and scaling strategies.
//   - Schema: (Optional) Schema for the behavior of Pinecone's internal metadata index. By default, all metadata is indexed.
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
	ReadCapacity       *ReadCapacityRequest
	Schema             *MetadataSchema
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
//	    idx, err := pc.CreateIndexForModel(ctx, &pinecone.CreateIndexForModelRequest{
//		    Name:    indexName,
//		    Dimension: 3,
//		    Cloud:   pinecone.Aws,
//		    Region:  "us-east-1",
//		    Embed: pinecone.CreateIndexForModelEmbed{
//			    Model:    "multilingual-e5-large",
//			    FieldMap: map[string]interface{}{"text": "chunk_text"},
//			},
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

	readCapacity, err := readCapacityRequestToReadCapacity(in.ReadCapacity)
	if err != nil {
		return nil, err
	}

	req := db_control.CreateIndexForModelRequest{
		Name:   in.Name,
		Region: in.Region,
		Cloud:  string(in.Cloud),
		Embed: struct {
			Dimension       *int                    `json:"dimension,omitempty"`
			FieldMap        map[string]interface{}  `json:"field_map"`
			Metric          *string                 `json:"metric,omitempty"`
			Model           string                  `json:"model"`
			ReadParameters  *map[string]interface{} `json:"read_parameters,omitempty"`
			WriteParameters *map[string]interface{} `json:"write_parameters,omitempty"`
		}{
			Dimension:       in.Embed.Dimension,
			FieldMap:        in.Embed.FieldMap,
			Metric:          (*string)(in.Embed.Metric),
			Model:           in.Embed.Model,
			ReadParameters:  in.Embed.ReadParameters,
			WriteParameters: in.Embed.WriteParameters,
		},
		DeletionProtection: (*db_control.DeletionProtection)(&deletionProtection),
		Schema:             fromMetadataSchemaToRest(in.Schema),
		ReadCapacity:       readCapacity,
		Tags:               tags,
	}

	res, err := c.restClient.CreateIndexForModel(ctx, &db_control.CreateIndexForModelParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
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
	res, err := c.restClient.DescribeIndex(ctx, idxName, &db_control.DescribeIndexParams{XPineconeApiVersion: gen.PineconeApiVersion})
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
	res, err := c.restClient.DeleteIndex(ctx, idxName, &db_control.DeleteIndexParams{XPineconeApiVersion: gen.PineconeApiVersion})
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
	ReadCapacity       *ReadCapacityRequest
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
	if in.PodType == "" && in.Replicas == 0 && in.DeletionProtection == "" && in.Tags == nil && in.ReadCapacity == nil {
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
	request.Spec = &db_control.ConfigureIndexRequest_Spec{}

	// Apply pod configurations
	if podType != nil || replicas != nil {
		podSpec := db_control.ConfigureIndexRequestSpec1{
			Pod: struct {
				PodType  *string `json:"pod_type,omitempty"`
				Replicas *int32  `json:"replicas,omitempty"`
			}{
				PodType:  podType,
				Replicas: replicas,
			},
		}

		// Apply the pod spec to the request
		if err := request.Spec.FromConfigureIndexRequestSpec1(podSpec); err != nil {
			return nil, err
		}
	}

	// Apply serverless configurations
	if in.ReadCapacity != nil {
		var readCapacity *db_control.ReadCapacity
		readCapacity, err = readCapacityRequestToReadCapacity(in.ReadCapacity)
		if err != nil {
			return nil, err
		}
		serverLessSpec := db_control.ConfigureIndexRequestSpec0{
			Serverless: struct {
				ReadCapacity *db_control.ReadCapacity `json:"read_capacity,omitempty"`
			}{
				ReadCapacity: readCapacity,
			},
		}
		// Apply the serverless spec to the request
		if err := request.Spec.FromConfigureIndexRequestSpec0(serverLessSpec); err != nil {
			return nil, err
		}
	}

	// Apply embedding configurations
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

	res, err := c.restClient.ConfigureIndex(ctx, name, &db_control.ConfigureIndexParams{XPineconeApiVersion: gen.PineconeApiVersion}, request)
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
	res, err := c.restClient.ListCollections(ctx, &db_control.ListCollectionsParams{XPineconeApiVersion: gen.PineconeApiVersion})
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
	res, err := c.restClient.DescribeCollection(ctx, collectionName, &db_control.DescribeCollectionParams{XPineconeApiVersion: gen.PineconeApiVersion})
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
	res, err := c.restClient.CreateCollection(ctx, &db_control.CreateCollectionParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)

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
	res, err := c.restClient.DeleteCollection(ctx, collectionName, &db_control.DeleteCollectionParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete collection: ")
	}

	return nil
}

// [CreateBackupParams] contains the input parameters for creating a backup of a Pinecone index.
//
// Fields:
//   - IndexName: The unique name of the index to back up.
//   - Description: Optional description of the backup.
//   - Name: Optional name for the backup.
type CreateBackupParams struct {
	IndexName   string  `json:"index_name"`
	Description *string `json:"description,omitempty"`
	Name        *string `json:"name,omitempty"`
}

// [Client.CreateBackup] creates a [Backup] for an index.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateBackupParams] object.
//
// Note: Backups are only available for serverless Indexes.
//
// Returns a pointer to a [Backup] object or an error.
//
// Example:
//
//		 ctx := context.Background()
//
//		 clientParams := pinecone.NewClientParams{
//			    ApiKey:    "YOUR_API_KEY",
//		 }
//
//		 pc, err := pinecone.NewClient(clientParams)
//		 if err != nil {
//		        log.Fatalf("Failed to create Client: %v", err)
//		 } else {
//			    fmt.Println("Successfully created a new Client object!")
//		 }
//
//	     index, err := pc.DescribeIndex(ctx, "my-index")
//		 if err != nil {
//			    log.Fatalf("Failed to describe index: %v", err)
//		 }
//
//	     backupDesc := fmt.Sprintf("%s-backup", index.Name)
//	     backupName := "my-backup"
//		 backup, err := pc.CreateBackup(ctx, &pinecone.CreateBackupParams{
//		        IndexName:   index.Name,
//		        Name: &backupName,
//		        Description: &backupDesc,
//		 })
//		 if err != nil {
//			    log.Fatalf("Failed to create collection: %v", err)
//		 } else {
//			    fmt.Printf("Successfully created backup \"%s\" of index \"%s\".", backup.BackupId, index.Name)
//		 }
func (c *Client) CreateBackup(ctx context.Context, in *CreateBackupParams) (*Backup, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateBackupRequest) cannot be nil")
	}
	if in.IndexName == "" {
		return nil, fmt.Errorf("IndexName must be included in CreateBackupRequest")
	}

	res, err := c.restClient.CreateBackup(ctx, in.IndexName, &db_control.CreateBackupParams{XPineconeApiVersion: gen.PineconeApiVersion}, db_control.CreateBackupRequest{
		Description: in.Description,
		Name:        in.Name,
	})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to create backup: ")
	}

	return decodeBackup(res.Body)
}

// [CreateIndexFromBackupParams] contains the parameters needed to create a Pinecone index from a backup.
//
// Fields:
//   - BackupId: The unique identifier of the backup to restore from.
//   - Name: The name of the index to be created. Must be 1–45 characters, lowercase alphanumeric or '-'.
//   - DeletionProtection: Optional value configuring deletion protection for the new index. Can be either 'enabled' or 'disabled'.
//   - Tags: Optional custom user tags added to an index. Keys must be 80 characters or less. Values must be 120 characters or less. Keys must be alphanumeric, '_', or '-'.  Values must be alphanumeric, ';', '@', '_', '-', '.', '+', or ' '. To unset a key, set the value to be an empty string.
type CreateIndexFromBackupParams struct {
	BackupId           string              `json:"backup_id"`
	Name               string              `json:"name"`
	DeletionProtection *DeletionProtection `json:"deletion_protection,omitempty"`
	Tags               *IndexTags          `json:"tags,omitempty"`
}

// [CreateIndexFromBackupResponse] contains the response returned after creating an index from a backup. RestoreJobId can be used
// to track the progress of an index restoration through the [Client.DescribeRestoreJob] method.
//
// Fields:
//   - IndexId: The ID of the index that was created from the backup.
//   - RestoreJobId: The ID of the restore job initiated to restore the backup.
type CreateIndexFromBackupResponse struct {
	IndexId      string `json:"index_id"`
	RestoreJobId string `json:"restore_job_id"`
}

// [Client.CreateIndexFromBackup] creates a new [Index] from a [Backup].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateIndexFromBackupParams] object.
//
// Note: Backups are only available for serverless Indexes.
//
// Returns a pointer to a [CreateIndexFromBackupResponse] object or an error.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		       ApiKey: "YOUR_API_KEY",
//	    })
//	    if err != nil {
//			   log.Fatalf("Failed to create Client: %v", err)
//		} else {
//			   fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    createIndexFromBackupResp, err := pc.CreateIndexFromBackup(ctx, &pinecone.CreateIndexFromBackupParams{
//			   BackupId: "my-backup-id",
//			   Name:     "my-new-index-restored",
//		})
//		if err != nil {
//			   log.Fatalf("Failed to create a new index from a backup: %v", err)
//		}
//
//	    // retrieve the restore job
//	    restoreJob, err := pc.DescribeRestoreJob(ctx, createIndexFromBackupResp.RestoreJobId)
//	    if err != nil {
//	      	   log.Fatalf("Failed to describe restore job: %v", err)
//	    }
func (c *Client) CreateIndexFromBackup(ctx context.Context, in *CreateIndexFromBackupParams) (*CreateIndexFromBackupResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateIndexFromBackupRequest) cannot be nil")
	}
	if in.BackupId == "" {
		return nil, fmt.Errorf("BackupId must be included in CreateIndexFromBackupRequest")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("Name must be included in CreateIndexFromBackupRequest")
	}

	res, err := c.restClient.CreateIndexFromBackupOperation(ctx, in.BackupId, &db_control.CreateIndexFromBackupOperationParams{XPineconeApiVersion: gen.PineconeApiVersion}, db_control.CreateIndexFromBackupRequest{
		Name:               in.Name,
		DeletionProtection: (*db_control.DeletionProtection)(in.DeletionProtection),
		Tags:               (*db_control.IndexTags)(in.Tags),
	})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return nil, handleErrorResponseBody(res, "failed to create index from backup: ")
	}

	var response *db_control.CreateIndexFromBackupResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}
	if response == nil {
		return nil, nil
	}
	return &CreateIndexFromBackupResponse{
		IndexId:      response.IndexId,
		RestoreJobId: response.RestoreJobId,
	}, nil
}

// [Client.DescribeBackup] describes a specific [Backup] by ID.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A string representing the ID of the [Backup] to describe.
//
// Returns a pointer to a [Backup] object or an error.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		       ApiKey: "YOUR_API_KEY",
//	    })
//	    if err != nil {
//			   log.Fatalf("Failed to create Client: %v", err)
//		} else {
//			   fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    backup, err := pc.DescribeBackup(ctx, "my-backup-id")
//		if err != nil {
//			   log.Fatalf("Failed to describe backup ID %s: %w", "my-backup-id", err)
//		}
func (c *Client) DescribeBackup(ctx context.Context, backupId string) (*Backup, error) {
	if backupId == "" {
		return nil, fmt.Errorf("you must provide a backupId to describe a backup")
	}

	res, err := c.restClient.DescribeBackup(ctx, backupId, &db_control.DescribeBackupParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe backup: ")
	}

	return decodeBackup(res.Body)
}

// [ListBackupsParams] contains the query parameters used when listing backups.
//
// Fields:
//   - IndexName: Optional filter to list backups for a specific index. Otherwise, all backups in the project will be listed.
//   - Limit: Optional maximum number of backups to return.
//   - PaginationToken: Optional token to retrieve the next page of results. Will be nil if there are no more results.
type ListBackupsParams struct {
	IndexName       *string `json:"index_name,omitempty"`
	Limit           *int    `json:"limit,omitempty"`
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// [Client.ListBackups] lists backups for a specific [Index], or all of the backups in a Pinecone project.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [ListBackupsParams] object.
//
// Returns a pointer to a [BackupList] object or an error.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//	           ApiKey: "YOUR_API_KEY",
//		})
//		if err != nil {
//			   log.Fatalf("Failed to create Client: %w", err)
//		}
//
//	    indexName := "my-index"
//	    limit := 5
//		backups, err := pc.ListBackups(ctx, &pinecone.ListBackupsParams{
//	           IndexName: &indexName,
//	           Limit: &limit,
//	    })
//	    if err != nil {
//			   log.Fatalf("Failed to list backups: %w", err)
//		}
func (c *Client) ListBackups(ctx context.Context, in *ListBackupsParams) (*BackupList, error) {
	var response *http.Response
	var err error
	if in == nil {
		response, err = c.restClient.ListProjectBackups(ctx, nil)
		if err != nil {
			return nil, err
		}
	} else if in.IndexName == nil {
		response, err = c.restClient.ListProjectBackups(ctx, &db_control.ListProjectBackupsParams{
			Limit:           in.Limit,
			PaginationToken: in.PaginationToken,
		})
		if err != nil {
			return nil, err
		}
	} else {
		response, err = c.restClient.ListIndexBackups(ctx, *in.IndexName, &db_control.ListIndexBackupsParams{
			Limit:           in.Limit,
			PaginationToken: in.PaginationToken,
		})
		if err != nil {
			return nil, err
		}
	}

	if response.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(response, "failed to list backups: ")
	}
	return decodeBackupList(response.Body)
}

// [Client.DeleteBackup] deletes a specific [Backup] by ID.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A string representing the ID of the [Backup] to delete.
//
// Returns an error if the deletion fails.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//	           ApiKey: "YOUR_API_KEY",
//		})
//		if err != nil {
//			   log.Fatalf("Failed to create Client: %w", err)
//		}
//
//		err := pc.DeleteBackup(ctx, "my-backup-id"))
//	    if err != nil {
//			   log.Fatalf("Failed to delete backup: %w", err)
//		}
func (c *Client) DeleteBackup(ctx context.Context, backupId string) error {
	if backupId == "" {
		return fmt.Errorf("you must provide a backupId to delete a backup")
	}

	res, err := c.restClient.DeleteBackup(ctx, backupId, &db_control.DeleteBackupParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return handleErrorResponseBody(res, "failed to delete backup: ")
	}

	return nil
}

// [Client.DescribeRestoreJob] describes a specific [RestoreJob] by ID.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A string representing the ID of the [Backup] to describe.
//
// Returns a pointer to a [Backup] object or an error.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		       ApiKey: "YOUR_API_KEY",
//	    })
//	    if err != nil {
//			   log.Fatalf("Failed to create Client: %v", err)
//		} else {
//			   fmt.Println("Successfully created a new Client object!")
//	    }
//
//	    restoreJob, err := pc.DescribeRestoreJob(ctx, "my-restore-job-id")
//		if err != nil {
//			   log.Fatalf("Failed to describe restore job ID %s: %w", "my-restore-job-id", err)
//		}
func (c *Client) DescribeRestoreJob(ctx context.Context, restoreJobId string) (*RestoreJob, error) {
	if restoreJobId == "" {
		return nil, fmt.Errorf("you must provide a restoreJobId to describe a restore job")
	}

	res, err := c.restClient.DescribeRestoreJob(ctx, restoreJobId, &db_control.DescribeRestoreJobParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe restore job: ")
	}

	return decodeRestoreJob(res.Body)
}

// [ListRestoreJobsParams] contains the query parameters used when listing restore jobs.
//
// Fields:
//   - Limit: Optional maximum number of restore jobs to return.
//   - PaginationToken: Optional token to retrieve the next page of results. Will be nil if there are no more results.
type ListRestoreJobsParams struct {
	Limit           *int    `json:"limit,omitempty"`
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// [Client.ListRestoreJobs] lists all restore jobs in a Pinecone project.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [ListRestoreJobsParams] object.
//
// Returns a pointer to a [RestoreJobList] object or an error.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//	           ApiKey: "YOUR_API_KEY",
//		})
//		if err != nil {
//			   log.Fatalf("Failed to create Client: %w", err)
//		}
//
//	    indexName := "my-index"
//	    limit := 5
//		restoreJobs, err := pc.ListRestoreJobs(ctx, &pinecone.ListRestoreJobsParams{
//	           IndexName: &indexName,
//	           Limit: &limit,
//	    })
//	    if err != nil {
//			   log.Fatalf("Failed to list restore jobs: %w", err)
//		}
func (c *Client) ListRestoreJobs(ctx context.Context, in *ListRestoreJobsParams) (*RestoreJobList, error) {
	var response *http.Response
	var err error
	if in == nil {
		response, err = c.restClient.ListRestoreJobs(ctx, nil)
		if err != nil {
			return nil, err
		}
	} else {

		response, err = c.restClient.ListRestoreJobs(ctx, &db_control.ListRestoreJobsParams{
			Limit:           in.Limit,
			PaginationToken: in.PaginationToken,
		})
		if err != nil {
			return nil, err
		}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(response, "failed to list restore jobs: ")
	}

	return decodeRestoreJobList(response.Body)
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
//   - InputType: (Optional) A common property used to distinguish between different types of data. For example, "passage", or "query".
//   - Truncate: (Optional) How to handle inputs longer than those supported by the model. if "NONE", when the input exceeds
//     the maximum input token length, an error will be returned.
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
		TotalTokens *int32 `json:"total_tokens,omitempty"`
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
	if in.Parameters != nil {
		params := map[string]interface{}(in.Parameters)
		req.Parameters = &params
	}

	res, err := i.client.Embed(ctx, &inference.EmbedParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
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
//   - Model: (Required) The [model] to use for reranking.
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

// [RankedDocument] represents a ranked document with a relevance score and an index position.
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
// Returns a pointer to a [RerankResponse] object or an error.
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
	res, err := i.client.Rerank(ctx, &inference.RerankParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to rerank: ")
	}
	return decodeRerankResponse(res.Body)
}

// [InferenceService.DescribeModel] gets a description of a model hosted by Pinecone.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - modelName: The name of the model to retrieve information about.
//
// Returns a pointer to a [ModelInfo] object or an error.
//
// Example:
//
//	     ctx := context.Background()
//
//		 clientParams := pinecone.NewClientParams{
//			    ApiKey:    "YOUR_API_KEY",
//			    SourceTag: "your_source_identifier", // optional
//		 }
//
//	     pc, err := pinecone.NewClient(clientParams)
//		 if err != nil {
//			    log.Fatalf("Failed to create Client: %v", err)
//		 }
//
//	     model, err := pc.Inference.DescribeModel(ctx, "multilingual-e5-large")
//		 if err != nil {
//			    log.Fatalf("Failed to get model: %v", err)
//		 }
//
//	     fmt.Printf("Model (multilingual-e5-large): %+v\n", model)
func (i *InferenceService) DescribeModel(ctx context.Context, modelName string) (*ModelInfo, error) {
	res, err := i.client.GetModel(ctx, modelName, &inference.GetModelParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to get model: ")
	}
	var modelInfo ModelInfo
	err = json.NewDecoder(res.Body).Decode(&modelInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to decode model info response: %w", err)
	}
	return &modelInfo, nil
}

// [ListModelsParams] holds the parameters for filtering model results when calling [InferenceService.ListModels].
//
// Fields:
//   - Type: (Optional) The type of model to filter by. Can be either "embed" or "rerank".
//   - VectorType: (Optional) The vector type of the model to filter by. Can be either "dense" or "sparse".
//     Only relevant if Type is "embed".
type ListModelsParams struct {
	Type       *string
	VectorType *string
}

// [InferenceService.ListModels] lists all available models hosted by Pinecone. You can filter results using [ListModelsParams].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: The name of the model to retrieve information about.
//
// Returns a pointer to a [ModelInfoList] object or an error.
//
// Example:
//
//		 ctx := context.Background()
//
//		 clientParams := pinecone.NewClientParams{
//		        ApiKey:    "YOUR_API_KEY",
//		 		SourceTag: "your_source_identifier", // optional
//	     }
//
//		 pc, err := pinecone.NewClient(clientParams)
//	     if err != nil {
//		        log.Fatalf("Failed to create Client: %v", err)
//		 }
//
//	     embed := "embed"
//		 embedModels, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{ Type: &embed })
//	     if err != nil {
//		        log.Fatalf("Failed to list models: %v", err)
//		 }
//
//		 fmt.Printf("Embed Models: %+v\n", embedModels)
func (i *InferenceService) ListModels(ctx context.Context, in *ListModelsParams) (*ModelInfoList, error) {
	var params *inference.ListModelsParams
	if in != nil {
		params = &inference.ListModelsParams{
			Type:       in.Type,
			VectorType: in.VectorType,
		}
	}

	res, err := i.client.ListModels(ctx, params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list models: ")
	}
	var modelInfoList ModelInfoList
	err = json.NewDecoder(res.Body).Decode(&modelInfoList)
	if err != nil {
		return nil, fmt.Errorf("failed to decode model info list response: %w", err)
	}
	return &modelInfoList, nil
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

func getIndexSpecType(spec db_control.IndexModel_Spec) string {
	rawJSON, err := spec.MarshalJSON()
	if err != nil {
		return "unknown"
	}
	var rawData map[string]interface{}
	err = json.Unmarshal(rawJSON, &rawData)
	if err != nil {
		return "unknown"
	}
	if _, ok := rawData["pod"]; ok {
		return "pod"
	} else if _, ok := rawData["serverless"]; ok {
		return "serverless"
	} else if _, ok := rawData["byoc"]; ok {
		return "byoc"
	}
	return "unknown"
}

func toIndex(idx *db_control.IndexModel) (*Index, error) {
	if idx == nil {
		return nil, nil
	}

	spec := &IndexSpec{}
	specType := getIndexSpecType(idx.Spec)

	switch specType {
	case "pod":
		if podSpec, err := idx.Spec.AsIndexModelSpec1(); err == nil {
			spec.Pod = &PodSpec{
				Environment:      podSpec.Pod.Environment,
				PodType:          podSpec.Pod.PodType,
				PodCount:         derefOrDefault(podSpec.Pod.Pods, 1),
				Replicas:         derefOrDefault(podSpec.Pod.Replicas, 1),
				ShardCount:       derefOrDefault(podSpec.Pod.Shards, 1),
				SourceCollection: podSpec.Pod.SourceCollection,
			}
			if podSpec.Pod.MetadataConfig != nil {
				spec.Pod.MetadataConfig = &PodSpecMetadataConfig{Indexed: podSpec.Pod.MetadataConfig.Indexed}
			}
		}
	case "serverless":
		if serverlessSpec, err := idx.Spec.AsIndexModelSpec0(); err == nil {
			readCapacity, err := toReadCapacity(&serverlessSpec.Serverless.ReadCapacity)
			if err != nil {
				return nil, err
			}
			spec.Serverless = &ServerlessSpec{
				Cloud:            Cloud(serverlessSpec.Serverless.Cloud),
				Region:           serverlessSpec.Serverless.Region,
				SourceCollection: serverlessSpec.Serverless.SourceCollection,
				Schema:           toMetadataSchemaFromRest(serverlessSpec.Serverless.Schema),
				ReadCapacity:     readCapacity,
			}
		}
	case "byoc":
		if byocSpec, err := idx.Spec.AsIndexModelSpec2(); err == nil {
			spec.BYOC = &BYOCSpec{
				Environment: byocSpec.Byoc.Environment,
				Schema:      toMetadataSchemaFromRest(byocSpec.Byoc.Schema),
			}
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
		PrivateHost:        idx.PrivateHost,
		Metric:             IndexMetric(idx.Metric),
		VectorType:         idx.VectorType,
		DeletionProtection: DeletionProtection(deletionProtection),
		Dimension:          idx.Dimension,
		Spec:               spec,
		Status:             status,
		Tags:               tags,
		Embed:              embed,
	}, nil
}

func decodeIndex(resBody io.ReadCloser) (*Index, error) {
	var idx db_control.IndexModel
	err := json.NewDecoder(resBody).Decode(&idx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IndexModel response: %w", err)
	}
	index, err := toIndex(&idx)
	if err != nil {
		return nil, err
	}
	return index, nil
}

func decodeBackupList(resBody io.ReadCloser) (*BackupList, error) {
	var backupListDb db_control.BackupList
	if err := json.NewDecoder(resBody).Decode(&backupListDb); err != nil {
		return nil, fmt.Errorf("failed to decode backup list response: %w", err)
	}
	var backupList BackupList
	if backupListDb.Data != nil {
		backupList.Data = make([]*Backup, len(*backupListDb.Data))
		for i, backup := range *backupListDb.Data {
			backupList.Data[i] = toBackup(&backup)
		}
		backupList.Pagination = (*Pagination)(backupListDb.Pagination)
	} else {
		backupList.Data = make([]*Backup, 0)
		backupList.Pagination = (*Pagination)(backupListDb.Pagination)
	}
	return &backupList, nil
}

func decodeRestoreJobList(resBody io.ReadCloser) (*RestoreJobList, error) {
	var restoreJobListDb db_control.RestoreJobList
	if err := json.NewDecoder(resBody).Decode(&restoreJobListDb); err != nil {
		return nil, fmt.Errorf("failed to decode restore job list response: %w", err)
	}
	var restoreJobList RestoreJobList
	if len(restoreJobListDb.Data) > 0 {
		restoreJobList.Data = make([]*RestoreJob, len(restoreJobListDb.Data))
		for i, restoreJob := range restoreJobListDb.Data {
			restoreJobList.Data[i] = toRestoreJob(&restoreJob)
		}
		restoreJobList.Pagination = (*Pagination)(restoreJobListDb.Pagination)
	} else {
		restoreJobList.Data = make([]*RestoreJob, 0)
		restoreJobList.Pagination = (*Pagination)(restoreJobListDb.Pagination)
	}
	return &restoreJobList, nil
}

func toBackup(backup *db_control.BackupModel) *Backup {
	if backup == nil {
		return nil
	}

	return &Backup{
		BackupId:        backup.BackupId,
		Cloud:           backup.Cloud,
		CreatedAt:       backup.CreatedAt,
		Description:     backup.Description,
		Dimension:       backup.Dimension,
		Metric:          (*IndexMetric)(backup.Metric),
		Name:            backup.Name,
		NamespaceCount:  backup.NamespaceCount,
		RecordCount:     backup.RecordCount,
		Region:          backup.Region,
		Schema:          toMetadataSchemaFromRest(backup.Schema),
		SizeBytes:       backup.SizeBytes,
		SourceIndexId:   backup.SourceIndexId,
		SourceIndexName: backup.SourceIndexName,
		Status:          backup.Status,
		Tags:            (*IndexTags)(backup.Tags),
	}
}

func decodeBackup(resBody io.ReadCloser) (*Backup, error) {
	var backup db_control.BackupModel
	if err := json.NewDecoder(resBody).Decode(&backup); err != nil {
		return nil, fmt.Errorf("failed to decode backup response: %w", err)
	}

	return toBackup(&backup), nil
}

func toRestoreJob(restoreJob *db_control.RestoreJobModel) *RestoreJob {
	if restoreJob == nil {
		return nil
	}

	return &RestoreJob{
		BackupId:        restoreJob.BackupId,
		CompletedAt:     restoreJob.CompletedAt,
		CreatedAt:       restoreJob.CreatedAt,
		PercentComplete: restoreJob.PercentComplete,
		RestoreJobId:    restoreJob.RestoreJobId,
		Status:          restoreJob.Status,
		TargetIndexId:   restoreJob.TargetIndexId,
		TargetIndexName: restoreJob.TargetIndexName,
	}
}

func decodeRestoreJob(resBody io.ReadCloser) (*RestoreJob, error) {
	var restoreJob db_control.RestoreJobModel
	if err := json.NewDecoder(resBody).Decode(&restoreJob); err != nil {
		return nil, fmt.Errorf("failed to decode restore job response: %w", err)
	}

	return toRestoreJob(&restoreJob), nil
}

func decodeEmbedResponse(resBody io.ReadCloser) (*EmbedResponse, error) {
	var rawEmbedResponse inference.EmbeddingsList
	if err := json.NewDecoder(resBody).Decode(&rawEmbedResponse); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	decodedEmbeddings := make([]Embedding, len(rawEmbedResponse.Data))
	for i, embedding := range rawEmbedResponse.Data {

		switch rawEmbedResponse.VectorType {
		case "sparse":
			dbSparseEmbedding, err := embedding.AsSparseEmbedding()
			if err != nil {
				return nil, fmt.Errorf("failed to decode SparseEmbedding: %w", err)
			}
			decodedEmbeddings[i] = Embedding{SparseEmbedding: &SparseEmbedding{
				VectorType:    dbSparseEmbedding.VectorType,
				SparseValues:  dbSparseEmbedding.SparseValues,
				SparseIndices: dbSparseEmbedding.SparseIndices,
				SparseTokens:  dbSparseEmbedding.SparseTokens,
			}}
		case "dense":
			dbDenseEmbedding, err := embedding.AsDenseEmbedding()
			if err != nil {
				return nil, fmt.Errorf("failed to decode SparseEmbedding: %w", err)
			}
			decodedEmbeddings[i] = Embedding{DenseEmbedding: &DenseEmbedding{
				VectorType: dbDenseEmbedding.VectorType,
				Values:     dbDenseEmbedding.Values,
			}}
		default:
			return nil, fmt.Errorf("unsupported VectorType: %s", rawEmbedResponse.VectorType)
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
	baseError := errors.New(string(jsonString))

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

func toMetadataSchemaFromRest(schema *struct {
	Fields map[string]struct {
		Filterable *bool `json:"filterable,omitempty"`
	} `json:"fields"`
}) *MetadataSchema {
	if schema == nil {
		return nil
	}

	fields := make(map[string]MetadataSchemaField)
	for key, value := range schema.Fields {
		fields[key] = MetadataSchemaField{
			Filterable: derefOrDefault(value.Filterable, false),
		}
	}

	return &MetadataSchema{
		Fields: fields,
	}
}

func fromMetadataSchemaToRest(schema *MetadataSchema) *struct {
	Fields map[string]struct {
		Filterable *bool `json:"filterable,omitempty"`
	} `json:"fields"`
} {
	if schema == nil {
		return nil
	}

	fields := make(map[string]struct {
		Filterable *bool `json:"filterable,omitempty"`
	})

	for key, value := range schema.Fields {
		filterable := value.Filterable
		fields[key] = struct {
			Filterable *bool `json:"filterable,omitempty"`
		}{
			Filterable: &filterable,
		}
	}

	return &struct {
		Fields map[string]struct {
			Filterable *bool `json:"filterable,omitempty"`
		} `json:"fields"`
	}{
		Fields: fields,
	}
}

// Converts the ReadCapacityRequest to db_control.ReadCapacity - used in CreateIndex, CreateIndexForModel, and ConfigureIndex
func readCapacityRequestToReadCapacity(request *ReadCapacityRequest) (*db_control.ReadCapacity, error) {
	// OnDemand - default if Dedicated is nil
	if request == nil || request.Dedicated == nil {
		var result db_control.ReadCapacity
		onDemandSpec := db_control.ReadCapacityOnDemandSpec{
			Mode: "OnDemand",
		}
		if err := result.FromReadCapacityOnDemandSpec(onDemandSpec); err != nil {
			return nil, err
		}
		return &result, nil
	}

	// Dedicated
	var result db_control.ReadCapacity
	dedicatedConfig := db_control.ReadCapacityDedicatedConfig{
		NodeType: request.Dedicated.NodeType,
	}

	// Scaling if provided
	if request.Dedicated.Scaling != nil && request.Dedicated.Scaling.Manual != nil {
		dedicatedConfig.Scaling = "Manual"
		dedicatedConfig.Manual = &db_control.ScalingConfigManual{
			Replicas: request.Dedicated.Scaling.Manual.Replicas,
			Shards:   request.Dedicated.Scaling.Manual.Shards,
		}
	}

	// Dedicated spec
	dedicatedSpec := db_control.ReadCapacityDedicatedSpec{
		Dedicated: dedicatedConfig,
		Mode:      "Dedicated",
	}
	if err := result.FromReadCapacityDedicatedSpec(dedicatedSpec); err != nil {
		return nil, err
	}
	return &result, nil
}

func toReadCapacity(rc *db_control.ReadCapacityResponse) (*ReadCapacity, error) {
	if rc == nil {
		return nil, nil
	}

	mode, err := rc.Discriminator()
	if err != nil {
		return nil, err
	}

	switch mode {
	case "OnDemand":
		onDemandSpec, err := rc.AsReadCapacityOnDemandSpecResponse()
		if err != nil {
			return nil, err
		}

		return &ReadCapacity{
			OnDemand: &ReadCapacityOnDemand{
				Status: ReadCapacityStatus{
					State:           onDemandSpec.Status.State,
					CurrentReplicas: onDemandSpec.Status.CurrentReplicas,
					CurrentShards:   onDemandSpec.Status.CurrentShards,
					ErrorMessage:    onDemandSpec.Status.ErrorMessage,
				},
			},
		}, nil
	case "Dedicated":
		dedicatedSpec, err := rc.AsReadCapacityDedicatedSpecResponse()
		if err != nil {
			return nil, err
		}

		dedicated := &ReadCapacityDedicated{
			NodeType: dedicatedSpec.Dedicated.NodeType,
			Status: ReadCapacityStatus{
				State:           dedicatedSpec.Status.State,
				CurrentReplicas: dedicatedSpec.Status.CurrentReplicas,
				CurrentShards:   dedicatedSpec.Status.CurrentShards,
				ErrorMessage:    dedicatedSpec.Status.ErrorMessage,
			},
		}

		// Scaling if present
		if strings.ToLower(dedicatedSpec.Dedicated.Scaling) == "manual" {
			dedicated.Scaling = &ReadCapacityScaling{
				Manual: &ReadCapacityManualScaling{
					Replicas: dedicatedSpec.Dedicated.Manual.Replicas,
					Shards:   dedicatedSpec.Dedicated.Manual.Shards,
				},
			}
		}
		return &ReadCapacity{
			Dedicated: dedicated,
		}, nil
	}
	return nil, nil
}
