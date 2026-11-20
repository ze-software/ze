//go:build ze_l2tp && ze_vpp && ze_rest && ze_radius

// Design: docs/labs/pppoe-interop.md -- the product daemon loads the config the lab writes.
// Related: product_plan.go -- productConfigWith, productInterfaces

// The constraint names the gate of each block the product config writes: pppoe
// and l2tp (ze_l2tp), the vpp section and the VPP PPPoE provider (ze_vpp), the
// REST api-server (ze_rest) and the framed lab's RADIUS auth (ze_radius). A
// build without one of them has no schema for its block and cannot judge it.

package testvpppppoe

import (
	"strings"
	"testing"

	configcli "github.com/ze-software/ze/internal/component/config/cli"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // The schema and validators the staged daemon loads.
)

// productVLANAccess is the VLAN 100 sub-interface prepareProduct names in
// modeVLAN.
const productVLANAccess = "host-eth0.100"

// PREVENTS: a product action whose daemon refuses its own config at load, as
// it did once Ze required every PPPoE access interface to be declared.
// Method: every config the base and framed actions write, in each access
// topology, through `ze config validate`'s own walk, with the schema and
// validators the daemon links.
//
// The IPv6 action's configs (productIPv6Config) are not in the table: they
// write `pool { ipv6-na { ... } }`, a pool field Ze has never had, so they are
// refused at load. Journal row test-against-broken-path.md, 2026-11-20, owned
// by spec-pppoe-vpp-dataplane; add them back once that row is fixed.
func TestProductConfigsValidate(t *testing.T) {
	framed := framedAuth("198.19.40.3", "198.19.40.5")
	for name, config := range map[string]string{
		"plain":     productConfig(hostAccessInterface, ""),
		"namespace": productConfig(hostAccessInterface, controlNamespace),
		"vlan":      productConfig(productVLANAccess, ""),
		"framed":    productConfigWith(hostAccessInterface, "", framed),
	} {
		t.Run(name, func(t *testing.T) {
			if err := configcli.ValidateContent(config, "product-"+name); err != nil {
				t.Fatalf("product config refused:\n%v\nconfig:\n%s", err, config)
			}
		})
	}
}

// PREVENTS: TestProductConfigsValidate passing because this test binary does
// not link the check that refused the lab's config.
// Method: the plain and VLAN configs with their access interface renamed to one
// the interface section does not declare must be refused with that message.
func TestProductConfigRefusesUndeclaredAccess(t *testing.T) {
	for access, undeclared := range map[string]string{
		hostAccessInterface: "host-eth1",
		productVLANAccess:   "host-eth0.200",
	} {
		config := productConfig(access, "")
		served := "interface " + access + " { }"
		if !strings.Contains(config, served) {
			t.Fatalf("product config does not serve %s:\n%s", access, config)
		}
		config = strings.Replace(config, served, "interface "+undeclared+" { }", 1)
		err := configcli.ValidateContent(config, "product-undeclared")
		if err == nil || !strings.Contains(err.Error(), "interface \""+undeclared+"\" is not configured") {
			t.Fatalf("undeclared access %s: error = %v, want it refused as not configured", undeclared, err)
		}
	}
}
