// Design: docs/contributing/running-commands.md -- inherited seed controls.
package seedinit

import (
	"reflect"
	"testing"
)

// TestEnvironmentPreservesControls proves the launch cannot hide an override
// by deleting it, including empty values and dotted/uppercase spellings.
func TestEnvironmentPreservesControls(t *testing.T) {
	for _, key := range []string{"ZE_CONFIG_DIR", "ze.config.dir", "ZE_CRASH_DIR", "ze.storage.blob", "ZE_SESSION_SEED_DIRECTORY"} {
		input := []string{"PATH=/fixture", key + "="}
		before := append([]string(nil), input...)
		if _, err := Environment(input, "/fixture/config"); err == nil {
			t.Errorf("accepted control %s", key)
		}
		if !reflect.DeepEqual(input, before) {
			t.Errorf("changed caller environment: %v", input)
		}
	}
	input := []string{"PATH=/fixture", "OTHER=kept"}
	got, err := Environment(input, "/fixture/config")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PATH=/fixture", "OTHER=kept", DirectoryKey + "=/fixture/config"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("environment = %v, want %v", got, want)
	}
}
