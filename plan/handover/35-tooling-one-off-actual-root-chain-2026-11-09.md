# Tooling one-off defects: the actual-root fix chain, 2026-11-09

## Saved state after the Mac port — 2026-11-11

This section supersedes the old Next and Last run sections below. Work on this
spec continued by accident on the Mac checkout for nine commits and some
uncommitted work, stopped on 2026-11-10. That work has been carried to this
checkout by re-implementing each change against current code, and the port is
committed here: `0502b9fd20`, `b823463535`, `b87b910f0b`, `d54128e6f8`,
`0b44189f53`, `4f3d1b5c6b`, `36d3cdf3f3`, `d645d0357b`, and the plan record
`65017ff2a4`. The spec stays open; Review Gate and the two-commit closure have not happened,
and no scope was dropped.

### The nine Mac commits

| Mac commit | Work | Here |
| --- | --- | --- |
| `2fa7ad5808` | CodeQL patched-Go bootstrap | Adapted in `4f3d1b5c6b`: `2300efb0f9` already selected Go through `./le go stdpatch goroot`; added `Initialize CodeQL` after the Select step, `CGO_ENABLED=0`, and the two bootstrap tests |
| `9bd860189a` | Stop-hook recovery blocks, fences, both rule generators | Adapted into this checkout's `preserveState` in `b823463535`; fence handling, the second generator needle, seals renewed |
| `15fb445289` | Inventory interface result types | **Not ported** (see below) |
| `8dcbf9ff62` | CodeQL extraction entries and finalized-database checker | Ported in `4f3d1b5c6b` |
| `64a2928e2f` | Rule metadata polarity proof | Ported in `0502b9fd20` |
| `3514e9c43b` | Weakening approval validated before writes | Adapted in `0b44189f53`: this checkout's writer already resolved the unit (`4aafc843c0`); only the registered action's read-only preparation was ported. P-31's red walk is recorded in the evidence file (2026-11-11, red) |
| `35fc637fca` | Trailing child help words forwarded unchanged | Ported in `d54128e6f8` |
| `2290209274` | Finite inventory field equations | **Not ported** (see below) |
| `6362c8fbd5` | Typed-NLRI listener readiness, current-session EOR | Mostly already here: listener readiness is solved by the speaker redial (`ac1a28dc21`, `41fb15dad0`); the cancelled-identity test and one doc sentence were ported in `b87b910f0b` |

**The inventory work is not ported, and the actual-root chain is closed by
supersession, not by a result.** `spec-inventory-fast-registry-checks` deletes
the whole-tree external-effect proof (`Snapshot.externalCertificate`, every
`external*` and `assembly*` file) and replaces it with three fast registry
checks; its Design Insights mark the actual-root refusal chain, this
handover's old "Next" items 1 and 2 and "Last run" actual-root stop, and the
completeness half of AC-32/P-32 superseded. Porting `15fb445289`, `2290209274`
or the Mac's uncommitted ambient-capture repair would layer work onto code
that spec deletes (`ai/rules/no-layering.md`). The Mac's 30-minute timeout of
`TestAnswerAnswersTheInventory`, its CPU profile and the fixture-population
pass are history of that proof only; do not run the actual-root command from
the Mac record.

### The Mac's uncommitted work, as carried here

The regression tests are committed in `36d3cdf3f3`, which records the rfc,
deployment, commit, hookruntime, chaos/web and setup cases passing here, and
carries the two HTMX capture tests red on purpose. The evidence file's section
"Mac working-tree evidence of 2026-11-10, ported (2026-11-11)" records each
item and what it owes. In short:

- P-18: `TestGokrazyBuildCancelEndsTheGrandchildren` has the approved bounded
  wait; no baseline, cut or restored run yet. The guest (`d3195649`),
  Yangcontract (`191ba60d`) and hugepages (`58756b12`) cleanup proposals are
  approved but not applied or run; their unsafe cuts must not run first.
- P-13: the broadcast-interleaving test is committed and passes in
  `36d3cdf3f3`'s record; its cut and restored runs are owed. The two HTMX
  capture tests are committed red (`TestBrowserScreenshotNativeWaitsForHTMXBody`,
  `TestBrowserScreenshotNativeHTMXLifecycle`); the Mac's browser repair
  proposal is not applied.
- P-16: both positive-criteria entry tests are committed and pass in
  `36d3cdf3f3`'s record.
- P-14: the real vendor publication test is committed and passes in
  `36d3cdf3f3`'s record.
- P-24: `design-admission.json` carries the completed fixture, with the
  `ze-spec.md` range corrected to this checkout's lines (`211-275`). A model
  found a real nil-input scope defect in that completion. The fix proposal
  `p24-model-proof/nil-preservation-proposal.json` was judged APPROVE (b) by an
  independent `ze-test-judge` on this host and applied (fixture SHA256
  `cdb562cc...57fa`). Fresh model observations over the corrected inputs are
  owed; do not certify the fixture complete before they are recorded.
- P-25: the `RFC7296-2.1-1` provenance assertion in `TestOwnedDocumentDisposition`
  is committed and passes in `36d3cdf3f3`'s record.
- P-12 (a) LOCAL_PREF, **settled**: `d645d0357b` applied the repair Thomas
  authorized ("Authorize this exact repair and approval record"). The checker
  accepts the FIFO fence in either NLRI order inside the UPDATE that completes
  the three subjects (RFC 4271 Section 4.3: "An UPDATE message is used to
  advertise feasible routes that share common path attributes to a peer"), and
  refuses a fence in an earlier UPDATE than the third subject. This replaces
  the in-UPDATE NLRI-order rule of `7081cfdfa7`. The independent judge approved
  the change for `bgp.TestLocalPrefStripChecksDestinationWire` (the
  `Weakening-approved:` line in `d645d0357b`). Owed: the lab red walk on
  `bgp-local-pref-strip-gobgp` (break the rule in the checker or in Ze's
  `applyFactsLocalPref`, rebuild, red, restore, green) and the discrimination
  record.
- P-35: the Mac's bounded BFD batch is a second observation of a carrier this
  checkout already walked (evidence, "P-35: the GR purge, BFD join and
  document-too-wide members").

Mac logs, proposals and the P-24 payloads used above are in
`tmp/mac-port/logs/`; the rest of the Mac scratch
(`tmp/session/2026-11-10-002721/scratch/` on the Mac) is not on this host.

### Resume order

1. Done: the port is committed, in the commits named at the top of this
   section.
2. Prove the LOCAL_PREF repair in the lab (the `applyFactsLocalPref` strip
   removed), then commit the record it writes:

   ```
   ./le rfc record id RFC4271-5.1.5-2 polarity positive unit internal/le/interoplab/bgp/check_rfc.go::checkLocalPrefStrip route mutant report tmp/session/2026-11-11-004732/scratch/walks/lp-mutant.json mutant internal/component/bgp/reactor/forward_local_pref.go:120:2#1 citation 13
   ```

   On 2026-11-11 its clean run passed (4m11s); the broken run never started
   because another session's uncommitted edit left
   `internal/le/interoplab/topology` not compiling (`undefined: cpeIKEName`),
   and the overlay build compiles the working tree. The checker half is
   already walked at unit level (evidence, "SERVER unit red walks").
3. Done on 2026-11-11, each recorded in the evidence file: P-18 (three
   cleanups applied in `c408086726`, seven cuts red), P-13 HTMX repair
   (`7b5d645b94`) and broadcast walk, P-16, P-14, P-25, P-8 and P-28 walks,
   P-24 fresh observations (all seven matched), P-31, the readiness probe
   (`fe96a2c67d`, with the BMP collector and, in `ab4a67d631`, BIRD's IPv6
   sysctls, both text matches on scenario configs), and the CodeQL
   `CGO_ENABLED=0` rationale (`9be512c243`: not needed on the Select step,
   dropped and documented).
4. Complete every remaining P-1 through P-41 proof, the Review Gate and the
   two-commit closure. AC-32 and P-32 are restated by
   `spec-inventory-fast-registry-checks` AC-17 when that spec lands.

## Earlier handover record (superseded by the saved state above)

Spec: `plan/post-release/engineering/spec-tooling-one-off-defects.md` (claimed,
in progress). Goal set by Thomas: close the spec, every AC met by its proof
obligation P-1 to P-41 through the real entry with attributed red, repair and
restored green, every owned journal row closed or moved to "Rows found already
fixed", Review Gate passed, two-commit closure.

## Where it stands

`TestAnswerAnswersTheInventory` (the actual repository) still refuses. Each
commit below removed one actual-root stop; the reported stop varies run to run,
because only the first refusal of the final fixpoint round is named.

| Commit | Analyzer capability added |
|--------|---------------------------|
| 6928ef7c82 | a call result's fields are the callee's returned fields; external results flattened |
| f0fe8070e1 | generic type methods execute; instantiation census for unbound type parameters |
| 7ae5e8afc4 | `for _, x = range c` stores c's contents |
| 0822a16666 | tuple operands type their fields; named results return their variables |
| 7c77569769 | assembly leaf without NOSPLIT |
| 3c5348f09d | assembly leaf conditional arms read branch-neutrally |
| 562a0305cf | indexed elements carry the proved element summary; direct element-field stores (storeLocation) |
| 75ad75aeab | interface receiver dispatches to the methods its value carries |
| 925dac4ff0 | bare generic export keeps its declaration binding (five header fixtures now pass) |
| 2ada4d93a0 | range copies drop the collection's own location; the FlowSpec projection's store passes the metadata write rule (family fixture now passes) |

All six package fixture failures that predated this session are now fixed.
The full package suite was last run before 2ada4d93a0: rerun it first. The
family-fixture notes below are kept for the reasoning; both causes are fixed.

Package suite (`go test` of `internal/le/repo/inventory` with the full tag set,
skipping the five actual-root tests): one known failure left,
`TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection`
("external mutation can change registration metadata through familySet"). It
predates this session. Two causes, traced with probes:

1. Traced: `fam`, the range copy of `familySet`, carries the slice's own
   location (rangedValue joins the flattened collection). `fam.String()` binds
   it to the receiver of the exported `family.Family.String`, whose receiver
   also holds the ambient pool (origins), so `return f.name` answers the whole
   receiver and its return grows the pool with `familySet`. An UNCOMMITTED
   change in `externalrangeelements.go` (`withoutCollectionLocation`, called
   from rangedValue after a proved population) drops the collection
   variable's own location from the copy: a range copy never refers to the
   backing array, and a stored `&x[j]` arrives through the element values.
   With it the familySet refusal is gone. It still needs a discriminating
   test and a red walk before it is committed.
2. The next refusal is the metadata write rule in `externalEffects.solve`:
   any write to an object the registration metadata reads ("external
   registration metadata depends on a mutated retained backing location"),
   here `names[i] = fam.String()` inside the helper. That contradicts the
   make/range/index/return projection the test says is supported. Before
   relaxing it, check whether the static family evaluator
   (`familyexpressions.go`) refuses local writes it does not model; if it does
   not, the write rule is the only guard and must stay for those writes.

## Next

Items 1 and 2 are superseded by `spec-inventory-fast-registry-checks`, which
deleted the external-effect proof this chain repaired; do not run them.

1. Thomas must allow restarting the actual-root run: the harness stopped the
   last one for low memory, and its rule is to restart only when asked.
2. Rerun the package suite, then continue the actual-root chain. The
   last seen stops were protobuf's validator (`impl/validate.go`,
   `st.mi.Desc.FullName()`) and `filedesc/desc_list.go` (`desc.Index()`), both
   interface method calls, which 75ad75aeab addresses; the next stop is
   unknown.
3. Then the remaining proof obligations: P-35 missing; P-10, P-12 (plan in
   `tmp/session/2026-11-07-095909/scratch/p12-plan.txt`, 15 interop checkers,
   5 RTR fixtures), P-18, P-36, P-26, P-32, P-33(1), P-4, P-14 partial; the
   owned journal rows; Review Gate; two-commit closure.

## P-12 progress (later in the same session)

Recorded in `tooling-one-off-evidence.md`, sections P-12 (a) to (d).

- (d) build-only bench staging: proved (`e9c131dc27`).
- (c) IPsec log key: unit carrier proved; interop checker half owed.
- (b) RTR fixtures: `rpki-revalidate-late-sync` proved. The four relay
  fixtures need the loopback alias (`sudo ifconfig lo0 alias 127.0.0.2`, an
  owner action). The shared harness cut cannot be observed with an
  uncommitted edit, because `./le` runs a build of the committed HEAD.
- (a) interop checkers: 10 of 15 proved (self next hop, MED across AS, MED
  removal, RFC 2545, blackhole, ADD-PATH, NO_EXPORT, IS-IS hostname, NSSA,
  relay withdrawal). NSSA needed lab repairs `b31f79495e` and `9281c5f8a0`.
  Unproved: LOCAL_PREF strip and typed NLRI (session readiness unreliable
  without any break), End-of-RIB (`docker exec` into FRR times out every
  run), route-server AS path (three prepend breaks stay green; the `bgp-rs`
  producer is untraced), RFC 7606 mixed update (producer untraced).
- Lab method: `tmp/session/2026-11-07-095909/scratch/interopwalk.py` takes
  a JSON list of walks (single edit or `edits` list), applies the break, runs
  the scenario, restores and checks the bytes, then runs the restored green.
  Checker code runs inside `./le`, so a checker change must be committed
  before a lab run uses it.

## Last run (end of session)

- Package suite (`go test` of `internal/le/repo/inventory` with the full tag
  set, skipping the five actual-root tests): **green**, `ok` in 949s.
- Actual root (superseded by `spec-inventory-fast-registry-checks`: the proof
  that refused is deleted, and `TestAnswerAnswersTheInventory` passes on the
  declared walk) (`TestAnswerAnswersTheInventory`): refused at
  `vendor/google.golang.org/protobuf/compiler/protogen/protogen.go`
  (the 654th line), "external registration capability is unknown: returned
  root field ownership is unresolved". That is the next analyzer fix:
  probe which expression's root type fails `fieldReceiverType`
  (`externalrootfields.go`), as the earlier stops were traced.
- RTR fixtures (P-12 b): all five proved after the owner added the
  127.0.0.2 loopback alias (`947d1a699b`).
- P-12 (c): closed on its unit carrier by owner decision; the lab check is
  `spec-ipsec-lab-debug-log-check.md` (`b43f7a8889`).
- Interop checkers (P-12 a): 11 of 15 proved, the route-server AS path
  included (`4bb4b0c651`); #2 and #8 lab session flaky, #14 exec hang, #7
  checker cannot see the message shape (judged checker change owed).

## Earlier records

This is the only handover for this spec. Handovers 32, 33 and 34 were removed
(they stay in git history); the diagnostic and unfinished-source assets 33
carried moved to `plan/post-release/engineering/tooling-one-off-reboot-assets/`.
The P-12 proof queue is there at `diagnostics/phase9-current-proof-queue.md`.

## Method notes

- Red walks: `tmp/session/2026-11-07-095909/scratch/redwalk.py` takes a JSON
  list of `{name, file, old, new, pkg, test, tags}`; MUTATION notes in test
  comments are parsed into it.
- Every changed existing test went to an independent `ze-test-judge`; approvals
  are in `tmp/commit-weakening-approved-54456e39.md`.
- Never name a new test file without checking it is free: one overwrite of a
  tracked test was caught by the commit gate and restored from HEAD.
- The inventory selects amd64 assembly on the arm64 Mac.
