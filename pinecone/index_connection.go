package pinecone

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/pinecone-io/go-pinecone/v6/internal/gen"
	db_data_grpc "github.com/pinecone-io/go-pinecone/v6/internal/gen/db_data/grpc"
	db_data_rest "github.com/pinecone-io/go-pinecone/v6/internal/gen/db_data/rest"
	"github.com/pinecone-io/go-pinecone/v6/internal/useragent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// IndexConnection is a data-plane client for one index host, targeting one namespace. Create one
// with [Client.Index]; use [IndexConnection.WithNamespace] to target another namespace over the
// same gRPC connection.
//
// Vector, namespace, and stats methods use gRPC. On failure they return the gRPC status error
// unchanged (inspect it with status.FromError), not a [PineconeError]. Records, import, and
// document methods use REST and return a [PineconeError] for non-success responses.
//
// When the Client has a RetryPolicy, gRPC calls that fail with RESOURCE_EXHAUSTED or UNAVAILABLE
// are retried on every method, writes included.
type IndexConnection struct {
	namespace          string
	additionalMetadata map[string]string
	restClient         *db_data_rest.Client
	grpcClient         *db_data_grpc.VectorServiceClient
	grpcConn           *grpc.ClientConn
}

type newIndexParameters struct {
	host               string
	namespace          string
	sourceTag          string
	additionalMetadata map[string]string
	dbDataClient       *db_data_rest.Client
}

func newIndexConnection(in newIndexParameters, dialOpts ...grpc.DialOption) (*IndexConnection, error) {
	target, isSecure := normalizeHost(in.host)

	// configure default gRPC DialOptions
	grpcOptions := []grpc.DialOption{
		grpc.WithAuthority(target),
		grpc.WithUserAgent(useragent.BuildUserAgentGRPC(in.sourceTag)),
	}

	if isSecure {
		config := &tls.Config{}
		grpcOptions = append(grpcOptions, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	} else {
		grpcOptions = append(grpcOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// if we have user-provided dialOpts, append them to the defaults here
	dialOpts = append(grpcOptions, dialOpts...)

	conn, err := grpc.NewClient(
		target,
		dialOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	dataClient := db_data_grpc.NewVectorServiceClient(conn)

	idx := IndexConnection{
		namespace:          in.namespace,
		restClient:         in.dbDataClient,
		grpcClient:         &dataClient,
		grpcConn:           conn,
		additionalMetadata: in.additionalMetadata,
	}
	return &idx, nil
}

// Close closes the underlying gRPC connection. Connections derived with
// [IndexConnection.WithNamespace] share it, so closing any of them closes all of them; call Close
// once, after every derived connection is done.
//
// Returns an error if the connection cannot be closed, otherwise returns nil.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection: %v", err)
//	    }
//
//	    err = idxConnection.Close()
//	    if err != nil {
//			log.Fatalf("Failed to close index connection. Error: %v", err)
//	    }
func (idx *IndexConnection) Close() error {
	err := idx.grpcConn.Close()
	return err
}

// Namespace returns the namespace this connection targets, as it was set. "" means the default
// namespace, which the API reports as "__default__".
func (idx *IndexConnection) Namespace() string {
	return idx.namespace
}

// WithNamespace creates a new copy of [IndexConnection] that targets a new namespace within that index while
// sharing the underlying gRPC connection. This is useful for performing operations across namespaces in an index without re-creating the index connection.
//
// Example:
//
//	    ctx := context.Background()
//		clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//		}
//
//		pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//		idxConnNs1, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "namespace1"})
//		if err != nil {
//			log.Fatalf("Failed to create IndexConnection: %v", err)
//		}
//
//		metadataMap := map[string]interface{}{
//			"genre": "classical",
//		}
//		metadata, err := pinecone.NewMetadata(metadataMap)
//		if err != nil {
//				log.Fatalf("Failed to create metadata map. Error: %v", err)
//		}
//
//		values := []float32{1.0, 2.0}
//		vectors := []*pinecone.Vector{
//			{
//				Id:       "abc-1",
//				Values:   &values,
//				Metadata: metadata,
//			},
//		}
//
//		_, err = idxConnNs1.UpsertVectors(ctx, vectors)
//		if err != nil {
//			log.Fatalf("Failed to upsert vectors in %s. Error: %v", idxConnNs1.Namespace(), err)
//		}
//		idxConnNs2 := idxConnNs1.WithNamespace("namespace2")
//		_, err = idxConnNs2.UpsertVectors(ctx, vectors)
//		if err != nil {
//			log.Fatalf("Failed to upsert vectors in %s. Error: %v", idxConnNs2.Namespace(), err)
//		}
func (idx *IndexConnection) WithNamespace(namespace string) *IndexConnection {
	return &IndexConnection{
		namespace:          namespace,
		additionalMetadata: idx.additionalMetadata,
		restClient:         idx.restClient,
		grpcClient:         idx.grpcClient,
		grpcConn:           idx.grpcConn,
	}
}

// UpsertVectors writes vectors into the connection's namespace, overwriting any vector with the
// same ID. It sends a single request and does no batching; the API accepts at most 1000 vectors per
// request, and request size is also capped. Each vector needs Values, SparseValues, or both.
// Metadata values must be strings, numbers, booleans, or lists of strings.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: The vectors to upsert.
//
// Returns the number of vectors upserted or an error if the request fails.
//
// Example:
//
//		ctx := context.Background()
//		clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//		}
//
//		pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//		idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//		if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//		metadataMap := map[string]interface{}{
//			"genre": "classical",
//		}
//		metadata, err := pinecone.NewMetadata(metadataMap)
//		if err != nil {
//			log.Fatalf("Failed to create metadata map. Error: %v", err)
//		}
//		denseValues := []float32{1.0, 2.0}
//
//		sparseValues := pinecone.SparseValues{
//			Indices: []uint32{0, 1},
//			Values:  []float32{1.0, 2.0},
//		}
//
//		vectors := []*pinecone.Vector{
//			{
//				Id:           "abc-1",
//				Values:       &denseValues,
//				Metadata:     metadata,
//				SparseValues: &sparseValues,
//			},
//		}
//
//		count, err := idxConnection.UpsertVectors(ctx, vectors)
//		if err != nil {
//	    		log.Fatalf("Failed to upsert vectors. Error: %v", err)
//		} else {
//				log.Printf("Successfully upserted %d vector(s)!\n", count)
//		}
func (idx *IndexConnection) UpsertVectors(ctx context.Context, in []*Vector) (uint32, error) {
	vectors := make([]*db_data_grpc.Vector, len(in))
	for i, v := range in {
		if v != nil {
			if err := validateMetadata(v.Metadata); err != nil {
				return 0, err
			}
		}
		vectors[i] = vecToGrpc(v)
	}

	req := &db_data_grpc.UpsertRequest{
		Vectors:   vectors,
		Namespace: idx.namespace,
	}

	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	res, err := (*idx.grpcClient).Upsert(ctx, req)
	if err != nil {
		return 0, err
	}
	return res.UpsertedCount, nil
}

// UpdateVectorRequest holds the parameters for the [IndexConnection.UpdateVector] method.
type UpdateVectorRequest struct {
	// Id is the unique ID of the vector to update.
	Id string
	// Values are the values with which you want to update the vector.
	Values []float32
	// SparseValues are the sparse values with which you want to update the vector.
	SparseValues *SparseValues
	// Metadata holds metadata keys to set or overwrite. Keys not listed keep their current values;
	// an update never removes a key. Null values are rejected.
	Metadata *Metadata
}

// UpdateVector updates a vector in the connection's namespace by ID. Id plus at least one of
// Values, SparseValues, or Metadata is required. Values and SparseValues replace the stored values;
// Metadata is merged into the stored metadata.
//
// Returns an error if the request fails, returns nil otherwise.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: An [UpdateVectorRequest] object with the parameters for the request.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    id := "abc-1"
//
//	    err = idxConnection.UpdateVector(ctx, &pinecone.UpdateVectorRequest{
//			Id:     id,
//			Values: []float32{7.0, 8.0},
//	    })
//
//	    if err != nil {
//			log.Fatalf("Failed to update vector with ID %s. Error: %s", id, err)
//	    }
func (idx *IndexConnection) UpdateVector(ctx context.Context, in *UpdateVectorRequest) error {
	if in == nil {
		return fmt.Errorf("in (*UpdateVectorRequest) cannot be nil")
	}
	if err := validateMetadata(in.Metadata); err != nil {
		return err
	}
	hasId := in.Id != ""

	// Validate mutual exclusivity of Id and Filter
	if !hasId {
		return fmt.Errorf("an Id value must be provided to update a vector")
	}

	// Validate Id-based filtering
	if in.Values == nil && in.SparseValues == nil && in.Metadata == nil {
		return fmt.Errorf("a vector Id plus at least one of Values, SparseValues, or Metadata must be provided to update a vector")
	}

	req := &db_data_grpc.UpdateRequest{
		Id:           in.Id,
		Values:       in.Values,
		SparseValues: sparseValToGrpc(in.SparseValues),
		SetMetadata:  in.Metadata,
		Namespace:    idx.namespace,
	}

	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	// Updating a vector by Id without a filter makes the UpdateResponse empty, so we ignore it here
	_, err := (*idx.grpcClient).Update(ctx, req)
	return err
}

// UpdateVectorsByMetadataRequest holds the parameters for the [IndexConnection.UpdateVectorsByMetadata] method.
type UpdateVectorsByMetadataRequest struct {
	// Filter (Required) is the metadata filter used to match vectors. It must contain at least one
	// condition; an empty filter is rejected.
	Filter *MetadataFilter
	// Metadata (Required) holds metadata keys to set or overwrite on every matched vector. Keys
	// not listed keep their current values. Null values are rejected.
	Metadata *Metadata
	// DryRun (Optional), if true, returns the number of vectors that match the filter without
	// executing the update. Default is false.
	DryRun *bool
}

// UpdateVectorsByMetadataResponse is returned by the [IndexConnection.UpdateVectorsByMetadata] method.
type UpdateVectorsByMetadataResponse struct {
	// MatchedRecords is the number of vectors that matched the filter.
	MatchedRecords int32 `json:"matched_records,omitempty"`
}

// UpdateVectorsByMetadata updates vectors in a Pinecone [Index] that match a metadata filter.
// You can update metadata for all vectors that match the filter criteria, and optionally use DryRun to
// count how many vectors would be updated without actually performing the update.
//
// Returns a pointer to an [UpdateVectorsByMetadataResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: An [UpdateVectorsByMetadataRequest] object with the parameters for the request. The Filter and Metadata fields are required.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    filterMap := map[string]interface{}{
//			"genre": map[string]interface{}{
//				"$eq": "rock",
//		    },
//	    }
//
//	    filter, err := pinecone.NewMetadataFilter(filterMap)
//	    if err != nil {
//			log.Fatalf("Failed to create metadata filter. Error: %v", err)
//	    }
//
//	    metadataMap := map[string]interface{}{
//			"genre":   "rock",
//			"year":    2021,
//			"updated": true,
//	    }
//
//	    metadata, err := pinecone.NewMetadata(metadataMap)
//	    if err != nil {
//			log.Fatalf("Failed to create metadata. Error: %v", err)
//	    }
//
//	    res, err := idxConnection.UpdateVectorsByMetadata(ctx, &pinecone.UpdateVectorsByMetadataRequest{
//			Filter:   filter,
//			Metadata: metadata,
//	    })
//
//	    if err != nil {
//			log.Fatalf("Failed to update vectors by metadata. Error: %s", err)
//	    }
//
//	    fmt.Printf("Updated %d vector(s)\n", res.MatchedRecords)
func (idx *IndexConnection) UpdateVectorsByMetadata(ctx context.Context, in *UpdateVectorsByMetadataRequest) (*UpdateVectorsByMetadataResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateVectorsByMetadataRequest) cannot be nil")
	}
	if err := validateNonEmptyFilter(in.Filter); err != nil {
		return nil, fmt.Errorf("Filter is required to update vectors by metadata: %w", err)
	}
	if in.Metadata == nil {
		return nil, fmt.Errorf("Metadata is required to update vectors by metadata")
	}
	if err := validateMetadata(in.Metadata); err != nil {
		return nil, err
	}

	req := &db_data_grpc.UpdateRequest{
		Filter:      in.Filter,
		SetMetadata: in.Metadata,
		DryRun:      in.DryRun,
		Namespace:   idx.namespace,
	}

	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	res, err := (*idx.grpcClient).Update(ctx, req)
	if err != nil {
		return nil, err
	}

	if res != nil && res.MatchedRecords != nil {
		return &UpdateVectorsByMetadataResponse{
			MatchedRecords: *res.MatchedRecords,
		}, nil
	} else {
		return &UpdateVectorsByMetadataResponse{
			MatchedRecords: 0,
		}, nil
	}
}

// FetchVectorsResponse is returned by the [IndexConnection.FetchVectors] method.
type FetchVectorsResponse struct {
	// Vectors are the fetched vectors, keyed by ID.
	Vectors map[string]*Vector `json:"vectors,omitempty"`
	// Usage is the usage information for the request.
	Usage *Usage `json:"usage,omitempty"`
	// Namespace is the namespace from which the vectors were fetched.
	Namespace string `json:"namespace"`
}

// FetchVectors fetches vectors by ID from the connection's namespace. IDs that are not found are
// absent from the returned map rather than reported as an error. ids must be non-empty and each ID
// must be 1–512 characters; violations are rejected. The API accepts at most 1000 IDs per request.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - ids: The unique IDs of the vectors to fetch.
//
// Returns a pointer to any fetched vectors or an error if the request fails.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    res, err := idxConnection.FetchVectors(ctx, []string{"abc-1"})
//	    if err != nil {
//			log.Fatalf("Failed to fetch vectors, error: %+v", err)
//	    }
//
//	    if len(res.Vectors) != 0 {
//			fmt.Println(res)
//	    } else {
//			fmt.Println("No vectors found")
//	    }
func (idx *IndexConnection) FetchVectors(ctx context.Context, ids []string) (*FetchVectorsResponse, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids must contain at least one vector ID")
	}
	for i, id := range ids {
		if err := validateVectorId(fmt.Sprintf("ids[%d]", i), id); err != nil {
			return nil, err
		}
	}
	req := &db_data_grpc.FetchRequest{
		Ids:       ids,
		Namespace: idx.namespace,
	}

	res, err := (*idx.grpcClient).Fetch(idx.akCtx(ctx), req)
	if err != nil {
		return nil, err
	}

	vectors := make(map[string]*Vector, len(res.Vectors))
	for id, vector := range res.Vectors {
		vectors[id] = toVector(vector)
	}

	return &FetchVectorsResponse{
		Vectors:   vectors,
		Usage:     toUsage(res.Usage),
		Namespace: res.Namespace,
	}, nil
}

// FetchVectorsByMetadataRequest holds the parameters passed into the [IndexConnection.FetchVectorsByMetadata] method.
type FetchVectorsByMetadataRequest struct {
	// Filter (Required) is the metadata filter used to match vectors. It must contain at least one
	// condition; an empty filter is rejected.
	Filter *MetadataFilter
	// Limit (Optional) is the maximum number of vectors per page, 1–10000. Defaults to 100.
	Limit *uint32
	// PaginationToken (Optional) is the token for paginating through results. Use it to
	// continue a previous listing operation.
	PaginationToken *string
	// Namespace (Optional) is the namespace from which to fetch vectors. If nil, the connection's
	// namespace is used.
	Namespace *string
}

// FetchVectorsByMetadataResponse is returned by the [IndexConnection.FetchVectorsByMetadata] method.
type FetchVectorsByMetadataResponse struct {
	// Vectors are the fetched vectors, keyed by ID.
	Vectors map[string]*Vector `json:"vectors,omitempty"`
	// Usage is the usage information for the request.
	Usage *Usage `json:"usage,omitempty"`
	// Namespace is the namespace from which the vectors were fetched.
	Namespace string `json:"namespace"`
	// Pagination holds the token for the next page, or is nil when there are no more results.
	Pagination *Pagination `json:"pagination,omitempty"`
}

// FetchVectorsByMetadata fetches vectors matching a metadata filter. You can filter vectors
// by metadata, limit the number of vectors returned, and paginate through results.
//
// Returns a pointer to a [FetchVectorsByMetadataResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A [FetchVectorsByMetadataRequest] object with the parameters for the request. The Filter field is required.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    limit := uint32(10)
//
//	    metadataMap := map[string]interface{}{
//			"genre": map[string]interface{}{
//				"$eq": "action",
//			},
//	    }
//
//	    filter, err := structpb.NewStruct(metadataMap)
//	    if err != nil {
//			log.Fatalf("Failed to create metadata filter. Error: %v", err)
//	    }
//
//	    res, err := idxConnection.FetchVectorsByMetadata(ctx, &pinecone.FetchVectorsByMetadataRequest{
//			Filter: filter,
//			Limit:  &limit,
//	    })
//	    if err != nil {
//			log.Fatalf("Failed to fetch vectors by metadata, error: %+v", err)
//	    }
//
//	    if len(res.Vectors) != 0 {
//			fmt.Printf("Found %d vector(s)\n", len(res.Vectors))
//	    } else {
//			fmt.Println("No vectors found")
//	    }
func (idx *IndexConnection) FetchVectorsByMetadata(ctx context.Context, in *FetchVectorsByMetadataRequest) (*FetchVectorsByMetadataResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*FetchVectorsByMetadataRequest) cannot be nil")
	}
	if err := validateNonEmptyFilter(in.Filter); err != nil {
		return nil, fmt.Errorf("Filter is required to fetch vectors by metadata: %w", err)
	}
	if in.Limit != nil && (*in.Limit < minListLimit || *in.Limit > maxFetchByMetadataLimit) {
		return nil, fmt.Errorf("Limit must be between %d and %d, got %d", minListLimit, maxFetchByMetadataLimit, *in.Limit)
	}

	namespace := idx.namespace
	if in.Namespace != nil {
		namespace = *in.Namespace
	}

	req := &db_data_grpc.FetchByMetadataRequest{
		Namespace:       namespace,
		Filter:          in.Filter,
		Limit:           in.Limit,
		PaginationToken: in.PaginationToken,
	}

	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	res, err := (*idx.grpcClient).FetchByMetadata(ctx, req)
	if err != nil {
		return nil, err
	}

	vectors := make(map[string]*Vector, len(res.Vectors))
	for id, vector := range res.Vectors {
		vectors[id] = toVector(vector)
	}

	var pagination *Pagination
	if res.Pagination != nil {
		pagination = &Pagination{
			Next: res.Pagination.Next,
		}
	}

	return &FetchVectorsByMetadataResponse{
		Vectors:    vectors,
		Usage:      toUsage(res.Usage),
		Namespace:  res.Namespace,
		Pagination: pagination,
	}, nil
}

// ListVectorsRequest holds the parameters passed into the [IndexConnection.ListVectors] method.
type ListVectorsRequest struct {
	// Prefix (Optional) limits results to IDs starting with this value. Leave it nil to list every
	// ID; a non-nil Prefix must be 1–512 characters.
	Prefix *string
	// Limit (Optional) is the maximum number of IDs per page, 1–100. Defaults to 100.
	Limit *uint32
	// PaginationToken (Optional) is the token for paginating through results.
	PaginationToken *string
}

// ListVectorsResponse is returned by the [IndexConnection.ListVectors] method.
type ListVectorsResponse struct {
	// VectorIds are the unique IDs of the returned vectors.
	VectorIds []*string `json:"vector_ids,omitempty"`
	// Usage is the usage information for the request.
	Usage *Usage `json:"usage,omitempty"`
	// NextPaginationToken is the token for the next page, or nil when there are no more results.
	NextPaginationToken *string `json:"next_pagination_token,omitempty"`
	// Namespace is the namespace the vector IDs are listed from.
	Namespace string `json:"namespace"`
}

// ListVectors lists vectors in a Pinecone index. You can filter vectors by prefix,
// limit the number of vectors returned, and paginate through results.
//
// Note: ListVectors is only available for Serverless indexes.
//
// Returns a pointer to a [ListVectorsResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A [ListVectorsRequest] object with the parameters for the request.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    prefix := "abc"
//	    limit := uint32(10)
//
//	    res, err := idxConnection.ListVectors(ctx, &pinecone.ListVectorsRequest{
//			Prefix: &prefix,
//			Limit:  &limit,
//	    })
//
//	    if err != nil {
//			log.Fatalf("Failed to list vectors in index: %s. Error: %s\n", idx.Name, err)
//	    }
//
//	    if len(res.VectorIds) == 0 {
//			fmt.Println("No vectors found")
//	    } else {
//			fmt.Printf("Found %d vector(s)\n", len(res.VectorIds))
//	    }
func (idx *IndexConnection) ListVectors(ctx context.Context, in *ListVectorsRequest) (*ListVectorsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*ListVectorsRequest) cannot be nil")
	}
	if in.Prefix != nil {
		if err := validateVectorId("Prefix", *in.Prefix); err != nil {
			return nil, err
		}
	}
	if in.Limit != nil && (*in.Limit < minListLimit || *in.Limit > maxListLimit) {
		return nil, fmt.Errorf("Limit must be between %d and %d, got %d", minListLimit, maxListLimit, *in.Limit)
	}
	req := &db_data_grpc.ListRequest{
		Prefix:          in.Prefix,
		Limit:           in.Limit,
		PaginationToken: in.PaginationToken,
		Namespace:       idx.namespace,
	}
	res, err := (*idx.grpcClient).List(idx.akCtx(ctx), req)
	if err != nil {
		return nil, err
	}

	vectorIds := make([]*string, len(res.Vectors))
	for i := 0; i < len(res.Vectors); i++ {
		vectorIds[i] = &res.Vectors[i].Id
	}

	return &ListVectorsResponse{
		VectorIds:           vectorIds,
		Usage:               toUsage(res.Usage),
		NextPaginationToken: toPaginationTokenGrpc(res.Pagination),
		Namespace:           res.Namespace,
	}, nil
}

// QueryByVectorValuesRequest holds the parameters for the [IndexConnection.QueryByVectorValues] method.
type QueryByVectorValuesRequest struct {
	// Vector is the dense query vector, with a size matching the index's dimension. Required on
	// dense indexes; set it together with SparseValues for a hybrid query (dotproduct indexes).
	// Leave it nil on sparse indexes, which accept only SparseValues.
	Vector []float32
	// TopK (Required) is the number of matches to return, 1–10000.
	TopK uint32
	// MetadataFilter (Optional) is the filter to apply to your query.
	MetadataFilter *MetadataFilter
	// IncludeValues (Optional) controls whether the values of the vectors are included in
	// the response.
	IncludeValues bool
	// IncludeMetadata (Optional) controls whether the metadata associated with the vectors
	// is included in the response.
	IncludeMetadata bool
	// SparseValues are the sparse query values. Required on sparse indexes; on dense indexes they
	// can only accompany Vector.
	SparseValues *SparseValues
	// ScanFactor (Optional) is an optimization parameter for IVF dense indexes in dedicated read
	// node indexes. It adjusts how much of the index is scanned to find vector candidates.
	// Range: 0.5 – 4 (default). Only supported for dedicated (DRN) dense indexes.
	ScanFactor *float32
	// MaxCandidates (Optional) is an optimization parameter that controls the maximum number
	// of candidate dense vectors to rerank. Reranking computes exact distances to improve
	// recall but increases query latency. Range: TopK – 100000. Only supported for
	// dedicated (DRN) dense indexes.
	MaxCandidates *uint32
}

// QueryVectorsResponse is returned by [IndexConnection.QueryByVectorValues] and
// [IndexConnection.QueryByVectorId].
type QueryVectorsResponse struct {
	// Matches are the matches, ordered from most to least similar.
	Matches []*ScoredVector `json:"matches,omitempty"`
	// Usage is the usage information for the request.
	Usage *Usage `json:"usage,omitempty"`
	// Namespace is the namespace from which the vectors were queried.
	Namespace string `json:"namespace"`
}

// QueryByVectorValues queries a Pinecone [Index] for vectors that are most similar to a provided query vector.
//
// Returns a pointer to a [QueryVectorsResponse] object or an error if the request fails.
//
// Note: To issue a hybrid query with both dense and sparse values,
// your index's similarity metric must be dot-product.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A [QueryByVectorValuesRequest] object with the parameters for the request.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    queryVector := []float32{1.0, 2.0}
//	    topK := uint32(10)
//
//	    metadataMap := map[string]interface{}{
//			"genre": "classical",
//	    }
//
//	    MetadataFilter, err := pinecone.NewMetadataFilter(metadataMap)
//	    if err != nil {
//			log.Fatalf("Failed to create metadata map. Error: %v", err)
//	    }
//
//	    sparseValues := pinecone.SparseValues{
//			Indices: []uint32{0, 1},
//			Values:  []float32{1.0, 2.0},
//	    }
//
//	    res, err := idxConnection.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
//			Vector:          queryVector,
//			TopK:            topK, // number of vectors to be returned
//			MetadataFilter:          MetadataFilter,
//			SparseValues:    &sparseValues,
//			IncludeValues:   true,
//			IncludeMetadata: true,
//	    })
//	    if err != nil {
//			log.Fatalf("Error encountered when querying by vector: %v", err)
//	    } else {
//			for _, match := range res.Matches {
//				fmt.Printf("Match vector `%s`, with score %f\n", match.Vector.Id, match.Score)
//			}
//	    }
func (idx *IndexConnection) QueryByVectorValues(ctx context.Context, in *QueryByVectorValuesRequest) (*QueryVectorsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*QueryByVectorValuesRequest) cannot be nil")
	}
	if err := validateTopK(in.TopK); err != nil {
		return nil, err
	}
	req := &db_data_grpc.QueryRequest{
		Namespace:       idx.namespace,
		TopK:            in.TopK,
		Filter:          in.MetadataFilter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		Vector:          in.Vector,
		SparseVector:    sparseValToGrpc(in.SparseValues),
		ScanFactor:      in.ScanFactor,
		MaxCandidates:   in.MaxCandidates,
	}

	return idx.query(ctx, req)
}

// QueryByVectorIdRequest holds the parameters for the [IndexConnection.QueryByVectorId] method.
type QueryByVectorIdRequest struct {
	// VectorId (Required) is the unique ID of the vector used to find similar vectors.
	VectorId string
	// TopK (Required) is the number of matches to return, 1–10000.
	TopK uint32
	// MetadataFilter (Optional) is the filter to apply to your query.
	MetadataFilter *MetadataFilter
	// IncludeValues (Optional) controls whether the values of the vectors are included in
	// the response.
	IncludeValues bool
	// IncludeMetadata (Optional) controls whether the metadata associated with the vectors
	// is included in the response.
	IncludeMetadata bool
	// ScanFactor (Optional) is an optimization parameter for IVF dense indexes in dedicated read
	// node indexes. It adjusts how much of the index is scanned to find vector candidates.
	// Range: 0.5 – 4 (default). Only supported for dedicated (DRN) dense indexes.
	ScanFactor *float32
	// MaxCandidates (Optional) is an optimization parameter that controls the maximum number
	// of candidate dense vectors to rerank. Reranking computes exact distances to improve
	// recall but increases query latency. Range: TopK – 100000. Only supported for
	// dedicated (DRN) dense indexes.
	MaxCandidates *uint32
}

// QueryByVectorId uses a vector ID to query a Pinecone [Index] and retrieve vectors that are most similar to the
// provided ID's underlying vector.
//
// Returns a pointer to a [QueryVectorsResponse] object or an error if the request fails.
//
// Note: QueryByVectorId returns up to TopK matches, and the stored vector itself is normally one of
// them. Its own score depends on the index metric. A MetadataFilter can exclude it, and fewer than
// TopK matches come back when fewer vectors qualify.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A QueryByVectorIdRequest object with the parameters for the request.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    vectorId := "abc-1"
//	    topK := uint32(10)
//
//	    res, err := idxConnection.QueryByVectorId(ctx, &pinecone.QueryByVectorIdRequest{
//			VectorId:        vectorId,
//			TopK:            topK, // number of vectors you want returned
//			IncludeValues:   true,
//			IncludeMetadata: true,
//	    })
//
//	    if err != nil {
//			log.Fatalf("Error encountered when querying by vector ID `%s`. Error: %s", vectorId, err)
//	    } else {
//			for _, match := range res.Matches {
//				fmt.Printf("Match vector with ID `%s`, with score %f\n", match.Vector.Id, match.Score)
//			}
//	    }
func (idx *IndexConnection) QueryByVectorId(ctx context.Context, in *QueryByVectorIdRequest) (*QueryVectorsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*QueryByVectorIdRequest) cannot be nil")
	}
	if err := validateTopK(in.TopK); err != nil {
		return nil, err
	}
	req := &db_data_grpc.QueryRequest{
		Id:              in.VectorId,
		Namespace:       idx.namespace,
		TopK:            in.TopK,
		Filter:          in.MetadataFilter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		ScanFactor:      in.ScanFactor,
		MaxCandidates:   in.MaxCandidates,
	}

	return idx.query(ctx, req)
}

// UpsertRecords upserts records into the connection's namespace of an
// [index with integrated embedding]; Pinecone embeds each record's field_map text field
// server-side. Each record must carry exactly one of an "_id" or "id" field, plus the field named in
// the index's field_map; all other fields are stored as metadata. A request may contain at most 96
// records (2 MiB total).
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - records: The [IntegratedRecord] objects to upsert.
//
// Returns an error if the request fails; a non-success response is a [PineconeError].
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "my-namespace"})
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err))
//	    }
//
//	    records := []*pinecone.IntegratedRecord{
//			{
//				"_id":        "rec1",
//				"chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
//				"category":   "product",
//			},
//			{
//				"_id":        "rec2",
//				"chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
//				"category":   "nutrition",
//			},
//			{
//				"_id":        "rec3",
//				"chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
//				"category":   "cultivation",
//			},
//			{
//				"_id":        "rec4",
//				"chunk_text": "In 2001, Apple released the iPod, which transformed the music industry by making portable music widely accessible.",
//				"category":   "product",
//			},
//			{
//				"_id":        "rec5",
//				"chunk_text": "Apple went public in 1980, making history with one of the largest IPOs at that time.",
//				"category":   "milestone",
//			},
//			{
//				"_id":        "rec6",
//				"chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
//				"category":   "nutrition",
//			},
//			{
//				"_id":        "rec7",
//				"chunk_text": "Known for its design-forward products, Apple's branding and market strategy have greatly influenced the technology sector and popularized minimalist design worldwide.",
//				"category":   "influence",
//			},
//			{
//				"_id":        "rec8",
//				"chunk_text": "The high fiber content in apples can also help regulate blood sugar levels, making them a favorable snack for people with diabetes.",
//				"category":   "nutrition",
//			},
//	    }
//
//	    err = idxConnection.UpsertRecords(ctx, records)
//	    if err != nil {
//			log.Fatalf("Failed to upsert records. Error: %v", err)
//	    }
//	    log.Printf("Successfully upserted %d record(s)!\n", len(records))
//
// [index with integrated embedding]: https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models
func (idx *IndexConnection) UpsertRecords(ctx context.Context, records []*IntegratedRecord) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)

	for _, record := range records {
		if record != nil {
			_, hasUnderscoreId := (*record)["_id"]
			_, hasId := (*record)["id"]

			if !hasUnderscoreId && !hasId {
				return fmt.Errorf("record must have an 'id' or '_id' field")
			}
			if hasUnderscoreId && hasId {
				return fmt.Errorf("record must have only one of an 'id' or '_id' field, not both")
			}
		}
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("failed to encode record: %v", err)
		}
	}

	res, err := idx.restClient.UpsertRecordsNamespaceWithBody(ctx, resolveNamespace(idx.namespace), &db_data_rest.UpsertRecordsNamespaceParams{XPineconeApiVersion: gen.PineconeApiVersion}, "application/x-ndjson", &buffer)
	if err != nil {
		return fmt.Errorf("failed to upsert records: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return handleErrorResponseBody(res, "failed to upsert records: ")
	}
	return nil
}

// SearchRecords searches the connection's namespace with query text (Query.Inputs), a query vector
// (Query.Vector), or a record ID (Query.Id) and returns the most similar records with their scores,
// optionally reranked. Text queries require an [index with integrated embedding].
//
// Returns a pointer to a [SearchRecordsResponse], or an error; a non-success response is a
// [PineconeError].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: The [SearchRecordsRequest] describing the query, the fields to return, and optional reranking.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "my-namespace"})
//
//	    records := []*pinecone.IntegratedRecord{
//			{
//				"_id":        "rec1",
//				"chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
//				"category":   "product",
//			},
//			{
//				"_id":        "rec2",
//				"chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
//				"category":   "nutrition",
//			},
//			{
//				"_id":        "rec3",
//				"chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
//				"category":   "cultivation",
//			},
//			{
//				"_id":        "rec4",
//				"chunk_text": "In 2001, Apple released the iPod, which transformed the music industry by making portable music widely accessible.",
//				"category":   "product",
//			},
//			{
//				"_id":        "rec5",
//				"chunk_text": "Apple went public in 1980, making history with one of the largest IPOs at that time.",
//				"category":   "milestone",
//			},
//			{
//				"_id":        "rec6",
//				"chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
//				"category":   "nutrition",
//			},
//			{
//				"_id":        "rec7",
//				"chunk_text": "Known for its design-forward products, Apple's branding and market strategy have greatly influenced the technology sector and popularized minimalist design worldwide.",
//				"category":   "influence",
//			},
//			{
//				"_id":        "rec8",
//				"chunk_text": "The high fiber content in apples can also help regulate blood sugar levels, making them a favorable snack for people with diabetes.",
//				"category":   "nutrition",
//			},
//	    }
//
//	    err = idxConnection.UpsertRecords(ctx, records)
//	    if err != nil {
//			log.Fatalf("Failed to upsert vectors. Error: %v", err)
//	    }
//
//	    res, err := idxConnection.SearchRecords(ctx, &pinecone.SearchRecordsRequest{
//			Query: pinecone.SearchRecordsQuery{
//				TopK: 5,
//				Inputs: &map[string]interface{}{
//					"text": "Disease prevention",
//				},
//			},
//	    })
//	    if err != nil {
//			log.Fatalf("Failed to search records: %v", err)
//	    }
//	    fmt.Printf("Search results: %+v\n", res)
//
// [index with integrated embedding]: https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models
func (idx *IndexConnection) SearchRecords(ctx context.Context, in *SearchRecordsRequest) (*SearchRecordsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*SearchRecordsRequest) cannot be nil")
	}
	if in.Rerank != nil && len(in.Rerank.RankFields) == 0 {
		return nil, fmt.Errorf("rerank.RankFields must contain at least one field")
	}
	var convertedVector *db_data_rest.SearchRecordsVector
	if in.Query.Vector != nil {
		var sparseIndices *[]int64
		if in.Query.Vector.SparseIndices != nil {
			converted := make([]int64, len(*in.Query.Vector.SparseIndices))
			for i, index := range *in.Query.Vector.SparseIndices {
				converted[i] = int64(index)
			}
			sparseIndices = &converted
		}
		convertedVector = &db_data_rest.SearchRecordsVector{
			Values:        in.Query.Vector.Values,
			SparseIndices: sparseIndices,
			SparseValues:  in.Query.Vector.SparseValues,
		}
	}

	var convertedInputs *db_data_rest.EmbedInputs
	if in.Query.Inputs != nil {
		inputMap := db_data_rest.EmbedInputs(*in.Query.Inputs)
		convertedInputs = &inputMap
	}

	var matchTerms *db_data_rest.SearchMatchTerms
	if in.Query.MatchTerms != nil {
		strat := "all"
		if in.Query.MatchTerms.Strategy != nil {
			strat = *in.Query.MatchTerms.Strategy
		}
		var terms []string
		if in.Query.MatchTerms.Terms != nil {
			terms = *in.Query.MatchTerms.Terms
		}
		matchTerms = &db_data_rest.SearchMatchTerms{
			Strategy: strat,
			Terms:    terms,
		}
	}

	req := db_data_rest.SearchRecordsRequest{
		Fields: in.Fields,
		Query: struct {
			Filter     *map[string]interface{}           `json:"filter,omitempty"`
			Id         *string                           `json:"id,omitempty"`
			Inputs     *db_data_rest.EmbedInputs         `json:"inputs,omitempty"`
			MatchTerms *db_data_rest.SearchMatchTerms    `json:"match_terms,omitempty"`
			TopK       int32                             `json:"top_k"`
			Vector     *db_data_rest.SearchRecordsVector `json:"vector,omitempty"`
		}{
			Filter:     in.Query.Filter,
			Id:         in.Query.Id,
			Inputs:     convertedInputs,
			TopK:       in.Query.TopK,
			Vector:     convertedVector,
			MatchTerms: matchTerms,
		},
	}

	if in.Rerank != nil {
		req.Rerank = &struct {
			Model      string                  `json:"model"`
			Parameters *map[string]interface{} `json:"parameters,omitempty"`
			Query      *string                 `json:"query,omitempty"`
			RankFields []string                `json:"rank_fields"`
			TopN       *int32                  `json:"top_n,omitempty"`
		}{
			Model:      in.Rerank.Model,
			Parameters: in.Rerank.Parameters,
			Query:      in.Rerank.Query,
			RankFields: in.Rerank.RankFields,
			TopN:       in.Rerank.TopN,
		}
	}

	res, err := (*idx.restClient).SearchRecordsNamespace(idx.akCtx(ctx), resolveNamespace(idx.namespace), &db_data_rest.SearchRecordsNamespaceParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to search records: ")
	}
	return decodeSearchRecordsResponse(res.Body)
}

// DeleteVectorsById deletes vectors by ID from the connection's namespace. The delete is
// irreversible. IDs that don't exist are ignored rather than reported as an error. The API accepts at
// most 1000 IDs per request. Returns an error if the request fails, otherwise returns nil.
//
// To target another namespace, set Namespace in [NewIndexConnParams] or use [IndexConnection.WithNamespace].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - ids: IDs of the vectors you want to delete.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "custom-namespace"})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    vectorId := "your-vector-id"
//	    err = idxConnection.DeleteVectorsById(ctx, []string{vectorId})
//	    if err != nil {
//			log.Fatalf("Failed to delete vector with ID: %s. Error: %s\n", vectorId, err)
//	    }
func (idx *IndexConnection) DeleteVectorsById(ctx context.Context, ids []string) error {
	req := db_data_grpc.DeleteRequest{
		Ids:       ids,
		Namespace: idx.namespace,
	}

	return idx.delete(ctx, &req)
}

// DeleteVectorsByFilter deletes every vector in the connection's namespace whose metadata matches
// metadataFilter. The delete is irreversible.
//
// metadataFilter must contain at least one condition; a nil or empty filter is rejected.
// Use [IndexConnection.DeleteAllVectorsInNamespace] to delete everything.
// Returns an error if the request fails, otherwise returns nil.
//
// To target another namespace, set Namespace in [NewIndexConnParams] or use [IndexConnection.WithNamespace].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - metadataFilter: The filter selecting the vectors to delete.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err))
//	    }
//
//	    MetadataFilter := map[string]interface{}{
//			"genre": "classical",
//	    }
//
//	    filter, err := pinecone.NewMetadataFilter(MetadataFilter)
//	    if err != nil {
//			log.Fatalf("Failed to create metadata filter. Error: %v", err)
//	    }
//
//	    err = idxConnection.DeleteVectorsByFilter(ctx, filter)
//	    if err != nil {
//			log.Fatalf("Failed to delete vector(s) with filter: %+v. Error: %s\n", filter, err)
//	    }
func (idx *IndexConnection) DeleteVectorsByFilter(ctx context.Context, metadataFilter *MetadataFilter) error {
	if err := validateNonEmptyFilter(metadataFilter); err != nil {
		return fmt.Errorf("delete with an empty metadata filter is not allowed; use DeleteAllVectorsInNamespace to delete everything: %w", err)
	}
	req := db_data_grpc.DeleteRequest{
		Filter:    metadataFilter,
		Namespace: idx.namespace,
	}

	return idx.delete(ctx, &req)
}

// DeleteAllVectorsInNamespace deletes every vector in the connection's namespace. The delete is
// irreversible. The namespace itself remains; use [IndexConnection.DeleteNamespace] to remove it.
// Returns an error if the request fails, otherwise returns nil.
//
// To target another namespace, set Namespace in [NewIndexConnParams] or use [IndexConnection.WithNamespace].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "your-namespace"})
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err))
//	    }
//
//	    err = idxConnection.DeleteAllVectorsInNamespace(ctx)
//	    if err != nil {
//			log.Fatalf("Failed to delete vectors in namespace: \"%s\". Error: %s", "your-namespace", err)
//	    }
func (idx *IndexConnection) DeleteAllVectorsInNamespace(ctx context.Context) error {
	req := db_data_grpc.DeleteRequest{
		Namespace: idx.namespace,
		DeleteAll: true,
	}

	return idx.delete(ctx, &req)
}

// DescribeIndexStatsResponse is returned by the [IndexConnection.DescribeIndexStats] method.
type DescribeIndexStatsResponse struct {
	// Dimension is the dimension of the [Index].
	Dimension *uint32 `json:"dimension"`
	// IndexFullness is the fullness level of the [Index]. Only available on pod-based indexes.
	IndexFullness float32 `json:"index_fullness"`
	// TotalVectorCount is the total number of vectors in the [Index].
	TotalVectorCount uint32 `json:"total_vector_count"`
	// Metric is the similarity metric configured for the [Index], when available.
	Metric *IndexMetric `json:"metric,omitempty"`
	// VectorType is the vector type configured for the [Index], when available.
	VectorType *string `json:"vector_type,omitempty"`
	// MemoryFullness is the fraction of memory used by a dedicated index; nil when the index does
	// not report it.
	MemoryFullness *float32 `json:"memory_fullness,omitempty"`
	// StorageFullness is the fraction of storage used by a dedicated index; nil when the index does
	// not report it.
	StorageFullness *float32 `json:"storage_fullness,omitempty"`
	// Namespaces summarizes the namespace(s) in the [Index], keyed by namespace name.
	Namespaces map[string]*NamespaceSummary `json:"namespaces,omitempty"`
}

// DescribeIndexStats returns statistics about a Pinecone [Index]. Statistics cover every namespace
// in the index, regardless of the connection's namespace.
//
// Returns a pointer to a [DescribeIndexStatsResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err))
//	    }
//
//	    res, err := idxConnection.DescribeIndexStats(ctx)
//	    if err != nil {
//			log.Fatalf("Failed to describe index \"%s\". Error: %s", idx.Name, err)
//	    } else {
//			fmt.Printf("%+v\n", *res)
//	    }
func (idx *IndexConnection) DescribeIndexStats(ctx context.Context) (*DescribeIndexStatsResponse, error) {
	return idx.DescribeIndexStatsFiltered(ctx, nil)
}

// DescribeIndexStatsFiltered returns statistics about a Pinecone [Index], filtered by a given filter.
//
// Returns a pointer to a [DescribeIndexStatsResponse] object or an error if the request fails.
//
// Note: a non-empty filter is supported only on pod-based indexes; serverless indexes reject it (a
// nil filter behaves like [IndexConnection.DescribeIndexStats]). Only the per-namespace counts
// reflect the filter; TotalVectorCount and IndexFullness describe the whole index.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - metadataFilter: The filter to apply to the request.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err))
//	    }
//
//	    MetadataFilter := map[string]interface{}{
//			"genre": "classical",
//	    }
//
//	    filter, err := pinecone.NewMetadataFilter(MetadataFilter)
//	    if err != nil {
//			log.Fatalf("Failed to create filter %+v. Error: %s", MetadataFilter, err)
//	    }
//
//	    res, err := idxConnection.DescribeIndexStatsFiltered(ctx, filter)
//	    if err != nil {
//			log.Fatalf("Failed to describe index \"%s\". Error: %s", idx.Name, err)
//	    } else {
//			for name, summary := range res.Namespaces {
//				fmt.Printf("Namespace: \"%s\", has %d vector(s) that match the given filter\n", name, summary.VectorCount)
//			}
//	    }
func (idx *IndexConnection) DescribeIndexStatsFiltered(ctx context.Context, metadataFilter *MetadataFilter) (*DescribeIndexStatsResponse, error) {
	req := &db_data_grpc.DescribeIndexStatsRequest{
		Filter: metadataFilter,
	}
	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	res, err := (*idx.grpcClient).DescribeIndexStats(ctx, req)
	if err != nil {
		return nil, err
	}

	namespaceSummaries := make(map[string]*NamespaceSummary)
	for key, value := range res.Namespaces {
		namespaceSummaries[key] = &NamespaceSummary{
			VectorCount: value.VectorCount,
		}
	}

	var metric *IndexMetric
	if res.Metric != nil {
		m := IndexMetric(*res.Metric)
		metric = &m
	}

	return &DescribeIndexStatsResponse{
		Dimension:        res.Dimension,
		IndexFullness:    res.IndexFullness,
		TotalVectorCount: res.TotalVectorCount,
		Metric:           metric,
		VectorType:       res.VectorType,
		MemoryFullness:   res.MemoryFullness,
		StorageFullness:  res.StorageFullness,
		Namespaces:       namespaceSummaries,
	}, nil
}

// StartImportResponse holds the response parameters for the [IndexConnection.StartImport] method.
type StartImportResponse struct {
	// Id is the ID of the import process that was started.
	Id string `json:"id,omitempty"`
}

// StartImport starts an asynchronous import of Parquet files from object storage into the index.
// The uri names a directory, not a single file: s3://BUCKET/DIR (AWS-hosted indexes only),
// gs://BUCKET/DIR, or https://ACCOUNT.blob.core.windows.net/CONTAINER/DIR. For buckets that are not
// publicly readable, configure a [storage integration] and pass its ID. Indexes with integrated
// embedding do not support import. StartImport returns once the import is accepted; poll
// [IndexConnection.DescribeImport] for progress.
//
// Returns a pointer to a [StartImportResponse] object with the [Import] ID or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - uri: The directory URI of the Parquet files to import; see the forms above.
//   - integrationId: If your bucket requires authentication to access, you need to pass the id of your storage integration using this property.
//     Pass nil if not required.
//   - errorMode: How the import handles records that fail: "abort" stops the import at the first
//     failure, and "continue" skips failing records. Pass string([ImportErrorModeAbort]) or
//     string([ImportErrorModeContinue]), or nil for the default, "abort".
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    uri := "s3://BUCKET_NAME/PATH/TO/DIR"
//	    errorMode := string(pinecone.ImportErrorModeContinue)
//	    importRes, err := idxConnection.StartImport(ctx, uri, nil, &errorMode)
//	    if err != nil {
//			log.Fatalf("Failed to start import: %v", err)
//	    }
//	    fmt.Printf("Import started with ID: %s", importRes.Id)
//
// [storage integration]: https://docs.pinecone.io/guides/operations/integrations/manage-storage-integrations
func (idx *IndexConnection) StartImport(ctx context.Context, uri string, integrationId *string, errorMode *string) (*StartImportResponse, error) {
	if uri == "" {
		return nil, fmt.Errorf("must specify a uri to start an import")
	}

	req := db_data_rest.StartImportRequest{
		Uri:           uri,
		IntegrationId: integrationId,
	}

	if errorMode != nil {
		req.ErrorMode = &db_data_rest.ImportErrorMode{
			OnError: errorMode,
		}
	}

	res, err := (*idx.restClient).StartBulkImport(idx.akCtx(ctx), &db_data_rest.StartBulkImportParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to start import: ")
	}

	return decodeStartImportResponse(res.Body)
}

// DescribeImport retrieves information about a specific [Import] operation.
//
// Returns a pointer to an [Import] object representing the current state of the import process, or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - id: The id of the import operation. This is returned when you call [IndexConnection.StartImport], or can be retrieved
//     through the [IndexConnection.ListImports] method.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//	    importDesc, err := idxConnection.DescribeImport(ctx, "your-import-id")
//	    if err != nil {
//			log.Fatalf("Failed to describe import: %s - %v", "your-import-id", err)
//	    }
//	    fmt.Printf("Import ID: %s, Status: %s", importDesc.Id, importDesc.Status)
func (idx *IndexConnection) DescribeImport(ctx context.Context, id string) (*Import, error) {
	res, err := (*idx.restClient).DescribeBulkImport(idx.akCtx(ctx), id, &db_data_rest.DescribeBulkImportParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to describe import: ")
	}

	importModel, err := decodeImportModel(res.Body)
	if err != nil {
		return nil, err
	}
	return toImport(importModel), nil
}

// ListImportsRequest holds the parameters for the [IndexConnection.ListImports] method.
type ListImportsRequest struct {
	// Limit (Optional) is the maximum number of imports per page, 1–100. Defaults to 100.
	Limit *int32
	// PaginationToken (Optional) is the NextPaginationToken from a previous [ListImportsResponse].
	// Omit it to fetch the first page.
	PaginationToken *string
}

// ListImportsResponse holds the result of listing [Import] objects.
type ListImportsResponse struct {
	// Imports are the [Import] objects returned.
	Imports []*Import `json:"imports,omitempty"`
	// NextPaginationToken is the token for the next page, or nil when there are no more results.
	NextPaginationToken *string `json:"next_pagination_token,omitempty"`
}

// ListImports returns information about [Import] operations. It returns operations in a
// paginated form, with a pagination token to fetch the next page of results.
//
// Returns a pointer to a [ListImportsResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: An optional [ListImportsRequest] with pagination options. Pass nil to use the defaults.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    limit := int32(10)
//	    firstImportPage, err := idxConnection.ListImports(ctx, &pinecone.ListImportsRequest{Limit: &limit})
//	    if err != nil {
//			log.Fatalf("Failed to list imports: %v", err)
//	    }
//	    fmt.Printf("First page of imports: %+v", firstImportPage.Imports)
//
//	    paginationToken := firstImportPage.NextPaginationToken
//	    nextImportPage, err := idxConnection.ListImports(ctx, &pinecone.ListImportsRequest{
//	        Limit:           &limit,
//	        PaginationToken: paginationToken,
//	    })
//	    if err != nil {
//			log.Fatalf("Failed to list imports: %v", err)
//	    }
//	    fmt.Printf("Second page of imports: %+v", nextImportPage.Imports)
func (idx *IndexConnection) ListImports(ctx context.Context, in *ListImportsRequest) (*ListImportsResponse, error) {
	params := db_data_rest.ListBulkImportsParams{XPineconeApiVersion: gen.PineconeApiVersion}
	if in != nil {
		if in.Limit != nil && (*in.Limit < minListLimit || *in.Limit > maxListLimit) {
			return nil, fmt.Errorf("Limit must be between %d and %d, got %d", minListLimit, maxListLimit, *in.Limit)
		}
		params.Limit = in.Limit
		params.PaginationToken = in.PaginationToken
	}

	res, err := (*idx.restClient).ListBulkImports(idx.akCtx(ctx), &params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, handleErrorResponseBody(res, "failed to list imports: ")
	}

	listImportsResponse, err := decodeListImportsResponse(res.Body)
	if err != nil {
		return nil, err
	}

	return listImportsResponse, nil
}

// CancelImport cancels an [Import] operation by id.
//
// Returns an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - id: The id of the [Import] operation to cancel.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//	    err = idxConnection.CancelImport(ctx, "your-import-id")
//	    if err != nil {
//			log.Fatalf("Failed to cancel import: %s", "your-import-id")
//	    }
func (idx *IndexConnection) CancelImport(ctx context.Context, id string) error {
	res, err := (*idx.restClient).CancelBulkImport(idx.akCtx(ctx), id, &db_data_rest.CancelBulkImportParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return handleErrorResponseBody(res, "failed to cancel import: ")
	}

	return nil
}

// CreateNamespaceParams holds the parameters for creating a new namespace within a serverless index.
type CreateNamespaceParams struct {
	// Name (Required) is the namespace name: at most 512 ASCII bytes with no NUL byte. Creating
	// a name that already exists fails with codes.AlreadyExists.
	Name string
	// Schema (Optional) lists the metadata fields to index (at most 50, each with Filterable true).
	// When nil, the namespace inherits the index's metadata configuration.
	Schema *MetadataSchema
}

// CreateNamespace creates a new namespace within a serverless index.
//
// Returns a pointer to a [NamespaceDescription] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A pointer to a [CreateNamespaceParams] object. See [CreateNamespaceParams] for more information.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//		idxConnection, err := pc.Index(pinecone.NewIndexConnParams{
//		    Host: "your-index-host",
//		})
//		if err != nil {
//		    log.Fatalf("Failed to create IndexConnection: %v", err)
//		}
//
//		namespace, err := idxConnection.CreateNamespace(ctx, &pinecone.CreateNamespaceParams{
//		    Name: "my-namespace",
//		})
//		if err != nil {
//		    log.Fatalf("Failed to create namespace: %v", err)
//		} else {
//		    fmt.Printf("Successfully created namespace: %s with %d records", namespace.Name, namespace.RecordCount)
//		}
func (idx *IndexConnection) CreateNamespace(ctx context.Context, in *CreateNamespaceParams) (*NamespaceDescription, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*CreateNamespaceParams) cannot be nil")
	}
	req := db_data_grpc.CreateNamespaceRequest{
		Name:   in.Name,
		Schema: fromMetadataSchemaToGrpc(in.Schema),
	}
	res, err := (*idx.grpcClient).CreateNamespace(idx.akCtx(ctx), &req)
	if err != nil {
		return nil, err
	}

	return toNamespaceDescription(res), nil
}

// DescribeNamespace describes a namespace within a serverless index. The namespace argument is used
// as given; the connection's own namespace is ignored. Pass "" or "__default__" for the default
// namespace. Use [IndexConnection.ListNamespaces] to describe many namespaces.
//
// Returns a pointer to a [NamespaceDescription] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - namespace: The unique name of the namespace to describe.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//	    namespace, err := idxConnection.DescribeNamespace(ctx, "your-namespace-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe namespace \"%s\". Error:%s", "your-namespace-name", err)
//		}
//	    fmt.Printf("Namespace %s has %d records\n", namespace.Name, namespace.RecordCount)
func (idx *IndexConnection) DescribeNamespace(ctx context.Context, namespace string) (*NamespaceDescription, error) {
	res, err := (*idx.grpcClient).DescribeNamespace(idx.akCtx(ctx), &db_data_grpc.DescribeNamespaceRequest{Namespace: resolveNamespace(namespace)})
	if err != nil {
		return nil, err
	}

	return toNamespaceDescription(res), nil
}

// ListNamespacesResponse is returned by the [IndexConnection.ListNamespaces] method.
type ListNamespacesResponse struct {
	// Namespaces are the [NamespaceDescription] objects returned.
	Namespaces []*NamespaceDescription
	// Pagination is the [Pagination] object for paginating results.
	Pagination *Pagination
	// TotalCount is the total number of namespaces in the index matching the prefix.
	TotalCount int32
}

// ListNamespacesParams holds the parameters for the [IndexConnection.ListNamespaces] method.
type ListNamespacesParams struct {
	// PaginationToken (Optional) is Pagination.Next from the previous response.
	PaginationToken *string
	// Limit (Optional) is the maximum number of namespaces per page, 1–100. Defaults to 100.
	Limit *uint32
	// Prefix (Optional) returns only namespaces whose names start with this value.
	Prefix *string
}

// ListNamespaces lists namespaces within a serverless index.
//
// Returns a pointer to a [ListNamespacesResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: Optional limit, prefix, and pagination options; nil lists from the start with defaults.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//	    limit := uint32(10)
//	    namespaces, err := idxConnection.ListNamespaces(ctx, &pinecone.ListNamespacesParams{Limit: &limit})
//	    if err != nil {
//			log.Fatalf("Failed to list namespaces for index \"%s\". Error:%s", idx.Name, err)
//		}
//	    for _, ns := range namespaces.Namespaces {
//		    fmt.Printf("%s: %d records\n", ns.Name, ns.RecordCount)
//	    }
func (idx *IndexConnection) ListNamespaces(ctx context.Context, in *ListNamespacesParams) (*ListNamespacesResponse, error) {
	var listRequest *db_data_grpc.ListNamespacesRequest
	if in != nil {
		listRequest = &db_data_grpc.ListNamespacesRequest{
			PaginationToken: in.PaginationToken,
			Limit:           in.Limit,
			Prefix:          in.Prefix,
		}
	}
	res, err := (*idx.grpcClient).ListNamespaces(idx.akCtx(ctx), listRequest)
	if err != nil {
		return nil, err
	}
	return toListNamespacesResponse(res), nil
}

// DeleteNamespace deletes the named namespace, and every record in it, from a serverless index. The
// delete is irreversible. The namespace argument is used as given; the connection's own namespace is
// ignored. Pass "" or "__default__" for the default namespace. To empty a namespace but keep it, use
// [IndexConnection.DeleteAllVectorsInNamespace].
//
// Returns an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - namespace: The unique name of the namespace to delete.
//
// Example:
//
//	    ctx := context.Background()
//
//	    clientParams := pinecone.NewClientParams{
//			ApiKey:    "YOUR_API_KEY",
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//			panic(fmt.Errorf("Failed to create Client: %v", err))
//		}
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//			log.Fatalf("Failed to describe index: %v", err)
//		}
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//			log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//	    err = idxConnection.DeleteNamespace(ctx, "your-namespace-name")
//	    if err != nil {
//			log.Fatalf("Failed to delete namespace \"%s\". Error:%s", "your-namespace-name", err)
//		}
func (idx *IndexConnection) DeleteNamespace(ctx context.Context, namespace string) error {
	_, err := (*idx.grpcClient).DeleteNamespace(idx.akCtx(ctx), &db_data_grpc.DeleteNamespaceRequest{
		Namespace: resolveNamespace(namespace),
	})
	if err != nil {
		return err
	}
	return nil
}

func (idx *IndexConnection) query(ctx context.Context, req *db_data_grpc.QueryRequest) (*QueryVectorsResponse, error) {
	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	res, err := (*idx.grpcClient).Query(ctx, req)
	if err != nil {
		return nil, err
	}

	matches := make([]*ScoredVector, len(res.Matches))
	for i, match := range res.Matches {
		matches[i] = toScoredVector(match)
	}

	return &QueryVectorsResponse{
		Matches:   matches,
		Usage:     toUsage(res.Usage),
		Namespace: res.Namespace,
	}, nil
}

func (idx *IndexConnection) delete(ctx context.Context, req *db_data_grpc.DeleteRequest) error {
	// Add Content-Type header for gRPC gateway
	ctx = metadata.AppendToOutgoingContext(idx.akCtx(ctx), "content-type", "application/json")
	_, err := (*idx.grpcClient).Delete(ctx, req)
	return err
}

func decodeSearchRecordsResponse(body io.ReadCloser) (*SearchRecordsResponse, error) {
	var searchRecordsResponse *db_data_rest.SearchRecordsResponse
	if err := json.NewDecoder(body).Decode(&searchRecordsResponse); err != nil {
		return nil, err
	}

	return toSearchRecordsResponse(searchRecordsResponse), nil
}

func decodeListImportsResponse(body io.ReadCloser) (*ListImportsResponse, error) {
	var listImportsResponse *db_data_rest.ListImportsResponse
	if err := json.NewDecoder(body).Decode(&listImportsResponse); err != nil {
		return nil, err
	}

	return toListImportsResponse(listImportsResponse), nil
}

func decodeImportModel(body io.ReadCloser) (*db_data_rest.ImportModel, error) {
	var importModel db_data_rest.ImportModel
	if err := json.NewDecoder(body).Decode(&importModel); err != nil {
		return nil, err
	}

	return &importModel, nil
}

func decodeStartImportResponse(body io.ReadCloser) (*StartImportResponse, error) {
	var importResponse *db_data_rest.StartImportResponse
	if err := json.NewDecoder(body).Decode(&importResponse); err != nil {
		return nil, err
	}

	return toImportResponse(importResponse), nil
}

func (idx *IndexConnection) akCtx(ctx context.Context) context.Context {
	newMetadata := []string{}

	for key, value := range idx.additionalMetadata {
		newMetadata = append(newMetadata, key, value)
	}

	return metadata.AppendToOutgoingContext(ctx, newMetadata...)
}

func toVector(vector *db_data_grpc.Vector) *Vector {
	if vector == nil {
		return nil
	}
	var vectorValues *[]float32
	if vector.Values != nil {
		vectorValues = &vector.Values
	}

	return &Vector{
		Id:           vector.Id,
		Values:       vectorValues,
		Metadata:     vector.Metadata,
		SparseValues: toSparseValues(vector.SparseValues),
	}
}

func toScoredVector(sv *db_data_grpc.ScoredVector) *ScoredVector {
	if sv == nil {
		return nil
	}
	v := toVector(&db_data_grpc.Vector{
		Id:           sv.Id,
		Values:       sv.Values,
		SparseValues: sv.SparseValues,
		Metadata:     sv.Metadata,
	})
	return &ScoredVector{
		Vector: v,
		Score:  sv.Score,
	}
}

func toSparseValues(sv *db_data_grpc.SparseValues) *SparseValues {
	if sv == nil {
		return nil
	}
	return &SparseValues{
		Indices: sv.Indices,
		Values:  sv.Values,
	}
}

func toUsage(u *db_data_grpc.Usage) *Usage {
	if u == nil {
		return nil
	}
	return &Usage{
		ReadUnits: derefOrDefault(u.ReadUnits, 0),
	}
}

func toPaginationTokenGrpc(p *db_data_grpc.Pagination) *string {
	if p == nil {
		return nil
	}
	return &p.Next
}

func toPaginationTokenRest(p *db_data_rest.Pagination) *string {
	if p == nil {
		return nil
	}
	return &p.Next
}

func toImport(importModel *db_data_rest.ImportModel) *Import {
	if importModel == nil {
		return nil
	}

	createdAt := importModel.CreatedAt

	return &Import{
		Id:              importModel.Id,
		Uri:             importModel.Uri,
		Status:          ImportStatus(importModel.Status),
		CreatedAt:       &createdAt,
		FinishedAt:      importModel.FinishedAt,
		Error:           importModel.Error,
		PercentComplete: importModel.PercentComplete,
		RecordsImported: importModel.RecordsImported,
	}
}

func toImportResponse(importResponse *db_data_rest.StartImportResponse) *StartImportResponse {
	if importResponse == nil {
		return nil
	}

	return &StartImportResponse{
		Id: importResponse.Id,
	}
}

func toListImportsResponse(listImportsResponse *db_data_rest.ListImportsResponse) *ListImportsResponse {
	if listImportsResponse == nil {
		return nil
	}

	var imports []*Import
	if listImportsResponse.Data != nil {
		imports = make([]*Import, len(*listImportsResponse.Data))
		for i, importModel := range *listImportsResponse.Data {
			imports[i] = toImport(&importModel)
		}
	}

	return &ListImportsResponse{
		Imports:             imports,
		NextPaginationToken: toPaginationTokenRest(listImportsResponse.Pagination),
	}
}

func toSearchRecordsResponse(searchRecordsResponse *db_data_rest.SearchRecordsResponse) *SearchRecordsResponse {
	if searchRecordsResponse == nil {
		return nil
	}

	hits := make([]Hit, len(searchRecordsResponse.Result.Hits))
	for i, hit := range searchRecordsResponse.Result.Hits {
		hits[i] = Hit{
			Id:     hit.Id,
			Score:  hit.Score,
			Fields: hit.Fields,
		}
	}

	return &SearchRecordsResponse{
		Result: struct {
			Hits []Hit "json:\"hits\""
		}{Hits: hits},
		Usage: SearchUsage{
			ReadUnits:        searchRecordsResponse.Usage.ReadUnits,
			EmbedTotalTokens: searchRecordsResponse.Usage.EmbedTotalTokens,
			RerankUnits:      searchRecordsResponse.Usage.RerankUnits,
		},
	}
}

func toListNamespacesResponse(listNamespacesResponse *db_data_grpc.ListNamespacesResponse) *ListNamespacesResponse {
	if listNamespacesResponse == nil {
		return nil
	}

	namespaces := make([]*NamespaceDescription, len(listNamespacesResponse.Namespaces))
	for i, ns := range listNamespacesResponse.Namespaces {
		namespaces[i] = toNamespaceDescription(ns)
	}
	var pagination *Pagination
	if listNamespacesResponse.Pagination != nil {
		pagination = &Pagination{
			Next: listNamespacesResponse.Pagination.Next,
		}
	}

	return &ListNamespacesResponse{
		Namespaces: namespaces,
		Pagination: pagination,
		TotalCount: listNamespacesResponse.TotalCount,
	}
}

func toNamespaceDescription(ns *db_data_grpc.NamespaceDescription) *NamespaceDescription {
	if ns == nil {
		return nil
	}

	nsDesc := &NamespaceDescription{
		Name:        ns.Name,
		RecordCount: ns.RecordCount,
		Schema:      toMetadataSchemaGrpc(ns.Schema),
		SizeBytes:   ns.SizeBytes,
	}

	if ns.IndexedFields != nil {
		nsDesc.IndexedFields = &IndexedFields{
			Fields: ns.IndexedFields.Fields,
		}
	}

	return nsDesc
}

func vecToGrpc(v *Vector) *db_data_grpc.Vector {
	if v == nil {
		return nil
	}
	var vecValues []float32
	if v.Values != nil {
		vecValues = *v.Values
	}

	return &db_data_grpc.Vector{
		Id:           v.Id,
		Values:       vecValues,
		Metadata:     v.Metadata,
		SparseValues: sparseValToGrpc(v.SparseValues),
	}
}

func sparseValToGrpc(sv *SparseValues) *db_data_grpc.SparseValues {
	if sv == nil {
		return nil
	}
	return &db_data_grpc.SparseValues{
		Indices: sv.Indices,
		Values:  sv.Values,
	}
}

func normalizeHost(host string) (string, bool) {
	// default to secure unless http is specified
	isSecure := true

	parsedHost, err := url.Parse(host)
	if err != nil {
		log.Default().Printf("Failed to parse host %s: %v", host, err)
		return host, isSecure
	}

	if parsedHost.Scheme == "http" {
		isSecure = false
	}

	// the gRPC client is not expecting a scheme so we strip that out
	if parsedHost.Scheme == "https" {
		host = strings.TrimPrefix(host, "https://")
	} else if parsedHost.Scheme == "http" {
		host = strings.TrimPrefix(host, "http://")
	}

	return host, isSecure
}

// Converts MetadataSchema to db_data_grpc.MetadataSchema defined in the generated gRPC API
func fromMetadataSchemaToGrpc(schema *MetadataSchema) *db_data_grpc.MetadataSchema {
	if schema == nil {
		return nil
	}

	fields := make(map[string]*db_data_grpc.MetadataFieldProperties)
	for key, value := range schema.Fields {
		fields[key] = &db_data_grpc.MetadataFieldProperties{
			Filterable: value.Filterable,
		}
	}

	return &db_data_grpc.MetadataSchema{
		Fields: fields,
	}
}

// Converts db_data_grpc.MetadataSchema defined in the generated gRPC API to a MetadataSchema
func toMetadataSchemaGrpc(schema *db_data_grpc.MetadataSchema) *MetadataSchema {
	if schema == nil {
		return nil
	}

	fields := make(map[string]MetadataSchemaField)
	for key, value := range schema.Fields {
		fields[key] = MetadataSchemaField{
			Filterable: value.Filterable,
		}
	}

	return &MetadataSchema{
		Fields: fields,
	}
}

// resolveNamespace maps "" to "__default__", the name REST paths and namespace operations require
// for the default namespace.
func resolveNamespace(ns string) string {
	if ns == "" {
		return "__default__"
	}
	return ns
}

// documentsRequest issues a documents API request against the connection's namespace, checks the
// expected success status, and decodes the response body into out.
func (idx *IndexConnection) documentsRequest(ctx context.Context, operation string, body map[string]interface{}, expectedStatus int, out interface{},
	call func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error)) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode %s request: %w", operation, err)
	}

	res, err := call(ctx, resolveNamespace(idx.namespace), "application/json", bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("failed to %s: %w", operation, err)
	}
	defer res.Body.Close()

	if res.StatusCode != expectedStatus {
		return handleErrorResponseBody(res, fmt.Sprintf("failed to %s: ", operation))
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode %s response: %w", operation, err)
	}
	return nil
}

// UpsertDocuments writes documents into the namespace of a document index (an
// index created with [Client.CreateIndex] using a document schema). Each [Document] must carry an
// "_id" field and at least one field declared in the index schema; any other field is stored as
// filterable metadata. A document with an existing "_id" is overwritten. A request may contain at
// most 1000 documents, each at most 2,000,000 bytes, with a 2 MiB total request body. Null field
// values are dropped. The upsert is applied asynchronously, so documents may not be immediately
// visible to [IndexConnection.SearchDocuments] or [IndexConnection.FetchDocuments].
//
// Returns an [UpsertDocumentsResponse] whose UpsertedCount is the number of documents accepted for
// upsert, or an error.
//
// Example:
//
//	    res, err := idxConnection.UpsertDocuments(ctx, &pinecone.UpsertDocumentsRequest{
//		    Documents: []pinecone.Document{
//			    {"_id": "doc-1", "embedding": []float32{0.1, 0.2}, "genre": "drama"},
//		    },
//	    })
//	    if err != nil {
//		    log.Fatalf("Failed to upsert documents: %v", err)
//	    }
//	    fmt.Println(res.UpsertedCount)
func (idx *IndexConnection) UpsertDocuments(ctx context.Context, in *UpsertDocumentsRequest) (*UpsertDocumentsResponse, error) {
	if in == nil || len(in.Documents) == 0 {
		return nil, fmt.Errorf("in (*UpsertDocumentsRequest) must contain at least one Document")
	}
	for i, document := range in.Documents {
		if _, ok := document["_id"]; !ok {
			return nil, fmt.Errorf("document at index %d must have an \"_id\" field", i)
		}
	}

	var response UpsertDocumentsResponse
	err := idx.documentsRequest(ctx, "upsert documents", map[string]interface{}{"documents": in.Documents}, http.StatusAccepted, &response,
		func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error) {
			return idx.restClient.UpsertDocumentsWithBody(ctx, namespace, &db_data_rest.UpsertDocumentsParams{XPineconeApiVersion: gen.PineconeApiVersion}, contentType, body)
		})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// SearchDocuments searches the namespace for the documents most similar to the
// query described by ScoreBy, ranked by the given scoring methods. See [SearchDocumentsRequest] and
// [DocumentScoringMethod] for the accepted combinations.
//
// Example:
//
//	    query := "vector database"
//	    res, err := idxConnection.SearchDocuments(ctx, &pinecone.SearchDocumentsRequest{
//		    TopK: 10,
//		    ScoreBy: []pinecone.DocumentScoringMethod{
//			    {Type: "text", Fields: []string{"title", "body"}, Query: &query},
//		    },
//		    IncludeFields: []string{"*"},
//	    })
func (idx *IndexConnection) SearchDocuments(ctx context.Context, in *SearchDocumentsRequest) (*SearchDocumentsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*SearchDocumentsRequest) cannot be nil")
	}
	if in.TopK < 1 {
		return nil, fmt.Errorf("TopK must be at least 1")
	}
	if len(in.ScoreBy) == 0 {
		return nil, fmt.Errorf("ScoreBy must contain at least one DocumentScoringMethod")
	}
	if len(in.ScoreBy) > 1 {
		for _, method := range in.ScoreBy {
			if method.Type != "text" && method.Type != "query_string" {
				return nil, fmt.Errorf("several ScoreBy methods may be combined only when every one is \"text\" or \"query_string\"; a %q method must appear on its own", method.Type)
			}
		}
	}
	if in.Filter != nil && len(in.Filter) == 0 {
		return nil, fmt.Errorf("Filter must not be empty when provided")
	}

	body := map[string]interface{}{
		"top_k":    in.TopK,
		"score_by": in.ScoreBy,
	}
	if in.Filter != nil {
		body["filter"] = in.Filter
	}
	if in.IncludeFields != nil {
		body["include_fields"] = in.IncludeFields
	}

	var response SearchDocumentsResponse
	err := idx.documentsRequest(ctx, "search documents", body, http.StatusOK, &response,
		func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error) {
			return idx.restClient.SearchDocumentsWithBody(ctx, namespace, &db_data_rest.SearchDocumentsParams{XPineconeApiVersion: gen.PineconeApiVersion}, contentType, body)
		})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// FetchDocuments retrieves documents from the namespace by ID or by metadata
// filter. Exactly one of Ids or Filter must be provided; see [FetchDocumentsRequest].
//
// Example:
//
//	    res, err := idxConnection.FetchDocuments(ctx, &pinecone.FetchDocumentsRequest{
//		    Ids: []string{"doc-1", "doc-2"},
//	    })
func (idx *IndexConnection) FetchDocuments(ctx context.Context, in *FetchDocumentsRequest) (*FetchDocumentsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*FetchDocumentsRequest) cannot be nil")
	}
	if (len(in.Ids) == 0) == (in.Filter == nil) {
		return nil, fmt.Errorf("exactly one of Ids or Filter must be provided in FetchDocumentsRequest")
	}
	if in.Filter != nil && len(in.Filter) == 0 {
		return nil, fmt.Errorf("Filter must not be empty; a fetch matching every document is rejected")
	}
	if in.PaginationToken != nil && in.Filter == nil {
		return nil, fmt.Errorf("PaginationToken is only valid together with Filter")
	}

	body := map[string]interface{}{}
	if len(in.Ids) > 0 {
		body["ids"] = in.Ids
	}
	if in.Filter != nil {
		body["filter"] = in.Filter
	}
	if in.IncludeFields != nil {
		body["include_fields"] = in.IncludeFields
	}
	if in.Limit != nil {
		body["limit"] = *in.Limit
	}
	if in.PaginationToken != nil {
		body["pagination_token"] = *in.PaginationToken
	}

	var response FetchDocumentsResponse
	err := idx.documentsRequest(ctx, "fetch documents", body, http.StatusOK, &response,
		func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error) {
			return idx.restClient.FetchDocumentsWithBody(ctx, namespace, &db_data_rest.FetchDocumentsParams{XPineconeApiVersion: gen.PineconeApiVersion}, contentType, body)
		})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteDocuments deletes documents from the namespace by ID, by metadata filter,
// or all at once. Exactly one of Ids, Filter, or DeleteAll must be provided; see
// [DeleteDocumentsRequest]. The delete is applied asynchronously.
//
// Example:
//
//	    _, err := idxConnection.DeleteDocuments(ctx, &pinecone.DeleteDocumentsRequest{
//		    Ids: []string{"doc-1"},
//	    })
func (idx *IndexConnection) DeleteDocuments(ctx context.Context, in *DeleteDocumentsRequest) (*DeleteDocumentsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*DeleteDocumentsRequest) cannot be nil")
	}
	selectors := 0
	if len(in.Ids) > 0 {
		selectors++
	}
	if in.Filter != nil {
		selectors++
	}
	if in.DeleteAll {
		selectors++
	}
	if selectors != 1 {
		return nil, fmt.Errorf("exactly one of Ids, Filter, or DeleteAll must be provided in DeleteDocumentsRequest")
	}
	if in.Filter != nil && len(in.Filter) == 0 {
		return nil, fmt.Errorf("Filter must not be empty; to delete every document in the namespace, set DeleteAll")
	}

	body := map[string]interface{}{}
	if len(in.Ids) > 0 {
		body["ids"] = in.Ids
	}
	if in.Filter != nil {
		body["filter"] = in.Filter
	}
	if in.DeleteAll {
		body["delete_all"] = true
	}

	var response DeleteDocumentsResponse
	err := idx.documentsRequest(ctx, "delete documents", body, http.StatusAccepted, &response,
		func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error) {
			return idx.restClient.DeleteDocumentsWithBody(ctx, namespace, &db_data_rest.DeleteDocumentsParams{XPineconeApiVersion: gen.PineconeApiVersion}, contentType, body)
		})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateDocuments applies partial updates to documents in the namespace, either as
// per-document updates (Documents) or as a filtered patch (Filter with SetFields and/or
// RemoveFields); see [UpdateDocumentsRequest]. The patch is applied asynchronously.
//
// Example:
//
//	    _, err := idxConnection.UpdateDocuments(ctx, &pinecone.UpdateDocumentsRequest{
//		    Filter:    map[string]interface{}{"genre": "drama"},
//		    SetFields: map[string]interface{}{"reviewed": true},
//	    })
func (idx *IndexConnection) UpdateDocuments(ctx context.Context, in *UpdateDocumentsRequest) (*UpdateDocumentsResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("in (*UpdateDocumentsRequest) cannot be nil")
	}
	hasDocuments := len(in.Documents) > 0
	hasPatch := in.Filter != nil || len(in.SetFields) > 0 || len(in.RemoveFields) > 0
	if hasDocuments == hasPatch {
		return nil, fmt.Errorf("either Documents or a filtered patch (Filter with SetFields and/or RemoveFields) must be provided in UpdateDocumentsRequest, not both")
	}
	if hasPatch {
		if in.Filter == nil {
			return nil, fmt.Errorf("SetFields and RemoveFields are only valid together with Filter")
		}
		if len(in.Filter) == 0 {
			return nil, fmt.Errorf("Filter must not be empty; a patch matching every document is rejected")
		}
		if len(in.SetFields) == 0 && len(in.RemoveFields) == 0 {
			return nil, fmt.Errorf("a filtered patch must set SetFields and/or RemoveFields")
		}
	}
	if hasDocuments {
		for i, document := range in.Documents {
			if _, ok := document["_id"]; !ok {
				return nil, fmt.Errorf("document at index %d must have an \"_id\" field", i)
			}
		}
	}

	body := map[string]interface{}{}
	if hasDocuments {
		body["documents"] = in.Documents
	} else {
		body["filter"] = in.Filter
		if len(in.SetFields) > 0 {
			body["set_fields"] = in.SetFields
		}
		if len(in.RemoveFields) > 0 {
			body["remove_fields"] = in.RemoveFields
		}
	}

	var response UpdateDocumentsResponse
	err := idx.documentsRequest(ctx, "update documents", body, http.StatusAccepted, &response,
		func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error) {
			return idx.restClient.UpdateDocumentsWithBody(ctx, namespace, &db_data_rest.UpdateDocumentsParams{XPineconeApiVersion: gen.PineconeApiVersion}, contentType, body)
		})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// ListDocuments lists the IDs of documents in the namespace, in sorted order,
// optionally restricted to a prefix. See [ListDocumentsRequest]; pass nil to list with defaults.
//
// Example:
//
//	res, err := idxConnection.ListDocuments(ctx, &pinecone.ListDocumentsRequest{})
func (idx *IndexConnection) ListDocuments(ctx context.Context, in *ListDocumentsRequest) (*ListDocumentsResponse, error) {
	body := map[string]interface{}{}
	if in != nil {
		if in.Prefix != nil {
			body["prefix"] = *in.Prefix
		}
		if in.Limit != nil {
			body["limit"] = *in.Limit
		}
		if in.PaginationToken != nil {
			body["pagination_token"] = *in.PaginationToken
		}
	}

	var response ListDocumentsResponse
	err := idx.documentsRequest(ctx, "list documents", body, http.StatusOK, &response,
		func(ctx context.Context, namespace string, contentType string, body io.Reader) (*http.Response, error) {
			return idx.restClient.ListDocumentsWithBody(ctx, namespace, &db_data_rest.ListDocumentsParams{XPineconeApiVersion: gen.PineconeApiVersion}, contentType, body)
		})
	if err != nil {
		return nil, err
	}
	return &response, nil
}
