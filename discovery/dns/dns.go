package dns

import (
	"fmt"
	"net"
)

// Provider is a discovery provider that can be used to discover ringpop peers
// via dns. This is useful in cluster management systems that register all
// services with a dns name, eg. kubernetes with the SkyDNS addon.
type Provider struct {
	Hostname string
	Port     int
}

// Hosts returns a slice with all hostports ringpop is currently running based
// on a dns lookup.
func (k *Provider) Hosts() ([]string, error) {
	addrs, err := net.LookupHost(k.Hostname)
	if err != nil {
		return nil, err
	}

	for i := range addrs {
		addrs[i] = fmt.Sprintf("%s:%d", addrs[i], k.Port)
	}
	return addrs, nil
}
