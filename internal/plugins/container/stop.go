// Design: docs/architecture/container/runtime.md -- the stop: the stop signal, the stop timeout, the kill
// Related: supervise.go -- the supervisor, which sends the signal and kills after the timeout
// Related: config.go -- the stop-signal and stop-timeout leaves this file reads
// Related: storeseam.go -- the image's own stop signal, in the store's answer

package container

import (
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The words of the stop leaves, as the plugin's YANG module spells them.
const (
	leafStopTimeout = "stop-timeout"
	leafStopSignal  = "stop-signal"
)

// The range and the default of stop-timeout, in seconds. The top is 30
// because the drain budget gives the containers 30 seconds to obey their stop
// signals (drainBudgetSeconds).
const (
	stopTimeoutSecondsMin     = 1
	stopTimeoutSecondsMax     = 30
	stopTimeoutSecondsDefault = 10
)

// killCleanupTimeout bounds the wait for a container's cleanup after its
// control group was killed: the last 5 seconds of the drain budget.
const killCleanupTimeout = 5 * time.Second

// stopSignal is the signal the config's stop-signal leaf names. Zero is
// stopSignalFromImage, the guard for a config that names none: the image's
// own signal is sent then, or the termination signal when the image declares
// none. It never names a signal.
type stopSignal uint8

const (
	stopSignalFromImage stopSignal = iota
	stopSignalTerm
	stopSignalInt
	stopSignalQuit
	stopSignalHup
	stopSignalUsr1
	stopSignalUsr2
)

// stopSignalWords holds the YANG word of each named value.
var stopSignalWords = [...]string{
	stopSignalTerm: "term",
	stopSignalInt:  "int",
	stopSignalQuit: "quit",
	stopSignalHup:  "hup",
	stopSignalUsr1: "usr1",
	stopSignalUsr2: "usr2",
}

// stopSignalNumbers holds the kernel signal of each named value.
var stopSignalNumbers = [...]syscall.Signal{
	stopSignalTerm: syscall.SIGTERM,
	stopSignalInt:  syscall.SIGINT,
	stopSignalQuit: syscall.SIGQUIT,
	stopSignalHup:  syscall.SIGHUP,
	stopSignalUsr1: syscall.SIGUSR1,
	stopSignalUsr2: syscall.SIGUSR2,
}

// parseStopSignal answers the value the YANG word names, and false for a word
// the enumeration does not hold.
func parseStopSignal(word string) (stopSignal, bool) {
	for value := stopSignalTerm; value <= stopSignalUsr2; value++ {
		if stopSignalWords[value] == word {
			return value, true
		}
	}
	return stopSignalFromImage, false
}

// imageSignalNumberMax is the highest signal number Linux delivers (SIGRTMAX).
const imageSignalNumberMax = 64

// stopSignalOf answers the signal a stop sends to the named container: the
// config's stop-signal, else the signal the image declares, else the
// termination signal. The image writes its signal as a name, with or without
// its SIG prefix, or as a number. One Ze cannot read refuses the start, never
// falls back to the termination signal: the workload asked for another signal
// to stop cleanly, and a stop with the wrong one ends in a kill.
func stopSignalOf(name string, configured stopSignal, image string) (syscall.Signal, error) {
	if configured != stopSignalFromImage {
		return stopSignalNumbers[configured], nil
	}
	written := strings.TrimSpace(image)
	if written == "" {
		return syscall.SIGTERM, nil
	}
	if number, err := strconv.Atoi(written); err == nil {
		if number < 1 {
			return 0, refuseImageStopSignal(name, image)
		}
		if number > imageSignalNumberMax {
			return 0, refuseImageStopSignal(name, image)
		}
		return syscall.Signal(number), nil
	}
	signalName := strings.ToUpper(written)
	if !strings.HasPrefix(signalName, "SIG") {
		signalName = "SIG" + signalName
	}
	if signal := unix.SignalNum(signalName); signal != 0 {
		return signal, nil
	}
	return 0, refuseImageStopSignal(name, image)
}

func refuseImageStopSignal(name, image string) error {
	return opererr.New(
		"container "+name+": its image declares the stop signal "+strconv.Quote(image)+", which ze cannot read",
		"a stop sends the signal the image names, and ze refuses to guess another one",
		"set container instance "+name+" "+leafStopSignal+" term",
	)
}
