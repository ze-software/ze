// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package spec

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("spec", "the spec workflow: claim, track, review and release specs, with their journal and roadmap")
}
