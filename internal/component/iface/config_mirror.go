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

// mirrorSpecFor returns the mirror one unit asks for, with both destinations
// resolved through the apply's selector map. A mirror names two interfaces and
// the destination is as selectable as the source, so a capture port bound by
// mac/match or aliased by os-name has to be translated here too: tc installs
// the filter toward a device, and the logical name reaches whatever else
// carries it.
func mirrorSpecFor(u *unitEntry, devices map[string]string) mirrorSpec {
	return mirrorSpec{
		ingress: mirrorDestination(devices, u.MirrorIngress),
		egress:  mirrorDestination(devices, u.MirrorEgress),
	}
}

// mirrorDestination returns the kernel device a mirror destination names. An
// empty name asks for no mirror in that direction and stays empty. A name whose
// selector answers with nothing is UNBOUND and also returns empty: the capture
// port is not present, and a mirror toward a device that is not there is a
// mirror the next apply installs, not one to point at a stranger.
func mirrorDestination(devices map[string]string, name string) string {
	if name == "" {
		return ""
	}
	device, bound := deviceFor(devices, name)
	if !bound {
		return ""
	}
	return device
}

// indexMirrorSpecs returns an OS device name -> mirrorSpec map of the mirrors
// a config asks for. A disabled interface or unit contributes nothing, so
// disabling one is a way of asking for no mirror.
//
// devices is bindDevices' answer for cfg, so a mirror on an interface selected
// by mac/match or aliased by os-name is keyed by the kernel device tc installs
// the filter on, and points at the kernel device the capture port resolves to.
// An entry whose binding is unbound contributes nothing: there is no device to
// mirror, and keying it by the logical name would install the filter on
// whatever else carries that name.
func indexMirrorSpecs(cfg *ifaceConfig, devices map[string]string) map[string]mirrorSpec {
	if cfg == nil {
		return nil
	}
	specs := make(map[string]mirrorSpec)
	for _, e := range allIfaceEntries(cfg) {
		if e.Disable {
			continue
		}
		device, bound := deviceFor(devices, e.Name)
		if !bound {
			continue
		}
		for i := range e.Units {
			u := &e.Units[i]
			if u.Disable || (u.MirrorIngress == "" && u.MirrorEgress == "") {
				continue
			}
			specs[unitOSName(device, u)] = mirrorSpecFor(u, devices)
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
// That skip is decided by whether the backend can read the device, which cannot
// tell a deleted device from a device it failed to read, so the skip is logged.
// A read that failed for another reason leaves the mirror installed, and the
// next apply carries the new config as its previous, so nothing retries it.
// Reconciling live kernel state instead of a config delta is the fix, and it is
// the same fix a restart needs (R-2 in spec-fixit-mirror-clsact-ownership).
//
// Each removal is journalled with the previous mirror as its undo, so a later
// failure in the same apply puts the mirror back.
func removeStaleMirrors(cfg, previous *ifaceConfig, devices, previousDevices map[string]string, b Backend, journal *sdk.Journal) []error {
	previousSpecs := indexMirrorSpecs(previous, previousDevices)
	if len(previousSpecs) == 0 {
		return nil
	}
	desired := indexMirrorSpecs(cfg, devices)

	var errs []error
	// Walk the previous config in its own order, so the teardown sequence is
	// reproducible rather than map-random.
	for _, e := range allIfaceEntries(previous) {
		device, bound := deviceFor(previousDevices, e.Name)
		if !bound {
			continue
		}
		for i := range e.Units {
			osName := unitOSName(device, &e.Units[i])
			prev, had := previousSpecs[osName]
			if !had || desired[osName] == prev {
				continue
			}
			delete(previousSpecs, osName)
			if _, err := b.GetInterface(osName); err != nil {
				loggerPtr.Load().Warn("iface config: mirror teardown skipped, cannot read the interface",
					"iface", osName, "err", err)
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

// applyMirror installs the mirror a unit asks for, toward the kernel device
// each destination resolves to. A unit that asks for none is left alone here:
// retiring a mirror the config dropped is removeStaleMirrors' job, which runs
// first and has the previous config to compare against. A destination whose
// selector has no answer yet is dropped with a warning rather than installed
// toward its logical name, exactly as an unbound source is skipped.
// Returns one error per mirror operation that failed.
func applyMirror(b Backend, osName string, u unitEntry, devices map[string]string, journal *sdk.Journal) []error {
	if u.MirrorIngress == "" && u.MirrorEgress == "" {
		return nil
	}
	desired := mirrorSpecFor(&u, devices)
	if desired.ingress == "" && u.MirrorIngress != "" {
		loggerPtr.Load().Warn("iface config: no present device answers this mirror destination's hardware selector, ingress mirror deferred",
			"iface", osName, "destination", u.MirrorIngress)
	}
	if desired.egress == "" && u.MirrorEgress != "" {
		loggerPtr.Load().Warn("iface config: no present device answers this mirror destination's hardware selector, egress mirror deferred",
			"iface", osName, "destination", u.MirrorEgress)
	}
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
