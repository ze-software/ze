// Design: docs/features/ai-first.md -- prepare one safe explicit commit
package commit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/hunks"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// notCheckoutDetail says why no native verification judges this commit. The
// tree is not the Ze checkout, so there is no Go in it for a gate to run over
// and no certificate that could cover it.
const notCheckoutDetail = "root is not the Ze checkout, so no native gate judges it"

// Options is the callable commit-preparation contract. Paths and removals are
// explicit and repeatable; no option broadens them to a directory or whole tree.
type Options struct {
	Session string   `json:"session,omitempty"`
	Tag     string   `json:"tag,omitempty"`
	Subject string   `json:"subject"`
	Body    []string `json:"body,omitempty"`
	Files   []string `json:"files,omitempty"`
	// Hunks is each `hunks <path> <patch-file>` pair: a path whose commit
	// carries HEAD's blob with only that patch's hunks applied.
	Hunks          []HunkSelection `json:"hunks,omitempty"`
	Remove         []string        `json:"remove,omitempty"`
	Script         string          `json:"script,omitempty"`
	Append         bool            `json:"append"`
	Replace        bool            `json:"replace"`
	NoTest         string          `json:"no-test,omitempty"`
	ReviewOverride string          `json:"review-override,omitempty"`
	RFCChangeOK    string          `json:"rfc-change-ok,omitempty"`
	DryRun         bool            `json:"dry-run"`
	// Attest is the commit an attestation answers and AttestWords the owner's
	// words. An attestation names no file and no remove path (D-7).
	Attest      string `json:"attest,omitempty"`
	AttestWords string `json:"attest-words,omitempty"`
	// LedgerCarried is one `<sha> <gate key>: <reason>` per person-act row
	// the ledger-deletion commit carries (D-4).
	LedgerCarried []string `json:"ledger-carried,omitempty"`
	// SpecWork is the stem of the spec the preparing session claims, resolved
	// by the action layer (spec.ClaimedSpec), or "" when it claims none. A
	// native commit that changes files carries it as its last trailer,
	// `Spec-work: <stem>`, and a closure gathers the spec's change set by it.
	SpecWork string `json:"spec-work,omitempty"`
}

// Prepared is the structured result printed by `le commit create`.
type Prepared struct {
	Session     string                 `json:"session"`
	Message     string                 `json:"message"`
	Script      string                 `json:"script"`
	Verify      VerificationState      `json:"verify"`
	Review      *ReviewResult          `json:"review,omitempty"`
	NoTest      string                 `json:"no-test,omitempty"`
	DryRun      bool                   `json:"dry-run"`
	MessageText string                 `json:"message-text,omitempty"`
	ScriptText  string                 `json:"script-text,omitempty"`
	Added       []string               `json:"added"`
	Removed     []string               `json:"removed"`
	Weakened    []testweakened.Finding `json:"weakened,omitempty"`
	RFCChanges  []RFCChange            `json:"rfc-changes,omitempty"`
	// RFCApprovals is what the commit carries as RFC-approved: trailers and
	// what its script drops from the session's approval file once it lands.
	RFCApprovals RFCApprovals `json:"rfc-approvals"`
	// WeakeningApprovals is what the commit carries as Weakening-approved:
	// trailers and what its script drops from the session's weakening
	// approval file once it lands.
	WeakeningApprovals testweakened.Approvals `json:"weakening-approvals"`
	// Trailers records the person acts this commit owes: `Review-owed:`,
	// `RFC-approval-owed:` and automatic `Linux-limit-owed:` annotations,
	// plus owner attestations and carried obligations.
	Trailers []string `json:"trailers,omitempty"`
	// Debt is the derived verification debt as this commit is prepared: how
	// many commits owe a scoped verify, and since which scoped record. It is
	// empty outside the Ze checkout.
	Debt string `json:"debt,omitempty"`
	// Stats is how much of each staged path this commit carries, measured
	// against HEAD. See FileStat: it is printed, never enforced.
	Stats []FileStat `json:"stats,omitempty"`
}

// Create validates every gate over the prospective commit, writes the message
// and executable script, and never touches the shared Git index.
func Create(root string, options *Options) (Prepared, error) {
	var result Prepared
	if options.Append && options.Replace {
		return result, refusedKeywords(errors.New("append and replace are mutually exclusive"))
	}
	paths, err := normalizePaths(root, options.Files, validateAddPath)
	if err != nil {
		return result, err
	}
	removed, err := normalizePaths(root, options.Remove, validateRemovePath)
	if err != nil {
		return result, err
	}
	attesting := options.Attest != ""
	if attesting && len(slices.Concat(paths, removed))+len(options.Hunks) != 0 {
		return result, refusedKeywords(errors.New("attest refused with a file or remove path: an attestation must not change the tree, " +
			"because a changed tree moves HEAD off the green tree and owes the verify the attestation exists to spare; " +
			"prepare the attestation alone with ./le commit create subject \"<subject>\" attest <commit> \"<owner words>\", " +
			"and the files in a commit of their own"))
	}
	if attesting {
		if err := refuseAttestAdmission(options); err != nil {
			return result, err
		}
	}
	if !attesting && len(paths) == 0 && len(removed) == 0 && len(options.Hunks) == 0 {
		return result, refusedKeywords(errors.New("at least one file or remove path is required, unless the commit is an attestation: " +
			`attest <commit> "<owner words>"`))
	}
	// The message is judged before any gate runs, so a bad subject is the
	// first refusal; it is composed again below once the gates have answered
	// which RFC approvals it carries as trailers.
	message, err := Message(options.Subject, options.Body, nil)
	if err != nil {
		return result, err
	}
	// The tag is validated here, beside the message, and not where nextTag
	// derives the message path: nextTag runs after every gate, so a tag refused
	// there costs the caller a full gate pass to learn of a typo.
	if err := validateTag(options.Tag); err != nil {
		return result, err
	}
	result.NoTest = strings.TrimSpace(options.NoTest)
	if options.SpecWork != "" {
		if !plainStem(options.SpecWork) {
			return result, fmt.Errorf("the claimed spec stem %q is not one plain name, so it cannot be the %s trailer; "+
				"see the claim with ./le spec current", options.SpecWork, strings.TrimSpace(trailerSpecWork))
		}
	}

	// Every gate below reads Ze's own sources: the test ledger, the RFC-tagged
	// tests, the discovery index, the native verification certificate and the
	// plan/ specs. ZE_REPO_ROOT also names sibling checkouts that PUBLISH Ze
	// rather than build it, the site and the wiki, and none of those sources
	// exists there. A gate then refuses the commit for want of a file that tree
	// never had, or reads a path that means something else in it: gh-pages
	// publishes plan/ pages, which the closure gate read as a spec closing.
	//
	// What still holds in either tree is what protects any tree, and none of it
	// is skipped: the path validation above, the message contract and the
	// private index the generated script commits from.
	// The commit session is resolved before the gates rather than after,
	// because a caller that names its session names the namespace its owner
	// approvals are under too (checkSourceGates). One identity names the script
	// and the message, so a second author cannot reach either.
	session, err := lepath.CommitSession(root, options.Session)
	if err != nil {
		return result, err
	}
	result.Session = session

	// HEAD is read BEFORE any content is taken: the hunks blobs below and the
	// snapshot further down. A peer commit landing after this read is then one
	// the script refuses over, if it touched a named path, even when the
	// snapshot already holds it: a refusal that costs a re-run. Read after,
	// the same commit would be in the recorded HEAD and missing from the
	// content, and the script would write over it in silence. A hunks patch
	// applies to this same HEAD, so the script's refusal covers its base too.
	preparedHead, err := gitOutput(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return result, couldNotRun(fmt.Errorf("read the HEAD this commit is prepared against: %w", err))
	}
	preparedHead = strings.TrimSpace(preparedHead)
	// The selected blobs are built before any gate runs, so every gate judges
	// what the commit carries for a hunks path rather than its working-tree
	// file, which holds hunks the commit leaves behind.
	wholePaths := paths
	hunkPaths, selection, err := selectHunks(root, preparedHead, wholePaths, options.Hunks)
	if err != nil {
		return result, err
	}
	paths = slices.Concat(wholePaths, hunkPaths)

	native := lepath.IsCheckout(root)
	reviewCheck := ""
	switch {
	case attesting:
		// An empty commit changes no byte, so no source or verification gate
		// has anything to judge; the trailer is the whole commit.
		trailer, err := attestationTrailer(root, native, options)
		if err != nil {
			return result, err
		}
		result.Trailers = []string{trailer}
		result.Verify = VerificationState{State: verifyNotApplicable, Detail: "an attestation changes no file"}
		message, err = Message(options.Subject, options.Body, result.Trailers)
		if err != nil {
			return result, err
		}
	case native:
		if err := checkSourceGates(root, options, &result, paths, removed, session, selection); err != nil {
			return result, err
		}
		reviewCheck, err = checkVerificationGates(root, options, &result, paths, removed, selection)
		if err != nil {
			return result, err
		}
		// A spec the commit edits owes its Existing Mechanisms analysis
		// (ai/META-DECISIONS.md, "How agents work"). It runs after the review
		// gate, so a closure still learns first that it owes its review.
		if err := specMechanismsRefusal(root, paths, removed, selection); err != nil {
			return result, err
		}
		carried, err := ledgerCarriedTrailers(root, options.LedgerCarried, paths, removed)
		if err != nil {
			return result, err
		}
		result.Trailers = append(result.Trailers, carried...)
	default:
		if len(options.LedgerCarried) != 0 {
			return result, refusedKeywords(errors.New("ledger-carried refused: this tree is not the Ze checkout, and only the Ze checkout carries the ledger"))
		}
		result.Verify = VerificationState{State: verifyNotApplicable, Detail: notCheckoutDetail}
	}
	if native {
		result.Debt = debtReport(root)
	}

	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return result, err
	}
	tag, messagePath, err := nextTag(root, session, options.Tag)
	if err != nil {
		return result, err
	}
	// nextTag allocates the message file for EVERY tag, named or automatic, so
	// every path it hands back is this call's reservation and this call owns
	// cleaning it. An empty one left behind holds a name for nothing.
	keepReservation := false
	defer func() {
		if keepReservation {
			return
		}
		info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(messagePath)))
		if statErr == nil && info.Size() == 0 {
			_ = os.Remove(filepath.Join(root, filepath.FromSlash(messagePath)))
		}
	}()

	scriptPath, existing, err := targetScript(root, session, tag, options)
	if err != nil {
		return result, err
	}
	// The snapshot is taken HERE, after every gate has judged these paths, so
	// what the script commits is what the gates read. A hunks path carries
	// the entry selectHunks built, which the gates read through selection.
	entries, err := snapshotIndexEntries(root, wholePaths)
	if err != nil {
		return result, err
	}
	for _, path := range hunkPaths {
		entries = append(entries, selection[path].Entry)
	}
	if native && !attesting {
		// This producer MUST read the exact entries and removals commitBlock
		// receives, not files or modes that a peer can still edit.
		owed, err := linuxLimitTrailers(root, preparedHead, entries, removed)
		if err != nil {
			return result, err
		}
		result.Trailers = append(result.Trailers, owed...)
		trailers := slices.Concat(result.RFCApprovals.Trailers, result.WeakeningApprovals.Trailers, result.Trailers)
		// The attribution owes nothing, so it stays out of result.Trailers,
		// which lists person acts, and closes the message.
		if options.SpecWork != "" {
			trailers = append(trailers, trailerSpecWork+options.SpecWork)
		}
		message, err = Message(options.Subject, options.Body, trailers)
		if err != nil {
			return result, err
		}
	}
	block := commitBlock{
		Tag: tag, Subject: strings.TrimSpace(options.Subject), Paths: paths, Removed: removed,
		Selected:     hunkPaths,
		PreparedHead: preparedHead,
		IndexEntries: entries, MessagePath: messagePath, ReviewCheck: reviewCheck,
		Approvals: []approvalPrune{
			{Path: result.RFCApprovals.Path, Dropped: result.RFCApprovals.Dropped},
			{Path: result.WeakeningApprovals.Path, Dropped: result.WeakeningApprovals.Dropped},
		},
	}
	if options.Replace && existing != "" {
		if err := refuseForeignReplace(existing, append(paths, removed...)); err != nil {
			return result, err
		}
	}
	scriptText, err := composeScript(root, existing, scriptPath, session, block, options.Append)
	if err != nil {
		return result, err
	}
	result.Message = messagePath
	result.Script = scriptPath
	result.DryRun = options.DryRun
	result.Added = paths
	result.Stats = fileStats(root, paths, selection)
	result.Removed = removed
	if options.DryRun {
		result.MessageText = message
		result.ScriptText = scriptText
		return result, nil
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(messagePath)), []byte(message), 0o600); err != nil {
		return result, err
	}
	keepReservation = true
	fullScript := filepath.Join(root, filepath.FromSlash(scriptPath))
	if err := os.WriteFile(fullScript, []byte(scriptText), 0o750); err != nil { //nolint:gosec // the prepared commit script is executed, so it needs the owner execute bit
		return result, err
	}
	if err := os.Chmod(fullScript, 0o750); err != nil { //nolint:gosec // the prepared commit script is executed, so it needs the owner execute bit
		return result, err
	}
	return result, nil
}

// checkSourceGates judges the sources a prospective commit carries: the test
// ledger, the RFC-tagged tests, the discovery index and the test-coverage
// obligation. Each one reads a file only the Ze checkout holds, so Create runs
// this over that checkout alone.
func checkSourceGates(root string, options *Options, result *Prepared, paths, removed []string, session string,
	selection hunks.Selection,
) error {
	prospective, problems := testweakened.ProspectiveCommit(root, paths, removed, selection)
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	// An owner approval, of a weakening or of an RFC-tagged change, is keyed
	// on the main thread's namespace, not on the agent's
	// (lepath.SessionCommitSession): Thomas answers the main thread. A caller
	// that named its session named the namespace its approvals are under too.
	approvalSession := session
	if options.Session == "" {
		var err error
		approvalSession, err = lepath.SessionCommitSession(root)
		if err != nil {
			return couldNotRun(fmt.Errorf("resolve this session's owner approval namespace: %w", err))
		}
	}
	weakening := testweakened.Check(testweakened.Request{
		Root: root, ApprovalSession: approvalSession, Paths: paths, Removed: removed,
		RenamePairs: prospective.RenamePairs, Selected: selection,
	})
	result.Weakened = weakening.Findings
	rfcChanges, rfcApprovals, rfcProblems := rfcChangeProblems(root, approvalSession, prospective, selection)
	result.RFCChanges = rfcChanges
	// A unit names its directory by base name only, so one approval of a
	// unit this commit changes in two such directories cannot say which test
	// Thomas approved. That holds for a weakening row, an RFC row and an owed
	// admission alike, so the refusal comes before any of them is judged.
	sites := make([]testweakened.UnitSite, 0, len(weakening.Findings)+len(rfcChanges))
	for _, finding := range weakening.Findings {
		sites = append(sites, testweakened.UnitSite{Unit: finding.Unit(), Path: finding.Path})
	}
	for index := range rfcChanges {
		sites = append(sites, testweakened.UnitSite{Unit: rfcChanges[index].unit(), Path: rfcChanges[index].Path})
	}
	if problems := testweakened.SharedUnitProblems(sites); len(problems) != 0 {
		return errors.New(strings.Join(problems, "\n\n"))
	}
	if len(weakening.Problems) != 0 {
		return errors.New(strings.Join(weakening.Problems, "\n\n"))
	}
	result.WeakeningApprovals = weakening.Approvals
	// An RFC approval row that admitted a weakening is carried as its
	// RFC-approved: trailer too, once, whichever gate used it.
	result.RFCApprovals = mergeRFCApprovals(rfcApprovals, weakening.RFCApprovals)
	if len(rfcProblems) != 0 && strings.TrimSpace(options.RFCChangeOK) == "" {
		return errors.New(strings.Join(rfcProblems, "\n\n"))
	}
	if len(rfcProblems) != 0 {
		// The admission is recorded per unit, so a gate that could not name
		// the units it judged has nothing to record, and admitting the commit
		// would publish an owner approval nobody owes.
		if len(rfcApprovals.Unapproved) == 0 {
			return errors.New(keywordRFCChangeOK + " records one " + strings.TrimSpace(trailerRFCApprovalOwed) +
				" trailer per changed unit, and the gate named no unit:\n" + strings.Join(rfcProblems, "\n\n"))
		}
		for _, unit := range rfcApprovals.Unapproved {
			trailer, err := admissionTrailer(trailerRFCApprovalOwed, unit, options.RFCChangeOK)
			if err != nil {
				return err
			}
			result.Trailers = append(result.Trailers, trailer)
		}
	}
	problems, err := testCoverageProblemsSelected(root, paths, selection)
	if err != nil {
		return err
	}
	if len(problems) != 0 && strings.TrimSpace(options.NoTest) == "" {
		return fmt.Errorf("%s\n  or name no-test with a truthful reason",
			strings.Join(problems, "\n"))
	}
	return nil
}

// mergeRFCApprovals adds to the RFC gate's approvals the rows the weakening
// gate used from the same file, each trailer and each dropped line once.
func mergeRFCApprovals(approvals RFCApprovals, weakening testweakened.Approvals) RFCApprovals {
	for _, trailer := range weakening.Trailers {
		if !slices.Contains(approvals.Trailers, trailer) {
			approvals.Trailers = append(approvals.Trailers, trailer)
		}
	}
	for _, line := range weakening.Dropped {
		if !slices.Contains(approvals.Dropped, line) {
			approvals.Dropped = append(approvals.Dropped, line)
		}
	}
	return approvals
}

// checkVerificationGates judges the commit against the native verification
// certificate and the independent-review artifact. It refuses a closure with
// no clean review and no override, and answers the review re-check line the
// script runs. It never refuses for a missing verification, which is owed
// before a push and the push derives, nor for a red, which is owed at release
// (ai/META-DECISIONS.md, "Ze is pre-release").
func checkVerificationGates(root string, options *Options, result *Prepared, paths, removed []string,
	selection hunks.Selection,
) (string, error) {
	all := append(append([]string{}, paths...), removed...)
	result.Verify = verificationState(root, all)
	if len(selection) != 0 {
		// The manifest hashes each path's working-tree file, and a hunks path's
		// file holds hunks this commit leaves behind, so FRESH would vouch for
		// content the commit does not carry.
		result.Verify = VerificationState{State: "stale", Detail: hunkSelectedDetail}
	}
	base, err := appendBase(root, result.Session, options)
	if err != nil {
		return "", err
	}
	stem, err := closureStemSelected(root, paths, removed, base, selection)
	if err != nil {
		return "", err
	}
	if stem == "" {
		return "", nil
	}
	// The closure review covers the spec's whole change set: every commit
	// attributed to it, every earlier block of this script (commit A, not yet
	// landed), and this commit, which carries little more than the removal.
	population, err := specWorkPopulation(root, stem, "HEAD")
	if err != nil {
		return "", err
	}
	if options.Append {
		earlier, err := appendedPaths(root, result.Session, options)
		if err != nil {
			return "", err
		}
		population.Paths = unique(slices.Concat(population.Paths, earlier))
	}
	covered := unique(slices.Concat(paths, population.Paths))
	review := checkReviewSelected(root, stem, covered, selection)
	review.Commits = population.Commits
	result.Review = &review
	if !review.Clean && strings.TrimSpace(options.ReviewOverride) == "" {
		return "", errors.New(strings.Join(review.Problems, "\n"))
	}
	if !review.Clean {
		trailer, err := admissionTrailer(trailerReviewOwed, stem, options.ReviewOverride)
		if err != nil {
			return "", err
		}
		result.Trailers = append(result.Trailers, trailer)
		return "", nil
	}
	// A hunks path is pinned by its blob and was judged above; the script's
	// re-check hashes the working tree, which holds hunks the commit leaves.
	return reviewCheckCommand(stem, withoutSelected(covered, selection)), nil
}

// admissionTrailer renders the record of one admission: `<key><subject>:
// <reason>`, or `<key><reason>` for an admission about no subject. A trailer
// is one message line the derivation reads back, so a reason spanning lines
// is refused rather than cut.
func admissionTrailer(key, subject, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if strings.ContainsAny(reason, "\r\n") {
		return "", refusedKeywords(fmt.Errorf("the reason given for %s must be one line, because it becomes the %s trailer: %q",
			strings.TrimSpace(key), strings.TrimSuffix(strings.TrimSpace(key), ":"), reason))
	}
	if subject == "" {
		return key + reason, nil
	}
	return key + subject + ": " + reason, nil
}

func normalizePaths(root string, raw []string, validate func(string, string) error) ([]string, error) {
	paths := make([]string, 0, len(raw))
	for _, value := range raw {
		path, err := normalizePath(root, value)
		if err != nil {
			return nil, err
		}
		if err := validate(root, path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return unique(paths), nil
}

// testCoverageProblemsSelected answers whether a commit carrying Go carries a
// test too, judging each hunks path by the selected blob. A path it cannot read is an error that
// no-test cannot waive: the gate cannot say whether that path owes a test, and
// a hunks entry that names no blob would otherwise pass unjudged. A hunks test
// path is read before it counts as the commit's test, for the same reason: an
// entry that names no blob carries no test.
func testCoverageProblemsSelected(root string, paths []string, selection hunks.Selection) ([]string, error) {
	owing := make([]string, 0)
	hasTest := false
	for _, path := range paths {
		if isCommitTestPath(path) && selection.Selected(path) {
			if _, err := selection.Read(root, path); err != nil {
				return nil, couldNotRun(fmt.Errorf("test-coverage gate cannot read %s: %w", path, err))
			}
		}
		if isCommitTestPath(path) {
			hasTest = true
		}
		if !testCoverageRequired(path) {
			continue
		}
		content, err := selection.Read(root, path)
		if err != nil {
			return nil, couldNotRun(fmt.Errorf("test-coverage gate cannot read %s: %w", path, err))
		}
		head := content
		if len(head) > 500 {
			head = head[:500]
		}
		if strings.Contains(string(head), "Code generated") ||
			strings.Contains(string(head), "DO NOT EDIT") {
			continue
		}
		owing = append(owing, path)
	}
	if len(owing) == 0 || hasTest {
		return nil, nil
	}
	return []string{"this commit carries Go and no test:\n  " + strings.Join(owing, "\n  ")}, nil
}

func testCoverageRequired(path string) bool {
	if !strings.HasSuffix(path, ".go") ||
		strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_gen.go") ||
		strings.HasPrefix(path, "vendor/") || strings.Contains(path, "/vendor/") ||
		strings.HasPrefix(path, "cmd/") || strings.Contains(path, "/cmd/") {
		return false
	}
	switch filepath.Base(path) {
	case "register.go", "embed.go", "doc.go":
		return false
	}
	return true
}

func isCommitTestPath(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	return (strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et")) &&
		(strings.HasPrefix(path, "test/") || strings.Contains(path, "/test/"))
}

func targetScript(root, session, tag string, options *Options) (string, string, error) {
	if options.Script != "" {
		relative, err := normalizePath(root, options.Script)
		if err != nil {
			return "", "", err
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			return "", "", refusedKeywords(fmt.Errorf("script does not exist: %s", relative))
		}
		if !strings.Contains(string(content), scriptMarker) {
			return "", "", refusedKeywords(fmt.Errorf("script is not a generated commit script: %s", relative))
		}
		if !options.Append && !options.Replace {
			return "", "", refusedKeywords(fmt.Errorf("%s exists; name append or replace", relative))
		}
		return relative, string(content), nil
	}
	if options.Append {
		pattern := filepath.Join(root, "tmp", "commit-"+session+"-*.sh")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", "", err
		}
		candidates := matches[:0]
		for _, match := range matches {
			content, readErr := os.ReadFile(match) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
			if readErr == nil && strings.Contains(string(content), scriptMarker) {
				candidates = append(candidates, match)
			}
		}
		matches = candidates
		if len(matches) == 0 {
			return "", "", refusedKeywords(errors.New("append: this session has no prepared script"))
		}
		if len(matches) > 1 {
			slices.Sort(matches)
			return "", "", refusedKeywords(fmt.Errorf("append is ambiguous: this session has %d prepared scripts; name script", len(matches)))
		}
		content, err := os.ReadFile(matches[0])
		if err != nil {
			return "", "", err
		}
		relative, err := filepath.Rel(root, matches[0])
		if err != nil {
			return "", "", err
		}
		return filepath.ToSlash(relative), string(content), nil
	}
	relative, err := allocateScript(root, session, tag)
	return relative, "", err
}

func refuseForeignReplace(script string, paths []string) error {
	declared := declaredPaths(script)
	if len(declared) == 0 {
		return nil
	}
	for _, path := range paths {
		if declared[path] {
			return nil
		}
	}
	return refusedKeywords(errors.New("replace refused: script was prepared for a different commit and shares none of these files"))
}

// refuseAttestAdmission refuses an attestation that also carries a create
// keyword the keyword table does not declare WithAttest. The admission
// keywords record why a commit that CHANGES files was let past a gate, and
// no-test and ledger-carried act on a commit's files too; an attestation
// changes no file and runs no gate, so the keyword would be dropped in silence
// and the reason the author gave recorded nowhere. The refusal is derived from
// the create table (createParameters) and its policy (createPolicy), so a
// keyword added to the table is refused until it is declared WithAttest. The
// keywords are judged in sorted order, so two given at once always refuse
// naming the same one.
func refuseAttestAdmission(options *Options) error {
	return refuseUndeclaredWithAttest(options, createParameters, createPolicy)
}

// refuseUndeclaredWithAttest is refuseAttestAdmission over a given table and
// policy, so a test can add a keyword nobody decided without mutating either.
func refuseUndeclaredWithAttest(options *Options, parameters []leaction.Parameter, policy map[string]keywordPolicy) error {
	keywords := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		// A retired keyword never reaches Options: parsing refuses it.
		if parameter.Retired != "" {
			continue
		}
		keywords = append(keywords, parameter.Keyword)
	}
	slices.Sort(keywords)
	for _, keyword := range keywords {
		rule := policy[keyword]
		if rule.WithAttest {
			continue
		}
		// A live keyword with no Given reader, or no policy entry at all,
		// cannot be judged, so the refusal says so rather than letting an
		// attestation carry it.
		if rule.Given == nil {
			return refusedKeywords(errors.New("attest refused: create keyword " + keyword + " has no Given reader, " +
				"so the attest refusal cannot tell whether an attestation carries it; " +
				"give " + keyword + " a Given in createPolicy, in internal/le/commit/actions.go"))
		}
		if !rule.Given(options) {
			continue
		}
		return refusedKeywords(errors.New("attest refused with " + keyword + ": an attestation changes no file and runs no gate, " +
			"so there is nothing for " + keyword + " to act on and its value would be recorded nowhere; " +
			"give " + keyword + " on a commit of its own, and prepare the attestation alone with " +
			`./le commit create subject "<subject>" attest <commit> "<owner words>"`))
	}
	return nil
}

// scriptHeader opens every generated script, a commit script and a push-only
// script alike, so append and replace recognize both by scriptMarker.
//
// The script names the checkout it was prepared for, rather than asking git
// which checkout it is being RUN in. Those are two different trees whenever
// ZE_REPO_ROOT points le at a sibling: the script sits in the site's tmp/
// and is run from the Ze checkout, where `git rev-parse --show-toplevel`
// answered "the Ze checkout" and staged this commit's paths there.
func scriptHeader(root, scriptPath, session string) string {
	return "#!/bin/bash\nset -euo pipefail\n" +
		"cd " + shellQuote(root) + "\n\n" +
		markerLine(scriptMarker, scriptPath+" session="+session) + "\n\n"
}

func composeScript(root, existing, scriptPath, session string, block commitBlock, appendBlock bool) (string, error) {
	body := scriptHeader(root, scriptPath, session)
	if appendBlock && existing != "" {
		if err := refusePushAppend(existing); err != nil {
			return "", err
		}
		if err := refuseBlocksWithoutBookkeeping(scriptPath, existing); err != nil {
			return "", err
		}
		body = strings.TrimRight(existing, "\n") + "\n\n"
	}
	body += renderBlock(block, scriptPath)
	return body, nil
}
