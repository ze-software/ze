// Design: docs/guide/command-reference.md -- NTP state enriches show system date.
// Related: internal/plugins/ntp/register.go -- the runtime NTP state owner.

package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// ntpForkedSourceField is the key under which the NTP status answer names its
// synchronization source.
const ntpForkedSourceField = "source"

func init() {
	Register("plugin/system-date-ntp-forked", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "system-date-ntp-forked-test", &sdk.Registration{}, observeForkedNTPState)
	}, StopsDaemon)
}

// observeForkedNTPState reads an enabled NTP process with no time servers. The
// worker publishes its real unsynchronized state. The explicit fork installs no
// daemon store, so it cannot restore a saved clock, and no server can supply a
// response. The ready callback precedes this scenario; the poll waits for that
// worker's first state publication rather than assuming its goroutine ran.
func observeForkedNTPState(ctx context.Context, p *sdk.Plugin) error {
	var state map[string]any
	var last error
	if !Poll(ctx, 60, 50*time.Millisecond, func() bool {
		state, last = plugin16DoneMap(ctx, p, "show system ntp", "forked NTP")
		if last != nil {
			return false
		}
		return state["enabled"] == true
	}) {
		if last != nil {
			return fmt.Errorf("forked NTP never reported enabled=true: state=%v: %w", state, last)
		}
		return fmt.Errorf("forked NTP never reported enabled=true: state=%v", state)
	}
	if synced, ok := state["synced"].(bool); !ok || synced {
		return fmt.Errorf("forked NTP synced=%v, want worker-published false", state["synced"])
	}
	if source, ok := state[ntpForkedSourceField].(string); !ok || source != "" {
		return fmt.Errorf("forked NTP source=%v, want the worker's empty source", state[ntpForkedSourceField])
	}
	if offset, ok := state["offset"].(string); !ok || offset != "0s" {
		return fmt.Errorf("forked NTP offset=%v, want worker-published 0s", state["offset"])
	}
	date, err := plugin16DoneMap(ctx, p, "show system date", "date with forked NTP")
	if err != nil {
		return err
	}
	for _, field := range []struct{ date, state string }{
		{date: "ntp-synced", state: "synced"},
		{date: "ntp-source", state: ntpForkedSourceField},
		{date: "ntp-offset", state: "offset"},
	} {
		actual, present := date[field.date]
		if !present {
			return fmt.Errorf("date omitted forked NTP field %q: %v", field.date, date)
		}
		if actual != state[field.state] {
			return fmt.Errorf("date %s=%v differs from NTP %s=%v", field.date, actual, field.state, state[field.state])
		}
	}
	if stamp, ok := date["time"].(string); !ok || stamp == "" {
		return fmt.Errorf("date time=%v, want the daemon wall clock", date["time"])
	}
	peers, err := plugin16DoneMap(ctx, p, "show system ntp peers", "forked NTP peers")
	if err != nil {
		return err
	}
	rows, ok := peers["peers"].([]any)
	if !ok {
		return fmt.Errorf("forked NTP peers=%T, want an array", peers["peers"])
	}
	if len(rows) != 0 {
		return fmt.Errorf("forked NTP has %d peers, want no configured servers", len(rows))
	}
	count, ok := plugin16Number(peers["count"])
	if !ok || count != 0 {
		return fmt.Errorf("forked NTP peer count=%v, want 0", peers["count"])
	}
	fmt.Fprintln(os.Stderr, "OK: forked NTP status and date preserve unsynchronized state and zero offset; peers=0")
	return nil
}
