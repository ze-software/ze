// Fixture: the ./le test qemu cache refusal, before and after its fix
// (plan/journal/error-names-no-fix-command.md, 2026-10-22).
package qemu

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// cacheBefore is the pre-fix form: the mkdir failure, naming neither the
// dangling link nor its target.
func cacheBefore(dir string) {
	leaction.ReportError(fmt.Errorf("mkdir %s: file exists", dir)) // shape:le-before
}

// cacheAfter is the fixed form: the operator error naming the link, its
// target and the command that recreates it.
func cacheAfter(link, target string) {
	leaction.ReportError(danglingLinkRefusal(link, target)) // shape:le-after
}

// cacheNamed is a fixed form that names a command in plain text.
func cacheNamed() {
	leaction.ReportError(errors.New("tmp/qemu is a dangling link: run ./le scratch migrate")) // shape:le-named
}

// danglingLinkRefusal is the fixed refusal.
func danglingLinkRefusal(link, target string) error {
	return opererr.New(
		link+" is a scratch link to "+target+", which does not exist",
		"the QEMU run writes its caches and guest shares through this link, so every write through it fails",
		"mkdir -p "+target,
	)
}
