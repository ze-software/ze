// Design: docs/architecture/bgp/protocol-event-capture.md -- selected incoming capture ownership.
// Related: rfc4271_incoming_listener_test.go -- actual listener and independent Session helpers.
package reactor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/capture"
)

// TestIncomingCaptureSurvivesCandidatePromotion records a real incoming
// candidate beside an existing capture. Both handoff receipts must preserve the
// candidate's original OPEN, KEEPALIVE, config history and later UPDATE, while
// retiring the old capture before rotating its complete file aside.
func TestIncomingCaptureSurvivesCandidatePromotion(t *testing.T) {
	for _, incumbentState := range []fsm.State{fsm.StateOpenSent, fsm.StateOpenConfirm} {
		t.Run(incumbentState.String(), func(t *testing.T) {
			live := incomingCapturePeer(t, incumbentState)
			incoming := incomingListenerDial(t, live.reactor)
			delayOpenReadType(t, incoming, 1)
			var candidate *incomingCandidate
			delayOpenAwait(t, func() bool {
				live.peer.mu.RLock()
				defer live.peer.mu.RUnlock()
				var ok bool
				candidate, ok = live.peer.pending.(*incomingCandidate)
				return ok && candidate.session.State() == fsm.StateOpenSent
			})
			reader := candidate.session.bufReader
			// This operation occurs while the candidate has no published epoch.
			// Its later replay still needs the submitted configuration operation.
			live.reactor.CaptureConfigEvent(capture.OpCommit, "during-candidate", []byte(`{"hold-time":90}`))
			remoteOpen := message.PackTo(delayOpenRemote(0x03030303), nil)
			delayOpenWrite(t, incoming, delayOpenRemote(0x03030303))
			delayOpenReadType(t, incoming, 4)
			openObservedBefore := time.Now()
			if incumbentState == fsm.StateOpenSent {
				if live.peer.currentSession() != live.original {
					t.Fatal("an incoming OPEN selected the candidate before its private handshake completed")
				}
			}
			delayOpenWrite(t, incoming, &message.Keepalive{})
			delayOpenAwait(t, func() bool {
				return live.peer.currentSession() == candidate.session && candidate.session.State() == fsm.StateEstablished
			})
			if candidate.session.bufReader != reader || candidate.session.Conn() != candidate.conn {
				t.Fatal("promotion replaced the candidate's Session, reader or socket")
			}
			update := incomingCaptureWithdrawal(t, live.peer, incoming)
			incomingCaptureStop(t, live.reactor)
			header, events := readEvents(t, filepath.Join(live.directory, captureFileName(live.peer.settings.Address)))
			want := [][]byte{remoteOpen, message.PackTo(&message.Keepalive{}, nil), update}
			incomingCaptureMessages(t, header, events, want)
			incomingCaptureConfig(t, events, "during-candidate")
			for _, event := range events {
				if event.Type == capture.EventMessage && event.MsgType == 1 {
					when, err := time.Parse(capture.TimeFormat, event.TS)
					if err != nil {
						t.Fatal(err)
					}
					if when.After(openObservedBefore) {
						t.Fatalf("capture stamped the retained OPEN at promotion: input=%s observed-before=%s", event.TS, openObservedBefore)
					}
				}
				if event.Type == capture.EventConfig && event.TxID == "before-candidate" {
					t.Fatal("the candidate capture inherited a config event from before its own lifetime")
				}
			}
			previousHeader, previous := readEvents(t, filepath.Join(live.directory, captureFileName(live.peer.settings.Address))+rotatedSuffix)
			incomingCaptureConfig(t, previous, "before-candidate")
			var oldMessages [][]byte
			if live.oldOpen != nil {
				oldMessages = append(oldMessages, live.oldOpen)
			}
			incomingCaptureMessages(t, previousHeader, previous, oldMessages)
			incomingCaptureFiles(t, live.directory, 2)
		})
	}
}

// TestIncomingCaptureLoserPreservesIncumbent proves that closing an unpublished
// candidate never opens or rotates its pending capture over the useful
// incumbent's live file. The same original Session then carries new traffic.
func TestIncomingCaptureLoserPreservesIncumbent(t *testing.T) {
	live := incomingCapturePeer(t, fsm.StateOpenConfirm)
	incoming := incomingListenerDial(t, live.reactor)
	delayOpenReadType(t, incoming, 1)
	delayOpenWrite(t, incoming, delayOpenRemote(1))
	refusal := delayOpenReadType(t, incoming, 3)
	if !bytes.Equal(refusal[message.HeaderLen:], []byte{6, 7}) {
		t.Fatalf("candidate was not refused by the real collision path: %x", refusal)
	}
	if err := incoming.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var octet [1]byte
	if count, err := incoming.Read(octet[:]); count != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("refused candidate was not closed: bytes=%d error=%v", count, err)
	}
	delayOpenAwait(t, func() bool {
		live.peer.mu.RLock()
		defer live.peer.mu.RUnlock()
		return live.peer.pending == nil
	})
	if live.peer.currentSession() != live.original || live.original.State() != fsm.StateOpenConfirm {
		t.Fatal("refusing the candidate replaced the useful incumbent")
	}
	delayOpenWrite(t, live.outgoing, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return live.original.State() == fsm.StateEstablished })
	update := incomingCaptureWithdrawal(t, live.peer, live.outgoing)
	incomingCaptureStop(t, live.reactor)
	header, events := readEvents(t, filepath.Join(live.directory, captureFileName(live.peer.settings.Address)))
	incomingCaptureMessages(t, header, events, [][]byte{live.oldOpen, message.PackTo(&message.Keepalive{}, nil), update})
	incomingCaptureFiles(t, live.directory, 1)
}

// incomingCaptureFixture retains the one live owner and its original history.
// Only incomingCapturePeer constructs it after the requested state is reached.
type incomingCaptureFixture struct {
	reactor   *Reactor
	peer      *Peer
	original  *Session
	outgoing  net.Conn
	directory string
	oldOpen   []byte
}

func incomingCapturePeer(t *testing.T, state fsm.State) incomingCaptureFixture {
	t.Helper()
	dir := t.TempDir()
	r, peer, original, outgoing, _ := incomingListenerPeer(t, true, 0, func(peer *Peer) {
		peer.settings.Capture = testCaptureSettings(dir)
	})
	delayOpenReadType(t, outgoing, 1)
	var oldOpen []byte
	if state == fsm.StateOpenConfirm {
		oldOpen = message.PackTo(delayOpenRemote(0x02020202), nil)
		delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
		delayOpenReadType(t, outgoing, 4)
	}
	delayOpenAwait(t, func() bool { return original.State() == state })
	r.CaptureConfigEvent(capture.OpCommit, "before-candidate", []byte(`{"hold-time":90}`))
	return incomingCaptureFixture{
		reactor: r, peer: peer, original: original, outgoing: outgoing,
		directory: dir, oldOpen: oldOpen,
	}
}

func incomingCaptureWithdrawal(t *testing.T, peer *Peer, remote net.Conn) []byte {
	t.Helper()
	before := peer.Stats().UpdatesReceived
	wire := buildUpdateMsg([]byte{0, 4, 24, 198, 51, 100, 0, 0})
	if _, err := remote.Write(wire); err != nil {
		t.Fatal(err)
	}
	delayOpenAwait(t, func() bool { return peer.Stats().UpdatesReceived == before+1 })
	return wire
}

func incomingCaptureStop(t *testing.T, r *Reactor) {
	t.Helper()
	r.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Wait(ctx); err != nil {
		t.Fatalf("captured reactor did not join its session and capture workers: %v", err)
	}
	if r.CapturesOpen() {
		t.Fatal("a capture stayed registered after the reactor stopped")
	}
}

func incomingCaptureMessages(t *testing.T, header capture.Header, events []*capture.Event, want [][]byte) {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("capture contains no lifecycle or input events")
	}
	if header.LocalAS != 65000 || header.PeerAS != 65001 || header.RouterID != 0x01010101 {
		t.Fatalf("capture replay identity changed: %+v", header)
	}
	started, err := time.Parse(capture.TimeFormat, header.Started)
	if err != nil {
		t.Fatal(err)
	}
	var messages [][]byte
	for _, event := range events {
		when, err := time.Parse(capture.TimeFormat, event.TS)
		if err != nil {
			t.Fatal(err)
		}
		if when.Before(started) {
			t.Fatalf("captured input predates its replay header: event=%s started=%s", event.TS, header.Started)
		}
		if event.Type == capture.EventMessage {
			if event.Direction != capture.DirectionReceived {
				t.Fatalf("candidate input changed direction: %+v", event)
			}
			messages = append(messages, event.Data)
		}
	}
	if !slices.EqualFunc(messages, want, bytes.Equal) {
		t.Fatalf("selected capture lost or changed original inbound frames: messages=%x, want %x", messages, want)
	}
	last := events[len(events)-1]
	if last.Type != capture.EventSession || last.Event != capture.SessionCaptureStop {
		t.Fatalf("selected capture was not drained and terminated: %+v", last)
	}
}

func incomingCaptureConfig(t *testing.T, events []*capture.Event, transaction string) {
	t.Helper()
	for _, event := range events {
		if event.Type == capture.EventConfig && event.TxID == transaction {
			return
		}
	}
	t.Fatalf("capture omitted config transaction %s", transaction)
}

func incomingCaptureFiles(t *testing.T, dir string, count int) {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != count {
		t.Fatalf("capture retained %d files, want %d", len(files), count)
	}
	var total int64
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > oneMiB {
			t.Fatalf("capture %s exceeds its configured byte cap", file.Name())
		}
		total += info.Size()
	}
	if total > 2*oneMiB {
		t.Fatalf("peer capture disk budget changed: bytes=%d, maximum=%d", total, 2*oneMiB)
	}
}
