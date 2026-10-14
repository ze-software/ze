// Design: docs/contributing/patching-the-standard-library.md -- the routes that run before le exists
//
// Command goroot prints the patched GOROOT for the checkout in the current
// directory, building it on first use. It is the entry the launcher's
// build_le, the QEMU guest profiles, the Dockerfiles and CI run with
// `go run ./internal/le/go/stdpatch/goroot`, under the unpatched GOROOT,
// because le itself cannot compile until a tree exists. Once le exists the
// same answer is `./le go stdpatch goroot`.
//
// A refusal (a stale review stamp, a patch that no longer applies) prints the
// generator's error on standard error and exits 1, so the caller stops.
package main

import (
	"os"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	tree, err := gen.Tree(root)
	if err != nil {
		fail(err)
	}
	if _, err := os.Stdout.WriteString(tree + "\n"); err != nil {
		fail(err)
	}
}

// fail reports err and exits 1.
func fail(err error) {
	_, _ = os.Stderr.WriteString(err.Error() + "\n") //nolint:errcheck // nothing is left to report a failed write to
	os.Exit(1)
}
