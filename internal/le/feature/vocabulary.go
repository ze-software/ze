// Design: docs/contributing/feature-maturity.md -- the declaration model, its kinds, scopes and levels
// Related: declaration.go -- the parser that admits only these values
// Related: check.go -- the ceiling computed over these levels
//
// The ONE declaration of the feature maturity vocabulary. The rendered
// docs/features.md header, the site's card state and every refusal message read
// the lists from here, so no second copy of a level name exists in le
// (ai/rules/principles.md, derive once).

package feature

import (
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Kind decides what the S1, S2 and S3 criteria mean for one feature.
type Kind uint8

// Kinds. Zero is Unspecified, so a declaration that never set one is refused
// rather than read as a protocol.
const (
	KindUnspecified Kind = iota
	KindProtocol
	KindDaemon
	KindLibrary
	KindDevTool
	KindPackaging
	KindTestInfra
	KindUmbrella
)

// kindNames is the declared spelling of each kind, in report order.
var kindNames = []namedValue[Kind]{
	{KindProtocol, "protocol"},
	{KindDaemon, "daemon"},
	{KindLibrary, "library"},
	{KindDevTool, "dev-tool"},
	{KindPackaging, "packaging"},
	{KindTestInfra, "test-infra"},
	{KindUmbrella, "umbrella"},
}

// Scope answers how much of the stated feature exists.
type Scope uint8

// Scopes. Zero is Unspecified.
const (
	ScopeUnspecified Scope = iota
	ScopeComplete
	ScopePartial
	ScopeFuture
	ScopeRejected
)

var scopeNames = []namedValue[Scope]{
	{ScopeComplete, "complete"},
	{ScopePartial, "partial"},
	{ScopeFuture, "future"},
	{ScopeRejected, "rejected"},
}

// Level answers how well the implemented scope is proven. The constants are
// ordered weakest first, so a comparison of two levels is a comparison of their
// values; LevelUnspecified sorts below every real level.
type Level uint8

// Levels, weakest first.
const (
	LevelUnspecified Level = iota
	LevelStubBacked
	LevelExperimental
	LevelSupported
)

var levelNames = []namedValue[Level]{
	{LevelStubBacked, "stub-backed"},
	{LevelExperimental, "experimental"},
	{LevelSupported, "supported"},
}

// levelLabels is the public word each level renders as.
var levelLabels = map[Level]string{
	LevelStubBacked:   "Stub-backed",
	LevelExperimental: "Experimental",
	LevelSupported:    "Supported",
}

// scopeLabels is the public word a scope with no level renders as.
var scopeLabels = map[Scope]string{
	ScopeFuture:   "Future",
	ScopeRejected: "Rejected",
}

// levelMeanings and scopeMeanings are what each public label asserts, for the
// vocabulary sentence that opens the rendered docs/features.md. Each states
// what the check requires of the level, so the sentence cannot promise more
// than check.go enforces.
var levelMeanings = map[Level]string{
	LevelSupported: "every completion criterion its kind requires holds, " +
		"including a current recorded green run of each real-path test and of each counted interop scenario",
	LevelExperimental: "implemented and reachable through its real entry point, " +
		"without the evidence Supported requires",
	LevelStubBacked: "implemented, with the evidence for its external dependency " +
		"coming only from a stub harness",
}

var scopeMeanings = map[Scope]string{
	ScopePartial:  "the named scope gaps are not implemented or not proven",
	ScopeFuture:   "planned but not shipped",
	ScopeRejected: "unsupported by design",
}

// kindHeadings titles the table each kind's features render under, in the
// order of kindNames (D-11: product and tooling rows stay on one page,
// labeled by Kind).
var kindHeadings = map[Kind]string{
	KindProtocol:  "Protocols",
	KindDaemon:    "Daemon services",
	KindLibrary:   "Libraries",
	KindDevTool:   "Development tools",
	KindPackaging: "Packaging",
	KindTestInfra: "Test infrastructure",
	KindUmbrella:  "Umbrellas",
}

type namedValue[T comparable] struct {
	value T
	name  string
}

func lookup[T comparable](table []namedValue[T], name string) (T, bool) {
	for _, entry := range table {
		if entry.name == name {
			return entry.value, true
		}
	}
	var zero T
	return zero, false
}

func nameOf[T comparable](table []namedValue[T], value T) string {
	for _, entry := range table {
		if entry.value == value {
			return entry.name
		}
	}
	return "unspecified"
}

func names[T comparable](table []namedValue[T]) []string {
	out := make([]string, 0, len(table))
	for _, entry := range table {
		out = append(out, entry.name)
	}
	return out
}

// KindNames, ScopeNames and LevelNames answer the declared vocabulary in order.
func KindNames() []string  { return names(kindNames) }
func ScopeNames() []string { return names(scopeNames) }
func LevelNames() []string { return names(levelNames) }

func (k Kind) String() string  { return nameOf(kindNames, k) }
func (s Scope) String() string { return nameOf(scopeNames, s) }
func (l Level) String() string { return nameOf(levelNames, l) }

// Implemented answers whether the scope describes code that exists, which is
// the condition under which a Level is required.
func (s Scope) Implemented() bool {
	return s == ScopeComplete || s == ScopePartial
}

// StatusLabel renders the public Status cell from the pair, per the Levels
// table of the spec: "Supported", "Experimental (partial)", "Future".
func StatusLabel(scope Scope, level Level) string {
	if !scope.Implemented() {
		return scopeLabels[scope]
	}
	label := levelLabels[level]
	if scope == ScopePartial {
		var tb textbuf.Buffer
		return tb.Str(label).Str(" (partial)").String()
	}
	return label
}

// StatusLabels answers every label a rendered row can carry, in ladder order,
// for the vocabulary sentence of the rendered page and the site's card legend.
func StatusLabels() []string {
	out := make([]string, 0, 2*len(levelNames)+len(scopeLabels))
	for _, entry := range slices.Backward(levelNames) {
		out = append(out, StatusLabel(ScopeComplete, entry.value))
	}
	for _, entry := range slices.Backward(levelNames) {
		out = append(out, StatusLabel(ScopePartial, entry.value))
	}
	return append(out, scopeLabels[ScopeFuture], scopeLabels[ScopeRejected])
}
