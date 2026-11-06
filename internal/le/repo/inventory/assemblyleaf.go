// Design: docs/architecture/plugin/feature-gates.md -- a leaf assembly body acts only on its registers, frame and arguments.
// Related: assembly.go -- certifyAssembly; assemblysyntax.go -- the shared statement lexer.
package repoinventory

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// leafDataDirectives emit raw bytes into the instruction stream, so they can
// encode any instruction, a CALL included, that the opcode checks never see.
var leafDataDirectives = map[string]bool{"BYTE": true, "WORD": true, "LONG": true, "QUAD": true, "DWORD": true, "OCTA": true}

// leafTextFlags maps each TEXT flag a leaf header may name to its canonical
// name. Every other flag refuses: DUPOK, for one, lets the linker keep
// another definition of the symbol in place of the one read here.
var leafTextFlags = map[string]string{"NOSPLIT": "NOSPLIT", "4": "NOSPLIT", "NOFRAME": "NOFRAME"}

// leafStep is what one leaf instruction does to control flow, and the
// symbols it reads.
type leafStep struct {
	branch bool       // the instruction may branch to target
	target string     // the label a branch names, set only when branch is true
	ends   bool       // control never reaches the next statement: RET or JMP
	reads  []leafRead // the symbol operands the instruction only reads
}

// leafDefinition proves that one retained TEXT definition is a leaf, and
// returns the Go names of the same-package variables it reads. It reads only
// the retained header tokens and body bytes, and executes nothing.
func leafDefinition(index *assemblyIndex, definition *assemblyDefinition, body []byte, name, prefix string) ([]string, error) {
	if err := leafHeader(definition.header, name); err != nil {
		return nil, fmt.Errorf("%s: %w", definition.path, err)
	}
	reads, err := leafBody(body, definition.path, definition.macros)
	if err != nil {
		return nil, err
	}
	return leafVariables(index, definition, prefix, reads)
}

// leafHeader keeps the trampoline's entry symbol, admits the flags of
// leafFlags, and accepts any canonical frame: `$N`, which Go assembly allows
// when the Go prototype gives the argument size, or `$N-M`.
func leafHeader(tokens []string, name string) error {
	if len(tokens) < 8 {
		return fmt.Errorf("unsupported assembly TEXT flags, frame or ABI grammar")
	}
	if err := textHeaderSymbol(tokens, name); err != nil {
		return err
	}
	consumed, noframe, err := leafFlags(tokens[6:])
	if err != nil {
		return err
	}
	frame := tokens[6+consumed:]
	if len(frame) != 2 && len(frame) != 4 {
		return fmt.Errorf("unsupported assembly TEXT flags, frame or ABI grammar")
	}
	if frame[0] != "$" {
		return fmt.Errorf("unsupported assembly frame")
	}
	if err := assemblySize(frame[1], "frame"); err != nil {
		return err
	}
	// runtime/textflag.h: NOFRAME is "Only valid on functions that declare a
	// frame size of 0."
	if noframe && frame[1] != "0" {
		return fmt.Errorf("assembly NOFRAME requires a zero frame")
	}
	if len(frame) == 2 {
		return nil
	}
	if frame[2] != "-" {
		return fmt.Errorf("unsupported assembly argument size")
	}
	return assemblySize(frame[3], "argument")
}

// leafFlags reads a TEXT flag expression of leafTextFlags names joined by
// `|`, each at most once, through its closing comma, and returns the tokens
// it consumed and whether NOFRAME is set. Neither admitted flag can affect
// registration, because each changes only the prologue the assembler emits:
//
//   - NOSPLIT is required. Without it the assembler inserts a stack-split
//     check that calls runtime·morestack, a transfer outside the body.
//   - NOFRAME stops the assembler emitting the base-pointer save and frame
//     allocation. With the zero frame leafHeader requires and no CALL in a
//     leaf body, cmd/internal/obj/x86's preprocess sets the same no-frame
//     attribute itself, so the emitted code is the same.
func leafFlags(tokens []string) (int, bool, error) {
	seen := make(map[string]bool)
	offset := 0
	for {
		if offset >= len(tokens) {
			return 0, false, fmt.Errorf("unsupported assembly TEXT flags, frame or ABI grammar")
		}
		canonical, admitted := leafTextFlags[tokens[offset]]
		if !admitted {
			return 0, false, fmt.Errorf("unsupported assembly TEXT flag %s: a leaf proves only NOSPLIT and NOFRAME", tokens[offset])
		}
		if seen[canonical] {
			return 0, false, fmt.Errorf("assembly TEXT flags repeat %s", canonical)
		}
		seen[canonical] = true
		offset++
		if offset >= len(tokens) {
			return 0, false, fmt.Errorf("unsupported assembly TEXT flags, frame or ABI grammar")
		}
		if tokens[offset] == "," {
			break
		}
		if tokens[offset] != "|" {
			return 0, false, fmt.Errorf("unsupported assembly TEXT flags")
		}
		offset++
	}
	if !seen["NOSPLIT"] {
		return 0, false, fmt.Errorf("assembly leaf requires NOSPLIT: without it the assembler inserts a call to runtime·morestack")
	}
	return offset + 1, seen["NOFRAME"], nil
}

// leafBody proves that a complete entered body is a leaf: every statement is
// macro-free and condition-free, names a symbol only as a read
// (leafSymbolReads), calls nothing, emits no raw data and defines nothing;
// every branch names a label of this body; and the last statement cannot
// fall past the body's end into whatever follows it. It returns every symbol
// read, for leafVariables to resolve.
func leafBody(raw []byte, name string, macros map[string]bool) ([]leafRead, error) {
	labels := make(map[string]bool)
	var targets []string
	var reads []leafRead
	terminated := false
	err := assemblyStatements(raw, name, func(statement assemblyStatement) error {
		tokens := statement.tokens
		// The complete unit's preprocessing was already validated by the census.
		if tokens[0] == "#" {
			if assemblyConditionalDirective(tokens) != assemblyConditionalNone {
				return fmt.Errorf("%s: entered assembly body contains conditional", name)
			}
			return nil
		}
		if err := macroFreeTokens(tokens, macros); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		for len(tokens) >= 2 && tokens[1] == ":" {
			labels[tokens[0]] = true
			// A branch to this label runs what follows it, so the body is
			// terminated only by an instruction after the last label.
			terminated = false
			tokens = tokens[2:]
		}
		if len(tokens) == 0 {
			return nil
		}
		step, err := leafInstruction(tokens)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if step.branch {
			targets = append(targets, step.target)
		}
		reads = append(reads, step.reads...)
		terminated = step.ends
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		if !labels[target] {
			return nil, fmt.Errorf("%s: assembly leaf branch target %s is not a label of its body", name, target)
		}
	}
	if !terminated {
		return nil, fmt.Errorf("%s: assembly leaf body can run past its end", name)
	}
	return reads, nil
}

// leafInstruction classifies one label-free instruction of a leaf body. The
// caller MUST NOT pass a preprocessing statement.
func leafInstruction(tokens []string) (leafStep, error) {
	opcode := tokens[0]
	if assemblyDefinitionWord(opcode) {
		return leafStep{}, fmt.Errorf("assembly leaf body holds a %s definition", opcode)
	}
	if leafDataDirectives[opcode] {
		return leafStep{}, fmt.Errorf("assembly leaf body emits raw %s data, which can encode any instruction", opcode)
	}
	if slices.Contains(tokens[1:], "TLS") {
		return leafStep{}, fmt.Errorf("assembly leaf body reads thread-local storage")
	}
	if strings.HasPrefix(opcode, "CALL") {
		return leafStep{}, fmt.Errorf("assembly leaf body calls through %s", opcode)
	}
	if !leafBranch(opcode) {
		reads, err := leafSymbolReads(opcode, tokens[1:])
		if err != nil {
			return leafStep{}, err
		}
		return leafStep{ends: opcode == "RET", reads: reads}, nil
	}
	if slices.Contains(tokens[1:], "SB") {
		return leafStep{}, fmt.Errorf("assembly leaf branch %s goes through a symbol", opcode)
	}
	if len(tokens) != 2 {
		return leafStep{}, fmt.Errorf("assembly leaf branch %s has no single label target", opcode)
	}
	target := tokens[1]
	// Go assembly register names are upper case and the assembler reads a
	// register before a label, so only a label holding a lower-case letter can
	// never be read as an indirect jump through a register.
	if !strings.ContainsFunc(target, unicode.IsLower) {
		return leafStep{}, fmt.Errorf("assembly leaf branch target %s can name a register", target)
	}
	return leafStep{branch: true, target: target, ends: opcode == "JMP"}, nil
}

// leafBranch reports the amd64 opcodes that take a code address: every jump
// (all spelled J...), the LOOP family and XBEGIN's abort target.
func leafBranch(opcode string) bool {
	if strings.HasPrefix(opcode, "J") {
		return true
	}
	if strings.HasPrefix(opcode, "LOOP") {
		return true
	}
	return opcode == "XBEGIN"
}
