// Design: docs/architecture/web-interface.md -- names an operator gave to next-hop ranges
// Related: server.go -- LGServer holds the list behind an atomic pointer
// Related: graph_nexthop.go -- the next-hop graph nodes the names label
// Related: handler_ui.go -- the route rows the names label

package lg

import (
	"cmp"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// nextHopRange is one range of next-hop addresses and the name the operator
// gave it.
type nextHopRange struct {
	prefix netip.Prefix
	name   string
}

// nextHopRanges is the list of named next-hop ranges, most specific first.
// Only newNextHopRanges builds one, so every list holds valid, masked, distinct
// prefixes, a name on each, and at most zeconfig.LGNextHopRangesMax entries.
// It is never changed after it is built: a reload builds a new list and
// LGServer swaps the pointer, so a reader sees the old list or the new one and
// never a mix. Safe for concurrent use.
type nextHopRanges struct {
	list []nextHopRange
}

var (
	errNextHopRangesTooMany  = errors.New("lg server: more named next-hop ranges than the looking glass takes")
	errNextHopRangesPrefix   = errors.New("lg server: a named next-hop range is not a prefix with its host bits cleared")
	errNextHopRangesNameless = errors.New("lg server: a named next-hop range has no name")
)

// newNextHopRanges builds the list from ranges keyed by prefix. The map key
// makes two names for one range impossible to write. The checks below repeat
// what zeconfig.ExtractLGNextHopRanges guarantees, because this is the second
// code path the same properties are checked on: a caller that builds the map
// itself gets an error, never a list that names a next hop wrongly.
func newNextHopRanges(ranges map[netip.Prefix]string) (*nextHopRanges, error) {
	if len(ranges) > zeconfig.LGNextHopRangesMax {
		return nil, fmt.Errorf("%w: %d, at most %d", errNextHopRangesTooMany, len(ranges), zeconfig.LGNextHopRangesMax)
	}

	list := make([]nextHopRange, 0, len(ranges))
	for prefix, name := range ranges {
		if !prefix.IsValid() {
			return nil, fmt.Errorf("%w: %s", errNextHopRangesPrefix, prefix)
		}
		if prefix != prefix.Masked() {
			return nil, fmt.Errorf("%w: %s", errNextHopRangesPrefix, prefix)
		}
		// nameOf reads an IPv4-mapped next hop as its IPv4 address, so an
		// IPv4-mapped range would contain no next hop it is asked about.
		if prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("%w: %s is IPv4-mapped", errNextHopRangesPrefix, prefix)
		}
		// An empty name is what nameOf answers for a next hop in no range, so
		// a range must never carry one.
		if name == "" {
			return nil, fmt.Errorf("%w: %s", errNextHopRangesNameless, prefix)
		}
		list = append(list, nextHopRange{prefix: prefix, name: name})
	}

	// Longest prefix first, so the first range that contains an address is the
	// most specific one. Two ranges of one length that both contain an address
	// are the same range, which the map key excludes, so the order within one
	// length changes no answer; it is fixed anyway, by address, to keep the
	// list the same from one build to the next.
	slices.SortFunc(list, func(a, b nextHopRange) int {
		if c := cmp.Compare(b.prefix.Bits(), a.prefix.Bits()); c != 0 {
			return c
		}
		return a.prefix.Addr().Compare(b.prefix.Addr())
	})
	return &nextHopRanges{list: list}, nil
}

// nameOf answers the name of the most specific range that contains nextHop,
// and "" when no range contains it or nextHop is not one address. An empty
// answer means the page shows the address alone: Ze derives no name from DNS,
// from a peer or from the address. A nil list names nothing, which is the
// state of a server that was given no range.
//
// The scan is linear and bounded by zeconfig.LGNextHopRangesMax.
func (n *nextHopRanges) nameOf(nextHop string) string {
	if n == nil {
		return ""
	}
	address, err := netip.ParseAddr(nextHop)
	if err != nil {
		return ""
	}
	// An IPv4 next hop written as an IPv4-mapped IPv6 address is inside the
	// IPv4 range that holds it.
	address = address.Unmap()
	for _, r := range n.list {
		if r.prefix.Contains(address) {
			return r.name
		}
	}
	return ""
}

// UpdateNextHopRanges replaces the whole list of named next-hop ranges. Bound
// listeners and open connections are untouched: a page rendered after the call
// shows the new names, and one rendered before it showed the old ones.
//
// A list this server cannot use is refused, and the list installed before it
// keeps naming next hops.
//
// restore puts back the list this call replaced. The caller runs it when the
// reload that brought ranges is rolled back, so the page keeps the names of
// the configuration that stays in force. It is nil when err is not.
//
// Safe for concurrent use. restore MUST NOT run after a later
// UpdateNextHopRanges: it would put back a list two reloads old. Reloads run
// one at a time, and a reload calls restore before it returns.
func (s *LGServer) UpdateNextHopRanges(ranges map[netip.Prefix]string) (restore func(), err error) {
	replacement, err := newNextHopRanges(ranges)
	if err != nil {
		return nil, err
	}
	replaced := s.nextHopRanges.Swap(replacement)
	s.logger.Info("lg server next-hop range names replaced", "ranges", len(replacement.list))
	return func() {
		s.nextHopRanges.Store(replaced)
		s.logger.Info("lg server next-hop range names put back: the reload was rolled back")
	}, nil
}

// nextHopName answers the operator's name for a next hop, "" when its address
// is in no named range.
func (s *LGServer) nextHopName(nextHop string) string {
	return s.nextHopRanges.Load().nameOf(nextHop)
}

// routeRowsWithNextHopNames types the decoded RIB rows as routeRows does and adds, to
// each row, the name of the range its next hop is in.
func (s *LGServer) routeRowsWithNextHopNames(routes []any) []routeRow {
	rows := routeRows(routes)
	for i := range rows {
		rows[i].NextHopName = s.nextHopName(rows[i].NextHop)
	}
	return rows
}

// nameNextHopNodes gives each node of a next-hop graph its name: the name of
// the range its address is in, or, for an AS outside the local one, the AS
// name the AS path graph shows (decorateGraphNodes).
func (s *LGServer) nameNextHopNodes(g *nextHopGraph) {
	for i := range g.Nodes {
		if g.Nodes[i].Outside {
			g.Nodes[i].Name = s.resolveASN(textbuf.StringInt(int64(g.Nodes[i].AS)))
			continue
		}
		g.Nodes[i].Name = s.nextHopName(g.Nodes[i].Address)
	}
}

// validateNextHopRanges is the tree validator of the list. It refuses, at
// commit and wherever else a config is validated, a list the looking glass
// could not use: two entries for one range, too many entries, an entry with
// no name. The reader and the validator are one function, so they cannot
// disagree about which list is valid.
func validateNextHopRanges(tree *zeconfig.Tree) error {
	_, err := zeconfig.ExtractLGNextHopRanges(tree)
	return err
}
