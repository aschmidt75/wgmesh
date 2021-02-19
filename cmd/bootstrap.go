package cmd

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"gortc.io/stun"
)

// BootstrapCommand struct
type BootstrapCommand struct {
	CommandDefaults

	fs                     *flag.FlagSet
	meshName               string
	nodeName               string
	cidrRange              string
	ip                     string
	wgListenAddr           string
	wgListenPort           int
	grpcBindAddr           string
	grpcBindPort           int
	memberListFile         string
	agentGrpcBindSocket    string
	agentGrpcBindSocketIDs string
}

// NewBootstrapCommand creates the Bootstrap Command
func NewBootstrapCommand() *BootstrapCommand {
	c := &BootstrapCommand{
		CommandDefaults:        NewCommandDefaults(),
		fs:                     flag.NewFlagSet("bootstrap", flag.ContinueOnError),
		meshName:               "",
		nodeName:               "",
		cidrRange:              "10.232.0.0/16",
		ip:                     "10.232.1.1",
		wgListenAddr:           "",
		wgListenPort:           54540,
		grpcBindAddr:           "0.0.0.0",
		grpcBindPort:           5000,
		agentGrpcBindSocket:    "/var/run/wgmesh.sock",
		agentGrpcBindSocketIDs: "",
		memberListFile:         "",
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short)")
	c.fs.StringVar(&c.nodeName, "node-name", c.nodeName, "(optional) name of this node")
	c.fs.StringVar(&c.cidrRange, "cidr", c.cidrRange, "CIDR range of this mesh (internal ips)")
	c.fs.StringVar(&c.ip, "ip", c.ip, "internal ip of the bootstrap node")
	c.fs.StringVar(&c.wgListenAddr, "listen-addr", c.wgListenAddr, "external wireguard ip")
	c.fs.IntVar(&c.wgListenPort, "listen-port", c.wgListenPort, "set the (external) wireguard listen port")
	c.fs.StringVar(&c.grpcBindAddr, "grpc-bind-addr", c.grpcBindAddr, "(public) address to bind grpc mesh service to")
	c.fs.IntVar(&c.grpcBindPort, "grpc-bind-port", c.grpcBindPort, "port to bind grpc mesh service to")
	c.fs.StringVar(&c.agentGrpcBindSocket, "agent-grpc-bind-socket", c.agentGrpcBindSocket, "local socket file to bind grpc agent to")
	c.fs.StringVar(&c.agentGrpcBindSocketIDs, "agent-grpc-bind-socket-id", c.agentGrpcBindSocketIDs, "<uid:gid> to change bind socket to")
	c.fs.StringVar(&c.memberListFile, "memberlist-file", c.memberListFile, "optional name of file for a log of all current mesh members")
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

	// ip must be a local one
	if pr, _ := isPrivateIP(g.ip); pr == false {
		return fmt.Errorf("-ip %s is not RFC1918, must be a private address", g.ip)
	}

	if g.wgListenPort < 0 || g.wgListenPort > 65535 {
		return fmt.Errorf("%d is not valid for -listen-port", g.wgListenPort)
	}

	if net.ParseIP(g.grpcBindAddr) == nil {
		return fmt.Errorf("%s is not a valid ip for -grpc-bind-addr", g.grpcBindAddr)
	}

	if g.grpcBindPort < 0 || g.grpcBindPort > 65535 {
		return fmt.Errorf("%d is not valid for -grpc-bind-port", g.grpcBindPort)
	}

	if g.agentGrpcBindSocketIDs != "" {
		re := regexp.MustCompile(`^[0-9]+:[0-9]+$`)

		if !re.Match([]byte(g.agentGrpcBindSocketIDs)) {
			return fmt.Errorf("%s is not valid for -grpc-bind-socket-id", g.agentGrpcBindSocketIDs)
		}
	}

	return nil
}

// Run runs the command by creating the wireguard interface,
// starting the serf cluster and grpc server
func (g *BootstrapCommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	ms := meshservice.NewMeshService(g.meshName)
	log.WithField("ms", ms).Trace(
		"created",
	)
	ms.SetMemberlistExportFile(g.memberListFile)

	var wgListenAddr net.IP
	if g.wgListenAddr == "" {
		log.Info("Fetching external IP from STUN server")
		// Creating a "connection" to STUN server.
		c, err := stun.Dial("udp4", "stun.l.google.com:19302")
		if err != nil {
			return err
		}
		// Building binding request with random transaction id.
		message, err := stun.Build(stun.TransactionID, stun.BindingRequest)
		if err != nil {
			return err
		}
		// Sending request to STUN server, waiting for response message.
		if err := c.Do(message, func(res stun.Event) {
			if res.Error != nil {
				return
			}
			// Decoding XOR-MAPPED-ADDRESS attribute from message.
			var xorAddr stun.XORMappedAddress
			if err := xorAddr.GetFrom(res.Message); err != nil {
				return
			}
			wgListenAddr = xorAddr.IP
			log.WithField("ip", wgListenAddr).Info("Using external IP when connecting with mesh")
		}); err != nil {
			return err
		}
	} else {

		wgListenAddr = getIPFromIPOrIntfParam(g.wgListenAddr)
		log.WithField("ip", wgListenAddr).Trace("parsed -listen-addr")
		if wgListenAddr == nil {
			return errors.New("need -listen-addr")
		}

	}

	_, cidrRangeIpnet, err := net.ParseCIDR(g.cidrRange)
	if err != nil {
		return err
	}
	ms.CIDRRange = *cidrRangeIpnet

	// MeshIP ist composed of what user specifies using -ip, but
	// with the net mask of -cidr. e.g. 10.232.0.0/16 with an
	// IP of 10.232.5.99 becomes 10.232.5.99/16
	ms.MeshIP = net.IPNet{
		IP:   net.ParseIP(g.ip),
		Mask: ms.CIDRRange.Mask,
	}
	log.WithField("meship", ms.MeshIP).Trace("using mesh ip")

	pk, err := g.wireguardSetup(&ms, wgListenAddr)
	if err != nil {
		return err
	}

	err = g.serfSetup(&ms, pk, wgListenAddr)
	if err != nil {
		return err
	}

	if err = g.grpcSetup(&ms); err != nil {
		return err
	}

	ms.SetTimestamps(time.Now().Unix(), time.Now().Unix())

	g.wait()

	if err = g.cleanUp(&ms); err != nil {
		return err
	}

	return nil
}

// wireguardSetup ...
func (g *BootstrapCommand) wireguardSetup(ms *meshservice.MeshService, wgListenAddr net.IP) (pk string, err error) {
	// From the given IP and listen port, create the wireguard interface
	// and set up a basic configuration for it. Up the interface
	pk, err = ms.CreateWireguardInterfaceForMesh(g.ip, g.wgListenPort)
	if err != nil {
		return "", err
	}
	ms.WireguardPubKey = pk
	ms.WireguardListenPort = g.wgListenPort
	ms.WireguardListenIP = wgListenAddr

	// add a route so that all traffic regarding fiven cidr range
	// goes to the wireguard interface.
	err = ms.SetRoute()
	if err != nil {
		return "", err
	}

	ms.SetNodeName(g.nodeName)

	return pk, nil
}

// SerfSetup ...
func (g *BootstrapCommand) serfSetup(ms *meshservice.MeshService, pk string, wgListenAddr net.IP) (err error) {
	// create and start the serf cluster
	ms.NewSerfCluster()

	err = ms.StartSerfCluster(true, pk, wgListenAddr.String(), g.wgListenPort, ms.MeshIP.IP.String())
	if err != nil {
		return err
	}

	ms.StartStatsUpdater()

	return nil
}

// GrpcSetup ...
func (g *BootstrapCommand) grpcSetup(ms *meshservice.MeshService) (err error) {

	// set up grpc mesh service
	ms.GrpcBindAddr = g.grpcBindAddr
	ms.GrpcBindPort = g.grpcBindPort

	go func() {
		log.Infof("Starting gRPC mesh Service at %s:%d", ms.GrpcBindAddr, ms.GrpcBindPort)
		err = ms.StartGrpcService()
		if err != nil {
			log.Error(err)
		}
	}()

	// start the local agent if argument is given
	if g.agentGrpcBindSocket != "" {
		ms.MeshAgentServer = meshservice.NewMeshAgentServerSocket(ms, g.agentGrpcBindSocket, g.agentGrpcBindSocketIDs)
		log.WithField("mas", ms.MeshAgentServer).Trace("agent")
		go func() {
			log.Infof("Starting gRPC Agent Service at %s", g.agentGrpcBindSocket)
			err = ms.MeshAgentServer.StartAgentGrpcService()
			if err != nil {
				log.Error(err)
			}
		}()
	}

	return nil
}

// waits until being stopped
func (g *BootstrapCommand) wait() {

	stopCh := make(chan struct{})
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		stopCh <- struct{}{}
	}()

	<-stopCh
}

// CleanUp ..
func (g *BootstrapCommand) cleanUp(ms *meshservice.MeshService) error {
	// take everything down
	ms.MeshAgentServer.StopAgentGrpcService()

	ms.LeaveSerfCluster()

	ms.StopGrpcService()

	// delete memberlist-file
	os.Remove(g.memberListFile)

	err := ms.RemoveWireguardInterfaceForMesh()
	if err != nil {
		return err
	}

	return nil
}
