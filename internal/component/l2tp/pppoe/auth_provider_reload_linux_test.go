//go:build linux

package pppoe

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"golang.org/x/sys/unix"
)

// The test stops at authentication, before any interface configuration. Calling
// the embedded nil backend would fail if a session crossed that boundary.
type authReloadBackend struct{ ppp.IfaceBackend }

// MUTATION: resolving the handler once before starting the goroutine makes the
// second session use the previous provider, despite the registry replacement.
func TestPPPoEAuthDrainFollowsProviderReplacement(t *testing.T) {
	original := subscriber.GetAuthHandler()
	t.Cleanup(func() {
		subscriber.UnregisterAuthHandler()
		subscriber.RegisterAuthHandler(original)
	})
	bus := newRecordBus()
	decisions := make(chan *subevents.SessionAuthResultPayload, 3)
	subevents.SessionAuthResult.Subscribe(bus, func(result *subevents.SessionAuthResultPayload) {
		decisions <- result
	})
	driver := ppp.NewProductionDriver(slog.Default(), authReloadBackend{})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	var pending sync.Map
	local := func(ppp.EventAuthRequest, subscriber.AuthRespondFunc) subscriber.AuthResult {
		return subscriber.AuthResult{Accept: true}
	}
	remote := func(ppp.EventAuthRequest, subscriber.AuthRespondFunc) subscriber.AuthResult {
		return subscriber.AuthResult{Accept: false}
	}
	subscriber.RegisterAuthHandler(local)
	done := startPPPoEAuthDrain(slog.Default(), driver, subscriber.GetAuthHandler, bus, &pending)
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
			subscriber.RegisterAuthHandler(remote)
		} else {
			subscriber.RegisterAuthHandler(local)
		}
		func() {
			fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = unix.Close(fds[1]) }()
			unit, err := unix.Dup(fds[0])
			if err != nil {
				_ = unix.Close(fds[0])
				t.Fatal(err)
			}
			// Proxy LCP with only MRU gets the production PPP driver to its
			// authentication request without a kernel PPP device or timer.
			options := []byte{1, 4, 5, 220}
			driver.SessionsIn() <- ppp.StartSession{
				TunnelID: 7, SessionID: uint16(index + 1), ChanFD: fds[0], UnitFD: unit,
				LNSMode: true, ProxyLCPInitialRecv: options,
				ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
			}
			select {
			case decision := <-decisions:
				if decision.Accept != want || decision.AccessType != subscriber.AccessPPPoE {
					t.Fatalf("request %d decision=%+v, want accept=%t via PPPoE", index, decision, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("request %d received no authentication decision", index)
			}
		}()
	}
}
