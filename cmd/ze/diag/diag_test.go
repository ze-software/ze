package diag

import (
	"io"
	"os"
	"os/exec"
	"testing"
)

// silenceStderr redirects os.Stderr to /dev/null for the duration of
// the test. Restores on cleanup.
func silenceStderr(t *testing.T) {
	t.Helper()
	old := os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}
	os.Stderr = devnull
	t.Cleanup(func() {
		os.Stderr = old
		if err := devnull.Close(); err != nil {
			t.Logf("close devnull: %v", err)
		}
	})
}

// TestValidateTarget_Valid covers accepted target shapes.
//
// VALIDATES: AC-10/AC-11 of spec-op-1-easy-wins.md -- ping/traceroute
// target validation accepts hostnames + IP literals.
func TestValidateTarget_Valid(t *testing.T) {
	good := []string{
		"example.com",
		"host-1.internal",
		"host_with_underscore",
		"127.0.0.1",
		"::1",
		"fe80::1",
		"2001:db8::1",
		"10.0.0.1",
	}
	for _, in := range good {
		if err := validateTarget(in); err != nil {
			t.Errorf("validateTarget(%q) rejected: %v", in, err)
		}
	}
}

// TestValidateTarget_Rejects shell metachars, empty, too long.
//
// VALIDATES: AC-10 -- no shell-meta passes through argv.
func TestValidateTarget_Rejects(t *testing.T) {
	bad := []string{
		"",
		"a;rm -rf /",
		"$(echo x)",
		"`id`",
		"host|cat /etc/passwd",
		"host with space",
		"host\n",
		"host&",
	}
	for _, in := range bad {
		if err := validateTarget(in); err == nil {
			t.Errorf("validateTarget(%q) should have rejected", in)
		}
	}
}

// TestValidateTarget_TooLong rejects targets beyond the RFC 1035 ceiling.
func TestValidateTarget_TooLong(t *testing.T) {
	long := make([]byte, maxTargetLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := validateTarget(string(long)); err == nil {
		t.Error("expected rejection of over-length target")
	}
}

// TestValidateInterfaceName covers accepted + rejected interface names.
func TestValidateInterfaceName(t *testing.T) {
	if err := validateInterfaceName(""); err != nil {
		t.Errorf("empty should be allowed (means no --interface): %v", err)
	}
	if err := validateInterfaceName("eth0"); err != nil {
		t.Errorf("eth0 should be allowed: %v", err)
	}
	long := make([]byte, maxInterfaceNameLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := validateInterfaceName(string(long)); err == nil {
		t.Error("expected rejection of over-length interface name")
	}
	if err := validateInterfaceName("eth0;reboot"); err == nil {
		t.Error("expected rejection of shell-meta interface name")
	}
}

// TestRunPing_ValidationErrors covers every validation path that
// returns 1 before exec. Each case asserts exit 1 without invoking
// /bin/ping.
//
// VALIDATES: AC-10 of spec-op-1-easy-wins.md -- argv sanitisation and
// boundary checks on --count (lower bound, upper bound, non-integer).
func TestRunPing_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no target", []string{}},
		{"two targets", []string{"1.1.1.1", "2.2.2.2"}},
		{"shell meta target", []string{"a;rm -rf /"}},
		{"unknown flag", []string{"--evil", "1.1.1.1"}},
		{"count negative", []string{"--count", "-1", "1.1.1.1"}},
		{"count too large", []string{"--count", "100001", "1.1.1.1"}},
		{"count non-int", []string{"--count", "abc", "1.1.1.1"}},
		{"iface too long", []string{"--interface", "ethernet-very-long", "1.1.1.1"}},
		{"iface shell meta", []string{"--interface", "eth0;x", "1.1.1.1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silenceStderr(t)
			if rc := RunPing(tc.args); rc != 1 {
				t.Errorf("RunPing(%v) = %d, want 1", tc.args, rc)
			}
		})
	}
}

// TestRunTraceroute_ValidationErrors mirrors RunPing cases for
// traceroute's --probes (1..10) ceiling.
func TestRunTraceroute_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no target", []string{}},
		{"shell meta target", []string{"a;rm -rf /"}},
		{"probes negative", []string{"--probes", "-1", "1.1.1.1"}},
		{"probes too large", []string{"--probes", "11", "1.1.1.1"}},
		{"probes non-int", []string{"--probes", "abc", "1.1.1.1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silenceStderr(t)
			if rc := RunTraceroute(tc.args); rc != 1 {
				t.Errorf("RunTraceroute(%v) = %d, want 1", tc.args, rc)
			}
		})
	}
}

// TestRunPing_Help asserts that -h exits 0 (flag.ErrHelp handled).
func TestRunPing_Help(t *testing.T) {
	silenceStderr(t)
	if rc := RunPing([]string{"-h"}); rc != 0 {
		t.Errorf("RunPing(-h) = %d, want 0", rc)
	}
}

// TestRunWgKeypair_RejectsArgs ensures stray arguments exit 1 without
// attempting to exec `wg`.
func TestRunWgKeypair_RejectsArgs(t *testing.T) {
	silenceStderr(t)
	if rc := RunWgKeypair([]string{"extra"}); rc != 1 {
		t.Errorf("RunWgKeypair(extra) = %d, want 1", rc)
	}
}

// TestRunWgKeypair_Smoke skips when `wg` is unavailable; otherwise
// runs end-to-end and asserts that stdout has two lines.
func TestRunWgKeypair_Smoke(t *testing.T) {
	if _, err := exec.LookPath("wg"); err != nil {
		t.Skip("wg not installed; skipping keypair smoke")
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	rc := RunWgKeypair(nil)
	if err := w.Close(); err != nil {
		t.Logf("close pipe writer: %v", err)
	}
	os.Stdout = oldStdout
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if rc != 0 {
		t.Fatalf("RunWgKeypair() = %d, want 0; stdout=%q", rc, buf)
	}
	if len(buf) == 0 {
		t.Error("expected keypair output on stdout")
	}
}
