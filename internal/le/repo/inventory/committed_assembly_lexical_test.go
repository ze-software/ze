// Design: docs/architecture/plugin/feature-gates.md -- assembler labels and comments retain real statement semantics.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedAssemblyLabelPrefixesCannotHideDefinitions drives public Collect
// with leading labels accepted by the real assembler. Foreign TEXT, GLOBL and
// DATA ownership must not disappear from the selected symbol census.
func TestCommittedAssemblyLabelPrefixesCannotHideDefinitions(t *testing.T) {
	for _, kind := range []string{"text-other-file", "text-other-package", "globally-defined-data", "data-initialization"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			assembly := "owned: TEXT syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n"
			file := asmOrdinaryDir + "/labelled_linux_amd64.s"
			switch kind {
			case "text-other-file":
				fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
			case "text-other-package":
				fixture.goFile(t, asmOrdinaryDir+"/import_foreign.go", "package ordinary\nimport _ \"github.com/ze-software/ze/internal/foreign\"\n")
				fixture.goFile(t, "internal/foreign/hidden.go", asmHiddenPluginGo("foreign"))
				file = "internal/foreign/labelled_linux_amd64.s"
			case "globally-defined-data":
				assembly = "owned: GLOBL syscall·Syscall(SB),16,$8\n"
			case "data-initialization":
				assembly = "owned: DATA syscall·Syscall+0(SB)/8,$0\n"
			}
			write(t, fixture.root, file, assembly)
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}

// TestCommittedAssemblyMultilineCommentsPreserveStatements keeps exact report
// controls while a block comment contains physical newlines inside a TEXT
// header or between JMP and its operand. The assembler skips those comments
// without introducing statement separators.
func TestCommittedAssemblyMultilineCommentsPreserveStatements(t *testing.T) {
	for _, placement := range []string{"after-text", "inside-header", "after-jmp", "inside-destination"} {
		t.Run(placement, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			body := asmTargetBody
			switch placement {
			case "after-text":
				body = strings.Replace(body, "TEXT ·inventoryGate", "TEXT /* retained\n block */ ·inventoryGate", 1)
			case "inside-header":
				body = strings.Replace(body, ",NOSPLIT,", ",/* retained\n block */NOSPLIT,", 1)
			case "after-jmp":
				body = strings.Replace(body, "JMP syscall", "JMP /* retained\n block */ syscall", 1)
			case "inside-destination":
				body = strings.Replace(body, "syscall·Syscall(SB)", "syscall·Syscall/* retained\n block */(SB)", 1)
			}
			write(t, fixture.root, asmTargetFile, body)
			commitInventoryFixture(t, fixture.root)
			inv, err := Collect(fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			fixture.checkReport(t, inv)
		})
	}
}
