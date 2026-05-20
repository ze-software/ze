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
}
