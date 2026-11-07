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

// TestBindListenersRefusesUnboundInterface pins R-13: when an interface
// fails to bind, bindListeners returns an error naming it and the fix, and
// never logs a false "dhcpserver: started".
func TestBindListenersRefusesUnboundInterface(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	dhcpListen = func(string) (*net.UDPConn, error) { return nil, errors.New("bind refused") }

	log, buf := captureLogger()
	got, err := bindListeners(serverConfig{Enabled: true, ListenInterfaces: []string{"enp2s0"}}, nil, log, true)
	if err == nil || got != nil {
		t.Fatalf("expected an error and no listeners when the bind fails, got %d, %v", len(got), err)
	}
	if !strings.Contains(err.Error(), `"enp2s0"`) || !strings.Contains(err.Error(), "Fix:") {
		t.Errorf("error does not name the interface and the fix: %v", err)
	}
	if strings.Contains(buf.String(), "dhcpserver: started") {
		t.Errorf("must not log a false \"started\": %s", buf.String())
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

// TestBindListenersReportsBoundCount covers the success path: every interface
// binds, so bindListeners returns them all and logs "started" with the count.
func TestBindListenersReportsBoundCount(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	dhcpListen = func(string) (*net.UDPConn, error) { return ephemeralUDP(t), nil }

	log, buf := captureLogger()
	got, err := bindListeners(serverConfig{Enabled: true, ListenInterfaces: []string{"eth0", "eth1"}}, nil, log, true)
	if err != nil || len(got) != 2 {
		t.Fatalf("expected 2 listeners, got %d, %v", len(got), err)
	}
	out := buf.String()
	if !strings.Contains(out, "dhcpserver: started") || !strings.Contains(out, "listeners=2") {
		t.Errorf("expected started log with listeners=2, got: %s", out)
	}
}

// TestBindListenersPartialBindBindsNothing covers the partial path: one
// interface binds and the next fails. bindListeners closes the bound one and
// returns the error, so an install never serves part of its interfaces.
func TestBindListenersPartialBindBindsNothing(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	var first *net.UDPConn
	dhcpListen = func(string) (*net.UDPConn, error) {
		if first == nil {
			first = ephemeralUDP(t)
			return first, nil
		}
		return nil, errors.New("second interface down")
	}

	log, _ := captureLogger()
	got, err := bindListeners(serverConfig{Enabled: true, ListenInterfaces: []string{"eth0", "eth1"}}, nil, log, true)
	if err == nil || got != nil {
		t.Fatalf("expected an error and no listeners, got %d, %v", len(got), err)
	}
	addr, ok := first.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.UDPAddr", first.LocalAddr())
	}
	if _, wErr := first.WriteToUDP([]byte{0}, addr); wErr == nil {
		t.Error("the listener bound before the failure was left open")
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

// TestBindListenersAtBootServesWhatBinds pins the boot mode: Stage 2 has no
// previous config to keep and the install path brings interfaces up after
// the daemon, so a failed interface is logged and the others serve.
func TestBindListenersAtBootServesWhatBinds(t *testing.T) {
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	dhcpListen = func(iface string) (*net.UDPConn, error) {
		if iface == "eth1" {
			return nil, errors.New("second interface down")
		}
		return ephemeralUDP(t), nil
	}

	log, buf := captureLogger()
	got, err := bindListeners(serverConfig{Enabled: true, ListenInterfaces: []string{"eth0", "eth1"}}, nil, log, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("expected 1 listener and no error at boot, got %d, %v", len(got), err)
	}
	if !strings.Contains(buf.String(), "listen failed") {
		t.Errorf("expected a listen-failed log for the down interface: %s", buf.String())
	}
}

// TestListenerIgnoresOtherSegments serves DISCOVERs through the request
// server of three listeners: two each addressed in its own shared network,
// one addressed in none.
// VALIDATES: AC-10: a listener answers only from the shared network its
// device is addressed in, and never offers a reservation of another segment;
// AC-11: a listener whose device holds no address in a configured subnet
// answers nothing and writes one warning naming the interface, its addresses
// and the fix, however many requests arrive.
// PREVENTS: every listener answering for every configured subnet.
func TestListenerIgnoresOtherSegments(t *testing.T) {
	previous := dhcpDeviceAddrs
	dhcpDeviceAddrs = func(device string) ([]netip.Addr, error) {
		switch device {
		case "eth-a":
			return []netip.Addr{netip.MustParseAddr("192.168.50.1")}, nil
		case "eth-b":
			return []netip.Addr{netip.MustParseAddr("192.168.60.1")}, nil
		default:
			return []netip.Addr{netip.MustParseAddr("203.0.113.1")}, nil
		}
	}
	t.Cleanup(func() { dhcpDeviceAddrs = previous })

	reservedB := net.HardwareAddr{0x02, 0, 0, 0, 0x60, 0x20}
	subnet := func(prefix, start, stop string, statics ...staticMapping) subnetConfig {
		return subnetConfig{
			Prefix:         netip.MustParsePrefix(prefix),
			Ranges:         []addressRange{{Name: "pool", Start: netip.MustParseAddr(start), Stop: netip.MustParseAddr(stop)}},
			LeaseTimeSec:   600,
			StaticMappings: statics,
		}
	}
	handlers := buildHandlers(serverConfig{Enabled: true, SharedNetworks: []sharedNetwork{
		{Name: "lan-a", Subnets: []subnetConfig{subnet("192.168.50.0/24", "192.168.50.100", "192.168.50.199")}},
		{Name: "lan-b", Subnets: []subnetConfig{subnet("192.168.60.0/24", "192.168.60.100", "192.168.60.199",
			staticMapping{Name: "printer", MAC: reservedB, IP: netip.MustParseAddr("192.168.60.20")})}},
	}}, nil)
	t.Cleanup(func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	})

	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	serve := func(device string, mac net.HardwareAddr) ([]byte, bool) {
		r := &requestServer{device: device, handlers: handlers, log: log}
		return r.answer(buildDiscover(mac, 0x6001))
	}

	for _, tc := range []struct {
		device string
		mac    net.HardwareAddr
		within string
	}{
		{"eth-a", net.HardwareAddr{0x02, 0, 0, 0, 0x50, 0x01}, "192.168.50.0/24"},
		{"eth-b", net.HardwareAddr{0x02, 0, 0, 0, 0x60, 0x01}, "192.168.60.0/24"},
		// The lan-b reservation is not offered on lan-a's interface.
		{"eth-a", reservedB, "192.168.50.0/24"},
	} {
		offer, resolved := serve(tc.device, tc.mac)
		if !resolved || offer == nil {
			t.Fatalf("%s, %s: no OFFER (resolved %v)", tc.device, tc.mac, resolved)
		}
		if got := netip.AddrFrom4([4]byte(offer[16:20])); !netip.MustParsePrefix(tc.within).Contains(got) {
			t.Errorf("%s, %s: offered %v, want an address of %s", tc.device, tc.mac, got, tc.within)
		}
	}

	r := &requestServer{device: "eth-c", handlers: handlers, log: log}
	for range 3 {
		if offer, resolved := r.answer(buildDiscover(net.HardwareAddr{0x02, 0, 0, 0, 0x70, 0x01}, 0x6002)); offer != nil || resolved {
			t.Fatalf("eth-c holds no address in a configured subnet: reply %d bytes, resolved %v", len(offer), resolved)
		}
	}
	lines := strings.Split(strings.TrimSpace(logged.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want one rate-limited warning:\n%s", len(lines), logged.String())
	}
	for _, want := range []string{"level=WARN", "interface=eth-c", "203.0.113.1", "Fix: add an address in a configured subnet", "listen-interface"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("warning lacks %q:\n%s", want, lines[0])
		}
	}
}
