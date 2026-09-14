// Design: docs/architecture/api/architecture.md -- doctor check registration for the gRPC transport
// Related: doctor.go -- checkGRPCTLSMaterial, the check registered here
//
// The gRPC transport is not a plugin, so it registers through the component
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field. Keeping the check and its registration under
// internal/component/api/grpc means removing the transport removes its
// diagnostic with it.

package grpc

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it -- a
	// duplicate name, a phase that does not exist, a code without the doctor
	// prefix -- and none of them can be reached from a config or a peer, so it
	// stops the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(grpcTLSDoctorCheck); err != nil {
		panic("BUG: api-grpc doctor check registration refused: " + err.Error())
	}
}
