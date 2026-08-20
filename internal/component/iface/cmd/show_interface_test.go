package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// These cover the `show interface` family, moved here with the handlers from
// internal/component/cmd/show. Most skip when no iface backend is loaded (the
// unit-test default), exercising the read paths when one is.
//
// The dispatch half of the contract -- that the daemon hands each handler the
// tokens the operator typed -- is NOT provable here: these call the handlers
// directly, so they pass whatever args the test chooses. It is proven end to end
// by test/plugin/interface-type-show.ci and test/plugin/interface-errors-show.ci
// (ai/rules/evidence.md, "drive the guard from the entry point").

// skipWithoutBackend skips when the response is the no-backend refusal.
func skipWithoutBackend(t *testing.T, resp *plugin.Response) {
	t.Helper()
	if resp.Status == "error" && resp.Error == "iface: no backend loaded" {
		t.Skip("iface backend not available in test environment")
	}
}

func TestHandleShowInterface(t *testing.T) {
	resp, err := handleShowInterface(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	skipWithoutBackend(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestHandleShowInterfaceRejectsStrayToken pins the bare handler's contract: it
// owns the no-argument form only. Every keyword has its own wire method, so a
// leftover token is a typo, not a subcommand, and must be refused with the usage
// text rather than silently listing every interface.
func TestHandleShowInterfaceRejectsStrayToken(t *testing.T) {
	for _, arg := range []string{"lo", "brief", "errors", "type", "rate"} {
		resp, err := handleShowInterface(nil, []string{arg})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "error", resp.Status, "stray token %q must not be served", arg)
		assert.Equal(t, usageShowInterface, resp.Error)
	}
}

// TestHandleShowInterfaceBrief verifies the brief handler returns the compact
// shape, not the full InterfaceInfo list.
//
// VALIDATES: show interface brief reaches showInterfaceBrief.
// PREVENTS: brief falling back to the full-detail listing.
func TestHandleShowInterfaceBrief(t *testing.T) {
	resp, err := handleShowInterfaceBrief(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	skipWithoutBackend(t, resp)
	require.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "brief response should be a map, got %T", resp.Data)
	require.Contains(t, data, "interfaces")
	require.Contains(t, data, "count")

	rows, ok := data["interfaces"].([]map[string]any)
	require.True(t, ok, "interfaces should be rows, got %T", data["interfaces"])
	for _, row := range rows {
		// The brief row carries name/state/mtu (+ optional address) and nothing
		// else. A full InterfaceInfo would bring index, type and stats with it.
		assert.Contains(t, row, "name")
		assert.NotContains(t, row, "stats", "brief must not carry full detail")
		assert.NotContains(t, row, "index", "brief must not carry full detail")
	}
}

// TestHandleShowInterfaceTypeNeedsAType checks the fail-closed half: with no
// type token there is nothing to filter on, so the handler must refuse rather
// than answer with the unfiltered list.
func TestHandleShowInterfaceTypeNeedsAType(t *testing.T) {
	resp, err := handleShowInterfaceType(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "usage: show interface type <type>", resp.Error)
}

// TestHandleShowInterfaceTypeRejectsUnknown checks that an unmatched type is
// refused and that the refusal derives the valid list from the running set
// (ai/rules/evidence.md, derive never hardcode).
func TestHandleShowInterfaceTypeRejectsUnknown(t *testing.T) {
	all, err := handleShowInterface(nil, nil)
	require.NoError(t, err)
	skipWithoutBackend(t, all)

	resp, err := handleShowInterfaceType(nil, []string{"zz-not-an-interface-type"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "unknown interface type")
	assert.True(t,
		strings.Contains(resp.Error, "valid types:") ||
			strings.Contains(resp.Error, "no interfaces have a classified type"),
		"refusal must say what is valid, got %q", resp.Error)
}

// TestHandleShowInterfaceErrorsShape checks the errors view returns the wrapper
// the table renderer unwraps, and that every row carries all four counters.
func TestHandleShowInterfaceErrorsShape(t *testing.T) {
	resp, err := handleShowInterfaceErrors(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	skipWithoutBackend(t, resp)
	require.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "errors response should be a map, got %T", resp.Data)
	rows, ok := data["interfaces"].([]map[string]any)
	require.True(t, ok, "interfaces should be rows, got %T", data["interfaces"])
	for _, row := range rows {
		for _, key := range []string{"name", "rx-errors", "rx-dropped", "tx-errors", "tx-dropped"} {
			assert.Contains(t, row, key)
		}
	}
}

// TestHandleShowInterfaceRateNamedFormUsesTheName drives the handler the
// dispatcher registers for `ze-show:interface-rate` with the named form, which
// nothing exercised before: test/plugin/interface-rate-show.ci and
// test/plugin/iface-rate-json.ci both send the bare `show interface rate`.
//
// VALIDATES: `show interface rate <name>` reaches iface.GetRate with the name
// the operator typed, and an unknown name is refused with a message naming it
// rather than crashing or answering with the whole list.
// PREVENTS: the named form losing its argument. The bare form answers
// "rate tracker not running" (or the full list once a tracker is up), so an
// argument dropped anywhere between the wire method and handleShowInterfaceRate
// changes this exact string -- which is why the assertion is on the string and
// not merely on the error status.
func TestHandleShowInterfaceRateNamedFormUsesTheName(t *testing.T) {
	const name = "zz-not-an-interface0"

	resp, err := handleShowInterfaceRateCmd(nil, []string{name})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, "interface not found: "+name, resp.Error)
	assert.Nil(t, resp.Data, "a refused lookup must carry no rate payload")
}
