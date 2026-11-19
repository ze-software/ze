// Design: docs/research/vpp-deployment-reference.md -- exact cleanup peer comparison
// Related: process_peer.go -- retained kernel process witnesses and lease ownership
package vpp

import (
	"errors"
	"io"
)

// SameProcess compares two retained, live kernel process witnesses. A known
// different process returns false,nil. Missing, closed, dead or unobservable
// leases return an error, never equality. Connector generations, reported PIDs
// and native server nonces do not participate in this comparison.
// Callers MUST retain both leases while using the result and MUST refuse a
// cleanup candidate on false or error. Equality is an observation, not an exit
// proof, a new lease, or permission to retire either owner.
// Safe for concurrent use, including opposite-direction comparisons and Close.
func (l *ProcessLease) SameProcess(other *ProcessLease) (bool, error) {
	left, err := l.comparisonWitness()
	if err != nil {
		return false, err
	}
	right, err := other.comparisonWitness()
	if err != nil {
		return false, err
	}
	// Witnesses serialize descriptor observation and final Close themselves.
	// Do not nest lease locks or join either process-lifetime worker here.
	if err := left.alive(); err != nil {
		return false, err
	}
	if err := right.alive(); err != nil {
		return false, err
	}
	if _, err := l.comparisonWitness(); err != nil {
		return false, err
	}
	if _, err := other.comparisonWitness(); err != nil {
		return false, err
	}
	return left.identity == right.identity, nil
}

// comparisonWitness borrows an immutable witness pointer, never its descriptor.
// SameProcess MUST check the lease again after the descriptor observations;
// Close can revoke this lease while another reference keeps that witness alive.
func (l *ProcessLease) comparisonWitness() (*processWitness, error) {
	if l == nil {
		return nil, errors.New("vpp: process comparison has a nil lease")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil, io.ErrClosedPipe
	}
	if l.witness == nil {
		return nil, errors.New("vpp: process comparison has an uninitialized lease")
	}
	return l.witness, nil
}
