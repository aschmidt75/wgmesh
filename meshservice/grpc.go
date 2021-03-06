package meshservice

import (
	context "context"
	"crypto/tls"
	"errors"
	"math/rand"
	"net"
	"strconv"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
)

// Join allows other nodes to join by sending a JoinRequest
func (ms *MeshService) Join(ctx context.Context, req *JoinRequest) (*JoinResponse, error) {

	log.WithField("req", req).Trace("Got join request")

	if req.MeshName != ms.MeshName {
		return &JoinResponse{
			Result:            JoinResponse_ERROR,
			ErrorMessage:      "Unknown mesh",
			JoiningNodeMeshIP: "",
		}, nil
	}

	// choose a random ip address from the address pool of this node
	// which has not been used before
	var mip net.IP
	for {
		mip, _ = newIPInNet(ms.CIDRRange)

		if ms.isIPAvailable(mip) {
			break
		}
	}

	// TODO: check if joining node wishes to have a explicit node name
	// if so, check if this name is already in use.
	if ms.isNodeNameInUse(req.NodeName) {
		return &JoinResponse{
			Result:            JoinResponse_ERROR,
			ErrorMessage:      "Request node name is already in use",
			JoiningNodeMeshIP: "",
		}, nil
	}

	//
	targetWGIP := net.IPNet{
		mip,
		net.CIDRMask(32, 32),
	}

	/*
		keepAliveSeconds := 0
		if req.Nat {
			keepAliveSeconds = 20
		}
	*/
	// take public key and endpoint, add as peer to own wireguard interface
	p := wgwrapper.WireguardPeer{
		RemoteEndpointIP: req.EndpointIP,
		ListenPort:       int(req.EndpointPort),
		Pubkey:           req.Pubkey,
		AllowedIPs: []net.IPNet{
			targetWGIP,
		},
		Psk: nil,
		//PersistentKeepaliveInterval: time.Duration(keepAliveSeconds) * time.Second,
	}
	log.WithField("peer", p).Trace("Adding peer")

	wg := wgwrapper.New()

	ok, err := wg.AddPeer(ms.WireguardInterface, p)
	if err != nil {
		log.Error(err)
		return &JoinResponse{
			Result:            JoinResponse_ERROR,
			ErrorMessage:      "Unable to add peer",
			JoiningNodeMeshIP: "",
		}, nil
	}
	if !ok && err == nil {
		return &JoinResponse{
			Result:            JoinResponse_ERROR,
			ErrorMessage:      "Peer already present",
			JoiningNodeMeshIP: "",
		}, nil
	}

	log.WithFields(log.Fields{
		"ip": mip.String(),
	}).Info("node joined mesh")
	log.WithFields(log.Fields{
		"ip": mip.String(),
		"pk": req.Pubkey,
	}).Debug("node joined mesh")

	// send out a Peer Update as message to all serf nodes
	peerAnnouncementBuf, _ := proto.Marshal(&Peer{
		Type:         Peer_JOIN,
		Pubkey:       req.Pubkey,
		EndpointIP:   req.EndpointIP,
		EndpointPort: int32(req.EndpointPort),
		MeshIP:       targetWGIP.IP.String(),
	})
	// send out a join request event
	ms.Serf().UserEvent(serfEventMarkerJoin, []byte(peerAnnouncementBuf), true)

	// return successful join response to client
	return &JoinResponse{
		Result:            JoinResponse_OK,
		ErrorMessage:      "",
		JoiningNodeMeshIP: mip.String(),
		MeshCidr:          ms.CIDRRange.String(),
		CreationTS:        int64(ms.creationTS.Unix()),
		SerfEncryptionKey: ms.GetEncryptionKey(),
	}, nil
}

// Peers serves a list of all current peers, starting with this node.
// All data is derived from serf's memberlist
func (ms *MeshService) Peers(e *Empty, stream Mesh_PeersServer) error {
	for _, member := range ms.Serf().Members() {
		t := member.Tags

		//log.WithField("t", t).Trace("Peers: sending member tags")

		port, _ := strconv.Atoi(t[nodeTagPort])
		err := stream.Send(&Peer{
			Pubkey:       t[nodeTagPubKey],
			EndpointIP:   t[nodeTagAddr],
			EndpointPort: int32(port),
			MeshIP:       t[nodeTagMeshIP],
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

func (ms *MeshService) newTLSCredentials() credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		//ServerName: serverNameOverride,
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{ms.TLSConfig.Cert},
		ClientCAs:          ms.TLSConfig.CertPool,
	})
}

// StartGrpcService ..
func (ms *MeshService) StartGrpcService() error {
	lis, err := net.Listen("tcp", net.JoinHostPort(ms.GrpcBindAddr, strconv.Itoa(ms.GrpcBindPort)))
	if err != nil {
		log.Errorf("failed to listen: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	if ms.TLSConfig != nil {
		log.Debug("Starting TLS gRPC mesh service")
		ms.grpcServer = grpc.NewServer(grpc.Creds(ms.newTLSCredentials()))
	} else {
		log.Warn("Starting an insecure gRPC mesh service")
		ms.grpcServer = grpc.NewServer()
	}
	RegisterMeshServer(ms.grpcServer, ms)
	if err := ms.grpcServer.Serve(lis); err != nil {
		log.Errorf("failed to serve: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	return nil
}

// StopGrpcService stops the grpc server
func (ms *MeshService) StopGrpcService() {

	log.Debug("Stopping gRPC mesh service")
	ms.grpcServer.GracefulStop()
	log.Info("Stopped gRPC mesh service")
}

func (ms *MeshService) isIPAvailable(ip net.IP) bool {
	s := ip.String()

	for _, member := range ms.Serf().Members() {
		wgIP := member.Tags[nodeTagAddr]
		if wgIP == s {
			return false
		}
	}

	return true
}
