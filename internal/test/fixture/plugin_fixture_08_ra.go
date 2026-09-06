// Design: test/plugin/iface-ra-slaac.ci -- the scenario this fixture asserts
// Related: plugin_fixture_08_iface.go -- the ip -j helpers this file reuses

package fixture

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	registerPlugin08("plugin/iface-ra-slaac", "iface-ra-slaac", ifaceRASLAAC08)
}

// The devices, the prefix and the lifetimes this fixture shares with
// test/plugin/iface-ra-slaac.ci. The lifetimes are written in both places on
// purpose: the .ci configures them and the fixture asserts them, so a value
// that never reaches the wire fails here rather than passing unread.
const (
	raSLAACRouter08 = "zeraslc0"
	raSLAACHost08   = "zeraslc1"
	raSLAACPrefix08 = "2001:db8:5a1::/64"
	// raSLAACValidLifetime08 is the valid-lifetime the .ci advertises. A
	// statically configured address carries 4294967295 instead, so an address
	// bounded by this number can only have come from the advertisement.
	raSLAACValidLifetime08 = 86400
	// raSLAACPreferredLifetime08 is the preferred-lifetime the .ci advertises.
	raSLAACPreferredLifetime08 = 43200
	// raSLAACDeviceAttempts08 and raSLAACAddressAttempts08 bound the two polls
	// at 15 s and 30 s. The advertisement repeats every 3 to 4 s, so 30 s holds
	// at least seven of them, and a host that has acted on none of seven has
	// not acted on the advertisement at all.
	raSLAACDeviceAttempts08  = 60
	raSLAACAddressAttempts08 = 120
	raSLAACPollDelay08       = 250 * time.Millisecond
)

// raHostSysctl08 puts one per-device IPv6 sysctl on the receiving end.
func raHostSysctl08(device, key, value string) error {
	path := "/proc/sys/net/ipv6/conf/" + device + "/" + key
	// The mode is ignored: every path here already exists in procfs, so
	// WriteFile never creates a file and never applies it.
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return fmt.Errorf("write %s = %s: %w", path, value, err)
	}
	return nil
}

// raAddrInfo08 returns the addr_info rows the kernel holds for device.
func raAddrInfo08(ctx context.Context, device string) []map[string]any {
	var found []map[string]any
	for _, row := range ipJSONOptional08(ctx, "addr", "show", device) {
		infos, _ := row["addr_info"].([]any)
		for _, value := range infos {
			if info, ok := value.(map[string]any); ok {
				found = append(found, info)
			}
		}
	}
	return found
}

// raAutoconfigured08 returns the address the host formed inside prefix, and
// reports whether it found one.
//
// The host end carries no address in the configuration, so a global address
// inside the advertised prefix can only have come from the advertisement. The
// lifetimes are checked by the caller, which is what tells an autoconfigured
// address from one somebody added by hand.
func raAutoconfigured08(ctx context.Context, device string, prefix netip.Prefix) (map[string]any, bool) {
	for _, info := range raAddrInfo08(ctx, device) {
		local, _ := info["local"].(string)
		addr, err := netip.ParseAddr(local)
		if err != nil {
			continue
		}
		if prefix.Contains(addr) {
			return info, true
		}
	}
	return nil, false
}

// ifaceRASLAAC08 proves the whole path from an operator's configuration to a
// host address: the interface component parses the router-advertisement
// container, reconcileRA starts an iface-ra sender through the factory seam,
// the sender advertises on the veth, and the kernel on the far end forms an
// address by stateless address autoconfiguration.
func ifaceRASLAAC08(ctx context.Context, _ *sdk.Plugin) error {
	prefix, err := netip.ParsePrefix(raSLAACPrefix08)
	if err != nil {
		return fmt.Errorf("prefix %s: %w", raSLAACPrefix08, err)
	}

	if !Poll(ctx, raSLAACDeviceAttempts08, raSLAACPollDelay08, func() bool {
		return len(ipJSONOptional08(ctx, "link", "show", raSLAACHost08)) != 0
	}) {
		return fmt.Errorf("%s never appeared: the daemon did not create the veth pair", raSLAACHost08)
	}

	// accept_ra 2 accepts advertisements even while the node forwards, which
	// the router end of this pair turns on. autoconf and accept_ra_pinfo are
	// the two that turn a Prefix Information option into an address, and
	// use_tempaddr 0 keeps a privacy address from appearing beside it.
	for _, s := range []struct{ key, value string }{
		{"disable_ipv6", "0"},
		{"accept_ra", "2"},
		{"accept_ra_pinfo", "1"},
		{"autoconf", "1"},
		{"use_tempaddr", "0"},
	} {
		if err := raHostSysctl08(raSLAACHost08, s.key, s.value); err != nil {
			return err
		}
	}

	var info map[string]any
	if !Poll(ctx, raSLAACAddressAttempts08, raSLAACPollDelay08, func() bool {
		var ok bool
		info, ok = raAutoconfigured08(ctx, raSLAACHost08, prefix)
		return ok
	}) {
		return fmt.Errorf("%s formed no address inside %s: the host acted on no advertisement from %s (addresses: %v)",
			raSLAACHost08, prefix, raSLAACRouter08, addresses08(ctx, raSLAACHost08))
	}

	return raReportAutoconfigured08(info, prefix)
}

// raReportAutoconfigured08 checks the address the host formed and prints the
// line the .ci matches on.
func raReportAutoconfigured08(info map[string]any, prefix netip.Prefix) error {
	local, _ := info["local"].(string)
	if got := int64(number08(info["prefixlen"])); got != int64(prefix.Bits()) {
		return fmt.Errorf("%s formed %s/%d, want a /%d", raSLAACHost08, local, got, prefix.Bits())
	}

	// The lifetimes are the discriminating half of this test. A statically
	// configured address carries 4294967295 in both fields, so an address
	// inside the advertised prefix would pass the check above against a build
	// whose Prefix Information lifetimes never left the encoder. These two
	// numbers reached the host only through the option.
	valid := int64(number08(info["valid_life_time"]))
	if valid <= 0 || valid > raSLAACValidLifetime08 {
		return fmt.Errorf("%s valid lifetime = %d, want 1..%d from the advertised prefix",
			local, valid, raSLAACValidLifetime08)
	}
	preferred := int64(number08(info["preferred_life_time"]))
	if preferred <= 0 || preferred > raSLAACPreferredLifetime08 {
		return fmt.Errorf("%s preferred lifetime = %d, want 1..%d from the advertised prefix",
			local, preferred, raSLAACPreferredLifetime08)
	}

	var out textbuf.Buffer
	out.Str("OK: ").Str(raSLAACHost08).Str(" autoconfigured ").Str(local).Byte('/').Int(int64(prefix.Bits()))
	out.Str(" from the advertisement on ").Str(raSLAACRouter08)
	out.Str(" (valid ").Int(valid).Str("s, preferred ").Int(preferred).Str("s)\n")
	return out.StdErr()
}
