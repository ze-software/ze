// Design: (none -- new feature, shell completion generation)
// Detail: bash.go -- bash completion script generation
// Detail: zsh.go -- zsh completion script generation
// Detail: fish.go -- fish completion script generation
// Detail: nushell.go -- nushell completion script generation
// Detail: words.go -- dynamic completion data source for shell scripts
// Detail: peers.go -- dynamic peer selector completion from running daemon
//
// Package completion provides the ze completion subcommand.
// It generates shell completion scripts for bash, zsh, fish, and nushell.
package completion

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/core/helpfmt"
)

// Run executes the completion subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	switch args[0] {
	case shellBash:
		return generate(shellBash, os.Stdout)
	case shellZsh:
		return generate(shellZsh, os.Stdout)
	case shellFish:
		return generate(shellFish, os.Stdout)
	case shellNushell, "nu":
		return generate(shellNushell, os.Stdout)
	case "words":
		return words(args[1:])
	case "flags":
		return flags(args[1:])
	case "families":
		return families()
	case "peers":
		return peers()
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s (supported: %s)\n", args[0], subcommands())
		usage()
		return 1
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze completion",
		Summary: "Generate shell completion scripts",
		Usage:   []string{"ze completion <shell>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Shells", Entries: []helpfmt.HelpEntry{
				{Name: shellBash, Desc: "Generate bash completion script"},
				{Name: shellZsh, Desc: "Generate zsh completion script"},
				{Name: shellFish, Desc: "Generate fish completion script"},
				{Name: shellNushell, Desc: "Generate nushell completion script (alias: nu)"},
			}},
		},
		Examples: []string{
			`eval "$(ze completion bash)"`,
			"ze completion bash > /etc/bash_completion.d/ze",
			"ze completion zsh > ~/.zsh/completions/_ze",
			"ze completion fish > ~/.config/fish/completions/ze.fish",
			`ze completion nushell | save -f ($nu.default-config-dir | path join "completions" "ze.nu")`,
		},
	}
	p.WriteErr()
}

// generate writes the completion script for the given shell to w.
func generate(shell string, w io.Writer) int {
	var s string
	switch shell {
	case shellBash:
		s = bashScript()
	case shellZsh:
		s = zshScript()
	case shellFish:
		s = fishScript()
	case shellNushell:
		s = nushellScript()
	default:
		return 1
	}
	if _, err := fmt.Fprint(w, s); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
