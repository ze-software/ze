//go:build ze_core

// Design: docs/architecture/hub-architecture.md -- the outer store ownership boundary.
// Related: ze_core_start.go -- every daemon run borrows this lifetime-owned store.
package main

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runOwnedDaemonStore owns the bound store until run and every borrower defer
// finish. run MUST finish all store users before returning and MUST NOT close
// its borrowed handle. Only a clean restart may apply an accepted recovery.
// Not safe for concurrent use of the same source lifetime.
func runOwnedDaemonStore(source storage.Storage, run func(storage.Storage) int) int {
	if source == nil {
		return daemonStoreFailure(opererr.New("the daemon has no store owner",
			"a recovery lifetime requires the bound store opened by ze start", "ze doctor"))
	}
	owner, err := storage.NewDaemonRecovery(source)
	if err != nil {
		return daemonStoreFailure(errors.Join(err, source.Close()))
	}
	code := run(owner.Borrow())
	closed, err := owner.Close()
	if err != nil {
		return daemonStoreFailure(err)
	}
	if code == rpc.DaemonRestartExitStatus {
		err = closed.Apply()
	} else {
		err = closed.Discard()
	}
	if err != nil {
		return daemonStoreFailure(err)
	}
	return code
}

func daemonStoreFailure(cause error) int {
	refusal := cause
	operatorError, found := errors.AsType[*opererr.Error](cause)
	if !found || cause != operatorError { //nolint:errorlint // Identity keeps wrapped or joined causes in the recovery diagnostic.
		fix := "ze doctor"
		if found {
			fix = operatorError.Fix()
		}
		refusal = opererr.Wrap(cause, "the daemon store could not finish its owned lifetime",
			"the daemon did not complete a clean shutdown and recovery is not confirmed", fix)
	}
	var output textbuf.Buffer
	output.Str("error: ").Err(refusal).Byte('\n').StdErr() //nolint:errcheck // The daemon exits with failure after attempting its final diagnostic.
	return 1
}
