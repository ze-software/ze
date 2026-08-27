// Design: ai/rules/protocol.md -- the protocol skeleton, as a lens
//
// Package protocolskeleton classifies each protocol's subpackages against the
// standard skeleton: a canonical module, per-peer state named by the protocol's
// own RFC term, a wire-version directory, a domain module, or a documented
// legacy exception.
//
// It is ADVISORY and report mode always exits 0. An enforced skeleton would
// need a large allowlist, which the tiers work already measured as the wrong
// trade; only the selftest may fail, and it fails when the classifier itself
// stopped working.
//
// Detail: report.go holds the answer, selftest.go the fixture cases,
// actions.go the command surface.
package protocolskeleton

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// The five classes a module falls in.
const (
	classCanonical = "canonical"
	classRFCState  = "rfc-state"
	classVersion   = "version"
	classDomain    = "domain"
	classLegacy    = "legacy-exception"
)

// The protocol and module names the tables share. They are named because three
// tables spell each one -- the manifest, the exception list and the selftest --
// and a typo in one of the three would be invisible in the other two.
const (
	protoBGP  = "bgp"
	protoBFD  = "bfd"
	protoIKE  = "ike"
	protoISIS = "isis"
	protoOSPF = "ospf"

	moduleWire   = "wire"
	modulePacket = "packet"
	moduleYang   = "yang"
)

// manifest is the protocol list: the display name and the repo-relative root.
// Protocols are not mechanically discoverable -- "is this directory a protocol"
// needs judgement -- so the list is declared here and mirrored by the probe
// table in ai/rules/protocol.md. A row is added when a protocol lands.
var manifest = []Protocol{
	{Name: protoBGP, Root: "internal/component/bgp"},
	{Name: protoBFD, Root: "internal/component/bfd"},
	{Name: protoIKE, Root: "internal/component/ike"},
	{Name: protoISIS, Root: "internal/plugins/isis"},
	{Name: protoOSPF, Root: "internal/plugins/ospf"},
	{Name: "ldp", Root: "internal/plugins/ldp"},
	{Name: "rsvpte", Root: "internal/plugins/rsvpte"},
}

// canonical names the skeleton's own modules, speaking the go-standards
// glossary.
var canonical = []string{
	modulePacket, "transport", "engine", moduleYang, "types", "cli", "cmd", "redistribute",
}

// rfcState names the per-peer conversation state, under whichever term the
// protocol's own RFC uses for it.
var rfcState = []string{"session", "adjacency", "neighbor", "fsm"}

// legacyExceptions are the documented kept names that predate the glossary,
// keyed by protocol and module. It mirrors the exceptions table in
// ai/rules/protocol.md.
var legacyExceptions = map[string][]string{
	protoBGP: {"message", "wireu", "reactor"},
	protoIKE: {moduleWire},
}

// Classify answers which of the five classes a module falls in.
//
// The exception is checked FIRST so that a name added to both an exception and
// a class list resolves as the documented exception. No name is in two lists
// today, so the order changes no answer this tree can produce; it is what keeps
// that true for the next name somebody adds.
func Classify(protocol, module string) string {
	if slices.Contains(legacyExceptions[protocol], module) {
		return classLegacy
	}
	if slices.Contains(canonical, module) {
		return classCanonical
	}
	if slices.Contains(rfcState, module) {
		return classRFCState
	}
	if isVersion(module) {
		return classVersion
	}
	return classDomain
}

// isVersion reports whether a module name is a wire-version directory: a v
// followed by digits, and nothing else.
func isVersion(module string) bool {
	if len(module) < 2 || module[0] != 'v' {
		return false
	}
	for _, char := range module[1:] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// modulesOf answers the immediate subpackage directories of one protocol root,
// sorted. Hidden directories and testdata are not packages a reader classifies.
func modulesOf(tree, root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(tree, filepath.FromSlash(root)))
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, item := range entries {
		if !item.IsDir() || strings.HasPrefix(item.Name(), ".") || item.Name() == "testdata" {
			continue
		}
		names = append(names, item.Name())
	}
	sort.Strings(names)
	return names, nil
}

// Build classifies every protocol of the manifest given, over the tree given.
//
// A root that is not there is MISSING rather than empty, and a missing root is
// never single-package: the manifest is hand-maintained, so a stale row must
// read as a stale row rather than as a protocol that happens to have no
// subpackages.
func Build(tree string, protocols []Protocol) (Report, error) {
	report := Report{Protocols: make([]Protocol, 0, len(protocols))}

	for _, protocol := range protocols {
		info, err := os.Stat(filepath.Join(tree, filepath.FromSlash(protocol.Root)))
		if err != nil || !info.IsDir() {
			protocol.Missing = true
			report.Protocols = append(report.Protocols, protocol)
			continue
		}

		names, err := modulesOf(tree, protocol.Root)
		if err != nil {
			return Report{}, err
		}
		protocol.Modules = make([]Module, 0, len(names))
		single := true
		for _, name := range names {
			class := Classify(protocol.Name, name)
			protocol.Modules = append(protocol.Modules, Module{Name: name, Class: class})
			report.count(class)
			if name != moduleYang {
				single = false
			}
		}
		protocol.SinglePackage = single
		report.Protocols = append(report.Protocols, protocol)
	}
	return report, nil
}

// Manifest answers the declared protocol list. The caller receives a copy, so a
// test that runs its own list cannot edit the one the command uses.
func Manifest() []Protocol { return slices.Clone(manifest) }

// count adds one module to the class it fell in.
func (r *Report) count(class string) {
	switch class {
	case classCanonical:
		r.Counts.Canonical++
	case classRFCState:
		r.Counts.RFCState++
	case classVersion:
		r.Counts.Version++
	case classLegacy:
		r.Counts.LegacyException++
	default:
		r.Counts.Domain++
	}
}
