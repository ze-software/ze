// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_firewall_linux.go -- espFormFirewallBlocks, the judgement this reader and watch feed
// Related: espform_fastpath_gate_linux.go -- the gate that opens the reader and the watch

//go:build linux

package dataplane

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/mdlayher/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// espFormOpaqueExpr stands for an expression the kernel holds that this file cannot
// read: a name the vendored library has no type for (fwd, osf, xfrm, last, inner,
// tunnel), or a known one whose data did not decode. espFormStep judges it unsafe.
//
// It embeds expr.Notrack only because expr.Any's methods are unexported, and the
// embedding is the one way a type outside package expr can sit in a Rule's Exprs.
// It is never marshaled: the reader is read-only, and no rule holding one is ever
// handed to Conn.AddRule.
type espFormOpaqueExpr struct {
	expr.Notrack
	name string
	err  error // why a known expression's data did not decode, nil for an unknown name
}

var _ expr.Any = (*espFormOpaqueExpr)(nil)

func (e *espFormOpaqueExpr) reason() string {
	switch e.name {
	case "fwd":
		return espFormReasonForward
	case "match", "target":
		return espFormReasonXT
	}
	return espFormReasonUnknown + " " + e.name
}

// espFormKernelReader reads the ruleset of the network namespace it was opened in,
// through dumps of its own on one netfilter socket, tables, chains and rules alike.
// The vendored library's readers are not used, because each one reads less strictly
// than the judgement needs:
//
//   - Conn.GetRules drops each expression the library has no type for
//     (vendor/github.com/google/nftables/expr/expr.go, exprsFromBytes: "continue //
//     unsupported expression type"), and also the library types its decoder does not
//     name (dup, tproxy, rt, socket, byteorder), so a netdev `fwd` would read as a
//     rule with no verdict.
//   - Conn.ListTables and Conn.ListChains hand a message the kernel flagged
//     NLM_F_DUMP_INTR through as if it were whole.
//   - chainFromMsg never reads its attribute decoder's error, and drops the error
//     hookFromMsg answers (vendor/github.com/google/nftables/chain.go), so a chain
//     whose hook did not decode reads as a base chain on hook 0, or as a regular
//     chain the walk never reads.
//
// Not safe for concurrent use.
type espFormKernelReader struct {
	dumps *netlink.Conn
}

var _ espFormNFTReader = (*espFormKernelReader)(nil)

// openESPFormNFTReader opens the reader in the calling thread's network namespace.
// The socket is opened here, so the reader stays in that namespace whichever thread
// dumps through it later. The returned func closes it.
func openESPFormNFTReader() (espFormNFTReader, func() error, error) {
	dumps, err := netlink.Dial(unix.NETLINK_NETFILTER, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open a netfilter socket to dump the nftables ruleset: %w", err)
	}
	r := &espFormKernelReader{dumps: dumps}
	return r, r.close, nil
}

func (r *espFormKernelReader) close() error {
	return r.dumps.Close()
}

// dump sends one nftables dump request, message type msg of family, and answers
// every message the kernel sent back but the NLMSG_DONE that ends it.
func (r *espFormKernelReader) dump(msg uint16, family byte, attrs []netlink.Attribute) ([]netlink.Message, error) {
	data := []byte{family, unix.NFNETLINK_V0, 0, 0}
	if len(attrs) > 0 {
		encoded, err := netlink.MarshalAttributes(attrs)
		if err != nil {
			return nil, fmt.Errorf("encode the dump request: %w", err)
		}
		data = append(data, encoded...)
	}
	request := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | msg),
			Flags: netlink.Request | netlink.Dump,
		},
		Data: data,
	}
	return r.dumps.Execute(request)
}

// ListTables dumps every table of every family.
func (r *espFormKernelReader) ListTables() ([]*nftables.Table, error) {
	replies, err := r.dump(unix.NFT_MSG_GETTABLE, unix.NFPROTO_UNSPEC, nil)
	if err != nil {
		return nil, fmt.Errorf("dump the tables: %w", err)
	}
	tables := make([]*nftables.Table, 0, len(replies))
	for _, m := range replies {
		t, err := espFormTableFromMessage(m)
		if err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// ListChains dumps every chain of every family.
func (r *espFormKernelReader) ListChains() ([]*nftables.Chain, error) {
	replies, err := r.dump(unix.NFT_MSG_GETCHAIN, unix.NFPROTO_UNSPEC, nil)
	if err != nil {
		return nil, fmt.Errorf("dump the chains: %w", err)
	}
	chains := make([]*nftables.Chain, 0, len(replies))
	for _, m := range replies {
		c, err := espFormChainFromMessage(m)
		if err != nil {
			return nil, err
		}
		chains = append(chains, c)
	}
	return chains, nil
}

// Generation asks the kernel for the ruleset's generation, one NFT_MSG_GETGEN request
// that is not a dump, and decodes the NEWGEN message it answers:
//
//	nfgenmsg (4 octets: family, version, resource id)
//	NFTA_GEN_ID                            u32, big endian
//	NFTA_GEN_PROC_PID, NFTA_GEN_PROC_NAME  the last committer, not read
//
// The kernel fills it from the counter every commit bumps (nf_tables_fill_gen_info,
// net/netfilter/nf_tables_api.c, v7.0: "nla_put_be32(skb, NFTA_GEN_ID,
// htonl(nft_base_seq(net)))"). A reply that is not one NEWGEN message, or carries no
// NFTA_GEN_ID, is an error, never generation zero: zero would compare equal to
// another failed read.
func (r *espFormKernelReader) Generation() (uint32, error) {
	request := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | unix.NFT_MSG_GETGEN),
			Flags: netlink.Request,
		},
		Data: []byte{unix.NFPROTO_UNSPEC, unix.NFNETLINK_V0, 0, 0},
	}
	replies, err := r.dumps.Execute(request)
	if err != nil {
		return 0, fmt.Errorf("ask for the ruleset generation: %w", err)
	}
	if len(replies) != 1 {
		return 0, fmt.Errorf("the generation request answered %d messages, not one", len(replies))
	}
	return espFormGenerationFromMessage(replies[0])
}

// espFormGenerationFromMessage decodes the NFTA_GEN_ID of one NEWGEN message.
func espFormGenerationFromMessage(m netlink.Message) (uint32, error) {
	ad, err := espFormDumpMessage("the generation request", unix.NFT_MSG_NEWGEN, m)
	if err != nil {
		return 0, err
	}
	var generation uint32
	found := false
	for ad.Next() {
		if ad.Type() == unix.NFTA_GEN_ID {
			generation = ad.Uint32()
			found = true
		}
	}
	if err := ad.Err(); err != nil {
		return 0, fmt.Errorf("decode the generation reply: %w", err)
	}
	if !found {
		return 0, errors.New("the generation request answered no generation id")
	}
	return generation, nil
}

// errESPFormDumpInterrupted is the cause when a dump read across a commit
// (NLM_F_DUMP_INTR): what it returned is no one ruleset.
var errESPFormDumpInterrupted = errors.New("the kernel flagged the dump interrupted (NLM_F_DUMP_INTR): " +
	"the ruleset changed while it was read, so what was read is not one ruleset")

// errESPFormChainNoHookNum is the decode error of a base chain whose hook carries no
// hook number. The kernel dumps one for every base chain
// (nft_dump_basechain_hook in net/netfilter/nf_tables_api.c, v7.0:
// "nla_put_be32(skb, NFTA_HOOK_HOOKNUM, htonl(ops->hooknum))").
var errESPFormChainNoHookNum = errors.New("a base chain's hook carries no hook number")

// espFormDumpMessage checks one message of a dump, which what names for an error: it
// is refused when the kernel flagged it NLM_F_DUMP_INTR, when it is not of message
// type want, or when it is shorter than its nfgenmsg header. It answers the decoder of
// the attributes after that header, in network byte order.
//
// The kernel flags each message it builds after the ruleset's generation moved since
// the dump's previous batch (nl_dump_check_consistent, called after each table, chain
// and rule in nf_tables_dump_tables, nf_tables_dump_chains and __nf_tables_dump_rules,
// net/netfilter/nf_tables_api.c, v7.0), and the vendored netlink library hands the
// flag through without acting on it. A dump that read across a commit is two
// rulesets spliced, so it is never read as a ruleset that could be safe: the pass
// that met the flag counts it as a commit (espFormRulesetMoved, judgeFirewall), which
// shuts the gate, steps the epoch and judges again, waiting between judgments while
// such commits keep coming, and never publishes it as a failed dump (DumpFailed). The
// flag is no complete record: the
// NLMSG_DONE message, which the library consumes, may carry it unseen, a commit
// inside one batch of the dump is not flagged at all, and a commit between two dumps
// of one judgment falls in no dump. The bracket of two generation reads around the
// whole judgment (espFormFirewallBlocks) refuses a splice whose commit stores its new
// generation between the two reads. It misses one that stores it before the first
// read and applies its table and chain changes after it, because the kernel bumps the
// number before it applies them; the NEWGEN the watch signals once that commit applied
// shuts the gate and judges again (judgeFirewall), so such a splice is open for at
// most the watch's latency. This check refuses the splices the kernel reports a
// little earlier.
func espFormDumpMessage(what string, want uint16, m netlink.Message) (*netlink.AttributeDecoder, error) {
	if m.Header.Flags&netlink.DumpInterrupted != 0 {
		return nil, fmt.Errorf("%s: %w", what, errESPFormDumpInterrupted)
	}
	if m.Header.Type != netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8|want) {
		return nil, fmt.Errorf("%s answered message type %d, not the type it asked for", what, m.Header.Type)
	}
	if len(m.Data) < 4 {
		return nil, fmt.Errorf("%s answered a message of %d octets, shorter than its header", what, len(m.Data))
	}
	ad, err := netlink.NewAttributeDecoder(m.Data[4:])
	if err != nil {
		return nil, fmt.Errorf("decode a message of %s: %w", what, err)
	}
	ad.ByteOrder = binary.BigEndian
	return ad, nil
}

// espFormTableFromMessage decodes one NEWTABLE message of the table dump:
//
//	nfgenmsg (4 octets: family, version, resource id)
//	NFTA_TABLE_NAME                        string
//
// Only the name is read: the judgement names a table by family and name.
func espFormTableFromMessage(m netlink.Message) (*nftables.Table, error) {
	ad, err := espFormDumpMessage("the table dump", unix.NFT_MSG_NEWTABLE, m)
	if err != nil {
		return nil, err
	}
	t := &nftables.Table{Family: nftables.TableFamily(m.Data[0])}
	for ad.Next() {
		if ad.Type() == unix.NFTA_TABLE_NAME {
			t.Name = ad.String()
		}
	}
	if err := ad.Err(); err != nil {
		return nil, fmt.Errorf("decode a table of the table dump: %w", err)
	}
	if t.Name == "" {
		return nil, errors.New("the table dump answered a table with no name")
	}
	return t, nil
}

// espFormChainFromMessage decodes one NEWCHAIN message of the chain dump:
//
//	nfgenmsg (4 octets: family, version, resource id)
//	NFTA_CHAIN_TABLE, NFTA_CHAIN_NAME      strings
//	NFTA_CHAIN_HOOK                        nested, base chains only:
//	                                       NFTA_HOOK_HOOKNUM u32, big endian
//	NFTA_CHAIN_POLICY                      u32, big endian, base chains only
//
// Only the fields the judgement reads are decoded: the table, the name, the hook
// number and the policy. A chain with no table or no name, a hook with no hook
// number, and an attribute of the wrong size are each an error, never a chain read
// as regular or skipped.
func espFormChainFromMessage(m netlink.Message) (*nftables.Chain, error) {
	ad, err := espFormDumpMessage("the chain dump", unix.NFT_MSG_NEWCHAIN, m)
	if err != nil {
		return nil, err
	}
	family := nftables.TableFamily(m.Data[0])
	c := &nftables.Chain{}
	for ad.Next() {
		switch ad.Type() {
		case unix.NFTA_CHAIN_TABLE:
			c.Table = &nftables.Table{Name: ad.String(), Family: family}
		case unix.NFTA_CHAIN_NAME:
			c.Name = ad.String()
		case unix.NFTA_CHAIN_POLICY:
			policy := nftables.ChainPolicy(ad.Uint32())
			c.Policy = &policy
		case unix.NFTA_CHAIN_HOOK:
			ad.Nested(func(hook *netlink.AttributeDecoder) error {
				return espFormChainHook(c, hook)
			})
		}
	}
	if err := ad.Err(); err != nil {
		return nil, fmt.Errorf("decode a chain of the chain dump: %w", err)
	}
	if c.Name == "" {
		return nil, errors.New("the chain dump answered a chain with no name")
	}
	if c.Table == nil {
		return nil, fmt.Errorf("the chain dump answered chain %s with no table", c.Name)
	}
	if c.Table.Name == "" {
		return nil, fmt.Errorf("the chain dump answered chain %s in a table with no name", c.Name)
	}
	return c, nil
}

// espFormChainHook decodes a base chain's hook into c: its hook number, which the
// kernel always dumps.
func espFormChainHook(c *nftables.Chain, hook *netlink.AttributeDecoder) error {
	hook.ByteOrder = binary.BigEndian
	for hook.Next() {
		if hook.Type() == unix.NFTA_HOOK_HOOKNUM {
			hooknum := nftables.ChainHook(hook.Uint32())
			c.Hooknum = &hooknum
		}
	}
	if err := hook.Err(); err != nil {
		return err
	}
	if c.Hooknum == nil {
		return errESPFormChainNoHookNum
	}
	return nil
}

// GetRules dumps the rules of one chain, keeping every expression in order.
func (r *espFormKernelReader) GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error) {
	replies, err := r.dump(unix.NFT_MSG_GETRULE, byte(t.Family), []netlink.Attribute{
		{Type: unix.NFTA_RULE_TABLE, Data: []byte(t.Name + "\x00")},
		{Type: unix.NFTA_RULE_CHAIN, Data: []byte(c.Name + "\x00")},
	})
	if err != nil {
		return nil, fmt.Errorf("dump the rules: %w", err)
	}
	rules := make([]*nftables.Rule, 0, len(replies))
	for _, m := range replies {
		rule, err := espFormRuleFromMessage(t, c, m)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// espFormRuleFromMessage decodes one NEWRULE message, as ruleFromMsg in the vendored
// library does, except that no expression is dropped:
//
//	nfgenmsg (4 octets: family, version, resource id)
//	NFTA_RULE_TABLE, NFTA_RULE_CHAIN       strings
//	NFTA_RULE_HANDLE, NFTA_RULE_POSITION   u64, big endian
//	NFTA_RULE_EXPRESSIONS                  nested list of NFTA_LIST_ELEM,
//	                                       each NFTA_EXPR_NAME + NFTA_EXPR_DATA
//	NFTA_RULE_USERDATA                     bytes
//
// A message flagged NLM_F_DUMP_INTR is refused before it is read
// (espFormDumpMessage says why).
func espFormRuleFromMessage(t *nftables.Table, c *nftables.Chain, m netlink.Message) (*nftables.Rule, error) {
	ad, err := espFormDumpMessage("the rule dump of chain "+c.Name, unix.NFT_MSG_NEWRULE, m)
	if err != nil {
		return nil, err
	}

	rule := &nftables.Rule{Table: t, Chain: c}
	for ad.Next() {
		switch ad.Type() {
		case unix.NFTA_RULE_HANDLE:
			rule.Handle = ad.Uint64()
		case unix.NFTA_RULE_POSITION:
			rule.Position = ad.Uint64()
		case unix.NFTA_RULE_USERDATA:
			rule.UserData = ad.Bytes()
		case unix.NFTA_RULE_EXPRESSIONS:
			ad.Nested(func(list *netlink.AttributeDecoder) error {
				exprs, err := espFormExprsFromList(byte(t.Family), list)
				rule.Exprs = exprs
				return err
			})
		}
	}
	if err := ad.Err(); err != nil {
		return nil, fmt.Errorf("decode a rule of chain %s: %w", c.Name, err)
	}
	return rule, nil
}

// espFormExprsFromList decodes a rule's expression list, in order.
func espFormExprsFromList(family byte, list *netlink.AttributeDecoder) ([]expr.Any, error) {
	var exprs []expr.Any
	for list.Next() {
		if list.Type() != unix.NFTA_LIST_ELEM {
			return nil, fmt.Errorf("an expression list holds attribute %d, not a list element", list.Type())
		}
		list.Nested(func(elem *netlink.AttributeDecoder) error {
			e, err := espFormExprFromElement(family, elem)
			if err != nil {
				return err
			}
			exprs = append(exprs, e)
			return nil
		})
	}
	return exprs, list.Err()
}

// espFormExprFromElement decodes one list element: its name, then its data.
func espFormExprFromElement(family byte, elem *netlink.AttributeDecoder) (expr.Any, error) {
	var name string
	var data []byte
	for elem.Next() {
		switch elem.Type() {
		case unix.NFTA_EXPR_NAME:
			name = elem.String()
		case unix.NFTA_EXPR_DATA:
			data = elem.Bytes()
		}
	}
	if err := elem.Err(); err != nil {
		return nil, err
	}
	return espFormExprDecode(family, name, data), nil
}

// espFormExprDecode decodes an expression's data by its kernel name. Data that does
// not decode is no error of the dump: the expression is kept, as an opaque one
// carrying the decode error, so the judgement sees it and refuses it.
func espFormExprDecode(family byte, name string, data []byte) expr.Any {
	e := espFormExprByName(name)
	if _, opaque := e.(*espFormOpaqueExpr); opaque {
		return e
	}
	if err := expr.Unmarshal(family, data, e); err != nil {
		return &espFormOpaqueExpr{name: name, err: err}
	}
	if ct, ok := e.(*expr.Ct); ok {
		if err := espFormCtRegisters(ct, data); err != nil {
			return &espFormOpaqueExpr{name: name, err: err}
		}
	}
	// A verdict is an immediate that writes the verdict register and carries no
	// data value; the library re-reads it as a Verdict the same way.
	imm, ok := e.(*expr.Immediate)
	if ok && imm.Register == unix.NFT_REG_VERDICT && len(imm.Data) == 0 {
		v := &expr.Verdict{}
		if err := expr.Unmarshal(family, data, v); err != nil {
			return &espFormOpaqueExpr{name: name, err: err}
		}
		return v
	}
	return e
}

// errESPFormCtRegisters is the decode error of a ct expression that names both a
// source and a destination register, or neither: one the kernel refuses to load,
// so one no dump of a loaded ruleset should hold.
var errESPFormCtRegisters = errors.New("a ct expression names both a source and a destination register, or neither")

// espFormCtRegisters tells a ct load from a ct set. The vendored decoder reads
// NFTA_CT_DREG and never NFTA_CT_SREG (vendor/github.com/google/nftables/expr/ct.go,
// unmarshal), so every `ct zone set` or `ct label set` would read as a load into
// register 0, which espFormStep judges a selector. The kernel dumps a set with
// NFTA_CT_SREG (nft_ct_set_dump in net/netfilter/nft_ct.c, v7.0:
// "nft_dump_register(skb, NFTA_CT_SREG, priv->sreg)"), and refuses to load a ct
// naming both (nft_ct_select_ops: "if (tb[NFTA_CT_DREG] && tb[NFTA_CT_SREG])
// return ERR_PTR(-EINVAL);"). The attribute numbers are the uapi
// enum nft_ct_attributes (include/uapi/linux/netfilter/nf_tables.h): DREG 1, KEY 2,
// DIRECTION 3, SREG 4, which unix.NFTA_CT_* carries.
func espFormCtRegisters(ct *expr.Ct, data []byte) error {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	ad.ByteOrder = binary.BigEndian
	dreg, sreg := false, false
	for ad.Next() {
		if ad.Type() == unix.NFTA_CT_DREG {
			dreg = true
		}
		if ad.Type() == unix.NFTA_CT_SREG {
			sreg = true
			ct.Register = ad.Uint32()
		}
	}
	if err := ad.Err(); err != nil {
		return err
	}
	if dreg == sreg {
		return errESPFormCtRegisters
	}
	ct.SourceRegister = sreg
	return nil
}

// espFormExprByName answers an empty expression of the kernel name, for every type
// the vendored expr package declares, and an opaque one for any other name.
func espFormExprByName(name string) expr.Any {
	switch name {
	case "bitwise":
		return &expr.Bitwise{}
	case "byteorder":
		return &expr.Byteorder{}
	case "cmp":
		return &expr.Cmp{}
	case "connlimit":
		return &expr.Connlimit{}
	case "counter":
		return &expr.Counter{}
	case "ct":
		return &expr.Ct{}
	case "ctexpect":
		return &expr.CtExpect{}
	case "cthelper":
		return &expr.CtHelper{}
	case "cttimeout":
		return &expr.CtTimeout{}
	case "dup":
		return &expr.Dup{}
	case "dynset":
		return &expr.Dynset{}
	case "exthdr":
		return &expr.Exthdr{}
	case "fib":
		return &expr.Fib{}
	case "flow_offload":
		return &expr.FlowOffload{}
	case "hash":
		return &expr.Hash{}
	case "immediate":
		return &expr.Immediate{}
	case "limit":
		return &expr.Limit{}
	case "log":
		return &expr.Log{}
	case "lookup":
		return &expr.Lookup{}
	case "masq":
		return &expr.Masq{}
	case "match":
		return &expr.Match{}
	case "meta":
		return &expr.Meta{}
	case "nat":
		return &expr.NAT{}
	case "notrack":
		return &expr.Notrack{}
	case "numgen":
		return &expr.Numgen{}
	case "objref":
		return &expr.Objref{}
	case "payload":
		return &expr.Payload{}
	case "queue":
		return &expr.Queue{}
	case "quota":
		return &expr.Quota{}
	case "range":
		return &expr.Range{}
	case "redir":
		return &expr.Redir{}
	case "reject":
		return &expr.Reject{}
	case "rt":
		return &expr.Rt{}
	case "secmark":
		return &expr.SecMark{}
	case "socket":
		return &expr.Socket{}
	case "synproxy":
		return &expr.SynProxy{}
	case "target":
		return &expr.Target{}
	case "tproxy":
		return &expr.TProxy{}
	}
	return &espFormOpaqueExpr{name: name}
}

// errESPFormNFTWatchEnded is the cause when the monitor's channel closed with no
// failure the library reported and no Close asked for it.
var errESPFormNFTWatchEnded = errors.New("the nftables monitor ended without a reported failure")

// espFormNFTWatchDepth is the library monitor's channel depth. The worker never
// blocks while it holds a batch, so the depth only absorbs a burst of commits.
const espFormNFTWatchDepth = 8

// espFormNFTWatchBufferOctets is the receive buffer the watch's socket asks for. The
// kernel queues one notification for each object a transaction changes, a NEWRULE for
// each rule, before the NEWGEN that ends it, and charges each to this buffer. Ze's
// firewall applies its whole ruleset on every apply, so the default
// (net.core.rmem_default, 208 KiB on most kernels) can overflow on a large one: the
// kernel then drops the notifications and the next receive fails with ENOBUFS, which
// ends the monitor. 16 MiB is a ceiling the kernel charges as messages queue, not
// memory allocated up front. It only moves the limit, so the fast path's worker still
// watches again after a failure. NETLINK_NO_ENOBUFS is not the answer: it hides the
// overflow, and a NEWGEN lost in it is a commit the gate never judges.
const espFormNFTWatchBufferOctets = 16 << 20

// espFormNFTWatch tells its owner that the nftables ruleset changed. It owns one
// long-lived worker goroutine, started by startESPFormNFTWatch: the worker reads the
// vendored library's generational monitor, which delivers one batch per committed
// transaction (the kernel's NFT_MSG_NEWGEN), and turns each into a signal on a
// one-deep channel, so commits that arrive before the owner reads coalesce into one.
// When the monitor's channel closes the worker records why and closes the signal
// channel. Close stops the monitor and waits for the worker. Safe for concurrent use.
type espFormNFTWatch struct {
	stop   func() error
	signal chan struct{}
	done   chan struct{}
	once   sync.Once

	mu      sync.Mutex
	closing bool
	cause   error
}

// startESPFormNFTWatch starts watching the ruleset of ns, or of the calling thread's
// network namespace when ns is not open (netns.None()). The socket is opened in that
// namespace here and stays there, so the caller's thread state matters only for
// this call. The channel signals "a commit happened", coalesced, and closes when the
// monitor ends; Err then names why. The caller MUST call Close.
//
// The socket is a lasting one the watch closes itself. The library's monitor ends on
// a failed receive without closing its socket, and its Close then finds the monitor
// closed and skips the closer (vendor/github.com/google/nftables/monitor.go, monitor
// and Close), so a transient socket would leak on every failure the fast path's
// worker answers by watching again. Close here closes the lasting socket either way.
func startESPFormNFTWatch(ns netns.NsHandle) (*espFormNFTWatch, <-chan struct{}, error) {
	opts := []nftables.ConnOption{nftables.AsLasting(), nftables.WithSockOptions(espFormNFTWatchBuffer)}
	if ns.IsOpen() {
		opts = append(opts, nftables.WithNetNSFd(int(ns)))
	}
	conn, err := nftables.New(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("open an nftables connection to watch the ruleset: %w", err)
	}
	monitor := nftables.NewMonitor(nftables.WithMonitorEventBuffer(espFormNFTWatchDepth))
	events, err := conn.AddGenerationalMonitor(monitor)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("join the nftables multicast group to watch the ruleset: %w", err), conn.CloseLasting())
	}
	w := newESPFormNFTWatch(events, func() error { return errors.Join(monitor.Close(), conn.CloseLasting()) })
	return w, w.signal, nil
}

// espFormNFTWatchBuffer is the watch socket's option: it raises the receive buffer to
// espFormNFTWatchBufferOctets. A refusal is logged and the watch keeps the kernel's
// default, because a watch with a small buffer still works until a large commit
// overflows it, and the worker recovers that by watching again. It never fails the
// dial: the library returns an option's error without closing the socket it dialed
// (vendor/github.com/google/nftables/conn.go, dialNetlink).
func espFormNFTWatchBuffer(conn *netlink.Conn) error {
	if err := forceESPFormNFTWatchBuffer(conn); err != nil {
		slogutil.Logger("ike.dataplane").Warn("esp-form: the nftables watch keeps the kernel's default receive "+
			"buffer, so a large firewall commit can overflow it; the fast path then stays shut until it "+
			"watches the ruleset again",
			"error", err, "octets", espFormNFTWatchBufferOctets,
			"fix", privilege.DaemonCapabilitiesFix,
			"code", diagnostic.CodeDoctorIPsecESPFormFirewall)
	}
	return nil
}

// forceESPFormNFTWatchBuffer sets the socket's receive buffer with SO_RCVBUFFORCE,
// past net.core.rmem_max. It needs CAP_NET_ADMIN, which joining the nftables group
// already needs, so a watch that can join can also take the buffer.
func forceESPFormNFTWatchBuffer(conn *netlink.Conn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("reach the nftables watch socket: %w", err)
	}
	var setErr error
	err = raw.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, espFormNFTWatchBufferOctets)
	})
	if err != nil {
		return fmt.Errorf("reach the nftables watch socket: %w", err)
	}
	if setErr != nil {
		return fmt.Errorf("set SO_RCVBUFFORCE to %d octets: %w", espFormNFTWatchBufferOctets, setErr)
	}
	return nil
}

// newESPFormNFTWatch starts the worker over a monitor's channel. stop MUST end that
// channel: the library closes it once the monitor's socket is closed.
func newESPFormNFTWatch(events <-chan *nftables.MonitorEvents, stop func() error) *espFormNFTWatch {
	w := &espFormNFTWatch{
		stop:   stop,
		signal: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	go w.run(events)
	return w
}

// run is the worker. It reads the monitor's channel to its end, so the library's
// reader never blocks on a send, even after a failure batch.
func (w *espFormNFTWatch) run(events <-chan *nftables.MonitorEvents) {
	defer close(w.done)
	defer close(w.signal)
	for batch := range events {
		if failed, err := espFormMonitorFailed(batch); failed {
			w.ended(err)
			continue // the library closes its channel right after a failure batch
		}
		select {
		case w.signal <- struct{}{}:
		default:
			// A signal is pending: the owner's next read covers this commit too.
		}
	}
	w.ended(errESPFormNFTWatchEnded)
}

// espFormMonitorFailed answers whether a batch reports the monitor's failure, with
// its cause. Every other batch is a commit: the library sends one per NFT_MSG_NEWGEN.
func espFormMonitorFailed(batch *nftables.MonitorEvents) (bool, error) {
	if batch == nil {
		return false, nil
	}
	if batch.GeneratedBy == nil {
		return false, nil
	}
	if batch.GeneratedBy.Type != nftables.MonitorEventTypeOOB {
		return false, nil
	}
	if batch.GeneratedBy.Error != nil {
		return true, fmt.Errorf("the nftables monitor failed: %w", batch.GeneratedBy.Error)
	}
	return true, errESPFormNFTWatchEnded
}

// ended records the first cause, unless Close asked for the end.
func (w *espFormNFTWatch) ended(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return
	}
	if w.cause == nil {
		w.cause = err
	}
}

// Err names why the signal channel closed. It MUST be read after the channel
// closed, and answers nil when Close ended the watch.
func (w *espFormNFTWatch) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cause
}

// Close stops the monitor and waits for the worker, which closes the signal
// channel. A second Close only waits. When the monitor's socket fails to close, the
// worker may never end, so Close logs it and returns without waiting.
func (w *espFormNFTWatch) Close() {
	var err error
	w.once.Do(func() {
		w.mu.Lock()
		w.closing = true
		w.mu.Unlock()
		err = w.stop()
	})
	if err != nil {
		slogutil.Logger("ike.dataplane").Warn("esp-form: the nftables watch did not close its socket, so its worker may outlive it",
			"error", err, "code", diagnostic.CodeDoctorIPsecESPFormFirewall)
		return
	}
	<-w.done
}
