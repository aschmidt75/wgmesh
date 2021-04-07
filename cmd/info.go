package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	config "github.com/aschmidt75/wgmesh/config"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// InfoCommand struct
type InfoCommand struct {
	CommandDefaults

	fs *flag.FlagSet

	// configuration file
	config string
	// configuration struct
	meshConfig config.Config

	// options not in config, only from parameters
	watchFlag bool
}

// NewInfoCommand creates the Info Command structure and sets the parameters
func NewInfoCommand() *InfoCommand {
	c := &InfoCommand{
		CommandDefaults: NewCommandDefaults(),
		config:          envStrWithDefault("WGMESH_CONFIG", ""),
		meshConfig:      config.NewDefaultConfig(),
		fs:              flag.NewFlagSet("info", flag.ContinueOnError),
		watchFlag:       false,
	}

	c.fs.StringVar(&c.config, "config", c.config, "file name of config file (optional).\nenv:WGMESH_cONFIG")
	c.fs.StringVar(&c.meshConfig.Agent.GRPCSocket, "agent-grpc-socket", c.meshConfig.Agent.GRPCSocket, "agent socket to dial")
	c.fs.BoolVar(&c.watchFlag, "watch", c.watchFlag, "watch for changes until interrupted")

	c.DefaultFields(c.fs)

	return c
}

// Name returns the name of the command
func (g *InfoCommand) Name() string {
	return g.fs.Name()
}

// Init sets up the command struct from arguments
func (g *InfoCommand) Init(args []string) error {
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

// Run queries the agent for Info info
func (g *InfoCommand) Run() error {
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

	ctx0, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if g.watchFlag {

		g.singleCycle(ctx0, agent)

		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			r, err := agent.WaitForChangeInMesh(ctx, &meshservice.WaitInfo{
				TimeoutSecs: 10,
			})
			if err != nil {
				log.WithError(err).Error("error while waiting for mesh changes")
				break
			}
			for {
				wr, err := r.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.WithError(err).Debug("error while waiting for mesh changes")
					break
				}
				log.WithField("wr", wr).Trace(".")

				if wr.WasTimeout {
					continue
				}
				if wr.ChangesOccured {
					time.Sleep(1 * time.Second)
					g.singleCycle(ctx, agent)
				}

			}
		}

		return nil
	}

	return g.singleCycle(ctx0, agent)
}

func (g *InfoCommand) singleCycle(ctx context.Context, agent meshservice.AgentClient) error {

	meshInfo, err := agent.Info(ctx, &meshservice.AgentEmpty{})
	if err != nil {
		log.WithError(err).Error("Unable to query infos from agent")
		return err
	}

	fmt.Printf("Mesh '%s' has %d nodes, started %s\n", meshInfo.Name, meshInfo.NodeCount, time.Unix(int64(meshInfo.MeshCeationTS), 0))
	fmt.Printf("This node '%s' joined %s\n", meshInfo.NodeName, time.Unix(int64(meshInfo.NodeJoinTS), 0))

	r, err := agent.Nodes(ctx, &meshservice.AgentEmpty{})
	if err != nil {
		log.WithError(err).Error("Unable to query nodes from agent")
	}

	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)

	fmt.Fprintln(w, "Name\tAddress\tStatus\tRTT\tTags\t")

	for {
		memberInfo, err := r.Recv()
		if err != nil {
			break
		}

		tagStr := ""
		for _, tag := range memberInfo.Tags {
			if tag.Key[0] != '_' {
				tagStr = fmt.Sprintf("%s %s=%s,", tagStr, tag.Key, tag.Value)
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t\n", memberInfo.NodeName, memberInfo.Addr, memberInfo.Status, memberInfo.RttMsec, tagStr)
	}
	w.Flush()

	return nil
}
