// Design: docs/architecture/bgp/filter-irr.md -- IRR plugin command handlers (show bgp irr)

package filter_irr

import (
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/asn"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	statusDone  = "done"
	statusError = "error"
)

// handleCommand answers the three show commands. The filter fetches nothing,
// so it serves no update or clear verb: `update irr ...` and `clear irr ...`
// belong to the irr plugin, which owns the stored copies.
func (plug *irrPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case cmdShowIRR:
		return plug.showIRR()
	case cmdShowIRRPrefix:
		return plug.showIRRPrefix(args)
	case cmdShowIRRCheck:
		return plug.showIRRCheck(args)
	}
	return statusError, nil, errors.New("unknown command")
}

// showIRR answers one row per enrolled ASN: the set it is filtered by, whether
// a stored copy is enforced, and which one. "ok" is a stored copy holding
// prefixes, "error" a stored value the filter could not read (it keeps the
// copy it enforced before, if any), and "pending" nothing stored yet: the irr
// plugin has not fetched the set, or `clear irr` removed it.
func (plug *irrPlugin) showIRR() (string, any, error) {
	plug.mu.RLock()
	defer plug.mu.RUnlock()

	b := textbuf.Get()
	defer b.Release()

	sortedASNs := make([]uint32, 0, len(plug.byASN))
	for number := range plug.byASN {
		sortedASNs = append(sortedASNs, number)
	}
	slices.Sort(sortedASNs)

	b.Str(`{"entries":[`)
	for i, number := range sortedASNs {
		st := plug.byASN[number]
		if i > 0 {
			b.Byte(',')
		}
		// The AS number is a row an operator reads, so it carries the notation
		// bgp/as-notation selected (asn.AppendJSON states the rule).
		b.Str(`{"asn":`).Str(asn.JSONValue(st.asn))
		b.Str(`,"as-set":`).Quoted(st.asSet)
		switch {
		case st.readErr != "":
			b.Str(`,"status":"error"`)
			b.Str(`,"error":`).Quoted(st.readErr)
		case st.list != nil && len(st.list.entries) > 0:
			b.Str(`,"status":"ok"`)
		default:
			b.Str(`,"status":"pending"`)
		}
		b.Str(`,"ipv4-count":`).Int(int64(st.v4Count))
		b.Str(`,"ipv6-count":`).Int(int64(st.v6Count))
		if st.list != nil {
			b.Str(`,"server":`).Quoted(st.pointer.Server)
			b.Str(`,"generation":`).Uint32(st.pointer.Generation)
			writeShowTime(b, "checked-at", st.checkedAt)
			writeShowTime(b, "refreshed-at", st.refreshedAt)
		}
		b.Str(`,"peers":[`)
		for j, addr := range st.peerAddrs {
			if j > 0 {
				b.Byte(',')
			}
			b.Quoted(addr)
		}
		b.Str(`]}`)
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

// writeShowTime writes one RFC 3339 time field, or nothing for an unset time.
func writeShowTime(b *textbuf.Buffer, key string, at time.Time) {
	if at.IsZero() {
		return
	}
	b.Str(`,"`).Str(key).Str(`":"`).Str(at.UTC().Format(time.RFC3339)).Byte('"')
}

func (plug *irrPlugin) showIRRPrefix(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: show bgp irr prefix <peer>")
	}
	peerAddr := args[0]

	plug.mu.RLock()
	defer plug.mu.RUnlock()

	for _, st := range plug.byASN {
		if slices.Contains(st.peerAddrs, peerAddr) {
			return plug.renderPrefixes(st)
		}
	}
	return statusError, nil, errors.New("peer not found in IRR state")
}

func (plug *irrPlugin) renderPrefixes(st *asnState) (string, any, error) {
	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"asn":`).Str(asn.JSONValue(st.asn))
	b.Str(`,"as-set":`).Quoted(st.asSet)
	b.Str(`,"prefixes":[`)
	if st.list != nil {
		for i, e := range st.list.entries {
			if i > 0 {
				b.Byte(',')
			}
			b.Byte('"').Str(e.prefix.String()).Byte('"')
		}
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) showIRRCheck(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, nil, errors.New("usage: show bgp irr check <peer> <prefix>")
	}
	peerAddr := args[0]
	prefixStr := args[1]

	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return statusError, nil, errors.New("invalid prefix")
	}

	plug.mu.RLock()
	defer plug.mu.RUnlock()

	var matchSt *asnState
	for _, st := range plug.byASN {
		if slices.Contains(st.peerAddrs, peerAddr) {
			matchSt = st
			break
		}
	}
	if matchSt == nil {
		return statusError, nil, errors.New("peer not found in IRR state")
	}

	accepted := false
	matchedEntry := ""
	if matchSt.list != nil {
		accepted = evaluatePrefix(matchSt.list.entries, prefix)
		if accepted {
			matchedEntry = findMatchingEntry(matchSt.list.entries, prefix)
		}
	}
	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"prefix":"`).Str(prefixStr).Byte('"')
	b.Str(`,"asn":`).Str(asn.JSONValue(matchSt.asn))
	b.Str(`,"accepted":`).Bool(accepted)
	if matchedEntry != "" {
		b.Str(`,"matched-entry":"`).Str(matchedEntry).Byte('"')
	}
	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}
