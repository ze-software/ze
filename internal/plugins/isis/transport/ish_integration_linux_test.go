//go:build integration && linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- ISO 9542 raw transport.
// Goal: prove ISH multicast reaches an actual AF_PACKET receiver without promiscuous mode.
// Method: run the production transport at both ends of an isolated veth pair.

package transport

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// RFC requirement: RFC1195-4.4-2 positive -- the production AF_PACKET transport
// transmits an ISO 9542 ISH to AllESs and receives its intact NET and protocol
// options over the peer veth using normal OSI 802.3/LLC encapsulation.
func TestRFC1195ISHVethTransport(t *testing.T) {
	withVethPair(t, func() {
		tr := New(NewBackend())
		t.Cleanup(tr.Close)
		for _, name := range []string{vethA, vethB} {
			tr.EnableInterface(name, Level1)
			if err := tr.HandleLinkUp(name); err != nil {
				if strings.Contains(err.Error(), "CAP_NET_RAW") {
					t.Skipf("requires CAP_NET_RAW: %v", err)
				}
				t.Fatal(err)
			}
		}
		net, err := types.ParseNET("49.0001.0000.0000.0001.00")
		if err != nil {
			t.Fatal(err)
		}
		h := packet.ISH{NET: net, HoldingTime: 30, TLVs: []packet.TLV{
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
		}}
		var buf [254]byte
		pdu := buf[:h.WriteTo(buf[:], 0)]
		peerIndex, _, _, ok := tr.CircuitInfo(vethB)
		if !ok {
			t.Fatal("peer circuit not open")
		}
		if err := tr.SendISH(vethA, pdu); err != nil {
			t.Fatal(err)
		}
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case frame := <-tr.Receive():
				if frame.IfIndex != peerIndex {
					continue
				}
				if frame.DstMAC != AllESs || !bytes.Equal(frame.PDU, pdu) {
					t.Fatalf("wire ISH mismatch: destination=%x PDU=%x", frame.DstMAC, frame.PDU)
				}
				got, err := packet.DecodeISH(frame.PDU)
				if err != nil {
					t.Fatal(err)
				}
				defer packet.ReleaseTLVs(got.TLVs)
				if !got.NET.Equal(net) || got.HoldingTime != 30 {
					t.Fatalf("decoded wire identity: NET=%s hold=%d", got.NET, got.HoldingTime)
				}
				return
			case <-deadline.C:
				t.Fatal("peer did not receive ISO 9542 multicast")
			}
		}
	})
}
