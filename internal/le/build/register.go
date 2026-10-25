// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package build owns the le build namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package build

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("build",
		"build the appliance images, initrd and host driver",
		"Build the appliance pieces: gokrazy images, the installer initrd, the host driver and the packed go.sum files.")
}
