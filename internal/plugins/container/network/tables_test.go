package network

import (
	"encoding/json"
	"testing"
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
	raw, err := buildTables(cfg)
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
	raw, err := buildTables(Config{})
	if err != nil {
		t.Fatalf("buildTables: %v", err)
	}
	if string(raw) != `{"firewall":{"table":{}}}` {
		t.Fatalf("the empty set is %s, want an explicit empty table map", raw)
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
