// Design: docs/guide/command-reference.md -- operator display of peer FQDN
// (draft-walton-bgp-hostname-capability, code 73).
//
// VALIDATES: show bgp, show bgp peer list, show bgp peer detail, and
//            show bgp peer capabilities display a peer's advertised hostname
//            and domain name when present and omit those keys when absent.
// PREVENTS:  received code 73 FQDN values being dropped before operator display,
//            or emitting empty hostname/domain/fqdn keys for peers that did not
//            advertise code 73.

package peer

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func fqdnReactor(hostname, domain string) *mockReactor {
	return &mockReactor{
		peers: []plugin.PeerInfo{{
			Address:             netip.MustParseAddr("192.0.2.1"),
			Name:                "peer1",
			PeerAS:              rpc.RemoteASOf(65001),
			LocalAS:             65000,
			State:               plugin.PeerStateEstablished,
			NegotiationComplete: true,
			NegotiatedFamilies:  []family.Family{family.IPv4Unicast},
			PeerHostname:        hostname,
			PeerDomainName:      domain,
		}},
		peerCaps: &plugin.PeerCapabilitiesInfo{
			Families:   []string{"ipv4/unicast"},
			ASN4:       true,
			Hostname:   hostname,
			DomainName: domain,
		},
		stats: plugin.ReactorStats{PeerCount: 1, RouterID: 0x0a000001, LocalAS: 65000},
	}
}

// RFC requirement: DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1 positive -- standard
// BGP show commands (summary, peer list, peer detail, peer capabilities) display
// the peer's advertised hostname and/or domain name for troubleshooting and omit
// empty or absent fields.
func TestPeerShowCommandsDisplayFQDN(t *testing.T) {
	tests := []struct {
		name           string
		hostname       string
		domain         string
		wantHostname   bool
		wantDomain     bool
		wantFQDNObject bool
	}{
		{
			name:           "both_hostname_and_domain",
			hostname:       "spine-01",
			domain:         "dc1.example",
			wantHostname:   true,
			wantDomain:     true,
			wantFQDNObject: true,
		},
		{
			name:           "hostname_only",
			hostname:       "spine-01",
			domain:         "",
			wantHostname:   true,
			wantDomain:     false,
			wantFQDNObject: true,
		},
		{
			name:           "domain_only",
			hostname:       "",
			domain:         "dc1.example",
			wantHostname:   false,
			wantDomain:     true,
			wantFQDNObject: true,
		},
		{
			name:           "absent",
			hostname:       "",
			domain:         "",
			wantHostname:   false,
			wantDomain:     false,
			wantFQDNObject: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := fqdnReactor(tt.hostname, tt.domain)
			ctx := newTestContext(reactor)
			ctx.Peer = "192.0.2.1"

			// 1. show bgp (summary)
			sumResp, err := handleBgpSummary(ctx, commandtest.Args())
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, sumResp.Status)
			sumRow := arrayRowOf("peers")(t, sumResp)
			assertRowFQDN(t, "show bgp", sumRow, tt.hostname, tt.domain, tt.wantHostname, tt.wantDomain)

			// 2. show bgp peer list
			listResp, err := handleBgpPeerList(ctx, commandtest.Args())
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, listResp.Status)
			listRow := keyedPeerRow(t, listResp)
			assertRowFQDN(t, "show bgp peer list", listRow, tt.hostname, tt.domain, tt.wantHostname, tt.wantDomain)

			// 3. show bgp peer detail
			detailResp, err := handleBgpPeerDetail(ctx, commandtest.Args())
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, detailResp.Status)
			detailRow := keyedPeerRow(t, detailResp)
			assertRowFQDN(t, "show bgp peer detail", detailRow, tt.hostname, tt.domain, tt.wantHostname, tt.wantDomain)
			detailCaps, ok := detailRow["capabilities"].(map[string]any)
			require.True(t, ok)
			assertCapFQDN(t, "show bgp peer detail capabilities", detailCaps, tt.hostname, tt.domain, tt.wantHostname, tt.wantDomain, tt.wantFQDNObject)

			// 4. show bgp peer capabilities
			capsResp, err := handleBgpPeerCapabilities(ctx, commandtest.Args())
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, capsResp.Status)
			capsRaw, err := json.Marshal(capsResp.Data)
			require.NoError(t, err)
			var capsDoc map[string]any
			require.NoError(t, json.Unmarshal(capsRaw, &capsDoc))
			capsRow := capsDoc["peers"].([]any)[0].(map[string]any)
			neg, ok := capsRow["negotiated"].(map[string]any)
			require.True(t, ok)
			assertCapFQDN(t, "show bgp peer capabilities negotiated", neg, tt.hostname, tt.domain, tt.wantHostname, tt.wantDomain, tt.wantFQDNObject)
		})
	}
}

func assertRowFQDN(t *testing.T, surface string, row map[string]any, host, domain string, wantHost, wantDomain bool) {
	t.Helper()
	gotHost, hasHost := row["hostname"]
	assert.Equal(t, wantHost, hasHost, "%s hostname presence", surface)
	if wantHost {
		assert.Equal(t, host, gotHost, "%s hostname value", surface)
	}
	gotDomain, hasDomain := row["domain"]
	assert.Equal(t, wantDomain, hasDomain, "%s domain presence", surface)
	if wantDomain {
		assert.Equal(t, domain, gotDomain, "%s domain value", surface)
	}
}

func assertCapFQDN(t *testing.T, surface string, capMap map[string]any, host, domain string, wantHost, wantDomain, wantObj bool) {
	t.Helper()
	rawFQDN, hasFQDN := capMap["fqdn"]
	assert.Equal(t, wantObj, hasFQDN, "%s fqdn presence", surface)
	if !wantObj {
		return
	}
	fqdn, ok := rawFQDN.(map[string]any)
	require.True(t, ok, "%s fqdn is not an object: %#v", surface, rawFQDN)
	assertRowFQDN(t, surface+".fqdn", fqdn, host, domain, wantHost, wantDomain)
}
