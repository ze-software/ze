// Design: ai/rules/context-economy.md -- where a session's tokens go
// Detail: aggregate.go -- the figures derived from what this file reads
// Detail: run.go -- the command that resolves a store and fills the report
//
// Package tokeneconomy measures token use by this repository's Claude Code
// sessions. An agent session pays context cost on every API call. Total cost
// therefore depends on call count and context size.
//
// Store layout (read-only, written by Claude Code):
//
//	~/.claude/projects/<project-slug>/<session>.jsonl
//	~/.claude/projects/<project-slug>/<session>/subagents/agent-<id>.jsonl
//	~/.claude/projects/<project-slug>/<session>/subagents/agent-<id>.meta.json
//
// THE INPUT LIVES OUTSIDE THE CHECKOUT. Other le tools inspect their current
// tree. This tool reads a growing, machine-local store that commits do not
// change. It reports an absent store instead of zero cost.
//
// ONE API CALL PRODUCES SEVERAL ASSISTANT RECORDS with one message id. Counting
// records inflates context totals. One transcript had 168 records for 98 calls.
// A 26-file sample had 3,613 records for 1,732 calls.
//
// Records with one id differ. Context fields repeat, while output count grows.
// Only the last record has the final output count. ScanTranscript therefore
// merges each field by maximum.
//
// ONE API CALL CAN APPEAR IN SEVERAL TRANSCRIPTS. Resumed sessions and forked
// agents copy older records into their files. AssignOwners selects one transcript
// for each id across the store.
//
// The report contains token counts, not prices. Converting them to money requires
// an external price list that this tool does not own.
package tokeneconomy

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the command word. It omits the report verb from gate name
// ze-token-economy-report.
const area = "token-economy"

// These are numeric bounds and defaults from the script. Tests compare
// CAP_MIN, CAP_MAX, TOP_MIN, TOP_MAX, DEFAULT_CAP, and DEFAULT_TOP by VALUE.
// Output comparison cannot detect a bound unless its fixture reaches that bound.
const (
	CapMin     = 1
	CapMax     = 1_000_000
	TopMin     = 1
	TopMax     = 1000
	DefaultCap = 200_000
	DefaultTop = 15
)

// CharsPerToken estimates tokens from recorded result TEXT. Each derived result
// is approximate, and each output line identifies that approximation.
const CharsPerToken = 3.6

// ResultTableRows limits named tools before the result-size table combines the
// remaining tools. This keeps the long one-off tail below high-volume tools.
const ResultTableRows = 10

// BucketEdges contains token upper bounds for context-size buckets. The final
// bucket is open.
var BucketEdges = []int64{50_000, 100_000, 200_000, 400_000, 600_000, 800_000, 1_000_000}

// PhaseRule is one row of the phase classification.
type PhaseRule struct {
	Phase    string
	Keywords []string
}

// PhaseRules classify a spawn into a workflow phase. This is a KEYWORD
// HEURISTIC over the spawn description, not a recorded fact: nothing in the
// store labels an agent with the phase it ran. FIRST MATCH WINS, so the order
// below is the ranking, and it is behavior rather than style. A description
// matching nothing is unclassified rather than silently folded into a
// neighboring phase.
var PhaseRules = []PhaseRule{
	{"review", []string{"review", "critique", "adversarial", "lens", "referee"}},
	{"audit", []string{"audit", "conformance"}},
	{"fix/debug", []string{"debug", "fix", "repair", "failing", "diagnose", "flake", "red "}},
	{"test", []string{"test", "coverage", "fixture", "reproduce"}},
	{"implement", []string{"implement", "build", "wire", "port", "migrate", "refactor", "add "}},
	{"docs/rules", []string{"doc", "rule", "spec", "learned", "summar", "index", "write"}},
	{"research", []string{
		"research", "explore", "investigate", "find", "search", "survey",
		"classify", "measure", "check", "verify", "read", "map", "trace",
	}},
}

// Unclassified is the phase of a spawn whose description matches no rule, and
// of a spawn that carries no description at all.
const Unclassified = "unclassified"

// Call is one API call, from the deduped usage of its assistant records.
type Call struct {
	Input      int64 `json:"input"`
	CacheWrite int64 `json:"cache-write"`
	CacheRead  int64 `json:"cache-read"`
	Output     int64 `json:"output"`
	Tools      int   `json:"tools"`
}

// Context answers the tokens fed to the model on this call, which is what the
// call is charged for.
func (c Call) Context() int64 { return c.Input + c.CacheWrite + c.CacheRead }

// ToolCall is one tool_use block, with the size of the result fed back for it.
//
// ResultChars is 0 when no tool_result names the block: the tool was denied,
// the transcript ends before the result, or the result carried no text. That is
// a real zero and is counted as one, never dropped.
type ToolCall struct {
	Name        string `json:"name"`
	FilePath    string `json:"file-path,omitempty"`
	ResultChars int    `json:"result-chars"`
}

// Agent is one subagent transcript plus the spawn metadata beside it.
type Agent struct {
	Name        string
	AgentType   string
	Description string
	Calls       []Call
	ToolCalls   []ToolCall
	IsFork      bool
	PromptChars int
}

// Phase classifies this agent's spawn description.
func (a Agent) Phase() string { return PhaseOf(a.Description, a.AgentType) }

// HarnessFloor answers first-call context after removal of the spawn prompt. It
// measures system prompts, tool schemas, and SubagentStart injection.
//
// Prompt subtraction makes agent comparisons stable as sessions grow.
// Otherwise, the figure measures parent prompt length and cannot describe an
// agent TYPE.
//
// The caller excludes forks. A fork inherits its parent's context, so none of
// its first call is a harness floor.
func (a Agent) HarnessFloor() int64 {
	if len(a.Calls) == 0 {
		return 0
	}
	floor := a.Calls[0].Context() - int64(ApproxTokens(a.PromptChars))
	if floor < 0 {
		return 0
	}
	return floor
}

// Session is a main-thread transcript and the subagents spawned under it.
type Session struct {
	SID           string
	MainCalls     []Call
	Agents        []Agent
	MainToolCalls []ToolCall
}

// SubagentCalls answers every call made inside a subagent of this session.
func (s Session) SubagentCalls() []Call {
	out := make([]Call, 0, len(s.Agents))
	for _, agent := range s.Agents {
		out = append(out, agent.Calls...)
	}
	return out
}

// AllCalls answers the main thread's calls followed by every subagent's.
func (s Session) AllCalls() []Call {
	out := make([]Call, 0, len(s.MainCalls))
	out = append(out, s.MainCalls...)
	return append(out, s.SubagentCalls()...)
}

// Threads answers one tool-call list per CONTEXT WINDOW: the main thread, then
// each agent. A repeated read costs nothing across two windows, so the windows
// are what a repeat is counted within.
func (s Session) Threads() [][]ToolCall {
	out := make([][]ToolCall, 0, len(s.Agents)+1)
	out = append(out, s.MainToolCalls)
	for _, agent := range s.Agents {
		out = append(out, agent.ToolCalls)
	}
	return out
}

// AllToolCalls answers every tool call of the session, across every window.
func (s Session) AllToolCalls() []ToolCall {
	out := make([]ToolCall, 0, len(s.MainToolCalls))
	for _, thread := range s.Threads() {
		out = append(out, thread...)
	}
	return out
}

// SlugForPath answers the project slug that Claude Code derives from a working
// directory. It replaces each nonletter, nondigit, and nonhyphen with a hyphen.
//
// It counts CHARACTERS, not bytes. One accented letter becomes ONE hyphen in
// Python. A byte walk would produce two and name a nonexistent directory.
func SlugForPath(path string) string {
	var out strings.Builder
	out.Grow(len(path))
	for _, r := range path {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			out.WriteRune(r)
		default:
			out.WriteByte('-')
		}
	}
	return out.String()
}

// DefaultRoot answers the transcript store root, which is ~/.claude/projects.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// usageInt reads one usage field as untrusted input. It returns 0 for an
// unusable value. A boolean returns 0 instead of 1. A flag in a token field is a
// producer defect, and a one-token count would hide that defect.
func usageInt(value any) int64 {
	number, ok := value.(json.Number)
	if !ok {
		return 0
	}
	if whole, err := number.Int64(); err == nil {
		return whole
	}
	fraction, err := number.Float64()
	if err != nil {
		return 0
	}
	return int64(fraction)
}

// truthy answers what Python's `or` and `bool()` answer for a decoded JSON
// value. read_meta and the message-id fallback both rely on it.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		f, err := v.Float64()
		return err == nil && f != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}

// text answers a decoded JSON value when it is a string, and "" otherwise.
func text(value any) string {
	s, _ := value.(string)
	return s
}

// object answers a decoded JSON value when it is an object, and nil otherwise.
func object(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

// Scan is one transcript, read once: its API calls, their origin, and its tool
// use. Keyed by message id throughout, so the store-wide owner resolution in
// AssignOwners can drop an id and everything hanging off it together.
//
// Order is load-bearing in two cases. An agent's FIRST call sets its harness
// floor. The first tool NAME decides a tie in the result-size table.
//
// A Go map iterates at random. The keys stay beside the map in the order of
// their first record, which preserves the script's dict order.
type Scan struct {
	order     []string
	calls     map[string]Call
	origins   map[string]string
	toolOrder []string
	tools     map[string][]ToolCall
	// PromptChars is the character count of the FIRST user message: for a
	// subagent, the prompt its parent wrote. Held so the harness floor can take
	// it back out of the first call and leave what the harness supplied. Zero
	// when the transcript opens with no user text.
	PromptChars int
	// Unreadable is the error that stopped this transcript being opened.
	//
	// The script has no such field. It swallows the open error and yields
	// nothing. Thus, an unreadable transcript contributes no calls, and the
	// whole report becomes SMALLER without an explanation. A lower number looks
	// like a passing result. Naming the error is this port's fix.
	Unreadable error
}

// Keys answers the message ids of this transcript in first-seen order.
func (s *Scan) Keys() []string { return s.order }

// Call answers one API call by its message id.
func (s *Scan) Call(key string) Call { return s.calls[key] }

// Has reports whether this transcript recorded the given message id.
func (s *Scan) Has(key string) bool { _, ok := s.calls[key]; return ok }

// Origin answers the session a record names as the one that MADE the call.
func (s *Scan) Origin(key string) string { return s.origins[key] }

// ToolKeys answers the message ids that carried a tool call, in first-seen
// order.
func (s *Scan) ToolKeys() []string { return s.toolOrder }

// Tools answers the tool calls of one API call.
func (s *Scan) Tools(key string) []ToolCall { return s.tools[key] }

func newScan() *Scan {
	return &Scan{calls: map[string]Call{}, origins: map[string]string{}, tools: map[string][]ToolCall{}}
}

func (s *Scan) put(key string, call Call) {
	if _, seen := s.calls[key]; !seen {
		s.order = append(s.order, key)
	}
	s.calls[key] = call
}

// blocks answers the content blocks of a message record, or an empty list.
func blocks(message any) []map[string]any {
	msg := object(message)
	if msg == nil {
		return nil
	}
	list, ok := msg["content"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if block := object(item); block != nil {
			out = append(out, block)
		}
	}
	return out
}

// resultChars answers the characters of one tool_result, whose content is text
// or a block list. Characters rather than bytes: the script measures a Python
// string, so one emoji in a result is one character and not four.
func resultChars(block map[string]any) int {
	switch content := block["content"].(type) {
	case string:
		return utf8.RuneCountInString(content)
	case []any:
		total := 0
		for _, item := range content {
			inner := object(item)
			if inner == nil {
				continue
			}
			if s, ok := inner["text"].(string); ok {
				total += utf8.RuneCountInString(s)
			}
		}
		return total
	default:
		return 0
	}
}

// records reads one transcript and calls visit for each JSON object it holds.
//
// records skips a line that does not parse. Transcript lines are untrusted. A
// truncated final line from a live or killed session does not invalidate the
// rest of the report.
//
// A file that cannot be OPENED is a different fact, so records answers an error.
// The script swallows that error. It then treats an unreadable transcript as
// empty and silently removes its calls from every figure.
func records(path string, visit func(map[string]any)) error {
	handle, err := os.Open(path) //nolint:gosec // a read-only report over the store it was pointed at
	if err != nil {
		return err
	}
	defer handle.Close() //nolint:errcheck // read-only

	// Reader has no line ceiling. A transcript line contains a whole assistant
	// message and sometimes spans megabytes. Scanner would stop at its buffer
	// limit and report a truncated store as complete.
	//
	// The file size bounds each line. The harness writes this local file on this
	// machine. A peer does not control it through a socket.
	reader := bufio.NewReaderSize(handle, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			decoder := json.NewDecoder(strings.NewReader(trimmed))
			decoder.UseNumber()
			var value any
			if decoder.Decode(&value) == nil {
				if record := object(value); record != nil {
					visit(record)
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// ScanTranscript reads one transcript's API calls, owning session, and tool use.
//
// Several assistant records represent one call. Context fields repeat, so
// record counts would multiply context totals.
//
// OUTPUT instead GROWS across records for one id. Only the last has the final
// count. Across 1,172 message ids, growth was always monotonic. First-record
// values totaled 149k output tokens, versus the correct 1.22M. MAXIMUM therefore
// merges repeated context and monotonic output exactly, independent of order.
//
// Every measured usage record had a message id (0 missing of 3,613). Request ids
// always agreed. Record uuid is not a fallback because it differs per RECORD and
// would restore double counting.
//
// Records also split tool_use blocks and repeat each block. Their tool-use ids
// deduplicate them before call attachment. Later user records contain results
// with the same ids. The final pass joins results because each follows its call.
func ScanTranscript(path string) *Scan {
	found := newScan()

	// uses holds one tool-use id per API call, in first-seen order, and results
	// holds the size fed back for each of them.
	useOrder := map[string][]string{}
	uses := map[string]map[string][2]string{}
	results := map[string]int{}

	found.Unreadable = records(path, func(record map[string]any) {
		message := record["message"]
		if text(record["type"]) == "user" {
			scanUserRecord(found, message, results)
			return
		}
		if text(record["type"]) != "assistant" {
			return
		}
		msg := object(message)
		if msg == nil {
			return
		}
		usage := object(msg["usage"])
		if usage == nil {
			return
		}
		key := text(msg["id"])
		if !truthy(msg["id"]) {
			key = text(record["requestId"])
		}
		if key == "" {
			return
		}
		call := Call{
			Input:      usageInt(usage["input_tokens"]),
			CacheWrite: usageInt(usage["cache_creation_input_tokens"]),
			CacheRead:  usageInt(usage["cache_read_input_tokens"]),
			Output:     usageInt(usage["output_tokens"]),
		}
		if previous, seen := found.calls[key]; seen {
			call = merge(previous, call)
		}
		found.put(key, call)

		// The record names the session that MADE the call and, separately, the
		// file it now sits in. A resumed session rewrites the second and leaves
		// the first, which is what tells a copy from an original.
		origin := text(record["session_id"])
		if origin == "" {
			origin = text(record["sessionId"])
		}
		if _, seen := found.origins[key]; origin != "" && !seen {
			found.origins[key] = origin
		}

		for _, block := range blocks(message) {
			if text(block["type"]) != "tool_use" {
				continue
			}
			useID, okID := block["id"].(string)
			name, okName := block["name"].(string)
			if !okID || !okName {
				continue
			}
			target := text(object(block["input"])["file_path"])
			if uses[key] == nil {
				uses[key] = map[string][2]string{}
				found.toolOrder = append(found.toolOrder, key)
			}
			if _, seen := uses[key][useID]; !seen {
				useOrder[key] = append(useOrder[key], useID)
			}
			uses[key][useID] = [2]string{name, target}
		}
	})

	for _, key := range found.toolOrder {
		byUseID := uses[key]
		calls := make([]ToolCall, 0, len(byUseID))
		for _, useID := range useOrder[key] {
			pair := byUseID[useID]
			calls = append(calls, ToolCall{Name: pair[0], FilePath: pair[1], ResultChars: results[useID]})
		}
		found.tools[key] = calls
		call := found.calls[key]
		call.Tools = len(byUseID)
		found.calls[key] = call
	}
	return found
}

// scanUserRecord reads the spawn prompt and the tool results of one user
// record.
func scanUserRecord(found *Scan, message any, results map[string]int) {
	if len(found.order) == 0 && found.PromptChars == 0 {
		// Before the first assistant record, so this is the spawn prompt
		// rather than a tool result fed back mid-run.
		msg := object(message)
		switch content := msg["content"].(type) {
		case string:
			found.PromptChars = utf8.RuneCountInString(content)
		case []any:
			total := 0
			for _, item := range content {
				block := object(item)
				if block == nil || text(block["type"]) != "text" {
					continue
				}
				total += utf8.RuneCountInString(text(block["text"]))
			}
			found.PromptChars = total
		}
	}
	for _, block := range blocks(message) {
		if text(block["type"]) != "tool_result" {
			continue
		}
		useID, ok := block["tool_use_id"].(string)
		if !ok || useID == "" {
			continue
		}
		if chars := resultChars(block); chars > results[useID] {
			results[useID] = chars
		}
	}
}

// merge answers the field-wise maximum of two records of the SAME API call.
//
// Maximum replaces last-record-wins, so record order does not affect the merge.
// Only the output field differs between duplicate records. ScanTranscript
// carries the measurement behind both facts.
func merge(first, second Call) Call {
	return Call{
		Input:      max(first.Input, second.Input),
		CacheWrite: max(first.CacheWrite, second.CacheWrite),
		CacheRead:  max(first.CacheRead, second.CacheRead),
		Output:     max(first.Output, second.Output),
		Tools:      max(first.Tools, second.Tools),
	}
}

// Meta is the spawn metadata beside a subagent transcript.
type Meta struct {
	AgentType   string
	Description string
	IsFork      bool
}

// ReadMeta answers the agent type, the description and the fork flag of a
// spawn.
//
// A fork inherits its parent's whole conversation, so its transcript repeats
// the parent's records. AssignOwners uses the flag to return those calls to the
// parent.
//
// Unreadable metadata produces empty fields and not-a-fork. A missing metadata
// file is usual for a store from an older harness. Its calls still count.
func ReadMeta(path string) Meta {
	body, err := os.ReadFile(path) //nolint:gosec // a read-only report over the store it was pointed at
	if err != nil {
		return Meta{}
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return Meta{}
	}
	meta := object(value)
	if meta == nil {
		return Meta{}
	}
	fork := false
	if flag, ok := meta["isFork"].(bool); ok && flag {
		fork = true
	}
	if truthy(meta["parentAgentId"]) {
		fork = true
	}
	return Meta{AgentType: text(meta["agentType"]), Description: text(meta["description"]), IsFork: fork}
}

// PhaseOf classifies a spawn into a workflow phase from its description
// keywords. The search text also includes the agent type. An EMPTY description
// remains unclassified regardless of the type. A type-only phase would give the
// same label to every spawn of that agent type.
func PhaseOf(description, agentType string) string {
	var haystack strings.Builder
	haystack.WriteString(description)
	haystack.WriteByte(' ')
	haystack.WriteString(agentType)
	lowered := strings.ToLower(haystack.String())
	if strings.TrimSpace(description) == "" {
		return Unclassified
	}
	for _, rule := range PhaseRules {
		for _, word := range rule.Keywords {
			if strings.Contains(lowered, word) {
				return rule.Phase
			}
		}
	}
	return Unclassified
}

// Transcript is one transcript file, its identity in the store, and what it
// holds.
type Transcript struct {
	Path    string
	SID     string
	IsAgent bool
	Meta    Meta
	Found   *Scan
}

// Collect answers every transcript of a project store, read once, in a stable
// order.
func Collect(store string) []Transcript {
	// Glob fails only on a malformed PATTERN. Both patterns here are literals,
	// so an error is a Ze defect instead of a store error. A store with no
	// transcript answers an empty list. Run reports that list as an empty store.
	mains, _ := filepath.Glob(filepath.Join(store, "*.jsonl")) //nolint:errcheck // a literal pattern cannot be malformed
	sort.Strings(mains)
	agents, _ := filepath.Glob(filepath.Join(store, "*", "subagents", "agent-*.jsonl")) //nolint:errcheck // a literal pattern cannot be malformed
	sort.Strings(agents)

	out := make([]Transcript, 0, len(mains)+len(agents))
	for _, path := range mains {
		out = append(out, Transcript{Path: path, SID: stem(path), Found: ScanTranscript(path)})
	}

	for _, path := range agents {
		out = append(out, Transcript{
			Path:    path,
			SID:     filepath.Base(filepath.Dir(filepath.Dir(path))),
			IsAgent: true,
			Meta:    ReadMeta(metaPath(path)),
			Found:   ScanTranscript(path),
		})
	}
	return out
}

// stem answers a file name with its last extension removed.
func stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// metaPath answers the spawn metadata beside a subagent transcript.
func metaPath(path string) string {
	var name strings.Builder
	name.WriteString(stem(path))
	name.WriteString(".meta.json")
	return filepath.Join(filepath.Dir(path), name.String())
}

// AssignOwners answers one owning transcript per message id, for the whole
// store.
//
// A resumed session and a forked agent both COPY earlier records into their own
// transcript. Thus, a dedup scoped to one file counts those calls once for each
// file.
//
// A measurement on 2026-08-05 found 487 ids in two transcripts. Those copies
// held 208,167,121 context tokens and 487 API calls. The per-session table also
// put the parent's calls on the copy's row.
//
// Ownership uses the store records in this order:
//
//   - First, select the transcript whose session matches the session that the
//     record names as its caller. This rule settles a resumed main session. The
//     copy rewrites its file id but keeps the caller id, which points to the
//     original.
//
//   - If no session matches, a transcript that is not a fork beats a fork. A
//     fork inherits its parent's context, and its metadata records that fact.
//
//   - Otherwise, select the first transcript in collection order. No store fact
//     separates the candidates. This choice does not change store-wide totals.
func AssignOwners(transcripts []Transcript) []map[string]bool {
	owners := make([]map[string]bool, len(transcripts))
	for i := range owners {
		owners[i] = map[string]bool{}
	}

	order := []string{}
	candidates := map[string][]int{}
	for index, transcript := range transcripts {
		for _, key := range transcript.Found.Keys() {
			if _, seen := candidates[key]; !seen {
				order = append(order, key)
			}
			candidates[key] = append(candidates[key], index)
		}
	}

	for _, key := range order {
		indexes := candidates[key]
		if len(indexes) > 1 {
			matched := []int{}
			for _, i := range indexes {
				if transcripts[i].Found.Origin(key) == transcripts[i].SID {
					matched = append(matched, i)
				}
			}
			switch {
			case len(matched) == 1:
				indexes = matched
			default:
				pool := matched
				if len(pool) == 0 {
					pool = indexes
				}
				original := []int{}
				for _, i := range pool {
					if !transcripts[i].Meta.IsFork {
						original = append(original, i)
					}
				}
				indexes = original
				if len(indexes) == 0 {
					indexes = pool
				}
			}
		}
		owners[indexes[0]][key] = true
	}
	return owners
}

// FindSessions answers every session of a project store, each with its subagent
// transcripts.
//
// Every API call is attributed to exactly ONE transcript in the store.
// AssignOwners defines the rule and removes duplicate calls.
func FindSessions(store string) ([]Session, []string) {
	transcripts := Collect(store)
	owners := AssignOwners(transcripts)

	unreadable := []string{}
	byID := map[string]*Session{}
	ids := []string{}
	for index, transcript := range transcripts {
		found := transcript.Found
		if found.Unreadable != nil {
			unreadable = append(unreadable, transcript.Path)
		}
		owned := owners[index]

		calls := []Call{}
		for _, key := range found.Keys() {
			if owned[key] {
				calls = append(calls, found.Call(key))
			}
		}
		tools := []ToolCall{}
		for _, key := range found.ToolKeys() {
			if owned[key] {
				tools = append(tools, found.Tools(key)...)
			}
		}

		session := byID[transcript.SID]
		if session == nil {
			session = &Session{SID: transcript.SID}
			byID[transcript.SID] = session
			ids = append(ids, transcript.SID)
		}
		if transcript.IsAgent {
			session.Agents = append(session.Agents, Agent{
				Name:        stem(transcript.Path),
				AgentType:   transcript.Meta.AgentType,
				Description: transcript.Meta.Description,
				Calls:       calls,
				ToolCalls:   tools,
				IsFork:      transcript.Meta.IsFork,
				PromptChars: found.PromptChars,
			})
			continue
		}
		session.MainCalls = calls
		session.MainToolCalls = tools
	}

	sort.Strings(ids)
	out := make([]Session, 0, len(ids))
	for _, id := range ids {
		out = append(out, *byID[id])
	}
	return out, unreadable
}

// comma renders a whole number with thousands separators, which is what the
// script's format spec does.
func comma(value int64) string {
	digits := textbuf.StringInt(value)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var out strings.Builder
	out.WriteString(sign)
	lead := len(digits) % 3
	if lead == 0 {
		lead = 3
	}
	out.WriteString(digits[:lead])
	for i := lead; i < len(digits); i += 3 {
		out.WriteByte(',')
		out.WriteString(digits[i : i+3])
	}
	return out.String()
}
