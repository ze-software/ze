# Handover: IPsec dual-form receive SPD bypass (2026-11-12)

Spec: `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`. It stays in
progress. The owner ordered everything committed and the rest finished in a fresh
session, so this commit carries code that is only partly verified. The open items
are listed below. Read the spec's Decision section and Key Design Decisions table
first: every design choice below is recorded there with its rejected alternative.

## What the work is

On a Child SA moved to UDP 4500, Linux holds one inbound XFRM state per SA that
accepts one ESP form (LL-XFRM-STATE-ONE-ESP-FORM). RFC 7296 Section 2.23 requires
both forms "at any time". Before this spec, bare ESP was re-presented through
loopback with no inbound SPD check. Thomas chose option A, and the coordinator
extended it to A+B.

- **Complete path.** A raw `ip4:50` reader re-presents bare ESP as ESP-in-UDP into
  the TUN `ze-espform`, which crosses netfilter and the SPD.
  - Receiver: `internal/component/ike/dataplane/espform_linux.go`; open and revive
    code: `espform_open_linux.go`; registry: `espform.go`.
- **Fast path.** A TCX BPF rewrite (`espform_program_linux.go`), gated by the map
  `ze_esp_gate`. The gate opens only when an analyzer of the live nftables ruleset
  proves no rule can drop or change bare IPv4 ESP.
  - Analyzer: `espform_firewall_linux.go`; kernel reader:
    `espform_firewall_kernel_linux.go`; gate: `espform_fastpath_gate_linux.go`;
    manager: `espform_fastpath_linux.go`.
- **Doctor.** `internal/component/ike/engine/doctor_esp_form_fast_path.go` and
  `doctor_child_install.go`.

## Done and proven

- **Batches 1-10**, run serially by the coordinator:
  - host unit packages: ike dataplane (race on), the zetest flavour, ike engine,
    firewall, gtsm, ddos/local, reactor TestGTSM, every producer `register_tables.go`
    package, trafficusage, systemd, privilege, doctor, diagnostic;
  - the guest-root dataplane, engine and trafficusage tests on Ze's kernel (22/22 in
    batch 8);
  - every arch gate, scoped lint and gofmt.

  All of it ran green on the round-11 code (batch 10 was on round-12 code).
- **Mutation reds** (batch 9 part B, plus 19/19b/19e and 24-38 in batch 10): every
  host entry went red naming its intended test, and each file was restored and
  verified.
- **Red proofs observed earlier:** TUN a-d; R2, R4, R5.
- **Judge approvals** are in the untracked `tmp/commit-weakening-approved-79a29bf3.md`.
  The commit carries them as `Weakening-approved:` trailers: row 18
  (`gtsm.assertListenDrop`) and the dataplane rows 19-33, plus the round-12 rows for
  `TestXFRMMigrationUnwatchedSAOpensThePathsBeforeTheMove`, `driveESPFormRunConn` and
  `driveESPFormRun`.
- **Adversarial review rounds 2-13.** No safety defect has been found since round 7.
  These invariants held from round 8 to round 13:
  - the gate's only open write is `publishFirewall`, under `mu`, behind the epoch
    compare and `holds == 0`;
  - every ruleset change (commit, hold, release, watch end, rewatch, stop, an
    `NLM_F_DUMP_INTR` dump) steps the epoch and shuts the gate, with a rejudge backoff
    of 50 ms doubling to 1 s;
  - each judgment is bracketed by `NFT_MSG_GETGEN`, with the following NEWGEN as the
    backstop;
  - only base chains are read; a rule is excluded only when its own first two
    expressions are a protocol load and an equality compare against a non-ESP value;
    the harmless list is closed; every set lookup is unsafe;
  - the receiver registry is keyed by (SPI, local address);
  - MigrateTunnel runs `admitsMove`, then acquires the receiver's paths before the
    SAD move (an open failure refuses with nothing changed), then moves, then
    `watchHeld` or `retarget`; the hold count keeps paths open between acquire and
    register;
  - Watch takes the full target, ports included.

## Open: round-13 fixes are written but untested

The round-13 fix agent finished every round-13 item before the stop order. So the
working tree carries all of them. None has been compiled or run since; the agent's
final `gopls check` was refused at its call budget. A fresh session must first run
the compile check and the scoped tests below, then a round-14 review of that diff.

| # | Item | State |
|---|------|-------|
| 1 | Revive worker Warn carries `slog.String("code", diagnostic.CodeDoctorIPsecESPFormTUN)` at the call (doctor-coverage) | done |
| 2 | EACCES on the raw socket or IP_XFRM_POLICY: `espFormSecurityModuleRefusal`, fix `sudo journalctl -b --grep 'avc:\|apparmor="DENIED"'` | done |
| 3 | `espFormFixRetry` removed. EMFILE gets the unit reinstall (unit sets `LimitNOFILE=65536`); ENFILE gets `sudo sysctl -w fs.file-max=...`; ENOMEM/ENOBUFS get `free -m`; anything else gets `ESPFormFixLog` | done |
| 4 | Reader stop raised on the report bus. New code `doctor-ipsec-esp-form-reader-stopped` (codes.go; `RegisterEventCode` in `engine/register_esp_form_reader.go`). The sink is registered in `runEngine` and calls `p.RaiseIssue`. Assumes `sdk.Plugin.RaiseIssue` is goroutine-safe: verify it | done, unverified |
| 5 | `reopenMin` seam; the test no longer sleeps 1 s | done |
| 6 | `closed` flag: Watch and acquire after Close answer `errESPFormReceiverClosed` | done |
| 7 | Receiver-level `reopenWait`, reset only after a reader served `espFormReopenResetAfter` (1 min) | done |
| 8-11 | Doctor wording "moved onto the ESP-in-UDP form"; stale `espFormTUNFindings` comment; dedupe (`espFormDeviceFindings`, `espFormSameCause`); "30 seconds" from the constant | done |
| 12 | TUN step refusals reworded; EPERM uses `privilege.DaemonCapabilitiesFix`; other answers use the log fix | done |
| 13 | `espFormBypassInboundPolicy(conn *net.IPConn)` | done |
| 14 | `settleESPFormTUNSysctls` failure is an opererr (`espFormTUNSysctlWriteRefusal`, `sudo sysctl -w net.ipv4.conf.default.<leaf>=<want>`) | done |
| 15 | .ci comment names `&` | done |
| 16 | "the IPsec complete path" replaced in operator text | partial: the other fast-path findings still say "the complete path" |
| .ci 9 | The draft regex expects sorted keys `code`, `fix`, `message`, `severity`, because `applyJSON` re-encodes the answer | done, not run |

Known advisory: `internal/component/ike/engine/register.go` is now 1072 lines, past
the 1000-line hook advisory. `xfrm_linux.go` was already over at HEAD; the owner
said leave it, and a split would be its own later commit.

Tracked tests the round-13 pass changed owe a ze-test-judge verdict before they
count as proven:
- `dataplane/espform_refusal_linux_test.go` (EACCES, EMFILE and TUN rows);
- `dataplane/espform_tun_sysctl_linux_test.go` (the EIO row);
- `engine/doctor_esp_form_complete_path_test.go`;
- `dataplane/espform_linux_test.go` (`TestESPFormBypassRefusesANonIPSocket` became
  `TestESPFormBypassRefusesASocketItCannotExempt`);
- `dataplane/rfc4301_override_guard_integration_linux_test.go` (a type assertion).

When the commit body or spec states why the `ze show doctor` fallback was removed,
say: "before the round-12 fix, doctor had no finding for these causes". Doctor now
has one.

## Remaining run list, in order

All runs go one at a time in the main checkout. No agent runs `./le verify`.

1. Compile check, then `./le test unit pkg path ./internal/component/ike/dataplane/`
   (and the zetest flavour), `./internal/component/ike/engine/`, and
   `./internal/core/diagnostic/`.
2. Gates: `./le arch` compound-guard, refuse-by-default, error-wrap, operator-error
   and doctor-coverage; scoped lint on `./internal/component/ike/...` and
   `./internal/core/diagnostic/...`; gofmt on the edited files. `./le doc check all`
   failed only on other sessions' anchors in batch 10.
3. A3, guest root, on the final receiver code:
   - the batch-8 guest set: the program, the gate kernel ruleset, the NFT watch
     receive buffer, TUN node, unit caps, every `TestESPFormFirewallStock*`, the
     xfrm migrate/teardown/formchange integration tests;
   - `TestESPFormFastPathSecondSATakesOver`, `TestESPFormFastPathWarnsOfASharedSPIWhileAttached`;
   - engine `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`,
     `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE`,
     `TestRFC4301BareESPFirewallDropKeepsTheFastPathOff`;
   - trafficusage guest tests;
   - mutant entry 20.
4. Part B on the entries the round-13 pass changed (27, 31, 32, 33, 37) and added
   (39-49). The file is `tmp/session/2026-11-09-222726/scratch/b9-mutants.json` (57
   entries; the check script `b9-mutants-check.py` in the same directory last reported
   57 and 0 bad). Format: `{"id","file","old","new","pkg","run","guest"}`. The
   coordinator ran them with its scripted loop: save, mutate, test, restore, cmp.
   Entry 23 needs `-race`; 8, 9 and 16 go red by timeout; 20 is guest. Re-run the
   check script first: the round-13 edits came after its last verification.
5. The draft `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`. Steps 1-8 passed in
   batch 10; step 9 was red on the old regex and is now aligned. Then
   `mv test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci test/ipsec/` and one run
   from there.
6. Records, after a fresh discriminate report for each stem
   (`./le rfc discriminate stem rfc4301|rfc7296|rfc5082 report <path>`). Recompute
   every selector, because the files moved:
   - RFC4301-5.2-2 positive: unit
     `internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go::TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`.
     Mutant: `espFormBypassInboundPolicy(conn)` to `error(nil)`. The call is in
     `espform_open_linux.go` or `espform_linux.go`; find it.
   - RFC4301-5.2-2 negative: same unit. Mutant: the `0` returned by `want()` for
     `espFormTUNDisablePolicy` in `espform_tun_linux.go` becomes `1`.
   - RFC7296-2.24-2 positive: unit `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE`.
     Mutant: `tos` in `buf[1] = tos` (`espform.go`) becomes `0`.
   - RFC5082-3-4 re-record: unit
     `internal/component/gtsm/rfc5082_listener_integration_linux_test.go::TestGTSMListenerUnknownNotDropped`,
     producer `listenPortTerm`. Mutant: remove `firewall.MatchSourceAddress{Prefix: source}`
     (gtsm.go, about line 405).

   Then `./le rfc check`, and drop `{partial}` from `rfc/short/rfc4301.md`
   `RFC4301-5.2-2` (AC-4).
7. Interop, `./le test integration interop-ipsec`, one scenario at a time:
   - `mtu-nat-installed-endpoint`;
   - `icmp-error-payload-check` (AC-1), as committed with no `encap = yes`, plus its two
     red proofs;
   - `esp-form-change` (AC-2);
   - `esp-form-firewall-drop`, plus its red: make `publishFirewall` ignore a blocked
     verdict so the gate is forced open, expect the scenario's ESP-drop check to go red,
     then restore.
8. Spec bookkeeping still owed (below), then `/ze-review`, then `/ze-close`.

## Spec bookkeeping still owed

Items (a) to (e6) and the round-11 MigrateTunnel rewrite are applied in the spec. These
remain (from the session scratch file, copied here because `tmp/` is not committed):

- **(e7) Round 12 and round 13.** Add a Decision line, deviation rows and test rows
  for each of these:
  - receiver open failures are opererrs with their own fixes (EPERM/EACCES/EMFILE/
    ENFILE/ENOMEM classes, the security-module and log fixes);
  - the recorded last open failure, with its doctor finding under
    `doctor-ipsec-esp-form-tun` and the dedupe;
  - the dead reader: its exit marks its life dead and records the cause; the next
    Watch or acquire reopens or refuses; the per-reader revive worker uses
    receiver-level backoff (1 s doubling to 30 s, reset after 1 min of service), joined
    by Close, with the `closed` flag;
  - the reader stop on the report bus (`doctor-ipsec-esp-form-reader-stopped`, through
    `RegisterESPFormReaderStopped`);
  - the hold-count BUG guards and the deferred release.

  The dead-reader decision (coordinator, 2026-11-12) puts the reopen in this spec, not
  in `spec-doctor-coverage-3-access-security.md` AC-34. Add a note to AC-34 if its
  wording needs one.
- **(f)** After the records, drop `{partial}` from RFC4301-5.2-2.
- `./le spec citation anchors spec plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`
  (`gtsm.go` and `match.go` are new subject files).

Spec edits need the doors and the producing source ranges read in the editing agent
(the spec-edit hook enforces it), so batch them in one agent.

## Follow-on specs and journal rows created

New skeleton specs, each owning its journal row:
- `plan/release/fixes/spec-privilege-drop-keeps-capabilities.md` (row in
  `lifetime-ends-before-its-last-reader.md`);
- `plan/release/fixes/spec-anomaly-shape-limit-drops-nothing.md` (row in
  `silent-fall-through.md`);
- `plan/release/fixes/spec-ike-peer-info-reads-child-after-unlock.md` (new class
  `field-read-after-its-lock-is-released.md`: `PeerSession.Info` reads child fields
  after `ps.mu` is released while `migrateMobikePair` writes them).

Rows added to existing owners:
- `hook-existing-patterns-false-positive.md` (two rows) and `two-owners-share-one-name.md`,
  owned by `plan/post-release/engineering/spec-weakening-check-judges-units-head-never-held.md`;
- `process-global-read-across-the-plugin-boundary.md`, owned by
  `spec-doctor-coverage-3-access-security.md`;
- `error-names-no-fix-command.md`, owned by
  `plan/post-release/features/spec-config-list-entry-insert.md`, which gained an
  Existing Mechanisms section;
- `kernel-refuses-what-the-installer-sends.md` (esp4_offload GRO; CAP_BPF), owned here.

## Second spec: rfc4301-architecture-gaps

`plan/release/fixes/spec-rfc4301-architecture-gaps.md` is committed (8b3c7a34cd,
2db581e7d6) but not closed. Still owed:

- (a) Interop `mtu-nat-installed-endpoint` and `ipsec-bgp-redistribute-frr`, proven
  green with its `ze.conf` changes. Both were blocked earlier, by the `/dev/net/tun`
  lab gap (now fixed here) and by another session's cli build.
- (b) The two red proofs of `icmp-error-payload-check`, which this spec's AC-1 shares.
- (c) The seq=4 hunk of `test/parse/ipsec-icmp-type-selector-refused.ci`. It is left
  uncommitted for `spec-config-enum-refusal-proposes-valid-values`, because its
  producer is uncommitted. Its combined weakening reason, verbatim: "Only the seq=1
  peers gain icmp-error-check false, which no selector validator reads, so seq=1 still
  exercises the same validators and seq=2 to 10 are unchanged and refused before the
  ICMP check; seq=4 keeps exit=1 and contains=icmp-type and now asserts icmp-type:
  \"256\" is not an accepted value, which names the leaf and the exact refused value as
  the enum-kind refusal problem() renders it, so it proves at least what invalid value
  \"256\" did."
- (d) Then `/ze-close` for that spec.
