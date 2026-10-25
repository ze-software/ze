package bgpconfig

import (
	"bytes"
	"maps"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// pluginRouteFromConfig parses one peer whose update block holds attrs and a
// single nlri line, and returns the extracted plugin routes or the load error.
func pluginRouteFromConfig(t *testing.T, attrs, nlriLine string) ([]PluginRouteConfig, error) {
	t.Helper()
	input := `
bgp {
    session { asn { local 65000 } }
    peer mypeer {
        connection { remote { ip 192.168.1.1 } }
        session { asn { remote 65001 } }
        update {
            attribute {
                next-hop 10.0.0.1;
` + attrs + `
            }
            nlri {
                ` + nlriLine + `;
            }
        }
    }
}
`
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	peer := tree.GetContainer("bgp").GetList("peer")["mypeer"]
	if peer == nil {
		t.Fatal("peer mypeer not found")
	}
	routes, err := extractRoutesFromTree(peer)
	if err != nil {
		return nil, err
	}
	return routes.PluginRoutes, nil
}

// TestPluginRouteCarriesGenericAttributes verifies that the generic attributes
// of a plugin family's update block reach the built route, whatever the family.
//
// VALIDATES: LARGE_COMMUNITIES, ATOMIC_AGGREGATE, AIGP and a generic attribute
// are pre-built with their exact value octets, and AGGREGATOR is carried typed
// for send time, for FlowSpec and VPLS alike.
// PREVENTS: a family plugin's copy list leaving a configured attribute off the
// wire with no error (the journal row config-route-request-drops-parsed-attributes).
func TestPluginRouteCarriesGenericAttributes(t *testing.T) {
	const attrs = `
                large-community [ 1:2:3 ];
                aggregator 65010:10.0.0.9;
                atomic-aggregate true;
                med 0;
                community [ 65000:2 65000:1 ];
`
	// The block holds one generic attribute, and AIGP has no keyword of its
	// own, so the unknown code and the AIGP each take a run.
	generics := map[string]string{
		"unknown": "attribute [0x99 0xc0 0x00000064];",
		"aigp":    "attribute [0x1a 0x80 0x01000b0000000000000064];",
	}
	families := map[string]string{
		"ipv4/flow":  "ipv4/flow add destination-ipv4 192.168.0.5/32",
		"l2vpn/vpls": "l2vpn/vpls rd 192.168.201.1:123 add ve-id 5 ve-block-offset 1 ve-block-size 8 label-base 10702",
	}
	generated := map[string]map[attribute.AttributeCode][]byte{
		"unknown": {attribute.AttributeCode(0x99): {0, 0, 0, 0x64}},
		"aigp":    {attribute.AttrAIGP: {0x01, 0x00, 0x0B, 0, 0, 0, 0, 0, 0, 0, 100}},
	}
	for name, line := range families {
		for generic, generalAttr := range generics {
			t.Run(name+"/"+generic, func(t *testing.T) {
				want := map[attribute.AttributeCode][]byte{
					attribute.AttrOrigin:          {0x00},
					attribute.AttrMED:             {0, 0, 0, 0},
					attribute.AttrAtomicAggregate: {},
					attribute.AttrCommunity:       {0xFD, 0xE8, 0, 1, 0xFD, 0xE8, 0, 2},
					attribute.AttrLargeCommunity:  {0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3},
				}
				maps.Copy(want, generated[generic])
				checkPluginRouteAttrs(t, attrs+generalAttr, line, want)
			})
		}
	}
}

// checkPluginRouteAttrs loads one route and compares its attributes with want.
func checkPluginRouteAttrs(t *testing.T, attrs, line string, want map[attribute.AttributeCode][]byte) {
	t.Helper()
	routes, err := pluginRouteFromConfig(t, attrs, line)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	route := routes[0]
	got := map[attribute.AttributeCode][]byte{}
	for _, a := range route.Attrs {
		code := attribute.AttributeCode(a.Code)
		if _, twice := got[code]; twice {
			t.Errorf("attribute %s appears twice", code)
		}
		got[code] = a.Value
	}
	for code, value := range want {
		have, ok := got[code]
		if !ok {
			t.Errorf("attribute %s missing", code)
			continue
		}
		if !bytes.Equal(have, value) {
			t.Errorf("attribute %s = %x, want %x", code, have, value)
		}
	}
	if !route.HasAggregator || route.AggregatorASN != 65010 || route.AggregatorIP != [4]byte{10, 0, 0, 9} {
		t.Errorf("aggregator = %v %d %v, want 65010 10.0.0.9", route.HasAggregator, route.AggregatorASN, route.AggregatorIP)
	}
}

// TestSRPolicyRouteCarriesRouteTarget verifies an SR Policy route keeps the
// Route Target configured in its block.
//
// VALIDATES: the IPv4 address-specific Route Target reaches the route as
// EXTENDED_COMMUNITIES beside the Tunnel Encapsulation (RFC 9830 Section 1).
// PREVENTS: an SR Policy route refused at load, or sent with no Route Target,
// so no headend recognizes it (bgp-sr-policy-gobgp interop).
func TestSRPolicyRouteCarriesRouteTarget(t *testing.T) {
	routes, err := pluginRouteFromConfig(t,
		"extended-community [ target:172.30.0.5:0 ];",
		"ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	var extComm, tunnel int
	for _, a := range routes[0].Attrs {
		//exhaustive:ignore // counts only the two attributes the route must carry; the others are not asserted here
		switch attribute.AttributeCode(a.Code) {
		case attribute.AttrExtCommunity:
			extComm++
			if want := []byte{0x01, 0x02, 172, 30, 0, 5, 0, 0}; !bytes.Equal(a.Value, want) {
				t.Errorf("extended communities = %x, want %x", a.Value, want)
			}
		case attribute.AttrTunnelEncap:
			tunnel++
		}
	}
	if extComm != 1 || tunnel != 1 {
		t.Errorf("EXTENDED_COMMUNITIES x%d, TUNNEL_ENCAPSULATION x%d, want one of each", extComm, tunnel)
	}
}

// TestPluginRouteRefusesExtCommunityItDropped verifies the dispatcher refuses
// configured extended communities a family plugin's route left out.
//
// VALIDATES: the refusal names the family and the attribute.
// PREVENTS: a future family plugin silently sending a route without them.
func TestPluginRouteRefusesExtCommunityItDropped(t *testing.T) {
	req := registry.ConfigRouteRequest{ExtCommunity: []byte{0x00, 0x02, 0xFD, 0xE8, 0, 0, 0, 1}}
	_, err := pluginRouteFromRegistry("ipv4/example", req, registry.PluginRoute{}, configRouteGeneric{})
	if err == nil || !strings.Contains(err.Error(), "extended-community is configured, but a ipv4/example route") {
		t.Fatalf("error = %v, want the extended-community refusal", err)
	}
}

// TestPluginRouteRefusesWhatItCannotSend verifies a configured attribute the
// plugin route cannot carry stops the load with its reason.
//
// VALIDATES: a generic AS_PATH holding an AS_SET is refused by name.
// PREVENTS: the attribute being dropped while the route is still sent.
func TestPluginRouteRefusesWhatItCannotSend(t *testing.T) {
	cases := map[string]struct {
		attrs, line, want string
	}{
		"generic AS_SET on flow": {
			attrs: "attribute [0x02 0x40 0x01020000FDE80000FDEA];",
			line:  "ipv4/flow add destination-ipv4 192.168.0.5/32",
			want:  "RFC 9774 Section 3",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := pluginRouteFromConfig(t, tc.attrs, tc.line)
			if err == nil {
				t.Fatal("load succeeded, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %q", err, tc.want)
			}
		})
	}
}
