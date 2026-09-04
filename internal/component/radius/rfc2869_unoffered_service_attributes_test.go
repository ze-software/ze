// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute dictionary
// Related: dict.go -- the attribute type constants under test
//
// VALIDATES: that the RADIUS attribute dictionary declares an attribute only for
// a service Ze's NAS actually offers.
// PREVENTS: a contributor adding the ARAP attribute set to dict.go as "support",
// when no ARAP module exists anywhere in the tree. RFC 2869 Section 1.1 forbids
// exactly that, and the cost is not cosmetic: an attribute in an Access-Request
// commits the NAS to the obligations of the service it names, so a server that
// answered on that service would meet a NAS that cannot continue the
// conversation.
//
// EAP-Message was on the forbidden list until 2026-09-04. It moved to the
// offered list when the admin backend gained a RADIUS/EAP conversation, because
// the rule is conditional on the service and reverses with it: the NAS that
// offers EAP owes the attribute rather than being forbidden it.
//
// The test reads dict.go itself rather than a hand-kept list, because a constant
// nobody registered is the thing being looked for. A registry would answer only
// for what was registered.

package radius

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// unofferedServiceAttrs maps a RADIUS attribute type this document defines to
// the service it belongs to, for every service Ze's NAS does not offer.
//
// RFC 2869 Section 5 assigns these values: 70 ARAP-Password, 71 ARAP-Features,
// 72 ARAP-Zone-Access, 73 ARAP-Security, 74 ARAP-Security-Data, 79 EAP-Message,
// 84 ARAP-Challenge-Response.
var unofferedServiceAttrs = map[int]string{
	70: "ARAP",
	71: "ARAP",
	72: "ARAP",
	73: "ARAP",
	74: "ARAP",
	84: "ARAP",
}

// offeredServiceAttrs names attributes Ze must declare, because it does offer
// the service each one belongs to: PPP with PAP or CHAP over L2TP, RADIUS
// accounting with Gigaword counters, and RADIUS/EAP for operator login.
//
// EAP-Message moved here on 2026-09-04, when plan/spec-radius-admin-eap.md gave
// the admin backend an EAP peer of its own. The RFC 2869 Section 1.1 rule is
// conditional on the service, so the attribute is forbidden while the service is
// absent and REQUIRED once it exists: `auth-method eap-md5` and `eap-mschapv2`
// run a full RADIUS/EAP conversation (eap.go, authenticator.go), and a NAS that
// offers EAP and declares no EAP-Message could not encapsulate one.
var offeredServiceAttrs = map[string]int{
	"AttrUserPassword":        2,
	"AttrCHAPPassword":        3,
	"AttrCHAPChallenge":       60,
	"AttrAcctInputGigawords":  52,
	"AttrAcctOutputGigawords": 53,
	"AttrEAPMessage":          79,
	"AttrNASPortID":           87,
}

// declaredAttrConstants parses dict.go and returns every `Attr...` constant it
// declares, keyed by name.
func declaredAttrConstants(t *testing.T) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "dict.go", nil, 0)
	if err != nil {
		t.Fatalf("parse dict.go: %v", err)
	}

	found := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || len(spec.Values) != 1 {
			return true
		}
		name := spec.Names[0].Name
		if len(name) < 5 || name[:4] != "Attr" {
			return true
		}
		lit, ok := spec.Values[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		v, convErr := strconv.Atoi(lit.Value)
		if convErr != nil {
			return true
		}
		found[name] = v
		return true
	})
	if len(found) == 0 {
		t.Fatal("dict.go declared no Attr constant; the parser found nothing to judge")
	}
	return found
}

// TestRFC2869DictionaryCoversTheServicesZeOffers is the other half of the pair:
// it proves the parser sees the constants that ARE declared, so the absence the
// negative case asserts is a real absence and not a parser that found nothing.
//
// RFC requirement: RFC2869-1.1-1 positive -- the dictionary declares the RADIUS
// attributes for the services Ze's NAS does offer (dict.go).
// RFC requirement: RFC3579-1-1 positive -- RFC 3579 Section 1 states the rule
// for EAP ("a NAS that is unable to offer EAP service MUST NOT implement the
// RADIUS attributes for EAP"). Ze IS able to offer it: `auth-method eap-md5`
// and `eap-mschapv2` run a RADIUS/EAP conversation (authenticator_eap.go
// authenticateEAP). This case asserts the other half of the pairing, that
// AttrEAPMessage (dict.go, type 79) is declared, so the two halves together
// hold the attribute and the service to each other.
// RFC requirement: RFC2865-1.1-1 positive -- RFC 2865 Section 1.1 carries the
// same sentence, and dict.go is the one dictionary both documents bind, so the
// constants this test finds for the services Ze does offer are the control for
// the RFC 2865 obligation too.
func TestRFC2869DictionaryCoversTheServicesZeOffers(t *testing.T) {
	declared := declaredAttrConstants(t)
	for name, want := range offeredServiceAttrs {
		got, ok := declared[name]
		if !ok {
			t.Errorf("dict.go does not declare %s; Ze offers the service it belongs to", name)
			continue
		}
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService walks every
// attribute constant dict.go declares and refuses one that belongs to a service
// Ze's NAS does not offer.
//
// RFC 2869 Section 1.1: "A NAS that does not implement a given service MUST NOT
// implement the RADIUS attributes for that service. For example, a NAS that is
// unable to offer ARAP service MUST NOT implement the RADIUS attributes for
// ARAP."
//
// RFC requirement: RFC2869-1.1-1 negative -- no attribute constant names an ARAP
// attribute, because Ze carries no ARAP module anywhere (dict.go).
// RFC requirement: RFC2865-1.1-1 negative -- RFC 2865 Section 1.1 states it in
// the same words ("A NAS that does not implement a given service MUST NOT
// implement the RADIUS attributes for that service"), and dict.go declares no
// ARAP attribute.
// RFC requirement: RFC3579-1-1 negative -- RFC 3579 Section 1 states the same
// rule for EAP ("a NAS that is unable to offer EAP service MUST NOT implement
// the RADIUS attributes for EAP"). This case is the refusing half: it walks
// every constant dict.go declares and fails on one whose service Ze does not
// offer. AttrEAPMessage is no longer on that list, because Ze does offer EAP,
// and the sibling test above requires the constant for exactly that reason.
func TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService(t *testing.T) {
	for name, value := range declaredAttrConstants(t) {
		if service, forbidden := unofferedServiceAttrs[value]; forbidden {
			t.Errorf("dict.go declares %s = %d, a RADIUS attribute for %s; "+
				"Ze's NAS offers no %s service", name, value, service, service)
		}
	}
}
