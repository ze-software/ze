package message

import (
	"strings"
	"testing"
)

// asPathSegment returns one AS_PATH segment: type, count, then each AS number in
// asnOctets octets, big-endian.
func asPathSegment(segType uint8, asnOctets int, asns ...uint32) []byte {
	out := []byte{segType, byte(len(asns))}
	for _, asn := range asns {
		if asnOctets == 4 {
			out = append(out, byte(asn>>24), byte(asn>>16))
		}
		out = append(out, byte(asn>>8), byte(asn))
	}
	return out
}

func joinSegments(segments ...[]byte) []byte {
	var out []byte
	for _, segment := range segments {
		out = append(out, segment...)
	}
	return out
}

// TestValidateASPathRejectsSetSegments proves the AS_PATH half of the RFC 9774 Section 3
// receive rule. Goal: an AS_SET or AS_CONFED_SET segment anywhere in a well-framed AS_PATH,
// on a two-octet or a four-octet session, is treat-as-withdraw naming RFC 9774 Section 3,
// while AS_SEQUENCE and AS_CONFED_SEQUENCE paths stay accepted. Method: build each path
// from segments and call the registered validator, validateASPathAttr, directly.
//
// RFC requirement: RFC9774-3-2 positive -- an AS_SET or AS_CONFED_SET segment in a
// well-framed AS_PATH, at either AS width, is treat-as-withdraw on attribute code 2 with
// the RFC 9774 Section 3 description.
// RFC requirement: RFC9774-3-2 negative -- AS_PATHs of AS_SEQUENCE and AS_CONFED_SEQUENCE
// segments only draw no verdict.
func TestValidateASPathRejectsSetSegments(t *testing.T) {
	for _, asnOctets := range []int{2, 4} {
		asn4 := asnOctets == 4
		seq := asPathSegment(asPathTypeASSequence, asnOctets, 65001, 65002)
		confedSeq := asPathSegment(asPathTypeConfedSeq, asnOctets, 65100)
		set := asPathSegment(asPathTypeASSet, asnOctets, 65010, 65011)
		confedSet := asPathSegment(asPathTypeConfedSet, asnOctets, 65110)

		rejected := []struct {
			name    string
			path    []byte
			segName string
		}{
			{"set only", set, "AS_SET"},
			{"set first", joinSegments(set, seq), "AS_SET"},
			{"set middle", joinSegments(seq, set, seq), "AS_SET"},
			{"set last", joinSegments(seq, set), "AS_SET"},
			{"confed set only", confedSet, "AS_CONFED_SET"},
			{"confed set after confed seq", joinSegments(confedSeq, confedSet, seq), "AS_CONFED_SET"},
			{"confed set last", joinSegments(seq, confedSet), "AS_CONFED_SET"},
		}
		for _, tc := range rejected {
			got := validateASPathAttr(attrCodeASPath, len(tc.path), tc.path, false, asn4, receivePolicy{})
			if got == nil {
				t.Fatalf("asn4=%v %s: accepted, want treat-as-withdraw", asn4, tc.name)
			}
			if got.Action != RFC7606ActionTreatAsWithdraw {
				t.Errorf("asn4=%v %s: action %v, want treat-as-withdraw", asn4, tc.name, got.Action)
			}
			if got.AttrCode != attrCodeASPath {
				t.Errorf("asn4=%v %s: attr code %d, want %d", asn4, tc.name, got.AttrCode, attrCodeASPath)
			}
			want := "RFC 9774 Section 3: " + tc.segName + " in AS_PATH"
			if got.Description != want {
				t.Errorf("asn4=%v %s: description %q, want %q", asn4, tc.name, got.Description, want)
			}
		}

		accepted := map[string][]byte{
			"sequence":                  seq,
			"confed sequence then seq":  joinSegments(confedSeq, seq),
			"two sequences":             joinSegments(seq, seq),
			"empty path":                nil,
			"confed sequence alone":     confedSeq,
			"sequence then confed seq":  joinSegments(seq, confedSeq),
			"three mixed non-set types": joinSegments(confedSeq, seq, seq),
		}
		for name, path := range accepted {
			if got := validateASPathAttr(attrCodeASPath, len(path), path, false, asn4, receivePolicy{}); got != nil {
				t.Errorf("asn4=%v %s: %q, want accepted", asn4, name, got.Description)
			}
		}
	}
}

// TestValidateASPathStructureBeforeSet proves the precedence AC-8 asks for. Goal: an
// AS_PATH that holds a set AND a structural fault reports the RFC 7606 Section 7.2
// structural description, never the RFC 9774 one. Method: an AS_SET segment followed by
// an overrunning segment, and one followed by a stray trailing octet.
func TestValidateASPathStructureBeforeSet(t *testing.T) {
	set := asPathSegment(asPathTypeASSet, 4, 65010)
	overrun := joinSegments(set, []byte{asPathTypeASSequence, 3, 0, 0, 0xfd, 0xe9})
	underrun := joinSegments(set, []byte{asPathTypeASSequence})
	for name, path := range map[string][]byte{"overrun": overrun, "underrun": underrun} {
		got := validateASPathAttr(attrCodeASPath, len(path), path, false, true, receivePolicy{})
		if got == nil {
			t.Fatalf("%s: accepted, want the structural fault", name)
		}
		if !strings.HasPrefix(got.Description, "RFC 7606 Section 7.2:") {
			t.Errorf("%s: description %q, want the RFC 7606 Section 7.2 structural fault", name, got.Description)
		}
	}
}

// TestValidateAS4PathRejectsSetSegments proves the AS4_PATH half of the RFC 9774 Section 3
// receive rule. Goal: a set segment in a well-framed AS4_PATH is treat-as-withdraw (not
// the RFC 6793 attribute discard), even when the same AS4_PATH also holds AS 0; AS 0
// without a set still earns attribute discard; a misframed AS4_PATH holding a set is not
// judged by the set scan (A-7). Method: call validateAS4PathAttr directly, once with each
// asn4 value, since the verdict does not depend on it (owner Q-2 (a)).
//
// RFC requirement: RFC9774-3-2 positive -- an AS_SET or AS_CONFED_SET segment in a
// well-framed AS4_PATH, with four-octet AS support negotiated or not, is treat-as-withdraw on
// attribute code 17 with the RFC 9774 Section 3 description, also when the AS4_PATH holds AS 0.
func TestValidateAS4PathRejectsSetSegments(t *testing.T) {
	seq := asPathSegment(asPathTypeASSequence, 4, 65001, 4200000000)
	set := asPathSegment(asPathTypeASSet, 4, 65010, 65011)
	confedSet := asPathSegment(asPathTypeConfedSet, 4, 65110)
	zeroSeq := asPathSegment(asPathTypeASSequence, 4, 0)

	for _, asn4 := range []bool{false, true} {
		cases := []struct {
			name    string
			path    []byte
			segName string
		}{
			{"set", joinSegments(seq, set), "AS_SET"},
			{"confed set", joinSegments(confedSet, seq), "AS_CONFED_SET"},
			{"set outranks AS 0 discard", joinSegments(zeroSeq, set), "AS_SET"},
		}
		for _, tc := range cases {
			got := validateAS4PathAttr(attrCodeAS4Path, len(tc.path), tc.path, false, asn4, receivePolicy{})
			if got == nil {
				t.Fatalf("asn4=%v %s: accepted, want treat-as-withdraw", asn4, tc.name)
			}
			if got.Action != RFC7606ActionTreatAsWithdraw {
				t.Errorf("asn4=%v %s: action %v, want treat-as-withdraw", asn4, tc.name, got.Action)
			}
			if got.AttrCode != attrCodeAS4Path {
				t.Errorf("asn4=%v %s: attr code %d, want %d", asn4, tc.name, got.AttrCode, attrCodeAS4Path)
			}
			want := "RFC 9774 Section 3: " + tc.segName + " in AS4_PATH"
			if got.Description != want {
				t.Errorf("asn4=%v %s: description %q, want %q", asn4, tc.name, got.Description, want)
			}
		}

		got := validateAS4PathAttr(attrCodeAS4Path, len(zeroSeq), zeroSeq, false, asn4, receivePolicy{})
		if got == nil || got.Action != RFC7606ActionAttributeDiscard {
			t.Errorf("asn4=%v AS 0 without set: %+v, want attribute discard", asn4, got)
		}
		if got := validateAS4PathAttr(attrCodeAS4Path, len(seq), seq, false, asn4, receivePolicy{}); got != nil {
			t.Errorf("asn4=%v clean AS4_PATH: %q, want accepted", asn4, got.Description)
		}

		misframed := joinSegments(set, []byte{asPathTypeASSequence, 2, 0, 0})
		if got := validateAS4PathAttr(attrCodeAS4Path, len(misframed), misframed, false, asn4, receivePolicy{}); got != nil {
			t.Errorf("asn4=%v misframed AS4_PATH with set: %q, want no verdict from the set scan", asn4, got.Description)
		}
	}
}

// TestValidateUpdateAcceptASSetKeepsSetsOnly proves the operator exception of RFC 9774
// Section 3 at the validator: with acceptASSet true an AS_SET in AS_PATH or AS4_PATH
// draws no verdict, while with false it is treat-as-withdraw; AS 0 and a structural
// fault still draw their own verdicts with the exception on. Method: one UPDATE
// attribute block per case through ValidateUpdateRFC7606Peer, both policies.
func TestValidateUpdateAcceptASSetKeepsSetsOnly(t *testing.T) {
	wantAction := func(want, got RFC7606Action, format string, args ...any) {
		t.Helper()
		if want != got {
			t.Fatalf("action = %v, want %v: "+format, append([]any{got, want}, args...)...)
		}
	}
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	nextHop := []byte{0x40, 0x03, 0x04, 192, 0, 2, 1}
	attrs := func(code byte, value []byte) []byte {
		out := append(append([]byte(nil), origin...), 0x40, code, byte(len(value)))
		out = append(out, value...)
		return append(out, nextHop...)
	}
	set4 := []byte{2, 1, 0, 0, 0xfd, 0xea, 1, 2, 0, 0, 0xfd, 0xf2, 0, 0, 0xfd, 0xf3}
	confed4 := []byte{2, 1, 0, 0, 0xfd, 0xea, 4, 1, 0, 0, 0xfd, 0xf2}
	zeroInSet := []byte{2, 1, 0, 0, 0xfd, 0xea, 1, 1, 0, 0, 0, 0}
	overrun := []byte{2, 3, 0, 0, 0xfd, 0xea}
	for _, tc := range []struct {
		name       string
		block      []byte
		withdraw   RFC7606Action
		acceptWant RFC7606Action
	}{
		{"AS_SET in AS_PATH", attrs(2, set4), RFC7606ActionTreatAsWithdraw, RFC7606ActionNone},
		{"AS_CONFED_SET in AS_PATH", attrs(2, confed4), RFC7606ActionTreatAsWithdraw, RFC7606ActionNone},
		{"AS 0 inside a set", attrs(2, zeroInSet), RFC7606ActionTreatAsWithdraw, RFC7606ActionTreatAsWithdraw},
		{"segment overrun", attrs(2, overrun), RFC7606ActionTreatAsWithdraw, RFC7606ActionTreatAsWithdraw},
	} {
		got := ValidateUpdateRFC7606Peer(tc.block, true, false, true, nil, false, false)
		wantAction(tc.withdraw, got.Action, "%s, withdraw: %s", tc.name, got.Description)
		got = ValidateUpdateRFC7606Peer(tc.block, true, false, true, nil, false, true)
		wantAction(tc.acceptWant, got.Action, "%s, accept: %s", tc.name, got.Description)
	}

	// AS4_PATH on a 2-octet session. With the operator configuration on, a set draws no
	// verdict, and AS 0 still earns its attribute discard (RFC 7607 Section 2, RFC 6793
	// Section 6).
	asPath2 := []byte{2, 2, 0xfd, 0xea, 0x5b, 0xa0}
	as4 := func(value []byte) []byte {
		block := attrs(2, asPath2)
		block = append(block, 0xc0, 17, byte(len(value)))
		return append(block, value...)
	}
	as4Set := []byte{2, 1, 0, 0, 0xfd, 0xea, 1, 1, 0xfa, 0x56, 0xea, 0x01}
	as4Zero := []byte{2, 1, 0, 0, 0, 0}
	got := ValidateUpdateRFC7606Peer(as4(as4Set), true, false, false, nil, false, false)
	wantAction(RFC7606ActionTreatAsWithdraw, got.Action, got.Description)
	got = ValidateUpdateRFC7606Peer(as4(as4Set), true, false, false, nil, false, true)
	wantAction(RFC7606ActionNone, got.Action, got.Description)
	got = ValidateUpdateRFC7606Peer(as4(as4Zero), true, false, false, nil, false, true)
	wantAction(RFC7606ActionAttributeDiscard, got.Action, got.Description)
}
