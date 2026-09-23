//go:build linux

package l2tp

import (
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"golang.org/x/sys/unix"
)

// Authentication completes before interface configuration. The nil embedded
// backend fails if a session unexpectedly reaches that boundary.
type authReloadBackend struct{ ppp.IfaceBackend }

// MUTATION: resolving the auth handler once at drain startup makes the second
// session retain the accepting provider after the registry switches to rejection.
func TestL2TPAuthDrainFollowsProviderReplacement(t *testing.T) {
	// The provider slot is process-global, so this test must remain serial.
	original := subscriber.GetAuthHandler()
	t.Cleanup(func() {
		subscriber.UnregisterAuthHandler()
		subscriber.RegisterAuthHandler(original)
	})
	bus := newTestBus()
	decisions := make(chan *subevents.SessionAuthResultPayload, 3)
	subevents.SessionAuthResult.Subscribe(bus, func(result *subevents.SessionAuthResultPayload) {
		decisions <- result
	})
	driver := ppp.NewProductionDriver(slog.Default(), authReloadBackend{})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	local := func(ppp.EventAuthRequest, AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true}
	}
	replacement := func(ppp.EventAuthRequest, AuthRespondFunc) AuthResult {
		return AuthResult{Accept: false}
	}
	RegisterAuthHandler(local)
	done := startAuthDrain(slog.Default(), driver, GetAuthHandler, bus)
	t.Cleanup(func() {
		driver.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("auth drain did not stop")
		}
	})
	for index, want := range []bool{true, false, true} {
		if index == 1 {
			RegisterAuthHandler(replacement)
		} else {
			RegisterAuthHandler(local)
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
			// Proxy LCP with only MRU reaches the production admission request
			// without a kernel PPP device. NCP remains unconfigured.
			options := []byte{1, 4, 5, 220}
			driver.SessionsIn() <- ppp.StartSession{
				TunnelID: tunnelID, SessionID: sessionID, ChanFD: fds[0], UnitFD: unit,
				LNSMode: true, ProxyLCPInitialRecv: options,
				ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
			}
			// Queue delivery transfers both descriptors to the driver.
			select {
			case decision := <-decisions:
				if decision.Accept != want || decision.AccessType != subscriber.AccessL2TP ||
					decision.SessionID != l2tpSessionID(tunnelID, sessionID) {
					t.Fatalf("request %d decision=%+v, want accept=%t via L2TP session %d/%d",
						index, decision, want, tunnelID, sessionID)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("request %d received no authentication decision", index)
			}
		}()
	}
}
