// Design: docs/architecture/testing/interop.md -- the frr-software-version scenario.
// Related: check_special.go -- specialCheckers registry entry.
// Related: ../../../component/bgp/plugins/softver/softver.go -- the encoder under test.
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

// zeSoftwareVersion is the version string ze advertises, a copy of
// softver.ZeVersion. TestZeSoftwareVersionMatchesProducer compares the two.
const zeSoftwareVersion = "Ze/0.1.0"

// frrReceivedVersionKey is the field FRR's `show bgp neighbor <ip> json`
// writes the peer's Software Version capability under (bgpd/bgp_vty.c).
const frrReceivedVersionKey = "receivedSoftwareVersion"

// checkSoftwareVersionFRR drives the owner decision D-13 against FRR 10.3.1.
// ze's peer legacy sends the length-prefixed form over IPv4 and ze's peer
// draft the draft's bare form over IPv6, so one ze and one FRR hold both
// sessions. Both are configured from startup: a plugin declares its
// capabilities once, at stage 3, so a reload cannot switch the form a running
// ze sends. A control run with peer draft set to `encoding legacy` reaches
// Established, so the encoding is the only thing that separates the two peers.
func checkSoftwareVersionFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("software-version scenario has no selected IPv4 network")
	}
	if !check.Network.IPv6.IsValid() {
		return errors.New("software-version scenario has no selected IPv6 network")
	}
	legacyAddress := networkHostAddress(check.Network, 2)
	if err := requireFRRShowsZeVersion(ctx, check.Lab, legacyAddress, "legacy form"); err != nil {
		return err
	}
	return requireFRRRefusesDraftForm(ctx, check.Lab, networkHostAddress6(check.Network, 3), networkHostAddress6(check.Network, 2))
}

// FRR's answer to the draft form, RFC 4271 Section 4.5: error code 2 is
// "OPEN Message Error", and subcode 0 is the unspecific subcode FRR sends when
// a capability fails to parse (bgpd/bgp_open.c, bgp_capability_parse).
const (
	notifyOpenMessageError = 2
	notifyUnspecific       = 0
)

// notifyDirectionReceived is the `last-notification` direction ze writes for
// a NOTIFICATION the peer sent it (plugin.NotifReceived.String).
const notifyDirectionReceived = "received"

// requireFRRRefusesDraftForm requires the session on which ze sends the bare
// form to be refused by FRR. FRR 10.3.1 reads the capability's first octet as
// a length (bgp_capability_software_version), so "Ze/0.1.0" reads as a
// 90-octet string in an 8-octet capability, and FRR rejects the OPEN. The
// refusal is visible on ze's side, where the NOTIFICATION arrives: FRR counts
// no OPEN received for a message it failed to parse.
func requireFRRRefusesDraftForm(ctx context.Context, lab interoplab.CheckerLab, frrAddress, zeAddress string) error {
	var text textbuf.Buffer
	peer := zeCommand(text.Str("show bgp peer ").Str(frrAddress).Str(" detail").String())
	answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "ze's record of FRR's NOTIFICATION to the draft form",
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, "ze", peer, queryEnvironment("ze", peer))
	}, func(output string) bool {
		code, subcode, ok := zeLastNotificationReceived(output, frrAddress)
		return ok && code == notifyOpenMessageError && subcode == notifyUnspecific
	})
	if err != nil {
		return fmt.Errorf("draft form: FRR 10.3.1 reads the first octet of the Software Version "+
			"capability as a length, so it must refuse ze's bare form with NOTIFICATION OPEN Message Error/Unspecific (2/0); "+
			"ze never recorded one received from FRR at %s: %w\n%s", frrAddress, err, strings.TrimSpace(answer))
	}
	neighbor := []string{cmdVtysh, "-c", text.Reset().Str("show bgp neighbor ").Str(zeAddress).Str(" json").String()}
	state, err := lab.Query(ctx, peerFRR, neighbor, nil)
	if err != nil {
		return fmt.Errorf("draft form: read FRR neighbor %s: %w", zeAddress, err)
	}
	var document any
	if err := json.Unmarshal([]byte(state), &document); err != nil {
		return fmt.Errorf("draft form: decode FRR neighbor %s: %w", zeAddress, err)
	}
	value, ok := findJSONField(document, "bgpState")
	if !ok {
		return fmt.Errorf("draft form: FRR shows no session state for ze at %s:\n%s", zeAddress, strings.TrimSpace(state))
	}
	if value == stateEstablished {
		return fmt.Errorf("draft form: FRR refused ze's OPEN yet shows the session with ze at %s as %s", zeAddress, stateEstablished)
	}
	return nil
}

// zeLastNotificationReceived reads the code and subcode of the last
// NOTIFICATION ze received from peer, out of `show bgp peer <ip> detail`, and
// reports false when ze holds none received from it.
func zeLastNotificationReceived(output, peer string) (code, subcode int, ok bool) {
	var document struct {
		Peers map[string]struct {
			LastNotification *struct {
				Code      *float64 `json:"code"`
				Subcode   *float64 `json:"subcode"`
				Direction string   `json:"direction"`
			} `json:"last-notification"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return 0, 0, false
	}
	last := document.Peers[peer].LastNotification
	if last == nil {
		return 0, 0, false
	}
	if last.Code == nil {
		return 0, 0, false
	}
	if last.Subcode == nil {
		return 0, 0, false
	}
	if last.Direction != notifyDirectionReceived {
		return 0, 0, false
	}
	return int(*last.Code), int(*last.Subcode), true
}

// requireFRRShowsZeVersion waits for FRR's session with ze's address to reach
// Established, then requires FRR to show the version ze sent on it.
func requireFRRShowsZeVersion(ctx context.Context, lab interoplab.CheckerLab, zeAddress, form string) error {
	var command textbuf.Buffer
	session := []string{cmdVtysh, "-c", command.Str("show bgp neighbor ").Str(zeAddress).Str(" json").String()}
	if err := waitJSONFields(ctx, lab, peerFRR, session, 90*time.Second,
		map[string]string{"bgpState": stateEstablished}, nil); err != nil {
		return fmt.Errorf("%s: FRR never reached Established with ze at %s: %w", form, zeAddress, err)
	}
	answer, err := lab.Query(ctx, peerFRR, session, nil)
	if err != nil {
		return fmt.Errorf("%s: read FRR neighbor %s: %w", form, zeAddress, err)
	}
	version, ok := frrReceivedSoftwareVersion(answer)
	if !ok {
		return fmt.Errorf("%s: FRR shows no software version received from ze at %s:\n%s", form, zeAddress, strings.TrimSpace(answer))
	}
	if version != zeSoftwareVersion {
		return fmt.Errorf("%s: FRR shows ze's version as %q, want %q", form, version, zeSoftwareVersion)
	}
	return nil
}

// frrReceivedSoftwareVersion reads the version FRR received from ze out of
// `show bgp neighbor <ip> json`, and reports false when FRR holds none.
func frrReceivedSoftwareVersion(output string) (string, bool) {
	var document any
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return "", false
	}
	value, ok := findJSONField(document, frrReceivedVersionKey)
	if !ok {
		return "", false
	}
	version, ok := value.(string)
	return version, ok
}
