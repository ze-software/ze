package registry

import (
	"net"
	"testing"
)

// TestBootEffectsRunOnce verifies each registered plugin's BootEffect runs
// once, with the config name and text, in plugin name order, that a plugin with none
// is passed over, and that a second RunBootEffects (a reload) runs nothing.
//
// VALIDATES: AC-14's once-only daemon-start contract of RunBootEffects.
// PREVENTS: a config reload re-running crash harvest, or an effect skipped.
// Not parallel: it replaces the process-wide registry.
func TestBootEffectsRunOnce(t *testing.T) {
	saved := Snapshot()
	Reset()
	bootMu.Lock()
	savedRan := bootEffectsRan
	bootEffectsRan = false
	bootMu.Unlock()
	t.Cleanup(func() {
		Restore(saved)
		bootMu.Lock()
		bootEffectsRan = savedRan
		bootMu.Unlock()
	})

	var calls []string
	register := func(name string, effect func(string, []byte)) {
		t.Helper()
		if err := Register(Registration{
			Name:        name,
			Description: "boot effect test plugin",
			RunEngine:   func(net.Conn) int { return 0 },
			CLIHandler:  func([]string) int { return 0 },
			BootEffect:  effect,
		}); err != nil {
			t.Fatalf("Register(%q): %v", name, err)
		}
	}
	register("zeta", func(configName string, configData []byte) {
		calls = append(calls, "zeta:"+configName+":"+string(configData))
	})
	register("alpha", func(configName string, configData []byte) {
		calls = append(calls, "alpha:"+configName+":"+string(configData))
	})
	register("middle", nil)

	RunBootEffects("/etc/ze/ze.conf", []byte("first"))
	RunBootEffects("/etc/ze/other.conf", []byte("second"))

	want := []string{"alpha:/etc/ze/ze.conf:first", "zeta:/etc/ze/ze.conf:first"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	}
}
