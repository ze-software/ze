//go:build ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStartAcceptsConfigPath verifies that `ze start` extracts the leading
// positional config path (keyword-first grammar, ai/rules/cli-grammar.md R1)
// while never mistaking a flag's value for the path.
//
// VALIDATES: startConfigPath returns the first non-flag positional and skips
// the value of --web/--mcp/--mcp-token.
// PREVENTS: A `ze start --web <port>` launch treating the port as a config path,
// or a config-name-collision file being dropped.
func TestStartAcceptsConfigPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"leading path", []string{"router.conf"}, "router.conf"},
		{"absolute path", []string{"/etc/ze/router.conf"}, "/etc/ze/router.conf"},
		{"command-name file", []string{"signal"}, "signal"},
		{"web port not mistaken for path", []string{"--web", "3443", "router.conf"}, "router.conf"},
		{"web port only, no path", []string{"--web", "3443"}, ""},
		{"cli flag then path", []string{"--cli", "router.conf"}, "router.conf"},
		{"mcp flags then path", []string{"--mcp", "8080", "--mcp-token", "secret", "x.conf"}, "x.conf"},
		{"insecure-web then path", []string{"--insecure-web", "--web", "3443", "x.conf"}, "x.conf"},
		{"web-only no path", []string{"--web-only", "--web", "3443"}, ""},
		{"no args", nil, ""},
		{"unknown flag skipped", []string{"--unknown", "keep.conf"}, "keep.conf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, startConfigPath(tt.args))
		})
	}
}
