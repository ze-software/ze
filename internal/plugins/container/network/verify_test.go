package network

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// refusalCase is one config the rules refuse, and a phrase the refusal's
// problem holds.
type refusalCase struct {
	name      string
	container string
	phrase    string
}

// expectRefusals parses each case and asserts an operator error whose
// problem names the case's phrase. A refusal of another rule fails the case,
// so a case cannot pass on the wrong rule.
func expectRefusals(t *testing.T, cases []refusalCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSections(sections(tc.container))
			if err == nil {
				t.Fatal("the config was accepted")
			}
			refusal, isOperator := errors.AsType[*opererr.Error](err)
			if !isOperator {
				t.Fatalf("the refusal is not an operator error: %v", err)
			}
			if !strings.Contains(refusal.Error(), tc.phrase) {
				t.Fatalf("the refusal %q does not name %q", refusal.Error(), tc.phrase)
			}
		})
	}
}

const lanNetwork = `"network":{"lan":{"prefix":["10.88.0.0/24"]}}`

// TestVerifyAcceptsValidNetwork is the positive control of the refusals
// below: two networks, set and assigned addresses, the attachment order kept.
func TestVerifyAcceptsValidNetwork(t *testing.T) {
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"],"mtu":"1400","masquerade":"false"},"dmz":{"prefix":["10.89.0.0/30"]}},`+
		`"instance":{"web":{"network":[{"name":"lan","address":["10.88.0.10"]},{"name":"dmz"}]}}}`)
	if len(cfg.Networks) != 2 {
		t.Fatalf("networks %v, want dmz and lan", cfg.Networks)
	}
	lan, _ := cfg.network("lan")
	if lan.MTU != 1400 {
		t.Fatalf("lan mtu %d, want 1400", lan.MTU)
	}
	if lan.Masquerade {
		t.Fatal("lan masquerade is true, want the set false")
	}
	if lan.Bridge != "zcn-lan" {
		t.Fatalf("lan bridge %q, want zcn-lan", lan.Bridge)
	}
	web := cfg.Instances[0]
	if web.Attachments[0].Network != "lan" {
		t.Fatalf("web's first attachment is %q, want lan, the one written first", web.Attachments[0].Network)
	}
}

// TestVerifyRefusesDuplicateAddress: rule V-4, one address set on two
// attachments of a network.
func TestVerifyRefusesDuplicateAddress(t *testing.T) {
	expectRefusals(t, []refusalCase{{
		name:      "same address",
		container: `{` + lanNetwork + `,"instance":{"a":{"network":[{"name":"lan","address":["10.88.0.9"]}]},"b":{"network":[{"name":"lan","address":["10.88.0.9"]}]}}}`,
		phrase:    "container a holds it",
	}})
}

// TestVerifyRefusesAddressOutsidePrefix: rule V-4, an address outside the
// prefix, the network, broadcast and gateway addresses, and two addresses of
// one family.
func TestVerifyRefusesAddressOutsidePrefix(t *testing.T) {
	attach := func(address string) string {
		return `{` + lanNetwork + `,"instance":{"a":{"network":[{"name":"lan","address":[` + address + `]}]}}}`
	}
	expectRefusals(t, []refusalCase{
		{"outside", attach(`"10.99.0.9"`), "not inside the network's prefixes"},
		{"network address", attach(`"10.88.0.0"`), "network or broadcast address"},
		{"broadcast", attach(`"10.88.0.255"`), "network or broadcast address"},
		{"gateway", attach(`"10.88.0.1"`), "the network's gateway"},
		{"two of one family", attach(`"10.88.0.9","10.88.0.8"`), "already sets an address of that family"},
		{"unknown network", `{` + lanNetwork + `,"instance":{"a":{"network":[{"name":"wan"}]}}}`, "network wan is not a configured container network"},
	})
}

// TestVerifyRefusesOverlappingNetworks: rule V-5, two networks whose
// prefixes overlap, and two prefixes of one family on one network.
func TestVerifyRefusesOverlappingNetworks(t *testing.T) {
	expectRefusals(t, []refusalCase{
		{"overlap", `{"network":{"a":{"prefix":["10.88.0.0/16"]},"b":{"prefix":["10.88.1.0/24"]}}}`, "overlaps network b"},
		{"two of one family", `{"network":{"a":{"prefix":["10.88.0.0/24","10.89.0.0/24"]}}}`, "second prefix"},
		{"host bits", `{"network":{"a":{"prefix":["10.88.0.5/24"]}}}`, "host bits set"},
	})
}

// TestVerifyRefusesExhaustedPrefix: rule V-15. A /30 holds two host
// addresses, one of them the gateway: one attachment fills it, two are
// refused.
func TestVerifyRefusesExhaustedPrefix(t *testing.T) {
	mustParse(t, `{"network":{"p2p":{"prefix":["10.88.0.0/30"]}},"instance":{"a":{"network":[{"name":"p2p"}]}}}`)
	expectRefusals(t, []refusalCase{{
		name:      "two on a /30",
		container: `{"network":{"p2p":{"prefix":["10.88.0.0/30"]}},"instance":{"a":{"network":[{"name":"p2p"}]},"b":{"network":[{"name":"p2p"}]}}}`,
		phrase:    "2 containers attach and prefix 10.88.0.0/30 has 1 free addresses",
	}})
}

// TestVerifyRefusesGatewayOutsidePrefix: rule V-5, a set gateway outside its
// prefix, on its network address, and two of one family.
func TestVerifyRefusesGatewayOutsidePrefix(t *testing.T) {
	expectRefusals(t, []refusalCase{
		{"outside", `{"network":{"a":{"prefix":["10.88.0.0/24"],"gateway":["10.99.0.1"]}}}`, "outside the network's prefixes"},
		{"network address", `{"network":{"a":{"prefix":["10.88.0.0/24"],"gateway":["10.88.0.0"]}}}`, "network or broadcast address"},
		{"two", `{"network":{"a":{"prefix":["10.88.0.0/24"],"gateway":["10.88.0.1","10.88.0.2"]}}}`, "already has the gateway"},
	})
}

// TestVerifyRefusesPrefixLength: rule V-13 at its boundaries: IPv4 /30 and
// IPv6 /126 are accepted, /31 and /127 refused.
func TestVerifyRefusesPrefixLength(t *testing.T) {
	mustParse(t, `{"network":{"a":{"prefix":["10.88.0.0/30","fd00::/126"]}}}`)
	expectRefusals(t, []refusalCase{
		{"ipv4 31", `{"network":{"a":{"prefix":["10.88.0.0/31"]}}}`, "length a container network cannot use"},
		{"ipv6 127", `{"network":{"a":{"prefix":["fd00::/127"]}}}`, "length a container network cannot use"},
	})
}

// TestVerifyRefusesIPv6BelowMinimumMTU: rule V-14, and the mtu bounds.
func TestVerifyRefusesIPv6BelowMinimumMTU(t *testing.T) {
	mustParse(t, `{"network":{"a":{"prefix":["10.88.0.0/24"],"mtu":"68"}}}`)
	mustParse(t, `{"network":{"a":{"prefix":["fd00::/64"],"mtu":"1280"}}}`)
	expectRefusals(t, []refusalCase{
		{"ipv6 1279", `{"network":{"a":{"prefix":["fd00::/64"],"mtu":"1279"}}}`, "below 1280"},
		{"67", `{"network":{"a":{"prefix":["10.88.0.0/24"],"mtu":"67"}}}`, "below 68"},
		{"16001", `{"network":{"a":{"prefix":["10.88.0.0/24"],"mtu":"16001"}}}`, "above 16000"},
	})
}

// TestVerifyRefusesBridgeNameInInterfaceConfig: rule V-16, the bridge name
// declared under interface, and an interface address inside a prefix.
func TestVerifyRefusesBridgeNameInInterfaceConfig(t *testing.T) {
	refuse := func(interfaces string) error {
		_, err := parseSections([]sdk.ConfigSection{
			{Root: containerRoot, Data: `{"container":{` + lanNetwork + `}}`},
			{Root: interfaceRoot, Data: `{"interface":` + interfaces + `}`},
			{Root: firewallRoot, Data: `{"firewall":{}}`},
		})
		return err
	}
	if err := refuse(`{"bridge":{"zcn-lan":{}}}`); err == nil || !strings.Contains(err.Error(), "bridge device zcn-lan") {
		t.Fatalf("the bridge name under interface was not refused: %v", err)
	}
	if err := refuse(`{"ethernet":{"eth0":{"unit":{"0":{"address":["10.88.0.7/24"]}}}}}`); err == nil || !strings.Contains(err.Error(), "10.88.0.7") {
		t.Fatalf("an interface address inside the prefix was not refused: %v", err)
	}
	if err := refuse(`{"ethernet":{"eth0":{"unit":{"0":{"address":["192.0.2.7/24"]}}}}}`); err != nil {
		t.Fatalf("an interface address outside every prefix was refused: %v", err)
	}
}

// TestVerifyRefusesDuplicateHardwareAddress: rule V-17, one set hardware
// address on two attachments of a network.
func TestVerifyRefusesDuplicateHardwareAddress(t *testing.T) {
	expectRefusals(t, []refusalCase{{
		name:      "same mac",
		container: `{` + lanNetwork + `,"instance":{"a":{"network":[{"name":"lan","mac-address":"02:00:00:00:00:01"}]},"b":{"network":[{"name":"lan","mac-address":"02:00:00:00:00:01"}]}}}`,
		phrase:    "the same hardware address 02:00:00:00:00:01",
	}})
}

// TestVerifyRefusesNetworkWithoutComponents: a network needs the interface
// and firewall components, so a config without their sections is refused
// naming the section to add. A config with no network needs neither.
func TestVerifyRefusesNetworkWithoutComponents(t *testing.T) {
	only := []sdk.ConfigSection{{Root: containerRoot, Data: `{"container":{` + lanNetwork + `}}`}}
	if _, err := parseSections(only); err == nil || !strings.Contains(err.Error(), "no interface section") {
		t.Fatalf("a network with no interface section: %v", err)
	}
	only = append(only, sdk.ConfigSection{Root: interfaceRoot, Data: `{"interface":{}}`})
	if _, err := parseSections(only); err == nil || !strings.Contains(err.Error(), "no firewall section") {
		t.Fatalf("a network with no firewall section: %v", err)
	}
	if _, err := parseSections([]sdk.ConfigSection{{Root: containerRoot, Data: `{"container":{"instance":{"a":{}}}}`}}); err != nil {
		t.Fatalf("a config with no network was refused: %v", err)
	}
}

// TestVerifyRefusesUnsupportedBackend: rule V-19, through the schema's
// ze:backend gate for the interface backend, and the firewall backend that
// cannot carry the tables.
func TestVerifyRefusesUnsupportedBackend(t *testing.T) {
	parse := func(interfaces, firewall string) error {
		_, err := parseSections([]sdk.ConfigSection{
			{Root: containerRoot, Data: `{"container":{` + lanNetwork + `}}`},
			{Root: interfaceRoot, Data: `{"interface":` + interfaces + `}`},
			{Root: firewallRoot, Data: `{"firewall":` + firewall + `}`},
		})
		return err
	}
	if err := parse(`{"backend":"vpp"}`, `{}`); err == nil || !strings.Contains(err.Error(), "set interface backend netlink") {
		t.Fatalf("a network on the vpp interface backend: %v", err)
	}
	if err := parse(`{}`, `{"backend":"vpp"}`); err == nil || !strings.Contains(err.Error(), "firewall backend vpp") {
		t.Fatalf("a translating network on the vpp firewall backend: %v", err)
	}
	if err := parse(`{"backend":"netlink"}`, `{"backend":"nft"}`); err != nil {
		t.Fatalf("a network on netlink and nft was refused: %v", err)
	}
}
