// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: ensure.go -- ze:ensure-exists chain wrapping for compound commands

package server

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errReactorNotAvailable = errors.New("reactor not available")

// Peer-selector resolution failures (ResolveSinglePeer). Separate sentinels so a
// caller can tell "you gave me a wildcard" from "nothing matched" from "several
// matched" without string matching.
var (
	errNoSpecificPeer        = errors.New("command requires one specific peer")
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

// BuiltinCount returns the number of registered builtin handlers.
func BuiltinCount() int {
	return len(AllBuiltinRPCs())
}

// LoadBuiltins registers all builtin handlers with the dispatcher.
// The wireToPath map provides the dispatch key for each handler, derived from
// the YANG command tree (WireMethod -> CLI path). pathToDesc provides YANG
// descriptions for help text. Handlers without a YANG entry are skipped.
func LoadBuiltins(d *Dispatcher, wireToPath, pathToDesc map[string]string, pathToArgDefs map[string][]command.ArgDef) {
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
		})
	}
}

// LoadBuiltinsWithAliases registers all builtin handlers with the dispatcher,
// including all YANG command aliases for each wire method. When cmdTree is
// non-nil, commands whose YANG path passes through a ze:ensure-exists node
// are wrapped to auto-ensure the parent resource and rollback on failure.
func LoadBuiltinsWithAliases(d *Dispatcher, wireToPaths map[string][]string, pathToDesc map[string]string, pathToArgDefs map[string][]command.ArgDef, cmdTree *command.Node) {
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

// RegisterDefaultHandlers registers all builtin handlers with the dispatcher.
func RegisterDefaultHandlers(d *Dispatcher, wireToPath, pathToDesc map[string]string, pathToArgDefs map[string][]command.ArgDef) {
	LoadBuiltins(d, wireToPath, pathToDesc, pathToArgDefs)
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
	Meta           map[string]any    // Route metadata from UpdateRoute RPC; nil if not set.
	Selectors      map[string]string // Extracted typed selector values, keyed by selector keyword.
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
// (ai/rules/derive-not-hardcode.md).
//
// The wildcard is REFUSED, deliberately and by name. Every caller acts on a
// single session -- injecting raw bytes, pausing a read loop -- and silently
// fanning that across every peer is a footgun, so `*` is rejected with the
// selector quoted rather than being allowed to mean something surprising
// (ai/rules/fail-closed-guards.md). An ambiguous selector that matches several
// peers is refused the same way: this returns ONE peer or an error, never a
// guess at which one was meant.
func ResolveSinglePeer(ctx *CommandContext, action string) (netip.Addr, *plugin.Response, error) {
	r, errResp, err := RequireReactor(ctx)
	if err != nil {
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
	peers := r.Peers()
	var matched []netip.Addr
	for i := range peers {
		if selectorMatchesPeer(parsed, &peers[i]) {
			matched = append(matched, peers[i].Address)
		}
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

// selectorMatchesPeer applies the selector to one peer, mirroring the kind
// handling the peer-listing commands use (cmd/peer's filterPeersBySelectorValue)
// so a selector means the same thing whichever command consumes it.
func selectorMatchesPeer(sel *selector.Selector, p *plugin.PeerInfo) bool {
	switch sel.SelectorKind() {
	case selector.KindName:
		return p.Name == sel.NameValue()
	case selector.KindASN:
		return p.PeerAS == sel.ASNValue()
	default:
		return sel.Matches(p.Address)
	}
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
type Command struct {
	Name             string
	Handler          Handler
	Help             string
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
// (ai/rules/derive-not-hardcode.md). Callers outside the dispatcher -- notably
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
		registry:   NewCommandRegistry(),
		pending:    NewPendingRequests(),
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

// SetSubsystems sets the subsystem manager.
func (d *Dispatcher) SetSubsystems(sm *SubsystemManager) {
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
		Name:    name,
		Handler: handler,
		Help:    help,
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
		Help:             help,
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

func (d *Dispatcher) matchBuiltinTokens(tokens []string) (*Command, []string, map[string]string, bool) {
	for _, key := range d.sortedKeys {
		cmd := d.commands[key]
		if cmd == nil {
			continue
		}
		args, selectors, ok := matchCommandTokens(tokens, key, cmd.ArgDefs)
		if ok {
			return cmd, args, selectors, true
		}
	}
	return nil, nil, nil, false
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
		// appear between a resource token and a later action token.
		if keyIdx+1 < len(keyTokens) && inIdx+1 < len(tokens) && !strings.EqualFold(tokens[inIdx+1], keyTokens[keyIdx+1]) {
			if def := implicitSelectorDef(keyTokens, defs, selectors); def != nil {
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

func implicitSelectorDef(keyTokens []string, defs []command.ArgDef, matched map[string]string) *command.ArgDef {
	var candidate *command.ArgDef
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
		if candidate != nil {
			return nil
		}
		candidate = def
	}
	return candidate
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
	if selector, ok := ctx.Selectors["selector"]; ok {
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
	if d.authorizer == nil {
		return true
	}
	var username, remoteAddr string
	if ctx != nil {
		username = ctx.Username
		remoteAddr = ctx.RemoteAddr
	}
	return d.authorizer.Authorize(username, remoteAddr, input, readOnly)
}

// isAuthorizedCommandArgs checks if the user is allowed to execute a typed
// command dispatch. This must prefer aaa.CommandArgsAuthorizer when available,
// so built-in policy sees the exact command, args, and selector scope.
// aaa.CanonicalCommand is fallback only for legacy string authorizers.
func (d *Dispatcher) isAuthorizedCommandArgs(ctx *CommandContext, command string, args []string, peer string, readOnly bool) bool {
	if d.authorizer == nil {
		return true
	}
	var username, remoteAddr string
	if ctx != nil {
		username = ctx.Username
		remoteAddr = ctx.RemoteAddr
	}
	if authzArgs, ok := d.authorizer.(aaa.CommandArgsAuthorizer); ok {
		return authzArgs.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, readOnly)
	}
	return d.authorizer.Authorize(username, remoteAddr, aaa.CanonicalCommand(command, args, peer), readOnly)
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
		applyExtractedSelectors(ctx, selectors)

		explicitSelector := false
		if _, ok := selectors["selector"]; ok {
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

		// Validate args against YANG-declared types, counting inline selector
		// values extracted during prefix matching as already matched.
		if len(matchedCmd.ArgDefs) > 0 {
			if valErr := validateCommandArgs(args, matchedCmd.ArgDefs, selectors); valErr != nil {
				return &plugin.Response{
					Status: plugin.StatusError,
					Error:  valErr.Error(),
				}, valErr
			}
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

// validateCommandArgs implements two-phase validation of command arguments
// against YANG-declared ArgDefs.
//
// Phase 1 (keyword extraction): scan args for tokens matching ArgDef leaf names;
// when found, the next token is validated as that leaf's typed value.
// Phase 2 (positional matching): remaining args are validated against ArgDefs
// with enum values. Unmatched args pass through to the handler.
// Phase 3 (mandatory check): ArgDefs with Mandatory=true must have been matched.
func validateCommandArgs(args []string, defs []command.ArgDef, preMatched map[string]string) error {
	if len(defs) == 0 {
		return nil
	}

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
			return fmt.Errorf("duplicate keyword %q", args[i])
		}
		consumed[i] = true
		if i+1 >= len(args) {
			return fmt.Errorf("%s requires a value", args[i])
		}
		i++
		consumed[i] = true
		if err := command.ValidateArgString(args[i], def); err != nil {
			return err
		}
		matched[def.Name] = true
	}

	// Phase 2: positional matching for unconsumed args.
	for i, arg := range args {
		if consumed[i] {
			continue
		}
		found := false
		unmatchedDefs := 0
		for j := range defs {
			def := &defs[j]
			if matched[def.Name] {
				continue
			}
			unmatchedDefs++
			if def.Kind == command.ArgEnum || def.Kind == command.ArgUnion {
				if command.ValidateArgString(arg, def) == nil {
					matched[def.Name] = true
					found = true
					break
				}
			}
			if def.Kind == command.ArgString {
				if err := command.ValidateArgString(arg, def); err != nil {
					continue
				}
				matched[def.Name] = true
				found = true
				break
			}
		}
		if unmatchedDefs == 0 {
			continue
		}
		if !found {
			return positionalError(arg, defs)
		}
	}

	// Phase 3: mandatory check.
	for i := range defs {
		if defs[i].Mandatory && !matched[defs[i].Name] {
			return fmt.Errorf("required argument missing: %s", defs[i].Name)
		}
	}

	return nil
}

// firstFlagToken returns the first flag-shaped token in args, or "" if there is
// none. Flag-shaped means a leading dash followed by a letter ("-u", "-user",
// "--user"), which no producer of a daemon command emits.
//
// Deliberately NOT flag-shaped:
//   - "-" alone, a conventional stdin/placeholder token;
//   - "--", the end-of-options marker;
//   - "-5" and friends, so a value that merely looks negative is passed through
//     to the handler rather than rejected here.
func firstFlagToken(args []string) string {
	for _, a := range args {
		rest, ok := strings.CutPrefix(a, "-")
		if !ok {
			continue
		}
		// A second dash is still a flag ("--user"); a lone "--" is not.
		if after, dashdash := strings.CutPrefix(rest, "-"); dashdash {
			rest = after
		}
		if rest == "" {
			continue
		}
		r := rest[0]
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return a
		}
	}
	return ""
}

// positionalError builds an error for an unmatched positional arg.
// Uses the first enum/union ArgDef for a specific message, or lists
// available keyword names when no enum/union exists.
func positionalError(arg string, defs []command.ArgDef) error {
	for i := range defs {
		if defs[i].Kind == command.ArgEnum || defs[i].Kind == command.ArgUnion {
			return command.ValidateArgString(arg, &defs[i])
		}
	}
	names := make([]string, 0, len(defs))
	for i := range defs {
		names = append(names, defs[i].Name)
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
		matchedPlugin, matchedLen = d.registry.LookupDeprecatedPrefix(lowerInput)
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

	rpcCtx, cancel := context.WithTimeout(parentCtx, cmd.Timeout)
	defer cancel()

	rpcOut, err := conn.SendExecuteCommand(rpcCtx, "", cmd.Name, args, peerSelector)
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("failed to send request: ").Err(err).String()}, nil
	}
	if rpcOut != nil {
		if rpcOut.Status == plugin.StatusError {
			return &plugin.Response{Status: plugin.StatusError, Error: string(rpcOut.Data)}, nil
		}
		return &plugin.Response{Status: rpcOut.Status, Data: plugin.RawJSON(rpcOut.Data)}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone}, nil
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
				current.Reset()
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
