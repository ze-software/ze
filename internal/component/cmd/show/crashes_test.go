package show

import (
	"testing"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func TestShowCrashesRegistered(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:crashes" {
			if r.Handler == nil {
				t.Error("ze-show:crashes handler must not be nil")
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("ze-show:crashes not registered via pluginserver.RegisterRPCs")
	}
}
