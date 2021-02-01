package cmd

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	cluster "github.com/aschmidt75/wgmesh/cluster"
	wireguard "github.com/aschmidt75/wgmesh/wireguard"
	log "github.com/sirupsen/logrus"
)

// BootstrapCommand struct
type BootstrapCommand struct {
	CommandDefaults

	fs         *flag.FlagSet
	meshName   string
	cidrRange  string
	ip         string
	listenPort int
}

// NewBootstrapCommand creates the Bootstrap Command
func NewBootstrapCommand() *BootstrapCommand {
	c := &BootstrapCommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("bootstrap", flag.ContinueOnError),
		meshName:        "",
		cidrRange:       "10.232.0.0/16",
		ip:              "10.232.1.1",
		listenPort:      54540,
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short)")
	c.fs.StringVar(&c.cidrRange, "cidr", c.cidrRange, "CIDR range of this mesh (internal ips)")
	c.fs.StringVar(&c.ip, "ip", c.ip, "internal ip of the bootstrap node")
	c.fs.IntVar(&c.listenPort, "listen-port", c.listenPort, "set the (external) wireguard listen port")
	c.DefaultFields(c.fs)

	return c
}

// Name returns the name of the command
func (g *BootstrapCommand) Name() string {
	return g.fs.Name()
}

// Init sets up the command struct from arguments
func (g *BootstrapCommand) Init(args []string) error {
	err := g.fs.Parse(args)
	if err != nil {
		return err
	}
	g.ProcessDefaults()

	if g.meshName == "" {
		return errors.New("mesh name (--name, -n) may not be empty")
	}
	if len(g.meshName) > 10 {
		return errors.New("mesh name (--name, -n) must have maximum length of 10")
	}
	_, _, err = net.ParseCIDR(g.cidrRange)
	if err != nil {
		return fmt.Errorf("%s is not a valid cidr range for -cidr", g.cidrRange)
	}
	if net.ParseIP(g.ip) == nil {
		return fmt.Errorf("%s is not a valid ip for -ip", g.ip)
	}

	if g.listenPort < 0 || g.listenPort > 65535 {
		return fmt.Errorf("%d is not valid for -listen-port", g.listenPort)
	}

	return nil
}

// Run runs the command by creating the wireguard interface,
// starting the serf cluster and grpc server
func (g *BootstrapCommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	err := wireguard.CreateWireguardInterfaceForMesh(g.meshName, g.ip, g.listenPort)
	if err != nil {
		return err
	}

	nodeName := fmt.Sprintf("%s-%s", g.meshName, g.ip)

	clusterConfig := cluster.NewCluster(nodeName, g.ip)

	err = cluster.StartCluster(clusterConfig)
	if err != nil {
		return err
	}

	// wait until stopped
	stopCh := make(chan struct{})
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		stopCh <- struct{}{}
	}()

	<-stopCh

	return nil
}
