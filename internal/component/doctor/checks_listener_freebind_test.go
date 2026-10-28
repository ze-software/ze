package doctor

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// absentAddr is an address from TEST-NET-1 (RFC 5737), which no test host
// holds, so a plain bind to it fails with EADDRNOTAVAIL.
const absentAddr = "192.0.2.53"

// VALIDATES: probeListener accepts an address the host does not hold for a
// service that binds with IP_FREEBIND, and still reports it for one that does
// not. Method: probe a TEST-NET-1 address both ways.
// PREVENTS: ze doctor warning doctor-listen-unavailable about a resolver
// listener on a PPP pool gateway that is not up yet, which the resolver binds
// anyway.
func TestProbeListenerFreebindAcceptsAbsentAddress(t *testing.T) {
	for _, network := range []string{"udp", "tcp"} {
		plain := serviceListener{service: "test", network: network, host: absentAddr, port: "10053", code: "doctor-listen-unavailable"}
		err := probeListener(plain)
		if !errors.Is(err, syscall.EADDRNOTAVAIL) {
			t.Fatalf("%s: plain probe of %s = %v, want EADDRNOTAVAIL: the test address is on this host", network, absentAddr, err)
		}

		freebind := plain
		freebind.freebind = true
		if err := probeListener(freebind); err != nil {
			t.Errorf("%s: freebind probe of an address not on the host = %v, want nil: the service binds it with IP_FREEBIND", network, err)
		}
	}
}

// VALIDATES: a freebind service whose port another socket holds is still
// reported. Method: hold a loopback TCP port, then probe it as a freebind
// service.
// PREVENTS: the freebind tolerance swallowing every bind error.
func TestProbeListenerFreebindStillReportsBusyPort(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold a port: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not TCP", ln.Addr())
	}

	busy := serviceListener{service: "test", network: "tcp", host: "127.0.0.1", port: strconv.Itoa(tcpAddr.Port), code: "doctor-listen-unavailable", freebind: true}
	if err := probeListener(busy); !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("freebind probe of a held port = %v, want EADDRINUSE", err)
	}
}

// VALIDATES: the services that bind with IP_FREEBIND reach the probe marked,
// and one that does not stays unmarked. Method: derive the probes for each
// service from the schema, as ze doctor does, and read the flag.
// PREVENTS: a registration in a plugin's register.go that never reaches
// collectSchemaListeners, and the flag leaking onto every service.
func TestCollectSchemaListenersMarksFreebindServices(t *testing.T) {
	want := map[string]bool{
		"service-dns-resolver-listener":       true,
		"service-as112-ipv4-anycast-listener": true,
		"service-as112-ipv6-anycast-listener": true,
		"service-geodns-listener":             false,
	}
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	seen := map[string]bool{}
	for _, svc := range config.DiscoverListenerServices(schema) {
		wantFreebind, tracked := want[svc.Name]
		if !tracked {
			continue
		}
		listeners, err := collectSchemaListeners(listenerEntryWithoutPortTree(svc))
		if err != nil {
			t.Fatalf("%s: collect listeners: %v", svc.Name, err)
		}
		for _, l := range listeners {
			if l.host != "127.0.0.1" {
				continue // a sibling service's default endpoint, not this entry
			}
			seen[svc.Name] = true
			if l.freebind != wantFreebind {
				t.Errorf("%s %s probe: freebind = %v, want %v", svc.Name, l.network, l.freebind, wantFreebind)
			}
		}
	}
	// The resolver is always linked. as112 and geodns are linked only under
	// ze_as112 and ze_geodns, so their rows are proven by a tagged run
	// (-tags ze_as112,ze_geodns) and are absent from the schema otherwise.
	if !seen["service-dns-resolver-listener"] {
		t.Errorf("no probe derived for service-dns-resolver-listener: the service or its listener default is gone")
	}
	for name := range want {
		if !seen[name] {
			t.Logf("%s is not in this build's schema; its row was not checked", name)
		}
	}
}
