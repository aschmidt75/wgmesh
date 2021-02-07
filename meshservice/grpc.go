package meshservice

import (
	context "context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// Join allows other nodes to join by sending a JoinRequest
func (ms *MeshService) Join(ctx context.Context, req *JoinRequest) (*JoinResponse, error) {

	log.WithField("req", req).Trace("Got join request")

	// choose a random ip adress from the adress pool of this node
	// which has not been used before
	var mip net.IP
	for {
		mip, _ = newIPInNet(ms.CIDRRange)

		if ms.isIPAvailable(mip) {
			break
		}
	}

	targetWGIP := net.IPNet{
		mip,
		net.CIDRMask(32, 32),
	}

	// take public key and endpoint, add as peer to own wireguard interface
	p := wgwrapper.WireguardPeer{
		RemoteEndpointIP: req.EndpointIP,
		ListenPort:       int(req.EndpointPort),
		Pubkey:           req.Pubkey,
		AllowedIPs: []net.IPNet{
			targetWGIP,
		},
		Psk: nil,
	}
	log.WithField("peer", p).Trace("Adding peer")

	wg := wgwrapper.New()

	ok, err := wg.AddPeer(ms.WireguardInterface, p)
	if err != nil {
		return &JoinResponse{
			Result:       JoinResponse_ERROR,
			ErrorMessage: "Unable to add peer",
			JoinerMeshIP: "",
		}, nil
	}
	if !ok && err == nil {
		return &JoinResponse{
			Result:       JoinResponse_ERROR,
			ErrorMessage: "Peer already present",
			JoinerMeshIP: "",
		}, nil
	}

	// send out a Peer Update as message to all serf nodes
	peerAnnouncementBuf, _ := proto.Marshal(&Peer{
		Type:         Peer_JOIN,
		Pubkey:       req.Pubkey,
		EndpointIP:   req.EndpointIP,
		EndpointPort: int32(req.EndpointPort),
		MeshIP:       targetWGIP.IP.String(),
	})
	log.WithField("len", len(peerAnnouncementBuf)).Trace("peerAnnouncement protobuf len")

	//
	ms.s.UserEvent("j", []byte(peerAnnouncementBuf), true)

	// return successful join response to client
	return &JoinResponse{
		Result:       JoinResponse_OK,
		ErrorMessage: "",
		JoinerMeshIP: mip.String(),
		MeshCidr:     ms.CIDRRange.String(),
	}, nil
}

// Peers serves a list of all current peers, starting with this node.
// All data is derived from serf's memberlist
func (ms *MeshService) Peers(e *Empty, stream Mesh_PeersServer) error {
	for _, member := range ms.s.Members() {
		t := member.Tags
		port, _ := strconv.Atoi(t["port"])
		err := stream.Send(&Peer{
			Pubkey:       t["pk"],
			EndpointIP:   t["addr"],
			EndpointPort: int32(port),
			MeshIP:       t["i"],
			Type:         Peer_JOIN,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func newIPInNet(ipnet net.IPNet) (net.IP, error) {

	ipmask := ipnet.Mask
	log.WithField("ipmask", ipmask).Trace("dump")
	log.WithField("ip", ipnet.IP).Trace("dump")

	var newIP [4]byte
	if len(ipnet.IP) == 4 {
		newIP = [4]byte{
			(byte(rand.Intn(250)+2) & ^ipmask[0]) + ipnet.IP[0],
			(byte(rand.Intn(250)) & ^ipmask[1]) + ipnet.IP[1],
			(byte(rand.Intn(250)) & ^ipmask[2]) + ipnet.IP[2],
			(byte(rand.Intn(250)+1) & ^ipmask[3]) + ipnet.IP[3],
		}
	}
	if len(ipnet.IP) == 16 {
		newIP = [4]byte{
			(byte(rand.Intn(250)+2) & ^ipmask[0]) + ipnet.IP[12],
			(byte(rand.Intn(250)) & ^ipmask[1]) + ipnet.IP[13],
			(byte(rand.Intn(250)) & ^ipmask[2]) + ipnet.IP[14],
			(byte(rand.Intn(250)+1) & ^ipmask[3]) + ipnet.IP[15],
		}
	}
	log.WithField("newIP", newIP).Trace("newIPInNet.dump")

	return net.IPv4(newIP[0], newIP[1], newIP[2], newIP[3]), nil
}

// StartGrpcService ..
func (ms *MeshService) StartGrpcService() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ms.GrpcBindAddr, ms.GrpcBindPort))
	if err != nil {
		log.Errorf("failed to listen: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	ms.grpcServer = grpc.NewServer()
	RegisterMeshServer(ms.grpcServer, ms)
	if err := ms.grpcServer.Serve(lis); err != nil {
		log.Errorf("failed to serve: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	return nil
}

// StopGrpcService ...
func (ms *MeshService) StopGrpcService() {

	log.Debug("Stopping gRPC mesh service")
	ms.grpcServer.GracefulStop()
	log.Info("Stopped gRPC mesh service")
}

func (ms *MeshService) isIPAvailable(ip net.IP) bool {
	s := ip.String()

	for _, member := range ms.s.Members() {
		wgIP := member.Tags["addr"]
		if wgIP == s {
			return false
		}
	}

	return true
}
