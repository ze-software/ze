// Design: docs/features/ai-first.md -- prepare one safe explicit commit
package commit

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	Session         string   `json:"session,omitempty"`
	Tag             string   `json:"tag,omitempty"`
	Subject         string   `json:"subject"`
	Body            []string `json:"body,omitempty"`
	Files           []string `json:"files,omitempty"`
	Remove          []string `json:"remove,omitempty"`
	Script          string   `json:"script,omitempty"`
	Append          bool     `json:"append"`
	Replace         bool     `json:"replace"`
	NoTest          string   `json:"no-test,omitempty"`
	StructuralRedOK string   `json:"structural-red-ok,omitempty"`
	ReviewOverride  string   `json:"review-override,omitempty"`
	BrokenHeadFix   string   `json:"broken-head-fix,omitempty"`
	RFCChangeOK     string   `json:"rfc-change-ok,omitempty"`
	DryRun          bool     `json:"dry-run"`
	// Attest is the commit an attestation answers and AttestWords the owner's
	// words. An attestation names no file and no remove path (D-7).
	Attest      string `json:"attest,omitempty"`
	AttestWords string `json:"attest-words,omitempty"`
	// LedgerCarried is one `<sha> <gate key>: <reason>` per person-act row
	// the ledger-deletion commit carries (D-4).
	LedgerCarried []string `json:"ledger-carried,omitempty"`
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
	// Trailers is the record of each admission a gate granted this commit:
	// `Review-owed:` and `RFC-approval-owed:`, which the push derivation reads
	// as owed person acts, and the audit-only `Structural-red-ok:` and
	// `Broken-head-fix:`.
	Trailers []string `json:"trailers,omitempty"`
	// Debt is the derived verification debt as this commit is prepared: how
	// many commits owe a full verify, and since which green record. It is
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
		return result, errors.New("append and replace are mutually exclusive")
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
	if attesting && len(slices.Concat(paths, removed)) != 0 {
		return result, errors.New("attest refused with a file or remove path: an attestation must not change the tree, " +
			"because a changed tree moves HEAD off the green tree and owes the full verify the attestation exists to spare; " +
			"prepare the attestation alone with ./le commit create subject \"<subject>\" attest <commit> \"<owner words>\", " +
			"and the files in a commit of their own")
	}
	if attesting {
		if err := refuseAttestAdmission(options); err != nil {
			return result, err
		}
	}
	if !attesting && len(paths) == 0 && len(removed) == 0 {
		return result, errors.New("at least one file or remove path is required, unless the commit is an attestation: " +
			`attest <commit> "<owner words>"`)
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
	// because the ledger shards those gates read are named after it. One
	// identity names the script, the message and the two ledger shards, so a
	// second author cannot reach any of them.
	session, err := lepath.CommitSession(root, options.Session)
	if err != nil {
		return result, err
	}
	result.Session = session

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
		if err := checkSourceGates(root, options, &result, paths, removed, session); err != nil {
			return result, err
		}
		reviewCheck, err = checkVerificationGates(root, options, &result, paths, removed)
		if err != nil {
			return result, err
		}
		carried, err := ledgerCarriedTrailers(root, options.LedgerCarried, paths, removed)
		if err != nil {
			return result, err
		}
		result.Trailers = append(result.Trailers, carried...)
		// The message is composed once every gate has answered which
		// trailers it carries: the RFC approvals it uses, then the record of
		// each admission the gates granted.
		trailers := slices.Concat(result.RFCApprovals.Trailers, result.Trailers)
		message, err = Message(options.Subject, options.Body, trailers)
		if err != nil {
			return result, err
		}
	default:
		if len(options.LedgerCarried) != 0 {
			return result, errors.New("ledger-carried refused: this tree is not the Ze checkout, and only the Ze checkout carries the ledger")
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
	// HEAD is read BEFORE the snapshot. A peer commit landing between the two
	// is then one the script refuses over, if it touched a named path, even
	// though the snapshot may already hold it: a refusal that costs a re-run.
	// Read after, the same commit would be in the recorded HEAD and missing
	// from the snapshot, and the script would write over it in silence.
	preparedHead, err := gitOutput(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return result, fmt.Errorf("read the HEAD this commit is prepared against: %w", err)
	}
	// The snapshot is taken HERE, after every gate has judged these paths, so
	// what the script commits is what the gates read.
	entries, err := snapshotIndexEntries(root, paths)
	if err != nil {
		return result, err
	}
	block := commitBlock{
		Tag: tag, Subject: strings.TrimSpace(options.Subject), Paths: paths, Removed: removed,
		PreparedHead: strings.TrimSpace(preparedHead),
		IndexEntries: entries, MessagePath: messagePath, ReviewCheck: reviewCheck,
		ApprovalsPath: result.RFCApprovals.Path, ApprovalsDropped: result.RFCApprovals.Dropped,
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
	result.Stats = fileStats(root, paths)
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
func checkSourceGates(root string, options *Options, result *Prepared, paths, removed []string, session string) error {
	// A main thread commits the work its subagents did, so their shards are
	// its to carry; every other session's shard is foreign. Each subagent owns
	// its commit namespace (lepath.CommitSession keys it on ZE_AGENT_ID).
	children, err := lepath.ChildCommitSessions(root)
	if err != nil {
		return fmt.Errorf("resolve this session's subagent commit namespaces: %w", err)
	}
	if problems := testweakened.ForeignShardProblems(session, children, slices.Concat(paths, removed)); len(problems) != 0 {
		return errors.New(strings.Join(problems, "\n\n"))
	}
	prospective, problems := testweakened.ProspectiveCommit(root, paths, removed)
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	carried := testweakened.CarriedShards(session, children, paths)
	weakening := testweakened.CheckCommit(testweakened.Request{
		Root: root, Session: session, Children: children, Paths: paths, Removed: removed,
		RenamePairs: prospective.RenamePairs,
	}, len(carried) != 0)
	result.Weakened = weakening.Findings
	if len(weakening.Problems) != 0 {
		return errors.New(strings.Join(weakening.Problems, "\n\n"))
	}
	// An RFC approval is keyed on the main thread's namespace, not on the
	// agent's (lepath.SessionCommitSession). A caller that named its session
	// named the namespace its approvals are under too.
	approvalSession := session
	if options.Session == "" {
		approvalSession, err = lepath.SessionCommitSession(root)
		if err != nil {
			return fmt.Errorf("resolve this session's RFC approval namespace: %w", err)
		}
	}
	rfcChanges, rfcApprovals, rfcProblems := rfcChangeProblems(root, approvalSession, prospective)
	result.RFCChanges = rfcChanges
	result.RFCApprovals = rfcApprovals
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
	// Each shard this commit carries holds the rows this commit uses. Every
	// other row in it that git holds at HEAD is one an earlier commit
	// landed, and the gate drops it here rather than asking the author to. A
	// row that has not landed stays: its writer, a subagent whose shard this
	// main thread carries among them, may not have committed it yet. That
	// deletion by hand is what made a stale row block three commits it had
	// nothing to do with (plan/journal/concurrent-session-corruption.md). The
	// RFC approval rows are pruned by the SCRIPT instead, after the commit
	// that carries their trailers succeeds (renderApprovalPrune).
	if !options.DryRun {
		for _, shard := range carried {
			if err := testweakened.PruneLanded(root, shard, weakening.Keep[shard]); err != nil {
				return fmt.Errorf("prune landed rows from %s: %w", shard, err)
			}
		}
	}

	if problems := testCoverageProblems(root, paths); len(problems) != 0 &&
		strings.TrimSpace(options.NoTest) == "" {
		return fmt.Errorf("%s\n  or name no-test with a truthful reason",
			strings.Join(problems, "\n"))
	}
	return nil
}

// checkVerificationGates judges the commit against the native verification
// certificate, the recorded structural reds and the independent-review
// artifact. It refuses a charged structural red with no admission and a
// closure with no clean review and no override, and answers the review
// re-check line the script runs. It never refuses for a missing verification:
// that is owed before a push, and the push derives it.
func checkVerificationGates(root string, options *Options, result *Prepared, paths, removed []string) (string, error) {
	all := append(append([]string{}, paths...), removed...)
	result.Verify = verificationState(root, all)
	if result.Verify.State != verifyFresh {
		reds := structuralGateReds(root, all)
		charged := append([]string(nil), reds.Charged...)
		if len(charged) == 1 && charged[0] == compilesStage &&
			strings.TrimSpace(options.BrokenHeadFix) != "" {
			trailer, err := admissionTrailer(trailerBrokenHeadFix, "", options.BrokenHeadFix)
			if err != nil {
				return "", err
			}
			result.Trailers = append(result.Trailers, trailer)
			charged = nil
		}
		if len(charged) != 0 && strings.TrimSpace(options.StructuralRedOK) != "" {
			trailer, err := admissionTrailer(trailerStructuralRedOK, "", options.StructuralRedOK)
			if err != nil {
				return "", err
			}
			result.Trailers = append(result.Trailers, trailer)
		}
		if len(charged) != 0 && strings.TrimSpace(options.StructuralRedOK) == "" {
			detail := ""
			if len(reds.Unattributed) != 0 {
				detail = "\n  charged for want of path attribution: " +
					strings.Join(reds.Unattributed, ", ")
			}
			for _, cause := range reds.Causes {
				detail += "\n  " + cause
			}
			return "", fmt.Errorf(
				"deterministic structural gate(s) block this commit, red or not reached: %s%s\n"+
					"  fix the producer, use broken-head-fix for the sole compiles red, "+
					"or name structural-red-ok with a truthful reason",
				strings.Join(charged, ", "), detail,
			)
		}
	}

	base, err := appendBase(root, result.Session, options)
	if err != nil {
		return "", err
	}
	stem, err := closureStem(root, paths, removed, base)
	if err != nil {
		return "", err
	}
	if stem == "" {
		return "", nil
	}
	review := CheckReview(root, stem, paths)
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
	return reviewCheckCommand(stem, paths), nil
}

// admissionTrailer renders the record of one admission: `<key><subject>:
// <reason>`, or `<key><reason>` for an admission about no subject. A trailer
// is one message line the derivation reads back, so a reason spanning lines
// is refused rather than cut.
func admissionTrailer(key, subject, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if strings.ContainsAny(reason, "\r\n") {
		return "", fmt.Errorf("the reason given for %s must be one line, because it becomes the %s trailer: %q",
			strings.TrimSpace(key), strings.TrimSuffix(strings.TrimSpace(key), ":"), reason)
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

func testCoverageProblems(root string, paths []string) []string {
	owing := make([]string, 0)
	hasTest := false
	for _, path := range paths {
		if isCommitTestPath(path) {
			hasTest = true
		}
		if !testCoverageRequired(path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			continue
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
		return nil
	}
	return []string{"this commit carries Go and no test:\n  " + strings.Join(owing, "\n  ")}
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
			return "", "", fmt.Errorf("script does not exist: %s", relative)
		}
		if !strings.Contains(string(content), scriptMarker) {
			return "", "", fmt.Errorf("script is not a generated commit script: %s", relative)
		}
		if !options.Append && !options.Replace {
			return "", "", fmt.Errorf("%s exists; name append or replace", relative)
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
			return "", "", errors.New("append: this session has no prepared script")
		}
		if len(matches) > 1 {
			slices.Sort(matches)
			return "", "", fmt.Errorf("append is ambiguous: this session has %d prepared scripts; name script", len(matches))
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
	return errors.New("replace refused: script was prepared for a different commit and shares none of these files")
}

// refuseAttestAdmission refuses an attestation that also carries a create
// keyword the keyword table does not declare WithAttest. The admission
// keywords record why a commit that CHANGES files was let past a gate, and
// no-test and ledger-carried act on a commit's files too; an attestation
// changes no file and runs no gate, so the keyword would be dropped in silence
// and the reason the author gave recorded nowhere. The refusal is derived from
// createKeywords, so a keyword added there is refused until it is declared
// WithAttest. The keywords are judged in sorted order, so two given at once
// always refuse naming the same one.
func refuseAttestAdmission(options *Options) error {
	return refuseUndeclaredWithAttest(options, createKeywords)
}

// refuseUndeclaredWithAttest is refuseAttestAdmission over a given table, so a
// test can add a keyword nobody decided without mutating createKeywords.
func refuseUndeclaredWithAttest(options *Options, rules map[string]keywordRule) error {
	for _, keyword := range slices.Sorted(maps.Keys(rules)) {
		rule := rules[keyword]
		if rule.WithAttest {
			continue
		}
		// A removed keyword never reaches Options: parsing refuses it.
		if rule.Replacement != "" {
			continue
		}
		// A live keyword with no Given reader cannot be judged, so the
		// refusal says so rather than letting an attestation carry it.
		if rule.Given == nil {
			return errors.New("attest refused: create keyword " + keyword + " has no Given reader, " +
				"so the attest refusal cannot tell whether an attestation carries it; " +
				"give " + keyword + " a Given in createKeywords, in internal/le/commit/actions.go")
		}
		if !rule.Given(options) {
			continue
		}
		return errors.New("attest refused with " + keyword + ": an attestation changes no file and runs no gate, " +
			"so there is nothing for " + keyword + " to act on and its value would be recorded nowhere; " +
			"give " + keyword + " on a commit of its own, and prepare the attestation alone with " +
			`./le commit create subject "<subject>" attest <commit> "<owner words>"`)
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
