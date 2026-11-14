// Design: docs/functional-tests.md -- the binaries a suite runs against
// Related: runner.go -- Build, the direct route that builds UpgradeBinary beside ze
// Related: ../../le/test/functional/binaries.go -- daemonBuildCommands, the isolated and guest sets that build it

package runner

// UpgradeBinary is the second build of the DUT, the one
// test/ui/native-install-upgrade.ci installs over an installation of ze. It
// carries the DUT's exact tags and a fixed release stamp, so the upgrade goes
// to a binary whose bytes and reported version both differ from ze's. It is
// built beside ze, so a fixture execs it by bare name off the child PATH.
const UpgradeBinary = "ze-upgrade"

// UpgradeRelease is the release UpgradeBinary reports. It is a valid YY.MM.DD
// release that neither the unstamped DUT ("dev") nor Build's stamp (the build
// day) produces before 2099-12-31.
const UpgradeRelease = "99.12.31"

// UpgradeLDFlags is the -ldflags value that stamps UpgradeRelease. Every
// route that builds UpgradeBinary passes it, and no other build does.
const UpgradeLDFlags = "-X main.version=" + UpgradeRelease
