package iface

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// VALIDATES: every place that takes dhcpMu around the DHCP and RA reconcile
// counts the apply first.
//
// PREVENTS: the counter silently ceasing to count. countConfigApply is called
// from two closures inside a registration function that no unit test can reach,
// so deleting either call, or moving one below dhcpMu.Unlock(), is invisible to
// `go test ./...`. Moving it below the unlock is not hypothetical: that is how
// it was first written on 2026-09-04, and the counter was then unreadable for
// the whole 1.1 to 3.3 s window it exists to report, because the reconcile holds
// the lock for as long as stopping forty DHCP clients takes.
//
// This reads the source rather than the behaviour, which is a weaker test than
// driving the reload, and it is here because the stronger one needs a seam the
// component does not have. It is deliberately narrow: it asserts the ORDER of
// two lines it can find, and says plainly when it can find neither.
func TestEveryReconcileUnderTheLockCountsTheApplyFirst(t *testing.T) {
	data, err := os.ReadFile("register.go")
	if err != nil {
		t.Fatalf("read register.go: %v", err)
	}
	source := string(data)

	// Each site is: countConfigApply(), then dhcpMu.Lock(), then the reconcile
	// pair, then dhcpMu.Unlock(). Find every reconcileDHCP call that sits under
	// the lock and check what precedes its Lock.
	lockSites := regexp.MustCompile(`(?s)dhcpMu\.Lock\(\)\s*\n\s*reconcileDHCP\(`).FindAllStringIndex(source, -1)
	if len(lockSites) == 0 {
		t.Fatal("found no `dhcpMu.Lock()` immediately followed by `reconcileDHCP(`; the shape this test reads has changed, so re-read it rather than deleting it")
	}
	if len(lockSites) < 2 {
		t.Errorf("found %d reconcile-under-lock site(s), expected the startup apply and the reload apply; if one was removed, this test needs re-reading", len(lockSites))
	}

	for _, site := range lockSites {
		before := source[:site[0]]
		// The count must be the last thing before the lock, allowing comments
		// and blank lines between them.
		lastCall := strings.LastIndex(before, "countConfigApply()")
		if lastCall == -1 {
			t.Errorf("a reconcile under dhcpMu at byte %d has no countConfigApply() before it anywhere: the apply counter stops counting and nothing else notices", site[0])
			continue
		}
		between := before[lastCall+len("countConfigApply()"):]
		if strings.Contains(between, "dhcpMu.Unlock()") {
			t.Errorf("the countConfigApply() nearest the reconcile at byte %d is separated from it by a dhcpMu.Unlock(), so it belongs to a different site and this one counts nothing", site[0])
		}
		if strings.Contains(between, "reconcileDHCP(") {
			t.Errorf("the countConfigApply() nearest the reconcile at byte %d sits before a DIFFERENT reconcile, so this site is uncounted", site[0])
		}
	}
}

// VALIDATES: the counter is incremented before the lock is taken, never after
// it is released, at every site.
//
// PREVENTS: the exact regression described above, stated as its own assertion
// so a failure names the cause rather than an offset.
func TestTheApplyCountIsTakenBeforeTheLockNotAfterTheUnlock(t *testing.T) {
	data, err := os.ReadFile("register.go")
	if err != nil {
		t.Fatalf("read register.go: %v", err)
	}
	source := string(data)

	// A count sitting between an Unlock and anything else is the mistake: it
	// would report the apply only once the window it describes has closed.
	afterUnlock := regexp.MustCompile(`(?s)dhcpMu\.Unlock\(\)\s*\n\s*(?://[^\n]*\n\s*)*countConfigApply\(\)`)
	if loc := afterUnlock.FindStringIndex(source); loc != nil {
		t.Errorf("countConfigApply() is called right after a dhcpMu.Unlock() at byte %d.\n"+
			"The reconcile holds that lock for as long as stopping every DHCP client takes, measured at 1.1 to 3.3 s for forty of them,\n"+
			"so a count taken here is invisible for the whole window it exists to report, and an observer reads a flat counter and\n"+
			"concludes the apply never happened. Count before the Lock.", loc[0])
	}
}
