package cmd

import (
	"flag"
	"fmt"

	config "github.com/aschmidt75/wgmesh/config"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
)

// UICommand struct
type UICommand struct {
	CommandDefaults

	fs *flag.FlagSet

	// configuration file
	config string
	// configuration struct
	meshConfig config.Config

	// options not in config, only from parameters
}

// NewUICommand creates the UI Command structure and sets the parameters
func NewUICommand() *UICommand {
	c := &UICommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("ui", flag.ContinueOnError),
		config:          envStrWithDefault("WGMESH_CONFIG", ""),
		meshConfig:      config.NewDefaultConfig(),
	}

	c.fs.StringVar(&c.config, "config", c.config, "file name of config file (optional).\nenv:WGMESH_cONFIG")
	c.fs.StringVar(&c.meshConfig.Agent.GRPCSocket, "agent-grpc-socket", c.meshConfig.Agent.GRPCSocket, "agent socket to dial")
	c.fs.StringVar(&c.meshConfig.UI.HTTPBindAddr, "http-bind-addr", c.meshConfig.UI.HTTPBindAddr, "HTTP bind address")
	c.fs.IntVar(&c.meshConfig.UI.HTTPBindPort, "http-bind-port", c.meshConfig.UI.HTTPBindPort, "HTTP bind port")

	c.DefaultFields(c.fs)

	return c
}

// Name returns the name of the command
func (g *UICommand) Name() string {
	return g.fs.Name()
}

// Init sets up the command struct from arguments
func (g *UICommand) Init(args []string) error {
	err := g.fs.Parse(args)
	if err != nil {
		return err
	}
	g.ProcessDefaults()

	// load config file if we have one
	if g.config != "" {
		g.meshConfig, err = config.NewConfigFromFile(g.config)
		if err != nil {
			log.WithError(err).Error("Config read error")
			return fmt.Errorf("Unable to read configuration from %s", g.config)
		}
	}

	return nil
}

// Run starts an http server to serve the user interface
func (g *UICommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	uiServer := meshservice.NewUIServer(
		g.meshConfig.Agent.GRPCSocket,
		g.meshConfig.UI.HTTPBindAddr,
		g.meshConfig.UI.HTTPBindPort)
	uiServer.Serve()

	return nil
}
