package dhcpserver

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"
)

// TestStartListenersGuardsZeroBind pins the silent-failure class fixed across
// the server plugins: when every interface fails to bind, startListeners must
// report it and return nil, and must NOT log a false "dhcpserver: started".
func TestStartListenersGuardsZeroBind(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	dhcpListen = func(string) (*net.UDPConn, error) { return nil, errors.New("bind refused") }

	log, buf := captureLogger()
	got := startListeners(serverConfig{ListenInterfaces: []string{"enp2s0"}}, nil, log)
	if got != nil {
		t.Fatalf("expected nil listeners when all binds fail, got %d", len(got))
	}
	out := buf.String()
	if !strings.Contains(out, "no interfaces bound") {
		t.Errorf("missing zero-listener error in: %s", out)
	}
	if strings.Contains(out, "dhcpserver: started") {
		t.Errorf("must not log a false \"started\" with zero listeners: %s", out)
	}
}

// ephemeralUDP returns a real loopback UDP socket (closed on cleanup) so the
// dhcpListen stub can model a successful bind. serveMulti reads from it and
// exits silently when it is closed (register.go), so no packets and no log
// writes race the assertions.
func ephemeralUDP(t *testing.T) *net.UDPConn {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ephemeral udp: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestStartListenersReportsBoundCount covers the success path: every interface
// binds, so startListeners returns them all and logs "started" with the count.
func TestStartListenersReportsBoundCount(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	dhcpListen = func(string) (*net.UDPConn, error) { return ephemeralUDP(t), nil }

	log, buf := captureLogger()
	got := startListeners(serverConfig{ListenInterfaces: []string{"eth0", "eth1"}}, nil, log)
	if len(got) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(got))
	}
	out := buf.String()
	if !strings.Contains(out, "dhcpserver: started") || !strings.Contains(out, "listeners=2") {
		t.Errorf("expected started log with listeners=2, got: %s", out)
	}
}

// TestStartListenersPartialBind covers the partial path: one interface binds,
// another fails. startListeners returns only the bound listener, logs the
// failure, and still reports "started" because at least one bound.
func TestStartListenersPartialBind(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	calls := 0
	dhcpListen = func(string) (*net.UDPConn, error) {
		calls++
		if calls == 1 {
			return ephemeralUDP(t), nil
		}
		return nil, errors.New("second interface down")
	}

	log, buf := captureLogger()
	got := startListeners(serverConfig{ListenInterfaces: []string{"eth0", "eth1"}}, nil, log)
	if len(got) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(got))
	}
	out := buf.String()
	if !strings.Contains(out, "listen failed") {
		t.Errorf("expected a listen-failed log for the down interface: %s", out)
	}
	if !strings.Contains(out, "dhcpserver: started") || !strings.Contains(out, "listeners=1") {
		t.Errorf("expected started log with listeners=1, got: %s", out)
	}
}

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestLogExchangePXEOffer(t *testing.T) {
	h := newTestPXEServer(t)
	mac := net.HardwareAddr{0x60, 0xbe, 0xb4, 0x20, 0xe8, 0xd4}
	req := buildPXEDiscover(mac, 0x1234, pxeArchUEFIx64)
	resp := h.handle(req)
	if resp == nil {
		t.Fatal("expected an OFFER reply")
	}

	log, buf := captureLogger()
	logExchange(log, req, resp, nil)
	out := buf.String()

	for _, want := range []string{"OFFER", mac.String(), "bootfile"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}

func TestLogExchangeNoReply(t *testing.T) {
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	req := buildPXEDiscover(mac, 0x1, pxeArchUEFIx64)

	log, buf := captureLogger()
	logExchange(log, req, nil, nil)
	out := buf.String()

	if !strings.Contains(out, "no reply") || !strings.Contains(out, mac.String()) {
		t.Errorf("expected a no-reply log carrying the MAC, got %q", out)
	}
}

// TestLogExchangeForeignServerID pins the diagnostic that explains a PXE install
// which boots but then stalls: the booted kernel's ip=dhcp REQUEST selected a
// competing DHCP server on the segment, so we stay silent. The log must name the
// foreign server-id and ours so the operator can spot the second server.
func TestLogExchangeForeignServerID(t *testing.T) {
	mac := net.HardwareAddr{0x60, 0xbe, 0xb4, 0x20, 0xe8, 0xd5}
	// SELECTING REQUEST that selected a different server (192.168.1.1), asking
	// for an address (198.19.255.2) that happens to be inside our subnet.
	req := buildRequest(mac, 0x99, netip.MustParseAddr("198.19.255.2"), netip.MustParseAddr("192.168.1.1"))
	ours := []netip.Addr{netip.MustParseAddr("198.19.255.1")}

	log, buf := captureLogger()
	logExchange(log, req, nil, ours)
	out := buf.String()

	for _, want := range []string{"another DHCP server", "192.168.1.1", "198.19.255.1", mac.String()} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}

func TestLogExchangeShortPacketIgnored(t *testing.T) {
	log, buf := captureLogger()
	logExchange(log, []byte{1, 2, 3}, nil, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no log for a runt packet, got %q", buf.String())
	}
}

func TestMsgTypeName(t *testing.T) {
	cases := map[byte]string{
		msgDiscover: "DISCOVER",
		msgOffer:    "OFFER",
		msgRequest:  "REQUEST",
		msgAck:      "ACK",
		msgNak:      "NAK",
		0xff:        "UNKNOWN",
	}
	for code, want := range cases {
		if got := msgTypeName(code); got != want {
			t.Errorf("msgTypeName(%d) = %q, want %q", code, got, want)
		}
	}
}
