package meshservice

import (
	"errors"
	"fmt"
	"net"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	log "github.com/sirupsen/logrus"
)

// CreateWireguardInterfaceForMesh creates a new wireguard interface based on the
// name of the mesh, a bootstrap IP and a listen port. The interfacae is also up'ed.
func (ms *MeshService) CreateWireguardInterfaceForMesh(bootstrapIP string, wgListenPort int) error {
	log.WithFields(log.Fields{
		"n":  ms.MeshName,
		"ip": bootstrapIP,
	}).Trace("CreateWireguardInterfaceForMesh")

	intfName := fmt.Sprintf("wg%s", ms.MeshName)

	ms.NodeName = fmt.Sprintf("wg-%s", ms.MeshName)
	ms.MeshIP = net.ParseIP(bootstrapIP)
	ms.WireguardListenPort = wgListenPort

	i, err := net.InterfaceByName(intfName)
	if err == nil {
		log.WithField("i", i).Error("Interface already exists")
		return errors.New("a create wireguard interface for this mesh already exists")
	}

	wg := wgwrapper.New()

	wgi := wgwrapper.NewWireguardInterface(intfName, ms.MeshIP)

	err = wg.AddInterface(wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to create interface")
		return errors.New("unable to create wireguard interface")
	}

	log.WithField("wgi", wgi).Debug("created")

	// listen on a port
	wgi.ListenPort = ms.WireguardListenPort
	err = wg.Configure(&wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to configure interface")
		return errors.New("unable to configure wireguard interface")
	}

	err = wg.SetInterfaceUp(wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to ifup interface")
		return errors.New("unable to setup wireguard interface")
	}

	log.Infof("Created annd configured wireguard interface %s", intfName)

	return nil
}

// CreateWireguardInterface creates a new wireguard interface based on the
// name of the mesh, and a listen port. The interfacae does not yet carry an internal ip and is not up'ed.
// Returns the pub key
func (ms *MeshService) CreateWireguardInterface(wgListenPort int) (string, error) {
	log.WithFields(log.Fields{
		"n": ms.MeshName,
	}).Trace("CreateWireguardInterface")

	intfName := fmt.Sprintf("wg%s", ms.MeshName)

	ms.NodeName = fmt.Sprintf("wg-%s", ms.MeshName)
	ms.WireguardListenPort = wgListenPort

	i, err := net.InterfaceByName(intfName)
	if err == nil {
		log.WithField("i", i).Error("Interface already exists")
		return "", errors.New("a create wireguard interface for this mesh already exists")
	}

	wg := wgwrapper.New()

	wgi := wgwrapper.NewWireguardInterfaceNoAddr(intfName)
	wgi.IP = nil

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

	log.Infof("Created annd configured wireguard interface %s as no-up", intfName)

	return wgi.PublicKey, nil
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
