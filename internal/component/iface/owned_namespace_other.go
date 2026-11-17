//go:build !linux

// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: owned_namespace_linux.go -- the Linux form

package iface

import (
	"os"
	"runtime"

	"github.com/ze-software/ze/internal/core/opererr"
)

// openOwnedLinkNamespace refuses off Linux: a network namespace is a Linux
// primitive, so no link can have its peer end in one.
func openOwnedLinkNamespace(_ int, _ uint64) (*os.File, error) {
	return nil, opererr.New(
		"Owned links need network namespaces, which "+runtime.GOOS+" does not have",
		"A link's peer end is created inside a network namespace, so no link was made",
		"Run ze on Linux for owned links",
	)
}
