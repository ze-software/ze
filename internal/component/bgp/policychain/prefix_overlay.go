// Design: docs/architecture/core-design.md — the in-process BGP chain, read-after-set
// Related: chain.go — Chain.Run calls overlayPrefix after a passing member
// Related: intents.go — Sets.DropNLRI, the drops this overlay answers
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI value layout

package policychain

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/family"
)

// errStrayDrop is wrapped when a member drops at an offset that starts no NLRI
// entry of the section the view answers.
var errStrayDrop = errors.New("drops at an offset that starts no NLRI entry")

// prefixOverlay is the RunSets scratch of the prefix overlay. Each section
// keeps two buffers that alternate, because each overlay is built from the
// one before it; each is bounded by the section, so by the UPDATE body, and
// keeps its storage across runs.
type prefixOverlay struct {
	body     [2][]byte
	mpReach  [2][]byte
	bodyNext uint8
	mpNext   uint8
	// received is prefix as the UPDATE carries it, taken at the run's first
	// overlay, when the view still answers the wire. A later member's drops
	// are translated against it.
	received    policyview.Value
	receivedSet bool
}

// overlayPrefix overlays the view's prefix with the NLRI the member's drops
// leave, so a later member reads the subset (read-after-set), as a later
// text filter reads the NLRI an earlier text filter's delta kept. A member's
// offsets name entries of the section as the view answered it; once the
// section is overlaid that is the subset, so they are rewritten to the
// entry's offset in the received section, which is where the folded drops
// and the reactor's rewrite (libraryNLRIOps) count. A family the member
// strips leaves prefix with its section, as the reactor's rewrite removes it
// whatever the drops name, so a later member reads none of its NLRI, as a
// later text filter reads none after an earlier delta removed them. It runs
// only for a member that dropped from the body NLRI or MP_REACH_NLRI, the two
// sections prefix reads, or that stripped a family, so a chain that does
// neither reads the wire and allocates nothing.
func (c *Chain) overlayPrefix(view *policyview.View, sets *RunSets) error {
	member := &sets.member
	strips := member.written&setKindFamily.bit() != 0
	if member.written&setKindPrefix.bit() == 0 && !strips {
		return nil
	}
	announce := member.NLRIDrops(NLRISectionAnnounce)
	mpReach := member.NLRIDrops(NLRISectionMPReach)
	if len(announce) == 0 && len(mpReach) == 0 && !strips {
		return nil // withdrawn drops: prefix reads no withdrawn section
	}
	current, err := view.Read(c.prefix)
	if err != nil {
		return fmt.Errorf("prefix: %w", err)
	}
	overlay := &sets.prefix
	if !overlay.receivedSet {
		overlay.received, overlay.receivedSet = current, true
	}
	body := current.Wire()
	switch {
	case member.StripsFamily(family.IPv4Unicast):
		// RFC 4271 Section 4.3: the body NLRI field carries IPv4 unicast only.
		body = nil
	case len(announce) > 0:
		if body, err = c.dropBody(view, sets, body, announce); err != nil {
			return err
		}
	}
	mp := current.MPReach()
	switch {
	case mp != nil && member.StripsFamily(mp.Family()):
		// RFC 4760 Section 2: the family leaves the UPDATE with its section.
		mp = nil
	case len(mpReach) > 0:
		if mp, err = c.dropMPReach(view, sets, mp, mpReach); err != nil {
			return err
		}
	}
	if err := view.SetPrefix(c.prefix, body, mp); err != nil {
		return fmt.Errorf("prefix: %w", err)
	}
	return nil
}

// dropBody answers the body NLRI less the member's drops, and rewrites the
// drops to received offsets.
func (c *Chain) dropBody(view *policyview.View, sets *RunSets, body []byte, drops []uint16) ([]byte, error) {
	// RFC 4271 Section 4.3: the body NLRI field carries IPv4 unicast only.
	addPath, err := view.AddPath(family.IPv4Unicast)
	if err != nil {
		return nil, fmt.Errorf("prefix: %w", err)
	}
	overlay := &sets.prefix
	buf := &overlay.body[overlay.bodyNext]
	kept, ok := wireu.DropCIDREntries((*buf)[:0], body, addPath, drops)
	if !ok {
		return nil, fmt.Errorf("prefix: body NLRI %w", errStrayDrop)
	}
	overlay.bodyNext ^= 1
	*buf = kept
	receivedDrops(overlay.received.Wire(), addPath, sets.folded.NLRIDrops(NLRISectionAnnounce), drops)
	return kept, nil
}

// dropMPReach answers the MP_REACH_NLRI value less the member's drops (nil
// when none is left), and rewrites the drops to received offsets.
func (c *Chain) dropMPReach(view *policyview.View, sets *RunSets, mp wireu.MPReachWire, drops []uint16) (wireu.MPReachWire, error) {
	if mp == nil {
		return nil, errors.New("prefix: drops from an MP_REACH_NLRI the UPDATE does not carry")
	}
	fam := mp.Family()
	if !filtertext.CIDRFamily(fam) {
		return nil, fmt.Errorf("prefix: drops from %s, whose NLRI is not a CIDR prefix", fam)
	}
	addPath, err := view.AddPath(fam)
	if err != nil {
		return nil, fmt.Errorf("prefix: %w", err)
	}
	section := mp.NLRIBytes()
	// RFC 4760 Section 3: AFI(2) SAFI(1) NHLen(1) NextHop Reserved(1), then the
	// NLRI, so the overlay keeps the head and rewrites what follows.
	head := mp[:len(mp)-len(section)]
	overlay := &sets.prefix
	buf := &overlay.mpReach[overlay.mpNext]
	out := append((*buf)[:0], head...)
	out, ok := wireu.DropCIDREntries(out, section, addPath, drops)
	if !ok {
		return nil, fmt.Errorf("prefix: MP_REACH_NLRI %w", errStrayDrop)
	}
	overlay.mpNext ^= 1
	*buf = out
	receivedDrops(overlay.received.MPReach().NLRIBytes(), addPath, sets.folded.NLRIDrops(NLRISectionMPReach), drops)
	if len(out) == len(head) {
		return nil, nil // every NLRI dropped: the reactor drops the attribute
	}
	return out, nil
}

// receivedDrops rewrites drops, ascending offsets of the section the view
// answers (received less folded), to the offsets of the same entries in the
// received section. The view answers the received section until the chain
// folds a drop of it, so with no folded drop the offsets already are received
// ones. drops MUST already have been checked against the view's section.
func receivedDrops(received []byte, addPath bool, folded, drops []uint16) {
	if len(folded) == 0 {
		return
	}
	cursor := wireu.NewCIDRCursor(received, addPath)
	viewOffset, i := 0, 0
	for i < len(drops) {
		entry, _, start, more := cursor.Next()
		if !more {
			return
		}
		if len(folded) > 0 && int(folded[0]) == start {
			folded = folded[1:]
			continue // an earlier member dropped it: the view does not carry it
		}
		if int(drops[i]) == viewOffset {
			drops[i] = uint16(start) //nolint:gosec // an offset in an UPDATE body, at most 65535 octets
			i++
		}
		viewOffset += len(entry)
	}
}
