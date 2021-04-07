package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	config "github.com/aschmidt75/wgmesh/config"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// RTTCommand struct
type RTTCommand struct {
	CommandDefaults

	fs *flag.FlagSet

	// configuration file
	config string
	// configuration struct
	meshConfig config.Config
}

// NewRTTCommand creates the Tag Command
func NewRTTCommand() *RTTCommand {
	c := &RTTCommand{
		CommandDefaults: NewCommandDefaults(),
		config:          envStrWithDefault("WGMESH_CONFIG", ""),
		meshConfig:      config.NewDefaultConfig(),
		fs:              flag.NewFlagSet("rtt", flag.ContinueOnError),
	}

	c.fs.StringVar(&c.config, "config", c.config, "file name of config file (optional).\nenv:WGMESH_cONFIG")
	c.fs.StringVar(&c.meshConfig.Agent.GRPCSocket, "agent-grpc-socket", c.meshConfig.Agent.GRPCSocket, "agent socket to dial")
	c.DefaultFields(c.fs)

	return c
}

// Name returns the name of the command
func (g *RTTCommand) Name() string {
	return g.fs.Name()
}

// Init sets up the command struct from arguments
func (g *RTTCommand) Init(args []string) error {
	err := g.fs.Parse(args)
	if err != nil {
		return err
	}
	g.ProcessDefaults()

	// load config file if we have one
	if g.config != "" {
		err = g.meshConfig.LoadConfigFromFile(g.config)
		if err != nil {
			log.WithError(err).Error("Config read error")
			return fmt.Errorf("Unable to read configuration from %s", g.config)
		}
	}

	err = g.fs.Parse(args)
	if err != nil {
		return err
	}
	log.WithField("cfg", g.meshConfig).Trace("Read")
	log.WithField("cfg.agent", g.meshConfig.Agent).Trace("Read")

	return nil
}

// Run queries the agent for RTT info
func (g *RTTCommand) Run() error {
	log.WithField("g", g).Trace(
		"Running cli command",
	)

	//
	endpoint := fmt.Sprintf("unix://%s", g.meshConfig.Agent.GRPCSocket)

	conn, err := grpc.Dial(endpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Error(err)
		return fmt.Errorf("cannot connect to %s", endpoint)
	}
	defer conn.Close()

	agent := meshservice.NewAgentClient(conn)
	log.WithField("agent", agent).Trace("got grpc service client")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := agent.RTT(ctx, &meshservice.AgentEmpty{})
	if err != nil {
		log.WithError(err).Error("Unable to query RTTs from agent")
	}

	allNames := make([]string, 0)
	res := make(map[string]map[string]int)
	for {
		rttInfo, err := r.Recv()
		if err != nil {
			break
		}

		log.WithField("r", rttInfo).Trace("Got response")
		allNames = append(allNames, rttInfo.NodeName)
		res[rttInfo.NodeName] = make(map[string]int)

		for _, nodeInfo := range rttInfo.Rtts {
			elem := res[rttInfo.NodeName]
			elem[nodeInfo.NodeName] = int(nodeInfo.RttMsec)
		}

	}
	log.WithField("res", res).Trace("results")

	// sort allNames

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
	line := "/"

	for _, colsName := range allNames {
		line = fmt.Sprintf("%s\t%s", line, colsName)
	}
	line = fmt.Sprintf("%s\t", line)

	fmt.Fprintln(w, line)

	for _, rowsName := range allNames {
		line := rowsName

		for _, colsName := range allNames {
			line = fmt.Sprintf("%s\t%d", line, res[rowsName][colsName])
		}

		line = fmt.Sprintf("%s\t", line)
		fmt.Fprintln(w, line)

	}
	w.Flush()

	return err
}
