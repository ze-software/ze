//go:build !linux

// Design: docs/labs/pppoe-interop.md -- exact-session probes run in Linux guests.
package testvpppppoe

import "errors"

type productFamilyWire struct {
	Request, Response []byte
	Received          bool
}

func productFamilyProbe(string, string, string, string, string, string) (productFamilyWire, error) {
	return productFamilyWire{}, errors.New("the family probe requires the Ze-kernel Linux guest")
}
