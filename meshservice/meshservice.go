package meshservice

import (
	"fmt"
	"net"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
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
	MeshIP net.IPNet

	// Listen port for Wireguard
	WireguardListenPort int

	// Listen IP for Wireguard
	WireguardListenIP net.IP

	// Own public key
	WireguardPubKey string

	// The interface we're controlling
	WireguardInterface wgwrapper.WireguardInterface

	// Bind Address for gRPC Mesh service
	GrpcBindAddr string

	// Bind port for gRPC Mesh service
	GrpcBindPort int

	// Serf
	cfg *serf.Config
	s   *serf.Serf

	// if set, exports the serf member list to this file
	memberExportFile string

	// gRPC
	UnimplementedMeshServer
	grpcServer *grpc.Server

	// Agent gRPC
	MeshAgentServer *MeshAgentServer

	//
	rttResponseChan *chan RTTResponse
}

// NewMeshService creates a new MeshService for a node
func NewMeshService(meshName string) MeshService {
	return MeshService{
		MeshName: meshName,
	}
}

// SetNodeName applies a name to this node
func (ms *MeshService) SetNodeName() {
	if len(ms.MeshIP.IP) == 16 {
		i := int(ms.MeshIP.IP[12]) * 16777216
		i += int(ms.MeshIP.IP[13]) * 65536
		i += int(ms.MeshIP.IP[14]) * 256
		i += int(ms.MeshIP.IP[15])
		ms.NodeName = fmt.Sprintf("%s%X", ms.MeshName, i)
	}
	if len(ms.MeshIP.IP) == 4 {
		i := int(ms.MeshIP.IP[0]) * 16777216
		i += int(ms.MeshIP.IP[1]) * 65536
		i += int(ms.MeshIP.IP[2]) * 256
		i += int(ms.MeshIP.IP[3])
		ms.NodeName = fmt.Sprintf("%s%X", ms.MeshName, i)
	}
}

func (ms *MeshService) setRttResponseCh(ch *chan RTTResponse) {
	ms.rttResponseChan = ch
}

func (ms *MeshService) releaseRttResponseCh() {
	ms.rttResponseChan = nil
}
