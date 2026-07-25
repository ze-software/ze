// VALIDATES: AC-2 the source term (MatchSourceAddress + Limit, Drop fallback) and
// the per-family table grouping (v4/v6 into separate firewall tables), plus the
// allowlist overlap gate.
// PREVENTS: a destination match instead of source, a missing rate-limit action,
// and v4/v6 terms colliding in one table family.

package shape

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestBuildSourceTerm(t *testing.T) {
	entity := netip.MustParsePrefix("198.51.100.5/32")

	limitCfg := DefaultConfig() // action=limit
	term := buildSourceTerm(entity, limitCfg)
	if len(term.Matches) != 1 {
		t.Fatalf("term matches = %d, want 1", len(term.Matches))
	}
	sa, ok := term.Matches[0].(firewall.MatchSourceAddress)
	if !ok || sa.Prefix != entity {
		t.Fatalf("match = %#v, want MatchSourceAddress{%v}", term.Matches[0], entity)
	}
	hasLimit := false
	for _, a := range term.Actions {
		if _, ok := a.(firewall.Limit); ok {
			hasLimit = true
		}
	}
	if !hasLimit {
		t.Errorf("limit action missing: %#v", term.Actions)
	}

	dropCfg := DefaultConfig()
	dropCfg.Action = "drop"
	dterm := buildSourceTerm(entity, dropCfg)
	hasDrop := false
	for _, a := range dterm.Actions {
		if _, ok := a.(firewall.Drop); ok {
			hasDrop = true
		}
	}
	if !hasDrop {
		t.Errorf("drop fallback action missing: %#v", dterm.Actions)
	}
}

func TestBuildTablesPerFamily(t *testing.T) {
	cfg := DefaultConfig()
	armed := []netip.Prefix{
		netip.MustParsePrefix("198.51.100.5/32"),
		netip.MustParsePrefix("203.0.113.9/32"),
		netip.MustParsePrefix("2001:db8::1/128"),
	}
	tables := buildTables(armed, cfg)
	if len(tables) != 2 {
		t.Fatalf("tables = %d, want 2 (one v4, one v6)", len(tables))
	}
	byFamily := map[firewall.TableFamily]int{}
	for _, tb := range tables {
		byFamily[tb.Family] = len(tb.Chains[0].Terms)
	}
	if byFamily[firewall.FamilyIP] != 2 {
		t.Errorf("v4 terms = %d, want 2", byFamily[firewall.FamilyIP])
	}
	if byFamily[firewall.FamilyIP6] != 1 {
		t.Errorf("v6 terms = %d, want 1", byFamily[firewall.FamilyIP6])
	}

	if got := buildTables(nil, cfg); got != nil {
		t.Errorf("no armed entities -> %v, want nil tables", got)
	}
}

func TestAllowlisted(t *testing.T) {
	allow := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if !allowlisted(netip.MustParsePrefix("10.1.2.3/32"), allow) {
		t.Error("10.1.2.3/32 should be allowlisted by 10.0.0.0/8")
	}
	if allowlisted(netip.MustParsePrefix("198.51.100.5/32"), allow) {
		t.Error("198.51.100.5/32 should not be allowlisted")
	}
}
