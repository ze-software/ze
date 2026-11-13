package appliance

import "testing"

// TestConfigUploadVerifyCommandNamesTheRunningConfig holds the advice the host
// driver prints after a refused or uncertain upload to the one command that
// answers the configuration the device applied. `show config` alone is a
// command group and answers nothing over SSH, which is what this advice named
// until a real guest refused it. The check is on the printed text because the
// host driver does not link the daemon's command registry.
func TestConfigUploadVerifyCommandNamesTheRunningConfig(t *testing.T) {
	cases := []struct {
		name, addr, user, want string
	}{
		{"address with port", "192.0.2.1:2222", "admin", "ssh -p 2222 admin@192.0.2.1 'show config running'"},
		{"address the driver cannot split", "192.0.2.1", "admin", "show config running"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := configUploadVerifyCommand(tc.addr, tc.user); got != tc.want {
				t.Fatalf("verify command = %q, want %q", got, tc.want)
			}
		})
	}
}
