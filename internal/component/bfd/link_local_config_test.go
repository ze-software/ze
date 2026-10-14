package bfd

import (
	"encoding/json"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseBFDConfigText runs config text through the config file parser over
// the YANG schema, then hands the tree's JSON to the plugin's own parser, as
// a commit does. It returns the parser error or the plugin error.
func parseBFDConfigText(t *testing.T, text string) (*pluginConfig, error) {
	t.Helper()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	tree, err := config.NewParser(schema).Parse(text)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(tree.ToMap())
	if err != nil {
		t.Fatalf("marshal tree: %v", err)
	}
	return parseSections([]sdk.ConfigSection{{Root: configRoot, Data: string(data)}})
}

// TestTwoLinkLocalSingleHopSessionsParseApart proves one link-local address
// configured on two links reaches the plugin as two sessions, each with its
// own interface and its own leaves. Method: two `single-hop-session fe80::b`
// entries differing in `interface` go through the config file parser and
// parseSections. Before the canonical full-key address the parser refused
// the second entry as a duplicate list key.
func TestTwoLinkLocalSingleHopSessionsParseApart(t *testing.T) {
	cfg, err := parseBFDConfigText(t, `bfd {
	enabled true;
	bind-v6 true;
	profile fast { desired-min-tx-us 100000; }
	single-hop-session fe80::b { interface zbl0; local fe80::a; }
	single-hop-session fe80::b { interface zbl1; local fe80::a; profile fast; }
}
`)
	if err != nil {
		t.Fatalf("two entries for one peer on two interfaces refused: %v", err)
	}
	byIface := make(map[string]sessionConfig)
	for _, s := range cfg.sessions {
		byIface[s.iface] = s
	}
	if len(cfg.sessions) != 2 {
		t.Fatalf("sessions: got %d, want 2: %+v", len(cfg.sessions), cfg.sessions)
	}
	peer := netip.MustParseAddr("fe80::b")
	for _, iface := range []string{"zbl0", "zbl1"} {
		s, ok := byIface[iface]
		if !ok {
			t.Fatalf("no session on %s: %+v", iface, cfg.sessions)
		}
		if s.peer != peer {
			t.Errorf("%s: peer %s, want %s", iface, s.peer, peer)
		}
	}
	if byIface["zbl0"].profile != "" {
		t.Errorf("zbl0 took profile %q, which only the zbl1 entry names", byIface["zbl0"].profile)
	}
	if byIface["zbl1"].profile != "fast" {
		t.Errorf("zbl1 profile %q, want fast", byIface["zbl1"].profile)
	}
}

// TestTwoEntriesForOneSessionKeyRefused proves two entries that resolve to
// one session are refused by the plugin, naming each entry's peer as written
// and the interface. Method: `fe80::b` and `fe80:0::b` are two distinct
// config keys (peer is a YANG string), which netip.ParseAddr reads as one
// address on one interface.
func TestTwoEntriesForOneSessionKeyRefused(t *testing.T) {
	_, err := parseBFDConfigText(t, `bfd {
	enabled true;
	bind-v6 true;
	single-hop-session fe80::b { interface zbl0; }
	single-hop-session fe80:0::b { interface zbl0; }
}
`)
	if err == nil {
		t.Fatal("two spellings of one peer on one interface accepted")
	}
	for _, want := range []string{"fe80::b", "fe80:0::b", "interface zbl0", "one session"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestSetFormReplayKeepsTwoLinkLocalSessions proves a set-form save of two
// entries for one link-local peer on two links replays, through both
// SetParser paths, into two sessions the plugin keys on their interfaces.
// Method: brace form through the file parser, set form out, set form back
// in with Parse and with ParseWithMeta, then the tree's JSON to parseSections.
func TestSetFormReplayKeepsTwoLinkLocalSessions(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	tree, err := config.NewParser(schema).Parse(`bfd {
	enabled true;
	bind-v6 true;
	single-hop-session fe80::b { interface zbl0; }
	single-hop-session fe80::b { interface zbl1; }
}
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	setForm := config.SerializeSet(tree, schema)
	replays := map[string]func() (*config.Tree, error){
		"Parse": func() (*config.Tree, error) { return config.NewSetParser(schema).Parse(setForm) },
		"ParseWithMeta": func() (*config.Tree, error) {
			replayed, _, perr := config.NewSetParser(schema).ParseWithMeta(setForm)
			return replayed, perr
		},
	}
	for name, replay := range replays {
		t.Run(name, func(t *testing.T) {
			replayed, err := replay()
			if err != nil {
				t.Fatalf("replay: %v\n%s", err, setForm)
			}
			data, err := json.Marshal(replayed.ToMap())
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: string(data)}})
			if err != nil {
				t.Fatalf("plugin parse: %v", err)
			}
			ifaces := make([]string, 0, len(cfg.sessions))
			for _, s := range cfg.sessions {
				ifaces = append(ifaces, s.iface)
			}
			slices.Sort(ifaces)
			if !slices.Equal(ifaces, []string{"zbl0", "zbl1"}) {
				t.Errorf("sessions on %q, want zbl0 and zbl1\n%s", ifaces, setForm)
			}
		})
	}
}

// TestSingleHopLinkLocalPeerNeedsInterface is AC-5, AC-6 and AC-6a of
// spec-bfd-link-local-peer-zone.
//
// GOAL: a link-local single-hop peer is configured only together with the
// link it is on. METHOD: config text through the config file parser and the
// plugin's parseSections, as a commit, a reload and a start all do.
//
// VALIDATES: a `fe80::/10` peer with no `interface` is refused in the default
// VRF and in `vrf blue`, the error naming the peer and `interface`; the same
// peer with `interface`, a global IPv6 peer and an IPv4 peer (link-local
// included) without one are accepted.
// PREVENTS: a link-local session that names no link loading and then sending
// on whichever link the kernel's route lookup picks (RFC 5881 Section 3).
func TestSingleHopLinkLocalPeerNeedsInterface(t *testing.T) {
	cases := []struct {
		name    string
		session string
		refused bool
	}{
		{"link-local without interface", `single-hop-session fe80::b { local fe80::a; }`, true},
		{"link-local without interface in vrf blue", `single-hop-session fe80::b { vrf blue; }`, true},
		{"link-local with interface", `single-hop-session fe80::b { interface zbl0; }`, false},
		{"link-local with interface in vrf blue", `single-hop-session fe80::b { interface zbl0; vrf blue; }`, false},
		{"global IPv6 without interface", `single-hop-session 2001:db8::b { }`, false},
		{"IPv4 without interface", `single-hop-session 192.0.2.9 { }`, false},
		{"IPv4 link-local without interface", `single-hop-session 169.254.0.9 { }`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseBFDConfigText(t, "bfd {\n\tenabled true;\n\tbind-v6 true;\n\t"+tc.session+"\n}\n")
			if !tc.refused {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted a link-local peer that names no interface")
			}
			want := "bfd: single-hop-session fe80::b: a link-local peer is on one link only: add interface naming the link it is on"
			if err.Error() != want {
				t.Errorf("error %q, want %q", err, want)
			}
		})
	}
}

// TestSingleHopPeerRefusesAZone is AC-7 of spec-bfd-link-local-peer-zone.
//
// GOAL: the link of a session is written once, in `interface`. METHOD:
// `peer` (the entry's first key value) and `local` each written with a zone,
// through the config file parser and parseSections.
//
// VALIDATES: each is refused, the error naming the leaf and its value and
// saying to write the address without the zone and the link in `interface`.
// PREVENTS: a second spelling of the link that can disagree with `interface`.
func TestSingleHopPeerRefusesAZone(t *testing.T) {
	cases := []struct {
		name    string
		session string
		want    string
	}{
		{
			"peer", `single-hop-session fe80::b%zbl0 { interface zbl0; }`,
			`bfd: single-hop-session fe80::b%zbl0: peer "fe80::b%zbl0" carries a zone: write the address as fe80::b and the link in interface`,
		},
		{
			"local", `single-hop-session fe80::b { interface zbl0; local fe80::a%zbl0; }`,
			`bfd: single-hop-session fe80::b: local "fe80::a%zbl0" carries a zone: write the address as fe80::a and the link in interface`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseBFDConfigText(t, "bfd {\n\tenabled true;\n\tbind-v6 true;\n\t"+tc.session+"\n}\n")
			if err == nil {
				t.Fatalf("accepted a zoned %s", tc.name)
			}
			if err.Error() != tc.want {
				t.Errorf("error %q, want %q", err, tc.want)
			}
		})
	}
}
