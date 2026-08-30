# Handover: nine protocol defects, the fixit backlog, and the commit block

Written 2026-08-29, session `fix-it`. Supersedes nothing; read alongside
`plan/handover/2026-08-28-fixit-and-the-lint-gate.md`, which covers the lint
burn-down and the fixit spec dispositions in detail. This one carries the work
that is still OPEN.

## The shape of what is left

Nine protocol defects found by comparing Ze against ExaBGP's main branch in both
directions. Every one is verified at the producing function. **Nothing is
committed.** The tree holds roughly 1200 changed files from this session and two
peer sessions.

Fix them in the order below. It is ordered by what an operator meets first, not
by effort.

## First, the state you are inheriting

Three other sessions share this checkout:

| Session | State |
|---------|-------|
| `le-reorg` | Landed 7 commits renaming 13 directories under `internal/le/`. Implementing a CLI namespace spec next: `./le verify lint run` becomes `./le verify lint run`, family by family. It will message when each lands |
| `green` | goconst in `internal/component/bgp/plugins/**` and `internal/plugins/ddos/**`, committing |
| a fourth | Leaves `.go` files with mismatched package names in its scratch dir, which poisons gopls. Ignore the resulting phantom "missing package" diagnostics |

**Every path under `internal/le/` moved.** `lintgate -> verifylint`,
`speclifecycle -> specsession`, `weakened -> testweakened`,
`verifyworktree -> verify`, `sitebuild -> site`, `devsetup -> setup`,
`aisync -> ai`, `lejob -> job`. Use the new paths in any commit script.

## Why nothing is committed

`./le commit create` refuses every commit in this checkout, for all three
sessions. Two conditions hold together: `verify-status` is not FRESH, and a
structural gate is recorded red in `tmp/ze-verify-failures.json`.

**The mechanism, and it is the highest-value `le` fix outstanding.**
`structuralGateReds` (`internal/le/commit/verification.go`) reads that JSON. A
stage that declares no failure group gets one synthesised by
`verifyengine/artifacts.go` as `{Kind: "generic", Related: []string{}}`.
`groupRelatedPaths` returns nil for any kind that is not `files`, `lint` or
`package`. A group naming no path is unattributable, and an unattributable red
is charged to **every** commit regardless of its file list.

Only two of 43 stages publish real groups: `docwiring/groups.go` and
`functional/run.go`. **So any one of the other 41 going red blocks the whole
repository.** Do not repeat this session's error: a gate printing file paths in
its OUTPUT TEXT is not publishing a failure group.

## Fixed in this session

Two of the nine are closed, each with a test whose discrimination was MEASURED
by backing the fix out and confirming the test reddens.

| Defect | Fix | Test |
|--------|-----|------|
| 6, BGP-LS key sub-TLVs elided on zero | `NodeDescriptor` carries `HasOSPFAreaID` and `HasBGPLSIdentifier` apart from the value, and `Bytes`, `Len` and `WriteTo` all honor them. The decoder sets them, so a received zero survives a re-encode. The other uint32 fields keep the value-only test, because AS 0 is reserved by RFC 7607 and a BGP Router-ID of 0.0.0.0 identifies nothing | `types_descriptor_key_test.go`, four cases including a backbone-versus-area-less discrimination |
| 8, wrong NOTIFICATION for an unknown message type | `handleUnknownType` sends subcode `NotifyHeaderBadType` with the offending Type octet as Data, per RFC 4271 Section 6.1. The constant already existed and was simply unused | `session_unknown_type_test.go`, reading the bytes off a `net.Pipe` rather than asserting on the returned error |

Defect 6 removes the `RFC9552-5.2.1.1-2` violation at its source. The row is not
yet CLOSED in `rfc/check`: that also needs the MT-ID and OSPF Route Type
emission below, and tagged `RFC requirement:` tests on both polarities.

Defect 1 was ATTEMPTED and reverted. The attempt is the useful part: adding
ipv4/unicast to the fallback reddens `TestBuildOpenPluginFamiliesUnchanged`,
which legitimately pins that a plugin declaring one family gets exactly that
family. So the defect is not in the fallback logic; it is that the in-tree NLRI
catalog is not an operator choice the way one purpose-built plugin is. Three
readings are recorded as a `KNOWN DEFECT` comment at the site.

## The nine defects

### 1. A peer with no family block cannot negotiate IPv4 unicast

`buildOpen`/`buildCapabilities` (`internal/component/bgp/reactor/session_negotiate.go`)
advertises **every plugin decode family** when the peer's config names no family
block. The live set is 17 families and `ipv4/unicast` is **not among them**;
BGP-LS is. `Negotiate` (`internal/core/bgp/capability/negotiated.go`) supplies
its implicit IPv4-unicast default only when `len(localFamilies) == 0`, and 17 is
not 0, so it never fires.

Against ExaBGP or FRR the most ordinary configuration in BGP negotiates nothing
usable. ExaBGP's equivalent advertises exactly what the operator configured.

This is also the answer to `RFC9552-8.2.6-2`: Ze currently ships BGP-LS to every
peer nobody asked. Thomas proposed deny-by-default and it is right — with the
fallback removed or narrowed, an operator cannot fail to have the import policy
§8.2.6 asks for, which is doing MORE than the RFC binds Ze to and needs no
ruling.

Decide the shape: drop the fallback so no family block means ipv4/unicast, as
ExaBGP does, or narrow it to a native set. The second still ships BGP-LS to
everyone unless BGP-LS is explicitly excluded.

### 2. RFC 9072 extended OPEN is broken in both directions

RFC 9072 §2 extends the Parameter Length field to two octets.

- **Send**: `buildOptionalParams` (`session_negotiate.go`) writes **one-octet**
  parameter lengths. `Open.WriteTo` (`internal/component/bgp/message/open.go`)
  then wraps them in the extended envelope when `optLen > 255`. The envelope
  declares two-octet lengths; the parameters inside carry one-octet ones.
- **Receive**: `UnpackOpen` strips the envelope but returns only the payload,
  with no signal that the extended form was used.
  `ParseFromOptionalParams` (`internal/core/bgp/capability/capability.go`) reads
  `optParams[offset+1]` as the length unconditionally.

**Nothing catches this because Ze writes and reads the same non-conformant form,
so Ze-to-Ze round-trips.** ExaBGP crosses 255 easily: it emits one type-2
parameter per capability VALUE, and Multiprotocol yields one per family.

The two decisions live in different packages with no shared state. That is how
they diverged and the fix must join them.

### 3. Six families decode and then vanish

`nlrisplit` (`internal/core/bgp/nlri/nlrisplit/register.go`) registers splitters
for unicast, multicast, EVPN, MVPN, MUP and labeled-unicast only. Every RIB
entry point gates on `nlrisplit.Supported(fam)` and returns early —
`insertPoolNLRIs`, `removePoolNLRIs` (`plugins/rib/rib.go`), plus
`rib_structured.go` and `rib_inject.go`.

So **mpls-vpn (128), flowspec (133), flow-vpn (134), VPLS (65), RTC (132) and
BGP-LS (71/72)** are accepted on the wire, decoded, and dropped with one `Debug`
line. `removePoolNLRIs` does not even log. ExaBGP originates the first four.

By `plan/future/README.md`'s own definition — "Ze loses, duplicates, or reorders
a route" — this is a release defect, and the worst kind: nothing is red.

Note this is upstream of all the BGP-LS work. Ze cannot store a BGP-LS route at
all, so origination and the Instance-ID sit behind a family with no RIB path.

### 4. Unrecognized optional NON-TRANSITIVE attributes are never stripped on egress

RFC 4271 §9: "If an optional non-transitive attribute is unrecognized, it is
quietly ignored." The ingress half exists —
`attribute.SetPartialOnUnrecognizedTransitive` (`internal/core/bgp/attribute/partial.go`),
called from `session_validation.go`. There is no egress counterpart:
`tryDirectPrepend` (`internal/component/bgp/wireu/aspath_rewrite.go`) copies the
attribute section verbatim, and nothing in `reactor/` or `wireu/` tests the
Transitive bit against `Recognized()`.

Ze leaks another speaker's non-transitive attributes to third parties.

**Write a functional test before acting.** The surveying agent confirmed the
verbatim-copy fast path and found no strip anywhere, but that is
absence-of-evidence for the remaining slow egress paths.

### 5. `Recognized()` answers a question it does not know

`attribute.RegisterName` (`internal/core/bgp/attribute/attribute.go`) marks a
code recognized. The mvpn plugin calls `RegisterName(22, "PMSI_TUNNEL")`
(`nlri/mvpn/register.go`) while `knownAttrParsers[22]` stays nil and no code
reads a PMSI byte. PMSI is optional **transitive**, so RFC 4271 §9 requires the
Partial bit to be set when forwarding an unrecognized one — Ze forwards it clean
because the name registry says it is recognized.

Same shape for codes 29 and 40. 29 is non-transitive so no obligation attaches;
40 is genuinely read at best-path time.

The fix is to make recognition mean "a parser exists", not "a name was
registered".

### 6. BGP-LS encoder loses the node key

Three parts, all in `internal/component/bgp/plugins/nlri/ls/types_descriptor.go`:

- `NodeDescriptor.Bytes`, `.Len` and `.WriteTo` all guard `nd.OSPFAreaID != 0`,
  `nd.BGPLSIdentifier != 0`, `nd.BGPRouterID != 0`, `nd.ConfedMember != 0`.
  **OSPF Area-ID 0.0.0.0 is the backbone area**, ASN 0 and BGP-LS Identifier 0
  are legal, and §5.2 RECOMMENDS 0 for TLV 513. A backbone node and an
  area-less node encode to the same key. **This is `RFC9552-5.2.1.1-2`
  verbatim.**
- `PrefixDescriptor` declares `MultiTopologyID` and `OSPFRouteType` and encodes
  neither — it emits TLV 265 only.
- `LinkDescriptor` declares `MultiTopologyID` and never writes TLV 263.
  §5.2.1.1 names MT-ID as part of the uniqueness key, so two nodes differing
  only by topology also collapse.

The three `NodeDescriptor` methods agree with each other, so there is no length
mismatch — the value is simply lost.

Fixing this is what closes `RFC9552-5.2.1.1-1` and `-2` by IMPLEMENTING rather
than annotating, which is the answer `ai/rules/rfc-compliance.md` prefers and
which needs no ruling.

### 7. EVPN types 1 and 4 cannot be originated in a live session

Two encoders disagree and the reachable one is narrower.
`buildEVPNFromParams` (`nlri/evpn/plugin.go`) gates on `{type2, type3, type5}`;
it is what `EncodeNLRIHex` calls, which the runtime `announce` and `update`
commands reach. `parseEVPNSection` (`plugins/cmd/update/update_text_evpn.go`)
refuses the tokens earlier still.

`l2vpnRouteToEVPNParams` (`nlri/evpn/encode.go`) handles all five and calls
`NewEVPNType1`/`NewEVPNType4`, but its only caller is `EncodeRoute` →
`internal/component/bgp/cli/encode.go`, the offline `ze bgp encode` hex tool.

RFC 7432 §7.1 and §7.4 make Ethernet A-D and Ethernet Segment the two routes
all-active multihoming needs. ExaBGP cannot originate any EVPN type, so this is
an RFC gap, not a parity gap.

### 8. Unknown message type sends the wrong NOTIFICATION subcode

`Session.handleUnknownType` (`internal/component/bgp/reactor/session_handlers.go`)
sends code 1 subcode 0 with a text string. RFC 4271 §6.1 requires subcode 3 (Bad
Message Type) with the erroneous Type field as Data.

Its comment cites ExaBGP as the authority. That is stale — ExaBGP fixed
`Message.unpack` to raise `Notify(1,3)` — and a comment is not an authority
anyway (`ai/rules/evidence.md`).

### 9. AGGREGATOR is written context-free on the commit path

**Corrected 2026-08-29 — this is worse than first reported, and the first
reading was wrong about which half is broken.**

RFC 6793 §4.2.2 states the obligation as a PAIR: "if the NEW BGP speaker has to
send the AGGREGATOR attribute, and if the aggregating Autonomous System's AS
number is a non-mappable four-octet AS number, then the speaker MUST use the
AS4_AGGREGATOR attribute and set the AS number field in the existing AGGREGATOR
attribute to the reserved AS number, AS_TRANS." It adds: "if the AS number is
mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent."

A pair cannot be satisfied by a method that writes one attribute, so
`(*Aggregator).WriteToWithContext` (`internal/core/bgp/attribute/simple.go`) can
never be the whole fix. It does the AS_TRANS half and cannot do the other.

But it is not even reached. `commitAttributes` (`internal/component/bgp/rib/commit.go`)
writes ORIGIN, AS_PATH, NEXT_HOP and LOCAL_PREF individually — AS_PATH through
`WriteAttrToWithContext` — and then loops `otherAttrs` through **`WriteAttrTo`**,
the context-FREE writer. A forwarded AGGREGATOR is in `otherAttrs`. So:

- the AS_TRANS downgrade never happens, so nothing is silently lost; and
- **no context handling happens either**, so a peer that negotiated no ASN4
  receives the 8-octet AGGREGATOR that `WriteTo` produces, where RFC 4271
  defines a 6-octet attribute for that peer.

Ze originates no AGGREGATOR of its own (`attribute.Aggregator{}` has no
constructor outside `_test.go`), so every AGGREGATOR on this path is FORWARDED.
The fix therefore belongs at the assembly site: `otherAttrs` needs
context-dependent writing, and the AS4_AGGREGATOR companion must be emitted in
the same pass when the downgrade happens.

Not attempted here. It changes bytes on the wire for every non-ASN4 peer, so it
needs a functional test against a real OLD speaker before it lands, and the
`attrSize`/`WriteAttrTo` invariant at the bottom of `commitAttributes` has to
move with it or the size check fires.

## Where Ze is RIGHT and ExaBGP is wrong — do not "fix" these

- ExaBGP raises `Notify(3,10)` for a BGP-LS Protocol-ID outside 1-6 and 227.
  RFC 9552 §8.2.2: "A Link-State NLRI MUST NOT be considered malformed or
  invalid based on the inclusion/exclusion of TLVs or contents of the TLV
  fields." §7.1.2 makes an unassigned value a future allocation. Ze's
  `bgplsNLRIWellFormed` treats those bytes as fixed-width and never reads them.
- ExaBGP raises `Notify(7,2)` for an unknown ROUTE-REFRESH subtype. RFC 7313 §5
  says MUST ignore. Ze logs and returns nil.

## The four RFC 9552 rows that block `rfc/check`

| Row | Binds | Disposition |
|-----|-------|-------------|
| `RFC9552-5.2.1.1-1` / `-2` | a Producer | **Fix defect 6.** They close by implementation, not annotation |
| `RFC9552-8.2.3-5` | the implementation | Thomas authorised implementing. A `uint64` YANG leaf, default 0 per §5.2, reachable from `getBGPLSYANG` (returns `""` today, `nlri/ls/plugin.go`), plumbed to the `id uint64` parameter of `NewBGPLSNode`/`Link`/`PrefixV4`/`PrefixV6`. §5.2 makes it **per routing-protocol instance**, not per neighbor. The wire path already carries the value end to end |
| `RFC9552-8.2.6-2` | the **operator** | Closes by construction if defect 1 is fixed as deny-by-default. Its sibling `-8.2.6-1` has a live `{gap}`: `countPrefixEntries` (`reactor/session_prefix.go`) is a CIDR walk that never parses a type-length Link-State NLRI, so the bgp-ls prefix maximum counts something unrelated. Fix them together |

## Two more BGP-LS decode defects, live today

- `parseNodeDescriptorTLVs` (`nlri/ls/types.go`) unwraps container TLV 256 with
  `data = value; continue`, replacing the iteration buffer and **discarding
  every byte after the container**. For NLRI Type 6 the SRv6 SID Descriptors
  follow at NLRI level, so `srv6-sid` is never emitted for any wire-decoded
  NLRI. Deterministic, reproducible, needs a row or a spec.
- `ls.ParseBGPLS` parses descriptors only for Node (1) and SRv6 SID (6). Link
  (2) and Prefix v4/v6 (3/4) get protocol-ID and identifier, then the whole body
  is stashed as `cached` bytes. ExaBGP decodes all of it.
- Stale `VIOLATION` comments on dead constant aliases at `nlri/ls/types.go`
  (`TLVLinkDescriptors`, `TLVPrefixDescriptors`).

## Verify state at handover

**The run was KILLED at 41/43 and never wrote its index.** `le-reorg` began its
namespace implementation on Thomas's instruction, which voids an in-place run
from its first edit. Killing was the right call rather than letting it finish:
the two remaining stages were `functional` and `functional/exabgp-test`, and a
spuriously red functional stage would have been recorded as a real red and
charged, unattributably, to every commit in the checkout — worse than no index.

**So `tmp/ze-verify-failures.json` still holds the 16:02 SIGINT run with
`verify-lint/run` red, and the commit block persists unchanged.** Nothing about
the block has improved; only our knowledge has.

The per-stage logs in `tmp/verify/full-2610784870/` ARE valid for stages 1-41,
because all of them completed before `le-reorg`'s first edit. Read them there.
**`verify-lint/run` is 0 issues across all 18 flavors** — that was the gate
blocking everyone, and that verdict stands on an untouched tree.

Re-run `./le verify current mode full` only AFTER `le-reorg`'s phases 1 and 2
are committed, not during them, or it will be lost the same way. It said it
would message when they land.

Twelve reds. Triage before acting; several are environmental:

| Stage | Verdict |
|-------|---------|
| `rfc/check` exit 2 | The four rows above. Real |
| `staticcheck-feature-matrix` exit 1 | **Toolchain, not code.** `export data version 4 > maximum supported 2`. `go.mod` pins `honnef.co/go/tools v0.8.1`, which cannot read Go 1.27 export data. Needs a dependency bump, which needs Thomas |
| `verify-deps/vulnerability` exit **127** | 127 is command-not-found. A missing tool, not a finding |
| `hook-check/unit` exit 2 | 30/31 native selftests pass. One fixture. Likely mine — I changed three hook guards |
| `doc-wiring`, `doc-check/verify`, `doc-check/templ-output`, `docs-to-code/index-check` | Documentation drift. `doc-wiring` delegates to `doc-check/verify`, so they are one cause |
| `test-health/check` exit 2 | Not investigated |
| `site-facts/check` | Clears when the tree is committed — it compares HEAD against derived values |
| `verify-deps/unit-cached`, `unit-race-changed` | Not investigated. Check against the HEAD baseline before assuming they are yours |

## A TENTH defect, found late and not diagnosed

`TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin`
(`internal/component/bgp/reactor/config_direction_test.go`) fails:

```
"15" is not greater than "93"
```

The assertion is `checked.tree > checked.text`, and its own message says what
that means: **"a surface that used to parse stopped parsing."** `walkTreeConfigs`
reads every `.ci`, `.conf`, `.md` and `.et` under `test/`, `docs/`, `demos/` and
`contrib/`, and counts attach blocks reached through a PARSED tree against those
found by a raw text scan. The parser is now reaching 15 where the text scan finds
93.

What is established:

- It is a REGRESSION, not a long-standing red. An agent measured this test GREEN
  in two runs earlier the same day.
- It is not from the BGP-LS descriptor or `handleUnknownType` changes; neither
  can affect config parsing.
- No COMMIT explains it. So it is in the UNCOMMITTED tree, which means the
  goconst sweep or a peer session.
- The sibling assertions still pass, including `parsed.documents != 0`, so the
  schema loads and documents do parse.

**Six candidate causes were checked and ELIMINATED.** Do not re-check these:

| Checked | Result |
|---------|--------|
| `internal/component/config/loader.go`, `parser*.go`, `tree*.go` | no diff at all — the parser itself is untouched |
| `internal/test/tmpfs` | no diff. The `.ci` embedded-block reader is unchanged |
| `internal/component/bgp/reactor/config_direction_test.go` | no diff. The test and its pinned plugin-name set are unchanged |
| every `*.yang` in the tree | none modified |
| the four pinned plugins' registrations (`plugins/{rr,rs,rpki,rpki_decorator}`) | no `Root`, `ConfigRoots`, `Name` or `WantsConfig` change |
| `yang_schema.go`'s two goconst substitutions | `configTrue = "true"` and `valueTypeString = "string"` — both hold the original values |

**Correct the framing before hunting.** The test's own message says "a surface
that used to parse stopped parsing", and this handover first repeated that as
though it had been confirmed. It has not been. `checked.tree` and `checked.text`
count BLOCKS, not documents, so one refused document holding many attach blocks
moves the ratio a long way while `parsed.documents > parsed.refused` still
passes. The right question is therefore not "what broke the parser" — nothing in
the parser changed — but "which FEW documents are now refused", and each of those
holds many blocks.

Fastest route: instrument `scanOneDocument` to print the path and the error when
`ParseTreeForValidation` returns one. The walk already has the path. That names
the documents in one run instead of bisecting 1200 changed files.

This is exactly the guard this repository builds on purpose: a counter that says
HOW the tree was read, so a silent loss of coverage shows up as a number rather
than as a green bar. It worked.

## Known reds that are NOT from this work

- `TestDefaultOriginateAppendsLinkLocalWhenSection3Holds` and
  `TestSendAnnounceAppendsLinkLocalWhenSection3Holds` fail **at HEAD**, verified
  in a clean detached worktree. Row in `plan/journal/unwired-feature.md`.
- `TestRFC7606Section54PropagatesUnknownBGPLSType` is red and **needs Thomas**.
  Its fixture `lsWireNLRI(1, 0x02, 0x00)` is a Node NLRI with a 2-octet body,
  impossible per RFC 9552 Figure 7, so the new validator correctly discards it.
  Fixture bytes only, but the file carries an `RFC requirement:` tag so
  `testweakened.Proposed` requires an owner approval row in
  `test/rfc-changed.md`. Row drafted in
  `plan/journal/fixture-encodes-an-impossible-state.md`.
- `internal/component/support` does not compile for FreeBSD (`disk_unix.go`
  multiplies `stat.Bavail` int64 by `uint64(stat.Bsize)`). Invisible to the gate
  because `plan` gives a flavor only the packages holding a file no earlier
  flavor claimed.
- `removePeerMetrics` (`reactor/reactor_peers.go`) open-codes `"route_refresh"`
  while the counters stamp `type="refresh"`, so a removed peer leaks two
  ROUTE-REFRESH counters forever.

## The fixit specs

All seven reviewed and classified last session; detail in the 2026-08-28
handover. Outstanding work:

- `peer-pending-sync-settles-too-early` — **the one unfixed RFC MUST**, an
  `RFC4724-4-1` violation. Read its re-cut Task, not the 2026-08-08 one. The
  End-of-RIB goes out 2s late and an announce raised in that window drains
  before it. Do NOT widen the hold condition — measured, it delivers a relayed
  route twice. Two owner decisions open.
- `vpp-slaac-no-dataplane-path` and `lint-blind-to-every-other-build-tag` — both
  owe **only closure**.
- `linger-rejection-reaches-no-verdict` — AC-4/AC-5 need one suite run over the
  63 fixtures pairing `option=linger` with `reject=`.
- `functional-suite-pins-the-unshipped-backend` — copied to `plan/future/`; the
  source under `plan/` still exists and its removal must ride the commit.

## Traps that cost this session hours

- **Never edit while a gate runs.** The post-edit hook lints each touched file
  and takes the same global golangci-lint flock; an in-place
  `./le verify current` is void the moment the tree moves under it.
- **Never trust a count from a capped run.** Three layers:
  `max-issues-per-linter: 50`, `max-same-issues: 10`, and `uniq-by-line` default
  true. Always `--max-same-issues=0 --max-issues-per-linter=0
  --uniq-by-line=false`, and work to a fixed point per package.
- **Always check the exit code**, not just the output.
- **`Mode: "decode"` on a `FamilyDecl` gates nothing in-process.** It is read
  only by `registrationFromRPC`, the forked-plugin path. The real discriminator
  is which `InProcess*` hooks each `register.go` sets. Do not cite it as
  evidence a family is receive-only — this session did, wrongly.
- **`unused` and `SA4023` in a package with `_linux.go` or `_other.go` siblings
  are darwin false positives.** Check `GOOS=linux go vet ./<pkg>/`.
- **Two AST-parity tests** recognise only `*ast.BasicLit` case values, so
  hoisting case literals to constants makes them see zero cases and pass
  vacuously. `internal/component/config/schema/cli` and `.../yang/cli` still
  have the old shape.
- **`embedlit` elision does not compile** for a struct field; use Go 1.27
  promoted-field initialisation.
- **A `//nolint` inside a `//` comment does not apply.** Reword.
- **Bare `go test ./internal/component/bgp/...` fails ~124 tests at HEAD**
  because the plugins compile out. Use the `.golangci.yml` tag list.

## Do this next, in order

1. **Wait for the verify to land**, then read `tmp/ze-verify-failures.json`.
   Do not edit while it runs.
2. **Triage the twelve reds** against a clean HEAD worktree before assuming any
   are yours. Three are environmental.
3. **Fix defects 1 and 2.** They are the two an ExaBGP or FRR peer meets on the
   first session, and neither is visible from inside this repository — both
   round-trip cleanly Ze-to-Ze. Each needs an interop test against a real peer,
   not a Ze-to-Ze one (`ai/rules/interop-and-goal-validation.md`).
4. **Fix defect 3.** Six families losing routes silently.
5. **Fix defects 6, then 5 and 4.** 6 closes two RFC rows by implementation.
6. **Then 7, 8, 9**, and the two BGP-LS decode defects.
7. **Land the commits.** `le-reorg` is blocked on thirteen files that are this
   session's work, all under `internal/le/`, at the NEW paths. Include
   `remove plan/spec-fixit-functional-suite-pins-the-unshipped-backend.md`.
8. **Close `vpp-slaac` and `lint-blind`**, which is what reduces the open count.

Commit granularity: these are separate focuses and must not become one commit.
`ai/rules/git-safety.md` requires single-focus commits, and a commit's file set
must be closed under "declares what the included files reference" — judged by
compiling the COMMIT, not the working tree. That rule broke HEAD three times in
the last day when it was not followed.
