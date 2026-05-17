// Design: docs/architecture/config/syntax.md -- config archive command
// Overview: main.go -- dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
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
		p.Write()
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

	result, err := sshclient.ExecCommand(creds, "config archive "+archiveName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if result != "" {
		fmt.Fprint(os.Stderr, result)
	}

	return exitOK
}
