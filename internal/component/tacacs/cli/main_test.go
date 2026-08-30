// Design: docs/architecture/aaa-tacacs.md -- ze tacacs show reachability probe
//
// VALIDATES: `ze tacacs show <config>` probes every TACACS+ server the config
//   names and reports each one's reachability as a row, with the verdict over
//   all of them as the exit code.
// PREVENTS: the deleted `--json` flag coming back as a second, hand-written
//   renderer, and a probe that reports a listening server unreachable or a
//   released port reachable.

package cli

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

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

func TestProbeReportsEachServerReachability(t *testing.T) {
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
			results, code := probeConfig([]string{writeProbeConfig(t, "probe.conf", testCase.address)})
			if code != exitOK {
				t.Fatalf("probeConfig code = %d, want %d", code, exitOK)
			}
			if len(results) != 1 {
				t.Fatalf("probeConfig answered %#v, want one row", results)
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

// An unreachable server is a ROW, not a failure of the probe: the rows are the
// answer, and the verdict over all of them is the exit code.
func TestProbeAnswersEvenWhenNoServerIsReachable(t *testing.T) {
	configPath := writeProbeConfig(t, "all-down.conf", released(t))

	results, code := probeConfig([]string{configPath})
	if code != exitOK {
		t.Fatalf("probeConfig code = %d, want %d", code, exitOK)
	}
	if len(results) != 1 {
		t.Fatalf("probeConfig answered %#v, want one row", results)
	}
	if got := showExitCode(results); got != exitAllUnreach {
		t.Errorf("showExitCode = %d, want %d", got, exitAllUnreach)
	}
}
