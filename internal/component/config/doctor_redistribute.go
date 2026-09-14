// Design: docs/guide/redistribution.md -- the redistribute root an operator writes
// Overview: register_doctor.go -- the init() that installs the registration below
// Related: loader_redistribute.go -- ExtractRedistributeRules, the load that refuses the source this reports
// Related: validators.go -- RedistributeSourceValidator and RegisteredProtocolValidator, the two registries it reads
//
// The redistribution chain is silent by construction. Two rules end as a route
// that is not there: one whose source no component registers, and one whose
// destination no protocol can consume. These two checks are where an operator
// asks why.
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. The loader that refuses the unknown source lives here,
// and so does the tree type the check reads, so the check does too
// (ai/patterns/registration.md, "Doctor Check Registry").

package config

import (
	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeRedistUnknownSource names a `redistribute` import whose source is in
// no component's source registry. The daemon refuses to start on it.
const codeRedistUnknownSource = "doctor-redistribute-unknown-source"

// codeRedistUnknownDestination names a `redistribute` destination that no
// protocol can ever consume. The daemon starts and the rules under it never
// fire.
const codeRedistUnknownDestination = "doctor-redistribute-unknown-destination"

// redistributeDoctorCheck describes the check. register_doctor.go registers it.
//
// Order 2140 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it ran right after the BGP role check at
// 2130 (internal/component/bgp/config/doctor_checks.go) and before the AS112
// coordination checks at 2150 (internal/component/doctor/doctor_checks.go).
var redistributeDoctorCheck = diagnostic.DoctorCheck{
	Name:         "redistribute-rules",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2140,
	Component:    "config",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeRedistUnknownSource, codeRedistUnknownDestination},
	Check:        checkRedistributeRules,
}

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
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkRedistributeRules(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*Tree)
	if !ok || tree == nil {
		return nil
	}
	redist := tree.GetContainer("redistribute")
	if redist == nil {
		return nil
	}

	var out []diagnostic.Diagnostic
	for _, dest := range redist.GetListOrdered("destination") {
		out = append(out, unknownRedistDestination(dest.Key)...)
		for _, entry := range dest.Value.GetListOrdered("import") {
			out = append(out, unknownRedistSource(entry.Key, dest.Key)...)
		}
		// Scalar form: `import static;` is stored as a key-value rather than a
		// list entry, and it reaches the daemon by the same path.
		if scalar, ok := dest.Value.Get("import"); ok && scalar != "" {
			out = append(out, unknownRedistSource(scalar, dest.Key)...)
		}
	}
	return out
}

// unknownRedistDestination reports a destination protocol that registered no
// redistribution identity.
func unknownRedistDestination(protocol string) []diagnostic.Diagnostic {
	if protocol == "" {
		return nil
	}
	if _, ok := redistevents.ProtocolIDOf(protocol); ok {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     codeRedistUnknownDestination,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("redistribute destination ").Str(protocol).
			Str(" names no protocol this build registers, so every import under it is inert").String(),
	}}
}

// unknownRedistSource reports an import whose source is in no source registry.
func unknownRedistSource(source, protocol string) []diagnostic.Diagnostic {
	if source == "" {
		return nil
	}
	if _, ok := redistribute.LookupSource(source); ok {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     codeRedistUnknownSource,
		Severity: diagnostic.SeverityError,
		Message: tb.Str("redistribute destination ").Str(protocol).Str(" imports ").Str(source).
			Str(", which no component registers as a source (the daemon will not start on it)").String(),
	}}
}
