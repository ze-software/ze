// Design: docs/architecture/testing/ci-format.md -- per-step trace output

package trace

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// StepResult records the outcome of a single test step.
type StepResult struct {
	Step   int
	Line   int
	Kind   string
	Assert string
	Passed bool
	Detail string
}

type stepJSON struct {
	File   string `json:"file"`
	Step   int    `json:"step"`
	Line   int    `json:"line,omitempty"`
	Kind   string `json:"kind"`
	Assert string `json:"assert,omitempty"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// PrintTrace writes per-step human and machine trace lines.
func PrintTrace(w io.Writer, file string, steps []StepResult, colorEnabled bool) {
	for _, s := range steps {
		writeHuman(w, s, colorEnabled)
		writeMachine(w, file, s)
	}
}

func writeHuman(w io.Writer, s StepResult, colorEnabled bool) {
	c := textbuf.C
	loc := s.Step
	if s.Line > 0 {
		loc = s.Line
	}

	var b textbuf.Buffer
	b.SetColor(colorEnabled)
	b.Str("  ")
	padNum(&b, loc)
	b.Byte(' ')

	if s.Passed {
		b.Colored(c.BrightGreen).Str("✓").Colored(c.Reset)
	} else {
		b.Colored(c.BoldRed).Str("✗").Colored(c.Reset)
	}

	b.Byte(' ').Str(s.Kind)
	if s.Assert != "" {
		b.Byte(' ').Str(s.Assert)
	}
	if !s.Passed {
		detail := s.Detail
		if detail == "" {
			detail = "FAILED"
		}
		b.Str(" -> ").Str(detail)
	}
	b.Byte('\n')
	io.WriteString(w, b.Slice()) //nolint:errcheck // terminal output
}

func writeMachine(w io.Writer, file string, s StepResult) {
	status := "pass"
	if !s.Passed {
		status = "fail"
	}
	j := stepJSON{
		File:   file,
		Step:   s.Step,
		Line:   s.Line,
		Kind:   s.Kind,
		Assert: s.Assert,
		Status: status,
		Detail: s.Detail,
	}
	data, _ := json.Marshal(j)
	io.WriteString(w, "VERIFY STEP: ") //nolint:errcheck // terminal output
	w.Write(data)                      //nolint:errcheck // terminal output
	io.WriteString(w, "\n")            //nolint:errcheck // terminal output
}

// ErrString returns err.Error() or "" if err is nil.
func ErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func padNum(b *textbuf.Buffer, n int) {
	s := strconv.Itoa(n)
	for range 3 - len(s) {
		b.Byte(' ')
	}
	b.Str(s)
}
