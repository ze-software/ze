// Design: docs/architecture/behavior/signals.md -- the daemon announces each operating-system signal it takes
// Related: main.go -- waitLoop announces a SIGHUP before the reload; run announces the stop signal once forceCh is registered

package hub

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/daemonevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// requestedStop is the stop a command (`daemon shutdown`) or the end of stdin
// asks for. It travels on the signal channel so one loop orders every stop,
// and it is its own type so waitLoop can tell it from a signal the operating
// system delivered: only the second is announced.
type requestedStop struct{}

func (requestedStop) String() string { return "requested stop" }

// Signal makes requestedStop an os.Signal; it carries no signal number.
func (requestedStop) Signal() {}

// announceSignal publishes one operating-system signal on the EventBus
// (daemonevents.Signal), for every subsystem that tells its processes about
// it. The bus delivers to engine subscribers synchronously, so each has queued
// its lines to its processes before the daemon reloads or stops. Queued, not
// written: the reactor enqueues per process and waits for no answer
// (deliverPeerLine), so a wedged subscriber cannot hold the stop. A nil signal
// (the server ended on its own) and requestedStop announce nothing.
func announceSignal(bus ze.EventBus, sig os.Signal) {
	number, isSystem := sig.(syscall.Signal)
	if !isSystem {
		return
	}
	received := daemonevents.Received{Number: int(number), Name: unix.SignalName(number)}
	if _, err := daemonevents.Signal.Emit(bus, received); err != nil {
		slogutil.Logger("hub").Warn("signal announcement not published", "signal", received.Name, "error", err)
	}
}
