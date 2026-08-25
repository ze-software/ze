// Design: docs/features/interfaces.md -- Traffic mirroring from interface config
// Related: config_apply.go -- reconciliation, config_sysctl.go -- sysctl application

package iface

import (
	"fmt"
	"sort"

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

// removeStaleMirrors removes every mirror the previous config installed
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
//
// reconcileMirrors is what retries it, and what covers the two cases a delta
// cannot reach at all:
//
//   - A mirror the operator removed from the config file while ze was down.
//   - A teardown this function skipped.
//
// It reads the dataplane rather than the previous config, so it needs neither.
//
// This function still earns its place beside it. It runs BEFORE applyMirror
// installs the new destinations. A mirror the operator retired therefore stops
// copying at the start of the apply rather than at its reconcile phase.
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

// mirrorScope returns every kernel device the configuration configures, keyed
// by the OS name a mirror would be installed on. A disabled entry or unit is
// still ze's, so it is in scope and its mirror is retired rather than left.
//
// It exists because a mirror is a tc filter on a SHARED attachment point, and
// the dataplane cannot say who installed one. What ze can say is which
// interfaces it configures, so that is the boundary the reconcile enforces.
// On an interface in this set the mirror state must equal what the config asks
// for. On every other interface ze has no authority and touches nothing.
//
// Both the current and the previous config contribute. An interface the
// previous config configured is one ze had authority over moments ago. That
// authority is what lets a teardown an earlier apply skipped be retried after
// the operator has also removed the interface.
func mirrorScope(cfg, previous *ifaceConfig, devices, previousDevices map[string]string) map[string]bool {
	scope := make(map[string]bool)
	add := func(c *ifaceConfig, bound map[string]string) {
		if c == nil {
			return
		}
		for _, e := range allIfaceEntries(c) {
			device, ok := deviceFor(bound, e.Name)
			if !ok {
				continue
			}
			scope[device] = true
			for i := range e.Units {
				scope[unitOSName(device, &e.Units[i])] = true
			}
		}
	}
	add(cfg, devices)
	add(previous, previousDevices)
	return scope
}

// reconcileMirrors makes the dataplane's mirrors equal the ones cfg asks for,
// deciding from LIVE state rather than from a delta between two configs. It is
// what closes the two holes a delta leaves open, and both leave packets copied
// to a destination the operator deleted:
//
//   - A mirror removed from the config file while ze was down. The next boot
//     applies with no previous config, so removeStaleMirrors has nothing to
//     compare and the tc filter survives every later apply.
//   - A teardown removeStaleMirrors skipped because it failed to read the
//     interface. That apply consumed the delta, so no later commit retries it.
//
// It acts only on an interface mirrorScope names. The backend reports the whole
// namespace truthfully, and a priority-1 matchall mirred filter is a SHAPE
// rather than a mark of ownership. Removing every one of them would delete a
// filter another tool installed on an interface ze does not configure. The
// sibling passes decide ownership the same way: reconcileOwnedDevices reads a
// positive kernel marker. The vpp backend drops a SPAN entry whose source is
// not a device it names.
//
// The cost of that boundary is one case this cannot reach. An interface whose
// whole stanza was deleted while ze was down keeps its mirror, because nothing
// then distinguishes it from a filter ze never installed.
//
// Reading live state costs a full dump. netlinkBackend.ListMirrors does one
// LinkList and two FilterList per link. This pass runs on every config commit.
// It also runs on every vpp connect and every registry change, because each of
// those re-decides the mirrors as it re-decides every address. The cost is the
// same order as the address reconcile beside it. Know that before you add a
// THIRD trigger.
//
// A mirror is retired and reinstalled rather than edited. A tc filter is
// additive per hook, so dropping ONE direction cannot be expressed as an
// install. Clearing both directions, then installing what is wanted, is the one
// sequence that reaches every desired state from every live one.
//
// A backend that cannot read its live mirrors removes nothing. The pass logs
// and returns, because "I cannot tell" is not "there is nothing there".
// Removing mirrors on an unreadable dataplane is the destructive reading of the
// two (ai/rules/evidence.md).
//
// With a nil journal (the vpp-connect and registry-change paths) a removal that
// succeeds before an install fails leaves the interface with no mirror, and the
// error is reported. That is the honest state rather than a gap. The config no
// longer asks for what was removed, and this pass is decided from live state,
// so the next one retries the install.
func reconcileMirrors(cfg, previous *ifaceConfig, b Backend, devices, previousDevices map[string]string, journal *sdk.Journal) []error {
	live, err := b.ListMirrors()
	if err != nil {
		loggerPtr.Load().Warn("iface config: mirror reconcile skipped, cannot read the live mirrors", "err", err)
		return nil
	}

	scope := mirrorScope(cfg, previous, devices, previousDevices)
	current := make(map[string]mirrorSpec, len(live))
	for _, state := range live {
		if !scope[state.Interface] {
			continue
		}
		current[state.Interface] = mirrorSpec{ingress: state.Ingress, egress: state.Egress}
	}
	desired := indexMirrorSpecs(cfg, devices)

	// One deterministic order over both sides, so a failing interface reports
	// the same way on every run rather than map-random.
	names := make([]string, 0, len(current)+len(desired))
	for name := range current {
		names = append(names, name)
	}
	for name := range desired {
		if _, alsoLive := current[name]; !alsoLive {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var errs []error
	for _, osName := range names {
		installed := current[osName]
		want, asked := desired[osName]
		if installed == want {
			continue
		}
		if installed != (mirrorSpec{}) {
			if err := applyBackendStep(journal, func() error {
				return b.RemoveMirror(osName)
			}, func() error {
				return setupMirrorSpec(b, osName, installed)
			}); err != nil {
				loggerPtr.Load().Warn("iface config: mirror reconcile teardown", "iface", osName, "err", err)
				errs = append(errs, fmt.Errorf("%s mirror reconcile teardown: %w", osName, err))
				continue
			}
			// The two removals are different facts and the operator reads
			// them differently. A unit that asks for a mirror whose
			// destination has no present device resolves to an empty spec.
			// Saying that the config does not ask for one would be wrong.
			if asked && want == (mirrorSpec{}) {
				loggerPtr.Load().Info("iface config: retired a mirror until its destination device appears",
					"iface", osName, "ingress", installed.ingress, "egress", installed.egress)
			} else {
				loggerPtr.Load().Info("iface config: removed a mirror the configuration does not ask for",
					"iface", osName, "ingress", installed.ingress, "egress", installed.egress)
			}
		}
		if want == (mirrorSpec{}) {
			continue
		}
		if err := applyBackendStep(journal, func() error {
			return setupMirrorSpec(b, osName, want)
		}, func() error {
			return b.RemoveMirror(osName)
		}); err != nil {
			loggerPtr.Load().Warn("iface config: mirror reconcile install", "iface", osName, "err", err)
			errs = append(errs, fmt.Errorf("%s mirror reconcile install: %w", osName, err))
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
