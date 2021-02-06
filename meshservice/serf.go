package meshservice

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"os"
	reflect "reflect"
	"time"

	memberlist "github.com/hashicorp/memberlist"
	serf "github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
)

// NewSerfCluster sets up a cluster with a given nodeName,
// a bind address
func (ms *MeshService) NewSerfCluster() {

	cfg := serfCustomWANConfig(ms.NodeName, ms.MeshIP.IP.String())

	ch := make(chan serf.Event, 1)
	go func(ch <-chan serf.Event) {
		for {
			select {
			case ev := <-ch:
				log.WithField("ev", ev).Debug("received event")
			}
		}
	}(ch)
	cfg.EventCh = ch

	cfg.LogOutput = log.StandardLogger().Out

	ms.cfg = cfg

}

// StartSerfCluster is used by bootstrap to set up the initial serf cluster node
func (ms *MeshService) StartSerfCluster() error {

	s, err := serf.Create(ms.cfg)
	if err != nil {
		return errors.New("Unable to set up serf cluster")
	}

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

// StatsUpdate produces a mesh statistic update on log with severity INFO
func (ms *MeshService) StatsUpdate() {
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
	Addr   string `json:"addr"`
	Status string `json:"st"`
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
	for _, member := range ms.s.Members() {
		e.Members[member.Name] = exportedMember{
			Addr:   member.Addr.String(),
			Status: member.Status.String(),
		}
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
	ticker := time.NewTicker(1000 * time.Millisecond)
	done := make(chan bool)

	var last *statsContent = nil

	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-ticker.C:
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
}

func serfCustomWANConfig(nodeName string, bindAddr string) *serf.Config {

	ml := memberlist.DefaultWANConfig()
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
