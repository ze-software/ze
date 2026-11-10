// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- the blind-detector diagnostic through show doctor
// Overview: register_ddos_detect_blind.go -- the observer registered from this function
// Related: plugin_fixture_06.go -- fixture06DispatchObject, the show doctor read
// Related: dns_source_operator.go -- operatorPollAttempts, operatorPollDelay

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// ddosBlindExternalText is the part of the blind-detector warning that names
// the external-process cause. A finding without it (the no-interface text an
// external detector's empty samples would otherwise produce) fails the run.
const ddosBlindExternalText = "runs as an external plugin process"

// doctorDDoSDetectBlind polls show doctor until it carries
// doctor-ddos-detect-blind with the external-process cause.
func doctorDDoSDetectBlind(ctx context.Context, p *sdk.Plugin) error {
	var last map[string]any
	found := Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		data, err := fixture06DispatchObject(ctx, p, "show doctor")
		if err != nil {
			return false
		}
		last = data
		return ddosBlindExternalReported(data)
	})
	if !found {
		return fmt.Errorf("show doctor lacks %s naming %q; last answer %v",
			diagnostic.CodeDoctorDDoSDetectBlind, ddosBlindExternalText, last)
	}
	_, err := fmt.Fprintln(os.Stderr, "OK: show doctor reports doctor-ddos-detect-blind for an external ddos-detect")
	return err
}

// ddosBlindExternalReported answers whether data holds the blind-detector
// diagnostic naming the external-process cause.
func ddosBlindExternalReported(data map[string]any) bool {
	diagnostics, ok := data["diagnostics"].([]any)
	if !ok {
		return false
	}
	for _, raw := range diagnostics {
		entry, isMap := raw.(map[string]any)
		if !isMap || entry["code"] != diagnostic.CodeDoctorDDoSDetectBlind {
			continue
		}
		if message, isString := entry["message"].(string); isString && strings.Contains(message, ddosBlindExternalText) {
			return true
		}
	}
	return false
}
