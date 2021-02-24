package meshservice

import (
	"net"

	log "github.com/sirupsen/logrus"
	"gortc.io/stun"
)

// STUNService ...
type STUNService struct {
	ip4, ip6 bool

	stunServerURI string
}

// NewSTUNService creates a new STUNService struct with
// default settings working for IPv4
func NewSTUNService() STUNService {
	return STUNService{
		ip4:           true,
		ip6:           false,
		stunServerURI: "stun.l.google.com:19302",
	}
}

// GetExternalIP retrieves my own external ip by querying it
// from the STUN server
func (st *STUNService) GetExternalIP() ([]net.IP, error) {

	ips := make([]net.IP, 0)

	log.Info("Fetching external IP from STUN server")
	if st.ip4 {
		c, err := stun.Dial("udp4", st.stunServerURI)
		if err != nil {
			return ips, err
		}

		message, err := stun.Build(stun.TransactionID, stun.BindingRequest)
		if err != nil {
			return ips, err
		}

		if err := c.Do(message, func(res stun.Event) {
			if res.Error != nil {
				return
			}

			var xorAddr stun.XORMappedAddress
			if err := xorAddr.GetFrom(res.Message); err != nil {
				return
			}
			ips = append(ips, xorAddr.IP)
		}); err != nil {
			return ips, err
		}

	}

	return ips, nil
}
