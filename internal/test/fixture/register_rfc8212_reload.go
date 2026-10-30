// Design: docs/functional-tests.md -- compiled entrypoints for the RFC 8212 reload tests.
// Related: rfc8212_reload.go, rfc8212_reload_fault.go -- existing protocol paths only.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
)

func init() {
	Register("reload/rfc8212-explicit-policy-reload", rfc8212ReloadDriver(rfc8212ReloadRecover))
	Register("reload/rfc8212-import-no-refresh", rfc8212ReloadDriver(rfc8212ReloadNoRefresh))
	Register("reload/rfc8212-import-invalidate", rfc8212ReloadDriver(rfc8212ReloadInvalidate))
	Register("reload/rfc8212-import-inherited", rfc8212ReloadDriver(rfc8212ReloadInherited))
	Register("reload/rfc8212-import-rollback", rfc8212ReloadDriver(rfc8212ReloadRollback))
	Register("reload/rfc8212-import-owner-failure", rfc8212ReloadDriver(rfc8212ReloadOwner))
	Register("reload/rfc8212-owner", rfc8212ReloadFaultDriver)
	Register("reload/rfc8212-finalizer", rfc8212ReloadFinalizer)
	Register("reload/rfc8212-select-owner", rfc8212SelectOwner)
	Register("reload/rfc8212-refusal-guard", rfc8212ReloadRefusalGuard)
}

func rfc8212SelectOwner(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("rfc8212-select-owner needs unsupported, ack, deadline, death or finalize")
	}
	allowed := map[string]bool{"unsupported": true, "ack": true, "deadline": true, "death": true, "finalize": true}
	if !allowed[args[0]] {
		return fmt.Errorf("unknown owner fault %q", args[0])
	}
	return os.WriteFile("owner.mode", []byte(args[0]), 0o600)
}
