// Design: docs/functional-tests.md -- the native command keeps its closed option table
// Overview: run.go -- bounded orchestration and first-failure capture

package stressrepro

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "stress-repro"

// runAction is the only verb this command accepts.
const runAction = "run"

const (
	defaultIterations = 80
	defaultMinutes    = 20.0
	defaultTimeout    = 120
)

// Action documents one accepted action and its complete keyword grammar.
type Action struct {
	Action string `json:"action"`
	Usage  string `json:"usage"`
}

// ActionList is the structured command inventory returned by a bare invocation.
type ActionList struct {
	Actions []Action `json:"actions"`
}

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

// Actions answers the only action and pins every old long flag to the
// same-named keyword, without its leading dashes.
func Actions() ActionList {
	return ActionList{Actions: []Action{{
		Action: runAction,
		Usage:  "run suite <suite> [test <selector>] [iterations <count>] [parallel <count>] [burners <count>] [minutes <count>] [timeout <seconds>] [race] [any-failure] [tags <tags>]",
	}}}
}

// Subs is the argument-aware hint rendered beneath the command.
func Subs() string { return Actions().Actions[0].Usage }

// Answer dispatches the action-before-identifier command grammar.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions(), 0
	}
	if args[0] != runAction {
		refuse(args[0])
		return nil, 2
	}
	opts, err := parseOptions(args[1:])
	if err != nil {
		refuse(err.Error())
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return runAt(root, opts)
}

func parseOptions(args []string) (Options, error) {
	opts := Options{Iterations: defaultIterations, Minutes: defaultMinutes, Timeout: defaultTimeout}
	seenSuite := false
	for len(args) != 0 {
		key := args[0]
		args = args[1:]
		switch key {
		case "race":
			opts.Race = true
		case "any-failure":
			opts.AnyFailure = true
		case "suite", "test", "iterations", "parallel", "burners", "minutes", "timeout", "tags":
			if len(args) == 0 {
				return Options{}, fmt.Errorf("%s requires a value", key)
			}
			value := args[0]
			args = args[1:]
			switch key {
			case "suite":
				if seenSuite {
					return Options{}, errors.New("suite may be specified only once")
				}
				seenSuite = true
				opts.Suite = value
			case "test":
				opts.Test = value
			case "iterations":
				parsed, err := parseInt(key, value)
				if err != nil {
					return Options{}, err
				}
				opts.Iterations = parsed
			case "parallel":
				parsed, err := parseInt(key, value)
				if err != nil {
					return Options{}, err
				}
				opts.Parallel = parsed
			case "burners":
				parsed, err := parseInt(key, value)
				if err != nil {
					return Options{}, err
				}
				opts.Burners = parsed
			case "minutes":
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return Options{}, fmt.Errorf("minutes %q is not a number", value)
				}
				opts.Minutes = parsed
			case "timeout":
				parsed, err := parseInt(key, value)
				if err != nil {
					return Options{}, err
				}
				opts.Timeout = parsed
			case "tags":
				opts.Tags = value
			}
		default:
			return Options{}, fmt.Errorf("unknown keyword %q", key)
		}
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

func refuse(message string) {
	leaction.ReportError(errors.New(area + ": " + message))
}
