package reactor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestPeerPathsLimitOutputAbsence defends the operator distinction between an
// enforced maximum and no limit. Real negotiation is projected through the peer
// fact producer, both command handlers, and the standard text/JSON renderers.
// The positive, different-valued directions are exercised over live SSH by
// fixture plugin/paths-limit-live before its independent receiver exits.
func TestPeerPathsLimitOutputAbsence(t *testing.T) {
	for _, state := range []string{"absent", "zero", "disconnected"} {
		t.Run(state, func(t *testing.T) {
			caps := []capability.Capability{capIPv4(), capAddPathIPv4Both()}
			if state != "absent" {
				var limit uint16
				if state == "disconnected" {
					limit = 3
				}
				caps = append(caps, &capability.PathsLimit{Entries: []capability.PathsLimitEntry{{
					AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Limit: limit,
				}}})
			}
			p := NewPeer(negotiationSettings(caps...))
			p.negotiated.Store(NewNegotiatedCapabilities(capability.Negotiate(caps, caps, 65001, 65002)))
			if state == "disconnected" {
				// The peer lifecycle clears this published snapshot on teardown.
				p.negotiated.Store(nil)
			}
			r := newTestReactor(t)
			r.peers[p.Settings().PeerKey()] = p
			srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, &reactorAPIAdapter{r: r})
			require.NoError(t, err)
			ctx := &pluginserver.CommandContext{Server: srv}

			for _, surface := range []string{"capabilities", "detail"} {
				t.Run(surface, func(t *testing.T) {
					var handler pluginserver.Handler
					for _, reg := range pluginserver.AllBuiltinRPCs() {
						if reg.WireMethod == "ze-bgp:peer-"+surface {
							handler = reg.Handler
							break
						}
					}
					require.NotNil(t, handler)
					response, err := handler(ctx, []string{p.Settings().Address.String()})
					require.NoError(t, err)
					require.Equal(t, plugin.StatusDone, response.Status)
					payload, err := json.Marshal(response.Data)
					require.NoError(t, err)

					for _, format := range []string{"json", "text"} {
						_, render, refusal := command.ProcessPipesDefaultFormatChecked("show bgp peer "+surface+" | "+format, "")
						require.Empty(t, refusal)
						answer := render(string(payload))
						assert.NotContains(t, answer, "paths-limit", "%s must not look enforced in %s", state, format)
						if format == "json" {
							var row map[string]any
							if surface == "capabilities" {
								var peers []map[string]any
								require.NoError(t, json.Unmarshal([]byte(answer), &peers))
								require.Len(t, peers, 1)
								row = peers[0]
							} else {
								var decoded struct {
									Peers json.RawMessage `json:"peers"`
								}
								require.NoError(t, json.Unmarshal([]byte(answer), &decoded))
								var peers map[string]struct {
									Capabilities map[string]any `json:"capabilities"`
								}
								require.NoError(t, json.Unmarshal(decoded.Peers, &peers))
								row = peers[p.Settings().Address.String()].Capabilities
							}
							assert.Equal(t, state != "disconnected", row["negotiation-complete"])
						}
					}
				})
			}
		})
	}
}
