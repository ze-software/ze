// Design: docs/guide/redistribution.md -- the ebgp and ibgp sources
// Related: events.go -- RouteChangeEntry.BGPClass, the field that carries it

package redistevents

// BGPClass is the class of a BGP best path: learned from an external peer or
// from an internal one (RFC 4271 Section 1.1, "External BGP (eBGP)" and
// "Internal BGP (iBGP)"). The BGP producer stamps it on every entry it emits,
// because a batch names one protocol (bgp) while each best path in it has its
// own class. The redistribute sources `ebgp` and `ibgp` and the route-policy
// `protocol` condition read it.
//
// The zero value is BGPClassUnspecified: the entry is not a BGP best path, or
// its peer class is not known (a locally originated route). It never matches
// `ebgp` or `ibgp`, so an unknown class moves through `import bgp` only.
type BGPClass uint8

const (
	BGPClassUnspecified BGPClass = 0
	BGPClassExternal    BGPClass = 1
	BGPClassInternal    BGPClass = 2
)

// The source names of the two classes. They are declared here once: the BGP
// producer registers its redistribute sources under them, and the route-policy
// `protocol` condition accepts them.
const (
	SourceNameEBGP = "ebgp"
	SourceNameIBGP = "ibgp"
)

// SourceName answers the redistribute source name of the class, and false for
// BGPClassUnspecified, which names no source finer than the batch protocol.
func (c BGPClass) SourceName() (string, bool) {
	switch c {
	case BGPClassExternal:
		return SourceNameEBGP, true
	case BGPClassInternal:
		return SourceNameIBGP, true
	case BGPClassUnspecified:
		return "", false
	}
	panic("BUG: redistevents: BGPClass out of range")
}
