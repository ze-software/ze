package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the observer peer-wait guard (plugin_fixture_peer_wait_test.go)
// refuses each way a registration could look as if it waits while it does not,
// and refuses a registration it cannot read rather than passing it.
// PREVENTS: the false passes three ze-test-judge probes found in earlier
// forms of the guard. The third: N1 a helper that writes the peer it is given
// through a pointer, N2 a builder that hands its scenario to a helper and
// discards the result, N3 a table field passed into a builder that ignores
// it, N4 an inline driver that discards what a helper hands back, N5 a poll
// that holds a peer's command and sends nothing. The second: P4 a helper that reassigns the peer it is given,
// P4b one that shadows it, P6 a builder that ignores the scenario it is given,
// P1 a table field the loop never passes, P2 a wait converted rather than
// handed on. The first, in the guard's first form, which followed every identifier naming a package function and paired a
// fixture with the values of any call or literal: M2b `_ = waitPeerEOR08` in
// place of the wait, U5 a package table `[]any{"plugin/x", waitPeerEOR08}` no
// registration reads, U7 `_ = p12L2TPWithdraw` in place of p12WaitPeersEOR, and
// U1-U3 registrations whose driver the scan cannot resolve.
// METHOD: write a small package and its .ci under a temporary directory, run
// observerPeerWaitFindings over it, and compare with a control that does wait.
func TestPluginObserverPeerWaitGuardRefusesFalsePasses(t *testing.T) {
	const helpers08 = `package fixture

func Register(name string, driver any) {}

func Observe(ctx any, name string, registration any, scenario func(ctx, p any) error) error {
	return scenario(ctx, nil)
}

func Poll(ctx any, attempts int, delay any, predicate func() bool) bool { return predicate() }

func command(ctx, p any, text string) bool { return true }

func waitPeerCounter08(ctx, p any, peer, counter string, minimum float64, attempts int) bool { return true }

func waitPeersEOR08(ctx, p any, peers ...string) error {
	for _, peer := range peers {
		waitPeerCounter08(ctx, p, peer, "eor-sent", 1, 40)
	}
	return nil
}

func waitPeerEOR08(ctx, p any) error { return waitPeersEOR08(ctx, p, "peer1") }

func registerPlugin08(name, pluginName string, scenario func(ctx, p any) error) {
	Register(name, func(ctx any, args []string) error { return scenario(ctx, nil) })
}
`
	const helpers12 = `package fixture

func p12WaitPeersEOR(ctx, plugin any, peers ...string) error { return nil }

func p12L2TPWithdraw(ctx, plugin any) error { return p12WaitPeersEOR(ctx, plugin, "peer1") }

func p12ObserveDriver(name string, scenario func(ctx, p any) error) any {
	return func(ctx any, _ []string) error { return Observe(ctx, name, nil, scenario) }
}
`
	cases := []struct {
		name    string
		files   map[string]string
		finding bool
		refused string
	}{
		{
			name: "control-08 waits",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error { return waitPeerEOR08(ctx, p) }
`},
		},
		{
			name: "M2b the wait named, not called",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error {
	_ = waitPeerEOR08
	return nil
}
`},
			finding: true,
		},
		{
			name: "U5 a table no registration ranges over",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
var table = []any{"plugin/x", waitPeerEOR08}
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "control-08 a count passed down through a builder",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", built(1)) }
func built(peers int) func(ctx, p any) error {
	return func(ctx, p any) error { return waitFor(ctx, p, "peer1") }
}
func waitFor(ctx, p any, peer string) error { return waitPeersEOR08(ctx, p, peer) }
`},
		},
		{
			name: "control an inline driver hands its scenario to Observe",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	Register("plugin/x", func(ctx any, args []string) error { return Observe(ctx, "x-test", nil, scenarioX) })
}
func scenarioX(ctx, p any) error { return waitPeerEOR08(ctx, p) }
`},
		},
		{
			name: "M2b inside an inline driver",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	Register("plugin/x", func(ctx any, args []string) error {
		_ = waitPeerEOR08
		return Observe(ctx, "x-test", nil, nil)
	})
}
`},
			finding: true,
		},
		{
			name: "control a poll on the peer's own state",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error {
	Poll(ctx, 40, nil, func() bool { return command(ctx, p, "show bgp peer peer1 detail") })
	return nil
}
`},
		},
		{
			name: "a poll on something other than the peer",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error {
	Poll(ctx, 40, nil, func() bool { return command(ctx, p, "show bfd sessions") })
	return nil
}
`},
			finding: true,
		},
		{
			name: "P4 a helper reassigns the peer it was given",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error { return waitFor(ctx, p, "peer1") }
func waitFor(ctx, p any, peer string) error {
	peer = "peer9"
	return waitPeersEOR08(ctx, p, peer)
}
`},
			finding: true,
		},
		{
			name: "P4b a helper shadows the peer it was given",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error { return waitFor(ctx, p, "peer1") }
func waitFor(ctx, p any, peer string) error {
	for _, peer := range []string{"peer9"} {
		waitPeersEOR08(ctx, p, peer)
	}
	return nil
}
`},
			finding: true,
		},
		{
			name: "P6 a builder ignores the scenario it is given",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", ignores(waitPeerEOR08)) }
func ignores(_ func(ctx, p any) error) func(ctx, p any) error { return noWait }
func noWait(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "control a builder runs the scenario it is given",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", runs(waitPeerEOR08)) }
func runs(scenario func(ctx, p any) error) func(ctx, p any) error {
	return func(ctx, p any) error { return scenario(ctx, p) }
}
`},
		},
		{
			name: "P1 a table field the loop never passes",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	for _, row := range []struct {
		name string
		wait func(ctx, p any) error
	}{{"plugin/x", waitPeerEOR08}} {
		registerPlugin08(row.name, "x-test", noWait)
	}
}
func noWait(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "control a table field the loop passes",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	for _, row := range []struct {
		name string
		wait func(ctx, p any) error
	}{{"plugin/x", waitPeerEOR08}} {
		registerPlugin08(row.name, "x-test", row.wait)
	}
}
`},
		},
		{
			name: "P2 an inline driver converts the wait instead of handing it on",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	Register("plugin/x", func(ctx any, args []string) error {
		_ = any(waitPeerEOR08)
		return Observe(ctx, "x-test", nil, noWait)
	})
}
func noWait(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "N1 a helper writes the peer it was given through a pointer",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error { return waitFor(ctx, p, "peer1") }
func waitFor(ctx, p any, peer string) error {
	alias := &peer
	*alias = "peer9"
	return waitPeersEOR08(ctx, p, peer)
}
`},
			finding: true,
		},
		{
			name: "N2 a builder hands the scenario to a function whose result it discards",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", built(waitPeerEOR08)) }
func keep(s func(ctx, p any) error) func(ctx, p any) error { return s }
func built(s func(ctx, p any) error) func(ctx, p any) error {
	keep(s)
	return noWait
}
func noWait(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "control a builder returns what a helper hands back",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", built(waitPeerEOR08)) }
func keep(s func(ctx, p any) error) func(ctx, p any) error { return s }
func built(s func(ctx, p any) error) func(ctx, p any) error { return keep(s) }
`},
		},
		{
			name: "N3 a table field passed into a builder that ignores it",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	for _, row := range []struct {
		name string
		wait func(ctx, p any) error
	}{{"plugin/x", waitPeerEOR08}} {
		registerPlugin08(row.name, "x-test", ignores(row.wait))
	}
}
func ignores(_ func(ctx, p any) error) func(ctx, p any) error { return noWait }
func noWait(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "N4 an inline driver discards what a helper hands back",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	Register("plugin/x", func(ctx any, args []string) error {
		_ = keep(waitPeerEOR08)
		return Observe(ctx, "x-test", nil, noWait)
	})
}
func keep(s func(ctx, p any) error) func(ctx, p any) error { return s }
func noWait(ctx, p any) error { return nil }
`},
			finding: true,
		},
		{
			name: "N5 a poll holds the peer's command and sends nothing",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() { registerPlugin08("plugin/x", "x-test", scenarioX) }
func scenarioX(ctx, p any) error {
	Poll(ctx, 1, nil, func() bool {
		_ = "show bgp peer peer1 detail"
		return true
	})
	return nil
}
`},
			finding: true,
		},
		{
			name: "control a map table whose value an inline driver hands to Observe",
			files: map[string]string{"plugin_fixture_08_case.go": `package fixture
func init() {
	observers := map[string]func(ctx, p any) error{"plugin/x": waitPeerEOR08}
	for name, scenario := range observers {
		n, s := name, scenario
		Register(n, func(ctx any, args []string) error { return Observe(ctx, n, nil, s) })
	}
}
`},
		},
		{
			name: "control-12 waits",
			files: map[string]string{"plugin_fixture_12_case.go": `package fixture
func init() { Register("plugin/x", p12ObserveDriver("x-test", scenarioX)) }
func scenarioX(ctx, p any) error { return p12L2TPWithdraw(ctx, p) }
`},
		},
		{
			name: "U7 the wait helper named, not called",
			files: map[string]string{"plugin_fixture_12_case.go": `package fixture
func init() { Register("plugin/x", p12ObserveDriver("x-test", scenarioX)) }
func scenarioX(ctx, p any) error {
	_ = p12L2TPWithdraw
	return nil
}
`},
			finding: true,
		},
		{
			name: "U1 a driver the package does not hold",
			files: map[string]string{"plugin_fixture_12_case.go": `package fixture
func init() { Register("plugin/x", noSuchDriver) }
`},
			refused: "which the scan did not find",
		},
		{
			name: "U2 a driver built by another package",
			files: map[string]string{"plugin_fixture_12_case.go": `package fixture
func init() { Register("plugin/x", other.Driver("x-test")) }
`},
			refused: "cannot resolve",
		},
		{
			name: "U3 a driver read out of a map",
			files: map[string]string{"plugin_fixture_12_case.go": `package fixture
var drivers = map[string]any{}
func init() { Register("plugin/x", drivers["plugin/x"]) }
`},
			refused: "cannot resolve",
		},
	}
	const ci = `stdin=peer:terminator=EOF_PEER
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
EOF_PEER
stdin=ze-conf:terminator=EOF_CONF
bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.1; port $PORT; } local { ip 127.0.0.1; } }
	}
}
EOF_CONF
cmd=background:seq=1:exec=le test peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze -:stdin=ze-conf
`
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			dir := t.TempDir()
			testRoot := t.TempDir()
			writeGuardFile(t, filepath.Join(testRoot, "plugin", "x.ci"), ci)
			writeGuardFile(t, filepath.Join(dir, "plugin_fixture_08.go"), helpers08)
			writeGuardFile(t, filepath.Join(dir, "plugin_fixture_12.go"), helpers12)
			for name, text := range current.files {
				writeGuardFile(t, filepath.Join(dir, name), text)
			}
			findings, err := observerPeerWaitFindings(dir, testRoot)
			if current.refused != "" {
				if err == nil || !strings.Contains(err.Error(), current.refused) {
					t.Fatalf("err = %v, want a refusal naming %q", err, current.refused)
				}
				return
			}
			if err != nil {
				t.Fatalf("observerPeerWaitFindings: %v", err)
			}
			if current.finding && len(findings) == 0 {
				t.Fatal("the guard accepted a registration that never waits for peer1")
			}
			if !current.finding && len(findings) != 0 {
				t.Fatalf("the guard refused a registration that waits: %v", findings)
			}
		})
	}
}

// writeGuardFile writes one file, creating its directory.
func writeGuardFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
