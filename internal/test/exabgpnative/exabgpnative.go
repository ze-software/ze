// Design: docs/architecture/testing/ci-format.md — test fixtures the repository carries
// Related: ../../component/bgp/config/loader_test.go — TestParseAllConfigFiles reads this classification
// Related: ../../component/config/cli/validate_corpus_test.go — the config sweep reads it too

// Package exabgpnative classifies the fixtures under test/exabgp-compat/native.
//
// Most of those files predate Ze's native syntax and are kept as examples of
// the ExaBGP shapes they show. A few are curated: written in Ze's native
// syntax, so they must load. Every file is one or the other, and an
// unclassified file is a defect its reader reports. The classification is
// declared once, here, so every reader (the BGP loader's parse test and the
// repository config sweep) judges a fixture the same way.
package exabgpnative

import "strings"

// Dir is the directory, relative to the repository root, that holds the
// fixtures this package classifies.
const Dir = "test/exabgp-compat/native"

// Kind is the class of one fixture. Its zero value means the file is not
// classified, which a reader reports rather than guessing.
type Kind uint8

const (
	// KindUnclassified is a file this package does not know.
	KindUnclassified Kind = iota
	// KindCurated is a fixture in Ze's native syntax: it must load.
	KindCurated
	// KindLegacy is an example kept in an older syntax: it is not loaded,
	// and its reader names the reason.
	KindLegacy
)

// Class is the classification of one fixture: its kind, and for a legacy
// fixture the reason it is not loaded.
type Class struct {
	Kind   Kind
	Reason string
}

// curated names the fixtures written in Ze's native syntax.
var curated = map[string]bool{
	"parse-community.conf":        true,
	"parse-dual-neighbor.conf":    true,
	"parse-md5.conf":              true,
	"parse-multiple-process.conf": true,
	"parse-process.conf":          true,
	"parse-simple-v4.conf":        true,
	"parse-simple-v6.conf":        true,
	"parse-ttl.conf":              true,
}

// Classify answers the class of the fixture with this file name (the base
// name, not a path). A name it does not know answers KindUnclassified.
func Classify(name string) Class {
	if curated[name] {
		return Class{Kind: KindCurated}
	}
	if reason, ok := legacyReason(name); ok {
		return Class{Kind: KindLegacy, Reason: reason}
	}
	return Class{Kind: KindUnclassified}
}

// CuratedCount answers how many curated fixtures exist, so a reader can
// prove it exercised every one.
func CuratedCount() int {
	return len(curated)
}

// legacyReason answers why a legacy fixture is not loaded, and false for a
// name no legacy rule covers.
func legacyReason(name string) (string, bool) {
	switch {
	case name == "parse-multisession.conf":
		return "legacy example for ExaBGP multi-session; ze rejects that removed capability in native syntax", true
	case strings.HasPrefix(name, "api-"):
		return "legacy ExaBGP-style API example awaiting native conversion", true
	case strings.HasPrefix(name, "conf-"):
		return "legacy ExaBGP-style configuration example awaiting native conversion", true
	case name == "example-healthcheck.conf":
		return "legacy healthcheck example awaiting native conversion", true
	case name == "extended-nexthop.conf":
		return "legacy extended-nexthop example awaiting native conversion", true
	case name == "unknown-message.conf":
		return "legacy unknown-message example awaiting native conversion", true
	}
	return "", false
}
