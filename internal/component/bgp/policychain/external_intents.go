// Design: docs/architecture/core-design.md -- The in-process BGP chain, named intents answered as filter-update sets
// Related: external.go -- applyExternalSet, which hands the as-path, prefix and family sets here
// Related: intents.go -- the Sets methods each intent calls, as an in-process member does
//
// An external filter answers the named intents Ze resolves with state the
// plugin lacks (D-5) as sets of the as-path, prefix and family fields. Each
// set calls the Sets method an in-process member calls, so the reactor builds
// the same UPDATE bytes from either. A prefix is named in the UPDATE event's
// NLRI shape and mapped here to its section and offset in the view the
// member read, the coordinates DropNLRI takes.

package policychain

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The remove-private modes an as-path remove names.
const (
	removePrivateStripValue  = "private"
	removePrivatePeerASValue = "private-peer-as"
)

// errPrefixNotAnnounced is answered when a prefix remove names no entry the
// route announces in the shape the session writes it.
var errPrefixNotAnnounced = errors.New("the route announces no NLRI with that prefix " +
	"(under ADD-PATH the value is {\"prefix\":\"<prefix>\",\"path-id\":<id>}, without it the prefix string)")

// intentTarget is what a reply's named intents resolve against: the view the
// member read, and the chain's prefix reader, so a prefix maps to the offsets
// of the sections as that member saw them.
type intentTarget struct {
	view   *policyview.View
	prefix policyview.Reader
	// importSide is set on an import chain, the only one a MULTI_EXIT_DISC
	// removal is honored on (errMEDRemoveOnExport).
	importSide bool
}

// applyASPathIntent answers an as-path set: prepend a count of local AS
// copies, or remove the private AS numbers.
func applyASPathIntent(sets *Sets, set *rpc.FilterSet) error {
	switch set.Action {
	case rpc.FilterSetActionPrepend:
		count, err := strconv.ParseUint(string(set.Value), 10, 8)
		if err != nil || count < 1 || count > ASPathPrependMax {
			return fmt.Errorf("the value is not a count from 1 to %d", ASPathPrependMax)
		}
		return sets.PrependASPath(uint8(count))
	case rpc.FilterSetActionRemove:
		mode, ok := plainString(set.Value)
		switch {
		case ok && mode == removePrivateStripValue:
			return sets.RemovePrivateAS(RemovePrivateStrip)
		case ok && mode == removePrivatePeerASValue:
			return sets.RemovePrivateAS(RemovePrivatePeerAS)
		}
		return errors.New("the value is \"private\" or \"private-peer-as\"")
	case rpc.FilterSetActionSet, rpc.FilterSetActionIncrement, rpc.FilterSetActionDecrement,
		rpc.FilterSetActionAdd:
	}
	return errors.New("the field takes prepend or remove")
}

// applyFamilyIntent answers a family remove: every section of the named
// family leaves the UPDATE.
func applyFamilyIntent(sets *Sets, set *rpc.FilterSet) error {
	if set.Action != rpc.FilterSetActionRemove {
		return errors.New("the field takes remove")
	}
	name, ok := plainString(set.Value)
	if !ok {
		return errors.New("the value is not a family name such as \"ipv6/unicast\"")
	}
	fam, ok := family.LookupFamily(name)
	if !ok {
		return fmt.Errorf("%q names no registered family", name)
	}
	return sets.StripFamily(fam)
}

// announcedPrefix is a prefix remove's value: the prefix, and the path
// identifier when the value carries one.
type announcedPrefix struct {
	prefix    netip.Prefix
	pathID    uint32
	hasPathID bool
	// labeled is set when the value carried a label stack, the shape the
	// event writes for a labeled-unicast NLRI; labels then holds each
	// three-octet label entry as it is on the wire, bottom of stack last.
	labeled bool
	labels  []uint32
}

// applyPrefixIntent answers a prefix remove: every entry of the body NLRI
// and of MP_REACH_NLRI that carries the prefix, in the view the member read,
// is dropped. A prefix the route does not announce fails the route rather
// than leave it unchanged in silence.
func applyPrefixIntent(sets *Sets, set *rpc.FilterSet, target intentTarget) error {
	if set.Action != rpc.FilterSetActionRemove {
		return errors.New("the field takes remove")
	}
	want, err := announcedPrefixValue(set.Value)
	if err != nil {
		return err
	}
	current, err := target.view.Read(target.prefix)
	if err != nil {
		return err
	}
	// RFC 4271 Section 4.3: the body NLRI carries IPv4 unicast only.
	dropped, err := dropAnnounced(sets, target.view, NLRISectionAnnounce, family.IPv4Unicast, current.Wire(), want)
	if err != nil {
		return err
	}
	if mp := current.MPReach(); mp != nil {
		more, err := dropAnnounced(sets, target.view, NLRISectionMPReach, mp.Family(), mp.NLRIBytes(), want)
		if err != nil {
			return err
		}
		dropped += more
	}
	if dropped == 0 {
		return errPrefixNotAnnounced
	}
	return nil
}

// dropAnnounced drops from section, of family fam, every entry carrying
// want, and answers how many it dropped. A unicast or multicast entry is
// named by the prefix shape, a labeled-unicast entry by the labeled shape,
// label stack included; every other family carries no entry in either.
func dropAnnounced(sets *Sets, view *policyview.View, section NLRISection, fam family.Family, wire []byte,
	want announcedPrefix,
) (int, error) {
	if len(wire) == 0 {
		return 0, nil
	}
	labeled := fam.SAFI == family.SAFIMPLSLabel
	if !labeled && fam.SAFI != family.SAFIUnicast && fam.SAFI != family.SAFIMulticast {
		return 0, nil
	}
	if labeled != want.labeled {
		return 0, nil
	}
	if (fam.AFI == family.AFIIPv4) != want.prefix.Addr().Is4() {
		return 0, nil
	}
	addPath, err := view.AddPath(fam)
	if err != nil {
		return 0, err
	}
	if addPath != want.hasPathID {
		return 0, nil
	}
	dropped := 0
	cursor := wireu.NewCIDRCursor(wire, addPath)
	for {
		entry, cidr, start, more := cursor.Next()
		if !more {
			break
		}
		if addPath && binary.BigEndian.Uint32(entry) != want.pathID {
			continue
		}
		bits, octets := int(cidr[0]), cidr[1:]
		if labeled {
			// RFC 8277 Section 2.3
			var framed bool
			if bits, octets, framed = nlrisplit.LabeledPrefix(cidr, false); !framed {
				return 0, fmt.Errorf("the %s NLRI label field does not frame", fam)
			}
			if !labelsCarry(cidr[1:len(cidr)-len(octets)], want.labels) {
				continue
			}
		}
		if !prefixCarries(bits, octets, want.prefix) {
			continue
		}
		if err := sets.DropNLRI(section, uint16(start)); err != nil { //nolint:gosec // G115: an offset in an UPDATE body, at most 65535 octets
			return 0, err
		}
		dropped++
	}
	if cursor.Bad() {
		return 0, fmt.Errorf("the %s NLRI does not frame", fam)
	}
	return dropped, nil
}

// labelsCarry reports whether the label field of one labeled NLRI entry,
// field, holds exactly the label entries want names, in order.
//
// RFC 8277 Section 2.3: "In all labels except the last (i.e., in all labels except the one
// immediately preceding the prefix), the S bit MUST be 0.  In the
// last label, the S bit MUST be 1."
//
//	+0 Label(20 bits) Rsrv(3 bits) S(1 bit), repeated, S set on the last
func labelsCarry(field []byte, want []uint32) bool {
	if len(field) != 3*len(want) {
		return false
	}
	for i, entry := range want {
		got := uint32(field[3*i])<<16 | uint32(field[3*i+1])<<8 | uint32(field[3*i+2])
		if got != entry {
			return false
		}
	}
	return true
}

// prefixCarries reports whether a prefix of bits bits, written in octets,
// is want. The ADD-PATH identifier, when the session carries one, is
// compared by the caller.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE message, the NLRI
// encoding MUST be extended by prepending the Path Identifier field,
// which is of four octets."
// RFC 4271 Section 4.3: "The Prefix field contains an IP address prefix, followed by
// the minimum number of trailing bits needed to make the end
// of the field fall on an octet boundary.  Note that the value
// of trailing bits is irrelevant."
//
//	+0 [Path Identifier(4), ADD-PATH only] Length(1) Prefix(ceil(Length/8))
func prefixCarries(bits int, prefix []byte, want netip.Prefix) bool {
	if bits != want.Bits() {
		return false
	}
	addr := want.Addr().As16()
	octets := addr[:]
	if want.Addr().Is4() {
		octets = octets[12:]
	}
	full, rest := bits/8, bits%8 // rest: the bits of the last, partial octet
	if len(prefix) < full+min(rest, 1) {
		return false
	}
	if !bytes.Equal(prefix[:full], octets[:full]) {
		return false
	}
	if rest == 0 {
		return true
	}
	mask := byte(0xff) << (8 - rest)
	return prefix[full]&mask == octets[full]
}

// announcedPrefixValue reads a prefix remove's value in the UPDATE event's
// NLRI shape: the prefix string, under ADD-PATH {"prefix": ..., "path-id":
// ...}, and for a labeled-unicast NLRI {"labels": [[label, entry], ...],
// "prefix": ...}. The prefix is canonical: a value with host bits set names
// no NLRI.
func announcedPrefixValue(value []byte) (announcedPrefix, error) {
	var out announcedPrefix
	text, ok := plainString(value)
	if !ok {
		var shaped struct {
			Labels *[][]uint32 `json:"labels"`
			Prefix *string     `json:"prefix"`
			PathID *uint32     `json:"path-id"`
		}
		decoder := json.NewDecoder(bytes.NewReader(value))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&shaped); err != nil {
			return out, errors.New("the value is not a prefix string, {\"prefix\":...,\"path-id\":...} " +
				"or {\"labels\":...,\"prefix\":...}")
		}
		if shaped.Prefix == nil {
			return out, errors.New("the value names no prefix")
		}
		text = *shaped.Prefix
		if shaped.Labels != nil {
			labels, err := labelEntries(*shaped.Labels)
			if err != nil {
				return out, err
			}
			out.labeled, out.labels = true, labels
		}
		if shaped.PathID != nil {
			out.pathID, out.hasPathID = *shaped.PathID, true
		}
		if !out.labeled && !out.hasPathID {
			return out, errors.New("the value names no path-id")
		}
	}
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return out, fmt.Errorf("%q is not a prefix", text)
	}
	if prefix != prefix.Masked() {
		return out, fmt.Errorf("%s has host bits set; the NLRI is %s", prefix, prefix.Masked())
	}
	out.prefix = prefix
	return out, nil
}

// labelEntries reads the label stack of the labeled shape, each member the
// pair the UPDATE event writes: the 20-bit label, then the three-octet entry
// it came from, which the event leaves out when the entry is zero. It answers
// the entries as they are on the wire.
func labelEntries(pairs [][]uint32) ([]uint32, error) {
	if len(pairs) == 0 {
		return nil, errors.New("the value names an empty label stack")
	}
	entries := make([]uint32, len(pairs))
	for i, pair := range pairs {
		if len(pair) == 1 && pair[0] == 0 {
			continue
		}
		if len(pair) != 2 {
			return nil, fmt.Errorf("label %d is not the [label, entry] pair the event writes", i+1)
		}
		if pair[1] >= 1<<24 {
			return nil, fmt.Errorf("label %d's entry %d is wider than three octets", i+1, pair[1])
		}
		if nlri.LabelValue(pair[1]) != pair[0] {
			return nil, fmt.Errorf("label %d is %d but its entry %d carries %d", i+1, pair[0], pair[1], nlri.LabelValue(pair[1]))
		}
		entries[i] = pair[1]
	}
	return entries, nil
}

// plainString answers the text of a plain JSON string value. An escape is no
// part of any community, mode, family or prefix spelling, so one is refused
// rather than decoded.
func plainString(value []byte) (string, bool) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' || bytes.IndexByte(value, '\\') >= 0 {
		return "", false
	}
	return string(value[1 : len(value)-1]), true
}
