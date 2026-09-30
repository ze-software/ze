// Design: docs/architecture/bfd.md -- authenticated Control packet transmit
// Related: loop.go -- sendLocked signs every Control packet and advances bfd.XmitAuthSeq
//
// VALIDATES: RFC 5880 Section 6.7.4 -- "bfd.XmitAuthSeq SHOULD be incremented
// when the session state changes, or when the transmitted BFD Control packet
// carries different contents than the previously transmitted packet." A Keyed
// SHA1 session is driven through the engine's real transmit path (Loop.tick for
// the periodic packets, Loop.handleInbound for the Final answering a Poll) and
// every packet handed to the transport is parsed for its state, its flags and
// the Sequence Number of its authentication section.
// PREVENTS: a transmit path that signs without advancing the counter; one that
// advances it only after a periodic packet, so the periodic packet after a
// Final repeats the Final's sequence (the positive); and one that advances it
// only when the periodic timer fires, before the periodic packet, so a Final
// sent outside the timer repeats the sequence of the packet before it (the
// negative).
package engine

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/auth"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// xmitSeqTransport records a copy of every Control packet handed to Send. The
// engine releases the pooled buffer when Send returns, so the bytes are copied.
// Not safe for concurrent use: the tests never Start the loop.
type xmitSeqTransport struct {
	packets [][]byte
}

func (*xmitSeqTransport) Start() error { return nil }
func (*xmitSeqTransport) Stop() error  { return nil }
func (c *xmitSeqTransport) Send(o transport.Outbound) error {
	c.packets = append(c.packets, append([]byte(nil), o.Bytes...))
	return nil
}
func (*xmitSeqTransport) RX() <-chan transport.Inbound { return nil }

// xmitSeqPacket is what one transmitted packet says about the rule: the state
// and flags a reader compares, and the Sequence Number it was signed with.
type xmitSeqPacket struct {
	control packet.Control
	seq     uint32
}

// xmitSeqLast parses the most recent packet the transport recorded. The Keyed
// SHA1 section starts right after the mandatory section; its Sequence Number
// follows Auth Type, Auth Len, Key ID and Reserved (RFC 5880 Section 4.4).
func xmitSeqLast(t *testing.T, ct *xmitSeqTransport) xmitSeqPacket {
	t.Helper()
	if len(ct.packets) == 0 {
		t.Fatal("no Control packet was transmitted")
	}
	raw := ct.packets[len(ct.packets)-1]
	c, _, err := packet.ParseControl(raw)
	if err != nil {
		t.Fatalf("ParseControl of a transmitted packet: %v", err)
	}
	if !c.Auth {
		t.Fatal("a Keyed SHA1 session transmitted a packet without the A bit")
	}
	if len(raw) < packet.MandatoryLen+8 {
		t.Fatalf("transmitted packet is %d octets, too short for a Keyed SHA1 section", len(raw))
	}
	return xmitSeqPacket{control: c, seq: binary.BigEndian.Uint32(raw[packet.MandatoryLen+4:])}
}

// xmitSeqLoop builds an unstarted Loop on a stepped clock with one Keyed SHA1
// single-hop session, and a signer holding the same key so the test can play
// the peer.
func xmitSeqLoop(t *testing.T) (*Loop, *xmitSeqTransport, *steppedClock, api.Key, auth.Signer) {
	t.Helper()
	settings := auth.Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 5, Secret: rfc5880Secret}
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	ct := &xmitSeqTransport{}
	l := NewLoop(ct, clk)
	req := reqFor(addrB, addrA)
	req.Auth = &api.AuthSettings{Type: settings.Type, KeyID: settings.KeyID, Secret: settings.Secret}
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	signer, err := auth.NewSigner(settings)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return l, ct, clk, req.Key(), signer
}

// xmitSeqPeer builds the peer's Keyed SHA1 Control packet in state, naming
// yd, signed with seq, with the Poll bit set when poll is true.
func xmitSeqPeer(key api.Key, signer auth.Signer, state packet.State, yd, seq uint32, poll bool) transport.Inbound {
	c := packet.Control{
		Version:               packet.Version,
		State:                 state,
		Poll:                  poll,
		Auth:                  true,
		DetectMult:            3,
		Length:                uint8(packet.MandatoryLen + signer.BodyLen()),
		MyDiscriminator:       peerMyDiscr,
		YourDiscriminator:     yd,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
	}
	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c.WriteTo(buf, 0)
	signer.Sign(buf, packet.MandatoryLen, seq)
	return transport.Inbound{
		From:      key.Peer,
		Local:     key.Local,
		Interface: key.Interface,
		Mode:      api.SingleHop,
		TTL:       255,
		Bytes:     buf,
	}
}

// xmitSeqPeriodic moves the clock to the session's next transmit deadline,
// delivers in (when not nil) at that instant so the Detection Time runs from
// it, and ticks once: the periodic packet that tick sends is returned.
func xmitSeqPeriodic(t *testing.T, l *Loop, ct *xmitSeqTransport, clk *steppedClock, key api.Key, in *transport.Inbound) xmitSeqPacket {
	t.Helper()
	m := machineFor(t, l, key)
	if next := m.NextTxDeadline(); next.After(clk.now) {
		clk.now = next
	}
	if in != nil {
		l.handleInbound(*in)
	}
	before := len(ct.packets)
	l.tick()
	if len(ct.packets) != before+1 {
		t.Fatalf("tick at the transmit deadline sent %d packets, want 1", len(ct.packets)-before)
	}
	return xmitSeqLast(t, ct)
}

// RFC requirement: RFC5880-6.7.4-9 positive -- "bfd.XmitAuthSeq SHOULD be
// incremented when the session state changes, or when the transmitted BFD
// Control packet carries different contents than the previously transmitted
// packet." A Keyed SHA1 session sends its periodic packets through Loop.tick.
// State changes: Down, then Init after the peer's Down, then Up after the
// peer's Init; each packet carries a different Sequence Number from the one
// before. Content change with no state change: the Final answering the peer's
// Poll (State Up, F set) is followed by a periodic packet (State Up, F clear),
// and that periodic packet's Sequence Number differs from the Final's.
func TestRFC5880XmitAuthSeqAdvancesOnStateAndContentChange(t *testing.T) {
	l, ct, clk, key, signer := xmitSeqLoop(t)
	m := machineFor(t, l, key)

	down := xmitSeqPeriodic(t, l, ct, clk, key, nil)
	peerDown := xmitSeqPeer(key, signer, packet.StateDown, 0, 100, false)
	initPkt := xmitSeqPeriodic(t, l, ct, clk, key, &peerDown)
	peerInit := xmitSeqPeer(key, signer, packet.StateInit, m.LocalDiscriminator(), 101, false)
	up := xmitSeqPeriodic(t, l, ct, clk, key, &peerInit)

	for _, step := range []struct {
		name         string
		prev, next   xmitSeqPacket
		stateBefore  packet.State
		stateChanged packet.State
	}{
		{"Down to Init", down, initPkt, packet.StateDown, packet.StateInit},
		{"Init to Up", initPkt, up, packet.StateInit, packet.StateUp},
	} {
		if step.prev.control.State != step.stateBefore || step.next.control.State != step.stateChanged {
			t.Fatalf("%s: precondition: transmitted states %v then %v", step.name, step.prev.control.State, step.next.control.State)
		}
		if step.next.seq == step.prev.seq {
			t.Errorf("%s: the session state changed and the Sequence Number stayed %d", step.name, step.next.seq)
		}
	}

	l.handleInbound(xmitSeqPeer(key, signer, packet.StateUp, m.LocalDiscriminator(), 102, true))
	final := xmitSeqLast(t, ct)
	after := xmitSeqPeriodic(t, l, ct, clk, key, nil)
	if !final.control.Final || after.control.Final {
		t.Fatalf("precondition: F bit %v on the reply to the Poll, %v on the next periodic packet", final.control.Final, after.control.Final)
	}
	if final.control.State != packet.StateUp || after.control.State != packet.StateUp {
		t.Fatalf("precondition: states %v then %v, want Up with no state change", final.control.State, after.control.State)
	}
	if after.seq == final.seq {
		t.Errorf("the periodic packet differs from the Final before it (F clear) and repeats its Sequence Number %d", after.seq)
	}
}

// RFC requirement: RFC5880-6.7.4-9 negative -- the Final answering a Poll is
// sent from Loop.handleInbound, outside the periodic timer, at the same
// instant as the periodic packet before it: the input that would make a
// transmit path advancing the counter only when its timer fires repeat a
// Sequence Number. The Final differs from that packet (F set) and still carries a
// Sequence Number different from it, and every packet of the run whose state
// or contents differ from the previous one carries a new Sequence Number.
func TestRFC5880XmitAuthSeqAdvancesForAnOutOfTimerFinal(t *testing.T) {
	l, ct, clk, key, signer := xmitSeqLoop(t)
	m := machineFor(t, l, key)

	xmitSeqPeriodic(t, l, ct, clk, key, nil)
	peerDown := xmitSeqPeer(key, signer, packet.StateDown, 0, 200, false)
	xmitSeqPeriodic(t, l, ct, clk, key, &peerDown)
	peerInit := xmitSeqPeer(key, signer, packet.StateInit, m.LocalDiscriminator(), 201, false)
	periodic := xmitSeqPeriodic(t, l, ct, clk, key, &peerInit)

	sentAt := clk.now
	l.handleInbound(xmitSeqPeer(key, signer, packet.StateUp, m.LocalDiscriminator(), 202, true))
	final := xmitSeqLast(t, ct)
	if !clk.now.Equal(sentAt) {
		t.Fatal("precondition: the clock moved between the periodic packet and the Final")
	}
	if !final.control.Final || periodic.control.Final {
		t.Fatalf("precondition: F bit %v on the periodic packet, %v on the reply to the Poll", periodic.control.Final, final.control.Final)
	}
	if final.seq == periodic.seq {
		t.Errorf("the Final sent outside the timer repeats the Sequence Number %d of the periodic packet before it", final.seq)
	}

	for i := 1; i < len(ct.packets); i++ {
		prev, next := ct.packets[i-1], ct.packets[i]
		if bytes.Equal(prev[:packet.MandatoryLen], next[:packet.MandatoryLen]) {
			continue
		}
		prevSeq := binary.BigEndian.Uint32(prev[packet.MandatoryLen+4:])
		nextSeq := binary.BigEndian.Uint32(next[packet.MandatoryLen+4:])
		if nextSeq == prevSeq {
			t.Errorf("packet %d differs from packet %d and repeats its Sequence Number %d", i, i-1, nextSeq)
		}
	}
}
