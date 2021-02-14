package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"strings"
	"time"

	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// TagsCommand struct
type TagsCommand struct {
	CommandDefaults

	fs            *flag.FlagSet
	tagStr        string
	deleteFlag    string
	agentGrpcAddr string
	agentGrpcPort int
}

// NewTagsCommand creates the Tag Command
func NewTagsCommand() *TagsCommand {
	c := &TagsCommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("tags", flag.ContinueOnError),
		tagStr:          "",
		deleteFlag:      "",
		agentGrpcAddr:   "127.0.0.1",
		agentGrpcPort:   5001,
	}

	c.fs.StringVar(&c.tagStr, "set", c.tagStr, "set tag key=value")
	c.fs.StringVar(&c.deleteFlag, "delete", c.deleteFlag, "to delete a key")
	c.fs.StringVar(&c.agentGrpcAddr, "agent-grpc-addr", c.agentGrpcAddr, "address of agent to dial")
	c.fs.IntVar(&c.agentGrpcPort, "agent-grpc-port", c.agentGrpcPort, "port of agent to dial")

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

	if g.tagStr == "" && g.deleteFlag == "" {
		return errors.New("Either set a tag using -set key=value or delete a tag using -delete=key")
	}

	if g.tagStr != "" {
		arr := strings.Split(g.tagStr, "=")
		if len(arr) != 2 {
			return errors.New("Set a tag using -tag key=value")
		}
	}

	if net.ParseIP(g.agentGrpcAddr) == nil {
		return fmt.Errorf("%s is not a valid ip for -agent-grpc-addr", g.agentGrpcAddr)
	}

	if g.agentGrpcPort < 0 || g.agentGrpcPort > 65535 {
		return fmt.Errorf("%d is not valid for -agent-grpc-port", g.agentGrpcPort)
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
	endpoint := fmt.Sprintf("%s:%d", g.agentGrpcAddr, g.agentGrpcPort)

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

	if g.deleteFlag != "" {
		r, err := agent.Untag(ctx, &meshservice.TagRequest{
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

	} else {

		arr := strings.Split(g.tagStr, "=")

		r, err := agent.Tag(ctx, &meshservice.TagRequest{
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
