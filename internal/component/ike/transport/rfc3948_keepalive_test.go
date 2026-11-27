// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive tests

package transport

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// RFC requirement: RFC3948-4-1 positive -- Run emits a 1-byte 0xFF NAT-keepalive on the wire
// at the configured interval (keepalive.go, Run), the mechanism that refreshes the NAT UDP
// binding before it can expire; the server side reads exactly one 0xFF byte.
// RFC requirement: RFC3948-2.3-2 positive -- "The sender MUST use a one-octet-long payload
// with the value 0xFF" (rfc/full/rfc3948.txt, Section 2.3). Keepalive.Run writes the single
// constant octet keepaliveByte = 0xFF (keepalive.go, keepaliveByte and Run), and the peer socket reads exactly
// one byte whose value is 0xFF.
func TestNATKeepalive(t *testing.T) {
	// Create a UDP pair for testing.
	serverAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := net.ListenUDP("udp4", serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverConn.Close() }()

	log := slogutil.DiscardLogger()
	client, err := NewNATTTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	remote, ok := serverConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}

	ka := NewKeepalive(client, nil, remote, 50*time.Millisecond, log)
	go ka.Run()
	defer ka.Stop()

	// Read a keepalive packet from the server side.
	if err := serverConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, _, readErr := serverConn.ReadFromUDP(buf)
	if readErr != nil {
		t.Fatalf("read keepalive: %v", readErr)
	}
	if n != 1 || buf[0] != 0xFF {
		t.Fatalf("keepalive: got %d bytes %x, want 1 byte 0xFF", n, buf[:n])
	}
}

// TestKeepaliveDefaultInterval pins the value of the DefaultKeepaliveInterval constant:
// positive and at most 20 seconds. It proves no behavior; that the keepalive timer runs
// at this default when no interval is set is TestRFC3948UnsetKeepaliveIntervalFallsBackToM.
func TestKeepaliveDefaultInterval(t *testing.T) {
	if DefaultKeepaliveInterval <= 0 {
		t.Fatalf("DefaultKeepaliveInterval = %v, want a positive interval", DefaultKeepaliveInterval)
	}
	if DefaultKeepaliveInterval > 20*time.Second {
		t.Errorf("DefaultKeepaliveInterval = %v, want <= 20s (shorter than a typical NAT UDP binding timeout)", DefaultKeepaliveInterval)
	}
}

// VALIDATES: a NAT-keepalive that arrives on the NAT-T socket is dropped by the transport
// and never becomes a packet a session can read.
// PREVENTS: keepalive reception standing in for liveness. A keepalive leaves a peer on a
// timer and says nothing about whether that peer still holds the IKE SA, so a liveness
// check that counted one would hold a dead SA up until user traffic failed.
//
// RFC requirement: RFC3948-4-3 negative -- reception of NAT-keepalive packets is not used
// to detect whether a connection is live: Run refuses to deliver the datagram at all
// (udp.go, the 28-byte IKE header floor), so no liveness path in the engine can reach one.
func TestNATKeepaliveIsNeverDeliveredToASession(t *testing.T) {
	log := slogutil.DiscardLogger()
	tr, err := NewNATTTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	go tr.Run()

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	if _, err := sender.Write([]byte{0xFF}); err != nil {
		t.Fatalf("write keepalive: %v", err)
	}

	select {
	case pkt := <-tr.Recv():
		t.Fatalf("a NAT-keepalive of %d byte(s) was delivered as a session packet", len(pkt.Data))
	case <-time.After(100 * time.Millisecond):
	}
}

type keepaliveLogCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *keepaliveLogCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // slog.Handler interface requires slog.Record by value
func (h *keepaliveLogCapture) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.records = append(h.records, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *keepaliveLogCapture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *keepaliveLogCapture) WithGroup(string) slog.Handler      { return h }

func (h *keepaliveLogCapture) warns() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []slog.Record
	for i := range h.records {
		if h.records[i].Level == slog.LevelWarn {
			out = append(out, h.records[i])
		}
	}
	return out
}

func recordAttrString(r *slog.Record, key string) (string, bool) {
	var val string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			found = true
			return false
		}
		return true
	})
	return val, found
}

// VALIDATES: consecutive NAT-T keepalive send failures emit exactly one Warn
// line carrying CodeDoctorIPsecNATKeepaliveFailed, a fix command, and the
// report sink outcome, and forward the failure event to the registered sink
// without flooding the log on every tick.
// PREVENTS: failing NAT-T keepalives staying invisible at Debug while NAT
// bindings silently expire, or flooding Warn logs on every keepalive tick.
func TestKeepaliveSendFailureWarnsOncePerCondition(t *testing.T) {
	capture := &keepaliveLogCapture{}
	log := slog.New(capture)

	var (
		sinkMu     sync.Mutex
		sinkEvents []NATKeepaliveEvent
	)
	RegisterNATKeepaliveSink(func(ev NATKeepaliveEvent) slog.Attr {
		sinkMu.Lock()
		sinkEvents = append(sinkEvents, ev)
		sinkMu.Unlock()
		if ev.Kind == NATKeepaliveEventFailed {
			return slog.String("report", "queued")
		}
		return slog.Attr{}
	})
	t.Cleanup(func() { RegisterNATKeepaliveSink(nil) })

	client, err := NewNATTTransport("127.0.0.1:0", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	bound, ok := client.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	badPort := 1
	if bound.Port == 1 {
		badPort = 2
	}
	local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: badPort}
	remote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4500}

	ka := NewKeepalive(client, local, remote, 15*time.Millisecond, log)
	go ka.Run()
	time.Sleep(85 * time.Millisecond)
	ka.Stop()

	warns := capture.warns()
	if len(warns) != 1 {
		t.Fatalf("warn count = %d, want 1 across consecutive keepalive send failures", len(warns))
	}
	if code, ok := recordAttrString(&warns[0], "code"); !ok || code != CodeDoctorIPsecNATKeepaliveFailed {
		t.Errorf("warn code = %q (present=%v), want %q", code, ok, CodeDoctorIPsecNATKeepaliveFailed)
	}
	if fix, ok := recordAttrString(&warns[0], "fix"); !ok || fix != "ze show vpn ipsec sa" {
		t.Errorf("warn fix = %q (present=%v), want %q", fix, ok, "ze show vpn ipsec sa")
	}
	if rep, ok := recordAttrString(&warns[0], "report"); !ok || rep != "queued" {
		t.Errorf("warn report = %q (present=%v), want %q", rep, ok, "queued")
	}

	sinkMu.Lock()
	var failedCount int
	for _, ev := range sinkEvents {
		if ev.Kind == NATKeepaliveEventFailed && ev.Err != nil {
			failedCount++
		}
	}
	sinkMu.Unlock()
	if failedCount == 0 {
		t.Errorf("sink received 0 NATKeepaliveEventFailed events, want at least 1")
	}
}
