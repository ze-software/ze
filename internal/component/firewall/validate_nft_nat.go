// Design: docs/guide/firewall.md -- nftables NAT placement at configuration verify

package firewall

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

type nftNATHooks uint16

// Linux v7.3 nft_nat_validate, nft_masq_validate and nft_redir_validate
// require "nft_chain_validate_dependency(ctx->chain, NFT_CHAIN_T_NAT)".
// Their hook masks are POST_ROUTING | LOCAL_IN for SNAT, POST_ROUTING for
// masquerade, and PRE_ROUTING | LOCAL_OUT for DNAT and redirect. These bits
// use Ze's ChainHook ordinal, independent of Linux's NF_INET values.
const (
	nftSNATHooks         nftNATHooks = 1<<HookPostrouting | 1<<HookInput
	nftMasqueradeHooks   nftNATHooks = 1 << HookPostrouting
	nftDNATHooks         nftNATHooks = 1<<HookPrerouting | 1<<HookOutput
	nftNATBaseHooks                  = nftSNATHooks | nftDNATHooks
	nftConntrackPriority             = -200 // NF_IP_PRI_CONNTRACK
)

func (hooks nftNATHooks) allows(hook ChainHook) bool {
	return hooks&(1<<hook) != 0
}

func (hooks nftNATHooks) String() string {
	var names []string
	for hook := HookInput; hook <= HookEgress; hook++ {
		if hooks.allows(hook) {
			names = append(names, hook.String())
		}
	}
	return strings.Join(names, " or ")
}

func isNftNATFamily(family TableFamily) bool {
	return family == FamilyIP || family == FamilyIP6 || family == FamilyInet
}

// Linux v7.3 registers type nat only for IPv4, IPv6 and inet at the four
// NAT hooks. nft_chain_nat_validate refuses "hook->priority <= NF_IP_PRI_CONNTRACK".
func validateNftNATBaseChain(tbl *Table, ch *Chain) error {
	table := strings.TrimPrefix(tbl.Name, tableNamePrefix)
	location := fmt.Sprintf("Firewall table %q chain %q", table, ch.Name)
	editChain := fmt.Sprintf("edit firewall table %s chain %s", table, ch.Name)
	if !isNftNATFamily(tbl.Family) {
		return opererr.New(location+" cannot use type nat",
			fmt.Sprintf("Family %s does not provide nat chains. Use a table of family ip, ip6 or inet.", tbl.Family),
			"edit firewall table "+table)
	}
	if !nftNATBaseHooks.allows(ch.Hook) {
		return opererr.New(location+" cannot use type nat",
			fmt.Sprintf("Hook %s does not provide nat chains. Choose %s.", ch.Hook, nftNATBaseHooks),
			editChain)
	}
	if ch.Priority <= nftConntrackPriority {
		return opererr.New(location+" cannot use type nat",
			fmt.Sprintf("Priority %d runs at or before connection tracking. A nat chain needs a priority greater than %d.", ch.Priority, nftConntrackPriority),
			fmt.Sprintf("set firewall table %s chain %s priority %d", table, ch.Name, nftConntrackPriority+1))
	}
	return nil
}

func validateNftNATAction(tbl *Table, ch *Chain, term *Term, action Action) error {
	switch value := action.(type) {
	case SNAT:
		return validateNftNATPlacement(tbl, ch, term, "snat", value.Address, nftSNATHooks)
	case DNAT:
		return validateNftNATPlacement(tbl, ch, term, "dnat", value.Address, nftDNATHooks)
	case Masquerade:
		return validateNftNATPlacement(tbl, ch, term, "masquerade", netip.Addr{}, nftMasqueradeHooks)
	case Redirect:
		return validateNftNATPlacement(tbl, ch, term, "redirect", netip.Addr{}, nftDNATHooks)
	}
	return nil
}

func validateNftNATPlacement(tbl *Table, ch *Chain, term *Term, action string, target netip.Addr, hooks nftNATHooks) error {
	table := strings.TrimPrefix(tbl.Name, tableNamePrefix)
	problem := fmt.Sprintf("Firewall table %q chain %q term %q cannot use %s", table, ch.Name, term.Name, action)
	editTerm := fmt.Sprintf("edit firewall table %s chain %s term %s", table, ch.Name, term.Name)
	if !isNftNATFamily(tbl.Family) {
		return opererr.New(problem,
			fmt.Sprintf("Family %s does not provide NAT actions. Move the term to a table of family ip, ip6 or inet.", tbl.Family), editTerm)
	}
	// nft_nat_init: "if (ctx->family != NFPROTO_INET && ctx->family != family)".
	// nftables lowerNAT chooses the target family with Addr.Is4().
	if target.IsValid() && tbl.Family != FamilyInet && target.Is4() != (tbl.Family == FamilyIP) {
		return opererr.New(problem,
			fmt.Sprintf("Target %s does not match table family %s. Choose a target of that family or move the term to an inet table.", target, tbl.Family), editTerm)
	}
	if base := nftNATIncompatibleBase(tbl, ch, hooks); base != nil {
		return opererr.New(problem,
			fmt.Sprintf("Base chain %q has type %s at hook %s. Place %s where every reaching base chain has type nat and hook %s.", base.Name, base.Type, base.Hook, action, hooks), editTerm)
	}
	return nil
}

// nft_table_validate walks base chains, and nft_chain_validate retains the
// original base as ctx->chain when it validates jump and goto targets.
func nftNATIncompatibleBase(tbl *Table, ch *Chain, hooks nftNATHooks) *Chain {
	if ch.IsBase {
		if ch.Type != ChainNAT || !hooks.allows(ch.Hook) {
			return ch
		}
		return nil
	}
	byName := make(map[string]*Chain, len(tbl.Chains))
	for i := range tbl.Chains {
		byName[tbl.Chains[i].Name] = &tbl.Chains[i]
	}
	for i := range tbl.Chains {
		base := &tbl.Chains[i]
		if !base.IsBase || base.Type == ChainNAT && hooks.allows(base.Hook) {
			continue
		}
		if chainReaches(byName, base, ch.Name) {
			return base
		}
	}
	return nil
}
