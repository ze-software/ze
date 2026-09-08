// Design: docs/architecture/testing/verify-freshness-scope.md -- a debt row's obligation is discharged by evidence a machine re-derives
// Related: internal/le/commit/debt.go -- the ledger the overlay narrows.
package commit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

// The four ways a debt row's obligation is met. Three are DERIVED from git on
// every read; owner is an attestation and is marked as one wherever it prints,
// so a reader can tell a discharge a machine re-checked from one it cannot.
const (
	kindNotApplicable = "not-applicable"
	kindClosed        = "closed"
	kindReviewed      = "reviewed"
	kindOwner         = "owner"
)

// dischargeKinds is the closed vocabulary, in the order the refusal lists it.
var dischargeKinds = []string{kindNotApplicable, kindClosed, kindReviewed, kindOwner}

// dischargeDirName is the subdirectory of the ledger the records live in. It is
// a directory so readDebtRows, which skips directories, never reads a discharge
// row as a debt row.
const dischargeDirName = "discharged"

// dischargeRequest is one operator claim: the rows it answers, the kind, and
// the evidence that kind's verifier re-reads.
type dischargeRequest struct {
	Shard    string
	Lines    []int
	Kind     string
	Commit   string
	Artifact string
	Owner    string
}

// dischargeRecord is one written row. It holds no verdict, by construction: a
// stored verdict is a claim by whoever last edited the file, and this file is
// committed (ai/rules/principles.md).
type dischargeRecord struct {
	Date     string
	Shard    string
	Line     int
	Digest   string
	Kind     string
	Commit   string
	Artifact string
	Owner    string
}

// dischargeResult is the answer of one discharge pass over one shard.
type dischargeResult struct {
	Shard      string   `json:"shard"`
	Kind       string   `json:"kind"`
	Discharged []int    `json:"discharged-lines,omitempty"`
	Refused    []string `json:"refused,omitempty"`
	Record     string   `json:"record,omitempty"`
}

// dischargeDebt verifies the operator's claim over every named row and writes
// the records only when EVERY one of them verified.
//
// All or nothing is the fail-closed shape: a pass that wrote the rows it liked
// and refused the rest leaves the operator to work out which half landed, and
// the ledger carrying a partial answer to a question the operator asked whole.
func dischargeDebt(root, session string, request dischargeRequest) (dischargeResult, int) {
	result := dischargeResult{Shard: request.Shard, Kind: request.Kind}
	rows, err := readDebtRows(root)
	if err != nil {
		return result, reportDischargeError(err)
	}
	byLine := make(map[int]Debt, len(rows))
	for index := range rows {
		if rows[index].Shard == request.Shard {
			byLine[rows[index].Line] = rows[index]
		}
	}
	if len(byLine) == 0 {
		return result, reportDischargeError(errors.New("no ledger shard " + request.Shard +
			" holds a debt row; name a file under " + debtDir))
	}

	stamp := time.Now().UTC().Format(time.DateOnly)
	records := make([]dischargeRecord, 0, len(request.Lines))
	commits := newCommitCache()
	for _, line := range request.Lines {
		row, exists := byLine[line]
		if !exists {
			result.Refused = append(result.Refused,
				request.Shard+":"+strconv.Itoa(line)+" holds no debt row")
			continue
		}
		record := dischargeRecord{
			Date: stamp, Shard: request.Shard, Line: line, Digest: debtRowDigest(row.Raw),
			Kind: request.Kind, Commit: request.Commit, Artifact: request.Artifact,
			Owner: request.Owner,
		}
		if err := verifyDischarge(root, row, record, commits); err != nil {
			result.Refused = append(result.Refused,
				request.Shard+":"+strconv.Itoa(line)+": "+err.Error())
			continue
		}
		records = append(records, record)
		result.Discharged = append(result.Discharged, line)
	}
	if len(result.Refused) != 0 {
		result.Discharged = nil
		return result, 1
	}
	path, err := writeDischargeRecords(root, session, records)
	if err != nil {
		return result, reportDischargeError(err)
	}
	result.Record = path
	return result, 0
}

// reportDischargeError prints the refusal and answers the exit code for a
// failure the operator cannot fix by naming different rows.
func reportDischargeError(err error) int {
	leaction.ReportError(err)
	return 2
}

// parseDischarge reads the closed keyword grammar into a request, and refuses
// every shape the verifiers below cannot judge.
func parseDischarge(args []string) (dischargeRequest, error) {
	values, err := parseKeywords(args, map[string]keywordRule{
		"shard": {Value: true}, "line": {Value: true, Repeat: true}, "kind": {Value: true},
		"commit": {Value: true}, "artifact": {Value: true}, "owner": {Value: true},
	})
	if err != nil {
		return dischargeRequest{}, err
	}
	request := dischargeRequest{
		Shard: values.one("shard"), Kind: values.one("kind"),
		Commit: strings.TrimSpace(values.one("commit")),
		// The authorisation is trimmed of its surrounding whitespace once, here,
		// and recorded verbatim from this point on.
		Artifact: strings.TrimSpace(values.one("artifact")),
		Owner:    strings.TrimSpace(values.one("owner")),
	}
	// The kind is judged first, so a refusal always lists the closed vocabulary
	// the operator has to pick from.
	if !slices.Contains(dischargeKinds, request.Kind) {
		return dischargeRequest{}, errors.New("kind " + strconv.Quote(request.Kind) +
			" is not one of " + strings.Join(dischargeKinds, ", "))
	}
	if err := checkShardName(request.Shard); err != nil {
		return dischargeRequest{}, err
	}
	if err := checkArtifactPath(request.Artifact); err != nil {
		return dischargeRequest{}, err
	}
	if err := checkRevision(request.Commit); err != nil {
		return dischargeRequest{}, err
	}
	lines, err := parseDischargeLines(values["line"])
	if err != nil {
		return dischargeRequest{}, err
	}
	request.Lines = lines
	if request.Kind == kindOwner && request.Owner == "" {
		return dischargeRequest{}, errors.New("kind owner requires owner <authorisation>, " +
			"and an empty authorisation attests nothing")
	}
	if request.Kind != kindOwner && request.Commit == "" {
		return dischargeRequest{}, errors.New("kind " + request.Kind +
			" requires commit <sha>: the row carries none, so the derivation has nothing to read")
	}
	return request, nil
}

// parseDischargeLines reads the repeated line keyword. A line is an index into
// the shard, so it counts from one, and naming one twice is a mistake rather
// than two discharges of one row.
func parseDischargeLines(declared []string) ([]int, error) {
	if len(declared) == 0 {
		return nil, errors.New("name at least one line <n> of the shard")
	}
	lines := make([]int, 0, len(declared))
	for _, value := range declared {
		line, err := strconv.Atoi(value)
		if err != nil || line < 1 {
			return nil, errors.New("line " + strconv.Quote(value) +
				" is not a line number: a shard's first row is line 1")
		}
		if slices.Contains(lines, line) {
			return nil, errors.New("line " + value + " is named more than once")
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// checkShardName keeps the shard a base name under the ledger directory. The
// value reaches a filesystem path, so a separator or a parent reference is
// refused rather than cleaned.
func checkShardName(shard string) error {
	if shard == "" {
		return errors.New("name the ledger shard: shard <name>")
	}
	if shard != filepath.Base(shard) || shard == "." || shard == ".." ||
		strings.ContainsAny(shard, `/\`+"\x00") {
		return errors.New("shard " + strconv.Quote(shard) +
			" is not a base name under " + debtDir)
	}
	if !strings.HasSuffix(shard, ".md") {
		return errors.New("shard " + strconv.Quote(shard) + " is not a .md ledger shard")
	}
	return nil
}

// checkArtifactPath keeps the artifact inside the checkout. The value is joined
// to the checkout root, so a parent reference or an absolute path would read a
// file outside it, and the operator has no business naming one.
func checkArtifactPath(artifact string) error {
	if artifact == "" {
		return nil
	}
	slashed := filepath.ToSlash(artifact)
	if strings.HasPrefix(slashed, "/") || filepath.IsAbs(artifact) {
		return errors.New("artifact " + strconv.Quote(artifact) +
			" is absolute; name it relative to the checkout root")
	}
	for element := range strings.SplitSeq(slashed, "/") {
		if element == ".." {
			return errors.New("artifact " + strconv.Quote(artifact) +
				" leaves the checkout root")
		}
	}
	return nil
}

// checkRevision keeps the commit an OPERAND rather than a flag. The value is
// handed to git as argv, and git reads a leading dash as an option of its own,
// so a revision starting with one is refused here rather than at the first
// command that happens to accept it.
func checkRevision(revision string) error {
	if revision == "" {
		return nil
	}
	if strings.HasPrefix(revision, "-") {
		return errors.New("commit " + strconv.Quote(revision) +
			" starts with a dash, which git reads as an option")
	}
	if strings.ContainsAny(revision, " \t\n\r\x00") {
		return errors.New("commit " + strconv.Quote(revision) + " is not one revision")
	}
	return nil
}

// debtRowDigest pins a discharge to the exact bytes of the row it answers, so
// an edited row drops its discharge and counts open again.
func debtRowDigest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// applyDischarges overlays every valid discharge record onto the rows, and
// answers the records it could NOT apply.
//
// Each record's verdict is re-derived here, on every read. Nothing is read from
// the record but the kind and the evidence, so a discharge whose evidence stops
// deriving returns its row to open with no file edited (AC-13).
func applyDischarges(root string, rows []Debt) []string {
	records, invalid := readDischargeRecords(root)
	if len(records) == 0 {
		return invalid
	}
	at := make(map[string]int, len(rows))
	for index := range rows {
		at[rows[index].Shard+":"+strconv.Itoa(rows[index].Line)] = index
	}
	commits := newCommitCache()
	for _, record := range records {
		key := record.Shard + ":" + strconv.Itoa(record.Line)
		index, exists := at[key]
		if !exists {
			invalid = append(invalid, key+": the ledger holds no row there")
			continue
		}
		row := &rows[index]
		if record.Digest != debtRowDigest(row.Raw) {
			invalid = append(invalid, key+": the row's digest does not match the record")
			continue
		}
		if err := verifyDischarge(root, *row, record, commits); err != nil {
			invalid = append(invalid, key+": "+err.Error())
			continue
		}
		row.Status = statusDischarged
		row.DischargeKind = record.Kind
		row.DischargeEvidence = record.evidence()
	}
	slices.Sort(invalid)
	return invalid
}

// evidence renders the record's input for a reader of the ledger.
func (r dischargeRecord) evidence() string {
	if r.Kind == kindOwner {
		return "owner: " + r.Owner
	}
	if r.Artifact != "" {
		return "commit " + r.Commit + " artifact " + r.Artifact
	}
	return "commit " + r.Commit
}

// verifyDischarge answers nil when the record's evidence still discharges the
// row, and the reason it does not otherwise. Every branch fails closed: an
// unreadable commit, a missing spec and an unknown kind each leave the row open.
func verifyDischarge(root string, row Debt, record dischargeRecord, commits *commitCache) error {
	switch record.Kind {
	case kindOwner:
		if strings.TrimSpace(record.Owner) == "" {
			return errors.New("the owner discharge carries no authorisation")
		}
		return nil
	case kindNotApplicable:
		facts, err := dischargeCommit(root, row, record, commits)
		if err != nil {
			return err
		}
		return verifyKindNotApplicable(root, row, facts)
	case kindClosed:
		facts, err := dischargeCommit(root, row, record, commits)
		if err != nil {
			return err
		}
		return verifyKindClosed(root, facts)
	case kindReviewed:
		facts, err := dischargeCommit(root, row, record, commits)
		if err != nil {
			return err
		}
		return verifyKindReviewed(root, record, facts)
	}
	return errors.New("discharge kind " + strconv.Quote(record.Kind) + " is not one of " +
		strings.Join(dischargeKinds, ", "))
}

// dischargeCommit reads the named commit and checks it against the row.
func dischargeCommit(root string, row Debt, record dischargeRecord, commits *commitCache) (commitFacts, error) {
	facts, err := commits.read(root, record.Commit)
	if err != nil {
		return commitFacts{}, err
	}
	if len(facts.Paths) == 0 && len(facts.Removed) == 0 {
		return commitFacts{}, errors.New("commit " + facts.SHA +
			" names no file, so there is nothing to derive from")
	}
	return facts, commitCoversRow(row, facts)
}

// commitCoversRow refuses a commit whose subject is unrelated to the row's.
//
// The debt row carries no SHA, so the operator supplies one, and a wrong one
// would derive a true answer about the wrong commit. The row keeps the subject
// of the FIRST commit it covers, and a commit's subject can be extended or
// abbreviated in the cell, so containment either way is the test.
func commitCoversRow(row Debt, facts commitFacts) error {
	subject, _ := debtCovered(row.Subject)
	commitSubject := debtCell(facts.Subject)
	if subject == "" || commitSubject == "" {
		return errors.New("commit " + facts.SHA + " or the row carries no subject to pair on")
	}
	if strings.Contains(commitSubject, subject) || strings.Contains(subject, commitSubject) {
		return nil
	}
	return errors.New("commit " + facts.SHA + " is subject " + strconv.Quote(facts.Subject) +
		", and the row's subject is " + strconv.Quote(row.Subject))
}

// verifyKindNotApplicable re-runs the producer that CREATED the obligation, over
// the commit the operator named. The gate the row names decides which producer
// that is, and a gate a verification can re-run has none: those rows clear
// through `le commit debt-clear` by running the gate.
func verifyKindNotApplicable(root string, row Debt, facts commitFacts) error {
	at := debtGateAt(row.Gate)
	if at < 0 {
		return errors.New("the row names gate " + strconv.Quote(row.Gate) +
			", which debtGates does not declare")
	}
	switch debtGates[at].Key {
	case gateReviewOverride:
		stem, err := closedSpecStem(facts.Paths, facts.Removed)
		if err != nil {
			return err
		}
		if stem != "" {
			return errors.New("commit " + facts.SHA + " closes spec " + stem +
				", so a review was owed for it")
		}
		return nil
	case gateRFCChangeOK:
		changed, err := changedTaggedUnits(root, facts)
		if err != nil {
			return err
		}
		if len(changed) != 0 {
			return errors.New("commit " + facts.SHA + " changes RFC-tagged test(s) " +
				strings.Join(changed, ", "))
		}
		return nil
	}
	return errors.New("kind not-applicable has no derivation for gate " +
		strconv.Quote(row.Gate) + "; a gate a verification re-runs clears through debt-clear")
}

// changedTaggedUnits answers the RFC-tagged units the commit changed, by the
// same reading the owner-approval gate itself performs: a unit that carried a
// tag at the PARENT and whose text moved at the commit.
func changedTaggedUnits(root string, facts commitFacts) ([]string, error) {
	names := make([]string, 0)
	if facts.Parent == "" {
		// A root commit changed no unit that existed before it.
		return names, nil
	}
	for _, path := range append(append([]string{}, facts.Paths...), facts.Removed...) {
		if !rfc.IsTagCarrier(path) {
			continue
		}
		oldText, _, problem := committedText(root, facts.Parent, path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if oldText == "" || !rfcTagPattern.MatchString(oldText) {
			continue
		}
		newText, _, problem := committedText(root, facts.SHA, path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		for _, change := range changedRFCUnits(path, oldText, newText) {
			names = append(names, change.Path+":"+change.Name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// verifyKindClosed answers whether the spec the commit CLOSES recorded a
// review that ran, read out of the spec's own bytes at the closure commit's
// parent. Closure removes the spec, so the parent is where its text lives.
func verifyKindClosed(root string, facts commitFacts) error {
	spec, err := removedSpecPath(facts)
	if err != nil {
		return err
	}
	if spec == "" {
		return errors.New("commit " + facts.SHA + " removes no spec, so it closes nothing")
	}
	text, present, problem := committedText(root, facts.Parent, spec)
	if problem != "" {
		return errors.New(problem)
	}
	if !present {
		return errors.New(spec + " is not in " + facts.SHA + "^, so its Review Gate cannot be read")
	}
	// A spec that carried no implementation owed no review, and the Status at
	// the parent is what says so. Reading the ABSENCE of a gate would say the
	// same thing about a spec whose review is simply missing (AC-20, AC-7b).
	status := specStatus(text)
	if status == "skeleton" || status == "design" {
		return nil
	}
	if status == "" {
		return errors.New(spec + " has no Status row at " + facts.SHA + "^")
	}
	return reviewGateRecorded(spec, text)
}

// verifyKindReviewed judges the review artifact the operator named against the
// bytes of the commit it reviewed, and falls back to the Review Gate the
// closure committed when that artifact is gone.
//
// The fallback is the durable half. Artifacts live under tmp/review/, which is
// untracked and is emptied, so a review recorded months ago has no artifact to
// read. The Review Gate is committed prose the closure preserves for ever.
func verifyKindReviewed(root string, record dischargeRecord, facts commitFacts) error {
	if record.Artifact == "" {
		return verifyKindClosed(root, facts)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(record.Artifact))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return verifyKindClosed(root, facts)
	}
	result := ReviewResult{Artifact: record.Artifact}
	judgeReviewCoverage(&result, string(content), facts.Paths, func(path string) string {
		text, present, problem := committedText(root, facts.SHA, path)
		if problem != "" {
			return "UNREADABLE"
		}
		if !present {
			return "DELETED"
		}
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:])
	})
	if !result.Clean {
		return errors.New(strings.Join(result.Problems, "; "))
	}
	return nil
}

// removedSpecPath answers the path of the spec the commit closes, or "".
func removedSpecPath(facts commitFacts) (string, error) {
	stem, err := closedSpecStem(facts.Paths, facts.Removed)
	if err != nil || stem == "" {
		return "", err
	}
	for _, path := range facts.Removed {
		if specpath.IsSpec(path) && specpath.Stem(filepath.Base(path)) == stem {
			return path, nil
		}
	}
	return "", errors.New("closure stem " + stem + " names no removed spec path")
}

// specStatus answers a spec's Status cell, lowercased, from its metadata table.
func specStatus(spec string) string {
	for line := range strings.SplitSeq(spec, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 4 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cells[1]), "Status") {
			return strings.ToLower(strings.TrimSpace(cells[2]))
		}
	}
	return ""
}

// reviewGateRecorded answers nil when a spec's `## Review Gate` section records
// a review that RAN: a filled artifact reference and a rounds count.
//
// The predicate reads the section's ROWS and never one field label. The verdict
// row is spelled `review_gate.py check` in some closed specs and `review check`
// in others, the two do not sort by date, and a verifier matching one spelling
// reads the other as absent. What every recorded gate carries whatever it calls
// its verdict row is a filled artifact and a count of the rounds it ran.
func reviewGateRecorded(spec, text string) error {
	rows, found := reviewGateRows(text)
	if !found {
		return errors.New(spec + " carries no ## Review Gate section")
	}
	artifact := ""
	rounds := 0
	for _, row := range rows {
		field, value := row[0], row[1]
		lower := strings.ToLower(value)
		if strings.Contains(lower, "not recorded") || strings.Contains(lower, "not run") {
			return errors.New(spec + " records its Review Gate as " + strconv.Quote(value))
		}
		if isTemplatePlaceholder(value) {
			continue
		}
		if strings.Contains(strings.ToLower(field), "artifact") {
			artifact = value
		}
		if strings.Contains(strings.ToLower(field), "round") {
			rounds = leadingCount(value)
		}
	}
	if artifact == "" {
		return errors.New(spec + " has a Review Gate with no artifact reference")
	}
	if rounds < 1 {
		return errors.New(spec + " has a Review Gate with no rounds count")
	}
	return nil
}

// reviewGateRows answers the field and value of every table row under the
// `## Review Gate` heading, and whether the heading was found at all.
func reviewGateRows(text string) ([][2]string, bool) {
	rows := make([][2]string, 0)
	inside := false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if strings.EqualFold(trimmed, "## Review Gate") {
				inside = true
				continue
			}
			if inside && strings.HasPrefix(trimmed, "## ") {
				break
			}
			continue
		}
		if !inside {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) != 4 {
			continue
		}
		field := strings.Trim(strings.TrimSpace(cells[1]), "`")
		value := strings.TrimSpace(cells[2])
		if field == "" || strings.HasPrefix(field, "---") {
			continue
		}
		rows = append(rows, [2]string{field, value})
	}
	return rows, inside
}

// isTemplatePlaceholder answers whether a cell still holds the template's own
// square-bracketed instruction rather than a filled value.
func isTemplatePlaceholder(value string) bool {
	return value == "" || (strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"))
}

// leadingCount answers the first whole number in a cell, or zero.
func leadingCount(value string) int {
	digits := 0
	for digits < len(value) && value[digits] >= '0' && value[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return 0
	}
	count, err := strconv.Atoi(value[:digits])
	if err != nil {
		return 0
	}
	return count
}

// commitFacts is what a discharge re-derives about one commit.
type commitFacts struct {
	SHA     string
	Parent  string
	Subject string
	Paths   []string
	Removed []string
}

// commitCache holds one git read per revision, so a ledger read costs one read
// per commit rather than one per row. Its bound is the number of DISTINCT
// commits the discharge records name, which is the discharged population.
type commitCache struct {
	facts   map[string]commitFacts
	refused map[string]string
}

func newCommitCache() *commitCache {
	return &commitCache{facts: make(map[string]commitFacts), refused: make(map[string]string)}
}

func (c *commitCache) read(root, revision string) (commitFacts, error) {
	if facts, cached := c.facts[revision]; cached {
		return facts, nil
	}
	if problem, cached := c.refused[revision]; cached {
		return commitFacts{}, errors.New(problem)
	}
	facts, err := readCommitFacts(root, revision)
	if err != nil {
		c.refused[revision] = err.Error()
		return commitFacts{}, err
	}
	c.facts[revision] = facts
	return facts, nil
}

// readCommitFacts reads the identity, the subject and the file population of
// one commit. Rename detection is OFF: a spec moved between release buckets is
// a removal and an addition, which is the pair relocatedSpecs reads.
func readCommitFacts(root, revision string) (commitFacts, error) {
	if revision == "" {
		return commitFacts{}, errors.New("name the commit: commit <sha>")
	}
	sha, err := gitOutput(root, "rev-parse", "--verify", "-q", revision+"^{commit}")
	if err != nil {
		return commitFacts{}, errors.New("commit " + strconv.Quote(revision) +
			" does not resolve in this checkout")
	}
	facts := commitFacts{SHA: strings.TrimSpace(sha)}
	// A root commit has no parent, and that is not a failure: nothing existed
	// before it, so every "did this change X" question answers no.
	if parent, err := gitOutput(root, "rev-parse", "--verify", "-q", facts.SHA+"^"); err == nil {
		facts.Parent = strings.TrimSpace(parent)
	}
	shown, err := gitOutput(root, "show", "--no-renames", "--name-status",
		"--format=%s%x00", facts.SHA)
	if err != nil {
		return commitFacts{}, err
	}
	subject, listing, split := strings.Cut(shown, "\x00")
	if !split {
		return commitFacts{}, errors.New("git show " + facts.SHA + " printed no subject")
	}
	facts.Subject = strings.TrimSpace(subject)
	for line := range strings.SplitSeq(listing, "\n") {
		status, path, tabbed := strings.Cut(strings.TrimSpace(line), "\t")
		if !tabbed || path == "" {
			continue
		}
		if strings.HasPrefix(status, "D") {
			facts.Removed = append(facts.Removed, path)
			continue
		}
		facts.Paths = append(facts.Paths, path)
	}
	return facts, nil
}

// gitOutput runs one read-only git query and answers its stdout.
func gitOutput(root string, args ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- fixed Git queries; every value is an argv operand.
	command.Dir = root
	var stdout, complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err,
			strings.TrimSpace(complaint.String()))
	}
	return stdout.String(), nil
}

// dischargePath is the per-session record file. A separate file per session is
// what keeps two discharging sessions off one another's writes, exactly as the
// debt shards do.
func dischargePath(session string) string {
	return filepath.ToSlash(filepath.Join(debtDir, dischargeDirName, session+".md"))
}

// dischargeHeader is the record file's prose and table head, written once.
func dischargeHeader(session string) []string {
	return []string{
		"# Verification-debt discharges -- commit session " + session,
		"",
		"How one debt row's obligation was met. A row here holds the INPUT alone,",
		"the kind and its evidence: every read re-derives the verdict from git, so",
		"nothing on this page is a verdict a reader trusts.",
		"",
		"The row digest pins the discharge to the exact debt row it answers. An",
		"edited row drops its discharge and counts open again.",
		"",
		"Write rows only through `le commit debt-discharge`.",
		"",
		"| Date | Shard | Line | Row digest | Kind | Commit | Artifact | Authorisation |",
		"|------|-------|------|------------|------|--------|----------|---------------|",
	}
}

// writeDischargeRecords appends one row per record under an exclusive lock, and
// answers the record file's path. The caller MUST have verified every record.
func writeDischargeRecords(root, session string, records []dischargeRecord) (string, error) {
	relative := dischargePath(session)
	if len(records) == 0 {
		return relative, nil
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // the explicit write/sync result owns the verdict

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // process exit releases the advisory lock

	content, err := os.ReadFile(path) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return "", err
	}
	lines := dischargeHeader(session)
	if len(content) != 0 {
		lines = strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	}
	for _, record := range records {
		lines = append(lines, record.row())
	}
	rendered := strings.Join(lines, "\n") + "\n"
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	if _, err := file.WriteString(rendered); err != nil {
		return "", err
	}
	if err := file.Truncate(int64(len(rendered))); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	return relative, nil
}

// row renders one record as a markdown table row.
func (r dischargeRecord) row() string {
	var text textbuf.Buffer
	text.Str("| ").Str(r.Date).Str(" | ").Str(r.Shard).Str(" | ").Int(int64(r.Line)).
		Str(" | ").Str(r.Digest).Str(" | ").Str(r.Kind).
		Str(" | ").Str(dischargeCell(r.Commit)).
		Str(" | ").Str(dischargeCell(r.Artifact)).
		Str(" | ").Str(dischargeCell(r.Owner)).Str(" |")
	return text.String()
}

// readDischargeRecords reads every record file, and answers the lines that look
// like records and are not.
func readDischargeRecords(root string) ([]dischargeRecord, []string) {
	dir := filepath.Join(root, filepath.FromSlash(debtDir), dischargeDirName)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{"read " + dischargePath("*") + ": " + err.Error()}
	}
	records := make([]dischargeRecord, 0)
	invalid := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			invalid = append(invalid, entry.Name()+": "+err.Error())
			continue
		}
		for number, line := range strings.Split(string(content), "\n") {
			record, kind, ok := parseDischargeRow(line)
			if !ok {
				continue
			}
			if kind != "" {
				invalid = append(invalid, entry.Name()+":"+strconv.Itoa(number+1)+": "+kind)
				continue
			}
			records = append(records, record)
		}
	}
	return records, invalid
}

// parseDischargeRow reads one line. A line is a RECORD when it has the row's
// cell count and a declared kind, so the table head and the prose are not
// records; a record with an unusable digest or line is a record that is WRONG,
// and it is reported rather than ignored.
func parseDischargeRow(line string) (record dischargeRecord, problem string, isRecord bool) {
	cells := strings.Split(line, "|")
	if len(cells) != 10 {
		return dischargeRecord{}, "", false
	}
	for index := range cells {
		cells[index] = strings.TrimSpace(cells[index])
	}
	if !slices.Contains(dischargeKinds, cells[5]) {
		return dischargeRecord{}, "", false
	}
	number, err := strconv.Atoi(cells[3])
	if err != nil || number < 1 {
		return dischargeRecord{}, "line " + strconv.Quote(cells[3]) + " is not a line number", true
	}
	if !isRowDigest(cells[4]) {
		return dischargeRecord{}, "digest " + strconv.Quote(cells[4]) +
			" is not 64 hexadecimal characters", true
	}
	if err := checkShardName(cells[2]); err != nil {
		return dischargeRecord{}, err.Error(), true
	}
	artifact, commit := dischargeValue(cells[7]), dischargeValue(cells[6])
	if err := checkArtifactPath(artifact); err != nil {
		return dischargeRecord{}, err.Error(), true
	}
	if err := checkRevision(commit); err != nil {
		return dischargeRecord{}, err.Error(), true
	}
	return dischargeRecord{
		Date: cells[1], Shard: cells[2], Line: number, Digest: cells[4], Kind: cells[5],
		Commit: commit, Artifact: artifact, Owner: dischargeValue(cells[8]),
	}, "", true
}

// isRowDigest answers whether a cell is a SHA-256 in lowercase hexadecimal.
func isRowDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for index := range len(value) {
		digit := value[index]
		hexadecimal := (digit >= '0' && digit <= '9') || (digit >= 'a' && digit <= 'f')
		if !hexadecimal {
			return false
		}
	}
	return true
}

// dischargeCell renders a value into one table cell. A pipe and a newline are
// written as their character references, which is how a markdown table carries
// them, so the value round-trips through a cell split on "|" unchanged.
// dischargeValue is the inverse and MUST decode in the opposite order.
func dischargeCell(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "|", "&#124;")
	value = strings.ReplaceAll(value, "\r", "&#13;")
	return strings.ReplaceAll(value, "\n", "&#10;")
}

// dischargeValue reads a cell back. It MUST be called on every cell
// dischargeCell wrote, and it decodes the ampersand last.
func dischargeValue(cell string) string {
	cell = strings.ReplaceAll(cell, "&#10;", "\n")
	cell = strings.ReplaceAll(cell, "&#13;", "\r")
	cell = strings.ReplaceAll(cell, "&#124;", "|")
	return strings.ReplaceAll(cell, "&amp;", "&")
}

// Text renders the discharge answer for a person.
func (r dischargeResult) Text() string {
	var text textbuf.Buffer
	for _, line := range r.Refused {
		text.Str("REFUSED  ").Str(line).Byte('\n')
	}
	if len(r.Discharged) == 0 {
		return text.Str("discharged 0 row(s)\n").String()
	}
	text.Str("discharged ").Int(int64(len(r.Discharged))).Str(" row(s) of ").Str(r.Shard).
		Str(" as ").Str(r.Kind).Str(" -> ").Str(r.Record).Byte('\n')
	if r.Kind == kindOwner {
		text.Str("kind owner is an ATTESTATION: no derivation re-checks it\n")
	}
	return text.String()
}
