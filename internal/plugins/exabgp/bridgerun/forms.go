// Design: docs/architecture/exabgp-bridge.md -- per-script message rendering forms (`--form`) and bus event format selection.
// Related: fleet.go -- Fleet.Broadcast reads FormSpecs.For for each delivered message.

package bridgerun

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	formDirectionReceive = "receive"
	formDirectionSend    = "send"
	busMsgUpdate         = bgpevents.Namespace + ":" + bgpevents.EventUpdate
)

// ErrInvalidFormSpec is returned when a `--form` flag argument is malformed or
// names an unknown form modifier or invalid neighbor address.
var ErrInvalidFormSpec = errors.New("invalid --form specification")

// FormSpecs holds the parsed `--form <[neighbor:]direction-form>` modifiers for
// one ExaBGP script. The zero value requests no wire-form overrides, so For
// returns bridge.FormParsed and NeedsBytes returns false.
//
// Safe for concurrent read access once constructed by ParseFormSpecs.
type FormSpecs struct {
	global  map[string]bool
	perPeer map[netip.Addr]map[string]bool
}

// ParseFormSpecs parses repeatable `--form` values of the form
// `<direction>-<parsed|packets|consolidate>` (global across all peers) or
// `<neighbor>:<direction>-<parsed|packets|consolidate>` (scoped to one IPv4 or
// IPv6 peer).
func ParseFormSpecs(raw []string) (FormSpecs, error) {
	var specs FormSpecs
	for _, entry := range raw {
		if entry == "" {
			return FormSpecs{}, fmt.Errorf("%w: empty value", ErrInvalidFormSpec)
		}
		if bridge.IsFormAPIKey(entry) {
			if specs.global == nil {
				specs.global = make(map[string]bool)
			}
			specs.global[entry] = true
			continue
		}
		// Split on the LAST colon so bare IPv6 addresses (e.g.
		// "2001:db8::1:receive-packets") parse cleanly without brackets.
		sep := strings.LastIndexByte(entry, ':')
		if sep <= 0 {
			return FormSpecs{}, fmt.Errorf("%w %q: expected <receive|send>-<parsed|packets|consolidate> or <neighbor>:<form>", ErrInvalidFormSpec, entry)
		}
		peerPart := entry[:sep]
		token := entry[sep+1:]
		if !bridge.IsFormAPIKey(token) {
			return FormSpecs{}, fmt.Errorf("%w %q: unknown form %q", ErrInvalidFormSpec, entry, token)
		}
		addr, err := netip.ParseAddr(peerPart)
		if err != nil {
			return FormSpecs{}, fmt.Errorf("%w %q: invalid neighbor address %q: %w", ErrInvalidFormSpec, entry, peerPart, err)
		}
		if specs.perPeer == nil {
			specs.perPeer = make(map[netip.Addr]map[string]bool)
		}
		peerForms := specs.perPeer[addr]
		if peerForms == nil {
			peerForms = make(map[string]bool)
			specs.perPeer[addr] = peerForms
		}
		peerForms[token] = true
	}
	return specs, nil
}

// For returns the bridge.Forms bitmask for one peer and ExaBGP direction
// ("receive" or "send"). When no form spec matches that (peer, direction), For
// returns bridge.FormParsed.
func (f FormSpecs) For(peer, direction string) bridge.Forms {
	if len(f.global) == 0 && len(f.perPeer) == 0 {
		return bridge.FormParsed
	}
	var peerForms map[string]bool
	if len(f.perPeer) > 0 {
		if addr, err := netip.ParseAddr(peer); err == nil {
			peerForms = f.perPeer[addr]
		}
	}
	return bridge.FormsFor(direction, func(apiKey string) bool {
		if peerForms[apiKey] {
			return true
		}
		return f.global[apiKey]
	})
}

// NeedsBytes reports whether any configured global or per-neighbor form spec
// requests wire packet bytes (`packets` or effective `consolidate`).
func (f FormSpecs) NeedsBytes() bool {
	for _, direction := range []string{formDirectionReceive, formDirectionSend} {
		if bridge.FormsFor(direction, func(apiKey string) bool { return f.global[apiKey] })&(bridge.FormPackets|bridge.FormConsolidated) != 0 {
			return true
		}
		for _, peerForms := range f.perPeer {
			granted := func(apiKey string) bool {
				if peerForms[apiKey] {
					return true
				}
				return f.global[apiKey]
			}
			if bridge.FormsFor(direction, granted)&(bridge.FormPackets|bridge.FormConsolidated) != 0 {
				return true
			}
		}
	}
	return false
}

// EventFormat answers the wire event format the wrapper requests from the
// engine (`plugin.FormatFull` or `plugin.FormatParsed`) given its compiled
// participant filter and configured `--form` modifiers.
//
// It returns `full` when filter is nil (unscoped endpoint), when forms requests
// raw packet bytes (`NeedsBytes`), or when filter allows receiving `bgp:update`
// (because decoded UPDATE lines read AS_PATH segments and extended community
// values off the message's wire bytes). Otherwise it returns `parsed`.
func EventFormat(filter *rpc.BusParticipantFilter, forms FormSpecs) string {
	if filter == nil {
		return plugin.FormatFull
	}
	if forms.NeedsBytes() {
		return plugin.FormatFull
	}
	if filter.ReceiveNone {
		return plugin.FormatParsed
	}
	if filter.ReceiveFromNone {
		return plugin.FormatParsed
	}
	for _, rule := range filter.ReceiveRules {
		if rule.Message != busMsgUpdate {
			continue
		}
		if !rule.Allowed {
			return plugin.FormatParsed
		}
		if rule.TargetNone {
			return plugin.FormatParsed
		}
		return plugin.FormatFull
	}
	if filter.ReceiveAll && filter.ReceiveFromAll {
		return plugin.FormatFull
	}
	return plugin.FormatParsed
}
