package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// thirdPartyInsertion is the one insertion the tests below declare.
var thirdPartyInsertion = rpc.FilterInsertion{
	Filter:    "third-party:judge",
	Direction: rpc.FilterImport,
	Priority:  100,
	Config:    []string{"fakefilter", "insert"},
	Values:    []string{"true"},
}

// TestRegistrationFromRPCKeepsFilterInsertions checks
// spec-filter-wire-6-stage-filters A-12 at the sink: the insertions a
// plugin declares at Stage 1 are stored on its registration, because a
// third-party plugin has no compiled registration that could carry them.
// Method: convert a declaration carrying one insertion and read it back.
func TestRegistrationFromRPCKeepsFilterInsertions(t *testing.T) {
	reg := registrationFromRPC(&rpc.DeclareRegistrationInput{
		FilterInsertions: []rpc.FilterInsertion{thirdPartyInsertion},
	})
	require.Equal(t, []rpc.FilterInsertion{thirdPartyInsertion}, reg.FilterInsertions,
		"the Stage 1 insertions are the only record a third-party plugin's insertions have")
}

// TestThirdPartyFilterInsertionsAnswerProcessesWithNoCompiledRegistration
// checks the seam config load reads (registry.RuntimeFilterInsertions): it
// answers the Stage 1 insertions of a running process no compiled
// registration describes, and leaves out a process a compiled registration
// describes, whose insertions config load already reads from registry.All().
// Method: one external process with insertions, one process filed under a
// compiled registration with the same insertions, one process that declared
// nothing yet.
func TestThirdPartyFilterInsertionsAnswerProcessesWithNoCompiledRegistration(t *testing.T) {
	const compiled = "test-compiled-inserter"
	registerSessionReadyPlugin(t, compiled, false)

	s := sessionReadyTestServer()
	external := runningProcess(s, "third-party", "/usr/local/bin/third-party-filter")
	external.SetRegistration(&plugin.PluginRegistration{FilterInsertions: []rpc.FilterInsertion{thirdPartyInsertion}})
	aliased := runningProcess(s, "aliased", compiled)
	aliased.SetRegistration(&plugin.PluginRegistration{FilterInsertions: []rpc.FilterInsertion{thirdPartyInsertion}})
	runningProcess(s, "not-declared-yet", "/usr/local/bin/other")

	got := s.thirdPartyFilterInsertions()
	require.Len(t, got, 1, "only the process with no compiled registration answers")
	assert.Equal(t, "third-party", got[0].Process)
	assert.Equal(t, []rpc.FilterInsertion{thirdPartyInsertion}, got[0].Insertions)

	assert.Empty(t, sessionReadyTestServerNoManager().thirdPartyFilterInsertions(),
		"a server with no process manager runs no process")
}

func sessionReadyTestServerNoManager() *Server {
	return &Server{config: &ServerConfig{}, registry: plugin.NewPluginRegistry(), loadedPlugins: make(map[string]bool)}
}
