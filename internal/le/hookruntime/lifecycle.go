// Design: docs/architecture/core-design.md -- native Claude lifecycle hooks
package hookruntime

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/ai"
	"github.com/ze-software/ze/internal/le/commit"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/rules"
	"github.com/ze-software/ze/internal/le/session"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
	specsession "github.com/ze-software/ze/internal/le/spec/session"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// gitTimeout bounds one git call a lifecycle hook makes. Each reads the index
// or a ref, which is milliseconds, so a run past this is a wedged repository
// rather than a slow one. Both callers already treat a git error as no answer.
const gitTimeout = 60 * time.Second

func runLifecycleHook(kind string, ctx context, out, errOut io.Writer) int {
	switch kind {
	case "session-start":
		return hookSessionStart(ctx, out)
	case "compaction-reminder":
		return hookCompactionReminder(ctx, errOut)
	case "verify-claim-reminder":
		fmt.Fprintln(out, "Reminder: verify a claim about code by reading the function that PRODUCES it, not the caller. Unread means unverified. Cite file + symbol.") //nolint:errcheck // hook protocol
	case "delegation-reminder":
		fmt.Fprintln(out, "Reminder: delegation is pre-approved. For 2+ independent tasks, parallelize with subagents; no permission request is needed.") //nolint:errcheck // hook protocol
	case "block-until-lsp":
		return hookUntilLSP(ctx, errOut)
	case "pre-compact-save":
		return hookPreCompact(ctx, errOut)
	case "block-premature-stop":
		return hookStop(ctx, errOut)
	case "rule-coverage-report":
		return hookRuleCoverage(ctx, errOut)
	case "session-end-summary":
		return hookEndSummary(ctx, errOut)
	case "session-end-deferrals":
		return hookDeferrals(ctx, errOut)
	case "subagent-context":
		return hookSubagentContext(ctx, out)
	case "mark-lsp-invoked":
		return writeSessionMarker(ctx, ".lsp-invoked-", "")
	case "mark-source-read":
		return hookSourceRead(ctx)
	case "mark-agent-spawned":
		return writeSessionMarker(ctx, ".agent-spawned-", "")
	case "validate-spec":
		return hookValidateSpec(ctx, errOut)
	default:
		fmt.Fprintf(errOut, "unknown hook runtime %q\n", kind) //nolint:errcheck // hook protocol
		return 2
	}
	return 0
}

func writeSessionMarker(ctx context, prefix, body string) int {
	path := sessionMarker(ctx, prefix)
	if path == "" {
		return 0
	}
	if body == "" {
		body = time.Now().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0
	}
	_ = os.WriteFile(path, []byte(body+"\n"), 0o600)
	return 0
}

// lspBlockedMessage is the refusal the LSP gate prints. It is a constant so the
// call that writes it fits on one line with its errcheck exemption.
const lspBlockedMessage = "❌ Blocked: LSP tool must be loaded before any other tool call.\n\n" +
	"   First tool call of every session MUST be:\n       ToolSearch query=\"select:LSP\"\n\n" +
	"   See .claude/rules/session-start.md, \"LSP Load (step 1) -- no-exceptions clause\".\n" +
	"   No task-type exception (shell-only, docs-only, trivial, etc.) applies.\n"

func hookUntilLSP(ctx context, errOut io.Writer) int {
	id := resolvedSessionID(ctx)
	if id == "" {
		return 0
	}
	marker := filepath.Join(ctx.root, "tmp", "session", ".lsp-loaded-"+id)
	if ctx.tool == "ToolSearch" {
		if strings.Contains(strings.ToLower(stringInput(ctx.input, "query")), "lsp") {
			_ = os.MkdirAll(filepath.Dir(marker), 0o750)
			_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
		}
		return 0
	}
	if _, err := os.Stat(marker); err == nil {
		return 0
	}
	fmt.Fprint(errOut, lspBlockedMessage) //nolint:errcheck // hook protocol
	return 2
}

func hookSessionStart(ctx context, out io.Writer) int {
	if id, present := payloadSessionID(ctx.payload); present && id != "" {
		_ = os.Setenv("CLAUDE_CODE_SESSION_ID", id)
		if environmentFile := os.Getenv("CLAUDE_ENV_FILE"); environmentFile != "" {
			file, err := os.OpenFile(environmentFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the path is CLAUDE_ENV_FILE, which the harness owns
			if err == nil {
				fmt.Fprintf(file, "export CLAUDE_CODE_SESSION_ID=%s\n", id) //nolint:errcheck // environment contract
				_ = file.Close()
			}
		}
	}
	id := resolvedSessionID(ctx)
	claim := readFirstLine(filepath.Join(ctx.root, "tmp", "session", ".session-"+id))
	if claim == specUnassigned {
		claim = ""
	}
	gitStatus, cancelStatus := stdcontext.WithTimeout(stdcontext.Background(), gitTimeout)
	command := exec.CommandContext(gitStatus, "git", "status", "--porcelain")
	command.Dir = ctx.root
	status, _ := command.Output()
	cancelStatus()
	if strings.TrimSpace(string(status)) == "" {
		fmt.Fprintln(out, "Clean tree") //nolint:errcheck // hook protocol
	} else {
		lines := strings.Split(strings.TrimSuffix(string(status), "\n"), "\n")
		modified, added := 0, 0
		for _, line := range lines {
			if strings.HasPrefix(line, " M") {
				modified++
			}
			if strings.HasPrefix(line, "??") {
				added++
			}
		}
		fmt.Fprintf(out, "Warning: %d uncommitted: %dM %dA\n", len(lines), modified, added) //nolint:errcheck // hook protocol
	}
	specs, _ := filepath.Glob(filepath.Join(ctx.root, "plan", "spec-*.md"))
	if claim != "" {
		if _, err := os.Stat(filepath.Join(ctx.root, "plan", claim)); err == nil {
			fmt.Fprintf(out, "SPEC: %s (+%d others)\n   -> READ plan/%s BEFORE any work\n", claim, max(0, len(specs)-1), claim) //nolint:errcheck // hook protocol
		}
	} else if len(specs) != 0 {
		fmt.Fprintf(out, "%d specs, none claimed by this session\n", len(specs)) //nolint:errcheck // hook protocol
	}
	if len(specs) != 0 {
		// The breakdown comes from specstatus so this line and `./le spec
		// status` name one vocabulary in one order. This hook used to keep its
		// own list of seven statuses, which named no default and so counted
		// only what it listed: `done` was absent, and the line under-reported
		// the population it sits beside. StatusPhrases consults no git, which
		// is the reason it exists rather than Collect.
		if phrases, err := specstatus.StatusPhrases(ctx.root); err == nil && len(phrases) != 0 {
			fmt.Fprintf(out, "   (%s)\n", strings.Join(phrases, ", ")) //nolint:errcheck // hook protocol
		}
	}
	if claim != "" {
		found := stateFile(ctx)
		if _, err := os.Stat(found); err != nil {
			stem := strings.TrimSuffix(strings.TrimPrefix(claim, "spec-"), ".md")
			found, _ = specsession.LatestStateForSpec(ctx.root, stem)
			if found != "" && !filepath.IsAbs(found) {
				found = filepath.Join(ctx.root, found)
			}
		}
		if _, err := os.Stat(found); err == nil {
			fmt.Fprintf(out, "Session state: %s\n", found) //nolint:errcheck // hook protocol
		}
	}
	if debts, err := commit.ListDebt(ctx.root); err == nil {
		open := make([]commit.Debt, 0)
		for _, debt := range debts {
			if strings.EqualFold(debt.Status, "open") {
				open = append(open, debt)
			}
		}
		if len(open) != 0 {
			fmt.Fprintf(out, "Warning: verification debt: %d gate(s) owed, --push is refused until cleared\n", len(open)) //nolint:errcheck // hook protocol
			for _, debt := range open[:min(5, len(open))] {
				fmt.Fprintf(out, "   - %s  (%s)\n", debt.Gate, debt.Subject) //nolint:errcheck // hook protocol
			}
		}
	}
	if _, err := os.Stat(filepath.Join(ctx.root, "ai", "DOCS-TO-CODE.md")); os.IsNotExist(err) {
		if _, updateErr := docstocode.Update(ctx.root); updateErr == nil {
			fmt.Fprintln(out, "Built ai/DOCS-TO-CODE.md (derived, not tracked)") //nolint:errcheck // hook protocol
		}
	}
	if _, err := os.Stat(filepath.Join(ctx.root, "ai", "CODE-TO-DOCS.md")); os.IsNotExist(err) {
		if _, updateErr := docstocode.UpdateCodeIndex(ctx.root); updateErr == nil {
			fmt.Fprintln(out, "Built ai/CODE-TO-DOCS.md (derived, not tracked)") //nolint:errcheck // hook protocol
		}
	}
	if report, err := (ai.Mirror{Root: ctx.root}).Check(); err != nil || len(report.Stale) != 0 {
		fmt.Fprintln(out, "Warning: generated agent files are stale (CLAUDE.md / AGENTS.md / skills mirrors)") //nolint:errcheck // hook protocol
		fmt.Fprintln(out, "   -> run: ./le ai skills-sync")                                                    //nolint:errcheck // hook protocol
	}
	fmt.Fprintln(out, "Warning: BLOCKING (no task-type exception): ToolSearch query=\"select:LSP\" MUST be your FIRST tool call.")                                 //nolint:errcheck // hook protocol
	fmt.Fprintln(out, "Warning:   Do NOT skip because the task looks shell-only, docs-only, or trivial.")                                                          //nolint:errcheck // hook protocol
	fmt.Fprintln(out, "Warning:   See .claude/rules/session-start.md 'LSP Load (step 1) -- no-exceptions clause'.")                                                //nolint:errcheck // hook protocol
	fmt.Fprintln(out, "Warning: RULE: Read spec + source files BEFORE writing any code")                                                                           //nolint:errcheck // hook protocol
	fmt.Fprintln(out, "Rules: ai/rules/INDEX.md is a one-line overview of every rule -- scan it, read the listed file in full before acting on a topic it covers") //nolint:errcheck // hook protocol
	if claim == "" && len(specs) != 0 {
		fmt.Fprintln(out, "Tip: /ze-status for a cross-project attention view") //nolint:errcheck // hook protocol
	}
	return 0
}

func readFirstLine(path string) string {
	body, err := os.ReadFile(path) //nolint:gosec // every caller passes a path built from the checkout root
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(body), "\n")
	return strings.TrimSpace(line)
}

func hookCompactionReminder(ctx context, errOut io.Writer) int {
	message := ctx.payload.Prompt
	if message == "" {
		message = ctx.payload.LastMessage
	}
	lower := strings.ToLower(message)
	if !strings.Contains(lower, "continued from a previous conversation") {
		return 0
	}
	if !anyContains(lower, "ran out of context", "context compaction") {
		return 0
	}
	_ = writeSessionMarker(ctx, ".compaction-detected-", time.Now().Format(time.RFC3339))
	fmt.Fprintln(errOut, "⚠ Context compaction detected. Read this session's digest before continuing, then verify every carried claim against source.") //nolint:errcheck // hook protocol
	return 0
}

func stateFile(ctx context) string {
	paths, err := lepath.ResolveSession(ctx.root, false)
	if err != nil {
		return ""
	}
	claim := readFirstLine(filepath.Join(ctx.root, "tmp", "session", ".session-"+paths.ID))
	name := "session-state-" + paths.ID + ".md"
	if claim != "" && claim != specUnassigned {
		stem := strings.TrimSuffix(strings.TrimPrefix(claim, "spec-"), ".md")
		name = "session-state-" + stem + "-" + paths.ID + ".md"
	}
	return filepath.Join(ctx.root, paths.Dir, "state", name)
}

func hookPreCompact(ctx context, errOut io.Writer) int {
	path := stateFile(ctx)
	if path == "" {
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the session state path is built from the checkout root
	if err != nil {
		return 0
	}
	fmt.Fprintf(file, "\n## Pre-compaction snapshot %s\n\nContext was compacted. Re-read current source before trusting earlier conclusions.\n", time.Now().Format(time.RFC3339)) //nolint:errcheck // state record
	_ = file.Close()
	fmt.Fprintf(errOut, "Session state saved to %s\n", path) //nolint:errcheck // hook protocol
	return 0
}

func stripStopMarkup(text string) string {
	var out textbuf.Buffer
	out.Reset()
	fence := false
	fenceLength := 0
	pending := make([]string, 0)
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		ticks := 0
		for ticks < len(trimmed) && trimmed[ticks] == '`' {
			ticks++
		}
		if ticks >= 3 {
			if !fence {
				fence, fenceLength, pending = true, ticks, pending[:0]
				continue
			}
			if ticks >= fenceLength {
				fence, pending = false, pending[:0]
				continue
			}
		}
		if fence {
			pending = append(pending, line)
			continue
		}
		if strings.Count(line, "`")%2 == 0 {
			line = regexp.MustCompile("`[^`]*`").ReplaceAllString(line, "")
		}
		out.Str(line).Byte('\n')
	}
	if fence {
		out.Join(pending, "\n")
	}
	// String detaches the heap slice over 128 bytes, so the buffer is read once
	// and the result is what both branches below answer with.
	stripped := out.String()
	if strings.TrimSpace(stripped) == "" {
		return text
	}
	return stripped
}

func hookStop(ctx context, errOut io.Writer) int {
	text := ctx.payload.LastMessage
	if text == "" {
		return 0
	}
	reasons := make([]string, 0, 3)
	openWork := false
	id := resolvedSessionID(ctx)
	claimPath := filepath.Join(ctx.root, "tmp", "session", ".session-"+id)
	claim := readFirstLine(claimPath)
	if claim != "" && claim != specUnassigned {
		if report, _, closureErr := specstatus.CheckClosure(ctx.root, filepath.Join("plan", claim)); closureErr == nil && report.Blocked() {
			fmt.Fprintln(errOut, "BLOCKED: spec implemented but not closed.") //nolint:errcheck // hook protocol
			fmt.Fprint(errOut, report.Text())                                 //nolint:errcheck // hook protocol
			return 2
		}
		specBody, err := os.ReadFile(filepath.Join(ctx.root, "plan", claim)) //nolint:gosec // the claimed spec lives under the checkout plan directory
		if err == nil && regexp.MustCompile(`(?m)^\|[ \t]*Status[ \t]*\|.*in-progress`).Match(specBody) {
			openWork = true
			reasons = append(reasons, "Spec '"+claim+"' in-progress")
		}
		if _, err := os.Stat(filepath.Join(ctx.root, "tmp", "session", ".agent-spawned-"+id)); err != nil {
			reasons = append(reasons, "Delegation: no subagent spawned")
		}
	}
	if !ctx.payload.StopHookActive {
		scan := stripStopMarkup(text)
		patterns := []string{"let me know if you", "would you like me to", "feel free to", "if you.d like me to", "if you want me to", "happy to help", "I can [a-z]+ .* if you", "I.ll stop here", "I will stop here", "I.ll pause here", "I will pause here", "that.s all for now", "I.ll leave .* to you", "I will leave .* to you", "should I (proceed|continue|go ahead)", "do you want me to", "(?m)^want me to", "want me to .* or", "shall I (proceed|continue|go ahead|start|keep)", "before I proceed", "ready for me to", "or (leave|skip|ignore) (them|it|this|that)", "or should I", "or something else"}
		if openWork {
			patterns = append(patterns, "what would you like", "what do you want to do", "what.s next", "what next")
		}
		for _, pattern := range patterns {
			if regexp.MustCompile("(?i)" + pattern).MatchString(scan) {
				reasons = append(reasons, "Stop phrase: "+pattern)
				break
			}
		}
	}
	phrase := false
	for _, reason := range reasons {
		phrase = phrase || strings.HasPrefix(reason, "Stop phrase:")
	}
	if phrase {
		fmt.Fprintln(errOut, "BLOCKED: Premature stop detected.") //nolint:errcheck // hook protocol
		for _, reason := range reasons {
			fmt.Fprintln(errOut, "  - "+reason) //nolint:errcheck // hook protocol
		}
		fmt.Fprintln(errOut, "Delete the sentence that asked, then answer one question: who asked for that work?\n  The user did: finish it now, and do not ask permission again.\n  You did: DROP IT. Do not start it, size it, or offer it again.\nThis block is not an instruction to do the work you just offered.") //nolint:errcheck // hook protocol
		return 2
	}
	if len(reasons) != 0 {
		// One line. The transcript renders a non-blocking hook exit verbatim, so a
		// header plus one bullet per reason spends three lines to say what one says.
		fmt.Fprintln(errOut, "Warning: open session state -- "+strings.Join(reasons, "; ")) //nolint:errcheck // hook protocol
		return 1
	}
	return 0
}

func hookRuleCoverage(ctx context, errOut io.Writer) int {
	id := resolvedSessionID(ctx)
	report, code := rules.RunSessionCoverage(ctx.root, rules.SessionCoverageOptions{
		Quiet: true, Transcript: ctx.transcript, Session: id,
	}, rules.NativeTranscriptSource{}, time.Now, errOut)
	if report == nil {
		return min(code, 1)
	}
	// A quiet report repeats nothing, so an unchanged miss set renders no text. A
	// non-zero exit behind that silence reaches the transcript as a bare hook
	// failure with no stderr, which names no rule and asks for nothing.
	text := report.Text()
	if text == "" {
		return 0
	}
	fmt.Fprintln(errOut, text) //nolint:errcheck // hook protocol
	if len(report.Missed) != 0 {
		return 1
	}
	return 0
}
func hookEndSummary(ctx context, errOut io.Writer) int {
	paths, err := lepath.ResolveSession(ctx.root, false)
	if err != nil {
		fmt.Fprintln(errOut, "session-end-summary: native summary failed; recovery snapshot not updated") //nolint:errcheck // hook protocol
		return 0
	}
	if _, err := session.EndSummary(ctx.root, paths, time.Now()); err != nil {
		fmt.Fprintln(errOut, "session-end-summary: native summary failed; recovery snapshot not updated") //nolint:errcheck // hook protocol
	}
	return 0
}

func hookDeferrals(ctx context, errOut io.Writer) int {
	entries, _ := filepath.Glob(filepath.Join(ctx.root, "plan", "deferrals", "*.md"))
	open := make([]string, 0)
	for _, entry := range entries {
		body, _ := os.ReadFile(entry) //nolint:gosec // a plan/deferrals/*.md path this hook globbed in the checkout
		for line := range strings.SplitSeq(string(body), "\n") {
			cells := strings.Split(line, "|")
			if len(cells) >= 7 && strings.EqualFold(strings.TrimSpace(cells[5]), "open") {
				open = append(open, strings.TrimSpace(cells[3]))
			}
		}
	}
	if len(open) != 0 {
		fmt.Fprintf(errOut, "Open deferrals: %d\n", len(open)) //nolint:errcheck // hook protocol
		for _, item := range open[:min(5, len(open))] {
			fmt.Fprintln(errOut, "  - "+item) //nolint:errcheck // hook protocol
		}
	}
	return 0
}

func hookSubagentContext(ctx context, out io.Writer) int {
	id, present := payloadSessionID(ctx.payload)
	if present && id == "" {
		id = ""
	} else if !present {
		id = resolvedSessionID(ctx)
	}
	branch := "unknown"
	gitBranch, cancelBranch := stdcontext.WithTimeout(stdcontext.Background(), gitTimeout)
	command := exec.CommandContext(gitBranch, "git", "branch", "--show-current")
	command.Dir = ctx.root
	if body, err := command.Output(); err == nil && strings.TrimSpace(string(body)) != "" {
		branch = strings.TrimSpace(string(body))
	}
	cancelBranch()
	var text textbuf.Buffer
	text.Reset().Str("Ze is a Network OS in Go (BGP, CLI, web, plugins). Key constraints:\n- Zero-copy, buffer-first encoding: WriteTo(buf, off) int -- no make/append in encoding\n- Registration pattern: init() in register.go, never direct imports between components\n- YANG required for all RPCs -- no command module category\n- Lazy over eager: pass raw bytes, offset iterators, no intermediate structs\n- JSON keys: kebab-case\n- Goroutines: long-lived workers on channels, never per-event\n- Rules: ai/rules/\n- Branch: ").Str(branch).Byte('\n')
	if id != "" {
		paths, err := lepath.SessionForID(ctx.root, id)
		if err == nil {
			claim := readFirstLine(filepath.Join(ctx.root, "tmp", "session", ".session-"+id))
			if claim != "" && claim != specUnassigned {
				text.Str("\nSpec claimed by the session that spawned you: plan/").Str(claim).
					Str("\nRead it before acting. Its acceptance criteria are what your work is judged against.\n")
			}
			text.Str("\nParent session ID: ").Str(id).
				Str("\nParent session scratch: ").Str(paths.Scratch).
				Str("\nSet CLAUDE_CODE_SESSION_ID=").Str(id).
				Str(" for every Bash tool call. The Bash PreToolUse hook adds this prefix to the command.\n")
		}
	}
	text.Str("\nYou are a subagent under ai/rules/planning.md. Report grounded facts, read routed rules before acting, never weaken tests, and write scratch only under ./le session scratch ensure.\n")
	response := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "SubagentStart", "additionalContext": text.String()}}
	_ = json.NewEncoder(out).Encode(response)
	return 0
}

func hookSourceRead(ctx context) int {
	path := filepath.ToSlash(ctx.path)
	kind := ""
	switch {
	case strings.HasSuffix(path, ".go"):
		kind = "go"
	case strings.HasSuffix(path, ".sh"):
		kind = "sh"
	case strings.HasSuffix(path, ".yang"):
		kind = "yang"
	case strings.HasSuffix(path, ".mk") || filepath.Base(path) == "Makefile":
		kind = "make"
	}
	if kind == "" {
		return 0
	}
	return writeSessionMarker(ctx, ".source-read-"+kind+"-", "")
}

func hookValidateSpec(ctx context, errOut io.Writer) int {
	if ctx.tool == "" {
		fmt.Fprintln(errOut, "❌ validate-spec: no tool name in the hook payload -- NOTHING WAS CHECKED.\n  This is a PostToolUse hook: it reads a JSON payload on stdin and takes no arguments.") //nolint:errcheck // hook protocol
		return 2
	}
	if !oneOf(ctx.tool, toolWrite, "Edit") {
		return 0
	}
	path := filepath.ToSlash(ctx.path)
	if !regexp.MustCompile(`(^|/)plan/spec-[^/]*\.md$`).MatchString(path) {
		return 0
	}
	body, err := os.ReadFile(absolutePath(ctx))
	if err != nil {
		return 0
	}
	errors, warnings := validateSpecText(ctx.root, string(body))
	status := ""
	if match := regexp.MustCompile(`(?m)^\| Status \| *([a-z-]+)`).FindStringSubmatch(string(body)); len(match) > 1 {
		status = match[1]
	}
	if status != "skeleton" {
		report, auditErr := speccitation.AuditAnchors(ctx.root, ctx.path)
		if auditErr != nil {
			errors = append(errors, "Design document owner check could not run: "+auditErr.Error())
		} else if len(report.Owners) != 0 {
			documents := make([]string, 0, len(report.Owners))
			for _, owner := range report.Owners {
				documents = append(documents, owner.Document)
			}
			errors = append(errors, "Design document(s) declared by this spec's own code, never named in it: "+strings.Join(documents, " "))
		}
	}
	if len(errors) != 0 {
		fmt.Fprintf(errOut, "%s❌ Spec invalid (%d errors):%s\n", red, len(errors), reset) //nolint:errcheck // hook protocol
		for _, problem := range errors[:min(5, len(errors))] {
			fmt.Fprintf(errOut, "  %s✗%s %s\n", red, reset, problem) //nolint:errcheck // hook protocol
		}
		return 2
	}
	if len(warnings) != 0 {
		fmt.Fprintf(errOut, "%s⚠ Spec: %d warnings%s\n", yellow, len(warnings), reset) //nolint:errcheck // hook protocol
		for _, warning := range warnings[:min(5, len(warnings))] {
			fmt.Fprintf(errOut, "  %s!%s %s\n", yellow, reset, warning) //nolint:errcheck // hook protocol
		}
	}
	return 0
}

func validateSpecText(root, text string) ([]string, []string) {
	errors := make([]string, 0)
	warnings := make([]string, 0)
	if !strings.Contains(text, "| Status |") {
		errors = append(errors, "Missing metadata table. Add Status, Depends, Phase, Updated rows")
	} else {
		statusMatch := regexp.MustCompile(`(?m)^\| Status \| *([a-z-]+)`).FindStringSubmatch(text)
		status := ""
		if len(statusMatch) > 1 {
			status = statusMatch[1]
		}
		if !oneOf(status, "skeleton", "design", "ready", "in-progress", "verification", "blocked", "deferred", "done") {
			errors = append(errors, "Invalid Status '"+status+"'. Must be: skeleton, design, ready, in-progress, verification, blocked, deferred, done")
		}
		if !regexp.MustCompile(`(?m)^\| Updated \| *\d{4}-\d{2}-\d{2}`).MatchString(text) {
			warnings = append(warnings, "Metadata: Updated field should have a date (YYYY-MM-DD)")
		}
	}
	for _, section := range []string{"## Task", "## Required Reading", "## Current Behavior", "## Data Flow", "## Wiring Test", "## 🧪 TDD Test Plan", "### Unit Tests", "## Files to Modify", "## Implementation Steps", "## Checklist"} {
		if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(section)).MatchString(text) {
			errors = append(errors, "Missing required section: "+section)
		}
	}
	for _, section := range []string{"### Integration Checklist", "### Documentation Update Checklist"} {
		if !strings.Contains(text, section) {
			warnings = append(warnings, "Missing section: "+section)
		}
	}
	// The extension set is what Ze's sources ACTUALLY are, not what they were.
	// The original list was go|sh|rs|ts|js|mk, from before the repository's own
	// tooling moved into Go, and it never carried the kinds a protocol router is
	// mostly made of: a YANG module, a .ci fixture, a templ template, a vendored
	// C file. A spec that read those could not satisfy the check whatever it
	// listed, so the section it names was refused for holding the wrong KIND of
	// source rather than for holding none, which is what the message says.
	current := markdownSection(text, "## Current Behavior")
	if current != "" && !regexp.MustCompile("(?m)^\\s*-\\s*\\[[ x]\\]\\s*`[^`]+\\.(go|sh|rs|ts|js|mk|py|yang|ci|et|wb|templ|c|h|json|yml|yaml|md|txt|proto)(:[0-9]+)?`").MatchString(current) {
		errors = append(errors, "Current Behavior section must list the source files read, each as a `- [ ] `backticked/path.ext`` bullet")
	}
	data := markdownSection(text, "## Data Flow")
	for _, subsection := range []string{"### Entry Point", "### Transformation Path", "### Boundaries Crossed", "### Integration Points"} {
		if data != "" && !strings.Contains(data, subsection) {
			errors = append(errors, "Data Flow section missing '"+strings.TrimPrefix(subsection, "### ")+"' subsection")
		}
	}
	if strings.Contains(data, "[Where data enters") || strings.Contains(data, "[Format at entry]") {
		errors = append(errors, "Data Flow: Entry Point contains placeholder text. Document actual entry points!")
	}
	unit := markdownSection(text, "### Unit Tests")
	if unit != "" && !regexp.MustCompile(`\|.*\|.*\|`).MatchString(unit) {
		errors = append(errors, "Unit Tests section must use table format")
	}
	for _, item := range []string{"Tests written", "Tests FAIL", "Tests PASS"} {
		if !strings.Contains(text, item) {
			errors = append(errors, "Missing checklist item: "+item)
		}
	}
	if !strings.Contains(text, "./le verify worktree") {
		errors = append(errors, "Missing verification checklist item: './le verify worktree'")
	}
	if regexp.MustCompile("```(go|rust|java|c|cpp|javascript|typescript)").MatchString(text) {
		errors = append(errors, "Specs MUST NOT contain code blocks. Use tables/prose instead")
	}
	wiring := markdownSection(text, "## Wiring Test")
	if wiring != "" && !regexp.MustCompile(`\|.*(→|->).*\|`).MatchString(wiring) {
		errors = append(errors, "Wiring Test section must have table with Entry Point -> Feature Code -> Test columns")
	}
	if !strings.Contains(text, "## Acceptance Criteria") {
		warnings = append(warnings, "Missing '## Acceptance Criteria' section. Define testable AC-N assertions before implementation")
	}
	if !strings.Contains(text, "## Risks & Assumptions") {
		warnings = append(warnings, "Missing '## Risks & Assumptions' section")
	}
	_ = root
	return errors, warnings
}

func markdownSection(text, heading string) string {
	_, rest, found := strings.Cut(text, heading)
	if !found {
		return ""
	}
	level := strings.Count(strings.Fields(heading)[0], "#")
	lines := strings.Split(rest, "\n")
	end := len(lines)
	for index, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, strings.Repeat("#", level)+" ") {
			end = index + 1
			break
		}
	}
	return strings.Join(lines[:end], "\n")
}
