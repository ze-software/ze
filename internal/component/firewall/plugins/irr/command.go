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
	case "clear firewall irr asn":
		return plug.clearASN(args)
	case "clear firewall irr as-set":
		return plug.clearASSet(args)
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
	now := time.Now()
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
				// "stale" is the operator's answer to "is what I am enforcing
				// what the IRR last said?". An entry goes stale when a refresh
				// learns nothing and the previous prefixes stay in force.
				if entry.Stale() {
					b.Str(`,"status":"stale"`)
				} else {
					b.Str(`,"status":"ok"`)
				}
				b.Str(`,"ipv4-count":`).Int(int64(len(entry.IPv4)))
				b.Str(`,"ipv6-count":`).Int(int64(len(entry.IPv6)))
				if !entry.RefreshedAt.IsZero() {
					b.Str(`,"last-refresh":"`).Str(entry.RefreshedAt.Format(time.RFC3339)).Byte('"')
					b.Str(`,"data-age-seconds":`).Int(int64(now.Sub(entry.RefreshedAt).Seconds()))
				}
				if entry.Stale() {
					b.Str(`,"stale-since":"`).Str(entry.StaleSince.Format(time.RFC3339)).Byte('"')
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

// clearASN removes an ASN's cached prefixes. See purge for why the command
// exists.
func (plug *irrPlugin) clearASN(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: clear firewall irr asn <asn>")
	}
	n, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil || n == 0 || n > 4294967294 {
		return statusError, nil, errors.New("invalid ASN: must be 1-4294967294")
	}
	var tb textbuf.Buffer
	return plug.purge(tb.Str("AS").Uint32(uint32(n)).String()) //nolint:gosec // range checked
}

// clearASSet removes an AS-SET's cached prefixes. See purge.
func (plug *irrPlugin) clearASSet(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: clear firewall irr as-set <as-set>")
	}
	if err := irr.ValidateASSetName(args[0]); err != nil {
		return statusError, nil, errors.New("invalid AS-SET name")
	}
	return plug.purge(args[0])
}

// purge drops name's cached prefixes from memory and from the persisted cache,
// then re-applies the firewall tables so the change takes effect at once.
//
// It is the deliberate exit from last-known-good. A refresh that learns nothing
// keeps the previous prefixes rather than emptying a live filter, so an AS-SET
// that was deregistered upstream stops being enforced when an operator says so.
func (plug *irrPlugin) purge(name string) (string, any, error) {
	ps := plug.getPrefixStore()
	if ps == nil {
		return statusError, nil, errors.New("no prefix store available")
	}
	if !ps.Purge(name) {
		return statusError, nil, fmt.Errorf("no cached data for %v", name)
	}
	logger().Warn("firewall-irr: cached prefixes purged by operator", "name", name)

	// The entry is gone either way, so the apply outcome is reported rather
	// than returned bare: a config that still references the name now has a
	// term with no set behind it, and the operator needs both facts.
	if err := plug.applyTables(); err != nil {
		logger().Warn("firewall-irr: apply after purge failed", "name", name, "error", err)
		var tb textbuf.Buffer
		tb.Str("firewall irr: cleared the cached prefixes for ").Str(name)
		tb.Str(", but the firewall tables could not be re-applied: ").Str(err.Error())
		tb.Str("; remove the config references to ").Str(name)
		tb.Str(" or fetch it again with 'update firewall irr'")
		return statusError, nil, errors.New(tb.String())
	}

	var tb textbuf.Buffer
	tb.Str(`{"cleared":`).Quoted(name).Byte('}')
	return statusDone, json.RawMessage(tb.String()), nil
}
