// Design: docs/architecture/testing/test-health.md -- exact lexical function boundaries.
// Related: semantic_scope_test.go -- approval selection against committed source.
package testweakened

import (
	"strings"
	"testing"
)

// TestFunctionUnitBoundaries checks declarations with signature and body braces,
// while keeping comments and literals out of the declaration inventory.
func TestFunctionUnitBoundaries(t *testing.T) {
	for _, declaration := range []string{
		"func TestA(t *testing.T) { require.Equal(t, 1, got) }",
		"func (r *Receiver[T]) TestA(t *testing.T) { require.Equal(t, 1, got) }",
		"func TestA[T interface{ M() }](t T) struct{ X int } { return struct{ X int }{} }",
		"func TestA() interface{ M() struct{ X int } } { return nil }",
		"func TestA() (x struct{ X int }, y interface{ M() }) { return }",
		"func TestA() func() struct{ X int } { return func() struct{ X int } { return struct{ X int }{} } }",
		"func TestA() { f := func() { if true { println(1) } }; f() }",
		"func TestA() { /* } func Fake() {} */ println(\"} func Fake() {}\", '}') }",
		"func TestA() { println(`\nfunc Fake() {}\n}\n`) }",
	} {
		t.Run(declaration, func(t *testing.T) {
			const doc = "// Attached declaration comment.\n/* Attached block comment. */\n"
			content := doc + declaration
			units := unitsByName(content + "\n\n// Detached comment.\n\nvar outside = 1\n")
			if len(units) != 1 || units[0].name != "TestA" || units[0].text != content {
				t.Fatalf("units = %#v, want exactly TestA with text %q", units, content)
			}
		})
	}
}

// TestFunctionUnitInventory checks fake declarations and repeated method names
// without changing the source-order grouping used by structural attribution.
func TestFunctionUnitInventory(t *testing.T) {
	const first = "func (a A) TestSame() { require.Equal(t, 1, got) }"
	const second = "func (b B) TestSame() { require.Equal(t, 2, got) }"
	content := "package p\n/*\nfunc Fake() {}\n*/\nvar text = `\nfunc Fake() {}\n`\n" +
		"var callback = func() { println(1) }\ntype Callback func()\n" + first + "\n\n" + second + "\n"
	units := unitsByName(content)
	if len(units) != 1 || units[0].name != "TestSame" || units[0].text != first+"\n"+second {
		t.Fatalf("inventory = %#v", units)
	}
}

// TestIncompleteFunctionRetainsStructuralFinding ensures partial fragments
// still reach the whole-text detector when exact unit attribution is impossible.
func TestIncompleteFunctionRetainsStructuralFinding(t *testing.T) {
	const before = "func TestA() {\nrequire.Equal(t, 1, got)\nrequire.NoError(t, err)\n"
	const after = "func TestA() {\nrequire.Equal(t, 1, got)\n"
	verdicts := weakenedUnits("pkg/partial_test.go", before, after)
	if len(verdicts) != 1 || verdicts[0].name != "partial_test" ||
		!strings.Contains(strings.Join(verdicts[0].details, "\n"), "removing assertions") {
		t.Fatalf("partial fragment lost file-scoped finding: %#v", verdicts)
	}
}

// TestCheckIncompleteFunctionRefusesSemanticApproval checks that a truncated
// selected declaration cannot spend a reserved verdict as a known unit change.
func TestCheckIncompleteFunctionRefusesSemanticApproval(t *testing.T) {
	for _, after := range []string{
		"package a\nfunc TestA(t *testing.T) { require.Equal(t, 2, got)\n",
		"package a\nfunc TestA(t *testing.T) (struct{ X int }\n",
		"package a\n/* func TestA(t *testing.T) {}",
		"package a\nfunc",
		"package a\nfunc (r Receiver)\n",
		"package a\nfunc TestA() { println(\"unterminated) }\n",
		"package a\nfunc TestA() { require.Equal(t, 1, got) }\nfunc TestA() {\n",
	} {
		t.Run(after, func(t *testing.T) {
			root := newParityRepository(t)
			const path = "pkg/a_test.go"
			// The judge rules before the edit, as the edit hook requires.
			if _, err := Approve(root, fixtureSession, "pkg.TestA", "reserved verdict"); err != nil {
				t.Fatal(err)
			}
			writeParityFile(t, root, path, after)
			result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
			if result.ExitCode() != 2 || !strings.Contains(strings.Join(result.Problems, "\n"), "cannot judge Go function boundaries") {
				t.Fatalf("incomplete source was not refused: %#v", result)
			}
			if len(result.Approvals.Trailers) != 0 || len(result.Approvals.Dropped) != 0 {
				t.Fatalf("incomplete source spent an approval: %#v", result.Approvals)
			}
		})
	}
}

// TestFunctionUnitScannerErrorsStayLocal checks that malformed unrelated bytes
// cannot change a closed declaration, and that CRLF comments retain their bytes.
func TestFunctionUnitScannerErrorsStayLocal(t *testing.T) {
	const declaration = "// Attached.\r\n/* Block\r\n comment. */\r\nfunc TestA() {}"
	const following = "\n\ufffd\n"
	before := declaration + following
	after := declaration + "\n\n" + following
	changed, err := changedUnitNames("pkg/a_test.go", before, after)
	if err != nil || len(changed) != 0 {
		t.Fatalf("unrelated scanner diagnostic changed exact unit: %v, %v", changed, err)
	}
	if text := unitText(unitsByName(before), "TestA"); text != declaration {
		t.Fatalf("CRLF declaration = %q, want %q", text, declaration)
	}
}

// TestCheckIncompleteBoundaryKeepsStructuralEvidence ensures a cannot-judge
// semantic result still reports the assertion loss found in the selected text.
func TestCheckIncompleteBoundaryKeepsStructuralEvidence(t *testing.T) {
	root := newParityRepository(t)
	const path = "pkg/a_test.go"
	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) {\n")
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	if result.ExitCode() != 2 || len(result.Findings) != 1 || result.Findings[0].Name != "a_test" {
		t.Fatalf("cannot-judge lost structural evidence: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Findings[0].Details, "\n"), "removing assertions") {
		t.Fatalf("assertion loss absent: %#v", result.Findings)
	}
}

// TestSemanticBoundaryUnknownBaseline refuses to treat an unreadable baseline
// inventory as proof that its replacement consists only of new functions.
func TestSemanticBoundaryUnknownBaseline(t *testing.T) {
	for _, baseline := range []string{
		"/* unterminated",
		"func TestA(t *testing.T) {",
	} {
		changed, err := changedUnitNames("pkg/a_test.go", baseline, "func TestA(t *testing.T) {}")
		if err == nil || len(changed) != 0 {
			t.Fatalf("unknown baseline %q became known changes %v: %v", baseline, changed, err)
		}
	}
}

// TestFunctionUnitIncompleteSignatures exercises truncated and mismatched
// delimiters without manufacturing a function end from the available text.
func TestFunctionUnitIncompleteSignatures(t *testing.T) {
	for _, content := range []string{
		"func (receiver",
		"func 123",
		"func TestA[T",
		"func TestA[T any]",
		"func TestA;",
		"func TestA(",
		"func TestA() struct",
		"func TestA() struct wrong",
		"func TestA() )",
		"func TestA() { ] }",
		"func TestA() { println(\"\\q\") }",
	} {
		t.Run(content, func(t *testing.T) {
			units := unitsByName(content)
			if len(units) == 0 {
				t.Fatal("incomplete declaration silently became an empty inventory")
			}
			for _, unit := range units {
				if !unit.incomplete || unit.text != "" {
					t.Fatalf("incomplete source became an exact unit: %#v", unit)
				}
			}
		})
	}
}

// TestFunctionUnitBodylessDeclarations checks exact signature ends, including EOF,
// and proves a complete external declaration does not hide a following test.
func TestFunctionUnitBodylessDeclarations(t *testing.T) {
	for _, declaration := range []string{
		"func external()",
		"func external() int",
		"func external() (int, error)",
		"func external() struct{ X int }",
		"func external() func() interface{ M() }",
		"func (r Receiver) external()",
	} {
		for _, suffix := range []string{"", "\n", ";", " // Trailing comment.\n", "\nfunc TestA() {}\n"} {
			t.Run(declaration+suffix, func(t *testing.T) {
				units := unitsByName(declaration + suffix)
				if len(units) == 0 || units[0].incomplete || units[0].name != "external" || units[0].text != declaration {
					t.Fatalf("bodyless declaration not exact: %#v", units)
				}
				if strings.Contains(suffix, "TestA") {
					if len(units) != 2 || units[1].name != "TestA" || units[1].text != "func TestA() {}" {
						t.Fatalf("bodyless declaration hid later test: %#v", units)
					}
				}
			})
		}
	}
}

// TestFunctionUnitTruncatedResultIsNotBodyless rejects signatures that end in a
// semicolon but still lack a result type, rather than accepting them as external.
func TestFunctionUnitTruncatedResultIsNotBodyless(t *testing.T) {
	for _, signature := range []string{
		"func external() pkg.",
		"func external() *",
		"func external() map[string]",
		"func external() func",
		"func external() chan",
	} {
		t.Run(signature, func(t *testing.T) {
			units := unitsByName(signature + ";\nfunc TestA() {}\n")
			if len(units) != 1 || units[0].name != "external" || !units[0].incomplete || units[0].text != "" {
				t.Fatalf("truncated signature became a complete declaration: %#v", units)
			}
		})
	}
}

// TestFunctionUnitDetachedComments leaves comments across blank lines or after
// other code outside the following declaration, and excludes anonymous funcs.
func TestFunctionUnitDetachedComments(t *testing.T) {
	for _, prefix := range []string{
		"// Detached.\n\n",
		"var x = 1 // Trailing.\n",
		"func() func() {}\n",
		"func(x int) Result {}\n",
	} {
		const declaration = "func TestA() {}"
		units := unitsByName(prefix + declaration)
		if len(units) != 1 || units[0].name != "TestA" || units[0].text != declaration {
			t.Fatalf("prefix %q changed declaration inventory: %#v", prefix, units)
		}
	}
}
