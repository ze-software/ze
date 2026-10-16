// Design: docs/architecture/hub-architecture.md -- daemon start sequence
// Related: registry.go -- Registration.BootEffect

package registry

import "sync"

// There is no ordering field and no reload-time call. An effect that needs
// either is not a boot effect.

var (
	bootMu sync.Mutex
	// bootEffectsRan guards the once-only contract: RunBootEffects runs the
	// effects on its first call and does nothing on any later call.
	bootEffectsRan bool
)

// RunBootEffects runs the BootEffect of every registered plugin once, in
// plugin name order, with the path of the config the daemon started from.
// The daemon start path MUST call it exactly once, after host tuning; a later
// call runs nothing, so a config reload cannot repeat a boot effect.
// Safe for concurrent use.
func RunBootEffects(configPath string) {
	bootMu.Lock()
	if bootEffectsRan {
		bootMu.Unlock()
		return
	}
	bootEffectsRan = true
	bootMu.Unlock()

	for _, reg := range All() {
		if reg.BootEffect != nil {
			reg.BootEffect(configPath)
		}
	}
}
