package network

import (
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// portInstances is a container body with lanNetwork and the instances body,
// whose attachments name lan.
func portInstances(instances string) string {
	return `{` + lanNetwork + `,"instance":{` + instances + `}}`
}

// TestVerifyAcceptsPublishedPorts is the positive control of the port
// refusals: two containers publish one protocol and port on two different
// router addresses, one of them publishes the same port over udp on every
// address, and a range is published onto the same range. The protocol
// defaults to tcp.
func TestVerifyAcceptsPublishedPorts(t *testing.T) {
	cfg := mustParse(t, portInstances(
		`"a":{"network":[{"name":"lan"}],"port":{"web":{"public-address":["192.0.2.1"],"public-port":"8080","container-port":"80"},`+
			`"span":{"public-port":"9000-9001","container-port":"9000-9001"}}},`+
			`"b":{"network":[{"name":"lan"}],"port":{"web":{"public-address":["192.0.2.2"],"public-port":"8080","container-port":"80"},`+
			`"dns":{"protocol":"udp","public-port":"8080","container-port":"53"}}}`))
	if len(cfg.Instances) != 2 {
		t.Fatalf("parsed %d instances, want 2: %+v", len(cfg.Instances), cfg.Instances)
	}
	ports := cfg.Instances[0].Ports
	if len(ports) != 2 {
		t.Fatalf("instance a has %d ports, want 2: %+v", len(ports), ports)
	}
	// Sorted by name: span before web; tcp is the default protocol.
	want := []Port{
		{Name: "span", Protocol: protocolTCP, PublicPorts: portRange{first: 9000, last: 9001}, ContainerPorts: portRange{first: 9000, last: 9001}},
		{
			Name: "web", Protocol: protocolTCP, PublicAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")},
			PublicPorts: portRange{first: 8080, last: 8080}, ContainerPorts: portRange{first: 80, last: 80},
		},
	}
	if !reflect.DeepEqual(ports, want) {
		t.Fatalf("instance a ports %+v, want %+v", ports, want)
	}
}

// TestVerifyRefusesPortWithNoNetwork is rule V-18: a port on an instance with
// no network entry, with networks configured and with none.
func TestVerifyRefusesPortWithNoNetwork(t *testing.T) {
	const phrase = "container solo publishes a port and has no network"
	expectRefusals(t, []refusalCase{
		{"other instances attached", portInstances(
			`"attached":{"network":[{"name":"lan"}]},"solo":{"port":{"web":{"public-port":"8080","container-port":"80"}}}`), phrase},
		{"no network configured", `{"instance":{"solo":{"port":{"web":{"public-port":"8080","container-port":"80"}}}}}`, phrase},
	})
}

// TestVerifyRefusesUnequalPortRanges is rule V-6 on one entry: ranges of two
// lengths, a range that runs backward, a port outside 1 to 65535, and a
// range published onto another range of the same length, which the
// translation cannot carry exactly.
func TestVerifyRefusesUnequalPortRanges(t *testing.T) {
	port := func(listen, target string) string {
		return portInstances(`"a":{"network":[{"name":"lan"}],"port":{"p":{"public-port":"` + listen + `","container-port":"` + target + `"}}}`)
	}
	expectRefusals(t, []refusalCase{
		{"two lengths", port("8080-8081", "80"), "container a port p: 2 ports are published to 1 ports"},
		{"backward", port("8090-8080", "8090-8080"), "public-port 8090-8080 runs backward"},
		{"zero", port("0", "80"), "public-port 0 is not a port"},
		{"above the range", port("8080", "65536"), "container-port 65536 is not a port"},
		{"shifted range", port("8000-8010", "5000-5010"), "the range 8000-8010 is published to the range 5000-5010"},
		{"no container port", portInstances(`"a":{"network":[{"name":"lan"}],"port":{"p":{"public-port":"8080"}}}`), "it has no container-port"},
		{"unknown protocol", portInstances(`"a":{"network":[{"name":"lan"}],"port":{"p":{"protocol":"sctp","public-port":"8080","container-port":"80"}}}`), "protocol sctp is neither tcp nor udp"},
	})
}

// TestVerifyRefusesDuplicateListenEndpoint is rule V-6 across entries: two
// ports of one protocol whose port ranges overlap, on the same address, or
// where either is published on every address of the family. The positive
// control, two different addresses or two protocols, is
// TestVerifyAcceptsPublishedPorts.
func TestVerifyRefusesDuplicateListenEndpoint(t *testing.T) {
	pair := func(first, second string) string {
		return portInstances(`"a":{"network":[{"name":"lan"}],"port":{"p":{` + first + `}}},` +
			`"b":{"network":[{"name":"lan"}],"port":{"q":{` + second + `}}}`)
	}
	expectRefusals(t, []refusalCase{
		{"every address twice", pair(`"public-port":"8080","container-port":"80"`, `"public-port":"8080","container-port":"81"`),
			"container b port q and container a port p both publish tcp port 8080 on every IPv4 address of the router"},
		{"one address twice", pair(`"public-address":["192.0.2.1"],"public-port":"8080","container-port":"80"`,
			`"public-address":["192.0.2.1"],"public-port":"8080","container-port":"80"`),
			"both publish tcp port 8080 on 192.0.2.1"},
		{"an address under every address", pair(`"public-port":"8080","container-port":"80"`,
			`"public-address":["192.0.2.1"],"public-port":"8080","container-port":"80"`),
			"both publish tcp port 8080 on 192.0.2.1"},
		{"overlapping ranges", pair(`"public-port":"8000-8010","container-port":"8000-8010"`, `"public-port":"8005","container-port":"80"`),
			"both publish tcp port 8005 on every IPv4 address of the router"},
		{"two ports of one instance", portInstances(`"a":{"network":[{"name":"lan"}],"port":{` +
			`"p":{"public-port":"8080","container-port":"80"},"q":{"public-port":"8080","container-port":"81"}}}`),
			"container a port q and container a port p both publish tcp port 8080"},
	})
}

// TestVerifyRefusesPortFamilyWithoutAddress: a public address of a family the
// instance's first network has no prefix of has no address to translate to.
func TestVerifyRefusesPortFamilyWithoutAddress(t *testing.T) {
	expectRefusals(t, []refusalCase{
		{"IPv6 public address on an IPv4 network", portInstances(
			`"a":{"network":[{"name":"lan"}],"port":{"p":{"public-address":["2001:db8::1"],"public-port":"8080","container-port":"80"}}}`),
			"public-address 2001:db8::1 has no address of its family to reach on network lan"},
	})
}

// TestVerifyRefusesPortOnFirewallWithoutTranslation is the published port half
// of rule V-19: a port needs the nft firewall backend, even when no network
// masquerades.
func TestVerifyRefusesPortOnFirewallWithoutTranslation(t *testing.T) {
	container := `{"network":{"lan":{"prefix":["10.88.0.0/24"],"masquerade":"false"}},` +
		`"instance":{"a":{"network":[{"name":"lan"}],"port":{"p":{"public-port":"8080","container-port":"80"}}}}}`
	_, err := parseSections([]sdk.ConfigSection{
		{Root: containerRoot, Data: `{"container":` + container + `}`},
		{Root: interfaceRoot, Data: `{"interface":{}}`},
		{Root: firewallRoot, Data: `{"firewall":{"backend":"vpp"}}`},
	})
	if err == nil {
		t.Fatal("a published port was accepted on the vpp firewall backend")
	}
	refusal, isOperator := errors.AsType[*opererr.Error](err)
	if !isOperator {
		t.Fatalf("the refusal is not an operator error: %v", err)
	}
	if !strings.Contains(refusal.Error(), "container a publishes a port and firewall backend vpp cannot carry its translation") {
		t.Fatalf("the refusal %q does not name the port and the backend", refusal.Error())
	}
}

// TestVerifyJudgesDisabledInstancePorts: a disabled instance's ports are not
// translated, but they are judged like any other, so the config stays valid
// when the instance is enabled again: one that overlaps an enabled
// instance's port is refused (V-6). An enabled leaf that is neither true nor
// false is refused rather than read as either.
//
// Method: each case is a delivery the parse must refuse, with the phrase the
// refusal names.
func TestVerifyJudgesDisabledInstancePorts(t *testing.T) {
	expectRefusals(t, []refusalCase{
		{"a disabled instance's overlapping port", portInstances(
			`"a":{"network":[{"name":"lan"}],"port":{"p":{"public-port":"8080","container-port":"80"}}},` +
				`"b":{"enabled":"false","network":[{"name":"lan"}],"port":{"q":{"public-port":"8080","container-port":"81"}}}`),
			"container b port q and container a port p both publish tcp port 8080"},
		{"an unreadable enabled leaf", portInstances(`"a":{"enabled":"maybe","network":[{"name":"lan"}]}`),
			"container a: enabled is neither true nor false"},
	})
}
