// Design: docs/architecture/testing/runner-architecture.md -- resource admission ordering.

package runner

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestResourceAdmissionBeforeLease holds a real lease while scheduling a skipped
// or malformed member. Neither verdict can depend on waiting for that lease.
func TestResourceAdmissionBeforeLease(t *testing.T) {
	group := t.TempDir()
	lease, err := leaseExclusiveGroup(context.Background(), group)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close() //nolint:errcheck // Test owns this advisory lease.
	for _, malformed := range []bool{false, true} {
		pr := NewParallelRunner[int](NewColors())
		pr.SetQuiet(true)
		record := pr.AddTestWithNick("refused", "refused", 0, func(context.Context, int) (bool, error) {
			t.Error("admission reached execution")
			return true, nil
		})
		parseErr := errors.New("malformed resource declaration")
		record.ExclusiveGroup = group
		record.SkipReason = "unavailable capability"
		record.ParseFailed = malformed
		if malformed {
			record.Error = parseErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		passed := pr.Run(ctx)
		cancel()
		if malformed {
			if passed {
				t.Fatal("malformed case passed")
			}
			if !errors.Is(record.Error, parseErr) {
				t.Fatalf("parse failure lost to admission: %v", record.Error)
			}
			continue
		}
		if !passed {
			t.Fatalf("skipped case waited for exclusive lease: %v", record.Error)
		}
		if record.State != StateSkip {
			t.Fatalf("state %v, want skip", record.State)
		}
	}
}

// TestSharedResourceGrammar checks both capability outcomes using the existing
// probe seam, and checks that the shared parser does not claim suite options.
func TestSharedResourceGrammar(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("capability refusal is evaluated on Linux")
	}
	original := hasCaps
	t.Cleanup(func() { hasCaps = original })
	hasCaps = func(tokens []string) (bool, error) { return !slices.Contains(tokens, capsSysTime), nil }
	var capable Record
	if _, err := ParseResourceOption(&capable, "needs-linux:caps=net-admin"); err != nil {
		t.Fatal(err)
	}
	if capable.SkipReason != "" {
		t.Fatalf("capable target skipped: %s", capable.SkipReason)
	}
	var rec Record
	for _, declaration := range []string{"needs-linux:caps=net-admin", "needs-linux:caps=sys-time", "exclusive:group=admission"} {
		matched, err := ParseResourceOption(&rec, declaration)
		if err != nil {
			t.Fatal(err)
		}
		if !matched {
			t.Fatalf("resource option not recognized: %s", declaration)
		}
	}
	if !rec.NeedsLinux {
		t.Fatal("lost Linux requirement")
	}
	if !strings.Contains(rec.SkipReason, capsSysTime) {
		t.Fatalf("skip does not name missing capability: %s", rec.SkipReason)
	}
	if strings.Contains(rec.SkipReason, capsNetAdmin) {
		t.Fatalf("skip names a capability the host holds: %s", rec.SkipReason)
	}
	if rec.ExclusiveGroup != "admission" {
		t.Fatalf("group: %s", rec.ExclusiveGroup)
	}
	if matched, err := ParseResourceOption(&rec, "width:value=80"); matched || err != nil {
		t.Fatalf("claimed suite option: %t, %v", matched, err)
	}
	if matched, err := ParseResourceOption(&rec, "timeout:value=1s"); matched || err != nil {
		t.Fatalf("claimed nonresource runner option: %t, %v", matched, err)
	}
	for _, declaration := range []string{"exclusive:group=a:group=b", "exclusive:wrong=a", "exclusive", "needs-linux:caps=typo", "needs-linux:caps="} {
		var invalid Record
		matched, err := ParseResourceOption(&invalid, declaration)
		if !matched {
			t.Fatalf("lost resource option: %s", declaration)
		}
		if err == nil {
			t.Fatalf("accepted malformed resource: %s", declaration)
		}
	}
}
