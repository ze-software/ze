// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route

package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestDecodePcapPathConfinedOnDaemonRoute proves `show bgp decode pcap <file>`
// reads its capture through command.CallerPath on the daemon route.
//
// Goal: a capture named outside the configuration directories is refused on
// the daemon route with the local command to run, before any byte is read.
// Method: decodePcapInput on RouteRemote and RouteUnspecified with
// `/etc/passwd`; this package registers no configuration directory, so no
// file is admitted, and the refusal must come from the confinement rather than
// from a pcap parse of the file.
func TestDecodePcapPathConfinedOnDaemonRoute(t *testing.T) {
	for _, route := range []command.Route{command.RouteRemote, command.RouteUnspecified} {
		answer := decodePcapInput([]string{"/etc/passwd"}, &decodeRequest{}, route)
		if answer.Code == 0 || answer.Payload != nil {
			t.Fatalf("route %d: the daemon route decoded /etc/passwd: code %d", route, answer.Code)
		}
		want := "run 'ze show bgp decode pcap /etc/passwd' on the host that holds the file"
		if !strings.Contains(answer.Diagnostic, want) {
			t.Errorf("route %d: refusal %q does not name %q", route, answer.Diagnostic, want)
		}
	}
}
