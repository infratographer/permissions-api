package selecthost

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrHostRemoved is set on a host if it no longer exists in the discovered hosts.
	ErrHostRemoved = fmt.Errorf("%w: host removed", ErrSelectHost)
)

// JoinHostPort combines host and port into a network address of the form "host:port".
// If host contains a colon, as found in literal IPv6 addresses, then JoinHostPort returns "[host]:port".
// If port is not defined, port is left out.
func JoinHostPort(host string, port string) string {
	if strings.ContainsRune(host, ':') {
		host = "[" + host + "]"
	}

	if port != "" {
		host += ":" + port
	}

	return host
}

func hostID(host string, port, priority, weight uint16) string {
	parts := []string{
		strings.TrimRight(host, "."),
		strconv.FormatUint(uint64(port), 10),
		strconv.FormatUint(uint64(priority), 10),
		strconv.FormatUint(uint64(weight), 10),
	}

	return strings.Join(parts, ":")
}

// ParseHost parses the provided host allowing for a host without a port and returns a new [Host].
func ParseHost(selector *Selector, host string) (Host, error) {
	var port string

	shost, sport, err := net.SplitHostPort(host)
	if err != nil {
		if addrErr, ok := err.(*net.AddrError); !ok || addrErr.Err != "missing port in address" {
			return nil, err
		}
	} else {
		host = shost
		port = sport
	}

	return newHost(selector, host, port, net.SRV{}), nil
}

func diffHosts(s *Selector, srvs []*net.SRV) ([]Host, []Host, Hosts) {
	var (
		trackedHosts = make(map[string]Host, len(s.hosts))
		srvTargets   = make(map[string]*net.SRV, len(srvs))

		addedHosts   = make([]Host, 0)
		matchedHosts = make(Hosts, len(srvs))
		removedHosts = make([]Host, 0)
	)

	for _, host := range s.hosts {
		trackedHosts[host.ID()] = host
	}

	for i, srv := range srvs {
		srvKey := hostID(srv.Target, srv.Port, srv.Priority, srv.Weight)

		srvTargets[srvKey] = srv

		host := trackedHosts[srvKey]
		if host != nil {
			matchedHosts[i] = host
		} else {
			matchedHosts[i] = newHost(s, srv.Target, strconv.FormatUint(uint64(srv.Port), 10), *srv)
			addedHosts = append(addedHosts, matchedHosts[i])
		}
	}

	for _, host := range s.hosts {
		if _, ok := srvTargets[host.ID()]; !ok {
			host.setError(ErrHostRemoved)
			removedHosts = append(removedHosts, host)
		}
	}

	return addedHosts, removedHosts, matchedHosts
}

func toMilliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
