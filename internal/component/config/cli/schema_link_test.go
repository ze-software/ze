// Design: docs/architecture/config/yang-config-design.md -- schema resolution in test binaries.
// Related: all_import_test.go -- the blank imports that link this binary's YANG modules.

package cli_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestThisBinaryLinksEveryModuleItsSchemaImports resolves, in the binary they
// run in, the schema the deactivate, activate, diff, history and migrate cases
// of this package read.
//
// VALIDATES: the modules this test binary registers, which its blank imports
// decide, resolve with no error, so every module a linked module imports is
// linked under the name it is imported by. DefaultLoader resolves every
// registered module, so the set checked is the registrations this binary
// holds, not a list written here.
// PREVENTS: the schema-dependent cases of this package failing far from the
// cause, as they did with "no such module: ze-bgp-conf", when the binary does
// not register a module its schema imports. This test names the missing
// module first.
//
// MUTATION: (internal/component/bgp/yang/ze-bgp-conf.yang, `module ze-bgp-conf {`, `module ze-bgp-conf-unlinked {`): this test goes red with "no such module: ze-bgp-conf", because linked modules such as ze-rib import ze-bgp-conf and no registered module carries that name any more.
func TestThisBinaryLinksEveryModuleItsSchemaImports(t *testing.T) {
	if len(yang.Modules()) == 0 {
		t.Fatal("this binary registers no YANG module: its blank imports link no component")
	}
	if _, err := yang.DefaultLoader(); err != nil {
		t.Fatalf("this binary's schema does not resolve, so a module it imports is not linked; add the blank import of the package that registers it to all_import_test.go: %v", err)
	}
}
