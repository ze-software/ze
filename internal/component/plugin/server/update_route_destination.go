// Design: docs/architecture/api/process-protocol.md -- bound replay route and marker requests.
// Related: update_route_cancel.go -- one admitted cancellation boundary for both variants.
package server

import (
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// updateRouteDispatchCommand accepts one raw DTO variant before dispatch. The
// registered marker handler still owns capability checks and refresh grants.
func updateRouteDispatchCommand(input rpc.UpdateRouteInput, peer string) (string, error) {
	refuse := func() (string, error) {
		return "", opererr.New("Replay request has an invalid operation",
			"No route or marker was dispatched.",
			"Use a route command alone, or peer-marker borr/eorr with marker-family and both captured destination identifiers; omit route metadata for markers.")
	}
	if input.DestinationOwner != 0 {
		if input.DeadlineMS <= 0 {
			return refuse()
		}
	}
	if input.PeerMarker == "" {
		if input.MarkerFamily != "" {
			return refuse()
		}
		if input.Command == "" {
			return refuse()
		}
		var text textbuf.Buffer
		return text.Str("send bgp ").Str(peer).Byte(' ').Str(input.Command).String(), nil
	}
	if input.DestinationOwner == 0 {
		return refuse()
	}
	if input.DestinationSession == 0 {
		return refuse()
	}
	if input.Command != "" {
		return refuse()
	}
	if len(input.Meta) != 0 {
		return refuse()
	}
	if input.SourcePolicyEpoch != 0 {
		return refuse()
	}
	if input.OriginAdmissionID != 0 {
		return refuse()
	}
	if _, ok := family.LookupFamily(input.MarkerFamily); !ok {
		return refuse()
	}
	switch input.PeerMarker {
	case "borr", "eorr":
		var text textbuf.Buffer
		return text.Str("request peer ").Str(peer).Byte(' ').Str(input.PeerMarker).Byte(' ').Str(input.MarkerFamily).String(), nil
	default:
		// Unknown external marker spellings cannot turn into a command fragment.
		return refuse()
	}
}
