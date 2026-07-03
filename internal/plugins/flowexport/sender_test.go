package flowexport

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestBufferPoolGetPut(t *testing.T) {
	b := GetBuf()
	if b == nil {
		t.Fatal("GetBuf returned nil")
	}
	if len(*b) != MaxDatagramSize {
		t.Fatalf("buffer size = %d, want %d", len(*b), MaxDatagramSize)
	}

	// Write some data, put back, get again: pool reuses.
	(*b)[0] = 0xFF
	PutBuf(b)

	b2 := GetBuf()
	if b2 == nil {
		t.Fatal("second GetBuf returned nil")
	}
	if len(*b2) != MaxDatagramSize {
		t.Fatalf("second buffer size = %d, want %d", len(*b2), MaxDatagramSize)
	}
	PutBuf(b2)
}

func TestPutBufNil(t *testing.T) {
	PutBuf(nil) // must not panic
}

// TestFlowExportSourceAddress verifies the collector source-address leaf is
// parsed into CollectorConfig (via parseCollectorMap, the real production path,
// NOT the struct json tag) and that NewSender binds it as the UDP local source.
//
// VALIDATES: AC-7/AC-8 -- source-address flows YANG->config->UDP source bind.
// PREVENTS: regression of the wiring bug where parseCollectorMap ignored
// source-address, leaving the json tag dead and the feature inert.
func TestFlowExportSourceAddress(t *testing.T) {
	// Keyed-map form (as YANG delivers a list) and array form both route
	// through parseCollectorMap; test the keyed-map form here.
	data := `{"flow-export":{"collector":{
		"c1":{"address":"10.0.0.50","port":6343,"source-address":"192.168.1.1","protocol":"sflow"}
	}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 1 {
		t.Fatalf("expected 1 collector, got %d", len(cfg.Collectors))
	}
	if cfg.Collectors[0].SourceAddress != "192.168.1.1" {
		t.Fatalf("SourceAddress = %q, want %q", cfg.Collectors[0].SourceAddress, "192.168.1.1")
	}

	// A source-address not assigned to any local interface must fail the UDP
	// bind -- proving NewSender actually applies it as the socket's local addr.
	// 192.0.2.2 is RFC 5737 TEST-NET-1 (not a local address). The error must
	// name source-address so the misconfiguration is diagnosable.
	_, bindErr := NewSender("127.0.0.1", 9995, "192.0.2.2")
	if bindErr == nil {
		t.Fatal("expected bind failure for non-local source-address, got nil")
	}
	if !strings.Contains(bindErr.Error(), "source-address") {
		t.Errorf("bind error = %v, want it to mention source-address", bindErr)
	}

	// An unparseable source-address is rejected, not silently treated as a
	// wildcard bind.
	if _, err := NewSender("127.0.0.1", 9995, "not-an-ip"); err == nil {
		t.Fatal("expected error for invalid source-address, got nil")
	}

	// A loopback source is assignable: NewSender succeeds.
	s, err := NewSender("127.0.0.1", 9995, "127.0.0.1")
	if err != nil {
		t.Fatalf("NewSender with loopback source: %v", err)
	}
	if closeErr := s.Close(); closeErr != nil {
		t.Logf("close: %v", closeErr)
	}
}

func TestSenderUDP(t *testing.T) {
	// Listen on a random loopback port.
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

	s, err := NewSender("127.0.0.1", addr.Port, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	payload := []byte{0x01, 0x02, 0x03, 0x04}
	if err := s.Send(payload); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 64)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) {
		t.Fatalf("received %d bytes, want %d", n, len(payload))
	}
	for i := range payload {
		if buf[i] != payload[i] {
			t.Fatalf("byte %d: got %02x, want %02x", i, buf[i], payload[i])
		}
	}

	datagrams, bytes, errors := s.Stats()
	if datagrams != 1 {
		t.Fatalf("datagrams = %d, want 1", datagrams)
	}
	if bytes != 4 {
		t.Fatalf("bytes = %d, want 4", bytes)
	}
	if errors != 0 {
		t.Fatalf("errors = %d, want 0", errors)
	}
}
