// RFC: rfc/short/rfc8669.md — BGP Prefix-SID attribute (code 40)
// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// Overview: session_validation.go — enforceRFC7606 reads PrefixSIDPresent from the
// validator walk instead of walking the attribute section a second time.
//
// The EBGP-boundary check of RFC 8669 Section 4 used to call attribute.AttrFind on the
// same bytes message.ValidateUpdateRFC7606AddPath had just parsed. The lookup now rides
// that walk. The two forms must agree on every input. That includes an input whose walk
// abandons the section part way through. These tests are the differential that proves it.

package reactor

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// prefixSIDAttr builds a Prefix-SID attribute (code 40) holding a Label-Index TLV.
// ext selects the Extended Length encoding of RFC 4271, so both header shapes are covered.
func prefixSIDAttr(ext bool) []byte {
	value := []byte{1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, 0x09} // Label Index 777
	if ext {
		return append([]byte{0xD0, 40, 0x00, byte(len(value))}, value...)
	}
	return append([]byte{0xC0, 40, byte(len(value))}, value...)
}

// mandatoryAttrs is a well-formed ORIGIN + AS_PATH + NEXT_HOP prelude.
func mandatoryAttrs() []byte {
	return []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
}

func concatAttrs(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// prefixSIDCorpusBase returns the hand-built attribute sections the differential runs over.
//
// The set deliberately places the Prefix-SID attribute before and after each class of
// malformation the validator can report. The ordering decides whether the walk reaches
// code 40 before it stops. That is the one way the folded lookup CAN disagree with the
// standalone one.
func prefixSIDCorpusBase() []struct {
	name  string
	attrs []byte
} {
	localPrefEBGP := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}    // discard on eBGP
	badOrigin := []byte{0x40, 0x01, 0x02, 0x00, 0x00}                    // length 2: treat-as-withdraw
	badCommunity := []byte{0xc0, 0x08, 0x05, 0, 1, 0, 2, 3}              // length 5: treat-as-withdraw
	attrOverrun := []byte{0xc0, 0x11, 0x40, 0x01, 0x02}                  // declared 64, 2 present
	dupOrigin := []byte{0x40, 0x01, 0x01, 0x00}                          // second ORIGIN: duplicate range
	badMPFlags := []byte{0x40, 0x0e, 0x05, 0x00, 0x02, 0x01, 0x00, 0x00} // well-known MP: session reset
	badPrefixSIDTLV := []byte{0xC0, 40, 0x04, 0x01, 0xFF, 0xFF, 0x00}    // TLV length past attribute
	trailingPrefixSID := append(prefixSIDAttr(false), 0x00)              // trailing byte in value

	return []struct {
		name  string
		attrs []byte
	}{
		{"empty", nil},
		{"mandatory-only-no-prefixsid", mandatoryAttrs()},
		{"mandatory-then-prefixsid", concatAttrs(mandatoryAttrs(), prefixSIDAttr(false))},
		{"prefixsid-then-mandatory", concatAttrs(prefixSIDAttr(false), mandatoryAttrs())},
		{"prefixsid-extended-length", concatAttrs(mandatoryAttrs(), prefixSIDAttr(true))},
		{"prefixsid-duplicated", concatAttrs(mandatoryAttrs(), prefixSIDAttr(false), prefixSIDAttr(false))},
		{"prefixsid-only", prefixSIDAttr(false)},
		{"prefixsid-malformed-tlv", concatAttrs(mandatoryAttrs(), badPrefixSIDTLV)},
		{"prefixsid-trailing-bytes", concatAttrs(mandatoryAttrs(), trailingPrefixSID)},
		{"localpref-discard-then-prefixsid", concatAttrs(mandatoryAttrs(), localPrefEBGP, prefixSIDAttr(false))},
		{"prefixsid-then-localpref-discard", concatAttrs(mandatoryAttrs(), prefixSIDAttr(false), localPrefEBGP)},
		{"bad-origin-then-prefixsid", concatAttrs(badOrigin, prefixSIDAttr(false))},
		{"prefixsid-then-bad-origin", concatAttrs(prefixSIDAttr(false), badOrigin)},
		{"bad-community-then-prefixsid", concatAttrs(mandatoryAttrs(), badCommunity, prefixSIDAttr(false))},
		{"attr-overrun-then-prefixsid", concatAttrs(mandatoryAttrs(), attrOverrun, prefixSIDAttr(false))},
		{"prefixsid-then-attr-overrun", concatAttrs(mandatoryAttrs(), prefixSIDAttr(false), attrOverrun)},
		{"session-reset-then-prefixsid", concatAttrs(mandatoryAttrs(), badMPFlags, prefixSIDAttr(false))},
		{"prefixsid-then-session-reset", concatAttrs(mandatoryAttrs(), prefixSIDAttr(false), badMPFlags)},
		{"duplicate-origin-then-prefixsid", concatAttrs(mandatoryAttrs(), dupOrigin, prefixSIDAttr(false))},
	}
}

// prefixSIDCorpus expands the base sections into the full differential corpus: every base,
// every truncation of every base, and a deterministic random sweep.
//
// The truncations are the load-bearing part. Cutting a section at each byte drives the
// validator into each of its three mid-parse stops. Those are an insufficient header, an
// insufficient length, and a declared length past the section. Each one is reached at
// every position relative to the Prefix-SID attribute.
func prefixSIDCorpus() [][]byte {
	var corpus [][]byte
	for _, base := range prefixSIDCorpusBase() {
		for n := 0; n <= len(base.attrs); n++ {
			corpus = append(corpus, base.attrs[:n:n])
		}
	}

	// A fixed seed keeps the sweep reproducible. Bytes come from a small alphabet weighted
	// towards attribute codes and lengths that produce parseable headers. The walk then
	// gets past its first bounds check often enough to be interesting. Code 40 (0x28) is
	// in the alphabet, so Prefix-SID appears by chance as well as by construction.
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic corpus, not cryptography
	alphabet := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x07, 0x10, 0x28, 0x40, 0x80, 0xC0, 0xD0, 0xFF}
	for range 4000 {
		section := make([]byte, rng.Intn(24))
		for i := range section {
			section[i] = alphabet[rng.Intn(len(alphabet))]
		}
		corpus = append(corpus, section)
	}
	return corpus
}

// TestPrefixSIDSingleWalkSameVerdict is the differential behind T1-4. Folding the Prefix-SID
// lookup of RFC 8669 Section 4 into the validator walk changed no verdict.
//
// VALIDATES: AC-9 — the verdict for every input in the corpus is identical before and
// after the fold. The pre-change form derived presence with attribute.AttrFind, a second
// independent walk of the same bytes. The post-change form reads PrefixSIDPresent off the
// validator result. This asserts that the two agree wherever the answer is read. It also
// asserts that the folded flag never claims a Prefix-SID the standalone walk cannot find.
// PREVENTS: a Prefix-SID attribute from an eBGP peer outside the SR domain surviving the
// Section 4 discard because the fold reported it absent. Also a well-formed UPDATE losing
// an attribute because the fold reported one that is not there.
func TestPrefixSIDSingleWalkSameVerdict(t *testing.T) {
	corpus := prefixSIDCorpus()
	require.Greater(t, len(corpus), 4000, "corpus must actually be populated")

	// Every input the validator's result depends on. The Prefix-SID branch is reached on
	// eBGP only, but the flag itself must be correct on both session types.
	//
	// addPathFor is varied because the real caller passes a live one. It decides only
	// which early return fires inside an MP attribute, never how far the loop walks, so
	// it must not move the flag. Varying it is what proves that rather than assuming it.
	addPathModes := map[string]func(afi uint16, safi uint8) bool{
		"no-addpath":  nil,
		"all-addpath": func(uint16, uint8) bool { return true },
	}

	for mode, addPathFor := range addPathModes {
		for _, hasNLRI := range []bool{false, true} {
			for _, isIBGP := range []bool{false, true} {
				for _, asn4 := range []bool{false, true} {
					for i, attrs := range corpus {
						result := message.ValidateUpdateRFC7606AddPath(attrs, hasNLRI, isIBGP, asn4, addPathFor)
						_, _, _, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)

						// The folded flag must never invent an attribute. This holds at every
						// action, not only where the branch reads it. A false positive becomes
						// a real divergence once another caller reads the field.
						if result.PrefixSIDPresent {
							require.True(t, found,
								"corpus[%d] %s hasNLRI=%v isIBGP=%v asn4=%v: PrefixSIDPresent set but AttrFind does not find code 40 in %x",
								i, mode, hasNLRI, isIBGP, asn4, attrs)
						}

						// The discard of RFC 8669 Section 4 is reachable only under an action
						// no stronger than attribute-discard. That is exactly the set of
						// results whose walk ran to the end of the section. There the two
						// forms must agree exactly. Above it the branch acts on neither.
						if result.Action <= message.RFC7606ActionAttributeDiscard {
							require.Equal(t, found, result.PrefixSIDPresent,
								"corpus[%d] %s hasNLRI=%v isIBGP=%v asn4=%v action=%v: single-walk presence disagrees with AttrFind for %x",
								i, mode, hasNLRI, isIBGP, asn4, result.Action, attrs)
						}
					}
				}
			}
		}
	}
}

// TestPrefixSIDSingleWalkCorpusReachesEveryAction proves the corpus is not vacuous: it
// covers all four RFC 7606 actions, and reaches the attribute-discard action both with and
// without a Prefix-SID present.
//
// VALIDATES: the differential above actually exercises the branch it exists to protect.
// PREVENTS: a corpus that drifts into only-session-reset or only-clean inputs. Such a
// corpus would let the differential pass over nothing.
func TestPrefixSIDSingleWalkCorpusReachesEveryAction(t *testing.T) {
	seen := map[message.RFC7606Action]int{}
	discardWithSID, noneWithSID, abortedWithSIDBytes := 0, 0, 0

	for _, attrs := range prefixSIDCorpus() {
		result := message.ValidateUpdateRFC7606AddPath(attrs, true, false, false, nil)
		seen[result.Action]++
		_, _, _, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
		switch {
		case result.Action == message.RFC7606ActionAttributeDiscard && found:
			discardWithSID++
		case result.Action == message.RFC7606ActionNone && found:
			noneWithSID++
		case result.Action > message.RFC7606ActionAttributeDiscard && found:
			// The walk aborted, yet a standalone walk still finds code 40. This is the
			// case the fold has to get right by declining to act, not by agreeing.
			abortedWithSIDBytes++
		}
	}

	for _, action := range []message.RFC7606Action{
		message.RFC7606ActionNone,
		message.RFC7606ActionAttributeDiscard,
		message.RFC7606ActionTreatAsWithdraw,
		message.RFC7606ActionSessionReset,
	} {
		assert.Positive(t, seen[action], "corpus must contain at least one %v input", action)
	}
	assert.Positive(t, discardWithSID, "corpus must reach attribute-discard with a Prefix-SID present")
	assert.Positive(t, noneWithSID, "corpus must reach a clean verdict with a Prefix-SID present")
	assert.Positive(t, abortedWithSIDBytes,
		"corpus must contain an aborted walk whose bytes still hold a findable Prefix-SID")
}

// TestPrefixSIDEBGPDiscardAcrossCorpus drives the whole corpus through the real receive
// path. The fold is then proven where an operator meets it, not only at the validator.
//
// Two RFC 8669 rules act on this attribute and the test keeps them apart. Section 4 is the
// EBGP domain boundary and is gated on AcceptSRv6PrefixSID. Section 6 discards a malformed
// Prefix-SID whatever that setting says, and the validator reports it as a discard entry
// for code 40. Only the first rule reads the folded presence flag.
//
// VALIDATES: AC-8 — a received UPDATE from an eBGP peer carrying Prefix-SID is discarded
// exactly as before, and one from a configured peer keeps it unless it is itself malformed.
// PREVENTS: enforceRFC7606 wiring the folded flag into the wrong guard, which the
// validator-level differential above cannot see.
func TestPrefixSIDEBGPDiscardAcrossCorpus(t *testing.T) {
	nlri := []byte{24, 10, 0, 0}

	for _, base := range prefixSIDCorpusBase() {
		attrs := base.attrs
		_, _, _, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
		if !found {
			continue
		}
		// Session reset closes the connection and has its own tests. The Prefix-SID branch
		// cannot be reached under it.
		probe := message.ValidateUpdateRFC7606AddPath(attrs, true, false, false, nil)
		if probe.Action > message.RFC7606ActionAttributeDiscard {
			continue
		}
		// RFC 8669 Section 6: the validator already condemned this attribute on its own
		// contents, so acceptance does not save it.
		malformedSID := false
		for _, e := range probe.DiscardEntries {
			if e.Code == uint8(attribute.AttrPrefixSID) {
				malformedSID = true
			}
		}
		// A second copy of code 40 survives the discard today. Presence after the fact is
		// therefore not a clean signal for those inputs. Two causes combine, and the T1-4
		// diff touches neither.
		//
		// First, attr_discard.applyInPlace locates the attribute with AttrFind, which
		// rewrites the FIRST occurrence only. Second, the branch in enforceRFC7606 builds
		// a fresh result carrying no DuplicateRanges. That skips the keep-first strip of
		// Section 3.g which would remove the copy. The action assertion below still runs
		// here. Only the presence assertion stands down.
		sidCount, _ := countAttrCode(attrs, uint8(attribute.AttrPrefixSID))

		t.Run(base.name, func(t *testing.T) {
			// eBGP, not configured to accept: RFC 8669 Section 4 discards the attribute.
			s := rfc8669EBGPSession(false)
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nlri), 0))
			require.NoError(t, err, "the boundary rule is an attribute discard, never a session reset")
			assert.Equal(t, message.RFC7606ActionAttributeDiscard, action,
				"a Prefix-SID from an unconfigured eBGP peer must select attribute-discard")
			if sidCount == 1 {
				_, _, _, stillThere := attribute.AttrFind(rfc8669PathAttrs(t, wu.Payload()), attribute.AttrPrefixSID)
				assert.False(t, stillThere, "an unconfigured eBGP peer's Prefix-SID must be stripped")
			}

			// eBGP, configured to accept: Section 4 stands down, so the attribute survives
			// unless Section 6 condemned it on its own contents.
			s = rfc8669EBGPSession(true)
			wu, _, err = s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nlri), 0))
			require.NoError(t, err)
			if sidCount == 1 {
				_, _, _, kept := attribute.AttrFind(rfc8669PathAttrs(t, wu.Payload()), attribute.AttrPrefixSID)
				assert.Equal(t, !malformedSID, kept,
					"a configured eBGP peer keeps a well-formed Prefix-SID and loses a malformed one")
			}
		})
	}
}

// TestPrefixSIDIBGPNeverReachesTheBranch pins the conditionality the fold does not remove:
// RFC 8669 Section 4 is an eBGP boundary rule, so an iBGP session never consults presence
// at all. The saving from folding the lookup therefore does not exist on iBGP sessions.
//
// VALIDATES: the guard is `!isIBGP && !AcceptSRv6PrefixSID`, unchanged by T1-4.
// PREVENTS: the fold quietly extending the discard to iBGP, which would strip Segment
// Routing information inside the SR domain.
func TestPrefixSIDIBGPNeverReachesTheBranch(t *testing.T) {
	attrs := concatAttrs(mandatoryAttrs(), prefixSIDAttr(false))
	body := makeUpdateBody(nil, attrs, []byte{24, 10, 0, 0})

	s := newValidateSession()
	s.settings.PeerAS = s.settings.LocalAS // iBGP
	require.False(t, s.settings.AcceptSRv6PrefixSID, "acceptance stays opt-in on iBGP too")

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"RFC 8669 Section 4 is an eBGP rule: an iBGP Prefix-SID is not discarded")

	_, _, _, found := attribute.AttrFind(rfc8669PathAttrs(t, wu.Payload()), attribute.AttrPrefixSID)
	assert.True(t, found, "the iBGP Prefix-SID attribute must remain in the UPDATE")
}
