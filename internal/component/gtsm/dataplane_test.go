// Design: ai/patterns/registration.md -- standalone data-plane feature declaration
// Related: register.go -- gtsmDataPlaneInUse predicate and init() registration

package gtsm

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestGTSMInUseFollowsTTLLeaves validates the GTSM data-plane in-use predicate.
// VALIDATES: AC-5 -- GTSM is in use iff a BGP peer, peer group, or peer inside
// a group sets a nonzero ttl max, ttl set, or ttl min leaf, and the finding
// names that leaf and its delete command as the second fix.
// PREVENTS: refusing BGP configurations under VPP when no GTSM ttl leaf is
// active (absent or zero), or missing a nonzero ttl leaf on a group or peer.
func TestGTSMInUseFollowsTTLLeaves(t *testing.T) {
	t.Parallel()

	// Confirm init() registered the standalone declaration under "gtsm".
	var registered *registry.DataPlaneFeature
	for _, feat := range registry.StandaloneDataPlaneFeatures() {
		if feat.Name == filterOwner {
			f := feat
			registered = &f
			break
		}
	}
	if registered == nil {
		t.Fatal("standalone DataPlaneFeature for gtsm was not registered by init()")
	}
	if registered.InUseKind != registry.DataPlaneInUsePredicate || registered.InUse == nil {
		t.Fatalf("unexpected gtsm feature declaration: %+v", registered)
	}

	cases := []struct {
		name           string
		tree           map[string]any
		wantInUse      bool
		wantConfigPath string
		wantSecondFix  string
	}{
		{
			name:      "no bgp section",
			tree:      map[string]any{},
			wantInUse: false,
		},
		{
			name: "peer with no ttl block",
			tree: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"r1": map[string]any{
							"connection": map[string]any{
								"remote": map[string]any{"ip": "192.0.2.1"},
							},
						},
					},
				},
			},
			wantInUse: false,
		},
		{
			name: "peer with zero ttl leaves",
			tree: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"r1": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{
									"max": "0",
									"set": "0",
									"min": "0",
								},
							},
						},
					},
				},
			},
			wantInUse: false,
		},
		{
			name: "group with zero ttl leaves",
			tree: map[string]any{
				"bgp": map[string]any{
					"group": map[string]any{
						"ebgp": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{
									"max": "0",
									"set": "",
									"min": "0",
								},
							},
						},
					},
				},
			},
			wantInUse: false,
		},
		{
			name: "standalone peer with nonzero ttl max",
			tree: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"r1": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{"max": "1"},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp peer r1 connection ttl max",
			wantSecondFix:  "delete bgp peer r1 connection ttl max",
		},
		{
			name: "standalone peer with nonzero ttl set",
			tree: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"r2": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{"set": "255"},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp peer r2 connection ttl set",
			wantSecondFix:  "delete bgp peer r2 connection ttl set",
		},
		{
			name: "standalone peer with nonzero ttl min",
			tree: map[string]any{
				"bgp": map[string]any{
					"peer": map[string]any{
						"r3": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{"min": "254"},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp peer r3 connection ttl min",
			wantSecondFix:  "delete bgp peer r3 connection ttl min",
		},
		{
			name: "group with nonzero ttl max",
			tree: map[string]any{
				"bgp": map[string]any{
					"group": map[string]any{
						"transit": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{"max": "2"},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp group transit connection ttl max",
			wantSecondFix:  "delete bgp group transit connection ttl max",
		},
		{
			name: "group with nonzero ttl set",
			tree: map[string]any{
				"bgp": map[string]any{
					"group": map[string]any{
						"transit": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{"set": "64"},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp group transit connection ttl set",
			wantSecondFix:  "delete bgp group transit connection ttl set",
		},
		{
			name: "group with nonzero ttl min",
			tree: map[string]any{
				"bgp": map[string]any{
					"group": map[string]any{
						"transit": map[string]any{
							"connection": map[string]any{
								"ttl": map[string]any{"min": "255"},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp group transit connection ttl min",
			wantSecondFix:  "delete bgp group transit connection ttl min",
		},
		{
			name: "peer inside group with nonzero ttl max",
			tree: map[string]any{
				"bgp": map[string]any{
					"group": map[string]any{
						"ix": map[string]any{
							"peer": map[string]any{
								"p1": map[string]any{
									"connection": map[string]any{
										"ttl": map[string]any{"max": "1"},
									},
								},
							},
						},
					},
				},
			},
			wantInUse:      true,
			wantConfigPath: "bgp group ix peer p1 connection ttl max",
			wantSecondFix:  "delete bgp group ix peer p1 connection ttl max",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := registered.InUse(tc.tree)
			if got.InUse != tc.wantInUse {
				t.Fatalf("InUse = %v, want %v (result: %+v)", got.InUse, tc.wantInUse, got)
			}
			if got.ConfigPath != tc.wantConfigPath {
				t.Fatalf("ConfigPath = %q, want %q", got.ConfigPath, tc.wantConfigPath)
			}
			if got.SecondFix != tc.wantSecondFix {
				t.Fatalf("SecondFix = %q, want %q", got.SecondFix, tc.wantSecondFix)
			}
		})
	}
}
