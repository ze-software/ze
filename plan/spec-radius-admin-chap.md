# Spec: radius-admin-chap

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `internal/component/radius/authenticator.go` -- `(*radiusAuthenticator).Authenticate`, the PAP path this branches.
3. `internal/component/radius/attr.go` -- `EncodeCHAPPassword`, which builds the wire value and does not compute the digest.
4. `internal/component/radius/config.go` -- `ExtractConfig`, where the new leaf is read.
5. `internal/component/l2tp/plugins/authradius/handler.go` -- `buildAuthAttrs`, the in-tree CHAP attribute reference.

## Task

Ze's RADIUS admin backend sends PAP only. `(*radiusAuthenticator).Authenticate`
(`internal/component/radius/authenticator.go`) puts the operator's password into
a `User-Password` attribute on every Access-Request, hidden per RFC 2865 Section
5.2 with the shared secret. Anyone holding that secret and a captured request
recovers the password, because Section 5.2's hiding is an XOR against
`MD5(secret + Request Authenticator)` and is reversible by construction.

Give the operator the other credential RFC 2865 defines. Ze is the NAS and holds
the plaintext password, so it can generate its own challenge and compute the
response itself: `CHAP-Password` carries `MD5(identifier + password + challenge)`
and the password never reaches the wire in a recoverable form.

**The choice is the operator's, and it is a real one.** RFC 2865 Section 2.2:
"CHAP requires that the user's password be available in cleartext to the server
so that it can encrypt the CHAP challenge and compare that to the CHAP response.
If the password is not available in cleartext to the RADIUS server then the
server MUST send an Access-Reject to the client." A server storing password
hashes can verify PAP and cannot verify CHAP. Selecting CHAP against such a
server rejects every login.

**Design decision (owner, 2026-09-03): a config leaf, `pap` by default.** RFC
2865 Section 4.1 permits either credential, so this is a MAY and
`ai/rules/rfc-compliance.md` puts it to the owner rather than to the
implementer. He chose the config option. An operator who does nothing keeps the
behavior shipped today.

**One semantic point, recorded rather than hidden.** RFC 2865 Section 5.3
describes CHAP-Password as "the response value provided by a PPP
Challenge-Handshake Authentication Protocol (CHAP) user in response to the
challenge", and Section 5.40 describes CHAP-Challenge as "the CHAP Challenge
sent by the NAS to a PPP ... user". Admin login has no PPP peer, so ze produces
both halves. Nothing on the wire differs: the server verifies the same digest
over the same three inputs, and neither section places a MUST on where the
challenge came from. The gain is against a captured request, not against a
hostile server.

## Required Reading

### Architecture Docs
- [x] `docs/guide/radius.md` -- the shipped PAP admin backend, its config surface
  and its chain semantics.
  → Constraint: CHAP must not change the PAP default, the profile-mapping path,
  or the fall-through-on-unreachable behavior that stops an operator lockout.
- [x] `ai/rules/config.md` -- YANG leaf versus env var.
  → Decision: `auth-method` is operator policy that belongs in the running
  config, beside `profile-attribute`, not an env var.
- [x] `ai/patterns/config-option.md` -- structural template for the leaf.
- [x] `docs/research/l2tpv2-ze-integration.md` -- named as the design document by
  `internal/component/radius/config.go`, the file this spec edits.
  → Constraint: it describes the L2TP SUBSCRIBER path, where PAP, CHAP and
  MS-CHAPv2 credentials come from the PPP peer. It says nothing about admin
  login, so it constrains this spec only by keeping the two paths separate.

### RFC Summaries (Scope: protocol)
- [x] RFC 2865 Section 2.2 -- the cleartext-at-the-server requirement quoted in
  the Task.
  → Constraint: the leaf's YANG description states this, so an operator reads
  the cost before selecting it.
- [x] RFC 2865 Section 4.1 -- "An Access-Request MUST contain a User-Password or
  a CHAP-Password or a State. An Access-Request MUST NOT contain both a
  User-Password and a CHAP-Password."
  → Constraint: the two credentials are one switch with one arm each. The
  existing `rfc/short/rfc2865.md` row `RFC2865-4.1-4` is marked
  `{single-polarity: positive}` on exactly that reasoning, and the CHAP branch
  must preserve it.
- [x] RFC 2865 Section 5.3 -- CHAP-Password: Type 3, Length 19, one octet CHAP
  Ident then 16 octets of CHAP Response.
  → Constraint: the value is 17 octets. `EncodeCHAPPassword` already builds it.
- [x] RFC 2865 Section 5.40 -- CHAP-Challenge, attribute 60. "If the CHAP
  challenge value is 16 octets long it MAY be placed in the Request
  Authenticator field instead of using this attribute."
  → Decision: ze does NOT take that MAY. The Request Authenticator already
  carries the reply-verification role the client checks on the way back
  (`verifyResponseMessageAuthenticator`), so overloading it would tie two
  independent jobs to one field. The challenge goes in attribute 60.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/radius/authenticator.go` -- `(*radiusAuthenticator).Authenticate`
  builds a fixed four-attribute list: `User-Password` from `request.Password`,
  `Service-Type = Login`, `NAS-Identifier`, then `User-Name` through
  `AppendTextAttr` and `NAS-IP-Address` when a source address is set. It calls
  `RandomAuthenticator` once for the Request Authenticator and
  `(*Client).SendToServers` once.
- [x] `internal/component/radius/attr.go` -- `EncodeCHAPPassword` writes the
  identifier into the first octet and copies the response after it. It builds
  the wire value only. It does NOT compute the digest, and no MD5 over
  identifier, password and challenge exists anywhere in the package.
- [x] `internal/component/radius/dict.go` -- `AttrCHAPPassword` and
  `AttrCHAPChallenge` already exist, at 3 and 60.
- [x] `internal/component/radius/config.go` -- `ExtractedConfig` carries
  `Servers`, `Timeout`, `Retries`, `SourceAddress`, `ProfileAttr` and
  `DefaultProfiles`. `ExtractConfig` mirrors the YANG defaults.
- [x] `internal/component/radius/doctor.go` -- `radiusAdminReachable` sends a
  probe Access-Request carrying a fixed `User-Password` of `ze-doctor`.
- [x] `internal/component/l2tp/plugins/authradius/handler.go` -- `buildAuthAttrs`
  appends `AttrCHAPPassword` with `EncodeCHAPPassword` and `AttrCHAPChallenge`
  with the peer's challenge. There the PPP peer supplies both; here ze produces
  both. That is the only structural difference.

**Behavior to preserve:** the PAP attribute list byte for byte when the leaf is
absent or `pap`; profile mapping; the Access-Accept, Access-Reject,
Access-Challenge and unreachable-server branches; the L2TP subscriber path,
which is untouched.

**Behavior to change:** the credential attribute the Access-Request carries,
selected by one new config leaf.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An operator SSH or web login. The plaintext password arrives in
`authz.AuthRequest`, built by `(*Server).authenticatePasswordResult`
(`internal/component/ssh/passwordauth.go`) and by the web Basic-auth and
form-login sites (`internal/component/web/auth.go`), and reaches
`(*radiusAuthenticator).Authenticate` unchanged through
`aaa.ChainAuthenticator.Authenticate`.

MCP is not on this path: nothing under `internal/component/mcp/` imports
`component/aaa` or `component/authz`.

### Transformation Path
1. `ExtractConfig` reads `auth-method` into `ExtractedConfig.AuthMethod`,
   defaulting to PAP.
2. `radiusBackend.Build` carries the method into the authenticator.
3. `Authenticate` builds the credential attributes for the selected method:
   PAP appends `User-Password`; CHAP draws a 16-octet challenge and a one-octet
   identifier from `crypto/rand`, computes `MD5(identifier || password ||
   challenge)`, and appends `CHAP-Password` and `CHAP-Challenge`.
4. Everything after the credential is unchanged: the same Service-Type,
   NAS-Identifier, User-Name and NAS-IP-Address, the same `SendToServers`, the
   same reply handling.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → method selection | `auth-method` leaf read by `ExtractConfig` | [ ] |
| Backend build → authenticator | `ExtractedConfig.AuthMethod` carried by `radiusBackend.Build` | [ ] |
| Authenticator → RADIUS server | CHAP-Password + CHAP-Challenge on the Access-Request, and no User-Password | [ ] |

### Integration Points
- `internal/component/radius/yang/ze-radius-conf.yang` -- the `auth-method` leaf.
- `internal/component/radius/config.go` -- `ExtractedConfig`, `ExtractConfig`.
- `internal/component/radius/authenticator.go` -- the credential branch.
- `internal/component/radius/aaa.go` -- `radiusBackend.Build` passing the method.

### Architectural Verification
The method is a typed value, not a string, so an unknown method cannot reach the
authenticator as data. The YANG enum refuses an unknown word at config load, and
the Go type has two values. No central switch gains a case: the credential
builder lives beside the authenticator that uses it.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze holds the operator's plaintext password when `Authenticate` runs | `(*radiusAuthenticator).Authenticate` puts `request.Password` straight into an `AttrUserPassword` value | The whole spec is N/A | read at the producer | confirmed |
| A-2 | `EncodeCHAPPassword` builds the 17-octet value and nothing else, so the digest is new code | `EncodeCHAPPassword` writes the identifier then copies the response | Less new code than planned | read at the producer | confirmed |
| A-3 | The doctor probe does not need the method, because a reject still proves reachability and a correct shared secret | `radiusAdminReachable` verifies the Response Authenticator, not the verdict | The probe would false-negative on a CHAP-only server | AC-7 | confirmed by `TestRadiusAdminDoctorProbeStaysPap`, which drives `radiusAdminReachable` against a server answering Access-Reject |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator selects CHAP against a server storing hashes and locks out every RADIUS login | Every login rejects with the server answering | The YANG description quotes RFC 2865 Section 2.2; the operator's local bcrypt account is a separate credential and still works |
| R-2 | The challenge or identifier comes from a weak source | none at runtime | `crypto/rand` only, and a generation failure returns an error rather than a zero challenge |
| R-3 | Both credentials reach one packet, violating Section 4.1 | none at runtime | The builder returns one credential set, and AC-4 asserts the absence of the other |

## Blast Radius

`internal/component/radius` only. The L2TP subscriber path has its own CHAP
assembly and is not touched. The AAA chain, the profile mapping and the SSH,
web and API surfaces see no signature change: the authenticator gains a field,
not a new method.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `auth-method chap` in the config tree | → | the Access-Request the client sends carries CHAP-Password | `TestRadiusAdminChapReachesTheWire` |

The wiring test drives the config tree, not the builder, so a leaf that parses
and never reaches the authenticator fails it.

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `auth-method` absent | The Access-Request is byte-identical to today's: `User-Password`, no `CHAP-Password`, no `CHAP-Challenge` |
| AC-2 | `auth-method pap` | Same as AC-1 |
| AC-3 | `auth-method chap` | The Access-Request carries `CHAP-Password` (Type 3, Length 19) and `CHAP-Challenge` (Type 60, 16 octets) |
| AC-4 | `auth-method chap` | The Access-Request carries NO `User-Password`, per RFC 2865 Section 4.1 |
| AC-5 | `auth-method chap`, known password and challenge | The CHAP Response is `MD5(identifier \|\| password \|\| challenge)`, asserted against a hex vector computed independently of the producer |
| AC-6 | `auth-method chap`, two successive logins | The challenge and the identifier differ between them |
| AC-7 | `auth-method chap` | `ze doctor` still probes with the fixed PAP credential and still reports reachable against a server that answers, because the probe tests reachability and the shared secret, not the verdict |
| AC-8 | `auth-method` set to an unknown word | The config is refused at load, naming the leaf and the permitted values |
| AC-9 | `crypto/rand` fails | `Authenticate` returns an error and sends nothing; no zero challenge and no zero identifier reach the wire |
| AC-10 | `auth-method chap`, Access-Accept with Filter-Id | Profiles map exactly as the PAP path maps them, and the session is tagged `source=radius` |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `auth-method chap` and logs in over SSH | login → CHAP Access-Request → Access-Accept → profiles | `test/plugin/aaa-radius-chap.ci` |
| 2 | Changes nothing and logs in over SSH | login → PAP Access-Request → Access-Accept → profiles | `test/plugin/aaa-radius-admin.ci`, unchanged and still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRadiusAdminChapAttributes` | `internal/component/radius/authenticator_test.go` | AC-3, AC-4 | green |
| `TestRadiusAdminChapResponseDigest` | `internal/component/radius/chap_test.go` | AC-5 | green |
| `TestRadiusAdminChapCredentialShape` | `internal/component/radius/chap_test.go` | boundary widths | green |
| `TestRadiusAdminChapChallengeIsFreshPerLogin` | `internal/component/radius/authenticator_test.go` | AC-6 | green |
| `TestRadiusAdminPapIsTheDefault` | `internal/component/radius/authenticator_test.go` | AC-1, AC-2 | green |
| `TestRadiusAdminChapProfileMapping` | `internal/component/radius/authenticator_test.go` | AC-10 | green |
| `TestRadiusAdminUnknownAuthMethodSendsNothing` | `internal/component/radius/authenticator_test.go` | the switch's default guard | green |
| `TestRadiusAdminChapFailsClosedOnRandomError` | `internal/component/radius/chap_test.go` | AC-9 | green |
| `TestRadiusAdminDoctorProbeStaysPap` | `internal/component/radius/doctor_test.go` | AC-7 | green |
| `TestExtractConfigAuthMethod` | `internal/component/radius/config_test.go` | AC-8, extraction side | green |
| `TestRadiusAuthMethodEnum` | `internal/component/config/radius_auth_method_enum_test.go` | AC-8, schema side | green |
| `TestAdminAccessRequestCarriesExactlyOneCredential` | `internal/component/radius/rfc2865_walk_test.go` | RFC2865-4.1-3, RFC2865-4.1-4, both arms | green |
| `TestRadiusAdminChapReachesTheWire` | `internal/component/radius/aaa_test.go` | Wiring | green |

### Boundary Tests (numeric inputs)
| Input | Boundary | Expected |
|-------|----------|----------|
| CHAP-Password value length | 17 octets exactly | attribute Length is 19 |
| CHAP-Challenge value length | 16 octets | attribute Length is 18 |
| password of length 0 | empty credential | the digest is over identifier and challenge alone; the request is still well-formed and the server rejects it |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-chap` | `test/plugin/aaa-radius-chap.ci` | CHAP admin login accepted, log shows `source=radius` | green, and RED under the reverted branch |

### Interop Tests (Scope: protocol)
| Scenario | Peer implementation | Asserts |
|----------|--------------------|---------|
| `radius-admin-chap-freeradius` | FreeRADIUS with a cleartext password | A real server verifies ze's digest and returns Access-Accept, which is the claim a mock cannot make |

## Files to Modify
- `internal/component/radius/yang/ze-radius-conf.yang` -- the `auth-method` leaf.
- `internal/component/radius/config.go` -- `ExtractedConfig.AuthMethod`, read in `ExtractConfig`.
- `internal/component/radius/authenticator.go` -- the credential branch.
- `internal/component/radius/aaa.go` -- carry the method into the authenticator.
- `docs/guide/radius.md` -- the leaf, the tradeoff, and the "PAP only" note.
- `docs/config-reference.md` -- regenerated, not hand-edited.

## Files to Create
- `internal/component/radius/chap.go` -- the digest and the challenge source.
- `internal/component/radius/chap_test.go`.
- `test/plugin/aaa-radius-chap.ci`.
- `test/interop/scenarios/radius-admin-chap-freeradius/`.

### Integration Checklist
- [ ] The leaf appears in the CLI schema and completes.
- [ ] `ze config dump` round-trips a config carrying `auth-method chap`.
- [ ] The authenticator's new field has a non-test caller.

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/guide/radius.md` -- the `auth-method` row in the leaf table, the
      RFC 2865 Section 2.2 cost, and the "PAP only in the MVP" operational note
      corrected.
- [ ] `docs/features.md` -- the RADIUS row, if it spells the method.
- [ ] `docs/features/rfc-status.md` -- driven by `rfc/short/rfc2865.md`, so
      regenerate rather than edit.
- [ ] `ai/CODE-TO-DOCS.md` / `ai/DOCS-TO-CODE.md` -- regenerated for the new file.

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring first.** Add the leaf, the typed method, the field, and a
   test that drives the config tree and asserts CHAP-Password on the wire. Prove
   it RED before the branch exists.
2. **Phase: The digest.** `chap.go`: the challenge source and
   `MD5(identifier || password || challenge)`, with an independent hex vector.
3. **Phase: The branch.** Replace the fixed credential attribute with the
   selected one. One credential, never both.
4. **Phase: Failure paths.** The `crypto/rand` error path, and the unknown-enum
   refusal.
5. **Phase: Functional and interop.** The `.ci` and the FreeRADIUS scenario.
6. **Phase: Docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | The digest order is identifier, password, challenge, in that order |
| Fail closed | A random-source failure returns an error, never a zero challenge |
| Section 4.1 | Exactly one credential attribute per request, asserted by absence |
| PAP untouched | The default path's attribute list is unchanged |
| L2TP untouched | No change under `internal/component/l2tp/plugins/authradius/` |

### Deliverables Checklist
| Deliverable | Verification method | Status |
|-------------|--------------------|--------|
| `auth-method` leaf | `ze config schema` shows the enum | |
| CHAP credential on the wire | `TestRadiusAdminChapAttributes` | |
| Digest correctness | `TestRadiusAdminChapResponseDigest` against an independent vector | |
| Wiring from config to wire | `TestRadiusAdminChapReachesTheWire` | |
| Functional proof | `test/plugin/aaa-radius-chap.ci` | |
| Interop proof | `radius-admin-chap-freeradius` scenario | |
| Guide updated | `docs/guide/radius.md` diff | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret handling | The shared secret and the password are never logged, and the password does not appear in an error string |
| Challenge quality | `crypto/rand` only; no `math/rand`, no time-derived seed, no reuse across logins |
| Fail closed | A random-source error aborts the login rather than sending a predictable challenge |
| Credential exclusivity | No path can emit both User-Password and CHAP-Password |
| Downgrade | Nothing at runtime can move a configured CHAP back to PAP |

### Failure Routing
| Failure | Route |
|---------|-------|
| `crypto/rand` error | Return an error from `Authenticate`; the chain treats it as infrastructure and tries the next backend, so an operator is not locked out |
| Unknown `auth-method` word | Refused at config load by the YANG enum |
| Server rejects every CHAP login | Operator-visible Access-Reject; the guide names RFC 2865 Section 2.2 as the cause |

## Design Insights

The transport was never the obstacle. The skeleton's open question assumed CHAP
needs a PPP peer to challenge; it needs a password, and ze has one. What the
choice actually costs is storage at the server, which is why it is a leaf and
not a default.

## Key Design Decisions

| Decision | Why | What it forecloses |
|----------|-----|--------------------|
| Config leaf, `pap` default | RFC 2865 Section 4.1 permits either credential, so the owner decides (`ai/rules/rfc-compliance.md`); most servers store hashes | An operator must opt in to get the wire benefit |
| Challenge in attribute 60, not the Request Authenticator | Section 5.40's placement is a MAY, and the Request Authenticator already carries reply verification | Two octets more per request |
| The doctor probe stays PAP | It tests reachability and the shared secret, and an Access-Reject answers both | A CHAP-only server that refuses to answer a PAP request at all would read as unreachable |
| A typed method, not a string | An unknown method cannot reach the authenticator as data | Nothing |

## Known Limitations

- CHAP requires the RADIUS server to hold the password in cleartext (RFC 2865
  Section 2.2). This is the operator's tradeoff, stated in the guide.
- MS-CHAP and MS-CHAPv2 are not in scope. They are RFC 2433 and RFC 2759 with
  vendor-specific attributes, and neither RFC is enrolled.

## RFC Documentation (Scope: protocol)
- RFC 2865 Sections 2.2, 4.1, 5.3 and 5.40.
- **The extraction was NOT re-run, and the CHAP obligations that bind ze already
  carry ids.** The two are `RFC2865-4.1-3` ("An Access-Request MUST contain
  either a User-Password or a CHAP-Password or a State") and `RFC2865-4.1-4`
  ("An Access-Request MUST NOT contain both"). Sections 5.3 and 5.40 state a
  wire format rather than an obligation, and Section 2.2's one MUST addresses
  the RADIUS server, which ze is not on this path.
  `TestAdminAccessRequestCarriesExactlyOneCredential` now drives both arms and
  carries both tags, and each has a fresh discrimination record in
  `rfc/discrimination/rfc2865.json` under a break of
  `(*radiusAuthenticator).credential`.
- The YANG enum refusal is tagged `RFC7950-9.6-1` in both polarities, with
  records in `rfc/discrimination/rfc7950.json` under a break of
  `validateEnumeration`.
- `rfc/requirements/rfc2865.md`, `rfc/requirements/rfc7950.md` and
  `ai/RFC-REQUIREMENTS.md` are stale against these new tags.
  `./le rfc index-update` regenerates all of them at once and would fold in
  three other sessions' in-flight rows, so it was NOT run here.

## Implementation Notes

**Where the AC-5 vector came from.** `TestRadiusAdminChapResponseDigest` asserts
`f30a3da4592d46dfe5358518dc83b689` for identifier `0x16`, password `Hello` and
challenge `01..10`. It was computed twice outside this package and outside Go:
by `python3 -c "hashlib.md5(...)"` and by `printf ... | openssl dgst -md5`. Both
agree, so the test cannot agree with a wrong producer.

**How the random source is injectable.** `radiusAuthenticator` carries one
`random io.Reader` field, set to `crypto/rand.Reader` in
`newRadiusAuthenticator`. It is never nil, so there is no default-on-nil branch,
and a test in the same package assigns it to force the failure. No option, no
interface, no constructor parameter: the smallest shape that lets AC-9 drive the
failure from `Authenticate` rather than from the helper.

**The interop scenario is NOT built, and the directory was deliberately not
created.** `test/interop/scenarios/` belongs to the BGP suite:
`interoplab.Discover` (`internal/le/interoplab/discover.go`) joins every
directory there against the BGP checker registry and returns
`scenario <name> has no Go checker` for one that is missing, so an empty
`radius-admin-chap-freeradius/` would break `./le integration interop` outright.
The repository holds no FreeRADIUS lab of any kind: no `Dockerfile.freeradius`,
no RADIUS package under `internal/le/interoplab/`, and no `./le integration`
verb for it. `plan/future/spec-radius-admin-interop-freeradius.md` (skeleton,
2026-07-08) exists for that build and calls it "a distinct infrastructure
build". The nearest honest proof is `test/plugin/aaa-radius-chap.ci`, which
drives a real SSH login through the production wire path against a server that
computes the digest itself from the cleartext password and rejects a request
carrying both credentials.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] The producer of the password was read, not inferred.
- [x] The RFC text was read in `rfc/full/rfc2865.txt`, not in the summary.
- [x] The MAY was put to the owner and he answered.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Wiring test RED before the branch, GREEN after
- [ ] Interop scenario passes against FreeRADIUS
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Deferral rows naming this spec resolved
- [ ] Citations repointed

---

## Implementation Summary

### What Was Implemented
- `auth-method` enum leaf on `system/authentication/radius`
  (`internal/component/radius/yang/ze-radius-conf.yang`), default `pap`, read by
  `ExtractConfig` into `ExtractedConfig.AuthMethod` through `parseAuthMethod`
  (`internal/component/radius/config.go`).
- `AuthMethod`, a typed `uint8` whose zero value is `AuthMethodPAP`, with one
  `authMethodNames` map that both `String` and `parseAuthMethod` read, so the
  two can never name different sets.
- `(*radiusAuthenticator).credential` (`internal/component/radius/authenticator.go`),
  which SELECTS one credential set and never appends, and refuses an unnamed
  method rather than defaulting it.
- `chapCredential` and `chapResponse` (`internal/component/radius/chap.go`):
  one 17-octet read from the injected `random io.Reader` supplies the CHAP
  Identifier and the 16-octet challenge, and the response is
  `MD5(identifier || password || challenge)`.
- `radiusAuthenticator.random`, set to `crypto/rand.Reader` in
  `newRadiusAuthenticator`. It is never nil, so there is no default-on-nil
  branch, and a same-package test assigns it to force AC-9.
- The functional fixture `extra1RadiusChap`
  (`internal/test/fixture/plugin_shell_extra_1_auth.go`) and its `.ci`
  (`test/plugin/aaa-radius-chap.ci`).
- `internal/test/mock/radius/radius.go` verifies EITHER credential and REJECTS a
  request carrying both, which is what makes the fixture able to fail a builder
  that appends instead of selecting.

### Bugs Found/Fixed
- None in the product code. One documentation defect this change itself caused
  was found by the Review Gate and fixed at closure: see Documentation Updates.

### Documentation Updates
- `docs/guide/radius.md` -- the `auth-method` row, the PAP-versus-CHAP table,
  and the RFC 2865 Section 2.2 cost. Committed in `0971cff50d`. Its later hunks
  in the working tree belong to the EAP and interop sessions and are NOT part of
  this closure.
- `docs/features.md` -- the RADIUS authentication row. Committed in `0971cff50d`.
- `rfc/short/rfc2865.md`, `| Support remaining |` -- FIXED AT CLOSURE. The row
  read "Operator/admin login RADIUS (PAP)", which this change made false. It now
  names the `auth-method` leaf and its producer,
  `(*radiusAuthenticator).credential`, rather than enumerating credentials that
  go stale again.
- `docs/features/rfc-status.md` -- regenerated by `./le rfc index-update` from
  that Meta row. Its RFC 3579 line also moves, because `rfc/short/rfc3579.md` is
  already committed and the generated page was lagging behind it; no
  uncommitted source of any other session is carried.
- `docs/config-reference.md` -- NO update owed. `grep -n -i radius
  docs/config-reference.md` returns only L2TP and PPPoE hits; the page never
  described `system/authentication/radius`.
- `./le doc check verify` -- exit 1, and `grep -i "radius\|chap"` over its log
  returns nothing. Every finding is CLI-reference drift in other sessions'
  surfaces.

### Deviations from Plan
- The interop scenario was NOT built by this spec. It is now built and green, by
  `spec-radius-admin-interop-freeradius` (commit `2bea01249`), and the
  deliverable is satisfied there. See Goal Validation.
- `docs/config-reference.md` was listed under Files to Modify and needs no edit,
  with the grep above as evidence.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Documentation Update Checklist listed `docs/features/rfc-status.md` as "driven by `rfc/short/rfc2865.md`, so regenerate rather than edit", and the implementation regenerated nothing and edited nothing | The Meta row of `rfc/short/rfc2865.md` was itself stale: its `Support remaining` cell said the admin backend was PAP, which this change falsified. Regenerating an unedited source changes nothing, so the checklist row was answered by a no-op | The Review Gate's documentation-drift lens, round 1, reading the Meta table rather than the generated page | Fixed in commit A. Journal row in `plan/journal/comment-describes-superseded-behaviour.md`. The lesson: a checklist row saying "regenerate" is answered only after the SOURCE it regenerates from has been re-read against the change |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Give the operator RFC 2865's other credential | Done | `chapCredential`, `internal/component/radius/chap.go` | CHAP-Password (3) plus CHAP-Challenge (60) |
| The choice is the operator's, `pap` by default | Done | `leaf auth-method`, `internal/component/radius/yang/ze-radius-conf.yang`, `default "pap"` | The zero value of `AuthMethod` is `AuthMethodPAP`, so an absent leaf and an explicit `pap` are one path |
| The password never reaches the wire recoverably under `chap` | Done | `chapResponse`, `internal/component/radius/chap.go` | Only the digest and the challenge are sent; the password never leaves the process |
| One credential, never both (RFC 2865 Section 4.1) | Done | `(*radiusAuthenticator).credential`, `internal/component/radius/authenticator.go` | A `switch` that returns; it has no append |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRadiusAdminPapIsTheDefault` | PASS |
| AC-2 | Done | `TestRadiusAdminPapIsTheDefault` | PASS |
| AC-3 | Done | `TestRadiusAdminChapAttributes`, `TestRadiusAdminChapCredentialShape` | PASS |
| AC-4 | Done | `TestRadiusAdminChapAttributes`, `TestAdminAccessRequestCarriesExactlyOneCredential` | PASS |
| AC-5 | Done | `TestRadiusAdminChapResponseDigest` | PASS; the two vectors were recomputed at closure, outside Go |
| AC-6 | Done | `TestRadiusAdminChapChallengeIsFreshPerLogin` | PASS |
| AC-7 | Done | `TestRadiusAdminDoctorProbeStaysPap` | PASS |
| AC-8 | Done | `TestExtractConfigAuthMethod`, `TestRadiusAuthMethodEnum` | PASS on both the Go and the schema side |
| AC-9 | Done | `TestRadiusAdminChapFailsClosedOnRandomError` | PASS; both a read error and a truncated source, asserted at `Authenticate` and by `srv.noRequest` |
| AC-10 | Done | `TestRadiusAdminChapProfileMapping` | PASS |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The twelve unit tests of the TDD table | Done | `internal/component/radius/`, `internal/component/config/` | All twelve PASS; see Pre-Commit Verification |
| `aaa-radius-chap` | Done | `test/plugin/aaa-radius-chap.ci` | PASS in `./le functional plugin` at closure |
| `radius-admin-chap-freeradius` | Done | `test/interop-radius/scenarios/radius-admin-chap-freeradius/` | Built and green by `spec-radius-admin-interop-freeradius`, commit `2bea01249` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/radius/yang/ze-radius-conf.yang` | Done | leaf added |
| `internal/component/radius/config.go` | Done | `AuthMethod`, `parseAuthMethod`, read in `ExtractConfig` |
| `internal/component/radius/authenticator.go` | Done | `credential`, `random` |
| `internal/component/radius/aaa.go` | Done | `radiusBackend.Build` carries the method |
| `internal/component/radius/chap.go`, `chap_test.go` | Done | created |
| `test/plugin/aaa-radius-chap.ci` | Done | created |
| `docs/guide/radius.md` | Done | committed in `0971cff50d` |
| `docs/config-reference.md` | Changed | No edit owed; the page never covered this surface |
| `test/interop/scenarios/radius-admin-chap-freeradius/` | Changed | Built at `test/interop-radius/scenarios/radius-admin-chap-freeradius/` instead: `test/interop/scenarios/` belongs to the BGP suite and `interoplab.Discover` refuses a directory there with no BGP checker |

### Audit Summary
- **Total items:** 27
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2, both recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A captured Access-Request no longer yields the operator password to a holder of the shared secret | functional | `test/plugin/aaa-radius-chap.ci`: a real SSH login where the mock records `method=chap` and the fixture FAILS if `method=pap` or `method=both` appears. PASS in `./le functional plugin` at closure |
| A server ze did not write reproduces ze's digest | interop | `radius-admin-chap-freeradius`, against `docker.io/freeradius/freeradius-server:3.2.7` with `radiusop Cleartext-Password := "fixture-chap-secret"`. `checkCHAP` calling `checkAcceptedLogin` (`internal/le/interoplab/radius/checkers.go`) asserts the login succeeds AND that the server's own linelog line reads `user-password=absent chap-password=present`. `PASS 3 scenario(s)`, recorded in `spec-radius-admin-interop-freeradius`, with the break of `(*ChainAuthenticator).Authenticate` reddening it at assertion 5 |
| The operator cost RFC 2865 Section 2.2 states is real and documented | interop | `radius-admin-chap-hashed-freeradius`: the same fixture password stored `{sha256}` is accepted over PAP by `radclient` and REFUSED over CHAP. `checkCHAPHashed` (`internal/le/interoplab/radius/checkers.go`) asserts the PAP probe first, so the storage form is the only thing left to explain the rejection. That is the cost `docs/guide/radius.md` tells the operator `auth-method chap` carries |
| An operator who changes nothing keeps today's bytes | functional | `test/plugin/aaa-radius-admin.ci` unchanged and PASS at closure, and `TestRadiusAdminPapIsTheDefault` |
| RFC 2865 Section 4.1 is not violated | RFC-tagged unit | `TestAdminAccessRequestCarriesExactlyOneCredential`, tagged `RFC2865-4.1-3 positive` and `RFC2865-4.1-4 positive`, each with a discrimination record under a break of `credential` in `rfc/discrimination/rfc2865.json`, verified by `./le rfc discriminate stem rfc2865` at closure |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec metadata declares no deferral shard, and no file under `plan/deferrals/` carries this stem. There is nothing to remove |
| The interop scenario, deferred by `0971cff50d` to a named future spec | done | Built and green: `spec-radius-admin-interop-freeradius`, commit `2bea01249`. That spec is still open and owns its own closure |
| `plan/verification-debt/5db0ba0d.md`, two rows for this commit | deferred | Both say "full native verification over this commit's Go". `./le verify current mode full` cannot exit 0 over this tree: `internal/component/l2tp` does not compile (another agent's `TerminateCause` refactor, call sites unpassed) and `./le repository check` reports 5 unwired exports in `bgp/reactor` and `config/yang/cli`. The rows stay open for whoever clears that shard; they are not this spec's to clear, and `./le commit debt-clear` needs the gate at 0 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/radius-admin-chap-f89390ec-889f-4a7a-8172-1e2cfd108a12.md` (5 files, verdict=clean) |
| `./le spec session review check` | `review_gate: OK (0 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 found one ISSUE, the stale `rfc/short/rfc2865.md` Meta row. Round 2 clean over the fix |
| Reviewer lenses used | wiring and logic (steps 2 and 14), security and the fail-closed guard audit (step 12 and the guard audit of step 14), protocol/RFC with interop and documentation drift (steps 4, 20 and 21) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The public RFC 2865 status row says operator/admin login RADIUS is "(PAP)". This change added a second credential, so the row is false, and it is the row `docs/features/rfc-status.md` publishes | `rfc/short/rfc2865.md`, the `Support remaining` row of the `## Meta` table | Rewrote the cell to name the `auth-method` leaf and its producer `(*radiusAuthenticator).credential` rather than a credential list, then ran `./le rfc index-update` |

Round 1 also confirmed, from source, that no BLOCKER was present. `credential`
is reached from `radiusBackend.Build` (`internal/component/radius/aaa.go`) and
driven end to end by `TestRadiusAdminChapReachesTheWire`. The random-source
failure returns before `exchange` is called (`(*radiusAuthenticator).Authenticate`),
so the guard fails closed and `srv.noRequest` proves nothing was sent.
`grep -rn "math/rand" internal/component/radius/` returns nothing. `a.method` is
assigned nowhere outside `newRadiusAuthenticator`, so no runtime path downgrades
a configured CHAP to PAP. No `log`, `slog` or error string in `authenticator.go`
or `chap.go` carries the password or the shared secret. `./le commit audit` over
the worktree reports no `[DELETED]` and no `[WEAKENED]` finding under
`internal/component/radius`, so this closure needs no row in
`test/rfc-changed.md`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/radius/chap.go` | yes | read in full at closure; defines `chapCredential` and `chapResponse` |
| `internal/component/radius/chap_test.go` | yes | read in full at closure |
| `test/plugin/aaa-radius-chap.ci` | yes | read in full at closure; four `expect=` lines |
| `test/interop-radius/scenarios/radius-admin-chap-freeradius/users` and `ze.conf` | yes | both read at closure; the config carries `auth-method chap`, the users file `Cleartext-Password` |
| `test/interop-radius/scenarios/radius-admin-chap-hashed-freeradius/users` and `ze.conf` | yes | both read at closure; the users file carries `Password-With-Header := "{sha256}..."` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | An absent or `pap` leaf keeps today's bytes | `--- PASS: TestRadiusAdminPapIsTheDefault (0.00s)` |
| AC-3 | CHAP-Password and CHAP-Challenge on the wire | `--- PASS: TestRadiusAdminChapAttributes`, `--- PASS: TestRadiusAdminChapCredentialShape` |
| AC-4 | No User-Password under `chap` | `--- PASS: TestAdminAccessRequestCarriesExactlyOneCredential` |
| AC-5 | The response is `MD5(identifier, password, challenge)` | `--- PASS: TestRadiusAdminChapResponseDigest`, and the two vectors recomputed at closure outside Go: identifier `0x16` with `Hello` gives `f30a3da4592d46dfe5358518dc83b689`, and with an empty password `df14f6a05c60dac0a2a48cf28431301d`, both matching the file |
| AC-6 | A fresh challenge and identifier per login | `--- PASS: TestRadiusAdminChapChallengeIsFreshPerLogin` |
| AC-7 | The doctor probe stays PAP | `--- PASS: TestRadiusAdminDoctorProbeStaysPap` |
| AC-8 | An unknown word is refused at load | `--- PASS: TestExtractConfigAuthMethod`, `--- PASS: TestRadiusAuthMethodEnum` |
| AC-9 | A random-source failure sends nothing | `--- PASS: TestRadiusAdminChapFailsClosedOnRandomError (0.60s)`, both sub-sources |
| AC-10 | Profiles map as on the PAP path | `--- PASS: TestRadiusAdminChapProfileMapping` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `auth-method chap` in the config tree, reaching CHAP-Password on the Access-Request | `test/plugin/aaa-radius-chap.ci` | yes. `./le functional plugin` at closure prints `7.0s 2/729 PASS 2 aaa-radius-chap`, beside `1/729 PASS 1 aaa-radius-admin` and `3/729 PASS 3 aaa-radius-fallback`. The fixture drives a real SSH login and FAILS if the server logs `method=pap` or `method=both` |
| The same, against a server ze did not write | `radius-admin-chap-freeradius` | yes. `PASS 3 scenario(s)` from `./le integration interop-radius` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `(*radiusAuthenticator).credential` receives `request.Password` from `Authenticate` (`internal/component/radius/authenticator.go`), read at the producer |
| A-2 | confirmed | `chap.go` holds the whole digest; `EncodeCHAPPassword` is called with the response already computed |
| A-3 | confirmed | `TestRadiusAdminDoctorProbeStaysPap` PASS |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/radius.md`, the `auth-method` row and the credential table | committed in `0971cff50d`; the surviving working-tree hunks belong to the EAP and interop sessions | yes |
| `docs/features.md`, the RADIUS authentication row | committed in `0971cff50d` | yes |
| `rfc/short/rfc2865.md` `Support remaining`, and the regenerated `docs/features/rfc-status.md` | the cell now names `(*radiusAuthenticator).credential`, `internal/component/radius/authenticator.go`, read at that producer | yes, fixed at closure |
| `docs/config-reference.md` | `grep -n -i radius docs/config-reference.md` returns five hits, all L2TP or PPPoE; the page never described `system/authentication/radius` | yes, no update owed |
| `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` | `git status --porcelain` reports both unmodified, and neither index carries a row for any file in `internal/component/radius` | yes, no update owed |
| `./le doc check verify` | exit 1; `grep -i "radius\|chap"` over its log returns nothing. Every finding is CLI-reference drift in other sessions' surfaces | yes, no finding of this spec's |

## Core Insight

CHAP for admin login needed no PPP peer, only a password and a random source.
What the operator actually pays for is not on ze's side at all: it is the
storage form at the server, and RFC 2865 Section 2.2 makes it a hard refusal
rather than a degradation. That is why `radius-admin-chap-hashed-freeradius`
exists. A scenario proving CHAP WORKS proves half the feature. The scenario
proving it is REFUSED against a hashed store is what proves the guide tells the
operator the truth about the cost.
