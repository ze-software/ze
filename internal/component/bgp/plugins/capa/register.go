// Design: docs/architecture/core-design.md -- core BGP capability decode plugin

package capa

import (
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-capa",
		Description:     "Core BGP capability decoding (multiprotocol, asn4, add-path, paths-limit, extended-nexthop, extended-message)",
		RFCs:            []string{"4760", "6793", "7911", "8654", "8950", "draft-abraitis-idr-addpath-paths-limit"},
		SupportsCapa:    true,
		Features:        "capa",
		CapabilityCodes: []uint8{1, 5, 6, 65, 69, 76},
		RunEngine: func(_ net.Conn) int {
			return 0
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return runDecodeMode(input, output)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.SupportsCapa = true
		cfg.RunDecode = runDecodeMode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bgp-capa: registration failed: %v\n", err)
		os.Exit(1)
	}
}
