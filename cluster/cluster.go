package cluster

import (
	"os"
	"time"

	memberlist "github.com/hashicorp/memberlist"
	serf "github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
)

// Cluster ...
type Cluster struct {
	cfg *serf.Config
	s   *serf.Serf
}

// NewCluster sets up a cluster with a given nodeName,
// a bind address
func NewCluster(nodeName string, bindAddr string) *Cluster {

	cfg := serfCustomWANConfig(nodeName, bindAddr)

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

	return &Cluster{
		cfg: cfg,
	}
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
