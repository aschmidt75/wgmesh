package cmd

import (
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"

	wgwrapper "github.com/aschmidt75/go-wg-wrapper/pkg/wgwrapper"
)

// given an IP address or interface name or empty, this returns the IP
// as net.IP. If empty string is given, this takes the IP address of
// the interface where the default route is attached to.
func getIPFromIPOrIntfParam(i string) net.IP {

	if i == "" {
		wg := wgwrapper.New()
		i, _ = wg.DefaultRouteInterface()
	}

	// is this an IP address?
	ipv6_regex := `^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$`
	ipv4_regex := `^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`

	ok, _ := regexp.MatchString(ipv4_regex+`|`+ipv6_regex, i)
	if ok {
		return net.ParseIP(i)
	}

	arr := strings.Split(i, "%")
	idx := 0
	if len(arr) >= 2 {
		i = arr[0]
		var err error
		idx, err = strconv.Atoi(arr[1])
		if err != nil {
			idx = 0
		}
	}

	// is it a valid interface name?
	intf, err := net.InterfaceByName(i)
	if err != nil {
		return nil
	}

	addrs, err := intf.Addrs()
	if err != nil {
		return nil
	}

	if idx >= len(addrs) {
		return nil
	}

	s := addrs[idx].String()
	if strings.IndexAny(s, "/") >= 0 {
		arr = strings.Split(s, "/")
		s = arr[0]
	}

	return net.ParseIP(s)
}

// https://stackoverflow.com/questions/41240761/check-if-ip-address-is-in-private-network-space/41273687#41273687
func isPrivateIP(ip string) (bool, error) {
	var err error
	private := false
	IP := net.ParseIP(ip)
	if IP == nil {
		err = errors.New("Invalid IP")
	} else {
		_, private24BitBlock, _ := net.ParseCIDR("10.0.0.0/8")
		_, private20BitBlock, _ := net.ParseCIDR("172.16.0.0/12")
		_, private16BitBlock, _ := net.ParseCIDR("192.168.0.0/16")
		private = private24BitBlock.Contains(IP) || private20BitBlock.Contains(IP) || private16BitBlock.Contains(IP)
	}
	return private, err
}
