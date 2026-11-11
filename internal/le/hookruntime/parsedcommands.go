// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: rawheavy.go -- rawAdvice, which advises the parsed command it refused
// Related: lossypipe.go -- advisedSegment, which advises the parsed command it refused
// Related: readerline.go -- the reader-line walk, which judges what this file only reads
package hookruntime

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// parsedCommand is one simple command of any parsed line, reader or not,
// for the advice a refusal prints (rawAdvice, advisedSegment).
type parsedCommand struct {
	// words holds each argument's value; readable reports that every
	// argument and assignment has one the hook can read and none expands,
	// so the words, each written back with quotedWord, are the command.
	words    []string
	readable bool
	// assigns holds the assignments in front, as typed.
	assigns []string
	// source is the command's own text from its first assignment or word
	// to its last word, as typed, with no redirection.
	source string
	// pipeline numbers the pipeline the command stands in, and stage its
	// place in it from 0; a command outside a pipeline is a pipeline of one.
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

// everyCommand answers every simple command of a parsed line at any depth,
// in source order, with the pipeline each stands in. It judges nothing: it
// is the reading the advice is written from, so a refusal names the command
// the shell runs rather than a text segment cut at a quoted bar (AC-15).
func everyCommand(source string, statements []*syntax.Stmt) []parsedCommand {
	walk := readerWalk{source: source}
	commands := make([]parsedCommand, 0, 8)
	stages := make(map[*syntax.Stmt][2]int)
	pipelines := 0
	for _, statement := range statements {
		// syntax.Walk visits a statement before the statements inside it,
		// so a pipeline numbers its stages before the walk reaches them.
		syntax.Walk(statement, func(node syntax.Node) bool {
			held, ok := node.(*syntax.Stmt)
			if !ok {
				return true
			}
			if _, staged := stages[held]; !staged {
				stages[held] = [2]int{pipelines, 0}
				pipelines++
			}
			pipelineStages(held, stages)
			call, simple := held.Cmd.(*syntax.CallExpr)
			if !simple {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			command := walk.parsedCall(call)
			command.pipeline, command.stage = stages[held][0], stages[held][1]
			commands = append(commands, command)
			return true
		})
	}
	return commands
}

// pipelineStages numbers the stages of a pipeline statement: every leaf of
// a chain of `|` and `|&` takes the pipeline of the statement and its place.
func pipelineStages(statement *syntax.Stmt, stages map[*syntax.Stmt][2]int) {
	binary, ok := statement.Cmd.(*syntax.BinaryCmd)
	if !ok {
		return
	}
	if binary.Op != syntax.Pipe && binary.Op != syntax.PipeAll {
		return
	}
	pipeline := stages[statement][0]
	for stage, leaf := range pipelineLeaves(statement, nil) {
		stages[leaf] = [2]int{pipeline, stage}
	}
}

// pipelineLeaves flattens a chain of pipes, left to right. The chain is
// bounded by the line.
func pipelineLeaves(statement *syntax.Stmt, leaves []*syntax.Stmt) []*syntax.Stmt {
	binary, ok := statement.Cmd.(*syntax.BinaryCmd)
	if !ok {
		return append(leaves, statement)
	}
	if binary.Op != syntax.Pipe && binary.Op != syntax.PipeAll {
		return append(leaves, statement)
	}
	leaves = pipelineLeaves(binary.X, leaves)
	return pipelineLeaves(binary.Y, leaves)
}

// parsedCall reads one simple command for the advice. MUST be called with a
// command that has at least one argument.
func (walk *readerWalk) parsedCall(call *syntax.CallExpr) parsedCommand {
	command := parsedCommand{readable: true}
	start := call.Pos().Offset()
	end := call.Args[len(call.Args)-1].End().Offset()
	if start <= end && end <= uint(len(walk.source)) {
		command.source = walk.source[start:end]
	}
	for _, assign := range call.Assigns {
		command.assigns = append(command.assigns, walk.text(assign))
	}
	for _, argument := range call.Args {
		value, err := readWord(argument)
		if err != nil {
			command.readable = false
			continue
		}
		if value.expands {
			command.readable = false
		}
		command.words = append(command.words, value.text)
	}
	return command
}
