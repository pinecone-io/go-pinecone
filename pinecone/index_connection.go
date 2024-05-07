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

type IndexConnection struct {
	Namespace          string
	apiKey             string
	additionalMetadata map[string]string
	dataClient         *data.VectorServiceClient
	grpcConn           *grpc.ClientConn
}

type newIndexParameters struct {
	apiKey             string
	host               string
	namespace          string
	sourceTag          string
	additionalMetadata map[string]string
}

func newIndexConnection(in newIndexParameters) (*IndexConnection, error) {
	config := &tls.Config{}
	target := fmt.Sprintf("%s:443", in.host)
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithAuthority(target),
		grpc.WithBlock(),
		grpc.WithUserAgent(useragent.BuildUserAgentGRPC(in.sourceTag)),
	)

	if err != nil {
		log.Fatalf("fail to dial: %v", err)
		return nil, err
	}

	dataClient := data.NewVectorServiceClient(conn)

	idx := IndexConnection{Namespace: in.namespace, apiKey: in.apiKey, dataClient: &dataClient, grpcConn: conn, additionalMetadata: in.additionalMetadata}
	return &idx, nil
}

func (idx *IndexConnection) Close() error {
	err := idx.grpcConn.Close()
	return err
}

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

type FetchVectorsResponse struct {
	Vectors map[string]*Vector `json:"vectors,omitempty"`
	Usage   *Usage             `json:"usage,omitempty"`
}

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
	fmt.Printf("VECTORS: %+v\n", vectors)

	return &FetchVectorsResponse{
		Vectors: vectors,
		Usage:   toUsage(res.Usage),
	}, nil
}

type ListVectorsRequest struct {
	Prefix          *string
	Limit           *uint32
	PaginationToken *string
}

type ListVectorsResponse struct {
	VectorIds           []*string `json:"vector_ids,omitempty"`
	Usage               *Usage    `json:"usage,omitempty"`
	NextPaginationToken *string   `json:"next_pagination_token,omitempty"`
}

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
	}, nil
}

type QueryByVectorValuesRequest struct {
	Vector          []float32
	TopK            uint32
	Filter          *Filter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

type QueryVectorsResponse struct {
	Matches []*ScoredVector `json:"matches,omitempty"`
	Usage   *Usage          `json:"usage,omitempty"`
}

func (idx *IndexConnection) QueryByVectorValues(ctx context.Context, in *QueryByVectorValuesRequest) (*QueryVectorsResponse, error) {
	req := &data.QueryRequest{
		Namespace:       idx.Namespace,
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
	TopK            uint32
	Filter          *Filter
	IncludeValues   bool
	IncludeMetadata bool
	SparseValues    *SparseValues
}

func (idx *IndexConnection) QueryByVectorId(ctx context.Context, in *QueryByVectorIdRequest) (*QueryVectorsResponse, error) {
	req := &data.QueryRequest{
		Id:              in.VectorId,
		Namespace:       idx.Namespace,
		TopK:            in.TopK,
		Filter:          in.Filter,
		IncludeValues:   in.IncludeValues,
		IncludeMetadata: in.IncludeMetadata,
		SparseVector:    sparseValToGrpc(in.SparseValues),
	}

	return idx.query(ctx, req)
}

func (idx *IndexConnection) DeleteVectorsById(ctx context.Context, ids []string) error {
	req := data.DeleteRequest{
		Ids:       ids,
		Namespace: idx.Namespace,
	}

	return idx.delete(ctx, &req)
}

func (idx *IndexConnection) DeleteVectorsByFilter(ctx context.Context, filter *Filter) error {
	req := data.DeleteRequest{
		Filter:    filter,
		Namespace: idx.Namespace,
	}

	return idx.delete(ctx, &req)
}

func (idx *IndexConnection) DeleteAllVectorsInNamespace(ctx context.Context) error {
	req := data.DeleteRequest{
		Namespace: idx.Namespace,
		DeleteAll: true,
	}

	return idx.delete(ctx, &req)
}

type UpdateVectorRequest struct {
	Id           string
	Values       []float32
	SparseValues *SparseValues
	Metadata     *Metadata
}

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

type DescribeIndexStatsResponse struct {
	Dimension        uint32                       `json:"dimension"`
	IndexFullness    float32                      `json:"index_fullness"`
	TotalVectorCount uint32                       `json:"total_vector_count"`
	Namespaces       map[string]*NamespaceSummary `json:"namespaces,omitempty"`
}

func (idx *IndexConnection) DescribeIndexStats(ctx context.Context) (*DescribeIndexStatsResponse, error) {
	return idx.DescribeIndexStatsFiltered(ctx, nil)
}

func (idx *IndexConnection) DescribeIndexStatsFiltered(ctx context.Context, filter *Filter) (*DescribeIndexStatsResponse, error) {
	req := &data.DescribeIndexStatsRequest{
		Filter: filter,
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
		Matches: matches,
		Usage:   toUsage(res.Usage),
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
	newMetadata = append(newMetadata, "api-key", idx.apiKey)

	for key, value := range idx.additionalMetadata {
		newMetadata = append(newMetadata, key, value)
	}

	return metadata.AppendToOutgoingContext(ctx, newMetadata...)
}
