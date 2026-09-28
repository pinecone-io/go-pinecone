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
	"slices"
	"strings"
	"time"

	"github.com/pinecone-io/go-pinecone/v7/internal/gen"
	"github.com/pinecone-io/go-pinecone/v7/internal/gen/db_control"
	db_data_rest "github.com/pinecone-io/go-pinecone/v7/internal/gen/db_data/rest"
	"github.com/pinecone-io/go-pinecone/v7/internal/gen/inference"
	"github.com/pinecone-io/go-pinecone/v7/internal/provider"
	"github.com/pinecone-io/go-pinecone/v7/internal/useragent"
	"google.golang.org/grpc"
)

// Client holds the parameters for connecting to the Pinecone service. It is returned by the [NewClient] and [NewClientBase]
// functions. To use Client, first build the parameters of the request using [NewClientParams] (or [NewClientBaseParams]).
// Then, pass those parameters into the [NewClient] (or [NewClientBase]) function to create a new [Client] object.
// Once instantiated, you can use [Client] to execute Pinecone API requests (e.g. create an [Index], list Indexes,
// etc.), and Inference API requests. Read more about different Pinecone API routes [here].
//
// Note: Client methods are safe for concurrent use.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//		       log.Fatalf("Failed to describe index: %v", err)
//	    } else {
//		       fmt.Printf("Successfully found the \"%s\" index!\n", idx.Name)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    } else {
//		       log.Println("IndexConnection created successfully!")
//	    }
//
// [here]: https://docs.pinecone.io/reference/api/latest/control-plane/list_indexes
//
// [Inference API]: https://docs.pinecone.io/reference/api/latest/inference/generate-vectors
type Client struct {
	// Inference exposes methods for interacting with the Pinecone [Inference API].
	Inference  *InferenceService
	restClient *db_control.Client
	baseParams *NewClientBaseParams
}

// NewClientParams holds the parameters for creating a new [Client] instance while authenticating via an API key.
//
// See [Client] for code example.
type NewClientParams struct {
	// ApiKey (Required) is the API key used to authenticate with the Pinecone API. It may be omitted
	// if the PINECONE_API_KEY environment variable is set.
	ApiKey string
	// Headers (Optional) is a map of HTTP headers to include in each REST API request. Headers from
	// the PINECONE_ADDITIONAL_HEADERS environment variable (a JSON object) are merged in; values in
	// Headers take precedence. The "Api-Key" header is always set from ApiKey, and NewClient writes
	// it into this map. gRPC data-plane calls made through [IndexConnection] send only the
	// authentication header, not other custom headers.
	Headers map[string]string
	// Host (Optional) is the host URL of the Pinecone API, used for control-plane and inference
	// requests. Defaults to the PINECONE_CONTROLLER_HOST environment variable, or
	// "https://api.pinecone.io" if unset. "https://" is prepended when no scheme is given.
	Host string
	// RestClient (Optional) is the HTTP client used to communicate with the Pinecone API.
	RestClient *http.Client
	// SourceTag (Optional) is a string used to help Pinecone attribute API activity.
	SourceTag string
	// RetryPolicy (Optional) enables retries on rate-limit and transient errors for REST and gRPC.
	RetryPolicy *RetryPolicy
}

// NewClientBaseParams holds the parameters for creating a new [Client] instance while passing custom authentication
// headers. If there is no API key or authentication provided through Headers, API calls will fail.
//
// See [Client] for code example.
type NewClientBaseParams struct {
	// Headers (Optional) is a map of HTTP headers to include in each REST API request.
	// "Authorization" and "X-Project-Id" headers are required if authenticating using a JWT. Headers
	// from the PINECONE_ADDITIONAL_HEADERS environment variable (a JSON object) are merged in; values
	// in Headers take precedence. gRPC data-plane calls made through [IndexConnection] send only the
	// authentication header, not other custom headers.
	Headers map[string]string
	// Host (Optional) is the host URL of the Pinecone API, used for control-plane and inference
	// requests. Defaults to the PINECONE_CONTROLLER_HOST environment variable, or
	// "https://api.pinecone.io" if unset. "https://" is prepended when no scheme is given.
	Host string
	// RestClient (Optional) is the HTTP client used to communicate with the Pinecone API.
	RestClient *http.Client
	// SourceTag (Optional) is a string used to help Pinecone attribute API activity.
	SourceTag string
	// RetryPolicy (Optional) enables retries on rate-limit and transient errors for REST and gRPC.
	RetryPolicy *RetryPolicy
}

// NewIndexConnParams holds the parameters for creating an [IndexConnection] to a Pinecone index.
//
// See [Client.Index] for code example.
type NewIndexConnParams struct {
	// Host (Required) is the host URL of the Pinecone index. Find it with [Client.DescribeIndex] or
	// [Client.ListIndexes], or in the Pinecone web console.
	Host string
	// Namespace (Optional) is the index namespace to use for operations. "" (the zero value) and
	// "__default__" both refer to the default namespace.
	Namespace string
	// AdditionalMetadata (Optional) is metadata to send with each RPC request.
	AdditionalMetadata map[string]string
}

// NewClient creates and initializes a new instance of [Client].
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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

	return NewClientBase(NewClientBaseParams{Headers: clientHeaders, Host: in.Host, RestClient: in.RestClient, SourceTag: in.SourceTag, RetryPolicy: in.RetryPolicy})
}

// NewClientBase creates and initializes a new instance of [Client] with custom authentication headers.
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
//		ctx := context.Background()
//
//	    clientParams := pinecone.NewClientBaseParams{
//			Headers: map[string]string{
//				"Authorization": "Bearer " + "<your OAuth token>",
//				"X-Project-Id":  "<Your Pinecone project ID>",
//			},
//		}
//
//	    pc, err := pinecone.NewClientBase(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
func NewClientBase(in NewClientBaseParams) (*Client, error) {
	if err := in.RetryPolicy.validate(); err != nil {
		return nil, err
	}
	// Retries apply to all REST clients (control/data/inference) via a wrapped transport.
	if in.RetryPolicy != nil {
		in.RestClient = NewRetryHTTPClient(in.RetryPolicy, in.RestClient)
	}

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

// Index creates an [IndexConnection] to a specified host.
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
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        panic(fmt.Errorf("Failed to create Client: %v", err))
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//		       log.Fatalf("Failed to describe index: %v", err)
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

	// Enable gRPC data-plane retries; user-supplied dialOpts come after and win on conflict.
	if c.baseParams.RetryPolicy != nil {
		dialOpts = append(RetryDialOptions(c.baseParams.RetryPolicy), dialOpts...)
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

// ListIndexes retrieves a list of all Indexes in a Pinecone [project].
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
//	        panic(fmt.Errorf("Failed to create Client: %v", err))
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

// CreateIndexRequest holds the parameters for creating an index from an explicit schema with
// [Client.CreateIndex].
type CreateIndexRequest struct {
	// Name (Optional) is the name of the [Index]. Must be unique within the project, 1-45
	// characters, start and end with an alphanumeric character, and consist only of lower case
	// alphanumeric characters or '-'. If empty, Pinecone generates a name. Provide a name if you
	// need to retry the request safely: a retry with the same name fails with a conflict instead of
	// creating a second index.
	Name string
	// Schema (Required) is the [IndexSchema] defining the index's fields. At creation you can
	// declare [DenseVectorField], [SparseVectorField], and [StringField] with FullTextSearch set.
	// Metadata fields don't need to be declared; they are indexed automatically when you upsert data.
	// A schema containing only the reserved fields "_values" (dense) and/or "_sparse_values"
	// (sparse) creates a vector index, used with the vector operations such as
	// [IndexConnection.UpsertVectors]. Any other schema creates a document index, used with the
	// document operations such as [IndexConnection.UpsertDocuments]. At most one dense and one
	// sparse vector field, and at most 100 full-text-search fields, are allowed. Field names must be
	// at most 64 bytes and must not start with '$' or '_', except that "_values" and
	// "_sparse_values" may make up the entire schema. The schema can't be changed after the index is
	// created, apart from the embedding settings updated with [Client.ConfigureIndex].
	Schema IndexSchema
	// Deployment (Optional) describes where the index runs. Defaults to a serverless index on AWS in
	// "us-east-1".
	Deployment *IndexDeployment
	// ReadCapacity (Optional) is the read capacity configuration. Defaults to OnDemand. Some BYOC
	// environments reject OnDemand and require Dedicated.
	ReadCapacity *ReadCapacityParams
	// CmekId (Optional) is the ID of a customer-managed encryption key (CMEK) to use for this index.
	// Not supported for pod-based or BYOC deployments.
	CmekId *string
	// DeletionProtection (Optional) determines whether deletion protection is "enabled" or
	// "disabled" for the index. Defaults to "disabled".
	DeletionProtection *DeletionProtection
	// Tags (Optional) is a map of tags to associate with the index. See [IndexTags] for the limits.
	Tags *IndexTags
}

// CreateIndex creates a new [Index] from an explicit [IndexSchema] and, optionally, an
// [IndexDeployment]. Use it to create document indexes, such as indexes with full-text search or
// with named dense and sparse vector fields. To create a vector index from a dimension and metric,
// you can also use [Client.CreateServerlessIndex] or [Client.CreateBYOCIndex].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateIndexRequest] object. See [CreateIndexRequest] for more information.
//
// Returns a pointer to an [Index] object or an error.
//
// Example:
//
//	    // A document index with named dense and sparse fields, used with the document operations.
//	    idx, err := pc.CreateIndex(ctx, &pinecone.CreateIndexRequest{
//		    Name: "hybrid",
//		    Schema: pinecone.IndexSchema{
//			    Fields: map[string]pinecone.IndexSchemaField{
//				    "embedding":    {DenseVector: &pinecone.DenseVectorField{Dimension: 1536, Metric: pinecone.IndexMetricDotproduct}},
//				    "sparse_terms": {SparseVector: &pinecone.SparseVectorField{}},
//			    },
//		    },
//		    Deployment: &pinecone.IndexDeployment{
//			    Managed: &pinecone.ManagedDeployment{Cloud: pinecone.CloudAWS, Region: "us-east-1"},
//		    },
//	    })
func (c *Client) CreateIndex(ctx context.Context, in *CreateIndexRequest) (*Index, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateIndexRequest) cannot be nil")
	}
	if len(in.Schema.Fields) == 0 {
		return nil, fmt.Errorf("Schema must contain at least one field")
	}

	schema, err := toDbCreateIndexSchema(in.Schema)
	if err != nil {
		return nil, err
	}

	deployment, err := toDbDeploymentRequest(in.Deployment)
	if err != nil {
		return nil, err
	}

	readCapacity, err := readCapacityParamsToReadCapacity(in.ReadCapacity)
	if err != nil {
		return nil, err
	}

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	req := db_control.CreateIndexRequest{
		Name:               pointerOrNil(in.Name),
		Schema:             schema,
		Deployment:         deployment,
		ReadCapacity:       readCapacity,
		CmekId:             in.CmekId,
		DeletionProtection: (*db_control.DeletionProtection)(in.DeletionProtection),
		Tags:               tags,
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

// CreateServerlessIndexRequest holds the parameters for creating a new [Serverless] Index.
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
//	    if err != nil {
//	        panic(fmt.Errorf("Failed to create Client: %v", err))
//	    }
//
//		indexName := "my-serverless-index"
//
//		dimension := int32(3)
//		metric := pinecone.IndexMetricCosine
//		idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//		    Name:      indexName,
//			Dimension: &dimension,
//			Metric:  &metric,
//			Cloud:   pinecone.CloudAWS,
//			Region:  "us-east-1",
//	    })
//
//		if err != nil {
//		    log.Fatalf("Failed to create serverless index: %v", err)
//		} else {
//		    fmt.Printf("Successfully created serverless index: %s", idx.Name)
//		}
//
// [Serverless]: https://docs.pinecone.io/guides/index-data/create-an-index
//
// [dimensionality]: https://docs.pinecone.io/guides/core-concepts/key-terms#dense-vector
// [similarity]: https://docs.pinecone.io/guides/index-data/create-an-index#similarity-metrics
// [region]: https://docs.pinecone.io/guides/index-data/create-an-index#cloud-regions
// [cloud provider]: https://docs.pinecone.io/guides/index-data/create-an-index#cloud-regions
// [deletion protection]: https://docs.pinecone.io/guides/manage-data/manage-indexes#configure-deletion-protection
type CreateServerlessIndexRequest struct {
	// Name (Required) is the name of the [Index]. Must be 1-45 characters long, start and end with an
	// alphanumeric character, and consist only of lower case alphanumeric characters or '-'.
	Name string
	// Cloud (Required) is the public [cloud provider] where the index is hosted.
	Cloud Cloud
	// Region (Required) is the [region] where the index is created.
	Region string
	// Metric (Optional) is the metric used to measure the [similarity] between vectors ('euclidean',
	// 'cosine', or 'dotproduct'). Defaults to `cosine` or `dotproduct` depending on the VectorType.
	// Use `dotproduct` for an index that stores both dense and sparse vectors, since hybrid queries
	// require it.
	Metric *IndexMetric
	// DeletionProtection (Optional) determines whether [deletion protection] is "enabled" or
	// "disabled" for the index. When "enabled", the index cannot be deleted. Defaults to "disabled".
	DeletionProtection *DeletionProtection
	// Dimension is the [dimensionality] of the vectors to be inserted in the index. Required unless
	// VectorType is "sparse", in which case it must be omitted.
	Dimension *int32
	// VectorType (Optional) is the index vector type, `dense` or `sparse`. If `dense`, Dimension must
	// be specified. If `sparse`, Dimension should not be specified, and Metric must be `dotproduct`.
	// Defaults to `dense`.
	VectorType *string
	// ReadCapacity (Optional) is the read capacity configuration for the index. Use it to configure
	// dedicated read capacity with specific node types and scaling strategies.
	ReadCapacity *ReadCapacityParams
	// Schema is not supported; setting it returns an error. Metadata fields are indexed
	// automatically when you upsert data.
	Schema *MetadataSchema
	// Tags (Optional) is a map of tags to associate with the index. See [IndexTags] for the limits.
	Tags *IndexTags
	// SourceCollection is not supported; setting it returns an error. To restore data into a new
	// index, use [Client.CreateIndexFromBackup].
	SourceCollection *string
}

// ReadCapacityParams is the read capacity configuration for a new index, or for an index changed
// with [Client.ConfigureIndex]. Set exactly one of Dedicated or OnDemand. When nil, a new index uses
// OnDemand read capacity; some BYOC environments reject OnDemand and require Dedicated.
type ReadCapacityParams struct {
	// Dedicated selects Dedicated read capacity mode. Requires NodeType and Scaling configuration.
	Dedicated *ReadCapacityDedicatedConfig `json:"dedicated,omitempty"`
	// OnDemand selects OnDemand read capacity mode. No configuration is required.
	OnDemand *ReadCapacityOnDemandConfig `json:"on_demand,omitempty"`
}

// ReadCapacityDedicatedConfig represents Dedicated read capacity configuration for indexes.
// When creating a Dedicated index or converting an existing OnDemand index to Dedicated, you must specify NodeType, Scaling.Manual.Replicas,
// and Scaling.Manual.Shards.
type ReadCapacityDedicatedConfig struct {
	// NodeType is the type of machines to use. Available options: "b1" and "t1". "t1" includes
	// increased processing power and memory.
	NodeType *string `json:"node_type"`
	// Scaling is the scaling strategy configuration. Currently supports manual scaling.
	Scaling *ReadCapacityScaling `json:"scaling,omitempty"`
}

// ReadCapacityOnDemandConfig represents OnDemand read capacity configuration for indexes.
// The struct is intentionally empty because OnDemand does not support configuration values.
// When creating an OnDemand index, you can leave [ReadCapacityParams] empty in the create request.
type ReadCapacityOnDemandConfig struct{}

// CreateServerlessIndex creates and initializes a new serverless index via the specified [Client].
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
//	        panic(fmt.Errorf("Failed to create Client: %v", err))
//	    }
//
//	    indexName := "my-serverless-index"
//
//	    dimension := int32(3)
//	    metric := pinecone.IndexMetricCosine
//	    idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
//		    Name:    indexName,
//		    Dimension: &dimension,
//		    Metric:  &metric,
//		    Cloud:   pinecone.CloudAWS,
//		    Region:  "us-east-1",
//		})
//
//		if err != nil {
//		    log.Fatalf("Failed to create serverless index: %v", err)
//		} else {
//		    fmt.Printf("Successfully created serverless index: %s", idx.Name)
//		}
func (c *Client) CreateServerlessIndex(ctx context.Context, in *CreateServerlessIndexRequest) (*Index, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateServerlessIndexRequest) cannot be nil")
	}
	if in.Name == "" || in.Cloud == "" || in.Region == "" {
		return nil, fmt.Errorf("fields Name, Cloud, and Region must be included in CreateServerlessIndexRequest")
	}

	if in.SourceCollection != nil {
		return nil, fmt.Errorf("SourceCollection is not supported by Pinecone API version 2026-07, which does not support creating an index from a collection; use CreateIndexFromBackup to restore a backup into a new index")
	}
	if in.Schema != nil {
		return nil, fmt.Errorf("Schema is not supported by Pinecone API version 2026-07: metadata fields are indexed automatically when you upsert data, so they don't need to be declared")
	}

	vectorType, err := validateVectorType(in.VectorType, in.Dimension, in.Metric)
	if err != nil {
		return nil, err
	}

	var deletionProtection *db_control.DeletionProtection
	if in.DeletionProtection != nil {
		deletionProtection = pointerOrNil(db_control.DeletionProtection(*in.DeletionProtection))
	}

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	readCapacity, err := readCapacityParamsToReadCapacity(in.ReadCapacity)
	if err != nil {
		return nil, err
	}

	schema, err := classicVectorSchema(vectorType, in.Dimension, in.Metric)
	if err != nil {
		return nil, err
	}

	deployment, err := toDbDeploymentRequest(&IndexDeployment{Managed: &ManagedDeployment{
		Cloud:  in.Cloud,
		Region: in.Region,
	}})
	if err != nil {
		return nil, err
	}

	req := db_control.CreateIndexRequest{
		Name:               &in.Name,
		Schema:             schema,
		Deployment:         deployment,
		ReadCapacity:       readCapacity,
		DeletionProtection: deletionProtection,
		Tags:               tags,
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

// CreateIndexForModelRequest defines the desired configuration for creating an index with an associated embedding model.
//
// To create an index with an associated embedding model, use the [Client.CreateIndexForModel] method.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//	         ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//	        panic(fmt.Errorf("Failed to create Client: %v", err))
//	    }
//
//	    request := &pinecone.CreateIndexForModelRequest{
//	        Name:   "my-index",
//	        Cloud:  pinecone.CloudAWS,
//	        Region: "us-east-1",
//	        Embed: pinecone.CreateIndexForModelEmbed{
//			    Model:    "multilingual-e5-large",
//		        FieldMap: map[string]interface{}{"text": "chunk_text"},
//			},
//	    }
//
//	    idx, err := pc.CreateIndexForModel(ctx, request)
//	    if err != nil {
//	        log.Fatalf("Failed to create index: %v", err)
//	    }
//
// [Index]: https://docs.pinecone.io/guides/index-data/indexing-overview#indexes
// [region]: https://docs.pinecone.io/guides/index-data/create-an-index#cloud-regions
// [cloud provider]: https://docs.pinecone.io/guides/index-data/create-an-index#cloud-regions
// [deletion protection]: https://docs.pinecone.io/guides/manage-data/manage-indexes#configure-deletion-protection
// [similarity metric]: https://docs.pinecone.io/guides/index-data/create-an-index#similarity-metrics
type CreateIndexForModelRequest struct {
	// Name (Required) is the name of the [Index]. Must be 1-45 characters long, start and end with an
	// alphanumeric character, and consist only of lower case alphanumeric characters or '-'.
	Name string
	// Cloud (Required) is the public [cloud provider] where the index is hosted.
	Cloud Cloud
	// Region (Required) is the [region] where the index is created.
	Region string
	// DeletionProtection (Optional) determines whether [deletion protection] is "enabled" or
	// "disabled" for the index. When "enabled", the index cannot be deleted. Defaults to "disabled".
	DeletionProtection *DeletionProtection
	// Embed (Required) is the [CreateIndexForModelEmbed] embedding model configuration. The model
	// can't be changed after the index is created; the read and write parameters can be updated
	// with [Client.ConfigureIndex].
	Embed CreateIndexForModelEmbed
	// ReadCapacity (Optional) is the read capacity configuration for the index. Use it to configure
	// dedicated read capacity with specific node types and scaling strategies.
	ReadCapacity *ReadCapacityParams
	// Schema (Optional) configures the behavior of Pinecone's internal metadata index. By default,
	// all metadata is indexed.
	Schema *MetadataSchema
	// Tags (Optional) are custom user tags added to the index. See [IndexTags] for the limits.
	Tags *IndexTags
}

// CreateIndexForModelEmbed defines the embedding model configuration for an index.
//
// The `CreateIndexForModelEmbed` struct is used as part of the [CreateIndexForModelRequest] when creating an index
// with an associated embedding model. The model can't be changed after the index is created; the read and
// write parameters can be updated with [Client.ConfigureIndex] using [ConfigureIndexParams].Schema.
//
// [similarity metric]: https://docs.pinecone.io/guides/index-data/create-an-index#similarity-metrics
type CreateIndexForModelEmbed struct {
	// Model (Required) is the name of the embedding model to use for the index.
	Model string
	// FieldMap (Required) identifies the name of the text field from your document model that will
	// be embedded.
	FieldMap map[string]interface{}
	// Dimension (Optional) is the dimensionality of the vectors to be inserted in the index.
	// Defaults according to the model.
	Dimension *int
	// Metric (Optional) is the [similarity metric] used for similarity search: 'euclidean',
	// 'cosine', or 'dotproduct'. Defaults according to the model. Cannot be updated once set.
	Metric *IndexMetric
	// ReadParameters (Optional) are the read parameters for the embedding model.
	ReadParameters *map[string]interface{}
	// WriteParameters (Optional) are the write parameters for the embedding model.
	WriteParameters *map[string]interface{}
}

// CreateIndexForModel creates and initializes a new serverless Index via the specified [Client] with integrated
// embedding, using one of Pinecone's hosted embedding models. After the index is created, you can upsert and search for records
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
//		if err != nil {
//		    panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    indexName := "my-serverless-index"
//
//	    idx, err := pc.CreateIndexForModel(ctx, &pinecone.CreateIndexForModelRequest{
//		    Name:    indexName,
//		    Cloud:   pinecone.CloudAWS,
//		    Region:  "us-east-1",
//		    Embed: pinecone.CreateIndexForModelEmbed{
//			    Model:    "multilingual-e5-large",
//			    FieldMap: map[string]interface{}{"text": "chunk_text"},
//			},
//		})
//
//		if err != nil {
//		    log.Fatalf("Failed to create serverless index: %v", err)
//		} else {
//		    fmt.Printf("Successfully created serverless index: %s", idx.Name)
//		}
func (c *Client) CreateIndexForModel(ctx context.Context, in *CreateIndexForModelRequest) (*Index, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateIndexForModelRequest) cannot be nil")
	}
	if in.Name == "" || in.Cloud == "" || in.Region == "" || in.Embed.Model == "" {
		return nil, fmt.Errorf("fields Name, Cloud, Region, and Embed.Model must be included in CreateIndexForModelRequest")
	}

	deletionProtection := derefOrDefault(in.DeletionProtection, "disabled")

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	readCapacity, err := readCapacityParamsToReadCapacity(in.ReadCapacity)
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

// CreateBYOCIndexRequest holds the parameters for creating a new BYOC ([Bring Your Own Cloud]) Index.
//
// To create a new BYOC Index, use the [Client.CreateBYOCIndex] method.
//
// Example:
//
//	    ctx := context.Background()
//
//		clientParams := pinecone.NewClientParams{
//		    ApiKey:    "YOUR_API_KEY",
//	    }
//
//		pc, err := pinecone.NewClient(clientParams)
//		if err != nil {
//		    panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//		indexName := "my-byoc-index"
//
//		dimension := int32(3)
//		metric := pinecone.IndexMetricCosine
//		idx, err := pc.CreateBYOCIndex(ctx, &pinecone.CreateBYOCIndexRequest{
//		    Name:        indexName,
//			Environment: "my-environment",
//			Dimension:   &dimension,
//			Metric:      &metric,
//	    })
//
//		if err != nil {
//		    log.Fatalf("Failed to create BYOC index: %v", err)
//		} else {
//		    fmt.Printf("Successfully created BYOC index: %s", idx.Name)
//		}
//
// [Bring Your Own Cloud]: https://docs.pinecone.io/guides/production/bring-your-own-cloud
//
// [dimensionality]: https://docs.pinecone.io/guides/core-concepts/key-terms#dense-vector
// [similarity]: https://docs.pinecone.io/guides/index-data/create-an-index#similarity-metrics
// [deletion protection]: https://docs.pinecone.io/guides/manage-data/manage-indexes#configure-deletion-protection
type CreateBYOCIndexRequest struct {
	// Name (Required) is the name of the [Index]. Must be 1-45 characters long, start and end with an
	// alphanumeric character, and consist only of lower case alphanumeric characters or '-'.
	Name string
	// Environment (Required) is the environment identifier for the BYOC index.
	Environment string
	// Dimension is the [dimensionality] of the vectors to be inserted in the index. Required unless
	// VectorType is "sparse", in which case it must be omitted.
	Dimension *int32
	// VectorType (Optional) is the index vector type, `dense` or `sparse`. If `dense`, Dimension must
	// be specified. If `sparse`, Dimension should not be specified, and Metric must be `dotproduct`.
	// Defaults to `dense`.
	VectorType *string
	// Metric (Optional) is the metric used to measure the [similarity] between vectors ('euclidean',
	// 'cosine', or 'dotproduct'). Defaults to `cosine` for an index that stores dense vectors and
	// `dotproduct` for one that stores only sparse vectors. Use `dotproduct` for an index that stores
	// both dense and sparse vectors, since hybrid queries require it.
	Metric *IndexMetric
	// DeletionProtection (Optional) determines whether [deletion protection] is "enabled" or
	// "disabled" for the index. When "enabled", the index cannot be deleted. Defaults to "disabled".
	DeletionProtection *DeletionProtection
	// ReadCapacity (Optional) is the read capacity configuration for the index. Defaults to
	// OnDemand when nil. Some BYOC environments support only Dedicated read capacity and reject
	// OnDemand, including the default; in those, set Dedicated with NodeType,
	// Scaling.Manual.Replicas, and Scaling.Manual.Shards.
	ReadCapacity *ReadCapacityParams
	// Schema is not supported; setting it returns an error. Metadata fields are indexed
	// automatically when you upsert data.
	Schema *MetadataSchema
	// Tags (Optional) is a map of tags to associate with the index. See [IndexTags] for the limits.
	Tags *IndexTags
}

// CreateBYOCIndex creates and initializes a new BYOC (Bring Your Own Cloud) Index via the specified [Client].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateBYOCIndexRequest] object. See [CreateBYOCIndexRequest] for more information.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    indexName := "my-byoc-index"
//
//	    dimension := int32(3)
//	    metric := pinecone.IndexMetricCosine
//	    idx, err := pc.CreateBYOCIndex(ctx, &pinecone.CreateBYOCIndexRequest{
//		    Name:        indexName,
//		    Environment: "my-environment",
//		    Dimension:  &dimension,
//		    Metric:     &metric,
//		})
//
//		if err != nil {
//		    log.Fatalf("Failed to create BYOC index: %v", err)
//		} else {
//		    fmt.Printf("Successfully created BYOC index: %s", idx.Name)
//		}
func (c *Client) CreateBYOCIndex(ctx context.Context, in *CreateBYOCIndexRequest) (*Index, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateBYOCIndexRequest) cannot be nil")
	}
	if in.Name == "" || in.Environment == "" {
		return nil, fmt.Errorf("fields Name, and Environment must be included in CreateBYOCIndexRequest")
	}

	if in.Schema != nil {
		return nil, fmt.Errorf("Schema is not supported by Pinecone API version 2026-07: metadata fields are indexed automatically when you upsert data, so they don't need to be declared")
	}

	deletionProtection := derefOrDefault(in.DeletionProtection, "disabled")

	var tags *db_control.IndexTags
	if in.Tags != nil {
		tags = (*db_control.IndexTags)(in.Tags)
	}

	readCapacity, err := readCapacityParamsToReadCapacity(in.ReadCapacity)
	if err != nil {
		return nil, err
	}

	vectorType, err := validateVectorType(in.VectorType, in.Dimension, in.Metric)
	if err != nil {
		return nil, err
	}

	schema, err := classicVectorSchema(vectorType, in.Dimension, in.Metric)
	if err != nil {
		return nil, err
	}

	deployment, err := toDbDeploymentRequest(&IndexDeployment{Byoc: &ByocDeployment{
		Environment: in.Environment,
	}})
	if err != nil {
		return nil, err
	}

	req := db_control.CreateIndexRequest{
		Name:               &in.Name,
		Schema:             schema,
		Deployment:         deployment,
		ReadCapacity:       readCapacity,
		DeletionProtection: (*db_control.DeletionProtection)(&deletionProtection),
		Tags:               tags,
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

// DescribeIndex retrieves information about a specific [Index]. See [Index] for more information.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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

// DeleteIndex deletes a specific [Index]. Deletion is asynchronous: the index may still be listed,
// in the Terminating state, for a short time after DeleteIndex returns. DeleteIndex returns a
// [PineconeError] with Code 403 if deletion protection is enabled (disable it with
// [Client.ConfigureIndex] first), or with Code 412 if a collection is being created from the index.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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

// ConfigureIndexParams contains parameters for configuring an [Index]. For any index you can
// configure DeletionProtection and Tags. For serverless and BYOC indexes you can also configure
// ReadCapacity, for pod-based indexes the number of Replicas and the PodType, and for indexes with
// integrated embedding the read and write parameters of the embedding model through Schema.
// Each of the fields is optional, but at least one field must be set.
// See [scale a pods-based index] for more information.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{
//		    DeletionProtection: pinecone.DeletionProtectionEnabled,
//		    Replicas:           4,
//	    })
//	    if err != nil {
//		    log.Fatalf("Failed to configure index: %v", err)
//	    }
//	    fmt.Printf("Configured index %s\n", idx.Name)
//
// [scale a pods-based index]: https://docs.pinecone.io/guides/indexes/pods/scale-pod-based-indexes
//
// [app.pinecone.io]: https://app.pinecone.io
// [deletion protection]: https://docs.pinecone.io/guides/manage-data/manage-indexes#configure-deletion-protection
type ConfigureIndexParams struct {
	// PodType (Optional) is the pod size to scale the index to. For a "p1" pod type, pass "p1.x2"
	// to scale to the "x2" size, "p1.x4" for the "x4" size, and so forth. The pod size can only be
	// increased, and the pod family can't be changed. Only applies to pod-based indexes; setting it on
	// any other index returns an error.
	PodType string
	// Replicas (Optional) is the number of replicas to scale the index to. This is capped by the
	// maximum number of replicas allowed in your Pinecone project, which you can configure at
	// [app.pinecone.io]. Only applies to pod-based indexes; setting it on any other index returns an
	// error.
	Replicas int32
	// DeletionProtection (Optional) determines whether [deletion protection] is "enabled" or
	// "disabled" for the index. When "enabled", the index cannot be deleted.
	DeletionProtection DeletionProtection
	// Tags (Optional) is a map of tags to merge into the index's existing tags. To unset a key, set
	// its value to "".
	Tags IndexTags
	// Embed is not supported; setting it returns an error without calling the API.
	//
	// Deprecated: Pinecone API version 2026-07 no longer supports configuring an index's embedding
	// through Embed. To update the read or write parameters of the embedding model, use Schema.
	Embed *ConfigureIndexEmbed
	// ReadCapacity (Optional) is the [ReadCapacityParams] to apply to a serverless or BYOC index;
	// setting it on a pod-based index returns an error. Converting an OnDemand index to Dedicated
	// requires NodeType, Scaling.Manual.Replicas, and Scaling.Manual.Shards. Fields omitted from a
	// Dedicated configuration keep their current values. Some BYOC environments reject switching to
	// OnDemand.
	ReadCapacity *ReadCapacityParams
	// Schema (Optional) is the [ConfigureIndexSchema] updating the read or write parameters of the
	// embedding model on an index with integrated embedding.
	Schema *ConfigureIndexSchema
}

// ConfigureIndexSchema holds a semantic text field update for [Client.ConfigureIndex]. It updates the
// read and write parameters of an index's integrated embedding model; the model can't be changed.
//
// Example:
//
//	    _, err := pc.ConfigureIndex(ctx, "my-integrated-index", pinecone.ConfigureIndexParams{
//		    Schema: &pinecone.ConfigureIndexSchema{
//			    Fields: map[string]pinecone.ConfigureSemanticTextField{
//				    "chunk_text": {ReadParameters: &map[string]interface{}{"input_type": "query", "truncate": "NONE"}},
//			    },
//		    },
//	    })
type ConfigureIndexSchema struct {
	// Fields (Required) is the semantic text field to update, keyed by the name of the
	// [SemanticTextField] in the index's [IndexSchema]. Exactly one field must be given.
	Fields map[string]ConfigureSemanticTextField
}

// ConfigureSemanticTextField holds updated parameters for a semantic text field.
type ConfigureSemanticTextField struct {
	// Model (Optional) is the field's embedding model. The model can't be changed, so if set it must
	// match the current model.
	Model *string
	// ReadParameters (Optional) are the model parameters to apply at query time.
	ReadParameters *map[string]interface{}
	// WriteParameters (Optional) are the model parameters to apply at write time.
	WriteParameters *map[string]interface{}
}

// ConfigureIndexEmbed contains parameters for configuring the integrated embedding settings for an [Index].
//
// Deprecated: Pinecone API version 2026-07 no longer supports configuring an index's embedding through Embed, so
// setting [ConfigureIndexParams].Embed returns an error. To update the read or write parameters of the embedding
// model, use [ConfigureIndexParams].Schema. The model can't be changed after the index is created.
type ConfigureIndexEmbed struct {
	// FieldMap (Optional) identifies the name of the text field from your document model that will
	// be embedded.
	FieldMap *map[string]interface{}
	// Model (Optional) is the name of the embedding model to use with the index.
	Model *string
	// ReadParameters (Optional) are the read parameters for the embedding model.
	ReadParameters *map[string]interface{}
	// WriteParameters (Optional) are the write parameters for the embedding model.
	WriteParameters *map[string]interface{}
}

// ConfigureIndex is used to configure an existing [Index] allowing you to update the index's deletion protection status, tags,
// read capacity configuration, and the read and write parameters of an integrated embedding model. You can also
// [scale a pods-based index] by increasing the pod size or changing the number of replicas.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - name: The name of the [Index] to configure.
//   - in: A [ConfigureIndexParams] object that contains the parameters for configuring the [Index].
//
// Note: For pod-based indexes, Replicas can be scaled up or down, and PodType can be changed to a
// larger size in the same pod family (for example "p1.x2" to "p1.x4"). The pod size can't be
// decreased and the pod family can't be changed; to do either, create a new index.
//
// Returns a pointer to a configured [Index] object or an error.
//
// Example:
//
//		// To scale the size of your pods-based index from "x2" to "x4":
//		 _, err := pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{PodType: "p1.x4"})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
//		// To scale the number of replicas:
//		 _, err = pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{Replicas: 4})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
//		// To scale both the size of your pods and the number of replicas to 4:
//		 _, err = pc.ConfigureIndex(ctx, "my-pod-index", pinecone.ConfigureIndexParams{PodType: "p1.x4", Replicas: 4})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
//	    // To enable deletion protection:
//		 _, err = pc.ConfigureIndex(ctx, "my-index", pinecone.ConfigureIndexParams{DeletionProtection: "enabled"})
//		 if err != nil {
//		     fmt.Printf("Failed to configure index: %v\n", err)
//		 }
//
// [scale a pods-based index]: https://docs.pinecone.io/guides/indexes/pods/scale-pod-based-indexes
func (c *Client) ConfigureIndex(ctx context.Context, name string, in ConfigureIndexParams) (*Index, error) {
	if in.PodType == "" && in.Replicas == 0 && in.DeletionProtection == "" && in.Tags == nil && in.ReadCapacity == nil && in.Schema == nil && in.Embed == nil {
		return nil, fmt.Errorf("must specify PodType, Replicas, DeletionProtection, ReadCapacity, Schema, or Tags when configuring an index")
	}
	if in.Embed != nil {
		return nil, fmt.Errorf("Embed is not supported by Pinecone API version 2026-07: to update the read or write parameters of an index's embedding model, use Schema")
	}
	if in.Schema != nil && len(in.Schema.Fields) != 1 {
		return nil, fmt.Errorf("Schema must contain exactly one field to update")
	}

	podType := pointerOrNil(in.PodType)
	replicas := pointerOrNil(in.Replicas)
	deletionProtection := pointerOrNil(in.DeletionProtection)

	// Describe index in order to merge existing tags with incoming tags,
	// and evaluate the deployment type to validate type-specific parameters.
	idxDesc, err := c.DescribeIndex(ctx, name)
	if err != nil {
		return nil, err
	}
	existingTags := idxDesc.Tags

	isPod := idxDesc.Deployment != nil && idxDesc.Deployment.Pod != nil

	// Validate that deployment-specific parameters match the index type
	if isPod && in.ReadCapacity != nil {
		return nil, fmt.Errorf("cannot configure ReadCapacity on a pod index; ReadCapacity is only supported for serverless and BYOC indexes")
	}
	if !isPod && (podType != nil || replicas != nil) {
		return nil, fmt.Errorf("cannot configure PodType or Replicas on a non-pod index; these parameters are only supported for pod indexes")
	}

	var request db_control.ConfigureIndexRequest

	// Pod scaling nests under deployment (no deployment_type key is accepted).
	if podType != nil || replicas != nil {
		request.Deployment = &db_control.PatchIndexDeploymentRequest{
			PodType:  podType,
			Replicas: replicas,
		}
	}

	// Read capacity is a top-level patch covering managed and BYOC indexes.
	if in.ReadCapacity != nil {
		readCapacity, err := patchReadCapacity(in.ReadCapacity, idxDesc.ReadCapacity)
		if err != nil {
			return nil, err
		}
		request.ReadCapacity = readCapacity
	}

	if in.Schema != nil {
		fields := make(map[string]db_control.PatchSemanticTextField, len(in.Schema.Fields))
		for fieldName, field := range in.Schema.Fields {
			fields[fieldName] = db_control.PatchSemanticTextField{
				Type:            db_control.PatchSemanticTextFieldTypeSemanticText,
				Model:           field.Model,
				ReadParameters:  field.ReadParameters,
				WriteParameters: field.WriteParameters,
			}
		}
		request.Schema = &db_control.PatchIndexSchema{Fields: fields}
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

// ListCollections retrieves a list of all Collections in a Pinecone [project]. See [understanding collections] for more information.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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
// [understanding collections]: https://docs.pinecone.io/guides/indexes/pods/understanding-collections
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

	if collectionsResponse.Collections == nil {
		return nil, nil
	}

	var collections []*Collection
	for _, collectionModel := range *collectionsResponse.Collections {
		collections = append(collections, toCollection(&collectionModel))
	}

	return collections, nil
}

// DescribeCollection retrieves information about a specific [Collection]. See [understanding collections]
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    collection, err := pc.DescribeCollection(ctx, "my-collection")
//	    if err != nil {
//		       log.Fatalf("Error describing collection: %v", err)
//	    } else {
//		       fmt.Printf("Collection: %+v\n", *collection)
//	    }
//
// [dimensionality]: https://docs.pinecone.io/guides/indexes/pods/choose-a-pod-type-and-size#dimensionality-of-vectors
// [understanding collections]: https://docs.pinecone.io/guides/indexes/pods/understanding-collections
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

// CreateCollectionRequest holds the parameters for creating a new [Collection].
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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
	// Name (Required) is the name of the [Collection].
	Name string
	// Source (Required) is the name of the index to use as the source for the [Collection].
	Source string
}

// CreateCollection creates and initializes a new [Collection] via the specified [Client].
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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
	if in == nil {
		return nil, fmt.Errorf("in (*CreateCollectionRequest) cannot be nil")
	}
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

// DeleteCollection deletes a specific [Collection].
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    collectionName := "my-collection"
//
//	    err = pc.DeleteCollection(ctx, collectionName)
//	    if err != nil {
//		       log.Fatalf("Failed to delete collection: %v", err)
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

// CreateBackupParams contains the input parameters for creating a backup of a Pinecone index.
type CreateBackupParams struct {
	// IndexName (Required) is the name of the index to back up.
	IndexName string `json:"index_name"`
	// Description (Optional) is a description of the backup.
	Description *string `json:"description,omitempty"`
	// Name (Optional) is a name for the backup.
	Name *string `json:"name,omitempty"`
}

// CreateBackup creates a [Backup] for an index. The returned [Backup] starts with Status
// "Initializing"; poll [Client.DescribeBackup] until Status is "Ready" before restoring from it.
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
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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
//			    log.Fatalf("Failed to create backup: %v", err)
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

// CreateIndexFromBackupParams contains the parameters needed to create a Pinecone index from a backup.
type CreateIndexFromBackupParams struct {
	// BackupId (Required) is the unique identifier of the backup to restore from.
	BackupId string `json:"backup_id"`
	// Name (Required) is the name of the index to be created. Must be 1-45 characters, lowercase
	// alphanumeric or '-'.
	Name string `json:"name"`
	// DeletionProtection (Optional) determines whether deletion protection is "enabled" or
	// "disabled" for the new index.
	DeletionProtection *DeletionProtection `json:"deletion_protection,omitempty"`
	// Tags (Optional) are custom user tags added to the index. See [IndexTags] for the limits.
	Tags *IndexTags `json:"tags,omitempty"`
	// ReadCapacity (Optional) is the read capacity for the new index. Defaults to OnDemand when nil.
	// Dedicated capacity must be large enough to hold the backup's data.
	ReadCapacity *ReadCapacityParams `json:"read_capacity,omitempty"`
}

// CreateIndexFromBackupResponse contains the response returned after creating an index from a backup. RestoreJobId can be used
// to track the progress of an index restoration through the [Client.DescribeRestoreJob] method.
type CreateIndexFromBackupResponse struct {
	// IndexId is the ID of the index that was created from the backup.
	IndexId string `json:"index_id"`
	// RestoreJobId is the ID of the restore job initiated to restore the backup.
	RestoreJobId string `json:"restore_job_id"`
}

// CreateIndexFromBackup creates a new [Index] from a [Backup]. The new index inherits the schema of
// the backup's source index, including full-text-search fields and integrated embedding, and the
// request can't override it. Backups of pod-based and BYOC indexes can't be restored, and the backup
// must be Ready. The restore runs asynchronously; track it with [Client.DescribeRestoreJob] using the
// returned RestoreJobId.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
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
	readCapacity, err := readCapacityParamsToReadCapacity(in.ReadCapacity)
	if err != nil {
		return nil, err
	}

	res, err := c.restClient.CreateIndexFromBackupOperation(ctx, in.BackupId, &db_control.CreateIndexFromBackupOperationParams{XPineconeApiVersion: gen.PineconeApiVersion}, db_control.CreateIndexFromBackupRequest{
		Name:               in.Name,
		DeletionProtection: (*db_control.DeletionProtection)(in.DeletionProtection),
		Tags:               (*db_control.IndexTags)(in.Tags),
		ReadCapacity:       readCapacity,
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

// DescribeBackup describes a specific [Backup] by ID.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - backupId: The ID of the [Backup] to describe.
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
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    backup, err := pc.DescribeBackup(ctx, "my-backup-id")
//		if err != nil {
//			   log.Fatalf("Failed to describe backup ID %s: %v", "my-backup-id", err)
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

// ListBackupsParams contains the query parameters used when listing backups.
type ListBackupsParams struct {
	// IndexName (Optional) limits the results to backups of the given index. If nil, all backups in
	// the project are listed. The API returns a 404 error if no active index has this name, or, when
	// IncludeDeleted is true, if no index with this name ever existed.
	IndexName *string `json:"index_name,omitempty"`
	// Limit (Optional) is the maximum number of backups to return per page. Defaults to 100.
	Limit *int `json:"limit,omitempty"`
	// PaginationToken (Optional) is the token from a previous response used to retrieve the next
	// page of results.
	PaginationToken *string `json:"pagination_token,omitempty"`
	// IncludeDeleted (Optional), with IndexName, also lists backups of deleted indexes that had
	// that name. Requires IndexName.
	IncludeDeleted *bool `json:"include_deleted,omitempty"`
}

// ListBackups lists backups for a specific [Index], or all of the backups in a Pinecone project.
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
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    indexName := "my-index"
//	    limit := 5
//		backups, err := pc.ListBackups(ctx, &pinecone.ListBackupsParams{
//	           IndexName: &indexName,
//	           Limit: &limit,
//	    })
//	    if err != nil {
//			   log.Fatalf("Failed to list backups: %v", err)
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
		if in.IncludeDeleted != nil {
			return nil, fmt.Errorf("IncludeDeleted requires IndexName")
		}
		response, err = c.restClient.ListProjectBackups(ctx, &db_control.ListProjectBackupsParams{
			Limit:           in.Limit,
			PaginationToken: in.PaginationToken,
		})
		if err != nil {
			return nil, err
		}
	} else {
		response, err = c.restClient.ListIndexBackups(ctx, *in.IndexName, &db_control.ListIndexBackupsParams{
			IncludeDeleted:  in.IncludeDeleted,
			Limit:           in.Limit,
			PaginationToken: in.PaginationToken,
		})
		if err != nil {
			return nil, err
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(response, "failed to list backups: ")
	}
	return decodeBackupList(response.Body)
}

// DeleteBackup deletes a specific [Backup] by ID.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - backupId: The ID of the [Backup] to delete.
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
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//		err = pc.DeleteBackup(ctx, "my-backup-id")
//	    if err != nil {
//			   log.Fatalf("Failed to delete backup: %v", err)
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

// DescribeRestoreJob describes a specific [RestoreJob] by ID.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - restoreJobId: The ID of the [RestoreJob] to describe, as returned in
//     [CreateIndexFromBackupResponse].RestoreJobId.
//
// Returns a pointer to a [RestoreJob] object or an error.
//
// Example:
//
//		ctx := context.Background()
//
//		pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		       ApiKey: "YOUR_API_KEY",
//	    })
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    restoreJob, err := pc.DescribeRestoreJob(ctx, "my-restore-job-id")
//		if err != nil {
//			   log.Fatalf("Failed to describe restore job ID %s: %v", "my-restore-job-id", err)
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

// ListRestoreJobsParams contains the query parameters used when listing restore jobs.
type ListRestoreJobsParams struct {
	// Limit (Optional) is the maximum number of restore jobs to return.
	Limit *int `json:"limit,omitempty"`
	// PaginationToken (Optional) is the token from a previous response used to retrieve the next
	// page of results.
	PaginationToken *string `json:"pagination_token,omitempty"`
}

// ListRestoreJobs lists all restore jobs in a Pinecone project. Restore jobs are listed for the whole
// project and can't be filtered by index. When Pagination is non-nil, pass Pagination.Next as
// PaginationToken to fetch the next page.
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
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    limit := 5
//		restoreJobs, err := pc.ListRestoreJobs(ctx, &pinecone.ListRestoreJobsParams{Limit: &limit})
//	    if err != nil {
//			   log.Fatalf("Failed to list restore jobs: %v", err)
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

// InferenceService exposes methods for interacting with the [Pinecone Inference API]: generating
// embeddings, reranking documents, and describing hosted models. Access it through Client.Inference.
//
// [Pinecone Inference API]: https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models
type InferenceService struct {
	client *inference.Client
}

// EmbedRequest holds the parameters for generating embeddings for a list of input strings.
type EmbedRequest struct {
	// Model (Required) is the model to use for generating embeddings.
	Model string
	// TextInputs (Required) is the list of strings to generate embeddings for.
	TextInputs []string
	// Parameters (Optional) are additional model-specific parameters to use when generating
	// embeddings.
	Parameters EmbedParameters
}

// EmbedParameters contains model-specific parameters for generating embeddings. Keys and values are
// sent to the API as-is, so keys must use the API's snake_case names. The API rejects unknown keys
// and keys the model doesn't support; call [InferenceService.DescribeModel] and read
// SupportedParameters to see which keys a model accepts.
//
// Keys:
//   - "input_type": "query" or "passage". Required by asymmetric models such as
//     multilingual-e5-large, llama-text-embed-v2, and pinecone-sparse-english-v0.
//   - "truncate": "END" (default) or "NONE". With "NONE", an input longer than the model's maximum
//     token length returns an error.
//   - "dimension": (integer) the output dimension, for models that support more than one.
//   - "return_tokens": (boolean) sparse models only; return SparseEmbedding.SparseTokens.
//   - "max_tokens_per_sequence": (integer) for models that support it.
type EmbedParameters map[string]interface{}

// EmbedResponse holds the embeddings generated by [InferenceService.Embed], one [Embedding] per input,
// in the same order as EmbedRequest.TextInputs.
//
// [Total Tokens]: https://docs.pinecone.io/guides/manage-cost/understanding-cost#embedding
type EmbedResponse struct {
	// Data is the list of [Embedding] objects generated for the inputs.
	Data []Embedding `json:"data"`
	// Model is the model used to generate the embeddings.
	Model string `json:"model"`
	// VectorType indicates whether the embeddings are dense or sparse.
	VectorType string `json:"vector_type"`
	// Usage reports usage statistics ([Total Tokens]) for the request.
	Usage struct {
		// TotalTokens is the total number of tokens consumed across all inputs.
		TotalTokens *int32 `json:"total_tokens,omitempty"`
	} `json:"usage"`
}

// Embed generates embeddings for a list of inputs using the specified model and (optional) parameters.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to an EmbedRequest object that contains the model to use for embedding generation, the
//     list of input strings to generate embeddings for, and any additional parameters to use for generation.
//
// Returns a pointer to an [EmbedResponse] containing one [Embedding] per input, or an error.
//
// Example:
//
//	ctx := context.Background()
//
//	pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		ApiKey: "YOUR_API_KEY",
//	})
//	if err != nil {
//		log.Fatalf("Failed to create Client: %v", err)
//	}
//
//	res, err := pc.Inference.Embed(ctx, &pinecone.EmbedRequest{
//		Model:      "multilingual-e5-large",
//		TextInputs: []string{"Who created the first computer?"},
//		Parameters: pinecone.EmbedParameters{
//			"input_type": "passage",
//			"truncate":   "END",
//		},
//	})
//	if err != nil {
//		log.Fatalf("Failed to embed: %v", err)
//	}
//	fmt.Printf("Generated %d embeddings\n", len(res.Data))
func (i *InferenceService) Embed(ctx context.Context, in *EmbedRequest) (*EmbedResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*EmbedRequest) cannot be nil")
	}
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

// Document is a map representing a document. It is used both for the document operations on an
// [IndexConnection] (where it carries an "_id" field plus the document's field values) and as the
// document input to [InferenceService.Rerank].
type Document map[string]interface{}

// RerankRequest holds the parameters for calling [InferenceService.Rerank] and reranking documents
// by a specified query and model.
//
// [model]: https://docs.pinecone.io/guides/search/rerank-results#reranking-models
type RerankRequest struct {
	// Model (Required) is the [model] to use for reranking.
	Model string
	// Query (Required) is the query to rerank Documents against.
	Query string
	// Documents (Required) is the list of [Document] objects to be reranked. Documents are ranked by
	// their "text" field unless RankFields says otherwise.
	Documents []Document
	// RankFields (Optional) are the Document fields to rank by. Defaults to ["text"]. The number of
	// fields supported is model-specific, and every Document must contain each field listed.
	RankFields *[]string
	// ReturnDocuments (Optional) determines whether to include Documents in the response. Defaults
	// to true.
	ReturnDocuments *bool
	// TopN (Optional) is how many Documents to return. Defaults to the number of input Documents.
	TopN *int
	// Parameters (Optional) are additional model-specific parameters for the reranker.
	Parameters *map[string]interface{}
}

// RankedDocument represents a ranked document with a relevance score and an index position.
type RankedDocument struct {
	// Document is the ranked [Document]. It is nil if the request set ReturnDocuments to false.
	Document *Document `json:"document,omitempty"`
	// Index is the position of the Document in the original request.
	Index int `json:"index"`
	// Score is the relevance of the Document to the query, between 0 and 1, with scores closer to 1
	// indicating higher relevance.
	Score float32 `json:"score"`
}

// RerankResponse is the result of a reranking operation.
//
// [Rerank Units]: https://docs.pinecone.io/guides/manage-cost/understanding-cost#reranking
type RerankResponse struct {
	// Data is the list of reranked [RankedDocument] objects, sorted by relevance with the most
	// relevant first.
	Data []RankedDocument `json:"data,omitempty"`
	// Model is the model used to rerank the documents.
	Model string `json:"model"`
	// Usage reports usage statistics ([Rerank Units]) for the reranking operation.
	Usage RerankUsage `json:"usage"`
}

// Rerank reranks documents with associated relevance scores that represent the relevance of each [Document]
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
//	ctx := context.Background()
//
//	pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		ApiKey: "YOUR_API_KEY",
//	})
//	if err != nil {
//		log.Fatalf("Failed to create Client: %v", err)
//	}
//
//	topN := 2
//	returnDocuments := true
//	documents := []pinecone.Document{
//		{"id": "doc1", "text": "Apple is a popular fruit known for its sweetness and crisp texture."},
//		{"id": "doc2", "text": "Many people enjoy eating apples as a healthy snack."},
//		{"id": "doc3", "text": "Apple Inc. has revolutionized the tech industry with its sleek designs and user-friendly interfaces."},
//		{"id": "doc4", "text": "An apple a day keeps the doctor away, as the saying goes."},
//	}
//
//	ranking, err := pc.Inference.Rerank(ctx, &pinecone.RerankRequest{
//		Model:           "bge-reranker-v2-m3",
//		Query:           "i love to eat apples",
//		ReturnDocuments: &returnDocuments,
//		TopN:            &topN,
//		RankFields:      &[]string{"text"},
//		Documents:       documents,
//	})
//	if err != nil {
//		log.Fatalf("Failed to rerank: %v", err)
//	}
//	fmt.Printf("Rerank result: %+v\n", ranking)
func (i *InferenceService) Rerank(ctx context.Context, in *RerankRequest) (*RerankResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*RerankRequest) cannot be nil")
	}
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

// DescribeModel gets a description of a model hosted by Pinecone.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - modelName: (Required) The exact name of the model to describe, e.g. "multilingual-e5-large".
//     Call [InferenceService.ListModels] to see the available names. An unknown name returns a 404
//     [PineconeError].
//
// Returns a pointer to a [ModelInfo] object or an error.
//
// Example:
//
//	ctx := context.Background()
//
//	pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		ApiKey: "YOUR_API_KEY",
//	})
//	if err != nil {
//		log.Fatalf("Failed to create Client: %v", err)
//	}
//
//	model, err := pc.Inference.DescribeModel(ctx, "multilingual-e5-large")
//	if err != nil {
//		log.Fatalf("Failed to describe model: %v", err)
//	}
//	fmt.Printf("Model (multilingual-e5-large): %+v\n", model)
func (i *InferenceService) DescribeModel(ctx context.Context, modelName string) (*ModelInfo, error) {
	if modelName == "" {
		return nil, fmt.Errorf("modelName must not be empty")
	}
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

// ListModelsParams holds the parameters for filtering model results when calling [InferenceService.ListModels].
type ListModelsParams struct {
	// Type (Optional) is the type of model to filter by: "embed" or "rerank".
	Type *string
	// VectorType (Optional) is the vector type to filter by: "dense" or "sparse". Setting it implies
	// Type "embed"; combining it with Type "rerank" returns an error from the API.
	VectorType *string
}

// ListModels lists all available models hosted by Pinecone. You can filter results using [ListModelsParams].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime, allowing for the request
//     to be canceled or to timeout according to the context's deadline.
//   - in: An optional [ListModelsParams] for filtering by model type and vector type. Pass nil to
//     list all models.
//
// Returns a pointer to a [ModelInfoList] object or an error.
//
// Example:
//
//	ctx := context.Background()
//
//	pc, err := pinecone.NewClient(pinecone.NewClientParams{
//		ApiKey: "YOUR_API_KEY",
//	})
//	if err != nil {
//		log.Fatalf("Failed to create Client: %v", err)
//	}
//
//	embed := "embed"
//	embedModels, err := pc.Inference.ListModels(ctx, &pinecone.ListModelsParams{Type: &embed})
//	if err != nil {
//		log.Fatalf("Failed to list models: %v", err)
//	}
//	fmt.Printf("Embed models: %+v\n", embedModels)
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
		if slices.Contains(possibleAuthKeys, strings.ToLower(key)) {
			return map[string]string{key: value}
		}
	}

	return nil
}

func toIndex(idx *db_control.IndexModel) (*Index, error) {
	if idx == nil {
		return nil, nil
	}

	deployment := toIndexDeployment(idx.Deployment)

	readCapacity, err := toReadCapacity(idx.ReadCapacity)
	if err != nil {
		return nil, err
	}

	status := &IndexStatus{
		Ready: idx.Status.Ready,
		State: IndexStatusState(idx.Status.State),
	}

	tags := (*IndexTags)(idx.Tags)
	deletionProtection := valueOrFallback(string(idx.DeletionProtection), "disabled")

	index := &Index{
		Name:               idx.Name,
		Host:               idx.Host,
		PrivateHost:        idx.PrivateHost,
		Schema:             toIndexSchema(&idx.Schema),
		Deployment:         deployment,
		ReadCapacity:       readCapacity,
		SourceCollection:   idx.SourceCollection,
		SourceBackupId:     idx.SourceBackupId,
		CmekId:             idx.CmekId,
		DeletionProtection: DeletionProtection(deletionProtection),
		Status:             status,
		Tags:               tags,
	}

	// Populate the deprecated computed fields (Metric, VectorType, Dimension, Spec, Embed).
	applyIndexCompatFields(index)

	return index, nil
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

	var createdAt *string
	if backup.CreatedAt != nil {
		formatted := backup.CreatedAt.Format(time.RFC3339Nano)
		createdAt = &formatted
	}

	result := &Backup{
		BackupId:             backup.BackupId,
		Cloud:                backup.Cloud,
		CreatedAt:            createdAt,
		Description:          backup.Description,
		Name:                 backup.Name,
		NamespaceCount:       backup.NamespaceCount,
		RecordCount:          backup.RecordCount,
		Region:               backup.Region,
		Schema:               toIndexSchema(backup.Schema),
		SizeBytes:            backup.SizeBytes,
		SourceIndexDeletedAt: backup.SourceIndexDeletedAt,
		SourceIndexId:        backup.SourceIndexId,
		SourceIndexName:      backup.SourceIndexName,
		Status:               backup.Status,
		Tags:                 (*IndexTags)(backup.Tags),
	}

	// Populate the deprecated computed Dimension/Metric from the schema's dense vector field.
	if dense := denseFieldForCompat(result.Schema); dense != nil {
		dimension := dense.Dimension
		metric := dense.Metric
		result.Dimension = &dimension
		result.Metric = &metric
	}

	return result
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

	var percentComplete *float32
	if restoreJob.PercentComplete != nil {
		converted := float32(*restoreJob.PercentComplete)
		percentComplete = &converted
	}

	return &RestoreJob{
		BackupId:        restoreJob.BackupId,
		CompletedAt:     restoreJob.CompletedAt,
		CreatedAt:       derefOrDefault(restoreJob.CreatedAt, time.Time{}),
		PercentComplete: percentComplete,
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
				return nil, fmt.Errorf("failed to decode DenseEmbedding: %w", err)
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
		Data:       decodedEmbeddings,
		Model:      rawEmbedResponse.Model,
		VectorType: rawEmbedResponse.VectorType,
		Usage:      rawEmbedResponse.Usage,
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

func validateVectorType(vectorType *string, dimension *int32, metric *IndexMetric) (string, error) {
	// Default to dense if vectorType is not specified
	out := "dense"

	if vectorType != nil {
		switch *vectorType {
		case "sparse":
			if dimension != nil {
				return "", fmt.Errorf("Dimension should not be specified when VectorType is 'sparse'")
			} else if metric != nil && *metric != IndexMetricDotproduct {
				return "", fmt.Errorf("Metric should be 'dotproduct' when VectorType is 'sparse'")
			}
		case "dense":
			if dimension == nil {
				return "", fmt.Errorf("Dimension should be specified when VectorType is 'dense'")
			}
		default:
			return "", fmt.Errorf("unsupported VectorType: %s", *vectorType)
		}
		out = *vectorType
	}
	return out, nil
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

// Converts MetadataSchema to the inline struct defined in the generated REST API
func fromMetadataSchemaToRest(schema *MetadataSchema) *db_control.MetadataSchema {
	if schema == nil {
		return nil
	}

	fields := make(map[string]struct {
		Filterable db_control.MetadataSchemaFieldsFilterable `json:"filterable"`
	})

	for key, value := range schema.Fields {
		fields[key] = struct {
			Filterable db_control.MetadataSchemaFieldsFilterable `json:"filterable"`
		}{
			Filterable: db_control.MetadataSchemaFieldsFilterable(value.Filterable),
		}
	}

	return &db_control.MetadataSchema{
		Fields: &fields,
	}
}

// Takes the new ReadCapacityParams and the index's current ReadCapacity configuration and builds the
// read-capacity PATCH for ConfigureIndex. Fields omitted from a Dedicated patch keep their current value.
func patchReadCapacity(new *ReadCapacityParams, old *ReadCapacity) (*db_control.ReadCapacityPatch, error) {
	// nil new params -> return nil
	if new == nil || (new.Dedicated == nil && new.OnDemand == nil) {
		return nil, nil
	}

	if new.Dedicated != nil && new.OnDemand != nil {
		return nil, fmt.Errorf("both Dedicated and OnDemand cannot be specified in ReadCapacityParams")
	}

	var result db_control.ReadCapacityPatch

	if new.OnDemand != nil {
		if err := result.FromReadCapacityOnDemandSpec(db_control.ReadCapacityOnDemandSpec{Mode: "OnDemand"}); err != nil {
			return nil, err
		}
		return &result, nil
	}

	// nil / OnDemand -> Dedicated
	// When converting from OnDemand to Dedicated, NodeType, Replicas, and Shards are required
	if old == nil || old.OnDemand != nil {
		if new.Dedicated.NodeType == nil ||
			new.Dedicated.Scaling == nil ||
			new.Dedicated.Scaling.Manual == nil ||
			new.Dedicated.Scaling.Manual.Replicas == nil ||
			new.Dedicated.Scaling.Manual.Shards == nil {
			return nil, fmt.Errorf("Dedicated read capacity must be configured with a node type, scaling strategy, and manual scaling configuration")
		}
	}

	patchConfig := db_control.ReadCapacityDedicatedPatchConfig{
		NodeType: new.Dedicated.NodeType,
	}
	if new.Dedicated.Scaling != nil && new.Dedicated.Scaling.Manual != nil {
		patchConfig.Scaling = pointerOrNil("Manual")
		patchConfig.Manual = &db_control.ScalingConfigManualPatch{
			Replicas: new.Dedicated.Scaling.Manual.Replicas,
			Shards:   new.Dedicated.Scaling.Manual.Shards,
		}
	}

	if err := result.FromReadCapacityDedicatedPatchSpec(db_control.ReadCapacityDedicatedPatchSpec{
		Dedicated: patchConfig,
		Mode:      "Dedicated",
	}); err != nil {
		return nil, err
	}

	return &result, nil
}

// Converts the ReadCapacityParams to db_control.ReadCapacity - used for CreateServerlessIndex,
// CreateBYOCIndex, CreateIndex, CreateIndexForModel, and CreateIndexFromBackup operations
func readCapacityParamsToReadCapacity(request *ReadCapacityParams) (*db_control.ReadCapacity, error) {
	// If no ReadCapacityParams provided or if it's an empty struct, return nil to use server default (OnDemand)
	if request == nil || (request.Dedicated == nil && request.OnDemand == nil) {
		return nil, nil
	}

	if request.Dedicated != nil && request.OnDemand != nil {
		return nil, fmt.Errorf("both Dedicated and OnDemand cannot be specified in ReadCapacityParams")
	}

	var result db_control.ReadCapacity

	// OnDemand
	if request.OnDemand != nil {
		onDemandSpec := db_control.ReadCapacityOnDemandSpec{
			Mode: "OnDemand",
		}
		if err := result.FromReadCapacityOnDemandSpec(onDemandSpec); err != nil {
			return nil, err
		}
		return &result, nil
	}

	// Dedicated: the 2026-07 create request requires node type and a manual scaling configuration.
	if request.Dedicated.NodeType == nil ||
		request.Dedicated.Scaling == nil ||
		request.Dedicated.Scaling.Manual == nil ||
		request.Dedicated.Scaling.Manual.Replicas == nil ||
		request.Dedicated.Scaling.Manual.Shards == nil {
		return nil, fmt.Errorf("Dedicated read capacity must be configured with a node type, scaling strategy, and manual scaling configuration")
	}

	dedicatedSpec := db_control.ReadCapacityDedicatedSpec{
		Dedicated: db_control.ReadCapacityDedicatedConfig{
			NodeType: *request.Dedicated.NodeType,
			Scaling:  "Manual",
			Manual: db_control.ScalingConfigManual{
				Replicas: *request.Dedicated.Scaling.Manual.Replicas,
				Shards:   *request.Dedicated.Scaling.Manual.Shards,
			},
		},
		Mode: "Dedicated",
	}
	if err := result.FromReadCapacityDedicatedSpec(dedicatedSpec); err != nil {
		return nil, err
	}

	return &result, nil
}

// Converts the db_control.ReadCapacityResponse to ReadCapacity.
// This function is permissive: it returns nil for unknown/empty modes rather than erroring,
// allowing the SDK to continue working even if the API introduces new read capacity modes.
func toReadCapacity(rc *db_control.ReadCapacityResponse) (*ReadCapacity, error) {
	if rc == nil {
		return nil, nil
	}

	mode, err := rc.Discriminator()
	if err != nil {
		// If we can't determine the mode, return nil rather than failing.
		// This handles cases like empty responses or unknown modes gracefully.
		return nil, nil
	}

	// Empty mode string means no configuration present
	if mode == "" {
		return nil, nil
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

		nodeType := dedicatedSpec.Dedicated.NodeType
		dedicated := &ReadCapacityDedicated{
			NodeType: &nodeType,
			Status: ReadCapacityStatus{
				State:           dedicatedSpec.Status.State,
				CurrentReplicas: dedicatedSpec.Status.CurrentReplicas,
				CurrentShards:   dedicatedSpec.Status.CurrentShards,
				ErrorMessage:    dedicatedSpec.Status.ErrorMessage,
			},
		}

		// Scaling
		if strings.EqualFold(dedicatedSpec.Dedicated.Scaling, "manual") {
			replicas := dedicatedSpec.Dedicated.Manual.Replicas
			shards := dedicatedSpec.Dedicated.Manual.Shards
			dedicated.Scaling = &ReadCapacityScaling{
				Manual: &ReadCapacityManualScaling{
					Replicas: &replicas,
					Shards:   &shards,
				},
			}
		}

		return &ReadCapacity{
			Dedicated: dedicated,
		}, nil
	default:
		// Be permissive: return nil for unknown modes (e.g., future API additions)
		// rather than failing the entire operation
		return nil, nil
	}
}
