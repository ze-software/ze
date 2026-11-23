// Design: docs/architecture/api/process-protocol.md -- accepted-origin route-only field.
package server

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// MUTATION: removing the OriginAdmissionID marker guard accepts the malformed
// mixed operation while the unchanged marker and accepted-route controls pass.
func TestAcceptedOriginIDCannotBecomeReplayMarker(t *testing.T) {
	for _, marker := range []string{"borr", "eorr"} {
		t.Run(marker, func(t *testing.T) {
			input := rpc.UpdateRouteInput{PeerMarker: marker, MarkerFamily: "ipv4/unicast", DestinationOwner: 17, DestinationSession: 19, DeadlineMS: 1}
			command, err := updateRouteDispatchCommand(input, "192.0.2.7")
			if err != nil {
				t.Fatalf("useful original marker refused: %v", err)
			}
			if command != "request peer 192.0.2.7 "+marker+" ipv4/unicast" {
				t.Fatalf("original marker changed: %q", command)
			}
			input.OriginAdmissionID = 23
			if command, err = updateRouteDispatchCommand(input, "192.0.2.7"); err == nil || command != "" {
				t.Fatalf("accepted-origin ID was discarded by marker: command=%q err=%v", command, err)
			}
		})
	}
	input := rpc.UpdateRouteInput{OriginAdmissionID: 23, Command: "update text origin incomplete nhop self nlri add 10.0.0.0/24"}
	command, err := updateRouteDispatchCommand(input, "192.0.2.7")
	if err != nil {
		t.Fatalf("useful accepted route refused: %v", err)
	}
	if command != "send bgp 192.0.2.7 "+input.Command {
		t.Fatalf("accepted route command changed: %q", command)
	}
}
