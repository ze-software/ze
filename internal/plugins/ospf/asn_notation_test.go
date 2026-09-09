// Design: docs/architecture/ospf.md -- AS-number notation on the inter-AS TE link
package ospf

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	ospfyang "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// interASConfigText returns an OSPF config whose inter-AS remote-as is spelled
// as given.
func interASConfigText(remoteAS string) string {
	return `ospf {
    router-id 1.1.1.1
    opaque true
    areas {
        area 0.0.0.0 {
            area-type normal
        }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.0
            traffic-engineering {
                enable true
                inter-as {
                    remote-as ` + remoteAS + `
                    remote-asbr-ipv4 203.0.113.9
                }
            }
        }
    }
}`
}

// parseInterASFromText runs the whole path an operator's config takes. The
// text goes through the YANG-driven parser. The production plugin map comes
// out of the tree. That map goes through the plugin's own config reader.
func parseInterASFromText(t *testing.T, remoteAS string) ospfConfig {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(interASConfigText(remoteAS), map[string]string{
		"ospf": ospfyang.ZeOSPFConfYANG,
	})
	if err != nil {
		t.Fatalf("remote-as %s: parse tree: %v", remoteAS, err)
	}
	pluginMap := tree.ToPluginMap()
	data, err := json.Marshal(map[string]any{Namespace: pluginMap[Namespace]})
	if err != nil {
		t.Fatalf("remote-as %s: marshal plugin map: %v", remoteAS, err)
	}
	cfg, err := parseOSPFConfig([]configSection{{Root: Namespace, Data: string(data)}}, nil)
	if err != nil {
		t.Fatalf("remote-as %s: parseOSPFConfig: %v", remoteAS, err)
	}
	return cfg
}

// TestInterASRemoteASReadsEveryNotation proves the inter-AS TE Remote AS
// Number takes any of the three RFC 5396 spellings and reaches the plugin as
// one number.
//
// RFC 5392 section 3.3.1 makes the Remote AS Number sub-TLV four octets wide.
// This leaf therefore carries a four-byte AS number, and every spelling of one
// applies to it. The leaf is typed zt:asn and was never edited. It changed
// behavior because the config parser now normalizes that type through
// asn.Parse, and this test holds the leaf to it.
//
// VALIDATES: `remote-as 1.10` reaches parseInterAS as 65546 (AC-1).
// PREVENTS: a dotted remote-as parsing as no number at all. That leaves
// HasRemoteAS false, and the link then fails validation for a value the
// operator did write.
func TestInterASRemoteASReadsEveryNotation(t *testing.T) {
	for _, tc := range []struct {
		spelling string
		want     uint32
	}{
		{"65546", 65546},   // asplain
		{"1.10", 65546},    // asdot
		{"0.65001", 65001}, // asdot+, which asdot writes as plain 65001
	} {
		cfg := parseInterASFromText(t, tc.spelling)
		if len(cfg.Interfaces) != 1 {
			t.Fatalf("remote-as %s: %d interfaces, want 1", tc.spelling, len(cfg.Interfaces))
		}
		interAS := cfg.Interfaces[0].TE.InterAS
		if interAS == nil {
			t.Fatalf("remote-as %s: no inter-as block parsed", tc.spelling)
		}
		if !interAS.HasRemoteAS || interAS.RemoteAS != tc.want {
			t.Errorf("remote-as %s: HasRemoteAS=%v RemoteAS=%d, want true/%d",
				tc.spelling, interAS.HasRemoteAS, interAS.RemoteAS, tc.want)
		}
	}
}

// TestInterASRemoteASRefusesAMalformedASNumber proves the dotted spellings did
// not widen the leaf into accepting anything with a period in it. The refusal
// happens at the schema, so the plugin never sees the token.
//
// VALIDATES: zt:asn on remote-as reaches asn.Parse through ValidateValue.
// PREVENTS: an inter-AS link advertised with a Remote AS Number nobody typed.
func TestInterASRemoteASRefusesAMalformedASNumber(t *testing.T) {
	for _, value := range []string{"1.99999", "65536.0", "1.2.3", "neighbor"} {
		_, err := config.ParseTreeWithYANG(interASConfigText(value), map[string]string{
			"ospf": ospfyang.ZeOSPFConfYANG,
		})
		if err == nil {
			t.Errorf("remote-as %q was accepted, and it names no AS number", value)
		}
	}
}
