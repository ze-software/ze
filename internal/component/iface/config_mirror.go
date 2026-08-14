// Design: docs/features/interfaces.md -- Traffic mirroring from interface config
// Related: config_apply.go -- reconciliation, config_sysctl.go -- sysctl application

package iface

import (
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// mirrorSpec is the mirror one unit asks for: the destination interface of
// each direction, empty when that direction is not mirrored.
type mirrorSpec struct {
	ingress string
	egress  string
}

// indexMirrorSpecs returns an OS device name -> mirrorSpec map of the mirrors
// a config asks for. A disabled interface or unit contributes nothing, so
// disabling one is a way of asking for no mirror.
func indexMirrorSpecs(cfg *ifaceConfig) map[string]mirrorSpec {
	if cfg == nil {
		return nil
	}
	specs := make(map[string]mirrorSpec)
	for _, e := range allIfaceEntries(cfg) {
		if e.Disable {
			continue
		}
		for i := range e.Units {
			u := &e.Units[i]
			if u.Disable || (u.MirrorIngress == "" && u.MirrorEgress == "") {
				continue
			}
			specs[unitOSName(e.Name, u)] = mirrorSpec{ingress: u.MirrorIngress, egress: u.MirrorEgress}
		}
	}
	return specs
}

// removeStaleMirrors tears down every mirror the previous config installed
// that the new config no longer asks for, or asks for differently. It runs
// before applyMirror because tc filters are additive: installing a new
// destination does not retire the old one, so a changed mirror is a remove
// followed by an install.
//
// A mirror on a device that is gone needs no teardown: the kernel dropped the
// tc state with the device, and the backend would fail to resolve the name.
//
// Each removal is journalled with the previous mirror as its undo, so a later
// failure in the same apply puts the mirror back.
func removeStaleMirrors(cfg, previous *ifaceConfig, b Backend, journal *sdk.Journal) []error {
	previousSpecs := indexMirrorSpecs(previous)
	if len(previousSpecs) == 0 {
		return nil
	}
	desired := indexMirrorSpecs(cfg)

	var errs []error
	// Walk the previous config in its own order, so the teardown sequence is
	// reproducible rather than map-random.
	for _, e := range allIfaceEntries(previous) {
		for i := range e.Units {
			osName := unitOSName(e.Name, &e.Units[i])
			prev, had := previousSpecs[osName]
			if !had || desired[osName] == prev {
				continue
			}
			delete(previousSpecs, osName)
			if !interfaceExists(b, osName) {
				continue
			}
			if err := applyBackendStep(journal, func() error {
				return b.RemoveMirror(osName)
			}, func() error {
				return setupMirrorSpec(b, osName, prev)
			}); err != nil {
				loggerPtr.Load().Warn("iface config: mirror teardown", "iface", osName, "err", err)
				errs = append(errs, fmt.Errorf("%s mirror teardown: %w", osName, err))
			}
		}
	}
	return errs
}

// setupMirrorSpec installs a mirrorSpec on one interface. Two destinations
// need two backend calls, one per direction, because SetupMirror carries one
// destination. A failure on the second call removes what the first installed,
// so the interface never keeps half a mirror the config did not ask for.
func setupMirrorSpec(b Backend, osName string, m mirrorSpec) error {
	if m.ingress != "" && m.ingress == m.egress {
		return b.SetupMirror(osName, m.ingress, true, true)
	}
	if m.ingress != "" {
		if err := b.SetupMirror(osName, m.ingress, true, false); err != nil {
			return err
		}
	}
	if m.egress != "" {
		if err := b.SetupMirror(osName, m.egress, false, true); err != nil {
			if m.ingress != "" {
				if undoErr := b.RemoveMirror(osName); undoErr != nil {
					loggerPtr.Load().Warn("iface config: mirror ingress left installed after the egress direction failed",
						"iface", osName, "err", undoErr)
				}
			}
			return err
		}
	}
	return nil
}

// applyMirror installs the mirror a unit asks for. A unit that asks for none
// is left alone here: retiring a mirror the config dropped is removeStaleMirrors'
// job, which runs first and has the previous config to compare against.
// Returns one error per mirror operation that failed.
func applyMirror(b Backend, osName string, u unitEntry, journal *sdk.Journal) []error {
	desired := mirrorSpec{ingress: u.MirrorIngress, egress: u.MirrorEgress}
	if desired == (mirrorSpec{}) {
		return nil
	}

	if err := applyBackendStep(journal, func() error {
		return setupMirrorSpec(b, osName, desired)
	}, func() error {
		return b.RemoveMirror(osName)
	}); err != nil {
		loggerPtr.Load().Warn("iface config: mirror", "iface", osName, "err", err)
		return []error{fmt.Errorf("%s mirror: %w", osName, err)}
	}
	return nil
}
