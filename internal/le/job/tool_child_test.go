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
// The ordinary exit cases start /bin/sh; the signal case starts this test
// executable. It sends signals to this test process itself, which catches
// them too, so none of them ends the parent test binary.

package job

import (
	"bytes"
	"flag"
	"net"
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
// while it runs. Its method: a real child installs all three signal handlers
// before publishing readiness, marks each signal only after the one before
// it, and exits 42 on the third. The parent retries each signal because the
// relay is installed only once the child has started. The parent MUST close
// the owned control connection on every failure and MUST join the child and
// relay before cleanup. The child's read owner MUST end when that connection
// closes; it does not rely on the signal relay to stop.
func TestRunToolForwardsEverySignalUntilTheChildExits(t *testing.T) {
	if args := flag.Args(); len(args) == 4 {
		if args[0] == "job-signal-child" {
			if code, exit := runJobSignalChild(t, args[1], args[2], args[3]); exit {
				os.Exit(code)
			}
			return
		}
	}
	own := make(chan os.Signal, 8)
	signal.Notify(own, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	t.Cleanup(func() { signal.Stop(own) })

	marks := filepath.Join(t.TempDir(), "mark")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	type acceptance struct {
		connection net.Conn
		err        error
	}
	accepted := make(chan acceptance, 1)
	go func() {
		defer close(accepted)
		connection, err := listener.Accept()
		accepted <- acceptance{connection: connection, err: err}
	}()
	deadline := time.Now().Add(20 * time.Second)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	retry := time.NewTicker(50 * time.Millisecond)
	defer retry.Stop()
	point := centralPoint{errOut: &bytes.Buffer{}}
	answered := make(chan int, 1)
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		answered <- point.runTool(Tool{
			Path: binary,
			Argv: []string{
				binary, "-test.run=^TestRunToolForwardsEverySignalUntilTheChildExits$",
				"--", "job-signal-child", listener.Addr().String(), marks,
				deadline.Format(time.RFC3339Nano),
			},
			Environ: os.Environ(),
		})
	}()
	var control net.Conn
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close fixture listener: %v", err)
		}
		if control != nil {
			if err := control.Close(); err != nil {
				t.Errorf("stop child fixture: %v", err)
			}
		}
		for result := range accepted {
			if result.connection != nil {
				if err := result.connection.Close(); err != nil {
					t.Errorf("stop late child fixture: %v", err)
				}
			}
		}
		<-ended
	})
	select {
	case result := <-accepted:
		if result.err != nil {
			t.Fatalf("accept child fixture: %v", result.err)
		}
		control = result.connection
	case code := <-answered:
		t.Fatalf("the child exited before installing its signal handlers: code %d", code)
	case <-timer.C:
		t.Fatal("the child never connected its cleanup control")
	}
	for {
		select {
		case code := <-answered:
			t.Fatalf("the child exited before installing its signal handlers: code %d", code)
		default:
		}
		if _, err := os.Stat(marks + ".ready"); err == nil {
			break
		}
		select {
		case <-timer.C:
			t.Fatal("the child never installed its signal handlers")
		case <-retry.C:
		}
	}

	steps := []struct {
		signal syscall.Signal
		mark   string
	}{
		{syscall.SIGTERM, marks + ".term"},
		{syscall.SIGHUP, marks + ".hup"},
	}
	for _, step := range steps {
		for {
			select {
			case code := <-answered:
				t.Fatalf("the child exited before answering %v: code %d", step.signal, code)
			default:
			}
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

func runJobSignalChild(t *testing.T, addr, marks, deadlineArg string) (int, bool) {
	t.Helper()
	deadline, err := time.Parse(time.RFC3339Nano, deadlineArg)
	if err != nil {
		t.Fatal(err)
	}
	dialer := net.Dialer{Deadline: deadline}
	control, err := dialer.DialContext(t.Context(), "tcp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := control.SetReadDeadline(deadline); err != nil {
		if closeErr := control.Close(); closeErr != nil {
			t.Errorf("close child control: %v", closeErr)
		}
		t.Fatal(err)
	}
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(signals)
	disconnected := make(chan struct{})
	go func() {
		defer close(disconnected)
		var input [1]byte
		// Every read completion stops the fixture: EOF, the original
		// deadline, or unexpected control data.
		if _, err := control.Read(input[:]); err == nil {
			t.Error("unexpected child control data")
		}
	}()
	defer func() {
		if err := control.Close(); err != nil {
			t.Errorf("close child control: %v", err)
		}
		<-disconnected
	}()
	if err := os.WriteFile(marks+".ready", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-disconnected:
			return 0, false
		case got := <-signals:
			switch got {
			case syscall.SIGTERM:
				if err := os.WriteFile(marks+".term", nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case syscall.SIGHUP:
				if _, err := os.Stat(marks + ".term"); err == nil {
					if err := os.WriteFile(marks+".hup", nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case syscall.SIGQUIT:
				if _, err := os.Stat(marks + ".hup"); err == nil {
					return 42, true
				}
			}
		}
	}
}
