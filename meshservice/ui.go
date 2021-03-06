package meshservice

import (
	context "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	sync "sync"
	"time"

	rice "github.com/GeertJohan/go.rice"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	grpc "google.golang.org/grpc"
)

// UIServer  ...
type UIServer struct {
	agentGrpcSocket string
	httpBindAddr    string
	httpBindPort    int
	conf            rice.Config
	box             *rice.Box

	meshInfo    *MeshInfo
	members     []*MemberInfo
	m           sync.Mutex
	lastUpdated time.Time
}

// NewUIServer ...
func NewUIServer(agentGrpcSocket string, httpBindAddr string, httpBindPort int) *UIServer {
	conf := rice.Config{
		LocateOrder: []rice.LocateMethod{rice.LocateAppended, rice.LocateFS},
	}
	box, err := conf.FindBox("../web/dist")
	if err != nil {
		log.WithError(err).Fatalf("unable to serve web interface\n", err)
	}

	log.WithField("box", box).Trace("Loaded assets")

	return &UIServer{
		agentGrpcSocket: agentGrpcSocket,
		httpBindAddr:    httpBindAddr,
		httpBindPort:    httpBindPort,
		box:             box,
		conf:            conf,
		meshInfo:        &MeshInfo{},
		members:         make([]*MemberInfo, 0),
		lastUpdated:     time.Now(),
	}
}

// Serve starts the HTTP server and the agent query
func (u *UIServer) Serve() {

	http.Handle("/", http.FileServer(u.box.HTTPBox()))
	http.HandleFunc("/api/nodes", u.apiNodesHandler)
	http.HandleFunc("/api/mesh", u.apiMeshHandler)
	http.Handle("/api/updates", websocket.Handler(u.updater))

	listenSpec := fmt.Sprintf("%s:%d", u.httpBindAddr, u.httpBindPort)

	fmt.Printf("Serving files on %s, press ctrl-C to exit\n", listenSpec)
	go func() {
		err := http.ListenAndServe(listenSpec, nil)
		if err != nil {
			log.WithError(err).Fatalf("error serving files")
		}
	}()

	go func() {
		err := u.agentUpdater()
		if err != nil {
			log.WithError(err).Fatalf("Unable to query meshervice agent for updates")
		}
	}()

	select {}

}

// simple websocket updater
func (u *UIServer) updater(conn *websocket.Conn) {
	lastUpdated := time.Now()
	for {

		l := func(u *UIServer) time.Time {
			u.m.Lock()
			defer u.m.Unlock()

			return u.lastUpdated
		}(u)

		if l.After(lastUpdated) {

			type wsUpdateStruct struct {
				// Aspect decribes what has changes
				Aspect string `json:"a"`
				// Ts is the timestamp
				Ts string `json:"ts"`
			}
			upd := wsUpdateStruct{
				Aspect: "nodes",
				Ts:     l.Format(time.UnixDate),
			}
			updJSON, err := json.Marshal(upd)
			if err == nil {
				_, err := conn.Write(updJSON)
				if err != nil {
					log.WithError(err).Error("Error sending ws update")
					return
				}
			}

			lastUpdated = l
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// returns all nodes
func (u *UIServer) apiNodesHandler(w http.ResponseWriter, req *http.Request) {
	u.m.Lock()
	defer u.m.Unlock()

	type uiNodeInfo struct {
		Name    string            `json:"name"`
		MeshIP  string            `json:"meshIP"`
		Tags    map[string]string `json:"tags"`
		RttMsec int32             `json:"rttMsec"`
		IsSelf  bool              `json:"isSelf"`
	}

	type uiNodes struct {
		Nodes []uiNodeInfo `json:"nodes"`
	}

	nodes := uiNodes{
		Nodes: make([]uiNodeInfo, len(u.members)),
	}
	for idx, member := range u.members {
		log.WithField("m", member).Trace(".")
		nodes.Nodes[idx] = uiNodeInfo{
			Name:    member.NodeName,
			MeshIP:  member.Addr,
			Tags:    make(map[string]string),
			RttMsec: member.RttMsec,
			IsSelf:  false,
		}
		for _, tag := range member.Tags {
			m := nodes.Nodes[idx].Tags
			m[tag.Key] = tag.Value
		}
	}

	w.Header().Add("Content-Type", "application/json")

	bytes, err := json.Marshal(nodes)
	if err != nil {
		log.WithError(err).Error("Unable to marshal nodelist as json")
		fmt.Fprintf(w, "[]")
	}

	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(bytes)))
	w.Write(bytes)
}

// returns mesh info
func (u *UIServer) apiMeshHandler(w http.ResponseWriter, req *http.Request) {
	u.m.Lock()
	defer u.m.Unlock()

	if u.meshInfo == nil {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Content-Length", fmt.Sprintf("%d", 0))
		return
	}

	type uiMeshInfo struct {
		Name      string `json:"name"`
		NodeName  string `json:"thisNodeName"`
		NodeCount int    `json:"nodeCount"`
	}

	meshInfo := uiMeshInfo{
		Name:      u.meshInfo.Name,
		NodeName:  u.meshInfo.NodeName,
		NodeCount: int(u.meshInfo.NodeCount),
	}

	w.Header().Add("Content-Type", "application/json")

	bytes, err := json.Marshal(meshInfo)
	if err != nil {
		log.WithError(err).Error("Unable to marshal mesh info as json")
		fmt.Fprintf(w, "[]")
	}

	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(bytes)))
	w.Write(bytes)
}

// watches for changes in mesh via agent grpc socket
func (u *UIServer) agentUpdater() error {
	endpoint := fmt.Sprintf("unix://%s", u.agentGrpcSocket)

	conn, err := grpc.Dial(endpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Error(err)
		return fmt.Errorf("cannot connect to %s", endpoint)
	}
	defer conn.Close()

	agent := NewAgentClient(conn)
	log.WithField("agent", agent).Trace("got grpc service client")

	ctx0, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	u.singleCycle(ctx0, agent)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		r, err := agent.WaitForChangeInMesh(ctx, &WaitInfo{
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

			if wr.WasTimeout {
				continue
			}
			if wr.ChangesOccured {
				time.Sleep(1 * time.Second)
				u.singleCycle(ctx, agent)
			}

		}
	}

	return nil
}

func (u *UIServer) singleCycle(ctx context.Context, agent AgentClient) error {
	u.m.Lock()
	defer u.m.Unlock()

	meshInfo, err := agent.Info(ctx, &AgentEmpty{})
	if err != nil {
		log.WithError(err).Error("Unable to query infos from agent")
		return err
	}
	u.meshInfo = meshInfo

	r, err := agent.Nodes(ctx, &AgentEmpty{})
	if err != nil {
		log.WithError(err).Error("Unable to query nodes from agent")
	}

	u.members = make([]*MemberInfo, meshInfo.NodeCount)
	idx := 0
	for {
		memberInfo, err := r.Recv()
		if err != nil {
			break
		}
		u.members[idx] = memberInfo
		idx = idx + 1
	}
	log.WithField("u.members", u.members).Trace("query")

	u.lastUpdated = time.Now()

	return nil
}
