// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: netlink_refusal.go -- netlinkRequest, the classifier under test
// Related: restart_fix_test.go -- the restart every fix ends in, for both memberships

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/privilege"
)

// The restart texts under each membership (privilege.RestartRemedy).
const (
	testRestartService = "sudo systemctl restart ze"
	testRestartOther   = "no single command restarts an unsupervised ze: run ze signal stop, then start ze again " +
		"the way it was started (ze start, or ze start <config-file>)"
)

// VALIDATES: netlinkRequest.faultOf classifies a refusal by the errno its kernel path
// returns, per request kind: EPERM is the capability every family asks first, the
// request's own absent errnos (EPROTONOSUPPORT for NETLINK_XFRM, EAFNOSUPPORT for the
// IPv6 rule, EPROTONOSUPPORT and EOPNOTSUPP for nf_tables) a kernel without the
// feature, EACCES a security module, a PolicyOwnedError the ledger that asked no
// kernel, and anything else (EINVAL, EEXIST, ENOENT, another family's absent errno,
// an error with no errno) Unknown.
// METHOD: wrap each answer as its producer wraps it, and classify it.
// PREVENTS: an operator told to grant CAP_NET_ADMIN for a refusal no capability fixes.
func TestNetlinkFaultOfClassifiesTheKernelAnswer(t *testing.T) {
	owned := &dataplane.PolicyOwnedError{HeldBy: "peer-a", Wanted: "catch-all"}
	policy := func(errno syscall.Errno) error { return fmt.Errorf("xfrm: policy update: %w", errno) }
	rule := func(errno syscall.Errno) error {
		return fmt.Errorf("ip rule add from 2001:db8::1/128 lookup 1000: %w", errno)
	}
	flush := func(errno syscall.Errno) error { return fmt.Errorf("firewallnft: flush: %w", errno) }
	cases := []struct {
		name    string
		request netlinkRequest
		err     error
		want    netlinkFault
	}{
		{"xfrm EPERM", xfrmRequest, policy(syscall.EPERM), netlinkFaultCapability},
		{"xfrm EPROTONOSUPPORT", xfrmRequest, policy(syscall.EPROTONOSUPPORT), netlinkFaultKernel},
		{"xfrm EACCES", xfrmRequest, policy(syscall.EACCES), netlinkFaultSecurityModule},
		{"xfrm ownership record", xfrmRequest, fmt.Errorf("ike: %w", owned), netlinkFaultNotAsked},
		{"xfrm EINVAL", xfrmRequest, policy(syscall.EINVAL), netlinkFaultUnknown},
		{"xfrm EEXIST", xfrmRequest, policy(syscall.EEXIST), netlinkFaultUnknown},
		{"xfrm ENOENT", xfrmRequest, policy(syscall.ENOENT), netlinkFaultUnknown},
		{"xfrm EAFNOSUPPORT", xfrmRequest, policy(syscall.EAFNOSUPPORT), netlinkFaultUnknown},
		{"xfrm no errno", xfrmRequest, errors.New("operation not permitted"), netlinkFaultUnknown},
		{"xfrm joined", xfrmRequest, errors.Join(policy(syscall.EINVAL), policy(syscall.EPERM)), netlinkFaultCapability},
		{"rule EPERM", sourceRuleRequest, rule(syscall.EPERM), netlinkFaultCapability},
		{"rule EAFNOSUPPORT", sourceRuleRequest, rule(syscall.EAFNOSUPPORT), netlinkFaultKernel},
		{"rule EACCES", sourceRuleRequest, rule(syscall.EACCES), netlinkFaultSecurityModule},
		{"rule EPROTONOSUPPORT", sourceRuleRequest, rule(syscall.EPROTONOSUPPORT), netlinkFaultUnknown},
		{"rule EINVAL", sourceRuleRequest, rule(syscall.EINVAL), netlinkFaultUnknown},
		{"nft EPERM", nftRequest, flush(syscall.EPERM), netlinkFaultCapability},
		{"nft EPROTONOSUPPORT", nftRequest, fmt.Errorf("firewallnft: open netlink: %w", syscall.EPROTONOSUPPORT), netlinkFaultKernel},
		{"nft EOPNOTSUPP", nftRequest, flush(syscall.EOPNOTSUPP), netlinkFaultKernel},
		{"nft EACCES", nftRequest, flush(syscall.EACCES), netlinkFaultSecurityModule},
		{"nft EINVAL", nftRequest, flush(syscall.EINVAL), netlinkFaultUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.request.faultOf(tc.err); got != tc.want {
				t.Fatalf("faultOf(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// VALIDATES: every netlinkFault class has its own cause and a fix under both
// memberships, a fix given a step ends in that step, and the zero value and an
// undeclared class panic rather than answer.
// METHOD: walk every declared class through causeOf and fixOf for each request kind.
// PREVENTS: a class added without a text, which would reach the operator as an empty
// clause, or two classes that read the same.
func TestNetlinkFaultEveryClassIsAnswered(t *testing.T) {
	for _, member := range []privilege.ServiceMembership{privilege.MembershipZeService, privilege.MembershipOther} {
		t.Run(member.String(), func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(member, nil))
			for _, request := range []netlinkRequest{xfrmRequest, sourceRuleRequest, nftRequest} {
				seen := make(map[string]netlinkFault)
				for fault := netlinkFaultUnspecified + 1; fault <= netlinkFaultUnknown; fault++ {
					cause := request.causeOf(fault)
					if cause == "" {
						t.Errorf("%s: class %d has no cause", request.family, fault)
					}
					if other, dup := seen[cause]; dup {
						t.Errorf("%s: classes %d and %d read the same: %q", request.family, other, fault, cause)
					}
					seen[cause] = fault
					if fix := request.fixOf(fault, ""); fix == "" {
						t.Errorf("%s: class %d has no fix", request.family, fault)
					}
					if fix := request.fixOf(fault, "commit again"); !strings.HasSuffix(fix, "commit again") {
						t.Errorf("%s: class %d's fix does not end in the step it was given: %q", request.family, fault, fix)
					}
				}
			}
		})
	}
	for name, call := range map[string]func(){
		"causeOf unspecified": func() { xfrmRequest.causeOf(netlinkFaultUnspecified) },
		"causeOf undeclared":  func() { xfrmRequest.causeOf(netlinkFaultUnknown + 1) },
		"fixOf unspecified":   func() { xfrmRequest.fixOf(netlinkFaultUnspecified, "") },
		"fixOf undeclared":    func() { xfrmRequest.fixOf(netlinkFaultUnknown+1, "") },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s answered instead of panicking", name)
				}
			}()
			call()
		})
	}
}

// VALIDATES: each class's fix, exactly, under ze.service and outside it: the
// capability remedy for EPERM, the kernel boot then the restart for a missing
// feature, the journal command then the restart for a security module, and the
// restart alone (or the step given) for a cause Ze cannot name; no cause other than
// the capability names CAP_NET_ADMIN.
// METHOD: answer fix and cause for one error per class of the XFRM request.
// PREVENTS: the old single text, "the daemon needs CAP_NET_ADMIN", given for a kernel
// without XFRM or a security module's denial.
func TestNetlinkFaultFixPerClass(t *testing.T) {
	denials := `sudo journalctl -b --grep 'avc:|apparmor="DENIED"'`
	cases := []struct {
		name       string
		err        error
		again      string
		service    string
		other      string
		capability bool
	}{
		{
			name: "capability", err: syscall.EPERM, capability: true,
			service: privilege.DaemonCapabilitiesFix,
		},
		{
			name: "capability, then commit", err: syscall.EPERM, again: "commit again", capability: true,
			service: privilege.DaemonCapabilitiesFix + ", then commit again",
		},
		{
			name:    "kernel",
			err:     syscall.EPROTONOSUPPORT,
			service: "boot a kernel built with CONFIG_XFRM_USER (Ze's own kernel carries it), then restart ze: " + testRestartService,
			other:   "boot a kernel built with CONFIG_XFRM_USER (Ze's own kernel carries it), then restart ze: " + testRestartOther,
		},
		{
			name:    "security module",
			err:     syscall.EACCES,
			service: "find the denial with " + denials + ", allow ze its NETLINK_XFRM requests in that policy, then restart ze: " + testRestartService,
			other:   "find the denial with " + denials + ", allow ze its NETLINK_XFRM requests in that policy, then restart ze: " + testRestartOther,
		},
		{
			name: "security module, then commit", err: syscall.EACCES, again: "commit again",
			service: "find the denial with " + denials + ", allow ze its NETLINK_XFRM requests in that policy, then commit again",
		},
		{name: "unknown", err: syscall.EINVAL, service: testRestartService, other: testRestartOther},
		{
			name: "unknown, then commit", err: syscall.EINVAL, again: "commit again",
			service: "remove the cause the error names, then commit again",
			other:   "remove the cause the error names, then commit again",
		},
	}
	for _, tc := range cases {
		err := fmt.Errorf("xfrm: policy update: %w", tc.err)
		t.Run(tc.name+"/ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
			if got := xfrmRequest.fix(err, tc.again); got != tc.service {
				t.Errorf("fix = %q, want %q", got, tc.service)
			}
			if cause := xfrmRequest.cause(err); strings.Contains(cause, "CAP_NET_ADMIN") != tc.capability {
				t.Errorf("the cause names CAP_NET_ADMIN = %v, want %v: %q", !tc.capability, tc.capability, cause)
			}
		})
		if tc.other == "" {
			continue
		}
		t.Run(tc.name+"/other", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
			got := xfrmRequest.fix(err, tc.again)
			if got != tc.other {
				t.Errorf("fix = %q, want %q", got, tc.other)
			}
			if strings.Contains(got, "systemctl") {
				t.Errorf("outside ze.service the fix names systemctl: %q", got)
			}
		})
	}
	t.Run("capability/other", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		got := xfrmRequest.fix(fmt.Errorf("xfrm: policy update: %w", syscall.EPERM), "")
		if want := privilege.CapabilityRemedy("CAP_NET_ADMIN"); got != want {
			t.Errorf("fix = %q, want %q", got, want)
		}
		if !strings.Contains(got, "CAP_NET_ADMIN") || strings.Contains(got, "systemctl") {
			t.Errorf("outside ze.service the capability fix does not name CAP_NET_ADMIN alone: %q", got)
		}
	})
}

// VALIDATES: every IPsec SPD site reads the XFRM classification: the reload's
// catch-all refusal names the cause and its fix, the start's names the cause alone,
// the start and exemption operator errors carry the class's fix, and the bound
// interface's reload refusal names the first interface's cause and fix.
// METHOD: drive each producer with a kernel without CONFIG_XFRM_USER and with a
// missing capability, under ze.service.
// PREVENTS: a site that keeps "the daemon needs CAP_NET_ADMIN" for every refusal.
func TestSPDSitesNameTheKernelAnswer(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	kernel := fmt.Errorf("xfrm: policy update: %w", syscall.EPROTONOSUPPORT)
	capability := fmt.Errorf("xfrm: policy update: %w", syscall.EPERM)
	kernelFix := xfrmRequest.fix(kernel, "")
	entry := &xfrmInterfaceEntry{iface: "xfrm0", params: dataplane.SPParams{Dir: dataplane.SADirFwd}}

	texts := map[string]struct {
		text, fix string
	}{
		"catch-all reload": {text: catchAllInstallHint(kernel, applyReload)},
		"catch-all start":  {text: catchAllInstallHint(kernel, applyStartup)},
		"catch-all start refusal": {
			text: unmatchedStartRefusal(dataplane.SPActionDiscard, kernel).Why(),
			fix:  unmatchedStartRefusal(dataplane.SPActionDiscard, kernel).Fix(),
		},
		"exemption refusal": {
			text: interfaceExemptionRefusal(entry, kernel).Why(),
			fix:  interfaceExemptionRefusal(entry, kernel).Fix(),
		},
		"interface catch-all start refusal": {
			text: interfaceCatchAllStartRefusal("xfrm0", kernel).Why(),
			fix:  interfaceCatchAllStartRefusal("xfrm0", kernel).Fix(),
		},
		"interface catch-all reload refusal": {text: interfaceCatchAllRefused(map[string]error{"xfrm0": kernel}).Error()},
	}
	for name, got := range texts {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got.text, "CONFIG_XFRM_USER") {
				t.Errorf("the text does not name the kernel option: %q", got.text)
			}
			if strings.Contains(got.text, "CAP_NET_ADMIN") {
				t.Errorf("a kernel without XFRM netlink is sent after a capability: %q", got.text)
			}
			if got.fix != "" && got.fix != kernelFix {
				t.Errorf("fix = %q, want %q", got.fix, kernelFix)
			}
		})
	}
	if got := catchAllInstallHint(kernel, applyReload); !strings.HasSuffix(got, "; to fix it: "+xfrmRequest.fix(kernel, "commit again")) {
		t.Errorf("the reload's catch-all refusal does not end in its fix: %q", got)
	}
	if got := catchAllInstallHint(kernel, applyStartup); strings.Contains(got, "to fix it") {
		t.Errorf("the start's catch-all refusal names a fix its operator error already carries: %q", got)
	}
	if got := unmatchedStartRefusal(dataplane.SPActionDiscard, capability).Fix(); got != privilege.DaemonCapabilitiesFix {
		t.Errorf("the capability start refusal's fix = %q, want %q", got, privilege.DaemonCapabilitiesFix)
	}
	reload := interfaceCatchAllRefused(map[string]error{"xfrm0": capability}).Error()
	if !strings.HasSuffix(reload, "to fix it: "+privilege.DaemonCapabilitiesFix+", then commit again") {
		t.Errorf("the interface reload refusal does not end in the capability fix and the commit: %q", reload)
	}
	if !errors.Is(interfaceCatchAllRefused(map[string]error{"xfrm0": capability}), syscall.EPERM) {
		t.Error("the interface reload refusal no longer wraps the kernel's answer")
	}
}

// VALIDATES: the tunnel-source guard's two log lines name the cause of their own
// producer: an IPv6 rule refused EAFNOSUPPORT names CONFIG_IPV6_MULTIPLE_TABLES, and
// an nft publish acked EOPNOTSUPP names CONFIG_NF_TABLES, neither a capability.
// METHOD: fail the recorder's rule install, then its publish, and read the log.
// PREVENTS: "the daemon needs CAP_NET_ADMIN" logged for a kernel without the feature.
func TestSourceGuardLogNamesTheKernelAnswer(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	want := map[string]sourceGuardSpec{
		"vti0": {iface: "vti0", device: "vti0", ifIndex: 7, ifID: vti0IfID, sources: []netip.Addr{netip.MustParseAddr("2001:db8::1")}},
	}
	cases := []struct {
		name, option string
		arm          func(*guardRecorder)
	}{
		{"rule", "CONFIG_IPV6_MULTIPLE_TABLES", func(r *guardRecorder) {
			r.failRule = fmt.Errorf("ip rule add from 2001:db8::1/128 lookup 1000: %w", syscall.EAFNOSUPPORT)
		}},
		{"publish", "CONFIG_NF_TABLES", func(r *guardRecorder) {
			r.failPublish = fmt.Errorf("firewallnft: flush: %w", syscall.EOPNOTSUPP)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newGuardRecorder()
			tc.arm(recorder)
			var logged bytes.Buffer
			newSourceGuards(recorder).install(want, slog.New(slog.NewTextHandler(&logged, nil)))
			out := logged.String()
			if !strings.Contains(out, tc.option) {
				t.Errorf("the log does not name %s: %s", tc.option, out)
			}
			if strings.Contains(out, "CAP_NET_ADMIN") {
				t.Errorf("a kernel without the feature is sent after a capability: %s", out)
			}
			if !strings.Contains(out, "restart ze: "+testRestartService) {
				t.Errorf("the log does not name the restart after the kernel fix: %s", out)
			}
		})
	}
}
