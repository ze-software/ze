// Design: docs/architecture/plugin/feature-gates.md -- a leaf body is judged after its own file's macros expand.
// Related: committed_assembly_leaf_test.go -- the leaf fixture and its literal controls.
package repoinventory

import (
	"strconv"
	"testing"
)

// asmLeafXXHashDecl is the Go side of the vendored cespare/xxhash/v2
// xxhash_amd64.s (asmLeafXXHashUnit): the primes the assembly reads through
// ·primes(SB), the Digest that writeBlocks loads and stores through its
// pointer argument, and the two bodyless declarations of xxhash_asm.go.
const asmLeafXXHashDecl = "const (\n" +
	"\tprime1 uint64 = 11400714785074694791\n\tprime2 uint64 = 14029467366897019727\n" +
	"\tprime3 uint64 = 1609587929392839161\n\tprime4 uint64 = 9650029242287828579\n" +
	"\tprime5 uint64 = 2870177450012600261\n)\n" +
	"var primes = [...]uint64{prime1, prime2, prime3, prime4, prime5}\n" +
	"type Digest struct {\n\tv1 uint64\n\tv2 uint64\n\tv3 uint64\n\tv4 uint64\n\ttotal uint64\n\tmem [32]byte\n\tn int\n}\n" +
	"//go:noescape\nfunc Sum64(b []byte) uint64\n" +
	"//go:noescape\nfunc writeBlocks(d *Digest, b []byte) int\n"

// TestCommittedAssemblyLeafXXHashMacrosPreserveFullReport reproduces the
// actual-root refusal at cespare/xxhash/v2: writeBlocks and Sum64 are written
// with the file's own object-like register macros and function-like macros
// continued over several lines, one of them holding a label. Each body is a
// leaf once those macros expand, so the full report publishes for both
// registration sinks.
//
// MUTATION: (assemblyexpand.go, `if !expansion.local[name] {`, `if true {`):
// no local macro expands, macroFreeTokens refuses the first register macro,
// and the report refuses as it did before expansion existed.
// MUTATION: (assemblyleaf.go, `walk.labels[tokens[0]] = true`,
// `walk.labels[tokens[0]] = false`): the `loop:` blockLoop expands loses its
// label and `JLE loop` refuses.
// MUTATION: (assemblysyntax.go, `definitionRecord.locals = maps.Clone(unit.current)`,
// `definitionRecord.locals = nil`): no macro is defined where the bodies
// start, and the report refuses with "is used where it is not defined".
func TestCommittedAssemblyLeafXXHashMacrosPreserveFullReport(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, asmLeafXXHashUnit, asmLeafXXHashDecl, "_ = Sum64(nil)\n_ = writeBlocks(&Digest{}, nil)")
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedAssemblyLeafMacroUndefRedefinePublishes proves #undef: a name
// defined, removed and defined again expands to its second replacement, and
// the leaf publishes.
//
// MUTATION: (assemblysyntax.go, `if assemblyUndef(tokens) && !blocks.inside() {`,
// `if false {`):
// the census reads `#undef val` as unknown preprocessing and the report
// refuses.
// MUTATION: (assemblymacros.go, `if unit.local[name] && unit.current[name] == nil {`,
// `if false {`): the second #define binds an existing name and the census
// refuses it as a redefinition.
func TestCommittedAssemblyLeafMacroUndefRedefinePublishes(t *testing.T) {
	const macros = "#define val BX\n#undef val\n#define val CX\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, macros+asmLeafReadBody("MOVQ val, AX"), asmLeafReadDecl("var leafPrimes = [...]uint64{1, 2, 3}"), asmLeafReadCall)
			externalEffectsPositive(t, fixture)
		})
	}
}

// asmLeafDoublingMacros defines a0 as one register and each later aN as two
// copies of the one before, so a16 expands to 65536 tokens and its expansion
// pushes about twice that, past leafExpansionTokensMax.
func asmLeafDoublingMacros() string {
	macros := "#define a0 AX\n"
	for level := 1; level <= 16; level++ {
		previous := "a" + strconv.Itoa(level-1)
		macros += "#define a" + strconv.Itoa(level) + " " + previous + " " + previous + "\n"
	}
	return macros
}

// TestCommittedAssemblyLeafMacroControlsRefuse keeps every leaf rule applying
// to the expanded body, and every expansion the Go assembler would refuse or
// read differently refusing, each with its own cause.
//
// MUTATION (call): (assemblyleaf.go, `if strings.HasPrefix(opcode, "CALL") {`,
// `if false {`): the expanded `CALL ·inventoryOther(SB)` refuses only as a
// write through its last operand, without "calls through CALL".
// MUTATION (write): (assemblyexpand.go,
// `if formal := slices.Index(macro.formals, normalizedAssemblyToken(text)); formal >= 0 {`,
// `if formal := slices.Index(macro.formals, normalizedAssemblyToken(text)); false {`):
// the formal `dst` is not substituted, `MOVQ AX, dst` names no symbol, and the
// store to ·leafPrimes publishes.
// MUTATION (write): (assemblyleafreads.go,
// `if offset == len(operands)-1 && !leafCompares[opcode] {`,
// `if offset == len(operands)-1 && !leafCompares[opcode] && false {`): the
// expanded store to ·leafPrimes publishes.
// MUTATION (arity): (assemblyexpand.go, `if len(actuals) != len(macro.formals) {`,
// `if false {`): `keep(BX, CX)` substitutes BX and publishes.
// MUTATION (before-definition, after-undef): (assemblyexpand.go,
// `if !expansion.local[name] {`,
// `if !expansion.local[name] || expansion.defined[name] == nil {`): the name
// passes as a plain token, as cmd/asm reads it, and refuses only through
// macroFreeTokens, without "is used where it is not defined".
// MUTATION (redefinition): (assemblyheaders.go, `if unit.bound[name] {`,
// `if unit.bound[name] && false {`): the second #define replaces the first
// and the leaf publishes.
// MUTATION (recursive): (assemblyexpand.go, `if slices.Contains(head.chain, name) {`,
// `if false {`): `val` expands until the token bound refuses it, without
// "expands to itself".
// MUTATION (bound): (assemblyexpand.go, `if expansion.budget < 0 {`,
// `if false {`): the 65536-token operand expands in full and publishes.
// MUTATION (label-twice): (assemblyleaf.go, `if walk.labels[tokens[0]] {`,
// `if false {`): the second `again:` redefines the label and publishes.
// MUTATION (label-twice): (assemblyexpand.go, `if !leafStatementEnd(text) {`,
// `if true {`): each replacement reads as one statement whose opcode is a
// newline, so no label is defined and the body publishes.
// MUTATION (definition-word): (assemblymacros.go,
// `if assemblyDefinitionWord(normalizedAssemblyToken(word)) {`, `if false {`):
// the census admits the DATA replacement and the leaf refuses it only as
// "holds a DATA definition", without "definition-word origin".
func TestCommittedAssemblyLeafMacroControlsRefuse(t *testing.T) {
	const keep = "#define keep(dst) MOVQ AX, dst\n"
	for _, test := range []struct{ name, unit, cause string }{
		{"call", "#define dispatch CALL ·inventoryOther(SB)\n" + asmLeafReadBody("dispatch"), "calls through CALL"},
		{"write", keep + asmLeafReadBody("keep(·leafPrimes+0(SB))"), "writes symbol memory"},
		{"arity", keep + asmLeafReadBody("keep(BX, CX)"), "is called with 2 arguments, but its definition names 1"},
		{"before-definition", asmLeafReadBody("MOVQ later, AX") + "#define later BX\n", "is used where it is not defined"},
		{"after-undef", "#define val BX\n#undef val\n" + asmLeafReadBody("MOVQ val, AX"), "is used where it is not defined"},
		{"redefinition", "#define val BX\n#define val CX\n" + asmLeafReadBody("MOVQ val, AX"), "assembly macro val is redefined"},
		{"recursive", "#define val val\n" + asmLeafReadBody("MOVQ val, AX"), "expands to itself"},
		{"bound", asmLeafDoublingMacros() + asmLeafReadBody("MOVQ a16, BX"), "passes 65536 tokens"},
		{"label-twice", "#define spin() \\\nagain: \\\n JNE again\n" + asmLeafReadBody("spin()\n\tspin()"), "defines label again twice"},
		{"definition-word", "#define emit DATA leafTable<>+8(SB)/8, $0\n" + asmLeafReadBody("emit"), "definition-word origin"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(test.name+"/"+kind, func(t *testing.T) {
				fixture := asmLeafFixture(t, kind, test.unit, asmLeafReadDecl("var leafPrimes = [...]uint64{1, 2, 3}"), asmLeafReadCall)
				asmLeafRefusal(t, fixture, test.cause)
			})
		}
	}
}

// asmLeafXXHashUnit is vendor/github.com/cespare/xxhash/v2/xxhash_amd64.s,
// byte for byte.
const asmLeafXXHashUnit = `//go:build !appengine && gc && !purego
// +build !appengine
// +build gc
// +build !purego

#include "textflag.h"

// Registers:
#define h      AX
#define d      AX
#define p      SI // pointer to advance through b
#define n      DX
#define end    BX // loop end
#define v1     R8
#define v2     R9
#define v3     R10
#define v4     R11
#define x      R12
#define prime1 R13
#define prime2 R14
#define prime4 DI

#define round(acc, x) \
	IMULQ prime2, x   \
	ADDQ  x, acc      \
	ROLQ  $31, acc    \
	IMULQ prime1, acc

// round0 performs the operation x = round(0, x).
#define round0(x) \
	IMULQ prime2, x \
	ROLQ  $31, x    \
	IMULQ prime1, x

// mergeRound applies a merge round on the two registers acc and x.
// It assumes that prime1, prime2, and prime4 have been loaded.
#define mergeRound(acc, x) \
	round0(x)         \
	XORQ  x, acc      \
	IMULQ prime1, acc \
	ADDQ  prime4, acc

// blockLoop processes as many 32-byte blocks as possible,
// updating v1, v2, v3, and v4. It assumes that there is at least one block
// to process.
#define blockLoop() \
loop:  \
	MOVQ +0(p), x  \
	round(v1, x)   \
	MOVQ +8(p), x  \
	round(v2, x)   \
	MOVQ +16(p), x \
	round(v3, x)   \
	MOVQ +24(p), x \
	round(v4, x)   \
	ADDQ $32, p    \
	CMPQ p, end    \
	JLE  loop

// func Sum64(b []byte) uint64
TEXT ·Sum64(SB), NOSPLIT|NOFRAME, $0-32
	// Load fixed primes.
	MOVQ ·primes+0(SB), prime1
	MOVQ ·primes+8(SB), prime2
	MOVQ ·primes+24(SB), prime4

	// Load slice.
	MOVQ b_base+0(FP), p
	MOVQ b_len+8(FP), n
	LEAQ (p)(n*1), end

	// The first loop limit will be len(b)-32.
	SUBQ $32, end

	// Check whether we have at least one block.
	CMPQ n, $32
	JLT  noBlocks

	// Set up initial state (v1, v2, v3, v4).
	MOVQ prime1, v1
	ADDQ prime2, v1
	MOVQ prime2, v2
	XORQ v3, v3
	XORQ v4, v4
	SUBQ prime1, v4

	blockLoop()

	MOVQ v1, h
	ROLQ $1, h
	MOVQ v2, x
	ROLQ $7, x
	ADDQ x, h
	MOVQ v3, x
	ROLQ $12, x
	ADDQ x, h
	MOVQ v4, x
	ROLQ $18, x
	ADDQ x, h

	mergeRound(h, v1)
	mergeRound(h, v2)
	mergeRound(h, v3)
	mergeRound(h, v4)

	JMP afterBlocks

noBlocks:
	MOVQ ·primes+32(SB), h

afterBlocks:
	ADDQ n, h

	ADDQ $24, end
	CMPQ p, end
	JG   try4

loop8:
	MOVQ  (p), x
	ADDQ  $8, p
	round0(x)
	XORQ  x, h
	ROLQ  $27, h
	IMULQ prime1, h
	ADDQ  prime4, h

	CMPQ p, end
	JLE  loop8

try4:
	ADDQ $4, end
	CMPQ p, end
	JG   try1

	MOVL  (p), x
	ADDQ  $4, p
	IMULQ prime1, x
	XORQ  x, h

	ROLQ  $23, h
	IMULQ prime2, h
	ADDQ  ·primes+16(SB), h

try1:
	ADDQ $4, end
	CMPQ p, end
	JGE  finalize

loop1:
	MOVBQZX (p), x
	ADDQ    $1, p
	IMULQ   ·primes+32(SB), x
	XORQ    x, h
	ROLQ    $11, h
	IMULQ   prime1, h

	CMPQ p, end
	JL   loop1

finalize:
	MOVQ  h, x
	SHRQ  $33, x
	XORQ  x, h
	IMULQ prime2, h
	MOVQ  h, x
	SHRQ  $29, x
	XORQ  x, h
	IMULQ ·primes+16(SB), h
	MOVQ  h, x
	SHRQ  $32, x
	XORQ  x, h

	MOVQ h, ret+24(FP)
	RET

// func writeBlocks(d *Digest, b []byte) int
TEXT ·writeBlocks(SB), NOSPLIT|NOFRAME, $0-40
	// Load fixed primes needed for round.
	MOVQ ·primes+0(SB), prime1
	MOVQ ·primes+8(SB), prime2

	// Load slice.
	MOVQ b_base+8(FP), p
	MOVQ b_len+16(FP), n
	LEAQ (p)(n*1), end
	SUBQ $32, end

	// Load vN from d.
	MOVQ s+0(FP), d
	MOVQ 0(d), v1
	MOVQ 8(d), v2
	MOVQ 16(d), v3
	MOVQ 24(d), v4

	// We don't need to check the loop condition here; this function is
	// always called with at least one block of data to process.
	blockLoop()

	// Copy vN back to d.
	MOVQ v1, 0(d)
	MOVQ v2, 8(d)
	MOVQ v3, 16(d)
	MOVQ v4, 24(d)

	// The number of bytes written is p minus the old base pointer.
	SUBQ b_base+8(FP), p
	MOVQ p, ret+32(FP)

	RET
`
