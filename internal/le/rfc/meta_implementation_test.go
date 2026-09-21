// VALIDATES: spec-rfc-implementation-classification AC-1, AC-2 and AC-3 -- a
// summary declares WHOSE code answers the document, a kind that owes an
// implementer names one, and a document Ze writes no Go for leaves the gated
// population.
// PREVENTS: a conformance share taken over obligations another layer owns. The
// ledger answers one question, whether ZE's Go code does what the RFC says
// (owner directive, 2026-09-21), and a document counted without Go behind it
// measures somebody else's work as Ze's.

package rfc

import (
	"strings"
	"testing"
)

// metaHead is the part of a Meta table every case below shares.
const metaHead = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n| Title | Widgets |\n"

// metaWhere is the path a refusal names.
const metaWhere = "rfc/short/rfc9999.md"

// enrolmentRows gate the summary, so each case below varies only the
// implementation it declares.
const enrolmentRows = "| Enrolment | enrolled |\n| Enrolment reason | proven both ways |\n"

func TestEveryKindTheVocabularyDeclaresIsReadable(t *testing.T) {
	for _, kind := range ImplementationKinds() {
		t.Run(kind, func(t *testing.T) {
			reason := "Ze's own Go answers it: internal/component/bgp/reactor/peer.go::Run"
			if kind == implementationMixed || kind == implementationThirdParty {
				reason = "Linux XFRM builds every ESP packet; Ze installs the " +
					"security association at internal/component/ike/dataplane/xfrm_linux.go::InstallSA"
			}
			meta, err := ParseMeta(metaHead+enrolmentRows+
				"| Implementation | "+kind+" |\n"+
				"| Implementation reason | "+reason+" |\n"+
				"| Support | - |\n", "rfc9999", metaWhere)
			if err != nil {
				t.Fatalf("a summary declaring %q did not parse: %v", kind, err)
			}
			if meta.Implementation != kind {
				t.Errorf("the parse read %q, want %q", meta.Implementation, kind)
			}
		})
	}
}

func TestADocumentZeWritesNoGoForLeavesTheGatedPopulation(t *testing.T) {
	for _, one := range []struct {
		kind   string
		reason string
		gated  bool
	}{
		{implementationZe, "internal/component/bgp/reactor/peer.go::Run answers it", true},
		{implementationMixed, "Linux XFRM moves the packets; Ze installs the state at " +
			"internal/component/ike/dataplane/xfrm_linux.go::InstallSA", true},
		{implementationThirdParty, "the Linux ipip module builds every outer header", false},
		{implementationFoundation, "the document registers code points and obliges no implementer", false},
	} {
		t.Run(one.kind, func(t *testing.T) {
			meta, err := ParseMeta(metaHead+enrolmentRows+
				"| Implementation | "+one.kind+" |\n"+
				"| Implementation reason | "+one.reason+" |\n"+
				"| Support | - |\n", "rfc9999", metaWhere)
			if err != nil {
				t.Fatalf("the summary did not parse: %v", err)
			}
			if meta.Enrolled() != one.gated {
				t.Errorf("%q gated=%v, want %v: the enrolment row says enrolled either way, "+
					"so the implementation kind is what decides it",
					one.kind, meta.Enrolled(), one.gated)
			}
		})
	}
}

func TestAnImplementationDeclarationIsRefusedWhenItSaysTooLittle(t *testing.T) {
	for _, one := range []struct {
		name string
		rows string
		want string
	}{
		{"absent", "| Support | - |\n", "no `Implementation` row"},
		{"unknown value", "| Implementation | someone-else |\n" +
			"| Implementation reason | x |\n| Support | - |\n", "not one of"},
		{"no reason", "| Implementation | ze |\n| Support | - |\n",
			"no `Implementation reason` row"},
		{"third-party naming only the kernel", "| Implementation | third-party |\n" +
			"| Implementation reason | the kernel does it |\n| Support | - |\n",
			"names no implementer"},
		{"mixed naming only the kernel", "| Implementation | mixed |\n" +
			"| Implementation reason | the kernel handles the rest |\n| Support | - |\n",
			"names no implementer"},
	} {
		t.Run(one.name, func(t *testing.T) {
			_, err := ParseMeta(metaHead+enrolmentRows+one.rows, "rfc9999", metaWhere)
			if err == nil {
				t.Fatalf("%s was accepted", one.name)
			}
			if !strings.Contains(err.Error(), one.want) {
				t.Errorf("the refusal does not say %q:\n%s", one.want, err)
			}
		})
	}
}
