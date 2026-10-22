// Design: docs/architecture/exabgp-bridge.md -- the shutdown line
// Related: plugin_fixture_06.go -- registers plugin/exabgp-shutdown-helper
// Related: plugin_fixture_06_exec.go -- the other ExaBGP-style helpers

package fixture

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// exabgpShutdownHelper is the ExaBGP script of
// test/plugin/exabgp-bridge-shutdown-*.ci. It takes one argument, the format
// the bridge writes it (`json` or `text`). It reads ExaBGP lines from stdin,
// asks the daemon to stop with ExaBGP's `shutdown` command once its neighbor is
// up, and passes only when the bridge then writes ExaBGP's shutdown line in
// that format before stdin closes: JSON `"notification": "shutdown"` with
// `"type": "notification"`, text `shutdown <pid> <ppid>`
// (exabgp Response.JSON.shutdown and Response.Text.shutdown).
//
// Every line read is traced on stderr, which the bridge passes through, so a
// failure shows what the script did see. The verdict is the exit status and
// the OK line the .ci asserts.
func exabgpShutdownHelper(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("exabgp shutdown helper takes one argument, json or text")
	}
	isUp, isShutdown, err := exabgpShutdownMatchers(args[0])
	if err != nil {
		return err
	}
	lines := bufio.NewScanner(os.Stdin)
	lines.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	asked := false
	for lines.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := lines.Text()
		fmt.Fprintln(os.Stderr, "exabgp-shutdown-helper read:", line)
		if isShutdown(line) {
			if !asked {
				return fmt.Errorf("shutdown line before the helper asked for a stop: %s", line)
			}
			fmt.Fprintln(os.Stderr, "OK: exabgp shutdown line in", args[0]+":", line)
			return nil
		}
		if !asked && isUp(line) {
			fmt.Println("shutdown")
			asked = true
		}
	}
	if err := lines.Err(); err != nil {
		return err
	}
	return fmt.Errorf("stdin closed with no shutdown line (stop asked: %t)", asked)
}

// exabgpShutdownMatchers answers, for one bridge format, the test for the
// neighbor-up line and the test for the shutdown line.
func exabgpShutdownMatchers(format string) (isUp, isShutdown func(string) bool, err error) {
	switch format {
	case "json":
		return exabgpJSONIsUp, exabgpJSONIsShutdown, nil
	case "text":
		return exabgpTextIsUp, exabgpTextIsShutdown, nil
	}
	return nil, nil, fmt.Errorf("unknown format %q, want json or text", format)
}

func exabgpJSONIsUp(line string) bool {
	var event struct {
		Type     string `json:"type"`
		Neighbor struct {
			State string `json:"state"`
		} `json:"neighbor"`
	}
	if json.Unmarshal([]byte(line), &event) != nil {
		return false
	}
	if event.Type != "state" {
		return false
	}
	return event.Neighbor.State == "up"
}

func exabgpJSONIsShutdown(line string) bool {
	var event struct {
		Type         string `json:"type"`
		Notification string `json:"notification"`
	}
	if json.Unmarshal([]byte(line), &event) != nil {
		return false
	}
	if event.Type != "notification" {
		return false
	}
	return event.Notification == "shutdown"
}

// exabgpTextIsUp reads `neighbor <address> up`.
func exabgpTextIsUp(line string) bool {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return false
	}
	if fields[0] != "neighbor" {
		return false
	}
	return fields[2] == "up"
}

// exabgpTextIsShutdown reads `shutdown <pid> <ppid>`, both positive integers.
func exabgpTextIsShutdown(line string) bool {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return false
	}
	if fields[0] != "shutdown" {
		return false
	}
	for _, id := range fields[1:] {
		value, err := strconv.Atoi(id)
		if err != nil {
			return false
		}
		if value < 1 {
			return false
		}
	}
	return true
}
