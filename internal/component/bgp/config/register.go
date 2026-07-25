// Design: docs/architecture/config/syntax.md -- BGP config registration hooks

package bgpconfig

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/grmarker"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/core/slogutil"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

var errBgpCoordinatorMissingBgpStore = errors.New("bgp: coordinator missing bgp.store")

func init() {
	zeconfig.RegisterPluginExtractor(extractBGPInlinePlugins)
	registry.RegisterReactorFactory(createReactorFromCoordinator)

	// Fill the always-on infra seams (internal/component/config/infra) so
	// `ze config dump|diff|validate` and the daemon reboot path reach BGP
	// behavior without importing this package. With ze_bgp off these stay
	// nil and every caller takes its no-BGP branch.
	infra.SetBGPTreeResolver(ResolveBGPTree)
	infra.SetBGPPeerValidator(validatePeersFromTree)
	infra.SetGRMarkerWriter(writeGRMarker)
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

// createReactorFromCoordinator builds a BGP reactor using config state stored
// in the coordinator by the hub. This keeps bgp/config imports out of the hub.
func createReactorFromCoordinator(coord registry.CoordinatorAccessor) (registry.BGPReactorHandle, error) {
	bs := coord.Bootstrap()
	configPath := bs.ConfigPath
	cliPlugins := bs.CLIPlugins
	configData := bs.ConfigData

	store := bs.Store
	if store == nil {
		return nil, errBgpCoordinatorMissingBgpStore
	}

	// Re-read config from disk for reload support. Stdin uses captured data.
	// Mirrors the hub's initial-load fallback (cmd/ze/hub/main.go Run): try the
	// blob store first, and if the store is blob-only (e.g., gokrazy read-only
	// root) fall back to a direct filesystem read. Without this fallback, all
	// encode/plugin .ci tests that pass a /tmp/... config path via `ze <file>`
	// fail with "read file/active/...: file does not exist" because the
	// filesystem path is not a valid blob key.
	var data []byte
	if configPath != "" && configPath != "-" && store != nil {
		var err error
		data, err = store.ReadFile(configPath)
		if err != nil && storage.IsBlobStorage(store) {
			data, err = os.ReadFile(configPath) //nolint:gosec // path supplied by the daemon operator
		}
		if err != nil {
			return nil, fmt.Errorf("re-read config for reactor: %w", err)
		}
	} else {
		data = configData
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

	// GR marker from storage (RFC 4724 Section 4.1).
	readGRMarker(r, store)

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
