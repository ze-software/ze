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
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/rules"
	"github.com/ze-software/ze/internal/le/session"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
	specsession "github.com/ze-software/ze/internal/le/spec/session"
	"github.com/ze-software/ze/internal/le/spec/specpath"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// gitTimeout bounds one git call a lifecycle hook makes. Each reads the index
// or a ref, which is milliseconds, so a run past this is a wedged repository
// rather than a slow one. Both callers already treat a git error as no answer.
const gitTimeout = 60 * time.Second

// runLifecycleHook runs one lifecycle hook and reports its exit code and
// whether a hook of that name is served. The caller renders the refusal for a
// name nothing serves, so the served set stays declared once, in this switch,
// and a probe can ask whether a kind exists without matching a message.
func runLifecycleHook(kind string, ctx context, out, errOut io.Writer) (int, bool) {
	switch kind {
	case "session-start":
		return hookSessionStart(ctx, out), true
	case "compaction-reminder":
		hookCompactionReminder(ctx, errOut)
	case "session-id":
		return runSessionID(ctx, out, errOut), true
	case "verify-claim-reminder":
		fmt.Fprintln(out, "Reminder: verify a claim about code by reading the function that PRODUCES it, not the caller. Unread means unverified. Cite file + symbol.") //nolint:errcheck // hook protocol
	case "delegation-reminder":
		fmt.Fprintln(out, "Reminder: delegation is pre-approved. For 2+ independent tasks, parallelize with subagents; no permission request is needed.") //nolint:errcheck // hook protocol
	case "block-until-lsp":
		return hookUntilLSP(ctx, errOut), true
	case "pre-compact-save":
		hookPreCompact(ctx, errOut)
	case "block-premature-stop":
		return hookStop(ctx, errOut), true
	case "rule-coverage-report":
		return hookRuleCoverage(ctx, errOut), true
	case "session-end-summary":
		hookEndSummary(ctx, errOut)
	case "subagent-context":
		return hookSubagentContext(ctx, out), true
	case "mark-lsp-invoked":
		return writeSessionMarker(ctx, ".lsp-invoked-", ""), true
	case "mark-source-read":
		return hookSourceRead(ctx), true
	case "mark-agent-spawned":
		return writeSessionMarker(ctx, ".agent-spawned-", ""), true
	case "validate-spec":
		return hookValidateSpec(ctx, errOut), true
	default:
		return 0, false
	}
	return 0, true
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
	specs, _ := specpath.All(ctx.root)
	if claim != "" {
		// The claim marker holds the file NAME, so the line has to say which
		// bucket to open. It named plan/<claim> until the release buckets
		// arrived, and that path opened nothing for a spec in two of the three.
		if relative, err := specpath.Find(ctx.root, claim); err == nil {
			fmt.Fprintf(out, "SPEC: %s (+%d others)\n   -> READ %s BEFORE any work\n", claim, max(0, len(specs)-1), relative) //nolint:errcheck // hook protocol
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
		for index := range debts {
			if strings.EqualFold(debts[index].Status, "open") {
				open = append(open, debts[index])
			}
		}
		if len(open) != 0 {
			fmt.Fprintf(out, "Warning: verification debt: %d gate(s) owed, --push is refused until cleared\n", len(open)) //nolint:errcheck // hook protocol
			for index := range open[:min(5, len(open))] {
				fmt.Fprintf(out, "   - %s  (%s)\n", open[index].Gate, open[index].Subject) //nolint:errcheck // hook protocol
			}
		}
	}
	// Every derived artifact the tree does not hold is built here, and the
	// registry is what says which they are. Two hardcoded os.Stat blocks named
	// ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md until 2026-09-11, which made
	// this hook a central enumeration a third artifact had to be added to.
	//
	// ABSENT-ONLY is a budget decision, not an oversight. `.claude/settings.json`
	// gives this hook 5 seconds, and rendering all three artifacts does not fit
	// inside what is LEFT of it. The rendering itself is about a second; the
	// hook's own cost is the rest, most of it the `commit.ListDebt` call above,
	// which reads every shard in plan/verification-debt/ and applies the
	// discharge overlay.
	// That read grows with the ledger, so the margin shrinks on its own, and a
	// loaded machine has none: measured at 2.2s here and at 4.3, 4.8 and 5.9s on
	// the same checkout the same afternoon. Read the rebuild as the change that
	// does not fit, never as the reason the budget is tight
	// (plan/journal/test-gate-repeats-expensive-work.md).
	//
	// A hook killed at its timeout is worse than a stale artifact in
	// two ways at once: every artifact after the kill point is left exactly as
	// it was, and the whole session-start message goes with it, the BLOCKING LSP
	// notice and the verification-debt warning included. Measure before changing
	// this, and put a number here only when you have:
	//
	//	dir=$(./le session scratch ensure)
	//	echo '{}' | time ./le hook-check session-start > "$dir/session-start.log" 2>&1
	//
	// What the bound costs is a STATED limitation rather than a hidden one: a
	// write no Write or Edit hook sees (`sed -i`, a heredoc, `git rebase`, `git
	// stash pop`, a generator) leaves the artifact present and stale until the
	// next hooked write to one of its inputs.
	// docs/contributing/navigating-the-code.md tells the reader so.
	for _, artifact := range derived.All() {
		if _, err := os.Stat(filepath.Join(ctx.root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			continue
		}
		if err := artifact.Rebuild(ctx.root); err != nil {
			fmt.Fprintf(out, "Warning: %s is absent and could not be built: %v\n", artifact.Path, err) //nolint:errcheck // hook protocol
			continue
		}
		fmt.Fprintf(out, "Built %s (derived, not tracked)\n", artifact.Path) //nolint:errcheck // hook protocol
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

// hookCompactionReminder writes the post-compaction reminder. It cannot
// refuse anything, so it answers no exit code.
func hookCompactionReminder(ctx context, errOut io.Writer) {
	message := ctx.payload.Prompt
	if message == "" {
		message = ctx.payload.LastMessage
	}
	lower := strings.ToLower(message)
	if !strings.Contains(lower, "continued from a previous conversation") {
		return
	}
	if !anyContains(lower, "ran out of context", "context compaction") {
		return
	}
	_ = writeSessionMarker(ctx, ".compaction-detected-", time.Now().Format(time.RFC3339))
	fmt.Fprintln(errOut, "⚠ Context compaction detected. Read this session's digest before continuing, then verify every carried claim against source.") //nolint:errcheck // hook protocol
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

// hookPreCompact appends a pre-compaction snapshot to the session state file.
// It cannot refuse anything, so it answers no exit code.
func hookPreCompact(ctx context, errOut io.Writer) {
	path := stateFile(ctx)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the session state path is built from the checkout root
	if err != nil {
		return
	}
	fmt.Fprintf(file, "\n## Pre-compaction snapshot %s\n\nContext was compacted. Re-read current source before trusting earlier conclusions.\n", time.Now().Format(time.RFC3339)) //nolint:errcheck // state record
	_ = file.Close()
	fmt.Fprintf(errOut, "Session state saved to %s\n", path) //nolint:errcheck // hook protocol
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
		if report, _, closureErr := specstatus.CheckClosure(ctx.root, claim); closureErr == nil && report.Blocked() {
			fmt.Fprintln(errOut, "BLOCKED: spec implemented but not closed.") //nolint:errcheck // hook protocol
			fmt.Fprint(errOut, report.Text())                                 //nolint:errcheck // hook protocol
			return 2
		}
		specBody, err := readClaimedSpec(ctx.root, claim)
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

// hookEndSummary rewrites the session recovery snapshot. It cannot refuse
// anything, so it answers no exit code.
func hookEndSummary(ctx context, errOut io.Writer) {
	paths, err := lepath.ResolveSession(ctx.root, false)
	if err != nil {
		fmt.Fprintln(errOut, "session-end-summary: native summary failed; recovery snapshot not updated") //nolint:errcheck // hook protocol
		return
	}
	if _, err := session.EndSummary(ctx.root, paths, time.Now()); err != nil {
		fmt.Fprintln(errOut, "session-end-summary: native summary failed; recovery snapshot not updated") //nolint:errcheck // hook protocol
	}
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

// readClaimedSpec reads the spec a session marker claims. The marker holds the
// file NAME, so specpath answers which bucket holds it.
func readClaimedSpec(root, claim string) ([]byte, error) {
	relative, err := specpath.Find(root, claim)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(relative))) //nolint:gosec // a spec path specpath resolved under the checkout root
}

func hookValidateSpec(ctx context, errOut io.Writer) int {
	if ctx.tool == "" {
		fmt.Fprintln(errOut, "❌ validate-spec: no tool name in the hook payload -- NOTHING WAS CHECKED.\n  This is a PostToolUse hook: it reads a JSON payload on stdin and takes no arguments.") //nolint:errcheck // hook protocol
		return 2
	}
	if !oneOf(ctx.tool, toolWrite, "Edit") {
		return 0
	}
	// Every release bucket, from the one declaration. The predicate was a
	// regex naming plan/ alone, and it answered "not a spec" for
	// plan/immediate/ and plan/pre-release/: every spec in two of the three
	// buckets was written with NO validation, and the hook reported success
	// (ai/rules/evidence.md).
	if !specpath.IsSpec(relativePath(ctx)) {
		return 0
	}
	// The path IS a spec and the write already happened, so a read that fails
	// here validated nothing. Returning 0 is the same failure as the predicate
	// above, one line below its repair: the hook protocol reads it as
	// "checked, allowed" (ai/rules/evidence.md).
	body, err := os.ReadFile(absolutePath(ctx))
	if err != nil {
		fmt.Fprintln(errOut, "❌ validate-spec: "+relativePath(ctx)+" could not be read -- NOTHING WAS CHECKED.\n  "+err.Error()) //nolint:errcheck // hook protocol
		return 2
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
	// The status is read into function scope because one check below is scoped
	// to it. A skeleton is ALLOWED to carry the template's placeholders, which
	// is what plan/README.md states: "skeleton is the one status allowed to
	// carry template placeholders. From design onward the native validation
	// hook blocks them, because the author is then claiming those sections are
	// written."
	status := ""
	if !strings.Contains(text, "| Status |") {
		errors = append(errors, "Missing metadata table. Add Status, Depends, Phase, Updated rows")
	} else {
		statusMatch := regexp.MustCompile(`(?m)^\| Status \| *([a-z-]+)`).FindStringSubmatch(text)
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
	// A skeleton is a spec with no design yet, so an unwritten Entry Point is
	// its honest state rather than a defect. Refusing it here made the hook
	// disagree with plan/README.md and with the tree: 39 committed specs carry
	// this placeholder, so the check refused a state the repository is full of,
	// and the author of a NEW skeleton could not write one at all.
	if status != "skeleton" &&
		(strings.Contains(data, "[Where data enters") || strings.Contains(data, "[Format at entry]")) {
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
