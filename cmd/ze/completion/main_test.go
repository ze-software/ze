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
		"bgp", "config", "cli", "validate", "schema", "show", "run", "status",
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
		{"config)", []string{"edit", "check", "migrate", "fmt", "dump", "diff", "completion"}},
		{"cli)", []string{"bgp"}},
		{"schema)", []string{"list", "show", "handlers", "methods", "events", "protocol"}},
		{"signal)", []string{"reload", "stop", "quit"}},
		{"exabgp)", []string{"plugin", "migrate"}},
		{"completion)", []string{"bash", "zsh"}},
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

// VALIDATES: show completions are dynamic (call ze show help), not hardcoded.
func TestBashShowIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze show help") {
		t.Error("bash show completion should call 'ze show help' dynamically")
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

// VALIDATES: bash show awk uses Available commands section filter.
// PREVENTS: awk matching "ze" from Examples section in ze show help output.
func TestBashShowAwkPattern(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "Available commands:") {
		t.Error("bash show awk should filter within 'Available commands:' section")
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

// VALIDATES: AC-2 — all top-level commands present in zsh commands array.
func TestZshContainsCommands(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	for _, cmd := range []string{
		"bgp:", "config:", "cli:", "validate:", "schema:", "show:", "run:", "status:",
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

// VALIDATES: zsh show completions are dynamic.
func TestZshShowIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze show help") {
		t.Error("zsh show completion should call 'ze show help' dynamically")
	}
}

// VALIDATES: zsh show awk uses Available commands section filter.
// PREVENTS: awk matching spurious lines outside the commands section.
func TestZshShowAwkPattern(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "Available commands:") {
		t.Error("zsh show awk should filter within 'Available commands:' section")
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

// VALIDATES: run completions are dynamic (call ze run help), not hardcoded.
// PREVENTS: run command being silently omitted from completion.
func TestBashRunIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("bash", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze run help") {
		t.Error("bash run completion should call 'ze run help' dynamically")
	}
}

// VALIDATES: zsh run completions are dynamic.
func TestZshRunIsDynamic(t *testing.T) {
	var buf strings.Builder
	generate("zsh", &buf)
	out := buf.String()

	if !strings.Contains(out, "ze run help") {
		t.Error("zsh run completion should call 'ze run help' dynamically")
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
	code := Run([]string{"fish"})
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

// extractLine returns the first line containing prefix, or empty string.
func extractLine(text, prefix string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, prefix) {
			return line
		}
	}
	return ""
}
