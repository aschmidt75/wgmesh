package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// TagsCommand struct
type TagsCommand struct {
	CommandDefaults

	fs              *flag.FlagSet
	tagStr          string
	deleteFlag      string
	agentGrpcSocket string
}

// NewTagsCommand creates the Tag Command
func NewTagsCommand() *TagsCommand {
	c := &TagsCommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("tags", flag.ContinueOnError),
		tagStr:          "",
		deleteFlag:      "",
		agentGrpcSocket: "/var/run/wgmesh.sock",
	}

	c.fs.StringVar(&c.tagStr, "set", c.tagStr, "set tag key=value")
	c.fs.StringVar(&c.deleteFlag, "delete", c.deleteFlag, "to delete a key")
	c.fs.StringVar(&c.agentGrpcSocket, "agent-grpc-socket", c.agentGrpcSocket, "agent socket to dial")

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
	endpoint := fmt.Sprintf("unix://%s", g.agentGrpcSocket)

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
