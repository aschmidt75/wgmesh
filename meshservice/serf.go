package meshservice

import (
	"errors"
	"fmt"
	ioutil "io/ioutil"

	"os"
	reflect "reflect"
	"time"

	memberlist "github.com/hashicorp/memberlist"
	serf "github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
)

// NewSerfCluster sets up a cluster with a given nodeName,
// a bind address. it also registers a user event listener
// which acts upon Join and Leave user messages
func (ms *MeshService) NewSerfCluster(lanMode bool) {

	cfg := serfCustomConfig(ms.NodeName, ms.MeshIP.IP.String(), lanMode)

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

	if len(ms.serfEncryptionKey) == 32 {
		cfg.MemberlistConfig.SecretKey = ms.serfEncryptionKey
	}

	ms.cfg = cfg

}

func (ms *MeshService) isNodeNameInUse(nodeName string) bool {
	if nodeName == "" {
		return false
	}
	return false
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
		nodeTagNodeType: nodeType,
		nodeTagPubKey:   pubkey,
		nodeTagAddr:     fmt.Sprintf("%s", endpointIP),
		nodeTagPort:     fmt.Sprintf("%d", endpointPort),
		nodeTagMeshIP:   meshIP,
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

	time.Sleep(3 * time.Second)
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

func serfCustomConfig(nodeName string, bindAddr string, lanMode bool) *serf.Config {

	var ml *memberlist.Config

	if lanMode {
		ml = memberlist.DefaultLANConfig()
	} else {
		ml = memberlist.DefaultWANConfig()
	}
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
