// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Detail: espform_fastpath_gate_linux.go -- the gate this judgement opens or keeps shut
// Related: espform_program_linux.go -- the rewrite program the gate guards
// Related: espform_fastpath_linux.go -- the fast path's attach lifecycle
// Related: espform_firewall_kernel_linux.go -- the kernel reader, its lossless rule decoder, and the watch

//go:build linux

package dataplane

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"unicode"
	"unicode/utf8"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/userdata"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The fast path rewrites a peer's bare ESP at TCX ingress, ahead of every netfilter
// hook, so a rule that would have dropped, stolen or rewritten that ESP never sees
// it. espFormFirewallBlocks reads the KERNEL ruleset, Ze's tables and foreign ones
// alike, and answers whether any base chain bare IPv4 ESP crosses before local
// delivery could act on it. It reads the kernel because firewall.LastApplied is a
// package global: a forked IKE engine holds its own empty copy, which would read as
// "no rule".
//
// The walk is conservative by construction, and it models no register file: four
// rounds of review each found a register write the model forgot, and every one was
// a rule judged safe that could drop ESP. A rule is excluded only when its own first
// two expressions are a fresh load of the packet's protocol (or nfproto) and an
// equality compare of that register against a value IPv4 ESP never carries
// (espFormExcludesESP), so nothing an earlier expression left behind can matter.
// Every other rule can match ESP, and it is harmless only when every expression in
// it is on a short closed list that writes nothing (espFormHarmless).
//
// The receive order is (__netif_receive_skb_core, net/core/dev.c): skb_vlan_untag on
// a frame whose protocol is a VLAN type, then sch_handle_ingress, which runs TCX and
// so the program, then nf_ingress, the netdev and inet ingress hooks, and only then
// vlan_do_receive, which hands a tagged skb to its VLAN device for another round. So
// on a VLAN's parent link TCX and the netdev hooks see the same skb, its tag held
// out of band and its protocol the inner one. The program passes every tagged skb
// untouched (espFormFastPathInstructions), so a rule on the parent that matches the
// tag sees bare ESP, and a VLAN device's own TCX and hooks run on the next round. Any expression this file
// does not name is unsafe, so a new kernel expression blocks the fast path rather
// than slipping past it. Only base chains are read: a rule that can match ESP and
// sends it anywhere but accept or drop (jump, goto, return, a verdict map) is
// unsafe, and its target is never walked (espFormJudgeVerdict says why).

// Hook numbers golang.org/x/sys/unix does not carry. A hook number means nothing
// without its family: NF_NETDEV_EGRESS is 1, the same number as NF_INET_LOCAL_IN.
const (
	// espFormHookInetIngress is NF_INET_INGRESS, which the kernel defines as
	// NF_INET_NUMHOOKS (include/uapi/linux/netfilter.h).
	espFormHookInetIngress = 5
	// espFormHookBridgePrerouting and espFormHookBridgeInput are NF_BR_PRE_ROUTING
	// and NF_BR_LOCAL_IN (include/uapi/linux/netfilter_bridge.h).
	espFormHookBridgePrerouting = 0
	espFormHookBridgeInput      = 1
)

// The hook names a site reports, in nft's spelling.
const (
	espFormHookNameIngress    = "ingress"
	espFormHookNamePrerouting = "prerouting"
	espFormHookNameInput      = "input"
)

// The reason classes a site reports. Each names what the rule would do to the ESP
// the fast path rewrites before the rule can see it.
const (
	espFormReasonDrop       = "drop"
	espFormReasonReject     = "reject"
	espFormReasonQueue      = "queue"
	espFormReasonNAT        = "nat"
	espFormReasonForward    = "forward"
	espFormReasonTProxy     = "tproxy"
	espFormReasonSynProxy   = "synproxy"
	espFormReasonRewrite    = "packet rewrite"
	espFormReasonStateSet   = "meta or ct set"
	espFormReasonVerdictMap = "verdict map"
	espFormReasonMap        = "map lookup"
	espFormReasonSet        = "set lookup"
	espFormReasonSetUpdate  = "set update"
	espFormReasonDup        = "dup"
	espFormReasonObjref     = "object reference"
	espFormReasonUnlisted   = "expression off the harmless list"
	espFormReasonVerdict    = "verdict"
	espFormReasonJump       = "jump"
	espFormReasonGoto       = "goto"
	espFormReasonReturn     = "return"
	espFormReasonXT         = "xt compat"
	espFormReasonUnknown    = "unknown expression"
	espFormReasonPolicyDrop = "policy drop"
	espFormReasonPolicy     = "unknown policy"
)

// espFormNFTReader reads the kernel ruleset. Its dump methods are *nftables.Conn's, by
// name and signature, so a test fakes them; the production reader is
// espFormKernelReader, which dumps all three itself: it keeps the expressions
// Conn.GetRules drops and refuses what the library's readers let through.
// Generation answers the ruleset's generation (NFT_MSG_GETGEN), which every commit
// changes; espFormFirewallBlocks reads it before the first dump and after the last.
type espFormNFTReader interface {
	Generation() (uint32, error)
	ListTables() ([]*nftables.Table, error)
	ListChains() ([]*nftables.Chain, error)
	GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error)
}

// errESPFormRulesetChanged is the cause when the generation read after the last dump
// of a judgment differs from the one read before the first: a commit landed between
// two of its dumps, so the tables, chains and rules it read are no one ruleset. The
// gate does not publish such a judgment: it shuts and judges again (judgeFirewall).
var errESPFormRulesetChanged = errors.New("the ruleset changed while it was read (its nftables generation moved " +
	"between the first dump and the last), so what was read is not one ruleset")

// espFormFirewallSite is the first place in the ruleset that keeps the fast path
// off. Chain is the base chain that holds the rule: no other chain is read. Rule is
// "handle N" with the rule's comment quoted after it when it has one (Ze writes the
// term name there), or "policy drop".
type espFormFirewallSite struct {
	Family string
	Table  string
	Chain  string
	Hook   string
	Rule   string
	Reason string
}

// String reads as one line for a log or a doctor finding.
func (s espFormFirewallSite) String() string {
	var b textbuf.Buffer
	b.Str(s.Family).Str(" table ").Str(s.Table)
	b.Str(", chain ").Str(s.Chain)
	b.Str(", hook ").Str(s.Hook)
	b.Str(", ").Str(s.Rule).Str(": ").Str(s.Reason)
	return b.String()
}

// espFormFirewallBlocks answers whether any rule in the kernel ruleset could act on
// bare IPv4 ESP before local delivery, and names the first one. It walks every base
// chain of family inet, ip, netdev and bridge on hook ingress, prerouting or input,
// in the order the kernel lists them, and each chain's rules in the order the kernel
// returns them, which is the order it evaluates them. A rule's handle says nothing
// about its place: `nft insert` puts a rule with a new, higher handle at the head.
//
// A ruleset it cannot read is an error, never a verdict: the caller keeps the fast
// path off and names the cause.
//
// One judgment is several dumps (the tables, the chains, then the rules of each base
// chain), and NLM_F_DUMP_INTR flags a commit inside one dump only. A transaction that
// moves `meta l4proto esp drop` from table b to table a between the dump of a's rules
// and the dump of b's reads as no drop at all. So the walk is bracketed by two reads
// of the ruleset's generation, and its verdict, blocked or not, stands only when both
// read the same one: every commit bumps it (nf_tables_commit,
// net/netfilter/nf_tables_api.c, v7.0: "Bump generation counter, invalidate any dump
// in progress." then "while (++base_seq == 0)" and
// "smp_store_release(&net->nft.base_seq, base_seq);"), and GETGEN answers that
// counter (nf_tables_fill_gen_info: "nla_put_be32(skb, NFTA_GEN_ID,
// htonl(nft_base_seq(net)))"). A batch that changes nothing returns before the bump,
// and changes nothing a dump reads. Two reads that differ answer
// errESPFormRulesetChanged.
//
// The bracket does not refuse every splice. The commit stores the new number before
// it flips the generation cursor that decides which rules are active, and before its
// commit loop applies table and chain updates, chain policy included, while GETGEN is
// answered under RCU, not under the commit mutex. A first read can therefore answer
// the new number while the dumps still read the old state, and the two reads agree.
// The NEWGEN the watch signals once the commit applied closes that case: it shuts the
// gate and judges again (judgeFirewall, which quotes the kernel's order), so the
// splice is open for at most the watch's latency.
func espFormFirewallBlocks(r espFormNFTReader) (blocked bool, site espFormFirewallSite, err error) {
	before, err := r.Generation()
	if err != nil {
		return false, espFormFirewallSite{}, fmt.Errorf("read the nftables generation before the dumps: %w", err)
	}
	blocked, site, err = espFormFirewallWalk(r)
	if err != nil {
		return false, espFormFirewallSite{}, err
	}
	after, err := r.Generation()
	if err != nil {
		return false, espFormFirewallSite{}, fmt.Errorf("read the nftables generation after the dumps: %w", err)
	}
	if after != before {
		return false, espFormFirewallSite{}, errESPFormRulesetChanged
	}
	return blocked, site, nil
}

// espFormFirewallWalk is espFormFirewallBlocks's walk: the dumps, and the verdict over
// what they read, which only the generation bracket makes one ruleset.
func espFormFirewallWalk(r espFormNFTReader) (blocked bool, site espFormFirewallSite, err error) {
	tables, err := r.ListTables()
	if err != nil {
		return false, espFormFirewallSite{}, fmt.Errorf("list the nftables tables: %w", err)
	}
	chains, err := r.ListChains()
	if err != nil {
		return false, espFormFirewallSite{}, fmt.Errorf("list the nftables chains: %w", err)
	}

	bases, err := espFormBaseChains(tables, chains)
	if err != nil {
		return false, espFormFirewallSite{}, err
	}
	for _, base := range bases {
		found, unsafe, err := espFormBaseBlocks(r, base)
		if err != nil {
			return false, espFormFirewallSite{}, err
		}
		if unsafe {
			return true, found, nil
		}
	}
	return false, espFormFirewallSite{}, nil
}

// espFormTableKey names a table the way the kernel does: by family and name.
type espFormTableKey struct {
	family nftables.TableFamily
	name   string
}

// espFormBase is one base chain bare IPv4 ESP crosses before local delivery.
type espFormBase struct {
	chain  *nftables.Chain
	family string
	hook   string
}

// espFormChainTable answers the table a chain belongs to. Every kernel chain message
// names its table (espFormChainFromMessage refuses one that does not), so a chain
// with none is a read that went wrong: an error, never a chain skipped, because a
// skipped base chain is a rule the judgement never saw.
func espFormChainTable(c *nftables.Chain) (espFormTableKey, error) {
	if c == nil {
		return espFormTableKey{}, errESPFormNilChain
	}
	if c.Table == nil {
		return espFormTableKey{}, fmt.Errorf("the chain dump answered chain %s with no table", c.Name)
	}
	return espFormTableKey{family: c.Table.Family, name: c.Table.Name}, nil
}

// errESPFormNilChain and errESPFormNilTable are the causes when a reader answered a
// nil entry in place of a chain or a table.
var (
	errESPFormNilChain = errors.New("the chain dump answered no chain in one entry")
	errESPFormNilTable = errors.New("the table dump answered no table in one entry")
)

// espFormBaseChains lists the base chains to walk, grouped by table in the order
// ListTables answered. A chain whose table that dump did not list belongs to a table
// created between the two dumps; it is walked too, after the listed ones. A nil
// table, a nil chain or a chain with no table is an error.
func espFormBaseChains(tables []*nftables.Table, chains []*nftables.Chain) ([]espFormBase, error) {
	var order []espFormTableKey
	seen := map[espFormTableKey]bool{}
	for _, t := range tables {
		if t == nil {
			return nil, errESPFormNilTable
		}
		key := espFormTableKey{family: t.Family, name: t.Name}
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
		}
	}
	for _, c := range chains {
		key, err := espFormChainTable(c)
		if err != nil {
			return nil, err
		}
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
		}
	}

	var bases []espFormBase
	for _, key := range order {
		family, ok := espFormFamilyName(key.family)
		if !ok {
			continue
		}
		for _, c := range chains {
			if base, ok := espFormBaseOf(c, key, family); ok {
				bases = append(bases, base)
			}
		}
	}
	return bases, nil
}

// espFormBaseOf answers c as a base chain of table key on a hook bare ESP crosses.
// espFormBaseChains has already refused every chain without a table.
func espFormBaseOf(c *nftables.Chain, key espFormTableKey, family string) (espFormBase, bool) {
	table := espFormTableKey{family: c.Table.Family, name: c.Table.Name}
	if table != key {
		return espFormBase{}, false
	}
	if c.Hooknum == nil {
		return espFormBase{}, false // a regular chain: reached only by a jump
	}
	hook, ok := espFormHookName(key.family, *c.Hooknum)
	if !ok {
		return espFormBase{}, false
	}
	return espFormBase{chain: c, family: family, hook: hook}, true
}

// espFormFamilyName names the families whose base chains bare IPv4 ESP crosses
// before local delivery, in nft's spelling, and refuses every other.
func espFormFamilyName(family nftables.TableFamily) (string, bool) {
	// Only the families bare IPv4 ESP crosses are named: ip6 sees only IPv6, arp
	// sees only ARP, and an unspecified family holds no table.
	//exhaustive:ignore
	switch family {
	case nftables.TableFamilyINet:
		return "inet", true
	case nftables.TableFamilyIPv4:
		return "ip", true
	case nftables.TableFamilyNetdev:
		return "netdev", true
	case nftables.TableFamilyBridge:
		return "bridge", true
	}
	return "", false
}

// espFormHookName names the hooks bare ESP crosses before local delivery, per
// family, and refuses forward, output, postrouting and egress. The receive order is
// TCX ingress (where the fast path rewrites), netdev and inet ingress, the bridge,
// then prerouting and input.
func espFormHookName(family nftables.TableFamily, hook nftables.ChainHook) (string, bool) {
	switch {
	case family == nftables.TableFamilyINet && hook == unix.NF_INET_PRE_ROUTING:
		return espFormHookNamePrerouting, true
	case family == nftables.TableFamilyINet && hook == unix.NF_INET_LOCAL_IN:
		return espFormHookNameInput, true
	case family == nftables.TableFamilyINet && hook == espFormHookInetIngress:
		return espFormHookNameIngress, true
	case family == nftables.TableFamilyIPv4 && hook == unix.NF_INET_PRE_ROUTING:
		return espFormHookNamePrerouting, true
	case family == nftables.TableFamilyIPv4 && hook == unix.NF_INET_LOCAL_IN:
		return espFormHookNameInput, true
	case family == nftables.TableFamilyNetdev && hook == unix.NF_NETDEV_INGRESS:
		return espFormHookNameIngress, true
	case family == nftables.TableFamilyBridge && hook == espFormHookBridgePrerouting:
		return espFormHookNamePrerouting, true
	case family == nftables.TableFamilyBridge && hook == espFormHookBridgeInput:
		return espFormHookNameInput, true
	}
	return "", false
}

// espFormBaseBlocks reads one base chain's rules once, in the order the kernel
// returns them, then judges its policy if ESP can reach its end. No other chain
// is read: a rule that could send ESP to one is unsafe on its own
// (espFormJudgeVerdict).
func espFormBaseBlocks(r espFormNFTReader, b espFormBase) (espFormFirewallSite, bool, error) {
	site := espFormFirewallSite{Family: b.family, Table: b.chain.Table.Name, Chain: b.chain.Name, Hook: b.hook}
	rules, err := r.GetRules(b.chain.Table, b.chain)
	if err != nil {
		return espFormFirewallSite{}, false, fmt.Errorf(
			"list the rules of chain %s in %s table %s: %w", b.chain.Name, b.family, b.chain.Table.Name, err)
	}
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		j := espFormJudgeRule(b.chain.Table.Family, rule.Exprs)
		switch j.kind {
		case espFormRuleExcluded, espFormRulePasses:
			continue
		case espFormRuleAccepts:
			return espFormFirewallSite{}, false, nil
		case espFormRuleUnsafe:
			return espFormRuleSite(site, rule, j.reason), true, nil
		case espFormRuleUnspecified:
			panic("BUG: espFormJudgeRule answered no judgement")
		}
	}
	found, blocked := espFormPolicy(site, b.chain)
	return found, blocked, nil
}

// espFormPolicy judges the end of a base chain ESP may reach. site names the chain.
func espFormPolicy(site espFormFirewallSite, c *nftables.Chain) (espFormFirewallSite, bool) {
	// refuse-by-default: guards espFormFirewallBlocks -- the design reads a base chain sent with no policy as accept, the kernel's default
	if c.Policy == nil {
		return espFormFirewallSite{}, false
	}
	site.Rule = espFormReasonPolicyDrop
	switch *c.Policy {
	case nftables.ChainPolicyAccept:
		return espFormFirewallSite{}, false
	case nftables.ChainPolicyDrop:
		site.Reason = espFormReasonPolicyDrop
		return site, true
	default:
		// The set is open: the value comes from the kernel, which today sends only
		// NF_DROP and NF_ACCEPT. A value nothing here can read keeps the fast path off.
		site.Rule = "policy " + textbuf.StringUint32(uint32(*c.Policy))
		site.Reason = espFormReasonPolicy
		return site, true
	}
}

// espFormRuleSite names a rule of the base chain site names.
func espFormRuleSite(site espFormFirewallSite, r *nftables.Rule, reason string) espFormFirewallSite {
	var rule textbuf.Buffer
	rule.Str("handle ").Uint(r.Handle)
	if comment := espFormRuleComment(r.UserData); comment != "" {
		rule.Byte(' ').Quoted(comment)
	}
	site.Rule = rule.String()
	site.Reason = reason
	return site
}

// espFormRuleComment reads a rule's comment from its user data. nft writes a TLV
// list whose comment is type NFTNL_UDATA_RULE_COMMENT; Ze writes the term name as
// the raw bytes (internal/plugins/firewall/nft/backend_linux.go, addChainRules). A
// TLV comment wins; otherwise printable text is taken as Ze's name; anything else
// reads as no comment. The TLV is walked here because userdata.Get slices past the
// end of a short buffer, and foreign user data is any bytes.
func espFormRuleComment(data []byte) string {
	for rest := data; len(rest) >= 2; {
		typ, length := userdata.Type(rest[0]), int(rest[1])
		if len(rest) < 2+length {
			break
		}
		if typ == userdata.TypeComment {
			return string(bytes.TrimSuffix(rest[2:2+length], []byte{0}))
		}
		rest = rest[2+length:]
	}
	if !utf8.Valid(data) {
		return ""
	}
	for _, r := range string(data) {
		if !unicode.IsPrint(r) {
			return ""
		}
	}
	return string(data)
}

// espFormRuleKind is what one rule does to bare IPv4 ESP. The zero value is never
// answered.
type espFormRuleKind uint8

const (
	espFormRuleUnspecified espFormRuleKind = iota
	// espFormRuleExcluded: the rule's first two expressions stop it for every IPv4
	// ESP packet (espFormExcludesESP), before anything else in it runs.
	espFormRuleExcluded
	// espFormRulePasses: every expression of the rule is on the harmless list
	// (espFormHarmless), and the rule ends with no verdict, a break or an accept,
	// so it changes nothing about ESP, and the ESP it does not accept goes on to
	// the next rule of the same chain.
	espFormRulePasses
	// espFormRuleAccepts: the definite accept (espFormAcceptsESP), which accepts
	// every IPv4 ESP packet that reaches it and ends the chain's walk as safe.
	espFormRuleAccepts
	// espFormRuleUnsafe: everything else. The rule writes something, sends ESP
	// anywhere but the next rule or acceptance, or holds an expression the
	// harmless list does not name.
	espFormRuleUnsafe
)

// espFormRuleJudgement is the verdict on one rule.
type espFormRuleJudgement struct {
	kind   espFormRuleKind
	reason string // espFormRuleUnsafe: the reason class
}

func espFormUnsafe(reason string) espFormRuleJudgement {
	return espFormRuleJudgement{kind: espFormRuleUnsafe, reason: reason}
}

// espFormJudgeRule judges one rule that bare IPv4 ESP can reach in a base chain.
//
// No register file is modeled. Exclusion and the definite accept read only the
// rule's own first two expressions, a load and the compare of the register that
// load has just written, so nothing an earlier rule or expression left in the
// registers can change the answer: nft_do_chain runs a rule's expressions in
// order from the first, and stops the rule at the first that sets a verdict other
// than NFT_CONTINUE. Every other rule is judged by what its expressions are,
// never by what they compute: it is harmless only when each one is on the
// harmless list and its verdict leaves ESP in the same chain or accepts it.
func espFormJudgeRule(family nftables.TableFamily, exprs []expr.Any) espFormRuleJudgement {
	if espFormExcludesESP(family, exprs) {
		return espFormRuleJudgement{kind: espFormRuleExcluded}
	}
	if espFormAcceptsESP(family, exprs) {
		return espFormRuleJudgement{kind: espFormRuleAccepts}
	}
	for _, e := range exprs {
		switch e := e.(type) {
		case *expr.Verdict:
			if e.Kind == expr.VerdictContinue {
				continue // NFT_CONTINUE: the kernel goes on to the next expression
			}
			return espFormJudgeVerdict(e)
		default:
			if reason := espFormHarmless(e); reason != "" {
				return espFormUnsafe(reason)
			}
		}
	}
	return espFormRuleJudgement{kind: espFormRulePasses}
}

// espFormJudgeVerdict judges the verdict that ends a rule that can match ESP and is
// not the definite accept.
//
// Accept ends the evaluation of the ESP it matches where the rule stands, and
// leaves the ESP it did not match at the next rule. A break sends every ESP packet
// there: it is what a compare that fails sets (NFT_BREAK), so it ends the rule the
// way any unmatched selector does. Jump, goto and return move the evaluation
// elsewhere. nft_do_chain (net/netfilter/nf_tables_core.c, v7.0) handles them
// after the rule's verdict, once accept, drop, queue and stolen have returned:
//
//	switch (regs.verdict.code) {
//	case NFT_JUMP:
//		if (WARN_ON_ONCE(stackptr >= NFT_JUMP_STACK_SIZE))
//			return NF_DROP;
//		jumpstack[stackptr].rule = nft_rule_next(rule);
//		stackptr++;
//		fallthrough;
//	case NFT_GOTO:
//		chain = regs.verdict.chain;
//		goto do_chain;
//	case NFT_CONTINUE:
//	case NFT_RETURN:
//		break;
//	default:
//		WARN_ON_ONCE(1);
//	}
//
//	if (stackptr > 0) {
//		stackptr--;
//		rule = jumpstack[stackptr].rule;
//		goto next_rule;
//	}
//	...
//	return nft_base_chain(basechain)->policy;
//
// So a return resumes after the last jump, and in a base chain, with nothing on
// the stack, it skips every later rule and meets the policy. A goto pushes nothing:
// the end of its target is a return from the chain that went to it. A verdict map
// picks any of these per packet. Following them means carrying, for each ESP
// packet, which rules it skipped and where it resumes. A walk that reads on past a
// return or a goto meets a later ESP accept that the ESP which left never reaches,
// and calls safe a ruleset whose policy drops that ESP: the rewritten datagram is
// then accepted as UDP to port 4500. So the analyzer refuses every rule that can
// match ESP and moves it, and reads base chains only. A ruleset that sends ESP
// through a jump keeps the fast path off, and the site names the jump for the
// operator to put an ESP accept ahead of it.
func espFormJudgeVerdict(v *expr.Verdict) espFormRuleJudgement {
	switch v.Kind {
	case expr.VerdictAccept, expr.VerdictBreak, expr.VerdictContinue:
		return espFormRuleJudgement{kind: espFormRulePasses}
	case expr.VerdictReturn:
		return espFormUnsafe(espFormReasonReturn)
	case expr.VerdictJump:
		return espFormUnsafe(espFormReasonJump)
	case expr.VerdictGoto:
		return espFormUnsafe(espFormReasonGoto)
	case expr.VerdictDrop:
		return espFormUnsafe(espFormReasonDrop)
	case expr.VerdictQueue:
		return espFormUnsafe(espFormReasonQueue)
	case expr.VerdictStolen, expr.VerdictRepeat, expr.VerdictStop:
		return espFormUnsafe(espFormReasonVerdict)
	default:
		// The set is open: the code comes from the kernel.
		return espFormUnsafe(espFormReasonVerdict)
	}
}

// espFormLoad is what a rule's first expression loads, when it is one of the two
// loads exclusion can rest on. The zero value is any other expression.
type espFormLoad uint8

const (
	espFormLoadUnspecified espFormLoad = iota
	// espFormLoadProtocol: meta l4proto in any family walked, or the IPv4 protocol
	// octet (network-header offset 9, one octet) in an inet or ip table.
	espFormLoadProtocol
	// espFormLoadNFProto: meta nfproto in an inet or ip table.
	espFormLoadNFProto
)

var (
	espFormProtoESP  = []byte{unix.IPPROTO_ESP}
	espFormNFProtoV4 = []byte{unix.NFPROTO_IPV4}
)

// espFormFirstLoad answers which of the two named loads e is, and the register it
// writes. Each is a value the kernel computes from the IPv4 header of the very
// packet the fast path rewrites, read before the rewrite: an untagged skb whose
// protocol is ETH_P_IP and, on an Ethernet link, whose ether type in the frame
// reads the same (espFormFastPathInstructions), with an IPv4 header of IHL 5 at
// the network header. The quotes are v7.0.
//
// meta l4proto. nft_meta_get_eval (net/netfilter/nft_meta.c): "case
// NFT_META_L4PROTO: if (!(pkt->flags & NFT_PKTINFO_L4PROTO)) goto err;
// nft_reg_store8(dest, pkt->tprot);". For an IPv4 packet tprot is its header's
// protocol octet, 50 for ESP, in every family walked
// (include/net/netfilter/nf_tables_ipv4.h): nft_set_pktinfo_ipv4 for the ip and
// inet hooks, "pkt->tprot = ip->protocol;", and __nft_set_pktinfo_ipv4_validate
// for netdev and bridge, "pkt->tprot = iph->protocol;". nft_do_chain_netdev picks
// it on "switch (skb->protocol) { case htons(ETH_P_IP):", nft_do_chain_bridge on
// "switch (eth_hdr(skb)->h_proto) { case htons(ETH_P_IP):"
// (net/netfilter/nft_chain_filter.c), and the program rewrites only a packet where
// both read ETH_P_IP. A header that fails validation sets no NFT_PKTINFO_L4PROTO,
// so the load stops the rule, which then acts on nothing.
//
// ip protocol. nft_payload_eval (net/netfilter/nft_payload.c): "case
// NFT_PAYLOAD_NETWORK_HEADER: offset = skb_network_offset(skb);", then
// "skb_copy_bits(skb, offset, dest, priv->len)". Octet 9 of an IPv4 header is its
// protocol, 50 for ESP. It is the octet the program checks:
// __netif_receive_skb_core runs skb_reset_network_header(skb) and
// skb_reset_mac_len(skb) before the hooks (net/core/dev.c), and tcx_run runs the
// program after "__skb_push(skb, skb->mac_len)", so the program's L2 offset lands
// on the network header. It is taken in inet and ip tables only, the two whose
// hooks run on the IPv4 path the lowering programs it for.
//
// meta nfproto. nft_meta_get_eval: "case NFT_META_NFPROTO: nft_reg_store8(dest,
// nft_pf(pkt));", and nft_pf is "return pkt->state->pf;"
// (include/net/netfilter/nf_tables.h). An IPv4 packet crosses prerouting and input
// as "NF_HOOK(NFPROTO_IPV4, NF_INET_PRE_ROUTING," in ip_rcv and
// "NF_HOOK(NFPROTO_IPV4, NF_INET_LOCAL_IN," in ip_local_deliver
// (net/ipv4/ip_input.c); an ip table registers for NFPROTO_IPV4 only,
// nft_do_chain_inet reads the same pf ("switch (state->pf) { case NFPROTO_IPV4:"),
// and nft_do_chain_inet_ingress sets "ingress_state.pf = NFPROTO_IPV4;" under
// "case htons(ETH_P_IP):". So nfproto reads ipv4 for every IPv4 packet in an inet
// or ip table, and the rewrite keeps the packet IPv4. In a netdev or bridge table
// nfproto is that family, and it is not taken.
//
// The nfproto load is kept because stock Ze rules need it: the firewall lowering
// puts "meta nfproto ipv6" first in every IPv6 address match in an inet table
// (nfprotoGuard, internal/plugins/firewall/nft/lower_linux.go), and the VRRP accept
// filter drops traffic to each virtual address in an inet input chain
// (internal/plugins/vrrp/acceptfilter.go). Without it, a VRRP instance with an IPv6
// address would keep the fast path shut.
//
// Every other load, link-layer octets, meta protocol and every other
// network-header field included, is no ground for exclusion. A link-layer load is
// the clearest reason why: nft_payload_eval rebuilds a VLAN tag the device took off
// into the octets it copies ("if (skb_vlan_tag_present(skb) &&
// nft_payload_need_vlan_adjust(priv->offset, priv->len)) { if
// (!nft_payload_copy_vlan(dest, skb, priv->offset, priv->len))"), so the ether type
// a rule reads is 0x8100 where the skb's protocol reads IPv4.
func espFormFirstLoad(family nftables.TableFamily, e expr.Any) (espFormLoad, uint32) {
	ipFamily := family == nftables.TableFamilyINet || family == nftables.TableFamilyIPv4
	switch e := e.(type) {
	case *expr.Meta:
		if e.SourceRegister {
			return espFormLoadUnspecified, 0
		}
		if e.Key == expr.MetaKeyL4PROTO {
			return espFormLoadProtocol, e.Register
		}
		if e.Key == expr.MetaKeyNFPROTO && ipFamily {
			return espFormLoadNFProto, e.Register
		}
	case *expr.Payload:
		if e.OperationType != expr.PayloadLoad {
			return espFormLoadUnspecified, 0
		}
		if !ipFamily {
			return espFormLoadUnspecified, 0
		}
		if e.Base == expr.PayloadBaseNetworkHeader && e.Offset == 9 && e.Len == 1 {
			return espFormLoadProtocol, e.DestRegister
		}
	}
	return espFormLoadUnspecified, 0
}

// espFormESPValue is the octet every IPv4 ESP packet loads for load: 50 for its
// protocol, NFPROTO_IPV4 for its nfproto.
func espFormESPValue(load espFormLoad) []byte {
	if load == espFormLoadNFProto {
		return espFormNFProtoV4
	}
	return espFormProtoESP
}

// espFormExcludesESP answers whether the rule's first two expressions stop it for
// every IPv4 ESP packet: one of the loads espFormFirstLoad names, then a compare of
// exactly the register that load wrote, for equality, against one octet that is
// not ESP's value. The load stores one octet with nft_reg_store8, so the compare
// of one octet reads that octet and nothing else. No other shape excludes: not a
// compare later in the rule, not != 50, not a load into one register compared
// through an alias of it.
func espFormExcludesESP(family nftables.TableFamily, exprs []expr.Any) bool {
	if len(exprs) < 2 {
		return false
	}
	load, reg := espFormFirstLoad(family, exprs[0])
	c, compared := exprs[1].(*expr.Cmp)
	if load == espFormLoadUnspecified {
		return false
	}
	if compared && c.Op == expr.CmpOpEq && c.Register == reg && len(c.Data) == 1 {
		return !bytes.Equal(c.Data, espFormESPValue(load))
	}
	return false
}

// espFormAcceptsESP answers whether the rule is the definite accept: a protocol
// load as its first expression, a compare of that register for equality with 50
// as its second, at most a counter, then accept, and nothing else. Ze's lowering
// emits it for a term that matches protocol esp and accepts (lowerProtoMatch, with
// the counter withCounter puts at the match/action boundary), and nft for `meta
// l4proto esp accept`, or `ip protocol esp accept` in an ip table.
func espFormAcceptsESP(family nftables.TableFamily, exprs []expr.Any) bool {
	if len(exprs) < 3 {
		return false
	}
	load, reg := espFormFirstLoad(family, exprs[0])
	c, compared := exprs[1].(*expr.Cmp)
	rest := exprs[2:]
	if _, counted := rest[0].(*expr.Counter); counted {
		rest = rest[1:]
	}
	if len(rest) != 1 {
		return false
	}
	v, verdict := rest[0].(*expr.Verdict)
	if load != espFormLoadProtocol {
		return false
	}
	return compared && verdict && v.Kind == expr.VerdictAccept &&
		c.Op == expr.CmpOpEq && c.Register == reg && bytes.Equal(c.Data, espFormProtoESP)
}

// espFormHarmless answers "" for an expression on the harmless list, and otherwise
// the reason class that makes it unsafe. The list is closed and short: an
// expression it does not name, a new kernel expression included, is unsafe. Every
// entry writes nothing but a data register, and either selects (it can only stop
// the rule, with NFT_BREAK) or records the packet somewhere outside it. No packet,
// meta or ct write is on it, and no stock Ze config needs a packet write on a base
// chain bare ESP crosses: the one DSCP set a stock producer emits, flowspec's, sits
// in a regular chain behind a jump (internal/plugins/flowspec-firewall/rule_chains.go),
// which closes the fast path on its own. A policy route's mark set is a meta write:
// it closes the fast path when the rule can match ESP, and a rule that opens with
// a protocol other than ESP is excluded before the write.
func espFormHarmless(e expr.Any) string {
	switch e := e.(type) {
	case *expr.Cmp, *expr.Range:
		// A compare reads a register and, when it fails, sets NFT_BREAK.
		return ""
	case *expr.Counter:
		// Adds the packet and its length to the rule's own counter; the packet is
		// not touched.
		return ""
	case *expr.Log:
		// Hands a copy of the packet's headers to the log or an nflog group; the
		// packet goes on unchanged and the rule's verdict is not set.
		return ""
	case *expr.Limit:
		// A token bucket that sets NFT_BREAK when the packet is over (or, with
		// over, under) the rate: a selector, and it writes nothing else.
		return ""
	case *expr.Meta:
		if e.SourceRegister {
			// meta mark set included: a mark can choose a route through an fwmark
			// ip rule, and every write is off the list.
			return espFormReasonStateSet
		}
		return "" // a load into a data register
	case *expr.Ct:
		if e.SourceRegister {
			return espFormReasonStateSet
		}
		// A load into a data register. Holding a ct expression registers the
		// conntrack hooks at rule load, which the packet's own walk never changes.
		return ""
	case *expr.Payload:
		if e.OperationType == expr.PayloadLoad {
			return ""
		}
		// Every write, the DSCP set included: nft_payload_set_eval writes the
		// checksum delta at the offset the rule chose (csum_offset, any value up to
		// U8_MAX), so one octet of DSCP can rewrite the TTL and protocol too.
		return espFormReasonRewrite
	case *expr.Exthdr:
		// Only the load is harmless: a destination register and no source one. The
		// kernel picks the operation from the registers (nft_exthdr_select_ops,
		// net/netfilter/nft_exthdr.c, v7.0): for a TCP option "if
		// (tb[NFTA_EXTHDR_SREG]) return &nft_exthdr_tcp_set_ops; if
		// (tb[NFTA_EXTHDR_DREG]) return &nft_exthdr_tcp_ops; return
		// &nft_exthdr_tcp_strip_ops;", and the strip writes the packet and can
		// drop it (nft_exthdr_tcp_strip_eval: "/* can't remove, no choice but to
		// drop */ regs->verdict.code = NF_DROP;"). A register number of zero is
		// the attribute's absence: the vendored decoder leaves the field zero, and
		// register 0 is the verdict register, never a data register.
		if e.SourceRegister != 0 {
			return espFormReasonRewrite
		}
		if e.DestRegister == 0 {
			return espFormReasonRewrite
		}
		return ""
	case *expr.Bitwise, *expr.Byteorder, *expr.Rt:
		// Each computes a data register from the packet or the routing state, and
		// writes nothing else.
		return ""
	case *expr.Fib:
		// A data register, or NFT_BREAK, for an IPv4 packet (nft_fib4_eval and
		// nft_fib4_eval_type, net/ipv4/netfilter/nft_fib_ipv4.c, v7.0, set no
		// other verdict). The inet form's "regs->verdict.code = NF_DROP;"
		// (nft_fib_inet_eval, net/netfilter/nft_fib_inet.c) is reached for IPv4
		// only on a result other than "case NFT_FIB_RESULT_OIF: case
		// NFT_FIB_RESULT_OIFNAME:" and "case NFT_FIB_RESULT_ADDRTYPE:", which
		// nft_fib_init refuses ("default: return -EINVAL;", nft_fib.c), so no
		// such rule loads. The netdev form sets NFT_BREAK for what is not IP.
		return ""
	case *expr.Socket:
		// A data register, or NFT_BREAK when no socket or no answer is found
		// (nft_socket_eval, net/netfilter/nft_socket.c, v7.0: every verdict it
		// writes is "regs->verdict.code = NFT_BREAK;"). It looks a socket up and
		// releases it; it does not assign one to the packet.
		return ""
	case *expr.Hash, *expr.Numgen:
		// A data register and nothing else: nft_jhash_eval and nft_symhash_eval
		// write "regs->data[priv->dreg] = h + priv->offset;", nft_ng_inc_eval and
		// nft_ng_random_eval write regs->data[priv->dreg] (net/netfilter/nft_hash.c
		// and nft_numgen.c, v7.0), and neither file holds a map form any more.
		return ""
	case *expr.Immediate:
		if e.Register == unix.NFT_REG_VERDICT {
			return espFormReasonVerdict
		}
		return "" // constant data into a data register
	case *expr.Lookup:
		return espFormLookup(e)
	case *expr.Notrack:
		// Marks the packet's conntrack untracked: a ct write.
		return espFormReasonStateSet
	case *expr.Dup:
		// The original goes on, but the copy the device receives would be the
		// rewritten UDP rather than the bare ESP the rule was written for.
		return espFormReasonDup
	case *expr.Dynset:
		// Adds or updates an element of a set, which a lookup in a later rule, a
		// later chain or a later packet reads: what that lookup matches for ESP
		// then depends on what this rule wrote, which no rule-by-rule walk sees.
		return espFormReasonSetUpdate
	case *expr.Objref:
		return espFormObjref(e)
	case *expr.Quota, *expr.Connlimit:
		return espFormReasonUnlisted + " " + reflect.TypeOf(e).String()
	case *expr.Reject:
		return espFormReasonReject
	case *expr.Queue:
		return espFormReasonQueue
	case *expr.NAT, *expr.Masq, *expr.Redir:
		return espFormReasonNAT
	case *expr.TProxy:
		return espFormReasonTProxy
	case *expr.SynProxy:
		return espFormReasonSynProxy
	case *expr.Target, *expr.Match:
		return espFormReasonXT
	case *espFormOpaqueExpr:
		return e.reason()
	case nil:
		return espFormReasonUnknown + " nil"
	default:
		return espFormReasonUnknown + " " + reflect.TypeOf(e).String()
	}
}

// espFormLookup classifies a set lookup. None is harmless, membership included,
// because a hit runs the matched element's own stateful expressions, which the rule
// does not carry and this file never reads: nft_lookup_eval (net/netfilter/
// nft_lookup.c, v7.0), "if (ext) { if (priv->dreg_set) nft_data_copy(...);
// nft_set_elem_update_expr(ext, regs, pkt); }", and nft_set_elem_update_expr
// (include/net/netfilter/nf_tables.h, v7.0) runs "expr->ops->eval(expr, regs,
// pkt);" for each of them. A connlimit or quota there sets the verdict, so
// `ip saddr @s counter accept` can drop ESP from an element alone. Reading them
// would mean dumping every set and its elements in the generation bracket. No stock
// Ze rule needs it: copp's set match sits behind its tcp guard, which excludes the
// rule first, and irr's whitelist closes with a drop of its own.
//
// A lookup with an output also copies the element's data into the destination
// register, "nft_data_copy(&regs->data[priv->dreg], nft_set_ext_data(ext),
// set->dlen);", and a verdict map's data is the rule's verdict.
func espFormLookup(e *expr.Lookup) string {
	if !e.IsDestRegSet {
		return espFormReasonSet
	}
	if e.DestRegister == unix.NFT_REG_VERDICT {
		return espFormReasonVerdictMap
	}
	return espFormReasonMap
}

// espFormObjref classifies a reference to a stateful object. None is on the
// harmless list: a named counter is harmless in itself, but no stock Ze rule on a
// path ESP crosses uses one, and the map form decodes as Type 0 with no name
// (the vendored decoder does not read NFTA_OBJREF_SET_*), so the object it picks
// per packet is unknown.
func espFormObjref(e *expr.Objref) string {
	if e.Type == unix.NFT_OBJECT_SYNPROXY {
		return espFormReasonSynProxy
	}
	return espFormReasonObjref + " " + textbuf.StringInt(int64(e.Type))
}
