package reactor

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// admissionSession completes a real wire handshake before exposing the session.
// The fixture MUST stop admission, close I/O, and join every worker at cleanup.
type admissionSession struct {
	s       *Session
	client  net.Conn
	clock   *sim.FakeClock
	workers sync.WaitGroup
	wire    chan msgtype.MessageType
}

func newAdmissionSession(t *testing.T) *admissionSession {
	t.Helper()
	return newAdmissionSessionWithConn(t, nil)
}

func newAdmissionSessionWithConn(t *testing.T, wrap func(net.Conn) net.Conn) *admissionSession {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	s := NewSession(settings)
	fc := sim.NewFakeClock(time.Now())
	s.SetClock(fc)
	require.NoError(t, s.Start())
	server, client := net.Pipe()
	if wrap != nil {
		server = wrap(server)
	}
	acceptWithReader(t, s, server, client)
	f := &admissionSession{s: s, client: client, clock: fc, wire: make(chan msgtype.MessageType, 64)}
	t.Cleanup(func() {
		s.stopAdmission()
		s.timers.StopAll()
		s.stopSendHoldTimer()
		_ = client.Close()
		_ = server.Close()
		f.workers.Wait()
		s.resetCoalesce()
	})
	f.workers.Go(func() {
		var header [message.HeaderLen]byte
		for {
			if _, err := io.ReadFull(client, header[:]); err != nil {
				return
			}
			length := int(binary.BigEndian.Uint16(header[16:18]))
			if _, err := io.CopyN(io.Discard, client, int64(length-message.HeaderLen)); err != nil {
				return
			}
			f.wire <- msgtype.MessageType(header[18])
		}
	})
	written := f.write(rfc9687PeerOpen(90))
	require.NoError(t, s.ReadAndProcess())
	require.NoError(t, <-written)
	written = f.write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, s.ReadAndProcess())
	require.NoError(t, <-written)
	require.Equal(t, fsm.StateEstablished, s.State())
	return f
}

func (f *admissionSession) write(bytes []byte) <-chan error {
	result := make(chan error, 1)
	f.workers.Go(func() {
		_, err := f.client.Write(bytes)
		result <- err
	})
	return result
}

func (f *admissionSession) read(coalesced bool) <-chan error {
	result := make(chan error, 1)
	f.workers.Go(func() {
		if coalesced {
			result <- f.s.readAndProcessCoalesced(f.s.Conn(), f.s.bufReader)
		} else {
			result <- f.s.ReadAndProcess()
		}
	})
	return result
}

func admissionContext(t *testing.T, f *admissionSession) context.Context {
	t.Helper()
	ctx, cancel := context.WithDeadline(t.Context(), f.clock.Now().Add(time.Minute))
	t.Cleanup(cancel)
	return ctx
}

func admissionUpdate(last byte) []byte {
	// ASN4 AS_SEQUENCE containing the actual remote AS, not an empty EBGP path.
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea,
		0x40, 3, 4, 192, 0, 2, 1}
	return buildUpdateMsg(buildUpdateBody(attrs, []byte{24, 10, 1, last}))
}

func admissionCapture(s *Session, bodies chan<- []byte, before func()) {
	s.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, body []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if typ == msgtype.TypeUPDATE {
			if before != nil {
				before()
			}
			bodies <- append([]byte(nil), body...)
		}
		return false
	}
}

// VALIDATES: A receipt excludes semantic admission, not an idle/partial socket read.
// PREVENTS: A read-deadline wake losing partial headers/bodies or closing the session.
func TestAdmissionPreservesPartialRead(t *testing.T) {
	for _, split := range []int{7, message.HeaderLen + 5} {
		t.Run(strconv.Itoa(split), func(t *testing.T) {
			f := newAdmissionSession(t)
			bodies := make(chan []byte, 2)
			admissionCapture(f.s, bodies, nil)
			packet := admissionUpdate(1)
			result := f.read(false)
			require.NoError(t, <-f.write(packet[:split]))
			receipt, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
			require.NoError(t, err, "a partial/idle read must not delay a receipt")
			t.Cleanup(receipt.Release)
			require.NoError(t, <-f.write(packet[split:]))
			require.Eventually(t, func() bool {
				f.s.pauseMu.Lock()
				defer f.s.pauseMu.Unlock()
				return f.s.resumeCh != nil
			}, time.Second, time.Millisecond, "complete packet must wait at admission")
			require.Empty(t, bodies)
			f.s.Resume()
			require.True(t, f.s.IsPaused(), "public resume cannot release the receipt")
			require.NoError(t, receipt.Check())
			receipt.Release()
			require.NoError(t, <-result)
			require.Equal(t, packet[message.HeaderLen:], <-bodies)
			require.Empty(t, bodies)
			require.Equal(t, fsm.StateEstablished, f.s.State())
		})
	}
}

// VALIDATES: Already-coalesced and in-flight messages retain exact, single delivery.
func TestAdmissionPreservesCoalescedBatch(t *testing.T) {
	f := newAdmissionSession(t)
	bodies := make(chan []byte, 3)
	admissionCapture(f.s, bodies, nil)
	one, two := admissionUpdate(1), admissionUpdate(2)
	written := f.write(append(append([]byte(nil), one...), two...))
	require.NoError(t, <-f.read(true))
	require.NoError(t, <-written)
	require.NotNil(t, f.s.coalesce.body)
	require.Empty(t, bodies)
	receipt, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
	require.NoError(t, err)
	t.Cleanup(receipt.Release)
	result := f.read(true)
	require.Eventually(t, func() bool {
		f.s.pauseMu.Lock()
		defer f.s.pauseMu.Unlock()
		return f.s.resumeCh != nil
	}, time.Second, time.Millisecond)
	require.Empty(t, bodies)
	receipt.Release()
	require.NoError(t, <-result)
	expected := append(append([]byte(nil), one[message.HeaderLen:]...), []byte{24, 10, 1, 2}...)
	require.Equal(t, expected, <-bodies)
	require.Empty(t, bodies)
	require.Nil(t, f.s.coalesce.body)
	require.Equal(t, fsm.StateEstablished, f.s.State())
}

// VALIDATES: Active admission finishes before acknowledgement; canceled acquisition
// releases only its own claim, leaving public backpressure and successor sessions alone.
func TestAdmissionWaitCancellationAndOwners(t *testing.T) {
	f := newAdmissionSession(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	bodies := make(chan []byte, 2)
	admissionCapture(f.s, bodies, func() {
		close(entered)
		<-release
	})
	readResult := f.read(false)
	require.NoError(t, <-f.write(admissionUpdate(1)))
	<-entered
	ctx, cancel := context.WithCancel(admissionContext(t, f))
	defer cancel()
	result := make(chan error, 1)
	f.workers.Go(func() {
		receipt, err := f.s.PauseAdmission(ctx, time.Second)
		if receipt != nil {
			receipt.Release()
		}
		result <- err
	})
	require.Eventually(t, f.s.IsPaused, time.Second, time.Millisecond)
	select {
	case err := <-result:
		t.Fatalf("acknowledged active callback: %v", err)
	default:
	}
	f.s.Pause()
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
	require.True(t, f.s.IsPaused())
	unblock()
	require.NoError(t, <-readResult)
	require.Len(t, bodies, 1)
	first, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
	require.NoError(t, err)
	t.Cleanup(first.Release)
	second, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
	require.NoError(t, err)
	t.Cleanup(second.Release)
	f.s.Resume()
	first.Release()
	first.Release()
	require.True(t, f.s.IsPaused(), "second owned claim survives")
	replacement := newFlowSession()
	replacement.Pause()
	peer := &Peer{session: f.s}
	peer.mu.Lock()
	peer.session = replacement
	peer.mu.Unlock()
	second.Release()
	require.False(t, f.s.IsPaused())
	require.True(t, peer.session.IsPaused(), "captured receipt cannot touch replacement")
	require.Error(t, second.Check())
}

// VALIDATES: A live budget bounds an active-admission wait, with abort time reserved.
func TestAdmissionWaitUsesResidualBudget(t *testing.T) {
	f := newAdmissionSession(t)
	f.s.timers.SetHoldTime(10 * time.Second)
	f.s.timers.StartHoldTimer()
	f.clock.Add(7 * time.Second)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	bodies := make(chan []byte, 2)
	admissionCapture(f.s, bodies, func() {
		close(entered)
		<-release
	})
	readResult := f.read(false)
	require.NoError(t, <-f.write(admissionUpdate(1)))
	<-entered
	result := make(chan error, 1)
	pauseCtx := admissionContext(t, f)
	f.workers.Go(func() {
		receipt, err := f.s.PauseAdmission(pauseCtx, time.Second)
		if receipt != nil {
			receipt.Release()
		}
		result <- err
	})
	// HoldTimer, KeepaliveTimer, SendHoldTimer, and the admission deadline.
	require.True(t, f.clock.AwaitTimers(4, time.Second))
	f.clock.Add(2 * time.Second)
	require.ErrorIs(t, <-result, context.DeadlineExceeded)
	require.False(t, f.s.IsPaused())
	require.Equal(t, time.Second, f.s.HoldBudget().Remaining)
	_, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
	require.Error(t, err, "no room remains for abort plus work")
	require.False(t, f.s.IsPaused(), "preflight refusal must not mutate pause state")
	unblock()
	require.NoError(t, <-readResult)
}

// VALIDATES: Successful receipt expiration does not auto-resume a tentative policy;
// outgoing keepalives continue and the original HoldTimer still ends the real Run.
func TestAdmissionDoesNotExtendHoldTimer(t *testing.T) {
	f := newAdmissionSession(t)
	f.s.timers.SetHoldTime(5 * time.Second)
	f.s.timers.StartHoldTimer()
	f.s.timers.SetKeepaliveTime(time.Second)
	f.s.timers.StartKeepaliveTimer()
	f.clock.Add(2 * time.Second)
	receipt, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
	require.NoError(t, err)
	t.Cleanup(receipt.Release)
	require.Equal(t, f.clock.Now().Add(2*time.Second), receipt.Deadline())
	require.Equal(t, f.clock.Now().Add(3*time.Second), receipt.CompletionDeadline())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runResult := make(chan error, 1)
	f.workers.Go(func() { runResult <- f.s.Run(ctx) })
	require.Eventually(t, func() bool {
		f.s.pauseMu.Lock()
		defer f.s.pauseMu.Unlock()
		return f.s.resumeCh != nil
	}, time.Second, time.Millisecond, "Run must reach its admission gate before advancing time")
	f.clock.Add(2 * time.Second)
	require.ErrorIs(t, receipt.Check(), context.DeadlineExceeded)
	require.NoError(t, receipt.CheckCompletion(), "reserved time remains available for compensation")
	require.True(t, f.s.IsPaused(), "coordinator must explicitly release after compensation")
	require.Equal(t, time.Second, f.s.HoldBudget().Remaining)
	f.clock.Add(time.Second)
	require.Error(t, <-runResult)
	require.Equal(t, fsm.StateIdle, f.s.State())
	require.Error(t, receipt.CheckCompletion())
	require.Nil(t, f.s.Conn())
	f.workers.Wait() // Join the wire observer before inspecting its final packet set.
	var keepalives, notifications int
	for len(f.wire) != 0 {
		switch <-f.wire {
		case msgtype.TypeKEEPALIVE:
			keepalives++
		case msgtype.TypeNOTIFICATION:
			notifications++
		case msgtype.TypeOPEN, msgtype.TypeUPDATE, msgtype.TypeROUTEREFRESH:
			t.Fatal("unexpected wire message")
		default:
			t.Fatal("unexpected wire message")
		}
	}
	require.Greater(t, keepalives, 1, "keepalive writes continue while admission is held")
	require.Equal(t, 1, notifications)
}

// VALIDATES: Disabled is not unarmed; zero HoldTime still requires a deadline.
func TestAdmissionDisabledHoldUsesTransactionDeadline(t *testing.T) {
	f := newAdmissionSession(t)
	f.s.timers.StopHoldTimer()
	_, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
	require.Error(t, err)
	f.s.timers.SetHoldTime(0)
	_, err = f.s.PauseAdmission(context.Background(), time.Second)
	require.Error(t, err)
	ctx := admissionContext(t, f)
	receipt, err := f.s.PauseAdmission(ctx, time.Second)
	require.NoError(t, err)
	t.Cleanup(receipt.Release)
	deadline, _ := ctx.Deadline()
	require.Equal(t, deadline.Add(-time.Second), receipt.Deadline())
	require.Equal(t, deadline, receipt.CompletionDeadline())
}

// VALIDATES: Run acknowledges only after an active semantic callback leaves,
// then preserves the next packet until this receipt is explicitly released.
func TestAdmissionRunAcknowledgesCompletedInput(t *testing.T) {
	f := newAdmissionSession(t)
	f.s.coalesceEnabled = false
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	bodies := make(chan []byte, 3)
	admissionCapture(f.s, bodies, func() {
		select {
		case <-entered:
		default:
			close(entered)
			<-release
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runResult := make(chan error, 1)
	f.workers.Go(func() { runResult <- f.s.Run(ctx) })
	one, two := admissionUpdate(1), admissionUpdate(2)
	require.NoError(t, <-f.write(one))
	<-entered
	pauseCtx := admissionContext(t, f)
	receipts := make(chan *AdmissionReceipt, 1)
	errors := make(chan error, 1)
	f.workers.Go(func() {
		receipt, err := f.s.PauseAdmission(pauseCtx, time.Second)
		receipts <- receipt
		errors <- err
	})
	require.Eventually(t, f.s.IsPaused, time.Second, time.Millisecond)
	require.Empty(t, receipts, "active semantic work must precede acknowledgement")
	unblock()
	receipt := <-receipts
	require.NoError(t, <-errors)
	t.Cleanup(receipt.Release)
	require.Equal(t, one[message.HeaderLen:], <-bodies)
	written := f.write(two)
	require.Empty(t, bodies)
	require.NoError(t, receipt.Check())
	receipt.Release()
	require.NoError(t, <-written)
	require.Equal(t, two[message.HeaderLen:], <-bodies)
	cancel()
	require.ErrorIs(t, <-runResult, context.Canceled)
	require.Empty(t, bodies)
}

// VALIDATES: Terminal completion wakes a standalone semantic reader without a
// Run watcher. The caller immediately closes on return, so an early wake would
// destroy the required notification before its producer can send it.
func TestAdmissionStandaloneLossWakesReader(t *testing.T) {
	cases := []struct {
		name string
		end  func(*admissionSession) error
		want error
	}{
		{"hold", func(f *admissionSession) error {
			f.clock.Add(5 * time.Second)
			return nil
		}, ErrHoldTimerExpired},
		{"administrative", func(f *admissionSession) error {
			return f.s.Teardown(message.NotifyCeaseAdminShutdown, "")
		}, ErrTeardown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdmissionSession(t)
			f.s.timers.SetHoldTime(5 * time.Second)
			f.s.timers.StartHoldTimer()
			bodies := make(chan []byte, 2)
			admissionCapture(f.s, bodies, nil)
			receipt, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
			require.NoError(t, err)
			t.Cleanup(receipt.Release)
			result := make(chan error, 1)
			f.workers.Go(func() {
				err := f.s.ReadAndProcess()
				f.s.closeConn()
				result <- err
			})
			require.NoError(t, <-f.write(admissionUpdate(1)))
			require.Eventually(t, func() bool {
				f.s.pauseMu.Lock()
				defer f.s.pauseMu.Unlock()
				return f.s.resumeCh != nil
			}, time.Second, time.Millisecond)
			require.NoError(t, tc.end(f))
			select {
			case err := <-result:
				require.ErrorIs(t, err, tc.want)
			case <-time.After(time.Second):
				t.Fatal("standalone admission reader did not wake on terminal completion")
			}
			f.workers.Wait()
			require.Len(t, f.wire, 2, "handshake KEEPALIVE and terminal NOTIFICATION must both reach the peer")
			require.Equal(t, msgtype.TypeKEEPALIVE, <-f.wire)
			require.Equal(t, msgtype.TypeNOTIFICATION, <-f.wire)
			require.Empty(t, bodies)
			require.Equal(t, fsm.StateIdle, f.s.State())
			require.Error(t, receipt.CheckCompletion())
		})
	}
}

type admissionNotificationConn struct {
	net.Conn
	entered chan struct{}
	release chan struct{}
}

func (c *admissionNotificationConn) Write(b []byte) (int, error) {
	if len(b) >= message.HeaderLen && b[18] == byte(msgtype.TypeNOTIFICATION) {
		close(c.entered)
		<-c.release
	}
	return c.Conn.Write(b)
}

// VALIDATES: Releasing every pause claim during a real terminal NOTIFICATION
// cannot turn early cause publication into a read-loop exit or new admission.
func TestAdmissionReleaseWaitsForTerminalNotification(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		t.Run(strconv.FormatBool(standalone), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			f := newAdmissionSessionWithConn(t, func(conn net.Conn) net.Conn {
				return &admissionNotificationConn{Conn: conn, entered: entered, release: release}
			})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			f.s.timers.SetHoldTime(5 * time.Second)
			f.s.timers.StartHoldTimer()
			bodies := make(chan []byte, 2)
			admissionCapture(f.s, bodies, nil)
			receipt, err := f.s.PauseAdmission(admissionContext(t, f), time.Second)
			require.NoError(t, err)
			t.Cleanup(receipt.Release)
			f.s.Pause()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			f.workers.Go(func() {
				if standalone {
					result <- f.s.ReadAndProcess()
				} else {
					// The exact gate Run calls; observing its return directly
					// avoids closeConn's writeMu masking an early return.
					result <- f.s.waitForResume(ctx)
				}
			})
			if standalone {
				require.NoError(t, <-f.write(admissionUpdate(1)))
			}
			var waiting chan struct{}
			require.Eventually(t, func() bool {
				f.s.pauseMu.Lock()
				defer f.s.pauseMu.Unlock()
				waiting = f.s.resumeCh
				return waiting != nil
			}, time.Second, time.Millisecond)
			advanced := make(chan struct{})
			f.workers.Go(func() {
				f.clock.Add(5 * time.Second)
				close(advanced)
			})
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("HoldTimer did not reach its terminal NOTIFICATION write")
			}
			require.NotNil(t, f.s.closeReason.Load())
			receipt.Release()
			f.s.Resume()
			require.Eventually(t, func() bool {
				f.s.pauseMu.Lock()
				defer f.s.pauseMu.Unlock()
				return len(result) != 0 || (f.s.resumeCh != nil && f.s.resumeCh != waiting)
			}, time.Second, time.Millisecond, "reader must observe release and repark for terminal completion")
			require.Empty(t, result, "cause publication cannot terminate the reader before notification completes")
			require.Empty(t, bodies)
			unblock()
			<-advanced
			require.ErrorIs(t, <-result, ErrHoldTimerExpired)
			require.Empty(t, bodies)
		})
	}
}
