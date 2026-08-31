// Design: docs/architecture/bgp/on-demand-origination.md -- on-demand route origination CLI verbs
// RFC: rfc/short/rfc2545.md -- the next-hop value must be a global IPv6 address (handleAnnounceUnicast)

package announce

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	kwTag       = "tag"
	kwFor       = "for"
	kwSelector  = "selector"
	kwValue     = "value"
	kwFamily    = "family"
	kwNextHop   = "next-hop"
	kwCommunity = "community"
	kwSelf      = "self"
)

// fieldWithdrawn is the payload key the four withdraw handlers answer with, and
// it holds the number of tracked announcements the handler removed. Naming it
// once is what keeps the four answers reading the same to an operator who asks
// for one of them by name.
const fieldWithdrawn = "withdrawn"

const maxTagLen = 128

var (
	errMissingPrefix = errors.New("missing prefix")
	errTagTooLong    = errors.New("tag key or value exceeds 128 characters")

	errFlowspecRequiresAction    = errors.New("flowspec announce requires an action: rate-limit <bytes-per-sec>, discard, or community <action>")
	errCommunityRequiresAction   = errors.New("community requires an action, such as traffic-rate <asn> <rate> or redirect <asn> <target>")
	errFlowspecActionExtraTokens = errors.New("flowspec action does not use every token given to it")
	errRateLimitRequiresBytes    = errors.New("rate-limit requires a bytes-per-second value")
	errMissingFlowspecComponents = errors.New("flowspec announce requires at least one match component (e.g. destination <prefix>)")
)

var (
	registryMu     sync.Mutex
	globalRegistry *Registry
)

func getOrInitRegistry(ctx *pluginserver.CommandContext) *Registry {
	registryMu.Lock()
	defer registryMu.Unlock()

	if globalRegistry != nil {
		return globalRegistry
	}
	bgpReactor, _, err := requireBGPReactor(ctx)
	if err != nil {
		return nil
	}
	globalRegistry = NewRegistry(func(sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
		return bgpReactor.WithdrawNLRIBatch(sel, batch, sender)
	})
	return globalRegistry
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:announce-unicast", Handler: handleAnnounceUnicastCmd},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:announce-blackhole", Handler: handleAnnounceBlackholeCmd},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:announce-flowspec", Handler: handleAnnounceFlowspecCmd},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:withdraw-tag", Handler: handleWithdrawTag},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:withdraw-id", Handler: handleWithdrawID},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:withdraw-all", Handler: handleWithdrawAll},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:show-announcements", Handler: handleShowAnnouncements},
	)
}

// announceRegistry answers the reactor and the announcement registry the three
// announce commands share, or the response their caller owes when the reactor
// is down.
//
// It exists for the reason withdrawRegistry does: each form is its own command
// with its own wire method, so the reactor check that used to sit once in front
// of a keyword switch is owed three times.
func announceRegistry(ctx *pluginserver.CommandContext) (bgptypes.BGPReactor, *Registry, *plugin.Response, error) {
	bgpReactor, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return nil, nil, errResp, err
	}
	reg := getOrInitRegistry(ctx)
	if reg == nil {
		return nil, nil, nil, errReactorNotAvailable
	}
	return bgpReactor, reg, nil, nil
}

// handleAnnounceUnicastCmd answers `announce unicast <prefix> ...`.
func handleAnnounceUnicastCmd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	bgpReactor, reg, errResp, err := announceRegistry(ctx)
	if err != nil {
		return errResp, err
	}
	return handleAnnounceUnicast(ctx, bgpReactor, reg, args)
}

// handleAnnounceBlackholeCmd answers `announce blackhole <prefix> ...`.
func handleAnnounceBlackholeCmd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	bgpReactor, reg, errResp, err := announceRegistry(ctx)
	if err != nil {
		return errResp, err
	}
	return handleAnnounceBlackhole(ctx, bgpReactor, reg, args)
}

// handleAnnounceFlowspecCmd answers `announce flowspec <components> ...`.
func handleAnnounceFlowspecCmd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	bgpReactor, reg, errResp, err := announceRegistry(ctx)
	if err != nil {
		return errResp, err
	}
	return handleAnnounceFlowspec(ctx, bgpReactor, reg, args)
}

type announceOpts struct {
	tagKey   string
	tagValue string
	duration time.Duration
}

func parseTrailingOpts(args []string) (announceOpts, error) {
	var opts announceOpts
	i := 0
	for i < len(args) {
		switch strings.ToLower(args[i]) {
		case kwTag:
			if i+2 >= len(args) {
				return opts, errors.New("tag requires <key> <value>")
			}
			if len(args[i+1]) > maxTagLen || len(args[i+2]) > maxTagLen {
				return opts, errTagTooLong
			}
			opts.tagKey = args[i+1]
			opts.tagValue = args[i+2]
			i += 3
		case kwFor:
			if i+1 >= len(args) {
				return opts, errors.New("for requires <duration> (e.g. 300s)")
			}
			d, err := parseDuration(args[i+1])
			if err != nil {
				return opts, fmt.Errorf("invalid duration: %w", err)
			}
			if d <= 0 {
				return opts, errors.New("duration must be positive")
			}
			opts.duration = d
			i += 2
		default:
			return opts, nil
		}
	}
	return opts, nil
}

// maxDurationSeconds is the largest whole-second count representable as a
// time.Duration (an int64 nanosecond count).
const maxDurationSeconds = uint64(math.MaxInt64 / int64(time.Second))

func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	secs, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("expected duration (e.g. 300s) or seconds: %s", s)
	}
	// time.Duration is an int64 nanosecond count, so a seconds value beyond
	// maxDurationSeconds wraps: reject it instead of returning a negative delay.
	if secs > maxDurationSeconds {
		return 0, fmt.Errorf("duration %s seconds out of range 0..%d", s, maxDurationSeconds)
	}
	return time.Duration(secs) * time.Second, nil //nolint:gosec // G115: bounded above
}

func prefixToFamily(prefix netip.Prefix) family.Family {
	if prefix.Addr().Is6() {
		return family.IPv6Unicast
	}
	return family.IPv4Unicast
}

func announceAndTrack(reg *Registry, bgpReactor bgptypes.BGPReactor, sel *selector.Selector, batch bgptypes.NLRIBatch, opts announceOpts, source string, sender plugin.Sender) (*plugin.Response, error) {
	if err := bgpReactor.AnnounceNLRIBatch(sel, batch, sender); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	if opts.tagKey == "" {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"announced": 1},
		}, nil
	}

	id, err := reg.Announce(opts.tagKey, opts.tagValue, sel, batch, source, sender, opts.duration)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	var tb textbuf.Buffer
	tagStr := tb.Str(opts.tagKey).Byte('=').Str(opts.tagValue).String()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"announced": 1, "id": id, "tag": tagStr},
	}, nil
}

func handleAnnounceUnicast(ctx *pluginserver.CommandContext, bgpReactor bgptypes.BGPReactor, reg *Registry, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, errMissingPrefix
	}

	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid prefix: %w", err)
	}

	remaining := args[1:]
	builder := attribute.NewBuilder()
	builder.SetOrigin(0) // IGP
	nextHop := bgptypes.NewNextHopSelf()
	// RFC 7999 Section 3.1 binds the community, not the verb that attaches it, so
	// this path meets the same agreement gate when the operator names BLACKHOLE.
	blackholeAsked := false

	i := 0
	for i < len(remaining) {
		kw := strings.ToLower(remaining[i])
		switch kw {
		case kwNextHop:
			if i+1 >= len(remaining) {
				return nil, errors.New("next-hop requires a value")
			}
			nhVal := remaining[i+1]
			if strings.EqualFold(nhVal, kwSelf) {
				nextHop = bgptypes.NewNextHopSelf()
			} else {
				addr, parseErr := netip.ParseAddr(nhVal)
				if parseErr != nil {
					return nil, fmt.Errorf("invalid next-hop address: %w", parseErr)
				}
				// RFC 2545 Section 3: the Network Address of Next Hop field
				// carries the GLOBAL IPv6 address of the next hop. The link-local
				// half is appended by the encoder from the session's link-local
				// leaf when the section's condition holds, so one offered here has
				// no global address to follow.
				if formErr := attribute.ValidateGlobalNextHop(addr); formErr != nil {
					return nil, formErr
				}
				nextHop = bgptypes.NewNextHopExplicit(addr)
			}
			i += 2
		case kwCommunity:
			if i+1 >= len(remaining) {
				return nil, errors.New("community requires a value")
			}
			comm, parseErr := attribute.ParseCommunity(remaining[i+1])
			if parseErr != nil {
				return nil, fmt.Errorf("invalid community: %w", parseErr)
			}
			builder.AddCommunityValue(comm)
			if attribute.Community(comm) == attribute.CommunityBlackhole {
				blackholeAsked = true
			}
			i += 2
		case kwTag, kwFor:
			goto parseOpts
		default:
			return nil, fmt.Errorf("unexpected token: %s", remaining[i])
		}
	}

parseOpts:
	opts, err := parseTrailingOpts(remaining[i:])
	if err != nil {
		return nil, err
	}

	sel := selector.ParseDefault(ctx.PeerSelector())
	if blackholeAsked {
		sel, err = agreedSelector(ctx, sel, "announce unicast")
		if err != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
		}
	}
	fam := prefixToFamily(prefix)
	inet := nlri.NewINET(fam, prefix, 0)

	batch := bgptypes.NLRIBatch{
		Family:  fam,
		NLRIs:   []nlri.NLRI{inet},
		NextHop: nextHop,
		Attrs:   builder,
	}

	return announceAndTrack(reg, bgpReactor, sel, batch, opts, "cli", ctx.Sender)
}

func handleAnnounceBlackhole(ctx *pluginserver.CommandContext, bgpReactor bgptypes.BGPReactor, reg *Registry, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, errMissingPrefix
	}

	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid prefix: %w", err)
	}

	remaining := args[1:]

	i := 0
	for i < len(remaining) {
		kw := strings.ToLower(remaining[i])
		switch kw {
		case kwTag, kwFor:
			goto parseOpts
		default:
			return nil, fmt.Errorf("unexpected token: %s", remaining[i])
		}
	}

parseOpts:
	opts, err := parseTrailingOpts(remaining[i:])
	if err != nil {
		return nil, err
	}

	builder := attribute.NewBuilder()
	builder.SetOrigin(0) // IGP
	builder.AddCommunityValue(uint32(attribute.CommunityBlackhole))

	// RFC 7999 Section 3.1: "In a bilateral peering relationship, use of the
	// BLACKHOLE community MUST be agreed upon by the two networks before
	// advertising it." This verb attaches that community itself, so the fan-out
	// is narrowed to the sessions that recorded the agreement.
	sel, err := agreedSelector(ctx, selector.ParseDefault(ctx.PeerSelector()), "announce blackhole")
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}
	fam := prefixToFamily(prefix)
	inet := nlri.NewINET(fam, prefix, 0)

	batch := bgptypes.NLRIBatch{
		Family:  fam,
		NLRIs:   []nlri.NLRI{inet},
		NextHop: bgptypes.NewNextHopSelf(),
		Attrs:   builder,
	}

	return announceAndTrack(reg, bgpReactor, sel, batch, opts, "cli", ctx.Sender)
}

// handleAnnounceFlowspec originates a tracked FlowSpec rule on demand. Grammar:
//
//	announce flowspec <components...> (rate-limit <bytes-per-sec> | discard) [tag <k> <v>] [for <dur>]
//
// The match components (destination/source/protocol/port/tcp-flags/...) are
// encoded into a FlowSpec NLRI through the family registration seam (the same
// encoder the update-text path uses), so no direct dependency on the flowspec
// NLRI plugin is introduced. The action becomes an RFC 8955 traffic-rate
// extended community (rate 0 == discard).
func handleAnnounceFlowspec(ctx *pluginserver.CommandContext, bgpReactor bgptypes.BGPReactor, reg *Registry, args []string) (*plugin.Response, error) {
	components, actionArgs, optArgs, err := splitFlowspecArgs(args)
	if err != nil {
		return nil, err
	}
	if len(components) == 0 {
		return nil, errMissingFlowspecComponents
	}

	fam, ok := family.LookupFamily(flowspecFamilyName(components))
	if !ok {
		return nil, errMissingFlowspecComponents
	}

	flowNLRI, err := encodeFlowspecNLRI(fam, components)
	if err != nil {
		return nil, err
	}

	builder := attribute.NewBuilder()
	builder.SetOrigin(0) // IGP
	ecs, consumed, err := route.ParseExtendedCommunities(actionArgs)
	if err != nil {
		return nil, fmt.Errorf("invalid flowspec action: %w", err)
	}
	// Every action form reads a fixed number of tokens, so a token left over is
	// a word the operator typed and this rule does not carry. The two sugar
	// spellings are synthesized here and always consume what they produce; the
	// `community` form is the operator's own text, and announcing a rule that
	// silently drops part of it advertises something they did not describe.
	if consumed < len(actionArgs) {
		return nil, fmt.Errorf("%w: %s", errFlowspecActionExtraTokens, strings.Join(actionArgs[consumed:], " "))
	}
	for _, ec := range ecs {
		builder.AddExtendedCommunity(ec)
	}

	opts, err := parseTrailingOpts(optArgs)
	if err != nil {
		return nil, err
	}

	sel := selector.ParseDefault(ctx.PeerSelector())
	batch := bgptypes.NLRIBatch{
		Family:  fam,
		NLRIs:   []nlri.NLRI{flowNLRI},
		NextHop: bgptypes.NewNextHopSelf(),
		Attrs:   builder,
	}
	return announceAndTrack(reg, bgpReactor, sel, batch, opts, "cli", ctx.Sender)
}

// splitFlowspecArgs separates the match components from the traffic action and
// the trailing tag/for options. An action is mandatory (no fabricated default):
// `rate-limit <bps>` maps to a traffic-rate function, `discard` to a traffic-rate
// of 0. Everything before the action keyword is a component token.
func splitFlowspecArgs(args []string) (components, action, opts []string, err error) {
	for i := range args {
		switch strings.ToLower(args[i]) {
		case "discard":
			return args[:i], []string{"discard"}, args[i+1:], nil
		case "rate-limit":
			if i+1 >= len(args) {
				return nil, nil, nil, errRateLimitRequiresBytes
			}
			return args[:i], []string{"traffic-rate", "0", args[i+1], "bytes"}, args[i+2:], nil
		case kwCommunity:
			// RFC 8955 Section 7 makes the action an extended community, and
			// route.ParseExtendedCommunities reads every form it defines. The
			// two spellings above are sugar over one, so this is the general
			// case rather than a third alternative beside them.
			tail := args[i+1:]
			end := trailingOptsAt(tail)
			if end == 0 {
				return nil, nil, nil, errCommunityRequiresAction
			}
			return args[:i], tail[:end], tail[end:], nil
		case kwTag, kwFor:
			return nil, nil, nil, errFlowspecRequiresAction
		}
	}
	return nil, nil, nil, errFlowspecRequiresAction
}

// trailingOptsAt answers where the tag/for options begin, which is where the
// action's own tokens end. An action is variable length: `discard` is one word
// and `traffic-rate <asn> <rate> bytes` is four, so only the option keywords
// say where it stops.
func trailingOptsAt(args []string) int {
	for i := range args {
		switch strings.ToLower(args[i]) {
		case kwTag, kwFor:
			return i
		}
	}
	return len(args)
}

// flowspecFamilyName picks "ipv4/flow" vs "ipv6/flow" from the destination or
// source prefix (a v6 prefix always contains ':'), defaulting to ipv4/flow.
func flowspecFamilyName(components []string) string {
	for i := 0; i+1 < len(components); i++ {
		switch strings.ToLower(components[i]) {
		case "destination", "source":
			if strings.Contains(components[i+1], ":") {
				return "ipv6/flow"
			}
		}
	}
	return "ipv4/flow"
}

// encodeFlowspecNLRI builds a FlowSpec NLRI from component tokens via the family
// registration seam, mirroring the update-text path's encodeViaRegistry.
func encodeFlowspecNLRI(fam family.Family, components []string) (nlri.NLRI, error) {
	hexStr, err := registry.EncodeNLRIByFamily(fam.String(), components)
	if err != nil {
		return nil, fmt.Errorf("%s encode: %w", fam, err)
	}
	wireBytes, err := hex.DecodeString(strings.ToLower(hexStr))
	if err != nil {
		return nil, fmt.Errorf("%s hex decode: %w", fam, err)
	}
	return nlri.NewWireNLRI(fam, wireBytes, false)
}

// withdrawRegistry answers the announcement registry the three withdraw
// commands share, or the response their caller owes when the reactor is down.
//
// It exists because each form is now its own command with its own wire method,
// so the reactor check that used to sit once in front of a keyword switch is
// owed three times.
func withdrawRegistry(ctx *pluginserver.CommandContext) (*Registry, *plugin.Response, error) {
	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return nil, errResp, err
	}
	reg := getOrInitRegistry(ctx)
	if reg == nil {
		return nil, nil, errReactorNotAvailable
	}
	return reg, nil, nil
}

// handleWithdrawTag answers `withdraw tag <key> [value <value>]`.
func handleWithdrawTag(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	reg, errResp, err := withdrawRegistry(ctx)
	if err != nil {
		return errResp, err
	}
	return withdrawByTag(reg, args)
}

// handleWithdrawID answers `withdraw id <id>`.
//
// The id arrives as a SELECTOR rather than in args: the container and its leaf
// are both called `id`, so matchCommandTokens
// (internal/component/plugin/server/command.go) matched the keyword against the
// leaf of the same name and lifted the value out of the argument list. That is
// the same route `request l2tp outgoing-call remote <remote> called <called>`
// takes, and the model states the type in one place because of it.
func handleWithdrawID(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	reg, errResp, err := withdrawRegistry(ctx)
	if err != nil {
		return errResp, err
	}
	return withdrawByID(reg, ctx.Selector("id"))
}

// handleWithdrawAll answers `withdraw all [selector <selector>]`.
func handleWithdrawAll(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	reg, errResp, err := withdrawRegistry(ctx)
	if err != nil {
		return errResp, err
	}
	return withdrawEvery(reg, args)
}

func withdrawByTag(reg *Registry, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, errors.New("withdraw tag requires <key> [<value|*>]")
	}

	key := args[0]
	if key == "*" {
		n, err := reg.withdrawAllTags()
		if err != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldWithdrawn: n},
		}, nil
	}

	// The value is optional, so the model declares it as an optional leaf and
	// the generated line reads `withdraw tag <key> [value <value>]`. The
	// framework binds an optional leaf from its keyword or from a bare
	// positional and passes both through unchanged (validateCommandArgs), so
	// the keyword is dropped here rather than read as the value itself.
	rest := args[1:]
	if len(rest) >= 2 && strings.EqualFold(rest[0], kwValue) {
		rest = rest[1:]
	}
	value := "*"
	if len(rest) >= 1 {
		value = rest[0]
	}

	var n int
	var wErr error
	if value == "*" {
		n, wErr = reg.withdrawTagKey(key)
	} else {
		n, wErr = reg.withdrawTag(key, value)
	}
	if wErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: wErr.Error()}, wErr
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldWithdrawn: n},
	}, nil
}

func withdrawByID(reg *Registry, idArg string) (*plugin.Response, error) {
	if idArg == "" {
		return nil, errors.New("withdraw id requires <id>")
	}

	id, err := strconv.ParseUint(idArg, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}

	found, wErr := reg.withdrawEntryByID(id)
	if !found {
		var tb textbuf.Buffer
		msg := tb.Str("announcement id ").Uint(id).Str(" not found").String()
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  msg,
		}, errors.New(msg)
	}
	// The entry is gone from the registry either way, so a refused withdraw must
	// reach the caller here or reach nobody at all. Reporting it as done would
	// tell an operator a route was retracted while it is still on the peer.
	if wErr != nil {
		var tb textbuf.Buffer
		msg := tb.Str("announcement id ").Uint(id).Str(" was dropped from the registry but its withdraw failed: ").Err(wErr).String()
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  msg,
		}, wErr
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldWithdrawn: 1},
	}, nil
}

func withdrawEvery(reg *Registry, args []string) (*plugin.Response, error) {
	selFilter := ""
	if len(args) >= 2 {
		for i := range len(args) - 1 {
			if strings.EqualFold(args[i], kwSelector) {
				selFilter = args[i+1]
				break
			}
		}
	}

	n, err := reg.withdrawAll(selFilter)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldWithdrawn: n},
	}, nil
}

func handleShowAnnouncements(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	reg := getOrInitRegistry(ctx)
	if reg == nil {
		return nil, errReactorNotAvailable
	}

	var filter listFilter
	for i := 0; i < len(args)-1; i++ {
		switch strings.ToLower(args[i]) {
		case kwTag:
			filter.TagKey = args[i+1]
			i++
		case kwSelector:
			filter.Selector = args[i+1]
			i++
		case kwFamily:
			filter.Family = args[i+1]
			i++
		}
	}

	entries := reg.List(filter)
	result := make([]plugin.Map, 0, len(entries))
	for _, e := range entries {
		m := plugin.Map{
			"id":        e.ID,
			"tag-key":   e.TagKey,
			"tag-value": e.TagValue,
			"family":    e.Family,
			"selector":  e.Selector.String(),
			"source":    e.Source,
			"created":   e.CreatedAt.Format(time.RFC3339),
		}
		if e.ExpiresAt != nil {
			m["expires"] = e.ExpiresAt.Format(time.RFC3339)
		}
		result = append(result, m)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"announcements": result, "count": len(result)},
	}, nil
}
