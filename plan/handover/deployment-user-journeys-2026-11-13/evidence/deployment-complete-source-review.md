# Deployment source preparation

Reviewer: deployment_complete_source_review. Pin:
`6307518170f6ddc66198b3e956ec683952673c95`.
This is preparation, not a CLEAN Review Gate or closure. No product authoring,
test/build/formatter execution, tracked edit, claim change or subagent work.

## Population

Verified 46 commits whose message contains the exact line
`Spec-work: deployment-user-journeys`. Their paths, filtered by the pinned
`internal/le/spec/review.go` `IsReviewCode` declaration, select **400** files.
The former 395-path list plus these five is the current population:

- `internal/plugins/local/native_purge_ownership_linux_test.go`
- `internal/plugins/ospf/config_runtime_rollback_test.go`
- `internal/plugins/ospf/config_transaction.go`
- `internal/plugins/ospf/export_chain.go`
- `internal/plugins/ospf/export_chain_slot_test.go`

Existing files enter the population when this spec first changes them; the
last two are therefore additions to the population despite already existing.
The 120 reviewable paths from `abeb7d6535` remain included. No ownership-based
exclusion is applied.

## Review method and current limit

For each bucket, read all selected historical hunks and the pinned producers
they change. Apply runtime/security ownership, tests/wiring/boundaries, and
style/documentation lenses. Record actual completed reads separately below.
Prior reports and the 52-producer ledger are navigation, not acceptance.

Root owns automated prechecks and runtime/artifact/site evidence. Known
in-progress IS-IS transaction/transport repairs and the daemon rollback draft
are assigned already; later committed deltas remain owed here. Native and
OSPF execution claims belong to root. No missing proof is accepted as a pass.

## Actual read ledger

The native classifier and architectural/rule contracts were read to establish
scope. Completed source/test hunks at the pin:

- `internal/component/ike/dataplane/espform.go`: full historical Spec-work
  change and pinned source. Registry identity `(SPI, local address)`, ordered
  fast-path selection, exact forget semantics, token accounting, IPv4 length
  admission, TOS/header reconstruction and checksum reviewed. Caller wiring
  continues with the receiver bucket below; no whole-IKE acceptance inferred.
- `internal/component/ike/dataplane/espform_test.go`: full historical changed
  assertions and pinned unchanged limiter, builder and refusal controls read.
  Multi-SA identity, shared-SPI order, exact byte preservation, independent
  checksum vector/reused-buffer tests, TOS and malformed-length controls read.
  Existing assertions retained under the changed helper signatures.
- `internal/component/ike/dataplane/espform_linux.go`: entire pinned source
  and historical removed lines/hunk boundaries read. Watch and hold admission,
  stopped-reader reopen, socket/TUN teardown ordering, immutable failure
  snapshot, exact retarget collision, per-socket bypass policy, run-loop
  admission and tuple preservation reviewed with the three lenses.
- `internal/component/ike/dataplane/espform_open_linux.go`: entire new source
  read. Errno-specific actionable refusals, socket cleanup when later opening
  fails, worker retirement and Close/revive lock ordering reviewed.
- `internal/component/ike/dataplane/espform_refusal_linux_test.go` and
  `internal/component/ike/dataplane/espform_reopen_linux_test.go`: entire new
  source read. Refusal errno/fix chains, panic-safe hold release, stopped
  reader reopening, failure snapshots, eventual healing, backoff and closed
  receiver assertions reviewed. A test-only timing note is recorded below.
- `internal/component/ike/dataplane/espform_tun_linux.go`: all 890 new lines
  read. Fixed clone-device paths/mode, create-only-on-ENOENT, own-directory
  cleanup, nonpersistent TUN ownership, address/MTU/up order, integer sysctl
  reads, read-only effective-value decisions, no-write probe, short-write
  refusal and one-shot fd Close reviewed. conf/all disable_policy ownership
  exception matches the explicit architecture contract; no RFC verdict added.
- `espform_tun_open_linux_test.go`, `espform_tun_sysctl_linux_test.go`,
  `espform_tun_probe_linux.go`, `espform_tun_probe_linux_test.go`,
  `espform_tun_probe_other.go` (all in the same dataplane directory): all new
  source read. Errno/capability and exact syscall-order cases, namespace-bound
  /dev tests, inherited/effective-value matrix, shared decision rule, probe
  write prevention and non-Linux refusal reviewed.
- `espform_backend_linux.go`, `espform_backend_other.go`, `espform_state.go`,
  `espform_firewall_hold.go` (same directory): all new source read. Reporter
  registration interfaces, closed enums, atomic immutable callback publication,
  caller-owned hold release and status/operator contracts reviewed.
- `espform_fastpath_linux.go`, `espform_fastpath_gate_linux.go`,
  `espform_firewall_linux.go`, `espform_firewall_kernel_linux.go`,
  `espform_program_linux.go` (same directory): all new source read. Two-level
  lifecycle locks, namespace/fd ownership, subscription draining, gate epoch and
  hold transitions, failure detach, bounded rewatch/rejudge, conservative
  first-two-expression exclusion, closed harmless list, lossless/opaque rule
  decoding, generation bracketing and monitor failure lifecycle reviewed.
  Bytecode review covers source/destination/SPI checks, lengths, VLAN/fragment/
  options refusal, checksum validation/recomputation, short store drop and
  counter placement. IPv6 producer deliberately unreachable under current
  admission; no future feature requirement invented. Test/wiring coverage of
  these five files is still partial until the test cohorts below are read.
- `espform_fastpath_linux_test.go`,
  `espform_program_integration_linux_test.go` (same directory): all new source
  read. Link type matrix and independent exact-byte BPF_PROG_TEST_RUN oracle,
  gate-open positive/shut control, source/local/SPI mismatch, fragments/options,
  malformed lengths/checksum, padding and checksum-state cases reviewed.
- `espform_fastpath_gate_linux_test.go`,
  `espform_fastpath_integration_linux_test.go`,
  `espform_firewall_linux_test.go`, `espform_firewall_stock_linux_test.go`
  (same directory): all new source read, including the recovered truncated
  moving-stream fixture region. Reviewed stale judgment/hold/commit schedules,
  positive safe reads and failed dumps, failed-shut detach, exact rewatch timers,
  bounded teardown, stream state replacement, real link ownership/query and
  map identity, full conservative rule-shape table, raw ct-set/opaque decoding,
  generation mismatch/interrupt refusal, coalesced monitor and lowered stock
  GTSM/DDoS controls. No test run or mutation result is inferred from source.
- `espform_linux_test.go`, `espform_run_linux_test.go` (same directory): full
  pinned files plus all historical removed lines/hunk boundaries read. Existing
  socket lifetime and byte-content assertions survive the TUN/ReadMsgIP seams.
  Removed non-IP PacketConn control is now statically excluded by *net.IPConn;
  the current unbacked-IPConn failure still checks refusal and operator error.
  Registry race, exact local identity/shared SPI, unavailable fastpath fallback,
  source/destination/TOS preservation and malformed/unwatched controls reviewed.

Checkpoint: **29/400 population paths have full source/hunk reads**, listed
exactly in `deployment-complete-source-read-paths.txt`. This is not 29 accepted
requirements: independent runtime proofs and remaining producer/caller/test
relationships are still owed. The other **371** paths remain untouched or only
partially read as described next. No population path was dropped.

Partial producer reads, not completed-file coverage: `xfrm_linux.go`
(historical Spec-work diff, pinned InstallSA, RemoveSA and Close);
`xfrm_migrate_linux.go` (pinned MigrateTunnel/followMove and symbol map);
engine `application_override.go`, `child.go`, `create_child.go`,
`reconcile.go` (historical changed hunks; current producers/callers owed).
Backend.mu serializes migration holds against Close. XfrmStateAdd precedes
receiver Watch, preserving an existing SA watch on duplicate-add refusal.

## Precise continuation boundary

First finish the partial XFRM lifecycle bucket: pinned
`xfrm_migrate_linux.go` lines 1-144, 288-376, 445-end and its complete historical
diff; other changed producers in `xfrm_linux.go` outside InstallSA/RemoveSA/Close;
then `xfrm_linux_test.go`, `xfrm_migrate_linux_test.go`,
`xfrm_state_del_failure_linux_test.go`, `xfrm_migrate_integration_linux_test.go`,
`xfrm_teardown_integration_linux_test.go`.

Then foreign IKE integration and recording seams: `encap_dualform`,
`encap_formchange`, `encap_hybrid`, `rfc4301_override_guard`,
`unit_capabilities` integration Linux tests, `recording_shipped_test.go`,
`recording_zetest.go`, `register_recording_zetest.go`.

Then the engine half of `deployment-closure-foreign-ike-review-population.txt`
(all engine paths, including the four whose changed hunks alone were read),
doctor/capability/systemd/runner/QEMU wiring, firewall observer/registry/table
registration, stock producer GTSM/DDoS/trafficusage changes and associated tests.
All remaining non-IKE paths of the full 400-path native classifier population
still require their historical Spec-work hunks and pinned producer review.
The old 395 list plus the five additions above defines this full population.

Later root commits require delta review as well, including the already assigned
IS-IS transaction and transport fixes and real-daemon rollback proof. The
prose-only 02b14b3c78 routing commit is not code acceptance.

## Findings

No whole-population verdict yet. Population arithmetic corrected from 398 to
400 and reported to root. Known pending proof/implementation remains pending.

### Source-bounded unrelated finding: complete-path limiter

PLAUSIBLE, not executed: `espFormLimiter.allow` resets a drained bucket to
10,000 after one second despite the 5,000 token/second declaration. Initial
10,000 at t0 plus another 10,000 at t0+1s exceeds the token-bucket allowance
15,000 at that time. Existing `TestESPFormLimiter` counts the half-second
refill but checks only one token after the full second. A deterministic
regression should count the whole one-second refill after draining t0.
No RFC violation or measured throughput is claimed. Root classified this as
outside the deployment acceptance criteria and will route it to its IPsec
owner rather than expanding the current implementation. No code was changed.
The full body predates this spec (blame `1963345b4eb`); pinned/current body is
unchanged, producer `espform.go:187-200`. Root subsequently reports routing
this source-only note in `02b14b3c78`, a new journal plus an owned row in
`spec-ipsec-esp-dual-form-receive`. That prose-only commit leaves the source
population and this code pin unchanged.

### Test-only NOTE: transient reopen failure assertion

Disposition correction, 2026-11-13: WITHDRAWN at630 and current HEAD
c1522b8ad4bfc8e6d10b841d062512f12fa2d529 plus working tree. The test already
captures r.failure into failureBeforeSuccess while openPaths holds r.mu on
the third attempt, before success clears the failure. Its later assertion
reads that captured pointer. The historical note below overlooked this seam;
it is not a remaining plausible timing defect and needs no journal/fix/test.

Not executed: `TestESPFormReceiverReopensAStoppedReaderByItself` sets a zero
minimum reopen delay. The notification for open 2 precedes publishing its
failure; the test waits for the receiver mutex but then releases it. Open 3
can clear the failure before the assertion reacquires that mutex. A channel
barrier before returning the third open could preserve both assertions and
remove that schedule. This is a possible false failure in an unrelated IKE
test, not a product defect or evidence that any acceptance criterion passed.

## Second source checkpoint: 120/400

This checkpoint supersedes the first checkpoint's partial XFRM/engine boundary.
All **120** paths in `deployment-closure-foreign-ike-review-population.txt` now
have their historical exact Spec-work hunks and pinned changed/reached producer
bodies reviewed through runtime/security ownership, tests/wiring/edges, and
style/contracts lenses. The exact completed set is
`deployment-complete-source-read-paths.txt`; the exact **280** remaining paths
are `deployment-complete-source-remaining-paths.txt`. Neither count is an
acceptance-criterion pass or a CLEAN judgment. Newly added files were read in
full; unrelated unchanged branches of existing files were not treated as a
requirement to reread. Truncated outputs were recovered in bounded regions.

Actual additional reads, beyond the first 29:

- XFRM lifecycle (7 paths): all changed InstallSA/RemoveSA/Close producers and
  reached state construction; complete MigrateTunnel/rollback/hold/registry
  producers, five regression files' changed bodies and fixture edges. The
  new hold precedes SAD mutation, failure releases it, tuple identity stays
  old until successful kernel migration, and irrecoverable rollback forgets
  the watch while retaining PROTECT. Tests preserve positive old ownership,
  mutation-failure controls and teardown assertions. No new defect found.
- Remaining dataplane integration/recording (8): dual-form, form-change,
  hybrid and override-guard historical hunks and reached fixture functions;
  unit-capability helper file, recording tags and registration. Distinguish
  manual reinjection from the actual production receive proof. Form-change
  uses routed nonlocal addresses and exact same-SA identity. Typed IPConn
  statically replaces the former non-IP fixture. See construction findings
  below; no compile or runtime success inferred.
- All engine paths (21): child refusal classification and cleanup/backoff,
  initiator/responder integration, immutable atomic refusal/TUN snapshots,
  apply-only TUN recording, Stage-1 doctor declaration/answer/sink plumbing,
  firewall hold observer and table controls. Full new doctor source/tests
  (976/1332 lines) and real XFRM policy integration (1108 lines) were read,
  including fragments, shared SPI/local identity, ECN ancillary witness,
  foreign firewall rule and owned-table remedy metadata. Operator commands
  constrain/quote values; no new RFC claim or runtime result is made.
- Firewall (7): full new observer and tests, all ApplyAll mutation wiring,
  reservation owner/control atomicity and full changed tests/registrations.
  Holds encompass the actual apply and release on failure/panic. Observer
  list is copied before callbacks; registration validation precedes writes.
  Removal, nil-after, no-apply and table ownership cases remain represented.
- Other table producers (8): GTSM, anomaly, CoPP, DDoS, flowspec, interface,
  policyroute and VRRP changed registration functions. Prior owner/name
  constants remain; all three control fields are supplied, with VRRP's four
  tables sharing the same operator control. No new unowned table admitted.
- Shared privilege/service/doctor/code wiring (10): full new capability
  declaration and copy test, shared cap probe, exact service ambient/bounding
  output, startup warning, daemon doctor registration/test and changed code
  catalog entries. Four capabilities are shared by actual unit and harness;
  missing/unreadable privilege state gives an actionable finding. Existing
  root bypass is not newly presented as a capability sufficiency proof.
- Stock GTSM/DDoS producers (6): changed protocol-first match ordering and
  corresponding expectations; mitigation table extraction and production
  apply/registry failure edges; complete new real-datagram drop integration.
  Rule counters and alternate-port positive control support the negative
  delivery assertion. No widening of the matched victim/protocol was found.
- Traffic accounting (6): full changed program producer plus removed
  historical instructions, full changed test bodies and helper functions,
  complete TCX chaining and unit-capability integration files, and doctor
  fix/message changes. IHL remains scalar for helper reads; whole L4 bounds
  refuse into port zero, every path returns TCX_NEXT, existing byte counts
  remain. Truncated-length coverage 14..33 moves to actual TCX/veth with
  affinity/sentinel ordering; equality at 34 and whole/short L4 boundaries
  get explicit controls. Non-root helpers verify exact CapEff before actual
  production Available/Attach. Reached unchanged attacher cleanup was read.
- Interop/runner/QEMU/service-test plumbing (18): full new firewall-drop
  checker/tests and TUN declaration; all changed peer grant/setup functions,
  source-derived device scanner, runner grant/gate, QEMU guest-node guard and
  call sites, evidence/service-unit expectations. TUN access is explicitly
  given to IPsec peers; scoped writable sysctls use the existing generic
  network namespace profile. Guest chmod is independently guarded per node
  by the 9p /workspace mount; no production host chmod is introduced. The
  firewall checker requires disagreeing forms, positive directed ESP control,
  actual outbound stimulus, all-loss, no inbound-SA movement, drop counter
  increase and no mismatch movement. Reached counter parser/filter, wait,
  ping summary and CLI helper bodies were read as sibling caller context.

Next: start the 280 remaining paths with native command dispatch and its
subdispatch help behavior, then local installer/uninstaller and systemd
trust/ownership/unit lifecycle. Follow with storage/SSH refusal, appliance
ISO/OTA/vendor patches, preview/runtime images, netlab/IGP and website/code
data paths. Later root commits still require separate delta review. The
new IS-IS transaction/transport/callback changes are not covered by the630 pin.

### Pinned recording constructor: current source remedy already present

Source-only compile classification, no execution: `recording_zetest.go`'s
new constructor at630 keys promoted `policies` in a literal of a struct
embedding `recordingDataplane`. Go keyed struct literals cannot name promoted
fields. The current working-tree diff already nests the value under an
explicit `recordingDataplane: recordingDataplane{...}` initializer; root
acknowledged this remedy belongs to ongoing work. No duplicate edit or test
was requested. The build tag is `zetest`, so this is distinguished from
shipped-build behavior and root's unrelated current daemon build blocker.

### Grouped unrelated netlink fixture constructor class

PLAUSIBLE source-only/unexecuted compile classification. At630, these new
Linux fixture literals use promoted LinkAttrs fields as keyed literals:

- `dataplane/espform_fastpath_gate_linux_test.go:55` (`linux`), Dummy.Name.
- `dataplane/espform_fastpath_integration_linux_test.go:115,390,393,396,407`
  (`linux`), Dummy/Veth/Tuntap/Iptun.Name.
- `dataplane/encap_formchange_integration_linux_test.go:78` (`linux`), Dummy.Name.
- `engine/rfc4301_espform_policy_integration_linux_test.go:171` (`linux`),
  Veth.Name and MTU.

Paths above are relative to `internal/component/ike/`. Vendored netlink
LinkAttrs declares Name/MTU; Dummy, Veth, Tuntap and Iptun embed LinkAttrs.
Current source equals pin for the first three paths when checked; no current
equivalence claim is made for the fourth without a later delta check. Root
has this grouped construction class and will route it to the existing IPsec
owner; it is outside the original deployment/IGP acceptance tests. No new
investigation/test/fix is opened here, and these files remain reviewed in
the full population rather than being excluded. This is distinct from the
foreign sysrib route_view_output blocker in root's actual daemon build.

## Third source checkpoint: 139/400

An additional 19 paths now have all historical selected hunks and pinned
changed/reached producers reviewed through the same three lenses. Exact
completed/remaining ledgers now contain **139 / 261** unique population paths.
This checkpoint adds:

- Install/uninstall dispatch and native ownership API (3): default ownership
  is registered, nil/duplicate registrations panic only on internal misuse;
  service Preflight/Stop/Install/Restore ordering is explicit in the contract.
- Subdispatch producer and two full new tests (3): bare/options routes,
  explicit targets, help and unknown-target separation, generated default
  options and help header. Reached textbuf.String and helpfmt.Page.WriteTo
  were read: String empties the buffer, so reuse for the header is valid.
- All local-plugin population paths (10): full new native lifecycle,
  replacement/recovery and four full new native test files; changed explicit
  local command bodies, atomic copy producer, registration and package test.
  Paths are preflighted before native mutation, package-owned shared state is
  guarded on default install/purge, service state is leased for rollback,
  maintenance ownership fences binary replacement/removal, existing private
  credentials and inactive service policy are preserved. New mode/ancestor
  and isolated package-state positive controls were read. The explicit-local
  bypass below is a newly reported PLAUSIBLE deployment issue, not CLEAN.
- Init registration/profile/test (3): full changed bootstrap producer and
  its publication/population closure, new 39-line test, owner registration.
  Loopback-only host and nonzero 16-bit port validation precede publication;
  local profile seeds only SSH and credentials, preserving ordinary/seed/
  managed profile behavior without taking over discovered host interfaces.

Precise next bucket: **systemd native service ownership and trust**. Symbol
map for `internal/plugins/systemd/native.go` read, no producer body yet:
Available28, Preflight47, Stop65, Install96/100, Restore104,
installSelected111, Uninstall136, refuse143, compatibleUnit150,
matchNativeUnit209, dropins228. Read this full new 258-line source plus new
native_owner_linux.go/other.go, native_trust.go, native_unit.go,
native_review_test.go and native_test.go; then historical changes/reached
bodies in cmd_install/cmd_uninstall/doctor/main/register/service_test and
uninstall_stop_test. The already reviewed unit.go/unit_test.go remain in the
139 completed ledger. Afterward continue exact remaining-path ledger rather
than treating earlier navigation reports as coverage. Runtime proofs,
late root commits, complete spec mapping and ze-close remain owed.

### Deployment ISSUE candidate: explicit-local purge bypasses shared package state

PLAUSIBLE, source-only/unexecuted. `internal/plugins/local/cmd_uninstall.go`
lines42,50,62-69 maps `/usr/local/bin/ze` to `/etc/ze`, checks only the selected
binary prefix's package marker, accepts an absent binary, then RemoveAlls
the config directory. `packageOwnership` derives only
`/usr/local/share/ze/install-method`; the canonical
`/usr/share/ze/install-method` check added by554 is in
nativeTarget.stateOwnership and is called only by default native paths.
Pinned/current diff of cmd_uninstall.go and native.go was empty when checked.

Concrete counterexample: package marker `/usr/share/ze/install-method` and
private package credentials `/etc/ze/database/credentials` exist; no marker
under `/usr/local/share/ze`; run
`ze uninstall local --prefix /usr/local --purge --yes`. The changed explicit
local package guard permits this path, including when no standalone binary
exists, and the package credentials are removed. docs/guide/ze-install.md
continues to advertise explicit local removal and states shared package
state is protected. This reaches the deployment ownership objective and
differs from the unrelated IPsec notes. Root received the counterexample
and owns isolated reproduction (reuse existing userns/chroot fixture shape)
before repair and focused regression. No root execution outcome is inferred.

## Fourth source checkpoint: 267/400

128 further paths completed at the same630 code pin, with all historical exact
Spec-work hunks, pinned changed/reached producers and three lenses. Exact read
and remaining ledgers now contain **267 / 133** paths. No path was excluded;
source-reviewed files can still contain the PLAUSIBLE findings below. This is
not a CLEAN verdict or acceptance of root's outstanding runtime evidence.

- Systemd (14): entire new native/owner/trust/effective-unit files and new
  tests; all historical changed existing installer/uninstaller/parser/runtime/
  registration/test hunks and reached bodies. Root ownership and nonwritable
  executable ancestry, masks/vendor/runtime unit refusal, effective fragment
  and ordered drop-in matching, indirect config/execution refusal, compatible
  administrator policy, inactive service preservation, prior-active recovery
  and stop-before-delete reviewed. No new finding in this bucket.
- Storage (8): full new descriptor-safe maintenance and regressions; all
  lock sync-removal hunks, retained inode/shared-reader assertions and O_PATH
  integration. Reached ownershipOpenFolder/ownershipLock, openLive/openTree,
  lockStoreFile/openLockNode and blob acquisition bodies read. Coordination
  retains the same live kernel lock, while data/publication barriers remain
  unchanged. Root maintenance does not become a store-data opener.
- SSH/client (9): full new host trust producer and four test files, typed
  refusal preservation test and changed dial/credential producers. Reached
  credential resolution, no-prompt policy and net.JoinHostPort paths read.
  Known/hashed/custom-port/IPv6 trust is checked; unknown, changed, revoked,
  missing/malformed files refuse without enrollment. Repair strings preserve
  shell and OpenSSH argument boundaries. Real handshake and OpenSSH fixture
  tests retain both positive identity and changed-key controls. A standalone
  downloaded binary uses explicit credentials without treating empty store
  location as the working directory. No test was run here.
- Plugin runtime cleanup (3): historical hunks, changed cleanup body, both
  dispatch drain paths, restart release ordering and full new report race
  regression. Reached process once/done implementation read. Reports raised
  after manager removal clear after admitted calls drain; old repeated cleanup
  cannot clear the replacement generation's state.
- Runner/stress contention (7): all historical hunks/new tests and changed
  batch/environment/factor producers, reached ChildParallelFactor, budget
  publisher and effective concurrency selection. Nested contention propagates
  maximum once, final isolated batches stay tight, malformed parent values
  retain floor1; explicit operator invocation timeout is retained.
- Preview (18): all new source/test files in full and the later phase-binding
  historical removals. Compiler-selected inputs, starting provenance, both
  target ELF stamps/features/static settings, exact public archive population,
  checksums, source revalidation before images and per-role staged recipe
  binding reviewed. Full719-line Docker archive producer and440-line test
  file read: OCI/config/graphdriver identities, ordered DiffIDs, metadata
  ownership, platform/attestation separation, unreferenced content, path/link/
  graph limits, layer gzip bounds and no extraction reviewed. One outer-tar
  sparse expansion issue remains PLAUSIBLE below.
- Docker runtime image seams (6): full new staging, stage identifier and
  immutable SaveImage producers/tests plus historical Build/OwnedTag changes.
  Reached Build lifecycle read. Only caller-owned recipe/binary enter staged
  context; descriptor identity and executable mode checked, stage is one argv,
  immutable IDs survive export, only the unique retained tag is released.
- Compiler/registration (6): full new daemon/input selection and tests plus
  registration hunks. Reached shared execute read. Both Linux target source
  populations include embeds/untracked files, reject external/incomplete
  inputs, share tags/compiler policy and strip ambient overlay/workspace/ISA
  tuning; compilation and source listing share that environment.
- Appliance ISO/OTA/build and le gokrazy (36): all historical hunks and new
  files/tests in full, pinned changed producers and relevant existing callers.
  ISO selected target/module directory flows to the actual GRUB argv; distro
  repairs retain target and appliance selection. Optical media enters source
  scanning while source exclusion and optical target rejection remain. ISO
  finalization synchronizes, rereads partitions, and only then logs completion.
  New explicit-builder prerequisite issue below. Named builds read existing
  token/certificate/key, store them0600 only in disposable private prepared
  state, clear transferred key bytes, restore working directory/architecture,
  and distinguish helper completion from an image. Actual embedded failure
  and helper test controls were read. OTA parses/bounds GPT/FAT/SquashFS before
  network access, streams root/bootonly/440-byte MBR in order, stops after any
  upload failure, refuses redirects and bounds the transaction. CA device SAN
  addition and exact payload/auth/sequence/retained-state controls reviewed.
- Gokrazy vendor/runtime code (17): all selected historical hunks, full new
  regressions and current changed/reached bodies. URL credentials are added
  after authority parsing, printed URL is redacted, errors return through gok
  instead of exiting, buffered DNS result cannot strand its sender on failure,
  helper fd/process owners close/join on returns and pure helper classification
  rejects joined cleanup errors. Packer preserves cmdline padding+newline;
  runtime testboot edits stay inside retained FAT extents and are idempotent.
  Reboot acceptance flushes before interface-owning service shutdown, while
  power waits for shutdown. Reached upstream updater.Reboot returns on200
  headers without consuming the blocked body, matching that contract.
- Vendor normalization/launcher tests (4): full new runtime comparison220L
  plus wrappers and launcher77L; reached reverse-check helper and pinned ze
  launcher read. Both runtime file populations/content and all required pins
  are compared, excluding only explicit top-level docs trees. Fresh-launch
  fixture removes host-target leakage and requires patched compiler selection.

Next exact bucket: internal/le/test/netlab population, beginning image/runtime
identity, lease and live lifecycle; then remaining QEMU install/OTA harness,
IGP, deployment fixture, site/data and small tool/dependency buckets. No new
file from those133 paths is credited yet. Later root commits require deltas.

### Preview ISSUE candidate: sparse outer Docker entry hashes before any bound

PLAUSIBLE source-only/unexecuted. Current docker_archive.go equals630 when
checked. scanImageArchive lines230-239 accepts tar.TypeReg and calls unbounded
readerSum(tar.Reader) before classifying entry metadata/layer or applying
layerSizeLimit. A PAX sparse entry has a small physical body but a huge logical
Size; tar.Reader materializes holes. Thus a tiny outer Docker archive can force
arbitrarily large hashing before any later metadata/graph/layer rejection.
The ordinary checkArchiveWithin already explicitly recognizes and guards this
logical sparse size, but scanImageArchive does not. Local stdlib tar reader
source confirms PAX sparse records install a sparseFileReader while retaining
the regular entry type; no compiler/runtime execution was used for this note.
Root received the exact producer and owns a bounded isolated regression before
repair. Do not run an enormous sparse witness without a strict bounded harness.

### ISO ISSUE candidate: explicit missing builder can pass prerequisites

PLAUSIBLE source-only/unexecuted, cmd_iso.go/iso_modules.go current equals630
when checked. checkISOPrerequisites206-274 directly uses a nonempty
opts.builderPath and asks only for its modules. It never reaches the
resolveExecutable admission used by actual resolveISOBuilder577-629.
With valid kernel/initrd/xorriso and a real matching module directory,
`iso --check --builder /prefix/bin/missing-grub` can return0 and print ready
although that executable is absent or nonexecutable. The actual build then
refuses the same selected builder. A bounded regression can seed a private
prefix/lib/grub/<target>/modinfo.sh plus other prerequisite seams, omit the
builder, and require prerequisite refusal; retain a real executable positive
control. Root received this counterexample and owns reproduction/repair.

## Source-preparation checkpoint: 302/400 at immutable 6307518170f6ddc66198b3e956ec683952673c95

This continuation reviewed163 additional paths beyond139: the128 recorded
above and the following35. Exact read and remaining ledgers now hold302 and98
paths, with no population exclusions. This is partial source preparation,
not a CLEAN Review Gate or completion claim. All credited paths received the
three lenses: runtime/security/ownership, tests/wiring/edges/removal, and
style/contracts. Historical exact Spec-work hunks and changed/reached pinned
producer bodies were reviewed; new files were read fully. No tests, builds,
formatters, lint, production commands, or tracked edits were performed here.

- Netlab21: all remaining native-classifier paths under internal/le/test/netlab
  (exact names in the read ledger), including all historical scoped removals
  and revised run/startup ordering. Reached netlab discovery, build/copy,
  contrib/netlab/ze.yml, initial.j2 and guide contracts were read. The retained
  owned Docker tag is bound to the expected image ID before startup. Both
  image roles consume the staged runtime only. An exclusive directory lease
  rejects existing native pickle/YAML/containerlab state before startup or
  cleanup. Startup creates the owned snapshot before initial configuration;
  initial:sh runs inside the container after interfaces are ready. Live proof
  checks the three reference nodes, mandatory protocol counts/states, static
  FIB routes and all six directed sourced loopback pings. Diagnostics precede
  teardown under fresh bounded contexts, private files, essentials-first
  ordering and explicit failure artifacts. Teardown errors fail the verdict.
  Package creation preserves existing operator state. Failure/cancellation,
  snapshot/name/lease, image identity and runtime/startup tests were read.
- QEMU14: remaining actions/alltests/install boot/build/ISO/OTA and firmware,
  poweroff, health, progress, deadline, protocol and SSH-stream regressions.
  Clean QEMU process exit proves ISO poweroff; completion text alone cannot
  pass. Split UEFI CODE is readonly and VARS is a private snapshot, including
  installed-image second boot. ISO-contained source hash, first four GPT
  entries, full untargeted-disk hash and authenticated runtime identity are
  asserted. The serial sink remains bounded and its completion process joins.
  Appliance arch/SSH identity is written even when arch already matches.
  OTA builds two distinct images, authenticates upload/testboot and fresh SSH,
  checks persisted hostname and unchanged permanent UUID, then reboots to the
  original root and repeats authenticated identity/config/health assertions.
  Independent upload/reboot timeouts do not inherit short status deadlines.
  SSH cancellation closes transport and joins its watcher; stdout JSON stays
  separate from stderr diagnostics. Health validation rejects malformed or
  inconsistent/missing mandatory rows and permits exact inactive L2TP/IPsec
  reasons only when absent from the parsed seed. Pinned health producers were
  read to confirm those exact reason strings. Real SSH/TLS protocol fixtures
  retain their stubbed VM/build/push scope; they are not an appliance VM pass.

No additional finding arose in these35 paths. Root still owes actual netlab
and extended OTA/ISO/runtime receipts as applicable; older baseline evidence
retains its original scope. Preview sparse-hash and ISO explicit-builder
candidates above remain PLAUSIBLE/source-only, awaiting root reproduction.
Explicit-local purge is already assigned elsewhere; do not duplicate it.

Exact remaining98: IS-IS7, OSPF19, site35, deployment fixtures14, website5,
small tooling/interop18 (including the3 IGP-loopback interop paths). Next
bucket is pinned IS-IS then OSPF and loopback interop. Later implementation
commits need independent deltas after the pinned population; none is silently
covered by this checkpoint. Root must explicitly clear closure preconditions
before ze-close or any formal CLEAN record begins.

## Pinned source preparation: 400/400 reviewed; acceptance still OPEN

This bounded continuation added the exact remaining98 at immutable
6307518170f6ddc66198b3e956ec683952673c95. Read ledger400 was compared as a
sorted exact set with the native IsReviewCode classifier over all46 historical
exact Spec-work commits; equality holds, with no foreign-path exclusion.
Remaining pinned-path ledger is empty. The prose-only47th commit adds no code.
All credited paths received runtime/security/ownership, tests/wiring/edges and
style/contracts judgments over their historical scoped hunks and changed or
reached pinned producers. New files were fully read. Truncated broad output
was recovered through smaller scoped reads before coverage was credited.

- IS-IS7: Config.EnabledCircuits/interfaceLevel, validateInterfaceLevels,
  startup and verify callbacks, reconcile's no-NET lifecycle, doctor metadata
  and interfaceSnapshot. New503-line level_scope_test was fully read, including
  actual SDK startup/verify, no-NET activation/removal, actionable diagnostics,
  L2 adjacency/own LSP/SPF reachability and positive controls. Reached origination,
  level-state and circuit lifecycle producers were read. Historical rollback
  and transport ownership defects remain root's already assigned fixes; this
  pinned review does not claim their later implementations were reviewed.
- OSPF19: coherent single-read IPv4 inventory, secondary distinct stubs, exact
  loopback host costs, per-area preservation of Router/Network LSAs on unknown
  inventory, and shared instance-safe warning lifecycle. Full new warning,
  inventory, topology and rollback test files were read. Warning publication
  occurs off topology locks with retryable raise/clear acknowledgements, old
  runtime admission guards and joined shutdown. ConfigTx's complete typed
  runtime/export value and the actual startup/apply/self-undo/rollback installer
  were traced into instanceManager and SDK Apply/Rollback. Replacement export
  test preserves once-only exact identity and absent-apply behavior. Scoped
  historical fixture changes retain original event/assertion paths; their
  independent approval receipts remain root-owned. RFC2328 own text around
 12.4.1 was read before judging loopback-zero-cost semantics. No new RFC claim
  or feature obligation was invented. Real-daemon and FRR proof still owed.
- IGP-loopback interop3: full new checkers, registrations and scripted failures.
  Reached requireRouteInstalledVia requires the same FRR entry's exact prefix,
  protocol, selected/destSelected/installed and matching active FIB next hop.
  Ze exact-prefix proto250 route plus each source-bound ping are independently
  required. Docker.Exec returns nonzero exits as errors. These fixture tests
  cannot substitute for the four root-owned real-peer RED/GREEN runs.
- Deployment fixtures14: full new private two-daemon quickstart, standalone
  strict remote trust, native installation lifecycle, readiness, budgets and
  registrations plus three .ci callers. Quickstart checks both actual received
  RIBs and authentication rejection, then clean shutdown. Remote trust tests
  unknown/changed keys and bad passwords with precise refusal, valid JSON
  success, immutable selected/default trust files and no client store. Password
  diagnostics are redacted and owned processes joined before cleanup. Native
  lifecycle checks installed daemon login, preserved config/login on reinstall,
  busy refusal, ordinary retention and confined purge; runtime proofs stay root's.
- Site35 and authored website5: all historical exact scoped hunks and changed
  pinned producers reviewed. RFC wording retains counters/markers, optional/
  unreviewed exclusions and partial/fault-record semantics; the reached browser
  JS reader uses the same new strings. Docker guide is in the manifest and
  homepage journey links reach published destinations. Milestone changes retain
  all key points and every report, stable newest-first grouping and HTML/mirror
  parity; invalid reports refuse publication. Ancillary change-week grouping,
  filter JS, CSS tokens and Website Sources contract were read. Authored install
  lab discloses missing public images and scoped passing/skipped boot evidence;
  interop command and prerequisite guidance reaches the actual tool grammar.
  Source review does not claim fresh site build/browser or copy-paste evidence.
- Remaining tooling15: full new dependency-policy and negative tests, current
  commit/deps admission bodies and prospective boundary callers reviewed.
  HEAD ordinary modules permit provider-exact reuse; circl retains exact use
  approval, uncarried module renames/boundaries cannot grant allowances and new
  nested providers cannot borrow a parent. Exact progress-message exclusions
  preserve error admission on altered site/literal/shape/operands. Named-launcher
  extraction and preview workflow wiring preserve manual, read-only artifact
  preparation. The reached workflow has no release or registry write action.

No new product issue arose in routing, fixtures or the smaller tooling. One
additional source-bounded public quickstart concern follows. Formal acceptance
remains open for this concern, the previously reported reproduction/fix work,
all later source deltas, source/spec/story/docs reconciliation and root's actual
runtime receipts. No tracked edit, test, build, formatting/lint, production run,
CLEAN record or ze-close was performed by this reviewer.

### Public quickstart: fixture redirects a shared resolver output

PLAUSIBLE/source-only/unexecuted. Shared-store concern is CLEAR: cmdStart
resolves the explicit config path and openExplicitStore calls StoreDir, whose
explicit-file parent outranks ZE_CONFIG_DIR (core/resolve/resolve.go32-43).
Both supplied configs omit system dns/resolv-conf-path. ExtractSystemConfig
(component/config/system/system.go189-200) selects /tmp/resolv.conf; hub/main.go
1097 passes it to ProcessResolvers.Configure, whose refresh/flush reaches
resolv_linux.go:writeResolvFile18-34. That producer writes the common .tmp name
then renames /tmp/resolv.conf. Both public peers therefore write shared output
outside their copied lab directories. The functional prepareQuickstart changes
that leaf to a private per-peer file, so its pass cannot prove this literal
public behavior. Failure only warns in hub; no confirmed startup failure is
claimed. Homepage, both configs and the default/writer producers equal630 when
checked. Root received the exact trace and a bounded isolated-mount sentinel
reproducer suggestion; do not mutate the host's existing resolver file while
proving it. Root confirmed no literal unadapted public-command pass exists.
If the lab should leave shared resolver state alone, disabling that output in
the supplied routing-only configs and aligning fixture/public semantics is one
possible repair after reproduction; this reviewer made no implementation choice.
