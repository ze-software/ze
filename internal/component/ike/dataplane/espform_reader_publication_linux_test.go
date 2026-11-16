//go:build linux

// Design: docs/guide/ipsec.md -- Both ESP wire forms
// Related: espform_open_linux.go -- failure publication before reader retirement

package dataplane

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"
)

// espFormPublicationSocket lets Forget contend with read as the failed socket returns.
// The signal belongs to the scripted socket, before the actual run and read producers
// publish the failure; it neither publishes dead nor copies their stop decision.
type espFormPublicationSocket struct {
	*espFormScriptedSocket
	failed chan struct{}
}

func (s *espFormPublicationSocket) ReadMsgIP(p, oob []byte) (n, oobn, flags int, addr *net.IPAddr, err error) {
	n, oobn, flags, addr, err = s.espFormScriptedSocket.ReadMsgIP(p, oob)
	close(s.failed)
	return n, oobn, flags, addr, err
}

// TestESPFormReceiverConcurrentFailureAndRetirementReportsEveryRecordedFailure proves
// a last Forget cannot leave a published read failure with no event from its worker.
// METHOD: the actual socket signals its read error to a competing Forget; run, read,
// retireLocked and revive decide their own order. After read and Forget have ended and
// Close has joined revive, every published dead must have exactly one matching event;
// retirement that won before admission must have none. The fixed 1024 attempts bound
// contention, not an elapsed-time success oracle. The separate ordering tests retain
// the positive already-recorded failure and intentional-retirement controls.
// MUTATION: remove the per-life publication/retirement serialization; a scheduler yield
// after read's stop check exposes retirement before dead is published, losing its event.
func TestESPFormReceiverConcurrentFailureAndRetirementReportsEveryRecordedFailure(t *testing.T) {
	const spi = 0x5E0F0002
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	for attempt := range 1024 {
		paths := newIdleESPFormPaths()
		failedSocket, ok := paths.conn.(*espFormScriptedSocket)
		if !ok {
			t.Fatalf("reader fixture socket = %T, want *espFormScriptedSocket", paths.conn)
		}
		socket := &espFormPublicationSocket{
			espFormScriptedSocket: failedSocket,
			failed:                make(chan struct{}),
		}
		paths.conn = socket
		r := espFormTestReceiver(t, func() (espFormPaths, error) {
			return espFormPaths{}, errors.New("test: a retiring reader attempted to reopen")
		})
		r.reopenWait = time.Hour
		events := make(chan ESPFormReaderStopped, 2)
		r.readerStopped = func(stopped ESPFormReaderStopped) { events <- stopped }
		life := espFormReaderBeforeRevive(r, paths)
		r.reg.watch(spi, natTarget(peer, local))
		close(socket.fail)
		retired := make(chan struct{})
		// This is one bounded test lifecycle. Forget reaches the production stop only
		// after the scripted socket has selected its failing read outcome.
		go func() {
			<-socket.failed
			r.Forget(spi, local)
			close(retired)
		}()
		r.revivers.Add(1)
		go r.revive(life)
		r.wg.Add(1)
		r.read(paths.conn, paths.tun, life)
		select {
		case <-retired:
		case <-time.After(espFormReopenWait):
			t.Fatalf("iteration %d: last Forget did not join the reader", attempt)
		}
		if err := r.Close(); err != nil {
			t.Fatalf("iteration %d: join the revive worker: %v", attempt, err)
		}
		select {
		case <-life.dead:
			if len(events) != 1 {
				t.Fatalf("iteration %d: recorded failure produced %d events, want exactly one", attempt, len(events))
			}
			if stopped := <-events; !errors.Is(stopped.Err, errESPFormSocketFailed) {
				t.Errorf("iteration %d: reported %v, want the socket failure", attempt, stopped.Err)
			}
		default:
			if len(events) != 0 {
				t.Fatalf("iteration %d: intentional retirement produced %d failure events", attempt, len(events))
			}
		}
	}
}
