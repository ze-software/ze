// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Related: summary.go -- parseNotApplicable, parseLowerLayer, parseFeatureDeclined: the payloads it judges
// Related: gokrazy/kernel/kernel.config -- the kernel options a linux: layer path is held to
// Related: check_quote.go -- the row quote rule, whose section matcher it shares
// Related: check_baseline.go -- baselineLevels, the HEAD^ fingerprints it compares
package rfc

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
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

// evidenceReader judges the evidence of one corpus's exclusion rows. It reads
// each RFC text, the Go standard library root and Ze's kernel config at most
// once, so a corpus of a thousand rows pays for each source once. Not safe for
// concurrent use.
type evidenceReader struct {
	tree    string
	sources map[string]*quoteSource

	goRead    bool
	goSource  string // $(go env GOROOT)/src
	goRefusal string

	kernelRead    bool
	kernelOptions map[string]bool // option name to enabled (=y or =m)
	kernelRefusal string
}

func newEvidenceReader(tree string) *evidenceReader {
	return &evidenceReader{tree: tree, sources: map[string]*quoteSource{}}
}

// kernelConfigRels are the config fragments Ze's runtime kernel is built
// from, in the order the builder layers them: runtime.config sets options
// over kernel.config, whose entries it names as already carried (its own
// comments on XFRM_USER and NF_TABLES_INET).
var kernelConfigRels = []string{"gokrazy/kernel/kernel.config", "gokrazy/kernel/runtime.config"}

// vendorDir is the directory a vendored module path opens under, in this
// checkout and in the Go source tree alike, where it holds modules the
// standard library vendors rather than standard library packages.
const vendorDir = "vendor"

// kernelIncludePrefix opens a fragment's `# ze-include: <name>` directive,
// which pulls <name>.config from kernelbuilder.DefaultCommonDir.
const kernelIncludePrefix = "# ze-include:"

// kernelOptionRE is the CONFIG_ option a `linux:` layer path names.
var kernelOptionRE = regexp.MustCompile(`^CONFIG_[A-Z0-9_]+$`)

// layerPathGap answers what is wrong with a {lower-layer} layer path, false
// when a reader can check it. A vendored module is held to this tree, a
// kernel option to Ze's kernel config, and a Go package to the toolchain's
// standard library source.
func (reader *evidenceReader) layerPathGap(layerPath string) (string, bool) {
	var tb textbuf.Buffer
	if layerPath == "" {
		return "its layer names no component a reader can check: add linux:CONFIG_<option>, go:<standard library package> or vendor/<module path> to the layer", true
	}
	kind, rel, found := strings.Cut(layerPath, ":")
	if !found {
		kind, rel = vendorDir, layerPath
	}
	if slices.Contains(strings.Split(rel, "/"), "..") {
		return tb.Str("its layer path ").Str(layerPath).Str(" climbs out of its source tree with `..`; name the component itself").String(), true
	}
	switch kind {
	case vendorDir:
		if _, err := os.Stat(filepath.Join(reader.tree, rel)); err != nil {
			return tb.Str("its layer path ").Str(layerPath).
				Str(" cannot be opened in this checkout (").Str(err.Error()).
				Str("); name the vendored module directory under vendor/").String(), true
		}
	case "linux":
		return reader.kernelOptionGap(layerPath, rel)
	case "go":
		return reader.goPackageGap(layerPath, rel)
	default:
		panic("BUG: layerPathRE admitted an unknown layer path form")
	}
	return "", false
}

// kernelOptionGap answers what is wrong with a `linux:` layer path: it must
// name a CONFIG_ option that Ze's kernel config enables, built in or as a
// module.
func (reader *evidenceReader) kernelOptionGap(layerPath, option string) (string, bool) {
	var tb textbuf.Buffer
	if !kernelOptionRE.MatchString(option) {
		return tb.Str("its layer path ").Str(layerPath).
			Str(" names no CONFIG_ option; write linux:CONFIG_<option>, as linux:CONFIG_IPV6, naming the option of ").
			Str(strings.Join(kernelConfigRels, " or ")).Str(" that builds the layer").String(), true
	}
	options, refusal := reader.enabledKernelOptions()
	if refusal != "" {
		return refusal, true
	}
	if !options[option] {
		return tb.Str("its layer path ").Str(layerPath).Str(" is not enabled in Ze's kernel (").
			Str(strings.Join(kernelConfigRels, ", ")).
			Str(" set it to neither y nor m), so no kernel Ze ships performs it; name the option that builds the layer, or enable it in the config").String(), true
	}
	return "", false
}

// goPackageGap answers what is wrong with a `go:` layer path: it must name a
// package directory under the toolchain's standard library source.
func (reader *evidenceReader) goPackageGap(layerPath, pkg string) (string, bool) {
	source, refusal := reader.goStandardSource()
	if refusal != "" {
		return refusal, true
	}
	top, _, _ := strings.Cut(pkg, "/")
	info, err := os.Stat(filepath.Join(source, pkg))
	standard := err == nil && info.IsDir() && pkg != "" && top != "cmd" && top != vendorDir
	if standard {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str("its layer path ").Str(layerPath).Str(" is not a package of the Go standard library (no directory ").
		Str(filepath.Join(source, pkg)).
		Str("); name the package as go:<import path>, and a module from outside the standard library as vendor/<module path>").String(), true
}

// goStandardSource answers $(go env GOROOT)/src, asking the toolchain once.
func (reader *evidenceReader) goStandardSource() (string, string) {
	if reader.goRead {
		return reader.goSource, reader.goRefusal
	}
	reader.goRead = true
	cmd := exec.Command("go", "env", "GOROOT") //nolint:gosec,noctx // this developer tool asks its own toolchain where the standard library is
	cmd.Dir = reader.tree
	out, err := cmd.Output()
	root := strings.TrimSpace(string(out))
	if err != nil {
		reader.goRefusal = "cannot ask the Go toolchain for its standard library (`go env GOROOT`: " + err.Error() +
			"), so no go: layer path can be checked; put go on PATH and rerun `./le rfc check`"
		return "", reader.goRefusal
	}
	if root == "" {
		reader.goRefusal = "`go env GOROOT` answered nothing, so no go: layer path can be checked; set GOROOT and rerun `./le rfc check`"
		return "", reader.goRefusal
	}
	reader.goSource = filepath.Join(root, "src")
	return reader.goSource, ""
}

// enabledKernelOptions answers every option Ze's kernel config sets, mapped
// to whether it is enabled, reading each fragment and its one level of
// includes once. A later fragment overrides an earlier one, as the builder
// layers them. A fragment that cannot be read is a refusal: the option
// cannot be told absent from unreadable.
func (reader *evidenceReader) enabledKernelOptions() (map[string]bool, string) {
	if reader.kernelRead {
		return reader.kernelOptions, reader.kernelRefusal
	}
	reader.kernelRead = true
	options := map[string]bool{}
	for _, rel := range kernelConfigRels {
		includes, refusal := readKernelFragment(reader.tree, rel, options)
		if refusal != "" {
			reader.kernelRefusal = refusal
			return nil, refusal
		}
		for _, name := range includes {
			included := kernelbuilder.DefaultCommonDir + "/" + name + ".config"
			if _, refusal := readKernelFragment(reader.tree, included, options); refusal != "" {
				reader.kernelRefusal = refusal
				return nil, refusal
			}
		}
	}
	reader.kernelOptions = options
	return options, ""
}

// readKernelFragment records each `CONFIG_<name>=<value>` line of one config
// fragment into options and answers the names its include directives pull
// in, or a refusal naming the file when it cannot be read.
func readKernelFragment(tree, rel string, options map[string]bool) ([]string, string) {
	raw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(rel))) //nolint:gosec // the path is the checkout's own kernel config fragment, not input
	if err != nil {
		return nil, "cannot read " + rel + " (" + err.Error() +
			"), so no linux: layer path can be checked against Ze's kernel config; restore the file from git"
	}
	var includes []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if name, include := strings.CutPrefix(line, kernelIncludePrefix); include {
			includes = append(includes, strings.TrimSpace(name))
			continue
		}
		name, value, assigned := strings.Cut(line, "=")
		if !assigned || !strings.HasPrefix(name, "CONFIG_") {
			continue
		}
		options[name] = value == "y" || value == "m"
	}
	return includes, ""
}

// gap answers the evidence one row's exclusion kind owes and does not carry,
// and true; "" and false when the row carries it or no evidence kind
// annotates it.
//
// The row's own text is already the RFC's sentence (checkRowQuotes); what each
// kind adds is the fact that removes the row from Ze's count (owner ruling,
// 2026-10-29): for {not-applicable} a role-condition quote from the cited
// section, the role it binds and the role Ze fills, for {feature-declined}
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
				Str(", names no role that quote binds and no role ze fills").String(), true
		}
		if req.Annotation.BoundRole == "" || req.Annotation.Role == "" {
			// Unreachable through the parser, which refuses a quote with no
			// bound role or no ze role. The check still refuses it: the roles
			// are half the fact.
			return "it names no role the quote binds or no role ze fills", true
		}
		return quoteGap(req, loadQuoteSource(reader.sources, reader.tree, req.RFC), "role-condition quote")
	case AnnotationFeatureDeclined:
		return quoteGap(req, loadQuoteSource(reader.sources, reader.tree, req.RFC), "optionality quote")
	case AnnotationLowerLayer:
		return reader.layerPathGap(req.Annotation.LayerPath)
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

// priorState is what git answers about HEAD^, the revision the ratchet
// compares against. Zero is no answer at all, which no caller may pass.
type priorState int

const (
	priorUnspecified priorState = iota
	// priorAbsent is a checkout with no HEAD^ to read: no git, no commit, or
	// a root commit. Nothing is known to be new, so every row is counted.
	priorAbsent
	// priorRead is a HEAD^ whose summaries and committed counts were read.
	priorRead
	// priorUnreadable is a HEAD^ git holds and could not answer for. It is a
	// refusal, never the absent case: a broken reader that passes is how a
	// ratchet stops judging.
	priorUnreadable
)

// evidenceBaseline is HEAD^ as the exclusion evidence ratchet reads it.
type evidenceBaseline struct {
	state priorState
	// reason says why the state is absent or unreadable.
	reason string
	// prints is each requirement id's evidenceFingerprint at HEAD^, and
	// unreadable the stems whose summary HEAD^ could not parse.
	prints     map[string]string
	unreadable map[string]bool
	// backlog is the committed count of each kind at HEAD^, nil when HEAD^
	// held no backlog file (the commit that introduced it).
	backlog map[string]int
}

// priorRevisionState answers whether HEAD^ exists, telling a checkout that
// has none from one git cannot read. A tree under no `.git` and a HEAD with
// no parent are absent; `git rev-parse --verify --quiet` exits 1 for a
// revision that does not resolve and with another status when git itself
// fails, which is unreadable.
func priorRevisionState(tree string) (priorState, string) {
	if !insideGitCheckout(tree) {
		return priorAbsent, tree + " is not a git checkout"
	}
	var tb textbuf.Buffer
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", //nolint:gosec,noctx // this developer tool queries the checkout it was given
		tb.Str(priorRevision).Str("^{commit}").String())
	cmd.Dir = tree
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return priorRead, ""
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return priorAbsent, "HEAD has no parent commit (a root commit, or no commit yet)"
	}
	return priorUnreadable, "`git rev-parse --verify HEAD^` failed: " + strings.TrimSpace(err.Error()+" "+stderr.String())
}

// insideGitCheckout answers whether tree or a directory above it holds a
// `.git` entry: a directory in a clone, a file in a worktree. A path that
// cannot be made absolute answers true, so git is asked and answers
// unreadable rather than the checkout passing as absent.
func insideGitCheckout(tree string) bool {
	dir, err := filepath.Abs(tree)
	if err != nil {
		return true
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// priorEvidence reads HEAD^ for the ratchet: the fingerprints
// baselineLevels already parsed, and the committed counts of the backlog
// file at HEAD^, which is how a commit that raises a line is caught.
func priorEvidence(tree string, prints map[string]string, unreadable map[string]bool, levelsKnown bool) evidenceBaseline {
	state, reason := priorRevisionState(tree)
	if state != priorRead {
		return evidenceBaseline{state: state, reason: reason}
	}
	if !levelsKnown {
		return evidenceBaseline{state: priorUnreadable, reason: "git could not read the summaries under " + summaryRel + " at HEAD^"}
	}
	blobs, ok := gitCatBlobs(tree, priorRevision, []string{evidenceBacklogRel})
	if !ok {
		return evidenceBaseline{state: priorUnreadable, reason: "`git cat-file` could not read HEAD^:" + evidenceBacklogRel}
	}
	base := evidenceBaseline{state: priorRead, prints: prints, unreadable: unreadable}
	text, held := blobs[evidenceBacklogRel]
	if !held {
		return base
	}
	backlog, refusal := parseEvidenceBacklog(text, "HEAD^:"+evidenceBacklogRel)
	if refusal != "" {
		return evidenceBaseline{state: priorUnreadable, reason: refusal}
	}
	base.backlog = backlog
	return base
}

// lostRowsNamedMax bounds how many rows a count refusal names in full.
const lostRowsNamedMax = 10

// checkExclusionEvidence is the exclusion evidence ratchet. It answers the
// refusals, and a notice for the report when HEAD^ is absent.
//
// A row the commit added or changed against HEAD^ that lacks its evidence is
// refused by its id and is not counted: it is this commit's to fix, and
// counting it too would bill the backlog for it. Every other row lacking
// evidence is counted per kind, and that count must equal the committed count
// in evidenceBacklogRel: above it, a row lost its evidence; below it, the
// committed count is lowered in the same commit. The committed count itself
// may not rise above HEAD^'s. A kind whose committed count is zero is refused
// row by row, which is full refusal. A HEAD^ git cannot read is refused; only
// an absent HEAD^ judges every row as untouched, and the notice says so.
func checkExclusionEvidence(tree string, requirements []Requirement, base evidenceBaseline) ([]string, string) {
	backlog, refusal := readEvidenceBacklog(tree)
	if refusal != "" {
		return []string{refusal}, ""
	}
	var notice string
	switch base.state {
	case priorRead:
	case priorAbsent:
		notice = "exclusion evidence: no HEAD^ (" + base.reason + "), so no row is known to be new: every row lacking evidence is counted against " +
			evidenceBacklogRel + " and no committed count is compared with HEAD^"
	case priorUnreadable:
		return []string{evidenceBacklogRel + ": cannot read HEAD^ (" + base.reason +
			"), so no row can be told new from untouched and the committed counts cannot be compared with HEAD^. " +
			"Repair the checkout until `git cat-file -p HEAD^:" + evidenceBacklogRel + "` answers, then rerun `./le rfc check`"}, ""
	case priorUnspecified:
		panic("BUG: checkExclusionEvidence was given no HEAD^ state")
	default:
		panic("BUG: unknown HEAD^ state")
	}
	reader := newEvidenceReader(tree)
	lacking := map[string]int{}
	lost := map[string][]string{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		missing, lacks := reader.gap(req)
		if !lacks {
			continue
		}
		kind := req.Annotation.Kind
		touched := evidenceRowIsNew(req, base)
		if !touched {
			lacking[kind]++
			if req.Annotation.Quote != "" || req.Annotation.LayerPath != "" {
				lost[kind] = append(lost[kind], requirementWhere(*req)+" "+req.RID)
			}
		}
		if !touched && backlog[kind] > 0 {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(requirementWhere(*req)).Str(": ").Str(req.RID).
			Str(" is annotated {").Str(kind).Str("} and lacks the evidence that kind owes: ").
			Str(missing).Str(". Rewrite the annotation on that line to carry it. ").Str(evidenceFormat(kind)).String())
	}
	for _, kind := range evidenceKinds {
		if message, refused := evidenceCountRefusal(kind, lacking[kind], backlog[kind], lost[kind]); refused {
			errs = append(errs, message)
		}
		if message, refused := evidencePriorRefusal(kind, backlog[kind], base.backlog); refused {
			errs = append(errs, message)
		}
	}
	return errs, notice
}

// evidenceRowIsNew answers whether the commit added or changed this row's
// evidence against HEAD^. A row of a summary HEAD^ could not parse is not
// known to be new, and neither is any row when HEAD^ is absent.
func evidenceRowIsNew(req *Requirement, base evidenceBaseline) bool {
	if base.state != priorRead {
		return false
	}
	if base.unreadable[req.RFC] {
		return false
	}
	held, existed := base.prints[req.RID]
	// refuse-by-default: guards checkExclusionEvidence -- an id HEAD^ never held is a row this commit added
	if !existed {
		return true
	}
	return held != evidenceFingerprint(req)
}

// evidenceCountRefusal compares one kind's count of untouched rows lacking
// evidence with its committed count. lost names the counted rows that carry
// an evidence form which no longer holds.
func evidenceCountRefusal(kind string, count, committed int, lost []string) (string, bool) {
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
	tb.Str(evidenceBacklogRel).Str(": ").Int(int64(count)).Str(" {").Str(kind).
		Str("} rows the commit left alone lack their evidence, and the committed count is ").Int(int64(committed)).Str(". ")
	if len(lost) == 0 {
		return tb.Str("None of them carries an evidence form, so the line was set below the rows it counts: set it back to `").
			Str(kind).Byte(' ').Int(int64(count)).Str("`").String(), true
	}
	tb.Str("The evidence these rows carry no longer holds, because the RFC text or the component they name changed under them: ")
	for i, row := range lost {
		if i == lostRowsNamedMax {
			tb.Str(", and ").Int(int64(len(lost) - lostRowsNamedMax)).Str(" more")
			break
		}
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(row)
	}
	return tb.Str("; restore what each names, or correct its annotation, then rerun `./le rfc check`").String(), true
}

// evidencePriorRefusal refuses a committed count above HEAD^'s for the same
// kind. prior is nil when HEAD^ held no backlog file, and then nothing is
// compared.
func evidencePriorRefusal(kind string, committed int, prior map[string]int) (string, bool) {
	held, counted := prior[kind]
	if !counted {
		return "", false
	}
	if committed <= held {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str(evidenceBacklogRel).Str(": the {").Str(kind).Str("} count rose from ").Int(int64(held)).
		Str(" at HEAD^ to ").Int(int64(committed)).
		Str("; the committed count never rises. Set the line back to `").Str(kind).Byte(' ').Int(int64(held)).
		Str("`, and give each row this commit added or changed the evidence its kind owes rather than counting it").String(), true
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
		backlog[fields[0]] = count
	}
	for _, kind := range evidenceKinds {
		if _, held := backlog[kind]; !held {
			return nil, source + ": no line counts {" + kind + "}; add `" + kind + " <count>`"
		}
	}
	return backlog, ""
}
