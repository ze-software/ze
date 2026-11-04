// Design: (none -- new tool, predates documentation)
// Overview: benchmark.go -- benchmark orchestration producing Result

package perf

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SessionSetup holds session establishment timing in milliseconds.
type SessionSetup struct {
	Sender   int `json:"sender"`
	Receiver int `json:"receiver"`
}

// Pass names which measurement one `le perf send` run made. It is the one
// declaration of the passes: the result JSON value, the comparison's Pass
// column and the perf runner's summary phrase and result file name all derive
// from it. BenchmarkConfig.SenderFirst selects the pass, and PassFor maps it.
type Pass uint8

const (
	// PassUnrecorded is a result written before results named their pass. It
	// is neither pass, and is never read as one.
	PassUnrecorded Pass = iota
	// PassLiveForwarding connects the receiver first, so the DUT forwards each
	// UPDATE as it arrives.
	PassLiveForwarding
	// PassRIBPropagation connects the sender first (-sender-first), so the DUT
	// holds every route before the receiver connects and sends it its table.
	PassRIBPropagation
)

// passUnrecordedName is what the comparison writes in the Pass column of a
// result that names no pass.
const passUnrecordedName = "not recorded"

// passNaming spells one pass on every surface that names it.
type passNaming struct {
	value  string // the JSON value of Result.Pass
	phrase string // how the perf runner's summary names the pass
	suffix string // what the perf runner's result file appends to the DUT name
}

// passNamings holds the naming of every recorded pass, indexed by Pass.
var passNamings = [...]passNaming{
	PassLiveForwarding: {value: "live-forwarding", phrase: "the measurement", suffix: ""},
	PassRIBPropagation: {value: "rib-propagation", phrase: "the propagation pass", suffix: "-propagation"},
}

// PassFor answers the pass a run makes with BenchmarkConfig.SenderFirst set
// to senderFirst.
func PassFor(senderFirst bool) Pass {
	if senderFirst {
		return PassRIBPropagation
	}
	return PassLiveForwarding
}

// naming answers the spelling of a recorded pass, or false for
// PassUnrecorded and for a value no constant names.
func (p Pass) naming() (passNaming, bool) {
	if p == PassUnrecorded || int(p) >= len(passNamings) {
		return passNaming{}, false
	}
	return passNamings[p], true
}

// SenderFirst answers whether the pass connects the sender first, the
// `le perf send -sender-first` flag.
func (p Pass) SenderFirst() bool {
	return p == PassRIBPropagation
}

// String answers the pass as the comparison's Pass column writes it: its JSON
// value, or "not recorded" for PassUnrecorded.
func (p Pass) String() string {
	if naming, ok := p.naming(); ok {
		return naming.value
	}
	if p == PassUnrecorded {
		return passUnrecordedName
	}
	return "unknown pass " + strconv.Itoa(int(p))
}

// Phrase answers how the perf runner's summary names the pass, as in "the
// propagation pass lost routes". A pass no constant names answers String.
func (p Pass) Phrase() string {
	if naming, ok := p.naming(); ok {
		return naming.phrase
	}
	return p.String()
}

// ResultFile answers the base name of the result file the perf runner writes
// for dut in this pass: "ze.json" for the live-forwarding pass,
// "ze-propagation.json" for the propagation pass. PassUnrecorded and a value
// no constant names have no file, so they are refused rather than given the
// live-forwarding name.
func (p Pass) ResultFile(dut string) (string, error) {
	naming, ok := p.naming()
	if !ok {
		return "", fmt.Errorf("pass %s has no result file: the runner measures only %s", p, acceptedPasses())
	}
	return dut + naming.suffix + ".json", nil
}

// MarshalText answers the JSON value of a recorded pass. Result.Pass is
// omitted when unrecorded, so a refusal here means a value no constant names.
func (p Pass) MarshalText() ([]byte, error) {
	naming, ok := p.naming()
	if !ok {
		return nil, fmt.Errorf("pass %s has no JSON value: the accepted values are %s", p, acceptedPasses())
	}
	return []byte(naming.value), nil
}

// UnmarshalText reads the JSON value of a pass, and refuses any value no pass
// declares, naming the value and the accepted ones.
func (p *Pass) UnmarshalText(text []byte) error {
	for i := range passNamings {
		if passNamings[i].value != "" && passNamings[i].value == string(text) {
			*p = Pass(i) //nolint:gosec // i indexes passNamings, whose length fits a Pass
			return nil
		}
	}
	return fmt.Errorf("unknown pass %q: the accepted values are %s", text, acceptedPasses())
}

// acceptedPasses answers the JSON value of every recorded pass, comma
// separated, for a refusal.
func acceptedPasses() string {
	values := make([]string, 0, len(passNamings))
	for i := range passNamings {
		if passNamings[i].value != "" {
			values = append(values, passNamings[i].value)
		}
	}
	return strings.Join(values, ", ")
}

// Result holds the outcome of a single performance test run.
// ConvergenceMs is raw data: a nonzero value carries its own presence, while a
// measured zero MUST be recorded with RecordConvergence. JSON decoding records
// that fact automatically. The zero Result has no convergence measurement.
type Result struct {
	DUTName             string       `json:"dut-name"`
	Pass                Pass         `json:"pass,omitempty"`
	DUTVersion          string       `json:"dut-version"`
	DUTAddr             string       `json:"dut-addr"`
	DUTPort             int          `json:"dut-port"`
	DUTASN              int          `json:"dut-asn"`
	Routes              int          `json:"routes"`
	Family              string       `json:"family"`
	ForceMP             bool         `json:"force-mp"`
	Seed                uint64       `json:"seed"`
	Timestamp           string       `json:"timestamp"`
	Repeat              int          `json:"repeat"`
	RepeatKept          int          `json:"repeat-kept"`
	WarmupRuns          int          `json:"warmup-runs"`
	IterDelayMs         int          `json:"iter-delay-ms"`
	SessionSetupMs      SessionSetup `json:"session-setup-ms"`
	FirstRouteMs        int          `json:"first-route-ms"`
	ConvergenceMs       int          `json:"convergence-ms"`
	ConvergenceStddevMs int          `json:"convergence-stddev-ms"`
	RoutesSent          int          `json:"routes-sent"`
	RoutesReceived      int          `json:"routes-received"`
	RoutesLost          int          `json:"routes-lost"`
	ThroughputAvg       int          `json:"throughput-avg"`
	ThroughputAvgStddev int          `json:"throughput-avg-stddev"`
	ThroughputPeak      int          `json:"throughput-peak"`
	LatencyP50Ms        int          `json:"latency-p50-ms"`
	LatencyP90Ms        int          `json:"latency-p90-ms"`
	LatencyP99Ms        int          `json:"latency-p99-ms"`
	LatencyP99StddevMs  int          `json:"latency-p99-stddev-ms"`
	LatencyMaxMs        int          `json:"latency-max-ms"`
	WithdrawalMs        int          `json:"withdrawal-ms"`
	WithdrawalStddevMs  int          `json:"withdrawal-stddev-ms"`

	convergenceZeroMeasured bool
}

// RecordConvergence records a measured duration, including a legitimate zero.
// Direct measurement producers MUST use it when setting convergence.
func (r *Result) RecordConvergence(milliseconds int) {
	r.ConvergenceMs = milliseconds
	r.convergenceZeroMeasured = milliseconds == 0
}

// HasConvergence distinguishes an explicitly measured zero from missing data.
// Existing nonzero raw values already carry evidence that the field was set.
func (r *Result) HasConvergence() bool {
	if r.ConvergenceMs != 0 {
		return true
	}
	return r.convergenceZeroMeasured
}

// UnmarshalJSON retains convergence presence without rejecting diagnostic loss.
// A failed decode leaves the previous result unchanged.
func (r *Result) UnmarshalJSON(data []byte) error {
	type resultJSON Result
	var decoded Result
	wire := struct {
		*resultJSON
		Convergence *int `json:"convergence-ms"`
	}{resultJSON: (*resultJSON)(&decoded)}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Convergence != nil {
		decoded.RecordConvergence(*wire.Convergence)
	}
	*r = decoded
	return nil
}

// MarshalJSON preserves missing convergence and diagnostic loss for every JSON
// value form, including non-addressable map values. A value receiver is required
// by encoding/json's method set; embedding the alias avoids a second Result copy.
//
//nolint:gocritic // hugeParam: JSON must also preserve presence for Result values and map entries.
func (r Result) MarshalJSON() ([]byte, error) {
	type resultJSON Result
	wire := struct {
		*resultJSON
		Convergence *int `json:"convergence-ms,omitempty"`
	}{resultJSON: (*resultJSON)(&r)}
	if r.HasConvergence() {
		wire.Convergence = &r.ConvergenceMs
	}
	return json.Marshal(&wire)
}

// Shortfall answers nil when every route the sender announced reached the
// receiver, and otherwise an error naming how many routes of how many arrived.
// A result that lost a route measured a DUT that did not do the work, so its
// timings describe the routes that arrived, or nothing: it is not a
// measurement of forwarding. It is the one judgement of a complete result:
// `le perf send` exits 1 on it, and `le perf run` fails the DUT on it.
func (r *Result) Shortfall() error {
	if r.RoutesSent <= 0 {
		return errors.New("the sender announced no route, so nothing was measured")
	}
	if r.RoutesReceived < r.RoutesSent {
		return fmt.Errorf("%d of %d routes reached the receiver", r.RoutesReceived, r.RoutesSent)
	}
	return nil
}

// ComparisonError refuses raw measurements that cannot be ranked. Diagnostic
// writers deliberately do not call it: lost routes must remain in the evidence.
func (r *Result) ComparisonError() error {
	if r.RoutesLost != 0 {
		return fmt.Errorf("DUT %q family %q force-mp %t pass %q timestamp %q lost %d routes: convergence cannot be compared; measure this DUT again with le perf send before comparing", r.DUTName, r.Family, r.ForceMP, r.Pass, r.Timestamp, r.RoutesLost)
	}
	if !r.HasConvergence() {
		return fmt.Errorf("DUT %q family %q force-mp %t pass %q timestamp %q has no convergence measurement: missing convergence is not zero; measure this DUT again with le perf send before comparing", r.DUTName, r.Family, r.ForceMP, r.Pass, r.Timestamp)
	}
	return nil
}

// ReadNDJSON reads newline-delimited JSON results from the reader.
// Blank lines are skipped. Returns an error on malformed JSON.
func ReadNDJSON(r io.Reader) ([]Result, error) {
	var results []Result

	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		var res Result
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		results = append(results, res)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading NDJSON: %w", err)
	}

	return results, nil
}

// WriteNDJSON writes a single Result as one JSON line to the writer.
//
//nolint:gocritic // hugeParam: Result is passed by value for API simplicity; not a hot path.
func WriteNDJSON(w io.Writer, r Result) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	data = append(data, '\n')

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing result: %w", err)
	}

	return nil
}
