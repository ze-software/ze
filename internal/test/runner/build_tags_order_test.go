// Design: docs/architecture/testing/runner-architecture.md -- pre-resource admission

package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestBuildTagAdmissionBeforeExclusiveLease holds a real group lease. A build
// mismatch must finish as a skip without waiting for that unavailable group.
func TestBuildTagAdmissionBeforeExclusiveLease(t *testing.T) {
	group := t.TempDir()
	lease, err := leaseExclusiveGroup(t.Context(), group)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close() //nolint:errcheck // Test owns this advisory lease.
	pr := NewParallelRunner[int](NewColors())
	pr.SetQuiet(true)
	pr.setNoSummary(true)
	pr.SetAdmission(func(int) (string, error) { return "selected DUT lacks required tag", nil })
	record := pr.AddTestWithNick("mismatch", "mismatch", 0, func(context.Context, int) (bool, error) {
		t.Error("build mismatch reached execution")
		return true, nil
	})
	record.ExclusiveGroup = group
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if !pr.Run(ctx) {
		t.Fatalf("build admission waited for a group lease: %v", record.Error)
	}
	if record.State != StateSkip {
		t.Fatalf("state %v, want skip", record.State)
	}
}

// TestBuildTagAdmissionBeforeSemaphore fills the sole execution slot. Its
// callback signals actual occupancy; the other case must still be admitted.
func TestBuildTagAdmissionBeforeSemaphore(t *testing.T) {
	pr := NewParallelRunner[int](NewColors())
	pr.SetQuiet(true)
	pr.setNoSummary(true)
	pr.SetConcurrency(1)
	// The deadline only bounds a broken scheduler's deadlock. Successful
	// admission cancels the occupant after observing its started signal.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := make(chan struct{})
	pr.SetAdmission(func(number int) (string, error) {
		if number == 0 {
			return "", nil
		}
		select {
		case <-started:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		cancel()
		return "selected DUT lacks required tag", nil
	})
	pr.AddTestWithNick("occupant", "occupant", 0, func(ctx context.Context, _ int) (bool, error) {
		close(started)
		<-ctx.Done()
		return false, ctx.Err()
	})
	record := pr.AddTestWithNick("mismatch", "mismatch", 1, func(context.Context, int) (bool, error) {
		t.Error("build mismatch reached execution")
		return true, nil
	})
	if pr.Run(ctx) {
		t.Fatal("the deliberately canceled occupant passed")
	}
	if record.State != StateSkip {
		t.Fatalf("build admission waited for the semaphore: state %v, error %v", record.State, record.Error)
	}
}

// TestBuildTagAdmissionPreservesEarlierVerdicts proves neither a malformed
// declaration nor a host skip can invoke later artifact admission.
func TestBuildTagAdmissionPreservesEarlierVerdicts(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		pr := NewParallelRunner[int](NewColors())
		pr.SetQuiet(true)
		pr.setNoSummary(true)
		var calls atomic.Int32
		pr.SetAdmission(func(int) (string, error) {
			calls.Add(1)
			return "", errors.New("artifact must not be read")
		})
		record := pr.AddTestWithNick("earlier", "earlier", 0, func(context.Context, int) (bool, error) {
			t.Error("earlier verdict reached execution")
			return true, nil
		})
		cause := errors.New("original parse error")
		record.SkipReason = "original host skip"
		record.ParseFailed = malformed
		if malformed {
			record.Error = cause
		}
		passed := pr.Run(t.Context())
		if calls.Load() != 0 {
			t.Fatal("earlier verdict invoked artifact admission")
		}
		if malformed {
			if passed {
				t.Fatal("malformed case passed")
			}
			if !errors.Is(record.Error, cause) {
				t.Fatalf("parse cause changed: %v", record.Error)
			}
			continue
		}
		if !passed {
			t.Fatalf("host skip failed: %v", record.Error)
		}
		if record.State != StateSkip {
			t.Fatalf("host verdict changed: %v", record.State)
		}
	}
}
