// VALIDATES: a `stdin=` block reaches the child by the route the .ci declares.
//            The `-` that is a ze daemon's config argument becomes a FILE; every
//            other `-` is the cliio stdin token and the block is PIPED.
// PREVENTS: the rewrite that took the FIRST `-` in argv whatever it meant, which
//            ran `ze bgp decode pcap <workdir>/ze-bgp.conf` for a .ci that wrote
//            `ze bgp decode pcap -` and piped nothing.

package runner

import "testing"

// TestCIStdinPipesForNonDaemonZeCommand checks that a `-` a ze VERB owns leaves
// argv untouched, so the block is piped. The method is one argv per Group 2 row
// of spec-fixit-ci-runner-cannot-test-stdin, plus the two forms the corpus
// actually carries a leading flag on.
func TestCIStdinPipesForNonDaemonZeCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "bgp decode hex", args: []string{"bgp", "decode", "-"}},
		{name: "bgp decode pcap", args: []string{"bgp", "decode", "pcap", "-"}},
		{name: "bgp decode with flags", args: []string{"bgp", "decode", "--json", "--family", "ipv4/unicast", "-"}},
		{name: "config validate", args: []string{"config", "validate", "-"}},
		{name: "config validate behind a global flag", args: []string{"--no-color", "config", "validate", "-"}},
		{name: "config fmt", args: []string{"config", "fmt", "-"}},
		{name: "config fmt write", args: []string{"config", "fmt", "-w", "-"}},
		{name: "config set", args: []string{"config", "set", "-", "bgp", "asn", "65000"}},
		{name: "config deactivate", args: []string{"config", "deactivate", "-", "bgp", "peer", "edge1"}},
		{name: "config activate", args: []string{"config", "activate", "-", "bgp", "peer", "edge1"}},
		{name: "config show", args: []string{"config", "show", "-"}},
		{name: "config dump", args: []string{"config", "dump", "-"}},
		{name: "config graph", args: []string{"config", "graph", "-"}},
		{name: "config fix", args: []string{"config", "fix", "-"}},
		{name: "config completion", args: []string{"config", "completion", "--context", "bgp", "--input", "set+", "-"}},
		{name: "config import", args: []string{"config", "import", "-"}},
		{name: "config diff", args: []string{"config", "diff", "a.conf", "-"}},
		{name: "config migrate", args: []string{"config", "migrate", "-"}},
		{name: "config edit", args: []string{"config", "edit", "-"}},
		{name: "config rollback", args: []string{"config", "rollback", "1", "-"}},
		{name: "config history", args: []string{"config", "history", "-"}},
		{name: "schema validate", args: []string{"schema", "validate", "-"}},
		{name: "doctor", args: []string{"doctor", "-"}},
		{name: "support", args: []string{"support", "bundle", "-"}},
		{name: "plugin test", args: []string{"plugin", "test", "-"}},
		{name: "data store", args: []string{"data", "banner", "-"}},
		{name: "tacacs show", args: []string{"tacacs", "show", "-"}},
		{name: "exabgp migrate", args: []string{"exabgp", "migrate", "-"}},
		{name: "appliance init cert", args: []string{"appliance", "init", "--cert", "-"}},
		{name: "appliance import", args: []string{"appliance", "import", "-"}},
		// A daemon launch naming a REAL config path still pipes: the block is
		// not the config, so nothing in argv is replaced. This is what the
		// first-dash scan already did, and it must not change.
		{name: "daemon with a real config path", args: []string{"start", "ze-bgp.conf"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, idx := routeStdinBlock(binNameZe, tt.args)
			if route != stdinRoutePipe {
				t.Fatalf("routeStdinBlock(ze, %v) = %d, want stdinRoutePipe (%d)", tt.args, route, stdinRoutePipe)
			}
			if idx != -1 {
				t.Fatalf("routeStdinBlock(ze, %v) index = %d, want -1", tt.args, idx)
			}
		})
	}
}

// TestCIStdinSubstitutesFileForDaemonLaunch checks that every daemon-launch
// spelling the corpus carries still asks for a config FILE, and names the argv
// index the `start <file>` pair replaces.
func TestCIStdinSubstitutesFileForDaemonLaunch(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "bare", args: []string{"-"}, want: 0},
		{name: "debug flag", args: []string{"-d", "-"}, want: 1},
		{name: "plugin", args: []string{"--plugin", "ze.bgp-rib", "-"}, want: 2},
		{name: "mcp", args: []string{"--mcp", "8080", "-"}, want: 2},
		{name: "pprof", args: []string{"--pprof", "6060", "-"}, want: 2},
		{name: "web", args: []string{"--web", "3443", "--insecure-web", "-"}, want: 3},
		{name: "two plugins", args: []string{"--plugin", "a", "--plugin", "b", "-"}, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, idx := routeStdinBlock(binNameZe, tt.args)
			if route != stdinRouteDaemonConfig {
				t.Fatalf("routeStdinBlock(ze, %v) = %d, want stdinRouteDaemonConfig (%d)", tt.args, route, stdinRouteDaemonConfig)
			}
			if idx != tt.want {
				t.Fatalf("routeStdinBlock(ze, %v) index = %d, want %d", tt.args, idx, tt.want)
			}
		})
	}
}

// TestCIStdinZePeerHonorsDeclaredMode checks both ze-peer routes. LoadExpectFile
// opens its path argument through cliio, so `-` reads standard input there
// (internal/test/peer/expect.go). A line that writes `-` gets the pipe; a line
// that writes no path keeps the appended temporary file, which is what all 702
// ze-peer stdin lines in the corpus rely on.
func TestCIStdinZePeerHonorsDeclaredMode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want stdinRoute
	}{
		{name: "no dash takes the file", args: []string{"peer", "--port", "1179"}, want: stdinRoutePeerFile},
		{name: "dash takes the pipe", args: []string{"peer", "--port", "1179", "-"}, want: stdinRoutePipe},
		{name: "dash before a flag takes the pipe", args: []string{"peer", "-", "--port", "1179"}, want: stdinRoutePipe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, idx := routeStdinBlock(binNameZePeer, tt.args)
			if route != tt.want {
				t.Fatalf("routeStdinBlock(ze-peer, %v) = %d, want %d", tt.args, route, tt.want)
			}
			if idx != -1 {
				t.Fatalf("routeStdinBlock(ze-peer, %v) index = %d, want -1", tt.args, idx)
			}
		})
	}
}

// TestCIStdinPipesForEveryOtherBinary checks that a binary neither branch names
// keeps piping. ze-test and the helper scripts have always piped, by accident of
// the two guards naming only ze and ze-peer, and the corpus depends on it.
func TestCIStdinPipesForEveryOtherBinary(t *testing.T) {
	for _, bin := range []string{binNameZeTest, "sh", "./script.sh"} {
		t.Run(bin, func(t *testing.T) {
			route, idx := routeStdinBlock(bin, []string{"engine-steps", "-"})
			if route != stdinRoutePipe {
				t.Fatalf("routeStdinBlock(%s, ...) = %d, want stdinRoutePipe (%d)", bin, route, stdinRoutePipe)
			}
			if idx != -1 {
				t.Fatalf("routeStdinBlock(%s, ...) index = %d, want -1", bin, idx)
			}
		})
	}
}
