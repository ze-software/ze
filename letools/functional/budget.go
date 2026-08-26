// Design: docs/architecture/testing/ci-format.md -- the wall-clock budget a suite runs under
//
// The budget defines the behavior here, not the dispatch.
// Each suite runs as `timeout --kill-after=<K> <T>` in its own process group.
// A stuck subprocess can keep an output pipe open after SIGKILL.
// Only `timeout` signals the whole group, which prevents the runner from waiting indefinitely.
//
// A kill is reported as a kill. `timeout` answers 124 for this event.
// Previously, the run treated 124 as a budget expiry and printed the failures emitted before the kill.
// The report did not name the kill. The plugin suite showed this fault on 2026-08-18 at 599.7s.
//
// A suite that is CREEPING toward its cap warns while it is still green. Raising
// a cap is not a fix on its own (ai/rules/completion.md), so the number has to
// stay visible as it climbs.
//
// A slow suite gets its own budget instead of raising the shared budget for the other 23 suites.
// The `plugin` suite has 663 .ci files and a 1500s budget.
// Its measured runtime was 855s on 2026-08-19 (spec verify-scope-4, A-1).
//
// Five other sessions raised the load average from 6.6 to 18.7 across 32 cores.
// The warning point sits 40% above that measurement to avoid a warning on every contended run.
// The calculation is 855 * 1.40 / 0.80 = 1496s, rounded to a whole minute.
// The kill occurs at 1.75 times the measurement, which identifies a stuck run instead of a busy host.
//
// A report and `timeout` that use different budgets are worse than no budget.
// A mismatched report says 1500s while the kill occurs at 600s.
// Suite.Budget supplies the `timeout` argument, runtime line, and warning calculation.
//
// `encode` and `plugin` run through the BGP runner.
// Their -p value was previously fixed at 8 on every host.
// GitHub's 4-vCPU hosted runner supports 8, but a 32-core workstation supports more.
// The derived value uses the current minimum as its floor and the core count as its cap.
// The floor preserves identical CI behavior and is runner.SuiteConcurrencyFloor.
//
// Measurements on a 32-core host set the CAP.
// The plugin suite has 96% parallel efficiency at 8 and 88% at 16.
// Efficiency falls to 74% at 32 and 36% at 64.
// At concurrency 64, runtime falls inside the two-run spread measured at 32.
//
// These figures do not apply to the other 22 suites.
// Those suites keep DefaultSuiteConcurrency's 2x CPUs because the measurement did not cover them.

package functional

import (
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
)

// These constants name the types of the environment registry entries.
// This package uses each value nine times, which goconst reports as repeated literals.
const (
	envString = "string"
	envInt    = "int"
	envBool   = "bool"
)

// DefaultBudget is the shared per-suite wall-clock cap, and DefaultKillAfter is
// the grace `timeout` allows between its TERM and its KILL.
const (
	DefaultBudget    = "600s"
	DefaultKillAfter = "10s"
)

// DefaultWarnPercent is when a suite warns about budget use while it remains green.
const DefaultWarnPercent = 80

// ParallelFloor is the minimum default concurrency for the two measured suites.
// It equals runner.SuiteConcurrencyFloor, which ZE_PLUGIN_PARALLEL used on GitHub's 4-vCPU runner.
// The floor therefore preserves CI behavior on a small host.
const ParallelFloor = runner.SuiteConcurrencyFloor

// sharedBudgetVar is the environment variable that owns every suite's cap
// unless the suite has one of its own.
const sharedBudgetVar = "ZE_SUITE_TIMEOUT"

// budgetDefaults names suites that need more time than the shared budget.
// Each name uses its own variable for the timeout, runtime, warning, and suggested adjustment.
// Lower the plugin value after the suite becomes faster or is split.
// First, read the derivation in this file's header.
var budgetDefaults = map[string]string{suitePlugin: "1500s"}

var (
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.suite.timeout",
		Type:        envString,
		Default:     DefaultBudget,
		Description: "the shared wall-clock cap every functional suite runs under",
		Private:     true,
	})
	// The per-suite override. A prefix entry, because the suite table is the
	// record of which suites exist and a second list here would drift from it.
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.suite.timeout.<suite>",
		Type:        envString,
		Default:     "",
		Description: "one suite's own wall-clock cap, which then owns it everywhere",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.suite.kill.after",
		Type:        envString,
		Default:     DefaultKillAfter,
		Description: "the grace timeout allows a suite between its TERM and its KILL",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.suite.warn.percent",
		Type:        envInt,
		Default:     strconv.Itoa(DefaultWarnPercent),
		Description: "the share of its budget a suite may use before a green run warns about it",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.suite.cores",
		Type:        envInt,
		Default:     "",
		Description: "the core count the suite concurrency derivation caps at; empty means the floor",
		Private:     true,
	})
	// The per-suite concurrency override, one entry per suite that takes the
	// derived concurrency. Derived from the suite table rather than listed, so a
	// new scaled suite needs no second edit here.
	_ = registerScaledParallelKeys()
)

// registerScaledParallelKeys declares ZE_<SUITE>_PARALLEL for every scaled
// suite and answers the entries it declared.
func registerScaledParallelKeys() []env.EnvEntry {
	keys := make([]env.EnvEntry, 0, len(Suites))
	for _, suite := range Suites {
		if !suite.Scaled {
			continue
		}
		entry := env.MustRegister(env.EnvEntry{
			Key:         parallelKey(suite.Name),
			Type:        envInt,
			Default:     "",
			Description: "this suite's own -p, which beats the derivation and moves no other suite",
			Private:     true,
		})
		keys = append(keys, entry)
	}
	return keys
}

// envKey turns an environment variable's OS spelling into the dot-notation key
// internal/core/env resolves. env.Get normalizes dots to underscores and
// lowercases, so the two spellings name one variable.
func envKey(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", "."))
}

// budgetVarName is the OS spelling of one suite's own budget variable. It is
// what the reports print, because telling a reader to raise ZE_SUITE_TIMEOUT
// when the kill came from ZE_SUITE_TIMEOUT_PLUGIN sends them to raise the cap
// for all 24 suites.
func budgetVarName(suite string) string {
	var tb textbuf.Buffer
	return tb.Str(sharedBudgetVar).Byte('_').
		Str(strings.ToUpper(strings.ReplaceAll(suite, "-", "_"))).String()
}

// parallelKey is the dot-notation key of one suite's own -p override.
func parallelKey(suite string) string {
	var tb textbuf.Buffer
	return tb.Str("ze.").Str(strings.ReplaceAll(suite, "-", ".")).Str(".parallel").String()
}

// BudgetVar answers the environment variable that OWNS this suite's budget.
func (s Suite) BudgetVar() string {
	own := budgetVarName(s.Name)
	if _, declared := budgetDefaults[s.Name]; declared || env.Get(envKey(own)) != "" {
		return own
	}
	return sharedBudgetVar
}

// Budget answers this suite's wall-clock cap, as `timeout` spells a duration.
func (s Suite) Budget() string {
	variable := s.BudgetVar()
	if set := env.Get(envKey(variable)); set != "" {
		return set
	}
	if own, declared := budgetDefaults[s.Name]; declared {
		return own
	}
	return DefaultBudget
}

// KillAfter answers the grace `timeout` allows between its TERM and its KILL.
func KillAfter() string {
	if set := strings.TrimSpace(env.Get("ze.suite.kill.after")); set != "" {
		return set
	}
	return DefaultKillAfter
}

// WarnPercent answers the share of its budget at which a green suite is warned
// about. A value that is not a whole number is the default rather than zero:
// zero would warn about every suite that ran at all.
func WarnPercent() int {
	raw := strings.TrimSpace(env.Get("ze.suite.warn.percent"))
	if !isDigits(raw) {
		return DefaultWarnPercent
	}
	found, err := strconv.Atoi(raw)
	if err != nil {
		return DefaultWarnPercent
	}
	return found
}

// durationSuffixes are the units `timeout` accepts after a number.
const durationSuffixes = "smhd"

// scale is what each `timeout` duration suffix multiplies by.
var scale = map[string]int{"": 1, "s": 1, "m": 60, "h": 3600, "d": 86400}

// DurationSeconds answers a `timeout` duration in seconds, or 0 when it is not
// one this can measure.
//
// Zero is the answer for `0s`, for a non-number, and for a fractional duration
// `timeout` accepts and integer arithmetic cannot divide by. The caller prints
// the runtime anyway and skips the percentage, because a budget nobody can
// measure against is not a reason to lose the measurement.
func DurationSeconds(text string) int {
	suffix := ""
	if text != "" && strings.ContainsAny(text[len(text)-1:], durationSuffixes) {
		suffix = text[len(text)-1:]
	}
	number := strings.TrimSuffix(text, suffix)
	if !isDigits(number) {
		return 0
	}
	found, err := strconv.Atoi(number)
	if err != nil {
		return 0
	}
	return found * scale[suffix]
}

// isDigits reports whether every byte of s is a decimal digit, and s is not
// empty. It is str.isdigit(), which is what the script tested with.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// declared reports whether name is present in this process's environment, with
// any value including an empty one.
//
// env.Get maps an absent variable and a present empty variable to the same value.
// ZE_SUITE_CORES needs that distinction.
// A container can set the variable to empty when its core count is unknown.
// Treating empty as absent would use the machine's core count instead of the floor.
func declared(name string) bool {
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte('=').String()
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

// cores answers the core count the concurrency derivation caps at.
//
// ZE_SUITE_CORES lets a test represent a small host on any machine.
// A present nonnumeric value uses the floor instead of an empty -p.
func cores() int {
	if declared("ZE_SUITE_CORES") {
		raw := strings.TrimSpace(env.Get("ze.suite.cores"))
		if !isDigits(raw) {
			return ParallelFloor
		}
		found, err := strconv.Atoi(raw)
		if err != nil || found == 0 {
			return ParallelFloor
		}
		return found
	}
	// runtime.NumCPU honors the affinity mask, which is what bare `nproc`
	// reports and what the recipe measured on any host that has it.
	if found := runtime.NumCPU(); found > 0 {
		return found
	}
	return ParallelFloor
}

// Parallel answers the -p one scaled suite runs at: floor at the runner's own
// floor, cap at the core count.
//
// ZE_<SUITE>_PARALLEL wins, because an operator's own value must beat a
// derivation, and overriding one suite must not move the other.
func Parallel(suite string) string {
	if own := strings.TrimSpace(env.Get(parallelKey(suite))); own != "" {
		return own
	}
	return strconv.Itoa(max(cores(), ParallelFloor))
}
