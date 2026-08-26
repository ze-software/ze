// Design: ai/rules/context-economy.md -- the token-economy command
// Overview: tokeneconomy.go -- the store read, report.go -- what it answers
//
// run.go reads command keywords, resolves their store, and reports its sessions.
//
// The store is machine-local input outside the checkout. Another program writes
// it, and it can grow during a run. The answer therefore includes its path and
// project slug. A figure without its corpus is not a measurement.

package tokeneconomy

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/lepath"
)

// Each value requires a keyword (ai/rules/cli.md). This also avoids the script's
// argument workaround. Absolute paths produce project slugs that ALWAYS start
// with a hyphen, which argparse treats as another option. Keyword grammar makes
// the value unambiguous.
const (
	rootKeyword    = "root"
	projectKeyword = "project"
	capKeyword     = "cap"
	topKeyword     = "top"
	sessionKeyword = "session"
)

// Options is what a caller asked for. An empty Root or Project is filled in
// from the machine and the checkout when the run starts.
type Options struct {
	Root    string
	Project string
	Cap     int
	Top     int
	Session string
}

// Defaults answers the options a caller who names nothing gets.
func Defaults() Options { return Options{Cap: DefaultCap, Top: DefaultTop} }

// Run reads the store the options name and answers the report over it.
//
// Every report state exits 0, including an absent store. Transcripts are
// machine-local and can be absent in a fresh checkout. Failure on no data would
// make this report a gate.
func Run(opts Options) (Report, int) {
	report := Report{
		State:   StateOK,
		Project: opts.Project,
		Store:   filepath.Join(opts.Root, opts.Project),
		Session: opts.Session,
		Cap:     opts.Cap,
		Top:     opts.Top,
	}

	if !isDir(report.Store) {
		report.State = StateAbsent
		return report, 0
	}

	found, unreadable := FindSessions(report.Store)
	report.Unreadable = unreadable

	sessions := make([]Session, 0, len(found))
	for _, session := range found {
		if len(session.AllCalls()) > 0 {
			sessions = append(sessions, session)
		}
	}
	if opts.Session != "" {
		matched := make([]Session, 0, len(sessions))
		for _, session := range sessions {
			if strings.HasPrefix(session.SID, opts.Session) {
				matched = append(matched, session)
			}
		}
		sessions = matched
		if len(sessions) == 0 {
			report.State = StateUnmatched
			return report, 0
		}
	}
	if len(sessions) == 0 {
		report.State = StateEmpty
		return report, 0
	}

	report.fill(sessions)
	return report, 0
}

// isDir reports whether a path names a readable directory. An absent store and
// a FILE store both mean that the named path contains nothing to measure.
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// fill computes every table of the report from the sessions that survived the
// filter.
func (r *Report) fill(sessions []Session) {
	everyCall := []Call{}
	mainCalls := []Call{}
	subCalls := []Call{}
	agents := []Agent{}
	everyTool := []ToolCall{}
	mainTools := []ToolCall{}
	for _, session := range sessions {
		everyCall = append(everyCall, session.AllCalls()...)
		mainCalls = append(mainCalls, session.MainCalls...)
		subCalls = append(subCalls, session.SubagentCalls()...)
		agents = append(agents, session.Agents...)
		everyTool = append(everyTool, session.AllToolCalls()...)
		mainTools = append(mainTools, session.MainToolCalls...)
	}

	r.Sessions = len(sessions)
	r.Subagents = len(agents)
	r.Totals = Aggregate(everyCall)
	r.MainCalls = len(mainCalls)
	r.SubagentCalls = len(subCalls)
	r.SubagentContext = Aggregate(subCalls).Context

	ranked := append([]Session(nil), sessions...)
	context := make(map[string]int64, len(ranked))
	for _, session := range ranked {
		context[session.SID] = Aggregate(session.AllCalls()).Context
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return context[ranked[i].SID] > context[ranked[j].SID]
	})
	if len(ranked) > r.Top {
		ranked = ranked[:r.Top]
	}
	for _, session := range ranked {
		agg := Aggregate(session.AllCalls())
		r.Listed = append(r.Listed, SessionRow{
			SID:         session.SID,
			Calls:       agg.Calls,
			Main:        len(session.MainCalls),
			Sub:         len(session.SubagentCalls()),
			Agents:      len(session.Agents),
			MeanContext: agg.ContextMean,
			MaxContext:  agg.ContextMax,
			CacheRead:   agg.CacheRead,
			CacheWrite:  agg.CacheWrite,
			Output:      agg.Output,
		})
	}

	// A transcript can record calls whose every usage field is zero, so the
	// share is guarded on the DENOMINATOR rather than on the call list.
	if len(subCalls) > 0 && r.Totals.Context != 0 {
		r.ShowSubagentShare = true
		r.SubagentShare = 100.0 * float64(r.SubagentContext) / float64(r.Totals.Context)
	}

	r.Histogram = Histogram(everyCall)
	r.MainHistogram = Histogram(mainCalls)
	r.SubHistogram = Histogram(subCalls)
	r.Capped = CappedCounterfactual(everyCall, r.Cap)

	if len(agents) > 0 {
		r.Phases = phaseRows(agents, r.SubagentContext)
		r.AgentTypes = AgentTypeStartup(agents)
	}

	if len(everyTool) == 0 {
		return
	}
	r.ToolsPerCall = ToolCallDistribution(everyCall)
	r.SingleTool = SingleToolShare(everyCall)
	r.Results = ResultSizes(everyTool)
	for _, session := range sessions {
		for _, thread := range session.Threads() {
			reads := RepeatReads(thread)
			r.ThreadReads.Reads += reads.Reads
			r.ThreadReads.Repeats += reads.Repeats
		}
		reads := RepeatReads(session.AllToolCalls())
		r.SessionReads.Reads += reads.Reads
		r.SessionReads.Repeats += reads.Repeats
	}
	r.MainToolCalls = len(mainTools)
	for _, tool := range mainTools {
		if tool.Name == "Bash" {
			r.MainBash++
		}
	}
}

// phaseRows groups agents by phase. It uses the script's tuple order: descending
// context, then descending phase name for equal context.
func phaseRows(agents []Agent, subContext int64) []PhaseRow {
	order := []string{}
	byPhase := map[string][]Agent{}
	for _, agent := range agents {
		phase := agent.Phase()
		if _, seen := byPhase[phase]; !seen {
			order = append(order, phase)
		}
		byPhase[phase] = append(byPhase[phase], agent)
	}

	rows := make([]PhaseRow, 0, len(order))
	for _, phase := range order {
		members := byPhase[phase]
		calls := []Call{}
		for _, agent := range members {
			calls = append(calls, agent.Calls...)
		}
		agg := Aggregate(calls)
		share := 0.0
		if subContext != 0 {
			share = 100.0 * float64(agg.Context) / float64(subContext)
		}
		rows = append(rows, PhaseRow{
			Phase:        phase,
			Agents:       len(members),
			Calls:        agg.Calls,
			CallsPerAgnt: float64(agg.Calls) / float64(len(members)),
			Context:      agg.Context,
			Share:        share,
			MeanContext:  agg.ContextMean,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Context != rows[j].Context {
			return rows[i].Context > rows[j].Context
		}
		return rows[i].Phase > rows[j].Phase
	})
	return rows
}

// Answer is the `le token-economy` command.
func Answer(args []string) (any, int) {
	opts, code := ParseOptions(args)
	if code != 0 {
		return nil, code
	}

	if opts.Root == "" {
		root, err := DefaultRoot()
		if err != nil {
			return nil, refuse("no home directory, so no transcript store: ", err.Error())
		}
		opts.Root = root
	}
	if opts.Project == "" {
		slug, err := DefaultProject()
		if err != nil {
			return nil, refuse("no checkout, so no project slug: ", err.Error())
		}
		opts.Project = slug
	}

	report, code := Run(opts)
	for _, path := range report.Unreadable {
		// Report unreadable transcripts instead of dropping them. The script
		// silently lowers all following figures, which makes a lower number look
		// like a pass.
		var line textbuf.Buffer
		writeErr(line.Str("warning: ").Str(area).Str(": unreadable transcript, its calls are missing from every figure below: ").
			Str(path).Byte('\n').String())
	}
	return &report, code
}

// DefaultProject answers the checkout's Claude Code project slug.
//
// It first resolves symlinks because the harness names the store from its
// startup directory. The script resolves its own path the same way.
func DefaultProject() (string, error) {
	root, err := lepath.Root()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return SlugForPath(root), nil
}

// ParseOptions reads the keywords this command takes. It answers the options
// and 0, or the zero options and the exit code of a refusal already printed.
func ParseOptions(args []string) (Options, int) {
	opts := Defaults()
	for index := 0; index < len(args); index += 2 {
		keyword := args[index]
		if index+1 >= len(args) {
			return Options{}, refuse("this keyword takes a value: ", keyword)
		}
		value := args[index+1]
		switch keyword {
		case rootKeyword:
			opts.Root = value
		case projectKeyword:
			opts.Project = value
		case sessionKeyword:
			opts.Session = value
		case capKeyword:
			number, code := bounded(capKeyword, value, CapMin, CapMax)
			if code != 0 {
				return Options{}, code
			}
			opts.Cap = number
		case topKeyword:
			number, code := bounded(topKeyword, value, TopMin, TopMax)
			if code != 0 {
				return Options{}, code
			}
			opts.Top = number
		default:
			return Options{}, refuse("no such keyword: ", keyword)
		}
	}
	return opts, 0
}

// bounded parses one numeric keyword and applies the script's range. Tests
// compare bounds by value because ordinary output fixtures do not expose them.
func bounded(keyword, raw string, low, high int) (int, int) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		var what textbuf.Buffer
		return 0, refuse(what.Str(keyword).Str(" must be an integer, got ").String(), raw)
	}
	if value < low || value > high {
		var what textbuf.Buffer
		what.Str(keyword).Str(" must be between ").Int(int64(low)).Str(" and ").
			Int(int64(high)).Str(", got ")
		return 0, refuse(what.String(), raw)
	}
	return value, 0
}

// usageLine is the one line a refusal prints under itself.
func usageLine() string {
	var tb textbuf.Buffer
	return tb.Str("usage: le ").Str(area).
		Str(" [root <path>] [project <slug>] [cap <n>] [top <n>] [session <prefix>]").
		Str(" [| json | yaml | table]").String()
}

// refuse prints one refusal and answers its exit code.
//
// The caller provides the error description and offending word separately. This
// function formats the full line.
func refuse(what, word string) int {
	var line textbuf.Buffer
	writeErr(line.Str("error: ").Str(area).Str(": ").Str(what).Str(word).Byte('\n').String())
	var usage textbuf.Buffer
	writeErr(usage.Str(usageLine()).Byte('\n').String())
	return 1
}

// writeErr puts one finished line on stderr. The engine owns stdout, so a
// refusal and a warning are the only things this tool writes itself.
func writeErr(line string) {
	if _, err := os.Stderr.WriteString(line); err != nil {
		return // stderr is unavailable; there is nowhere left to report it
	}
}
