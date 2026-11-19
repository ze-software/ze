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
		got, err := testPPPoETiming.reconnectDelay(tt.attempt)
		require.NoError(t, err)
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
			SessionID:    42,
			UnitNum:      0,
			NegMTU:       1492,
			LocalIP:      netip.MustParseAddr("10.0.0.42"),
			PeerIP:       netip.MustParseAddr("10.0.0.1"),
			Done:         done,
			Cleanup:      func() error { return nil },
			NativeWriter: logicalNativeWriter{nil, "pppoe0"},
		},
	}

	client := NewPPPoEClient(cfg, dialer, slog.Default())
	client.Start()

	// Give the goroutine time to reach sessStateUp.
	time.Sleep(50 * time.Millisecond)

	status := client.status()
	assert.Equal(t, "up", status.State)
	assert.Equal(t, uint16(42), status.SessionID)
	assert.Equal(t, "pppoe0", status.PPPInterface)

	if err := client.Stop(); err != nil {
		t.Error(err)
	}

	status = client.status()
	assert.Equal(t, "up", status.State) // state frozen at stop
}

type fakeDialer struct {
	sess PPPoESession
	err  error
}

func (d *fakeDialer) Dial(_ PPPoEClientConfig, stopCh <-chan struct{}, _ *slog.Logger) (PPPoESession, error) { //nolint:gocritic // hugeParam: PPPoEDialer fixes the value signature; the fake preserves that dial boundary.
	if d.err != nil {
		return PPPoESession{}, d.err
	}
	return d.sess, nil
}

// TestPPPoEClientUsesReturnedNativeWriter verifies the actual iface consumer
// applies MTU, admin-up and route through the writer returned by its Dial.
// This logical writer records its supplied diagnostic label pppoe0; UnitNum 7
// cannot select another writer or cause the consumer to reconstruct ppp7.
// It does not prove kernel name creation. The actual configured-name creation
// and original/replacement identity proof is retained in
// TestNativeCPEConsumerKeepsOriginalAcrossNameReplacement.
func TestPPPoEClientUsesReturnedNativeWriter(t *testing.T) {
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
			Cleanup:   func() error { return nil },
		},
	}
	backend := &fakeBackend{ifaces: map[string]fakeIface{
		"pppoe0": {name: "pppoe0", linkType: "ppp", state: "down"},
	}}

	dialer.sess.NativeWriter = logicalNativeWriter{backend, "pppoe0"}
	client := NewPPPoEClient(cfg, dialer, slog.Default())
	client.Start()
	assert.Eventually(t, func() bool { return client.status().State == "up" },
		time.Second, 5*time.Millisecond, "client did not reach up")
	if err := client.Stop(); err != nil {
		t.Error(err)
	}

	require.Len(t, backend.routeAdds, 1, "AddRoute was not called")
	assert.Equal(t, "pppoe0", backend.routeAdds[0].ifaceName,
		"AddRoute did not use the returned writer's recorded label")
	assert.Equal(t, "up", backend.adminSet["pppoe0"],
		"SetAdminUp did not reach the supplied pppoe0 writer")
	_, hasPppN := backend.ifaces["ppp7"]
	assert.False(t, hasPppN, "UnitNum reconstructed a different writer target ppp7")
	assert.Equal(t, 1492, backend.mtuSet["pppoe0"], "SetMTU did not reach the supplied pppoe0 writer")
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
