<!-- ste: ignore-file preserved verbatim from a design pass; it quotes RFC 7296 at length. -->

# WP-7 design -- traffic selectors, transport mode, port ranges

Rows: `RFC7296-2.9-2`, `2.9.2-1`, `2.9.2-2`, `2.23.1-1`, `2.23.1-2`, `2.23.1-3`,
`1.3.1-1`, `1.3.1-2`, `3.13.1-1`, `3.13.1-2`, `3.13.1-3` (11).
Plus the pre-existing `{gap}` on `RFC7296-2.9-1`, which the spec's Critical Review
Checklist makes this package clear (`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`, `:710`).

Source spec: `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`, phase list item 12.

**Read-only design. No tracked file was modified.** Every `file:line` below was read in
the working tree on 2026-07-31, and every claim about behaviour cites the function that
PRODUCES it, not a caller and not a comment (`ai/rules/evidence.md`). Other agents
are editing `internal/component/ike/engine/`, so line numbers move: **every citation names
its FUNCTION, and the implementer must re-locate by function name before quoting a line in
a tag.**

**Naming collision, same as WP-5's.** The 2026-07-30 re-triage renumbered the work
packages. "WP-7" now names "COOKIE, DH-group retry, KE payload agreement" (10 rows,
`:1571`), and this package's 11 rows sit in the new **"WP-12, Traffic-selector narrowing
and transport mode"** (`:1576`). The brief and the phase list use the OLD numbering. The
rows are what matter. Say which numbering a commit message uses.

---

## 0. Verdict

| Row | Appendix A class | This design's verdict | Needs production code? |
|-----|------------------|-----------------------|------------------------|
| `RFC7296-2.9-2` | **NOT IMPL** | **absent, and actively violated.** The responder answers with a full wildcard, which is WIDER than the initiator's proposal, not a subset of it | yes, large |
| `RFC7296-2.9.2-1` | **NOT IMPL** | **absent.** Rekey emits the same wildcard and compares nothing against the original SA | yes |
| `RFC7296-2.9.2-2` | **NOT IMPL** | **absent.** Same producer; no notion of "scope currently in use" exists | yes |
| `RFC7296-2.23.1-1` | **NOT IMPL** | **absent.** Ze has no transport mode at all | yes |
| `RFC7296-2.23.1-2` | **NOT IMPL** | **absent.** Same | yes |
| `RFC7296-2.23.1-3` | **NOT IMPL** | **absent.** Same | yes |
| `RFC7296-1.3.1-1` | **NOT IMPL** | **absent.** `NotifyUseTransportMode` is a declared constant with no other referent in the tree | yes |
| `RFC7296-1.3.1-2` | **NOT IMPL** | **absent.** Ze never requests transport mode, so it never has to react to a decline | yes |
| `RFC7296-3.13.1-1` | **NOT IMPL** | **conformant on the send side, by construction, but UNPROVEN and fragile.** `anyTrafficSelector` hardcodes `StartPort: 0` | no; test only, plus the port work below |
| `RFC7296-3.13.1-2` | **NOT IMPL** | **conformant on the send side, by construction, but UNPROVEN and fragile.** Same function hardcodes `EndPort: 65535` | no; test only, plus the port work below |
| `RFC7296-3.13.1-3` | **NOT IMPL** | **antecedent unreachable TODAY, and WP-7 makes it reachable.** Ze cannot express OPAQUE ports because it cannot express ports at all. The moment WP-7 adds port selectors, this becomes a live sender obligation | yes, as part of the port work |

**Nine of eleven rows need real production code, and the package is far larger than the
phase list's three files suggest.** The phase list names `engine/child.go`,
`engine/responder.go` and `wire/payload_ts.go`. That estimate predates the
mapping below. It misses the config surface, the config validator, the dataplane mode
plumbing, and the VPP backend.

**The headline finding is worse than the recorded gap.** See section 1.3.

---

## 1. `RFC7296-2.9-2` and the pre-existing `{gap}` on `RFC7296-2.9-1`

### 1.1 The obligations, verbatim

`RFC7296-2.9-2` is the third bullet of §2.9's narrowing procedure:

> "o  If the responder's policy allows it to accept the first selector
>       of TSi and TSr, then the responder MUST narrow the Traffic
>       Selectors to a subset that includes the initiator's first choices.
>       In this example above, the responder might respond with TSi being
>       (198.51.100.43 - 198.51.100.43) with all ports and IP protocols."

`rfc/full/rfc7296.txt:2434-2438`. The MUST is on `:2435`. Appendix A quotes the first
sentence exactly and drops the worked example, which is a legitimate elision.

The bullet the `{gap}` row rests on is the first of the same list:

> "o  If the responder's policy does not allow it to accept any part of
>       the proposed Traffic Selectors, it responds with a TS_UNACCEPTABLE
>       Notify message."

`rfc/full/rfc7296.txt:2426-2428`.

And the sentence that forbids widening:

> "When the responder chooses a subset of the traffic proposed by the
>    initiator, it narrows the Traffic Selectors to some subset of the
>    initiator's proposal (provided the set does not become the null set)."

`rfc/full/rfc7296.txt:2393-2395`.

**Note for the implementer, and an owner item.** Neither of the two sentences behind
`RFC7296-2.9-1` contains an RFC 2119 keyword. Both are indicative ("it responds with",
"it narrows"). The summary row is classed `[MUST]` and its text
("Responder may narrow traffic selectors but never widen; if narrowed result is empty,
respond with TS_UNACCEPTABLE") is a **paraphrase of two indicative bullets, not a
quotation**. `rfc/short/rfc7296.md`. That is a real defect in the row, separate from
the `{gap}`, and it is section 13's second owner item. The genuinely normative MUST in
§2.9 is `2.9-2`.

### 1.2 The pre-existing annotation, verbatim

`rfc/short/rfc7296.md`:

> `- [ ] [RFC7296-2.9-1] [MUST] Responder may narrow traffic selectors but never widen; if
> narrowed result is empty, respond with TS_UNACCEPTABLE (§2.9) {gap: the responder records
> the initiator's TSi/TSr and installs XFRM from them
> (internal/component/ike/engine/responder.go) but never narrows them to the
> configured policy and never emits TS_UNACCEPTABLE; narrowTS
> (internal/component/ike/engine/child.go) is unused in production and
> NotifyTSUnacceptable (internal/component/ike/wire/payload_notify.go) is never sent}`

**All three locators in the annotation are STALE**, exactly as WP-5 found for its own rows:

| Annotation says | Actually at | Function |
|-----------------|-------------|----------|
| `responder.go` | `responder.go` | `buildAuthResponse` |
| `child.go` | `child.go` | `narrowTS` |
| `payload_notify.go` | `payload_notify.go` | `NotifyTSUnacceptable` declaration |

The annotation's three factual claims are otherwise **true**, and I verified each against
the producing code:

- `narrowTS` is unused in production. `grep -rn 'narrowTS' internal/` returns exactly two
  files: the definition at `internal/component/ike/engine/child.go` and
  `child_test.go,383`. **It has no non-test caller.**
- `NotifyTSUnacceptable` is never sent. `grep -rn 'TSUnacceptable\|TS_UNACCEPTABLE'
  internal/` returns exactly one line, its own declaration at
  `internal/component/ike/wire/payload_notify.go`.
- The responder does record the initiator's TS and install XFRM from them:
  `buildAuthResponse` at `responder.go` sets `sa.NegotiatedTSi/TSr`, and
  `createFirstChildSA` at `child.go` copies them into `ChildSA.TSLocal/TSRemote`,
  which `installChildSA` puts into the policy at `child.go` and `:328-337`.

### 1.3 What the annotation MISSES, and it is the headline finding

The annotation says the responder "records the initiator's TSi/TSr and installs XFRM from
them ... but never narrows them". A reader takes that to mean the responder ECHOES the
initiator's selectors. **It does not. It answers with a full wildcard.**

The producing chain, read end to end:

| Step | Function | `file:line` | What it does |
|------|----------|-------------|--------------|
| 1 | `buildAuthResponse` | `internal/component/ike/engine/responder.go` | `espSPI, saPayload, respTSi, respTSr, err := buildChildSAResponsePayloads(sa)` |
| 2 | `buildChildSAResponsePayloads` | `internal/component/ike/engine/initiator.go` | `tsi, tsr := anyChildTSPayloads(sa)`. It never reads `sa.NegotiatedTSi/TSr` |
| 3 | `anyChildTSPayloads` | `internal/component/ike/engine/rekey.go` | returns one `anyTrafficSelector` for each of TSi and TSr |
| 4 | `anyTrafficSelector` | `internal/component/ike/engine/initiator.go` | `StartAddress: []byte{0,0,0,0}`, `EndAddress: []byte{255,255,255,255}`, `IPProtocol: 0`, `StartPort: 0`, `EndPort: 65535` |
| 5 | `buildAuthResponse` | `internal/component/ike/engine/responder.go` | appends `respTSi`, `respTSr` to the inner payload chain |

So the IKE_AUTH response carries TSi = TSr = `0.0.0.0-255.255.255.255`, all ports, all
protocols, regardless of what the initiator proposed.

**Three separate defects follow, and only the first is what the annotation describes.**

1. **No narrowing.** `2.9-2` is unimplemented.
2. **Active widening, which is the `2.9-1` violation proper.** The RFC permits a subset of
   the initiator's proposal. A wildcard is a strict SUPERSET of any non-wildcard proposal.
   The responder is not merely failing to narrow; it is answering with more traffic than
   was asked for.
3. **The wire and the dataplane disagree.** The response says ANY. The installed policy
   uses the initiator's proposed TS (`child.go`). Whenever the peer proposes
   anything narrower than `0.0.0.0/0`, Ze tells the peer one thing and programs another.
   The comment at `responder.go` asserts the opposite ("so the installed Child SA
   and the echoed TS agree"). **That comment is false in code**, and per
   `ai/rules/evidence.md` a comment is its author's belief, not a decision record.
   Fix the comment in the same commit (`ai/rules/stale-comments.md`).

The security consequence is the one the spec already identified as this package's whole
point: a peer selects its own traffic selectors and Ze installs XFRM policy for
them, unchecked against any operator policy. Section 1.4 shows why that is currently
unavoidable rather than merely unimplemented.

### 1.4 There is no operator policy to narrow AGAINST

`RFC7296-2.9-2`'s antecedent is "the responder's policy". **Ze has no configured traffic
selector policy.** I read the schema and the struct:

- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` has **no** traffic-selector leaf.
  Under `vpn/ipsec/site-to-site/peer` the leaves are `name` `:327`,
  `ike-group` `:332`, `esp-group` `:338`, `connection-type` `:344`,
  `local-address` `:353`, `remote-address` `:358`, the `authentication` container `:363`,
  and the `vti` container `:446`. `grep -i "subnet\|selector\|local-ts\|remote-ts"` over the
  file returns nothing.
- `SiteToSitePeer` (`internal/component/ike/ipsec/types.go`) carries
  `LocalAddress` and `RemoteAddress` and no selector field.

`local-address`/`remote-address` are **tunnel endpoints, not selectors**. They reach the
dataplane as `SAParams.Src/Dst` and `SPParams.TunnelSrc/TunnelDst`, and they only become a
selector via `ipToFullNet` (`child.go`) as a /32 fallback when nothing was
negotiated (`child.go`).

**Consequence for scope: WP-7 must ADD the traffic-selector configuration surface before
narrowing can mean anything.** This is not gold-plating and it is not scope creep; it is
the antecedent of the MUST. A narrowing implementation with nothing to narrow against
would be dead code, which is precisely the state `narrowTS` is in today.

### 1.5 `narrowTS` is the wrong shape and must be DELETED, not wired

`narrowTS` (`internal/component/ike/engine/child.go`) is:

```
func narrowTS(proposed, allowed *net.IPNet) *net.IPNet
```

It takes two `*net.IPNet`. A `*net.IPNet` is a CIDR prefix. An RFC 7296 traffic selector is
an arbitrary inclusive address RANGE plus a port RANGE plus a protocol
(`internal/component/ike/wire/payload_ts.go`: `TSType`, `IPProtocol`, `StartPort`,
`EndPort`, `StartAddress`, `EndAddress`). `narrowTS` can express none of the port,
protocol, or non-CIDR-aligned range semantics that §2.9 and §3.13.1 require.

It is also not an intersection despite its doc comment. Reading the body: it returns
`proposed` when `allowed` contains it and `proposed` is the more specific, returns
`allowed` in the mirror case, and otherwise returns nil. For two partially overlapping
ranges it returns nil rather than the overlap.

`ai/rules/no-layering.md`: when replacing X with Y, DELETE X first. **`narrowTS` and
`maskLen` are deleted in the same commit that adds the real narrowing engine.** Deleting
`narrowTS` also removes the misleading `RFC requirement: RFC4301-4.4.2-1 positive` tag at
`child_test.go`, whose text claims the narrowed result reaches a Child SA install.
That tag is on a dead function.

**Deleting a tagged test is exactly what `.claude/hooks/pretool-writeedit.py`'s
`rfc-tagged-test` check blocks, and `// test-relax:` does NOT satisfy it.** `TestNarrowTS`
(`child_test.go`) carries an `RFC requirement:` tag, so removing it needs
`// rfc-test-change-approved: <date> <what the user approved>` and **only Thomas may
authorise it**. This is section 13's third owner item. The RFC4301 obligation itself does
not disappear: it must be re-bound to the new narrowing engine's test in the same commit,
so coverage moves rather than drops (`check_evidence_ratchet`).

### 1.6 What clearing the `{gap}` requires

The spec says WP-7 clears it, and AC-18 makes any surviving
annotation a blocker. Clearing means **deleting the `{gap: ...}` text from
`rfc/short/rfc7296.md`**, which is only honest once all four of these hold:

| # | Requirement | Evidence |
|---|-------------|----------|
| 1 | A narrowing function has a NON-TEST caller on the responder path | `grep` shows a call from `buildAuthResponse` |
| 2 | The response carries the narrowed selectors, not `anyChildTSPayloads` | the `2.9-2` positive below |
| 3 | `NotifyTSUnacceptable` is SENT when the narrowed set is empty | the `2.9-1` positive below |
| 4 | `RFC7296-2.9-1` carries BOTH polarities as tagged tests | `make ze-rfc-check` exits 0 |

Removing the annotation without 1-3 is the "annotation smuggled in" failure the spec's
Critical Review Checklist calls the one thing the gate cannot catch.

**`2.9-1` and `2.9-2` are different obligations and their tags must say so**, or they fall
into the trap the spec recorded for `1.4.1-4`: two rows proving one
observable, neither establishing which case it is in.

- `2.9-1` is about the **empty** case: narrowing yields nothing, so send TS_UNACCEPTABLE.
- `2.9-2` is about the **non-empty** case: the policy accepts the first selector, so the
  answer MUST be a subset that INCLUDES that first choice.

The discriminator is the fixture: `2.9-1`'s drives a proposal disjoint from policy,
`2.9-2`'s drives a proposal whose first selector policy accepts.

---

## 2. `RFC7296-2.9.2-1` and `-2` -- rekeying must not narrow

### 2.1 The obligations, verbatim

> "Thus, the new
>    SA MUST NOT have narrower selectors than the original."

`rfc/full/rfc7296.txt:2539-2540`. Appendix A quotes it exactly.

> "The responder MUST NOT narrow down the Traffic Selectors
>    narrower than the scope currently in use."

`rfc/full/rfc7296.txt:2551-2552`. Appendix A quotes it exactly.

### 2.2 What Ze does today

Both are **absent**, and for the same reason as §1: the rekey responder emits the wildcard.

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The rekey REQUEST carries wildcard TS | `initiateChildRekey` | `internal/component/ike/engine/rekey.go` (calls `anyChildTSPayloads`) |
| The rekey RESPONSE carries wildcard TS | `respondChildRekey` | `internal/component/ike/engine/rekey.go` (same call) |
| The rekeyed child inherits the OLD selectors for the dataplane | `newRekeyedChild` | `internal/component/ike/engine/rekey.go` (`TSLocal: old.TSLocal`, `TSRemote: old.TSRemote`) |

**A wildcard is never narrower than anything, so today's wire behaviour does not VIOLATE
either MUST.** It satisfies them vacuously, by answering the widest possible set. That is
the "holds only by absence" shape, and it is the shape that EXPIRES the moment section 1's
narrowing lands: once the responder narrows, a rekey narrowing below the in-use scope
becomes reachable, and these two MUSTs become live constraints on the new code.

This ordering matters and the implementer must not miss it: **`2.9.2-1` and `2.9.2-2` are
constraints ON the narrowing engine, not separate features.** They are the reason the
narrowing function needs a "floor" parameter.

`newRekeyedChild` inheriting `old.TSLocal/TSRemote` is the right dataplane behaviour and
must be preserved: it is what makes the new SA carry the old scope.

---

## 3. `RFC7296-2.23.1-1`, `-2`, `-3` -- transport mode uses exactly one IP address

### 3.1 The obligations, verbatim

> "Because this is transport mode, it uses exactly same addresses as the
>    Traffic Selectors and outer IP address of the IKE packets.  For
>    transport mode, it MUST use exactly one IP address in the TSi and TSr
>    payloads.  It can have multiple Traffic Selectors if it has, for
>    example, multiple port ranges that it wants to negotiate, but all TSi
>    entries must use the IP1-IP1 range as the IP addresses, and all TSr
>    entries must have the IPN2-IPN2 range as IP addresses."

`rfc/full/rfc7296.txt:3712-3718`. The MUST is on `:3714`.

From "A summary of the rules for NAT traversal in transport mode ... For the client
proposing transport mode:" (`rfc/full/rfc7296.txt:3815-3817`):

> "   - The TSi entries MUST have exactly one IP address, and that MUST
>      match the source address of the IKE SA."

`rfc/full/rfc7296.txt:3819-3820`.

> "   - The TSr entries MUST have exactly one IP address, and that MUST
>      match the destination address of the IKE SA."

`rfc/full/rfc7296.txt:3822-3823`.

**All three are obligations on the side PROPOSING transport mode**, which §2.23.1 calls
"the client" and Ze calls the initiator. `2.23.1-1`'s enclosing paragraph is about the
client's triggering packet; `-2` and `-3` sit explicitly under "For the client proposing
transport mode". A responder-only implementation would leave all three with unreachable
antecedents. **Ze is both initiator and responder** (`sa.IsInitiator`, used at
`child.go`), so both halves are in scope, and the initiator half is where these three
MUSTs bind.

Note the important allowance in `:3715-3718`: multiple traffic selectors ARE permitted in
transport mode, as long as every one carries the same single address. The constraint is one
ADDRESS, not one SELECTOR. An implementation that rejected multiple selectors outright
would be wrong.

### 3.2 What Ze does today

**Absent. Ze has no transport mode on the IKE path at all.**

| Property | Producing function | `file:line` | Verdict |
|----------|--------------------|-------------|---------|
| The mode constant exists | declaration | `internal/component/ike/engine/child.go` (`modeTunnel = 2`) | engine-local duplicate of `dataplane.ModeTunnel` |
| Every policy and state install hardcodes tunnel | `installChildSA` | `internal/component/ike/engine/child.go`, `:286`, `:317`, `:333` (`Mode: modeTunnel`) | four sites, no other value reachable |
| No `modeTransport` constant exists in the engine | -- | `grep -rn 'modeTransport' internal/component/ike/engine/` returns nothing outside tests | -- |

The dataplane layer, by contrast, is **already complete for transport mode**, and this is
the single most favourable fact in the package:

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| `ModeTransport uint8 = 1` is Ze's vocabulary, 1-based so an unset field is invalid | declaration | `internal/component/ike/dataplane/dataplane.go` |
| Kernel translation handles transport and fails closed on anything else | `kernelXFRMMode` | `internal/component/ike/dataplane/dataplane.go` |
| A transport-mode policy with tunnel endpoints is REJECTED | `tunnelEndpoints` | `internal/component/ike/dataplane/dataplane.go` |
| The mode reaches the kernel state | `InstallSA` | `internal/component/ike/dataplane/xfrm_linux.go`, `Mode: netlink.Mode(mode)` |
| The mode reaches the kernel policy template | `xfrmPolicyFromParams` | `internal/component/ike/dataplane/xfrm_linux.go`, `:170` |

`kernelXFRMMode`'s doc comment (`dataplane.go`) records that a wrong mode number is
silent and that `ModeTransport` once reached the kernel as `XFRM_MODE_TUNNEL` with no
error. That history is why the guard exists, and why section 8's QEMU test is mandatory
rather than optional.

**The precise dataplane trap for the implementer.** `installChildSA` currently passes
`TunnelSrc: child.RemoteAddr, TunnelDst: child.LocalAddr` on both policies
(`child.go`, `:328-337`). `tunnelEndpoints` (`dataplane.go`) returns an
error for a non-tunnel mode that carries any endpoint:

> "transport mode must carry no tunnel endpoints, got src=%v dst=%v: RFC 4301 Section
> 4.4.1.2 leaves them unused"

So **flipping `Mode` to transport without also dropping `TunnelSrc`/`TunnelDst` fails the
install outright.** That is a fail-closed guard behaving exactly as designed, and it is the
first thing a naive implementation will hit. Section 6 encodes it.

---

## 4. `RFC7296-1.3.1-1` and `-2` -- USE_TRANSPORT_MODE

### 4.1 The obligations, verbatim

> "The USE_TRANSPORT_MODE notification MAY be included in a request
>    message that also includes an SA payload requesting a Child SA.  It
>    requests that the Child SA use transport mode rather than tunnel mode
>    for the SA created.  If the request is accepted, the response MUST
>    also include a notification of type USE_TRANSPORT_MODE.  If the
>    responder declines the request, the Child SA will be established in
>    tunnel mode.  If this is unacceptable to the initiator, the initiator
>    MUST delete the SA.  Note: Except when using this option to negotiate
>    transport mode, all Child SAs will use tunnel mode."

`rfc/full/rfc7296.txt:802-810`. `1.3.1-1`'s MUST is on `:805`; `1.3.1-2`'s MUST is on
`:808-809`. Appendix A quotes both accurately.

The final sentence is worth carrying into the tags: it is the RFC's own
statement that tunnel is the default, which is what makes today's hardcoded tunnel a
conformant DEFAULT rather than a violation of §1.3.1.

### 4.2 What Ze does today

**Absent, and cleanly so.**

`NotifyUseTransportMode uint16 = 16391` is declared at
`internal/component/ike/wire/payload_notify.go`. I grepped the whole IKE tree:

    grep -rn "NotifyUseTransportMode\|16391" internal/component/ike/
    internal/component/ike/wire/payload_notify.go:  NotifyUseTransportMode  uint16 = 16391

**One line. Its own declaration.** It is never constructed, never sent, never matched on
receive. The responder's notify switch (`handleAuthRequest`,
`internal/component/ike/engine/responder.go`) handles `NotifyInitialContact` and
`NotifySetWindowSize` only.

`1.3.1-1` is therefore vacuously satisfied today: Ze never accepts a transport-mode
request, because it never recognises one, so the "request is accepted" antecedent is
unreachable. `1.3.1-2` is vacuous for the mirror reason: Ze never proposes transport mode,
so it is never declined.

**Both vacuities EXPIRE the moment transport mode is implemented.** Neither may be argued
away as conformant-by-absence, because WP-7's own `2.23.1` rows require implementing the
feature that makes them live. This is the `RFC7296-3.4-1` expiring-negative shape the spec
records, and here it applies for real, unlike in WP-5.

`1.3.1-2` deserves a specific warning: it is an **initiator** obligation with a
DESTRUCTIVE consequent ("the initiator MUST delete the SA"). Getting it wrong in the
permissive direction silently downgrades an operator's transport-mode request to tunnel
mode, which is a security-relevant surprise; getting it wrong in the strict direction tears
down working tunnels. The config must therefore distinguish "transport preferred" from
"transport required", and only the latter deletes. See section 5.

---

## 5. `RFC7296-3.13.1-1`, `-2`, `-3` -- port field encoding

### 5.1 The obligations, verbatim

> "o  Start Port (2 octets, unsigned integer) - Value specifying the
>       smallest port number allowed by this Traffic Selector.  For
>       protocols for which port is undefined (including protocol 0), or
>       if all ports are allowed, this field MUST be zero."

`rfc/full/rfc7296.txt:6033-6036`. The MUST is on `:6036`.

> "o  End Port (2 octets, unsigned integer) - Value specifying the
>       largest port number allowed by this Traffic Selector.  For
>       protocols for which port is undefined (including protocol 0), or
>       if all ports are allowed, this field MUST be 65535."

`rfc/full/rfc7296.txt:6055-6058`. The MUST is on `:6058`.

> "Systems that are complying with [IPSECARCH] that wish to indicate
>    "ANY" ports MUST set the start port to 0 and the end port to 65535;
>    note that according to [IPSECARCH], "ANY" includes "OPAQUE".  Systems
>    working with [IPSECARCH] that wish to indicate "OPAQUE" ports, but
>    not "ANY" ports, MUST set the start port to 65535 and the end port
>    to 0."

`rfc/full/rfc7296.txt:6074-6079`. `3.13.1-3`'s MUST is on `:6078`.

**Appendix A's `3.13.1-3` text drops "working with [IPSECARCH]" and the quotation marks
around OPAQUE and ANY.** `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. Dropping the
qualifier WIDENS the row: as written it binds every system, where the RFC binds systems
working with RFC 4301. Restore the qualifier when the row lands. Note also that the
sentence immediately before it is a fourth MUST in the same paragraph, the
ANY encoding, which Appendix A captures as `3.13.1-1`/`-2` between them.

### 5.2 What Ze does today

**Send side: conformant by construction, but unproven, and by accident rather than by
decision.**

`anyTrafficSelector` (`internal/component/ike/engine/initiator.go`) is the only
producer of a `wire.TrafficSelector` on any send path. Both branches hardcode
`IPProtocol: 0`, `StartPort: 0`, `EndPort: 65535` (`:455-457` for IPv6, `:464-466` for
IPv4). Protocol 0 with ports 0..65535 is exactly what `3.13.1-1` and `3.13.1-2` require,
and exactly the ANY encoding of `:6074-6076`.

It is conformant because it is a constant, not because anything decided it. **The moment
WP-7 lets an operator configure ports, that constant is replaced by computed values and
these two rows stop being free.** Their tests must therefore be written against the NEW
producer, not against `anyTrafficSelector`.

**`3.13.1-3`: the antecedent is unreachable today, and WP-7 makes it reachable.**

The MUST binds a system that "wishes to indicate OPAQUE ports, but not ANY ports". Ze
cannot wish that: it has no port configuration at all (section 1.4), so it always indicates
ANY. The obligation has no reachable antecedent on the send side.

**That is NOT a reason to classify the row away.** It is a reason to implement OPAQUE
support as part of the port work, because WP-7's own port selectors create the antecedent.
Per `ai/rules/rfc-compliance.md`, full compliance plus full proof is on the table here, so
it is what WP-7 does.

**Receive side: a real defect, but not this row's.** `tsToIPNet`
(`internal/component/ike/engine/initiator.go`) reads `selectors[0]` only
and builds a `*net.IPNet` from `StartAddress`/`EndAddress`. `StartPort`, `EndPort` and
`IPProtocol` are never read. The spec records this as "`tsToIPNet` discards the OPAQUE port
data" (`:1624`).

Be precise about which obligation that breaks. `3.13.1-3` is a SENDER encoding rule;
discarding a received value does not violate it. What discarding DOES break is
exact-or-reject and §2.9: a peer that proposes OPAQUE-only ports (65535/0) has its
selector silently promoted to an ANY-ports policy, which is **widening**. That is the same
class of defect as section 1.3, and section 7 is where it is caught.

`tsToIPNet` also returns nil for any range that is not CIDR-aligned, and a nil
`NegotiatedTSi` falls back to `ipToFullNet` at `child.go`, silently substituting a
/32. A peer proposing `10.0.0.5-10.0.0.9` therefore gets a policy for the tunnel endpoint
alone, with no error. Section 7 covers this too.

---

## 6. Production code changes

Nine rows need code. The work is six changes, in dependency order. **The first two are
prerequisites that the spec's phase list does not mention at all.**

### 6.1 Config surface (new) -- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`

**Why this layer owns it.** §2.9's antecedent is "the responder's policy". Policy is
operator input, and operator input is YANG (`ai/rules/config.md`: the default
answer is YANG config, and a selector is something an operator documents in a change
ticket). Nothing else in the tree can supply it.

Under `vpn/ipsec/site-to-site/peer`, add:

| Element | Shape | Why |
|---------|-------|-----|
| `list tunnel { key "number"; ... }` | one entry per selector pair | RFC 7296 permits several selectors; a list is the only shape that carries them. `number` keyed as a string per `ai/rules/cli.md` "Identifiers Are Strings" |
| `container local { leaf prefix; leaf port; }` | `zt:ip-prefix` and a port | TSi when Ze initiates, TSr when it responds. Mirrors `child.go`'s role mapping |
| `container remote { leaf prefix; leaf port; }` | same | the other half |
| `leaf protocol` | `uint8`, default 0 | §3.13.1's IP protocol ID; 0 = all |
| `leaf mode { type enumeration { enum tunnel; enum transport; } default tunnel; }` | on the PEER, not the tunnel entry | §1.3.1 negotiates mode per Child SA. RFC 7296 `:809-810` makes tunnel the default, so the YANG default states the RFC's own default |
| `leaf transport-required { type boolean; default false; }` | on the peer | `1.3.1-2`'s "if this is unacceptable to the initiator". Distinguishes prefer from require. Positive assertion per `ai/rules/config.md` |

Use `zt:ip-prefix` rather than a raw string, per `ai/rules/config.md` Value Typing.
`mode` is a genuine two-value choice where neither value is "off", so an `enumeration` is
correct and the no-boolean-as-enum rule does not apply.

Mirror in `SiteToSitePeer` (`internal/component/ike/ipsec/types.go`): a
`TrafficSelectors []TrafficSelectorPolicy`, a `Mode`, and a `TransportRequired`.

### 6.2 Config validator (exact-or-reject) -- `internal/component/ike/ipsec/`, called from `internal/component/ike/engine/config.go`

**Why this layer owns it.** `validateIPsecSections`
(`internal/component/ike/engine/config.go`) IS the plugin's `OnConfigVerify` body,
registered at `internal/component/ike/engine/register.go`. It already chains
`ValidateGroupRefs`, `ValidatePKIRefs`, `ValidateIdentities` and `ValidateRemoteAccess`. A
new `cfg.ValidateTrafficSelectors()` joins that chain. This is the layer `ze config verify`
and `ze config commit` reach, which is exactly what `ai/rules/protocol.md` requires.

Section 7 gives the rejection list.

### 6.3 The narrowing engine (new) -- `internal/component/ike/engine/ts_narrow.go`

**Why a new file.** `child.go` is already the Child SA lifecycle concern; narrowing is a
distinct one (`ai/rules/go-standards.md`). It needs `// Design:` and `// Related:`
headers pointing at `child.go` and `responder.go` (`ai/rules/go-standards.md`,
`ai/rules/go-standards.md`).

Delete `narrowTS` and `maskLen` from `child.go` first (`ai/rules/no-layering.md`).
Keep `ipToFullNet` (`child.go`); it is still the no-TS fallback.

The function computes, over the full selector shape rather than a prefix:

| Input | Meaning |
|-------|---------|
| `proposed []wire.TrafficSelector` | the peer's TSi or TSr, order significant: `[0]` is the first choice |
| `policy []ipsec.TrafficSelectorPolicy` | the configured selectors for this peer |
| `floor *ChildSA` | the SA being rekeyed, or nil for a fresh Child SA |

It returns the narrowed `[]wire.TrafficSelector` and a "usable" flag. The algorithm follows
§2.9's four bullets in order (`rfc/full/rfc7296.txt:2426-2442`):

1. No part of the proposal is acceptable to policy -> empty. The caller sends
   TS_UNACCEPTABLE. (`2.9-1`)
2. Policy allows the entire proposed set -> return the proposal unchanged.
3. Policy allows `proposed[0]` -> the result MUST be a subset that INCLUDES `proposed[0]`.
   Put the intersection covering `proposed[0]` FIRST. (`2.9-2`)
4. Policy does not allow `proposed[0]` -> return an acceptable subset.

Three constraints layer on top, and each maps to a row:

- **Never widen.** Every returned selector is a subset of some proposed selector. This is
  `2.9-1`'s "never widen" half and it is what today's wildcard breaks.
- **Never below the floor.** When `floor != nil`, the result must not be narrower than the
  floor's selectors. If narrowing would go below it, return the floor's scope. (`2.9.2-1`,
  `2.9.2-2`)
- **Exactly programmable.** Every returned selector must be one the configured dataplane
  backend can program byte-for-byte. Section 7 explains why this belongs here and not only
  in the validator.

Also add a richer wire-to-policy conversion beside `tsToIPNet`, carrying ports and
protocol. `tsToIPNet` (`initiator.go`) stays only if some caller still needs the
prefix-only form; prefer deleting it once the callers move.

### 6.4 Responder wiring -- `internal/component/ike/engine/responder.go`, `initiator.go`

| Function | `file:line` | Change |
|----------|-------------|--------|
| `buildAuthResponse` | `responder.go` | replace the unconditional record with a narrowing call against `sa.PeerCfg`'s selectors. On empty, return a sentinel the caller turns into TS_UNACCEPTABLE. Fix the false comment at `:596-597` |
| `buildChildSAResponsePayloads` | `initiator.go` | take the narrowed selectors as a parameter; delete the `anyChildTSPayloads(sa)` call at `:447` |
| `buildAuthResponse` | `responder.go` | unchanged in shape; it appends whatever `respTSi`/`respTSr` now carry |
| `startResponderEAP` | `responder_eap.go` | same narrowing; the EAP path stashes TS separately and would otherwise keep the old behaviour. **This is the second-producer trap the spec has recorded three times** (`:1778-1783`) |
| `handleAuthRequest` | `responder.go` | add a `NotifyUseTransportMode` case to the notify switch |

The TS_UNACCEPTABLE response is a notify in the encrypted IKE_AUTH response. Follow the
existing error-notify shape in the same file rather than inventing one.

### 6.5 Rekey -- `internal/component/ike/engine/rekey.go`

| Function | `file:line` | Change |
|----------|-------------|--------|
| `respondChildRekey` | `rekey.go` | narrow with `floor` set to the SA being rekeyed (`2.9.2-2`) |
| `initiateChildRekey` | `rekey.go` | propose the old SA's selectors, not a wildcard (`2.9.2-1`) |
| `newRekeyedChild` | `rekey.go` | unchanged; inheriting `old.TSLocal/TSRemote` is correct |

### 6.6 Transport mode -- `internal/component/ike/engine/child.go`

| Site | `file:line` | Change |
|------|-------------|--------|
| mode constant | `child.go` | add `modeTransport = 1`. Better: delete the engine-local duplicates and use `dataplane.ModeTransport`/`ModeTunnel` (`dataplane.go`) directly, removing a two-vocabulary hazard the `kernelXFRMMode` comment already warns about |
| `createFirstChildSA` | `child.go` | carry the negotiated mode on `ChildSA` |
| `installChildSA` state installs | `child.go`, `:286` | `Mode:` becomes the negotiated mode |
| `installChildSA` policy installs | `child.go`, `:333` | `Mode:` becomes the negotiated mode, **AND `TunnelSrc`/`TunnelDst` must be omitted in transport mode** (`child.go`, `:328-337`), or `tunnelEndpoints` (`dataplane.go`) rejects the install |

The initiator half of `1.3.1-2` lives in `handleAuthResponse`
(`internal/component/ike/engine/fsm.go`, TS branch at `:611-617`): when Ze proposed
transport mode and the response carries no USE_TRANSPORT_MODE, the SA is tunnel mode; if
`transport-required` is set, delete it.

---

## 7. Exact-or-reject: where the check belongs, and the one that is easy to miss

`ai/rules/protocol.md` applies hard here. It has **two homes in this package**, and
the second is the one an implementer will skip.

### 7.1 Config time -- `ValidateTrafficSelectors`, reached by `ze config verify`

An operator selector Ze cannot program EXACTLY must fail at verify, never be silently
approximated. The backend is XFRM, and I read its limits rather than assuming them:

| Operator writes | Programmable? | Producing evidence |
|-----------------|---------------|--------------------|
| A CIDR prefix | yes | `XfrmPolicy.Src/Dst` are `*net.IPNet`; `selFromPolicy` writes `PrefixlenS/PrefixlenD` (`xfrm_policy_linux.go`) |
| A non-CIDR address range (`10.0.0.5-10.0.0.9`) | **NO** | the selector carries a prefix length, not a range. There is no field for it |
| All ports (0..65535) | yes | `Dport == 0` leaves `DportMask` 0, matching any port (`xfrm_policy_linux.go`) |
| A single port (`N..N`) | yes | `Dport = N` sets `DportMask = ^uint16(0)` (`xfrm_policy_linux.go`) |
| **Any other port range (`1024..2048`)** | **NO** | the kernel selector is port + MASK (`nl/xfrm_linux.go`: `Dport`, `DportMask`, `Sport`, `SportMask`), and `selFromPolicy` only ever writes mask 0 or mask 0xffff. An arbitrary inclusive range has no encoding |
| A protocol ID | yes | `SPParams.UpperProto` (`dataplane.go`) threads onto the selector |

So `ValidateTrafficSelectors` MUST reject, with the value and the valid alternatives in the
message (`ai/rules/cli.md`):

1. a port range that is neither `0..65535` nor `N..N`;
2. an address range that is not CIDR-expressible;
3. transport mode with a prefix that is not a single host (`/32` or `/128`) --
   this is `2.23.1-1` enforced at config time;
4. transport mode combined with a `vti bind` (`ze-ipsec-conf.yang`), since an XFRM
   interface implies tunnel encapsulation;
5. OPAQUE (`65535..0`) combined with any other port spec on the same selector, since
   `:6076` makes ANY include OPAQUE and the two together are contradictory.

**`SPParams` has no port fields today** (`dataplane.go`: `Src`, `Dst`, `Dir`,
`Proto`, `Mode`, `IfID`, `ReqID`, `UpperProto`, `IfIndex`, `TunnelSrc`, `TunnelDst`).
Adding port selectors therefore extends the dataplane API, and every backend must honour
the new fields or reject them.

### 7.2 Negotiation time -- inside the narrowing engine, and this is the one that is missed

Config validation only constrains what the OPERATOR writes. **The peer's proposal is
attacker-controlled and never passes through the validator.** A peer can propose
`1024..2048`, which Ze cannot program.

Today's code fails this silently in both directions: a non-CIDR range makes `tsToIPNet`
return nil (`initiator.go`) and `createFirstChildSA` substitutes a /32
(`child.go, 178-183`); a port range is discarded outright.

The narrowing engine must therefore treat "exactly programmable" as part of the narrowing
predicate, not as a later check:

- If the intersection of proposal and policy is exactly programmable, return it.
- If it is not, narrow FURTHER to the largest programmable subset (for a port range, a
  single port; for a non-CIDR address range, the largest contained prefix). Narrowing is
  always permitted by §2.9, so this stays conformant.
- If no non-empty programmable subset exists, send TS_UNACCEPTABLE.
- **Never round outward.** Answering `0..65535` because `1024..2048` is unprogrammable is
  widening, and it violates `2.9-1` and `rfc/full/rfc7296.txt:2393-2395`.

This is what makes the wire and the dataplane agree, which is the invariant section 1.3
shows is broken today. State it in the tags: **the selectors Ze puts on the wire are
exactly the selectors it programs.**

---

## 8. Dataplane, VPP, and QEMU

### 8.1 An XFRM policy in transport mode is a different object

Confirmed against the producer, not inferred:

| Difference | Producing function | `file:line` |
|------------|--------------------|-------------|
| The kernel mode number differs (0 vs 1) | `kernelXFRMMode` | `dataplane.go` |
| A transport-mode policy carries NO tunnel endpoints, and endpoints are rejected | `tunnelEndpoints` | `dataplane.go` |
| The mode is written into the policy TEMPLATE | `xfrmPolicyFromParams` | `xfrm_linux.go`, `:170` |
| The mode is written into the STATE | `InstallSA` | `xfrm_linux.go`, `:85` (`netlink.XfrmStateAdd`) |

The netlink backend needs **no change** for transport mode. The change is in the engine
(section 6.6): supply `ModeTransport` and stop supplying endpoints.

### 8.2 VPP needs the same change, and today it would silently do the wrong thing

`ai/rules/architecture.md`: "If the feature has a netlink implementation, add the VPP
implementation in the same work. Ze targets both dataplanes; netlink-only features create
drift."

`internal/component/ike/dataplane/vpp.go` (`//go:build ze_vpp`, backend registered at
`register_vpp.go`) contains **zero references to `Mode`**:

    grep -c "Mode" internal/component/ike/dataplane/vpp.go
    0

`vppBackend.InstallSA` unconditionally sets tunnel endpoints, and `vppBackend.InstallPolicy`
never reads `p.Mode`. **A transport-mode request to the VPP backend today installs a
tunnel-shaped entry and reports success.** That is precisely the silent-wrong-mode failure
`kernelXFRMMode`'s comment says the netlink guard was added to stop
(`dataplane.go`), reproduced in the other backend.

Two options, and the choice is the owner's:

| Option | Cost | Verdict |
|--------|------|---------|
| Implement transport mode in the VPP backend | GoVPP `ipsec_sad_entry_add_del_v3` carries a tunnel flag; the SPD entry needs the matching change | correct, and what the rule asks for |
| Reject transport mode in the VPP backend with a clear error | small, and fail-closed | **the minimum acceptable**, never a silent tunnel install |

Doing neither ships a backend that lies about the mode it programmed. `ai/rules/completion.md`
applies: this defect is now in scope because WP-7 is the entry point that reaches it.

Note `internal/plugins/ospf/ipsec_install.go` already passes
`Mode: dataplane.ModeTransport`, so the VPP gap is reachable TODAY through OSPF, not only
through WP-7. It is pre-existing, which says when it started, not whose it is.

### 8.3 QEMU is required

`ai/rules/platform-linux.md`: linux-only code must ship with integration tests that run in
the QEMU Alpine VM, and "needs hardware" is never an excuse.

The mode plumbing terminates in `//go:build linux` code (`xfrm_linux.go`), and the failure
mode being guarded is one the kernel accepts without error. A unit test with a fake backend
proves the engine's arithmetic; **only a real kernel proves the SA was installed in
transport mode.**

The good news is that the wiring already exists and needs no Makefile edit:

- `internal/component/ike/dataplane/xfrm_integration_linux_test.go` already carries
  `//go:build integration && linux` and gives the harness pattern (`TestXFRMListSAs`
  constructs `&xfrmBackend{}` directly and skips on `EPERM`/`EACCES`).
- `ZE_QEMU_INTEGRATION_PKGS` (`mk/test-integration.mk`) is **derived**: it greps for
  `^//go:build integration && linux` across `internal/` and `cmd/`. So a new test in that
  same package is picked up by `make ze-qemu-integration-test` automatically.

Add a transport-mode case there: install a transport-mode policy and state, read them back
with `netlink.XfrmStateList`, and assert the kernel reports `XFRM_MODE_TRANSPORT` (0) and
that the template carries no tunnel endpoints. Assert the tunnel-mode case in the same test
so the two cannot be confused.

**Caveat the implementer must know:** `ze-qemu-integration-test` is run by NOTHING
automated (`ai/rules/platform-linux.md`, "What actually RUNS these suites"). It must be run
by hand and its output pasted as evidence.

---

## 9. Tagged tests

### 9.1 The carrier constraint, which shapes everything below

**A tag in `test/ipsec-interop/` is REFUSED.** `ai/rules/testing.md` states it, and the
carrier table confirms it: `interop-ipsec` is declared with `TIER_UNRUN`
(`scripts/dev/rfc_requirements.py`, `CARRIERS`, the "other three interop trees have runners
but NO automated caller" entry). Only `test/interop/scenarios/*/check.py` earns
`interop/nightly`. There are zero `RFC requirement:` tags under `test/ipsec-interop/` today.

So the 22 tags must live in:

| Carrier | Tier | Where |
|---------|------|-------|
| `*_test.go` | `unit/verify` | `internal/component/ike/engine/`, `internal/component/ike/ipsec/` |
| `test/ipsec/*.ci` | `functional/verify` | the `ipsec` suite IS in `all_suites` and HAS a `run_suite` line (`mk/test-functional.mk`, `:217`), so it earns the verify tier |

The interop scenarios `15-ts-narrowing` and `16-transport-mode` are
**goal-validation evidence, not tagged evidence**. Write them; do not tag them.

**Prefer a `.ci` over a unit test wherever the behaviour is reachable from a config plus a
daemon** (owner decision D3, cited in `ai/rules/testing.md`). Narrowing is reachable that
way; the XFRM mode is not, on darwin.

### 9.2 The pairs

Home: `internal/component/ike/engine/rfc7296_ts_test.go` (new) for the negotiation rows,
`internal/component/ike/ipsec/` for the validator rows, and
`internal/component/ike/dataplane/xfrm_integration_linux_test.go` for the kernel proof.

| Row | Positive asserts | Negative asserts (the discriminator) |
|-----|------------------|--------------------------------------|
| `2.9-1` | a proposal DISJOINT from policy yields a TS_UNACCEPTABLE notify in the response, and no Child SA is installed | the same fixture with an OVERLAPPING proposal yields a Child SA and NO TS_UNACCEPTABLE. Without this, "always send TS_UNACCEPTABLE" passes |
| `2.9-2` | a proposal whose FIRST selector policy accepts yields a response whose selectors are a subset of the proposal AND contain `proposed[0]` | a proposal whose first selector policy REJECTS still yields a valid narrowed subset (bullet 4), so the test is not "echo the first selector" |
| `2.9.2-1` | a rekey whose proposal is narrower than the original yields selectors NOT narrower than the original | a rekey proposing the SAME scope is unchanged, so the floor is a floor and not a constant |
| `2.9.2-2` | the responder's rekey answer is never narrower than the scope in use, even when policy has since narrowed | a FRESH Child SA (floor nil) with the same narrowed policy IS narrowed. Proves the floor is rekey-specific, not a blanket refusal to narrow |
| `2.23.1-1` | a transport-mode Child SA has exactly one address in every TSi and TSr entry | a transport-mode proposal carrying MULTIPLE selectors with the SAME single address is ACCEPTED. Guards against rejecting multiple selectors |
| `2.23.1-2` | the TSi address equals the IKE SA source address | a TSi address differing from it is refused |
| `2.23.1-3` | the TSr address equals the IKE SA destination address | a TSr address differing from it is refused |
| `1.3.1-1` | a request carrying USE_TRANSPORT_MODE that Ze ACCEPTS yields a response carrying USE_TRANSPORT_MODE, and a transport-mode Child SA | a request carrying it that Ze DECLINES yields a response WITHOUT it and a tunnel-mode Child SA. Proves acceptance is a decision |
| `1.3.1-2` | with `transport-required`, a response lacking USE_TRANSPORT_MODE deletes the SA | without `transport-required`, the same response keeps a tunnel-mode SA. Proves the delete is conditional |
| `3.13.1-1` | over EVERY built TS payload, a selector with protocol 0 or all-ports carries `StartPort == 0` | the encoder CAN emit a non-zero start port, so zero is a decision. Assert a configured single-port selector emits `N` |
| `3.13.1-2` | same, `EndPort == 65535` | same, mirrored |
| `3.13.1-3` | a configured OPAQUE-only selector emits `StartPort == 65535, EndPort == 0` | an ANY selector emits `0/65535`, NOT the OPAQUE form. Proves the two encodings are distinguished |

**Anti-vacuity guard, mandatory** (the `2.5-12` lesson from `plan/handover/02-design-wp5.md`
section 2.5). The `3.13.1` positives sweep "every built TS payload". Assert the swept set is
NON-EMPTY and name its size in the failure message, or the whole assertion passes over
nothing.

**Reach BOTH responder producers.** `buildAuthResponse` (`responder.go`) and
`startResponderEAP` (`responder_eap.go`) both stash TS. A test covering only the
first leaves the EAP path unproven. This is the second-producer shape the spec has recorded
three times, and this package has it again.

### 9.3 Mutations

A test no mutation can redden is worthless. Run each of these and confirm the named test
flips.

| # | Mutation | Site | Must redden |
|---|----------|------|-------------|
| 1 | narrowing call replaced by `anyChildTSPayloads(sa)` (today's behaviour) | `buildAuthResponse`, `responder.go` | `2.9-2` positive AND `2.9-1` positive |
| 2 | narrowing returns the intersection but drops `proposed[0]` from the front | `ts_narrow.go` | `2.9-2` positive ONLY. This is the surgical one: it separates "narrowed" from "narrowed to include the first choice" |
| 3 | the empty case returns a wildcard instead of signalling TS_UNACCEPTABLE | `ts_narrow.go` | `2.9-1` positive |
| 4 | TS_UNACCEPTABLE sent unconditionally | `responder.go` | `2.9-1` NEGATIVE. Proves the negative gates something |
| 5 | the `floor` parameter is ignored | `ts_narrow.go` | `2.9.2-1` and `2.9.2-2` positives, and NOT the `2.9.2-2` negative |
| 6 | narrowing skipped on the EAP path only | `startResponderEAP`, `responder_eap.go` | the EAP-fixture half. **If nothing reddens, the suite does not reach the second producer** |
| 7 | `Mode: modeTunnel` restored at one of the four sites | `child.go`, `:286`, `:317`, `:333` | the `1.3.1-1` positive, and the QEMU transport case. Run all FOUR separately; a test reaching only the policy pair leaves the state pair unproven |
| 8 | `TunnelSrc`/`TunnelDst` still supplied in transport mode | `child.go`, `:333` | the transport install fails via `tunnelEndpoints` (`dataplane.go`). Confirms the guard is exercised rather than bypassed |
| 9 | the response notify is omitted when transport is accepted | `buildAuthResponse` | `1.3.1-1` positive |
| 10 | `transport-required` ignored | `handleAuthResponse`, `fsm.go` | `1.3.1-2` positive |
| 11 | `StartPort: 0` becomes `1` | `anyTrafficSelector`, `initiator.go` (and its replacement) | `3.13.1-1` positive |
| 12 | `EndPort: 65535` becomes `65534` | `initiator.go` | `3.13.1-2` positive |
| 13 | OPAQUE encoded as `0/65535` | the new port encoder | `3.13.1-3` positive |
| 14 | a `1024..2048` port range accepted and programmed as ANY | `ValidateTrafficSelectors` / `ts_narrow.go` | the exact-or-reject tests in section 7 |

Mutations 6, 7 and 8 are the ones that prove the harness reaches every producer. **Run them
explicitly rather than sampling.**

---

## 10. Id allocation

`check_id_allocation` (`scripts/dev/rfc_requirements.py`) refuses a new id whose
ordinal is at or below its section's high-water mark. The mark comes from the COMMITTED
HEAD (`git show HEAD:<path>`), and **a section with no mark is skipped entirely** (`:500-502`).

`_head_of` keys on the section STRING via `_ID_RE = ^(?P<head>.+)-(?P<ord>\d+)$`
(`:162`). The `.+` is greedy up to the last hyphen-digits, so `RFC7296-2.9.2-1` parses as
head `RFC7296-2.9.2`, ordinal 1 -- a **different scope** from `RFC7296-2.9`. Verified, and
it is what makes this package's allocation trivial.

### Marks measured 2026-07-31, from HEAD

    for s in '2\.9' '2\.9\.2' '2\.23\.1' '1\.3\.1' '3\.13\.1'; do
      git show HEAD:rfc/short/rfc7296.md | grep -o "RFC7296-${s}-[0-9]*" | sort -V | tail -1
    done

| Section | HEAD ids | Mark | Appendix A ordinal | Verdict |
|---------|----------|------|--------------------|---------|
| `RFC7296-2.9` | `-1` | **1** | `-2` | **lands as `-2`.** 2 > 1 |
| `RFC7296-2.9.2` | none | none | `-1`, `-2` | **land unchanged.** Section skipped |
| `RFC7296-2.23.1` | none | none | `-1`, `-2`, `-3` | **land unchanged** |
| `RFC7296-1.3.1` | none | none | `-1`, `-2` | **land unchanged** |
| `RFC7296-3.13.1` | none | none | `-1`, `-2`, `-3` | **land unchanged** |

**All 11 ids land at their Appendix A ordinals. No renumbering.** This is the opposite of
WP-5's situation, where three of four ids had to move.

`rfc/short/rfc7296.md` is modified in the working tree (`git status --porcelain` reports
` M`). I checked whether the pending diff moves any of these marks:

    git diff rfc/short/rfc7296.md | grep -o 'RFC7296-[0-9.]*-[0-9]*' | sort -V | uniq

It touches §2.5, §2.6, §2.10, §3.1, §3.2, §3.3, §3.11, §3.12, §3.14. **It touches none of
WP-7's five sections**, so the marks are the same read from HEAD or from the pending commit.

`plan/handover/02-design-wp3.md` claims no id in any of these sections either.

### Contiguity

`check_id_allocation` does not require contiguity, but leaving a hole strands ordinals
permanently (WP-5's section 5 argument). WP-7 is the **sole claimant of all five sections**
-- Appendix A holds exactly `2.9-1`/`-2`, `2.9.2-1`/`-2`, `2.23.1-1..-3`, `1.3.1-1`/`-2`,
`3.13.1-1..-3` and nothing else.

**So the contiguity risk is not between packages, it is within this one.** If WP-7 lands in
several commits, each SECTION must land whole. Landing `2.23.1-1` and `-3` while holding
`-2` back sets the mark to 3 and strands `-2` forever.

`RFC7296-2.9-1` keeps its id: it exists at HEAD, so WP-7 only deletes its `{gap}` annotation
and adds its two tags.

**Recompute at the moment of landing. Never hardcode.**

---

## 11. What this must NOT break

| Invariant | Why it is at risk | The guard |
|-----------|-------------------|-----------|
| **Every `test/ipsec-interop/` scenario stays green** | R-2 names WP-7's TS narrowing as one of four packages that change what a peer sees. Scenarios 01-11 all negotiate Child SAs, and today they get a wildcard. Once Ze narrows, a scenario whose `ze.conf` has no selector config must still produce a working SA | `make ze-ipsec-interop-test` at the package boundary, not at the end. A red is this package's own defect and is fixed here (`ai/rules/completion.md`) |
| **A peer with no configured selectors still works** | If "no `tunnel` list configured" narrows to the empty set, every existing config breaks with TS_UNACCEPTABLE | The absent-config default must be "policy allows everything", preserving today's behaviour exactly. Assert it with a fixture that configures no selectors |
| `RFC4301-4.4.2-1`'s existing coverage | `child_test.go` (`TestChildSAInboundPolicyUsesNegotiatedTS`) asserts the inbound policy carries the negotiated TS. Section 6 changes what "negotiated" means | The test sets `sa.NegotiatedTSi/TSr` directly, so it survives. Re-read it before editing; do not weaken it |
| `RFC4301-4.4.2-1`'s OTHER binding | `child_test.go` tags `TestNarrowTS`, which section 1.5 deletes | The obligation must be RE-BOUND to the new engine's test in the SAME commit, or `check_evidence_ratchet` fires. **Needs Thomas's `rfc-test-change-approved`** |
| `RFC4301-4.1-1/-3/-4` tunnel-mode assertions | `child_test.go` asserts every installed Child SA carries `Mode == modeTunnel`. Section 6.6 makes the mode variable | These tests must keep asserting tunnel for a TUNNEL-mode SA. If they sweep "every child", they need a mode-aware fixture. **`RFC4301-4.1-1`'s transport half is currently delegated to an OSPF test**; once IKE can do transport, re-point it |
| The dataplane fail-closed guards | `kernelXFRMMode` (`dataplane.go`) and `tunnelEndpoints` exist because a wrong mode is silent | WP-7 must not add a permissive default to either. Mutation 8 confirms the guard fires |
| `ipToFullNet` remains the no-TS fallback | `child.go` | Keep it when deleting `narrowTS`'s neighbours |
| Existing `test/ipsec/*.ci` | eight files, all exercising SA install and show output | They earn the verify tier, so a regression is caught by `make ze-verify` |

---

## 12. Risks

| Id | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-WP7-1 | **Narrowing breaks every existing interop scenario**, because none configures selectors and the default becomes deny | scenario 01 reds immediately | The absent-config default is "allow everything". Run `make ze-ipsec-interop-test` FIRST, before writing any narrowing logic, to capture the green baseline |
| R-WP7-2 | **The wire and the dataplane still disagree**, in a new way: the response carries a range Ze then programs approximately | none; both sides look plausible in isolation | Section 7.2's programmable-subset rule, plus a test asserting the emitted selectors EQUAL the installed policy selectors. Make that assertion explicit; it is the invariant the whole package exists to restore |
| R-WP7-3 | **The EAP responder path is left unnarrowed.** `startResponderEAP` (`responder_eap.go`) is a second producer | mutation 6 leaves the suite green | Fixtures for BOTH paths. This shape has now cost this spec three times |
| R-WP7-4 | **Transport mode installs fail with a confusing error** because `TunnelSrc`/`TunnelDst` were left in place | `tunnelEndpoints`' error text appears in logs | Section 6.6 names the exact sites (`child.go`, `:333`). Mutation 8 pins it |
| R-WP7-5 | **VPP silently installs tunnel mode when transport was negotiated.** `vpp.go` has zero `Mode` references | none. It reports success | Section 8.2: implement it, or reject transport in that backend. Never silent. Reachable today via OSPF, so it is not hypothetical |
| R-WP7-6 | **The config surface is judged out of scope**, narrowing is written against a hardcoded policy, and the feature is dead on arrival | a reviewer asks what `policy` is at the call site | Section 1.4: policy IS the antecedent of `2.9-2`. Without config there is nothing to narrow against, and `narrowTS`'s current state is the proof of what that looks like |
| R-WP7-7 | **Deleting `TestNarrowTS` is blocked by the `rfc-tagged-test` hook**, and the implementer works around it by leaving dead code | the hook rejects the edit | Section 13 item 3: get Thomas's `rfc-test-change-approved` BEFORE starting. `// test-relax:` does not satisfy this gate |
| R-WP7-8 | **`3.13.1-3` is classified `{gap}` or `{not-applicable}`** because OPAQUE looks exotic | a `{gap}` appears in the diff | AC-18 makes any annotation a blocker. Section 5.2: WP-7's own port work creates the antecedent, so the row is implemented, not classified. If that is contested, it is an owner question, not a self-service call (`ai/rules/rfc-compliance.md`) |
| R-WP7-9 | **`2.9-1` and `2.9-2` prove the same observable** and neither tag establishes which case it is in | a reviewer cannot tell the two tags apart | Section 1.6: `2.9-1` is the empty case, `2.9-2` the non-empty one. The fixtures differ, and both tags must say how. This is the `1.4.1-4` trap |
| R-WP7-10 | **`2.9.2-1`/`-2` are marked conformant** on today's wildcard, which satisfies them vacuously | a verdict of "conformant" with no floor parameter in the code | Section 2.2: the vacuity expires the moment narrowing lands, and narrowing lands in this same package. They are constraints ON the new engine |
| R-WP7-11 | **QEMU is skipped** because the unit tests pass with a fake backend | no signal; nothing automated runs `ze-qemu-integration-test` | Section 8.3. The failure being guarded is one the kernel accepts silently. Paste the QEMU output as evidence |
| R-WP7-12 | **An interop scenario is tagged**, and the scan refuses it | `make ze-rfc-check` errors naming the file | Section 9.1: interop-ipsec is `TIER_UNRUN`. Tags go in `_test.go` and `test/ipsec/*.ci` |
| R-WP7-13 | **Line numbers move under concurrent agents.** Several sessions are editing `internal/component/ike/engine/` | a tag cites a line holding different code | Every citation here names its function. Re-locate by function name and re-read before quoting a line in a tag |
| R-WP7-14 | **Port selectors need a dataplane API change** (`SPParams` has no port fields) that is discovered mid-implementation | the narrowing engine has nowhere to put the port | Section 7.1 states it up front. Extend `SPParams`, and make every backend honour or reject the new fields |

---

## 13. Items for the owner

Each quotes the requirement, cites the producing code, states the cost, and asks which way
-- never "may I skip it" (`ai/rules/rfc-compliance.md`).

**1. The package is roughly twice the phase list's scope, and the extra is a config surface.**
The phase list names three files. Sections 6.1 and 6.2 add a YANG surface, a
Go config type and a validator, because `RFC7296-2.9-2`'s antecedent is "the responder's
policy" and Ze has none (`ze-ipsec-conf.yang` has no selector leaf;
`SiteToSitePeer`, `types.go`, has no selector field). This is strictly MORE work,
so it needs no permission under `ai/rules/completion.md`. It is raised because it changes the
package's size and its user-visible config surface, which the spec's Integration Checklist
(`:571`, `:581`) will need to re-answer.

**2. `RFC7296-2.9-1`'s row text is a paraphrase, not a quotation.** The row reads
"Responder may narrow traffic selectors but never widen; if narrowed result is empty,
respond with TS_UNACCEPTABLE" and is classed `[MUST]` (`rfc/short/rfc7296.md`). The two
§2.9 sentences behind it (`rfc/full/rfc7296.txt:2393-2395` and `:2426-2428`) contain no RFC
2119 keyword; both are indicative. The obligation is real and derivable, and Ze should meet
it either way. The question is whether the row's TEXT should be corrected to quote the
source while keeping its id -- which `check_id_allocation` explicitly permits ("text edits
completely free", `scripts/dev/rfc_requirements.py`) and
`check_retired_requirements` requires be done under the same id. **Which way do you want it
recorded?**

**3. Deleting `TestNarrowTS` needs your authorisation, and only yours.**
`child_test.go` carries `RFC requirement: RFC4301-4.4.2-1 positive`
(`child_test.go`), whose text claims the narrowed result "a Child SA install then
writes into the inbound policy". That claim is false: `narrowTS`
(`internal/component/ike/engine/child.go`) has no non-test caller. Section 1.5 deletes
the function under `ai/rules/no-layering.md` and re-binds `RFC4301-4.4.2-1` to the new
narrowing engine's test in the same commit, so coverage moves rather than drops. The
`rfc-tagged-test` hook blocks this and `// test-relax:` does not satisfy it. **The ask is a
`// rfc-test-change-approved: <date> <what you approved>` for that deletion.** The
alternative is keeping a dead function with a false tag, which is worse.

**4. VPP transport mode: implement, or reject?** Section 8.2.
`internal/component/ike/dataplane/vpp.go` has zero `Mode` references, so a transport-mode
request installs a tunnel-shaped entry and reports success -- the exact silent-wrong-mode
failure the netlink guard was written to stop (`dataplane.go`). It is reachable today
through `internal/plugins/ospf/ipsec_install.go`, independently of WP-7.
`ai/rules/architecture.md` says add the VPP implementation in the same work. Implementing
it means the GoVPP SAD tunnel flag and the matching SPD entry. Rejecting it is a few lines
and fails closed. **Which way do you want it fixed?** Doing neither is not on the table.

---

## 14. Summary row texts

Land them in section order in the checklist of `rfc/short/rfc7296.md`. Each row is ONE
physical line in the file (`parse_checklist_line` reads one row per line); the wrapping
below is for this document only. The `(§X.Y)` citation is validated against the id's section
segment, so it is not decoration.

    - [ ] [RFC7296-1.3.1-1] [MUST] If the request is accepted, the response MUST also
      include a notification of type USE_TRANSPORT_MODE (§1.3.1)

    - [ ] [RFC7296-1.3.1-2] [MUST] If the responder declines the request, the Child SA will
      be established in tunnel mode. If this is unacceptable to the initiator, the initiator
      MUST delete the SA (§1.3.1)

    - [ ] [RFC7296-2.9-2] [MUST] If the responder's policy allows it to accept the first
      selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a
      subset that includes the initiator's first choices (§2.9)

    - [ ] [RFC7296-2.9.2-1] [MUST NOT] Thus, the new SA MUST NOT have narrower selectors
      than the original (§2.9.2)

    - [ ] [RFC7296-2.9.2-2] [MUST NOT] The responder MUST NOT narrow down the Traffic
      Selectors narrower than the scope currently in use (§2.9.2)

    - [ ] [RFC7296-2.23.1-1] [MUST] For transport mode, it MUST use exactly one IP address
      in the TSi and TSr payloads (§2.23.1)

    - [ ] [RFC7296-2.23.1-2] [MUST] The TSi entries MUST have exactly one IP address, and
      that MUST match the source address of the IKE SA (§2.23.1)

    - [ ] [RFC7296-2.23.1-3] [MUST] The TSr entries MUST have exactly one IP address, and
      that MUST match the destination address of the IKE SA (§2.23.1)

    - [ ] [RFC7296-3.13.1-1] [MUST] For protocols for which port is undefined (including
      protocol 0), or if all ports are allowed, the Start Port field MUST be zero (§3.13.1)

    - [ ] [RFC7296-3.13.1-2] [MUST] For protocols for which port is undefined (including
      protocol 0), or if all ports are allowed, the End Port field MUST be 65535 (§3.13.1)

    - [ ] [RFC7296-3.13.1-3] [MUST] Systems working with IPSECARCH that wish to indicate
      OPAQUE ports, but not ANY ports, MUST set the start port to 65535 and the end port
      to 0 (§3.13.1)

Two corrections against Appendix A are folded in above:

- `2.23.1-1`'s citation is `(§2.23.1)` alone. Appendix A writes `(§2.23.1, §2.23)`
  (`:1159`), but the sentence appears only in §2.23.1 (`rfc/full/rfc7296.txt:3714`), and
  `parse_checklist_line` validates the section segment against the id.
- `3.13.1-3` restores "working with IPSECARCH", which Appendix A drops. Without
  it the row binds every system; the RFC binds systems working with RFC 4301.

And `rfc/short/rfc7296.md` loses its `{gap: ...}` annotation, keeping id `RFC7296-2.9-1`
and gaining two tagged tests (section 1.6).

After the rows land, run `make ze-rfc-index` and commit `ai/RFC-REQUIREMENTS.md` in the SAME
commit. The ledger records each tagged test's `file:line`, and both verify modes of
`ze-rfc-check` fail on a stale ledger (`ai/rules/testing.md`, RFC-Tagged Tests).
