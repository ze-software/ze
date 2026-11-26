package l2tp

// Blank import triggers the schema package's init(), which registers
// `ze-l2tp-conf.yang` with the config/yang module registry.
import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/health"

	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
)

func init() {
	if err := config.RegisterServiceBlock(SubsystemName, "L2TP subsystem"); err != nil {
		panic("BUG: l2tp: service block registration: " + err.Error())
	}
	if err := config.RegisterServiceBlock("environment/l2tp", "L2TP subsystem"); err != nil {
		panic("BUG: l2tp: environment service block registration: " + err.Error())
	}
	// UDPListener.Start (listener.go) binds every IPv6 listener with
	// IPV6_V6ONLY, so `0.0.0.0` and `::` on one port both bind. Conflict
	// validation must know it, or it refuses the pair as a dual-stack overlap.
	config.RegisterListenerV6Only(SubsystemName)
	if err := config.RegisterScopedTreeValidator("l2tp-mru-policy", SubsystemName, validateMRUPolicy); err != nil {
		panic("BUG: registering l2tp MRU policy: " + err.Error())
	}
	if err := config.RegisterScopedTreeValidator("l2tp-auth-policy", SubsystemName, validateAuthPolicy); err != nil {
		panic("BUG: registering l2tp authentication policy: " + err.Error())
	}
	if err := config.RegisterScopedTreeValidator("l2tp-parameters", SubsystemName, validateParameters); err != nil {
		panic("BUG: registering l2tp parameters: " + err.Error())
	}
	if err := registry.RegisterDataPlaneFeature(registry.DataPlaneFeature{
		Name:       SubsystemName,
		ConfigPath: "environment l2tp server",
		Why:        "L2TP programs Linux PPP interfaces in the kernel only",
		Planes:     []string{"kernel"},
		InUseKind:  registry.DataPlaneInUsePredicate,
		InUse:      l2tpDataPlaneInUse,
	}); err != nil {
		panic("BUG: l2tp: data-plane feature registration failed: " + err.Error())
	}
	health.Register("l2tp", checkHealth)
	// Register the redistribute source at init so `import l2tp` resolves during
	// `ze config validate`, which imports plugins but does not start their engines.
	// Subsystem.Start also calls this; sync.Once makes the second call a no-op.
	registerL2TPSources()
}

// l2tpDataPlaneInUse reports whether tree configures an active L2TP listener.
//
// L2TP programs Linux PPP interfaces only when a listener starts a kernel
// worker (subsystem.go, prepareKernelBackend). The `l2tp` block alone also
// holds the subscriber authentication and address pool that PPPoE shares on
// VPP, so L2TP is in use only when at least one `environment l2tp server`
// entry exists and `l2tp enabled` is not false.
func l2tpDataPlaneInUse(tree map[string]any) registry.DataPlaneFeatureUse {
	l2tpEnv := configvalue.Section("environment/l2tp", tree)
	if l2tpEnv == nil {
		return registry.DataPlaneFeatureUse{}
	}
	if len(configvalue.ListEntries(l2tpEnv["server"])) == 0 {
		return registry.DataPlaneFeatureUse{}
	}
	if l2tpRoot := configvalue.Section("l2tp", tree); l2tpRoot != nil {
		if raw, present := l2tpRoot["enabled"]; present {
			if enabled, ok := configvalue.Bool(raw); ok {
				if !enabled {
					return registry.DataPlaneFeatureUse{}
				}
			}
		}
	}
	return registry.DataPlaneFeatureUse{InUse: true}
}
