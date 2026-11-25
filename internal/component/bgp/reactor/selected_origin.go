// Design: docs/architecture/update-building.md -- selected local output from issued data.
// Related: origin_build.go -- immutable original input; origin_write.go -- final-use admission.
// RFC 4271 Section 9.1.3 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"context"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectOrigin borrows only the issuer's immutable build data. StoredRoute is
// a legacy shape echo on this variant and MUST NOT choose attributes, next hop,
// framing or a received source. The caller MUST keep the selected stream lock
// through final settlement; this call MUST complete its issued write on every
// return. No candidate, history, forwarding queue or received lease is created.
func (a *reactorAPIAdapter) selectOrigin(ctx context.Context, peer *Peer, work *selectedWrite, fam family.Family, key fwdPathKey) (rpc.SelectedRouteResult, error) {
	write, err := a.r.originReceipts.admit(work.route.OriginReceipt, work.output.owner, peer, work.snapshot, key)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	defer write.complete()
	value := write.build()
	if value.family != fam {
		return rpc.SelectedRouteResult{}, errAdjOutProvenance
	}
	if !bytes.Equal(value.native, work.route.KeyNLRI) {
		return rpc.SelectedRouteResult{}, opererr.New("The selected local route differs from its original admission.", "The original native path must be selected before it can be advertised.", "ze show bgp rib")
	}
	kind, err := work.snapshot.ExportKind(work.output.owner, false)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	work.output.kind = kind
	work.origin = write
	ctx, release := write.writeContext(ctx)
	defer release()
	nc, err := peer.sessionNegotiated(work.session)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	if !nc.Has(fam) {
		return a.writeSelected(ctx, peer, work, nil)
	}
	nextHop, err := peer.resolveNextHop(work.session, value.nextHop, fam)
	if err != nil {
		// Families without a forwarding hop permit an explicit absent policy.
		// The same local batch rail ignores that unused resolution answer.
		if fam.NeedsNextHop() {
			return rpc.SelectedRouteResult{}, err
		}
	}
	batch := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{value.originIdentity}, NextHop: value.nextHop, OriginAS: value.originAS}
	if value.attributesExplicit {
		batch.Wire = value.wire
	}
	facts := announceFactsFor(peer, fam, nextHop, peer.IsIBGP(), nc)
	attrBuffer, nlriBuffer := getBuildBuf(), getBuildBuf()
	defer putBuildBuf(attrBuffer)
	defer putBuildBuf(nlriBuffer)
	update, err := a.buildBatchAnnounceUpdate(attrBuffer.Buf, nlriBuffer.Buf, batch, facts)
	if err != nil {
		return rpc.SelectedRouteResult{}, err
	}
	if update == nil {
		return rpc.SelectedRouteResult{}, errAnnounceTooLarge
	}
	// The two pool buffers remain owned until writeSelected's actual flush
	// joins. The local gate applies export policy once, after local building;
	// passing this destination-shaped result through receive forwarding would
	// prepend again and invent a received source for an originated route.
	items := [1]fwdItem{{peer: peer, session: work.session, authority: adjOutSelected, selected: work,
		originated: true, updates: []*message.Update{update}}}
	return a.writeSelected(ctx, peer, work, items[:])
}

// writeContext cancels and joins the existing request writer when its exact
// issued record is revoked. The caller MUST release this context after final
// flush joins and before completing the write; no later socket inherits it.
func (write originWrite) writeContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(write.record.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

// originIdentity is already validated and owns native payload bytes. These
// methods adapt that SAME identity to the shared local batch builder without
// allocating a prefixed byte copy or discarding legal Path Identifier zero.
// Bytes borrows immutable storage; callers MUST NOT mutate the returned slice.
func (identity *originIdentity) Family() family.Family { return identity.family }
func (identity *originIdentity) Bytes() []byte         { return identity.native }
func (identity *originIdentity) Len() int              { return len(identity.native) }
func (identity *originIdentity) PathID() uint32        { return identity.pathID }
func (identity *originIdentity) HasAddPath() bool      { return identity.hasPathID }
func (identity *originIdentity) SupportsAddPath() bool { return true }
func (identity *originIdentity) WriteTo(buffer []byte, off int) int {
	return copy(buffer[off:], identity.native)
}
func (identity *originIdentity) String() string {
	var text textbuf.Buffer
	return text.Reset().Str("origin[").Str(identity.family.String()).Byte(']').String()
}
