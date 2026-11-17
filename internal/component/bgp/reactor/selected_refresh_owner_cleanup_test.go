// Design: docs/architecture/api/process-protocol.md -- pending refresh owner retirement.
// Related: selected_refresh_socket_test.go -- active-RPC cancellation is distinct.
package reactor

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// selectedRefreshCleanupWire owns one real peer pipe and one bounded frame
// reader. Its cleanup releases Close, closes both ends and joins the reader.
// The close gate lets a test distinguish starting retirement from joining it.
type selectedRefreshCleanupWire struct {
	net.Conn
	closing     chan struct{}
	duplicate   chan struct{}
	closeCalls  atomic.Uint32
	writeCut    atomic.Pointer[selectedRefreshCleanupWriteCut]
	closeGate   chan struct{}
	releaseOnce sync.Once
	frames      chan selectedRefreshCleanupFrame
	done        chan struct{}
}

type selectedRefreshCleanupFrame struct {
	data []byte
	err  error
}

// A controlled final UPDATE failure lets the actual local writer own direct
// retirement Close under writeMu. Earlier markers still cross the real pipe.
type selectedRefreshCleanupWriteCut struct {
	entered chan struct{}
	fail    <-chan struct{}
}

func selectedRefreshCleanupPeerWire(t *testing.T, session *Session, holdClose bool) *selectedRefreshCleanupWire {
	t.Helper()
	left, right := net.Pipe()
	wire := &selectedRefreshCleanupWire{Conn: left, closing: make(chan struct{}), duplicate: make(chan struct{}),
		closeGate: make(chan struct{}), frames: make(chan selectedRefreshCleanupFrame, 8), done: make(chan struct{})}
	if !holdClose {
		wire.release()
	}
	// This lifecycle reader sees at most Begin, End and KEEPALIVE before EOF;
	// the eight-slot channel accommodates all frames even during a failed test.
	go func() {
		defer close(wire.done)
		for {
			header := make([]byte, 19)
			if _, err := io.ReadFull(right, header); err != nil {
				wire.frames <- selectedRefreshCleanupFrame{err: err}
				return
			}
			length := int(binary.BigEndian.Uint16(header[16:18]))
			if length < len(header) {
				wire.frames <- selectedRefreshCleanupFrame{err: io.ErrUnexpectedEOF}
				return
			}
			frame := make([]byte, length)
			copy(frame, header)
			if _, err := io.ReadFull(right, frame[len(header):]); err != nil {
				wire.frames <- selectedRefreshCleanupFrame{err: err}
				return
			}
			wire.frames <- selectedRefreshCleanupFrame{data: frame}
		}
	}()
	t.Cleanup(func() {
		wire.release()
		require.NoError(t, wire.Close())
		require.NoError(t, right.Close())
		join, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		select {
		case <-wire.done:
		case <-join.Done():
			t.Error("peer wire reader did not join")
		}
	})
	session.conn = wire
	writer := &observedBGPWriter{session: session, writer: wire}
	writer.context.Store(uint32(session.sendCtxID))
	session.wireWriter, session.bufWriter = writer, bufio.NewWriter(writer)
	return wire
}

func (wire *selectedRefreshCleanupWire) Close() error {
	call := wire.closeCalls.Add(1)
	if call != 1 {
		if call == 2 {
			close(wire.duplicate)
		}
		// Concurrent net.Conn.Close need not join the first Close's work.
		// Returning immediately makes a duplicate-Close pseudo-join fail.
		return nil
	}
	close(wire.closing)
	<-wire.closeGate
	return wire.Conn.Close()
}

func (wire *selectedRefreshCleanupWire) Write(data []byte) (int, error) {
	if cut := wire.writeCut.Load(); cut != nil {
		close(cut.entered)
		<-cut.fail
		return 0, context.Canceled
	}
	return wire.Conn.Write(data)
}

func (wire *selectedRefreshCleanupWire) release() {
	wire.releaseOnce.Do(func() { close(wire.closeGate) })
}

// selectedRefreshCleanupClock observes the existing close-claim wait without
// changing its duration or other Session timers. Stopped means awaitClaim has
// left its bounded wait; process cleanup must still join the held real Close.
type selectedRefreshCleanupClock struct {
	clock.Clock
	waits chan *selectedRefreshCleanupTimer
}

type selectedRefreshCleanupTimer struct {
	clock.Timer
	stopped chan struct{}
	once    sync.Once
}

func (timerClock *selectedRefreshCleanupClock) NewTimer(duration time.Duration) clock.Timer {
	timer := timerClock.Clock.NewTimer(duration)
	if duration != closeClaimWait {
		return timer
	}
	observed := &selectedRefreshCleanupTimer{Timer: timer, stopped: make(chan struct{})}
	select {
	case timerClock.waits <- observed:
	default: // A failed test's extra cleanup close must not block on the observer.
	}
	return observed
}

func (timer *selectedRefreshCleanupTimer) Stop() bool {
	timer.once.Do(func() { close(timer.stopped) })
	return timer.Timer.Stop()
}

func (wire *selectedRefreshCleanupWire) next(t *testing.T, ctx context.Context) selectedRefreshCleanupFrame {
	t.Helper()
	select {
	case frame := <-wire.frames:
		return frame
	case <-ctx.Done():
		t.Fatal("peer wire did not produce the expected frame or EOF")
		return selectedRefreshCleanupFrame{err: ctx.Err()}
	}
}

func selectedRefreshCleanupSDK(t *testing.T, fixture *selectedWriterFixture, socket bool, work context.Context) (*pluginserver.Server, *sdk.Plugin, *process.Process) {
	t.Helper()
	declaration := []rpc.DisseminationDecl{{Namespace: "bgp", Role: SelectedDisseminationRole,
		SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state", "refresh", "update-sent"},
		DestinationSend: []string{"update"}}}
	ready, runnerDone := make(chan *sdk.Plugin, 1), make(chan struct{})
	const implementation = "selected-refresh-owner-cleanup-runner"
	require.NoError(t, registry.Register(registry.Registration{Name: implementation,
		Dissemination: declaration, CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
			defer close(runnerDone)
			if socket {
				conn = selectedRefreshSocketConn{conn}
			}
			client := sdk.NewWithConn(selectedWriterProcess, conn)
			defer func() {
				_ = client.Close() //nolint:errcheck // The server intentionally closes the selecting process in this fixture.
			}()
			client.OnConfigure(func([]sdk.ConfigSection) error { return nil })
			client.OnStarted(func(context.Context) error { ready <- client; return nil })
			_ = client.Run(work, &sdk.Registration{Dissemination: declaration}) //nolint:errcheck // Stop is the exercised engine exit, joined below.
			return 0
		}}))
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{}),
		Plugins:         []plugin.PluginConfig{{Name: selectedWriterProcess, Run: implementation, Internal: true, Encoder: plugin.EncodingJSON}},
	}, fixture.api)
	require.NoError(t, err)
	host.SetProcessSpawner(&selectedRefreshSpawner{ctx: work})
	grant := plugin.PeerProcessBinding{PluginName: selectedWriterProcess, Explicit: true,
		Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent},
		Send:    map[string]bool{"update": true}}
	host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		{Addr: "192.0.2.79", Bindings: []plugin.PeerProcessBinding{{PluginName: selectedWriterProcess, Explicit: true,
			Receive: map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth}}}},
		{Addr: "192.0.2.80", Bindings: []plugin.PeerProcessBinding{grant}},
		{Addr: "192.0.2.81", Bindings: []plugin.PeerProcessBinding{grant}},
	})
	fixture.api.r.api = host
	require.NoError(t, host.StartWithContext(work))
	t.Cleanup(func() {
		host.Stop()
		join, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		require.NoError(t, host.Wait(join))
		select {
		case <-runnerDone:
		case <-join.Done():
			t.Error("refresh cleanup SDK runner did not join")
		}
	})
	require.NoError(t, host.WaitForStartupComplete(work))
	var client *sdk.Plugin
	select {
	case client = <-ready:
	case <-work.Done():
		t.Fatal("refresh cleanup SDK never became ready")
	}
	proc := host.ProcessManager().GetProcess(selectedWriterProcess)
	require.NotNil(t, proc)
	require.Equal(t, !socket, proc.Conn().HasBridge(), "exercise both public transports")
	fixture.peer.refreshForwardFacts()
	return host, client, proc
}

// A completed Begin survives the RPC itself: killing its real selecting
// process between calls MUST retire the exact peer connection, even with an
// empty route cut. A completed Begin/End and a sibling remain useful. Holding
// the actual Close proves process cleanup joins retirement rather than merely
// requesting it. Both native and framed public SDK calls take this lifecycle.
// MUTATION: skip pending-marker ownership in process cleanup; open cases fail
// when cleanup returns while the original peer still has an unfinished Begin.
func TestSelectedRefreshOwnerCleanupBetweenCalls(t *testing.T) {
	for _, transport := range []string{"native", "framed"} {
		t.Run(transport, func(t *testing.T) {
			for _, ending := range []struct {
				name      string
				completed bool
				claimed   bool
			}{
				{name: "pending-begin"},
				{name: "pending-begin-existing-close", claimed: true},
				{name: "completed-begin-end", completed: true},
			} {
				t.Run(ending.name, func(t *testing.T) {
					fixture, input := selectedRefreshFixture(t)
					work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
					defer cancel()
					host, client, proc := selectedRefreshCleanupSDK(t, fixture, transport == "framed", work)
					original := fixture.peer.session
					wire := selectedRefreshCleanupPeerWire(t, original, !ending.completed)
					sibling, _ := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
					sibling.settings.ExportPolicy = fixture.peer.settings.ExportPolicy
					sibling.settings.ProcessBindings = fixture.peer.settings.ProcessBindings
					sibling.reactor = fixture.api.r
					sibling.session.initialReplay.Store(72)
					sibling.negotiated.Load().EnhancedRouteRefresh = true
					sibling.refreshForwardFacts()
					fixture.api.r.peers[sibling.Settings().PeerKey()] = sibling
					siblingWire := selectedRefreshCleanupPeerWire(t, sibling.session, false)
					sendMarker := func(request rpc.SelectRefreshInput, target *selectedRefreshCleanupWire, label string) {
						result, err := client.SelectRefresh(work, request)
						require.NoError(t, err)
						require.Equal(t, rpc.SelectedRefreshWritten, result.Outcome)
						require.NoError(t, result.Validate(request))
						frame := target.next(t, work)
						require.NoError(t, frame.err)
						require.Equal(t, []string{label}, refreshWireLabels(t, frame.data))
					}
					sendMarker(input, wire, "borr")
					if ending.completed {
						input.Action = rpc.SelectedRefreshEnd
						sendMarker(input, wire, "eorr")
					}
					siblingInput := input
					siblingInput.Destination, siblingInput.PeerOwner, siblingInput.DestinationSession = "192.0.2.81", sibling.sourceOwner, 72
					siblingInput.Action = rpc.SelectedRefreshBegin
					sendMarker(siblingInput, siblingWire, "borr")
					siblingInput.Action = rpc.SelectedRefreshEnd
					sendMarker(siblingInput, siblingWire, "eorr")
					require.Empty(t, fixture.peer.adjOut.routes, "this is a real empty replay, not route cleanup")
					var claimWait <-chan *selectedRefreshCleanupTimer
					if ending.claimed {
						claimClock := &selectedRefreshCleanupClock{Clock: original.clock,
							waits: make(chan *selectedRefreshCleanupTimer, 1)}
						original.clock = claimClock
						// Keep the observed claim wait for the post-Stop assertion.
						claimWait = claimClock.waits
						closeDone := make(chan struct{})
						go func() { original.closeConnWithin(0); close(closeDone) }()
						t.Cleanup(func() {
							wire.release()
							join, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
							defer stop()
							select {
							case <-closeDone:
							case <-join.Done():
								t.Error("original socket close did not join")
							}
						})
						select {
						case <-wire.closing:
						case <-work.Done():
							t.Fatal("original socket close never reached its held Close")
						}
						// This second Close really returns while the first remains
						// held; its return must never satisfy process retirement.
						require.NoError(t, wire.Close())
					}
					proc.Stop()
					cleanupDone := make(chan error, 1)
					go func() { cleanupDone <- proc.WaitRuntimeCleanupContext(work) }()
					if !ending.completed {
						closeEntered := wire.closing
						if ending.claimed {
							var timer *selectedRefreshCleanupTimer
							select {
							case timer = <-claimWait:
							case err := <-cleanupDone:
								t.Fatalf("cleanup treated duplicate Close as first-close completion: %v", err)
							case <-work.Done():
								t.Fatal("cleanup never reached its original socket claim")
							}
							select {
							case <-timer.stopped:
							case <-work.Done():
								t.Fatal("bounded close-claim wait did not finish")
							}
							// WaitRuntimeCleanupContext itself is the observable
							// barrier: after normal awaitClaim expires, it must
							// still refuse to report a held first Close as joined.
							probe, stop := context.WithTimeout(work, 100*time.Millisecond)
							require.ErrorIs(t, proc.WaitRuntimeCleanupContext(probe), context.DeadlineExceeded)
							stop()
						}
						select {
						case <-closeEntered:
						case err := <-cleanupDone:
							t.Fatalf("process cleanup finished without retiring the unfinished Begin's original wire: %v", err)
						case <-work.Done():
							t.Fatal("unfinished Begin was neither retired nor joined")
						}
						select {
						case err := <-cleanupDone:
							t.Fatalf("process cleanup reported completion before actual held Close returned: %v", err)
						default:
						}
						// Even while A's retirement is held, B's real writer works.
						require.NoError(t, sibling.session.writeMessage(siblingWire, &message.Keepalive{}))
						frame := siblingWire.next(t, work)
						require.NoError(t, frame.err)
						require.Equal(t, byte(4), frame.data[18])
						wire.release()
					}
					select {
					case err := <-cleanupDone:
						require.NoError(t, err)
					case <-work.Done():
						t.Fatal("process cleanup did not join")
					}
					require.NoError(t, host.Context().Err(), "one failed replay must not end the daemon")
					if ending.completed {
						require.NoError(t, original.writeMessage(wire, &message.Keepalive{}))
						frame := wire.next(t, work)
						require.NoError(t, frame.err)
						require.Equal(t, byte(4), frame.data[18])
					} else {
						frame := wire.next(t, work)
						require.ErrorIs(t, frame.err, io.EOF)
						require.Error(t, original.writeMessage(wire, &message.Keepalive{}))
					}
					require.NoError(t, sibling.session.writeMessage(siblingWire, &message.Keepalive{}))
					frame := siblingWire.next(t, work)
					require.NoError(t, frame.err)
					require.Equal(t, byte(4), frame.data[18])
				})
			}
		})
	}
}

// A source grant cut removes an actual selected route during a refresh, but
// leaves the healthy selecting process able to finish its own Begin/End. This
// distinguishes source retirement from whole-process retirement on the wire.
func TestSelectedRefreshSourceCleanupPreservesPendingBegin(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	wire := selectedRefreshCleanupPeerWire(t, fixture.peer.session, false)
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	frame := wire.next(t, work)
	require.NoError(t, frame.err)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, frame.data))
	route := fixture.route(t, 1, "192.0.2.79", 0, false)
	output, err := fixture.send(t, work, route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, output.Results[0].Outcome)
	frame = wire.next(t, work)
	require.NoError(t, frame.err)
	bodies := updateBodies(t, frame.data)
	require.Len(t, bodies, 1)
	require.Len(t, updatePrefixes(t, bodies[0], false), 1)
	sourceReceipts.mu.Lock()
	lease := sourceReceipts.entries[route.SourceReceipt].leases[fixture.owner]
	sourceReceipts.mu.Unlock()
	source := lease.record.facts.source.info.peer
	source.advanceForwardGeneration()
	source.state.Store(int32(PeerStateStopped))
	require.NoError(t, sourceReceipts.revokePeerOwner(work, source, fixture.owner))
	require.NoError(t, fixture.api.r.retireSelectedOwner(work, fixture.owner, source))
	frame = wire.next(t, work)
	require.NoError(t, frame.err)
	bodies = updateBodies(t, frame.data)
	require.Len(t, bodies, 1)
	require.Len(t, updatePrefixes(t, bodies[0], true), 1)
	input.Action = rpc.SelectedRefreshEnd
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	frame = wire.next(t, work)
	require.NoError(t, frame.err)
	require.Equal(t, []string{"eorr"}, refreshWireLabels(t, frame.data))
	require.NoError(t, fixture.peer.session.writeMessage(wire, &message.Keepalive{}))
	frame = wire.next(t, work)
	require.NoError(t, frame.err)
	require.Equal(t, byte(4), frame.data[18])
}

// Two genuine same-name Process lifetimes cannot complete or replace one
// another's pending Begin. Cleanup of the old owner closes only its original
// Session, and cannot reach a replacement Session's fresh successful replay.
func TestSelectedRefreshPendingOwnerAndReplacementIsolation(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	fixture, input := selectedRefreshFixture(t)
	work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	old := selectedCleanupProcess(t, fixture, makeProcess)
	oldOwner := fixture.owner
	wire := selectedRefreshCleanupPeerWire(t, fixture.peer.session, false)
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	frame := wire.next(t, work)
	require.NoError(t, frame.err)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, frame.data))
	successor := selectedCleanupProcess(t, fixture, makeProcess)
	require.False(t, oldOwner.SameIncarnation(fixture.owner))
	for _, action := range []rpc.SelectedRefreshAction{rpc.SelectedRefreshBegin, rpc.SelectedRefreshEnd} {
		input.Action = action
		request, err := plugin.NewSelectedRefresh(input)
		require.NoError(t, err)
		_, err = fixture.api.SelectRefresh(work, request, fixture.owner)
		require.Error(t, err, "a new process must not erase the old incarnation's pending Begin")
	}
	old.Stop()
	require.NoError(t, fixture.api.RetireSelectedOwner(work, oldOwner))
	frame = wire.next(t, work)
	require.ErrorIs(t, frame.err, io.EOF)
	require.NoError(t, successor.RequirePeerDeclarations())
	replacement, _ := newGroupUpdatesPeer(t, "192.0.2.80", "absent")
	replacement.settings.ExportPolicy = fixture.peer.settings.ExportPolicy
	replacement.settings.ProcessBindings = fixture.peer.settings.ProcessBindings
	replacement.reactor = fixture.api.r
	replacement.session.initialReplay.Store(73)
	replacement.negotiated.Load().EnhancedRouteRefresh = true
	replacement.refreshForwardFacts()
	fixture.api.r.peers[replacement.Settings().PeerKey()] = replacement
	fixture.peer = replacement
	freshWire := selectedRefreshCleanupPeerWire(t, replacement.session, false)
	input.PeerOwner, input.DestinationSession = replacement.sourceOwner, 73
	for _, action := range []rpc.SelectedRefreshAction{rpc.SelectedRefreshBegin, rpc.SelectedRefreshEnd} {
		input.Action = action
		require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
		frame = freshWire.next(t, work)
		require.NoError(t, frame.err)
		label := "borr"
		if action == rpc.SelectedRefreshEnd {
			label = "eorr"
		}
		require.Equal(t, []string{label}, refreshWireLabels(t, frame.data))
	}
	require.NoError(t, fixture.api.RetireSelectedOwner(work, oldOwner))
	require.NoError(t, replacement.session.writeMessage(freshWire, &message.Keepalive{}))
	frame = freshWire.next(t, work)
	require.NoError(t, frame.err)
	require.Equal(t, byte(4), frame.data[18])
}

// Ending one negotiated family's actual Begin cannot erase another family's
// obligation. Once both Ends reach the wire, whole-owner cleanup preserves the
// same useful connection. This proves a family ledger rather than one bool.
func TestSelectedRefreshPendingFamiliesNeedTheirOwnEnd(t *testing.T) {
	for _, finishBoth := range []bool{false, true} {
		name := "one-family-pending"
		if finishBoth {
			name = "all-families-completed"
		}
		t.Run(name, func(t *testing.T) {
			fixture, input := selectedRefreshFixture(t)
			work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			fixture.peer.negotiated.Load().families[family.IPv6Unicast] = true
			wire := selectedRefreshCleanupPeerWire(t, fixture.peer.session, false)
			send := func(fam family.Family, action rpc.SelectedRefreshAction, subtype byte) {
				input.Family, input.Action = fam.String(), action
				require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
				frame := wire.next(t, work)
				require.NoError(t, frame.err)
				// Check the complete normal ROUTE-REFRESH frame independently
				// of refreshWireLabels, whose marker names are IPv4-specific.
				expected := []byte{
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0, 23, 5, byte(fam.AFI >> 8), byte(fam.AFI), subtype, byte(fam.SAFI),
				}
				require.Equal(t, expected, frame.data)
			}
			send(family.IPv4Unicast, rpc.SelectedRefreshBegin, 1)
			send(family.IPv6Unicast, rpc.SelectedRefreshBegin, 1)
			send(family.IPv4Unicast, rpc.SelectedRefreshEnd, 2)
			if finishBoth {
				send(family.IPv6Unicast, rpc.SelectedRefreshEnd, 2)
			}
			require.NoError(t, fixture.api.RetireSelectedOwner(work, fixture.owner))
			if finishBoth {
				require.NoError(t, fixture.peer.session.writeMessage(wire, &message.Keepalive{}))
				frame := wire.next(t, work)
				require.NoError(t, frame.err)
				require.Equal(t, byte(4), frame.data[18])
				return
			}
			frame := wire.next(t, work)
			require.ErrorIs(t, frame.err, io.EOF)
		})
	}
}

// A refusal, explicit NotNegotiated receipt or standalone End has no unfinished
// Begin to retire. A real subsequent KEEPALIVE must be the next useful frame.
func TestSelectedRefreshWithoutBeginPreservesConnection(t *testing.T) {
	for _, mode := range []string{"refused", "not-negotiated", "end-only"} {
		t.Run(mode, func(t *testing.T) {
			fixture, input := selectedRefreshFixture(t)
			work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			wire := selectedRefreshCleanupPeerWire(t, fixture.peer.session, false)
			if mode == "refused" {
				input.DestinationSession++
			}
			if mode == "not-negotiated" {
				fixture.peer.negotiated.Load().EnhancedRouteRefresh = false
			}
			if mode == "end-only" {
				input.Action = rpc.SelectedRefreshEnd
			}
			request, err := plugin.NewSelectedRefresh(input)
			require.NoError(t, err)
			output, err := fixture.api.SelectRefresh(work, request, fixture.owner)
			if mode == "refused" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NoError(t, output.Validate(input))
				if mode == "not-negotiated" {
					require.Equal(t, rpc.SelectedRefreshNotNegotiated, output.Outcome)
				} else {
					require.Equal(t, rpc.SelectedRefreshWritten, output.Outcome)
					frame := wire.next(t, work)
					require.NoError(t, frame.err)
					require.Equal(t, []string{"eorr"}, refreshWireLabels(t, frame.data))
				}
			}
			require.NoError(t, fixture.api.RetireSelectedOwner(work, fixture.owner))
			require.NoError(t, fixture.peer.session.writeMessage(wire, &message.Keepalive{}))
			frame := wire.next(t, work)
			require.NoError(t, frame.err)
			require.Equal(t, byte(4), frame.data[18])
		})
	}
}

// A failed/canceled local UPDATE owns direct retirement Close under writeMu.
// Its first Close remains held after a later ordinary socket claimant's duplicate
// Close and done channel finish. Selected-owner cleanup must join that actual
// writer as well, while preserving another Session's useful output.
func TestSelectedRefreshOwnerCleanupJoinsCanceledLocalWriter(t *testing.T) {
	for _, transport := range []string{"native", "framed"} {
		t.Run(transport, func(t *testing.T) {
			fixture, input := selectedRefreshFixture(t)
			work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			_, client, proc := selectedRefreshCleanupSDK(t, fixture, transport == "framed", work)
			session := fixture.peer.session
			wire := selectedRefreshCleanupPeerWire(t, session, true)
			result, err := client.SelectRefresh(work, input)
			require.NoError(t, err)
			require.Equal(t, rpc.SelectedRefreshWritten, result.Outcome)
			frame := wire.next(t, work)
			require.NoError(t, frame.err)
			require.Equal(t, []string{"borr"}, refreshWireLabels(t, frame.data))
			sibling, _ := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
			sibling.settings.ExportPolicy = fixture.peer.settings.ExportPolicy
			sibling.settings.ProcessBindings = fixture.peer.settings.ProcessBindings
			sibling.reactor = fixture.api.r
			sibling.session.initialReplay.Store(72)
			sibling.refreshForwardFacts()
			fixture.api.r.peers[sibling.Settings().PeerKey()] = sibling
			siblingWire := selectedRefreshCleanupPeerWire(t, sibling.session, false)
			local, stopLocal := context.WithCancel(work)
			cut := &selectedRefreshCleanupWriteCut{entered: make(chan struct{}), fail: local.Done()}
			wire.writeCut.Store(cut)
			localDone, localJoined := make(chan error, 1), make(chan struct{})
			go func() {
				defer close(localJoined)
				localDone <- session.sendUpdateCounted(local, &message.Update{}, nil, false, nil, (*Session).writeUpdate)
			}()
			t.Cleanup(func() {
				stopLocal()
				wire.release()
				join, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
				defer stop()
				select {
				case <-localJoined:
				case <-join.Done():
					t.Error("canceled local UPDATE did not join")
				}
			})
			select {
			case <-cut.entered:
			case err := <-localDone:
				t.Fatalf("local UPDATE never entered its actual writer: %v", err)
			case <-work.Done():
				t.Fatal("local UPDATE never entered its actual writer")
			}
			stopLocal()
			select {
			case <-wire.closing:
			case <-work.Done():
				t.Fatal("canceled local UPDATE never reached its direct retirement Close")
			}
			proc.Stop()
			cleanupDone := make(chan error, 1)
			go func() { cleanupDone <- proc.WaitRuntimeCleanupContext(work) }()
			select {
			case <-wire.duplicate:
			case err := <-cleanupDone:
				t.Fatalf("process cleanup did not retire the pending marker socket: %v", err)
			case <-work.Done():
				t.Fatal("process cleanup never reached the captured peer socket")
			}
			claim := session.socketClosed.Load()
			require.NotNil(t, claim)
			require.Same(t, wire, claim.conn)
			select {
			case <-claim.done:
			case <-work.Done():
				t.Fatal("ordinary duplicate-close claimant never completed")
			}
			probe, stop := context.WithTimeout(work, 100*time.Millisecond)
			require.ErrorIs(t, proc.WaitRuntimeCleanupContext(probe), context.DeadlineExceeded,
				"a completed duplicate-close claim cannot stand in for the held local writer")
			stop()
			require.NoError(t, sibling.session.writeMessage(siblingWire, &message.Keepalive{}))
			frame = siblingWire.next(t, work)
			require.NoError(t, frame.err)
			require.Equal(t, byte(4), frame.data[18])
			wire.release()
			select {
			case err := <-localDone:
				require.ErrorIs(t, err, context.Canceled)
			case <-work.Done():
				t.Fatal("local writer never joined its actual Close")
			}
			select {
			case err := <-cleanupDone:
				require.NoError(t, err)
			case <-work.Done():
				t.Fatal("process cleanup never joined the original writer")
			}
			frame = wire.next(t, work)
			require.ErrorIs(t, frame.err, io.EOF)
			require.True(t, session.writeMu.TryLock(), "cleanup must join the actual final writer")
			session.writeMu.Unlock()
		})
	}
}

// A previous destination may exhaust the shared process-cleanup context.
// A second pending Session must still join its actual direct-retirement writer;
// returning the context error after a duplicate Close is not terminal cleanup.
func TestSelectedRefreshPreexpiredCleanupJoinsLocalWriter(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	session := fixture.peer.session
	wire := selectedRefreshCleanupPeerWire(t, session, true)
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	frame := wire.next(t, work)
	require.NoError(t, frame.err)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, frame.data))
	local, stopLocal := context.WithCancel(work)
	cut := &selectedRefreshCleanupWriteCut{entered: make(chan struct{}), fail: local.Done()}
	wire.writeCut.Store(cut)
	localDone, localJoined := make(chan error, 1), make(chan struct{})
	go func() {
		defer close(localJoined)
		localDone <- session.sendUpdateCounted(local, &message.Update{}, nil, false, nil, (*Session).writeUpdate)
	}()
	retirementJoined := make(chan struct{})
	t.Cleanup(func() {
		stopLocal()
		wire.release()
		join, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		select {
		case <-localJoined:
		case <-join.Done():
			t.Error("preexpired cleanup's local writer did not join")
		}
		select {
		case <-retirementJoined:
		case <-join.Done():
			t.Error("preexpired terminal cleanup did not join")
		}
	})
	select {
	case <-cut.entered:
	case <-work.Done():
		t.Fatal("local UPDATE never reached its actual writer")
	}
	stopLocal()
	select {
	case <-wire.closing:
	case <-work.Done():
		t.Fatal("local writer never reached direct retirement Close")
	}
	expired, expire := context.WithCancel(work)
	expire()
	retired := make(chan error, 1)
	go func() {
		defer close(retirementJoined)
		retired <- fixture.api.RetireSelectedOwner(expired, fixture.owner)
	}()
	select {
	case <-wire.duplicate:
	case err := <-retired:
		t.Fatalf("expired cleanup returned before retiring its pending Session: %v", err)
	case <-work.Done():
		t.Fatal("expired cleanup never reached its original socket")
	}
	claim := session.socketClosed.Load()
	require.NotNil(t, claim)
	select {
	case <-claim.done:
	case <-work.Done():
		t.Fatal("duplicate-close claimant did not complete")
	}
	probe, stop := context.WithTimeout(work, 100*time.Millisecond)
	select {
	case err := <-retired:
		stop()
		t.Fatalf("expired cleanup returned while the actual first local Close was held: %v", err)
	case <-probe.Done():
		stop()
	}
	wire.release()
	select {
	case err := <-localDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-work.Done():
		t.Fatal("local writer did not join its actual Close")
	}
	select {
	case err := <-retired:
		require.ErrorIs(t, err, context.Canceled, "report overdue only after actual resource joins")
	case <-work.Done():
		t.Fatal("expired cleanup did not join its terminal writer")
	}
	frame = wire.next(t, work)
	require.ErrorIs(t, frame.err, io.EOF)
	require.True(t, session.writeMu.TryLock(), "expired cleanup must join its actual writer")
	session.writeMu.Unlock()
}
