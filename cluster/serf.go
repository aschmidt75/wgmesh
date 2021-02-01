package cluster

import (
	"errors"

	serf "github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
)

// StartCluster is used by bootstrap to set up the initial serf cluster node
func StartCluster(cl *Cluster) error {

	log.WithField("cl", cl).Trace("using serf config")

	s, err := serf.Create(cl.cfg)
	if err != nil {
		return errors.New("Unable to set up serf cluster")
	}

	cl.s = s

	log.Debug("started serf cluster")

	return nil
}
