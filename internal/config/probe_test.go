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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProbeConfigType(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}
