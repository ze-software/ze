// Design: docs/architecture/plugin/rib-storage-design.md -- labeled unicast selection keys.
// Related: rib_selected_dispatch.go -- selectedDispatchKey strips RFC 8277 label stacks.
package rib

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedDispatchKeyLabeledUnicast verifies that selectedDispatchKey strips
// RFC 8277 label stacks for ipv4/mpls-label and ipv6/mpls-label native
// advertisement NLRIs while rejecting malformed or unlabeled keys.
// Method: table-driven calls to selectedDispatchKey covering single-label,
// multi-label, default-prefix, IPv6, and malformed negative-space inputs.
func TestSelectedDispatchKeyLabeledUnicast(t *testing.T) {
	v4Labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	v6Labeled := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}

	validCases := []struct {
		name   string
		family string
		raw    []byte
		want   advertiseKey
	}{
		{
			name:   "ipv4 single label 800001 for 198.51.100.100/32",
			family: "ipv4/mpls-label",
			raw:    []byte{56, 0xc3, 0x50, 0x11, 198, 51, 100, 100},
			want:   advertiseKey{family: v4Labeled, prefix: netip.MustParsePrefix("198.51.100.100/32")},
		},
		{
			name:   "ipv4 multi-label stack for 198.51.100.100/32",
			family: "ipv4/mpls-label",
			raw:    []byte{80, 0x00, 0x06, 0x40, 0xc3, 0x50, 0x11, 198, 51, 100, 100},
			want:   advertiseKey{family: v4Labeled, prefix: netip.MustParsePrefix("198.51.100.100/32")},
		},
		{
			name:   "ipv4 default prefix with label",
			family: "ipv4/mpls-label",
			raw:    []byte{24, 0xc3, 0x50, 0x11},
			want:   advertiseKey{family: v4Labeled, prefix: netip.MustParsePrefix("0.0.0.0/0")},
		},
		{
			name:   "ipv6 single label for 2001:db8::/32",
			family: "ipv6/mpls-label",
			raw:    []byte{56, 0xc3, 0x50, 0x11, 0x20, 0x01, 0x0d, 0xb8},
			want:   advertiseKey{family: v6Labeled, prefix: netip.MustParsePrefix("2001:db8::/32")},
		},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectedDispatchKey(tc.family, tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}

	invalidCases := []struct {
		name   string
		family string
		raw    []byte
	}{
		{
			name:   "bare ipv4 prefix without label stack",
			family: "ipv4/mpls-label",
			raw:    []byte{32, 198, 51, 100, 100},
		},
		{
			name:   "truncated label stack",
			family: "ipv4/mpls-label",
			raw:    []byte{24, 0xc3, 0x50},
		},
		{
			name:   "missing bottom-of-stack bit",
			family: "ipv4/mpls-label",
			raw:    []byte{56, 0xc3, 0x50, 0x10, 198, 51, 100, 100},
		},
		{
			name:   "ipv4 prefix exceeds 32 bits after label",
			family: "ipv4/mpls-label",
			raw:    []byte{57, 0xc3, 0x50, 0x11, 198, 51, 100, 100, 0x80},
		},
		{
			name:   "trailing octet past declared bit length",
			family: "ipv4/mpls-label",
			raw:    []byte{56, 0xc3, 0x50, 0x11, 198, 51, 100, 100, 0xff},
		},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := selectedDispatchKey(tc.family, tc.raw)
			require.Error(t, err)
		})
	}
}

// TestRIBOriginLabeledUnicastAnnounceReplaceWithdraw proves that an originated
// ipv4/mpls-label route is admitted, queried via CurrentSelection, replaced on
// label change under the same prefix key (RFC 8277 Section 2.5), and withdrawn.
// Method: drives selectedDispatch.originIntake and currentSelection with native
// RFC 8277 advertisement framing matching ExaBGP api-route-forms-v4.
func TestRIBOriginLabeledUnicastAnnounceReplaceWithdraw(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("127.0.0.1")
	writes := make(chan rpc.SelectRoutesInput, 8)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.destinationOwnerUp(peer, 340, 440))
	require.NoError(t, d.barrier(ctx))

	label800001 := []byte{56, 0xc3, 0x50, 0x11, 198, 51, 100, 100}
	first := originIntakeFixture(peer, 340, 940, 100)
	first.Route.Family = "ipv4/mpls-label"
	first.Route.KeyNLRI = label800001
	first.Route.SourceAddPath = false
	first.Route.SourcePathID = 0
	first.Route.NextHopPolicy = rpc.OriginNextHopExplicit
	first.Route.NextHop = "1.1.1.1"

	out, err := d.originIntake(ctx, first)
	require.NoError(t, err)
	require.Equal(t, uint32(1), out.Announced)
	require.Len(t, out.Admissions, 1)
	require.Equal(t, label800001, out.Admissions[0].KeyNLRI)

	dispatched := selectedDispatchAwait(t, ctx, writes)
	require.Len(t, dispatched.Routes, 1)
	require.Equal(t, rpc.SelectedRouteOrigin, dispatched.Routes[0].Kind)
	require.Equal(t, "ipv4/mpls-label", dispatched.Routes[0].Family)
	require.Equal(t, label800001, dispatched.Routes[0].KeyNLRI)

	query := rpc.CurrentSelectionInput{
		Destination:        peer.String(),
		DestinationSession: 440,
		Keys: []rpc.CurrentSelectionKey{{
			Family:   "ipv4/mpls-label",
			KeyNLRI:  label800001,
			Revision: dispatched.Routes[0].Revision,
		}},
	}
	answer, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Len(t, answer.Results, 1)
	require.Equal(t, rpc.CurrentSelectionPresent, answer.Results[0].Presence)
	require.Equal(t, dispatched.Routes[0].Revision, answer.Results[0].CurrentRevision)

	// RFC 8277 Section 2.5: replacing the label for the same prefix updates
	// the existing selection key rather than creating a second prefix entry.
	label800002 := []byte{56, 0xc3, 0x50, 0x21, 198, 51, 100, 100}
	replacement := rpc.CloneOriginIntake(first)
	replacement.OriginReceipt = 941
	replacement.Route.KeyNLRI = label800002

	out, err = d.originIntake(ctx, replacement)
	require.NoError(t, err)
	require.Equal(t, uint32(1), out.Announced)
	replaced := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, label800002, replaced.Routes[0].KeyNLRI)
	require.Greater(t, replaced.Routes[0].Revision, dispatched.Routes[0].Revision)

	d.mu.Lock()
	require.Len(t, d.origins, 1, "label replacement must share the same prefix key")
	d.mu.Unlock()

	withdraw := rpc.CloneOriginIntake(replacement)
	withdraw.Operation = rpc.OriginIntakeWithdraw
	out, err = d.originIntake(ctx, withdraw)
	require.NoError(t, err)
	require.Equal(t, uint32(1), out.Withdrawn)
	absent := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, rpc.SelectedRouteAbsent, absent.Routes[0].Kind)

	query.Keys[0].KeyNLRI = label800002
	query.Keys[0].Revision = absent.Routes[0].Revision
	answer, err = d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Equal(t, rpc.CurrentSelectionAbsent, answer.Results[0].Presence)
}
