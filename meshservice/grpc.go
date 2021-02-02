package meshservice

import (
	context "context"
	"errors"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
)

// BeginJoin allows other nodes to join by sending a JoinRequest
func (ms *MeshService) BeginJoin(ctx context.Context, req *JoinRequest) (*JoinResponse, error) {

	log.WithField("req", req).Trace("Got join request")

	return &JoinResponse{
		Pubkey: "123",
	}, nil
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

	log.Info("Stopping gRPC mesh service")
	ms.grpcServer.GracefulStop()
	log.Info("Stopped gRPC mesh service")
}
