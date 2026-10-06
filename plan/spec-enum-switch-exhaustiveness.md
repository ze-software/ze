# Spec: enum-switch-exhaustiveness

| Field | Value |
|-------|-------|
| Status | in-progress |
| Owner | Thomas |
| Scope | tooling |
| Depends | - |
| Phase | 4/11 |
| Handoff | plan/handover/enum-switch-exhaustiveness-checkpoint.md |
| Updated | 2026-10-06 |

## Task

Make enum coverage explicit without turning unexpected input into silent success or a daemon panic. The initial scope was analysis and this spec only. Thomas subsequently authorized implementation after the BGP and startup-race verification repairs; that implementation starts here.

### Owner requirement, 2026-10-04

> Task: make every Go switch over an enum list every value, then turn off
> `default-signifies-exhaustive` in .golangci.yml so the exhaustive linter
> enforces it for all code. Owner: Thomas. Run it through /ze-spec first
> (non-trivial, repo-wide sweep).

> A default arm can stay, but only for values outside the named set.

| Class | What the switch is over | Fix |
|-------|------------------------|-----|
| Closed, internal | a value only Ze produces | List every value. Keep a `default` only as `panic("BUG: <what>")` |
| Open, from outside | a code read from the wire, a file, config or a plugin | List every named value. Keep `default` as the handler for unknown codes, with a comment that the set is open. Never panic: a peer MUST NOT be able to panic the daemon |
| Intentionally partial | e.g. a few attribute codes out of a large registry | `//exhaustive:ignore // <why this switch handles a subset>` |

The table is Thomas's requirement, verbatim. The inventory uses C, O and P for these classes. P is the explicit subset exception, not permission to hide an incomplete complete-dispatch function. A complete state/event transition table is C or O even when some named events intentionally do nothing; independent event observers can be P.

### Owner completion requirement, 2026-10-05

Thomas expanded completion after the implementation checkpoint:

> Make sure that we end by ensuring the full linting and fixing of the bugs identified.

The identified defects are now repair scope, not journal-only closure items.
Completion requires the complete native lint matrix to pass, regression and
real-entry-point evidence for the identified bugs, and renewal of affected RFC
proofs and audit judgments. This overrides the earlier preservation-only
boundary where repairing an identified defect changes existing behavior.
Thomas also requested additional parallel agents to accelerate these repairs.
Thomas subsequently included the VPP SRv6 SR-policy and local binding-SID
lifecycle, with real VPP forwarding proof required before closure. This is the
specific exception to the unrelated-feature boundary; it does not authorize
SRPM, remote decapsulation or a new BGP tenant-table producer.

Thomas also authorized preserving the historical VRRP shell recorder verbatim
as non-executable Markdown, updating its references, and leaving the
no-interpreted-source guard unchanged.

### Scope boundaries

| Included | Excluded |
|----------|----------|
| First-party enum value switches, including old code, tests, platform builds and existing exhaustive suppressions | Hand-written third-party analyzer changes, string-to-enum migrations, map exhaustiveness and sealed-interface type-switch tooling; the owner separately authorized the supported tool upgrade recorded below |
| Named zero values, sentinels, bit fields, aliases and test-only enum members; the separately authorized VPP SRv6 policy lifecycle | New algorithm, protocol, family or plugin support outside that explicit authorization |
| Existing named-value behavior; safe unknown-input behavior; BUG assertions for proven internal impossible values | Changes to wire formats, supported algorithms, routing policy or authentication policy |
| Style-guide correction and global linter cutover | A new bespoke switch checker or opt-in enforcement mode |

## Required Reading

- [ ] `docs/contributing/ze-go-style.md`, Types that cannot lie and the linter table.
  → Decision: replace the no-default wording with Thomas's three-class rule; the enum rule applies to old and new code.
- [ ] `docs/contributing/running-commands.md`, The lint gate; `docs/contributing/testing.md`.
  → Constraint: use the registered le action and its build matrix, not a host-only bare linter command.
- [ ] `vendor/github.com/nishanths/exhaustive/{doc.go,switch.go,common.go,enum.go}`.
  → Constraint: completeness is by distinct constant value, with only accessible members required across package boundaries. The analyzer does not validate input provenance or subset reasons.
- [ ] `internal/le/go/lint/{actions.go,verifylint.go,matrix.go}`.
  → Constraint: the action owns admission, build selection and tracked-file coverage. Its current CLI has only a package-scope selector.
- [ ] `ai/rules/{evidence,go-standards,testing,rfc-compliance,documentation,commands}.md` and each changed file's design page.
  → Constraint: prove a closed producer before adding a panic; retain protocol guards and read applicable RFC text before changing a protocol obligation.

**Key insights:** an unknown wire number is not an internal invariant violation. A named zero is not necessarily invalid. A useful default must not exempt missing named values. A reasoned subset switch is different from a complete dispatcher.

## Initial Behavior (analysis snapshot)

| Producer read | Observed behavior |
|---------------|-------------------|
| `.golangci.yml`, exhaustive settings | `default-signifies-exhaustive` is true; report caps are 50 per linter and 10 identical messages |
| `switch.go`, `switchChecker` | A default suppresses missing-member findings globally; enforce comments do not override that setting |
| `actions.go`, `runHere`; `verifylint.go`, `newRunner` and `passPlan` | le reads the checkout config and runs its platform/tag matrix; no alternate-config or linter selector is exposed |
| `common.go`, `checklist.add` and `checklist.found` | In-package private constants count; cross-package private constants do not; one case covers same-valued aliases |
| `internal/le/hookruntime/writeedit.go`, `writeGoPatterns` | Existing panic guard permits BUG-prefixed literal assertions; it is not an enum-exhaustiveness checker |
| First-party marker/check search | No enforce markers or bespoke closed-switch/default checker found in the inspected source, hooks and ruleguard rules; do not delete unrelated enumeration/config gates |

### Measurement

The exhaustive-only run exited 1 because it found missing cases, not because analysis failed. It executed 18 passes and reported 487 diagnostics: **237 distinct switch locations, 167 files, 86 packages**. Host reported 226; Linux integration reported 237. Other passes repeated locations already in that union. No type-check diagnostics appeared.

Command: `env PATH="<session-scratch>/exhaustive-bin:$PATH" CGO_ENABLED=0 ./le --name enum-switch-spec go lint run`.

The session-local adapter ran the installed golangci-lint with `--enable-only exhaustive`, an alternate config and per-pass JSON output. The config copied `.golangci.yml`, set `default-signifies-exhaustive` false, disabled both issue caps and retained feature tags. A second scratch copy omitted tags for le's compile-out passes. le still chose packages, targets, tags, resource limits and job admission. The root config was not edited.

Tool: golangci-lint v2.13.1, exhaustive v0.12.0, built with Go 1.27.0. HEAD observed after the run was `a829eb68d15cc2bd2cbaad8a0782e9f52b8168a1`; the run inspected the shared working tree, not an immutable checkout of that commit. Deduplication key: filename, line and column. Package counts below derive from those distinct findings.

Research artifacts are under `tmp/session/2026-10-02-ba93202e-6f62-48a4-9b4e-c0aa37a4cc74/scratch/`: `exhaustive-run.log`, `exhaustive-measurement.json`, `exhaustive-findings.json`, `exhaustive-package-counts.json`, both configs, per-pass JSON and classification reports. The source locations, findings counts and classifications needed to implement are retained below; full per-switch producer evidence and proposed actions are also in exhaustive-inventory-final.json in that scratch directory; the scratch directory is not the acceptance proof for the finished change.

### Coverage limits and bypasses

- The registered action reports three tracked-file coverage exceptions: `.golangci/ruleguard/modern.go`, `examples/plugin/go/main.go` and `tools.go`. These exceptions are not evidence that their source contains no enum switches. Inspect their source at cutover; do not broaden this work into fixing the separate-module/tool-pin infrastructure.
- Standard golangci generated-file filtering remains active. Source research found two generated value switches, both over built-in string fields in web templates, not typed enums. Reconcile generated sources again at cutover; edit a generator rather than its output if an enum switch appears.
- Existing `nolint:exhaustive` directives hide **117 sites in 80 files** from the diagnostic run. Their classifications are included below. Remove these directives: use named cases for C/O, or the owner's reasoned switch-local ignore form for P.
- `internal/component/bgp/cli/encode.go`, `cmdEncode`, deliberately uses an untagged switch to avoid exhaustive checking. Replace this bypass with a typed SAFI switch and a reasoned P annotation; preserve the registry fallback.
- Type switches, untyped constant families and arbitrary boolean switches are outside this analyzer. Do not claim that this setting enforces sealed-interface completeness. Do not introduce casts or untagged switches to evade enum checks.
- Missing-case diagnostics cannot find a bad default on an already-exhaustive switch. `internal/component/command/argvalidate.go`, `ValidateArgString`, names every ArgKind but returns nil for an unknown kind. G001 records this verified policy gap. Before editing batches, perform a type-aware census of all first-party enum switches across the same build populations, including complete switches. Assign each a class and inspect its default and post-switch behavior. Use scratch analysis, not a new permanent checker. The measured inventory and commit sizes below are lower bounds until that census is reconciled.

### Implementation checkpoint, 2026-10-05

The native type-aware census found **822 enum identities in 514 files and 204
packages**, including 474 already-complete representative observations. It
analyzed 19 populations from the real lint plan; the plan skipped linux-arm64
because it was already covered. The collector's raw `incomplete-see-errors`
status is retained: two excluded-source lexical candidates needed supplemental
typing. Source and LSP establish both as built-in string switches. Generated
sources likewise contain two built-in string switches, not enum switches.

All 356 seed rows and all 117 actual `nolint:exhaustive` directives were
reconciled. The implementation packets contain 825 identities: the 822 typed
switches, the SAFI avoidance and two marker false positives. Their returned
classifications, after the independent IS-IS correction, are **458 C, 202 O,
163 P and 2 outside-enum**. BGP also converted the existing `encoderSlot`
conditional to a typed C switch and removed two BMP non-enum suppressions.
The durable [per-switch inventory](enum-switch-exhaustiveness-inventory.json)
now reconciles **825 typed identities: 459 C, 202 O and 164 P**, including
the concurrent OSPF interop subset. Its two original marker false positives
and two BMP non-enum cleanups are separate. JSON parsing, unique identities
and class totals pass; a stable final census remains owed before AC-1 closes.

| Batch | Assigned identities | Independent static review | Runtime verification |
|-------|---------------------|---------------------------|----------------------|
| Commands | 81 | No actionable finding | Landed in `93bb60878ad8`; 7 owning packages passed race; boundary probes and all 5 committed flavors passed |
| ConfigPlatform | 160 | No actionable finding; parent traced both production kernelcap enrollments and all probe results | 47 owning packages passed race; 1 has no tests |
| Tools | 143 | No actionable finding | 38 owning packages passed race; 1 has no tests; 5 red packages, qualified below |
| SecuritySubscriber | 109 | No actionable finding | 16 owning packages passed race; 1 has no tests; DH boundary probes passed; RFC proof renewal open |
| BGP | 191 | No actionable finding | 43 owning packages passed race; RR/RS VPN inventory tests red; capability and ADD-PATH probes passed |
| RoutingPlugins | 77 | Complete `configFormsLevel` dispatch reclassified P→C; independent correction review cleared | 23 owning packages passed race; separately scoped VPP SRv6 probe remains red |
| OSPF | 62 | No actionable finding | All 12 owning packages passed race |
| Parent setup sites | 2 | Reviewed with Tools; no actionable finding | Setup package passed race |

The original installed linter detected a direct omission but missed its alias
through the normal native action. Thomas chose the scoped supported dependency
upgrade. **`d0bf51e9af43`** lands golangci-lint v2.14.0, exhaustive v0.13.0,
their required module/vendor graph and the aligned setup pin. Existing direct
dependency annotations were preserved and both canonical vendor patches
reapplied. Setup/vendor-patch race packages and all five committed build flavors
pass. No custom analyzer or launcher adapter was introduced.

The working-tree root setting is now false and the guide states global
enforcement. The **normal native scoped action** passed all five strict probe
phases for direct and aliased inputs: omission despite `default` fails with the
exact missing-member diagnostic; adding the case passes; adding a member fails;
justified subsets pass; adjacent unmarked omissions still fail. Both actual
scoped passes reported each expected omission. The disposable package was
removed, and watched source/config/tool hashes did not drift. This is not a
whole-tree lint certificate.

Evidence is under
`tmp/session/2026-10-04-01a10694-3795-71f2-8250-25e3a877f4cf/scratch/`:
`enum-census/{census-reconciliation,seed-reconciliation,marker-reconciliation}.json`,
`enum-implementation/{results,review}/`,
`enum-implementation/integration-corrections.json`, and
`enum-census/alias-probe/strict-e00c79db7a884f1a84a5fcb41dbaed82/result.json`.
All 825 assigned identities reconcile against their returned reports; 601
runtime/test/doc paths are in the scoped review manifest. Parent formatted the
579 Go paths. The full native race command reported 637 passing packages,
218 packages with no tests and 11 failing packages. Independent source triage
found no enum-caused failure: the reds concern live-tree parity, two existing
synchronization gaps, concurrent VPN inventory work, hook fixture/HEAD state,
selector/build deadlines, cache-fixture accounting and already-recorded
publication, historical-script and VPP gaps. This is not a green full-suite result.

All seven runtime boundary probes passed: unknown capability bytes and named
unsupported capability bytes round-trip unchanged; unsupported DH and encoder
inputs retain their errors; CLI enum/string acceptance, ADD-PATH labels and
cleared DH state retain their contracts. The disposable fixture was removed
and its watched inputs did not drift.

The first post-edit census observed all 822 original typed identities, the
typed SAFI replacement, `encoderSlot`, and one concurrent OSPF interop switch.
It refused completion with 712 recorded errors, including source drift and
ill-typed platform populations. The native plan had selected additional BGP
packages for FreeBSD, DragonFly and WASI; those roots expose existing platform
gaps, not an exhaustive-version regression. Concurrent VPN test references
also preceded their producer declaration. The subsequent native collection
retained 825 typed identities but reported 70 errors: the two known excluded
string candidates, concurrent VPN test type errors and source drift. Neither
run is a stable final census. The immutable-export collection then completed
every native population without those type/drift failures and retained only
the two known excluded lexical string candidates. It preserves the native
plan and all errors; it is not a lint certificate and predates the repair pass.

The first full native strict lint completed and was red. Its only exhaustive
finding was the new OSPF interop subset, now explicitly justified. Parent
corrected the owned new-tool diagnostics, including discarded panic-test
results, the typed MCP range and the private IS-IS `classOf` return contract.
Focused race runs passed; independent review cleared the integration delta.
The next complete native lint was also red: 136 distinct diagnostics remained,
but none was exhaustive. The owner's expanded requirement makes those lint
findings and the identified runtime and proof defects repair scope.

Canonical RFC collection identified 284 existing-record candidates and three
changed claims without records. The primary native recorder completed all
272 approved entries. Seven additional native records and two actual QEMU
guest records completed, for 281 records. Twelve original owned audit rows were
independently rejudged and stamped; the canonical resealer refreshed 168
mechanically shifted rows and refused changed units. Foreign audit scope,
including whole-set MED, remains untouched.

Correcting false RFC8654 minimum-Length prose and EAP wrong-role assertions
required three further independent rejudgments. They are now `weak`, with
the actual evidence limits recorded; the RFC8654 and RFC3748 public support
claims were corrected to partial. The corrected EAP unit passes under the
native race detector. The owner then authorized repair of the identified
bugs. Native probes reproduced wrong-role EAP Failure output, termination
after 21 undefined EAP Codes, and rejection of a locally advertised extended
UPDATE when the peer did not advertise the capability. Behavioral regressions
also reproduced the AS_PATH section overrun, BGP start-event teardown, PPP
transition defects, invalid IS-IS neighbor creation, and blocked callback
waiters during shutdown. JSON engine-step probes reached TLS setup for
unknown kinds instead of refusing them at the file/stdin boundary.

Ten independent repair owners cover EAP/crypto, reactor, RIB/wire consumers,
build tooling, harness fixtures, interop lint, remaining lint, protocol
transitions, platform contracts, and lint-plan coherence. They do not run
competing build or lint jobs; the parent owns integration and verification.
Their implementation reports are not verification results. Final census,
complete lint, affected runtime/interop proof, renewed RFC evidence,
independent review and six enum package commits remain open.

### Linux amd64 continuation, 2026-10-06

The destination checkout contains both checkpoint commits. All 448 portable
evidence files were restored and match the archived SHA-256 manifest. Native
execution uses Linux amd64, not the original host's ARM64 settings. The earlier
package changes are already checkpointed; do not repeat their commits.

Continuation logs and reports below are under
`tmp/session/2026-10-06-7487271c-d4f1-4454-972e-5298f85f9b43/scratch/`.

| Obligation | Current observation | Limit |
|------------|---------------------|-------|
| Input fingerprint admission | The exact tracked-name and full-diff Git queries succeed; named native admission also completed the scale and platform runs below | The historical killed scan's cause remains unproven. No fingerprint error, input, timeout or coherence check was suppressed |
| Scale lifecycle | Full-feature `go test -race -count=1` through `le job run` passed `TestInProcessScale20` in `internal/chaos/inprocess` | This is the named scale regression, not a full-suite result |
| Platform contracts | DragonFly amd64 builds passed for diskspace, doctor, support, config/storage and zefs; js/wasm under Node passed all three `TestUnsupported` refusal tests | DragonFly execution was not claimed |
| Native tool prerequisites | Installed golangci-lint 2.14.0; pinned native amd64 VPP image reports v26.06-release | Both native forwarding rails pass. No manual hugepage setting was changed; the post-run host reports 19 free 2MiB hugepages, so this is not evidence of running without hugepages |
| VPP external forwarding | The original native probe failed its first withdrawal after Ze restart. Moving subscription ahead of ownership restoration restored the external packet/state rail, including restart identity and final withdrawal | Missing-policy discrimination now has the concrete red and restored green recorded below; managed forwarding is a separate rail |
| VPP subscription regression | `job-enum-vpp-publication-race-aaa5a355.log` passed both current-writer/publication tests under race detection. The no-lock overlay failed specifically with `withdrawal bypassed the lifecycle lock before publication` in `job-enum-vpp-publication-no-lock-ff8e8c99.log` | Committed as `ad3fd9de09` after independent production and follow-up review. Native commit recorded full-verification debt; this is not final spec review |
| Managed VPP startup and forwarding | `job-enum-vpp-policy-restored-native-1fd7f254.log` passed the freshly built native fixture, external probe and managed probe. Managed VPP restarted PID 255 → 318; production replay restored both tenant tables. Captures prove distinct remote SIDs, replacement, and withdrawal absence | Evidence: `tmp/evidence/vpp-srv6-876310729`. The separate legacy deployment action also passed, as recorded below |
| VPP component and lab regression | Complete VPP component race passed (`job-enum-vpp-complete-grammar-race-6f679828.log`); complete interoplab race also passed. Grammar repair committed as `78c88604fc` after `VPPGrammarReview`; the subsequent committed-tree compile gate passed all five flavors | The earlier lifecycle commit is `ad3fd9de09`. Full native verification debt remains open |
| Extended Message peer fixtures | `job-enum-native-peer-matrix-ba5e23c8.log` passed all five directional Extended Message scenarios, `bgp-ebgp-ipv4-frr` and `isis-p2p-frr`. Native reports in `native-peer-matrix-sykhihkh/` identify each fresh scenario and successful result | All five Extended Message runs exercised the stronger two-sided FRR/Ze Established barrier. Canonical interop discrimination remains separate |
| RFC recordings | The first 32 commands in `rfc-native-unit-run-xc_3kt6e/results.jsonl` completed successfully. The serial wave was stopped before further fixture corrections | These are observed native recordings, not a claim that every record is current after later edits. Recollect and renew invalidated or unfinished covers |
| Independent RFC judgments | Independent judgments cover 76 requirements: 68 enforced, seven weak and one unimplemented. All 29 native stamp commands passed; per-command results are retained in `audit-native-stamps-o21agd5o/results.json` | RFC2545's earlier Supported claim is corrected to Partial; invalid-global and shared-subnet findings still need wire reproduction. The zero-length AS_PATH clause lacks a discriminating case; type-13 SRv6 BSID remains unimplemented. Later source edits still owe freshness collection |
| RFC proof repairs | Corrected tunnel codec/receive/export, encrypted EAP carrier, extended accumulation/discard and actual-RIB treat-as-withdraw cases passed under race detection. The initially red alternate-best case now passes on actual TCP in `job-enum-recovery-causal-race-941bd345.log` | `RecoveryCausalProofReview` is clear and complete reactor race20 passed. Canonical RFC renewal remains a separate obligation |
| Recovery integration and review | The original complete reactor race20 failed three negative send-hold repetitions because established fixtures left timers armed. Both timer owners and real session runners now have joined cleanup; `SendHoldOwnershipReview` is clear. The complete repaired reactor race20 passed in `job-enum-reactor-owned-lifecycle-race20-b90b8167.log` (4303.147 seconds) | The cleanup-omission fault fails all four retained-owner assertions. Focused race20 also passed in `job-enum-sendhold-lifecycle-race20-5198c479.log`. Canonical RFC renewal and final acceptance remain separate |
| Recovery failure policy | Remove the arbitrary source-DOWN command expiry; bind pending work to lifecycle cancellation. An unusable sent-ownership projection cannot authorize a stale repair | Use the existing destination-session close path for hard failure rather than retaining known-invalid advertisements or parking an inert obligation; no timer retry or second route inventory |
| Causal proof execution | `job-enum-recovery-causal-race-941bd345.log` passed all eight real-TCP queued-delivery scenarios, the framed IPC observer controls, strict delivery receipts, atomic sections, source-generation and reconnect cases, hard writer failures, and `TestRFC8654FatalLengthReelectsAlternateBest` | JSON IPC uses the real transport in-process, not a forked executable. The subsequent complete reactor race20 passed; this is not whole-tree acceptance |
| Native test-change audit | Independent protocol and tooling reconciliation found no lost obligation in seven removal, rename or assertion-extraction findings; explicit explanations are in `test/weakened/56e1ee9b.md`. The subsequent native audit still reports 25 unexplained findings in `job-enum-proof-structure-ledger-reconciliation-9b157efc.log` | Pending explanations are not a cleared audit. They do not supply owner RFC approval. The native `rfc-change-ok` route can carry that approval as verification debt for a local commit; acceptance clearance remains owed |
| Proof maintenance | The RIB shutdown diagnostic proved a competing AIGP election could remove the route before the DOWN purge. The repaired fixture orders that election before DOWN without changing the SDK drain fence; `ShutdownScheduleReview` is clear | Final race200 passed in `job-enum-rib-final-schedule-race200-cdab96c1.log`. Omitting `WaitDelivery` still fails with early plugin exit and mirror detachment in `job-enum-rib-final-schedule-no-drain-6d68daf1.log`; test-owned delivery cleanup now joins even that faulty path |
| Carrier, tunnel and FlowSpec discrimination | All three FlowSpec faults and all seven carrier faults discriminated. The tunnel wave initially credited 29 cases; renewal credited 11 more. Independent source mapping justified the remaining 14 observed semantic reds: expected leaf offsets were one byte too high, while both AFIs and all 56 required controls ran correctly | Immutable raw results remain unchanged. `tunnel-semantic/receive-source-mapped-assessment.json` records the correction, hashes and provenance limit; this is not a native canonical record. All six caller faults also discriminated in `semantic-observations-toqp67n1/result.json`. Canonical records and stamps remain open |
| VPP missing-policy discrimination | Fresh daemon/probe builds with the policy-install omission failed with `policies = 0, want 1 shared-SID policies` in `job-enum-vpp-policy-omission-diagnostic-1fd7f254.log`; the restored build passed both forwarding rails in `job-enum-vpp-policy-restored-native-1fd7f254.log` | Red evidence: `tmp/evidence/vpp-srv6-2329346853`; green: `tmp/evidence/vpp-srv6-876310729`. The probe now preserves its last installed-state mismatch when the final API read times out |
| Recovery follow-up findings | The repaired causal race wave passed deadline/write/flush faults and both replacement-session ownership cases without a race report. Complete RPC race and real SDK final-OK success/failure passed. The IPC close-observation fixture passed 20 race iterations after waiting for `mux.Done` | The unrelated state-RPC deadline recurrence is recorded in its existing journal row. The removal-retry fixture passed 20 race iterations in `job-enum-ppp-removal-proof-race-acafa7fb.log` after waiting for transaction release as well as retained failure metadata; production unchanged |
| Legacy VPP deployment | `job-enum-vpp-acl-reconciled-native-e9966882.log` passed the complete native action: IPsec, FIB/MPLS, traffic restart/orphan/classify lifecycle, and firewall install/singleton restart/orphan removal. Full firewall and deployment race passed; independent ACL review is clear | Traffic repair is `c9149d0768`; firewall repair is `866f893845`. Both committed-tree compile gates passed. The pre-existing ACL-vector width defect is recorded in `bound-wraps-before-it-refuses.md`, not claimed fixed |
| Recovery semantic discrimination | `job-enum-recovery-semantic-b3e7da3d.log` credited both faults. Omitting the applied barrier failed the actual newer-owner TCP assertion on DirectBridge and JSON IPC, while all six controls passed. Forgetting earlier delivery failure failed both transports' retained-error assertions while success controls passed | Results: `semantic-observations-df5wkvnr/result.json`. `RecoveryCausalProofReview` found no actionable defect; this is scoped recovery evidence, not final spec review |
| L2TP and PPPoE peer acceptance | Simple L2TP passed against xl2tpd 1.3.18; identity regressions passed 20 race iterations. Native no-auth and CHAP-MD5 same-transport restart passed in `job-enum-l2tp-ppp-management-native-3e6d5363.log` and `job-enum-l2tp-ppp-chap-management-native-4f052b53.log`. Native PPPoE CHAP/IPv4 client also passed | Management/data-session distinction is committed as `64ecb0d2b7`; its committed-tree compile gate passed. Fresh IPCP, restored traffic, teardown and wrong-secret refusal passed. Absent IPv6 subscriber scenarios are not claimed |
| Transmit and directional semantic proof | All 25 added faults discriminated in `semantic-observations-9r9vkava/result.json`: 22 RFC9830 transmit fields, valid-update recovery rejection, bilateral accumulation gating and bilateral header gating. Required controls passed; independent rejudgment supports repaired claims | Binding SID Flags `0x10` was reproduced and corrected. The original retained run passed 70/80; orphan-contaminated 23/24 and 32/32 counts remain invalid. Corrected vectors are promoted and pass the live suites below; canonical records remain separate |
| Current native gates | Final native lint after the framing repair passed all 19 enabled populations with zero findings in `job-enum-post-framing-final-native-lint-2c7ef39f.log`; its three declared exceptions remain explicit. The complete RIB race and final post-publication-lock reactor race20 passed; the latter reports `ok .../reactor 4311.804s` in `job-enum-reactor-publication-final-race20-89d29726.log` | Complete native unit execution remains red in `native-unit-all-complete-transcript.log`. Scoped repairs and post-commit compilation do not upgrade that observation |
| Canonical RFC continuation | After the framing repair, native renewal and resealing reported 4862 verified records, four retained historical tag-gone records, 2295 fresh judgments and 8252 covers in `job-enum-post-framing-proof-freshness-bd3b0aca.log` | `job-enum-post-framing-canonical-rfc-check-f8aecd88.log` retains nine filename/tag debts, recorded in `plan/journal/test-type-not-backfilled.md`. Historical tag-gone records remain intact. RFC9012-13-1 is now enforced; -13-2 remains weak because complete downstream withdrawal is not asserted |
| Gap-disclosure proof boundary | The existing valid-state IS-IS control gained a positive tag without changing its assertions; `job-enum-isis-valid-control-record-2b8ca20f.log` records its native green/red observation. Independent first judgments were stamped for RFC5303-3.2-7 and RFC9012-13-1, -2, -14 and -15: one enforced and four weak | The tunnel judgments retain the framing-fixture isolation limit, missing complete downstream/carrier proof and endpoint address-semantic gap. Structural endpoint removal is not full Section 3.1 enforcement. RFC5303 remains Experimental and RFC9012 Partial; no absent feature or stronger conformance credit was added |
| Framing-fixture isolation repair | All eight original framing cases passed with `end > size` omitted: malformed sub-TLV fixtures also lacked an endpoint, hiding the missing boundary check. The owning test now supplies a valid endpoint before each malformed sub-TLV, preserving every case and exact outcome | The same selective fault now fails both short/long value cases in `job-enum-framing-boundary-after-red-da6b72ea.log`; real-producer framing and sibling race smoke passes in `job-enum-framing-repaired-race-smoke-5388cca1.log`. Four canonical covers were renewed; independent rejudgment was stamped in `job-enum-framing-independent-rejudgment-6b42ee1e.log`. Producer-panic records do not replace this selective semantic proof |
| Negotiation fault cleanup | The retained producer fault timed out at 30 seconds with `closeConn` waiting on `writeMu`. `negotiateWith` now computes under the session lock before acquiring the writer lock; publication and resize remain protected. The same fault now exits with its intended panic in 0.082 seconds | Before/after: `job-enum-negotiation-fault-timeout-stack-ec108d2d.log` and `job-enum-negotiation-fault-after-publication-fix-ec108d2d.log`. Independent concurrency and proof-mapping reviews are clear. The neither-advertised FRR recording and completed post-change reactor race20 now supply both pending runtime proofs |
| Frozen enum census | Capture `run-20261006T091123Z-q7oqzwbw` passed capture, protected-index and export guards. Raw exit 2 and both builtin-string-qualified errors remain intact. Independent and parent reconciliation preserve all 834 identities and 13461 observations: 462 C, 207 O and 165 P. Parent checked every typed-source hash and input, plus the complete Go path population | Complete capture readback verified 114068 entries. `plan/handover/enum-switch-exhaustiveness-20261006-census.json` indexes three lossless archive parts; concatenation SHA-256 is `2481e95d27dd8aa16a97cf0627804b85df84aa5d75ef2edded66db73239e5b7b`. Supplemental source comparison identifies only the IS-IS comment tag and framing-fixture deltas, neither changing enum classification. Nineteen historical artifacts remain unavailable; historical replay is limited, not the fresh classification |
| Startup diagnosis and live vectors | Owned evidence locates nine failures before TCP; one records 18 completed fsyncs totaling 8.985411 seconds. The unassisted run still passed only 79/80. After explicit build-filesystem writeback drain, compatibility and wire80 passed; subsequent live BGP encode passed 63/63 and ExaBGP encoding passed 42/42 in `job-enum-live-encode-compat-acceptance-ec925470.log` | Both live suites include the corrected SR-policy fixtures. Fresh binaries, original deadlines and durability barriers were retained. This is successful evidence under the recorded drain precondition, not a production startup repair or kernel/device diagnosis. Former drafts remain preserved as `promoted-*.ci` under scratch |
| Admission fixture follow-up | Five verifier tests now use the existing committed repository fixture without dropping branch, timeout, cleanup or diagnostic assertions. Complete verifier race passed; independent logic, lifetime and proof reviews are clear | Committed as `a97ec9bbb6`; all five post-commit compilation flavors passed. Production admission is unchanged |
| Tooling and proof follow-up | Complete RFC, inventory, interoplab, verifier, RIB and VPP race suites passed. Corrected BGP fixtures now cancel after the initial observation; their complete owning package passed in `job-enum-bgp-readiness-owning-race-5d09e3b7.log`. Narrow follow-up review is clear | The reduced 18-case RFC credit matrix passed. Inventory source-population repair is committed as `d301378b52`, with native entry-point proof and all five post-commit compilation flavors passing. These are scoped results, not whole-tree acceptance |
| Cancellation admission proof | The real Wait API admitted zero probes when pre-canceled, retained a real subprocess failure after one probe, and retained measured value 37 after a later failure. `job-enum-native-wait-cancellation-observed-error-be85aac8.log` records all three outcomes | Omitting only the admission guard failed with `BUG: pre-cancelled wait admitted a probe` in `job-enum-wait-cancellation-omission-25883eee.log`. The earlier smoke assumed exit status 1 and instead observed a deadline-killed subprocess; the corrected oracle compares the actual execution error, without extending the deadline |
| Documentation reconciliation | Native reverse-index and RFC-index generation passed. Final private build/check produced all 995 registered artifacts from 22 producers under `enum-final-disclosure-site-20261006`. Chromium shows RFC7311 Partial with the failed native source-cost proof explicitly disclosed. Prior final framing screenshots show RFC9012-13-1 enforced/fresh and -13-2 weak/fresh | The default documentation gate remains red on sibling catalogs and runtime-evidence traversal. After fetch, `../gh-pages` is ahead 1/behind 10 with modified pages; `../wiki` is ahead 3/behind 4 with untracked commit artifacts. Neither sibling was edited or rebased. Private generation is not public publication |
| Shared AIGP ownership and runtime limit | The interrupted source-cost session's dependency-complete checker cohort is preserved in `7f21ff109e`, after independent source review. Its native FRR scenario failed at the first accumulated-metric assertion: the decoded route had no AIGP field. The unchanged failure is recorded in `job-enum-shared-aigp-native-acceptance-730cf9cd.log` and the existing broken-path journal | Later transitions and exact-wire assertions were not reached. Assertions and deadlines remain intact; no producer cause or RFC violation is inferred. RFC7311 stays Partial with explicit end-to-end uncertainty. This red is not acceptance evidence |
| Completed local landing | IS-IS disclosure/proof correction: `8efffa2104`; causal recovery: `22bba5fded`; preserved AIGP cohort: `7f21ff109e`; integrated runtime proofs and canonical judgments: `bd02b3b5dd`; remaining source-linked pages and quote punctuation: `36e00133cf`. All five committed-tree compilation flavors passed after each Go-carrying commit | The shared AIGP red is disclosed, not approved as correct. Structural reconciliation supplies no owner RFC-test approval. Native preparation recognized the EAP carrier move as non-weakening and refused its two stale waiver rows; they were retained as historical evidence and removed from the active ledger, without changing tests |
| Final evidence integration | `plan/handover/enum-switch-exhaustiveness-20261006-final-proof.json` indexes two lossless parts containing 4063 entries, including native logs, complete review reports/history, final private publication, browser screenshots and VPP packet/state/binary evidence. Every retained byte hash, mode and link target was read back; 201 scratch recipes were decoded and checked as inert data | Archive SHA-256: `3926ec7a48587fa6f29d5eb461fe356f83e2f64f35657d7834d1e3761e884207`. Canonical inventory links the durable final source relation at `36e00133cf`: 17193 live Go files, two already-absent carrier retirements and only four preserved foreign red probes outside HEAD. No source/commit integration obligation remains |

`plan/handover/enum-switch-exhaustiveness-20261006-acceptance.tar.gz`
preserves nine completed observations from this continuation, with source paths,
lengths and SHA-256 values in its manifest. Archive readback verified all nine.
This checkpoint supplements, and does not replace, the earlier handover archive.
It is committed with the progress record as `b7a247d066`; the spec stays open.

Final lint, post-lock race proof, canonical renewal, source landing, private
disclosure derivation and durable evidence/inventory integration are complete.
The bounded final acceptance review found no new substantive mismatch.
Thomas approved the five named commits' RFC-tagged test changes; the native
owner-attestation route discharged their five approval rows. He reconciled both
public sibling checkouts, then authorized the remaining documentation repairs.
Those repairs are now exercised and locally committed. The final native lint
matrix and documentation gate pass. Public artifact derivation and painted
browser proof pass; no push or remote deployment is claimed.
Historical whole-unit and startup reds, the shared AIGP red and unavailable
author artifacts remain qualified. Formal spec closure is not claimed.

### Publication clearance, 2026-10-06

`plan/handover/enum-switch-exhaustiveness-20261006-publication.json` indexes the
supplemental archive. It retains the new logs, full review reports, exact test
partition, source relation, inert overlay inputs and before/after screenshots.
The earlier census and final-proof archives remain unchanged.

| Obligation | Observed evidence | Limit |
|------------|-------------------|-------|
| First-party source counts | `87e967d79a`; old-producer overlay fails, all new counter regressions and the complete `yangcontract` race package pass | Excluded runtime artifacts are pruned before descent; genuine first-party read failures remain findings |
| Literal command prose | `939786ce71`; renderer regressions preserve placeholders, literal markup and block-start punctuation on reference, LLMS and detail surfaces | Code-span usage remains a separate path; contract readers were not weakened |
| Valid isolated build inputs | `246980057e`; seven previously failing build cases pass after supplying their required `ai/` directory | Assertions and deadlines unchanged |
| Complete owning site test population | `enum-publication-site-race-coverage.json`: actual listed population of 358 tests equals 233 + 124 + 1 observed race passes | Original aggregate reds remain recorded; this is partitioned coverage, not one green whole-package invocation |
| Actual painted reference | `c4fea842f5`; threshold 0 replaces the impossible 1% intersection requirement; `enum-publication-reveal-after.json` and `enum-publication-cli-painted-final.png` show the literal selector at opacity one | Earlier DOM text and blank screenshots did not prove visibility; they remain as failed evidence |
| Source-aware documentation | `b783523a4c`; four retired anchors repaired, path-weight and retained-handle claims corrected, both indexes regenerated | Final `job-enum-publication-painted-doc-check-29876447.log` ends `Documentation tests PASSED` |
| Local sibling publication | Wiki `d004de6`: regenerated 473-command catalog. Pages `49988df280`: native build covers all 995 artifacts from 22 producers | Original 22 untracked sibling artifacts retain their bytes and modes; no push authorized |
| Final lint | `job-enum-publication-landed-native-lint-2c7ef39f.log`: all 19 populations pass on settled source commit `c4fea842f5` | The preceding attempt refused before producing a plan because HEAD changed during planning; it is not a finding-producing lint run |
| Source and review continuity | `enum-publication-source-relation.json` accounts for 14 later Go files; independent publication, fixture and final acceptance reviews find no substantive AC-1–12 mismatch | Extends, rather than replaces, the frozen census; formal closure and historical qualifications remain distinct |

## Goal Validation

The continuation logs below are under
`tmp/session/2026-10-06-7487271c-d4f1-4454-972e-5298f85f9b43/scratch/`.
The census archive index retains the exact frozen inputs; later source relations
describe deltas rather than replacing captured bytes. A qualified result is not
whole-tree acceptance.
The final-proof archive index above supplies durable member mappings for these
continuation paths, while the original handover archive retains the historical
alias and VRRP observations.

| Task goal | Concrete evidence | Result / limit |
|-----------|-------------------|----------------|
| Explicit closed, open and partial switch policy | Canonical inventory: 834 identities, 462 C / 207 O / 165 P; `enum-q7-parent-final-recount-and-source-relation.json`; lossless census archive index | Fresh population reconciled; 19 unavailable historical author artifacts remain a provenance limit. Final source relation records non-enum deltas and already-absent carrier retirements |
| Enforce future omissions globally despite defaults | Original handover archive, `enum-census/alias-probe/strict-e00c79db7a884f1a84a5fcb41dbaed82/result.json` and raw logs: direct and aliased omissions fail both native passes; explicit cases pass; a future member fails again; justified subsets pass while adjacent unmarked controls fail | Observed real `le` entry-point red/green, not a configuration-text test |
| Preserve safe external-input and internal-state outcomes | Named boundary observations at lines 176–180 are historical; current causal TCP/IPC proof is in `job-enum-recovery-causal-race-941bd345.log`, with selective barrier faults in `semantic-observations-df5wkvnr/result.json`; framing rejection/control proof is in the three `job-enum-framing-*` logs named above | Current scoped runtime evidence and source classification complement, rather than reconstruct, unavailable historical artifacts. No blanket peer-safety claim follows from lint alone |
| Complete native lint and repair identified runtime defects | `job-enum-post-framing-final-native-lint-2c7ef39f.log`; final reactor race20 and RIB race200; live FRR, xl2tpd and PPPoE results in the continuation table | All 19 lint populations pass. Whole-unit red, conditional startup success and the newly observed shared AIGP red remain explicit; they are not converted into successful checks |
| Repair invalid proof and renew affected RFC evidence | Selective framing omission changes from eight false-green cases to the intended value-boundary reds; real producer race smoke passes. `job-enum-post-framing-proof-freshness-bd3b0aca.log` records current canonical proof; Chromium observes revised RFC9012 judgments | 4862 verified records and 2295 fresh judgments; four historical tag-gone records and nine naming debts remain. Owner approval and public sibling publication are not supplied by these checks |
| Prove VPP SR-policy and local binding-SID lifecycle through forwarding | `job-enum-vpp-policy-omission-diagnostic-1fd7f254.log` rejects missing installed policy; rebuilt restored producer passes both forwarding rails in `job-enum-vpp-policy-restored-native-1fd7f254.log` | Actual VPP packet/state proof; not SRPM, remote decapsulation, underlay provisioning or a new tenant-table producer |
| Preserve historical VRRP recorder without weakening the guard | Original handover archive, `enum-followthrough/historical-archive-proof.json`: all 2602 bytes match SHA-256 `f569172e8ceca4167c3711d4fb91e8e491cb0fbd34de3f50d549d6c2915f26d6`, Markdown is non-executable and old shell path is absent; `job-enum-archive-wiki-contract-ed24ac13.log` | Recorder preserved as authorized; no executable historical shell or guard exception added |

## Remaining Clearance

| Blocker | Why it remains open | Required action |
|---------|---------------------|-----------------|
| Formal spec closure | The qualified implementation/publication baseline is distinct from whole-tree green. The owner closure ruling below explicitly skips security and whole-tree verification while another agent codes | Complete the nonwaived closure records, citation cleanup and hash-pinned non-security review; preserve every historical qualification |

Thomas selected “Approve reviewed changes” for the RFC-tagged test changes in
`d769ff084b`, `8efffa2104`, `22bba5fded`, `bd02b3b5dd` and `36e00133cf`.
The native `owner` discharge records that attestation in
`plan/verification-debt/discharged/56e1ee9b.md`, covering rows 12–16 of the
session's debt shard. It does not clear other verification or publication debt.

The nine filename/tag debts are retained convention-gate observations, not an
additional AC or permission to rewrite foreign tests. The documentation gate
and local public-artifact derivation now pass; neither is a push or remote
deployment, nor does either clear unrelated whole-tree verification debt.

## Data Flow

| Path | Transformation | Boundary / preservation |
|------|----------------|-------------------------|
| Developer invokes le lint | le admission → build-flavor package selection → golangci → exhaustive facts and switch checking → diagnostics | Run through the real action; no permanent launcher adapter |
| Peer, file, kernel or plugin supplies a code | Decode or parse → typed value → switch | O until an inspected producer/validator proves a finite internal set; retain unknown handler |
| Ze constructs a state or category | Constructor/parser/classifier → stored enum → switch | C only after tracing constructors, mutations and callers; valid zeros remain explicit |
| Consumer selects an independent subset | Shared enum → predicate, projection or state-specific handler | P preserves nonmatching behavior and names the subset reason |

| Architectural check | Holds in this design? | Evidence / constraint |
|---------------------|-----------------------|-----------------------|
| No bypassed layers | Yes | Existing le action and runtime producers stay in place |
| No unintended coupling | Yes | Package-owned cases and comments; no central enum registry added |
| No duplicated functionality | Yes | Use upstream exhaustive, not a Ze checker |
| Zero-copy preserved | Required | No new per-packet allocation, conversion, lookup table or helper layer for this sweep |
| Registration, outbound | N-A | No new feature, command, family or handler |
| Registration, inbound | Required | Preserve plugin registry fallbacks, including `cmdEncode`; enum completeness must not replace discovery with a feature list |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | The baseline switch inventory still names the implementation tree | Shared-tree measurement and source reads | New or moved switches escape the batch list | Frozen census plus later source relations supersede the initial list; see closure Mistake Log | broken |
| A-2 | Each C switch receives only locally produced values | Per-switch producer evidence in research | A peer or plugin can trigger a new panic | Recorded per-switch domain review and independent package reviews; closure security explicitly unverified by owner | confirmed by recorded implementation review |
| A-3 | Generated/coverage-exempt sources add no missed enum switch | Bounded generated-source research, not a whole-AST proof | “Every switch” overstates coverage | Captured supplementary typing identifies excluded/generated candidates as built-in strings; raw collector errors preserved | confirmed for captured population |
| A-4 | Moving a test-only enum declaration does not add protocol support | Unknown/rejection tests and current decoder defaults | An unsupported algorithm or capability becomes accepted | Recorded boundary probes preserve unknown capability round-trip and DH/negotiation refusals | confirmed by recorded boundary proof |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A useful default is deleted, producing silent no-op or fallthrough | Case-only mechanical patch | Move known behavior into explicit cases; retain an appropriate unknown handler; inspect code after the switch |
| R-2 | Known sentinel behavior changes | Named zero newly reaches a BUG default | Enumerate sentinel outcomes explicitly, including existing errors, display strings and valid zero values |
| R-3 | A broad ignore hides incomplete complete dispatch | Reason says only “unknown handled in default” | C/O stays checked; P reason names an actual predicate, projection or state-specific subset |
| R-4 | Same-valued aliases or masks cause duplicate cases | Bit-field and imported registry enums | Enumerate distinct values once; keep mask semantics and named aliases documented at their declaration |
| R-5 | A default has masked-code behavior, not just fallback | OSPF Router Information function-code recognition | Preserve recognition for unnamed values with the same masked code; do not limit it to newly explicit constants |
| R-6 | Test-only typed members cannot be named in production | `codeSoftwareVersion` and `encrNull` | Move each sole named declaration to its owning production enum with an unsupported-value comment; retain unknown formatting and refusal, not new support |
| R-7 | Scope drifts into known-value semantic repair | PendingChange.Summary renders activate/deactivate as set | This sweep does not authorize new output policy. Keep current named behavior explicit; record any proposed semantic correction separately rather than disguise it as lint cleanup |
| R-8 | Internal misuse already has a documented/tested fallback | AuthMode.String, SetupOutcome.String, MulticastMACForLevel | Apply the owner's closed-set policy deliberately; update the affected misuse contract/tests, not external-input rejection |
| R-9 | Enum closure is lost through mutation or exported fields | DHExchange.GroupID and PortForm consumers | Preserve validating constructors and peer guards; repeat source-reference inspection against the tree being edited |
| R-10 | Measurement undercounts | Issue caps, nolint filters, generated filter, build tags | Uncapped per-pass reports, separate suppression inventory, declared coverage limits and final global run |
| R-11 | Unnamed zero is a valid existing construction | Zero Selector, ScopeReport.Print empty, private hook sentinel, shared IPsec direction | Preserve the zero path explicitly; do not infer zero-invalid from newer style guidance |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A daemon panic, changed routing/authentication decision, altered CLI output, or silently lost handling of a new enum member |
| How is it undone? | No schema/data migration. If needed, prepare an owner-approved corrective patch; do not run destructive git commands |
| Who else touches these paths? | Active protocol/RFC proof work, including `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`; reconcile concurrent edits without overwriting them |

## Wiring Test

| Entry point | Feature code | Test / observable result |
|-------------|--------------|--------------------------|
| `./le go lint run scope <scratch-fixture-package>` | Final root config → upstream `switchChecker` | Throwaway `EnumSwitchLintContract`: an unmarked switch missing a member fails even with a default; adding the case passes; adding a new member fails again |
| Same real lint action | Switch-local ignore semantics | Throwaway `EnumSwitchSubsetContract`: justified ignored subset passes; an adjacent unmarked incomplete switch still fails |
| Normal full-tree lint action | All registered build flavors | No exhaustive finding; preserve declared coverage exceptions and report any unrelated lint failure separately |

## Acceptance Criteria

| ID | Input / condition | Expected behavior |
|----|-------------------|-------------------|
| AC-1 | Full type-aware source census and final switch inventories | Every first-party enum switch, including already-complete switches absent from diagnostics, has exactly one C/O/P disposition. Reconcile build populations, generated sources, coverage exceptions, new/moved findings and every preexisting suppression; a clean linter run alone cannot satisfy this criterion |
| AC-2 | C switch, each distinct named value | Explicit case preserves its intended existing behavior, including valid zeros and named sentinel errors |
| AC-3 | C switch, impossible unnamed internal value | No plausible fallback or silent success; retained default is a BUG-prefixed panic, unreachable from unvalidated external input |
| AC-4 | O switch, named and unknown values | Every accessible distinct named value is explicit; unknown handler remains non-panicking and has an open-set comment |
| AC-5 | P switch | Immediately preceding reasoned exhaustive-ignore directive; existing subset and nonmatching behavior preserved; no blanket/file exemption |
| AC-6 | New named member, unmarked C/O switch with default | Real le lint action reports that switch; explicit-exhaustive-switch remains false/unset |
| AC-7 | Existing suppressions, marker/check footprint | No first-party nolint:exhaustive workaround or enforce marker; replace the known untagged avoidance; remove a bespoke checker only if one actually exists at implementation time |
| AC-8 | Test-only unknown values and mask/alias cases | No new protocol support, duplicate cases, changed unknown rendering, or lost masked-code recognition |
| AC-9 | Documentation | Guide states the three classes and global old/new-code coverage; it does not claim exhaustive checks sealed-interface type switches |
| AC-10 | Final whole-tree analysis and runtime smoke | The complete native lint matrix is clean across all enabled linters and build populations, not merely exhaustive; fixture omission trips through le; changed external-input paths keep their safe outcomes |
| AC-11 | Identified runtime and tooling defects | Repair the identified causes, retain discriminating regressions and real-entry-point evidence, exercise affected race/lifecycle boundaries, and provide named-peer interoperability for protocol changes |
| AC-12 | Identified proof and documentation defects | Correct invalid fixtures and overstated claims without weakening valid obligations; renew affected RFC discrimination and independent judgments; derive published indexes from their current producers |

## TDD Test Plan

No permanent tests that merely duplicate case lists or configuration text. Add a regression test only for a changed consumer-visible boundary not already covered.

| Kind | Concrete test / probe | Purpose |
|------|-----------------------|---------|
| Lint smoke | `EnumSwitchLintContract` and `EnumSwitchSubsetContract` above | Red/green proof through the real developer entry point; scratch fixtures only |
| Existing unit | `TestDHUnsupportedGroup`, `internal/component/ike/crypto/dh_test.go` | Unsupported constructor group still returns its error rather than panicking |
| Existing unit | `TestSoftwareVersionCapabilityDecidesNothing` and `TestSoftwareVersionCapabilityIsRecordedForDisplay` | Moving code 75's declaration preserves unknown storage and negotiation results |
| Existing unit | `TestRFC7296NegotiationRefusesNullIntegrityAndNullCipher` | Moving ENCR_NULL's declaration preserves rejection; do not weaken assertions or alter its claim |
| Existing unit | `TestParseEncoderRefusesAThirdWord` | External encoder strings still fail before internal enum dispatch |
| Boundary smoke | Exported capability parser with raw unknown code; DH constructor with unknown group; CLI argument parser with unknown token | Invoke real entry APIs and check exact existing unknown/error outcomes, not bare no-panic assertions |
| Package regression | Owning tests for every edited package, through registered/admitted le commands | Preserve state transitions, output, sentinels and rejection behavior |
| Final gate | Full registered lint and relevant unit verification once after integration | No compile-out or platform-specific case omission |

| Boundary | Named edge | Unknown edge | Expected result |
|----------|------------|--------------|-----------------|
| Local enum | Zero/sentinel and highest named distinct value | An unnamed representable value | Named contract explicit; unknown BUG only after closure proof |
| Open enum | Unsupported but named code | Unknown protocol/kernel/plugin code | Existing safe rejection/preservation path; no new support |
| Aliases/masks | Equal-valued aliases and flag combinations | Unnamed value matching an existing mask | One case per value; keep existing masked behavior |

Functional/interoperability requirement: the enum edits preserve wire behavior, but the owner-authorized defect repairs can change it. Each such repair requires the applicable named-peer scenario, RFC review and discrimination evidence before landing. A lint success is not proof of protocol conformance.

## Files to Modify

| Surface | Change |
|---------|--------|
| Go files in the switch inventory below | Apply the assigned class, preserve control flow and update stale comments |
| Owning enum declarations and existing boundary tests | Resolve test-only members, aliases and genuine internal-invalid contract changes; inspect references first |
| `.golangci.yml` | Final cutover sets default-signifies-exhaustive false; keep global checking, no opt-in mode |
| `docs/contributing/ze-go-style.md` | Types that cannot lie: replace no-default wording and enum old-code exemption; mechanical table: state actual global enum checking, separate type switches |
| Source-linked subsystem pages, when a described contract changes | Update in the same batch as the code; do not bulk-rewrite unrelated pages |
| `plan/spec-enum-switch-exhaustiveness.md` | This analysis/spec record; no permanent new checker, command, config flag or runtime dependency |

## Files to Create

Only this spec is created during analysis. Implementation adds no permanent checker, wrapper or fixture solely to duplicate upstream exhaustive behavior. Any new behavioral regression test belongs beside its owning existing package tests.

### Integration Checklist

| Integration point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | No new RPC/config leaf |
| YANG validation constraints | N-A | Preserve parsers and validation |
| YANG custom validators | N-A | No new option |
| CLI commands/flags | N-A | Preserve command grammar and behavior; no lint CLI extension |
| CLI grammar | N-A | No command syntax change |
| Editor autocomplete | N-A | Preserve current descriptors |
| Functional test for new RPC/API | N-A | No new RPC/API |
| Pipe completeness | Yes | Existing command pipe cases and fallback behavior remain covered by owning tests |
| Env var registration | N-A | No new env var |
| Doctor/runtime dependency | N-A | No new daemon dependency |
| Prometheus counters | N-A | Preserve existing metric behavior |
| BGP family surface | N-A | No new family/capability/attribute support |

### Documentation Update Checklist

| # | Surface | Applies? | Action / reason |
|---|---------|----------|-----------------|
| 1 | Feature list | N-A | No user feature |
| 2 | Config syntax | N-A | No syntax change |
| 3 | CLI reference | N-A | No command added |
| 4 | API/RPC | N-A | No API change |
| 5 | Plugin guide | N-A | No plugin contract change |
| 6 | Contributor guide | Yes | Update ze-go-style.md, Types that cannot lie and linter table |
| 7 | Wire format | N-A | Preserve bytes and protocol behavior |
| 8 | Plugin SDK | N-A | Preserve transport/contracts |
| 9 | RFC behavior/claims | Conditional | Preserve existing claims; changed tagged behavior requires the applicable RFC evidence and discrimination record |
| 10 | Test infrastructure | N-A | Reuse existing lint action; scratch probes are not new infrastructure |
| 11 | Comparison | N-A | No new support claim |
| 12 | Architecture | Conditional | Update only a source-linked contract changed by BUG-only internal fallback semantics |
| 13 | Route metadata | N-A | No keys added |
| 14 | Metrics | N-A | Preserve current counters |
| 15 | Registries/inventory | N-A | No new runtime feature; preserve discovery |
| 16 | Source anchors | Yes | Derive touched-file pages with le spec citation anchors and inspect Design headers in each batch; name unchanged contracts explicitly |
| 17 | Examples | Yes | Correct any guide example of enum default policy; do not alter unrelated CLI/config examples |

### Discovery

| Question | Answer |
|----------|--------|
| Where does an agent look first? | Existing ai/INDEX.md rows for the Go style guide and le go lint |
| What prevents regression? | Upstream exhaustive in global mode, with default-signifies-exhaustive false |
| What prevents a second source of truth? | Enum declarations remain authoritative; this inventory is a dated migration record, not a runtime registry |
| What proves reachability? | The real le lint action rejects the scratch omission probe |

## Implementation Steps

1. **Establish policy and full population.** Retain the measured strict red baseline. Complete the type-aware enum-switch census described above, inspect complete switches as well as findings, and extend/recount the package inventory before assigning code batches. Correct the guide's three-class policy without yet claiming that the global cutover has landed. The omission smoke waits for step 4; it cannot prove strict enforcement against the current permissive root config.
2. **Apply the package batches below.** One owner per file. Read the enum, producer, caller/mutation references and post-switch code; apply C/O/P to the full census, not just the seed rows. Keep safe external defaults and explicit sentinels. Migrate existing exhaustive suppressions in the same package batch, not in a separate overlapping pass.
3. **Resolve declaration edges in their owning batch.** Move the test-only unsupported capability/algorithm declarations once, without enabling them. Replace variable aliases used as constant cases with the owning declared constants. Preserve hidden/unnamed zero contracts explicitly.
4. **Cut over globally.** After all batches, set the root setting false and update the guide's enforcement statement. Run EnumSwitchLintContract and EnumSwitchSubsetContract through the normal scoped le action using the final root config, with direct and aliased enum inputs. Require the expected missing-member diagnostic, not merely a nonzero exit. After removing those disposable probes, reconcile the inventory and run the full lint matrix once. No transient enforce markers, permanent adapter or bespoke checker.
5. **Repair the identified defects.** Execute the owner's expanded requirement with separate producer ownership. Preserve before-fix evidence, correct runtime and proof defects at their causes, and update affected consumers and documentation. New repair switches join the same C/O/P inventory.
6. **Verify and review.** Run the relevant boundary probes, owning regressions, lifecycle race checks and named-peer scenarios. Renew changed RFC evidence. Independent review checks closed provenance, safe open defaults, subset reasons, repair correctness, proof discrimination and source-linked docs. Close only after the complete native lint matrix and all agreed acceptance criteria are satisfied.

### Alternatives and decision

| Approach | Tradeoff | Decision |
|----------|----------|----------|
| Package-aligned batches, final global config cutover | Keeps protocol/input provenance with its owner; limits overlapping file edits; final cutover waits for every batch | Chosen |
| One atomic repo-wide switch patch | One commit but mixes internal state, external codes, masks and test fixtures; difficult to review for peer-safe behavior | Rejected |
| Opt-in markers or a bespoke Ze check | Leaves unmarked switches unchecked or duplicates upstream behavior | Rejected by owner |

Simplicity: existing cases/defaults and upstream linter only. Uniformity: one three-class policy, including old suppressions. Performance: no new runtime allocation or dispatch abstraction. Strongest risk: mistaking a typed external number for a closed internal enum.

### Critical Review Checklist

| Check | Evidence required |
|-------|-------------------|
| Completeness | Every inventory row and every final new finding has a disposition; no remaining first-party exhaustive nolint |
| Closed provenance | Constructor/validator and mutation/caller evidence, not an enum name or comment alone |
| External safety | Exact error/unknown behavior on hostile or future codes; no new peer-reachable panic |
| Subset legitimacy | The ignored switch is a projection/predicate/state-specific subset, not a complete dispatcher missing cases |
| Preservation | Named zero, sentinels, same-valued aliases, default masks, fallthrough and code after the switch retain their contract |
| Test honesty | No weakened negative test, no new unsupported protocol capability, no assertion that only echoes source wiring |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| Explicit or justified enum switches | Uncapped exhaustive-only inventory plus independent C/O/P review |
| Global linter enforcement | Real le omission smoke and final full matrix |
| Safe runtime behavior | Named boundary probes and owning package regression output |
| Correct guide | Source-aware reading and le doc check verify after implementation edits |

### Owner closure ruling, 2026-10-07

> ignore the security and green as we have another agent coding

Thomas authorized proceeding with this closure while another agent codes.
Security review, its missing checklist, and whole-tree green entry/closure
verification are **SKIPPED / UNVERIFIED by owner**, not PASS. This does not
waive the substantive deliverables, documentation, citation cleanup or
independent non-security review. Historical failures and the frozen census
remain unchanged. Closure includes no moving implementation from that agent.

The existing Deliverables table above supplies the required four verification
rows; its heading is normalized without inventing a completed review.

### Historical bounded reviews

Seven independent package reviews covered the original assigned scopes; the `configFormsLevel` P2 finding was corrected and cleared. Subsequent bounded reviews cover runtime/proof repairs, final integration and publication clearance. The final publication supplement retains complete reports from CommandPublicationReview, SourcePopulationReview and FinalAcceptanceDeltaReview. They found no remaining substantive AC-1–12 mismatch in the inspected deltas. Thomas's RFC-test approval and sibling reconciliation are resolved. Formal `ze-close` eligibility remains separate from this qualified acceptance.

| Run | Scope | Result | Evidence |
|-----|-------|--------|----------|
| FinalSpecAcceptanceReview | Whole spec before the last framing repair | Found the malformed fixtures' independent missing-endpoint rejection; required isolation and selective discrimination | Subsequent raw before/red/green logs and canonical rejudgment above close that finding |
| FramingProofRejudgment | Strengthened framing fixture and revised claims | No weakening; RFC9012-13-1 enforced, -13-2 weak because downstream route removal is not asserted | `FramingProofRejudgment-full-report.txt`; native audit-stamp output |
| StructuralFlagReconciliation | All 25 retained test-change flags | Structural explanations reconciled; owner RFC approval remains distinct | `StructuralFlagReconciliation-full-report.txt`; current weakened-test ledger and native commit preparation |
| SharedAIGPLandingReview and IntegratedInteropDocsReview | Dependency-complete interrupted AIGP cohort and shared pages | Source-backed clearance only; later native FRR red remains authoritative | Full reports and `job-enum-shared-aigp-native-acceptance-730cf9cd.log` |
| RemainingProofDocsReview | Four remaining page companions and AIGP uncertainty disclosure | No false claim or missing HEAD dependency | `RemainingProofDocsReview-full-report.txt`; final private site build/check and Chromium screenshot |
| FinalAcceptanceDeltaReview | Framing fix, renewal, final lint/race, archive packaging and source/commit relation | No new substantive mismatch. Clarification withdraws naming cleanup as an unsupported additional AC: retain the nine debts, not change tests solely to clear a gate | `FinalAcceptanceDeltaReview-history.txt` and `FinalAcceptanceDeltaReview-clarification-full-report.txt`; final source relation |

## Analysis Disposition

At the end of analysis on 2026-10-04, implementation had not been authorized and no switch had been fixed. The analyzed seed inventory has 356 sites in 243 files and 113 packages: 153 C, 42 O and 161 P. It combines 237 diagnostics, 117 suppression sites, one deliberate analyzer avoidance and one verified policy gap in an already-complete switch. These are dated lower bounds, not the population of all enum switches. The full census and implementation checkpoint above supersede them for implementation scope. The global config was true at that analysis stage.

## Measured Package Counts

Counts in this table are the 237 uncapped linter findings only. Existing suppressions and the untagged avoidance are additional inventory rows, not fabricated linter findings.

| Package | Reported switches |
|---------|-------------------|
| `cmd/ze` | 2 |
| `cmd/ze/hub` | 1 |
| `internal/chaos/orchestrator` | 2 |
| `internal/chaos/scenario` | 1 |
| `internal/chaos/web` | 1 |
| `internal/component/bgp/plugins/ls_export` | 3 |
| `internal/component/bgp/plugins/rib/storage` | 1 |
| `internal/component/bgp/plugins/rpki` | 3 |
| `internal/component/bgp/reactor` | 10 |
| `internal/component/bgp/redistribute` | 1 |
| `internal/component/bgp/wireu` | 3 |
| `internal/component/cli` | 2 |
| `internal/component/command` | 15 |
| `internal/component/config` | 10 |
| `internal/component/config/cli` | 3 |
| `internal/component/config/storage` | 1 |
| `internal/component/config/system` | 1 |
| `internal/component/config/transaction` | 3 |
| `internal/component/doctor` | 1 |
| `internal/component/iface` | 1 |
| `internal/component/ike/crypto` | 5 |
| `internal/component/ike/dataplane` | 1 |
| `internal/component/ike/engine` | 4 |
| `internal/component/ike/ipsec` | 4 |
| `internal/component/kernelcap` | 1 |
| `internal/component/l2tp` | 2 |
| `internal/component/l2tp/plugins/authlocal` | 1 |
| `internal/component/l2tp/ppp` | 11 |
| `internal/component/l2tp/pppoeclient` | 1 |
| `internal/component/mcp` | 5 |
| `internal/component/mtu/cmd` | 19 |
| `internal/component/plugin/all` | 1 |
| `internal/component/plugin/registry` | 1 |
| `internal/component/plugin/server` | 2 |
| `internal/component/radius` | 2 |
| `internal/component/support` | 1 |
| `internal/component/vpp` | 1 |
| `internal/component/web` | 3 |
| `internal/core/bgp/capability` | 2 |
| `internal/core/bgp/routeaction` | 1 |
| `internal/core/eap` | 2 |
| `internal/core/family` | 2 |
| `internal/core/ikeprobe` | 2 |
| `internal/core/ipsecinventory` | 1 |
| `internal/core/probe` | 7 |
| `internal/core/selector` | 1 |
| `internal/exabgp/bridge` | 1 |
| `internal/le` | 1 |
| `internal/le/cli/catalog` | 1 |
| `internal/le/job` | 1 |
| `internal/le/repo/changed` | 1 |
| `internal/le/rfc` | 2 |
| `internal/le/site` | 1 |
| `internal/le/spec/status` | 1 |
| `internal/le/test/qemu` | 5 |
| `internal/le/weekly` | 1 |
| `internal/plugins/exabgp/bridgerun` | 1 |
| `internal/plugins/fib/kernel` | 3 |
| `internal/plugins/firewall/nft` | 1 |
| `internal/plugins/firewall/vpp` | 1 |
| `internal/plugins/flowspec-firewall` | 2 |
| `internal/plugins/geodns` | 2 |
| `internal/plugins/iface/netlink` | 3 |
| `internal/plugins/iface/vpp` | 1 |
| `internal/plugins/isis` | 7 |
| `internal/plugins/isis/adjacency` | 1 |
| `internal/plugins/isis/lsdb` | 3 |
| `internal/plugins/isis/packet` | 5 |
| `internal/plugins/isis/transport` | 2 |
| `internal/plugins/ldp` | 1 |
| `internal/plugins/memlock` | 1 |
| `internal/plugins/mrt` | 1 |
| `internal/plugins/ntp` | 1 |
| `internal/plugins/ospf` | 22 |
| `internal/plugins/ospf/lsdb` | 1 |
| `internal/plugins/ospf/neighbor` | 1 |
| `internal/plugins/ospf/packet` | 1 |
| `internal/plugins/ospf/redistribute` | 1 |
| `internal/plugins/ospf/spf` | 1 |
| `internal/plugins/ospf/sr` | 1 |
| `internal/plugins/ospf/types` | 3 |
| `internal/plugins/ospf/v3/types` | 1 |
| `internal/plugins/rsvpte` | 1 |
| `internal/test/cli` | 1 |
| `internal/test/fixture` | 1 |
| `internal/test/runner` | 3 |

## Commit Batches

Seven initial package-aligned code batches, then one global cutover commit. Counts below include diagnostics, existing suppressions, the known avoidance and G001. They are lower bounds: the full census in step 1 updates the counts and splits a batch further when necessary. Each code commit includes its contract-test/comment/doc updates; these are not seven competing writers to the same file. Final reconciliation can add sites but cannot silently omit them.

| Batch | Switch sites | Files | C | O | P | Ownership |
|-------|--------------|-------|---|---|---|-----------|
| 1. Commands | 55 | 28 | 34 | 0 | 21 | CLI, command, MTU commands, MCP |
| 2. ConfigPlatform | 66 | 45 | 34 | 8 | 24 | Config, plugin infrastructure, web, probes and platform consumers |
| 3. Tools | 45 | 38 | 20 | 1 | 24 | cmd, le, test harness, chaos |
| 4. SecuritySubscriber | 44 | 25 | 24 | 4 | 16 | IKE/IPsec, PPP/L2TP, RADIUS, EAP |
| 5. BGP | 85 | 62 | 18 | 18 | 49 | BGP engine/core, ExaBGP, capability declaration edge |
| 6. RoutingPlugins | 28 | 20 | 17 | 4 | 7 | IS-IS, LDP, RSVP, MRT, FIB, FlowSpec firewall |
| 7. OSPF | 33 | 25 | 6 | 7 | 20 | OSPF packet, LSDB, export and state consumers |
| 8. Global cutover | All | Config and guide | - | - | - | Root setting false, enforcement wording, final matrix and omission smoke |

## Switch Classification Inventory

This is the dated seed inventory, not a generated-code allowlist or a complete census. D identifiers are linter diagnostics, S identifiers are existing suppressions, B identifies the untagged avoidance, and G identifies an already-complete switch with a policy gap. Repeated function names refer to distinct switches in source order. The class policy above supplies the required edit; the basis explains why that class applies. Recheck producer evidence before a panic edit. Step 1 must inventory, classify and audit the defaults and post-switch outcomes of already-complete enum switches too; upstream exhaustive does not inspect those outcomes.

### Commands

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| G001 | `internal/component/command/argvalidate.go` | ValidateArgString / `command.ArgKind` | C | Complete validation dispatcher over internal argument descriptors; every named case exists, but the default returns nil for an unknown kind. Preserve named validation and replace the impossible-value success with a BUG assertion after caller/producer proof. |
| S065 | `internal/component/cli/completer.go` | Completer.TypeHint / `gyang.TypeKind` | P | Presentation override gives friendly hints for string, uint8/16/32, bool and enum over generic schema-name rendering; unions are handled after the switch. |
| S066 | `internal/component/cli/completer_validate.go` | validateYangType / `gyang.TypeKind` | P | Best-effort completion filter recognizes union, string patterns, uint8/16/32, bool and enum; other kinds intentionally remain eligible rather than implementing authoritative validation. |
| D029 | `internal/component/cli/editor.go` | pendingChangeKey / `config.PendingChangeKind` | P | Rename alone uses two paths; every other change uses the generic path/member key. |
| S067 | `internal/component/cli/editor_commit.go` | appendStructuralOpConflict / `config.StructuralOpType` | P | Staleness pre-scan specializes Rename source/destination and InsertMember reference checks, not execution of every structural operation. |
| S068 | `internal/component/cli/editor_draft.go` | applyMemberOp / `config.StructuralOpType` | P | Member helper for InsertMember, DeactivateMember and ActivateMember; caller explicitly narrows to these three. |
| S069 | `internal/component/cli/editor_leaflist.go` | Editor.writeThroughMemberOp / `config.StructuralOpType` | P | Leaf-list operation specialization for insert/deactivate/activate; other structural execution lives elsewhere. |
| S070 | `internal/component/cli/editor_test.go` | TestCmdCommitSessionConflictFormatting / `contract.ConflictType (cli alias)` | C | Full conflict-format dispatcher, not subset observer. Enum is exactly Live=0 and Stale=1, both rendered. |
| S071 | `internal/component/cli/model_commands_commit.go` | Model.cmdCommitSession / `contract.ConflictType (cli alias)` | C | Formats every returned conflict; omitting a future conflict kind would be incomplete dispatch. |
| D030 | `internal/component/cli/model_commands_session.go` | formatChangeEntry / `config.PendingChangeKind` | C | Complete formatting dispatch over internally constructed changes. |
| D031 | `internal/component/command/argvalidate.go` | Constraint / `command.ArgKind` | C | Kind describes a Ze argument definition, not an operator-provided code. |
| D032 | `internal/component/command/pipe.go` | foldFilters / `command.pipeKind` | P | Only selected command-owned filters are folded; unrelated operators must remain in the chain. |
| S072 | `internal/component/command/pipe.go` | collectPipeMeta / `pipeKind` | P | Records row-selection provenance only: Match, Count, valid First/Last and command-owned/alias Unknown filters. Formats/enrichment/display/paging do not explain row inclusion. |
| D033 | `internal/component/command/pipe.go` | validatePipeArgument / `command.PipeArgKind` | C | Complete dispatch over the catalog's internal argument contracts. |
| D034 | `internal/component/command/pipe.go` | shapeDescription / `command.AnswerShape` | C | Shape is a registered internal descriptor; external spellings are parsed to known shapes. |
| D035 | `internal/component/command/pipe.go` | isDataTransformOp / `command.pipeKind` | P | Membership predicate for answer-changing operators. |
| D036 | `internal/component/command/pipe.go` | isStructuredTransformOp / `command.pipeKind` | P | Membership predicate for structured transforms, not complete execution dispatch. |
| D037 | `internal/component/command/pipe.go` | isLineTransformOp / `command.pipeKind` | P | Membership predicate for operations meaningful over rendered lines. |
| S073 | `internal/component/command/pipe.go` | processStreamPipes / `pipeKind` | P | Extracts Resolve/Origin flags; other operations remain in the later stream pipeline. |
| D038 | `internal/component/command/pipe_catalog.go` | AnswerShape.String / `command.AnswerShape` | C | Closed descriptor vocabulary; raw plugin shape strings are normalized rather than cast. |
| D039 | `internal/component/command/pipe_catalog.go` | PipeClass.String / `command.PipeClass` | C | Catalog-owned descriptor, never an operator string cast. |
| D040 | `internal/component/command/pipe_catalog.go` | PipeArgKind.String / `command.PipeArgKind` | C | All argument kind values originate in the compiled catalog. |
| D041 | `internal/component/command/pipe_catalog.go` | PipeRepeat.String / `command.PipeRepeat` | C | Compiled operator repetition contract. |
| D042 | `internal/component/command/pipe_catalog.go` | PipeOperator.ArgHint / `command.PipeArgKind` | C | Complete rendering dispatch for catalog argument descriptors. |
| S074 | `internal/component/command/pipe_columns.go` | columnsInChain / `pipeKind` | P | Projects Display and Fill into columnRequest; does not execute the other operators. |
| S075 | `internal/component/command/pipe_records.go` | applyPipesRecords NDJSON specialization / `pipeKind` | P | Overrides Match, Count, First and Last after NDJSON rendering for line semantics; other kinds continue to normal dispatch below. |
| D043 | `internal/component/command/pipe_records.go` | applyRecordOp / `command.pipeKind` | P | Record-stage specialization: count/display have preceding handling and formatting belongs to later stages. |
| S076 | `internal/component/command/pipe_records.go` | positionalAddressFields / `pipeKind` | P | Address-enrichment specialization derives suffixes for Resolve and Origin only. |
| S077 | `internal/component/command/pipe_records.go` | transformPositionalAddressItem / `pipeKind` | P | Executes two positional address enrichments, not arbitrary pipe operations. |
| D044 | `internal/component/command/render_records.go` | recordRendererKeeps / `command.pipeKind` | P | Selects the renderer's subset after record transforms, not a complete operator dispatcher. |
| D045 | `internal/component/command/usage.go` | usageValues / `command.ArgKind` | C | Ordinary descriptor dispatch already explicitly handles non-enumerated string and integer kinds. |
| D096 | `internal/component/mcp/auth.go` | AuthMode.String / `mcp.AuthMode` | C | Configuration strings are normalized into Ze auth strategies, not numeric wire values. |
| D097 | `internal/component/mcp/bearer.go` | buildAuthenticator / `mcp.AuthMode` | P | Helper implements only static bearer strategies; OAuth construction belongs to the caller. |
| D098 | `internal/component/mcp/jwt.go` | hashBytes / `crypto.Hash` | P | Intentional JWT hashing specialization; the imported hash universe is larger than the supported JWT algorithms. |
| D099 | `internal/component/mcp/streamable_auth.go` | buildAuthForMode / `mcp.AuthMode` | C | Complete dispatch over normalized internal authentication strategies. |
| D100 | `internal/component/mcp/tools.go` | TaskSupportLevel.String / `mcp.TaskSupportLevel` | C | Internal tool descriptor built from string metadata. |
| D101 | `internal/component/mtu/cmd/arith.go` | ipFamily.String / `cmd.ipFamily` | C | Family is derived from parsed addresses, not cast from a wire family code. |
| D102 | `internal/component/mtu/cmd/arith.go` | minimumMTU / `cmd.ipFamily` | C | Internal normalized family, with legitimate missing-family error behavior. |
| D103 | `internal/component/mtu/cmd/arith.go` | mss / `cmd.ipFamily` | C | Complete arithmetic dispatch on internally derived address family. |
| D104 | `internal/component/mtu/cmd/mtu.go` | runStatus.String / `cmd.runStatus` | C | Run outcome is computed by Ze. |
| D105 | `internal/component/mtu/cmd/mtu.go` | runVerdict.String / `cmd.runVerdict` | C | Verdict is an internally computed decision, not an input code. |
| D106 | `internal/component/mtu/cmd/mtu.go` | noteSeverity.String / `cmd.noteSeverity` | C | Severity is selected by diagnostic code. |
| D107 | `internal/component/mtu/cmd/overhead.go` | deriveESPOverhead / `ipsecinventory.Mode` | C | Unlike encryption transform IDs, the inventory mode is normalized by Ze rather than passed through from a protocol or kernel code. |
| D108 | `internal/component/mtu/cmd/run.go` | proberKind.String / `cmd.proberKind` | C | Names the local measurement implementation chosen by the run. |
| D109 | `internal/component/mtu/cmd/run.go` | inventoryState.String / `cmd.inventoryState` | C | Internal classification of whether the inventory read answered. |
| D110 | `internal/component/mtu/cmd/run.go` | mtuRun.measure / `cmd.searchOutcome` | C | The local search algorithm constructs outcomes; received ICMP fields are not cast to this enum. |
| D111 | `internal/component/mtu/cmd/search.go` | probeOutcome.String / `cmd.probeOutcome` | C | Probe results are normalized outcomes, not raw external codes. |
| D112 | `internal/component/mtu/cmd/state.go` | tcpMTUProbing.String / `cmd.tcpMTUProbing` | C | The raw OS integer is validated and converted before this switch. |
| D113 | `internal/component/mtu/cmd/state.go` | tcpMTUProbing.finding / `cmd.tcpMTUProbing` | C | Explicit dispatch already distinguishes every valid normalized setting; only sentinel is missing. |
| D114 | `internal/component/mtu/cmd/verdict.go` | tunnelVerdict.String / `cmd.tunnelVerdict` | C | Local sizing decision, not wire code. |
| D115 | `internal/component/mtu/cmd/verdict.go` | tunnelVerdict.needsCommand / `cmd.tunnelVerdict` | P | Predicate deliberately selects only verdicts with a usable different recommendation. |
| D116 | `internal/component/mtu/cmd/verdict.go` | runVerdictOf / `cmd.tunnelVerdict` | C | Complete fold over internally produced verdicts, including explicit neutral cases. |
| D117 | `internal/component/mtu/cmd/verdict.go` | underlayOutcome.String / `cmd.underlayOutcome` | C | The advice matrix computes this enum. |
| D118 | `internal/component/mtu/cmd/verdict.go` | underlayOutcome.severity / `cmd.underlayOutcome` | C | Complete classification of advice matrix results. |
| D119 | `internal/component/mtu/cmd/verdict.go` | underlayOutcome.text / `cmd.underlayOutcome` | C | Complete text dispatch for locally computed advice outcomes. |

### ConfigPlatform

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| D046 | `internal/component/config/change_file.go` | StructuralOp.PendingChange / `config.StructuralOpType` | C | The file parser selects named operation constructors rather than casting arbitrary file text into the enum. |
| D047 | `internal/component/config/change_file.go` | PendingChange.ConflictPaths / `config.PendingChangeKind` | P | Rename is the deliberate two-path exception to ordinary Path overlap checks. |
| D048 | `internal/component/config/change_file.go` | PendingChange.Summary / `config.PendingChangeKind` | C | Complete operator-summary dispatch over locally produced pending-change variants; omitted known variants require a semantic decision, not a subset exemption. |
| D049 | `internal/component/config/change_file.go` | formatStructuralLine / `config.StructuralOpType` | C | Serialization dispatch for the same recognized structural operations. |
| D050 | `internal/component/config/cli/cmd_dump.go` | maskValue / `config.DisplayMode` | C | Display policy is selected locally, not decoded as an arbitrary numeric mode. |
| D051 | `internal/component/config/cli/cmd_migrate.go` | configMigrateWithWarnings / `config.ConfigFormat` | C | External file bytes are classified into exactly three named formats. |
| D052 | `internal/component/config/cli/cmd_validate.go` | runValidation / `config.ConfigFormat` | C | DetectFormat's output is a finite local classification even when input is untrusted configuration. |
| S078 | `internal/component/config/cli/cmd_validate.go` | yangRepair / `configyang.ErrorType` | P | Optional repair lookup supports only implemented repair recipes. |
| S079 | `internal/component/config/cli/cmd_validate.go` | yangErrorCode / `configyang.ErrorType` | C | Complete code mapping omits named zero ErrTypeUnknown. |
| S080 | `internal/component/config/parser_freeform.go` | Parser.parseFlex / `config.tokenType` | C | Full syntax-mode dispatcher, not a predicate. |
| S081 | `internal/component/config/parser_freeform.go` | Parser.parseFlexValue / `config.tokenType` | P | Enclosing loop admits only word, string, LBracket and LParen; switch specializes bracketed collection. |
| S082 | `internal/component/config/parser_freeform.go` | Parser.parseInlineList / `config.tokenType` | C | Complete attribute-value grammar dispatcher. |
| S083 | `internal/component/config/parser_freeform.go` | Parser.skipBlock / `config.tokenType` | P | Delimiter-depth scanner deliberately discards content. |
| S084 | `internal/component/config/parser_freeform.go` | Parser.collectArray / `config.tokenType` | C | Full token-to-content dispatcher including catch-all transformation. |
| S085 | `internal/component/config/parser_freeform.go` | Parser.collectParenthesized / `config.tokenType` | C | Complete content assembly dispatcher. |
| S086 | `internal/component/config/probe.go` | ProbeConfigType / `config.tokenType` | P | Top-level bgp-block predicate, not full parsing. |
| S087 | `internal/component/config/reader.go` | tokensToNestedMap / `config.tokenType` | C | Actual suppression: reader.go, switch at 143. Already lists all ten kinds. |
| S088 | `internal/component/config/reader.go` | extractBraceContent / `config.tokenType` | P | Actual suppression: reader.go, switch at 207. Brace-depth projection. |
| D053 | `internal/component/config/related.go` | RelatedPlacement.String / `config.RelatedPlacement` | C | Descriptor parser validates tokens before assigning enum. |
| D054 | `internal/component/config/related.go` | RelatedPresentation.String / `config.RelatedPresentation` | C | Descriptor presentation is validated at construction. |
| D055 | `internal/component/config/related.go` | RelatedClass.String / `config.RelatedClass` | C | Styling class is a validated descriptor classification. |
| D056 | `internal/component/config/related.go` | RelatedEmpty.String / `config.RelatedEmpty` | C | Validated empty-placeholder policy. |
| D057 | `internal/component/config/schema.go` | parseNumericRangeValue / `config.ValueType` | P | Numeric range parsing intentionally supports numeric types only. |
| D058 | `internal/component/config/setparser.go` | NormalizeLeafValue / `config.ValueType` | P | Only bool and ASN have canonicalization rules; other accepted strings must remain untouched. |
| D059 | `internal/component/config/storage/source.go` | ReadConfigSource / `storage.ConfigSource` | C | Source identity is bound to an owned storage wrapper, not read as a file enum. |
| D060 | `internal/component/config/system/doctor.go` | checkResolvConfPath / `host.PlatformType` | P | This diagnostic only compares gokrazy and conventional Linux resolver-path conventions. |
| S089 | `internal/component/config/tokenizer_test.go` | TestTokenizerNestedBraces / `config.tokenType` | P | Brace-balance projection in test. |
| D061 | `internal/component/config/transaction/executor.go` | settlementResource / `transaction.SettlementResourceSource` | C | Registered in-process settlement rules select operation fields. |
| D062 | `internal/component/config/transaction/solver.go` | addsAddressing / `rpc.OperationVerb` | P | Predicate deliberately recognizes additions/modifications, not destroy operations. |
| D063 | `internal/component/config/transaction/solver.go` | startsABinder / `rpc.OperationVerb` | P | Predicate selects starts/changes; stops deliberately do not qualify. |
| S090 | `internal/component/config/yang/command.go` | yangTypeToArgDef / `goyang.TypeKind` | O | Actual suppression: yang/command.go, switch at 662. Complete supported/unsupported conversion; unsupported default is not grounds for exemption. |
| S091 | `internal/component/config/yang/validator.go` | ErrorType.String / `configyang.ErrorType` | C | Actual suppression: yang/validator.go, switch at 35. Complete local rendering omits named zero. |
| S092 | `internal/component/config/yang/validator.go` | Validator.validateYangType / `goyang.TypeKind` | O | Actual suppression: yang/validator.go, switch at 221. Full validation dispatcher currently accepts unimplemented types; not a predicate. |
| S093 | `internal/component/config/yang_schema.go` | yangToNode / `goyang.EntryKind` | P | Actual suppression: yang_schema.go, switch at 375. Selective config-node projection of leaves/directories. |
| S094 | `internal/component/config/yang_schema.go` | yangTypeToValueType / `goyang.TypeKind` | O | Actual suppression: yang_schema.go, switch at 1117. Full representation mapping, not a predicate. |
| D064 | `internal/component/doctor/checks_linux.go` | checkRandomSeed / `host.PlatformType` | P | Checks known Linux seed-service arrangements only. |
| D065 | `internal/component/iface/link_queue_test.go` | TestSubscribersHandOffRatherThanApply / `iface.linkEventClass` | P | Fixture emits carrier/router events only and deliberately fails any other event class. |
| D080 | `internal/component/kernelcap/kernelcap.go` | State.String / `kernelcap.State` | C | Kernel facts are classified into local verdicts, not cast into State. |
| D120 | `internal/component/plugin/all/config_claims_test.go` | TestClaimAllowlistReasons / `claims.Kind` | P | Test isolates allowlist hygiene; other claim problems have separate tests. |
| D121 | `internal/component/plugin/registry/setup.go` | SetupOutcome.String / `registry.SetupOutcome` | C | Registry records locally produced setup results and refuses invalid outcomes. |
| D122 | `internal/component/plugin/server/command.go` | selectorMatchesPeer / `selector.Kind` | P | Only name and ASN require peer metadata; IP-shaped matching is delegated. |
| D123 | `internal/component/plugin/server/failure_policy.go` | Server.applyFailurePolicy / `rpc.FailurePolicy` | O | Policy is a plugin registration input; text decoding rejects unknown spellings, but complete proof of every registration path's validation was not established. |
| D126 | `internal/component/support/collect_linux.go` | tableFamilyName / `nftables.TableFamily` | O | Family comes from kernel nftables readback. |
| S102 | `internal/component/traffic/config.go` | parseFilterValue / `traffic.FilterType` | C | Already lists four named values, including filterUnknown; suppression stale. |
| D127 | `internal/component/vpp/govpp_logrus.go` | slogLevelFromLogrus / `logrus.Level` | O | Hook consumes third-party log entries; no range-validation boundary is shown. |
| D128 | `internal/component/web/editor.go` | EditorManager.Diff / `contract.PendingChangeKind` | C | Production editor adapter copies locally constructed pending-change kinds. |
| D129 | `internal/component/web/editor.go` | EditorManager.pendingChangePaths / `contract.PendingChangeKind` | P | Rename is the only two-location path-marking specialization. |
| D130 | `internal/component/web/ui_mode.go` | UIMode.String / `web.UIMode` | C | Cookies/env text is normalized to one of two local values. |
| D141 | `internal/core/probe/df.go` | DFMode.String / `probe.DFMode` | C | CLI mode text is converted by a rejecting parser into local enum values. |
| D142 | `internal/core/probe/errqueue.go` | ErrQueueOutcome.String / `probe.ErrQueueOutcome` | C | Network/kernel errors are locally classified, not cast into outcome values. |
| D143 | `internal/core/probe/icmp.go` | Family.String / `probe.Family` | C | Family is derived from an address with a total local classifier. |
| D144 | `internal/core/probe/icmp.go` | Family.network / `probe.Family` | C | Resolver family is the local Family classification. |
| D145 | `internal/core/probe/socket.go` | SocketKind.String / `probe.SocketKind` | C | Socket kind is assigned by successful local open branches. |
| D146 | `internal/core/probe/socket.go` | Family.icmpNetwork / `probe.Family` | C | Local address-family enum must explicitly reject its no-family sentinel. |
| D147 | `internal/core/probe/socket_linux.go` | pmtuDiscValue / `probe.DFMode` | C | Locally selected DF policy becomes a kernel socket option. |
| D148 | `internal/core/selector/selector.go` | Selector.MatchesPeerKey / `selector.Kind` | C | Complete key-matching dispatch; ASN has a documented false result rather than being an accidental missing arm. |
| D168 | `internal/plugins/firewall/nft/lower_linux.go` | tableNFProto / `nftables.TableFamily` | P | Extracts a single IP family only; multi-family and non-IP tables intentionally have no single nfproto. |
| D169 | `internal/plugins/firewall/vpp/backend_linux.go` | hookIsInput / `firewall.ChainHook` | C | Binary direction mapping for the full configured hook set, not an incomplete feature handler. |
| D172 | `internal/plugins/geodns/rfc2181_config_test.go` | TestParseConfigMixedAddresses / `geodns.recordKind` | P | Test detects presence of both address record families, not all DNS record kinds. |
| D173 | `internal/plugins/geodns/server.go` | recordRR / `geodns.recordKind` | C | DNS query type does not become recordKind; records are created by validated config builders. |
| D174 | `internal/plugins/iface/netlink/show_linux.go` | macvlanModeName / `netlink.MacvlanMode` | P | Documented readback specialization names only the two modes Ze creates; other deliberately means drift, not a missing pretty-printer name. |
| D175 | `internal/plugins/iface/netlink/xfrm_linux.go` | xfrmDirString / `netlink.Dir` | O | Unvalidated kernel XFRM policy direction. |
| D176 | `internal/plugins/iface/netlink/xfrm_linux.go` | xfrmModeString / `netlink.Mode` | O | Mode comes directly from kernel policy templates. |
| D177 | `internal/plugins/iface/vpp/tunnel.go` | vppBackendImpl.CreateTunnel / `iface.TunnelKind` | P | Backend intentionally implements only its supported tunnel subset and rejects every other kind. |
| D197 | `internal/plugins/memlock/memlock_linux_test.go` | TestMemlockRecordsItsOutcome / `registry.SetupOutcome` | P | Test accepts exactly the plugin's success/soft-failure outcomes and rejects everything else. |
| D199 | `internal/plugins/ntp/doctor.go` | clockNoSyncSeverity / `host.PlatformType` | P | Disabled clock synchronization is diagnosed only on platforms where Ze defines an operational policy. |

### Tools

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| D001 | `cmd/ze/help_command.go` | argKindString / `command.ArgKind` | C | Complete argument-category renderer, not a specialization. Schema types are converted to internal named categories. |
| D002 | `cmd/ze/hub/service_web.go` | startWebServer (showHandler closure) / `web.UIMode` | C | Although selected through HTTP/config, the actual enum is normalized to a closed set before dispatch. |
| D003 | `cmd/ze/ze_core_pipe.go` | pipeUsage / `command.PipeClass` | C | Complete grouping of the in-process static operator catalog. |
| S001 | `internal/chaos/inprocess/chaos.go` | endsSession / `engine.ActionType` | P | Predicate selects guaranteed-session-ending TCPDisconnect, NotificationCease, DisconnectDuringBurst, ReconnectStorm and HoldTimerExpiry; collision need not end the original session. |
| S002 | `internal/chaos/inprocess/runner.go` | Run event-draining goroutine / `peer.EventType` | P | Tracks Established, Disconnected and ChaosExecuted for establishment and settle accounting; all events still reach Consumer and collectedEvents. |
| S003 | `internal/chaos/inprocess/runner_test.go` | TestInProcessEventLogFormat / `peer.EventType` | P | Presence assertion observes Established and RouteSent; timestamp assertion separately checks every event. |
| S004 | `internal/chaos/inprocess/runner_test.go` | TestInProcessDisconnectReconnect / `peer.EventType` | P | Counts only peer-zero Established and Disconnected transitions for reconnect timing bounds. |
| S005 | `internal/chaos/inprocess/runner_test.go` | TestInProcessChaosReconnect / `peer.EventType` | P | Counts Established and Disconnected to require a chaos-induced fall and recovery. |
| D004 | `internal/chaos/orchestrator/cli.go` | CLIRun / `scenario.Target` | C | Complete target-to-config dispatch after validated CLI parsing. |
| D005 | `internal/chaos/orchestrator/fork.go` | forkDaemon / `scenario.Target` | P | This helper deliberately launches only external FRR/BIRD daemons; Ze has a separate launch path. |
| D006 | `internal/chaos/scenario/target.go` | Target.DefaultBinary / `scenario.Target` | C | Complete binary-name mapping for validated target selection. |
| S006 | `internal/chaos/shrink/causal.go` | removeWithDependents precondition switch / `peer.EventType` | P | Selects establishment-dependent RouteSent, RouteReceived, RouteWithdrawn, EORSent, WithdrawalSent, ChaosExecuted, Error and Disconnected after the removal point. |
| S007 | `internal/chaos/shrink/causal.go` | removeWithDependents establishment tracker / `peer.EventType` | P | Only Established and Disconnected change the session-lifetime boolean. |
| S008 | `internal/chaos/validation/props_convergence.go` | ConvergenceDeadline.ProcessEvent / `peer.EventType` | P | Pairs RouteSent with RouteReceived; every event separately advances lastTime. |
| S009 | `internal/chaos/validation/props_duplicate.go` | NoDuplicateRoutes.ProcessEvent / `peer.EventType` | P | RouteSent checks/records duplicates, RouteWithdrawn clears a prefix and Disconnected resets the peer. |
| S010 | `internal/chaos/validation/props_holdtimer.go` | HoldTimerEnforcement.ProcessEvent / `peer.EventType` | P | ChaosExecuted with hold-timer-expiry starts pending expiry; Disconnected and Established clear it. |
| S011 | `internal/chaos/validation/props_ordering.go` | MessageOrdering.ProcessEvent / `peer.EventType` | P | Established/Disconnected update state; RouteSent/RouteReceived are checked against that state. |
| S012 | `internal/chaos/validation/props_route.go` | RouteConsistency.ProcessEvent / `peer.EventType` | P | Selects Established, RouteSent, RouteReceived, RouteWithdrawn and Disconnected to maintain expected/received route models; diagnostics and scheduler events are not model inputs. |
| D007 | `internal/chaos/web/render.go` | toastForEvent / `peer.EventType` | P | Toast selection intentionally excludes routine route and successful lifecycle events. |
| D150 | `internal/le/cli/catalog/catalog.go` | argumentKind / `command.ArgKind` | C | Complete category renderer over the same internal YANG command model as help_command.go. |
| D151 | `internal/le/group_test.go` | TestGateStagesAreNotWorkflowOrReport / `leroot.Group` | P | Membership assertion deliberately accepts only gate-compatible groups and fails everything else; this is not a missing renderer or dispatcher. |
| D152 | `internal/le/job/job.go` | Kind.String / `job.Kind` | C | Admission outcome is assigned by Ze's admission machinery, not read as an arbitrary registry numeric code. |
| D153 | `internal/le/repo/changed/scope.go` | ScopeReport.Text / `repochanged.printMode` | C | The production report receives a validated print mode; its zero-value report additionally has intentional packages rendering. |
| D154 | `internal/le/rfc/check_core.go` | rollupRowVerdict / `rfc.RollupState` | P | Formatting specialization adds a cause only for gap and unproven contributions; otherwise it passes the state through unchanged. |
| D155 | `internal/le/rfc/rfc.go` | RollupState.String / `rfc.RollupState` | C | Complete string representation of Ze-derived rollup states. |
| D156 | `internal/le/site/commands_test.go` | visibleText / `html.TokenType` | P | Visible-text extraction intentionally ignores tokens without visible text. |
| D157 | `internal/le/spec/status/answer.go` | Answer / `specstatus.closureAction` | C | Raw CLI words have already been classified by a total internal parser; malformed input becomes a named sentinel. |
| D158 | `internal/le/test/qemu/install.go` | InstallKind.String / `testqemu.InstallKind` | C | Installer kind comes from compiled action wrappers, not environment numeric input. |
| D159 | `internal/le/test/qemu/install.go` | InstallVerdict.String / `testqemu.InstallVerdict` | C | Ze assigns the proof verdicts internally; zero is deliberately not success. |
| D160 | `internal/le/test/qemu/install.go` | Installer.prefix / `testqemu.InstallKind` | C | Complete log-prefix dispatch for internally selected installer proofs. |
| D161 | `internal/le/test/qemu/install.go` | Installer.Execute / `testqemu.InstallKind` | C | Full execution dispatch over the internal installer action set, with a meaningful unspecified refusal. |
| D162 | `internal/le/test/qemu/netns.go` | networkNamespace.String / `testqemu.networkNamespace` | C | Namespace mode is a private, table-assigned test policy. |
| D163 | `internal/le/weekly/poster.go` | stampWanted / `weekly.Stamp` | C | Complete decision over the three internal parsed date-stamp choices. |
| S111 | `internal/test/cli/cmd_l2tp_scale.go` | lacSimulator.setupTunnel / `l2tp.AVPType` | P | Extracts MessageType, AssignedTunnelID and Challenge from setup replies; not a complete AVP dispatcher. |
| S112 | `internal/test/cli/cmd_l2tp_scale.go` | lacSimulator.setupSessions / `l2tp.AVPType` | P | Extracts MessageType and AssignedSessionID for session setup; other AVPs are outside this projection. |
| S113 | `internal/test/cli/cmd_peer_test.go` | setNonZero / `reflect.Kind` | C | Complete supported/rejected-kind dispatcher for local peer.Config fields: every considered field must get a nonzero value or fail loudly; not a subset observation predicate. |
| D233 | `internal/test/cli/cmd_vpp_stub.go` | vppStubState.reply / `api.MessageType` | O | Imported protocol message-category metadata enters from a registry interface without demonstrated category validation. It is not a raw peer-supplied numeric field, but closing the interface requires additional proof. |
| D234 | `internal/test/fixture/ui_fixture_cli_format_default.go` | cliFormatDefaultLength / `reflect.Kind` | P | Length is defined only for the selected length-bearing kinds; other kinds must remain an error. |
| S114 | `internal/test/peer/open_capability.go` | ownedCapabilities / `capability.Code` | P | Overrides sender-owned Role, AddPath, FQDN, GracefulRestart, LLGR, SoftwareVersion and PathsLimit values. ASN4 is handled before switch. Others are intentionally mirrored. |
| D235 | `internal/test/runner/display.go` | Display.TestFinished / `runner.State` | P | Completion-line rendering deliberately suppresses nonterminal states. |
| D236 | `internal/test/runner/parallel.go` | parallelRunner.Run (worker closure) / `runner.State` | C | This is the scheduler's complete state-finalization decision, not merely a formatting subset: every nonterminal state currently gets a result-derived terminal state. |
| D237 | `internal/test/runner/peer_contract.go` | EncodingTests.validateOnePeerBlock / `peer.Claim` | C | File text is normalized by Ze's claim parser; the returned enum has four fixed possibilities, including an explicit unclaimed result. |
| S115 | `internal/test/runner/record.go` | Record.Colored / `runner.State` | C | Complete state-to-presentation renderer; named None/Starting states are hidden in default, not an observation subset. |
| S116 | `internal/test/runner/record_collection.go` | Tests.Summary / `runner.State` | P | Projects completed Success, Fail, Timeout and Skip into four counters; progress states have no completed-result count. |
| S117 | `internal/test/runner/stress.go` | IterationStats.Add / `runner.State` | P | Counts executed Success, Fail and Timeout only; Skip deliberately is not an executed iteration outcome. |

### SecuritySubscriber

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| D066 | `internal/component/ike/crypto/cipher.go` | integrityHashFunc / `crypto.IntegrityID` | O | Cryptographic algorithm selector originates in negotiated protocol/config data; retain refusal rather than assert on unsupported algorithms. |
| D067 | `internal/component/ike/crypto/dh.go` | NewDHExchange / `crypto.DHGroupID` | O | Constructor accepts algorithm IDs from config/negotiation and must reject unsupported IDs safely. |
| D068 | `internal/component/ike/crypto/dh.go` | (*DHExchange).SharedSecret / `crypto.DHGroupID` | C | The switched group belongs to a locally constructed exchange, not the peer public-key bytes. NewDHExchange returns an exchange only for three supported groups. |
| D069 | `internal/component/ike/crypto/transform.go` | EncryptionID.String / `crypto.EncryptionID` | O | Formatting an open wire identifier must remain safe for unsupported values. |
| D070 | `internal/component/ike/crypto/transform.go` | DHGroupID.String / `crypto.DHGroupID` | O | Formats protocol IDs, including unknown and no-group values. |
| D071 | `internal/component/ike/dataplane/xfrm_migrate_linux.go` | (*xfrmMobikeBackend).prepareMigration / `dataplane.SADir` | P | Migration deliberately accepts only the inbound/outbound pair, not forwarding policies. |
| D072 | `internal/component/ike/engine/bypass_test.go` | TestIKEBypassPoliciesSelectorAndPriority / `dataplane.SADir` | C | Test checks the full direction result of a local policy producer; forwarding is a named forbidden output, not an ignored category. |
| D073 | `internal/component/ike/engine/child.go` | selectorPort / `ipsec.PortForm` | C | Form is Ze's normalized representation, not a direct cast of a peer protocol code; wire/config producers choose one of three forms. |
| D074 | `internal/component/ike/engine/eap_auth.go` | eapMethodType / `ipsec.AuthMode` | P | EAP specialization intentionally answers only EAP modes and reports non-EAP for other auth modes. |
| D075 | `internal/component/ike/engine/reconcile_test.go` | mutateForTest / `reflect.Kind` | P | Test mutation helper supports only kinds it knows how to mutate; unsupported kinds deliberately fail its caller's coverage assertion. |
| D076 | `internal/component/ike/ipsec/rfc4301_selector_set_test.go` | holdsSelector / `reflect.Kind` | P | Only collection kinds require extraction of an element type. |
| D077 | `internal/component/ike/ipsec/rfc4301_selector_set_test.go` | TestRFC4301SPDEntryCannotCarryASecondSelectorSet / `reflect.Kind` | P | Distinguishes collection fields from scalar fields for a structural test. |
| D078 | `internal/component/ike/ipsec/traffic_selector.go` | PortSelector.Wire / `ipsec.PortForm` | C | Validated constructors translate external representations into a finite internal form. |
| D079 | `internal/component/ike/ipsec/traffic_selector.go` | PortSelector.String / `ipsec.PortForm` | C | Formats the same finite normalized PortForm model. |
| D081 | `internal/component/l2tp/metrics.go` | (*l2tpStatsPoller).poll / `l2tp.L2TPSessionState` | C | This is a complete bucket classifier over Ze session FSM state, not a subset handler. |
| D082 | `internal/component/l2tp/plugins/authlocal/auth.go` | (*localAuth).handle / `ppp.AuthMethod` | P | Wire-auth dispatch occurs after the explicit no-auth path has already returned; this switch intentionally handles only the remaining methods. |
| D083 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Initial / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D084 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Starting / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D085 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Closed / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D086 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Stopped / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D087 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Closing / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D088 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Stopping / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D089 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / ReqSent / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D090 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / AckRcvd / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D091 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / AckSent / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D092 | `internal/component/l2tp/ppp/ppp_fsm.go` | LCPDoTransition / Opened / `ppp.lCPEvent` | C | Complete state/event transition table: ignored named events still have an explicit unchanged-state/no-action result, not an independent subscription subset. |
| D093 | `internal/component/l2tp/ppp/session_run.go` | (*pppSession).applyTransition / `ppp.LCPState` | P | Side-effect specialization preserves peer-termination tracking only across Stopping/Stopped; it is not the main FSM dispatcher. |
| S095 | `internal/component/l2tp/pppoeclient/session.go` | negotiateLCP / `ppp.LCPState` | C | Complete Configure-Ack transition dispatcher over locally maintained state, not a predicate. |
| D094 | `internal/component/l2tp/pppoeclient/session.go` | negotiateIPCP / ConfigureAck handling / `ppp.LCPState` | C | Complete Configure-Ack transition dispatcher over locally maintained state, like negotiateLCP. Preserve the three transitions and explicitly group other named states under their existing no-change outcome; only an unnamed impossible internal state may reach a BUG assertion. |
| S096 | `internal/component/l2tp/session_fsm.go` | parseICRQ / `l2tp.AVPType` | P | ICRQ-specific projection from shared AVP catalog. |
| S097 | `internal/component/l2tp/session_fsm.go` | parseICCN / `l2tp.AVPType` | P | ICCN connection/proxy field projection. |
| S098 | `internal/component/l2tp/session_fsm.go` | parseOCRQ / `l2tp.AVPType` | P | OCRQ-specific extraction, not universal AVP dispatch. |
| S099 | `internal/component/l2tp/session_fsm.go` | parseOCCN / `l2tp.AVPType` | P | OCCN speed/framing/sequencing projection. |
| S100 | `internal/component/l2tp/session_fsm.go` | parseCDN / `l2tp.AVPType` | P | CDN result/session/cause projection. |
| D095 | `internal/component/l2tp/subscriber_lifetime_linux_test.go` | TestL2TPSubscriberNetworkLifetime / route event callback / `redistevents.RouteAction` | C | Test asserts every action emitted by a local subscriber route producer; Unspecified is explicitly forbidden, not a legitimate unhandled event. |
| S101 | `internal/component/l2tp/tunnel_initiator.go` | parseSCCRP / `l2tp.AVPType` | P | SCCRP-specific projection from common tunnel/session catalog. |
| D124 | `internal/component/radius/authenticator.go` | (*radiusAuthenticator).credential / `radius.AuthMethod` | P | Non-EAP credential helper intentionally handles PAP/CHAP only; Authenticate routes EAP elsewhere first. |
| D125 | `internal/component/radius/config.go` | AuthMethod.EAPType / `radius.AuthMethod` | P | Queries the EAP subset of the authentication vocabulary. |
| D134 | `internal/core/eap/eap.go` | (*Session).Process / `eap.sessionState` | C | Complete dispatch over private session lifecycle state; terminal states have intentional explicit behavior to record. |
| D135 | `internal/core/eap/eap_mschapv2.go` | (*mschapv2Method).Process / `eap.mschapv2State` | C | Private method-state dispatch needs explicit Done refusal; peer opcode is separately validated. |
| S106 | `internal/core/eap/rfc3748_peer_timer_test.go` | timerField / `reflect.Kind` | P | Selected reflection traversal for timer ownership test, not universal Kind dispatch. |
| D138 | `internal/core/ikeprobe/registry.go` | Outcome.String / `ikeprobe.Outcome` | C | Outcome is synthesized by Ze's probe lifecycle, not copied from a peer code. |
| D139 | `internal/core/ikeprobe/registry.go` | Refusal.String / `ikeprobe.Refusal` | C | Refusal is an internal reason selected by the engine. |
| D140 | `internal/core/ipsecinventory/registry.go` | Mode.String / `ipsecinventory.Mode` | C | Inventory mode is normalized by a local producer, rather than directly accepting kernel or peer mode codes. |

### BGP

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| B001 | `internal/component/bgp/cli/encode.go` | cmdEncode family dispatch / `family.SAFI` | P | Unicast is encoded locally; all other families dispatch through the registered route encoder, not a static family list. |
| S013 | `internal/component/bgp/config/loader_test.go` | TestLoadReactorRouteRefreshCapabilities / `capability.Code` | P | Assertion counts two requested capabilities, not a capability dispatcher. |
| S014 | `internal/component/bgp/config/peers.go` | validatePeerProcessCaps / `capability.Code` | P | Predicate selects capabilities requiring a route-producing process. |
| S015 | `internal/component/bgp/format/decode.go` | notificationSubcodeString / `message.NotifyErrorCode` | O | Full subcode rendering dispatch with generic fallback, not a predicate. |
| S016 | `internal/component/bgp/format/text_human.go` | appendAttributeText / `attribute.AttributeCode` | O | Full attribute renderer with generic hexadecimal representation. |
| S017 | `internal/component/bgp/format/text_update.go` | appendNonUpdate / `msgtype.MessageType` | O | Formatting dispatcher chooses dedicated or raw representation for every input. |
| S018 | `internal/component/bgp/fsm/fsm.go` | FSM.handleIdle / `fsm.Event` | C | Complete state transition dispatch, including intentionally ignored events. |
| S019 | `internal/component/bgp/fsm/fsm.go` | FSM.handleConnect / `fsm.Event` | C | Full state dispatcher; state-specific errors are real behavior, not grounds for an exemption. |
| S020 | `internal/component/bgp/fsm/fsm.go` | FSM.handleActive / `fsm.Event` | C | Full state dispatcher. |
| S021 | `internal/component/bgp/fsm/fsm.go` | FSM.handleOpenSent / `fsm.Event` | C | Full event/state transition table including BFD substates. |
| S022 | `internal/component/bgp/fsm/fsm.go` | FSM.handleOpenConfirm / `fsm.Event` | C | Full event dispatcher, not a subscription subset. |
| S023 | `internal/component/bgp/fsm/fsm.go` | FSM.handleEstablished / `fsm.Event` | C | Complete event handling including unexpected messages. |
| S024 | `internal/component/bgp/message/notification.go` | Notification.subcodeString / `NotifyErrorCode` | O | Complete rendering of wire error-code/subcode pairs. |
| S025 | `internal/component/bgp/message/rfc7606_shape.go` | NLRIBearingFieldCount / `attribute.AttributeCode` | P | Shape predicate counts only attributes carrying NLRI. |
| S026 | `internal/component/bgp/message/update_build_plugin.go` | UpdateBuilder.BuildPlugin / `attribute.AttributeCode` | P | Transformation intercepts engine-owned attributes and records ORIGIN presence; it does not decode all attributes. |
| S027 | `internal/component/bgp/plugins/adj_rib_in/rib.go` | runAdjRIBInPlugin.OnStructuredEvent callback / `rpc.EventKind` | P | Adj-RIB-In consumes state and update events only. |
| S028 | `internal/component/bgp/plugins/adj_rib_in/rib.go` | AdjRIBInManager.dispatch / `rpc.EventKind` | P | Same selective consumer on JSON rail. |
| S029 | `internal/component/bgp/plugins/adj_rib_in/rib.go` | AdjRIBInManager.handleReceived / `routeaction.Action` | P | Family-operation Add/Del specialization over an enum also used for best-change Update/Withdraw. |
| S030 | `internal/component/bgp/plugins/bmp/bmp.go` | BMPPlugin.processInitiation / `uint16 TLV.Type (not a defined enum)` | O | Initiation TLV dispatcher with unknown TLVs ignored. |
| S031 | `internal/component/bgp/plugins/bmp/bmp_events.go` | BMPPlugin.handleStructuredEvent (pre-sender switch) / `rpc.EventKind` | P | Cache maintenance is a projection before later full sender handling. |
| S032 | `internal/component/bgp/plugins/bmp/bmp_events.go` | BMPPlugin.handleStructuredEvent (nested state switch) / `rpc.SessionState` | C | Up/Down are the entire actionable lifecycle enum, not a broader set of connection states. |
| S033 | `internal/component/bgp/plugins/bmp/bmp_events.go` | BMPPlugin.handleStructuredEvent (sender switch) / `rpc.EventKind` | P | BMP sender consumes session state plus six wire-message-related kinds, not EOR/negotiated/control event vocabulary. |
| S034 | `internal/component/bgp/plugins/bmp/bmp_events.go` | BMPPlugin.handleSenderState / `rpc.SessionState` | C | Complete Up/Down lifecycle handling. |
| S035 | `internal/component/bgp/plugins/bmp/msg.go` | DecodeMsg / `uint8 CommonHeader.Type (not a defined enum)` | O | Complete BMP wire-message dispatch. |
| S036 | `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` | parseNLRI / `family.SAFI` | C | Caller narrows config/plugin family before prefix parser; generic default is not evidence of an open input here. |
| S037 | `internal/component/bgp/plugins/gr/gr.go` | grPlugin.handleStructuredEvent / `rpc.EventKind` | P | GR structured rail owns state and OPEN; EOR uses text rail. |
| S038 | `internal/component/bgp/plugins/gr/gr.go` | grPlugin.handleStructuredState / `rpc.SessionState` | C | Full typed lifecycle dispatch, not arbitrary FSM state subset. |
| D008 | `internal/component/bgp/plugins/ls_export/export_encode.go` | encodeTopology / `linkstateevents.Protocol` | P | Only IGP protocols use the unreachable-node exclusion set; this is not protocol dispatch. |
| D009 | `internal/component/bgp/plugins/ls_export/export_plugin.go` | runTopologyExporter event callback / `rpc.EventKind` | P | Subscribed state/refresh events update export lifecycle; other event kinds are irrelevant. |
| D010 | `internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go` | epeProofBus.Emit / `mplsfib.Action` | C | Test bus observes native EPE producer operations, not a peer-supplied numeric action. |
| S039 | `internal/component/bgp/plugins/nlri/flowspec/plugin_decode.go` | formatWithOperator / `FlowOperator bit flags` | P | Comparison-bit projection excludes framing flags and shares same-valued aliases with bitmask operators. |
| S040 | `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` | componentMaxValue / `FlowComponentType` | P | Numeric-component specialization called only from numeric text parser, not full component dispatch. |
| S041 | `internal/component/bgp/plugins/nlri/flowspec/types_numeric.go` | numericComponent.numericString / `FlowOperator bit flags` | P | Numeric comparison rendering, excluding framing flags and protocol's special spelling. |
| S042 | `internal/component/bgp/plugins/nlri/ls/types.go` | BGPLSProtocolID.String / `BGPLSProtocolID` | O | Full wire protocol identifier renderer. |
| S043 | `internal/component/bgp/plugins/nlri/ls/types.go` | parseBGPLS / `BGPLSNLRIType` | O | Complete NLRI wire decoder. |
| S044 | `internal/component/bgp/plugins/persist/server.go` | RunPersistServer.OnStructuredEvent callback / `rpc.EventKind` | P | Persistence consumes state, sent update and received OPEN. |
| S045 | `internal/component/bgp/plugins/rib/rib.go` | RIBManager.dispatch / `rpc.EventKind` | P | Route-storage subscription dispatch excludes non-route events. |
| S046 | `internal/component/bgp/plugins/rib/rib.go` | RIBManager.handleSent / `routeaction.Action` | P | FamilyOperation Add/Del specialization; shared Action also contains best-change Update/Withdraw. |
| S047 | `internal/component/bgp/plugins/rib/rib_structured.go` | RIBManager.dispatchStructured / `rpc.EventKind` | P | Structured RIB subscription subset; BoRR/EoRR use text dispatch. |
| S048 | `internal/component/bgp/plugins/rib/ribout_entry.go` | reconstructRoute / `attribute.AttributeCode` | P | Display-field projection, not the full replay attribute dispatcher. |
| S049 | `internal/component/bgp/plugins/rib/storage/attrparse.go` | ParseAttributes / `attribute.AttributeCode` | O | Full storage dispatch routes each attribute to specialized or opaque pool; generic storage is not a predicate. |
| D011 | `internal/component/bgp/plugins/rib/storage/familyrib.go` | IsCIDRFamily / `family.SAFI` | P | Predicate selects the three storage shapes that can use CIDR keys; all others use opaque storage. |
| D012 | `internal/component/bgp/plugins/rpki/aspa_verify.go` | verifyASPAPath / `rpki.aspaMode` | C | Configured strings are mapped to one of three internal algorithm choices before this dispatch. |
| D013 | `internal/component/bgp/plugins/rpki/rpki.go` | rpkiOriginASFromASPath / `attribute.ASPathSegmentType` | O | Origin computation consumes received path content; conservatively retain unknown-segment safety rather than infer every AttributesWire producer is validated. |
| D014 | `internal/component/bgp/plugins/rpki/rpki_config.go` | actionSource.String / `rpki.actionSource` | C | This enum records Ze's precedence decision, not the operator's raw enum. |
| S050 | `internal/component/bgp/plugins/rr/rr.go` | runRouteReflector.OnStructuredEvent callback / `rpc.EventKind` | P | Reflector consumes Update/State/Open subset. |
| S051 | `internal/component/bgp/plugins/rr/rr.go` | routeReflector.handleStructuredState / `rpc.SessionState` | C | Up and Down are all real typed lifecycle states; 'connected' default comment is stale. |
| S052 | `internal/component/bgp/plugins/rs/server.go` | RunRouteServer.OnStructuredEvent callback / `rpc.EventKind` | P | Route-server consumes route/lifecycle/negotiation/refresh subset of RPC events. |
| S053 | `internal/component/bgp/reactor/filter/loop.go` | LoopIngress [actual internal/component/bgp/reactor/filter/loop.go] / `attribute.AttributeCode` | P | Loop-check predicate selects AS_PATH, ORIGINATOR_ID and CLUSTER_LIST. |
| D015 | `internal/component/bgp/reactor/filter_chain.go` | formatFilterAttrs / `reactor.filterAttrID` | P | Formatting exceptions for NLRI and flag-only attributes sit above a generic name/value formatter. |
| D016 | `internal/component/bgp/reactor/forward_body.go` | fwdReencodeMPAttributes / `attribute.AttributeCode` | P | Only MP_REACH and MP_UNREACH carry the NLRI framing this pass changes. |
| D017 | `internal/component/bgp/reactor/forward_validation.go` | validationBuildWire / `attribute.AttributeCode` | P | Selective replacement of NLRI-bearing attributes, not complete attribute dispatch. |
| D018 | `internal/component/bgp/reactor/peer.go` | Peer.resolveNextHop / `types.NextHopPolicy` | C | Ze constructs the policy discriminator separately from the externally supplied address. |
| D019 | `internal/component/bgp/reactor/peer.go` | rfc8950Family / `family.SAFI` | P | Helper intentionally identifies a fixed subset for the existing next-hop gate. |
| D020 | `internal/component/bgp/reactor/peer.go` | handshakeInFlight / `fsm.State` | P | Only OpenSent/OpenConfirm represent the bounded pending handshake this helper counts. |
| D021 | `internal/component/bgp/reactor/reactor_api_batch.go` | reactorAPIAdapter.sendStaleReadvertiseUnit / `reactor.staleOutcome` | C | Internal decision helper constructs every outcome after translating filter results. |
| D022 | `internal/component/bgp/reactor/reactor_api_relay.go` | reactorAPIAdapter.buildRelayUpdate / `rpc.NLRIFraming` | O | Stored-route metadata comes from a plugin, including older or forked producers. |
| S054 | `internal/component/bgp/reactor/reactor_dynamic_inherit_test.go` | fillDistinctValue [actual internal/component/bgp/reactor/reactor_dynamic_inherit_test.go] / `reflect.Kind` | P | Test-only generator deliberately supports PeerSettings field kinds and errors on unsupported kinds. |
| S055 | `internal/component/bgp/reactor/reactor_notify.go` | Reactor.notifyMessageReceiver receive counters [actual internal/component/bgp/reactor/reactor_notify.go] / `msgtype.MessageType` | P | Metric projection for UPDATE/KEEPALIVE, not dispatch. |
| S056 | `internal/component/bgp/reactor/reactor_notify.go` | Reactor.notifyMessageReceiver sent counters [actual internal/component/bgp/reactor/reactor_notify.go] / `msgtype.MessageType` | P | Metric projection for UPDATE/KEEPALIVE only. |
| D023 | `internal/component/bgp/reactor/rfc8277_label_rsrv.go` | mpReachLabeledNLRI / `family.SAFI` | P | This pass operates only on the two selected label-bearing SAFIs. |
| D024 | `internal/component/bgp/reactor/session_accept_own.go` | discardNonVPNAcceptOwn / `family.SAFI` | P | Selected route-distinguisher-bearing families are exceptions to the nonRD classification. |
| S057 | `internal/component/bgp/reactor/session_bfd_strict.go` | Session.bfdTeardown [actual internal/component/bgp/reactor/session_bfd_strict.go] / `fsm.Event` | P | Teardown predicate selects three BFD events; other events do not owe teardown here. |
| S058 | `internal/component/bgp/reactor/session_handlers.go` | fsmErrorNotifies / `fsm.State` | P | Boolean predicate selecting states that send an FSM-error notification. |
| S059 | `internal/component/bgp/reactor/session_handlers.go` | updateIsUnexpected / `fsm.State` | P | Boolean predicate selecting pre-established read states. |
| S060 | `internal/component/bgp/reactor/session_read.go` | Session.processMessage / `msgtype.MessageType` | O | Full received-message wire dispatch. |
| D025 | `internal/component/bgp/redistribute/producer.go` | convertBestChange / `routeaction.Action` | P | Best-change bridge deliberately maps Add/Update/Withdraw, not the separate command-level Del vocabulary. |
| S061 | `internal/component/bgp/rib/commit.go` | CommitService.packAttributesWithASPath / `attribute.AttributeCode` | P | Attribute transformation intercepts ORIGIN/LOCAL_PREF and substitutes AS_PATH/NEXT_HOP, preserving remaining attributes generically. |
| S062 | `internal/component/bgp/server/events.go` | messageTypeToEventKind / `msgtype.MessageType` | O | Full wire-type-to-event mapping with unsupported input sentinel. |
| S063 | `internal/component/bgp/server/events.go` | formatMessageForSubscription / `msgtype.MessageType` | O | Full formatter; claimed caller narrowing is not established for every batch element. |
| D026 | `internal/component/bgp/wireu/aspath_collapse.go` | CollapseAS4Family / `attribute.AttributeCode` | P | Rewrites only the four AS-path-family attributes and copies every unrelated attribute. |
| D027 | `internal/component/bgp/wireu/aspath_collapse.go` | as4FamilySpans.scan / `attribute.AttributeCode` | P | Collector locates only the four attributes needed by the collapse. |
| D028 | `internal/component/bgp/wireu/aspath_transcode.go` | TranscodeASPath / `attribute.AttributeCode` | P | Width conversion specializes the AS-path family; unrelated attributes remain byte-identical. |
| S064 | `internal/component/bgp/wireu/split.go` | separateMPAttributes / `attribute.AttributeCode` | P | Partitions MP-bearing attributes from ordinary byte-preserved attributes. |
| S103 | `internal/core/bgp/attribute/rfc6793_reconcile_test.go` | reconcileSection / `AttributeCode` | P | Fixture projection extracts four inputs to AS-path reconciliation. |
| D131 | `internal/core/bgp/capability/capability.go` | Code.String / `capability.Code` | O | Capability codes are received bytes; the reported missing member is a test-only typed declaration for code 75. |
| S104 | `internal/core/bgp/capability/capability.go` | parseCapability / `capability.Code` | O | Complete wire decoder with deliberately opaque plugin-owned and unknown capabilities. |
| D132 | `internal/core/bgp/capability/capability.go` | AddPathMode.Label / `capability.AddPathMode` | O | Direction values are cast directly from received capability tuples without vocabulary validation in the parser. |
| S105 | `internal/core/bgp/capability/negotiated_test.go` | TestNegotiateMismatches / `capability.Code` | P | Assertion checks selected mismatch categories, not complete negotiation dispatch. |
| D133 | `internal/core/bgp/routeaction/routeaction.go` | Action.Verb / `routeaction.Action` | O | Shared action mapping sits on wire/plugin and forwarding boundaries; retain safe no-op absent proof that every caller only receives validated internal actions. |
| D136 | `internal/core/family/family.go` | Family.LegacyNextHop / `family.SAFI` | P | Predicate deliberately identifies the family subset receiving the existing legacy-next-hop treatment. |
| D137 | `internal/core/family/family.go` | Family.NeedsNextHop / `family.SAFI` | P | FlowSpec families specialize the generic requirement for a next hop. |
| S107 | `internal/core/family/registry.go` | afiSlot / `family.AFI` | P | Cache-admission specialization, not protocol-family support dispatch. |
| D149 | `internal/exabgp/bridge/bridge_encoder.go` | Encoder.String / `bridge.Encoder` | C | Encoder text is validated and converted into Ze's three-value vocabulary. |
| D164 | `internal/plugins/exabgp/bridgerun/fleet.go` | Fleet.render / `bridge.Encoder` | C | Current production construction sites use validated config results or the literal JSON encoder; discriminator is not a raw script/peer byte. |

### RoutingPlugins

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| D165 | `internal/plugins/fib/kernel/mplsentry.go` | (*fibKernel).handleMPLSEntry / `mplsfib.Action` | O | Unvalidated plugin-event payload; not a private FSM value. |
| D166 | `internal/plugins/fib/kernel/mplsentry.go` | (*fibKernel).addMPLSEntryLocked / `mplsfib.Op` | O | Operation comes from the same unvalidated plugin-event entry. |
| D167 | `internal/plugins/fib/kernel/mplsentry.go` | (*fibKernel).delMPLSEntryLocked / `mplsfib.Op` | O | Operation is an unchecked plugin-event field, including on removal. |
| S108 | `internal/plugins/fib/vpp/fibvpp.go` | fibVPP.processMPLSChange / `routeaction.Verb` | C | Complete forwarding dispatcher omits explicit no-op verb. |
| S109 | `internal/plugins/fib/vpp/srv6.go` | fibVPP.processSRv6Change / `routeaction.Verb` | C | Complete SRv6 operation dispatcher omits explicit no-op verb. |
| D170 | `internal/plugins/flowspec-firewall/transport.go` | transportProtocols / `flowspec.FlowComponentType` | P | This pass only derives transport constraints; it is not the component translator. |
| D171 | `internal/plugins/flowspec-firewall/transport.go` | tcpFlagsMatch / `flowspec.FlowOperator` | P | Bitmask interpretation intentionally excludes numeric and encoding-control flags of the shared flag type. |
| D178 | `internal/plugins/isis/adjacency/adjacency.go` | State.String / `adjacency.State` | C | Local adjacency state is derived by the FSM, not cast from the peer's three-way state. |
| D179 | `internal/plugins/isis/auth_wiring.go` | isIIHType / `packet.PDUType` | P | Hello-membership predicate, deliberately not a complete PDU dispatch. |
| D180 | `internal/plugins/isis/circuits.go` | circuitLevels / `isis.Level` | C | Config boundary normalizes arbitrary input into three named levels. |
| D181 | `internal/plugins/isis/config.go` | Level.String / `isis.Level` | C | Formatting normalized local config level, not raw configuration token. |
| D182 | `internal/plugins/isis/config.go` | Level.TransportLevel / `isis.Level` | C | Complete conversion of a normalized config level. |
| D183 | `internal/plugins/isis/flooding_wiring.go` | (*engine).handleLSP / `lsdb.Freshness` | C | Complete outcome handling; the no-event Older outcome deserves an explicit case rather than a subset exemption. |
| D184 | `internal/plugins/isis/lsdb/flooding.go` | levelOf / `packet.PDUType` | P | Database-level classifier intentionally handles only LSP and SNP classes, not Hellos. |
| D185 | `internal/plugins/isis/lsdb/flooding.go` | (*Flooder).ReceiveLSP / `lsdb.Freshness` | C | Full mapping of internally computed freshness to flooding actions. |
| D186 | `internal/plugins/isis/lsdb/lsdb.go` | (*LSDB).Receive / `lsdb.Freshness` | C | Comparison outcome is manufactured locally from incoming fields, not read as an enum from wire. |
| D187 | `internal/plugins/isis/lsdb_wiring.go` | originationLevels / `isis.Level` | C | Complete mapping of normalized node configuration. |
| D188 | `internal/plugins/isis/packet/auth_types.go` | authTypeFor / `packet.AuthAlgorithm` | C | Algorithm enum is a local key-store choice, not the received authentication type byte. |
| D189 | `internal/plugins/isis/packet/auth_types.go` | digestLen / `packet.AuthAlgorithm` | C | Complete algorithm-property mapping, with named non-digest outcomes currently hidden in default. |
| D190 | `internal/plugins/isis/packet/auth_types.go` | newHash / `packet.AuthAlgorithm` | C | Complete algorithm-to-hash mapping, not arbitrary wire algorithm dispatch. |
| D191 | `internal/plugins/isis/packet/auth_verify.go` | authLayoutForReceived / `packet.pduClass` | C | Complete layout selection over a validated local class, not a wire type. |
| D192 | `internal/plugins/isis/packet/header.go` | PDUType.Level / `packet.PDUType` | O | Method classifies a protocol code with an explicit unknown-code contract; P2P has no implied level. |
| D193 | `internal/plugins/isis/server.go` | spfLevelsFor / `isis.Level` | C | Complete config-to-SPF mapping after level normalization. |
| D194 | `internal/plugins/isis/transport/multicast.go` | Level.String / `transport.Level` | C | Transport level is selected locally, not cast from incoming PDU bytes. |
| D195 | `internal/plugins/isis/transport/multicast.go` | MulticastMACForLevel / `transport.Level` | C | Send-target selector consumes locally selected transport levels; named zero means no target. |
| D196 | `internal/plugins/ldp/session.go` | (*Session).processMessages / `ldp.SessionState` | P | Guard applies only to two initialization states before normal message dispatch. |
| D198 | `internal/plugins/mrt/dump.go` | updateAddPath / `attribute.AttributeCode` | P | ADD-PATH detection deliberately inspects only attributes carrying address-family fields. |
| D232 | `internal/plugins/rsvpte/peer_identity.go` | (*engine).updatePeerIdentity / `linkstateevents.Protocol` | P | Security-relevant native-IGP allowlist, not general protocol dispatch. |

### OSPF

| ID | File | Function / enum | Class | Basis |
|----|------|-----------------|-------|-------|
| D200 | `internal/plugins/ospf/afstrategy_v6.go` | v6BuildRoutes / `v3/types.LSType` | O | Reference type is an unvalidated 16-bit body field, not an internally selected vertex kind. |
| D201 | `internal/plugins/ospf/afstrategy_v6.go` | v6SummaryReader / `v3/types.LSType` | P | This reader selects only inter-area prefix and router summaries from the whole LSDB. |
| D202 | `internal/plugins/ospf/bgpls_export.go` | (*bgplsArea).indexV2 / `types.LSType` | P | Topology correlation index deliberately selects Router and Network LSAs; opaque TE indexing is separate above the switch. |
| D203 | `internal/plugins/ospf/bgpls_export.go` | (*bgplsArea).v2 / `types.LSType` | O | This is the top-level v2 export dispatch over LSDB data, not just a selected index. |
| D204 | `internal/plugins/ospf/bgpls_export.go` | (*bgplsArea).v3 / `v3/types.LSType` | O | Complete native export dispatch receives arbitrary native LS types; RI processing currently lives in default. |
| D205 | `internal/plugins/ospf/bgpls_export.go` | (*bgplsArea).extendedV3 / `v3/types.LSType` | P | Offset specialization: only three extended types have a nonzero fixed header before the TLV stream. |
| D206 | `internal/plugins/ospf/bgpls_export.go` | (*bgplsArea).resolveV3Links / `v3/types.LSType` | P | Collects interface addresses and opaque link information only from Link and ELink LSAs. |
| S110 | `internal/plugins/ospf/cli/decode.go` | v3OfflineTypedBody / `ospfv3types.LSType` | P | Optional typed-body specialization: six base types decoded, other known/unknown types retain generic bytes. |
| D207 | `internal/plugins/ospf/decode_view.go` | opaqueScopeCommand / `ospf.OpaqueScope` | C | The command dispatcher supplies one of three literal scope constants. |
| D208 | `internal/plugins/ospf/decode_view_v3.go` | v3ScopeName / `types.LSType (masked expression)` | P | The switch classifies scope bits, not complete LSType values; its actual domain is 0x0000, 0x2000, 0x4000 and 0x6000. |
| D209 | `internal/plugins/ospf/instance.go` | (*engine).handleNeighborPacket / `ospf.PacketType` | P | Dedicated common handler for DBDesc and LSReq; other packet kinds have different handlers. |
| D210 | `internal/plugins/ospf/ipsec_install.go` | (*ipsecInstaller).setGauges / `dataplane.SADir` | C | rec.sas contains Ze-created SA records, not directions returned by the backend; zero has intentional shared-state meaning. |
| D211 | `internal/plugins/ospf/lsdb/lsdb.go` | (*LSDB).publishSizeMetricLocked / `types.LSType` | P | Two exact v2 type codes override the ordinary per-area metric-counting path; this is a scope/storage specialization, not complete protocol-body dispatch. |
| D212 | `internal/plugins/ospf/neighbor/table.go` | (*Table).setStateLocked / `neighbor.state` | P | State transition itself is already performed; this switch only applies state-specific retransmission cleanup. |
| D213 | `internal/plugins/ospf/origination_v6_ri.go` | v6RILSType / `ospf.OpaqueScope` | C | Origination code selects literal scope constants after configuration policy checks. |
| D214 | `internal/plugins/ospf/origination_v6_summary.go` | v6SummaryNetworks / `v3/types.LSType` | O | References read from received IntraAreaPrefix bodies are not validated to Router/Network. |
| D215 | `internal/plugins/ospf/packet/json.go` | lsaToJSON / `types.LSType` | P | Body-rendering specialization for implemented v2 body codecs; generic header/checksum representation is already constructed. |
| D216 | `internal/plugins/ospf/redistribute/consumer.go` | (*Consumer).injectorFor / `family.AFI` | O | Redistribution boundary accepts a family supplied by another component/plugin; IPv4 and IPv6 are the only supported injector domains. |
| D217 | `internal/plugins/ospf/rfc7474_replay_test.go` | rfc7474Packet / `packet.PacketType` | P | Test fixture intentionally constructs only Hello and LSAck packets. |
| D218 | `internal/plugins/ospf/rfc8665_mapped_sid_intra_spf_test.go` | rfc8665IntraInstall / `mplsfib.Op` | P | Transit-only classification after a separate OpPush branch has already consumed and continued past pushes. |
| D219 | `internal/plugins/ospf/rfc8665_mapped_sid_php_test.go` | rfc8665MappedInstall / `mplsfib.Op` | C | Complete classification of the three operations emitted by the internal installer, with invalid zero currently caught only by default. |
| D220 | `internal/plugins/ospf/rfc8665_mapped_sid_spf_test.go` | rfc8665SPFRead / `mplsfib.Op` | C | Complete operation classifier for captured internal SR output. |
| D221 | `internal/plugins/ospf/rfc8666_sr_reception_v6_test.go` | TestOSPFv3PrefixSIDInstallsPush / `mplsfib.Op` | P | Selects Push and Swap entries for assertions rather than classifying every captured event. |
| D222 | `internal/plugins/ospf/rfc9552_bgpls_export_route_type_test.go` | TestRFC9552ExtendedPrefixResolvesRouteType / `types.LSType` | P | Finite fixture table deliberately uses Router, SummaryNetwork, ASExternal and NSSA base advertisements. |
| D223 | `internal/plugins/ospf/spf/graph.go` | BuildGraph / `types.LSType` | P | Transit graph intentionally uses only Router and Network bodies; source-origin observation happens before filtering. |
| D224 | `internal/plugins/ospf/sr/install.go` | OutgoingLabel / `sr.OutgoingAction` | C | An internal decision result, not a decoded wire action number; zero is the valid named ActionKeep. |
| D225 | `internal/plugins/ospf/sr_fib_test.go` | TestSRFIBInstallPrefixSIDPush / `mplsfib.Op` | P | Selects the two entry kinds this assertion examines. |
| D226 | `internal/plugins/ospf/sr_install_test.go` | TestSRInstallPrefixSIDPushAndSwap / `mplsfib.Op` | P | Selects ingress Push and transit Swap for label/next-hop assertions. |
| D227 | `internal/plugins/ospf/sr_install_test.go` | TestSRInstallHeterogeneousSRGB / `mplsfib.Op` | P | Selects Push/Swap from transit-case output; the penultimate PHP scenario is asserted separately later. |
| D228 | `internal/plugins/ospf/types/lstype.go` | LSType.inScope / `types.LSType` | P | Membership predicate deliberately identifies the six implemented base v2 body types. |
| D229 | `internal/plugins/ospf/types/lstype.go` | LSType.IsOpaque / `types.LSType` | P | Membership predicate for exactly the three opaque v2 carrier codes. |
| D230 | `internal/plugins/ospf/types/lstype.go` | LSType.String / `types.LSType` | O | Human rendering of shared wire-valued types; type declaration also includes typed scope masks that are not LSAs. |
| D231 | `internal/plugins/ospf/v3/types/lsa.go` | LSType.Known / `v3/types.LSType` | O | Boundary recognition function must accept unrestricted wire codes as input and preserve masked RI recognition. |


## Implementation Summary

### What Was Implemented

The committed implementation starts with the package work recorded at
`93bb60878ad8`, the supported analyzer upgrade `d0bf51e9af43`, and checkpoint
`d9844ae724`. The continuation and publication tables above enumerate the
subsequent repairs through `c4fea842f5` and documentation commit `b783523a4c`.
The spec record was committed as `22b6b530416be2b121996748cf4f5f979382a60a`;
closure does not mistake that metadata-only diff for the implementation.
Later preserved known-red probes in `037c2d2fa1` and `7800fd95f5` are not
successful tests. The source baseline before this closure is
`f406b21e82f6`; no concurrent coding is included.

The inventory retains 834 C/O/P identities and 13461 captured observations.
Its frozen classification and provenance limits are not a fresh whole-tree
certificate. The final-proof and publication indexes retain runtime,
discrimination, independent-review and source-continuity evidence.

### Bugs Found/Fixed

The continuation table above records the causal recovery, VPP lifecycle,
protocol boundary and proof-fixture repairs with their actual positive and
negative controls. The publication supplement records source-count pruning,
literal command prose, required fixture inputs and painted-page visibility.
The original red observations remain red. No product or test code is changed
by closure.

### Documentation Updates

The durable C/O/P policy is `docs/contributing/ze-go-style.md`, “Every enum
switch has a coverage policy”, anchored to `.golangci.yml` and upstream
`switchChecker`. The publication table records the source-linked companion
pages, regenerated indexes, native documentation PASS and actual browser
evidence. Closure repoints the live validated-construction policy citation to
that guide and the BGP progress citation to the final-proof index. It changes
only the inventory's two spec provenance descriptors, not payload hashes,
dispositions or observations.

Historical `plan/handover/` references are intentionally unchanged:
`internal/le/doc/citation/policed.go`, `excludes` and `Excluded`, classifies
them as records; `internal/le/doc/check/links.go`, `sweepTracked`, preserves
that exclusion. In particular the census index's bytes stay unchanged because
the inventory pins their hash. No citation baseline is broadened.

### Deviations from Plan

The owner closure ruling above skips security review and whole-tree green
verification because another agent is coding. Neither is reported as PASS.
The initial inventory assumption was broken by the expanded census and was
replaced by the frozen reconciliation plus explicit post-capture relations.
The original tool did not enforce alias omissions; the authorized supported
upgrade, rather than a custom checker, supplies that coverage.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The initial switch list was treated as the population | Complete switches and later source deltas also matter | Native typed census and independent reconciliation | Retain frozen 834-identity inventory and source relations; A-1 broken |
| approach | Original analyzer missed alias omissions | Direct omission proof was insufficient | Real native alias smoke | Supported exhaustive upgrade; retain direct and alias red/green evidence |
| approach | DOM text was taken as publication visibility | Reveal threshold prevented painting a very long section | Retained blank screenshot and measured intersection ratio | Threshold-zero repair and actual painted-page evidence |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Explicit C/O/P coverage and global enforcement | Done with recorded evidence limits | Canonical inventory; root exhaustive config; guide | Frozen census, not a claim about another agent's moving tree |
| Identified runtime/proof repairs and VPP forwarding | Done for accepted qualified scope | Linux continuation and publication evidence tables | Historical full-unit/startup/AIGP qualifications remain; no expanded RFC claim |
| Preserve historical VRRP recorder | Done | Original evidence archive and historical-archive-proof.json | Non-executable record; original bytes retained |
| Security and whole-tree green closure checks | Skipped by owner | Owner closure ruling, 2026-10-07 | UNVERIFIED, not PASS |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done, qualified | 834-entry inventory; census and source-relation archives | 19 unavailable historical author artifacts remain a provenance limit |
| AC-2 | Done, recorded review | C dispositions and seven package reviews | Named and zero/sentinel preservation, not inferred from lint alone |
| AC-3 | Done, recorded review | Closed producer-domain justifications and boundary observations | No fresh security certification; owner skipped closure security review |
| AC-4 | Done, recorded proof | Open classifications and seven runtime boundary probes | Original exact unknown/rejection outcomes retained |
| AC-5 | Done, recorded review | P dispositions and corrected configFormsLevel classification | No blanket exemption |
| AC-6 | Done, exercised | strict-e00c79db7a884f1a84a5fcb41dbaed82 native probe | Direct and alias omission/add-case/new-member sequence |
| AC-7 | Done, recorded reconciliation | Marker reconciliation and frozen source census | No newly introduced checker or enforcement opt-in |
| AC-8 | Done, recorded proof | DH/capability/encoder boundary probes and package reviews | Unsupported values remain unsupported |
| AC-9 | Done | Guide's enum coverage policy and root config | Sealed-interface limitation remains explicit |
| AC-10 | Done, qualified | Settled-source 19-population lint and runtime probes | Historical tests are not converted to whole-tree green |
| AC-11 | Done for accepted scope, qualified | Final-proof archive and named-peer continuation table | Retain shared AIGP red; do not infer additional implementation scope |
| AC-12 | Done, qualified | Publication supplement and canonical RFC judgments | Weak judgments stay weak; publication local only |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Native omission/subset/direct/alias smoke | Observed historical red/green | Original archive, strict probe result | Not rerun against unchanged inputs |
| DH, capability, negotiation and encoder boundaries | Observed historical package/boundary proof | Original implementation evidence; final-proof continuation | Missing historical author artifacts explicitly retained |
| Owning regressions and named peers for repairs | Observed scoped proof | Final-proof archive | Exact passing and failing populations remain distinct |
| Complete lint | Observed PASS on settled source | Publication archive, landed-native-lint log | All 19 populations; three declared exceptions |
| Whole-tree unit/functional/verify-worktree | SKIPPED / UNVERIFIED by owner | Owner closure ruling | No refreshed green certificate |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Enum source packages and their existing regressions | Committed | Package checkpoints and source-continuity records above |
| .golangci.yml and supported dependency graph | Committed | Global default-signifies-exhaustive false; supported tool upgrade |
| docs/contributing/ze-go-style.md and source-linked pages | Committed | Publication evidence supplies actual derivation and visibility |
| This spec and its live citers | Closure records | Commit A preserves this record; commit B removes only this spec |

### Audit Summary

- **Total items:** 25 rows across the four audit tables.
- **Done:** 19 implemented/committed evidence rows, with qualifications stated individually.
- **Partial:** None silently accepted as full behavior.
- **Skipped:** 2 security/whole-tree verification rows, expressly authorized.
- **Changed:** 4 file-population rows distinguish existing committed implementation from closure-only records.

## Goal Validation (closure)

The earlier Goal Validation table is the detailed evidence mapping. Its limits
remain controlling; this summary does not replace the retained raw results.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Explicit policy across the complete measured population | Census and independent review | 834 identities; census index and publication source relation |
| Future omissions rejected through the real developer action | Functional negative/positive control | Original strict alias-probe result; missing member fails, explicit member passes |
| Preserve external boundaries while repairing identified defects | Runtime, race, interop and discrimination | Final-proof archive and continuation table, with AIGP/startup qualifications intact |
| Correct public claims and actual rendered artifacts | Native derivation and Chromium | Publication supplement: 995 artifacts, 22 producers, painted literal selector |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No new implementation item is transferred by this closure | Qualified scope and recorded historical limitations are unchanged; security/whole-tree closure checks are owner-skipped, not missing implementation claimed complete | None; existing BGP parent/child scopes remain open and unchanged |

## Review Gate

Round 1 scope: the closure-only spec record, two live sibling citation repoints
and two inventory provenance strings, plus the intended deletion of this spec.
Previously committed implementation is represented by the independent package,
repair and final-acceptance reviews retained in the immutable archives above;
this round does not claim a fresh rereview of every historical source file.
The independent closure context did not author that implementation.

Lenses: evidence/logic (preserve qualifiers, provenance and owner attribution)
and closure integrity (citation consumers, immutable hashes, scoped commits and
other specs' unchanged ownership). Security is SKIPPED / UNVERIFIED by owner.
There is no Go delta in this closure, so Go allocation/lifecycle/style and
protocol behavior checks have no new code to classify.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/enum-switch-exhaustiveness-7487271c-d4f1-4454-972e-5298f85f9b43.md`; four closure files hash-recorded, no implementation files included |
| `./le spec review check` | OK: clean, 0 code files; this native check does not certify historical implementation or skipped security |
| Rounds | 1 |
| Reviewer lenses used | Evidence/logic; closure/citation integrity; security explicitly skipped |
| Final findings | 0 BLOCKER, 0 ISSUE in closure-only non-security scope; record-only NOTE below |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | Deliverables heading and stale closure-entry prose needed the actual owner decision | This record | Normalize existing table and retain verbatim owner ruling; no extra review round |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Canonical inventory, final-proof index and publication archive | Yes | Read directly during closure; archive members resolve through the publication index |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1–8 | Existing census and implementation proof remain qualified | Read inventory semantics and archived final acceptance/source-continuity reports; no census or outcome rewritten |
| AC-9 | Durable guide matches global enforcement setting | Read guide C/O/P and sealed-interface paragraphs; .golangci.yml sets default-signifies-exhaustive false |
| AC-10–12 | Recorded final lint, repair and publication evidence is not whole-tree green | Read publication index and complete independent acceptance reports; owner ruling expressly skips new whole-tree verification |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Native le lint scope action | Disposable EnumSwitchLintContract/EnumSwitchSubsetContract | Historical native red/green is retained; no permanent fixture or new action introduced by closure |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Initial 356-site seed was not the full population; frozen census and post-capture relations replace it |
| A-2 | confirmed by recorded implementation review; closure security unverified | Per-switch C provenance and independent package reviews; no fresh security claim |
| A-3 | confirmed for captured population | Two excluded lexical candidates and two generated switches were built-in strings, recorded as supplemental evidence without rewriting raw errors |
| A-4 | confirmed by recorded boundary proof | Unknown capability round-trip, unsupported DH and negotiation refusals retained in original implementation evidence |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| C/O/P, old/new global enum coverage; separate sealed-interface rule | Guide; root exhaustive config; native omission smoke retained in original archive | Source/config and retained evidence read; no new behavior |
| Publication and source-linked companion pages | Publication index and full independent reports; historical Documentation tests PASSED log | Historical PASS only; no current whole-tree certificate |
| Closure citations and historical records | policed.go excludes/Excluded; links.go sweepTracked; live citer edits | Historical archive/index hashes unchanged; live policy reference repointed to surviving guide |

### Actual closure checks, 2026-10-07

All commands used `CGO_ENABLED=0`, the parent session identity and the named
`./le --name enum-resume` build.

| Command | Actual result | Scope and limit |
|---------|---------------|-----------------|
| `repo check` | PASS: all checks passed | Pre-review structural check; not whole-tree runtime proof |
| `commit audit` | CLEAN: no unexplained test weakening | HEAD to worktree; zero changed test files, not an audit of the historical implementation range |
| `spec citation` | RED: 15 unrelated dangling references plus line-token drift warnings | No enum target finding. Same 15 already recorded in `plan/journal/claim-outlives-the-evidence-it-cites.md`; no duplicate journal row |
| `doc check verify` | PASS: Documentation tests PASSED | Ran independently after citation failure; validates documentation, not runtime/security |
| `spec review record` / `spec review check` | CLEAN / OK, round 1, four hash-recorded files, zero code files | Closure-only independent non-security review; not a new certificate for historical source |
| Whole-tree lint/unit/functional/verify-worktree and security review | SKIPPED / UNVERIFIED by owner | Owner ruling above; no known red rerun |

The owner's whole-tree-green waiver also covers the unrelated global citation
red. Enum-only citation clearance remains required. The historical handover
references excluded by the citation consumer retain their original meaning,
and the live policy/progress references now resolve to surviving documents.
No baseline change, unrelated spec repair or duplicate lesson is included.
