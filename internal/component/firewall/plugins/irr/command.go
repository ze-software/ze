// Design: docs/architecture/firewall/firewall-irr.md -- CLI command handlers (show firewall irr)
//
// The two show verbs answer what the firewall enforces: the copy of each
// reference the reader read from the irr plugin's store. Fetching and removing
// a stored list are the irr plugin's `update irr` and `clear irr`.

package irr

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	statusDone  = "done"
	statusError = "error"
)

func (plug *irrPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case cmdShowIRR:
		return plug.showIRR()
	case cmdShowIRRPrefix:
		return plug.showIRRPrefix(args)
	}
	return statusError, nil, errors.New("unknown command")
}

// showIRR answers one entry per configured reference, in name order: what is
// enforced for it, and where that copy came from.
func (plug *irrPlugin) showIRR() (string, any, error) {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	names := make([]string, 0, len(plug.refs))
	for name := range plug.refs {
		names = append(names, name)
	}
	slices.Sort(names)

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"entries":[`)
	now := time.Now()
	for i, name := range names {
		if i > 0 {
			b.Byte(',')
		}
		writeShowEntry(b, plug.refs[name], now)
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

// writeShowEntry writes one reference's entry. "status" is the operator's
// answer to "what does this rule enforce": ok, stale (the last refresh learned
// nothing and the stored prefixes stay in force), oversized (one family holds
// more prefixes than a firewall set takes, so the rules naming it are not in
// the kernel), unreadable (a stored value exists and cannot be used; the copy
// read before stays enforced) or missing (nothing is stored, so the rules
// naming it are held back).
func writeShowEntry(b *textbuf.Buffer, st *refState, now time.Time) {
	b.Str(`{"name":`).Quoted(st.ref.Name)
	if st.ref.IsASSet {
		b.Str(`,"type":"as-set"`)
	} else {
		b.Str(`,"type":"asn"`)
	}
	copied := st.copy
	switch {
	case st.keyErr != "":
		b.Str(`,"status":"unreadable","error":`).Quoted(st.keyErr)
	case st.readErr != "":
		b.Str(`,"status":"unreadable","error":`).Quoted(st.readErr)
	case copied == nil:
		b.Str(`,"status":"missing"`)
	case oversizedEntryError(st.ref.Name, copied.entry) != nil:
		b.Str(`,"status":"oversized"`)
	case copied.entry.Stale():
		b.Str(`,"status":"stale"`)
	default:
		b.Str(`,"status":"ok"`)
	}
	if copied == nil {
		b.Str(`,"ipv4-count":0,"ipv6-count":0}`)
		return
	}
	entry := copied.entry
	b.Str(`,"as-set":`).Quoted(copied.asSet)
	b.Str(`,"server":`).Quoted(copied.pointer.Server)
	b.Str(`,"generation":`).Uint32(copied.pointer.Generation)
	b.Str(`,"ipv4-count":`).Int(int64(len(entry.IPv4)))
	b.Str(`,"ipv6-count":`).Int(int64(len(entry.IPv6)))
	if !copied.checkedAt.IsZero() {
		b.Str(`,"checked-at":"`).Str(copied.checkedAt.UTC().Format(time.RFC3339)).Byte('"')
	}
	if !entry.RefreshedAt.IsZero() {
		b.Str(`,"refreshed-at":"`).Str(entry.RefreshedAt.UTC().Format(time.RFC3339)).Byte('"')
		b.Str(`,"data-age-seconds":`).Int(int64(now.Sub(entry.RefreshedAt).Seconds()))
	}
	if entry.Stale() {
		b.Str(`,"stale-since":"`).Str(entry.StaleSince.UTC().Format(time.RFC3339)).Byte('"')
	}
	b.Byte('}')
}

// showIRRPrefix answers the prefixes enforced for one configured reference.
// The name is the one the config spells; an AS-SET matches in any case.
func (plug *irrPlugin) showIRRPrefix(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, opererr.New("show firewall irr prefix needs a reference name",
			"it shows the prefixes enforced for one ASN or AS-SET", "show firewall irr prefix <asn-or-as-set>")
	}
	name := args[0]
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	st := plug.refs[name]
	if st == nil {
		for refName, candidate := range plug.refs {
			if strings.EqualFold(refName, name) {
				st = candidate
				break
			}
		}
	}
	if st == nil || st.copy == nil {
		return statusError, nil, opererr.New("firewall irr: no list is enforced for "+name,
			"no firewall rule names it, or nothing is stored for it", "show firewall irr")
	}
	entry := st.copy.entry

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"name":`).Quoted(st.ref.Name)
	b.Str(`,"as-set":`).Quoted(st.copy.asSet)
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
