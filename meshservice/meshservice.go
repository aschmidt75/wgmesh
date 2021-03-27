package meshservice

import (
	"encoding/base64"
	"fmt"
	"net"
	"time"

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

	// If set, this bootstrap will assign IP addresses from
	// this range only.
	CIDRRangeIPAM *net.IPNet

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

	// (optional) TLS config struct for gRPC Mesh service
	TLSConfig *TLSConfig

	// Serf
	cfg               *serf.Config
	s                 *serf.Serf
	serfEncryptionKey []byte

	// if set, exports the serf member list to this file
	memberExportFile string

	// timestamp of latest update to the member state
	lastUpdatedTS  time.Time
	lastExportedTS time.Time

	// gRPC
	UnimplementedMeshServer
	grpcServer *grpc.Server

	// Local agent gRPC server
	MeshAgentServer *MeshAgentServer

	// when the first bootstrap node started this mesh
	creationTS time.Time

	// when this node joined the mesh
	joinTS time.Time

	//
	rttResponseChan *chan RTTResponse

	//
	serfEventNotifierMap map[string]SerfEventChan
}

const (
	nodeTagPort     = "_port"
	nodeTagAddr     = "_addr"
	nodeTagPubKey   = "_pk"
	nodeTagMeshIP   = "_i"
	nodeTagNodeType = "_t"

	serfEventMarkerJoin   = "_j"
	serfEventMarkerRTTReq = "_rtt0"
	serfEventMarkerRTTRes = "_rtt1"
)

// SerfEventChan is a pointer to a channel of serf events,
// so that events can be forwarded to other listeners
type SerfEventChan *chan serf.Event

// RegisterEventNotifier registers an channel
func (ms *MeshService) RegisterEventNotifier(key string, sec SerfEventChan) {
	ms.serfEventNotifierMap[key] = sec
}

// DeregisterEventNotifier registers an channel
func (ms *MeshService) DeregisterEventNotifier(key string) {
	delete(ms.serfEventNotifierMap, key)
}

// NewMeshService creates a new MeshService for a node
func NewMeshService(meshName string) MeshService {
	return MeshService{
		MeshName:             meshName,
		creationTS:           time.Now(),
		serfEventNotifierMap: make(map[string]SerfEventChan),
		serfEncryptionKey:    make([]byte, 0),
	}
}

// SetNodeName applies a name to this node
func (ms *MeshService) SetNodeName(name string) {
	if name != "" {
		ms.NodeName = name
		return
	}

	// if no name is given, derive one from the
	// IPv4 address
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

// SetTimestamps sets the creation and join timestamp after grpc join call
func (ms *MeshService) SetTimestamps(creationTS, joinTS int64) {
	ms.creationTS = time.Unix(creationTS, 0)
	ms.joinTS = time.Unix(joinTS, 0)
}

// GetTimestamps returns the creation and join timestamp
func (ms *MeshService) GetTimestamps() (time.Time, time.Time) {
	return ms.creationTS, ms.joinTS
}

// SetEncryptionKey sets serf encryption key from a base64 string
func (ms *MeshService) SetEncryptionKey(encKeyB64 string) error {
	var err error
	ms.serfEncryptionKey, err = base64.StdEncoding.DecodeString(encKeyB64)
	if err != nil {
		return err
	}
	return nil
}

// GetEncryptionKey returns serf encryption key from a base64 string
func (ms *MeshService) GetEncryptionKey() string {
	return base64.StdEncoding.EncodeToString(ms.serfEncryptionKey)
}

// Serf returns the serf instance
func (ms *MeshService) Serf() *serf.Serf {
	return ms.s
}
