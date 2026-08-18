// External test package: internal/component/firewall imports this package's
// register.go, so an in-package test that reached back for the canonical table
// would close an import cycle.
package yang_test

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	fwyang "github.com/ze-software/ze/internal/component/firewall/yang"
)

// TestProtocolNameEnumMatchesCanonicalTable ties the YANG surface to the one
// protocol table the backends resolve against.
//
// VALIDATES: the protocol-name typedef enumerates exactly the names
// firewall.ProtocolNames returns.
// PREVENTS: an operator committing a protocol the schema accepts and every
// backend then refuses, and the reverse -- a name the table gained that no
// config can reach.
func TestProtocolNameEnumMatchesCanonicalTable(t *testing.T) {
	body, ok := typedefBody(fwyang.ZeFirewallConfYANG, "protocol-name")
	if !ok {
		t.Fatal("ze-firewall-conf.yang no longer declares typedef protocol-name")
	}

	inSchema := make(map[string]bool)
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "enum ") {
			continue
		}
		inSchema[strings.TrimSuffix(strings.TrimPrefix(line, "enum "), ";")] = true
	}

	for _, name := range firewall.ProtocolNames() {
		if !inSchema[name] {
			t.Errorf("firewall.ProtocolNames has %q but typedef protocol-name does not enumerate it", name)
		}
		delete(inSchema, name)
	}
	for name := range inSchema {
		t.Errorf("typedef protocol-name enumerates %q, which firewall.ProtocolNames does not carry", name)
	}
}

// typedefBody returns the text of the named typedef, braces included.
func typedefBody(module, name string) (string, bool) {
	start := strings.Index(module, "typedef "+name+" {")
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
