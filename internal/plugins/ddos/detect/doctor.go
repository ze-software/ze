// Design: docs/architecture/ddos/cp-survival-5-detect-5-characterization.md -- flow-source readiness.
// Related: register.go -- DoctorChecks registration (ddos-detect-flow-source)
// Related: characterize.go -- the trafficusage / flow-export queries this guards
// Related: blind.go -- the rate feed watch checkRateFeed reports
//
// Characterization needs an on-box flow source: traffic-usage (track-ip) for the
// fast victim target and/or flow-export (conntrack) for the classifier. With
// neither configured the detector still runs but degrades to generic-flood with
// no target, so `ze doctor` warns rather than leaving the gap silent.
package detect

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rateFeedCheck is the plugin runtime doctor check that reports an armed
// detector whose rate feed shows no traffic. It is declared in the SDK
// registration (register.go) and answered by the plugin's OnDoctorCheck
// callback, because only the running plugin holds the detector: a check in the
// daemon's registry runs in the daemon process and sees no detector when the
// plugin runs external.
const rateFeedCheck = "ddos-detect-rate-feed"

// rateFeedCheckDecl is the Stage 1 declaration of rateFeedCheck.
var rateFeedCheckDecl = rpc.DoctorCheckDecl{
	Name:  rateFeedCheck,
	Phase: rpc.DoctorPhasePostConfig,
	Codes: []string{diagnostic.CodeDoctorDDoSDetectBlind},
}

// checkRateFeed answers the plugin doctor callback. det is the armed detector,
// or nil while ddos detect is disabled. A blind feed is a warning, not an
// error: an idle lab or standby router legitimately carries no traffic.
func checkRateFeed(name string, det *detector) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != rateFeedCheck {
		return nil, errors.New("ddos-detect has no doctor check named " + strconv.Quote(name) +
			"; it declares only " + rateFeedCheck)
	}
	// refuse-by-default: guards runEngine -- it publishes a nil detector while ddos detect is disabled, and a disabled detector subscribes to no feed, so it cannot be blind
	if det == nil {
		return nil, nil
	}
	f := det.rateFeedFinding()
	switch f.state {
	case feedStateSeeing:
		return nil, nil
	case feedStateNoSample, feedStateZeroTraffic, feedStateNoInterface:
		return []rpc.DoctorCheckDiagnostic{{
			Code:     diagnostic.CodeDoctorDDoSDetectBlind,
			Severity: "warning",
			Message:  blindMessage(f),
		}}, nil
	case feedStateUnarmed:
		return nil, errors.New("ddos-detect is enabled but its detector is not subscribed to a rate feed, so its state cannot be judged")
	case feedStateUnspecified:
		return nil, errors.New("ddos-detect: the rate feed state was never set")
	}
	return nil, errors.New("ddos-detect: unknown rate feed state " + strconv.Itoa(int(f.state)))
}

// checkFlowSource warns when ddos-detect is enabled with characterization on but
// neither traffic-usage nor flow-export is configured. ddos-detect owns the
// characterization behavior, so it owns this readiness check (repo-maintenance.md).
func checkFlowSource(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	dd := tree.GetContainerPath(configRoot)
	if dd == nil {
		return nil
	}
	if enabled, ok := dd.Get("enabled"); !ok || enabled != "true" {
		return nil
	}
	// characterize-enable defaults true; only a value present is authoritative.
	if ce, ok := dd.Get("characterize-enable"); ok && ce == "false" {
		return nil
	}
	if tree.GetContainerPath("traffic/usage") != nil || tree.GetContainer("flow-export") != nil {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ddos-detect-no-flow-source",
		Severity: "warning",
		Message:  "ddos-detect characterization is enabled but neither traffic-usage (track-ip) nor flow-export (conntrack) is configured; mitigation falls back to generic-flood with no target",
	}}
}
