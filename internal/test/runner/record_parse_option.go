// Design: docs/architecture/testing/ci-format.md -- the option= directive family
// Overview: record_parse.go -- the line parser that dispatches option= here
// Related: record_parse_vocabulary.go -- recordOptionArms, the keys each arm reads

package runner

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// parseOption handles option=type:key=value lines. The type's arm in
// recordOptionArms (record_parse_vocabulary.go) names the keys it reads, so an
// unknown type and an unknown key are both refused before any arm runs.
func (et *EncodingTests) parseOption(r *Record, ciFile, optType string, kv map[string]string) error {
	if err := checkDirectiveType(directiveOption, optType); err != nil {
		return err
	}
	arm := recordOptionArms[optType]
	if err := checkKeys("option="+optType, kv, arm.keys...); err != nil {
		return err
	}
	return arm.parse(r, ciFile, kv)
}

// parseOptionFile reads option=file.
func parseOptionFile(r *Record, ciFile string, kv map[string]string) error {
	configName := kv["path"]
	if configName == "" {
		return errOptionFileMissingPath
	}
	configPath := filepath.Join(filepath.Dir(ciFile), configName)
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}
	absTestDir, err := filepath.Abs(filepath.Dir(ciFile))
	if err != nil {
		return fmt.Errorf("invalid test dir: %w", err)
	}
	if !strings.HasPrefix(absConfig, absTestDir+string(filepath.Separator)) && absConfig != absTestDir {
		return fmt.Errorf("config file outside test directory: %s", configName)
	}
	r.Conf["config"] = configPath
	r.ConfigFile = configPath
	r.Files = append(r.Files, configPath)
	return nil
}

// parseOptionAsn reads option=asn.
func parseOptionAsn(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionAsnMissingValue
	}
	r.Extra["asn"] = value
	r.Options = append(r.Options, "option=asn:value="+value)
	return nil
}

// parseOptionBind reads option=bind.
func parseOptionBind(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionBindMissingValue
	}
	r.Extra["bind"] = value
	r.Options = append(r.Options, "option=bind:value="+value)
	return nil
}

// parseOptionTimeout reads option=timeout.
func parseOptionTimeout(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionTimeoutMissingValue
	}
	// resolveOrchestratedTimeout parses this and keeps the suggested budget
	// when it fails, so a typo here buys a test a different timeout with no
	// line said about it. Judged where the author can be told.
	if _, err := time.ParseDuration(value); err != nil {
		return fmt.Errorf("option=timeout:value=%q is not a duration: %w", value, err)
	}
	r.Extra["timeout"] = value
	return nil
}

// parseOptionTcpConnections reads option=tcp_connections.
func parseOptionTcpConnections(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionTcpConnectionsMissingValue
	}
	r.Options = append(r.Options, "option=tcp_connections:value="+value)
	return nil
}

// parseOptionOpen reads option=open.
func parseOptionOpen(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionOpenMissingValue
	}
	r.Options = append(r.Options, "option=open:value="+value)
	return nil
}

// parseOptionUpdate reads option=update.
func parseOptionUpdate(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionUpdateMissingValue
	}
	r.Options = append(r.Options, "option=update:value="+value)
	return nil
}

// parseOptionEnv reads option=env.
func parseOptionEnv(r *Record, _ string, kv map[string]string) error {
	varName := kv["var"]
	value := kv["value"]
	if varName == "" {
		return errOptionEnvMissingVar
	}
	// Store as KEY=VALUE for environment setting
	r.EnvVars = append(r.EnvVars, varName+"="+value)
	return nil
}

// parseOptionSkipOs reads option=skip-os.
func parseOptionSkipOs(r *Record, _ string, kv map[string]string) error {
	value := kv["value"]
	if value == "" {
		return errOptionSkipOsMissingValue
	}
	// Record a skip reason when the current GOOS is in the skip list.
	// The .ci format has no build tags, so OS-specific features (e.g.
	// Linux-only IP_RECVTTL for BFD echo) gate at parse time. The runner
	// honors SkipReason regardless of whether the user selected the test
	// by name -- asking for an unsupported test on the wrong OS reports
	// SKIP (with reason), never FAIL. Multiple skip-os options
	// accumulate; any match skips.
	for skipOS := range strings.SplitSeq(value, ",") {
		if strings.TrimSpace(skipOS) == runtime.GOOS {
			var tb textbuf.Buffer
			r.SkipReason = tb.Str("skip-os=").Str(value).Str(" (current GOOS=").Str(runtime.GOOS).Byte(')').String()
			return nil
		}
	}
	return nil
}

// parseOptionNeedsLinux reads option=needs-linux.
func parseOptionNeedsLinux(r *Record, _ string, kv map[string]string) error {
	// Marks a .ci test that requires a real Linux kernel (netlink interface
	// management, nftables, kernel sockets, ...) and therefore cannot pass
	// natively on a non-Linux host. On such a host the test is SKIPPED with
	// a reason pointing at the QEMU runner; inside the QEMU Alpine VM
	// (GOOS=linux, via `./le test qemu all`) the directive is
	// inert and the test runs normally. This is how Linux-only functional
	// tests are validated automatically via QEMU instead of failing
	// natively. See ai/rules/platform-linux.md "Linux-only functional tests".
	//
	// An optional `caps=net-admin` declares that the test also needs
	// privileged network configuration. Without it, a test that applies
	// interface config on an unprivileged Linux host does not fail cleanly:
	// the interface plugin dies with "operation not permitted" during its
	// stage-3 handshake, the daemon never reaches the asserted state, and
	// the test HANGS until the suite timeout. Declaring the capability turns
	// that hang into an honest skip that names the QEMU runner, where the
	// test does have the capability and therefore really runs.
	r.NeedsLinux = true
	if raw, present := kv["caps"]; present && strings.TrimSpace(raw) == "" {
		return fmt.Errorf("option=needs-linux: caps= is empty; name each capability the test needs "+
			"(supported: %s), e.g. option=needs-linux:caps=net-admin, or drop caps=", capsAccepted())
	}

	// Validate the declared capability BEFORE the GOOS early-return. A typo
	// is a property of the .ci file, not of the host, so it must be rejected
	// everywhere: validating after the return would silently accept
	// `caps=net-admn` on macOS, leaving the author who wrote it with no
	// feedback and the gate quietly disabled until someone ran on Linux.
	// caps is a comma-separated LIST: a test that both programs netlink and
	// loads eBPF declares `caps=net-admin,bpf` and is gated on BOTH.
	// Declaring only one of a pair a test really needs makes the gate fail
	// OPEN -- a host holding just that one passes a check it cannot satisfy,
	// and the test then fails or hangs exactly as it did before the marker
	// existed (ai/rules/evidence.md).
	var caps []string
	if raw := kv["caps"]; raw != "" {
		for c := range strings.SplitSeq(raw, ",") {
			c = strings.TrimSpace(c)
			if _, ok := capsRequired[c]; !ok {
				return fmt.Errorf("option=needs-linux: unknown caps %q (supported: %s)", c, capsAccepted())
			}
			caps = append(caps, c)
		}
	}
	if runtime.GOOS != goosLinux {
		var tb textbuf.Buffer
		r.SkipReason = tb.Str("needs-linux (run via ./le test qemu all; current GOOS=").Str(runtime.GOOS).Byte(')').String()
		return nil
	}
	if len(caps) > 0 && !hasCaps(caps) {
		var tb textbuf.Buffer
		r.SkipReason = tb.Str("needs-linux caps=").Str(strings.Join(caps, ",")).
			Str(" (capability absent; run via ./le test qemu all)").String()
		return nil
	}
	return nil
}

// parseOptionNeedsArch reads option=needs-arch.
//
// It declares the host CPU arches the test may run on (needs_arch.go). On any
// other host it SKIPS with a reason naming both arches. Like skip-os it never
// clears a SkipReason an earlier option set.
func parseOptionNeedsArch(r *Record, _ string, kv map[string]string) error {
	reason, err := needsArchSkipReason(kv["value"], runtime.GOARCH)
	if err != nil {
		return err
	}
	if reason != "" {
		r.SkipReason = reason
	}
	return nil
}

// parseOptionNeedsPath reads option=needs-path.
func parseOptionNeedsPath(r *Record, ciFile string, kv map[string]string) error {
	// Declares a repo-relative path the test cannot run without, where the
	// path is an OPTIONAL heavyweight artifact rather than something a
	// checkout carries. The only such artifact today is the appliance module
	// cache: gokrazy/modcache/.gitignore ignores everything except the
	// vendored gokrazy init source, so a downloaded module exists only
	// after `./le setup install`.
	//
	// Absent the path the test SKIPS with a reason naming the path and the
	// command that materializes it. That is deliberately not a silent pass:
	// test/install/ze-kernel-overlay.ci read the pinned vmlinuz with no guard
	// and failed `shasum: ... No such file or directory` on every CI run,
	// and hiding that behind an `exit 0` would have swapped a red for a
	// green bar over a test that ran nothing.
	//
	// The path is resolved against the repo root, not the working directory:
	// each test runs in its own temp dir.
	value := kv["value"]
	if value == "" {
		return errOptionNeedsPathMissingValue
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
		return fmt.Errorf("option=needs-path: value %q must be repo-relative and contain no %q", value, "..")
	}
	root, rootErr := repoRootFrom(ciFile)
	if rootErr != nil {
		return fmt.Errorf("option=needs-path: locate repo root from %s: %w", ciFile, rootErr)
	}
	if !needsPathSatisfied(root, value) {
		var tb textbuf.Buffer
		reason := tb.Str("needs-path=").Str(value).Str(" (absent")
		if hint := kv["hint"]; hint != "" {
			reason = reason.Str("; run: ").Str(hint)
		}
		r.SkipReason = reason.Byte(')').String()
		return nil
	}
	return nil
}

// parseOptionNetnsLink reads option=netns-link.
func parseOptionNetnsLink(r *Record, _ string, kv map[string]string) error {
	// Provision an interface inside the per-test network namespace before ze
	// launches. name= is the link; address= is an optional CIDR assigned to it.
	// Without peer= the link is a dummy, which has no far end: it drops what is
	// written to it, so it gives a netns connectivity (a connected route for a
	// next-hop) but can never carry a conversation. peer= makes it a veth PAIR
	// instead, both ends in this same namespace, which is what a test needs when
	// a daemon must exchange real frames with a client (PPPoE discovery, RFC
	// 2516). vlan= additionally creates an 802.1Q sub-interface on each end,
	// named <link>.<tag>.
	//
	// Consumed only under netns mode (ZE_TEST_NETNS); on the default host path
	// the option is inert and never touches the host. Every field is validated
	// here so a typo fails at parse time on every platform, not silently at run
	// time on Linux only.
	name := kv["name"]
	if name == "" {
		return errOptionNetnsLinkMissingName
	}
	spec := NetnsLinkSpec{Name: name, Peer: kv["peer"]}
	if spec.Peer == name {
		return fmt.Errorf("option=netns-link: peer %q must differ from name (a veth pair has two ends)", spec.Peer)
	}
	if addr := kv["address"]; addr != "" {
		p, err := netip.ParsePrefix(addr)
		if err != nil {
			return fmt.Errorf("option=netns-link: invalid address %q: %w", addr, err)
		}
		spec.Address = p
	}
	if raw := kv["vlan"]; raw != "" {
		// 1..4094: 0 means "no tag" and would be indistinguishable from an absent
		// vlan=, and 4095 is reserved by 802.1Q. Rejecting both ends here keeps an
		// unusable tag from reaching netlink as a run-time error on Linux only.
		tag, tagErr := strconv.ParseUint(raw, 10, 16)
		if tagErr != nil || tag < 1 || tag > 4094 {
			return fmt.Errorf("option=netns-link: invalid vlan %q (want 1..4094)", raw)
		}
		spec.VLAN = uint16(tag)
	}
	r.NetnsLinks = append(r.NetnsLinks, spec)
	// The skip gate for this option is applied once, after every option is
	// parsed (see parseAndAdd), so it does not depend on the order the .ci
	// file lists its options in.
	return nil
}

// parseOptionExclusive reads option=exclusive.
func parseOptionExclusive(r *Record, _ string, kv map[string]string) error {
	// Serialize this test against every other test carrying the SAME group
	// name. Unrelated tests keep running concurrently, so this is much cheaper
	// than -p 1 for the whole suite. Applies on every platform and in every
	// runner mode -- the contention it guards is a property of the tests, not
	// of the host.
	group := kv["group"]
	if group == "" {
		return errOptionExclusiveMissingGroup
	}
	r.ExclusiveGroup = group
	return nil
}

// parseOptionSkipEnv reads option=skip-env.
func parseOptionSkipEnv(r *Record, _ string, kv map[string]string) error {
	varName := kv["var"]
	if varName == "" {
		return errOptionSkipEnvMissingVar
	}
	expected := kv["value"]
	actual := os.Getenv(varName)
	var tb textbuf.Buffer
	if expected == "" {
		if actual != "" {
			r.SkipReason = tb.Str("skip-env=").Str(varName).Str(" (set to ").Str(actual).Byte(')').String()
			return nil
		}
	} else if actual == expected {
		r.SkipReason = tb.Reset().Str("skip-env=").Str(varName).Byte('=').Str(expected).String()
		return nil
	}
	return nil
}
