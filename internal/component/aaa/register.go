// Design: ai/patterns/registration.md -- registration at init, discovered by a registry
// Overview: doctor.go -- the readiness check this file registers

package aaa

import "github.com/ze-software/ze/internal/core/diagnostic"

// The aaa component registers no AAA backend of its own: it owns the registry
// every backend registers INTO. What it does own is the chain, and the one
// readiness question no single backend can answer, which doctor.go carries.
func init() {
	if err := diagnostic.RegisterDoctorCheck(aaaLocalFallbackDoctorCheck); err != nil {
		panic("BUG: aaa: doctor check registration failed: " + err.Error())
	}
}
