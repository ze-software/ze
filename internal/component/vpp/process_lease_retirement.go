// Design: docs/research/vpp-deployment-reference.md -- retirement proof validation
// Related: process_peer.go -- sealed physical exit and nonjoining lease retirement
package vpp

import "errors"

// ValidateRetirement checks exact sealed physical proof without releasing a
// reference, changing a marker, observing a descriptor, or joining a worker.
// Callers MUST validate every retained lease before publishing native absence.
// Closed leases retain their original identity. Success transfers no ownership;
// Retire MUST still release each live lease. Safe for concurrent use. Nil/zero refuse.
func (l *ProcessLease) ValidateRetirement(proof ProcessExit) error {
	if l == nil {
		return errors.New("vpp: retirement validation has a nil lease")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.witness == nil {
		return errors.New("vpp: retirement validation has an uninitialized lease")
	}
	if !l.witness.coveredBy(proof) {
		return errors.New("vpp: exit proof does not cover this retained process lease")
	}
	return nil
}
