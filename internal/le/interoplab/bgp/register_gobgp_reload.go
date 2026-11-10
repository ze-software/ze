// Design: docs/architecture/testing/interop.md -- new GoBGP reload successor proof.
package bgp

const (
	gobgpReloadOrdinary  = "bgp-export-reload-gobgp"
	gobgpReloadSecondary = "bgp-export-reload-secondary-gobgp"
	gobgpReloadRunning   = "/run/gobgp.toml"
	gobgpReloadMounted   = "/etc/gobgp/reload/"
)

// gobgpReloadPhaseRepeat is the phase that loads the running config again
// unchanged.
const gobgpReloadPhaseRepeat = "repeat"

// The phase order is part of the frozen stimulus, not inferred from daemon output.
var gobgpReloadPhases = [...]string{
	"initial", "accept", "attributes", "reject", "fallback", "replacement",
	"last", "simultaneous", gobgpReloadPhaseRepeat, "restore",
}

var gobgpReloadPrefixes = [...]string{
	"10.240.0.0/24", "10.240.1.0/24", "10.240.2.0/24", "10.240.3.0/24",
	"10.240.4.0/24", "10.240.5.0/24", "10.240.6.0/24", "10.240.7.0/24",
}

func init() {
	specialCheckers[gobgpReloadOrdinary] = checkGoBGPExportReload
	specialCheckers[gobgpReloadSecondary] = checkGoBGPExportReload
	registerPeerPreparation(gobgpReloadOrdinary, prepareGoBGPReloadPeers)
	registerPeerPreparation(gobgpReloadSecondary, prepareGoBGPReloadPeers)
}
