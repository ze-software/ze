// Design: docs/contributing/rfc-conformance-gates.md -- live observation output and retained attribution.
// Related: discriminate_observe.go.
package rfc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"testing"
	"time"
)

// TestObservationExecStreamsProgress proves both child streams are visible before
// completion: a real child waits for an acknowledgment after each stream's line.
// A separate recorder process keeps the test from replacing global os.Stderr.
func TestObservationExecStreamsProgress(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, exitCode := range []int{0, 7} {
		t.Run(strconv.Itoa(exitCode), func(t *testing.T) {
			listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close() //nolint:errcheck // Closing the test listener releases its port.
			deadline := time.Now().Add(30 * time.Second)
			if err := listener.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithDeadline(t.Context(), deadline)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestObservationProgressProcess$",
				"--", "--observation-progress", "recorder", listener.Addr().String(), strconv.Itoa(exitCode))
			var result bytes.Buffer
			cmd.Stdout = &result
			progress, err := cmd.StderrPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			// Every failure path cancels and reaps the recorder; its own deadline
			// also bounds the observed child if the recorder is killed first.
			defer func() {
				cancel()
				if cmd.ProcessState == nil {
					_ = cmd.Wait() //nolint:errcheck // Cancellation is expected on an assertion failure.
				}
			}()
			conn, err := listener.AcceptTCP()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close() //nolint:errcheck // Closing the handshake connection releases the child.
			if err := conn.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(progress)
			for _, want := range []string{"stdout before release\n", "stderr before release\n"} {
				got, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("child progress was not visible before release: %v", err)
				}
				if got != want {
					t.Fatalf("live progress = %q, want %q", got, want)
				}
				if _, err := conn.Write([]byte{1}); err != nil {
					t.Fatal(err)
				}
			}
			remaining, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if string(remaining) != "child released\n" {
				t.Fatalf("final progress = %q, want child released", remaining)
			}
			if err := cmd.Wait(); err != nil {
				t.Fatalf("recorder failed: %v; result: %s", err, result.String())
			}
			var observed observationProgressResult
			if err := json.Unmarshal(result.Bytes(), &observed); err != nil {
				t.Fatalf("structured stdout contaminated: %v; result: %s", err, result.String())
			}
			if observed.Passed != (exitCode == 0) {
				t.Errorf("passed = %v for child exit %d", observed.Passed, exitCode)
			}
			if observed.Error != "" {
				t.Errorf("ordinary child exit became an observation error: %s", observed.Error)
			}
			const wantOutput = "stdout before release\nstderr before release\nchild released\n"
			if observed.Output != wantOutput {
				t.Errorf("captured attribution = %q, want %q", observed.Output, wantOutput)
			}
		})
	}
}

type observationProgressResult struct {
	Passed bool
	Output string
	Error  string
}

// TestObservationProgressProcess is the real recorder/child subprocess entry.
// The child cannot finish until its parent has observed each stream's progress.
func TestObservationProgressProcess(t *testing.T) {
	marker := slices.Index(os.Args, "--observation-progress")
	if marker < 0 {
		return
	}
	args := os.Args[marker+1:]
	if len(args) != 3 {
		t.Fatal("expected role, handshake address and exit code")
	}
	if args[0] == "recorder" {
		runner := observationRunner{}
		passed, output, err := runner.exec(20*time.Second, []string{
			os.Args[0], "-test.run=^TestObservationProgressProcess$",
			"--", "--observation-progress", "child", args[1], args[2],
		}, os.Environ(), "")
		result := observationProgressResult{Passed: passed, Output: output}
		if err != nil {
			result.Error = err.Error()
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	if args[0] != "child" {
		t.Fatalf("unknown subprocess role %q", args[0])
	}
	os.Exit(runObservationProgressChild(t, args[1], args[2]))
}

// runObservationProgressChild is the child's handshake, kept apart from
// os.Exit so the deferred Close runs on every failure path. It answers the
// exit code the parent asked for.
func runObservationProgressChild(t *testing.T, address, exitArg string) int {
	t.Helper()
	dialer := net.Dialer{Timeout: 20 * time.Second}
	conn, err := dialer.DialContext(t.Context(), "tcp4", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // Failure paths release the handshake connection.
	if err := conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, stream := range []struct {
		writer io.Writer
		line   string
	}{
		{os.Stdout, "stdout before release\n"},
		{os.Stderr, "stderr before release\n"},
	} {
		if _, err := io.WriteString(stream.writer, stream.line); err != nil {
			t.Fatal(err)
		}
		var acknowledgment [1]byte
		if _, err := io.ReadFull(conn, acknowledgment[:]); err != nil {
			t.Fatalf("parent did not observe progress: %v", err)
		}
		if acknowledgment[0] != 1 {
			t.Fatalf("invalid progress acknowledgment %d", acknowledgment[0])
		}
	}
	if _, err := os.Stdout.WriteString("child released\n"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	exitCode, err := strconv.Atoi(exitArg)
	if err != nil {
		t.Fatal(err)
	}
	return exitCode
}
