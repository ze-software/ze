package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProbeConfigType verifies config type detection from content.
//
// VALIDATES: Detects bgp, hub, unknown from top-level blocks.
// PREVENTS: Wrong daemon started for config type.
func TestProbeConfigType(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    ConfigType
	}{
		{
			name:    "bgp_block",
			content: "bgp {\n\tpeer 127.0.0.1 { }\n}",
			want:    ConfigTypeBGP,
		},
		{
			name:    "bgp_with_environment",
			content: "environment { }\nbgp { peer 127.0.0.1 { } }",
			want:    ConfigTypeBGP,
		},
		{
			name:    "plugin_external",
			content: "plugin {\n\texternal bgp { run \"ze bgp\"; }\n}",
			want:    ConfigTypeHub,
		},
		{
			name:    "unknown_empty",
			content: "",
			want:    ConfigTypeUnknown,
		},
		{
			name:    "unknown_only_environment",
			content: "environment { log { level info; } }",
			want:    ConfigTypeUnknown,
		},
		{
			name:    "unknown_comments_only",
			content: "# just a comment\n# another comment",
			want:    ConfigTypeUnknown,
		},
		{
			name:    "bgp_precedence_over_plugin",
			content: "plugin { external x { } }\nbgp { peer 1.2.3.4 { } }",
			want:    ConfigTypeBGP,
		},
		{
			name:    "nested_bgp_ignored",
			content: "foo { bgp { } }",
			want:    ConfigTypeUnknown,
		},
		{
			name:    "nested_plugin_ignored",
			content: "foo { plugin { } }",
			want:    ConfigTypeUnknown,
		},
		{
			name:    "set_format_bgp",
			content: "set bgp router-id 127.0.0.1\nset bgp peer test remote as 1234",
			want:    ConfigTypeBGP,
		},
		{
			name:    "set_format_plugin",
			content: "set plugin hub listen 127.0.0.1:5555",
			want:    ConfigTypeHub,
		},
		{
			name:    "set_meta_format_bgp",
			content: "#thomas %2026-03-20T13:15:55Z set bgp router-id 127.0.0.123",
			want:    ConfigTypeBGP,
		},
		{
			name:    "set_meta_format_bgp_with_source",
			content: "#thomas @local %2026-03-20T12:51:20Z set bgp peer test local as 1234",
			want:    ConfigTypeBGP,
		},
		{
			name:    "set_format_unknown",
			content: "set environment log level info",
			want:    ConfigTypeUnknown,
		},
		{
			name:    "delete_format_bgp",
			content: "delete bgp peer test",
			want:    ConfigTypeBGP,
		},
		{
			name:    "delete_format_plugin",
			content: "delete plugin hub listen",
			want:    ConfigTypeHub,
		},
		{
			name:    "set_format_bgp_precedence",
			content: "set plugin hub listen 127.0.0.1:5555\nset bgp router-id 1.2.3.4",
			want:    ConfigTypeBGP,
		},
		{
			name:    "set_meta_with_previous",
			content: "#thomas @local %2026-03-20T12:00:00Z ^old set bgp router-id 1.2.3.4",
			want:    ConfigTypeBGP,
		},
		{
			name:    "bare_hash_line",
			content: "#\nset bgp router-id 1.2.3.4",
			want:    ConfigTypeUnknown, // bare # triggers FormatHierarchical in DetectFormat
		},
		{
			name:    "metadata_only_line",
			content: "#user\nset bgp router-id 1.2.3.4",
			want:    ConfigTypeBGP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProbeConfigType(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}
