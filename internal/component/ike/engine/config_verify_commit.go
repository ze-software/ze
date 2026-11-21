// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions and their commit refusals
// Related: register.go -- runEngine, which registers verifyIPsecCommit as the plugin's OnConfigVerify callback
// Related: config.go -- validateIPsecSections and parseVPNSections, the first two steps
// Related: spd_forward_bypass.go -- verifyForwardBypass, the RFC 4301 Section 7.4 refusal

package engine

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// verifyIPsecCommit is the plugin's OnConfigVerify callback (runEngine registers it):
// it refuses a candidate the loaded dataplane or the running state cannot carry, and
// stages the one it accepts for the apply that follows. state is the engine's running
// state, read only for the discard catch-all now in force.
func verifyIPsecCommit(state *ikeEngineState, staging *ikeConfigStaging, sections []sdk.ConfigSection) error {
	if err := validateIPsecSections(sections); err != nil {
		return fmt.Errorf("ike config: %w", err)
	}
	cfg, err := parseVPNSections(sections)
	if err != nil {
		return fmt.Errorf("ike config: %w", err)
	}
	// A discard catch-all the loaded dataplane cannot hold is refused here, at
	// commit, rather than failing the apply (verifyUnmatchedEnforceable, unmatched.go).
	if err := verifyUnmatchedEnforceable(dataplane.Get(), cfg.Unmatched); err != nil {
		return fmt.Errorf("ike config: %w", err)
	}
	// A df-bit, dscp-map or icmp-error-check the loaded dataplane cannot apply is
	// refused here too, at commit, rather than at install once IKE has negotiated
	// the Child SA (verifyPeerSAOptions, sa_options.go). Its refusal is already an
	// operator error naming the peer and the fix, so it passes through unwrapped.
	dp, dpName := dataplane.GetNamed()
	if err := verifyPeerSAOptions(dp, dpName, cfg); err != nil {
		return err
	}
	// RFC 4301 Section 7.4: a bypass with a port that forwarded traffic can match is
	// refused at commit on a dataplane with a forward lookup that runs before
	// reassembly, because no such bypass reaches that lookup there; on Ze's kernel,
	// which gathers fragments first, it is projected and accepted
	// (verifyForwardBypass, spd_forward_bypass.go). Its refusal is
	// an operator error naming the entry and the fix, so it passes through unwrapped.
	if err := verifyForwardBypass(dp, cfg); err != nil {
		return err
	}
	// RFC 4301 Section 4.4.1: a commit that turns the default discard catch-all on
	// without the operator writing it is refused (refuseImplicitDiscard, unmatched.go).
	if err := refuseImplicitDiscard(cfg, state.discardInForce.Load()); err != nil {
		return fmt.Errorf("ike config: %w", err)
	}
	// AC-12 of spec-ipsec-spd-selection: an IKE local address that is a tunnel
	// address would be steered into its own tunnel by the source guard
	// (refuseTunnelAddressAsIKEEndpoint, source_guard.go).
	if err := refuseTunnelAddressAsIKEEndpoint(cfg); err != nil {
		return fmt.Errorf("ike config: %w", err)
	}
	staging.stage(cfg)
	return nil
}
