package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// captureDocStdio runs fn with stdout and stderr redirected to pipes and
// answers its exit code and what it wrote to each. Not safe for parallel tests.
func captureDocStdio(t *testing.T, fn func() int) (code int, stdout, stderr string) {
	t.Helper()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWrite, errWrite
	outDone := make(chan []byte, 1)
	go drainDocPipe(outRead, outDone)
	code = fn()
	os.Stdout, os.Stderr = savedOut, savedErr
	if closeErr := outWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if closeErr := errWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	errOut, _ := io.ReadAll(errRead) //nolint:errcheck // a short read shows as a mismatch
	errRead.Close()                  //nolint:errcheck // test pipe
	return code, string(<-outDone), string(errOut)
}

// drainDocPipe reads the stdout pipe to its end, so the long command list
// cannot fill the pipe and block the function under test.
func drainDocPipe(r *os.File, done chan<- []byte) {
	data, _ := io.ReadAll(r) //nolint:errcheck // a short read shows as a mismatch
	r.Close()                //nolint:errcheck // test pipe
	done <- data
}

// TestShowYANGDocAnswersTheDocumentationText proves `show yang doc` answers
// its documentation text as a value, writing nothing itself, and `ze yang doc`
// prints exactly the text formatDocList and formatDocCommand wrote to stdout
// before the conversion.
//
// VALIDATES: --list and one command's page each come back as a command.Blob
// equal to the writer's output; the printing route writes those bytes and
// nothing on stderr; no argument and an unknown command come back as the old
// lines with exit code 1.
// PREVENTS: the documentation landing on the daemon's stdout when it serves
// the command.
func TestShowYANGDocAnswersTheDocumentationText(t *testing.T) {
	var list bytes.Buffer
	if err := formatDocList(&list); err != nil {
		t.Fatal(err)
	}
	docs, err := AllRPCDocs()
	if err != nil || len(docs) == 0 {
		t.Fatalf("AllRPCDocs = %d docs, %v; the test needs one", len(docs), err)
	}
	var page bytes.Buffer
	if err := formatDocCommand(&page, docs[0].CLICommand); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"list", []string{"--list"}, list.String()},
		{"command", strings.Fields(docs[0].CLICommand), page.String()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var answer command.LocalAnswer
			_, stdout, stderr := captureDocStdio(t, func() int {
				answer = docAnswer(tc.args)
				return 0
			})
			if stdout != "" || stderr != "" {
				t.Errorf("docAnswer wrote stdout %d bytes, stderr %q; a handler writes nothing", len(stdout), stderr)
			}
			blob, isBlob := answer.Payload.(command.Blob)
			if !isBlob || string(blob) != tc.want || answer.Code != 0 || answer.Diagnostic != "" {
				t.Errorf("docAnswer = code %d diagnostic %q blob %v; want the writer's text", answer.Code, answer.Diagnostic, isBlob)
			}

			code, stdout, stderr := captureDocStdio(t, func() int { return cmdDoc(tc.args) })
			if code != 0 || stdout != tc.want || stderr != "" {
				t.Errorf("ze yang doc: code %d stderr %q, stdout equal %v; want 0 and the text", code, stderr, stdout == tc.want)
			}
		})
	}

	usage := docAnswer(nil)
	if usage.Payload != nil || usage.Code != 1 || usage.Diagnostic != "Usage: ze yang doc <command> or ze yang doc --list\n" {
		t.Errorf("doc with no command = %+v; want the usage line and code 1", usage)
	}
	unknown := docAnswer([]string{"no-such-command-xyz"})
	if unknown.Payload != nil || unknown.Code != 1 || unknown.Diagnostic != "error: unknown command: no-such-command-xyz\n" {
		t.Errorf("doc of an unknown command = %+v; want the error line and code 1", unknown)
	}
	help := docAnswer([]string{"-h"})
	if help.Payload != nil || help.Code != 0 || !strings.Contains(help.Diagnostic, "ze yang doc [--list] [<command>]") {
		t.Errorf("doc -h = %+v; want the help page and code 0", help)
	}
}
