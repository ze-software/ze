// Design: plan/spec-granular-debug.md -- granular debug with toggle semantics and profiles
// Related: profile.go -- profile storage, show.go -- structured display, register.go -- CLI registration

package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	debugyang "codeberg.org/thomas-mangin/ze/internal/component/debug/yang"
	"codeberg.org/thomas-mangin/ze/internal/core/duration"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.debug.store", Type: "string", Description: "Override debug profile store path (default: <config-dir>/debug.zefs)"})

const (
	defaultDebugStore = "debug.zefs"
	defaultProfile    = "default"
)

var debugStoreOverride string

var subcommands = map[string]func([]string) int{
	"show":    cmdShow,
	"restore": cmdRestore,
	"clear":   func(_ []string) int { return cmdClear() },
	"profile": cmdProfile,
	"timeout": cmdTimeout,
}

// Run executes the debug command with toggle semantics.
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage()
		return 0
	}

	if handler, ok := subcommands[args[0]]; ok {
		return handler(args[1:])
	}

	return cmdToggle(args)
}

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

func cmdToggle(args []string) int {
	module := args[0]

	if module == "" || strings.ContainsAny(module, "/\x00") {
		return printInvalidSubsystem(module)
	}

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)

	if len(args) == 1 {
		enabled := p.ToggleModule(module)
		if err := SaveProfile(storePath, defaultProfile, p); err != nil {
			var tb textbuf.Buffer
			stderrLine(tb.Str("error: ").Err(err).String())
			return 2
		}
		applyProfile(p)
		var tb textbuf.Buffer
		if enabled {
			stdoutLine(tb.Str("debug ").Str(module).Str(": enabled").String())
		} else {
			stdoutLine(tb.Str("debug ").Str(module).Str(": disabled").String())
		}
		return 0
	}

	return handleModuleArgs(storePath, p, module, args[1:])
}

func handleModuleArgs(storePath string, p *Profile, module string, args []string) int {
	if len(args) == 0 {
		return 1
	}

	handlers := map[string]func() int{
		"level": func() int {
			if len(args) < 2 {
				var tb textbuf.Buffer
				stderrLine(tb.Str("usage: debug ").Str(module).Str(" level <level>").String())
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
		},
		"flag": func() int {
			if len(args) < 2 {
				var tb textbuf.Buffer
				stderrLine(tb.Str("usage: debug ").Str(module).Str(" flag <flag>").String())
				return 1
			}
			flag := args[1]
			if debugyang.HasModule(module) && !debugyang.ValidateFlag(module, flag) {
				var tb textbuf.Buffer
				stderrLine(tb.Str("error: unknown flag ").Quoted(flag).Str(" for ").Str(module).String())
				validFlags := debugyang.FlagsFor(module)
				if len(validFlags) > 0 {
					tb.Reset()
					stderrLine(tb.Str("valid flags: ").Join(validFlags, ", ").String())
				}
				return 1
			}
			if !p.HasModule(module) {
				p.ToggleModule(module)
			}
			p.ToggleFlag(module, flag)
			return 0
		},
		"scope": func() int {
			if len(args) < 3 {
				var tb textbuf.Buffer
				stderrLine(tb.Str("usage: debug ").Str(module).Str(" scope <kind> <value>").String())
				return 1
			}
			kind := args[1]
			if debugyang.HasModule(module) && !debugyang.ValidateScope(module, kind) {
				var tb textbuf.Buffer
				stderrLine(tb.Str("error: unknown scope ").Quoted(kind).Str(" for ").Str(module).String())
				return 1
			}
			if !p.HasModule(module) {
				p.ToggleModule(module)
			}
			p.ToggleScope(module, kind, args[2])
			return 0
		},
	}

	handler, ok := handlers[args[0]]
	if !ok {
		var tb textbuf.Buffer
		stderrLine(tb.Str("unknown debug option: ").Str(args[0]).String())
		return 1
	}

	if code := handler(); code != 0 {
		return code
	}

	if err := SaveProfile(storePath, defaultProfile, p); err != nil {
		storeError(err)
		return 2
	}
	applyProfile(p)
	return 0
}

func cmdShow(args []string) int {
	subtree := ""
	if len(args) > 0 {
		showHandlers := map[string]func([]string) int{
			"saved":   cmdShowSaved,
			"profile": cmdShowProfile,
		}
		if handler, ok := showHandlers[args[0]]; ok {
			return handler(args[1:])
		}
		subtree = args[0]
	}

	storePath := debugStorePath()
	p := loadOrNewProfile(storePath)

	entries := showEntries(p, subtree)
	printShowTable(entries)
	return 0
}

func cmdShowSaved(args []string) int {
	storePath := debugStorePath()
	name := defaultProfile
	if len(args) > 0 {
		name = args[0]
	}

	p, err := LoadProfile(storePath, name)
	if err != nil {
		storeError(err)
		return 2
	}

	entries := showEntries(p, "")
	printShowTable(entries)
	return 0
}

func cmdShowProfile(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: debug show profile <name>")
		return 1
	}
	return cmdShowSaved(args)
}

func cmdRestore(args []string) int {
	storePath := debugStorePath()
	name := defaultProfile
	if len(args) >= 2 && args[0] == "profile" {
		name = args[1]
	}

	p, err := LoadProfile(storePath, name)
	if err != nil {
		var tb textbuf.Buffer
		stderrLine(tb.Str("error: load profile ").Str(name).Str(": ").Err(err).String())
		return 2
	}

	applyProfile(p)
	var tb textbuf.Buffer
	stdoutLine(tb.Str("debug profile ").Quoted(name).Str(" restored").String())
	return 0
}

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

func cmdProfile(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: debug profile <save|list|delete> [args...]")
		return 1
	}

	profileHandlers := map[string]func([]string) int{
		"save":   cmdProfileSave,
		"list":   func(_ []string) int { return cmdProfileList() },
		"delete": cmdProfileDelete,
	}

	handler, ok := profileHandlers[args[0]]
	if !ok {
		var tb textbuf.Buffer
		stderrLine(tb.Str("unknown profile command: ").Str(args[0]).String())
		return 1
	}
	return handler(args[1:])
}

func cmdProfileSave(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: debug profile save <name>")
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

func cmdProfileDelete(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: debug profile delete <name>")
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

func cmdTimeout(args []string) int {
	if len(args) == 0 {
		stderrLine("usage: debug timeout <duration>  (e.g. 30m, 1h, 90s, 0; seconds rounded up to minutes)")
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

func usage() {
	p := helpfmt.Page{
		Command: "ze debug",
		Summary: "Granular debug with toggle semantics and named profiles",
		Usage:   []string{"ze debug <module> [flag <flag>] [scope <kind> <value>]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Module Toggle", Entries: []helpfmt.HelpEntry{
				{Name: "<module>", Desc: "Toggle debug on/off for a subsystem"},
				{Name: "<module> level <level>", Desc: "Set log level (debug/info/warn/error)"},
				{Name: "<module> flag <flag>", Desc: "Toggle a debug flag"},
				{Name: "<module> scope <kind> <value>", Desc: "Toggle scope filter (plugin-defined kinds)"},
			}},
			{Title: "Display", Entries: []helpfmt.HelpEntry{
				{Name: "show", Desc: "Show active debug state"},
				{Name: "show <module>", Desc: "Show debug state for module subtree"},
				{Name: "show saved", Desc: "Show saved profile (may differ from active after reboot)"},
				{Name: "show profile <name>", Desc: "Inspect a named profile"},
			}},
			{Title: "Profiles", Entries: []helpfmt.HelpEntry{
				{Name: "restore", Desc: "Load and apply default profile"},
				{Name: "restore profile <name>", Desc: "Load and apply named profile"},
				{Name: "clear", Desc: "Clear default profile"},
				{Name: "profile save <name>", Desc: "Save current state as named profile"},
				{Name: "profile list", Desc: "List available profiles"},
				{Name: "profile delete <name>", Desc: "Delete a named profile"},
			}},
			{Title: "Other", Entries: []helpfmt.HelpEntry{
				{Name: "timeout <duration>", Desc: "Auto-disable timer (e.g. 30m, 1h, 90s; 0 to disable)"},
			}},
		},
		Examples: []string{
			"ze debug bgp.reactor",
			"ze debug bgp.reactor flag update",
			"ze debug bgp.reactor scope neighbor 192.0.2.1",
			"ze debug bgp.reactor scope direction receive",
			"ze debug show",
			"ze debug show bgp",
			"ze debug profile save bgp-deep",
			"ze debug restore",
		},
	}
	p.WriteErr()
}
