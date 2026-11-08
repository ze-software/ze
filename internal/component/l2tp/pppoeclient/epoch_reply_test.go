// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- reply correlation across restarts.
// Related: connected_exchange_test.go -- independent packet oracle and transport.

package pppoeclient

import (
	"io"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestConnectedLCPStaleReplies proves delayed MRU replies do not change the next
// epoch, then completes that epoch. Both Nak and Reject can match unchanged MRU
// bytes even though the request's Magic-Number changed.
func TestConnectedLCPStaleReplies(t *testing.T) {
	for _, code := range []uint8{3, 4} {
		t.Run(map[uint8]string{3: "nak", 4: "reject"}[code], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				s, initial := w.session(t)
				done, stop := make(chan struct{}), make(chan struct{})
				defer close(stop)
				go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
				var previous []byte
				var lastID uint8
				identifier := initial.ipcpIdentifier
				for epoch := range 3 {
					options := exchangePeerOptions(0)
					w.send(ppp.ProtoLCP, 1, 91, options)
					request := w.take(t, ppp.ProtoLCP, 1)
					w.take(t, ppp.ProtoLCP, 2)
					if previous != nil {
						mru := previous[6:10]
						if code == 3 {
							mru = []byte{1, 4, 5, 120}
						}
						w.send(ppp.ProtoLCP, code, previous[3], mru)
						w.quiet(t)
						if request[3] != lastID+1 {
							t.Fatalf("restart Identifier %d, want %d", request[3], lastID+1)
						}
					}
					previous = request
					// A valid Nak advances within the epoch too. The next restart
					// must continue after this request, not just count restarts.
					w.send(ppp.ProtoLCP, 3, request[3], []byte{1, 4, 5, 120})
					request = w.take(t, ppp.ProtoLCP, 1)
					w.send(ppp.ProtoLCP, 2, request[3], request[6:])
					identifier = w.openIPCP(t, identifier, epoch)
					// The epoch passes IPv4 once the interface applied it.
					applyPending(t, s)
					lastID = request[3]
				}
			})
		})
	}
}

// TestConnectedPAPStaleAck repeats PAP with a retry and injects each old Ack
// before the next verdict. IPCP must not start until the current Ack arrives.
func TestConnectedPAPStaleAck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		magic, identifier := initial.magic, initial.ipcpIdentifier
		var previous uint8
		for epoch := range 3 {
			magic = w.openLCP(t, ppp.ProtoPAP, magic)
			request := w.take(t, ppp.ProtoPAP, 1)
			if epoch > 0 {
				w.send(ppp.ProtoPAP, 2, previous, []byte{0})
				w.quiet(t)
			}
			if epoch == 1 {
				// sleep(timer): advance the real PAP retry ticker in virtual time.
				time.Sleep(papRetryInterval)
				retry := w.take(t, ppp.ProtoPAP, 1)
				if retry[3] != request[3]+1 {
					t.Fatalf("retry Identifier %d, want %d", retry[3], request[3]+1)
				}
				request = retry
			}
			w.send(ppp.ProtoPAP, 2, request[3], []byte{0})
			identifier = w.openIPCP(t, identifier, epoch)
			// The epoch passes IPv4 once the interface applied it.
			applyPending(t, s)
			previous = request[3]
		}
	})
}
