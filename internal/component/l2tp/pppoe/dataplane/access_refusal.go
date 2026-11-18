// Design: docs/architecture/l2tp/bng-5-pppoe.md -- error-owned refused preparations
// Related: access_recovery.go -- settlement of an original constructed access
package dataplane

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"time"
)

// RefuseAccess consumes the original unpublished owner's Close exactly once.
// No usable Access is returned. If Close fails, the returned error owns that
// exact recovery obligation until SettleAccessFailure succeeds. Providers MUST
// preserve this error with standard Unwrap wrapping/joining; formatting its text
// into another error discards ownership and is forbidden. The original owner
// MUST obey AccessRecovery's context, native authority and final join contract.
func RefuseAccess(cause error, owner AccessRecovery) error {
	if cause == nil {
		cause = errors.New("PPPoE preparation refusal has no original cause")
	}
	if nilResource(owner) {
		return errors.Join(cause, errors.New("PPPoE preparation refusal has no original owner"))
	}
	failure := closeAccessOwner(owner)
	if failure == nil {
		return cause
	}
	return &refusedAccessError{cause: errors.Join(cause, failure), owner: owner, identity: owner}
}

// refusedAccessError alone binds cleanup to an original constructor result.
// Neither an exported marker, a caller supplied bool nor errors.As to an
// arbitrary custom interface grants this authority. No owner accessor exists.
type refusedAccessError struct {
	cause    error
	owner    AccessRecovery
	identity any
}

func (e *refusedAccessError) Error() string { return e.cause.Error() }
func (e *refusedAccessError) Unwrap() error { return e.cause }

type refusedConstructedAccess struct{ original *preparedAccess }

func (o *refusedConstructedAccess) Close() error { return o.original.Close() }
func (o *refusedConstructedAccess) Settle(ctx context.Context) error {
	return SettleAccess(ctx, o.original)
}

type unrecoverableAccessResult struct {
	original io.Closer
	failure  error
}

func (o *unrecoverableAccessResult) Close() error                 { return o.failure }
func (o *unrecoverableAccessResult) Settle(context.Context) error { return o.failure }

// refusePreparedAccess MUST be called once for the nonnil result a failed
// provider transferred. The provider still answers nil/error. A foreign result
// is closed and retained but never receives constructed-access authority.
func refusePreparedAccess(cause error, access Access) error {
	failure := closeAccessOwner(access)
	if failure == nil {
		return cause
	}
	if original, ok := access.(*preparedAccess); ok {
		if original != nil {
			return &refusedAccessError{cause: errors.Join(cause, failure), owner: &refusedConstructedAccess{original}, identity: original}
		}
	}
	return &refusedAccessError{cause: errors.Join(cause, failure), owner: &unrecoverableAccessResult{original: access, failure: failure}}
}

type accessFailureWalk struct {
	failure error
	exit    bool
}
type accessFailureTraversalError struct {
	cause  error
	reason string
}

func (e *accessFailureTraversalError) Error() string {
	return "PPPoE preparation cleanup cannot be inspected: " + e.reason
}
func (e *accessFailureTraversalError) Unwrap() error { return e.cause }

// SettleAccessFailure collects ALL library-issued original cleanup receipts
// before invoking any owner. Shared error DAG nodes are visited once; a real
// active-path cycle, unsupported oversized graph or cancellation refuses.
// hasCleanup=false,nil means the transparent error tree contains no receipt.
// Wrappers MUST expose standard Unwrap; opaque text replacement cannot carry
// hidden cleanup ownership. Native attempts share one20s/earlier context;
// final original physical joins remain mandatory and can outlast it.
func SettleAccessFailure(ctx context.Context, failure error) (hasCleanup bool, err error) {
	if nilResource(ctx) {
		return false, errors.New("PPPoE refused access settlement requires a context")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	owners, err := collectAccessFailures(ctx, failure)
	if err != nil {
		return len(owners) != 0, err
	}
	if len(owners) == 0 {
		return false, nil
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var result error
	for _, original := range owners {
		if err := bounded.Err(); err != nil {
			return true, errors.Join(result, err)
		}
		if failure := settleAccessOwner(bounded, original.owner); failure != nil {
			result = errors.Join(result, original.cause, failure)
		}
	}
	return true, errors.Join(result, bounded.Err())
}

func collectAccessFailures(ctx context.Context, failure error) (owners []*refusedAccessError, err error) {
	const maxFailureNodes = 65536 // Finite traversal work; exhaustion retains the original root.
	stack := []accessFailureWalk{{failure: failure}}
	active := make(map[error]struct{})
	complete := make(map[error]struct{})
	identities := make(map[any]struct{})
	steps := 0
	for len(stack) != 0 {
		if err := ctx.Err(); err != nil {
			return owners, err
		}
		step := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		current := step.failure
		if current == nil {
			continue
		}
		if nilResource(current) {
			return owners, &accessFailureTraversalError{cause: failure, reason: "error graph contains a typed-nil error node"}
		}
		comparable := reflect.ValueOf(current).Comparable()
		if step.exit {
			if comparable {
				delete(active, current)
				complete[current] = struct{}{}
			}
			continue
		}
		steps++
		if steps > maxFailureNodes {
			return owners, &accessFailureTraversalError{cause: failure, reason: "error graph exceeds the finite inspection bound"}
		}
		if comparable {
			if _, done := complete[current]; done {
				continue
			}
			if _, cycle := active[current]; cycle {
				return owners, &accessFailureTraversalError{cause: failure, reason: "error graph has an active-path cycle"}
			}
			active[current] = struct{}{}
		}
		stack = append(stack, accessFailureWalk{failure: current, exit: true})
		if original, ok := current.(*refusedAccessError); ok { //nolint:errorlint // Inspect this exact walk node; recursive matching would bypass remaining graph validation.
			identity := original.identity
			if identity == nil {
				identity = original
			}
			if !reflect.ValueOf(identity).Comparable() {
				identity = original
			}
			if _, seen := identities[identity]; !seen {
				identities[identity] = struct{}{}
				owners = append(owners, original)
			}
		}
		switch wrapped := current.(type) { //nolint:errorlint // Unwrap only this exact node; the bounded walk must validate every child itself.
		case interface{ Unwrap() []error }:
			children := wrapped.Unwrap()
			if len(children) > maxFailureNodes-len(stack) {
				return owners, &accessFailureTraversalError{cause: failure, reason: "error graph exceeds the finite inspection bound"}
			}
			for _, child := range slices.Backward(children) {
				stack = append(stack, accessFailureWalk{failure: child})
			}
		case interface{ Unwrap() error }:
			stack = append(stack, accessFailureWalk{failure: wrapped.Unwrap()})
		}
	}
	return owners, nil
}
