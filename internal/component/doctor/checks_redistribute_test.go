package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// redistTree builds a `redistribute { destination <dest> { import <source> } }`
// tree directly, with no parse step. The check is then driven by the shape it
// reads rather than by YANG.
func redistTree(dest, source string) *config.Tree {
	tree := config.NewTree()
	redist := tree.GetOrCreateContainer("redistribute")
	destination := config.NewTree()
	destination.AddListEntry("import", source, config.NewTree())
	redist.AddListEntry("destination", dest, destination)
	return tree
}

// codes flattens a diagnostic list to the codes it carries, which is what the
// assertions are about.
func codes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

// TestRedistributeCheckSilentOnAWorkingRule keeps the check from crying wolf.
//
// VALIDATES: a rule whose source is registered and whose destination protocol
// registered an identity produces no diagnostic.
// PREVENTS: a warning on every correct config, which an operator learns to
// ignore and which then hides the one that matters.
func TestRedistributeCheckSilentOnAWorkingRule(t *testing.T) {
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "connected", Protocol: "connected"}))
	redistevents.RegisterProtocol("ospf")

	assert.Empty(t, checkRedistributeRules(redistTree("ospf", "connected")))
}

// TestRedistributeCheckNamesAnUnknownSource is the error half: the daemon
// refuses to start on this config, and `ze doctor` says so first.
//
// VALIDATES: the diagnostic names both the source and the destination, so the
// operator can find the line.
func TestRedistributeCheckNamesAnUnknownSource(t *testing.T) {
	redistevents.RegisterProtocol("ospf")

	diags := checkRedistributeRules(redistTree("ospf", "rip"))
	require.Len(t, diags, 1)
	assert.Equal(t, diagnosticRedistUnknownSource, diags[0].Code)
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
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "connected", Protocol: "connected"}))

	diags := checkRedistributeRules(redistTree("ospv3", "connected"))
	require.Len(t, diags, 1)
	assert.Equal(t, diagnosticRedistUnknownDestination, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "ospv3")
}

// TestRedistributeCheckReportsBothHalves proves the two judgements are
// independent. A rule can be wrong on both sides, and the operator is told
// both rather than the first one found.
func TestRedistributeCheckReportsBothHalves(t *testing.T) {
	diags := checkRedistributeRules(redistTree("ospv3", "rip"))
	assert.ElementsMatch(t,
		[]string{diagnosticRedistUnknownDestination, diagnosticRedistUnknownSource},
		codes(diags))
}

// TestRedistributeCheckSilentWithNoBlock keeps the check off every config that
// configures no redistribution at all.
func TestRedistributeCheckSilentWithNoBlock(t *testing.T) {
	assert.Empty(t, checkRedistributeRules(config.NewTree()))
	assert.Empty(t, checkRedistributeRules(nil))
}
