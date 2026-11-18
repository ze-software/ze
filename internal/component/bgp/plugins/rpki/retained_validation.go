// Design: docs/architecture/plugin/rib-storage-design.md -- GR retained validation.
// Related: origin_tracker.go, aspa_tracker.go -- current original-source rows.
package rpki

import (
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
)

// beginRetention opens only the actual attached source lifetime. Missing
// provenance remains observational; it cannot receive a GR retention verdict.
func (rp *rPKIPlugin) beginRetention(owner, connection uint64) {
	if owner == 0 {
		return
	}
	if connection == 0 {
		return
	}
	// Both arguments were checked above, so Begin cannot refuse this lifetime.
	if err := rp.retentionVerdicts.Begin(owner, connection); err != nil {
		panic("BUG: validated RPKI retention lifetime refused")
	}
}

func (rp *rPKIPlugin) handleRetainedValidationEvent(delivered string) (bool, error) {
	if envelope, matched := retention.Accepts(delivered); matched {
		return true, rp.retentionVerdicts.Record(envelope)
	}
	envelope, matched := retention.AcceptsCleanup(delivered)
	if !matched {
		return false, nil
	}
	cleanup, err := retention.DecodeCleanup(envelope)
	if err != nil {
		return true, err
	}
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	rp.dispatchMu.Lock()
	defer rp.dispatchMu.Unlock()
	source := rpkiSource{owner: cleanup.Owner(), connection: cleanup.Connection()}
	rp.originTracker.mu.Lock()
	for key, route := range rp.originTracker.routes {
		if key.source != source {
			continue
		}
		if key.peerAddr != cleanup.Address().String() {
			continue
		}
		fam, known := family.LookupFamily(key.family)
		if !known {
			continue
		} // Only parsed registered families enter the trackers.
		if !cleanup.MatchesFamily(fam) {
			continue
		}
		if cleanup.NoLLGROnly() {
			if !route.noLLGR {
				continue
			}
		}
		delete(rp.originTracker.routes, key)
	}
	rp.originTracker.mu.Unlock()
	rp.aspaTracker.mu.Lock()
	for key, route := range rp.aspaTracker.routes {
		if key.source != source {
			continue
		}
		if key.peerAddr != cleanup.Address().String() {
			continue
		}
		fam, known := family.LookupFamily(key.family)
		if !known {
			continue
		}
		if !cleanup.MatchesFamily(fam) {
			continue
		}
		if cleanup.NoLLGROnly() {
			if !route.noLLGR {
				continue
			}
		}
		rp.aspaTracker.removeFromIndexLocked(route)
		delete(rp.aspaTracker.routes, key)
	}
	rp.aspaTracker.mu.Unlock()
	return true, nil
}

// removeUnretainedValidation follows the received source's actual DOWN. It
// preserves original cache-revalidation inputs for exactly GR's retained
// families, including an Invalid route; retention supplies no validation.
// Caller MUST hold validationMu through both tracker mutations.
func (rp *rPKIPlugin) removeUnretainedValidation(address string, source rpkiSource, retained map[family.Family]bool) {
	rp.originTracker.mu.Lock()
	for key, route := range rp.originTracker.routes {
		if key.peerAddr != address {
			continue
		}
		if key.source != source {
			continue
		}
		fam, known := family.LookupFamily(key.family)
		if known {
			if retained[fam] {
				route.retained = true
				rp.originTracker.routes[key] = route
				continue
			}
		}
		if route.retained {
			continue
		}
		delete(rp.originTracker.routes, key)
	}
	rp.originTracker.mu.Unlock()
	rp.aspaTracker.mu.Lock()
	for key, route := range rp.aspaTracker.routes {
		if key.peerAddr != address {
			continue
		}
		if key.source != source {
			continue
		}
		fam, known := family.LookupFamily(key.family)
		if known {
			if retained[fam] {
				route.retained = true
				rp.aspaTracker.routes[key] = route
				continue
			}
		}
		if route.retained {
			continue
		}
		rp.aspaTracker.removeFromIndexLocked(route)
		delete(rp.aspaTracker.routes, key)
	}
	rp.aspaTracker.mu.Unlock()
}

// validationSourceAdmitted checks the original lifetime at the actual SDK
// dispatch, after a copied request may have waited in the queue. Live initial
// decisions need no tracker slot: its existing cap does not change validation.
// After DOWN only the exact current retained input can supply a decision.
// Caller MUST hold dispatchMu through BatchValidate; source cleanup holds that
// same mutex through ledger and tracker retirement. No tombstones are stored.
func (rp *rPKIPlugin) validationSourceAdmitted(request validationRequest) bool {
	if request.source.owner == 0 {
		return request.source.connection == 0
	}
	if rp.retentionVerdicts.Active(request.source.owner, request.source.connection) {
		return true
	}
	key := routeKey{peerAddr: request.peerAddr, source: request.source, family: request.family, prefix: request.prefix, pathID: request.pathID}
	rp.originTracker.mu.Lock()
	defer rp.originTracker.mu.Unlock()
	route := rp.originTracker.routes[key]
	if route == nil {
		return false
	}
	return route.msgID == request.msgID
}
