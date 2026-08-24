// Design: docs/architecture/core-design.md -- nftables backend Linux implementation
// Related: readback_linux.go -- ListTables' kernel read-back path
// Related: lower_linux.go -- forward lowering helpers

//go:build linux

package firewallnft

import (
	"fmt"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"

	"github.com/ze-software/ze/internal/component/firewall"
)

// zeTablePrefix is the ownership prefix for ze-managed tables in the kernel.
const zeTablePrefix = "ze_"

// backend implements firewall.Backend using google/nftables.
type backend struct {
	conn    *nftables.Conn
	applied map[string]struct{}
}

func newBackend() (firewall.Backend, error) {
	// Every netlink round-trip is bounded: firewall.ApplyAll holds the
	// process-wide reconcileMu across Apply, so an unbounded call would let a
	// wedged kernel stall every firewall owner. See withNetlinkDeadline.
	conn, err := nftables.New(withNetlinkDeadline(netlinkTimeout()))
	if err != nil {
		return nil, fmt.Errorf("firewallnft: open netlink: %w", err)
	}
	return &backend{conn: conn, applied: make(map[string]struct{})}, nil
}

// Apply receives the full desired state and reconciles against the kernel.
// Creates new ze_* tables, replaces changed ones, deletes orphans, and, on the
// FIRST reconcile of the process only, deletes the tables an earlier ze build
// wrote without the ownership prefix.
// All operations are atomic via Flush().
func (b *backend) Apply(desired []firewall.Table) error {
	// R-2 host-safety gate: under the functional-test netns launch mode, refuse
	// to touch the kernel firewall unless this process is provably in an isolated
	// namespace. No-op in production (the env is test-only). Runs before any
	// kernel op so a refused apply never stages, let alone flushes, to the host.
	if err := refuseHostNetnsFirewall(); err != nil {
		return err
	}

	desiredNames := tableNameSet(desired)

	// The removal of what an older ze build left in the kernel is a migration
	// with an end, not a standing deletion policy: it runs on the first
	// reconcile of the process and on no later one. Read once, before the loop,
	// so every table in one reconcile gets the same answer
	// (firewall.LegacySweepPending, cleared by ApplyAll once this returns).
	sweepLegacy := firewall.LegacySweepPending()

	// List current ze_* tables.
	currentTables, err := b.conn.ListTables()
	if err != nil {
		return fmt.Errorf("firewallnft: list tables: %w", asKernelTimeout(err))
	}

	// Delete only tables this Apply owns: current desired names and tables
	// applied by this backend instance earlier. Unknown ze_* tables may belong
	// to another Ze producer or process and must not be swept by prefix alone.
	//
	// This loop is also where a withdrawn route stops being enforced. RFC 8955
	// Section 4 carries a flow specification in MP_REACH_NLRI and
	// MP_UNREACH_NLRI, so it is an ordinary BGP route, and RFC 4271 Section 9
	// says a withdrawn route "SHALL be removed from the Adj-RIB-In ... the
	// previously advertised route is no longer available for use". The withdraw
	// leaves the owner's table out of desiredNames, and deleting it here is what
	// takes the rules out of the kernel. RFC 8955 states no withdrawal rule of
	// its own: it never uses the word.
	for _, ct := range currentTables {
		if sweepLegacy && b.isLegacyTable(ct) {
			// An earlier ze build wrote this table without the ownership prefix,
			// so no sweep could ever reach it and an upgrade would leave its
			// rules enforcing for the life of the box. Delete it once.
			firewall.Logger().Info("firewallnft: deleting a table an earlier ze build left without the ownership prefix",
				"table", ct.Name)
			b.conn.DelTable(ct)
			continue
		}
		if b.shouldDeleteTable(ct, desiredNames) {
			b.conn.DelTable(ct)
		}
	}

	// Create or replace desired tables.
	for i := range desired {
		if err := b.applyTable(&desired[i]); err != nil {
			return fmt.Errorf("firewallnft: table %q: %w", desired[i].Name, err)
		}
	}

	// Commit all changes atomically.
	if err := b.conn.Flush(); err != nil {
		return fmt.Errorf("firewallnft: flush: %w", asKernelTimeout(err))
	}
	b.applied = desiredNames
	return nil
}

// isLegacyTable reports whether a kernel table is one an earlier ze build wrote
// under a name carrying no ownership prefix, so no sweep could reach it. The
// caller decides WHETHER to ask: Apply asks on the first reconcile of the
// process only, which is what keeps this a migration rather than a policy.
//
// The name and the family are only the pre-filter. The decision needs the
// table's CHAINS as well, because these names are ordinary words that another
// tool programming nftables can use: deleting that tool's table would be a
// worse failure than the stale rule this removal exists to clear
// (firewall.IsLegacyTable). Reading the chains costs a netlink round trip, so
// IsLegacyTableName runs first and keeps that cost off every other table.
//
// A kernel that cannot be read here answers no. The table then survives, which
// is the safe direction: a missed removal leaves a stale rule, and a wrong one
// destroys somebody else's.
func (b *backend) isLegacyTable(t *nftables.Table) bool {
	if t == nil {
		return false
	}
	family, err := raiseFamily(t.Family)
	if err != nil || !firewall.IsLegacyTableName(t.Name, family) {
		return false
	}
	chains, err := b.conn.ListChainsOfTableFamily(t.Family)
	if err != nil {
		return false
	}
	names := make([]string, 0, len(chains))
	for _, kc := range chains {
		if kc.Table != nil && kc.Table.Name == t.Name {
			names = append(names, kc.Name)
		}
	}
	return firewall.IsLegacyTable(t.Name, family, names)
}

func (b *backend) shouldDeleteTable(t *nftables.Table, desiredNames map[string]struct{}) bool {
	if t == nil || !strings.HasPrefix(t.Name, zeTablePrefix) {
		return false
	}
	if _, ok := desiredNames[t.Name]; ok {
		return true
	}
	_, ok := b.applied[t.Name]
	return ok
}

func tableNameSet(tables []firewall.Table) map[string]struct{} {
	result := make(map[string]struct{}, len(tables))
	for i := range tables {
		result[tables[i].Name] = struct{}{}
	}
	return result
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
		c.Priority = new(nftables.ChainPriority(chain.Priority))
		c.Policy = &policy
	}
	b.conn.AddChain(c)

	ctx := &lowerCtx{conn: b.conn, table: t, sets: sets}
	for i := range chain.Terms {
		// A term is usually one rule, and is two in an inet table when an
		// action rewrites a network-header field: lowerTerm carries why.
		// Every rule of a term takes the term's name, which is what lets
		// mergeRuleCounters put the two rules' packets back under the one
		// term the operator wrote.
		rules, err := lowerTerm(ctx, &chain.Terms[i])
		if err != nil {
			return fmt.Errorf("term %q: %w", chain.Terms[i].Name, err)
		}
		for _, exprs := range rules {
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
	nftSet, elements, err := lowerSet(t, s)
	if err != nil {
		return nil, err
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
		Priority: new(nftables.FlowtablePriority(ft.Priority)),
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
// applyChain). readRuleCounter decodes both; rules lacking either
// (e.g. inserted out-of-band) surface with empty Name and zeroes.
//
// One term can hold more than one rule, so the counters of the rules
// sharing a term name are summed into one entry: the result is one row per
// TERM, which is the unit the operator wrote and the unit every caller
// looks up by.
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
		result = append(result, firewall.ChainCounters{
			Chain: c.Name,
			Terms: mergeRuleCounters(rules),
		})
	}
	return result, nil
}

// mergeRuleCounters turns one chain's kernel rules into one TermCounter per
// TERM, in rule order.
//
// A term becomes more than one rule when an action needs an address family
// (lowerTerm, lower_linux.go), and every one of those rules carries the term's
// name. Their packets are summed back into the one term the operator wrote:
// every caller keys the result by name (handleShowFirewallRuleset in
// cmd_show.go, the web firewall page), so a second row for one term would
// replace the first and report one family's packets as the term's total.
//
// A rule carrying no name was not programmed by ze, so each one stays its own
// row and stays visible.
func mergeRuleCounters(rules []*nftables.Rule) []firewall.TermCounter {
	terms := make([]firewall.TermCounter, 0, len(rules))
	at := make(map[string]int, len(rules))
	for _, r := range rules {
		tc := readRuleCounter(r)
		if tc.Name == "" {
			terms = append(terms, tc)
			continue
		}
		first, seen := at[tc.Name]
		if !seen {
			at[tc.Name] = len(terms)
			terms = append(terms, tc)
			continue
		}
		terms[first].Packets += tc.Packets
		terms[first].Bytes += tc.Bytes
	}
	return terms
}

// readRuleCounter extracts the term name (from Rule.UserData) and the
// first Counter expression's packet/byte values, for ONE rule. Rules
// programmed outside ze (no UserData, no Counter) return a zero-valued
// TermCounter with an empty Name, and the caller still surfaces each one
// so a rule ze did not write is visible rather than dropped.
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
