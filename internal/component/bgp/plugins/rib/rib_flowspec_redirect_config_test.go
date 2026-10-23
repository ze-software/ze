// Design: docs/architecture/plugin/rib-storage-design.md -- per-session redirect-check inheritance
// Related: rib_flowspec_redirect.go -- the config parser and runtime session lookup

package rib

import (
	"net/netip"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestFlowSpecRedirectDynamicGroupLookup delivers real plugin JSON to the RIB
// parser, then asks the session reader rather than inspecting its stored keys.
// Opposite bgp/group values in both directions distinguish a lost template from
// a deliberate false, and an address-shaped group name exercises both namespaces.
func TestFlowSpecRedirectDynamicGroupLookup(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    string
		group   string
		checked bool
	}{
		{name: "group disables", base: "true", group: "false", checked: false},
		{name: "group enables", base: "false", group: "true", checked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, present, err := parseRIBConfig([]sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{
				"flowspec-redirect-check":{"enabled":` + tc.base + `},
				"peer":{"192.0.2.1":{"flowspec-redirect-check":{"enabled":` + tc.base + `}}},
				"group":{"192.0.2.1":{
					"connection":{"remote":{"ip":"dynamic","range":["198.51.100.0/24"]}},
					"flowspec-redirect-check":{"enabled":` + tc.group + `},
					"peer":{
						"override":{"connection":{"remote":{"ip":"198.51.100.2"}},
							"flowspec-redirect-check":{"enabled":` + tc.base + `}},
						"inherited":{"connection":{"remote":{"ip":"198.51.100.3"}}}
					}
				}}
			}}`}})
			if err != nil {
				t.Fatal(err)
			}
			if !present {
				t.Fatal("the BGP delivery was ignored")
			}

			r := &RIBManager{peerMeta: make(map[netip.Addr]*peerMetadata)}
			r.flowSpecRedirectCfg.Store(&cfg.redirect)
			for _, session := range []struct {
				name    string
				address string
				group   string
				want    bool
			}{
				{name: "dynamic member", address: "198.51.100.7", group: "192.0.2.1", want: tc.checked},
				{name: "named override", address: "198.51.100.2", group: "192.0.2.1", want: !tc.checked},
				{name: "named inheritance", address: "198.51.100.3", group: "192.0.2.1", want: tc.checked},
				{name: "peer sharing group identity", address: "192.0.2.1", want: !tc.checked},
				{name: "different group", address: "198.51.100.8", group: "other", want: !tc.checked},
				{name: "no group", address: "198.51.100.9", want: !tc.checked},
			} {
				t.Run(session.name, func(t *testing.T) {
					addr := netip.MustParseAddr(session.address)
					r.peerMu.Lock()
					r.peerMeta[addr] = &peerMetadata{GroupName: session.group}
					r.peerMu.Unlock()
					r.peerMu.RLock()
					got := r.flowSpecRedirectChecked(addr)
					r.peerMu.RUnlock()
					if got != session.want {
						t.Errorf("session %s in group %q: redirect check = %v, want %v", addr, session.group, got, session.want)
					}
				})
			}
		})
	}
}

// TestFlowSpecRedirectDynamicGroupRejectsInvalidSwitch proves that an invalid
// template is visited and refused through the same parser as a plugin delivery,
// even when no static peer exists to cause that group to be read.
func TestFlowSpecRedirectDynamicGroupRejectsInvalidSwitch(t *testing.T) {
	_, _, err := parseRIBConfig([]sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"group":{"ix":{
		"connection":{"remote":{"ip":"dynamic","range":["198.51.100.0/24"]}},
		"flowspec-redirect-check":{"enabled":"maybe"}
	}}}}`}})
	if err == nil {
		t.Fatal("an invalid redirect check on a peerless dynamic group was accepted")
	}
}
