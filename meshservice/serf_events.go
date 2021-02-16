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
				net.IPNet{
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
		sl := rand.Intn(ms.s.NumNodes() * 1000)
		log.WithField("msec", sl).Trace("Delaying rtt response")
		time.Sleep(time.Duration(sl) * time.Millisecond)

		// compose my own rtt list
		myCoord, err := ms.s.GetCoordinate()
		if err != nil {
			log.WithError(err).Warn("Unable to get my own coordinate, check config")
			return
		}
		rtts := make([]*RTTResponseInfo, ms.s.NumNodes())
		for idx, member := range ms.s.Members() {
			memberCoord, ok := ms.s.GetCachedCoordinate(member.Name)
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
		ms.s.UserEvent("rtt1", []byte(rttResponseBuf), true)

	}(ms)
}

func (ms *MeshService) serfEventHandler(ch <-chan serf.Event) {
	for {
		select {
		case ev := <-ch:
			if ev.EventType() == serf.EventUser {
				userEv := ev.(serf.UserEvent)

				if userEv.Name == "j" {
					log.WithField("ev", userEv).Debug("received join request event")
					ms.serfHandleJoinRequestEvent(userEv)
				}
				if userEv.Name == "rtt0" {
					log.WithField("ev", userEv).Debug("received rtt request event")
					ms.serfHandleRTTRequestEvent(userEv)
				}
				if userEv.Name == "rtt1" {
					log.WithField("ev", userEv).Debug("received rtt response event")

					rttResponse := &RTTResponse{}
					err := proto.Unmarshal(userEv.Payload, rttResponse)
					if err != nil {
						log.WithError(err).Error("unable to unmarshal rtt response user event")
					}
					if rttResponse.Node == ms.NodeName {
						// skip, this is me
					} else {
						log.WithField("rttinfo", rttResponse).Trace("user event: rttResponse")

						// forward to current rtt response chan
						if ms.rttResponseChan != nil {
							*ms.rttResponseChan <- *rttResponse
						}
					}
				}

			}

			if ev.EventType() == serf.EventMemberJoin {
				evJoin := ev.(serf.MemberEvent)

				log.WithField("members", evJoin.Members).Debug("received join event")
			}
			if ev.EventType() == serf.EventMemberLeave || ev.EventType() == serf.EventMemberFailed || ev.EventType() == serf.EventMemberReap {
				evJoin := ev.(serf.MemberEvent)

				log.WithField("members", evJoin.Members).Debug("received leave/failed event")

				for _, member := range evJoin.Members {
					// remove this peer from wireguard interface
					wg := wgwrapper.New()

					err := wg.RemovePeerByPubkey(ms.WireguardInterface, member.Tags["pk"])
					if err != nil {
						log.WithError(err).Error("unable to remove failed/left wireguard peer")
					}

					err = ms.s.RemoveFailedNodePrune(member.Name)
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

		}
	}
}
