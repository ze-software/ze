// Design: docs/architecture/api/commands.md -- request bgp watchdog dispatch through its YANG tree
// Related: cmd_request.go -- the forwarding handlers under test

package watchdog

import (
	"errors"
	"strings"
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/watchdog/yang" // init() registers ze-watchdog-cmd
)

// TestWatchdogCommandsPassTheirYANGArguments drives each request bgp watchdog
// line the ExaBGP bridge and an operator send through the daemon dispatcher.
//
// VALIDATES: the YANG argument model accepts every form the plugin's own
// grammar reads, including a give whose update text holds the words owner,
// selector and announce, so the line reaches ForwardToPlugin unchanged. With
// no plugin process running, ForwardToPlugin answers ErrUnknownCommand, which
// is the proof the arguments were accepted.
// PREVENTS: a YANG declaration that refuses a line the bridge sends, which
// would stop every watchdog route an ExaBGP script gives.
func TestWatchdogCommandsPassTheirYANGArguments(t *testing.T) {
	lines := []string{
		"request bgp watchdog announce dog",
		"request bgp watchdog announce dog med 100",
		"request bgp watchdog announce dog med 100 *",
		"request bgp watchdog withdraw dog 192.0.2.1",
		"request bgp watchdog give dog owner script-1 selector * announce 4 update text nhop set 192.0.2.1 withdrawal update text nlri ipv4/unicast del 10.0.0.0/24",
		"request bgp watchdog give dog owner script-1 selector 192.0.2.1 withdraw announce 3 owner selector announce withdrawal update text nlri ipv4/unicast del 10.0.0.0/24",
		"request bgp watchdog forget script-1",
	}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx := &pluginserver.CommandContext{Server: server}
	for _, line := range lines {
		_, err := server.Dispatcher().Dispatch(ctx, line)
		// Only ForwardToPlugin, which runs after the arguments are judged,
		// says the plugin may not be running; a line no YANG node took is
		// refused by the dispatcher with other words.
		if !errors.Is(err, pluginserver.ErrUnknownCommand) || !strings.Contains(err.Error(), "plugin may not be running") {
			t.Errorf("%q: want the forward to answer that no plugin serves it, got %v", line, err)
		}
	}
}
