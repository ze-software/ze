// Design: docs/features/interfaces.md -- Interface-name validation
// Related: validate_linux.go -- VLAN/MTU range helpers (Linux-only)

package iface

import (
	"fmt"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// Interface name length limits (Linux kernel IFNAMSIZ = 16, including
// NUL). Applied on every platform so `ze config validate` rejects
// oversized names uniformly regardless of where ze was built.
const (
	minIfaceNameLen = 1
	maxIfaceNameLen = 15
)

// reservedIfaceNames holds CLI keywords that MUST NOT be used as
// interface names -- every direct child of the `show interface` or
// `clear interface` containers in the YANG command tree. Allowing an
// operator to call an interface `counters` (or `brief`, `type`, etc.)
// creates an unresolvable grammar ambiguity where
// `show interface counters` could mean "the counters subview" or
// "details of the interface named counters".
//
// Populated once from the YANG command tree (see loadReservedIfaceNames)
// so the list stays in sync with whatever is in ze-cli-show-cmd.yang
// and ze-cli-clear-cmd.yang -- no separate hand-maintained copy to
// drift (rules/derive-not-hardcode.md). The YANG grammar itself does
// not enforce this (YANG patterns are POSIX regex with awkward
// exclusion syntax); rejection instead happens at every call site
// that validates a name (config parser, backend ops) so the reserved
// name fails at verify time with a clear message.
var (
	reservedIfaceNames     = map[string]string{}
	reservedIfaceNamesOnce sync.Once
)

// loadReservedIfaceNames walks the merged YANG command tree and
// collects every direct child of `show interface` and `clear interface`.
// Called lazily on first ValidateIfaceName invocation so the YANG
// modules have time to register via init(). In a test binary where no
// -cmd schema is imported the map stays empty and the reserved check
// becomes a no-op -- tests that want to exercise the check
// blank-import the relevant schema packages.
func loadReservedIfaceNames() {
	loader, err := yang.DefaultLoader()
	if err != nil || loader == nil {
		return
	}
	tree := yang.BuildCommandTree(loader)
	if tree == nil {
		return
	}
	for _, root := range []string{"show", "clear"} {
		rootNode := tree.Children[root]
		if rootNode == nil {
			continue
		}
		ifaceNode := rootNode.Children["interface"]
		if ifaceNode == nil {
			continue
		}
		for name := range ifaceNode.Children {
			if _, seen := reservedIfaceNames[name]; !seen {
				reservedIfaceNames[name] = root + " interface " + name
			}
		}
	}
}

// ValidateIfaceName checks that name is a valid interface name for
// every platform ze targets. The Linux kernel forbids '/' and NUL in
// interface names (IFNAMSIZ); we also reject whitespace, ".."
// path-traversal sequences, and a small set of names reserved by the
// CLI command grammar (see reservedIfaceNames).
//
// Exported so backend implementations AND the config parser can use
// it -- the parser invocation ensures `ze config validate` rejects
// reserved names up front, not only at apply/runtime.
func ValidateIfaceName(name string) error {
	n := len(name)
	if n < minIfaceNameLen || n > maxIfaceNameLen {
		return fmt.Errorf("iface: name %q length %d not in [%d, %d]",
			name, n, minIfaceNameLen, maxIfaceNameLen)
	}
	for i := range n {
		c := name[i]
		if c == '/' || c == 0 || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return fmt.Errorf("iface: name %q contains forbidden character", name)
		}
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("iface: name %q contains path traversal sequence", name)
	}
	reservedIfaceNamesOnce.Do(loadReservedIfaceNames)
	if usage, reserved := reservedIfaceNames[name]; reserved {
		return fmt.Errorf("iface: name %q is a reserved CLI keyword (used by %q); rename the interface",
			name, usage)
	}
	return nil
}
