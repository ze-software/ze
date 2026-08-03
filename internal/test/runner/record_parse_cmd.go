// Design: docs/architecture/testing/ci-format.md -- cmd=background/foreground/stop directive parsing

package runner

import (
	"fmt"
	"strconv"
	"strings"
)

// markerSeq is the shared ":seq=" field marker used by the cmd= directive parsers.
const markerSeq = ":seq="

// parseCmdExec extracts fields from a cmd=background/foreground line using
// marker-based parsing. This handles exec= values containing colons correctly.
//
// Format: cmd=background:seq=N:exec=COMMAND[:stdin=BLOCK][:timeout=DUR][:exit=N][:name=NAME].
// name= assigns a handle a later cmd=stop directive can reference (see parseCmdStop).
func parseCmdExec(mode, line string) (RunCommand, error) {
	seqMarker := markerSeq
	execMarker := ":exec="
	stdinMarker := ":stdin="
	timeoutMarker := ":timeout="
	exitMarker := ":exit="
	nameMarker := ":name="

	seqIdx := strings.Index(line, seqMarker)
	execIdx := strings.Index(line, execMarker)

	if seqIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:%s missing seq=", mode)
	}
	if execIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:%s missing exec=", mode)
	}

	// Extract seq value: from after ":seq=" to the next known marker or end.
	seqStart := seqIdx + len(seqMarker)
	seqEnd := nextMarker(line, seqStart, execMarker, stdinMarker, timeoutMarker, exitMarker, nameMarker)
	seqStr := line[seqStart:seqEnd]
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return RunCommand{}, fmt.Errorf("cmd:%s invalid seq=%q", mode, seqStr)
	}

	// Extract exec value: from after ":exec=" to the next known marker or end.
	// This correctly preserves colons inside the exec value.
	execStart := execIdx + len(execMarker)
	execEnd := nextMarker(line, execStart, stdinMarker, timeoutMarker, exitMarker, nameMarker)
	execVal := line[execStart:execEnd]
	if execVal == "" {
		return RunCommand{}, fmt.Errorf("cmd:%s missing exec=", mode)
	}

	rc := RunCommand{
		Mode: mode,
		Seq:  seq,
		Exec: execVal,
	}

	// Extract optional stdin=, timeout=, name= and exit= values.
	if idx := strings.Index(line, stdinMarker); idx >= 0 {
		start := idx + len(stdinMarker)
		end := nextMarker(line, start, timeoutMarker, exitMarker, nameMarker)
		rc.Stdin = line[start:end]
	}
	if idx := strings.Index(line, timeoutMarker); idx >= 0 {
		start := idx + len(timeoutMarker)
		end := nextMarker(line, start, exitMarker, nameMarker)
		rc.Timeout = line[start:end]
	}
	if idx := strings.Index(line, nameMarker); idx >= 0 {
		start := idx + len(nameMarker)
		// Bound against every OTHER marker (execMarker included) so name= is
		// order-independent: a line that writes name= before exec= must not let
		// the name value swallow ":exec=...".
		end := nextMarker(line, start, execMarker, stdinMarker, timeoutMarker, exitMarker)
		rc.Name = line[start:end]
	}
	if idx := strings.Index(line, exitMarker); idx >= 0 {
		start := idx + len(exitMarker)
		end := nextMarker(line, start, stdinMarker, timeoutMarker, nameMarker)
		codeStr := line[start:end]
		code, err := strconv.Atoi(codeStr)
		if err != nil || code < 0 || code > 255 {
			return RunCommand{}, fmt.Errorf("cmd:%s invalid exit=%q (want 0..255)", mode, codeStr)
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
	seqMarker := markerSeq
	nameMarker := ":name="
	signalMarker := ":signal="

	seqIdx := strings.Index(line, seqMarker)
	if seqIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:stop missing seq=")
	}
	nameIdx := strings.Index(line, nameMarker)
	if nameIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:stop missing name=")
	}

	seqStart := seqIdx + len(seqMarker)
	seqEnd := nextMarker(line, seqStart, nameMarker, signalMarker)
	seqStr := line[seqStart:seqEnd]
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return RunCommand{}, fmt.Errorf("cmd:stop invalid seq=%q", seqStr)
	}

	nameStart := nameIdx + len(nameMarker)
	nameEnd := nextMarker(line, nameStart, seqMarker, signalMarker)
	name := line[nameStart:nameEnd]
	if name == "" {
		return RunCommand{}, fmt.Errorf("cmd:stop missing name=")
	}

	rc := RunCommand{
		Mode:   modeStop,
		Seq:    seq,
		Name:   name,
		Signal: signalKill,
	}

	if idx := strings.Index(line, signalMarker); idx >= 0 {
		start := idx + len(signalMarker)
		end := nextMarker(line, start, seqMarker, nameMarker)
		sig := line[start:end]
		if sig != signalKill && sig != signalTerm {
			return RunCommand{}, fmt.Errorf("cmd:stop invalid signal=%q (want %q or %q)", sig, signalKill, signalTerm)
		}
		rc.Signal = sig
	}

	return rc, nil
}
