// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched.go -- catchAllInstallHint and unmatchedStartRefusal, the node-wide catch-all's refusals
// Related: unmatched_xfrm_interface.go -- the refusals of a bound XFRM interface's SPD entries
// Related: source_guard.go -- the refusals of the tunnel-source rule and nft table
// Related: ../cmd/show_dataplane.go -- dataplaneReadError, a refused read of the SAD or SPD
// Related: ../transport/udp.go -- bypassFaultOf, the same classification for setsockopt(IP_XFRM_POLICY)

package engine

import (
	"errors"
	"syscall"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/privilege"
)

// netlinkFault classifies why a netlink request Ze sent to install or read IPsec state
// was refused, which decides the fix (netlinkRequest.faultOf). It is closed: only
// faultOf produces it.
type netlinkFault uint8

const (
	// netlinkFaultUnspecified is the zero value, which faultOf never answers.
	netlinkFaultUnspecified netlinkFault = iota
	// netlinkFaultNotAsked is Ze's own SPD ownership record refusing an entry before
	// any request reached the kernel (dataplane.PolicyOwnedError).
	netlinkFaultNotAsked
	// netlinkFaultCapability is a process without CAP_NET_ADMIN: EPERM.
	netlinkFaultCapability
	// netlinkFaultKernel is a kernel built without the request's feature: one of the
	// request's absent errnos.
	netlinkFaultKernel
	// netlinkFaultSecurityModule is a security module that refused the request: EACCES.
	netlinkFaultSecurityModule
	// netlinkFaultUnknown is any other answer: Ze cannot tell which fix applies, and
	// says so rather than name one that may not.
	netlinkFaultUnknown
)

// netlinkRequest is one kind of netlink request whose refusal Ze explains to the
// operator: its family, the kernel option the family needs, and the errnos a kernel
// built without that option answers. Never changed after its declaration.
type netlinkRequest struct {
	// family is the netlink family as socket(7) and a security policy name it.
	family string
	// feature is the kernel option the request needs.
	feature string
	// absent are the errnos a kernel built without feature answers.
	absent []syscall.Errno
}

var (
	// xfrmRequest is every request the XFRM backend sends over NETLINK_XFRM: an SPD
	// entry (XFRM_MSG_UPDPOLICY), the namespace forward default (XFRM_MSG_SETDEFAULT)
	// and the SAD and SPD dumps. Without CONFIG_XFRM_USER no NETLINK_XFRM protocol is
	// registered, so the socket the request needs is refused EPROTONOSUPPORT.
	xfrmRequest = netlinkRequest{family: "NETLINK_XFRM", feature: "CONFIG_XFRM_USER", absent: []syscall.Errno{syscall.EPROTONOSUPPORT}}
	// sourceRuleRequest is the IPv6 source rule of the tunnel-source guard
	// (iprule.Source.Add, RTM_NEWRULE over NETLINK_ROUTE). NETLINK_ROUTE is always
	// built, and a kernel without IPv6 policy routing answers EAFNOSUPPORT.
	sourceRuleRequest = netlinkRequest{family: "NETLINK_ROUTE", feature: "CONFIG_IPV6_MULTIPLE_TABLES", absent: []syscall.Errno{syscall.EAFNOSUPPORT}}
	// nftRequest is the nf_tables batch that publishes the tunnel-source guard's table
	// (firewall.ApplyAll, the kernel backend, over NETLINK_NETFILTER). A kernel without
	// nfnetlink refuses the socket EPROTONOSUPPORT, and one without nf_tables acks the
	// batch EOPNOTSUPP.
	nftRequest = netlinkRequest{family: "NETLINK_NETFILTER", feature: "CONFIG_NF_TABLES", absent: []syscall.Errno{syscall.EPROTONOSUPPORT, syscall.EOPNOTSUPP}}
)

// faultOf classifies err, the error a netlink request of kind r answered, by the
// errno the kernel returned (Linux master):
//
//   - EPERM: each family asks CAP_NET_ADMIN before it reads the request.
//     xfrm_user_rcv_msg (net/xfrm/xfrm_user.c): "/* All operations require
//     privileges, even GET */ if (!netlink_net_capable(skb, CAP_NET_ADMIN)) return
//     -EPERM;", so an SAD dump is refused the same as an SPD write.
//     rtnetlink_rcv_msg (net/core/rtnetlink.c): "if (kind != RTNL_KIND_GET &&
//     !netlink_net_capable(skb, CAP_NET_ADMIN)) return -EPERM;". nfnetlink_rcv
//     (net/netfilter/nfnetlink.c): "if (!netlink_net_capable(skb, CAP_NET_ADMIN)) {
//     netlink_ack(skb, nlh, -EPERM, NULL);". The capability is asked first, so no
//     other errno means it is missing.
//   - The request's absent errnos: netlink_create (net/netlink/af_netlink.c) loads
//     the family's module and then answers "err = -EPROTONOSUPPORT;" when
//     "nl_table[protocol].registered" is still false, which is a kernel without
//     CONFIG_XFRM_USER for NETLINK_XFRM and without nfnetlink for NETLINK_NETFILTER.
//     fib_newrule (net/core/fib_rules.c) answers "err = -EAFNOSUPPORT;" when
//     "ops = lookup_rules_ops(net, frh->family);" finds none, and the IPv6 rules ops
//     are built only with "ipv6-$(CONFIG_IPV6_MULTIPLE_TABLES) += fib6_rules.o"
//     (net/ipv6/Makefile). nfnetlink_rcv_batch loads "nfnetlink-subsys-%d" and, when
//     the nf_tables subsystem is still missing, acks "netlink_ack(oskb, nlh,
//     -EOPNOTSUPP, NULL);". The errno is checked against r's own list, because
//     EPROTONOSUPPORT from a NETLINK_ROUTE request says nothing about IPv6 rules.
//   - EACCES: neither the capability check nor a missing family answers it. The
//     security module does: netlink_sendmsg runs "err = security_netlink_send(sk,
//     skb);" before the family sees the message, SELinux's selinux_netlink_send
//     answers "rc = sock_has_perm(sk, perm);" and selinux_socket_create answers
//     avc_has_perm(..., SOCKET__CREATE, ...), whose avc_denied answers "return
//     -EACCES;" (security/selinux/avc.c), and AppArmor's aa_check_perms sets "error =
//     -EACCES;" (security/apparmor/lib.c), as bypassFaultOf (transport/udp.go)
//     quotes. No capability changes it.
//   - A dataplane.PolicyOwnedError is the XFRM backend's ownership record refusing an
//     entry before it sends anything, so no errno is involved: NotAsked. The other
//     producers never answer it.
//   - Anything else (EINVAL for an entry the kernel refused, ENOMEM, ENOBUFS, an error
//     from a backend that is not the kernel's, as VPP's): Unknown. EEXIST and ENOENT
//     are not privilege answers either: XFRM_MSG_UPDPOLICY never answers EEXIST, and
//     iprule.Source.Add already takes an existing rule as success.
//
// The errno constants come from syscall, so the classifier builds on every platform.
func (r netlinkRequest) faultOf(err error) netlinkFault {
	if _, owned := errors.AsType[*dataplane.PolicyOwnedError](err); owned {
		return netlinkFaultNotAsked
	}
	if errors.Is(err, syscall.EPERM) {
		return netlinkFaultCapability
	}
	for _, errno := range r.absent {
		if errors.Is(err, errno) {
			return netlinkFaultKernel
		}
	}
	if errors.Is(err, syscall.EACCES) {
		return netlinkFaultSecurityModule
	}
	return netlinkFaultUnknown
}

// cause answers, in the operator's words, why a request of kind r was refused with
// err: a clause each site puts after its own subject. Safe for concurrent use.
func (r netlinkRequest) cause(err error) string {
	return r.causeOf(r.faultOf(err))
}

// causeOf is cause for one fault.
func (r netlinkRequest) causeOf(fault netlinkFault) string {
	switch fault {
	case netlinkFaultNotAsked:
		return "the kernel was not asked: Ze's own record of the SPD holds that selector for another entry, as the error says"
	case netlinkFaultCapability:
		return "ze lacks CAP_NET_ADMIN, which the kernel asks of this " + r.family + " request"
	case netlinkFaultKernel:
		return "the kernel is built without " + r.feature + ", so it cannot take this " + r.family + " request"
	case netlinkFaultSecurityModule:
		return "a security module (SELinux or AppArmor) refused ze this " + r.family + " request"
	case netlinkFaultUnknown:
		return "Ze does not read the answer as a missing capability, a kernel built without " + r.feature +
			" or a security module's denial, so the error is the only account of the cause"
	case netlinkFaultUnspecified:
		panic("BUG: engine: netlinkRequest.faultOf answered netlinkFaultUnspecified")
	}
	panic("BUG: engine: undeclared netlinkFault")
}

// fix answers the fix for a request of kind r refused with err. again is the step
// that applies the entry once its cause is gone ("commit again"); empty means a
// restart of ze, which installs the running configuration again. The capability and
// kernel fixes restart ze themselves, so they name again only when it is not that
// restart. privilege.CapabilityRemedy words the capability fix for this process, and
// privilege.RestartRemedy every other restart: systemctl only under ze.service, the
// stop-and-start steps otherwise. For a cause Ze cannot name the fix is again alone,
// or the restart command alone, which each site's text says follows the removal of the
// cause the error names. Safe for concurrent use.
func (r netlinkRequest) fix(err error, again string) string {
	return r.fixOf(r.faultOf(err), again)
}

// fixOf is fix for one fault.
func (r netlinkRequest) fixOf(fault netlinkFault, again string) string {
	switch fault {
	case netlinkFaultCapability:
		return thenAgain(privilege.CapabilityRemedy("CAP_NET_ADMIN"), again)
	case netlinkFaultKernel:
		return thenAgain("boot a kernel built with "+r.feature+" (Ze's own kernel carries it), then restart ze: "+
			privilege.RestartRemedy(), again)
	case netlinkFaultSecurityModule:
		step := again
		if step == "" {
			step = "restart ze: " + privilege.RestartRemedy()
		}
		return "find the denial with " + dataplane.SecurityModuleDenials + ", allow ze its " + r.family +
			" requests in that policy, then " + step
	case netlinkFaultNotAsked, netlinkFaultUnknown:
		if again == "" {
			return privilege.RestartRemedy()
		}
		return "remove the cause the error names, then " + again
	case netlinkFaultUnspecified:
		panic("BUG: engine: netlinkRequest.faultOf answered netlinkFaultUnspecified")
	}
	panic("BUG: engine: undeclared netlinkFault")
}

// thenAgain appends again to a fix that ends in a restart, when again is a step the
// restart does not take.
func thenAgain(fix, again string) string {
	if again == "" {
		return fix
	}
	return fix + ", then " + again
}

// XFRMRequestCause answers, in the operator's words, why the kernel refused a
// NETLINK_XFRM request of the dataplane with err (netlinkRequest.cause), for the
// `show vpn ipsec dataplane` commands. Safe for concurrent use.
func XFRMRequestCause(err error) string {
	return xfrmRequest.cause(err)
}

// XFRMRequestFix answers the fix for a NETLINK_XFRM request of the dataplane refused
// with err; again is the step that repeats the request once its cause is gone
// (netlinkRequest.fix). Safe for concurrent use.
func XFRMRequestFix(err error, again string) string {
	return xfrmRequest.fix(err, again)
}
