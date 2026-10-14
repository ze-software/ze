package bfd

import (
	"encoding/json"
	"net/netip"
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
