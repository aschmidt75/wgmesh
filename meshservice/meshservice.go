package meshservice

import (
	"net"

	serf "github.com/hashicorp/serf/serf"
	grpc "google.golang.org/grpc"
)

// MeshService collects all information about running a mesh node
// for both bootstrap and join modes.
type MeshService struct {
	// Name of the mesh network.
	MeshName string

	// Name of this node
	NodeName string

	// eg. 10.232.0.0/16. All nodes in the mesh will have an
	// IP address within this range
	CIDRRange net.IPNet

	// Local mesh IP of this node
	MeshIP net.IP

	// Listen port for Wireguard
	WireguardListenPort int

	// Listen IP for Wireguard
	WireguardListenIP net.IP

	// Own public key
	WireguardPubKey string

	// Bind Address for gRPC Mesh service
	GrpcBindAddr string

	// Bind port for gRPC Mesh service
	GrpcBindPort int

	// Serf
	cfg *serf.Config
	s   *serf.Serf

	// gRPC
	UnimplementedMeshServer
	grpcServer *grpc.Server
}

// NewMeshService creates a new MeshService for a node
func NewMeshService(meshName string) MeshService {
	return MeshService{
		MeshName: meshName,
	}
}
