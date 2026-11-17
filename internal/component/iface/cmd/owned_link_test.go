//go:build linux

// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: owned_devices.go -- the link handler under test
// Related: ../owned_namespace_linux.go -- the inode check it reaches

package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The owner and prefix this test grants, a prefix no producer uses.
const (
	linkTestOwner  = "test-owned-link-owner"
	linkTestPrefix = "ztl"
)

// TestLinkRequestRefusesNamespaceMismatch proves the link handler compares the
// namespace inode the request carries with the namespace of the process id it
// names, and refuses a mismatch before any other state is read. Method: name
// this test's own process with its real inode plus one, expect the inode
// refusal; then name the real inode, and expect a refusal for another reason
// (no interface backend is loaded here), which proves the first refusal is
// tied to the inode and not to everything.
func TestLinkRequestRefusesNamespaceMismatch(t *testing.T) {
	iface.MustGrantOwnedDeviceNames(linkTestOwner, linkTestPrefix)
	if err := iface.CloseBackend(); err != nil {
		t.Fatalf("reset backend: %v", err)
	}
	info, err := os.Stat("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("read this process's namespace: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("the namespace answered no inode")
	}
	proc := process.NewProcess(plugin.PluginConfig{Name: linkTestOwner, Run: "test"})
	request := func(inode uint64) error {
		params, err := json.Marshal(rpc.OwnedLinkCreateInput{
			Key: "wire", Bridge: linkTestPrefix + "-net", Slot: 1, PID: os.Getpid(),
			NamespaceInode: inode, Name: "eth0", MTU: 1500,
		})
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}
		_, err = ownedLinkCreateHandler(nil, t.Context(), proc, params)
		return err
	}

	err = request(stat.Ino + 1)
	if err == nil {
		t.Fatalf("a request naming another namespace inode was accepted")
	}
	if !strings.Contains(err.Error(), "is not the one the request names") {
		t.Fatalf("the refusal is not the namespace mismatch: %v", err)
	}

	err = request(stat.Ino)
	if err == nil {
		t.Fatalf("a request with no interface backend loaded was accepted")
	}
	if strings.Contains(err.Error(), "is not the one the request names") {
		t.Fatalf("the matching inode was refused as a mismatch: %v", err)
	}
}
