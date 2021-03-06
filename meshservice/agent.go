package meshservice

import (
	context "context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// MeshAgentServer implements the gRPC part of agent.proto
type MeshAgentServer struct {
	UnimplementedAgentServer
	grpcServer *grpc.Server

	grpcBindSocket   string
	grpcBindSocketID string

	ms *MeshService
}

// NewMeshAgentServerSocket creates a new agent service for a local bind socket
func NewMeshAgentServerSocket(ms *MeshService, grpcBindSocket string, grpcBindSocketID string) *MeshAgentServer {
	return &MeshAgentServer{
		grpcServer:       grpc.NewServer(),
		ms:               ms,
		grpcBindSocket:   grpcBindSocket,
		grpcBindSocketID: grpcBindSocketID,
	}
}

// Info returns details about the mesh
func (as *MeshAgentServer) Info(ctx context.Context, ae *AgentEmpty) (*MeshInfo, error) {
	log.Trace("agent: Info requested")

	creationTS, nodeJoinTS := as.ms.GetTimestamps()

	return &MeshInfo{
		Name:          as.ms.MeshName,
		NodeName:      as.ms.NodeName,
		NodeCount:     int32(as.ms.s.NumNodes()),
		MeshCeationTS: int64(creationTS.Unix()),
		NodeJoinTS:    int64(nodeJoinTS.Unix()),
	}, nil
}

// Tag ...
func (as *MeshAgentServer) Tag(ctx context.Context, tr *TagRequest) (*TagResult, error) {
	log.WithFields(log.Fields{
		"k": tr.Key,
		"v": tr.Value,
	}).Trace("agent: Tag requested")

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
	log.WithFields(log.Fields{
		"k": tr.Key,
		"v": tr.Value,
	}).Trace("agent: Untag requested")

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

// Nodes ...
func (as *MeshAgentServer) Nodes(cte *AgentEmpty, agentNodesServer Agent_NodesServer) error {
	log.Trace("agent: Nodes requested")

	myCoord, err := as.ms.s.GetCoordinate()
	if err != nil {
		log.WithError(err).Warn("Unable to get my own coordinate, check config")
		return err
	}

	for _, member := range as.ms.s.Members() {
		var rtt int32 = 0
		memberCoord, ok := as.ms.s.GetCachedCoordinate(member.Name)
		if ok && memberCoord != nil {
			d := memberCoord.DistanceTo(myCoord)
			rtt = int32(d / time.Millisecond)
		}

		tags := make([]*MemberInfoTag, 0)
		for tagKey, tagValue := range member.Tags {
			tags = append(tags, &MemberInfoTag{
				Key:   tagKey,
				Value: tagValue,
			})
		}

		memberInfo := &MemberInfo{
			NodeName: member.Name,
			Addr:     member.Addr.String(),
			Status:   member.Status.String(),
			RttMsec:  rtt,
			Tags:     tags,
		}

		if err := agentNodesServer.Send(memberInfo); err != nil {
			log.WithError(err).Error("unable to stream send nodes info")
		}
	}

	return nil
}

// WaitForChangeInMesh ...
func (as *MeshAgentServer) WaitForChangeInMesh(wi *WaitInfo, server Agent_WaitForChangeInMeshServer) error {

	ch := make(chan serf.Event)
	key := fmt.Sprintf("agent-waitforchange-%d", rand.Int63n(math.MaxInt64))
	as.ms.RegisterEventNotifier(key, &ch)

	for {
		select {
		case <-ch:
			as.ms.DeregisterEventNotifier(key)
			server.Send(&WaitResponse{
				WasTimeout:     false,
				ChangesOccured: true,
			})
			return nil
		case <-time.After(time.Duration(wi.TimeoutSecs) * time.Second):
			as.ms.DeregisterEventNotifier(key)
			server.Send(&WaitResponse{
				WasTimeout:     true,
				ChangesOccured: false,
			})
			return nil
		}
	}
}

// RTT ...
func (as *MeshAgentServer) RTT(cte *AgentEmpty, rttServer Agent_RTTServer) error {
	log.Trace("agent: RTT requested")

	ch := make(chan RTTResponse)
	doneCh := make(chan struct{})

	go func() {
		for {
			select {
			case rtt := <-ch:
				//log.WithField("rtt", rtt).Trace("RTT")

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
	as.ms.s.UserEvent(serfEventMarkerRTTReq, []byte(rttRequestBuf), true)

	// wait until all are collected and streamed out
	time.Sleep(time.Duration(as.ms.s.NumNodes()+2) * time.Second)

	// done
	doneCh <- struct{}{}

	return nil

}

// StartAgentGrpcService ..
func (as *MeshAgentServer) StartAgentGrpcService() error {
	lis, err := net.Listen("unix", as.grpcBindSocket)
	if err != nil {
		log.Errorf("failed to listen: %v", err)
		return errors.New("unable to start grpc mesh service")
	}

	if as.grpcBindSocketID != "" {
		arr := strings.Split(as.grpcBindSocketID, ":")
		if len(arr) == 2 {

			uid, _ := strconv.Atoi(arr[0])
			gid, _ := strconv.Atoi(arr[1])

			if err := os.Chown(as.grpcBindSocket, uid, gid); err != nil {
				log.WithError(err).Error("unable to assign uid:gid as per -grpc-bing-socket-id")
				os.Exit(10)
			}
		}
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
