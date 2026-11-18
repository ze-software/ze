// Design: docs/plugin-development/schema.md -- extensions that name a registration
// Overview: plugin_fixture_external_yang.go -- the external-yang fixture and its variant table
// Related: ../externalyang/unheld.go -- the module this variant declares

package fixture

import "github.com/ze-software/ze/internal/test/externalyang"

// The unheld-validate variant brings a config module whose leaf names a
// ze:validate function ze does not hold. The daemon refuses the module at the
// join, so a transport that ends is the pass and a ready plugin is the failure
// (externalYANGVariant.refused reads the module's Refused mark).
//
// It is added from this file rather than written into the variant table, so
// the variant lives beside the module it brings. The table is a package-level
// map literal, filled before any init runs.
func init() {
	externalYANGVariants["unheld-validate"] = externalYANGVariant{
		schema: externalyang.UnheldValidate(),
		wants:  []string{"acme-unheld"},
	}
}
