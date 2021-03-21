package cmd

import (
	"flag"

	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
)

// UICommand struct
type UICommand struct {
	CommandDefaults

	fs              *flag.FlagSet
	agentGrpcSocket string
}

// NewUICommand creates the UI Command structure and sets the parameters
func NewUICommand() *UICommand {
	c := &UICommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("ui", flag.ContinueOnError),
		agentGrpcSocket: "/var/run/wgmesh.sock",
	}

	c.fs.StringVar(&c.agentGrpcSocket, "agent-grpc-socket", c.agentGrpcSocket, "agent socket to dial")

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

	return nil
}

// Run starts an http server to serve the user interface
func (g *UICommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	uiServer := meshservice.NewUIServer(g.agentGrpcSocket)
	uiServer.Serve()

	return nil
}
