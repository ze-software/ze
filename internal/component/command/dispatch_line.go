// Design: docs/architecture/api/commands.md — the pipe chain: declared operations and the residual chain
// Related: pipe_partition.go — the one split of a chain between the command and the engine
// Related: invocation_scan.go — the one grammar the line is scanned with
//
// dispatch_line.go is the command package's entry point for the daemon
// dispatcher: a typed line read once, under the one grammar, into what
// accounting, authorization, the handler and the answer step each read.

package command

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DispatchLine is a typed line as the dispatcher reads it, scanned once.
//
// words is the command part decoded into words. chain is each operation of the
// pipe chain as typed, its decoded words, name first, before any alias is
// expanded: what the operator entered, for accounting and authorization. part
// is the partition of the expanded chain for the command part, and columns the
// command's column order for the answer step.
//
// A DispatchLine is read-only after ReadDispatchLine returns it, and safe for
// concurrent use.
type DispatchLine struct {
	words   []string
	chain   [][]string
	part    pipePartition
	columns []ColumnOrder
}

// ReadDispatchLine scans input once under the one grammar and partitions its
// pipe chain for the command part (partitionPipeChain). It refuses, with the
// column, an unfinished quote anywhere on the line and a backslash outside
// quotes in the command part (scanExecution). A fault of the chain itself (an
// unknown operation on a command that declares a set, a declared operation
// typed after a residual one, a bad argument) is no read error: the partition
// carries it in the residual chain, and ResidualRefusal reports it for the
// caller that applies that chain.
func ReadDispatchLine(input string) (DispatchLine, error) {
	scanned, chain, err := scanExecution(input)
	if err != nil {
		return DispatchLine{}, err
	}
	line := DispatchLine{words: scanned.tokens}
	commandPart := strings.TrimSpace(input)
	var ops []pipeOp
	if scanned.pipeAt >= 0 {
		commandPart = strings.TrimSpace(input[:scanned.pipeAt])
		line.chain = chain.operations
		ops = chainPipeOps(chain.operations)
	}
	commandPart, ops = expandPipeChain(commandPart, ops)
	line.columns = ColumnsForCommand(commandPart)
	line.part = partitionPipeChain(commandPart, ops)
	return line, nil
}

// TypedLineTokens scans input under the one grammar and answers its typed
// tokens (DispatchLine.TypedTokens), with no partition: the accountant reads
// the masked line this way, text the dispatcher never runs.
func TypedLineTokens(input string) ([]string, error) {
	scanned, chain, err := scanExecution(input)
	if err != nil {
		return nil, err
	}
	if scanned.pipeAt < 0 {
		return scanned.tokens, nil
	}
	return typedTokens(scanned.tokens, chain.operations), nil
}

// Command answers the command part as typed, trimmed and never rewritten.
func (l DispatchLine) Command() string {
	return l.part.command
}

// Words answers a copy of the command part's decoded words.
func (l DispatchLine) Words() []string {
	return append([]string(nil), l.words...)
}

// TypedTokens answers the line's tokens as the operator typed them: the
// command part's words, then each operation of the chain as a pipe token, its
// name and its argument words, decoded by the chain grammar, so a backslash
// in the chain stays a literal character (`| match a\b`). They are for
// accounting and authorization only, which judge what was entered; no handler
// receives a chain token.
func (l DispatchLine) TypedTokens() []string {
	return typedTokens(l.words, l.chain)
}

// typedTokens joins the command words and the chain operations into one
// token list, a pipe token before each operation. Its size is the sum of the
// words it copies, so nothing it reads is unbounded.
func typedTokens(words []string, chain [][]string) []string {
	size := len(words)
	for _, operation := range chain {
		size += 1 + len(operation)
	}
	tokens := make([]string, 0, size)
	tokens = append(tokens, words...)
	for _, operation := range chain {
		tokens = append(tokens, "|")
		tokens = append(tokens, operation...)
	}
	return tokens
}

// DeclaredOperations answers a copy of the operations the command declares,
// in chain order, which its handler receives.
func (l DispatchLine) DeclaredOperations() []PipeOperation {
	return append([]PipeOperation(nil), l.part.declared...)
}

// DeclaredLine answers the command part as typed, followed by the operations
// the command declares, in chain order, each written in the chain grammar
// (WriteInvocationValue quotes a word that would change its token or open an
// operation). It is the line a renderer in another process sends, which holds
// no plugin.CallerIdentity to say it renders: the daemon runs the declared
// operations in the handler and answers data, and the sender MUST apply the
// residual chain itself. A command that declares nothing answers its command
// part alone.
func (l DispatchLine) DeclaredLine() string {
	var tb textbuf.Buffer
	tb.Str(l.part.command)
	for _, operation := range l.part.declared {
		tb.Str(" | ").Str(operation.Name)
		for _, word := range operation.Args {
			tb.Byte(' ')
			WriteInvocationValue(&tb, word)
		}
	}
	return tb.String()
}

// HasAnswerStep reports whether the engine changes the answer of this line
// for a caller that does not render the chain itself: a residual operation to
// run, or the metadata of an operation that shaped the answer (a declared one
// included) to inject. A line with no chain has neither, so its answer is the
// handler's, unchanged.
func (l DispatchLine) HasAnswerStep() bool {
	return len(l.part.residual) > 0 || len(l.part.meta) > 0
}

// ResidualRefusal answers why the engine cannot apply the residual chain for
// a caller that receives one answer, or nil. Only the caller that applies the
// chain asks: a surface that renders the chain itself validates it on its own
// surface. `| save` is refused here, because the daemon never writes a
// caller's file, and so is every fault the partition carried.
func (l DispatchLine) ResidualRefusal() error {
	msg := validatePipesForSurface(l.part.command, l.part.residual, pipeSurfaceOneShot, false)
	if msg == "" {
		return nil
	}
	return errors.New(msg)
}

// ApplyAnswerStep runs the residual chain over document, the answer's JSON,
// with the chain's metadata, as the operator surfaces run it
// (docs/architecture/api/commands.md). rendered is the result; text is true
// when the chain holds a format that prints text (`| ndjson`, `| table`,
// `| text`, `| yaml`), so rendered is that text rather than a JSON document.
// `| json` and `| raw` print a JSON document, so text is false for them.
// failure is the chain's refusal of this answer, or empty.
//
// It MUST be called only after ResidualRefusal answered nil.
func (l DispatchLine) ApplyAnswerStep(document string) (rendered string, text bool, failure string) {
	if len(l.part.residual) == 0 {
		rendered, failure = injectPipeMeta(document, l.part.meta)
		return rendered, false, failure
	}
	rendered, failure = ApplyPipes(document, l.part.residual, l.part.meta, l.columns)
	return rendered, hasTextFormatOp(l.part.residual), failure
}
