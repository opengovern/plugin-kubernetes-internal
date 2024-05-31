// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: plugin/proto/server.proto

package golang

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

const (
	Optimization_KubernetesPodOptimization_FullMethodName = "/pluginkubernetes.optimization.v1.Optimization/KubernetesPodOptimization"
)

// OptimizationClient is the client API for Optimization service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OptimizationClient interface {
	KubernetesPodOptimization(ctx context.Context, opts ...grpc.CallOption) (Optimization_KubernetesPodOptimizationClient, error)
}

type optimizationClient struct {
	cc grpc.ClientConnInterface
}

func NewOptimizationClient(cc grpc.ClientConnInterface) OptimizationClient {
	return &optimizationClient{cc}
}

func (c *optimizationClient) KubernetesPodOptimization(ctx context.Context, opts ...grpc.CallOption) (Optimization_KubernetesPodOptimizationClient, error) {
	stream, err := c.cc.NewStream(ctx, &Optimization_ServiceDesc.Streams[0], Optimization_KubernetesPodOptimization_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &optimizationKubernetesPodOptimizationClient{stream}
	return x, nil
}

type Optimization_KubernetesPodOptimizationClient interface {
	Send(*KubernetesPodOptimizationRequest) error
	Recv() (*KubernetesPodOptimizationResponse, error)
	grpc.ClientStream
}

type optimizationKubernetesPodOptimizationClient struct {
	grpc.ClientStream
}

func (x *optimizationKubernetesPodOptimizationClient) Send(m *KubernetesPodOptimizationRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *optimizationKubernetesPodOptimizationClient) Recv() (*KubernetesPodOptimizationResponse, error) {
	m := new(KubernetesPodOptimizationResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// OptimizationServer is the server API for Optimization service.
// All implementations must embed UnimplementedOptimizationServer
// for forward compatibility
type OptimizationServer interface {
	KubernetesPodOptimization(Optimization_KubernetesPodOptimizationServer) error
	mustEmbedUnimplementedOptimizationServer()
}

// UnimplementedOptimizationServer must be embedded to have forward compatible implementations.
type UnimplementedOptimizationServer struct {
}

func (UnimplementedOptimizationServer) KubernetesPodOptimization(Optimization_KubernetesPodOptimizationServer) error {
	return status.Errorf(codes.Unimplemented, "method KubernetesPodOptimization not implemented")
}
func (UnimplementedOptimizationServer) mustEmbedUnimplementedOptimizationServer() {}

// UnsafeOptimizationServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OptimizationServer will
// result in compilation errors.
type UnsafeOptimizationServer interface {
	mustEmbedUnimplementedOptimizationServer()
}

func RegisterOptimizationServer(s grpc.ServiceRegistrar, srv OptimizationServer) {
	s.RegisterService(&Optimization_ServiceDesc, srv)
}

func _Optimization_KubernetesPodOptimization_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(OptimizationServer).KubernetesPodOptimization(&optimizationKubernetesPodOptimizationServer{stream})
}

type Optimization_KubernetesPodOptimizationServer interface {
	Send(*KubernetesPodOptimizationResponse) error
	Recv() (*KubernetesPodOptimizationRequest, error)
	grpc.ServerStream
}

type optimizationKubernetesPodOptimizationServer struct {
	grpc.ServerStream
}

func (x *optimizationKubernetesPodOptimizationServer) Send(m *KubernetesPodOptimizationResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *optimizationKubernetesPodOptimizationServer) Recv() (*KubernetesPodOptimizationRequest, error) {
	m := new(KubernetesPodOptimizationRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Optimization_ServiceDesc is the grpc.ServiceDesc for Optimization service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Optimization_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "pluginkubernetes.optimization.v1.Optimization",
	HandlerType: (*OptimizationServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "KubernetesPodOptimization",
			Handler:       _Optimization_KubernetesPodOptimization_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "plugin/proto/server.proto",
}
