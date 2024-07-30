package pinecone

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	"github.com/pinecone-io/go-pinecone/internal/gen/data"
	"github.com/pinecone-io/go-pinecone/internal/useragent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// IndexConnection holds the parameters for a Pinecone IndexConnection object.
//
// Fields:
//   - Namespace: The namespace where index operations will be performed.
//   - additionalMetadata: Additional metadata to be sent with each RPC request.
//   - dataClient: The gRPC client for the index.
//   - grpcConn: The gRPC connection.
type IndexConnection struct {
	Namespace          string
	additionalMetadata map[string]string
	dataClient         *data.VectorServiceClient
	grpcConn           *grpc.ClientConn
}

type newIndexParameters struct {
	host               string
	namespace          string
	sourceTag          string
	additionalMetadata map[string]string
}

func newIndexConnection(in newIndexParameters, dialOpts ...grpc.DialOption) (*IndexConnection, error) {
	config := &tls.Config{}
	target := fmt.Sprintf("%s:443", in.host)

	// configure default gRPC DialOptions
	grpcOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithAuthority(target),
		grpc.WithUserAgent(useragent.BuildUserAgentGRPC(in.sourceTag)),
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

	dataClient := data.NewVectorServiceClient(conn)

	idx := IndexConnection{
		Namespace:          in.namespace,
		dataClient:         &dataClient,
		grpcConn:           conn,
		additionalMetadata: in.additionalMetadata,
	}
	return &idx, nil
}

// Close closes the grpc.ClientConn to a Pinecone index.
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

// UpsertVectors upserts vectors into a Pinecone index.
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
	vectors := make([]*data.Vector, len(in))
	for i, v := range in {
		vectors[i] = vecToGrpc(v)
	}

	req := &data.UpsertRequest{
		Vectors:   vectors,
		Namespace: idx.Namespace,
	}

	res, err := (*idx.dataClient).Upsert(idx.akCtx(ctx), req)
	if err != nil {
		return 0, err
	}
	return res.UpsertedCount, nil
}

// FetchVectorsResponse holds the parameters for the FetchVectorsResponse object,
// which is returned by the FetchVectors method.
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

// FetchVectors fetches vectors by ID from a Pinecone index.
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
	req := &data.FetchRequest{
		Ids:       ids,
		Namespace: idx.Namespace,
	}

	res, err := (*idx.dataClient).Fetch(idx.akCtx(ctx), req)
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

// ListVectorsRequest holds the parameters for the ListVectorsRequest object,
// which is passed into the ListVectors method.
//
// Fields:
//   - Prefix: The prefix by which to filter. If unspecified,
//     an empty string will be used which will list all vector ids in the namespace
//   - Limit: The maximum number of vectors to return. If unspecified, the server will use a default value.
//   - PaginationToken: The token for paginating through results.
type ListVectorsRequest struct {
	Prefix          *string
	Limit           *uint32
	PaginationToken *string
}

// ListVectorsResponse holds the parameters for the ListVectorsResponse object,
// which is returned by the ListVectors method.
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

// ListVectors lists vectors in a Pinecone index. You can filter vectors by prefix,
// limit the number of vectors returned, and paginate through results.
//
// Note: ListVectors is only available for Serverless indexes.
//
// Returns a pointer to a ListVectorsResponse object or an error if the request fails.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A ListVectorsRequest object with the parameters for the request.
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
	req := &data.ListRequest{
		Prefix:          in.Prefix,
		Limit:           in.Limit,
		PaginationToken: in.PaginationToken,
		Namespace:       idx.Namespace,
	}
	res, err := (*idx.dataClient).List(idx.akCtx(ctx), req)
	if err != nil {
		return nil, err
	}

	vectorIds := make([]*string, len(res.Vectors))
	for i := 0; i < len(res.Vectors); i++ {
		vectorIds[i] = &res.Vectors[i].Id
	}

	return &ListVectorsResponse{
		VectorIds:           vectorIds,
		Usage:               &Usage{ReadUnits: derefOrDefault(res.Usage.ReadUnits, 0)},
		NextPaginationToken: toPaginationToken(res.Pagination),
		Namespace:           idx.Namespace,
	}, nil
}

// QueryByVectorValuesRequest holds the parameters for the QueryByVectorValuesRequest object,
// which is passed into the QueryByVectorValues method.
//
// Fields:
//   - Vector: The query vector used to find similar vectors.
//   - TopK: The number of vectors to return.
//   - MetadataFilter: The filter to apply to your query.
//   - IncludeValues: Whether to include the values of the vectors in the response.
//   - IncludeMetadata: Whether to include the metadata associated with the vectors in the response.
//   - SparseValues: The sparse values of the query vector, if applicable.
type QueryByVectorValuesRequest struct {
	Vector          []float32
	TopK            uint32
	MetadataFilter  *MetadataFilter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

// QueryVectorsResponse holds the parameters for the QueryVectorsResponse object,
// which is returned by the QueryByVectorValues method.
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

// QueryByVectorValues queries a Pinecone index for vectors that are most similar to a provided query vector.
//
// Returns a pointer to a QueryVectorsResponse object or an error if the request fails.
//
// Note: To issue a hybrid query with both dense and sparse values,
// your index's similarity metric must be dot-product.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: A QueryByVectorValuesRequest object with the parameters for the request.
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
	req := &data.QueryRequest{
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

// QueryByVectorIdRequest holds the parameters for the QueryByVectorIdRequest object,
// which is passed into the QueryByVectorId method.
//
// Fields:
//   - VectorId: The unique ID of the vector used to find similar vectors.
//   - TopK: The number of vectors to return.
//   - MetadataFilter: The filter to apply to your query.
//   - IncludeValues: Whether to include the values of the vectors in the response.
//   - IncludeMetadata: Whether to include the metadata associated with the vectors in the response.
//   - SparseValues: The sparse values of the query vector, if applicable.
type QueryByVectorIdRequest struct {
	VectorId        string
	TopK            uint32
	metadataFilter  *MetadataFilter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

// QueryByVectorId uses a vector ID to query a Pinecone index and retrieve vectors that are most similar to the
// provided ID's underlying vector.
//
// Returns a pointer to a QueryVectorsResponse object or an error if the request fails.
//
// Note: QueryByVectorId executes a nearest neighbors search, meaning that unless TopK=1 in the QueryByVectorIdRequest
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
	req := &data.QueryRequest{
		Id:              in.VectorId,
		Namespace:       idx.Namespace,
		TopK:            in.TopK,
		Filter:          in.metadataFilter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
}

// DeleteVectorsById deletes vectors by ID from a Pinecone index.
//
// Returns an error if the request fails,
// otherwise returns nil. This method will also return nil if the passed vector ID does not exist in the index or
// namespace.
//
// Note: You must instantiate an Index connection with a Namespace in NewIndexConnParams in order to delete vectors
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
	req := data.DeleteRequest{
		Ids:       ids,
		Namespace: idx.Namespace,
	}

	return idx.delete(ctx, &req)
}

// DeleteVectorsByFilter deletes vectors from a Pinecone index, given a filter.
//
// Returns an error if the request fails, otherwise returns nil.
//
// Note: DeleteVectorsByFilter is only available on pods-based indexes.
// Additionally, you must instantiate an IndexConnection using the Index method with a Namespace in NewIndexConnParams
// in order to delete vectors in a namespace other than the default.
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
	req := data.DeleteRequest{
		Filter:    metadataFilter,
		Namespace: idx.Namespace,
	}

	return idx.delete(ctx, &req)
}

// DeleteAllVectorsInNamespace deletes all vectors in a specific namespace.
//
// Returns an error if the request fails, otherwise returns nil.
//
// Note: You must instantiate an IndexConnection using the Index method with a Namespace in NewIndexConnParams
// in order to delete vectors in a namespace other than the default.
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
	req := data.DeleteRequest{
		Namespace: idx.Namespace,
		DeleteAll: true,
	}

	return idx.delete(ctx, &req)
}

// UpdateVectorRequest holds the parameters for the UpdateVectorRequest object,
// which is passed into the UpdateVector method.
//
// Fields:
//   - Id: The unique ID of the vector to update.
//   - Values: The values with which you want to update the vector.
//   - SparseValues: The sparse values with which you want to update the vector.
//   - Metadata: The metadata with which you want to update the vector.
type UpdateVectorRequest struct {
	Id           string
	Values       []float32
	SparseValues *SparseValues
	Metadata     *Metadata
}

// UpdateVector updates a vector in a Pinecone index by ID.
//
// Returns an error if the request fails, returns nil otherwise.
//
// Parameters:
//   - ctx: A context.Context object controls the request's lifetime,
//     allowing for the request to be canceled or to timeout according to the context's deadline.
//   - in: An UpdateVectorRequest object with the parameters for the request.
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
	req := &data.UpdateRequest{
		Id:           in.Id,
		Values:       in.Values,
		SparseValues: sparseValToGrpc(in.SparseValues),
		SetMetadata:  in.Metadata,
		Namespace:    idx.Namespace,
	}

	_, err := (*idx.dataClient).Update(idx.akCtx(ctx), req)
	return err
}

// DescribeIndexStatsResponse holds the parameters for the DescribeIndexStatsResponse object,
// which is returned by the DescribeIndexStats method.
//
// Fields:
//   - Dimension: The dimension of the index.
//   - IndexFullness: The fullness level of the index. Note: only available on pods-based indexes.
//   - TotalVectorCount: The total number of vectors in the index.
//   - Namespaces: The namespace(s) in the index.
type DescribeIndexStatsResponse struct {
	Dimension        uint32                       `json:"dimension"`
	IndexFullness    float32                      `json:"index_fullness"`
	TotalVectorCount uint32                       `json:"total_vector_count"`
	Namespaces       map[string]*NamespaceSummary `json:"namespaces,omitempty"`
}

// DescribeIndexStats returns statistics about a Pinecone index.
//
// Returns a pointer to a DescribeIndexStatsResponse object or an error if the request fails.
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

// DescribeIndexStatsFiltered returns statistics about a Pinecone index, filtered by a given filter.
//
// Returns a pointer to a DescribeIndexStatsResponse object or an error if the request fails.
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
	req := &data.DescribeIndexStatsRequest{
		Filter: metadataFilter,
	}
	res, err := (*idx.dataClient).DescribeIndexStats(idx.akCtx(ctx), req)
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

func (idx *IndexConnection) query(ctx context.Context, req *data.QueryRequest) (*QueryVectorsResponse, error) {
	res, err := (*idx.dataClient).Query(idx.akCtx(ctx), req)
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

func (idx *IndexConnection) delete(ctx context.Context, req *data.DeleteRequest) error {
	_, err := (*idx.dataClient).Delete(idx.akCtx(ctx), req)
	return err
}

func toVector(vector *data.Vector) *Vector {
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

func toScoredVector(sv *data.ScoredVector) *ScoredVector {
	if sv == nil {
		return nil
	}
	v := toVector(&data.Vector{
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

func toSparseValues(sv *data.SparseValues) *SparseValues {
	if sv == nil {
		return nil
	}
	return &SparseValues{
		Indices: sv.Indices,
		Values:  sv.Values,
	}
}

func toUsage(u *data.Usage) *Usage {
	if u == nil {
		return nil
	}
	return &Usage{
		ReadUnits: derefOrDefault(u.ReadUnits, 0),
	}
}

func toPaginationToken(p *data.Pagination) *string {
	if p == nil {
		return nil
	}
	return &p.Next
}

func vecToGrpc(v *Vector) *data.Vector {
	if v == nil {
		return nil
	}
	return &data.Vector{
		Id:           v.Id,
		Values:       v.Values,
		Metadata:     v.Metadata,
		SparseValues: sparseValToGrpc(v.SparseValues),
	}
}

func sparseValToGrpc(sv *SparseValues) *data.SparseValues {
	if sv == nil {
		return nil
	}
	return &data.SparseValues{
		Indices: sv.Indices,
		Values:  sv.Values,
	}
}

func (idx *IndexConnection) akCtx(ctx context.Context) context.Context {
	newMetadata := []string{}

	for key, value := range idx.additionalMetadata {
		newMetadata = append(newMetadata, key, value)
	}

	return metadata.AppendToOutgoingContext(ctx, newMetadata...)
}
