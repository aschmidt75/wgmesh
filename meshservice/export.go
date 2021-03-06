package meshservice

import (
	"encoding/json"
	ioutil "io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"time"

	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
)

// SetMemberlistExportFile sets the file name for an export
// of the current memberlist. If empty no file is written
func (ms *MeshService) SetMemberlistExportFile(f string) {
	ms.memberExportFile = f
}

type exportedMember struct {
	Addr   string            `json:"addr"`
	Status string            `json:"st"`
	RTT    int64             `json:"rtt"`
	Tags   map[string]string `json:"tags"`
}

type exportedService struct {
	Nodes []string          `json:"nodes"`
	Port  int               `json:"port"`
	Tags  map[string]string `json:"tags"`
}

type exportedMemberList struct {
	Members    map[string]exportedMember  `json:"members"`
	Services   map[string]exportedService `json:"services"`
	LastUpdate int64                      `json:"lastUpdate"`
}

func (ms *MeshService) updateMemberExport() {

	if ms.lastUpdatedTS.Unix() <= ms.lastExportedTS.Unix() {
		return
	}
	log.Debug("updateMemberExport")

	e := &exportedMemberList{
		Members:    make(map[string]exportedMember),
		Services:   make(map[string]exportedService),
		LastUpdate: ms.lastUpdatedTS.Unix(),
	}
	myCoord, err := ms.Serf().GetCoordinate()
	if err != nil {
		log.WithError(err).Warn("Unable to get my own coordinate, check config")
		myCoord = nil
	}

	for _, member := range ms.Serf().Members() {
		em := exportedMember{
			Addr:   member.Addr.String(),
			Status: member.Status.String(),
			Tags:   member.Tags,
		}
		// compute RTT if we have all distances
		memberCoord, ok := ms.Serf().GetCachedCoordinate(member.Name)
		if ok && memberCoord != nil {
			d := memberCoord.DistanceTo(myCoord)
			em.RTT = int64(d / time.Millisecond)

			// TODO: for LAN mode add Microseconds as well
		}

		//
		e.Members[member.Name] = em

		// grab tags for service entries, put into service map
		ms.processTagsForMember(&member, e)
	}

	content, err := json.MarshalIndent(e, "", " ")
	if err != nil {
		log.WithError(err).Error("unable to write to file")
	}

	ioutil.WriteFile(ms.memberExportFile, content, 0640)

	ms.lastExportedTS = ms.lastUpdatedTS
}

func (ms *MeshService) processTagsForMember(member *serf.Member, e *exportedMemberList) {
	svcKeyRe := regexp.MustCompile(`^svc:`)
	for k, v := range member.Tags {
		arr := svcKeyRe.Split(k, 2)
		if arr != nil && len(arr) == 2 && arr[1] != "" {

			expSvc, ex := e.Services[arr[1]]
			if !ex {
				expSvc = exportedService{
					Tags:  make(map[string]string),
					Nodes: make([]string, 0),
				}
			}

			// put member on the node list
			expSvc.Nodes = append(expSvc.Nodes, member.Name)

			// Split value
			arrV := strings.Split(v, ",")
			for _, elemV := range arrV {
				arrE := strings.Split(elemV, "=")

				if len(arrE) > 0 {
					ek := arrE[0]

					if ek == "port" && len(arrE) == 2 {
						expSvc.Port, _ = strconv.Atoi(arrE[1])
						continue
					}
					// put into general tags otherwise
					if len(arrE) >= 1 {
						ek = arrE[0]
					}
					ev := ""
					if len(arrE) == 2 {
						ev = arrE[1]
					}
					expSvc.Tags[ek] = ev

				}
			}

			e.Services[arr[1]] = expSvc

		}
	}

}
