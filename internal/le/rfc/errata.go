// Design: docs/contributing/rfc-conformance-gates.md -- the row quote, a row that cites an erratum
// Related: check_quote.go -- rowQuoteRefusal, which judges a row against the RFC as its errata correct it
// Related: inventory.go -- quoteSource and quoteHaystack, the matcher the erratum text is read into
package rfc

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errataRel holds one directory per stem, and in it one file per verified
// erratum the stem's rows cite, named <number>.txt (owner decision D-11,
// 2026-09-27).
const errataRel = "rfc/errata"

// erratumCiteRE reads one erratum citation: "erratum 8301", "errata 543" or
// "Errata ID 7840", the three forms the corpus writes.
var erratumCiteRE = regexp.MustCompile(`(?i)\berrat(?:um|a)\s+(?:ID\s+)?(\d+)`)

// The headings that open the two text blocks of an erratum file, each alone on
// its line.
const (
	erratumOriginalHeading  = "Original Text:"
	erratumCorrectedHeading = "Corrected Text:"
)

// erratum is one verified erratum of an RFC, its two texts already in
// quoteHaystack form.
type erratum struct {
	number string
	// section is the section the erratum's Location names, "" when it names
	// none. The original text is looked for there and nowhere else, because the
	// same sentence can stand in two sections and an erratum corrects one.
	section   string
	original  string
	corrected string
}

// citedErrata answers the erratum numbers a row's trailing parenthetical cites,
// in order and each once. The citation lives beside the section the row cites,
// so an erratum named in the row's own words is not a citation.
func citedErrata(text string) []string {
	tail := trailingParenRE.FindStringSubmatch(text)
	if tail == nil {
		return nil
	}
	var numbers []string
	for _, found := range erratumCiteRE.FindAllStringSubmatch(tail[1], -1) {
		if !slices.Contains(numbers, found[1]) {
			numbers = append(numbers, found[1])
		}
	}
	return numbers
}

// erratumPath answers where the repository stores one erratum of a stem.
func erratumPath(stem, number string) string {
	return stemPath(errataRel+"/"+stem, number, ".txt")
}

// treeErrata answers a reader of the errata the working tree stores for one
// stem. A file that cannot be read answers false, the same as a file that is
// not there: either way the row's citation has nothing to be checked against.
func treeErrata(tree, stem string) func(string) (string, bool) {
	return func(number string) (string, bool) {
		data, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(erratumPath(stem, number)))) // #nosec G304 -- a path under the checkout this gate judges
		if err != nil {
			return "", false
		}
		return string(data), true
	}
}

// blobErrata answers a reader of the errata one revision's blobs hold for one
// stem.
func blobErrata(blobs map[string]string, stem string) func(string) (string, bool) {
	return func(number string) (string, bool) {
		text, held := blobs[erratumPath(stem, number)]
		return text, held
	}
}

// parseErratum reads one stored erratum. It refuses a file that is not the
// erratum it is named after, not of this stem's RFC, not verified, or missing
// either text: an erratum that is only reported is a claim by its reporter,
// and the row check takes only the text the stream's approver verified.
func parseErratum(stem, number, text string) (erratum, error) {
	header, texts, found := strings.Cut(text, "\n"+erratumOriginalHeading+"\n")
	if !found {
		return erratum{}, errors.New("it has no line " + pyRepr(erratumOriginalHeading))
	}
	original, corrected, found := strings.Cut(texts, "\n"+erratumCorrectedHeading+"\n")
	if !found {
		return erratum{}, errors.New("it has no line " + pyRepr(erratumCorrectedHeading))
	}
	fields := erratumFields(header)
	if fields["Errata ID"] != number {
		return erratum{}, errors.New("its Errata ID is " + pyRepr(fields["Errata ID"]) + ", not " + number)
	}
	if "rfc"+fields["RFC"] != stem {
		return erratum{}, errors.New("its RFC is " + pyRepr(fields["RFC"]) + ", not the RFC of " + stem)
	}
	if fields["Status"] != "Verified" {
		return erratum{}, errors.New("its Status is " + pyRepr(fields["Status"]) + ". Only a Verified erratum is evidence")
	}
	e := erratum{number: number, original: quoteHaystack(original), corrected: quoteHaystack(corrected)}
	if e.original == "" {
		return erratum{}, errors.New("its original text is empty")
	}
	if e.corrected == "" {
		return erratum{}, errors.New("its corrected text is empty")
	}
	if section, cut := strings.CutPrefix(fields["Location"], "Section "); cut {
		e.section = section
	}
	return e, nil
}

// erratumFields reads the "Key: value" lines of an erratum file's header.
func erratumFields(header string) map[string]string {
	fields := map[string]string{}
	for line := range strings.SplitSeq(header, "\n") {
		key, value, found := strings.Cut(line, ": ")
		if found {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return fields
}

// withErrata gives the source the reader of its RFC's stored errata and
// answers it. It MUST be called before the source is shared, since nothing
// writes to a source after that. A source without a reader holds no erratum,
// so a row that cites one is refused.
func (q *quoteSource) withErrata(read func(string) (string, bool)) *quoteSource {
	q.readErratum = read
	return q
}

// loadErrata reads each cited erratum. It answers the refusal that names the
// file when one is absent or unusable, and "" when every one loaded.
func (q *quoteSource) loadErrata(stem string, numbers []string) ([]erratum, string) {
	errata := make([]erratum, 0, len(numbers))
	for _, number := range numbers {
		var tb textbuf.Buffer
		tb.Str("the row cites erratum ").Str(number).Str(", and ").Str(erratumPath(stem, number))
		if q.readErratum == nil {
			return nil, tb.Str(" cannot be read here. Store the verified text from https://www.rfc-editor.org/errata/eid").
				Str(number).Str(" there").String()
		}
		text, held := q.readErratum(number)
		if !held {
			return nil, tb.Str(" is not in this repository. Store the erratum's Original Text and Corrected Text there, as verified at https://www.rfc-editor.org/errata/eid").
				Str(number).String()
		}
		e, err := parseErratum(stem, number, text)
		if err != nil {
			return nil, tb.Str(" is not usable: ").Str(err.Error()).String()
		}
		errata = append(errata, e)
	}
	return errata, ""
}

// corrected answers the RFC as the errata correct it: in the section each
// erratum names, and its subsections, the erratum's original text is replaced
// by its corrected text. It also answers each erratum whose original text is
// not verbatim there. For such an erratum nobody can tell which published
// sentence it replaced, so the caller MUST NOT accept a published sentence
// while one is unplaced.
func (q *quoteSource) corrected(errata []erratum) (*quoteSource, []erratum) {
	sections := slices.Clone(q.sections)
	var unplaced []erratum
	for i := range errata {
		e := &errata[i]
		placed := false
		for j := range sections {
			if !inSectionTree(sections[j].id, e.section) {
				continue
			}
			if strings.Contains(sections[j].body, e.original) {
				sections[j].body = strings.ReplaceAll(sections[j].body, e.original, e.corrected)
				placed = true
			}
		}
		if !placed {
			unplaced = append(unplaced, *e)
		}
	}
	return &quoteSource{sections: sections, readErratum: q.readErratum}, unplaced
}

// inSectionTree answers whether id is the section root or one of its
// subsections. An empty root names no section and holds none.
func inSectionTree(id, root string) bool {
	if root == "" {
		return false
	}
	return id == root || strings.HasPrefix(id, root+".")
}

// erratumCarries answers whether quote is one contiguous span of the corrected
// text of one of the errata.
func erratumCarries(errata []erratum, quote string) bool {
	needle := quoteNeedle(quote)
	for i := range errata {
		if strings.Contains(errata[i].corrected, needle) {
			return true
		}
	}
	return false
}

// erratumRowRefusal judges a row that cites errata against the RFC as those
// errata correct it (owner decision D-11, 2026-09-27). The quote passes when it
// is a verbatim span of one erratum's corrected text, or of the cited section
// once every cited erratum is applied. It never passes as a span joining an
// erratum's text to the section around it: after the correction that text is
// in the section, and before it, the two were never one sentence.
func erratumRowRefusal(tb *textbuf.Buffer, req *Requirement, source *quoteSource, resolved, quote string, cited []string) (string, bool) {
	errata, problem := source.loadErrata(req.RFC, cited)
	if problem != "" {
		return tb.Str(problem).String(), true
	}
	if erratumCarries(errata, quote) {
		return "", false
	}
	corrected, unplaced := source.corrected(errata)
	if len(unplaced) == 0 && corrected.inSection(resolved, quote) {
		return "", false
	}
	if source.inSection(resolved, quote) {
		tb.Str("the sentence is verbatim in section ").Str(resolved).Str(" as published")
		if len(unplaced) > 0 {
			return writeUnplaced(tb, unplaced).Str(". So the check cannot tell whether the erratum replaced the sentence. Quote the erratum's corrected text").String(), true
		}
		return tb.Str(", and ").Str(errataNames(errata)).Str(" replaced it. Quote the text as the erratum corrects it").String(), true
	}
	if found, elsewhere := corrected.sectionOf(quote); elsewhere {
		return tb.Str("the sentence is in section ").Str(found).Str(", not in section ").Str(resolved).
			Str(" or its subsections. Cite the section it is in").String(), true
	}
	return tb.Str("the row's text is not a verbatim span of the corrected text of ").Str(errataNames(errata)).
		Str(", nor of section ").Str(resolved).Str(" as they correct it. A row states the RFC's own sentence, copied verbatim, never a paraphrase").String(), true
}

// writeUnplaced writes why each unplaced erratum could not be applied.
func writeUnplaced(tb *textbuf.Buffer, unplaced []erratum) *textbuf.Buffer {
	for i := range unplaced {
		e := &unplaced[i]
		if e.section == "" {
			tb.Str(", but erratum ").Str(e.number).Str(" names no section in its Location")
			continue
		}
		tb.Str(", but the original text of erratum ").Str(e.number).Str(" is not verbatim in section ").Str(e.section)
	}
	return tb
}

// errataNames answers "erratum 8301" or "erratum 8299, erratum 8300".
func errataNames(errata []erratum) string {
	var tb textbuf.Buffer
	for i := range errata {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str("erratum ").Str(errata[i].number)
	}
	return tb.String()
}
