// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared access admission lifetime
// Related: access.go -- original owner dispatch and completed Close refusal

package dataplane

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestAccessOpenRefusesCompletedClose proves both successful and failed Close
// permanently stop owner dispatch while repeated Close preserves its exact result.
func TestAccessOpenRefusesCompletedClose(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "successful close"
		var closeFailure error
		if failed {
			name = "failed close"
			closeFailure = errors.New("original backend close failed")
		}
		t.Run(name, func(t *testing.T) {
			owner := &accessOpenOwner{openFailure: errors.New("owner refused child"), closeFailure: closeFailure}
			access, err := NewAccess(testAccessInfo(), owner)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if got := access.Close(); got != any(closeFailure) {
					t.Errorf("cleanup close=%v, want exact %v", got, closeFailure)
				}
			})
			opened, err := access.Open(t.Context(), Request{})
			if opened != nil {
				t.Fatal("refused child returned a session")
			}
			if !errors.Is(err, owner.openFailure) {
				t.Fatalf("live access lost its owner's refusal: %v", err)
			}
			if owner.opens != 1 {
				t.Fatalf("live owner opens=%d, want 1", owner.opens)
			}
			if owner.closes != 0 {
				t.Fatal("failed child closed its parent")
			}
			if got := access.Close(); got != any(closeFailure) {
				t.Fatalf("first close=%v, want exact %v", got, closeFailure)
			}
			for range 2 {
				opened, err = access.Open(t.Context(), Request{})
				if opened != nil {
					t.Fatal("closed access returned a session")
				}
				if err == nil {
					t.Fatal("closed access accepted a child")
				}
				if !strings.Contains(err.Error(), "closed") {
					t.Fatalf("closed access returned another refusal: %v", err)
				}
				if got := access.Close(); got != any(closeFailure) {
					t.Fatalf("repeated close=%v, want exact %v", got, closeFailure)
				}
			}
			if owner.opens != 1 {
				t.Fatalf("completed Close reopened owner: opens=%d", owner.opens)
			}
			if owner.closes != 1 {
				t.Fatalf("owner closes=%d, want 1", owner.closes)
			}
		})
	}
}

// TestAccessOpenRejectsNilContextBeforeOwner checks plain and typed nils at the
// actual Access boundary, then proves an unchanged valid context still dispatches.
func TestAccessOpenRejectsNilContextBeforeOwner(t *testing.T) {
	var absent *accessOpenNilContext
	for name, ctx := range map[string]context.Context{"nil": nil, "typed nil": absent} {
		t.Run(name, func(t *testing.T) {
			owner := &accessOpenOwner{openFailure: errors.New("original owner refusal")}
			access, err := NewAccess(testAccessInfo(), owner)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := access.Close(); err != nil {
					t.Error(err)
				}
			})
			opened, err := access.Open(ctx, Request{})
			if opened != nil {
				t.Fatal("missing context returned a session")
			}
			if err == nil {
				t.Fatal("missing context was accepted")
			}
			if !strings.Contains(err.Error(), "context") {
				t.Fatalf("missing context reached another refusal: %v", err)
			}
			if owner.opens != 0 {
				t.Fatal("missing context reached the owner")
			}
			if owner.closes != 0 {
				t.Fatal("missing context closed the parent")
			}
			valid := t.Context()
			opened, err = access.Open(valid, Request{})
			if opened != nil {
				t.Fatal("refused child returned a session")
			}
			if !errors.Is(err, owner.openFailure) {
				t.Fatalf("valid context did not reach the owner: %v", err)
			}
			if owner.opens != 1 {
				t.Fatalf("valid context owner opens=%d, want 1", owner.opens)
			}
			if owner.context != valid {
				t.Fatal("Access replaced the caller's context")
			}
		})
	}
}

type accessOpenOwner struct {
	opens        int
	closes       int
	context      context.Context
	openFailure  error
	closeFailure error
}

func (o *accessOpenOwner) Open(ctx context.Context, _ Request) (OpenedSession, error) {
	o.opens++
	o.context = ctx
	return nil, o.openFailure
}

func (o *accessOpenOwner) Close() error {
	o.closes++
	return o.closeFailure
}

type accessOpenNilContext struct{ context.Context }
