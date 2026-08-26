// Related: leroot.go -- the registration adapter these tests drive directly

package leroot

import (
	"os"
	"strings"
	"testing"
)

// captureStderr runs fn with os.Stderr replaced by a pipe and answers what fn
// wrote. The refusal writes to os.Stderr by name, which is what keeps it inside
// the one exemption the no-Sprintf check makes, so reading it back means moving
// the file rather than injecting a writer.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("open a pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write
	t.Cleanup(func() { os.Stderr = saved })

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, readErr := read.Read(buf)
			sb.Write(buf[:n]) //nolint:errcheck // strings.Builder never fails
			if readErr != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	write.Close() //nolint:errcheck // the reader answers what it has
	captured := <-done
	read.Close() //nolint:errcheck // read to EOF
	return captured
}

// VALIDATES: a value typed after an argument-less command is refused, the
// message names the command and quotes what was typed, and the usage line says
// what may follow.
// PREVENTS: fourteen hand-written copies of this refusal disagreeing about what
// a developer may type, which is the reason it is stated once.
func TestRefuseArgumentNamesTheCommandAndWhatWasTyped(t *testing.T) {
	var code int
	captured := captureStderr(t, func() { code = RefuseArgument("iface-resolution", "internal") })

	if code != 1 {
		t.Errorf("the refusal answers %d, want 1", code)
	}
	lines := strings.Split(strings.TrimSuffix(captured, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("the refusal wrote %d lines, want the message and the usage: %q", len(lines), captured)
	}
	if lines[0] != `error: iface-resolution takes no arguments, got "internal"` {
		t.Errorf("the message is %q", lines[0])
	}
	if lines[1] != "usage: le iface-resolution [| json | yaml | table]" {
		t.Errorf("the usage line is %q", lines[1])
	}
}
