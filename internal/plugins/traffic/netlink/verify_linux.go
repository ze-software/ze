// Design: docs/guide/traffic-control.md -- tc rate admission before kernel mutation

//go:build linux

package trafficnetlink

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/traffic"
)

func init() {
	if err := traffic.RegisterVerifier("tc", verifyDesired); err != nil {
		fmt.Fprintf(os.Stderr, "trafficnetlink: verifier registration failed: %v\n", err)
		os.Exit(1)
	}
}

// verifyDesired runs at config verification and at Apply for dynamic subscriber
// callers. Invalid rates or classless/classful mixtures must not replace a live
// root and then fail before its upload policer is installed.
func verifyDesired(desired map[string]traffic.InterfaceQoS) error {
	for _, name := range sortedDesiredNames(desired) {
		qos := desired[name]
		// An absent qdisc block carries no root configuration to verify.
		if qos.Qdisc.Type != 0 || qos.Qdisc.RateBps != 0 || len(qos.Qdisc.Classes) != 0 || qos.Qdisc.DefaultClass != "" {
			if _, err := translateQdisc(qos.Qdisc, 0); err != nil {
				return fmt.Errorf("interface %q: %w", name, err)
			}
			for i, class := range qos.Qdisc.Classes {
				if _, err := translateClass(qos.Qdisc.Type, class, 0, tcHandle(0), uint32(i+1)); err != nil {
					return fmt.Errorf("interface %q class %q: %w", name, class.Name, err)
				}
			}
		}
		if qos.Ingress.Set() {
			if _, err := ingressPolicerFilter(qos.Ingress, 0); err != nil {
				return fmt.Errorf("interface %q: %w", name, err)
			}
		}
	}
	return nil
}
