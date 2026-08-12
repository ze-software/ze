package softver

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// truncatedReader gives one partial line, then fails. It models a pipe that
// breaks mid-stream: bufio.Scanner reports that failure through Err(), never
// through Scan().
type truncatedReader struct{ sent bool }

func (r *truncatedReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, io.ErrUnexpectedEOF
	}
	r.sent = true
	return copy(p, "decode capability "), nil
}

// TestRunDecodeModeReportsReadError checks that a read failure is not reported
// as a clean end of input.
//
// VALIDATES: RunDecodeMode returns non-zero and writes the read error when the
// input reader fails part way through a line.
// PREVENTS: a truncated decode stream exiting 0 as if every request decoded.
func TestRunDecodeModeReportsReadError(t *testing.T) {
	var output bytes.Buffer
	code := RunDecodeMode(&truncatedReader{}, &output)
	if code == 0 {
		t.Fatalf("expected non-zero exit after a read failure, got 0")
	}
	if !strings.Contains(output.String(), "decoded error") {
		t.Fatalf("read failure not reported, output: %q", output.String())
	}
	if !strings.Contains(output.String(), io.ErrUnexpectedEOF.Error()) {
		t.Fatalf("error text missing, output: %q", output.String())
	}
}
