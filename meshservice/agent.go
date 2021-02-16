package meshservice

import (
	context "context"
	"errors"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// MeshAgentServer
type MeshAgentServer struct {
	UnimplementedAgentServer
	grpcServer *grpc.Server

	grpcBindAddr string
	grpcBindPort int

	ms *MeshService
}

// NewMeshAgentServer creates a new agent service
func NewMeshAgentServer(ms *MeshService, grpcBindAddr string, grpcBindPort int) *MeshAgentServer {
	return &MeshAgentServer{
		grpcServer:   grpc.NewServer(),
		ms:           ms,
		grpcBindAddr: grpcBindAddr,
		grpcBindPort: grpcBindPort,
	}
}

// Tag ...
func (as *MeshAgentServer) Tag(ctx context.Context, tr *TagRequest) (*TagResult, error) {
	log.WithField("tr", *tr).Trace("Tag requested")

	t := as.ms.s.LocalMember().Tags
	t[tr.Key] = tr.Value
	err := as.ms.s.SetTags(t)
	if err != nil {
		log.WithError(err).Error("unable to set tags at serf node")
		return &TagResult{
			Ok: false,
		}, nil
	}
	return &TagResult{
		Ok: true,
	}, nil
}

// Untag ...
func (as *MeshAgentServer) Untag(ctx context.Context, tr *TagRequest) (*TagResult, error) {
	log.WithField("tr", *tr).Trace("Untag requested")

	t := as.ms.s.LocalMember().Tags

	if _, ex := t[tr.Key]; ex == false {
		return &TagResult{
			Ok: false,
		}, nil
	}

	delete(t, tr.Key)

	err := as.ms.s.SetTags(t)
	if err != nil {
		log.WithError(err).Error("unable to set tags at serf node")
		return &TagResult{
			Ok: false,
		}, nil
	}
	return &TagResult{
		Ok: true,
	}, nil
}

// RTT ...
func (as *MeshAgentServer) RTT(cte *AgentEmpty, rttServer Agent_RTTServer) error {
	log.Trace("RTT requested")

	ch := make(chan RTTResponse)
	doneCh := make(chan struct{})

	go func() {
		for {
			select {
			case rtt := <-ch:
				log.WithField("rtt", rtt).Trace("RTT")

				rtts := make([]*RTTNodeInfo, len(rtt.Rtts))
				for idx, rttResponseInfo := range rtt.Rtts {
					rtts[idx] = &RTTNodeInfo{
						NodeName: rttResponseInfo.Node,
						RttMsec:  rttResponseInfo.RttMsec,
					}
				}
				rttInfo := &RTTInfo{
					NodeName: rtt.Node,
					Rtts:     rtts,
				}
				if err := rttServer.Send(rttInfo); err != nil {
					log.WithError(err).Error("unable to stream send rtt info")
				}

			case <-doneCh:
				return
			}
		}
	}()
	as.ms.setRttResponseCh(&ch)

	// send a user event which makes all nodes report their rtts
	rttRequestBuf, _ := proto.Marshal(&RTTRequest{
		RequestedBy: as.ms.NodeName,
	})
	as.ms.s.UserEvent("rtt0", []byte(rttRequestBuf), true)

	// wait until all are collected and streamed out
	time.Sleep(time.Duration(as.ms.s.NumNodes()+1) * time.Second)

	// done
	doneCh <- struct{}{}

	return nil

}

// StartAgentGrpcService ..
func (as *MeshAgentServer) StartAgentGrpcService() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", as.grpcBindAddr, as.grpcBindPort))
	if err != nil {
		log.Errorf("failed to listen: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	RegisterAgentServer(as.grpcServer, as)

	if err := as.grpcServer.Serve(lis); err != nil {
		log.Errorf("failed to serve: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	return nil
}

// StopAgentGrpcService ...
func (as *MeshAgentServer) StopAgentGrpcService() {

	log.Debug("Stopping gRPC Agent service")
	as.grpcServer.GracefulStop()
	log.Info("Stopped gRPC Agent service")
}
