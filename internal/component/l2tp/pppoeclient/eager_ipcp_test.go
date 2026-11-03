// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- eager NCP phase boundary.
// Related: session.go -- actual descriptor readers; phases.go -- phase owner.

package pppoeclient

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// eagerPhaseGate holds the phase consumer while input queues receive packets.
// The test owns release and MUST close it before joining the phase consumer.
type eagerPhaseGate struct {
	message string
	release chan struct{}
	reached chan struct{}
}

func (h *eagerPhaseGate) Enabled(context.Context, slog.Level) bool { return true }
func (h *eagerPhaseGate) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *eagerPhaseGate) WithGroup(string) slog.Handler            { return h }
func (h *eagerPhaseGate) Handle(_ context.Context, record slog.Record) error { //nolint:gocritic // slog.Handler requires the Record parameter by value.
	if record.Message == h.message {
		close(h.reached)
		<-h.release
	}
	return nil
}

// TestClientEagerIPCPReaders queues authenticated success before the peer's first
// IPCP request through the production pull worker. The consumer is paused
// before receiving success, as it can be by scheduling. No retransmission is sent.
// The no-auth case pauses after LCP opens, before IPCP starts. Linux descriptor
// ordering is separately exercised by TestOrderedDescriptorDispatch and the
// unchanged TestDialAcknowledgesFirstEagerIPCP real-PPP test.
func TestClientEagerIPCPReaders(t *testing.T) {
	for _, auth := range []uint16{ppp.ProtoCHAP, ppp.ProtoPAP, 0} {
		name := "CHAP"
		if auth == ppp.ProtoPAP {
			name = "PAP"
		}
		if auth == 0 {
			name = "no-auth"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				control := make(chan readFrame, 4)
				unit := make(chan readFrame, 4)
				stop := make(chan struct{})
				reader := testFrameReader(t, control, unit)
				w := newConnectedExchange()
				gate := &eagerPhaseGate{
					message: "pppoe-client: CHAP response sent",
					release: make(chan struct{}), reached: make(chan struct{}),
				}
				if auth == ppp.ProtoPAP {
					gate.message = "pppoe-client: PAP auth-request sent"
				}
				if auth == 0 {
					gate.message = "pppoe-client: LCP opened"
				}
				result := make(chan error, 1)
				defer func() {
					select {
					case <-gate.release:
					default:
						close(gate.release)
					}
					close(stop)
					<-result
				}()
				go func() {
					var ipcpID, lcpID, papID, rejectID uint8
					// RFC 1661 Sections 3.4-3.6.
					_, err := negotiatePhases(w, reader,
						sessionConfig{mtu: 1492, username: "subscriber", password: "secret"},
						0, nil, &ipcpID, stop, slog.New(gate), &lcpID, &papID, &rejectID)
					result <- err
				}()
				request := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
				control <- readFrame{data: exchangeFrame(ppp.ProtoLCP, ppp.LCPConfigureRequest, 91, exchangePeerOptions(auth))}
				w.take(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
				control <- readFrame{data: exchangeFrame(ppp.ProtoLCP, ppp.LCPConfigureAck, request[3], request[6:])}
				if auth == ppp.ProtoCHAP {
					control <- readFrame{data: exchangeFrame(auth, 1, 63, []byte{4, 9, 8, 7, 6, 'a', 'c'})}
					w.take(t, auth, 2)
					<-gate.reached
					control <- readFrame{data: exchangeFrame(auth, 3, 63, nil)}
				}
				if auth == ppp.ProtoPAP {
					pap := w.take(t, auth, 1)
					<-gate.reached
					control <- readFrame{data: exchangeFrame(auth, 2, pap[3], []byte{0})}
				}
				if auth == 0 {
					<-gate.reached
				}
				synctest.Wait()
				if auth != 0 {
					if len(control) != 1 {
						t.Fatal("authentication success was not queued before eager IPCP")
					}
				}
				peer := []byte{3, 6, 198, 51, 100, 1}
				unit <- readFrame{data: exchangeFrame(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 94, peer)}
				synctest.Wait()
				if len(unit) != 1 {
					t.Fatal("eager IPCP was consumed before dispatch authorization")
				}
				if len(reader.requests) != 0 {
					t.Fatal("reader authorized read-ahead while phase dispatch was paused")
				}
				close(gate.release)
				w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
				synctest.Wait()
				want := exchangeFrame(ppp.ProtoIPCP, ppp.LCPConfigureAck, 94, peer)
				acknowledged := false
				for len(w.writes) > 0 {
					if bytes.Equal(<-w.writes, want) {
						acknowledged = true
					}
				}
				if !acknowledged {
					t.Error("first eager peer IPCP request was not acknowledged; no retransmission was sent")
				}
			})
		})
	}
}
