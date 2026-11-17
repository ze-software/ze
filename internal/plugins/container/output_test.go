// Design: docs/architecture/container/runtime.md -- the output of a container

package container

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestOutputBufferKeepsTheNewestBytes proves the bound of a container's
// output buffer and what a read answers: rows past the newest 1 MiB are
// dropped oldest first, sequence numbers rise by one from 1 and survive the
// drop, each row keeps its stream, a read after a sequence number answers
// only the later rows, and a reader that fell behind learns how many rows it
// missed.
//
// Method: rows of a known size are added until the buffer dropped some, then
// the held rows and two reads are compared with what the bound allows.
//
// MUTATION: drop the loop that removes the oldest rows from add and the cost
// passes the bound; read rows from the start in after and the read after a
// sequence number answers earlier rows.
func TestOutputBufferKeepsTheNewestBytes(t *testing.T) {
	buffer := newOutputBuffer()
	line := strings.Repeat("x", 960) // 960 + 64 = 1024 bytes a row: 1024 rows fit
	const added = 1500
	for i := range added {
		stream := outputStreamStdout
		if i%2 == 1 {
			stream = outputStreamStderr
		}
		buffer.add(stream, line+strconv.Itoa(i))
	}

	all := buffer.after(0)
	cost := 0
	for _, row := range all.rows {
		cost += rowCost(row.line)
	}
	if cost > outputBufferBytes {
		t.Fatalf("the buffer holds rows that cost %d bytes, above its bound of %d", cost, outputBufferBytes)
	}
	if len(all.rows) < 1000 {
		t.Fatalf("the buffer holds %d rows, want the newest that fit 1 MiB, about 1000", len(all.rows))
	}
	newest := all.rows[len(all.rows)-1]
	if newest.sequence != added {
		t.Errorf("the newest row has sequence %d, want %d", newest.sequence, added)
	}
	if newest.line != line+strconv.Itoa(added-1) {
		t.Errorf("the newest row is not the last line added")
	}
	if newest.stream != outputStreamStderr {
		t.Errorf("the newest row has stream %s, want stderr", newest.stream)
	}
	for i := 1; i < len(all.rows); i++ {
		if all.rows[i].sequence != all.rows[i-1].sequence+1 {
			t.Fatalf("row %d has sequence %d after %d, want one more", i, all.rows[i].sequence, all.rows[i-1].sequence)
		}
	}
	oldest := all.rows[0].sequence
	if all.dropped != oldest-1 {
		t.Errorf("a read from the start answered %d dropped rows, want the %d older than the oldest held", all.dropped, oldest-1)
	}

	later := buffer.after(added - 3)
	if len(later.rows) != 3 {
		t.Fatalf("a read after sequence %d answered %d rows, want 3", added-3, len(later.rows))
	}
	if later.rows[0].sequence != added-2 {
		t.Errorf("a read after sequence %d opens at %d, want %d", added-3, later.rows[0].sequence, added-2)
	}
	if later.dropped != 0 {
		t.Errorf("a read after a held sequence answered %d dropped rows, want 0", later.dropped)
	}
	rows := buffer.newest(2)
	if len(rows) != 2 {
		t.Fatalf("the newest two rows are %v, want two", rows)
	}
	if rows[1].sequence != added {
		t.Errorf("the newest of two rows has sequence %d, want %d", rows[1].sequence, added)
	}
}

// TestOutputBufferBoundsEmptyLines proves a stream of empty lines cannot grow
// the buffer past its bound: each row costs its overhead as well as its line.
//
// MUTATION: make rowCost answer len(line) alone and the rows grow without end.
func TestOutputBufferBoundsEmptyLines(t *testing.T) {
	buffer := newOutputBuffer()
	limit := outputBufferBytes / outputRowOverheadBytes
	for range limit * 2 {
		buffer.add(outputStreamStdout, "")
	}
	if held := len(buffer.after(0).rows); held > limit {
		t.Fatalf("the buffer holds %d empty rows, want at most %d", held, limit)
	}
}

// TestOutputTextStripsControlCharacters proves what a row holds of bytes
// that are not plain text: control characters, a carriage return and an
// escape included, are stripped, a tab is kept, and a byte sequence that is
// not UTF-8 becomes the replacement character, so a row cannot drive a
// terminal or forge a log line.
//
// MUTATION: return string(raw) and every case keeps its raw bytes.
func TestOutputTextStripsControlCharacters(t *testing.T) {
	cases := map[string]string{
		"\x1b[31mred\x1b[0m\r": "[31mred[0m",
		"a\tb":                 "a\tb",
		"nul\x00bell\x07":      "nulbell",
		"\xff\xfeok":           "��ok",
		"café":                 "café",
	}
	for raw, want := range cases {
		if got := outputText([]byte(raw)); got != want {
			t.Errorf("outputText(%q) = %q, want %q", raw, got, want)
		}
	}
}

// talkingLibrary is a runtime library whose containers write two known rows
// into their buffer when created and end when the test makes them, or at the
// first stop signal or kill. It records every container it created. Safe for
// concurrent use.
type talkingLibrary struct {
	mu      sync.Mutex
	created []*talkingContainer
}

func (l *talkingLibrary) sweep() []leftover { return nil }

func (l *talkingLibrary) create(_ context.Context, request *startRequest) (createdContainer, error) {
	request.output.add(outputStreamStdout, "hello from "+request.instance.name)
	request.output.add(outputStreamStderr, "boom: a setting is missing")
	created := &talkingContainer{exited: make(chan int, 1)}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.created = append(l.created, created)
	return created, nil
}

func (l *talkingLibrary) last() *talkingContainer {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.created[len(l.created)-1]
}

// talkingContainer is one container of talkingLibrary. Its init exits once,
// with the first code it is given.
type talkingContainer struct {
	exited chan int
	once   sync.Once
}

func (c *talkingContainer) exit(code int) {
	c.once.Do(func() { c.exited <- code })
}

func (c *talkingContainer) wait() (int, error)          { return <-c.exited, nil }
func (c *talkingContainer) signal(syscall.Signal) error { c.exit(143); return nil }
func (c *talkingContainer) kill() error                 { c.exit(137); return nil }
func (c *talkingContainer) initProcessID() int          { return 7 }
func (c *talkingContainer) networkNamespacePath() string {
	return ""
}
func (c *talkingContainer) release() error { return nil }
func (c *talkingContainer) destroy() error { return nil }

// endWithin makes created exit with code and waits for the watcher of the
// named container to end its cleanup.
func endWithin(t *testing.T, sup *supervisor, name string, created *talkingContainer, code int) {
	t.Helper()
	sup.mu.Lock()
	running, isRunning := sup.running[name]
	sup.mu.Unlock()
	if !isRunning {
		t.Fatalf("container %s does not run", name)
	}
	created.exit(code)
	select {
	case <-running.ended:
	case <-time.After(10 * time.Second):
		t.Fatalf("the watcher of %s did not end", name)
	}
}

// TestContainerEndedOnItsOwnIsReportedWithItsOutput proves a container whose
// entrypoint exits by itself with a failure reaches the operator with its
// own explanation: the stopped check reports it with its exit status and the
// last rows it wrote to both streams, and the Warn line carries the same
// cause under the check's code. The record lasts while the stopped state
// does: a restart that runs the container clears it. An end a stop asked for,
// and an exit with status 0, are not reported.
//
// Method: a library whose container writes one row to each stream and exits
// when the test says, under a supervisor that logs to a buffer.
//
// MUTATION: drop the recordFailedExitLocked call from watch and no finding
// is answered; skip the stopRequested check and the stop at the end is
// reported; drop the problemStopped delete from setState and the finding
// outlives the restart.
func TestContainerEndedOnItsOwnIsReportedWithItsOutput(t *testing.T) {
	ctx := context.Background()
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	store := &fakeStore{}
	store.setAnswer(readyImage())
	library := &talkingLibrary{}
	sup := newSupervisor(log, library, storeFactoryOf(store), nil)
	if err := sup.install(configOf(instanceConfig{name: "talker", enabled: true}, instanceConfig{name: "quiet", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	if _, err := sup.start(ctx, "quiet"); err != nil {
		t.Fatalf("the start of quiet failed: %v", err)
	}
	endWithin(t, sup, "quiet", library.last(), 0)
	if _, err := sup.start(ctx, "talker"); err != nil {
		t.Fatalf("the start of talker failed: %v", err)
	}
	endWithin(t, sup, "talker", library.last(), 3)

	findings, err := sup.answerDoctorCheck("container-stopped")
	if err != nil {
		t.Fatalf("the stopped check refused: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("the stopped check answered %v, want one finding, for talker alone", findings)
	}
	want := "container talker ended on its own and stays stopped until it is started again: it exited with status 3; " +
		"its last output: stdout: hello from talker | stderr: boom: a setting is missing"
	if findings[0].Message != want {
		t.Errorf("the finding says %q, want %q", findings[0].Message, want)
	}
	if findings[0].Code != codeStopped {
		t.Errorf("the finding has code %s, want %s", findings[0].Code, codeStopped)
	}
	if findings[0].Fix != "ze request container restart name talker" {
		t.Errorf("the finding's fix is %q, want the restart command", findings[0].Fix)
	}
	if state, known := sup.state("talker"); !known {
		t.Errorf("talker holds no state, want %s", StateStopped)
	} else if state != StateStopped {
		t.Errorf("talker holds state %s, want %s", state, StateStopped)
	}
	if !strings.Contains(logged.String(), "code="+codeStopped) {
		t.Errorf("no log line carries code %s: %s", codeStopped, logged.String())
	}
	if !strings.Contains(logged.String(), "stderr: boom: a setting is missing") {
		t.Errorf("the log does not carry the container's last output: %s", logged.String())
	}

	if _, err := sup.restart(ctx, "talker"); err != nil {
		t.Fatalf("the restart of talker failed: %v", err)
	}
	if findings := stoppedFindings(t, sup); len(findings) != 0 {
		t.Fatalf("after a restart that runs talker the stopped check answered %v, want no finding", findings)
	}

	if err := sup.stop(ctx, "talker"); err != nil {
		t.Fatalf("the stop of talker failed: %v", err)
	}
	if findings := stoppedFindings(t, sup); len(findings) != 0 {
		t.Errorf("a stop Ze asked for was reported as %v, want no finding", findings)
	}
}

// stoppedFindings answers the findings of the stopped check.
func stoppedFindings(t *testing.T, sup *supervisor) []string {
	t.Helper()
	findings, err := sup.answerDoctorCheck("container-stopped")
	if err != nil {
		t.Fatalf("the stopped check refused: %v", err)
	}
	messages := make([]string, 0, len(findings))
	for _, finding := range findings {
		messages = append(messages, finding.Message)
	}
	return messages
}
