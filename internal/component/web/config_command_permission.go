// Design: docs/architecture/web-interface.md -- shared config mutation authorization.

package web

// ConfigCommandPermission returns the live browser editor's mutation permission
// for a configuration command. An empty permission means the command has no
// editor mutation gate; it does not establish that the command is supported.
// Callers MUST independently admit their supported command grammar, as the
// live executor does after this gate. Parse failures never grant permission.
func ConfigCommandPermission(line string) (string, error) {
	command := parseCLICommand(line)
	if command.parseErr != nil {
		return "", command.parseErr
	}
	return terminalAuthCommand(command), nil
}
