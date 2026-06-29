package server

import (
	"net"
	"testing"
)

func TestResolveAdvertiseIPsEmptyHost(t *testing.T) {
	ips, err := resolveAdvertiseIPs("")
	if err != nil {
		t.Fatalf("resolveAdvertiseIPs(\"\") error: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least one IP from local interfaces")
	}
	for _, ip := range ips {
		if ip.To4() == nil && ip.To16() == nil {
			t.Fatalf("expected valid IPv4 or IPv6, got %v", ip)
		}
	}
}

func TestResolveAdvertiseIPsWildcardIPv4(t *testing.T) {
	ips, err := resolveAdvertiseIPs("0.0.0.0")
	if err != nil {
		t.Fatalf("resolveAdvertiseIPs(\"0.0.0.0\") error: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least one IP from local interfaces")
	}
}

func TestResolveAdvertiseIPsWildcardIPv6(t *testing.T) {
	ips, err := resolveAdvertiseIPs("::")
	if err != nil {
		t.Fatalf("resolveAdvertiseIPs(\"::\") error: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least one IP from local interfaces")
	}
}

func TestResolveAdvertiseIPsConcreteHost(t *testing.T) {
	ips, err := resolveAdvertiseIPs("127.0.0.1")
	if err != nil {
		t.Fatalf("resolveAdvertiseIPs(\"127.0.0.1\") error: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least one IP for 127.0.0.1")
	}
	found := false
	for _, ip := range ips {
		if ip.Equal(net.ParseIP("127.0.0.1")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 127.0.0.1 in results, got %v", ips)
	}
}

func TestLocalInterfaceIPsReturnsValidIPs(t *testing.T) {
	ips, err := localInterfaceIPs()
	if err != nil {
		t.Fatalf("localInterfaceIPs error: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least one non-loopback interface IP")
	}
	for _, ip := range ips {
		if ip.IsLoopback() {
			t.Fatalf("expected non-loopback IP, got %v", ip)
		}
		if ip.To4() == nil && ip.To16() == nil {
			t.Fatalf("expected valid IP, got %v", ip)
		}
	}
}

func TestShutdownNilReceiver(t *testing.T) {
	var m *MDNSAdvertiser
	m.Shutdown()
}

func TestShutdownEmptyServer(t *testing.T) {
	m := &MDNSAdvertiser{srv: nil}
	m.Shutdown()
}

func TestAdvertiseMulticastDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mDNS integration test in short mode")
	}

	adv, err := AdvertiseMulticastDNS("0.0.0.0", 4318)
	if err != nil {
		t.Fatalf("AdvertiseMulticastDNS error: %v", err)
	}
	if adv == nil {
		t.Fatal("expected non-nil advertiser")
	}
	adv.Shutdown()
}
