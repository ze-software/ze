package role

import (
	"slices"
	"testing"
)

// TestRolelessPeersNamesEveryUnboundExternalSession checks the set the
// roleless report names, over the shapes that decide it: a role on the peer
// or on its group binds it; an eBGP peer without one is named; a session that
// is internal by plain equality or by the RFC 7705 Section 4.2 migration ASN
// is left out; a dynamic group without a role is always named; a peer the
// engine drops (no remote AS) is not named. The local AS is inherited from
// the bgp block, as the engine resolves it. Method: one subtree, one
// exact sorted answer, so a wrong member or a missing one fails.
func TestRolelessPeersNamesEveryUnboundExternalSession(t *testing.T) {
	asn := func(remote string) map[string]any {
		return map[string]any{"asn": map[string]any{"remote": remote}}
	}
	remoteIP := func(ip string) map[string]any {
		return map[string]any{"remote": map[string]any{"ip": ip}}
	}
	subtree := map[string]any{
		"session": map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"ext-a":    map[string]any{"connection": remoteIP("192.0.2.1"), "session": asn("65001")},
			"ext-role": map[string]any{"connection": remoteIP("192.0.2.2"), "session": asn("65002"), "role": map[string]any{"import": "customer"}},
			"int-a":    map[string]any{"connection": remoteIP("192.0.2.3"), "session": asn("65000")},
			"migrate": map[string]any{"connection": remoteIP("192.0.2.4"), "session": map[string]any{
				"asn": map[string]any{"remote": "64999", "migration": "64999"},
			}},
			"no-remote-as": map[string]any{"connection": remoteIP("192.0.2.5")},
			"own-local": map[string]any{"connection": remoteIP("192.0.2.6"), "session": map[string]any{
				"asn": map[string]any{"remote": "65000", "local": "65100"},
			}},
		},
		"group": map[string]any{
			"bound": map[string]any{
				"role": map[string]any{"import": "provider"},
				"peer": map[string]any{
					"grp-bound": map[string]any{"connection": remoteIP("192.0.2.7"), "session": asn("65003")},
				},
			},
			"unbound": map[string]any{
				"session": asn("65004"),
				"peer": map[string]any{
					"grp-ext": map[string]any{"connection": remoteIP("192.0.2.8")},
				},
			},
			"ix": map[string]any{"connection": remoteIP("dynamic")},
		},
	}

	got, err := rolelessPeers(subtree)
	if err != nil {
		t.Fatalf("rolelessPeers: %v", err)
	}
	want := []string{"ext-a", "grp-ext", "ix", "own-local"}
	if !slices.Equal(got, want) {
		t.Fatalf("rolelessPeers = %v, want %v", got, want)
	}
}

// TestRolelessPeersRefusesAnUnreadableASN checks that an ASN the report
// cannot read is an error, never a guess about whether the session is
// internal.
func TestRolelessPeersRefusesAnUnreadableASN(t *testing.T) {
	subtree := map[string]any{"peer": map[string]any{
		"p": map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
			"session":    map[string]any{"asn": map[string]any{"remote": "not-an-asn"}},
		},
	}}
	if _, err := rolelessPeers(subtree); err == nil {
		t.Fatal("rolelessPeers accepted an unreadable remote ASN")
	}
}
