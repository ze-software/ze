package fixture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// TestSessionEditorWaitForKeepsFollowingOutput drives the actual session wait
// through a needle split across pending output and the next read. Following
// output stays available exactly once, without duplicating the transcript.
func TestSessionEditorWaitForKeepsFollowingOutput(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if _, err := writer.WriteString("EDLE tail NEXT suffix"); err != nil {
		t.Fatal(err)
	}
	pending := []byte("prefix NE")
	var transcript bytes.Buffer
	transcript.Write(pending)
	rest, err := sessionEditorWaitFor(t.Context(), reader, pending, &transcript, "NEEDLE")
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != " tail NEXT suffix" {
		t.Fatalf("rest=%q", rest)
	}
	if transcript.String() != "prefix NEEDLE tail NEXT suffix" {
		t.Fatalf("transcript=%q", transcript.String())
	}
	pending[0] = 'X'
	rest, err = sessionEditorWaitFor(t.Context(), reader, rest, &transcript, "NEXT")
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != " suffix" {
		t.Fatalf("second rest=%q", rest)
	}
	if transcript.String() != "prefix NEEDLE tail NEXT suffix" {
		t.Fatalf("transcript=%q", transcript.String())
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := sessionEditorWaitFor(ctx, reader, rest, &transcript, "NEXT"); !errors.Is(err, context.Canceled) {
		t.Fatalf("consumed needle matched again: %v", err)
	}
	if transcript.String() != "prefix NEEDLE tail NEXT suffix" {
		t.Fatalf("repeated transcript=%q", transcript.String())
	}
}

// TestSessionEditorWaitForUsesCallerLifetime keeps a silent writer open. The
// caller's cancellation retains pending output, and its deadline also ends a
// wait with no output. Neither route waits for another PTY timeout.
func TestSessionEditorWaitForUsesCallerLifetime(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var transcript bytes.Buffer
	pending := []byte("unfinished result")
	transcript.Write(pending)
	_, err = sessionEditorWaitFor(ctx, reader, pending, &transcript, "commit completed")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error=%v, want caller cancellation", err)
	}
	if transcript.String() != "unfinished result" {
		t.Fatalf("transcript=%q", transcript.String())
	}

	ctx, cancel = context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	_, err = sessionEditorWaitFor(ctx, reader, nil, &transcript, "commit completed")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("silent expired wait=%v", err)
	}
}

// firstLiveWaitContext records the first live cancellation check. Err returns
// the value read before publishing the checkpoint, so concurrent cancellation
// cannot replace that nil result with a pre-canceled fast path.
type firstLiveWaitContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (ctx *firstLiveWaitContext) Err() error {
	err := ctx.Context.Err()
	if err == nil {
		ctx.once.Do(func() { close(ctx.checked) })
	}
	return err
}

type sessionWaitObservation struct {
	pending    []byte
	transcript string
	err        error
}

// observeSessionWait owns the transcript until it publishes its result. Its
// caller MUST cancel, close the pipe, and join done on every exit; the buffered
// answer lets this worker finish even when the test already failed.
func observeSessionWait(ctx context.Context, reader *os.File, pending []byte, answer chan<- sessionWaitObservation, done chan<- struct{}) {
	defer close(done)
	var transcript bytes.Buffer
	transcript.Write(pending)
	rest, err := sessionEditorWaitFor(ctx, reader, pending, &transcript, "commit completed")
	answer <- sessionWaitObservation{pending: rest, transcript: transcript.String(), err: err}
}

// TestSessionEditorWaitForCancelsLiveSilentRead cancels only after the actual
// wait has observed a live context. There is no context deadline to wake Poll:
// the bounded cancellation checkpoint must end a silent read itself.
func TestSessionEditorWaitForCancelsLiveSilentRead(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	lifetime, cancel := context.WithCancel(context.Background())
	ctx := &firstLiveWaitContext{Context: lifetime, checked: make(chan struct{})}
	if _, bounded := ctx.Deadline(); bounded {
		t.Fatal("live-read control unexpectedly has a deadline")
	}
	answer := make(chan sessionWaitObservation, 1)
	done := make(chan struct{})
	defer func() {
		cancel()
		writer.Close()
		reader.Close()
		<-done
	}()
	pending := []byte("unfinished result")
	go observeSessionWait(ctx, reader, pending, answer, done)
	guard := time.NewTimer(5 * time.Second)
	defer guard.Stop()
	select {
	case <-ctx.checked:
	case <-guard.C:
		t.Fatal("session wait never checked its live context")
	}
	cancel()
	select {
	case observed := <-answer:
		if !errors.Is(observed.err, context.Canceled) {
			t.Fatalf("live silent wait=%v, want caller cancellation", observed.err)
		}
		if len(observed.pending) != 0 {
			t.Fatalf("canceled wait consumed an absent status: %q", observed.pending)
		}
		if observed.transcript != "unfinished result" {
			t.Fatalf("canceled transcript=%q", observed.transcript)
		}
		if string(pending) != "unfinished result" {
			t.Fatalf("pending output changed: %q", pending)
		}
	case <-guard.C:
		t.Fatal("live silent wait did not finish after cancellation")
	}
}

// TestReadPTYUntilContextKeepsEOFContract checks the shared read boundary and
// its legacy wrapper: closing a writer is completion only for an EOF drain,
// never evidence that an absent expected status appeared.
func TestReadPTYUntilContextKeepsEOFContract(t *testing.T) {
	readers := []struct {
		name string
		read func(*os.File, []byte, bool, ...string) (string, error)
	}{
		{name: "case context", read: func(file *os.File, initial []byte, eofOK bool, needles ...string) (string, error) {
			return readPTYUntilContext04(t.Context(), file, initial, eofOK, needles...)
		}},
		{name: "legacy bound", read: readPTYUntil04},
	}
	for _, reader := range readers {
		for _, eofOK := range []bool{false, true} {
			name := reader.name + "/status"
			if eofOK {
				name = reader.name + "/drain"
			}
			t.Run(name, func(t *testing.T) {
				file, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if _, err := writer.WriteString("partial"); err != nil {
					writer.Close()
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				got, err := reader.read(file, []byte("prefix "), eofOK, "absent")
				if got != "prefix partial" {
					t.Fatalf("output=%q", got)
				}
				if eofOK {
					if err != nil {
						t.Fatalf("drain=%v", err)
					}
				} else {
					if !errors.Is(err, io.EOF) {
						t.Fatalf("absent status=%v, want EOF", err)
					}
				}
			})
		}
	}
}
