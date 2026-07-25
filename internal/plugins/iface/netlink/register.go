package ifacenetlink

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	if err := iface.RegisterBackend("netlink", newNetlinkBackend); err != nil {
		fmt.Fprintf(os.Stderr, "iface-netlink: backend registration failed: %v\n", err)
		os.Exit(1)
	}
	// The macvlan capability doctor check (doctor.go) travels with the netlink
	// backend, so removing this package removes the check.
	if err := diagnostic.RegisterDoctorCheck(macvlanDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "iface-netlink: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}
