// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package mock owns the le mock namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package mock

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("mock",
		"stand-in servers and plugins a test starts",
		"Stand-in servers and plugins a functional test starts in place of a real one: DNS, Cymru, IRR, PeeringDB, RPKI and RTR caches, RADIUS, TACACS+, syslog, HTTP and a looking glass. The plugins Ze itself spawns from a test are hidden from help and completion.")
}
