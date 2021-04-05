package cmd

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	config "github.com/aschmidt75/wgmesh/config"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
)

// BootstrapCommand struct
type BootstrapCommand struct {
	CommandDefaults

	fs *flag.FlagSet

	// configuration file
	config string
	// configuration struct
	meshConfig config.Config

	// options not in config, only from parameters
	devMode bool
}

// NewBootstrapCommand creates the Bootstrap Command
func NewBootstrapCommand() *BootstrapCommand {
	c := &BootstrapCommand{
		CommandDefaults: NewCommandDefaults(),

		config:     envStrWithDefault("WGMESH_CONFIG", ""),
		meshConfig: config.NewDefaultConfig(),

		fs:      flag.NewFlagSet("bootstrap", flag.ContinueOnError),
		devMode: false,
	}

	c.fs.StringVar(&c.config, "config", c.config, "file name of config file (optional).\nenv:WGMESH_cONFIG")
	c.fs.StringVar(&c.meshConfig.MeshName, "name", c.meshConfig.MeshName, "name of the mesh network.\nenv:WGMESH_MESH_NAME")
	c.fs.StringVar(&c.meshConfig.MeshName, "n", c.meshConfig.MeshName, "name of the mesh network (short).\nenv:WGMESH_MESH_NAME")
	c.fs.StringVar(&c.meshConfig.NodeName, "node-name", c.meshConfig.NodeName, "(optional) name of this node.\nenv:WGMESH_NODE_NAME")
	c.fs.StringVar(&c.meshConfig.Bootstrap.MeshCIDRRange, "cidr", c.meshConfig.Bootstrap.MeshCIDRRange, "CIDR range of this mesh (internal ips).\nenv:WGMESH_CIDR_RANGE")
	c.fs.StringVar(&c.meshConfig.Bootstrap.MeshIPAMCIDRRange, "cidr-ipam", c.meshConfig.Bootstrap.MeshIPAMCIDRRange, "CIDR (sub)range where this bootstrap mode may allocate ips from. Must be within -cidr range.\nenv:WGMESH_CIDR_RANGE_IPAM")
	c.fs.StringVar(&c.meshConfig.Bootstrap.NodeIP, "ip", c.meshConfig.Bootstrap.NodeIP, "internal ip of the bootstrap node. Must be set fixed for bootstrap nodes.\nenv:WGMESH_MESH_IP")
	c.fs.StringVar(&c.meshConfig.Wireguard.ListenAddr, "listen-addr", c.meshConfig.Wireguard.ListenAddr, "external wireguard ip.\nenv:WGMESH_WIREGUARD_LISTEN_ADDR")
	c.fs.IntVar(&c.meshConfig.Wireguard.ListenPort, "listen-port", c.meshConfig.Wireguard.ListenPort, "set the (external) wireguard listen port.\nenv:WGMESH_WIREGUARD_LISTEN_PORT")
	c.fs.StringVar(&c.meshConfig.Bootstrap.GRPCBindAddr, "grpc-bind-addr", c.meshConfig.Bootstrap.GRPCBindAddr, "(public) address to bind grpc mesh service to.\nenv:WGMESH_GRPC_BIND_ADDR")
	c.fs.IntVar(&c.meshConfig.Bootstrap.GRPCBindPort, "grpc-bind-port", c.meshConfig.Bootstrap.GRPCBindPort, "port to bind grpc mesh service to.\nenv:WGMESH_GRPC_BIND_PORT")
	c.fs.StringVar(&c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerKey, "grpc-server-key", c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerKey, "points to PEM-encoded private key to be used by grpc server.\nenv:WGMESH_SERVER_KEY")
	c.fs.StringVar(&c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerCert, "grpc-server-cert", c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerCert, "points to PEM-encoded certificate be used by grpc server.\nenv:WGMESH_SERVER_CERT")
	c.fs.StringVar(&c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert, "grpc-ca-cert", c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert, "points to PEM-encoded CA certificate.\nenv:WGMESH_CA_CERT")
	c.fs.StringVar(&c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath, "grpc-ca-path", c.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath, "points to a directory containing PEM-encoded CA certificates.\nenv:WGMESH_CA_PATH")
	c.fs.StringVar(&c.meshConfig.Bootstrap.MemberlistFile, "memberlist-file", c.meshConfig.Bootstrap.MemberlistFile, "optional name of file for a log of all current mesh members.\nenv:WGMESH_MEMBERLIST_FILE")
	c.fs.StringVar(&c.meshConfig.Bootstrap.MeshEncryptionKey, "mesh-encryption-key", c.meshConfig.Bootstrap.MeshEncryptionKey, "optional key for symmetric encryption of internal mesh traffic. Must be 32 Bytes base64-ed.\nenv:WGMESH_ENCRYPTION_KEY")
	c.fs.BoolVar(&c.devMode, "dev", c.devMode, "Enables development mode which runs without encryption, authentication and without TLS")
	c.fs.BoolVar(&c.meshConfig.Bootstrap.SerfModeLAN, "serf-mode-lan", c.meshConfig.Bootstrap.SerfModeLAN, "Activates LAN mode or cluster communication. Default is false (=WAN mode).\nenv:WGMESH_SERF_MODE_LAN")
	c.fs.StringVar(&c.meshConfig.Agent.GRPCBindSocket, "agent-grpc-bind-socket", c.meshConfig.Agent.GRPCBindSocket, "local socket file to bind grpc agent to.\nenv:WGMESH_AGENT_BIND_SOCKET")
	c.fs.StringVar(&c.meshConfig.Agent.GRPCBindSocketIDs, "agent-grpc-bind-socket-id", c.meshConfig.Agent.GRPCBindSocketIDs, "<uid:gid> to change bind socket to.\nenv:WGMESH_AGENT_BIND_SOCKET_ID")
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

	// load config file if we have one
	if g.config != "" {
		g.meshConfig, err = config.NewConfigFromFile(g.config)
		if err != nil {
			log.WithError(err).Trace("Config read error")
			return fmt.Errorf("Unable to read configuration from %s", g.config)
		}

		log.WithField("cfg", g.meshConfig).Trace("Read")
		log.WithField("cfg.bootstrap", g.meshConfig.Bootstrap).Trace("Read")
		log.WithField("cfg.wireguard", g.meshConfig.Wireguard).Trace("Read")
		log.WithField("cfg.agent", g.meshConfig.Agent).Trace("Read")
	}

	// validate given parameters/config

	if g.meshConfig.MeshName != "" && len(g.meshConfig.MeshName) > 10 {
		return errors.New("mesh name (--name, -n, mesh-name) must have maximum length of 10")
	}

	_, _, err = net.ParseCIDR(g.meshConfig.Bootstrap.MeshCIDRRange)
	if err != nil {
		return fmt.Errorf("%s is not a valid cidr range for -cidr / bootstrap.mesh-cidr-range", g.meshConfig.Bootstrap.MeshCIDRRange)
	}

	if net.ParseIP(g.meshConfig.Bootstrap.NodeIP) == nil {
		return fmt.Errorf("%s is not a valid ip for -ip", g.meshConfig.Bootstrap.NodeIP)
	}

	// ip must be a local one
	if pr, _ := isPrivateIP(g.meshConfig.Bootstrap.NodeIP); pr == false {
		return fmt.Errorf("-ip %s is not RFC1918, must be a private address", g.meshConfig.Bootstrap.NodeIP)
	}

	if g.meshConfig.Wireguard.ListenPort < 0 || g.meshConfig.Wireguard.ListenPort > 65535 {
		return fmt.Errorf("%d is not valid for -listen-port", g.meshConfig.Wireguard.ListenPort)
	}

	if net.ParseIP(g.meshConfig.Bootstrap.GRPCBindAddr) == nil {
		return fmt.Errorf("%s is not a valid ip for -grpc-bind-addr", g.meshConfig.Bootstrap.GRPCBindAddr)
	}

	if g.meshConfig.Bootstrap.GRPCBindPort < 0 || g.meshConfig.Bootstrap.GRPCBindPort > 65535 {
		return fmt.Errorf("%d is not valid for -grpc-bind-port", g.meshConfig.Bootstrap.GRPCBindPort)
	}

	if g.meshConfig.Agent.GRPCBindSocketIDs != "" {
		re := regexp.MustCompile(`^[0-9]+:[0-9]+$`)

		if !re.Match([]byte(g.meshConfig.Agent.GRPCBindSocketIDs)) {
			return fmt.Errorf("%s is not valid for -grpc-bind-socket-id", g.meshConfig.Agent.GRPCBindSocketIDs)
		}
	}

	if g.meshConfig.Bootstrap.MeshEncryptionKey != "" {
		b, err := base64.StdEncoding.DecodeString(g.meshConfig.Bootstrap.MeshEncryptionKey)
		if err != nil || len(b) != 32 {
			return fmt.Errorf("%s is not valid for -mesh-encryption-key, must be 32 bytes, base64-encoded", g.meshConfig.Bootstrap.MeshEncryptionKey)
		}
	}

	withGrpcSecure := false
	if g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerKey != "" {
		withGrpcSecure = true

		if !fileExists(g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerKey) {
			return fmt.Errorf("%s not found for -grpc-server-key", g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerKey)
		}
	}
	if g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerCert != "" {
		withGrpcSecure = true

		if !fileExists(g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerCert) {
			return fmt.Errorf("%s not found for -grpc-server-cert", g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerCert)
		}
	}
	if g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert != "" {
		withGrpcSecure = true

		if !fileExists(g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert) {
			return fmt.Errorf("%s not found for -grpc-ca-cert", g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert)
		}
	}
	if g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath != "" {
		withGrpcSecure = true

		if !dirExists(g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath) {
			return fmt.Errorf("%s not found for -grpc-ca-path", g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath)
		}
	}

	// when secure setup is desired ..
	if withGrpcSecure {
		// then we need these settings..
		if g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerKey == "" || g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCServerCert == "" || (g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert == "" && g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath == "") {
			//
			return fmt.Errorf("-grpc-server-key, -grpc-server-cert, -grpc-ca-cert / -grp-ca-path must be specified together")
		}
		// and we cannot have dev mode also
		if g.devMode {
			return fmt.Errorf("Must either set -dev mode for insecure setup or -grpc-server-key, -grpc-server-cert, -grpc-ca-cert / -grp-ca-path must be specified together")
		}
	} else {
		// in a non-secure setup we definitely need -dev mode
		if !g.devMode {
			return fmt.Errorf("Must either set -dev mode for insecure setup or -grpc-server-key, -grpc-server-cert, -grpc-ca-cert / -grp-ca-path must be specified together")
		}
	}

	if g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaCert != "" && g.meshConfig.Bootstrap.GRPCTLSConfig.GRPCCaPath != "" {
		return fmt.Errorf("-grpc-ca-cert / -grp-ca-path are mutually exclusive")
	}

	if g.devMode {
		if withGrpcSecure || g.meshConfig.Bootstrap.MeshEncryptionKey != "" {
			return fmt.Errorf("cannot combine security parameters -mesh-encryption-key in -dev mode")
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

	cfg := g.meshConfig

	// if mesh name is empty
	if cfg.MeshName == "" {
		cfg.MeshName = randomMeshName()
		log.WithField("meshName", cfg.MeshName).Warn("auto-generated mesh name. Use this as -n parameter when joining the mesh.")
	}

	ms := meshservice.NewMeshService(cfg.MeshName)
	log.WithField("ms", ms).Trace(
		"created",
	)
	ms.SetMemberlistExportFile(cfg.Bootstrap.MemberlistFile)

	// Set serf encryption key when given and we're not in dev mode
	if !g.devMode && cfg.Bootstrap.MeshEncryptionKey != "" {
		ms.SetEncryptionKey(cfg.Bootstrap.MeshEncryptionKey)
	}

	var wgListenAddr net.IP
	if cfg.Wireguard.ListenAddr == "" {

		st := meshservice.NewSTUNService()
		ips, err := st.GetExternalIP()

		if err != nil {
			return err
		}
		if len(ips) > 0 {
			wgListenAddr = ips[0]
			log.WithField("ip", wgListenAddr).Info("Using external IP when connecting with mesh")
		}
	}
	if wgListenAddr == nil {
		wgListenAddr = getIPFromIPOrIntfParam(cfg.Wireguard.ListenAddr)
		log.WithField("ip", wgListenAddr).Trace("parsed -listen-addr")
		if wgListenAddr == nil {
			return errors.New("need -listen-addr")
		}

	}

	_, cidrRangeIpnet, err := net.ParseCIDR(cfg.Bootstrap.MeshCIDRRange)
	if err != nil {
		return err
	}
	ms.CIDRRange = *cidrRangeIpnet

	if cfg.Bootstrap.MeshIPAMCIDRRange != "" {
		_, cidrRangeIPAMIpnet, err := net.ParseCIDR(cfg.Bootstrap.MeshIPAMCIDRRange)
		if err != nil {
			return err
		}

		// TODO check if this is within cidr range above..

		ms.CIDRRangeIPAM = cidrRangeIPAMIpnet

	}

	// MeshIP ist composed of what user specifies using -ip, but
	// with the net mask of -cidr. e.g. 10.232.0.0/16 with an
	// IP of 10.232.5.99 becomes 10.232.5.99/16
	ms.MeshIP = net.IPNet{
		IP:   net.ParseIP(cfg.Bootstrap.NodeIP),
		Mask: ms.CIDRRange.Mask,
	}
	log.WithField("meship", ms.MeshIP).Trace("using mesh ip")

	// - prepare wireguard interface
	pk, err := g.wireguardSetup(&ms, wgListenAddr)
	if err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}
	// remove wg interface in all cases - at errors
	// or at the end of this func.
	defer func() {
		ms.RemoveWireguardInterfaceForMesh()
	}()

	// set up serf
	err = g.serfSetup(&ms, pk, wgListenAddr)
	if err != nil {
		return err
	}

	// set up external gRPC interface, be able to listen
	// for join requests
	if err = g.grpcSetup(&ms); err != nil {
		return err
	}

	// we created this mesh now
	ms.SetTimestamps(time.Now().Unix(), time.Now().Unix())

	// print out user information on how to connect to this mesh

	fmt.Printf("** \n")
	fmt.Printf("** Mesh '%s' has been bootstrapped. Other nodes can join now.\n", cfg.MeshName)
	fmt.Printf("** \n")
	fmt.Printf("** Mesh name:                       %s\n", cfg.MeshName)
	fmt.Printf("** Mesh CIDR range:                 %s\n", ms.CIDRRange.String())
	fmt.Printf("** gRPC Service listener endpoint:  %s:%d\n", ms.GrpcBindAddr, ms.GrpcBindPort)
	fmt.Printf("** This node's name:                %s\n", ms.NodeName)
	fmt.Printf("** This node's mesh IP:             %s\n", ms.MeshIP.IP.String())
	if cfg.Bootstrap.MemberlistFile != "" {
		fmt.Printf("** Mesh node details export to:     %s\n", cfg.Bootstrap.MemberlistFile)
	}
	fmt.Printf("** \n")
	if g.devMode {
		fmt.Printf("** This mesh is running in DEVELOPMENT MODE without encryption.\n")
		fmt.Printf("** Do not use this in a production setup.\n")
		fmt.Printf("** \n")
		fmt.Printf("** To have another node join this mesh, use this command:\n")
		ba := ms.GrpcBindAddr
		if ba == "0.0.0.0" {
			ba = "<IP_OF_THIS_NODE>"
		}
		fmt.Printf("** wgmesh join -v -dev -n %s -bootstrap-addr %s:%d\n", cfg.MeshName, ba, ms.GrpcBindPort)
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

	// wait until stopped
	g.wait()

	// clean up everything
	if err = g.cleanUp(&ms); err != nil {
		return err
	}

	return nil
}

// wireguardSetup creates the wireguard interface from parameters. Returns
// the private key to be shared with the mesh
func (g *BootstrapCommand) wireguardSetup(ms *meshservice.MeshService, wgListenAddr net.IP) (pk string, err error) {
	cfg := g.meshConfig

	// From the given IP and listen port, create the wireguard interface
	// and set up a basic configuration for it. Up the interface
	pk, err = ms.CreateWireguardInterfaceForMesh(cfg.Bootstrap.NodeIP, cfg.Wireguard.ListenPort)
	if err != nil {
		return "", err
	}
	ms.WireguardPubKey = pk
	ms.WireguardListenPort = cfg.Wireguard.ListenPort
	ms.WireguardListenIP = wgListenAddr

	// add a route so that all traffic regarding fiven cidr range
	// goes to the wireguard interface.
	err = ms.SetRoute()
	if err != nil {
		return "", err
	}

	ms.SetNodeName(g.meshConfig.NodeName)

	return pk, nil
}

// serfSetup initializes the serf cluster from parameters
func (g *BootstrapCommand) serfSetup(ms *meshservice.MeshService, pk string, wgListenAddr net.IP) (err error) {
	cfg := g.meshConfig

	// create and start the serf cluster
	ms.NewSerfCluster(cfg.Bootstrap.SerfModeLAN)

	err = ms.StartSerfCluster(
		true,
		pk,
		wgListenAddr.String(),
		cfg.Wireguard.ListenPort,
		ms.MeshIP.IP.String())
	if err != nil {
		return err
	}

	ms.StartStatsUpdater()

	return nil
}

// GrpcSetup ...
func (g *BootstrapCommand) grpcSetup(ms *meshservice.MeshService) (err error) {
	cfg := g.meshConfig

	// set up TLS config from parameter unless we're in dev mode
	if !g.devMode {
		ms.TLSConfig, err = meshservice.NewTLSConfigFromFiles(
			cfg.Bootstrap.GRPCTLSConfig.GRPCCaCert,
			cfg.Bootstrap.GRPCTLSConfig.GRPCCaPath,
			cfg.Bootstrap.GRPCTLSConfig.GRPCServerCert,
			cfg.Bootstrap.GRPCTLSConfig.GRPCServerKey)
		if err != nil {
			return err
		}
	}

	// set up grpc mesh service
	ms.GrpcBindAddr = cfg.Bootstrap.GRPCBindAddr
	ms.GrpcBindPort = cfg.Bootstrap.GRPCBindPort

	go func() {
		log.Infof("Starting gRPC mesh Service at %s:%d", ms.GrpcBindAddr, ms.GrpcBindPort)
		err = ms.StartGrpcService()
		if err != nil {
			log.Error(err)
		}
	}()

	// start the local agent if argument is given
	if cfg.Agent.GRPCBindSocket != "" {
		ms.MeshAgentServer = meshservice.NewMeshAgentServerSocket(ms, cfg.Agent.GRPCBindSocket, cfg.Agent.GRPCBindSocketIDs)
		log.WithField("mas", ms.MeshAgentServer).Trace("agent")
		go func() {
			log.Infof("Starting gRPC Agent Service at %s", cfg.Agent.GRPCBindSocket)
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

// CleanUp takes down all internal services and cleans up
// interfaces, sockets etc.
func (g *BootstrapCommand) cleanUp(ms *meshservice.MeshService) error {
	cfg := g.meshConfig

	ms.MeshAgentServer.StopAgentGrpcService()

	ms.LeaveSerfCluster()

	ms.StopGrpcService()

	// delete memberlist-file
	os.Remove(cfg.Bootstrap.MemberlistFile)

	// Wireguard will be removed by deferred func

	return nil
}
