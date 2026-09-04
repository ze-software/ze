# Spec: radius-attribute-exclusion

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | plan/spec-radius-acct-session-attributes.md |
| Phase | 1/6 |
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

`plan/spec-radius-acct-session-attributes.md` makes ze emit Calling-Station-Id,
Event-Timestamp, Acct-Delay-Time and Acct-Terminate-Cause unconditionally, on the
owner's ruling to copy Juniper. That ruling is right and it leaves an operator no
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
| `TestNoExclusionsLeavesThePacketUnchanged` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-1 | |
| `TestExcludeWithNoPacketTypeAppliesEverywhere` | same | AC-2 | |
| `TestExcludePerPacketType` | same | AC-3 | |
| `TestExcludeBeatsAKnownValue` | same | AC-4 | |
| `TestExcludeOnAccessRequestOnly` | `.../handler_test.go` | AC-5 | |
| `TestExcludePreservesTheRemainingOrder` | `.../acct_test.go` | AC-7 | |
| `TestRequiredAttributesAreNotInTheEnum` | `.../config_test.go` | AC-8 | |
| `TestExcludeRefusesUnknownWords` | same | AC-9 | |
| `TestFullyExcludedRecordIsStillConformant` | `.../acct_test.go` | AC-10 | |
| `TestExcludedAttributeIsAbsentFromTheWire` | same | Wiring | |
| `TestExcludeAcctDelayTimePerPacketType` | `.../exclude_test.go` | AC-3 for the attribute the client stamps | |
| `TestAccountingRequestOmitsAcctDelayTimeOnRequest` | `internal/component/radius/acct_delay_time_omit_test.go` | AC-2 for Acct-Delay-Time, read off the wire a UDP socket received | |

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
| The schema refuses a required attribute | `TestRequiredAttributesAreNotInTheEnum` | |
| Per-packet-type exclusion | `TestExcludePerPacketType` | |
| Wiring from config to wire | `TestExcludedAttributeIsAbsentFromTheWire` | |
| A fully excluded record is conformant | `TestFullyExcludedRecordIsStillConformant` | |
| Functional proof | `test/l2tp/radius-acct-exclude.ci` | |
| Guide updated | the `docs/guide/l2tp.md` diff | |

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
