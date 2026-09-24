package crashlog

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// VALIDATES: relayStderr marks a panic trace whose read stopped early.
// PREVENTS: a crash file that ends mid-trace being read as the whole panic,
// which sends the next reader after the last frame that reached the pipe
// instead of the last frame the process ran.

// failingReader yields prefix, then fails. It models the stderr pipe breaking
// while a panic trace is still printing.
type failingReader struct {
	prefix []byte
	err    error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.prefix) == 0 {
		return 0, f.err
	}
	n := copy(p, f.prefix)
	f.prefix = f.prefix[n:]
	return n, nil
}

const panicTrace = "panic: boom\n\ngoroutine 1 [running]:\nmain.main()\n\t/src/main.go:10 +0x20\n"

func TestRelayStderrMarksTruncatedPanic(t *testing.T) {
	want := errors.New("pipe broke")

	buf, inPanic, err := relayStderr(&failingReader{prefix: []byte(panicTrace), err: want}, nil, nil)

	if !inPanic {
		t.Fatal("panic start was not detected")
	}
	if !errors.Is(err, want) {
		t.Fatalf("scan error not reported: %v", err)
	}
	if !strings.Contains(string(buf), "TRUNCATED") {
		t.Fatalf("the crash trace does not say it was cut short:\n%s", buf)
	}
	if !strings.Contains(string(buf), "pipe broke") {
		t.Fatalf("the crash trace does not name the read failure:\n%s", buf)
	}
}

func TestRelayStderrLeavesWholeTraceUnmarked(t *testing.T) {
	buf, inPanic, err := relayStderr(strings.NewReader(panicTrace), nil, nil)

	if err != nil {
		t.Fatalf("a whole trace reported an error: %v", err)
	}
	if !inPanic {
		t.Fatal("panic start was not detected")
	}
	if strings.Contains(string(buf), "TRUNCATED") {
		t.Fatalf("a whole trace was marked truncated:\n%s", buf)
	}
	if !strings.Contains(string(buf), "main.main()") {
		t.Fatalf("the trace body is missing:\n%s", buf)
	}
}

func TestRelayStderrNoPanicNoTrace(t *testing.T) {
	buf, inPanic, err := relayStderr(strings.NewReader("just a log line\n"), nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inPanic {
		t.Error("a plain log line was read as a panic")
	}
	if len(buf) != 0 {
		t.Errorf("a trace was collected with no panic:\n%s", buf)
	}
}

// TestRelayStderrSurvivesALongLine proves a line longer than the relay buffer
// neither stops the relay nor loses a byte.
//
// The method relays a line twice the buffer size and a short line after it into
// a file standing in for the original stderr, then compares the file with the
// input. The relay once stopped at such a line: every later line was lost, and
// the unread pipe then blocked the process at its next write.
func TestRelayStderrSurvivesALongLine(t *testing.T) {
	sink, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close() //nolint:errcheck // test cleanup

	input := strings.Repeat("x", 2*relayStderrBuffer) + "\nthe line after\n"
	buf, inPanic, err := relayStderr(strings.NewReader(input), sink, nil)
	if err != nil {
		t.Fatalf("the relay stopped: %v", err)
	}
	if inPanic || len(buf) != 0 {
		t.Fatalf("a long line opened a panic trace: %q", buf)
	}
	got, err := os.ReadFile(sink.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Fatalf("relayed %d bytes, want %d; tail %q", len(got), len(input), got[max(0, len(got)-40):])
	}
}
