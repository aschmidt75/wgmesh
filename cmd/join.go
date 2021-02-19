package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gortc.io/stun"
)

// JoinCommand struct
type JoinCommand struct {
	CommandDefaults

	fs                     *flag.FlagSet
	meshName               string
	nodeName               string
	endpoint               string
	listenPort             int
	listenIP               string
	memberListFile         string
	agentGrpcBindSocket    string
	agentGrpcBindSocketIDs string
}

// NewJoinCommand creates the Join Command
func NewJoinCommand() *JoinCommand {
	c := &JoinCommand{
		CommandDefaults:        NewCommandDefaults(),
		fs:                     flag.NewFlagSet("join", flag.ContinueOnError),
		meshName:               "",
		nodeName:               "",
		endpoint:               "",
		listenPort:             54540,
		agentGrpcBindSocket:    "/var/run/wgmesh.sock",
		agentGrpcBindSocketIDs: "",
		memberListFile:         "",
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short)")
	c.fs.StringVar(&c.nodeName, "node-name", c.nodeName, "(optional) name of this node")
	c.fs.StringVar(&c.endpoint, "bootstrap-addr", c.endpoint, "IP:Port of remote mesh bootstrap node")
	c.fs.IntVar(&c.listenPort, "listen-port", c.listenPort, "set the (external) wireguard listen port")
	c.fs.StringVar(&c.listenIP, "listen-addr", c.listenIP, "set the (external) wireguard listen IP. May be an IP adress, or an interface name (e.g. eth0) or a numbered address on an interface (e.g. eth0%1)")
	c.fs.StringVar(&c.agentGrpcBindSocket, "agent-grpc-bind-socket", c.agentGrpcBindSocket, "local socket file to bind grpc agent to")
	c.fs.StringVar(&c.agentGrpcBindSocketIDs, "agent-grpc-bind-socket-id", c.agentGrpcBindSocketIDs, "<uid:gid> to change bind socket to")
	c.fs.StringVar(&c.memberListFile, "memberlist-file", c.memberListFile, "optional name of file for a log of all current mesh members")
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
func (g *JoinCommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	var listenIP net.IP
	if g.listenIP == "" {
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
			listenIP = xorAddr.IP
			log.WithField("ip", listenIP).Info("Using external IP when connecting with mesh")
		}); err != nil {
			return err
		}
	} else {

		listenIP = getIPFromIPOrIntfParam(g.listenIP)
		log.WithField("ip", listenIP).Trace("parsed -listen-addr")
		if listenIP == nil {
			return errors.New("need -listen-addr")
		}

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	joinResponse, err := service.Join(ctx, &meshservice.JoinRequest{
		Pubkey:       ms.WireguardPubKey,
		EndpointIP:   listenIP.String(),
		EndpointPort: int32(g.listenPort),
		MeshName:     g.meshName,
	})
	if err != nil {
		log.Error(err)

		// remove wireguard interface
		err = ms.RemoveWireguardInterfaceForMesh()
		if err != nil {
			log.Error(err)
		}
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

	ms.SetTimestamps(joinResponse.CreationTS, time.Now().Unix())

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

	ms.SetNodeName(g.nodeName)

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

	// apply peer updates. So we will have wireguard peerings
	// to all nodes before we join the serf cluster.
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
	ms.MeshAgentServer = meshservice.NewMeshAgentServerSocket(ms, g.agentGrpcBindSocket, g.agentGrpcBindSocketIDs)
	log.WithField("mas", ms.MeshAgentServer).Trace("agent")
	go func() {
		log.Infof("Starting gRPC Agent Service at %s", g.agentGrpcBindSocket)
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
	// take everything down
	ms.MeshAgentServer.StopAgentGrpcService()

	ms.LeaveSerfCluster()

	// delete memberlist-file
	os.Remove(g.memberListFile)

	err := ms.RemoveWireguardInterfaceForMesh()
	if err != nil {
		return err
	}
	return nil
}
