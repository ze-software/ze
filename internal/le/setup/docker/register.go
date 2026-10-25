// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package setupdocker owns the le setup docker namespace: it says what the
// namespace is for, and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package setupdocker

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("setup docker",
		"set up the Docker host Ze's labs run on",
		"Set up the Docker host Ze's labs run on: Ze's docker-host image as the macOS Colima profile, and the Linux daemon's kernel and AppArmor profiles.")
}
