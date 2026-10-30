// VALIDATES: the heading form of {feature-declined}, `heading "<feature
// words>"; ze never <uses the feature>: <path>.go::<Symbol> <how>`. The words
// must be whole words of the title of the row's section or of a section above
// it, never the RFC's own title, never generic heading words or under three
// characters, and the reason carries a never clause on those words naming a
// producer the tree declares outside a test.
// PREVENTS: a feature declined by a heading that does not name it, by a word
// every RFC titles a section with, or by the document's own name, which would
// take a whole document out of the count; and a heading form with no evidence
// that Ze never uses the feature.
// METHOD: rows parsed by the real parser and judged by the exclusion evidence
// ratchet's gap, over the real texts of RFC 8907 and RFC 9915 copied into a
// fixture checkout beside a producer file.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	headingChapRow = "The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response."
	headingLLTRow  = "The hardware type MUST be a valid hardware type assigned by IANA; see [IANA-HARDWARE-TYPES]."
	headingNever   = "ze never uses CHAP: internal/x/frame.go::sendFrame sends PAP only"
)

// headingFixtureTree copies the real texts of RFC 8907 and RFC 9915 into a
// temporary checkout beside the producer file every case names.
func headingFixtureTree(t *testing.T) string {
	t.Helper()
	files := map[string]string{conditionalProducerFile: "package x\n\nfunc sendFrame() {}\n"}
	for _, stem := range []string{"rfc8907", "rfc9915"} {
		text, err := os.ReadFile(filepath.Join("..", "..", "..", "rfc", "full", stem+".txt"))
		if err != nil {
			t.Fatalf("read the %s text: %v", stem, err)
		}
		files["rfc/full/"+stem+".txt"] = string(text)
	}
	tree := t.TempDir()
	writeFixtureFiles(t, tree, files)
	return tree
}

// headingCase is one heading-form row: its RFC and section, its own
// sentence, the feature words, the reason, and what a refusal says (accepted
// when empty).
type headingCase struct {
	name, stem, cites, rowText, words, reason string
	says                                      []string
}

func TestAFeatureDeclinedHeadingNamesTheFeatureItScopes(t *testing.T) {
	tree := headingFixtureTree(t)
	cases := []headingCase{
		{name: "RFC 8907 §5.4.2.3 CHAP Login", stem: "rfc8907", cites: "5.4.2.3", rowText: headingChapRow,
			words: "CHAP", reason: headingNever},
		{name: "RFC 9915 §11.2 DUID-LLT", stem: "rfc9915", cites: "11.2", rowText: headingLLTRow,
			words: "DUID-LLT", reason: "ze never generates a DUID-LLT: internal/x/frame.go::sendFrame builds a DUID-LL"},
		{name: "a heading of a section above the row's", stem: "rfc9915", cites: "11.2",
			rowText: headingLLTRow, words: "DUID", reason: "ze never generates a DUID: internal/x/frame.go::sendFrame sends frames",
		},
		{name: "words no title above the section holds", stem: "rfc8907", cites: "5.4.2.3", rowText: headingChapRow,
			words: "PAP", reason: "ze never uses PAP: internal/x/frame.go::sendFrame sends frames",
			says: []string{"'PAP' is in no title of section 5.4.2.3", "'CHAP Login'", "'Authentication'"}},
		{name: "a word inside a hyphenated title word", stem: "rfc8907", cites: "5.4.2.4", rowText: headingChapRow,
			words: "CHAP", reason: headingNever,
			says: []string{"'CHAP' is in no title of section 5.4.2.4", "'MS-CHAP v1 Login'"}},
		{name: "a cited section no heading answers", stem: "rfc8907", cites: "99", rowText: headingChapRow,
			words: "CHAP", reason: headingNever,
			says: []string{"the section it cites, 99, names no heading"}},
		{name: "words of the RFC's own title", stem: "rfc8907", cites: "5.4.2.3", rowText: headingChapRow,
			words: "TACACS+", reason: "ze never uses TACACS+: internal/x/frame.go::sendFrame sends frames",
			says: []string{"is in the RFC's own title", "the whole document is not one feature"}},
		{name: "no never clause", stem: "rfc8907", cites: "5.4.2.3", rowText: headingChapRow,
			words: "CHAP", reason: "internal/x/frame.go::sendFrame sends PAP only",
			says: []string{"its heading form declines the feature 'CHAP'", "no never clause", neverClauseForm}},
		{name: "a never clause on another condition", stem: "rfc8907", cites: "5.4.2.3", rowText: headingChapRow,
			words: "CHAP", reason: "ze never compresses a frame: internal/x/frame.go::sendFrame sends PAP only",
			says: []string{"shares no word with the heading's feature words 'CHAP'"}},
		{name: "a never clause naming a test", stem: "rfc8907", cites: "5.4.2.3", rowText: headingChapRow,
			words: "CHAP", reason: "internal/x/frame.go::sendFrame sends PAP; ze never uses CHAP: internal/x/frame_test.go::TestSendFrame checks it",
			says: []string{"names the test internal/x/frame_test.go::TestSendFrame"}},
		{name: "a never clause naming a function its file does not declare", stem: "rfc8907", cites: "5.4.2.3",
			rowText: headingChapRow, words: "CHAP", reason: "ze never uses CHAP: internal/x/frame.go::chapLogin is absent",
			says: []string{"declares no chapLogin"}},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			row := conditionalRow(t, one.stem, one.cites, one.rowText,
				`feature-declined: heading "`+one.words+`"; `+one.reason)
			if row.Annotation.Heading != one.words || row.Annotation.Quote != "" {
				t.Fatalf("the heading form parsed to heading %q and quote %q", row.Annotation.Heading, row.Annotation.Quote)
			}
			missing, lacks := newEvidenceReader(tree).gap(&row)
			if len(one.says) == 0 {
				if lacks {
					t.Fatalf("heading %q under §%s of %s was refused: %s", one.words, one.cites, one.stem, missing)
				}
				return
			}
			if !lacks {
				t.Fatalf("heading %q under §%s of %s was accepted", one.words, one.cites, one.stem)
			}
			for _, want := range one.says {
				if !strings.Contains(missing, want) {
					t.Errorf("the refusal %q does not say %q", missing, want)
				}
			}
		})
	}
}

// VALIDATES: the parser refuses a heading form whose words are too short or
// only generic, whose words are not quoted, or whose producer is a test.
func TestAFeatureDeclinedHeadingIsRefusedAtParse(t *testing.T) {
	cases := []struct {
		name, payload string
		says          []string
	}{
		{name: "a generic heading", payload: `feature-declined: heading "Security Considerations"; ` + headingNever,
			says: []string{"only stopwords or words any heading uses"}},
		{name: "a generic heading and a stopword", payload: `feature-declined: heading "Message Format of the Procedures"; ` + headingNever,
			says: []string{"only stopwords or words any heading uses"}},
		{name: "two characters", payload: `feature-declined: heading "ID"; ` + headingNever,
			says: []string{"under 3 characters"}},
		{name: "unquoted words", payload: `feature-declined: heading CHAP; ` + headingNever,
			says: []string{"heading needs the feature words in double quotes"}},
		{name: "a test producer", payload: `feature-declined: heading "CHAP"; ze never uses CHAP: internal/x/frame_test.go::TestSendFrame checks it`,
			says: []string{"heading names the test internal/x/frame_test.go::TestSendFrame"}},
		{name: "no producer", payload: `feature-declined: heading "CHAP"; ze never uses CHAP`,
			says: []string{"heading names no producer"}},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := parseAnnotation(one.payload, "fixture")
			if err == nil {
				t.Fatalf("{%s} parsed", one.payload)
			}
			for _, want := range one.says {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not say %q", err, want)
				}
			}
		})
	}
}

// VALIDATES: the RFC's own title, and a section's title with its ancestors',
// are read from the real text.
func TestTheHeadingTitlesOfARealRFC(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "..", "rfc", "full", "rfc8907.txt"))
	if err != nil {
		t.Fatalf("read the RFC 8907 text: %v", err)
	}
	source := newQuoteSource(string(text))
	if want := "The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol"; source.documentTitle != want {
		t.Errorf("the title is %q, want %q", source.documentTitle, want)
	}
	got := strings.Join(source.headingTitles("5.4.2.3"), " | ")
	if want := "CHAP Login | Common Authentication Flows | Description of Authentication Process | Authentication"; got != want {
		t.Errorf("the titles of 5.4.2.3 and above are %q, want %q", got, want)
	}
}

// VALIDATES: checkFeatureDeclined owes a heading form its RFC's text and
// passes one whose text and producer are present, without placing a quote it
// does not carry.
func TestCheckFeatureDeclinedOwesAHeadingItsText(t *testing.T) {
	tree := headingFixtureTree(t)
	present := conditionalRow(t, "rfc8907", "5.4.2.3", headingChapRow, `feature-declined: heading "CHAP"; `+headingNever)
	absent := conditionalRow(t, "rfc8906", "5.4.2.3", headingChapRow, `feature-declined: heading "CHAP"; `+headingNever)
	errs := checkFeatureDeclined(tree, newSourceReader(tree), []Requirement{present, absent})
	if len(errs) != 1 {
		t.Fatalf("want one refusal, for the RFC with no text, got %d: %q", len(errs), errs)
	}
	if !strings.Contains(errs[0], "its heading can be checked against nothing") || !strings.Contains(errs[0], "rfc8906") {
		t.Errorf("the refusal %q does not name the absent text", errs[0])
	}
}
