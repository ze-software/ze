// Design: docs/architecture/core-design.md -- nftables read-back of ze_* tables
// Related: backend_linux.go -- ListTables entry point that drives this file
// Related: lower_linux.go -- forward path; raise* helpers are the inverses here

//go:build linux

package firewallnft

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/nftables"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// readTables walks every ze_* table in the kernel and reconstructs the
// firewall.Table tree (chains + terms + sets + flowtables) so CLI
// `show` commands see more than table headers. Term matches and
// actions are NOT reverse-lowered: the kernel-side expressions don't
// carry enough metadata to unambiguously name the original ze Match /
// Action, so we preserve the term name (read from Rule.UserData) and
// leave the body empty. Operators who need the body consult config.
func (b *backend) readTables() ([]firewall.Table, error) {
	kernelTables, err := b.conn.ListTables()
	if err != nil {
		return nil, fmt.Errorf("firewallnft: list tables: %w", err)
	}

	var result []firewall.Table
	for _, kt := range kernelTables {
		if !strings.HasPrefix(kt.Name, zeTablePrefix) {
			continue
		}
		family, err := raiseFamily(kt.Family)
		if err != nil {
			return nil, fmt.Errorf("firewallnft: table %q: %w", kt.Name, err)
		}
		t := firewall.Table{
			Name:   kt.Name,
			Family: family,
		}
		if err := b.fillChains(kt, &t); err != nil {
			return nil, err
		}
		if err := b.fillSets(kt, &t); err != nil {
			return nil, err
		}
		if err := b.fillFlowtables(kt, &t); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

func (b *backend) fillChains(kt *nftables.Table, t *firewall.Table) error {
	chains, err := b.conn.ListChainsOfTableFamily(kt.Family)
	if err != nil {
		return fmt.Errorf("firewallnft: list chains: %w", err)
	}
	for _, kc := range chains {
		if kc.Table == nil || kc.Table.Name != kt.Name {
			continue
		}
		c := firewall.Chain{Name: kc.Name}
		if kc.Hooknum != nil && kc.Priority != nil {
			c.IsBase = true
			if hook, ok := raiseHook(kc.Hooknum, kt.Family); ok {
				c.Hook = hook
			}
			if pol, ok := raisePolicy(kc.Policy); ok {
				c.Policy = pol
			}
			if ct, ok := raiseChainType(kc.Type); ok {
				c.Type = ct
			}
			c.Priority = int32(*kc.Priority)
		}
		rules, err := b.conn.GetRules(kt, kc)
		if err != nil {
			return fmt.Errorf("firewallnft: get rules for chain %q: %w", kc.Name, err)
		}
		for _, r := range rules {
			c.Terms = append(c.Terms, firewall.Term{Name: string(r.UserData)})
		}
		t.Chains = append(t.Chains, c)
	}
	return nil
}

func (b *backend) fillSets(kt *nftables.Table, t *firewall.Table) error {
	sets, err := b.conn.GetSets(kt)
	if err != nil {
		return fmt.Errorf("firewallnft: get sets: %w", err)
	}
	for _, ks := range sets {
		s := firewall.Set{Name: ks.Name}
		if typ, ok := raiseSetType(ks.KeyType); ok {
			s.Type = typ
		}
		if ks.Interval {
			s.Flags |= firewall.SetFlagInterval
		}
		if ks.HasTimeout {
			s.Flags |= firewall.SetFlagTimeout
		}
		if ks.Constant {
			s.Flags |= firewall.SetFlagConstant
		}
		if ks.Dynamic {
			s.Flags |= firewall.SetFlagDynamic
		}
		elems, err := b.conn.GetSetElements(ks)
		if err != nil {
			return fmt.Errorf("firewallnft: get elements for set %q: %w", ks.Name, err)
		}
		for i := range elems {
			el := &elems[i]
			// Kernel timeout is a time.Duration; truncate to whole
			// seconds for the ze model's uint32 field. Negative or
			// sub-second values cannot occur (kernel represents
			// timeouts in milliseconds and ze writes whole-second
			// values), so the truncation is lossless for anything
			// ze itself programmed.
			var timeout uint32
			if el.Timeout > 0 {
				timeout = uint32(el.Timeout / time.Second)
			}
			s.Elements = append(s.Elements, firewall.SetElement{
				Value:   decodeSetElementKey(s.Type, el.Key),
				Timeout: timeout,
			})
		}
		t.Sets = append(t.Sets, s)
	}
	return nil
}

func (b *backend) fillFlowtables(kt *nftables.Table, t *firewall.Table) error {
	fts, err := b.conn.ListFlowtables(kt)
	if err != nil {
		return fmt.Errorf("firewallnft: list flowtables: %w", err)
	}
	for _, kft := range fts {
		ft := firewall.Flowtable{
			Name:    kft.Name,
			Devices: append([]string(nil), kft.Devices...),
		}
		if kft.Hooknum != nil {
			if hook, ok := raiseFlowtableHook(kft.Hooknum); ok {
				ft.Hook = hook
			}
		}
		if kft.Priority != nil {
			ft.Priority = int32(*kft.Priority)
		}
		t.Flowtables = append(t.Flowtables, ft)
	}
	return nil
}

// raiseHook maps the kernel hook number back to the ze ChainHook enum.
// The hook number space overlaps between the INET and NETDEV families
// (e.g. NF_INET_LOCAL_IN and NF_NETDEV_INGRESS are both 0), so the
// family is needed to disambiguate. An unknown combination surfaces as
// HookUnspecified + false.
func raiseHook(h *nftables.ChainHook, family nftables.TableFamily) (firewall.ChainHook, bool) {
	if h == nil {
		return 0, false
	}
	num := uint32(*h)
	if family == nftables.TableFamilyNetdev {
		switch num {
		case unix.NF_NETDEV_INGRESS:
			return firewall.HookIngress, true
		case unix.NF_NETDEV_EGRESS:
			return firewall.HookEgress, true
		}
		return 0, false
	}
	switch num {
	case unix.NF_INET_LOCAL_IN:
		return firewall.HookInput, true
	case unix.NF_INET_LOCAL_OUT:
		return firewall.HookOutput, true
	case unix.NF_INET_FORWARD:
		return firewall.HookForward, true
	case unix.NF_INET_PRE_ROUTING:
		return firewall.HookPrerouting, true
	case unix.NF_INET_POST_ROUTING:
		return firewall.HookPostrouting, true
	}
	return 0, false
}

// raiseFlowtableHook is a separate function because flowtables only
// use the ingress hook value; any other number implies a future kernel
// extension we don't model yet.
func raiseFlowtableHook(h *nftables.FlowtableHook) (firewall.ChainHook, bool) {
	if h == nil {
		return 0, false
	}
	if *h == unix.NF_NETDEV_INGRESS {
		return firewall.HookIngress, true
	}
	return 0, false
}

func raisePolicy(p *nftables.ChainPolicy) (firewall.Policy, bool) {
	if p == nil {
		return 0, false
	}
	switch *p {
	case nftables.ChainPolicyAccept:
		return firewall.PolicyAccept, true
	case nftables.ChainPolicyDrop:
		return firewall.PolicyDrop, true
	}
	return 0, false
}

func raiseChainType(ct nftables.ChainType) (firewall.ChainType, bool) {
	switch ct {
	case nftables.ChainTypeFilter:
		return firewall.ChainFilter, true
	case nftables.ChainTypeNAT:
		return firewall.ChainNAT, true
	case nftables.ChainTypeRoute:
		return firewall.ChainRoute, true
	}
	return 0, false
}

// raiseSetType compares the kernel-reported SetDatatype name to the
// names baked into google/nftables' type table. A mismatch (e.g. a
// concatenated type we didn't emit) surfaces as false.
func raiseSetType(sd nftables.SetDatatype) (firewall.SetType, bool) {
	switch sd.Name {
	case nftables.TypeIPAddr.Name:
		return firewall.SetTypeIPv4, true
	case nftables.TypeIP6Addr.Name:
		return firewall.SetTypeIPv6, true
	case nftables.TypeEtherAddr.Name:
		return firewall.SetTypeEther, true
	case nftables.TypeInetService.Name:
		return firewall.SetTypeInetService, true
	case nftables.TypeMark.Name:
		return firewall.SetTypeMark, true
	case nftables.TypeIFName.Name:
		return firewall.SetTypeIfname, true
	}
	return 0, false
}
