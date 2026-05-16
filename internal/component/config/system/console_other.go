// Design: docs/architecture/config/syntax.md — console serial apply (non-linux stub)

//go:build !linux

package system

// ConsoleResult reports what the console apply engine did.
type ConsoleResult struct {
	Applied []string
	Skipped []ConsoleSkip
	Errors  []ConsoleError
}

// ConsoleSkip records a device that was intentionally skipped.
type ConsoleSkip struct {
	Device string
	Reason string
}

// ConsoleError records a failed console apply operation.
type ConsoleError struct {
	Device string
	Err    error
}

// ApplyConsole is a no-op on non-Linux platforms.
func ApplyConsole(_ []ConsoleDeviceEntry) ConsoleResult {
	return ConsoleResult{}
}
