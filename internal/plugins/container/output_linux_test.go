// Design: docs/architecture/container/runtime.md -- the output of a container

package container

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/opencontainers/runc/libcontainer"
)

// TestCopyOutputLinesCutsALongLineAtACharacterBoundary proves the line bound:
// a line longer than outputLineBytesMax becomes several rows, none longer
// than the bound, cut before a character the first row has no room for
// whole, so no byte is lost and no character is split.
//
// Method: a line of outputLineBytesMax-1 ASCII bytes, then a two-byte
// character, then a tail; the bound falls inside the character.
//
// MUTATION: cut at the bound with no runeCut and the first row ends in half
// a character, which outputText turns into the replacement character.
func TestCopyOutputLinesCutsALongLineAtACharacterBoundary(t *testing.T) {
	head := strings.Repeat("a", outputLineBytesMax-1)
	buffer := newOutputBuffer()
	if err := copyOutputLines(strings.NewReader(head+"étail\nnext\n"), buffer, outputStreamStdout); err != nil {
		t.Fatalf("the copy failed: %v", err)
	}
	rows := buffer.after(0).rows
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row.line) > outputLineBytesMax {
			t.Errorf("row %d holds %d bytes, above the bound of %d", row.sequence, len(row.line), outputLineBytesMax)
		}
		lines = append(lines, row.line)
	}
	want := []string{head, "étail", "next"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the rows are %q, want %q", shorten(lines), shorten(want))
	}
}

// shorten answers lines with each cut to its first 16 bytes and its length,
// for a failure message.
func shorten(lines []string) []string {
	short := make([]string, 0, len(lines))
	for _, line := range lines {
		short = append(short, line[:min(len(line), 16)]+"...("+strconv.Itoa(len(line))+")")
	}
	return short
}

// TestCopyOutputLinesKeepsTheOrderOfAStream proves the rows of one stream
// keep the order the container wrote them, whatever the reads cut, and that
// a last line with no line end is kept at the end of the stream.
//
// Method: a reader that answers one byte at a time, so every line spans many
// reads.
//
// MUTATION: drop the add of pending at the end of the stream and the last
// line is lost.
func TestCopyOutputLinesKeepsTheOrderOfAStream(t *testing.T) {
	buffer := newOutputBuffer()
	source := iotest.OneByteReader(strings.NewReader("one\r\ntwo\n\nthree"))
	if err := copyOutputLines(source, buffer, outputStreamStderr); err != nil {
		t.Fatalf("the copy failed: %v", err)
	}
	rows := buffer.after(0).rows
	want := []string{"one", "two", "", "three"}
	if len(rows) != len(want) {
		t.Fatalf("the copy added %d rows, want %d", len(rows), len(want))
	}
	for i, row := range rows {
		if row.line != want[i] {
			t.Errorf("row %d is %q, want %q", i, row.line, want[i])
		}
		if row.stream != outputStreamStderr {
			t.Errorf("row %d has stream %s, want stderr", i, row.stream)
		}
		if row.sequence != uint64(i+1) {
			t.Errorf("row %d has sequence %d, want %d", i, row.sequence, i+1)
		}
	}
}

// writeLines writes count numbered lines to file, then closes done. It is
// the writer of TestOutputPipesNeverBlockTheContainer.
func writeLines(file *os.File, prefix string, count int, done chan<- error) {
	line := strings.Repeat("y", 1000)
	for i := range count {
		if _, err := file.WriteString(prefix + strconv.Itoa(i) + " " + line + "\n"); err != nil {
			done <- err
			return
		}
	}
	done <- nil
}

// TestOutputPipesNeverBlockTheContainer proves the pipes a container gets:
// a writer that writes four times the buffer to each stream, with nobody
// reading the buffer, is never held back, both streams reach the one buffer
// with their own mark, each in its order, and once the write ends are
// closed the readers end and the newest rows are held.
//
// Method: attachOutput on a library process, whose write ends the test
// writes to as the entrypoint would.
//
// MUTATION: make the readers stop at the bound instead of dropping the
// oldest rows, and the writers block on a full pipe until the test's bound.
func TestOutputPipesNeverBlockTheContainer(t *testing.T) {
	buffer := newOutputBuffer()
	process := &libcontainer.Process{}
	output, err := attachOutput("talker", process, buffer)
	if err != nil {
		t.Fatalf("the pipes were not made: %v", err)
	}
	stdout, isFile := process.Stdout.(*os.File)
	if !isFile {
		t.Fatalf("the process's stdout is %T, want the write end of a pipe", process.Stdout)
	}
	stderr, isFile := process.Stderr.(*os.File)
	if !isFile {
		t.Fatalf("the process's stderr is %T, want the write end of a pipe", process.Stderr)
	}
	if process.Stdin != nil {
		t.Errorf("the process's stdin is %T, want nothing connected", process.Stdin)
	}
	const count = 4 << 10 // about 4 MiB a stream
	outDone, errDone := make(chan error, 1), make(chan error, 1)
	go writeLines(stdout, "out ", count, outDone)
	go writeLines(stderr, "err ", count, errDone)
	for _, done := range []chan error{outDone, errDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("a write failed: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("a writer was held back by its output")
		}
	}
	if err := output.closeWriteEnds(); err != nil {
		t.Fatalf("the write ends did not close: %v", err)
	}
	if err := output.wait(); err != nil {
		t.Fatalf("the readers ended with %v", err)
	}

	read := buffer.after(0)
	if read.dropped == 0 {
		t.Errorf("no row was dropped after %d bytes a stream, want the oldest dropped", count*1000)
	}
	last := map[outputStream]int{outputStreamStdout: -1, outputStreamStderr: -1}
	prefix := map[outputStream]string{outputStreamStdout: "out ", outputStreamStderr: "err "}
	for _, row := range read.rows {
		number, _, cut := strings.Cut(strings.TrimPrefix(row.line, prefix[row.stream]), " ")
		if !cut {
			t.Fatalf("row %d of %s is not a written line: %.40q", row.sequence, row.stream, row.line)
		}
		index, err := strconv.Atoi(number)
		if err != nil {
			t.Fatalf("row %d of %s is not one of its stream's lines: %.40q", row.sequence, row.stream, row.line)
		}
		if index <= last[row.stream] {
			t.Fatalf("row %d of %s is line %d after line %d", row.sequence, row.stream, index, last[row.stream])
		}
		last[row.stream] = index
	}
	if last[outputStreamStdout] != count-1 {
		t.Errorf("the newest stdout row is line %d, want %d", last[outputStreamStdout], count-1)
	}
	if last[outputStreamStderr] != count-1 {
		t.Errorf("the newest stderr row is line %d, want %d", last[outputStreamStderr], count-1)
	}
}
