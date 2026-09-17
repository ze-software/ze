// Design: docs/architecture/core-design.md — CLI session transcript recording
// Related: model.go — Model struct, SetCommandExecutor wrapping point

package cli

import (
	"fmt"
	"os"
	"path/filepath"
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

// transcriptEnabled returns true if the ze.cli.transcript env var is set to a truthy value.
func transcriptEnabled() bool {
	v := env.Get("ze.cli.transcript")
	return v == boolTrue || v == "1" || v == "yes" || v == "enabled"
}

// OpenTranscriptFile creates `$XDG_DATA_HOME/ze/transcripts/` and one file in
// it, `transcript-<stamp>-<tag>.log`, when transcript recording is enabled.
// The tag tells two sessions of the same second apart: a client passes its
// pid, the daemon passes its pid and the session's sequence number, never the
// user name, which RADIUS and TACACS+ hand back unsanitized. It returns
// nil when recording is disabled or the file cannot be created, and a
// warning on stderr names the failure. The caller closes the file.
func OpenTranscriptFile(tag string) *os.File {
	if !transcriptEnabled() {
		return nil
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: transcript: %v\n", err)
			return nil
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(dataHome, "ze", "transcripts")
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		fmt.Fprintf(os.Stderr, "warning: transcript directory: %v\n", mkErr)
		return nil
	}

	path := filepath.Join(dir, "transcript-"+time.Now().Format("20060102-150405")+"-"+tag+".log")
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // the path is the data home plus a constant subpath and a caller tag
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "warning: transcript file: %v\n", openErr)
		return nil
	}
	return f
}

// WrapExecutorWithTranscript wraps a command executor so that every command
// and its response are recorded. Completion ownership remains on the returned
// CommandOutput for the UI writer.
func WrapExecutorWithTranscript(fn CommandExecutor, tw *TranscriptWriter) CommandExecutor {
	// A nil executor stays nil: the model's "no daemon connection" guard reads
	// nil, and a closure over a nil fn would panic on the first command.
	if fn == nil || tw == nil {
		return fn
	}
	return func(input string) (CommandOutput, error) {
		output, err := fn(input)
		tw.Record(input, output.Text)
		return output, err
	}
}
