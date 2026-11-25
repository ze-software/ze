// Design: docs/research/l2tpv2-ze-integration.md -- L2TP data-plane feature predicate
// Related: register.go -- l2tpDataPlaneInUse predicate and init() registration

package l2tp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestL2TPDataPlaneInUse validates the standalone L2TP data-plane predicate.
// VALIDATES: AC-5, U-11 -- L2TP is in use iff an environment l2tp server entry
// exists and l2tp enabled is not false; an l2tp block holding only subscriber
// auth and pool (shared with PPPoE on VPP) is not in use.
// PREVENTS: refusing PPPoE-on-VPP configs that share the l2tp auth/pool block,
// or missing active L2TP listeners under interface backend vpp.
func TestL2TPDataPlaneInUse(t *testing.T) {
	t.Parallel()

	var registered *registry.DataPlaneFeature
	for _, feat := range registry.StandaloneDataPlaneFeatures() {
		if feat.Name == SubsystemName {
			f := feat
			registered = &f
			break
		}
	}
	if registered == nil {
		t.Fatal("standalone DataPlaneFeature for l2tp was not registered by init()")
	}
	if registered.InUseKind != registry.DataPlaneInUsePredicate || registered.InUse == nil {
		t.Fatalf("unexpected l2tp feature declaration: %+v", registered)
	}

	cases := []struct {
		name      string
		tree      map[string]any
		wantInUse bool
	}{
		{
			name:      "empty tree",
			tree:      map[string]any{},
			wantInUse: false,
		},
		{
			name: "l2tp auth and pool only without server",
			tree: map[string]any{
				"l2tp": map[string]any{
					"auth": map[string]any{"local": map[string]any{}},
					"pool": map[string]any{"ipv4": map[string]any{}},
				},
			},
			wantInUse: false,
		},
		{
			name: "environment l2tp server with default enabled",
			tree: map[string]any{
				"l2tp": map[string]any{},
				"environment": map[string]any{
					"l2tp": map[string]any{
						"server": map[string]any{
							"main": map[string]any{"ip": "0.0.0.0", "port": "1701"},
						},
					},
				},
			},
			wantInUse: true,
		},
		{
			name: "environment l2tp server with l2tp enabled true",
			tree: map[string]any{
				"l2tp": map[string]any{"enabled": "true"},
				"environment": map[string]any{
					"l2tp": map[string]any{
						"server": map[string]any{
							"main": map[string]any{"ip": "0.0.0.0"},
						},
					},
				},
			},
			wantInUse: true,
		},
		{
			name: "environment l2tp server with l2tp enabled false",
			tree: map[string]any{
				"l2tp": map[string]any{"enabled": "false"},
				"environment": map[string]any{
					"l2tp": map[string]any{
						"server": map[string]any{
							"main": map[string]any{"ip": "0.0.0.0"},
						},
					},
				},
			},
			wantInUse: false,
		},
		{
			name: "environment l2tp server without l2tp container",
			tree: map[string]any{
				"environment": map[string]any{
					"l2tp": map[string]any{
						"server": map[string]any{
							"main": map[string]any{"ip": "0.0.0.0"},
						},
					},
				},
			},
			wantInUse: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := registered.InUse(tc.tree)
			if got.InUse != tc.wantInUse {
				t.Fatalf("InUse = %v, want %v", got.InUse, tc.wantInUse)
			}
		})
	}
}
