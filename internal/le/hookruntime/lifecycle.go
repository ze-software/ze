// Design: docs/architecture/core-design.md -- native Claude lifecycle hooks
package hookruntime

import (
	"bytes"
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
	airules "github.com/ze-software/ze/internal/le/ai/rules"
	aisync "github.com/ze-software/ze/internal/le/ai/sync"
	"github.com/ze-software/ze/internal/le/commit"
	"github.com/ze-software/ze/internal/le/derived"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/session"
	"github.com/ze-software/ze/internal/le/spec"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
	specjournal "github.com/ze-software/ze/internal/le/spec/journal"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
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
		hookSessionStart(ctx, out, errOut)
	case "compaction-reminder":
		hookCompactionReminder(ctx, errOut)
	case "session-id":
		return runSessionID(ctx, out, errOut), true
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
	return writeMarkerFile(sessionMarker(ctx, prefix), body)
}

// writeMarkerFile writes one marker, holding body or the current time when
// body is empty. An empty path names no marker and writes nothing. A marker
// hook never blocks the tool call it follows, so it always answers 0.
func writeMarkerFile(path, body string) int {
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

// sessionStartBudget is how long the session-start hook reports before it cuts
// what is left. `.claude/settings.json` gives the hook 5 s, and a hook the
// harness kills at that timeout loses everything it printed. The 1.5 s between the two is the margin for starting the
// binary on a loaded machine. Measured 2026-09-26 on this checkout, the whole
// hook took 3.4 to 4.9 s at a load average near 28, and the ledger read was 2.7
// to 3.0 s of it. Once commit.ListDebt stopped forking git four times for each
// discharged commit, the hook took 0.6 to 0.8 s. Tests shorten the budget to
// prove the cut.
var sessionStartBudget = 3500 * time.Millisecond

// sessionStartSteps are the hook's reports, in the order they print. Each one
// writes into its own report and nothing else, so a step cut at the deadline
// leaves no partial line on the hook's output. Tests prepend a slow step.
var sessionStartSteps = []sessionStartStep{
	{name: "tree", run: reportTree},
	{name: "specs", run: reportSpecs},
	{name: "session state", run: reportSessionState},
	{name: "verification debt", run: reportDebt},
	{name: "journal", run: reportDueJournal},
	{name: "derived artifacts", run: buildDerivedArtifacts},
	{name: "agent files", run: reportAgentFiles},
}

// sessionStartStep is one report of the session-start hook.
type sessionStartStep struct {
	name string
	run  func(stdcontext.Context, *sessionStart, *sessionStartReport)
}

// sessionStart is what every step reads. No step writes to it.
type sessionStart struct {
	ctx   context
	claim string
	specs []string
}

// sessionStartReport is what one step prints: out goes to the session, note to
// stderr.
type sessionStartReport struct {
	out  bytes.Buffer
	note bytes.Buffer
}

// hookSessionStart prints the session-start message. The notices every session
// MUST see print FIRST and cost no I/O, so no slow step can take them with it.
// The reports follow under sessionStartBudget, and one the budget cuts is named
// on stderr with the ones after it, instead of the harness dropping the whole
// message at its timeout.
func hookSessionStart(ctx context, out, errOut io.Writer) {
	fmt.Fprintln(out, "Warning: RULE: Read spec + source files BEFORE writing any code")                                                                           //nolint:errcheck // hook protocol
	fmt.Fprintln(out, "Rules: ai/rules/INDEX.md is a one-line overview of every rule -- scan it, read the listed file in full before acting on a topic it covers") //nolint:errcheck // hook protocol
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
	state := sessionStart{ctx: ctx}
	state.claim = readFirstLine(filepath.Join(ctx.root, "tmp", "session", ".session-"+resolvedSessionID(ctx)))
	if state.claim == specUnassigned {
		state.claim = ""
	}
	state.specs, _ = specpath.All(ctx.root)
	if !runSessionStartSteps(&state, sessionStartSteps, sessionStartBudget, out, errOut) {
		return
	}
	if state.claim == "" && len(state.specs) != 0 {
		fmt.Fprintln(out, "Tip: /ze-status for a cross-project attention view") //nolint:errcheck // hook protocol
	}
}

// runSessionStartSteps runs the steps in order on one goroutine and prints each
// report as it arrives, until the budget runs out. It answers false when the
// budget cut a step, after one stderr line names that step and every step after
// it.
//
// The goroutine lives for this one hook run. It runs the steps in order and
// stops before the next step once the deadline passes. The report channel holds
// one slot for each step, so the goroutine never blocks on a send after this
// function returned. A step that reads the context, the git status among them,
// stops at the deadline; one that does not ends with the process.
func runSessionStartSteps(state *sessionStart, steps []sessionStartStep, budget time.Duration, out, errOut io.Writer) bool {
	deadline, cancel := stdcontext.WithTimeout(stdcontext.Background(), budget)
	defer cancel()
	reports := make(chan *sessionStartReport, len(steps))
	go func() {
		for _, step := range steps {
			if deadline.Err() != nil {
				return
			}
			report := &sessionStartReport{}
			step.run(deadline, state, report)
			reports <- report
		}
	}()
	for index := range steps {
		report, arrived := nextSessionStartReport(deadline, reports)
		if !arrived {
			skipped := make([]string, 0, len(steps)-index)
			for _, step := range steps[index:] {
				skipped = append(skipped, step.name)
			}
			fmt.Fprintf(errOut, "session-start: the %s budget ran out, so these reports were skipped: %s\n", budget, strings.Join(skipped, ", ")) //nolint:errcheck // hook protocol
			return false
		}
		out.Write(report.out.Bytes())     //nolint:errcheck // hook protocol
		errOut.Write(report.note.Bytes()) //nolint:errcheck // hook protocol
	}
	return true
}

// nextSessionStartReport answers the next report, and false once the deadline
// passed with no report waiting. A report that is ready wins over a deadline
// that is also ready: select picks between two ready cases at random, and a
// finished step must never print as skipped.
func nextSessionStartReport(deadline stdcontext.Context, reports <-chan *sessionStartReport) (*sessionStartReport, bool) {
	select {
	case report := <-reports:
		return report, true
	case <-deadline.Done():
	}
	select {
	case report := <-reports:
		return report, true
	default:
		return nil, false
	}
}

// reportTree counts the uncommitted files. The git call stops at the hook's
// deadline, and a status git cannot give is said, never read as a clean tree.
func reportTree(deadline stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	command := exec.CommandContext(deadline, "git", "status", "--porcelain")
	command.Dir = state.ctx.root
	status, err := command.Output()
	if err != nil {
		fmt.Fprintf(&report.note, "session-start: git status failed, so the tree was not counted: %v\n", err) //nolint:errcheck // hook protocol
		return
	}
	if strings.TrimSpace(string(status)) == "" {
		fmt.Fprintln(&report.out, "Clean tree") //nolint:errcheck // hook protocol
		return
	}
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
	fmt.Fprintf(&report.out, "Warning: %d uncommitted: %dM %dA\n", len(lines), modified, added) //nolint:errcheck // hook protocol
}

// reportSpecs names this session's claimed spec and the spec population.
func reportSpecs(_ stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	root, claim, specs := state.ctx.root, state.claim, state.specs
	if claim != "" {
		// The claim marker holds the file NAME, so the line has to say which
		// bucket to open. It named plan/<claim> until the release buckets
		// arrived, and that path opened nothing for a spec in two of the three.
		if relative, err := specpath.Find(root, claim); err == nil {
			fmt.Fprintf(&report.out, "SPEC: %s (+%d others)\n   -> READ %s BEFORE any work\n", claim, max(0, len(specs)-1), relative) //nolint:errcheck // hook protocol
		}
	} else if len(specs) != 0 {
		fmt.Fprintf(&report.out, "%d specs, none claimed by this session\n", len(specs)) //nolint:errcheck // hook protocol
	}
	if len(specs) == 0 {
		return
	}
	// The breakdown comes from specstatus so this line and `./le spec
	// status` name one vocabulary in one order. This hook used to keep its
	// own list of seven statuses, which named no default and so counted
	// only what it listed: `done` was absent, and the line under-reported
	// the population it sits beside. StatusPhrases consults no git, which
	// is the reason it exists rather than Collect.
	if phrases, err := specstatus.StatusPhrases(root); err == nil && len(phrases) != 0 {
		fmt.Fprintf(&report.out, "   (%s)\n", strings.Join(phrases, ", ")) //nolint:errcheck // hook protocol
	}
}

// reportSessionState names the claimed spec's session state file.
func reportSessionState(_ stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	if state.claim == "" {
		return
	}
	found := stateFile(state.ctx)
	if _, err := os.Stat(found); err != nil {
		stem := strings.TrimSuffix(strings.TrimPrefix(state.claim, "spec-"), ".md")
		found, _ = spec.LatestStateForSpec(state.ctx.root, stem)
		if found != "" && !filepath.IsAbs(found) {
			found = filepath.Join(state.ctx.root, found)
		}
	}
	if _, err := os.Stat(found); err == nil {
		fmt.Fprintf(&report.out, "Session state: %s\n", found) //nolint:errcheck // hook protocol
	}
}

// reportDebt warns about the open verification-debt rows. commit.ListDebt is
// the one producer of "is this row open", so this step holds no rule of its
// own. A ledger it cannot read is said on stderr, never read as no debt.
func reportDebt(_ stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	debts, err := commit.ListDebt(state.ctx.root)
	if err != nil {
		fmt.Fprintf(&report.note, "session-start: verification debt not read: %v\n", err) //nolint:errcheck // hook protocol
		return
	}
	open := make([]commit.Debt, 0)
	for index := range debts {
		if strings.EqualFold(debts[index].Status, "open") {
			open = append(open, debts[index])
		}
	}
	if len(open) == 0 {
		return
	}
	fmt.Fprintf(&report.out, "Warning: verification debt: %d gate(s) owed, --push is refused until cleared\n", len(open)) //nolint:errcheck // hook protocol
	for index := range open[:min(5, len(open))] {
		fmt.Fprintf(&report.out, "   - %s  (%s)\n", open[index].Gate, open[index].Subject) //nolint:errcheck // hook protocol
	}
}

// reportDueJournal prints one line when journal classes are due for a fix
// pass. A journal it cannot read costs the session this line only: the reason
// goes to stderr and the rest of the hook runs.
func reportDueJournal(_ stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	journal, err := specjournal.Check(state.ctx.root)
	if err != nil {
		fmt.Fprintf(&report.note, "journal: due classes not counted: %s\n", strings.ReplaceAll(err.Error(), "\n", " ")) //nolint:errcheck // hook protocol
		return
	}
	if due := journal.DueCount(); due != 0 {
		fmt.Fprintf(&report.out, "journal: %d problem classes are due for a fix pass (./le spec journal report)\n", due) //nolint:errcheck // hook protocol
	}
}

// buildDerivedArtifacts builds every derived artifact the tree does not hold
// and the registry declares for this hook.
//
// The registry is what says which they are. Two hardcoded os.Stat blocks named
// ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md until 2026-09-11, which made this
// hook a central enumeration a third artifact had to be added to.
//
// ABSENT-ONLY is a budget decision, not an oversight. Rendering all three
// artifacts is about a second, which does not fit beside the hook's other
// reports inside sessionStartBudget on a loaded machine. Measure before
// changing this, and put a number here only when you have:
//
//	dir=$(./le session scratch ensure)
//	echo '{}' | time ./le ai hooks session-start > "$dir/session-start.log" 2>&1
//
// Each artifact is written to a temporary file and renamed, so a render the
// budget cuts leaves the old file or none, never a partial one.
//
// What the bound costs is a STATED limitation rather than a hidden one: a
// write no Write or Edit hook sees (`sed -i`, a heredoc, `git rebase`, `git
// stash pop`, a generator) leaves the artifact present and stale until the
// next hooked write to one of its inputs.
// docs/contributing/navigating-the-code.md tells the reader so.
func buildDerivedArtifacts(deadline stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	for _, artifact := range derived.All() {
		// The artifact declares whether this hook may render it, because the
		// budget above is fixed and the renders are not the same size. An
		// artifact a search reads WITHOUT naming its path has to exist before
		// that search runs; one every reader NAMES is built by
		// preMaterializeDerived on the command that names it, and rendering it
		// here buys nothing for what it spends (derived.SessionStartPolicy).
		if artifact.SessionStart != derived.SessionStartBuild {
			continue
		}
		if deadline.Err() != nil {
			return
		}
		if _, err := os.Stat(filepath.Join(state.ctx.root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			continue
		}
		if err := artifact.Rebuild(state.ctx.root); err != nil {
			fmt.Fprintf(&report.out, "Warning: %s is absent and could not be built: %v\n", artifact.Path, err) //nolint:errcheck // hook protocol
			continue
		}
		fmt.Fprintf(&report.out, "Built %s (derived, not tracked)\n", artifact.Path) //nolint:errcheck // hook protocol
	}
}

// reportAgentFiles warns when the generated agent files are stale.
func reportAgentFiles(_ stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	if mirror, err := (aisync.Mirror{Root: state.ctx.root}).Check(); err != nil || len(mirror.Stale) != 0 {
		fmt.Fprintln(&report.out, "Warning: generated agent files are stale (AGENTS.md / a leftover CLAUDE.md / skills mirrors)") //nolint:errcheck // hook protocol
		fmt.Fprintln(&report.out, "   -> run: ./le ai sync write")                                                                //nolint:errcheck // hook protocol
	}
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

// hookStop judges the session's state when it stops, never the words of its
// last message. A claimed spec that is implemented but not closed refuses the
// stop, because the closure commit is the work still owed. An in-progress
// claim, or a claim with no subagent spawned, is reported in one warning line
// and does not refuse.
func hookStop(ctx context, errOut io.Writer) int {
	id := resolvedSessionID(ctx)
	claim := readFirstLine(filepath.Join(ctx.root, "tmp", "session", ".session-"+id))
	if claim == "" {
		return 0
	}
	if claim == specUnassigned {
		return 0
	}
	if report, _, closureErr := specstatus.CheckClosure(ctx.root, claim); closureErr == nil && report.Blocked() {
		fmt.Fprintln(errOut, "BLOCKED: spec implemented but not closed.") //nolint:errcheck // hook protocol
		fmt.Fprint(errOut, report.Text())                                 //nolint:errcheck // hook protocol
		return 2
	}
	reasons := make([]string, 0, 2)
	specBody, err := readClaimedSpec(ctx.root, claim)
	if err == nil && regexp.MustCompile(`(?m)^\|[ \t]*Status[ \t]*\|.*in-progress`).Match(specBody) {
		reasons = append(reasons, "Spec '"+claim+"' in-progress")
	}
	if _, err := os.Stat(filepath.Join(ctx.root, "tmp", "session", ".agent-spawned-"+id)); err != nil {
		reasons = append(reasons, "Delegation: no subagent spawned")
	}
	if len(reasons) == 0 {
		return 0
	}
	// One line. The transcript renders a non-blocking hook exit verbatim, so a
	// header plus one bullet per reason spends three lines to say what one says.
	fmt.Fprintln(errOut, "Warning: open session state -- "+strings.Join(reasons, "; ")) //nolint:errcheck // hook protocol
	return 1
}

func hookRuleCoverage(ctx context, errOut io.Writer) int {
	id := resolvedSessionID(ctx)
	report, code := airules.RunSessionCoverage(ctx.root, airules.SessionCoverageOptions{
		Quiet: true, Transcript: ctx.transcript, Session: id,
	}, airules.NativeTranscriptSource{}, time.Now, errOut)
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

// readToolLineCap is how many lines the Read tool returns when a call names no
// limit.
const readToolLineCap = 2000

// readWholeFile reports whether a Read call returned every line of its file:
// no offset past the first line, and a limit, or the tool's own cap when there
// is none, that reaches the last line. An offset or a limit that is not a
// number, and a file that cannot be read, answer false, so a read nobody can
// measure is never recorded as whole.
func readWholeFile(ctx context) bool {
	if offset, present := ctx.input["offset"]; present {
		first, ok := offset.(float64)
		if !ok {
			return false
		}
		if first > 1 {
			return false
		}
	}
	limit := float64(readToolLineCap)
	if value, present := ctx.input["limit"]; present {
		named, ok := value.(float64)
		if !ok {
			return false
		}
		limit = named
	}
	body, err := os.ReadFile(absolutePath(ctx)) //nolint:gosec // the path the Read tool call itself named
	if err != nil {
		return false
	}
	lines := bytes.Count(body, []byte("\n"))
	if len(body) != 0 && body[len(body)-1] != '\n' {
		lines++
	}
	return float64(lines) <= limit
}

// hookSourceRead records what a Read tool call read. A read of the Go style
// guide that returned every line writes the marker writeStyleGuideRead asks for, and a read of source
// writes the marker writeDesignEvidence asks for.
func hookSourceRead(ctx context) int {
	if relativePath(ctx) == styleGuidePath {
		if !readWholeFile(ctx) {
			return 0
		}
		return writeMarkerFile(styleGuideMarker(ctx), "")
	}
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
	if strings.Contains(text, "| Status |") {
		statusMatch := regexp.MustCompile(`(?m)^\| Status \| *([a-z-]+)`).FindStringSubmatch(text)
		if len(statusMatch) > 1 {
			status = statusMatch[1]
		}
		if !specstatus.Declared(status) {
			errors = append(errors, "Invalid Status '"+status+"'. Must be: "+strings.Join(specstatus.Vocabulary, ", "))
		}
		if !regexp.MustCompile(`(?m)^\| Updated \| *\d{4}-\d{2}-\d{2}`).MatchString(text) {
			warnings = append(warnings, "Metadata: Updated field should have a date (YYYY-MM-DD)")
		}
	} else {
		errors = append(errors, "Missing metadata table. Add Status, Depends, Phase, Updated rows")
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
