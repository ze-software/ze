# Spec: ipsec-dual-form-receive-spd-bypass

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P0 |
| Release reason | Security defect in shipped IPsec: on a Child SA moved to UDP 4500 without a NAT, bare ESP from an authenticated peer is decrypted and delivered with no inbound SPD check, so the peer can inject traffic with any inner source and destination. |
| RFC gaps | RFC4301-5.2-2 |
| Depends | - |
| Phase | 1 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the inbound SPD bypass of the dual-form receive path, keeping both ESP
forms on one Child SA, for IPv4 outer headers. Serving both forms over an IPv6
outer header is a gap owed after the first release (AC-8): Ze has no IPv6 IKE
socket, so no IPv6 dual-form SA can be negotiated, and
`plan/post-release/features/spec-ike-ipv6-transport.md` owns that work.

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
| AC-1 | interop `icmp-error-payload-check`, strongSwan sending bare ESP (no `encap = yes`) | the scenario passes; its two red proofs in `plan/release/fixes/spec-rfc4301-architecture-gaps.md` are recorded | the in-selector error is still delivered, because the selector check passes it |
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
| `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked` | `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go` (guest-root; engine package so the SA is built by `createFirstChildSA`) | AC-3 | |
| `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE` | same file (guest-root) | AC-11 | |
| `TestRFC4301BareESPFirewallDropKeepsTheFastPathOff` | same file (guest-root; no RFC tag, because it proves a firewall interaction) | AC-13: an operator's `nft` drop rule on ESP keeps the fast path off and the rule drops the bare ESP; with the rule gone the fast path rewrites again | |
| `TestEncapEstablishedSAServesAPeerFormChange` (judged change) | `internal/component/ike/dataplane/encap_formchange_integration_linux_test.go` | AC-2 at the kernel: the complete path over `ze-espform` on routed addresses, with the inbound IKE bypass installed as production does | |
| `ipsec-esp-form-fast-path-doctor` | `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`, promoted to `test/ipsec/` after its runs | AC-10: `ze doctor` names the fast path state, under the zetest-only `recording-esp-form-unavailable` backend | |
| `service-unit-gen` (updated) | `test/ui/service-unit-gen.ci` | AC-12: the generated unit's capability lines | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `icmp-error-payload-check` | `test/interop-ipsec/scenarios/` | strongSwan | AC-1 | |
| `esp-form-change` | `test/interop-ipsec/scenarios/` | strongSwan | AC-2 | |
| `esp-form-firewall-drop` | `test/interop-ipsec/scenarios/esp-form-firewall-drop/` (`ze.conf`, `swanctl.conf`), checker `checkESPFormFirewallDrop` (`internal/le/interoplab/ipsec/espformfirewall.go`) | strongSwan | AC-13: strongSwan sends bare ESP on a 4500-without-NAT SA, Ze's `drop-esp` term drops it and counts it, the doctor names the blocking table and term; red when `writeGateLocked` writes the open value whatever it is given (`docs/architecture/testing/interop.md`) | |

## Files to Modify

From `git status --short` over the spec's paths on 2026-11-12, this spec's files only. **Shared** marks a file that also carries a hunk this spec cannot claim (another session's work, for example the RFC 4301 ICMP and implicit-discard work): commit this spec's hunks of it with `hunks <path> <patch-file>` after judging the rest against HEAD (`ai/rules/git-safety.md`).

Feature code:
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
- `internal/component/ike/engine/reconcile.go` - `PeerSession.childRefused`, read by the `ipsec-child-install` check.
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
- `internal/component/ike/dataplane/espform_linux_test.go`, `espform_run_linux_test.go`, `espform_test.go`, `recording_shipped_test.go`.
- `internal/component/ike/dataplane/encap_formchange_integration_linux_test.go`, `encap_dualform_integration_linux_test.go`, `encap_hybrid_integration_linux_test.go`, `xfrm_migrate_integration_linux_test.go`, `xfrm_migrate_linux_test.go`, `xfrm_teardown_integration_linux_test.go` - routed addresses instead of loopback, the IKE bypass, the TOS argument; with `xfrm_state_del_failure_linux_test.go`, the registry read by SPI and local address (review rounds 9 and 10).
- `internal/component/ike/dataplane/xfrm_linux_test.go` - `TestXFRMInstallSARefusesIPv6DualFormSA`.
- `internal/plugins/systemd/unit_test.go`, `internal/test/runner/caps_netns_test.go`, `internal/test/runner/caps_option_test.go`, `test/ui/service-unit-gen.ci`.
- `internal/plugins/trafficusage/program_test.go`, `doctor_test.go`.
- `internal/component/gtsm/gtsm_test.go` - `assertListenDrop` expects the reordered matches (judged change); `internal/plugins/ddos/local/match_test.go` - `TestBuildDropTermPutsTheProtocolFirst`.
- `internal/le/interoplab/ipsec/privilege_test.go`, `internal/le/verify/evidence/evidence_test.go`, `internal/le/test/qemu/netns_alltests_test.go`.

Docs and records:
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
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` `RFC4301-5.2-2` (`{partial}` reason rewritten; dropped once the records exist, AC-4); `RFC7296-2.24-2` in `rfc/short/rfc7296.md` carries no annotation and needs no edit, and its new tag owes a record |
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
| Resource exhaustion | the complete path stays rate-bounded; the map has a fixed size and a full map refuses the install |
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
| Main-thread `./le repo compiles check` after checkpoint `abeb7d6535` | all five committed-tree build configurations compiled; this excludes test files and is not proof of the later working-tree edits |
| Initial final-code main-thread `./le test unit pkg path ./internal/component/ike/dataplane/ path ./internal/component/ike/engine/ path ./internal/core/diagnostic/` | engine and diagnostic packages passed; three dataplane fixture/expectation failures were subsequently corrected and committed in `bd8e21b487`. The targeted package run `job-unit-pkg-58fce57a.log` and narrow race/count-20 scheduling run `job-ipsec-reopen-schedule-e00a01c8.log` passed. Final guest verification remains owed |
| Receiver doctor and binding fixture units | the named tests passed under race/verbose execution in `job-ipsec-binding-doctor-units-419db7fc.log`; this includes the offload and complete-path doctor findings |
| Host mutation walks 27, 31–33, 37, 39–49 | all sixteen returned the native red verdict, with clean own-test passes and broken own-test failures, in `job-ipsec-dualform-host-redwalks-aa95f75a.log`; guest mutant 20 remains owed |
| Final guest-root tests, guest mutant 20, doctor draft stress/promotion, RFC records and interop | not completed on the final code; neither the host mutation results nor earlier batches close these obligations |

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
