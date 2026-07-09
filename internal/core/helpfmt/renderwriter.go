// Design: docs/architecture/core-design.md -- CLI render write-error capture
//
// RenderWriter is the shared error-capturing writer for one-shot CLI render
// paths (help pages, command catalogs, the AI reference). Historically these
// paths used fmt.Println / fmt.Fprintf and discarded the write error, so a
// broken pipe (`ze help | head`) or a full disk produced a silently truncated
// page with a zero exit code. RenderWriter records the first write error; the
// command checks it once at the end (ExitCode) and exits non-zero.

package helpfmt

import "io"

// RenderWriter wraps an io.Writer and records the first write error. Its Str /
// Line helpers return nothing, so a render loop stays free of per-line error
// checks (and of the banned fmt.Fprintf-to-custom-writer primitive) while still
// honoring the non-zero-exit-on-write-error contract via Err / ExitCode.
type RenderWriter struct {
	w   io.Writer
	err error
}

// NewRenderWriter returns a RenderWriter over w.
func NewRenderWriter(w io.Writer) *RenderWriter { return &RenderWriter{w: w} }

// Write forwards to the underlying writer and records the first error. Once an
// error is recorded every later Write is a no-op returning that error, so a
// caller looping over a broken pipe stops issuing syscalls. Implements
// io.Writer so fmt.Fprint/json.Encoder can target it.
func (rw *RenderWriter) Write(p []byte) (int, error) {
	if rw.err != nil {
		return 0, rw.err
	}
	n, err := rw.w.Write(p)
	if err != nil {
		rw.err = err
	}
	return n, err
}

// WriteString implements io.StringWriter so io.WriteString avoids a []byte copy
// and the error is captured the same way as Write.
func (rw *RenderWriter) WriteString(s string) (int, error) {
	if rw.err != nil {
		return 0, rw.err
	}
	n, err := io.WriteString(rw.w, s)
	if err != nil {
		rw.err = err
	}
	return n, err
}

// Str writes s, recording the first write error. No return value: the render
// loop checks Err/ExitCode once at the end.
func (rw *RenderWriter) Str(s string) {
	if rw.err != nil {
		return
	}
	if _, err := io.WriteString(rw.w, s); err != nil {
		rw.err = err
	}
}

// Line writes s followed by a newline (matching fmt.Println's byte output).
func (rw *RenderWriter) Line(s string) {
	rw.Str(s)
	rw.Str("\n")
}

// Err returns the first write error, or nil if every write succeeded.
func (rw *RenderWriter) Err() error { return rw.err }

// ExitCode returns 1 if a write error was recorded, else 0. CLI handlers return
// it so a truncated render surfaces as a non-zero exit.
func (rw *RenderWriter) ExitCode() int {
	if rw.err != nil {
		return 1
	}
	return 0
}
