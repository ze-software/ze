// Design: docs/architecture/core-design.md -- nftables backend Linux implementation

//go:build linux

package firewallnft

import (
	"fmt"
	"strings"

	"github.com/google/nftables"

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
		b.conn.AddRule(&nftables.Rule{
			Table: t,
			Chain: c,
			Exprs: exprs,
		})
	}
	return nil
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

// GetCounters returns per-term counter values for a table.
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
		cc := firewall.ChainCounters{Chain: c.Name}
		result = append(result, cc)
	}
	return result, nil
}

// Close releases resources. nftables.Conn has no explicit close.
func (b *backend) Close() error {
	return nil
}
