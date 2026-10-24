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
			text.Str("  ask Thomas first; once he has approved, run: ").Str(command).Byte('\n')
		}
	}
	if r.Blocking {
		text.Str("  Fix the code by default. Only Thomas approves a test weakening or a change to an ").
			Str("RFC-tagged test, and he is asked BEFORE the edit; the command records his words, never yours.\n")
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
	if request.Exists == nil && request.Tool == "Write" &&
		request.Old == nil && request.OldBase64 == nil {
		if _, currentErr := proposedCurrent(root, path); errors.Is(currentErr, os.ErrNotExist) {
			return report, nil
		}
	}
	oldText, newText, wholeFile, err := proposedTexts(root, path, request)
	if err != nil {
		return ProposedReport{}, err
	}
	hookTest := strings.HasSuffix(path, "_test.go") ||
		(strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et")) &&
			(strings.HasPrefix(path, "test/") || strings.Contains(path, "/test/"))
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
	findings := proposedFindings(path, oldText, newText)
	report.Weakened = findings
	if len(verdict.blocking) != 0 {
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
	} else if len(verdict.advisory) != 0 {
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
	listed, stderr, code, started := gitCapture(root, "ls-tree", "--name-only", headRevision, "--", path)
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
	return path, nil
}

func proposedFindings(path, oldText, newText string) []Finding {
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}
	verdicts := weakenedUnits(path, oldText, newText)
	findings := make([]Finding, 0, len(verdicts))
	for _, verdict := range verdicts {
		findings = append(findings, Finding{
			Path: path, Package: packageName, Name: verdict.name,
			Details: append([]string(nil), verdict.details...),
		})
	}
	return findings
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
			if !rowMatches(row.Name, finding) {
				continue
			}
			if row.Carrier != "" {
				ledger.Spent = append(ledger.Spent, ProposedSpent{Name: name, Carrier: row.Carrier})
				continue
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
