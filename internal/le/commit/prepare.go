// Design: docs/features/ai-first.md -- prepare one safe explicit commit
package commit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/testweakened"
	"github.com/ze-software/ze/internal/le/verify/engine"
)

// Options is the callable commit-preparation contract. Paths and removals are
// explicit and repeatable; no option broadens them to a directory or whole tree.
type Options struct {
	Session             string   `json:"session,omitempty"`
	Tag                 string   `json:"tag,omitempty"`
	Subject             string   `json:"subject"`
	Body                []string `json:"body,omitempty"`
	Files               []string `json:"files,omitempty"`
	Remove              []string `json:"remove,omitempty"`
	Script              string   `json:"script,omitempty"`
	Append              bool     `json:"append"`
	Replace             bool     `json:"replace"`
	Push                string   `json:"push,omitempty"`
	NoTest              string   `json:"no-test,omitempty"`
	Unverified          string   `json:"unverified,omitempty"`
	StructuralRedOK     string   `json:"structural-red-ok,omitempty"`
	MissingFullVerifyOK string   `json:"missing-full-verify-ok,omitempty"`
	StaleIndexOK        string   `json:"stale-index-ok,omitempty"`
	ReviewOverride      string   `json:"review-override,omitempty"`
	BrokenHeadFix       string   `json:"broken-head-fix,omitempty"`
	RFCChangeOK         string   `json:"rfc-change-ok,omitempty"`
	DryRun              bool     `json:"dry-run"`
}

// Prepared is the structured result printed by `le commit create`.
type Prepared struct {
	Session     string                 `json:"session"`
	Message     string                 `json:"message"`
	Script      string                 `json:"script"`
	Verify      VerificationState      `json:"verify"`
	Review      *ReviewResult          `json:"review,omitempty"`
	Debt        []Debt                 `json:"debt,omitempty"`
	NoTest      string                 `json:"no-test,omitempty"`
	Push        bool                   `json:"push"`
	DryRun      bool                   `json:"dry-run"`
	MessageText string                 `json:"message-text,omitempty"`
	ScriptText  string                 `json:"script-text,omitempty"`
	Added       []string               `json:"added"`
	Removed     []string               `json:"removed"`
	Weakened    []testweakened.Finding `json:"weakened,omitempty"`
	RFCChanges  []RFCChange            `json:"rfc-changes,omitempty"`
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
	if len(paths) == 0 && len(removed) == 0 {
		return result, errors.New("at least one file or remove path is required")
	}
	message, err := Message(options.Subject, options.Body)
	if err != nil {
		return result, err
	}
	// The tag is validated here, beside the message, and not where nextTag
	// derives the message path. nextTag runs after recordDebt, so a tag refused
	// there leaves verification-debt rows naming a commit that was never made.
	if err := validateTag(options.Tag); err != nil {
		return result, err
	}
	authorisation, err := pushAuthorisation(options.Push)
	if err != nil {
		return result, err
	}
	result.NoTest = strings.TrimSpace(options.NoTest)

	prospective, problems := testweakened.ProspectiveCommit(root, paths, removed)
	if len(problems) != 0 {
		return result, errors.New(strings.Join(problems, "\n"))
	}
	carriesLedger := slices.Contains(paths, testweakened.ContractPath)
	weakening := testweakened.CheckCommit(testweakened.Request{
		Root: root, Paths: paths, Removed: removed, RenamePairs: prospective.RenamePairs,
	}, carriesLedger)
	result.Weakened = weakening.Findings
	if len(weakening.Problems) != 0 {
		return result, errors.New(strings.Join(weakening.Problems, "\n\n"))
	}
	rfcChanges, rfcProblems := rfcChangeProblems(
		root, prospective, slices.Contains(paths, rfcChangedPath),
	)
	result.RFCChanges = rfcChanges
	if len(rfcProblems) != 0 && strings.TrimSpace(options.RFCChangeOK) == "" {
		return result, errors.New(strings.Join(rfcProblems, "\n\n"))
	}

	if options.StaleIndexOK == "" {
		if err := checkDiscoveryIndex(root, paths); err != nil {
			return result, fmt.Errorf("%w\n  or name stale-index-ok with a truthful reason", err)
		}
	}
	if problems := testCoverageProblems(root, paths); len(problems) != 0 &&
		strings.TrimSpace(options.NoTest) == "" {
		return result, fmt.Errorf("%s\n  or name no-test with a truthful reason",
			strings.Join(problems, "\n"))
	}

	session, err := SessionID(root, options.Session)
	if err != nil {
		return result, err
	}
	result.Session = session
	all := append(append([]string{}, paths...), removed...)
	result.Verify = verificationState(root, all)
	observed := make(map[string]string)
	if result.Verify.State != verifyFresh {
		observed[gateUnverified] = "verify-status is not FRESH-green: " + result.Verify.Detail
	}
	if carriesGo(all) && result.Verify.Mode != verifyengine.Mode {
		observed[gateMissingFullVerifyOK] = "no full native verification covers this commit's Go"
	}
	if result.Verify.State != verifyFresh {
		reds := structuralGateReds(root, all)
		charged := append([]string(nil), reds.Charged...)
		if len(charged) == 1 && charged[0] == trackedBuildStage &&
			strings.TrimSpace(options.BrokenHeadFix) != "" {
			charged = nil
		}
		if len(charged) != 0 && strings.TrimSpace(options.StructuralRedOK) == "" {
			detail := ""
			if len(reds.Unattributed) != 0 {
				detail = "\n  charged for want of path attribution: " +
					strings.Join(reds.Unattributed, ", ")
			}
			return result, fmt.Errorf(
				"deterministic structural gate(s) are red for this commit: %s%s\n"+
					"  fix the producer, use broken-head-fix for the sole tracked-build red, "+
					"or name structural-red-ok with a truthful reason",
				strings.Join(charged, ", "), detail,
			)
		}
	}

	stem, err := closureStem(root, paths, removed)
	if err != nil {
		return result, err
	}
	reviewCheck := ""
	if stem != "" {
		review := CheckReview(root, session, stem, paths)
		result.Review = &review
		if !review.Clean && strings.TrimSpace(options.ReviewOverride) == "" {
			return result, errors.New(strings.Join(review.Problems, "\n"))
		}
		if review.Clean {
			reviewCheck = reviewCheckCommand(stem, paths)
		}
	}

	overrides := map[string]string{
		gateUnverified:          options.Unverified,
		gateStructuralRedOK:     options.StructuralRedOK,
		gateMissingFullVerifyOK: options.MissingFullVerifyOK,
		gateStaleIndexOK:        options.StaleIndexOK,
		gateReviewOverride:      options.ReviewOverride,
		gateBrokenHeadFix:       options.BrokenHeadFix,
		gateRFCChangeOK:         options.RFCChangeOK,
	}
	owed := owedDebt(session, options.Subject, overrides, observed)
	if len(owed) != 0 && !options.DryRun {
		debtFile, err := recordDebt(root, session, options.Subject, owed)
		if err != nil {
			return result, err
		}
		if !slices.Contains(paths, debtFile) {
			paths = append(paths, debtFile)
			result.Verify = verificationState(root, append(paths, removed...))
		}
	}
	result.Debt = owed
	if authorisation != "" {
		open, err := openDebt(root)
		if err != nil {
			return result, err
		}
		if len(open)+len(owed) != 0 {
			return result, fmt.Errorf("refusing push: %d open verification-debt row(s); run le commit debt-clear", len(open)+len(owed))
		}
	}

	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return result, err
	}
	tag, messagePath, err := nextTag(root, session, options.Tag)
	if err != nil {
		return result, err
	}
	reserved := options.Tag == ""
	keepReservation := false
	defer func() {
		if reserved && !keepReservation {
			info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(messagePath)))
			if statErr == nil && info.Size() == 0 {
				_ = os.Remove(filepath.Join(root, filepath.FromSlash(messagePath)))
			}
		}
	}()

	scriptPath, existing, err := targetScript(root, session, tag, options)
	if err != nil {
		return result, err
	}
	block := commitBlock{
		Tag: tag, Subject: strings.TrimSpace(options.Subject), Paths: paths, Removed: removed,
		MessagePath: messagePath, ReviewCheck: reviewCheck,
	}
	if options.Replace && existing != "" {
		if err := refuseForeignReplace(existing, append(paths, removed...)); err != nil {
			return result, err
		}
	}
	scriptText, err := composeScript(existing, scriptPath, session, block, options.Append, authorisation)
	if err != nil {
		return result, err
	}
	result.Message = messagePath
	result.Script = scriptPath
	result.Push = authorisation != ""
	result.DryRun = options.DryRun
	result.Added = paths
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

func owedDebt(session, subject string, overrides, observed map[string]string) []Debt {
	owed := make([]Debt, 0)
	for _, gate := range debtGates {
		reason := strings.TrimSpace(overrides[gate.Key])
		if reason == "" {
			reason = strings.TrimSpace(observed[gate.Key])
		}
		if reason != "" {
			owed = append(owed, Debt{Session: session, Subject: subject, Gate: gate.Name, Reason: reason, Status: statusOpen})
		}
	}
	return owed
}

func checkDiscoveryIndex(root string, paths []string) error {
	required := false
	for _, path := range paths {
		content, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if discoveryindex.IsSource(path, string(content)) {
			required = true
			break
		}
	}
	if required && !slices.Contains(paths, discoveryindex.OutputRel) {
		return fmt.Errorf("commit changes an index-feeding source and omits %s", discoveryindex.OutputRel)
	}
	report, err := discoveryindex.Check(root)
	if err != nil {
		return fmt.Errorf("discovery-index check could not run: %w", err)
	}
	if report.Stale {
		return fmt.Errorf("generated discovery index is stale: %s", report.File)
	}
	return nil
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
		if !strings.Contains(string(content), "git commit -F ") {
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
			if readErr == nil && strings.Contains(string(content), "git commit -F ") {
				candidates = append(candidates, match)
			}
		}
		matches = candidates
		if len(matches) == 0 {
			return "", "", errors.New("append: this session has no prepared script")
		}
		if len(matches) > 1 {
			sort.Strings(matches)
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

func composeScript(existing, scriptPath, session string, block commitBlock, appendBlock bool, push string) (string, error) {
	header := "#!/bin/bash\nset -euo pipefail\n" +
		"cd \"$(git rev-parse --show-toplevel)\"\n\n" +
		markerLine(scriptMarker, scriptPath+" session="+session) + "\n\n"
	body := header
	existingPush := ""
	if appendBlock && existing != "" {
		var err error
		body, existingPush, err = splitPush(existing)
		if err != nil {
			return "", err
		}
		body = strings.TrimRight(body, "\n") + "\n\n"
	}
	body += renderBlock(block, scriptPath)
	if push == "" {
		push = existingPush
	}
	if push != "" {
		body += "\n" + renderPush(push)
	}
	return body, nil
}
