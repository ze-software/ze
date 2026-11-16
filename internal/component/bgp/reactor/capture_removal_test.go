// Design: docs/architecture/bgp/protocol-event-capture.md -- capture admission follows Peer membership.
// Related: peer_removal.go -- all removal paths release the old capture namespace.
package reactor

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/core/clock"
)

// TestRemovedPeerFactoryCannotCaptureReplacement gates the old real factory
// while its input is being constructed, completes removal and addition, then
// lets that factory attempt registration. It must own no capture after the
// removal cut; the replacement's real TCP OPEN must remain in its own file.
func TestRemovedPeerFactoryCannotCaptureReplacement(t *testing.T) {
	r := newRemovalTestReactor()
	settings := testCapturePeer(t.TempDir())
	require.NoError(t, r.AddPeer(settings))
	old := r.peers[settings.PeerKey()]
	initial := old.newSession()
	initialInput, ok := initial.captureWriter.(*pendingSessionCapture)
	require.True(t, ok)
	t.Cleanup(func() {
		initial.stopSessionTimers()
		old.stopCapture(initialInput)
	})
	require.NoError(t, initial.prepareOpen())
	initialInput.recordConfig(r.clock.Now(), capture.OpCommit, "before-removal", []byte(`{"hold-time":90}`))
	initialInput.selectForPublication()
	gate := &captureRemovalClock{
		Clock: clock.RealClock{}, entered: make(chan struct{}), release: make(chan struct{}),
	}
	old.SetClock(gate)
	release := sync.OnceFunc(func() { close(gate.release) })
	t.Cleanup(release)
	finished := make(chan *Session, 1)
	// This one factory lifecycle is joined below, including assertion cleanup.
	go func() { finished <- old.newSession() }()
	var late *Session
	t.Cleanup(func() {
		release()
		if late == nil {
			select {
			case late = <-finished:
			case <-time.After(5 * time.Second):
				t.Error("old factory did not join during cleanup")
				return
			}
		}
		late.closeConn()
		late.stopSessionTimers()
		old.stopCapture(late.captureWriter)
	})
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("old factory did not reach its pre-registration input constructor")
	}
	require.NoError(t, r.RemovePeer(settings.Address))
	select {
	case <-initialInput.finished:
	default:
		t.Fatal("removal returned before joining the old capture worker")
	}
	require.False(t, r.CapturesOpen(), "removal releases every capture registered before its cut")
	require.NoError(t, r.AddPeer(settings))
	replacement := r.peers[settings.PeerKey()]
	require.NotSame(t, old, replacement)
	release()
	select {
	case late = <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("removed factory did not finish after release")
	}
	require.Nil(t, late.captureWriter, "an input constructed before removal cannot register after its Peer has left")
	require.False(t, r.CapturesOpen(), "no late old worker may retain the replacement's capture namespace")

	session := replacement.newSession()
	input, ok := session.captureWriter.(*pendingSessionCapture)
	require.True(t, ok, "the replacement still admits its configured capture")
	t.Cleanup(func() {
		session.closeConn()
		session.stopSessionTimers()
		replacement.stopCapture(input)
	})
	input.selectForPublication()
	require.NoError(t, session.Start())
	remote, local := collisionProofTCPPair(t)
	require.NoError(t, session.Accept(local))
	delayOpenReadType(t, remote, 1)
	open := delayOpenRemote(0x02020202)
	wire := message.PackTo(open, nil)
	delayOpenWrite(t, remote, open)
	require.NoError(t, session.ReadAndProcess())
	delayOpenReadType(t, remote, 4)
	replacement.stopCapture(input)
	_, events := readEvents(t, input.Path())
	messages := 0
	for _, event := range events {
		if event.Type != capture.EventMessage {
			continue
		}
		messages++
		require.True(t, bytes.Equal(wire, event.Data), "the replacement retains its complete original OPEN")
	}
	require.Equal(t, 1, messages)
	require.Equal(t, capture.SessionCaptureStop, events[len(events)-1].Event)
	require.False(t, r.CapturesOpen(), "joined capture workers must release their registry membership")
}

// captureRemovalClock holds exactly the first input-construction instant.
// Its owner MUST close release and join the factory goroutine before returning.
// Clock supplies all other time operations; Now is safe for concurrent use.
// Its zero value and nil pointer are invalid; the fixture supplies every field.
type captureRemovalClock struct {
	clock.Clock
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (c *captureRemovalClock) Now() time.Time {
	c.once.Do(func() {
		close(c.entered)
		<-c.release
	})
	return c.Clock.Now()
}
