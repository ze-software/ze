// Design: docs/architecture/cli/command-completion.md -- shared interactive controls.

package contract

// LifecycleCommand resolves only an exact bare interactive lifecycle control.
// An interactive caller MUST confirm before dispatching the canonical command
// through its authenticated executor; recognition itself grants no authority.
// Arguments and whitespace are deliberately not consumed as part of a control.
func LifecycleCommand(input string) (canonical string, recognized bool) {
	switch input {
	case "stop":
		return "request shutdown", true
	case "restart":
		return "request restart", true
	}
	return "", false
}
