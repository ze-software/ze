// Design: plan/spec-cp-survival-4-flowspec-origination.md -- on-demand route origination CLI verbs

package announce

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	kwTag       = "tag"
	kwFor       = "for"
	kwAll       = "all"
	kwID        = "id"
	kwSelector  = "selector"
	kwFamily    = "family"
	kwNextHop   = "next-hop"
	kwCommunity = "community"
	kwSelf      = "self"
)

const maxTagLen = 128

var (
	errMissingFamily          = errors.New("missing family")
	errMissingPrefix          = errors.New("missing prefix")
	errUnknownFamily          = errors.New("unknown family")
	errMissingWithdrawKeyword = errors.New("withdraw requires: tag, id, or all")
	errTagTooLong             = errors.New("tag key or value exceeds 128 characters")
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
	globalRegistry = NewRegistry(func(sel *selector.Selector, batch bgptypes.NLRIBatch) error {
		return bgpReactor.WithdrawNLRIBatch(sel, batch)
	})
	return globalRegistry
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:announce", Handler: handleAnnounce, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:withdraw", Handler: handleWithdraw},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:show-announcements", Handler: handleShowAnnouncements},
	)
}

func handleAnnounce(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	bgpReactor, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) < 1 {
		return nil, errMissingFamily
	}

	reg := getOrInitRegistry(ctx)
	if reg == nil {
		return nil, errReactorNotAvailable
	}

	familyName := strings.ToLower(args[0])
	remaining := args[1:]

	switch familyName {
	case "unicast":
		return handleAnnounceUnicast(ctx, bgpReactor, reg, remaining)
	case "blackhole":
		return handleAnnounceBlackhole(ctx, bgpReactor, reg, remaining)
	case "flowspec":
		return handleAnnounceFlowspec(ctx, bgpReactor, reg, remaining)
	default:
		return nil, fmt.Errorf("%w: %s (valid: unicast, blackhole, flowspec)", errUnknownFamily, familyName)
	}
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

func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	secs, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("expected duration (e.g. 300s) or seconds: %s", s)
	}
	return time.Duration(secs) * time.Second, nil //nolint:gosec // G115: bounded by uint64 range
}

func prefixToFamily(prefix netip.Prefix) family.Family {
	if prefix.Addr().Is6() {
		return family.IPv6Unicast
	}
	return family.IPv4Unicast
}

func announceAndTrack(reg *Registry, bgpReactor bgptypes.BGPReactor, sel *selector.Selector, batch bgptypes.NLRIBatch, opts announceOpts, source string) (*plugin.Response, error) {
	if err := bgpReactor.AnnounceNLRIBatch(sel, batch); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	if opts.tagKey == "" {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"announced": 1},
		}, nil
	}

	id, err := reg.Announce(opts.tagKey, opts.tagValue, sel, batch, source, opts.duration)
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
	fam := prefixToFamily(prefix)
	inet := nlri.NewINET(fam, prefix, 0)

	batch := bgptypes.NLRIBatch{
		Family:  fam,
		NLRIs:   []nlri.NLRI{inet},
		NextHop: nextHop,
		Attrs:   builder,
	}

	return announceAndTrack(reg, bgpReactor, sel, batch, opts, "cli")
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

	sel := selector.ParseDefault(ctx.PeerSelector())
	fam := prefixToFamily(prefix)
	inet := nlri.NewINET(fam, prefix, 0)

	batch := bgptypes.NLRIBatch{
		Family:  fam,
		NLRIs:   []nlri.NLRI{inet},
		NextHop: bgptypes.NewNextHopSelf(),
		Attrs:   builder,
	}

	return announceAndTrack(reg, bgpReactor, sel, batch, opts, "cli")
}

func handleAnnounceFlowspec(_ *pluginserver.CommandContext, _ bgptypes.BGPReactor, _ *Registry, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "flowspec announce not yet implemented",
	}, errors.New("flowspec announce not yet implemented")
}

func handleWithdraw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	reg := getOrInitRegistry(ctx)
	if reg == nil {
		return nil, errReactorNotAvailable
	}

	if len(args) < 1 {
		return nil, errMissingWithdrawKeyword
	}

	keyword := strings.ToLower(args[0])
	switch keyword {
	case kwTag:
		return handlewithdrawTag(reg, args[1:])
	case kwID:
		return handleWithdrawID(reg, args[1:])
	case kwAll:
		return handlewithdrawAll(reg, args[1:])
	default:
		return nil, fmt.Errorf("%w (got: %s)", errMissingWithdrawKeyword, keyword)
	}
}

func handlewithdrawTag(reg *Registry, args []string) (*plugin.Response, error) {
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
			Data:   plugin.Map{"withdrawn": n},
		}, nil
	}

	value := "*"
	if len(args) >= 2 {
		value = args[1]
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
		Data:   plugin.Map{"withdrawn": n},
	}, nil
}

func handleWithdrawID(reg *Registry, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, errors.New("withdraw id requires <N>")
	}

	id, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}

	found := reg.withdrawEntryByID(id)
	if !found {
		var tb textbuf.Buffer
		msg := tb.Str("announcement id ").Uint(id).Str(" not found").String()
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  msg,
		}, errors.New(msg)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"withdrawn": 1},
	}, nil
}

func handlewithdrawAll(reg *Registry, args []string) (*plugin.Response, error) {
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
		Data:   plugin.Map{"withdrawn": n},
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
