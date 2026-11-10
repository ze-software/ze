// Design: docs/architecture/plugin/feature-gates.md -- naming a plugin the build left out
// Related: retired.go -- UnknownKeywordHint, the hint every other unknown keyword carries
// Related: optional_provider.go -- writeBuildTags, the fix-line wording this hint shares
// Related: parser.go -- parseBody and parseContainer, the brace parser's refusals
// Related: setparser.go -- unknownFieldHint, the set parser's refusal

package config

import (
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// KeywordHint answers the text a parse error carries after an unknown keyword
// written under parent (nil at the top level). A keyword that a gated plugin's
// config root holds is unknown only because this build was compiled without
// that plugin, so the hint says so and names the build tags that carry it
// (registry.GatedConfigRootRequirements, from the generated table every build
// compiles). That holds at the top level (`bgp`, `mrt`) and below it, where a
// gated plugin owns a nested root (`fib vpp`, `environment bmp`). Any other
// name is a keyword no build of ze knows, and gets UnknownKeywordHint: a
// retired keyword's replacement or the closest valid keyword.
//
// parent holds the keywords from the top level down, list keys left out, the
// form the table's roots take; a path that crosses a list matches no root, so
// a typo inside a list entry is never answered as a compiled-out plugin.
func KeywordHint(parent []string, name string, known []string) string {
	var path strings.Builder
	for _, keyword := range parent {
		path.WriteString(keyword)
		path.WriteByte('/')
	}
	path.WriteString(name)
	requirements, gated := registry.GatedConfigRootRequirements(path.String())
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
