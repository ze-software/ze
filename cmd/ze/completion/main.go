// Design: (none -- new feature, shell completion generation)
// Detail: bash.go -- bash completion script generation
// Detail: zsh.go -- zsh completion script generation
//
// Package completion provides the ze completion subcommand.
// It generates shell completion scripts for bash and zsh.
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
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s (supported: bash, zsh)\n", args[0])
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

Examples:
  eval "$(ze completion bash)"
  ze completion bash > /etc/bash_completion.d/ze
  ze completion zsh > ~/.zsh/completions/_ze
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
	default:
		return 1
	}
	if _, err := fmt.Fprint(w, s); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
