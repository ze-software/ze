// Design: docs/exabgp/exabgp-migration.md -- shared byte producers for local and daemon migration.
// Related: migration_supplied.go -- bounded supplied-input admission.

package exabgp

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/migration"
)

func migrationEnvironmentDryRunWarning() *opererr.Error {
	return opererr.New("--dry-run is not supported with --env",
		"Environment conversion still returns configuration text; omit dry-run for this operation.",
		"ze exabgp migrate --env <file>")
}

type migrationProducer func(data []byte, name string, dryRun bool) command.LocalAnswer

// printMigrationFile adapts caller-local input to the byte producer the daemon
// also serves. No producer opens a filename or writes to the process's streams.
func printMigrationFile(path, source string, dryRun bool, produce migrationProducer) int {
	data, err := cliio.ReadFile(source)
	if err != nil {
		return command.PrintLocalAnswer(path, migrationRefusal(path,
			"The ExaBGP input could not be read", err))
	}
	return command.PrintLocalAnswer(path, produce(data, source, dryRun))
}

// migrationConfigBytes preserves the converter's warning, RIB and process
// diagnostics on both routes. Dry-run deliberately answers no converted bytes.
func migrationConfigBytes(data []byte, name string, dryRun bool) command.LocalAnswer {
	const path = "show exabgp migration config"
	tree, err := migration.ParseExaBGPConfig(string(data))
	if err != nil {
		return migrationRefusal(path, "The ExaBGP configuration could not be parsed", err)
	}
	result, err := migration.MigrateFromExaBGP(tree)
	if err != nil {
		return migrationRefusal(path, "The ExaBGP configuration could not be converted", err)
	}
	var diagnostic textbuf.Buffer
	for _, warning := range result.Warnings {
		diagnostic.Str("warning: ").Str(warning).Byte('\n')
	}
	if result.RIBInjected {
		diagnostic.Str("info: RIB plugin injected (required for GR, route-refresh, and a process's routes kept across sessions)\n")
	}
	for _, process := range result.Processes {
		diagnostic.Str("process:").Str(process.Name).Byte(':').Str(process.RunCmd).Byte('\n')
	}
	if dryRun {
		diagnostic.Str("dry-run: would migrate ").Str(name).Byte('\n')
		return command.LocalAnswer{Payload: command.Blob{}, Diagnostic: diagnostic.String()}
	}
	return command.LocalAnswer{
		Payload: command.Blob(migration.SerializeTree(result.Tree)), Diagnostic: diagnostic.String(),
	}
}

// migrationEnvironmentBytes maps the INI bytes without installing any value in
// this process. As with the standalone --env form, dry-run warns and converts.
func migrationEnvironmentBytes(data []byte, _ string, dryRun bool) command.LocalAnswer {
	const path = "show exabgp migration environment"
	var diagnostic textbuf.Buffer
	if dryRun {
		diagnostic.Str("warning: ").Str(migrationEnvironmentDryRunWarning().Error()).Byte('\n')
	}
	entries, err := migration.ParseExaBGPEnv(string(data))
	if err != nil {
		answer := migrationRefusal(path, "The ExaBGP environment file could not be parsed", err)
		answer.Diagnostic = diagnostic.String() + answer.Diagnostic
		return answer
	}
	if err := migration.ValidateEnvEntries(entries); err != nil {
		answer := migrationRefusal(path, "The ExaBGP environment file contains an invalid value", err)
		answer.Diagnostic = diagnostic.String() + answer.Diagnostic
		return answer
	}
	return command.LocalAnswer{
		Payload: command.Blob(migration.MapEnvToZe(entries)), Diagnostic: diagnostic.String(),
	}
}

func migrationRefusal(path, problem string, cause error) command.LocalAnswer {
	return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.Wrap(cause,
		problem,
		"Conversion needs valid ExaBGP input; no source file or running configuration is changed.",
		path+" help")))
}
