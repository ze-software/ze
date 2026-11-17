// Design: docs/architecture/container/runtime.md -- the output of a container
// Related: output.go -- the buffer the readers fill
// Related: library_linux.go -- the create that hands the pipes to the entrypoint

package container

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/opencontainers/runc/libcontainer"
)

// outputReadBytes is the size of one read from a container's pipe.
const outputReadBytes = 32 << 10

// containerOutput is the two pipes of one container's standard output and
// standard error and their two readers. The write ends go to the entrypoint
// as its descriptors 1 and 2, the way the plugin process package hands a
// forked plugin its stderr: a pipe the Cmd did not create is never closed by
// its Wait, so a reader sees every byte through to end of file. Standard
// input is left unset, which the library's command opens as /dev/null.
//
// Each reader is the one goroutine of its stream for the life of the
// container: it reads to end of file, which comes once every process of the
// container is gone, because only they hold the write ends after the start.
// No idle bound applies: every writer dies with the control group. A reader
// copies into memory and never waits on anyone, so a container never blocks
// on its output.
type containerOutput struct {
	stdout, stderr *os.File
	readers        sync.WaitGroup
	// failures holds what stopped a reader short of the end of its stream,
	// at most one for each of the two. Guarded by mu.
	mu       sync.Mutex
	failures []error
}

// attachOutput makes the two pipes, hands their write ends to process and
// starts one reader per pipe into buffer. The caller MUST call
// closeWriteEnds once the start returned, whatever its result, and MAY call
// wait after the container's processes are gone.
func attachOutput(name string, process *libcontainer.Process, buffer *outputBuffer) (*containerOutput, error) {
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(err, stdoutRead.Close(), stdoutWrite.Close())
	}
	process.Stdout, process.Stderr = stdoutWrite, stderrWrite
	output := &containerOutput{stdout: stdoutWrite, stderr: stderrWrite}
	output.readers.Add(2)
	go output.read(name, stdoutRead, buffer, outputStreamStdout)
	go output.read(name, stderrRead, buffer, outputStreamStderr)
	return output, nil
}

// closeWriteEnds closes the daemon's copy of the write ends, so the readers
// end once the container's own copies are gone. It MUST be called once,
// after the start returned.
func (o *containerOutput) closeWriteEnds() error {
	return errors.Join(o.stdout.Close(), o.stderr.Close())
}

// wait returns once both readers read their stream to its end, so every row
// the container wrote is in its buffer, and answers what stopped a reader
// short of the end or kept its pipe open. It MUST be called after
// closeWriteEnds and once no process of the container is left, or it waits
// for them.
func (o *containerOutput) wait() error {
	o.readers.Wait()
	o.mu.Lock()
	defer o.mu.Unlock()
	return errors.Join(o.failures...)
}

// read is the reader of one stream: it copies the pipe's lines into buffer
// until end of file, then closes the pipe.
func (o *containerOutput) read(name string, pipe *os.File, buffer *outputBuffer, stream outputStream) {
	defer o.readers.Done()
	failure := errors.Join(copyOutputLines(pipe, buffer, stream), pipe.Close())
	if failure == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failures = append(o.failures, fmt.Errorf("container %s: its %s could not be read to its end: %w", name, stream, failure))
}

// copyOutputLines reads source to its end and adds one row of stream to
// buffer for each line, without its line end. A line longer than
// outputLineBytesMax is added as several rows, cut at character boundaries,
// and a last line with no line end is added at the end of the stream.
func copyOutputLines(source io.Reader, buffer *outputBuffer, stream outputStream) error {
	chunk := make([]byte, outputReadBytes)
	pending := make([]byte, 0, outputLineBytesMax)
	for {
		count, readErr := source.Read(chunk)
		data := chunk[:count]
		for len(data) > 0 {
			line, rest, complete := bytes.Cut(data, []byte{'\n'})
			room := outputLineBytesMax - len(pending)
			if len(line) > room {
				pending = append(pending, line[:room]...)
				cut := runeCut(pending)
				buffer.add(stream, outputText(pending[:cut]))
				// What follows the cut, at most the start of one character,
				// opens the next row.
				pending = append(pending[:0], pending[cut:]...)
				data = data[room:]
				continue
			}
			pending = append(pending, line...)
			if !complete {
				break
			}
			buffer.add(stream, outputText(pending))
			pending = pending[:0]
			data = rest
		}
		if readErr == nil {
			continue
		}
		if len(pending) > 0 {
			buffer.add(stream, outputText(pending))
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		return readErr
	}
}
