package hookcheck

import (
	"io"
	"strings"
	"testing"
	"time"
)

// neverAnswers models os.Stdin when no hook payload is coming: a descriptor the
// parent holds open, so the read has nothing to reach EOF on.
//
// It is a plain io.Reader on purpose. An os.Pipe would be pollable and would let
// a read deadline pass this test while bounding nothing in the product, which is
// exactly the fix that shipped and did nothing.
type neverAnswers struct{ release chan struct{} }

func (r neverAnswers) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

// TestPayloadReaderBoundsAReaderThatNeverAnswers is the case the bound exists for.
//
// VALIDATES: a reader that never returns ends the wait at the bound, and the
// caller is told the bound was reached.
// PREVENTS: the hang that put three le processes on one machine at 59 minutes, 7
// hours and 21 hours, each a hook verb typed as a command.
func TestPayloadReaderBoundsAReaderThatNeverAnswers(t *testing.T) {
	blocked := neverAnswers{release: make(chan struct{})}
	defer close(blocked.release)

	start := time.Now()
	in, waited := payloadReader(blocked, 50*time.Millisecond)

	if !waited {
		t.Error("waited = false, want true: a reader that never answers must reach the bound")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("payloadReader took %v, want under a second: the bound was 50ms", elapsed)
	}
	rest, err := io.ReadAll(in)
	if err != nil {
		t.Fatalf("read the answered stream: %v", err)
	}
	if len(rest) != 0 {
		t.Errorf("answered %d bytes, want 0: no payload arrived", len(rest))
	}
}

// TestPayloadReaderPassesAPayloadThrough pins the path every real hook takes.
//
// VALIDATES: the payload is readable in full through the answered reader, first
// byte included, and the bound is not reported.
// PREVENTS: two defects the first byte invites. A bound that refuses the hook
// runner would block every tool call, and a reader that consumed the first byte
// to test it would hand the decoder a payload missing its opening brace.
func TestPayloadReaderPassesAPayloadThrough(t *testing.T) {
	const payload = `{"tool_name":"Write","tool_input":{"file_path":"x"}}`

	in, waited := payloadReader(strings.NewReader(payload), payloadWait)

	if waited {
		t.Error("waited = true, want false: the payload was there to read")
	}
	got, err := io.ReadAll(in)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != payload {
		t.Errorf("read %q, want %q", got, payload)
	}
}

// TestPayloadReaderTreatsAnEmptyStreamAsAbsentNotUnending separates the two ways
// a payload can fail to arrive.
//
// VALIDATES: a stream at EOF answers at once and is NOT reported as having
// reached the bound.
// PREVENTS: a closed stdin being reported to the operator as a ten-second wait
// that never happened. hookruntime.Run already decides what each hook kind does
// with an absent payload, and this path must leave that decision alone.
func TestPayloadReaderTreatsAnEmptyStreamAsAbsentNotUnending(t *testing.T) {
	in, waited := payloadReader(strings.NewReader(""), payloadWait)

	if waited {
		t.Error("waited = true, want false: EOF is an absent payload, not an unending wait")
	}
	got, err := io.ReadAll(in)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("read %d bytes, want 0", len(got))
	}
}
