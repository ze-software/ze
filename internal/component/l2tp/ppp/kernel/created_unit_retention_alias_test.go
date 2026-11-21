// Design: docs/research/l2tpv2-ze-integration.md -- original unit retention causes
// Related: created_unit_retirement_test.go -- private regular-file model only

package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A private duplicate fault returns the actual original label plus an error.
// The real retention producer must retain that cause without closing an original.
func TestCreatedUnitRetentionOriginalAliasKeepsDuplicateCause(t *testing.T) {
	testCreatedUnitRetentionAliasCause(t, true)
}

// A distinct regular-file channel label reaches the other real alias refusal.
// Its supplied duplicate cause and both originals must survive the refusal.
func TestCreatedUnitRetentionChannelAliasKeepsDuplicateCause(t *testing.T) {
	testCreatedUnitRetentionAliasCause(t, false)
}

// Regular files model bookkeeping only, never PPP grants or native retirement.
// Hold the channel's actual RawConn through every label comparison. Successful
// later duplicates are explicit private port outcomes with no OS close authority.
func testCreatedUnitRetentionAliasCause(t *testing.T, originalAlias bool) {
	t.Helper()
	unit := retirementTestUnit(t)
	channel, err := os.Create(filepath.Join(t.TempDir(), "modeled-original-channel"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := channel.Close(); err != nil {
			t.Error(err)
		}
	})
	connection, err := channel.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Control(func(channelDescriptor uintptr) {
		channelFD := int(channelDescriptor)
		cause := errors.New("private duplicate operation failed with an original alias")
		var closed []int
		closeDuplicate := func(fd int) error { //nolint:unparam // retainNativeUnit's close callback type fixes the error result; this model close always succeeds.
			closed = append(closed, fd)
			return nil
		}
		hold, err := unit.retainNativeUnit(channelFD, func(original int) (int, error) {
			if originalAlias {
				return original, cause
			}
			return channelFD, cause
		}, closeDuplicate)
		if err == nil {
			t.Fatal("an original alias was accepted as a duplicate")
		}
		if !errors.Is(err, cause) {
			t.Fatalf("alias refusal lost the supplied duplicate cause: %v", err)
		}
		refusal := "ppp: native unit retention returned the original channel"
		if originalAlias {
			refusal = "ppp: native unit retention returned the original descriptor"
		}
		if !strings.Contains(err.Error(), refusal) {
			t.Fatalf("duplicate cause obscured the specific original alias refusal: %v", err)
		}
		if hold == nil {
			t.Fatal("alias refusal lost its completed no-descriptor hold")
		}
		if len(closed) != 0 {
			t.Fatalf("alias refusal closed a borrowed original: %v", closed)
		}
		if _, err := unit.file.Stat(); err != nil {
			t.Fatalf("alias refusal closed the original unit: %v", err)
		}
		if _, err := channel.Stat(); err != nil {
			t.Fatalf("alias refusal closed the original channel: %v", err)
		}

		retainedFD := -1
		fresh, err := unit.retainNativeUnit(channelFD, func(original int) (int, error) {
			// Three bounded model labels leave one distinct from both originals.
			for _, candidate := range []int{91, 92, 93} {
				if candidate == original {
					continue
				}
				if candidate == channelFD {
					continue
				}
				retainedFD = candidate
				return candidate, nil
			}
			return -1, errors.New("private retention model has no distinct duplicate label")
		}, closeDuplicate)
		if err != nil {
			t.Fatalf("alias refusal poisoned later retention of the same original: %v", err)
		}
		if fresh == nil {
			t.Fatal("later valid retention has no exact hold")
		}
		t.Cleanup(func() {
			if err := fresh.close(closeDuplicate); err != nil {
				t.Error(err)
			}
		})
		if err := unit.Close(); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := hold.close(closeDuplicate); err != nil {
				t.Fatalf("completed alias cleanup changed its result: %v", err)
			}
		}
		if len(closed) != 0 {
			t.Fatalf("completed alias cleanup closed an original or the later hold: %v", closed)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := unit.WaitNativePPPUnitRetired(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("alias cleanup incorrectly released the later hold: %v", err)
		}
		for range 2 {
			if err := fresh.close(closeDuplicate); err != nil {
				t.Fatalf("later exact duplicate close: %v", err)
			}
		}
		if len(closed) != 1 {
			t.Fatalf("later duplicate close count: %v", closed)
		}
		if closed[0] != retainedFD {
			t.Fatalf("later close selected %d, want exact model duplicate %d", closed[0], retainedFD)
		}
		if err := unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
			t.Fatalf("confirmed later cleanup did not retire the original: %v", err)
		}
		if _, err := channel.Stat(); err != nil {
			t.Fatalf("retention cleanup harmed the distinct original channel: %v", err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}
