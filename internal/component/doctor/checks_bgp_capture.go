// Design: plan/spec-improve-3-event-replay.md -- BGP protocol event capture readiness
// Related: checks_storage.go -- writability probes for config-declared destinations

// BGP protocol event capture introduces a runtime dependency: a directory the
// daemon must be able to create and write. An operator who enables capture and
// gets nothing has no way to tell a quiet peer from a directory the daemon could
// not open, so doctor answers that question before the daemon is started.

package doctor

import (
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// defaultBGPCaptureDirectory MUST match the default of the peer's
// capture/directory leaf in ze-bgp-conf.yang, and reactor.DefaultCaptureDirectory.
const defaultBGPCaptureDirectory = "/var/lib/ze/capture"

// checkBGPCaptureDirectory reports every peer whose capture is enabled but whose
// capture directory cannot be written.
//
// It probes only peers that opted in. A disabled capture names no dependency, so
// a directory that does not exist is not a finding.
func checkBGPCaptureDirectory(tree *config.Tree) []diagnostic.Diagnostic {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}
	var diags []diagnostic.Diagnostic
	seen := make(map[string]bool)

	probe := func(peerName string, peer *config.Tree) {
		capture := peer.GetContainer("capture")
		if !configEnabled(capture, false) {
			return
		}
		dir := valueOrDefault(capture, "directory", defaultBGPCaptureDirectory)
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		if err := probeWritableDirCreating(dir); err != nil {
			var tb textbuf.Buffer
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-bgp-capture-directory",
				Severity: diagnostic.SeverityWarning,
				Message: tb.Str("bgp peer ").Str(peerName).
					Str(" enables protocol event capture but its directory is not usable: ").
					Str(dir).Str(": ").Err(err).String(),
				Path: dir,
			})
		}
	}

	for _, p := range bgp.GetListOrdered("peer") {
		probe(p.Key, p.Value)
	}
	for _, g := range bgp.GetListOrdered("group") {
		for _, p := range g.Value.GetListOrdered("peer") {
			probe(p.Key, p.Value)
		}
	}
	return diags
}

// probeWritableDirCreating creates the directory if absent, then probes it. The
// daemon does exactly this at session start, so the check must too: probing
// without creating would report every first run as broken.
func probeWritableDirCreating(dir string) error {
	// captureDirPerm mirrors the mode the reactor uses (capture_replay.go): a
	// capture file holds a peer's routing data, so the directory is not
	// world-readable.
	const captureDirPerm = 0o750
	if err := os.MkdirAll(dir, captureDirPerm); err != nil {
		return err
	}
	return probeWritableDir(dir)
}
