// Design: docs/architecture/rib/unified-locrib.md -- authoritative candidates.
// Related: candidates.go -- the existing candidate display value producer.

package sysrib

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// routeViewCandidateSnapshot owns one immutable scan, independently of selected
// cursors. Only the session lease may use it. Nil and an empty id are unusable.
// Expiration releases the bulk values while retaining a small refusal identity.
type routeViewCandidateSnapshot struct {
	id       string
	selector routeViewCandidateSelector
	entries  []json.RawMessage
	cursor   uint64
	deadline time.Time
	expired  bool
}

// The selector's zero value means all families and prefixes. Only its parser
// constructs a constrained selector from raw request fields.
type routeViewCandidateSelector struct {
	family family.Family
	prefix netip.Prefix
}

func (session *routeViewSession) releaseCandidates() {
	if session.candidates != nil {
		session.candidates.entries = nil
		session.candidates.expired = true
	}
	session.candidates = nil
}

// expireCandidates is called by the existing lifecycle worker under requestMu.
func (session *routeViewSession) expireCandidates(now time.Time) {
	pin := session.candidates
	if pin == nil {
		return
	}
	if now.Before(pin.deadline) {
		return
	}
	pin.entries = nil
	pin.expired = true
}

func (authority *routeViewAuthority) readCandidates(ctx context.Context, owner *process.Process, input rpc.RouteViewCandidatesInput) (rpc.RouteViewCandidatesOutput, error) {
	if input.Limit == 0 {
		return rpc.RouteViewCandidatesOutput{}, routeViewRefusal("Candidate page limit is zero", "A page must request at least one prefix", "Resend route-view-candidates with limit between 1 and 4096")
	}
	if input.Limit > rpc.RouteViewRecordsMax {
		return rpc.RouteViewCandidatesOutput{}, routeViewRefusal("Candidate page limit exceeds 4096", "A reply cannot exceed the record bound", "Resend route-view-candidates with limit between 1 and 4096")
	}
	selector, err := routeViewCandidateSelectorOf(input)
	if err != nil {
		return rpc.RouteViewCandidatesOutput{}, err
	}
	if input.Snapshot == "" {
		if input.Cursor != 0 {
			return rpc.RouteViewCandidatesOutput{}, routeViewRefusal("A new candidate scan has a nonzero cursor", "No previous page establishes that position", "Resend route-view-candidates with snapshot omitted and cursor 0")
		}
	}
	var output rpc.RouteViewCandidatesOutput
	err = authority.withSession(ctx, owner, input.Session, func(session *routeViewSession) error {
		if input.Snapshot == "" {
			session.releaseCandidates()
			pin, err := session.scanCandidates(ctx, selector)
			if err != nil {
				return err
			}
			session.candidates = pin
		}
		pin := session.candidates
		if pin == nil {
			return routeViewRefusal("Candidate snapshot is no longer pinned", "Completed or closed scans cannot be continued", "Resend route-view-candidates with snapshot omitted and cursor 0")
		}
		if input.Snapshot != "" {
			if input.Snapshot != pin.id {
				return routeViewRefusal("Candidate snapshot identity does not match", "Another scan's cursor cannot address this scan", "Use the snapshot identity returned to this session by route-view-candidates")
			}
		}
		if input.Cursor != pin.cursor {
			return routeViewRefusal("Candidate snapshot cursor does not match its next page", "Stale and future positions cannot skip or replay pinned rows", "Continue route-view-candidates with the cursor from its most recent page")
		}
		if selector != pin.selector {
			return routeViewRefusal("Candidate selectors changed during paging", "Every page belongs to one immutable scan", "Keep the same family and prefix selectors or begin a new scan at cursor 0")
		}
		session.expireCandidates(time.Now())
		if pin.expired {
			return routeViewRefusal("Candidate snapshot expired", "Its retained rows have been released", "Resend route-view-candidates with snapshot omitted and cursor 0")
		}
		var count int
		var err error
		output, count, err = pin.page(session, int(input.Limit))
		if err != nil {
			return err
		}
		pin.cursor = output.Cursor
		clear(pin.entries[:count])
		pin.entries = pin.entries[count:]
		if output.Complete {
			session.releaseCandidates()
		}
		return nil
	})
	return output, err
}

// page counts the complete result, including the actual cursor width and the
// completion flag. Prefixes are indivisible; a fitting prefix is never refused
// by a guessed path count or a wider cursor than the one this page returns.
func (pin *routeViewCandidateSnapshot) page(session *routeViewSession, limit int) (rpc.RouteViewCandidatesOutput, int, error) {
	output := rpc.RouteViewCandidatesOutput{Session: session.token, Epoch: session.epoch, Snapshot: pin.id}
	header, err := json.Marshal(&output)
	if err != nil {
		return rpc.RouteViewCandidatesOutput{}, 0, opererr.Wrap(err, "Candidate reply header could not be encoded", "No pinned rows were consumed", "Resend route-view-candidates with snapshot omitted and cursor 0")
	}
	count := 0
	payloadBytes := 0
	for i := range min(limit, len(pin.entries)) {
		var digits [20]byte
		cursorWidth := len(strconv.AppendUint(digits[:0], pin.cursor+uint64(i+1), 10))
		// The header contains cursor 0 and complete false, with no entries.
		headerBytes := len(header) - 1 + cursorWidth + len(`,"entries":[]`)
		if i+1 == len(pin.entries) {
			headerBytes-- // complete true uses one byte less than false.
		}
		comma := 0
		if i > 0 {
			comma = 1
		}
		remaining := rpc.RouteViewResultBudget() - headerBytes - payloadBytes - comma
		if len(pin.entries[i]) > remaining {
			if count == 0 {
				return rpc.RouteViewCandidatesOutput{}, 0, routeViewRefusal("A candidate prefix exceeds the complete reply frame", "Splitting its paths would lose the atomic prefix answer", "Request a prefix whose complete candidate entry fits the routing-view reply budget")
			}
			break
		}
		payloadBytes += comma + len(pin.entries[i])
		count++
	}
	output.Cursor = pin.cursor + uint64(count)
	output.Complete = count == len(pin.entries)
	output.Entries = slices.Clone(pin.entries[:count])
	return output, count, nil
}

func routeViewCandidateSelectorOf(input rpc.RouteViewCandidatesInput) (routeViewCandidateSelector, error) {
	const fix = "Resend route-view-candidates with a registered afi/safi pair and a masked prefix of that address family"
	selector := routeViewCandidateSelector{prefix: input.Prefix}
	if input.AFI == 0 {
		if input.SAFI != 0 {
			return selector, routeViewRefusal("Candidate family has safi without afi", "Both family fields must identify the same registered family", fix)
		}
	}
	if input.AFI != 0 {
		if input.SAFI == 0 {
			return selector, routeViewRefusal("Candidate family has afi without safi", "Both family fields must identify the same registered family", fix)
		}
		selector.family = family.Family{AFI: family.AFI(input.AFI), SAFI: family.SAFI(input.SAFI)}
		registered := false
		for _, name := range family.RegisteredFamilyNames() {
			fam, ok := family.LookupFamily(name)
			if ok {
				if fam == selector.family {
					registered = true
					break
				}
			}
		}
		if !registered {
			return selector, opererr.Wrap(fmt.Errorf("family %d/%d is not registered", input.AFI, input.SAFI), "Candidate family is unknown", "Wire input cannot register address families", fix)
		}
	}
	if !input.Prefix.IsValid() {
		return selector, nil
	}
	if input.Prefix != input.Prefix.Masked() {
		return selector, routeViewRefusal("Candidate prefix contains host bits", "The selector must name one exact stored prefix", fix)
	}
	if input.Prefix.Addr().Is4In6() {
		return selector, routeViewRefusal("Candidate prefix is IPv4-mapped IPv6", "The selector must use its native address family", fix)
	}
	if input.Prefix.Addr().Zone() != "" {
		return selector, routeViewRefusal("Candidate prefix contains an address zone", "Stored prefixes have no interface zone", fix)
	}
	if selector.family.AFI == family.AFIIPv4 {
		if !input.Prefix.Addr().Is4() {
			return selector, routeViewRefusal("Candidate IPv4 family has an IPv6 prefix", "The family and prefix select different tables", fix)
		}
	}
	if selector.family.AFI == family.AFIIPv6 {
		if !input.Prefix.Addr().Is6() {
			return selector, routeViewRefusal("Candidate IPv6 family has an IPv4 prefix", "The family and prefix select different tables", fix)
		}
	}
	return selector, nil
}

// scanCandidates captures only owned renderer values under the RIB's locks.
// Sorting and JSON encoding happen after every shard callback has returned.
func (session *routeViewSession) scanCandidates(ctx context.Context, selector routeViewCandidateSelector) (*routeViewCandidateSnapshot, error) {
	deadline := time.Now().Add(routeViewSnapshotLifetime)
	work, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	var captured []candidateEntry
	for _, fam := range session.authoritativeRIB().Families() {
		if selector.family.AFI != 0 {
			if fam != selector.family {
				continue
			}
		}
		if err := session.checkRequest(work); err != nil {
			return nil, routeViewStopped(err, session.owner)
		}
		if selector.prefix.IsValid() {
			session.authoritativeRIB().Inspect(fam, selector.prefix, func(group locrib.PathGroup) {
				captured = append(captured, candidateEntryOf(fam, selector.prefix, &group))
			})
			continue
		}
		var scanErr error
		session.authoritativeRIB().Iterate(fam, func(prefix netip.Prefix, group locrib.PathGroup) bool {
			if err := session.checkRequest(work); err != nil {
				scanErr = err
				return false
			}
			captured = append(captured, candidateEntryOf(fam, prefix, &group))
			return true
		})
		if scanErr != nil {
			return nil, routeViewStopped(scanErr, session.owner)
		}
	}
	slices.SortFunc(captured, func(a, b candidateEntry) int {
		if compared := cmp.Compare(a.Family.String(), b.Family.String()); compared != 0 {
			return compared
		}
		return a.Prefix.Compare(b.Prefix)
	})
	pin := &routeViewCandidateSnapshot{id: rand.Text(), selector: selector, deadline: deadline, entries: make([]json.RawMessage, len(captured))}
	for i := range captured {
		if err := session.checkRequest(work); err != nil {
			return nil, routeViewStopped(err, session.owner)
		}
		encoded, err := json.Marshal(&captured[i])
		if err != nil {
			return nil, opererr.Wrap(err, "Candidate prefix could not be encoded", "No page from this scan was returned", "Resend route-view-candidates with snapshot omitted and cursor 0")
		}
		pin.entries[i] = encoded
		captured[i] = candidateEntry{}
	}
	if err := session.checkRequest(work); err != nil {
		return nil, routeViewStopped(err, session.owner)
	}
	return pin, nil
}
