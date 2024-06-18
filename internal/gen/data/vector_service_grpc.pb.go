// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.4.0
// - protoc             v3.20.3
// source: pinecone/data/v1/vector_service.proto

package data

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.62.0 or later.
const _ = grpc.SupportPackageIsVersion8

const (
	VectorService_Upsert_FullMethodName             = "/VectorService/Upsert"
	VectorService_Delete_FullMethodName             = "/VectorService/Delete"
	VectorService_Fetch_FullMethodName              = "/VectorService/Fetch"
	VectorService_List_FullMethodName               = "/VectorService/List"
	VectorService_Query_FullMethodName              = "/VectorService/Query"
	VectorService_Update_FullMethodName             = "/VectorService/Update"
	VectorService_DescribeIndexStats_FullMethodName = "/VectorService/DescribeIndexStats"
)

// VectorServiceClient is the client API for VectorService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// The `VectorService` interface is exposed by Pinecone's vector index services.
// This service could also be called a `gRPC` service or a `REST`-like api.
type VectorServiceClient interface {
	// Upsert vectors
	//
	// The `upsert` operation writes vectors into a namespace. If a new value is upserted for an existing vector ID, it will overwrite the previous value.
	//
	// For guidance and examples, see [Upsert data](https://docs.pinecone.io/docs/upsert-data).
	Upsert(ctx context.Context, in *UpsertRequest, opts ...grpc.CallOption) (*UpsertResponse, error)
	// Delete vectors
	//
	// The `delete` operation deletes vectors, by id, from a single namespace.
	//
	// For guidance and examples, see [Delete data](https://docs.pinecone.io/docs/delete-data).
	Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error)
	// Fetch vectors
	//
	// The `fetch` operation looks up and returns vectors, by ID, from a single namespace. The returned vectors include the vector data and/or metadata.
	//
	// For guidance and examples, see [Fetch data](https://docs.pinecone.io/reference/fetch).
	Fetch(ctx context.Context, in *FetchRequest, opts ...grpc.CallOption) (*FetchResponse, error)
	// List vector IDs
	//
	// The `list` operation lists the IDs of vectors in a single namespace of a serverless index. An optional prefix can be passed to limit the results to IDs with a common prefix.
	//
	// `list` returns up to 100 IDs at a time by default in sorted order (bitwise/"C" collation). If the `limit` parameter is set, `list` returns up to that number of IDs instead. Whenever there are additional IDs to return, the response also includes a `pagination_token` that you can use to get the next batch of IDs. When the response does not include a `pagination_token`, there are no more IDs to return.
	//
	// For guidance and examples, see [Get record IDs](https://docs.pinecone.io/docs/get-record-ids).
	//
	// **Note:** `list` is supported only for serverless indexes.
	List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error)
	// Query vectors
	//
	// The `query` operation searches a namespace, using a query vector. It retrieves the ids of the most similar items in a namespace, along with their similarity scores.
	//
	// For guidance and examples, see [Query data](https://docs.pinecone.io/docs/query-data).
	Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error)
	// Update a vector
	//
	// The `update` operation updates a vector in a namespace. If a value is included, it will overwrite the previous value. If a `set_metadata` is included, the values of the fields specified in it will be added or overwrite the previous value.
	//
	// For guidance and examples, see [Update data](https://docs.pinecone.io/reference/update).
	Update(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (*UpdateResponse, error)
	// Get index stats
	//
	// The `describe_index_stats` operation returns statistics about the contents of an index, including the vector count per namespace and the number of dimensions, and the index fullness.
	//
	// Serverless indexes scale automatically as needed, so index fullness is relevant only for pod-based indexes.
	//
	// For pod-based indexes, the index fullness result may be inaccurate during pod resizing; to get the status of a pod resizing process, use [`describe_index`](https://www.pinecone.io/docs/api/operation/describe_index/).
	DescribeIndexStats(ctx context.Context, in *DescribeIndexStatsRequest, opts ...grpc.CallOption) (*DescribeIndexStatsResponse, error)
}

type vectorServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewVectorServiceClient(cc grpc.ClientConnInterface) VectorServiceClient {
	return &vectorServiceClient{cc}
}

func (c *vectorServiceClient) Upsert(ctx context.Context, in *UpsertRequest, opts ...grpc.CallOption) (*UpsertResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpsertResponse)
	err := c.cc.Invoke(ctx, VectorService_Upsert_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteResponse)
	err := c.cc.Invoke(ctx, VectorService_Delete_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Fetch(ctx context.Context, in *FetchRequest, opts ...grpc.CallOption) (*FetchResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(FetchResponse)
	err := c.cc.Invoke(ctx, VectorService_Fetch_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListResponse)
	err := c.cc.Invoke(ctx, VectorService_List_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryResponse)
	err := c.cc.Invoke(ctx, VectorService_Query_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Update(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (*UpdateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateResponse)
	err := c.cc.Invoke(ctx, VectorService_Update_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) DescribeIndexStats(ctx context.Context, in *DescribeIndexStatsRequest, opts ...grpc.CallOption) (*DescribeIndexStatsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DescribeIndexStatsResponse)
	err := c.cc.Invoke(ctx, VectorService_DescribeIndexStats_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VectorServiceServer is the server API for VectorService service.
// All implementations must embed UnimplementedVectorServiceServer
// for forward compatibility
//
// The `VectorService` interface is exposed by Pinecone's vector index services.
// This service could also be called a `gRPC` service or a `REST`-like api.
type VectorServiceServer interface {
	// Upsert vectors
	//
	// The `upsert` operation writes vectors into a namespace. If a new value is upserted for an existing vector ID, it will overwrite the previous value.
	//
	// For guidance and examples, see [Upsert data](https://docs.pinecone.io/docs/upsert-data).
	Upsert(context.Context, *UpsertRequest) (*UpsertResponse, error)
	// Delete vectors
	//
	// The `delete` operation deletes vectors, by id, from a single namespace.
	//
	// For guidance and examples, see [Delete data](https://docs.pinecone.io/docs/delete-data).
	Delete(context.Context, *DeleteRequest) (*DeleteResponse, error)
	// Fetch vectors
	//
	// The `fetch` operation looks up and returns vectors, by ID, from a single namespace. The returned vectors include the vector data and/or metadata.
	//
	// For guidance and examples, see [Fetch data](https://docs.pinecone.io/reference/fetch).
	Fetch(context.Context, *FetchRequest) (*FetchResponse, error)
	// List vector IDs
	//
	// The `list` operation lists the IDs of vectors in a single namespace of a serverless index. An optional prefix can be passed to limit the results to IDs with a common prefix.
	//
	// `list` returns up to 100 IDs at a time by default in sorted order (bitwise/"C" collation). If the `limit` parameter is set, `list` returns up to that number of IDs instead. Whenever there are additional IDs to return, the response also includes a `pagination_token` that you can use to get the next batch of IDs. When the response does not include a `pagination_token`, there are no more IDs to return.
	//
	// For guidance and examples, see [Get record IDs](https://docs.pinecone.io/docs/get-record-ids).
	//
	// **Note:** `list` is supported only for serverless indexes.
	List(context.Context, *ListRequest) (*ListResponse, error)
	// Query vectors
	//
	// The `query` operation searches a namespace, using a query vector. It retrieves the ids of the most similar items in a namespace, along with their similarity scores.
	//
	// For guidance and examples, see [Query data](https://docs.pinecone.io/docs/query-data).
	Query(context.Context, *QueryRequest) (*QueryResponse, error)
	// Update a vector
	//
	// The `update` operation updates a vector in a namespace. If a value is included, it will overwrite the previous value. If a `set_metadata` is included, the values of the fields specified in it will be added or overwrite the previous value.
	//
	// For guidance and examples, see [Update data](https://docs.pinecone.io/reference/update).
	Update(context.Context, *UpdateRequest) (*UpdateResponse, error)
	// Get index stats
	//
	// The `describe_index_stats` operation returns statistics about the contents of an index, including the vector count per namespace and the number of dimensions, and the index fullness.
	//
	// Serverless indexes scale automatically as needed, so index fullness is relevant only for pod-based indexes.
	//
	// For pod-based indexes, the index fullness result may be inaccurate during pod resizing; to get the status of a pod resizing process, use [`describe_index`](https://www.pinecone.io/docs/api/operation/describe_index/).
	DescribeIndexStats(context.Context, *DescribeIndexStatsRequest) (*DescribeIndexStatsResponse, error)
	mustEmbedUnimplementedVectorServiceServer()
}

// UnimplementedVectorServiceServer must be embedded to have forward compatible implementations.
type UnimplementedVectorServiceServer struct {
}

func (UnimplementedVectorServiceServer) Upsert(context.Context, *UpsertRequest) (*UpsertResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Upsert not implemented")
}
func (UnimplementedVectorServiceServer) Delete(context.Context, *DeleteRequest) (*DeleteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedVectorServiceServer) Fetch(context.Context, *FetchRequest) (*FetchResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Fetch not implemented")
}
func (UnimplementedVectorServiceServer) List(context.Context, *ListRequest) (*ListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedVectorServiceServer) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedVectorServiceServer) Update(context.Context, *UpdateRequest) (*UpdateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedVectorServiceServer) DescribeIndexStats(context.Context, *DescribeIndexStatsRequest) (*DescribeIndexStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DescribeIndexStats not implemented")
}
func (UnimplementedVectorServiceServer) mustEmbedUnimplementedVectorServiceServer() {}

// UnsafeVectorServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to VectorServiceServer will
// result in compilation errors.
type UnsafeVectorServiceServer interface {
	mustEmbedUnimplementedVectorServiceServer()
}

func RegisterVectorServiceServer(s grpc.ServiceRegistrar, srv VectorServiceServer) {
	s.RegisterService(&VectorService_ServiceDesc, srv)
}

func _VectorService_Upsert_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpsertRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).Upsert(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_Upsert_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).Upsert(ctx, req.(*UpsertRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VectorService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).Delete(ctx, req.(*DeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VectorService_Fetch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FetchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).Fetch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_Fetch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).Fetch(ctx, req.(*FetchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VectorService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).List(ctx, req.(*ListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VectorService_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).Query(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_Query_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).Query(ctx, req.(*QueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VectorService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_Update_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).Update(ctx, req.(*UpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VectorService_DescribeIndexStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DescribeIndexStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VectorServiceServer).DescribeIndexStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: VectorService_DescribeIndexStats_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).DescribeIndexStats(ctx, req.(*DescribeIndexStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// VectorService_ServiceDesc is the grpc.ServiceDesc for VectorService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var VectorService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "VectorService",
	HandlerType: (*VectorServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Upsert",
			Handler:    _VectorService_Upsert_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _VectorService_Delete_Handler,
		},
		{
			MethodName: "Fetch",
			Handler:    _VectorService_Fetch_Handler,
		},
		{
			MethodName: "List",
			Handler:    _VectorService_List_Handler,
		},
		{
			MethodName: "Query",
			Handler:    _VectorService_Query_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _VectorService_Update_Handler,
		},
		{
			MethodName: "DescribeIndexStats",
			Handler:    _VectorService_DescribeIndexStats_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pinecone/data/v1/vector_service.proto",
}
