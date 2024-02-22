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
	"time"
)

type IndexConnection struct {
	host       string
	dataClient *data.VectorServiceClient
	ctx        *context.Context
	ctxCancel  context.CancelFunc
	grpcConn   *grpc.ClientConn
}

func newIndexConnection(apiKey string, host string) (*IndexConnection, error) {
	config := &tls.Config{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	ctx = metadata.AppendToOutgoingContext(ctx, "api-key", apiKey)
	target := fmt.Sprintf("%s:443", host)

	conn, err := grpc.DialContext(
		ctx,
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

	idx := IndexConnection{host: host, dataClient: &dataClient, ctx: &ctx, ctxCancel: cancel, grpcConn: conn}
	return &idx, nil
}

func (idx *IndexConnection) Close() error {
	idx.ctxCancel()
	err := idx.grpcConn.Close()
	return err
}

type UpsertVectorsRequest struct {
	Vectors   []*Vector
	Namespace string
}

func (idx *IndexConnection) UpsertVectors(in *UpsertVectorsRequest) (uint32, error) {
	vectors := make([]*data.Vector, len(in.Vectors))
	for i, v := range in.Vectors {
		vectors[i] = vecToGrpc(v)
	}

	req := &data.UpsertRequest{
		Vectors:   vectors,
		Namespace: in.Namespace,
	}

	res, err := (*idx.dataClient).Upsert(*idx.ctx, req)
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

func (idx *IndexConnection) FetchVectors(in *FetchVectorsRequest) (*FetchVectorsResponse, error) {
	req := &data.FetchRequest{
		Ids:       in.Ids,
		Namespace: in.Namespace,
	}

	res, err := (*idx.dataClient).Fetch(*idx.ctx, req)
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

func (idx *IndexConnection) ListVectors(in *ListVectorsRequest) (*ListVectorsResponse, error) {
	req := &data.ListRequest{
		Prefix:          in.Prefix,
		Limit:           in.Limit,
		PaginationToken: in.PaginationToken,
		Namespace:       in.Namespace,
	}
	res, err := (*idx.dataClient).List(*idx.ctx, req)
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

type QueryByVectorRequest struct {
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

func (idx *IndexConnection) QueryByVector(in *QueryByVectorRequest) (*QueryVectorsResponse, error) {
	req := &data.QueryRequest{
		Namespace:       in.Namespace,
		TopK:            in.TopK,
		Filter:          in.Filter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		Vector:          in.Vector,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(req)
}

type QueryByIdRequest struct {
	Id              string
	Namespace       string
	TopK            uint32
	Filter          *Filter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

func (idx *IndexConnection) QueryById(in *QueryByIdRequest) (*QueryVectorsResponse, error) {
	req := &data.QueryRequest{
		Id:              in.Id,
		Namespace:       in.Namespace,
		TopK:            in.TopK,
		Filter:          in.Filter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(req)
}

type DeleteVectorsRequest struct {
	Namespace string
	Ids       []string
	Filter    *Filter
	DeleteAll bool
}

func (idx *IndexConnection) DeleteVectors(in *DeleteVectorsRequest) error {
	req := data.DeleteRequest{
		Ids:       in.Ids,
		DeleteAll: in.DeleteAll,
		Namespace: in.Namespace,
		Filter:    in.Filter,
	}

	_, err := (*idx.dataClient).Delete(*idx.ctx, &req)
	return err
}

type UpdateVectorRequest struct {
	Id           string
	Values       []float32
	SparseValues *SparseValues
	Metadata     *Metadata
	Namespace    string
}

func (idx *IndexConnection) UpdateVector(in *UpdateVectorRequest) error {
	req := &data.UpdateRequest{
		Id:           in.Id,
		Values:       in.Values,
		SparseValues: sparseValToGrpc(in.SparseValues),
		SetMetadata:  in.Metadata,
		Namespace:    in.Namespace,
	}

	_, err := (*idx.dataClient).Update(*idx.ctx, req)
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

func (idx *IndexConnection) DescribeIndexStats(in *DescribeIndexStatsRequest) (*DescribeIndexStatsResponse, error) {
	req := &data.DescribeIndexStatsRequest{
		Filter: in.Filter,
	}
	res, err := (*idx.dataClient).DescribeIndexStats(*idx.ctx, req)
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

func (idx *IndexConnection) query(req *data.QueryRequest) (*QueryVectorsResponse, error) {
	res, err := (*idx.dataClient).Query(*idx.ctx, req)
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
	return &Vector{
		Id:           vector.Id,
		Values:       vector.Values,
		Metadata:     vector.Metadata,
		SparseValues: toSparseValues(vector.SparseValues),
	}
}

func toScoredVector(sv *data.ScoredVector) *ScoredVector {
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
