package ppp

import (
	"errors"
	"net/netip"
	"testing"
	"time"
)

// VALIDATES: ParseIPCPOptions decodes IP-Address (type 3) plus both DNS
//
//	options (RFC 1877 types 129/131) into the struct.
func TestIPCPParseOptions(t *testing.T) {
	// IP-Address=192.168.1.10 + Primary-DNS=1.1.1.1 + Secondary-DNS=8.8.8.8.
	buf := []byte{
		3, 6, 192, 168, 1, 10,
		129, 6, 1, 1, 1, 1,
		131, 6, 8, 8, 8, 8,
	}
	opts, err := ParseIPCPOptions(buf)
	if err != nil {
		t.Fatalf("ParseIPCPOptions: %v", err)
	}
	if !opts.HasIPAddress || opts.IPAddress != netip.MustParseAddr("192.168.1.10") {
		t.Errorf("IPAddress = %+v, want 192.168.1.10", opts)
	}
	if !opts.HasPrimary || opts.PrimaryDNS != netip.MustParseAddr("1.1.1.1") {
		t.Errorf("PrimaryDNS = %+v, want 1.1.1.1", opts)
	}
	if !opts.HasSecondary || opts.SecondaryDNS != netip.MustParseAddr("8.8.8.8") {
		t.Errorf("SecondaryDNS = %+v, want 8.8.8.8", opts)
	}
}

// VALIDATES: WriteIPCPOptions + ParseIPCPOptions round-trip preserves
//
//	every populated field.
func TestIPCPRoundtrip(t *testing.T) {
	src := IPCPOptions{
		IPAddress:    netip.MustParseAddr("10.0.0.5"),
		HasIPAddress: true,
		PrimaryDNS:   netip.MustParseAddr("9.9.9.9"),
		HasPrimary:   true,
		SecondaryDNS: netip.MustParseAddr("149.112.112.112"),
		HasSecondary: true,
	}
	buf := make([]byte, 64)
	n := WriteIPCPOptions(buf, 0, src)
	if n != 3*6 {
		t.Fatalf("WriteIPCPOptions wrote %d bytes, want 18", n)
	}
	got, err := ParseIPCPOptions(buf[:n])
	if err != nil {
		t.Fatalf("ParseIPCPOptions: %v", err)
	}
	if got != src {
		t.Errorf("roundtrip mismatch: got %+v want %+v", got, src)
	}
}

// VALIDATES: ParseIPCPOptions rejects structurally malformed wire data
//
//	(short option, wrong length, truncated buffer). PREVENTS:
//	out-of-bounds reads on hostile input.
func TestIPCPParseRejects(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		wantErr error
	}{
		{"too short", []byte{3}, errOptionTooShort},
		{"len below header", []byte{3, 1}, errOptionLengthMismatch},
		{"len exceeds buf", []byte{3, 10, 1, 2, 3}, errOptionLengthMismatch},
		{"wrong address length", []byte{3, 7, 1, 2, 3, 4, 5}, errIPCPBadOptionLen},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseIPCPOptions(tc.buf)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// VALIDATES: ipcpHasUnknownOption flags any option type not in the
//
//	recognized set {3, 129, 131}. PREVENTS: Configure-Ack on
//	options we cannot interpret (RFC 1661 §5.4).
func TestIPCPUnknownOption(t *testing.T) {
	known := []byte{3, 6, 1, 2, 3, 4}
	if ipcpHasUnknownOption(known) {
		t.Error("known options flagged as unknown")
	}
	mixed := []byte{3, 6, 1, 2, 3, 4, 99, 4, 0xDE, 0xAD}
	if !ipcpHasUnknownOption(mixed) {
		t.Error("mixed options not flagged")
	}
}

// VALIDATES: AC-3 (partial), AC-27 -- when ze sends its initial CONFREQ
//
//	AFTER EventIPRequest is answered, the CR carries the assigned
//	local address (deviation from AC-2 text; LNS role).
//
// PREVENTS: ze emitting CONFREQ with IP-Address=0.0.0.0 which would
//
//	force the peer to negotiate on ze's behalf.
func TestIPCPInitialRequestEmitsEvent(t *testing.T) {
	td := newNCPTestDriver(t)
	defer td.cleanup()

	// Emit EventIPRequest is observed by the NCP handler goroutine the
	// test helper starts; the first CONFREQ we observe on the wire must
	// carry the assigned local address.
	pkt := td.readPeerNCPPacket(t, ProtoIPCP)
	if pkt.Code != LCPConfigureRequest {
		t.Fatalf("first IPCP code = %d, want CONFREQ", pkt.Code)
	}
	opts, err := ParseIPCPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseIPCPOptions: %v", err)
	}
	if !opts.HasIPAddress {
		t.Fatalf("initial CR missing IP-Address option")
	}
	if opts.IPAddress != ipcpTestLocal {
		t.Errorf("initial CR IP-Address = %v, want %v (assigned local)",
			opts.IPAddress, ipcpTestLocal)
	}
}

// VALIDATES: AC-16 -- peer's Configure-Reject of IP-Address tears down
//
//	the session.
//
// PREVENTS: silent drop where IPCP reject leaves the session without
//
//	IPv4 connectivity but L2TP reports the session as up.
func TestIPCPRejectTearsDown(t *testing.T) {
	td := newNCPTestDriverCfg(t, StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	initial := td.readPeerNCPPacket(t, ProtoIPCP)
	// Reply with Configure-Reject of the IP-Address option so ze's
	// FSM sees a fatal outcome.
	reject := []byte{3, 6, 0, 0, 0, 0}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureReject, initial.Identifier, reject)

	ev := td.waitForEvent(t, 2*time.Second)
	if _, ok := ev.(EventSessionDown); !ok {
		t.Fatalf("event = %T, want EventSessionDown", ev)
	}
}
