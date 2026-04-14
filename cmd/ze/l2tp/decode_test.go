package l2tp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// These tests exercise the decode subcommand end-to-end by redirecting
// os.Stdin and os.Stdout and calling cmdDecode directly. They are the
// wiring test for the l2tp-1-wire phase (see spec Wiring Test table).

// TestDecodeSCCRQ corresponds to AC-21 and test/parse/l2tp-wire-decode-sccrq.ci:
// given the hex of a full SCCRQ, `ze l2tp decode` emits JSON with the
// expected header fields and named AVPs, exit 0.
func TestDecodeSCCRQ(t *testing.T) {
	// Same hex as in test/parse/l2tp-wire-decode-sccrq.ci.
	hex := "c8020044000000000000000080080000000000018008000000020100800a0000000300000003800e000000076c61632d64656d6f800800000009123480080000000a0010\n"

	stdout, _, code := runDecode(t, hex, nil)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	contents := string(stdout)
	wants := []string{
		`"is-control":true`,
		`"length":68`,
		`"name":"message-type"`,
		`"name":"protocol-version"`,
		`"name":"host-name"`,
		`"value":"6c61632d64656d6f"`,
	}
	for _, w := range wants {
		if !strings.Contains(contents, w) {
			t.Fatalf("stdout missing %q\noutput:\n%s", w, contents)
		}
	}
	// Sanity: valid JSON.
	var m map[string]any
	if err := json.Unmarshal(stdout, &m); err != nil {
		t.Fatalf("stdout not valid JSON: %v", err)
	}
}

// TestDecodeTruncated corresponds to AC-22 and test/parse/l2tp-wire-decode-truncated.ci.
// A 1-byte input cannot be parsed as an L2TP header; the command exits 1 and
// stderr mentions "short buffer".
func TestDecodeTruncated(t *testing.T) {
	_, stderr, code := runDecode(t, "c8\n", nil)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(string(stderr), "short buffer") {
		t.Fatalf("stderr missing 'short buffer': %s", stderr)
	}
}

// runDecode invokes cmdDecode with the given stdin and returns stdout, stderr, exit code.
func runDecode(t *testing.T, stdin string, args []string) ([]byte, []byte, int) {
	t.Helper()
	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	t.Cleanup(func() { os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr })

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe in: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe out: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe err: %v", err)
	}

	os.Stdin = inR
	os.Stdout = outW
	os.Stderr = errW

	writeErr := make(chan error, 1)
	go func() {
		_, e := inW.WriteString(stdin)
		if cerr := inW.Close(); cerr != nil && e == nil {
			e = cerr
		}
		writeErr <- e
	}()
	done := make(chan int, 1)
	go func() { done <- cmdDecode(args) }()
	code := <-done

	if err := outW.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	if err := errW.Close(); err != nil {
		t.Fatalf("close stderr: %v", err)
	}
	var outBuf, errBuf bytes.Buffer
	if _, copyErr := io.Copy(&outBuf, outR); copyErr != nil {
		t.Fatalf("copy stdout: %v", copyErr)
	}
	if _, copyErr := io.Copy(&errBuf, errR); copyErr != nil {
		t.Fatalf("copy stderr: %v", copyErr)
	}
	if e := <-writeErr; e != nil && !errors.Is(e, io.ErrClosedPipe) {
		t.Fatalf("write stdin: %v", e)
	}
	return outBuf.Bytes(), errBuf.Bytes(), code
}
