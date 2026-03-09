// Design: (none -- new feature, shell completion generation)
// Detail: bash.go -- bash completion script generation
// Detail: zsh.go -- zsh completion script generation
// Detail: fish.go -- fish completion script generation
// Detail: words.go -- dynamic completion data source for shell scripts
//
// Package completion provides the ze completion subcommand.
// It generates shell completion scripts for bash, zsh, and fish.
package completion

import (
	"fmt"
	"io"
	"os"
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
	case "words":
		return words(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s (supported: bash, zsh, fish)\n", args[0])
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze completion <shell>

Generate shell completion scripts.

Shells:
  bash    Generate bash completion script
  zsh     Generate zsh completion script
  fish    Generate fish completion script

Examples:
  eval "$(ze completion bash)"
  ze completion bash > /etc/bash_completion.d/ze
  ze completion zsh > ~/.zsh/completions/_ze
  ze completion fish > ~/.config/fish/completions/ze.fish
`)
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
	default:
		return 1
	}
	if _, err := fmt.Fprint(w, s); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
