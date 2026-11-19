// Design: docs/architecture/buffer-architecture.md -- retained decoded OPEN bodies.
// Related: session_open_publication.go -- bounded private OPEN ownership.
package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPrivateOpenPublicationOwnsBorrowedBodies overwrites both borrowed inputs
// before selection. A real read-pool handle stays with its original owner;
// selected publication must still deliver the exact observed pair once.
func TestPrivateOpenPublicationOwnsBorrowedBodies(t *testing.T) {
	epoch := newPrivateNegotiationEpoch()
	var got []incomingDecodedOpenObservation
	var receipts []BufHandle
	callback := epoch.messageCallback(func(_ *Session, _ netip.Addr, _ msgtype.MessageType, body []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		buffer BufHandle, _ map[string]any, _ string, _ uint64) bool {
		got = append(got, incomingDecodedOpenObservation{direction: direction, body: bytes.Clone(body)})
		receipts = append(receipts, buffer)
		return false
	})
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	session := NewSession(settings)
	session.onMessageReceived = callback
	localWire := message.PackTo(&message.Open{Version: 4, MyAS: 65000, HoldTime: 90,
		BGPIdentifier: 0x01010101}, nil)
	remoteWire := message.PackTo(delayOpenRemote(0x09090909), nil)
	frame, err := session.readFrame(nil, bufio.NewReader(bytes.NewReader(remoteWire)))
	if err != nil {
		t.Fatal(err)
	}
	defer session.returnReadBuffer(frame.buffer)
	localBody := localWire[message.HeaderLen:]
	wantLocal := bytes.Clone(localBody)
	wantRemote := bytes.Clone(frame.body())
	if callback(nil, settings.Address, msgtype.TypeOPEN, localBody, nil, 0, rpc.DirectionSent, BufHandle{}, nil, "", 0) {
		t.Fatal("private local OPEN took ownership of its writer's buffer")
	}
	if callback(nil, settings.Address, msgtype.TypeOPEN, frame.body(), nil, 0, rpc.DirectionReceived, frame.buffer, nil, "", 0) {
		t.Fatal("private remote OPEN took ownership of the read-pool handle")
	}
	if len(got) != 0 {
		t.Fatal("private OPEN bodies were published before selection")
	}
	clear(localBody)
	clear(frame.body())
	session.openMu.Lock()
	defer session.unlockOpen()
	observed := epoch.selectMessages()
	if len(observed) != 2 {
		t.Fatalf("retained OPEN observations=%d, want exact pair", len(observed))
	}
	for _, event := range observed {
		if err := event.publish(session); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("selected OPEN observations=%d, want exact pair", len(got))
	}
	if got[0].direction != rpc.DirectionSent {
		t.Fatalf("first observation direction=%v, want sent", got[0].direction)
	}
	if !bytes.Equal(got[0].body, wantLocal) {
		t.Fatalf("selected local OPEN body=%x, want original %x", got[0].body, wantLocal)
	}
	if got[1].direction != rpc.DirectionReceived {
		t.Fatalf("second observation direction=%v, want received", got[1].direction)
	}
	if !bytes.Equal(got[1].body, wantRemote) {
		t.Fatalf("selected remote OPEN body=%x, want original %x", got[1].body, wantRemote)
	}
	if receipts[1].ID != noPoolBufID {
		t.Fatalf("selected received body claimed a read-pool handle: %d", receipts[1].ID)
	}
	if got := epoch.selectMessages(); len(got) != 0 {
		t.Fatal("selection replayed the observed OPEN pair twice")
	}
}
