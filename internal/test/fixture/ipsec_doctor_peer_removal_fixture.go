// Design: docs/guide/ipsec.md -- runtime IPsec device findings
// Related: reload_fixture_live_stages.go -- liveStagesReload, the generation fence
// Related: misc_fixture_observers.go -- dispatchJSON, the structured daemon read

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// These causes identify the zetest backend's synthetic retained state. Requiring
// the counter warning and device condition in one answer excludes a failed
// callback followed by a separate successful callback from proving absence.
const (
	ipsecDoctorRetainedCause = "the recording-esp-form-retained test backend " +
		"retains an earlier receiver-open failure"
	ipsecDoctorCounterCause = "the recording-esp-form-retained test backend " +
		"keeps counter reads unavailable"
	ipsecDoctorPeerRemovalMarker = "IPSEC_DOCTOR_PEER_REMOVAL_AND_READD_PASSED"
	ipsecDoctorUnreachableCode   = "doctor-plugin-unreachable"
)

func init() {
	Register("ipsec/doctor-peer-removal", observeDriver("ipsec-doctor-peer-removal-test", ipsecDoctorPeerRemoval), StopsDaemon)
}

// ipsecDoctorPeerRemoval drives the same engine through three applied peer sets.
// The .ci keeps this observer, the IKE root and any external IKE block identical
// in all configurations so neither process is replaced between observations.
func ipsecDoctorPeerRemoval(ctx context.Context, p *sdk.Plugin) error {
	if err := ipsecDoctorPeerSnapshot(ctx, p, true); err != nil {
		return fmt.Errorf("configured idle peer: %w", err)
	}
	if err := liveStagesReload(ctx, p, "no-peer.conf"); err != nil {
		return fmt.Errorf("remove last peer: %w", err)
	}
	if err := ipsecDoctorPeerSnapshot(ctx, p, false); err != nil {
		return fmt.Errorf("no configured peer: %w", err)
	}
	if err := liveStagesReload(ctx, p, "with-peer.conf"); err != nil {
		return fmt.Errorf("re-add peer: %w", err)
	}
	if err := ipsecDoctorPeerSnapshot(ctx, p, true); err != nil {
		return fmt.Errorf("re-added idle peer: %w", err)
	}
	_, err := fmt.Fprintln(os.Stderr, ipsecDoctorPeerRemovalMarker)
	return err
}

// ipsecDoctorPeerSnapshot checks device applicability and a positive callback
// witness in one daemon answer per attempt. A missing callback, an unreachable
// plugin or an independent warning without its synthetic cause cannot pass.
func ipsecDoctorPeerSnapshot(ctx context.Context, p *sdk.Plugin, wantDevice bool) error {
	last := "no answer"
	if Poll(ctx, liveStagesPollAttempts, liveStagesPollDelay, func() bool {
		status, value, err := dispatchJSON(ctx, p, "show doctor config ze-bgp.conf")
		if err != nil {
			last = err.Error()
			return false
		}
		if status != statusDone {
			last = "status " + status
			return false
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			last = err.Error()
			return false
		}
		last = string(encoded)
		var report diagnostic.DoctorResult
		if err := json.Unmarshal(encoded, &report); err != nil {
			last = err.Error()
			return false
		}
		if report.SchemaVersion < 1 {
			return false
		}
		var device, retainedCause, counterWarning bool
		for i := range report.Diagnostics {
			finding := &report.Diagnostics[i]
			switch finding.Code {
			case ipsecDoctorUnreachableCode:
				return false
			case "doctor-ipsec-esp-form-tun":
				device = true
				retainedCause = retainedCause || strings.Contains(finding.Message, ipsecDoctorRetainedCause)
			case "doctor-ipsec-esp-form-fast-path":
				counterWarning = counterWarning || finding.Severity == diagnostic.SeverityWarning &&
					strings.Contains(finding.Message, ipsecDoctorCounterCause)
			}
		}
		last = fmt.Sprintf("device=%t retained-cause=%t counter-warning=%t; %s", device, retainedCause, counterWarning, encoded)
		return counterWarning && device == wantDevice && (!wantDevice || retainedCause)
	}) {
		return nil
	}
	return fmt.Errorf("no single doctor answer had retained device finding=%t and the counter warning; last answer: %.1600s", wantDevice, last)
}
