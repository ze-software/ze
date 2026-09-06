// Design: docs/architecture/firewall/firewall-domain-group.md -- CLI command handlers (show/update/clear firewall domain-group)

package domain

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	statusDone  = "done"
	statusError = "error"
)

func (plug *domainPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case cmdShowDomainGroup:
		return plug.showDomainGroup(args)
	case cmdUpdateDomainGroup:
		return plug.updateDomainGroup(args)
	case cmdClearDomainGroup:
		return plug.clearDomainGroup(args)
	}
	return statusError, nil, errors.New("unknown command")
}

// showDomainGroup reports every configured group, or one named group, with
// what each of its names currently resolves to.
//
// It answers the question an operator asks before a commit: does this group
// have data, and how old is it. A group with no addresses is reported as
// "missing" rather than as an empty list, because an empty permit set blocks
// everything and an empty deny set blocks nothing, and the operator needs to
// know which situation they are in.
func (plug *domainPlugin) showDomainGroup(args []string) (string, any, error) {
	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()

	wanted := ""
	if len(args) > 0 {
		wanted = args[0]
	}
	if wanted != "" {
		if _, ok := cfg.groupByName(wanted); !ok {
			return statusError, nil, unknownGroupError(cfg, wanted)
		}
	}

	now := time.Now()
	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"groups":[`)

	first := true
	if cfg != nil {
		for _, g := range cfg.groups {
			if wanted != "" && g.Name != wanted {
				continue
			}
			if !first {
				b.Byte(',')
			}
			first = false
			writeGroupJSON(b, plug, g, now)
		}
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func writeGroupJSON(b *textbuf.Buffer, plug *domainPlugin, g group, now time.Time) {
	v4Name, v6Name := setNames(g.Name)
	b.Str(`{"name":`).Quoted(g.Name)
	b.Str(`,"ttl-floor-seconds":`).Int(int64(g.TTLFloor))
	b.Str(`,"ipv4-set":`).Quoted(v4Name)
	b.Str(`,"ipv6-set":`).Quoted(v6Name)
	b.Str(`,"ipv4-count":`).Int(int64(len(plug.cache.groupAddresses(g.Name, g.Names, familyV4))))
	b.Str(`,"ipv6-count":`).Int(int64(len(plug.cache.groupAddresses(g.Name, g.Names, familyV6))))

	entries := plug.cache.entriesFor(g)
	b.Str(`,"names":[`)
	for i, name := range g.Names {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"name":`).Quoted(name)
		b.Str(`,"families":[`)
		for j, fam := range families {
			if j > 0 {
				b.Byte(',')
			}
			entry, ok := entries[nameKey{group: g.Name, name: name, family: fam}]
			b.Str(`{"family":`).Quoted(fam.label)
			if !ok {
				// No answer has ever been recorded for this name and family.
				// Distinct from an answer that legitimately carried none: the
				// first says nothing was asked, the second says the name holds
				// no address of this family.
				b.Str(`,"status":"missing","addresses":[],"count":0}`)
				continue
			}
			b.Str(`,"status":`).Quoted(entry.Status)
			b.Str(`,"count":`).Int(int64(len(entry.Addresses)))
			if entry.failing() {
				// The addresses below are the last good ones, and the name has
				// not answered since this time. An operator reading a count
				// alone would have no way to see that.
				b.Str(`,"failing-since":`).Quoted(entry.FailingSince.Format(time.RFC3339))
			}
			b.Str(`,"addresses":[`)
			for k, addr := range entry.Addresses {
				if k > 0 {
					b.Byte(',')
				}
				b.Quoted(addr)
			}
			b.Byte(']')
			if !entry.ResolvedAt.IsZero() {
				b.Str(`,"last-resolved":`).Quoted(entry.ResolvedAt.Format(time.RFC3339))
				b.Str(`,"data-age-seconds":`).Int(int64(now.Sub(entry.ResolvedAt).Seconds()))
			}
			b.Byte('}')
		}
		b.Str(`]}`)
	}
	b.Str(`]}`)
}

// updateDomainGroup resolves every name of a group at once.
//
// It is the only path back from a cold cache. Nothing is registered at startup
// for a group that has never resolved, so every rule naming it is held back by
// the registry until this command runs, and the message a refused commit
// carries names this command for that reason.
func (plug *domainPlugin) updateDomainGroup(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: update firewall domain-group <name>")
	}

	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()

	g, ok := cfg.groupByName(args[0])
	if !ok {
		return statusError, nil, unknownGroupError(cfg, args[0])
	}
	if len(g.Names) == 0 {
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: ").Str(g.Name).Str(" holds no domain-names, so there is nothing to resolve")
		return statusError, nil, errors.New(tb.String())
	}

	changed, err := plug.refreshGroupNow(g)
	if err != nil {
		// Both facts go to the operator. Some names may have resolved and been
		// programmed; the error names the one that did not, and reporting only
		// the failure would hide the addresses now in the kernel.
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: ").Str(g.Name).Str(" partially refreshed (")
		tb.Int(int64(changed)).Str(" changed): ").Str(err.Error())
		return statusError, nil, errors.New(tb.String())
	}

	var tb textbuf.Buffer
	tb.Str(`{"group":`).Quoted(g.Name)
	tb.Str(`,"changed":`).Int(int64(changed))
	tb.Str(`,"ipv4-count":`).Int(int64(len(plug.cache.groupAddresses(g.Name, g.Names, familyV4))))
	tb.Str(`,"ipv6-count":`).Int(int64(len(plug.cache.groupAddresses(g.Name, g.Names, familyV6))))
	tb.Byte('}')
	return statusDone, json.RawMessage(tb.String()), nil
}

// clearDomainGroup drops a group's cached addresses and re-applies the tables.
//
// It is the deliberate exit from last-known-good. A refresh that fails keeps
// the addresses it already had rather than emptying a live filter, so an
// operator who knows a group is finished says so here. Config that still names
// the group then fails to verify until it is resolved again, which is the
// intended consequence and not a side effect.
func (plug *domainPlugin) clearDomainGroup(args []string) (string, any, error) {
	if len(args) == 0 || args[0] == "" {
		return statusError, nil, errors.New("usage: clear firewall domain-group <name>")
	}

	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()

	g, ok := cfg.groupByName(args[0])
	if !ok {
		return statusError, nil, unknownGroupError(cfg, args[0])
	}

	removed, err := plug.cache.purge(g)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: ").Str(g.Name).Str(" cleared from memory, but the persisted cache could not be updated: ")
		tb.Str(err.Error()).Str("; the addresses will come back on the next restart")
		return statusError, nil, errors.New(tb.String())
	}
	if removed == 0 {
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: no cached addresses for ").Str(g.Name)
		return statusError, nil, errors.New(tb.String())
	}
	logger().Warn("firewall-domain: cached addresses purged by operator", "group", g.Name, "entries", removed)

	// The addresses are gone either way, so the apply outcome is reported
	// rather than returned bare: a config that still names the group now has a
	// rule with no set behind it, and the operator needs both facts.
	if applyErr := plug.applyTables(); applyErr != nil {
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: cleared the addresses for ").Str(g.Name)
		tb.Str(", but the firewall tables could not be re-applied: ").Str(applyErr.Error())
		tb.Str("; remove the rules naming ").Str(g.Name)
		tb.Str(" or resolve it again with 'update firewall domain-group ").Str(g.Name).Byte('\'')
		return statusError, nil, errors.New(tb.String())
	}

	var tb textbuf.Buffer
	tb.Str(`{"cleared":`).Quoted(g.Name).Str(`,"entries":`).Int(int64(removed)).Byte('}')
	return statusDone, json.RawMessage(tb.String()), nil
}

// unknownGroupError names the group that does not exist and lists the ones
// that do, so an operator who mistyped one sees the answer without a second
// command.
func unknownGroupError(cfg *domainConfig, wanted string) error {
	var tb textbuf.Buffer
	tb.Str("firewall domain-group: no group named ").Str(wanted)
	names := cfg.groupNames()
	if len(names) == 0 {
		tb.Str("; no domain groups are configured")
		return errors.New(tb.String())
	}
	slices.Sort(names)
	tb.Str("; configured: ").Str(textbuf.Join(names, ", "))
	return errors.New(tb.String())
}
