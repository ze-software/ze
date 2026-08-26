// Design: docs/architecture/core-design.md -- what this gate still starts
// Overview: delegate.go -- the eight targets it answers in Go instead
//
// forks.go publishes every command line that this gate STARTS instead of
// answering itself. letools/parity uses the list to classify
// `ze-doc-wiring-check`. Thus, the census cannot mark the gate as ported while
// another program does the work.
//
// The list comes from the same two tables as the run: the selection order minus
// targets implemented in delegate.go, plus the one script-only checker. When
// the final script gets a port, the list becomes empty. The census then marks
// the gate converted without a register.go edit.

package docwiring

import (
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// designRefChecker is the one check here implemented by a script.
// scripts/dev/check_doc_links.py has no Go implementation, so this gate starts
// it. checks.go contains the invocation.
var designRefChecker = [...]string{"python3", "scripts/dev/check_doc_links.py", "--design-only"}

// Forks answers every command line in run order. It starts with the
// design-reference checker, then has one Make invocation for each delegated
// target without a Go package.
func Forks() [][]string {
	forks := [][]string{slices.Clone(designRefChecker[:])}

	var tb textbuf.Buffer
	for _, target := range targetOrder {
		if _, inGo := goTargets[target]; target == wiringTarget || inGo {
			continue
		}
		forks = append(forks, []string{defaultMake, tb.Reset().Str(target).String()})
	}
	return forks
}
