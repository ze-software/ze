package mrt

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	reg := registry.Registration{
		Name:        "mrt",
		Description: "MRT routing information export (RFC 6396)",
		Features:    "yang",
		ConfigRoots: []string{"mrt"},
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "mrt: registration failed: %v\n", err)
		os.Exit(1)
	}
}
