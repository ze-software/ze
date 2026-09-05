// Design: docs/architecture/core-design.md -- reading a build stage out of a Dockerfile
// Overview: goversion.go -- the gate these stages are judged for
//
// dockerfile.go reads the two facts the gate needs from a Dockerfile: the image
// each stage builds ON, and whether that stage copies THIS MODULE in from the
// build context.
//
// It is not a Dockerfile parser and does not try to be one. A directive this
// gate does not read is passed over rather than refused, because a Dockerfile
// this file cannot fully model still answers both questions correctly, and a
// refusal over an unread RUN line would make the gate about Docker syntax
// instead of about the Go version.

package goversion

import "strings"

// stage is one FROM block: the image it builds on, the line that named it, and
// whether the block copies this module in from the build context.
type stage struct {
	Base   string
	Line   int
	Copies bool
}

// instruction is one logical Dockerfile line, with its continuations joined and
// the line number of the word that opened it.
type instruction struct {
	Words []string
	Line  int
}

// stagesOf answers every FROM block of a Dockerfile, in file order.
//
// A COPY or ADD before the first FROM belongs to no stage and is dropped:
// Docker refuses such a file, so there is no build for this gate to judge.
func stagesOf(body string) []stage {
	var stages []stage
	for _, one := range instructionsOf(body) {
		if len(one.Words) < 2 {
			continue
		}
		switch strings.ToUpper(one.Words[0]) {
		case "FROM":
			stages = append(stages, stage{Base: baseOf(one.Words[1:]), Line: one.Line})
		case "COPY", "ADD":
			if len(stages) == 0 {
				continue
			}
			if !copiesModule(one.Words[1:]) {
				continue
			}
			stages[len(stages)-1].Copies = true
		}
	}
	return stages
}

// instructionsOf splits a Dockerfile into logical lines.
//
// A line ending in a backslash continues onto the next one, which is how a long
// COPY is written, and a comment is dropped whether or not a continuation is
// open. The bound is the file: the loop reads each physical line once.
func instructionsOf(body string) []instruction {
	var out []instruction
	var open []string
	openLine := 0

	for index, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		continued := strings.HasSuffix(line, `\`)
		if continued {
			line = strings.TrimSuffix(line, `\`)
		}
		if len(open) == 0 {
			openLine = index + 1
		}
		open = append(open, strings.Fields(line)...)
		if continued {
			continue
		}

		out = append(out, instruction{Words: open, Line: openLine})
		open = nil
	}

	// A file whose last line leaves a continuation open still names whatever it
	// has said so far, and dropping it would lose a stage.
	if len(open) != 0 {
		out = append(out, instruction{Words: open, Line: openLine})
	}
	return out
}

// baseOf answers the image a FROM names, passing over its flags:
// `FROM --platform=$BUILDPLATFORM golang:1.27 AS builder` names golang:1.27.
func baseOf(words []string) string {
	for _, word := range words {
		if strings.HasPrefix(word, "--") {
			continue
		}
		return word
	}
	return ""
}

// copiesModule reports whether a COPY or ADD brings this module in from the
// build context.
//
// `--from=` names another stage or a named build context, so such a copy never
// brings the module in however its sources read. Otherwise the last word is the
// destination and everything before it is a source, and the module arrives as
// `go.mod` or as the whole context.
func copiesModule(words []string) bool {
	sources := make([]string, 0, len(words))
	for _, word := range words {
		if strings.HasPrefix(word, "--from=") {
			return false
		}
		if strings.HasPrefix(word, "--") {
			continue
		}
		sources = append(sources, word)
	}
	if len(sources) < 2 {
		return false
	}

	for _, source := range sources[:len(sources)-1] {
		if source == goModFile || source == wholeContext || source == "./" {
			return true
		}
	}
	return false
}
