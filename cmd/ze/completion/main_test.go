package completion

import (
	"strings"
	"testing"
)

// VALIDATES: AC-1 — bash script has correct structure.
// PREVENTS: shipping a bash completion script with broken boilerplate.
func TestRunBash(t *testing.T) {
	var buf strings.Builder
	code := generate("bash", &buf)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := buf.String()

	for _, want := range []string{
		"COMPREPLY=(",
		"_ze()",
		"complete -F _ze ze",
		"_ze_filedir",
		"_init_completion",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("bash output missing %q", want)
		}
	}
}

// VALIDATES: AC-3 — all top-level commands present in bash command list.
func TestBashContainsCommands(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	// The commands variable line is the single source of truth
	commandsLine := extractLine(out, `local commands="`)
	if commandsLine == "" {
		t.Fatal("bash output missing 'local commands=' line")
	}

	for _, cmd := range []string{
		"bgp", "config", "cli", "schema", "show", "run", "status",
		"plugin", "exabgp", "signal", "completion", "version", "help",
	} {
		if !strings.Contains(commandsLine, cmd) {
			t.Errorf("bash commands line missing %q: %s", cmd, commandsLine)
		}
	}
}

// VALIDATES: AC-4 — subcommand completions present in bash case branches.
func TestBashContainsSubcommands(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	// Each case branch has a compgen -W "..." line with subcommands
	tests := []struct {
		branch string
		subs   []string
	}{
		{"bgp)", []string{"decode", "encode"}},
		{"config)", []string{"edit", "validate", "migrate", "fmt", "dump", "diff", "completion"}},
		{"cli)", []string{"help"}},
		{"schema)", []string{"list", "show", "handlers", "methods", "events", "protocol"}},
		{"signal)", []string{"reload", "stop", "restart", "reboot", "status", "quit"}},
		{"exabgp)", []string{"plugin", "migrate"}},
		{"completion)", []string{"bash", "zsh", "fish", "nushell"}},
	}

	for _, tt := range tests {
		// Find the case branch section
		idx := strings.Index(out, tt.branch)
		if idx < 0 {
			t.Errorf("bash output missing case branch %q", tt.branch)
			continue
		}
		// Extract until next ;; (end of case branch)
		section := out[idx:]
		if end := strings.Index(section, ";;"); end > 0 {
			section = section[:end]
		}
		for _, sub := range tt.subs {
			if !strings.Contains(section, sub) {
				t.Errorf("bash %s branch missing subcommand %q", tt.branch, sub)
			}
		}
	}
}

// VALIDATES: AC-5 — dynamic plugin completion calls ze --plugins --json.
func TestBashDynamicPlugins(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze --plugins --json") {
		t.Error("bash output missing dynamic plugin completion via 'ze --plugins --json'")
	}
}

// VALIDATES: show completions are dynamic (call ze completion words), not hardcoded.
func TestBashShowIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words show") {
		t.Error("bash show completion should call 'ze completion words show' dynamically")
	}
}

// VALIDATES: schema module completion is dynamic (calls ze schema list).
func TestBashSchemaIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze schema list") {
		t.Error("bash schema completion should call 'ze schema list' dynamically")
	}
}

// VALIDATES: bash handles global flags before subcommand (_ze_find_subcmd).
// PREVENTS: completion breaking when global flags precede the subcommand.
func TestBashGlobalFlagSkipping(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "_ze_find_subcmd") {
		t.Error("bash should have _ze_find_subcmd to skip global flags")
	}

	// Verify global flags are handled in the finder
	for _, flag := range []string{
		"--debug", "--plugin", "--pprof", "--chaos-seed", "--chaos-rate", "--plugins",
	} {
		if !strings.Contains(out, flag) {
			t.Errorf("bash _ze_find_subcmd missing global flag %q", flag)
		}
	}
}

// VALIDATES: bash show completion uses ze completion words with path.
// PREVENTS: hardcoded show subcommand lists going stale.
func TestBashShowUsesCompletionWords(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "path_words") {
		t.Error("bash show completion should build path_words for multi-level completion")
	}
}

// VALIDATES: AC-2 — zsh script has correct structure.
func TestRunZsh(t *testing.T) {
	var buf strings.Builder
	code := generate("zsh", &buf)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := buf.String()

	for _, want := range []string{
		"compdef _ze ze",
		"_ze()",
		"#compdef ze",
		"_arguments -C",
		"_describe",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("zsh output missing %q", want)
		}
	}
}

// VALIDATES: fish script has correct structure.
func TestRunFish(t *testing.T) {
	var buf strings.Builder
	code := generate("fish", &buf)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := buf.String()

	for _, want := range []string{
		"complete -c ze",
		"__ze_needs_command",
		"__ze_under_command",
		"__ze_complete_dynamic",
		"ze completion words $subcmd",
		"__ze_complete_dynamic show",
		"__ze_complete_dynamic run",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fish output missing %q", want)
		}
	}
}

// VALIDATES: fish top-level commands have descriptions.
func TestFishCommandDescriptions(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	for _, cmd := range []string{
		"bgp", "config", "cli", "schema", "show", "run",
		"plugin", "exabgp", "status", "signal", "completion", "version", "help",
	} {
		pattern := "-a " + cmd + " -d '"
		if !strings.Contains(out, pattern) {
			t.Errorf("fish output missing command with description: %q", cmd)
		}
	}
}

// VALIDATES: fish uses depth guards for static subcommands.
// PREVENTS: completing subcommands at wrong depth in fish.
func TestFishDepthGuards(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	// __ze_depth function must exist
	if !strings.Contains(out, "__ze_depth") {
		t.Fatal("fish output missing __ze_depth function")
	}

	// Static subcommands should use depth = 0 guards
	for _, cmd := range []string{"bgp", "config", "cli", "schema", "signal", "exabgp", "completion"} {
		pattern := "__ze_depth " + cmd + ") = 0"
		if !strings.Contains(out, pattern) {
			t.Errorf("fish missing depth guard for %q", cmd)
		}
	}
}

// VALIDATES: fish plugin completion is dynamic (calls ze --plugins --json).
// PREVENTS: fish missing dynamically registered plugin names.
func TestFishDynamicPlugins(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze --plugins --json") {
		t.Error("fish output missing dynamic plugin completion via 'ze --plugins --json'")
	}
}

// VALIDATES: fish schema completion has dynamic module names at depth 2.
// PREVENTS: fish missing YANG module names for "schema show" and "schema methods".
func TestFishSchemaIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze schema list") {
		t.Error("fish schema completion should call 'ze schema list' for dynamic module names")
	}
}

// VALIDATES: AC-2 — all top-level commands present in zsh commands array.
func TestZshContainsCommands(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	for _, cmd := range []string{
		"bgp:", "config:", "cli:", "schema:", "show:", "run:", "status:",
		"plugin:", "exabgp:", "signal:", "completion:", "version:", "help:",
	} {
		if !strings.Contains(out, "'"+cmd) {
			t.Errorf("zsh output missing command entry starting with %q", cmd)
		}
	}
}

// VALIDATES: zsh includes all global flags.
func TestZshGlobalFlags(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	for _, flag := range []string{
		"--debug", "--help", "--version", "--plugin", "--plugins", "--pprof",
		"--chaos-seed", "--chaos-rate",
	} {
		if !strings.Contains(out, flag) {
			t.Errorf("zsh output missing global flag %q", flag)
		}
	}
}

// VALIDATES: zsh show completions are dynamic via ze completion words.
func TestZshShowIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words show") {
		t.Error("zsh show completion should call 'ze completion words show' dynamically")
	}
}

// VALIDATES: zsh show completion supports multi-level path navigation.
func TestZshShowUsesCompletionWords(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "path_words") {
		t.Error("zsh show completion should build path_words for multi-level completion")
	}
}

// VALIDATES: zsh bgp and cli branches guard with CURRENT == 2.
// PREVENTS: completing subcommands at wrong depth.
func TestZshDepthGuards(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	// bgp branch should have CURRENT == 2 guard
	bgpIdx := strings.Index(out, "bgp)")
	if bgpIdx < 0 {
		t.Fatal("zsh missing bgp) branch")
	}
	bgpSection := out[bgpIdx:]
	if end := strings.Index(bgpSection, ";;"); end > 0 {
		bgpSection = bgpSection[:end]
	}
	if !strings.Contains(bgpSection, "CURRENT == 2") {
		t.Error("zsh bgp branch should guard with CURRENT == 2")
	}

	// cli branch should have CURRENT == 2 guard
	cliIdx := strings.Index(out, "cli)")
	if cliIdx < 0 {
		t.Fatal("zsh missing cli) branch")
	}
	cliSection := out[cliIdx:]
	if end := strings.Index(cliSection, ";;"); end > 0 {
		cliSection = cliSection[:end]
	}
	if !strings.Contains(cliSection, "CURRENT == 2") {
		t.Error("zsh cli branch should guard with CURRENT == 2")
	}
}

// VALIDATES: run completions are dynamic (call ze completion words run), not hardcoded.
// PREVENTS: run command being silently omitted from completion.
func TestBashRunIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words run") {
		t.Error("bash run completion should call 'ze completion words run' dynamically")
	}
}

// VALIDATES: zsh run completions are dynamic.
func TestZshRunIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words run") {
		t.Error("zsh run completion should call 'ze completion words run' dynamically")
	}
}

// VALIDATES: bash offers plugin name completion for --plugin argument.
// PREVENTS: completing commands instead of plugin names after --plugin.
func TestBashPluginArgCompletion(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	// Should have a prev-based check for --plugin that calls ze --plugins --json
	if !strings.Contains(out, `"${prev}"`) {
		t.Error("bash should check prev for flag argument completion")
	}

	// The --plugin prev case should trigger plugin name completion
	if !strings.Contains(out, `--plugin)`) {
		t.Error("bash should have --plugin) case for prev-based completion")
	}
}

// VALIDATES: AC-6 — no args shows usage and exits 1.
func TestRunNoArgs(t *testing.T) {
	code := Run(nil)
	if code != 1 {
		t.Fatalf("expected exit 1 for no args, got %d", code)
	}
}

// VALIDATES: AC-7 — unknown shell shows error and exits 1.
func TestRunUnknown(t *testing.T) {
	code := Run([]string{"powershell"})
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown shell, got %d", code)
	}
}

// VALIDATES: help flag returns 0.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		code := Run([]string{arg})
		if code != 0 {
			t.Errorf("Run(%q) = %d, want 0", arg, code)
		}
	}
}

// VALIDATES: words subcommand is reachable through Run dispatch.
// PREVENTS: words being wired to internal writeWords but not to Run.
func TestRunWords(t *testing.T) {
	// "words show" should succeed (exit 0) — produces output to stdout.
	code := Run([]string{"words", "show"})
	if code != 0 {
		t.Errorf("Run(words show) = %d, want 0", code)
	}

	// "words" with no further args should also succeed (silent, no output).
	code = Run([]string{"words"})
	if code != 0 {
		t.Errorf("Run(words) = %d, want 0", code)
	}
}

// VALIDATES: nushell script has correct structure.
func TestRunNushell(t *testing.T) {
	var buf strings.Builder
	code := generate("nushell", &buf)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := buf.String()

	for _, want := range []string{
		`extern "ze"`,
		`extern "ze bgp"`,
		`extern "ze config"`,
		`extern "ze show"`,
		`extern "ze run"`,
		"nu-complete ze plugins",
		"nu-complete ze schema-modules",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("nushell output missing %q", want)
		}
	}
}

// VALIDATES: nushell top-level extern has all global flags.
func TestNushellGlobalFlags(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	for _, flag := range []string{
		"--debug(-d)", "--help(-h)", "--version(-V)",
		"--plugin:", "--plugins", "--pprof:", "--chaos-seed:", "--chaos-rate:",
	} {
		if !strings.Contains(out, flag) {
			t.Errorf("nushell output missing global flag %q", flag)
		}
	}
}

// VALIDATES: nushell has extern definitions for all top-level subcommands.
func TestNushellContainsSubcommands(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	for _, cmd := range []string{
		"ze bgp", "ze config", "ze cli", "ze schema", "ze show", "ze run",
		"ze plugin", "ze exabgp", "ze status", "ze signal", "ze completion",
		"ze version", "ze help",
	} {
		want := `extern "` + cmd + `"`
		if !strings.Contains(out, want) {
			t.Errorf("nushell output missing extern %q", want)
		}
	}
}

// VALIDATES: nushell show/run completions are dynamic via ze completion words.
func TestNushellShowRunDynamic(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words show") {
		t.Error("nushell show completion should call 'ze completion words show' dynamically")
	}
	if !strings.Contains(out, "ze completion words run") {
		t.Error("nushell run completion should call 'ze completion words run' dynamically")
	}
}

// VALIDATES: nushell plugin completion is dynamic.
func TestNushellDynamicPlugins(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze --plugins --json") {
		t.Error("nushell output missing dynamic plugin completion via 'ze --plugins --json'")
	}
}

// VALIDATES: nushell schema completion is dynamic.
func TestNushellSchemaIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze schema list") {
		t.Error("nushell schema completion should call 'ze schema list' for dynamic module names")
	}
}

// VALIDATES: "nu" alias works as shorthand for "nushell".
func TestRunNuAlias(t *testing.T) {
	code := Run([]string{"nu"})
	if code != 0 {
		t.Errorf("Run(nu) = %d, want 0", code)
	}
}

// extractLine returns the first line containing prefix, or empty string.
func extractLine(text, prefix string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, prefix) {
			return line
		}
	}
	return ""
}
