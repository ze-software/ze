// Design: docs/architecture/testing/runner-architecture.md -- accepted external fixture modules.
// Related: externalyang.go -- the acme variants and their existing corpus origins.

package externalyang

import "strings"

func init() {
	for _, module := range Accepted() {
		mustRegisterConfigDeclaration(ConfigDeclaration{
			Plugin: strings.TrimSuffix(module.Name, "-conf"),
			Module: module,
		})
	}
}
