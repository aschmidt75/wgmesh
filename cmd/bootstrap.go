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

	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
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
	grpcServerKey          string
	grpcServerCert         string
	grpcCaCert             string
	grpcCaPath             string
	memberListFile         string
	agentGrpcBindSocket    string
	agentGrpcBindSocketIDs string
	meshEncryptionKey      string
	devMode                bool
}

// NewBootstrapCommand creates the Bootstrap Command
func NewBootstrapCommand() *BootstrapCommand {
	c := &BootstrapCommand{
		CommandDefaults:        NewCommandDefaults(),
		fs:                     flag.NewFlagSet("bootstrap", flag.ContinueOnError),
		meshName:               envStrWithDefault("WGMESH_MESH_NAME", ""),
		nodeName:               envStrWithDefault("WGMESH_NODE_NAME", ""),
		cidrRange:              envStrWithDefault("WGMESH_CIDR_RANGE", "10.232.0.0/16"),
		ip:                     envStrWithDefault("WGMESH_MESH_IP", "10.232.1.1"),
		wgListenAddr:           envStrWithDefault("WGMESH_WIREGUARD_LISTEN_ADDR", ""),
		wgListenPort:           envIntWithDefault("WGMESH_WIREGUARD_LISTEN_PORT", 54540),
		grpcBindAddr:           envStrWithDefault("WGMESH_GRPC_BIND_ADDR", "0.0.0.0"),
		grpcBindPort:           envIntWithDefault("WGMESH_GRPC_BIND_PORT", 5000),
		grpcServerKey:          envStrWithDefault("WGMESH_SERVER_KEY", ""),
		grpcServerCert:         envStrWithDefault("WGMESH_SERVER_CERT", ""),
		grpcCaCert:             envStrWithDefault("WGMESH_CA_CERT", ""),
		grpcCaPath:             envStrWithDefault("WGMESH_CA_PATH", ""),
		agentGrpcBindSocket:    envStrWithDefault("WGMESH_AGENT_BIND_SOCKET", "/var/run/wgmesh.sock"),
		agentGrpcBindSocketIDs: envStrWithDefault("WGMESH_AGENT_BIND_SOCKET_ID", ""),
		memberListFile:         envStrWithDefault("WGMESH_MEMBERLIST_FILE", ""),
		meshEncryptionKey:      envStrWithDefault("WGMESH_ENCRYPTION_KEY", ""),
		devMode:                false,
	}

	c.fs.StringVar(&c.meshName, "name", c.meshName, "name of the mesh network.\nenv:WGMESH_MESH_NAME")
	c.fs.StringVar(&c.meshName, "n", c.meshName, "name of the mesh network (short).\nenv:WGMESH_MESH_NAME")
	c.fs.StringVar(&c.nodeName, "node-name", c.nodeName, "(optional) name of this node.\nenv:WGMESH_NODE_NAME")
	c.fs.StringVar(&c.cidrRange, "cidr", c.cidrRange, "CIDR range of this mesh (internal ips).\nenv:WGMESH_CIDR_RANGE")
	c.fs.StringVar(&c.ip, "ip", c.ip, "internal ip of the bootstrap node. Must be set fixed for bootstrap nodes.\nenv:WGMESH_MESH_IP")
	c.fs.StringVar(&c.wgListenAddr, "listen-addr", c.wgListenAddr, "external wireguard ip.\nenv:WGMESH_WIREGUARD_LISTEN_ADDR")
	c.fs.IntVar(&c.wgListenPort, "listen-port", c.wgListenPort, "set the (external) wireguard listen port.\nenv:WGMESH_WIREGUARD_LISTEN_PORT")
	c.fs.StringVar(&c.grpcBindAddr, "grpc-bind-addr", c.grpcBindAddr, "(public) address to bind grpc mesh service to.\nenv:WGMESH_GRPC_BIND_ADDR")
	c.fs.IntVar(&c.grpcBindPort, "grpc-bind-port", c.grpcBindPort, "port to bind grpc mesh service to.\nenv:WGMESH_GRPC_BIND_PORT")
	c.fs.StringVar(&c.agentGrpcBindSocket, "agent-grpc-bind-socket", c.agentGrpcBindSocket, "local socket file to bind grpc agent to.\nenv:WGMESH_AGENT_BIND_SOCKET")
	c.fs.StringVar(&c.agentGrpcBindSocketIDs, "agent-grpc-bind-socket-id", c.agentGrpcBindSocketIDs, "<uid:gid> to change bind socket to.\nenv:WGMESH_AGENT_BIND_SOCKET_ID")
	c.fs.StringVar(&c.grpcServerKey, "grpc-server-key", c.grpcServerKey, "points to PEM-encoded private key to be used by grpc server.\nenv:WGMESH_SERVER_KEY")
	c.fs.StringVar(&c.grpcServerCert, "grpc-server-cert", c.grpcServerCert, "points to PEM-encoded certificate be used by grpc server.\nenv:WGMESH_SERVER_CERT")
	c.fs.StringVar(&c.grpcCaCert, "grpc-ca-cert", c.grpcCaCert, "points to PEM-encoded CA certificate.\nenv:WGMESH_CA_CERT")
	c.fs.StringVar(&c.grpcCaPath, "grpc-ca-path", c.grpcCaPath, "points to a directory containing PEM-encoded CA certificates.\nenv:WGMESH_CA_PATH")
	c.fs.StringVar(&c.memberListFile, "memberlist-file", c.memberListFile, "optional name of file for a log of all current mesh members.\nenv:WGMESH_MEMBERLIST_FILE")
	c.fs.StringVar(&c.meshEncryptionKey, "mesh-encryption-key", c.meshEncryptionKey, "optional key for symmetric encryption of internal mesh traffic. Must be 32 Bytes base64-ed.\nenv:WGMESH_ENCRYPTION_KEY")
	c.fs.BoolVar(&c.devMode, "dev", c.devMode, "Enables development mode which runs without encryption, authentication and without TLS")
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

	if g.meshName != "" && len(g.meshName) > 10 {
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

	if g.meshEncryptionKey != "" {
		b, err := base64.StdEncoding.DecodeString(g.meshEncryptionKey)
		if err != nil || len(b) != 32 {
			return fmt.Errorf("%s is not valid for -mesh-encryption-key, must be 32 bytes, base64-encoded", g.meshEncryptionKey)
		}
	}

	withGrpcSecure := false
	if g.grpcServerKey != "" {
		withGrpcSecure = true

		if !fileExists(g.grpcServerKey) {
			return fmt.Errorf("%s not found for -grpc-server-key", g.grpcServerKey)
		}
	}
	if g.grpcServerCert != "" {
		withGrpcSecure = true

		if !fileExists(g.grpcServerCert) {
			return fmt.Errorf("%s not found for -grpc-server-cert", g.grpcServerCert)
		}
	}
	if g.grpcCaCert != "" {
		withGrpcSecure = true

		if !fileExists(g.grpcCaCert) {
			return fmt.Errorf("%s not found for -grpc-ca-cert", g.grpcCaCert)
		}
	}
	if g.grpcCaPath != "" {
		withGrpcSecure = true

		if !dirExists(g.grpcCaPath) {
			return fmt.Errorf("%s not found for -grpc-ca-path", g.grpcCaPath)
		}
	}

	if withGrpcSecure {
		if g.grpcServerKey == "" || g.grpcServerCert == "" || (g.grpcCaCert == "" && g.grpcCaPath == "") {
			//
			return fmt.Errorf("-grpc-server-key, -grpc-server-cert, -grpc-ca-cert / -grp-ca-path must be specified together")
		}
	}

	if g.grpcCaCert != "" && g.grpcCaPath != "" {
		return fmt.Errorf("-grpc-ca-cert / -grp-ca-path are mutually exclusive")
	}

	if g.devMode {
		if withGrpcSecure || g.meshEncryptionKey != "" {
			return fmt.Errorf("cannot combine security parameters in -dev mode")
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

	// if mesh name is empty
	if g.meshName == "" {
		g.meshName = randomMeshName()
		log.WithField("meshName", g.meshName).Warn("auto-generated mesh name. Use this as -n parameter when joining the mesh.")
	}

	ms := meshservice.NewMeshService(g.meshName)
	log.WithField("ms", ms).Trace(
		"created",
	)
	ms.SetMemberlistExportFile(g.memberListFile)

	// Set serf encryption key when given and we're not in dev mode
	if !g.devMode && g.meshEncryptionKey != "" {
		ms.SetEncryptionKey(g.meshEncryptionKey)
	}

	var wgListenAddr net.IP
	if g.wgListenAddr == "" {

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
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	err = g.serfSetup(&ms, pk, wgListenAddr)
	if err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	if err = g.grpcSetup(&ms); err != nil {
		err2 := ms.RemoveWireguardInterfaceForMesh()
		if err2 != nil {
			return err2
		}
		return err
	}

	// we created this mesh now
	ms.SetTimestamps(time.Now().Unix(), time.Now().Unix())

	// print out user information on how to connect to this mesh

	fmt.Printf("** \n")
	fmt.Printf("** Mesh '%s' has been bootstrapped. Other nodes can join now.\n", g.meshName)
	fmt.Printf("** \n")
	fmt.Printf("** Mesh name:                       %s\n", g.meshName)
	fmt.Printf("** Mesh CIDR range:                 %s\n", ms.CIDRRange.String())
	fmt.Printf("** gRPC Service listener endpoint:  %s:%d\n", ms.GrpcBindAddr, ms.GrpcBindPort)
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
		fmt.Printf("** To have another node join this mesh, use this command (only for non-NATed setups with public ip):\n")
		ba := ms.GrpcBindAddr
		if ba == "0.0.0.0" {
			ba = "<PUBLIC_IP_OF_THIS_NODE>"
		}
		fmt.Printf("** wgmesh join -v -dev -n %s -bootstrap-addr %s:%d\n", g.meshName, ba, ms.GrpcBindPort)
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

// serfSetup initializes the serf cluster from parameters
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

	// set up TLS config from parameter unless we're in dev mode
	if !g.devMode {
		ms.TLSConfig, err = meshservice.NewTLSConfigFromFiles(g.grpcCaCert, g.grpcCaPath, g.grpcServerCert, g.grpcServerKey)
		if err != nil {
			return err
		}
	}

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

// CleanUp takes down all internal services and cleans up
// interfaces, sockets etc.
func (g *BootstrapCommand) cleanUp(ms *meshservice.MeshService) error {

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
