package crashlog

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"testing/iotest"
	"time"
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

	input := strings.Repeat("x", 2*relayLineOctetsMax) + "\nthe line after\n"
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

// forwardedWrites holds every write the relay forwards, one string per write,
// so a test can wait for one without polling a shared buffer.
type forwardedWrites chan string

func (c forwardedWrites) Write(p []byte) (int, error) {
	c <- string(p)
	return len(p), nil
}

// TestRelayStderrForwardsAPartialLine proves a write with no newline reaches the
// real stderr without waiting for one.
//
// The method writes a prompt with no line end into a pipe the relay reads, and
// writes nothing else, then waits for the prompt to come out of the relay. The
// relay once forwarded whole lines only: `ze init` wrote "username: " into the
// pipe and blocked reading the terminal, and the prompt stayed in the relay, so
// the operator saw nothing to answer.
func TestRelayStderrForwardsAPartialLine(t *testing.T) {
	pr, pw := io.Pipe()
	out := make(forwardedWrites, 16)
	done := make(chan error, 1)
	go relayPartialLine(pr, out, done)

	if _, err := pw.Write([]byte("username: ")); err != nil {
		t.Fatal(err)
	}

	var got string
	timeout := time.After(5 * time.Second)
	for got != "username: " {
		select {
		case s := <-out:
			got += s
		case <-timeout:
			pw.Close() //nolint:errcheck // test cleanup
			t.Fatalf("the prompt did not reach stderr without a newline; got %q", got)
		}
	}

	pw.Close() //nolint:errcheck // ends the relay
	if err := <-done; err != nil {
		t.Fatalf("the relay stopped: %v", err)
	}
}

func relayPartialLine(r io.Reader, out io.Writer, done chan<- error) {
	_, _, err := relayStderr(r, out, nil)
	done <- err
}

// TestRelayStderrKeepsLineFramingForPanics proves a panic header split across
// reads is still recognized at its line start, and a partial line at the end
// of input is collected without a newline being invented on stderr.
//
// The method feeds the trace one byte per read, so no read holds a whole line,
// and compares what reached stderr with the input byte for byte.
func TestRelayStderrKeepsLineFramingForPanics(t *testing.T) {
	sink, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close() //nolint:errcheck // test cleanup

	input := panicTrace + "no line end"
	buf, inPanic, err := relayStderr(iotest.OneByteReader(strings.NewReader(input)), sink, nil)
	if err != nil {
		t.Fatalf("the relay stopped: %v", err)
	}
	if !inPanic {
		t.Fatal("a panic header split across reads was not detected")
	}
	if !strings.Contains(string(buf), "main.main()\n") {
		t.Fatalf("the trace lost the frame line split across reads:\n%s", buf)
	}
	if !strings.HasSuffix(string(buf), "no line end\n") {
		t.Fatalf("the trace lost the partial line at the end of input:\n%s", buf)
	}
	got, err := os.ReadFile(sink.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Fatalf("stderr got %q, want %q", got, input)
	}
}
