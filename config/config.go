package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strconv"
)

// Config is the main Configuration struct
type Config struct {
	// MeshName is the name of the mesh to form or to join
	MeshName string `yaml:"mesh-name"`

	// NodeName is the name of the current node. If not set it
	// will be formed from the mesh ip assigned
	NodeName string `yaml:"node-name"`

	// Bootstrap is the config part for bootstrap mode
	Bootstrap *BootstrapConfig `yaml:"bootstrap,omitempty"`

	// Wireguard is the configuration part for wireguard-related settings
	Wireguard *WireguardConfig `yaml:"wireguard,omitempty"`

	// Agent contains optional agent configuration
	Agent *AgentConfig `yaml:"agent,omitempty"`

	// UI contains web user interface configuration
	UI *UIConfig `yaml:"ui,omitempty"`
}

// BootstrapConfig contains condfiguration parts for bootstrap mode
type BootstrapConfig struct {
	// MeshCIDRRange is the CIDR (e.g. 10.232.0.0/16) to be used for the mesh
	// when assigning new mesh-internal ip addresses
	MeshCIDRRange string `yaml:"mesh-cidr-range"`

	// MeshIPAMCIDRRange is an optional setting where this is a subnet of
	// MeshCIDRRange and IP addresses are assigned only from this range
	MeshIPAMCIDRRange string `yaml:"mesh-ipam-cidr-range"`

	// NodeIP sets the internal mesh ip of this node (e.g. .1 for a given subnet)
	NodeIP string `yaml:"node-ip"`

	// GRPCBindAddr is the ip address where bootstrap node expose their
	// gRPC intnerface and listen for join requests
	GRPCBindAddr string `yaml:"grpc-bind-addr"`

	// GRPCBindPort is the port number where bootstrap node expose their
	// gRPC intnerface and listen for join requests
	GRPCBindPort int `yaml:"grpc-bind-port"`

	// GRPCTLSConfig is the optional TLS settings struct for the gRPC interface
	GRPCTLSConfig *BootstrapGRPCTLSConfig `yaml:"grpc-tls,omitempty"`

	// MemberlistFile is an optional setting. If set, node information is written
	// here periodically
	MemberlistFile string `yaml:"memberlist-file"`

	// MeshEncryptionKey is an optional key for symmetric encryption of internal mesh traffic.
	// Must be 32 Bytes base64-ed.
	MeshEncryptionKey string `yaml:"mesh-encryption-key"`

	// SerfModeLAN activates LAN mode or cluster communication. Default is false (=WAN mode).
	SerfModeLAN bool `yaml:"serf-mode-lan"`
}

// BootstrapGRPCTLSConfig contains settings necessary for configuration TLS for the bootstrap node
type BootstrapGRPCTLSConfig struct {
	// GRPCServerKey points to PEM-encoded private key to be used by grpc server.
	GRPCServerKey string `yaml:"grpc-server-key"`

	// GRPCServerCert points to PEM-encoded certificate be used by grpc server.
	GRPCServerCert string `yaml:"grpc-server-cert"`

	// GRPCCaCert points to PEM-encoded CA certificate.
	GRPCCaCert string `yaml:"grpc-ca-cert"`

	// GRPCCaPath points to a directory containing PEM-encoded CA certificates.
	GRPCCaPath string `yaml:"grpc-ca-path"`
}

// WireguardConfig contains wireguard-related settings
type WireguardConfig struct {
	// ListenAddr is the ip address where wireguard should listen for packets
	ListenAddr string `yaml:"listen-addr"`

	// ListenPort is the (external) wireguard listen port
	ListenPort int `yaml:"listen-port"`
}

// AgentConfig contains settings for the gRPC-based local agent
type AgentConfig struct {
	// GRPCBindSocket is the local socket file to bind grpc agent to.
	GRPCBindSocket string `yaml:"agent-grpc-bind-socket"`

	// GRPCBindSocketIDs of the form <uid:gid> to change bind socket to.
	GRPCBindSocketIDs string `yaml:"agent-grpc-bind-socket-id"`

	// GRPCSocket is the local socket file, used by agent clients.
	GRPCSocket string `yaml:"agent-grpc-socket"`
}

// UIConfig contains config entries for the web user interface
type UIConfig struct {
	HTTPBindAddr string `yaml:"http-bind-addr"`
	HTTPBindPort int    `yaml:"http-bind-port"`
}

// NewConfigFromFile reads yaml config file from given path
func NewConfigFromFile(path string) (Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	c := NewDefaultConfig()
	if err = yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}

	return c, nil
}

// NewDefaultConfig creates a default configuration with valid presets.
// These presets can be used with `-dev` mode.
func NewDefaultConfig() Config {
	return Config{
		MeshName: envStrWithDefault("WGMESH_MESH_NAME", ""),
		NodeName: envStrWithDefault("WGMESH_NODE_NAME", ""),
		Bootstrap: &BootstrapConfig{
			MeshCIDRRange:     envStrWithDefault("WGMESH_CIDR_RANGE", "10.232.0.0/16"),
			MeshIPAMCIDRRange: envStrWithDefault("WGMESH_CIDR_RANGE_IPAM", ""),
			NodeIP:            envStrWithDefault("WGMESH_MESH_IP", "10.232.1.1"),
			GRPCBindAddr:      envStrWithDefault("WGMESH_GRPC_BIND_ADDR", "0.0.0.0"),
			GRPCBindPort:      envIntWithDefault("WGMESH_GRPC_BIND_PORT", 5000),
			GRPCTLSConfig: &BootstrapGRPCTLSConfig{
				GRPCServerKey:  envStrWithDefault("WGMESH_SERVER_KEY", ""),
				GRPCServerCert: envStrWithDefault("WGMESH_SERVER_CERT", ""),
				GRPCCaCert:     envStrWithDefault("WGMESH_CA_CERT", ""),
				GRPCCaPath:     envStrWithDefault("WGMESH_CA_PATH", ""),
			},
			MemberlistFile:    envStrWithDefault("WGMESH_MEMBERLIST_FILE", ""),
			MeshEncryptionKey: envStrWithDefault("WGMESH_ENCRYPTION_KEY", ""),
			SerfModeLAN:       envBoolWithDefault("WGMESH_SERF_MODE_LAN", false),
		},
		Wireguard: &WireguardConfig{
			ListenAddr: envStrWithDefault("WGMESH_WIREGUARD_LISTEN_ADDR", ""),
			ListenPort: envIntWithDefault("WGMESH_WIREGUARD_LISTEN_PORT", 54540),
		},
		Agent: &AgentConfig{
			GRPCBindSocket:    envStrWithDefault("WGMESH_AGENT_BIND_SOCKET", "/var/run/wgmesh.sock"),
			GRPCBindSocketIDs: envStrWithDefault("WGMESH_AGENT_BIND_SOCKET_ID", ""),
			GRPCSocket:        envStrWithDefault("WGMESH_AGENT_SOCKET", "/var/run/wgmesh.sock"),
		},
		UI: &UIConfig{
			HTTPBindAddr: envStrWithDefault("WGMESH_HTTP_BIND_ADDR", "127.0.0.1"),
			HTTPBindPort: envIntWithDefault("WGMESH_HTTP_BIND_PORT", 9095),
		},
	}
}

func envStrWithDefault(key string, defaultValue string) string {
	res := os.Getenv(key)
	if res == "" {
		return defaultValue
	}
	return res
}

func envBoolWithDefault(key string, defaultValue bool) bool {
	res := os.Getenv(key)
	if res == "" {
		return defaultValue
	}
	if res == "1" || res == "true" || res == "on" {
		return true
	}
	return false
}

func envIntWithDefault(key string, defaultValue int) int {
	res := os.Getenv(key)
	if res == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(res)
	if err != nil {
		return -1
	}
	return v
}
