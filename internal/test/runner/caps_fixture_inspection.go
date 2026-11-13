// Design: docs/functional-tests.md -- inspection belongs to the private fixture controller.

package runner

import "github.com/ze-software/ze/internal/core/privilege"

// FixtureProcessInspectionCapability permits exact procfs identity reads of a
// nondumpable setcap daemon without depending on a passwd or sudo account.
// It MUST remain outside the daemon grant and the caps= vocabulary.
func FixtureProcessInspectionCapability() privilege.Capability {
	return privilege.Capability{Name: "CAP_SYS_PTRACE", Bit: 19, Use: "inspect the authenticated fixture daemon and child identities"}
}
