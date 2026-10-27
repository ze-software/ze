// Design: docs/architecture/testing/interop.md -- installed ESP transform and received traffic proof.
// Related: helpers.go -- directed kernel counters and real tunnel traffic.
package ipsec

import (
	"context"
	"fmt"
	"strings"
)

// checkESPKernel binds a scenario's negotiated cipher and installed transforms.
// It reads both installed directions and then requires received traffic.
func checkESPKernel(encryption string, transforms ...string) scenarioChecker {
	return func(ctx context.Context, lab *scenarioLab) error {
		if err := establish(ctx, lab); err != nil {
			return err
		}
		negotiated, err := lab.zeChildESPEncryption(ctx, swanConfigPeer)
		if err != nil {
			return err
		}
		if negotiated != encryption {
			return fmt.Errorf("negotiated ESP encryption %q, want %s", negotiated, encryption)
		}
		for _, peer := range []string{zePeer, swanPeer} {
			if _, err := lab.waitXFRM(ctx, peer); err != nil {
				return err
			}
			for _, direction := range [][2]string{{zeIP, swanIP}, {swanIP, zeIP}} {
				state, err := lab.inboundXFRMState(ctx, peer, direction[0], direction[1])
				if err != nil {
					return err
				}
				if err := requireESPKernelTransforms(state, transforms); err != nil {
					return fmt.Errorf("%s: %w", peer, err)
				}
			}
		}
		return lab.verifyTunnelTraffic(ctx, encryption+" traffic did not reach its receiver")
	}
}

func requireESPKernelTransforms(state string, transforms []string) error {
	for _, transform := range transforms {
		found := false
		for field := range strings.FieldsSeq(state) {
			if field == transform {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("installed ESP state lacks %s: %s", transform, state)
		}
	}
	return nil
}
