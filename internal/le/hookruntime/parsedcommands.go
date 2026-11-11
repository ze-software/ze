// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: rawheavy.go -- treeRawHeavy, the raw-job guard's judgment of every parsed command
// Related: lossypipe.go -- treeLossyPipe, the piped-command guard's judgment of every parsed pipeline
// Related: readerline.go -- the reader-line walk, which judges what this file only reads
package hookruntime

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// parsedCommand is one simple command of any parsed line, reader or not: the
// unit the tree judgments of the raw-job and piped-command guards read
// (treeRawHeavy, treeLossyPipe), and the command their refusals advise.
type parsedCommand struct {
	// words holds each argument's value; readable reports that every
	// argument and assignment has one the hook can read and none expands,
	// so the words, each written back with quotedWord, are the command.
	words    []string
	readable bool
	// assigns holds the assignments in front, as typed.
	assigns []string
	// tokens holds the command as the word judgments read the words of a
	// text segment (rawHeavyWords, expensiveWords): one token per assignment
	// node in front, `name=value`, then one per argument. A value the hook
	// reads exactly (readWord, nothing expands) is that value; any other is
	// its typed text read the way the text judgment reads a segment
	// (typedToken), so the tree judgment reads every command at least as the
	// text judgment reads the same words. tokens[len(assigns)] is the command
	// word.
	tokens []string
	// written holds each token as a refusal writes it back, one shell word
	// each (admittedCommand): a value the hook reads exactly through
	// quotedWord, and any other word by its own source text as typed, quotes
	// included, so the shell expands it when the advice runs, as it did when
	// the agent's line ran. A word's expanded value is never written: the
	// hook does not know it, and the source text adds nothing the agent did
	// not type. written[i] is tokens[i] written back.
	written []string
	// source is the command's own text from its first assignment or word
	// to its last word, as typed, with no redirection.
	source string
	// stages holds every pipeline stage the command runs in (stageCommands):
	// none outside a pipeline, two for a command in a pipeline nested inside
	// a stage of another.
	stages []pipelineStage
}

// pipelineStage is one place in one pipeline of a line: the pipeline's
// number on the line, and the stage's place in it from 0.
type pipelineStage struct {
	pipeline int
	stage    int
}

// segment answers the command as a refusal advises it: its assignments as
// typed and its words each written back as one shell word when every word is
// readable, and its own source text otherwise. A redirection is never part
// of it, so `2>&1` is never read as a package path.
func (command parsedCommand) segment() string {
	if !command.readable {
		return command.source
	}
	words := make([]string, 0, len(command.assigns)+len(command.words))
	words = append(words, command.assigns...)
	for _, word := range command.words {
		words = append(words, quotedWord(word))
	}
	return strings.Join(words, " ")
}

// commandWord answers the token of the command word, the first argument.
// parsedCall builds a command only from a call with an argument, so it
// always exists.
func (command parsedCommand) commandWord() string {
	return command.tokens[len(command.assigns)]
}

// feeds reports whether command runs in an earlier stage of a pipeline that
// later runs in too, so that later reads what command prints.
func (command parsedCommand) feeds(later parsedCommand) bool {
	for _, own := range command.stages {
		for _, other := range later.stages {
			if own.pipeline != other.pipeline {
				continue
			}
			if own.stage < other.stage {
				return true
			}
		}
	}
	return false
}

// everyCommand answers every simple command of a parsed line at any depth,
// in source order, with the pipeline stages each runs in. It judges nothing:
// it is the reading the tree judgments and their advice are written from, so
// a refusal names the command the shell runs rather than a text segment cut
// at a quoted bar (AC-15). Only a call node is a command: a heavy job written
// as an argument of another command (`./le job run ... command go test ./x`)
// is that command's word, and a string handed to a shell is never opened. A
// command inside a command substitution, a process substitution or an
// assignment's value is a call node of its own, so it is found.
func everyCommand(source string, statements []*syntax.Stmt) []parsedCommand {
	walk := readerWalk{source: source}
	stages := pipelineMembership(statements)
	commands := make([]parsedCommand, 0, 8)
	for _, statement := range statements {
		syntax.Walk(statement, func(node syntax.Node) bool {
			call, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}
			if len(call.Args) == 0 {
				// Bare assignments run no command; the walk goes on into
				// their values, where a substitution runs one.
				return true
			}
			command := walk.parsedCall(call)
			command.stages = stages[call]
			commands = append(commands, command)
			return true
		})
	}
	return commands
}

// pipelineMembership answers the pipeline stages every simple command of a
// line runs in. Each chain of `|` and `|&` is one pipeline, numbered in
// source order, and each leaf of the chain is a stage (pipelineLeaves).
// Every command a stage holds runs in it (stageCommands), so a command
// inside a group, a subshell, a loop or a conditional that is a stage reads
// from the stage before it and prints to the stage after it.
func pipelineMembership(statements []*syntax.Stmt) map[*syntax.CallExpr][]pipelineStage {
	members := make(map[*syntax.CallExpr][]pipelineStage)
	inner := make(map[*syntax.Stmt]bool)
	pipelines := 0
	for _, statement := range statements {
		// syntax.Walk visits a statement before the statements inside it, so
		// the top of a chain is met before the chain's inner links, which
		// markChain then keeps from being read as pipelines of their own.
		syntax.Walk(statement, func(node syntax.Node) bool {
			held, ok := node.(*syntax.Stmt)
			if !ok {
				return true
			}
			if inner[held] {
				return true
			}
			if _, pipe := pipeOf(held); !pipe {
				return true
			}
			markChain(held, inner)
			for stage, leaf := range pipelineLeaves(held, nil) {
				for _, call := range stageCommands(leaf) {
					members[call] = append(members[call], pipelineStage{pipeline: pipelines, stage: stage})
				}
			}
			pipelines++
			return true
		})
	}
	return members
}

// stageCommands answers every simple command one stage of a pipeline runs,
// at any depth, except inside a command or process substitution: what such
// a command prints goes to the substitution and never to the pipe. A stage
// that holds a pipeline of its own counts that pipeline's commands too, so a
// filter in it is read as reading the stage before, which is the side a
// refusal errs on.
func stageCommands(leaf *syntax.Stmt) []*syntax.CallExpr {
	var calls []*syntax.CallExpr
	syntax.Walk(leaf, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
			return false
		case *syntax.CallExpr:
			if len(node.Args) != 0 {
				calls = append(calls, node)
			}
		}
		return true
	})
	return calls
}

// pipeOf answers the pipe a statement is, when its command is a `|` or `|&`.
func pipeOf(statement *syntax.Stmt) (*syntax.BinaryCmd, bool) {
	binary, ok := statement.Cmd.(*syntax.BinaryCmd)
	if !ok {
		return nil, false
	}
	if binary.Op != syntax.Pipe && binary.Op != syntax.PipeAll {
		return nil, false
	}
	return binary, true
}

// markChain marks every link of a pipe chain below its top, so the walk in
// pipelineMembership reads the chain once. The chain is bounded by the line.
func markChain(statement *syntax.Stmt, inner map[*syntax.Stmt]bool) {
	binary, ok := pipeOf(statement)
	if !ok {
		return
	}
	for _, side := range []*syntax.Stmt{binary.X, binary.Y} {
		if _, pipe := pipeOf(side); pipe {
			inner[side] = true
			markChain(side, inner)
		}
	}
}

// pipelineLeaves flattens a chain of pipes, left to right. The chain is
// bounded by the line.
func pipelineLeaves(statement *syntax.Stmt, leaves []*syntax.Stmt) []*syntax.Stmt {
	binary, ok := pipeOf(statement)
	if !ok {
		return append(leaves, statement)
	}
	leaves = pipelineLeaves(binary.X, leaves)
	return pipelineLeaves(binary.Y, leaves)
}

// parsedCall reads one simple command. MUST be called with a command that
// has at least one argument.
func (walk *readerWalk) parsedCall(call *syntax.CallExpr) parsedCommand {
	command := parsedCommand{readable: true}
	start := call.Pos().Offset()
	end := call.Args[len(call.Args)-1].End().Offset()
	if start <= end && end <= uint(len(walk.source)) {
		command.source = walk.source[start:end]
	}
	for _, assign := range call.Assigns {
		command.assigns = append(command.assigns, walk.text(assign))
		token, exact := walk.assignToken(assign)
		command.tokens = append(command.tokens, token)
		command.written = append(command.written, walk.writtenToken(assign, token, exact))
	}
	for _, argument := range call.Args {
		value, err := readWord(argument)
		if err != nil {
			command.readable = false
			command.tokens = append(command.tokens, typedToken(walk.text(argument)))
			command.written = append(command.written, walk.text(argument))
			continue
		}
		token := value.text
		if value.expands {
			command.readable = false
			token = typedToken(walk.text(argument))
		}
		command.tokens = append(command.tokens, token)
		command.written = append(command.written, walk.writtenToken(argument, token, !value.expands))
		command.words = append(command.words, value.text)
	}
	return command
}

// writtenToken answers one token of a command as a refusal writes it back
// (parsedCommand.written): quotedWord of the token when exact reports that
// the hook read its value exactly, and otherwise node's own source text as
// typed, which the shell expands again when the advice runs.
func (walk *readerWalk) writtenToken(node syntax.Node, token string, exact bool) string {
	if exact {
		return quotedWord(token)
	}
	return walk.text(node)
}

// quotedWords writes each of values as one shell word (quotedWord): the
// written form of the words a text judgment read, which no parse says the
// shell expands.
func quotedWords(values []string) []string {
	written := make([]string, 0, len(values))
	for _, value := range values {
		written = append(written, quotedWord(value))
	}
	return written
}

// writtenByValue reports that a refusal writes token back by its value,
// through quotedWord, rather than by its source text as typed
// (writtenToken), which it does for a word the shell expands or the hook
// cannot read. The two forms never coincide: quotedWord writes a literal word
// or single-quoted strings joined by `\'`, and a word typed in that form
// neither expands nor fails to read, so it is written by its value.
func writtenByValue(token, written string) bool {
	return written == quotedWord(token)
}

// assignToken reads one assignment in front of a command as the token the
// word judgments read (parsedCommand.tokens): `name=value` with the value
// read exactly when nothing in it expands, and the assignment's typed text
// otherwise, so `ZE_ADMIT_RAW="$why"` keeps a value that is not empty, as
// the text judgment reads it. exact reports the first form, a value read
// exactly, which a refusal may write back by its value (writtenToken).
func (walk *readerWalk) assignToken(assign *syntax.Assign) (token string, exact bool) {
	if assign.Name == nil {
		return typedToken(walk.text(assign)), false
	}
	if assign.Value == nil {
		return typedToken(walk.text(assign)), false
	}
	if assign.Append {
		return typedToken(walk.text(assign)), false
	}
	if assign.Index != nil {
		return typedToken(walk.text(assign)), false
	}
	value, err := readWord(assign.Value)
	if err != nil {
		return typedToken(walk.text(assign)), false
	}
	if value.expands {
		return typedToken(walk.text(assign)), false
	}
	return assign.Name.Value + "=" + value.text, true
}

// typedToken reads one word's typed text as the text judgment reads a
// segment (shellWords), kept one token: a word the parser delimited is one
// word, whatever spaces its quotes or its substitution hold.
func typedToken(text string) string {
	return strings.Join(shellWords(text), " ")
}
