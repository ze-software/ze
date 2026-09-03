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

// VALIDATES: AC-3 — top-level commands derived from registry appear in bash command list.
func TestBashContainsCommands(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	commandsLine := extractLine(out, `local commands="`)
	if commandsLine == "" {
		t.Fatal("bash output missing 'local commands=' line")
	}

	// Check commands available through the test import chain.
	// Commands from cmd/ze/ (version, help, bgp, config, etc.) are only
	// registered when the full binary is built.
	for _, cmd := range []string{"cli", "schema", "show", "plugin", "completion", "env"} {
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

// VALIDATES: AC-5 — dynamic plugin completion calls ze cli -c "show plugins | json".
func TestBashDynamicPlugins(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, `ze cli -c 'show plugins | json'`) {
		t.Error(`bash output missing dynamic plugin completion via "ze cli -c 'show plugins | json'"`)
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
		"--debug", "--plugin", "--pprof", "--chaos-seed", "--chaos-rate",
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
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fish output missing %q", want)
		}
	}
}

// VALIDATES: fish top-level commands have descriptions from registry.
func TestFishCommandDescriptions(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	// Check commands available through the test import chain.
	for _, cmd := range []string{"cli", "schema", "show", "plugin", "completion", "env"} {
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

// VALIDATES: fish plugin completion is dynamic (calls ze cli -c "show plugins | json").
// PREVENTS: fish missing dynamically registered plugin names.
func TestFishDynamicPlugins(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	if !strings.Contains(out, `ze cli -c 'show plugins | json'`) {
		t.Error(`fish output missing dynamic plugin completion via "ze cli -c 'show plugins | json'"`)
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

// VALIDATES: AC-2 — registry-derived top-level commands present in zsh commands array.
func TestZshContainsCommands(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	// Check commands available through the test import chain.
	for _, cmd := range []string{"cli:", "schema:", "show:", "plugin:", "completion:", "env:"} {
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
		"--debug", "--help", "--version", "--plugin", "--pprof",
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

// VALIDATES: bash offers plugin name completion for --plugin argument.
// PREVENTS: completing commands instead of plugin names after --plugin.
func TestBashPluginArgCompletion(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	// Should have a prev-based check for --plugin that calls ze cli -c "show plugins | json".
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
		"nu-complete ze commands",
		"nu-complete ze subargs",
		"nu-complete ze plugins",
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
		"--plugin:", "--pprof:", "--chaos-seed:", "--chaos-rate:",
	} {
		if !strings.Contains(out, flag) {
			t.Errorf("nushell output missing global flag %q", flag)
		}
	}
}

// VALIDATES: nushell subargs completer delegates to ze completion words dynamically.
func TestNushellContainsSubcommands(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words $subcmd") {
		t.Error("nushell subargs completer should call 'ze completion words $subcmd' dynamically")
	}
	if !strings.Contains(out, "nu-complete ze subargs") {
		t.Error("nushell script missing nu-complete ze subargs completer")
	}
}

// VALIDATES: nushell show completions are dynamic via ze completion words.
func TestNushellShowDynamic(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words $subcmd") {
		t.Error("nushell completions should call 'ze completion words $subcmd' dynamically")
	}
}

// VALIDATES: nushell plugin completion is dynamic.
func TestNushellDynamicPlugins(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, `^ze cli -c "show plugins | json"`) {
		t.Error(`nushell output missing dynamic plugin completion via '^ze cli -c "show plugins | json"'`)
	}
}

// VALIDATES: nushell schema completion is dynamic via generic subargs.
func TestNushellSchemaIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words $subcmd") {
		t.Error("nushell completions should call 'ze completion words $subcmd' for dynamic subcommand data")
	}
}

// VALIDATES: "nu" alias works as shorthand for "nushell".
func TestRunNuAlias(t *testing.T) {
	code := Run([]string{"nu"})
	if code != 0 {
		t.Errorf("Run(nu) = %d, want 0", code)
	}
}

// VALIDATES: AC-4 — shell scripts expose env as a root command from registry.
// PREVENTS: env missing from shell completion because it was never added to a hardcoded list.
func TestShellScriptsExposeEnvRootFromRegistry(t *testing.T) {
	roots := shellRootCommands()
	found := false
	for _, r := range roots {
		if r.Name == "env" {
			found = true
			if r.Description == "" {
				t.Error("env root entry should have a description")
			}
			break
		}
	}
	if !found {
		t.Error("shellRootCommands() must include 'env' (registered via registry.MustRegisterRootHandler)")
	}
}

// VALIDATES: AC-4 — registry-derived root discovery preserves all pre-migration root commands.
// PREVENTS: converting to registry-derived roots silently dropping existing commands.
func TestShellRootMigrationPreservesExistingCommands(t *testing.T) {
	roots := shellRootCommands()
	nameSet := make(map[string]bool, len(roots))
	for _, r := range roots {
		nameSet[r.Name] = true
	}

	// Commands available through the test import chain (plugin/all + cli/client).
	// version, help, bgp, config, exabgp, signal, status are registered from
	// cmd/ze/ or build-tag-gated packages and may not be present in unit tests.
	testAvailable := []string{
		"cli", "interface", "plugin", "schema", "completion", "show", "env",
	}
	for _, cmd := range testAvailable {
		if !nameSet[cmd] {
			t.Errorf("pre-migration root command %q missing from shellRootCommands()", cmd)
		}
	}

	if !nameSet["show"] {
		t.Error("synthetic 'show' entry missing from shellRootCommands()")
	}

	if len(roots) < len(testAvailable) {
		t.Errorf("shellRootCommands() returned only %d entries, expected at least %d", len(roots), len(testAvailable))
	}
}

// VALIDATES: AC-4 — bash script includes env branch with dynamic completion.
// PREVENTS: env subcommand completion missing from bash.
func TestBashEnvBranch(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "env)") {
		t.Error("bash script missing env) case branch")
	}
	if !strings.Contains(out, "ze completion words env") {
		t.Error("bash env branch should call 'ze completion words env' dynamically")
	}
}

// VALIDATES: AC-4 — zsh script includes env branch with dynamic completion.
func TestZshEnvBranch(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "env)") {
		t.Error("zsh script missing env) case branch")
	}
	if !strings.Contains(out, "ze completion words env") {
		t.Error("zsh env branch should call 'ze completion words env' dynamically")
	}
}

// VALIDATES: AC-4 — fish script includes env with dynamic completion.
func TestFishEnvBranch(t *testing.T) {
	var buf strings.Builder
	generate("fish", &buf)
	out := buf.String()

	if !strings.Contains(out, "__ze_complete_dynamic env") {
		t.Error("fish script should use __ze_complete_dynamic for env")
	}
}

// VALIDATES: AC-4 — nushell script includes env with dynamic completion.
func TestNushellEnvBranch(t *testing.T) {
	var buf strings.Builder
	generate("nushell", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze completion words $subcmd") {
		t.Error("nushell subargs completer should call ze completion words dynamically for all commands including env")
	}
}

// VALIDATES: AC-4 — shell scripts use dynamic env helpers, not embedded env key lists.
// PREVENTS: env keys being hardcoded into shell scripts and going stale.
func TestShellScriptsUseDynamicEnvHelper(t *testing.T) {
	shells := []struct {
		name    string
		gen     func() string
		pattern string
	}{
		{"bash", bashScript, "ze completion words env"},
		{"zsh", zshScript, "ze completion words env"},
		{"fish", fishScript, "__ze_complete_dynamic env"},
		{"nushell", nushellScript, "ze completion words $subcmd"},
	}
	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			out := sh.gen()
			if !strings.Contains(out, sh.pattern) {
				t.Errorf("%s script should contain %q for dynamic env completion", sh.name, sh.pattern)
			}
		})
	}
}

// VALIDATES: nushell subargs completer uses correct column indices for tab-separated output.
// PREVENTS: value/description swap from wrong column0/column1 references.
func TestNushellColumnIndices(t *testing.T) {
	out := nushellScript()

	if strings.Contains(out, "$row.column2") {
		t.Error("nushell script references column2 but ze completion words produces only two columns (column0=word, column1=description)")
	}

	// Every { value: $row.columnN must use column0 (the word), not column1 (the description).
	for _, bad := range []string{"value: $row.column1", "value: $row.column2"} {
		if strings.Contains(out, bad) {
			t.Errorf("nushell script has %q; value should reference $row.column0 (the command/key name)", bad)
		}
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

// TestEveryShellReadsTheFlagInventory pins the rule that a shell completes flag
// names from the registry rather than from a list this package writes down. All
// four generators are asserted together, because the defect this catches is one
// shell being left behind when the other three are wired.
//
// VALIDATES: registration over hardcoding for flag names. A command that
// registers a flag completes in every shell without an edit here.
// PREVENTS: nushell keeping the hardcoded global-flag list as its only flag
// knowledge, which is what it had until 2026-09-03 while bash, zsh and fish all
// called `ze completion flags`.
func TestEveryShellReadsTheFlagInventory(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "nushell"} {
		var buf strings.Builder
		if code := generate(shell, &buf); code != 0 {
			t.Fatalf("generate(%s) = %d, want 0", shell, code)
		}
		if !strings.Contains(buf.String(), "completion flags") {
			t.Errorf("the %s script never calls `ze completion flags`, so it cannot complete a "+
				"flag the registry knows about", shell)
		}
	}
}

// TestEveryShellCompletesConfigSections pins the same rule for the section paths
// under `ze config show <file>`, which every shell answers through the one
// `ze config completion` engine rather than by walking the file itself.
//
// VALIDATES: `ze config show <file> <TAB>` offers section paths in every shell.
// PREVENTS: nushell answering that position from `ze completion words`, which
// knows command words and not the sections of an operator's config file.
func TestEveryShellCompletesConfigSections(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "nushell"} {
		var buf strings.Builder
		if code := generate(shell, &buf); code != 0 {
			t.Fatalf("generate(%s) = %d, want 0", shell, code)
		}
		if !strings.Contains(buf.String(), "config completion") {
			t.Errorf("the %s script never calls `ze config completion`, so `ze config show "+
				"<file> <TAB>` cannot offer section paths", shell)
		}
	}
}
