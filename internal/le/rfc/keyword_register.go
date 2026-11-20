// Design: docs/contributing/rfc-conformance-gates.md -- the {not-applicable} shapes, and the lowercase keyword register
// Related: notapplicable_roles.go -- boundRoleRefusal, the parse-time reader of a quote's first keyword
// Related: check_evidence.go -- wordIndex, the whole-word matcher, and the evidence-time readers
// Related: check_conditional.go -- conditionalGuard, which reads the first mandatory keyword
// Related: inventory.go -- registerProse, the extraction register the lowercase reading starts from
package rfc

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// KeywordRegister is how an RFC's text spells the BCP 14 keywords its
// obligations are stated in, so a quote is read for a keyword the way its own
// document writes one. Zero is no register: a caller that has not asked the
// stem cannot say which spellings count, and every method refuses it.
type KeywordRegister uint8

const (
	registerUnspecified KeywordRegister = iota
	// registerUpperCase is a document that writes its obligations in the BCP
	// 14 keywords, and every draft: only the upper-case spelling is a keyword.
	registerUpperCase
	// registerLowercase is a document that states its obligations in
	// lowercase: an RFC published before RFC 2119, or a later RFC that never
	// adopted the BCP 14 keywords (KeywordRegisterOf). Its "must" is read as
	// MUST and its "must not" as MUST NOT, everywhere a keyword is looked for
	// (ai/META-DECISIONS.md), beside the upper-case spellings a few such RFCs
	// already use.
	registerLowercase
)

// bcp14RFCNumber is the number of RFC 2119, the document that defined the
// BCP 14 keywords.
const bcp14RFCNumber = 2119

// pre2119Spellings are the lowercase words a lowercase-register document
// states an obligation in, keyed by the level each is read as. The owner rule
// names these two and no other, so a lowercase "should" or "may" stays prose
// and is never read as SHOULD or MAY.
var pre2119Spellings = map[string]string{levelMust: "must", levelMustNot: "must not"}

// bcp14AdoptionRE finds a document's adoption of the BCP 14 keywords: the
// boilerplate that says its key words are to be interpreted as described in
// RFC 2119 (and RFC 8174, which BCP 14 also holds) names RFC 2119 or BCP 14,
// in either spacing and across a line break.
var bcp14AdoptionRE = regexp.MustCompile(`\bRFC\s*2119\b|\bBCP\s*14\b`)

// KeywordRegisterOf answers the register the document stem names is read in,
// from what the checkout under tree declares of it. The objective is the owner
// rule's (ai/META-DECISIONS.md): a document written in lowercase obligation
// language has its "must" read as MUST, whatever its number. Two declared
// facts answer it, and no list of stems is kept:
//
//   - An RFC numbered below 2119 is lowercase (registerByNumber), with or
//     without an extraction: it was published before the keywords existed.
//   - A later RFC is lowercase when its extraction sign-off
//     (rfc/extraction/<stem>.json) declares register prose, the walk that
//     read its obligations from lowercase modals, and its own text never
//     adopts BCP 14 (bcp14AdoptionRE). RFC 2181 is the case: its Section 3
//     says it "does not use the oft used expressions MUST, SHOULD, MAY".
//
// The prose register alone does not decide it. A walk also signs prose when
// a document's capitalised sentences are fewer than its gated rows, and such
// a document (RFC 4035) adopts BCP 14, under which RFC 8174 gives a lowercase
// "must" no keyword meaning. A draft keeps the upper-case register, and so
// does an RFC from 2119 on with no extraction: nothing declares how it
// writes its obligations. An extraction that cannot be read, and a prose one
// whose RFC text is missing, are errors, because the register cannot be
// answered from either.
func KeywordRegisterOf(tree, stem string) (KeywordRegister, error) {
	byNumber := registerByNumber(stem)
	if byNumber == registerLowercase {
		return byNumber, nil
	}
	if !numberedRFC(stem) {
		return byNumber, nil
	}
	signed, found, err := extractionRegisterOf(tree, stem)
	if err != nil {
		return registerUnspecified, err
	}
	if !found {
		return byNumber, nil
	}
	if signed != registerProse {
		return byNumber, nil
	}
	text, held := SourceText(tree, stem)
	if !held {
		var tb textbuf.Buffer
		return registerUnspecified, errors.New(tb.Str(extractionRel).Byte('/').Str(stem).
			Str(".json signs register prose, but the RFC text is not in ").Str(fullRel).
			Str(", so whether the document adopts BCP 14 cannot be read. Fetch it to ").
			Str(fullRel).Byte('/').Str(stem).Str(".txt").String())
	}
	if bcp14AdoptionRE.MatchString(text) {
		return byNumber, nil
	}
	return registerLowercase, nil
}

// extractionRegisterOf answers the register the extraction sign-off of stem
// declares, and false when tree holds no extraction for it. It reads that one
// field: a sign-off whose other fields fail their own gate still declares
// its register, and checkExtractions refuses the rest by name.
func extractionRegisterOf(tree, stem string) (string, bool, error) {
	var name textbuf.Buffer
	rel := name.Str(extractionRel).Byte('/').Str(stem).Str(".json").String()
	raw, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- a path under the checkout this gate judges
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	var tb textbuf.Buffer
	if err != nil {
		return "", false, errors.New(tb.Str(rel).Str(": cannot read: ").Err(err).String())
	}
	var declared struct {
		Register string `json:"register"`
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		return "", false, errors.New(tb.Str(rel).Str(": is not JSON, so its register cannot be read: ").Err(err).String())
	}
	if !knownRegister(declared.Register) {
		return "", false, errors.New(tb.Str(rel).Str(": register ").Str(pyRepr(declared.Register)).
			Str(" is missing or unknown; expected one of ").Str(pyRepr(Registers())).String())
	}
	return declared.Register, true, nil
}

// numberedRFC answers whether stem is rfc and digits alone, the stem of a
// published RFC rather than of a draft or another named document.
func numberedRFC(stem string) bool {
	digits, numbered := strings.CutPrefix(stem, "rfc")
	if !numbered || digits == "" {
		return false
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// registerByNumber answers the register the RFC number stem carries decides
// alone: the RFC Editor numbers RFCs in the order it publishes them, so a
// number below 2119 is a document published before RFC 2119 defined the
// keywords, and lowercase. Every other stem answers upper case here, which
// KeywordRegisterOf refines from the extraction for a later RFC; a stem that
// is not rfc and digits alone is a draft or another named document, and every
// one of those postdates RFC 2119.
func registerByNumber(stem string) KeywordRegister {
	if !numberedRFC(stem) {
		return registerUpperCase
	}
	digits := strings.TrimPrefix(stem, "rfc")
	number := 0
	for _, digit := range digits {
		// Past 2119 the answer is decided, and stopping there keeps a stem
		// of more digits than an int holds from overflowing.
		if number > bcp14RFCNumber {
			return registerUpperCase
		}
		number = number*10 + int(digit-'0')
	}
	if number < bcp14RFCNumber {
		return registerLowercase
	}
	return registerUpperCase
}

// find answers the byte offset of the first whole-word spelling of level in
// text that this register reads as that level, -1 when none is. Every
// spelling of one level has the same length, so a caller that steps past a
// keyword by len(level) steps past the spelling it found.
func (r KeywordRegister) find(text, level string) int {
	at := wordIndex(text, level)
	switch r {
	case registerUpperCase:
		return at
	case registerLowercase:
		lower, read := pre2119Spellings[level]
		if !read {
			return at
		}
		found := wordIndex(text, lower)
		if found < 0 {
			return at
		}
		if at < 0 {
			return found
		}
		return min(at, found)
	case registerUnspecified:
		panic("BUG: KeywordRegister.find on an unspecified register: ask KeywordRegisterOf for the stem")
	}
	panic("BUG: KeywordRegister.find on an unknown register")
}

// firstKeyword answers the byte offset of the first BCP 14 keyword in quote
// as a whole word, and the level it is read as, -1 when it holds none. Where
// two keywords start at one offset the longer wins, so "MUST NOT" is not read
// as "MUST", nor "must not" as MUST.
func (r KeywordRegister) firstKeyword(quote string) (int, string) {
	at, keyword := -1, ""
	for _, candidate := range bcp14Keywords {
		found := r.find(quote, candidate)
		if found < 0 {
			continue
		}
		if at < 0 || found < at || (found == at && len(candidate) > len(keyword)) {
			at, keyword = found, candidate
		}
	}
	return at, keyword
}

// firstMandatoryKeyword answers the byte offset of the first mandatory BCP 14
// keyword in quote as a whole word, -1 when it holds none.
func (r KeywordRegister) firstMandatoryKeyword(quote string) int {
	at := -1
	for _, level := range gatedLevelNames() {
		found := r.find(quote, level)
		if found < 0 {
			continue
		}
		if at < 0 || found < at {
			at = found
		}
	}
	return at
}

// levelsIn answers the advisory and the mandatory BCP 14 levels quote holds
// as whole words, each in rfc.go's sorted order.
func (r KeywordRegister) levelsIn(quote string) ([]string, []string) {
	var advisory, mandatory []string
	for _, level := range advisoryLevelNames() {
		if r.find(quote, level) >= 0 {
			advisory = append(advisory, level)
		}
	}
	for _, level := range gatedLevelNames() {
		if r.find(quote, level) >= 0 {
			mandatory = append(mandatory, level)
		}
	}
	return advisory, mandatory
}
