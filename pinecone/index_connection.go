package pinecone

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/pinecone-io/go-pinecone/internal/gen/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"log"
)

type IndexConnection struct {
	apiKey     string
	dataClient *data.VectorServiceClient
	grpcConn   *grpc.ClientConn
}

func newIndexConnection(apiKey string, host string) (*IndexConnection, error) {
	config := &tls.Config{}
	target := fmt.Sprintf("%s:443", host)
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithAuthority(target),
		grpc.WithBlock(),
	)

	if err != nil {
		log.Fatalf("fail to dial: %v", err)
		return nil, err
	}

	dataClient := data.NewVectorServiceClient(conn)

	idx := IndexConnection{apiKey: apiKey, dataClient: &dataClient, grpcConn: conn}
	return &idx, nil
}

func (idx *IndexConnection) Close() error {
	err := idx.grpcConn.Close()
	return err
}

type UpsertVectorsRequest struct {
	Vectors   []*Vector
	Namespace string
}

func (idx *IndexConnection) UpsertVectors(ctx *context.Context, in *UpsertVectorsRequest) (uint32, error) {
	vectors := make([]*data.Vector, len(in.Vectors))
	for i, v := range in.Vectors {
		vectors[i] = vecToGrpc(v)
	}

	req := &data.UpsertRequest{
		Vectors:   vectors,
		Namespace: in.Namespace,
	}

	res, err := (*idx.dataClient).Upsert(idx.akCtx(*ctx), req)
	if err != nil {
		return 0, err
	}
	return res.UpsertedCount, nil
}

type FetchVectorsRequest struct {
	Ids       []string
	Namespace string
}

type FetchVectorsResponse struct {
	Vectors   map[string]*Vector
	Namespace string
	Usage     *Usage
}

func (idx *IndexConnection) FetchVectors(ctx *context.Context, in *FetchVectorsRequest) (*FetchVectorsResponse, error) {
	req := &data.FetchRequest{
		Ids:       in.Ids,
		Namespace: in.Namespace,
	}

	res, err := (*idx.dataClient).Fetch(idx.akCtx(*ctx), req)
	if err != nil {
		return nil, err
	}

	vectors := make(map[string]*Vector, len(res.Vectors))
	for id, vector := range res.Vectors {
		vectors[id] = toVector(vector)
	}

	return &FetchVectorsResponse{
		Vectors:   vectors,
		Namespace: res.Namespace,
		Usage:     toUsage(res.Usage),
	}, nil
}

type ListVectorsRequest struct {
	Prefix          *string
	Limit           *uint32
	PaginationToken *string
	Namespace       string
}

type ListVectorsResponse struct {
	VectorIds           []*string
	Namespace           string
	Usage               *Usage
	NextPaginationToken *string
}

func (idx *IndexConnection) ListVectors(ctx *context.Context, in *ListVectorsRequest) (*ListVectorsResponse, error) {
	req := &data.ListRequest{
		Prefix:          in.Prefix,
		Limit:           in.Limit,
		PaginationToken: in.PaginationToken,
		Namespace:       in.Namespace,
	}
	res, err := (*idx.dataClient).List(idx.akCtx(*ctx), req)
	if err != nil {
		return nil, err
	}

	vectorIds := make([]*string, len(res.Vectors))
	for i := 0; i < len(res.Vectors); i++ {
		vectorIds[i] = &res.Vectors[i].Id
	}

	return &ListVectorsResponse{
		VectorIds:           vectorIds,
		Namespace:           res.Namespace,
		Usage:               &Usage{ReadUnits: res.Usage.ReadUnits},
		NextPaginationToken: toPaginationToken(res.Pagination),
	}, nil
}

type QueryByVectorValuesRequest struct {
	Vector          []float32
	Namespace       string
	TopK            uint32
	Filter          *Filter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

type QueryVectorsResponse struct {
	Matches   []*ScoredVector
	Namespace string
	Usage     *Usage
}

func (idx *IndexConnection) QueryByVectorValues(ctx *context.Context, in *QueryByVectorValuesRequest) (*QueryVectorsResponse, error) {
	req := &data.QueryRequest{
		Namespace:       in.Namespace,
		TopK:            in.TopK,
		Filter:          in.Filter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		Vector:          in.Vector,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
}

type QueryByVectorIdRequest struct {
	VectorId        string
	Namespace       string
	TopK            uint32
	Filter          *Filter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

func (idx *IndexConnection) QueryByVectorId(ctx *context.Context, in *QueryByVectorIdRequest) (*QueryVectorsResponse, error) {
	req := &data.QueryRequest{
		Id:              in.VectorId,
		Namespace:       in.Namespace,
		TopK:            in.TopK,
		Filter:          in.Filter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
}

type DeleteVectorsRequest struct {
	Namespace string
	Ids       []string
	Filter    *Filter
	DeleteAll bool
}

func (idx *IndexConnection) DeleteVectors(ctx *context.Context, in *DeleteVectorsRequest) error {
	req := data.DeleteRequest{
		Ids:       in.Ids,
		DeleteAll: in.DeleteAll,
		Namespace: in.Namespace,
		Filter:    in.Filter,
	}

	_, err := (*idx.dataClient).Delete(idx.akCtx(*ctx), &req)
	return err
}

type UpdateVectorRequest struct {
	Id           string
	Values       []float32
	SparseValues *SparseValues
	Metadata     *Metadata
	Namespace    string
}

func (idx *IndexConnection) UpdateVector(ctx *context.Context, in *UpdateVectorRequest) error {
	req := &data.UpdateRequest{
		Id:           in.Id,
		Values:       in.Values,
		SparseValues: sparseValToGrpc(in.SparseValues),
		SetMetadata:  in.Metadata,
		Namespace:    in.Namespace,
	}

	_, err := (*idx.dataClient).Update(idx.akCtx(*ctx), req)
	return err
}

type DescribeIndexStatsRequest struct {
	Filter *Filter
}

type DescribeIndexStatsResponse struct {
	Dimension        uint32
	IndexFullness    float32
	TotalVectorCount uint32
	Namespaces       map[string]*NamespaceSummary
}

func (idx *IndexConnection) DescribeIndexStats(ctx *context.Context, in *DescribeIndexStatsRequest) (*DescribeIndexStatsResponse, error) {
	req := &data.DescribeIndexStatsRequest{
		Filter: in.Filter,
	}
	res, err := (*idx.dataClient).DescribeIndexStats(idx.akCtx(*ctx), req)
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

func (idx *IndexConnection) query(ctx *context.Context, req *data.QueryRequest) (*QueryVectorsResponse, error) {
	res, err := (*idx.dataClient).Query(idx.akCtx(*ctx), req)
	if err != nil {
		return nil, err
	}

	matches := make([]*ScoredVector, len(res.Matches))
	for i, match := range res.Matches {
		matches[i] = toScoredVector(match)
	}

	return &QueryVectorsResponse{
		Matches:   matches,
		Namespace: res.Namespace,
		Usage:     toUsage(res.Usage),
	}, nil
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
		ReadUnits: u.ReadUnits,
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
	return metadata.AppendToOutgoingContext(ctx, "api-key", idx.apiKey)
}
