package iface

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Config parsing tests (Phase 1) ---

// TestPPPoEClientConfigParse verifies that a complete pppoe-client config
// with source-interface, authentication, MTU, and service-name is parsed
// into a pppoeClientEntry with all fields populated.
//
// VALIDATES: AC-1 - Config with pppoe-client, source-interface, and auth credentials.
func TestPPPoEClientConfigParse(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"pppoe-client": {
				"pppoe0": {
					"source-interface": "eth2",
					"authentication": {
						"username": "user@isp.net",
						"password": "secret123"
					},
					"service-name": "internet",
					"ac-name": "myisp",
					"mtu": "1480",
					"no-default-route": [null]
				}
			}
		}
	}`)
	require.Len(t, cfg.PPPoEClient, 1)
	e := cfg.PPPoEClient[0]
	assert.Equal(t, "pppoe0", e.Name)
	assert.Equal(t, "eth2", e.SourceInterface)
	assert.Equal(t, "user@isp.net", e.Username)
	assert.Equal(t, "secret123", e.AuthSecret)
	assert.Equal(t, "internet", e.ServiceName)
	assert.Equal(t, "myisp", e.ACName)
	assert.Equal(t, 1480, e.MTU)
	assert.True(t, e.NoDefaultRoute)
	assert.False(t, e.Disable)
}

// TestPPPoEClientConfigDefaults verifies that omitted optional fields get
// sensible defaults: MTU defaults to 1492 (PPPoE max), no-default-route
// defaults to false (install the route).
//
// VALIDATES: AC-7 - PPPoE interface created with default MTU 1492.
// VALIDATES: AC-8 - no-default-route absent means default route installed.
func TestPPPoEClientConfigDefaults(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"pppoe-client": {
				"pppoe0": {
					"source-interface": "eth0",
					"authentication": {
						"username": "user",
						"password": "pass"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.PPPoEClient, 1)
	e := cfg.PPPoEClient[0]
	assert.Equal(t, pppoeDefaultMTU, e.MTU)
	assert.False(t, e.NoDefaultRoute)
	assert.Empty(t, e.ServiceName)
	assert.Empty(t, e.ACName)
}

// TestPPPoEConfigValidation verifies that mandatory fields are enforced:
// missing source-interface, missing authentication block, missing username,
// missing password all produce clear errors.
//
// VALIDATES: AC-1 - source-interface must be present; credentials required.
func TestPPPoEConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name: "missing source-interface",
			json: `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"authentication": {
								"username": "user",
								"password": "pass"
							}
						}
					}
				}
			}`,
			wantErr: "source-interface is required",
		},
		{
			name: "missing authentication block",
			json: `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"source-interface": "eth0"
						}
					}
				}
			}`,
			wantErr: "authentication block is required",
		},
		{
			name: "missing username",
			json: `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"source-interface": "eth0",
							"authentication": {
								"password": "pass"
							}
						}
					}
				}
			}`,
			wantErr: "authentication username is required",
		},
		{
			name: "missing password",
			json: `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"source-interface": "eth0",
							"authentication": {
								"username": "user"
							}
						}
					}
				}
			}`,
			wantErr: "authentication password is required",
		},
		{
			name: "MTU too high for PPPoE",
			json: `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"source-interface": "eth0",
							"authentication": {
								"username": "user",
								"password": "pass"
							},
							"mtu": "1493"
						}
					}
				}
			}`,
			wantErr: "mtu 1493 out of range",
		},
		{
			name: "MTU too low",
			json: `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"source-interface": "eth0",
							"authentication": {
								"username": "user",
								"password": "pass"
							},
							"mtu": "67"
						}
					}
				}
			}`,
			wantErr: "mtu 67 out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseIfaceConfig(tt.json)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestPPPoEClientMTUBoundary validates the boundary values for PPPoE MTU.
//
// VALIDATES: Boundary tests - MTU range 68-1492.
func TestPPPoEClientMTUBoundary(t *testing.T) {
	tests := []struct {
		name    string
		mtu     string
		wantMTU int
		wantErr bool
	}{
		{"minimum valid", "68", 68, false},
		{"maximum valid (PPPoE max)", "1492", 1492, false},
		{"below minimum", "67", 0, true},
		{"above PPPoE max", "1493", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `{
				"interface": {
					"pppoe-client": {
						"pppoe0": {
							"source-interface": "eth0",
							"authentication": {
								"username": "user",
								"password": "pass"
							},
							"mtu": "` + tt.mtu + `"
						}
					}
				}
			}`
			cfg, err := parseIfaceConfig(input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, cfg.PPPoEClient, 1)
			assert.Equal(t, tt.wantMTU, cfg.PPPoEClient[0].MTU)
		})
	}
}

// TestPPPoEReconnectBackoff validates exponential backoff with cap (AC-10).
//
// VALIDATES: AC-10 - Automatic reconnection with exponential backoff.
func TestPPPoEReconnectBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, 60 * time.Second}, // capped at max
		{8, 60 * time.Second},
		{100, 60 * time.Second},
	}
	for _, tt := range tests {
		got := ReconnectDelay(tt.attempt)
		assert.Equal(t, tt.want, got, "attempt=%d", tt.attempt)
	}
}

// TestPPPoEClientLifecycle verifies Start/Stop with a fake dialer.
func TestPPPoEClientLifecycle(t *testing.T) {
	cfg := PPPoEClientConfig{
		Name:            "pppoe0",
		SourceInterface: "eth0",
		Username:        "user",
		AuthSecret:      "pass",
		MTU:             1492,
	}

	done := make(chan struct{})
	dialer := &fakeDialer{
		sess: PPPoESession{
			SessionID: 42,
			UnitNum:   0,
			NegMTU:    1492,
			LocalIP:   netip.MustParseAddr("10.0.0.42"),
			PeerIP:    netip.MustParseAddr("10.0.0.1"),
			Done:      done,
			Cleanup:   func() {},
		},
	}

	client := NewPPPoEClient(cfg, dialer, nil, slog.Default())
	client.Start()

	// Give the goroutine time to reach sessStateUp.
	time.Sleep(50 * time.Millisecond)

	status := client.status()
	assert.Equal(t, "up", status.State)
	assert.Equal(t, uint16(42), status.SessionID)
	assert.Equal(t, "pppoe0", status.PPPInterface)

	client.Stop()

	status = client.status()
	assert.Equal(t, "up", status.State) // state frozen at stop
}

type fakeDialer struct {
	sess PPPoESession
	err  error
}

func (d *fakeDialer) Dial(_ PPPoEClientConfig, stopCh <-chan struct{}, _ *slog.Logger) (PPPoESession, error) {
	if d.err != nil {
		return PPPoESession{}, d.err
	}
	return d.sess, nil
}

// TestPPPoEClientUsesConfiguredDeviceName verifies child-1 Phase 1 wiring:
// runSession names the device by the configured key (here pppoe0), not by
// the kernel-chosen pppN. The fake dialer returns a session with unit num 7
// so a regression that keeps textbuf.StrInt("ppp", 7) would reach the
// backend with "ppp7" and the assertions below fail. The configured name
// must appear on every backend call runSession makes after Dial returns.
// VALIDATES: AC-1, AC-8 with a fake dialer
// (plan/release/fixes/spec-home-cpe-1-logical-interface-name.md).
func TestPPPoEClientUsesConfiguredDeviceName(t *testing.T) {
	cfg := PPPoEClientConfig{
		Name:            "pppoe0",
		SourceInterface: "eth2",
		Username:        "user",
		AuthSecret:      "secret",
	}
	done := make(chan struct{})
	dialer := &fakeDialer{
		sess: PPPoESession{
			SessionID: 42,
			UnitNum:   7,
			NegMTU:    1492,
			LocalIP:   netip.MustParseAddr("192.0.2.2"),
			PeerIP:    netip.MustParseAddr("192.0.2.1"),
			Done:      done,
			Cleanup:   func() {},
		},
	}
	backend := &fakeBackend{ifaces: map[string]fakeIface{
		"pppoe0": {name: "pppoe0", linkType: "ppp", state: "down"},
	}}

	client := NewPPPoEClient(cfg, dialer, backend, slog.Default())
	client.Start()
	assert.Eventually(t, func() bool { return client.status().State == "up" },
		time.Second, 5*time.Millisecond, "client did not reach up")
	client.Stop()

	require.Len(t, backend.routeAdds, 1, "AddRoute was not called")
	assert.Equal(t, "pppoe0", backend.routeAdds[0].ifaceName,
		"AddRoute carried kernel name, not the configured key")
	assert.Equal(t, "up", backend.adminSet["pppoe0"],
		"SetAdminUp did not name pppoe0")
	_, hasPppN := backend.ifaces["ppp7"]
	assert.False(t, hasPppN, "a backend call named ppp7 instead of pppoe0")
	assert.Equal(t, 1492, backend.mtuSet["pppoe0"], "SetMTU did not name pppoe0")
}

// TestPPPoEClientNameValidated proves AC-6 (IFNAMSIZ limit) and AC-7
// (cross-kind collision refused): the parser refuses a pppoe-client key
// longer than 15 octets and a key equal to another configured interface's
// key, so the kernel never meets a name it will refuse at RTM_NEWLINK.
// VALIDATES: AC-6, AC-7.
func TestPPPoEClientNameValidated(t *testing.T) {
	t.Run("too-long", func(t *testing.T) {
		_, err := parseIfaceConfig(`{
			"interface": {
				"pppoe-client": {
					"pppoe-0123456789": {
						"source-interface": "eth0",
						"authentication": {"username": "u", "password": "p"}
					}
				}
			}
		}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "length")
		assert.Contains(t, err.Error(), "pppoe-0123456789")
	})
	t.Run("collides-with-ethernet", func(t *testing.T) {
		_, err := parseIfaceConfig(`{
			"interface": {
				"ethernet": {"pppoe0": {"mac": {"address": "00:11:22:33:44:55"}}},
				"pppoe-client": {
					"pppoe0": {
						"source-interface": "eth0",
						"authentication": {"username": "u", "password": "p"}
					}
				}
			}
		}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ethernet")
		assert.Contains(t, err.Error(), "pppoe-client")
		assert.Contains(t, err.Error(), "pppoe0")
	})
}

// TestPPPoEClientDisable verifies the disable flag is parsed.
func TestPPPoEClientDisable(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"pppoe-client": {
				"pppoe0": {
					"source-interface": "eth0",
					"authentication": {
						"username": "user",
						"password": "pass"
					},
					"disable": [null]
				}
			}
		}
	}`)
	require.Len(t, cfg.PPPoEClient, 1)
	assert.True(t, cfg.PPPoEClient[0].Disable)
}

// TestPPPoEClientHomeConf validates the ze equivalent of the VyOS home.conf
// PPPoE section: pppoe pppoe0 { authentication { password; username };
// mtu 1492; no-default-route; source-interface eth2 }.
func TestPPPoEClientHomeConf(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth2": {
					"mac": {"address": "60:be:b4:20:70:6e"}
				}
			},
			"pppoe-client": {
				"pppoe0": {
					"source-interface": "eth2",
					"authentication": {
						"username": "exa-211124-01@dsl.exa-networks.co.uk",
						"password": "secret123"
					},
					"mtu": "1492",
					"no-default-route": [null]
				}
			}
		}
	}`)

	require.Len(t, cfg.Ethernet, 1)
	assert.Equal(t, "eth2", cfg.Ethernet[0].Name)
	assert.Equal(t, "60:be:b4:20:70:6e", cfg.Ethernet[0].MACAddress)

	require.Len(t, cfg.PPPoEClient, 1)
	e := cfg.PPPoEClient[0]
	assert.Equal(t, "pppoe0", e.Name)
	assert.Equal(t, "eth2", e.SourceInterface)
	assert.Equal(t, "exa-211124-01@dsl.exa-networks.co.uk", e.Username)
	assert.Equal(t, "secret123", e.AuthSecret)
	assert.Equal(t, 1492, e.MTU)
	assert.True(t, e.NoDefaultRoute)
	assert.False(t, e.Disable)
	assert.Empty(t, e.ServiceName)
	assert.Empty(t, e.ACName)
}
