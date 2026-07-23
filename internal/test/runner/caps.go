// Design: docs/architecture/testing/ci-format.md -- option=needs-linux capability gating
// Related: caps_linux.go -- the real probe; record_parse.go -- the needs-linux option

package runner

// goosLinux is runtime.GOOS on Linux. Named so the several capability/netns
// guards in this package share one spelling instead of repeating the literal.
const goosLinux = "linux"

// capsNetAdmin is the only value `option=needs-linux:caps=` accepts today. It
// declares that the test performs privileged network configuration (creating
// interfaces, bringing links up, programming netlink).
const capsNetAdmin = "net-admin"

// hasNetAdmin reports whether this process may perform privileged network
// configuration. It is a package var, not a direct call, so a test can drive
// BOTH polarities of the gate on any host: asserting only the host's real
// answer makes the test vacuous in exactly the environment where the suite runs
// for real (the QEMU VM, where the capability is present and the "skips without
// it" branch is never taken). Same seam as interfaceByName in
// internal/component/ike/engine/doctor.go.
var hasNetAdmin = probeNetAdmin
