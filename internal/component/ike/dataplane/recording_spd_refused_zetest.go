//go:build zetest

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- dataplane backend registry
// Related: recording_zetest.go -- the recording dataplane embedded by this fixture
// Related: register_recording_zetest.go -- its registration, in the same build only

package dataplane

import "errors"

// recordingSPDRefusedPriority is the order at which the recording-spd-refused backend
// refuses an operator SPD entry. A .ci reaches the refusal by writing `order 4242` on
// a `vpn ipsec policy` entry, and every other entry is recorded as the recording
// backend records it.
const recordingSPDRefusedPriority = 4242

// errRecordingSPDRefused is the backend's answer to an entry at the refused order. It
// names the test backend, so no reader takes it for a kernel's refusal, and it is
// none of the answers the engine reads as a platform without XFRM
// (engine/child.go isXFRMUnsupported), so the engine treats it as the refusal of a
// working dataplane.
var errRecordingSPDRefused = errors.New("the recording-spd-refused test backend refuses an operator SPD entry at order 4242")

// newRecordingSPDRefusedBackend is the recording backend with one refusal: an operator
// SPD entry installed at recordingSPDRefusedPriority. The kernel offers no refusal an
// operator configuration can reach once the commit verifier has accepted it: every
// EINVAL of verify_newpolicy_info (net/xfrm/xfrm_user.c) judges a field the verifier
// already holds to the same rule (selector family and prefix lengths, action,
// direction), an entry carries no template for validate_tmpl to judge, and the
// backend's update form never meets the EEXIST of an exclusive insert. So a .ci proves
// that a refused install refuses the commit through this backend, unprivileged, with
// ze.test.ike.dataplane=recording-spd-refused (test/ipsec/ipsec-spd-discard-entry.ci).
func newRecordingSPDRefusedBackend() (Dataplane, error) {
	r := &recordingSPDRefused{}
	r.policies = map[recordedPolicyKey]SPParams{}
	return r, nil
}

// recordingSPDRefused is the recording backend that refuses an operator entry at one
// order. Its zero value is not usable: the factory initializes the policy map. Safe
// for concurrent use, as recordingDataplane is; its own state is constant.
type recordingSPDRefused struct {
	recordingDataplane
}

// InstallPolicy refuses an operator entry at the refused order and records every
// other policy.
func (d *recordingSPDRefused) InstallPolicy(p SPParams) error {
	if p.Role == PolicyRoleOperator && p.Priority == recordingSPDRefusedPriority {
		return errRecordingSPDRefused
	}
	return d.recordingDataplane.InstallPolicy(p)
}
