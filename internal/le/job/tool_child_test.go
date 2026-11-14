// Related: tool.go -- runTool, the child these cases start
// Related: job.go -- forwardSignals and relaySignals, the relay the signal case drives
// Related: tool_test.go -- the hold's cases, which stub the child
//
// VALIDATES: `le exec` runs a heavy tool as a real child and answers its exit
// code exactly (gaterun.ExitCode), answers gaterun.CannotStart with its
// message for a program that cannot start, answers 128 plus the signal for a
// child killed by one, and relays every SIGTERM, SIGHUP and SIGQUIT le
// receives to the child until it exits, a second and a third signal
// included.
// PREVENTS: a tool left running after le was hung up or sent a second
// signal, while the registry reads le's dead PID as a free slot.
//
// Every case starts /bin/sh. The signal case sends signals to this test
// process itself, which it catches too, so none of them ends the test binary.

package job

import (
	"bytes"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/gaterun"
)

// shellTool answers a Tool running script under /bin/sh, with args as its
// positional parameters.
func shellTool(script string, args ...string) Tool {
	return Tool{Path: "/bin/sh", Argv: append([]string{"sh", "-c", script, "sh"}, args...), Environ: os.Environ()}
}

// TestRunToolAnswersTheChildsOwnCode asks whether runTool answers a real
// child's exit code as it is. Its method: children that exit 0, 7 and 255.
func TestRunToolAnswersTheChildsOwnCode(t *testing.T) {
	point := centralPoint{errOut: &bytes.Buffer{}}
	for _, code := range []string{"0", "7", "255"} {
		got := point.runTool(shellTool("exit " + code))
		if want := map[string]int{"0": 0, "7": 7, "255": 255}[code]; got != want {
			t.Errorf("a child that exits %s answered %d", code, got)
		}
	}
}

// TestRunToolAnswersCannotStartWithItsReason asks whether a program that
// cannot start answers gaterun.CannotStart and writes why. Its method: a
// path that does not exist.
func TestRunToolAnswersCannotStartWithItsReason(t *testing.T) {
	var errOut bytes.Buffer
	missing := filepath.Join(t.TempDir(), "absent")
	point := centralPoint{errOut: &errOut}
	code := point.runTool(Tool{Path: missing, Argv: []string{"absent"}, Environ: os.Environ()})
	if code != gaterun.CannotStart {
		t.Errorf("a program that cannot start answered %d, want %d", code, gaterun.CannotStart)
	}
	if !strings.Contains(errOut.String(), missing) || !strings.Contains(errOut.String(), "could not be started inside its job slot") {
		t.Errorf("the refusal does not name the program and the slot: %q", errOut.String())
	}
}

// TestRunToolAnswersADeathBySignal asks whether a child killed by a signal
// answers 128 plus the signal's number, the code `./le job run` answers for
// one. Its method: a child that sends itself SIGKILL.
func TestRunToolAnswersADeathBySignal(t *testing.T) {
	point := centralPoint{errOut: &bytes.Buffer{}}
	code := point.runTool(shellTool("kill -KILL $$"))
	if want := 128 + int(syscall.SIGKILL); code != want {
		t.Errorf("a child killed by SIGKILL answered %d, want %d", code, want)
	}
}

// TestRunToolForwardsEverySignalUntilTheChildExits asks whether SIGTERM,
// then SIGHUP, then SIGQUIT sent to le each reach the child, in that order,
// while it runs. Its method: a child that marks a file for each signal only
// after the one before it, and exits 42 on the third; the test sends each
// signal to itself until the child's mark for it appears, because the relay
// is installed only once the child has started.
func TestRunToolForwardsEverySignalUntilTheChildExits(t *testing.T) {
	own := make(chan os.Signal, 8)
	signal.Notify(own, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	t.Cleanup(func() { signal.Stop(own) })

	marks := filepath.Join(t.TempDir(), "mark")
	script := `trap 'touch "$1.term"' TERM
trap 'test -e "$1.term" && touch "$1.hup"' HUP
trap 'test -e "$1.hup" && exit 42' QUIT
while :; do sleep 0.02; done`
	point := centralPoint{errOut: &bytes.Buffer{}}
	answered := make(chan int, 1)
	go func() { answered <- point.runTool(shellTool(script, marks)) }()

	deadline := time.Now().Add(20 * time.Second)
	steps := []struct {
		signal syscall.Signal
		mark   string
	}{
		{syscall.SIGTERM, marks + ".term"},
		{syscall.SIGHUP, marks + ".hup"},
	}
	for _, step := range steps {
		for {
			if _, err := os.Stat(step.mark); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the child never answered %v: the signal was not forwarded", step.signal)
			}
			if err := syscall.Kill(os.Getpid(), step.signal); err != nil {
				t.Fatalf("send %v: %v", step.signal, err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	for {
		select {
		case code := <-answered:
			if code != 42 {
				t.Errorf("the child answered %d, want 42 from its third forwarded signal", code)
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the child never answered SIGQUIT: the signal was not forwarded")
		}
		if err := syscall.Kill(os.Getpid(), syscall.SIGQUIT); err != nil {
			t.Fatalf("send SIGQUIT: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
