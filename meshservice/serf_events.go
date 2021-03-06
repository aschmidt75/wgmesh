package meshservice

import (
	"math/rand"
	"net"
	"time"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	serf "github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// parses the user event as a Peer announcement and adds the peer
// to the wireguard interface
func (ms *MeshService) serfHandleJoinRequestEvent(userEv serf.UserEvent) {
	peerAnnouncement := &Peer{}
	err := proto.Unmarshal(userEv.Payload, peerAnnouncement)
	if err != nil {
		log.WithError(err).Error("unable to unmarshal a user event")
	}
	log.WithField("pa", peerAnnouncement).Trace("user event: peerAnnouncement")

	if peerAnnouncement.Type == Peer_JOIN {

		wg := wgwrapper.New()
		ok, err := wg.AddPeer(ms.WireguardInterface, wgwrapper.WireguardPeer{
			RemoteEndpointIP: peerAnnouncement.EndpointIP,
			ListenPort:       int(peerAnnouncement.EndpointPort),
			Pubkey:           peerAnnouncement.Pubkey,
			AllowedIPs: []net.IPNet{
				{
					IP:   net.ParseIP(peerAnnouncement.MeshIP),
					Mask: net.CIDRMask(32, 32),
				},
			},
		})

		if err != nil {
			log.WithError(err).Error("unable to add peer after user event")
		} else {
			if ok {
				log.WithFields(log.Fields{
					"pk": peerAnnouncement.Pubkey,
					"ip": peerAnnouncement.MeshIP,
				}).Info("added peer")
			} else {
				// if we're a bootstrap node then this peer has already been added
				// by the grpc join request function.
			}
		}
	}
}

func (ms *MeshService) serfHandleRTTRequestEvent(userEv serf.UserEvent) {
	go func(ms *MeshService) {
		//
		sl := rand.Intn(ms.Serf().NumNodes() * 1000)
		log.WithField("msec", sl).Trace("Delaying rtt response")
		time.Sleep(time.Duration(sl) * time.Millisecond)

		// compose my own rtt list
		myCoord, err := ms.Serf().GetCoordinate()
		if err != nil {
			log.WithError(err).Warn("Unable to get my own coordinate, check config")
			return
		}
		rtts := make([]*RTTResponseInfo, ms.Serf().NumNodes())
		for idx, member := range ms.Serf().Members() {
			memberCoord, ok := ms.Serf().GetCachedCoordinate(member.Name)
			if ok && memberCoord != nil {
				d := memberCoord.DistanceTo(myCoord)
				rtts[idx] = &RTTResponseInfo{
					Node:    member.Name,
					RttMsec: int32(d / time.Millisecond),
				}
			}
		}

		// post as user event
		rttResponseBuf, err := proto.Marshal(&RTTResponse{
			Node: ms.NodeName,
			Rtts: rtts,
		})
		if err != nil {
			log.WithError(err).Error("Unable to marshal rtt response message")
			return
		}
		ms.Serf().UserEvent(serfEventMarkerRTTRes, []byte(rttResponseBuf), true)

	}(ms)
}

func (ms *MeshService) serfHandleRTTResponseEvent(userEv serf.UserEvent) {

	rttResponse := &RTTResponse{}
	err := proto.Unmarshal(userEv.Payload, rttResponse)
	if err != nil {
		log.WithError(err).Error("unable to unmarshal rtt response user event")
	}

	log.WithField("rttinfo", rttResponse).Trace("user event: rttResponse")

	// forward to current rtt response chan
	if ms.rttResponseChan != nil {
		*ms.rttResponseChan <- *rttResponse
	}
}

func (ms *MeshService) serfHandleMemberEvent(ev serf.MemberEvent) {
	for _, member := range ev.Members {
		// remove this peer from wireguard interface
		wg := wgwrapper.New()

		err := wg.RemovePeerByPubkey(ms.WireguardInterface, member.Tags[nodeTagPubKey])
		if err != nil {
			log.WithError(err).Error("unable to remove failed/left wireguard peer")
		}

		err = ms.Serf().RemoveFailedNodePrune(member.Name)
		if err != nil {
			log.WithError(err).Error("unable to remove failed/left serf node")
		} else {
			log.WithFields(log.Fields{
				"node": member.Name,
				"ip":   member.Addr.String(),
			}).Info("node left mesh")
		}

	}

}

func (ms *MeshService) serfEventHandler(ch <-chan serf.Event) {
	for {
		select {
		case ev := <-ch:

			go func(ev serf.Event) {
				for key, ch := range ms.serfEventNotifierMap {
					log.WithFields(log.Fields{
						"key": key,
						"ev":  ev}).Trace("Forwarding event")
					*ch <- ev
				}
			}(ev)

			if ev.EventType() == serf.EventUser {
				userEv := ev.(serf.UserEvent)

				if userEv.Name == serfEventMarkerJoin {
					log.WithField("ev", userEv).Debug("received join request event")
					go ms.serfHandleJoinRequestEvent(userEv)
				}
				if userEv.Name == serfEventMarkerRTTReq {
					log.WithField("ev", userEv).Debug("received rtt request event")
					go ms.serfHandleRTTRequestEvent(userEv)
				}
				if userEv.Name == serfEventMarkerRTTRes {
					log.WithField("ev", userEv).Debug("received rtt response event")
					go ms.serfHandleRTTResponseEvent(userEv)
				}

			}

			if ev.EventType() == serf.EventMemberJoin {
				evJoin := ev.(serf.MemberEvent)

				log.WithField("members", evJoin.Members).Debug("received join event")
				ms.lastUpdatedTS = time.Now()
			}
			if ev.EventType() == serf.EventMemberUpdate {
				evUpdate := ev.(serf.MemberEvent)

				log.WithField("members", evUpdate.Members).Debug("received member update")
				ms.lastUpdatedTS = time.Now()
			}
			if ev.EventType() == serf.EventMemberLeave || ev.EventType() == serf.EventMemberFailed || ev.EventType() == serf.EventMemberReap {
				evMember := ev.(serf.MemberEvent)

				log.WithField("members", evMember.Members).Debug("received leave/failed event")
				ms.lastUpdatedTS = time.Now()

				go ms.serfHandleMemberEvent(evMember)

			}

		}
	}
}
