
Copyright (c) 2020-2021 Pinecone Systems Inc. All right reserved.


// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package pinecone

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// VectorServiceClient is the client API for VectorService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type VectorServiceClient interface {
	// Upsert
	//
	// The `Upsert` operation writes vectors into a namespace.
	// If a new value is upserted for an existing vector id, it will overwrite the previous value.
	Upsert(ctx context.Context, in *UpsertRequest, opts ...grpc.CallOption) (*UpsertResponse, error)
	// Delete
	//
	// The `Delete` operation deletes vectors, by id, from a single namespace.
	// You can delete items by their id, from a single namespace.
	Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error)
	// Fetch
	//
	// The `Fetch` operation looks up and returns vectors, by id, from a single namespace.
	// The returned vectors include the vector data and/or metadata.
	Fetch(ctx context.Context, in *FetchRequest, opts ...grpc.CallOption) (*FetchResponse, error)
	// Query
	//
	// The `Query` operation searches a namespace, using a query vector.
	// It retrieves the ids of the most similar items in a namespace, along with their similarity scores.
	Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error)
	// Update
	//
	// The `Update` operation updates vector in a namespace.
	// If a value is included, it will overwrite the previous value.
	// If a set_metadata is included, the values of the fields specified in it will be added or overwrite the previous value.
	Update(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (*UpdateResponse, error)
	// DescribeIndexStats
	//
	// The `DescribeIndexStats` operation returns statistics about the index's contents.
	// For example: The vector count per namespace and the number of dimensions.
	DescribeIndexStats(ctx context.Context, in *DescribeIndexStatsRequest, opts ...grpc.CallOption) (*DescribeIndexStatsResponse, error)
}

type vectorServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewVectorServiceClient(cc grpc.ClientConnInterface) VectorServiceClient {
	return &vectorServiceClient{cc}
}

func (c *vectorServiceClient) Upsert(ctx context.Context, in *UpsertRequest, opts ...grpc.CallOption) (*UpsertResponse, error) {
	out := new(UpsertResponse)
	err := c.cc.Invoke(ctx, "/VectorService/Upsert", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error) {
	out := new(DeleteResponse)
	err := c.cc.Invoke(ctx, "/VectorService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Fetch(ctx context.Context, in *FetchRequest, opts ...grpc.CallOption) (*FetchResponse, error) {
	out := new(FetchResponse)
	err := c.cc.Invoke(ctx, "/VectorService/Fetch", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error) {
	out := new(QueryResponse)
	err := c.cc.Invoke(ctx, "/VectorService/Query", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) Update(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (*UpdateResponse, error) {
	out := new(UpdateResponse)
	err := c.cc.Invoke(ctx, "/VectorService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vectorServiceClient) DescribeIndexStats(ctx context.Context, in *DescribeIndexStatsRequest, opts ...grpc.CallOption) (*DescribeIndexStatsResponse, error) {
	out := new(DescribeIndexStatsResponse)
	err := c.cc.Invoke(ctx, "/VectorService/DescribeIndexStats", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VectorServiceServer is the server API for VectorService service.
// All implementations must embed UnimplementedVectorServiceServer
// for forward compatibility
type VectorServiceServer interface {
	// Upsert
	//
	// The `Upsert` operation writes vectors into a namespace.
	// If a new value is upserted for an existing vector id, it will overwrite the previous value.
	Upsert(context.Context, *UpsertRequest) (*UpsertResponse, error)
	// Delete
	//
	// The `Delete` operation deletes vectors, by id, from a single namespace.
	// You can delete items by their id, from a single namespace.
	Delete(context.Context, *DeleteRequest) (*DeleteResponse, error)
	// Fetch
	//
	// The `Fetch` operation looks up and returns vectors, by id, from a single namespace.
	// The returned vectors include the vector data and/or metadata.
	Fetch(context.Context, *FetchRequest) (*FetchResponse, error)
	// Query
	//
	// The `Query` operation searches a namespace, using a query vector.
	// It retrieves the ids of the most similar items in a namespace, along with their similarity scores.
	Query(context.Context, *QueryRequest) (*QueryResponse, error)
	// Update
	//
	// The `Update` operation updates vector in a namespace.
	// If a value is included, it will overwrite the previous value.
	// If a set_metadata is included, the values of the fields specified in it will be added or overwrite the previous value.
	Update(context.Context, *UpdateRequest) (*UpdateResponse, error)
	// DescribeIndexStats
	//
	// The `DescribeIndexStats` operation returns statistics about the index's contents.
	// For example: The vector count per namespace and the number of dimensions.
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
		FullMethod: "/VectorService/Upsert",
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
		FullMethod: "/VectorService/Delete",
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
		FullMethod: "/VectorService/Fetch",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VectorServiceServer).Fetch(ctx, req.(*FetchRequest))
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
		FullMethod: "/VectorService/Query",
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
		FullMethod: "/VectorService/Update",
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
		FullMethod: "/VectorService/DescribeIndexStats",
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
	Metadata: "vector_service.proto",
}
