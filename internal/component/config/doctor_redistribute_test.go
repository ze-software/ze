// Design: docs/guide/redistribution.md -- the redistribute readiness check tests
// Detail: doctor_redistribute.go -- checkRedistributeRules and its registration
//
// These cases arrived from internal/component/doctor with the check they
// drive, case for case. The one that drives `ze doctor` end to end stays in
// that package, because it is the runner it exercises.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// redistDoctorTree builds a `redistribute { destination <dest> { import
// <source> } }` tree directly, with no parse step. The check is then driven by
// the shape it reads rather than by YANG.
func redistDoctorTree(dest, source string) *Tree {
	tree := NewTree()
	redist := tree.GetOrCreateContainer("redistribute")
	destination := NewTree()
	destination.AddListEntry("import", source, NewTree())
	redist.AddListEntry("destination", dest, destination)
	return tree
}

// redistDoctorCodes flattens a diagnostic list to the codes it carries, which
// is what the assertions are about.
func redistDoctorCodes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	// Indexed, not ranged by value: Diagnostic is 184 bytes (gocritic
	// rangeValCopy), and only Code is read here.
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

func redistDoctorDiags(tree *Tree) []diagnostic.Diagnostic {
	return checkRedistributeRules(diagnostic.DoctorCheckContext{Tree: tree})
}

// TestRedistributeCheckSilentOnAWorkingRule keeps the check from crying wolf.
//
// VALIDATES: a rule whose source is registered and whose destination protocol
// registered an identity produces no diagnostic.
// PREVENTS: a warning on every correct config, which an operator learns to
// ignore and which then hides the one that matters.
func TestRedistributeCheckSilentOnAWorkingRule(t *testing.T) {
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "connected", Protocol: "connected"}))
	redistevents.RegisterProtocol("ospf")

	assert.Empty(t, redistDoctorDiags(redistDoctorTree("ospf", "connected")))
}

// TestRedistributeCheckNamesAnUnknownSource is the error half: the daemon
// refuses to start on this config, and `ze doctor` says so first.
//
// VALIDATES: the diagnostic names both the source and the destination, so the
// operator can find the line.
func TestRedistributeCheckNamesAnUnknownSource(t *testing.T) {
	redistevents.RegisterProtocol("ospf")

	diags := redistDoctorDiags(redistDoctorTree("ospf", "rip"))
	require.Len(t, diags, 1)
	assert.Equal(t, codeRedistUnknownSource, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "rip")
	assert.Contains(t, diags[0].Message, "ospf")
}

// TestRedistributeCheckNamesAnUnknownDestination is the warning half, and it is
// the one nothing else reports. The destination leaf carries no ze:validate,
// the loader accepts any name, and the rules under it are inert forever.
//
// VALIDATES: a destination protocol nothing registered is named.
// PREVENTS: `destination ospv3 { import bgp }` starting cleanly and moving no
// route, which is the silent-zero shape this spec exists to remove.
func TestRedistributeCheckNamesAnUnknownDestination(t *testing.T) {
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "connected", Protocol: "connected"}))

	diags := redistDoctorDiags(redistDoctorTree("ospv3", "connected"))
	require.Len(t, diags, 1)
	assert.Equal(t, codeRedistUnknownDestination, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "ospv3")
}

// TestRedistributeCheckReportsBothHalves proves the two judgements are
// independent. A rule can be wrong on both sides, and the operator is told
// both rather than the first one found.
func TestRedistributeCheckReportsBothHalves(t *testing.T) {
	diags := redistDoctorDiags(redistDoctorTree("ospv3", "rip"))
	assert.ElementsMatch(t,
		[]string{codeRedistUnknownDestination, codeRedistUnknownSource},
		redistDoctorCodes(diags))
}

// TestRedistributeCheckSilentWithNoBlock keeps the check off every config that
// configures no redistribution at all.
func TestRedistributeCheckSilentWithNoBlock(t *testing.T) {
	assert.Empty(t, redistDoctorDiags(NewTree()))
	assert.Empty(t, checkRedistributeRules(diagnostic.DoctorCheckContext{}), "nil tree")
}

// TestRedistributeDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this component's check, at the phase and order the
// declaration states, and whether every code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register_doctor.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestRedistributeDoctorCheckRegistered(t *testing.T) {
	want := redistributeDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
	assert.Equal(t, want.Order, found.Order)
	assert.Equal(t, want.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		assert.NotNil(t, diagnostic.Lookup(code), "diagnostic code %q is not registered", code)
	}
}
