// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.25.3
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

// OptimizationClient is the client API for Optimization service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OptimizationClient interface {
	KubernetesPodOptimization(ctx context.Context, in *KubernetesPodOptimizationRequest, opts ...grpc.CallOption) (*KubernetesPodOptimizationResponse, error)
	KubernetesDeploymentOptimization(ctx context.Context, in *KubernetesDeploymentOptimizationRequest, opts ...grpc.CallOption) (*KubernetesDeploymentOptimizationResponse, error)
	KubernetesStatefulsetOptimization(ctx context.Context, in *KubernetesStatefulsetOptimizationRequest, opts ...grpc.CallOption) (*KubernetesStatefulsetOptimizationResponse, error)
}

type optimizationClient struct {
	cc grpc.ClientConnInterface
}

func NewOptimizationClient(cc grpc.ClientConnInterface) OptimizationClient {
	return &optimizationClient{cc}
}

func (c *optimizationClient) KubernetesPodOptimization(ctx context.Context, in *KubernetesPodOptimizationRequest, opts ...grpc.CallOption) (*KubernetesPodOptimizationResponse, error) {
	out := new(KubernetesPodOptimizationResponse)
	err := c.cc.Invoke(ctx, "/pluginkubernetes.optimization.v1.Optimization/KubernetesPodOptimization", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optimizationClient) KubernetesDeploymentOptimization(ctx context.Context, in *KubernetesDeploymentOptimizationRequest, opts ...grpc.CallOption) (*KubernetesDeploymentOptimizationResponse, error) {
	out := new(KubernetesDeploymentOptimizationResponse)
	err := c.cc.Invoke(ctx, "/pluginkubernetes.optimization.v1.Optimization/KubernetesDeploymentOptimization", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optimizationClient) KubernetesStatefulsetOptimization(ctx context.Context, in *KubernetesStatefulsetOptimizationRequest, opts ...grpc.CallOption) (*KubernetesStatefulsetOptimizationResponse, error) {
	out := new(KubernetesStatefulsetOptimizationResponse)
	err := c.cc.Invoke(ctx, "/pluginkubernetes.optimization.v1.Optimization/KubernetesStatefulsetOptimization", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OptimizationServer is the server API for Optimization service.
// All implementations must embed UnimplementedOptimizationServer
// for forward compatibility
type OptimizationServer interface {
	KubernetesPodOptimization(context.Context, *KubernetesPodOptimizationRequest) (*KubernetesPodOptimizationResponse, error)
	KubernetesDeploymentOptimization(context.Context, *KubernetesDeploymentOptimizationRequest) (*KubernetesDeploymentOptimizationResponse, error)
	KubernetesStatefulsetOptimization(context.Context, *KubernetesStatefulsetOptimizationRequest) (*KubernetesStatefulsetOptimizationResponse, error)
	mustEmbedUnimplementedOptimizationServer()
}

// UnimplementedOptimizationServer must be embedded to have forward compatible implementations.
type UnimplementedOptimizationServer struct {
}

func (UnimplementedOptimizationServer) KubernetesPodOptimization(context.Context, *KubernetesPodOptimizationRequest) (*KubernetesPodOptimizationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method KubernetesPodOptimization not implemented")
}
func (UnimplementedOptimizationServer) KubernetesDeploymentOptimization(context.Context, *KubernetesDeploymentOptimizationRequest) (*KubernetesDeploymentOptimizationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method KubernetesDeploymentOptimization not implemented")
}
func (UnimplementedOptimizationServer) KubernetesStatefulsetOptimization(context.Context, *KubernetesStatefulsetOptimizationRequest) (*KubernetesStatefulsetOptimizationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method KubernetesStatefulsetOptimization not implemented")
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

func _Optimization_KubernetesPodOptimization_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KubernetesPodOptimizationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptimizationServer).KubernetesPodOptimization(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pluginkubernetes.optimization.v1.Optimization/KubernetesPodOptimization",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptimizationServer).KubernetesPodOptimization(ctx, req.(*KubernetesPodOptimizationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Optimization_KubernetesDeploymentOptimization_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KubernetesDeploymentOptimizationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptimizationServer).KubernetesDeploymentOptimization(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pluginkubernetes.optimization.v1.Optimization/KubernetesDeploymentOptimization",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptimizationServer).KubernetesDeploymentOptimization(ctx, req.(*KubernetesDeploymentOptimizationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Optimization_KubernetesStatefulsetOptimization_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KubernetesStatefulsetOptimizationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptimizationServer).KubernetesStatefulsetOptimization(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pluginkubernetes.optimization.v1.Optimization/KubernetesStatefulsetOptimization",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptimizationServer).KubernetesStatefulsetOptimization(ctx, req.(*KubernetesStatefulsetOptimizationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Optimization_ServiceDesc is the grpc.ServiceDesc for Optimization service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Optimization_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "pluginkubernetes.optimization.v1.Optimization",
	HandlerType: (*OptimizationServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "KubernetesPodOptimization",
			Handler:    _Optimization_KubernetesPodOptimization_Handler,
		},
		{
			MethodName: "KubernetesDeploymentOptimization",
			Handler:    _Optimization_KubernetesDeploymentOptimization_Handler,
		},
		{
			MethodName: "KubernetesStatefulsetOptimization",
			Handler:    _Optimization_KubernetesStatefulsetOptimization_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "plugin/proto/server.proto",
}
