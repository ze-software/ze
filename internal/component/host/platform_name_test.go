package host

import "testing"

// TestValidPlatformName pins the platform-name membership test that doctor and
// diagnostic use to validate registered DoctorCheck platforms.
//
// VALIDATES: ValidPlatformName accepts exactly the registered PlatformType
// names (every PlatformType.String() output) and rejects everything else.
// PREVENTS: the doctor and diagnostic platform validators re-enumerating and
// drifting from the host platform set — both now derive validity from here, so
// a new PlatformType is recognized everywhere by registering it once.
func TestValidPlatformName(t *testing.T) {
	for _, p := range []PlatformType{
		PlatformUnknown, PlatformGokrazy, PlatformSystemd,
		PlatformContainer, PlatformPlainLinux, PlatformDarwin,
	} {
		if !ValidPlatformName(p.String()) {
			t.Errorf("ValidPlatformName(%q) = false for PlatformType %d, want true", p.String(), p)
		}
	}

	for _, name := range []string{"", "any", "linux", "Gokrazy", "plainlinux", "windows"} {
		if ValidPlatformName(name) {
			t.Errorf("ValidPlatformName(%q) = true, want false", name)
		}
	}
}
