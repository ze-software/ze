// Design: docs/architecture/api/process-protocol.md -- origin admission results.
// Related: types.go -- UpdateRouteOutput; selected.go -- separate final receipts.
package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
)

// RouteAdmissionOutcome is an engine's admission answer, never output authority.
// Zero and unknown values are invalid on both transports.
type RouteAdmissionOutcome uint8

const (
	RouteAdmissionUnspecified RouteAdmissionOutcome = iota
	RouteAdmissionRetained
	RouteAdmissionRefused
	RouteAdmissionFailed
)

// RouteAdmissionDisposition describes the election/export answer, not a socket
// write. Zero is valid only for a refused or failed admission.
type RouteAdmissionDisposition uint8

const (
	RouteAdmissionDispositionUnspecified RouteAdmissionDisposition = iota
	RouteAdmissionSelected
	RouteAdmissionSuppressedSelection
	RouteAdmissionSuppressedExport
)

// RouteAdmission is raw result input at the SDK boundary. One value describes
// one native key in one concrete authorized Peer scope. Generation identifies
// the retained contribution; it is not an OriginReceipt or a writer revision.
// No field in this result grants permission to select or export a route.
type RouteAdmission struct {
	Family      string                    `json:"family"`
	KeyNLRI     []byte                    `json:"key-nlri"`
	Destination string                    `json:"destination"`
	PeerOwner   uint64                    `json:"peer-owner"`
	Generation  uint64                    `json:"generation"`
	Outcome     RouteAdmissionOutcome     `json:"outcome"`
	Disposition RouteAdmissionDisposition `json:"disposition"`
	Reason      string                    `json:"reason,omitempty"`
}

// Validate checks a raw receipt before a consumer uses its admission facts.
func (admission *RouteAdmission) Validate() error {
	if admission == nil {
		return errors.New("update-route admission is missing")
	}
	if admission.Family == "" {
		return errors.New("update-route admission family is missing")
	}
	if len(admission.KeyNLRI) == 0 {
		return errors.New("update-route admission native key is missing")
	}
	if _, err := netip.ParseAddr(admission.Destination); err != nil {
		return fmt.Errorf("update-route admission destination: %w", err)
	}
	if admission.PeerOwner == 0 {
		return errors.New("update-route admission Peer scope is missing")
	}
	// Open wire enum: unknown plugin/engine values fail closed.
	switch admission.Outcome {
	case RouteAdmissionRetained:
		if admission.Generation == 0 {
			return errors.New("update-route retained admission generation is missing")
		}
		return admission.validateDisposition()
	case RouteAdmissionRefused, RouteAdmissionFailed:
		if admission.Generation != 0 {
			return errors.New("update-route unsuccessful admission carries a retained generation")
		}
		if admission.Disposition != RouteAdmissionDispositionUnspecified {
			return errors.New("update-route unsuccessful admission carries a selection disposition")
		}
		if admission.Reason == "" {
			return errors.New("update-route unsuccessful admission reason is missing")
		}
		return nil
	case RouteAdmissionUnspecified:
		return errors.New("update-route admission outcome is unspecified")
	default:
		return errors.New("update-route admission outcome is unknown")
	}
}

func (admission *RouteAdmission) validateDisposition() error {
	// Open wire enum: an unknown disposition cannot imply retained success.
	switch admission.Disposition {
	case RouteAdmissionSelected, RouteAdmissionSuppressedSelection, RouteAdmissionSuppressedExport:
		return nil
	case RouteAdmissionDispositionUnspecified:
		return errors.New("update-route retained admission disposition is unspecified")
	default:
		return errors.New("update-route retained admission disposition is unknown")
	}
}

// Validate checks result shape. An explicit count-only result remains valid for
// direct origination and control commands; it never establishes admission.
// A nil whole result is missing, unlike an explicit result with zero counts.
func (output *UpdateRouteOutput) Validate() error {
	if output == nil {
		return errors.New("update-route result is missing")
	}
	for i := range output.Admissions {
		if err := output.Admissions[i].Validate(); err != nil {
			return fmt.Errorf("update-route admission %d: %w", i, err)
		}
	}
	// The typed adapter cannot accept a result the framed adapter cannot carry.
	// Ordinary count-only direct/control answers keep their allocation-free check.
	if len(output.Admissions) == 0 {
		if len(output.Warnings) == 0 {
			if output.Failure == "" {
				return nil
			}
		}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return err
	}
	if idPrefixMax+len("ok ")+len(encoded) > MaxMessageSize {
		return errors.New("update-route result exceeds the frame limit; split the route command or authorized scope")
	}
	return nil
}

// Err preserves a real command failure while leaving partial counts, receipts
// and warnings available to the caller. Intentional suppression is success.
func (output *UpdateRouteOutput) Err() error {
	if err := output.Validate(); err != nil {
		return err
	}
	if output.Failure != "" {
		return errors.New(output.Failure)
	}
	for _, admission := range output.Admissions {
		// Only valid retained admissions succeeded. Validate checked the enum.
		if admission.Outcome != RouteAdmissionRetained {
			return errors.New(admission.Reason)
		}
	}
	return nil
}

// Retained answers only an explicit successful admission. Counts and warning
// text cannot substitute for it. Safe for concurrent reads of an immutable result.
func (output *UpdateRouteOutput) Retained() bool {
	if err := output.Err(); err != nil {
		return false
	}
	for _, admission := range output.Admissions {
		if admission.Outcome == RouteAdmissionRetained {
			return true
		}
	}
	return false
}
