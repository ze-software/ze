// Design: docs/architecture/wire/nlri-bgpls.md -- domain source refusal
// Related: register.go -- sourceNeeds, the IGP sources a build may leave out

package ls_export

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// verifyDomainSources refuses every `domain` entry whose source names a BGP-LS
// namespace no plugin can publish. The exporter subscribes only to the
// namespaces compiled-in plugins register (runTopologyExporter), so such an
// entry would be accepted and map no snapshot: a typo, or `rsvp-te`, which
// registers no namespace in any build.
//
// A source is known when a plugin in this build registered it
// (linkstateevents.Sources), or when it is the value of one of the exporter's
// config needs (sourceNeeds). An IGP the build left out is refused by that need
// instead, naming its build tag, and refusing it here as unknown would hide
// the tag. Every refused entry is named, in the order the operator wrote them.
//
// It runs as a registered original-tree validator, so boot, reload,
// `ze config validate` and the editor commit reach one verdict. Safe for
// concurrent use: it reads the tree and the source registry only.
func verifyDomainSources(tree *config.Tree) error {
	published := linkstateevents.Sources()
	var refusals []error
	if export := tree.GetContainer(exporterName); export != nil {
		for _, domain := range export.GetListOrdered("domain") {
			source, set := domain.Value.Get("source")
			if !set {
				// The YANG mandatory check names a domain with no source.
				continue
			}
			if slices.Contains(published, source) {
				continue
			}
			if slices.ContainsFunc(sourceNeeds, func(need registry.ConfigNeed) bool { return need.Value == source }) {
				continue
			}
			refusals = append(refusals, unpublishedSourceError(domain.Key, source, published))
		}
	}
	return errors.Join(refusals...)
}

// unpublishedSourceError writes the problem, why it matters, the sources the
// build does publish, and the fix on a line of its own
// (ai/rules/user-facing-errors.md).
func unpublishedSourceError(domain, source string, published []string) error {
	known := "none"
	if len(published) > 0 {
		known = textbuf.Join(published, ", ")
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str(`config: "`).Str(exporterName).Str(" domain ").Str(domain).
		Str(" source ").Str(source).Str(`" names a BGP-LS source that no plugin in this build publishes`).
		Str("\n  the domain would map no native snapshot and export nothing, so ze refuses the config").
		Str("\n  sources this build publishes: ").Str(known).
		Str("\n  fix: set the source of ").Str(exporterName).Str(" domain ").Str(domain).
		Str(" to one of them, or remove that domain from the config").String())
}
