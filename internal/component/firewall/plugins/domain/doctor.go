// Design: docs/architecture/firewall/firewall-domain-group.md -- doctor check for domain-group data
// Related: domain.go -- verify, the commit-time guard this check mirrors at runtime
// Related: register.go -- registerDomainDoctor registers the check and its codes at init
// Related: domain.go -- runFirewallDomain declares the unsaved-answer runtime check and answers it
//
// A refresh that fails keeps the addresses it already has, so a firewall can
// enforce a name's addresses long after the server stopped confirming them
// without anything failing. `show firewall domain-group` reports it per group;
// this check reports it for the whole node, which is where an operator looks
// when nothing is obviously broken.

package domain

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// exampleDoctorJSON is the doctor invocation every check of this plugin cites
// as its first example.
const exampleDoctorJSON = "ze doctor --json"

const (
	// codeDomainGroupStaleData fires when a group is enforcing addresses no DNS
	// answer has confirmed for longer than staleAfter.
	codeDomainGroupStaleData = "doctor-firewall-domain-group-stale-data"
	// codeDomainGroupNoData fires when a rule names a group with no resolved
	// addresses at all, so the rule filters nothing.
	codeDomainGroupNoData = "doctor-firewall-domain-group-no-data"
	// codeDomainGroupUnreadableData fires when a stored answer of a referenced
	// group exists and cannot be read or decoded, so its name reads as
	// unresolved and the group enforces less than the box learned.
	codeDomainGroupUnreadableData = "doctor-firewall-domain-group-unreadable-data"
	// codeDomainGroupUnsavedAnswer fires when an answer this process received
	// could not be written to the store, so the name keeps enforcing what it
	// held before and a restart does not have the newer answer.
	codeDomainGroupUnsavedAnswer = "doctor-firewall-domain-group-unsaved-answer"
)

// unsavedAnswerCheckName names the runtime check in the plugin's Stage 1
// registration and in its doctor callback's dispatch.
const unsavedAnswerCheckName = "firewall-domain-group-unsaved-answer"

// unsavedAnswerCheckDecl declares the runtime check in the plugin's Stage 1
// registration. A failed write is known only to the process that tried it, and
// the plugin may run forked, so `show doctor` asks this process through the
// doctor callback rather than reading the store from its own
// (docs/architecture/doctor-and-health-checks.md, "The phase decides what a
// check may read").
var unsavedAnswerCheckDecl = sdk.DoctorCheckDecl{
	Name:  unsavedAnswerCheckName,
	Phase: sdk.DoctorPhaseRuntime,
	Codes: []string{codeDomainGroupUnsavedAnswer},
}

// staleAfter is how long a name may go on failing to resolve before the check
// reports the addresses it is still enforcing as stale.
//
// It is a day rather than a multiple of the TTL because a short outage is
// ordinary and recovering from one is what keeping the last good answer is
// for. An operator paged for a two-minute upstream blip stops reading the
// check, which costs more than the blip.
const staleAfter = 24 * time.Hour

// domainDiagnosticCodes is the explanation metadata for the codes this plugin
// owns, so `ze show diagnostic <code>` can describe them. They live with the plugin, so
// removing the plugin removes the codes (ai/rules/plugins.md).
var domainDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDomainGroupStaleData,
		Title:       "Firewall domain group is enforcing stale addresses",
		Description: "A firewall rule references a domain group holding a DNS name that has not resolved for more than a day. Ze keeps the addresses it learned before, because emptying the set on a failed query would enforce a DNS outage rather than a fact about the name. The filter is enforcing addresses the DNS has stopped confirming. Check that the configured name servers are reachable and that the name still exists, then run 'update firewall domain-group <name>'. When a name is gone upstream for good, run 'clear firewall domain-group <name>' to remove its addresses.",
		Examples:    []string{exampleDoctorJSON, cmdShowDomainGroup, "ze show diagnostic doctor-firewall-domain-group-stale-data"},
	},
	{
		Code:        codeDomainGroupNoData,
		Title:       "Firewall domain group has no addresses",
		Description: "A firewall rule references a domain group with no resolved addresses. The rule filters nothing, and the table carrying it is held back until the group resolves. Run 'update firewall domain-group <name>' to resolve its names.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-firewall-domain-group-no-data"},
	},
	{
		Code:        codeDomainGroupUnreadableData,
		Title:       "Firewall domain group has a stored answer Ze cannot read",
		Description: "A firewall rule references a domain group whose stored DNS answer for one name and family could not be read or decoded. Ze treats the name as unresolved: its addresses are missing from the set, and the name is asked for again at once. The next answer rewrites the stored value. Run 'update firewall domain-group <name>' to resolve the group now.",
		Examples:    []string{exampleDoctorJSON, cmdShowDomainGroup, "ze show diagnostic doctor-firewall-domain-group-unreadable-data"},
	},
	{
		Code:        codeDomainGroupUnsavedAnswer,
		Title:       "Firewall domain group answer was not saved",
		Description: "A DNS answer for a name of a domain group could not be written to the store. Ze keeps the store and the enforced addresses equal, so the name goes on enforcing the addresses it held before the answer, and a restart does not have the newer answer. The cause is the store write error in the message: a full or read-only config filesystem, or a store the daemon cannot write. Fix the cause, then run 'update firewall domain-group <name>' to ask again and save the answer. The next write that succeeds clears the finding.",
		Examples:    []string{exampleDoctorJSON, cmdShowDomainGroup, "ze show diagnostic doctor-firewall-domain-group-unsaved-answer"},
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

	dir := ctx.ConfigDir
	if dir == "" {
		dir = resolve.StoreDir("")
	}
	persistence, err := storage.OpenReadOnly(dir, dir)
	if err != nil {
		return nil // The storage check reports unavailable or unsafe stores.
	}
	defer func() { _ = persistence.Close() }()
	cache := newStore(persistence)
	if err := cache.open(cfg.groups); err != nil {
		return nil // the cache file is the store's own problem to report
	}

	return domainGroupDiagnostics(cfg, cache, time.Now())
}

// domainGroupDiagnostics is the check's judgement, separated from reading the
// config tree and opening the store. The two halves fail for unrelated reasons
// and only this one carries the rules an operator is told about, so it is the
// half a test drives.
func domainGroupDiagnostics(cfg *domainConfig, cache *store, now time.Time) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	for _, name := range cfg.referencedGroups() {
		diags = append(diags, unreadableDiagnostics(cache, name)...)
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

// unreadableDiagnostics names each stored answer of the group that the last
// open could not read or decode (store.open).
func unreadableDiagnostics(cache *store, groupName string) []diagnostic.Diagnostic {
	unreadable := cache.unreadableFor(groupName)
	diags := make([]diagnostic.Diagnostic, 0, len(unreadable))
	for _, key := range unreadable {
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: ").Str(groupName).Str(" has a stored answer for ").Str(key.name)
		tb.Str(" ").Str(key.family.label).Str(" that cannot be read, so its addresses are not enforced")
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeDomainGroupUnreadableData,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Help:     updateHelp(groupName),
			Fix:      updateFix(groupName),
		})
	}
	return diags
}

// unsavedAnswerFindings answers the runtime check: one warning for each unit
// whose last answer could not be written (store.put), naming the cause and the
// command that asks again. Safe for concurrent use.
func unsavedAnswerFindings(cache *store) []sdk.DoctorCheckDiagnostic {
	unsaved := cache.unsavedFor()
	out := make([]sdk.DoctorCheckDiagnostic, 0, len(unsaved))
	for _, unit := range unsaved {
		var tb textbuf.Buffer
		tb.Str("firewall domain-group: ").Str(unit.key.group).Str(" could not save the answer for ")
		tb.Str(unit.key.name).Str(" ").Str(unit.key.family.label).Str(" at ")
		tb.Str(unit.failure.at.UTC().Format(time.RFC3339)).Str(" (").Err(unit.failure.err)
		tb.Str("), so it enforces the addresses it held before and a restart does not have the newer answer")
		out = append(out, sdk.DoctorCheckDiagnostic{
			Code:     codeDomainGroupUnsavedAnswer,
			Severity: string(diagnostic.SeverityWarning),
			Message:  tb.String(),
			Fix:      updateFix(unit.key.group),
		})
	}
	return out
}

// answerDoctorCheck is the plugin's doctor callback. It answers the one
// runtime check the plugin declares, and refuses a name it did not declare.
func (plug *domainPlugin) answerDoctorCheck(name string) ([]sdk.DoctorCheckDiagnostic, error) {
	if name != unsavedAnswerCheckName {
		return nil, fmt.Errorf("firewall-domain: unknown doctor check %s", name)
	}
	return unsavedAnswerFindings(plug.cache), nil
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

// updateFix is the command alone that resolves this group, for a finding's Fix
// and a WARN's fix attribute.
func updateFix(groupName string) string {
	var tb textbuf.Buffer
	tb.Str("update firewall domain-group ").Str(groupName)
	return tb.String()
}
