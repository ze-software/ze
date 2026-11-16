// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device
// Detail: index.go -- the module index this pass reads
// Detail: glob.go -- the alias pattern match
// Detail: sysfs.go -- the device walk
// Related: register_linux.go -- the start step that runs this pass on the appliance
// Related: finit_linux.go -- the finit_module load the pass is handed

// Package coldplug loads the kernel driver modules the devices present at
// boot need. The appliance has no udev and no modprobe, so without it a NIC
// whose driver the kernel builds as a module has no driver and no interface.
//
// One pass, cold-plug only: walk the sysfs device tree, read each device's
// modalias, match it against the patterns in modules.alias, and load each
// matching module after the modules modules.dep and modules.softdep say it
// needs, skipping a module that is built in or already loaded. A device that
// appears after the pass gets nothing from it.
package coldplug

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// loadFunc loads one module file, named by its path. It answers
// errAlreadyLoaded when the kernel already holds the module, and any other
// error when the load failed. The pass takes it as a parameter so a test
// needs no privilege and loads nothing.
type loadFunc func(file string) error

// errAlreadyLoaded says the kernel already holds the module: something loaded
// it after the pass read the loaded-module list. The pass counts it as
// loaded, not as a failure.
var errAlreadyLoaded = errors.New("kernel module already loaded")

// passOutcome is how far one pass got. The zero value is no outcome.
type passOutcome uint8

const (
	passUnspecified passOutcome = iota
	// passNoIndex: the kernel has no module index, so there is nothing to
	// load and nothing is wrong.
	passNoIndex
	// passIndexBroken: an index file is present and unreadable or malformed;
	// logged, and nothing loaded.
	passIndexBroken
	// passDevicesUnreadable: the sysfs device tree could not be walked;
	// logged, and nothing loaded.
	passDevicesUnreadable
	// passRan: the pass matched the devices and loaded what they need.
	passRan
)

// passReport is what one pass did. loaded holds the modules it loaded, in
// load order; failed holds the modules a present device needs that are not
// loaded, in the order the pass met them.
type passReport struct {
	outcome passOutcome
	loaded  []string
	failed  []string
}

// fixKernelLog is the fix a failed load names: the kernel prints the module's
// own reason (an unknown symbol, a version mismatch, a refused signature).
const fixKernelLog = "read the module's own reason in the kernel log: show system kernel-log level warning"

// rebuildCommands are the two commands that replace the image on a device, as
// an operator types them on the build machine (docs/guide/appliance.md). The
// device's own ze does not carry `appliance`.
const rebuildCommands = "./ze-host appliance build <name> then ./ze-host appliance push <name>"

// fixRebuild is the fix an index fault names: the image's module tree is not
// the one its kernel build wrote.
const fixRebuild = "on the build machine, rebuild the appliance image so its /lib/modules tree is whole and push it: " + rebuildCommands

// fixUncompressed is the fix a compressed module names: finit_module is called
// with no flags, so the kernel refuses a compressed file.
const fixUncompressed = "on the build machine, build the kernel with CONFIG_MODULE_COMPRESS_NONE=y, then rebuild the appliance image and push it: " + rebuildCommands

// coldplug runs one cold-plug pass over the devices under sysRoot and the
// module index under modulesRoot (/sys and /lib/modules/<release> on a
// device). loaded is the set of modules the kernel already holds; a nil set
// is a list that could not be read, and the pass then relies on load
// answering errAlreadyLoaded. Every fault is logged with its fix and never
// stops the pass: one module that does not load costs its own device only.
func coldplug(sysRoot, modulesRoot string, loaded map[string]bool, load loadFunc, log *slog.Logger) passReport {
	idx, err := readIndex(modulesRoot)
	if errors.Is(err, errNoIndex) {
		log.Debug("no kernel module index for this kernel, so no driver module is loaded", "reason", err)
		return passReport{outcome: passNoIndex}
	}
	if err != nil {
		log.Warn("the kernel module index cannot be read, so no driver module is loaded for the devices present",
			"code", diagnostic.CodeDoctorModuleMissing, "modules", modulesRoot, "error", err, "fix", fixRebuild)
		return passReport{outcome: passIndexBroken}
	}
	walk, err := walkDevices(sysRoot)
	if err != nil {
		log.Warn("the device tree cannot be read, so no driver module is loaded for the devices present",
			"code", diagnostic.CodeDoctorModuleMissing, "sysfs", sysRoot, "error", err, "fix", fixKernelLog)
		return passReport{outcome: passDevicesUnreadable}
	}
	if walk.unreadable > 0 {
		log.Warn("some device entries cannot be read, and a device under them gets no driver module",
			"code", diagnostic.CodeDoctorModuleMissing, "unreadable", walk.unreadable,
			"first-error", walk.firstUnreadable, "fix", fixKernelLog)
	}
	p := newPlanner(idx, loaded, log)
	for _, device := range walk.aliases {
		p.addDevice(device)
	}
	return runPlan(p, modulesRoot, load, log)
}

// plannedModule is one module the pass loads: its name, its index entry, and
// the device modalias that asked for it, which for a dependency is the device
// that asked for the module needing it.
type plannedModule struct {
	name   string
	file   moduleFile
	device string
}

// planner orders the modules the devices need. It is used by one pass on one
// goroutine.
type planner struct {
	idx    moduleIndex
	loaded map[string]bool
	log    *slog.Logger
	// placed holds every module the planner has decided about: ordered,
	// skipped as built in or loaded, or missing. A module is placed before
	// its dependencies are visited, which is what ends a cycle.
	placed map[string]bool
	order  []plannedModule
	// missing holds the modules a device or a dependency names that the
	// index does not hold.
	missing []string
}

func newPlanner(idx moduleIndex, loaded map[string]bool, log *slog.Logger) *planner {
	return &planner{idx: idx, loaded: loaded, log: log, placed: make(map[string]bool)}
}

// addDevice places every module whose alias pattern matches the device's
// modalias. A device matching several patterns gets every module they name,
// as modprobe loads every match.
func (p *planner) addDevice(device string) {
	for _, rule := range p.idx.aliases {
		if !strings.HasPrefix(device, rule.literal) {
			continue
		}
		if !aliasMatch(rule.pattern, device) {
			continue
		}
		p.add(rule.module, device, false)
	}
}

// add places name after the modules it needs: its soft "pre" modules, then
// its modules.dep dependencies, then itself, then its soft "post" modules. A
// soft dependency the index does not hold is skipped in silence, because the
// module works without it; a hard one is reported.
//
// The recursion follows the index the kernel build wrote, not anything a peer
// sends, and it is bounded by the number of modules in that index: each
// module is placed before add recurses into it, so it is visited once.
func (p *planner) add(name, device string, soft bool) {
	if p.placed[name] {
		return
	}
	p.placed[name] = true
	if p.idx.builtin[name] {
		return
	}
	if p.loaded[name] {
		return
	}
	file, known := p.idx.modules[name]
	if !known {
		if soft {
			return
		}
		p.missing = append(p.missing, name)
		p.log.Warn("a present device needs a kernel module the module index does not hold, so the device has no driver",
			"code", diagnostic.CodeDoctorModuleMissing, "module", name, "device", device, "fix", fixRebuild)
		return
	}
	deps := p.idx.softdeps[name]
	for _, pre := range deps.pre {
		p.add(pre, device, true)
	}
	for _, dep := range file.deps {
		p.add(dep, device, false)
	}
	p.order = append(p.order, plannedModule{name: name, file: file, device: device})
	for _, post := range deps.post {
		p.add(post, device, true)
	}
}

// runPlan loads the planned modules in order. A module whose hard dependency
// did not load is not tried, because the kernel would refuse it for the
// symbols it lacks, and it is reported with the dependency that blocked it.
func runPlan(p *planner, modulesRoot string, load loadFunc, log *slog.Logger) passReport {
	report := passReport{outcome: passRan, failed: p.missing}
	failed := make(map[string]bool, len(p.missing))
	for _, name := range p.missing {
		failed[name] = true
	}
	for _, module := range p.order {
		if blocker, blocked := firstFailed(module.file.deps, failed); blocked {
			failed[module.name] = true
			report.failed = append(report.failed, module.name)
			log.Warn("a kernel driver module was not loaded because a module it needs did not load",
				"code", diagnostic.CodeDoctorModuleMissing, "module", module.name, "needs", blocker,
				"device", module.device, "fix", fixKernelLog)
			continue
		}
		if !strings.HasSuffix(module.file.path, ".ko") {
			failed[module.name] = true
			report.failed = append(report.failed, module.name)
			log.Warn("a kernel driver module is compressed, and Ze loads only uncompressed modules, so its device has no driver",
				"code", diagnostic.CodeDoctorModuleMissing, "module", module.name, "file", module.file.path,
				"device", module.device, "fix", fixUncompressed)
			continue
		}
		err := load(filepath.Join(modulesRoot, module.file.path))
		if errors.Is(err, errAlreadyLoaded) {
			continue
		}
		if err != nil {
			failed[module.name] = true
			report.failed = append(report.failed, module.name)
			log.Warn("a kernel driver module did not load, so the device that needs it has no driver",
				"code", diagnostic.CodeDoctorModuleMissing, "module", module.name, "file", module.file.path,
				"device", module.device, "error", err, "fix", fixKernelLog)
			continue
		}
		report.loaded = append(report.loaded, module.name)
	}
	if len(report.loaded) > 0 {
		log.Info("loaded the kernel driver modules the present devices need", "modules", strings.Join(report.loaded, " "))
	}
	return report
}

// firstFailed answers the first of deps that failed, and whether any did.
func firstFailed(deps []string, failed map[string]bool) (string, bool) {
	for _, dep := range deps {
		if failed[dep] {
			return dep, true
		}
	}
	return "", false
}
