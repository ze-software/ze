// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
// Related: ownership_test.go -- dispatches le arch tier check through the registry

package archtier_test

// TestRegisteredTierCheckEnforcesPluginOwnership dispatches `le arch tier
// check` through leroot.Dispatch, which refuses a namespace with no summary.
// The tier check links areas under arch, plugin and repo, and repo's own
// package pulls areas from a dozen more namespaces, so this test binary links
// the composition root, as le itself does. An external test package may
// import it: internal/le imports this package, which an in-package test file
// could not import back.
import _ "github.com/ze-software/ze/internal/le"
