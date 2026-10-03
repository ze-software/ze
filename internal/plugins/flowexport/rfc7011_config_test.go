// RFC 7011 conformance tests for the exporter's collector port. The port is a
// config surface (config.go) reached by the UDP sender (sender.go), so the test
// lives here rather than in the ipfix encoder package.
//
// VALIDATES: an operator can point the Exporting Process at a port other than
// the default, and the sender dials that port; a port outside 1-65535 is
// refused before a sender is built.
// PREVENTS: the port leaf silently staying at the default, or a garbage port
// reaching the socket.

package flowexport

import (
	"context"
	"net"
	"testing"
	"time"
)

// RFC requirement: RFC7011-10.1-1 positive -- a collector configured with
// port 4739 parses to CollectorConfig.Port 4739 rather than the 6343 default,
// and a Sender built from a parsed non-default port delivers its datagram to
// that port.
func TestRFC7011CollectorPortConfigurable(t *testing.T) {
	cfg, err := ParseConfig(`{"flow-export":{"collector":[{"name":"c1","address":"127.0.0.1","port":4739,"protocol":"ipfix"}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Collectors[0].Port != 4739 {
		t.Fatalf("port = %d, want 4739", cfg.Collectors[0].Port)
	}

	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	data := `{"flow-export":{"collector":[{"name":"c1","address":"127.0.0.1","port":` +
		itoa(addr.Port) + `,"protocol":"ipfix"}]}}`
	cfg, err = ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.Collectors[0]
	if c.Port != addr.Port {
		t.Fatalf("port = %d, want %d", c.Port, addr.Port)
	}
	s, err := NewSender(c.Address, c.Port, "", DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Send([]byte{0, 0x0a}); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	if err := pc.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("datagram did not reach port %d: %v", addr.Port, err)
	}
	if n != 2 {
		t.Fatalf("received %d octets, want 2", n)
	}
}

// RFC requirement: RFC7011-10.1-1 negative -- a collector port of 0 or 65536
// is refused by Validate, so a port outside the configurable range never
// reaches NewSender.
func TestRFC7011CollectorPortOutOfRangeRefused(t *testing.T) {
	for _, port := range []int{0, 65536} {
		cfg := &Config{Collectors: []CollectorConfig{
			{Name: "c1", Address: "127.0.0.1", Port: port, Protocol: "ipfix", PollingInterval: 20, TemplateRefresh: 600, MaxDatagramSize: DatagramSizeDefault},
		}}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("port %d accepted, want refusal", port)
		}
	}
}

// itoa formats a port for the JSON fixture without pulling fmt into the test.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
