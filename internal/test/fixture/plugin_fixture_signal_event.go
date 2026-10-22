// Design: docs/architecture/api/json-format.md -- the signal event
// Related: plugin_fixture_12.go -- registers plugin/signal-event-json and plugin/signal-event-text
// Related: plugin_fixture_fsm_events.go -- the connected and fsm events, judged the same way

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// p12SignalEventJSON is the JSON half of test/plugin/signal-event.ci. The
// plugin subscribes to `signal`, sends SIGHUP to the daemon itself (the pid
// the runner published in daemon.pid), and passes only when the daemon
// delivers `"signal":{"code":1,"name":"SIGHUP"}` for the configured peer.
// It is the only half that sends the signal, so one SIGHUP serves both.
func p12SignalEventJSON(plugin *sdk.Plugin) p12Scenario {
	return p12SignalEvent(plugin, "json", true, func(event string) bool {
		var decoded struct {
			BGP struct {
				Message struct {
					Type string `json:"type"`
				} `json:"message"`
				Signal struct {
					Code int    `json:"code"`
					Name string `json:"name"`
				} `json:"signal"`
			} `json:"bgp"`
		}
		if json.Unmarshal([]byte(event), &decoded) != nil {
			return false
		}
		if decoded.BGP.Message.Type != eventSignal {
			return false
		}
		if decoded.BGP.Signal.Code != int(syscall.SIGHUP) {
			return false
		}
		return decoded.BGP.Signal.Name == "SIGHUP"
	})
}

// p12SignalEventText is the text half: the same SIGHUP must reach a process
// that asks for text encoding as `peer <address> remote as <asn> signal 1 SIGHUP`.
func p12SignalEventText(plugin *sdk.Plugin) p12Scenario {
	plugin.SetEncoding("text")
	return p12SignalEvent(plugin, "text", false, func(event string) bool {
		return strings.HasPrefix(event, "peer 127.0.0.1 remote as 1 signal 1 SIGHUP")
	})
}

// p12SignalEvent records every event the matcher accepts in the named encoding, sends SIGHUP to the
// daemon once it is ready when sendSignal is set, and waits up to ten seconds
// for one accepted event. A timeout returns an error, so the runner reports
// the ZE-OBSERVER-FAIL sentinel rather than trusting the exit code.
func p12SignalEvent(plugin *sdk.Plugin, encoding string, sendSignal bool, matches func(string) bool) p12Scenario {
	seen := make(chan string, 16)
	plugin.OnEvent(func(event string) error {
		if !matches(event) {
			return nil
		}
		select {
		case seen <- event:
		default:
		}
		return nil
	})

	return func(ctx context.Context, _ *sdk.Plugin) error {
		if sendSignal {
			if err := p12AwaitDaemonReady(ctx); err != nil {
				return err
			}
			if err := p12SignalDaemon(fileDaemonPID); err != nil {
				return err
			}
		}
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("no signal event for SIGHUP reached the plugin within 10s")
		case event := <-seen:
			fmt.Fprintln(os.Stderr, "OK: signal event delivered as", encoding+":", strings.TrimSpace(event))
			return nil
		}
	}
}

// p12AwaitDaemonReady waits for the runner's readiness files, which exist
// only after the daemon registered its signal handlers. The plugin runs in the
// daemon's working directory, so the files resolve by bare name.
func p12AwaitDaemonReady(ctx context.Context) error {
	for _, path := range []string{fileDaemonPID, fileDaemonReady} {
		if !waitForFile(ctx, path, 100, 100*time.Millisecond) {
			return fmt.Errorf("signal fixture timed out waiting for %s", path)
		}
	}
	return nil
}
