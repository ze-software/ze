package trafficnetlink

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

func init() {
	if err := traffic.RegisterBackend("tc", newBackend); err != nil {
		fmt.Fprintf(os.Stderr, "trafficnetlink: registration failed: %v\n", err)
		os.Exit(1)
	}
}
