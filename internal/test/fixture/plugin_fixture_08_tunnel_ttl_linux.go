//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	registerPlugin08("plugin/tunnel-ttl-default", "tunnel-ttl-default-test", tunnelTTLDefault08)
}

// tunnelOuterTTL08 reads the outer-header TTL the kernel stores on one tunnel
// netdev. Each tunnel kind carries the value in its own netlink Go type, and
// the IPv6-underlay kinds keep their hop limit in that same field, so one
// reader serves every row of the table below.
//
// The read goes to the kernel rather than to `show interface`, because the
// interface list carries no outer-header field: the value under test is a
// device attribute that only a link read-back reports.
func tunnelOuterTTL08(device string) (uint8, error) {
	link, err := netlink.LinkByName(device)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", device, err)
	}
	switch tunnel := link.(type) {
	case *netlink.Gretun:
		return tunnel.Ttl, nil
	case *netlink.Gretap:
		return tunnel.Ttl, nil
	case *netlink.Iptun:
		return tunnel.Ttl, nil
	case *netlink.Sittun:
		return tunnel.Ttl, nil
	default:
		return 0, fmt.Errorf("%s is a %T, which carries no outer TTL", device, link)
	}
}

// tunnelTTLExpectation08 is one device and the outer TTL its stanza in
// test/plugin/tunnel-ttl-default.ci must produce.
type tunnelTTLExpectation08 struct {
	device string
	ttl    uint8
	why    string
}

// tunnelTTLExpectations08 covers every acceptance criterion of the
// tunnel-ttl-default spec: the four IPv4-underlay kinds with no ttl leaf, the
// two explicit values, and the IPv6-underlay kind whose hoplimit default has
// read 64 in the schema since the tunnel kinds landed.
var tunnelTTLExpectations08 = []tunnelTTLExpectation08{
	{"ttdgre", 64, "gre with no ttl leaf takes the schema default"},
	{"ttdgretap", 64, "gretap with no ttl leaf takes the schema default"},
	{"ttdipip", 64, "ipip with no ttl leaf takes the schema default"},
	{"ttdsit", 64, "sit with no ttl leaf takes the schema default"},
	{"ttdinherit", 0, "an explicit ttl 0 still means inherit from the inner packet"},
	{"ttdexplicit", 200, "an explicit ttl is applied unchanged"},
	{"ttdip6gre", 64, "ip6gre with no hoplimit leaf takes the schema default"},
}

// tunnelTTLDefault08 reads the outer-header TTL of every tunnel the config
// created and compares it against the table above.
//
// The devices appear a moment after the apply reports success, so the read
// polls. A device that never appears fails on its own name rather than on a
// TTL mismatch, which tells the reader whether the apply or the default is at
// fault.
func tunnelTTLDefault08(ctx context.Context, p *sdk.Plugin) error {
	if _, err := requireDone08(ctx, p, "show interface"); err != nil {
		return err
	}
	found := make(map[string]uint8, len(tunnelTTLExpectations08))
	var absent []string
	Poll(ctx, statusPollAttempts, 250*time.Millisecond, func() bool {
		absent = nil
		for _, row := range tunnelTTLExpectations08 {
			ttl, err := tunnelOuterTTL08(row.device)
			if err != nil {
				absent = append(absent, err.Error())
				continue
			}
			found[row.device] = ttl
		}
		return len(absent) == 0
	})
	if len(absent) != 0 {
		sort.Strings(absent)
		return fmt.Errorf("the apply left no device for: %s", strings.Join(absent, "; "))
	}

	var wrong []string
	for _, row := range tunnelTTLExpectations08 {
		if found[row.device] == row.ttl {
			continue
		}
		var line textbuf.Buffer
		line.Str(row.device).Str(" outer TTL is ").Uint8(found[row.device]).
			Str(", want ").Uint8(row.ttl).Str(" (").Str(row.why).Str(")")
		wrong = append(wrong, line.String())
	}
	if len(wrong) != 0 {
		sort.Strings(wrong)
		return errors.New(strings.Join(wrong, "; "))
	}

	var report textbuf.Buffer
	report.Str("OK: all ").Int(int64(len(tunnelTTLExpectations08))).
		Str(" tunnels carry the outer TTL their config asks for\n")
	return report.StdErr()
}
