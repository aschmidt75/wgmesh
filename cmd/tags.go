package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	config "github.com/aschmidt75/wgmesh/config"
	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// TagsCommand struct
type TagsCommand struct {
	CommandDefaults

	fs *flag.FlagSet

	// configuration file
	config string
	// configuration struct
	meshConfig config.Config

	// options not in config, only from parameters
	tagStr     string
	deleteFlag string
}

// NewTagsCommand creates the Tag Command
func NewTagsCommand() *TagsCommand {
	c := &TagsCommand{
		CommandDefaults: NewCommandDefaults(),
		config:          envStrWithDefault("WGMESH_CONFIG", ""),
		meshConfig:      config.NewDefaultConfig(),
		fs:              flag.NewFlagSet("tags", flag.ContinueOnError),
		tagStr:          "",
		deleteFlag:      "",
	}

	c.fs.StringVar(&c.config, "config", c.config, "file name of config file (optional).\nenv:WGMESH_cONFIG")
	c.fs.StringVar(&c.tagStr, "set", c.tagStr, "set tag key=value")
	c.fs.StringVar(&c.deleteFlag, "delete", c.deleteFlag, "to delete a key")
	c.fs.StringVar(&c.meshConfig.Agent.GRPCSocket, "agent-grpc-socket", c.meshConfig.Agent.GRPCSocket, "agent socket to dial")

	c.DefaultFields(c.fs)

	return c
}

// Name returns the name of the command
func (g *TagsCommand) Name() string {
	return g.fs.Name()
}

// Init sets up the command struct from arguments
func (g *TagsCommand) Init(args []string) error {
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

	if g.tagStr != "" {
		arr := strings.Split(g.tagStr, "=")
		if len(arr) < 2 {
			return errors.New("Set a tag using -set=key=value")
		}
		if strings.HasPrefix(arr[0], "_") {
			return errors.New("Tag keys may not start with underscore _")
		}
	}

	return nil
}

// Run runs the command by creating the wireguard interface,
// starting the serf cluster and grpc server
func (g *TagsCommand) Run() error {
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

	if g.tagStr == "" && g.deleteFlag == "" {
		// show all tags
		client, err := agent.Tags(ctx, &meshservice.AgentEmpty{})
		if err != nil {
			log.Error(err)
			return fmt.Errorf("cannot communicate with endpoint at %s", endpoint)
		}

		c := 0
		for {
			tag, err := client.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.WithError(err).Debug("error while retrieving tag list")
				break
			}
			if !strings.HasPrefix(tag.Key, "_") {
				fmt.Printf("%s=%s\n", tag.Key, tag.Value)
				c++
			}
		}
		if c == 0 {
			fmt.Println("no tags")
		}
		return nil
	}

	if g.deleteFlag != "" {
		r, err := agent.Untag(ctx, &meshservice.NodeTag{
			Key: g.deleteFlag,
		})
		if err != nil {
			log.Error(err)
			return fmt.Errorf("cannot communicate with endpoint at %s", endpoint)
		}
		log.WithField("r", r).Trace("got tagResponse")

		if r.Ok {
			log.Info("Tag deleted")
		} else {
			log.Error("Tag not deleted")
		}
	}

	if g.tagStr != "" {
		arr := strings.SplitN(g.tagStr, "=", 2)

		r, err := agent.Tag(ctx, &meshservice.NodeTag{
			Key:   arr[0],
			Value: arr[1],
		})
		if err != nil {
			log.Error(err)
			return fmt.Errorf("cannot communicate with endpoint at %s", endpoint)
		}
		log.WithField("r", r).Trace("got tagResponse")

		if r.Ok {
			log.Info("Tag set")
		} else {
			log.Error("Tag not set")
		}

	}

	return nil
}
