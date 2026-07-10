package dataplane

import (
	"fmt"
	"os"
)

func init() {
	if err := Register("xfrm", newXFRMBackend); err != nil {
		fmt.Fprintf(os.Stderr, "dataplane: xfrm registration failed: %v\n", err)
		os.Exit(1)
	}
	if err := Register("vpp", newVPPBackend); err != nil {
		fmt.Fprintf(os.Stderr, "dataplane: vpp registration failed: %v\n", err)
		os.Exit(1)
	}
	// Test infrastructure: side-effect-free backend for unprivileged
	// control-plane tests (selected via ze.test.ike.dataplane=noop; see
	// noop.go for why EPERM on the real xfrm backend stays fatal instead).
	if err := Register("noop", newNoopBackend); err != nil {
		fmt.Fprintf(os.Stderr, "dataplane: noop registration failed: %v\n", err)
		os.Exit(1)
	}
}
