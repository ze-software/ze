package server

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestEngineSinkNamesTheRegistrationTheBlockRuns proves the engine's Stage 1
// sink hands the driver the registration an aliased block runs, and the
// block's own name where the block names no registry row.
//
// VALIDATES: the engine half of the aliased Stage 1 judgement.
// PREVENTS: the driver judging `internal gr { use bgp-gr; }` as "gr".
// Method: engine sinks over hand-built processes.
func TestEngineSinkNamesTheRegistrationTheBlockRuns(t *testing.T) {
	registerSessionReadyPlugin(t, aliasTestRegistration, false)
	aliased := &engineStartupSink{proc: process.NewProcess(plugin.PluginConfig{
		Name: "gr", Internal: true, Run: aliasTestRegistration,
	})}
	assert.Equal(t, "gr", aliased.pluginName())
	assert.Equal(t, aliasTestRegistration, aliased.registrationName())

	thirdParty := &engineStartupSink{proc: process.NewProcess(plugin.PluginConfig{
		Name: "my-filter", Run: "/usr/local/bin/my-filter-binary",
	})}
	assert.Equal(t, "my-filter", thirdParty.registrationName(),
		"a program no registry row describes declares under its block name")
}

// TestTwoBlocksRunningOneFilterPluginAreRefused proves a reference that two
// blocks could serve is refused rather than routed to either: Stage 1
// refuses the declaration naming both blocks and the fix, and the dispatch
// guard answers the same refusal. A plugin declaring no filter keeps its
// two blocks.
//
// VALIDATES: `internal gr { use bgp-gr; }` beside `internal bgp-gr { }`
// cannot leave `bgp-gr:llgr-ingress` ambiguous.
// PREVENTS: a route judged by whichever block a map walk met first.
// Method: a process manager holding two blocks of one registration.
func TestTwoBlocksRunningOneFilterPluginAreRefused(t *testing.T) {
	registerSessionReadyPlugin(t, aliasTestRegistration, false)
	s, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	pm := process.NewProcessManager(nil)
	s.procManager.Store(pm)
	var calls atomic.Int32
	first := aliasedSlotProcess(t, "gr", aliasTestRegistration, rpc.FilterAccept, &calls)
	pm.AddProcess("gr", first)
	pm.AddProcess("gr-two", aliasedSlotProcess(t, "gr-two", aliasTestRegistration, rpc.FilterAccept, &calls))

	declaring := &rpc.DeclareRegistrationInput{Filters: []rpc.FilterDecl{{Name: "ingress", Direction: rpc.FilterImport}}}
	err = s.refuseAmbiguousRegistration(first, declaring)
	require.Error(t, err, "Stage 1 accepted a second block of a filter plugin")
	for _, want := range []string{`"gr"`, `"gr-two"`, `"` + aliasTestRegistration + `"`, "keep one block"} {
		assert.Contains(t, err.Error(), want)
	}
	assert.NoError(t, s.refuseAmbiguousRegistration(first, &rpc.DeclareRegistrationInput{}),
		"a plugin declaring no filter was refused for running twice")

	input := rpc.FilterUpdateInput{Filter: "ingress"}
	route := rpc.InputFilterRoute(&input)
	_, err = s.CallFilterUpdate(context.Background(), aliasTestRegistration, &route)
	require.Error(t, err, "an ambiguous reference was routed to one of two blocks")
	assert.Contains(t, err.Error(), "keep one block")
	assert.Equal(t, int32(0), calls.Load(), "a block judged a route the reference could not attribute")
}
