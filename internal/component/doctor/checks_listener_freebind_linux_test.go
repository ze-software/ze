//go:build linux

package doctor

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"testing"
)

// VALIDATES: a freebind service on an address the host does not hold is
// still reported when its port is taken. Linux checks the address before the
// port, so a plain probe meets EADDRNOTAVAIL and never sees the conflict; the
// probe binds with IP_FREEBIND, as the service does, and meets EADDRINUSE.
// Method: hold a wildcard port for TCP and UDP, then probe the absent address
// on it as a freebind service.
// PREVENTS: the freebind tolerance hiding a port the service will not get.
func TestProbeListenerFreebindAbsentAddressBusyPort(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("hold a tcp port: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not TCP", ln.Addr())
	}
	port := strconv.Itoa(tcpAddr.Port)
	pc, err := lc.ListenPacket(context.Background(), "udp4", net.JoinHostPort("0.0.0.0", port))
	if err != nil {
		t.Skipf("udp port %s is taken by another program, so the pair cannot be held: %v", port, err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	for _, network := range []string{"udp", "tcp"} {
		busy := serviceListener{service: "test", network: network, host: absentAddr, port: port, code: "doctor-listen-unavailable", freebind: true}
		if err := probeListener(busy); !errors.Is(err, syscall.EADDRINUSE) {
			t.Errorf("%s: freebind probe of %s on a held port = %v, want EADDRINUSE", network, absentAddr, err)
		}
	}
}
