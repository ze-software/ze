// Design: docs/functional-tests.md -- the native command keeps its closed option table
// Overview: run.go -- bounded orchestration and first-failure capture

package teststressrepro

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const area = "test stress-repro"

// runAction is the only verb this command accepts.
const runAction = "run"

const (
	defaultIterations = 80
	defaultMinutes    = 20.0
	defaultTimeout    = 120
)

// The keywords `run` takes, each the old long flag without its leading dashes.
const (
	suiteKeyword      = "suite"
	testKeyword       = "test"
	iterationsKeyword = "iterations"
	parallelKeyword   = "parallel"
	burnersKeyword    = "burners"
	minutesKeyword    = "minutes"
	timeoutKeyword    = "timeout"
	raceKeyword       = "race"
	anyFailureKeyword = "any-failure"
	tagsKeyword       = "tags"
)

// valueCount names the value slot of every keyword that takes a count.
const valueCount = "count"

// Options is the exact option surface of the former stress-repro.py command.
// Parallel and Burners use zero to request their CPU-derived defaults.
type Options struct {
	Suite      string  `json:"suite"`
	Test       string  `json:"test,omitempty"`
	Iterations int     `json:"iterations"`
	Parallel   int     `json:"parallel"`
	Burners    int     `json:"burners"`
	Minutes    float64 `json:"minutes"`
	Timeout    int     `json:"timeout"`
	Race       bool    `json:"race"`
	AnyFailure bool    `json:"any-failure"`
	Tags       string  `json:"tags,omitempty"`
}

// table is the whole command surface, declared once, so the listing, the
// usage, the help pages and the parse read one grammar (internal/le/le/action).
// line is the invocation being answered: the run records it in the job
// registry as the command it admitted (admitRun). The listing passes nil.
//
// The table's parser refuses a dash-leading word in every value slot, the
// trailing one included, so `run suite -help` is refused by name with 2 and
// the load generator never starts (2026-09-11, about 943% CPU for twenty
// minutes). The refusal is on the keyword's own word and stays there: the run
// shell-splits `suite` and `test` into the child's argv (harnessArgs in
// run.go, realProcessRunner.Invoke in process.go), and docs/functional-tests.md
// ships `suite "plugin --draft"`, so a refusal widened to the split words would
// refuse the documented recipe.
func table(line []string) leaction.Area {
	return leaction.New(area, leaction.Action{
		Verb: runAction,
		Why: "run one functional-test selection repeatedly under bounded CPU, GC and process pressure," +
			" and stop at the first failure it captures",
		Parameters: []leaction.Parameter{
			{Keyword: suiteKeyword, Kind: leaction.KindString, Value: "suite", Requirement: leaction.Required,
				Help: "the le test suite to run, split on whitespace into its words"},
			{Keyword: testKeyword, Kind: leaction.KindString, Value: "selector", Requirement: leaction.Optional,
				Help: "the test selector within the suite, split on whitespace into its words"},
			{Keyword: iterationsKeyword, Kind: leaction.KindUint, Value: valueCount, Requirement: leaction.Optional,
				Help: "how many invocations to run; 80 when not given"},
			{Keyword: parallelKeyword, Kind: leaction.KindUint, Value: valueCount, Requirement: leaction.Optional,
				Help: "how many invocations run at once; 0 or not given derives it from the CPU count"},
			{Keyword: burnersKeyword, Kind: leaction.KindUint, Value: valueCount, Requirement: leaction.Optional,
				Help: "how many CPU burners run beside them; 0 or not given derives it from the CPU count"},
			{Keyword: minutesKeyword, Kind: leaction.KindString, Value: valueCount, Requirement: leaction.Optional,
				Help: "the wall-clock budget in minutes, a finite non-negative number; 20 when not given"},
			{Keyword: timeoutKeyword, Kind: leaction.KindUint, Value: "seconds", Requirement: leaction.Optional,
				Help: "the deadline of one invocation in seconds, above zero; 120 when not given"},
			{Keyword: raceKeyword, Kind: leaction.KindSwitch,
				Help: "build the daemon with the race detector"},
			{Keyword: anyFailureKeyword, Kind: leaction.KindSwitch,
				Help: "stop at any failing invocation, not only one carrying a crash signature"},
			{Keyword: tagsKeyword, Kind: leaction.KindString, Value: "tags", Requirement: leaction.Optional,
				Help: "the build tags the daemon is built with"},
		},
		AnswerArgs: func(parsed leaction.Arguments) (any, int) { return answerRun(line, parsed) },
	})
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the tests that check them all read one table.
func Actions() leaction.List { return table(nil).Actions() }

// Subs is the argument-aware hint rendered beneath the command, derived from
// the table so it cannot disagree with the listing about what `run` takes.
func Subs() string {
	grammar, held := Actions().Grammar(runAction)
	if !held {
		panic("BUG: the stress-repro table does not hold " + runAction)
	}
	return grammar
}

// Answer dispatches the action-before-identifier command grammar.
func Answer(args []string) (any, int) { return table(args).Answer(args) }

// answerRun runs one parsed `run` line. line is the whole invocation, verb
// included, which the run records as the command it admitted.
func answerRun(line []string, parsed leaction.Arguments) (any, int) {
	opts, err := optionsOf(parsed)
	if err != nil {
		// 2, what the old parser answered for every refusal, so a caller
		// reads a refused line apart from a run that found a failure.
		return nil, table(line).Refuse(runAction, err)
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return runAt(root, line, opts)
}

// parseOptions reads the words after `run` through the table and turns them
// into the run's options. It runs nothing.
func parseOptions(args []string) (Options, error) {
	parsed, err := table(nil).Parse(runAction, args)
	if err != nil {
		return Options{}, err
	}
	return optionsOf(parsed)
}

// optionsOf turns the parsed keywords into the run's options and refuses a
// value outside its bounds. The parser does not yet refuse a missing required
// keyword (D-P2-required in spec-le-cli-usability), so this does.
func optionsOf(args leaction.Arguments) (Options, error) {
	opts := Options{
		Suite: args.One(suiteKeyword), Test: args.One(testKeyword),
		Iterations: defaultIterations, Minutes: defaultMinutes, Timeout: defaultTimeout,
		Race: args.Has(raceKeyword), AnyFailure: args.Has(anyFailureKeyword), Tags: args.One(tagsKeyword),
	}
	seenSuite := args.Has(suiteKeyword)
	counts := []struct {
		keyword string
		field   *int
	}{
		{iterationsKeyword, &opts.Iterations}, {parallelKeyword, &opts.Parallel},
		{burnersKeyword, &opts.Burners}, {timeoutKeyword, &opts.Timeout},
	}
	for _, count := range counts {
		if !args.Has(count.keyword) {
			continue
		}
		parsed, err := parseInt(count.keyword, args.One(count.keyword))
		if err != nil {
			return Options{}, err
		}
		*count.field = parsed
	}
	if args.Has(minutesKeyword) {
		value := args.One(minutesKeyword)
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return Options{}, fmt.Errorf("minutes %q is not a number", value)
		}
		opts.Minutes = parsed
	}
	if !seenSuite {
		return Options{}, errors.New("run requires suite <suite>")
	}
	if opts.Iterations < 0 {
		return Options{}, errors.New("iterations must be zero or greater")
	}
	if opts.Parallel < 0 {
		return Options{}, errors.New("parallel must be zero or greater")
	}
	if opts.Burners < 0 {
		return Options{}, errors.New("burners must be zero or greater")
	}
	maxMinutes := float64(^uint64(0)>>1) / float64(time.Minute)
	if math.IsNaN(opts.Minutes) || math.IsInf(opts.Minutes, 0) ||
		opts.Minutes < 0 || opts.Minutes > maxMinutes {
		return Options{}, errors.New("minutes must be a finite non-negative duration")
	}
	maxTimeout := int64(^uint64(0)>>1) / int64(time.Second)
	if opts.Timeout <= 0 || int64(opts.Timeout) > maxTimeout {
		return Options{}, errors.New("timeout must be a positive duration that fits the process deadline")
	}
	return opts, nil
}

func parseInt(key, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not an integer", key, value)
	}
	return parsed, nil
}
