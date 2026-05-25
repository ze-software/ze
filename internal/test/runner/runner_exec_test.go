package runner

import "testing"

func TestZeDaemonConfigArgIndex(t *testing.T) {
	// VALIDATES: runner only disables blob storage for ze daemon config
	// invocations, not for ze subcommands that consume config paths themselves.
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "stdin config", args: []string{"-"}, want: 0},
		{name: "plugin flags", args: []string{"--plugin", "ze.bgp-rib", "-"}, want: 2},
		{name: "web flags", args: []string{"--web", "3443", "--insecure-web", "test.conf"}, want: 3},
		{name: "mcp flags", args: []string{"--mcp", "8080", "--mcp-token", "secret", "ze.conf"}, want: 4},
		{name: "config subcommand", args: []string{"config", "validate", "-"}, want: -1},
		{name: "doctor subcommand", args: []string{"doctor", "--json", "empty.conf"}, want: -1},
		{name: "service subcommand", args: []string{"service", "install", "--dry-run"}, want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeDaemonConfigArgIndex(tt.args); got != tt.want {
				t.Fatalf("zeDaemonConfigArgIndex(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestZeDaemonShouldForceFileStorage(t *testing.T) {
	// VALIDATES: web functional tests keep blob storage enabled because the web
	// server requires it, while plain daemon tests still avoid shared zefs state.
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "plain config", args: []string{"test.conf"}, want: true},
		{name: "web config", args: []string{"--web", "3443", "--insecure-web", "test.conf"}, want: false},
		{name: "web equals", args: []string{"--web=3443", "test.conf"}, want: false},
		{name: "subcommand", args: []string{"config", "validate", "test.conf"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeDaemonShouldForceFileStorage(tt.args); got != tt.want {
				t.Fatalf("zeDaemonShouldForceFileStorage(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
