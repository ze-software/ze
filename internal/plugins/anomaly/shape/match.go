// Design: docs/architecture/anomaly/anomaly-2-shape.md -- source entity to firewall term/table

package shape

import (
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The table names carry the "ze_" ownership prefix, as every other firewall
// owner does. The nft backend sweeps a table before it re-adds it only when the
// name carries that prefix, so without it an auto-reverted rule kept enforcing
// and every reinstall appended a second copy of each rule. The owner key stays
// "anomaly-shape" and so does the name an operator reads: the CLI and the web
// pages render it through firewall.StripZeTablePrefix.
const (
	tableNameV4 = "ze_anomaly-shape"
	tableNameV6 = "ze_anomaly-shape6"
)

var termNameSanitizer = strings.NewReplacer("/", "-", ".", "-", ":", "-")

// buildActions returns the terminal action for an armed entity: a source
// rate-limit (surgical throttle) by default, or an outright drop as the fallback.
func buildActions(cfg *Config) []firewall.Action {
	if cfg.Action == ActionDrop {
		return []firewall.Action{firewall.Counter{}, firewall.Drop{}}
	}
	return []firewall.Action{firewall.Counter{}, firewall.Limit{
		Rate:      cfg.LimitRate,
		Unit:      cfg.LimitUnit,
		Dimension: firewall.RateDimensionPackets,
		Burst:     cfg.LimitBurst,
	}}
}

// buildSourceTerm builds a firewall term matching the anomalous SOURCE prefix and
// applying the configured action.
func buildSourceTerm(entity netip.Prefix, cfg *Config) firewall.Term {
	var tb textbuf.Buffer
	tb.Str("anomaly-").Str(termNameSanitizer.Replace(entity.String()))
	return firewall.Term{
		Name:    tb.String(),
		Matches: []firewall.Match{firewall.MatchSourceAddress{Prefix: entity}},
		Actions: buildActions(cfg),
	}
}

// buildTables renders the armed entities into firewall tables, one per address
// family (v4/v6 cannot share an nft table family). Returns nil when nothing is
// armed. This is the whole owner term set, re-registered on every change.
func buildTables(armed []netip.Prefix, cfg *Config) []firewall.Table {
	var v4, v6 []firewall.Term
	for _, e := range armed {
		term := buildSourceTerm(e, cfg)
		if e.Addr().Is4() {
			v4 = append(v4, term)
		} else {
			v6 = append(v6, term)
		}
	}
	var tables []firewall.Table
	if len(v4) > 0 {
		tables = append(tables, mkTable(tableNameV4, firewall.FamilyIP, v4))
	}
	if len(v6) > 0 {
		tables = append(tables, mkTable(tableNameV6, firewall.FamilyIP6, v6))
	}
	return tables
}

func mkTable(name string, family firewall.TableFamily, terms []firewall.Term) firewall.Table {
	return firewall.Table{
		Name:   name,
		Family: family,
		Chains: []firewall.Chain{{
			Name:     "ingress",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: -200,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}
}

// allowlisted reports whether entity overlaps any protected prefix; such sources
// are never armed (self-lockout guard).
func allowlisted(entity netip.Prefix, allowlist []netip.Prefix) bool {
	for _, allow := range allowlist {
		if allow.Overlaps(entity) {
			return true
		}
	}
	return false
}
