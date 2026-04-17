package config

import (
	"reflect"
	"testing"
)

// TestCollectContainerPathsUsesSlashSeparator locks the convention that
// CollectContainerPaths emits paths using the config package's PathSep
// ("/"), NOT the dot separator that a stale docstring used to claim.
// This matches the separator used by ExtractConfigSubtree and every
// other path consumer in the plugin server.
//
// VALIDATES: plugin auto-load contract -- plugin ConfigRoots must be
//
//	declared with "/" to match the present-path set that
//	CollectContainerPaths feeds into the auto-loader.
//
// PREVENTS:  silent auto-load failure when a plugin declares
//
//	"fib.vpp" (dot) and the matcher looks for "fib/vpp"
//	(slash). Before this test, fib-kernel, fib-p4, and
//	fib-vpp all used dotted ConfigRoots and silently never
//	auto-loaded, masked by the absence of any .ci test that
//	asserted on plugin lifecycle logs.
func TestCollectContainerPathsUsesSlashSeparator(t *testing.T) {
	tree := NewTree()
	fib := NewTree()
	kernel := NewTree()
	fib.SetContainer("kernel", kernel)
	vpp := NewTree()
	vpp.Set("enabled", "true")
	fib.SetContainer("vpp", vpp)
	tree.SetContainer("fib", fib)

	got := CollectContainerPaths(tree)
	want := []string{"fib", "fib/kernel", "fib/vpp"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("CollectContainerPaths:\n  got  %v\n  want %v", got, want)
	}
}
