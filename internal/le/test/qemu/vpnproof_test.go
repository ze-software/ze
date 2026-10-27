// Design: docs/architecture/testing/qemu-integration.md -- module-free VPN guest proof.
// Related: vpnproof.go -- emitted config requirements; dockerlab.go -- guest preparation.
package testqemu

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVPNBuiltinsRejectModularOrMissing consumes emitted configs rather than
// requested fragments, including an optional =y suffix in a manifest.
func TestVPNBuiltinsRejectModularOrMissing(t *testing.T) {
	for _, test := range []struct {
		name, config string
		valid        bool
	}{
		{"built in", "CONFIG_WIREGUARD=y\nCONFIG_CRYPTO_GCM=y\n", true},
		{"modular", "CONFIG_WIREGUARD=m\nCONFIG_CRYPTO_GCM=y\n", false},
		{"absent", "CONFIG_CRYPTO_GCM=y\n", false},
		{"substring", "CONFIG_WIREGUARD_EXTRA=y\nCONFIG_CRYPTO_GCM=y\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireVPNBuiltins(test.config, "# Built-ins only.\nCONFIG_WIREGUARD\nCONFIG_CRYPTO_GCM=y\n")
			if (err == nil) != test.valid {
				t.Fatalf("config verdict: %v; want valid=%t", err, test.valid)
			}
		})
	}
}

// TestGuestPreparationPrecedesServices executes the generated script with local
// service doubles. A refused preparation must start no service; a successful one
// must finish before either service is started. No host service or /run is touched.
func TestGuestPreparationPrecedesServices(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		name := "accepted"
		if rejected {
			name = "rejected"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "ready")
			calls := filepath.Join(dir, "calls")
			for tool, body := range map[string]string{
				"systemctl": "#!/bin/sh\nset -eu\ntest \"$(cat \"$VPN_MARKER\")\" = ready\nprintf '%s\\n' \"$*\" >> \"$VPN_CALLS\"\n",
				"touch":     "#!/bin/sh\nexit 0\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, tool), []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			preparation := "printf ready > \"$VPN_MARKER\""
			if rejected {
				preparation = "exit 9"
			}
			guest := dockerLabGuest{bootPreparation: preparation}
			var script string
			for _, file := range guest.cidataFiles() {
				if file.name == "boot.sh" {
					script = string(file.body)
				}
			}
			if script == "" {
				t.Fatal("guest has no boot script")
			}
			command := exec.CommandContext(t.Context(), "/bin/sh", "-c", script)
			command.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "VPN_MARKER="+marker, "VPN_CALLS="+calls)
			output, runErr := command.CombinedOutput()
			if rejected {
				if runErr == nil {
					t.Fatal("refused preparation exited successfully")
				}
				if _, err := os.Stat(calls); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("a service ran after refused preparation: %v", err)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("prepared guest script: %v: %s", runErr, output)
			}
			started, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(started)) != "start containerd.service\nstart docker.service" {
				t.Fatalf("service order: %s", started)
			}
		})
	}
}
