// Design: docs/architecture/testing/ci-format.md -- compiled fixtures a .ci drives through `le test fixture`.
// Related: tunnel_fixture_ipsec.go -- the XFRM readers and waits these phases reuse, and the registrations.
//
// The two phases of test/ipsec/ipsec-respond-reinit-supersede.ci (spec-home-cpe-22 AC-2
// and AC-15 on the daemon path): a `respond` daemon holds an IKE SA, its peer restarts
// and re-initiates in parallel, and the successor supersedes the old SA only once its
// own Child SA is in the kernel.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// reinitOldSPIsFile carries the first phase's SPIs to the second. The runner starts
// every command of one test in that test's own working directory, so a relative name
// is private to the test.
const reinitOldSPIsFile = "reinit-old-spis"

// reinitEndpoints are the two daemons' addresses: only states between them count.
var reinitEndpoints = map[string]bool{addrLoopback: true, "127.0.0.2": true}

// reinitNetworks are the traffic selector halves the policies carry.
var reinitNetworks = []string{"192.0.2.0/25", "192.0.2.128/25"}

// tunnelIPsecReinitEstablished waits for the first IKE SA's Child SA in the kernel of
// the `respond` daemon and records its SPIs for tunnelIPsecReinitSuperseded.
func tunnelIPsecReinitEstablished(ctx context.Context, _ []string) error {
	readSPIs := func() ([]string, error) { return tunnelIPsecSPIs(ctx, reinitEndpoints) }
	readPolicies := func() ([]string, error) { return tunnelIPsecPolicies(ctx, reinitNetworks...) }
	spis, err := tunnelIPsecWait(ctx, 240, 250*time.Millisecond, readSPIs, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return fmt.Errorf("the first Child SA never reached the kernel: %w", err)
	}
	policies, err := tunnelIPsecWait(ctx, 240, 250*time.Millisecond, readPolicies, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return fmt.Errorf("the first Child SA's policies never reached the kernel: %w", err)
	}
	record := strings.Join(spis, " ") + "\n" + strings.Join(policies, "\n")
	if err := os.WriteFile(reinitOldSPIsFile, []byte(record), 0o600); err != nil {
		return fmt.Errorf("record the first Child SA's SPIs: %w", err)
	}
	fmt.Println("INSTALLED:", strings.Join(spis, " "))
	fmt.Println("POLICIES:", strings.Join(policies, " | "))
	return nil
}

// tunnelIPsecReinitSuperseded runs after the initiating daemon was killed (no Delete
// sent) and restarted. Its new IKE_SA_INIT is a parallel re-initiation on the `respond`
// session. The phase requires, in order: the old states stay installed until the
// successor's own states are in the kernel (never a moment with neither), the
// successor's states arrive, the old ones are then removed, and the policies are the
// ones the first SA installed.
func tunnelIPsecReinitSuperseded(ctx context.Context, _ []string) error {
	recorded, err := os.ReadFile(reinitOldSPIsFile)
	if err != nil {
		return fmt.Errorf("read the first phase's SPIs: %w", err)
	}
	lines := strings.Split(string(recorded), "\n")
	oldSPIs := strings.Fields(lines[0])
	if len(oldSPIs) < 2 {
		return fmt.Errorf("the first phase recorded %d SPIs, want at least 2", len(oldSPIs))
	}
	firstPolicies := lines[1:]
	old := make(map[string]bool, len(oldSPIs))
	for _, spi := range oldSPIs {
		old[spi] = true
	}
	readSPIs := func() ([]string, error) { return tunnelIPsecSPIs(ctx, reinitEndpoints) }

	// Phase A: until the successor's states appear, every old state is still there.
	var gap error
	var replacement []string
	_, err = tunnelIPsecWait(ctx, 240, 250*time.Millisecond, readSPIs, func(values []string) bool {
		replacement, gap = reinitSplit(values, old)
		return gap != nil || len(replacement) >= 2
	})
	if gap != nil {
		return gap
	}
	if err != nil {
		return fmt.Errorf("the successor's Child SA never reached the kernel: %w", err)
	}
	fmt.Println("SUCCESSOR-INSTALLED:", strings.Join(replacement, " "))

	// Phase B: the old states then leave.
	_, err = tunnelIPsecWait(ctx, 240, 250*time.Millisecond, readSPIs, func(values []string) bool {
		for _, spi := range values {
			if old[spi] {
				return false
			}
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("the superseded Child SA's states stayed in the kernel: %v", oldSPIs)
	}
	fmt.Println("OLD-RETIRED:", strings.Join(oldSPIs, " "))

	policies, err := tunnelIPsecPolicies(ctx, reinitNetworks...)
	if err != nil {
		return err
	}
	if !tunnelSameStrings(policies, firstPolicies) {
		return fmt.Errorf("POLICY-MOVED: before=%v after=%v", firstPolicies, policies)
	}
	fmt.Println("POLICY-STABLE:", strings.Join(policies, " | "))
	return nil
}

// reinitSplit answers the SPIs in values that the old Child SA did not hold, and an
// error when any old SPI is already gone while fewer than two new ones are installed:
// the old Child SA was removed before its successor's was installed.
func reinitSplit(values []string, old map[string]bool) ([]string, error) {
	present := make(map[string]bool, len(values))
	replacement := make([]string, 0, len(values))
	for _, spi := range values {
		present[spi] = true
		if !old[spi] {
			replacement = append(replacement, spi)
		}
	}
	if len(replacement) >= 2 {
		return replacement, nil
	}
	for spi := range old {
		if !present[spi] {
			return replacement, errors.New("OLD-REMOVED-EARLY: old state " + spi + " left the kernel before the successor's Child SA was installed")
		}
	}
	return replacement, nil
}
