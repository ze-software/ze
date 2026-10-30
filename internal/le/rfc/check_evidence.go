// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Related: summary.go -- parseNotApplicable, parseLowerLayer, parseFeatureDeclined: the payloads it judges
// Related: check_evidence_linked.go -- linkedPackages, what a go: or vendor/ layer path is held to
// Related: internal/appliance/kernelreg.go -- RuntimeKernelLayers, the kernel config a linux: layer path is held to
// Related: check_quote.go -- the row quote rule, whose section matcher it shares
// Related: check_baseline.go -- baselineLevels, the HEAD^ fingerprints it compares
package rfc

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// evidenceBacklogRel is the committed count of rows, per counted kind, that
// still lack the evidence their exclusion kind owes. The count may only fall,
// and a kind whose count is zero is refused row by row.
const evidenceBacklogRel = "rfc/exclusion-evidence-backlog.txt"

// evidenceKinds are the exclusion kinds whose rows lacking evidence the
// backlog may count, in the order a refusal lists them. {feature-declined} is
// judged ((*evidenceReader).gap) but never counted: parseFeatureDeclined
// refuses a row with no quote, so every such row carries its evidence form,
// and a row whose form fails is refused by id. {linux-limit} is neither:
// checkLinuxLimit already holds its register entry, which is its whole
// evidence.
var evidenceKinds = []string{AnnotationLowerLayer, AnnotationNotApplicable}

// retiredEvidenceKinds are kinds a committed copy of the backlog file once
// counted and the ratchet no longer does. {feature-declined} left the file
// when it became judged row by row and never counted, and every copy committed
// before then still holds its line. A historical copy (committedBacklogAt)
// skips such a line rather than failing to parse: refusing it would leave
// every earlier commit without a copy that parses, and the rise check against
// HEAD^ would compare nothing while it seemed to pass. The working-tree copy
// is still refused for one, because the commit in hand must not write it.
var retiredEvidenceKinds = []string{AnnotationFeatureDeclined}

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

// containsWords answers whether phrase occurs in text at word boundaries
// (wordIndex), so "s" is not found in "a speaker" and "speaker" is found in
// "a bgp speaker must".
func containsWords(text, phrase string) bool {
	return wordIndex(text, phrase) >= 0
}

// wordIndex answers the byte offset of the first occurrence of phrase in text
// at word boundaries, -1 when there is none or phrase is empty: every match is
// tried, and one counts only when neither neighbor is a letter or a digit.
func wordIndex(text, phrase string) int {
	if phrase == "" {
		return -1
	}
	for start := 0; start <= len(text)-len(phrase); {
		found := strings.Index(text[start:], phrase)
		if found < 0 {
			return -1
		}
		found += start
		end := found + len(phrase)
		before, _ := utf8.DecodeLastRuneInString(text[:found])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if !isWordRune(before) && !isWordRune(after) {
			return found
		}
		start = found + 1
	}
	return -1
}

// isWordRune answers whether r continues a word. utf8.RuneError, which the
// decoders answer at either end of the text, does not.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
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

// evidenceReader judges the evidence of one corpus's exclusion rows. It reads
// each RFC text, this checkout's vendor/modules.txt, the package set the
// shipped ze links (one `go list -deps` per shipped flavor, linkedPackages)
// and Ze's runtime kernel config with its require manifests at most once, so
// a corpus of a thousand rows pays for each source once. Not safe for
// concurrent use.
type evidenceReader struct {
	tree    string
	sources map[string]*quoteSource

	// listLinked answers what the shipped ze links; goListLinked, except in
	// a test that judges a fixture checkout holding no Go module.
	listLinked    func(tree string) (linkedPackages, string)
	linkedRead    bool
	linked        linkedPackages
	linkedRefusal string

	kernelRead      bool
	kernelOptions   map[string]bool // option name to built in (=y)
	kernelRequired  map[string]bool // options a .require manifest lists
	kernelManifests []string        // the manifests read, in build order
	kernelRefusal   string

	vendorRead    bool
	vendorModules map[string]bool // module paths vendor/modules.txt names
	vendorRefusal string
}

func newEvidenceReader(tree string) *evidenceReader {
	return &evidenceReader{tree: tree, sources: map[string]*quoteSource{}, listLinked: goListLinked}
}

// vendorDir is the directory a vendored module path opens under, in this
// checkout and in the Go source tree alike, where it holds modules the
// standard library vendors rather than standard library packages.
const vendorDir = "vendor"

// vendorModulesRel is the list `go mod vendor` writes, one `# <module>
// <version>` line per vendored module.
const vendorModulesRel = vendorDir + "/modules.txt"

// kernelOptionRE is the CONFIG_ option a `linux:` layer path names.
var kernelOptionRE = regexp.MustCompile(`^CONFIG_[A-Z0-9_]+$`)

// layerPathGap answers what is wrong with a {lower-layer} layer path, false
// when a reader can check it. A vendored module is held to this tree's
// vendor/modules.txt and to the modules the shipped ze links, a kernel
// option to Ze's runtime kernel config and its require manifests, and a Go
// package to the standard library packages the shipped ze links.
func (reader *evidenceReader) layerPathGap(layerPath string) (string, bool) {
	var tb textbuf.Buffer
	if layerPath == "" {
		return "its layer names no component a reader can check: add linux:CONFIG_<option>, go:<standard library package> or vendor/<module path> to the layer", true
	}
	kind, rel, found := strings.Cut(layerPath, ":")
	if !found {
		kind, rel = vendorDir, layerPath
	}
	segments := strings.Split(rel, "/")
	if slices.Contains(segments, "..") {
		return tb.Str("its layer path ").Str(layerPath).Str(" climbs out of its source tree with `..`; name the component itself").String(), true
	}
	if slices.Contains(segments, ".") {
		return tb.Str("its layer path ").Str(layerPath).Str(" holds a `.` segment, which names the directory around it; name the component itself").String(), true
	}
	switch kind {
	case vendorDir:
		return reader.vendorModuleGap(layerPath, strings.TrimPrefix(rel, vendorDir+"/"))
	case "linux":
		return reader.kernelOptionGap(layerPath, rel)
	case "go":
		return reader.goPackageGap(layerPath, rel)
	default:
		panic("BUG: layerPathRE admitted an unknown layer path form")
	}
}

// vendorModuleGap answers what is wrong with a `vendor/` layer path: it must
// name a module exactly as vendor/modules.txt lists it, the module's
// directory must be in this checkout, and some shipped flavor of ze must
// link a package of it. A package inside a module, the vendor directory
// itself or a file under it is no module, and a module only the tooling uses
// (most of vendor/ serves golangci-lint) performs nothing for Ze.
func (reader *evidenceReader) vendorModuleGap(layerPath, module string) (string, bool) {
	modules, refusal := reader.vendoredModules()
	if refusal != "" {
		return refusal, true
	}
	var tb textbuf.Buffer
	if !modules[module] {
		return tb.Str("its layer path ").Str(layerPath).Str(" names no module ").Str(vendorModulesRel).
			Str(" lists (no `# ").Str(module).Str(" <version>` line); name the vendored module as vendor/<module path>, exactly as that file spells it").String(), true
	}
	if _, err := os.Stat(filepath.Join(reader.tree, vendorDir, filepath.FromSlash(module))); err != nil {
		return tb.Str("its layer path ").Str(layerPath).
			Str(" cannot be opened in this checkout (").Str(err.Error()).
			Str("); run `go mod vendor`, or name a module whose code is vendored").String(), true
	}
	linked, refusal := reader.linkedPackages()
	if refusal != "" {
		return refusal, true
	}
	if !linked.modules[module] {
		return tb.Str("its layer path ").Str(layerPath).Str(" names a module ").Str(vendorModulesRel).
			Str(" lists that no shipped flavor of ze links (no package of it is in `go list -deps` over ").Str(shippedMain).
			Str("), so it performs nothing for Ze; name the vendored module whose code the shipped ze runs").String(), true
	}
	return "", false
}

// vendoredModules answers every module vendor/modules.txt names, reading it
// once. An unreadable list refuses every vendor/ path: a module cannot be
// told absent from a list nobody read.
func (reader *evidenceReader) vendoredModules() (map[string]bool, string) {
	if reader.vendorRead {
		return reader.vendorModules, reader.vendorRefusal
	}
	reader.vendorRead = true
	raw, err := os.ReadFile(filepath.Join(reader.tree, filepath.FromSlash(vendorModulesRel))) //nolint:gosec // the path is the checkout's own vendor list, not input
	if err != nil {
		reader.vendorRefusal = "cannot read " + vendorModulesRel + " (" + err.Error() +
			"), so no vendor/ layer path can be checked; run `go mod vendor`"
		return nil, reader.vendorRefusal
	}
	modules := map[string]bool{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "#" {
			continue
		}
		modules[fields[1]] = true
	}
	reader.vendorModules = modules
	return modules, ""
}

// linkedPackages answers what the shipped ze links, asking listLinked once.
func (reader *evidenceReader) linkedPackages() (linkedPackages, string) {
	if !reader.linkedRead {
		reader.linkedRead = true
		reader.linked, reader.linkedRefusal = reader.listLinked(reader.tree)
	}
	return reader.linked, reader.linkedRefusal
}

// kernelOptionGap answers what is wrong with a `linux:` layer path: it must
// name a CONFIG_ option that Ze's kernel config builds in (=y) and that a
// require manifest lists, so the build has verified the final .config
// carries it. enforceRequiredSymbols accepts only =y for a listed option, so
// a listed option at =m fails the build and no shipped kernel carries it.
func (reader *evidenceReader) kernelOptionGap(layerPath, option string) (string, bool) {
	var tb textbuf.Buffer
	if !kernelOptionRE.MatchString(option) {
		return tb.Str("its layer path ").Str(layerPath).
			Str(" names no CONFIG_ option; write linux:CONFIG_<option>, as linux:CONFIG_IPV6, naming the option of Ze's runtime kernel config (gokrazy/kernel) that builds the layer").String(), true
	}
	options, refusal := reader.enabledKernelOptions()
	if refusal != "" {
		return refusal, true
	}
	if !options[option] {
		return tb.Str("its layer path ").Str(layerPath).Str(" is not enabled in Ze's kernel (").
			Str(strings.Join(reader.kernelFragments(), ", ")).
			Str(" do not set it to y, the only value the build verifies a required option at), so no kernel Ze ships performs it; name the option that builds the layer, or build it in with =y").String(), true
	}
	if !reader.kernelRequired[option] {
		return tb.Str("its layer path ").Str(layerPath).Str(" is asked for in Ze's kernel config but listed in none of ").
			Str(strings.Join(reader.kernelManifests, ", ")).
			Str(", and a fragment line is a request kconfig may answer otherwise; list ").Str(option).
			Str(" in the manifest beside the fragment that sets it, so the build verifies the final .config carries it").String(), true
	}
	return "", false
}

// kernelFragments names the fragments a refusal cites: the manifests read,
// each with its .config name.
func (reader *evidenceReader) kernelFragments() []string {
	fragments := make([]string, 0, len(reader.kernelManifests))
	for _, manifest := range reader.kernelManifests {
		fragments = append(fragments, strings.TrimSuffix(manifest, ".require")+".config")
	}
	return fragments
}

// goPackageGap answers what is wrong with a `go:` layer path: it must name a
// standard library package some shipped flavor of ze links. cmd/ and vendor/
// hold the toolchain's own programs and modules, not the standard library,
// and the go tool ignores a directory named testdata or opening with `_` or
// `.`, so no package lives under one. A package the binary does not link
// performs nothing for Ze, however real it is.
func (reader *evidenceReader) goPackageGap(layerPath, pkg string) (string, bool) {
	var tb textbuf.Buffer
	for segment := range strings.SplitSeq(pkg, "/") {
		if goToolIgnores(segment) {
			return tb.Str("its layer path ").Str(layerPath).Str(" holds the segment ").Str(segment).
				Str(", which the go tool ignores (testdata, or a name opening with `_` or `.`), so it names no package; name the standard library package itself as go:<import path>").String(), true
		}
	}
	linked, refusal := reader.linkedPackages()
	if refusal != "" {
		return refusal, true
	}
	top, _, _ := strings.Cut(pkg, "/")
	if top != "cmd" && top != vendorDir && linked.standard[pkg] {
		return "", false
	}
	return tb.Str("its layer path ").Str(layerPath).
		Str(" is not a package of the Go standard library that the shipped ze links (`go list -deps` over ").Str(shippedMain).
		Str(" in every shipped flavor lists no such standard package); name a package the binary links as go:<import path>, and a module from outside the standard library as vendor/<module path>").String(), true
}

// goTestdataDir is the directory name the go tool never reads a package from.
const goTestdataDir = "testdata"

// goToolIgnores answers whether the go tool skips a directory of this name
// when it looks for packages (`go help packages`).
func goToolIgnores(segment string) bool {
	return segment == goTestdataDir || strings.HasPrefix(segment, "_") || strings.HasPrefix(segment, ".")
}

// enabledKernelOptions answers every option Ze's runtime kernel config sets,
// mapped to whether it is built in, and records the options the require
// manifests list. The fragments and manifests, and their order, are the
// builder's own (appliance.RuntimeKernelLayers): a later fragment overrides
// an earlier one, an include overrides both, and a `# ze-base:` profile is
// read where the builder reads it. A fragment or manifest that cannot be
// read is a refusal: the option cannot be told absent from unreadable.
func (reader *evidenceReader) enabledKernelOptions() (map[string]bool, string) {
	if reader.kernelRead {
		return reader.kernelOptions, reader.kernelRefusal
	}
	reader.kernelRead = true
	fragments, manifests, err := appliance.RuntimeKernelLayers(reader.tree)
	if err != nil {
		reader.kernelRefusal = "cannot resolve Ze's runtime kernel config (" + err.Error() +
			"), so no linux: layer path can be checked against it; restore the fragments under gokrazy/kernel from git"
		return nil, reader.kernelRefusal
	}
	options := map[string]bool{}
	for _, fragment := range fragments {
		if refusal := readKernelFragment(reader.tree, fragment, options); refusal != "" {
			reader.kernelRefusal = refusal
			return nil, refusal
		}
	}
	required := map[string]bool{}
	for _, manifest := range manifests {
		rel := relTo(reader.tree, manifest)
		if refusal := readKernelRequire(manifest, rel, required); refusal != "" {
			reader.kernelRefusal = refusal
			return nil, refusal
		}
		reader.kernelManifests = append(reader.kernelManifests, rel)
	}
	reader.kernelOptions = options
	reader.kernelRequired = required
	return options, ""
}

// kernelUnsetSuffix closes kconfig's `# CONFIG_<name> is not set` line, which
// disables an option a fragment before it set.
const kernelUnsetSuffix = " is not set"

// readKernelFragment records each `CONFIG_<name>=<value>` line of one config
// fragment into options, and each `# CONFIG_<name> is not set` line as
// disabled, or answers a refusal naming the file when it cannot be read.
func readKernelFragment(tree, path string, options map[string]bool) string {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is the builder's resolution of the checkout's own kernel config, not input
	if err != nil {
		return "cannot read " + relTo(tree, path) + " (" + err.Error() +
			"), so no linux: layer path can be checked against Ze's kernel config; restore the file from git"
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if unset, disabled := strings.CutSuffix(line, kernelUnsetSuffix); disabled {
			if name, commented := strings.CutPrefix(unset, "# "); commented && kernelOptionRE.MatchString(name) {
				options[name] = false
			}
			continue
		}
		name, value, assigned := strings.Cut(line, "=")
		if !assigned || !strings.HasPrefix(name, "CONFIG_") {
			continue
		}
		options[name] = value == "y"
	}
	return ""
}

// readKernelRequire records each option one require manifest lists, written
// `CONFIG_<name>` or `CONFIG_<name>=y` as enforceRequiredSymbols reads it.
// A line of any other shape is a refusal naming the file and line, as the
// build refuses it.
func readKernelRequire(path, rel string, required map[string]bool) string {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is the builder's resolution of the checkout's own kernel require manifest, not input
	if err != nil {
		return "cannot read " + rel + " (" + err.Error() +
			"), so no linux: layer path can be checked against the options the kernel build verifies; restore the file from git"
	}
	for number, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name := strings.TrimSuffix(line, "=y")
		if !kernelOptionRE.MatchString(name) {
			return rel + ":" + strconv.Itoa(number+1) + ": " + pyRepr(line) +
				" is not `CONFIG_<name>` or `CONFIG_<name>=y`, so the manifest cannot be read; correct the line as the kernel build requires"
		}
		required[name] = true
	}
	return ""
}

// gap answers the evidence one row's exclusion kind owes and does not carry,
// and true; "" and false when the row carries it or no evidence kind
// annotates it.
//
// The row's own text is already the RFC's sentence (checkRowQuotes); what each
// kind adds is the fact that removes the row from Ze's count (owner ruling,
// 2026-10-29): for {not-applicable} a role-condition quote from the cited
// section and the specific role it binds, which checkNotApplicableRoles holds
// against the roles the summary's Meta table declares Ze fills, for {feature-declined}
// its optionality quote within the cited section, for {lower-layer} a layer
// path a reader can check beside the producer checkLowerLayerProducer already
// holds.
func (reader *evidenceReader) gap(req *Requirement) (string, bool) {
	if req.Annotation == nil {
		return "", false
	}
	switch req.Annotation.Kind {
	case AnnotationNotApplicable:
		if req.Annotation.Quote == "" {
			var tb textbuf.Buffer
			return tb.Str("it carries no role-condition quote from section ").Str(req.Section).
				Str(" and names no role that quote binds").String(), true
		}
		if req.Annotation.BoundRole == "" {
			// Unreachable through the parser, which refuses a quote with no
			// bound role. The check still refuses it: the role is half the fact.
			return "it names no role the quote binds", true
		}
		return reader.roleQuoteGap(req)
	case AnnotationFeatureDeclined:
		return quoteGap(req, loadQuoteSource(reader.sources, reader.tree, req.RFC), "optionality quote")
	case AnnotationLowerLayer:
		return reader.layerPathGap(req.Annotation.LayerPath)
	default:
		return "", false
	}
}

// roleQuoteGap judges a {not-applicable} row's quote against its RFC and its
// row: it must be in the cited section or its subsections (quoteGap), a whole
// sentence there (wholeSentence), and, when it holds a BCP 14 keyword, the
// row's own obligation sentence (rowOwnSentence), so a row cannot borrow
// another obligation's subject to exclude Ze from its own. A quote with no
// keyword is a scoping sentence, judged for its marker and role by the parser
// (scopingRefusal).
func (reader *evidenceReader) roleQuoteGap(req *Requirement) (string, bool) {
	source := loadQuoteSource(reader.sources, reader.tree, req.RFC)
	if missing, lacks := quoteGap(req, source, "role-condition quote"); lacks {
		return missing, true
	}
	var tb textbuf.Buffer
	section, _ := source.resolve(req.Section)
	if !source.wholeSentence(section, req.Annotation.Quote) {
		return tb.Str("its role-condition quote is not a whole sentence of section ").Str(section).
			Str(": quote it from the word that opens the sentence to the full stop that ends it, so no condition of the sentence is cut away").String(), true
	}
	if at, _ := firstKeyword(req.Annotation.Quote); at >= 0 && !rowOwnSentence(req.Text, req.Annotation.Quote) {
		return tb.Str("its role-condition quote holds a BCP 14 keyword and is not the row's own obligation sentence, so the role it binds may not be the row's. ").
			Str(strings.TrimSpace(notApplicableShapes)).String(), true
	}
	return "", false
}

// rowOwnSentence answers whether quote is the obligation the row states: the
// row text holds the quote, or the quote holds the row text, compared in
// quoteNeedle's form as a row quote is matched. An empty row text holds
// nothing.
func rowOwnSentence(text, quote string) bool {
	row, held := quoteNeedle(strings.TrimSpace(text)), quoteNeedle(strings.TrimSpace(quote))
	if row == "" {
		return false
	}
	if strings.Contains(held, row) {
		return true
	}
	return strings.Contains(row, held)
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

// countedRowsNamedMax bounds how many rows a count refusal names in full.
const countedRowsNamedMax = 10

// checkExclusionEvidence is the exclusion evidence ratchet. It answers the
// refusals, and a notice for the report when HEAD^ is absent.
//
// A row added or changed since HEAD^ or the pushed branch (evidenceRowIsNew)
// that lacks its evidence is refused by its id and is not counted: it is the
// commit's to fix, and counting it too would bill the backlog for it. So is
// an untouched row whose evidence form no longer passes its check. Every other
// row lacking evidence is counted per kind, and that count must equal the
// committed count in evidenceBacklogRel: below it, the committed count is
// lowered in the same commit; above it, the line was set below the rows it
// counts. The committed count itself may rise neither above HEAD^'s nor above
// the pushed branch's. A kind whose committed count is zero is refused row by
// row, which is full refusal, and a judged kind the backlog never counts
// ({feature-declined}) is refused row by row too. A committed copy that does
// not parse is not refused here: the commit that wrote it was refused by
// readEvidenceBacklog, and the commit repairing it must be able to pass, so
// the counts compare with the newest copy that parses. A HEAD^ git cannot
// read is refused; only an absent HEAD^ judges every row as untouched, and
// the notice says so, as it says when the pushed branch is absent.
func checkExclusionEvidence(tree string, requirements []Requirement, base evidenceBaseline) ([]string, string) {
	backlog, refusal := readEvidenceBacklog(tree)
	if refusal != "" {
		return []string{refusal}, ""
	}
	var notice string
	switch base.state {
	case priorRead:
		notice = joinNotices(base.historyNotice, base.pushedNotice)
	case priorAbsent:
		notice = "exclusion evidence: no HEAD^ (" + base.reason + "), so no row is known to be new: every row lacking evidence is counted against " +
			evidenceBacklogRel + " and no committed count is compared with HEAD^"
	case priorUnreadable:
		// The reason ends with its own fix: a shallow clone, a lost object
		// and a failing command each repair differently.
		return []string{evidenceBacklogRel + ": cannot read HEAD^, so no row can be told new from untouched and the committed counts cannot be compared with HEAD^: " +
			base.reason}, ""
	case priorUnspecified:
		panic("BUG: checkExclusionEvidence was given no HEAD^ state")
	default:
		panic("BUG: unknown HEAD^ state")
	}
	var errs []string
	reader := newEvidenceReader(tree)
	counted := map[string][]string{}
	for i := range requirements {
		req := &requirements[i]
		missing, lacks := reader.gap(req)
		if !lacks {
			continue
		}
		kind := req.Annotation.Kind
		var tb textbuf.Buffer
		tb.Str(requirementWhere(*req)).Str(": ").Str(req.RID).Str(" is annotated {").Str(kind).Str("} and ")
		if !slices.Contains(evidenceKinds, kind) {
			errs = append(errs, tb.Str("lacks the evidence that kind owes: ").
				Str(missing).Str(". Rewrite the annotation on that line to carry it. ").Str(evidenceFormat(kind)).String())
			continue
		}
		touched := evidenceRowIsNew(req, base)
		// An untouched row that carries an evidence form and fails its check
		// is never backlog: counting it would let a commit break one row's
		// evidence while it backfills another and keep the count level.
		if !touched && carriesEvidenceForm(req.Annotation) {
			errs = append(errs, tb.Str("carries evidence that no longer passes its check: ").Str(missing).
				Str(". The RFC text or the component it names changed under it: restore what it names, or rewrite the annotation on that line. ").
				Str(evidenceFormat(kind)).String())
			continue
		}
		if !touched {
			counted[kind] = append(counted[kind], requirementWhere(*req)+" "+req.RID)
		}
		if !touched && backlog[kind] > 0 {
			continue
		}
		errs = append(errs, tb.Str("lacks the evidence that kind owes: ").
			Str(missing).Str(". Rewrite the annotation on that line to carry it. ").Str(evidenceFormat(kind)).String())
	}
	for _, kind := range evidenceKinds {
		if message, refused := evidenceCountRefusal(kind, backlog[kind], counted[kind]); refused {
			errs = append(errs, message)
		}
		if message, refused := evidencePriorRefusal(kind, backlog[kind], base.backlog, base.backlogAt); refused {
			errs = append(errs, message)
		}
		if message, refused := evidencePriorRefusal(kind, backlog[kind], base.pushed, base.pushedAt); refused {
			errs = append(errs, message)
		}
	}
	return errs, notice
}

// joinNotices joins the non-empty notices into the one report line the check
// prints, each ending where the report adds its full stop.
func joinNotices(notices ...string) string {
	var held []string
	for _, notice := range notices {
		if notice != "" {
			held = append(held, notice)
		}
	}
	return strings.Join(held, ".\n")
}

// checkZeRolesInText refuses a role a summary's `Ze roles` row declares when
// its RFC text never names it as whole words after roleKey, read through the
// loaded quote source (loadQuoteSource). A declared role the RFC never uses
// ("observer" for an RFC about speakers) is held by no bound role, so every
// row would pass against it. Only a summary holding a quoted {not-applicable}
// row is judged, since no other row is held against the declaration; and a
// summary whose RFC text is absent is refused, because the roles cannot be
// found in a text nobody has.
func checkZeRolesInText(tree string, metas map[string]Meta, requirements []Requirement) []string {
	sources := map[string]*quoteSource{}
	judged := map[string]bool{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		if req.Annotation == nil || req.Annotation.Kind != AnnotationNotApplicable || req.Annotation.BoundRole == "" {
			continue
		}
		if judged[req.RFC] {
			continue
		}
		judged[req.RFC] = true
		source := loadQuoteSource(sources, tree, req.RFC)
		var tb textbuf.Buffer
		if source == nil {
			errs = append(errs, tb.Str(req.Source).Str(": the Meta `").Str(metaZeRoles).Str("` row cannot be checked, because rfc/full/").
				Str(req.RFC).Str(".txt is absent; fetch it with `curl -o rfc/full/").Str(req.RFC).
				Str(".txt https://www.rfc-editor.org/rfc/").Str(req.RFC).Str(".txt`").String())
			continue
		}
		for _, role := range metas[req.RFC].ZeRoles {
			if source.holdsWords(roleKey(role)) {
				continue
			}
			var line textbuf.Buffer
			errs = append(errs, line.Str(req.Source).Str(": the Meta `").Str(metaZeRoles).Str("` row declares ").Str(pyRepr(role)).
				Str(", which rfc/full/").Str(req.RFC).Str(".txt never names; name each role ze fills in the RFC's own words, so a bound role can be told from it").String())
		}
	}
	return errs
}

// holdsWords answers whether any section of the RFC holds key as whole words
// once the section passes through roleKey, as a declared role is compared.
func (q *quoteSource) holdsWords(key string) bool {
	for i := range q.sections {
		if containsWords(roleKey(q.sections[i].body), key) {
			return true
		}
	}
	return false
}

// evidenceRowIsNew answers whether this row's evidence was added or changed
// since HEAD^ or since the pushed branch. A row is untouched only when both
// record it unchanged, so a row one red commit added while it backfilled
// another, keeping the count level, is still new in the next commit. A clone
// with no pushed branch judges against HEAD^ alone. A row of a summary HEAD^
// could not parse is judged against the pushed branch only, and is new
// without one: a summary nobody could read never shelters a row. No row is
// new when HEAD^ is absent.
func evidenceRowIsNew(req *Requirement, base evidenceBaseline) bool {
	if base.state != priorRead {
		return false
	}
	fingerprint := evidenceFingerprint(req)
	if base.unreadable[req.RFC] {
		return !base.pushedRead || !heldUnchanged(base.pushedPrints, req.RID, fingerprint)
	}
	if !heldUnchanged(base.prints, req.RID, fingerprint) {
		return true
	}
	return base.pushedRead && !heldUnchanged(base.pushedPrints, req.RID, fingerprint)
}

// carriesEvidenceForm answers whether an annotation is written in its kind's
// evidence form (a quote, or a layer path), as against a row written before
// the kind carried evidence.
func carriesEvidenceForm(annotation *Annotation) bool {
	return annotation.Quote != "" || annotation.LayerPath != ""
}

// heldUnchanged answers whether prints records rid with this fingerprint. An
// id it never held is a row added since, so it answers false.
func heldUnchanged(prints map[string]string, rid, fingerprint string) bool {
	held, existed := prints[rid]
	return existed && held == fingerprint
}

// evidenceCountRefusal compares one kind's count of untouched rows lacking
// evidence, the rows counted, with its committed count. No counted row
// carries an evidence form: checkExclusionEvidence refuses such a row by id.
func evidenceCountRefusal(kind string, committed int, counted []string) (string, bool) {
	count := len(counted)
	if count == committed {
		return "", false
	}
	if committed == 0 {
		// Full refusal: checkExclusionEvidence already refused each row by id.
		return "", false
	}
	var tb textbuf.Buffer
	if count < committed {
		tb.Str(evidenceBacklogRel).Str(": ").Int(int64(count)).Str(" {").Str(kind).
			Str("} rows lack their evidence, and the committed count is ").Int(int64(committed)).Str(". ")
		return tb.Str("The count may only fall, so the file records the fall: set the line to `").
			Str(kind).Byte(' ').Int(int64(count)).Str("` in this commit").String(), true
	}
	// Never advise raising the line: a higher line is refused as a rise.
	tb.Str(evidenceBacklogRel).Str(": ").Int(int64(count)).Str(" {").Str(kind).
		Str("} rows the commit left alone lack their evidence, and the committed count is ").Int(int64(committed)).
		Str(". None of them carries an evidence form, so the line stands below the rows it counts, and the line never rises: give ").
		Int(int64(count - committed)).Str(" of these rows the evidence their kind owes: ")
	writeRowsNamed(&tb, counted)
	return tb.Str(". ").Str(evidenceFormat(kind)).String(), true
}

// writeRowsNamed writes rows comma-separated, at most countedRowsNamedMax of
// them, then how many more there are.
func writeRowsNamed(tb *textbuf.Buffer, rows []string) {
	for i, row := range rows {
		if i == countedRowsNamedMax {
			tb.Str(", and ").Int(int64(len(rows) - countedRowsNamedMax)).Str(" more")
			return
		}
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(row)
	}
}

// evidencePriorRefusal refuses a committed count above the one an earlier
// commit holds for the same kind: HEAD^'s, or the pushed branch's. prior is
// nil when no such commit held the file, and then nothing is compared; at
// names where prior was read.
func evidencePriorRefusal(kind string, committed int, prior map[string]int, at string) (string, bool) {
	held, counted := prior[kind]
	if !counted {
		return "", false
	}
	if committed <= held {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str(evidenceBacklogRel).Str(": the {").Str(kind).Str("} count rose from ").Int(int64(held)).
		Str(" at ").Str(at).Str(" to ").Int(int64(committed)).
		Str("; the committed count never rises above HEAD^'s or the pushed ").Str(backlogRevision).
		Str("'s. Set the line back to `").Str(kind).Byte(' ').Int(int64(held)).
		Str("`, and give each row added or changed since then the evidence its kind owes rather than counting it").String(), true
}

// readEvidenceBacklog reads the committed count of each evidence kind from
// the working tree. An unreadable file is a refusal: a ratchet with no
// committed count judges nothing, and passing it silently is how a guard
// stops guarding.
func readEvidenceBacklog(tree string) (map[string]int, string) {
	raw, err := os.ReadFile(filepath.Join(tree, evidenceBacklogRel)) //nolint:gosec // the path is the checkout's own committed backlog file, not input
	if err != nil {
		return nil, evidenceBacklogRel + ": cannot read the committed evidence counts (" + err.Error() +
			"). Restore the file from git: it holds one `<kind> <count>` line for each of " +
			strings.Join(evidenceKinds, ", ")
	}
	return parseEvidenceBacklog(string(raw), evidenceBacklogRel)
}

// parseHistoricalBacklog is parseEvidenceBacklog over a committed copy of the
// file, with each line of a retired kind (retiredEvidenceKinds) skipped.
func parseHistoricalBacklog(text, source string) (map[string]int, string) {
	var kept strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && slices.Contains(retiredEvidenceKinds, fields[0]) {
			// Keep the line count, so a refusal names the line it read.
			kept.WriteString("\n")
			continue
		}
		kept.WriteString(line)
		kept.WriteString("\n")
	}
	return parseEvidenceBacklog(kept.String(), source)
}

// parseEvidenceBacklog reads one `<kind> <count>` line per kind from text,
// naming source in each refusal. An unknown kind, a malformed count or a
// missing kind is a refusal.
func parseEvidenceBacklog(text, source string) (map[string]int, string) {
	backlog := map[string]int{}
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		where := source + ":" + strconv.Itoa(number+1)
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, where + ": expected `<kind> <count>`, got " + pyRepr(line) + "; write one kind and its count"
		}
		if !slices.Contains(evidenceKinds, fields[0]) {
			return nil, where + ": " + pyRepr(fields[0]) + " is not a kind this ratchet counts; use one of " +
				strings.Join(evidenceKinds, ", ")
		}
		count, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, where + ": the count " + pyRepr(fields[1]) + " is not a whole number; write the number of rows lacking evidence"
		}
		if count < 0 {
			return nil, where + ": the count " + fields[1] + " is negative; write the number of rows lacking evidence"
		}
		if _, twice := backlog[fields[0]]; twice {
			return nil, where + ": a second line counts {" + fields[0] + "}; keep one line per kind"
		}
		backlog[fields[0]] = count
	}
	for _, kind := range evidenceKinds {
		if _, held := backlog[kind]; !held {
			return nil, source + ": no line counts {" + kind + "}; add `" + kind + " <count>`"
		}
	}
	return backlog, ""
}
