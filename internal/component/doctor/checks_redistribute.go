// Design: docs/guide/redistribution.md -- the redistribute root an operator writes
// Related: checks_config.go -- the other config-coherence checks
// Related: internal/component/config/loader_redistribute.go -- ExtractRedistributeRules

// The redistribution chain is silent by construction. Two rules end as a route
// that is not there: one whose source no component registers, and one whose
// destination no protocol can consume. These two checks are where an operator
// asks why.

package doctor

import (
	"github.com/ze-software/ze/internal/component/config"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// diagnosticRedistUnknownSource names a `redistribute` import whose source is in
// no component's source registry. The daemon refuses to start on it.
const diagnosticRedistUnknownSource = "doctor-redistribute-unknown-source"

// diagnosticRedistUnknownDestination names a `redistribute` destination that no
// protocol can ever consume. The daemon starts and the rules under it never
// fire.
const diagnosticRedistUnknownDestination = "doctor-redistribute-unknown-destination"

// checkRedistributeRules judges every rule the `redistribute` root declares
// against the two registries that decide whether it can ever move a route.
//
// The source registry is filled by each component's init, so an unknown source
// is decidable here, and it is an ERROR. ExtractRedistributeRules refuses the
// load on it, and `ze doctor` is where an operator finds out first.
//
// A destination is a protocol name whose consumer registers at plugin startup,
// which is after this process reads anything. So the check asks the weaker
// question it can answer honestly. Did any protocol register this name at all?
//
// A name nothing registered can have no consumer under any startup order. The
// YANG leaf carries no validation for it
// (internal/component/config/redistribute/yang/ze-redistribute-conf.yang), so a
// typo there is invisible everywhere else. It is a WARNING, because a build
// that omits the destination protocol is a legitimate reason for the name to be
// unknown.
func checkRedistributeRules(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	redist := tree.GetContainer("redistribute")
	if redist == nil {
		return nil
	}

	var out []diagnostic.Diagnostic
	for _, dest := range redist.GetListOrdered("destination") {
		out = append(out, unknownDestination(dest.Key)...)
		for _, entry := range dest.Value.GetListOrdered("import") {
			out = append(out, unknownSource(entry.Key, dest.Key)...)
		}
		// Scalar form: `import static;` is stored as a key-value rather than a
		// list entry, and it reaches the daemon by the same path.
		if scalar, ok := dest.Value.Get("import"); ok && scalar != "" {
			out = append(out, unknownSource(scalar, dest.Key)...)
		}
	}
	return out
}

// unknownDestination reports a destination protocol that registered no
// redistribution identity.
func unknownDestination(protocol string) []diagnostic.Diagnostic {
	if protocol == "" {
		return nil
	}
	if _, ok := redistevents.ProtocolIDOf(protocol); ok {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     diagnosticRedistUnknownDestination,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("redistribute destination ").Str(protocol).
			Str(" names no protocol this build registers, so every import under it is inert").String(),
	}}
}

// unknownSource reports an import whose source is in no source registry.
func unknownSource(source, protocol string) []diagnostic.Diagnostic {
	if source == "" {
		return nil
	}
	if _, ok := configredist.LookupSource(source); ok {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     diagnosticRedistUnknownSource,
		Severity: diagnostic.SeverityError,
		Message: tb.Str("redistribute destination ").Str(protocol).Str(" imports ").Str(source).
			Str(", which no component registers as a source (the daemon will not start on it)").String(),
	}}
}
