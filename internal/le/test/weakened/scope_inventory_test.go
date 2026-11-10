// Design: docs/architecture/testing/test-health.md -- one Go unit inventory for both gates.
// Related: scope_go.go -- goUnits, the weakening check's reading of rfc.DeclaredUnits.
package testweakened

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestWeakeningAndRFCGatesReadOneGoUnitPopulation proves the weakening check
// names exactly the Go units the RFC gate names: the same names, in the same
// order, with byte-identical text.
//
// METHOD: differential, over written-out shapes on which the deleted lexical
// scanner and the RFC gate disagreed (a block comment above a declaration, the
// line ending after a closing brace, CRLF, a bodyless declaration, two methods
// sharing a name) and over every Go file in this package. goUnits is compared
// with rfc.FunctionUnits, the projection the RFC gate's readers consume.
// PREVENTS: a private scanner in the weakening check, which would let the two
// gates disagree about what a test unit is, so one gate's approval or verdict
// names text the other never judged.
func TestWeakeningAndRFCGatesReadOneGoUnitPopulation(t *testing.T) {
	texts := []string{
		"package p\n/* Block comment. */\nfunc TestA() {}\n",
		"package p\n// Doc.\nfunc TestA() { println(1) }\n\nfunc TestB() {}",
		"package p\r\n// Doc.\r\nfunc TestA() {}\r\nfunc TestB() {}\r\n",
		"package p\nfunc external() int\nfunc TestA() {}\n",
		"package p\nfunc (a A) TestSame() {}\nfunc (b B) TestSame() {}\n",
		"package p\nfunc TestA() {}; func TestB() {}\n",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, readErr := os.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		texts = append(texts, string(content))
	}
	for _, text := range texts {
		weakening := goUnits(text)
		gate := rfc.FunctionUnits(text)
		if len(weakening) != len(gate) {
			t.Fatalf("%.60q: the weakening check names %d units, the RFC gate %d", text, len(weakening), len(gate))
		}
		for index, unit := range weakening {
			if unit.incomplete {
				t.Fatalf("%.60q: unit %d (%s) is incomplete in source that parses", text, index, unit.name)
			}
			if unit.name != gate[index].Name || unit.text != gate[index].Text {
				t.Fatalf("%.60q: unit %d is %s %q in the weakening check, %s %q in the RFC gate",
					text, index, unit.name, unit.text, gate[index].Name, gate[index].Text)
			}
		}
	}
}
