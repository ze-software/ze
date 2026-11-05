// Design: docs/architecture/plugin/feature-gates.md -- macro namespace evidence is not instruction execution.
package repoinventory

import (
	"testing"
)

const asmMacroEntry = "TEXT ·inventoryGate(SB),NOSPLIT,$0-56\n JMP syscall·Syscall(SB)\n"
const asmOperandMacros = `#define lane AX
#define item BX
#define blend(acc, value) \
 IMULQ $3, value \
 ADDQ value, acc
#define fold(value) \
 blend(lane, value) \
 XORQ value, lane
#define walk() \
 again: \
 MOVQ $1, item \
 fold(item) \
 CMPQ lane, item \
 JNE again
`
const asmMacroOtherBody = `TEXT ·arithmeticOnly(SB),NOSPLIT|NOFRAME,$0-0
 walk()
 CALL runtime·entersyscall(SB)
 MOVQ lane, item
 RET
`

// TestCommittedAssemblyMacroNamespacesPreserveFullReport admits source-backed
// unrelated instruction macros without executing or expanding them. Register
// aliases, nested calls, continued logical declarations and label production
// must not add ASM to report inputs or remove exact useful report metadata.
func TestCommittedAssemblyMacroNamespacesPreserveFullReport(t *testing.T) {
	for _, placement := range []string{"separate-unit", "same-unit-before-entry", "same-unit-after-entry", "separate-package", "function-like-adjacency", "object-like-space", "object-like-comment", "multiline-comment-formals", "tokens-not-concatenated"} {
		t.Run(placement, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			unit := asmOperandMacros + asmMacroOtherBody
			file := asmOrdinaryDir + "/operands_linux_amd64.s"
			switch placement {
			case "same-unit-before-entry":
				write(t, fixture.root, asmTargetFile, "#include \"textflag.h\"\n"+unit+asmMacroEntry)
			case "same-unit-after-entry":
				write(t, fixture.root, asmTargetFile, "#include \"textflag.h\"\n"+asmMacroEntry+unit)
			case "separate-package":
				fixture.goFile(t, asmOrdinaryDir+"/import_arithmetic.go", "package ordinary\nimport _ \"github.com/ze-software/ze/internal/arithmetic\"\n")
				fixture.goFile(t, "internal/arithmetic/arithmetic.go", "package arithmetic\nfunc marker() {}\n")
				file = "internal/arithmetic/operands_linux_amd64.s"
			case "function-like-adjacency":
				unit = "#define operand(value) value\nTEXT ·arithmeticOnly(SB),NOSPLIT,$0-0\n MOVQ operand(AX), BX\n RET\n"
			case "object-like-space":
				unit = "#define operand (AX)\nTEXT ·arithmeticOnly(SB),NOSPLIT,$0-0\n MOVQ operand, BX\n RET\n"
			case "object-like-comment":
				unit = "#define operand/* retained\n comment */ (AX)\nTEXT ·arithmeticOnly(SB),NOSPLIT,$0-0\n MOVQ operand, BX\n RET\n"
			case "multiline-comment-formals":
				unit = "#define blend(acc, /* retained\n comment */ value) \\\n IMULQ $3, value \\\n ADDQ value, acc\nTEXT ·arithmeticOnly(SB),NOSPLIT,$0-0\n blend(AX, BX)\n RET\n"
			case "tokens-not-concatenated":
				// This unused valid definition contains TE and XT as DISTINCT
				// tokens. The assembler never re-lexes their joined spelling.
				unit = "#define pieces TE XT\n#define TEXTual AX\nTEXT ·arithmeticOnly(SB),NOSPLIT,$0-0\n MOVQ TEXTual, BX\n RET\n"
			}
			if placement != "same-unit-before-entry" && placement != "same-unit-after-entry" {
				write(t, fixture.root, file, "#include \"textflag.h\"\n"+unit)
			}
			commitInventoryFixture(t, fixture.root)
			inv, err := Collect(fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			fixture.checkReport(t, inv)
			snapshot, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			asmCheckSourcesStayGo(t, snapshot, len(fixture.goSources))
		})
	}
}

// TestCommittedAssemblyMacroDefinitionOriginsRefuse preserves C/D together:
// a forbidden definition word can originate in a replacement, formal, actual
// argument or prefix-hidden raw statement. Old preprocessing refusal may mask
// these negatives; logged causes must not be called exploit-red evidence.
func TestCommittedAssemblyMacroDefinitionOriginsRefuse(t *testing.T) {
	cases := map[string]string{
		"redefine-text":          "#define TEXT JMP\n",
		"redefine-globl":         "#define GLOBL JMP\n",
		"redefine-data":          "#define DATA JMP\n",
		"replacement-text":       "#define create TEXT syscall·Syscall(SB),4,$0-56\ncreate\n JMP ·installHidden(SB)\n",
		"replacement-globl":      "#define create GLOBL syscall·Syscall(SB),16,$8\ncreate\n",
		"replacement-data":       "#define create DATA syscall·Syscall+0(SB)/8,$0\ncreate\n",
		"continued-definition":   "#define create() \\\n owned: \\\n TEXT syscall·Syscall(SB),4,$0-56 \\\n JMP ·installHidden(SB)\ncreate()\n",
		"nested-definition":      "#define outer inner\n#define inner TEXT syscall·Syscall(SB),4,$0-56\nouter\n JMP ·installHidden(SB)\n",
		"formal-definition-word": "#define dispatch(TEXT) TEXT\ndispatch(TEXT) syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"actual-text":            "#define dispatch(op) op\ndispatch(TEXT) syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"actual-globl":           "#define dispatch(op) op\ndispatch(GLOBL) syscall·Syscall(SB),16,$8\n",
		"actual-data":            "#define dispatch(op) op\ndispatch(DATA) syscall·Syscall+0(SB)/8,$0\n",
		"nested-actual":          "#define dispatch(op) op\n#define nested(op) dispatch(op)\nnested(TEXT) syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"empty-prefix":           "#define empty\nempty TEXT syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"label-prefix":           "#define prefix owned:\nprefix TEXT syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"newline-prefix":         "#define prefix() \\\n owned:\nprefix() TEXT syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"directive-token":        "#define directive # define TEXT JMP\n",
		"token-pasting":          "#define join(a, b) a ## b\njoin(TE, XT) syscall·Syscall(SB),4,$0-56\n",
		"redefinition":           "#define operand AX\n#define operand BX\n",
		"malformed-escape":       "#define operand \\x\n",
	}
	for name, unit := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
			write(t, fixture.root, asmOrdinaryDir+"/origins_linux_amd64.s", "#include \"textflag.h\"\n"+unit)
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}

// TestCommittedAssemblyMacroHeaderAndBodyStabilityRefuse preserves E/F even
// when no macro emits a definition opcode. Whole-unit later definitions count;
// namespace safety does not certify an expanded entered JMP or its operands.
func TestCommittedAssemblyMacroHeaderAndBodyStabilityRefuse(t *testing.T) {
	cases := map[string]string{
		"entry-name":              "#define ·inventoryGate ·different\n" + asmMacroEntry,
		"foreign-definition-name": "#define alias syscall·Syscall\n" + asmMacroEntry + "TEXT alias(SB),4,$0-56\n JMP ·installHidden(SB)\n",
		"address-register":        "#define SB AX\n" + asmMacroEntry,
		"flags":                   "#define FLAGS 4\nTEXT ·inventoryGate(SB),FLAGS,$0-56\n JMP syscall·Syscall(SB)\n",
		"frame":                   "#define ZERO 0\nTEXT ·inventoryGate(SB),4,$ZERO-56\n JMP syscall·Syscall(SB)\n",
		"later-entry-name":        asmMacroEntry + "#define ·inventoryGate ·different\n",
		"later-frame-token":       asmMacroEntry + "#define SB AX\n",
		"entered-opcode":          "#define JMP CALL\n" + asmMacroEntry,
		"later-entered-opcode":    asmMacroEntry + "#define JMP CALL\n",
		"entered-operand":         "#define syscall·Syscall ·installHidden\n" + asmMacroEntry,
		"later-entered-operand":   asmMacroEntry + "#define syscall·Syscall ·installHidden\n",
		"entered-function-macro":  "#define transfer(target) JMP target\nTEXT ·inventoryGate(SB),4,$0-56\n transfer(syscall·Syscall)\n",
	}
	for name, unit := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
			write(t, fixture.root, asmTargetFile, "#include \"textflag.h\"\n"+unit)
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}

// TestCommittedAssemblyMacroSnapshotRetainsEvidence proves macro source comes
// from the captured immutable batch, not a dirty reopen or later HEAD. Exact
// useful output survives dirty bytes; a fresh incompatible committed namespace
// is refused while the retained snapshot still describes its original plugin.
func TestCommittedAssemblyMacroSnapshotRetainsEvidence(t *testing.T) {
	fixture := newASMInventoryFixture(t)
	fixture.addBoundary(t, "syscall")
	file := asmOrdinaryDir + "/operands_linux_amd64.s"
	write(t, fixture.root, file, "#include \"textflag.h\"\n"+asmOperandMacros+asmMacroOtherBody)
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	revision := snapshot.Revision()
	write(t, fixture.root, file, "#define dispatch(op) op\ndispatch(TEXT) syscall·Syscall(SB),4,$0-56\n RET\n")
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkReport(t, inv)
	plugins, err := snapshot.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkPlugins(t, plugins)
	commitInventoryFixture(t, fixture.root)
	if snapshot.Revision() != revision {
		t.Fatal("macro snapshot revision moved")
	}
	plugins, err = snapshot.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkPlugins(t, plugins)
	asmCheckSourcesStayGo(t, snapshot, len(fixture.goSources))
	asmExpectRefusal(t, fixture)
}
