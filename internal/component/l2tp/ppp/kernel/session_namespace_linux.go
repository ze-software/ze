// Design: docs/research/l2tpv2-ze-integration.md -- producer-held namespace files

//go:build linux

package kernel

import (
	"os"

	"github.com/vishvananda/netns"
)

// The actual producer MUST pin its OS thread through this capture and native
// creation. The original backend verifies that same thread/socket namespace.
func captureSessionNamespace() (*capturedSessionNamespace, error) {
	namespace, err := netns.Get()
	if err != nil {
		return nil, err
	}
	return &capturedSessionNamespace{file: os.NewFile(uintptr(namespace), "ppp.creator.netns")}, nil
}
