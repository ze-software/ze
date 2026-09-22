package l2tpauthradius

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	txevents "github.com/ze-software/ze/internal/component/config/transaction/events"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	l2tpauthlocal "github.com/ze-software/ze/internal/component/l2tp/plugins/authlocal"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The engine delivers in-process events synchronously. Callbacks are copied
// before dispatch so unsubscribe has the same lifetime boundary as the engine.
type providerLifecycleBus struct {
	mu       sync.Mutex
	next     uint64
	handlers map[string]map[uint64]func(any)
}

func (b *providerLifecycleBus) Subscribe(namespace, event string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := namespace + "/" + event
	if b.handlers[key] == nil {
		b.handlers[key] = make(map[uint64]func(any))
	}
	b.next++
	id := b.next
	b.handlers[key][id] = handler
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.handlers[key], id)
	}
}

func (b *providerLifecycleBus) Emit(namespace, event string, payload any) (int, error) {
	b.mu.Lock()
	var handlers []func(any)
	for _, handler := range b.handlers[namespace+"/"+event] {
		handlers = append(handlers, handler)
	}
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(payload)
	}
	return 0, nil
}

func installProviderLifecycleBus(t *testing.T) *providerLifecycleBus {
	t.Helper()
	bus := &providerLifecycleBus{handlers: make(map[string]map[uint64]func(any))}
	eventBusMu.Lock()
	previous := storedBus
	eventBusMu.Unlock()
	reg := registry.Lookup(Name)
	reg.ConfigureEventBus(bus)
	t.Cleanup(func() { reg.ConfigureEventBus(previous) })
	return bus
}

func commitProviderLifecycle(t *testing.T, bus *providerLifecycleBus) {
	t.Helper()
	// The coordinator commits first; the server accepts after the whole reload.
	for _, event := range []string{txevents.EventCommitted, txevents.EventAccepted} {
		if _, err := bus.Emit(txevents.Namespace, event, `{"transaction-id":"provider-reload"}`); err != nil {
			t.Fatal(err)
		}
	}
}

func lifecycleRadiusConfig(t *testing.T, address string, timeout int) string {
	t.Helper()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(`{"l2tp":{"auth":{"radius":{"timeout":%d,"server":[{"name":"test","address":%q,"port":%s,"shared-key":"lifecycle-key"}]}}}}`, timeout, host, port)
}

func preserveLifecycleLocal(t *testing.T) {
	t.Helper()
	original := l2tp.GetAuthHandler()
	t.Cleanup(func() {
		l2tpauthlocal.ResetForTest()
		l2tp.RegisterAuthHandler(original)
	})
}

func lifecycleSessionUp(t *testing.T, bus *providerLifecycleBus, sessionID uint16) {
	t.Helper()
	if _, err := bus.Emit(l2tpevents.Namespace, l2tpevents.SessionIPAssignedEvent, &l2tpevents.SessionIPAssignedPayload{
		TunnelID: 71, SessionID: sessionID, Username: "alice", PeerAddr: "192.0.2.71",
	}); err != nil {
		t.Fatal(err)
	}
}

// Removal used to send Admin-Reboot and discard the session before another
// participant could reject apply. Its eventual subscriber teardown then vanished.
func TestRegisteredRadiusRemovalRollbackPreservesAccounting(t *testing.T) {
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	capture := newAcctCapture()
	server, address := startAcctServer(t, []byte("lifecycle-key"), capture)
	t.Cleanup(func() { _ = server.Close() })
	startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	ctx, remote := startAuthProvider(t, Name, lifecycleRadiusConfig(t, address, 1))
	lifecycleSessionUp(t, bus, 1)
	start := capture.waitN(t, 1)[0]

	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "before", true, false)
	capture.mu.Lock()
	for _, packet := range capture.packets {
		if packet.statusType == radius.AcctStatusStop {
			t.Error("tentative removal sent Accounting-Stop before commit")
		}
	}
	capture.mu.Unlock()
	rollbackAuthConfig(t, ctx, remote)
	if _, err := bus.Emit(l2tpevents.Namespace, l2tpevents.SessionDownEvent, &l2tpevents.SessionDownPayload{
		TunnelID: 71, SessionID: 1, Cause: l2tpevents.TerminateCauseUserRequest,
	}); err != nil {
		t.Fatal(err)
	}
	packets := capture.waitN(t, 1)
	last := packets[len(packets)-1]
	if len(packets) != 2 || start.statusType != radius.AcctStatusStart || last.statusType != radius.AcctStatusStop ||
		last.sessionID != start.sessionID || last.terminateCause != uint32(l2tpevents.TerminateCauseUserRequest) {
		t.Fatalf("rollback did not preserve the subscriber's accounting lifetime: %+v", packets)
	}
}

// The declared one-second apply budget must hold even while several accounting
// requests have no response. This is a deadline contract, not a speed benchmark.
func TestRegisteredRadiusRemovalApplyDoesNotWaitForAccounting(t *testing.T) {
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	ctx, remote := startAuthProvider(t, Name, lifecycleRadiusConfig(t, server.LocalAddr().String(), 30))
	// Closing the client on failure also releases the old-source overlay's
	// synchronous Stop, so the regression cannot strand its registered engine.
	authInstance.mu.Lock()
	client := authInstance.client
	authInstance.mu.Unlock()
	t.Cleanup(func() { _ = client.Close() })
	for id := uint16(1); id <= 3; id++ {
		lifecycleSessionUp(t, bus, id)
	}
	if err := server.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	wire := make([]byte, radius.MaxPacketLen)
	for range 3 {
		if _, _, err := server.ReadFromUDP(wire); err != nil {
			t.Fatal(err)
		}
	}
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	applyAuthConfig(t, applyCtx, remote)
	assertProviderPAP(t, "before", true, false)
	commitProviderLifecycle(t, bus)
}

// Hold a committed removal's Stop on the wire while a new provider is installed.
// Finishing that retirement must close the old client without clearing the new one.
func TestRegisteredRadiusCommittedRetirementKeepsNewProvider(t *testing.T) {
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	capture := newAcctCapture()
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() { resume(); _ = server.Close() })
	go func() {
		wire := make([]byte, radius.MaxPacketLen)
		for {
			n, peer, err := server.ReadFromUDP(wire)
			if err != nil {
				return
			}
			packet, err := radius.Decode(wire[:n])
			if err != nil {
				continue
			}
			capture.add(packet)
			if status := packet.FindAttr(radius.AttrAcctStatusType); len(status) == 4 && status[3] == radius.AcctStatusStop {
				<-release
			}
			response := make([]byte, radius.HeaderLen)
			response[0], response[1] = radius.CodeAccountingResp, packet.Identifier
			binary.BigEndian.PutUint16(response[2:4], radius.HeaderLen)
			auth := radius.ResponseAuthenticator(radius.CodeAccountingResp, packet.Identifier, radius.HeaderLen, packet.Authenticator, nil, []byte("lifecycle-key"))
			copy(response[4:], auth[:])
			_, _ = server.WriteToUDP(response, peer)
		}
	}()
	startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	ctx, remote := startAuthProvider(t, Name, lifecycleRadiusConfig(t, server.LocalAddr().String(), 1))
	t.Cleanup(resume)
	authInstance.mu.Lock()
	oldClient := authInstance.client
	authInstance.mu.Unlock()
	lifecycleSessionUp(t, bus, 1)
	start := capture.waitN(t, 1)[0]
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	// An old-source removal blocks here; releasing its held Stop in cleanup
	// keeps RED bounded by the RPC context rather than teardown timeouts.
	applyCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	applyAuthConfig(t, applyCtx, remote)
	commitProviderLifecycle(t, bus)
	packets := capture.waitN(t, 1)
	stop := packets[len(packets)-1]
	if stop.statusType != radius.AcctStatusStop || stop.sessionID != start.sessionID ||
		stop.terminateCause != uint32(l2tpevents.TerminateCauseAdminReboot) {
		t.Fatalf("committed removal did not close its accounting record: %+v", packets)
	}

	acceptServer, acceptAddress := startMockRADIUS(t, []byte("lifecycle-key"), radius.CodeAccessAccept)
	t.Cleanup(func() { _ = acceptServer.Close() })
	verifyAuthConfig(t, ctx, remote, lifecycleRadiusConfig(t, acceptAddress, 1), rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	commitProviderLifecycle(t, bus)
	assertProviderPAP(t, "remote-only", true, true)
	resume()
	// The public client operation observes resource retirement. It succeeds
	// against the acknowledging server until that client's socket is closed.
	deadline, stopWaiting := context.WithTimeout(ctx, 5*time.Second)
	defer stopWaiting()
	for {
		_, err := oldClient.SendToServers(deadline, &radius.Packet{Code: radius.CodeAccountingReq})
		if err != nil {
			if deadline.Err() != nil {
				t.Fatal("retired client remained open until the observation deadline")
			}
			break
		}
		select {
		case <-deadline.Done():
			t.Fatal("retired client remained usable")
		case <-time.After(10 * time.Millisecond):
		}
	}
	assertProviderPAP(t, "remote-only", true, true)
	rollbackAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "remote-only", true, true)
}

// Two registered engine lifetimes reproduce the process manager's joined
// in-process restart. A RADIUS closure must never become the second fallback.
func TestRegisteredRadiusRestartRemovalRestoresLocal(t *testing.T) {
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	server, address := startMockRADIUS(t, []byte("lifecycle-key"), radius.CodeAccessReject)
	t.Cleanup(func() { _ = server.Close() })
	ctx, local := startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	config := lifecycleRadiusConfig(t, address, 1)
	t.Run("first-engine", func(t *testing.T) {
		startAuthProvider(t, Name, config)
		assertProviderPAP(t, "before", false, true)
	})
	_, remote := startAuthProvider(t, Name, config)
	assertProviderPAP(t, "before", false, true)
	verifyAuthConfig(t, ctx, local, localAfterReload, rpc.StatusOK)
	applyAuthConfig(t, ctx, local)
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	commitProviderLifecycle(t, bus)
	assertProviderPAP(t, "after", true, false)
	assertProviderPAP(t, "before", false, false)
}

// A whole reload can finish after another transaction has applied. Even if both
// remove RADIUS, the older acceptance cannot retire the newer pending lifetime.
func TestRegisteredRadiusOlderAcceptanceKeepsNewerRemoval(t *testing.T) {
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	capture := newAcctCapture()
	server, address := startAcctServer(t, []byte("lifecycle-key"), capture)
	t.Cleanup(func() { _ = server.Close() })
	startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	config := lifecycleRadiusConfig(t, address, 1)
	ctx, remote := startAuthProvider(t, Name, config)
	lifecycleSessionUp(t, bus, 1)
	first := capture.waitN(t, 1)[0]
	lifecycleSessionUp(t, bus, 2)
	starts := capture.waitN(t, 1)
	second := starts[len(starts)-1]
	emit := func(event, id string) {
		t.Helper()
		if _, err := bus.Emit(txevents.Namespace, event, fmt.Sprintf(`{"transaction-id":%q}`, id)); err != nil {
			t.Fatal(err)
		}
	}

	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	emit(txevents.EventCommitted, "older-removal")
	verifyAuthConfig(t, ctx, remote, config, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	emit(txevents.EventCommitted, "reactivated")
	emit(txevents.EventAccepted, "reactivated")
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	emit(txevents.EventCommitted, "newer-removal")
	emit(txevents.EventAccepted, "older-removal")

	if _, err := bus.Emit(l2tpevents.Namespace, l2tpevents.SessionDownEvent, &l2tpevents.SessionDownPayload{
		TunnelID: 71, SessionID: 1, Cause: l2tpevents.TerminateCauseUserRequest,
	}); err != nil {
		t.Fatal(err)
	}
	packets := capture.waitN(t, 1)
	stop := packets[len(packets)-1]
	if len(packets) != 3 || stop.sessionID != first.sessionID ||
		stop.statusType != radius.AcctStatusStop || stop.terminateCause != uint32(l2tpevents.TerminateCauseUserRequest) {
		t.Fatalf("older completion retired newer pending accounting: %+v", packets)
	}
	emit(txevents.EventAccepted, "newer-removal")
	packets = capture.waitN(t, 1)
	stop = packets[len(packets)-1]
	if len(packets) != 4 || stop.sessionID != second.sessionID ||
		stop.statusType != radius.AcctStatusStop || stop.terminateCause != uint32(l2tpevents.TerminateCauseAdminReboot) {
		t.Fatalf("newer accepted removal did not retire its remaining session: %+v", packets)
	}
}

// Compensation is itself a transaction. If another participant rejects that
// transaction, returning to the pending removal must retain the original record.
func TestRegisteredRadiusFailedCompensationKeepsAccounting(t *testing.T) {
	preserveLifecycleLocal(t)
	bus := installProviderLifecycleBus(t)
	capture := newAcctCapture()
	server, address := startAcctServer(t, []byte("lifecycle-key"), capture)
	t.Cleanup(func() { _ = server.Close() })
	config := lifecycleRadiusConfig(t, address, 1)
	ctx, remote := startAuthProvider(t, Name, config)
	lifecycleSessionUp(t, bus, 1)
	start := capture.waitN(t, 1)[0]
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	if _, err := bus.Emit(txevents.Namespace, txevents.EventCommitted, `{"transaction-id":"failed-reload"}`); err != nil {
		t.Fatal(err)
	}
	verifyAuthConfig(t, ctx, remote, config, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	rollbackAuthConfig(t, ctx, remote)
	if _, err := bus.Emit(l2tpevents.Namespace, l2tpevents.SessionDownEvent, &l2tpevents.SessionDownPayload{
		TunnelID: 71, SessionID: 1, Cause: l2tpevents.TerminateCauseUserRequest,
	}); err != nil {
		t.Fatal(err)
	}
	packets := capture.waitN(t, 1)
	stop := packets[len(packets)-1]
	if len(packets) != 2 || stop.sessionID != start.sessionID ||
		stop.statusType != radius.AcctStatusStop || stop.terminateCause != uint32(l2tpevents.TerminateCauseUserRequest) {
		t.Fatalf("failed compensation discarded the original accounting lifetime: %+v", packets)
	}
}
