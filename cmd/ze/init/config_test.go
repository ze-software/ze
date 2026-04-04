package init

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// VALIDATES: generateInterfaceConfig produces correct Ze config syntax
// PREVENTS: malformed config from interface discovery breaking config parser

func TestGenerateInterfaceConfig(t *testing.T) {
	tests := []struct {
		name       string
		discovered []iface.DiscoveredInterface
		wantEmpty  bool
		contains   []string
		excludes   []string
	}{
		{
			name:       "empty input",
			discovered: nil,
			wantEmpty:  true,
		},
		{
			name: "single ethernet with MAC",
			discovered: []iface.DiscoveredInterface{
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
			},
			contains: []string{
				"interface {",
				"ethernet eth0 {",
				"mac-address aa:bb:cc:dd:ee:ff;",
				"os-name eth0;",
			},
		},
		{
			name: "single ethernet without MAC",
			discovered: []iface.DiscoveredInterface{
				{Name: "eth0", Type: "ethernet"},
			},
			contains: []string{
				"ethernet eth0 {",
				"os-name eth0;",
			},
			excludes: []string{
				"mac-address",
			},
		},
		{
			name: "loopback only",
			discovered: []iface.DiscoveredInterface{
				{Name: "lo", Type: "loopback"},
			},
			contains: []string{
				"interface {",
				"loopback {",
			},
			excludes: []string{
				"ethernet",
				"os-name",
			},
		},
		{
			name: "mixed types",
			discovered: []iface.DiscoveredInterface{
				{Name: "br0", Type: "bridge", MAC: "11:22:33:44:55:66"},
				{Name: "dummy0", Type: "dummy"},
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "lo", Type: "loopback"},
			},
			contains: []string{
				"bridge br0 {",
				"dummy dummy0 {",
				"ethernet eth0 {",
				"loopback {",
			},
		},
		{
			name: "invalid name with brace is skipped",
			discovered: []iface.DiscoveredInterface{
				{Name: "bad{name", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "eth0", Type: "ethernet", MAC: "11:22:33:44:55:66"},
			},
			contains: []string{
				"ethernet eth0 {",
			},
			excludes: []string{
				"bad{name",
			},
		},
		{
			name: "invalid name with semicolon is skipped",
			discovered: []iface.DiscoveredInterface{
				{Name: "bad;name", Type: "ethernet"},
			},
			contains: []string{
				"interface {",
			},
			excludes: []string{
				"bad;name",
				"ethernet",
			},
		},
		{
			name: "invalid name with newline is skipped",
			discovered: []iface.DiscoveredInterface{
				{Name: "bad\nname", Type: "ethernet"},
			},
			excludes: []string{
				"ethernet",
			},
		},
		{
			name: "invalid name with space is skipped",
			discovered: []iface.DiscoveredInterface{
				{Name: "bad name", Type: "ethernet"},
			},
			excludes: []string{
				"bad name",
			},
		},
		{
			name: "os-name populated in config",
			discovered: []iface.DiscoveredInterface{
				{Name: "enp3s0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
			},
			contains: []string{
				"os-name enp3s0;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateInterfaceConfig(tt.discovered)

			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty string, got %q", got)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, got)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("output should not contain %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

// VALIDATES: safeIfaceName rejects config-breaking characters
// PREVENTS: interface names with special characters breaking config syntax

func TestSafeIfaceName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid simple", "eth0", true},
		{"valid with dash", "enp3s0", true},
		{"valid with dot", "veth0.1", true},
		{"empty", "", false},
		{"contains brace open", "bad{", false},
		{"contains brace close", "bad}", false},
		{"contains semicolon", "bad;", false},
		{"contains newline", "bad\n", false},
		{"contains carriage return", "bad\r", false},
		{"contains tab", "bad\t", false},
		{"contains space", "bad name", false},
		{"contains null", "bad\x00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeIfaceName(tt.in)
			if got != tt.want {
				t.Errorf("safeIfaceName(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
