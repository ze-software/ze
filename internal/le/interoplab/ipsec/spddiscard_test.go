// Design: docs/architecture/testing/interop.md -- spd-discard-entry and pad-subtree-identity, the pure judgements their checkers use.
// Related: spddiscard.go -- requireESPStill
// Related: padsubtree.go -- the identities and the charon log line
package ipsec

import (
	"strings"
	"testing"
)

// TestRequireESPStillRefusesMovementAndAbsence proves the "none of them left over the
// tunnel" half of spd-discard-entry's assertion 4: a direction whose SA moved even one
// byte is refused, and so is a direction with no SA at all, which moves nothing whatever
// the dataplane did. Another direction moving is not this direction's business.
func TestRequireESPStillRefusesMovementAndAbsence(t *testing.T) {
	out := saKey{source: zeIP, target: swanIP, spi: "0xc0000001"}
	in := saKey{source: swanIP, target: zeIP, spi: "0xc0000002"}
	before := map[saKey]uint64{out: 840, in: 1000}

	if err := requireESPStill(before, map[saKey]uint64{out: 840, in: 1336}, zeEncrypts); err != nil {
		t.Fatalf("an outbound SA that moved nothing was refused: %v", err)
	}
	if err := requireESPStill(before, map[saKey]uint64{out: 841, in: 1000}, zeEncrypts); err == nil {
		t.Fatal("an outbound SA that moved one byte was judged still")
	}
	if err := requireESPStill(map[saKey]uint64{in: 1000}, map[saKey]uint64{in: 1000}, zeEncrypts); err == nil {
		t.Fatal("a direction with no SA was judged still")
	}
	if err := requireESPStill(before, map[saKey]uint64{in: 1000}, zeEncrypts); err == nil {
		t.Fatal("an outbound SA that disappeared was judged still")
	}
}

// TestPADIdentitiesStraddleTheSubtree pins the scenario's design: the first identity
// lies under the configured sub-tree, the second does not, and the second still ends
// in the characters after the sub-tree's leading dot, so a match that dropped the label
// boundary would admit it and the scenario would see it.
func TestPADIdentitiesStraddleTheSubtree(t *testing.T) {
	if !strings.HasSuffix(padInsideIdentity, padSubtree) {
		t.Fatalf("%s is not under %s", padInsideIdentity, padSubtree)
	}
	if strings.HasSuffix(padOutsideIdentity, padSubtree) {
		t.Fatalf("%s is under %s, so the refusal phase has nothing to refuse", padOutsideIdentity, padSubtree)
	}
	if !strings.HasSuffix(padOutsideIdentity, strings.TrimPrefix(padSubtree, ".")) {
		t.Fatalf("%s does not end in %s, so it cannot catch a match without the label boundary", padOutsideIdentity, padSubtree)
	}
	if strings.Contains(padOutsideIdentity, padSubtree) {
		t.Fatalf("%s contains %s, so the refusal line check could match the asserted name for the remote-id", padOutsideIdentity, padSubtree)
	}
}

// TestSwanAuthenticatesAsNamesTheIdentity checks the charon line the PAD checker waits
// for carries the identity in the quotes psk_authenticator.c build prints around it, so
// the line for one identity never satisfies the wait for the other.
func TestSwanAuthenticatesAsNamesTheIdentity(t *testing.T) {
	line := "12[IKE] authentication of 'vpn.notexample.com' (myself) with pre-shared key"
	if !strings.Contains(line, swanAuthenticatesAs(padOutsideIdentity)) {
		t.Fatalf("%q does not match charon's line %q", swanAuthenticatesAs(padOutsideIdentity), line)
	}
	if strings.Contains(line, swanAuthenticatesAs(padInsideIdentity)) {
		t.Fatalf("the line for %s matched the wait for %s", padOutsideIdentity, padInsideIdentity)
	}
}
