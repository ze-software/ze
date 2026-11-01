package server

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// chainRefusedMessage is the refusal the test judges answer, worded like the
// BGP judge's (bgp/config judgeExternalChainRef), so the assertions read the
// judge's message through the startup error.
const chainRefusedMessage = `peer 192.0.2.1 filter import: chain names "chain-refused:f", ` +
	`and plugin "chain-refused" declares no filter of that name; it declares filter g`

// runChainJudgedPlugin installs judge as the Stage 1 filter judge, starts one
// plugin of the default (ignore) failure policy that declares a filter, and
// answers the server once the phase has run. The phase error is returned too:
// the plugin was refused at Stage 1, so it is never nil.
func runChainJudgedPlugin(t *testing.T, judge func(string, []registry.DeclaredFilter) error) (*Server, error) {
	t.Helper()
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })
	previous := registry.SetFilterDeclarationCheck(judge)
	t.Cleanup(func() { registry.SetFilterDeclarationCheck(previous) })

	const name = "chain-refused"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares one filter and nothing about its own failure",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{
				Filters: []sdk.FilterDecl{{Name: "g", Direction: rpc.FilterImport}},
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	})
	return s, err
}

// TestStartupRefusesALoadedChainItsPluginRefuses verifies that a chain the
// loaded config holds, refused by the Stage 1 filter judge when its plugin
// declares, fails daemon startup rather than one plugin.
//
// VALIDATES: spec-filter-wire-0-umbrella AC-5 and AC-21 at daemon start: a
// chain reference no declaration covers, or whose declared fields the
// attachment does not fit, fails the config the way a reload refuses it at
// load, whatever the plugin's failure policy. The startup error names the
// reference, the plugin, and the fix.
// PREVENTS: the state the wiring tests found, where the refusal was a plugin
// startup failure under the default ignore policy and the daemon ran on with
// the chain still naming the refused filter.
func TestStartupRefusesALoadedChainItsPluginRefuses(t *testing.T) {
	s, err := runChainJudgedPlugin(t, func(string, []registry.DeclaredFilter) error {
		return registry.RefuseLoadedChain(errors.New(chainRefusedMessage))
	})

	require.Error(t, err, "the plugin was refused at Stage 1, so the phase must fail")
	require.Error(t, s.startupErr, "a refused chain in the loaded config must fail daemon startup")
	assert.ErrorIs(t, s.startupErr, registry.ErrLoadedChainRefused)
	assert.Contains(t, s.startupErr.Error(), chainRefusedMessage, "the startup error carries the judge's refusal")
	assert.Contains(t, s.startupErr.Error(), "ze does not start", "the startup error says what ze does")
	assert.Contains(t, s.startupErr.Error(), "correct the chain reference", "the startup error names the fix")
}

// TestStartupKeepsThePolicyForARefusedDeclaration is the polarity that makes
// the test above discriminate: a judge refusal that holds no loaded chain (a
// declaration the vocabulary does not hold) is the plugin's own failure, and
// its failure policy, here the default ignore, decides what ze does.
//
// PREVENTS: a startup path that fails the daemon for every Stage 1 filter
// refusal, which would pass the test above while stopping ze for an optional
// plugin whose declaration alone is wrong.
func TestStartupKeepsThePolicyForARefusedDeclaration(t *testing.T) {
	s, err := runChainJudgedPlugin(t, func(string, []registry.DeclaredFilter) error {
		return errors.New(`plugin "chain-refused" filter g reads colour, which is not a route field`)
	})

	require.Error(t, err, "the plugin was refused at Stage 1, so the phase must fail")
	assert.NoError(t, s.startupErr, "a refused declaration with no loaded chain must leave the policy to decide")
}
