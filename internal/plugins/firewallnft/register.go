package firewallnft

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

func init() {
	if err := firewall.RegisterBackend("nft", newBackend); err != nil {
		fmt.Fprintf(os.Stderr, "firewallnft: registration failed: %v\n", err)
		os.Exit(1)
	}
}
