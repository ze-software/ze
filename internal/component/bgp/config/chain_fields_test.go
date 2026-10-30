package bgpconfig

import (
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify"
	"github.com/ze-software/ze/internal/component/config"
)

// chainFieldsConfig renders a BGP peer whose import chain names filter, beside
// a policy root holding a modify that sets distance, a modify that adds a
// community, and a match instance.
func chainFieldsConfig(direction, filter string) string {
	return `
policy {
	modify SET-DISTANCE {
		set { distance 90; }
	}
	modify ADD-COMMUNITY {
		set { community-add [ 65000:1 ]; }
	}
	match TAG-1 {
		tag [ 1 ];
	}
}
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000 } }
	peer p1 {
		connection {
			remote { ip 10.0.0.1 }
			local { ip auto }
		}
		session { asn { remote 65001 } }
		filter {
			` + direction + ` { filters [ ` + filter + ` ]; final accept; }
		}
	}
}
`
}

// TestBGPChainFieldCheck is AC-10 on the BGP side: a BGP peer chain runs the
// route field check of its attachment, `bgp import` or `bgp export`. Method:
// load a peer whose chain names each instance. A modify that sets distance (a
// Loc-RIB field BGP routes do not let a filter set) is refused naming the
// attachment and the field, whether named plain or by its filter type; a match
// instance, which no BGP filter serves, is refused naming the type rather than
// passed to a plugin that does not exist; a modify that adds a community
// loads.
func TestBGPChainFieldCheck(t *testing.T) {
	cases := []struct {
		direction, filter string
		want              []string // empty: the config loads
	}{
		{"import", "SET-DISTANCE", []string{"peer p1 import filters", "bgp import", "SET-DISTANCE", "sets distance"}},
		{"export", "SET-DISTANCE", []string{"peer p1 export filters", "bgp export", "sets distance"}},
		{"import", "modify:SET-DISTANCE", []string{"bgp import", "sets distance"}},
		{"import", "TAG-1", []string{"peer p1 import filters", "TAG-1", "no BGP filter serves the match type"}},
		{"export", "match:TAG-1", []string{"no BGP filter serves the match type", "bgp export"}},
		{"import", "ADD-COMMUNITY", nil},
	}
	for _, tc := range cases {
		t.Run(tc.direction+" "+tc.filter, func(t *testing.T) {
			tree, err := config.ParseTreeWithYANG(chainFieldsConfig(tc.direction, tc.filter), nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = PeersFromConfigTree(tree)
			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("refused a chain BGP serves: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("loaded")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err %q does not name %q", err, w)
				}
			}
		})
	}
}
