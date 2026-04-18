// Design: docs/architecture/core-design.md -- nftables backend Linux implementation

//go:build linux

package firewallnft

import (
	"fmt"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// zeTablePrefix is the ownership prefix for ze-managed tables in the kernel.
const zeTablePrefix = "ze_"

// backend implements firewall.Backend using google/nftables.
type backend struct {
	conn *nftables.Conn
}

func newBackend() (firewall.Backend, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("firewallnft: open netlink: %w", err)
	}
	return &backend{conn: conn}, nil
}

// Apply receives the full desired state and reconciles against the kernel.
// Creates new ze_* tables, replaces changed ones, deletes orphans.
// All operations are atomic via Flush().
func (b *backend) Apply(desired []firewall.Table) error {
	// List current ze_* tables.
	currentTables, err := b.conn.ListTables()
	if err != nil {
		return fmt.Errorf("firewallnft: list tables: %w", err)
	}

	// Build set of desired table names for orphan detection.
	desiredNames := make(map[string]bool, len(desired))
	for i := range desired {
		desiredNames[desired[i].Name] = true
	}

	// Delete orphan ze_* tables (present in kernel but not in desired state).
	for _, ct := range currentTables {
		if !strings.HasPrefix(ct.Name, zeTablePrefix) {
			continue // not ours
		}
		if desiredNames[ct.Name] {
			continue // still desired
		}
		b.conn.DelTable(ct)
	}

	// Create or replace desired tables.
	for i := range desired {
		if err := b.applyTable(&desired[i]); err != nil {
			return fmt.Errorf("firewallnft: table %q: %w", desired[i].Name, err)
		}
	}

	// Commit all changes atomically.
	if err := b.conn.Flush(); err != nil {
		return fmt.Errorf("firewallnft: flush: %w", err)
	}
	return nil
}

func (b *backend) applyTable(tbl *firewall.Table) error {
	family := lowerFamily(tbl.Family)
	t := b.conn.AddTable(&nftables.Table{
		Name:   tbl.Name,
		Family: family,
	})

	for i := range tbl.Chains {
		if err := b.applyChain(t, &tbl.Chains[i]); err != nil {
			return fmt.Errorf("chain %q: %w", tbl.Chains[i].Name, err)
		}
	}

	for i := range tbl.Sets {
		if err := b.applySet(t, &tbl.Sets[i]); err != nil {
			return fmt.Errorf("set %q: %w", tbl.Sets[i].Name, err)
		}
	}

	for i := range tbl.Flowtables {
		b.applyFlowtable(t, &tbl.Flowtables[i])
	}

	return nil
}

func (b *backend) applyChain(t *nftables.Table, chain *firewall.Chain) error {
	c := &nftables.Chain{
		Name:  chain.Name,
		Table: t,
	}
	if chain.IsBase {
		hooknum := lowerHook(chain.Hook)
		policy := lowerPolicy(chain.Policy)
		c.Type = lowerChainType(chain.Type)
		c.Hooknum = hooknum
		c.Priority = nftables.ChainPriorityRef(nftables.ChainPriority(chain.Priority))
		c.Policy = &policy
	}
	b.conn.AddChain(c)

	for i := range chain.Terms {
		exprs, err := lowerTerm(&chain.Terms[i])
		if err != nil {
			return fmt.Errorf("term %q: %w", chain.Terms[i].Name, err)
		}
		// Ensure the rule carries at least one counter expression so
		// `show firewall ruleset` can always report per-rule packet/
		// byte counts. If the operator already declared an explicit
		// `counter` action (lowered to an expr.Counter in exprs), use
		// theirs -- prepending a second one would give us two Counter
		// exprs on the wire and readRuleCounter would silently pick
		// whichever comes first, making any named/explicit counter
		// inaccessible.
		var allExprs []expr.Any
		if hasCounterExpr(exprs) {
			allExprs = exprs
		} else {
			allExprs = make([]expr.Any, 0, len(exprs)+1)
			allExprs = append(allExprs, &expr.Counter{})
			allExprs = append(allExprs, exprs...)
		}
		b.conn.AddRule(&nftables.Rule{
			Table:    t,
			Chain:    c,
			Exprs:    allExprs,
			UserData: []byte(chain.Terms[i].Name),
		})
	}
	return nil
}

// hasCounterExpr reports whether the lowered expression list already
// contains a counter expression -- avoids double-Counter rules when the
// operator declared `counter` explicitly in the term's `then` block.
func hasCounterExpr(exprs []expr.Any) bool {
	for _, e := range exprs {
		if _, ok := e.(*expr.Counter); ok {
			return true
		}
	}
	return false
}

func (b *backend) applySet(t *nftables.Table, s *firewall.Set) error {
	nftSet := &nftables.Set{
		Name:     s.Name,
		Table:    t,
		KeyType:  lowerSetType(s.Type),
		Interval: s.Flags&firewall.SetFlagInterval != 0,
	}
	var elements []nftables.SetElement
	for _, e := range s.Elements {
		key, err := encodeSetElementKey(s.Type, e.Value)
		if err != nil {
			return fmt.Errorf("element %q: %w", e.Value, err)
		}
		elements = append(elements, nftables.SetElement{Key: key})
	}
	if err := b.conn.AddSet(nftSet, elements); err != nil {
		return fmt.Errorf("add set: %w", err)
	}
	return nil
}

func (b *backend) applyFlowtable(t *nftables.Table, ft *firewall.Flowtable) {
	hooknum := lowerFlowtableHook(ft.Hook)
	b.conn.AddFlowtable(&nftables.Flowtable{
		Table:    t,
		Name:     ft.Name,
		Hooknum:  hooknum,
		Priority: nftables.FlowtablePriorityRef(nftables.FlowtablePriority(ft.Priority)),
		Devices:  ft.Devices,
	})
}

// ListTables returns current ze_* tables from the kernel.
func (b *backend) ListTables() ([]firewall.Table, error) {
	tables, err := b.conn.ListTables()
	if err != nil {
		return nil, fmt.Errorf("firewallnft: list tables: %w", err)
	}

	var result []firewall.Table
	for _, t := range tables {
		if !strings.HasPrefix(t.Name, zeTablePrefix) {
			continue
		}
		result = append(result, firewall.Table{
			Name:   t.Name,
			Family: raiseFamily(t.Family),
		})
	}
	return result, nil
}

// GetCounters returns per-term packet/byte counter values for a table.
// Each rule carries its term name in UserData (set by applyChain) and
// an anonymous counter expression as its first Expr (also set by
// applyChain). readRuleCounters decodes both; rules lacking either
// (e.g. inserted out-of-band) surface with empty Name and zeroes.
func (b *backend) GetCounters(tableName string) ([]firewall.ChainCounters, error) {
	tables, err := b.conn.ListTables()
	if err != nil {
		return nil, fmt.Errorf("firewallnft: list tables: %w", err)
	}

	var target *nftables.Table
	for _, t := range tables {
		if t.Name == tableName {
			target = t
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("firewallnft: table %q not found", tableName)
	}

	chains, err := b.conn.ListChainsOfTableFamily(target.Family)
	if err != nil {
		return nil, fmt.Errorf("firewallnft: list chains: %w", err)
	}

	var result []firewall.ChainCounters
	for _, c := range chains {
		if c.Table.Name != tableName {
			continue
		}
		rules, err := b.conn.GetRules(target, c)
		if err != nil {
			return nil, fmt.Errorf("firewallnft: get rules for chain %q: %w", c.Name, err)
		}
		cc := firewall.ChainCounters{Chain: c.Name}
		for _, r := range rules {
			cc.Terms = append(cc.Terms, readRuleCounter(r))
		}
		result = append(result, cc)
	}
	return result, nil
}

// readRuleCounter extracts the term name (from Rule.UserData) and the
// first Counter expression's packet/byte values. Rules programmed
// outside ze (no UserData, no Counter) return a zero-valued TermCounter
// with an empty Name; the caller still sees them so the list of rules
// and the list of terms stay aligned by index.
func readRuleCounter(r *nftables.Rule) firewall.TermCounter {
	tc := firewall.TermCounter{Name: string(r.UserData)}
	for _, e := range r.Exprs {
		ctr, ok := e.(*expr.Counter)
		if !ok {
			continue
		}
		tc.Packets = ctr.Packets
		tc.Bytes = ctr.Bytes
		break
	}
	return tc
}

// Close releases resources. nftables.Conn has no explicit close.
func (b *backend) Close() error {
	return nil
}
