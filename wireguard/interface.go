package wireguard

import (
	"errors"
	"fmt"
	"net"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
	log "github.com/sirupsen/logrus"
)

// CreateWireguardInterfaceForMesh ...
func CreateWireguardInterfaceForMesh(meshName string, bootstrapIP string, wgListenPort int) error {
	log.WithFields(log.Fields{
		"n":  meshName,
		"ip": bootstrapIP,
	}).Debug("CreateWireguardInterfaceForMesh")

	intfName := fmt.Sprintf("wg%s", meshName)

	i, err := net.InterfaceByName(intfName)
	if err == nil {
		log.WithField("i", i).Error("Interface already exists")
		return errors.New("a create wireguard interface for this mesh already exists")
	}

	wg := wgwrapper.New()

	wgi := wgwrapper.NewWireguardInterface(intfName, net.ParseIP(bootstrapIP))

	err = wg.AddInterface(wgi)
	if err != nil {
		log.WithField("err", err).Error("unable to create interface")
		return errors.New("unable to create wireguard interface")
	}

	log.WithField("wgi", wgi).Debug("created")

	// listen on a port
	wgi.ListenPort = wgListenPort
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
