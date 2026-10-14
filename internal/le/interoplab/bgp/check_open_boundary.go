// Design: docs/architecture/testing/interop.md -- the bgp-open-extended-params-boundary-frr scenario.
// Related: register_open_boundary.go -- specialCheckers registry entry.
// Related: ../../../core/bgp/capability/optional_params.go -- the one Optional Parameters packer under test.
// RFC: rfc/short/rfc9072.md -- Section 2, the extended Optional Parameters encoding.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const scenarioOpenExtendedParamsBoundaryFRR = "bgp-open-extended-params-boundary-frr"

// frrSessionStateKey is the field FRR's `show bgp neighbor <ip> json` writes the
// session state under (bgpd/bgp_vty.c).
const frrSessionStateKey = "bgpState"

// boundaryHost is the hostname the scenario's ze.conf configures: 200 octets,
// the padding that brings ze's capabilities to 254 octets. Its length is part
// of the arithmetic in that file's header, so a change to one changes both.
var boundaryHost = "zeboundary" + strings.Repeat("h", 190)

// checkOpenExtendedParamsBoundaryFRR requires FRR to reach Established with a
// ze whose capabilities total 254 octets and to report ze's hostname, which
// sits in the same type-2 parameter as every other capability. RFC 9072
// Section 2: "However, if the length of the Optional Parameters in the BGP
// OPEN message does exceed 255, the OPEN message MUST be encoded according to
// the procedure below." At 254 the field is 256 octets, so a ze that framed
// the parameter with a one-octet length inside the extended envelope sends an
// OPEN FRR cannot parse, and the session never comes up.
func checkOpenExtendedParamsBoundaryFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("extended-params boundary scenario has no selected IPv4 network")
	}
	zeAddress := networkHostAddress(check.Network, 2)
	var command textbuf.Buffer
	session := []string{cmdVtysh, "-c", command.Str("show bgp neighbor ").Str(zeAddress).Str(" json").String()}
	if err := waitJSONFields(ctx, check.Lab, peerFRR, session, 90*time.Second,
		map[string]string{frrSessionStateKey: stateEstablished}, nil); err != nil {
		return fmt.Errorf("RFC 9072 boundary: FRR never reached Established with ze at %s, whose OPEN carries "+
			"254 octets of capabilities in the extended form: %w", zeAddress, err)
	}
	answer, err := check.Lab.Query(ctx, peerFRR, session, nil)
	if err != nil {
		return fmt.Errorf("RFC 9072 boundary: read FRR neighbor %s: %w", zeAddress, err)
	}
	hostname, ok := frrReceivedHostname(answer)
	if !ok {
		return fmt.Errorf("RFC 9072 boundary: FRR reached Established but reports no hostname from ze at %s, "+
			"so the capability parameter was not read as ze framed it:\n%s", zeAddress, strings.TrimSpace(answer))
	}
	// FRR keeps at most BGP_MAX_HOSTNAME (64) octets of a received hostname
	// (bgpd/bgp_open.c, bgp_capability_hostname), so it holds a prefix of ours.
	if !strings.HasPrefix(boundaryHost, hostname) {
		return fmt.Errorf("RFC 9072 boundary: FRR reports ze's hostname as %q, which is not a prefix of the "+
			"%d-octet hostname ze sent", hostname, len(boundaryHost))
	}
	return nil
}

// frrReceivedHostname reads the hostname FRR received from ze out of
// `show bgp neighbor <ip> json`, and reports false when FRR holds none.
func frrReceivedHostname(output string) (string, bool) {
	var document any
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return "", false
	}
	value, ok := findJSONField(document, "hostname")
	if !ok {
		return "", false
	}
	hostname, ok := value.(string)
	if !ok {
		return "", false
	}
	return hostname, hostname != ""
}
