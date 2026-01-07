// Package api provides the update text parser for the "update text" command format.
//
// The parser handles:
//
//	[attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]... [watchdog <name>]
//
// Attributes accumulate across sections; each nlri section captures a snapshot.
package api

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// UpdateText command keywords.
const (
	kwAttr        = "attr"
	kwNLRI        = "nlri"
	kwWatchdog    = "watchdog"
	kwAdd         = "add"
	kwDel         = "del"
	kwSet         = "set"
	kwNextHop     = "next-hop"
	kwNextHopSelf = "next-hop-self"
)

// isBoundaryKeyword returns true if token starts a new section.
func isBoundaryKeyword(token string) bool {
	return token == kwAttr || token == kwNLRI || token == kwWatchdog
}

// parsedAttrs tracks attribute state during parsing.
// Includes next-hop which is NOT part of PathAttributes.
type parsedAttrs struct {
	NextHop     netip.Addr
	NextHopSelf bool
	PathAttributes
}

// applySet merges other into a, overwriting only fields that are set in other.
func (a *parsedAttrs) applySet(other parsedAttrs) {
	if other.NextHop.IsValid() {
		a.NextHop = other.NextHop
	}
	if other.NextHopSelf {
		a.NextHopSelf = true
	}
	if other.Origin != nil {
		a.Origin = other.Origin
	}
	if other.LocalPreference != nil {
		a.LocalPreference = other.LocalPreference
	}
	if other.MED != nil {
		a.MED = other.MED
	}
	if other.ASPath != nil {
		a.ASPath = other.ASPath
	}
	if other.Communities != nil {
		a.Communities = other.Communities
	}
	if other.LargeCommunities != nil {
		a.LargeCommunities = other.LargeCommunities
	}
	if other.ExtendedCommunities != nil {
		a.ExtendedCommunities = other.ExtendedCommunities
	}
}

// validateListOp checks if other contains only list attributes.
// Returns error if scalar attrs or AS-PATH are set.
func (a *parsedAttrs) validateListOp(other parsedAttrs, scalarErr error) error {
	if other.ASPath != nil {
		return ErrASPathNotAddable
	}
	if other.Origin != nil {
		return fmt.Errorf("origin: %w", scalarErr)
	}
	if other.MED != nil {
		return fmt.Errorf("med: %w", scalarErr)
	}
	if other.LocalPreference != nil {
		return fmt.Errorf("local-preference: %w", scalarErr)
	}
	if other.NextHop.IsValid() {
		return fmt.Errorf("next-hop: %w", scalarErr)
	}
	if other.NextHopSelf {
		return fmt.Errorf("next-hop-self: %w", scalarErr)
	}
	return nil
}

// applyAdd appends list attributes from other into a.
// Returns error if non-list attributes are set in other.
func (a *parsedAttrs) applyAdd(other parsedAttrs) error {
	if err := a.validateListOp(other, ErrAddOnScalar); err != nil {
		return err
	}
	if other.Communities != nil {
		a.Communities = append(a.Communities, other.Communities...)
	}
	if other.LargeCommunities != nil {
		a.LargeCommunities = append(a.LargeCommunities, other.LargeCommunities...)
	}
	if other.ExtendedCommunities != nil {
		a.ExtendedCommunities = append(a.ExtendedCommunities, other.ExtendedCommunities...)
	}
	return nil
}

// applyDel removes list attributes in other from a.
// Returns error if non-list attributes are set in other.
func (a *parsedAttrs) applyDel(other parsedAttrs) error {
	if err := a.validateListOp(other, ErrDelOnScalar); err != nil {
		return err
	}
	if other.Communities != nil {
		a.Communities = removeFromSlice(a.Communities, other.Communities)
	}
	if other.LargeCommunities != nil {
		a.LargeCommunities = removeFromSlice(a.LargeCommunities, other.LargeCommunities)
	}
	if other.ExtendedCommunities != nil {
		a.ExtendedCommunities = removeFromSlice(a.ExtendedCommunities, other.ExtendedCommunities)
	}
	return nil
}

// snapshot returns a deep copy of the current attribute state.
// MUST deep copy slices AND pointers to isolate each group from later modifications.
func (a *parsedAttrs) snapshot() (PathAttributes, netip.Addr, bool) {
	var pa PathAttributes
	// Deep copy pointer fields
	if a.Origin != nil {
		v := *a.Origin
		pa.Origin = &v
	}
	if a.LocalPreference != nil {
		v := *a.LocalPreference
		pa.LocalPreference = &v
	}
	if a.MED != nil {
		v := *a.MED
		pa.MED = &v
	}
	if a.ASPath != nil {
		pa.ASPath = make([]uint32, len(a.ASPath))
		copy(pa.ASPath, a.ASPath)
	}
	if a.Communities != nil {
		pa.Communities = make([]uint32, len(a.Communities))
		copy(pa.Communities, a.Communities)
	}
	if a.LargeCommunities != nil {
		pa.LargeCommunities = make([]LargeCommunity, len(a.LargeCommunities))
		copy(pa.LargeCommunities, a.LargeCommunities)
	}
	if a.ExtendedCommunities != nil {
		pa.ExtendedCommunities = make([]attribute.ExtendedCommunity, len(a.ExtendedCommunities))
		copy(pa.ExtendedCommunities, a.ExtendedCommunities)
	}
	return pa, a.NextHop, a.NextHopSelf
}

// removeFromSlice removes all elements in remove from slice.
func removeFromSlice[T comparable](slice, remove []T) []T {
	if len(slice) == 0 || len(remove) == 0 {
		return slice
	}
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if !slices.Contains(remove, v) {
			result = append(result, v)
		}
	}
	return result
}

// ParseUpdateText parses the "update text" command format.
// Returns the parsed result or an error.
func ParseUpdateText(args []string) (*UpdateTextResult, error) {
	var accum parsedAttrs
	var groups []NLRIGroup
	var watchdog string
	i := 0

	for i < len(args) {
		switch args[i] { //nolint:gosec // G602 false positive: loop condition guards access
		case kwAttr:
			mode, attrs, consumed, err := parseAttrSection(args[i:])
			if err != nil {
				return nil, err
			}

			switch mode {
			case kwSet:
				accum.applySet(attrs)
			case kwAdd:
				if err := accum.applyAdd(attrs); err != nil {
					return nil, err
				}
			case kwDel:
				if err := accum.applyDel(attrs); err != nil {
					return nil, err
				}
			}
			i += consumed

		case kwNLRI:
			family, announce, withdraw, consumed, err := parseNLRISection(args[i:])
			if err != nil {
				return nil, err
			}

			attrs, nh, nhSelf := accum.snapshot()
			groups = append(groups, NLRIGroup{
				Family:      family,
				Announce:    announce,
				Withdraw:    withdraw,
				Attrs:       attrs,
				NextHop:     nh,
				NextHopSelf: nhSelf,
			})
			i += consumed

		case kwWatchdog:
			if i+1 >= len(args) {
				return nil, errors.New("missing watchdog name")
			}
			watchdog = args[i+1]
			i += 2

		default:
			return nil, fmt.Errorf("unexpected token: %s", args[i]) //nolint:gosec // G602 false positive: loop guards access
		}
	}

	return &UpdateTextResult{Groups: groups, WatchdogName: watchdog}, nil
}

// parseAttrSection parses attr <mode> <key> <value>... until boundary keyword.
// Returns mode, parsed attrs, consumed token count, and any error.
func parseAttrSection(args []string) (string, parsedAttrs, int, error) {
	// args[0] = "attr"
	if len(args) < 2 {
		return "", parsedAttrs{}, 0, ErrMissingAttrMode
	}
	mode := args[1]
	if mode != kwSet && mode != kwAdd && mode != kwDel {
		return "", parsedAttrs{}, 0, ErrInvalidAttrMode
	}

	consumed := 2 // "attr" + mode
	i := 2
	var attrs parsedAttrs

	for i < len(args) {
		key := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section
		if isBoundaryKeyword(key) {
			break
		}

		// Try next-hop (not in parseCommonAttribute)
		switch key {
		case kwNextHop:
			if i+1 >= len(args) {
				return "", parsedAttrs{}, 0, fmt.Errorf("missing next-hop value")
			}
			addr, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return "", parsedAttrs{}, 0, fmt.Errorf("invalid next-hop: %w", err)
			}
			attrs.NextHop = addr
			i += 2
			consumed += 2
			continue

		case kwNextHopSelf:
			attrs.NextHopSelf = true
			i++
			consumed++
			continue
		}

		// Try parseCommonAttribute for standard attrs
		extra, err := parseCommonAttribute(key, args, i, &attrs.PathAttributes)
		if err != nil {
			return "", parsedAttrs{}, 0, err
		}
		if extra > 0 {
			i += 1 + extra // key + extra args consumed
			consumed += 1 + extra
			continue
		}

		// Unknown attribute
		return "", parsedAttrs{}, 0, fmt.Errorf("%w: %s", ErrUnknownAttribute, key)
	}

	return mode, attrs, consumed, nil
}

// parseNLRISection parses nlri <family> [add <prefix>...]... [del <prefix>...]...
// Returns family, announce list, withdraw list, consumed token count, and any error.
func parseNLRISection(args []string) (nlri.Family, []nlri.NLRI, []nlri.NLRI, int, error) {
	// args[0] = "nlri"
	if len(args) < 2 {
		return nlri.Family{}, nil, nil, 0, ErrInvalidFamily
	}

	family, ok := nlri.ParseFamily(args[1])
	if !ok {
		return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: %s", ErrInvalidFamily, args[1])
	}

	// Check if family is supported
	if !isSupportedFamily(family) {
		return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: %s", ErrFamilyNotSupported, args[1])
	}

	consumed := 2 // "nlri" + family
	i := 2
	mode := "" // "", "add", or "del"
	var announce, withdraw []nlri.NLRI

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section
		if isBoundaryKeyword(token) {
			break
		}

		// Mode switches
		if token == kwAdd {
			mode = kwAdd
			i++
			consumed++
			continue
		}
		if token == kwDel {
			mode = kwDel
			i++
			consumed++
			continue
		}

		// Must have a mode before prefixes
		if mode == "" {
			return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: got %q", ErrMissingAddDel, token)
		}

		// Parse prefix based on family
		n, extra, err := parseINETNLRI(token, family)
		if err != nil {
			return nlri.Family{}, nil, nil, 0, err
		}

		if mode == kwAdd {
			announce = append(announce, n)
		} else {
			withdraw = append(withdraw, n)
		}
		i += 1 + extra
		consumed += 1 + extra
	}

	// Must have at least one prefix
	if len(announce) == 0 && len(withdraw) == 0 {
		return nlri.Family{}, nil, nil, 0, ErrEmptyNLRISection
	}

	return family, announce, withdraw, consumed, nil
}

// parseINETNLRI parses a single prefix for unicast/multicast families.
// Returns the NLRI, extra args consumed (always 0 for INET), and any error.
// The second return value exists for future family parsers (labeled, VPN)
// that consume additional arguments.
//
//nolint:unparam // int return value reserved for future families
func parseINETNLRI(token string, family nlri.Family) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", ErrFamilyMismatch, family)
	}

	return nlri.NewINET(family, prefix, 0), 0, nil // 0 extra args consumed
}

// isSupportedFamily returns true if the family is supported in text mode.
func isSupportedFamily(f nlri.Family) bool {
	switch f {
	case nlri.IPv4Unicast, nlri.IPv6Unicast, nlri.IPv4Multicast, nlri.IPv6Multicast:
		return true
	default:
		return false
	}
}

// handleUpdate dispatches update subcommands by encoding.
// Syntax: peer <addr> update <encoding> ...
func handleUpdate(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: peer <addr> update <text|hex|b64|cbor>")
	}

	encoding := strings.ToLower(args[0])
	switch encoding {
	case "text":
		return handleUpdateText(ctx, args[1:])
	case "hex", "b64", "cbor":
		return nil, fmt.Errorf("wire encoding %s not yet implemented", encoding)
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
}

// handleUpdateText handles: peer <addr> update text ...
// Parses the update text format and dispatches to reactor batch methods.
// RFC 4271 Section 4.3: UPDATE Message Format.
func handleUpdateText(ctx *CommandContext, args []string) (*Response, error) {
	result, err := ParseUpdateText(args)
	if err != nil {
		return &Response{Status: statusError, Data: err.Error()}, err
	}

	if result.WatchdogName != "" {
		errMsg := "watchdog not yet implemented for update text"
		return &Response{Status: statusError, Data: errMsg}, errors.New(errMsg)
	}

	peerSelector := ctx.PeerSelector()

	var announced, withdrawn int
	var warnings []string

	for _, group := range result.Groups {
		if len(group.Announce) > 0 {
			batch := NLRIBatch{
				Family:      group.Family,
				NLRIs:       group.Announce,
				NextHop:     group.NextHop,
				NextHopSelf: group.NextHopSelf,
				Attrs:       group.Attrs,
			}
			if err := ctx.Reactor.AnnounceNLRIBatch(peerSelector, batch); err != nil {
				if errors.Is(err, ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("announce %v: %s", group.Family, err))
					continue
				}
				return &Response{Status: statusError, Data: err.Error()}, err
			}
			announced += len(group.Announce)
		}
		if len(group.Withdraw) > 0 {
			batch := NLRIBatch{
				Family: group.Family,
				NLRIs:  group.Withdraw,
			}
			if err := ctx.Reactor.WithdrawNLRIBatch(peerSelector, batch); err != nil {
				if errors.Is(err, ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("withdraw %v: %s", group.Family, err))
					continue
				}
				return &Response{Status: statusError, Data: err.Error()}, err
			}
			withdrawn += len(group.Withdraw)
		}
	}

	if announced == 0 && withdrawn == 0 {
		msg := "no routes to announce or withdraw"
		if len(warnings) > 0 {
			msg = strings.Join(warnings, "; ")
		}
		return &Response{
			Status: "warning",
			Data:   msg,
		}, nil
	}

	// Include warnings in success response if any
	respData := map[string]any{
		"announced": announced,
		"withdrawn": withdrawn,
	}
	if len(warnings) > 0 {
		respData["warnings"] = warnings
	}

	return &Response{
		Status: statusDone,
		Data:   respData,
	}, nil
}
