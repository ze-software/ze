// Design: docs/guide/ospf.md -- Prefix-SID NP/E flags at the installer.
//
// VALIDATES: RFC 8665 sec 5, E-Flag, on the label the installer pushes toward the Prefix-SID
// originator: with NP and E set it is the IPv4 Explicit NULL, the literal label value 0, and
// with NP set and E clear it is the Prefix-SID label itself (16009), never 0.
// PREVENTS: an Explicit NULL constant of the wrong value, which the existing units cannot see
// because they compare the pushed label with sr.ExplicitNullV4 itself.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// RFC requirement: RFC8665-5-3 positive -- with the E-Flag (and NP) set on the originator's
// Prefix-SID, the label installed toward the originator is the IPv4 Explicit NULL, asserted
// as the literal value 0 (the "(0 for IPv4)" clause), not as sr.ExplicitNullV4.
// RFC requirement: RFC8665-5-12 positive -- NP and E both set: the Prefix-SID is replaced by
// an Explicit NULL label, the literal IPv4 value 0, in the installed mpls-fib entry.
func TestRFC8665ExplicitNullPushedAsLabelZero(t *testing.T) {
	label, ok := rfc8665PushTowardOriginator(t, sr.SIDFlags{NP: true, E: true})
	if !ok {
		t.Fatal("NP=1 E=1: no label pushed toward the originator, want the IPv4 Explicit NULL 0")
	}
	if label != 0 {
		t.Fatalf("NP=1 E=1: pushed label %d, want 0 (IPv4 Explicit NULL, RFC 3032)", label)
	}
}

// RFC requirement: RFC8665-5-3 negative -- with the E-Flag clear (NP set) the label is not
// replaced: the installer pushes the Prefix-SID label 16009, and not the value 0.
// RFC requirement: RFC8665-5-12 negative -- NP set without E is not the both-set case: no
// Explicit NULL (0) is installed, the Prefix-SID label 16009 is.
func TestRFC8665NoExplicitNullWithoutEFlag(t *testing.T) {
	label, ok := rfc8665PushTowardOriginator(t, sr.SIDFlags{NP: true})
	if !ok {
		t.Fatal("NP=1 E=0: no label pushed toward the originator, want 16009")
	}
	if label == 0 {
		t.Fatal("NP=1 E=0: pushed the IPv4 Explicit NULL 0 without the E-Flag")
	}
	if label != 16009 {
		t.Fatalf("NP=1 E=0: pushed label %d, want the Prefix-SID label 16009", label)
	}
}
