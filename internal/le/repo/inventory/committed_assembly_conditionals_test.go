// Design: docs/architecture/plugin/feature-gates.md -- branch-neutral namespace evidence is not entered execution proof.
package repoinventory

import "testing"

func asmConditionalProgram(directive, first, second string) string {
	return directive + "\n" + first + "#else\n" + second + "#endif\n"
}

func asmConditionalUnit(block string) string {
	return "#include \"textflag.h\"\nTEXT ·unentered(SB),4,$0-0\n" + block + " RET\n"
}

// TestCommittedAssemblyConditionalNamespacesPreserveReport admits every arm's
// namespace evidence without selecting a target/predicate value. Instructions
// and function-local labels may differ; linker definitions and bindings may not.
func TestCommittedAssemblyConditionalNamespacesPreserveReport(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{"unknown-predicate", asmConditionalProgram("#ifdef UNKNOWN_PREDICATE", " MOVQ $1, AX\n", " XORQ AX, AX\n")},
		{"known-present", asmConditionalProgram("#ifdef GOAMD64_v1", " MOVQ $1, AX\n", " XORQ AX, AX\n")},
		{"known-absent", asmConditionalProgram("#ifdef GOAMD64_v3", " MOVQ $1, AX\n", " XORQ AX, AX\n")},
		{"ifndef", asmConditionalProgram("#ifndef GOAMD64_v1", " MOVQ $1, AX\n", " XORQ AX, AX\n")},
		{"nested-local-labels", asmConditionalProgram("#ifdef OUTER", " first: MOVQ $1, AX\n"+asmConditionalProgram("#ifndef INNER", " XORQ BX, BX\n", " MOVQ $2, BX\n"), " second: XORQ AX, AX\n")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(test.block))
		})
	}
}

// TestCommittedAssemblyConditionalNamespaceMutationsRefuse inspects BOTH arms,
// including the arm the declared GOAMD64_v1 predicate would not execute.
func TestCommittedAssemblyConditionalNamespaceMutationsRefuse(t *testing.T) {
	cases := []struct {
		name      string
		statement string
	}{
		{"text", " TEXT syscall·Syscall(SB),4,$0-56\n JMP ·unentered(SB)\n"},
		{"labelled-text", " hidden: TEXT syscall·Syscall(SB),4,$0-56\n JMP ·unentered(SB)\n"},
		{"global", " GLOBL syscall·Syscall(SB),$8\n"},
		{"data", " DATA syscall·Syscall+0(SB)/8,$0\n"},
		{"assignment", " value = 1\n"},
		{"define", "#define operand AX\n"},
		{"include", "#include \"textflag.h\"\n"},
	}
	for _, test := range cases {
		for _, arm := range []string{"first", "second"} {
			t.Run(test.name+"/"+arm, func(t *testing.T) {
				first, second := " MOVQ $1, AX\n", " XORQ AX, AX\n"
				fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(asmConditionalProgram("#ifdef GOAMD64_v1", first, second)))
				if arm == "first" {
					first = test.statement
				} else {
					second = test.statement
				}
				write(t, fixture.root, asmHeaderUnit, asmConditionalUnit(asmConditionalProgram("#ifdef GOAMD64_v1", first, second)))
				commitInventoryFixture(t, fixture.root)
				asmHeaderRefusal(t, fixture, "conditional assembly changes namespace")
			})
		}
	}
}

// TestCommittedAssemblyConditionalActualOriginsRefuse preserves full-token C/D
// through the conditional certificate: an opcode supplied as a macro actual
// cannot disappear by selecting only the other branch.
func TestCommittedAssemblyConditionalActualOriginsRefuse(t *testing.T) {
	for _, arm := range []string{"first", "second"} {
		t.Run(arm, func(t *testing.T) {
			prefix := "#include \"textflag.h\"\n#define dispatch(op) op\nTEXT ·unentered(SB),4,$0-0\n"
			first, second := " dispatch(MOVQ) $1, AX\n", " XORQ AX, AX\n"
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, prefix+asmConditionalProgram("#ifdef GOAMD64_v1", first, second)+" RET\n")
			hidden := " dispatch(TEXT) syscall·Syscall(SB),4,$0-56\n"
			if arm == "first" {
				first = hidden
			} else {
				second = hidden
			}
			write(t, fixture.root, asmHeaderUnit, prefix+asmConditionalProgram("#ifdef GOAMD64_v1", first, second)+" RET\n")
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, "raw definition word is hidden outside the literal opcode position")
		})
	}
}

// TestCommittedAssemblyConditionalStructureRefuse rejects malformed structure,
// rather than treating an unknown directive or truncated block as a safe arm.
func TestCommittedAssemblyConditionalStructureRefuse(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{"unterminated", "#ifdef UNKNOWN\n MOVQ $1, AX\n"},
		{"else-without-if", "#else\n"},
		{"endif-without-if", "#endif\n"},
		{"repeated-else", "#ifdef UNKNOWN\n#else\n#else\n#endif\n"},
		{"missing-predicate", "#ifdef\n#endif\n"},
		{"nonidentifier-predicate", "#ifdef 1\n#endif\n"},
		{"extra-predicate", "#ifdef UNKNOWN EXTRA\n#endif\n"},
		{"else-operand", "#ifdef UNKNOWN\n#else EXTRA\n#endif\n"},
		{"endif-operand", "#ifdef UNKNOWN\n#endif EXTRA\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			positive := asmConditionalProgram("#ifdef UNKNOWN", " MOVQ $1, AX\n", " XORQ AX, AX\n")
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(positive))
			write(t, fixture.root, asmHeaderUnit, asmConditionalUnit(test.block))
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, "malformed assembly conditional")
		})
	}
}

// TestCommittedAssemblyEnteredConditionalsRefuse keeps the consuming body proof
// strict even when a global namespace certificate admitted the SAME marker
// structure. Empty branches and post-JMP markers must not be skipped as '#'.
func TestCommittedAssemblyEnteredConditionalsRefuse(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		unrelated string
	}{
		{"single-conditional-jmp", "#ifdef UNKNOWN\n JMP syscall·Syscall(SB)\n#endif\n", "#ifdef UNKNOWN\n MOVQ $1, AX\n#endif\n"},
		{"one-empty-arm", asmConditionalProgram("#ifdef UNKNOWN", " JMP syscall·Syscall(SB)\n", ""), asmConditionalProgram("#ifdef UNKNOWN", " MOVQ $1, AX\n", "")},
		{"empty-before-jmp", "#ifdef UNKNOWN\n#endif\n JMP syscall·Syscall(SB)\n", "#ifdef UNKNOWN\n#endif\n"},
		{"empty-after-jmp", " JMP syscall·Syscall(SB)\n#ifdef UNKNOWN\n#endif\n", "#ifdef UNKNOWN\n#endif\n"},
		{"neutral-after-jmp", " JMP syscall·Syscall(SB)\n" + asmConditionalProgram("#ifndef UNKNOWN", " MOVQ $1, AX\n", " XORQ AX, AX\n"), asmConditionalProgram("#ifndef UNKNOWN", " MOVQ $1, AX\n", " XORQ AX, AX\n")},
		{"nested-empty", "#ifdef OUTER\n#ifndef INNER\n#endif\n#endif\n JMP syscall·Syscall(SB)\n", "#ifdef OUTER\n#ifndef INNER\n#endif\n#endif\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(test.unrelated))
			write(t, fixture.root, asmTargetFile, "#include \"textflag.h\"\nTEXT ·inventoryGate(SB),NOSPLIT,$0-56\n"+test.body)
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, "entered assembly body contains conditional")
		})
	}
}

// TestCommittedAssemblyConditionalSnapshotRetainsEvidence anchors both source
// bytes and selected Go paths to one snapshot across dirty writes and later HEAD.
func TestCommittedAssemblyConditionalSnapshotRetainsEvidence(t *testing.T) {
	positive := asmConditionalProgram("#ifdef GOAMD64_v1", " MOVQ $1, AX\n", " XORQ AX, AX\n")
	fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(positive))
	expected := asmHeaderExpected(fixture)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	revision := snapshot.Revision()
	bad := asmConditionalProgram("#ifdef GOAMD64_v1", " MOVQ $1, AX\n", " hidden: TEXT syscall·Syscall(SB),4,$0-56\n JMP ·unentered(SB)\n")
	write(t, fixture.root, asmHeaderUnit, asmConditionalUnit(bad))
	later := asmHeaderProbeDir + "/later_conditional.go"
	fixture.goFile(t, later, "package headerprobe\nfunc LaterConditionalMarker() {}\n")
	asmHeaderPositive(t, expected, asmHeaderPaths())
	plugins, err := snapshot.Plugins()
	if err != nil {
		t.Fatalf("dirty conditional source changed retained membership: %v", err)
	}
	expected.checkPlugins(t, plugins)
	asmHeaderSelectedPaths(t, snapshot, asmHeaderPaths())
	commitInventoryFixture(t, fixture.root)
	if snapshot.Revision() != revision {
		t.Fatal("retained conditional snapshot revision moved")
	}
	fresh, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Revision() == revision {
		t.Fatal("later committed conditional mutation did not change HEAD")
	}
	asmHeaderSelectedPaths(t, fresh, asmHeaderPaths(later))
	asmHeaderRefusal(t, fixture, "conditional assembly changes namespace")
	plugins, err = snapshot.Plugins()
	if err != nil {
		t.Fatalf("later HEAD changed retained conditional membership: %v", err)
	}
	expected.checkPlugins(t, plugins)
	asmHeaderSelectedPaths(t, snapshot, asmHeaderPaths())
}

// TestCommittedAssemblyConditionalUnsupportedDirectivesRefuse prevents an
// unsupported directive from being skipped or evaluated inside a neutral block.
// Its own directive cause must precede any incidental nesting diagnostic.
func TestCommittedAssemblyConditionalUnsupportedDirectivesRefuse(t *testing.T) {
	cases := []struct {
		name      string
		statement string
	}{
		{"if", "#if 1\n MOVQ $2, BX\n#endif\n"},
		{"elif", "#elif OTHER\n MOVQ $2, BX\n"},
		{"undef", "#undef operand\n"},
	}
	for _, test := range cases {
		for _, arm := range []string{"first", "second"} {
			t.Run(test.name+"/"+arm, func(t *testing.T) {
				first, second := " MOVQ $1, AX\n", " XORQ AX, AX\n"
				fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(asmConditionalProgram("#ifdef GOAMD64_v1", first, second)))
				if arm == "first" {
					first = test.statement
				} else {
					second = test.statement
				}
				write(t, fixture.root, asmHeaderUnit, asmConditionalUnit(asmConditionalProgram("#ifdef GOAMD64_v1", first, second)))
				commitInventoryFixture(t, fixture.root)
				// The public problem wrapper terminates the cause with ';':
				// requiring that boundary keeps "if" from matching "ifdef".
				asmHeaderRefusal(t, fixture, "unsupported assembly preprocessing "+test.name+";")
			})
		}
	}
}

// TestCommittedAssemblyConditionalInactiveIfndefRefuses keeps the inactive
// first arm visible even when #ifndef's declared predicate is already false.
func TestCommittedAssemblyConditionalInactiveIfndefRefuses(t *testing.T) {
	first, second := " MOVQ $1, AX\n", " XORQ AX, AX\n"
	fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmConditionalUnit(asmConditionalProgram("#ifndef GOAMD64_v1", first, second)))
	first = " hidden: TEXT syscall·Syscall(SB),4,$0-56\n JMP ·unentered(SB)\n"
	write(t, fixture.root, asmHeaderUnit, asmConditionalUnit(asmConditionalProgram("#ifndef GOAMD64_v1", first, second)))
	commitInventoryFixture(t, fixture.root)
	asmHeaderRefusal(t, fixture, "conditional assembly changes namespace")
}
