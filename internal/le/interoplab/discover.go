// Design: docs/architecture/testing/interop.md -- scenario discovery and selector contract
// Related: lab.go -- discovered scenarios become lifecycle plans.
package interoplab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const defaultSessionTimeout = 90 * time.Second

// Checker is the protocol leaf's typed assertion entry point.
type Checker func(context.Context, *CheckContext) error

// ScenarioSource binds a directory to the Go checker that replaces check.py.
type ScenarioSource struct {
	Name      string  `json:"name"`
	Directory string  `json:"directory"`
	Checker   Checker `json:"-"`
}

// EnvironmentOptions names the selector and suffix variables for one lab family.
// Empty variable names disable only that optional lookup.
type EnvironmentOptions struct {
	SelectorVariable string
	SuffixVariable   string
	DefaultImage     string
	DefaultSuffix    string
	Lookup           func(string) (string, bool)
}

// Environment is the exact common environment shared by the four lab families.
type Environment struct {
	NoBuild        bool          `json:"no-build"`
	Verbose        bool          `json:"verbose"`
	SessionTimeout time.Duration `json:"session-timeout"`
	Selector       string        `json:"selector,omitempty"`
	Suffix         string        `json:"suffix"`
	Image          string        `json:"image,omitempty"`
}

// ReadEnvironment reads common lab settings. Invalid SESSION_TIMEOUT keeps the
// producer's 90-second default instead of inventing a different fallback.
func ReadEnvironment(options EnvironmentOptions) Environment {
	lookup := options.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	timeout := defaultSessionTimeout
	if value, ok := lookup("SESSION_TIMEOUT"); ok {
		seconds, err := strconv.Atoi(value)
		if err == nil {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	suffix := options.DefaultSuffix
	if suffix == "" {
		suffix = strconv.Itoa(os.Getpid())
	}
	if options.SuffixVariable != "" {
		if value, ok := lookup(options.SuffixVariable); ok {
			suffix = value
		}
	}
	image := options.DefaultImage
	if value, ok := lookup("FRR_IMAGE"); ok {
		image = value
	}
	selector := ""
	if options.SelectorVariable != "" {
		if value, ok := lookup(options.SelectorVariable); ok {
			selector = strings.TrimSpace(value)
		}
	}
	noBuild, _ := lookup("NO_BUILD")
	verbose, _ := lookup("VERBOSE")
	return Environment{
		NoBuild:        noBuild == "1",
		Verbose:        verbose == "1",
		SessionTimeout: timeout,
		Selector:       selector,
		Suffix:         suffix,
		Image:          image,
	}
}

// Discover returns directory names in lexical order. The selector is an exact
// name. A selected directory without a checker is an error, never a skipped test.
func Discover(root, selector string, checkers map[string]Checker) ([]ScenarioSource, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if selector != "" {
			if entry.Name() != selector {
				continue
			}
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		if selector == "" {
			return nil, errors.New("scenario directory contains no scenarios")
		}
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("no scenario matching '").Str(selector).Str("' found").String())
	}

	scenarios := make([]ScenarioSource, 0, len(names))
	for _, name := range names {
		checker, ok := checkers[name]
		if !ok {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("scenario ").Str(name).Str(" has no Go checker").String())
		}
		if checker == nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("scenario ").Str(name).Str(" has a nil Go checker").String())
		}
		scenarios = append(scenarios, ScenarioSource{
			Name:      name,
			Directory: filepath.Join(root, name),
			Checker:   checker,
		})
	}
	return scenarios, nil
}
