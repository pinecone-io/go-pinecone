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

	"github.com/pinecone-io/go-pinecone/v4/internal/gen"
	db_data_grpc "github.com/pinecone-io/go-pinecone/v4/internal/gen/db_data/grpc"
	db_data_rest "github.com/pinecone-io/go-pinecone/v4/internal/gen/db_data/rest"
	"github.com/pinecone-io/go-pinecone/v4/internal/useragent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// [IndexConnection] holds the parameters for a Pinecone [IndexConnection] object. You can
// instantiate a [IndexConnection] by calling the [Client.Index] method with a [NewIndexConnParams] object.
// You can use [IndexConnection.WithNamespace] to create a new [IndexConnection] that targets a different namespace
// while sharing the underlying gRPC connection.
//
// Fields:
//   - namespace: The namespace where index operations will be performed.
//   - additionalMetadata: Additional metadata to be sent with each RPC request.
//   - dataClient: The gRPC client for the index.
//   - grpcConn: The gRPC connection.
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
		log.Fatalf("failed to create grpc client: %v", err)
		return nil, err
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

// [IndexConnection.Close] closes the grpc.ClientConn to a Pinecone [Index].
//
// Returns an error if the connection cannot be closed, otherwise returns nil.
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
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection: %v", err)
//	    }
//
//	    err = idxConnection.Close()
//
//	    if err != nil {
//		       log.Fatalf("Failed to close index connection. Error: %v", err)
//	    }
func (idx *IndexConnection) Close() error {
	err := idx.grpcConn.Close()
	return err
}

// [IndexConnection.Namespace] allows returning the namespace the instance of [IndexConnection] is targeting.
func (idx *IndexConnection) Namespace() string {
	return idx.namespace
}

// [IndexConnection.WithNamespace] creates a new copy of [IndexConnection] that targets a new namespace within that index while
// sharing the underlying gRPC connection. This is useful for performing operations across namespaces in an index without re-creating the index connection.
//
// Example:
//
//	    ctx := context.Background()
//		clientParams := pinecone.NewClientParams{
//				ApiKey:    "YOUR_API_KEY",
//				SourceTag: "your_source_identifier", // optional
//		}
//		pc, err := pinecone.NewClient(clientParams)
//		if err != nil {
//				log.Fatalf("Failed to create Client: %v", err)
//		}
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//				log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		}
//
//		idxConnNs1, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "namespace1"})
//		if err != nil {
//				log.Fatalf("Failed to create IndexConnection: %v", err)
//		}
//
//		metadataMap := map[string]interface{}{
//				"genre": "classical",
//		}
//		metadata, err := structpb.NewStruct(metadataMap)
//		if err != nil {
//				log.Fatalf("Failed to create metadata map. Error: %v", err)
//		}
//		values := []float32{1.0, 2.0}
//		vectors := []*pinecone.Vector{
//				{
//					Id:       "abc-1",
//					Values:   &values,
//					Metadata: metadata,
//				},
//		}
//
//		_, err = idxConnNs1.UpsertVectors(ctx, vectors)
//		if err != nil {
//				log.Fatalf("Failed to upsert vectors in %s. Error: %v", idxConnNs1.Namespace, err)
//		}
//		idxConnNs2 := idxConnNs1.WithNamespace("namespace2")
//		_, err = idxConnNs2.UpsertVectors(ctx, vectors)
//		if err != nil {
//				log.Fatalf("Failed to upsert vectors in %s. Error: %v", idxConnNs2.Namespace, err)
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

// [IndexConnection.UpsertVectors] upserts vectors into a Pinecone [Index].
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
//			   ApiKey:    "YOUR_API_KEY",
//			   SourceTag: "your_source_identifier", // optional
//		}
//
//		pc, err := pinecone.NewClient(clientParams)
//		if err != nil {
//			   log.Fatalf("Failed to create Client: %v", err)
//		}
//
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			   log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		}
//
//		idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//		if err != nil {
//			   log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//		metadataMap := map[string]interface{}{
//			   "genre": "classical",
//		}
//		metadata, err := structpb.NewStruct(metadataMap)
//		if err != nil {
//			   log.Fatalf("Failed to create metadata map. Error: %v", err)
//		}
//	   	denseValues := []float32{1.0, 2.0}
//
//		sparseValues := pinecone.SparseValues{
//			   Indices: []uint32{0, 1},
//			   Values:  []float32{1.0, 2.0},
//		}
//
//		vectors := []*pinecone.Vector{
//				{
//			    	Id:           "abc-1",
//				    Values:       &denseValues,
//				    Metadata:     metadata,
//				    SparseValues: &sparseValues,
//			    },
//		}
//
//		count, err := idxConnection.UpsertVectors(ctx, vectors)
//		if err != nil {
//	    		log.Fatalf("Failed to upsert vectors. Error: %v", err)
//		} else {
//				log.Fatalf("Successfully upserted %d vector(s)!\n", count)
//		}
func (idx *IndexConnection) UpsertVectors(ctx context.Context, in []*Vector) (uint32, error) {
	vectors := make([]*db_data_grpc.Vector, len(in))
	for i, v := range in {
		vectors[i] = vecToGrpc(v)
	}

	req := &db_data_grpc.UpsertRequest{
		Vectors:   vectors,
		Namespace: idx.namespace,
	}

	res, err := (*idx.grpcClient).Upsert(idx.akCtx(ctx), req)
	if err != nil {
		return 0, err
	}
	return res.UpsertedCount, nil
}

// [UpdateVectorRequest] holds the parameters for the [IndexConnection.UpdateVector] method.
//
// Fields:
//   - Id: (Required) The unique ID of the vector to update.
//   - Values: The values with which you want to update the vector.
//   - SparseValues: The sparse values with which you want to update the vector.
//   - Metadata: The metadata with which you want to update the vector.
type UpdateVectorRequest struct {
	Id           string
	Values       []float32
	SparseValues *SparseValues
	Metadata     *Metadata
}

// [IndexConnection.UpdateVector] updates a vector in a Pinecone [Index] by ID.
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    id := "abc-1"
//
//	    err = idxConnection.UpdateVector(ctx, &pinecone.UpdateVectorRequest{
//		       Id:     id,
//		       Values: []float32{7.0, 8.0},
//	    })
//
//	    if err != nil {
//		       log.Fatalf("Failed to update vector with ID %s. Error: %s", id, err)
//	    }
func (idx *IndexConnection) UpdateVector(ctx context.Context, in *UpdateVectorRequest) error {
	if in.Id == "" {
		return fmt.Errorf("a vector ID plus at least one of Values, SparseValues, or Metadata must be provided to update a vector")
	}

	req := &db_data_grpc.UpdateRequest{
		Id:           in.Id,
		Values:       in.Values,
		SparseValues: sparseValToGrpc(in.SparseValues),
		SetMetadata:  in.Metadata,
		Namespace:    idx.namespace,
	}

	_, err := (*idx.grpcClient).Update(idx.akCtx(ctx), req)
	return err
}

// [FetchVectorsResponse] is returned by the [IndexConnection.FetchVectors] method.
//
// Fields:
//   - Vectors: The vectors fetched.
//   - Usage: The usage information for the request.
//   - Namespace: The namespace from which the vectors were fetched.
type FetchVectorsResponse struct {
	Vectors   map[string]*Vector `json:"vectors,omitempty"`
	Usage     *Usage             `json:"usage,omitempty"`
	Namespace string             `json:"namespace"`
}

// [IndexConnection.FetchVectors] fetches vectors by ID from a Pinecone [Index].
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    res, err := idxConnection.FetchVectors(ctx, []string{"abc-1"})
//
//	    if err != nil {
//		       log.Fatalf("Failed to fetch vectors, error: %+v", err)
//	    }
//
//	    if len(res.Vectors) != 0 {
//		       fmt.Println(res)
//	    } else {
//		       fmt.Println("No vectors found")
//	    }
func (idx *IndexConnection) FetchVectors(ctx context.Context, ids []string) (*FetchVectorsResponse, error) {
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
		Namespace: idx.namespace,
	}, nil
}

// [ListVectorsRequest] holds the parameters passed into the [IndexConnection.ListVectors] method.
//
// Fields:
//   - Prefix: (Optional) The prefix by which to filter. If unspecified,
//     an empty string will be used which will list all vector ids in the namespace
//   - Limit: (Optional) The maximum number of vectors to return. If unspecified, the server will use a default value.
//   - PaginationToken: (Optional) The token for paginating through results.
type ListVectorsRequest struct {
	Prefix          *string
	Limit           *uint32
	PaginationToken *string
}

// [ListVectorsResponse] is returned by the [IndexConnection.ListVectors] method.
//
// Fields:
//   - VectorIds: The unique IDs of the returned vectors.
//   - Usage: The usage information for the request.
//   - NextPaginationToken: The token for paginating through results.
//   - Namespace: The namespace vector ids are listed from.
type ListVectorsResponse struct {
	VectorIds           []*string `json:"vector_ids,omitempty"`
	Usage               *Usage    `json:"usage,omitempty"`
	NextPaginationToken *string   `json:"next_pagination_token,omitempty"`
	Namespace           string    `json:"namespace"`
}

// [IndexConnection.ListVectors] lists vectors in a Pinecone index. You can filter vectors by prefix,
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    prefix := "abc"
//	    limit := uint32(10)
//
//	    res, err := idxConnection.ListVectors(ctx, &pinecone.ListVectorsRequest{
//		       Prefix: &prefix,
//		       Limit:  &limit,
//	    })
//
//	    if err != nil {
//		       log.Fatalf("Failed to list vectors in index: %s. Error: %s\n", idx.Name, err)
//	    }
//
//	    if len(res.VectorIds) == 0 {
//		       fmt.Println("No vectors found")
//	    } else {
//		       fmt.Printf("Found %d vector(s)\n", len(res.VectorIds))
//	    }
func (idx *IndexConnection) ListVectors(ctx context.Context, in *ListVectorsRequest) (*ListVectorsResponse, error) {
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
		Namespace:           idx.namespace,
	}, nil
}

// [QueryByVectorValuesRequest] holds the parameters for the [IndexConnection.QueryByVectorValues] method.
//
// Fields:
//   - Vector: (Required) The query vector used to find similar vectors.
//   - TopK: (Required) The number of vectors to return.
//   - MetadataFilter: (Optional) The filter to apply to your query.
//   - IncludeValues: (Optional) Whether to include the values of the vectors in the response.
//   - IncludeMetadata: (Optional) Whether to include the metadata associated with the vectors in the response.
//   - SparseValues: (Optional) The sparse values of the query vector, if applicable.
type QueryByVectorValuesRequest struct {
	Vector          []float32
	TopK            uint32
	MetadataFilter  *MetadataFilter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

// [QueryVectorsResponse] is returned by the [IndexConnection.QueryByVectorValues] method.
//
// Fields:
//   - Matches: The vectors that are most similar to the query vector.
//   - Usage: The usage information for the request.
//   - Namespace: The namespace from which the vectors were queried.
type QueryVectorsResponse struct {
	Matches   []*ScoredVector `json:"matches,omitempty"`
	Usage     *Usage          `json:"usage,omitempty"`
	Namespace string          `json:"namespace"`
}

// [IndexConnection.QueryByVectorValues] queries a Pinecone [Index] for vectors that are most similar to a provided query vector.
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    queryVector := []float32{1.0, 2.0}
//	    topK := uint32(10)
//
//	    metadataMap := map[string]interface{}{
//		       "genre": "classical",
//	    }
//
//	    MetadataFilter, err := structpb.NewStruct(metadataMap)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create metadata map. Error: %v", err)
//	    }
//
//	    sparseValues := pinecone.SparseValues{
//		       Indices: []uint32{0, 1},
//		       Values:  []float32{1.0, 2.0},
//	    }
//
//	    res, err := idxConnection.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
//		       Vector:          queryVector,
//		       TopK:            topK, // number of vectors to be returned
//		       MetadataFilter:          MetadataFilter,
//		       SparseValues:    &sparseValues,
//		       IncludeValues:   true,
//		       IncludeMetadata: true,
//	    })
//
//	    if err != nil {
//		       log.Fatalf("Error encountered when querying by vector: %v", err)
//	    } else {
//		       for _, match := range res.Matches {
//			       fmt.Printf("Match vector `%s`, with score %f\n", match.Vector.Id, match.Score)
//		       }
//	    }
func (idx *IndexConnection) QueryByVectorValues(ctx context.Context, in *QueryByVectorValuesRequest) (*QueryVectorsResponse, error) {
	req := &db_data_grpc.QueryRequest{
		Namespace:       idx.namespace,
		TopK:            in.TopK,
		Filter:          in.MetadataFilter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		Vector:          in.Vector,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
}

// [QueryByVectorIdRequest] holds the parameters for the [IndexConnection.QueryByVectorId] method.
//
// Fields:
//   - VectorId: (Required) The unique ID of the vector used to find similar vectors.
//   - TopK: (Required) The number of vectors to return.
//   - MetadataFilter: (Optional) The filter to apply to your query.
//   - IncludeValues: (Optional) Whether to include the values of the vectors in the response.
//   - IncludeMetadata: (Optional) Whether to include the metadata associated with the vectors in the response.
//   - SparseValues: (Optional) The sparse values of the query vector, if applicable.
type QueryByVectorIdRequest struct {
	VectorId        string
	TopK            uint32
	MetadataFilter  *MetadataFilter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

// [IndexConnection.QueryByVectorId] uses a vector ID to query a Pinecone [Index] and retrieve vectors that are most similar to the
// provided ID's underlying vector.
//
// Returns a pointer to a [QueryVectorsResponse] object or an error if the request fails.
//
// Note: QueryByVectorId executes a nearest neighbors search, meaning that unless TopK=1 in the [QueryByVectorIdRequest]
// object, it will return 2+ vectors. The vector with a score of 1.0 is the vector with the same ID as the query vector.
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    vectorId := "abc-1"
//	    topK := uint32(10)
//
//	    res, err := idxConnection.QueryByVectorId(ctx, &pinecone.QueryByVectorIdRequest{
//		       VectorId:        vectorId,
//		       TopK:            topK, // number of vectors you want returned
//		       IncludeValues:   true,
//		       IncludeMetadata: true,
//	    })
//
//	    if err != nil {
//		       log.Fatalf("Error encountered when querying by vector ID `%s`. Error: %s", vectorId, err)
//	    } else {
//		       for _, match := range res.Matches {
//			       fmt.Printf("Match vector with ID `%s`, with score %f\n", match.Vector.Id, match.Score)
//		       }
//	    }
func (idx *IndexConnection) QueryByVectorId(ctx context.Context, in *QueryByVectorIdRequest) (*QueryVectorsResponse, error) {
	req := &db_data_grpc.QueryRequest{
		Id:              in.VectorId,
		Namespace:       idx.namespace,
		TopK:            in.TopK,
		Filter:          in.MetadataFilter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
}

// [IndexConnection.UpsertRecords] upserts records into an integrated [Pinecone Index].
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: The [IntegratedRecord] objects to upsert.
//
// Returns an error if the request fails.
//
// Example:
//
//	     ctx := context.Background()
//
//	     clientParams := pinecone.NewClientParams{
//		     ApiKey:    "YOUR_API_KEY",
//		     SourceTag: "your_source_identifier", // optional
//	     }
//
//	     pc, err := pinecone.NewClient(clientParams)
//
//	     if err != nil {
//		     log.Fatalf("Failed to create Client: %v", err)
//	     }
//
//	     idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	     if err != nil {
//		     log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	     }
//
//	     idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "my-namespace"})
//
//	     records := []*IntegratedRecord{
//		     {
//			     "_id":        "rec1",
//			     "chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
//			     "category":   "product",
//		     },
//		     {
//			     "_id":        "rec2",
//			     "chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
//			     "category":   "nutrition",
//		     },
//		     {
//			     "_id":        "rec3",
//			     "chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
//			     "category":   "cultivation",
//		     },
//		     {
//			     "_id":        "rec4",
//			     "chunk_text": "In 2001, Apple released the iPod, which transformed the music industry by making portable music widely accessible.",
//			     "category":   "product",
//		     },
//		     {
//			     "_id":        "rec5",
//			     "chunk_text": "Apple went public in 1980, making history with one of the largest IPOs at that time.",
//			     "category":   "milestone",
//		     },
//		     {
//			     "_id":        "rec6",
//			     "chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
//			     "category":   "nutrition",
//		     },
//		     {
//			     "_id":        "rec7",
//			     "chunk_text": "Known for its design-forward products, Apple's branding and market strategy have greatly influenced the technology sector and popularized minimalist design worldwide.",
//			     "category":   "influence",
//		     },
//		     {
//			     "_id":        "rec8",
//			     "chunk_text": "The high fiber content in apples can also help regulate blood sugar levels, making them a favorable snack for people with diabetes.",
//			     "category":   "nutrition",
//		     },
//	     }
//
//	     err = idxConnection.UpsertRecords(ctx, &records)
//	     if err != nil {
//		     log.Fatalf("Failed to upsert vectors. Error: %v", err)
//	     } else {
//		     log.Fatalf("Successfully upserted %d vector(s)!\n", count)
//	     }
//
// [Pinecone Index]: https://docs.pinecone.io/reference/api/2025-01/control-plane/create_for_model
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
		}
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("failed to encode record: %v", err)
		}
	}

	_, err := idx.restClient.UpsertRecordsNamespaceWithBody(ctx, idx.namespace, &db_data_rest.UpsertRecordsNamespaceParams{XPineconeApiVersion: gen.PineconeApiVersion}, "application/x-ndjson", &buffer)
	if err != nil {
		return fmt.Errorf("failed to upsert records: %v", err)
	}
	return nil
}

// [IndexConnection.SearchRecords] converts a query to a vector embedding and then searches a namespace in an integrated index.
// You can optionally provide a reranking operation as part of the search.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: The [IntegratedRecord] objects to upsert.
//
// Returns an error if the request fails.
//
// Example:
//
//		   ctx := context.Background()
//
//		   clientParams := pinecone.NewClientParams{
//			   ApiKey:    "YOUR_API_KEY",
//			   SourceTag: "your_source_identifier", // optional
//		   }
//
//		   pc, err := pinecone.NewClient(clientParams)
//
//		   if err != nil {
//			   log.Fatalf("Failed to create Client: %v", err)
//		   }
//
//		   idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//		   if err != nil {
//			   log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		   }
//
//		   idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "my-namespace"})
//
//		   records := []*IntegratedRecord{
//			     {
//				     "_id":        "rec1",
//				     "chunk_text": "Apple's first product, the Apple I, was released in 1976 and was hand-built by co-founder Steve Wozniak.",
//				     "category":   "product",
//			     },
//			     {
//				     "_id":        "rec2",
//				     "chunk_text": "Apples are a great source of dietary fiber, which supports digestion and helps maintain a healthy gut.",
//				     "category":   "nutrition",
//			     },
//			     {
//				     "_id":        "rec3",
//				     "chunk_text": "Apples originated in Central Asia and have been cultivated for thousands of years, with over 7,500 varieties available today.",
//				     "category":   "cultivation",
//			     },
//			     {
//				     "_id":        "rec4",
//				     "chunk_text": "In 2001, Apple released the iPod, which transformed the music industry by making portable music widely accessible.",
//				     "category":   "product",
//			     },
//			     {
//				     "_id":        "rec5",
//				     "chunk_text": "Apple went public in 1980, making history with one of the largest IPOs at that time.",
//				     "category":   "milestone",
//			     },
//			     {
//				     "_id":        "rec6",
//				     "chunk_text": "Rich in vitamin C and other antioxidants, apples contribute to immune health and may reduce the risk of chronic diseases.",
//				     "category":   "nutrition",
//			     },
//			     {
//				     "_id":        "rec7",
//				     "chunk_text": "Known for its design-forward products, Apple's branding and market strategy have greatly influenced the technology sector and popularized minimalist design worldwide.",
//				     "category":   "influence",
//			     },
//			     {
//				     "_id":        "rec8",
//				     "chunk_text": "The high fiber content in apples can also help regulate blood sugar levels, making them a favorable snack for people with diabetes.",
//				     "category":   "nutrition",
//			     },
//		     }
//
//	      err = idxConnection.UpsertRecords(ctx, records)
//	      if err != nil {
//		         log.Fatalf("Failed to upsert vectors. Error: %v", err)
//	      }
//
//	      res, err := idxConnection.SearchRecords(ctx, &SearchRecordsRequest{
//		         Query: SearchRecordsQuery{
//			         TopK: 5,
//			         Inputs: &map[string]interface{}{
//			 	         "text": "Disease prevention",
//			         },
//		         },
//	      })
//	      if err != nil {
//		         log.Fatalf("Failed to search records: %v", err)
//	      }
//	      fmt.Printf("Search results: %+v\n", res)
//
// [Pinecone Index]: https://docs.pinecone.io/reference/api/2025-01/control-plane/create_for_model
func (idx *IndexConnection) SearchRecords(ctx context.Context, in *SearchRecordsRequest) (*SearchRecordsResponse, error) {
	var convertedVector *db_data_rest.SearchRecordsVector
	if in.Query.Vector != nil {
		convertedVector = &db_data_rest.SearchRecordsVector{
			Values:        in.Query.Vector.Values,
			SparseIndices: in.Query.Vector.SparseIndices,
			SparseValues:  in.Query.Vector.SparseValues,
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
			Filter: in.Query.Filter,
			Id:     in.Query.Id,
			TopK:   in.Query.TopK,
			Vector: convertedVector,
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

	res, err := (*idx.restClient).SearchRecordsNamespace(idx.akCtx(ctx), idx.namespace, &db_data_rest.SearchRecordsNamespaceParams{XPineconeApiVersion: gen.PineconeApiVersion}, req)
	if err != nil {
		return nil, err
	}
	return decodeSearchRecordsResponse(res.Body)
}

// [IndexConnection.DeleteVectorsById] deletes vectors by ID from a Pinecone [Index].
//
// Returns an error if the request fails, otherwise returns nil. This method will also return
// nil if the passed vector ID does not exist in the index or namespace.
//
// Note: You must create an [IndexConnection] with a Namespace in [NewIndexConnParams] in order to delete vectors
// in a namespace other than the default: "".
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "custom-namespace"})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    vectorId := "your-vector-id"
//	    err = idxConnection.DeleteVectorsById(ctx, []string{vectorId})
//
//	    if err != nil {
//		       log.Fatalf("Failed to delete vector with ID: %s. Error: %s\n", vectorId, err)
//	    }
func (idx *IndexConnection) DeleteVectorsById(ctx context.Context, ids []string) error {
	req := db_data_grpc.DeleteRequest{
		Ids:       ids,
		Namespace: idx.namespace,
	}

	return idx.delete(ctx, &req)
}

// [IndexConnection.DeleteVectorsByFilter] deletes vectors from a Pinecone [Index], given a filter.
//
// Returns an error if the request fails, otherwise returns nil.
//
// Note: [DeleteVectorsByFilter] is only available on pods-based indexes.
// Additionally, you must create an [IndexConnection] using the [Client.Index] method with a Namespace in [NewIndexConnParams]
// in order to delete vectors in a namespace other than the default: "".
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - MetadataFilter: The filter to apply to the deletion.
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
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		        log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    MetadataFilter := map[string]interface{}{
//		       "genre": "classical",
//	    }
//
//	    filter, err := structpb.NewStruct(MetadataFilter)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create metadata filter. Error: %v", err)
//	    }
//
//	    err = idxConnection.DeleteVectorsByFilter(ctx, filter)
//
//	    if err != nil {
//		       log.Fatalf("Failed to delete vector(s) with filter: %+v. Error: %s\n", filter, err)
//	    }
func (idx *IndexConnection) DeleteVectorsByFilter(ctx context.Context, metadataFilter *MetadataFilter) error {
	req := db_data_grpc.DeleteRequest{
		Filter:    metadataFilter,
		Namespace: idx.namespace,
	}

	return idx.delete(ctx, &req)
}

// [IndexConnection.DeleteAllVectorsInNamespace] deletes all vectors in a specific namespace.
//
// Returns an error if the request fails, otherwise returns nil.
//
// Note: You must instantiate an [IndexConnection] using the [Client.Index] method with a Namespace in [NewIndexConnParams]
// in order to delete vectors in a namespace other than the default: "".
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: "your-namespace"})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    err = idxConnection.DeleteAllVectorsInNamespace(ctx)
//
//	    if err != nil {
//		       log.Fatalf("Failed to delete vectors in namespace: \"%s\". Error: %s", "your-namespace", err)
//	    }
func (idx *IndexConnection) DeleteAllVectorsInNamespace(ctx context.Context) error {
	req := db_data_grpc.DeleteRequest{
		Namespace: idx.namespace,
		DeleteAll: true,
	}

	return idx.delete(ctx, &req)
}

// [DescribeIndexStatsResponse] is returned by the [IndexConnection.DescribeIndexStats] method.
//
// Fields:
//   - Dimension: The dimension of the [Index].
//   - IndexFullness: The fullness level of the [Index]. Note: only available on pods-based indexes.
//   - TotalVectorCount: The total number of vectors in the [Index].
//   - Namespaces: The namespace(s) in the [Index].
type DescribeIndexStatsResponse struct {
	Dimension        *uint32                      `json:"dimension"`
	IndexFullness    float32                      `json:"index_fullness"`
	TotalVectorCount uint32                       `json:"total_vector_count"`
	Namespaces       map[string]*NamespaceSummary `json:"namespaces,omitempty"`
}

// [IndexConnection.DescribeIndexStats] returns statistics about a Pinecone [Index].
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index:", err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    res, err := idxConnection.DescribeIndexStats(ctx)
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error: %s", idx.Name, err)
//	    } else {
//		       log.Fatalf("%+v", *res)
//	    }
func (idx *IndexConnection) DescribeIndexStats(ctx context.Context) (*DescribeIndexStatsResponse, error) {
	return idx.DescribeIndexStatsFiltered(ctx, nil)
}

// [IndexConnection.DescribeIndexStatsFiltered] returns statistics about a Pinecone [Index], filtered by a given filter.
//
// Returns a pointer to a [DescribeIndexStatsResponse] object or an error if the request fails.
//
// Note: DescribeIndexStatsFiltered is only available on pods-based indexes.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - MetadataFilter: The filter to apply to the request.
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
//
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//
//	    MetadataFilter := map[string]interface{}{
//		       "genre": "classical",
//	    }
//
//	    filter, err := structpb.NewStruct(MetadataFilter)
//
//	    if err != nil {
//		       log.Fatalf("Failed to create filter %+v. Error: %s", MetadataFilter, err)
//	    }
//
//	    res, err := idxConnection.DescribeIndexStatsFiltered(ctx, filter)
//
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error: %s", idx.Name, err)
//	    } else {
//		       for name, summary := range res.Namespaces {
//			       fmt.Printf("Namespace: \"%s\", has %d vector(s) that match the given filter\n", name, summary.VectorCount)
//		       }
//	    }
func (idx *IndexConnection) DescribeIndexStatsFiltered(ctx context.Context, metadataFilter *MetadataFilter) (*DescribeIndexStatsResponse, error) {
	req := &db_data_grpc.DescribeIndexStatsRequest{
		Filter: metadataFilter,
	}
	res, err := (*idx.grpcClient).DescribeIndexStats(idx.akCtx(ctx), req)
	if err != nil {
		return nil, err
	}

	namespaceSummaries := make(map[string]*NamespaceSummary)
	for key, value := range res.Namespaces {
		namespaceSummaries[key] = &NamespaceSummary{
			VectorCount: value.VectorCount,
		}
	}

	return &DescribeIndexStatsResponse{
		Dimension:        res.Dimension,
		IndexFullness:    res.IndexFullness,
		TotalVectorCount: res.TotalVectorCount,
		Namespaces:       namespaceSummaries,
	}, nil
}

// [StartImportResponse] holds the response parameters for the [IndexConnection.StartImport] method.
//
// Fields:
//   - Id: The ID of the import process that was started.
type StartImportResponse struct {
	Id string `json:"id,omitempty"`
}

// [IndexConnection.StartImport] imports data from a storage provider into an [Index]. The uri parameter must start with the
// scheme of a supported storage provider (e.g. "s3://"). For buckets that are not publicly readable, you will also need to
// separately configure a [storage integration] and pass the integration id.
//
// Returns a pointer to a [StartImportResponse] object with the [Import] ID or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - uri: The URI of the data to import. The URI must start with the scheme of a supported storage provider.
//   - integrationId: If your bucket requires authentication to access, you need to pass the id of your storage integration using this property.
//     Pass nil if not required.
//   - errorMode: If set to "continue", the import operation will continue even if some records fail to import.
//     Pass "abort" to stop the import operation if any records fail. Will default to "continue" if nil is passed.
//
// Example:
//
//		 ctx := context.Background()
//
//		 clientParams := pinecone.NewClientParams{
//		     ApiKey:    "YOUR_API_KEY",
//			 SourceTag: "your_source_identifier", // optional
//	     }
//
//	     pc, err := pinecone.NewClient(clientParams)
//	     if err != nil {
//		     log.Fatalf("Failed to create Client: %v", err)
//	     }
//
//	     idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	     if err != nil {
//		     log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	     }
//
//	     idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	     if err != nil {
//		     log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	     }
//
//	     uri := "s3://BUCKET_NAME/PATH/TO/DIR"
//	     errorMode := "continue" // or "abort"
//	     importRes, err := idxConnection.StartImport(ctx, uri, nil, (*pinecone.ImportErrorMode)(&errorMode))
//	     if err != nil {
//	         log.Fatalf("Failed to start import: %v", err)
//	     }
//	     fmt.Printf("Import started with ID: %s", importRes.Id)
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

// [IndexConnection.DescribeImport] retrieves information about a specific [Import] operation.
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
//		       ApiKey:    "YOUR_API_KEY",
//		       SourceTag: "your_source_identifier", // optional
//	    }
//
//	    pc, err := pinecone.NewClient(clientParams)
//	    if err != nil {
//		       log.Fatalf("Failed to create Client: %v", err)
//	    }
//
//	    idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	    if err != nil {
//		       log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	    }
//
//	    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host})
//	    if err != nil {
//		       log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	    }
//	    importDesc, err := idxConnection.DescribeImport(ctx, "your-import-id")
//	    if err != nil {
//		       log.Fatalf("Failed to describe import: %s - %v", "your-import-id", err)
//	    }
//	    fmt.Printf("Import ID: %s, Status: %s", importDesc.Id, importDesc.Status)
func (idx *IndexConnection) DescribeImport(ctx context.Context, id string) (*Import, error) {
	res, err := (*idx.restClient).DescribeBulkImport(idx.akCtx(ctx), id, &db_data_rest.DescribeBulkImportParams{XPineconeApiVersion: gen.PineconeApiVersion})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	importModel, err := decodeImportModel(res.Body)
	if err != nil {
		return nil, err
	}
	return toImport(importModel), nil
}

// [ListImportsRequest] holds the parameters for the [IndexConnection.ListImports] method.
//
// Fields:
//   - Limit: The maximum number of imports to return.
//   - PaginationToken: The token to retrieve the next page of imports, if available.
type ListImportsRequest struct {
	Limit           *int32
	PaginationToken *string
}

// [ListImportsResponse] holds the result of listing [Import] objects.
//
// Fields:
//   - Imports: The list of [Import] objects returned.
//   - NextPaginationToken: The token for paginating through results, if more imports are available.
type ListImportsResponse struct {
	Imports             []*Import `json:"imports,omitempty"`
	NextPaginationToken *string   `json:"next_pagination_token,omitempty"`
}

// [IndexConnection.ListImports] returns information about [Import] operations. It returns operations in a
// paginated form, with a pagination token to fetch the next page of results.
//
// Returns a pointer to a [ListImportsResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - req: A [ListImportsRequest] object containing pagination and filter options.
//
// Example:
//
//	     ctx := context.Background()
//
//	     clientParams := NewClientParams{
//		     ApiKey:    "YOUR_API_KEY",
//		     SourceTag: "your_source_identifier", // optional
//	     }
//
//	     pc, err := NewClient(clientParams)
//	     if err != nil {
//		     log.Fatalf("Failed to create Client: %v", err)
//	     }
//
//	     idx, err := pc.DescribeIndex(ctx, "your-index-name")
//	     if err != nil {
//		     log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//	     }
//
//	     idxConnection, err := pc.Index(NewIndexConnParams{Host: idx.Host})
//	     if err != nil {
//		     log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//	     }
//
//	     limit := int32(10)
//	     firstImportPage, err := idxConnection.ListImports(ctx, &limit, nil)
//	     if err != nil {
//		     log.Fatalf("Failed to list imports: %v", err)
//	     }
//	     fmt.Printf("First page of imports: %+v", firstImportPage.Imports)
//
//	     paginationToken := firstImportPage.NextPaginationToken
//	     nextImportPage, err := idxConnection.ListImports(ctx, &limit, paginationToken)
//	     if err != nil {
//		     log.Fatalf("Failed to list imports: %v", err)
//	     }
//	     fmt.Printf("Second page of imports: %+v", nextImportPage.Imports)
func (idx *IndexConnection) ListImports(ctx context.Context, limit *int32, paginationToken *string) (*ListImportsResponse, error) {
	params := db_data_rest.ListBulkImportsParams{
		Limit:           limit,
		PaginationToken: paginationToken,
	}

	res, err := (*idx.restClient).ListBulkImports(idx.akCtx(ctx), &params)
	if err != nil {
		return nil, err
	}

	listImportsResponse, err := decodeListImportsResponse(res.Body)
	if err != nil {
		return nil, err
	}

	return listImportsResponse, nil
}

// [IndexConnection.CancelImport] cancels an [Import] operation by id.
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
//		ctx := context.Background()
//
//		clientParams := NewClientParams{
//	        ApiKey:    "YOUR_API_KEY",
//			SourceTag: "your_source_identifier", // optional
//		}
//
//		pc, err := NewClient(clientParams)
//		if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//		}
//
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			 log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		}
//
//		idxConnection, err := pc.Index(NewIndexConnParams{Host: idx.Host})
//		if err != nil {
//	         log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//	    err = idxConnection.CancelImport(ctx, "your-import-id")
//	    if err != nil {
//	         log.Fatalf("Failed to cancel import: %s", "your-import-id")
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

type CreateNamespaceParams struct {
	Name   string
	Schema *MetadataSchema
}

func (idx *IndexConnection) CreateNamespace(ctx context.Context, in *CreateNamespaceParams) (*NamespaceDescription, error) {
	req := db_data_grpc.CreateNamespaceRequest{
		Name:   in.Name,
		Schema: fromMetadataSchemaToGrpc(in.Schema),
	}
	res, err := (*idx.grpcClient).CreateNamespace(idx.akCtx(ctx), &req)
	if err != nil {
		return nil, err
	}
	return &NamespaceDescription{Name: res.Name, RecordCount: res.RecordCount}, nil
}

// [IndexConnection.DescribeNamespace] describes a namespace within a serverless index.
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
//		ctx := context.Background()
//
//		clientParams := NewClientParams{
//	        ApiKey:    "YOUR_API_KEY",
//			SourceTag: "your_source_identifier", // optional
//		}
//
//		pc, err := NewClient(clientParams)
//		if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//		}
//
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			 log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		}
//
//		idxConnection, err := pc.Index(NewIndexConnParams{Host: idx.Host})
//		if err != nil {
//	         log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//		namespace, err := idxConnection.DescribeNamespace(ctx, "your-namespace-name")
//		if err != nil {
//			 log.Fatalf("Failed to describe namespace \"%s\". Error:%s", "your-namespace-name", err)
//		}
func (idx *IndexConnection) DescribeNamespace(ctx context.Context, namespace string) (*NamespaceDescription, error) {
	res, err := (*idx.grpcClient).DescribeNamespace(idx.akCtx(ctx), &db_data_grpc.DescribeNamespaceRequest{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}

	nsDesc := &NamespaceDescription{
		Name:        res.Name,
		RecordCount: res.RecordCount,
		Schema:      toMetadataSchemaGrpc(res.Schema),
	}

	if res.IndexedFields != nil {
		nsDesc.IndexedFields = &IndexedFields{
			Fields: res.IndexedFields.Fields,
		}
	}

	return nsDesc, nil
}

// [ListNamespacesResponse] is returned by the [IndexConnection.ListNamespaces] method.
//
// Fields:
//   - Namespaces: A slice of [NamespaceDescription] objects.
//   - Pagination: The [Pagination] object for paginating results.
type ListNamespacesResponse struct {
	Namespaces []*NamespaceDescription
	Pagination *Pagination
	TotalCount int32
}

// [ListNamespacesParams] holds the parameters for the [IndexConnection.ListNamespaces] method.
//
// Fields:
//   - PaginationToken: The token to retrieve the next page of namespaces, if available.
//   - Limit: The maximum number of namespaces to return.
//   - Prefix: The prefix of the namespaces to list.
type ListNamespacesParams struct {
	PaginationToken *string
	Limit           *uint32
	Prefix          *string
}

// [IndexConnection.DescribeNamespace] lists namespaces within a serverless index.
//
// Returns a pointer to a [ListNamespacesResponse] object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A [ListNamespacesParams] object containing limit and pagination options.
//
// Example:
//
//		ctx := context.Background()
//
//		clientParams := NewClientParams{
//	        ApiKey:    "YOUR_API_KEY",
//			SourceTag: "your_source_identifier", // optional
//		}
//
//		pc, err := NewClient(clientParams)
//		if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//		}
//
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			 log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		}
//
//		idxConnection, err := pc.Index(NewIndexConnParams{Host: idx.Host})
//		if err != nil {
//	         log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//		limit := uint32(10)
//		namespaces, err := pc.ListNamespaces(ctx, &pinecone.ListNamespacesParams{ Limit: &limit })
//		if err != nil {
//			 log.Fatalf("Failed to list namespaces for index \"%s\". Error:%s", idx.Name, err)
//		}
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

// [IndexConnection.DeleteNamespace] describes a namespace within a serverless index.
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
//		ctx := context.Background()
//
//		clientParams := NewClientParams{
//	        ApiKey:    "YOUR_API_KEY",
//			SourceTag: "your_source_identifier", // optional
//		}
//
//		pc, err := NewClient(clientParams)
//		if err != nil {
//	        log.Fatalf("Failed to create Client: %v", err)
//		}
//
//		idx, err := pc.DescribeIndex(ctx, "your-index-name")
//		if err != nil {
//			 log.Fatalf("Failed to describe index \"%s\". Error:%s", idx.Name, err)
//		}
//
//		idxConnection, err := pc.Index(NewIndexConnParams{Host: idx.Host})
//		if err != nil {
//	         log.Fatalf("Failed to create IndexConnection for Host: %v. Error: %v", idx.Host, err)
//		}
//
//		err := pc.DeleteNamespace(ctx, "your-namespace-name")
//		if err != nil {
//			 log.Fatalf("Failed to delete namespace \"%s\". Error:%s", "your-namespace-name", err)
//		}
func (idx *IndexConnection) DeleteNamespace(ctx context.Context, namespace string) error {
	_, err := (*idx.grpcClient).DeleteNamespace(idx.akCtx(ctx), &db_data_grpc.DeleteNamespaceRequest{
		Namespace: namespace,
	})
	if err != nil {
		return err
	}
	return nil
}

func (idx *IndexConnection) query(ctx context.Context, req *db_data_grpc.QueryRequest) (*QueryVectorsResponse, error) {
	res, err := (*idx.grpcClient).Query(idx.akCtx(ctx), req)
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
		Namespace: idx.namespace,
	}, nil
}

func (idx *IndexConnection) delete(ctx context.Context, req *db_data_grpc.DeleteRequest) error {
	_, err := (*idx.grpcClient).Delete(idx.akCtx(ctx), req)
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
	return p.Next
}

func toImport(importModel *db_data_rest.ImportModel) *Import {
	if importModel == nil {
		return nil
	}

	return &Import{
		Id:              *importModel.Id,
		Uri:             *importModel.Uri,
		Status:          ImportStatus(*importModel.Status),
		CreatedAt:       importModel.CreatedAt,
		FinishedAt:      importModel.FinishedAt,
		Error:           importModel.Error,
		PercentComplete: derefOrDefault(importModel.PercentComplete, 0),
		RecordsImported: derefOrDefault(importModel.RecordsImported, 0),
	}
}

func toImportResponse(importResponse *db_data_rest.StartImportResponse) *StartImportResponse {
	if importResponse == nil {
		return nil
	}

	return &StartImportResponse{
		Id: derefOrDefault(importResponse.Id, ""),
	}
}

func toListImportsResponse(listImportsResponse *db_data_rest.ListImportsResponse) *ListImportsResponse {
	if listImportsResponse == nil {
		return nil
	}

	imports := make([]*Import, len(*listImportsResponse.Data))
	for i, importModel := range *listImportsResponse.Data {
		imports[i] = toImport(&importModel)
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
		namespaces[i] = &NamespaceDescription{
			Name:        ns.Name,
			RecordCount: ns.RecordCount,
		}
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
