// Design: docs/architecture/diagnostics/debug-filtering.md -- granular debug with named profiles
// Related: profile.go -- profile storage, show.go -- structured display and its text drawings, register.go -- CLI registration
//
// Grammar is verb-first (set/delete/show/clear), matching VyOS syslog-level
// configuration (docs.vyos.io): debug is persistent state edited in debug.zefs,
// not an operational toggle. The daemon `show debug` command (live runtime
// state) is a separate command in yang/ze-debug-cmd.yang.
//
// Every handler here is a local-data handler: it answers its result as a value
// and writes nothing, so the route that called it decides where the answer and
// the refusal go (`docs/architecture/api/commands.md`, "Local-data commands").

package debug

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
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

// refused answers a usage or validation refusal: exit code 1 and the lines of
// text, each ending in a newline, that the local route prints on stderr.
func refused(lines ...string) command.LocalAnswer {
	var tb textbuf.Buffer
	for _, line := range lines {
		tb.Str(line).Byte('\n')
	}
	return command.LocalRefusal(1, tb.String())
}

// storeRefusal answers a debug.zefs read or write failure: exit code 2, the
// error, and where to look.
func storeRefusal(err error) command.LocalAnswer {
	var tb textbuf.Buffer
	tb.Str("error: ").Err(err).Byte('\n')
	tb.Str("check debug.zefs file permissions and available disk space\n")
	return command.LocalRefusal(2, tb.String())
}

// changed answers a successful edit of the stored profile.
func changed(change debugChange) command.LocalAnswer {
	return command.LocalAnswer{Payload: debugChangeAnswer{Change: change}}
}

// saveAndApply persists the profile to the default slot and applies it live.
// It answers the store failure, or nil when the profile was saved.
func saveAndApply(storePath string, p *Profile) error {
	if err := saveProfile(storePath, defaultProfile, p); err != nil {
		return err
	}
	applyProfile(p)
	return nil
}

// optionRefusal is a refused module option and the lines that explain it, as
// the operator reads them.
type optionRefusal struct{ lines []string }

func (r *optionRefusal) Error() string { return textbuf.Join(r.lines, "\n") }

func refuseOption(lines ...string) error { return &optionRefusal{lines: lines} }

// optionAnswer turns a refused module option into the command's answer.
func optionAnswer(err error) command.LocalAnswer {
	var refusal *optionRefusal
	if errors.As(err, &refusal) {
		return refused(refusal.lines...)
	}
	return refused(err.Error())
}

// runSetModule handles `set debug module <name> [level <l> | flag <f> | scope <k> <v>]`.
// Bare `set debug module <name>` enables debug for the subsystem at the default
// level; the optional keyword adds/sets a level, flag, or scope.
func runSetModule(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return refused("usage: set debug module <name> [level <level> | flag <flag> | scope <kind> <value>]")
	}
	module := args[0]
	if module == "" || strings.ContainsAny(module, "/\x00") {
		return invalidSubsystem(module)
	}

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	if !p.HasModule(module) {
		p.toggleModule(module)
	}
	if len(args) > 1 {
		if err := setModuleSetting(p, module, args[1:]); err != nil {
			return optionAnswer(err)
		}
	}
	if err := saveAndApply(storePath, p); err != nil {
		return storeRefusal(err)
	}
	return changed(debugChange{Subject: subjectModule, Name: module, Result: "enabled"})
}

// setModuleSetting applies a level/flag/scope sub-option for `set`. The keyword
// follows a variable module selector, so this is argument parsing, not command
// dispatch.
func setModuleSetting(p *Profile, module string, args []string) error {
	keyword := args[0]
	if keyword == "level" {
		if len(args) < 2 {
			return refuseOption("usage: set debug module <name> level <level>")
		}
		if err := refuseTrailingWords(args, 2, "set debug module <name> level <level>"); err != nil {
			return err
		}
		level := args[1]
		if !slogutil.ValidateLevel(level) {
			var tb textbuf.Buffer
			return refuseOption(tb.Str("error: invalid level ").Quoted(level).Str(" (valid: disabled, debug, info, warn, error)").String())
		}
		p.SetLevel(module, level)
		return nil
	}
	if keyword == "flag" {
		if len(args) < 2 {
			return refuseOption("usage: set debug module <name> flag <flag>")
		}
		if err := refuseTrailingWords(args, 2, "set debug module <name> flag <flag>"); err != nil {
			return err
		}
		flag := args[1]
		if debugyang.HasModule(module) && !debugyang.ValidateFlag(module, flag) {
			var tb textbuf.Buffer
			lines := []string{tb.Str("error: unknown flag ").Quoted(flag).Str(" for ").Str(module).String()}
			if valid := debugyang.FlagsFor(module); len(valid) > 0 {
				tb.Reset()
				lines = append(lines, tb.Str("valid flags: ").Join(valid, ", ").String())
			}
			return refuseOption(lines...)
		}
		if !p.HasFlag(module, flag) {
			p.toggleFlag(module, flag)
		}
		return nil
	}
	if keyword == "scope" {
		if len(args) < 3 {
			return refuseOption("usage: set debug module <name> scope <kind> <value>")
		}
		if err := refuseTrailingWords(args, 3, "set debug module <name> scope <kind> <value>"); err != nil {
			return err
		}
		kind := args[1]
		if debugyang.HasModule(module) && !debugyang.ValidateScope(module, kind) {
			var tb textbuf.Buffer
			return refuseOption(tb.Str("error: unknown scope ").Quoted(kind).Str(" for ").Str(module).String())
		}
		if !p.HasScope(module, kind, args[2]) {
			p.toggleScope(module, kind, args[2])
		}
		return nil
	}
	var tb textbuf.Buffer
	return refuseOption(tb.Str("unknown debug option: ").Str(keyword).String())
}

// refuseTrailingWords refuses any word after the `used` words one option
// consumes. One command sets one option, so a second option written after the
// first was dropped unreported before this check.
func refuseTrailingWords(args []string, used int, usage string) error {
	if len(args) <= used {
		return nil
	}
	var tb textbuf.Buffer
	return refuseOption(tb.Str("error: unexpected word ").Quoted(args[used]).Str(" after ").Join(args[:used], " ").
		Str(": one command sets one option, so run one command per option (usage: ").Str(usage).Str(")").String())
}

// runDeleteModule handles `delete debug module <name> [flag <f> | scope <k> <v>]`.
// Bare `delete debug module <name>` disables debug for the subsystem entirely;
// the optional keyword removes just one flag or scope.
func runDeleteModule(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return refused("usage: delete debug module <name> [flag <flag> | scope <kind> <value>]")
	}
	module := args[0]

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	if !p.HasModule(module) {
		// Idempotent: deleting an already-absent module is a no-op success.
		if err := saveAndApply(storePath, p); err != nil {
			return storeRefusal(err)
		}
		return changed(debugChange{Subject: subjectModule, Name: module, Result: "disabled"})
	}
	if len(args) == 1 {
		p.toggleModule(module) // present -> removes the whole module
		if err := saveAndApply(storePath, p); err != nil {
			return storeRefusal(err)
		}
		return changed(debugChange{Subject: subjectModule, Name: module, Result: "disabled"})
	}
	if err := deleteModuleSetting(p, module, args[1:]); err != nil {
		return optionAnswer(err)
	}
	if err := saveAndApply(storePath, p); err != nil {
		return storeRefusal(err)
	}
	return changed(debugChange{Subject: subjectModule, Name: module, Result: "updated"})
}

// deleteModuleSetting removes a flag/scope sub-option for `delete`. Keyword
// after a variable selector: argument parsing, not command dispatch.
func deleteModuleSetting(p *Profile, module string, args []string) error {
	keyword := args[0]
	if keyword == "flag" {
		if len(args) < 2 {
			return refuseOption("usage: delete debug module <name> flag <flag>")
		}
		if err := refuseTrailingWords(args, 2, "delete debug module <name> flag <flag>"); err != nil {
			return err
		}
		if p.HasFlag(module, args[1]) {
			p.toggleFlag(module, args[1])
		}
		return nil
	}
	if keyword == "scope" {
		if len(args) < 3 {
			return refuseOption("usage: delete debug module <name> scope <kind> <value>")
		}
		if err := refuseTrailingWords(args, 3, "delete debug module <name> scope <kind> <value>"); err != nil {
			return err
		}
		if p.HasScope(module, args[1], args[2]) {
			p.toggleScope(module, args[1], args[2])
		}
		return nil
	}
	var tb textbuf.Buffer
	return refuseOption(tb.Str("unknown debug option: ").Str(keyword).String())
}

// runSetTimeout handles `set debug timeout <duration>`.
func runSetTimeout(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return refused("usage: set debug timeout <duration>  (e.g. 30m, 1h, 90s, 0; seconds rounded up to minutes)")
	}

	minutes, ok := duration.ParseMinutes(args[0])
	if !ok {
		return refused("error: invalid duration (use e.g. 30m, 1h, 90s, or 0 to disable; seconds rounded up to minutes)")
	}
	if minutes > 1440 {
		return refused("error: timeout must be at most 24h (1440m)")
	}

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	p.Timeout = minutes
	if err := saveProfile(storePath, defaultProfile, p); err != nil {
		return storeRefusal(err)
	}

	if minutes == 0 {
		return changed(debugChange{Subject: subjectTimeout, Result: "disabled"})
	}
	return changed(debugChange{Subject: subjectTimeout, Result: "set", Minutes: minutes})
}

// runShowProfile handles `show debug profile` (list) and
// `show debug profile name <name>` (inspect one). This is the stored view; the
// live runtime view is the separate daemon `show debug` command.
func runShowProfile(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return profileList()
	}
	if args[0] == "name" {
		if len(args) < 2 {
			return refused("usage: show debug profile name <name> [module <module>]")
		}
		// Optional `module <prefix>` filters the table to one subsystem subtree,
		// preserving the historical `debug show <module>` view. Reject any other
		// trailing tokens rather than silently ignoring them.
		subtree := ""
		if rest := args[2:]; len(rest) > 0 {
			if rest[0] != "module" || len(rest) != 2 {
				return refused("usage: show debug profile name <name> [module <module>]")
			}
			subtree = rest[1]
		}
		return showSaved(args[1], subtree)
	}
	var tb textbuf.Buffer
	return refused(tb.Str("unknown option: ").Str(args[0]).String())
}

func showSaved(name, subtree string) command.LocalAnswer {
	storePath := debugStorePath()
	// The default profile always conceptually exists (empty until first set),
	// matching the historical `debug show` view. A named profile that is absent
	// is a user error, so it reports instead of silently showing an empty table.
	if name == defaultProfile {
		return command.LocalAnswer{Payload: debugModulesAnswer{Modules: showEntries(loadOrNewProfile(storePath), subtree)}}
	}
	p, err := loadProfile(storePath, name)
	if err != nil {
		return storeRefusal(err)
	}
	return command.LocalAnswer{Payload: debugModulesAnswer{Modules: showEntries(p, subtree)}}
}

// runSaveProfile handles `set debug profile name <name>`: save the current
// default state as a named profile.
func runSaveProfile(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return refused("usage: set debug profile name <name>")
	}
	name := args[0]
	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)
	if err := saveProfile(storePath, name, p); err != nil {
		return storeRefusal(err)
	}
	return changed(debugChange{Subject: subjectProfile, Name: name, Result: "saved"})
}

// runRestoreProfile handles `set debug active name <name>`: load a named
// profile and apply it live. Applies without overwriting the default slot,
// preserving the historical restore semantics.
func runRestoreProfile(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return refused("usage: set debug active name <name>")
	}
	name := args[0]
	storePath := debugStorePath()
	p, err := loadProfile(storePath, name)
	if err != nil {
		var tb textbuf.Buffer
		return command.LocalRefusal(2, tb.Str("error: load profile ").Str(name).Str(": ").Err(err).Byte('\n').String())
	}
	applyProfile(p)
	return changed(debugChange{Subject: subjectProfile, Name: name, Result: "applied"})
}

// runDeleteProfileName handles `delete debug profile name <name>`.
func runDeleteProfileName(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return refused("usage: delete debug profile name <name>")
	}
	name := args[0]
	storePath := debugStorePath()
	if err := deleteProfile(storePath, name); err != nil {
		return storeRefusal(err)
	}
	return changed(debugChange{Subject: subjectProfile, Name: name, Result: "deleted"})
}

// profileList answers the names of the stored profiles. The slice is never
// nil, so an empty store answers an empty list rather than JSON null.
func profileList() command.LocalAnswer {
	storePath := debugStorePath()
	names, err := listProfiles(storePath)
	if err != nil {
		return storeRefusal(err)
	}
	if names == nil {
		names = []string{}
	}
	return command.LocalAnswer{Payload: debugProfilesAnswer{Profiles: names}}
}

// runClear handles `clear debug`: reset the default stored profile.
func runClear(command.ValidatedArgs) command.LocalAnswer {
	storePath := debugStorePath()
	p := newProfile()
	if err := saveProfile(storePath, defaultProfile, p); err != nil {
		return storeRefusal(err)
	}
	applyProfile(p)
	return changed(debugChange{Subject: subjectState, Result: "cleared"})
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
	p, err := loadProfile(storePath, defaultProfile)
	if err != nil {
		return newProfile()
	}
	return p
}

// invalidSubsystem refuses a module name no subsystem can carry, listing the
// subsystems that exist.
func invalidSubsystem(name string) command.LocalAnswer {
	var tb textbuf.Buffer
	lines := []string{tb.Str("error: unknown subsystem ").Quoted(name).String(), "valid subsystems:"}
	for _, info := range slogutil.Subsystems() {
		tb.Reset()
		if info.Description == "" {
			lines = append(lines, tb.Str("  ").Str(info.Name).String())
		} else {
			lines = append(lines, tb.Str("  ").PadRight(info.Name, 30).Str(info.Description).String())
		}
	}
	lines = append(lines, "\nhierarchical prefixes also work (e.g., \"bgp\" enables all bgp.* subsystems)")
	return refused(lines...)
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
