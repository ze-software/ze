// Design: docs/architecture/plugin/feature-gates.md -- a leaf assembly body is an external call over its arguments.
package repoinventory

import (
	"reflect"
	"strings"
	"testing"
)

const asmLeafFile = asmOrdinaryDir + "/leaf_linux_amd64.s"

// asmLeafCPUInfoBody is the klauspost/compress internal/cpuinfo x86extensions
// body under a fixture name: a `$0` frame with no argument size, a branch to
// a label of its own body, register work, and stores to its own FP results.
const asmLeafCPUInfoBody = "//go:build !noasm && gc\n\n#include \"textflag.h\"\n#include \"funcdata.h\"\n\n" +
	"TEXT ·inventoryLeaf(SB), NOSPLIT, $0\n" +
	"\t// 1. determine max EAX value\n\tXORQ AX, AX\n\tCPUID\n\n\tCMPQ AX, $7\n\tJB   unsupported\n\n" +
	"\tMOVQ $7, AX\n\tMOVQ $0, CX\n\tCPUID\n\n" +
	"\tBTQ   $3, BX // bit 3 = BMI1\n\tSETCS AL\n\n\tBTQ   $8, BX // bit 8 = BMI2\n\tSETCS AH\n\n" +
	"\tMOVB AL, bmi1+0(FP)\n\tMOVB AH, bmi2+1(FP)\n\tRET\n\n" +
	"unsupported:\n\tXORQ AX, AX\n\tMOVB AL, bmi1+0(FP)\n\tMOVB AL, bmi2+1(FP)\n\tRET\n"

const asmLeafResultsDecl = "func inventoryLeaf() (bmi1, bmi2 bool)\n"

// asmLeafStoreBody reads its pointer argument and writes the count field
// through it, beside the callback field it never touches. It
// references no symbol and transfers control nowhere.
const asmLeafStoreBody = "#include \"textflag.h\"\nTEXT ·inventoryLeafStore(SB),NOSPLIT,$0-8\n MOVQ target+0(FP), AX\n MOVQ $1, 8(AX)\n RET\n"

const asmLeafStoreDecl = "type leafHolder struct { hook func(); count int }\nfunc inventoryLeafStore(target *leafHolder)\n"

// asmLeafFixture is the external-effects fixture with a leaf unit and its
// Go caller. The unit and the call change together for each case.
func asmLeafFixture(t *testing.T, kind, unit, declarations, call string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	write(t, fixture.root, asmLeafFile, unit)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
	commitInventoryFixture(t, fixture.root)
	return fixture
}

// asmLeafRefusal requires both public entry points to refuse with no partial
// population, and the refusal to carry the certificate's own cause.
func asmLeafRefusal(t *testing.T, fixture *asmInventoryFixture, cause string) {
	t.Helper()
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr == nil {
		t.Fatalf("unproved leaf body published a report: %+v", inv)
	}
	if pluginErr == nil {
		t.Fatalf("unproved leaf body published plugins: %+v", plugins)
	}
	if !reflect.DeepEqual(inv, Inventory{}) {
		t.Fatalf("leaf refusal leaked a partial report: %+v", inv)
	}
	if plugins != nil {
		t.Fatalf("leaf refusal leaked partial plugins: %+v", plugins)
	}
	for _, err := range []error{reportErr, pluginErr} {
		if !strings.Contains(err.Error(), cause) {
			t.Fatalf("leaf refusal lacks %q: %v", cause, err)
		}
	}
}

// TestCommittedAssemblyLeafCPUInfoPreservesFullReport reproduces the
// actual-root refusal at klauspost/compress cpuinfo: an initializer calls a
// bodyless function whose retained assembly is a leaf. The leaf references
// no symbol and branches only inside itself, so the full report publishes for
// both registration sinks.
//
// MUTATION: (assemblyleaf.go, `if len(frame) != 2 && len(frame) != 4 {`,
// `if len(frame) != 4 {`): the `$0` header refuses, the trampoline grammar
// refuses it too, and the report refuses with "unsupported assembly TEXT
// flags, frame or ABI grammar", the actual-root message.
// MUTATION: (assemblyleaf.go, `walk.labels[tokens[0]] = true`,
// `walk.labels[tokens[0]] = false`): the local `JB unsupported` loses its
// label and the report refuses.
func TestCommittedAssemblyLeafCPUInfoPreservesFullReport(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, asmLeafCPUInfoBody, asmLeafResultsDecl, "_, _ = inventoryLeaf()")
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedAssemblyLeafTracksPointerArgument passes a pointer to a struct
// holding a callback into a leaf that writes through that pointer. The leaf
// is modeled as an external call over its arguments: an inert callback
// publishes, and a callback that registers refuses through the external
// capability model, exactly as a direct standard-library call would.
//
// MUTATION: in assemblyResult, delete `result.join(inputs)`: the registering
// callback reachable through the argument no longer reaches the external
// model and the report publishes.
func TestCommittedAssemblyLeafTracksPointerArgument(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, asmLeafStoreBody, asmLeafStoreDecl, "inventoryLeafStore(&leafHolder{hook: func() {}})")
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(asmLeafStoreDecl, "inventoryLeafStore(&leafHolder{hook: func() { install(0) }})"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedAssemblyLeafControlsRefuse keeps every body that is not a
// leaf refusing with the trampoline certificate's message of today. Each
// header carries a zero frame so the trampoline check reaches the body.
//
// MUTATION (call-register): in leafInstruction, delete the
// `strings.HasPrefix(opcode, "CALL")` refusal: `CALL AX` becomes a leaf and
// publishes.
// MUTATION (call-symbol): refused by three rules, so it publishes only under
// all three breaks. (assemblyleaf.go, `if strings.HasPrefix(opcode, "CALL") {`,
// `if false {`) lets the CALL reach leafSymbolReads;
// (assemblyleafreads.go, `if offset == len(operands)-1 && !leafCompares[opcode] {`,
// `if offset == len(operands)-1 && !leafCompares[opcode] && false {`) reads
// its one operand as a read; and ·inventoryOther, which no Go file and no
// assembly defines, then refuses in leafReadOnlyTable until
// (assemblyleafreads.go, `if globals != 1 {`, `if globals != 1 && false {`).
// MUTATION (jump-symbol): refused by three rules, so it publishes only under
// all three breaks. (assemblyleaf.go, `if slices.Contains(tokens[1:], "SB") {`,
// `if false {`) and (assemblyleaf.go, `if len(tokens) != 2 {`, `if false {`)
// make `·inventoryOther` the branch target, which holds a lower-case letter,
// so leafBody refuses it as a label the body never defines until
// (assemblyleaf.go, `if !walk.labels[target] {`,
// `if !walk.labels[target] && false {`).
// MUTATION (address-constant): the `$` sits in the symbol slot of the
// operand, so beyond the `$` rule three checks read it as the symbol: the
// operand-form check in leafSymbolOperand, its closing `(SB)` check, and
// assemblySymbol in leafSymbolTarget. It publishes under these three breaks:
// (assemblyleafreads.go, `if operand[0] == "$" {`,
// `if operand[0] == "$" && false {`);
// (assemblyleafreads.go, `read, err := leafSymbolOperand(operand)`,
// `read, err := leafSymbolOperand(operand[1:])`), which reads the operand
// past its first token, here the `$`; and, because ·inventoryRegistry is
// defined nowhere, (assemblyleafreads.go, `if globals != 1 {`,
// `if globals != 1 && false {`).
// MUTATION (register-named-label): in leafInstruction, change
// `strings.ContainsFunc(target, unicode.IsLower)` to `true`: a body that
// defines a label spelled `AX` turns the register jump `JMP AX` into a
// "local" branch and publishes.
// MUTATION (jump-register): refused by two rules, the lowercase-label rule
// and the defined-label rule; it publishes only when both are deleted.
// MUTATION (jump-missing-label): (assemblyleaf.go, `if !walk.labels[target] {`,
// `if !walk.labels[target] && false {`): a branch to a label the body never
// defines publishes.
// MUTATION (byte-directive): in leafInstruction, delete the data-directive
// refusal: `BYTE $0xe8` publishes.
// MUTATION (falls-through): (assemblyleaf.go, `if !walk.terminated {`,
// `if false {`): a body that runs off its end publishes.
// MUTATION (flag-expression): (assemblyleaf.go, `if !admitted {`,
// `if !admitted && false {`): DUPOK reads as an admitted flag beside NOSPLIT
// and the `NOSPLIT|DUPOK` leaf publishes.
// MUTATION (noncanonical-flag): the same break admits `DUPOK` alone, and the
// `DUPOK` leaf publishes.
// MUTATION (noframe-frame): in leafHeader, delete the NOFRAME zero-frame
// refusal: the `NOSPLIT|NOFRAME, $8-2` leaf publishes.
func TestCommittedAssemblyLeafControlsRefuse(t *testing.T) {
	const header = "#include \"textflag.h\"\nTEXT ·inventoryLeaf(SB),NOSPLIT,$0-2\n"
	for _, test := range []struct{ name, unit, cause string }{
		{"call-symbol", header + " CALL ·inventoryOther(SB)\n RET\n", "assembly body is not a direct JMP"},
		{"call-register", header + " CALL AX\n RET\n", "assembly body is not a direct JMP"},
		{"jump-symbol", header + " XORQ AX, AX\n JMP ·inventoryOther(SB)\n", "assembly body is not a direct JMP"},
		{"jump-register", header + " XORQ AX, AX\n JMP AX\n", "assembly body is not a direct JMP"},
		{"jump-missing-label", header + " XORQ AX, AX\n JNE elsewhere\n RET\n", "assembly body is not a direct JMP"},
		{"address-constant", header + " MOVQ $·inventoryRegistry(SB), AX\n RET\n", "assembly body is not a direct JMP"},
		{"byte-directive", header + " BYTE $0xe8\n RET\n", "assembly body is not a direct JMP"},
		{"falls-through", header + " XORQ AX, AX\n", "assembly body is not a direct JMP"},
		{"register-named-label", header + " XORQ AX, AX\n JMP AX\nAX:\n RET\n", "assembly body is not a direct JMP"},
		{"noncanonical-flag", "#include \"textflag.h\"\nTEXT ·inventoryLeaf(SB),DUPOK,$0-2\n MOVB $0, bmi1+0(FP)\n MOVB $0, bmi2+1(FP)\n RET\n", "only canonical NOSPLIT is proved"},
		{"flag-expression", "#include \"textflag.h\"\nTEXT ·inventoryLeaf(SB),NOSPLIT|DUPOK,$0-2\n MOVB $0, bmi1+0(FP)\n MOVB $0, bmi2+1(FP)\n RET\n", "a leaf proves only NOSPLIT and NOFRAME"},
		{"noframe-frame", "#include \"textflag.h\"\nTEXT ·inventoryLeaf(SB),NOSPLIT|NOFRAME,$8-2\n MOVB $0, bmi1+0(FP)\n MOVB $0, bmi2+1(FP)\n RET\n", "NOFRAME requires a zero frame"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(test.name+"/"+kind, func(t *testing.T) {
				fixture := asmLeafFixture(t, kind, test.unit, asmLeafResultsDecl, "_, _ = inventoryLeaf()")
				asmLeafRefusal(t, fixture, test.cause)
			})
		}
	}
}

// TestCommittedAssemblyLeafWithoutNOSPLITPublishes requires a leaf whose
// header lacks NOSPLIT to publish as a leaf, as klauspost/compress's
// regenerated amd64 decoders declare `TEXT ·buildDtable_asm(SB), $0-24`: the
// stack-split prologue the assembler then emits enters only runtime·morestack,
// which grows the stack and re-enters the function. Method: the shared leaf
// body under a NOFRAME-only header and under a header with no flag operand.
//
// MUTATION: (internal/le/repo/inventory/assemblyleaf.go,
// `	if len(tokens) != 0 && tokens[0] == "$" {`,
// `	if len(tokens) != 0 && tokens[0] == "-" {`): the flagless header reads
// `$` as a flag and refuses as an unsupported TEXT flag.
// MUTATION: (internal/le/repo/inventory/assemblyleaf.go,
// `	return offset + 1, seen["NOFRAME"], nil`,
// `	return offset + 1, seen["NOFRAME"], fmt.Errorf("x")`): a flagged leaf
// refuses, so the NOFRAME-only header refuses.
func TestCommittedAssemblyLeafWithoutNOSPLITPublishes(t *testing.T) {
	const body = " MOVB $0, bmi1+0(FP)\n MOVB $0, bmi2+1(FP)\n RET\n"
	for _, header := range []struct{ name, text string }{
		{"noframe-only", "TEXT ·inventoryLeaf(SB),NOFRAME,$0-2\n"},
		{"no-flags", "TEXT ·inventoryLeaf(SB), $0-2\n"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(header.name+"/"+kind, func(t *testing.T) {
				fixture := asmLeafFixture(t, kind, "#include \"textflag.h\"\n"+header.text+body, asmLeafResultsDecl, "_, _ = inventoryLeaf()")
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// asmLeafTable is a same-file read-only table: its one GLOBL carries
// NOPTR|RODATA and its one DATA holds an integer, no symbol address. It
// precedes the TEXT so it is not part of the entered body.
const asmLeafTable = "DATA leafTable<>+0(SB)/8, $0x9E3779B185EBCA87\nGLOBL leafTable<>(SB), NOPTR|RODATA, $8\n\n"

// asmLeafSumBody is the cespare/xxhash/v2 writeBlocks shape under a fixture
// name: a `NOSPLIT|NOFRAME, $0-16` header, reads of a same-package Go array
// through `·leafPrimes+N(SB)` and of the same-file RODATA table, a compare
// whose last operand is a symbol, and a store through its pointer argument.
const asmLeafSumBody = "#include \"textflag.h\"\n\n" + asmLeafTable +
	"TEXT ·inventoryLeafSum(SB), NOSPLIT|NOFRAME, $0-16\n" +
	"\tMOVQ ·leafPrimes+0(SB), AX\n\tMOVQ ·leafPrimes+8(SB), BX\n" +
	"\tADDQ leafTable<>+0(SB), AX\n\tIMULQ leafTable<>(SB), BX\n" +
	"\tCMPQ AX, ·leafPrimes+16(SB)\n\tJLT  done\n\tXORQ AX, AX\n" +
	"done:\n\tMOVQ target+0(FP), CX\n\tMOVQ AX, 8(CX)\n\tMOVQ BX, ret+8(FP)\n\tRET\n"

const asmLeafSumDecl = "type leafHolder struct { hook func(); count int }\nvar leafPrimes = [...]uint64{1, 2, 3}\nfunc inventoryLeafSum(target *leafHolder) uint64\n"

// asmLeafReadBody is a NOSPLIT|NOFRAME leaf that runs the given instruction
// and stores AX through its pointer argument. The read-only table and a
// NOPTR-only table are defined before it, outside the entered body.
func asmLeafReadBody(instruction string) string {
	return "#include \"textflag.h\"\n\n" + asmLeafTable + "DATA leafPlain<>+0(SB)/8, $1\nGLOBL leafPlain<>(SB), NOPTR, $8\n\n" +
		"TEXT ·inventoryLeafRead(SB), NOSPLIT|NOFRAME, $0-8\n\t" + instruction + "\n" +
		"\tMOVQ target+0(FP), CX\n\tMOVQ AX, 8(CX)\n\tRET\n"
}

const asmLeafReadCall = "inventoryLeafRead(&leafHolder{hook: func() {}})"

func asmLeafReadDecl(variables string) string {
	return "type leafHolder struct { hook func(); count int }\n" + variables + "\nfunc inventoryLeafRead(target *leafHolder)\n"
}

// TestCommittedAssemblyLeafSymbolReadsPreserveFullReport reproduces the
// actual-root refusal at cespare/xxhash/v2 writeBlocks: a leaf that reads a
// same-package Go variable and a same-file read-only table through (SB). Neither
// read adds a capability the call does not already model, so the full report
// publishes for both registration sinks.
//
// MUTATION: in leafFlags, delete the `"NOFRAME": "NOFRAME"` entry of
// leafTextFlags: the header refuses with "a leaf proves only NOSPLIT and
// NOFRAME" and the report refuses.
// MUTATION: in leafSymbolReads, change `!leafCompares[opcode]` to `true`:
// `CMPQ AX, ·leafPrimes+16(SB)` reads as a write and the report refuses.
// MUTATION: in leafReadOnlyTable, change `!flags["RODATA"]` to `true`: the
// table reads as writable and the report refuses.
func TestCommittedAssemblyLeafSymbolReadsPreserveFullReport(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, asmLeafSumBody, asmLeafSumDecl, "_ = inventoryLeafSum(&leafHolder{hook: func() {}})")
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedAssemblyLeafVariableIsCallInput proves that a same-package Go
// variable a leaf reads is an input of the call, joined like an argument. The
// same declarations publish while the leaf does not read them, and an inert
// callback publishes when it does; a registering callback, held directly in a
// func-typed variable or in a struct field, refuses through the external
// capability model once the leaf reads its variable.
//
// MUTATION: in assemblyResult, delete `result.join(value)` in the variable
// loop: the registering callback no longer reaches the external model and
// the read cases publish.
// MUTATION: in leafSymbolReads, delete the `reads = append(reads, read)`
// line: the leaf reports no variable and the read cases publish.
func TestCommittedAssemblyLeafVariableIsCallInput(t *testing.T) {
	const registering = "var leafHook = func() { install(0) }\nvar leafState = struct{ count int; hook func() }{hook: func() { install(0) }}"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind+"/unread", func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, asmLeafReadBody("XORQ AX, AX"), asmLeafReadDecl(registering), asmLeafReadCall)
			externalEffectsPositive(t, fixture)
		})
		t.Run(kind+"/inert", func(t *testing.T) {
			fixture := asmLeafFixture(t, kind, asmLeafReadBody("MOVQ ·leafHook(SB), AX"), asmLeafReadDecl("var leafHook = func() {}"), asmLeafReadCall)
			externalEffectsPositive(t, fixture)
		})
		for _, read := range []struct{ name, instruction string }{
			{"func-var", "MOVQ ·leafHook(SB), AX"},
			{"struct-field", "MOVQ ·leafState+8(SB), AX"},
		} {
			t.Run(kind+"/"+read.name, func(t *testing.T) {
				fixture := asmLeafFixture(t, kind, asmLeafReadBody(read.instruction), asmLeafReadDecl(registering), asmLeafReadCall)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedAssemblyLeafSymbolControlsRefuse keeps every symbol operand
// that is not a read of a same-file read-only table or a same-package Go
// variable refusing, with the leaf's own cause beside the trampoline's. A
// mutation that removes one rule while another still refuses changes the
// cause, so this test goes red on the message.
//
// MUTATION (write-destination): in leafSymbolReads, delete the
// last-operand write refusal: the store to ·leafPrimes publishes.
// MUTATION (lea): in leafSymbolReads, delete the LEA refusal: `LEAQ
// ·leafPrimes(SB), AX` publishes.
// MUTATION (address-constant): in leafSymbolReads, delete the `operand[0] ==
// "$"` refusal: `MOVQ $·leafPrimes(SB), AX` refuses only for its operand
// form, without "takes the address of a symbol".
// MUTATION (exchange): in leafSymbolReads, delete the leafExchanges refusal:
// `XCHGQ ·leafPrimes(SB), AX` publishes.
// MUTATION (other-package): in leafSymbolTarget, delete the
// another-package refusal: the runtime symbol is looked up as a table and
// refuses only as undefined, without "another package's symbol".
// MUTATION (writable-table): in leafReadOnlyTable, delete the RODATA
// refusal: the NOPTR-only table publishes.
// MUTATION (address-table): in leafReadOnlyTable, delete the DATA address
// refusal: a RODATA table holding `$·install(SB)` publishes.
// MUTATION (function-symbol): in leafReadOnlyTable, delete the
// `index.declarations[name]` refusal: `MOVQ ·install(SB), AX` refuses only
// as undefined, without "names a function".
// MUTATION (indexed): in leafSymbolOperand, accept any tail after the
// symbol: `MOVQ leafTable<>(SB)(AX*8), AX` publishes.
func TestCommittedAssemblyLeafSymbolControlsRefuse(t *testing.T) {
	const variables = "var leafPrimes = [...]uint64{1, 2, 3}"
	addressTable := "#include \"textflag.h\"\n\nDATA leafCode<>+0(SB)/8, $·install(SB)\nGLOBL leafCode<>(SB), RODATA, $8\n\n" +
		"TEXT ·inventoryLeafRead(SB), NOSPLIT|NOFRAME, $0-8\n\tMOVQ leafCode<>(SB), AX\n\tMOVQ target+0(FP), CX\n\tMOVQ AX, 8(CX)\n\tRET\n"
	for _, test := range []struct{ name, unit, cause string }{
		{"write-destination", asmLeafReadBody("MOVQ AX, ·leafPrimes+0(SB)"), "writes symbol memory"},
		{"lea", asmLeafReadBody("LEAQ ·leafPrimes(SB), AX"), "takes the address of a symbol"},
		{"address-constant", asmLeafReadBody("MOVQ $·leafPrimes(SB), AX"), "takes the address of a symbol"},
		{"exchange", asmLeafReadBody("XCHGQ ·leafPrimes(SB), AX"), "writes every operand"},
		{"other-package", asmLeafReadBody("MOVQ runtime·inventoryValue(SB), AX"), "another package's symbol"},
		{"writable-table", asmLeafReadBody("MOVQ leafPlain<>(SB), AX"), "is not RODATA"},
		{"address-table", addressTable, "holds a symbol address"},
		{"function-symbol", asmLeafReadBody("MOVQ ·install(SB), AX"), "names a function"},
		{"indexed", asmLeafReadBody("MOVQ leafTable<>(SB)(AX*8), AX"), "unsupported address form"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(test.name+"/"+kind, func(t *testing.T) {
				fixture := asmLeafFixture(t, kind, test.unit, asmLeafReadDecl(variables), asmLeafReadCall)
				asmLeafRefusal(t, fixture, test.cause)
			})
		}
	}
}

// TestCommittedAssemblyLeafConditionalArms requires a leaf body that chooses
// an instruction by a build predicate to publish when every arm is a leaf, as
// zstd's matchLen picks TZCNTQ or BSFQ under #ifdef GOAMD64_v3, and to refuse
// when one arm is not: an arm that calls, a body whose only RET sits inside
// an arm, and a macro defined inside an arm, which the unit census refuses
// before the body is read. The predicate is never read.
//
// MUTATION: (internal/le/repo/inventory/assemblyleaf.go,
// `			if conditional, err := blocks.accept(statement.tokens); conditional {`,
// `			if conditional, err := false, error(nil); conditional {`): no
// conditional is tracked, the RET inside the else arm terminates the body,
// and the arm-only-return shape publishes.
// MUTATION: (internal/le/repo/inventory/assemblyleaf.go,
// `			walk.terminated = false`, `			_ = walk`): the RET inside the
// else arm terminates the body, and the arm-only-return shape publishes.
func TestCommittedAssemblyLeafConditionalArms(t *testing.T) {
	const header = "#include \"textflag.h\"\nTEXT ·inventoryLeaf(SB),NOSPLIT,$0-2\n"
	const results = " MOVB $0, bmi1+0(FP)\n MOVB $0, bmi2+1(FP)\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run("matchlen/"+kind, func(t *testing.T) {
			unit := header + results + "#ifdef GOAMD64_v3\n TZCNTQ BX, BX\n#else\n BSFQ BX, BX\n#endif\n RET\n"
			fixture := asmLeafFixture(t, kind, unit, asmLeafResultsDecl, "_, _ = inventoryLeaf()")
			externalEffectsPositive(t, fixture)
		})
		for _, refused := range []struct{ name, body, cause string }{
			{"call-in-arm", "#ifdef GOAMD64_v3\n TZCNTQ BX, BX\n#else\n CALL ·inventoryOther(SB)\n#endif\n RET\n", "assembly leaf body calls through CALL"},
			{"arm-only-return", "#ifdef GOAMD64_v3\n XORQ AX, AX\n#else\n RET\n#endif\n", "assembly leaf body can run past its end"},
			{"define-in-arm", "#ifdef GOAMD64_v3\n#define SHIFT BX\n#endif\n RET\n", "conditional assembly changes namespace through define"},
		} {
			t.Run(refused.name+"/"+kind, func(t *testing.T) {
				fixture := asmLeafFixture(t, kind, header+results+refused.body, asmLeafResultsDecl, "_, _ = inventoryLeaf()")
				asmLeafRefusal(t, fixture, refused.cause)
			})
		}
	}
}
