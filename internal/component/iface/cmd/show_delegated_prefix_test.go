package cmd

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// standInDelegatedPrefixes replaces forwardDelegatedPrefixes with a stand-in
// for the iface plugin process, answering with resp and err.
func standInDelegatedPrefixes(t *testing.T, resp *plugin.Response, err error) *string {
	t.Helper()
	saved := forwardDelegatedPrefixes
	t.Cleanup(func() { forwardDelegatedPrefixes = saved })
	asked := new(string)
	forwardDelegatedPrefixes = func(_ *pluginserver.CommandContext) (*plugin.Response, error) {
		*asked = iface.DelegatedPrefixCommand
		return resp, err
	}
	return asked
}

// TestShowInterfaceDelegatedPrefixAsksThePluginProcess checks that `show
// interface delegated-prefix` answers the rows the iface plugin serves,
// decoded from the JSON that crosses the process boundary, so every surface
// that dispatches the command (CLI, MCP, REST, gRPC, web) gets the same
// answer. Method: a stand-in plugin answers one installed and one waiting row.
func TestShowInterfaceDelegatedPrefixAsksThePluginProcess(t *testing.T) {
	valid, preferred := uint32(3000), uint32(1200)
	sample := []iface.DelegatedUnitStatus{
		{
			Interface: "lan0", Unit: "0", Source: "pppoe0", SubnetID: 1, State: "installed", Device: "lan0",
			Address:       netip.MustParsePrefix("2001:db8:aa00:1::1/64"),
			Prefix:        netip.MustParsePrefix("2001:db8:aa00:1::/64"),
			Delegated:     netip.MustParsePrefix("2001:db8:aa00::/56"),
			ValidLifetime: &valid, PreferredLifetime: &preferred,
		},
		{Interface: "lan1", Unit: "0", Source: "pppoe0", SubnetID: 2, State: "waiting", Reason: "pppoe-client pppoe0 holds no delegated prefix"},
	}
	asked := standInDelegatedPrefixes(t, &plugin.Response{Status: plugin.StatusDone, Data: rawJSON(t, sample)}, nil)

	resp, err := handleShowInterfaceDelegatedPrefix(&pluginserver.CommandContext{}, commandtest.Args())
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status %q, error %q", resp.Status, resp.Error)
	}
	if *asked != iface.DelegatedPrefixCommand {
		t.Fatalf("forwarded %q, want %q", *asked, iface.DelegatedPrefixCommand)
	}
	rows, ok := resp.Data.(plugin.Slice[iface.DelegatedUnitStatus])
	if !ok {
		t.Fatalf("data %T, want plugin.Slice[iface.DelegatedUnitStatus]", resp.Data)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	got := rows[0]
	if got.Prefix != sample[0].Prefix || got.Address != sample[0].Address || got.Source != "pppoe0" {
		t.Errorf("installed row lost its /64, address or source: %+v", got)
	}
	if got.ValidLifetime == nil || *got.ValidLifetime != valid {
		t.Errorf("installed row valid-lifetime %v, want %d", got.ValidLifetime, valid)
	}
	if rows[1].ValidLifetime != nil {
		t.Errorf("waiting row carries a valid-lifetime %d it cannot know", *rows[1].ValidLifetime)
	}
}

// TestShowInterfaceDelegatedPrefixPluginAbsent checks the refusal when no
// process serves the command: it names the missing plugin and the command
// that starts it, never an empty list that reads as "no delegations".
func TestShowInterfaceDelegatedPrefixPluginAbsent(t *testing.T) {
	standInDelegatedPrefixes(t, nil, pluginserver.ErrUnknownCommand)
	resp, err := handleShowInterfaceDelegatedPrefix(&pluginserver.CommandContext{}, commandtest.Args())
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want error", resp.Status)
	}
	for _, want := range []string{"delegated prefix status unavailable", "is not running", "set interface backend netlink"} {
		if !strings.Contains(resp.Error, want) {
			t.Errorf("refusal %q lacks %q", resp.Error, want)
		}
	}
}

// TestShowInterfacePPPoEClientRefusalUnchanged checks that sharing the
// forwarding helper kept the PPPoE client refusal text the operator reads.
func TestShowInterfacePPPoEClientRefusalUnchanged(t *testing.T) {
	saved := forwardPPPoEClients
	t.Cleanup(func() { forwardPPPoEClients = saved })
	forwardPPPoEClients = func(_ *pluginserver.CommandContext) (*plugin.Response, error) {
		return nil, pluginserver.ErrUnknownCommand
	}
	resp, err := handleShowInterfacePPPoEClient(&pluginserver.CommandContext{}, commandtest.Args())
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	want := "PPPoE client status unavailable: the interface plugin, which runs the PPPoE clients, is not running. " +
		"Ze starts it only when the configuration holds an interface section. " +
		"To start it, add the section and commit: set interface backend netlink"
	if resp.Error != want {
		t.Errorf("refusal %q, want %q", resp.Error, want)
	}
}
