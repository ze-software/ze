// Design: docs/architecture/config/syntax.md — config archive command
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"io"
	"os"

	iconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
)

func cmdArchiveWithStorage(store storage.Storage, args []string) int {
	return cmdArchiveImpl(store, args)
}

func cmdArchive(args []string) int {
	return cmdArchiveImpl(storage.NewFilesystem(), args)
}

func cmdArchiveImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config archive", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config archive [options] <name> <config-file>

Archive a configuration file to a named archive destination.

The named archive block must be defined in the config's system { archive { } } section.

Supported schemes: file://, http://, https://

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze config archive local-backup config.conf
  ze config archive offsite config.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "error: requires <name> and <config-file>\n")
		fs.Usage()
		return exitError
	}

	archiveName := fs.Arg(0)
	configPath := fs.Arg(1)

	// Read config file via storage backend (stdin supported)
	var data []byte
	var err error
	if configPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = store.ReadFile(configPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	// Parse config to extract system and archive settings
	schema := iconfig.YANGSchema()
	if schema == nil {
		fmt.Fprintf(os.Stderr, "error: failed to load YANG schema\n")
		return exitError
	}

	parser := iconfig.NewParser(schema)
	tree, err := parser.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot parse config: %v\n", err)
		return exitError
	}

	// Extract system config and archive blocks
	sys := system.ExtractSystemConfig(tree)
	configs := archive.ExtractConfigs(tree)
	if len(configs) == 0 {
		fmt.Fprintf(os.Stderr, "error: no archive blocks configured in system { archive { } }\n")
		return exitError
	}

	// Find the named archive block
	var ac *archive.ArchiveConfig
	for i := range configs {
		if configs[i].Name == archiveName {
			ac = &configs[i]
			break
		}
	}

	if ac == nil {
		fmt.Fprintf(os.Stderr, "error: archive block %q not found\n", archiveName)
		fmt.Fprintf(os.Stderr, "available: ")
		for i, c := range configs {
			if i > 0 {
				fmt.Fprintf(os.Stderr, ", ")
			}
			fmt.Fprintf(os.Stderr, "%s", c.Name)
		}
		fmt.Fprintln(os.Stderr)
		return exitError
	}

	// Validate location
	if err := archive.ValidateLocation(ac.Location); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid location for %q: %v\n", archiveName, err)
		return exitError
	}

	// Archive to the named destination
	notifier := archive.NewNotifier(configPath, []archive.ArchiveConfig{*ac}, sys)
	errs := notifier(data)

	if len(errs) > 0 {
		for _, archiveErr := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", archiveErr)
		}
		return exitError
	}

	fmt.Fprintf(os.Stderr, "archived %q to %s\n", archiveName, ac.Location)
	return exitOK
}
