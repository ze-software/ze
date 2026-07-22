// Design: plan/learned/896-filter-irr.md -- IRR plugin command handlers (show/update bgp irr)

package filter_irr

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	statusDone  = "done"
	statusError = "error"
)

func (plug *irrPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case "show bgp irr":
		return plug.showIRR()
	case "show bgp irr prefix":
		return plug.showIRRPrefix(args)
	case "show bgp irr check":
		return plug.showIRRCheck(args)
	case "update bgp irr all":
		plug.refreshAllNow()
		if err := plug.anyRefreshError(); err != nil {
			return statusError, nil, err
		}
		return statusDone, json.RawMessage(`{"refreshed":"all"}`), nil
	case "update bgp irr asn":
		return plug.updateASN(args)
	case "update bgp irr as-set":
		return plug.updateASSet(args)
	}
	return statusError, nil, errors.New("unknown command")
}

func (plug *irrPlugin) showIRR() (string, any, error) {
	plug.mu.RLock()
	defer plug.mu.RUnlock()

	b := textbuf.Get()
	defer b.Release()

	server := defaultServer
	if plug.config != nil {
		server = plug.config.Server
	}
	b.Str(`{"server":`).Quoted(server)
	if !plug.lastRefresh.IsZero() {
		b.Str(`,"last-refresh":"`).Str(plug.lastRefresh.Format(time.RFC3339)).Byte('"')
	}
	if !plug.nextRefresh.IsZero() {
		b.Str(`,"next-refresh":"`).Str(plug.nextRefresh.Format(time.RFC3339)).Byte('"')
	}
	sortedASNs := make([]uint32, 0, len(plug.byASN))
	for asn := range plug.byASN {
		sortedASNs = append(sortedASNs, asn)
	}
	slices.Sort(sortedASNs)

	b.Str(`,"entries":[`)
	first := true
	for _, asn := range sortedASNs {
		st := plug.byASN[asn]
		if !first {
			b.Byte(',')
		}
		first = false
		b.Str(`{"asn":`).Uint32(st.asn)
		b.Str(`,"as-set":`).Quoted(st.asSet)
		switch {
		case st.lastErr != "":
			b.Str(`,"status":"error"`)
			b.Str(`,"error":`).Quoted(st.lastErr)
		case st.list != nil && len(st.list.entries) > 0:
			b.Str(`,"status":"ok"`)
		default:
			b.Str(`,"status":"pending"`)
		}
		b.Str(`,"ipv4-count":`).Int(int64(st.v4Count))
		b.Str(`,"ipv6-count":`).Int(int64(st.v6Count))
		if !st.lastOK.IsZero() {
			b.Str(`,"last-refresh":"`).Str(st.lastOK.Format(time.RFC3339)).Byte('"')
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
	b.Str(`{"asn":`).Uint32(st.asn)
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
	b.Str(`,"asn":`).Uint32(matchSt.asn)
	b.Str(`,"accepted":`).Bool(accepted)
	if matchedEntry != "" {
		b.Str(`,"matched-entry":"`).Str(matchedEntry).Byte('"')
	}
	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) anyRefreshError() error {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	asns := make([]uint32, 0, len(plug.byASN))
	for asn := range plug.byASN {
		asns = append(asns, asn)
	}
	slices.Sort(asns)
	for _, asn := range asns {
		if st := plug.byASN[asn]; st != nil && st.lastErr != "" {
			return fmt.Errorf("ASN %d: %s", asn, st.lastErr)
		}
	}
	return nil
}

func (plug *irrPlugin) asnRefreshError(asn uint32) string {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	if st := plug.byASN[asn]; st != nil {
		return st.lastErr
	}
	return ""
}

func (plug *irrPlugin) updateASN(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update bgp irr asn <asn>")
	}
	// readUint parses the full 64-bit range, so bound it here: narrowing an
	// out-of-range value would look up a different ASN than the operator typed.
	v, ok := readUint(args[0])
	if !ok || v > math.MaxUint32 {
		return statusError, nil, errors.New("invalid ASN (expected 0..4294967295)")
	}
	asn := uint32(v)
	plug.mu.RLock()
	_, known := plug.byASN[asn]
	plug.mu.RUnlock()
	if !known {
		return statusError, nil, fmt.Errorf("no IRR-filtered peer with ASN %d", asn)
	}
	plug.refreshASN(asn)
	if msg := plug.asnRefreshError(asn); msg != "" {
		return statusError, nil, fmt.Errorf("ASN %d refresh failed: %s", asn, msg)
	}
	var tb textbuf.Buffer
	return statusDone, json.RawMessage(tb.Str(`{"refreshed":"asn `).Uint32(asn).Str(`"}`).String()), nil
}

func (plug *irrPlugin) updateASSet(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update bgp irr as-set <as-set>")
	}
	target := args[0]
	if err := irr.ValidateASSetName(target); err != nil {
		return statusError, nil, errors.New("invalid AS-SET name")
	}
	plug.mu.RLock()
	var toRefresh []uint32
	for asn, st := range plug.byASN {
		if st.asSet == target {
			toRefresh = append(toRefresh, asn)
		}
	}
	plug.mu.RUnlock()

	if len(toRefresh) == 0 {
		return statusError, nil, fmt.Errorf("no IRR-filtered peer uses AS-SET %s", target)
	}
	for _, asn := range toRefresh {
		plug.refreshASN(asn)
	}
	for _, asn := range toRefresh {
		if msg := plug.asnRefreshError(asn); msg != "" {
			return statusError, nil, fmt.Errorf("AS-SET %s refresh failed for ASN %d: %s", target, asn, msg)
		}
	}
	var tb textbuf.Buffer
	tb.Str(`{"refreshed":"as-set `).Str(target).Str(`"}`)
	return statusDone, json.RawMessage(tb.String()), nil
}
