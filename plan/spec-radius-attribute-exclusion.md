# Spec: radius-attribute-exclusion

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-radius-acct-session-attributes, closed 2026-09-04 |
| Phase | closing (schema, typed set, wiring, filter and the two `.ci` in `6bd1653b41`; `Packet.OmitAcctDelayTime` in `0488b5dfac`; the docs in `a32367b75`; closure repairs in this commit) |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `internal/component/l2tp/plugins/authradius/acct.go` -- `(*radiusAcct).buildAcctPacket`, which builds the attribute list this spec filters.
3. `internal/component/l2tp/plugins/authradius/handler.go` -- `buildAccessRequestAttrs`, the Access-Request half.
4. `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang` -- where the new container goes.
5. `internal/component/l2tp/plugins/authradius/config.go` -- `parseConfigFromTree`.

## Task

Ze emits Calling-Station-Id, Event-Timestamp, Acct-Delay-Time and
Acct-Terminate-Cause unconditionally, on the owner's ruling to copy Juniper.
`spec-radius-acct-session-attributes` did that and closed on 2026-09-04, and
`docs/architecture/l2tp/bng-1-radius-attributes.md` is where the four attributes
are now described. That ruling is right and it leaves an operator no
way to suppress one attribute their server or their billing pipeline dislikes.

Give them Juniper's answer, because Juniper has one and the owner asked for it to
be the model (2026-09-03). Junos exposes `exclude` under
`[edit access profile <name> radius attributes]`, naming an attribute and the
message types to suppress it from: `access-request`, `accounting-start`,
`accounting-stop`, `accounting-on`, `accounting-off`. The default is include.

Junos is not alone, and the survey settles the shape rather than merely
permitting it. Six implementations were read (2026-09-03):

| Implementation | Shape | Scope | Curated |
|----------------|-------|-------|---------|
| Junos | `exclude`, opt-out | per access profile | named leaves AND a numeric escape |
| Nokia SR OS | `include-radius-attribute`, opt-in | per accounting policy | curated names only |
| Cisco IOS/XE | `radius-server attribute <n> include-in-acct-req`, opt-in | global | curated, about ten attributes |
| Cisco IOS/XE, cnBNG | `radius-server attribute list` bound by accept/reject | per server group | any number, but a required attribute's rejection is REFUSED |
| accel-ppp | four `[radius]` booleans | global | curated |
| osvbng | none; its mappings only ADD | - | - |

Three conclusions follow. The curated per-attribute toggle is the majority and is
the least typing for the operator, where an accept/reject list costs three
commands and a second object. Per-profile scope is the modern BNG answer, because
the reason to suppress an attribute belongs to the billing system behind one
profile rather than to the box. And every implementation but Junos either has no
numeric form or guards it: Cisco's list explicitly refuses to reject a required
attribute and passes it through anyway.

**Ze's equivalent scope is `l2tp auth radius`**, which is the profile-shaped
container in ze's tree: it already holds `nas-identifier`, `nas-port-id-format`,
`acct-interval`, `coa-port` and the `server` list. `attributes` becomes a
sibling of `server`.

### The one place ze diverges from Junos, deliberately

Junos names attributes two ways. The legacy way is a curated keyword set, and its
own reference page says it "allows you to configure only those attributes and
VSAs for which the statement syntax includes a specific option". Junos 18.1R1
added a numeric form, `standard-attribute <number>` and
`vendor-id <id> vendor-attribute <number>`, which accepts any number and whose
unsupported configurations simply "have no effect".

**Ze takes the curated form only.** A numeric form lets an operator write a line
that suppresses a required attribute and get no error either way. RFC 2866
Section 5.13's legend reads "1  Exactly one instance of this attribute MUST be
present", and the table gives that count to Acct-Status-Type and Acct-Session-Id.
Note 1 adds: "An Accounting-Request MUST contain either a NAS-IP-Address or
NAS-Identifier". A YANG enum that never names those three refuses the line at
config load, which is where the operator can still fix it.

This is `ai/rules/principles.md` applied to a config surface: a silent no-op is a
value that is silently wrong.

**Junos's named leaves carry the pattern worth copying twice over.** Each one is
individually constrained: in `junos-conf-access@2022-01-01.yang`,
`accounting-terminate-cause` accepts only `accounting-off`, because that is where
the attribute can appear. Ze does the same. Acct-Terminate-Cause is Stop-only in
ze (RFC 2866 Section 5.10), so `accounting-start` and `accounting-interim` are
not legal packet types for it and the schema says so. The guard is structural, so
no runtime "you may not disable that" check is ever written.

### The enum, grounded in what ze emits

Read at `(*radiusAcct).buildAcctPacket` and `buildAccessRequestAttrs`.

| Excludable | Attribute | Why the choice is real |
|-----------|-----------|------------------------|
| yes | `calling-station-id` (31) | peer-supplied, and a MAC on the relay path; some operators do not want it stored |
| yes | `event-timestamp` (55) | a server that stamps its own receive time may prefer to |
| yes | `acct-delay-time` (41) | accel-ppp ships it off by default |
| yes | `acct-terminate-cause` (49) | Stop records only, and some pipelines key on their own teardown signal |
| yes | `nas-port-id` (87) | already configurable in shape by `nas-port-id-format`, so suppressing it is the same decision taken further |
| yes | `framed-ip-address` (8) | an operator whose addressing is reported elsewhere |
| **no** | Acct-Status-Type (40) | RFC 2866 Section 5.13 count `1` |
| **no** | Acct-Session-Id (44) | RFC 2866 Section 5.13 count `1` |
| **no** | NAS-Identifier (32) / NAS-IP-Address (4) | RFC 2866 Section 5.13 Note 1, and `appendNASIdentity` already quotes it |

Service-Type, Framed-Protocol, NAS-Port-Type and NAS-Port are `0-1` in Section
5.13 and ze emits all four, so they COULD join the enum. They are left out until
someone asks (`ai/rules/simplicity.md`): six values nobody has to read beats ten
that were added on the chance.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli.md` -- keyword before value, and every command supports all
  pipe operators.
  → Constraint: `exclude <attribute> { packet-type [ ... ]; }` puts the keyword
  first and the value second.
- [ ] `ai/patterns/config-option.md` -- the structural template for a new leaf.
- [ ] `ai/rules/config.md` -- YANG versus env var.
  → Decision: this is operator policy in the running config, so YANG.
- [ ] `docs/guide/l2tp.md` -- the operator-facing RADIUS attribute list, which
  gains the exclusion section.
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` -- the attribute page
  the sibling spec is adding rows to.
- [ ] `docs/research/l2tpv2-ze-integration.md` -- declared as the design document
  by the sources this spec edits.
  → Constraint: accounting failures MUST NOT tear down sessions; a suppressed
  attribute must not change that.
- [ ] `docs/research/l2tpv2-implementation-guide.md` -- declared as the design
  document by the L2TP sources.
  → Constraint: its section numbering is its own and is never written into a code
  comment as an RFC citation (`plan/journal/reference-checked-claim-unchecked.md`).

### RFC Summaries (Scope: protocol)
- [ ] RFC 2866 Section 5.13 -- the Table of Attributes and its legend, quoted
  above.
  → Constraint: the enum omits every attribute whose count is `1`, and the NAS
  identity pair Note 1 governs.
- [ ] RFC 2865 Section 5 -- "Text of length zero (0) MUST NOT be sent; omit the
  entire attribute instead."
  → Constraint: exclusion and the existing omit-when-empty behavior are different
  mechanisms and must not be conflated in the code.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang`
  -- `l2tp/auth/radius` holds `source-address`, `nas-identifier`,
  `nas-port-id-format`, `timeout`, `retries`, `acct-interval`, `coa-port`,
  `require-message-authenticator` and the `server` list keyed on `name`. There is
  no attribute-selection surface of any kind.
- [x] `internal/component/l2tp/plugins/authradius/acct.go` --
  `(*radiusAcct).buildAcctPacket` appends User-Name through `AppendTextAttr`,
  then Acct-Status-Type, Acct-Session-Id, Service-Type, Framed-Protocol,
  NAS-Port-Type and NAS-Port in one literal, then `appendNASIdentity`, then
  NAS-Port-Id and Framed-IP-Address behind their own `ok` guards, then the
  counters on Stop and Interim.
- [x] `internal/component/radius/dict.go` -- ze sends `AcctStatusStart`,
  `AcctStatusStop` and `AcctStatusInterimUpdate` only. There is no Acct-On and no
  Acct-Off, so Junos's five message types become four for ze.

**Behavior to preserve:** every attribute ze emits when nothing is excluded; the
omit-when-empty guards, which are a separate mechanism; the rule that an
accounting failure never tears down a session.

**Behavior to change:** the attribute list is filtered by config before it is
encoded.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An operator writing `exclude` under `l2tp auth radius attributes`, then a
subscriber session producing an Access-Request or an accounting record.

### Transformation Path
1. `parseConfigFromTree` reads the `attributes` container into a typed exclusion
   set: attribute code to the set of message types it is suppressed from.
2. The set travels with the existing config into the handler and the accounting
   plugin.
3. `buildAccessRequestAttrs` and `buildAcctPacket` filter the assembled list
   against the set for the record type they are building.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → typed set | `parseConfigFromTree` | [ ] |
| Config → packet builders | the existing config plumbing, no new channel | [ ] |
| Builder → wire | the attribute is absent from the encoded packet | [ ] |

### Integration Points
- `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang`
- `internal/component/l2tp/plugins/authradius/config.go`
- `internal/component/l2tp/plugins/authradius/acct.go`
- `internal/component/l2tp/plugins/authradius/handler.go`

### Architectural Verification
The filter is applied at ONE place per builder, on the assembled list, rather
than as a condition on each append. A per-append condition would be six
conditions today and one more per attribute ever added, and it is the shape that
lets a future attribute be added without its exclusion working.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The six excludable attributes are all `0-1` in RFC 2866 Section 5.13 or absent from its table | read in `rfc/full/rfc2866.txt` | The enum offers a conformance hole | AC-8 | confirmed |
| A-2 | Ze emits no Acct-On and no Acct-Off | `dict.go` defines Start, Stop and Interim-Update only, and the plugin uses those three | The message-type enum is short by two | read at the producer | confirmed |
| A-3 | Filtering the assembled list cannot reorder attributes in a way a server rejects | RFC 2866 Section 3: "The order of attributes of different types is not required to be preserved" | Filtering must preserve position | AC-7, `TestExcludePreservesTheRemainingOrder` | confirmed |
| A-4 | `buildAcctPacket` appends every excludable attribute, so one filter point per builder reaches all six | the spec's own reading of the builder | One attribute needs a second mechanism | read at the producer | **broken** |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator excludes an attribute their billing system requires and finds out at month end | none at runtime | The guide names what each attribute feeds. FreeRADIUS's own default queries are the worked example, and they are not uniform: `raddb/mods-config/sql/ippool/mysql/queries.conf` matches `AND callingstationid = '%{Calling-Station-Id}'` in `stop_clear`, `alive_update` and `start_update` with NO `:-` fallback, so suppressing attribute 31 leaves IP pool leases unexpired. By contrast the main accounting queries are defensive: Event-Timestamp falls back to server time and Acct-Terminate-Cause defaults to `NAS-Reboot`, and Acct-Delay-Time appears in no default query at all |
| R-2 | The enum grows to include a required attribute later | a schema change | AC-8 asserts the three required attributes are absent from the enum, so adding one breaks a test |
| R-3 | Exclusion and omit-when-empty are conflated, so an excluded attribute reads as an absent value | a test that cannot tell them apart | AC-4 excludes an attribute whose value IS present |

## Blast Radius

`internal/component/l2tp/plugins/authradius` and its YANG, plus two additive
lines in the RADIUS client. The admin RADIUS path is untouched: this is
subscriber config, and the admin path sends a different and much smaller
attribute set.

**A-4 broke while the filter was written.** `buildAcctPacket` does NOT append
Acct-Delay-Time: `(*radius.Client).Exchange` writes it, through
`setAcctDelayTime` (`internal/component/radius/client.go`), because RFC 2866
Section 5.2 counts "how many seconds the client has been trying to send this
record for" and only the client knows that. A filter over the assembled list
therefore never sees the attribute, so the exclusion travels to the client as
`radius.Packet.OmitAcctDelayTime`, checked once where the client stamps. The
field's zero value stamps, so every other caller is unchanged and no existing
test moved. Dropping `acct-delay-time` from the enum instead would have been a
scope reduction, which needs the owner rather than the author.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `exclude calling-station-id` in the config tree | → | the attribute is absent from the Accounting-Request on the wire | `TestExcludedAttributeIsAbsentFromTheWire` |

The wiring test drives the config tree, so a container that parses and never
reaches a builder fails it.

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No `attributes` container | Every packet is byte-identical to what ze sends without this feature |
| AC-2 | `exclude calling-station-id` with no `packet-type` | The attribute is absent from every record type where it would appear |
| AC-3 | `exclude calling-station-id { packet-type [ accounting-interim ]; }` | Absent from Interim, present in Start and Stop |
| AC-4 | An excluded attribute whose value IS known and non-empty | Still absent. Exclusion is not the omit-when-empty path |
| AC-5 | `exclude` naming `access-request` | The attribute is absent from the Access-Request and the accounting records are unaffected |
| AC-6 | Two `exclude` entries | Both apply, independently |
| AC-7 | An excluded attribute | Every attribute NOT named is still present, in its original relative order |
| AC-8 | The YANG enum | Names none of Acct-Status-Type, Acct-Session-Id, NAS-Identifier or NAS-IP-Address, and the config refuses any of those words |
| AC-9 | An unknown attribute word, or an unknown packet type | Refused at config load, naming the leaf and the permitted values |
| AC-10 | Every attribute excluded that can be | The record still carries Acct-Status-Type, Acct-Session-Id and the NAS identity, so it is still a conformant Accounting-Request |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Excludes Calling-Station-Id from Interim records only | config → filtered builder → wire | `test/l2tp/radius-acct-exclude.ci` |
| 2 | Configures nothing and sees every attribute | config → builder → wire | the sibling spec's `radius-acct-wire.ci`, unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNoExclusionsLeavesThePacketUnchanged` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-1 | green |
| `TestExcludeWithNoPacketTypeAppliesEverywhere` | same | AC-2 | green |
| `TestExcludePerPacketType` | same | AC-3 | green |
| `TestExcludeBeatsAKnownValue` | same | AC-4 | green |
| `TestExcludeOnAccessRequestOnly` | `.../handler_test.go` | AC-5 | green |
| `TestTwoExclusionsApplyIndependently` | same | AC-6 | green; the row was missing from this table until closure |
| `TestExcludePreservesTheRemainingOrder` | `.../acct_test.go` | AC-7 | green |
| `TestRequiredAttributesAreNotInTheEnum` | `.../config_test.go` | AC-8 | green |
| `TestExcludeRefusesUnknownWords` | same | AC-9 | green, four subtests |
| `TestFullyExcludedRecordIsStillConformant` | `.../acct_test.go` | AC-10 | green |
| `TestExcludedAttributeIsAbsentFromTheWire` | same | Wiring | green |
| `TestExcludeAcctDelayTimePerPacketType` | `.../exclude_test.go` | AC-3 for the attribute the client stamps | green |
| `TestAccountingRequestOmitsAcctDelayTimeOnRequest` | `internal/component/radius/acct_delay_time_omit_test.go` | AC-2 for Acct-Delay-Time, read off the wire a UDP socket received | green, two subtests |
| `TestRFC2866AccountingRetransmitWithoutDelayTimeKeepsIdentifier` | same file | RFC 2866 Section 3: the identical-contents retransmit branch this spec MADE reachable. Added at closure | green; discrimination record in `rfc/discrimination/rfc2866.json` |

Every unit test above runs under `go test -race`, which was green over both
packages on 2026-09-04.

Every unit test above lives in
`internal/component/l2tp/plugins/authradius/exclude_test.go` rather than in the
three files this table names. One concern in one file, and none of the existing
RFC-tagged test files is touched.

### Boundary Tests (numeric inputs)
| Input | Boundary | Expected |
|-------|----------|----------|
| zero `exclude` entries | empty container | identical to no container |
| every excludable attribute excluded | six entries | a conformant record with the three required attributes |
| the same attribute excluded twice | duplicate key | refused by the YANG list key |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-acct-exclude` | `test/l2tp/radius-acct-exclude.ci` | An operator writes three exclusions and the config surface accepts them | |
| `radius-acct-exclude-invalid` | `test/l2tp/radius-acct-exclude-invalid.ci` | An operator names Acct-Session-Id and the config surface refuses it | |

The wire-level scenario is NOT delivered. It needs the accounting peer fixture
(`internal/test/fixture/tunnel_fixture_l2tp_ppp.go`, `checkRecordAttributes`) to
assert an ABSENT attribute, and that function is another session's uncommitted
work in this checkout. The two tests above own the config surface, and
`TestExcludedAttributeIsAbsentFromTheWire` owns the octets in process.

### Interop Tests (Scope: protocol)
| Scenario | Peer implementation | Asserts |
|----------|--------------------|---------|
| the existing `radius-acct-attrs` scenario, extended | a real RADIUS accounting server | A record with attributes excluded is still accepted and stored, which is the claim that matters: exclusion must not make the record unparseable |

## Files to Modify
- `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang`
- `internal/component/l2tp/plugins/authradius/config.go`
- `internal/component/l2tp/plugins/authradius/acct.go`
- `internal/component/l2tp/plugins/authradius/handler.go`
- `docs/guide/l2tp.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`

## Files to Create
- `test/l2tp/radius-acct-exclude.ci`

### Integration Checklist
- [ ] The container completes in the CLI and appears in `ze config schema`.
- [ ] `ze config dump` round-trips a config carrying exclusions.
- [ ] The typed exclusion set has a non-test caller in both builders.

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/guide/l2tp.md` -- the exclusion syntax, the enum, and what each
      attribute feeds on the server side, so an operator knows what they lose.
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` -- a note that the list
      is filterable, and which three attributes are not.
- [ ] `docs/config-reference.md` -- regenerated, not hand-edited.

## Implementation Steps

### Implementation Phases
1. **Phase: The schema.** The `attributes` container, the curated enum, the
   message-type enum. AC-8 and AC-9 first, because the refusal is the feature.
2. **Phase: The typed set.** `parseConfigFromTree` into a value the builders can
   ask one question of.
3. **Phase: Wiring first.** The config-driven wiring test RED before either
   builder filters anything.
4. **Phase: The filter.** One filter point per builder, on the assembled list.
5. **Phase: Functional and interop.**
6. **Phase: Docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| The enum | Names no attribute RFC 2866 Section 5.13 counts as `1`, and neither NAS identity leaf |
| One filter point | Not a condition per append |
| Order | The surviving attributes keep their relative order |
| Two mechanisms | Exclusion and omit-when-empty are distinguishable in a test |
| Default | An absent container changes nothing |

### Deliverables Checklist
| Deliverable | Verification method | Status |
|-------------|--------------------|--------|
| The schema refuses a required attribute | `TestRequiredAttributesAreNotInTheEnum` | done -- `go test -race ./internal/component/l2tp/plugins/authradius/` green 2026-09-04 |
| Per-packet-type exclusion | `TestExcludePerPacketType` | done -- same run |
| Wiring from config to wire | `TestExcludedAttributeIsAbsentFromTheWire` | done -- same run; it encodes the packet and walks the octets |
| A fully excluded record is conformant | `TestFullyExcludedRecordIsStillConformant` | done -- same run |
| Functional proof | `test/l2tp/radius-acct-exclude.ci` | done -- `./le functional l2tp` 23/23 PASS 2026-09-04, both exclude scenarios among them |
| Guide updated | the `docs/guide/l2tp.md` diff | done -- `a32367b75`, +77 lines, the exclusion section at line 328 onward |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Downgrade | No exclusion can remove an attribute a MUST requires |
| Silent no-op | An unknown word is refused, never ignored |
| Injection | The enum is closed, so no operator string reaches an attribute type |

### Failure Routing
| Failure | Route |
|---------|-------|
| Unknown attribute or packet type | Refused at config load by the YANG enum |
| A required attribute named | Refused the same way, because it is not in the enum |

## Design Insights

The interesting decision was not whether to copy Juniper but which of their two
naming forms to copy. Their numeric form is more capable and it is the one that
lets a config make the device non-conformant with no diagnostic. The curated enum
is less capable on purpose, and the capability it gives up is the ability to be
wrong quietly.

## Key Design Decisions

| Decision | Why | What it forecloses |
|----------|-----|--------------------|
| `attributes` under `l2tp auth radius` | It is ze's profile-shaped scope, matching Junos's access profile | A global setting across several RADIUS configurations |
| A curated enum, no numeric form | A numeric form accepts a line that suppresses a required attribute and has no effect | Suppressing a VSA, or an attribute not yet in the enum |
| Per message type, defaulting to all | Junos's shape, and ze's four attributes already differ by record type | Nothing |
| One filter point per builder | A per-append condition grows with the attribute list and lets a new attribute miss the feature | Nothing |
| Subscriber path only | The admin RADIUS path sends a different, much smaller set | An operator excluding an attribute from admin logins |

## Known Limitations

- Six attributes are excludable. Service-Type, Framed-Protocol, NAS-Port-Type and
  NAS-Port are `0-1` in RFC 2866 Section 5.13 and could join the enum when asked.
- There is no `ignore` counterpart for attributes ze RECEIVES in an Access-Accept.
  Junos has one; it is a separate feature and is not in this spec.

## RFC Documentation (Scope: protocol)
- RFC 2866 Section 5.13 and its Note 1; RFC 2865 Section 5. Both enrolled. This
  spec adds no protocol behavior: it removes optional attributes and asserts that
  the required ones survive.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] The Junos statement was read on Juniper's own reference page, not inferred.
- [x] RFC 2866 Section 5.13's legend and Note 1 were read in `rfc/full/rfc2866.txt`.
- [x] The excludable set was grounded in what `buildAcctPacket` actually emits.
- [x] The owner approved the shape on 2026-09-03.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Wiring test RED before the filter, GREEN after
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Citations repointed

---

## Implementation Summary

### What Was Implemented
- `attributes exclude` under `l2tp auth radius`, six presence containers, each
  with a `packet-type` leaf-list enumerating only the record types that
  attribute can reach
  (`internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang`).
- The typed set and its parser: `attributeExclusions`, `packetKind`,
  `packetKindSet`, `parseAttributeExclusions`, `parseExcludedPacketKinds`,
  `filter` (`internal/component/l2tp/plugins/authradius/exclude.go`).
- One filter point per builder: `(*radiusAcct).buildAcctPacket` and
  `buildAccessRequestAttrs`, each calling `attributeExclusions.filter` on the
  finished list.
- The reload path: `activateRadiusConfig` calls `setExclusions` on both the auth
  and the accounting instance (`register.go`).
- `radius.Packet.OmitAcctDelayTime` and the `stampsAcctDelayTime` branch of
  `(*radius.Client).Exchange`, which is how the one attribute the builder never
  appends is held back (`internal/component/radius/packet.go`, `client.go`).
- Two `.ci`: `test/l2tp/radius-acct-exclude.ci` and
  `test/l2tp/radius-acct-exclude-invalid.ci`.

### Bugs Found/Fixed
- **The committed tree could not compile its own test binary.** `6bd1653b41`
  changed the signature of `buildAccessRequestAttrs` from a `string` to an
  `attributePolicy` and did not carry `nasportid_test.go`, whose three call sites
  still passed a string. The fix already existed in the working tree, which is
  why the commit message could read "go test over the package is green": that
  sentence was measured against a tree the commit did not produce. The file
  lands in commit A. Journal:
  `plan/journal/tree-state-claim-published-unverified.md`.
- **Three claims elsewhere went stale when `OmitAcctDelayTime` landed**, and the
  diff could not show any of them, because each is a sentence in another file
  about `(*Client).Exchange`:
  - `rfc/short/rfc2866.md`, Enrolment reason: "ze stamps Acct-Delay-Time on
    every Accounting-Request". It does so by default only, and the same cell
    said the identical-contents half of RFC2866-3-3 is "proven where ze still
    produces it, on the Access-Request path", which stopped being the whole
    truth the moment the accounting path could produce it too.
  - `rfc/short/rfc2866.md`, Support coverage: the four attributes are now
    per-record-type suppressible.
  - The tag prose of `TestRFC2866AccountingRetransmitTakesANewIdentifier`
    (`internal/component/radius/rfc2866_accounting_test.go`): "an accounting
    retransmission whose attributes do not change is a packet ze no longer
    produces".
  All three are repaired, and `./le rfc index-update` re-rendered
  `rfc/enrolled.txt`, `rfc/requirements/rfc2866.md` and
  `docs/features/rfc-status.md`.
- **An RFC 2866 MUST became reachable and was left unproven.** RFC 2866 Section
  3 carries RFC 2865's rule that "For retransmissions where the contents are
  identical, the Identifier MUST remain unchanged". Excluding Acct-Delay-Time
  makes ze produce exactly that packet on the accounting path, where nothing
  asserted it. `TestRFC2866AccountingRetransmitWithoutDelayTimeKeepsIdentifier`
  now does, with a discrimination record in `rfc/discrimination/rfc2866.json`.
- **A discrimination record staled on our producer.** `rfc/discrimination/
  rfc2865.json` held a revert record for RFC2865-4.1-1 negative whose break was
  applied to `buildAcctPacket`, and `6bd1653b41` rewrote that function. Its
  producer fingerprint moved, so `./le rfc check` reported the red as never
  re-observed. Re-recorded with `./le rfc discriminate-record`.

### Documentation Updates
- `docs/guide/l2tp.md` and `docs/architecture/l2tp/bng-1-radius-attributes.md`
  were written in `a32367b75`. The architecture page carries
  `<!-- source: internal/component/l2tp/plugins/authradius/exclude.go -- attributeExclusions, filter, parseAttributeExclusions -->`
  and a second anchor on the YANG module.
- `rfc/short/rfc2866.md` repaired at closure, then `./le rfc index-update`.
- `./le doc check verify` is RED on this checkout and none of its findings names
  a RADIUS or L2TP config surface. Every one it reports is website command-catalog
  and llms.txt drift for BGP, interface and resolve commands. This spec adds no
  CLI command, so it is outside that population.

### Deviations from Plan
- The wire-level functional scenario stayed undelivered, as the spec's own TDD
  section states: it needs `checkRecordAttributes` to assert an ABSENT attribute,
  and that function is another session's uncommitted work.
  `TestExcludedAttributeIsAbsentFromTheWire` owns the octets in process instead.
- The TDD table named nine files it does not use. Every unit test lives in
  `exclude_test.go`, which the spec says one paragraph below the table.
- The TDD table had no row for AC-6. The test
  (`TestTwoExclusionsApplyIndependently`) existed from the first commit; only the
  row was missing. Added at closure.
- The Documentation Update Checklist called `docs/config-reference.md`
  "regenerated, not hand-edited". It is hand-authored: it carries no GENERATED
  header and its l2tp section is prose about `nas-port-id-format`. See the
  Documentation Verified table for why no edit is owed there.
- One test was added at closure that the spec never planned:
  `TestRFC2866AccountingRetransmitWithoutDelayTimeKeepsIdentifier`. The spec did
  not plan it because A-4 broke during implementation, and the branch it proves
  did not exist when the TDD plan was written.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4: `buildAcctPacket` appends every excludable attribute, so one filter point per builder reaches all six | `(*radius.Client).Exchange` writes Acct-Delay-Time, because RFC 2866 Section 5.2 counts what only the client knows | writing the filter and finding no attribute 41 in the list | `radius.Packet.OmitAcctDelayTime` carries the decision to the client. Recorded in Blast Radius during implementation |
| approach | `6bd1653b41` changed a function signature and committed six of the seven files that name it | `nasportid_test.go` held three call sites and stayed uncommitted, so the committed tree's test binary does not build | closure ran `git diff` over the package before trusting the tree | The file lands in commit A. `ai/rules/principles.md`: the work a change owes is what it can REACH, never the files you edited |
| escalation | The diff was searched for staled claims | Three of the four staled claims are sentences in files the diff never touched, each about `(*Client).Exchange` | deriving the set from the PRODUCER's name rather than from the diff | The producer-name search is what found the `rfc/short/` cells, the tagged comment and the staled `rfc2865.json` record. A diff cannot show a sentence in another file that just became false |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Give the operator Junos's `exclude`, at ze's profile-shaped scope | Done | `yang/ze-l2tp-auth-radius-conf.yang`, `container attributes` under `l2tp auth radius` | Sibling of `server`, as designed |
| The curated named form, never the numeric one | Done | `excludableAttributes` (`exclude.go`) and the six presence containers | No leaf accepts a number |
| Per-attribute typing of the legal record types | Done | each container's `packet-type` leaf-list | `acct-terminate-cause` enumerates `accounting-stop` alone |
| The schema refuses a required attribute | Done | the enum names six words and none of the four | `TestRequiredAttributesAreNotInTheEnum` |
| One filter point per builder | Done | `buildAcctPacket`, `buildAccessRequestAttrs` | Not a condition per append |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestNoExclusionsLeavesThePacketUnchanged` | Compares the encoded attribute stream of a configured-with-nothing instance against an untouched one, over all three record types |
| AC-2 | Done | `TestExcludeWithNoPacketTypeAppliesEverywhere` | And `TestAccountingRequestOmitsAcctDelayTimeOnRequest` for attribute 41 |
| AC-3 | Done | `TestExcludePerPacketType`, `TestExcludeAcctDelayTimePerPacketType` | Interim loses it, Start and Stop keep it |
| AC-4 | Done | `TestExcludeBeatsAKnownValue` | Verified at the producers: see Goal Validation |
| AC-5 | Done | `TestExcludeOnAccessRequestOnly` | Drives `buildAccessRequestAttrs` and then all three accounting records |
| AC-6 | Done | `TestTwoExclusionsApplyIndependently` | |
| AC-7 | Done | `TestExcludePreservesTheRemainingOrder` | |
| AC-8 | Done | `TestRequiredAttributesAreNotInTheEnum`, `test/l2tp/radius-acct-exclude-invalid.ci` | Verified at the schema: see Goal Validation |
| AC-9 | Done | `TestExcludeRefusesUnknownWords`, four subtests | Unknown attribute, unknown packet type, a packet type the attribute cannot reach, and a value where the name stands alone |
| AC-10 | Done | `TestFullyExcludedRecordIsStillConformant` | Excludes all six and asserts Acct-Status-Type, Acct-Session-Id and the NAS identity survive |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The twelve planned unit tests | Done | `internal/component/l2tp/plugins/authradius/exclude_test.go` and `internal/component/radius/acct_delay_time_omit_test.go` | All in one file per package, not the three the table named |
| `TestTwoExclusionsApplyIndependently` | Done | `exclude_test.go` | Present since the first commit; the table row was missing |
| `radius-acct-exclude`, `radius-acct-exclude-invalid` | Done | `test/l2tp/` | `./le functional l2tp` 23/23 PASS |
| The wire-level `.ci` | Changed | not delivered | Stated as not delivered in the spec's own TDD section, with the reason |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `yang/ze-l2tp-auth-radius-conf.yang` | Done | +188 lines |
| `config.go` | Done | `Exclusions` field and the parse call |
| `acct.go` | Done | `setExclusions`, `exclusionsNow`, the filter point |
| `handler.go` | Done | `attributePolicy`, `setExclusions`, the filter point |
| `docs/guide/l2tp.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md` | Done | `a32367b75` |
| `test/l2tp/radius-acct-exclude.ci` | Done | Plus the invalid twin, which the plan did not name |
| `exclude.go`, `exclude_test.go`, `register.go` | Changed | Created beyond the plan's file list |
| `internal/component/radius/packet.go`, `client.go` | Changed | Needed once A-4 broke |
| `nasportid_test.go` | Changed | The call sites `6bd1653b41` left behind |

### Audit Summary
- **Total items:** 30
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (each recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator can suppress one attribute their server dislikes, per record type | functional | `test/l2tp/radius-acct-exclude.ci` writes three exclusions through the real `OnConfigVerify` path and `./le functional l2tp` passes it, 23/23 on 2026-09-04 |
| The suppression reaches the wire | functional, in process | `TestExcludedAttributeIsAbsentFromTheWire` starts at configuration TEXT, runs the YANG parse, `ToPluginMap`, `parseConfigFromTree` and `buildAcctPacket`, then encodes the packet and walks its octets |
| The enum cannot express a non-conformant record | data correctness | Verified at the schema, not at the test: `container exclude` declares exactly six presence containers, and `acct-status-type`, `acct-session-id`, `nas-identifier` and `nas-ip-address` appear nowhere under it. `acct-terminate-cause`'s `packet-type` leaf-list has one member, `accounting-stop`, per RFC 2866 Section 5.10. Two refusals then back the schema: `parseAttributeExclusions` errors on a name `excludableAttributes` does not hold, and `parseExcludedPacketKinds` errors on an unknown packet type. `TestRequiredAttributesAreNotInTheEnum` and `radius-acct-exclude-invalid.ci` exercise both |
| Exclusion and omit-when-empty stay distinguishable | data correctness | Verified at the producers. `TestExcludeBeatsAKnownValue`'s middle case reaches `radius.AppendTextAttr`, which never appends an empty text and where the exclusion set is nil. Its third case reaches `append` followed by `attributeExclusions.filter`, over a session whose `callingStationID` is `"00:11:22:33:44:55"`. Two different functions decide the two absences, and the test `t.Fatal`s if the fixture value is empty, so the third case cannot degrade into the second |
| An absent container changes nothing | data correctness | `TestNoExclusionsLeavesThePacketUnchanged` compares encoded attribute streams over Start, Interim and Stop. `filter` returns `attrs` unchanged when `len(e) == 0`, and `parseAttributeExclusions` answers a nil map for an absent container, an absent `exclude` and an empty one |
| Discrimination: the feature is what the tests measure | red phase, re-observed at closure | Neutering `attributeExclusions.filter` to `return attrs` reddens exactly eight tests: `TestExcludedAttributeIsAbsentFromTheWire`, `TestExcludeWithNoPacketTypeAppliesEverywhere`, `TestExcludePerPacketType`, `TestExcludeBeatsAKnownValue`, `TestExcludeOnAccessRequestOnly`, `TestTwoExclusionsApplyIndependently`, `TestExcludePreservesTheRemainingOrder`, `TestFullyExcludedRecordIsStillConformant`. Neutering the value refusal in `parseExcludedPacketKinds` reddens exactly one subtest, `TestExcludeRefusesUnknownWords/a_value_where_the_name_stands_alone`. Both were re-run against the code as it stands on 2026-09-04, not taken from the commit message |
| RFC 2866 conformance survives the feature | RFC gate | `TestFullyExcludedRecordIsStillConformant` for Section 5.13, and `TestRFC2866AccountingRetransmitWithoutDelayTimeKeepsIdentifier` for the Section 3 rule the feature made reachable, the second with a discrimination record. `./le rfc check` reports nothing against rfc2865 or rfc2866 |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The metadata table declares no deferral shard, and no `plan/deferrals/radius-attribute-exclusion.md` exists. No row was created and none is resolved here. The neighbouring `plan/deferrals/radius-subscriber-attributes.md` belongs to `spec-radius-subscriber-attributes` and its rows were closed by `spec-radius-acct-session-attributes`, so this closure neither empties it nor removes it |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/radius-attribute-exclusion-f89390ec-889f-4a7a-8172-1e2cfd108a12.md`, 15 files, verdict clean |
| `./le spec session review check` | clean |
| Rounds | 3. Round 3 was earned by a PRODUCT finding: `./le verify lint run` scoped to the two packages reported four issues in this spec's own files |
| Reviewer lenses used | deliverable-versus-test (every Deliverables and TDD cell re-run, never accepted as its own evidence); red-phase re-observation of both recorded neuterings; staled-sibling-claim search derived from the producer name `(*Client).Exchange` and `stampsAcctDelayTime` rather than from the diff; guard and zero-value lens over `parseAttributeExclusions`, `parseExcludedPacketKinds` and `acctPacketKind`; concurrency lens over `exclusionsNow` and the `a.mu` call sites; aliasing lens over `filter`'s in-place write; `docs/contributing/ze-go-style.md` pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The committed tree does not compile the authradius test binary: `6bd1653b41` changed `buildAccessRequestAttrs` to take an `attributePolicy` and left three `string` call sites uncommitted | `internal/component/l2tp/plugins/authradius/nasportid_test.go` | Committing the working-tree fix in commit A, plus a journal row under `tree-state-claim-published-unverified` |
| 2 | ISSUE | `rfc/short/rfc2866.md` claims ze stamps Acct-Delay-Time on EVERY Accounting-Request, and that the identical-contents retransmit is a packet ze no longer produces on the accounting path | `rfc/short/rfc2866.md`, Enrolment reason and Support coverage | Both cells rewritten, `./le rfc index-update` re-rendered the three generated artifacts |
| 3 | ISSUE | The same false sentence sits in the tag prose of an RFC-tagged test, where `./le rfc check` counts it as the proof behind a public compliance claim | `internal/component/radius/rfc2866_accounting_test.go`, `TestRFC2866AccountingRetransmitTakesANewIdentifier` | Comment rewritten. `rfc.ChangedTags` strips comments, so the commit gate computes no tagged-test change and `test/rfc-changed.md` owes no row |
| 4 | ISSUE | RFC 2866 Section 3's identical-contents Identifier rule became reachable on the accounting path and nothing asserted it | `(*radius.Client).Exchange`, the false branch of `stampsAcctDelayTime` | `TestRFC2866AccountingRetransmitWithoutDelayTimeKeepsIdentifier`, tagged `RFC2866-3-3 positive`, with a discrimination record |
| 5 | ISSUE | `rfc/discrimination/rfc2865.json`'s revert record for RFC2865-4.1-1 negative no longer verified: its break was applied to `buildAcctPacket`, which this spec rewrote | `rfc/discrimination/rfc2865.json` | Re-recorded with `./le rfc discriminate-record`, red re-observed |
| 6 | ISSUE | `./le verify lint run` scoped to the two packages reported four findings never run against this diff: three `nilnil` on `parseAttributeExclusions`'s `return nil, nil`, and one `modernize` on `containsType` | `exclude.go`, `exclude_test.go` | The three `nil, nil` returns are correct, because a nil map is a usable value and the doc comment says it names the deployment that holds nothing back, so each carries `//nolint:nilnil` with that reason. `containsType` now calls `slices.Contains`. The scoped lint is 0 issues over both packages and both build flavors |
| 7 | NOTE | The TDD table had no row for AC-6 and named three files that hold no test | this spec | Rows corrected at closure. A record defect, fixed in one edit, earning no extra round |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/l2tp/radius-acct-exclude.ci` | yes | `ls -la` reports 2.0K, 2026-09-04 11:42 |
| `test/l2tp/radius-acct-exclude-invalid.ci` | yes | `ls -la` reports 1.4K, 2026-09-04 11:42 |
| `internal/component/l2tp/plugins/authradius/exclude.go` | yes | `ls -la` reports 8.3K |
| `internal/component/l2tp/plugins/authradius/exclude_test.go` | yes | `ls -la` reports 18K |
| `internal/component/radius/acct_delay_time_omit_test.go` | yes | `ls -la` reports 6.9K |
| `rfc/discrimination/rfc2866.json` | yes | written by `./le rfc discriminate-record`, two records |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | An absent container changes nothing | `go test -race` over the package, green, `TestNoExclusionsLeavesThePacketUnchanged` among the passes |
| AC-2, AC-3, AC-6, AC-7 | Exclusion applies where named and nowhere else, order preserved | the same run; all four reddened by the `filter` neutering, which is what proves they measure the feature |
| AC-4 | An excluded attribute whose value is known is still absent | `TestExcludeBeatsAKnownValue`, and the producer reading in Goal Validation |
| AC-5 | Access-Request only | `TestExcludeOnAccessRequestOnly` builds an Access-Request through `buildAccessRequestAttrs` and then three accounting records |
| AC-8 | The enum names none of the four | read at `yang/ze-l2tp-auth-radius-conf.yang`: six `container` names under `container exclude`, and `ze-radclose schema show ze-l2tp-auth-radius-conf` serves the same six |
| AC-9 | Unknown words refused | `TestExcludeRefusesUnknownWords`, four subtests, each asserting the refusal NAMES the offending word and the permitted ones |
| AC-10 | A fully excluded record is conformant | `TestFullyExcludedRecordIsStillConformant` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `exclude calling-station-id` in the config tree, to the attribute's absence on the wire | `TestExcludedAttributeIsAbsentFromTheWire` in process, and `test/l2tp/radius-acct-exclude.ci` for the config surface | yes. The `.ci` was read, not inferred from its name: it pipes a config carrying three exclusions into `ze config validate -` and requires exit 0 with `configuration valid`. The invalid twin requires exit 1 and `unknown field in exclude: acct-session-id` on stderr. The in-process test starts at configuration text and ends at encoded octets |
| Config reload installs the set on both builders | none | yes, read at `activateRadiusConfig` (`register.go`): `authInstance.setExclusions(cfg.Exclusions)` and `acctInstance.setExclusions(cfg.Exclusions)` |
| The container round-trips | none | yes. `ze config show <file> l2tp auth radius attributes` and `ze config fmt` both re-render `exclude { acct-terminate-cause; calling-station-id { packet-type accounting-interim } }` from a config built for this check |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The six excludable attributes are `0-1` in RFC 2866 Section 5.13 or absent from its table, read in `rfc/full/rfc2866.txt`. AC-10 asserts the consequence |
| A-2 | confirmed | `internal/component/radius/dict.go` defines `AcctStatusStart`, `AcctStatusStop` and `AcctStatusInterimUpdate` and no Acct-On or Acct-Off. `acctPacketKind` switches on exactly those three and answers `packetKindUnspecified` otherwise |
| A-3 | confirmed | `TestExcludePreservesTheRemainingOrder` derives the expected order from an unfiltered packet and requires the filtered one to match it |
| A-4 | **broken** | `buildAcctPacket` never appends Acct-Delay-Time. `(*radius.Client).Exchange` writes it through `setAcctDelayTime`. Mistake Log row 1, Deviations, and Blast Radius all carry it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/l2tp.md` -- the exclusion syntax and what each attribute feeds | The syntax block written there matches the YANG: an `attributes { exclude { ... } }` block beside `server`. `ze config fmt` re-renders the same shape | yes |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` -- the list is filterable, and which three are not | The page names the six excludable attributes and the three that are not, and its Acct-Delay-Time paragraph states the byte-identical retransmit. Both `<!-- source: -->` anchors resolve | yes |
| `docs/config-reference.md` -- regenerated | No. The claim in the checklist is wrong: the file carries no GENERATED header and no generator writes it. Its l2tp section is hand-written prose about `nas-port-id-format` alone, and it names none of `source-address`, `timeout`, `retries`, `acct-interval`, `coa-port` or the `server` list. The page is declared "a concise reference", so an unlisted leaf does not make it wrong. Adding `attributes exclude` beside a section that omits `server` would misstate what the page covers | no edit owed |
| RFC status | `rfc/short/rfc2866.md` Support coverage now names the knob and the three attributes it cannot reach, with `authradius/exclude.go excludableAttributes, attributeExclusions.filter` inside the cell. `./le rfc index-update` re-rendered `docs/features/rfc-status.md`, whose diff is one row, ours | yes |
| Doctor checks | This change adds no runtime dependency: no file path, socket, kernel module, port, binary or certificate. It removes attributes from a packet ze already sends to a server the operator already configured | none owed |
| CLI reference | This change adds no command. `ze schema show ze-l2tp-auth-radius-conf` serves the container, which is the config surface rather than a command surface | none owed |

## Core Insight

The interesting decision was which of Juniper's two naming forms to copy, and
the answer generalises: the curated enum is deliberately LESS capable, and the
capability it gives up is the ability to be wrong quietly. A numeric form accepts
a line that suppresses a mandatory attribute and, in Junos's own words, has "no
effect". The closed enum refuses the word at configuration load, where the
operator can still fix it, so no packet builder ever asks whether an exclusion
was permitted. The guard is structural, and a structural guard cannot be
forgotten by the next person who adds an attribute.

Closure found the cost of that shape. A feature that REMOVES something from a
packet makes a branch reachable that nothing produced before, and the claims that
went stale were all sentences saying "ze always does X". None of them was in the
diff. They were found by searching for the PRODUCER's name.
