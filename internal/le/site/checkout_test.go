// Design: website/AI.md -- the derivations the real-checkout tests share
package site

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	testhealth "github.com/ze-software/ze/internal/le/test/health"
)

// The real-checkout tests each ask the same questions of one unchanged tree:
// the RFC gate's snapshot, the test-health render and the RFC status page. Each
// answer takes minutes under the race detector and a whole build asks each one
// again, so one process derived the same answer up to seven times and the
// package outran its race-stage bound. They are derived once here and handed
// to every real-checkout test, the build included.
//
// Only the real checkout is shared. A fixture tree is written by its own test
// and may change between two of its reads, so every other root goes to the
// live derivation. Callers MUST NOT mutate what these return.

// checkoutComplianceSnapshot is the RFC gate snapshot of this checkout.
var checkoutComplianceSnapshot struct {
	once     sync.Once
	root     string
	snapshot rfcCompliance
	err      error
}

// complianceOfThisCheckout answers collectRFCCompliance over this checkout,
// derived once per process. Safe for concurrent use.
func complianceOfThisCheckout(t *testing.T) rfcCompliance {
	t.Helper()
	snapshot, err := sharedCompliance(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// sharedCompliance answers the shared snapshot for the real checkout and a
// fresh one for any other root.
func sharedCompliance(root string) (rfcCompliance, error) {
	checkoutComplianceSnapshot.once.Do(func() {
		checkoutComplianceSnapshot.root = root
		checkoutComplianceSnapshot.snapshot, checkoutComplianceSnapshot.err = collectRFCCompliance(root)
	})
	if checkoutComplianceSnapshot.root != root {
		return collectRFCCompliance(root)
	}
	return checkoutComplianceSnapshot.snapshot, checkoutComplianceSnapshot.err
}

// checkoutHealthRender is the test-health render of this checkout.
var checkoutHealthRender struct {
	once     sync.Once
	root     string
	rendered testhealth.Rendered
	err      error
}

// sharedTestHealth answers the shared render for the real checkout and a fresh
// one for any other root. Safe for concurrent use.
func sharedTestHealth(root string) (testhealth.Rendered, error) {
	checkoutHealthRender.once.Do(func() {
		checkoutHealthRender.root = root
		checkoutHealthRender.rendered, checkoutHealthRender.err = testhealth.Render(root)
	})
	if checkoutHealthRender.root != root {
		return testhealth.Render(root)
	}
	return checkoutHealthRender.rendered, checkoutHealthRender.err
}

// checkoutStatusPage is the RFC status page Markdown of this checkout.
var checkoutStatusPage struct {
	once sync.Once
	root string
	page string
	err  error
}

// sharedStatusPage answers the shared page for the real checkout and a fresh
// one for any other root. Safe for concurrent use.
func sharedStatusPage(root string) (string, error) {
	checkoutStatusPage.once.Do(func() {
		checkoutStatusPage.root = root
		checkoutStatusPage.page, checkoutStatusPage.err = rfc.StatusPage(root)
	})
	if checkoutStatusPage.root != root {
		return rfc.StatusPage(root)
	}
	return checkoutStatusPage.page, checkoutStatusPage.err
}

// shareCheckoutDerivations points the build's live seams at the shared
// derivations for the rest of one test: the RFC gate, the test-health render,
// the RFC status page, and the RFC collection and requirement ledger the
// corpus tests already share (rfcdetail_test.go).
func shareCheckoutDerivations(t *testing.T) {
	t.Helper()
	root := repositoryRoot(t)
	previousCompliance, previousHealth := liveRFCCompliance, liveTestHealth
	previousCollection, previousLedger := liveRFCLedger, liveRequirementLedger
	previousStatus := liveDocSources[rfcStatusSource]
	t.Cleanup(func() {
		liveRFCCompliance = previousCompliance
		liveTestHealth = previousHealth
		liveRFCLedger = previousCollection
		liveRequirementLedger = previousLedger
		liveDocSources[rfcStatusSource] = previousStatus
	})
	liveRFCCompliance = sharedCompliance
	liveTestHealth = sharedTestHealth
	liveDocSources[rfcStatusSource] = sharedStatusPage
	liveRFCLedger = func(tree string) (rfc.Collected, error) {
		if tree != root {
			return rfc.Collect(tree)
		}
		return collectedRFCsOfThisCheckout(t), nil
	}
	liveRequirementLedger = func(tree string) (rfcLedger, error) {
		if tree != root {
			return collectRequirementLedger(tree)
		}
		return publishedLedgerOfThisCheckout(t), nil
	}
}
