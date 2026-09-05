// Design: docs/architecture/core-design.md — policy filter chain
// Related: reactor_notify.go — ingress filter invocation point
// Related: reactor_api_forward.go — egress filter invocation point

package reactor

import (
	"context"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Policy filter attribute names. These are the keywords of the filter text
// format: a filter delta is a sequence of "<name> <value>" pairs, and the same
// keyword names the attribute in the wire-code table (attrNameToCode,
// filter_format.go) and in the modify directives (filter_delta.go). Every
// spelling of one of these tokens in this package is one of these constants,
// so a rename cannot leave a table half-converted.
const (
	policyAttrOrigin                  = "origin"
	policyAttrASPath                  = "as-path"
	policyAttrNextHop                 = "next-hop"
	policyAttrMED                     = "med"
	policyAttrLocalPreference         = "local-preference"
	policyAttrAtomicAggregate         = "atomic-aggregate"
	policyAttrAggregator              = "aggregator"
	policyAttrCommunity               = "community"
	policyAttrOriginatorID            = "originator-id"
	policyAttrClusterList             = "cluster-list"
	policyAttrExtendedCommunity       = "extended-community"
	policyAttrAIGP                    = "aigp"
	policyAttrLargeCommunity          = "large-community"
	policyAttrCommunityAdd            = "community-add"
	policyAttrCommunityRemove         = "community-remove"
	policyAttrLargeCommunityAdd       = "large-community-add"
	policyAttrLargeCommunityRemove    = "large-community-remove"
	policyAttrExtendedCommunityAdd    = "extended-community-add"
	policyAttrExtendedCommunityRemove = "extended-community-remove"
	policyAttrMEDRemove               = "med-remove"
	policyAttrASPathPrepend           = "as-path-prepend"
	policyAttrRemovePrivate           = "remove-private"
	policyAttrNLRI                    = "nlri"
)

// Policy filter chain direction tokens (the `direction` argument passed to
// PolicyFilterChain and on to each filter via the RPC FilterUpdateInput).
const (
	directionImport = "import"
	directionExport = "export"
)

type filterAttrID uint8

const (
	faOrigin filterAttrID = iota
	faASPath
	faNextHop
	faMED
	faLocalPreference
	faAtomicAggregate
	faAggregator
	faCommunity
	faOriginatorID
	faClusterList
	faExtendedCommunity
	faAIGP
	faLargeCommunity
	faCommunityAdd
	faCommunityRemove
	faLargeCommunityAdd
	faLargeCommunityRemove
	faExtendedCommunityAdd
	faExtendedCommunityRemove
	faMEDRemove
	faASPathPrepend
	faRemovePrivate
	faNLRI
	faCount
)

var filterAttrNames = [faCount]string{
	faOrigin: policyAttrOrigin, faASPath: policyAttrASPath, faNextHop: policyAttrNextHop,
	faMED: policyAttrMED, faLocalPreference: policyAttrLocalPreference,
	faAtomicAggregate: policyAttrAtomicAggregate, faAggregator: policyAttrAggregator,
	faCommunity: policyAttrCommunity, faOriginatorID: policyAttrOriginatorID,
	faClusterList: policyAttrClusterList, faExtendedCommunity: policyAttrExtendedCommunity,
	faAIGP: policyAttrAIGP, faLargeCommunity: policyAttrLargeCommunity,
	faCommunityAdd: policyAttrCommunityAdd, faCommunityRemove: policyAttrCommunityRemove,
	faLargeCommunityAdd: policyAttrLargeCommunityAdd, faLargeCommunityRemove: policyAttrLargeCommunityRemove,
	faExtendedCommunityAdd: policyAttrExtendedCommunityAdd, faExtendedCommunityRemove: policyAttrExtendedCommunityRemove,
	faMEDRemove:     policyAttrMEDRemove,
	faASPathPrepend: policyAttrASPathPrepend, faRemovePrivate: policyAttrRemovePrivate,
	faNLRI: policyAttrNLRI,
}

var filterAttrNameToID map[string]filterAttrID

func init() {
	filterAttrNameToID = make(map[string]filterAttrID, faCount)
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		filterAttrNameToID[filterAttrNames[id]] = id
	}
}

type filterAttrs struct {
	values      [faCount]string
	present     uint32
	unknownName string // first unrecognized token at key position (for validateModifyDelta)
}

func (a *filterAttrs) set(id filterAttrID, val string) {
	a.values[id] = val
	a.present |= 1 << id
}

func (a *filterAttrs) get(id filterAttrID) (string, bool) {
	if a.present&(1<<id) == 0 {
		return "", false
	}
	return a.values[id], true
}

func (a *filterAttrs) has(id filterAttrID) bool {
	return a.present&(1<<id) != 0
}

func (a *filterAttrs) clear(id filterAttrID) {
	a.present &^= 1 << id
	a.values[id] = ""
}

func (a *filterAttrs) merge(delta *filterAttrs) {
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		if delta.has(id) {
			a.set(id, delta.values[id])
		}
	}
	// med-remove and med are opposite instructions about attribute 4, and the
	// ORDER of the chain is what decides between them. The merged text holds one
	// slot per attribute and cannot carry that order, so a filter that SETS the
	// metric cancels a removal an earlier filter in the same chain asked for.
	//
	// The reverse needs no clearing and must not have any: a med-remove AFTER a
	// set already wins, because ExtractMEDRemoveOps records its Suppress after
	// textDeltaToModOps has recorded the Set and filterapi.LastSetOrSuppress is
	// last-wins. That is why medRemoveHasWork (filter_delta.go) counts a metric
	// this map holds as work to do: on a route that arrived without one, the
	// Set is the only reason there is anything to suppress. Clearing the med
	// slot here would silently lose that Set instead, because
	// textDeltaToModOps records nothing for an attribute absent from BOTH maps
	// and the original text never carries med (appendSingleAttr,
	// filter_format.go).
	if delta.has(faMED) {
		a.clear(faMEDRemove)
	}
}

// PolicyAction is the result of a policy filter evaluation.
type PolicyAction int

const (
	// PolicyAccept passes the update through unchanged.
	PolicyAccept PolicyAction = iota
	// PolicyReject drops the update (short-circuits the chain).
	PolicyReject
	// PolicyModify passes the update with delta-only attribute changes.
	PolicyModify
)

// PolicyResponse holds the outcome of a single filter invocation.
type PolicyResponse struct {
	Action PolicyAction
	// Delta contains only changed attribute text (action=modify).
	// Empty for accept/reject.
	Delta string
	// Raw, when non-empty on a modify, is a raw full UPDATE-body replacement
	// produced by a raw=true filter (e.g. MP_REACH/MP_UNREACH surgery the text
	// delta cannot express). A raw rewrite is terminal for the chain: it cannot
	// be composed with downstream text deltas.
	Raw []byte
	// Teardown requests the session be terminated (NOTIFICATION + close) after
	// the import chain. Honored only for import (received) UPDATEs. The route is
	// dropped. NotifyCode/NotifySubcode default to Cease / Connection Rejected.
	Teardown      bool
	NotifyCode    uint8
	NotifySubcode uint8
	// Failed marks a PolicyReject that the filter did NOT decide: an IPC error
	// under a fail-closed on-error policy, or a response we could not parse.
	// The route is still rejected -- that is the fail-closed contract -- but a
	// caller counting outcomes must not record it as a policy decision. Without
	// this, a plugin timing out under load is indistinguishable from a plugin
	// deliberately rejecting every route.
	Failed bool
}

// PolicyChainResult is the aggregate outcome of running a filter chain.
type PolicyChainResult struct {
	Action PolicyAction
	// Failed propagates PolicyResponse.Failed for the reject that
	// short-circuited the chain. See that field.
	Failed bool
	// Text is the accumulated modified update text (valid unless Action==PolicyReject).
	Text string
	// Raw is a raw full-payload replacement from a raw filter; terminal.
	// Empty when no raw filter rewrote the payload.
	Raw []byte
	// Teardown (import only) requests NOTIFICATION + session close; route dropped.
	Teardown      bool
	NotifyCode    uint8
	NotifySubcode uint8
}

// PolicyFilterFunc is the signature for calling a named filter.
// The caller provides direction, peer info, and the text-format update.
// Returns the filter's decision.
type PolicyFilterFunc func(pluginName, filterName, direction, peer string, peerAS uint32, updateText string) PolicyResponse

// PolicyFilterChain runs a chain of named filters on an update.
// Filters are piped: each sees the previous filter's output.
// Reject short-circuits. Default is accept at end of chain.
//
// filterRefs is the ordered list of "<plugin>:<filter>" strings.
// callFilter is the function to invoke each filter.
// updateText is the initial text representation of the update.
//
// Returns the aggregate chain result: final action, accumulated update text, an
// optional raw full-payload override, and an optional teardown request.
//
// A raw=true filter that returns a full-payload rewrite (PolicyResponse.Raw) is
// terminal: the rewrite replaces the wire payload and the chain stops, because a
// raw rewrite cannot be composed with downstream text deltas (the text was
// derived from the original payload). A teardown request also short-circuits and
// drops the route.
func PolicyFilterChain(filterRefs []filterapi.FilterRef, direction, peer string, peerAS uint32, updateText string, callFilter PolicyFilterFunc) PolicyChainResult {
	if len(filterRefs) == 0 {
		return PolicyChainResult{Action: PolicyAccept, Text: updateText}
	}

	current := updateText
	for _, ref := range filterRefs {
		if ref.Inactive {
			continue
		}
		pluginName, filterName, _ := strings.Cut(ref.Name, ":")
		result := callFilter(pluginName, filterName, direction, peer, peerAS, current)

		// Teardown short-circuits: the session is going away, so drop the route.
		if result.Teardown {
			return PolicyChainResult{
				Action:        PolicyReject,
				Teardown:      true,
				Failed:        result.Failed,
				NotifyCode:    result.NotifyCode,
				NotifySubcode: result.NotifySubcode,
			}
		}

		switch result.Action {
		case PolicyReject:
			return PolicyChainResult{Action: PolicyReject, Failed: result.Failed}
		case PolicyModify:
			// A raw full-payload rewrite is terminal (see doc comment).
			if len(result.Raw) > 0 {
				return PolicyChainResult{Action: PolicyModify, Text: current, Raw: result.Raw}
			}
			current = applyFilterDelta(current, result.Delta)
		case PolicyAccept:
			// continue with current text
		}
	}

	return PolicyChainResult{Action: PolicyAccept, Text: current}
}

// applyFilterDelta merges delta-only attribute changes into the current update text.
// The delta contains only changed fields. Fields not in delta remain unchanged.
//
// Both current and delta use the same text format:
//
//	"<attr-name> <value> [<attr-name> <value> ...] [nlri <family> <op> <prefix>...]"
//
// Delta fields overwrite corresponding fields in current.
func applyFilterDelta(current, delta string) string {
	if delta == "" {
		return current
	}

	currentAttrs := parseFilterAttrs(current)
	deltaAttrs := parseFilterAttrs(delta)

	currentAttrs.merge(deltaAttrs)

	return formatFilterAttrs(currentAttrs)
}

var policySingleToken = map[string]bool{
	policyAttrOrigin: true, policyAttrNextHop: true, policyAttrMED: true,
	policyAttrLocalPreference: true, policyAttrAtomicAggregate: true,
	policyAttrAggregator: true, policyAttrOriginatorID: true,
	policyAttrASPathPrepend: true, policyAttrRemovePrivate: true, policyAttrAIGP: true,
}

// parseFilterAttrsCalls counts parseFilterAttrs invocations. Test seam for
// TestFilterDeltaParseCallCount, which proves the filter modify paths parse
// each filter text exactly once (spec filter-delta-parse-once AC-2/AC-3).
var parseFilterAttrsCalls atomic.Uint64

// parseFilterAttrs parses text-format attributes into a fresh struct. The hot
// modify block calls parseFilterAttrsInto with storage of its own; this is for
// the cold callers, which are the filter chain's own text overlay and the tests.
func parseFilterAttrs(text string) *filterAttrs {
	attrs := &filterAttrs{}
	parseFilterAttrsInto(attrs, text)
	return attrs
}

// parseFilterAttrsInto parses text-format attributes into caller-provided
// storage. Each attribute is "name value" where value may contain spaces.
// Special key "nlri" captures the NLRI section.
//
// EVERY VALUE IS A WINDOW INTO text, so the caller MUST keep text alive for as
// long as it reads attrs. That was already true of every single-token value,
// which strings.Fields returned as a substring; a multi-token value now joins
// nothing in the common case, because the tokens already sit next to each other
// separated by one space. A run whose separators are anything else is rewritten
// by joinFilterTokens, which is the only path here that allocates.
func parseFilterAttrsInto(attrs *filterAttrs, text string) {
	parseFilterAttrsCalls.Add(1)
	*attrs = filterAttrs{}

	scan := filterTokens{text: text}
	if !scan.next() {
		return
	}

	for {
		nameStart, nameEnd := scan.start, scan.end
		name := text[nameStart:nameEnd]
		more := scan.next()

		id, known := filterAttrNameToID[name]
		if !known {
			if attrs.unknownName == "" {
				attrs.unknownName = name
			}
			if !more {
				return
			}
			continue
		}

		switch name {
		case policyAttrNLRI:
			// The NLRI value carries the "nlri" keyword itself, so its run
			// starts at the name rather than after it.
			var value string
			value, more = parseFilterAttrRun(&scan, nameStart, nameEnd, more)
			attrs.set(id, value)

		case policyAttrAtomicAggregate, policyAttrMEDRemove:
			// Valueless tokens. ATOMIC_AGGREGATE is an attribute whose wire
			// value is zero-length; med-remove is a directive that names an
			// action and needs no operand. Both are recorded as present with an
			// empty value, so the loop does not consume the token that follows.
			attrs.set(id, "")

		default:
			if policySingleToken[name] {
				if more {
					attrs.set(id, text[scan.start:scan.end])
					more = scan.next()
				}
				break
			}
			if !more {
				attrs.set(id, "")
				break
			}
			if isPolicyAttrName(text[scan.start:scan.end]) {
				attrs.set(id, "")
				break
			}
			start, end := scan.start, scan.end
			more = scan.next()
			var value string
			value, more = parseFilterAttrRun(&scan, start, end, more)
			attrs.set(id, value)
		}

		if !more {
			return
		}
	}
}

// parseFilterAttrRun consumes the tokens of one multi-token value, starting
// from a run that already covers text[start:end], and returns the value plus
// whether a token is still loaded in scan.
//
// The run ends at the next known attribute name, which is the rule the text
// format has always used: a value cannot contain a keyword.
func parseFilterAttrRun(scan *filterTokens, start, end int, more bool) (string, bool) {
	text := scan.text

	// A run is CLEAN when one space separates every pair of tokens, which is
	// what every machine-written filter text produces. A clean run needs no
	// rewrite, so the value is the window the tokens already sit in.
	clean := true
	for more && !isPolicyAttrName(text[scan.start:scan.end]) {
		if !oneSpaceApart(text, end, scan.start) {
			clean = false
		}
		end = scan.end
		more = scan.next()
	}

	value := text[start:end]
	if !clean {
		value = joinFilterTokens(value)
	}
	return value, more
}

// oneSpaceApart reports whether exactly one space separates a token ending at
// end from the next token starting at start. It is what decides whether a
// multi-token value can be the window its tokens already sit in.
//
// Reading text[end] is in range because the caller only asks about a token it
// has loaded, so start is inside the text and end is below it.
func oneSpaceApart(text string, end, start int) bool {
	if start != end+1 {
		return false
	}
	return text[end] == ' '
}

// joinFilterTokens rewrites a run of tokens with exactly one space between
// them. It is the fallback for a plugin delta whose separators are tabs, or
// runs of spaces, and it is the only allocation left in the parse.
func joinFilterTokens(run string) string {
	var b textbuf.Buffer
	scan := filterTokens{text: run}
	for scan.next() {
		if b.Len() > 0 {
			b.Byte(' ')
		}
		b.Str(run[scan.start:scan.end])
	}
	return b.String()
}

// filterTokens walks the whitespace-delimited tokens of a filter text and
// allocates nothing. It answers the split strings.Fields answers, and it also
// answers WHERE each token sits, which is what lets a multi-token value be a
// window into the text rather than a fresh join.
//
// One token is loaded at a time: next reports whether it found one, and start
// and end delimit it.
type filterTokens struct {
	text  string
	off   int // where the next scan starts
	start int // the loaded token's first byte
	end   int // one past the loaded token's last byte
}

// next loads the token after the current one. It reports false when nothing but
// whitespace is left, and leaves start and end on the last token it loaded.
func (f *filterTokens) next() bool {
	i := f.off
	for i < len(f.text) {
		size, space := filterSpaceAt(f.text, i)
		if !space {
			break
		}
		i += size
	}
	if i >= len(f.text) {
		f.off = i
		return false
	}

	f.start = i
	for i < len(f.text) {
		size, space := filterSpaceAt(f.text, i)
		if space {
			break
		}
		i += size
	}
	f.end = i
	f.off = i
	return true
}

// filterSpaceASCII is the whitespace table for the bytes a filter text is made
// of. It holds the six ASCII runes unicode.IsSpace holds, which is the set
// strings.Fields split on, and a table lookup is what keeps the scan as cheap
// as the strings.Fields it replaced.
var filterSpaceASCII = [utf8.RuneSelf]bool{
	'\t': true, '\n': true, '\v': true, '\f': true, '\r': true, ' ': true,
}

// filterSpaceAt reports whether the rune at off is whitespace, and how many
// bytes it occupies. It answers what strings.Fields answers, which splits on
// unicode.IsSpace, so a filter text carrying an exotic space splits the same
// way it split before the parse stopped calling strings.Fields.
func filterSpaceAt(s string, off int) (int, bool) {
	if c := s[off]; c < utf8.RuneSelf {
		return 1, filterSpaceASCII[c]
	}
	r, size := utf8.DecodeRuneInString(s[off:])
	return size, unicode.IsSpace(r)
}

// isPolicyAttrName returns true if the token is a known BGP attribute name.
func isPolicyAttrName(s string) bool {
	switch s {
	case policyAttrOrigin, policyAttrASPath, policyAttrNextHop, policyAttrMED, policyAttrLocalPreference,
		policyAttrAtomicAggregate, policyAttrAggregator, policyAttrCommunity, policyAttrOriginatorID,
		policyAttrClusterList, policyAttrExtendedCommunity, policyAttrAIGP, policyAttrLargeCommunity, policyAttrNLRI,
		policyAttrASPathPrepend, policyAttrRemovePrivate, policyAttrMEDRemove,
		policyAttrCommunityAdd, policyAttrCommunityRemove,
		policyAttrLargeCommunityAdd, policyAttrLargeCommunityRemove,
		policyAttrExtendedCommunityAdd, policyAttrExtendedCommunityRemove:
		return true
	}
	return false
}

// formatFilterAttrsOrder defines the output order for formatFilterAttrs.
// Matches the historical map-based order for deterministic results.
var formatFilterAttrsOrder = [...]filterAttrID{
	faOrigin, faASPath, faNextHop, faMED, faLocalPreference,
	faAtomicAggregate, faAggregator, faCommunity, faOriginatorID,
	faClusterList, faExtendedCommunity, faAIGP, faLargeCommunity,
	faCommunityAdd, faCommunityRemove,
	faLargeCommunityAdd, faLargeCommunityRemove,
	faExtendedCommunityAdd, faExtendedCommunityRemove,
	faMEDRemove,
	faASPathPrepend, faRemovePrivate, faNLRI,
}

// formatFilterAttrs converts a filterAttrs struct back to text format.
// Attributes are output in a fixed order for deterministic results.
func formatFilterAttrs(attrs *filterAttrs) string {
	var buf []byte
	for _, id := range formatFilterAttrsOrder {
		val, ok := attrs.get(id)
		if !ok {
			continue
		}
		if len(buf) > 0 {
			buf = append(buf, ' ')
		}
		name := filterAttrNames[id]
		switch id {
		case faNLRI:
			buf = append(buf, val...)
		case faAtomicAggregate, faMEDRemove:
			buf = append(buf, name...)
		default:
			buf = append(buf, name...)
			buf = append(buf, ' ')
			buf = append(buf, val...)
		}
	}
	return string(buf)
}

// policyFilterTimeout is the per-filter IPC timeout (spec: 5 seconds).
const policyFilterTimeout = 5 * time.Second

// filterTransport is the plugin-server surface a policy filter's IPC body talks
// to. *pluginserver.Server satisfies it, and is what production passes.
//
// It exists because that type is CONCRETE and its answer arrives off a live
// plugin socket, so two fail-closed branches of the body could not be driven at
// all: an IPC error under a fail-closed on-error policy, and ze overriding a
// filter's PolicyModify that touched an attribute the filter never declared.
// Both set PolicyResponse.Failed, which is the flag that stops a plugin timeout
// being counted as a policy decision, and a flag nothing can test is a flag a
// refactor can drop in silence.
//
// It is NOT policyFilterSeam in another shape, and the two sit at different
// layers of one function. That seam replaces the whole IPC body, so a test using
// it drives the CHAIN over a canned answer and never enters the body. This one
// replaces what the body TALKS TO, so a test using it drives the body itself.
type filterTransport interface {
	FilterInfo(pluginName, filterName string) (declaredAttrs []string, raw bool)
	FilterOnError(pluginName, filterName string) rpc.OnErrorPolicy
	CallFilterUpdate(ctx context.Context, pluginName string, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error)
}

// filterAPI returns the transport the policy filter path talks to: the plugin
// server in production, a test's stand-in when one is set.
//
// The nil check is load-bearing rather than defensive. A nil *pluginserver.Server
// stored in an interface is NOT a nil interface, and every caller of this method
// tests the result for nil to fail closed. Returning the typed nil would make
// that guard read "the filter engine is present" for a daemon that has none, and
// the route would go out unfiltered.
func (r *Reactor) filterAPI() filterTransport {
	if r.filterTransportSeam != nil {
		return r.filterTransportSeam
	}
	if r.api == nil {
		return nil
	}
	return r.api
}

// policyFilterFunc returns a PolicyFilterFunc that calls external plugins via IPC.
// The reactor's API server is used to look up plugin connections.
// rawPayload is the raw UPDATE body bytes for AC-15 (raw mode) - may be nil.
// Implements AC-13 (reject modify of undeclared attributes) and AC-15 (raw mode).
//
// policyFilterSeam replaces the IPC body when set. It is nil in production and
// set only by a test, which is the same shape as pluginServerMaker (reactor.go)
// and exists for the same reason: r.api is a concrete *pluginserver.Server whose
// filter answer comes off a live plugin socket, so the branches AFTER the chain
// runs -- a filter that returns a text delta, and a modification that then
// cannot be applied -- have no other entry point. Testing those on the helper
// instead of on runIngressPolicyChain / runEgressPolicyChainASN4 proves the
// helper and not that a caller reaches it (ai/rules/evidence.md).
func (r *Reactor) policyFilterFunc(rawPayload []byte) PolicyFilterFunc {
	if r.policyFilterSeam != nil {
		return r.policyFilterSeam
	}
	return func(pluginName, filterName, direction, peer string, peerAS uint32, updateText string) PolicyResponse {
		api := r.filterAPI()
		if api == nil {
			reactorLogger().Warn("policy filter: no API server", "plugin", pluginName, "filter", filterName)
			// Failed: a guard MISS, the filter never ran. Mirrors the same
			// condition in runEgressPolicyChainASN4, which reaches this state
			// first on the egress path but does not shadow it entirely.
			return PolicyResponse{Action: PolicyReject, Failed: true} // fail-closed
		}

		// Look up filter declaration for AC-13 (attribute validation) and AC-15 (raw mode).
		declaredAttrs, wantsRaw := api.FilterInfo(pluginName, filterName)

		// AC-15: If filter declared raw=true, include the raw UPDATE body as bytes
		// (encoding/json base64-encodes it; the plugin always receives a copy via
		// the DirectBridge/socket marshal, so passing rawPayload directly is safe).
		var rawBytes []byte
		if wantsRaw && len(rawPayload) > 0 {
			rawBytes = rawPayload
		}

		ctx, cancel := context.WithTimeout(context.Background(), policyFilterTimeout)
		defer cancel()

		input := &rpc.FilterUpdateInput{
			Filter:    filterName,
			Direction: direction,
			Peer:      peer,
			PeerAS:    peerAS,
			Update:    updateText,
			Raw:       rawBytes,
		}

		out, err := api.CallFilterUpdate(ctx, pluginName, input)
		if err != nil {
			onError := api.FilterOnError(pluginName, filterName)
			reactorLogger().Warn("policy filter IPC error", "plugin", pluginName, "filter", filterName, "on-error", onError, "error", err)
			if onError == rpc.OnErrorAccept {
				return PolicyResponse{Action: PolicyAccept}
			}
			// Failed: the filter never decided. Still rejected (fail-closed),
			// but not a policy decision -- see PolicyResponse.Failed.
			return PolicyResponse{Action: PolicyReject, Failed: true}
		}

		action, ok := toPolicyAction(out.Action)
		if !ok {
			reactorLogger().Warn("policy filter: invalid action", "plugin", pluginName, "filter", filterName, "action", out.Action)
			return PolicyResponse{Action: PolicyReject, Failed: true} // fail-closed on invalid response
		}

		// AC-13: Validate that modify delta only touches declared attributes.
		if action == PolicyModify && len(declaredAttrs) > 0 && out.Update != "" {
			if violation := validateModifyDelta(out.Update, declaredAttrs); violation != "" {
				reactorLogger().Warn("policy filter: modify of undeclared attribute",
					"plugin", pluginName, "filter", filterName, "violation", violation)
				// Failed: the filter decided PolicyModify and ze overrode it. The
				// route is still rejected (fail-closed), but no filter chose to
				// drop it, so an outcome counter must not read this as policy.
				return PolicyResponse{Action: PolicyReject, Failed: true} // reject invalid modify
			}
		}

		resp := PolicyResponse{Action: action, Delta: out.Update}
		// Raw full-payload replacement is only meaningful on a modify.
		if action == PolicyModify {
			resp.Raw = out.Raw
		}
		// Teardown is honored only for import (received) UPDATEs; ignore it on
		// export so a misconfigured/misbehaving filter cannot drop sessions there.
		if out.Teardown && direction == directionImport {
			resp.Teardown = true
			resp.NotifyCode = out.NotifyCode
			resp.NotifySubcode = out.NotifySubcode
		}
		return resp
	}
}

// decodeFilterRawOverride validates a full UPDATE-body replacement from a raw
// policy filter. A valid UPDATE body is at least 4 bytes: withdrawn-routes
// length(2) + path-attribute length(2).
//
// It answers TWO questions, and a single nil could only answer one of them
// (ai/rules/evidence.md). An EMPTY raw means the filter asked for no
// replacement, which is the ordinary case for every text-delta filter. A raw of
// 1..3 bytes means the filter DID ask for a replacement and handed over
// something that cannot be an UPDATE body.
//
// Folding those together made the second silently indistinguishable from the
// first: both call sites read nil as "no override", skipped the branch, and --
// because PolicyFilterChain makes a raw response terminal and returns the text
// UNCHANGED (Text: current), so the text-delta branch below does not run either
// -- forwarded or cached the ORIGINAL body. The filter had asked for the body to
// be replaced, and got the body it was replacing. On export that leaks whatever
// the raw surgery was removing; on import it installs it.
//
// malformed is therefore a guard MISS, not a policy decision: the caller
// suppresses (export) or drops (import) the route and says so.
func decodeFilterRawOverride(raw []byte) (override []byte, malformed bool) {
	if len(raw) == 0 {
		return nil, false
	}
	if len(raw) < 4 {
		return nil, true
	}
	return raw, false
}

// validateModifyDelta checks that a modify delta only contains attributes
// from the declared set. Returns the first violating attribute name, or "".
func validateModifyDelta(delta string, declaredAttrs []string) string {
	allowed := make(map[string]bool, len(declaredAttrs))
	for _, a := range declaredAttrs {
		allowed[a] = true
	}

	// Parse the delta to find which attributes it touches.
	deltaAttrs := parseFilterAttrs(delta)
	if deltaAttrs.unknownName != "" && !allowed[deltaAttrs.unknownName] {
		return deltaAttrs.unknownName
	}
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		if deltaAttrs.has(id) && !allowed[filterAttrNames[id]] {
			return filterAttrNames[id]
		}
	}

	return ""
}

// toPolicyAction maps the plugin's typed FilterAction to the reactor's
// internal PolicyAction. Returns false for unspecified or unknown values;
// the caller fails closed (reject) in that case.
func toPolicyAction(a rpc.FilterAction) (PolicyAction, bool) {
	switch a {
	case rpc.FilterAccept:
		return PolicyAccept, true
	case rpc.FilterReject:
		return PolicyReject, true
	case rpc.FilterModify:
		return PolicyModify, true
	case rpc.FilterUnspecified:
		return PolicyReject, false
	}
	return PolicyReject, false
}
