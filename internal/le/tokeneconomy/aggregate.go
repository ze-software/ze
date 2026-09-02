// Design: ai/rules/context-economy.md -- the figures the report is argued from
// Overview: tokeneconomy.go -- the store read that produces the calls
// Detail: report.go -- the page these figures are rendered into
//
// aggregate.go turns deduped calls and tool uses into the numbers the report
// prints. Every function here is arithmetic over calls that ALREADY HAPPENED.
// Nothing predicts, and the one counterfactual says so on the line that prints
// it.

package tokeneconomy

import (
	"slices"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Totals aggregates one set of calls. Context is summed AND kept per call,
// because the sum says what was paid and the per-call figures say where.
type Totals struct {
	Calls       int     `json:"calls"`
	Input       int64   `json:"input"`
	CacheWrite  int64   `json:"cache-write"`
	CacheRead   int64   `json:"cache-read"`
	Output      int64   `json:"output"`
	Context     int64   `json:"context"`
	ContextMax  int64   `json:"context-max"`
	ContextMean float64 `json:"context-mean"`
}

// Aggregate answers the totals of one set of calls.
func Aggregate(calls []Call) Totals {
	out := Totals{Calls: len(calls)}
	for _, call := range calls {
		out.Input += call.Input
		out.CacheWrite += call.CacheWrite
		out.CacheRead += call.CacheRead
		out.Output += call.Output
		context := call.Context()
		out.Context += context
		if context > out.ContextMax {
			out.ContextMax = context
		}
	}
	if len(calls) > 0 {
		out.ContextMean = float64(out.Context) / float64(len(calls))
	}
	return out
}

// Bucket is one row of the context histogram.
type Bucket struct {
	Label   string  `json:"label"`
	Calls   int     `json:"calls"`
	Context int64   `json:"context"`
	Share   float64 `json:"share"`
}

// BucketLabels answers the histogram's row labels, derived from BucketEdges so
// a changed edge cannot leave a label naming the old one.
func BucketLabels() []string {
	labels := make([]string, 0, len(BucketEdges)+1)
	var low int64
	for _, edge := range BucketEdges {
		var tb textbuf.Buffer
		tb.Str(Short(float64(low)))
		tb.Byte('-')
		tb.Str(Short(float64(edge)))
		labels = append(labels, tb.String())
		low = edge
	}
	var last textbuf.Buffer
	last.Str(Short(float64(BucketEdges[len(BucketEdges)-1])))
	last.Byte('+')
	return append(labels, last.String())
}

// Histogram attributes context tokens to their call-size bucket. Each row shows
// the context SIZE at which tokens were paid, not the call count for that size.
func Histogram(calls []Call) []Bucket {
	labels := BucketLabels()
	counts := make([]int, len(labels))
	context := make([]int64, len(labels))
	for _, call := range calls {
		index := len(BucketEdges)
		for i, edge := range BucketEdges {
			if call.Context() <= edge {
				index = i
				break
			}
		}
		counts[index]++
		context[index] += call.Context()
	}

	var grand int64
	for _, value := range context {
		grand += value
	}

	out := make([]Bucket, 0, len(labels))
	for i, label := range labels {
		share := 0.0
		if grand != 0 {
			share = 100.0 * float64(context[i]) / float64(grand)
		}
		out = append(out, Bucket{Label: label, Calls: counts[i], Context: context[i], Share: share})
	}
	return out
}

// ApproxTokens turns a character count into tokens. An approximation, and
// labeled as one wherever it is printed.
func ApproxTokens(chars int) float64 { return approxTokensOf(float64(chars)) }

// approxTokensOf divides a figure that is already a mean. It takes the mean
// BEFORE division. Reversing the order changes the final float bit, and the
// report prints both figures.
func approxTokensOf(chars float64) float64 { return chars / CharsPerToken }

// ToolsPerCallRow says how many API calls carried a given number of tool calls.
//
// A one-tool round trip pays its full context for one result. This distribution
// supports the batching rule. The 0, 1, 2, 3, and 4+ rows contain every API call.
type ToolsPerCallRow struct {
	Label string  `json:"label"`
	Calls int     `json:"calls"`
	Share float64 `json:"share"`
}

// ToolCallDistribution answers one row per tool-call count.
func ToolCallDistribution(calls []Call) []ToolsPerCallRow {
	labels := []string{"0", "1", "2", "3", "4+"}
	counts := make([]int, len(labels))
	for _, call := range calls {
		counts[min(call.Tools, 4)]++
	}
	out := make([]ToolsPerCallRow, 0, len(labels))
	for i, label := range labels {
		share := 0.0
		if len(calls) != 0 {
			share = 100.0 * float64(counts[i]) / float64(len(calls))
		}
		out = append(out, ToolsPerCallRow{Label: label, Calls: counts[i], Share: share})
	}
	return out
}

// SingleTool counts the tool-carrying API calls and how many carried exactly
// one. Each of those paid its whole context for a single result.
type SingleTool struct {
	Using int     `json:"using"`
	Alone int     `json:"alone"`
	Share float64 `json:"share"`
}

// SingleToolShare answers that count.
func SingleToolShare(calls []Call) SingleTool {
	out := SingleTool{}
	for _, call := range calls {
		if call.Tools == 0 {
			continue
		}
		out.Using++
		if call.Tools == 1 {
			out.Alone++
		}
	}
	if out.Using != 0 {
		out.Share = 100.0 * float64(out.Alone) / float64(out.Using)
	}
	return out
}

// ResultRow is one tool's share of the results fed back.
//
// Size is the recorded result TEXT, so it is characters divided by
// CharsPerToken. A denied or unanswered call contributes a real zero.
type ResultRow struct {
	Name       string  `json:"name"`
	Results    int     `json:"results"`
	MeanTokens float64 `json:"mean-tokens"`
	Tokens     float64 `json:"tokens"`
}

// ResultSizes answers one row per tool name, biggest total first.
//
// Ties retain the FIRST-SEEN tool order, as the script's dict does. Thus, tools
// with no returned data print in stable order.
func ResultSizes(toolCalls []ToolCall) []ResultRow {
	order := []string{}
	byName := map[string][]int{}
	for _, tool := range toolCalls {
		if _, seen := byName[tool.Name]; !seen {
			order = append(order, tool.Name)
		}
		byName[tool.Name] = append(byName[tool.Name], tool.ResultChars)
	}

	rows := make([]ResultRow, 0, len(order))
	for _, name := range order {
		chars := byName[name]
		total := 0
		for _, value := range chars {
			total += value
		}
		rows = append(rows, ResultRow{
			Name:       name,
			Results:    len(chars),
			MeanTokens: approxTokensOf(float64(total) / float64(len(chars))),
			Tokens:     ApproxTokens(total),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Tokens > rows[j].Tokens })
	return rows
}

// ReadTool is the tool whose repeats are counted. Only calls that name a file
// have meaningful repeats, and this tool names files.
const ReadTool = "Read"

// Repeats counts file reads and reads of an already-seen file.
//
// A group is one CONTEXT WINDOW: a thread or full session. Repeat count is total
// reads minus distinct paths, independent of order. Calls without paths belong
// to neither count because they have no comparison key.
type Repeats struct {
	Reads   int `json:"reads"`
	Repeats int `json:"repeats"`
}

// RepeatReads answers that pair for one group.
func RepeatReads(group []ToolCall) Repeats {
	seen := map[string]bool{}
	out := Repeats{}
	for _, tool := range group {
		if tool.Name != ReadTool {
			continue
		}
		if tool.FilePath == "" {
			// A read that named no file is in neither figure, because there is
			// nothing to compare it against.
			continue
		}
		out.Reads++
		seen[tool.FilePath] = true
	}
	out.Repeats = out.Reads - len(seen)
	return out
}

// AgentTypeRow contains one agent type's startup cost and total.
//
// MedianFloor subtracts the spawn prompt from the first call. It therefore
// measures harness input, not parent text. Every later call receives that input
// again, so it multiplies by Calls.
//
// A fork has floor 0 because it inherits the parent's full context. None of its
// first call is a harness floor. Including inherited context here would
// misrepresent the agent type.
type AgentTypeRow struct {
	Name        string `json:"name"`
	Agents      int    `json:"agents"`
	Calls       int    `json:"calls"`
	MedianFloor int64  `json:"median-floor"`
	Context     int64  `json:"context"`
}

// AgentTypeStartup answers one row for each agent type, ordered by descending
// total context. The first type feeds the most context, regardless of floor.
func AgentTypeStartup(agents []Agent) []AgentTypeRow {
	order := []string{}
	byType := map[string][]Agent{}
	for _, agent := range agents {
		name := agent.AgentType
		if name == "" {
			name = "unknown"
		}
		if _, seen := byType[name]; !seen {
			order = append(order, name)
		}
		byType[name] = append(byType[name], agent)
	}

	rows := make([]AgentTypeRow, 0, len(order))
	for _, name := range order {
		members := byType[name]
		calls := []Call{}
		floors := []int64{}
		for _, agent := range members {
			calls = append(calls, agent.Calls...)
			if len(agent.Calls) > 0 && !agent.IsFork {
				floors = append(floors, agent.HarnessFloor())
			}
		}
		rows = append(rows, AgentTypeRow{
			Name:        name,
			Agents:      len(members),
			Calls:       len(calls),
			MedianFloor: median(floors),
			Context:     Aggregate(calls).Context,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Context > rows[j].Context })
	return rows
}

// median answers the middle of a sample, and the MIDPOINT of the two middles
// for an even count -- not the upper of the two. Truncated to a whole number of
// tokens, which is what the report prints.
func median(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	slices.Sort(sorted)
	half := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[half]
	}
	return int64((float64(sorted[half-1]) + float64(sorted[half])) / 2)
}

// Capped is the real context total, the same calls with context capped, and
// the ratio between them.
//
// This is arithmetic over calls that already happened, never a prediction: a
// session run under a smaller context would have made different calls.
type Capped struct {
	Cap    int     `json:"cap"`
	Real   int64   `json:"real"`
	Capped int64   `json:"capped"`
	Share  float64 `json:"share"`
}

// CappedCounterfactual answers that arithmetic.
func CappedCounterfactual(calls []Call, ceiling int) Capped {
	out := Capped{Cap: ceiling}
	for _, call := range calls {
		context := call.Context()
		out.Real += context
		if context > int64(ceiling) {
			context = int64(ceiling)
		}
		out.Capped += context
	}
	if out.Real != 0 {
		out.Share = 100.0 * float64(out.Capped) / float64(out.Real)
	}
	return out
}

// Short renders token counts with three digits. For example, 1,180,000 becomes
// 1.18M.
//
// It rounds before it selects the unit, so 999,999 becomes 1M instead of 1000k.
//
// strconv.FormatFloat uses significant digits through the 'g' verb.
// textbuf.Float uses a fixed decimal count. Significant digits align 1.18M and
// 99.9k in one column. Comparison against the script covered 628 values,
// including boundaries, with full agreement.
func Short(value float64) string {
	units := []struct {
		name string
		size float64
	}{{"B", 1e9}, {"M", 1e6}, {"k", 1e3}}

	for _, unit := range units {
		scaled := value / unit.size
		magnitude := scaled
		if magnitude < 0 {
			magnitude = -magnitude
		}
		if magnitude < 0.9995 {
			continue
		}
		var tb textbuf.Buffer
		if magnitude >= 100 {
			tb.Float(scaled, 0)
		} else {
			tb.Str(strconv.FormatFloat(scaled, 'g', 3, 64))
		}
		tb.Str(unit.name)
		return tb.String()
	}
	return strconv.FormatFloat(value, 'f', 0, 64)
}

// fixed renders a float with the given number of decimals, which is what the
// script's format spec does.
func fixed(value float64, decimals int) string {
	return strconv.FormatFloat(value, 'f', decimals, 64)
}
