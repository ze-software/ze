//go:build linux

// VALIDATES: spec-ospf-ext-16 -- /proc/net/xfrm_stat parsing maps the kernel XFRM
// inbound-drop counters to the RFC 4552 §3/§4 silent-discard reasons (no-policy,
// auth-failed) that back ze_ospfv3_ipsec_kernel_drops_total.
// PREVENTS: a mis-parsed procfs line inflating or dropping the kernel-drop metric.
package ospf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadXfrmDrops(t *testing.T) {
	fixture := "XfrmInError\t0\n" +
		"XfrmInNoPols\t5\n" +
		"XfrmInStateProtoError\t3\n" +
		"XfrmInIntegFailures\t2\n" +
		"XfrmOutError\t7\n"
	path := filepath.Join(t.TempDir(), "xfrm_stat")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	old := xfrmStatPath
	xfrmStatPath = path
	defer func() { xfrmStatPath = old }()

	drops, err := readXfrmDropsPlatform()
	if err != nil {
		t.Fatalf("readXfrmDropsPlatform: %v", err)
	}
	if drops["no-policy"] != 5 {
		t.Errorf("no-policy = %d, want 5", drops["no-policy"])
	}
	// XfrmInStateProtoError (3) + XfrmInIntegFailures (2) both map to auth-failed.
	if drops["auth-failed"] != 5 {
		t.Errorf("auth-failed = %d, want 5", drops["auth-failed"])
	}
}
