// Design: docs/architecture/core-design.md -- nftables backend Linux implementation
// Related: readback_linux.go -- ListTables' kernel read-back path
// Related: lower_linux.go -- forward lowering helpers

//go:build linux

package firewallnft

import (
	"fmt"
	"strings"
	"time"

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
	family, err := lowerFamily(tbl.Family)
	if err != nil {
		return err
	}
	t := b.conn.AddTable(&nftables.Table{
		Name:   tbl.Name,
		Family: family,
	})

	// Sets are applied BEFORE chains so that chain rules that reference
	// them via MatchInSet can resolve the set's ID/Name from the map
	// below when lowering expressions. google/nftables assigns each
	// *nftables.Set a monotonically increasing ID inside AddSet; by the
	// time applyChain runs its Lookup expressions, the IDs are stable.
	sets := make(map[string]*nftables.Set, len(tbl.Sets))
	for i := range tbl.Sets {
		ns, err := b.applySet(t, &tbl.Sets[i])
		if err != nil {
			return fmt.Errorf("set %q: %w", tbl.Sets[i].Name, err)
		}
		sets[tbl.Sets[i].Name] = ns
	}

	for i := range tbl.Chains {
		if err := b.applyChain(t, sets, &tbl.Chains[i]); err != nil {
			return fmt.Errorf("chain %q: %w", tbl.Chains[i].Name, err)
		}
	}

	for i := range tbl.Flowtables {
		if err := b.applyFlowtable(t, &tbl.Flowtables[i]); err != nil {
			return fmt.Errorf("flowtable %q: %w", tbl.Flowtables[i].Name, err)
		}
	}

	return nil
}

func (b *backend) applyChain(t *nftables.Table, sets map[string]*nftables.Set, chain *firewall.Chain) error {
	c := &nftables.Chain{
		Name:  chain.Name,
		Table: t,
	}
	if chain.IsBase {
		hooknum, err := lowerHook(chain.Hook)
		if err != nil {
			return err
		}
		policy, err := lowerPolicy(chain.Policy)
		if err != nil {
			return err
		}
		chainType, err := lowerChainType(chain.Type)
		if err != nil {
			return err
		}
		c.Type = chainType
		c.Hooknum = hooknum
		c.Priority = nftables.ChainPriorityRef(nftables.ChainPriority(chain.Priority))
		c.Policy = &policy
	}
	b.conn.AddChain(c)

	ctx := &lowerCtx{conn: b.conn, table: t, sets: sets}
	for i := range chain.Terms {
		exprs, err := lowerTerm(ctx, &chain.Terms[i])
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

// applySet registers the set on the nftables connection and returns the
// *nftables.Set so applyTable can expose it to applyChain via the sets
// map. The returned pointer carries the kernel-assigned ID (allocated
// inside conn.AddSet) that expr.Lookup needs.
func (b *backend) applySet(t *nftables.Table, s *firewall.Set) (*nftables.Set, error) {
	keyType, err := lowerSetType(s.Type)
	if err != nil {
		return nil, err
	}
	nftSet := &nftables.Set{
		Name:     s.Name,
		Table:    t,
		KeyType:  keyType,
		Interval: s.Flags&firewall.SetFlagInterval != 0,
	}
	var elements []nftables.SetElement
	for _, e := range s.Elements {
		key, err := encodeSetElementKey(s.Type, e.Value)
		if err != nil {
			return nil, fmt.Errorf("element %q: %w", e.Value, err)
		}
		el := nftables.SetElement{Key: key}
		// Per-element timeout reaches the kernel as time.Duration.
		// Zero stays zero (no timeout) so unset elements keep the
		// prior behaviour. The set itself must carry flags-timeout
		// for the kernel to honour any per-element timeout; that
		// flag is applied at set construction above via the Flags
		// field on the parent firewall.Set.
		if e.Timeout != 0 {
			el.Timeout = time.Duration(e.Timeout) * time.Second
		}
		elements = append(elements, el)
	}
	if err := b.conn.AddSet(nftSet, elements); err != nil {
		return nil, fmt.Errorf("add set: %w", err)
	}
	return nftSet, nil
}

func (b *backend) applyFlowtable(t *nftables.Table, ft *firewall.Flowtable) error {
	hooknum, err := lowerFlowtableHook(ft.Hook)
	if err != nil {
		return err
	}
	b.conn.AddFlowtable(&nftables.Flowtable{
		Table:    t,
		Name:     ft.Name,
		Hooknum:  hooknum,
		Priority: nftables.FlowtablePriorityRef(nftables.FlowtablePriority(ft.Priority)),
		Devices:  ft.Devices,
	})
	return nil
}

// ListTables returns current ze_* tables from the kernel, each populated
// with its chains (and per-chain term names), sets (including elements),
// and flowtables. Term matches/actions are intentionally left empty:
// the forward lowering is not bijective, so faithfully reversing it is
// not possible without extra metadata beyond what nftables stores.
// Operators who need the full rule body consult config.
func (b *backend) ListTables() ([]firewall.Table, error) {
	return b.readTables()
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
