# Native IPv6 fragment TDD packet

Source only; not compiled, formatted, executed, or accepted. No existing test
or implementation file changed. Main owns installation and all execution.

Install the one source file as the new canonical file
`internal/component/l2tp/pppoe/dataplane/kernel/native_ipv6_fragment_integration_linux_test.go`.
It reuses the current real carrier fixture and needs no future API.

| Root | Required observation |
| --- | --- |
| `TestNativeIPv6FragmentUseful` | Real PPP carrier receives a fragmented, checksum-valid global UDP DHCPv6 payload, with Destination Options before Fragment. Existing four-lane controls remain useful. |
| `TestNativeIPv6FragmentDistinctOwners` | Two distinct live PPP units use identical global source/destination, ID, complete UDP bytes, and checksum. First A plus last B never completes anywhere. Later last A plus first B completes independently on both original carriers. |
| `TestNativeIPv6FragmentRetiredOwner` | A first fragment is queued on real PPP before deletion. Exact-index/name Ethernet receives the last half but cannot complete the old PPP queue or satisfy the stale carrier. Ethernet then completes its own queue under that same tuple/ID and remains useful. |
| `TestNativeIPv6FragmentRename` | A queued original PPP datagram completes after physical rename; a useful Ethernet survivor holds the original name. |
| `TestNativeIPv6FragmentOrdinaryEthernet` | Ordinary untagged veth retains useful native reassembly, including the extension header. |

All original global pairs are added through the admitted data plane's public
`AddAddressP2P` and acknowledged with `Connect`. Native address readback rejects
missing or tentative addresses. Only the independent ordinary Ethernet control
uses direct netlink address installation.

The global wire tuple is `2001:db8:6::2` to `2001:db8:6::1`, UDP 546 to 547.
It deliberately avoids link-local addresses, whose native fragment key can
already include ingress ifindex. The identical mixed-owner datagram prevents
UDP checksum rejection from hiding an ownership merge. A literal independently
summed UDP vector and literal first/final Fragment header vectors pin encoding.

Before any lifecycle cut, bounded polling requires both `Ip6ReasmReqds` to
advance and `FRAG6 inuse` to increase. Crossed inputs must reach the native
reassembly request count. Useful unfragmented datagrams then close the socket
observation window, rejecting any earlier unexpected fragment payload, and
`Ip6ReasmOKs` must remain unchanged. Successful independent completions require
the exact completion count. Poll/read deadlines only bound failures. There is
no time-only positive settlement and no new goroutine to join.

Current source pins: `newNativeV6Fixture`, `nativeV6Fixture.open`,
`nativeV6Acquire`, `nativeV6Endpoint.inject`, `nativeV6Fixture.retire`,
`nativeV6Fixture.rename`, `nativeV6ReplacementNew`, `nativeV6Useful`,
`nativeV6Replacement.useful`, `nativeV6Checksum`, `nativeV6CheckVector`, and
`nativeV6DHCP` in the existing carrier fixture. Original construction remains
`Access.Open` then `Dataplane.OpenIPv6Carrier`; the new helpers fabricate no
owner, admission, interface, receive result, feature grant, or fragment queue.

Main must read pinned Linux `net/ipv6/proc.c` and `net/ipv6/reassembly.c` to
confirm the native queue/counter producer layout before accepting that poll.
No extracted source root was supplied to this agent. `gopls symbols` could
not load because the sandbox denied an existing external Go cache entry; no
cache mutation, retry, escalation, build, compiler, test, or formatter followed.

Do not classify a failed prerequisite, original RS control, global pair
installation, native counter read, or first-queue poll as ownership RED. First
establish the existing original-traffic root on the corrected current fixture.
Then a RED at crossed completion, stale receive, or original rename behavior
is a candidate ownership regression, to diagnose against the native producer.
These tests do not activate or validate private kernel patches 0012 through
0013C and do not establish kernel feature acceptance.

Main-owned selector after canonical installation and a fresh native binary:

```text
^TestNativeIPv6Fragment(Useful|DistinctOwners|RetiredOwner|Rename|OrdinaryEthernet)$
```

Owed command inside the existing matched Ze-kernel native launcher, from the
canonical kernel package working directory, using the fresh package binary:

```sh
/tmp/root-ppp-ipv6.test -test.v -test.count=1 -test.timeout=5m -test.run '^TestNativeIPv6Fragment(Useful|DistinctOwners|RetiredOwner|Rename|OrdinaryEthernet)$'
```

Main must rebuild the package binary after installation; the existing binary
does not contain these roots. Use the current host's architecture, feature tags,
matched Ze kernel/image and established serialized build/launch action. The
command above is the exact test-binary selector, not an instruction to reuse a
stale guest binary or to skip the host's native job wrapper.

Retained temporary item: this directory contains the current small source
packet and this owning handoff. Its purpose is independent review and later
canonical installation. Remove the packet after the canonical file and its
main-owned review/run evidence supersede it. No obsolete export, server,
screenshot, build copy, or disposable output was created here.

## Original-traffic readiness proposal

`readiness.patch` is a source-only proposal against the current canonical
`native_ipv6_carrier_fixture_integration_linux_test.go`. Do not install it
before Main's independent test judge approves. No canonical file was changed,
and no formatter, helper, compiler, build or test was run for this proposal.

The compiled original-traffic root reached the native body in
`job-ppp-ipv6-corrected-original-traffic-actual-7436a54b.log` and failed because
its immediate address read observed TENTATIVE. This is setup readiness evidence,
not an IPv6 ownership result. The proposal replaces that immediate read with
the existing `internal/test/fixture.Poll`: bounded by `nativeV6Wait`, each check
reads real state, immediately fails on a native query error or DADFAILED, and
requires local `fe80::1` with its real peer `fe80::2/64` and no TENTATIVE flag.
The netlink decoder represents IFA_LOCAL as `/128` and carries the request's
prefix length on the peer; both are checked. Failure reports the context cause
and last actual address snapshot. The multicast check likewise polls the real
local-table `ff00::/8` RTN_MULTICAST route, retaining its last snapshot on failure.
No flags, address completion or admission route are fabricated. Administrative
UP, MULTICAST, disable_ipv6 and all wire assertions remain intact.

Primary source read anew via GitHub `fetch_file`: Linux `v7.3`,
`net/ipv6/addrconf.c`, blob `c90ee6dd7446cd10064c9fb95484b538b3ffc18c`.
[`ipv6_add_addr` at line 1155](https://github.com/torvalds/linux/blob/v7.3/net/ipv6/addrconf.c#L1155)
marks addresses tentative unless NODAD. `inet6_addr_add` at 3092 calls
`addrconf_dad_start`; that function at 4197 queues zero-delay work. In that
worker, `addrconf_dad_begin` at 4143 clears TENTATIVE for accept_dad=0, then
calls `addrconf_dad_completed` at 4343 to publish the valid address. Thus the
address-install ACK precedes readiness even though no DAD probes are requested.
`addrconf_add_dev` at 2571 invokes `addrconf_add_mroute`, whose configuration
at 2553 declares exactly the observed local-table ff00::/8 multicast route.
Polling observes that producer; it never retries installation or changes mode.

Main owes independent judgment, then canonical application, formatting and a
fresh package binary before rerunning `TestNativeIPv6CarrierOriginalTraffic`
through the existing matched Ze-kernel native launcher. The earlier native
failure cannot be called ownership RED. Retain `readiness.patch` only until its
approved canonical change and Main's retained execution evidence supersede it;
then remove this proposal and this temporary section.

### Successor after fixture import-cycle failure

`readiness-no-fixture.patch` applies to the current formatted canonical file
that already contains the preceding readiness proposal. The preceding
`readiness.patch` is not applied again. Main's fresh compilation exposed an
import cycle through `internal/test/fixture`, `le/rfc`, `interoplab/bgp`, the
plugin composition root and `pppoe-kernel` back to this kernel package. The
successor removes that one new import. Each existing readiness method now owns
its direct standard-library ticker/context select; no polling framework, helper
or goroutine is introduced. The first observation is immediate, later reads
wait on the ticker or context, and both the 201-observation cap and 2-second
context remain. All address/peer/mask/flag and route predicates, immediate
native-query/DADFAILED failures, context-cause and last-snapshot diagnostics,
administrative prerequisites and wire assertions remain. Deferred cancellation
and ticker stop cover success, fatal failure, cancellation and exhaustion.

This successor is source only, awaiting Main's independent judge and Primary's
release of the canonical file hold before Main applies it. No canonical source,
formatter, helper, compiler, build or test was run or changed for it. Retain the
small successor patch for judgment/application; remove it with this section
when the canonical change and Main's fresh compile/native evidence supersede it.
