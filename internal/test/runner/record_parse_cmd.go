// Design: docs/architecture/testing/ci-format.md -- cmd=background/foreground/stop directive parsing

package runner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The key vocabulary of a cmd= line. Each key is spelled once, as the marker
// the parser searches for, and the two lists below are what the runner accepts.
const (
	markerSeq     = ":seq="
	markerExec    = ":exec="
	markerStdin   = ":stdin="
	markerTimeout = ":timeout="
	markerExit    = ":exit="
	markerName    = ":name="
	markerSignal  = ":signal="
)

// cmdExecKeys and cmdStopKeys are the keys each cmd= parser reads, declared
// once. Every value below is bounded by this list, and checkMarkerKeys refuses
// a key that is not in it, so what the runner accepts and what the parser reads
// are one declaration rather than two (ai/rules/principles.md).
var (
	cmdExecKeys = []string{markerSeq, markerExec, markerStdin, markerTimeout, markerExit, markerName}
	cmdStopKeys = []string{markerSeq, markerName, markerSignal}
)

// markerValue returns the value marker introduces in line, and reports whether
// the line carries the marker at all. The value ends at the next occurrence of
// ANY key, marker included, so a key written out of canonical order is never
// swallowed into the value before it, and a repeated key ends its own value.
func markerValue(line, marker string, keys []string) (string, bool) {
	idx := strings.Index(line, marker)
	if idx < 0 {
		return "", false
	}
	start := idx + len(marker)
	return line[start:nextMarker(line, start, keys...)], true
}

// parseCmdExec extracts fields from a cmd=background/foreground line using
// marker-based parsing. This handles exec= values containing colons correctly.
//
// Format: cmd=background:seq=N:exec=COMMAND[:stdin=BLOCK][:timeout=DUR][:exit=N][:name=NAME].
// name= assigns a handle a later cmd=stop directive can reference (see parseCmdStop).
func parseCmdExec(mode, line string) (RunCommand, error) {
	directive := cmdDirective(mode)
	if err := checkMarkerKeys(directive, line, cmdExecKeys); err != nil {
		return RunCommand{}, err
	}

	seqStr, ok := markerValue(line, markerSeq, cmdExecKeys)
	if !ok {
		return RunCommand{}, fmt.Errorf("%s missing seq=", directive)
	}
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return RunCommand{}, fmt.Errorf("%s invalid seq=%q", directive, seqStr)
	}

	execVal, ok := markerValue(line, markerExec, cmdExecKeys)
	if !ok || execVal == "" {
		return RunCommand{}, fmt.Errorf("%s missing exec=", directive)
	}

	rc := RunCommand{
		Mode: mode,
		Seq:  seq,
		Exec: execVal,
	}

	rc.Stdin, _ = markerValue(line, markerStdin, cmdExecKeys)
	rc.Name, _ = markerValue(line, markerName, cmdExecKeys)

	// Both readers of Timeout discard a bad duration in silence
	// (resolveOrchestratedTimeout takes the test budget from a foreground line,
	// startBackgroundLifetime takes a background line's lifetime), so the value
	// is judged HERE, where the author can be told. Every one of the 1,106
	// timeout= values in the corpus parses; the one that did not was a swallowed
	// key, which the check above now refuses.
	if timeout, ok := markerValue(line, markerTimeout, cmdExecKeys); ok {
		if _, err := time.ParseDuration(timeout); err != nil {
			return RunCommand{}, fmt.Errorf("%s invalid timeout=%q: %w", directive, timeout, err)
		}
		rc.Timeout = timeout
	}

	if codeStr, ok := markerValue(line, markerExit, cmdExecKeys); ok {
		code, err := strconv.Atoi(codeStr)
		if err != nil || code < 0 || code > 255 {
			return RunCommand{}, fmt.Errorf("%s invalid exit=%q (want 0..255)", directive, codeStr)
		}
		rc.ExitCode = &code
	}

	return rc, nil
}

// parseCmdStop extracts fields from a cmd=stop line using marker-based parsing,
// consistent with parseCmdExec.
//
// Format: cmd=stop:seq=N:name=NAME[:signal=kill|term].
//
// name= is REQUIRED and must match the name= a prior cmd=background line assigned;
// the step executor fails the test if it names no tracked background process
// (fail-closed, ai/rules/evidence.md). signal= defaults to "kill"
// (SIGKILL) so the target goes silent for the DPD proof; "term" sends SIGTERM.
func parseCmdStop(line string) (RunCommand, error) {
	directive := cmdDirective(modeStop)
	if err := checkMarkerKeys(directive, line, cmdStopKeys); err != nil {
		return RunCommand{}, err
	}

	seqStr, ok := markerValue(line, markerSeq, cmdStopKeys)
	if !ok {
		return RunCommand{}, fmt.Errorf("%s missing seq=", directive)
	}
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return RunCommand{}, fmt.Errorf("%s invalid seq=%q", directive, seqStr)
	}

	name, ok := markerValue(line, markerName, cmdStopKeys)
	if !ok || name == "" {
		return RunCommand{}, fmt.Errorf("%s missing name=", directive)
	}

	rc := RunCommand{
		Mode:   modeStop,
		Seq:    seq,
		Name:   name,
		Signal: signalKill,
	}

	if sig, ok := markerValue(line, markerSignal, cmdStopKeys); ok {
		if sig != signalKill && sig != signalTerm {
			return RunCommand{}, fmt.Errorf("%s invalid signal=%q (want %q or %q)", directive, sig, signalKill, signalTerm)
		}
		rc.Signal = sig
	}

	return rc, nil
}

// cmdDirective names the directive an author wrote, for a message that quotes
// their spelling rather than the parser's mode word.
func cmdDirective(mode string) string {
	var b textbuf.Buffer
	return b.Str("cmd=").Str(mode).String()
}
