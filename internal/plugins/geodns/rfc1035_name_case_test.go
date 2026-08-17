// Design: docs/architecture/dns/geodns.md -- every name comparison geodns makes
// runs through fqdn, equalName or nsID. This file drives all three from
// answerQuestions, with the query name as the only variable.
// RFC: rfc/short/rfc1035.md -- case-insensitive comparison, and what it does not
// cover

package geodns

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// caseState serves one zone with three hosts. Two differ only in an octet that
// is not a letter, and their ASCII codes are 0x20 apart -- the same bit an
// alphabetic case fold flips. An implementation that folded case by clearing
// that bit over the whole byte range would answer one for the other.
func caseState(t *testing.T) *resolverState {
	t.Helper()
	return answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"nameserver":["10.0.0.1"],`+
		`"host-set":{"web":{"host":{`+
		`"web-1.t.example.":{"address":["10.0.0.5"]},`+
		`"web[.t.example.":{"address":["10.0.0.6"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)
}

// answerAddrs returns the A record addresses in msg's Answer section.
func answerAddrs(msg *dns.Msg) []string {
	var out []string
	for _, rr := range msg.Answer {
		if a, ok := rr.(*dns.A); ok {
			out = append(out, a.A.String())
		}
	}
	return out
}

// VALIDATES: a query name differing from the configured one only in letter case
// is answered identically, at the zone match, the host lookup and the apex
// comparison alike. A query differing in one octet that is not a letter is not
// answered, even when that octet is 0x20 away from the configured one.
// PREVENTS: the two failures this sentence pairs. A case-sensitive lookup makes
// every resolver that randomizes query case (a common cache-poisoning defense)
// see NODATA for a name that exists. A case fold applied by clearing bit 0x20
// over the whole byte range makes "web[" and "web{" the same name.
func TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly(t *testing.T) {
	t.Parallel()
	st := caseState(t)

	// RFC requirement: RFC1035-2.3.3-1 positive -- "For all parts of the DNS that
	// are part of the official protocol, all comparisons between character
	// strings (e.g., labels, domain names, etc.) are done in a case-insensitive
	// manner." RFC 1035 carries no capitalised RFC 2119 keyword anywhere, so this
	// quoted sentence is the whole anchor.
	// RFC requirement: RFC1035-3.1-5 positive -- "Name servers and resolvers must
	// compare labels in a case-insensitive manner (i.e., A=a), assuming ASCII
	// with zero parity."
	lower := answerAddrs(answerA(st, "web-1.t.example."))
	upper := answerAddrs(answerA(st, "WEB-1.T.EXAMPLE."))
	mixed := answerAddrs(answerA(st, "wEb-1.T.eXaMpLe."))
	if len(lower) != 1 {
		t.Fatalf("the exact-case query drew %d A records, want 1", len(lower))
	}
	if strings.Join(upper, ",") != strings.Join(lower, ",") {
		t.Errorf("upper-case query drew %v, the same name in lower case drew %v", upper, lower)
	}
	if strings.Join(mixed, ",") != strings.Join(lower, ",") {
		t.Errorf("mixed-case query drew %v, the same name in lower case drew %v", mixed, lower)
	}

	// The apex comparison is a separate code path from the host lookup: it
	// decides whether an SOA or NS query is answered from the zone or refused
	// into the Authority section.
	apex := new(dns.Msg)
	apex.SetQuestion("T.EXAMPLE.", dns.TypeSOA)
	msg := new(dns.Msg)
	msg.SetReply(apex)
	answerQuestions(msg, apex, st, netip.MustParseAddr("203.0.113.7"))
	if len(msg.Answer) == 0 {
		t.Error("an upper-case apex SOA query drew no answer, so the zone comparison is case-sensitive")
	}

	// RFC requirement: RFC1035-3.1-6 positive -- "Non-alphabetic codes must match
	// exactly." '[' is 0x5B and '{' is 0x7B: they differ by exactly the bit an
	// ASCII case fold flips, and neither is a letter, so they are two names.
	if got := answerAddrs(answerA(st, "web[.t.example.")); len(got) != 1 {
		t.Fatalf("the configured web[ host drew %d A records, want 1", len(got))
	}
	if got := answerAddrs(answerA(st, "web{.t.example.")); len(got) != 0 {
		t.Errorf("web{ drew %v; it is a different name from web[, which the folding of one bit would hide", got)
	}

	// RFC requirement: RFC1035-2.3.3-1 negative -- case-insensitive comparison
	// makes two spellings of ONE name equal. It does not make two names equal:
	// a query differing in a letter, rather than in that letter's case, is a
	// miss and draws the zone SOA instead of an address.
	miss := answerA(st, "web-2.t.example.")
	if got := answerAddrs(miss); len(got) != 0 {
		t.Errorf("web-2 drew %v, though only web-1 is configured", got)
	}
	// RFC requirement: RFC1035-3.1-5 negative -- the same, at the label
	// comparison: "A=a" is the whole of the equivalence, and a miss stays a miss.
	if len(miss.Ns) == 0 {
		t.Error("the miss drew no SOA in the Authority section, so it was not answered as a name in the zone")
	}
	// RFC requirement: RFC1035-3.1-6 negative -- an octet that IS a letter is
	// matched case-insensitively, which is what stops the exact-match rule above
	// being read as exact matching of the whole name.
	if got := answerAddrs(answerA(st, "WEB[.T.EXAMPLE.")); len(got) != 1 {
		t.Errorf("WEB[ drew %v A records, want the 1 configured for web[: the letters fold even though '[' does not", got)
	}
}

// VALIDATES: the reply echoes the query's own spelling of the name, in the
// question section and in the answer RR's owner name, though the config stores
// that name lower-cased. Two spellings of one name draw two differently-spelled
// replies carrying the same address.
// PREVENTS: a reply that lower-cases the name it was asked about. A resolver
// that randomizes query case compares the reply's name against what it sent, and
// discards an answer whose case does not match -- so normalizing the echo turns
// a working zone into a silent failure for exactly the clients defending
// themselves against cache poisoning.
func TestRFC1035_QueryNameCasePreservedInTheReply(t *testing.T) {
	t.Parallel()
	st := caseState(t)

	for _, asked := range []string{"web-1.t.example.", "WEB-1.T.EXAMPLE.", "wEb-1.t.ExAmPlE."} {
		msg := answerA(st, asked)
		// RFC requirement: RFC1035-2.3.3-2 positive -- "Loss of case sensitive
		// data must be minimized." The name the client wrote survives the round
		// trip in both the question and the answer, though geodns stores it
		// lower-cased.
		if len(msg.Question) != 1 || msg.Question[0].Name != asked {
			t.Errorf("question section echoes %v, want %q verbatim", msg.Question, asked)
		}
		if len(msg.Answer) != 1 {
			t.Fatalf("query %q drew %d answers, want 1", asked, len(msg.Answer))
		}
		if got := msg.Answer[0].Header().Name; got != asked {
			t.Errorf("answer owner name is %q, want the queried %q", got, asked)
		}
	}

	// RFC requirement: RFC1035-2.3.3-2 negative -- preserving case must not
	// change WHICH record is found. All three spellings above resolve to the one
	// configured address, so the preservation is in the echo and nowhere else.
	seen := map[string]bool{}
	for _, asked := range []string{"web-1.t.example.", "WEB-1.T.EXAMPLE.", "wEb-1.t.ExAmPlE."} {
		for _, a := range answerAddrs(answerA(st, asked)) {
			seen[a] = true
		}
	}
	if len(seen) != 1 {
		t.Errorf("three spellings of one name resolved to %d distinct addresses, want 1", len(seen))
	}
}
