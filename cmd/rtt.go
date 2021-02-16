package cmd

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"text/tabwriter"
	"time"

	meshservice "github.com/aschmidt75/wgmesh/meshservice"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// RTTCommand struct
type RTTCommand struct {
	CommandDefaults

	fs            *flag.FlagSet
	agentGrpcAddr string
	agentGrpcPort int
}

// NewRTTCommand creates the Tag Command
func NewRTTCommand() *RTTCommand {
	c := &RTTCommand{
		CommandDefaults: NewCommandDefaults(),
		fs:              flag.NewFlagSet("rtt", flag.ContinueOnError),
		agentGrpcAddr:   "127.0.0.1",
		agentGrpcPort:   5001,
	}

	c.fs.StringVar(&c.agentGrpcAddr, "agent-grpc-addr", c.agentGrpcAddr, "address of agent to dial")
	c.fs.IntVar(&c.agentGrpcPort, "agent-grpc-port", c.agentGrpcPort, "port of agent to dial")

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

	if net.ParseIP(g.agentGrpcAddr) == nil {
		return fmt.Errorf("%s is not a valid ip for -agent-grpc-addr", g.agentGrpcAddr)
	}

	if g.agentGrpcPort < 0 || g.agentGrpcPort > 65535 {
		return fmt.Errorf("%d is not valid for -agent-grpc-port", g.agentGrpcPort)
	}

	return nil
}

// Run queries the agent for RTT info
func (g *RTTCommand) Run() error {
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
