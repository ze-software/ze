// Design: plan/learned/891-granular-debug.md -- granular debug with named profiles
// Related: profile.go -- profile storage, show.go -- structured display, register.go -- CLI registration
//
// Grammar is verb-first (set/delete/show/clear), matching VyOS syslog-level
// configuration (docs.vyos.io): debug is persistent state edited in debug.zefs,
// not an operational toggle. The daemon `show debug` command (live runtime
// state) is a separate command in yang/ze-debug-cmd.yang.

package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	debugyang "github.com/ze-software/ze/internal/component/debug/yang"
	"github.com/ze-software/ze/internal/core/duration"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.debug.store", Type: "string", Description: "Override debug profile store path (default: <config-dir>/debug.zefs)"})

const (
	defaultDebugStore = "debug.zefs"
	defaultProfile    = "default"
)

var debugStoreOverride string

func stdoutLine(msg string) {
	os.Stdout.WriteString(msg)  //nolint:errcheck // CLI output
	os.Stdout.WriteString("\n") //nolint:errcheck // CLI output
}

func stderrLine(msg string) {
	os.Stderr.WriteString(msg)  //nolint:errcheck // CLI error output
	os.Stderr.WriteString("\n") //nolint:errcheck // CLI error output
}

func storeError(err error) {
	var tb textbuf.Buffer
	stderrLine(tb.Str("error: ").Err(err).String())
	stderrLine("check debug.zefs file permissions and available disk space")
}

// saveAndApply persists the profile to the default slot and applies it live.
func saveAndApply(storePath string, p *Profile) int {
	if err := SaveProfile(storePath, defaultProfile, p); err != nil {
		storeError(err)
		return 2
	}
	applyProfile(p)
	return 0
}

// runSetModule handles `set debug module <name> [level <l> | flag <f> | scope <k> <v>]`.
// Bare `set debug module <name>` enables debug for the subsystem at the default
// level; the optional keyword adds/sets a level, flag, or scope.
func runSetModule(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: set debug module <name> [level <level> | flag <flag> | scope <kind> <value>]")
		return 1
	}
	module := args[0]
	if module == "" || strings.ContainsAny(module, "/\x00") {
		return printInvalidSubsystem(module)
	}

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	if !p.HasModule(module) {
		p.ToggleModule(module)
	}
	if len(args) > 1 {
		if code := setModuleSetting(p, module, args[1:]); code != 0 {
			return code
		}
	}
	if code := saveAndApply(storePath, p); code != 0 {
		return code
	}
	var tb textbuf.Buffer
	stdoutLine(tb.Str("debug module ").Str(module).Str(" enabled").String())
	return 0
}

// setModuleSetting applies a level/flag/scope sub-option for `set`. The keyword
// follows a variable module selector, so this is argument parsing, not command
// dispatch.
func setModuleSetting(p *Profile, module string, args []string) int {
	keyword := args[0]
	if keyword == "level" {
		if len(args) < 2 {
			stderrLine("usage: set debug module <name> level <level>")
			return 1
		}
		level := args[1]
		if !slogutil.ValidateLevel(level) {
			var tb textbuf.Buffer
			stderrLine(tb.Str("error: invalid level ").Quoted(level).Str(" (valid: debug, info, warn, error)").String())
			return 1
		}
		p.SetLevel(module, level)
		return 0
	}
	if keyword == "flag" {
		if len(args) < 2 {
			stderrLine("usage: set debug module <name> flag <flag>")
			return 1
		}
		flag := args[1]
		if debugyang.HasModule(module) && !debugyang.ValidateFlag(module, flag) {
			var tb textbuf.Buffer
			stderrLine(tb.Str("error: unknown flag ").Quoted(flag).Str(" for ").Str(module).String())
			if valid := debugyang.FlagsFor(module); len(valid) > 0 {
				tb.Reset()
				stderrLine(tb.Str("valid flags: ").Join(valid, ", ").String())
			}
			return 1
		}
		if !p.HasFlag(module, flag) {
			p.ToggleFlag(module, flag)
		}
		return 0
	}
	if keyword == "scope" {
		if len(args) < 3 {
			stderrLine("usage: set debug module <name> scope <kind> <value>")
			return 1
		}
		kind := args[1]
		if debugyang.HasModule(module) && !debugyang.ValidateScope(module, kind) {
			var tb textbuf.Buffer
			stderrLine(tb.Str("error: unknown scope ").Quoted(kind).Str(" for ").Str(module).String())
			return 1
		}
		if !p.HasScope(module, kind, args[2]) {
			p.ToggleScope(module, kind, args[2])
		}
		return 0
	}
	var tb textbuf.Buffer
	stderrLine(tb.Str("unknown debug option: ").Str(keyword).String())
	return 1
}

// runDeleteModule handles `delete debug module <name> [flag <f> | scope <k> <v>]`.
// Bare `delete debug module <name>` disables debug for the subsystem entirely;
// the optional keyword removes just one flag or scope.
func runDeleteModule(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: delete debug module <name> [flag <flag> | scope <kind> <value>]")
		return 1
	}
	module := args[0]

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	if !p.HasModule(module) {
		// Idempotent: deleting an already-absent module is a no-op success.
		var tb textbuf.Buffer
		stdoutLine(tb.Str("debug module ").Str(module).Str(" disabled").String())
		return saveAndApply(storePath, p)
	}
	if len(args) == 1 {
		p.ToggleModule(module) // present -> removes the whole module
		if code := saveAndApply(storePath, p); code != 0 {
			return code
		}
		var tb textbuf.Buffer
		stdoutLine(tb.Str("debug module ").Str(module).Str(" disabled").String())
		return 0
	}
	if code := deleteModuleSetting(p, module, args[1:]); code != 0 {
		return code
	}
	if code := saveAndApply(storePath, p); code != 0 {
		return code
	}
	var tb textbuf.Buffer
	stdoutLine(tb.Str("debug module ").Str(module).Str(" updated").String())
	return 0
}

// deleteModuleSetting removes a flag/scope sub-option for `delete`. Keyword
// after a variable selector: argument parsing, not command dispatch.
func deleteModuleSetting(p *Profile, module string, args []string) int {
	keyword := args[0]
	if keyword == "flag" {
		if len(args) < 2 {
			stderrLine("usage: delete debug module <name> flag <flag>")
			return 1
		}
		if p.HasFlag(module, args[1]) {
			p.ToggleFlag(module, args[1])
		}
		return 0
	}
	if keyword == "scope" {
		if len(args) < 3 {
			stderrLine("usage: delete debug module <name> scope <kind> <value>")
			return 1
		}
		if p.HasScope(module, args[1], args[2]) {
			p.ToggleScope(module, args[1], args[2])
		}
		return 0
	}
	var tb textbuf.Buffer
	stderrLine(tb.Str("unknown debug option: ").Str(keyword).String())
	return 1
}

// runSetTimeout handles `set debug timeout <duration>`.
func runSetTimeout(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: set debug timeout <duration>  (e.g. 30m, 1h, 90s, 0; seconds rounded up to minutes)")
		return 1
	}

	minutes, ok := duration.ParseMinutes(args[0])
	if !ok {
		stderrLine("error: invalid duration (use e.g. 30m, 1h, 90s, or 0 to disable; seconds rounded up to minutes)")
		return 1
	}
	if minutes > 1440 {
		stderrLine("error: timeout must be at most 24h (1440m)")
		return 1
	}

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	p.Timeout = minutes
	if err := SaveProfile(storePath, defaultProfile, p); err != nil {
		storeError(err)
		return 2
	}

	if minutes == 0 {
		stdoutLine("debug timeout disabled")
	} else {
		var tb textbuf.Buffer
		stdoutLine(tb.Str("debug timeout set to ").Int(int64(minutes)).Str("m").String())
	}
	return 0
}

// runShowProfile handles `show debug profile` (list) and
// `show debug profile name <name>` (inspect one). This is the stored view; the
// live runtime view is the separate daemon `show debug` command.
func runShowProfile(args []string) int {
	if len(args) == 0 {
		return cmdProfileList()
	}
	if args[0] == "name" {
		if len(args) < 2 {
			stderrLine("usage: show debug profile name <name> [module <module>]")
			return 1
		}
		// Optional `module <prefix>` filters the table to one subsystem subtree,
		// preserving the historical `debug show <module>` view. Reject any other
		// trailing tokens rather than silently ignoring them.
		subtree := ""
		if rest := args[2:]; len(rest) > 0 {
			if rest[0] != "module" || len(rest) != 2 {
				stderrLine("usage: show debug profile name <name> [module <module>]")
				return 1
			}
			subtree = rest[1]
		}
		return cmdShowSaved(args[1], subtree)
	}
	var tb textbuf.Buffer
	stderrLine(tb.Str("unknown option: ").Str(args[0]).String())
	return 1
}

func cmdShowSaved(name, subtree string) int {
	storePath := debugStorePath()
	// The default profile always conceptually exists (empty until first set),
	// matching the historical `debug show` view. A named profile that is absent
	// is a user error, so it reports instead of silently showing an empty table.
	if name == defaultProfile {
		entries := showEntries(loadOrNewProfile(storePath), subtree)
		printShowTable(entries)
		return 0
	}
	p, err := LoadProfile(storePath, name)
	if err != nil {
		storeError(err)
		return 2
	}
	entries := showEntries(p, subtree)
	printShowTable(entries)
	return 0
}

// runSaveProfile handles `set debug profile name <name>`: save the current
// default state as a named profile.
func runSaveProfile(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: set debug profile name <name>")
		return 1
	}
	name := args[0]
	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	if err := SaveProfile(storePath, name, p); err != nil {
		storeError(err)
		return 2
	}
	var tb textbuf.Buffer
	stdoutLine(tb.Str("debug profile ").Quoted(name).Str(" saved").String())
	return 0
}

// runRestoreProfile handles `set debug active name <name>`: load a named
// profile and apply it live. Applies without overwriting the default slot,
// preserving the historical restore semantics.
func runRestoreProfile(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: set debug active name <name>")
		return 1
	}
	name := args[0]
	storePath := debugStorePath()
	p, err := LoadProfile(storePath, name)
	if err != nil {
		var tb textbuf.Buffer
		stderrLine(tb.Str("error: load profile ").Str(name).Str(": ").Err(err).String())
		return 2
	}
	applyProfile(p)
	var tb textbuf.Buffer
	stdoutLine(tb.Str("debug profile ").Quoted(name).Str(" applied").String())
	return 0
}

// runDeleteProfileName handles `delete debug profile name <name>`.
func runDeleteProfileName(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: delete debug profile name <name>")
		return 1
	}
	name := args[0]
	storePath := debugStorePath()
	if err := DeleteProfile(storePath, name); err != nil {
		storeError(err)
		return 2
	}
	var tb textbuf.Buffer
	stdoutLine(tb.Str("debug profile ").Quoted(name).Str(" deleted").String())
	return 0
}

func cmdProfileList() int {
	storePath := debugStorePath()
	names, err := ListProfiles(storePath)
	if err != nil {
		storeError(err)
		return 2
	}
	if len(names) == 0 {
		stdoutLine("no saved profiles")
		return 0
	}
	for _, name := range names {
		stdoutLine(name)
	}
	return 0
}

// cmdClear handles `clear debug`: reset the default stored profile.
func cmdClear() int {
	storePath := debugStorePath()
	p := NewProfile()
	if err := SaveProfile(storePath, defaultProfile, p); err != nil {
		storeError(err)
		return 2
	}
	applyProfile(p)
	stdoutLine("debug state cleared")
	return 0
}

func printShowTable(entries []ShowEntry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printRow(w, "MODULE", "LEVEL", "FLAGS", "SCOPES")
	printRow(w, "------", "-----", "-----", "------")
	for _, e := range entries {
		printRow(w, e.Module, e.Level, e.Flags, e.Scopes)
	}
	w.Flush() //nolint:errcheck // stdout
}

func printRow(w *tabwriter.Writer, cols ...string) {
	fmt.Fprintln(w, textbuf.Join(cols, "\t")) //nolint:errcheck // stdout
}

func applyProfile(p *Profile) {
	for _, info := range slogutil.Subsystems() {
		slogutil.RestoreLevel(info.Name)
		slogutil.ClearFilter(info.Name)
	}

	// Sort modules shortest-first so more-specific entries override less-specific.
	// E.g. "bgp" applies first, then "bgp.reactor" overrides for that subsystem.
	names := p.ModuleNames()
	for _, module := range names {
		entry := p.Module(module)
		if entry == nil {
			continue
		}
		for _, sub := range slogutil.SubsystemsMatching(module) {
			_ = slogutil.SetLevel(sub, entry.Level)
			if len(entry.Flags) > 0 || len(entry.Scopes) > 0 {
				flags := make([]string, len(entry.Flags))
				for i, f := range entry.Flags {
					flags[i] = f.Name
				}
				scopes := make(map[string]string, len(entry.Scopes))
				for _, s := range entry.Scopes {
					scopes[s.Kind] = s.Value
				}
				slogutil.ConfigureFilter(sub, flags, scopes)
			}
		}
	}
}

func loadOrNewProfile(storePath string) *Profile {
	p, err := LoadProfile(storePath, defaultProfile)
	if err != nil {
		return NewProfile()
	}
	return p
}

func printInvalidSubsystem(name string) int {
	var tb textbuf.Buffer
	stderrLine(tb.Str("error: unknown subsystem ").Quoted(name).String())
	stderrLine("valid subsystems:")
	for _, info := range slogutil.Subsystems() {
		if info.Description != "" {
			tb.Reset()
			stderrLine(tb.Str("  ").PadRight(info.Name, 30).Str(info.Description).String())
		} else {
			tb.Reset()
			stderrLine(tb.Str("  ").Str(info.Name).String())
		}
	}
	stderrLine("\nhierarchical prefixes also work (e.g., \"bgp\" enables all bgp.* subsystems)")
	return 1
}

func debugStorePath() string {
	if debugStoreOverride != "" {
		return debugStoreOverride
	}
	if v := env.Get("ze.debug.store"); v != "" {
		return v
	}
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return defaultDebugStore
	}
	return filepath.Join(configDir, defaultDebugStore)
}
