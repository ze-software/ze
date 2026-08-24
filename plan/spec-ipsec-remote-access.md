# Spec: ipsec-remote-access

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - (was `spec-fixit-ipsec-verify-siblings`, closed 2026-08-03) |
| Phase | - |
| Updated | 2026-07-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7296.md` Section 2.19 + 3.15 (Configuration payload), `rfc/short/rfc5216.md`
4. `internal/component/ike/engine/register.go` (dispatch + admission), `reconcile.go` (PeerSession),
   `responder_eap.go`, `internal/component/ike/eap/{eap.go,pool.go}`,
   `internal/component/ike/wire/payload_cp.go`

## Task

`vpn ipsec remote-access` is a complete, documented, YANG-validated config surface that the
daemon **silently ignores**. Traced 2026-07-23 (recorded in
the `fixit-ipsec-verify-siblings` record and in the correction appended to the
`fixit-codeql-security-triage` record, both retired with the learned corpus):

| Field | Consumer today | Effect |
|-------|----------------|--------|
| `ra.Pool.*` | `engine/register.go` builds `eap.NewPool(...)` into `ipPool` | **discarded**: ~~`register.go`~~ **`register.go`** is `_ = ipPool`. Line re-verified 2026-07-31, because the file moved under concurrent edits |
| `ra.Auth.*` | none | none |
| `ra.Users` (`eap-user`) | none | none |

The IKE responder admits only sources that `matchResponderPeer` (`engine/register.go`)
resolves to a configured **site-to-site** peer with a literal `remote-address` match; everything
else is logged "unsolicited IKE_SA_INIT from unconfigured source" and dropped
(`register.go`). `PeerSession.peerCfg` is populated exclusively from `cfg.Peers`
(`reconcile.go`). A road-warrior client, whose address is by definition not preconfigured,
can never establish.

Owner decision 2026-07-23: implement the feature rather than reject or warn about the
inert config.

**What already works and is reused, not rebuilt.** EAP-MSCHAPv2 and EAP-TLS authentication
against a *responder* are implemented and interop-proven today
(`test/interop-ipsec/scenarios/responder-eap-mschapv2`, `eap-tls`), expressed as a
site-to-site peer with `connection-type respond` and a fixed `remote-address`. The Configuration
payload codec (`wire/payload_cp.go`) and the virtual IP pool (`eap/pool.go`, with
`Allocate`/`Release`/`Available`) ~~are both complete and~~ both have zero callers.

> **Superseded 2026-07-31 (WP-9 design pass).** "Both complete" is wrong, and the
> correction is load-bearing. The codec folds the RESERVED bit into the attribute type
> (`wire/payload_cp.go` reads 16 bits where RFC 7296 Section 3.15.1 defines 1 reserved
> bit plus a 15-bit type), it omits four attribute constants, and the pool hands out
> addresses outside the configured range for an IPv6 prefix longer than `/64`
> (`eap/pool.go`). Both halves need repair before they can be joined. Detail is in
> "RFC 7296 Rows Homed Here" below.

So the missing pieces are admission, per-user credentials, and address assignment:

1. **Admission**: accept an IKE_SA_INIT from an unconfigured source when `remote-access` is
   configured, and give each client its own session.
2. **Per-user credentials**: resolve the EAP identity to an `eap-user` entry, instead of the
   single `pre-shared-secret` a site-to-site peer carries. Unknown user must fail closed.
3. **Address assignment**: honour the client's CFG_REQUEST, allocate from the pool, answer
   CFG_REPLY (RFC 7296 Section 2.19), narrow the responder traffic selector to the assigned
   address, and release the address when the SA goes away.
4. **Config correctness** (inherited from `plan/deferrals/fixit-ipsec-verify-siblings.md`):
   resolve `ra.Auth` certificate references against the candidate PKI and require a
   `ca-certificate` for `mode eap-tls` (RFC 5216 Section 5.3), now that the config is live.

## RFC 7296 Rows Homed Here (owner split, 2026-07-31)

**Provenance.** These rows arrived from the rfcgate-1b RFC 7296 pilot spec. Item 15
of that spec's phase list held 17 RFC 7296 rows as work package WP-9, "Configuration
payload and remote access" (the rfcgate-1b RFC 7296 pilot spec). On 2026-07-31
the owner split WP-9. The rows that need the address-assignment FEATURE move here. The
rows that are already conformant stay in the pilot, together with two live defects.

The split follows `ai/rules/planning.md`, which prefers an existing spec that
owns the topic over a new one. This spec owns the surface, and
`internal/component/ike/engine/config.go` names it as the owner in the source. The
deferral rows are in `plan/deferrals/rfcgate-1b-rfc7296-pilot.md`.

**Work-package numbering.** A re-triage on 2026-07-30 renumbered the pilot's work
packages. "WP-9" names a different package in some of the pilot's tables, and these 17
rows also appear there as WP-11. The row ids are the stable handle. A commit message must
say which numbering it uses.

**Evidence base.** The design pass that produced this section is `tmp/design-wp9.md`, a
read-only pass over all 17 rows written 2026-07-31. Every claim restated below was
re-verified against the working tree on the same day, because a concurrent session is
editing `internal/component/ike/engine/`. Cite by function name, then re-locate the line.

### The concrete gap this spec closes

`internal/component/ike/engine/register.go` is `_ = ipPool`. Verified 2026-07-31.

An operator can configure a virtual IP pool today. The daemon parses it (FUNCTION
`parseVirtualIPPool`, `ipsec/config.go`), validates its prefix bounds
(`ipsec/validate.go` for IPv4 and `:202` for IPv6, reached from
`engine/config.go`), builds a live `eap.Pool` and logs a creation message. Then it
discards the object four lines before the engine shuts down. No client receives an
address.

This is a shipped config surface with no behavior. `ai/rules/completion.md` gives three
permitted answers: wire it, delete it, or reject the config. This spec wires it.

### Roster: the rows homed here

Ten of the 17 rows need production code. Seven are conformant today, five of them by an
RFC-sanctioned choice rather than by accident.

**Count discrepancy, stated rather than resolved silently.** The design's prose says
"nine rows need production code" while its own verdict table marks ten. The difference is
`RFC7296-3.15.1-3`.

That row's send-side obligation binds the requester, which Ze is not, so Ze is conformant
there. Only the optional responder half needs code. Owner item OI-3 below holds the open
question of whether Ze answers a `SUPPORTED_ATTRIBUTES` query. The row is therefore
conditional. Nine rows need code unconditionally, and `3.15.1-3` makes ten.

| Row | Lands as | Handle | Why it needs code |
|-----|----------|--------|-------------------|
| `RFC7296-2.19-2` | `2.19-2` | CP before SA | Ze builds no CP payload. The responder chain is IDr, CERT, AUTH, SAr2, TSi, TSr |
| `RFC7296-2.19-3` | `2.19-3` | CP in the SA-bearing message | EAP is exactly the multiple-IKE_AUTH variation, and Ze implements it. Three drop sites, and the third serves real clients |
| `RFC7296-2.19-5` | `2.19-5` | No CFG_REPLY without a CFG_REQUEST | Vacuously true today. Becomes the primary authorization guard when the feature lands |
| `RFC7296-2.19-6` | `2.19-6` | FAILED_CP_REQUIRED | No config leaf expresses the policy, and notify 37 has zero referents |
| `RFC7296-3.15.1-1` | `3.15.1-1` | One netmask, only with an address | The codec enforces no cardinality of any kind, and no consumer applies the rule |
| `RFC7296-3.15.1-3` | `3.15.1-3` | SUPPORTED_ATTRIBUTES zero-length | **Conditional on OI-3.** Attribute type 14 is not a declared constant |
| `RFC7296-3.15.1-4` | `3.15.1-4` | Ignore unrecognized attributes | Nothing ignores and nothing acts. Blocked on the RESERVED-bit codec fix |
| `RFC7296-4-2` | `4-2` | Parse CFG_REQUEST, recognize the address attribute | The antecedent becomes true when this spec lands |
| `RFC7296-4-3` | `4-3` | Reply with an address of the requested type | Same. Type fidelity is a real constraint, not a formality |
| `RFC7296-1.7-1` | **`RFC7296-1.7-3`** | Ignore attribute type 5 | Forced renumber, see "Id allocation". The constant is not declared |

### Verbatim obligations

Quoted from `rfc/full/rfc7296.txt` and re-verified 2026-07-31. The quoted text is the
requirement. Do not paraphrase it into a checklist row.

Each quotation below is external text and stays exactly as the RFC writes it.

**`RFC7296-2.19-2`** (Section 2.19):

> In all cases, the CP payload MUST be inserted before the SA payload.

**`RFC7296-2.19-3`** (Section 2.19):

> In variations of the protocol where there are multiple IKE_AUTH exchanges, the CP
> payloads MUST be inserted in the messages containing the SA payloads.

**`RFC7296-2.19-5`** (Section 2.19):

> The responder MUST NOT send a CFG_REPLY without having first received a
> CP(CFG_REQUEST) from the initiator, because we do not want the IRAS to perform an
> unnecessary configuration lookup if the IRAC cannot process the REPLY.

**`RFC7296-2.19-6`** (Section 2.19). The sentences after the MUST bind the behavior, so
read them with it:

> In the case where the IRAS's configuration requires that CP be used for a given
> identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the request,
> and terminate the Child SA creation with a FAILED_CP_REQUIRED error. The
> FAILED_CP_REQUIRED is not fatal to the IKE SA; it simply causes the Child SA creation to
> fail. The initiator can fix this by later starting a new Configuration payload request.
> There is no associated data in the FAILED_CP_REQUIRED error.

**`RFC7296-3.15.1-1`** (Section 3.15.1):

> INTERNAL_IP4_NETMASK - The internal network's netmask. Only one netmask is allowed in
> the request and response messages (e.g., 255.255.255.0), and it MUST be used only with
> an INTERNAL_IP4_ADDRESS attribute.

**`RFC7296-3.15.1-3`** (Section 3.15.1):

> SUPPORTED_ATTRIBUTES - When used within a Request, this attribute MUST be zero-length
> and specifies a query to the responder to reply back with all of the attributes that it
> supports. The response contains an attribute that contains a set of attribute
> identifiers each in 2 octets. The length divided by 2 (octets) would state the number of
> supported attributes contained in the response.

**`RFC7296-3.15.1-4`** (Section 3.15.1):

> Unrecognized or unsupported attributes MUST be ignored in both requests and responses.

**`RFC7296-4-2`** (Section 4):

> If an implementation supports responding to such requests, it MUST parse the CP payload
> of type CFG_REQUEST in the first message in the IKE_AUTH exchange and recognize a field
> of type INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS.

**`RFC7296-4-3`** (Section 4):

> If it supports leasing an address of the appropriate type, it MUST return a CP payload
> of type CFG_REPLY containing an address of the requested type. The responder may include
> any other related attributes.

**`RFC7296-1.7-1`**, landing as `RFC7296-1.7-3` (Section 1.7):

> Implementations that conform to this document MUST ignore proposals that have
> configuration attribute type 5, the old value for INTERNAL_ADDRESS_EXPIRY.

**Erratum 5056 governs `1.7-1`.** The word "proposals" is wrong: a configuration attribute
belongs to a Configuration payload, not to an SA proposal. The row keeps the verbatim text
and the code implements the corrected semantics. Ze ignores the ATTRIBUTE, never the
proposal. The literal reading would discard an SA proposal because a Configuration payload
elsewhere in the same message carried attribute 5.

**`1.7-1` does not duplicate `3.15.1-4`.** The pilot asked this spec to settle the
question (the rfcgate-1b RFC 7296 pilot spec). They are separate obligations.

`3.15.1-4` scopes attributes Ze does not recognize. `1.7-1` names type 5 and binds even an
implementation that DOES recognize it, because RFC 4306 defined type 5. An implementation
with RFC 4306 heritage gets no instruction from `3.15.1-4` at all. Keep both sites, and
drop type 5 before the unknown-attribute default arm.

### What stays in the pilot

**Seven conformant rows.** Each is discharged by a property the code has, not by an absent
guard. They belong with the pilot's other evidence work.

| Row | Why it stays | Owner item |
|-----|--------------|------------|
| `RFC7296-2.19-1` | Conformant by non-participation. Ze is not an IRAC, and Section 4 makes that role optional | OI-1 |
| `RFC7296-2.19-4` | Sender obligation on the IRAC. Tied to `2.19-1` | OI-1 |
| `RFC7296-2.20-1` | Conformant. Ze sends no CP payload, which is one of the two permitted answers | - |
| `RFC7296-3.15.1-2` | Sender obligation on the requester. Receive-side tolerance only | - |
| `RFC7296-3.15.1-5` | Conformant by the RFC's own MAY at `rfc/full/rfc7296.txt:6477` | OI-2 |
| `RFC7296-3.15.1-6` | Same MAY | OI-2 |
| `RFC7296-3.15.1-7` | Same MAY, and satisfied by name: Ze returns a response without a CFG_ACK payload | OI-2 |

**Two live defects.** Both are live today, both are independent of the CP feature, and
both are landable on their own merits.

| Defect | Site | What is wrong | Row it violates |
|--------|------|---------------|-----------------|
| RESERVED-bit fold | `wire/payload_cp.go` reads, `:49` writes | The attribute type is read and written as 16 bits. RFC 7296 Section 3.15.1 defines "Reserved (1 bit) - This bit MUST be set to zero and MUST be ignored on receipt" plus "Attribute Type (15 bits)". A peer that sets the bit on `INTERNAL_IP4_ADDRESS` yields `0x8001` | `RFC7296-2.5-7` on read, `2.5-6` on write. Both are already gated |
| IPv6 out-of-range lease | `eap/pool.go` | `allocateV6` writes the host ID into `ip6[8:]`, which assumes a prefix no longer than `/64`. `validatePoolPrefix` permits `/48../126` (`ipsec/validate.go`). A `/96` pool leases addresses outside the configured range | none. It is a correctness bug with no row |

**Cross-spec dependency.** `RFC7296-3.15.1-4` is homed here but cannot be proven until the
RESERVED-bit fix lands. The negative half of its tagged pair IS the reserved-bit case. If
the pilot keeps that fix, this spec depends on it. If the fix is easier to land here,
move it and say so in both specs.

### Id allocation (cross-spec hazard, BLOCKING)

`check_id_allocation` (`scripts/dev/rfc_requirements.py`) refuses a new id whose
ordinal is at or below its section's high-water mark. The mark comes from the committed
HEAD summaries. A section with no mark is skipped entirely
(`scripts/dev/rfc_requirements.py`). Refusal is permanent for that ordinal: the
row must take a higher one and leaves document order.

**Marks measured 2026-07-31**, against `git show HEAD:rfc/short/rfc7296.md` and against
the working tree, which agree for every section below.

| Section | Ids present | Mark | Consequence |
|---------|-------------|------|-------------|
| `RFC7296-1.7` | `1.7-2` only | **2** | **`1.7-1` is REFUSED.** It lands as `RFC7296-1.7-3` |
| `RFC7296-2.19` | none | none | `-1` through `-6` accepted as written. This spec is the sole claimant |
| `RFC7296-2.20` | none | none | `-1` accepted as written. Sole claimant |
| `RFC7296-3.15.1` | none | none | `-1` through `-7` accepted as written. Sole claimant |
| `RFC7296-4` | none | none | Accepted **only if the landing order is right**. Three claimants |

**Section 4 has three claimants and no mark. Whichever package lands first sets it.**

| Appendix A id | Owner | Pilot phase item |
|---------------|-------|------------------|
| `4-1` | pilot WP-12 (notify shape, expired SA, INITIAL_CONTACT, NO_ADDITIONAL_SAS) | 7 |
| `4-2` | **this spec** | 15 |
| `4-3` | **this spec** | 15 |
| `4-4` | pilot WP-10 (certificates, identities, management interface) | 14 |

**Correction to the briefing that opened this work.** The briefing stated that `4-1` had
already landed and set the mark to 1. It has not. No `RFC7296-4-*` id exists at HEAD or in
the working tree, and all four rows are still `NOT IMPL` in the pilot's Appendix A
(the rfcgate-1b RFC 7296 pilot spec). The hazard is real, and its mechanism
is "first to land sets the mark", not "the mark is already 1".

**The rule: Section 4 must land in ascending ordinal order across specs.** `4-1` first,
then this spec's `4-2` and `4-3`, then `4-4`.

| Landing order | Result |
|---------------|--------|
| `4-1`, then `4-2`/`4-3`, then `4-4` | Mark goes 1, then 3, then 4. All four land at their document ordinals. Correct |
| `4-4` first (the pilot schedules WP-10 at item 14, before item 15) | Mark jumps to 4. `4-1`, `4-2` and `4-3` are all REFUSED and renumber out of document order |
| `4-2`/`4-3` first | Mark goes to 3. `4-1` is REFUSED and must become `-5` or higher |

**This spec's `4-2` and `4-3` MUST land before the pilot's `4-4`.** The pilot's own phase
list schedules WP-10 (item 14) before WP-9 (item 15), which strands these rows. Three ways
to honor the rule, in preference order:

1. The pilot defers `4-4` until this spec's Section 4 pair lands. `4-4` is a
   configuration-capability row with no dependency on the other three.
2. All four Section 4 rows land in one commit.
3. This spec lands its Section 4 pair early, in a small separate commit.

**Recompute every mark at the moment of landing. Never copy a number from this section.**
One command per section:

    git show HEAD:rfc/short/rfc7296.md | grep -oE 'RFC7296-4-[0-9]+' | sort -V | tail -1

After the rows land, run `make ze-rfc-index-update` and commit `ai/RFC-REQUIREMENTS.md` in the
SAME commit. The ledger records each tagged test's `file:line`, and both verify modes of
`ze-rfc-check` fail on a stale ledger.

**Tag carriers.** A tag can live in a `_test.go` or in a `.ci` under `test/ipsec/`, which
`ze-functional-test` runs. `test/interop-ipsec/` is REFUSED as a carrier, because nothing
runs that suite automatically. The strongSwan scenarios in this spec are goal-validation
evidence, and they earn no row a polarity.

### Security requirements (BLOCKING)

`RFC7296-2.19-5` and `RFC7296-2.19-6` are AUTHORIZATION checks.
`ai/rules/evidence.md` requires each to deny on a miss, on an empty set, on an
unmapped input and on an error.

**`2.19-5`: a nil test is NOT sufficient.** The idiomatic Go shape is a `*wire.PayloadCP`
variable set inside a type switch, then `if cpReq != nil`. That shape fails open twice. It
checks neither the CFG type nor a present-but-empty attribute set. A peer that sends
`CP(CFG_SET)`, which the RFC permits Ze to ignore, would receive a CFG_REPLY carrying a
leased address it never requested.

The decision is ONE function that returns an explicit `ok`. Every condition must hold.

| # | Condition | The miss it closes |
|---|-----------|--------------------|
| 1 | Exactly one CP payload in the decrypted inner chain of this request | Two payloads is ambiguous attacker-supplied input. "Take the first" is a silent choice |
| 2 | The CFG type is `CFGTypeRequest` | The unmapped-input case. `Reply`, `Set`, `ACK` and all 251 undefined values deny |
| 3 | At least one `INTERNAL_IP4_ADDRESS` or `INTERNAL_IP6_ADDRESS` attribute is present | The empty-set case. A CP with zero attributes is present but empty |
| 4 | The peer's profile enables the configuration payload | Policy, resolved explicitly |

**If the call site can express `if cpReq != nil`, the design has already failed.**

**`2.19-6`: the permissive branch is not where instinct puts it.** The permissive branch
is "proceed without CP". The restrictive branch is "fail the Child SA". A policy lookup
written as a bare map read returns `false` on a miss, and `false` means "not required",
which is permissive. The lookup returns an explicit `resolved` flag beside the answer.

| Profile state | Resolved | Behavior |
|---------------|----------|----------|
| A remote-access profile | true | Read the configured policy leaf |
| A site-to-site peer | true | Not required, structurally. The leaf does not exist on that branch |
| Nil or unresolvable | **false** | **DENY.** A session that reached response build with no bound profile is a bug state |

Two behaviors the RFC pins, and an implementer will get both wrong:

1. **The IKE SA survives.** The response carries the notify with no SA payload and no TS
   payload, and the IKE SA stays established. A teardown violates the RFC.
2. **No Child SA is installed.** The refusal must short-circuit BEFORE
   `createFirstChildSA` runs in FUNCTION `buildAuthResponse`. A refusal after it leaks an
   installed Child SA and derived keys.

**Pool exhaustion is the default failure mode, not an attack.** Verified 2026-07-31:

- `Pool.Release` (`eap/pool.go`) has **no non-test caller**. The only callers are
  `eap/pool_test.go` and `:128`.
- **No address lease or expiry concept exists anywhere in `internal/component/ike/`.** The
  only "expiry" matches are SA rekey lifetimes and PKI certificate expiry, and
  `releaseRequestWindow` is a message-ID window, not an address.

So every client that connects consumes an address permanently. A `/24` pool serving a
50-user site exhausts after 254 connections, which is a few days of normal laptop churn.
Then every later client is refused. No attacker is needed. **Release wiring plus lease
expiry is not a hardening extra. It is the difference between a feature that works for a
week and one that works.**

The authenticated attack is second. `Allocate` (`eap/pool.go`) takes no identity, so
one valid credential can open many IKE SAs and drain the pool. A per-identity lease
maximum bounds it, and address reuse for the same identity is what the RFC already asks
for.

**Mitigating fact, stated plainly.** The CFG_REQUEST arrives inside the encrypted inner
chain of an IKE_AUTH message. An attacker must hold valid credentials to reach the
allocator. That removes the unauthenticated denial of service and leaves an authenticated
one.

**Further bounds required:**

- Cap the attribute count per CP payload in `ReadFrom`.
- Make `NewPool` reject a range it cannot represent, rather than trust its caller.
- Give `allocateV4` a rotating cursor.
- Add a `doctor-ipsec-virtual-ip-pool` check that reports pool size and utilization, so
  exhaustion becomes visible before it is total.

**One correctness obligation with no row.** Section 3.15.4 requires an
`INTERNAL_ADDRESS_FAILURE` notification when address assignment fails. The IKE SA is still
created. Appendix A extracted no Section 3.15.4 row.

`NotifyInternalAddressFailure` (`wire/payload_notify.go`) has zero referents, as does
`NotifyFailedCPRequired`. An unextracted obligation is still an obligation
(`ai/rules/rfc-compliance.md`). Implement it. Owner item OI-4 asks whether Section 3.15.4
gains rows.

### Owner items (RFC compliance escalations)

`ai/rules/rfc-compliance.md` forbids choosing anything narrower than full compliance plus
full proof. Each item below is a place where something narrower is on the table. None is a
request to skip a row. In every case the tagged pair gets written and the row stays gated.
The question is which way the obligation is discharged. **The main thread raises these.**

| Id | Question | Cost of the wider answer |
|----|----------|--------------------------|
| OI-1 | Does Ze become an IRAC as well as an IRAS, or is declining the client role the recorded answer? Section 4 says implementations are not required to support requests for temporary IP addresses | A CP producer in the initiator build path, an initiator-side CFG_REPLY consumer, and an operator leaf. About 1.5 days |
| OI-2 | Confirm that ignoring CFG_SET is the recorded answer, so `3.15.1-5`, `-6` and `-7` are discharged by that choice rather than by absence | An attribute-acceptance model Ze has no use for, plus a CFG_ACK builder. About 0.5 day, for an exchange with no defined use |
| OI-3 | Does Ze answer a `SUPPORTED_ATTRIBUTES` query as responder? Answering is strictly more compliance and needs no permission. Ask only to decline | About 0.5 day. Attribute type 14 is not a declared constant |
| OI-4 | Does Section 3.15.4 gain checklist rows, or is that the extraction sign-off's business? | **Answered 2026-08-02: this spec's business.** Extraction cannot take it |

**OI-4 is answered, and the answer moves the work here.** The RFC 7296 extraction
sign-off walked Section 3.15.4 on 2026-08-02 and found that it states the
`INTERNAL_ADDRESS_FAILURE` obligation in indicative prose, with no capitalised
RFC 2119 keyword. The extractor derives no site there, so the sign-off has nothing to
map. `unsourced-ids` cannot carry it either: that field may only name a requirement id
that already exists in `rfc/short/rfc7296.md`, and none does.

So the obligation is real, is unextracted, and is invisible to every gate. Only a
checklist row makes it visible, and the row cannot be written before the behavior it
gates exists. This spec owns both.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugins.md` - registration and proximity
  -> Constraint: no new communication mechanism; the engine already owns its dispatch loop.
- [ ] `ai/rules/goroutine-lifecycle.md`
  -> Constraint: a per-client session is a goroutine per *lifecycle* (allowed), never a
     goroutine per event. It MUST be reaped when its SA dies, or a road-warrior gateway leaks
     one goroutine and one pool address per connection attempt.
- [ ] `ai/rules/protocol.md`
  -> Constraint: pool exhaustion must refuse the client with a clear reason, never assign a
     duplicate or silently omit the address.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - IKEv2
  -> Constraint: Section 2.19 the initiator requests an internal address with CP(CFG_REQUEST)
     in IKE_AUTH; the responder answers CP(CFG_REPLY) in the same exchange that carries SAr2/TSr.
  -> Constraint: Section 3.15 CFG_REQUEST attribute values MAY be zero-length (a request);
     the reply carries the assigned value.
  -> Constraint: Section 2.16 EAP: the responder authenticates with its own long-term credential
     in the first IKE_AUTH response, before the EAP exchange.
- [ ] `rfc/short/rfc5216.md` - EAP-TLS
  -> Constraint: Section 5.3 both sides MUST path-validate. The gateway validates the client
     chain against `ra.Auth.ca-certificate`.
- [ ] `rfc/short/rfc3748.md` - EAP
  -> Constraint: Section 5.1 the authenticator begins with an Identity request; the identity is
     therefore known before the method starts, which is what makes per-user lookup possible.

**Key insights:**
- `Session.handleIdentity` (`eap/eap.go`) sets `s.identity` and only then calls
  `s.method.Start(...)`. That ordering is the hook for per-user credential resolution: the
  method can be handed the right password after the identity is known and before it is used.
- `MethodConfig.Password` (`eap/eap.go`) is a single value captured at
  `newMSCHAPv2Method` (`eap/eap_mschapv2.go`), so it cannot express a user table as-is.
- `PeerSession.responderBusy` gates ONE in-flight half-open handshake **per session**
  (`reconcile.go`). Sharing one session across all road warriors would serialize them
  and let one client's handshake block every other's; a per-client session avoids this and
  reuses the established/DPD/rekey machinery unchanged.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/register.go` - `OnConfigure` (:250), pool built + discarded
  (:313-320, :372), `matchResponderPeer` (:536), `tryResponderSAInit` (:557), dispatch
- [ ] `internal/component/ike/engine/reconcile.go` - `PeerSession` (:18), `startPeerSession`
  (:353), `run`/`runOnce` reconnect loop
- [ ] `internal/component/ike/engine/responder.go` - `newResponderSA` (:25), `isEAPMode` (:49)
- [ ] `internal/component/ike/engine/responder_eap.go` - `eapMethodConfig` (:22),
  `startResponderEAP` (:88), `handleResponderEAP` (:164)
- [ ] `internal/component/ike/eap/eap.go` - `Session`, `MethodConfig` (:156), `Begin` (:163),
  `handleIdentity` (:205), `Identity()` (:192)
- [ ] `internal/component/ike/eap/eap_mschapv2.go` - `newMSCHAPv2Method` (:38), `Start` (:48)
- [ ] `internal/component/ike/eap/pool.go` - `NewPool` (:35), `Allocate` (:89), `Release` (:126)
- [ ] `internal/component/ike/wire/payload_cp.go` - complete CP codec, zero callers
- [ ] `internal/component/ike/ipsec/types.go` - `RemoteAccessConfig` (:420), `EAPUser` (:399)
- [ ] `test/interop-ipsec/scenarios/responder-eap-mschapv2/` - the shape that works today

**Behavior to preserve:**
- Site-to-site peers keep exact-match admission and priority. A configured peer address must
  never be diverted into the remote-access path.
- `responderBusy` semantics per session (RFC 7296 Section 2.4 accept-in-parallel) unchanged.
- Every existing interop scenario unchanged and green.
- A config with no `remote-access` block behaves exactly as today.

**Behavior to change:**
- An unsolicited IKE_SA_INIT from an unconfigured source is admitted when `remote-access` is
  configured (today: dropped).
- `eap-user` entries become live credentials.
- The virtual IP pool is assigned rather than discarded.

## Data Flow (MANDATORY)

### Entry Point
Inbound UDP IKE_SA_INIT from an arbitrary source address, on the IKE (500) or NAT-T (4500) port.

### Transformation Path
1. `dispatchInbound` -> no SATable entry, `ExchangeIKESAInit`, not a response.
2. `matchResponderPeer(src)` -> nil (no configured peer).
3. **NEW** `matchRemoteAccess(src)` -> if `remote-access` is configured, create (or find) a
   per-client `PeerSession` from a `SiteToSitePeer` synthesized from `RemoteAccessConfig`.
4. `newResponderSA` -> IKE_SA_INIT exchange -> IKE_AUTH.
5. IKE_AUTH carries IDi, no AUTH (EAP) plus **CP(CFG_REQUEST)**: stash the request on the SA.
6. `startResponderEAP` -> gateway AUTH from `ra.Auth` -> EAP-Request/Identity.
7. EAP rounds; **NEW** identity resolves to an `eap-user` before the method starts.
8. On EAP success + AUTH-from-MSK: **NEW** allocate from the pool, build CP(CFG_REPLY),
   narrow TSr to the assigned address, send with SAr2.
9. SA teardown (`StateDead`, DPD, delete, reap): **NEW** release the address, reap the session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| UDP transport <-> engine dispatch | `transport.Packet` | [ ] |
| engine <-> EAP session | `eap.MethodConfig`, `eap.Session` | [ ] |
| engine <-> pool | `eap.Pool.Allocate`/`Release` | [ ] |
| engine <-> wire | `wire.PayloadCP` | [ ] |

### Integration Points
- **Superseded 2026-07-31:**

  > ~~`eap.Pool` gains no API; `Allocate`/`Release` are already the right shape.~~

  `Allocate` takes no identity (`eap/pool.go`), so it can express neither a
  per-identity quota nor address reuse across a rekey. The signature changes.
- **Superseded 2026-07-31:**

  > ~~`wire.PayloadCP` gains no API; the codec is complete.~~

  The codec needs the RESERVED-bit mask on both sides, an attribute-count bound, and four
  missing constants (types 5, 13, 14 and 15).
- `eap.MethodConfig` gains a per-user credential resolver (the one genuinely new SDK-ish surface).

### Architectural Verification
- [ ] No bypassed layers - admission stays in dispatch, auth stays in the EAP session
- [ ] No unintended coupling - the pool is owned by the engine, not reached from `eap` internals
- [ ] No duplicated functionality - reuses `PeerSession`, `newResponderSA`, the EAP session, the
      CP codec and the pool; builds no parallel FSM
- [ ] Registration over hardcoding - no new per-feature switch in a shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A `SiteToSitePeer` synthesized from `RemoteAccessConfig` drives the existing responder FSM unchanged | scenario `responder-eap-mschapv2` proves the same FSM works for EAP with a peer struct | a parallel responder path would be needed, much larger | interop scenario `remote-access-eap-mschapv2` | unvalidated |
| A-2 | `wire.PayloadCP` round-trips the attributes strongSwan sends | codec reads/writes per RFC 7296 3.15 but has never run against a real peer | CP codec fixes needed first | unit round-trip + interop | **broken** (2026-07-31). `ReadFrom` reads a 16-bit attribute type at `wire/payload_cp.go`, but RFC 7296 Section 3.15.1 defines 1 reserved bit plus a 15-bit type. A peer that sets the reserved bit on `INTERNAL_IP4_ADDRESS` yields type `0x8001`, and Ze reads it as an unknown attribute. The "if wrong" column called this correctly: codec fixes come first |
| A-3 | `eap.Pool.Allocate` is safe under concurrent road-warrior handshakes | `pool.go` takes a lock in `allocateV4`/`allocateV6` | duplicate address assignment | `-race` test with concurrent Allocate | unvalidated |
| A-4 | A per-client `PeerSession` can be reaped without disturbing configured peers | `activePeersMap` is name-keyed and reconcile iterates config peers | reconcile would delete or resurrect dynamic sessions | reconcile test with both kinds present | unvalidated |
| A-5 | strongSwan can be driven as a road-warrior client in the existing lab | the lab already runs strongSwan as initiator (every non-responder scenario) | interop proof needs another client | scenario `remote-access-eap-mschapv2` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Admitting unconfigured sources is a DoS surface: each attempt costs a goroutine, an SA and a pool address | memory/goroutine growth under a flood | cap concurrent half-open remote-access sessions; release the address only on success, never on half-open; reuse the existing `responderBusy` per-client gate |
| R-2 | A dynamic session leaks when its SA dies in an unusual state | goroutine count grows across connect/disconnect cycles | explicit reap path plus a test that cycles N clients and asserts the session map returns to its base size |
| R-3 | Pool exhaustion silently assigns nothing and the client establishes with no address | client connects but cannot route | refuse the exchange with a clear log and a NOTIFY; never send an empty CFG_REPLY |
| R-4 | Per-user lookup changes `eap.Session` shape and breaks the site-to-site EAP path | scenario `responder-eap-mschapv2` goes red | keep `MethodConfig.Password` working as-is; the resolver is additive and only consulted when set |
| R-5 | The reaper races the dispatch goroutine, which may be mid-handshake for that client | `-race` failures in the ike suite | reuse the existing atomic/ownedSA discipline; run `-race` on the engine package |
| R-6 | **The `2.19-5` guard fails open on the CFG type.** The idiomatic `if cpReq != nil` hands a leased address to a peer that sent `CP(CFG_SET)` | none. The happy path passes, and an ordinary client never exercises the CFG_SET case | The four conditions in ONE function returning an explicit `ok`. Run the CFG-type mutation and the attribute-presence mutation SEPARATELY |
| R-7 | **Leases are never released, and the pool exhausts under normal churn.** `Release` has no non-test caller and no lease concept exists | `Available()` decreases monotonically. The first report is a user who cannot connect | Phase B. A lease table plus expiry, and a doctor check that surfaces utilization before exhaustion is total |
| R-8 | **Only the direct IKE_AUTH drop site is wired, and the feature silently fails against real clients.** Road warriors send the CFG_REQUEST in the final EAP IKE_AUTH | Unit tests pass, and the strongSwan scenario fails or hangs | AC-22's negative drives the EAP path specifically. Its mutation must redden ONLY the negative |
| R-9 | **The leased address never reaches the traffic selectors.** Allocation placed after the child response payloads are built leaves the client's proposal in place | The client gets an address and cannot route. Interop fails at the traffic stage, not the negotiation stage | Ordering constraint 1 in "Configuration Payload Phases". The interop scenario asserts traffic flows, not merely that a CFG_REPLY arrived |
| R-10 | **Section 4 ids are stranded because the pilot lands `4-4` first.** The pilot's own phase list schedules WP-10 before WP-9 | `check_id_allocation` fails naming `4-2` | "Id allocation" above. Recompute at landing, and prefer deferring `4-4` |
| R-11 | **The RESERVED-bit defect is not fixed, and `3.15.1-4` goes green under a test that never sets the bit.** A conforming peer stays misparsed | none. A peer that never sets the bit interoperates | AC-21's second clause IS the reserved-bit case, and its mutation reverts the mask |
| R-12 | **`2.19-6` stops the IKE SA.** "Fail the request" reads as "end the session" | The client retries in a loop | AC-20 asserts the SA is still established after the refusal |
| R-13 | **Engine line numbers move under a concurrent session.** A second agent edits `internal/component/ike/engine/` now | A tag cites a line holding different code | Every citation in this spec names its function. Re-locate by function name before you quote a line in a tag |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT from an unconfigured source, remote-access configured | -> | `matchRemoteAccess` -> dynamic `PeerSession` | `TestRemoteAccessAdmitsUnconfiguredSource` |
| EAP identity of a configured `eap-user` | -> | per-user credential resolver | `TestRemoteAccessResolvesEAPUserPassword` |
| CP(CFG_REQUEST) in IKE_AUTH | -> | pool allocate + CFG_REPLY | `TestRemoteAccessAssignsVirtualIP` |
| SA teardown | -> | pool release + session reap | `TestRemoteAccessReleasesAddressOnTeardown` |
| strongSwan road-warrior client | -> | the whole path | `test/interop-ipsec/scenarios/remote-access-eap-mschapv2` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IKE_SA_INIT from an unconfigured source, `remote-access` configured | admitted; a per-client session is created |
| AC-2 | IKE_SA_INIT from an unconfigured source, NO `remote-access` configured | dropped exactly as today |
| AC-3 | IKE_SA_INIT from a source matching a configured site-to-site peer, `remote-access` also configured | handled by the site-to-site peer, never diverted |
| AC-4 | EAP identity matching an `eap-user` with a password | authenticates with that user's password |
| AC-5 | EAP identity matching no `eap-user` | EAP-Failure; no fallback credential is tried |
| AC-6 | two road warriors with different identities | each authenticates with its own credential |
| AC-7 | CP(CFG_REQUEST) asking for INTERNAL_IP4_ADDRESS | CFG_REPLY carries an address from the pool, plus netmask and any configured DNS/domain. ~~(netmask unconditional)~~ **Amended 2026-07-31:** `RFC7296-3.15.1-1` permits at most one netmask, and only beside an `INTERNAL_IP4_ADDRESS`. An IPv6-only lease carries no netmask. See AC-19 |
| AC-8 | responder TSr after assignment | narrowed to the assigned address |
| AC-9 | pool exhausted | client refused with a clear reason; no duplicate address; no empty CFG_REPLY |
| AC-10 | SA teardown | the address returns to the pool (`Available()` restored) and the session is reaped |
| AC-11 | N connect/disconnect cycles | goroutine count and session map return to base |
| AC-12 | `remote-access mode eap-tls` with no `ca-certificate` | config verify rejects (RFC 5216 Section 5.3) |
| AC-13 | `ra.Auth` certificate/ca-certificate absent from candidate PKI | config verify rejects naming the reference |
| AC-14 | strongSwan road-warrior client, EAP-MSCHAPv2 | establishes, receives a virtual IP, passes traffic |
| AC-15 | concurrent `Allocate` under `-race` | no duplicate address, no race report |

**Added 2026-07-31 (owner split).** One criterion per homed RFC row, plus two for the
lease lifecycle the rows depend on. Each needs a tagged test pair, and each mutation must
redden the half named.

| AC ID | Row | Input / Condition | Expected Behavior |
|-------|-----|-------------------|-------------------|
| AC-16 | `2.19-2` | any IKE_AUTH response carrying both CP and SA | the CP payload index is strictly less than the SA payload index |
| AC-17 | `2.19-3` | an EAP session | the CFG_REPLY appears in the response carrying SAr2, which is the final IKE_AUTH, not the first. A non-EAP session puts it in the only response. Both flows are asserted in one test |
| AC-18 | `2.19-5` | a request with no CP, or a CP whose type is Reply, Set or ACK, or a CP with zero attributes | the response carries no CP payload. A well-formed request DOES get a CFG_REPLY, so the guard is not refusing everything |
| AC-19 | `3.15.1-1` | a CFG_REPLY | at most one netmask, and one only when the reply also carries `INTERNAL_IP4_ADDRESS`. An IPv6-only lease carries no netmask |
| AC-20 | `2.19-6` | the policy requires CP and the client sent none | the response carries `FAILED_CP_REQUIRED`, no SA payload and no TS payload. The IKE SA stays established, and no Child SA was installed |
| AC-21 | `3.15.1-4` | a CFG_REQUEST carrying unknown attribute types beside `INTERNAL_IP4_ADDRESS` | a correct CFG_REPLY. The unknown types appear in neither the reply nor any state. `INTERNAL_IP4_ADDRESS` carrying the RESERVED bit is recognized as type 1 and answered |
| AC-22 | `4-2` | a CFG_REQUEST in the first IKE_AUTH request, in both the direct and the EAP flow | parsed, and its address attribute recognized. A CFG_REQUEST arriving only in the final EAP IKE_AUTH is also recognized |
| AC-23 | `4-3` | a client requesting only IPv6 against an IPv4-only pool | no address of the wrong family. The session still completes, per RFC 7296 Section 3.15.4 |
| AC-24 | `1.7-1` | attribute type 5 in a CFG_REQUEST | ignored. It appears in no reply and changes no state. The rest of the payload, and the SA proposal in the same message, are processed as usual |
| AC-25 | `3.15.1-3` | conditional on OI-3. A zero-length `SUPPORTED_ATTRIBUTES` request | if answered: a reply whose value length is a multiple of 2, derived from the handler set rather than a hardcoded list. If declined: no `SUPPORTED_ATTRIBUTES` is emitted |
| AC-26 | lease expiry | a lease whose configured lifetime passes after its IKE SA ends | the address returns to the pool. `Available()` recovers without a new connection |
| AC-27 | lease quota | one identity opening more sessions than the configured maximum | the excess is refused. The pool does not drain from a single credential |
| AC-28 | IPv6 pool longer than `/64` | a `/96` pool | every leased address falls inside the configured prefix |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | road warrior dials in with EAP-MSCHAPv2 and gets a virtual IP | UDP -> admission -> responder SA -> EAP -> user lookup -> pool -> CFG_REPLY -> child SA | interop scenario `remote-access-eap-mschapv2` |
| 2 | road warrior dials in with EAP-TLS | same, with client-chain validation against `ra.Auth.ca-certificate` | interop scenario `remote-access-eap-tls` |
| 3 | operator commits a remote-access block with a bad certificate reference | commit -> tx bridge -> `OnConfigVerify` -> rejection | `test/reload/test-tx-ipsec-remote-access-pki.ci` |
| 4 | operator inspects who is connected | `show vpn ipsec ...` reflects dynamic sessions | `TestRemoteAccessSessionsVisible` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoteAccessAdmitsUnconfiguredSource` | `engine/remote_access_test.go` | AC-1 | |
| `TestRemoteAccessDropsWhenNotConfigured` | `engine/remote_access_test.go` | AC-2 | |
| `TestRemoteAccessNeverDivertsConfiguredPeer` | `engine/remote_access_test.go` | AC-3 | |
| `TestRemoteAccessResolvesEAPUserPassword` | `eap/eap_user_test.go` | AC-4 | |
| `TestRemoteAccessUnknownUserFailsClosed` | `eap/eap_user_test.go` | AC-5 | |
| `TestRemoteAccessDistinctUsers` | `eap/eap_user_test.go` | AC-6 | |
| `TestConfigPayloadRoundTrip` | `wire/payload_cp_test.go` | AC-7 codec | |
| `TestRemoteAccessAssignsVirtualIP` | `engine/remote_access_test.go` | AC-7 | |
| `TestRemoteAccessNarrowsTrafficSelector` | `engine/remote_access_test.go` | AC-8 | |
| `TestRemoteAccessPoolExhaustionRefuses` | `engine/remote_access_test.go` | AC-9 | |
| `TestRemoteAccessReleasesAddressOnTeardown` | `engine/remote_access_test.go` | AC-10 | |
| `TestRemoteAccessNoSessionLeak` | `engine/remote_access_test.go` | AC-11 | |
| `TestValidateRemoteAccessRequiresCAForEAPTLS` | `ipsec/validate_test.go` | AC-12 | |
| `TestValidateRemoteAccessRejectsUnknownPKIRefs` | `ipsec/validate_test.go` | AC-13 | |
| `TestPoolAllocateConcurrent` | `eap/pool_test.go` | AC-15 (`-race`) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pool `/N` IPv4 | /8../30 (existing `validatePoolPrefix`) | /30 | /7 | /31 |
| pool `/N` IPv6 | /48../126 (existing) | /126 | /47 | /127 |
| addresses issued from a /30 pool | usable hosts | last usable | N/A | one past the pool -> AC-9 refusal |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-ipsec-remote-access-pki` | `test/reload/test-tx-ipsec-remote-access-pki.ci` | commit of a remote-access block with an unresolvable certificate reference is refused | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `remote-access-eap-mschapv2` | `test/interop-ipsec/scenarios/` | strongSwan (road-warrior client) | AC-14: unconfigured source establishes, gets a virtual IP, passes traffic | |
| `remote-access-eap-tls` | `test/interop-ipsec/scenarios/` | strongSwan | EAP-TLS road warrior, client chain validated | |

## Files to Modify
- `internal/component/ike/engine/register.go` - admission fallback; stop discarding the pool
- `internal/component/ike/engine/reconcile.go` - dynamic session creation and reaping
- `internal/component/ike/engine/responder_eap.go` - per-user credential resolution
- `internal/component/ike/engine/responder.go` / `fsm.go` - CP request stash, CFG_REPLY, TS narrowing
- `internal/component/ike/eap/eap.go` - `MethodConfig` per-user resolver (additive)
- `internal/component/ike/eap/eap_mschapv2.go` - accept the resolved credential
- `internal/component/ike/ipsec/validate.go` - `ValidateRemoteAccess` PKI-aware (inherited item 4)
- `internal/component/ike/engine/config.go` - pass the candidate PKI closures through

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | ~~No~~ **Yes** | ~~the surface already exists in `ipsec/yang/ze-ipsec-conf.yang`~~ **Amended 2026-07-31:** the pool surface exists, but `RFC7296-2.19-6` and `2.20-1` need leaves that do not. Add `container configuration-payload` (`enabled`, `required`, `application-version`, `lease-lifetime`, `maximum-leases-per-identity`) to `ipsec/yang/ze-ipsec-conf.yang`, and widen `leaf dns` to `leaf-list dns` |
| CLI commands | Yes | dynamic sessions must appear in the existing `show vpn ipsec` views |
| Functional test | Yes | `test/reload/test-tx-ipsec-remote-access-pki.ci` |
| Doctor check | Yes | pool sanity (a pool that cannot serve one client) reuses the `ike` `DoctorChecks` added by the sibling spec |
| Prometheus counters | Yes | connected remote-access clients, pool utilisation, auth failures |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | remote-access becomes live, not new syntax; check the guide |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` if session views change |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | `docs/guide/vpn/*` remote-access section |
| 7 | Wire format changed? | [ ] | CP payload now emitted: `docs/architecture/wire/*` |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | `docs/features/rfc-status.md` RFC 7296 Section 2.19 row |
| 10 | Test infrastructure changed? | [ ] | new interop scenarios |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` remote-access VPN row |
| 12 | Internal architecture changed? | [ ] | IKE subsystem doc |
| 13 | Route metadata keys? | [ ] | |
| 14 | Prometheus counters? | [ ] | telemetry doc |
| 15 | Registered inventory changed? | [ ] | doctor check inventory |
| 16 | Doc source anchors on changed files? | [ ] | grep `docs/` per changed file |
| 17 | Existing docs show examples? | [ ] | verify remote-access examples now describe live behavior |

## Files to Create
- `internal/component/ike/engine/remote_access.go` + `_test.go`
- `internal/component/ike/eap/eap_user.go` + `_test.go` (or additive in `eap.go`)
- `test/reload/test-tx-ipsec-remote-access-pki.ci`
- `test/interop-ipsec/scenarios/remote-access-eap-mschapv2/{ze.conf,swanctl.conf,check.py}`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/remote-access-eap-tls/{ze.conf,swanctl.conf,check.py}`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - admission fallback + dynamic session, failing wiring test
   - Tests: `TestRemoteAccessAdmitsUnconfiguredSource`, `TestRemoteAccessDropsWhenNotConfigured`,
     `TestRemoteAccessNeverDivertsConfiguredPeer`
   - Files: `engine/remote_access.go`, `engine/register.go`, `engine/reconcile.go`
   - Verify: an unconfigured source reaches a session; no site-to-site behavior changes
2. **Phase: Per-user credentials** - AC-4..AC-6
   - Tests: the three `eap/eap_user_test.go` tests
   - Files: `eap/eap.go`, `eap/eap_mschapv2.go`, `engine/responder_eap.go`
   - Verify: unknown identity fails closed; scenario `responder-eap-mschapv2` still green
3. **Phase: Configuration payload + pool** - AC-7..AC-9
   - Tests: `TestConfigPayloadRoundTrip`, `TestRemoteAccessAssignsVirtualIP`,
     `TestRemoteAccessNarrowsTrafficSelector`, `TestRemoteAccessPoolExhaustionRefuses`
   - Files: `engine/responder.go`, `engine/fsm.go`, `engine/remote_access.go`
4. **Phase: Lifecycle** - AC-10, AC-11, AC-15
   - Tests: `TestRemoteAccessReleasesAddressOnTeardown`, `TestRemoteAccessNoSessionLeak`,
     `TestPoolAllocateConcurrent`
   - Verify: `go test -race` on the engine and eap packages
5. **Phase: Config validation** - AC-12, AC-13 (inherited deferral)
   - Files: `ipsec/validate.go`, `engine/config.go`, `test/reload/*.ci`
6. **Phase: Interop** - AC-14
   - `test/interop-ipsec/scenarios/remote-access-eap-mschapv2`, `remote-access-eap-tls`; `make ze-interop-ipsec-test`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
7. **Observability + docs** - counters, `show` views, documentation checklist
8. **Full verification, review gate, closure**

### Configuration Payload Phases (added 2026-07-31, from the WP-9 design pass)

The eight phases above predate the RFC row split and stay valid for admission and per-user
credentials. Phases A to G below refine phase 3 ("Configuration payload + pool"), which the
design showed to be a feature build rather than a wiring step. Phase 3 is superseded by
this table. Phases 1, 2 and 4 to 8 are unchanged.

| # | Phase | Work | Rows unblocked | Estimate |
|---|-------|------|----------------|----------|
| A | Codec correctness | The RESERVED-bit mask on read and write, the length-truncation guard, the trailing-remnant error, an attribute-count cap, and the four missing constants (types 5, 13, 14, 15). Plus codec tests | none directly. Unblocks `3.15.1-4`, `4-2`, `1.7-1` | 0.5 day |
| B | Pool hardening | Identity-bound allocation, the lease table and expiry, `NewPool` self-bounding, a rotating IPv4 cursor, and the IPv6 prefix fix for lengths beyond `/64` | none directly. Unblocks `4-3` | 1 day |
| C | Config surface | `container configuration-payload`, `leaf dns` widened to `leaf-list dns`, the multi-pool decision, parse, validate, doctor check, completion | `2.19-6`, `2.20-1` | 0.5 day |
| D | The consumer | `engine/cp.go`. The CP case at all THREE drop sites. The EAP start signature. The reply insertion between AUTH and SAr2. Traffic-selector narrowing before the child response payloads are built. Real pool wiring in place of `_ = ipPool` | `2.19-2`, `2.19-3`, `4-2`, `4-3`, `3.15.1-1` | 1.5 days |
| E | Authorization and error paths | The two fail-closed guards, `FAILED_CP_REQUIRED` emission with its short-circuit before the Child SA install, and `INTERNAL_ADDRESS_FAILURE` | `2.19-5`, `2.19-6` | 0.5 day |
| F | Tests | The tagged pairs, every mutation run and reverted, the pool tests, a `test/ipsec/` functional test, and the strongSwan road-warrior scenario | all 17 proven | 1.5 days |
| G | Discovery and closure | `docs/features.md`, the guide, the wire architecture page, `docs/features/rfc-status.md` rows, the summary rows, `make ze-rfc-index-update`, and the Integration Checklist re-answer | - | 0.5 day |

**Total: roughly 6 days.** Phases A, B and C are genuinely parallel.

**Phases A and B are worth landing alone if the rest slips.** Phase A repairs a live
`RFC7296-2.5-7` violation. Phase B repairs a live out-of-range IPv6 lease and removes the
exhaustion-by-churn failure. Neither depends on the CP consumer.

**Ordering constraints that are not negotiable:**

1. Address allocation happens BEFORE the responder builds its child traffic selectors.
   Otherwise the responder echoes the selectors the client proposed, and the leased address
   never reaches them. The client then negotiates an address and cannot route.
2. The `2.19-6` refusal short-circuits BEFORE the Child SA is installed.
3. Parse early, reply late. `4-2` requires parsing the CFG_REQUEST in the FIRST IKE_AUTH
   request. `2.19-3` requires the reply in the message carrying the SA payload, which under
   EAP is the FINAL response. A design that replies in the first response breaks EAP.

### What this must NOT break

| Invariant | Why it is at risk | The guard |
|-----------|-------------------|-----------|
| A message MUST NOT be rejected over payload order (`RFC7296-2.5-13`) | An implementer enforcing `2.19-2` adds a receive-side order check. `2.19-2` is a SEND obligation only | This spec adds no receive-side order check of any kind |
| Unrecognized attributes are ignored, never rejected (`RFC7296-3.15.1-4`) | The same instinct adds "reject unknown attribute" | The positive drives a request carrying unknown types and asserts the session still completes |
| The IKE SA survives a FAILED_CP_REQUIRED | "Fail the request" reads as "kill the session" | The positive asserts the SA is still established after the refusal |
| No Child SA or key material is installed on a refusal | The child SA install runs before the payload list is built | The positive asserts no Child SA was installed |
| EAP sessions still establish | Phase D changes the EAP start signature and the EAP response walk | `TestResponderEAPSessionWired` stays green |
| Site-to-site peers are unaffected | The policy lookup must return a determined "not required", not a miss | The negative drives a site-to-site peer and asserts normal establishment |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Fail-closed | Unknown identity, exhausted pool, and unparseable CP each DENY with a named reason |
| Resource safety | No goroutine, SA, or pool address outlives its client (R-1, R-2) |
| Site-to-site untouched | Every existing scenario green; admission precedence proven by AC-3 |
| Concurrency | `-race` on engine + eap; concurrent Allocate has no duplicate |
| Mutation-verify | Disable each new guard; its test must go red |
| Rule: no-layering | The old `_ = ipPool` discard is deleted, not bypassed |
| Rule: exact-or-reject | Pool exhaustion refuses; never a partial CFG_REPLY |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| pool no longer discarded | `grep -n '_ = ipPool' internal/component/ike/engine/register.go` returns nothing |
| CP payload emitted | interop capture or `TestRemoteAccessAssignsVirtualIP` |
| eap-user live | `TestRemoteAccessResolvesEAPUserPassword` |
| interop green | `make ze-interop-ipsec-test` scenarios `remote-access-eap-mschapv2`, `remote-access-eap-tls` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated resource consumption | admission happens before authentication by construction (RFC 7296); bound half-open sessions and do not allocate a pool address until EAP succeeds |
| Credential handling | `eap-user` passwords never logged, never serialized (`json:"-"` as `MethodConfig.Password` already is) |
| Identity spoofing | the EAP identity selects the credential but does not by itself authenticate; the method must still verify |
| Address reuse | a released address must not be handed out while the old child SA still forwards |
| Client chain validation | EAP-TLS gateway validates against `ra.Auth.ca-certificate`, never an empty pool (see the sibling spec's finding) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| CP codec mismatch with strongSwan | fix `wire/payload_cp.go`, add a round-trip case from the real capture |
| Scenario `responder-eap-mschapv2` regresses | the per-user resolver was not additive; restore `MethodConfig.Password` precedence |
| Goroutine leak | Phase 4 reap path |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The feature is mostly *wiring already-built parts*: the CP codec, the IP pool, and the EAP
  authenticator all exist and all have zero callers. That is worth stating plainly, because it
  is also how the gap survived so long -- every individual piece looked done.
- **Amended 2026-07-31.** The insight above is half right, and the missing half is why the
  estimate moved from a phase to roughly six days. The parts exist, and two of them are
  broken in ways only a consumer reveals. Zero callers is exactly why nobody found the
  reserved-bit fold or the IPv6 out-of-range lease. **A component with no consumer has no
  evidence of correctness, however finished it looks.** Treat "complete but unused" as
  unverified, not as done.
- The design pass and this spec disagreed in four places. Each disagreement was the spec
  trusting existing code too far. The codec was called complete. The pool was called the
  right shape. The YANG was called sufficient. AC-7 emitted a netmask unconditionally.
  Every one is struck through above with its reason.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-client dynamic `PeerSession` synthesized from `RemoteAccessConfig` | one shared remote-access session; a parallel road-warrior FSM | reuses the established/DPD/rekey/teardown machinery unchanged, and gives each client its own `responderBusy` gate so one client cannot block another |
| Per-user credential resolver on `MethodConfig`, consulted after the EAP identity | rebuild the session per identity; look the user up in the engine and recreate the method | `handleIdentity` already sequences identity before method start; additive so the site-to-site EAP path is untouched |
| Allocate the pool address only after EAP success | allocate at admission | an unauthenticated source must not be able to drain the pool (R-1) |

## Known Limitations
- (fill during implementation)

## RFC Documentation

`// RFC 7296 Section 2.19` above the CFG_REQUEST/CFG_REPLY handling,
`// RFC 7296 Section 3.15.1` above the attribute encoding,
`// RFC 5216 Section 5.3` above the client-chain validation,
`// RFC 3748 Section 5.1` above the identity-then-method ordering the resolver depends on.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| a road warrior can establish against ze | interop test | scenario `remote-access-eap-mschapv2` vs strongSwan |
| per-user credentials are live | unit + interop | |
| virtual IP assignment works | interop (client holds the address) | |
| no resource leak per connection | unit lifecycle test | |
| remote-access config is validated | functional `.ci` | |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
