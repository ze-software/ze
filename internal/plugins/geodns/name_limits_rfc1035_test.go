// Design: docs/architecture/dns/geodns.md -- geodns validates every configured
// name at parse time, and synthesizes the nameserver glue names that no operator
// ever types. This file drives both through parseConfig and then reads the
// resulting name off the packed answer.
// RFC: rfc/short/rfc1035.md -- name and label encoding, and its two size limits

package geodns

import (
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// zoneOfWireOctets builds a fully-qualified zone name whose wire form is exactly
// n octets: one length octet plus its own octets per label, then the root's zero
// octet. Every label it emits is within the 63-octet bound, so the only limit a
// test built on it can cross is the whole-name one.
func zoneOfWireOctets(t *testing.T, n int) string {
	t.Helper()
	if n < 3 {
		t.Fatalf("cannot build a zone of %d wire octets", n)
	}
	var labels []string
	remaining := n - 1 // the root label's zero octet
	for remaining > maxLabelOctets+1 {
		labels = append(labels, strings.Repeat("a", maxLabelOctets))
		remaining -= maxLabelOctets + 1
	}
	labels = append(labels, strings.Repeat("z", remaining-1))
	zone := strings.Join(labels, ".") + "."
	if got := nameWireOctets(zone); got != n {
		t.Fatalf("built a zone of %d wire octets, want %d", got, n)
	}
	return zone
}

// zoneConfig returns the smallest geodns config that serves zone with one
// nameserver, which is what makes the ns1.<zone> glue name reachable.
func zoneConfig(zone string) string {
	return `{"service":{"geodns":{"enabled":"true","zone":["` + zone + `"],` +
		`"nameserver":["10.0.0.1"]}}}`
}

// VALIDATES: a configured label of 64 octets is refused at parse time with the
// label and its length named, and one of 63 is accepted and reaches the wire in
// a length octet whose top two bits are clear.
// PREVENTS: a label the packer cannot encode reaching the answer path, where
// miekg/dns returns an error the harness discards -- every query for that zone
// would then draw no reply at all, with no log and no metric.
func TestRFC1035_ConfiguredLabelBoundedTo63Octets(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC1035-3.1-3 positive -- "The high order two bits of
	// every length octet must be zero, and the remaining six bits of the length
	// field limit the label to 63 octets or less." RFC 1035 carries no
	// capitalised RFC 2119 keyword anywhere, so this quoted sentence is the whole
	// anchor: a grep for MUST finds nothing in the document.
	over := strings.Repeat("a", maxLabelOctets+1) + ".test."
	if _, err := parseConfig(zoneConfig(over)); err == nil {
		t.Errorf("a zone with a %d-octet label parsed without error", maxLabelOctets+1)
	} else {
		for _, want := range []string{"64-octet label", "max 63"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q, so an operator cannot see which value is wrong", err, want)
			}
		}
	}

	// RFC requirement: RFC1035-3.1-3 negative -- "the remaining six bits of the
	// length field limit the label to 63 octets or less" is a bound, not a ban:
	// a label AT 63 octets is legal and reaches the wire in a length octet whose
	// high order two bits are zero.
	at := strings.Repeat("a", maxLabelOctets) + ".test."
	cfg, err := parseConfig(zoneConfig(at))
	if err != nil {
		t.Fatalf("a zone with a %d-octet label was refused: %v", maxLabelOctets, err)
	}

	st := buildState(cfg)
	st.serial = 1
	wire := packedSOAReply(t, st, at)
	lengths := labelLengths(t, wire)
	if len(lengths) == 0 {
		t.Fatal("the packed reply carries no question name")
	}
	if lengths[0] != maxLabelOctets {
		t.Errorf("first length octet = %d, want the configured %d", lengths[0], maxLabelOctets)
	}
	for i, l := range lengths {
		if l&0xC0 != 0 {
			t.Errorf("length octet %d = %#02x has a high bit set; the top two bits mark a compression pointer and must be zero on a label", i, l)
		}
		if l > maxLabelOctets {
			t.Errorf("length octet %d = %d exceeds the %d-octet label limit", i, l, maxLabelOctets)
		}
	}
}

// VALIDATES: the 255-octet whole-name bound is enforced over the SYNTHESIZED
// nameserver glue name, not only over what the operator typed. A zone whose own
// wire form is legal but whose ns1.<zone> is one octet too long is refused; the
// same zone one octet shorter is accepted and its glue answers.
// PREVENTS: the one case a YANG `length "1..255"` cannot see. That bound counts
// presentation characters of a leaf an operator wrote, and appendNS builds a
// name four wire octets longer that appears in no leaf at all.
func TestRFC1035_ConfiguredNameBoundedTo255WireOctets(t *testing.T) {
	t.Parallel()

	const glueOverhead = 4 // "ns" + one digit + "." as wire octets

	// RFC requirement: RFC1035-3.1-4 negative -- "the total length of a domain
	// name (i.e., label octets and label length octets) is restricted to 255
	// octets or less" admits a name AT 255 octets, so the zone whose glue name
	// lands exactly on the limit is served rather than refused.
	fits := zoneOfWireOctets(t, maxNameOctets-glueOverhead)
	cfg, err := parseConfig(zoneConfig(fits))
	if err != nil {
		t.Fatalf("a zone whose glue name is exactly %d wire octets was refused: %v", maxNameOctets, err)
	}
	st := buildState(cfg)
	st.serial = 1
	wire := packedSOAReply(t, st, fits)
	if n := len(wire); n == 0 {
		t.Fatal("the accepted zone packed to nothing")
	}

	// One octet more, and the same zone is refused because of a name no leaf
	// holds. The error must say so, or the operator hunts a value that is not
	// in the config.
	// RFC requirement: RFC1035-3.1-4 positive -- "To simplify implementations,
	// the total length of a domain name (i.e., label octets and label length
	// octets) is restricted to 255 octets or less." The name Ze has to hold to
	// that is the one it synthesizes, which no config leaf carries.
	over := zoneOfWireOctets(t, maxNameOctets-glueOverhead+1)
	_, err = parseConfig(zoneConfig(over))
	if err == nil {
		t.Fatalf("a zone whose glue name is %d wire octets parsed without error", maxNameOctets+1)
	}
	for _, want := range []string{"synthesized nameserver glue name", strconv.Itoa(maxNameOctets + 1), "max 255"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	// A configured name over the bound is refused on its own account too, which
	// is the case that does not depend on synthesis.
	direct := zoneOfWireOctets(t, maxNameOctets+1)
	if _, err := parseConfig(zoneConfig(direct)); err == nil {
		t.Errorf("a %d-octet zone parsed without error", maxNameOctets+1)
	} else if !strings.Contains(err.Error(), "zone") {
		t.Errorf("error %q does not say the zone leaf is the offending one", err)
	}
}

// VALIDATES: a name Ze puts on the wire is a run of labels, each one a length
// octet followed by exactly that many octets, closed by a zero octet and by
// nothing before it. Two names of different label counts and different label
// lengths are read back, so the walk cannot be satisfied by a fixed stride.
// PREVENTS: a reader assuming the encoding is settled because a library owns it.
// Ze chooses which names it hands the packer, and this walk is over the octets
// that left Ze's own answer path.
func TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot(t *testing.T) {
	t.Parallel()

	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"nameserver":["10.0.0.1"],`+
		`"host-set":{"web":{"host":{`+
		`"short.t.example.":{"address":["10.0.0.5"]},`+
		`"a-much-longer-label.deep.t.example.":{"address":["10.0.0.6"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)

	for _, name := range []string{"short.t.example.", "a-much-longer-label.deep.t.example."} {
		wire := packedAReply(t, st, name)
		labels, next := walkName(t, wire, 12)
		// RFC requirement: RFC1035-3.1-1 positive -- "Each label is represented as
		// a one octet length field followed by that number of octets." walkName
		// reads the name back under exactly that rule, and the labels reassemble
		// to the name Ze was asked about.
		if got := strings.Join(labels, ".") + "."; got != name {
			t.Errorf("the packed question name reads back as %q, want %q", got, name)
		}
		// RFC requirement: RFC1035-3.1-2 positive -- "Since every domain name ends
		// with the null label of the root, a domain name is terminated by a length
		// byte of zero."
		if wire[next-1] != 0 {
			t.Errorf("name %q does not end in a zero length octet, it ends in %#02x", name, wire[next-1])
		}
		// RFC requirement: RFC1035-3.1-2 negative -- the zero length byte
		// terminates the name, so it appears nowhere before the root. An empty
		// label mid-name would end the name early for every reader.
		for i, l := range labels {
			if l == "" {
				t.Errorf("name %q carries an empty label at index %d, which would terminate the name early", name, i)
			}
		}
	}

	// The two names must actually differ in shape, or the walk above proves
	// nothing about tracking the input.
	// RFC requirement: RFC1035-3.1-1 negative -- "a one octet length field
	// followed by that number of octets" is a per-label length, not a fixed
	// stride. Two names of different label counts and different label lengths
	// walk correctly under the same rule.
	shortLabels, _ := walkName(t, packedAReply(t, st, "short.t.example."), 12)
	longLabels, _ := walkName(t, packedAReply(t, st, "a-much-longer-label.deep.t.example."), 12)
	if len(shortLabels) == len(longLabels) {
		t.Fatalf("both names walked to %d labels, so the encoding was not exercised at two shapes", len(shortLabels))
	}
}

// walkName reads the name at off in wire as RFC 1035 section 3.1 defines it and
// returns its labels and the offset just past the terminating zero octet. It
// fails the test on any length octet that overruns the message, which is the
// property that makes a length-prefixed encoding parseable at all.
func walkName(t *testing.T, wire []byte, off int) ([]string, int) {
	t.Helper()
	var labels []string
	for {
		if off >= len(wire) {
			t.Fatalf("name at offset %d runs past the end of a %d-octet message", off, len(wire))
		}
		l := int(wire[off])
		if l == 0 {
			return labels, off + 1
		}
		if l&0xC0 != 0 {
			t.Fatalf("length octet %#02x at offset %d has a high bit set; this reply carries no compression", l, off)
		}
		if off+1+l > len(wire) {
			t.Fatalf("label of %d octets at offset %d runs past the end of a %d-octet message", l, off, len(wire))
		}
		labels = append(labels, string(wire[off+1:off+1+l]))
		off += 1 + l
	}
}

// labelLengths returns every length octet of the question name in wire.
func labelLengths(t *testing.T, wire []byte) []byte {
	t.Helper()
	var out []byte
	off := 12
	for off < len(wire) && wire[off] != 0 {
		l := wire[off]
		out = append(out, l)
		if l&0xC0 != 0 {
			return out
		}
		off += 1 + int(l)
	}
	return out
}

// packedAReply builds the reply geodns's answer policy produces for one A query
// and returns its wire octets.
func packedAReply(t *testing.T, st *resolverState, name string) []byte {
	t.Helper()
	return packReply(t, answerA(st, name))
}

// packedSOAReply does the same for an apex SOA query, which is the shape that
// makes geodns synthesize the NS records and their glue.
func packedSOAReply(t *testing.T, st *resolverState, zone string) []byte {
	t.Helper()
	r := new(dns.Msg)
	r.SetQuestion(zone, dns.TypeSOA)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))
	return packReply(t, msg)
}

func packReply(t *testing.T, msg *dns.Msg) []byte {
	t.Helper()
	wire, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack reply: %v", err)
	}
	return wire
}
