// Design: docs/architecture/config/syntax.md -- config archive command
// Overview: main.go -- dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/helpfmt"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdArchiveWithStorage(_ storage.Storage, args []string) int {
	return cmdArchiveImpl(args)
}

func cmdArchiveImpl(args []string) int {
	fs := flag.NewFlagSet("config archive", flag.ExitOnError)
	user := fs.String("user", "", "SSH user for daemon connection")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config archive",
			Summary: "Trigger a named config archive via the daemon",
			Usage:   []string{"ze config archive [options] <name>"},
			Examples: []string{
				"ze config archive local-backup",
				"ze config archive offsite",
			},
		}
		p.WriteErr()
		fmt.Fprintf(os.Stderr, "\nThe named archive block must be defined in the config's system { archive { } } section.\n")
		fmt.Fprintf(os.Stderr, "The daemon must be running. Archive settings come from the daemon's config.\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name>\n")
		fs.Usage()
		return exitError
	}

	archiveName := fs.Arg(0)

	creds, err := sshclient.LoadCredentialsWithFlags(*user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	// The operational command is verb-first `request config archive <name>`
	// (ai/rules/cli.md); this offline `ze config archive` tool dispatches it.
	var tb textbuf.Buffer
	result, err := sshclient.ExecCommand(creds, tb.Str("request config archive ").Str(archiveName).String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if result != "" {
		fmt.Fprint(os.Stderr, result)
	}

	return exitOK
}
