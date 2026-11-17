package network

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// webInstance publishes container port 80 of the instance web, which the
// apply gives 10.88.0.2 on lan, the first address after the gateway.
const webInstance = `"web":{"network":[{"name":"lan"}],"port":{"http":{"public-port":"8080","container-port":"80"}}}`

// webPortNamed is how a finding names webInstance's port.
const webPortNamed = "container web port http"

// webContainer is a container body with lanNetwork and webInstance.
var webContainer = portInstances(webInstance)

// The accepting terms the cases write, as the term list of a chain.
const (
	termToBridge  = `,"term":[{"name":"to-lan","from":{"output-interface":"zcn-lan"},"then":{"accept":{}}}]`
	termToAddress = `,"term":[{"name":"to-web","from":{"destination-address":"10.88.0.2/32"},"then":{"accept":{}}}]`
)

// forwardPolicy is a firewall body with one inet table, wan, holding one
// base chain named forward at the forward hook, with the policy and the term
// list given.
func forwardPolicy(policy, terms string) string {
	return `{"table":{"wan":{"family":"inet","chain":{"forward":{"type":"filter","hook":"forward","priority":"0","policy":"` +
		policy + `"` + terms + `}}}}}`
}

// policySections is a delivery with the container body and the firewall
// body given, and an empty interface config.
func policySections(container, firewall string) []sdk.ConfigSection {
	return []sdk.ConfigSection{
		{Root: containerRoot, Data: `{"container":` + container + `}`},
		{Root: interfaceRoot, Data: `{"interface":{}}`},
		{Root: firewallRoot, Data: `{"firewall":` + firewall + `}`},
	}
}

// portFindings parses the delivery, applies it through a fresh manager on a
// fake engine, as a commit does, and answers the port-blocked check.
func portFindings(t *testing.T, container, firewall string) []rpc.DoctorCheckDiagnostic {
	t.Helper()
	cfg, err := parseSections(policySections(container, firewall))
	if err != nil {
		t.Fatalf("parse refused a valid config: %v", err)
	}
	m := newManager()
	m.receive(newFakeEngine())
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	findings, err := m.answerPortBlocked()
	if err != nil {
		t.Fatalf("the check answered an error: %v", err)
	}
	return findings
}

// TestPortBlockedFindings: the port-blocked check reports a published port
// when a base chain at the forward hook has policy drop and no accepting
// term names the network's bridge as output interface or the instance's
// address as destination, and reports nothing otherwise.
//
// Method: each case is a container body and a firewall body applied as a
// commit applies them, with the "container NAME port NAME" each finding it
// expects must name, in order. A case that expects none fails on any finding,
// and the first case is the control that shows the check does report.
func TestPortBlockedFindings(t *testing.T) {
	twoInstances := portInstances(webInstance +
		`,"zdb":{"network":[{"name":"lan"}],"port":{"sql":{"public-port":"5432","container-port":"5432"}}}`)
	cases := []struct {
		name      string
		container string
		firewall  string
		want      []string
	}{
		{"policy drop and no accepting term", webContainer, forwardPolicy(policyDrop, ""), []string{webPortNamed}},
		{"accepting term by output interface the bridge", webContainer, forwardPolicy(policyDrop, termToBridge), nil},
		{"accepting term by output interface prefix", webContainer,
			forwardPolicy(policyDrop, `,"term":[{"name":"to-any","from":{"output-interface":"zcn-*"},"then":{"accept":{}}}]`), nil},
		{"accepting term by destination the instance address", webContainer, forwardPolicy(policyDrop, termToAddress), nil},
		{"accepting term by destination the network prefix", webContainer,
			forwardPolicy(policyDrop, `,"term":[{"name":"to-lan","from":{"destination-address":"10.88.0.0/24"},"then":{"accept":{}}}]`), nil},
		{"forward chain with policy accept", webContainer, forwardPolicy(chainPolicyAccept, ""), nil},
		{"no forward base chain", webContainer,
			`{"table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop"}}}}}`, nil},
		{"no firewall table", webContainer, `{}`, nil},
		{"two ports where one is allowed", twoInstances, forwardPolicy(policyDrop, termToAddress), []string{"container zdb port sql"}},
		{"a disabled instance", portInstances(strings.Replace(webInstance, `"web":{`, `"web":{"enabled":"false",`, 1)),
			forwardPolicy(policyDrop, ""), nil},
		{"a term that names another bridge", webContainer,
			forwardPolicy(policyDrop, `,"term":[{"name":"to-wan","from":{"output-interface":"zcn-wan"},"then":{"accept":{}}}]`),
			[]string{webPortNamed}},
		{"a term that names the bridge and drops", webContainer,
			forwardPolicy(policyDrop, `,"term":[{"name":"no-lan","from":{"output-interface":"zcn-lan"},"then":{"drop":{}}}]`),
			[]string{webPortNamed}},
		{"a destination that names a set is not judged", webContainer,
			forwardPolicy(policyDrop, `,"term":[{"name":"to-set","from":{"destination-address":"@servers"},"then":{"accept":{}}}]`),
			[]string{webPortNamed}},
		{"a dropping chain of the other family", webContainer,
			`{"table":{"wan6":{"family":"ip6","chain":{"forward":{"type":"filter","hook":"forward","priority":"0","policy":"drop"}}}}}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := portFindings(t, tc.container, tc.firewall)
			if len(findings) != len(tc.want) {
				t.Fatalf("%d findings %+v, want %d naming %v", len(findings), findings, len(tc.want), tc.want)
			}
			for position, finding := range findings {
				if finding.Code != codePortBlocked {
					t.Fatalf("finding %d carries code %q, want %q", position, finding.Code, codePortBlocked)
				}
				if !strings.HasPrefix(finding.Message, tc.want[position]+" is published and the firewall policy blocks it") {
					t.Fatalf("finding %d says %q, want it to name %q", position, finding.Message, tc.want[position])
				}
			}
		})
	}
}

// webFixLines are the lines the finding of webContainer under a dropping
// chain forward of table wan gives as its fix, before the commit.
const webFixLines = "set firewall table wan chain forward term container_web_http from protocol tcp\n" +
	"set firewall table wan chain forward term container_web_http from destination-port 80\n" +
	"set firewall table wan chain forward term container_web_http from output-interface zcn-lan\n" +
	"set firewall table wan chain forward term container_web_http then accept\n"

// TestPortBlockedFindingNamesTheTermToAdd: the finding is a warning that
// names the chain, the table, the bridge and the instance's address, and its
// fix is the accepting term as set lines followed by the commit.
//
// Method: one blocked port, the finding compared field by field, the fix
// compared whole.
func TestPortBlockedFindingNamesTheTermToAdd(t *testing.T) {
	findings := portFindings(t, webContainer, forwardPolicy(policyDrop, ""))
	if len(findings) != 1 {
		t.Fatalf("%d findings %+v, want one", len(findings), findings)
	}
	finding := findings[0]
	if finding.Severity != severityPortBlocked {
		t.Fatalf("severity %q, want %q", finding.Severity, severityPortBlocked)
	}
	for _, part := range []string{
		"chain forward of firewall table wan is at the forward hook with policy drop",
		"output-interface zcn-lan or destination-address 10.88.0.2,",
		"accepts tcp to port 80 leaving by zcn-lan",
		"connection-state established,related or by input-interface zcn-lan",
	} {
		if !strings.Contains(finding.Message, part) {
			t.Fatalf("the message %q does not hold %q", finding.Message, part)
		}
	}
	if finding.Fix != webFixLines+"commit now" {
		t.Fatalf("the fix is\n%s\nwant\n%scommit now", finding.Fix, webFixLines)
	}
}

// TestPortBlockedFixLinesParse: the set lines of the fix are statements the
// config parser accepts under the firewall schema, after the lines that
// declare the chain they extend.
//
// Method: the lines are parsed as a set-format config with the YANG schema
// of this build. The control is the same config with the output-interface
// leaf misspelled, which the parser must refuse: a parse that accepted any
// line would pass the first half alone.
func TestPortBlockedFixLinesParse(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("the YANG schema does not load: %v", err)
	}
	chain := "set firewall table wan family inet\n" +
		"set firewall table wan chain forward type filter\n" +
		"set firewall table wan chain forward hook forward\n" +
		"set firewall table wan chain forward priority 0\n" +
		"set firewall table wan chain forward policy drop\n"
	if _, err := config.ParseSetFormat(schema, chain+webFixLines, config.FormatSet); err != nil {
		t.Fatalf("the fix lines do not parse: %v\n%s", err, webFixLines)
	}
	misspelled := strings.Replace(webFixLines, " from output-interface ", " from output-device ", 1)
	if misspelled == webFixLines {
		t.Fatal("the control did not change the fix lines")
	}
	if _, err := config.ParseSetFormat(schema, chain+misspelled, config.FormatSet); err == nil {
		t.Fatal("the parser accepted a leaf the firewall schema does not hold, so it proves nothing about the fix lines")
	}
}

// TestPortBlockedJudgesEachFamily: a port published in both families is
// blocked by a chain of a table that carries one of them, and the finding
// names the address of that family only. A range is written as a range.
//
// Method: a dual-stack network, a port range with no public address, and one
// dropping forward chain in a table of family ip.
func TestPortBlockedJudgesEachFamily(t *testing.T) {
	container := `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"]}},` +
		`"instance":{"web":{"network":[{"name":"lan"}],"port":{"media":{"protocol":"udp","public-port":"5300-5310","container-port":"5300-5310"}}}}}`
	firewall := `{"table":{"wan4":{"family":"ip","chain":{"forward":{"type":"filter","hook":"forward","priority":"0","policy":"drop"}}}}}`
	findings := portFindings(t, container, firewall)
	if len(findings) != 1 {
		t.Fatalf("%d findings %+v, want one", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "destination-address 10.88.0.2,") {
		t.Fatalf("the message %q does not name the IPv4 address alone", findings[0].Message)
	}
	if strings.Contains(findings[0].Message, "fd00:88::") {
		t.Fatalf("the message %q names the IPv6 address a table of family ip does not carry", findings[0].Message)
	}
	want := "set firewall table wan4 chain forward term container_web_media from destination-port 5300-5310\n"
	if !strings.Contains(findings[0].Fix, want) {
		t.Fatalf("the fix\n%s\ndoes not hold %q", findings[0].Fix, want)
	}
	if !strings.Contains(findings[0].Fix, " from protocol udp\n") {
		t.Fatalf("the fix\n%s\ndoes not name the port's protocol", findings[0].Fix)
	}
}

// TestFirewallPolicyChangeIsANewConfig: a commit that changes only the
// firewall policy gives the network package a config that differs, so the
// config transaction installs it and the check judges the new policy. A
// second apply of the config that accepts the port clears the finding.
//
// Method: the same container body parsed with two firewall bodies; the two
// configs must differ, and one manager applies them in turn.
func TestFirewallPolicyChangeIsANewConfig(t *testing.T) {
	blocked, err := parseSections(policySections(webContainer, forwardPolicy(policyDrop, "")))
	if err != nil {
		t.Fatalf("parse refused the blocking policy: %v", err)
	}
	allowed, err := parseSections(policySections(webContainer, forwardPolicy(policyDrop, termToBridge)))
	if err != nil {
		t.Fatalf("parse refused the accepting policy: %v", err)
	}
	if reflect.DeepEqual(blocked, allowed) {
		t.Fatal("two firewall policies give one network config, so a policy commit installs nothing")
	}
	again, err := parseSections(policySections(webContainer, forwardPolicy(policyDrop, termToBridge)))
	if err != nil {
		t.Fatalf("the second parse refused the accepting policy: %v", err)
	}
	if !reflect.DeepEqual(allowed, again) {
		t.Fatal("two parses of one config differ, so every commit would install it again")
	}
	m := newManager()
	m.receive(newFakeEngine())
	ctx := context.Background()
	if err := m.apply(ctx, blocked); err != nil {
		t.Fatalf("apply of the blocking policy: %v", err)
	}
	findings, err := m.answerPortBlocked()
	if err != nil {
		t.Fatalf("the check answered an error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("%d findings %+v under the blocking policy, want one", len(findings), findings)
	}
	if err := m.apply(ctx, allowed); err != nil {
		t.Fatalf("apply of the accepting policy: %v", err)
	}
	findings, err = m.answerPortBlocked()
	if err != nil {
		t.Fatalf("the check answered an error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("the finding stays after the accepting term is applied: %+v", findings)
	}
}

// TestForwardChainsRefuseUnorderedTerms: a term list of two or more entries
// delivered with no order is refused, as the firewall component refuses it,
// rather than judged in an arbitrary order.
//
// Method: the map form of a two-term list with no order key.
func TestForwardChainsRefuseUnorderedTerms(t *testing.T) {
	terms := `,"term":{"a":{"then":{"accept":{}}},"b":{"then":{"accept":{}}}}`
	_, err := parseSections(policySections(webContainer, forwardPolicy(policyDrop, terms)))
	if err == nil {
		t.Fatal("a term list with no order was accepted")
	}
	if !strings.Contains(err.Error(), "the terms of chain forward of firewall table wan cannot be read in their order") {
		t.Fatalf("the refusal %q does not name the chain and the table", err.Error())
	}
}
