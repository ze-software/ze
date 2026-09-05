# Handover: fixes ready, blocked on two structural gates

Written 2026-08-29. Supersedes the status sections of
`2026-08-29-nine-defects-and-the-gate.md`; that file's defect descriptions still
stand except where corrected below.

## What the next session must know first

**Nothing is committed.** Two commits are prepared and refused by the commit
gate. Both refusals are pre-existing reds that no commit in this working tree
can clear, and `structural-red-ok` is owner-only.

**SUPERSEDED by the table below this one.** The staticcheck row was wrong: the
pin was already correct and the stale binary was local. See "The three reds, as
finally measured".

| Gate | Root cause, verified | Route out |
|------|----------------------|-----------|
| `staticcheck-feature-matrix/check` | staticcheck **2026.1** (`StaticcheckVersion`, `internal/le/setup/tools.go`) is the current release and cannot decode export data version 4. `go.mod` declares `go 1.27.0`. `Judge` (`internal/le/staticcheckfeaturematrix/judge.go`) runs `staticcheck` from PATH, so rebuilding it with go1.27 only swaps "could not import X" for "export data version 4 is greater than maximum supported version 2" | Owner decision on the canonical Go version. The 2026-08-22 row of `plan/journal/gate-verdict-depends-on-the-machine.md` measured both directions and reached the same conclusion |
| `doc-wiring` | 2940 documentation-drift issues, in `docs/features/pipe-operators.generated.md`, `../wiki/command-catalog.md` and `../gh-pages/reference/cli/index.html`. Generated CLI surfaces belonging to whichever session is changing the command set | That session lands, or the owner authorises a regeneration here |

Neither stage publishes a failure group, so `structuralGateReds`
(`internal/le/commit/verification.go`) charges both to every commit. That is the
documented unattributable-red defect, not a judgement about this work.

## The deferral review, and what it found

`plan/deferrals/` holds 110 shards. They are LEDGERS with a Status column, not
all open work: the live backlog was 134 `deferred` plus 5 `open` rows. Six
read-only agents classified them against the tree rather than against the rows,
which matters because many rows are months old and already discharged.

**Closed with evidence: 12 rows.** Eleven shards whose producers were verified
fixed or retired, plus the RFC-enrolment row, which claimed 172 ungated
summaries and 2136 owing MUSTs. Both numbers were a year stale: `rfc/enrolled.txt`
carries 171 stems, `rfc/not-enrolled.txt` carries 8, `rfc/short/` holds 179, so
the two files partition the corpus with nothing undeclared.

**Fixed this pass, both RFC-gate correctness:**

`RFC9552-8.2.6-1` MUST, "An implementation MUST have the means to limit inbound
updates", was satisfied on paper only. The per-family prefix maximum existed and
was enforced, but the count it compared came from a `[prefix-length][address]`
CIDR walk. For a Link-State NLRI that reads the high byte of the NLRI TYPE as a
prefix length, so no configured bgp-ls maximum could bind. `forEachPrefixEntry`
(`internal/component/bgp/reactor/session_prefix.go`) now dispatches through the
`nlrisplit` registry, the same per-family framing the RIB uses, which also
repairs the VPN and flowspec mis-measure the function's own comment conceded.

`splitOffBoilerplate` (`internal/le/rfc/inventory.go`) left a chunk WHOLE when
the key-words paragraph ran into an obligation with no terminator between them.
`sitesFor` drops a boilerplate-matching chunk entire, so the MUST inside it never
became a site and the RFC read as asking for nothing. It now cuts at the match
end when the tail still carries a MUST-level keyword, and only then, so a chunk
that is pure boilerplate is still dropped whole. The direction is what made this
worth taking: an over-count is visible to a reviewer, who deletes the row, and an
under-count is silent, because the gate cannot ask for evidence of an obligation
it never knew was owed.

**Also fixed: the firewall silently ignored two configured set flags.** An
operator writing `flags-constant` or `flags-dynamic` had it parsed by `parseSet`
(`internal/component/firewall/config.go`), rendered back by the web page, and
recognised by `readback_linux.go` on a set the kernel already held -- while
`lowerSet` (`internal/plugins/firewall/nft/lower_linux.go`) programmed neither.
The setting did nothing and said nothing. The vendored library exposes
`Set.Constant` and `Set.Dynamic` directly, so the fix is two fields beside the
`Interval` and `HasTimeout` already there, and the function's own comment already
gave the reason: a flag that does not lower is dropped on the way to the kernel.
`flags-constant` appeared in NO deferral row; it was found by reading the
lowering against the config parser.

The test compiles for Linux and cannot run on darwin (`exec format error`), so
its run is owed to QEMU under `ai/rules/platform-linux.md`.

**A designed behaviour I nearly broke, twice over.** `| display` dropping a field
name it cannot find is NOT a defect: `TestDisplayUnknownFieldIsInert`
(`internal/component/command/pipe_columns_test.go`) pins it as an acceptance
criterion, so that one wrong name cannot empty an answer or create a placeholder
column. The fix was written, the test refused it, and it was reverted whole.

That is the second time in this session a confident classification pointed at a
deliberate, tested decision -- the first was the plugin-family narrowing, refused
by `test/plugin/flowspec-open-capability.ci`. The general practice: before
changing a behaviour a report calls a defect, look for the test that PINS it.
A test asserting the current behaviour with a stated intent is a decision
already taken, and the report is arguing with the decision rather than with the
code.

**Four reported "defects" that are documented decisions, verified one by one.**
This is the most useful thing the pass produced, because each was reported
confidently and each would have been a regression to "fix":

| Reported as | What it actually is |
|-------------|---------------------|
| `\| display` silently drops an unmatched field name | An acceptance criterion. `TestDisplayUnknownFieldIsInert` pins the name as INERT so one wrong name cannot empty an answer or add a placeholder column |
| Narrowing plugin families to a peer's attached processes | Breaks `test/plugin/flowspec-open-capability.ci`, whose whole subject is that a plugin's families are advertised WITHOUT explicit family config |
| `routes_filtered` published as an unmeasured zero | Section 7.3 of `docs/architecture/api/birdwatcher-compat.md` states it outright: the count MUST be expected to be 0, ze retains no filtered routes, and the work is homed at `plan/immediate/spec-bgp-filtered-route-storage.md`. Section 7.2 is an owner decision of 2026-08-05 requiring all four counts be emitted |
| 32 enrolled RFCs missing from the public status page | Measured as 36, and the direction is the safe one: ze GATES more RFCs than it publicly CLAIMS. Under-disclosure, not a false claim. Authoring 36 accurate rows needs per-RFC knowledge, and rushing them would manufacture the false-claim defect this session already fixed once |
| `FamilyFragFlood` is a dead capability claim, named nowhere but its own declaration | A documented placeholder. `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md` states it: the enum "stays for a future sampling-based classifier". Deleting it would discard a recorded design intent |

FIVE of the reported defects, then, out of a review pass that was otherwise
accurate and found real work. The rate matters more than any single row: a
verdict reached from the code alone cannot see a decision recorded in a test's
intent, a contract section, or a design doc, and every one of these five reads as
a defect until that record is found. The reviewer is not careless; the evidence
they were given is genuinely incomplete.

The general practice, and it is the same one twice over: **before changing a
behaviour a report calls a defect, look for the test or the contract document
that PINS it.** A test asserting the current behaviour with a stated intent, or a
spec section stating the divergence and homing the work, is a decision already
taken. The report is then arguing with the decision, not with the code, and the
fix is a regression wearing a fix's clothes.

**Verified real, NOT fixed, each spec-sized rather than a patch:**

| Finding | Why it is not a patch |
|---------|----------------------|
| VRRP `RFC9568-6.4.3-7` MUST NOT, on an ENROLLED RFC. `EffectiveAcceptMode` (`internal/plugins/vrrp/groups.go`) reaches only the show snapshot; `fsm/events.go` says so outright, "stored for the state snapshot only". VIPs install unconditionally, so a non-owner with Accept_Mode false still accepts packets addressed to them | Conformance needs a per-VIP ingress filter with ICMPv6 NS/NA carve-outs per Section 6.1. Linux dataplane work, and `ai/rules/platform-linux.md` makes QEMU proof mandatory for it |
| The shipped `ze` binary accepts and silently discards unvalidated positional arguments, exit 0. `matchCommandTokens` returns the unmatched tail and reports success; `extractArgDefs` yields no ArgDefs for a node with no YANG leaf children, so the validation is skipped | The dangerous half is already mitigated: `firstFlagToken` rejects flag-shaped leftovers, and the code documents the residual precisely. Closing it means declaring the YANG leaves across the command surface and giving `ArgDef` a positional index |
| `internal/le/rfc/native_fixture_test.go` digest is red | Not mine and not re-sealable from here. Its own comment says the digest is computed over the COMMITTED file set, and another session has `carriers.go`, `check_compile.go` and `render.go` modified in the same package. Re-seal at commit time |
| adj-rib-in keeps ONE route per UPDATE for every complex family. `installComplexNLRIs` (`internal/component/bgp/plugins/adj_rib_in/rib.go`) loops the NLRIs and takes `if i == 0 { nlriHex = rawNLRIHex } else { continue }`, so an UPDATE carrying N VPN, EVPN or flowspec routes stores one entry, keyed on a prefix `wireNLRIsToAny` fabricated by walking non-CIDR framing as CIDR | The `continue` is a CONSEQUENCE, not an oversight, and this is the correction to make to the shard's own reading. `compactRouteKey` (`compact_key.go`) is typed on `netip.Prefix`, so a VPN or EVPN NLRI has no identity the key can hold: `netip.ParsePrefix` fails on the fabricated string, every complex NLRI collapses onto the same zero key, and storing them all would overwrite rather than accumulate. Splitting with `nlrisplit` is necessary and not sufficient. Widening the key is a store change and wants a spec |

## Resolving structural-red properly, not working around it

The override is a workaround. The root cause is one line, and half the fix is
done.

`structuralGateReds` (`internal/le/commit/verification.go`) charges a red to a
commit unless the failing stage published a group whose `kind` is `files`,
`lint` or `package` AND whose paths resolve. A group with no paths is
UNATTRIBUTABLE, and an unattributable red is charged to EVERY commit in the
checkout. That is why one session's red blocks every other session.

**Fixed: delegated sub-checks now name their files.** `runGoAction`
(`internal/le/doc/wiring/delegate.go`) is the single delegator for eleven native
sub-checks, and every one of them failed through `declareFailureGroup(action,
nil, ...)` -- discarding paths while the delegated report sat in `payload`
naming them. It now asks the report through an opt-in interface,
`failurePathLister`. A report that judges the tree as a whole keeps answering
nil, which is the honest answer for it. `docVerifyPage.FailingPaths`
(`docverify.go`) derives its paths from the prose it already prints, so a path
reaching the group is one the report showed the operator, and keeps only paths
that resolve.

A note on how that test was written, because the first version was WRONG. It
exercised `FailingPaths` and `delegatedFailurePaths` directly, and both still
passed with the wiring reverted: the change was untested by its own tests.
`TestRunGoActionPutsTheReportsPathsIntoTheGroup` drives the real failure path and
does redden when the wiring goes back to nil. A test that does not fail against
the old code is not evidence, however much of the new code it touches.

**Not fixed: the lint stage.** `verify lint/run` streams its children's output
rather than retaining it -- `Report.Text()` states that deliberately -- so it
holds no structured paths to hand over. Two routes, and the second is a design
decision rather than a patch:

- Capture each child's output in `PassResult`, parse the `file:line:col` lines,
  and emit a `lint` group. Contained, but it reverses a deliberate streaming
  design.
- Let the engine derive paths from a stage's detail log when the stage declares
  no group. `declaredGroups` (`internal/le/verify/engine/artifacts.go`) already
  reads that log. This is cheap and reaches every stage at once, and it changes
  what "unattributable" MEANS for all 43 of them. The engine currently prefers to
  admit ignorance rather than guess, and that preference is worth keeping or
  overturning deliberately, not by accident.

Doing the same for `verify lint/run` is what would finish it.

## The three reds, as finally measured

Five commit attempts, across two artifact refreshes, gave the same three. Each
was chased to its producer rather than assumed, and all three trace to ONE other
session's in-flight refactor of `internal/le/`, plus the one check the owner has
already exempted.

| Red | Verified cause | Can this session clear it? |
|-----|----------------|---------------------------|
| `verify lint/run` | `goimports` failures in `internal/le/verify/`, `internal/le/tier/`, `internal/le/plugin/boundary/`. My own 17 findings in this session's new test files ARE fixed; a full lint run reports zero issues in every file this session touched | No. Reformatting another session's files mid-refactor |
| `doc wiring` | Stale `<!-- source: -->` anchors naming symbols that are not declared where the doc says. Most point into `internal/le/rfc/actions.go`, `internal/le/verify/engine/`, `internal/le/testsensitivity/`. One names `RaisePrefixStale`, which exists nowhere in the tree and did not exist at HEAD either | No. The symbols are still moving; the anchors can only be corrected once they stop |
| `repository tracked-build/check` | The one check that cannot run before a commit exists. `ai/rules/git-safety.md` records the owner's 2026-08-04 ruling to KEEP the exemption, and the gate offers `broken-head-fix` for it | Not from here, by design |

Two things this cost, worth knowing before repeating them:

`./le` is an existence cache by design, so a Go change to the tooling is
invisible until `bin/le` is removed. That produced one wasted run reporting
"scenario ... has no Go checker".

The other session renamed CLI verbs mid-session: `verify-lint run` became
`verify lint run`, `doc-check verify` became `doc check verify`. Both silently
printed `le --help` and exited 0 instead of failing, so a run can LOOK like it
passed while executing nothing. Check the log's first line, not the exit code.

## Two things the owner owes before this can close

1. **A `test/rfc-changed.md` row** for the change to
   `TestRFC7606Section54PropagatesUnknownBGPLSType`
   (`internal/component/bgp/reactor/session_validation_nlritype_test.go`). Its
   fixture built a Node NLRI whose two-octet body cannot hold its own
   Protocol-ID and Identifier, so the RFC 9552 Section 8.2.2 syntactic walk drops
   it as malformed and the test measured the wrong rule. It now uses the
   well-formed `lsNodeNLRI` already in the package. That file's header says the
   row is the OWNER's decision, written down by the author who asked for it, so
   an agent must not write it.
   `./le rfc check` currently reports exactly one violation, and it is the audit
   verdict staleness on this same change. Reverting the fix would make the gate
   green by restoring a broken fixture, which is the coverage reduction
   `ai/rules/completion.md` bans.

2. **EVPN types 1 and 4 origination.** Not a defect fix: it needs an ESI config
   surface and CLI tokens. `buildEVPNFromParams`
   (`internal/component/bgp/plugins/nlri/evpn/plugin.go`) admits route types 2, 3
   and 5 only, and the CLI parser in
   `internal/component/bgp/plugins/cmd/update/update_text_evpn.go` accepts
   `mac-ip`, `ip-prefix` and `multicast`. First-release scope is the owner's call.

## One command clears two red unit tests

`./le test-unit bgp` has two failures and both are the loopback address
`./le setup check` already reports MISSING:

```
sudo ifconfig lo0 inet6 fd00::2 alias
```

`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds` and
`TestSendAnnounceAppendsLinkLocalWhenSection3Holds` assert RFC 2545 Section 3's
32-octet next hop, whose condition is that the speaker shares a subnet with the
peer. Without `fd00::2` on `lo0` the condition genuinely does not hold, ze
correctly emits the 16-octet form, and the tests correctly fail. Not a code
defect.

## Fixed and green in this session, uncommitted

| Defect | Where | Proof |
|--------|-------|-------|
| RFC 9552 Section 5.1 TLV ordering, both halves | `addressTLVs` and `srv6SIDsOrdered`, `internal/component/bgp/plugins/nlri/ls/types_descriptor.go` | `rfc9552_ordering_test.go`, 4 tests, each red with the fix backed out |
| RFC 9552 Section 5.2.1.1 node keys, (A) and (B) | same file: presence flags, and the ordering above | `types_descriptor_key_test.go` |
| RFC 4271 Section 5: unrecognized non-transitive optional attributes were relayed to third parties | `UnrecognizedNonTransitiveRanges` (`internal/component/bgp/message/rfc4271_nontransitive.go`), called from `publishBase` (`internal/component/bgp/reactor/session_validation.go`) | `rfc4271_test.go`; positive red with the strip disabled, negative red against an over-broad strip |
| Six families decoded then dropped at the RIB door | `internal/core/bgp/nlri/nlrisplit/`: `splitVPN`, `SplitFlowSpec`, `SplitVPLS`, `SplitBGPLS`, plus `splitCIDR` for RTC | `lost_families_test.go`, 6 tests red with the registrations backed out |
| VPLS misframed by the egress chunker, which read its 2-octet length as one | `vplsNLRISize` and `addPathVPLSNLRISize`, `internal/component/bgp/message/chunk_mp_nlri.go` | same class as the SAFI 72 omission already commented there |
| The descriptor walk discarded every TLV after the Local Node Descriptors container | `parseNodeDescriptorTLVsAt`, `internal/component/bgp/plugins/nlri/ls/types.go` | `rfc9514_srv6_descriptor_test.go`, 2 of 4 red with the fix backed out |
| A false public claim: `rfc-status.md` said EVPN types 1 and 4 are originated through the registered in-process route encoder | `RouteEncoderByFamily` has one non-test caller, `cmdEncode` (`internal/component/bgp/cli/encode.go`), the offline hex tool | the row and both annotations now say what a session reaches |
| `setup check` called staticcheck present without checking the toolchain that built it | `staticcheckBuiltNewEnough`, `internal/le/setup/probes.go` | `probes_toolchain_test.go` |

`./le rfc check` went from 6 violations to 1, and closed `RFC9552-5.1-1`,
`5.1-2`, `5.2.1.1-1`, `5.2.1.1-2`, `RFC7752-3.1-2`, `3.1-3` and `RFC4271-5-5`
as implemented-and-proven rather than as annotations. `RFC9552-8.2.3-5` and
`8.2.6-2` are annotated on the document's own text: the first is a Producer's
configuration and ze is never a Producer, the second binds the operator and
names its implementation half as a separate requirement.

## Two traps this session paid for

- **A raw `go test ./...` drops Ze's build tags.** It reported 125 failures
  across four packages. Through `./le test-unit bgp` the same tree has 2. Use the
  registered action; `ai/rules/commands.md` says so and the cost of ignoring it
  is a wholly invented failure set.
- **A stale analyser reports source defects that are not there.** 192 errors
  named unused imports and missing packages in files that were correct. The
  binary was the fault. Check what built a tool before believing what it says
  about code.

## Still unfixed from the nine

None. Defects 1, 3, 4, 5, 6, 8, 9 and 10 are closed, 2 is half fixed (the RFC
9072 send side below), and 7 is split: its false claim is fixed and its
capability is the owner's scope call.

## Defect 1 is NOT fixed, and the obvious fix is the owner's call

An earlier version of this handover claimed defect 1 was fixed without touching
AC-3. That was wrong, and the correction matters more than the claim did.

`getPluginFamilies` (`internal/component/bgp/reactor/peer.go`) answers with the
whole PROCESS's decode set, so a peer whose config names no family block takes
every loaded plugin's families into its OPEN, and the implicit ipv4/unicast
default cannot fire because that set is never empty. That is the defect.

Narrowing it to the processes ATTACHED to this peer passes every unit test,
including `TestBuildOpenPluginFamiliesUnchanged`, because that test drives the
getter through a STUB. It fails `test/plugin/flowspec-open-capability.ci`, which
drives the real one and whose whole subject is that loading a plugin auto-adds
its Multiprotocol capabilities "WITHOUT explicit family configuration". The peer
in that test attaches no process.

**The defect and the feature are the same behaviour seen from two sides.** Which
one wins is an operator-visible decision about a deliberate feature, not a defect
fix, so it needs the owner. The narrowing is written and tested and NOT wired:
`PluginRegistry.DecodeFamiliesForPlugins` with
`internal/component/plugin/decode_families_peer_test.go`. Wiring it is one line
in `getPluginFamilies`, and `flowspec-open-capability.ci` must change in the same
commit if it goes in.

The general practice this cost: a unit test that stubs the seam proves the
mechanism and says nothing about the production wiring. Run the FUNCTIONAL suite
before claiming a behaviour change is free.

## A real regression this session introduced, found and fixed

`test/encode/unknown-message.ci` expected the pre-fix NOTIFICATION for an unknown
message type: Message Header Error with subcode 0 and the ASCII sentence "can not
decode update message of type 255". RFC 4271 Section 6.1 requires "the Error
Subcode MUST be set to Bad Message Type. The Data field MUST contain the
erroneous Type field." The expectation was the violation with a green bar on top,
and it went red the moment the code became conformant. Corrected to
`01 03 FF`; the encode suite is 59/59.

## The plugin functional suite is not attributable right now

Three runs of `./le functional plugin` in one hour produced three DISJOINT
failure sets. `rfc9552-52-rs-opaque-withdraw-peer-down` fails with the BGP-LS
splitter registered AND with it backed out, so it is not caused by the six-family
change. Attribute nothing from a single run of this suite on a loaded machine;
`plan/journal/gate-verdict-depends-on-the-machine.md` already collects this.

## Defect 1, fixed without touching AC-3 (SUPERSEDED, see above)

`getPluginFamilies` (`internal/component/bgp/reactor/peer.go`) answered with
`GetDecodeFamilies()`, which is the WHOLE PROCESS's decode set. A peer whose
config named no family block therefore took every loaded plugin's families into
its OPEN, link-state among them; and because that set was never empty,
`capability.Negotiate`'s implicit ipv4/unicast default could not fire, so the
most ordinary configuration in BGP negotiated nothing an operator asked for.

The handover this supersedes framed the shape as a choice between dropping the
fallback (which breaks AC-3) and narrowing it to a native set (which still ships
link-state to everyone). There is a third shape and it costs neither: ask for the
decode families of the processes attached to THIS peer.
`PeerSettings.ProcessBindings` already carries them, and `r.api` is a concrete
`*pluginserver.Server` rather than an interface, so it is two small methods and
no mock. `PluginRegistry.DecodeFamiliesForPlugins` filters the family table by
plugin name.

AC-3's mechanism is untouched: a plugin-only peer still negotiates what its
plugin decodes, and `TestBuildOpenPluginFamiliesUnchanged` drives the getter
through a stub, so it neither changed nor needed to. A peer with no family block
and no attached process now gets an empty set, which is exactly what lets the
implicit default fire.

This also answers `RFC9552-8.2.6-2` in the direction the owner proposed: ze stops
offering BGP-LS to peers nobody configured for it, so an operator cannot fail to
have the import policy Section 8.2.6 asks for.

## Defect 9, fixed: AGGREGATOR on the RIB commit rail

`packAttributesWithASPath` (`internal/component/bgp/rib/commit.go`) sized and
wrote `otherAttrs` context-FREE while giving AS_PATH the destination's context.
A forwarded AGGREGATOR lives in `otherAttrs`, so a peer that negotiated no
four-octet AS support received the eight-octet form where RFC 4271 defines a
six-octet attribute, and the RFC 6793 Section 4.2.2 AS_TRANS downgrade never
ran.

Both loops now use `attrSizeWithContext` and `WriteAttrToWithContext`, which
already existed and already handled AGGREGATOR. `appendAS4AggregatorFor` adds the
companion Section 4.2.2 requires as the other half of the pair, and refuses to
add a second when the upstream already supplied one.

The wireu forward rail was already correct and already tested
(`internal/component/bgp/wireu/rfc6793_as4_test.go`). This was the SECOND
producer of the same attribute, with no coverage at all, which is why the defect
survived: `RFC6793-4.2.2-5` and `-6` read as proven while one of their two rails
was wrong.

**An interop run against a real two-byte-AS speaker is still owed** before this
ships. The unit tests pin the exact wire bytes, which is evidence about the
encoder, not about a peer.

### The interop scenario, built and two faults from passing

`test/interop/scenarios/bgp-aggregator-as4-downgrade-bird/` exists, and
`internal/le/interoplab/bgp/checkers.go` carries its entry. It does NOT pass yet,
and it must not be committed until it does.

The shape is right and two things in it are proven: the images build, and the
injector's own log reads `new session: sending 58 bytes to peer` then `sending 23
bytes to peer`, which is the hand-computed UPDATE carrying an AGGREGATOR of
4200000000 followed by End-of-RIB. So the hex arithmetic and the topology work.

The INJECTOR leg is proven. Its own log reads the OPEN exchange, then `sending 58
bytes to peer`, `sending 23 bytes to peer`, three KEEPALIVEs and `successful`. So
ze establishes, accepts the hand-built UPDATE carrying the four-octet AGGREGATOR,
and holds the session. The hex, the topology and the ze config for that half are
right.

**The BIRD SESSION establishes. The route is what never arrives.** The scenario's
assertion 1 is `opBIRDSession` and it passes; every failure has been at assertion
2, `opBIRDRoute`. BIRD's container log holds only `Started` whether the session is
up or not, so that log is evidence of nothing -- the passing control scenario
`bgp-ebgp-ipv4-bird` logs exactly the same single line.

So the remaining question is narrow: **why does ze not forward the injector's
route to the BIRD peer?**

Two leads were followed and both were WRONG. They are recorded so nobody spends
the time again:

| Wrong lead | Why it is wrong |
|------------|-----------------|
| The lab runs on 172.30.1.x while configs hardcode 172.30.0.x, so bird.conf dials a dead address | The copy walk in `prepare.go` rewrites the prefix in EVERY file of the scenario directory, not just ze.conf. Only the `zeCLIConfig` append is ze.conf-specific |
| BIRD is broken, or refuses the session | `INTEROP_SCENARIO=bgp-ebgp-ipv4-bird ./le integration interop` PASSES in this checkout. BIRD works |

What changed the symptom, and is worth keeping: adding
`attach process rib { receive [ update-received state ] }` to both peers moved
the `birdc` exec from a timeout to a clean exit, so the RIB engine is now being
fed. Ze still commits nothing to BIRD.

Where to look next: ze's EXPORT eligibility on the commit rail. The scenarios
that do forward a route to a third party either mark the source `rs-client true`
or carry an explicit filter, and this one has neither. Read what
`packAttributesWithASPath`'s caller requires of a peer before it commits, and
give the BIRD peer whatever that is.

The other candidate, if export is not it: `capability { asn4 false; }` is the
whole point of the scenario, so if THAT is what suppresses the forward, the
scenario needs a different old-speaker peer rather than a different setting. No
other scenario in the tree pairs it with a route-receiving destination.

**Do not chase this green.** The rail matters more than the colour: the scenario
exists to drive `packAttributesWithASPath`, and a version that passes because the
route reached BIRD by the route-server fast path or the adj-rib-in replay proves
nothing, since both were already correct. Back the `commit.go` change out and
confirm the scenario REDDENS before believing it.

Rebuild `bin/le` after touching `checkers.go`: `./le` is an existence cache by
design (see the comment in `le` itself), so a Go change to the lab is invisible
until the binary is removed. That cost one run here, reported as "scenario ... has
no Go checker".

**Defect 10 is CLOSED as not a defect.** It reproduced only under a raw
`go test`, where the absent `ze_bgp` tag leaves `plugin/all` registering no YANG
module. `TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin` then refuses every
document it reads and its counters invert. The test's own header states that
degradation in advance. It passes under `./le test-unit bgp`. The earlier
investigation eliminated six causes and could not have reached this one, because
the cause is not in the code.

## RFC 9072, half fixed

The SEND side is fixed and green. `writeToExtended` wrapped the RFC 9072
envelope around parameters that `buildOptionalParams` had packed with RFC 4271
one-octet lengths, and Section 2 requires two: "the Parameter Length field is
extended to be a two-octet unsigned integer." A conforming peer misframes from
the first parameter onward, and ze could not see it because ze read back what ze
wrote. `Open.ExtendedParams` now carries the framing choice and `WriteTo`, `Len`
and `UnpackOpen` all read it. `Len` mattered on its own: it inferred the envelope
from the total length, so extended-framed parameters under 256 octets would have
been written into a buffer four octets short.

The RECEIVE side is not fixed, and needs an owner row before it can be. Ze
selects the extended form on the Non-Ext OP LEN; RFC 9072 puts the test on the
Non-Ext OP TYPE. Section 3: "It is not considered a fatal error to receive an
OPEN message whose (non-extended) Optional Parameters Length value is not 255 and
whose first Optional Parameter type code is 255 -- in this case, the encoding of
this specification MUST be used for decoding the message."

A subtest of `TestOpenUnpackExtendedParams`
(`internal/component/bgp/message/open_test.go`) asserts the opposite and quotes a
sentence that **is not in RFC 9072** -- grep the full text for "treated as part of
the original" and it returns nothing. That subtest carries an `RFC9072-2-2` tag,
so correcting it needs the owner's `test/rfc-changed.md` row. The change closes
`RFC9072-2-5`, `2-6` and `3-1` together. The gap is documented at the branch in
`internal/component/bgp/message/open.go`.
