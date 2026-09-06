// Design: docs/architecture/firewall/firewall-domain-group.md -- doctor check for domain-group data
// Related: domain.go -- verify, the commit-time guard this check mirrors at runtime
// Related: register.go -- registerDomainDoctor registers the check and its codes at init
//
// A refresh that fails keeps the addresses it already has, so a firewall can
// enforce a name's addresses long after the server stopped confirming them
// without anything failing. `show firewall domain-group` reports it per group;
// this check reports it for the whole node, which is where an operator looks
// when nothing is obviously broken.

package domain

import (
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// codeDomainGroupStaleData fires when a group is enforcing addresses no DNS
	// answer has confirmed for longer than staleAfter.
	codeDomainGroupStaleData = "doctor-firewall-domain-group-stale-data"
	// codeDomainGroupNoData fires when a rule names a group with no resolved
	// addresses at all, so the rule filters nothing.
	codeDomainGroupNoData = "doctor-firewall-domain-group-no-data"
)

// staleAfter is how long a name may go on failing to resolve before the check
// reports the addresses it is still enforcing as stale.
//
// It is a day rather than a multiple of the TTL because a short outage is
// ordinary and recovering from one is what keeping the last good answer is
// for. An operator paged for a two-minute upstream blip stops reading the
// check, which costs more than the blip.
const staleAfter = 24 * time.Hour

// domainDiagnosticCodes is the explanation metadata for the codes this plugin
// owns, so `ze explain <code>` can describe them. They live with the plugin, so
// removing the plugin removes the codes (ai/rules/plugins.md).
var domainDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDomainGroupStaleData,
		Title:       "Firewall domain group is enforcing stale addresses",
		Description: "A firewall rule references a domain group holding a DNS name that has not resolved for more than a day. Ze keeps the addresses it learned before, because emptying the set on a failed query would enforce a DNS outage rather than a fact about the name. The filter is enforcing addresses the DNS has stopped confirming. Check that the configured name servers are reachable and that the name still exists, then run 'update firewall domain-group <name>'. When a name is gone upstream for good, run 'clear firewall domain-group <name>' to remove its addresses.",
		Examples:    []string{"ze doctor --json", cmdShowDomainGroup, "ze explain doctor-firewall-domain-group-stale-data"},
	},
	{
		Code:        codeDomainGroupNoData,
		Title:       "Firewall domain group has no addresses",
		Description: "A firewall rule references a domain group with no resolved addresses. The rule filters nothing, and the table carrying it is held back until the group resolves. Run 'update firewall domain-group <name>' to resolve its names.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-firewall-domain-group-no-data"},
	},
}

// checkDomainGroupData reports every referenced domain group that has no
// addresses or is enforcing stale ones. It is a no-op on a node with no domain
// groups, so a node that does not use them sees nothing.
func checkDomainGroupData(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	fw := tree.GetContainer(configRoot)
	if fw == nil {
		return nil
	}
	fwMap := fw.ToMap()
	root := map[string]any{configRoot: fwMap}

	cfg := &domainConfig{groups: parseGroups(fwMap), refs: extractRefsFromConfig(root)}
	if len(cfg.referencedGroups()) == 0 {
		return nil
	}

	cache := newStore(doctorCachePath(ctx.ConfigDir))
	if err := cache.open(cfg.groups); err != nil {
		return nil // the cache file is the store's own problem to report
	}
	defer cache.close()

	return domainGroupDiagnostics(cfg, cache, time.Now())
}

// domainGroupDiagnostics is the check's judgement, separated from reading the
// config tree and opening the store. The two halves fail for unrelated reasons
// and only this one carries the rules an operator is told about, so it is the
// half a test drives.
func domainGroupDiagnostics(cfg *domainConfig, cache *store, now time.Time) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	for _, name := range cfg.referencedGroups() {
		g, ok := cfg.groupByName(name)
		if !ok || !cache.hasAddresses(g) {
			var tb textbuf.Buffer
			tb.Str("firewall domain-group: ").Str(name).Str(" has no resolved addresses and filters nothing")
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeDomainGroupNoData,
				Severity: diagnostic.SeverityError,
				Message:  tb.String(),
				Help:     updateHelp(name),
			})
			continue
		}
		since, failingName, found := longestFailure(cache, g)
		if !found || now.Sub(since) <= staleAfter {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: ").Str(g.Name).Str(" is enforcing addresses for ").Str(failingName)
		tb.Str(", which has not resolved since ").Str(since.Format(time.RFC3339))
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeDomainGroupStaleData,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Actual:   int(now.Sub(since).Seconds()),
			Help:     updateHelp(g.Name),
		})
	}
	return diags
}

// longestFailure returns when the group's longest-running failure started, and
// the name it belongs to.
//
// It reads FailingSince rather than ResolvedAt on purpose. ResolvedAt says when
// the addresses last CHANGED, and a name that resolves correctly every five
// minutes to the same addresses has an ancient ResolvedAt: measuring staleness
// from it would report every stable group as broken. FailingSince is written
// only when a name stops answering, so it says the thing an operator needs.
//
// An entry holding no addresses is skipped. There is nothing being enforced for
// it, and the group is already reported by the no-data check.
func longestFailure(cache *store, g group) (since time.Time, name string, found bool) {
	for key, entry := range cache.entriesFor(g) {
		if len(entry.Addresses) == 0 || !entry.failing() {
			continue
		}
		if !found || entry.FailingSince.Before(since) {
			since, name, found = entry.FailingSince, key.name, true
		}
	}
	return since, name, found
}

// updateHelp names the command that resolves this group.
func updateHelp(groupName string) string {
	var tb textbuf.Buffer
	tb.Str("run 'update firewall domain-group ").Str(groupName).Byte('\'')
	return tb.String()
}

// doctorCachePath is the zefs file holding the resolved addresses. Doctor may
// be pointed at a config directory other than the default, so its own
// directory wins when it has one.
func doctorCachePath(configDir string) string {
	if configDir == "" {
		return cacheStorePath()
	}
	return filepath.Join(configDir, "database.zefs")
}
