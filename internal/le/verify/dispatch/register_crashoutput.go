// Design: docs/architecture/testing/verify-freshness-scope.md -- a stage process reports its own crash to the run
// Related: dispatch.go -- the parent side, which hands the stage the crash file

// The file is register_crashoutput.go, not register.go: under internal/le a
// register.go declares a command, and this package registers a dispatch hook
// and an env key but no command.

package verifydispatch

import (
	"os"
	"runtime/debug"

	"github.com/ze-software/ze/internal/core/env"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// crashFDKey names the descriptor a verify stage process hands the Go runtime
// for its crash output. Only dispatch sets it, on the stage processes it starts.
const crashFDKey = "ze.verify.stage.crashfd"

var crashFDEntry = env.MustRegister(env.EnvEntry{
	Key:         crashFDKey,
	Type:        "int",
	Default:     "0",
	Description: "the descriptor a verify stage process gives the Go runtime for its crash output",
	Private:     true,
})

func init() {
	leroot.RegisterBeforeDispatch("verify-dispatch", armStageCrashOutput)
}

// armStageCrashOutput makes the Go runtime write this process's fatal error,
// unrecovered panic or SIGQUIT dump to the descriptor the verify run passed, so
// the run learns the stage crashed from the runtime itself rather than from
// text a stage may only be relaying. It replaces the crash file crashlog.Init
// armed, if any: a stage's crash belongs to the run that started it.
//
// The key is cleared once read, because the descriptor is this process's
// alone and a process the stage starts holds something else there.
func armStageCrashOutput(string) {
	fd := env.GetInt(crashFDEntry.Key, 0)
	if fd <= 0 {
		return
	}
	env.Set(crashFDEntry.Key, "") //nolint:errcheck // a failed clear leaves a child arming a descriptor it does not hold, which SetCrashOutput refuses
	crashOut := os.NewFile(uintptr(fd), "verify-stage-crash")
	if crashOut == nil {
		return
	}
	// SetCrashOutput holds a duplicate of the descriptor, so this one closes.
	debug.SetCrashOutput(crashOut, debug.CrashOptions{}) //nolint:errcheck // no stderr is the run's to write to here; an unarmed stage reads as not crashed
	crashOut.Close()                                     //nolint:errcheck // the runtime holds its own duplicate
}
