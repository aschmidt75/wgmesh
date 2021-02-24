package meshservice

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	log "github.com/sirupsen/logrus"
)

// CreateWireguardInterfaceForMesh creates a new wireguard interface based on the
// name of the mesh, a bootstrap IP and a listen port. The interfacae is also up'ed.
func (ms *MeshService) CreateWireguardInterfaceForMesh(bootstrapIP string, wgListenPort int) (string, error) {
	log.WithFields(log.Fields{
		"n":  ms.MeshName,
		"ip": bootstrapIP,
	}).Trace("CreateWireguardInterfaceForMesh")

	intfName := fmt.Sprintf("wg%s", ms.MeshName)

	ms.WireguardListenPort = wgListenPort

	i, err := net.InterfaceByName(intfName)
	if err == nil {
		log.WithField("i", i).Error("Interface already exists")
		return "", errors.New("a create wireguard interface for this mesh already exists")
	}

	wg := wgwrapper.New()

	wgi := wgwrapper.NewWireguardInterface(intfName, ms.MeshIP)
	ms.WireguardInterface = wgi

	err = wg.AddInterface(wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to create interface")
		return "", errors.New("unable to create wireguard interface")
	}

	log.WithField("wgi", wgi).Debug("created")

	// listen on a port
	wgi.ListenPort = ms.WireguardListenPort
	err = wg.Configure(&wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to configure interface")
		return "", errors.New("unable to configure wireguard interface")
	}

	err = wg.SetInterfaceUp(wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to ifup interface")
		return "", errors.New("unable to setup wireguard interface")
	}

	log.Infof("Created and configured wireguard interface %s", intfName)

	return wgi.PublicKey, nil
}

// CreateWireguardInterface creates a new wireguard interface based on the
// name of the mesh, and a listen port. The interfacae does not yet carry an internal ip and is not up'ed.
// Returns the pub key
func (ms *MeshService) CreateWireguardInterface(wgListenPort int) (string, error) {
	log.WithFields(log.Fields{
		"n": ms.MeshName,
	}).Trace("CreateWireguardInterface")

	intfName := fmt.Sprintf("wg%s", ms.MeshName)

	ms.WireguardListenPort = wgListenPort

	i, err := net.InterfaceByName(intfName)
	if err == nil {
		log.WithField("i", i).Error("Interface already exists")
		return "", errors.New("a wireguard interface for this mesh already exists")
	}

	wg := wgwrapper.New()

	wgi := wgwrapper.NewWireguardInterfaceNoAddr(intfName)

	err = wg.AddInterfaceNoAddr(wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to create interface")
		return "", errors.New("unable to create wireguard interface")
	}

	log.WithField("wgi", wgi).Debug("created")
	ms.WireguardInterface = wgi

	// listen on a port
	wgi.ListenPort = ms.WireguardListenPort
	err = wg.Configure(&wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to configure interface")
		return "", errors.New("unable to configure wireguard interface")
	}

	log.Infof("Created and configured wireguard interface %s as no-up", intfName)

	return wgi.PublicKey, nil
}

// AssignJoinerIP sets the ip address of the wireguard interface
func (ms *MeshService) AssignJoinerIP(ip string) error {
	intfName := fmt.Sprintf("wg%s", ms.MeshName)

	intf, err := net.InterfaceByName(intfName)
	if err != nil {
		return err
	}

	// Assign IP if desired and not yet present
	a, err := intf.Addrs()
	if err != nil {
		return err
	}
	if len(a) == 0 {
		cmd := exec.Command("/sbin/ip", "address", "add", "dev", intfName, ip)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return err
		}
		_, errStr := string(stdout.Bytes()), string(stderr.Bytes())
		if len(errStr) > 0 {
			e := fmt.Sprintf("/sbin/ip reported: %s", errStr)
			return errors.New(e)
		}
	}

	a, err = intf.Addrs()
	if len(a) == 0 {
		e := fmt.Sprintf("unable to add ip address %s to interface %s: %s", ip, intfName, err)
		return errors.New(e)
	}

	return nil
}

// SetRoute adds a route for the cidr range to the wireguard interface
func (ms *MeshService) SetRoute() error {
	wg := wgwrapper.New()

	// Add route
	err := wg.SetRoute(ms.WireguardInterface, ms.CIDRRange.String())
	if err != nil {
		log.WithError(err).Error("unable to add route to target")
		return err
	}
	log.WithField("target", ms.CIDRRange.String()).Debug("Added route")

	return nil
}

// RemoveWireguardInterfaceForMesh ..
func (ms *MeshService) RemoveWireguardInterfaceForMesh() error {
	log.WithFields(log.Fields{
		"n": ms.MeshName,
	}).Trace("RemoveWireguardInterfaceForMesh")

	intfName := fmt.Sprintf("wg%s", ms.MeshName)

	i, err := net.InterfaceByName(intfName)
	if err != nil {
		log.WithField("i", i).Error("Interface does not exist")
		return nil
	}

	wg := wgwrapper.New()

	return wg.DeleteInterface(wgwrapper.WireguardInterface{InterfaceName: intfName})
}

// ApplyPeerUpdatesFromStream reads peer data from an incoming stream and apply
// these to the interface.
// Returns a list of MeshIPs from all peers, with the first entry being the bootstrap node
// where we joined.
func (ms *MeshService) ApplyPeerUpdatesFromStream(wg wgwrapper.WireguardWrapper, stream Mesh_PeersClient) []string {
	peerCh := make(chan *Peer, 10)
	go peerAdder(ms, wg, peerCh)

	res := make([]string, 0)

	for {
		peer, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.WithError(err).Error("Error while receiving peers")
			break
		}

		peerCh <- peer

		res = append(res, peer.MeshIP)
	}

	// terminate peerAdder by sending nil
	peerCh <- nil

	return res
}

func peerAdder(ms *MeshService, wg wgwrapper.WireguardWrapper, peerCh <-chan *Peer) error {
	for {
		select {
		case peer := <-peerCh:
			if peer == nil {
				log.Trace("processed all peers")
				return nil
			}
			log.WithField("peer", peer).Debug("received peer")

			// Add the peer to the interface, making the peer the
			// only allowed ip
			ok, err := wg.AddPeer(ms.WireguardInterface, wgwrapper.WireguardPeer{
				RemoteEndpointIP: peer.EndpointIP,
				ListenPort:       int(peer.EndpointPort),
				Pubkey:           peer.Pubkey,
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP(peer.MeshIP),
						Mask: net.CIDRMask(32, 32),
					},
				},
			})

			if err != nil {
				log.WithError(err).Errorf("unable to add peer %s", peer.Pubkey)
			} else {
				if !ok {
					log.Errorf("unable to add peer %s", peer.Pubkey)
				} else {
					log.Trace("added peer")
				}
			}
		}
	}
}
