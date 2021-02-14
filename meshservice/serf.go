package meshservice

import (
	"encoding/json"
	"errors"
	"fmt"
	ioutil "io/ioutil"
	"net"

	"os"
	reflect "reflect"
	"time"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	memberlist "github.com/hashicorp/memberlist"
	serf "github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// parses the user event as a Peer announcement and adds the peer
// to the wireguard interface
func (ms *MeshService) serfHandleJoinRequestEvent(userEv serf.UserEvent) {
	peerAnnouncement := &Peer{}
	err := proto.Unmarshal(userEv.Payload, peerAnnouncement)
	if err != nil {
		log.WithError(err).Error("unable to unmarshal a user event")
	}
	log.WithField("pa", peerAnnouncement).Trace("user event: peerAnnouncement")

	if peerAnnouncement.Type == Peer_JOIN {
		wg := wgwrapper.New()
		ok, err := wg.AddPeer(ms.WireguardInterface, wgwrapper.WireguardPeer{
			RemoteEndpointIP: peerAnnouncement.EndpointIP,
			ListenPort:       int(peerAnnouncement.EndpointPort),
			Pubkey:           peerAnnouncement.Pubkey,
			AllowedIPs: []net.IPNet{
				net.IPNet{
					IP:   net.ParseIP(peerAnnouncement.MeshIP),
					Mask: net.CIDRMask(32, 32),
				},
			},
		})

		log.WithField("ok", ok).Trace("1")
		log.WithField("err", err).Trace("1")

		if err != nil {
			log.WithError(err).Error("unable to add peer after user event")
		} else {
			if ok {
				log.WithFields(log.Fields{
					"pk": peerAnnouncement.Pubkey,
					"ip": peerAnnouncement.MeshIP,
				}).Info("added peer")
			} else {
				// if we're a bootstrap node then this peer has already been added
				// by the grpc join request function.
			}
		}
	}
}

func (ms *MeshService) serfEventHandler(ch <-chan serf.Event) {
	for {
		select {
		case ev := <-ch:
			if ev.EventType() == serf.EventUser {
				userEv := ev.(serf.UserEvent)

				if userEv.Name == "j" {
					log.WithField("ev", ev).Debug("received join request event")
					ms.serfHandleJoinRequestEvent(userEv)
				}

			}

			if ev.EventType() == serf.EventMemberJoin {
				evJoin := ev.(serf.MemberEvent)

				log.WithField("members", evJoin.Members).Debug("received join event")
			}
			if ev.EventType() == serf.EventMemberLeave || ev.EventType() == serf.EventMemberFailed || ev.EventType() == serf.EventMemberReap {
				evJoin := ev.(serf.MemberEvent)

				log.WithField("members", evJoin.Members).Debug("received leave/failed event")

				for _, member := range evJoin.Members {
					// remove this peer from wireguard interface
					wg := wgwrapper.New()

					err := wg.RemovePeerByPubkey(ms.WireguardInterface, member.Tags["pk"])
					if err != nil {
						log.WithError(err).Error("unable to remove failed/left wireguard peer")
					}

					err = ms.s.RemoveFailedNodePrune(member.Name)
					if err != nil {
						log.WithError(err).Error("unable to remove failed/left serf node")
					} else {
						log.WithFields(log.Fields{
							"node": member.Name,
							"ip":   member.Addr.String(),
						}).Info("node left mesh")
					}

				}

			}

		}
	}
}

// NewSerfCluster sets up a cluster with a given nodeName,
// a bind address. it also registers a user event listener
// which acts upon Join and Leave user messages
func (ms *MeshService) NewSerfCluster() {

	cfg := serfCustomWANConfig(ms.NodeName, ms.MeshIP.IP.String())

	// set up the event handler for all user events
	ch := make(chan serf.Event, 1)
	go ms.serfEventHandler(ch)
	cfg.EventCh = ch

	if log.GetLevel() == log.TraceLevel {
		log.Trace("enabling serf log output")
		cfg.LogOutput = log.StandardLogger().Out
	} else {
		cfg.LogOutput = ioutil.Discard
	}
	cfg.MemberlistConfig.LogOutput = cfg.LogOutput

	// @TODO cfg.MemberlistConfig.SecretKey = ...

	ms.cfg = cfg

}

// StartSerfCluster is used by bootstrap to set up the initial serf cluster node
// A set of node tags is derived from all parameters so that other nodes have
// all data to connect.
func (ms *MeshService) StartSerfCluster(isBootstrap bool, pubkey string, endpointIP string, endpointPort int, meshIP string) error {

	s, err := serf.Create(ms.cfg)
	if err != nil {
		return errors.New("Unable to set up serf cluster")
	}
	nodeType := "n"
	if isBootstrap {
		nodeType = "b"
	}
	tags := map[string]string{
		"t":    nodeType,
		"pk":   pubkey,
		"addr": fmt.Sprintf("%s", endpointIP),
		"port": fmt.Sprintf("%d", endpointPort),
		"i":    meshIP,
	}
	log.WithField("tags", tags).Trace("setting tags for this node")
	s.SetTags(tags)

	ms.s = s

	log.Debug("started serf cluster")

	return nil
}

// JoinSerfCluster calls serf.Join, given a number of cluster nodes received from the bootstrap node
func (ms *MeshService) JoinSerfCluster(clusterNodes []string) {
	log.WithField("l", clusterNodes).Trace("cluster node list")

	log.Debugf("Joining serf cluster via %d nodes", len(clusterNodes))
	ms.s.Join(clusterNodes, true)
}

// LeaveSerfCluster leaves the cluster
func (ms *MeshService) LeaveSerfCluster() {
	ms.s.Leave()

	time.Sleep(5 * time.Second)
	log.Info("Left the serf cluster")

	ms.s.Shutdown()
	log.Debug("Shut down the serf instance")
}

// StatsUpdate produces a mesh statistic update on log
func (ms *MeshService) StatsUpdate() {
	log.WithField("stats", ms.s.Stats()).Debug("serf cluster statistics")
}

type statsContent struct {
	numNodes int
}

func (ms *MeshService) getStats() *statsContent {
	return &statsContent{
		numNodes: ms.s.NumNodes(),
	}
}

// SetMemberlistExportFile sets the file name for an export
// of the current memberlist. If empty no file is written
func (ms *MeshService) SetMemberlistExportFile(f string) {
	ms.memberExportFile = f
}

type exportedMember struct {
	Addr   string            `json:"addr"`
	Status string            `json:"st"`
	RTT    int64             `json:"rtt"`
	Tags   map[string]string `json:"tags"`
}
type exportedMemberList struct {
	Members    map[string]exportedMember `json:"members"`
	LastUpdate time.Time                 `json:"lastUpdate"`
}

func (ms *MeshService) updateMemberExport() {
	e := &exportedMemberList{
		Members:    make(map[string]exportedMember),
		LastUpdate: time.Now(),
	}
	myCoord, err := ms.s.GetCoordinate()
	if err != nil {
		log.WithError(err).Warn("Unable to get my own coordinate, check config")
		myCoord = nil
	}
	for _, member := range ms.s.Members() {
		em := exportedMember{
			Addr:   member.Addr.String(),
			Status: member.Status.String(),
			Tags:   member.Tags,
		}
		// compute RTT if we have all distances
		memberCoord, ok := ms.s.GetCachedCoordinate(member.Name)
		if ok && memberCoord != nil {
			d := memberCoord.DistanceTo(myCoord)
			em.RTT = int64(d / time.Millisecond)

			// TODO: for LAN mode add Microseconds as well
		}

		//
		e.Members[member.Name] = em
	}

	content, err := json.MarshalIndent(e, "", " ")
	if err != nil {
		log.WithError(err).Error("unable to write to file")
	}

	err = ioutil.WriteFile(ms.memberExportFile, content, 0640)
}

// StartStatsUpdater starts the statistics update ticker
func (ms *MeshService) StartStatsUpdater() {

	// TODO make configurable
	ticker1 := time.NewTicker(1000 * time.Millisecond)
	ticker2 := time.NewTicker(60 * time.Second)
	done := make(chan bool)

	var last *statsContent = nil

	// The first update dumps the node count only when it changes
	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-ticker1.C:
				if ms.memberExportFile != "" {
					ms.updateMemberExport()
				}

				if last == nil {
					last = ms.getStats()
					log.Infof("Mesh has %d nodes", ms.s.NumNodes())
				} else {

					s := ms.getStats()
					if reflect.DeepEqual(*last, *s) == false {
						last = s
						log.Infof("Mesh has %d nodes", ms.s.NumNodes())
					}
				}
			}
		}
	}()

	// the seconds update dumps serf stats on trace
	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-ticker2.C:
				ms.StatsUpdate()
			}
		}
	}()
}

func serfCustomWANConfig(nodeName string, bindAddr string) *serf.Config {

	ml := memberlist.DefaultLANConfig()
	ml.BindPort = 5353
	ml.BindAddr = bindAddr

	return &serf.Config{
		NodeName:                     nodeName,
		BroadcastTimeout:             5 * time.Second,
		LeavePropagateDelay:          1 * time.Second,
		EventBuffer:                  512,
		QueryBuffer:                  512,
		LogOutput:                    os.Stderr,
		ProtocolVersion:              4,
		ReapInterval:                 15 * time.Second,
		RecentIntentTimeout:          5 * time.Minute,
		ReconnectInterval:            30 * time.Second,
		ReconnectTimeout:             24 * time.Hour,
		QueueCheckInterval:           30 * time.Second,
		QueueDepthWarning:            128,
		MaxQueueDepth:                4096,
		TombstoneTimeout:             24 * time.Hour,
		FlapTimeout:                  60 * time.Second,
		MemberlistConfig:             ml,
		QueryTimeoutMult:             16,
		QueryResponseSizeLimit:       1024,
		QuerySizeLimit:               1024,
		EnableNameConflictResolution: true,
		DisableCoordinates:           false,
		ValidateNodeNames:            false,
		UserEventSizeLimit:           512,
	}
}
