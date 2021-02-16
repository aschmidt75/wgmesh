package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// JoinCommand struct
type JoinCommand struct {
	CommandDefaults

	fs                *flag.FlagSet
	meshName          string
	endpoint          string
	listenPort        int
	listenIP          string
	memberListFile    string
	agentGrpcBindAddr string
	agentGrpcBindPort int
}

// NewJoinCommand creates the Join Command
func NewJoinCommand() *JoinCommand {
	c := &JoinCommand{
		CommandDefaults:   NewCommandDefaults(),
		fs:                flag.NewFlagSet("join", flag.ContinueOnError),
		meshName:          "",
		endpoint:          "",
		listenPort:        54540,
		memberListFile:    "",
		agentGrpcBindAddr: "127.0.0.1",
		agentGrpcBindPort: 5001,
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short)")
	c.fs.StringVar(&c.endpoint, "bootstrap-addr", c.endpoint, "IP:Port of remote mesh bootstrap node")
	c.fs.IntVar(&c.listenPort, "listen-port", c.listenPort, "set the (external) wireguard listen port")
	c.fs.StringVar(&c.listenIP, "listen-addr", c.listenIP, "set the (external) wireguard listen IP. May be an IP adress, or an interface name (e.g. eth0) or a numbered address on an interface (e.g. eth0%1)")
	c.fs.StringVar(&c.memberListFile, "memberlist-file", c.memberListFile, "optional name of file for a log of all current mesh members")
	c.fs.StringVar(&c.agentGrpcBindAddr, "agent-grpc-bind-addr", c.agentGrpcBindAddr, "(private) address to bind local agent service to")
	c.fs.IntVar(&c.agentGrpcBindPort, "agent-grpc-bind-port", c.agentGrpcBindPort, "port to bind local agent service to")
	c.DefaultFields(c.fs)

	return c
}

// Name returns the name of the command
func (g *JoinCommand) Name() string {
	return g.fs.Name()
}

// Init sets up the command struct from arguments
func (g *JoinCommand) Init(args []string) error {
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

	arr := strings.Split(g.endpoint, ":")
	if len(arr) != 2 {
		return errors.New("-bootstrap-addr must be <IP>:<port>")
	}
	if net.ParseIP(arr[0]) == nil {
		return fmt.Errorf("%s is not a valid ip for -bootstrap-addr", arr[0])
	}
	_, err = strconv.Atoi(arr[1])
	if err != nil {
		return fmt.Errorf("%s is not a valid port for -bootstrap-addr", arr[1])
	}

	if g.listenPort < 0 || g.listenPort > 65535 {
		return fmt.Errorf("%d is not valid for -listen-port", g.listenPort)
	}

	if net.ParseIP(g.agentGrpcBindAddr) == nil {
		return fmt.Errorf("%s is not a valid ip for -agent-grpc-bind-addr", g.agentGrpcBindAddr)
	}

	if g.agentGrpcBindPort < 0 || g.agentGrpcBindPort > 65535 {
		return fmt.Errorf("%d is not valid for -agent-grpc-bind-port", g.agentGrpcBindPort)
	}

	return nil
}

// Run runs the command by creating the wireguard interface,
// starting the serf cluster and grpc server
func (g *JoinCommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	listenIP := getIPFromIPOrIntfParam(g.listenIP)
	log.WithField("ip", listenIP).Trace("parsed -listen-addr")
	if listenIP == nil {
		return errors.New("need -listen-addr")
	}

	ms := meshservice.NewMeshService(g.meshName)
	log.WithField("ms", ms).Trace(
		"created",
	)
	ms.WireguardListenIP = listenIP
	ms.SetMemberlistExportFile(g.memberListFile)

	pk, err := ms.CreateWireguardInterface(g.listenPort)
	if err != nil {
		return err
	}
	ms.WireguardPubKey = pk

	conn, err := grpc.Dial(g.endpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Error(err)
		return fmt.Errorf("cannot connect to %s", g.endpoint)
	}
	defer conn.Close()

	service := meshservice.NewMeshClient(conn)
	log.WithField("service", service).Trace("got grpc service")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	joinResponse, err := service.Join(ctx, &meshservice.JoinRequest{
		Pubkey:       ms.WireguardPubKey,
		EndpointIP:   listenIP.String(),
		EndpointPort: int32(g.listenPort),
	})
	if err != nil {
		log.Error(err)
		return fmt.Errorf("cannot communicate with endpoint at %s", g.endpoint)
	}
	log.WithField("jr", joinResponse).Trace("got joinResponse")

	//
	if joinResponse.Result == meshservice.JoinResponse_ERROR {
		log.Errorf("Unable to join mesh, message: '%s'. Exiting", joinResponse.ErrorMessage)

		err = ms.RemoveWireguardInterfaceForMesh()
		if err != nil {
			return err
		}
		return nil
	}

	// MeshIP ist composed of what user specifies using -ip, but
	// with the net mask of -cidr. e.g. 10.232.0.0/16 with an
	// IP of 10.232.5.99 becomes 10.232.5.99/16
	ms.MeshIP = net.IPNet{
		IP:   net.ParseIP(joinResponse.JoinerMeshIP),
		Mask: ms.CIDRRange.Mask,
	}
	log.WithField("meship", ms.MeshIP).Trace("using mesh ip")

	// we have been assigned a local IP for the wireguard interface. Apply it.
	err = ms.AssignJoinerIP(joinResponse.JoinerMeshIP)
	if err != nil {
		log.Error(err)
		log.Error("Unable assign mesh ip. Exiting")

		// TODO: inform bootstrap explicitely about this, because we're not able
		// to inform the cluster via gossip.

		// take down interface
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	ms.SetNodeName()

	// query the list of all peers.
	stream, err := service.Peers(ctx, &meshservice.Empty{})
	if err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	wg := wgwrapper.New()

	// apply peer updates
	meshPeerIPs := ms.ApplyPeerUpdatesFromStream(wg, stream)

	// the interface is fully configured, up it
	wg.SetInterfaceUp(ms.WireguardInterface)

	// Add a route to the CIDR range of the mesh. All data
	// comes from the join response
	_, meshCidr, _ := net.ParseCIDR(joinResponse.MeshCidr)
	ms.CIDRRange = *meshCidr
	err = ms.SetRoute()
	if err != nil {
		return err
	}

	// start the serf part. make it join all received peers
	err = g.serfSetup(&ms, listenIP, meshPeerIPs)
	if err != nil {
		return err
	}

	err = g.grpcSetup(&ms)
	if err != nil {
		return err
	}

	g.wait()

	if err = g.cleanUp(&ms); err != nil {
		return err
	}

	return nil
}

// grpcSetup starts the local agent
func (g *JoinCommand) grpcSetup(ms *meshservice.MeshService) (err error) {

	// start the local agent
	ms.MeshAgentServer = meshservice.NewMeshAgentServer(ms, g.agentGrpcBindAddr, g.agentGrpcBindPort)
	log.WithField("mas", ms.MeshAgentServer).Trace("agent")
	go func() {
		log.Infof("Starting gRPC Agent Service at %s:%d", g.agentGrpcBindAddr, g.agentGrpcBindPort)
		err = ms.MeshAgentServer.StartAgentGrpcService()
		if err != nil {
			log.Error(err)
		}
	}()

	return nil
}

// serfSetup ...
func (g *JoinCommand) serfSetup(ms *meshservice.MeshService, listenIP net.IP, meshIPs []string) (err error) {
	ms.NewSerfCluster()

	err = ms.StartSerfCluster(false, ms.WireguardPubKey, listenIP.String(), g.listenPort, ms.MeshIP.IP.String())
	if err != nil {
		return err
	}

	ms.StartStatsUpdater()

	// join the cluster
	ms.JoinSerfCluster(meshIPs)

	return nil
}

// waits until being stopped
func (g *JoinCommand) wait() {

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
func (g *JoinCommand) cleanUp(ms *meshservice.MeshService) error {
	ms.LeaveSerfCluster()

	// delete memberlist-file
	os.Remove(g.memberListFile)

	err := ms.RemoveWireguardInterfaceForMesh()
	if err != nil {
		return err
	}
	return nil
}
