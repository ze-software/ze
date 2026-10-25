// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package chaos owns the le chaos namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package chaos

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("chaos", "chaos testing: run the orchestrator against ze, FRR or BIRD, and self-test the simulator")
}
