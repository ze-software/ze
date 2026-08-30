// Design: docs/architecture/core-design.md — CLI session transcript recording
// Related: model.go — Model struct, SetCommandExecutor wrapping point

package cli

import (
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.cli.transcript",
	Type:        "bool",
	Default:     boolFalse,
	Description: "Enable CLI session transcript recording",
})

// TranscriptWriter records CLI commands and their output to a local file.
// A nil *TranscriptWriter is a valid no-op receiver.
type TranscriptWriter struct {
	file *os.File
}

// NewTranscriptWriter creates a TranscriptWriter that writes to the given file.
// Writes a header with session metadata. The caller is responsible for creating
// the file and directory. Returns nil if f is nil.
func NewTranscriptWriter(f *os.File, username, remoteHost string) *TranscriptWriter {
	if f == nil {
		return nil
	}
	w := &TranscriptWriter{file: f}
	w.writeHeader(time.Now(), username, remoteHost)
	return w
}

func (w *TranscriptWriter) writeHeader(t time.Time, username, remoteHost string) {
	buf := textbuf.Get()
	defer buf.Release()
	buf.Str("# Ze CLI Transcript\n").
		Str("# Started: ").Str(t.Format(time.RFC3339)).Byte('\n').
		Str("# User: ").Str(username).Byte('\n').
		Str("# Host: ").Str(remoteHost).Byte('\n').
		Str("#\n\n")
	w.file.WriteString(buf.String()) //nolint:errcheck // best-effort transcript
}

// Record appends a command and its output to the transcript file.
// Errors are silently ignored (best-effort).
//
// The output is whatever the caller passes, and the two callers differ. A `-c`
// run records what the operator saw: the daemon renders the answer in the
// configured format before it reaches the client (internal/component/ssh/ssh.go,
// execMiddleware). An interactive session records the dispatcher's JSON, because
// WrapExecutorWithTranscript sits under the Model and the Model renders after
// the executor returns (model_mode.go, executeOperationalCommand). The command
// is recorded with its pipe operators either way, so the two lines together say
// what was asked and what came back.
func (w *TranscriptWriter) Record(command, output string) {
	if w == nil || w.file == nil {
		return
	}
	buf := textbuf.Get()
	defer buf.Release()
	buf.Byte('[').Str(time.Now().Format("15:04:05")).Str("] > ").Str(command).Byte('\n')
	if output != "" {
		buf.Str(output).Byte('\n')
	}
	buf.Byte('\n')
	w.file.WriteString(buf.String()) //nolint:errcheck // best-effort transcript
}

// Close closes the transcript file.
func (w *TranscriptWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	return w.file.Close()
}

// TranscriptEnabled returns true if the ze.cli.transcript env var is set to a truthy value.
func TranscriptEnabled() bool {
	v := env.Get("ze.cli.transcript")
	return v == boolTrue || v == "1" || v == "yes" || v == "enabled"
}

// WrapExecutorWithTranscript wraps a command executor so that every command
// and its response are recorded. Completion ownership remains on the returned
// CommandOutput for the UI writer.
func WrapExecutorWithTranscript(fn CommandExecutor, tw *TranscriptWriter) CommandExecutor {
	if tw == nil {
		return fn
	}
	return func(input string) (CommandOutput, error) {
		output, err := fn(input)
		tw.Record(input, output.Text)
		return output, err
	}
}
