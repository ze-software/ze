// Design: docs/architecture/testing/interop.md -- draft-ietf-bess-mup-safi Section 3.3.6 Direct Segment Discovery against GoBGP v4.11.0.
// Related: check_special.go -- registry entry for bgp-mup-dsd-gobgp.
// Related: check_rfc7947_filtered.go -- adjRIBInShowEnvelope decoded for show bgp adj-rib-in.
// RFC: rfc/short/draft-ietf-bess-mup-safi.md -- Section 3.3.6, Direct Segment Discovery with SRv6 Prefix-SID and unresolved IGP locator keep-and-warn.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	mupDSDGoBGPScenario = "bgp-mup-dsd-gobgp"

	mupDSDAddress        = "192.0.2.1"
	mupDSDRD             = "65002:100"
	mupDSDSIDPrefix      = "2001:db8::/48"
	mupDSDSIDAddr        = "2001:db8::"
	mupDSDNextHop        = "2001:db8:5::1"
	mupDSDFamily         = "ipv4/mup"
	mupDSDNLRIHex        = "0100020c0000fdea00000064c0000201"
	mupDSDControlNLRIHex = "180a1400"
	mupDSDUnresolvedCode = "doctor-bgp-mup-locator-unresolved"
	mupDSDReportSource   = "bgp-nlri-mup"
	mupDSDUnresolvedFix  = "ze show isis database"
	mupDSDNoActiveSource = "no-active-source"
)

type showWarningsEnvelope struct {
	Warnings []showWarningEntry `json:"warnings"`
	Count    int                `json:"count"`
}

type showWarningEntry struct {
	Source   string `json:"source"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}

func requireMUPDSDAdjRIBIn(raw, gobgpAddress string) error {
	var env adjRIBInShowEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return fmt.Errorf("decode show bgp adj-rib-in: %w", err)
	}
	rows := env.AdjRIBIn[gobgpAddress]
	if len(rows) == 0 {
		return fmt.Errorf("peer %s has no Adj-RIB-In entries", gobgpAddress)
	}
	var sawControl, sawDSD bool
	for _, row := range rows {
		nlri := strings.ToLower(row.NLRIHex)
		switch {
		case row.Family == zeIPv4Unicast && nlri == mupDSDControlNLRIHex:
			sawControl = true
		case row.Family == mupDSDFamily && nlri == mupDSDNLRIHex:
			if row.Filtered != nil && *row.Filtered {
				return fmt.Errorf("DSD route %s from %s unexpectedly marked filtered", nlri, gobgpAddress)
			}
			sawDSD = true
		}
	}
	if !sawControl {
		return fmt.Errorf("peer %s Adj-RIB-In missing control %s (%s)", gobgpAddress, peerPrefixFence, mupDSDControlNLRIHex)
	}
	if !sawDSD {
		return fmt.Errorf("peer %s Adj-RIB-In missing ipv4/mup DSD route %s", gobgpAddress, mupDSDNLRIHex)
	}
	return nil
}

func requireMUPDSDUnresolvedWarning(raw, gobgpAddress string) error {
	var env showWarningsEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return fmt.Errorf("decode show warnings: %w", err)
	}
	wantSubject := gobgpAddress + " " + mupDSDNoActiveSource
	for _, entry := range env.Warnings {
		if entry.Code != mupDSDUnresolvedCode {
			continue
		}
		if entry.Source != mupDSDReportSource {
			return fmt.Errorf("warning %s has source %q, want %q", mupDSDUnresolvedCode, entry.Source, mupDSDReportSource)
		}
		if entry.Subject != wantSubject {
			return fmt.Errorf("warning %s has subject %q, want %q", mupDSDUnresolvedCode, entry.Subject, wantSubject)
		}
		if entry.Fix != mupDSDUnresolvedFix {
			return fmt.Errorf("warning %s has fix %q, want %q", mupDSDUnresolvedCode, entry.Fix, mupDSDUnresolvedFix)
		}
		return nil
	}
	return fmt.Errorf("show warnings missing %s (count=%d)", mupDSDUnresolvedCode, len(env.Warnings))
}

func requireMUPDSDUnresolvedLog(logs, gobgpAddress string) error {
	if !strings.Contains(logs, mupDSDUnresolvedCode) {
		return fmt.Errorf("ze logs missing warning code %s", mupDSDUnresolvedCode)
	}
	if !strings.Contains(logs, gobgpAddress) {
		return fmt.Errorf("ze logs missing peer address %s alongside %s", gobgpAddress, mupDSDUnresolvedCode)
	}
	if !strings.Contains(logs, mupDSDSIDAddr) {
		return fmt.Errorf("ze logs missing SRv6 SID %s alongside %s", mupDSDSIDAddr, mupDSDUnresolvedCode)
	}
	return nil
}

func requireMUPDSDSessionStable(peerDetailJSON string) error {
	return jsonFieldsVerdict(peerDetailJSON, map[string]string{
		zeFieldConnectionsEstablished: "1",
		zeFieldConnectionsDropped:     "0",
	}, nil)
}

// checkMUPDSDGoBGP proves draft-ietf-bess-mup-safi Section 3.3.6 end to end
// against GoBGP v4.11.0:
//   - Ze and GoBGP establish an eBGP session negotiating ipv4/unicast and
//     ipv4/mup, with accept-srv6-prefix-sid true on Ze's peer.
//   - GoBGP originates an ipv4-mup Direct Segment Discovery route carrying an
//     SRv6 Prefix-SID (2001:db8::/48) alongside an ipv4/unicast control route.
//   - Ze stores both in bgp-adj-rib-in, keeps the DSD route unverified because
//     no IGP locator-origin source is active, raises and logs
//     doctor-bgp-mup-locator-unresolved, and holds the session up with zero drops.
func checkMUPDSDGoBGP(ctx context.Context, check *interoplab.CheckContext) error {
	const name = mupDSDGoBGPScenario
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("MUP DSD GoBGP scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	gobgpAddress := networkHostAddress(check.Network, 5)

	// Assertion 1: Both GoBGP and Ze report the eBGP session Established.
	if err := waitContainsFold(ctx, check.Lab, peerGoBGP, []string{cmdGoBGP, gobgpNeighbor, zeAddress}, 90*time.Second, peerStateEstablished); err != nil {
		return fail(1, err)
	}
	if err := waitZePeerState(ctx, check.Lab, gobgpAddress, 90*time.Second); err != nil {
		return fail(1, err)
	}

	// Assertion 2: GoBGP injects the control IPv4 route and a DSD route with Prefix-SID.
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, peerPrefixFence, "-a", gobgpFamilyIPv4,
		gobgpNextHop, gobgpAddress,
	}, nil); err != nil {
		return fail(2, fmt.Errorf("inject ipv4 control route on GoBGP: %w", err))
	}
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, "-a", gobgpFamilyIPv4MUP,
		"dsd", mupDSDAddress,
		"rd", mupDSDRD,
		"prefix", mupDSDSIDPrefix,
		"locator-node-length", "16",
		"function-length", "16",
		"behavior", "END_DT4",
		"rt", mupDSDRD,
		"mup", mupDSDRD,
		gobgpNextHop, mupDSDNextHop,
	}, nil); err != nil {
		return fail(2, fmt.Errorf("inject ipv4-mup DSD route on GoBGP: %w", err))
	}

	// Assertion 3: GoBGP's own ipv4-mup RIB holds the originated DSD route.
	if err := waitContains(ctx, check.Lab, peerGoBGP, []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4MUP}, 30*time.Second, mupDSDAddress, mupDSDRD); err != nil {
		return fail(3, err)
	}

	// Assertion 4: Ze stores both the control prefix and the live GoBGP DSD NLRI in bgp-adj-rib-in.
	showAdj := zeCommand("show bgp adj-rib-in")
	lastAdj, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     45 * time.Second,
		Interval:    time.Second,
		Description: "Ze Adj-RIB-In stores GoBGP ipv4/mup DSD route and control",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showAdj, queryEnvironment("ze", showAdj))
	}, func(output string) bool {
		return requireMUPDSDAdjRIBIn(output, gobgpAddress) == nil
	})
	if err != nil {
		return fail(4, interoplab.WithSearchedOutput(err, lastAdj))
	}

	// Assertion 5: Ze's show warnings carries doctor-bgp-mup-locator-unresolved.
	showWarnings := zeCommand("show warnings")
	lastWarn, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "Ze show warnings reports doctor-bgp-mup-locator-unresolved",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", showWarnings, queryEnvironment("ze", showWarnings))
	}, func(output string) bool {
		return requireMUPDSDUnresolvedWarning(output, gobgpAddress) == nil
	})
	if err != nil {
		return fail(5, interoplab.WithSearchedOutput(err, lastWarn))
	}

	// Assertion 6: Ze logs emit doctor-bgp-mup-locator-unresolved with peer and SID.
	lastLogs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "Ze logs record doctor-bgp-mup-locator-unresolved for GoBGP DSD SID",
	}, func(probe context.Context) (string, error) {
		logs, logErr := check.Lab.Logs(probe, "ze", 2000)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("ze logs were not read")
		}
		return logs.Text, nil
	}, func(output string) bool {
		return requireMUPDSDUnresolvedLog(output, gobgpAddress) == nil
	})
	if err != nil {
		return fail(6, interoplab.WithSearchedOutput(err, lastLogs))
	}

	// Assertion 7: Session stayed Established with zero drops on both Ze and GoBGP.
	detailCmd := zeCommand("show bgp peer " + gobgpAddress + " detail")
	detailOut, err := check.Lab.Query(ctx, "ze", detailCmd, queryEnvironment("ze", detailCmd))
	if err != nil {
		return fail(7, err)
	}
	if err := requireMUPDSDSessionStable(detailOut); err != nil {
		return fail(7, interoplab.WithSearchedOutput(err, detailOut))
	}
	if err := waitContainsFold(ctx, check.Lab, peerGoBGP, []string{cmdGoBGP, gobgpNeighbor, zeAddress}, 30*time.Second, peerStateEstablished); err != nil {
		return fail(7, err)
	}
	return nil
}
