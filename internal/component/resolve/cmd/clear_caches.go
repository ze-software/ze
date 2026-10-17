// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: register_clear_caches.go -- the RPC registration of these handlers
// Related: dns.go -- clear dns cache, the DNS resolver's own cache
//
// clear peeringdb cache, clear irr cache and clear cymru cache. Each owner of
// resolver clients answers for its own: the hub clears the resolvers it built
// at startup, and every plugin that declared the cache at Stage 1
// (rpc.DeclareRegistrationInput.ResolverCaches) is asked through the
// resolver-cache-clear callback, which reaches an external-process plugin the
// same way as an internal one. The answer sums the owners and lists each one.
// Each command names what it clears: all, or one key. The bare form has no
// ze:command, so the dispatcher refuses it and a prefix typed short clears
// nothing.

package cmd

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The payload keys and the action the clear-all forms report. A keyed answer
// names its key by the YANG leaf the operator typed it into. The leaves are
// as-number and ip-address rather than asn and address: a leaf named like its
// container reads the container word as its own keyword, and the handler would
// then see two tokens where the operator typed one value.
const (
	actionClearAll = rpc.ResolverCacheClearAll
	keyASNumber    = "as-number"
	keyIPAddress   = "ip-address"
	keyFound       = "found"
	keyRemoved     = "removed"
	keyOwners      = "owners"
	keyOwner       = "owner"
)

// ownerHub names the hub's own resolvers in the per-owner breakdown. Every
// other owner is a plugin, named as the plugin is.
const ownerHub = "hub"

// ownerClearTimeout bounds the wait for one plugin's answer. It is a variable
// only so a test can shorten it; no other code writes it.
var ownerClearTimeout = 5 * time.Second

var errNoPluginServer = errors.New("the plugin server is not running, so no owner of a resolver cache can be asked")

// cacheOwners finds and asks the plugins that own a resolver cache.
// *pluginserver.Server implements it.
type cacheOwners interface {
	ResolverCacheOwners(cache string) []string
	CallResolverCacheClear(ctx context.Context, pluginName string, input *rpc.ResolverCacheClearInput) (*rpc.ResolverCacheClearOutput, error)
}

// clearRequest is one clear an operator asked for. hub is the hub's own share
// of it, and is nil when the hub holds no client of the cache.
type clearRequest struct {
	input rpc.ResolverCacheClearInput
	hub   func() int
}

// ownerCount is how many entries one owner removed.
type ownerCount struct {
	owner   string
	removed int
}

// ownerFailure is an owner that did not answer, and why.
type ownerFailure struct {
	owner string
	err   error
}

// clearEveryOwner runs request on the hub's resolvers and on every plugin that
// owns the cache. It returns the count of each owner that answered and the
// owners that did not: a plugin that failed is never read as one that held
// nothing.
func clearEveryOwner(ctx context.Context, owners cacheOwners, request clearRequest) ([]ownerCount, []ownerFailure) {
	var counts []ownerCount
	if request.hub != nil {
		counts = append(counts, ownerCount{owner: ownerHub, removed: request.hub()})
	}
	var failures []ownerFailure
	for _, name := range owners.ResolverCacheOwners(request.input.Cache) {
		callCtx, cancel := context.WithTimeout(ctx, ownerClearTimeout)
		out, err := owners.CallResolverCacheClear(callCtx, name, &request.input)
		cancel()
		if err != nil {
			failures = append(failures, ownerFailure{owner: name, err: err})
			continue
		}
		counts = append(counts, ownerCount{owner: name, removed: out.Removed})
	}
	return counts, failures
}

// answerClear clears request at every owner and answers with the total and the
// per-owner breakdown. keyLeaf and keyValue name the key of a delete-entry, and
// are empty and nil for a clear-all. command is the line the operator ran.
func answerClear(ctx *pluginserver.CommandContext, command string, request clearRequest, keyLeaf string, keyValue any) *plugin.Response {
	owners, err := ownersOf(ctx)
	if err != nil {
		return clearRefusal(command+": "+err.Error()+".", "start the daemon, then run "+command+" again")
	}
	return clearAnswer(ctx.Context(), owners, command, request, keyLeaf, keyValue)
}

// clearAnswer is answerClear once the owners are known.
func clearAnswer(ctx context.Context, owners cacheOwners, command string, request clearRequest, keyLeaf string, keyValue any) *plugin.Response {
	counts, failures := clearEveryOwner(ctx, owners, request)

	removed := 0
	rows := make([]plugin.Map, 0, len(counts))
	for _, count := range counts {
		removed += count.removed
		rows = append(rows, plugin.Map{keyOwner: count.owner, keyRemoved: count.removed})
	}
	if len(failures) > 0 {
		return clearFailed(command, removed, rows, failures)
	}

	result := plugin.Map{keyAction: request.input.Action, keyRemoved: removed, keyOwners: rows}
	if keyValue != nil {
		result[keyLeaf] = keyValue
		result[keyFound] = removed > 0
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: result}
}

// ownersOf returns the plugin server behind ctx, which knows the plugins that
// own a resolver cache.
func ownersOf(ctx *pluginserver.CommandContext) (cacheOwners, error) {
	if ctx == nil {
		return nil, errNoPluginServer
	}
	if ctx.Server == nil {
		return nil, errNoPluginServer
	}
	return ctx.Server, nil
}

// clearFailed is the answer when an owner did not answer: the owners that did
// were cleared, and the operator learns which plugin was not and how to retry.
// The error carries the same removed total and per-owner rows a success does,
// so a caller reading the data sees what the owners that answered removed.
func clearFailed(command string, removed int, rows []plugin.Map, failures []ownerFailure) *plugin.Response {
	var tb textbuf.Buffer
	tb.Str(command).Str(": cleared ").Int(int64(removed)).Str(" entries, but ")
	for i, failure := range failures {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str("plugin ").Str(failure.owner).Str(" did not answer (").Err(failure.err).Str(")")
	}
	tb.Str(", so its cache may still hold entries.")
	resp := clearRefusal(tb.String(), "check the plugin is running with show plugin list, then run "+command+" again")
	resp.Data = plugin.Map{keyRemoved: removed, keyOwners: rows}
	return resp
}

// clearRefusal is the error answer of a clear command given the wrong words.
// problem says what is wrong, and usage is the command line that works.
func clearRefusal(problem, usage string) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: problem + "\n  " + usage}
}

// clearAllRequest is the clear-all of cache, with the hub's share hub.
func clearAllRequest(cache string, hub func() int) clearRequest {
	return clearRequest{input: rpc.ResolverCacheClearInput{Cache: cache, Action: rpc.ResolverCacheClearAll}, hub: hub}
}

// deleteEntryRequest is the delete-entry of key in cache, with the hub's share hub.
func deleteEntryRequest(cache, key string, hub func() int) clearRequest {
	return clearRequest{input: rpc.ResolverCacheClearInput{Cache: cache, Action: rpc.ResolverCacheDeleteEntry, Key: key}, hub: hub}
}

func handleClearPeeringDBCacheAll(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if len(validated.Tokens()) != 0 {
		return clearRefusal("clear peeringdb cache all takes no words after all.",
			"clear peeringdb cache all"), nil
	}
	var hub func() int
	if resolvers != nil && resolvers.PeeringDB != nil {
		hub = resolvers.PeeringDB.ClearCache
	}
	return answerClear(ctx, "clear peeringdb cache all", clearAllRequest(rpc.ResolverCachePeeringDB, hub), "", nil), nil
}

func handleClearPeeringDBCacheASN(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	if len(args) != 1 {
		return clearRefusal("clear peeringdb cache asn takes one AS number.",
			"clear peeringdb cache asn 65001"), nil
	}
	// asn.Parse reads the asdot spelling beside the decimal one, the forms
	// resolve peeringdb accepts.
	number, err := asn.Parse(args[0])
	if err != nil {
		return clearRefusal("clear peeringdb cache asn: "+args[0]+" is not an AS number. Give it in decimal or asdot notation.", //nolint:nilerr // the refusal is the answer, not a dispatch failure
			"clear peeringdb cache asn 65001"), nil
	}
	var hub func() int
	if resolvers != nil && resolvers.PeeringDB != nil {
		pdb := resolvers.PeeringDB
		hub = func() int { return pdb.ClearCachedASN(number) }
	}
	request := deleteEntryRequest(rpc.ResolverCachePeeringDB, textbuf.StringUint32(number), hub)
	return answerClear(ctx, "clear peeringdb cache asn "+args[0], request, keyASNumber, number), nil
}

func handleClearIRRCacheAll(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if len(validated.Tokens()) != 0 {
		return clearRefusal("clear irr cache all takes no words after all.",
			"clear irr cache all"), nil
	}
	var hub func() int
	if resolvers != nil && resolvers.IRR != nil {
		hub = resolvers.IRR.ClearCache
	}
	return answerClear(ctx, "clear irr cache all", clearAllRequest(rpc.ResolverCacheIRR, hub), "", nil), nil
}

func handleClearIRRCacheASSet(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	if len(args) != 1 {
		return clearRefusal("clear irr cache as-set takes one AS-SET name.",
			"clear irr cache as-set AS-EXAMPLE"), nil
	}
	name := args[0]
	if err := irr.ValidateASSetName(name); err != nil {
		return clearRefusal("clear irr cache as-set: the name is not an AS-SET name ("+err.Error()+"). Give the name without control characters.", //nolint:nilerr // the refusal is the answer, not a dispatch failure
			"clear irr cache as-set AS-EXAMPLE"), nil
	}
	var hub func() int
	if resolvers != nil && resolvers.IRR != nil {
		client := resolvers.IRR
		hub = func() int { return client.ClearCachedASSet(name) }
	}
	request := deleteEntryRequest(rpc.ResolverCacheIRR, name, hub)
	return answerClear(ctx, "clear irr cache as-set "+name, request, keyName, name), nil
}

func handleClearCymruCacheAll(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if len(validated.Tokens()) != 0 {
		return clearRefusal("clear cymru cache all takes no words after all.",
			"clear cymru cache all"), nil
	}
	var hub func() int
	if resolvers != nil && resolvers.Cymru != nil {
		hub = resolvers.Cymru.ClearCache
	}
	return answerClear(ctx, "clear cymru cache all", clearAllRequest(rpc.ResolverCacheCymru, hub), "", nil), nil
}

func handleClearCymruCacheAddress(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	if len(args) != 1 {
		return clearRefusal("clear cymru cache address takes one IP address.",
			"clear cymru cache address 192.0.2.1"), nil
	}
	addr, err := netip.ParseAddr(args[0])
	if err != nil {
		return clearRefusal("clear cymru cache address: "+args[0]+" is not an IPv4 or IPv6 address.", //nolint:nilerr // the refusal is the answer, not a dispatch failure
			"clear cymru cache address 192.0.2.1"), nil
	}
	var hub func() int
	if resolvers != nil && resolvers.Cymru != nil {
		resolver := resolvers.Cymru
		hub = func() int { return resolver.ClearCachedAddress(addr) }
	}
	request := deleteEntryRequest(rpc.ResolverCacheCymru, addr.String(), hub)
	return answerClear(ctx, "clear cymru cache address "+args[0], request, keyIPAddress, addr.String()), nil
}
