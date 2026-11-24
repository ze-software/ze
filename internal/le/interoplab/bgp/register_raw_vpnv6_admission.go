// Design: docs/architecture/testing/interop.md -- raw VPNv6 family admission against FRR.
package bgp

func init() {
	specialCheckers[rawVPNv6AdmissionScenario] = checkRawVPNv6FamilyAdmissionFRR
}
