// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Related: summary.go -- parseNotApplicable, parseLowerLayer, parseFeatureDeclined: the payloads it judges
// Related: check_quote.go -- the row quote rule, whose section matcher it shares
// Related: check_baseline.go -- baselineLevels, the HEAD^ fingerprints it compares
package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// evidenceBacklogRel is the committed count of rows, per kind, that still
// lack the evidence their exclusion kind owes. The count may only fall, and a
// kind whose count is zero is refused row by row.
const evidenceBacklogRel = "rfc/exclusion-evidence-backlog.txt"

// evidenceKinds are the exclusion kinds whose evidence this ratchet judges, in
// the order a refusal lists them. {linux-limit} is not one: checkLinuxLimit
// already holds its register entry, which is its whole evidence.
var evidenceKinds = []string{AnnotationFeatureDeclined, AnnotationLowerLayer, AnnotationNotApplicable}

// kernelSourceTops are the top-level directories of a Linux source tree. The
// kernel source is not in this repository, so a `linux:` layer path is held
// to the tree's shape rather than to a file: the path must open under one of
// these in the kernel internal/appliance/kernel.version names.
var kernelSourceTops = map[string]bool{
	"arch": true, "block": true, "certs": true, "crypto": true, "drivers": true,
	"fs": true, "include": true, "init": true, "io_uring": true, "ipc": true,
	"kernel": true, "lib": true, "mm": true, "net": true, "rust": true,
	"security": true, "sound": true, "virt": true,
}

// evidenceFingerprint is what a row's exclusion evidence depends on in its own
// summary line: the section it cites, its kind and its whole reason. A row
// whose fingerprint differs from HEAD^ is a row the commit changed. Empty for
// a row no evidence kind annotates.
func evidenceFingerprint(req *Requirement) string {
	if req.Annotation == nil {
		return ""
	}
	if !slices.Contains(evidenceKinds, req.Annotation.Kind) {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(req.Section).Byte(0).Str(req.Annotation.Kind).Byte(0).Str(req.Annotation.Reason).String()
}

// quotePlace is where an annotation's quote sits in its RFC, relative to the
// section the row cites.
type quotePlace int

const (
	quoteInSection quotePlace = iota + 1
	quoteOutsideSection
	quoteNotInRFC
	quoteNoText
	quoteSectionUnresolved
)

// loadQuoteSource answers one RFC's text cut into sections, read at most once
// per stem through sources. Nil when the repository holds no text for it.
func loadQuoteSource(sources map[string]*quoteSource, tree, stem string) *quoteSource {
	source, loaded := sources[stem]
	if loaded {
		return source
	}
	if text, found := SourceText(tree, stem); found {
		source = newQuoteSource(text)
	}
	sources[stem] = source
	return source
}

// placeQuote answers where quote sits against the section cited, and the
// section it names: the resolved cited section, or for a quote outside it the
// section that does carry it. The match is the row quote rule's own
// (quoteSource.inSection), so the cited section and its subsections count. A
// quote no section carries is quoteNotInRFC whether or not the cited section
// resolves, so the refusal names the absent sentence first.
func placeQuote(source *quoteSource, cited, quote string) (quotePlace, string) {
	if source == nil {
		return quoteNoText, ""
	}
	resolved, ok := source.resolve(cited)
	if ok && source.inSection(resolved, quote) {
		return quoteInSection, resolved
	}
	found, elsewhere := source.sectionOf(quote)
	if !elsewhere {
		return quoteNotInRFC, resolved
	}
	if !ok {
		return quoteSectionUnresolved, cited
	}
	return quoteOutsideSection, found
}

// quoteGap phrases a quote that is not in the row's cited section as the
// evidence the row lacks. False when the quote is in the section.
func quoteGap(req *Requirement, source *quoteSource, what string) (string, bool) {
	place, section := placeQuote(source, req.Section, req.Annotation.Quote)
	var tb textbuf.Buffer
	switch place {
	case quoteInSection:
		return "", false
	case quoteOutsideSection:
		return tb.Str("its ").Str(what).Str(" is in section ").Str(section).
			Str(", and the row cites section ").Str(req.Section).
			Str(". The quote must come from the section the row cites or its subsections").String(), true
	case quoteNotInRFC:
		return tb.Str("its ").Str(what).Str(" is in no section of ").Str(fullRel).Byte('/').Str(req.RFC).
			Str(".txt; quote the RFC's sentence verbatim, from section ").Str(req.Section).String(), true
	case quoteNoText:
		return tb.Str("the RFC's own text is not in this repository, so its ").Str(what).
			Str(" can be checked against nothing; fetch it to ").Str(fullRel).Byte('/').
			Str(req.RFC).Str(".txt").String(), true
	case quoteSectionUnresolved:
		return tb.Str("the section it cites, ").Str(section).
			Str(", names no heading of the RFC, so no section holds its ").Str(what).String(), true
	default:
		panic("BUG: unexpected quote place")
	}
}

// layerPathGap answers what is wrong with a {lower-layer} layer path, false
// when a reader can open it. A vendored module is held to this tree; the
// kernel and the Go standard library are not in it, so their paths are held to
// the shape of their source trees.
func layerPathGap(tree, layerPath string) (string, bool) {
	var tb textbuf.Buffer
	if layerPath == "" {
		return "its layer names no component path a reader can open: add linux:<kernel source path>, go:<standard library package> or vendor/<module path> to the layer", true
	}
	kind, rel, found := strings.Cut(layerPath, ":")
	if !found {
		kind, rel = "vendor", layerPath
	}
	if slices.Contains(strings.Split(rel, "/"), "..") {
		return tb.Str("its layer path ").Str(layerPath).Str(" climbs out of its source tree with `..`").String(), true
	}
	top, _, _ := strings.Cut(rel, "/")
	switch kind {
	case "vendor":
		if _, err := os.Stat(filepath.Join(tree, rel)); err != nil {
			return tb.Str("its layer path ").Str(layerPath).
				Str(" cannot be opened in this checkout (").Str(err.Error()).
				Str("); name the vendored module directory under vendor/").String(), true
		}
	case "linux":
		if !kernelSourceTops[top] {
			return tb.Str("its layer path ").Str(layerPath).
				Str(" opens under no top-level directory of a Linux source tree; name the file or directory, as linux:net/ipv6/ndisc.c").String(), true
		}
	case "go":
		if top == "" {
			return tb.Str("its layer path ").Str(layerPath).Str(" names no package").String(), true
		}
		if strings.Contains(top, ".") {
			return tb.Str("its layer path ").Str(layerPath).
				Str(" is not a standard library package; a module from outside the standard library is named as vendor/<module path>").String(), true
		}
	default:
		panic("BUG: layerPathRE admitted an unknown layer path form")
	}
	return "", false
}

// exclusionEvidenceGap answers the evidence one row's exclusion kind owes and
// does not carry, and true; "" and false when the row carries it or no
// evidence kind annotates it.
//
// The row's own text is already the RFC's sentence (checkRowQuotes); what each
// kind adds is the fact that removes the row from Ze's count (owner ruling,
// 2026-10-29): for {not-applicable} a role-condition quote from the cited
// section and the role Ze fills, for {feature-declined} its optionality quote
// within the cited section, for {lower-layer} a layer path a reader can open
// beside the producer checkLowerLayerProducer already holds.
func exclusionEvidenceGap(tree string, sources map[string]*quoteSource, req *Requirement) (string, bool) {
	if req.Annotation == nil {
		return "", false
	}
	switch req.Annotation.Kind {
	case AnnotationNotApplicable:
		if req.Annotation.Quote == "" {
			var tb textbuf.Buffer
			return tb.Str("it carries no role-condition quote from section ").Str(req.Section).
				Str(" and names no role ze fills").String(), true
		}
		if req.Annotation.Role == "" {
			// Unreachable through the parser, which refuses a quote with no
			// role. The check still refuses it: the role is half the fact.
			return "it names no role ze fills", true
		}
		return quoteGap(req, loadQuoteSource(sources, tree, req.RFC), "role-condition quote")
	case AnnotationFeatureDeclined:
		return quoteGap(req, loadQuoteSource(sources, tree, req.RFC), "optionality quote")
	case AnnotationLowerLayer:
		return layerPathGap(tree, req.Annotation.LayerPath)
	default:
		return "", false
	}
}

// evidenceFormat answers the format sentence a refusal for this kind ends with.
func evidenceFormat(kind string) string {
	switch kind {
	case AnnotationNotApplicable:
		return notApplicableFormat
	case AnnotationFeatureDeclined:
		return featureDeclinedFormat
	case AnnotationLowerLayer:
		return lowerLayerFormat
	default:
		panic("BUG: evidenceFormat asked for a kind that owes no evidence")
	}
}

// checkExclusionEvidence is the exclusion evidence ratchet.
//
// Every row of an evidence kind that lacks its evidence is counted per kind.
// A row the commit added or changed against HEAD^ is refused by its id. The
// count of each kind must equal the committed count in evidenceBacklogRel:
// above it, a row lost its evidence; below it, the committed count is lowered
// in the same commit, so it only falls. A kind whose committed count is zero
// is refused row by row, which is full refusal. When HEAD^ cannot be read no
// row is known to be new, and only the counts are judged.
func checkExclusionEvidence(tree string, requirements []Requirement, baseline map[string]string,
	unreadable map[string]bool, baselineKnown bool,
) []string {
	backlog, refusal := readEvidenceBacklog(tree)
	if refusal != "" {
		return []string{refusal}
	}
	sources := map[string]*quoteSource{}
	lacking := map[string]int{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		missing, lacks := exclusionEvidenceGap(tree, sources, req)
		if !lacks {
			continue
		}
		kind := req.Annotation.Kind
		lacking[kind]++
		if backlog[kind] > 0 && !evidenceRowIsNew(req, baseline, unreadable, baselineKnown) {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(requirementWhere(*req)).Str(": ").Str(req.RID).
			Str(" is annotated {").Str(kind).Str("} and lacks the evidence that kind owes: ").
			Str(missing).Str(". ").Str(evidenceFormat(kind)).String())
	}
	for _, kind := range evidenceKinds {
		if message, refused := evidenceCountRefusal(kind, lacking[kind], backlog[kind]); refused {
			errs = append(errs, message)
		}
	}
	return errs
}

// evidenceRowIsNew answers whether the commit added or changed this row's
// evidence against HEAD^. A row of a summary HEAD^ could not parse is not
// known to be new, and neither is any row when HEAD^ cannot be read.
func evidenceRowIsNew(req *Requirement, baseline map[string]string, unreadable map[string]bool,
	baselineKnown bool,
) bool {
	if !baselineKnown {
		return false
	}
	if unreadable[req.RFC] {
		return false
	}
	held, existed := baseline[req.RID]
	// refuse-by-default: guards checkExclusionEvidence -- an id HEAD^ never held is a row this commit added
	if !existed {
		return true
	}
	return held != evidenceFingerprint(req)
}

// evidenceCountRefusal compares one kind's count of rows lacking evidence with
// its committed count.
func evidenceCountRefusal(kind string, count, committed int) (string, bool) {
	if count == committed {
		return "", false
	}
	if committed == 0 {
		// Full refusal: checkExclusionEvidence already refused each row by id.
		return "", false
	}
	var tb textbuf.Buffer
	tb.Str(evidenceBacklogRel).Str(": ").Int(int64(count)).Str(" {").Str(kind).
		Str("} rows lack their evidence, and the committed count is ").Int(int64(committed)).Str(". ")
	if count < committed {
		return tb.Str("The count may only fall, so the file records the fall: set the line to `").
			Str(kind).Byte(' ').Int(int64(count)).Str("` in this commit").String(), true
	}
	return tb.Str("A row whose line this commit left alone lost its evidence, because the RFC text or the path it names changed under it. ").
		Str("Restore that evidence; the committed count never rises").String(), true
}

// readEvidenceBacklog reads the committed count of each evidence kind. An
// unreadable file, an unknown kind, a malformed count or a missing kind is a
// refusal: a ratchet with no committed count judges nothing, and passing it
// silently is how a guard stops guarding.
func readEvidenceBacklog(tree string) (map[string]int, string) {
	raw, err := os.ReadFile(filepath.Join(tree, evidenceBacklogRel))
	if err != nil {
		return nil, evidenceBacklogRel + ": cannot read the committed evidence counts (" + err.Error() +
			"). Restore the file from git: it holds one `<kind> <count>` line for each of " +
			strings.Join(evidenceKinds, ", ")
	}
	backlog := map[string]int{}
	for number, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		where := evidenceBacklogRel + ":" + strconv.Itoa(number+1)
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, where + ": expected `<kind> <count>`, got " + pyRepr(line)
		}
		if !slices.Contains(evidenceKinds, fields[0]) {
			return nil, where + ": " + pyRepr(fields[0]) + " is not a kind this ratchet counts; use one of " +
				strings.Join(evidenceKinds, ", ")
		}
		count, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, where + ": the count " + pyRepr(fields[1]) + " is not a whole number"
		}
		if count < 0 {
			return nil, where + ": the count " + fields[1] + " is negative"
		}
		backlog[fields[0]] = count
	}
	for _, kind := range evidenceKinds {
		if _, held := backlog[kind]; !held {
			return nil, evidenceBacklogRel + ": no line counts {" + kind + "}; add `" + kind + " <count>`"
		}
	}
	return backlog, ""
}
