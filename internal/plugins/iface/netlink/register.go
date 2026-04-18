package ifacenetlink

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func init() {
	if err := iface.RegisterBackend("netlink", newNetlinkBackend); err != nil {
		fmt.Fprintf(os.Stderr, "iface-netlink: backend registration failed: %v\n", err)
		os.Exit(1)
	}
}
