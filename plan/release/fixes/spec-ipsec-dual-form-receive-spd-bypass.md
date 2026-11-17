# Spec: ipsec-dual-form-receive-spd-bypass

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P0 |
| Release reason | Security defect in shipped IPsec: on a Child SA moved to UDP 4500 without a NAT, bare ESP from an authenticated peer is decrypted and delivered with no inbound SPD check, so the peer can inject traffic with any inner source and destination. |
| RFC gaps | RFC4301-5.2-2 |
| Depends | - |
| Phase | Guard landed; current queue counter, SA lifetime and raw/packet matrix accepted; context proof and 75 renewals remain; AC-4 stays open |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Current continuation checkpoint, 2026-11-17

This checkpoint supersedes the progress statements below about unrun raw and packet
populations, source-only SA lifetime, and 79 unexecuted renewals. Their dated receipts
retain their original scope and source identities.

The normal guard phase landed in `bde4a49a63473a875f99beb9754650fcd7521387`: 23 files,
including exact directional Child scopes, complete-capability refusal and runtime
doctor, active 0005 guard `235e022a`, its runtime inputs, packet regressions and owning
documentation. The commit also carries the coordinated shared VPP/PPP diagnostic
catalog; the IPsec tests make no runtime claim for those components. No push was made by
this session.

Main's separate native run 86673 accepted all three raw-socket authority/lifetime tests
in both families and the original eight packet tests: 11 top-level PASS, no FAIL or SKIP
(`job-ipsec-d87-raw-and-packet-matrix-e9855a9d.log`). It ran the unchanged `bbe5ef43`
engine binary on actual d87 kernel SHA256
`d2eb6a14cd07c913deb1a0e9f7ff667230982b64c597ebf8fbee8bc3f80d8f65` and matching image
`43a1d1b2ecccc59c`; check, command and lab exits were zero, interrupted was false, and
the VM stopped. Scoped engine lint 29505 also passed with zero host and Linux
integration issues (`job-ipsec-current-engine-scoped-lint-b4881bf4.log`).

The earlier lifetime run 70977 failed before IPv6 SA removal because local
`Ip6ReasmReqds` did not observe the guard's netfilter queue
(`job-ipsec-sa-lifetime-and-packet-matrix-41ab9d9e.log`); its conditional matrix never
executed. The additive ordinary counter test then went RED at absent `XfrmFrag6Queued`
on d87/d2eb+43a1, run 21799, no skips, check/command/lab exits 0/1/0, and the VM stopped
(`job-ipsec-frag6-counter-old-normal-red-128347fd.log`). These remain observer diagnosis
and TDD receipts, without packet-defect or native RFC proof credit. Fourteen reached
parser sites tolerate the additive field.

Normal counter build 43301 subsequently completed successfully with patch `e3ecc903`
(`job-ipsec-frag6-queue-counter-kernel-9c693ee4.log`). Actual B1 kernel SHA256 is
`f82654a4d1458db3094449274348cf5268e0f09e912776a95b52263f27816536`, matching image
`d9efda5bec4fd7f7`. First boot run 36055 passed the complete capability test and
`TestIPv6XFRMFragmentQueueCounter` in 4.73 seconds
(`job-ipsec-frag6-counter-first-boot-b1ed0e0b.log`). Unique pending admissions advance
the per-namespace counter; authenticated duplicates, completion and whole controls do
not, and the peer namespace stays zero. All eight kernel-input and Colima compiler holds
were released after the accepted first boot run stopped its VM.

The independently approved helper correction reads `XfrmFrag6Queued` for IPv6 and
rejects negatives before conversion. The IPv4 parser and lifetime assertions remain
unchanged. Source SHA256 is
`08e8a270890fb408a80e3f6c74fc0975bbdf28fd48cd33d79fe46a6b8cfdd1d6`; owning architecture
text is `3601b075`. Native helper approval binds `703a94a5..8749dfbd`. Formatting 84982
and Linux compilation 96345 passed; current static arm64 binary SHA256 is
`b1a1994cfb308964931342ebe052101fc6862295cb7a0ab4c64c18a74892fb19`
(`job-ipsec-sa-witness-linux-build-2cec0089.log`). Exact lifetime run 55413 passed in
7.08 seconds, both families and both arrival orders
(`job-ipsec-sa-witness-lifetime-b1-383e1e3c.log`). Separate run 59439 passed the three
raw tests and original eight packet tests: eleven top-level PASS
(`job-ipsec-sa-witness-packet-matrix-b1-0a3ac6e3.log`). Both used the B1/f826 kernel,
matching d9 image and current b1a1994c binary. Neither skipped a test; check, command
and lab exits were zero, interrupted was false, and both VMs stopped. Scoped engine lint
60545 passed host and Linux integration with zero issues
(`job-ipsec-sa-witness-engine-lint-45efbd1a.log`). The counter and helper phase landed
in `84d13c03ee526447bc06646b4fb652fb35dd9bd1` (three paths, 176 additions and three
deletions); native commit consumed the helper approval. These normal regressions retain
bounded scope; full context discrimination and RFC evidence remain open.

The first current renewal is recorded: native 19280 exited 0 after the exact
`TestSPDPolicyOrderInsideBypassBandRefused` clean PASS and its own producer-halt FAIL
(`job-ipsec-rfc4301-spd-order-proof-64ae3e54.log`). It wrote only the negative unit in
`rfc/discrimination/rfc4301/RFC4301-4.4.1-4.json`, producer seal `33222aa0e1c35ebb`; the
existing positive record is preserved. This is a function-dependency proof of the
partial tagged claim, with no C-enforcement or whole-requirement credit. Main
subsequently renewed orders 9, 15 and 44 through native runs 94797, 40175 and 17887:
RFC4301-3.2-1, RFC4301-4.4.2-1 and RFC4303-2-1 host-positive claims. Each selected unit
passed clean and failed at its own native installChildSA producer halt; only the three
selected producer seals changed to `17aa3b0ab62f6479`. Receipts are
`job-ipsec-rfc4301-esp-association-proof-05c96c4a.log`,
`job-ipsec-rfc4301-negotiated-selectors-proof-c580bfc1.log` and
`job-ipsec-rfc4303-esp-protocol-proof-e05930b5.log`. These remain dependency proofs of
partial tagged claims, including distinct claims on the same ESP unit. The current
manifest marks orders 58, 9, 15 and 44 RECORDED: 75 commands remain UNEXECUTED, 55 guest
and 20 host. The original 83 rows remain unchanged, and current freshness of the four
historical receipts is still UNKNOWN. A fresh native RFC check remains owed after
renewals.

Both automatic safety holds and the pending owner question remain unchanged: no
constrained-fixture retry or rejected private fault preparation/use is approved. The
generic C-proof design has independent schema, native-image and observer-manifest
reviews CLEAR (draft `2438aa68`, prose commit
`467baad97515e924378d0e7a58a8e56735046f98`); A-R2 and A-R4 stay unvalidated. Its
separate owner design question is PENDING: prepare the two kernel and image pairs
separately (recommended), or let the recorder build them. The design remains UNAPPROVED,
P3 and post-release, with the seven later RFC792 rows preserved and no tooling
implementation or proof credit. That design question grants no approval for either
rejected safety action. The compliant-fragment RED, full AC-4, partial RFC annotation
and separate mapped ICMP-return gap remain open.
## Task

Close the inbound SPD bypass of the dual-form receive path, keeping both ESP
forms on one Child SA, for IPv4 outer headers. Serving both forms over an IPv6
outer header is a gap owed after the first release (AC-8): Ze has no IPv6 IKE
socket, so no IPv6 dual-form SA can be negotiated, and
`plan/post-release/features/spec-ike-ipv6-transport.md` owns that work.

The final-code audit also closes the full-second token-refill defect in AC-9,
preserves a recorded reader-stop event across replacement (AC-19), and verifies
that AC-14's TUN diagnosis becomes silent when no IPsec peer remains configured.
These corrections keep the existing rate, recovery and diagnostic contracts.
It also repairs the existing `ze plugin ike` engine entrypoint (AC-20). The
registration advertised a runnable engine, while its CLI handler returned
before starting it. The handler now uses the shared plugin runner. The
actual-DUT doctor drafts passed with internal and forked IKE engines;
both completed 80-run qualification, meaningful discrimination cuts, promotion
and their promoted suite cases. That correction is committed in `66825526f2`.
The limiter correction is committed in `d5459aae36`.

Investigation of the affected functional suite reproduced a childless reload
handoff race (AC-21). An ESP-group-only reload must let an attached IKE SA enter its owner
loop before asking that loop to adopt the group. The existing bounded channel
handoff keeps the same IKE SPI without publishing ownership early.

AC-4's kernel capability probe also required a test-request correction (AC-22).
The read-only receipt's negative request placed its attribute before the fixed
header because the netlink serializer writes typed data before raw data.
The probe must serialize the fixed header before that attribute while keeping
every capability check and the `EINVAL` rejection expectation unchanged.
The malformed request's failure establishes no kernel defect.

AC-14's device-finding correction is committed in `2d25927741`. Both new
peer-removal daemon cases passed their initial internal and external runs.
Their paired applicability cuts and 80-run qualifications also completed,
and both cases were promoted unchanged to `test/ipsec/`. The full affected
50-case Linux population then passed without failures or skips.

The interop checker's audit also found a false acceptance (AC-23).
`checkESPFormChange` accepted an empty or whitespace-only Ze XFRM snapshot,
so it could report success without observing the state it must verify.
The checker now rejects that missing evidence. This correction does not
complete AC-2's mid-session form-switch or counter checks.

The current firewall baseline exposed two more defects in this receive path
(AC-24 and AC-25). The nft watch and link follower try to enter a namespace
that is already current, so they fail under the exact daemon capability set,
which deliberately excludes `CAP_SYS_ADMIN`. Keep namespace capture and socket
creation pinned, serve a valid captured current namespace without that extra
entry, and refuse an invalid or unauthorized different target without fallback.
A valid descriptor zero must still name its explicit target. Preserve worker
lifetime and teardown. The unreadable-firewall finding must also retain the
actual permission cause without inferring that every permission error means
`CAP_NET_ADMIN` is missing. Neither repair broadens the grant or relaxes the
firewall checker.

The selected full kernel route makes the complete scope receipt a runtime
dependency. Report a missing, partial or unreadable receipt through the
configured engine's doctor callback before any Child SA is attempted (AC-26),
using the same validation and repair commands as installation.

Scoped lint also found two new-test style issues and twelve existing mechanical
IPsec issues (AC-27). Apply their approved corrections while preserving every
assertion, packet byte, RFC tag and claim. Public refusal identity and wrapped
cause, plus the problem, explanation, fix and severity, must remain verified.
The bounded nine-path cleanup landed in `a62375a383`, with its scoped
verification recorded below. Broader outstanding checks retain their own scope.

The reached namespace tests also exposed a shared counter-observer defect
(AC-28). `sadSelStat` must read `/proc/thread-self/net/xfrm_stat` so a caller
that changes the locked OS thread's network namespace observes that namespace,
not the process leader's. Preserve the parser, every delivery and exact counter
assertion, packet input, RFC tag and claim. Update only the reader path and its
corresponding diagnostic/comments through the independently judged correction;
the existing fragment-permission contract and unrelated changes stay unchanged.

Preserve the carried raw-sender captured-authority contract through ordinary
IPv4 and IPv6 packet regressions (AC-29). Cover matching, missed and absent
OUT socket policies, captured marks and bound-device values, independent or
replacement policy generations, and closing a socket while fragments remain
queued. Same-authority permitted controls must survive in both arrival orders.
This adds evidence for the existing architecture contract, not new kernel
behavior or an RFC proof. The regression source is authored and independently
reviewed CLEAR; formatting and Linux build passed. Its first packet run exposed
a legitimate IPv4 output defect: both supposed ordinary route stages invoked
XFRM before the intended captured-authority lookup. Repair those two calls in
`raw_sendmsg` and `xfrm_fragment_route` without changing the final captured
lookup, guards or assertions. The source repair is reviewed, built and accepted
in its two formerly failing IPv4 cases. The separate full raw/packet acceptance
is recorded in the current continuation checkpoint above.

AC-4's existing queue-lifetime obligation also has an ordinary SA-replacement
regression: after a fragment is queued, deleting and recreating the same public
SAD identity/key must not let the replacement complete the removed SA's queue.
Whole and fresh same-generation fragment controls remain deliverable, in both
families and arrival orders, with exact generation counters. The new source
was independently reviewed CLEAR (`d030`), formatted and built. IPv4 passed
both orders; IPv6 stopped at the unsound local-counter witness before SA
replacement. The current checkpoint records the additive counter test and
remaining work; this adds no RFC claim or new acceptance criterion.

**The defect, confirmed by an A/B interop run on 2026-11-10.**

1. A Child SA whose IKE SA moved to UDP 4500 is installed with UDP encapsulation
   even when no NAT was detected: `newChildSA`
   (`internal/component/ike/engine/create_child.go`) sets `UDPEncap` when
   `sa.localPort == transport.NATTPort`.
2. The inbound XFRM state therefore carries an ESP-in-UDP template. strongSwan
   6.1.0 (`child_create.c`, `this->child.encap` set from `COND_NAT_ANY`) sends
   bare ESP when it sees no NAT.
3. The kernel refuses the bare form (`XfrmInStateMismatch`). Ze's
   `espFormReceiver` (`internal/component/ike/dataplane/espform_linux.go`,
   `run`, sent by `rawESPFormInjector.inject`) reads it from a raw
   `IPPROTO_ESP` socket carrying a per-socket policy bypass, and re-sends it as
   ESP-in-UDP to Ze's own address through an `IPPROTO_RAW` socket.
4. That datagram goes out `lo`, so after decapsulation `skb->dev` is `lo`.
   Linux sets `disable_policy=1` on loopback (`net/ipv4/devinet.c`, lines 1582
   to 1585 at 7.3), so the input path and the local-input route set
   `IPSKB_NOPOLICY` (`net/ipv4/ip_input.c` line 415, `net/ipv4/route.c` line
   2446), and `__xfrm_policy_check2` (`include/net/xfrm.h` line 1285) returns
   before any policy is consulted.
5. Result: for every bare-ESP datagram on such an SA, the inner packet is
   delivered with no inbound SPD check. The SA's selectors and the
   `unmatched discard` catch-all are not enforced on that traffic.

RFC 4301 Section 5.2, step 4: "Then match the packet against the inbound
selectors identified by the SAD entry to verify that the received packet is
appropriate for the SA via which it was received." Step 5: "If an IPsec system
receives an inbound packet on an SA and the packet's header fields are not
consistent with the selectors for the SA, it MUST discard the packet."

RFC 7296 Section 2.23 is the reason the re-presenting path exists, and the fix
keeps it: "all devices MUST be able to receive and process both
UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."

**Evidence.**

| Run | Result | Log |
|---|---|---|
| interop `icmp-error-payload-check`, as committed (strongSwan sends bare ESP) | FAIL: "the error embedding 10.2.0.1 -> 10.7.0.5, outside the SA's selectors: ze delivered 3 Time Exceeded (Icmp.InTimeExcds), want the error dropped" | `tmp/session/2026-11-09-222726/scratch/ab-plain.log` |
| the same scenario with strongSwan `encap = yes` (ESP-in-UDP, the state's own form, received on the real interface) | PASS | `tmp/session/2026-11-09-222726/scratch/ab-encap.log` |

The earlier dual-form work met this blind spot from the other side: its Phase 5
evidence records that `__xfrm_policy_check2` short-circuits on loopback, so the
QEMU probe could not see the receive-side policy defect
(`plan/release/validation/spec-ipsec-esp-dual-form-receive.md`).

## Decision (owner)

Thomas chose option A ("1 A"): a tc-ingress BPF program adds the UDP header to
bare ESP for the watched SPIs, so the kernel receives the state's own form on
the real interface and runs every check, with no userspace copy over loopback.

Design research then found that A alone cannot receive fragmented bare ESP: tc
ingress runs in `__netif_receive_skb_core` (`sch_handle_ingress` → `tcx_run`,
`net/core/dev.c`) before IP reassembly (`ip_local_deliver` → `ip_defrag`,
`net/ipv4/ip_input.c` line 257), so the program sees each fragment alone, a
non-initial fragment carries no SPI, and no first fragment can carry a correct
UDP length. Linux copies DF from the inner header to the outer one
(`net/xfrm/xfrm_output.c` line 311), so a large inner packet without DF yields
fragmented outer ESP. RFC 7296 Section 2.23 ("at any time") settles that those
must still be received. The coordinator therefore decided A+B:

| Path | Carries | Device the decapsulated packet arrives on |
|---|---|---|
| A, tc-ingress rewrite (fast path) | every whole bare datagram for a watched SPI whose outer header it can rewrite | the real receiving interface |
| B, TUN re-presentation (complete path) | whatever bare ESP for a watched SPI the fast path left alone: reassembled fragments, IPv4 with options, IPv6 with extension headers (as decided; IPv6 is now owed after release, see the revision below), a link the fast path is not attached to | the Ze TUN `ze-espform`, `disable_policy=0`, IPv4 `rp_filter=2`, address 192.0.0.8/32 scope host |

The cost of B: a firewall rule keyed on the input interface sees `ze-espform`
for the traffic the complete path carries, not the link it arrived on.

The `lo` injector is deleted (`ai/rules/no-layering.md`). Both paths are
policy-checked. Option C (clear `disable_policy` on `lo`) stays rejected: it
breaks loopback traffic under `unmatched discard`. Option D (template-free
state, or no re-presentation) stays rejected: it loses RFC 7296 Section 2.23.

Accepted settled points (coordinator, 2026-11-12):
- the fast path attaches to every non-loopback link while any SPI is watched,
  following link creation through a netlink link subscription; an unsupported
  link type is logged by name and served by the complete path;
- pure-Go `asm.Instructions`, no C, no bpf2go, no clang, tested through
  `BPF_PROG_TEST_RUN`;
- attach at `link.Head()`, return `TCX_NEXT`; UDP source port is the
  template's `encap_sport`; SPI-keyed map with a peer and local address check;
  fragments skipped;
- bare ESP over IPv6 is handled the same way, both paths. Revised during
  implementation (2026-11-12): Ze has no IPv6 IKE socket
  (`transport/udp.go` `newTransport` listens on `udp4`, and `engine/probe.go`
  refuses an IPv6 peer), so no IPv6 dual-form SA can exist. IPv6 serving is a
  gap owed after the first release, owned by
  `plan/post-release/features/spec-ike-ipv6-transport.md`. The IPv6 program
  variant stays, tested, and nothing writes its map yet (AC-8).
- the TUN clone device (coordinator, 2026-11-12): where `/dev/net/tun` is
  missing and the kernel allows it, Ze creates it (`mknod` of `c 10 200`, mode
  0600, with `/dev/net` created first when that is missing, and removed again
  when the node cannot be made; a node that exists is never replaced), in
  `openESPFormTUNClone` and `createESPFormTUNNode`
  (`ike/dataplane/espform_tun_linux.go`). The engine makes and opens the device
  on every configuration apply that configures an IPsec peer, at start and on
  reload (`recordESPFormTUN`, `ike/engine/apply.go`), as a Child SA install
  would. The `ipsec-esp-form-fast-path` doctor check only reads the outcome
  that apply recorded and reports it under `doctor-ipsec-esp-form-tun`. Reason:
  a runtime doctor check must only read, never change the host, and failing at
  apply shows the problem before the first Child SA is refused.
- the firewall gate (owner decision, coordinator, 2026-11-12): bare ESP for a
  watched SPI stays ESP until after INPUT whenever a firewall could drop it, so
  the firewall decides on it as it did before this spec. The fast path rewrites
  at tc ingress, ahead of every netfilter hook, so a datagram it rewrote would
  reach a `protocol esp` rule as UDP to port 4500; the complete path reads its
  copy after `NF_INET_LOCAL_IN`. The agreed design is
  `tmp/session/2026-11-09-222726/scratch/espform-firewall-design.md`. As built:
  - the gate is a one-element BPF array map, `ze_esp_gate`
    (`espFormFastPathGateMap`, `ike/dataplane/espform_program_linux.go`), that
    the program reads before it reads the packet; 0 answers `TCX_NEXT` with the
    packet untouched. It starts at 0 and returns to 0 whenever the ruleset
    cannot be read or watched (fail closed). The manager in
    `ike/dataplane/espform_fastpath_gate_linux.go` (`judgeFirewall`,
    `publishFirewall`, `writeGateLocked`) keeps an epoch, a hold count and a
    lost-firewall flag, and writes 1 only from a judgment whose epoch did not
    move while it read and with no hold open;
  - the analyzer reads the KERNEL ruleset, not Ze's configuration, because
    `firewall.LastApplied` is a package global that a forked IKE engine holds
    empty: `espFormFirewallBlocks` (`ike/dataplane/espform_firewall_linux.go`)
    walks every base chain on the `ingress`, `prerouting` and `input` hooks of
    the `inet`, `ip`, `netdev` and `bridge` families and reads base chains
    only: a rule that can match ESP and sends it anywhere but accept or drop
    (jump, goto, return, a verdict map) is unsafe, and its target is never
    walked (`espFormJudgeVerdict`). The reason is `nft_do_chain`
    (`net/netfilter/nf_tables_core.c`, v7.0): a return resumes after the last
    jump, and in a base chain, with nothing on the jump stack, it skips every
    later rule and meets the policy; a goto pushes nothing, so the end of its
    target is a return from the chain that went to it. A walk that read on past
    a return or a narrowed goto would meet a later ESP accept that the ESP which
    left never reaches, and call safe a ruleset whose policy drops that ESP. It
    excludes a rule only when the rule's own first two expressions are a protocol
    load and an equality compare that fails for every IPv4 ESP packet, and judges
    any expression off its harmless list as unsafe, so the
    first unsafe site (family, table, chain, hook, rule handle and comment,
    reason) is reported. The reader (`espFormKernelReader`,
    `ike/dataplane/espform_firewall_kernel_linux.go`) dumps tables, chains and
    rules itself on one netfilter socket (`espFormTableFromMessage`,
    `espFormChainFromMessage`, `espFormRuleFromMessage`), because the vendored
    `Conn.GetRules` drops every expression the library has no type for, and
    some it has (`exprsFromBytes`, "continue // unsupported expression type"),
    so a netdev `fwd` would read as a rule with no verdict, and
    `Conn.ListTables` and `Conn.ListChains` hand on a message the kernel
    flagged `NLM_F_DUMP_INTR`. Every dump message flagged `NLM_F_DUMP_INTR` is
    refused (`espFormDumpMessage`, `errESPFormDumpInterrupted`), so a dump read
    across a commit fails the judgment rather than publish two rulesets
    spliced;
  - the register model is deleted (review round 4, 2026-11-12, coordinator
    decision): a rule is excluded only when its own first two expressions are a
    load and an equality compare of exactly the register that load wrote
    against one octet that is not ESP's value (`espFormExcludesESP`). The load
    is `meta l4proto` in any family walked, the IPv4 protocol octet (network
    header offset 9, one octet) or `meta nfproto`, the last two only in an
    `inet` or `ip` table; ESP's value is 50, or `NFPROTO_IPV4` for nfproto
    (`espFormFirstLoad`, `espFormESPValue` in `espform_firewall_linux.go`, the
    kernel lines quoted at `espFormFirstLoad`). `!= 50`, the same pair later in
    the rule, and every other load exclude nothing. The definite accept
    (`espFormAcceptsESP`) is a protocol load, a compare for equality with 50,
    at most a counter, then accept, and nothing else. Every other rule passes
    only when each expression is on the closed harmless list
    (`espFormHarmless`) and its verdict is accept, break or continue; a meta or
    ct write (mark set, ct mark set, notrack), a payload or exthdr write (a DSCP
    set included), dup, dynset, a lookup with a data output, an objref, quota
    and connlimit close the fast path with their own reasons ("meta or ct
    set", "packet rewrite", "dup", "set update", "map lookup", "object
    reference", "expression off the harmless list");
  - the program passes every VLAN-tagged skb untouched to the complete path
    (review round 3, 2026-11-12): `skb_vlan_untag` runs before TCX in
    `__netif_receive_skb_core`, so on a VLAN's parent link TCX and the netdev
    hooks see one skb whose protocol is the inner IPv4 while a rule there can
    still match the tag (`nft_payload_copy_vlan`); the program tests
    `vlan_present` and, on the Ethernet variant, that the frame's ether type is
    the skb's protocol (`espFormFastPathInstructions`,
    `espform_program_linux.go`);
  - the fix a blocking site names follows who writes its table (review round
    3, 2026-11-12): `espFormFirewallAccept` answers the nft insert for a table
    not named `ze_`; for a `ze_` table it calls the answer the engine registers
    (`dataplane.RegisterESPFormOwnedTableAccept` from
    `ike/engine/register_esp_form_table.go`, `espFormOwnedTableAccept`), which
    names the producer's declared control for a reserved table
    (`firewall.TableControlOf`), the config editor lines for a table of the
    firewall config, and the ruleset listing for a name the config cannot
    hold;
  - Ze's own firewall commits hold the gate shut while they land: a firewall
    commit observer (`firewall.RegisterCommitObserver`, `applyObserved`,
    `internal/component/firewall/commit_observer.go`) runs inside
    `reconcileMu` just before `Backend.Apply` and calls its after function
    once Apply returns, success, error or panic; the IKE engine registers
    `holdESPFormFastPathForCommit` from `init()`
    (`ike/engine/register_firewall_observer.go`), which takes
    `ESPFormFirewallHolder.HoldESPFormFastPath` (`ike/dataplane/espform_firewall_hold.go`,
    implemented by the XFRM backend). The hold writes 0 before the commit, and
    its release asks the worker for a judgment that begins after it;
  - a commit by any other program is seen through the nftables monitor
    (`espFormNFTWatch`, `startESPFormNFTWatch`), one coalesced signal per
    committed generation. Its socket asks for a 16 MiB receive buffer
    (`espFormNFTWatchBufferOctets`), because a large ruleset can overflow the
    default and end the monitor with ENOBUFS; a watch that fails shuts the
    gate, and the worker opens it again after a wait that doubles from
    `espFormNFTRewatchFirst` (1 s) to `espFormNFTRewatchMax` (30 s), then
    judges the ruleset afresh (`watchLost`, `watchAgain`). A ruleset unread at
    start, a failed dump, or a watch `Close` ended keeps the gate shut until
    the next start; `ESPFormFirewallReading` (`ike/dataplane/espform_state.go`,
    being added while this spec was updated) says which, and the doctor check
    picks its fix from it;
  - six limits are documented, each with what to do about it, in `docs/guide/ipsec.md`
    ("Both ESP wire forms") and `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`:
    rules loaded with iptables-legacy live in x_tables, which no nftables dump
    returns, so the fast path rewrites past them; a tc ingress program or
    filter another owner attached after Ze's program sees the rewritten
    datagram as UDP to port 4500; a foreign commit is seen only after it
    landed, the monitor's latency plus one dump; and an IKE engine run forked
    takes no hold across the daemon's firewall commits, so those carry the
    same monitor window. A datagram in any window is still decrypted and
    checked against the SA's selectors; what it misses is the firewall's
    decision. Two more were added in review round 3 (2026-11-12): ESP the fast
    path rewrote just before a commit's hold began reaches the hooks as UDP to
    port 4500, and if the commit lands first a rule it adds for bare ESP does
    not see it (`firewall.CommitObserver`'s comment); and a rule that drops
    UDP (`meta l4proto udp drop`) is read as excluding ESP yet drops the
    rewritten datagram, which fails closed, and an operator who accepts the
    peer's IKE on UDP port 4500 ahead of it accepts the rewritten datagram too,
    because the program writes the IKE SA's ports (`espFormFastPathPorts`).
- decided in review rounds 4 to 10 (coordinator, 2026-11-12), each with its
  row in Key Design Decisions:
  → Decision: the GTSM listen-port drop (`listenPortTerm`,
  `internal/component/gtsm/gtsm.go`) and the DDoS local drop (`buildDropTerm`,
  `internal/plugins/ddos/local/match.go`) put the protocol match first, ahead of
  the address and port matches, so the rule opens with `meta l4proto` and the
  nfproto guard the lowering emits sits directly ahead of the address load. Stock
  GTSM over IPv4 and an armed DDoS mitigation on a named protocol other than ESP
  therefore keep the fast path on; a policy route that matches an address stays
  closed, and so does a DDoS vector whose protocol has no canonical name, or is ESP.
  → Decision: `MitigationTable` (`internal/plugins/ddos/local/responder.go`) is
  exported as the one builder of the table `applyMitigation` installs, as
  `gtsm.FilterTables` is for GTSM, so the fast path's stock-ruleset tests judge the
  production lowering read back from the kernel rather than a copy of its shape.
  → Decision: a judgment stands only when the ruleset generation read before its
  first dump equals the one read after its last (`espFormFirewallBlocks`, GETGEN);
  two that differ answer `errESPFormRulesetChanged`. Any set lookup is unsafe ("set
  lookup", `espFormLookup`), because a hit runs the matched element's own stateful
  expressions, which the walk never reads. An exthdr expression is harmless only as a
  load, with a destination register and no source register, because the kernel
  picks the strip operation, which can drop the packet, from the registers. A MOBIKE
  move's new addresses go through `xfrmMigrationIPv4`, which refuses an address with
  no IPv4 form.
  → Decision: every change the watch delivers or a judgment sees shuts the gate
  (`firewallCommitted`). A judgment runs at most `espFormNFTJudgePasses` (2) passes;
  one whose every pass saw the ruleset move answers Moving, and the worker judges
  again after `espFormNFTRejudgeFirst` (50 ms), doubling to `espFormNFTRejudgeMax`
  (1 s). A dump message flagged `NLM_F_DUMP_INTR` counts as the ruleset moving, never
  as a failed dump (`espFormRulesetMoved`). The bracket's claim is narrowed: a commit
  stores its new generation before it applies table and chain changes, so a splice
  whose first read already answers the new number passes the bracket, and the NEWGEN
  the watch signals once the commit applied shuts the gate again, which bounds that
  splice by the watch's latency. The `moving` reading (`ESPFormFirewallMoving`)
  overlays the others while the stream lasts, and the doctor reports it under
  `doctor-ipsec-esp-form-firewall` once it is `ESPFormFirewallMovingReportAfter`
  (10 s) old, with `sudo nft monitor` as the fix, the advice to make the committing
  tool (fail2ban, for example) batch its changes or stop it, and the complete path's
  bound (`espFormRate`) meanwhile (`espFormFirewallMoving`).
  → Decision: one removal is never signaled, and it is accepted as a liveness
  limit. When the process that owns a table created with `NFT_TABLE_F_OWNER` exits,
  the kernel removes that table outside any commit (`nft_rcv_nl_event`, then
  `__nft_release_table`, `net/netfilter/nf_tables_api.c`, v7.0), with no generation
  bump and no NEWGEN, so a gate that table shut stays shut until the next commit, by
  any program, judges again. That costs the fast path only, never the firewall's
  decision: an accept in one base chain does not stop another's, so removing a table
  makes no safe ruleset unsafe (the header comment of
  `espform_fastpath_gate_linux.go`; `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`,
  the watch's known limit).
  → Decision: the `Detached` status (`ESPFormFastPathDetached`): a gate write of 0 the
  kernel refuses detaches the program from every link and records the refused write
  as the cause (`detachLocked`); set, remove and gate writes write nothing while
  Detached, and the complete path serves every watched SPI. Its own doctor finding
  (`espFormFastPathDetached`, `doctor-ipsec-esp-form-fast-path`) names a restart,
  `sudo systemctl restart ze` (`ESPFormFixRestart`), never a capability or a kernel,
  because the program had loaded.
  → Decision: the registry is keyed by SPI and local address (`espFormRegistry`, a
  short list of targets per SPI), because Ze draws each inbound SPI at random with no
  check against those in use and XFRM installs two states that share one SPI on two
  local addresses. `Watch(spi, target)` adds or replaces the SA on that pair, and
  `Forget(spi, local)` removes that SA alone, so `RemoveSA` of an outbound SA whose
  SPI equals a watched inbound SPI no longer forgets the inbound one. The fast path's
  map serves the SA watched first (`fastTarget`); the complete path serves each, told
  apart by destination address. `retarget(spi, from, target)` refuses with
  `errESPFormLocalTaken` a move onto a local address another SA on that SPI holds
  (`moveSlotLocked`, the one check of that collision).
  → Decision (review round 11): `MigrateTunnel` asks the receiver first
  (`admitsMove`) and refuses a colliding move before the SAD changes. For an SA the
  receiver does not watch, it then opens and holds the receiver's paths, the raw ESP
  socket and `ze-espform` (`acquire`, counted in `holds`), before the SAD is touched:
  the moved inbound state is templated, so the kernel refuses its bare ESP and only
  the receiver serves that form. An open that fails refuses the move with the
  operator error `xfrmMigrationUnserved`, with nothing changed. RFC 7296 Section
  2.23: "all devices MUST be able to receive and process both UDP-encapsulated ESP
  and non-UDP-encapsulated ESP packets at any time." Then it moves the states and
  policies; a failed move ends the hold (`release`) and rolls back. Then the receiver
  follows (`followMove`): `watchHeld` registers an SA that was not watched on the
  held paths, which cannot fail, and `retarget` moves one that was. A retarget
  failure is logged at Warn with its fix and never rolls the SAD back, because the
  moved states still receive the ESP-in-UDP form. While a hold is counted, a `Forget`
  of the last other SA leaves the paths open.
  → Decision (review round 11): `Watch` takes the whole `espFormTarget`, ports
  included, so `InstallSA` makes one call with the template's ports
  (`UDPEncapSPort`, `UDPEncapDPort`), and the first datagram re-presented already
  carries the ports the state matches.
  → Decision: an SA the fast path does not serve because an SA watched first on
  another local address carries its SPI is one warning each (`espFormSharedSPI`,
  `doctor-ipsec-esp-form-fast-path`). Its fix is `ze clear vpn ipsec sa peer <name>`
  (`ESPFormFixClearPeer`) for the peer that holds it, or `ze show vpn ipsec sa`
  (`ESPFormFixListSAs`) when Ze cannot name that peer as one command word, because
  no command rekeys a single Child SA and clearing the peer gives its new Child SA a
  new random SPI.
  → Decision: the migration collision (`xfrmMigrationCollision`) and the IPv6
  refusals (`validateXFRMMigration`, `xfrmMigrationIPv4`, and `InstallSA`'s
  dual-form refusal) are operator errors (`opererr`) that name the problem, why, and
  the fix.
- decided in review rounds 12 and 13 (coordinator, 2026-11-12), extending the
  same receiver rather than adding a second recovery path:
  → Decision: every host-caused failure to open the raw ESP socket, exempt it
  from the inbound policy check, or configure `ze-espform` carries its own
  operator error. `espFormSocketRefusal`, `espFormBypassRefusal` and
  `espFormOpenFailure` distinguish EPERM (the daemon capability install), EACCES
  (the security-module denial listing), EMFILE (the unit install that sets
  `LimitNOFILE`), ENFILE (the host's `fs.file-max`), ENOMEM and ENOBUFS (the
  free-memory listing), and an unclassified answer (the receiver's warnings).
  `espFormTUNStepRefusal` uses the capability fix for EPERM and the warnings
  otherwise; `espFormTUNSysctlWriteRefusal` names the exact
  `net.ipv4.conf.default` write when the sysctl failure is not a read-only or
  permission refusal. `xfrmMigrationUnserved` renders the cause's fix once;
  a cause that is no operator error passes through as a Ze defect with no
  invented operator remedy. Before the round-12 fix, doctor had no finding
  for these causes, which is why the `ze show doctor` fallback was removed.
  → Decision: the receiver records its last open failure and its time under
  `mu`, clears it on the next successful open (`reopenReaderLocked`), and
  returns it in `ESPFormFastPathState.CompletePathFailure`. The runtime doctor
  check reads that record even when the fast-path counters cannot be read,
  under `doctor-ipsec-esp-form-tun` (`espFormCompletePathFindings`). It reports
  one error when the last apply's TUN record has the same cause, and two for
  different causes (`espFormDeviceFindings`, `espFormSameCause`). No doctor
  check opens a socket or device to learn this state.
  → Decision: a read failure publishes the cause before closing that reader's
  `dead` channel (`read`). A Watch or acquire that finds it dead opens new
  paths or refuses, and the reader's revive worker opens them again while an
  installed SA needs them, without waiting for a new Watch (`servingLocked`,
  `revive`). Its backoff belongs to the receiver, doubles from one second to
  `ESPFormCompletePathReopenMax` (30 seconds), and resets only after a reader
  served for `espFormReopenResetAfter` (one minute), so repeated short-lived
  readers cannot restart a tight retry loop (`reopenWaitLocked`,
  `reviveLocked`). `Close` sets `closed`, retires the reader under `mu`, then
  joins revive workers after unlocking; later Watch and acquire refuse with
  `errESPFormReceiverClosed`. Recovery stays in this spec; it is not left to
  the later doctor-coverage work.
  → Decision: each reader stop is also a transient report-bus error event
  under `doctor-ipsec-esp-form-reader-stopped`. `runEngine` registers
  `espFormReaderStoppedSink` through `RegisterESPFormReaderStopped`, and the
  sink uses the plugin's `RaiseIssue` request with a five-second bound, so the
  stop reaches the daemon from an internal or forked engine and remains in
  `show errors` after a successful reopen. The sink runs outside the receiver
  lock; its code is registered through `RegisterEventCode` and a failed raise
  is logged under the same code. `RaiseIssue`'s per-call request, multiplexed
  pipe writes, direct-dispatch admission and the daemon report-store locks
  support a revive worker calling it alongside another plugin RPC.
  → Decision: ending an acquire's hold without one is a Ze defect, guarded
  before changing the count in `watchHeld` and `release`. `MigrateTunnel`
  defers release while its hold remains live, so a panic before normal
  registration or rollback cannot leave the paths held with no watched SA.
- trafficusage answers `TCX_NEXT` (owner decision on a side defect,
  2026-11-12): its programs returned `TCX_PASS` (0), which ends the TCX hook,
  so a TCX program behind them never ran and `sch_handle_ingress` and
  `sch_handle_egress` skipped every clsact filter on the link (the
  traffic-control policer, the sFlow sampler, the interface mirror). They now
  return `TCX_NEXT` (`tcxNext`, `internal/plugins/trafficusage/program_linux.go`).

The coordinator's brief also said "delete the rate limiter". That was settled
for A alone. Under A+B the complete path still runs in userspace, which is what
the bound protects, so the bound stays on the complete path only.

On record: Thomas chose A ("1 A"). The coordinator decided A+B, after the
fragment finding above. Both stay recorded, because the complete path is the
coordinator's addition to the owner's choice, not a choice the owner made.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`
  → Constraint: `ike/dataplane` is ungated and also serves OSPF (RFC 4552); the change stays inside the package and adds no import of another component.
- [ ] `docs/guide/ipsec.md`, "Both ESP wire forms"
  → Constraint: the KNOWN DEFECT paragraph and the IPv4-only sentence become wrong with this fix and are rewritten in the same piece of work.
- [ ] `docs/architecture/linux-limitations.md`, `LL-XFRM-STATE-ONE-ESP-FORM`
  → Decision: the limitation stays (one state, one form); its "Ze meanwhile" changes to the two policy-checked paths.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`
  → Constraint: names `espFormBypassInboundPolicy` and `PriorityESPFormSocket`; the reader socket and its policy stay, so those rows stay, re-described.
- [ ] `docs/architecture/traffic/traffic-usage.md`, "Pure-Go eBPF, not bpf2go"
  → Decision: programs are built from `asm.Instructions` and loaded from memory; `BPF_PROG_TEST_RUN` tests are load-bearing.
- [ ] `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` (declared by `internal/plugins/ddos/local/match.go`, the vector to firewall term)
  → Constraint: the page names no match order; `buildDropTerm` keeps every match the vector carries and only puts the protocol first, so a mitigation drops the same packets.
- [ ] `rfc/short/rfc5082.md` `RFC5082-3-4` (declared by `internal/component/gtsm/gtsm.go`)
  → Constraint: `listenPortTerm` keeps its four matches and its drop, reordered only, so GTSM drops the same packets and still never drops a Trusted or Unknown one; its producer moved, so the `RFC5082-3-4` record is taken again (batch 7).
- [ ] `plan/release/validation/spec-ipsec-esp-dual-form-receive.md`, Phase 5 Evidence
  → Constraint: the loopback probe cannot observe the policy check; the new kernel test runs on a veth between two network namespaces.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` `RFC4301-5.2-2`; `rfc/full/rfc4301.txt` Section 5.2
  → Constraint: every decrypted inbound packet is matched against the SA's selectors, whatever ESP form carried it.
- [ ] `rfc/short/rfc7296.md` `RFC7296-2.23-10`, `RFC7296-2.23-11`; `rfc/full/rfc7296.txt` Section 2.23
  → Constraint: both forms at any time, fragments included.
- [ ] `rfc/short/rfc3948.md`; `rfc/full/rfc3948.txt` Section 2.1
  → Constraint: "the Source Port and Destination Port MUST be the same as that used by IKE traffic": the synthesized UDP header carries the template's ports.

**Key insights:**
- Kernel sources read: `tmp/session/2026-11-09-222726/scratch/kernelsrc73/`, `kernelsrc73b/`, `kernelsrc-bpf/` (v7.0: `net/core/filter.c`, `net/core/dev.c`, `net/ipv4/esp4_offload.c`, `include/linux/skbuff.h`).
- `bpf_skb_adjust_room(BPF_ADJ_ROOM_NET)` inserts after `skb_mac_header_len + sizeof(struct iphdr)` (or `ipv6hdr`), the fixed header, never after options (`filter.c` `bpf_skb_adjust_room`, `bpf_skb_net_base_len`).
- tc pushes `mac_len` before the program (`tcx_run`), so a packet starts with the L2 header on Ethernet and with the IP header on an L3 device.
- `tcx_run` stops at the first program that returns anything but `TCX_NEXT`, and `sch_handle_ingress` runs the clsact filters only after a `TCX_NEXT` verdict (`net/core/dev.c` v7.2, lines 4485 to 4490). trafficusage returned `TCX_PASS` at HEAD, which ended the hook; it now returns `TCX_NEXT` (`tcxNext`, `internal/plugins/trafficusage/program_linux.go`, Decision above). The rewrite still attaches at the head and returns `TCX_NEXT`, so every program and filter behind it runs, whichever was attached first.
- Without `CAP_PERFMON` the verifier refuses arithmetic on a packet pointer with a register operand ("pointer arithmetic with it prohibited for !root", `sanitize_err`, `kernel/bpf/verifier.c` v7.0), so trafficusage reads the L4 port with `bpf_skb_load_bytes` at a scalar offset (14 + IHL × 4 + port offset) instead of through a packet pointer at a variable offset. The unit's capability set carries no `CAP_PERFMON` (AC-12).
- `esp_input_done2` (`net/ipv4/esp4.c`) raises `km_new_mapping` when the UDP source port differs from `encap_sport`, so the synthesized port is the template's.
- IPv6 UDP with checksum 0 is refused unless the socket sets `UDP_NO_CHECK6_RX`; the fast path marks the skb `CHECKSUM_UNNECESSARY` through `bpf_csum_level` (`__skb_incr_checksum_unnecessary` turns `CHECKSUM_NONE` into it) and leaves a `CHECKSUM_COMPLETE` skb, where it cannot produce a valid checksum, to the complete path.
- After the fast path rewrites a datagram it is UDP, so the raw ESP reader never receives it; a datagram the fast path leaves alone is refused by XFRM and reaches the reader. No datagram is served twice.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/dataplane/espform_linux.go` - `espFormReceiver` watches SPIs, `run` reads refused bare ESP and `rawESPFormInjector.inject` re-sends it through `IPPROTO_RAW` to the local address; `espFormBypassInboundPolicy` exempts the reading socket.
- [ ] `internal/component/ike/dataplane/espform.go` - `espFormRegistry` (SPI to target), `espFormLimiter` (5000/s, burst 10000), `writeESPForm` (IPv4 + UDP header, header checksum left to the kernel), `espFormSPI`.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `InstallSA`, the `AcceptBothESPForms && UDPEncap` branch that calls `Watch` and refuses IPv6; `RemoveSA` calls `Forget`; `Close` calls `espForms.Close`.
- [ ] `internal/component/ike/dataplane/xfrm_migrate_linux.go` - `MigrateTunnel` calls `Watch`/`retarget`/`Forget` around a MOBIKE move, IPv4 addresses only.
- [ ] `internal/component/ike/dataplane/override_guard_linux.go` - cgroup setsockopt program in `asm.Instructions`, exempting Ze's own process (the reader's socket policy).
- [ ] `internal/plugins/trafficusage/attach_linux.go`, `program_linux.go` - TCX attach and `asm.Instructions` packet parsing.
- [ ] `internal/component/ike/engine/create_child.go` - `newChildSA` sets `UDPEncap` on port 4500 with no NAT.
- [ ] `internal/component/ike/engine/child.go` - sets `inbound.AcceptBothESPForms = true` for every inbound state, so an IPv6 Child SA on port 4500 is refused at install today.
- [ ] `internal/component/ike/engine/established.go` - at HEAD, an initiator whose first Child SA failed to install or bind ended the IKE SA; `runInitiator` had cleared `connectFailures`, so `reconnectDelay` answered `reconnectBase` and the handshake and its error repeated every second.
- [ ] `internal/component/ike/engine/create_child.go` - `childCreateSchedule` (in HEAD) paces a childless IKE SA initiator's Child SA creation: `childCreateFirst` 30 s, doubling on each refusal to `childCreateMax` 300 s.
- [ ] `internal/component/firewall/registry.go` - `ApplyAll` holds `reconcileMu`, merges and verifies the tables, then calls `Backend.Apply` once; the nft backend commits in one atomic flush.
- [ ] `vendor/github.com/google/nftables` - `Conn.ListTables`, `ListChains`, `GetRules` and the generational monitor; `exprsFromBytes` skips every expression it has no type for.

**Behavior to preserve:**
- One Child SA receives both ESP forms with no rekey and no delete (`TestEncapEstablishedSAServesAPeerFormChange`, interop `esp-form-change`).
- An SA that asks for both forms and cannot be served is refused at install (`errNoESPFormReceiver`, fail closed).
- The reader's socket policy is listed as `esp-form-socket`, and the override guard lets it through.
- MOBIKE retargets the watched SPI's addresses and ports.

**Behavior to change:**
- The refused form reaches XFRM on a device whose inbound policy check runs.
- None for IPv6: an SA that asks for both forms over IPv6 is still refused at install, now with an operator error that names the gap, and a MOBIKE move to IPv6 endpoints is refused the same way (AC-8).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Peer-local ESP form stimulus | `internal/le/interoplab/ipsec/peerform/` `parseCommand`, `runAction`, `migrationPayload`, `parsePacket`, `parseEchoReply` | Portable and native Linux packages passed; the live bare/UDP/bare scenario passed | AC-2 uses exact outbound-SA atomic migration and matching wire/reply observations, preserving SPI, identity and replay progress; the as-used binding fault failed at the first actual bare reply and the post-clean run passed; the later comment-only input is unexecuted |
| Image-owned helper staging | `internal/le/interoplab/ipsec/register_images.go` strongSwan registration; `ipsec.go` `LabBinaries`; `test/interop-ipsec/Dockerfile.strongswan` | Declares, stages and copies `ipsec-peerform` from its standalone Go package; wiring and five generic staging tests passed | Extends the existing registered binary inventory and ignore-file rules; the checker uses `/usr/bin/ipsec-peerform`, with no separate build path |
| Dual-form receiver | `ike/dataplane/espform_linux.go` `espFormReceiver` | Reads refused bare ESP, re-presents it through `lo` | Extends: keeps the reader, the registry, the rate bound and the socket policy; replaces the `IPPROTO_RAW` injector with a TUN writer; adds the fast path in front of it |
| Watched-SPI registry | `ike/dataplane/espform.go` `espFormRegistry` | SPI to address pair and ports | Extends: every watch, forget and retarget also writes the fast path's map |
| BPF loading in the IKE dataplane | `ike/dataplane/override_guard_linux.go` | Loads a cgroup program from `asm.Instructions` through `cilium/ebpf` | Extends its pattern: the rewrite program is built the same way, in the same package |
| TCX attach | `internal/plugins/trafficusage/attach_linux.go` `tcxAttacher` | Attaches ingress and egress programs to one interface | Follows its attach and close shape; adds `link.Head()` and `TCX_NEXT`, and trafficusage itself now answers `TCX_NEXT` too, so the two coexist in either order and the clsact filters still run |
| Firewall reconcile | `internal/component/firewall/registry.go` `ApplyAll` | Applies every owner's tables to the backend under `reconcileMu`, one `Backend.Apply` per reconcile | Extends: commit observers (`RegisterCommitObserver`, `applyObserved`, `commit_observer.go`) run around `Backend.Apply`; the IKE engine's observer holds the ESP form fast path shut while the commit lands |
| nftables access | `vendor/github.com/google/nftables` `Conn`, the generational monitor | Ze's nft backend writes its tables through it | Extends for reading: tables, chains and the monitor come from it; the rule dump does not, because `GetRules` drops expressions it cannot type, so `espFormKernelReader.GetRules` decodes GETRULE itself (`espFormRuleFromMessage`) |
| Childless IKE SA retry | `ike/engine/create_child.go` `childCreateSchedule` | Paces the Child SA creation a childless initiator retries | Extends: an install, binding or build refusal of the first Child SA keeps the IKE SA and starts this schedule instead of ending the IKE SA |
| Engine doctor answer | `ike/engine/doctor_runtime.go` `engineDoctorChecks`, `answerDoctorCheck`; `sdk.Registration.DoctorChecks`, `OnDoctorCheck` | (new here) the engine declares its runtime checks in Stage 1 and answers them in its own process | Replaces the post-config `registry.DoctorCheckDef` route for the new checks, which would read the engine's globals from the operator's `ze doctor` process or a daemon whose engine runs forked |
| Lab device and sysctl grants | `internal/le/interoplab/pppdevice.go` `PPPDeviceArguments`; `apparmor_netsysctl.go` `NetSysctlWriteArguments`, `NetSysctlAppArmorProfileName` | Hands the PPP labs `/dev/ppp`; makes `/proc/sys/net` writable under a narrowed profile | Extends: `tundevice.go` `TUNDeviceArguments` hands the IPsec interop ze peer and the home-lab CPE `/dev/net/tun` the same way, and the IPsec ze peer runs under the net-sysctl profile |
| QEMU netns device nodes | `internal/le/test/qemu/netns.go` (was `netnsDevPPPScript`) | Widened `/dev/ppp` to 0666 for the dropped per-test daemon | Extends: `netnsDevNodes`, `netnsDevNodesScript` widen `/dev/ppp` and `/dev/net/tun` |
| Doctor check of a dataplane program | `ike/engine/doctor_application_override.go` | Reports the override guard's state | Extends the pattern: a check reports the fast path's attach state and the links it could not serve |
| Inbound catch-all | `ike/engine/unmatched.go` `installUnmatched` | Final DISCARD entry of the SPD | Unchanged; the fix makes it reach bare ESP again |
| Inbound IKE bypass | `ike/engine/bypass.go` `ikeBypassPolicies`, `installIKEBypass` | dir-in BYPASS for UDP to ports 500 and 4500 at `PriorityIKEBypass`, installed after the catch-all at the first apply | Depends on it, unchanged: both paths hand the kernel UDP to port 4500, and `udp_queue_rcv_one_skb` (`net/ipv4/udp.c`, v7.0) checks the inbound policy before the encapsulation hook, so without the bypass the Child SA's policy would drop the datagram before XFRM sees it. Lo's `disable_policy` hid that check before |
| Daemon capability set | `internal/core/privilege/capabilities.go` `DaemonCapabilities` | (new) one declaration of the non-root daemon's capabilities | Replaces three copies: the systemd unit (`buildUnitFile`), the test runner's netns grant (`internal/test/runner/caps.go` `daemonCapBits`) and `CheckPrivileges` derive from it, and `CAP_BPF` is added once |
| Receiver open and reader lifetime | `ike/dataplane/espform_linux.go` `openLocked`, `reopenReaderLocked`, `Close`; `espform_open_linux.go` `read`, `revive` | The same receiver owns its socket, TUN, watched SAs and acquisition holds | Extends: host-caused refusals carry exact fixes and a recorded failure; a dead reader is reopened with receiver-level backoff, and Close joins every revive worker |
| Plugin report-issue request | `pkg/plugin/sdk/report.go` `RaiseIssue`; `ike/engine/register_esp_form_reader.go` `espFormReaderStoppedSink` | An internal or forked plugin raises a problem on the daemon's report bus | Extends: each raw ESP reader stop is a registered error event, kept after a successful reopen; no process-global report call from the dataplane |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Bare ESP over IPv4 from the peer, on any non-loopback link, for an SPI whose inbound state carries the ESP-in-UDP template. No IPv6 SPI is watched until Ze has an IPv6 IKE transport (AC-8).

### Transformation Path
1. Fast path: the tc-ingress program on the receiving link reads the outer header, looks the SPI up in the watched map, checks source and destination against the target, inserts the UDP header with the template's ports (IPv4: total length, protocol and header checksum rewritten; IPv6: payload length and next header rewritten, skb marked checksum-verified), and returns `TCX_NEXT`.
2. The kernel delivers the datagram to the port-4500 socket (`UDP_ENCAP_ESPINUDP`), `xfrm4_udp_encap_rcv`/`xfrm6_udp_encap_rcv` hand it to XFRM in the state's form, XFRM decrypts, and the decapsulated packet re-enters on the receiving link, where the inbound policy check runs.
3. Complete path: bare ESP the program left alone is refused by XFRM; the raw ESP reader receives its copy (reassembled), and writes the ESP-in-UDP datagram, carrying the received outer TOS, into `ze-espform`. The kernel receives it on that TUN: its 192.0.0.8/32 address keeps loose `rp_filter` from dropping it, the inbound IKE bypass lets the UDP datagram reach the port-4500 socket, and step 2 runs with `skb->dev` the TUN, where the policy check runs.
4. Both paths keep the outer TOS octet, so XFRM's decapsulation copies a CE mark into the inner header (RFC 7296 Section 2.24).
5. A reader that stops records its read error before waking its revive worker. The worker reports the stop through the engine's plugin request and retries opening the same receiver's paths on receiver-level backoff; an open failure is recorded for the runtime doctor, and a successful open clears it. `Close` retires the reader before joining its workers and refuses later attempts to reopen it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | BPF program + hash map (TCX links), raw ESP socket, TUN fd | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `test/interop-ipsec/`; terms `writeESPForm`, `rawESPFormInjector`, `inject(`, `espFormUDPPort`, `ip4:50`. Positive control: `writeESPForm` found in `espform.go` (writer) and `espform_test.go`, `espform_run_linux_test.go` (readers of its output). The datagram it builds is read only by the Linux kernel. No peer sees it: it never leaves the host.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Re-presented ESP-in-UDP datagram | writer | `ike/dataplane/espform.go` `writeESPForm` | builds IPv4+UDP with header checksum 0, relying on IP_HDRINCL | change: computes the IPv4 header checksum (a TUN write gets no kernel fill) and carries the received outer TOS (RFC 7296 Section 2.24); no IPv6 builder, which moved with AC-8 to `plan/post-release/features/spec-ike-ipv6-transport.md` |
| Re-presented ESP-in-UDP datagram | reader | Linux `xfrm4_udp_encap_rcv` (`net/ipv4/xfrm4_input.c`) via the port-4500 socket set by `transport.EnableESPInUDP` | reads ESP after the UDP header when the first four octets are non-zero | preserve |
| Rewritten bare ESP (fast path) | writer | new `ike/dataplane` rewrite program | in-kernel header insertion | new |
| Rewritten bare ESP (fast path) | reader | Linux UDP and XFRM receive, as above | same path as a peer's own ESP-in-UDP | preserve |

### Integration Points
- `xfrmBackend.InstallSA`, `RemoveSA`, `Close` and `xfrmMobikeBackend.MigrateTunnel`, through `espFormReceiver.Watch`, `Forget`, `Close` and `reg.retarget`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | both paths hand XFRM its own form on a policy-checked device |
| No unintended coupling (components stay isolated) | Yes | all code in `ike/dataplane`; no component import |
| No duplicated functionality (extends existing, does not recreate) | Yes | the receiver, registry and socket policy are kept |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the fast path rewrites in place; the complete path keeps one read and one write buffer per worker |
| Registration over hardcoding, outbound | N-A | no command, view or family |
| Registration over hardcoding, inbound | Yes | no list learns a name; the link-type table is local to the program builder |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A decapsulated packet re-enters on the device the outer packet arrived on | `xfrm_input` keeps `skb->dev` | the check could still be skipped | AC-3 kernel test | unvalidated |
| A-2 | A TUN with `disable_policy=0` gets the inbound policy check | `ip_route_input` sets `IPSKB_NOPOLICY` only from the device's `disable_policy` | the complete path keeps the defect | AC-3 fragment rows | unvalidated |
| A-3 | `BPF_PROG_TEST_RUN` runs `bpf_skb_adjust_room` on an Ethernet frame | `bpf_prog_test_run_skb` builds the skb through `eth_type_trans` | unit proof needs a link instead | AC-6 | unvalidated |
| A-4 | IPv6 UDP receive accepts a non-zero checksum on a `CHECKSUM_UNNECESSARY` skb | `udp6_csum_init` → `skb_checksum_init_zero_check` | IPv6 fast path hands everything to the complete path | moved with AC-8 to `plan/post-release/features/spec-ike-ipv6-transport.md`; the program side is `TestESPFormRewriteProgram` | moved |
| A-5 | An address-less TUN passes loose `rp_filter` | first draft of `openESPFormTUN` | every re-presented datagram dropped | `__fib_validate_source` (`net/ipv4/fib_frontend.c`, v7.0): `no_addr = idev->ifa_list == NULL;`, then `if (no_addr) goto last_resort;`, where `if (rpf)` drops it | broken: the device now carries 192.0.0.8/32 scope host (`espFormTUNAddress`) |
| A-6 | The re-presented UDP datagram reaches the port-4500 socket on a device with `disable_policy` 0 | the `lo` path never met the check | complete path drops everything | `udp_queue_rcv_one_skb` (`net/ipv4/udp.c`, v7.0) runs `xfrm4_policy_check` before `encap_rcv` | confirmed only with the inbound IKE bypass, which production installs (`engine/bypass.go`) and the kernel tests install too |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A firewall input rule keyed on the input interface sees `ze-espform` for complete-path traffic | operator rule miss | documented in the guide; the fast path carries whole datagrams on the real link |
| R-2 | BPF load refused (no CAP_BPF, user namespace) | Warn log, doctor check | the complete path serves all bare ESP, rate-bounded; the systemd unit now grants `CAP_BPF` (`DaemonCapabilities`), and the doctor warning tells an operator with an older unit to rewrite it |
| R-3 | Ze's firewall drops input on an unknown interface | complete-path test red | read the firewall's input chain before closing |
| R-4 | The fast path carries bare ESP past a firewall rule that drops ESP | interop `esp-form-firewall-drop` red, the `drop-esp` counter still | the firewall gate (Decision, AC-13): shut by default, by every Ze commit, and while any input-path rule or an unreadable ruleset could drop ESP; the four documented windows are Known Limitations |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | bare ESP on 4500-without-NAT Child SAs is dropped (traffic in one direction stops), or the bypass remains |
| How is it reverted? | single commit revert |
| Who else touches this path? | OSPF RFC 4552 uses `ike/dataplane` (no dual-form SAs); trafficusage TCX programs share links |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| interop `esp-form-change` peer-local stimulus | → | image registration → `LabBinaries` → standalone `peerform` build → strongSwan image COPY → checker helper invocation | wiring/four subtests and five staging consumers PASS; helper/checker packages PASS; native Linux helper PASS; live same-SA switch PASS in `job-ipsec-live-form-switch-clean-de5e18d6.log`; as-used binding fault RED and post-clean PASS are recorded below; the later comment-only input is unexecuted |
| strongSwan sending bare ESP on a 4500-without-NAT Child SA | → | fast path rewrite, then XFRM on the real interface | interop `icmp-error-payload-check` without `encap = yes` |
| bare ESP on a veth for a Child SA installed through `createFirstChildSA`, under the SPD a first apply builds | → | `InstallSA` → `espFormReceiver.Watch` → fast path (`espFormFastPathInstructions`) and complete path (`espFormReceiver.run` → `espFormTUN.inject`) | `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked` (`internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go`, QEMU guest-root) |
| bare ESP with outer ECN CE on the same SA | → | both paths, then XFRM's ECN decapsulation | `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE` (same file, QEMU guest-root) |
| first `Watch`, then a link created afterwards | → | `espFormFastPath.start` → `follow` → `linkAdded` → `link.AttachTCX` | `TestESPFormFastPathAttachesEveryLink` (`espform_fastpath_integration_linux_test.go`, guest-root) |
| `ze doctor` with the fast path unavailable (`ze_test_ike_dataplane=recording-esp-form-unavailable`, a zetest-only backend) | → | the daemon's doctor answer → the engine's `OnDoctorCheck` (`answerDoctorCheck`, `doctor_runtime.go`) → `checkIPsecESPFormFastPath`, a runtime check declared in Stage 1 with `doctor-ipsec-esp-form-fast-path`, `doctor-ipsec-esp-form-link`, `doctor-ipsec-esp-form-tun` and `doctor-ipsec-esp-form-firewall` | `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`, promoted to `test/ipsec/` after its runs; `TestEngineDoctorCheckDispatch`, `TestDoctorESPFormFastPathRegistered` (`internal/component/ike/engine/doctor_esp_form_fast_path_test.go`) |
| a configuration apply that configures an IPsec peer | → | `applyConfig` → `recordESPFormTUN` (`ike/engine/apply.go`) → `espFormTUNAvailable`, a Warn line carrying `doctor-ipsec-esp-form-tun` when the device cannot be made; the doctor check reads the record | `TestRecordESPFormTUN`, `TestDoctorESPFormTUN` (same file) |
| an operator's `nft` commit adding `meta l4proto esp counter drop` to an input chain while a dual-form SA is watched | → | `espFormNFTWatch` → `judgeFirewall` → `espFormFirewallBlocks` → `writeGateLocked` (gate 0) → the program leaves bare ESP untouched | `TestRFC4301BareESPFirewallDropKeepsTheFastPathOff` (`internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go`, guest-root), `TestESPFormFastPathGateFollowsTheKernelRuleset` (`espform_fastpath_gate_linux_test.go`, guest-root), interop `esp-form-firewall-drop` |
| a Ze firewall commit (`firewall.ApplyAll`) | → | `applyObserved` → `holdESPFormFastPathForCommit` → `HoldESPFormFastPath` → `espFormFastPath.hold`, released after `Backend.Apply` | `TestApplyAllRunsCommitObserversAroundApply` (`internal/component/firewall/commit_observer_test.go`), `TestFirewallCommitHoldsESPFormFastPath` (`internal/component/ike/engine/register_firewall_observer_test.go`) |
| an initiator whose first Child SA this node cannot install, bind or build | → | `runEstablished` (`established.go`) deletes the peer's half, keeps the IKE SA, starts `childCreateSchedule` and `recordChildRefused` → `ipsec-child-install` (`doctor_child_install.go`) under `doctor-ipsec-child-install-refused` | `TestInitiatorKeepsIKESAWhenItCannotInstallTheFirstChildSA`, `TestChildInstallRefusalBacksOffToTheCapAndResetsOnSuccess` (`child_install_refusal_test.go`), `TestFirstChildSARefusalIsRecordedForTheDoctor`, `TestChildInstallCheckRegistered` (`doctor_child_install_test.go`) |
| a daemon started without a capability of `DaemonCapabilities` | → | `warnDaemonPrivileges` (`cmd/ze/hub/main.go`) and the `daemon-capabilities` runtime check (`internal/component/doctor/checks_daemon_capabilities.go`) under `doctor-daemon-capability-missing` | `TestDaemonCapabilitiesCheck` (`internal/component/doctor/checks_daemon_capabilities_test.go`) |
| traffic-usage attached on a link that also carries a clsact filter | → | the trafficusage programs answer `TCX_NEXT` (`tcxNext`), so the filter runs | `TestProgram_ReturnsTCXNext`, `TestProgram_LetsAClsactFilterRun` (`internal/plugins/trafficusage/tcx_next_integration_linux_test.go`, guest-root) |
| `ze install systemd` | → | `buildUnitFile` → `writeCapabilities` → `privilege.DaemonCapabilities` | `TestUnitFileCapabilities` (`internal/plugins/systemd/unit_test.go`), `test/ui/service-unit-gen.ci` |
| a daemon holding exactly the unit's capability set | → | the ESP form fast path, the override guard and traffic-usage load their BPF programs | `TestESPFormFastPathLoadsUnderTheUnitCapabilities`, `TestOverrideGuardLoadsUnderTheUnitCapabilities`, `TestTrafficUsageLoadsUnderTheUnitCapabilities` (guest-root, run as uid 65534) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | interop `icmp-error-payload-check`, strongSwan sending bare ESP (no `encap = yes`) | the scenario passes; its two red proofs in `plan/release/fixes/spec-rfc4301-architecture-gaps.md` are recorded. Proof (c) omits the state ICMP flag and fails consistent-error delivery with generation and SA carriage observed. Proof (d) runs clean PASS, inbound protect-policy source widened alone PASS, inbound state-selector source widened alone PASS, both widened packet-delivery RED, then clean PASS. The private IPv4 tunnel faults preserve negotiated selectors and all other state and policy fields | the in-selector error is still delivered, because the selector check passes it; each single-source fault still rejects the inconsistent error through the other selector |
| AC-2 | interop `esp-form-change` | one Child SA delivers traffic in both forms, before and after the peer changes form, and keeps its SPI | both forms delivered, because RFC 7296 Section 2.23 requires it |
| AC-3 | bare ESP on a veth between two network namespaces, on a 4500-without-NAT tunnel-mode SA installed by `createFirstChildSA`, under the production SPD (`unmatched discard` through `installUnmatched`, then the inbound IKE bypass through `installIKEBypass`, in `applyConfig`'s order): whole and fragmented datagrams inside and outside the SA selectors, then an explicit higher-priority host DISCARD inside that scope and its exact removal | inside: delivered; outside: refused by the actual SA selector before SPD lookup. Exact `(XfrmInStateMismatch, XfrmInNoPols, XfrmInPolBlock)` deltas: whole outside `(1,0,0)`, fragmented outside `(2,0,0)`, fragmented inside `(1,0,0)`. The original bare-form refusal contributes one mismatch on the complete path. The explicit DISCARD separately proves SPD consultation: whole `(0,0,1)`, fragmented `(1,0,1)`, neither delivered; after removal both valid forms deliver, whole `(0,0,0)` and fragmented `(1,0,0)`. Whole datagrams advance `Rewritten4` by exactly one; fragmented datagrams leave it unchanged, advance reassembly and are re-presented through `ze-espform`. Counter observations wait for exact terminal deltas within a fixed bound | in-selector traffic delivered in both forms because the SA carries it, and again after the temporary DISCARD is removed; the production catch-all, inbound IKE bypass and reader exemption remain present in SPD readback |
| AC-4 | `rfc/short/rfc4301.md` `RFC4301-5.2-2` | carries no `{partial}` annotation, with discrimination records for the new tests | other annotations unchanged |
| AC-5 | the KNOWN DEFECT notes | `docs/guide/ipsec.md`, `LL-XFRM-STATE-ONE-ESP-FORM` and the `espFormBypassInboundPolicy` comment describe the two policy-checked paths and the firewall cost | the one-form limitation entry stays, because the kernel limit stands |
| AC-6 | `BPF_PROG_TEST_RUN` over crafted frames | a watched SPI from its peer to its local address comes out as ESP-in-UDP with the template ports, correct lengths and a valid IPv4 header checksum, and the program returns `TCX_NEXT` | unwatched SPI, other source, other destination, a fragment, IPv4 options, IPv6 extension headers and non-ESP come out byte-identical, because the complete path or the kernel owns them |
| AC-7 | first `Watch`, a link created afterwards, last `Forget`, `Close` | the program is attached to every non-loopback link of a supported type, a new link is attached, and links, map, TUN and sockets are released at the end; an unsupported link is logged by name | `lo` and `ze-espform` stay unattached, because a packet there is local or already encapsulated |
| AC-8 | an SA asking for both forms over an IPv6 outer header, and a MOBIKE move to IPv6 endpoints | `InstallSA` refuses the SA with an operator error that names the gap and the fix, and `MigrateTunnel` refuses the move, so no state is installed that receives one form only. Serving both forms over IPv6 is a gap owed after the first release, owned by `plan/post-release/features/spec-ike-ipv6-transport.md` ("Dual-form receipt over IPv6"): Ze has no IPv6 IKE socket (`transport/udp.go` listens on `udp4`, `engine/probe.go` refuses an IPv6 peer), so no such SA is negotiated today | IPv4 dual-form SAs install and are served; the IPv6 program variant stays, tested by `TestESPFormRewriteProgram`, and nothing writes its map yet |
| AC-9 | the complete path | writes into `ze-espform` (`disable_policy=0`, IPv4 `rp_filter=2`, address 192.0.0.8/32 scope host) and keeps the 5000/s bound; nothing is sent to `lo` | the rate bound, because the complete path runs in userspace |
| AC-10 | the fast path cannot load or a link refuses the attach | `ze doctor` reports it with the cause and the fix under `doctor-ipsec-esp-form-fast-path` or `doctor-ipsec-esp-form-link` (`test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`, promoted to `test/ipsec/` after its runs) | the complete path keeps receiving, so the SA still works |
| AC-11 | bare ESP on the AC-3 veth whose outer IPv4 header carries ECN CE, around an inner packet inside the selectors marked ECN-capable (0b10), whole and fragmented | delivered with the inner TOS octet CE (0x03), read from the `IP_TOS` control message, on both paths: the whole datagram advances `Rewritten4` by one, the fragmented one leaves it and is re-presented with the received outer TOS copied into the header the complete path builds (RFC 7296 Section 2.24, `RFC7296-2.24-2`; `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE`) | DSCP bits pass unchanged on both paths, because the fast path keeps the TOS octet and the complete path copies it whole |
| AC-12 | the systemd unit Ze writes, and a daemon holding exactly its capability set | the unit grants `CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE CAP_BPF`, ambient and bounding, from `privilege.DaemonCapabilities`; the runner's netns grant and `CheckPrivileges` derive from the same declaration; the ESP form fast path, the override guard and traffic-usage each load their BPF programs as uid 65534 holding exactly that set | no `CAP_PERFMON` and no `CAP_SYS_ADMIN`, because each program passes the verifier's unprivileged rules |
| AC-13 | a dual-form SA served by the fast path, then an nftables table committed from outside Ze whose input chain drops ESP (`meta l4proto esp counter drop`), then that table deleted; and interop `esp-form-firewall-drop`, whose `ze.conf` drops `protocol esp` in the `drop-esp` term | the firewall decides on the bare ESP: once the watch sees the commit the gate is 0 and `FirewallBlocked` names the table and the chain; a whole bare datagram inside the selectors leaves `Rewritten4` unchanged, the rule's counter moves by exactly one and nothing is delivered; once the table is gone the gate opens and the same datagram moves `Rewritten4` by one and is delivered (`TestRFC4301BareESPFirewallDropKeepsTheFastPathOff`). In the interop scenario `show doctor` names `doctor-ipsec-esp-form-firewall` with `ze_espdrop` and `drop-esp`, strongSwan loses every echo request, the `drop-esp` counter rises by at least the echo requests, and Ze's inbound SA and `XfrmInStateMismatch` stay still | IKE on UDP 500 and 4500 passes, because the term matches ESP only; bare ESP a firewall accepts is served by the complete path at its bound, because the gate stays shut while any input-path rule could drop ESP; a Ze firewall commit holds the gate shut while it lands (`TestFirewallCommitHoldsESPFormFastPath`) |
| AC-14 | an apply that configures a peer on a host where `ze-espform` cannot be made (no `/dev/net/tun`, read-only `/proc/sys`, no `CAP_NET_ADMIN`); and the fast path's gate shut by the firewall or by a ruleset Ze cannot read or watch | the apply records the outcome and logs one Warn line carrying `doctor-ipsec-esp-form-tun` (`recordESPFormTUN`); `ze doctor` answers, from the engine's process, an error under `doctor-ipsec-esp-form-tun` naming the cause and its fix, and a warning under `doctor-ipsec-esp-form-firewall` naming the first blocking site, or the read or watch failure, and the complete path's bound (`TestRecordESPFormTUN`, `TestDoctorESPFormTUN`, `TestDoctorESPFormFastPath`) | the doctor check only reads the record and never opens the device, because that open may create the node; no TUN finding while no peer is configured, because no Child SA can need the device |
| AC-15 | an initiator whose first Child SA this node cannot install (`errChildInstall`), bind (`errChildBinding`) or build (`errChildProposal`, `errChildKeying`, `errChildAddress`, or no recorded peer SPI) | the IKE SA is kept as a childless one, the peer's half is deleted, and `childCreateSchedule` asks again `childCreateFirst` (30 s) later, doubling after each refusal up to `childCreateMax` (300 s), one Warn line per attempt with its `retry-after` and its fix; the first attempt after the cause is removed installs the Child SA with no restart, and success resets the schedule (`TestInitiatorKeepsIKESAWhenItCannotInstallTheFirstChildSA`, `TestInitiatorKeepsIKESAWhenItCannotBuildTheFirstChildSA`, `TestChildInstallRefusalBacksOffToTheCapAndResetsOnSuccess`) | an IKE SA with no SK_* keys (`errChildNoIKEKeys`) is still ended, because it can protect no message; a responder does not retry, because only the initiator asks again (RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be deleted.") |
| AC-16 | a peer whose last Child SA this node refused, until a Child SA installs | `ze doctor` answers one error per such peer, in name order, under `doctor-ipsec-child-install-refused` (`ipsec-child-install`, `doctor_child_install.go`), with the time, the cause and the fix; the refusal's Warn line carries the same code; an install clears it (`TestRecordChildRefusedClearsOnInstall`, `TestChildInstallFindings`, `TestChildCreateRefusalIsRecordedForTheDoctor`, `TestFirstChildSARefusalIsRecordedForTheDoctor`) | a peer the configuration removed has no finding, because its session and its refusal are gone |
| AC-17 | a daemon that runs without a capability of `privilege.DaemonCapabilities` | the daemon logs one Warn line at start carrying `doctor-daemon-capability-missing`, the missing capabilities and what each serves, and the fix (`warnDaemonPrivileges`); the `daemon-capabilities` runtime check answers the same warning in `ze doctor` (`TestDaemonCapabilitiesCheck`) | a daemon running as root, or holding every capability of the set, reports nothing; the check runs in the runtime phase only, because the operator's own `ze doctor` process would measure itself |
| AC-18 | traffic-usage attached by TCX on a link that also carries a TCX program behind it and a clsact filter | each trafficusage program answers `TCX_NEXT`, so the program behind it and the clsact filter still run, on ingress and egress (`TestProgram_ReturnsTCXNext`, `TestProgram_LetsAClsactFilterRun`); the L4 port is copied with `bpf_skb_load_bytes` at a scalar offset, so the programs load without `CAP_PERFMON` (AC-12), IP options move the port read, a truncated or out-of-packet L4 header keys port 0, and a frame that ends inside the IPv4 header passes uncounted (`TestProgram_IPOptionsMoveThePort`, `TestProgram_TruncatedL4HeaderKeysPortZero`, `TestProgram_L4OffsetBeyondThePacketKeysPortZero`, `TestProgram_IPv4HeaderEndingTheFrameCounts`, `TestProgram_TruncatedPass`, `program_test.go`) | every packet passes, because the programs only count and never drop |
| AC-19 | A read failure races last Forget or reader replacement, including failure admission before retirement and intentional retirement before failure admission | failure admission and publication are serialized with retirement; every recorded failure is reported exactly once even when retirement precedes revive, and a retired worker neither reopens its life nor resets the replacement's backoff | intentional retirement that wins before failure admission records no failure and emits no reader-stop event, because the reader's close was requested |
| AC-20 | An external IKE process starts through `ze plugin ike` and its loaded backend supplies the runtime doctor finding | the entrypoint reaches the registered engine over the plugin connection, and the daemon's doctor answer carries that process's cause, fix and severity; a metadata `--features` invocation also succeeds without a hub connection | declaration queries still return registration without starting an engine; the internal engine retains its runtime doctor path |
| AC-21 | An ESP-group-only reload reaches an attached, established childless IKE SA before its owner is published | the existing channel handoff waits at most `espReloadWait` for the owner to take the request, and owner adoption keeps the session, both IKE SPIs and the responder's inactive creation schedule | a session with neither an owner nor an attached SA refuses immediately; an installed Child SA, pending responder handshake or own Child SA creation in flight still causes a restart |
| AC-22 | The exact-state-scope probe sends a capability attribute to test the read-only receipt | the request serializes its fixed header before the capability attribute, and the kernel rejects that correctly formed write with `EINVAL`; every positive capability assertion stays unchanged | the preceding malformed request's result supports no kernel-behavior claim; passing independent packet and fragment probes retain their results |
| AC-23 | The ESP-form interop checker receives missing, populated valid, or populated same-form Ze XFRM state evidence | empty or whitespace-only snapshots return a missing-state error; a populated valid-state control succeeds after the existing template, traffic, counter, direction and SPI checks; a populated same-form control reaches its distinct form-agreement diagnostic | a populated snapshot still proceeds through the existing template, traffic, direction and SPI checks; AC-2's full form-switch and counter repair remain owed |
| AC-24 | A non-root process holds exactly `privilege.DaemonCapabilities` (`CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_NET_BIND_SERVICE`, `CAP_BPF`, without `CAP_SYS_ADMIN`) and captures its current network namespace; additional cases use an unauthorized different target, valid descriptor zero, and invalid or closed handles | the actual nft reader and watch read/watch the captured current namespace, and the fast path starts, follows links and stops successfully. Namespace capture and socket creation stay pinned together. A valid descriptor zero keeps its explicit target meaning. A different target without entry authority and every invalid or closed handle refuse without opening a socket in a fallback namespace. Worker lifetime and teardown remain bounded and sound | the current namespace works without `CAP_SYS_ADMIN`; an authorized different target may be entered through the existing authority check, while a target that cannot be entered remains refused |
| AC-25 | The unreadable-firewall finding receives a permission cause, including a refused `setns` operation despite an adequate daemon capability grant | the finding retains the original cause, `doctor-ipsec-esp-form-firewall` code, warning severity, restart command and log guidance, and directs the operator to the measured `daemon-capabilities` finding without declaring `CAP_NET_ADMIN` missing from the permission error alone | a measured missing capability may still be reported by its own check; a permission error remains visible with its actual cause and recovery guidance |
| AC-26 | IPsec peers are configured on a backend that declares complete state-scope checking, before any Child SA attempt | the registered engine runtime doctor check freshly reads the same scope capability gate used by installation, diagnoses missing, partial or unreadable support with its original cause and canonical build commands, and clears after a complete receipt is read | no configured peers and undeclared backends stay silent; complete support is healthy without installing state or changing configuration |
| AC-27 | The approved scoped lint repairs cover two new probe style findings and twelve existing IPsec mechanical findings, including lowercase sentinel cause wording | the scoped style defects are corrected with identical assertions, packet bytes, tags, claims and valid/adversarial outcomes. Public refusal identity and wrapped cause remain verified, with the same problem, explanation, fix and severity. Post-style formatting, builds, host/Linux checks and lint run against the corrected source | unrelated shared diagnostic findings stay with their owner; earlier pre-style passes remain historical evidence and do not count as post-style verification |
| AC-28 | A namespace-changing test caller stays on its locked OS thread and reads XFRM counters through the shared `sadSelStat` producer | the reader opens `/proc/thread-self/net/xfrm_stat`, with matching path diagnostics/comments and the same parser/error conditions; observed packet delivery and exact rejection-counter assertions remain unchanged. The independently approved correction is applied and its affected counter/packet population is rebuilt and rerun | every existing packet input, expected outcome, RFC tag and claim remains; the separate constrained-fragment contract, mapped ICMP return defect and unrelated changes keep their existing ownership and proof obligations |
| AC-29 | Raw IPv4 and IPv6 sockets send whole and fragmented datagrams under captured OUT socket-policy and route authority | ordinary packet regressions distinguish matching policy, selector miss and explicit absence; marks, bound-device authority and distinct policy generations cannot borrow authority across a fragment queue. Equal absent authority may combine across sockets, and queued policy authority stays validly owned across replacement or socket close. Whole and same-authority controls pass in both arrival orders | the existing captured-authority architecture contract, packet deadlines and assertions stay intact; this preservation regression makes no new RFC claim and establishes no C defect before an attributable runtime failure |

### Selected full-kernel route checkpoint, 2026-11-16

The owner selected all three capabilities: exact per-SA permissions,
fragment-context isolation and fragment port-rule enforcement. AC-4 stays full;
the 79 renewals remain owed. Current source is authored and frozen, with
independent reviews CLEAR: directional immutable engine scopes are validated
before state publication, the K3 transport correction is active in
`0005-xfrm-fragment-guard.patch`, and runtime config/require enable the guard.
The historical proposal is retained only for native commit removal.

Observed preparation evidence: complete-scope build PASS; complete-receipt
RED on old c358/image45cb at features 0x7 versus required 0xf; child-scope
writer RED then GREEN 1.501s; full affected host packages PASS (engine 131.808s,
dataplane 3.453s) before the later doctor addition. The corrected transport
packet test went RED on four direct/Destination Options initial/tail cases;
whole, atomic, ciphertext and nested controls passed. AC-26's doctor regression
then went RED/GREEN 1.515s, with independent review CLEAR. Later pre-style host
packages also passed (`70092bc3`, engine 119.345s, dataplane 2.334s, diagnostic
1.283s). Post-style verification is separately owed below.

Main's cold native kernel/matching-image job completed with exit 0 in
`job-ipsec-k123-kernel-guest-f67febab.log`. New kernel SHA256 is
`9740ef22ce9ad40cde536908394d33a4d71dede15955113c15c839fe11f62cb9`,
matching image `47a4c8b74c98c558`. Its complete 0xf receipt test passed in
0.01s, with no skips and all zero exits. The unchanged qualifying-RED transport
binary then passed the direct transport fragment boundary in 4.92s and nested
control in 0.00s (`job-ipsec-transport-boundary-new-kernel-fcb2493d.log`),
including whole/atomic/ciphertext controls, no skips and stopped VM.
The first 9740/image47a4 profile then passed the eleven-test scope population
(`466ea81f`), including all eight ANY rows, K1 readback, multipair/rekey/bounds
and clear-forward controls, without skips. K1's private port cut produced four
own wrong-SA token failures with eight controls, independently CLEAR (`2459e7a5`).
That is a whole-packet discriminator: the explicit NONINITIAL flag stays false,
but widening ports also broadens effective fragment permission. IPv6 raw-port
acceptance passed all forms/phases in 0.42s (`0a6f6cb6`); its old-kernel RED
(`26098b15`) stopped at permitted WHOLE send EPERM, so it is not a separate
RED for each fragment. Ten PMTU tests passed without skips (`a8a7fb27`).

Actual configured/running daemon doctor probes with zero SAs/empty SAD showed
the exact old 0x7/0xf cause/fix and no new scope code (`d20bb5e4` complete and
`0c1dd1de` new), all exits zero and stopped daemons/VMs. The earlier non-complete
old attempt failed its BusyBox SAD read and gives no complete-probe credit.
Three doctor CIs passed without skips: internal fast 2.9s, external fast 3.4s
and internal removal 4.4s (`891d23f1`, `7ca262e0`, `0a49a03c`). Fourth external
removal passed in 4.0s (`job-ipsec-doctor-removal-external-k123-7fad9c53.log`):
one selected case, zero skipped phases, all exits zero, interrupted false and
VM stopped. That job completed the cold 88c98f02 profile and matching own-system
image after foreign runtime CFS requirements changed. That built kernel SHA256 is
`89a57fd6746aa444a1a453feab7c0eece000075f00276fb545cb62312038beb2`, at
`/Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-88c98f02-5d7f6651/vmlinuz`;
matching image is `a3933863bf7976cd`. Its direct complete-receipt test also passed
in 0.01s (`job-ipsec-current-profile-complete-caps-d4265cb4.log`), without skips,
all exits zero, interrupted false and VM stopped. Current runtime.config SHA256
is `2cf19c2ccdad059fd27d4e1bb9b8a42725f7c285865de13d43db0a9d032172d2`;
runtime.require is `b56687bcd5990d39d3a23dd86b48a3b52fb5cea7051e74470a80bd0b6d83e136`.

These are bounded pre-style results, not completion of the three-point goal.
The private fault kernel remains unbuilt/unrun. Automatic review rejected the
follow-up composite scope-key/transport-output preparation for possible
cybersecurity risk. Read-only inspection found partial private edits in the
existing `scratch/k2-fragment-proof/inputs/patches/`: 0004 clears the scope key;
0005 bypasses the transport output helper for both families. Do not build or use
these partial inputs, or retry, rephrase or reassign the rejected preparation.
The earlier pure K2 input is not an alternate retry. Keep this blocked artifact
in its owning state only until a safety decision or approved removal, with
ownership/reference/process checks before removal. That rejected preparation
changed no production patches; the separate normal repair below is authorized.
AC-27's bounded nine-path cleanup landed in `a62375a383`, native script
session 32163 exit 0, with 22 insertions and 21 deletions. The native commit
carried the ordinary `engine.espPolNoPols` weakening approval; the two other
RFC approval rows remain unspent. Main verified the commit and clean targeted
status. Post-style host packages passed in
`job-ipsec-post-style-host-packages-70092bc3.log`, inner `job-unit-pkg-0d6c7694.log`:
engine 123.593s, dataplane 2.521s and diagnostic 1.291s. Main observed successful
Linux integration builds for engine `b93836ee` and dataplane `668c4a79`; their
retained build logs are silent and do not independently establish an exit code.
The scoped lint left only the separately owned diagnostic formatting finding;
that owner's targeted retry passed on host and Linux, as read back in the
existing receipt. Capability and compile-out stages had no findings. This
completes the bounded AC-27 source phase, with no wider lint-green or spec
closure claim. Other outstanding checks remain owed.

Post-style Linux dataplane units also passed all 31 top-level tests without skips
(`job-ipsec-post-style-dataplane-linux-units-3641106f.log`): exact state-scope
capability/parameter/wire handling, SPD recording, ten forwarding-default cases
and the full receipt. Binary `bdaf3541` used 89a/a393; check/command/lab exits
were zero, interrupted false and VM stopped. This is bounded Go/unit evidence,
not runtime acceptance of the new C continuation repair.

The reached helper population (`job-ipsec-style-reached-linux-population-a644e963.log`)
ran 16 tests: ten PASS, six FAIL. Five failed zero-counter waits, one cleartext
and four forwarded-ICMP cases, used the shared process-namespace counter reader;
they do not establish kernel rejection failure. Exact observer patch `fe6c7813`
was independently CLEAR for its 16 tagged consumers, with native approvals in
the existing e729 ledger. The author confirmed application, and source hash
`711a26fd` was read back. A later authorized comment-only header correction
produced current source `cfeea450`, with functions/tags/assertions unchanged.
Main observed formatter `34367ae3` and full-manifest Linux integration/zetest
compile `61adff27` exit 0; the source remains `cfeea450`. Fresh binary
`engine-counter.test` (`880a1f6d`) passed all five former zero-counter failures
in `job-ipsec-thread-counter-five-linux-39fb9b9a.log`: cleartext 5.85s and four
forwarded-ICMP cases 0.18/0.20/0.20/0.24s, without skips, all three guest exits
zero and VM stopped on historical 89a/a393. The 16 tagged consumers then had
14 PASS/two FAIL (`job-ipsec-thread-counter-tagged-linux-f6906673.log`): the
known compliant-fragment delivery case failed in 3.30s; ordered inbound admit
passed its packet checks, then failed cleanup at line 53 with policy-delete
ENOENT in 1.01s. Noncompliant fragments passed with exact counter deltas
(state mismatch, no policy, policy block) = (1,0,0). No skips, check/lab zero,
command one and stopped VM; the already-closed native handle 74728 supplies no
recovered outer exit. The CI owner's full-selector cleanup fix, independently
approved for `orderedInstall`, is now committed with the counter correction in
`714970f5b0` (`test: observe IPsec counters in their packet namespace`), exactly
two source files at `cfeea450`/`8b36277e`, both clean in targeted status. Main's
native script exited 0 and automatically carried five RFC/helper trailers;
remaining approvals are unconsumed. This session has not pushed. External
CI-main receipt `job-ci-ordered-cleanup-linux-52ac831c.log` passes both ordered
tests, 11.26/0.51s, with every packet vector intact, no skips, all three exits
zero and stopped 89a/a393. This is CI-main evidence, not our runtime execution.
AC-28 records this bounded checkpoint; the known compliant-fragment contract
remains red and safety-blocked. Binary `880a1f6d` predates the cleanup correction,
so it is a qualified comparison, not a fresh-current-source binary. Its accepted
counter-phase commit has landed; main owes ownership/reference/process-checked
cleanup of superseded binaries and minimum-evidence retention.

Main reports the exact peer-form source-classification row added and reviewed
by author and main, with the existing TUN and reqid rows verified. The CI root
owns coordinated registry publication; its actual commit and receipts are still
pending. This records source classification only, with no runtime change.

The sixth failure is separate: mapped ICMP return had protected=0/clear=0.
Isolation reproduced it in 12.05s (`job-ipsec-mapped-return-isolated-e9eb39b7.log`)
on 89a/a393, without skips, check/lab exits zero, command exit one, interrupted
false and VM stopped. It matches the existing architecture spec's phase-3
`LL-XFRM-ICMP-TRANSIT` obligation; style changes do not establish a new C defect.
The exact wire assertion and locally generated positive controls stay intact.

The post-style seven-test matrix (`job-ipsec-current-fragment-matrix-e4293794.log`)
on 89a/88c with image a393 and engine binary `2285ba7c` had five PASS and two FAIL:
IPv4 ANY contexts, IPv6 raw port policies, nested transport/tunnel and both
transport-output families passed. IPv6 ANY lost four permitted same-SA
Destination Options rows; the transport receive test lost its initial whole
control. Isolated receive passed in 5.16s (`b6c76ed5`). Isolated IPv6 ANY still
failed those four rows in 8.68s (`324011f1`), with all eight base rows, exact
authenticated counts and whole controls correct. The same-process pair repeated
ANY failure in 10.19s but passed receive in 2.04s (`a015d255`). Each guest stopped
without skips or interruption; only the expected failing command exit was 1,
with check/lab exits zero. The batch receive loss remains unlocalized; neither
later pass proves a fix or that the ANY test caused it.

The normal IPv6 Options failure is a production continuation defect in
`xfrm6_fragment_prerouting`: gathering leaves the transport cursor beyond the
removed Fragment header while delivery still names a preceding extension.
Active 0005 now carries the reviewed normal repair, SHA256
`80cf2492bac5b88c302a915fc0fa4b07986add231ad777dcc222202827b36a00`.
It restores the cursor from the retained first fragment's headers/control block
and preserves processed Hop-by-Hop state; LOCAL_OUT is unchanged. Independent
C source review is CLEAR. The additive first-fragment Hop-by-Hop regression also
has independent CLEAR review. Its built-baseline run passed in 3.30s
(`job-ipsec-hop-options-old-normal-79f077b6.log`): all six both/initial-only/tail-only
Hop-by-Hop cases in both arrival orders delivered, with whole-chain/following
controls and exact SA counts. No skips; check/command/lab exits zero,
interrupted false and VM stopped on 89a/a393, engine binary `127cef6f`.
Main observed formatting `09ed7233` and test build `ecdb71b4` PASS. This is a
preservation PASS, not a mutation proof or repaired-C acceptance. The normal C
repair's native build job 72568 ended with exit 1
(`job-ipsec-normal-cursor-kernel-6fc0be2f.log`). The worker compiled the kernel,
installed modules and printed `done: /out/vmlinuz`, but the final host
`enforceKernelRequirements` rejected missing built-in `CONFIG_QFMT_V2`.
Shared base config/require and `kernelreq.go` changed at 17:26:56 during the
17:04:51 build, in commit `faa6297d78`; the worker checks live requirements
before make and the host rereads them after build. Frozen base fingerprints
`2bef4e61`/`57692fbb` became `be2894ce`/`6990de48`; runtime `2cf19c2c`/`b56687bc`
and active 0005 `80cf2492` stayed unchanged. The new quota/QFMT dependency is
valid and its requirements must not be weakened. Native cleanup removed the
rejected `tmp/kernel/runtime-build-2183217670`; target cache 265a does not exist.
That attempt produced no recoverable/published repaired kernel or matching image;
the worker's ready message was not host acceptance. The original Options run
remains natural production RED on historical pre-repair 89a/88c.

The two-job retry `9c693ee4`/session 82844 then ended 0 and published accepted
cache 6a87, kernel SHA256
`1b3b839f751e6ab9f69a896d15847c07c9f8b7e2a24f07f934cf7fb029076ab3`.
Read-back config has QUOTA, QFMT_V2 and XFRM_FRAGMENT_GUARD built in; modules
are `7.3.0-ze` and kernel.version identifies Linux 7.3 runtime/arm64/Docker.
First matching-image boot (`job-ipsec-corrected-kernel-first-boot-c6c943c2.log`)
on image `913df5db17ebb20a` passed complete receipt 0.01s, original IPv6 ANY
matrix 6.68s (all 16 rows correct, including four previously failing Options
positives and every mixed-SA rejection), and six Hop-by-Hop cases 0.15s with
whole/following controls and SA counts. The subsequent eight-test matrix
(`job-ipsec-corrected-eight-packet-matrix-4e681aeb.log`) passed all eight tests
using engine `880a1f6d` and dataplane `bdaf3541`. Both guest receipts have no
skips, zero check/command/lab exits, interrupted false and stopped VMs on that
accepted pair. This is cursor-repair acceptance, not full three-point conformance
or AC-4 closure. The original seven-test population subsequently passed five
times in one process: 35 top-level PASS results, with no FAIL or SKIP
(`job-ipsec-corrected-rx-stability-c25ff001.log`, main session 13208 exit 0).
It used the same accepted pair, count 5, shuffle off, the original eight-minute
test timeout and unchanged packet deadlines/assertions; check/command/lab
exits were zero, interrupted false and the VM stopped. The historical RX
failure's cause remains unlocalized; these passes establish stability evidence,
not a diagnosis or a claim that the cursor repair fixed that earlier loss.
Canonical documentation was updated as partial narrative only. The 79 original
renewals remain, and freshness of the four earlier renewals must be checked
again after the writer change. Full bits 0–3 admission remains unchanged.

AC-29 source snapshot `71d4` passed formatter session 6402 exit 0
(`job-ipsec-raw-authority-format-e9d970e8.log`) without a hash change and Linux
build session 80197 exit 0 (`job-ipsec-raw-authority-build-af85e020.log`; silent
log, terminal status supplied by main). Static Linux/arm64 binary SHA256 is
`b73f7299f689254045b7fb67fd32519e5bd29c5e9f2efa0ce738e562348a5145`.
Guest session 85260 exited 1 (`job-ipsec-raw-authority-packets-be4fca5b.log`):
IPv4 PolicyAuthority's first permitted whole send and RouteAuthority's first
admitted-mark whole send returned `EPERM` under fallback BLOCK. All three IPv6
subtests and IPv4 PolicyLifetime passed. No SKIP; check 0/command 1/lab 0,
interrupted false, VM stopped on kernel `1b3b839f`/image `913df5db17ebb20a`.

Four diagnostic lenses cleared fixture, initialization and encoding. Main read
canonical Linux 7.3 `ip_route_output_key` and `ip_route_output_flow`: the former
invokes the latter with a null socket and performs XFRM lookup. Both deferred
IPv4 ordinary-route stages therefore used the wrong helper. The normal source
repair changes only those two calls to `__ip_route_output_key`, in
`raw_sendmsg` and `xfrm_fragment_route`; final captured-authority lookup and all
guards remain unchanged. Active 0005 is now SHA256
`235e022ad1b9b1725db8e62852e955a7a3109dc35c2dc6b3492ef7d2cbf43854`, with
independent review CLEAR. Resolver session 72041 passed (`a5417483`) and names
candidate `runtime-docker-d87a6908-5d7f6651`; its vmlinuz was absent before
launch. All eight roots acknowledged input and Colima compiler holds before
normal two-job build session 29078 started, label `ipsec-ipv4-raw-route-kernel`,
PID/PGID 23658 (`job-ipsec-ipv4-raw-route-kernel-9c693ee4.log`). It completed
exit 0 and published kernel SHA256
`d2eb6a14cd07c913deb1a0e9f7ff667230982b64c597ebf8fbee8bc3f80d8f65`.
The repeated command key does not reuse the historical cursor build's result.
Firstboot session 3966 exited 0 (`job-ipsec-ipv4-route-first-boot-631e2522.log`),
building and booting matching image `43a1d1b2ecccc59c`, module `7.3.0-ze`.
Complete StateScope receipt passed 0.01s; unchanged `b73f` PolicyAuthority/ipv4
passed 1.24s and RouteAuthority/ipv4 passed 0.94s, including the formerly
failing whole controls. No SKIP; check/command/lab 0, interrupted false and VM
stopped. This accepts the normal two-call route repair for those failed cases.
Main sent all eight roots RELEASE of input and Colima compiler holds after
actual readback. Full three raw-authority tests, existing eight-test matrix
and the new SA-lifetime regression remain unrun on this pair. Older eight-test
and 35-run passes retain their recorded source/kernel identities; neither
targeted acceptance nor those older receipts close AC-29 or full AC-4.

The preparer's metadata-only stop is not a new owner policy: main may separately
run the existing 13 ordinary host reverts and 11 existing host mutants after its
phase boundary, each with a clean selected pass and attributable red. The two
actual safety rejections remain in force. All 79 renewals are still unexecuted,
the four historical records' current freshness is UNKNOWN, and the 55 guest
commands still name actual d87a6908 and remain UNEXECUTED, despite the pair's
new targeted runtime acceptance.
The 6a87 observations retain their historical kernel/source identities.
The owner's question about the two rejected tasks remains PENDING, not approval.
Native capability research separately found no C-discrimination route: the
existing P3/post-release kernel-route design needs REWORK and owner design
admission before tooling implementation or native C-proof credit. Research
is not implementation, an escape or dependency proof of the C primitive.
The existing fragment-context fixture correction is an
unresolved testing blocker: automated review stopped concrete patch preparation,
with no patch, approval or test change. That rejected task is not retried.
Completed source preparation and bounded passes do not close the full runtime proof.

The earlier space blocker was resolved when all roots acknowledged a quiet period
and main's supported coordinated cleaner completed with exit 0, freeing 67.4G of
checkout Go cache and 0.4G of lint cache, with 80.1G free
(`scratch/ipsec-coordinated-cache-clean.log`). Valid patched GOROOTs were kept;
the built pre-repair 88c kernel and a393 image remain preserved. Main released
all roots after completion. The private fault-preparation safety blocker remains unchanged;
no fault kernel has built or executed. The existing state owns this minimum
cleaner receipt through the related phase commit, then reference/ownership cleanup.
That 80.1G was the first cleanup snapshot. The second supported cleanup log
(`scratch/ipsec-coordinated-cache-clean-second.log`) reports 55.0G of checkout
cache and 0.3G of lint cache freed, with 74.9G free at cleanup. Its execution
72928 was already closed when recovered; no exit code was recovered or inferred.
Main sent RELEASE to all eight roots. The current-floor Darwin arm64/CGO-disabled
host driver built with code 0 (`job-ipsec-current-floor-host-driver-ae60c218.log`).
A temporary unrelated BGP compilation blocker was corrected by its owner;
main's fresh admission report then passed, with one of four slots occupied,
no waiters, 25.6GiB available and 52.1GiB disk free. Normal kernel session 81417
then failed with exit 1 (`job-ipsec-current-floor-cursor-kernel-799a7dd1.log`):
gcc's `cc1` was killed compiling Intel `ice_common.o`, and make exited 2. Guard
and quota objects compiled earlier, but no completed/published 6a87 kernel or
image exists. Main's fresh Docker info reports only 2 CPUs and 2,054,840,320
bytes of memory (about 1.9GiB). The Mac admission figures did not describe that
builder budget; four compile jobs were too aggressive. Builder-VM dmesg now
confirms OOM killed `cc1` in this build's owned container. The next normal retry must
use two jobs, which previously completed worker compilation. That hold was
released; root job 82844/PID 75336 completed with exit 0 as native key `9c693ee4`.
`job-ipsec-two-job-cursor-kernel-9c693ee4.log` ends with runtime kernel ready
at cache 6a87; the accepted kernel/image receipts are recorded above.
All eight roots received RELEASE of input and Colima compiler holds. Both exact old owned
containers are absent from main's successful filtered listing. Main confirmed
`tmp/kernel/runtime-build-2858380835` was empty and removed it with `rmdir`, exit 0.
The accepted 6a87 build's patch/base fingerprints were
`80cf2492`/`be2894ce`/`6990de48`; the later reviewed raw-route repair is recorded
above and is not yet built.
The unique writer of the intervening shared-base change is still unconfirmed.

### AC-24 and AC-25 current-namespace repair

The pre-repair clean firewall baseline failed after 1m35s while waiting for the doctor's
table/term finding (`job-ipsec-firewall-clean-current-c729df7b.log`). Both the
nft watch and link follower unnecessarily call `setns` for the already-current
namespace. Independent source/fixture investigation confirmed that the actual
grant and checker are correct. This failure gives no packet or fault-proof
credit. The corrected namespace build passed in
`job-ipsec-namespace-repair-linux-build-f27bbf5c.log`. All eight named parent
tests then passed without skips in `job-ipsec-namespace-unit-caps-green-59710cd6.log`:
actual current-namespace reader/watch, valid descriptor zero, different targets
at ordinary/zero descriptors, closed-high/closed-low handles and real TCX link
follow/delete/stop. Kernel `c3583fe5` and image `45cb736f9a13fd62`, all zero
check/command/lab exits, no interruption and stopped VM. Independent namespace
identity and lifecycle reviews both returned CLEAR; main's formatter passed.
The existing affected guest population matching
`^TestESPForm(FastPath|NFTWatch|Firewall|LinkSelection)` also passed without
skips (`job-ipsec-namespace-fastpath-firewall-population-cedac728.log`), on the
same kernel/image pair with all zero exits and stopped VM. Both full affected
host packages passed in `job-ipsec-namespace-repair-packages-a5a9d6ed.log`,
native `job-unit-pkg-f23601e1.log`: dataplane 2.350s, engine 114.757s.
Scoped two-package lint is RED, exit 1
(`job-ipsec-namespace-repair-scoped-lint-f39066c8.log`): Darwin has zero
findings, Linux integration 11, capability 4 and compile-out 3. All findings
are outside the current namespace/permission repair files. The known-red list
below preserves them. The current doc check exited 1
(`job-ipsec-namespace-repair-doc-check-f119ecea.log`) with 389 existing
catalog/wiki/gh-pages drift issues. All source references are valid across
5093 paths and 856 packages; all 3080 digest anchors across 23 digests resolve.
The stale discovery-index warning was repaired by native index update
`job-ipsec-namespace-doc-discovery-update-ae12aa2a.log`, which passed and changed
ignored generated files only. Source checks are complete with these bounded
known reds; no overall doc/lint-green claim. Lifecycle review still requires
post-fix clean `esp-form-change` and `icmp-error-payload-check` runs;
the unchanged seven-run ICMP discrimination sequence keeps its existing credit.
The namespace/permission source phase landed in `927f93fafe`. Rebuilt-current-source
firewall clean/fault/clean is now observed and independently CLEAR, as recorded
below. FRR passed its named scenario in 5s, post-fix form-switch clean in 8s,
and post-fix ICMP clean in 10s. Final independent lifecycle acceptance is CLEAR
for all three actual recordings. No live run remains owed for this namespace phase.

`TestDoctorESPFormFirewallPermissionCauseRetainsUncertainty` produced its own
RED in `job-ipsec-doctor-permission-cause-red-1535be55.log`, native
`job-unit-pkg-0751f303.log`: the current message wrongly infers missing
`CAP_NET_ADMIN` from refused `setns`. After the wording fix, that regression,
`TestDoctorESPFormFirewallReading` and `TestDoctorESPFormFastPath` passed in
`job-ipsec-doctor-permission-green-a5623ea2.log`, native
`job-unit-pkg-f8eb268c.log` (engine 1.496s). These are repairs within this spec,
with AC-4 and the 79 renewals unchanged. The repaired-source live firewall
sequence below is now observed.

Known scoped lint reds outside this repair: dataplane
`kernel_scope_params_linux_test.go` import grouping,
`xfrm_forward_default_linux.go` ineffassign/ST1005 and `recording_zetest.go`
modernize; engine `fragment_boundary_integration_linux_test.go` gocritic/noctx,
`mh_selector_boundary_integration_linux_test.go`,
`rfc4301_forward_icmp_integration_linux_test.go` and
`rfc4301_icmp_return_integration_linux_test.go` gocritic,
`doctor_forward_default_linux_test.go` modernize, and
`rfc4301_espform_policy_integration_linux_test.go` unused. No current repair-file
finding or overall lint-green result is claimed. These unrelated files stay
with their existing owners.

### AC-13 repaired-source firewall receipt

The normal rebuilt-source clean passed in 17s
(`job-ipsec-firewall-namespace-clean-c729df7b.log`), the private gate-open fault
failed in 14s (`job-ipsec-firewall-gate-open-cut-c729df7b.log`), and recovery
clean passed in 17s (`job-ipsec-firewall-clean-after-gate-cut-c729df7b.log`).
Final independent lifecycle review is CLEAR for this IPv4 single-tunnel sequence.
Both cleans advanced the positive-control counters by 252 bytes, lost all three
judged probes, advanced strongSwan 328→580 bytes/4→7 packets and the drop counter
4→7, while Ze inbound and mismatch stayed at zero. The fault retained the SA
and required firewall finding but delivered all three probes, failing the
checker's own loss assertion. That assertion stops before post-probe drop and
mismatch reads, so no post-fault drop/mismatch delta is claimed.

All VMs stopped without interruption or leftover resources on native image
`45cb736f9a13fd62`. Cleans had all zero exits; the fault had check/command exits
zero and lab exit one. The accepted receipt landed in `eb52f67f57`; main then
confirmed no open/process references and removed the private firewall Go/JSON
inputs. Small proof logs/recordings remain through closure.

FRR's own `ipsec-bgp-redistribute-frr` scenario passed in 5s
(`job-ipsec-frr-redistribute-current-029e809f.log`), recording
`ipsec-bgp-redistribute-frr-7GW5D6CTZCNUGK4QGU4RVLZ3ZK.jsonl`. All exits were
zero, interrupted false, VM stopped and left-behind none on native image
`45cb736f9a13fd62`. Post-fix form-switch and ICMP clean also passed in 8s and
10s, with final independent lifecycle acceptance CLEAR. Existing cuts retain
their original producer identity and credit; no new cut is claimed.

Independent FRR acceptance is CLEAR for BGP learning and withdrawal only.
FRR learns `10.200.0.0/24` as valid/best from `172.28.0.2`, AS path `65001`
and next hop `172.28.0.2`. Stopping strongSwan withdraws that prefix while BGP
stays Established, with one connection, dropped zero and accepted zero at
the final observation. The actual JSON has `fibInstalled: false`; this receipt
gives no FRR FIB-installation or payload-forwarding credit.

### AC-1 native ICMP proof receipt

All seven runs of the bare-ESP `icmp-error-payload-check` scenario are observed
and independently reviewed CLEAR for the current IPv4 single-pair tunnel.
Logs use the `job-ipsec-icmp-` prefix and `-98342191.log` suffix in session scratch.

| Proof | Observed sequence and packet assertion |
|---|---|
| (c) | `clean-current` PASS 10s, `flag-cut-current` FAIL 8s, `clean-after-flag-cut` PASS 10s. With the ICMP state flag omitted, the consistent error decrypted 336 ESP octets but delivered no Time Exceeded. The preceding guards require strongSwan error generation and SA carriage; the exact generation count is not printed |
| (d) | Starting from that recovery clean, `policy-only-cut` PASS 10s, `state-only-cut` PASS 10s, `combined-selectors-cut` FAIL 8s, `clean-after-selector-cuts` PASS 10s. Widening both sources delivered three Time Exceeded messages for the inconsistent embedded 10.2.0.1 to 10.7.0.5 packet. The consistent control and generation/carriage guards passed before this intended failure |

The production XFRM source stayed at SHA256 `cb56b012a0e12c75353b2d3a881701c0034a882a2da1ccf379550cbf5c26db19`.
Reviewed private source/overlay hashes matched before each trial. All seven
VMs stopped without interruption or leftover resources on the same native
image `45cb736f9a13fd62`. Fault runs had check/command exits zero and lab exit
one; passes had all zero exits. This observes AC-1 and its two required proofs.
It gives no general per-SA, multiple-pair or protected inner-fragment credit,
and leaves AC-4 unchanged.

The final post-namespace-fix ICMP clean passed in 10s
(`job-ipsec-icmp-after-namespace-98342191.log`), recording
`icmp-error-payload-check-3QRH2IBDU7IY5M23OU5CFPVIGM.jsonl`, independently CLEAR.
The consistent step generates three errors, advances inbound ESP 0→336 and
Time Exceeded 0→3. The inconsistent step generates three more, advances inbound
ESP 336→672, leaves Time Exceeded at three and prints no error. The temporary
peer policy is removed. All exits are zero, interrupted false, no leftovers
and VM stopped on native image `45cb736f9a13fd62`. This completes the namespace
phase's live clean validation, with AC-4 and all scope limits unchanged.

### AC-2 native form-switch receipt

The clean live run passed in 8s (`job-ipsec-live-form-switch-clean-de5e18d6.log`).
The as-used binding cut then failed in 16s at the first actual bare probe,
with egress observed and no reply after IKE and the Child SA established
(`job-ipsec-live-form-switch-binding-cut-de5e18d6.log`). The post-clean run
passed in 8s (`job-ipsec-live-form-switch-clean-after-cut-de5e18d6.log`), with
fresh recording `esp-form-change-73ZQMZSGXWCCXBQQR2WCLACH4B.jsonl`.
The cut left no lab resources; its lab exit was 1, while check and command
exits were zero. The post-clean run had all zero exits. Both VMs stopped.

The cut's AS-USED private source SHA256 is
`a47c67ee90560536a43531ebbedc8ed7ad54283870b809d8add1469e80a64e77`.
The later required mutation-marker correction changed only its comment;
current source SHA256
`0af0356d5c7ca16099e2a750c856d9f88cd94d678d93a098f3aebcf8c89ca937`
is unexecuted. The as-used trial validates the AC-2 goal. Its missing marker was
procedural and is corrected for future inputs, with no freshness rerun. The current
bytes do not inherit the as-used trial's identity. Other interop and RFC obligations
remain open.

The post-namespace-fix clean `esp-form-change` passed in 8s
(`job-ipsec-form-switch-after-namespace-89683801.log`), recording
`esp-form-change-YMJRYRZ2L4LFC4DBJTXCVALXSL.jsonl`. All exits were zero,
interrupted false, VM stopped and left-behind none on native image
`45cb736f9a13fd62`. Final independent lifecycle review is CLEAR. All four
directed counters advance 0→46→92→138 bytes, identities remain unchanged and
wire forms/sequences are bare 1 → UDP 2 → bare 3. Mismatch and socket drops
stay zero. This adds a clean lifecycle receipt; original discrimination keeps
its prior producer identity and no new cut is claimed.

### AC-23 missing-state checker correction

The new empty and whitespace regression cases both failed against the old
acceptance branch (`job-ipsec-form-change-missing-state-red-57ce2d65.log`,
native `job-unit-pkg-7f9efd7e.log`). The one-branch source correction now
returns a missing-state error. The focused run then passed both new top-level
tests and both missing-state subtests, plus the existing
`TestESPFormChangeClaimsThreeDirections`
(`job-ipsec-form-change-missing-state-green-3e1c452a.log`). The full checker
package also passed, with one package OK
(`job-ipsec-interop-checker-package-a60a1a60.log`). Live interop and AC-2's
form-switch and counter repair remain separate obligations. The correction
landed in `cb5e4a6044`. Its focused and full-package passes preceded the
subsequent helper wiring and provide no result for that later phase.

### AC-14 peer-removal proof

The internal and external applicability-guard cuts each failed at their own
no-peer observation. Both exposed `device=true retained-cause=true counter-warning=true`
in one doctor answer, so the failure includes the independent runtime-callback
witness. Both normal cases then completed all 80 requested stress invocations
with no skips, failures or panics. Each retained capture contains 80 zero-exit
invocations and 320 passing VERIFY STEP rows. The native receipts are
`job-doctor-peer-removal-stress-internal-7f7378fa.log` and
`job-doctor-peer-removal-stress-external-b1c4d427.log`. Both guests used the Ze
kernel `7.3.0-ze`, SHA256
`3c13600db1d2c6fb8028e8fdd16420e5b376a8f9315d008313edd027ab43502e`,
and stopped with check, command and lab exits zero.

Both unchanged cases are now live:
`test/ipsec/ipsec-esp-form-doctor-peer-removal.ci` and
`test/ipsec/ipsec-esp-form-doctor-peer-removal-external.ci`.
The full affected live Linux population then passed in one guest
(`job-ipsec-promoted-live-linux-suite-a630ddb5.log`). All 50 distinct live CI
names passed, matching the 50 expected files with none missing or unexpected.
Each selected case executed with one phase and zero skips. The guest stopped
with check, command and lab exits zero and no interruption, on the same Ze
kernel `7.3.0-ze` and SHA256 recorded above. This includes the 13 formerly
Linux-skipped cases, the same-SPI childless recovery case and both promoted
peer-removal cases. No separate 13-case rerun is owed. All other acceptance
criteria, including AC-4's partial RFC status and interop proofs, remain unchanged.

### AC-22 capability-probe correction

All five selected Linux engine probes executed in
`job-ipsec-engine-final-guest-007d433f.log`. Four passed, including the fragment
probe's 16 protection contexts. The capability probe reached its read-only
receipt assertion and failed with a nil error instead of `EINVAL`.
`NetlinkRequest.Serialize` emits its typed data before its raw data, so the
old mixed request put the capability attribute where the fixed header belongs.
The correction keeps the fixed header and appends the serialized attribute
as raw data. It changes no positive check or rejection expectation.
Independent judgment approved the one-line test correction, with native approval
recorded for `engine.TestRFC4301KernelAdvertisesExactStateScope`. The corrected
probe passed without skips on the same Ze 7.3.0 kernel
(`job-ipsec-receipt-encoding-guest-fc673346.log`), with check, command and lab
exits zero and the VM stopped. Its engine binary SHA256 was
`bb686d9bf0d12ac7005a5b15e784ba6ec1beee55a4547ecf6e6407b93b56e854`.
All five engine probes therefore have observed PASS results across the initial
four and this targeted correction. The unchanged four need no repeat solely
to renew their results. AC-4's RFC annotation and other proof obligations
remain open. No vendor, kernel or production code changed for this correction.

### AC-21 handoff proof

`TestESPGroupReloadWaitsForChildlessOwnerHandoff` fixes the ordering through the
real reload channel. The receiving worker publishes ownership only after it
receives the request, then calls `adoptESPGroup`. The test checks that adoption
keeps the attached SA, both IKE SPIs and the responder's inactive schedule.
It changes no existing refusal expectation.

The original guard produced the test's own RED in `job-unit-pkg-0903bb4a.log`.
Main's race-enabled five-test run then passed without skips in
`job-childless-handoff-green-552f98a4.log`, including the existing installed-Child,
pending-handshake, in-flight-creation, missing-group and retry-schedule controls.
The independent review was CLEAN. Main then ran
`./le --name session test ipsec -p 1 -v ipsec-childless-ike-sa-recovers`
without changing the CI file. It passed in 33.2 seconds with all 15 VERIFY STEP
rows passing, including the same-SPI output and rejection of `RECONNECTED`.
The deterministic RED proves the handoff defect separately from that functional
result. The original failed runtime trace does not establish this attribution.
The full affected engine package also passed: native `test unit pkg` reported
one package OK in 162.552 seconds with `CGO_ENABLED=0`
(`job-childless-engine-package-94d132fd.log`, native log
`job-unit-pkg-93693a4c.log`). All other verification obligations remain owed.

### AC-19 lifecycle proof

AC-19 uses the controlled Go-test exception in `ai/rules/testing.md`,
"Lifecycle Regressions". A daemon test cannot reliably delay the revive
worker until after failure publication and Watch replacement. The Go tests
control that schedule while calling the real `read`, `Watch`, `retireLocked`
and `revive` transitions. They do not replace those transitions with a model.

| Reader scenario | Proof and reason for the controlled seam |
|---|---|
| A failure is recorded before Watch replaces the reader, and revive starts afterward | `TestESPFormReceiverReportsFailureRetiredBeforeRevive` checks one event and a serving replacement with both channels ready. A daemon test cannot force this worker ordering. |
| Retirement intentionally stops a reader before it records a failure | `TestESPFormReceiverIntentionalRetirementHasNoFailureEvent` checks no event and no reopen through the real retirement transition. |
| Delayed recovery reaches a retired life | `TestESPFormReceiverRetiredLifeCannotReopen` checks no open, retry or backoff change. |
| A mature failed reader reports after its replacement has a backoff | `TestESPFormReceiverMatureRetiredLifeKeepsBackoff` checks one event, a serving replacement and unchanged backoff. A daemon clock cannot force the reset boundary and worker ordering. |
| Read failure and last Forget race with revive | `TestESPFormReceiverConcurrentFailureAndRetirementReportsEveryRecordedFailure` races the actual transitions and requires one event for every recorded failure after the workers join. It does not claim to force the original publication gap deterministically. |

The concurrent test failed with zero events for a recorded failure on its
first iteration under a scheduler-only yield between the old stop check and
failure publication. That is a forced-scheduling RED, not a naturally observed
baseline failure. Corrected source passed all six selected Linux tests without
skips under the same controlled scheduling yield
(`job-ipsec-reader-publication-green-guest-49f99f94.log`). Count coverage recorded
67 intentional read returns and 1090 failure publications, 278 stale and 812
current reader outcomes, and retirement of one absent and 1158 current readers.
The guest used Ze's 7.3.0 kernel and stopped with check, command and lab exits
zero. This is controlled-overlay evidence. The final unmodified-source Linux
dataplane run then passed all 35 selected tests without skips on Ze's 7.3.0
kernel (`job-ipsec-dataplane-final-guest-b9e9af5d.log`). Its binary SHA256 was
`0309b7b65ed086db18f3051fb4023cb959fd8c630b0e9548c384ddd25ea818cd`.
The isolated stale-backoff guard cut also produced the intended test failure:
`TestESPFormReceiverMatureRetiredLifeKeepsBackoff` observed 0 seconds instead
of 30 seconds (`job-ipsec-reader-stale-backoff-guest-5c17e031.log`). That guest
stopped with check and lab exits zero and command exit one. Reader code,
tests and the reader-only architecture hunk landed in `afd59c50b9`.
The ordered cases keep failure-before-retirement and retirement-before-publication
distinct. All other acceptance criteria and remaining proofs are unchanged.

These tests prove the receiver's event and recovery behavior, shared by both
plugin modes. They do not claim an observed reader failure in the daemon's
error ring. The unchanged `runEngine` registration and
`espFormReaderStoppedSink` connect that event to SDK `RaiseIssue`.
`TestESPFormReaderStoppedSinkRaisesEachStopOnce` checks the event's code,
severity, cause and fix. The existing
`test/plugin/doctor-plugin-report-raise-external.ci` tests the generic report
transport across the process boundary. Those are separate layers of evidence.

The standard plugin lifecycle scenarios below cannot be reached by the AC-19
correction. AC-20's internal and external engine-start proof remains separate.

| Standard scenario | Why AC-19 cannot change it |
|---|---|
| A plugin process ends after declaration, for restart, ignore or fatal policy | Reader failure ends a receiver worker, not the plugin process; failure-policy dispatch is unchanged. |
| A plugin fails a startup stage after declaration | The corrected worker transitions run after receiver start; startup-stage handlers are unchanged. |
| A restarting plugin exceeds its restart bound | Receiver reopen backoff is separate from plugin restart accounting, which is unchanged. |
| An operator's respawn leaf conflicts with the declaration | No declaration or respawn validation changes. |
| A plugin exits during stop drain or teardown | The correction joins or retires receiver workers; it does not change plugin-exit dispatch or the stop protocol. |
| A signal arrives during the config fence, drain or teardown | No signal handler or daemon stop-state transition changes. |
| A reload or commit arrives after shutdown begins | No configuration admission or shutdown fence changes. |
| A plugin crashes during verify or apply, misses a deadline or reports broken | No transaction callback, deadline or broken-state handling changes. |
| Reload context ends during startup handshake, or a sibling fails at the tier barrier | No handshake, tier barrier or reload cancellation changes. |
| A plugin dispatches another plugin's command at startup | The receiver reports an issue after failure; startup command dispatch is unchanged. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs a strongSwan site-to-site tunnel where IKE floated to 4500 with no NAT | bare ESP → fast path → XFRM → policy check | interop `icmp-error-payload-check` |
| 2 | peer switches ESP form mid-session | both forms on one SA | interop `esp-form-change` |
| 3 | runs `ze doctor` on a box whose fast path could not load | doctor check → cause and fix | `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci` (promoted to `test/ipsec/` after its runs) |
| 4 | installs Ze under systemd and runs a dual-form Child SA | unit grants `CAP_BPF` → fast path loads | `TestUnitFileCapabilities`, `TestESPFormFastPathLoadsUnderTheUnitCapabilities` |
| 5 | writes a firewall term that drops `protocol esp` on input, through Ze or with `nft` | the kernel ruleset → `espFormFirewallBlocks` → gate shut → the rule drops the bare ESP; `ze doctor` names the rule | interop `esp-form-firewall-drop`, `TestRFC4301BareESPFirewallDropKeepsTheFastPathOff` |
| 6 | runs a peer whose first Child SA this node cannot install | IKE SA kept, peer half deleted, retried on the schedule; `ze doctor` names the cause | `TestInitiatorKeepsIKESAWhenItCannotInstallTheFirstChildSA`, `TestChildInstallFindings` |

## 🧪 TDD Test Plan

### Unit Tests

Files under `internal/component/ike/dataplane/` unless a path is given.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRawSocketFragmentPolicyAuthority`, `TestRawSocketFragmentRouteAuthority`, `TestRawSocketFragmentPolicyLifetime` | `internal/component/ike/engine/raw_socket_fragment_policy_integration_linux_test.go` | AC-29: existing captured raw-sender authority through IPv4/IPv6 fragment gathering, with whole/same-authority and lifetime controls | authored/reviewed/formatted/built. Qualifying old-pair RED exposed two IPv4 whole EPERM failures; unchanged binary passes both cases on d87/d2eb +43a1. Full three-test population remains unrun on that pair; exact assertions stay unchanged |
| `TestFragmentSAReplacementKeepsQueuedIdentity` | `internal/component/ike/engine/fragment_sa_lifetime_integration_linux_test.go` | existing AC-4 queue lifetime: same-public-identity/key SA replacement cannot borrow a removed generation's queue, while whole/fresh same-generation controls deliver, both families/orders | Original `d030` independently CLEAR, formatted/built; IPv4 both orders PASS, IPv6 local-counter witness FAIL before replacement. See current checkpoint for additive counter-test source and owed checks; ordinary test, no new RFC claim |
| Seven real nft namespace/capability cases | `espform_netns_integration_linux_test.go` | AC-24: reader/watch work under the exact unit grant, valid current fd zero works, different targets at ordinary/fd-zero descriptors and closed/closed-low handles refuse without fallback | all seven PASS without skips in `job-ipsec-namespace-unit-caps-green-59710cd6.log` after the corrected Linux build; two independent CLEAR reviews |
| `TestESPFormFastPathFollowsLinksUnderTheUnitCapabilities` | `espform_fastpath_unit_caps_integration_linux_test.go` | AC-24: real start, initial links, link following and stop under the exact unit grant without `CAP_SYS_ADMIN` | actual TCX follow/delete/stop PASS without skips in the same eight-parent guest population, `job-ipsec-namespace-unit-caps-green-59710cd6.log` |
| `TestDoctorESPFormFirewallPermissionCauseRetainsUncertainty` | `internal/component/ike/engine/doctor_esp_form_permission_test.go` | AC-25: permission cause remains uncertain while code, warning, restart and log guidance stay intact | own RED `job-ipsec-doctor-permission-cause-red-1535be55.log`, then regression plus two doctor companions PASS in `job-ipsec-doctor-permission-green-a5623ea2.log`; native engine package 1.496s |
| `TestIPsecPeerFormWiring` | `internal/le/interoplab/ipsec/peerform_wiring_test.go` | AC-2 helper has one image owner, one suite staging entry and the executable COPY | named PASS and all four subtests PASS; five generic staging consumers also PASS in `job-ipsec-peerform-staging-named-green-0958ee3a.log` |
| Command/action, migration, replay, frame/reply and Linux projection/link tests | `internal/le/interoplab/ipsec/peerform/` `action_test.go`, `migration_test.go`, `replay_test.go`, `frame_test.go`, `reply_test.go`, `projection_linux_test.go`, `link_linux_test.go` | AC-2 helper selects one exact outbound SA, preserves identity/replay state, observes the requested ESP form and matches its probe reply | portable helper package PASS 0.242s in `job-ipsec-peerform-checker-packages-green-2d1ebed9.log`; Linux cross-build PASS and native Linux helper tests PASS without skips in `job-ipsec-peerform-linux-units-dcd7f143.log`, including projection, link, GETSA and migration framing |
| `TestESPFormSwitchKeepsOneSA`, `TestESPFormSwitchRequiresInitialDisagreement`, `TestESPFormSwitchRejectsAStalledDirection`, `TestESPFormSwitchRejectsInvalidProbeEvidence`, `TestESPFormSwitchRejectsInvalidMigrationEvidence`, `TestESPFormSwitchRejectsARefusedOperation`, `TestESPFormSwitchRejectsUnreadableProbeEvidence`, `TestESPFormSwitchRetainsSafetyChecks`, `TestESPFormSwitchRejectsOutboundOnlyZeTemplate` | `internal/le/interoplab/ipsec/espform_switch_test.go` | AC-2 bare/UDP/bare checker phases retain one SA, verify all four traffic directions and reject missing, outbound-only or inconsistent evidence | outbound-only template regression produced its own RED in `job-ipsec-outbound-template-regression-red-69e830c3.log`; directed-reader correction source review CLEAN, then full checker package PASS 10.829s in `job-ipsec-peerform-checker-packages-green-2d1ebed9.log`, including 12 direction/phase stalls and malformed receipts; old counter-fixture replacement was independently approved |
| `TestESPFormChangeRejectsMissingZeState`, `TestESPFormChangeAcceptsPresentZeState` | `internal/le/interoplab/ipsec/espform_change_snapshot_test.go` | AC-23: empty and whitespace-only snapshots refuse, while a populated state proceeds through the existing checker | actual missing-state RED followed by focused PASS with both subtests and the existing direction-claim control; full checker package PASS |
| `TestRFC4301KernelAdvertisesExactStateScope` | `internal/component/ike/engine/rfc4301_state_scope_readback_integration_linux_test.go` | AC-22: correctly ordered fixed header and capability attribute test the existing read-only `EINVAL` rejection, retaining every positive receipt check | corrected probe PASS without skips on the same Ze kernel; independent judgment and native approval recorded |
| `TestESPGroupReloadWaitsForChildlessOwnerHandoff` | `internal/component/ike/engine/reconcile_childless_handoff_test.go` | AC-21: attached childless SA reaches owner adoption before ownership publication and keeps both SPIs and the responder schedule | actual RED, then race-enabled PASS with four existing refusal/schedule tests and no skips; unchanged functional case PASS |
| `TestESPFormRewriteProgram` | `espform_program_integration_linux_test.go` | AC-6, IPv4 and IPv6 program variants, through `BPF_PROG_TEST_RUN` (guest-root) | |
| `TestWriteESPFormBuildsEncapsulatedDatagram` (updated) | `espform_test.go` | IPv4 header checksum now valid | |
| `TestESPFormIPv4ChecksumKnownHeader`, `TestWriteESPFormChecksumOverReusedBuffer` | `espform_test.go` | the RFC 791 checksum, and no stale octet from a reused buffer | |
| `TestWriteESPFormCarriesTheTOS`, `TestESPFormBareIPv4` | `espform_test.go` | AC-11 builder half: the received TOS reaches the built header; the header split refuses a malformed datagram | |
| `TestESPFormRunRepresentsWatchedSPI` (updated), `TestESPFormRunServesOnlyThePeersSource`, `TestESPFormRunServesOnlyTheSAsLocalAddress`, `TestESPFormRunKeepsTheOuterTOS` | `espform_run_linux_test.go` | AC-9: the complete path writes the datagram to the injector, serves only the SA's peer and local address, and keeps the outer TOS | |
| `TestXFRMInstallSARefusesIPv6DualFormSA` | `xfrm_linux_test.go` | AC-8: the IPv6 dual-form refusal is an operator error naming the gap | |
| `TestESPFormFastPathGateOpensOnSafeRuleset`, `TestESPFormFastPathGateShutsOnBlockingRule`, `TestESPFormFastPathGateShutsOnDumpError`, `TestESPFormFastPathGateShutsWhenWatchEnds`, `TestESPFormFastPathGateIgnoresJudgmentBeganBeforeHold`, `TestESPFormFastPathGateIgnoresJudgmentBeganBeforeCommit`, `TestESPFormFastPathHoldShutsGateBeforeReturning`, `TestESPFormFastPathReleaseJudgesAgain`, `TestESPFormFastPathHoldOnBareBackend` | `espform_fastpath_gate_linux_test.go` | AC-13: the gate opens only from a judgment that began after the last hold, release and commit, shuts on a blocking rule, a dump error or an ended watch, and a hold shuts it before it returns | |
| `TestESPFormFastPathGateFollowsTheKernelRuleset` | `espform_fastpath_gate_linux_test.go` | AC-13 on a real kernel: the gate follows nftables commits (guest-root) | |
| `TestESPFormFirewallBlocks`, `TestESPFormFirewallReaderError`, `TestESPFormFirewallSiteString` | `espform_firewall_linux_test.go` | AC-13: the ruleset walk (hooks, families, exclusions, verdicts, jumps, policy, unknown expressions) and the site it names | |
| `TestESPFormRuleFromMessage`, `TestESPFormNFTWatch` | `espform_firewall_linux_test.go` | AC-13: the GETRULE decode keeps every expression; the watch signals each commit and ends with its cause | |
| `TestESPFormFirewallGenerationBracket`, `TestESPFormGenerationFromMessage` | `espform_firewall_linux_test.go` | AC-13, review round 5: the GETGEN bracket around a judgment (`errESPFormRulesetChanged`) and the NEWGEN decode | |
| `TestESPFormFastPathGateRefusesASplicedRead` | `espform_fastpath_gate_linux_test.go` | AC-13, review round 5: a judgment read across a commit never publishes | |
| `TestESPFormFastPathRejudgeWaitDoublesToCap`, `TestESPFormFastPathGateStaysShutWhileCommitsKeepComing`, `TestESPFormFastPathMovingRulesetWaitsInTheLoop`, `TestESPFormFastPathInterruptedDumpCountsAsMoving`, `TestESPFormFastPathMovingReadingRecordsTheStream`, `TestESPFormFastPathMovingStreamEndsOnASafeRead`, `TestESPFormFastPathMovingStreamEndsOnAFailedDump`, `TestESPFormFastPathStopEndsTheMovingReading` | `espform_fastpath_gate_linux_test.go` | AC-13, review rounds 6 and 7: shut on every change, the rejudge backoff, `NLM_F_DUMP_INTR` as a move, and the `moving` reading | |
| `TestESPFormFastPathFailedShutDetaches`, `TestESPFormFastPathRefusedCommitShutDetaches`, `TestESPFormFastPathSubscriptionEndKeepsTheDetachCause` | `espform_fastpath_gate_linux_test.go` | review rounds 9 and 10: a refused gate shut records `Detached` with its cause | |
| `TestESPFormFirewallStockGTSMKeepsTheFastPathOn`, `TestESPFormFirewallStockGTSMRangesKeepTheFastPathOn`, `TestESPFormFirewallStockDDoSNamedProtocolKeepsTheFastPathOn`, `TestESPFormFirewallStockDDoSUnnamedProtocolClosesTheFastPath`, `TestESPFormFirewallStockDDoSESPVectorClosesTheFastPath` | `espform_firewall_stock_linux_test.go` | AC-13, review round 4: the production GTSM and DDoS tables (`gtsm.FilterTables`, `local.MitigationTable`), lowered by the nft backend and read back from the kernel, keep the fast path on, except a DDoS vector with no protocol name or on ESP | |
| `TestBuildDropTermPutsTheProtocolFirst` | `internal/plugins/ddos/local/match_test.go` | review round 4: the DDoS drop term opens with its protocol match | |
| `TestLocalMitigationDropsTheVectorAndNothingElse` | `internal/plugins/ddos/local/mitigation_drop_integration_linux_test.go` | review round 4: the reordered mitigation still drops the vector, and nothing else | |
| `assertListenDrop` (judged change) | `internal/component/gtsm/gtsm_test.go` | review round 4: the GTSM listen-port drop's four matches, the protocol first | |
| `TestXFRMMigrationIPv4RefusesAnAddressWithNoIPv4Form`, `TestValidateXFRMMigrationRefusesIPv6AsOperatorError` | `xfrm_migrate_linux_test.go` | AC-8: a MOBIKE move to an address with no IPv4 form is an operator error | |
| `TestXFRMMigrationUnwatchedSAOpensThePathsBeforeTheMove` | `xfrm_migrate_linux_test.go` | review round 11: an unwatched SA's paths open, and are held, before the SAD moves; paths that cannot open refuse the move as an operator error with nothing moved; a landed move serves the SA on the new address with its peer and ports; a failed move ends the hold and registers nothing | |
| `TestESPFormOpenRefusalsCarryTheirFix` | `espform_refusal_linux_test.go` | review rounds 12 and 13: EPERM, EACCES, file-limit, memory and unknown-error classes keep the kernel cause and each exact operator fix | written; final run owed |
| `TestESPFormBypassRefusesASocketItCannotExempt` | `espform_linux_test.go` | review round 13: the `*net.IPConn` exemption seam refuses a socket it cannot exempt and carries the operator error | written; final run owed |
| `TestXFRMMigrationUnservedRendersTheFixOnce`, `TestXFRMMigrationUnservedPassesADefectThrough` | `espform_refusal_linux_test.go` | review round 12: a migration refusal renders its cause's fix once; a Ze defect stays a wrapped defect without an invented operator fix | written; final run owed |
| `TestESPFormReceiverWatchReopensAStoppedReader`, `TestESPFormReceiverWatchRefusesWithTheRecordedOpenFailure` | `espform_reopen_linux_test.go` | review rounds 12 and 13: Watch meets a stopped reader, reopens or refuses, and retains the refusal for the doctor | written; final run owed |
| `TestESPFormReceiverReopensAStoppedReaderByItself` | `espform_reopen_linux_test.go` | review rounds 12 and 13: an installed SA is served again without a new Watch; the refused attempt is recorded and a successful open clears it | correction committed in `bd8e21b487`; targeted package run `job-unit-pkg-58fce57a.log` and race/count-20 scheduling run `job-ipsec-reopen-schedule-e00a01c8.log` passed; full final guest run owed |
| `TestESPFormReceiverHandsEachReaderStopOnOnce`, `TestESPFormReceiverRefusesAfterClose`, `TestESPFormReceiverBacksOffAReaderThatDiesAtOnce` | `espform_reopen_linux_test.go` | review round 13: one report per stopped reader, no Watch or acquire after Close, and receiver-level backoff preserved across short-lived readers | written; final run owed |
| `TestESPFormReceiverEndsOnlyAHoldItHolds`, `TestXFRMMigrationPanicDuringTheMoveEndsTheHold` | `espform_reopen_linux_test.go`, `espform_refusal_linux_test.go` | review round 12: an unmatched hold end panics before changing the count; panic during the SAD move releases its outstanding acquisition | written; final run owed |
| `TestDoctorESPFormCompletePathFailure`, `TestDoctorESPFormDeviceFindingsReportOneCauseOnce` | `internal/component/ike/engine/doctor_esp_form_complete_path_test.go` | review rounds 12 and 13: the receiver's recorded refusal carries its cause, time, consequence and exact fix even when counters cannot be read; duplicate causes produce one finding | written; final run owed |
| `TestESPFormReaderStoppedSinkRaisesEachStopOnce` | `internal/component/ike/engine/register_esp_form_reader_test.go` | review round 13: the sink raises the registered error event with its time, read error and warning-list fix | written; final run owed |
| `TestXFRMMigrationReceiverFollowsTheMove`, subtest "lost rollback forgets the SA" | `xfrm_migrate_linux_test.go` | review round 11: a lost rollback answers `ErrTunnelMigrationLost` and leaves the SA watched on neither address | |
| `TestXFRMMigrationFailedRollbackDiscardsStates` | `xfrm_migrate_integration_linux_test.go` | review round 11: the inbound SA is watched on its old address first, as `InstallSA` watches a templated SA, so the check reads a watch the lost rollback had to forget | |
| `TestXFRMMigrationReceiverFollowsTheMove`, `TestXFRMMigrationRetargetRefusesAnotherSAsAddress`, `TestXFRMMigrationRetargetServedWithNoFastPathProgram`, `TestXFRMMigrationRetargetsBothFormReceiver` (judged change) | `xfrm_migrate_linux_test.go` | review rounds 9 and 10: the collision check before the SAD moves, and the receiver following the move | |
| `TestESPFormRegistryHoldsTwoSAsOnOneSPI`, `TestESPFormRegistryListsSharedSPIs`, `TestESPFormRegistryWatchesOnlyTemplatedSPIs` (judged change) | `espform_test.go` | review rounds 9 and 10: the registry keyed by SPI and local address | |
| `TestESPFormReceiverReportsSharedSPI`, `TestESPFormReceiverWarnsOfNoSharedSPIWhileUnavailable`, `TestESPFormRegistryRetargetRefusesATakenAddress` | `espform_linux_test.go` | review rounds 9 and 10: the shared SPI is reported, and a retarget onto a taken address is refused | |
| `TestESPFormRunServesTwoSAsOnOneSPI` | `espform_run_linux_test.go` | review rounds 9 and 10: the complete path serves both SAs on one SPI, told apart by destination | |
| `TestESPFormFastPathWarnsOfASharedSPIWhileAttached` | `espform_fastpath_integration_linux_test.go` | review rounds 9 and 10: the shared-SPI warning while the fast path is attached (guest-root) | |
| `TestESPFormTUNOpenRefusalNamesTheCause`, `TestESPFormTUNSetIffRefusalNamesTheCause`, `TestESPFormTUNMissingNodeCreated`, `TestESPFormTUNNodeCreationOutcomes`, `TestESPFormTUNNodeUserNamespace`, `TestESPFormTUNNodeCreatedAsRoot` | `espform_tun_open_linux_test.go` | AC-9, AC-14: the clone device's open and `TUNSETIFF` refusals name the cause and fix; a missing node is created, and an existing one never replaced | |
| `TestESPFormTUNProbePredictsTheSysctlRefusal`, `TestESPFormTUNProbeAgreesWithTheReceiver`, `TestESPFormTUNSharedSysctlRule`, `TestESPFormTUNSysctlWritableWritesNothing` | `espform_tun_probe_linux_test.go` | AC-14: the apply's probe answers what the receiver's open would, and writes nothing | |
| `TestESPFormTUNSysctlsWriteOnlyWhatTheKernelNeeds`, `TestESPFormTUNSysctlReadParsesTheValue` | `espform_tun_sysctl_linux_test.go` | AC-9: only the sysctls the receive path depends on are written; review round 13 adds the operator refusal for an EIO write, preserving its cause and naming the exact conf/default fix | final run owed |
| `TestESPFormLinkSelection` | `espform_fastpath_linux_test.go` | link type to L2 offset, `lo` and `ze-espform` excluded | |
| `TestESPFormReceiverHoldsTUNOnlyWhileWatching` | `espform_linux_test.go` | AC-7, AC-9: the TUN exists only while an SPI is watched, with its sysctls and its 192.0.0.8/32 address (guest-root) | |
| `TestESPFormReceiverWatchCannotRaceTheLastForget` | `espform_linux_test.go` | a Watch racing the last Forget never finds the receiver stopping under it | |
| `TestESPFormReceiverServesWhenTheFastPathCannotLoad` | `espform_linux_test.go` | AC-10 precondition: an unavailable fast path never fails a Watch | |
| `TestESPFormFastPathAttachesEveryLink` | `espform_fastpath_integration_linux_test.go` | AC-7 (guest-root) | |
| `TestRecordingESPFormUnavailableBackendAbsentFromShippedBuild` | `recording_shipped_test.go` | the zetest-only `recording-esp-form-unavailable` backend never ships | |
| `TestESPFormFastPathLoadsUnderTheUnitCapabilities`, `TestOverrideGuardLoadsUnderTheUnitCapabilities` | `unit_capabilities_integration_linux_test.go` | AC-12 (guest-root, uid 65534); `TestDataplaneUnitCapabilitiesHelperProcess` is the child process they run, not a test of its own | |
| `TestTrafficUsageLoadsUnderTheUnitCapabilities` | `internal/plugins/trafficusage/unit_capabilities_integration_linux_test.go` | AC-12 (guest-root, uid 65534); `TestTrafficUsageUnitCapabilitiesHelperProcess` is its child process | |
| `TestDoctorESPFormFastPath`, `TestDoctorESPFormFastPathRegistered`, `TestEngineDoctorCheckDispatch` | `internal/component/ike/engine/doctor_esp_form_fast_path_test.go` | AC-10, AC-14: the findings, their four declared codes, and the engine answering its runtime checks in its own process | |
| `TestDoctorESPFormTUN`, `TestRecordESPFormTUN` | same file | AC-14: the apply records the TUN outcome, and the check only reads it | |
| `TestInitiatorKeepsIKESAWhenItCannotInstallTheFirstChildSA`, `TestInitiatorKeepsIKESAWhenItCannotBuildTheFirstChildSA`, `TestInitiatorEndsIKESAWithNoKeysForTheFirstChildSA`, `TestChildInstallRefusalBacksOffToTheCapAndResetsOnSuccess`, `TestChildInstallRefusedFix` | `internal/component/ike/engine/child_install_refusal_test.go` | AC-15 | |
| `TestRecordChildRefusedClearsOnInstall`, `TestChildInstallFindings`, `TestChildCreateRefusalIsRecordedForTheDoctor`, `TestFirstChildSARefusalIsRecordedForTheDoctor`, `TestChildInstallCheckRegistered` | `internal/component/ike/engine/doctor_child_install_test.go` | AC-16 | |
| `TestHoldESPFormFastPathForCommit`, `TestFirewallCommitHoldsESPFormFastPath` | `internal/component/ike/engine/register_firewall_observer_test.go` | AC-13: a firewall commit takes and releases the fast path's hold | |
| `TestApplyAllRunsCommitObserversAroundApply`, `TestCommitObserverNilAfterIsAllowed`, `TestFlushAllTablesRunsCommitObservers`, `TestCommitObserversSkippedWithoutApply`, `TestCommitObserverReleasedWhenApplyPanics` | `internal/component/firewall/commit_observer_test.go` | AC-13: observers run around every `Backend.Apply`, their after functions run on error and panic, and none runs without an Apply | |
| `TestDaemonCapabilitiesCheck` | `internal/component/doctor/checks_daemon_capabilities_test.go` | AC-17 | |
| `TestTrafficUsageUnknownCarriesTheCapabilitiesFix` | `internal/plugins/trafficusage/doctor_test.go` | AC-12: the eBPF warning carries `DaemonCapabilitiesFix` | |
| `TestProgram_TruncatedPass` (rewritten), `TestProgram_IPv4HeaderEndingTheFrameCounts`, `TestProgram_IPOptionsMoveThePort`, `TestProgram_TruncatedL4HeaderKeysPortZero`, `TestProgram_L4OffsetBeyondThePacketKeysPortZero`, `runProg` (wants `TCX_NEXT`) | `internal/plugins/trafficusage/program_test.go` | AC-18: the scalar-offset port read and the `TCX_NEXT` verdict on every `BPF_PROG_TEST_RUN` path; `TestProgram_TruncatedPass` runs on a TCX-attached veth pair (guest-root) | |
| `TestProgram_ReturnsTCXNext`, `TestProgram_LetsAClsactFilterRun` | `internal/plugins/trafficusage/tcx_next_integration_linux_test.go` | AC-18 (guest-root) | |
| `TestContainerDevicesCoverWhatTestedPathsOpen` (renamed from `TestContainerDevicesCoverWhatPPPOpens`) | `internal/test/runner/caps_option_test.go` | the clean-tree container's device list covers `/dev/net/tun`, which `openESPFormTUN` opens | |
| `TestThePreparationLetsTheDroppedDaemonOpenDevNetTUN` | `internal/le/test/qemu/netns_alltests_test.go` | the QEMU per-test namespace preparation widens `/dev/net/tun` for the dropped daemon | |
| `TestESPFirewallFindingNamesTheTableAndTheTerm`, `TestAssertESPStillRefusesWhatCannotProveStillness` | `internal/le/interoplab/ipsec/espformfirewall_test.go` | the `esp-form-firewall-drop` checker's parsing and its stillness refusal | |
| `privilege_test.go` (updated) | `internal/le/interoplab/ipsec/privilege_test.go` | ze's lab peer holds `/dev/net/tun` and `/proc/sys` writable under the net-sysctl profile | |
| `evidence_test.go` (updated) | `internal/le/verify/evidence/evidence_test.go` | the clean-tree container grants `/dev/net/tun` | |
| `TestDaemonCapabilityBitsAreTheKernels`, `TestDaemonCapabilitiesAnswersACopy` | `internal/core/privilege/capabilities_linux_test.go` | AC-12: each bit is the kernel's | |
| `TestUnitFileCapabilities` (updated) | `internal/plugins/systemd/unit_test.go` | AC-12: the unit lists `CAP_BPF` | |
| `TestNetnsDaemonSetcapNamesTheGrant` and siblings (updated) | `internal/test/runner/caps_netns_test.go` | AC-12: the runner's netns grant derives from `DaemonCapabilities` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ESP payload length | 8 to 65535 minus headers | 65507 (IPv4) | 7 | 65508 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-esp-form-doctor-peer-removal` | `test/ipsec/ipsec-esp-form-doctor-peer-removal.ci` | AC-14: configured peer, last-peer removal and re-addition preserve the retained failure while device findings follow applicability, with an independent callback witness in the same answer | initial PASS, intended guard-cut failure, all 80 stress invocations PASS, promoted unchanged; full 50-case affected Linux population PASS without skips |
| `ipsec-esp-form-doctor-peer-removal-external` | `test/ipsec/ipsec-esp-form-doctor-peer-removal-external.ci` | AC-14: the same apply/doctor contract through the external IKE process | initial PASS, intended guard-cut failure, all 80 stress invocations PASS, promoted unchanged; full 50-case affected Linux population PASS without skips |
| `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked` | `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go` (guest-root; engine package so the SA is built by `createFirstChildSA`) | AC-3 | |
| `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE` | same file (guest-root) | AC-11 | |
| `TestRFC4301BareESPFirewallDropKeepsTheFastPathOff` | same file (guest-root; no RFC tag, because it proves a firewall interaction) | AC-13: an operator's `nft` drop rule on ESP keeps the fast path off and the rule drops the bare ESP; with the rule gone the fast path rewrites again | |
| `TestEncapEstablishedSAServesAPeerFormChange` (judged change) | `internal/component/ike/dataplane/encap_formchange_integration_linux_test.go` | AC-2 at the kernel: the complete path over `ze-espform` on routed addresses, with the inbound IKE bypass installed as production does | |
| `ipsec-esp-form-fast-path-doctor` | `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`, promoted to `test/ipsec/` after its runs | AC-10: `ze doctor` names the fast path state, under the zetest-only `recording-esp-form-unavailable` backend | |
| `service-unit-gen` (updated) | `test/ui/service-unit-gen.ci` | AC-12: the generated unit's capability lines | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `icmp-error-payload-check` | `test/interop-ipsec/scenarios/` | strongSwan | AC-1 | current IPv4 single-pair clean and proofs (c)/(d) observed in all seven native runs, independently reviewed CLEAR |
| `esp-form-change` | `test/interop-ipsec/scenarios/` | strongSwan | AC-2 | |
| `esp-form-firewall-drop` | `test/interop-ipsec/scenarios/esp-form-firewall-drop/` (`ze.conf`, `swanctl.conf`), checker `checkESPFormFirewallDrop` (`internal/le/interoplab/ipsec/espformfirewall.go`) | strongSwan | AC-13: strongSwan sends bare ESP on a 4500-without-NAT SA, Ze's `drop-esp` term drops it and counts it, the doctor names the blocking table and term; red when `writeGateLocked` writes the open value whatever it is given (`docs/architecture/testing/interop.md`) | clean PASS 17s → finding-preserving gate-open packet FAIL 14s → clean PASS 17s, independently CLEAR for current IPv4 single tunnel; no post-fault drop/mismatch delta claimed |

## Files to Modify
- `internal/component/ike/dataplane/espform_firewall_kernel_linux.go`, `espform_fastpath_linux.go` - use the shared current-namespace socket helper for the real nft reader/watch and link follower, with pinned capture/creation and unchanged worker ownership (AC-24).
- `internal/component/ike/engine/doctor_esp_form_fast_path.go` - preserve the unreadable-firewall cause and recovery guidance without inferring a missing capability from every permission error (AC-25).

From `git status --short` over the spec's paths on 2026-11-12, this spec's files only. **Shared** marks a file that also carries a hunk this spec cannot claim (another session's work, for example the RFC 4301 ICMP and implicit-discard work): commit this spec's hunks of it with `hunks <path> <patch-file>` after judging the rest against HEAD (`ai/rules/git-safety.md`).

Feature code:
- `internal/le/interoplab/ipsec/ipsec.go`, `register_images.go`, `test/interop-ipsec/Dockerfile.strongswan`, `.gitignore`, `.dockerignore` - image-owned staging, helper executable COPY and matching ignore declarations for AC-2.
- `internal/le/interoplab/ipsec/checkers.go` - planned AC-2 bare/UDP/bare phases and counter checks through the peer-local helper; AC-23's missing-state refusal stays in force.
- `internal/le/interoplab/ipsec/checkers.go` - `checkESPFormChange` rejects empty or whitespace-only Ze XFRM snapshots (AC-23).
- `internal/component/ike/dataplane/espform_linux.go` - the TUN writer replaces the `IPPROTO_RAW` injector; the reader reads the IPv4 header (`ReadMsgIP`) for the TOS; the fast path lifecycle; Watch and Forget decide start and stop under one lock.
- `internal/component/ike/dataplane/espform.go` - IPv4 header checksum, received TOS, `espFormBareIPv4`, registry comments.
- `internal/component/ike/dataplane/xfrm_linux.go` (shared) - the IPv6 dual-form refusal as an operator error naming the gap.
- `internal/component/ike/dataplane/xfrm_migrate_linux.go` - the IPv6 MOBIKE refusal names the gap.
- `internal/component/ike/dataplane/recording_zetest.go`, `register_recording_zetest.go` - the zetest-only `recording-esp-form-unavailable` backend.
- `internal/component/ike/engine/register.go` - the engine declares its runtime doctor checks in Stage 1 (`DoctorChecks: engineDoctorCheckDecls()`) and answers them through `OnDoctorCheck`.
- `internal/component/ike/engine/apply.go` - `recordESPFormTUN`: the TUN attempt on every apply that configures a peer, with its Warn line.
- `internal/component/ike/engine/established.go` - the first Child SA's install, binding and build refusals keep the IKE SA on `childCreateSchedule` and record the refusal (AC-15, AC-16).
- `internal/component/ike/engine/create_child.go` - `errChildBinding`, `errChildInstall`, `childInstallRefusedFix`; the creation response's refusal records itself for the doctor.
- `internal/component/ike/engine/child.go` - the build failures wrap `errChildProposal`, `errChildKeying` and `errChildNoIKEKeys`, which `runEstablished` folds.
- `internal/component/ike/engine/reconcile.go` - `PeerSession.childRefused`, read by the `ipsec-child-install` check; `retargetESPGroup` waits for an attached SA's owner handoff under the existing deadline (AC-21).
- `internal/component/ike/engine/application_override.go` - the guard's fix text names the older unit.
- `internal/component/firewall/registry.go` - `ApplyAll` calls `Backend.Apply` through `applyObserved`.
- `internal/component/firewall/reservation.go` - `TableControl` (`Plugin`, `Decides`, `Command`, each required), `MustReserveTableNames(owner, control, names...)`, which panics on an empty field, and `TableControlOf`, which answers the control of a reserved kernel table name; `reservation_test.go` adds `TestMustReserveTableNamesNeedsAControl`.
- Every reservation producer declares its control: `internal/component/firewall/register.go` (the conntrack helpers), `internal/component/firewall/plugins/irr/register_tables.go`, `internal/component/gtsm/register_tables.go`, `internal/component/ike/engine/register_tables.go` (the tunnel-source guard), `internal/plugins/anomaly/shape/register_tables.go`, `internal/plugins/copp/register_tables.go`, `internal/plugins/ddos/local/register_tables.go`, `internal/plugins/flowspec-firewall/register_tables.go`, `internal/plugins/iface/netlink/register_tables.go`, `internal/plugins/policyroute/register_tables.go`, `internal/plugins/vrrp/register_tables.go`.
- `internal/component/gtsm/gtsm.go` - `listenPortTerm` puts the protocol match first, ahead of the source, so stock GTSM over IPv4 keeps the fast path on (Decision, review round 4).
- `internal/plugins/ddos/local/match.go` - `buildDropTerm` puts the protocol match first, ahead of the victim address; `responder.go` - `MitigationTable`, exported as the one builder of the table `applyMitigation` installs, for the fast path's stock-ruleset test (Decision, review round 4).
- `internal/component/doctor/register.go` - registers the `daemon-capabilities` check.
- `cmd/ze/hub/main.go` - `warnDaemonPrivileges`: the start warning goes through the hub logger with `doctor-daemon-capability-missing`.
- `internal/core/diagnostic/codes.go` (shared) - `doctor-ipsec-esp-form-fast-path`, `doctor-ipsec-esp-form-link`, `doctor-ipsec-esp-form-tun`, `doctor-ipsec-esp-form-firewall`, `doctor-ipsec-esp-form-reader-stopped`, `doctor-ipsec-child-install-refused`, `doctor-daemon-capability-missing`, each with its entry.
- `internal/core/privilege/check_linux.go` - `CheckPrivileges` reads `DaemonCapabilities`.
- `internal/plugins/systemd/unit.go` - the unit grants `DaemonCapabilities`, `CAP_BPF` included.
- `internal/plugins/trafficusage/program_linux.go` - `TCX_NEXT` (`tcxNext`) on every path, and the L4 port copied with `bpf_skb_load_bytes` at a scalar offset (AC-18, AC-12).
- `internal/plugins/trafficusage/doctor.go` - the eBPF fix text names the older unit.
- `internal/test/runner/caps.go` (shared), `record_parse_option.go` - the netns grant derives from `DaemonCapabilities`.
- `internal/test/runner/devices.go` - the clean-tree container's device list names `/dev/net/tun`.
- `internal/le/interoplab/ipsec/ipsec.go` - the ze peer holds `/dev/net/tun` and runs under the net-sysctl profile with `/proc/sys` writable; `checkers.go` registers `esp-form-firewall-drop`; `dns.go` comment.
- `internal/le/interoplab/topology/register_home_lab.go` - the home-lab CPE holds `/dev/net/tun` beside `/dev/ppp`.
- `internal/le/test/qemu/netns.go`, `netns_linux.go` - `netnsDevNodes`, `netnsDevNodesScript`: `/dev/net/tun` widened for the dropped per-test daemon beside `/dev/ppp`.

Not this spec's, though `git status` lists them beside it: `internal/core/diagnostic/codes_plugin_infra.go` and `codes_test.go` (`doctor-config-reload-refused`), `internal/le/interoplab/bgp/check_special.go`, `check_ldp_backoff_clear.go` and `test/interop/scenarios/ldp-backoff-clear-frr/` (LDP), `test/parse/ipsec-icmp-type-selector-refused.ci` (the RFC 4301 ICMP work).

Tests:
- `internal/component/ike/dataplane/espform_netns_integration_linux_test.go`, `espform_fastpath_unit_caps_integration_linux_test.go` - actual nft and link lifecycle under exact daemon capabilities, explicit descriptor-zero and fail-closed target controls (AC-24).
- `internal/component/ike/engine/doctor_esp_form_permission_test.go` - permission-diagnosis uncertainty with the existing finding contract intact (AC-25).
- `internal/component/ike/engine/rfc4301_state_scope_readback_integration_linux_test.go` - serialize the negative capability-write request's fixed header before its attribute, preserving every assertion (AC-22).
- `internal/component/ike/engine/raw_socket_fragment_policy_integration_linux_test.go` - ordinary captured raw-sender policy, route and lifetime preservation regressions (AC-29); both former IPv4 failures pass on d87, with the full three-test population still owed there.
- `internal/component/ike/engine/fragment_sa_lifetime_integration_linux_test.go` - ordinary SA replacement/queued-identity regression and additive IPv6 queue-counter test under existing AC-4 queue lifetime; see current checkpoint for observed IPv4 PASS, IPv6 witness blocker and source-only counter test; no new RFC claim.
- `internal/component/ike/dataplane/espform_linux_test.go`, `espform_run_linux_test.go`, `espform_test.go`, `recording_shipped_test.go`.
- `internal/component/ike/dataplane/encap_formchange_integration_linux_test.go`, `encap_dualform_integration_linux_test.go`, `encap_hybrid_integration_linux_test.go`, `xfrm_migrate_integration_linux_test.go`, `xfrm_migrate_linux_test.go`, `xfrm_teardown_integration_linux_test.go` - routed addresses instead of loopback, the IKE bypass, the TOS argument; with `xfrm_state_del_failure_linux_test.go`, the registry read by SPI and local address (review rounds 9 and 10).
- `internal/component/ike/dataplane/xfrm_linux_test.go` - `TestXFRMInstallSARefusesIPv6DualFormSA`.
- `internal/plugins/systemd/unit_test.go`, `internal/test/runner/caps_netns_test.go`, `internal/test/runner/caps_option_test.go`, `test/ui/service-unit-gen.ci`.
- `internal/plugins/trafficusage/program_test.go`, `doctor_test.go`.
- `internal/component/gtsm/gtsm_test.go` - `assertListenDrop` expects the reordered matches (judged change); `internal/plugins/ddos/local/match_test.go` - `TestBuildDropTermPutsTheProtocolFirst`.
- `internal/le/interoplab/ipsec/privilege_test.go`, `internal/le/verify/evidence/evidence_test.go`, `internal/le/test/qemu/netns_alltests_test.go`.

Docs and records:
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the bounded owner handoff for a childless ESP-group reload (AC-21), and the shared plugin CLI runner (AC-20).
- `docs/guide/ipsec.md` (shared), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (shared), `docs/architecture/linux-limitations.md` (shared) - the two policy-checked paths, ECN, IPv6 as a gap.
- `docs/architecture/system-architecture.md`, `docs/guide/operations.md`, `docs/guide/ze-install.md`, `docs/guide/command-reference.md` - the daemon's capability set.
- `docs/guide/traffic-usage.md`, `docs/architecture/traffic/traffic-usage.md` - the `CAP_BPF` fix text.
- `docs/architecture/testing/qemu-integration.md`, `docs/functional-tests.md` (shared) - the runner's grant derived from `DaemonCapabilities`.
- `docs/guide/firewall.md` - a firewall rule on `protocol esp` keeps the fast path off; `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - the commit observers.
- `docs/architecture/doctor-and-health-checks.md` (shared, with the `doctor-config-reload-refused` and LDP work) - the engine's runtime checks, the TUN record, the child-install and daemon-capabilities checks.
- `docs/architecture/testing/interop.md` (shared, with the `ldp-backoff-clear-frr` work) - `esp-form-firewall-drop`, and the TUN and net-sysctl grants of the IPsec ze peer.
- `rfc/short/rfc4301.md` (shared) - `RFC4301-5.2-2`.
- `plan/journal/kernel-refuses-what-the-installer-sends.md` - the two 2026-11-12 rows this spec owns.
- `plan/post-release/features/spec-ike-ipv6-transport.md` - receives AC-8.

## Files to Create
- `internal/component/ike/dataplane/espform_netns_linux.go` - shared pinned namespace/socket handling; current-target recognition, explicit descriptor-zero identity and refusal without fallback (AC-24).
- `internal/component/ike/dataplane/espform_netns_integration_linux_test.go`, `espform_fastpath_unit_caps_integration_linux_test.go` - seven actual nft target/capability cases and real start/follow/stop under the exact unit grant (AC-24).
- `internal/component/ike/engine/doctor_esp_form_permission_test.go` - retain permission-cause uncertainty and the warning/restart/log contract (AC-25).
- `internal/le/interoplab/ipsec/peerform/` `main.go`, `action.go`, `migration.go`, `replay.go`, `frame.go`, `socket_linux.go`, `socket_other.go`, `probe_linux.go` and their portable/Linux test files - peer-local AC-2 stimulus and independent wire/reply evidence.
- `internal/le/interoplab/ipsec/peerform_wiring_test.go`, `espform_switch_test.go` - image ownership/staging proof and planned same-SA form-switch checker regressions (AC-2).
- `internal/le/interoplab/ipsec/espform_change_snapshot_test.go` - missing-state regressions and populated-state control for the interop checker (AC-23).
- `internal/component/ike/engine/reconcile_childless_handoff_test.go` - channel-ordered regression for attached childless SA reload before ownership publication (AC-21).
- `internal/component/ike/dataplane/espform_program_linux.go` - the rewrite program in `asm.Instructions`.
- `internal/component/ike/dataplane/espform_fastpath_linux.go` - load, maps, TCX attach per link, link subscription.
- `internal/component/ike/dataplane/espform_tun_linux.go` - the `ze-espform` TUN, its sysctls and its 192.0.0.8/32 address.
- `internal/component/ike/dataplane/espform_open_linux.go` - host-caused open refusals, the raw reader's stop, and its revive worker with receiver-level backoff.
- `internal/component/ike/dataplane/espform_state.go`, `espform_backend_linux.go`, `espform_backend_other.go` - `ESPFormFastPathState` and its reporter; the doctor codes are read from `internal/core/diagnostic` directly, with no alias in the dataplane.
- `internal/component/ike/dataplane/espform_fastpath_gate_linux.go` - the firewall gate's manager: epoch, holds, judgment, publish, the watch and its restart (AC-13).
- `internal/component/ike/dataplane/espform_firewall_linux.go` - `espFormFirewallBlocks`, the walk of the kernel ruleset (AC-13).
- `internal/component/ike/dataplane/espform_firewall_kernel_linux.go` - `espFormKernelReader` with its own GETRULE decode, and `espFormNFTWatch` (AC-13).
- `internal/component/ike/dataplane/espform_firewall_hold.go` - `ESPFormFirewallHolder`, all platforms.
- `internal/component/ike/dataplane/espform_tun_probe_linux.go`, `espform_tun_probe_other.go` - `ESPFormTUNAvailable`, the probe the apply runs (AC-14).
- `internal/component/ike/engine/doctor_esp_form_fast_path.go` - the doctor check (AC-10, AC-14), and `espFormOwnedTableAccept`, the fix for a blocking site in a `ze_` table (`espFormConfigSite`, `espFormConfigName`).
- `internal/component/ike/engine/register_esp_form_table.go` - registers `espFormOwnedTableAccept` with `dataplane.RegisterESPFormOwnedTableAccept` from `init()`.
- `internal/component/ike/engine/doctor_runtime.go` - `engineDoctorChecks`, the engine's runtime checks declared at Stage 1 and answered in its own process.
- `internal/component/ike/engine/doctor_child_install.go` - `ipsec-child-install`, `recordChildRefused` (AC-16).
- `internal/component/ike/engine/register_firewall_observer.go` - `holdESPFormFastPathForCommit`, registered from `init()` (AC-13).
- `internal/component/ike/engine/register_esp_form_reader.go` - the reader-stop event code and its bounded plugin report-bus sink, registered by `runEngine`.
- `internal/component/ike/dataplane/espform_refusal_linux_test.go`, `espform_reopen_linux_test.go` - open-refusal fixes, migration cause rendering, reader recovery, retry backoff, closed-receiver refusals and hold-count guards.
- `internal/component/ike/engine/doctor_esp_form_complete_path_test.go`, `register_esp_form_reader_test.go` - recorded open-failure findings, duplicate-cause handling and reader-stop events.
- `internal/component/firewall/commit_observer.go` - `CommitObserver`, `RegisterCommitObserver`, `applyObserved` (AC-13).
- `internal/component/doctor/checks_daemon_capabilities.go` - the `daemon-capabilities` runtime check (AC-17).
- `internal/core/privilege/capabilities.go` - `DaemonCapabilities`, `DaemonCapabilitiesFix`, `DaemonCapabilitiesAdvice`.
- `internal/le/interoplab/tundevice.go` - `TUNDevice`, `TUNDeviceArguments`.
- `internal/le/interoplab/ipsec/espformfirewall.go` - `checkESPFormFirewallDrop`, the `esp-form-firewall-drop` checker.
- `test/interop-ipsec/scenarios/esp-form-firewall-drop/ze.conf`, `swanctl.conf` - the scenario (AC-13).
- Tests: `internal/component/ike/dataplane/espform_program_integration_linux_test.go`, `espform_fastpath_linux_test.go`, `espform_fastpath_integration_linux_test.go`, `espform_fastpath_gate_linux_test.go`, `espform_firewall_linux_test.go`, `espform_firewall_stock_linux_test.go`, `espform_tun_open_linux_test.go`, `espform_tun_probe_linux_test.go`, `espform_tun_sysctl_linux_test.go`, `unit_capabilities_integration_linux_test.go`; `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go`, `doctor_esp_form_fast_path_test.go`, `doctor_child_install_test.go`, `child_install_refusal_test.go`, `register_firewall_observer_test.go`; `internal/component/firewall/commit_observer_test.go`; `internal/plugins/ddos/local/mitigation_drop_integration_linux_test.go`; `internal/component/doctor/checks_daemon_capabilities_test.go`; `internal/core/privilege/capabilities_linux_test.go`; `internal/plugins/trafficusage/unit_capabilities_integration_linux_test.go`, `tcx_next_integration_linux_test.go`; `internal/le/interoplab/ipsec/espformfirewall_test.go`.
- `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci` - AC-10, promoted to `test/ipsec/` after its runs.
- `rfc/full/rfc7600.txt`, `rfc/full/rfc768.txt` - the texts the TUN address and the UDP header comments quote.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | none |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | BPF, TCX, TUN and the nftables ruleset: `ipsec-esp-form-fast-path` (`ike/engine/doctor_esp_form_fast_path.go`, runtime phase, answered in the engine's process through `doctor_runtime.go`; AC-10, AC-14); the daemon's capability set: `daemon-capabilities` (`internal/component/doctor/checks_daemon_capabilities.go`, runtime phase; AC-17); the TUN device is also tried at every apply that configures a peer (`recordESPFormTUN`, `ike/engine/apply.go`), and the check reads that record |
| Doctor code per problem class | Yes | `doctor-ipsec-esp-form-fast-path` (the fast path did not load or stopped following links), `doctor-ipsec-esp-form-link` (one link not served), `doctor-ipsec-esp-form-tun` (no usable `ze-espform`, so Child SAs on UDP 4500 are refused; also on the apply's Warn line), `doctor-ipsec-esp-form-firewall` (the gate is shut by a rule that could drop ESP, or by a ruleset Ze cannot read or watch), all four declared by `ipsec-esp-form-fast-path`; `doctor-ipsec-esp-form-reader-stopped` is a registered event code in `register_esp_form_reader.go`, raised through the plugin report bus and read by the recent-events doctor check; `doctor-ipsec-child-install-refused` (`ipsec-child-install`, `ike/engine/doctor_child_install.go`, runtime phase; the refusal's Warn lines in `established.go` and `create_child.go` carry it; AC-16); `doctor-daemon-capability-missing` (`daemon-capabilities`; the start warning `warnDaemonPrivileges` in `cmd/ze/hub/main.go` carries it; AC-17) |
| Prometheus counters/metrics | No | the program's rewrite counter is read by the doctor check and tests only |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | no feature level changes |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` ("Both ESP wire forms": the two paths, ECN, IPv6 as a gap); `docs/guide/operations.md`, `docs/guide/ze-install.md`, `docs/guide/command-reference.md` (the unit's `CAP_BPF`); `docs/guide/traffic-usage.md` (the fix text); `docs/guide/ipsec.md` also carries the firewall gate and its four limits; `docs/guide/firewall.md` (a rule on `protocol esp` keeps the fast path off) |
| 7 | Wire format changed? | No | nothing leaves the host |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` `RFC4301-5.2-2` (`{partial}` reason names the remaining scope-installation producer; remove it only after the full obligation is implemented and proven, AC-4); `RFC7296-2.24-2` in `rfc/short/rfc7296.md` carries no annotation and needs no edit, and its new tag's native record is observed |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/qemu-integration.md`: the runner's netns grant derives from `privilege.DaemonCapabilities`, and `/dev/net/tun` is widened beside `/dev/ppp`; `docs/architecture/testing/interop.md`: `esp-form-firewall-drop` and the IPsec ze peer's TUN and net-sysctl grants |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/linux-limitations.md`, `docs/architecture/system-architecture.md` (the daemon's capability set), `docs/architecture/traffic/traffic-usage.md` (`TCX_NEXT`); `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (the commit observers); `docs/architecture/doctor-and-health-checks.md` (the engine's runtime checks, the TUN record, `ipsec-child-install`, `daemon-capabilities`); `docs/architecture/core-design.md`, "Firewall reconcile concurrency" (the observers inside `reconcileMu`, owed: row 16) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `doctor-ipsec-esp-form-reader-stopped` is registered as an event code in `ike/engine/register_esp_form_reader.go`; `docs/guide/ipsec.md` and `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` describe its report-bus route and recovery |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md` lists them (the main thread runs it). Declared by `// Design:` headers: `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (`espform_state.go`, `espform_backend_linux.go`, `espform_program_linux.go`, `espform_fastpath_linux.go`, `doctor_esp_form_fast_path.go`, `override_guard_linux.go`), updated; `plan/release/validation/spec-ipsec-esp-dual-form-receive.md` (`espform_linux.go`, `espform.go`), a record of the earlier work, unaffected because a plan record is never repointed; `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md` (`espform_tun_linux.go`), this spec; `docs/architecture/system-architecture.md` (`internal/core/privilege/capabilities.go`), updated; `docs/architecture/cli/plugin-modes.md` (`internal/plugins/systemd/unit.go`), unaffected because the unit's shape is unchanged and only its capability list grew; `docs/architecture/traffic/traffic-usage.md` (`internal/plugins/trafficusage/attach_linux.go`, `doctor.go`), updated; `docs/architecture/ike/ipsec-7-ikev2-engine.md` (`internal/component/ike/engine/register.go`), unaffected because it lists no doctor check of the engine's registration; `docs/architecture/testing/ci-format.md` (`internal/test/runner/caps.go`, `record_parse_option.go`), unaffected because it documents the `caps=` tokens and their gate, not the netns grant's members, which `docs/functional-tests.md` and `docs/architecture/testing/qemu-integration.md` carry and which are updated; `docs/features/ai-first.md` (`internal/core/diagnostic/codes.go`), unaffected because it describes the code registry, and the registered codes describe themselves through `ze show diagnostic`; `docs/architecture/core-design.md` (`internal/component/firewall/registry.go`), section "Firewall reconcile concurrency", to update: it says `reconcileMu` is held for the kernel round trip, and the commit observers now run inside it before and after `Backend.Apply`, which extends that hold by the IKE hold's cost (`commit_observer.go` says an observer MUST return quickly); `docs/architecture/hub-architecture.md` (`cmd/ze/hub/main.go`), unaffected because it documents the plugin model and the hub CLI entry, not the start-time capability warning, which `docs/architecture/system-architecture.md` carries |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring and the negative test** -- write `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked` (IPv4 whole datagram, out-of-selector row) and show it red on the current path.
2. **Phase: complete path** -- TUN writer replaces the `lo` injector; IPv4 header checksum; rate bound kept. Test: fragment rows of AC-3 green, existing dual-form tests green.
3. **Phase: fast path IPv4** -- program, map, TCX attach per link, link subscription, registry hooks. Tests: AC-6 IPv4, AC-7, AC-3 whole-datagram rows.
4. **Phase: IPv6** -- the program variant only, built and tested (AC-6 IPv6 rows); `InstallSA` and `MigrateTunnel` refuse an IPv6 dual-form SA with an error naming the gap (AC-8). The reader, the builder and the receive side moved to `plan/post-release/features/spec-ike-ipv6-transport.md`, because Ze has no IPv6 IKE socket.
5. **Phase: doctor** -- AC-10; the unit's capability set from `DaemonCapabilities`, `CAP_BPF` included (AC-12).
6. **Phase: ECN** -- the complete path copies the received outer TOS (AC-11).
7. **Phase: docs and RFC** -- AC-4, AC-5; interop AC-1, AC-2 with red proofs.
8. **Phase: firewall gate** -- the `ze_esp_gate` map and the program's lookup, the kernel-ruleset analyzer and its GETRULE decode, the watch and its restart, the firewall commit observer and the engine's hold (AC-13); interop `esp-form-firewall-drop` with its red proof.
9. **Phase: doctor at runtime** -- the engine's checks declared at Stage 1 and answered in its process; `recordESPFormTUN` at apply; `doctor-ipsec-esp-form-tun`, `doctor-ipsec-esp-form-firewall`, `ipsec-child-install`, `daemon-capabilities` (AC-14, AC-16, AC-17).
10. **Phase: Child SA refusal backoff** -- the install, binding and build refusals keep the IKE SA on `childCreateSchedule` (AC-15).
11. **Phase: trafficusage** -- `TCX_NEXT`, the scalar-offset port read, the rewritten `TestProgram_TruncatedPass` (AC-18, AC-12).
12. **Phase: lab grants** -- `/dev/net/tun` and the net-sysctl profile for the IPsec interop ze peer, `/dev/net/tun` for the home-lab CPE, the clean-tree container and the QEMU per-test namespace.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | a datagram is either rewritten or re-presented, once; the program leaves byte-identical every packet it does not rewrite; it never rewrites a fragment |
| Data flow | every bare ESP datagram for a watched SPI reaches XFRM on a device with `disable_policy=0` |
| Rule: no-layering | `rawESPFormInjector` is gone and the complete path writes only to `ze-espform` |
| Rule: goroutine-lifecycle | the link subscription is one long-lived worker, stopped on last forget and on close |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `lo` injector removed | `grep -n IPPROTO_RAW internal/component/ike/dataplane/espform_linux.go` prints nothing |
| fast path program | `TestESPFormRewriteProgram` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the program bounds every read through `bpf_skb_load_bytes`; source and destination must match the target before any rewrite |
| Resource exhaustion | the complete path stays rate-bounded; the fast-path map has a fixed size, and a full map leaves the unserved SA on the complete path with a diagnostic |
| Authorization that could fail open | a failed fast path leaves the complete path, never the unchecked one |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- On a kernel with `esp4_offload` loaded, `esp4_gro_receive` (`net/ipv4/esp4_offload.c`) consumes bare ESP at GRO, before tc ingress and before raw sockets, and XFRM refuses it as a form mismatch, so neither path receives it. Ze's kernel does not build `CONFIG_INET_ESP_OFFLOAD`. Recorded as a journal row owned here (Known Limitations).
- A TUN with no IPv4 address loses every datagram written into it to loose reverse-path filtering: `__fib_validate_source` (`net/ipv4/fib_frontend.c`, v7.0) reads `no_addr = idev->ifa_list == NULL;` and sends an address-less device to `last_resort`, where `if (rpf)` drops it. `ze-espform` therefore carries 192.0.0.8/32 scope host, RFC 7600's dummy address, which Linux already uses as `INADDR_DUMMY`.
- The complete path depends on the inbound IKE bypass: `udp_queue_rcv_one_skb` (`net/ipv4/udp.c`, v7.0) runs `xfrm4_policy_check` before `encap_rcv`, so on a device with `disable_policy` 0 the Child SA's policy drops the re-presented UDP datagram unless the BYPASS for UDP 4500 ranks ahead of it. `lo` hid that check. The kernel tests install the production SPD (`installUnmatched`, then `installIKEBypass`). With exact inbound state binding, an out-of-selector packet is refused by its actual SA before SPD lookup (`XfrmInStateMismatch`). A host-specific explicit DISCARD inside the accepted state scope separately proves SPD consultation (`XfrmInPolBlock`), and its exact removal restores valid delivery. Complete-path rows also count the original bare-form mismatch; fast-path rows do not. AC-3 records the exact deltas.
- Which process runs the PostConfig doctor checks (doctor.Answer dispatch) decides whether ipsec-esp-form-fast-path and doctor-traffic-usage-ebpf measure the daemon's capabilities or the CLI's: settled and fixed in this spec.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `committed-tree-does-not-build.md` | 2026-11-13 | IKE recording and netlink constructors key promoted fields; source-only, unexecuted |
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-11-10 | `espFormReceiver` re-presents bare ESP through lo, so no inbound SPD check runs |
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-12 | `esp4_offload` GRO consumes bare ESP before tc ingress on a distro kernel |
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-12 | the systemd unit granted no `CAP_BPF`, so the kernel refused every BPF load Ze makes under it |
| `two-owners-share-one-name.md` | 2026-11-13 | concurrent kernel workers remove the same live extracted source tree on a shared volume |

### State of the owned occurrences before closure

- The loopback SPD bypass has its replacement paths written; the final
  guest-root selector tests, RFC records and interop proofs still decide
  whether its occurrence can be closed.
- The missing-`CAP_BPF` producer is changed: `DaemonCapabilities` includes the
  bit, `writeCapabilities` writes the unit's set from that declaration, and
  `checkDaemonCapabilities` reports the daemon's missing grant. The final
  guest-root tests must still prove the ESP rewrite, override guard and
  traffic-usage programs load under exactly that set. A written grant is not
  that proof.
- The `esp4_offload` GRO occurrence remains owned here. Its diagnostic and
  documentation repair landed in `bd8e21b487`: `answerESPFormOffload` reads the
  active receiver and `kernelcap.LoadedModules`, warns for loaded IPv4 ESP
  offload, and distinguishes an unreadable snapshot from module absence. Its
  fix stops Ze, unloads `esp4_offload`, then starts Ze; the finding also names
  the GRO-disable alternative and the built-in-feature detection limit.
  The named offload doctor units passed in
  `job-ipsec-binding-doctor-units-419db7fc.log`. Final functional stress and
  guest proof still remain owed; this is not a closure verdict.
- The concurrent kernel-worker source-tree collision was fixed in
  `14bec40b17`. The named concurrent-worker regression and scoped race package
  passed (`job-ipsec-kernel-workspace-green-9df4b9bb.log` and
  `job-ipsec-kernel-workspace-package-81a675f5.log`); the optional Docker
  cancellation proof was skipped because `KERNEL_CONTAINER_PROOF_IMAGE` was
  unset. The committed-tree check compiled all five configurations.

## Test changes judged

The rows below preserve preparation before checkpoint `abeb7d6535`. That commit
published the selected independent approval trailers, including the corrected
`TCX_NEXT` verdict reason; its historical references to an approval still owed
are not the current queue. Further test corrections during final validation
still require their own independent verdict before editing, and current
approvals remain in the native untracked approval files rather than this spec.

A change that could lower what a test proves is judged by an independent agent running `ze-test-judge` (`ai/META-DECISIONS.md`, "Test changes"). Its approval is recorded through `./le test weakened approve`, and the commit that carries the change carries the trailer.

| Unit | Change | Verdict | Trailer |
|------|--------|---------|---------|
| `dataplane.driveESPFormRun` (`espform_run_linux_test.go`) | delegates to `driveESPFormRunConn`, which scripts the `ReadMsgIP` seam the reader now reads through, with a fixed peer source | approved | `Weakening-approved: dataplane.driveESPFormRun` |
| `dataplane.encapInjectBare` (`encap_dualform_integration_linux_test.go`) | delegates to `encapInjectBareTo` with the loopback destination it used to fix | approved | `Weakening-approved: dataplane.encapInjectBare` |
| `dataplane.TestEncapEstablishedSAServesAPeerFormChange` (`encap_formchange_integration_linux_test.go`) | runs on routed addresses instead of loopback, because a loopback address is martian on `ze-espform`, and installs the inbound IKE bypass as production does | approved | `Weakening-approved: dataplane.TestEncapEstablishedSAServesAPeerFormChange` |
| `fakeESPFormConn` (`espform_run_linux_test.go`) | removes six `net.PacketConn` methods `run` no longer calls (`ReadFrom`, `WriteTo`, `LocalAddr`, `SetDeadline`, `SetReadDeadline`, `SetWriteDeadline`; `Close` stays): the reader goes through `espFormPacketReader.ReadMsgIP` alone | approved, one unit per method | the six `Weakening-approved:` lines below |
| `dataplane.TestESPFormReceiverHoldsTUNOnlyWhileWatching` (`espform_linux_test.go`) | removes the IPv6 sysctl assertions (`disable_ipv6` 0, IPv6 `disable_policy` 0): `espFormTUNSysctls` writes the IPv4 settings only, because no IPv6 dual-form SA exists (AC-8) | approved (b) | none owed: the unit is not in HEAD (its approval runs from `none`), so no commit removes an assertion HEAD holds |
| `runner.TestContainerDevicesCoverWhatPPPOpens` (`internal/test/runner/caps_option_test.go`) | renamed `TestContainerDevicesCoverWhatTestedPathsOpen`; keeps every old check and adds `internal/component/ike/dataplane` to the walk, which reaches `openESPFormTUN`'s open of `/dev/net/tun` | approved (a) | the `Weakening-approved:` line below |
| `internal/le/interoplab/ipsec/privilege_test.go` | ze's lab peer gets `/dev/net/tun` and `/proc/sys` writable under the generic net sysctl profile | approved (b) | the `Test-judge:` line below |
| `internal/le/verify/evidence/evidence_test.go` | the clean-tree container's expected devices add `/dev/net/tun` | approved (b) | the `Test-judge:` line below |
| `internal/plugins/systemd/unit_test.go` | `TestUnitFileCapabilities` expects `CAP_BPF` and the line end on both capability lines | approved (a) | the `Test-judge:` line below |
| `test/ui/service-unit-gen.ci` | both capability expectations add `CAP_BPF` | approved (a) | the `Test-judge:` line below |
| `trafficusage.TestProgram_TruncatedPass` (`internal/plugins/trafficusage/program_test.go`) | the frames shorter than the Ethernet and IPv4 headers, which `BPF_PROG_TEST_RUN` refuses on Ze's kernel so the old checks never ran, are sent through a TCX-attached veth pair instead | approved (a), but its recorded reason is now false and the approval MUST be re-recorded by a fresh `ze-test-judge` run before the commit: it says the exact verdict is structural because the one `Return` sits "at ret0 after Mov R0 0", and `ret0` now moves `tcxNext` (-1) into R0 (`internal/plugins/trafficusage/program_linux.go`, the `ret0` symbol) | the `Weakening-approved:` line below, replaced by the re-recorded one |
| `trafficusage.runProg` (`program_test.go`) | wants `TCX_NEXT` (-1, read back from the u32 retval through `tcxVerdict`) instead of 0 | approved (b) | the `Weakening-approved:` line below |
| `dataplane.espform_fastpath_gate_linux_test` (`internal/component/ike/dataplane/espform_fastpath_gate_linux_test.go`) | adds `TestESPFormFastPathGateFollowsTheKernelRuleset`, whose one skip is the package's root guard | approved (a); this row is removed once that test is written, because the file is not in HEAD and no commit owes its trailer | none owed: the file is not in HEAD |
| `internal/component/ike/engine/doctor_esp_form_fast_path_test.go`, `doctor_child_install_test.go`, `internal/component/ike/dataplane/espform_program_integration_linux_test.go` | new files | no record owed | none: not in HEAD, so no commit removes an assertion HEAD holds |
| `internal/component/ike/dataplane/espform_test.go` | adds tests, and passes the new `tos` argument in two `writeESPForm` calls | no record owed | none: the file is in HEAD, and its diff removes only those two call lines, no assertion |
| `gtsm.assertListenDrop` (`internal/component/gtsm/gtsm_test.go`) | the same four matches, the drop alone and the exact comparison over the same four callers' inputs, reordered to match `listenPortTerm`, protocol first | approved (a), approval file row 18 | the `Weakening-approved:` line `./le commit create` adds from row 18 |
| `dataplane.TestXFRMMigrationRetargetsBothFormReceiver` (`xfrm_migrate_linux_test.go`) | keeps every output assertion, adds that retarget succeeds and that a datagram to the pre-move local address is not served; the fixture sends to the SA's address after the move | approved (a) and (b), row 19 | the line commit create adds from row 19 |
| `dataplane.TestESPFormRegistryWatchesOnlyTemplatedSPIs` (`espform_test.go`) | each not-watched check moves from the SPI-only `target(spi)` to `watching(spi)`; the watched check uses `target(spi, local)`; `forget` names the local address | approved (a), row 20 | the line commit create adds from row 20 |
| `dataplane.TestESPFormReceiverHoldsSocketsOnlyWhileWatching` (`espform_linux_test.go`) | `Forget` gains the local address the SA was watched on | approved (a), row 21 | the line commit create adds from row 21 |
| `dataplane.TestESPFormReceiverNilIsFailClosed` (`espform_linux_test.go`) | `Forget` gains a local address argument | approved (a), row 22 | the line commit create adds from row 22 |
| `dataplane.TestESPFormReceiverCloseWhileWatching` (`espform_linux_test.go`) | the not-watched check moves from `target(spi)` to `watching(spi)` | approved (a), row 23 | the line commit create adds from row 23 |
| `dataplane.TestXFRMMigrationFailedRollbackDiscardsStates` (`xfrm_migrate_integration_linux_test.go`) | the stale-entry check moves from `target(spi)` to `watching(spi)` | approved (a), row 24 | the line commit create adds from row 24 |
| `dataplane.TestRemoveSAForgetsESPFormWhenStateDeleteFails` (`xfrm_state_del_failure_linux_test.go`) | the still-watched check moves from `target(spi)` to `watching(spi)` | approved (a), row 25 | the line commit create adds from row 25 |
| `dataplane.TestXFRMDoubleTeardownLeavesNothing` (`xfrm_teardown_integration_linux_test.go`) | the still-watched check moves from `target(spi)` to `watching(spi)`, and the test adds that `ze-espform` is gone and the fast path is Idle | approved (a), row 26 | the line commit create adds from row 26 |
| `dataplane.TestXFRMMigrationRekeyReceiverUsesMappedPorts` (`xfrm_migrate_integration_linux_test.go`) | the fixture moves from loopback to `encapRoutedNetns`, because a loopback source is martian on `ze-espform`; every assertion unchanged | approved (a), row 27 | the line commit create adds from row 27 |
| `dataplane.TestESPFormRunRepresentsWatchedSPI` (`espform_run_linux_test.go`) | the fake socket delivers each datagram behind an IPv4 header to the SA's local address, as `ReadMsgIP` does; every assertion kept | approved (a), row 28 | the line commit create adds from row 28 |
| `dataplane.TestESPFormRunIgnoresUnwatchedTraffic` (`espform_run_linux_test.go`) | the datagrams are wrapped in an IPv4 header to the watched SA's own address from its own peer | approved (a), row 29 | the line commit create adds from row 29 |
| `dataplane.TestWriteESPFormBuildsEncapsulatedDatagram` (`espform_test.go`) | `writeESPForm` gains a TOS argument passed as 0, and the test adds a header-checksum assertion | approved (a), row 30 | the line commit create adds from row 30 |
| `dataplane.TestWriteESPFormRefusesUnusableInput` (`espform_test.go`) | `writeESPForm` gains a TOS argument passed as 0 | approved (a), row 31 | the line commit create adds from row 31 |

Rows 18 to 31 name the line numbers of `tmp/commit-weakening-approved-79a29bf3.md`, whose reasons `./le commit create` copies into the trailers.

Trailers the commit that carries these changes owes, the `Weakening-approved:` reasons copied from the recorded approvals (`tmp/commit-weakening-approved-79a29bf3.md`). The `trafficusage.TestProgram_TruncatedPass` line is replaced by its re-recorded approval before the commit:

```
Weakening-approved: dataplane.ReadFrom: APPROVED (a): run's seam espFormPacketReader (espform_linux.go) declares only ReadMsgIP, so the fake's ReadFrom had no caller and no assertion; fakeESPFormConn.ReadMsgIP hands run the same scripted datagram (behind the IPv4 header ReadMsgIP leaves in place) and the same source, and every test drives the same datagrams through run and checks the same results.
Weakening-approved: dataplane.WriteTo: APPROVED (a): run's seam espFormPacketReader declares only ReadMsgIP, every fakeESPFormConn (driveESPFormRun, driveESPFormRunConn callers, xfrm_migrate_linux_test.go) is handed only to r.run, and the seven net.PacketConn methods hold no assertion and have no caller, so every test drives the same datagrams through the same ReadMsgIP and checks the same results.
Weakening-approved: dataplane.LocalAddr: APPROVED (a): run's seam espFormPacketReader declares only ReadMsgIP, every fakeESPFormConn (driveESPFormRun, driveESPFormRunConn callers, xfrm_migrate_linux_test.go) is handed only to r.run, and the seven net.PacketConn methods hold no assertion and have no caller, so every test drives the same datagrams through the same ReadMsgIP and checks the same results.
Weakening-approved: dataplane.SetDeadline: APPROVED (a): run's seam espFormPacketReader declares only ReadMsgIP, every fakeESPFormConn (driveESPFormRun, driveESPFormRunConn callers, xfrm_migrate_linux_test.go) is handed only to r.run, and the seven net.PacketConn methods hold no assertion and have no caller, so every test drives the same datagrams through the same ReadMsgIP and checks the same results.
Weakening-approved: dataplane.SetReadDeadline: APPROVED (a): run's seam espFormPacketReader declares only ReadMsgIP, every fakeESPFormConn (driveESPFormRun, driveESPFormRunConn callers, xfrm_migrate_linux_test.go) is handed only to r.run, and the seven net.PacketConn methods hold no assertion and have no caller, so every test drives the same datagrams through the same ReadMsgIP and checks the same results.
Weakening-approved: dataplane.SetWriteDeadline: APPROVED (a): run's seam espFormPacketReader declares only ReadMsgIP, every fakeESPFormConn (driveESPFormRun, driveESPFormRunConn callers, xfrm_migrate_linux_test.go) is handed only to r.run, and the seven net.PacketConn methods hold no assertion and have no caller, so every test drives the same datagrams through the same ReadMsgIP and checks the same results.
Weakening-approved: runner.TestContainerDevicesCoverWhatPPPOpens: APPROVED (a): renamed TestContainerDevicesCoverWhatTestedPathsOpen keeps every old check (the walk of internal/component/l2tp must find at least one /dev open, ContainerDevicePaths must equal the opened set exactly, DeviceHostModule must name a module for each node) and adds the same at-least-one requirement for internal/component/ike/dataplane, while the matcher only widens (Open/OpenFile in any case, literal or same-package string constant, which reaches openESPFormTUN's ops.open(espFormTUNClone) in espform_tun_linux.go).
Test-judge: internal/le/interoplab/ipsec/privilege_test.go: APPROVED (b): the old expectation that ze's peer gets neither /dev/net/tun nor a writable /proc/sys is wrong against the producer's contract, because openESPFormTUN opens /dev/net/tun and writes the ze-espform sysctls for any UDP-encapsulated Child SA, which a scenario decides at run time; the new expectation grants exactly those two and every other peer's assertion is unchanged.
Test-judge: internal/le/verify/evidence/evidence_test.go: APPROVED (b): the expected device list lacked /dev/net/tun, which the clean-tree container must now grant because ContainerDevicePaths names every /dev node a tested path opens and internal/component/ike/dataplane opens it (openESPFormTUN); the new list adds only that device and every other expected grant is unchanged.
Test-judge: internal/plugins/systemd/unit_test.go: APPROVED (a): both capability assertions keep the three old capabilities in the same order and add CAP_BPF, which privilege.DaemonCapabilities now declares, and the line end, so every unit the old assertions refused is still refused and a unit with a capability missing or appended is refused too.
Test-judge: test/ui/service-unit-gen.ci: APPROVED (a): both capability expectations keep the old three capabilities as their prefix and add CAP_BPF, which privilege.DaemonCapabilities now declares, so every output the old lines refused is still refused and an output without CAP_BPF is refused too.
Weakening-approved: trafficusage.TestProgram_TruncatedPass: APPROVED (a): the old test skipped on Ze's 7.2 kernel because bpf_prog_test_run_skb refuses data_size_in < ETH_HLEN and, after eth_type_trans, an ETH_P_IP frame with skb_headlen < sizeof(struct iphdr), so every 14-33 byte input it fed was refused and its ret==0 and no-map-write checks never ran; the new test drives the same 20 truncated IPv4 lengths through both programs on a TCX-attached veth pair, proves each egress frame arrives byte for byte (not dropped) with both egress maps empty and the ingress maps holding only the counted sentinel, and the exact ret==0 is structural because directionInsns has one Return, at ret0 after Mov R0 0, which every runProg test pins.
Weakening-approved: trafficusage.runProg: APPROVED (b): the old want of 0 (TCX_PASS) is wrong against the kernel the programs attach to by TCX: tcx_run stops at the first program that answers anything but TCX_NEXT, and sch_handle_ingress and sch_handle_egress skip tc_run unless the TCX verdict is TC_ACT_UNSPEC (net/core/dev.c v7.2, 4485-4490 and 4544-4549), so 0 ended the hook and switched off every clsact filter on the link, while uapi bpf.h enum tcx_action_base defines TCX_NEXT = -1; runProg still pins one exact verdict on every TEST_RUN path, now -1 read back from the u32 retval, and TestProgram_ReturnsTCXNext plus TestProgram_LetsAClsactFilterRun prove the verdict on both programs and that an ingress and an egress clsact mirror still runs.
```

The four `Test-judge:` sentences were written from the coordinator's verdict classes (privilege_test (b), evidence_test (b), unit_test (a), service-unit-gen (a)), because no recorded approval file holds them.

## Implementation deviations recorded before closure

These rows record the round-12 and round-13 implementation decisions. They do
not claim that their final-code tests or mutation proofs have run.

| Earlier shape | Final implementation | Reason and producing source |
|---------------|----------------------|-----------------------------|
| An open refusal used a generic retry or doctor fix | The refusal keeps the kernel cause and names the capability, security-policy, process-limit, host-limit, memory or log command for that cause | `espFormSocketRefusal`, `espFormBypassRefusal`, `espFormOpenFailure` and `espFormTUNStepRefusal`; before the round-12 fix, doctor had no finding for these causes |
| The receiver could refuse an install while doctor had no record of its open failure | The last failure and its time are read through `CompletePathFailure`, with duplicate TUN causes reported once | `reopenReaderLocked`, `espFormCompletePathFindings`, `espFormDeviceFindings`, `espFormSameCause` |
| A failed reader remained installed until another Watch noticed it | The reader publishes its failure and its revive worker opens the same paths again, with receiver-level backoff and a closed-receiver guard | `read`, `revive`, `reopenWaitLocked`, `reviveLocked`, `Watch`, `acquire`, `Close`; recovery remains in this spec |
| A successful reopen removed the only live state a doctor check could read | A registered reader-stop error event crosses the plugin boundary and remains in the daemon's error ring | `reportReaderStopped`, `RegisterESPFormReaderStopped`, `runEngine`, `espFormReaderStoppedSink` |
| An unmatched hold end could decrement the count, and panic during a move could skip normal release | Hold ends check their acquire obligation before changing the count; migration defers release while its hold remains live | `watchHeld`, `release`, `MigrateTunnel` |

## Verification state before closure

| Evidence | Observed state |
|----------|----------------|
| Current installed-endpoint MTU interop | `mtu-nat-installed-endpoint` PASS 4s in `job-ipsec-mtu-installed-endpoint-current-dcac5194.log`, recording `mtu-nat-installed-endpoint-4ON24PX74JNHTY3SGYHJZFDUNY.jsonl`. Native image `45cb736f9a13fd62`, check/command/lab exits zero, no interruption, VM stopped and no leftover resources |
| Main-thread `./le --name session doc check all` | This turn's direct run exited 1 and initially reported 389 documentation-drift issues; its remaining output was truncated and no new log path was retained. Earlier retained run `job-ipsec-doc-check-f119ecea.log` separately reports 389 drift issues, mostly published wiki/gh-pages CLI drift, plus 12 source-anchor findings outside IPsec paths. Neither receipt supplies an owned-docs PASS |
| Bounded lint-repair source phase | Both packages PASS in `job-ipsec-lint-repair-packages-4419bc1f.log` (native `job-unit-pkg-cf26cae2.log`): checker 12.442s, helper 1.304s. Linux rebuild PASS in `job-ipsec-peerform-lint-fix-linux-build-1732f37d.log`; all 31 top-level guest tests, including fuzz seeds, PASS without skips in `job-ipsec-peerform-lint-fix-linux-units-bad03c34.log`, on the current c3583fe5 kernel/45cb736f9a13fd62 image, all zero exits and stopped VM. Independent review CLEAR. After the final import-group correction, helper-only lint `job-ipsec-peerform-final-import-lint-25898e43.log` reports zero Linux issues and one Darwin `encapsulation` unused finding, for a field consumed by Linux. The 30-file code phase landed in `7d49a5e3f9`; proof metadata is prepared separately. Earlier parent-package findings plus the counter-test modernization note remain; no overall lint-green claim |
| AC-2 repaired helper/checker and clean live switch | Both full portable packages PASS in `job-ipsec-peerform-checker-packages-green-2d1ebed9.log`; native Linux helper PASS without skips in `job-ipsec-peerform-linux-units-dcd7f143.log`, on current kernel `c3583fe57d83e971a6c24c7d6e93d5b4e682a5797dbb0d15dad42d642f1c8798` and matching image `45cb736f9a13fd62`. Live `esp-form-change` PASS in 8s (`job-ipsec-live-form-switch-clean-de5e18d6.log`), with bare sequence 1, UDP acknowledgement 1, UDP probe sequence 2, bare acknowledgement 2 and bare probe sequence 3 on helper SPI `0x2bfcb9ba`, reqid 1. The real checker verified four counters and identities. Check/command/lab exits zero, no interruption, no resources left, VM stopped. The as-used binding fault produced its intended first-bare-reply failure in 16s, then the post-clean run passed in 8s (the AC-2 receipt records source identities); the later comment-only input remains unexecuted and AC-4 stays open |
| Fresh native clause records on the current Ze kernel | RFC4301-5.2-2 positive: clean PASS 5.54s, writer-suppression cut fails fragmented inside-selector row 3 in 4.59s (`job-ipsec-rfc4301-positive-record-6923ef2c.log`). Negative: clean PASS 5.27s, TUN policy-bypass cut delivers outside-selector fragmented traffic in row 4, FAIL 14.22s (`job-ipsec-rfc4301-negative-record-651671a3.log`). RFC7296-2.24-2 positive: clean PASS, TOS-zero cut fails with 0x02 instead of CE 0x03 in 3.21s (`job-ipsec-rfc7296-ecn-record-364024cc.log`). RFC5082-3-4 positive: clean PASS 8.91s across four subcases; source-match removal causes three Unknown-SYN failures in 19.50s while the other-port control passes (`job-ipsec-rfc5082-unknown-record-239f49a4.log`). Native records are under `rfc/discrimination/rfc4301/`, `rfc7296/` and `rfc5082/`. All use kernel SHA256 `c3583fe57d83e971a6c24c7d6e93d5b4e682a5797dbb0d15dad42d642f1c8798` and matching `45cb736f9a13fd62` image, without skips and with stopped VMs. These prove the tagged claims; AC-4 remains partial because general per-SA scope installation and isolation are unfinished |
| Main-thread `./le repo compiles check` after checkpoint `abeb7d6535` | all five committed-tree build configurations compiled; this excludes test files and is not proof of the later working-tree edits |
| Initial final-code main-thread `./le test unit pkg path ./internal/component/ike/dataplane/ path ./internal/component/ike/engine/ path ./internal/core/diagnostic/` | engine and diagnostic packages passed; three dataplane fixture/expectation failures were subsequently corrected and committed in `bd8e21b487`. The targeted package run `job-unit-pkg-58fce57a.log` and narrow race/count-20 scheduling run `job-ipsec-reopen-schedule-e00a01c8.log` passed. Final guest verification remains owed |
| Receiver doctor and binding fixture units | the named tests passed under race/verbose execution in `job-ipsec-binding-doctor-units-419db7fc.log`; this includes the offload and complete-path doctor findings |
| Host mutation walks 27, 31–33, 37, 39–49 | all sixteen returned the native red verdict, with clean own-test passes and broken own-test failures, in `job-ipsec-dualform-host-redwalks-aa95f75a.log`; guest mutant 20 remains owed |
| Remaining closure evidence | Final Linux populations, doctor qualification/promotion, repaired helper/checker, AC-2's as-used clean → binding-cut → clean proof and AC-1's seven-run local IPv4 single-pair ICMP sequence are observed. Installed-endpoint MTU also passed its named scenario in 4s. Repaired-source firewall clean 17s → intended gate-open packet failure 14s → recovery clean 17s is observed and independently CLEAR. FRR passed its own named scenario in 5s. Post-fix form-switch and ICMP clean passed in 8s and 10s, with final independent lifecycle acceptance CLEAR. No live run remains owed for the namespace phase; AC-4's scope/kernel implementation and broader RFC obligations stay open. The earlier clause records and four same-row transport/tunnel renewals were observed on their recorded source; no current freshness after the writer change is inferred. Tunnel positive `job-ipsec-sad-tunnel-positive-record-8cbbeb8e.log` and negative `job-ipsec-sad-tunnel-negative-record-3ca64234.log` each passed clean in 3.80s and failed with producer halt in 0.01s. Independent canonical-RFC review cleared the partial audit after all nine covers across six functions, and main natively stamped `rfc/audit/rfc4301/RFC4301-5.2-2.json`. The pre-full-route check `job-ipsec-rfc4301-partial-refresh-check-4fea72ad.log` reports 451 corpus findings but no audit/discrimination freshness finding for this exact ID, which appears only in the normal partial report. The 83-entry manifest keeps four historically observed renewals and 79 original unobserved renewals, including separate claims on the same units; current freshness of the four must be verified after the writer change. The selected full route now has directional engine scope source and activated K3/guard configuration; the first new profile passed full receipt, transport/scope/raw-port/PMTU populations and bounded K1 discrimination. The fourth doctor CI and complete receipt passed on 88c/imagea393. Post-style host packages, 31 Linux dataplane units and five packet-matrix tests passed; the repeated IPv6 Options failure exposed the normal C receive-cursor defect. Its reviewed repair and additive Hop-by-Hop preservation probe now have accepted normal 6a87/913df5db runtime evidence: full receipt, original IPv6 ANY/HBH and all eight reached packet tests PASS without skips and with zero guest exits. The original seven-test population also passed five times in one process, 35 top-level PASS results with no FAIL/SKIP and all exits zero in `job-ipsec-corrected-rx-stability-c25ff001.log`; packet deadlines/assertions were unchanged. The historical batch receive loss remains unlocalized, with no cause or fix claim. AC-27's bounded nine-path source cleanup landed in `a62375a383`. AC-29's authored/reviewed source passed formatting/build, but its first guest run exposed two IPv4 permitted whole-output EPERM failures; three IPv6 subtests and IPv4 lifetime passed. The normal two-call route repair passed the same b73f binary's two formerly failing IPv4 cases on actual d87/d2eb kernel and matching43a1 image (`631e2522`, no skips/all exits0/stopped); full three-raw/eight-packet and new SA-lifetime populations remain unrun there. The ordinary SA-replacement source d030 is independently CLEAR but still owes formatting/build/runtime under existing AC-4 queue lifetime. A native C-discrimination route is absent; the existing P3 design needs REWORK and owner design admission, with no escape or dependency credit substituted. Meaningful K2 proof remains safety-blocked, and wider checks/lint/RFC renewals remain owed. Bits 0–3 admission remains intact; AC-4 is still unmet and the existing partial annotation is not closed. These records do not reconstruct historical guest mutant 20 |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fast path plus complete path | A alone; B alone | A alone loses fragments (RFC 7296 Section 2.23); B alone keeps every bare datagram in userspace at 5000/s and on the TUN's input interface |
| Attach to every non-loopback link | the interface the peer's route uses | a route computed at install goes stale when routing to the peer changes, which a NOS does |
| IPv4 options left to the complete path; the IPv6 program variant leaves a datagram with extension headers untouched, and no path receives it yet: no IPv6 SPI is watched, the complete path's reader is `ip4:50` only, and IPv6 receipt is owed by `plan/post-release/features/spec-ike-ipv6-transport.md` | a variable-length header move in the program | the complete path already serves them correctly; the program stays short enough to prove |
| Rate bound kept on the complete path | deleting it | the complete path runs in userspace, which is what the bound protects. The coordinator's "delete it" was settled for A alone |
| `ze-espform` carries 192.0.0.8/32 scope host, `IFA_F_NOPREFIXROUTE` | no address, which loose `rp_filter` drops; a private address, which could collide with an operator's | RFC 7600 Section 6 reserves 192.0.0.8 as the IPv4 dummy address and Linux uses it as `INADDR_DUMMY`; scope host keeps it from being picked as a source, and no prefix route covers anyone else |
| The complete path relies on the production inbound IKE bypass for UDP 4500 | a bypass of its own for `ze-espform`; clearing the check on the device | the bypass already exists for the peer's own ESP-in-UDP and ranks ahead of every Child SA policy (`engine/bypass.go`); a second one would duplicate it, and clearing the check is the defect this spec closes |
| The complete path copies the received outer TOS into the header it builds | writing TOS 0, as it did | RFC 7296 Section 2.24 (`RFC7296-2.24-2`): XFRM copies CE from the outer header it decapsulates, so TOS 0 discarded every CE mark; the reader now reads the IPv4 header through `ReadMsgIP` |
| The engine creates `/dev/net/tun` when it is missing and the kernel allows it (`mknod` `c 10 200`, mode 0600, `/dev/net` too when missing), and makes and opens the device on every apply that configures an IPsec peer, at start and on reload (`recordESPFormTUN`); the doctor check only reads the recorded outcome (coordinator, 2026-11-12) | refuse every dual-form SA where the node is missing; create the node from the doctor check; probe only at the first Child SA install | a runtime doctor check must only read, never change the host; failing at apply shows the problem before the first Child SA is refused; a default Docker container can create the node (CAP_MKNOD, runc's `c *:* m` rule), so refusing would lose working tunnels |
| The fast path rewrites only while no input-path rule could drop ESP: a gate map `ze_esp_gate` the program reads first, 0 by default and on any doubt (owner decision, coordinator, 2026-11-12) | rewrite always and document that a `protocol esp` rule stops working; refuse a dual-form SA while such a rule exists | a datagram the fast path rewrote reaches every netfilter hook as UDP, so the firewall would lose a decision it held before this spec; refusing the SA would lose RFC 7296 Section 2.23; with the gate shut the complete path still serves what the rule lets through, after INPUT |
| The analyzer reads the kernel ruleset, every family and owner, and judges an unknown expression unsafe | read Ze's own `firewall.LastApplied` | `LastApplied` is a package global, empty in a forked IKE engine, and it never sees an operator's `nft` rule; a closed list of harmless statements keeps a new kernel expression from slipping past |
| Tables, chains and rules are dumped with the dataplane's own decode, and a message flagged `NLM_F_DUMP_INTR` fails the dump | the vendored `Conn.ListTables`, `Conn.ListChains` and `Conn.GetRules` | `GetRules` drops every expression it has no type for (`exprsFromBytes`), so a netdev `fwd` or an `xt` target would read as a rule with no verdict, and the walk would call it safe; `ListTables` and `ListChains` pass an interrupted dump on as whole, which is two rulesets spliced |
| Only base chains are read: a rule that can match ESP and jumps, goes to, returns or uses a verdict map is unsafe (review round 3, 2026-11-12) | follow jumps and gotos with a loop and depth bound, as first built | `nft_do_chain` (`net/netfilter/nf_tables_core.c`, v7.0): a return in a base chain skips every later rule and meets the policy, and a goto pushes nothing, so a walk that read on past either counted a later ESP accept that the ESP which left never reaches, and called safe a ruleset whose policy drops that ESP |
| A rule is excluded only when its own first two expressions are a protocol load (`meta l4proto`; the IPv4 protocol octet or `meta nfproto` in an `inet` or `ip` table) and an equality compare of that register against one octet that is not ESP's value, with no register model (review round 4, 2026-11-12, coordinator decision) | exclude on any compare the walk reasons ESP cannot pass; track the registers and judge a compare anywhere in the rule, as round 3 built | each load is a value the kernel computes from the header the program rewrites, with the kernel lines quoted at `espFormFirstLoad`; any other load can differ between what the program saw and what the rule reads, and each review round found a register write the model forgot, every one a rule judged safe that could drop ESP |
| Meta, ct and packet writes, map lookups, set updates, dup and object references close the fast path (review round 4) | accept mark set and DSCP set as harmless, as round 3 built | a mark selects a route through an fwmark rule, and `nft_payload_set_eval` writes the checksum delta at any `csum_offset` up to `U8_MAX` |
| GTSM's listen-port drop and the DDoS local drop open with the protocol match (`listenPortTerm`, `buildDropTerm`; coordinator decision, 2026-11-12) | keep the address first and accept that stock GTSM and every armed mitigation keep the fast path shut; teach the analyzer to exclude on a later protocol compare | the order changes no packet either term selects, while the analyzer excludes only on a rule's first two expressions; an address first lowers to `meta nfproto ipv4`, which every IPv4 ESP packet matches, so each such rule closed the fast path |
| The fast path's stock-ruleset tests judge the production lowering: `ddos/local` exports `MitigationTable`, the one builder `applyMitigation` calls, as `gtsm.FilterTables` is for GTSM (coordinator, option (a)) | a copy of the table's shape in the dataplane test | a copy drifts from the producer and goes on passing; the exported builder is the table the kernel receives |
| A set lookup of any kind closes the fast path ("set lookup", review round 5) | judge a membership lookup harmless, as a pure selector | `nft_lookup_eval` runs the matched element's own stateful expressions (`nft_set_elem_update_expr`), so a connlimit or quota on an element can drop ESP from the element alone, and reading them means dumping every set inside the generation bracket |
| A judgment is bracketed by two GETGEN reads and stands only when both answer the same generation (`errESPFormRulesetChanged`, review round 5) | trust `NLM_F_DUMP_INTR` alone | the flag marks a commit inside one dump only, so a commit between two dumps of one judgment (a drop moved from one table to another) read as no drop at all |
| Every change the watch delivers or a judgment sees shuts the gate, and a judgment that saw the ruleset move on every pass waits `espFormNFTRejudgeFirst`, doubling to `espFormNFTRejudgeMax`, before the next (review rounds 6 and 7) | judge again at once; keep the gate open until a judgment finds a drop | a stream of commits (fail2ban) had the worker dump back to back and never publish, and an open gate during a commit rewrote ESP past a rule the commit added |
| The bracket's claim is narrowed: the NEWGEN the watch signals once a commit applied is the backstop, bounding a splice the bracket passes by the watch's latency (review rounds 6 and 7) | claim the bracket refuses every splice | `nf_tables_commit` stores the bumped generation before it flips the generation cursor and applies table and chain updates, and GETGEN is answered under RCU, so a first read can answer the new number while the dumps read the old state |
| A dump message flagged `NLM_F_DUMP_INTR` counts as the ruleset moving (`espFormRulesetMoved`), never as a failed dump (review rounds 6 and 7) | record it as DumpFailed | it is a commit, not a failure: as DumpFailed the next pass replaced it at once, and a stream of such commits never met the wait |
| A stream of commits is its own reading, `moving`, which the doctor reports once it is 10 s old with `sudo nft monitor` and the advice to batch or stop the committing tool (review rounds 6 and 7) | report it as an unreadable ruleset, or not at all | the fast path stays off for as long as the stream lasts with no fault of Ze's, and only the operator can find and change the tool that commits; the complete path's bound applies meanwhile |
| An owner table's removal at its owner's exit, which the kernel signals with no generation bump and no NEWGEN, leaves a gate it shut closed until the next commit: a liveness limit, accepted (review rounds 6 and 7) | poll the ruleset to notice the removal | removing a table can make no safe ruleset unsafe, so the limit costs only bare ESP served by the complete path until the next commit; a poll would dump the ruleset on a timer forever to shorten a window that is never a safety risk |
| A refused gate shut detaches the program and records `Detached`, a status with its own doctor finding whose fix is a restart (review rounds 9 and 10) | record Unavailable; leave the program attached | Unavailable is a program that never loaded, whose fix is a capability or a kernel; an attached program behind a gate Ze cannot shut would rewrite bare ESP past a rule that should see it |
| The receiver's registry is keyed by SPI and local address, and `Forget` takes the local address (review rounds 9 and 10) | key on the SPI alone | two Child SAs on two local addresses can draw one random inbound SPI and XFRM installs both, so a second Watch replaced the first SA's target, and the `RemoveSA` of an outbound SA whose SPI equals a watched inbound SPI forgot that inbound SA |
| A MOBIKE move checks the receiver for a collision; for an unwatched SA it opens and holds the receiver's paths (`acquire`) and refuses with `xfrmMigrationUnserved` when they cannot open; then it moves the SAD, releasing the hold and rolling back on failure; then the receiver follows (`watchHeld` for an unwatched SA, `retarget` for a watched one), and a retarget failure warns and never rolls the SAD back (review round 11) | watch an unwatched SA after the move and warn on failure, as round 10 built; a plain acquire then register with no hold count; retarget the receiver first; roll the SAD back when a retarget fails | RFC 7296 Section 2.23: a watch that fails after the move leaves a templated SA receiving one form, which `InstallSA` refuses; without the hold count a `Forget` of the last other SA between the two steps would close the paths; a refusal before any change leaves the tunnel as it was; once the states moved the kernel receives the ESP-in-UDP form, so a rollback would trade a working tunnel for the bare form |
| `Watch` takes the whole `espFormTarget`, ports included, so `InstallSA` makes one call (review round 11) | `Watch` then `retarget`, as before | the two calls left the SA served with ports 4500/4500 until the retarget, and the retarget's error was discarded |
| A shared SPI is a doctor warning per unserved SA whose fix is `ze clear vpn ipsec sa peer <name>` (review rounds 9 and 10) | a fix that rekeys the one Child SA | Ze has no command that rekeys a single Child SA; clearing the peer negotiates a new Child SA, which draws a new random SPI and ends the collision |
| The migration collision and the IPv6 refusals are `opererr` operator errors (review rounds 9 and 10) | plain wrapped errors | every message an operator sees names the problem, why, and the fix (`ai/rules/user-facing-errors.md`) |
| The program passes every VLAN-tagged skb to the complete path (review round 3, 2026-11-12) | rewrite tagged frames too | the tag is out of band by TCX (`skb_vlan_untag`), the skb's protocol already names the inner IPv4, and a rule on the parent link can match the tag (`nft_payload_copy_vlan`), so a rewrite would hand that rule UDP where it matches ESP |
| A reservation declares what decides its table's rules (`firewall.TableControl`, required by `MustReserveTableNames`), and the doctor's fix for a `ze_` table comes from it through a hook the engine registers (review round 3, 2026-11-12) | name one fix for every `ze_` table (`ze config edit`, as first built); a list of producer tables in the dataplane or the engine | the editor cannot reach a producer's table and a producer rewrites it, so only the producer can say what changes its rule; a declaration at the reservation is the registry that already holds every producer table, so no central list names them, and the dataplane, which does not import the firewall, asks the engine through `RegisterESPFormOwnedTableAccept` |
| Ze's own firewall commits hold the gate shut through a firewall commit observer the IKE engine registers from `init()` | rely on the nftables monitor alone; have the firewall import the IKE dataplane | the monitor sees a commit only after it landed, so a rule Ze adds would be bypassed for the monitor's latency plus a dump; an observer registry keeps the firewall unaware of IKE (registration over enumeration), and the observer never calls the firewall back, because it runs inside `reconcileMu` |
| The watch asks for a 16 MiB receive buffer and is opened again after a failure, 1 s doubling to 30 s, with a fresh judgment once it is up | `NETLINK_NO_ENOBUFS`; give up until the next start | `NO_ENOBUFS` hides an overflow, and a lost NEWGEN is a commit the gate never judges; a watch that stays down keeps the gate shut, so it must come back by itself |
| An install, binding or build refusal of the first Child SA keeps the IKE SA on `childCreateSchedule` and deletes the peer's half | end the IKE SA, as HEAD did | RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be deleted."; ending it re-ran the whole handshake every second for as long as the cause lasted, because `connectFailures` had been cleared; the schedule already exists for a childless IKE SA |
| The engine's runtime checks are declared at Stage 1 and answered by the engine's own process (`OnDoctorCheck`) | post-config `registry.DoctorCheckDef`s in the daemon | the state they read lives in the engine's process: a post-config check reads empty globals in the operator's `ze doctor` process and in a daemon whose engine runs forked, and answers healthy |
| trafficusage answers `TCX_NEXT`, and copies the L4 port with `bpf_skb_load_bytes` at a scalar offset (owner decision on the side defect, 2026-11-12) | keep `TCX_PASS` and rely on the fast path attaching at the head; grant `CAP_PERFMON` | `TCX_PASS` also skipped every clsact filter on the link (policer, sFlow sampler, mirror); `CAP_PERFMON` widens what every program may do, while a helper read needs no packet-pointer arithmetic, which the unprivileged verifier refuses with a register operand |
| Every lab Ze that can run IPsec holds `/dev/net/tun` (`TUNDeviceArguments`), the IPsec interop ze peer runs under the net-sysctl profile, and the QEMU per-test namespace widens `/dev/net/tun` | grant the device only to scenarios that encapsulate | Ze decides at run time whether an SA is encapsulated (NAT detection, MOBIKE, a peer arriving on 4500), and the install is refused without the device, so a per-scenario grant would fail scenarios that float |
| Host-caused receiver open refusals are operator errors with a cause-specific fix (review rounds 12 and 13) | a generic retry command, a guessed configuration fix, or `ze show doctor` as the fallback | the cause distinguishes a capability, security policy, file limit, memory shortage or unclassified host answer; before the round-12 fix, doctor had no finding for these causes |
| The receiver records its last failed open for doctor and deduplicates it with the apply's TUN record (review rounds 12 and 13) | infer health from fast-path status or report two errors for the same refusal | an attached fast path does not prove the raw reader and TUN can open, and a failed counter read must not hide a recorded refusal |
| A stopped reader has a joined revive worker with receiver-level backoff and an explicit closed-receiver guard (review rounds 12 and 13) | wait for a new Watch; restart backoff on every newly opened reader; reopen after Close | installed SAs still need the receiver, a repeatedly short-lived socket must not cause a tight loop, and Close must leave no worker reopening resources |
| A raw reader stop is a registered report-bus error event sent through the engine's plugin request (review round 13) | rely on a live doctor record after the first successful reopen; raise on a process-local bus from a forked engine | a transient stop leaves no open-failure record after recovery; the daemon's error ring retains the event and both plugin transports reach it |
| Hold ends reject a missing acquire before changing the count, and migration defers release of a still-live hold (review round 12) | allow an unmatched decrement or rely only on normal return branches | an unmatched end is a Ze defect; panic during a SAD move must not hold paths open for an SA never registered |
| IPv6 dual-form serving is a gap owed after the first release | building the IPv6 reader and builder now | no IPv6 IKE socket exists (`transport/udp.go` listens on `udp4`, `engine/probe.go` refuses IPv6), so no code could reach them; `plan/post-release/features/spec-ike-ipv6-transport.md` owns the work, and the refusal names the gap |

## Known Limitations

Each item is owed work with an owner, never out of scope.

- `esp4_offload`/`esp6_offload` GRO on a distribution kernel consumes bare ESP before tc ingress and before raw sockets, so neither path receives it. Ze's kernel does not build `CONFIG_INET_ESP_OFFLOAD`. Owned by this spec through the journal row `kernel-refuses-what-the-installer-sends.md` 2026-11-12, `esp4_offload` (above).
- Both ESP forms over an IPv6 outer header: a gap owed after the first release, owned by `plan/post-release/features/spec-ike-ipv6-transport.md`, section "Dual-form receipt over IPv6" (AC-8).
- The firewall gate's four limits, accepted by the owner's firewall-gate design (Decision) and documented with their fix in `docs/guide/ipsec.md`, "Both ESP wire forms": iptables-legacy rules (x_tables, which no nftables dump returns); a tc ingress program or filter another owner attached after Ze's program; the window between a foreign nftables commit and its judgment (the monitor's latency plus one dump); and, with the IKE engine forked, the same window for the daemon's own firewall commits, which take no hold. In each a datagram is still decrypted and checked against the SA's selectors; what it misses is the firewall's decision. Two more are documented there: ESP rewritten just before a commit's hold began meets the commit's new rule as UDP when the commit lands first (`firewall.CommitObserver`), and a rule that drops UDP is read as excluding ESP yet drops the rewritten datagram, which fails closed.
- The config editor cannot put a firewall term ahead of the chain's existing terms, so the `accept-esp` lines the doctor names for a table of the firewall config accept ESP only where no earlier term drops it. Owned by `plan/post-release/features/spec-config-list-entry-insert.md` through the journal row in `plan/journal/` named in its Owned journal rows table.
- A `CHECKSUM_COMPLETE` IPv6 skb would take the complete path, because the program cannot produce a valid checksum for it (`espFormFastPathIPv6`). No IPv6 SPI is watched today, so nothing meets it; the IPv6 complete path that would carry it is owned by the same IPv6 spec, which lists it.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features
- [ ] `./le test unit pkg path ./internal/component/ike/dataplane/`, the guest-root tests in a QEMU guest on Ze's kernel, `./le test functional ipsec` for the doctor `.ci`, then `./le test integration interop-ipsec` for `icmp-error-payload-check` and `esp-form-change`
