// Design: docs/architecture/ospf/ospf-4-component-config.md -- the virtual-link config surface.
// Related: yang/ze-ospf-conf.yang -- list virtual-link, which carries no cost leaf.
// Related: rfc2328_virtual_cost_test.go -- the cost SPF derives for the link instead.
//
// VALIDATES: RFC 2328 Section 15, "The cost of a virtual link is NOT configured." The operator
// config text goes through the YANG-driven parser, which refuses a cost statement under a
// virtual link, while the same link without one is accepted and reaches the plugin.
// PREVENTS: a cost leaf added to the virtual-link list, which would let an operator override
// the intra-area path cost the link is defined to carry.
package ospf

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	ospfyang "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// rfc2328VirtualLinkConfigText returns an ABR config with a virtual link to 10.0.0.2 through
// Transit area 0.0.0.1, with extra appended inside the virtual-link block.
func rfc2328VirtualLinkConfigText(extra string) string {
	return `ospf {
    router-id 10.0.0.1
    areas {
        area 0.0.0.0 {
            area-type normal
        }
        area 0.0.0.1 {
            area-type normal
            virtual-link 10.0.0.2 {
                hello-interval 10
` + extra + `
            }
        }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.1
        }
        interface eth1 {
            area 0.0.0.0
        }
    }
}`
}

// TestRFC2328VirtualLinkCostNotConfigurable: the virtual link parses without a cost, and a
// cost statement under it is refused by the parser.
func TestRFC2328VirtualLinkCostNotConfigurable(t *testing.T) {
	schema := map[string]string{"ospf": ospfyang.ZeOSPFConfYANG}
	tree, err := config.ParseTreeWithYANG(rfc2328VirtualLinkConfigText(""), schema)
	if err != nil {
		t.Fatalf("a virtual link without a cost was refused: %v", err)
	}
	data, err := json.Marshal(map[string]any{Namespace: tree.ToPluginMap()[Namespace]})
	if err != nil {
		t.Fatalf("marshal plugin map: %v", err)
	}
	cfg, err := parseOSPFConfig([]configSection{{Root: Namespace, Data: string(data)}}, nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if len(cfg.VirtualLinks) != 1 || cfg.VirtualLinks[0].RemoteRouterID != vlRID(t, "10.0.0.2") {
		t.Fatalf("virtual links = %+v, want the one link to 10.0.0.2", cfg.VirtualLinks)
	}

	// RFC requirement: RFC2328-15-2 negative -- the cost of a virtual link is not configured:
	// config text with `cost 10` inside the virtual-link block is refused by the YANG-driven
	// parser over ZeOSPFConfYANG, whose virtual-link list declares no cost leaf, and the
	// refusal names the cost statement.
	_, err = config.ParseTreeWithYANG(rfc2328VirtualLinkConfigText("                cost 10"), schema)
	if err == nil {
		t.Fatal("a cost statement under a virtual link was accepted")
	}
	if !strings.Contains(err.Error(), "cost") {
		t.Fatalf("the refusal does not name the cost statement: %v", err)
	}
}

// rfc2328VirtualLinkV3ConfigText returns the same ABR shape under the OSPFv3 IPv6 address
// family, whose areas come from the ospf-af-topology grouping and its own virtual-link list,
// with extra appended inside the virtual-link block.
func rfc2328VirtualLinkV3ConfigText(extra string) string {
	return `ospf {
    router-id 10.0.0.1
    address-family {
        ipv6 {
            areas {
                area 0.0.0.0 {
                    area-type normal
                }
                area 0.0.0.1 {
                    area-type normal
                    virtual-link 10.0.0.2 {
                        hello-interval 10
` + extra + `
                    }
                }
            }
            interfaces {
                interface eth0 {
                    area 0.0.0.1
                }
                interface eth1 {
                    area 0.0.0.0
                }
            }
        }
    }
}`
}

// TestRFC2328VirtualLinkCostNotConfigurableV3 covers the second virtual-link list, the one
// the ospf-af-topology grouping gives every OSPFv3 address family: the link parses without a
// cost and reaches the IPv6 sub-config, and a cost statement under it is refused.
func TestRFC2328VirtualLinkCostNotConfigurableV3(t *testing.T) {
	schema := map[string]string{"ospf": ospfyang.ZeOSPFConfYANG}
	tree, err := config.ParseTreeWithYANG(rfc2328VirtualLinkV3ConfigText(""), schema)
	if err != nil {
		t.Fatalf("an OSPFv3 virtual link without a cost was refused: %v", err)
	}
	data, err := json.Marshal(map[string]any{Namespace: tree.ToPluginMap()[Namespace]})
	if err != nil {
		t.Fatalf("marshal plugin map: %v", err)
	}
	cfg, err := parseOSPFConfig([]configSection{{Root: Namespace, Data: string(data)}}, nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("the ipv6 address family did not reach the IPv6 sub-config")
	}
	if len(cfg.V6.VirtualLinks) != 1 || cfg.V6.VirtualLinks[0].RemoteRouterID != vlRID(t, "10.0.0.2") {
		t.Fatalf("OSPFv3 virtual links = %+v, want the one link to 10.0.0.2", cfg.V6.VirtualLinks)
	}

	// RFC requirement: RFC2328-15-2 negative -- the cost of a virtual link is not configured,
	// in the OSPFv3 address family too: config text with `cost 10` inside the virtual-link
	// block of `address-family ipv6` is refused by the YANG-driven parser over ZeOSPFConfYANG,
	// whose ospf-af-topology virtual-link list declares no cost leaf, and the refusal names
	// the cost statement.
	_, err = config.ParseTreeWithYANG(rfc2328VirtualLinkV3ConfigText("                        cost 10"), schema)
	if err == nil {
		t.Fatal("a cost statement under an OSPFv3 virtual link was accepted")
	}
	if !strings.Contains(err.Error(), "cost") {
		t.Fatalf("the refusal does not name the cost statement: %v", err)
	}
}
