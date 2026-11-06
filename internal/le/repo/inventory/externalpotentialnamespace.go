// Design: docs/architecture/plugin/feature-gates.md -- closed parent package authority.
package repoinventory

import "strings"

// externalParentModuleOverlap identifies every module whose namespace could
// supply an absent parent-qualified import: equal, ancestor or descendant.
// Main-module precedence and an absent vendor package row cannot prove that an
// unretained overlapping module contains no alternative package.
func externalParentModuleOverlap(identity string) bool {
	root := strings.TrimSuffix(modulePrefix, "/")
	if externalParentPackageIdentity(identity) {
		return true
	}
	return strings.HasPrefix(root, identity+"/")
}

func externalParentPackageIdentity(identity string) bool {
	if identity == strings.TrimSuffix(modulePrefix, "/") {
		return true
	}
	return strings.HasPrefix(identity, modulePrefix)
}
