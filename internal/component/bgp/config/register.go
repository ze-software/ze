// Design: docs/architecture/config/syntax.md -- BGP config registration hooks

package bgpconfig

import (
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/grmarker"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	zeconfig.RegisterPluginExtractor(extractBGPInlinePlugins)
	registry.RegisterReactorFactory(createReactorFromCoordinator)

	// Fill the always-on infra seams (internal/component/config/infra) so
	// `ze config dump|diff|validate` and the daemon reboot path reach BGP
	// behavior without importing this package. With ze_bgp off these stay
	// nil and every caller takes its no-BGP branch.
	infra.SetBGPTreeResolver(ResolveBGPTree)
	infra.SetBGPPeerValidator(validatePeersFromTree)
	infra.SetBGPRolelessPeerReporter(rolelessPeersFromTree)
	infra.SetGRMarkerWriter(writeGRMarker)

	// A peer that asks for BFD strict mode in a configuration with no BFD
	// engine never establishes at all, and nothing on its own surface says why.
	// bfd_strict_doctor.go reports that arrangement before the daemon starts.
	for _, meta := range bfdStrictDiagnosticCodes {
		_ = diagnostic.Register(meta)
	}
	if err := diagnostic.RegisterDoctorCheck(bfdStrictDoctorCheck); err != nil {
		doctorRegistrationFailed("bfd strict", err)
	}

	// A filter chain naming a policy nothing defines leaves the peer with a
	// shorter chain than the operator wrote. filter_reference_doctor.go reports
	// it from the tree, against the one filterInstanceName this package holds.
	if err := diagnostic.RegisterDoctorCheck(filterReferenceDoctorCheck); err != nil {
		doctorRegistrationFailed("filter reference", err)
	}

	// The peer resolution, TCP MD5, RFC 9234 role and capture directory checks
	// read BGP configuration, so this package owns them (doctor_checks.go).
	registerBGPDoctorChecks()
}

// doctorRegistrationFailed stops the process on a refused registration. Every
// refusal is a programmer error in the table beside the call -- a duplicate
// name, an unknown phase, a code without the doctor prefix -- so the daemon
// must not start one readiness check short of what it declares.
func doctorRegistrationFailed(check string, err error) {
	var tb textbuf.Buffer
	tb.Str("bgp: ").Str(check).Str(" doctor check registration failed: ").Err(err).Byte('\n')
	tb.StdErr() //nolint:errcheck // the process is exiting on the next line
	os.Exit(1)
}

// validatePeersFromTree adapts PeersFromConfigTree to the infra.BGPPeerValidator
// seam: callers use it purely as a gate, and the built []*reactor.PeerSettings
// is a BGP type the always-on side must not name.
func validatePeersFromTree(tree *zeconfig.Tree) error {
	_, err := PeersFromConfigTree(tree)
	return err
}

// writeGRMarker persists the RFC 4724 Restarting-Speaker marker so the next
// reactor start sets the R bit in its OPEN capabilities. Called through the
// infra.GRMarkerWriter seam on an operator-initiated restart or reboot.
func writeGRMarker(caps []plugin.InjectedCapability, store storage.Storage) {
	maxRestart := grmarker.MaxRestartTime(caps)
	if maxRestart <= 0 {
		return
	}
	expiresAt := time.Now().Add(time.Duration(maxRestart) * time.Second)
	log := slogutil.Logger("bgp.gr")
	if err := grmarker.Write(store, expiresAt); err != nil {
		log.Error("failed to write GR marker", "error", err)
		return
	}
	log.Info("GR marker written", "expires", expiresAt)
}

// bgpCaptureHandle is the method set the BGP config-transaction callbacks reach
// through a type assertion on the handle this factory returns
// (internal/component/bgp/plugin, captureBGPConfigEvent). That assertion FAILS
// OPEN: a handle that does not satisfy it records nothing and says nothing.
//
// The assertion below makes the failure a BUILD error here instead. It is the
// only place both types are visible: bgp/plugin cannot import bgp/reactor, and
// the interface there is unexported. Without it, wrapping the returned reactor
// would silently kill config event capture (spec improve-3, AC-6).
type bgpCaptureHandle interface {
	CapturesOpen() bool
	CaptureConfigEvent(op, txID string, payload []byte)
}

var _ bgpCaptureHandle = (*reactor.Reactor)(nil)

// createReactorFromCoordinator builds a BGP reactor using config state stored
// in the coordinator by the hub. This keeps bgp/config imports out of the hub.
func createReactorFromCoordinator(coord registry.CoordinatorAccessor) (registry.BGPReactorHandle, error) {
	bs := coord.Bootstrap()
	configPath := bs.ConfigPath
	cliPlugins := bs.CLIPlugins
	configData := bs.ConfigData

	// The hub records the selected source in stored history before startup.
	// A newly enabled BGP plugin uses the staged reload candidate; a restarted
	// plugin uses the accepted version. Neither reopens a mutable loose file.
	// Captured bytes remain authoritative for stdin and storeless startup.
	store := bs.Store
	data := configData
	if store != nil && configPath != "" && configPath != "-" {
		candidate, _, present, err := storage.ReadCandidateConfig(store, configPath)
		if err != nil {
			return nil, fmt.Errorf("read reactor candidate: %w", err)
		}
		if present {
			data = candidate
		} else {
			data, err = storage.ReadActiveConfig(store, configPath)
			if err != nil {
				return nil, fmt.Errorf("read accepted reactor config: %w", err)
			}
		}
	}

	// Set YANG validator for runtime attribute validation (origin enum, med/local-pref ranges).
	pluginYANG := plugin.CollectPluginYANG(cliPlugins)
	if v, vErr := zeconfig.YANGValidatorWithPlugins(pluginYANG); vErr == nil && v != nil {
		plugin.SetYANGValidator(v)
	}

	result, err := zeconfig.LoadConfig(string(data), configPath, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("parse config for reactor: %w", err)
	}

	// Production: borrow the hub-owned plugin server (standalone == false). The
	// hub injects it via SetPluginServerAny before StartWithContext.
	r, err := CreateReactor(result, configPath, store, false)
	if err != nil {
		return nil, err
	}

	// Chaos injection from hub-stored config.
	injectChaos(r, coord)

	// GR marker from storage (RFC 4724 Section 4.1). The tree goes with it:
	// the marker says Ze restarted, and the configuration says whether the
	// forwarding plane kept Ze's routes while it was down.
	readGRMarker(r, store, result.Tree.ToPluginMap())

	if cb := bs.HealthPeerCallback; cb != nil {
		r.AddPeerLifecycleCallback(cb)
	}

	// MRT bridges come from the registry seam (MRT self-registers them from its
	// init()), not from BGPBootstrap -- that is what keeps cmd/ze/hub free of an
	// internal/plugins/mrt import so //go:build ze_mrt can drop MRT. nil when MRT
	// is compiled out.
	if mcb := registry.GetMRTMessageCallback(); mcb != nil {
		r.AddMessageCallback(mcb)
	}
	if pcb := registry.GetMRTPeerCallback(); pcb != nil {
		r.AddPeerLifecycleCallback(pcb)
	}

	return r, nil
}
