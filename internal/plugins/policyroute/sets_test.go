// Design: docs/architecture/policyroute/policy-routing.md -- a @set reference resolves to a firewall table's set
// Overview: sets.go -- parsePolicySections, resolveSetReferences

package policyroute

import (
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/iprule"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// policyWithAddress is a policy/route section whose one rule matches the
// destination address value, which is a prefix or a @set reference.
func policyWithAddress(value string) sdk.ConfigSection {
	return sdk.ConfigSection{
		Root: configRoot,
		Data: `{"policy":{"route":{"steer":{"interface":"eth0","rule":{"bypass":{` +
			`"from":{"destination-address":"` + value + `"},"then":{"accept":{}}}}}}}}`,
	}
}

// firewallWithSets is a firewall section holding one inet table per entry of
// tables, each declaring the set body under the name given.
func firewallWithSets(tables map[string]string) sdk.ConfigSection {
	var body strings.Builder
	first := true
	for table, sets := range tables {
		if !first {
			body.WriteString(",")
		}
		first = false
		body.WriteString(`"` + table + `":{"family":"inet","set":{` + sets + `}}`)
	}
	return sdk.ConfigSection{
		Root: firewall.ConfigRoot,
		Data: `{"firewall":{"table":{` + body.String() + `}}}`,
	}
}

// translateSections runs one delivery through the same path the plugin's
// verify and apply take: parse and resolve, then translate.
func translateSections(t *testing.T, sections ...sdk.ConfigSection) firewall.Table {
	t.Helper()
	policies, err := parsePolicySections(sections)
	if err != nil {
		t.Fatalf("parsePolicySections: %v", err)
	}
	result, err := newAllocator().translate(policies)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(result.Tables) != 1 {
		t.Fatalf("translate produced %d tables, want 1", len(result.Tables))
	}
	return result.Tables[0]
}

// TestPolicySetReferenceCopiesTheFirewallSet proves a @set reference programs
// a set the ze_pr table itself carries. nftables resolves a set only inside the
// rule's own table, so before this fix the table carried the match and no set
// and the apply failed with "unknown set".
//
// The method: a firewall table declares the set, the policy names it, and the
// translated ze_pr table must hold the set with the firewall's type and
// elements, beside a term matching it by name. The control is the same policy
// with a prefix, which must carry no set at all.
func TestPolicySetReferenceCopiesTheFirewallSet(t *testing.T) {
	fw := firewallWithSets(map[string]string{
		"edge": `"bypass":{"type":"ipv4","element":{"192.0.2.10":{},"192.0.2.20":{}}}`,
	})
	table := translateSections(t, policyWithAddress("@bypass"), fw)

	if len(table.Sets) != 1 {
		t.Fatalf("ze_pr carries %d sets, want 1: %+v", len(table.Sets), table.Sets)
	}
	set := table.Sets[0]
	if set.Name != "bypass" {
		t.Fatalf("ze_pr set is named %q, want bypass", set.Name)
	}
	if set.Type != firewall.SetTypeIPv4 {
		t.Fatalf("ze_pr set has type %s, want ipv4", set.Type)
	}
	if got := elementValues(set); got != "192.0.2.10,192.0.2.20" {
		t.Fatalf("ze_pr set holds %s, want the firewall set's 192.0.2.10,192.0.2.20", got)
	}

	terms := table.Chains[0].Terms
	if len(terms) != 1 {
		t.Fatalf("ze_pr chain has %d terms, want 1", len(terms))
	}
	var found bool
	for _, m := range terms[0].Matches {
		in, ok := m.(firewall.MatchInSet)
		if !ok {
			continue
		}
		if in.SetName != "bypass" || in.MatchField != firewall.SetFieldDestAddr {
			t.Fatalf("set match is %+v, want bypass on the destination address", in)
		}
		found = true
	}
	if !found {
		t.Fatalf("the term carries no set match: %+v", terms[0].Matches)
	}

	// The control: a prefix makes no set, so a set above came from the reference.
	plain := translateSections(t, policyWithAddress("192.0.2.0/24"), fw)
	if len(plain.Sets) != 0 {
		t.Fatalf("a prefix match carried sets %+v, want none", plain.Sets)
	}
}

// TestPolicySetReferenceFollowsTheFirewallSet proves the copy is taken from the
// firewall section of each delivery, so a commit that changes the set's
// elements changes the ze_pr copy with it. Two deliveries that differ only in
// the firewall section must give two copies that differ the same way.
func TestPolicySetReferenceFollowsTheFirewallSet(t *testing.T) {
	before := translateSections(t, policyWithAddress("@bypass"), firewallWithSets(map[string]string{
		"edge": `"bypass":{"type":"ipv4","element":{"192.0.2.10":{}}}`,
	}))
	after := translateSections(t, policyWithAddress("@bypass"), firewallWithSets(map[string]string{
		"edge": `"bypass":{"type":"ipv4","element":{"198.51.100.30":{}}}`,
	}))

	if got := elementValues(before.Sets[0]); got != "192.0.2.10" {
		t.Fatalf("first copy holds %s, want 192.0.2.10", got)
	}
	if got := elementValues(after.Sets[0]); got != "198.51.100.30" {
		t.Fatalf("second copy holds %s, want 198.51.100.30: the copy did not follow the firewall set", got)
	}
}

// TestPolicySetReferenceUndeclaredRefusedAtVerify proves a reference to a set
// no firewall table declares is refused by the verifier, which `ze config
// validate` and the commit both run, naming the set. Before the fix it passed
// verify and failed at apply.
func TestPolicySetReferenceUndeclaredRefusedAtVerify(t *testing.T) {
	cases := map[string][]sdk.ConfigSection{
		"no firewall section": {policyWithAddress("@bypass")},
		"another set declared": {policyWithAddress("@bypass"), firewallWithSets(map[string]string{
			"edge": `"other":{"type":"ipv4","element":{"192.0.2.10":{}}}`,
		})},
	}
	for name, sections := range cases {
		t.Run(name, func(t *testing.T) {
			err := verifyPolicyConfig(sections)
			if err == nil {
				t.Fatal("verify accepted a reference to an undeclared set")
			}
			if !strings.Contains(err.Error(), `no firewall table declares a set named "bypass"`) {
				t.Fatalf("verify refused with %q, want it to name the undeclared set", err)
			}
		})
	}
}

// TestPolicySetReferenceAmbiguousRefused proves a name two firewall tables
// declare is refused rather than resolved to whichever table came first, and
// that the refusal lists both tables.
func TestPolicySetReferenceAmbiguousRefused(t *testing.T) {
	err := verifyPolicyConfig([]sdk.ConfigSection{
		policyWithAddress("@bypass"),
		firewallWithSets(map[string]string{
			"edge": `"bypass":{"type":"ipv4","element":{"192.0.2.10":{}}}`,
			"core": `"bypass":{"type":"ipv4","element":{"198.51.100.30":{}}}`,
		}),
	})
	if err == nil {
		t.Fatal("verify accepted a set name two firewall tables declare")
	}
	if !strings.Contains(err.Error(), "firewall tables core, edge each declare a set named \"bypass\"") {
		t.Fatalf("verify refused with %q, want it to name both tables", err)
	}
}

// TestPolicySetReferenceWrongTypeRefused proves an address match against a set
// that holds no addresses is refused at verify, where the firewall's own
// validator would refuse it for a firewall term.
func TestPolicySetReferenceWrongTypeRefused(t *testing.T) {
	err := verifyPolicyConfig([]sdk.ConfigSection{
		policyWithAddress("@bypass"),
		firewallWithSets(map[string]string{
			"edge": `"bypass":{"type":"inet-service","element":{"443":{}}}`,
		}),
	})
	if err == nil {
		t.Fatal("verify accepted an address match against an inet-service set")
	}
	if !strings.Contains(err.Error(), "an address match reads only an ipv4 or ipv6 set") {
		t.Fatalf("verify refused with %q, want the set type named", err)
	}
}

// TestPolicyWithoutSetReferenceIgnoresTheFirewallSection proves a policy that
// names no set does not parse the firewall section. The firewall engine refuses
// a bad firewall section at the same verify with its own message; policy
// routing refusing it too would report one mistake twice. The control is the
// same bad section beside a policy that does name a set, which must refuse.
func TestPolicyWithoutSetReferenceIgnoresTheFirewallSection(t *testing.T) {
	bad := sdk.ConfigSection{Root: firewall.ConfigRoot, Data: `{"firewall":{"table":{"edge":{"family":"nonsense"}}}}`}
	if err := verifyPolicyConfig([]sdk.ConfigSection{policyWithAddress("192.0.2.0/24"), bad}); err != nil {
		t.Fatalf("a policy naming no set was refused over the firewall section: %v", err)
	}
	if err := verifyPolicyConfig([]sdk.ConfigSection{policyWithAddress("@bypass"), bad}); err == nil {
		t.Fatal("a policy naming a set accepted a firewall section that does not parse")
	}
}

// TestFirewallOnlyDeliveryHasNoPolicies proves the reading parsePolicySections
// gives a delivery that carries the firewall root alone: the read root is
// delivered whole beside policy/route whenever either changes, so a delivery
// without policy/route means the configuration holds no policy route. The
// verify stages that empty answer, and the apply behind it has something to
// install; staging nothing would fail the apply closed (errApplyWithoutVerify).
func TestFirewallOnlyDeliveryHasNoPolicies(t *testing.T) {
	fw := firewallWithSets(map[string]string{"edge": `"bypass":{"type":"ipv4"}`})
	policies, err := parsePolicySections([]sdk.ConfigSection{fw})
	if err != nil {
		t.Fatalf("parsePolicySections: %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("a firewall-only delivery produced policies %+v, want none", policies)
	}
	if hasPolicySection([]sdk.ConfigSection{fw}) {
		t.Fatal("hasPolicySection reported a policy section in a firewall-only delivery")
	}
	if !hasPolicySection([]sdk.ConfigSection{fw, policyWithAddress("192.0.2.0/24")}) {
		t.Fatal("hasPolicySection missed the policy section")
	}
}

// TestTranslateRefusesAnUnresolvedSetReference proves the guard in
// parseAddressMatch: a @name whose set was never resolved is refused, rather
// than emitted as a match on a set ze_pr would not carry.
func TestTranslateRefusesAnUnresolvedSetReference(t *testing.T) {
	policy := PolicyRoute{
		Name:  "steer",
		Rules: []PolicyRule{{Name: "bypass", Match: PolicyMatch{DestinationAddress: "@bypass"}, Action: PolicyAction{Type: ActionAccept}}},
	}
	_, err := newAllocator().translate([]PolicyRoute{policy})
	if err == nil {
		t.Fatal("translate accepted a set reference no resolution attached a set to")
	}
	if !strings.Contains(err.Error(), `set "bypass" was never resolved`) {
		t.Fatalf("translate refused with %q, want the unresolved set named", err)
	}
}

// policyWithLeaves is a policy/route section whose one rule carries the from
// block given, so a test can name a port leaf or a tcp-flags value.
func policyWithLeaves(from string) sdk.ConfigSection {
	return sdk.ConfigSection{
		Root: configRoot,
		Data: `{"policy":{"route":{"steer":{"interface":"eth0","rule":{"bypass":{` +
			`"from":{` + from + `},"then":{"accept":{}}}}}}}}`,
	}
}

// TestPolicySetReferenceCopiesFlagsAndIPv6 proves the copy keeps what the
// firewall set declares beyond its elements: an interval set keeps its flag,
// and an ipv6 set keeps its type. A copy that dropped the interval flag would
// make the kernel refuse a prefix element, and one that took ipv4 for every
// set would read the wrong header bytes.
func TestPolicySetReferenceCopiesFlagsAndIPv6(t *testing.T) {
	fw := firewallWithSets(map[string]string{
		"edge": `"ranges":{"type":"ipv4","flags-interval":{},"element":{"192.0.2.0/24":{}}},` +
			`"hosts6":{"type":"ipv6","element":{"2001:db8::10":{}}}`,
	})
	table := translateSections(t,
		policyWithLeaves(`"source-address":"@ranges","destination-address":"@hosts6"`), fw)
	if len(table.Sets) != 2 {
		t.Fatalf("ze_pr carries %d sets, want 2: %+v", len(table.Sets), table.Sets)
	}
	ranges, hosts6 := table.Sets[0], table.Sets[1]
	if ranges.Name != "ranges" || ranges.Flags != firewall.SetFlagInterval {
		t.Fatalf("set %q has flags %v, want ranges with the interval flag", ranges.Name, ranges.Flags)
	}
	if got := elementValues(ranges); got != "192.0.2.0/24" {
		t.Fatalf("set ranges holds %s, want 192.0.2.0/24", got)
	}
	if hosts6.Name != "hosts6" || hosts6.Type != firewall.SetTypeIPv6 || hosts6.Flags != 0 {
		t.Fatalf("set %q is type %s flags %v, want hosts6, ipv6, no flags", hosts6.Name, hosts6.Type, hosts6.Flags)
	}
	if got := elementValues(hosts6); got != "2001:db8::10" {
		t.Fatalf("set hosts6 holds %s, want 2001:db8::10", got)
	}
}

// TestPolicyPortSetReferenceCopiesTheFirewallSet proves a port leaf's @name
// resolves like an address leaf's: against an inet-service set, copied into
// ze_pr, matched on the port field. Before, port-spec accepted @name and
// translate refused it at apply. The negative: a port leaf naming an ipv4 set
// is refused at verify, naming the type.
func TestPolicyPortSetReferenceCopiesTheFirewallSet(t *testing.T) {
	fw := firewallWithSets(map[string]string{
		"edge": `"web":{"type":"inet-service","element":{"80":{},"443":{}}},` +
			`"hosts":{"type":"ipv4","element":{"192.0.2.10":{}}}`,
	})
	table := translateSections(t, policyWithLeaves(`"protocol":"tcp","destination-port":"@web"`), fw)
	if len(table.Sets) != 1 || table.Sets[0].Name != "web" || table.Sets[0].Type != firewall.SetTypeInetService {
		t.Fatalf("ze_pr sets are %+v, want the inet-service set web", table.Sets)
	}
	var found bool
	for _, m := range table.Chains[0].Terms[0].Matches {
		if in, ok := m.(firewall.MatchInSet); ok {
			if in.SetName != "web" || in.MatchField != firewall.SetFieldDestPort {
				t.Fatalf("set match is %+v, want web on the destination port", in)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("the term carries no port set match")
	}

	err := verifyPolicyConfig([]sdk.ConfigSection{policyWithLeaves(`"source-port":"@hosts"`), fw})
	if err == nil {
		t.Fatal("verify accepted a port match against an ipv4 set")
	}
	if !strings.Contains(err.Error(), "a port match reads only an inet-service set") {
		t.Fatalf("verify refused with %q, want the set type named", err)
	}
}

// TestPolicyProvidedSetNameRefusedWithItsOwner proves a name another plugin
// fills at run time is refused with a message saying so, not with an
// instruction to declare a set the operator cannot declare. The control is an
// ordinary undeclared name, which keeps the declare instruction.
func TestPolicyProvidedSetNameRefusedWithItsOwner(t *testing.T) {
	domainName, _ := firewall.DomainGroupSetNames("partners")
	err := verifyPolicyConfig([]sdk.ConfigSection{policyWithAddress("@" + domainName)})
	if err == nil {
		t.Fatal("verify accepted a reference to a domain-group set")
	}
	if !strings.Contains(err.Error(), "filled at run time by the firewall-domain plugin") {
		t.Fatalf("verify refused with %q, want the owning plugin named", err)
	}
	if strings.Contains(err.Error(), "declare it with") {
		t.Fatalf("verify told the operator to declare a set another plugin fills: %q", err)
	}

	err = verifyPolicyConfig([]sdk.ConfigSection{policyWithAddress("@bypass")})
	if err == nil || !strings.Contains(err.Error(), "declare it with") {
		t.Fatalf("an ordinary undeclared name was refused with %v, want the declare instruction", err)
	}
}

// TestPolicyVerifyRunsTheTranslation proves a refusal only translate gives
// reaches verify. The tcp-flags value is stored as written by the parser and
// read only by translate (buildPolicyMatch, firewall.ParseTCPFlags), so before
// verify ran the translation this config committed and failed at apply.
func TestPolicyVerifyRunsTheTranslation(t *testing.T) {
	section := policyWithLeaves(`"protocol":"tcp","tcp-flags":"bogus"`)
	if _, err := parsePolicySections([]sdk.ConfigSection{section}); err != nil {
		t.Fatalf("the parse already refuses the value, so this test proves nothing about translate: %v", err)
	}
	err := verifyPolicyConfig([]sdk.ConfigSection{section})
	if err == nil {
		t.Fatal("verify accepted a tcp-flags value translate cannot read")
	}
	if !strings.Contains(err.Error(), "tcp-flags") {
		t.Fatalf("verify refused with %q, want the tcp-flags leaf named", err)
	}
}

// TestFirewallCommitNotChangingAReferencedSetTouchesNoIPRule proves a firewall
// commit that leaves every referenced set alone reinstalls nothing. Policy
// routing is a participant of every firewall commit, and a reinstall used to
// remove every ip rule before adding it back, a steering gap on each one.
//
// The method: two deliveries differ only in a set no policy names, and
// policiesUnchanged must call them equal; a delivery that changes the
// referenced set is the control and must not be equal. applyPolicies is then
// handed the unchanged delivery with a nil allocator, so any step past the
// skip dereferences it, and its installed state must stay the very value it
// was.
func TestFirewallCommitNotChangingAReferencedSetTouchesNoIPRule(t *testing.T) {
	policy := policyWithAddress("@bypass")
	parse := func(sets string) []PolicyRoute {
		t.Helper()
		policies, err := verifyPolicySections([]sdk.ConfigSection{policy, firewallWithSets(map[string]string{"edge": sets})})
		if err != nil {
			t.Fatalf("verifyPolicySections: %v", err)
		}
		return policies
	}
	const referenced = `"bypass":{"type":"ipv4","element":{"192.0.2.10":{}}}`
	installed := parse(referenced + `,"other":{"type":"ipv4","element":{"198.51.100.1":{}}}`)
	unrelatedEdit := parse(referenced + `,"other":{"type":"ipv4","element":{"198.51.100.2":{}}}`)
	setEdit := parse(`"bypass":{"type":"ipv4","element":{"192.0.2.20":{}}}`)

	result := &translationResult{}
	if !policiesUnchanged(result, installed, unrelatedEdit) {
		t.Fatal("an edit to a set no policy names counted as a policy change")
	}
	if policiesUnchanged(result, installed, setEdit) {
		t.Fatal("an edit to the referenced set counted as no change: the copy would go stale")
	}
	if policiesUnchanged(nil, nil, nil) {
		t.Fatal("with nothing installed, an install counted as no change")
	}

	var mu sync.Mutex
	current := installed
	currentResult := result
	if err := applyPolicies(nil, unrelatedEdit, result, &mu, &current, &currentResult); err != nil {
		t.Fatalf("applyPolicies over an unchanged policy set: %v", err)
	}
	if currentResult != result {
		t.Fatal("applyPolicies replaced the installed result for an unchanged policy set")
	}
}

// recordingRules is a ruleInstaller that records every call replaceRules makes.
type recordingRules struct {
	addedRules, removedRules   []iprule.Mark
	addedRoutes, removedRoutes []autoRouteSpec
}

func (r *recordingRules) applyAutoRoutes(routes []autoRouteSpec) error {
	r.addedRoutes = append(r.addedRoutes, routes...)
	return nil
}

func (r *recordingRules) applyIPRules(rules []iprule.Mark) error {
	r.addedRules = append(r.addedRules, rules...)
	return nil
}

func (r *recordingRules) removeIPRules(rules []iprule.Mark) {
	r.removedRules = append(r.removedRules, rules...)
}

func (r *recordingRules) removeAutoRoutes(routes []autoRouteSpec) {
	r.removedRoutes = append(r.removedRoutes, routes...)
}

// TestReplaceRulesMovesOnlyTheDifference proves a reinstall touches only the
// rules that changed: a rule both results hold is neither removed nor added,
// so the packets it steers are steered throughout. The control is the rule
// that changed table, which must be added and its old form removed.
func TestReplaceRulesMovesOnlyTheDifference(t *testing.T) {
	kept := iprule.Mark{Priority: 100, Mark: 0x50000, Mask: 0xFFFFFFFF, Table: 100}
	moved := iprule.Mark{Priority: 101, Mark: 0x50001, Mask: 0xFFFFFFFF, Table: 200}
	movedTo := iprule.Mark{Priority: 101, Mark: 0x50001, Mask: 0xFFFFFFFF, Table: 300}
	old := &translationResult{IPRules: []iprule.Mark{kept, moved}}
	next := &translationResult{IPRules: []iprule.Mark{kept, movedTo}}

	rec := &recordingRules{}
	if err := replaceRules(rec, old, next); err != nil {
		t.Fatalf("replaceRules: %v", err)
	}
	if len(rec.addedRules) != 1 || rec.addedRules[0] != movedTo {
		t.Fatalf("added %+v, want only %+v", rec.addedRules, movedTo)
	}
	if len(rec.removedRules) != 1 || rec.removedRules[0] != moved {
		t.Fatalf("removed %+v, want only %+v", rec.removedRules, moved)
	}

	same := &recordingRules{}
	if err := replaceRules(same, old, old); err != nil {
		t.Fatalf("replaceRules over identical results: %v", err)
	}
	if len(same.addedRules)+len(same.removedRules)+len(same.addedRoutes)+len(same.removedRoutes) != 0 {
		t.Fatalf("identical results touched rules or routes: %+v", same)
	}
}

func elementValues(set firewall.Set) string {
	values := make([]string, 0, len(set.Elements))
	for _, element := range set.Elements {
		values = append(values, element.Value)
	}
	return strings.Join(values, ",")
}
