// Design: plan/learned/824-rib-feed-replay-batch.md — stateful cursor for replay batching
// Overview: update_text.go — handleUpdate dispatch, parsedAttrs, snapshot, parseCommonAttributeText
// Related: update_text_nlri.go — parseNLRISection (reused unchanged)
package update

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

var (
	errCursorNoCursorState = errors.New("no cursor state; first update cursor must include attributes")
	errCursorMissingNLRI   = errors.New("update cursor requires nlri section (or done)")
	errCursorDelMissingKW  = errors.New("del requires attribute keyword")
)

// cursors stores per-(process, peer) attribute cursor state.
// Key: "processName:peerSelector".
var cursors sync.Map

func cursorKey(processName, peer string) string {
	return processName + ":" + peer
}

// ClearProcessCursors removes all cursor entries for a given process name.
// Called from cleanupProcess when a plugin exits without sending "done".
func ClearProcessCursors(processName string) {
	prefix := processName + ":"
	cursors.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			cursors.Delete(key)
		}
		return true
	})
}

// handleUpdateCursor handles: peer <addr> update cursor ...
// Maintains a stateful attribute cursor per (plugin, peer) pair.
// Delta encoding: only changed attributes need to be sent after the first command.
func handleUpdateCursor(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) == 0 {
		return nil, errCursorMissingNLRI
	}

	processName := ""
	if ctx.Process != nil {
		processName = ctx.Process.Name()
	}
	peer := ctx.PeerSelector()
	key := cursorKey(processName, peer)

	args = resolveAliases(args)

	if args[0] == "done" {
		cursors.Delete(key)
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"cursor": "cleared"},
		}, nil
	}

	existing, hasCursor := cursors.Load(key)
	var attrs *parsedAttrs
	if hasCursor {
		src, ok := existing.(*parsedAttrs)
		if ok {
			attrs = deepCopyAttrs(src)
		}
	}

	i := 0
	hasAttrChanges := false

	for i < len(args) {
		token := args[i]

		if token == kwNLRI {
			break
		}

		if token == kwDel {
			if i+1 >= len(args) {
				return nil, errCursorDelMissingKW
			}
			delTarget := textparse.ResolveAlias(args[i+1])
			if attrs != nil {
				clearAttr(attrs, delTarget)
			}
			hasAttrChanges = true
			i += 2
			continue
		}

		if isAttributeKeyword(token) || token == textparse.KWNextHop {
			if attrs == nil {
				attrs = &parsedAttrs{}
			}
			if token == textparse.KWNextHop {
				consumed, parseErr := parseNhopFlat(args[i:], attrs)
				if parseErr != nil {
					return nil, parseErr
				}
				i += consumed
			} else {
				extra, parseErr := parseCommonAttributeText(token, args, i, attrs)
				if parseErr != nil {
					return nil, parseErr
				}
				if extra == 0 {
					return nil, fmt.Errorf("missing value for %s", token)
				}
				i += 1 + extra
			}
			hasAttrChanges = true
			continue
		}

		return nil, fmt.Errorf("unexpected token in cursor command: %s", token)
	}

	if i >= len(args) || args[i] != kwNLRI {
		return nil, errCursorMissingNLRI
	}

	if attrs == nil {
		return nil, errCursorNoCursorState
	}

	if hasAttrChanges || !hasCursor {
		cursors.Store(key, deepCopyAttrs(attrs))
	}

	wire, nh, nlriAcc := attrs.snapshot()

	var groups []bgptypes.NLRIGroup
	for i < len(args) && args[i] == kwNLRI {
		result, parseErr := parseNLRISection(args[i:], nlriAcc)
		if parseErr != nil {
			return nil, parseErr
		}
		if len(result.Announce) > 0 || len(result.Withdraw) > 0 {
			groups = append(groups, bgptypes.NLRIGroup{
				Family:   result.Family,
				Announce: result.Announce,
				NextHop:  nh,
				Wire:     wire,
			})
		}
		i += result.Consumed
	}

	if len(groups) == 0 {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"announced": 0},
		}, nil
	}

	return DispatchNLRIGroups(ctx, groups)
}

// deepCopyAttrs creates an independent copy of parsedAttrs.
func deepCopyAttrs(src *parsedAttrs) *parsedAttrs {
	dst := *src
	if src.ASPath != nil {
		dst.ASPath = make([]uint32, len(src.ASPath))
		copy(dst.ASPath, src.ASPath)
	}
	if src.Communities != nil {
		dst.Communities = make([]uint32, len(src.Communities))
		copy(dst.Communities, src.Communities)
	}
	if src.LargeCommunities != nil {
		dst.LargeCommunities = make([]bgptypes.LargeCommunity, len(src.LargeCommunities))
		copy(dst.LargeCommunities, src.LargeCommunities)
	}
	if src.ExtendedCommunities != nil {
		dst.ExtendedCommunities = make([]attribute.ExtendedCommunity, len(src.ExtendedCommunities))
		copy(dst.ExtendedCommunities, src.ExtendedCommunities)
	}
	if src.Labels != nil {
		dst.Labels = make([]uint32, len(src.Labels))
		copy(dst.Labels, src.Labels)
	}
	return &dst
}

// clearAttr removes an attribute from the cursor by keyword.
func clearAttr(attrs *parsedAttrs, keyword string) {
	switch keyword {
	case kwOrigin:
		attrs.Origin = nil
	case kwMED:
		attrs.MED = nil
	case kwLocalPref:
		attrs.LocalPreference = nil
	case kwASPath:
		attrs.ASPath = nil
	case kwCommunity:
		attrs.Communities = nil
	case kwLargeCommunity:
		attrs.LargeCommunities = nil
	case kwExtendedCommunity:
		attrs.ExtendedCommunities = nil
	case textparse.KWNextHop:
		attrs.NextHop = netip.Addr{}
		attrs.NextHopSelf = false
	}
}
