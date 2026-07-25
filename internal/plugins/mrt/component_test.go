// VALIDATES: MRT record gating — New() precomputes the peer set and direction
// filter from config, and shouldRecord honors direction (received/sent/both)
// and peer-filter membership.
// PREVENTS: MRT dumps capturing the wrong direction or peers the operator
// explicitly excluded (or dropping peers they wanted).
package mrt

import (
	"bytes"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/plugin"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

func peerAt(addr string) *plugin.PeerInfo {
	return &plugin.PeerInfo{Address: netip.MustParseAddr(addr)}
}

func TestShouldRecordBothDirectionsNoFilter(t *testing.T) {
	c := New(Config{}, nil)
	p := peerAt("10.0.0.1")
	if !c.shouldRecord(p, true) || !c.shouldRecord(p, false) {
		t.Error("default config should record both sent and received")
	}
}

func TestShouldRecordReceivedOnly(t *testing.T) {
	c := New(Config{Direction: "received"}, nil)
	p := peerAt("10.0.0.1")
	if c.shouldRecord(p, true) {
		t.Error("received-only recorded a sent message")
	}
	if !c.shouldRecord(p, false) {
		t.Error("received-only dropped a received message")
	}
}

func TestShouldRecordSentOnly(t *testing.T) {
	c := New(Config{Direction: "sent"}, nil)
	p := peerAt("10.0.0.1")
	if !c.shouldRecord(p, true) {
		t.Error("sent-only dropped a sent message")
	}
	if c.shouldRecord(p, false) {
		t.Error("sent-only recorded a received message")
	}
}

func TestShouldRecordPeerFilter(t *testing.T) {
	c := New(Config{PeerFilter: []string{"10.0.0.1", "2001:db8::1"}}, nil)
	if !c.shouldRecord(peerAt("10.0.0.1"), false) {
		t.Error("filtered-in IPv4 peer dropped")
	}
	if !c.shouldRecord(peerAt("2001:db8::1"), false) {
		t.Error("filtered-in IPv6 peer dropped")
	}
	if c.shouldRecord(peerAt("10.0.0.99"), false) {
		t.Error("peer outside the filter was recorded")
	}
}

func TestShouldRecordDirectionAndPeerCombined(t *testing.T) {
	c := New(Config{Direction: "received", PeerFilter: []string{"10.0.0.1"}}, nil)
	// Right peer, wrong direction → dropped.
	if c.shouldRecord(peerAt("10.0.0.1"), true) {
		t.Error("combined filter recorded a sent message from the allowed peer")
	}
	// Right peer, right direction → kept.
	if !c.shouldRecord(peerAt("10.0.0.1"), false) {
		t.Error("combined filter dropped an allowed received message")
	}
	// Wrong peer, right direction → dropped.
	if c.shouldRecord(peerAt("10.0.0.2"), false) {
		t.Error("combined filter recorded a disallowed peer")
	}
}

func TestOneBGPMessagePerBGP4MPRecord(t *testing.T) {
	// RFC requirement: RFC6396-4.4.2-2 positive -- a BGP4MP_MESSAGE record encapsulates
	// exactly one BGP message [SHALL]. OnBGPMessage builds a single record per call with
	// one WriteBGP4MPMessage and writes it once (internal/plugins/mrt/component.go:99-143),
	// and a BGP4MP MESSAGE record's body is exactly that one encapsulated message
	// (WriteBGP4MPMessage, internal/mrt/encode.go:167-173). This drives one OnBGPMessage
	// call and asserts the dump holds exactly one BGP4MP MESSAGE record whose body equals
	// the single input message byte-for-byte (no second message appended).
	c := New(Config{}, nil)
	path := filepath.Join(t.TempDir(), "all.mrt")
	c.allMsgs = newAsyncWriter(mrtfmt.NewWriter(path), c.logger)

	peer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.0.2.1"),
		LocalAddress: netip.MustParseAddr("192.0.2.2"),
		PeerAS:       65001,
		LocalAS:      65002,
	}
	// A single well-formed 19-byte BGP KEEPALIVE: 16-byte marker + length 19 + type 4.
	bgpMsg := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x13, 0x04,
	}
	c.OnBGPMessage(peer, msgtype.TypeUPDATE, false, bgpMsg)
	if err := c.allMsgs.Close(); err != nil {
		t.Fatalf("close all writer: %v", err)
	}

	var messages [][]byte
	h := &mrtfmt.Handler{
		OnMessage: func(_ mrtfmt.Header, _ uint32, m *mrtfmt.MessageRecord) error {
			messages = append(messages, bytes.Clone(m.BGPMessage))
			return nil
		},
	}
	if err := mrtfmt.ReadFile(path, h); err != nil {
		t.Fatalf("read back dump: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("BGP4MP MESSAGE record count = %d, want exactly 1", len(messages))
	}
	if !bytes.Equal(messages[0], bgpMsg) {
		t.Errorf("encapsulated message = % x, want % x (record must carry exactly the one message)",
			messages[0], bgpMsg)
	}
}

// TestNewIgnoresMalformedPeerFilter proves a bad address in the filter is
// skipped (not fatal) while valid entries still gate.
func TestNewIgnoresMalformedPeerFilter(t *testing.T) {
	c := New(Config{PeerFilter: []string{"not-an-ip", "10.0.0.1"}}, nil)
	if !c.shouldRecord(peerAt("10.0.0.1"), false) {
		t.Error("valid peer dropped when filter also had a malformed entry")
	}
	if c.shouldRecord(peerAt("10.0.0.2"), false) {
		t.Error("filter with a valid entry still recorded a non-member")
	}
}
