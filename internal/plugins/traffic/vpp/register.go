// Design: plan/learned/627-fw-7-traffic-vpp.md -- Backend registration

package trafficvpp

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/traffic"
)

func init() {
	if err := traffic.RegisterBackend("vpp", newBackend); err != nil {
		fmt.Fprintf(os.Stderr, "trafficvpp: registration failed: %v\n", err)
		os.Exit(1)
	}
	if err := traffic.RegisterVerifier("vpp", Verify); err != nil {
		fmt.Fprintf(os.Stderr, "trafficvpp: verifier registration failed: %v\n", err)
		os.Exit(1)
	}
}
