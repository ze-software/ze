package sysctl

import (
	"errors"
	"testing"
)

// TestWriteGuardRegistry proves the guard registry refuses an unnamed, nil or
// duplicate guard, runs guards in name order, and forgets a removed one.
//
// Method: register two refusing guards out of order and read which refusal
// CheckWrite returns, then remove each and check the write passes.
func TestWriteGuardRegistry(t *testing.T) {
	errA, errB := errors.New("a"), errors.New("b")
	if _, err := RegisterWriteGuard("", func(string, string) error { return nil }); !errors.Is(err, errGuardNameEmpty) {
		t.Errorf("empty name: got %v", err)
	}
	if _, err := RegisterWriteGuard("x", nil); !errors.Is(err, errGuardNil) {
		t.Errorf("nil guard: got %v", err)
	}
	removeB, err := RegisterWriteGuard("test-b", func(string, string) error { return errB })
	if err != nil {
		t.Fatal(err)
	}
	removeA, err := RegisterWriteGuard("test-a", func(string, string) error { return errA })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterWriteGuard("test-a", func(string, string) error { return nil }); !errors.Is(err, errGuardDuplicate) {
		t.Errorf("duplicate: got %v", err)
	}
	if err := CheckWrite("k", "v"); !errors.Is(err, errA) {
		t.Errorf("first refusal: got %v, want the guard named first", err)
	}
	removeA()
	if err := CheckWrite("k", "v"); !errors.Is(err, errB) {
		t.Errorf("after removing test-a: got %v", err)
	}
	removeB()
	if err := CheckWrite("k", "v"); err != nil {
		t.Errorf("no guard left: got %v", err)
	}
}
