// Design: docs/architecture/testing/interop.md -- external SDK export-policy carrier.
// RFC 9252 Section 3.2.1 -- see rfc/short/rfc9252.md.
// Related: test/plugin/srv6-service-export-control.ci -- per-peer wire assertions.
package fixture

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/srv6-service-export-control", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "plugin/srv6-service-export-control", sdk.Registration{}, srv6ServiceExportControl)
	})
}

func srv6ServiceExportControl(ctx context.Context, p *sdk.Plugin) error {
	if err := waitEOR07(ctx, p, 2); err != nil {
		return err
	}
	for _, service := range []int{10, 20} {
		command := fmt.Sprintf("update text extended-community [target:65000:%d] bgp-prefix-sid-srv6 ( l3-service 2001:db8:%d:: 0x13 ) nhop 2001:db8::1 rd 65000:%d label 100 nlri ipv4/mpls-vpn add 10.%d.0.0/24", service, service, service, service)
		announced, withdrawn, err := p.UpdateRoute(ctx, "*", command)
		if err != nil {
			return fmt.Errorf("announce SRv6 service %d: %w", service, err)
		}
		if announced != 1 {
			return fmt.Errorf("announce SRv6 service %d: announced=%d withdrawn=%d, want 1/0", service, announced, withdrawn)
		}
		if withdrawn != 0 {
			return fmt.Errorf("announce SRv6 service %d withdrew %d routes", service, withdrawn)
		}
	}
	if err := quiesce07(ctx, p); err != nil {
		return err
	}
	for _, peer := range []struct {
		address string
		updates int
	}{
		{"127.0.0.1", 1},
		{"127.0.0.2", 2},
	} {
		row, err := peerRow07(ctx, p, peer.address)
		if err != nil {
			return err
		}
		if got := number07(row["updates-sent"]) - number07(row["eor-sent"]); got != peer.updates {
			return fmt.Errorf("peer %s received %d service UPDATEs, want %d: %s", peer.address, got, peer.updates, text07(row))
		}
	}
	return nil
}
