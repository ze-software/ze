# Interface address replay: independent source preparation

CLEAR for the frozen current address-owner scope; no concrete source blocker.
This review does not cover the concurrently authored connected consumer or BGP
Default implementation. It does not create a formal ReviewGate/CLEAN artifact.
The retained431-path review at5797 remains unchanged until immutable admission.

Actual scope is eight Go paths, including the new typed signal registration:
- internal/component/iface/register.go
- internal/core/iface/events/events.go
- internal/core/iface/events/register.go
- internal/plugins/iface/netlink/monitor_linux.go
- internal/plugins/iface/netlink/monitor_replay_linux_test.go
- internal/plugins/iface/vpp/ifacevpp.go
- internal/plugins/iface/vpp/monitor.go
- internal/plugins/iface/vpp/monitor_replay_test.go

Read every changed hunk/current reached producer and both full new test files
(363netlink/353VPP lines), forwarding/system contracts, typed SignalEvent producer,
namespace registration, VPP dump/fill/addrInfo and full seed/start/stop/publication
paths. Three lenses: ownership/security; tests/wiring/race/removal; style/docs.
Read the scoped docs/features/interfaces.md delta and source anchors.

Netlink serializes the retained cache with live changes, replay and link removal
inside its existing owner. Renames withdraw the old identity before publishing
and retaining its replacement. Tentative/DAD-failed IPv6 transitions withdraw
retained addresses; deletion reclaims all cached entries for that ifindex. Replay
admission is bounded/coalesced and inactive monitors are never started by a request.
The ready barrier publishes the request subscription before startup returns.

VPP seeds both address families from its native API and serializes complete
Add/Remove calls plus event publication against startup via addressMu. Event
publication/replay/deletion share publicationMu; requests only enqueue, so a
reentrant request does not re-enter that mutex. Stop revokes replay subscription
and joins the owner; seeded/address API failures refuse rather than invent an
empty complete inventory. Existing VPP external-address-notification limits are
not treated as a new support claim.

Tests retain real owner loops and synchronous reentrant bus delivery. They
require warm-provider replay, correct host/unit/index, repeated exact address
sets, removed/tentative/deleted exclusions, renames, stop/restart cleanup,
coalescing and no stale add after withdrawal under held publication. Positive
replacement publications and worker joins remain. No test was weakened here.

Actual root receipts read: job-unit-pkg-6f5f7efe.log has both netlink/VPP package
PASS; job-iface-address-replay-outcome-coverage-d4800f42.log names every focused
replay outcome PASS and reports focused scope6.9%/15.6%, not whole-package
coverage. No test/build/formatter/product execution by this reviewer.

Frozen hashes:
component/iface/register.go e94cb36a6b179c29246ab685a0b1b3c137842a41613e2849c4ed773552493868
core/iface/events/events.go db4bf03a55736710b4ee5b52c50e895051ebdd81db4e3723f8805b284ddf2ac8
core/iface/events/register.go c4b02d9a2762d668d90db70eced30b4b0946689b68534dc941db6b8fabaed289
netlink/monitor_linux.go 051f3da243ea3d48c10f1a2c1870e10a52f9d2685add2a2fc7251c1c6bb850bf
netlink/monitor_replay_linux_test.go cfe3608bdcccbb38843ea43184f15f6cc6799b339d86d10bb78404d0f477a74d
vpp/ifacevpp.go 3ac1e01520dfbe1bf8bfde20a420f2a9e967e0671367ac9606d2d0979f161bee
vpp/monitor.go 10643df194fad780ac5e73f157d0621b6c7a22af662c1d24c0ab4ba2dd4adbd1
vpp/monitor_replay_test.go eb711994c9a9ecc41eee33648daf50d5c3b401de63598f449b10bf56bce2802c

Root owns immutable landing, final census and live full lab proof. No tracked
edits, commits, subagents or closure by this reviewer.

