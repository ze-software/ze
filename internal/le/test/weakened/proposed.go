// Design: internal/le/hookruntime/writeedit.go -- pre-write weakening and RFC approval hatches
package testweakened

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
)

const (
	ProposedInputLimit = 16 << 20
	proposedFileLimit  = 8 << 20
)

// ProposedRequest is the bounded stdin contract for `le test weakened proposed`.
// Fully reconstructed old/new values win over ToolInput. Base64 fields preserve
// arbitrary bytes and are decoded with the hook's invalid-UTF-8 replacement rule.
type ProposedRequest struct {
	Path string `json:"path"`
	Tool string `json:"tool"`
	// ApprovalSession is the commit namespace whose owner approvals, of a
	// weakening and of an RFC-tagged change, authorize this edit: the main
	// thread's, because Thomas answers the main thread even when a subagent
	// makes the edit (lepath.SessionCommitSession). Empty resolves the running
	// harness's main thread.
	ApprovalSession string            `json:"approval-session,omitempty"`
	Exists          *bool             `json:"exists,omitempty"`
	ToolInput       ProposedToolInput `json:"tool-input"`
	Old             *string           `json:"old,omitempty"`
	New             *string           `json:"new,omitempty"`
	OldBase64       *string           `json:"old-base64,omitempty"`
	NewBase64       *string           `json:"new-base64,omitempty"`
}

// ProposedToolInput carries the native Write/Edit/MultiEdit payload subset.
type ProposedToolInput struct {
	FilePath   string         `json:"file_path,omitempty"`
	Content    string         `json:"content,omitempty"`
	OldString  string         `json:"old_string,omitempty"`
	NewString  string         `json:"new_string,omitempty"`
	ReplaceAll bool           `json:"replace_all,omitempty"`
	Edits      []ProposedEdit `json:"edits,omitempty"`
}

// ProposedEdit is one MultiEdit hunk.
type ProposedEdit struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// ProposedRFCChange is one owner-governed tagged unit changed by the proposal.
type ProposedRFCChange struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// ProposedLedger reports exactly which names one owner approval file accepts.
// Owner marks the RFC approval file, whose rows `./le rfc approve` writes;
// otherwise it is the weakening approval file, whose rows
// `./le test weakened approve` writes. Package qualifies the unit the command
// names.
type ProposedLedger struct {
	Path    string   `json:"path"`
	Owner   bool     `json:"owner,omitempty"`
	Package string   `json:"package,omitempty"`
	Rows    int      `json:"rows"`
	Names   []string `json:"names"`
	Missing []string `json:"missing,omitempty"`
	Matched []Row    `json:"matched,omitempty"`
	// Spent is each name a row covers whose trailer a landed commit already
	// carries. Such a row approves nothing further, so the name is in
	// Missing too, and the refusal names the commit that used it.
	Spent    []ProposedSpent `json:"spent,omitempty"`
	Problems []string        `json:"problems,omitempty"`
}

// ProposedSpent is one name whose approval row a landed commit already used.
type ProposedSpent struct {
	Name    string `json:"name"`
	Carrier string `json:"carrier"`
}

// ProposedReport is the structured pre-write verdict.
type ProposedReport struct {
	Path       string              `json:"path"`
	Tool       string              `json:"tool"`
	Weakened   []Finding           `json:"weakened,omitempty"`
	RFCChanges []ProposedRFCChange `json:"rfc-changes,omitempty"`
	Ledgers    []ProposedLedger    `json:"ledgers,omitempty"`
	Messages   []string            `json:"messages,omitempty"`
	Blocking   bool                `json:"blocking"`
	Notice     bool                `json:"notice"`
}

// ExitCode preserves the hook contract: a permitted edit or count-only notice
// is zero; a governed change without its exact row is two.
func (r ProposedReport) ExitCode() int {
	if r.Blocking {
		return 2
	}
	return 0
}

// Text renders the actionable hook diagnosis.
func (r ProposedReport) Text() string {
	var text textbuf.Buffer
	if !r.Blocking && !r.Notice {
		return text.Str("test-weakened proposed: clean for ").Str(r.Path).Byte('\n').String()
	}
	if r.Blocking {
		text.Str("BLOCKED: proposed test weakening in ").Str(r.Path).Byte('\n')
	} else {
		text.Str("notice: proposed edit lowers a test count in ").Str(r.Path).Byte('\n')
	}
	for _, message := range r.Messages {
		text.Str("  ").Str(message).Byte('\n')
	}
	for index := range r.Ledgers {
		ledger := &r.Ledgers[index]
		for _, problem := range ledger.Problems {
			text.Str("  ").Str(problem).Byte('\n')
		}
		for _, name := range ledger.Missing {
			command := ApproveCommand(ledger.Package + "." + name)
			if ledger.Owner {
				command = rfc.ApproveCommand(ledger.Package + "." + name)
			}
			if carrier := ledger.spentBy(name); carrier != "" {
				text.Str("  the approval naming ").Str(ledger.Package).Byte('.').Str(name).
					Str(" was already used by commit ").Str(carrier).Str(", so it approves nothing further\n")
			}
			text.Str("  hand it to an independent agent running /ze-test-judge; on approval it runs: ").Str(command).Byte('\n')
		}
	}
	if r.Blocking {
		text.Str("  Fix the code by default. Only an independent ze-test-judge agent approves a test weakening or a change to an ").
			Str("RFC-tagged test, BEFORE the edit; the command records its verdict, never the author's reasoning.\n")
	}
	return text.String()
}

// Proposed reads one bounded JSON request and judges the bytes the edit would
// produce without requiring those bytes to exist on disk yet.
func Proposed(root string, input io.Reader) (ProposedReport, error) {
	request, err := decodeProposedRequest(input)
	if err != nil {
		return ProposedReport{}, err
	}
	rawPath := request.Path
	if rawPath == "" {
		rawPath = request.ToolInput.FilePath
	}
	path, err := repositoryPath(root, rawPath)
	if err != nil {
		return ProposedReport{}, err
	}
	report := ProposedReport{Path: path, Tool: request.Tool}
	if request.Exists != nil && !*request.Exists {
		return report, nil
	}
	hookTest := strings.HasSuffix(path, "_test.go") ||
		(strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et")) &&
			(strings.HasPrefix(path, "test/") || strings.Contains(path, "/test/"))
	// A Write of a path the working tree lacks creates the file. A committed
	// file deleted from the working tree and written again carries HEAD's
	// proof, so that Write is judged with HEAD's text of the path as its old
	// text. A Go test file HEAD never held is judged against the committed
	// test files of its directory the working tree lacks, unit by unit name
	// (proposedRenamedFindings), on the Write that creates it and on every
	// whole-file edit after it, so a shell `mv` and a weaker copy under a new
	// name is refused here as an Edit of the moved file is.
	recreated := ""
	renamedWrite := false
	if request.Exists == nil && request.Tool == "Write" &&
		request.Old == nil && request.OldBase64 == nil {
		if _, currentErr := proposedCurrent(root, path); errors.Is(currentErr, os.ErrNotExist) {
			if !hookTest && !rfc.IsTagCarrier(path) {
				return report, nil
			}
			committed, err := proposedRecreated(root, path)
			if err != nil {
				return ProposedReport{}, err
			}
			recreated = committed
			if strings.TrimSpace(committed) == "" {
				recreated = ""
				// A .ci or .et has no unit names to match, so a new one is not
				// set beside a deleted one: the commit gate pairs the two.
				if !hookTest || !strings.HasSuffix(path, ".go") {
					return report, nil
				}
				// refuse-by-default: guards Proposed -- no repository holds the root, so no committed sibling exists to be renamed
				if !rfc.InsideGitCheckout(root) {
					return report, nil
				}
				renamedWrite = true
			}
		}
	}
	oldText, newText, wholeFile, err := proposedTexts(root, path, request)
	if err != nil {
		return ProposedReport{}, err
	}
	if recreated != "" {
		oldText = recreated
	}
	taggedCarrier := rfc.IsTagCarrier(path) && strings.Contains(oldText, "RFC requirement:")
	// The owner's lock protects what HEAD records, so a carrier's HEAD text is
	// read even when the working tree has lost its tag: a tag removed by Bash
	// or by another session still locks its committed unit.
	var committedText string
	if wholeFile && rfc.IsTagCarrier(path) {
		committedText, err = proposedCommitted(root, path)
		if err != nil {
			return ProposedReport{}, err
		}
		if strings.Contains(committedText, "RFC requirement:") {
			taggedCarrier = true
		}
	}
	if !hookTest && !taggedCarrier {
		return report, nil
	}
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}
	if err := validProposedSessions(request); err != nil {
		return ProposedReport{}, err
	}
	session := request.ApprovalSession
	if session == "" {
		resolved, err := lepath.SessionCommitSession(root)
		if err != nil {
			return ProposedReport{}, err
		}
		session = resolved
	}

	var rfcChanges []ProposedRFCChange
	if wholeFile {
		rfcChanges = committedRFCChanges(path, oldText, newText, committedText)
	} else {
		rfcChanges = proposedRFCChanges(path, oldText, newText, proposedWholeFileUnit(path, oldText))
	}
	report.RFCChanges = rfcChanges
	if len(rfcChanges) != 0 {
		names := make([]string, 0, len(rfcChanges))
		for _, change := range rfcChanges {
			names = appendUniqueName(names, change.Name)
			report.Messages = append(report.Messages,
				"RFC-TAGGED test changed: "+change.Name+" ("+strings.Join(change.Tags, ", ")+")")
		}
		ledger := proposedApprovals(root, session, path, packageName, names, true)
		report.Ledgers = append(report.Ledgers, ledger)
		if len(ledger.Missing) != 0 || len(ledger.Problems) != 0 {
			report.Blocking = true
		}
	}

	verdict := detect(oldText, newText, path)
	// A weakening is the loss of proof the committed tree holds, so a whole
	// working-tree file is judged only for the units a committed baseline
	// records. A caller's explicit pair, or a hunk the file does not hold,
	// states its own baseline and is judged whole, as before.
	var findings []Finding
	var blocking bool
	if renamedWrite {
		baseline, err := proposedCommittedUnits(root, path)
		if err != nil {
			return ProposedReport{}, err
		}
		renamed, refused := proposedRenamedFindings(path, oldText, newText, baseline.siblings)
		// The whole-file verdict set the new text beside an empty file, so the
		// messages are the findings' own.
		verdict = detectorVerdict{}
		findings, renamed = mergeRenamedFindings(nil, renamed)
		blocking = refused
		verdict = renamedMessages(verdict, renamed, refused)
	} else {
		var held func(name string) bool
		var baseline committedBaseline
		if wholeFile {
			baseline, err = proposedCommittedUnits(root, path)
			if err != nil {
				return ProposedReport{}, err
			}
			held = baseline.units.holds
		}
		findings, blocking = proposedFindings(path, oldText, newText, held)
		// A path HEAD does not hold has no committed text of its own, so its
		// old text is whatever the working tree holds: a stub written after a
		// shell `mv` would become the baseline, and a weaker copy of a moved
		// unit added to it later would be compared with nothing. Each edit of
		// such a path is also set beside the missing committed siblings, for
		// the units it changes.
		if wholeFile && !baseline.pathHeld {
			renamed, refused := proposedRenamedFindings(path, oldText, newText, baseline.siblings)
			findings, renamed = mergeRenamedFindings(findings, renamed)
			blocking = blocking || refused
			verdict = renamedMessages(verdict, renamed, refused)
		}
	}
	report.Weakened = findings
	// An edit whose every lowered count sits in units no commit holds lowers
	// nothing committed, so it earns no notice either.
	countsCommitted := !wholeFile || len(findings) != 0
	if blocking {
		names := make([]string, 0, len(findings))
		for _, finding := range findings {
			names = appendUniqueName(names, finding.Name)
		}
		if len(names) == 0 {
			names = []string{strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
		}
		// An owner who approved a change to a tagged unit approved its
		// weakening too, so a name the weakening file misses is looked up in
		// the RFC approval file before it is refused.
		ledger := proposedApprovals(root, session, path, packageName, names, false)
		if len(ledger.Missing) != 0 && len(ledger.Problems) == 0 {
			owner := proposedApprovals(root, session, path, packageName, ledger.Missing, true)
			ledger.Missing = owner.Missing
			ledger.Matched = append(ledger.Matched, owner.Matched...)
			ledger.Spent = append(ledger.Spent, owner.Spent...)
			ledger.Problems = owner.Problems
		}
		report.Ledgers = append(report.Ledgers, ledger)
		if len(ledger.Missing) != 0 || len(ledger.Problems) != 0 {
			report.Blocking = true
		}
		report.Messages = append(report.Messages, verdict.blocking...)
		report.Messages = append(report.Messages, verdict.advisory...)
	} else if len(verdict.advisory) != 0 && countsCommitted {
		report.Notice = true
		report.Messages = append(report.Messages, verdict.advisory...)
	}
	return report, nil
}

func decodeProposedRequest(input io.Reader) (ProposedRequest, error) {
	limited := io.LimitReader(input, ProposedInputLimit+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return ProposedRequest{}, err
	}
	if len(content) > ProposedInputLimit {
		return ProposedRequest{}, fmt.Errorf("proposed JSON exceeds %d bytes", ProposedInputLimit)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var request ProposedRequest
	if err := decoder.Decode(&request); err != nil {
		return ProposedRequest{}, fmt.Errorf("decode proposed JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ProposedRequest{}, errors.New("proposed stdin must contain exactly one JSON object")
	}
	return request, nil
}

// proposedTexts answers the old and new text the request judges, and whether
// the old text is the whole working-tree file. Only a whole working-tree file
// can be set beside its HEAD text: a caller's explicit old/new pair states its
// own baseline, and an Edit hunk that the file does not hold is judged as
// hunks, because the tool refuses that edit anyway.
func proposedTexts(root, path string, request ProposedRequest) (string, string, bool, error) {
	hasText := request.Old != nil || request.New != nil
	hasBase64 := request.OldBase64 != nil || request.NewBase64 != nil
	if hasText || hasBase64 {
		if hasText && hasBase64 || (request.Old == nil) != (request.New == nil) ||
			(request.OldBase64 == nil) != (request.NewBase64 == nil) {
			return "", "", false, errors.New("fully reconstructed input requires exactly one old/new or old-base64/new-base64 pair")
		}
		if hasText {
			return partProposedText(*request.Old, *request.New)
		}
		oldBytes, err := base64.StdEncoding.DecodeString(*request.OldBase64)
		if err != nil {
			return "", "", false, fmt.Errorf("decode old-base64: %w", err)
		}
		newBytes, err := base64.StdEncoding.DecodeString(*request.NewBase64)
		if err != nil {
			return "", "", false, fmt.Errorf("decode new-base64: %w", err)
		}
		return partProposedText(validUTF8(oldBytes), validUTF8(newBytes))
	}
	current, currentErr := proposedCurrent(root, path)
	switch request.Tool {
	case "Write":
		if currentErr != nil && !errors.Is(currentErr, os.ErrNotExist) {
			return "", "", false, currentErr
		}
		return wholeProposedText(current, request.ToolInput.Content)
	case "Edit":
		if currentErr == nil && request.ToolInput.OldString != "" && strings.Contains(current, request.ToolInput.OldString) {
			count := 1
			if request.ToolInput.ReplaceAll {
				count = -1
			}
			return wholeProposedText(current, strings.Replace(current, request.ToolInput.OldString, request.ToolInput.NewString, count))
		}
		return partProposedText(request.ToolInput.OldString, request.ToolInput.NewString)
	case "MultiEdit":
		if currentErr == nil {
			after := current
			for _, edit := range request.ToolInput.Edits {
				if edit.OldString == "" || !strings.Contains(after, edit.OldString) {
					return proposedJoinedHunks(request.ToolInput.Edits)
				}
				count := 1
				if edit.ReplaceAll {
					count = -1
				}
				after = strings.Replace(after, edit.OldString, edit.NewString, count)
			}
			return wholeProposedText(current, after)
		}
		return proposedJoinedHunks(request.ToolInput.Edits)
	default:
		return "", "", false, fmt.Errorf("unsupported tool %q; want Write, Edit, or MultiEdit", request.Tool)
	}
}

func proposedJoinedHunks(edits []ProposedEdit) (string, string, bool, error) {
	oldParts := make([]string, len(edits))
	newParts := make([]string, len(edits))
	for index, edit := range edits {
		oldParts[index] = edit.OldString
		newParts[index] = edit.NewString
	}
	return partProposedText(strings.Join(oldParts, "\n"), strings.Join(newParts, "\n"))
}

func proposedCurrent(root, path string) (string, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(path))
	closeErr := repository.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return validUTF8(content), nil
}

// wholeProposedText answers a bounded pair whose old text is the whole
// working-tree file.
func wholeProposedText(oldText, newText string) (string, string, bool, error) {
	oldText, newText, err := boundedProposedText(oldText, newText)
	return oldText, newText, err == nil, err
}

// partProposedText answers a bounded pair whose old text is not the whole
// working-tree file.
func partProposedText(oldText, newText string) (string, string, bool, error) {
	oldText, newText, err := boundedProposedText(oldText, newText)
	return oldText, newText, false, err
}

func boundedProposedText(oldText, newText string) (string, string, error) {
	if len(oldText) > proposedFileLimit || len(newText) > proposedFileLimit {
		return "", "", fmt.Errorf("proposed old/new file exceeds %d bytes", proposedFileLimit)
	}
	return oldText, newText, nil
}

// errCommittedUnreadable marks every failure to read a path's HEAD text. The
// hook refuses the edit on it rather than judging against an empty text.
var errCommittedUnreadable = errors.New("the committed text is unreadable")

// proposedCommitted answers the text HEAD records at path, and the empty text
// when HEAD records no such path: an untracked file has no committed evidence
// to weaken. A Git failure is an error, never an empty text, because an empty
// text would unlock every tagged unit in the file.
func proposedCommitted(root, path string) (string, error) {
	listed, stderr, code, started := gitCapture(root, "--literal-pathspecs", "ls-tree", "--name-only", headRevision, "--", path)
	if !started {
		return "", fmt.Errorf("%w: git could not start for %s", errCommittedUnreadable, path)
	}
	if code != 0 {
		return "", fmt.Errorf("%w: git ls-tree %s failed for %s: %s",
			errCommittedUnreadable, headRevision, path, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(listed) == "" {
		return "", nil
	}
	content, problem := revisionText(root, headRevision, path)
	if problem != "" {
		return "", fmt.Errorf("%w: %s", errCommittedUnreadable, problem)
	}
	if len(content) > proposedFileLimit {
		return "", fmt.Errorf("%w: committed %s exceeds %d bytes", errCommittedUnreadable, path, proposedFileLimit)
	}
	return strings.ToValidUTF8(content, "\uFFFD"), nil
}

// proposedRecreated answers HEAD's text of a path the working tree lacks, for
// a Write that creates it. A root no repository holds (an edit hook fixture
// tree) has no commit that could hold the path, so the Write creates a new
// file and the text is empty. That is decided on what the directory is, by
// the discovery git itself performs (rfc.InsideGitCheckout), never from the
// wording of a git refusal, which a localized git translates. Inside a
// checkout every git failure, an unborn HEAD included, is an error: the Write
// might recreate committed proof, and the hook refuses rather than guess.
func proposedRecreated(root, path string) (string, error) {
	// refuse-by-default: guards Proposed -- no repository holds the root, so no commit holds the path and the Write creates a new file
	if !rfc.InsideGitCheckout(root) {
		return "", nil
	}
	return proposedCommitted(root, path)
}

// committedBaseline is what HEAD records for one edited test path: the units
// of every committed baseline, the HEAD text of each committed sibling that
// joined it (proposedBaselineSiblings), read once, and whether HEAD records
// the path itself.
type committedBaseline struct {
	units    committedUnits
	siblings []committedSibling
	pathHeld bool
}

// committedSibling is one committed test file of the edited path's directory
// that is part of its baseline, with its HEAD text. linked marks a sibling
// still present at its committed path that is the edited file under a second
// name; any other sibling is missing from the working tree. The refusal
// (renamedLines) words its reason from it.
type committedSibling struct {
	path   string
	text   string
	linked bool
}

// committedUnits is what the committed tree proves about one edited test path:
// the names of the units a committed baseline declares, and, under the stem
// key, whether any committed baseline exists at all. A unit named for the
// file stem (evidence outside every function, or a whole non-Go file) is held
// exactly when a baseline exists.
type committedUnits struct {
	names map[string]bool
}

// holds reports whether a committed baseline records the unit name, the
// predicate proposedFindings keeps a finding by.
func (c committedUnits) holds(name string) bool {
	if c.names[name] {
		return true
	}
	return c.names[stemKey(name)]
}

// stemMarker prefixes the stem entry in committedUnits.names. A Go identifier
// never holds it, so the stem entry never collides with a function's name.
const stemMarker = "\x00stem:"

func stemKey(stem string) string { return stemMarker + stem }

// proposedCommittedUnits answers the units HEAD records for path. The
// baseline is HEAD's text at path and, because a test file moved with `git
// mv`, or deleted and written again under another name, keeps the proof it
// carried, the HEAD text of every committed test file of the same kind, in
// the edited file's own directory, that the working tree no longer holds at
// its committed path, or that is the edited file under a second name
// (proposedBaselineSiblings). An untracked file with no such sibling has no
// committed baseline, so nothing in it can be weakened. A Git failure is an
// error: an empty set would unlock every committed unit in the file.
//
// The sibling arm reads one directory, never the tree. A file deleted in
// another directory is another package's proof, and reading it here would
// lock every same-named unit of every edit for as long as it stays deleted,
// and would list every committed file of the checkout on each edit. A move
// across directories is caught at the commit instead: the commit gate pairs
// the deletion with the addition by rename detection and compares the two
// (weakenedTests, comparePath), and an unpaired deletion is judged as the
// loss of every unit it held.
//
// The siblings' HEAD texts are kept, so proposedRenamedFindings judges them
// with no second listing or read.
func proposedCommittedUnits(root, path string) (committedBaseline, error) {
	baseline := committedBaseline{units: committedUnits{names: make(map[string]bool)}}
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	text, err := proposedCommitted(root, path)
	if err != nil {
		return baseline, err
	}
	baseline.units.add(path, stem, text)
	baseline.pathHeld = strings.TrimSpace(text) != ""
	siblings, err := proposedBaselineSiblings(root, path)
	if err != nil {
		return baseline, err
	}
	for _, sibling := range siblings {
		text, err := proposedCommitted(root, sibling.path)
		if err != nil {
			return baseline, err
		}
		baseline.units.add(sibling.path, stem, text)
		sibling.text = text
		baseline.siblings = append(baseline.siblings, sibling)
	}
	return baseline, nil
}

// proposedBaselineSiblings answers the committed test files of path's own
// directory, of path's kind, whose HEAD text is part of path's baseline: each
// one the working tree no longer holds at its committed path (moved, or
// deleted and written again under another name), and each one that is the
// same file as path on disk (a hard link, or a spelling checkoutAlias could
// not map back), so an edit through a second name is judged as the committed
// file. A path that cannot be checked is an error, never read as moved. Each
// answer carries its path and whether it is a second name (linked); the
// caller reads its text.
func proposedBaselineSiblings(root, path string) ([]committedSibling, error) {
	// The committed directory is listed with ls-tree, which reads objects
	// only, and each test path is checked on disk: `git diff HEAD` would
	// refresh, and may rewrite, the index every session shares.
	directory := filepath.Dir(path)
	pathspec := "."
	if directory != "." {
		pathspec = directory + "/"
	}
	listed, stderr, code, started := gitCapture(root,
		"--literal-pathspecs", "ls-tree", "--name-only", "--full-name", "-z", headRevision, "--", pathspec)
	if !started {
		return nil, fmt.Errorf("%w: git could not start to list the committed files of %s", errCommittedUnreadable, directory)
	}
	if code != 0 {
		return nil, fmt.Errorf("%w: git ls-tree %s failed for %s: %s", errCommittedUnreadable, headRevision, directory, strings.TrimSpace(stderr))
	}
	edited, editedFound, err := proposedFileInfo(root, path)
	if err != nil {
		return nil, err
	}
	var siblings []committedSibling
	for sibling := range strings.SplitSeq(listed, "\x00") {
		if sibling == "" || sibling == path || filepath.Dir(sibling) != directory {
			continue
		}
		if !isTestPath(sibling) || filepath.Ext(sibling) != filepath.Ext(path) {
			continue
		}
		present, err := proposedPresent(root, sibling)
		if err != nil {
			return nil, err
		}
		if !present {
			siblings = append(siblings, committedSibling{path: sibling})
			continue
		}
		if !editedFound {
			continue
		}
		info, found, err := proposedFileInfo(root, sibling)
		if err != nil {
			return nil, err
		}
		if found && os.SameFile(edited, info) {
			siblings = append(siblings, committedSibling{path: sibling, linked: true})
		}
	}
	return siblings, nil
}

// proposedFileInfo answers the file a checkout path reaches, symlinks
// followed, and found false when it reaches none: a path the Write creates is
// no alias of anything. Any other failure is an error.
func proposedFileInfo(root, path string) (os.FileInfo, bool, error) {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if err == nil {
		return info, true, nil
	}
	// refuse-by-default: guards proposedBaselineSiblings -- a path that reaches no file is no alias of a committed one
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("%w: check %s in the working tree: %w", errCommittedUnreadable, path, err)
}

// proposedRenamedFindings judges a whole-file edit of a Go test file HEAD
// does not hold: the Write that creates it, and every later Write or Edit.
// Each committed test file of its directory in its baseline
// (proposedBaselineSiblings, read by proposedCommittedUnits) is set beside
// the new text, and a unit is judged only when the new text declares it under
// the same name and the edit changes it: a shell `mv` of a committed test,
// then a weaker copy under a new name, keeps the names it copied, whether the
// copy arrives in the creating Write or in an edit of a stub written first.
// A unit the edit leaves as it was is not this edit's change. It was judged
// here only if a hook-visible edit wrote it after the sibling went missing, or
// through a link to the sibling (a hard-linked sibling never goes missing, so
// its units are judged at that edit); one written by a shell command, or
// while an unlinked sibling still stood, is judged at
// the commit, as an addition beside a deletion. A unit of the missing file that the new text
// does not declare is not judged, so a new test that shares no name with a
// deleted one is clean, wherever the deletion came from: its loss is a
// deletion the commit gate judges. A new unit that does share a name with a
// deleted committed one and proves less than it (a helper both call setUp,
// written by another session) is refused like the Edit of a moved file that
// holds it: the hook cannot tell a copy from a coincidence, and the refusal
// names the approval route.
func proposedRenamedFindings(path, oldText, newText string, siblings []committedSibling) ([]renamedFinding, bool) {
	if len(siblings) == 0 {
		return nil, false
	}
	oldUnits := unitsByName(executableTestText(path, oldText))
	declared := make(map[string]bool)
	for _, unit := range unitsByName(executableTestText(path, newText)) {
		if unit.name == "" || unit.incomplete {
			continue
		}
		if unitUnchanged(oldUnits, unit) {
			continue
		}
		declared[unit.name] = true
	}
	if len(declared) == 0 {
		return nil, false
	}
	held := func(name string) bool { return declared[name] }
	var findings []renamedFinding
	blocking := false
	for _, sibling := range siblings {
		found, refused := proposedFindings(path, sibling.text, newText, held)
		for _, finding := range found {
			findings = append(findings, renamedFinding{finding: finding, sibling: sibling})
		}
		blocking = blocking || refused
	}
	return findings, blocking
}

// renamedFinding is one finding of proposedRenamedFindings with the committed
// sibling whose unit of the same name proved more, so the refusal names that
// file and says why this edit is judged against it. merged marks a finding
// whose unit an earlier finding of the same edit already named.
type renamedFinding struct {
	finding Finding
	sibling committedSibling
	merged  bool
}

// unitUnchanged reports whether units holds unit, complete and with the same
// text. A unit the old text lacks, or holds incomplete, is a change.
func unitUnchanged(units []namedUnit, unit namedUnit) bool {
	for _, old := range units {
		if old.name == unit.name {
			return !old.incomplete && old.text == unit.text
		}
	}
	return false
}

// mergeRenamedFindings adds each renamed finding to findings: one whose unit
// findings already names lends it the details it lacks and is marked merged,
// and any other is appended. It answers the merged findings and every renamed
// one, so the caller words the sibling's reason even for a unit another
// verdict named.
func mergeRenamedFindings(findings []Finding, renamed []renamedFinding) ([]Finding, []renamedFinding) {
	for item := range renamed {
		finding := renamed[item].finding
		index := slices.IndexFunc(findings, func(known Finding) bool { return known.Name == finding.Name })
		if index < 0 {
			findings = append(findings, finding)
			continue
		}
		renamed[item].merged = true
		for _, detail := range finding.Details {
			if !slices.Contains(findings[index].Details, detail) {
				findings[index].Details = append(findings[index].Details, detail)
			}
		}
	}
	return findings, renamed
}

// renamedMessages adds to verdict the lines that explain each renamed
// finding, as a refusal when the renamed judgement blocks and as advice
// otherwise. A merged finding is explained only when it blocks: its unit
// already carries another verdict's message, and an advisory line here could
// raise a notice the edit did not otherwise earn.
func renamedMessages(verdict detectorVerdict, renamed []renamedFinding, blocking bool) detectorVerdict {
	for _, item := range renamed {
		if item.merged && !blocking {
			continue
		}
		lines := renamedLines(item, blocking)
		if blocking {
			verdict.blocking = append(verdict.blocking, lines...)
		} else {
			verdict.advisory = append(verdict.advisory, lines...)
		}
	}
	return verdict
}

// renamedLines words one renamed finding: what is wrong (the unit proves less
// than its namesake in the committed sibling), why this file is judged against
// that sibling, and, for a refusal, both ways out. ProposedReport.Text prints
// the judge's approve command only when the ledger names the unit, so these
// lines do not point at a command that may be absent.
func renamedLines(item renamedFinding, blocking bool) []string {
	name, sibling := item.finding.Name, item.sibling.path
	lines := []string{name + " proves less than " + name + " in committed " + sibling + ": " +
		strings.Join(item.finding.Details, "; ")}
	if item.sibling.linked {
		lines = append(lines, "  why: "+sibling+" is committed, and this file is the same file on disk under another name")
		if blocking {
			lines = append(lines, "  fix: have an independent /ze-test-judge agent approve it")
		}
		return lines
	}
	lines = append(lines, "  why: "+sibling+" is committed and missing from the working tree, so this file is judged as its move")
	if blocking {
		lines = append(lines,
			"  fix: have an independent /ze-test-judge agent approve it,",
			"  or, if the two files are unrelated: "+sibling+" was deleted outside this edit; stop and report it to the main thread, so whoever deleted it commits the deletion first")
	}
	return lines
}

// proposedPresent answers whether the working tree holds a file at a committed
// path. A path that cannot be checked is an error, never read as moved.
func proposedPresent(root, path string) (bool, error) {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("%w: check %s in the working tree: %w", errCommittedUnreadable, path, err)
}

// add records the units one committed text declares. The stem entry is the
// edited path's own: a moved file's residual evidence is judged under the
// new file's stem.
func (c committedUnits) add(path, stem, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	c.names[stemKey(stem)] = true
	if !strings.HasSuffix(path, ".go") {
		return
	}
	for _, unit := range unitsByName(executableTestText(path, text)) {
		if unit.name != "" {
			c.names[unit.name] = true
		}
	}
}

func validUTF8(content []byte) string {
	return strings.ToValidUTF8(string(content), "\uFFFD")
}

func repositoryPath(root, raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") {
		return "", errors.New("proposed path is empty or unsafe")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := raw
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(absoluteRoot, filepath.Clean(path))
		if err != nil {
			return "", err
		}
	}
	path = filepath.Clean(path)
	if path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("proposed path is outside the checkout: %s", raw)
	}
	path = filepath.ToSlash(path)
	if path == ".git" || strings.HasPrefix(path, ".git/") {
		return "", fmt.Errorf("proposed path is a Git internal: %s", raw)
	}
	path, err = checkoutAlias(absoluteRoot, path)
	if err != nil {
		return "", err
	}
	if path == ".git" || strings.HasPrefix(path, ".git/") {
		return "", fmt.Errorf("proposed path reaches a Git internal: %s", raw)
	}
	return path, nil
}

// checkoutAlias answers the checkout path the file at path is, so an edit
// through an alias of a committed test is judged as that test: symlinks are
// resolved, and every existing component is spelled as its directory lists
// it, which turns a case variant on a case-insensitive filesystem
// (pkg/Intent_test.go for pkg/intent_test.go) into the committed spelling. A
// hard link is a second name the filesystem cannot map back; the sibling arm
// of proposedCommittedUnits matches it within its directory. A path whose
// target lies outside the checkout keeps its lexical form: what it reaches is
// no test of this checkout, and it is judged as before.
//
// This is the one place the weakening check resolves an alias, for the edit
// hook (writeWeakening passes the path as the tool named it) and for
// `le test weakened proposed` alike.
func checkoutAlias(root, path string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	// refuse-by-default: guards Proposed -- a root that does not exist holds no file, so no path under it can be an alias
	if errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve the checkout root %s: %w", root, err)
	}
	target, err := resolveExistingPrefix(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, target)
	// refuse-by-default: guards Proposed -- a target no relative path reaches lies outside the checkout, so the lexical path is judged as before
	if err != nil {
		return path, nil //nolint:nilerr // a target outside the checkout is an answer (no alias), not a failure
	}
	relative = filepath.ToSlash(relative)
	if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return path, nil
	}
	return spelledAsListed(resolvedRoot, relative)
}

// resolveExistingPrefix resolves the symlinks of path's longest existing
// prefix and appends the rest, for a Write that creates the file. A lookup
// that fails for any reason but absence is an error: the alias it hides
// would be judged as a new file.
func resolveExistingPrefix(path string) (string, error) {
	rest := ""
	for {
		_, err := os.Lstat(path)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			// Path exists, so a missing file here is the target that the link
			// at path names: say so and give the next step, as the edit hook
			// shows this error.
			if errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("%s is a symlink whose target is missing, so the edit cannot be judged as the file it names (%w)\n"+
					"  Next: stop and report the dangling link to the main thread", path, err)
			}
			if err != nil {
				return "", fmt.Errorf("resolve %s: %w", path, err)
			}
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect %s: %w", path, err)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("no existing ancestor of %s", path)
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}

// spelledAsListed answers relative, a symlink-free path under root, with each
// existing component replaced by the name its directory lists for the same
// file. On a case-sensitive filesystem every existing component is listed as
// spelled, so the path comes back unchanged.
func spelledAsListed(root, relative string) (string, error) {
	parts := strings.Split(relative, "/")
	directory := root
	for index, part := range parts {
		info, err := os.Lstat(filepath.Join(directory, part))
		if errors.Is(err, os.ErrNotExist) {
			break // the rest is created by the Write
		}
		if err != nil {
			return "", fmt.Errorf("inspect %s: %w", filepath.Join(directory, part), err)
		}
		listed, err := listedName(directory, part, info)
		if err != nil {
			return "", err
		}
		parts[index] = listed
		directory = filepath.Join(directory, listed)
	}
	return strings.Join(parts, "/"), nil
}

// listedName answers the entry of directory that is the file name reached:
// name itself when the directory lists it, otherwise the entry that differs
// from it in case alone and is the same file. A name no entry matches is
// answered as spelled, because the filesystem found it under that spelling.
func listedName(directory, name string, info os.FileInfo) (string, error) {
	handle, err := os.Open(directory) //nolint:gosec // a directory of this checkout, on the path being judged
	if err != nil {
		return "", fmt.Errorf("list %s: %w", directory, err)
	}
	names, readErr := handle.Readdirnames(-1)
	closeErr := handle.Close()
	if readErr != nil {
		return "", fmt.Errorf("list %s: %w", directory, readErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("list %s: %w", directory, closeErr)
	}
	if slices.Contains(names, name) {
		return name, nil
	}
	for _, candidate := range names {
		if !strings.EqualFold(candidate, name) {
			continue
		}
		other, err := os.Lstat(filepath.Join(directory, candidate))
		if err != nil {
			return "", fmt.Errorf("inspect %s: %w", filepath.Join(directory, candidate), err)
		}
		if os.SameFile(info, other) {
			return candidate, nil
		}
	}
	return name, nil
}

// proposedFindings answers the weakened units of one proposal and whether any
// of them blocks. held, when not nil, keeps only the units a committed
// baseline records (committedUnits.holds): a unit no commit holds has no
// committed proof to lose.
func proposedFindings(path, oldText, newText string, held func(name string) bool) ([]Finding, bool) {
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}
	verdicts := weakenedUnits(path, oldText, newText)
	findings := make([]Finding, 0, len(verdicts))
	blocking := false
	for _, verdict := range verdicts {
		if held != nil && !held(verdict.name) {
			continue
		}
		blocking = blocking || verdict.blocking
		findings = append(findings, Finding{
			Path: path, Package: packageName, Name: verdict.name,
			Details: append([]string(nil), verdict.details...),
		})
	}
	return findings, blocking
}

// proposedRFCChanges answers the tagged units newText changes in oldText.
// wholeFileUnit makes the file one unit named after its stem; otherwise each
// Go function is a unit. The caller decides the cut, so two calls it compares
// name their units the same way.
func proposedRFCChanges(path, oldText, newText string, wholeFileUnit bool) []ProposedRFCChange {
	if !rfc.IsTagCarrier(path) {
		return nil
	}
	fallback := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if wholeFileUnit {
		tags := rfc.ChangedTags(path, oldText, newText)
		if len(tags) == 0 {
			return nil
		}
		return []ProposedRFCChange{{Name: fallback, Tags: tags}}
	}
	newByName := make(map[string][]string)
	for _, unit := range rfc.FunctionUnits(newText) {
		newByName[unit.Name] = append(newByName[unit.Name], unit.Text)
	}
	changes := make([]ProposedRFCChange, 0)
	for _, unit := range rfc.FunctionUnits(oldText) {
		newUnit := ""
		if len(newByName[unit.Name]) == 1 {
			newUnit = newByName[unit.Name][0]
		}
		tags := rfc.ChangedTags(path, unit.Text, newUnit)
		if len(tags) == 0 {
			continue
		}
		name := unit.Name
		if name == "" {
			name = fallback
		}
		changes = append(changes, ProposedRFCChange{Name: name, Tags: tags})
	}
	sort.Slice(changes, func(left, right int) bool { return changes[left].Name < changes[right].Name })
	return changes
}

// committedRFCChanges answers the units tagged at HEAD that differ from their
// committed text in newText and that this edit touches, oldText being the
// working-tree file. The owner's lock protects committed evidence: a unit the
// author tagged or wrote since HEAD is still theirs to repair, a committed
// unit this edit leaves alone is not this edit's change, whoever changed it in
// the working tree, and a committed tag the working tree has lost still
// counts. The tags reported are the committed ones.
//
// One cut serves all three texts: a file-scope tag in the working tree or at
// HEAD makes the whole file the unit, so a tag written or removed since HEAD
// cannot move a committed unit out from under the lock.
func committedRFCChanges(path, oldText, newText, committedText string) []ProposedRFCChange {
	wholeFileUnit := proposedWholeFileUnit(path, oldText)
	if !wholeFileUnit {
		wholeFileUnit = proposedWholeFileUnit(path, committedText)
	}
	committed := proposedRFCChanges(path, committedText, newText, wholeFileUnit)
	if wholeFileUnit {
		if oldText == newText {
			return nil
		}
		return committed
	}
	fallback := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	kept := make([]ProposedRFCChange, 0, len(committed))
	for _, change := range committed {
		if !slices.Equal(proposedUnitTexts(oldText, change.Name, fallback), proposedUnitTexts(newText, change.Name, fallback)) {
			kept = append(kept, change)
		}
	}
	return kept
}

// proposedUnitTexts answers the text of every Go function in content that
// proposedRFCChanges would report under name.
func proposedUnitTexts(content, name, fallback string) []string {
	var texts []string
	for _, unit := range rfc.FunctionUnits(content) {
		unitName := unit.Name
		if unitName == "" {
			unitName = fallback
		}
		if unitName == name {
			texts = append(texts, unit.Text)
		}
	}
	return texts
}

// proposedWholeFileUnit answers whether content is judged as one unit: a
// carrier that is not Go, or Go carrying a tag at file scope.
func proposedWholeFileUnit(path, content string) bool {
	if rfc.ScopeReader(path) != rfc.ScopeGo {
		return true
	}
	return proposedTagOutsideFunction(path, content)
}

func proposedTagOutsideFunction(path, content string) bool {
	for lineNumber, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, "RFC requirement:") {
			continue
		}
		if rfc.UnitAt(path, content, lineNumber+1).Scope == rfc.ScopeFile {
			return true
		}
	}
	return false
}

// proposedApprovals checks the units an edit changes against one owner
// approval file: the RFC one when owner is set, the weakening one otherwise.
// It reads the file through LoadApprovals, the loader the commit gate uses,
// so a row a landed commit already carries admits nothing here either: the
// unit stays missing and Spent names that commit.
func proposedApprovals(root, session, findingPath, packageName string, names []string, owner bool) ProposedLedger {
	source := WeakeningApprovals(session)
	if owner {
		source = RFCApprovals(session)
	}
	ledger := ProposedLedger{Path: source.Path, Owner: owner, Package: packageName, Names: append([]string(nil), names...)}
	// A problem blocks the edit on its own; the names are still matched
	// against the rows that did parse, so the refusal also names what each
	// unit needs.
	rows, problems := LoadApprovals(root, source)
	ledger.Rows = len(rows)
	ledger.Problems = problems
	for _, name := range names {
		finding := Finding{Path: findingPath, Package: packageName, Name: name}
		matched := false
		for _, row := range rows {
			if row.Target != "" {
				continue
			}
			if !rowCovers(row.Row, finding) {
				continue
			}
			if row.Carrier != "" {
				ledger.Spent = append(ledger.Spent, ProposedSpent{Name: name, Carrier: row.Carrier})
				continue
			}
			if row.Stale != "" {
				continue // the verdict judged another change; the unit stays missing
			}
			ledger.Matched = append(ledger.Matched, row.Row)
			matched = true
			break
		}
		if !matched {
			ledger.Missing = append(ledger.Missing, name)
		}
	}
	return ledger
}

// spentBy answers the commit that already used an approval naming name, or
// empty when no spent row names it.
func (l ProposedLedger) spentBy(name string) string {
	for _, spent := range l.Spent {
		if spent.Name == name {
			return spent.Carrier
		}
	}
	return ""
}

func appendUniqueName(names []string, name string) []string {
	if slices.Contains(names, name) {
		return names
	}
	return append(names, name)
}

// validProposedSessions refuses a stdin-named namespace that is not one,
// before ApprovalPath or rfc.ApprovalPath names a file under tmp/ after it.
func validProposedSessions(request ProposedRequest) error {
	if request.ApprovalSession != "" && !lepath.ValidCommitSession(request.ApprovalSession) {
		return fmt.Errorf("proposed edit: approval-session %q is not a commit namespace (8 lowercase hexadecimal characters)", request.ApprovalSession)
	}
	return nil
}
