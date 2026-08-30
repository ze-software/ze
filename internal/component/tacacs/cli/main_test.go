// Design: docs/architecture/aaa-tacacs.md -- ze tacacs show reachability probe
//
// VALIDATES: `show tacacs servers` answers with structured rows that the pipe
//   layer renders, so `| json` and `| yaml` are two renderings of ONE payload
//   and an operator the answer's shape cannot support is refused by name
//   (ai/rules/cli.md, "--flag or Keyword").
// PREVENTS: the deleted `--json` flag coming back as a second, hand-written
//   renderer; the local-data registration being lost, which would leave the
//   probe reachable from no pipe layer again; and a row field renamed without
//   its declared column, which silently drops a column from the table.

package cli

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"

	// The probe parses a config file, so the TACACS+ YANG module has to be
	// registered for `system { authentication { tacacs ... } }` to parse. The
	// shipped binaries get it through internal/component/plugin/all.
	_ "github.com/ze-software/ze/internal/component/tacacs/yang"
)

// writeProbeConfig writes a config naming one TACACS+ server at host:port and
// answers the config's path.
//
// One server per config, because the server list is keyed by address: two
// entries on 127.0.0.1 collide and the second is stored under a disambiguated
// key that is no longer a dialable address.
func writeProbeConfig(t *testing.T, name, address string) string {
	t.Helper()

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("split %q: %v", address, err)
	}
	configPath := filepath.Join(t.TempDir(), name)
	config := "system {\n\tauthentication {\n\t\ttacacs {\n" +
		"\t\t\tserver " + host + " { port " + port + "; key \"probe\"; }\n" +
		"\t\t\ttimeout 1;\n\t\t}\n\t}\n}\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

// listening answers the address of a listener this test owns, which a probe
// against it reports as reachable.
func listening(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})
	return listener.Addr().String()
}

// released answers the address of a listener that has been closed, which a
// probe against it reports as unreachable without waiting for a timeout.
func released(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for a released port: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	return address
}

// rowsOf decodes a rendered JSON answer into its rows. `| json` unwraps a
// single row-set envelope, so the answer arrives as an array.
func rowsOf(t *testing.T, rendered string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(rendered), &rows); err != nil {
		t.Fatalf("the answer is not a JSON row list: %v: %s", err, rendered)
	}
	return rows
}

func TestProbeAnswersStructuredRowsRatherThanText(t *testing.T) {
	reachable := listening(t)
	unreachable := released(t)

	for name, testCase := range map[string]struct {
		address string
		up      bool
	}{
		"listening server": {address: reachable, up: true},
		"released port":    {address: unreachable},
	} {
		t.Run(name, func(t *testing.T) {
			payload, code := dataServers([]string{writeProbeConfig(t, "probe.conf", testCase.address)})
			if code != exitOK {
				t.Fatalf("dataServers code = %d, want %d", code, exitOK)
			}
			answer, isMap := payload.(map[string]any)
			if !isMap {
				t.Fatalf("payload is %T, want a map", payload)
			}
			results, isRows := answer[keyServers].([]probeResult)
			if !isRows || len(results) != 1 {
				t.Fatalf("answer[%q] is %#v, want one row", keyServers, answer[keyServers])
			}
			row := results[0]
			if row.Address != testCase.address {
				t.Errorf("row address = %q, want %q", row.Address, testCase.address)
			}
			if row.Reachable != testCase.up {
				t.Errorf("row reachable = %t, want %t: %#v", row.Reachable, testCase.up, row)
			}
			if (row.Error == "") != testCase.up {
				t.Errorf("row error = %q, want one only when unreachable", row.Error)
			}
			if row.RTT == "" || row.Port == 0 {
				t.Errorf("row lost its rtt or port: %#v", row)
			}
		})
	}
}

// An unreachable server is an ANSWER, not a failure of the command: a non-zero
// code here would leave `| json` with nothing to render, because
// command.ServeLocal drops the payload of a failed command. The verdict stays
// available as the exit code of the `ze tacacs show` spelling.
func TestProbeAnswersEvenWhenNoServerIsReachable(t *testing.T) {
	configPath := writeProbeConfig(t, "all-down.conf", released(t))

	payload, code := dataServers([]string{configPath})
	if code != exitOK {
		t.Fatalf("dataServers code = %d, want %d", code, exitOK)
	}
	answer, _ := payload.(map[string]any)
	results, isRows := answer[keyServers].([]probeResult)
	if !isRows || len(results) != 1 {
		t.Fatalf("answer = %#v, want one row", payload)
	}
	if got := showExitCode(results); got != exitAllUnreach {
		t.Errorf("showExitCode = %d, want %d", got, exitAllUnreach)
	}
}

func TestOneAnswerRendersAsJSONAndAsYAML(t *testing.T) {
	address := listening(t)
	configPath := writeProbeConfig(t, "probe.conf", address)

	rendered, code, served := command.ServeLocal("show tacacs servers "+configPath+" | json compact", "")
	if !served {
		t.Fatal("show tacacs servers is not served in this process")
	}
	if code != 0 {
		t.Fatalf("json code = %d, want 0: %s", code, rendered)
	}
	rows := rowsOf(t, rendered)
	if len(rows) != 1 {
		t.Fatalf("json rows = %d, want 1: %s", len(rows), rendered)
	}
	if rows[0]["address"] != address || rows[0]["reachable"] != true {
		t.Errorf("json row = %#v, want the listening server reported reachable", rows[0])
	}

	yamlOut, code, served := command.ServeLocal("show tacacs servers "+configPath+" | yaml", "")
	if !served || code != 0 {
		t.Fatalf("yaml served=%t code=%d: %s", served, code, yamlOut)
	}
	// The yaml rendering keeps the envelope the payload declares and carries
	// the same server: one payload, two renderings.
	for _, want := range []string{keyServers + ":", address, "reachable: true"} {
		if !strings.Contains(yamlOut, want) {
			t.Errorf("yaml rendering lost %q: %s", want, yamlOut)
		}
	}
}

// A command declares the shape of its answer so an operator the shape cannot
// support is refused BY NAME rather than answered wrongly.
func TestAnUnsupportableOperatorIsRefusedByName(t *testing.T) {
	configPath := writeProbeConfig(t, "probe.conf", released(t))

	// `| resolve` acts only on a field a command declares to hold an IP
	// address, and this command declares none.
	rendered, code, served := command.ServeLocal("show tacacs servers "+configPath+" | resolve", "")
	if !served {
		t.Fatal("show tacacs servers is not served in this process")
	}
	if code == 0 {
		t.Fatalf("an unsupportable operator was accepted: %s", rendered)
	}
	if !command.IsPipeError(rendered) || !strings.Contains(rendered, "resolve") {
		t.Errorf("refusal does not name the operator: %s", rendered)
	}
}

// The command declares its rows against a column order, which is what the
// table and text renderers read. A rename in main.go that misses register.go
// drops a column, so the two are pinned to each other here.
func TestTheDeclaredColumnsAreTheRowFields(t *testing.T) {
	shape, declared := command.ShapeForCommand("show tacacs servers")
	if !declared || shape != command.ShapeTab {
		t.Fatalf("declared shape = %v (declared=%t), want ShapeTab", shape, declared)
	}

	orders := command.ColumnsForCommand("show tacacs servers")
	if len(orders) != 1 {
		t.Fatalf("column orders = %d, want 1", len(orders))
	}
	encoded, err := json.Marshal(probeResult{Address: "127.0.0.1:49", Port: 49, RTT: "1ms", Error: "refused"})
	if err != nil {
		t.Fatalf("encode a row: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode a row: %v", err)
	}
	for _, name := range orders[0] {
		if _, exists := fields[name]; !exists {
			t.Errorf("declared column %q is not a field of the row: %v", name, fields)
		}
	}
	if len(orders[0]) != len(fields) {
		t.Errorf("declared columns %v do not cover every row field %v", orders[0], fields)
	}
}
