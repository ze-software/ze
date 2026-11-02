package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// standInPlugin replaces forwardRates with a stand-in for the iface plugin
// process: it answers RateCommand with the JSON document the plugin sends
// over its connection, the only thing that crosses the process boundary.
func standInPlugin(t *testing.T, answer func(command string, args []string) *plugin.Response) {
	t.Helper()
	saved := forwardRates
	t.Cleanup(func() { forwardRates = saved })
	forwardRates = func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		return answer(iface.RateCommand, args), nil
	}
}

func rawJSON(t *testing.T, v any) plugin.RawJSON {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return plugin.RawJSON(b)
}

// TestMonitorInterfaceRateAsksThePluginProcess checks that the one-shot answer
// of `monitor interface rate` carries the rates the iface plugin serves, while
// this process's own tracker was never started, which is the daemon's state
// whenever the plugin runs external. Method: the plugin is a stand-in that
// answers over the boundary's JSON; iface.ListRates is nil throughout, so a
// handler that read the daemon's tracker answers "rate tracker not running".
func TestMonitorInterfaceRateAsksThePluginProcess(t *testing.T) {
	if iface.ListRates() != nil {
		t.Fatal("precondition: this process must hold no running rate tracker")
	}
	sample := []iface.InterfaceRate{{Name: "eth0", RxBps: 800}, {Name: "eth1", TxPps: 3}}
	var asked string
	standInPlugin(t, func(command string, args []string) *plugin.Response {
		asked = command
		if len(args) > 0 {
			if len(args) != 1 || args[0] != "eth1" {
				t.Errorf("forwarded args %q, want [eth1]: the name must reach the plugin", args)
			}
			return &plugin.Response{Status: plugin.StatusDone, Data: rawJSON(t, sample[1])}
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: rawJSON(t, sample)}
	})

	resp, err := handleMonitorInterfaceRate(&pluginserver.CommandContext{}, commandtest.Args())
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status %q, error %q: want the plugin's rates", resp.Status, resp.Error)
	}
	if asked != iface.RateCommand {
		t.Fatalf("forwarded %q, want %q", asked, iface.RateCommand)
	}
	list, ok := resp.Data.(plugin.Slice[iface.InterfaceRate])
	if !ok {
		t.Fatalf("data %T, want plugin.Slice[iface.InterfaceRate]", resp.Data)
	}
	if len(list) != 2 || list[0].Name != "eth0" || list[0].RxBps != 800 || list[1].TxPps != 3 {
		t.Fatalf("rates %+v, want %+v", list, sample)
	}

	one, err := handleShowInterfaceRateCmd(&pluginserver.CommandContext{}, commandtest.Args("eth1"))
	if err != nil {
		t.Fatalf("show handler error: %v", err)
	}
	rate, ok := one.Data.(iface.InterfaceRate)
	if !ok || rate.Name != "eth1" || rate.TxPps != 3 {
		t.Fatalf("show interface rate eth1 answered %+v (%T)", one.Data, one.Data)
	}
}

// TestRateRefusalsNameTheCause checks that a plugin that cannot answer is
// reported with the plugin's own words, or with why it was not reached, and
// never as rates. Method: stand-ins for an unknown interface, an absent
// plugin command, and no dispatcher at all.
func TestRateRefusalsNameTheCause(t *testing.T) {
	standInPlugin(t, func(string, []string) *plugin.Response {
		return &plugin.Response{Status: plugin.StatusError, Error: "interface not found: eth9"}
	})
	resp, err := handleMonitorInterfaceRate(&pluginserver.CommandContext{}, commandtest.Args("eth9"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError || resp.Error != "interface not found: eth9" {
		t.Fatalf("got %q %q, want the plugin's refusal", resp.Status, resp.Error)
	}

	forwardRates = func(*pluginserver.CommandContext, []string) (*plugin.Response, error) {
		return nil, errors.New(`plugin command "show interface rate" not registered (plugin may not be running)`)
	}
	resp, _ = handleMonitorInterfaceRate(&pluginserver.CommandContext{}, commandtest.Args())
	if resp.Status != plugin.StatusError || !strings.HasPrefix(resp.Error, "interface rates unavailable: ") {
		t.Fatalf("got %q %q, want an unavailable refusal", resp.Status, resp.Error)
	}
}

// TestRatesWithoutDispatcherRefuse checks the production forwarder refuses
// when the command context reaches no dispatcher, rather than answering empty.
func TestRatesWithoutDispatcherRefuse(t *testing.T) {
	resp, err := handleMonitorInterfaceRate(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError || !strings.Contains(resp.Error, errNoDispatcher.Error()) {
		t.Fatalf("got %q %q, want the no-dispatcher refusal", resp.Status, resp.Error)
	}
}

// TestRatesWithoutPluginNameTheFix checks that a rate call finding no process
// serving the plugin command, which is what the dispatcher answers when the
// iface plugin is not running, refuses with the cause and the fix: the
// plugin is missing because no interface section is configured, and the
// command that adds one. The dispatcher's own text names only the command.
func TestRatesWithoutPluginNameTheFix(t *testing.T) {
	saved := forwardRates
	t.Cleanup(func() { forwardRates = saved })
	forwardRates = func(*pluginserver.CommandContext, []string) (*plugin.Response, error) {
		return nil, errors.Join(errors.New(`plugin command "show interface rate" not registered`), pluginserver.ErrUnknownCommand)
	}
	resp, err := handleMonitorInterfaceRate(&pluginserver.CommandContext{}, commandtest.Args())
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	want := "interface rates unavailable: the interface plugin, which measures interface rates, is not running. " +
		"Ze starts it only when the configuration holds an interface section. " +
		"To start it, add the section and commit: set interface backend netlink"
	if resp.Status != plugin.StatusError || resp.Error != want {
		t.Fatalf("got %q %q, want %q", resp.Status, resp.Error, want)
	}
}
