package crashlog

import (
	"errors"
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

	buf, inPanic, err := relayStderr(&failingReader{prefix: []byte(panicTrace), err: want}, nil)

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
	buf, inPanic, err := relayStderr(strings.NewReader(panicTrace), nil)

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
	buf, inPanic, err := relayStderr(strings.NewReader("just a log line\n"), nil)

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
