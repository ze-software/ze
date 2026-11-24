//go:build linux

package l2tp

import (
	"log/slog"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

// MUTATION: resolving the auth handler once at drain startup makes the second
// session retain the accepting provider after the registry switches to rejection.
func TestL2TPAuthDrainFollowsProviderReplacement(t *testing.T) {
	// The provider slots are process-global, so this test must remain serial.
	t.Cleanup(func() {
		UnregisterAuthHandler(AuthPriorityRemote)
		UnregisterAuthHandler(AuthPriorityLocal)
	})
	bus := newTestBus()
	decisions := make(chan *subevents.SessionAuthResultPayload, 3)
	subevents.SessionAuthResult.Subscribe(bus, func(result *subevents.SessionAuthResultPayload) {
		decisions <- result
	})
	driver := ppp.NewProductionDriver(slog.Default())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	local := func(ppp.EventAuthRequest, AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true}
	}
	replacement := func(ppp.EventAuthRequest, AuthRespondFunc) AuthResult {
		return AuthResult{Accept: false}
	}
	RegisterAuthHandler(AuthPriorityLocal, local)
	done := startAuthDrain(slog.Default(), driver, GetAuthHandler, bus)
	t.Cleanup(func() {
		driver.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("auth drain did not stop")
		}
	})
	for index, step := range []struct {
		accept   bool
		identity ppp.SessionIdentity
	}{
		{true, protocolFixtureIdentity("ppp0", 0x80000000)},
		{false, protocolFixtureIdentity("ppp1", 0x80000001)},
		{true, protocolFixtureIdentity("ppp2", 0x80000002)},
	} {
		if index == 1 {
			RegisterAuthHandler(AuthPriorityRemote, replacement)
		} else {
			UnregisterAuthHandler(AuthPriorityRemote)
		}
		func() {
			fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
			if err != nil {
				t.Fatal(err)
			}
			// Closing the peer releases the driver's blocking reader before
			// Driver.Stop waits for the session goroutines during cleanup.
			defer func() { _ = unix.Close(fds[1]) }()
			unit, err := unix.Dup(fds[0])
			if err != nil {
				_ = unix.Close(fds[0])
				t.Fatal(err)
			}
			const tunnelID = 7
			sessionID := uint16(index + 1)
			birth := newFixtureBirth(t)
			id := birth.String()
			// Proxy LCP with only MRU reaches the production admission request
			// without a kernel PPP device. NCP remains unconfigured.
			options := []byte{1, 4, 5, 220}
			frames, owner, err := ownedProtocolSessionFixture(pppkernel.SessionConfig{ChanFD: fds[0], UnitFD: unit, UnitNum: index}, step.identity)
			if err != nil {
				t.Fatal(err)
			}
			attempt, err := ppp.NewSessionAttempt(id, owner)
			if err != nil {
				t.Fatal(err)
			}
			driver.SessionsIn() <- ppp.StartSession{
				LCP:      testLCPConfig(),
				TunnelID: tunnelID, SessionID: sessionID,
				L2TPBirth: birth, SubscriberID: id,
				Attempt: attempt,
				Frames:  frames, Dataplane: owner,
				LNSMode: true, ProxyLCPInitialRecv: options,
				ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
			}
			// Queue delivery transfers the factory-created stream owner to the driver.
			select {
			case decision := <-decisions:
				if decision.Accept != step.accept || decision.AccessType != subscriber.AccessL2TP ||
					decision.SessionID != id {
					t.Fatalf("request %d decision=%+v, want accept=%t via L2TP session %d/%d",
						index, decision, step.accept, tunnelID, sessionID)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("request %d received no authentication decision", index)
			}
		}()
	}
}
