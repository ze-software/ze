// Design: docs/architecture/api/commands.md — config command ownership
//
// Register the `config` root command and its `show config *` / `validate
// config` offline shortcuts with the importable command registry. This is the
// owner package: the offline configuration CLI lives with
// internal/component/config, not under cmd/ze.
//
// Every handler parses its operands before resolving an optional store. Loose
// file readers never resolve runtime storage.
package cli

import (
	"slices"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// callerPathDirs answers the directories the daemon route reads a file a
// caller on another surface names from: the XDG config directory and the
// store's directory, the places the daemon already reads configuration from.
// The working directory `show config list` also scans is left out: the
// daemon's is wherever it was started from, a place no configuration rule
// names, and a remote caller cannot know what it holds. The local route
// reads any path its own operator names (command.CallerPath).
func callerPathDirs() []string {
	return []string{xdgConfigDir(), resolve.StoreDir("")}
}

// callerPathWithheld answers why a file inside those directories is still
// refused to a daemon-route read: the store's directory holds the store's own
// entries (the live tree, the lock, a replaced tree or blob), whose files carry
// every stored value, the HTTPS private key and the password hashes included.
// A caller reads a stored value with `show data cat`, which is
// edit-authorized; a config reader naming the store's file is not a second
// route to it.
func callerPathWithheld(resolved string) string {
	return storage.StoreOwnedPath(resolve.StoreDir(""), resolved)
}

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands, derived from storageHandlers and subcommandHandlers (the
// dispatch maps in main.go).
func subcommands() string {
	cmds := make([]string, 0, len(storageHandlers)+len(subcommandHandlers))
	for k := range storageHandlers {
		cmds = append(cmds, k)
	}
	for k := range subcommandHandlers {
		cmds = append(cmds, k)
	}
	slices.Sort(cmds)
	return textbuf.Join(cmds, ", ")
}

func init() {
	// A daemon-route handler reads a file its caller names only inside the
	// places this package reads configuration from (command.CallerPath).
	command.RegisterCallerPathDirs(callerPathDirs, callerPathWithheld)
	registerSuppliedCommands()
	command.MustRegisterLocalData("show config environment-validation", environmentValidationAnswer, registry.Meta{
		ShortHelp: "Check registered environment values and recorded range failures.", Mode: modeOffline,
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{"show config environment-validation"}, command.ShapeDoc)
	// Detached file output still carries the daemon host's usable credentials.
	// Edit authorization must run before the shared file reader, even though
	// only the operator's own local adapter can publish into that source file.
	command.MustRegisterLocalData("show config migrate", dataMigrate, registry.Meta{
		ShortHelp: "Convert a configuration file to set commands or nested blocks.", Mode: modeOffline,
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config fix-plan", dataFixPlan, registry.Meta{
		ShortHelp: "Plan repairs for a configuration file without changing it.", Mode: modeOffline,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config source", dataSource, registry.Meta{
		ShortHelp: "Show a file's masked source configuration at an optional path.", Mode: modeOffline,
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{"show config migrate", "show config source"}, command.ShapeBlob)
	command.RegisterShape([]string{"show config fix-plan"}, command.ShapeDoc)
	command.MustRegisterLocalData("show config set", dataSet, registry.Meta{
		ShortHelp: "Return a file transformed by one supplied set command.", Mode: modeOffline,
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config activate", dataActivate, registry.Meta{
		ShortHelp: "Return a file transformed by one supplied activate command.", Mode: modeOffline,
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config deactivate", dataDeactivate, registry.Meta{
		ShortHelp: "Return a file transformed by one supplied deactivate command.", Mode: modeOffline,
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config replace", dataReplace, registry.Meta{
		ShortHelp: "Return a validated replacement of a file without publishing it.", Mode: modeOffline,
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{"show config set", "show config activate", "show config deactivate", "show config replace"}, command.ShapeBlob)

	registry.MustRegisterRootHandler("config", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Configuration editing, formatting, validation, and history",
		Mode:      modeOffline,
		Section:   registry.SectionConfiguration,
		Subs:      subcommands(),
	})

	// `ze secret` sits beside `ze config`: both read and write config values
	// offline, and the $9$ codec it calls belongs to this component. Its
	// actions are root Subs so shell completion offers decode and encode.
	registry.MustRegisterRootHandler("secret", func(_ *registry.RuntimeContext, args []string) int {
		return cmdSecret(args)
	}, registry.Meta{
		ShortHelp: "Decode a $9$ secret to its plaintext, or encode a plaintext",
		Mode:      modeOffline,
		Section:   registry.SectionConfiguration,
		Subs:      secretSubs(),
	})

	// Flag inventory for shell completion (registration over hardcoding).
	// Mirrors the flag.FlagSet declarations in cmd_import.go.
	registry.RegisterCommandFlags("config import", []registry.FlagSpec{
		{Name: "--name", Description: "destination config name (one input only)", ValueHint: registry.FlagValueNone},
		{Name: "--dir", Description: "destination store folder", ValueHint: registry.FlagValueFile},
		{Name: "--yes", Description: "replace existing destination configs without asking", ValueHint: registry.FlagValueNone},
	})

	// File shortcuts read their explicit candidate/running config source.
	// Each of these answers with DATA, so the operator's pipe chain renders it
	// and no command carries a rendering flag of its own. They printed and
	// returned an exit code before, which is why
	// `ze cli -c "show config dump x.conf | json"` answered `unknown command`.
	// Complete private output requires edit authorization on the daemon route.
	command.MustRegisterLocalData("show config dump", dataDump, registry.Meta{
		ShortHelp:   "Show the fully resolved config tree of a file, as the daemon would load it.",
		Mode:        modeOffline,
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config diff", dataDiff, registry.Meta{
		ShortHelp: "Show what changed between the running and candidate configurations.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config fmt", func(validated command.ValidatedArgs) command.LocalAnswer {
		args, refusal := fmtLeafArgs(validated)
		if refusal != nil {
			return *refusal
		}
		return fmtAnswer(validated.Route(), args)
	}, registry.Meta{
		ShortHelp:   "Pretty-print the config with consistent formatting and ordering.",
		NotReadOnly: true,
	},
		command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config validation", dataValidate, registry.Meta{
		ShortHelp: "Check a config for errors without applying it.",
		Description: "Both the grammar of the file and the meaning of its values are checked, and each " +
			"problem is reported with the diagnostic code that explains it.",
		Mode: modeOffline,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config graph", func(validated command.ValidatedArgs) command.LocalAnswer {
		return graphAnswer(validated.Route(), fileLeafTokens(validated))
	}, registry.Meta{
		ShortHelp: "Show how components and peers depend on each other, as JSON.",
		Description: "It takes a config file path, or - to read the file on stdin. Inactive blocks are " +
			"pruned before the graph is built, so a deactivated peer contributes no edge and the " +
			"answer describes the config as it would run.",
	}, command.RenderLocalAnswer)

	// History shortcuts resolve the store lazily after parsing arguments.
	command.MustRegisterLocalData("show config history", dataHistory, registry.Meta{
		ShortHelp: "List config snapshots with timestamps and commit messages.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config list", dataList, registry.Meta{
		ShortHelp: "List stored config snapshots and loose config files.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)

	// dump, diff and the validation verdict are each ONE document, so the row
	// operators are refused over them by name. history and list answer rows.
	command.RegisterShape([]string{
		"show config dump", "show config diff", "show config validation",
	}, command.ShapeDoc)
	command.RegisterShape([]string{
		"show config history", "show config list",
	}, command.ShapeTab)
	command.RegisterColumns([]string{"show config history"},
		command.ColumnOrder{keyRevision, "timestamp", keyPath, "state"})
	command.RegisterColumns([]string{"show config list"}, command.ColumnOrder{keySource, keyPath})
	command.MustRegisterLocalData("show config cat", func(validated command.ValidatedArgs) command.LocalAnswer {
		return catAnswer(nil, validated.Tokens())
	}, registry.Meta{
		ShortHelp: "Print the full configuration text for a stored snapshot.",
		// The name may be any store key (meta/web/key included), and a config
		// file carries its secrets, so on the daemon the answer is authorized
		// as more than a read, as `show data cat` is.
		NotReadOnly: true,
	}, command.RenderLocalAnswer)

	// cat, fmt and graph answer TEXT the operator reads as it is (a stored
	// file, a formatted config, the graph's indented JSON), so each answers a
	// command.Blob, prints unchanged, and refuses the row operators by name.
	command.RegisterShape([]string{
		"show config cat", "show config fmt", "show config graph",
	}, command.ShapeBlob)
}

const (
	suppliedContent = "content"
	suppliedNameArg = "name"
)

// registerSuppliedCommands declares owned daemon forms for inputs held by the
// caller. The model owns every argument and its constraints; this metadata
// describes input composition for generic controls on any command surface.
// Paths and summaries remain literal so source-based contract gates read the
// same declarations that populate the runtime registry.
func registerSuppliedCommands() {
	command.MustRegisterLocalData("show config supplied set", suppliedHandler("show config supplied set", suppliedSet), registry.Meta{
		ShortHelp: "Apply one set statement to detached configuration and return its text.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, Fields: []string{suppliedBefore, suppliedAfter}},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied activate", suppliedHandler("show config supplied activate", suppliedActivate), registry.Meta{
		ShortHelp: "Apply one activate statement to detached configuration and return its text.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, Fields: []string{suppliedBefore, suppliedAfter}},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied deactivate", suppliedHandler("show config supplied deactivate", suppliedDeactivate), registry.Meta{
		ShortHelp: "Apply one deactivate statement to detached configuration and return its text.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, Fields: []string{suppliedBefore, suppliedAfter}},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied replace", suppliedHandler("show config supplied replace", suppliedReplace), registry.Meta{
		ShortHelp: "Validate and serialize a detached configuration replacement.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, Fields: []string{suppliedBefore, suppliedAfter}},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied migrate", suppliedHandler("show config supplied migrate", suppliedMigrate), registry.Meta{
		ShortHelp: "Convert supplied configuration to set commands or nested blocks.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied fix-plan", suppliedHandler("show config supplied fix-plan", suppliedFixPlan), registry.Meta{
		ShortHelp: "Plan repairs for supplied configuration without changing it.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied source", suppliedHandler("show config supplied source", suppliedSource), registry.Meta{
		ShortHelp: "Show the masked source configuration at an optional path.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied dump", suppliedHandler("show config supplied dump", suppliedDump), registry.Meta{
		ShortHelp: "Show the resolved configuration from supplied bytes.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied validation", suppliedHandler("show config supplied validation", suppliedValidation), registry.Meta{
		ShortHelp: "Check supplied configuration bytes without applying them.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied fmt", suppliedHandler("show config supplied fmt", suppliedFmt), registry.Meta{
		ShortHelp: "Format supplied configuration bytes without writing a file.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied graph", suppliedHandler("show config supplied graph", suppliedGraph), registry.Meta{
		ShortHelp: "Show dependencies from supplied configuration bytes.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, NameArg: suppliedNameArg},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show config supplied diff", suppliedHandler("show config supplied diff", suppliedDiff), registry.Meta{
		ShortHelp: "Compare two supplied configurations with secrets masked.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, Fields: []string{suppliedBefore, suppliedAfter}},
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{
		"show config supplied dump", "show config supplied validation", "show config supplied diff", "show config supplied fix-plan",
	}, command.ShapeDoc)
	command.RegisterShape([]string{
		"show config supplied fmt", "show config supplied graph", "show config supplied migrate", "show config supplied source",
		"show config supplied set", "show config supplied activate", "show config supplied deactivate", "show config supplied replace",
	}, command.ShapeBlob)
}
