package network

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// tableSection is the shape of the section buildTables writes, as far as the
// tests read it.
type tableSection struct {
	Firewall struct {
		Table map[string]struct {
			Family string `json:"family"`
			Chain  map[string]struct {
				Type     string           `json:"type"`
				Hook     string           `json:"hook"`
				Priority string           `json:"priority"`
				Policy   string           `json:"policy"`
				Term     []map[string]any `json:"term"`
			} `json:"chain"`
		} `json:"table"`
	} `json:"firewall"`
}

// TestTablesForOutboundTranslation: a translating dual-stack network gets,
// in each family, a postrouting term that jumps from its prefix to its own
// chain, which returns on its bridge and masquerades otherwise. A network
// with masquerade false gets nothing.
func TestTablesForOutboundTranslation(t *testing.T) {
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"]},"routed":{"prefix":["10.90.0.0/24"],"masquerade":"false"}}}`)
	raw, err := buildTables(cfg, nil)
	if err != nil {
		t.Fatalf("buildTables: %v", err)
	}
	var section tableSection
	if err := json.Unmarshal(raw, &section); err != nil {
		t.Fatalf("the section does not read: %v (%s)", err, raw)
	}
	for table, want := range map[string]struct{ family, source string }{
		tableIPv4: {familyIPv4, "10.88.0.0/24"},
		tableIPv6: {familyIPv6, "fd00:88::/64"},
	} {
		got, found := section.Firewall.Table[table]
		if !found {
			t.Fatalf("table %s is missing: %+v", table, section)
		}
		if got.Family != want.family {
			t.Fatalf("table %s family %q, want %q", table, got.Family, want.family)
		}
		base := got.Chain[chainPostrouting]
		if base.Type != "nat" || base.Hook != "postrouting" || base.Policy != "accept" || base.Priority != "100" {
			t.Fatalf("table %s postrouting chain %+v", table, base)
		}
		if len(base.Term) != 1 {
			t.Fatalf("table %s has %d postrouting terms, want 1 (routed has masquerade false)", table, len(base.Term))
		}
		from := object(t, base.Term[0]["from"])
		then := object(t, base.Term[0]["then"])
		if from["source-address"] != want.source || then["jump"] != "outbound-lan" {
			t.Fatalf("table %s term %v", table, base.Term[0])
		}
		outbound := got.Chain["outbound-lan"]
		if outbound.Hook != "" {
			t.Fatalf("the outbound chain is a base chain: %+v", outbound)
		}
		if len(outbound.Term) != 2 {
			t.Fatalf("the outbound chain holds %v", outbound.Term)
		}
		if object(t, outbound.Term[0]["from"])["output-interface"] != "zcn-lan" {
			t.Fatalf("the first outbound term does not match the bridge: %v", outbound.Term[0])
		}
		if _, returns := object(t, outbound.Term[0]["then"])["return"]; !returns {
			t.Fatalf("the bridge term does not return: %v", outbound.Term[0])
		}
		if _, masquerades := object(t, outbound.Term[1]["then"])["masquerade"]; !masquerades {
			t.Fatalf("the second outbound term does not masquerade: %v", outbound.Term[1])
		}
	}
}

// TestTablesWithdrawnWhenNothingTranslates: a config with no translating
// network answers an explicit empty table map, which withdraws both tables.
func TestTablesWithdrawnWhenNothingTranslates(t *testing.T) {
	raw, err := buildTables(Config{}, nil)
	if err != nil {
		t.Fatalf("buildTables: %v", err)
	}
	if string(raw) != `{"firewall":{"table":{}}}` {
		t.Fatalf("the empty set is %s, want an explicit empty table map", raw)
	}
}

// publishedConfig is a dual-stack network whose outbound translation is off,
// so the tables hold the published ports alone, and one instance with two
// ports: tcp 8080 to 80 on every router address, and udp 5300-5310 onto the
// same range on one IPv4 router address.
const publishedConfig = `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"],"masquerade":"false"}},` +
	`"instance":{"web":{"network":[{"name":"lan"}],"port":{` +
	`"http":{"listen-port":"8080","container-port":"80"},` +
	`"dns":{"protocol":"udp","listen-address":["192.0.2.1"],"listen-port":"5300-5310","container-port":"5300-5310"}}}}}`

// publishedRecords assigns web its addresses on lan, as the apply would.
func publishedRecords() map[string]addressRecord {
	return map[string]addressRecord{"lan": {Instances: map[string]assignment{
		"web": {Addresses: []netip.Addr{netip.MustParseAddr("10.88.0.2"), netip.MustParseAddr("fd00:88::2")}},
	}}}
}

// TestTablesForPublishedPort: each published port gets, in each family it is
// published in, the same destination translation term at prerouting and at
// output, in nat chains at the destination translation priority. Every term
// matches the protocol, the listen port and destination-local, so traffic the
// router forwards is never captured, and translates to the instance's
// assigned address on its first network and the container port. A port with
// a listen address matches it and is published in its family only.
func TestTablesForPublishedPort(t *testing.T) {
	raw, err := buildTables(mustParse(t, publishedConfig), publishedRecords())
	if err != nil {
		t.Fatalf("buildTables: %v", err)
	}
	var section tableSection
	if err := json.Unmarshal(raw, &section); err != nil {
		t.Fatalf("the section does not read: %v (%s)", err, raw)
	}
	type wantTerm struct{ name, protocol, port, address, target string }
	for table, want := range map[string][]wantTerm{
		tableIPv4: {
			{"port_web_dns_1", "udp", "5300-5310", "192.0.2.1/32", "10.88.0.2"},
			{"port_web_http", "tcp", "8080", "", "10.88.0.2:80"},
		},
		tableIPv6: {
			{"port_web_http", "tcp", "8080", "", "[fd00:88::2]:80"},
		},
	} {
		got, found := section.Firewall.Table[table]
		if !found {
			t.Fatalf("table %s is missing: %s", table, raw)
		}
		if _, outbound := got.Chain[chainPostrouting]; outbound {
			t.Fatalf("table %s has a postrouting chain with masquerade false: %s", table, raw)
		}
		for _, hook := range []string{"prerouting", "output"} {
			chain, present := got.Chain[hook]
			if !present {
				t.Fatalf("table %s has no %s chain: %s", table, hook, raw)
			}
			shape := [4]string{chain.Type, chain.Hook, chain.Policy, chain.Priority}
			if shape != [4]string{"nat", hook, "accept", "-100"} {
				t.Fatalf("table %s %s chain %+v", table, hook, chain)
			}
			if len(chain.Term) != len(want) {
				t.Fatalf("table %s %s holds %d terms, want %d: %v", table, hook, len(chain.Term), len(want), chain.Term)
			}
			for position, term := range chain.Term {
				checkPortTerm(t, table+" "+hook, term, want[position].name, want[position].protocol,
					want[position].port, want[position].address, want[position].target)
			}
		}
	}
}

// checkPortTerm asserts one destination translation term. An empty address
// asserts that the term matches no destination address.
func checkPortTerm(t *testing.T, where string, term map[string]any, name, protocol, port, address, target string) {
	t.Helper()
	if term["name"] != name {
		t.Fatalf("%s: term %v, want the name %s", where, term, name)
	}
	from := object(t, term["from"])
	if from["protocol"] != protocol {
		t.Fatalf("%s %s: protocol %v, want %s", where, name, from["protocol"], protocol)
	}
	if from["destination-port"] != port {
		t.Fatalf("%s %s: destination-port %v, want %s", where, name, from["destination-port"], port)
	}
	if _, local := from["destination-local"]; !local {
		t.Fatalf("%s %s: the term does not match destination-local, so it would capture forwarded traffic: %v", where, name, from)
	}
	listen, matched := from["destination-address"]
	if address == "" {
		if matched {
			t.Fatalf("%s %s: a port with no listen address matches destination-address %v", where, name, listen)
		}
	}
	if address != "" {
		if listen != address {
			t.Fatalf("%s %s: destination-address %v, want %s", where, name, listen, address)
		}
	}
	dnat := object(t, object(t, term["then"])["dnat"])
	if dnat["to"] != target {
		t.Fatalf("%s %s: dnat to %v, want %s", where, name, dnat["to"], target)
	}
}

// TestTablesRefusePortWithoutAssignment: a port whose instance has no
// assigned address is an error, never a term that translates to nothing.
func TestTablesRefusePortWithoutAssignment(t *testing.T) {
	_, err := buildTables(mustParse(t, publishedConfig), map[string]addressRecord{})
	if err == nil {
		t.Fatal("the tables were built with no address to translate to")
	}
	if !strings.Contains(err.Error(), "no assigned address") {
		t.Fatalf("the error %q does not name the missing address", err)
	}
}

// TestTablesFollowConfigNotRunState: the translation follows the config, not
// the run state. An enabled container's ports are translated whatever its
// run state, which the tables never read, so a stopped one keeps its terms.
// A disabled container's ports are not translated: a config whose only
// translation is a disabled container's ports asks for no table, and the
// apply withdraws the tables it sent. The disabled container keeps its
// address, so enabling it again gives it the same one.
//
// Method: publishedConfig, whose network does not masquerade, is applied
// with web enabled, then with web disabled, through one manager on the fake
// engine; the table sections and the address record are compared.
func TestTablesFollowConfigNotRunState(t *testing.T) {
	enabled := mustParse(t, publishedConfig)
	disabled := mustParse(t, strings.Replace(publishedConfig, `"web":{`, `"web":{"enabled":"false",`, 1))
	if !hasTables(enabled) {
		t.Fatalf("an enabled container's published ports ask for no table")
	}
	if hasTables(disabled) {
		t.Fatalf("a disabled container's published ports ask for a table")
	}
	raw, err := buildTables(disabled, publishedRecords())
	if err != nil {
		t.Fatalf("buildTables: %v", err)
	}
	if string(raw) != `{"firewall":{"table":{}}}` {
		t.Fatalf("a disabled container is translated: %s", raw)
	}

	engine := newFakeEngine()
	m := newManager()
	m.receive(engine)
	ctx := context.Background()
	if err := m.apply(ctx, enabled); err != nil {
		t.Fatalf("apply of the enabled container: %v", err)
	}
	key := zefs.KeyContainerNetworkAddress.Key("lan")
	assigned := string(engine.state[key])
	if !strings.Contains(assigned, `"web"`) {
		t.Fatalf("the enabled container has no address record: %s", assigned)
	}
	if err := m.apply(ctx, disabled); err != nil {
		t.Fatalf("apply of the disabled container: %v", err)
	}
	if len(engine.sections) != 2 {
		t.Fatalf("table sections %v, want the tables then their withdrawal", engine.sections)
	}
	if !strings.Contains(engine.sections[0], `"port_web_http"`) {
		t.Fatalf("the enabled container's tables hold no port term: %s", engine.sections[0])
	}
	if engine.sections[1] != `{"firewall":{"table":{}}}` {
		t.Fatalf("disabling the container did not withdraw its translation: %s", engine.sections[1])
	}
	if kept := string(engine.state[key]); kept != assigned {
		t.Fatalf("disabling the container changed its address record from %s to %s", assigned, kept)
	}
}

// object answers value as a config object, and fails the test when it is not
// one: a table the builder wrote holds objects at these names.
func object(t *testing.T, value any) map[string]any {
	t.Helper()
	fields, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%v is not a config object", value)
	}
	return fields
}
