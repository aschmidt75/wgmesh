// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package meshservice

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

// AgentClient is the client API for Agent service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AgentClient interface {
	// Info returns a summary about the running mesh
	Info(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (*MeshInfo, error)
	// Nodes streams the current list of nodes known to the mesh
	Nodes(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (Agent_NodesClient, error)
	// This methods blocks until a change in the mesh setup
	// has occured
	WaitForChangeInMesh(ctx context.Context, in *WaitInfo, opts ...grpc.CallOption) (Agent_WaitForChangeInMeshClient, error)
	// Tag sets a tag on a wgmesh node
	Tag(ctx context.Context, in *NodeTag, opts ...grpc.CallOption) (*TagResult, error)
	// Untag remove a tag on a wgmesh node
	Untag(ctx context.Context, in *NodeTag, opts ...grpc.CallOption) (*TagResult, error)
	// Tags streams all tags of the local node
	Tags(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (Agent_TagsClient, error)
	// RTT yields the complete rtt timings for all nodes
	RTT(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (Agent_RTTClient, error)
}

type agentClient struct {
	cc grpc.ClientConnInterface
}

func NewAgentClient(cc grpc.ClientConnInterface) AgentClient {
	return &agentClient{cc}
}

func (c *agentClient) Info(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (*MeshInfo, error) {
	out := new(MeshInfo)
	err := c.cc.Invoke(ctx, "/meshservice.Agent/Info", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentClient) Nodes(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (Agent_NodesClient, error) {
	stream, err := c.cc.NewStream(ctx, &Agent_ServiceDesc.Streams[0], "/meshservice.Agent/Nodes", opts...)
	if err != nil {
		return nil, err
	}
	x := &agentNodesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Agent_NodesClient interface {
	Recv() (*MemberInfo, error)
	grpc.ClientStream
}

type agentNodesClient struct {
	grpc.ClientStream
}

func (x *agentNodesClient) Recv() (*MemberInfo, error) {
	m := new(MemberInfo)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *agentClient) WaitForChangeInMesh(ctx context.Context, in *WaitInfo, opts ...grpc.CallOption) (Agent_WaitForChangeInMeshClient, error) {
	stream, err := c.cc.NewStream(ctx, &Agent_ServiceDesc.Streams[1], "/meshservice.Agent/WaitForChangeInMesh", opts...)
	if err != nil {
		return nil, err
	}
	x := &agentWaitForChangeInMeshClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Agent_WaitForChangeInMeshClient interface {
	Recv() (*WaitResponse, error)
	grpc.ClientStream
}

type agentWaitForChangeInMeshClient struct {
	grpc.ClientStream
}

func (x *agentWaitForChangeInMeshClient) Recv() (*WaitResponse, error) {
	m := new(WaitResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *agentClient) Tag(ctx context.Context, in *NodeTag, opts ...grpc.CallOption) (*TagResult, error) {
	out := new(TagResult)
	err := c.cc.Invoke(ctx, "/meshservice.Agent/Tag", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentClient) Untag(ctx context.Context, in *NodeTag, opts ...grpc.CallOption) (*TagResult, error) {
	out := new(TagResult)
	err := c.cc.Invoke(ctx, "/meshservice.Agent/Untag", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentClient) Tags(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (Agent_TagsClient, error) {
	stream, err := c.cc.NewStream(ctx, &Agent_ServiceDesc.Streams[2], "/meshservice.Agent/Tags", opts...)
	if err != nil {
		return nil, err
	}
	x := &agentTagsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Agent_TagsClient interface {
	Recv() (*NodeTag, error)
	grpc.ClientStream
}

type agentTagsClient struct {
	grpc.ClientStream
}

func (x *agentTagsClient) Recv() (*NodeTag, error) {
	m := new(NodeTag)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *agentClient) RTT(ctx context.Context, in *AgentEmpty, opts ...grpc.CallOption) (Agent_RTTClient, error) {
	stream, err := c.cc.NewStream(ctx, &Agent_ServiceDesc.Streams[3], "/meshservice.Agent/RTT", opts...)
	if err != nil {
		return nil, err
	}
	x := &agentRTTClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Agent_RTTClient interface {
	Recv() (*RTTInfo, error)
	grpc.ClientStream
}

type agentRTTClient struct {
	grpc.ClientStream
}

func (x *agentRTTClient) Recv() (*RTTInfo, error) {
	m := new(RTTInfo)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AgentServer is the server API for Agent service.
// All implementations must embed UnimplementedAgentServer
// for forward compatibility
type AgentServer interface {
	// Info returns a summary about the running mesh
	Info(context.Context, *AgentEmpty) (*MeshInfo, error)
	// Nodes streams the current list of nodes known to the mesh
	Nodes(*AgentEmpty, Agent_NodesServer) error
	// This methods blocks until a change in the mesh setup
	// has occured
	WaitForChangeInMesh(*WaitInfo, Agent_WaitForChangeInMeshServer) error
	// Tag sets a tag on a wgmesh node
	Tag(context.Context, *NodeTag) (*TagResult, error)
	// Untag remove a tag on a wgmesh node
	Untag(context.Context, *NodeTag) (*TagResult, error)
	// Tags streams all tags of the local node
	Tags(*AgentEmpty, Agent_TagsServer) error
	// RTT yields the complete rtt timings for all nodes
	RTT(*AgentEmpty, Agent_RTTServer) error
	mustEmbedUnimplementedAgentServer()
}

// UnimplementedAgentServer must be embedded to have forward compatible implementations.
type UnimplementedAgentServer struct {
}

func (UnimplementedAgentServer) Info(context.Context, *AgentEmpty) (*MeshInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Info not implemented")
}
func (UnimplementedAgentServer) Nodes(*AgentEmpty, Agent_NodesServer) error {
	return status.Errorf(codes.Unimplemented, "method Nodes not implemented")
}
func (UnimplementedAgentServer) WaitForChangeInMesh(*WaitInfo, Agent_WaitForChangeInMeshServer) error {
	return status.Errorf(codes.Unimplemented, "method WaitForChangeInMesh not implemented")
}
func (UnimplementedAgentServer) Tag(context.Context, *NodeTag) (*TagResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Tag not implemented")
}
func (UnimplementedAgentServer) Untag(context.Context, *NodeTag) (*TagResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Untag not implemented")
}
func (UnimplementedAgentServer) Tags(*AgentEmpty, Agent_TagsServer) error {
	return status.Errorf(codes.Unimplemented, "method Tags not implemented")
}
func (UnimplementedAgentServer) RTT(*AgentEmpty, Agent_RTTServer) error {
	return status.Errorf(codes.Unimplemented, "method RTT not implemented")
}
func (UnimplementedAgentServer) mustEmbedUnimplementedAgentServer() {}

// UnsafeAgentServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AgentServer will
// result in compilation errors.
type UnsafeAgentServer interface {
	mustEmbedUnimplementedAgentServer()
}

func RegisterAgentServer(s grpc.ServiceRegistrar, srv AgentServer) {
	s.RegisterService(&Agent_ServiceDesc, srv)
}

func _Agent_Info_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AgentEmpty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServer).Info(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/meshservice.Agent/Info",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServer).Info(ctx, req.(*AgentEmpty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Agent_Nodes_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(AgentEmpty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AgentServer).Nodes(m, &agentNodesServer{stream})
}

type Agent_NodesServer interface {
	Send(*MemberInfo) error
	grpc.ServerStream
}

type agentNodesServer struct {
	grpc.ServerStream
}

func (x *agentNodesServer) Send(m *MemberInfo) error {
	return x.ServerStream.SendMsg(m)
}

func _Agent_WaitForChangeInMesh_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(WaitInfo)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AgentServer).WaitForChangeInMesh(m, &agentWaitForChangeInMeshServer{stream})
}

type Agent_WaitForChangeInMeshServer interface {
	Send(*WaitResponse) error
	grpc.ServerStream
}

type agentWaitForChangeInMeshServer struct {
	grpc.ServerStream
}

func (x *agentWaitForChangeInMeshServer) Send(m *WaitResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _Agent_Tag_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeTag)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServer).Tag(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/meshservice.Agent/Tag",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServer).Tag(ctx, req.(*NodeTag))
	}
	return interceptor(ctx, in, info, handler)
}

func _Agent_Untag_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeTag)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServer).Untag(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/meshservice.Agent/Untag",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServer).Untag(ctx, req.(*NodeTag))
	}
	return interceptor(ctx, in, info, handler)
}

func _Agent_Tags_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(AgentEmpty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AgentServer).Tags(m, &agentTagsServer{stream})
}

type Agent_TagsServer interface {
	Send(*NodeTag) error
	grpc.ServerStream
}

type agentTagsServer struct {
	grpc.ServerStream
}

func (x *agentTagsServer) Send(m *NodeTag) error {
	return x.ServerStream.SendMsg(m)
}

func _Agent_RTT_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(AgentEmpty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AgentServer).RTT(m, &agentRTTServer{stream})
}

type Agent_RTTServer interface {
	Send(*RTTInfo) error
	grpc.ServerStream
}

type agentRTTServer struct {
	grpc.ServerStream
}

func (x *agentRTTServer) Send(m *RTTInfo) error {
	return x.ServerStream.SendMsg(m)
}

// Agent_ServiceDesc is the grpc.ServiceDesc for Agent service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Agent_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "meshservice.Agent",
	HandlerType: (*AgentServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Info",
			Handler:    _Agent_Info_Handler,
		},
		{
			MethodName: "Tag",
			Handler:    _Agent_Tag_Handler,
		},
		{
			MethodName: "Untag",
			Handler:    _Agent_Untag_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Nodes",
			Handler:       _Agent_Nodes_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "WaitForChangeInMesh",
			Handler:       _Agent_WaitForChangeInMesh_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Tags",
			Handler:       _Agent_Tags_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "RTT",
			Handler:       _Agent_RTT_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "agent.proto",
}
