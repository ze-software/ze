package command

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdio runs fn with os.Stdout and os.Stderr redirected to pipes and
// answers what each stream received. Not safe for parallel tests: it swaps the
// process's streams.
func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWrite, errWrite

	// The readers drain while fn writes, so a long write cannot fill a pipe
	// buffer and block fn forever.
	outText := make(chan string, 1)
	errText := make(chan string, 1)
	go drainPipe(outRead, outText)
	go drainPipe(errRead, errText)

	fn()

	os.Stdout, os.Stderr = savedOut, savedErr
	outWrite.Close() //nolint:errcheck // the write end is this function's own
	errWrite.Close() //nolint:errcheck // the write end is this function's own
	stdout, stderr = <-outText, <-errText
	outRead.Close() //nolint:errcheck // the read end is this function's own
	errRead.Close() //nolint:errcheck // the read end is this function's own
	return stdout, stderr
}

func drainPipe(read *os.File, text chan<- string) {
	var buffer strings.Builder
	io.Copy(&buffer, read) //nolint:errcheck // the pipe closes when the writer is done
	text <- buffer.String()
}

// TestLocalDataRefusalIsReturnedNotWritten proves a local-data handler's
// refusal travels in its LocalAnswer, and that each local route prints it.
// PREVENTS: a refusal written to the process's stderr by the handler, which on
// a daemon route lands on the daemon's stderr while the caller gets an empty
// failure.
// Method: one handler that refuses; call it directly with both streams
// captured (nothing may be written), then run it through ServeLocal and the
// plain route and compare stderr, stdout and the exit code with the answer.
func TestLocalDataRefusalIsReturnedNotWritten(t *testing.T) {
	const reason = "error: the thing is missing\n" +
		"hint: run 'ze show thing' to see what exists\n"
	withLocalDataArgs(t, func(ValidatedArgs) LocalAnswer {
		return LocalRefusal(2, reason)
	})

	handler, trailing := LookupLocalData(strings.Fields(localDataArgsPath + " abcd"))
	if handler == nil {
		t.Fatal("LookupLocalData found no handler for the registered path")
	}
	validated, err := ValidateArgs(trailing, nil, nil)
	if err != nil {
		t.Fatalf("judging %v: %v", trailing, err)
	}
	var answer LocalAnswer
	stdout, stderr := captureStdio(t, func() { answer = handler(validated) })
	if stdout != "" || stderr != "" {
		t.Fatalf("the handler wrote itself: stdout %q, stderr %q", stdout, stderr)
	}
	if answer.Payload != nil || answer.Code != 2 || answer.Diagnostic != reason {
		t.Fatalf("handler answer = %+v, want nil payload, code 2, the reason", answer)
	}

	var served bool
	var text string
	var code int
	stdout, stderr = captureStdio(t, func() { text, code, served = ServeLocal(localDataArgsPath+" abcd", "") })
	if !served {
		t.Fatal("ServeLocal did not serve the registered command")
	}
	if code != 2 || text != "" {
		t.Errorf("ServeLocal = (%q, %d), want (\"\", 2)", text, code)
	}
	if stderr != reason || stdout != "" {
		t.Errorf("ServeLocal printed stdout %q, stderr %q; want stderr %q only", stdout, stderr, reason)
	}

	plain, args, err := LookupLocal(strings.Fields(localDataArgsPath+" abcd"), notDeclared)
	if err != nil || plain == nil {
		t.Fatalf("LookupLocal: handler %v, err %v", plain != nil, err)
	}
	stdout, stderr = captureStdio(t, func() { code = InvokeLocal(localDataArgsPath, plain, args) })
	if code != 2 {
		t.Errorf("plain route exit %d, want 2", code)
	}
	if stderr != reason || stdout != "" {
		t.Errorf("plain route printed stdout %q, stderr %q; want stderr %q only", stdout, stderr, reason)
	}
}

// TestLocalDataWarningPrintsBeforeThePayload proves a Diagnostic beside a
// payload is a warning: the route prints it on stderr and still answers the
// payload with the handler's code.
// Method: a handler answering a payload and one warning line, through
// ServeLocal with both streams captured.
func TestLocalDataWarningPrintsBeforeThePayload(t *testing.T) {
	const warning = "warning: line 3: deprecated keyword\n"
	withLocalDataArgs(t, func(ValidatedArgs) LocalAnswer {
		return LocalAnswer{Payload: map[string]any{"answer": "kept"}, Diagnostic: warning}
	})

	var served bool
	var text string
	var code int
	stdout, stderr := captureStdio(t, func() { text, code, served = ServeLocal(localDataArgsPath+" abcd | json", "") })
	if !served || code != 0 {
		t.Fatalf("ServeLocal served %v code %d, want true 0", served, code)
	}
	if !strings.Contains(text, `"answer"`) || !strings.Contains(text, `"kept"`) {
		t.Errorf("the payload was lost beside the warning: %q", text)
	}
	if stderr != warning || stdout != "" {
		t.Errorf("ServeLocal printed stdout %q, stderr %q; want stderr %q only", stdout, stderr, warning)
	}
}
