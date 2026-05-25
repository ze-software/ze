package filter_remove_private_as

import (
	"strings"
	"testing"
)

// VALIDATES: remove-private-as definitions parse from bgp/policy config.
// PREVENTS: invalid replace-with values being accepted silently.
func TestParseRemovePrivateASDefs(t *testing.T) {
	tests := []struct {
		name      string
		bgpCfg    map[string]any
		wantMode  removeMode
		wantCount int
		wantErr   string
	}{
		{
			name: "default_strip",
			bgpCfg: map[string]any{"policy": map[string]any{"remove-private-as": map[string]any{
				"STRIP": map[string]any{},
			}}},
			wantMode:  removeModeStrip,
			wantCount: 1,
		},
		{
			name: "replace_peer_as",
			bgpCfg: map[string]any{"policy": map[string]any{"remove-private-as": map[string]any{
				"REPLACE": map[string]any{"replace-with": "peer-as"},
			}}},
			wantMode:  removeModePeerAS,
			wantCount: 1,
		},
		{
			name:      "no_policy",
			bgpCfg:    map[string]any{},
			wantCount: 0,
		},
		{
			name: "invalid_replace",
			bgpCfg: map[string]any{"policy": map[string]any{"remove-private-as": map[string]any{
				"BAD": map[string]any{"replace-with": "local-as"},
			}}},
			wantErr: "invalid replace-with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, err := parseRemovePrivateASDefs(tt.bgpCfg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(defs) != tt.wantCount {
				t.Fatalf("len(defs) = %d, want %d", len(defs), tt.wantCount)
			}
			for _, def := range defs {
				if def.mode != tt.wantMode {
					t.Fatalf("mode = %v, want %v", def.mode, tt.wantMode)
				}
			}
		})
	}
}
