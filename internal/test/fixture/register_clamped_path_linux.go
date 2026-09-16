//go:build linux

// register_clamped_path_linux.go names the two fixtures that build a network
// namespace topology and run one command inside it: the three-namespace
// clamped path the Don't Fragment and show mtu tests probe over, and the
// single empty namespace the doctor test runs in.
//
// Related: plugin_fixture_clamped_path_linux.go -- the topology and the launch
// Related: test/plugin/ping-do-not-fragment-reports-mtu.ci -- the privileged run
// Related: test/plugin/ping-do-not-fragment-unprivileged.ci -- the run without CAP_NET_RAW
// Related: test/plugin/doctor-icmp-probe-missing.ci -- the isolated namespace
// Related: test/plugin/show-mtu-host.ci -- the first of the five show mtu runs

package fixture

func init() {
	Register("plugin/clamped-path", clampedPathDriver)
	Register("plugin/isolated-netns", isolatedNetnsDriver)
}
