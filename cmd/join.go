package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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
	"github.com/cristalhq/jwt/v3"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
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
	clientKey              string
	clientCert             string
	caCert                 string
	agentGrpcBindSocket    string
	agentGrpcBindSocketIDs string
	devMode                bool
}

// NewJoinCommand creates the Join Command
func NewJoinCommand() *JoinCommand {
	c := &JoinCommand{
		CommandDefaults:        NewCommandDefaults(),
		fs:                     flag.NewFlagSet("join", flag.ContinueOnError),
		meshName:               envStrWithDefault("WGMESH_MESH_NAME", ""),
		nodeName:               envStrWithDefault("WGMESH_NODE_NAME", ""),
		endpoint:               envStrWithDefault("WGMESH_WIREGUARD_BOOTSTRAP_ADDR", ""),
		listenIP:               envStrWithDefault("WGMESH_WIREGUARD_LISTEN_ADDR", ""),
		listenPort:             envIntWithDefault("WGMESH_WIREGUARD_LISTEN_PORT", 54540),
		agentGrpcBindSocket:    envStrWithDefault("WGMESH_AGENT_GRPC_BIND_SOCKET", "/var/run/wgmesh.sock"),
		agentGrpcBindSocketIDs: envStrWithDefault("WGMESH_AGENT_GRPC_BIND_SOCKET_ID", ""),
		clientKey:              envStrWithDefault("WGMESH_CLIENT_KEY", ""),
		clientCert:             envStrWithDefault("WGMESH_CLIENT_CERT", ""),
		caCert:                 envStrWithDefault("WGMESH_CA_CERT", ""),
		memberListFile:         envStrWithDefault("WGMESH_MEMBERLIST_FILE", ""),
		devMode:                false,
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network.\nenv:WGMESH_MESH_NAME")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short).\nenv:WGMESH_MESH_NAME")
	c.fs.StringVar(&c.nodeName, "node-name", c.nodeName, "(optional) name of this node.\nenv:WGMESH_NODE_NAME")
	c.fs.StringVar(&c.endpoint, "bootstrap-addr", c.endpoint, "IP:Port of remote mesh bootstrap node.\nenv:WGMESH_WIREGUARD_BOOTSTRAP_ADDR")
	c.fs.StringVar(&c.listenIP, "listen-addr", c.listenIP, "set the (external) wireguard listen IP. May be an IP address, or an interface name (e.g. eth0) or a numbered address on an interface (e.g. eth0%1).\nenv:WGMESH_WIREGUARD_LISTEN_ADDR")
	c.fs.IntVar(&c.listenPort, "listen-port", c.listenPort, "set the (external) wireguard listen port.\nenv:WGMESH_WIREGUARD_LISTEN_PORT")
	c.fs.StringVar(&c.agentGrpcBindSocket, "agent-grpc-bind-socket", c.agentGrpcBindSocket, "local socket file to bind grpc agent to.\nenv:WGMESH_AGENT_GRPC_BIND_SOCKET")
	c.fs.StringVar(&c.agentGrpcBindSocketIDs, "agent-grpc-bind-socket-id", c.agentGrpcBindSocketIDs, "<uid:gid> to change bind socket to.\nenv:WGMESH_AGENT_GRPC_BIND_SOCKET_ID ")
	c.fs.StringVar(&c.clientKey, "client-key", c.clientKey, "points to PEM-encoded private key to be used.\nenv:WGMESH_CLIENT_KEY")
	c.fs.StringVar(&c.clientCert, "client-cert", c.clientCert, "points to PEM-encoded certificate be used.\nenv:WGMESH_CLIENT_CERT")
	c.fs.StringVar(&c.caCert, "ca-cert", c.caCert, "points to PEM-encoded CA certificate.\nenv:WGMESH_CA_CERT")
	c.fs.StringVar(&c.memberListFile, "memberlist-file", c.memberListFile, "optional name of file for a log of all current mesh members.\nenv:WGMESH_MEMBERLIST_FILE")
	c.fs.BoolVar(&c.devMode, "dev", c.devMode, "Enables development mode which runs without encryption, authentication and without TLS")
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

	withGrpcSecure := false
	if g.clientKey != "" {
		withGrpcSecure = true

		if !fileExists(g.clientKey) {
			return fmt.Errorf("%s not found for -client-key", g.clientKey)
		}
	}
	if g.clientCert != "" {
		withGrpcSecure = true

		if !fileExists(g.clientCert) {
			return fmt.Errorf("%s not found for -client-cert", g.clientCert)
		}
	}
	if g.caCert != "" {
		withGrpcSecure = true

		if !fileExists(g.caCert) {
			return fmt.Errorf("%s not found for -ca-cert", g.caCert)
		}
	}

	if withGrpcSecure {
		if g.clientKey == "" || g.clientCert == "" || g.caCert == "" {
			//
			return fmt.Errorf("-client-key, -client-cert, -ca-cert must be specified together")
		}
		if g.devMode {
			return fmt.Errorf("Must either set -dev mode for insecure setup or -client-key, -client-cert, -ca-cert must be specified together")
		}
	} else {
		if !g.devMode {
			return fmt.Errorf("Must either set -dev mode for insecure setup or -client-key, -client-cert, -ca-cert must be specified together")
		}
	}

	if g.devMode {
		if withGrpcSecure {
			return fmt.Errorf("cannot combine security parameters in -dev mode")
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

		st := meshservice.NewSTUNService()
		ips, err := st.GetExternalIP()

		if err != nil {
			return err
		}
		if len(ips) > 0 {
			listenIP = ips[0]
			log.WithField("ip", listenIP).Info("Using external IP when connecting with mesh")

		}
	}
	if listenIP == nil {
		listenIP = getIPFromIPOrIntfParam(g.listenIP)
		log.WithField("ip", listenIP).Trace("parsed -listen-addr")
		if listenIP == nil {
			return errors.New("need -listen-addr")
		}

	}

	ms := meshservice.NewMeshService(g.meshName)
	log.WithField("ms", ms).Trace("created")
	ms.WireguardListenIP = listenIP

	ms.SetMemberlistExportFile(g.memberListFile)

	pk, err := ms.CreateWireguardInterface(g.listenPort)
	if err != nil {
		return err
	}
	ms.WireguardPubKey = pk

	// set up TLS configuration from given parameters unless we're in dev mode
	if !g.devMode {
		ms.TLSConfig, err = meshservice.NewTLSConfigFromFiles(g.caCert, "", g.clientCert, g.clientKey)
		if err != nil {

			// remove wireguard interface
			err = ms.RemoveWireguardInterfaceForMesh()
			if err != nil {
				log.Error(err)
			}
			return err
		}
	}

	var opts []grpc.DialOption = []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTimeout(5 * time.Second),
	}
	if ms.TLSConfig != nil {
		transportCreds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{ms.TLSConfig.Cert},
			RootCAs:      ms.TLSConfig.CertPool,
		})

		opts = append(opts, grpc.WithTransportCredentials(transportCreds))
		log.Debug("TLS-connecting to gRPC mesh service")

	} else {
		log.Warn("Using insecure connection to gRPC mesh service")
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(g.endpoint, opts...)
	if err != nil {
		log.Error(err)

		// remove wireguard interface
		err = ms.RemoveWireguardInterfaceForMesh()
		if err != nil {
			log.Error(err)
		}

		return fmt.Errorf("cannot connect to %s", g.endpoint)
	}
	defer conn.Close()

	service := meshservice.NewMeshClient(conn)
	log.WithField("service", service).Trace("got grpc service")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token, err := g.handleHandshake(ctx, service, &ms)
	if err != nil {
		return err
	}
	md := metadata.Pairs("authorization", fmt.Sprintf("Bearer: %s", token))

	mdCtx := metadata.NewOutgoingContext(ctx, md)

	joinResponse, err := service.Join(mdCtx, &meshservice.JoinRequest{
		Pubkey:       ms.WireguardPubKey,
		EndpointIP:   listenIP.String(),
		EndpointPort: int32(g.listenPort),
		MeshName:     g.meshName,
		NodeName:     g.nodeName,
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

	if !g.devMode {
		ms.SetEncryptionKey(string(joinResponse.SerfEncryptionKey))
	}

	// MeshIP ist composed of what user specifies using -ip, but
	// with the net mask of -cidr. e.g. 10.232.0.0/16 with an
	// IP of 10.232.5.99 becomes 10.232.5.99/16
	ms.MeshIP = net.IPNet{
		IP:   net.ParseIP(joinResponse.JoiningNodeMeshIP),
		Mask: ms.CIDRRange.Mask,
	}
	log.WithField("meship", ms.MeshIP).Trace("using mesh ip")

	// we have been assigned a local IP for the wireguard interface. Apply it.
	err = ms.AssignJoiningNodeIP(joinResponse.JoiningNodeMeshIP)
	if err != nil {
		log.Error(err)
		log.Error("Unable assign mesh ip. Exiting")

		// TODO: inform bootstrap explicitly about this, because we're not able
		// to inform the cluster via gossip. Need to leave explicitly

		// take down interface
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	// set my own node name. Can be empty, it is then derived from the
	// local mesh ip to have unique names within the serf cluster
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

	// Add a route to the CIDR range of the mesh. All detail data
	// comes from the join response
	_, meshCidr, _ := net.ParseCIDR(joinResponse.MeshCidr)
	ms.CIDRRange = *meshCidr
	err = ms.SetRoute()
	if err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	// start the serf part. make it join all received peers
	err = g.serfSetup(&ms, listenIP, meshPeerIPs, joinResponse.SerfModeLAN)
	if err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	err = g.grpcSetup(&ms)
	if err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	fmt.Printf("** \n")
	fmt.Printf("** Mesh '%s' has been joined.\n", g.meshName)
	fmt.Printf("** \n")
	fmt.Printf("** Mesh name:                       %s\n", g.meshName)
	fmt.Printf("** Mesh CIDR range:                 %s\n", ms.CIDRRange.String())
	fmt.Printf("** This node's name:                %s\n", ms.NodeName)
	fmt.Printf("** This node's mesh IP:             %s\n", ms.MeshIP.IP.String())
	if g.memberListFile != "" {
		fmt.Printf("** Mesh node details export to:     %s\n", g.memberListFile)
	}
	fmt.Printf("** \n")
	if g.devMode {
		fmt.Printf("** This mesh is running in DEVELOPMENT MODE without encryption.\n")
		fmt.Printf("** Do not use this in a production setup.\n")
		fmt.Printf("** \n")
	} else {
		if ms.TLSConfig != nil && len(ms.TLSConfig.Cert.Certificate) > 0 {
			fmt.Printf("** TLS is enabled for gRPC mesh service\n")

			x, err := x509.ParseCertificate(ms.TLSConfig.Cert.Certificate[0])
			if err == nil {
				fmt.Printf("**  subject: %s\n", x.Subject)
				fmt.Printf("**  issuer: %s\n", x.Issuer)
			}
		}
	}
	fmt.Printf("** \n")
	fmt.Printf("** To inspect the wireguard interface and its peer data use:\n")
	fmt.Printf("** wg show %s\n", ms.WireguardInterface.InterfaceName)
	fmt.Printf("** \n")
	fmt.Printf("** To inspect the current mesh status use: wgmesh info\n")
	fmt.Printf("** \n")

	g.wait()

	if err = g.cleanUp(&ms); err != nil {
		return err
	}

	return nil
}

func (g *JoinCommand) handleHandshake(ctx context.Context, service meshservice.MeshClient, ms *meshservice.MeshService) (tokenStr string, err error) {
	handshakeResponse, err := service.Begin(ctx, &meshservice.HandshakeRequest{
		MeshName: g.meshName,
	})
	if err != nil {
		log.WithError(err).Error("unable to begin handshake")
	}
	log.WithField("hr", handshakeResponse).Trace("got handshakeResponse")
	if handshakeResponse.Result != meshservice.HandshakeResponse_OK {
		msg := "bootstrap node returned handshake error"
		log.WithField("msg", handshakeResponse.ErrorMessage).Error(msg)
		return "", errors.New(msg)
	}

	// extract token
	key := []byte(`secret`)
	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	if err != nil {
		return "", err
	}
	token, err := jwt.ParseAndVerifyString(handshakeResponse.JoinToken, verifier)
	if err != nil {
		log.WithError(err).Error("Unable to parse/verify join token")
		return "", err
	}
	var claims jwt.StandardClaims
	err = json.Unmarshal(token.RawClaims(), &claims)
	if err != nil {
		log.WithError(err).Error("Unable to parse/verify join token claims")
		return "", err
	}

	// check claims
	if !claims.IsForAudience("wgmesh") ||
		!claims.IsValidAt(time.Now()) {

		return "", errors.New("token claims not valid, aborting join")
	}

	log.WithField("claims", claims).Debug("Verified handshake token claims")

	return token.String(), nil
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
func (g *JoinCommand) serfSetup(ms *meshservice.MeshService, listenIP net.IP, meshIPs []string, lanMode bool) (err error) {

	ms.NewSerfCluster(lanMode)

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
