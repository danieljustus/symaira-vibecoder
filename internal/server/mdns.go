package server

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/hashicorp/mdns"
)

// MDNSAdvertiser wraps an mdns.Server so the symvibe server can announce
// _symvibe._tcp on the local network.
type MDNSAdvertiser struct {
	srv *mdns.Server
}

// AdvertiseMulticastDNS registers a _symvibe._tcp service. It is only called
// when server.access == "lan" and server.multicast_dns is true.
func AdvertiseMulticastDNS(host string, port int) (*MDNSAdvertiser, error) {
	ips, err := resolveAdvertiseIPs(host)
	if err != nil {
		return nil, fmt.Errorf("mdns ips: %w", err)
	}

	svc, err := mdns.NewMDNSService(
		"symvibe",
		"_symvibe._tcp",
		"",
		"symvibe.local.",
		port,
		ips,
		[]string{"path=/"},
	)
	if err != nil {
		return nil, fmt.Errorf("mdns service: %w", err)
	}

	srv, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return nil, fmt.Errorf("mdns server: %w", err)
	}

	slog.Info("mDNS advertisement active", "service", "_symvibe._tcp", "port", port, "ips", ips)
	return &MDNSAdvertiser{srv: srv}, nil
}

// Shutdown stops the mDNS advertisement.
func (m *MDNSAdvertiser) Shutdown() {
	if m == nil || m.srv == nil {
		return
	}
	_ = m.srv.Shutdown()
	slog.Info("mDNS advertisement stopped")
}

// resolveAdvertiseIPs returns the IPs the mDNS proxy should announce. For
// 0.0.0.0/:: we enumerate local interface addresses; for a concrete host we
// resolve it via the resolver.
func resolveAdvertiseIPs(host string) ([]net.IP, error) {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return localInterfaceIPs()
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

func localInterfaceIPs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []net.IP
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				if ip := ipnet.IP.To4(); ip != nil {
					out = append(out, ip)
				} else if ip := ipnet.IP.To16(); ip != nil {
					out = append(out, ip)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no suitable non-loopback interface address found")
	}
	return out, nil
}
