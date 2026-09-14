// Design: docs/features/ai-first.md -- the readiness checks the BGP engine owns
// Overview: register.go -- the init() that installs every entry below
// Related: bfd_strict_doctor.go -- the fifth entry, declared beside its own codes
// Related: internal/component/doctor/doctor.go -- the runner that reads the registry
//
// A doctor check belongs to the package that owns the runtime dependency it
// probes (ai/patterns/registration.md, "Doctor Check Registry"). These four
// probe BGP configuration, so dropping the BGP engine from a build drops them
// with it rather than leaving `ze doctor` calling into a component that is not
// there.
//
// They arrive from internal/component/doctor, where the runner called each one
// by name. The Order of each entry reproduces the position it held in that
// sequence: an entry the runner called BEFORE its registry dispatch keeps a
// low order, and one it called after keeps an order on the 2100 scale the
// doctor-owned table already uses. What the phase guarantees is that the config
// is loaded; order inside a phase decides only the order the diagnostics print
// in.
//
// Every one of them is unreachable on a build without the ze_bgp tag, because
// `ze-bgp-conf.yang` is registered from internal/component/bgp/yang, which only
// all_ze_bgp.go imports. Such a build refuses a configuration carrying a `bgp`
// block before any check sees it, so moving them behind the same tag drops
// nothing an operator could have been told.

package bgpconfig

import (
	"os"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// diagnosticBGPPeerConfig names a peer configuration the engine refuses.
const diagnosticBGPPeerConfig = "doctor-config-bgp-peer"

// diagnosticBGPMD5 names a TCP-MD5 configuration fault an operator sees in
// `ze doctor` output.
const diagnosticBGPMD5 = "doctor-bgp-md5"

// diagnosticBGPPeerNoRole names a peer or a dynamic group that declares no
// RFC 9234 role.
const diagnosticBGPPeerNoRole = "doctor-bgp-peer-no-role"

// diagnosticBGPCaptureDirectory names a capture directory the daemon cannot use.
const diagnosticBGPCaptureDirectory = "doctor-bgp-capture-directory"

// captureDirPerm mirrors the mode the reactor uses (reactor/capture_replay.go,
// captureDirPerm): a capture file holds a peer's routing data, so the directory
// is not world-readable. The probe below creates the directory exactly as the
// daemon does, so it must create it with the same mode.
const captureDirPerm = 0o750

// bgpDoctorChecks are the readiness checks this package owns, in the order the
// doctor runner ran them.
var bgpDoctorChecks = []diagnostic.DoctorCheck{{
	Name:         "bgp-peer-config",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        110,
	Component:    "bgp",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnosticBGPPeerConfig},
	Check:        doctorCheckBGPPeerConfig,
}, bfdStrictDoctorCheck, {
	Name:         "bgp-md5",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2120,
	Component:    "bgp",
	Dependencies: []string{"kernel"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnosticBGPMD5},
	Check:        doctorCheckBGPMD5,
}, {
	Name:         "bgp-peer-no-role",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2130,
	Component:    "bgp",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnosticBGPPeerNoRole},
	Check:        doctorCheckBGPPeersWithoutRole,
}, {
	Name:         "bgp-capture-directory",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2280,
	Component:    "bgp",
	Dependencies: []string{"filesystem"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnosticBGPCaptureDirectory},
	Check:        doctorCheckBGPCaptureDirectory,
}}

// registerBGPDoctorChecks installs every entry above. A refusal is a programmer
// error in the table beside it -- a duplicate name, a phase that does not
// exist, a code without the doctor prefix -- and none of them can be reached
// from a config or a peer, so it stops the process rather than leaving
// `ze doctor` quietly short of a check.
func registerBGPDoctorChecks() {
	for i := range bgpDoctorChecks {
		if err := diagnostic.RegisterDoctorCheck(bgpDoctorChecks[i]); err != nil {
			panic("BUG: bgp doctor check registration refused: " + err.Error())
		}
	}
}

// doctorTree answers the config tree a registered check reads.
//
// A nil Tree is a legitimate value: the missing-config phase runs with no
// config loaded. A Tree that holds something other than *config.Tree can only
// come from a runner defect, so it panics rather than returning a nil tree the
// check would read as "no config" (docs/contributing/ze-go-style.md, "A zero
// value is never an answer").
func doctorTree(ctx diagnostic.DoctorCheckContext) *config.Tree {
	if ctx.Tree == nil {
		return nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		panic("BUG: doctor check context carries a tree that is not *config.Tree")
	}
	return tree
}

// doctorCheckBGPPeerConfig runs the engine's own peer resolution over the tree,
// so `ze doctor` refuses exactly the configurations the daemon refuses.
//
// The tree is CLONED before it is handed over. config.PruneInactive is
// documented at internal/component/config/prune.go as "modified in place. Call
// on a clone if the original must be preserved", and the doctor runner shares
// one tree across every later check: validating in place silently deleted each
// `inactive:` peer from underneath them.
//
// The severity is ERROR, so the report is not ready and `ze doctor` exits 1
// (owner decision, 2026-07-27). The daemon will not start on this config, and a
// readiness check that answers "ready" for a config the engine refuses is the
// operator trap this check exists to close.
func doctorCheckBGPPeerConfig(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil || tree.GetContainer("bgp") == nil {
		return nil
	}
	err := validatePeersFromTree(tree.Clone())
	if err == nil {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     diagnosticBGPPeerConfig,
		Severity: diagnostic.SeverityError,
		Message:  tb.Str("bgp peer configuration rejected (the daemon will not start on it): ").Err(err).String(),
	}}
}

// doctorCheckBGPPeersWithoutRole enumerates the peers and the dynamic groups
// that declare no RFC 9234 role. Such a peer is accepted -- the transit-leak
// filter obligation (RFC 7454 Section 9) binds only a peer that declares a
// relationship -- so the operator is told which sessions no relationship was
// stated for, and the gap is a decision they can see rather than a silence.
//
// A WARNING, not an error: the config loads and the daemon starts on it. The
// engine names the same peers in one aggregated line at config load; this check
// is the door an operator opens on purpose.
//
// The tree is CLONED for the reason doctorCheckBGPPeerConfig records: the
// reporter prunes inactive nodes in place.
func doctorCheckBGPPeersWithoutRole(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil || tree.GetContainer("bgp") == nil {
		return nil
	}
	names := rolelessPeersFromTree(tree.Clone())
	if len(names) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     diagnosticBGPPeerNoRole,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("peers and dynamic groups declare no RFC 9234 role, so no transit-leak filter is required of them: ").
			Str(textbuf.Join(names, ", ")).String(),
	}}
}

// doctorCheckBGPMD5 reports the first peer or group that asks for TCP MD5 on a
// platform whose kernel cannot sign the segments. One finding is enough: the
// platform, not the peer, is what the operator has to change.
func doctorCheckBGPMD5(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if network.TCPMD5Supported() {
		return nil
	}
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	var tb textbuf.Buffer
	for _, p := range bgp.GetListOrdered("peer") {
		if md5Configured(nil, p.Value) {
			return []diagnostic.Diagnostic{md5Unsupported(tb.Reset().Str("BGP peer ").Str(p.Key))}
		}
	}
	for _, g := range bgp.GetListOrdered("group") {
		if md5Configured(nil, g.Value) {
			return []diagnostic.Diagnostic{md5Unsupported(tb.Reset().Str("BGP group ").Str(g.Key))}
		}
		for _, p := range g.Value.GetListOrdered("peer") {
			if md5Configured(g.Value, p.Value) {
				return []diagnostic.Diagnostic{md5Unsupported(tb.Reset().Str("BGP peer ").Str(g.Key).Byte('/').Str(p.Key))}
			}
		}
	}
	return nil
}

// md5Unsupported completes the message the caller has started with the subject
// name, so every arm of the walk above states the fault in one wording.
func md5Unsupported(tb *textbuf.Buffer) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     diagnosticBGPMD5,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str(" requires TCP MD5 but platform does not support it").String(),
	}
}

// md5Configured reports whether a node asks for TCP MD5, reading the group's
// value when the peer states none. The group is the only level a peer inherits
// from here, which is the same imprecision the AS112 coordination check accepts.
func md5Configured(parent, node *config.Tree) bool {
	if password, ok := md5Password(node); ok {
		return password != ""
	}
	password, ok := md5Password(parent)
	return ok && password != ""
}

// md5Password answers the `connection { md5 { password ... } }` leaf of one node.
func md5Password(node *config.Tree) (string, bool) {
	if node == nil {
		return "", false
	}
	connection := node.GetContainer("connection")
	if connection == nil {
		return "", false
	}
	md5 := connection.GetContainer("md5")
	if md5 == nil {
		return "", false
	}
	return md5.Get("password")
}

// doctorCheckBGPCaptureDirectory reports every peer whose capture is enabled but
// whose capture directory cannot be written.
//
// It probes only peers that opted in. A disabled capture names no dependency, so
// a directory that does not exist is not a finding.
func doctorCheckBGPCaptureDirectory(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}
	var diags []diagnostic.Diagnostic
	seen := make(map[string]bool)

	probe := func(peerName string, peer *config.Tree) {
		capture := peer.GetContainer("capture")
		if !captureEnabled(capture) {
			return
		}
		dir := reactor.DefaultCaptureDirectory
		if configured, ok := capture.Get("directory"); ok && configured != "" {
			dir = configured
		}
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		if err := probeCaptureDirectory(dir); err != nil {
			var tb textbuf.Buffer
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnosticBGPCaptureDirectory,
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

// captureEnabled reports whether a peer opted into protocol event capture.
// Capture is off unless the peer says otherwise, so an absent container and an
// absent leaf both answer false.
func captureEnabled(capture *config.Tree) bool {
	if capture == nil {
		return false
	}
	enabled, ok := capture.Get("enabled")
	return ok && enabled == "true"
}

// probeCaptureDirectory creates the directory if absent, then probes it. The
// daemon does exactly this at session start, so the check must too: probing
// without creating would report every first run as broken.
func probeCaptureDirectory(dir string) error {
	if err := os.MkdirAll(dir, captureDirPerm); err != nil {
		return err
	}
	return diagnostic.DoctorProbeWritableDir(dir)
}
