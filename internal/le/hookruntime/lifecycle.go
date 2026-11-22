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
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
	airules "github.com/ze-software/ze/internal/le/ai/rules"
	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
	aisync "github.com/ze-software/ze/internal/le/ai/sync"
	lecommit "github.com/ze-software/ze/internal/le/commit"
	"github.com/ze-software/ze/internal/le/gitenv"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
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
		return hookSourceRead(ctx), true
	case "mark-source-read":
		return hookSourceRead(ctx), true
	case "mark-agent-spawned":
		return hookAgentSpawned(ctx, out), true
	case "validate-spec":
		return hookValidateSpec(ctx, errOut), true
	case "bash-timestamp":
		hookBashTimestamp(ctx, out)
		return 0, true
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
// to 3.0 s of it. Once that read stopped forking git four times for each
// commit it named, the hook took 0.6 to 0.8 s. Tests shorten the budget to
// prove the cut.
var sessionStartBudget = 3500 * time.Millisecond

// sessionStartSteps are the hook's reports, in the order they print. Each one
// writes into its own report and nothing else, so a step cut at the deadline
// leaves no partial line on the hook's output. Tests prepend a slow step.
var sessionStartSteps = []sessionStartStep{
	{name: "tree", run: reportTree},
	{name: "specs", run: reportSpecs},
	{name: "spec sets", run: reportSpecSets},
	{name: "push debt", run: reportDerivedDebt},
	{name: "session state", run: reportSessionState},
	{name: "journal", run: reportDueJournal},
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
	fmt.Fprintln(out, "Warning: RULE: Read spec + source files BEFORE writing any code")                                                                            //nolint:errcheck // hook protocol
	fmt.Fprintln(out, "Rules: ai/rules/TRIGGERS.md names every rule with the situation it covers -- when one matches, read that rule's file in full before acting") //nolint:errcheck // hook protocol
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
	state.claim = specClaim(ctx, resolvedSessionID(ctx))
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
	// The deadline SIGKILLs git, and a status killed while it refreshes the index
	// leaves an orphaned .git/index.lock that blocks every session's git writes.
	command.Env = gitenv.ReadOnly(os.Environ())
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

// reportDerivedDebt prints the derived push debt: one line and at most five
// items, or nothing when nothing is owed. What is owed is
// commit.DebtWarning's answer, which defers to derivedDebt.owed: it warns for
// an open person act and for a push that is refused (its upstream moved, a
// trailer nobody can read), naming the refusal. It never names a verify: no
// push owes one. A derivation that
// cannot answer is said on stderr with its fix, and is never read as nothing
// owed. A linked worktree is the one silence: a worktree agent never pushes,
// so its branch having no upstream is normal.
func reportDerivedDebt(deadline stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	warning, err := lecommit.DebtWarning(state.ctx.root)
	if err == nil {
		report.out.WriteString(warning)
		return
	}
	if linkedWorktree(deadline, state.ctx.root) {
		return
	}
	report.note.WriteString("session-start: the push debt was not derived, so it was not counted: " + err.Error() + "\n")
}

// linkedWorktree answers whether root is a linked worktree: its git dir is
// not the repository's common dir. A git failure answers false, so the
// caller says what it could not derive rather than going silent.
func linkedWorktree(deadline stdcontext.Context, root string) bool {
	command := exec.CommandContext(deadline, "git", "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	command.Dir = root
	output, err := command.Output()
	// refuse-by-default: guards reportDerivedDebt -- false sends it on to name the debt it could not derive, never to silence
	if err != nil {
		return false
	}
	dirs := strings.Fields(string(output))
	if len(dirs) != 2 {
		return false
	}
	return dirs[0] != dirs[1]
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

// sessionStartClaimsBudget is how long the spec sets step waits for the
// claims. session.ListClaims costs a process-table scan and the reaper's walk
// of every idle session directory, which grows with that tree on the host and
// takes no deadline of its own, so the step bounds the wait rather than the
// walk. One second is under a third of sessionStartBudget, leaving the steps
// after it their time on a loaded machine. Tests shorten it to prove the cut.
var sessionStartClaimsBudget = time.Second

// sessionStartListClaims reads every claim with the reaper's verdict on its
// holder. Tests replace it with a fast or a slow fixture.
var sessionStartListClaims = func(root string) ([]session.Claim, error) { return session.ListClaims(root, "") }

// claimsUnreadAtSessionStart is the reason a set's claims carry when the read
// did not finish within sessionStartClaimsBudget.
const claimsUnreadAtSessionStart = "claims not read within the session-start budget"

// reportSpecSets prints the spec sets a session may need to pick up: each
// stalled set with its next child, its claim command and its handover, each
// set whose children all closed, and each started set whose state cannot be
// told, from specstatus.SessionStartSets. It reads the spec files, the git
// index and the shared record of closed specs, and runs no git. The claims are
// read under sessionStartClaimsBudget; past it, or when they cannot be read,
// they pass as not read with the reason, and every started set with an open
// child is named beside `./le spec status set list`, never called stalled or
// idea. It is a step of its own, so a budget cut names it and leaves the specs
// line standing.
func reportSpecSets(deadline stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	if len(state.specs) == 0 {
		return
	}
	claims := sessionStartClaims(deadline, state.ctx.root)
	lines, err := specstatus.SessionStartSets(state.ctx.root, claims)
	if err != nil {
		report.note.WriteString("session-start: the spec sets were not derived, so none was reported: " + err.Error() +
			"; `./le spec status set list` derives them\n")
		return
	}
	for _, line := range lines {
		report.out.WriteString(line + "\n")
	}
}

// sessionStartClaims reads the claims within sessionStartClaimsBudget, or the
// hook's own deadline when that comes first. A read that has not answered by
// then passes as not read, with claimsUnreadAtSessionStart; a read that failed
// passes as not read with its error. Neither fix is set, so both name
// `./le spec status set list`, which reads every claim and judges its holder.
func sessionStartClaims(deadline stdcontext.Context, root string) specstatus.SetClaims {
	bounded, cancel := stdcontext.WithTimeout(deadline, sessionStartClaimsBudget)
	defer cancel()
	answers := make(chan listedClaims, 1)
	// The reader is copied here, so the goroutine never reads the package
	// variable a test restores while the read may still run.
	list := sessionStartListClaims
	go readSessionStartClaims(list, root, answers)
	var answer listedClaims
	select {
	case answer = <-answers:
	case <-bounded.Done():
		// An answer that is ready wins over a deadline that is also ready:
		// select picks between two ready cases at random.
		select {
		case answer = <-answers:
		default:
			return specstatus.ClaimsUnread(claimsUnreadAtSessionStart, "")
		}
	}
	if answer.err != nil {
		return specstatus.ClaimsUnread("the claims could not be read: "+answer.err.Error(), "")
	}
	return specstatus.ClaimsRead(answer.claims)
}

// listedClaims is one answer of sessionStartListClaims.
type listedClaims struct {
	claims []session.Claim
	err    error
}

// readSessionStartClaims is the goroutine sessionStartClaims starts: one
// lifecycle step of one hook run, which makes one read and sends one answer.
// answers MUST hold one slot, so the send never blocks after
// sessionStartClaims returned at its deadline; a read still running then ends
// with the hook's process. list is the caller's copy of sessionStartListClaims.
func readSessionStartClaims(list func(string) ([]session.Claim, error), root string, answers chan<- listedClaims) {
	claims, err := list(root)
	answers <- listedClaims{claims: claims, err: err}
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

// reportAgentFiles warns when the generated agent files are stale.
func reportAgentFiles(_ stdcontext.Context, state *sessionStart, report *sessionStartReport) {
	if mirror, err := (aisync.Mirror{Root: state.ctx.root}).Check(); err != nil || len(mirror.Stale) != 0 {
		fmt.Fprintln(&report.out, "Warning: generated agent files are stale (AGENTS.md / a leftover CLAUDE.md / skills mirrors)") //nolint:errcheck // hook protocol
		fmt.Fprintln(&report.out, "   -> run: ./le ai sync update")                                                               //nolint:errcheck // hook protocol
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
	claim := specClaim(ctx, paths.ID)
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
	claim := specClaim(ctx, id)
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
		Quiet: true, Transcript: ctx.payload.TranscriptPath, Session: id,
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

// noPayloadSessionID is the start text of an agent whose payload carries no
// session_id: it says what bashAgentIdentity's refusal says for that payload.
const noPayloadSessionID = "\nThis start payload names an agent but carries no session id, so you have no identity line: " +
	"the Bash hook refuses every command, readers included. Next: stop and report the payload's session_id to the main thread.\n"

func hookSubagentContext(ctx context, out io.Writer) int {
	// A payload session id that cannot name a file (lepath.ValidSessionID,
	// which bounds its length too) answers "" and is never echoed.
	id, present := payloadSessionID(ctx.payload)
	if !present {
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
	text.Reset().Str("Ze is a Network OS in Go (BGP, CLI, web, plugins). Key constraints:\n- Zero-copy, buffer-first encoding: WriteTo(buf, off) int -- no make/append in encoding\n- Registration pattern: init() in register.go, never direct imports between components\n- YANG required for all RPCs -- no command module category\n- Lazy over eager: pass raw bytes, offset iterators, no intermediate structs\n- JSON keys: kebab-case\n- Goroutines: long-lived workers on channels, never per-event\n- Third parties hook in: pkg/ is a public API, and any plugin (ours included) may run as an external process, so a direct Go call into another component silently does nothing\n- Before your first code edit (.go, .yang, .templ), read ai/doors/system.md and the door covering the file, each in full: the write hook refuses the edit until you have (index: ai/doors/README.md)\n- Rules: ai/rules/\n- Branch: ").Str(startTextBounded(branch)).Byte('\n')
	if id != "" {
		paths, err := lepath.SessionForID(ctx.root, id)
		if err == nil {
			claim := specClaim(ctx, id)
			if claim != "" && claim != specUnassigned {
				relative, err := specpath.Find(ctx.root, claim)
				if err != nil {
					text.Str("\nCannot resolve the session's claimed spec ").Str(startTextBounded(claim)).
						Str(": ").Str(startTextBounded(err.Error())).
						Str("\nAsk the parent session to correct the spec placement or use ./le spec claim to update its claim before acting.\n")
				} else {
					text.Str("\nSpec claimed by the session that spawned you: ").Str(startTextBounded(relative)).
						Str("\nRead it before acting. Its acceptance criteria are what your work is judged against.\n")
				}
			}
			// The id passed lepath.ValidSessionID: no line break, and no longer
			// than startTextErrorMax, so it is printed whole.
			text.Str("\nParent session ID: ").Str(id).
				Str("\nParent session scratch: ").Str(startTextBounded(paths.Scratch))
			// The line is the one bashAgentIdentity compares a command with,
			// and it exists only for a payload that names an agent. A start
			// payload with no agent id gets none: the Bash hook judges a call
			// by its own payload (fromSubagent), reads one with no agent id as
			// the main thread's and asks it for no line, so a line printed
			// here would be one no check compares. Should this agent's Bash
			// calls carry an agent id all the same, the refusal prints the
			// line then. An agent id that cannot name a file owns no line
			// either: bashAgentIdentity refuses every command, readers
			// included, with stop-and-report, at the AgentFingerprint error
			// before it reads the reader set. A payload with no session_id
			// key owns no line either, though id was resolved for it: its
			// text is written below, whether or not paths resolved. The
			// sentence that orders the line typed is printed only where a
			// line follows it.
			agent := ctx.payload.AgentID
			switch {
			case agent == "":
				text.Str("\nThis start payload names no agent id, so you have no identity line to type here. " +
					"The Bash hook asks for one only on a call whose payload names an agent, and its refusal then prints the exact line.\n")
			case !present:
				// No line: the stop-and-report text is written below.
			case lepath.ValidAgentID(agent):
				// Both ids passed the one id check, which bounds their length
				// under startTextErrorMax, so the line is printed whole: a cut
				// id would make a line the Bash hook refuses.
				text.Str("\nStart every Bash command that runs anything but readers (grep, sed, cat, a git read verb) with your identity line, exactly as printed. The Bash hook checks the line and never adds it:\n").
					Str(identityLine(agent, id)).Byte('\n')
			default:
				text.Str("\nThis start payload's agent id cannot name a file: ").Str(lepath.IDRule()).
					Str(" So you have no identity line: the Bash hook refuses every command, readers included. " +
						"Next: stop and report the agent id to the main thread.\n")
			}
		}
	}
	// A subagent whose payload carries no session_id key, or one that cannot
	// name a file, has no identity line: bashAgentIdentity reads the payload's
	// own session id and refuses its every command, readers included, with
	// stop-and-report. The start text says so rather than print a line built
	// from an id the hook rejects, or say nothing. A missing key says so
	// whether a parent session resolved or not, and whether or not its paths
	// did: the text depends on the payload alone, as the refusal does.
	if ctx.payload.AgentID != "" {
		switch {
		case !present:
			text.Str(noPayloadSessionID)
		case id == "":
			text.Str("\nThis start payload's session id cannot name a file: ").Str(lepath.IDRule()).
				Str(" So you have no parent session and no identity line: the Bash hook refuses every command, readers included. " +
					"Next: stop and report the payload's session id to the main thread.\n")
		}
	}
	subagentCallRules(&text)
	text.Str("\nYou are a subagent under ai/rules/planning.md. Report grounded facts, read routed rules before acting, never weaken tests, and write scratch only under ./le session scratch ensure.\n")
	start, err := ruleset.ReadAgentStart(ctx.root, ctx.payload.AgentType)
	switch {
	case err != nil:
		text.Str("\nCannot read your agent definition (").Str(startTextBounded(err.Error())).
			Str("), so this start text cannot tell whether you loaded the project instruction files. If ").
			Str(ruleset.InstructionsFile).Str(" is not in your context, read its \"# ").Str(ruleset.HardBansHeading).
			Str("\" section, ").Str(ruleset.CoreFile).Str(", ").Str(ruleset.TriggersFile).Str(" and ").Str(ruleset.DecisionsFile).
			Str(" before acting.\n")
	case start.Omits():
		agentRuleSet(&text, ctx.root, ctx.payload.AgentType)
	}
	response := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "SubagentStart", "additionalContext": text.String()}}
	_ = json.NewEncoder(out).Encode(response)
	return 0
}

// subagentCallRules writes the seven rules a subagent's calls are judged by,
// each one a check it would otherwise learn from a refusal that costs it a
// turn: the whole-document read the edit gates record (readgate.go), the
// shell-write guard on plan/ and ai/rules/ (bashGovernedWrite), the batching
// a turn rewards, the heavy work the Bash hook refuses and the owed-command
// list that replaces it (bashSubagentHeavy), the lint its Go edit owes
// (postFormatGo), what it may commit, and the call budget
// (agentCallBudgetCheck), whose numbers come from its own constants.
func subagentCallRules(text *textbuf.Buffer) {
	text.Str("\nHow your calls are judged:\n").
		Str("- Read a door or the Go style guide with the Read tool, whole (no offset or limit): only a whole read is recorded, and the write hooks refuse a code edit until it is. Read a spec's source files whole too.\n").
		Str("- Edit files under plan/ and ai/rules/ with Edit or Write only: the Bash hook refuses a shell write there.\n").
		Str("- Put independent calls in one message: every call re-reads your whole context.\n").
		Str("- Hand tests, builds, lint and formatters to the main thread: the Bash hook refuses a test, build, vet, lint, formatter, `./le job run` and any heavy `./le` action from you. End your report with an `Owed commands` list (each exact command, what it proves, the result you expect), and the main thread runs it.\n").
		Str("- After your Go edit the hook formats the file and does not lint it, so the changed-code lint is owed: `./le go lint run scope \"<pkg>\"`.\n").
		Str("- Commit only a change that owes no execution (prose, spec bookkeeping, a blocked hand-back). The main thread commits code.\n").
		Str("- A ").Str(budgetedAgentType).Str(" agent's budget is ").Int(agentCallBudget).
		Str(" hooked calls (Bash, Write, Edit). A call a hook refuses is not counted.\n")
}

// agentRuleSetHeading is the heading of the start text that carries the agent
// rule set. The agent definitions' fallback paragraph names it: an agent whose
// start text lacks it reads the files itself. The files, the part headings and
// their reader are internal/le/ai/rules/ruleset's, the same reader the payload
// report measures with.
const agentRuleSetHeading = "Agent rule set"

// startTextErrorMax bounds, in bytes, a value the start text echoes that a
// payload or the checkout chooses: a claim, an error naming a path, the branch,
// the spec path. The session and agent ids are not cut: lepath.ValidSessionID
// and lepath.ValidAgentID bound them below this, and a cut id would print an
// identity line the Bash hook refuses. The start text shares the
// harness's 10,000-character cap with the rule set, so an unbounded echo could
// push the bans past it.
const startTextErrorMax = 240

// startTextSectionsMax bounds, in bytes, the list of decisions sections for
// one kind of work the start text carries: a heading, a range and a situation
// a line. A situation is never cut, because the words cut off could be the
// ones that name an agent's work, so a list past the bound sends the whole
// decisions file instead, and a longer list never pushes the bans past the
// harness cap.
const startTextSectionsMax = 2350

// agentInlineHardBansMarker identifies an agent definition that already embeds
// the verified `# Hard Bans` block (checked by
// TestAgentDefinitionsCarryTheHardBansOfTheInstructions), so SubagentStart
// does not inject a second verbatim copy into additionalContext.
const agentInlineHardBansMarker = "<!-- hard-bans: begin"

// startTextOneLine answers value with each carriage return and line feed
// replaced by a space, so a value the checkout chooses adds no line to the
// start text. Unlike startTextBounded it never cuts.
func startTextOneLine(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
}

// startTextBounded answers value on one line, each carriage return and line
// feed replaced by a space so an echoed value adds no line to the start text,
// cut to startTextErrorMax bytes on a rune boundary, marked "..." when it was
// cut.
func startTextBounded(value string) string {
	value = startTextOneLine(value)
	if len(value) <= startTextErrorMax {
		return value
	}
	end := startTextErrorMax
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "..."
}

// agentRuleSet writes what an agent that starts without the project
// instruction files needs at its first turn: the hard bans, read from
// ai/INSTRUCTIONS.md at each start, and the one read that brings the rest,
// with each decisions section an agent reads only before the work its
// situation names, listed by heading, line range and situation. A heading and
// a situation are file content: each is kept to one line, the heading is cut
// like any echoed value, and the whole list is bounded by
// startTextSectionsMax.
// The rest is named, never sent: the harness caps a hook's additionalContext
// at 10,000 characters and replaces a longer one with a file path it never
// asks the agent to read, and the core, the trigger index and the decisions
// are several times that.
func agentRuleSet(text *textbuf.Buffer, root, agentType string) {
	text.Str("\n# ").Str(agentRuleSetHeading).
		Str("\nYour definition omits the project instruction files, so this start text carries the hard bans and names the rules to read.\n\n")
	if agentDefinitionCarriesHardBans(root, agentType) {
		text.Str("Your agent definition already carries the verified \"# ").Str(ruleset.HardBansHeading).
			Str("\" section of ").Str(ruleset.InstructionsFile).Str(".\n")
	} else {
		bans, err := ruleset.HardBans(root)
		if err != nil {
			text.Str("Cannot read the hard bans: ").Str(startTextBounded(err.Error())).
				Str("\nRead the \"# ").Str(ruleset.HardBansHeading).Str("\" section of ").Str(ruleset.InstructionsFile).Str(" before acting.\n")
		} else {
			text.Str(strings.TrimSpace(bans.Text)).Byte('\n')
		}
	}
	text.Str("\nBefore any other action, read with the Read tool ").Str(ruleset.CoreFile).Str(" and ").Str(ruleset.TriggersFile).Str(" whole, and ")
	part, err := ruleset.AgentDecisions(root)
	if err != nil {
		text.Str(ruleset.DecisionsFile).Str(" whole: its every-agent part cannot be found, because ").Str(startTextBounded(err.Error())).Str(".\n")
		return
	}
	// Sections that cannot be listed send the whole file: a range alone would
	// read as "nothing else to read" to an agent about to do that work.
	sections, err := ruleset.SituationalDecisions(root)
	if err != nil {
		text.Str(ruleset.DecisionsFile).Str(" whole: its sections for one kind of work cannot be listed, because ").Str(startTextBounded(err.Error())).Str(".\n")
		return
	}
	var listed textbuf.Buffer
	for _, section := range sections {
		listed.Str("- ").Str(startTextBounded(section.Heading)).
			Str(" (lines ").Int(section.First).Byte('-').Int(section.Last).
			Str(", offset ").Int(section.First).Str(", limit ").Int(section.Last - section.First + 1).
			Str("): ").Str(startTextOneLine(section.Situation)).Byte('\n')
	}
	list := listed.String()
	if len(list) > startTextSectionsMax {
		text.Str(ruleset.DecisionsFile).Str(" whole: the list of its sections for one kind of work is ").Int(int64(len(list))).
			Str(" bytes, more than the ").Int(startTextSectionsMax).Str(" this start text carries.\n")
		return
	}
	text.Str(ruleset.DecisionsFile).Str(" lines ").Int(part.First).Byte('-').Int(part.Last).
		Str(" (offset ").Int(part.First).Str(", limit ").Int(part.Last - part.First + 1).
		Str("), the part headed \"").Str(ruleset.DecisionsAgentPartHeading).
		Str("\".\nBefore work one of these situations names, read that section of ").Str(ruleset.DecisionsFile).Str(" too:\n").
		Str(list).
		Str("The rest of that file binds the main session only.\n")
}

// agentDefinitionCarriesHardBans reports whether the mirrored agent definition
// for agentType already embeds the verified hard-bans block, so SubagentStart
// avoids sending a duplicate 2.9 KB copy on every subagent spawn.
func agentDefinitionCarriesHardBans(root, agentType string) bool {
	if !lepath.ValidAgentID(agentType) {
		return false
	}
	body, err := os.ReadFile(filepath.Join(root, ruleset.AgentDefinitionsDir, agentType+".md")) //nolint:gosec // validated agent type name under checkout mirror dir
	if err != nil {
		return false
	}
	return bytes.Contains(body, []byte(agentInlineHardBansMarker))
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

// hookSourceRead records what a Read tool call read. A read of a gated
// document (the Go style guide, or a page under ai/doors) that returned every
// line writes the marker writeStyleGuideRead, writeDesignDoorsRead and
// writeSpecDoorsRead ask for; a read of any other markdown page records every
// section it returned whole (recordSectionRead), which the Design decisions
// gate in unreadDoors asks for; and a read of source writes the marker
// writeDesignEvidence asks for. The record names the checkout the document
// lies in (readDocument): the session's root, or a worktree or other checkout
// of Ze, whose edits that read then counts for. A copy kept anywhere else
// records nothing.
func hookSourceRead(ctx context) int {
	if checkout, document, ok := readDocument(ctx, absolutePath(ctx)); ok {
		if !readWholeFile(ctx) {
			return 0
		}
		return writeMarkerFile(documentReadMarker(ctx, checkout, document), "")
	}
	if strings.HasSuffix(absolutePath(ctx), ".md") {
		return recordSectionRead(ctx)
	}
	return recordSourceRead(ctx)
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
	if !oneOf(ctx.tool, toolWrite, toolEdit, toolMultiEdit) {
		return 0
	}
	// Current discovery never falls back to the obsolete layout. A write there
	// must report the misplaced work rather than silently skip validation.
	relative := relativePath(ctx)
	if !specpath.IsSpec(relative) {
		if _, historical := specpath.HistoricalBucket(relative); historical {
			fmt.Fprintln(errOut, "validate-spec: "+relative+" is in an obsolete spec directory; move it to a current category listed in plan/README.md.") //nolint:errcheck // hook protocol
			return 2
		}
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
	// validateSpecText already records metadata errors. Only a successfully
	// parsed skeleton may omit the owner audit; malformed metadata may not.
	auditOwners := true
	if status, metadataErr := specstatus.MetadataStatus(string(body)); metadataErr == nil {
		auditOwners = status != specStatusSkeleton
	}
	if auditOwners {
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
	status, metadataErr := specstatus.MetadataStatus(text)
	if metadataErr != nil {
		errors = append(errors, "Spec metadata: "+metadataErr.Error())
	}
	if !regexp.MustCompile(`(?m)^\| Updated \| *\d{4}-\d{2}-\d{2}`).MatchString(text) {
		warnings = append(warnings, "Metadata: Updated field should have a date (YYYY-MM-DD)")
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
	if status != specStatusSkeleton &&
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
	// A spec names how its work is tested, not the full gate: agents run
	// targeted tests and never a `./le verify` command, and the full run is the
	// release check GitHub CI runs and Thomas starts (ai/META-DECISIONS.md).
	// Any `./le test` or `./le verify` command answers, so the specs that name
	// `./le verify worktree` still validate.
	if !strings.Contains(text, "./le test ") && !strings.Contains(text, "./le verify ") {
		errors = append(errors, "Missing verification checklist item: the spec names no `./le test` or `./le verify` command, "+
			"so nothing says how the work is proven. Name the targeted tests in the Checklist, for example "+
			"`./le test unit <group>` or `./le test functional <suite>`")
	}
	if regexp.MustCompile("```(go|rust|java|c|cpp|javascript|typescript)").MatchString(text) {
		errors = append(errors, "Specs MUST NOT contain code blocks. Use tables/prose instead")
	}
	wiring := markdownSection(text, "## Wiring Test")
	if wiring != "" && !regexp.MustCompile(`\|.*(→|->).*\|`).MatchString(wiring) {
		errors = append(errors, "Wiring Test section must have table with Entry Point -> Feature Code -> Test columns")
	}
	if metadataErr == nil {
		if problem := daemonProofProblem(root, text); problem != "" {
			errors = append(errors, problem)
		}
	}
	if !strings.Contains(text, "## Acceptance Criteria") {
		warnings = append(warnings, "Missing '## Acceptance Criteria' section. Define testable AC-N assertions before implementation")
	}
	if !strings.Contains(text, "## Risks & Assumptions") {
		warnings = append(warnings, "Missing '## Risks & Assumptions' section")
	}
	gapErrors, gapWarnings := validateSpecRFCGaps(root, text)
	errors = append(errors, gapErrors...)
	warnings = append(warnings, gapWarnings...)
	return errors, warnings
}

// validateSpecRFCGaps refuses an `RFC gaps` item that names no requirement id
// and no summary stem of the working tree's rfc/short/ (AC-16 of
// spec-rfc-priority-tree): the roadmap places a spec in a release profile
// through that item, so an unknown one would silently place it in none. A
// spec that declares no gaps reads no summary. An index that cannot be read
// is an error too: an unjudged cell is not a valid one.
func validateSpecRFCGaps(root, text string) ([]string, []string) {
	warnings := make([]string, 0)
	gaps := specstatus.RFCGaps([]byte(text), "this spec", func(message string) { warnings = append(warnings, message) })
	if len(gaps) == 0 {
		return nil, warnings
	}
	requirementStems, err := rfc.LoadRequirementStems(root)
	if err != nil {
		return []string{"RFC gaps could not be judged, because the rfc/short/ index did not load: " + err.Error() +
			". Fix that summary (./le rfc check names it), then save the spec again"}, warnings
	}
	paths, err := filepath.Glob(filepath.Join(root, "rfc", "short", "*.md"))
	if err != nil {
		return []string{"RFC gaps could not be judged: " + err.Error()}, warnings
	}
	summaryStems := make(map[string]bool, len(paths))
	for _, summary := range paths {
		summaryStems[strings.TrimSuffix(filepath.Base(summary), ".md")] = true
	}
	_, unknown := specstatus.ResolveRFCGaps(gaps, requirementStems, summaryStems)
	errors := make([]string, 0, len(unknown))
	for _, item := range unknown {
		errors = append(errors, specstatus.UnknownRFCGap("this spec", item))
	}
	return errors, warnings
}

// markdownSection answers the section a spec heading opens: the rest of the
// first line that starts with heading (leading blanks aside), through the line
// before the next heading of the same level, and "" when no line opens it. The
// heading must open its line. A substring match also found `## Current
// Behavior` inside `### Current Behavior` under a historical record, and inside
// prose quoting the heading, and then swept every later deeper heading into the
// section: the Data Flow of spec-release-distribution.md was judged as Current
// Behavior (plan/learned/HOOK-FRICTION.md, 2026-11-15).
func markdownSection(text, heading string) string {
	marker := strings.Repeat("#", strings.Count(strings.Fields(heading)[0], "#")) + " "
	lines := strings.Split(text, "\n")
	for start, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, heading) {
			continue
		}
		body := lines[start:]
		body[0] = strings.TrimPrefix(trimmed, heading)
		end := len(body)
		for index, next := range body[1:] {
			if strings.HasPrefix(strings.TrimSpace(next), marker) {
				end = index + 1
				break
			}
		}
		return strings.Join(body[:end], "\n")
	}
	return ""
}
