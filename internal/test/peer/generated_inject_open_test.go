// Design: docs/architecture/testing/ci-format.md -- generated UPDATE session attributes.
// Related: generated_bulk_open_test.go -- independent complete-frame wire assertions.
// Related: inject.go -- active OPEN exchange and doInject.

package peer

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"testing"
	"time"
)

// TestGeneratedInjectAttributesFollowOPEN captures the separate ModeInject
// producer over actual active/passive TCP handshakes. In active eBGP, no OpenAS
// declaration exists: the sent OPEN actually uses Inject.ASN, while the passive
// OPEN default mirrors the remote. Reusing the passive resolver in the active
// entry therefore cannot satisfy the external captured-wire control.
//
// This carrier covers neither connmap injection nor the scripted raw-send arm.
// RFC 4271 Section 5.1.5 is checked by checkBulkOPENUpdate on captured frames.
// MUTATION: classify active injection with the passive mirror instead of its
// actual sent OPEN, or drop the passive OPEN classification at doInject.
func TestGeneratedInjectAttributesFollowOPEN(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		for _, session := range []string{"ibgp", "ebgp"} {
			for _, rail := range []string{"active-inject", "passive-inject-accepted"} {
				t.Run(family+"/"+session+"/"+rail, func(t *testing.T) {
					runGeneratedInjectOPEN(t, family, session, rail)
				})
			}
		}
	}
}

func runGeneratedInjectOPEN(t *testing.T, family, session, rail string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	network, host := "tcp4", "127.0.0.1"
	prefix, nextHop := "10.82.0.0/24", "192.0.2.82"
	if family == "ipv6" {
		network, host = "tcp6", "::1"
		prefix, nextHop = "2001:db8:82::/64", "2001:db8::82"
	}
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, network, net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatalf("owned %s listener: %v (IPv6 loopback is a prerequisite; no skip)", network, err)
	}
	defer func() { _ = listener.Close() }()
	stopListener := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopListener()
	var transcript bytes.Buffer
	config := &Config{
		Mode: ModeInject, Output: &transcript,
		Inject: &InjectSpec{
			Prefix: netip.MustParsePrefix(prefix), NextHop: netip.MustParseAddr(nextHop),
			ASN: bulkOPENExternalAS, Count: 5, PerUpdate: 2, MaxMsgLen: 4096, EndOfRIB: true,
		},
	}
	wantAS := bulkOPENRemoteAS
	if session == "ebgp" {
		wantAS = bulkOPENExternalAS
	}
	if rail == "active-inject" {
		config.Dial = listener.Addr().String()
		if session == "ibgp" {
			config.OpenAS = []OpenASBinding{{AS: wantAS}}
		}
	} else if session == "ebgp" {
		config.OpenAS = []OpenASBinding{{Addr: netip.MustParseAddr(host), AS: wantAS}}
	}
	p, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan Result, 1)
	go runBulkOPENPeer(ctx, p, listener, result)
	defer func() {
		cancel()
		select {
		case outcome := <-result:
			if !outcome.Success {
				t.Errorf("actual inject entry: %v", outcome.Error)
			}
			t.Logf("peer transcript:\n%s", transcript.String())
		case <-time.After(2 * time.Second):
			t.Error("actual inject entry did not join after owned socket close")
		}
	}()
	var conn net.Conn
	if config.Dial != "" {
		conn, err = listener.Accept()
	} else {
		var dialer net.Dialer
		conn, err = dialer.DialContext(ctx, network, listener.Addr().String())
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	stopConn := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopConn()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	remoteOpen := bulkOPENMessage(family)
	var peerOpen []byte
	if config.Dial != "" {
		peerOpen = readBulkOPENFrame(t, conn)
		writeBulkOPENFrame(t, conn, remoteOpen)
	} else {
		writeBulkOPENFrame(t, conn, remoteOpen)
		peerOpen = readBulkOPENFrame(t, conn)
	}
	peerAS := bulkOPENReadAS(t, peerOpen)
	remoteAS := bulkOPENReadAS(t, remoteOpen)
	if peerAS != wantAS {
		t.Errorf("actual inject OPEN AS=%d want=%d", peerAS, wantAS)
	}
	if (peerAS == remoteAS) != (session == "ibgp") {
		t.Errorf("actual inject OPEN pair %d/%d does not establish %s", peerAS, remoteAS, session)
	}
	keepalive := readBulkOPENFrame(t, conn)
	if !bytes.Equal(keepalive, KeepaliveMsg()) {
		t.Fatalf("actual inject OPEN did not yield KEEPALIVE: %x", keepalive)
	}
	// RFC 4271 Section 8.2.2: complete the actual OPEN/KEEPALIVE exchange.
	writeBulkOPENFrame(t, conn, KeepaliveMsg())
	var frames [3][]byte
	for i := range frames {
		frames[i] = readBulkOPENFrame(t, conn)
	}
	eor := readBulkOPENFrame(t, conn)
	if hex.EncodeToString(eor) != bulkOPENAck {
		t.Errorf("actual inject stream lacks complete EOR: %x", eor)
	}
	// Closing this owned remote is the producer's supported dwell completion
	// event. No timed dwell or context-cancellation success replaces the wire.
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-result:
		result <- outcome
		if !outcome.Success {
			t.Fatalf("actual inject completion: %v", outcome.Error)
		}
	case <-ctx.Done():
		t.Fatal("actual injection did not finish after complete transmission and remote close")
	}
	var prefixes []netip.Prefix
	for _, frame := range frames {
		// RFC 4271 Section 5.1.5: classify solely from the actual OPEN pair.
		prefixes = append(prefixes, checkBulkOPENUpdate(t, frame, family, peerAS == remoteAS)...)
	}
	want := []string{"10.82.0.0/24", "10.82.1.0/24", "10.82.2.0/24", "10.82.3.0/24", "10.82.4.0/24"}
	if family == "ipv6" {
		want = []string{"2001:db8:82::/64", "2001:db8:82:1::/64", "2001:db8:82:2::/64", "2001:db8:82:3::/64", "2001:db8:82:4::/64"}
	}
	if len(prefixes) != len(want) {
		t.Fatalf("actual inject prefixes=%v want=%v", prefixes, want)
	}
	for i, prefix := range want {
		if prefixes[i] != netip.MustParsePrefix(prefix) {
			t.Errorf("actual inject prefix%d=%s want=%s", i, prefixes[i], prefix)
		}
	}
}
