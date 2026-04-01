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

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
)

// Run executes the completion subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	switch args[0] {
	case "bash":
		return generate("bash", os.Stdout)
	case "zsh":
		return generate("zsh", os.Stdout)
	case "fish":
		return generate("fish", os.Stdout)
	case "nushell", "nu":
		return generate("nushell", os.Stdout)
	case "words":
		return words(args[1:])
	case "peers":
		return peers()
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s (supported: bash, zsh, fish, nushell)\n", args[0])
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
				{Name: "bash", Desc: "Generate bash completion script"},
				{Name: "zsh", Desc: "Generate zsh completion script"},
				{Name: "fish", Desc: "Generate fish completion script"},
				{Name: "nushell", Desc: "Generate nushell completion script (alias: nu)"},
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
	p.Write()
}

// generate writes the completion script for the given shell to w.
func generate(shell string, w io.Writer) int {
	var s string
	switch shell {
	case "bash":
		s = bashScript()
	case "zsh":
		s = zshScript()
	case "fish":
		s = fishScript()
	case "nushell":
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
