// Design: docs/architecture/core-design.md -- Firewall YANG schema parity tests
// Related: ze-firewall-conf.yang -- typedef tcp-flags and grouping from-block
// Related: rate_unit_test.go -- sibling YANG pattern parity test

package yang_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	fwyang "github.com/ze-software/ze/internal/component/firewall/yang"
)

// TestTCPFlagsYANGPatternMatchesGo ties the TCP flag names an operator can
// write in ze-firewall-conf.yang to the single Go table in firewall/config.go.
//
// VALIDATES: typedef tcp-flags in ze-firewall-conf.yang accepts exactly the
// flag names firewall.TCPFlagNames returns in both the first and repeated
// pattern alternations, and grouping from-block declares leaf tcp-flags without
// a ze:backend restriction so both kernel and vpp accept it.
// PREVENTS: a TCP flag name drifting between the YANG pattern and
// ParseTCPFlags, or leaf tcp-flags disappearing from grouping from-block.
func TestTCPFlagsYANGPatternMatchesGo(t *testing.T) {
	body, ok := typedefBody(fwyang.ZeFirewallConfYANG, "tcp-flags")
	if !ok {
		t.Fatal("ze-firewall-conf.yang no longer declares typedef tcp-flags")
	}

	first, repeated, ok := tcpFlagAlternations(body)
	if !ok {
		t.Fatalf("typedef tcp-flags no longer carries expected comma-list pattern: %s", body)
	}

	declared := firewall.TCPFlagNames()
	if !slices.Equal(first, declared) {
		t.Errorf("typedef tcp-flags leading alternation accepts %v, and firewall accepts %v", first, declared)
	}
	if !slices.Equal(repeated, declared) {
		t.Errorf("typedef tcp-flags repeated alternation accepts %v, and firewall accepts %v", repeated, declared)
	}

	fromBlock, ok := blockBody(fwyang.ZeFirewallConfYANG, "grouping from-block")
	if !ok {
		t.Fatal("ze-firewall-conf.yang no longer declares grouping from-block")
	}
	leafBody, ok := blockBody(fromBlock, "leaf tcp-flags")
	if !ok {
		t.Fatal("grouping from-block no longer declares leaf tcp-flags")
	}
	if !strings.Contains(leafBody, "type tcp-flags;") {
		t.Errorf("leaf tcp-flags does not reference typedef tcp-flags: %s", leafBody)
	}
	if strings.Contains(leafBody, "ze:backend") {
		t.Errorf("leaf tcp-flags must not carry ze:backend because both kernel and vpp support MatchTCPFlags: %s", leafBody)
	}
}

// tcpFlagAlternations extracts the sorted flag-name lists from the leading
// `(a|b|...)` group and the trailing `(,(a|b|...))*` group of typedef tcp-flags.
func tcpFlagAlternations(body string) (first, repeated []string, ok bool) {
	_, afterPattern, found := strings.Cut(body, "pattern '")
	if !found {
		return nil, nil, false
	}
	pattern, _, closed := strings.Cut(afterPattern, "'")
	if !closed {
		return nil, nil, false
	}
	leadPart, tailPart, split := strings.Cut(pattern, "(,(")
	if !split {
		return nil, nil, false
	}
	if !strings.HasPrefix(leadPart, "(") {
		return nil, nil, false
	}
	if !strings.HasSuffix(leadPart, ")") {
		return nil, nil, false
	}
	tailGroup, _, hasClose := strings.Cut(tailPart, "))")
	if !hasClose {
		return nil, nil, false
	}
	first = strings.Split(leadPart[1:len(leadPart)-1], "|")
	repeated = strings.Split(tailGroup, "|")
	slices.Sort(first)
	slices.Sort(repeated)
	return first, repeated, true
}

// blockBody returns the text of a named YANG block (`<header> { ... }`), braces included.
func blockBody(module, header string) (string, bool) {
	start := strings.Index(module, header+" {")
	if start < 0 {
		return "", false
	}
	depth := 0
	for i := start; i < len(module); i++ {
		switch module[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return module[start : i+1], true
			}
		}
	}
	return "", false
}
