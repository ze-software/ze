// Design: docs/research/vpp-deployment-reference.md -- Linux Control Plane pair readback
// Related: lcp.go -- creation consumes the same active global LCP settings
// Related: query.go -- resolved data-plane names and interface indices

package ifacevpp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/lcp"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

const lcpPairReadTimeout = 5 * time.Second

var _ iface.LCPPairReader = (*vppBackendImpl)(nil)

// The query uses its own context-bound stream rather than the shared channel:
// lcp_itf_pair_get_v2 ends with an explicit reply, not a control-ping marker.
var newLCPQueryStream = func(ctx context.Context) (api.Stream, error) {
	connector := vppcomp.GetActiveConnector()
	if connector == nil {
		return nil, fmt.Errorf("ifacevpp: LCP query: %w", iface.ErrBackendNotReady)
	}
	return connector.NewStream(ctx)
}

// GetLCPPair reads the actual pair for a resolved data-plane device. It never
// creates a pair or reconstructs a Linux name. A pair outside the configured
// global LCP namespace is refused: one selected backend has one discovery
// namespace. The caller MUST first resolve the native device through this
// backend's link read. Its native name map is then the identity source; this
// query never starts context-free lazy channel or name-map population. Calls
// require a nonnil context and are safe for concurrent use.
func (b *vppBackendImpl) GetLCPPair(ctx context.Context, name string) (iface.LCPPair, error) {
	if ctx == nil {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP query requires a context")
	}
	if err := ctx.Err(); err != nil {
		return iface.LCPPair{}, err
	}
	settings, available := getActiveLCPSettings()
	if !available {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP settings are unavailable; start the configured VPP component")
	}
	if !settings.Enabled {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP is disabled; set vpp lcp enabled true and restart Ze")
	}
	bounded, cancel := context.WithTimeout(ctx, lcpPairReadTimeout)
	defer cancel()
	nativeIndex, resolved := b.names.lookupNativeIndex(name)
	if !resolved {
		return iface.LCPPair{}, fmt.Errorf("ifacevpp: LCP device %q has no resolved native identity; resolve the interface before reading its pair", name)
	}
	index := interface_types.InterfaceIndex(nativeIndex)
	if index == 0 {
		return iface.LCPPair{}, errors.New("ifacevpp: local0 is not an LCP access interface")
	}
	if index == ^interface_types.InterfaceIndex(0) {
		return iface.LCPPair{}, errors.New("ifacevpp: invalid LCP data-plane interface index")
	}
	return readLCPPair(bounded, index, settings.Netns, newLCPQueryStream)
}

func readLCPPair(ctx context.Context, index interface_types.InterfaceIndex, namespace string,
	open func(context.Context) (api.Stream, error)) (pair iface.LCPPair, err error) {
	stream, err := open(ctx)
	if err != nil {
		return iface.LCPPair{}, err
	}
	if stream == nil {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP query returned no stream")
	}
	defer func() {
		err = errors.Join(err, stream.Close())
		if err != nil {
			pair = iface.LCPPair{}
		}
	}()
	if err := stream.SendMsg(&lcp.LcpItfPairGetV2{SwIfIndex: index}); err != nil {
		return iface.LCPPair{}, fmt.Errorf("ifacevpp: send LCP pair query: %w", err)
	}
	var details *lcp.LcpItfPairDetails
	// The targeted C API branch sends exactly one details and one final reply.
	// A second details is a malformed answer, never an invitation to drain an
	// unbounded dump (third_party/vpp-linux-cp/src/lcp_api.c).
	for range 2 {
		message, err := stream.RecvMsg()
		if err != nil {
			return iface.LCPPair{}, fmt.Errorf("ifacevpp: receive LCP pair query: %w", err)
		}
		switch reply := message.(type) {
		case *lcp.LcpItfPairDetails:
			if reply == nil {
				return iface.LCPPair{}, errors.New("ifacevpp: LCP query returned nil pair details")
			}
			if details != nil {
				return iface.LCPPair{}, errors.New("ifacevpp: targeted LCP query returned several pairs")
			}
			// Keep a snapshot: a stream implementation may reuse its message.
			copied := *reply
			details = &copied
		case *lcp.LcpItfPairGetV2Reply:
			if reply == nil {
				return iface.LCPPair{}, errors.New("ifacevpp: LCP query returned nil completion")
			}
			if reply.Retval != 0 {
				return iface.LCPPair{}, fmt.Errorf("ifacevpp: LCP pair query: %w", api.RetvalToVPPApiError(reply.Retval))
			}
			if reply.Cursor != ^uint32(0) {
				return iface.LCPPair{}, errors.New("ifacevpp: targeted LCP query did not complete")
			}
			if details == nil {
				return iface.LCPPair{}, errors.New("ifacevpp: LCP query completed without a pair")
			}
			if err := ctx.Err(); err != nil {
				return iface.LCPPair{}, err
			}
			return lcpPairFromDetails(details, index, namespace)
		default: // The VPP message set is open; an unexpected reply cannot identify a pair.
			return iface.LCPPair{}, fmt.Errorf("ifacevpp: unexpected LCP query reply %T", message)
		}
	}
	return iface.LCPPair{}, errors.New("ifacevpp: LCP query returned no final reply")
}

func lcpPairFromDetails(details *lcp.LcpItfPairDetails, index interface_types.InterfaceIndex,
	namespace string) (iface.LCPPair, error) {
	if details.PhySwIfIndex != index {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP pair names another data-plane interface")
	}
	if details.HostSwIfIndex == 0 {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP control-plane index names local0")
	}
	if details.HostSwIfIndex == ^interface_types.InterfaceIndex(0) {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP pair has an invalid control-plane index")
	}
	if details.HostSwIfIndex == index {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP control-plane and data-plane indices are identical")
	}
	if details.VifIndex == 0 {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP pair has no Linux interface index")
	}
	if details.VifIndex > math.MaxInt32 {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP Linux interface index exceeds the kernel range")
	}
	if details.HostIfType != lcp.LCP_API_ITF_HOST_TAP {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP pair is not an Ethernet TAP")
	}
	device := trimCString(details.HostIfName)
	if device == "" {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP pair has no Linux device name")
	}
	if len(device) > lcpMaxHostName {
		return iface.LCPPair{}, errors.New("ifacevpp: LCP pair Linux device exceeds IFNAMSIZ")
	}
	actualNamespace := trimCString(details.Netns)
	if actualNamespace != namespace {
		return iface.LCPPair{}, fmt.Errorf("ifacevpp: LCP pair namespace %q differs from configured global namespace %q", actualNamespace, namespace)
	}
	return iface.LCPPair{DataplaneIndex: uint32(index), ControlPlaneIndex: uint32(details.HostSwIfIndex),
		LinuxIndex: int(details.VifIndex), ControlDevice: device, Namespace: actualNamespace}, nil
}
