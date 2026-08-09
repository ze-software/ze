// Design: docs/architecture/firewall/firewall-irr.md -- CLI command handlers (show/update firewall irr)

package irr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	statusDone  = "done"
	statusError = "error"
)

func (plug *irrPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case "show firewall irr":
		return plug.showIRR()
	case "show firewall irr prefix":
		return plug.showIRRPrefix(args)
	case "update firewall irr all":
		if err := plug.refreshAllNow(); err != nil {
			return statusError, nil, err
		}
		return statusDone, json.RawMessage(`{"refreshed":"all"}`), nil
	case "update firewall irr asn":
		return plug.updateASN(args)
	case "update firewall irr as-set":
		return plug.updateASSet(args)
	}
	return statusError, nil, errors.New("unknown command")
}

func (plug *irrPlugin) showIRR() (string, any, error) {
	ps := plug.getPrefixStore()

	plug.mu.RLock()
	cfg := plug.config
	lastRefresh := plug.lastRefresh
	plug.mu.RUnlock()

	server := defaultServer
	if cfg != nil {
		server = cfg.Server
	}

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"server":`).Quoted(server)
	if !lastRefresh.IsZero() {
		b.Str(`,"last-refresh":"`).Str(lastRefresh.Format(time.RFC3339)).Byte('"')
	}

	b.Str(`,"entries":[`)
	first := true
	if cfg != nil && ps != nil {
		for _, ref := range cfg.allRefs() {
			entry := ps.Get(ref.Name)
			if !first {
				b.Byte(',')
			}
			first = false
			b.Str(`{"name":`).Quoted(ref.Name)
			if ref.IsASSet {
				b.Str(`,"type":"as-set"`)
			} else {
				b.Str(`,"type":"asn"`)
			}
			if entry != nil {
				b.Str(`,"status":"ok"`)
				b.Str(`,"ipv4-count":`).Int(int64(len(entry.IPv4)))
				b.Str(`,"ipv6-count":`).Int(int64(len(entry.IPv6)))
				if !entry.RefreshedAt.IsZero() {
					b.Str(`,"last-refresh":"`).Str(entry.RefreshedAt.Format(time.RFC3339)).Byte('"')
				}
			} else {
				b.Str(`,"status":"missing"`)
				b.Str(`,"ipv4-count":0,"ipv6-count":0`)
			}
			b.Byte('}')
		}
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) showIRRPrefix(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: show firewall irr prefix <asn-or-as-set>")
	}
	name := args[0]
	ps := plug.getPrefixStore()
	if ps == nil {
		return statusError, nil, errors.New("no prefix store available")
	}
	entry := ps.Get(name)
	if entry == nil {
		return statusError, nil, fmt.Errorf("no cached data for %v", name)
	}

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"name":`).Quoted(name)
	b.Str(`,"ipv4":[`)
	for i, p := range entry.IPv4 {
		if i > 0 {
			b.Byte(',')
		}
		b.Byte('"').Prefix(p).Byte('"')
	}
	b.Str(`],"ipv6":[`)
	for i, p := range entry.IPv6 {
		if i > 0 {
			b.Byte(',')
		}
		b.Byte('"').Prefix(p).Byte('"')
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func (plug *irrPlugin) updateASN(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update firewall irr asn <asn>")
	}
	n, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil || n == 0 || n > 4294967294 {
		return statusError, nil, errors.New("invalid ASN: must be 1-4294967294")
	}
	var tb textbuf.Buffer
	name := tb.Str("AS").Uint32(uint32(n)).String() //nolint:gosec // range checked

	if err := plug.refreshName(name); err != nil {
		return statusError, nil, err
	}

	ps := plug.getPrefixStore()
	entry := ps.Get(name)
	tb.Reset()
	if entry != nil {
		tb.Str(`{"refreshed":"`).Str(name).Str(`","ipv4-count":`).Int(int64(len(entry.IPv4)))
		tb.Str(`,"ipv6-count":`).Int(int64(len(entry.IPv6))).Byte('}')
	} else {
		tb.Str(`{"refreshed":"`).Str(name).Str(`"}`)
	}
	return statusDone, json.RawMessage(tb.String()), nil
}

func (plug *irrPlugin) updateASSet(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update firewall irr as-set <as-set>")
	}
	name := args[0]
	if err := irr.ValidateASSetName(name); err != nil {
		return statusError, nil, errors.New("invalid AS-SET name")
	}

	if err := plug.refreshName(name); err != nil {
		return statusError, nil, err
	}

	ps := plug.getPrefixStore()
	entry := ps.Get(name)
	var tb textbuf.Buffer
	if entry != nil {
		tb.Str(`{"refreshed":"`).Str(name).Str(`","ipv4-count":`).Int(int64(len(entry.IPv4)))
		tb.Str(`,"ipv6-count":`).Int(int64(len(entry.IPv6))).Byte('}')
	} else {
		tb.Str(`{"refreshed":"`).Str(name).Str(`"}`)
	}
	return statusDone, json.RawMessage(tb.String()), nil
}
