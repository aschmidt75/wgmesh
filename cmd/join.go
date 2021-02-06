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

	fs         *flag.FlagSet
	meshName   string
	endpoint   string
	listenPort int
	listenIP   string
}

// NewJoinCommand creates the Join Command
func NewJoinCommand() *JoinCommand {
	c := &JoinCommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("join", flag.ContinueOnError),
		meshName:        "",
		endpoint:        "",
		listenPort:      54540,
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short)")
	c.fs.StringVar(&c.endpoint, "bootstrap-addr", c.endpoint, "IP:Port of remote mesh bootstrap node")
	c.fs.IntVar(&c.listenPort, "listen-port", c.listenPort, "set the (external) wireguard listen port")
	c.fs.StringVar(&c.listenIP, "listen-ip", c.listenIP, "set the (external) wireguard listen IP. May be an IP adress, or an interface name (e.g. eth0) or a numbered address on an interface (e.g. eth0%1)")
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

	// TODO endpoint

	if g.listenPort < 0 || g.listenPort > 65535 {
		return fmt.Errorf("%d is not valid for -listen-port", g.listenPort)
	}

	// TODO listenIP

	return nil
}

func getIPFromListenIPParam(i string) net.IP {

	// is this an IP address?
	ok := true
	for _, c := range i {
		if (c >= '0' && c <= '9') || c == '.' {
			ok = true
		} else {
			ok = false
			break
		}
	}
	if ok {
		return net.ParseIP(i)
	}

	arr := strings.Split(i, "%")
	idx := 0
	if len(arr) >= 2 {
		i = arr[0]
		var err error
		idx, err = strconv.Atoi(arr[1])
		if err != nil {
			idx = 0
		}
	}

	log.WithField("idx", idx).Trace(".")

	// is it a valid interface name?
	intf, err := net.InterfaceByName(i)
	if err != nil {
		return nil
	}

	addrs, err := intf.Addrs()
	if err != nil {
		return nil
	}

	log.WithField("addrs", addrs).Trace(".")

	if idx >= len(addrs) {
		return nil
	}

	s := addrs[idx].String()
	if strings.IndexAny(s, "/") >= 0 {
		arr = strings.Split(s, "/")
		s = arr[0]
	}

	return net.ParseIP(s)
}

// Run runs the command by creating the wireguard interface,
// starting the serf cluster and grpc server
func (g *JoinCommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	listenIP := getIPFromListenIPParam(g.listenIP)
	log.WithField("ip", listenIP).Trace("parsed -listen-ip")
	if listenIP == nil {
		return errors.New("need -listen-ip")
	}

	ms := meshservice.NewMeshService(g.meshName)
	log.WithField("ms", ms).Trace(
		"created",
	)
	ms.WireguardListenIP = listenIP

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

	joinResponse, err := service.BeginJoin(context.Background(), &meshservice.JoinRequest{
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
	meshIPs := ms.ApplyPeerUpdatesFromStream(wg, stream)

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

	// start the serf part
	ms.NewSerfCluster()

	err = ms.StartSerfCluster()
	if err != nil {
		return err
	}

	ms.StartStatsUpdater()

	// join the cluster
	ms.JoinSerfCluster(meshIPs)

	// wait until being stopped
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

	// take everything down
	err = ms.RemoveWireguardInterfaceForMesh()
	if err != nil {
		return err
	}
	return nil
}
