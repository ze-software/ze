// Design: docs/architecture/mpls/mpls-kernel.md -- private MPLS push contexts.
package fibkernel

import (
	"errors"
	"net/netip"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

// RSVP bypass tunnel IDs occupy the low 16 bits. The owner-level unreachable
// rule covers this namespace even after its last private route is withdrawn.
const mplsContextMark = uint32(0x5a000000)
const mplsContextMask = uint32(0xffff0000)

// mplsContextBackend acknowledges a push only after both route and selector are
// usable. Close removes each private context, but fixed namespace guards remain
// fail-closed across concurrent producer shutdown and owner restart.
type mplsContextBackend interface {
	addMPLSContext(RichRoute) error
	delMPLSContext(netip.Prefix, uint32) error
	mplsContextCount() int
}

func (f *fibKernel) addMPLSPushLocked(e *mplsfibevents.Entry, rb richRouteBackend) error {
	route := RichRoute{Prefix: e.FEC, NextHop: e.NextHop, Labels: e.OutLabels, TableID: e.TableID, PathMTU: e.PathMTU}
	if e.TableID != 0 {
		backend, ok := f.backend.(mplsContextBackend)
		if !ok {
			return errors.New("mpls-fib: backend cannot select private push contexts")
		}
		return backend.addMPLSContext(route)
	}

	// The ordinary FIB is shared with other writers. Add refuses a foreign
	// prefix; Replace is reserved for a push this owner already installed.
	key := route.Prefix.String()
	var err error
	if f.mplsInstalled[key] {
		err = rb.replaceRichRoute(route)
	} else {
		err = rb.addRichRoute(route)
	}
	if err == nil {
		f.mplsInstalled[key] = true
	}
	return err
}
