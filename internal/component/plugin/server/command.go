// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: ensure.go -- ze:ensure-exists chain wrapping for compound commands

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errReactorNotAvailable = errors.New("reactor not available")

// Peer-selector resolution failures (ResolveSinglePeer). Separate sentinels so a
// caller can tell "you gave me a wildcard" from "nothing matched" from "several
// matched" without string matching.
var (
	errNoSpecificPeer        = errors.New("command requires one specific peer")
	errExcludePeerSelector   = errors.New("exclusion selector cannot name one peer")
	errNoPeerMatchesSelector = errors.New("no peer matches selector")
	errAmbiguousPeerSelector = errors.New("selector matches more than one peer")
)

// ErrUnknownCommand is returned when a command is not recognized.
var ErrUnknownCommand = errors.New("unknown command")

// ErrEmptyCommand is returned when the command is empty.
var ErrEmptyCommand = errors.New("empty command")

// ErrUnauthorized is returned when a command is denied by authorization.
// Its text is plugin.UnauthorizedMessage because operators read it directly:
// the ssh exec handler prints this error, not the Response.Error below.
var ErrUnauthorized = errors.New(plugin.UnauthorizedMessage)

// unauthorizedError builds the Response.Error for a denied command. Surfaces
// that render Response.Error rather than the returned error still name the
// command, which the operator needs when a whole pipeline is denied.
func unauthorizedError(input string) string {
	var tb textbuf.Buffer
	return tb.Str(plugin.UnauthorizedMessage).Str(": ").Str(input).String()
}

// ErrPluginProcessNotRunning is returned when a plugin command targets a non-running process.
var ErrPluginProcessNotRunning = errors.New("plugin process not running")

// ErrPluginConnectionClosed is returned when the plugin's connection is no longer available.
var ErrPluginConnectionClosed = errors.New("plugin connection closed")

// AllBuiltinRPCs returns all RPCs registered via init() + RegisterRPCs().
// Includes server, handler, and editor RPCs (when their packages are imported).
func AllBuiltinRPCs() []RPCRegistration {
	return registeredRPCs
}

// LoadBuiltins registers all builtin handlers with the dispatcher.
// The wireToPath map provides the dispatch key for each handler, derived from
// the YANG command tree (WireMethod -> CLI path). pathToDesc provides the
// one-line summary each command's YANG description declares, and pathToHelp the
// long explanation its ze:help extension declares. Handlers without a YANG
// entry are skipped.
func LoadBuiltins(d *Dispatcher, wireToPath, pathToDesc, pathToHelp map[string]string, pathToArgDefs map[string][]command.ArgDef) {
	for _, reg := range AllBuiltinRPCs() {
		name := wireToPath[reg.WireMethod]
		if name == "" {
			continue // No YANG tree entry (editor-internal)
		}
		d.RegisterWithOptions(name, reg.Handler, pathToDesc[name], RegisterOptions{
			ReadOnly:         IsReadOnlyPath(name),
			RequiresSelector: reg.RequiresSelector,
			PluginProxy:      reg.PluginCommand != "",
			ArgDefs:          pathToArgDefs[name],
			LongHelp:         pathToHelp[name],
		})
	}
}

// loadBuiltinsWithAliases registers all builtin handlers with the dispatcher,
// including all YANG command aliases for each wire method. When cmdTree is
// non-nil, commands whose YANG path passes through a ze:ensure-exists node
// are wrapped to auto-ensure the parent resource and rollback on failure.
func loadBuiltinsWithAliases(d *Dispatcher, wireToPaths map[string][]string, pathToDesc, pathToHelp map[string]string, pathToArgDefs map[string][]command.ArgDef, cmdTree *command.Node) {
	wireToHandler := make(map[string]Handler, len(AllBuiltinRPCs()))
	for _, reg := range AllBuiltinRPCs() {
		if reg.Handler != nil {
			wireToHandler[reg.WireMethod] = reg.Handler
		}
	}

	for _, reg := range AllBuiltinRPCs() {
		paths := wireToPaths[reg.WireMethod]
		if len(paths) == 0 {
			continue
		}
		for _, name := range paths {
			handler := reg.Handler
			if chain := buildEnsureChain(cmdTree, name, wireToHandler); len(chain) > 0 {
				handler = wrapWithEnsureChain(handler, chain)
			}
			d.RegisterWithOptions(name, handler, pathToDesc[name], RegisterOptions{
				ReadOnly:         IsReadOnlyPath(name),
				RequiresSelector: reg.RequiresSelector,
				PluginProxy:      reg.PluginCommand != "",
				ArgDefs:          pathToArgDefs[name],
				LongHelp:         pathToHelp[name],
			})
		}
	}
}

// verbRIB is the CLI verb for system RIB commands (e.g., "rib show").
const verbRIB = "rib"

// IsReadOnlyPath returns true if the command path starts with a read-only verb.
// With verb-first grammar, "show", "monitor", and "resolve" are read-only;
// "clear", "set", "request", "commit", "update" are not.
func IsReadOnlyPath(path string) bool {
	verb, _, _ := strings.Cut(path, " ")
	switch verb {
	case "show", "monitor", "resolve", "validate",
		// Legacy noun-first forms still in the YANG tree. `event` and `command`
		// were migrated to `show event list` / `show command list`, so neither is a
		// top-level verb here anymore.
		"help",
		"system", "plugin", verbRIB:
		return true
	}
	return false
}

// registerDefaultHandlers registers all builtin handlers with the dispatcher.
func registerDefaultHandlers(d *Dispatcher, wireToPath map[string]string) {
	LoadBuiltins(d, wireToPath, nil, nil, nil)
}

// Handler processes a command and returns a response.
type Handler func(ctx *CommandContext, args []string) (*plugin.Response, error)

// CommandContext provides access to reactor and session state.
// Dependencies are accessed through Server; per-request state is stored directly.
type CommandContext struct {
	Server         *Server           // Gateway to all server state (reactor, dispatcher, etc.)
	Process        *process.Process  // The API process (for session state)
	RequestContext context.Context   // Request-scoped context from the trusted transport.
	Peer           string            // Peer selector: "*" for all, or specific peer selector value. Empty = "*"
	Username       string            // Authenticated username (empty = no auth, full access)
	RemoteAddr     string            // Remote address of the client (e.g., SSH peer IP:port)
	Surface        string            // Trusted caller surface for audit attribution.
	Authorizer     plugin.Authorizer // Request-carried policy generation; nil uses the dispatcher's live policy.
	Meta           map[string]any    // Route metadata from UpdateRoute RPC; nil if not set.
	Selectors      map[string]string // Extracted typed selector values, keyed by selector keyword.
	// Sender says who issued this command, and every path that builds a
	// CommandContext states it: plugin.ProcessSender(proc.Name()) on the plugin
	// server's own dispatch paths, plugin.OperatorSender() on the operator
	// surfaces (dispatch.go, dispatch_registry.go, cmd/ze/hub).
	//
	// A peer's `attach process <name> { send [ ... ] }` block is written against
	// it, so a guard that puts a message on a peer's wire reads THIS field and
	// nothing else. Process above answers a different question, which is the RPC
	// session the command arrived on, and no guard consults it.
	//
	// The zero value means nobody said. It is refused rather than read as the
	// operator, so a dispatch path added later cannot inherit the operator's
	// authority by omission (ai/rules/evidence.md,
	// internal/component/bgp/reactor/send_permission.go).
	Sender plugin.Sender
}

// Reactor returns the BGP reactor lifecycle interface via Server.
// Nil-safe: returns nil if Server is nil.
func (c *CommandContext) Reactor() plugin.ReactorLifecycle {
	if c.Server == nil {
		return nil
	}
	return c.Server.Reactor()
}

// ProtocolReactor returns a named protocol reactor from the Coordinator.
// Callers type-assert to the protocol-specific interface they need.
// Nil-safe: returns nil if Server is nil or protocol not registered.
func (c *CommandContext) ProtocolReactor(name string) any {
	if c.Server == nil {
		return nil
	}
	return c.Server.ReactorFor(name)
}

// Dispatcher returns the command dispatcher via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) Dispatcher() *Dispatcher {
	if c.Server == nil {
		return nil
	}
	return c.Server.Dispatcher()
}

// CommitManager returns the commit manager via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) CommitManager() any {
	if c.Server == nil {
		return nil
	}
	return c.Server.CommitManager()
}

// Subscriptions returns the subscription manager via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) Subscriptions() *SubscriptionManager {
	if c.Server == nil {
		return nil
	}
	return c.Server.Subscriptions()
}

// Context returns the request context for this command.
// Nil-safe: falls back from request -> server -> background.
func (c *CommandContext) Context() context.Context {
	if c != nil && c.RequestContext != nil {
		return c.RequestContext
	}
	if c != nil && c.Server != nil {
		if serverCtx := c.Server.Context(); serverCtx != nil {
			return serverCtx
		}
	}
	return context.Background()
}

// RequireReactor returns the reactor or an error response if not available.
func RequireReactor(ctx *CommandContext) (plugin.ReactorLifecycle, *plugin.Response, error) {
	r := ctx.Reactor()
	if r == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "reactor not available",
		}, errReactorNotAvailable
	}
	return r, nil, nil
}

// ResolveSinglePeer resolves the command's peer selector to exactly one peer
// address, accepting every form the selector vocabulary defines -- an address,
// a configured peer NAME, an `as<N>` ASN, a glob -- not only an IP literal.
//
// action names the command in error text ("raw", "pause", ...).
//
// Why this exists: the peer-scoped commands split into two camps. `teardown`
// and `flush` each carried their own hand-rolled "not an IP, try the name"
// loop, while `raw`, `pause`, `resume`, `clear soft` and `delete bgp peer` went
// straight to netip.ParseAddr and rejected the very selector their YANG leaf
// advertises ("Peer selector", e.g. ze-raw-cmd.yang:19). An operator who
// configured `peer peer1 { ... }` and typed `peer peer1 raw ...` got
// "invalid peer address". This is the one resolver for all of them, so the two
// existing copies stop being two and a sixth cannot appear
// (ai/rules/evidence.md).
//
// The wildcard is REFUSED, deliberately and by name. Every caller acts on a
// single session -- injecting raw bytes, pausing a read loop -- and silently
// fanning that across every peer is a footgun, so `*` is rejected with the
// selector quoted rather than being allowed to mean something surprising
// (ai/rules/evidence.md). An ambiguous selector that matches several
// peers is refused the same way: this returns ONE peer or an error, never a
// guess at which one was meant.
//
// An EXCLUSION selector ("!edge1", "!as65001", "!10.0.0.0/24") is refused on
// the same grounds -- see the comment on the check below for why a set
// complement cannot address a single destructive action.
//
// Arity, not vocabulary, is what separates this from cmd/peer's
// filterPeersBySelectorValue. That one FILTERS a list for `show`, where a
// complement is a perfectly good answer and "!edge1" legitimately returns
// several peers. This one must name one target for a destructive verb, so it
// accepts the same selector spellings and a strictly narrower set of meanings.
func ResolveSinglePeer(ctx *CommandContext, action string) (netip.Addr, *plugin.Response, error) {
	if _, errResp, err := RequireReactor(ctx); err != nil {
		return netip.Addr{}, errResp, err
	}

	sel := ctx.PeerSelector()
	if sel == "" || sel == "*" {
		var tb textbuf.Buffer
		msg := tb.Str(action).Str(" requires one specific peer, got ").Quoted(sel).
			Str("; use a peer address or a configured peer name").String()
		return netip.Addr{}, &plugin.Response{Status: plugin.StatusError, Error: msg}, errNoSpecificPeer
	}

	parsed := selector.ParseDefault(sel)

	// An EXCLUSION selector is refused for the same reason `*` is, only more
	// sharply. "!edge1" names a SET -- every peer except edge1 -- while every
	// caller here acts on exactly one session, so the two cannot be reconciled:
	// in a two-peer topology "!edge1" would resolve to precisely one peer and
	// silently tear down / delete / pause the OTHER one, and the same command
	// would start erroring the day a third peer is configured. A destructive
	// command whose target depends on how many peers happen to exist is not an
	// interface, so this refuses the whole exclusion family up front rather than
	// resolving it (ai/rules/evidence.md).
	//
	// The string check catches the forms ParseDefault cannot type: Parse rejects
	// "!*", "!" and "!a,b", and ParseDefault turns a parse error into
	// PeerName(s), so those would otherwise fall through to "no peer matches
	// selector "!*"" -- an accurate refusal with unusable advice, since it sends
	// the operator looking for a peer literally named "!*"
	// (ai/rules/cli.md, leg 3 must be TRUE).
	if parsed.IsExclude() || strings.HasPrefix(strings.TrimSpace(sel), "!") {
		var tb textbuf.Buffer
		msg := tb.Str(action).Str(": selector ").Quoted(sel).
			Str(" excludes peers instead of naming one; this command acts on a single peer, so give its address or configured name (see `show bgp peer list`)").String()
		return netip.Addr{}, &plugin.Response{Status: plugin.StatusError, Error: msg}, errExcludePeerSelector
	}

	found := PeersMatching(ctx, parsed)
	matched := make([]netip.Addr, 0, len(found))
	for i := range found {
		matched = append(matched, found[i].Address)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil, nil
	case 0:
		// An ADDRESS that matches no current peer is passed through unchanged.
		// That is the pre-existing contract every caller was written against:
		// the handler downstream (PausePeer, RemovePeer, ...) owns "no such
		// peer" and reports it with its own context, and several unit tests
		// drive these handlers with an empty peer table on purpose. Only a
		// NON-address selector -- a name, an ASN, a glob -- has to resolve here,
		// because there is nothing downstream that could interpret it.
		if ip, perr := netip.ParseAddr(sel); perr == nil {
			return ip, nil, nil
		}
		var tb textbuf.Buffer
		msg := tb.Str(action).Str(": no peer matches selector ").Quoted(sel).
			Str("; use a peer address or a configured peer name (see `show bgp peer list`)").String()
		return netip.Addr{}, &plugin.Response{Status: plugin.StatusError, Error: msg}, errNoPeerMatchesSelector
	default:
		var tb textbuf.Buffer
		msg := tb.Str(action).Str(": selector ").Quoted(sel).Str(" matches ").Int(int64(len(matched))).
			Str(" peers; it must identify exactly one").String()
		return netip.Addr{}, &plugin.Response{Status: plugin.StatusError, Error: msg}, errAmbiguousPeerSelector
	}
}

// PeersMatching returns every configured peer the selector names.
//
// It is the plural sibling of ResolveSinglePeer, and the two share one matcher
// on purpose: a caller that acts on a SUBSET of the fan-out has to resolve the
// same selector vocabulary a caller acting on one peer resolves, and a third
// hand-rolled loop is how "!edge1" came to mean two different things in two
// commands (see selectorMatchesPeer below).
//
// Returns nil when no reactor is attached, which is a context with nothing to
// select from rather than an empty selection.
func PeersMatching(ctx *CommandContext, sel *selector.Selector) []plugin.PeerInfo {
	r := ctx.Reactor()
	if r == nil || sel == nil {
		return nil
	}
	peers := r.Peers()
	var out []plugin.PeerInfo
	for i := range peers {
		if selectorMatchesPeer(sel, &peers[i]) {
			out = append(out, peers[i])
		}
	}
	return out
}

// selectorMatchesPeer applies the selector to one peer, mirroring the kind
// handling the peer-listing commands use (cmd/peer's filterPeersBySelectorValue)
// so a selector means the same thing whichever command consumes it.
//
// The exclude flag is applied HERE for the name and ASN kinds, and only there.
// Selector.Matches already negates internally for the address-shaped kinds
// (KindAddr, KindAddrs, KindGlob) but returns a flat false for KindName and
// KindASN, because IP-only matching cannot answer them -- so those two arms
// own the negation. Returning the raw comparison instead INVERTS them: "!edge1"
// matched edge1 and "!as65001" matched the peer inside AS 65001, which is the
// opposite of what the operator typed. ResolveSinglePeer now refuses exclusion
// selectors outright, so nothing reaches here with one today; the polarity is
// still correct rather than merely unreachable, because a future caller reading
// the doc line above has every right to expect the parity it claims
// (ai/rules/evidence.md).
func selectorMatchesPeer(sel *selector.Selector, p *plugin.PeerInfo) bool {
	var match bool
	switch sel.SelectorKind() {
	case selector.KindName:
		match = p.Name == sel.NameValue()
	case selector.KindASN:
		match = p.PeerAS == sel.ASNValue()
	default:
		return sel.Matches(p.Address)
	}
	if sel.IsExclude() {
		return !match
	}
	return match
}

// PeerSelector returns the effective neighbor selector.
// Returns "*" if no neighbor was specified.
func (c *CommandContext) PeerSelector() string {
	if c.Peer == "" {
		return "*"
	}
	return c.Peer
}

// Selector returns the extracted typed selector value for keyword `name`.
// Nil-safe. Lookup is case-insensitive.
func (c *CommandContext) Selector(name string) string {
	if c == nil || c.Selectors == nil {
		return ""
	}
	return c.Selectors[strings.ToLower(name)]
}

// Command represents a registered command with metadata.
//
// A Command is 168 bytes, past the 160-byte rangeValCopy bound .golangci.yml
// sets. A loop over a []Command therefore ranges by index and takes the address
// of the element, rather than copying it.
type Command struct {
	Name    string
	Handler Handler
	// Description is the one-line SUMMARY of the command, from its YANG description.
	// Every surface that shows the command on one line reads it.
	Description string
	// LongHelp is the explanation this command's own help page prints, from its
	// ze:help extension. Empty means the command declares no explanation, and
	// the help page then prints the summary alone. It is NEVER read as a
	// summary, and no one-line surface reads it at all.
	LongHelp         string
	ReadOnly         bool             // True if command only reads state (safe for "ze show")
	RequiresSelector bool             // True if command requires an explicit selector instead of implicit/all scope
	ArgDefs          []command.ArgDef // Typed argument definitions from YANG leaves.
}

// TakesInlineSelector reports whether Dispatch would consume an INLINE selector
// token for this command -- the `show bgp peer <selector> detail` shape, where
// the value sits between the resource token and a later action token rather
// than trailing the whole path.
//
// It answers with the dispatcher's OWN predicate (implicitSelectorDef over the
// command's key tokens and ArgDefs) rather than a second hardcoded list, so a
// caller that builds a command string cannot drift from what Dispatch accepts
// (ai/rules/evidence.md). Callers outside the dispatcher -- notably
// the MCP tool generator, which must decide whether to advertise a `peer`
// argument at all and where to splice its value -- rely on this.
//
// A single-token command can never carry an inline selector: matchCommandTokens
// only reaches the implicit-selector branch while keyIdx+1 < len(keyTokens), so
// there is no interior position to splice into.
func (c *Command) TakesInlineSelector() bool {
	if c == nil {
		return false
	}
	keyTokens := strings.Fields(c.Name)
	if len(keyTokens) < 2 {
		return false
	}
	return implicitSelectorDef(keyTokens, c.ArgDefs, nil) != nil
}

// RegisterOptions holds optional settings for command registration.
type RegisterOptions struct {
	ReadOnly         bool             // True if command only reads state
	RequiresSelector bool             // True if the command requires an explicit selector value
	PluginProxy      bool             // True if this builtin proxies to a plugin command (allows plugin to register same name)
	ArgDefs          []command.ArgDef // Typed argument definitions from YANG leaves
	LongHelp         string           // The long explanation the command's own help page prints (empty = none declared)
}

// Dispatcher routes commands to handlers.
type Dispatcher struct {
	commands   map[string]*Command
	sortedKeys []string          // sorted keys for longest-match lookup (longest first)
	registry   *CommandRegistry  // Plugin commands
	pending    *PendingRequests  // In-flight plugin requests
	subsystems *SubsystemManager // Forked subsystem processes
	authorizer aaa.Authorizer    // Authorization checker (nil = allow all)
	accountant aaa.Accountant    // Accounting recorder (nil = disabled)
	audit      audit.Recorder    // Local audit recorder (nil = disabled)
}

// NewDispatcher creates a new command dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		commands:   make(map[string]*Command),
		registry:   newCommandRegistry(),
		pending:    newPendingRequests(),
		subsystems: NewSubsystemManager(),
	}
}

// SetAuthorizer sets the authorization checker for the dispatcher.
// When set, all commands are checked against the authorizer before execution.
func (d *Dispatcher) SetAuthorizer(a aaa.Authorizer) {
	d.authorizer = a
}

// SetAccountingHook sets the accounting recorder for the dispatcher.
// When set, command START/STOP records are sent for every dispatched command.
// Accounting failures never block command execution.
func (d *Dispatcher) SetAccountingHook(h aaa.Accountant) {
	d.accountant = h
}

// SetAuditRecorder sets the structured audit recorder for mutation commands.
func (d *Dispatcher) SetAuditRecorder(recorder audit.Recorder) {
	d.audit = recorder
}

// Subsystems returns the subsystem manager.
func (d *Dispatcher) Subsystems() *SubsystemManager {
	return d.subsystems
}

// setSubsystems sets the subsystem manager.
func (d *Dispatcher) setSubsystems(sm *SubsystemManager) {
	d.subsystems = sm
}

// Registry returns the plugin command registry.
func (d *Dispatcher) Registry() *CommandRegistry {
	return d.registry
}

// Pending returns the pending requests tracker.
func (d *Dispatcher) Pending() *PendingRequests {
	return d.pending
}

// Register adds a builtin command handler.
// Also marks the command as builtin in the registry to prevent shadowing.
func (d *Dispatcher) Register(name string, handler Handler, help string) {
	// Store with lowercase key for case-insensitive matching
	key := strings.ToLower(name)
	d.commands[key] = &Command{
		Name:        name,
		Handler:     handler,
		Description: help,
	}
	d.updateSortedKeys()

	// Mark as builtin to prevent plugin shadowing
	d.registry.AddBuiltin(name)
}

// RegisterWithOptions adds a builtin command handler with additional options.
func (d *Dispatcher) RegisterWithOptions(name string, handler Handler, help string, opts RegisterOptions) {
	key := strings.ToLower(name)
	d.commands[key] = &Command{
		Name:             name,
		Handler:          handler,
		Description:      help,
		LongHelp:         opts.LongHelp,
		ReadOnly:         opts.ReadOnly,
		RequiresSelector: opts.RequiresSelector,
		ArgDefs:          opts.ArgDefs,
	}
	d.updateSortedKeys()

	// Plugin proxy handlers must not block the plugin from registering
	// the same command name in the CommandRegistry. ForwardToPlugin needs
	// the plugin's own registration to route the command to the process.
	if !opts.PluginProxy {
		d.registry.AddBuiltin(name)
	}
}

// updateSortedKeys rebuilds the sorted key list for longest-match lookup.
func (d *Dispatcher) updateSortedKeys() {
	d.sortedKeys = make([]string, 0, len(d.commands))
	for k := range d.commands {
		d.sortedKeys = append(d.sortedKeys, k)
	}
	// Sort by length descending (longest first)
	sort.Slice(d.sortedKeys, func(i, j int) bool {
		return len(d.sortedKeys[i]) > len(d.sortedKeys[j])
	})
}

// matchBuiltinTokens finds the longest registered builtin whose key prefixes
// tokens, and returns it with the tokens it did not consume as arguments.
//
// THE MATCH IS REFUSED WHEN THE TOKENS REACH A REGISTERED COMMAND FURTHER DOWN.
// Longest-prefix alone hands a command registered at a SHORT path the whole
// subtree below it. That subtree includes paths another owner registered as
// commands of their own. `show bgp` is a builtin key. `show bgp rpki status` is
// a plugin name that only the fallback below reaches. So an unguarded match
// sends the rpki subtree to handleBgpSummary, which reads its first argument as
// an address FAMILY. LookupLocal (internal/component/command/registry/registry.go)
// applies the same rule to the client-side lookup, and this is its daemon twin.
//
// A command still keeps every trailing token that names no registered command,
// which is how `show bgp ipv4` reaches its handler with the family. The
// test is a registered PATH, never the presence of leftover tokens: leftovers
// are how every argument-taking command works.
func (d *Dispatcher) matchBuiltinTokens(tokens []string) (*Command, []string, map[string]string, bool) {
	for _, key := range d.sortedKeys {
		cmd := d.commands[key]
		if cmd == nil {
			continue
		}
		args, selectors, ok := matchCommandTokens(tokens, key, cmd.ArgDefs)
		if !ok {
			continue
		}
		if d.longerCommandPath(tokens, len(tokens)-len(args)) {
			return nil, nil, nil, false
		}
		return cmd, args, selectors, true
	}
	return nil, nil, nil, false
}

// longerCommandPath reports whether a prefix of tokens longer than consumed is
// itself a registered command path. consumed is how many input tokens the
// builtin match claimed, so the walk starts one token past it.
//
// The walk stops at the first hit, and it does no work at all for the common
// case where the match consumed every token.
func (d *Dispatcher) longerCommandPath(tokens []string, consumed int) bool {
	if consumed >= len(tokens) {
		return false
	}
	var tb textbuf.Buffer
	for i := range consumed {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str(strings.ToLower(tokens[i]))
	}
	for i := consumed; i < len(tokens); i++ {
		tb.Byte(' ').Str(strings.ToLower(tokens[i]))
		if d.isCommandPath(tb.Bytes()) {
			return true
		}
	}
	return false
}

// isCommandPath reports whether lowerPath names a registered command exactly,
// in any of the three places Dispatch resolves a command from. The four
// `show bgp` subtrees this guard protects are PLUGIN names, so a guard reading
// the builtin keys alone would see none of them.
//
// lowerPath must already be lowercased by the caller.
//
// A nil registry or subsystem manager is not missing data. NewDispatcher builds
// both, so nil means no command of that kind can be registered here.
func (d *Dispatcher) isCommandPath(lowerPath []byte) bool {
	if d.commands[string(lowerPath)] != nil {
		return true
	}
	if d.registry != nil && d.registry.hasCommandPath(string(lowerPath)) {
		return true
	}
	return d.subsystems != nil && d.subsystems.hasCommandPath(string(lowerPath))
}

func matchCommandTokens(tokens []string, key string, defs []command.ArgDef) ([]string, map[string]string, bool) {
	keyTokens := strings.Fields(key)
	if len(keyTokens) == 0 || len(tokens) < len(keyTokens) {
		return nil, nil, false
	}
	defByName := make(map[string]*command.ArgDef, len(defs))
	for i := range defs {
		defByName[strings.ToLower(defs[i].Name)] = &defs[i]
	}
	inIdx := 0
	selectors := make(map[string]string)
	for keyIdx, keyTok := range keyTokens {
		if inIdx >= len(tokens) || !strings.EqualFold(tokens[inIdx], keyTok) {
			return nil, nil, false
		}

		// Explicit typed selectors such as `name <value>` or `id <value>`.
		if def, ok := defByName[strings.ToLower(keyTok)]; ok && inIdx+1 < len(tokens) {
			if keyIdx+1 >= len(keyTokens) || !strings.EqualFold(tokens[inIdx+1], keyTokens[keyIdx+1]) {
				value := tokens[inIdx+1]
				if err := command.ValidateArgString(value, def); err != nil {
					return nil, nil, false
				}
				selectors[def.Name] = value
				inIdx += 2
				continue
			}
		}

		// Generic implicit selectors: a single unmatched selector-like leaf may
		// appear between a resource token and a later action token. A leaf the
		// MODEL anchored to this key token is preferred, because the anchor is
		// the model's own answer to the question implicitSelectorDef guesses at.
		if keyIdx+1 < len(keyTokens) && inIdx+1 < len(tokens) && !strings.EqualFold(tokens[inIdx+1], keyTokens[keyIdx+1]) {
			def := anchoredDef(keyTok, defs, selectors)
			if def == nil {
				def = implicitSelectorDef(keyTokens, defs, selectors)
			}
			if def != nil {
				value := tokens[inIdx+1]
				if err := command.ValidateArgString(value, def); err != nil {
					return nil, nil, false
				}
				selectors[def.Name] = value
				inIdx += 2
				continue
			}
		}

		inIdx++
	}
	if len(selectors) == 0 {
		selectors = nil
	}
	return tokens[inIdx:], selectors, true
}

// implicitSelectorDef returns the one leaf a bare token sitting between two key
// tokens fills, or nil when the model does not say which.
//
// A leaf that states a PATTERN is not a candidate while a pattern-less one is
// available. The inline slot holds a free-form identifier -- an interface name,
// a peer selector -- and a leaf carrying a pattern states a typed value the
// operator reaches by position or by that leaf's own keyword. Without the
// preference, `request interface <name> mac <address>` offers two mandatory
// string leaves, answers nil, and matchCommandTokens then fails the whole match:
// declaring the MAC value its description used to spell in prose would have
// turned a working command into an unknown one.
//
// Ambiguity is still refused. Two pattern-less candidates, or two patterned ones
// with no pattern-less leaf, answer nil as before, so this preference can only
// resolve a case that used to resolve to nothing.
// anchoredDef answers the leaf the model ANCHORED to this key token. It answers
// nil when no leaf names the token, and nil when two do.
//
// An anchor is the model's own statement of which value follows which keyword.
// A leaf declared on a grouping container carries that container's name
// (appendAnchored, internal/component/config/yang/command.go). That name is the
// word the operator types the value after. Reading that answer is not the same
// as deriving a second one from the leaf shapes, which is what the caller falls
// back to (ai/rules/evidence.md).
//
// It is what resolves `peer <selector> announce unicast <prefix>`. That command
// carries two mandatory pattern-less strings, selector and prefix, so
// implicitSelectorDef sees two candidates and answers nil, and the command reads
// as unknown. Only the selector is anchored to `peer`.
//
// Two leaves anchored to one keyword answer nil for the same reason
// implicitSelectorDef does: the model has not said which.
func anchoredDef(keyTok string, defs []command.ArgDef, matched map[string]string) *command.ArgDef {
	var found *command.ArgDef
	for i := range defs {
		def := &defs[i]
		if _, ok := matched[def.Name]; ok {
			continue
		}
		if def.Anchor == "" || !strings.EqualFold(def.Anchor, keyTok) {
			continue
		}
		if found != nil {
			return nil
		}
		found = def
	}
	return found
}

func implicitSelectorDef(keyTokens []string, defs []command.ArgDef, matched map[string]string) *command.ArgDef {
	var loose, patterned *command.ArgDef
	looseCount, patternedCount := 0, 0
	for i := range defs {
		def := &defs[i]
		if _, ok := matched[def.Name]; ok {
			continue
		}
		if def.Kind != command.ArgString || !def.Mandatory {
			continue
		}
		if keyTokenPresent(keyTokens, def.Name) {
			continue
		}
		if def.Pattern != nil {
			patterned, patternedCount = def, patternedCount+1
			continue
		}
		loose, looseCount = def, looseCount+1
	}
	if looseCount == 1 {
		return loose
	}
	if looseCount == 0 && patternedCount == 1 {
		return patterned
	}
	return nil
}

func keyTokenPresent(keyTokens []string, name string) bool {
	for _, token := range keyTokens {
		if strings.EqualFold(token, name) {
			return true
		}
	}
	return false
}

func applyExtractedSelectors(ctx *CommandContext, selectors map[string]string) {
	if ctx == nil {
		return
	}
	if len(selectors) == 0 {
		ctx.Selectors = nil
		return
	}
	ctx.Selectors = make(map[string]string, len(selectors))
	for name, value := range selectors {
		ctx.Selectors[strings.ToLower(name)] = value
	}
	// Compatibility bridge for existing peer-scoped handlers.
	if selector, ok := ctx.Selectors[selectorLeaf]; ok {
		ctx.Peer = selector
	}
}

// Lookup finds a command by exact name.
func (d *Dispatcher) Lookup(name string) *Command {
	return d.commands[strings.ToLower(name)]
}

// Commands returns all registered commands.
func (d *Dispatcher) Commands() []*Command {
	result := make([]*Command, 0, len(d.commands))
	for _, cmd := range d.commands {
		result = append(result, cmd)
	}
	return result
}

// IsAuthorized checks if the user is allowed to execute the command.
// Exported for use by streaming handlers (e.g., monitor) that bypass the normal dispatch path.
func (d *Dispatcher) IsAuthorized(ctx *CommandContext, input string, readOnly bool) bool {
	return d.isAuthorized(ctx, input, readOnly)
}

// BeginAccounting records command START and returns a function that records STOP.
// It is exported for command paths such as streaming handlers that intentionally
// bypass Dispatch but must still share the same AAA accounting hook.
func (d *Dispatcher) BeginAccounting(ctx *CommandContext, input string) func() {
	if d.accountant == nil || ctx == nil || ctx.Username == "" {
		return func() {}
	}
	taskID := d.accountant.CommandStart(ctx.Username, ctx.RemoteAddr, input)
	return func() {
		d.accountant.CommandStop(taskID, ctx.Username, ctx.RemoteAddr, input)
	}
}

// isAuthorized checks if the user is allowed to execute the command.
func (d *Dispatcher) isAuthorized(ctx *CommandContext, input string, readOnly bool) bool {
	authorizer := d.authorizer
	var username, remoteAddr string
	if ctx != nil {
		username = ctx.Username
		remoteAddr = ctx.RemoteAddr
		if ctx.Authorizer != nil {
			authorizer = ctx.Authorizer
		}
	}
	if authorizer == nil {
		return true
	}
	return authorizer.Authorize(username, remoteAddr, input, readOnly)
}

// isAuthorizedCommandArgs checks if the user is allowed to execute a typed
// command dispatch. This must prefer aaa.CommandArgsAuthorizer when available,
// so built-in policy sees the exact command, args, and selector scope.
// aaa.CanonicalCommand is fallback only for legacy string authorizers.
func (d *Dispatcher) isAuthorizedCommandArgs(ctx *CommandContext, command string, args []string, peer string, readOnly bool) bool {
	authorizer := d.authorizer
	var username, remoteAddr string
	if ctx != nil {
		username = ctx.Username
		remoteAddr = ctx.RemoteAddr
		if ctx.Authorizer != nil {
			authorizer = ctx.Authorizer
		}
	}
	if authorizer == nil {
		return true
	}
	if authzArgs, ok := authorizer.(aaa.CommandArgsAuthorizer); ok {
		return authzArgs.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, readOnly)
	}
	return authorizer.Authorize(username, remoteAddr, aaa.CanonicalCommand(command, args, peer), readOnly)
}

// Dispatch parses and executes a command.
// Supports inline selector extraction both for typed forms like
// `show demo name <name> detail` and for positional selector slots that
// appear before a later action token.
func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*plugin.Response, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, ErrEmptyCommand
	}
	if ctx != nil {
		ctx.Selectors = nil
	}

	// Find the longest matching builtin command prefix, allowing selector
	// values to appear inline where the YANG arg metadata expects them.
	matchedCmd, args, selectors, ok := d.matchBuiltinTokens(tokens)
	if ok {
		// Typed-argument validation runs HERE, ahead of the selector guard,
		// because the guard needs its answer: matchCommandTokens fills only
		// INTERIOR selector slots (it stops at a later key token), so a command
		// whose own noun is the TERMINAL key token -- `delete bgp peer
		// <selector>` -- yields no selector at all and the guard below would
		// reject the documented form. validateCommandArgs is the one place that
		// binds a positional token to a leaf, so the guard consults ITS answer
		// rather than re-deriving a second one (ai/rules/evidence.md).
		//
		// Only the RESULT is used early. The error is HELD and reported at its
		// original place in the sequence, after authorization and the flag
		// check, so the message an operator sees is unchanged: a command typed
		// with no arguments at all is still told it "requires a selector"
		// rather than that leaf "selector" is missing.
		var argErr error
		var positional map[string]string
		if len(matchedCmd.ArgDefs) > 0 {
			positional, argErr = validateCommandArgs(args, matchedCmd.ArgDefs, selectors)
		}

		// Adopt the trailing positional as the peer selector, under three fences
		// that together mean no command which resolves today changes meaning:
		// the command must REQUIRE a selector (so the only path altered is one
		// that returns an error), none may have arrived out of band, and
		// validateCommandArgs must have bound the value from a LONE spare token.
		// The last fence is what keeps `announce unicast 10.0.0.0/24 ...` --
		// a single-token command that also carries `leaf selector mandatory` --
		// from announcing to a peer called "unicast".
		if matchedCmd.RequiresSelector && selectors[selectorLeaf] == "" && (ctx == nil || ctx.Peer == "") {
			if value, found := positional[selectorLeaf]; found {
				if selectors == nil {
					selectors = make(map[string]string, 1)
				}
				selectors[selectorLeaf] = value
			}
		}

		applyExtractedSelectors(ctx, selectors)

		explicitSelector := false
		if _, ok := selectors[selectorLeaf]; ok {
			explicitSelector = true
		}
		if matchedCmd.RequiresSelector && !explicitSelector && (ctx == nil || ctx.Peer == "") {
			return nil, fmt.Errorf("%s requires a selector", matchedCmd.Name)
		}

		// Authorization check, after command resolution, before execution.
		if !d.isAuthorized(ctx, input, matchedCmd.ReadOnly) {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  unauthorizedError(input),
			}, ErrUnauthorized
		}

		// Reject flag-shaped leftovers before the handler sees them.
		//
		// matchCommandTokens walks only the KEY's tokens and never checks that
		// the input is exhausted: it returns the unmatched tail as args and
		// reports a successful match. When the matched node has no leaves it has
		// no ArgDefs, so the validation below is skipped and the tail is handed
		// to a handler that may ignore it. `show l2tp --user alice tunnels` then
		// matches `show l2tp`, whose handler takes `_ []string`, and the operator
		// silently gets the summary for the DEFAULT user with exit 0.
		//
		// Only FLAG-shaped tokens are rejected, never leftovers generally: zero
		// ArgDefs does not mean "takes no arguments" (extractArgDefs reads YANG
		// leaf children only, so most nodes have none while their handlers still
		// read positional args), and `| peer X` is folded into trailing args
		// client-side by foldFilters. A leading dash, though, is never valid
		// here: no ArgKind is signed, and folding only emits bare filter names
		// and values. So a flag can only be a client-side flag that leaked.
		if flag := firstFlagToken(args); flag != "" {
			flagErr := fmt.Errorf("unexpected flag %q: flags are interpreted by the client, not the daemon", flag)
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  flagErr.Error(),
			}, flagErr
		}

		// Report the argument-validation verdict computed above. It is raised
		// here, not where it was computed, so authorization and the flag check
		// keep answering first.
		if argErr != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  argErr.Error(),
			}, argErr
		}

		// Execute handler.
		if matchedCmd.Handler == nil {
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}

		// Accounting: record command start/stop (AC-8).
		// Accounting failures never block command execution.
		defer d.BeginAccounting(ctx, input)()

		resp, handlerErr := matchedCmd.Handler(ctx, args)
		d.recordCommandAudit(ctx, input, resp, handlerErr)
		return resp, handlerErr
	}

	// No builtin match: the command is a forked subsystem's, a plugin's, or
	// nobody's. Resolve which BEFORE authorizing, so a command that exists
	// nowhere reports ErrUnknownCommand for every caller. Authorizing first
	// makes any typo come back as an authorization denial for a read-only
	// profile (this path authorizes as a write), which sends the operator to
	// debug their RBAC config for a command that never existed.
	pluginInput := input
	pluginLower := strings.ToLower(strings.TrimSpace(input))
	peerSelector := "*"
	if ctx != nil {
		peerSelector = ctx.PeerSelector()
	}

	var subsystemHandler *SubsystemHandler
	if d.subsystems != nil {
		subsystemHandler = d.subsystems.FindHandler(input)
	}
	if subsystemHandler == nil {
		if matched, _ := d.matchPluginCommand(pluginLower); matched == nil {
			// dispatchPlugin re-resolves and logs the registry contents.
			return d.dispatchPlugin(ctx, pluginInput, pluginLower, peerSelector)
		}
	}

	// The command exists. Authorization applies to these paths too; treat as
	// non-read-only (write), as before.
	if !d.isAuthorized(ctx, input, false) {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  unauthorizedError(input),
		}, ErrUnauthorized
	}

	if subsystemHandler != nil {
		return d.dispatchSubsystem(ctx, subsystemHandler, input)
	}
	return d.dispatchPlugin(ctx, pluginInput, pluginLower, peerSelector)
}

// selectorLeaf is the YANG leaf name every peer-scoped command uses for its
// selector ("Peer selector", e.g. ze-peer-cmd.yang:104). It is the one leaf the
// dispatcher bridges onto CommandContext.Peer, so both the bridge in
// applyExtractedSelectors and the guard in Dispatch name it from here rather
// than spelling the literal twice.
const selectorLeaf = "selector"

// validateCommandArgs implements two-phase validation of command arguments
// against YANG-declared ArgDefs.
//
// Phase 1 (keyword extraction): scan args for tokens matching ArgDef leaf names;
// when found, the next token is validated as that leaf's typed value.
// Phase 2 (positional matching): remaining args are offered to the unmatched
// ArgDefs. Unmatched args pass through to the handler.
// Phase 3 (mandatory check): ArgDefs with Mandatory=true must have been matched.
//
// Phase 3 answers BEFORE phase 2's refusal whenever the call left more tokens
// over than it left definitions open. The refusals are ordered, not the phases:
// a missing mandatory argument is what the model itself says is wrong with the
// call, while a token no definition accepts is a bad value only if the
// dispatcher can say which definition it was typed for.
//
// It returns the leaf a LONE spare positional token filled, keyed by leaf name,
// or nil. Dispatch needs that answer for the terminal-noun selector shape (see
// the fences there), and this is the only place a positional token is bound to a
// leaf, so reporting it is cheaper and safer than a second matcher that would
// drift. "Lone" is the point: when a command leaves several tokens unconsumed,
// which of them is the value is a guess, and the caller must not make it.
func validateCommandArgs(args []string, defs []command.ArgDef, preMatched map[string]string) (map[string]string, error) {
	consumed := make([]bool, len(args))
	matched := make(map[string]bool, len(preMatched))
	defByName := make(map[string]*command.ArgDef, len(defs))
	for i := range defs {
		defByName[defs[i].Name] = &defs[i]
	}
	for name := range preMatched {
		matched[name] = true
	}

	// Phase 1: keyword-value extraction.
	for i := 0; i < len(args); i++ {
		def, ok := defByName[args[i]]
		if !ok {
			continue
		}
		if matched[def.Name] {
			return nil, fmt.Errorf("duplicate keyword %q", args[i])
		}
		consumed[i] = true
		if i+1 >= len(args) {
			return nil, fmt.Errorf("%s requires a value", args[i])
		}
		i++
		consumed[i] = true
		if err := command.ValidateArgString(args[i], def); err != nil {
			return nil, err
		}
		matched[def.Name] = true
	}

	spare := 0
	for i := range consumed {
		if !consumed[i] {
			spare++
		}
	}

	// Phase 2: positional matching for unconsumed args. A token no open
	// definition accepts is kept rather than refused here, because whether it is
	// a bad value or a keyword the handler reads is a question the definitions
	// alone cannot answer, and a later token can still fill a definition this
	// one could not.
	var lone map[string]string
	unplaced := make([]string, 0, len(args))
	for i, arg := range args {
		if consumed[i] {
			continue
		}
		def := positionalDef(arg, defs, matched)
		if def == nil {
			unplaced = append(unplaced, arg)
			continue
		}
		matched[def.Name] = true
		if spare == 1 {
			lone = map[string]string{def.Name: arg}
		}
	}

	// A token that filled no definition is a bad VALUE only when every such
	// token can be attributed to a definition of its own: as many tokens left
	// over as definitions still open, or fewer. More tokens than open
	// definitions means at least one of them is a value for nothing, so it is
	// the keyword half of a group the command's own grammar declares and this
	// validator does not hold (`update <hex>` on `show policy test peer`). The
	// dispatcher then has no ground to name any token as the fault, and the
	// missing mandatory argument below is what is certainly wrong with the call.
	open := unmatchedDefCount(defs, matched)
	if len(unplaced) > 0 && open > 0 && len(unplaced) <= open {
		return nil, positionalError(unplaced[0], defs, matched)
	}

	// Phase 3: mandatory check.
	for i := range defs {
		if defs[i].Mandatory && !matched[defs[i].Name] {
			return lone, fmt.Errorf("required argument missing: %s", defs[i].Name)
		}
	}

	// Every mandatory definition is filled, so a token left over is the only
	// fault the call has, and naming it is the whole answer.
	if len(unplaced) > 0 && open > 0 {
		return nil, positionalError(unplaced[0], defs, matched)
	}

	return lone, nil
}

// positionalDef picks the ArgDef a positional token fills, or nil when none
// accepts it.
//
// EVERY ArgKind is offered the token. The shipped loop tested only ArgEnum,
// ArgUnion and ArgString, which made a mandatory non-string leaf impossible to
// fill positionally: `show tcp-check <host> <port>` skipped the uint16 `port`,
// bound the numeric token to the next STRING leaf, and Phase 3 then rejected a
// fully-formed command with "required argument missing: port".
//
// Mandatory defs are offered the token FIRST. An optional leaf that merely
// accepts the same lexical shape (a pattern-less string accepts anything) would
// otherwise swallow the value a required leaf needed, turning a complete
// command into "required argument missing". Preferring the required leaf can
// only ever fill more of them, never fewer, so this direction cannot invent a
// new failure.
//
// WITHIN a tier the token goes to the definition that constrains it most, and
// a tie goes to the lower name. Slice order decides nothing, which is what lets
// the definitions be reordered for display: `show system sockets 8080` reached
// the port leaf only because the alphabet put "port" before "state", and
// "state" is a pattern-less string that would have accepted it silently.
func positionalDef(arg string, defs []command.ArgDef, matched map[string]bool) *command.ArgDef {
	for _, wantMandatory := range [...]bool{true, false} {
		var best *command.ArgDef
		bestRank := command.ConstraintUnspecified
		for i := range defs {
			def := &defs[i]
			if matched[def.Name] || def.Mandatory != wantMandatory {
				continue
			}
			if command.ValidateArgString(arg, def) != nil {
				continue
			}
			rank := command.Constraint(def)
			if best == nil {
				best, bestRank = def, rank
				continue
			}
			if rank < bestRank {
				best, bestRank = def, rank
				continue
			}
			if rank == bestRank && def.Name < best.Name {
				best = def
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// unmatchedDefCount reports how many ArgDefs are still waiting for a value.
func unmatchedDefCount(defs []command.ArgDef, matched map[string]bool) int {
	n := 0
	for i := range defs {
		if !matched[defs[i].Name] {
			n++
		}
	}
	return n
}

// firstFlagToken returns the first flag-shaped token in args, or "" if there is
// none. Flag-shaped means a leading dash followed by a letter ("-u", "-user",
// "--user"), which no producer of a daemon command emits.
//
// The shape itself is grammar.FlagShaped, so the static gate that hunts a
// client building a flag into a daemon command string (F2,
// internal/le/cligrammar) and this refusal read one definition. A gate judging
// the shape differently from the daemon would pass a command the daemon
// rejects.
func firstFlagToken(args []string) string {
	for _, a := range args {
		if grammar.FlagShaped(a) {
			return a
		}
	}
	return ""
}

// positionalError builds an error for a token no OPEN definition accepts. Its
// caller has already found at least one definition still waiting for a value,
// so the list below is never empty, and it has already established that the
// token can be attributed to one: the caller counts the tokens it could not
// place against the definitions still open (validateCommandArgs).
//
// A definition that is still open is the one the operator's token was meant
// for, so when exactly one is open the error is that definition's own refusal.
// `show route lookup <ip>` publishes a bare value and no keyword, and the
// keyword list answered "valid keywords: ip" for a value that simply was not an
// address: it named a word the grammar never asks anybody to type and dropped
// the reason the value was refused (plan/journal/guard-addition-drops-what-it-refuses.md).
//
// A definition already filled is named by neither branch. It cannot take this
// token, so offering it as a keyword is an answer the dispatcher would reject.
func positionalError(arg string, defs []command.ArgDef, matched map[string]bool) error {
	open := make([]*command.ArgDef, 0, len(defs))
	for i := range defs {
		if !matched[defs[i].Name] {
			open = append(open, &defs[i])
		}
	}
	if len(open) == 1 {
		return command.ValidateArgString(arg, open[0])
	}
	for _, def := range open {
		if def.Kind == command.ArgEnum || def.Kind == command.ArgUnion {
			return command.ValidateArgString(arg, def)
		}
	}
	names := make([]string, 0, len(open))
	for _, def := range open {
		names = append(names, def.Name)
	}
	return fmt.Errorf("unexpected argument %q, valid keywords: %s", arg, textbuf.Join(names, ", "))
}

func (d *Dispatcher) recordCommandAudit(ctx *CommandContext, input string, resp *plugin.Response, err error) {
	action := auditActionForCommand(input)
	if d.audit == nil || action == "" || err != nil {
		return
	}
	if resp != nil && resp.Status == plugin.StatusError {
		return
	}
	entry := audit.Entry{
		Surface: audit.CLI,
		Action:  action,
		Detail:  input,
		Outcome: audit.OutcomeSuccess,
	}
	if ctx != nil {
		entry.Actor = ctx.Username
		entry.RemoteAddr = ctx.RemoteAddr
		if ctx.Surface != "" {
			entry.Surface = ctx.Surface
		}
	}
	if recordErr := d.audit.Record(entry); recordErr != nil {
		logger().Warn("audit record failed", "action", action, "command", input, "error", recordErr)
	}
}

func auditActionForCommand(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "request reload" {
		return audit.ActionDaemonReload
	}
	return ""
}

// dispatchSubsystem routes a command to a forked subsystem process.
func (d *Dispatcher) dispatchSubsystem(ctx *CommandContext, handler *SubsystemHandler, input string) (*plugin.Response, error) {
	return handler.Handle(ctx.Context(), input)
}

// ForwardToPlugin routes a command to a plugin process by exact name lookup.
// Used by proxy handlers that bridge CLI builtins to plugin commands.
// Returns ErrUnknownCommand if the command is not registered (plugin may not be running).
func (d *Dispatcher) ForwardToPlugin(cmdCtx *CommandContext, command string, args []string, peerSelector string) (*plugin.Response, error) {
	cmd := d.registry.Lookup(command)
	if cmd == nil {
		return nil, fmt.Errorf("plugin command %q not registered (plugin may not be running): %w", command, ErrUnknownCommand)
	}
	return d.routeToProcess(cmdCtx, cmd, args, peerSelector)
}

// matchPluginCommand finds the longest registered plugin command that prefixes
// lowerInput on a word boundary, falling back to deprecated aliases. It returns
// the match and the length of the matched prefix, or (nil, 0) when the command
// is registered nowhere.
//
// Split out of dispatchPlugin so Dispatch can answer "does this command exist?"
// without executing it: authorization must not report a denial for a command
// that exists nowhere.
// lowerInput must already be lowercased by the caller.
func (d *Dispatcher) matchPluginCommand(lowerInput string) (*RegisteredCommand, int) {
	var matchedPlugin *RegisteredCommand
	var matchedLen int

	for _, cmd := range d.registry.All() {
		key := cmd.LowerName
		if strings.HasPrefix(lowerInput, key) {
			// Check it's a word boundary
			if len(lowerInput) == len(key) || lowerInput[len(key)] == ' ' {
				if len(key) > matchedLen {
					matchedPlugin = cmd
					matchedLen = len(key)
				}
			}
		}
	}

	// Fall back to deprecated aliases if no primary match found.
	if matchedPlugin == nil {
		matchedPlugin, matchedLen = d.registry.lookupDeprecatedPrefix(lowerInput)
	}
	return matchedPlugin, matchedLen
}

// dispatchPlugin routes a command to a plugin process.
// lowerInput must already be lowercased by the caller (Dispatch).
func (d *Dispatcher) dispatchPlugin(ctx *CommandContext, input, lowerInput, peerSelector string) (*plugin.Response, error) {
	matchedPlugin, matchedLen := d.matchPluginCommand(lowerInput)

	if matchedPlugin == nil {
		all := d.registry.All()
		names := make([]string, len(all))
		for i, c := range all {
			names[i] = c.Name
		}
		logger().Debug("dispatchPlugin: no match", "input", lowerInput, "registry_count", len(all), "registered", textbuf.Join(names, ", "))
		return nil, ErrUnknownCommand
	}

	// Extract remaining args
	remaining := strings.TrimSpace(input[matchedLen:])
	var args []string
	if remaining != "" {
		var err error
		args, err = tokenize(remaining)
		if err != nil {
			return nil, err
		}
	}

	// Route to process
	return d.routeToProcess(ctx, matchedPlugin, args, peerSelector)
}

// routeToProcess sends a command request to a plugin process via synchronous RPC.
//
// The plugin's answer always arrives as a sequence: a head, its records and a
// terminator (PluginConn.SendExecuteCommandAnswer). A streamed answer becomes a
// plugin.Records payload, so the engine forwards the rows a consumer reads and
// never holds the collection (AC-4 and AC-8 of spec-record-answers-1-sdk-path).
// Every other answer is one document and becomes the plugin.RawJSON payload it
// has always been.
func (d *Dispatcher) routeToProcess(cmdCtx *CommandContext, cmd *RegisteredCommand, args []string, peerSelector string) (*plugin.Response, error) {
	proc := cmd.Process
	if proc == nil || !proc.Running() {
		return nil, ErrPluginProcessNotRunning
	}

	conn := proc.Conn()
	if conn == nil {
		return nil, ErrPluginConnectionClosed
	}

	parentCtx := context.Background()
	if cmdCtx != nil {
		parentCtx = cmdCtx.Context()
	}

	// The deadline bounds the ANSWER and not only the call, because a streamed
	// answer is read after this returns. So cancel travels with the walk rather
	// than being deferred here, and every path that does not hand the walk on
	// releases it before it returns.
	rpcCtx, cancel := context.WithTimeout(parentCtx, cmd.Timeout)

	input := &rpc.ExecuteCommandInput{Command: cmd.Name, Args: args, Peer: peerSelector}
	answer, err := conn.SendExecuteCommandAnswer(rpcCtx, input)
	if err != nil {
		cancel()
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("failed to send request: ").Err(err).String()}, nil
	}
	if answer.Type != rpc.AnswerTypeDocument {
		return streamedPluginResponse(cmd.Name, answer, sync.OnceFunc(cancel)), nil
	}

	defer cancel()
	rpcOut, valueErr := pluginipc.ExecuteCommandValue(answer)
	if valueErr != nil {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("failed to read answer: ").Err(valueErr).String()}, nil
	}
	if rpcOut.Status == plugin.StatusError {
		return &plugin.Response{Status: plugin.StatusError, Error: string(rpcOut.Data)}, nil
	}
	// An answer that carried no document leaves Data ABSENT rather than empty.
	// RawJSON("") marshals to `null` (plugin.RawJSON.MarshalJSON), and
	// ResponseJSON only reports "nothing" for a nil Data, so wrapping an empty
	// document here would print `null` to an operator where they previously read
	// nothing. The owner ruled on that spelling directly: returning nil is fine,
	// printing it is not.
	if len(rpcOut.Data) == 0 {
		return &plugin.Response{Status: rpcOut.Status}, nil
	}
	return &plugin.Response{Status: rpcOut.Status, Data: plugin.RawJSON(rpcOut.Data)}, nil
}

// streamedPluginResponse is the response a plugin's streamed answer becomes: the
// rows as they arrive, under the envelope and the column schema the head
// declared.
//
// The status is done, because the head declares no outcome and a streamed
// answer has not ended yet: what the walk turns out to be is stated by the
// terminator, which arrives after the last row. A consumer reads it through the
// walk (rpc.Verdict), exactly as it does for the engine's own answer to a
// failing command (WriteAnswer, internal/component/plugin/dispatch.go).
func streamedPluginResponse(command string, answer *rpc.Answer, release func()) *plugin.Response {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Records{
			Key:    answer.Key,
			Fields: answer.Fields,
			Rows:   pluginAnswerRows(command, answer, release),
		},
	}
}

// pluginAnswerRows yields the rows of a plugin's streamed answer and releases
// the call that carried it when the walk ends.
//
// The walk is the ONE reading of this answer, so the engine forwards each row
// and never holds the collection: a command that walks a million rows costs the
// rows the consumer keeps, which is what bounds `| first 10` over a plugin's
// table. Nothing here starts a goroutine, for the answer or for a row
// (ai/rules/goroutine-lifecycle.md).
//
// release runs exactly once, when the range ends by exhaustion or by the
// consumer stopping it, and it is where the call's deadline is given back. A
// consumer that never ranges at all leaves it to that deadline, which is the
// second reason the call carries one.
//
// Every row is checked before it is forwarded, because a plugin's record is
// untrusted and the parser hands its bytes on unread (rpc.Record). A row that is
// not JSON is rejected rather than forwarded (checkedRecord).
//
// An answer that stopped before its terminator is reported as ONE rejected row
// rather than handed on as complete, so an operator sees that the walk ended
// early instead of reading a short answer as the whole. That row carries a fixed
// sentence and the count: the cause is a Go error and goes to the daemon log,
// because a fault payload reaches an operator (Security Review of
// spec-record-answers-1-sdk-path).
func pluginAnswerRows(command string, answer *rpc.Answer, release func()) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		defer release()

		// The rows arrive as bytes and leave as the appender the engine's
		// writer takes. One row carries them all in turn: the writer appends it
		// before the yield returns and keeps no reference to it (rpc.Row), so
		// forwarding a plugin's table of a million rows allocates for none of
		// them.
		var forwarded rpc.RawRow

		var produced uint64
		for record := range answer.Records {
			produced++
			if !yield(forwardedRow(&forwarded, checkedRecord(produced, record))) {
				return
			}
		}
		// Read after the range, never before: the range is what fills it.
		err := answer.Err()
		if err == nil {
			return
		}
		logger().Warn("plugin answer ended before its terminator",
			"command", command, "records", produced, "error", err)
		forwarded = rpc.RawRow(answerTruncatedFault(produced))
		yield(rpc.RowRecord{Fault: &forwarded})
	}
}

// forwardedRow states one arrived record as the appending row the engine's
// answer writer takes, through the one row this walk refills for each of them.
//
// A record that is neither an item nor a fault cannot arrive here: checkedRecord
// rejects an empty payload as a rejected row, because nothing is not a row
// either.
func forwardedRow(forwarded *rpc.RawRow, record rpc.Record) rpc.RowRecord {
	if len(record.Fault) > 0 {
		*forwarded = rpc.RawRow(record.Fault)
		return rpc.RowRecord{Fault: forwarded}
	}
	*forwarded = rpc.RawRow(record.Item)
	return rpc.RowRecord{Item: forwarded}
}

// checkedRecord is record when its payload is JSON, and the rejected row that
// stands in for it when it is not.
//
// A record line's payload is the plugin's own bytes and the parser
// forwards them unread (rpc.Record), so this is the boundary where an untrusted
// payload is checked before an operator's rendering treats it as JSON. The
// buffered reading of the same answer checks it too, by re-encoding each row
// (rpc.CollapseRecords), so both readings refuse the same row.
//
// Rejecting the row rather than ending the walk is what keeps the rest of the
// answer, exactly as a row too wide for one line does (boundedRecord,
// pkg/plugin/rpc/answer_write.go): refusing one row must not cost the operator
// the rows around it. A record carrying neither an item nor a fault is rejected
// here as well, because nothing is not a row either.
//
// ordinal is the record's position in the walk, counted from one.
func checkedRecord(ordinal uint64, record rpc.Record) rpc.Record {
	payload := record.Item
	if len(record.Fault) > 0 {
		payload = record.Fault
	}
	if json.Valid(payload) {
		return record
	}
	return rpc.Record{Fault: answerRowNotJSONFault(ordinal, len(payload))}
}

// answerRowNotJSONFaultCapacity is the capacity answerRowNotJSONFault builds
// into: 62 bytes of fixed text and two decimal numbers of at most 20 digits
// each, so 128 holds every fault it can write without growing the slice.
const answerRowNotJSONFaultCapacity = 128

// answerRowNotJSONFault is the rejected row that stands in for a plugin record
// whose payload is not JSON. It names the record by its position in the walk and
// states its size, so an operator can find the row that was rejected.
//
// It quotes nothing of the payload. Those bytes are whatever the plugin sent,
// and a fault carrying them would put them in front of the operator through the
// rejection meant to keep them away.
func answerRowNotJSONFault(ordinal uint64, size int) json.RawMessage {
	fault := make([]byte, 0, answerRowNotJSONFaultCapacity)
	fault = append(fault, `{"message":"plugin answer row is not JSON","record":`...)
	fault = strconv.AppendUint(fault, ordinal, 10)
	fault = append(fault, `,"bytes":`...)
	fault = strconv.AppendInt(fault, int64(size), 10)
	return append(fault, '}')
}

// answerTruncatedFaultCapacity is the capacity answerTruncatedFault builds into:
// 66 bytes of fixed text and one decimal number of at most 20 digits, so 96
// holds every fault it can write without growing the slice.
const answerTruncatedFaultCapacity = 96

// answerTruncatedFault is the rejected row that states a plugin answer which
// stopped before its terminator. It names how many records did arrive, so an
// operator can tell an empty answer from a short one.
//
// It quotes no Go error and no path. The text an operator reads must not carry
// either, and the cause is logged beside it for whoever reads the daemon log.
func answerTruncatedFault(produced uint64) json.RawMessage {
	fault := make([]byte, 0, answerTruncatedFaultCapacity)
	fault = append(fault, `{"message":"plugin answer ended before its terminator","records":`...)
	fault = strconv.AppendUint(fault, produced, 10)
	return append(fault, '}')
}

var errBackslashInCommand = errors.New("backslash is not allowed in commands")

// tokenize splits a command string into tokens.
// Handles quoted strings: "hello world" → single token "hello world".
// Backslash is rejected: commands have no escape sequences.
// Quotes are stripped from the result.
func tokenize(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	if strings.IndexByte(input, '\\') >= 0 {
		return nil, errBackslashInCommand
	}

	var tokens []string
	var current textbuf.Buffer
	inQuote := false

	for _, r := range input {
		if r == '"' {
			inQuote = !inQuote
			continue
		}

		if (r == ' ' || r == '\t') && !inQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
			}
			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}
