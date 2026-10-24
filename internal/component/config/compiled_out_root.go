// Design: docs/architecture/plugin/feature-gates.md -- naming a plugin the build left out
// Related: retired.go -- UnknownKeywordHint, the hint every other unknown keyword carries
// Related: optional_provider.go -- writeBuildTags, the fix-line wording this hint shares
// Related: parser.go -- parseRoot, the brace parser's top-level refusal
// Related: setparser.go -- walkAndSet, the set parser's top-level refusal

package config

import (
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TopLevelKeywordHint answers the text a parse error carries after an unknown
// top-level keyword. A root that a gated plugin owns is unknown only because
// this build was compiled without that plugin, so the hint says so and names
// the build tags that carry it (registry.GatedConfigRootRequirements, from the
// generated table every build compiles). Any other name is a keyword no build
// of ze knows, and gets UnknownKeywordHint: a retired keyword's replacement or
// the closest valid keyword.
//
// Only the top level asks the table: a root is a config root a plugin owns,
// and a nested unknown field inside a present module is a typo, never a
// compiled-out plugin.
func TopLevelKeywordHint(name string, known []string) string {
	requirements, gated := registry.GatedConfigRootRequirements(name)
	if !gated {
		return UnknownKeywordHint(name, known)
	}

	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(name)
	b.WriteString(" is configured by a plugin this build of ze was compiled without, so the keyword is unknown here\n")
	b.WriteString("  fix: rebuild ze with ")
	for i, tags := range requirements {
		if i > 0 {
			b.WriteString(", or with ")
		}
		writeBuildTags(&b, tags)
	}
	b.WriteString(" added to its -tags list, or remove the ")
	b.WriteString(name)
	b.WriteString(" block from the config")

	return b.String()
}
