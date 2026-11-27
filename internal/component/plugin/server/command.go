// Design: docs/architecture/api/process-protocol.md — plugin process management
// Design: docs/architecture/api/commands.md — the dispatcher takes the line with its pipe chain
// Related: ensure.go -- ze:ensure-exists chain wrapping for compound commands

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/opid"
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
var ErrUnauthorized error = authorizationRefusal{}

type authorizationRefusal struct{}

func (authorizationRefusal) Error() string { return plugin.UnauthorizedMessage }

// PermissionDenied lets transport adapters classify a refusal without importing
// this server or comparing operator-facing text.
func (authorizationRefusal) PermissionDenied() bool { return true }

// unauthorizedError builds the Response.Error for a denied command. Surfaces
// that render Response.Error rather than the returned error still name the
// command, which the operator needs when a whole pipeline is denied.
func unauthorizedError(input string) string {
	var tb textbuf.Buffer
	return tb.Str(plugin.UnauthorizedMessage).Str(": ").Str(config.DisplayCommand(input)).String()
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
// one-line summary each command's ze:help extension declares, and pathToHelp
// the long explanation its YANG description declares. Handlers without a YANG
// entry are skipped. The local-data commands a YANG node declares are loaded
// beside them (loadLocalData). The first name another registration already
// holds stops the load and is returned (ErrCommandHeld).
// A non-nil cmdTree MUST come from the same resolved schema as the maps. It
// carries visibility for builtins, local-data commands and plugin declarations.
// The benchmark-only registerDefaultHandlers wrapper supplies no schema tree.
func LoadBuiltins(d *Dispatcher, wireToPath, pathToDesc, pathToHelp map[string]string, pathToArgDefs map[string][]command.ArgDef, cmdTree *command.Node) error {
	d.registry.setCommandTree(cmdTree)
	for _, reg := range AllBuiltinRPCs() {
		name := wireToPath[reg.WireMethod]
		if name == "" {
			continue // No YANG tree entry (editor-internal)
		}
		if err := d.RegisterWithOptions(name, reg.Handler, pathToDesc[name], RegisterOptions{
			Hidden:                    command.HiddenAtPath(cmdTree, strings.Fields(name)),
			ReadOnly:                  IsReadOnlyPath(name) && !reg.NotReadOnly,
			RequiresSelector:          reg.RequiresSelector,
			AcceptsPeerConnection:     reg.AcceptsPeerConnection,
			AcceptsDestinationSession: reg.AcceptsDestinationSession,
			PluginProxy:               reg.PluginCommand != "",
			ArgDefs:                   pathToArgDefs[name],
			Description:               pathToHelp[name],
			Owner:                     reg.WireMethod,
			Complete:                  reg.Complete,
			AuditAction:               reg.AuditAction,
			HandlerTimeout:            reg.HandlerTimeout,
		}); err != nil {
			return err
		}
	}
	wireToPaths := make(map[string][]string, len(wireToPath))
	for wire, path := range wireToPath {
		wireToPaths[wire] = []string{path}
	}
	return loadLocalData(d, wireToPaths, pathToDesc, pathToHelp, pathToArgDefs, cmdTree)
}

// ErrWireMethodHeld is the refusal for a builtin RPC whose wire method another
// linked builtin already registered.
var ErrWireMethodHeld = errors.New("wire method registered by two builtins")

// loadBuiltinsWithAliases registers the builtin handlers regs with the
// dispatcher, including all YANG command aliases for each wire method, and
// the local-data commands a YANG node declares (loadLocalData). When
// cmdTree is non-nil, commands whose YANG path passes through a
// ze:ensure-exists node are wrapped to auto-ensure the parent resource and
// rollback on failure.
//
// It refuses a wire method two registrations carry (ErrWireMethodHeld) and a
// command name two registrations reach (ErrCommandHeld), so neither owner can
// take the other's command by registering later. The server MUST NOT serve
// after a refusal.
func loadBuiltinsWithAliases(d *Dispatcher, regs []RPCRegistration, wireToPaths map[string][]string, pathToDesc, pathToHelp map[string]string, pathToArgDefs map[string][]command.ArgDef, cmdTree *command.Node) error {
	d.registry.setCommandTree(cmdTree)
	wireToHandler := make(map[string]Handler, len(regs))
	wireSeen := make(map[string]bool, len(regs))
	for _, reg := range regs {
		if wireSeen[reg.WireMethod] {
			return fmt.Errorf("%w: %s", ErrWireMethodHeld, reg.WireMethod)
		}
		wireSeen[reg.WireMethod] = true
		if reg.Handler != nil {
			wireToHandler[reg.WireMethod] = reg.Handler
		}
	}

	for _, reg := range regs {
		paths := wireToPaths[reg.WireMethod]
		if len(paths) == 0 {
			continue
		}
		for _, name := range paths {
			handler := reg.Handler
			if chain := buildEnsureChain(cmdTree, name, wireToHandler); len(chain) > 0 {
				handler = wrapWithEnsureChain(handler, chain)
			}
			if err := d.RegisterWithOptions(name, handler, pathToDesc[name], RegisterOptions{
				Hidden:                    command.HiddenAtPath(cmdTree, strings.Fields(name)),
				ReadOnly:                  IsReadOnlyPath(name) && !reg.NotReadOnly,
				RequiresSelector:          reg.RequiresSelector,
				AcceptsPeerConnection:     reg.AcceptsPeerConnection,
				AcceptsDestinationSession: reg.AcceptsDestinationSession,
				PluginProxy:               reg.PluginCommand != "",
				ArgDefs:                   pathToArgDefs[name],
				Description:               pathToHelp[name],
				Owner:                     reg.WireMethod,
				Complete:                  reg.Complete,
				AuditAction:               reg.AuditAction,
				HandlerTimeout:            reg.HandlerTimeout,
			}); err != nil {
				return err
			}
		}
	}
	return loadLocalData(d, wireToPaths, pathToDesc, pathToHelp, pathToArgDefs, cmdTree)
}

// legacyReadRoots are the noun-first tree roots that answer a read and are not
// canonical verbs. They predate the verb-first grammar and still root commands,
// so authorization must still call them reads. `event` and `command` were
// migrated to `show event list` / `show command list`, so neither is here.
//
// This is not a verb list and it MUST NOT grow one: a new read command is
// rooted at a read verb, which command.IsReadOnlyVerb already answers for
// (docs/architecture/cli/command-verbs.md, "Read verbs and their guarantee").
var legacyReadRoots = map[string]bool{
	"validate": true,
	"help":     true,
	"system":   true,
	"plugin":   true,
	"rib":      true,
}

// IsReadOnlyPath returns true if the command path starts with a read-only verb.
// The verb half of the answer comes from command.Verbs, the canonical
// vocabulary, so a verb whose role changes there changes the authorization
// section this path lands in. The legacy noun-first roots are the rest.
func IsReadOnlyPath(path string) bool {
	verb, _, _ := strings.Cut(path, " ")
	if command.IsReadOnlyVerb(verb) {
		return true
	}
	return legacyReadRoots[verb]
}

// registerDefaultHandlers registers all builtin handlers with the dispatcher.
func registerDefaultHandlers(d *Dispatcher, wireToPath map[string]string) error {
	return LoadBuiltins(d, wireToPath, nil, nil, nil, nil)
}

// Handler processes a command and returns a response.
type Handler func(ctx *CommandContext, args command.ValidatedArgs) (*plugin.Response, error)

// CommandContext provides access to reactor and session state.
// Dependencies are accessed through Server; per-request state is stored directly.
type CommandContext struct {
	Server            *Server           // Gateway to all server state (reactor, dispatcher, etc.)
	Process           *process.Process  // The API process (for session state)
	RequestContext    context.Context   // Request-scoped context from the trusted transport.
	Peer              string            // Peer selector: "*" for all, or specific peer selector value. Empty = "*"
	Username          string            // Authenticated username (empty = no auth, full access)
	RemoteAddr        string            // Remote address of the client (e.g., SSH peer IP:port)
	Surface           string            // Trusted caller surface for audit attribution.
	Authorizer        plugin.Authorizer // Request-carried policy generation; nil uses the dispatcher's live policy.
	Meta              map[string]any    // Route metadata from UpdateRoute RPC; nil if not set.
	SourcePolicyEpoch uint64            // Original received provenance from UpdateRoute RPC, not route metadata.
	PeerOwner         uint64            // Original constrained peer; zero with ConnectionID means unbound.
	ConnectionID      uint64            // Captured physical session, never a destination replay token.
	Selectors         map[string]string // Extracted typed selector values, keyed by selector keyword.
	// Sender says who issued this command, and every path that builds a
	// CommandContext states it: plugin.ProcessSender(proc.Name()) on the plugin
	// server's own dispatch paths, plugin.OperatorSender() on the operator
	// surfaces (dispatch.go, dispatch_registry.go, cmd/ze/hub).
	//
	// A peer's `bus { receive { ... } }` block is evaluated against
	// it, so a guard that puts a message on a peer's wire reads THIS field and
	// nothing else. Process above answers a different question, which is the RPC
	// session the command arrived on, and no guard consults it.
	//
	// The zero value means nobody said. It is refused rather than read as the
	// operator, so a dispatch path added later cannot inherit the operator's
	// authority by omission (ai/rules/evidence.md,
	// internal/component/bgp/reactor/send_permission.go).
	Sender plugin.Sender
	// BusCLIMessage is the normalized cli:<command> [args...] message gated by
	// dispatch-command or dispatch-command-args for a plugin sender. Peer-scoped
	// command handlers check send > to targets against this message (Q-T5).
	BusCLIMessage string
	// BusCLIReportMessage is the bounded cli:<command> label reported in refusal
	// errors and metrics when BusCLIMessage target enforcement refuses a peer.
	BusCLIReportMessage string
	// RendersResidualChain is the caller's rendering statement
	// (plugin.CallerIdentity.RendersResidualChain): true leaves the residual
	// pipe chain to the surface that renders the answer. False, the zero
	// value, is the guard: Dispatch applies that chain, so a caller that says
	// nothing receives the answer the chain asked for.
	RendersResidualChain bool

	// pipeOperations is the declared list of the line Dispatch is running:
	// the operations the command declares, in chain order. Only Dispatch
	// sets it, from the partition, so no caller can hand a handler an
	// operation the line did not hold.
	pipeOperations []command.PipeOperation
}

// PipeOperations answers a copy of the operations the dispatched command
// declares, in the order typed, which its handler computes with before the
// answer exists. A plugin's command receives the same list through the
// request routeToProcess sends.
func (c *CommandContext) PipeOperations() []command.PipeOperation {
	if c == nil {
		return nil
	}
	return append([]command.PipeOperation(nil), c.pipeOperations...)
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

// OpID returns the operation correlation ID attached to this command's context,
// or "" when none is present. Nil-safe.
func (c *CommandContext) OpID() string {
	if c == nil {
		return ""
	}
	return opid.From(c.Context())
}

// OriginContext captures original producer authority for local route commands.
// Ordinary control/read commands keep Context without an extra process token.
// An omitted/mismatched Process is an explicit failed capture, never operator.
func (c *CommandContext) OriginContext() context.Context {
	ctx := c.Context()
	if c == nil {
		return ctx
	}
	name, named := c.Sender.Process()
	if !named {
		return ctx
	}
	if c.Process == nil {
		return plugin.WithOriginProducerContext(ctx, plugin.SelectedOwner{}, errors.New("origin producer process is missing"))
	}
	if c.Process.Name() != name {
		return plugin.WithOriginProducerContext(ctx, plugin.SelectedOwner{}, errors.New("origin producer process does not match sender"))
	}
	owner, err := c.Process.SelectedOwner()
	return plugin.WithOriginProducerContext(ctx, owner, err)
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
// advertises (the selector leaf of ze-raw-cmd.yang). An operator who
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
		if errResp, err := PermitSenderPeer(ctx, matched[0]); err != nil {
			return netip.Addr{}, errResp, err
		}
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
			if errResp, err := PermitSenderPeer(ctx, ip); err != nil {
				return netip.Addr{}, errResp, err
			}
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

// SenderCLIToAll reports whether a plugin CLI sender has an unrestricted
// send > to target set (or whether the caller is not a plugin CLI dispatch).
// Plural peer handlers use this to decide whether "*" may act on all peers or
// must be narrowed to the sender's permitted peers (Q-T5, AC-33).
func SenderCLIToAll(ctx *CommandContext) bool {
	if ctx == nil {
		return true
	}
	if ctx.Server == nil {
		return true
	}
	if ctx.BusCLIMessage == "" {
		return true
	}
	procName, isProc := ctx.Sender.Process()
	// refuse-by-default: guards Dispatcher.Dispatch -- non-process callers (operator or internal RPC) are outside plugin bus target scoping.
	if !isProc {
		return true
	}
	compiled := ctx.Server.DeliveryGraph().Compiled()
	if compiled == nil {
		return true
	}
	senderPath := plugin.EndpointPath(procName)
	f := compiled.Filter(senderPath)
	if f == nil {
		return true
	}
	rule := longestMatchingCLISendRule(f, ctx.BusCLIMessage)
	if rule == nil {
		return false
	}
	return rule.ToAll && !rule.ToNone
}

// PermitSenderPeer enforces the compiled bus send > to target restriction for a
// plugin-dispatched CLI command targeting peer addr (Q-T5, AC-33). Returns nil
// for operator or internal-RPC senders and for non-CLI dispatch paths such as
// update-route (which enforces peer targets through BGP's send permission gate).
func PermitSenderPeer(ctx *CommandContext, addr netip.Addr) (*plugin.Response, error) {
	if ctx == nil {
		return nil, nil
	}
	if ctx.Server == nil {
		return nil, nil
	}
	if ctx.BusCLIMessage == "" {
		return nil, nil
	}
	procName, isProc := ctx.Sender.Process()
	// refuse-by-default: guards ResolveSinglePeer -- non-process callers (operator or internal RPC) are outside plugin bus target scoping.
	if !isProc {
		return nil, nil
	}
	compiled := ctx.Server.DeliveryGraph().Compiled()
	if compiled == nil {
		return nil, nil
	}
	senderPath := plugin.EndpointPath(procName)
	f := compiled.Filter(senderPath)
	if f == nil {
		return nil, nil
	}
	peerPath := "/bgp/ip/" + addr.String()
	ok, reason := compiled.MaySend(senderPath, ctx.BusCLIMessage, peerPath)
	if ok {
		return nil, nil
	}
	rule := longestMatchingCLISendRule(f, ctx.BusCLIMessage)
	if reason == SendRefusedDestination {
		if _, known := compiled.pathToIdx[peerPath]; !known {
			if rule != nil && rule.ToAll && !rule.ToNone {
				return nil, nil
			}
		}
	}
	return refuseSenderPeerTarget(ctx, senderPath, peerPath, rule)
}

// RefuseSenderAllPeers builds the Reason 3 refusal when a restricted plugin CLI
// sender targets "*" and zero peers fall inside its compiled send > to set.
func RefuseSenderAllPeers(ctx *CommandContext) (*plugin.Response, error) {
	procName, _ := ctx.Sender.Process()
	senderPath := plugin.EndpointPath(procName)
	var rule *BusCompiledSendRule
	if ctx != nil && ctx.Server != nil {
		if compiled := ctx.Server.DeliveryGraph().Compiled(); compiled != nil {
			rule = longestMatchingCLISendRule(compiled.Filter(senderPath), ctx.BusCLIMessage)
		}
	}
	return refuseSenderPeerTarget(ctx, senderPath, "*", rule)
}

func refuseSenderPeerTarget(
	ctx *CommandContext,
	senderPath, targetPath string,
	rule *BusCompiledSendRule,
) (*plugin.Response, error) {
	listSource := ""
	if rule != nil {
		listSource = firstOriginSource(rule.ToOrigin)
	}
	reportMsg := ctx.BusCLIReportMessage
	if reportMsg == "" {
		reportMsg = ctx.BusCLIMessage
	}
	refusal := FormatBusTargetRefusal(senderPath, targetPath, reportMsg, listSource)
	ReportBusSendRefusal(senderPath, reportMsg, refusal.Problem(), refusal.Fix())
	return &plugin.Response{Status: plugin.StatusError, Error: refusal.Error()}, refusal
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
	// Only name and ASN selectors need peer metadata; Matches owns address matching.
	//exhaustive:ignore
	switch sel.SelectorKind() {
	case selector.KindName:
		match = p.Name == sel.NameValue()
	case selector.KindASN:
		// A peer whose AS is not yet negotiated matches no AS selector.
		asn, known := p.PeerAS.ASN()
		match = known && asn == sel.ASNValue()
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

// ArgsOrSelector answers the positional arguments a handler reads for the
// value its YANG leaf named leaf declares, whichever form the operator used.
//
// Three forms reach a handler. `<value>` alone is answered as it is. `<leaf>
// <value>`, keyword before value (ai/rules/cli.md), is what the web admin form
// posts (commandArguments, internal/component/web/handler_admin.go): the
// dispatcher validates the pair (command.ValidateArgs) and still hands both
// tokens over, so the keyword is dropped here and the value leads. A value the
// dispatcher bound as a typed selector, because the leaf shares its name with
// a key token of the command (matchCommandTokens), reaches the handler as no
// argument at all and is answered from Selectors. A handler that reads args[0]
// therefore answers every form through this one line. Nil-safe.
func (c *CommandContext) ArgsOrSelector(args []string, leaf string) []string {
	if len(args) >= 2 && strings.EqualFold(args[0], leaf) {
		return args[1:]
	}
	if len(args) > 0 {
		return args
	}
	if value := c.Selector(leaf); value != "" {
		return []string{value}
	}
	return args
}

// Command represents a registered command with metadata.
//
// A Command is past the 160-byte rangeValCopy bound .golangci.yml
// sets. A loop over a []Command therefore ranges by index and takes the address
// of the element, rather than copying it.
type Command struct {
	Name    string
	Handler Handler
	// Owner names the registration that holds this command: the wire method of
	// the builtin RPC that serves it. A second registration of the same name is
	// refused, and the refusal names this owner. Empty means the caller declared
	// no owner, which only a test or a tool surface does.
	Owner string
	// ShortHelp is the one-line SUMMARY of the command, from its YANG ze:help
	// extension. Every surface that shows the command on one line reads it.
	ShortHelp string
	// Description is the explanation this command's own help page prints, from its
	// YANG description statement. Empty means the command declares no explanation, and
	// the help page then prints the summary alone. It is NEVER read as a
	// summary, and no one-line surface reads it at all.
	Description               string
	ReadOnly                  bool             // True if command only reads state (safe for "ze show")
	Hidden                    bool             // Hidden from discovery; full-path execution remains available.
	RequiresSelector          bool             // True if command requires an explicit selector instead of implicit/all scope
	AcceptsPeerConnection     bool             // Registration-owned opt-in for physical peer constraints.
	AcceptsDestinationSession bool             // Opt-in for original destination-session constraints.
	ArgDefs                   []command.ArgDef // Typed argument definitions from YANG leaves.
	// Complete answers the values the command's argument may take, from the
	// daemon's runtime state (arg_completion.go). Nil means the command
	// declares none, and its argument completes from its YANG leaves alone.
	Complete ArgCompleter
	// AuditAction is the audit action a successful run of this command records
	// (recordCommandAudit), from its builtin RPC registration. Empty means the
	// command records no audit entry.
	AuditAction    string
	HandlerTimeout func(command.ValidatedArgs) time.Duration
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
	return implicitSelectorDef(keyTokens, c.ArgDefs) != nil
}

// RegisterOptions holds optional settings for command registration.
type RegisterOptions struct {
	Hidden                    bool                                      // Schema visibility, including hidden ancestors.
	ReadOnly                  bool                                      // True if command only reads state
	RequiresSelector          bool                                      // True if the command requires an explicit selector value
	AcceptsPeerConnection     bool                                      // Registration-owned opt-in for physical peer constraints.
	AcceptsDestinationSession bool                                      // Opt-in for original destination-session constraints.
	PluginProxy               bool                                      // True if this builtin proxies to a plugin command (allows plugin to register same name)
	ArgDefs                   []command.ArgDef                          // Typed argument definitions from YANG leaves
	Description               string                                    // The long explanation the command's own help page prints (empty = none declared)
	Owner                     string                                    // The wire method of the builtin RPC registering the command (empty = none declared)
	Complete                  ArgCompleter                              // The argument completer the builtin RPC declares (nil = none declared)
	AuditAction               string                                    // The audit action a successful run records (empty = no audit entry)
	HandlerTimeout            func(command.ValidatedArgs) time.Duration // Optional per-command execution timeout.
}

// ErrCommandHeld is the refusal Dispatcher.RegisterWithOptions returns for a
// command name another registration already holds.
var ErrCommandHeld = errors.New("command name already registered")

// Dispatcher routes commands to handlers.
type Dispatcher struct {
	commands               map[string]*Command
	sortedKeys             []string          // sorted keys for longest-match lookup (longest first)
	registry               *CommandRegistry  // Plugin commands
	pending                *PendingRequests  // In-flight plugin requests
	subsystems             *SubsystemManager // Forked subsystem processes
	authorizer             aaa.Authorizer    // Authorization checker (nil = allow all)
	accountant             aaa.Accountant    // Accounting recorder (nil = disabled)
	audit                  audit.Recorder    // Local audit recorder (nil = disabled)
	handlerTimeoutOverride time.Duration     // Optional timeout override for tests
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
// Accounting failures never deny execution; bounded queues may delay admission.
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

// Register adds a builtin command handler with no options and no declared
// owner. It refuses a name already held, as RegisterWithOptions does.
func (d *Dispatcher) Register(name string, handler Handler, help string) error {
	return d.RegisterWithOptions(name, handler, help, RegisterOptions{})
}

// RegisterWithOptions adds a builtin command handler with additional options,
// and marks the name builtin in the plugin registry so no plugin shadows it.
//
// A name another registration already holds is refused with ErrCommandHeld,
// naming the holder, and the holder keeps the name. The order init() runs in
// across packages is not controlled, so a silent overwrite would let whichever
// owner registered last take the other's command.
func (d *Dispatcher) RegisterWithOptions(name string, handler Handler, help string, opts RegisterOptions) error {
	// Store with lowercase key for case-insensitive matching.
	key := strings.ToLower(name)
	if holder, held := d.commands[key]; held {
		return heldCommandError(name, holder.Owner)
	}
	d.commands[key] = &Command{
		Name:                      name,
		Handler:                   handler,
		Owner:                     opts.Owner,
		ShortHelp:                 help,
		Description:               opts.Description,
		ReadOnly:                  opts.ReadOnly,
		Hidden:                    opts.Hidden,
		RequiresSelector:          opts.RequiresSelector,
		AcceptsPeerConnection:     opts.AcceptsPeerConnection,
		AcceptsDestinationSession: opts.AcceptsDestinationSession,
		ArgDefs:                   opts.ArgDefs,
		Complete:                  opts.Complete,
		AuditAction:               opts.AuditAction,
		HandlerTimeout:            opts.HandlerTimeout,
	}
	d.updateSortedKeys()

	// Plugin proxy handlers must not block the plugin from registering
	// the same command name in the CommandRegistry. ForwardToPlugin needs
	// the plugin's own registration to route the command to the process.
	if !opts.PluginProxy {
		d.registry.AddBuiltin(name)
	}
	return nil
}

// heldCommandError is the refusal for a name the holder already registered.
// A holder with no declared owner is named as such, never as an empty string.
func heldCommandError(name, holder string) error {
	if holder == "" {
		return fmt.Errorf("%w: %q is held by a registration that declared no owner", ErrCommandHeld, name)
	}
	return fmt.Errorf("%w: %q is held by %s", ErrCommandHeld, name, holder)
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

// RecognizesCommand reports whether Dispatch can resolve input, without running
// authorization, accounting or a handler. Interactive surfaces use this before
// entering the config editor, so an operational command keeps its meaning even
// when its verb is also an editor command. The pipe chain runs over the answer
// or reaches the handler as its declared list, so it does not affect command
// recognition. Safe alongside registry publications;
// builtin registration follows Dispatch's existing startup/freeze contract.
func (d *Dispatcher) RecognizesCommand(input string) bool {
	tokens, pipeAt, quoteOpen, err := command.ScanInvocation(input)
	if pipeAt != noPipe {
		// Recognition reads the command part alone. A recorded pipe remains
		// authoritative when the chain after it holds an unfinished quote or
		// a quoted escape the grammar refuses: that fault belongs to the
		// chain, which every execution route refuses (command.ScanExecution).
		input = strings.TrimSpace(input[:pipeAt])
		tokens, _, quoteOpen, err = command.ScanInvocation(input)
	}
	if err != nil {
		return false
	}
	if quoteOpen {
		return false
	}
	if len(tokens) == 0 {
		return false
	}
	if _, _, _, found := d.matchBuiltinTokens(tokens); found {
		return true
	}
	if d.subsystems != nil {
		if d.subsystems.FindHandler(input) != nil {
			return true
		}
	}
	matched, _ := d.matchPluginCommand(strings.ToLower(strings.TrimSpace(input)))
	return matched != nil
}

func matchCommandTokens(tokens []string, key string, defs []command.ArgDef) ([]string, map[string]string, bool) {
	return command.MatchInvocationPath(tokens, key, defs)
}

// typedCommandHead writes the words of tokens before args, the tail the match
// left as arguments: the command as the operator typed it, a selector value
// inside the path included (`show bgp peer edge1 rib`), each word written in
// the invocation grammar so a value holding a space stays one word. args MUST
// be a tail of tokens, as matchCommandTokens answers it.
func typedCommandHead(tokens, args []string) string {
	var tb textbuf.Buffer
	for i, word := range tokens[:len(tokens)-len(args)] {
		if i > 0 {
			tb.Byte(' ')
		}
		command.WriteInvocationValue(&tb, word)
	}
	return tb.String()
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
// It is what resolves `send bgp <selector> unicast <prefix>`. That command
// carries two mandatory pattern-less strings, selector and prefix, so
// implicitSelectorDef sees two candidates and answers nil, and the command reads
// as unknown. Only the selector is anchored, to `bgp`, the keyword the operator
// types it after.
//
// Two leaves anchored to one keyword answer nil for the same reason
// implicitSelectorDef does: the model has not said which.
func anchoredDef(keyTok string, defs []command.ArgDef, matched map[string]string) *command.ArgDef {
	return command.AnchoredInvocationArg(keyTok, defs, matched)
}

func implicitSelectorDef(keyTokens []string, defs []command.ArgDef) *command.ArgDef {
	return command.ImplicitInvocationArg(keyTokens, defs, nil)
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
//
// input is the line as typed, pipe chain included (T-6 A of
// spec-pipe-chain-honored-on-every-command-surface): a string authorizer
// receives it whole and a typed one its tokens, read by the one grammar.
func (d *Dispatcher) IsAuthorized(ctx *CommandContext, input string, readOnly bool) bool {
	tokens, err := command.TypedLineTokens(input)
	if err != nil {
		// A line the grammar refuses never runs; a typed authorizer, which
		// judges tokens, has none to judge and denies it.
		tokens = nil
	}
	return d.isAuthorized(ctx, input, tokens, readOnly)
}

// BeginAccounting records command START and returns a function that records STOP.
// Streaming callers MUST call it before lookup and authorization and MUST defer
// the returned function until the stream ends, including refused commands.
//
// input is the line as typed, pipe chain included: RFC 8907 Section 8.3 asks
// for a record of every command entered, and the chain is part of what was
// entered. It is masked by config.DisplayCommand first, and a typed
// accountant receives the masked line's tokens as the one grammar reads them
// (command.TypedLineTokens), so a backslash in the chain stays one token.
func (d *Dispatcher) BeginAccounting(ctx *CommandContext, input string) func() {
	if d.accountant == nil {
		return func() {}
	}
	input = config.DisplayCommand(input)
	tokens, err := command.TypedLineTokens(input)
	if err != nil {
		// A malformed command is still accounted, but never executed.
		tokens = []string{input}
	}
	return d.beginAccounting(ctx, input, tokens)
}

// beginAccountingArgs preserves typed arguments without parsing rendered text.
func (d *Dispatcher) beginAccountingArgs(ctx *CommandContext, command string, args []string, peer string) func() {
	if d.accountant == nil {
		return func() {}
	}
	input := aaa.CanonicalCommand(command, args, peer)
	safe := config.DisplayCommand(input)
	if safe != input {
		return d.BeginAccounting(ctx, safe)
	}
	return d.beginAccounting(ctx, input, aaa.CanonicalCommandTokens(command, args, peer))
}

func (d *Dispatcher) beginAccounting(ctx *CommandContext, input string, tokens []string) func() {
	accountant := d.accountant
	var username, remoteAddr string
	if ctx != nil {
		username, remoteAddr = ctx.Username, ctx.RemoteAddr
	}
	if typed, ok := accountant.(aaa.CommandArgsAccountant); ok {
		taskID := typed.CommandStartArgs(username, remoteAddr, tokens)
		return func() {
			typed.CommandStopArgs(taskID, username, remoteAddr, tokens)
		}
	}
	taskID := accountant.CommandStart(username, remoteAddr, input)
	return func() {
		accountant.CommandStop(taskID, username, remoteAddr, input)
	}
}

// isAuthorized checks if the user is allowed to execute the command. input is
// the line as typed, for a string authorizer; tokens are its typed tokens
// (command.DispatchLine.TypedTokens), for a typed one: the command's words,
// then the pipe chain as a pipe token, each operation's name and its argument
// words. The chain tokens are for authorization only. readOnly is the
// command path's own flag, which a chain never changes.
func (d *Dispatcher) isAuthorized(ctx *CommandContext, input string, tokens []string, readOnly bool) bool {
	authorizer := d.authorizer
	var username, remoteAddr string
	if ctx != nil {
		username = ctx.Username
		remoteAddr = ctx.RemoteAddr
		if ctx.Authorizer != nil {
			authorizer = ctx.Authorizer
		}
	}
	// refuse-by-default: guards Dispatcher.Dispatch -- a dispatcher without an installed authorizer allows local callers by contract.
	if authorizer == nil {
		return true
	}
	if typed, ok := authorizer.(aaa.CommandArgsAuthorizer); ok {
		if len(tokens) == 0 {
			return false
		}
		return typed.AuthorizeCommandArgs(username, remoteAddr, tokens[0], tokens[1:], "*", readOnly)
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
	// refuse-by-default: guards Server.dispatchCommandArgsResponse -- a dispatcher without an installed authorizer allows local callers by contract.
	if authorizer == nil {
		return true
	}
	if authzArgs, ok := authorizer.(aaa.CommandArgsAuthorizer); ok {
		return authzArgs.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, readOnly)
	}
	return authorizer.Authorize(username, remoteAddr, aaa.CanonicalCommand(command, args, peer), readOnly)
}

// Dispatch parses and executes a command line, pipe chain included.
// Supports inline selector extraction both for typed forms like
// `show demo name <name> detail` and for positional selector slots that
// appear before a later action token.
//
// The line is accounted and authorized as typed. The operations the command
// declares reach its handler as a list (CommandContext.PipeOperations and
// command.ValidatedArgs.PipeOperations). The residual chain runs over the
// answer, through the same ApplyPipes the operator surfaces run, unless the
// caller said it renders the chain itself (CommandContext.RendersResidualChain).
func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*plugin.Response, error) {
	if ctx != nil {
		enrichedCtx, _ := opid.Ensure(ctx.Context())
		ctx.RequestContext = enrichedCtx
	}
	// RFC 8907 Section 8.3: "TACACS+ client devices MUST be configured to
	// send an accounting start packet for every command entered, irrespective
	// of how the commands were authorized." Resolution and authorization
	// failures are entered commands too.
	if strings.TrimSpace(input) != "" {
		defer d.BeginAccounting(ctx, input)()
	}
	// The line is scanned once, command part and pipe chain, and the chain
	// partitioned for the command (command.ReadDispatchLine): the operations
	// the command declares reach its handler as a list, never as arguments,
	// and the residual chain runs over the answer.
	line, err := command.ReadDispatchLine(input)
	if err != nil {
		refusal, operatorError := errors.AsType[*opererr.Error](err)
		if operatorError {
			return plugin.OperatorErrorResponse(refusal), refusal
		}
		return nil, err
	}
	engineApplies := line.HasAnswerStep() && (ctx == nil || !ctx.RendersResidualChain)
	if engineApplies {
		// Refused before any resolution, so no handler runs for a chain the
		// engine could not apply to its answer.
		if refusal := line.ResidualRefusal(); refusal != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: refusal.Error()}, refusal
		}
	}
	resp, err := d.dispatchLine(ctx, line, input)
	if !engineApplies {
		return resp, err
	}
	return applyAnswerStep(line, resp, err)
}

// applyAnswerStep runs line's residual pipe chain over a handler's answer for
// a caller that does not render it (T-1 of
// spec-pipe-chain-honored-on-every-command-surface): the answer becomes the
// chain's result, a JSON document (a chain with no format, or with `| json`
// or `| raw`), or, when the chain holds a format that prints text, that text
// as a renderedAnswer, which marshals to a JSON string. That is what
// `ze cli -c` prints for the same line. A failed answer, or one with no data,
// belongs to its failure and passes unchanged.
func applyAnswerStep(line command.DispatchLine, resp *plugin.Response, err error) (*plugin.Response, error) {
	if err != nil {
		return resp, err
	}
	if resp == nil || resp.Data == nil {
		return resp, nil
	}
	if resp.Status == plugin.StatusError {
		return resp, nil
	}
	document, failure := plugin.ResponseJSON(resp, nil)
	if failure != nil {
		// A row generator is walked once, so the document it collapsed to
		// replaces it; the caller's flatten reports the same failure again.
		if document != "" {
			resp.Data = plugin.RawJSON(document)
		}
		return resp, nil
	}
	rendered, text, problem := line.ApplyAnswerStep(document)
	if problem != "" {
		chainErr := errors.New(problem)
		return &plugin.Response{Status: plugin.StatusError, Error: problem}, chainErr
	}
	if !text {
		if !json.Valid([]byte(rendered)) {
			chainErr := errors.New("the pipe chain answered no JSON document: add a format operation such as | text to the line")
			return &plugin.Response{Status: plugin.StatusError, Error: chainErr.Error()}, chainErr
		}
		resp.Data = plugin.RawJSON(rendered)
		return resp, nil
	}
	resp.Data = renderedAnswer{text: rendered}
	return resp, nil
}

// renderedAnswer is the data of an answer the engine has already rendered:
// the text a format operation (`| table`, `| text`, `| yaml`, `| ndjson`)
// printed, for a caller that does not render the pipe chain. It marshals to
// that text as one JSON string, which is the data the caller receives (T-1
// and D-6 of spec-pipe-chain-honored-on-every-command-surface).
//
// It is not plugin.RawJSON, whose guard refuses a JSON string: a HANDLER that
// answers finished text hands a rendering surface text that `| yaml` or
// `| table` would print back. This text is the end of the chain instead.
// applyAnswerStep runs only for a caller that renders nothing itself
// (CommandContext.RendersResidualChain is false), so no formatter reads it
// again. The type is unexported so that the engine alone produces it: a
// handler still answers data, and RawJSON still refuses finished text from
// one. Immutable, safe for concurrent use.
type renderedAnswer struct {
	plugin.DataMarker
	text string
}

// MarshalJSON writes the rendered text as one JSON string.
func (r renderedAnswer) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.text)
}

// dispatchLine resolves and runs line, a line Dispatch already accounted and
// read. input is the line as typed: what authorization, audit and the
// refusals name.
func (d *Dispatcher) dispatchLine(ctx *CommandContext, line command.DispatchLine, input string) (*plugin.Response, error) {
	tokens := line.Words()
	if len(tokens) == 0 {
		return nil, ErrEmptyCommand
	}
	declared := line.DeclaredOperations()
	if ctx == nil {
		// The context is the declared list's carrier to a plugin process
		// (routeToProcessWithEvents reads it), so without one the list
		// would be dropped on that route with no error.
		if len(declared) > 0 {
			return nil, errors.New("a command line with declared pipe operations needs a command context to carry them")
		}
	} else {
		ctx.pipeOperations = declared
	}

	if ctx != nil {
		ctx.Selectors = nil
	}

	// Find the longest matching builtin command prefix, allowing selector
	// values to appear inline where the YANG arg metadata expects them.
	matchedCmd, args, selectors, ok := d.matchBuiltinTokens(tokens)
	if ctx != nil {
		if ctx.PeerOwner != 0 {
			if !ok {
				return nil, errors.New("this command cannot bind an original physical peer; dispatch an explicitly registered physical peer command")
			}
			if !matchedCmd.AcceptsPeerConnection {
				return nil, errors.New("this command does not accept a physical peer constraint; dispatch an explicitly registered physical peer command")
			}
		}
	}
	if ctx != nil {
		owner, _ := ctx.Sender.Destination()
		if owner != 0 {
			if !ok {
				return nil, errors.New("command cannot bind an original destination; use a registered destination command")
			}
			if !matchedCmd.AcceptsDestinationSession {
				return nil, errors.New("command does not accept an original destination; use a registered destination command")
			}
		}
	}
	if ok {
		// Typed-argument validation runs HERE, ahead of the selector guard,
		// because the guard needs its answer: matchCommandTokens fills only
		// INTERIOR selector slots (it stops at a later key token), so a command
		// whose own noun is the TERMINAL key token -- `delete bgp peer
		// <selector>` -- yields no selector at all and the guard below would
		// reject the documented form. command.ValidateArgs is the one place that
		// binds a positional token to a leaf, so the guard consults ITS answer
		// rather than re-deriving a second one (ai/rules/evidence.md).
		//
		// Only the RESULT is used early. The error is HELD and reported at its
		// original place in the sequence, after authorization and the flag
		// check, so the message an operator sees is unchanged: a command typed
		// with no arguments at all is still told it "requires a selector"
		// rather than that leaf "selector" is missing.
		//
		// It runs for every matched command, including one whose model declares
		// no definitions: that call passes the tokens through, and the handler
		// is invoked only with the value a successful call returned.
		//
		// A word the command declares as a pipe operation is refused first,
		// with the pipe form of the line as the fix: an operation is typed
		// after a pipe and reaches the handler in its declared list, so no
		// line carries one as an argument. The fix starts with the command
		// as typed, so a selector value inside the path stays where the
		// operator put it.
		var validated command.ValidatedArgs
		argErr := command.RefusePipeOperationWords(matchedCmd.Name, typedCommandHead(tokens, args), args)
		if argErr == nil {
			validated, argErr = command.ValidateArgs(args, matchedCmd.ArgDefs, selectors)
		}
		validated = validated.WithPipeOperations(declared)

		// Adopt the trailing positional as the peer selector, under three fences
		// that together mean no command which resolves today changes meaning:
		// the command must REQUIRE a selector (so the only path altered is one
		// that returns an error), none may have arrived out of band, and
		// command.ValidateArgs must have bound the value from a LONE spare token.
		// The last fence is what keeps `send bgp unicast 10.0.0.0/24 ...` --
		// a command that also carries `leaf selector mandatory` -- from taking
		// the prefix as the destination.
		adopted := ""
		if matchedCmd.RequiresSelector && selectors[selectorLeaf] == "" && (ctx == nil || ctx.Peer == "") {
			if value, found := adoptablePositional(validated, argErr); found {
				if selectors == nil {
					selectors = make(map[string]string, 1)
				}
				selectors[selectorLeaf] = value
				adopted = value
			}
		}

		// A line refused with a list of faults (several faults, one value
		// its leaf refuses, or one missing mandatory argument) is restated as every route restates it
		// (command.ArgumentFaults.RestatedForLine), with what only the
		// dispatcher knows: the command's path, where the selector is typed,
		// and the line that supplies a missing one. The guard below still
		// reads it as a list, and the held error keeps its place.
		if faults, listed := errors.AsType[*command.ArgumentFaults](argErr); listed {
			argErr = restateArgumentFaults(matchedCmd, faults, args, selectors[selectorLeaf], adopted)
		}

		applyExtractedSelectors(ctx, selectors)

		explicitSelector := false
		if _, ok := selectors[selectorLeaf]; ok {
			explicitSelector = true
		}
		// One guard per input: the selector guard speaks only when the selector
		// is the fault. When the held argErr names a fault of its own (a keyword
		// with no value, a value its leaf refuses, a word no leaf takes), that
		// fault is why no lone token was left to adopt, and fixing it lets the
		// selector resolve; the guard's "requires a selector" would name a
		// selector the operator typed. The held error is then reported at its
		// place below, after authorization and the flag check, and the handler
		// never runs on it.
		if matchedCmd.RequiresSelector && !explicitSelector && (ctx == nil || ctx.Peer == "") {
			if !argFaultOutsideSelector(argErr) {
				// Several unbound words need the guard's explanation of why no
				// selector was chosen. A typed lone missing selector with no
				// ambiguous payload keeps the shared refusal already restated
				// above, including its cause, declared purpose and whole fix.
				missing, found := errors.AsType[*command.MissingArgumentError](argErr)
				if !found {
					return nil, missingSelectorError(matchedCmd, validated.PositionalTokens(), nil)
				}
				unplaced := missing.UnplacedTokens()
				if len(unplaced) > 1 {
					return nil, missingSelectorError(matchedCmd, unplaced, missing)
				}
			}
		}

		// Authorization check, after command resolution, before execution.
		if !d.isAuthorized(ctx, input, line.TypedTokens(), matchedCmd.ReadOnly) {
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
		// no ArgDefs, so ValidateArgs (which runs for every matched command)
		// iterates nothing and passes the tail through unchanged to a handler
		// that may ignore it. `show l2tp --user alice tunnels` then
		// matches `show l2tp`, whose handler takes `_ command.ValidatedArgs`, and the operator
		// silently gets the summary for the DEFAULT user with exit 0.
		//
		// Only FLAG-shaped tokens are rejected, never leftovers generally: zero
		// ArgDefs does not mean "takes no arguments" (extractArgDefs reads YANG
		// leaf children only, so most nodes have none while their handlers still
		// read positional args). The pipe chain never reaches args: the
		// partition took it off the line, and a declared operation reaches
		// the handler in its list. A leading dash, though, is never valid
		// here: no ArgKind is signed. So a flag can only be a client-side flag
		// that leaked.
		flagArgs := args
		if argErr == nil {
			// Typed keyword/rest values are data. Only tokens that the
			// validator left positional can be leaked client flags.
			flagArgs = validated.PositionalTokens()
		}
		if flag := firstFlagToken(flagArgs); flag != "" {
			flagErr := flagRefusal(matchedCmd.Name, flag, input)
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  flagErr.Error(),
			}, flagErr
		}

		// Report the argument-validation verdict computed above. It is raised
		// here, not where it was computed, so authorization and the flag check
		// keep answering first.
		if argErr != nil {
			// Only an exact local-data registration shares the local route's
			// help hint. Other handlers already own their complete model fix.
			if handler, rest := command.LookupLocalData(strings.Fields(matchedCmd.Name)); handler != nil {
				if len(rest) == 0 {
					argErr = command.ArgumentRefusal(matchedCmd.Name, argErr)
				}
			}
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  argErr.Error(),
			}, argErr
		}

		// A streaming-only command has no one-shot answer: refuse it here,
		// where every one-shot surface dispatches, rather than run a handler
		// whose reply would carry no data.
		if refusal := streamingOnlyRefusal(matchedCmd.Name, input); refusal != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  refusal.Error(),
			}, refusal
		}

		// Execute handler.
		if matchedCmd.Handler == nil {
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}

		resp, handlerErr := d.invokeHandlerBounded(ctx, matchedCmd, validated)
		d.recordCommandAudit(ctx, matchedCmd.AuditAction, input, resp, handlerErr)
		if shapeErr := checkOverrideAnswer(matchedCmd.Name, declared, resp, handlerErr); shapeErr != nil {
			return plugin.OperatorErrorResponse(shapeErr), shapeErr
		}
		return resp, handlerErr
	}

	// No builtin match: the command is a forked subsystem's, a plugin's, or
	// nobody's. Resolve which BEFORE authorizing, so a command that exists
	// nowhere reports ErrUnknownCommand for every caller. Authorizing first
	// makes any typo come back as an authorization denial for a read-only
	// profile (this path authorizes as a write), which sends the operator to
	// debug their RBAC config for a command that never existed.
	//
	// These routes read the command part alone: the residual chain runs over
	// their answer, and a declared operation travels in ctx.
	commandPart := line.Command()
	pluginInput := commandPart
	pluginLower := strings.ToLower(commandPart)
	peerSelector := "*"
	if ctx != nil {
		peerSelector = ctx.PeerSelector()
	}

	var subsystemHandler *SubsystemHandler
	if d.subsystems != nil {
		subsystemHandler = d.subsystems.FindHandler(commandPart)
	}
	if subsystemHandler == nil {
		if matched, _ := d.matchPluginCommand(pluginLower); matched == nil {
			// dispatchPlugin re-resolves and logs the registry contents.
			return d.dispatchPlugin(ctx, pluginInput, pluginLower, peerSelector)
		}
	}

	// The command exists. Authorization applies to these paths too; treat as
	// non-read-only (write), as before.
	if !d.isAuthorized(ctx, input, line.TypedTokens(), false) {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  unauthorizedError(input),
		}, ErrUnauthorized
	}

	if subsystemHandler != nil {
		return d.dispatchSubsystem(ctx, subsystemHandler, commandPart)
	}
	return d.dispatchPlugin(ctx, pluginInput, pluginLower, peerSelector)
}

// adoptablePositional answers the value command.ValidateArgs bound to the
// selector leaf from a lone spare token: from the value it returned, or from
// its refusal when the call left another mandatory leaf missing, so that call
// is still told which leaf is missing rather than that it "requires a
// selector". A line that left several mandatory leaves missing and holds no
// other fault (*command.ArgumentFaults of missing arguments only) binds the
// same way, because every one of its missing arguments carries the same
// binding. Any other refusal binds nothing, a list holding a fault that is not
// a missing argument included (a lone value its leaf refuses is such a list of
// one): that fault sits beside the binding, and the operator must repair it
// first.
func adoptablePositional(validated command.ValidatedArgs, argErr error) (string, bool) {
	if argErr == nil {
		return validated.Positional(selectorLeaf)
	}
	if faults, listed := errors.AsType[*command.ArgumentFaults](argErr); listed {
		for _, fault := range faults.Faults() {
			if _, missing := errors.AsType[*command.MissingArgumentError](fault); !missing {
				return "", false
			}
		}
	}
	if missing, ok := errors.AsType[*command.MissingArgumentError](argErr); ok {
		return missing.Positional(selectorLeaf)
	}
	return "", false
}

// argFaultOutsideSelector reports whether argErr, the verdict
// command.ValidateArgs gave a command the selector guard is about to refuse,
// names a fault other than the absent selector. A *MissingArgumentError is the
// one verdict that does not: it is what a line with nothing after the path
// gets ("delete bgp peer"), and what a line whose values carry no keyword gets,
// and for both the guard's refusal names the fix. Every other verdict names the
// token or the leaf the operator must repair. An *ArgumentFaults containing
// only the missing selector still belongs to that guard. Every other list
// names its own fault, even when every fault it holds is a missing argument: the guard's
// refusal names the selector alone, while the list (restated by
// restateArgumentFaults) names each missing leaf, the missing selector by the
// guard's own form, so an operator who supplies the selector is not refused
// again for the next leaf.
func argFaultOutsideSelector(argErr error) bool {
	if argErr == nil {
		return false
	}
	if faults, listed := errors.AsType[*command.ArgumentFaults](argErr); listed {
		members := faults.Faults()
		if len(members) != 1 {
			return true
		}
		missing, found := errors.AsType[*command.MissingArgumentError](members[0])
		// refuse-by-default: guards Dispatcher.Dispatch -- any non-MissingArgumentError fault is outside the selector guard and reports its own refusal.
		if !found {
			return true
		}
		// The selector guard still chooses the ambiguous-payload diagnostic.
		// Without that ambiguity it preserves this typed shared refusal.
		return missing.Leaf != selectorLeaf
	}
	if _, missing := errors.AsType[*command.MissingArgumentError](argErr); missing {
		return false
	}
	return true
}

// selectorLeaf is the YANG leaf name every peer-scoped command uses for its
// selector (e.g. the peer container of ze-peer-cmd.yang). It is the one leaf the
// dispatcher bridges onto CommandContext.Peer, so both the bridge in
// applyExtractedSelectors and the guard in Dispatch name it from here rather
// than spelling the literal twice.
const selectorLeaf = "selector"

// flagRefusal is the answer to a flag that reached the daemon. A command with
// a local-data handler under the same path also runs in the operator's own
// process, where its flags are read, so the refusal names that `ze` command;
// any other command has no flag at all, and the refusal says only that.
func flagRefusal(path, flag, input string) error {
	var tb textbuf.Buffer
	tb.Str("unexpected flag ").Quoted(flag).Str(": flags are interpreted by the client, not the daemon")
	if handler, rest := command.LookupLocalData(strings.Fields(path)); handler != nil && len(rest) == 0 {
		tb.Str("; run 'ze ").Str(input).Str("' on the host that holds what it reads")
	}
	return errors.New(tb.String())
}

// checkOverrideAnswer holds a builtin's answer to the shape of the generic its
// last declared operation overrides (command.CheckOverrideAnswer), on the
// dispatcher's route as ServeLocal does on the local one. path is the command
// path, declared the list the handler was handed. Only a complete answer is
// flattened to be judged. A row generator is walked once, so it is judged
// without being consumed (command.AnswerStreamed): what it delivers is rows,
// which a `count` override never answers, and its rows are not counted.
func checkOverrideAnswer(path string, declared []command.PipeOperation, resp *plugin.Response, handlerErr error) *opererr.Error {
	if len(declared) == 0 {
		return nil
	}
	outcome := command.AnswerDone
	document := ""
	if handlerErr != nil || resp == nil || resp.Status == plugin.StatusError {
		outcome = command.AnswerFailed
	} else if _, generated := plugin.RecordRows(resp); generated {
		outcome = command.AnswerStreamed
	} else {
		flat, failure := plugin.ResponseJSON(resp, nil)
		if failure != nil {
			outcome = command.AnswerFailed
		}
		document = flat
	}
	err := command.CheckOverrideAnswer(path, declared, outcome, document)
	if err == nil {
		return nil
	}
	refusal, operatorError := errors.AsType[*opererr.Error](err)
	if !operatorError {
		return opererr.Wrap(err, "the answer of "+path+" does not keep the shape of the operation it overrides",
			"an override keeps the generic's answer shape", "show command help \""+path+"\"")
	}
	return refusal
}

// pluginPipe is the wire form of a declared list, ExecuteCommandInput.Pipe:
// each operation's name and its decoded argument words, in chain order. It
// answers nil for an empty list, because nil is what the wire states as "the
// line declared none", and an operation typed with no argument keeps nil Args
// (the argumentWords rule of the partition). It copies, so the request holds no
// slice the dispatcher's context still owns.
func pluginPipe(declared []command.PipeOperation) []rpc.PipeOperation {
	if len(declared) == 0 {
		return nil
	}
	pipe := make([]rpc.PipeOperation, 0, len(declared))
	for _, operation := range declared {
		pipe = append(pipe, rpc.PipeOperation{Name: operation.Name, Args: slices.Clone(operation.Args)})
	}
	return pipe
}

// firstFlagToken returns the first flag-shaped token in args, or "" if there is
// none. Flag-shaped means a leading dash followed by a letter ("-u", "-user",
// "--user"), which no producer of a daemon command emits.
//
// The shape itself is grammar.FlagShaped, so the static gate that hunts a
// client building a flag into a daemon command string (F2,
// internal/le/cli/grammar) and this refusal read one definition. A gate judging
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

// recordCommandAudit records one audit entry for a builtin command that ran,
// under action, the audit action its registration declares
// (RPCRegistration.AuditAction). An empty action records nothing, and so does
// a handler that failed or refused: the log records what changed state. The
// detail is the command line through config.DisplayCommand, so a secret or a
// `content` value reads as the placeholder.
func (d *Dispatcher) recordCommandAudit(ctx *CommandContext, action, input string, resp *plugin.Response, err error) {
	if d.audit == nil || action == "" || err != nil {
		return
	}
	if resp != nil && resp.Status == plugin.StatusError {
		return
	}
	input = config.DisplayCommand(input)
	entry := audit.Entry{
		Surface: audit.CLI,
		Action:  action,
		Detail:  input,
		Outcome: audit.OutcomeSuccess,
	}
	if ctx != nil {
		entry.Actor = ctx.Username
		entry.RemoteAddr = ctx.RemoteAddr
		entry.OpID = ctx.OpID()
		if ctx.Surface != "" {
			entry.Surface = ctx.Surface
		}
	}
	if recordErr := d.audit.Record(entry); recordErr != nil {
		logger().Warn("audit record failed", "code", "doctor-audit-record-failed", "action", action, "command", input, "error", recordErr)
	}
}

// dispatchSubsystem routes a command to a forked subsystem process.
//
// The subsystem receives the command as typed, one string, so the judgment is
// over the tokens after the command the subsystem declared, against the leaves
// the model declares for it, and a refused call never reaches the process. The
// string sent is the one whose tokens were judged. The command's declared pipe
// operations travel beside it, as they do to a plugin process (routeToProcess).
func (d *Dispatcher) dispatchSubsystem(ctx *CommandContext, handler *SubsystemHandler, input string) (*plugin.Response, error) {
	path, rest := handler.matchCommand(input)
	args, err := tokenize(rest)
	if err != nil {
		return nil, err
	}
	if _, argErr := command.ValidateModelArgs(path, args, nil); argErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: argErr.Error()}, argErr
	}
	return handler.Handle(ctx.Context(), input, pluginPipe(ctx.PipeOperations()))
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

// ForwardToPluginAfterEvents applies the captured command owner's queued events
// before executing a command that reads their projection. The same owner and
// connection carry both operations under one deadline. Callers MUST NOT hold
// locks needed by event handlers or call from that owner's event callback.
func (d *Dispatcher) ForwardToPluginAfterEvents(cmdCtx *CommandContext, command string, args []string, peerSelector string) (*plugin.Response, error) {
	cmd := d.registry.Lookup(command)
	if cmd == nil {
		return nil, fmt.Errorf("plugin command %q not registered (plugin may not be running): %w", command, ErrUnknownCommand)
	}
	return d.routeToProcessWithEvents(cmdCtx, cmd, args, peerSelector, true)
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
	all := d.registry.All()
	if i := longestReach(lowerInput, len(all), func(i int) string { return all[i].LowerName }); i >= 0 {
		return all[i], len(all[i].LowerName)
	}
	// Fall back to deprecated aliases if no primary match found.
	return d.registry.lookupDeprecatedPrefix(lowerInput)
}

// longestReach answers the index of the longest of count command keys that
// reaches lowerInput (commandReaches), or -1 when none does; of two keys of
// one length the first is answered. key answers the lower-case key at an
// index.
//
// It is the one rule a plugin command is routed by: the dispatcher sends a
// line to the plugin it answers (matchPluginCommand), and the Stage 1 check
// command-nodes-served calls a command node served when it answers a name of
// the plugin that brought the node (commandServed), so the two cannot
// disagree.
func longestReach(lowerInput string, count int, key func(int) string) int {
	matched := -1
	matchedLen := 0
	for i := range count {
		candidate := key(i)
		if !commandReaches(candidate, lowerInput) {
			continue
		}
		if len(candidate) > matchedLen {
			matched = i
			matchedLen = len(candidate)
		}
	}
	return matched
}

// commandReaches reports whether the command key reaches lowerInput on a word
// boundary: the line is key, or starts with key followed by a space. An empty
// key reaches nothing, as no registered command has one.
func commandReaches(key, lowerInput string) bool {
	if key == "" {
		return false
	}
	if !strings.HasPrefix(lowerInput, key) {
		return false
	}
	return len(lowerInput) == len(key) || lowerInput[len(key)] == ' '
}

// dispatchPlugin routes a command to a plugin process.
// lowerInput must already be lowercased by the caller (Dispatch).
func (d *Dispatcher) dispatchPlugin(ctx *CommandContext, input, lowerInput, peerSelector string) (*plugin.Response, error) {
	matchedPlugin, matchedLen := d.matchPluginCommand(lowerInput)

	if matchedPlugin == nil {
		// A prefix of registered commands is refused with the whole forms, so
		// stopping before the selector never reaches an all-effect.
		if err := d.incompleteCommand(lowerInput); err != nil {
			return nil, err
		}
		all := d.registry.All()
		names := make([]string, len(all))
		for i, c := range all {
			names[i] = c.Name
		}
		// The line is unmatched, so nothing says which of its words is a
		// secret: it is logged through config.DisplayCommand, as the audit
		// detail is, so a mistyped `update data key <k> content <base64>` or a
		// password never reaches the log in clear.
		logger().Debug("dispatchPlugin: no match", "input", config.DisplayCommand(lowerInput), "registry_count", len(all), "registered", textbuf.Join(names, ", "))
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
	return d.routeToProcessWithEvents(cmdCtx, cmd, args, peerSelector, false)
}

func (d *Dispatcher) routeToProcessWithEvents(cmdCtx *CommandContext, cmd *RegisteredCommand, args []string, peerSelector string, applyEvents bool) (*plugin.Response, error) {
	// Route R9, every plugin-process command: the tokens are judged against
	// the leaves the model declares for the plugin's command, before any
	// process is asked, and only the judged tokens are sent. A builtin proxy
	// forwarding here is judged again for the plugin's path, with the
	// selectors the dispatcher already bound.
	var bound map[string]string
	if cmdCtx != nil {
		bound = cmdCtx.Selectors
	}
	validated, argErr := command.ValidateModelArgs(cmd.LowerName, args, bound)
	if argErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: argErr.Error()}, argErr
	}

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

	if applyEvents {
		if err := d.drainCommandEvents(rpcCtx, cmd, proc, conn); err != nil {
			cancel()
			return nil, opererr.Wrap(err, "Cannot run "+cmd.Name+" on plugin "+proc.Name(),
				"Queued events were not successfully applied by the same plugin owner, so the command was not sent",
				"ze show errors")
		}
	}

	// The declared list travels beside the arguments, never inside them: the
	// plugin's handler computes with it, and the engine runs the rest of the
	// chain over the answer.
	input := &rpc.ExecuteCommandInput{
		Command: cmd.Name,
		Args:    validated.Tokens(),
		Peer:    peerSelector,
		Pipe:    pluginPipe(cmdCtx.PipeOperations()),
	}
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
	return documentPluginResponse(cmd.Name, answer)
}

// drainCommandEvents never substitutes a replacement process or connection for
// the owner whose queue was drained. No registry or process lock spans the wait.
func (d *Dispatcher) drainCommandEvents(ctx context.Context, cmd *RegisteredCommand, proc *process.Process, conn *pluginipc.PluginConn) error {
	if err := proc.DrainEventsApplied(ctx); err != nil {
		return err
	}
	current := d.registry.Lookup(cmd.Name)
	if current == nil {
		return ErrPluginProcessNotRunning
	}
	if current.Process != proc {
		return ErrPluginProcessNotRunning
	}
	if !proc.Running() {
		return ErrPluginProcessNotRunning
	}
	if proc.Conn() != conn {
		return ErrPluginConnectionClosed
	}
	return nil
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
	response := &plugin.Response{Status: plugin.StatusDone}
	response.Data = plugin.Records{
		Key:    answer.Key,
		Fields: answer.Fields,
		Rows:   pluginAnswerRows(command, answer, release, response),
	}
	return response
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
func pluginAnswerRows(command string, answer *rpc.Answer, release func(), response *plugin.Response) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		defer release()

		// The rows arrive as bytes and leave as the appender the engine's
		// writer takes. One row carries them all in turn: the writer appends it
		// before the yield returns and keeps no reference to it (rpc.Row), so
		// forwarding a plugin's table of a million rows allocates for none of
		// them.
		var forwarded rpc.RawRow

		var produced, count, faults uint64
		for record := range answer.Records {
			produced++
			checked := checkedRecord(produced, record)
			if len(checked.Fault) > 0 {
				faults++
			} else {
				count++
			}
			if !yield(forwardedRow(&forwarded, checked)) {
				return
			}
		}
		// Read after the range, never before: the range is what fills it.
		err := answer.Err()
		var metadata *rpc.ExecuteCommandRecordAnswer
		if err == nil {
			metadata, err = answer.RecordAnswerMetadata()
		}
		if err != nil {
			reportTruncatedPluginAnswer(command, produced, err)
			forwarded = rpc.RawRow(answerTruncatedFault(produced))
			if !yield(rpc.RowRecord{Fault: &forwarded}) {
				return
			}
			faults++
			metadata = &rpc.ExecuteCommandRecordAnswer{}
		}
		// Counts describe the validated rows forwarded above. An invalid row
		// became a refused row, and a missing end added one sanitized fault.
		metadata.Count, metadata.Faults = count, faults
		if err := response.CompleteRecordAnswer(metadata); err != nil {
			response.Status = plugin.StatusError
			response.Error = opererr.New("Cannot read the outcome of "+command,
				"The plugin returned inconsistent record metadata, so completion cannot be confirmed",
				"ze show errors").Error()
			logger().Warn("plugin answer outcome is invalid",
				"command", command, "error", err, "code", CodeDoctorPluginAnswerUndelivered)
			raiseServerEvent(CodeDoctorPluginAnswerUndelivered, command, "plugin answer outcome is invalid: "+err.Error())
		}
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

var errBackslashInCommand = command.ErrBackslashInInvocation

// noPipe is the pipe offset scanCommand answers for a line that holds no pipe
// character outside quotes.
const noPipe = -1

// tokenize splits a command string into tokens.
// Handles quoted strings: "hello world" → single token "hello world".
// Unquoted backslash is rejected; double-quoted values decode their escapes.
// Quotes are stripped from the result.
//
// It splits the argument text after a matched plugin or subsystem command,
// which holds no pipe chain: Dispatch read the line through
// command.ReadDispatchLine, which took the chain off it. Accounting and
// authorization read the typed tokens of that scan instead.
func tokenize(input string) ([]string, error) {
	tokens, _, err := scanCommand(input)
	return tokens, err
}

// scanCommand reads the shared operational scan of a command line. It answers the
// tokens tokenize answers, and the byte offset in input of the first pipe
// character outside double quotes, or noPipe when there is none.
//
// The offset comes from this scan and never from the tokens: the quotes are
// stripped from a token, so a quoted pipe (a value that is a literal pipe
// character) and an operator's pipe are the same token after it. A pipe joined
// to a word ("a|b") is found too, because every operator surface cuts at a pipe
// outside double quotes, joined to a word or not (command.ParsePipe).
func scanCommand(input string) (tokens []string, pipeAt int, err error) {
	tokens, pipeAt, _, err = command.ScanInvocation(input)
	if err != nil {
		return nil, noPipe, err
	}
	return tokens, pipeAt, err
}
