// Design: docs/architecture/api/commands.md — the pipe chain on the stream route
// Related: dispatch_line.go — the one-shot route's reading of the same line
// Related: pipe_partition.go — the one split of a chain between the command and the engine
// Related: pipe_stream.go — runStreamEvents, which judges and formats each event
//
// stream_line.go is the command package's entry point for the stream route: a
// monitor line read once, under the one grammar, into the command part its
// handler is looked up by, the operations the command declares, and the part
// of the residual chain the daemon applies to every event.

package command

import (
	"io"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

// StreamRendering says who presents the events of a stream, and so which part
// of the residual chain the daemon applies. The zero value is no answer: a
// caller states one of the named values.
type StreamRendering uint8

// The renderings a stream caller states.
const (
	StreamRenderingUnspecified StreamRendering = iota
	// StreamRenderingEngine: the daemon applies the whole residual chain, in
	// the configured default format when the chain names none. A line with no
	// chain passes the handler's events unchanged. The web terminal, the REST
	// and gRPC streams and the daemon-hosted model state it.
	StreamRenderingEngine
	// StreamRenderingTerminal: as StreamRenderingEngine, and a line with no
	// chain is rendered in the configured default format too, because the
	// caller writes the events to a terminal. SSH exec states it.
	StreamRenderingTerminal
	// StreamRenderingClientPresentation: a client in another process said it
	// presents the events (the SSH session key sshclient.EnvCLIRender). The
	// daemon still applies every operation before the presentation suffix,
	// a selecting or reducing one included, and answers data; the client
	// applies the suffix alone (PresentStreamSuffix). The key hands over
	// presentation, never selection.
	StreamRenderingClientPresentation
)

// StreamLine is a monitor line as the stream route reads it, scanned once.
//
// words is the command part decoded into words. part is the partition of the
// expanded chain for the command part. applied is what the daemon runs over
// each event: the residual chain, less the presentation suffix a client
// presents, with the default format appended where the rendering asks for
// one. An empty applied passes the events unchanged.
//
// A StreamLine is read-only after ReadStreamLine returns it, and safe for
// concurrent use.
type StreamLine struct {
	words   []string
	part    pipePartition
	columns []ColumnOrder
	applied []pipeOp
}

// ReadStreamLine scans input once under the one grammar, partitions its pipe
// chain for the command part (partitionPipeChain) and validates the part the
// daemon applies for the stream surface. It refuses, with the column, an
// unfinished quote anywhere on the line, and refuses a chain fault by name:
// an unknown or late operation, a bad argument, and `| save` outside the
// suffix a client presents, because the daemon never writes a caller's file.
func ReadStreamLine(input string, rendering StreamRendering) (StreamLine, error) {
	scanned, chain, err := scanExecution(input)
	if err != nil {
		return StreamLine{}, err
	}
	commandPart := strings.TrimSpace(input)
	var typed [][]string
	var ops []pipeOp
	if scanned.pipeAt >= 0 {
		commandPart = strings.TrimSpace(input[:scanned.pipeAt])
		typed = chain.operations
		ops = chainPipeOps(typed)
	}
	commandPart, ops = expandPipeChain(commandPart, ops)
	line := StreamLine{
		words:   scanned.tokens,
		columns: ColumnsForCommand(commandPart),
		part:    partitionPipeChain(commandPart, ops),
	}
	applied, err := streamAppliedOperations(line.part, typed, rendering)
	if err != nil {
		return StreamLine{}, err
	}
	line.applied = applied
	return line, nil
}

// streamAppliedOperations answers the operations the daemon runs over each
// event of a stream typed with the chain typed, for the rendering stated.
func streamAppliedOperations(part pipePartition, typed [][]string, rendering StreamRendering) ([]pipeOp, error) {
	residual := part.residual
	switch rendering {
	case StreamRenderingClientPresentation:
		residual = withoutPresentationSuffix(residual, presentationSuffixLength(typed))
	case StreamRenderingEngine, StreamRenderingTerminal:
	case StreamRenderingUnspecified:
		panic("BUG: ReadStreamLine called without a rendering")
	}
	if msg := validatePipesForSurface(part.command, residual, pipeSurfaceStream, false); msg != "" {
		return nil, opererr.New("the live monitor pipe chain was refused", msg,
			"remove the refused pipe and repeat the monitor command")
	}
	// `| log` is a display mode of the caller's stream, never an operation
	// on one event, so it leaves the chain here as it does on the local route.
	applied := make([]pipeOp, 0, len(residual)+1)
	for _, op := range residual {
		if op.kind != pipeLog {
			applied = append(applied, op)
		}
	}
	if hasFormatOp(applied) {
		return applied, nil
	}
	switch rendering {
	case StreamRenderingClientPresentation:
		// The client formats: it applies the suffix's format, or its default.
		return applied, nil
	case StreamRenderingEngine:
		if len(typed) == 0 {
			return nil, nil
		}
	case StreamRenderingTerminal:
	case StreamRenderingUnspecified:
		panic("BUG: ReadStreamLine called without a rendering")
	}
	return append(applied, pipeOp{kind: configuredDefault("")}), nil
}

// presentationSuffixLength answers how many operations end typed with a
// presentation operator of the catalog (PipeOperator.Presentation): the
// formats, paging and `save`. It reads the names as typed, before any alias
// is expanded, so an alias ends the suffix and the daemon applies its whole
// expansion, a format in it included. A name the catalog does not hold is no
// presentation operator, so a client that never heard of a command's own
// operation leaves it to the daemon and refuses nothing. Both the daemon and
// the client read the suffix through this one function.
func presentationSuffixLength(typed [][]string) int {
	length := 0
	for _, operation := range slices.Backward(typed) {
		entry, known := lookupPipeOperatorByName(operation[0])
		if !known {
			break
		}
		if !entry.Presentation {
			break
		}
		length++
	}
	return length
}

// withoutPresentationSuffix answers residual less its last length operations,
// the presentation suffix a client presents. The suffix holds catalog
// operators only, which neither an alias expansion nor the partition moves,
// so they are the last operations of the residual chain. A residual the
// partition replaced by one refusal is answered whole, so the refusal stands.
func withoutPresentationSuffix(residual []pipeOp, length int) []pipeOp {
	if length > len(residual) {
		return residual
	}
	tail := residual[len(residual)-length:]
	for _, op := range tail {
		entry, known := lookupPipeOperatorByKind(op.kind)
		if !known {
			return residual
		}
		if !entry.Presentation {
			return residual
		}
	}
	return residual[:len(residual)-length]
}

// Command answers the command part as typed, trimmed and never rewritten.
func (l StreamLine) Command() string {
	return l.part.command
}

// Words answers a copy of the command part's decoded words.
func (l StreamLine) Words() []string {
	return append([]string(nil), l.words...)
}

// DeclaredOperations answers a copy of the operations the command declares,
// in chain order, which its streaming handler receives.
func (l StreamLine) DeclaredOperations() []PipeOperation {
	return append([]PipeOperation(nil), l.part.declared...)
}

// Run runs produce, the streaming handler, writing to output through the
// operations the daemon applies to each event (RunStreamPipes), or straight
// to output when it applies none. produce MUST stop writing before it
// returns and MUST propagate writer failures (RunStreamPipes).
//
// When the last operation the command declares on this line is an override,
// each event the handler writes is first held to the shape of the generic it
// overrides (overrideEventJudge): an event of another shape is not written,
// and its refusal stops the stream and is the error answered. Every stream
// site runs the handler through this method, so the check has this one place.
// Such a stream is read event by event even when the daemon applies no
// operation to it, so a blank line is dropped and the last event ends in a
// newline.
func (l StreamLine) Run(output io.Writer, produce func(io.Writer) error) error {
	judge, held := overrideEventJudge(l.part.command, l.part.declared)
	if len(l.applied) == 0 {
		if !held {
			return produce(output)
		}
		return runStreamEvents(output, judge, func(event string) string { return event }, produce)
	}
	ops, meta, columns := l.applied, l.part.meta, l.columns
	return runStreamEvents(output, judge, func(rawJSON string) string {
		result, pipeErr := applyStreamEventPipes(rawJSON, ops, meta, columns)
		if pipeErr != "" {
			return pipeError(pipeErr)
		}
		return result
	}, produce)
}

// PresentStreamSuffix answers what a client in another process applies to the
// events of a stream it asked the daemon for under
// StreamRenderingClientPresentation: the presentation suffix of the line it
// typed (presentationSuffixLength), and nothing else. The format is the
// suffix's own; with none, it is defaultFn unless the chain typed names a
// format before the suffix, which the daemon already rendered. The client
// reads the catalog alone, never a registry of the daemon's: the daemon
// classifies, the client presents.
//
// The returned StreamSaves owns any `| save` temporary files. The caller MUST
// write exactly the displayed bytes to it, then call Commit after stream
// success or Abort after any failure.
func PresentStreamSuffix(input string, defaultFn func(string) string) (format func(string) string, saves *StreamSaves, errMsg string) {
	scanned, chain, err := scanExecution(input)
	if err != nil {
		return nil, nil, err.Error()
	}
	commandPart := strings.TrimSpace(input)
	var typed [][]string
	if scanned.pipeAt >= 0 {
		commandPart = strings.TrimSpace(input[:scanned.pipeAt])
		typed = chain.operations
	}
	start := len(typed) - presentationSuffixLength(typed)
	suffix := chainPipeOps(typed[start:])
	if msg := validatePipesForSurface(commandPart, suffix, pipeSurfaceStream, true); msg != "" {
		return nil, nil, msg
	}
	saves, errMsg = openStreamSaves(savePathsInChain(suffix))
	if errMsg != "" {
		return nil, nil, errMsg
	}
	presented := make([]pipeOp, 0, len(suffix))
	for _, op := range suffix {
		if op.kind != pipeSave {
			presented = append(presented, op)
		}
	}
	if hasFormatOp(presented) {
		columns := ColumnsForCommand(commandPart)
		return func(rawJSON string) string {
			result, pipeErr := ApplyPipes(rawJSON, presented, nil, columns)
			if pipeErr != "" {
				return pipeError(pipeErr)
			}
			return result
		}, saves, ""
	}
	if hasFormatOp(chainPipeOps(typed[:start])) {
		return func(rendered string) string { return rendered }, saves, ""
	}
	return defaultFn, saves, ""
}
