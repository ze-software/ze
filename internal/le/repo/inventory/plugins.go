// Design: (none -- build tool)
//
// Detail: inventory.go -- the walk this file borrows; report.go -- the answer.
//
// plugins.go answers what a plugin IS, beyond what registry.Registration
// states: the package directory it registers from, and the YANG files that sit
// beside that package. The public plugin catalog shows both, and neither is a
// field of the registration, so both are DERIVED from the registration rather
// than declared a second time.

package repoinventory

import (
	"context"
	"strings"
)

// modulePrefix is this module's import path, which every package path a
// registration resolves to starts with.
const modulePrefix = "github.com/ze-software/ze/"

// yangDirectory is the directory name a package keeps its YANG modules in.
const yangDirectory = "yang"

// DeclaredPlugins reports the registrations the committed Linux le source
// declares. The website, the inventory report, the gated-plugin table and the
// coupling check all read this list.
func DeclaredPlugins(root string) ([]Plugin, error) {
	snapshot, err := ReadCommitted(context.Background(), root)
	if err != nil {
		return nil, err
	}
	return snapshot.DeclaredPlugins()
}

// yangModuleName answers the module a YANG document declares.
//
// It reads the first "module" or "submodule" statement, which YANG requires to
// be the document's own opening statement. Comments and blank lines before it
// are skipped, so a license header does not hide the name.
func yangModuleName(document string) string {
	for line := range strings.SplitSeq(document, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || (fields[0] != "module" && fields[0] != "submodule") {
			return ""
		}
		return strings.TrimSuffix(fields[1], "{")
	}
	return ""
}
