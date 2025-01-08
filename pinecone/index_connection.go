package pinecone

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	db_data_grpc "github.com/pinecone-io/go-pinecone/v2/internal/gen/db_data/grpc"
	db_data_rest "github.com/pinecone-io/go-pinecone/v2/internal/gen/db_data/rest"
	"github.com/pinecone-io/go-pinecone/v2/internal/useragent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// [IndexConnection] holds the parameters for a Pinecone [IndexConnection] object. You can
// instantiate an [IndexConnection] by calling the [Client.Index] method with a [NewIndexConnParams] object.
//
// Fields:
//   - Namespace: The namespace where index operations will be performed.
//   - additionalMetadata: Additional metadata to be sent with each RPC request.
//   - dataClient: The gRPC client for the index.
//   - grpcConn: The gRPC connection.
type IndexConnection struct {
	Namespace          string
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
		Namespace:          in.namespace,
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
//	    metadataMap := map[string]interface{}{
//		       "genre": "classical",
//	    }
//
//	    metadata, err := structpb.NewStruct(metadataMap)
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
//	    vectors := []*pinecone.Vector{
//		       {
//			       Id:           "abc-1",
//			       Values:       []float32{1.0, 2.0},
//			       Metadata:     metadata,
//			       SparseValues: &sparseValues,
//		       },
//	    }
//
//	    count, err := idxConnection.UpsertVectors(ctx, vectors)
//
//	    if err != nil {
//		       log.Fatalf("Failed to upsert vectors. Error: %v", err)
//	    } else {
//		       log.Fatalf("Successfully upserted %d vector(s)!\n", count)
//	    }
func (idx *IndexConnection) UpsertVectors(ctx context.Context, in []*Vector) (uint32, error) {
	vectors := make([]*db_data_grpc.Vector, len(in))
	for i, v := range in {
		vectors[i] = vecToGrpc(v)
	}

	req := &db_data_grpc.UpsertRequest{
		Vectors:   vectors,
		Namespace: idx.Namespace,
	}

	res, err := (*idx.grpcClient).Upsert(idx.akCtx(ctx), req)
	if err != nil {
		return 0, err
	}
	return res.UpsertedCount, nil
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
		Namespace: idx.Namespace,
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
		Namespace: idx.Namespace,
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
		Namespace:       idx.Namespace,
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
		Namespace:           idx.Namespace,
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
		Namespace:       idx.Namespace,
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
		Namespace:       idx.Namespace,
		TopK:            in.TopK,
		Filter:          in.MetadataFilter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
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
		Namespace: idx.Namespace,
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
		Namespace: idx.Namespace,
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
//		       log.Fatalf("Failed to delete vectors in namespace: \"%s\". Error: %s", idxConnection.Namespace, err)
//	    }
func (idx *IndexConnection) DeleteAllVectorsInNamespace(ctx context.Context) error {
	req := db_data_grpc.DeleteRequest{
		Namespace: idx.Namespace,
		DeleteAll: true,
	}

	return idx.delete(ctx, &req)
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
		Namespace:    idx.Namespace,
	}

	_, err := (*idx.grpcClient).Update(idx.akCtx(ctx), req)
	return err
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
//	     uri := "s3://your-bucket/your-file.csv"
//	     errorMode := "abort"
//	     importRes, err := idxConnection.StartImport(ctx, uri, nil, &errorMode)
//	     if err != nil {
//	         log.Fatalf("Failed to start import: %v", err)
//	     }
//	     fmt.Printf("import starteed with ID: %s", importRes.Id)
//
// [storage integration]: https://docs.pinecone.io/guides/operations/integrations/manage-storage-integrations
func (idx *IndexConnection) StartImport(ctx context.Context, uri string, integrationId *string, errorMode *ImportErrorMode) (*StartImportResponse, error) {
	if uri == "" {
		return nil, fmt.Errorf("must specify a uri to start an import")
	}

	req := db_data_rest.StartImportRequest{
		Uri:           uri,
		IntegrationId: integrationId,
	}

	if errorMode != nil {
		req.ErrorMode = &db_data_rest.ImportErrorMode{
			OnError: pointerOrNil(db_data_rest.ImportErrorModeOnError(*errorMode)),
		}
	}

	res, err := (*idx.restClient).StartBulkImport(idx.akCtx(ctx), req)
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
	res, err := (*idx.restClient).DescribeBulkImport(idx.akCtx(ctx), id)
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
	res, err := (*idx.restClient).CancelBulkImport(idx.akCtx(ctx), id)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return handleErrorResponseBody(res, "failed to cancel import: ")
	}

	return nil
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
		Namespace: idx.Namespace,
	}, nil
}

func (idx *IndexConnection) delete(ctx context.Context, req *db_data_grpc.DeleteRequest) error {
	_, err := (*idx.grpcClient).Delete(idx.akCtx(ctx), req)
	return err
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
	return &Vector{
		Id:           vector.Id,
		Values:       vector.Values,
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
		Id:         *importModel.Id,
		Uri:        *importModel.Uri,
		Status:     ImportStatus(*importModel.Status),
		CreatedAt:  importModel.CreatedAt,
		FinishedAt: importModel.FinishedAt,
		Error:      importModel.Error,
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

func vecToGrpc(v *Vector) *db_data_grpc.Vector {
	if v == nil {
		return nil
	}
	return &db_data_grpc.Vector{
		Id:           v.Id,
		Values:       v.Values,
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
