// Design: docs/architecture/api/commands.md -- captured destination exclusion.
package rib

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"testing"
)

// Both real grammar judgements preserve the exact owner; the old clear keeps
// its literal peer selectors because the exclusion has a dedicated path.
// MUTATION: drop owner from ribClearOutExcludingOwnerArgs.
func TestClearOutProxyPreservesCapturedOwnerExclusion(t *testing.T) {
	_, defs := ribClearDispatcher(t)
	for _, tail := range [][]string{
		{"owner", "9007199254741009"},
		{"owner", "18446744073709551615", "family", "ipv4/unicast"},
	} {
		validated, err := command.ValidateArgs(tail, defs[cmdRibClearOutExcludingOwner], nil)
		require.NoError(t, err)
		args, refusal := ribClearOutExcludingOwnerArgs(validated)
		require.Nil(t, refusal)
		require.Equal(t, tail, args)
	}
	for _, tail := range [][]string{{}, {"owner", "0"}, {"owner", "-1"}, {"owner", "18446744073709551616"}, {"owner", "1.5"}, {"owner"}} {
		_, err := command.ValidateArgs(tail, defs[cmdRibClearOutExcludingOwner], nil)
		require.Error(t, err)
	}
	validated, err := command.ValidateArgs([]string{"owner", "12", "extra"}, defs[cmdRibClearOutExcludingOwner], nil)
	if err == nil {
		_, refusal := ribClearOutExcludingOwnerArgs(validated)
		require.NotNil(t, refusal)
	}
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib"})
	for _, result := range host.Dispatcher().Registry().Register(proc, []pluginserver.CommandDef{{Name: cmdRibClearOut}, {Name: cmdRibClearOutExcludingOwner}}) {
		require.True(t, result.OK)
	}
	for _, line := range []string{
		cmdRibClearOutExcludingOwner + " owner 9007199254741009 family ipv4/unicast",
		cmdRibClearOut + " exclude-owner",
		cmdRibClearOut + " owner",
		cmdRibClearOut + " * family ipv4/unicast",
	} {
		_, err = host.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: host, Sender: plugin.OperatorSender()}, line)
		require.ErrorIs(t, err, pluginserver.ErrPluginProcessNotRunning, "actual model/proxy handoff: %s", line)
	}
}
