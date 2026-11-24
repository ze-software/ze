// Design: docs/architecture/firewall/firewall-irr.md -- firewall IRR plugin entry point
// Related: stored.go -- the reader that reads the irr plugin's stored lists
// Related: doctor.go -- the ze doctor checks over the same references verifyStored guards
//
// Package irr implements IRR-based prefix-list filtering for firewall rules.
// It fetches nothing. The irr plugin (internal/plugins/irr) fetches, schedules
// and stores every AS-SET a consumer enforces; this plugin tells it which ones
// through its interest key, reads what it stored (stored.go), populates
// nftables interval sets, and registers them through the firewall table
// registry for merged apply.
//
// A refresh that learns nothing never replaces stored prefixes: the irr plugin
// keeps the last known good copy per family, marks it stale, and this plugin
// reports the stale copy. An answer carrying IPv4 and nothing for IPv6 keeps
// the stored IPv6 prefixes, so the interface binding keeps its IPv6 accept term
// and its drop term never takes every IPv6 packet on the port (sets.go).
package irr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var logger = slogutil.LazyLogger("firewall.irr")

// pluginName is the plugin's registered name, its table registry owner and the
// consumer name its interest key carries.
const pluginName = "firewall-irr"

// verifyReadTimeout bounds the store reads one commit verify makes.
const verifyReadTimeout = 30 * time.Second

// enforcedCopy is the stored copy of one reference this plugin enforces: the
// AS-SET it holds (the set bound to an AS number for an ASN reference), the
// pointer it was read under, and its prefixes in the form the set builders
// take (sets.go).
type enforcedCopy struct {
	asSet     string
	pointer   store.Pointer
	entry     *store.CachedEntry
	checkedAt time.Time
}

// refState is one configured reference and the copy enforced for it. The
// reference name is the identity: it names the nftables sets, so a reload
// keeping the name keeps the copy, and a new name starts from no list.
type refState struct {
	ref irrRef
	// configured is the AS-SET in key form for an AS-SET reference, and empty
	// for an ASN reference, whose set is the stored binding of asn.
	configured string
	asn        uint32
	// keyErr says why the reference names no key; such a reference is never
	// read.
	keyErr string
	copy   *enforcedCopy // nil: no list is enforced
	// readErr is the last read of a stored value that failed; the copy above
	// stays enforced meanwhile.
	readErr string
}

// interestFailure is the last refusal of the interest key write.
type interestFailure struct {
	err   string
	since time.Time
}

// applyFailure is the last apply of the firewall tables that failed.
type applyFailure struct {
	err   string
	since time.Time
}

type irrPlugin struct {
	plugin *sdk.Plugin
	reader *storeReader

	mu      sync.RWMutex
	config  *irrConfig
	refs    map[string]*refState // by reference name
	started bool

	interestErr atomic.Pointer[interestFailure]
	applyErr    atomic.Pointer[applyFailure]
}

func runFirewallIRR(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	plug := &irrPlugin{plugin: p, refs: make(map[string]*refState)}
	plug.reader = newStoreReader(plug, p)

	// pendingCfg carries the config verified by OnConfigVerify into
	// OnConfigApply, which receives only a diff. Without it a reload would
	// apply the config the daemon started with: the refs and the interface
	// bindings live here.
	var pendingCfg *irrConfig

	// The startup handshake delivers the config before any state RPC works,
	// so OnConfigure records it and nothing more. It registers no table and
	// reports no reference as unstored: the OnStarted read does both from
	// what is stored, once (AC-21).
	p.OnConfigure(plug.handshakeConfigure)

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg := parseIRRConfig(sections)
		if err := plug.verify(cfg); err != nil {
			return err
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg != nil {
			plug.configure(cfg)
		}
		// No verify ran when cfg is nil, so nothing new was approved: the
		// pass reconciles what is configured.
		return plug.reader.passNow(context.Background())
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string, _ []sdk.PipeOperation) (string, any, error) {
		return plug.handleCommand(command, args)
	})
	p.OnDoctorCheck(plug.answerDoctorCheck)

	// The irr plugin's notice of a stored or removed dataset. Subscribed from
	// Stage 1 Receives before any event flows, so a notice sent between the
	// subscription and the OnStarted read is delivered and queued.
	p.OnEvent(func(event string) error {
		plug.reader.notice(event)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	p.OnStarted(func(_ context.Context) error {
		plug.mu.Lock()
		plug.started = true
		plug.mu.Unlock()
		// The reader lives as long as the plugin, across every configure
		// cycle, and stops with ctx.
		go plug.reader.run(ctx)
		plug.reader.readAll()
		return nil
	})

	if err := p.Run(ctx, &sdk.Registration{
		Commands:     commandDecls(),
		WantsConfig:  []string{configRoot},
		DoctorChecks: []sdk.DoctorCheckDecl{storedCopyCheckDecl},
		Sends:        firewallIRRSends(),
		Receives:     firewallIRRReceives(),
		// The lists this plugin enforces survive its crash in the irr plugin's
		// store, so a restarted process reads them again and programs its sets.
		// Without the restart the registry holds back every table naming an IRR
		// set for the life of the daemon.
		FailurePolicy: sdk.FailureRestart,
	}); err != nil {
		logger().Error("firewall-irr plugin failed", "error", err)
		return 1
	}
	return 0
}

// handshakeConfigure is the OnConfigure callback: the config the daemon starts
// with, delivered inside the startup handshake, where no state RPC works yet.
// It records the config and does nothing else. Applying the tables here
// programmed them from an empty cache and warned that every reference held no
// data, at every start, before the OnStarted read programmed them again.
func (plug *irrPlugin) handshakeConfigure(sections []sdk.ConfigSection) error {
	plug.configure(parseIRRConfig(sections))
	return nil
}

// configure installs cfg as the plugin's running configuration. A reference
// whose name the previous config also carried keeps its enforced copy; every
// other one starts from no list until the reader reads it. It reads and
// writes no store and programs nothing: the reader does, on its pass.
func (plug *irrPlugin) configure(cfg *irrConfig) {
	refs := make(map[string]*refState)
	for _, ref := range cfg.allRefs() {
		if _, done := refs[ref.Name]; done {
			continue
		}
		refs[ref.Name] = newRefState(ref)
	}
	plug.mu.Lock()
	for name, st := range refs {
		if previous, ok := plug.refs[name]; ok && previous.configured == st.configured && previous.asn == st.asn {
			st.copy = previous.copy
		}
	}
	plug.config = cfg
	plug.refs = refs
	plug.mu.Unlock()
	logger().Debug("configured", "references", len(refs))
}

// newRefState keys ref: an AS-SET reference by its name in key form, an ASN
// reference by its AS number.
func newRefState(ref irrRef) *refState {
	st := &refState{ref: ref}
	if ref.IsASSet {
		name, err := store.ASSetName(ref.Name)
		if err != nil {
			st.keyErr = err.Error()
			return st
		}
		st.configured = name
		return st
	}
	number, err := asn.Parse(bareASN(ref.Name))
	if err != nil || number == 0 {
		st.keyErr = "not an AS number"
		return st
	}
	st.asn = number
	return st
}

// verify refuses a config whose IRR references cannot be enforced as written.
// It reads the irr plugin's store and nothing else: a commit sends no IRR
// query, so a reference with nothing stored is refused with the command that
// fetches it (Thomas, design gate).
func (plug *irrPlugin) verify(cfg *irrConfig) error {
	refs := cfg.allRefs()
	if len(refs) == 0 {
		return nil
	}
	plug.mu.RLock()
	started := plug.started
	plug.mu.RUnlock()
	if !started {
		return opererr.New("firewall irr: the stored IRR lists cannot be read before the plugin has started",
			"a firewall rule naming an ASN or AS-SET is checked against the stored list",
			"commit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), verifyReadTimeout)
	defer cancel()
	return verifyStored(ctx, plug.reader.state, refs)
}

// verifyStored refuses the first reference whose stored copy cannot be
// enforced as written: nothing stored, a stored value that cannot be read, a
// list holding no prefixes, or a list too long for a firewall set. Each
// message names the reference and the command that repairs it.
func verifyStored(ctx context.Context, state store.StateReader, refs []irrRef) error {
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		st := newRefState(ref)
		if st.keyErr != "" {
			return opererr.New("firewall irr: "+ref.Name+" names no IRR list ("+st.keyErr+")",
				"the rules naming it could filter nothing", "show firewall irr")
		}
		stored, err := readStoredCopy(ctx, state, st)
		if errors.Is(err, store.ErrNotStored) {
			return notStoredError(ref)
		}
		if err != nil {
			return opererr.Wrap(err, "firewall irr: the stored list for "+ref.Name+" cannot be read",
				"the rules naming it cannot be checked", fetchCommand(ref))
		}
		if stored.entry.PrefixList().Empty() {
			return opererr.New("firewall irr: the stored list for "+ref.Name+" holds no prefixes",
				"the rules naming it would filter nothing", fetchCommand(ref))
		}
		if err := oversizedEntryError(ref.Name, stored.entry); err != nil {
			return err
		}
	}
	return nil
}

// readStoredCopy reads the copy st enforces: the binding of an ASN reference,
// then the pointer of the set, then the dataset it names. ErrNotStored means
// the irr plugin holds nothing for it.
func readStoredCopy(ctx context.Context, state store.StateReader, st *refState) (*enforcedCopy, error) {
	asSet := st.configured
	if asSet == "" {
		binding, err := store.ReadBinding(ctx, state, st.asn)
		if err != nil {
			return nil, err
		}
		asSet = binding.ASSet
	}
	stored, err := store.ReadSet(ctx, state, asSet)
	if err != nil {
		return nil, err
	}
	return copyOf(st.ref.Name, asSet, stored), nil
}

// copyOf turns a stored set into the copy enforced under the reference name.
func copyOf(name, asSet string, stored store.StoredSet) *enforcedCopy {
	return &enforcedCopy{
		asSet:   asSet,
		pointer: stored.Pointer,
		entry: &store.CachedEntry{
			Name: name, ASSet: asSet,
			IPv4: stored.Dataset.IPv4(), IPv6: stored.Dataset.IPv6(),
			RefreshedAt: stored.Dataset.RefreshedAt(), StaleSince: stored.Dataset.StaleSince(),
		},
		checkedAt: stored.Dataset.CheckedAt(),
	}
}

// notStoredError is the refusal of a reference the irr plugin holds nothing
// for, naming the command that fetches and stores it.
func notStoredError(ref irrRef) error {
	var tb textbuf.Buffer
	tb.Str("firewall irr: nothing is stored for ")
	if ref.IsASSet {
		tb.Str("as-set ")
	}
	tb.Str(ref.Name)
	return opererr.New(tb.String(), "the rules naming it would filter nothing", fetchCommand(ref))
}

// fetchCommand is the irr plugin command that fetches and stores ref's list.
func fetchCommand(ref irrRef) string {
	if ref.IsASSet {
		return "update irr as-set " + ref.Name
	}
	return "update irr asn " + bareASN(ref.Name)
}

// clearCommand is the irr plugin command that removes ref's stored list.
func clearCommand(ref irrRef) string {
	if ref.IsASSet {
		return "clear irr as-set " + ref.Name
	}
	return "clear irr asn " + bareASN(ref.Name)
}

// enforcedLists is the copy enforced for each reference name, as the set
// builders read it.
type enforcedLists map[string]*store.CachedEntry

// Get answers the prefixes enforced for name, or nil for none.
func (l enforcedLists) Get(name string) *store.CachedEntry { return l[name] }

// snapshot returns the config and the enforced lists in one read of plug.mu.
func (plug *irrPlugin) snapshot() (*irrConfig, enforcedLists) {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	lists := make(enforcedLists, len(plug.refs))
	for name, st := range plug.refs {
		if st.copy != nil {
			lists[name] = st.copy.entry
		}
	}
	return plug.config, lists
}

// applyTables registers the sets and tables the config and the enforced
// copies ask for, and applies the merged ruleset. The reader is its only
// caller, after a read.
func (plug *irrPlugin) applyTables() error {
	cfg, lists := plug.snapshot()
	termRefs := cfg.termRefs()
	var ifaceBindings []ifaceBinding
	if cfg != nil {
		ifaceBindings = cfg.ifaceBindings
	}
	if len(termRefs) == 0 && len(ifaceBindings) == 0 {
		_ = firewall.RegisterTables(pluginName, nil) // a withdraw registers no name, so it cannot be refused
		return nil
	}

	// A reference that does not fit a firewall set stops the apply, and the
	// sets already registered stay in force. Programming what fits would put
	// part of the operator's rule in the kernel and report a success
	// (oversizedEntryError, sets.go).
	if err := refuseOversizedRefs(lists, cfg.allRefs()); err != nil {
		return err
	}

	tables := buildIRRTables(lists, termRefs)
	tables = append(tables, buildIfaceTables(lists, ifaceBindings)...)
	if err := firewall.RegisterTables(pluginName, tables); err != nil {
		return fmt.Errorf("firewall-irr: register: %w", err)
	}
	if err := firewall.ApplyAll(); err != nil {
		return fmt.Errorf("firewall-irr: apply: %w", err)
	}
	return nil
}

// isASSetName reports whether name is an AS-SET reference rather than a bare
// "AS<n>" identity. The two take different verbs.
func isASSetName(name string) bool {
	return bareASN(name) == name
}

// bareASN strips the "AS" prefix from an "AS<n>" name, returning name unchanged
// when what follows is not a number.
func bareASN(name string) string {
	if len(name) <= 2 || (name[0] != 'A' && name[0] != 'a') || (name[1] != 'S' && name[1] != 's') {
		return name
	}
	num := name[2:]
	if _, err := asn.Parse(num); err != nil {
		return name
	}
	return num
}

func extractIfaceRefs(root map[string]any) []irrRef {
	fw, ok := root[configRoot].(map[string]any)
	if !ok {
		return nil
	}
	irrBlock, ok := fw["irr"].(map[string]any)
	if !ok {
		return nil
	}
	ifaceMap, ok := irrBlock["interface"].(map[string]any)
	if !ok {
		return nil
	}
	var refs []irrRef
	for _, v := range ifaceMap {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		asSet, ok := entry["source-as-set"].(string)
		if !ok || asSet == "" {
			continue
		}
		refs = append(refs, irrRef{Name: asSet, IsASSet: true, IsSrc: true})
	}
	return refs
}

// buildIfaceBindingShapeTables reads firewall irr interface bindings from tree
// without mutating tree and returns the table shape (ze_irr_iface with its
// ingress chain, named set and input-interface terms) that each binding
// produces once its IRR list holds prefixes, regardless of stored prefix
// contents. Unlike buildIfaceTables on the apply path (which intentionally
// emits nothing for an empty prefix list so an interface is never blackholed),
// commit-time backend verification must see the shape even on a cold cache.
//
// Safe for concurrent use.
func buildIfaceBindingShapeTables(tree *config.Tree) ([]firewall.Table, error) {
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- a nil tree holds no IRR interface bindings
	if tree == nil {
		return nil, nil
	}
	fwTree := tree.GetContainer(configRoot)
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- absent firewall container holds no IRR interface bindings
	if fwTree == nil {
		return nil, nil
	}
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- inactive firewall container holds no IRR interface bindings
	if fwTree.IsInactive() {
		return nil, nil
	}
	irrTree := fwTree.GetContainer("irr")
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- absent firewall irr container holds no interface bindings
	if irrTree == nil {
		return nil, nil
	}
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- inactive firewall irr container holds no interface bindings
	if irrTree.IsInactive() {
		return nil, nil
	}

	sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRoot})
	if err != nil {
		return nil, err
	}
	bindings := parseIRRConfig(sections).ifaceBindings
	if len(bindings) == 0 {
		return nil, nil
	}

	seenSets := make(map[string]bool, len(bindings))
	sets := make([]firewall.Set, 0, len(bindings))
	terms := make([]firewall.Term, 0, len(bindings)*2)
	for _, ib := range bindings {
		v4Name, _ := setNames(ib.ASSet)
		if !seenSets[ib.ASSet] {
			seenSets[ib.ASSet] = true
			sets = append(sets, familySet(v4Name, firewall.SetTypeIPv4, nil))
		}
		terms = append(terms,
			firewall.Term{
				Name: ifaceTermName(ib.Interface, "v4"),
				Matches: []firewall.Match{
					firewall.MatchInputInterface{Name: ib.Interface},
					firewall.MatchInSet{SetName: v4Name, MatchField: firewall.SetFieldSourceAddr},
				},
				Actions: []firewall.Action{firewall.Accept{}},
			},
			firewall.Term{
				Name: ifaceTermName(ib.Interface, "drop"),
				Matches: []firewall.Match{
					firewall.MatchInputInterface{Name: ib.Interface},
				},
				Actions: []firewall.Action{firewall.Drop{}},
			},
		)
	}

	return []firewall.Table{{
		Name:   ifaceTableName,
		Family: firewall.FamilyInet,
		Sets:   sets,
		Chains: []firewall.Chain{{
			Name:     ifaceChainName,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookPrerouting,
			Priority: ifaceChainPrio,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}}, nil
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl). Fetching and removing
// a stored list are the irr plugin's `update irr` and `clear irr`.
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: cmdShowIRR, ShortHelp: "Show the IRR lists the firewall enforces"},
		{Name: cmdShowIRRPrefix, ShortHelp: "Show the prefixes the firewall enforces for an ASN or AS-SET", Args: []string{"<asn-or-as-set>"}},
	}
}
