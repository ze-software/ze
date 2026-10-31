// Design: docs/functional-tests.md -- compiled entrypoints for the RFC 8212 policy and reload tests.
// Related: rfc8212_policy.go, rfc8212_reload.go, rfc8212_reload_fault.go -- the drivers that name these values.
package fixture

// Values the RFC 8212 drivers share. The values shared with other fixtures
// (addrLoopbackSecond, directionExport, eventActionDel, ...) live in
// constants.go.
const (
	// addrLoopbackFourth is the eBGP peer that carries no policy in either
	// direction.
	addrLoopbackFourth = "127.0.0.4"
	// eventActionAdd is the action of an update event's NLRI entry that
	// announces it.
	eventActionAdd = "add"
	// rfc8212ActionModify is the filter verdict that accepts a route with
	// changes.
	rfc8212ActionModify = "modify"
	// rfc8212DirectionImport is the policy direction a driver mode or a
	// filter container names.
	rfc8212DirectionImport = "import"
	// rfc8212FamilyIPv4Unicast and rfc8212FamilyIPv6Unicast are the family
	// keys of an update event.
	rfc8212FamilyIPv4Unicast = "ipv4/unicast"
	rfc8212FamilyIPv6Unicast = "ipv6/unicast"
	// The rfc8212Fault values are the owner fault modes the reload drivers
	// select.
	rfc8212FaultAck         = "ack"
	rfc8212FaultApply       = "apply"
	rfc8212FaultCommit      = "commit"
	rfc8212FaultDeadline    = "deadline"
	rfc8212FaultDeath       = "death"
	rfc8212FaultFinalize    = "finalize"
	rfc8212FaultOuter       = "outer"
	rfc8212FaultUnsupported = "unsupported"
	// rfc8212PolicyAbsent is the final policy state that removes the filter
	// container.
	rfc8212PolicyAbsent = "absent"
	// rfc8212RootBGP and rfc8212RootEnvironment are config roots.
	rfc8212RootBGP         = "bgp"
	rfc8212RootEnvironment = "environment"
	// rfc8212VerdictUndetermined is the policy verdict of a peer whose
	// session type is not yet known.
	rfc8212VerdictUndetermined = "undetermined"
)
