// Design: docs/architecture/cli/plugin-modes.md -- query mode
// Related: sdk.go -- EnvPluginMode, ModeDeclare, QueryModeRequested, Run
// Related: pkg/plugin/rpc/declaration.go -- WriteDeclaration, the line both
// query-mode writers produce
//
// The entry point a plugin binary ze does not carry calls instead of running
// its own start. The ze binary answers a query with no plugin code at all
// (internal/component/plugin/cli.Run), so this file exists for the population
// that route cannot reach: a third-party binary, whose own code is the only
// thing that can hold it inert.
package sdk

import (
	"io"
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RunOrDeclare runs a plugin, or answers a declaration query without running
// it, and returns the process exit code. A third-party plugin's main hands its
// declaration and its start function to this call and exits with the code it
// returns.
//
// The declaration and the activation function are separate arguments, and that
// separation is the guarantee. Everything a live start does, which is building
// the Plugin, dialing the hub, initializing data, binding a socket and starting
// a timer, goes inside activate. Under the mode activate is never called, so
// none of it is reachable rather than merely discouraged. Work the caller does
// BEFORE this call is outside the guarantee, and a plugin that keeps such work
// in its main runs it under a query too.
//
// The declaration written here is the argument exactly as it is given. Run
// derives one field, WantsValidateOpen, from the callbacks a Plugin registered;
// a plugin that wants that field in its query answer states it in the
// declaration it passes here.
func RunOrDeclare(reg Registration, activate func() int) int {
	return runOrDeclare(os.Stdout, reg, activate)
}

// runOrDeclare carries the decision, with the answer's writer as a parameter so
// a test reads the line the caller writes to stdout.
func runOrDeclare(out io.Writer, reg Registration, activate func() int) int {
	if activate == nil {
		panic("BUG: sdk.RunOrDeclare called with no activation function")
	}
	if !QueryModeRequested() {
		return activate()
	}

	if err := rpc.WriteDeclaration(out, &reg); err != nil {
		var message textbuf.Buffer
		message.Str("error: declaration query: ").Err(err).Byte('\n')
		_ = message.StdErr() //nolint:errcheck // the answer already failed; the exit code carries it
		return 1
	}
	return 0
}
