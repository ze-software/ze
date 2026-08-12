package capa

import (
	"bufio"
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
// VALIDATES: runDecodeMode returns non-zero and writes the read error when the
// input reader fails part way through a line.
// PREVENTS: a truncated decode stream exiting 0 as if every request decoded.
func TestRunDecodeModeReportsReadError(t *testing.T) {
	var output bytes.Buffer
	code := runDecodeMode(&truncatedReader{}, &output)
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

// TestRunDecodeModeReportsOversizeLine checks the scanner failure that carries
// no underlying I/O error: one line above bufio.MaxScanTokenSize.
//
// VALIDATES: runDecodeMode returns non-zero and writes bufio.ErrTooLong when a
// request line exceeds the scanner token limit.
// PREVENTS: an over-long request being dropped and reported as a clean decode.
func TestRunDecodeModeReportsOversizeLine(t *testing.T) {
	var output bytes.Buffer
	line := "decode capability 1 " + strings.Repeat("00", bufio.MaxScanTokenSize)
	code := runDecodeMode(strings.NewReader(line+"\n"), &output)
	if code == 0 {
		t.Fatalf("expected non-zero exit for an over-long line, got 0")
	}
	if !strings.Contains(output.String(), bufio.ErrTooLong.Error()) {
		t.Fatalf("token-too-long failure not reported, output: %q", output.String())
	}
}
