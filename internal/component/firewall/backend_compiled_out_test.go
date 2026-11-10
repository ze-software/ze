package firewall

import "testing"

// TestLoadBackendCompiledOutNamesTag proves a backend whose provider plugin
// this build lacks is refused naming the build tag and both fixes, not as an
// unknown backend. Method: this package's test binary links the traffic
// registration (its ConfigNeed on `backend vpp`) and no vpp plugin, exactly
// as a build without ze_vpp; the full three-part text is compared.
func TestLoadBackendCompiledOutNamesTag(t *testing.T) {
	err := LoadBackend(backendVPP)
	if err == nil {
		t.Fatal("LoadBackend(vpp) succeeded in a build with no vpp plugin")
	}
	want := `firewall: "firewall backend vpp" names a backend this build of ze was compiled without` + "\n" +
		"the plugin that provides it (vpp) is not in this build, so firewall cannot load the backend\n" +
		`fix: rebuild ze with build tag ze_vpp added to its -tags list, or remove "firewall backend vpp" from the config`
	if got := err.Error(); got != want {
		t.Errorf("refusal =\n%s\nwant\n%s", got, want)
	}
}
