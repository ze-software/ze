# Spec: radius-blastradius-message-authenticator

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol |
| Priority | P0 |
| Release reason | Security fix in shipped code: Ze's RADIUS client accepts a forged Access-Accept (BlastRADIUS, CVE-2024-3596) on every PAP, CHAP and MS-CHAP login, admin and subscriber alike |
| RFC gaps | - |
| Depends | - |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The gaps are in draft-ietf-radext-deprecating-radius-10 Sections 4.1, 4.2,
4.6 and 4.8 (client half). The draft has no `rfc/short/` summary and no release
profile lists it, so the RFC gaps row stays `-`.

BlastRADIUS (CVE-2024-3596) lets an on-path attacker turn an Access-Reject into
an Access-Accept. The attacker injects a Proxy-State into an Access-Request
that carries no integrity check, then uses an MD5 chosen-prefix collision so the
Response Authenticator of the forged Accept still verifies. The defence the IETF
standardises has two halves on the client: sign every Access-Request with a
Message-Authenticator, and refuse every response to an Access-Request that
carries none.

Ze's RADIUS client (`internal/component/radius`) does neither for non-EAP
logins. Only the EAP builder adds a Message-Authenticator, so PAP, CHAP and
MS-CHAP Access-Requests go out unsigned, and `verifyResponseMessageAuthenticator`
accepts a non-EAP Access-Accept, Access-Reject or Access-Challenge with no
Message-Authenticator. Both clients built on it are exposed: the operator login
backend (`system authentication radius`) and the L2TP/PPPoE subscriber backend
(`l2tp auth radius`).

Goal: every Access-Request Ze sends carries a Message-Authenticator as its
first attribute, and by default Ze discards any response to an Access-Request
that carries no Message-Authenticator, with a per-server leaf to accept a legacy
server, a rate-limited log line and a counter on every discard.

## Required Reading

### Normative source
- [ ] `rfc/drafts/draft-ietf-radext-deprecating-radius-10.txt` - the IETF text
  that standardises the mitigation (RADEXT WG document, Standards Track
  intended, updates RFC 2865, 2866, 5176 and 7585, rev 10 of 3 July 2026,
  fetched for this spec; not yet an RFC per the IETF datatracker, state "Active
  / WG Document")
  → Constraint (Section 4.2): "Clients MUST add Message-Authenticator to all
  Access-Request packets." and "This behavior MUST NOT be configurable."
  → Constraint (Section 4.2): "The Message-Authenticator SHOULD be the first
  attribute in all Access-Request packets."
  → Constraint (Section 4.1): "Clients MUST have a per-server boolean
  configuration flag, which we call “require Message-Authenticator”." The same
  section: "The default value of all three configuration flags SHOULD be
  "false"" and "Implementers and Vendors MAY choose to set the default value for
  these values to "true" instead".
  → Constraint (Section 4.6): "When receiving any response to an Access-Request
  packet (Access- Accept, Access-Challenge, Access-Reject, or Protocol-Error),
  clients MUST consult the "require Message authenticator" flag prior to
  accepting the packet for processing.  This flag MUST NOT be consulted for
  responses to other types of request packets."
  → Constraint (Section 4.6): "Response packets which do not contain
  Message-Authenticator MUST be silently discarded.  This check MUST be done
  before the Response Authenticator or Message- Authenticator has been
  verified."
  → Constraint (Section 4.6): "Clients MUST NOT discard a packet based on the
  location of the Message-Authenticator attribute."
  → Constraint (Section 4.6): "any retransmission timers MUST NOT be modified as
  a result of receiving a packet which is silently discarded." and "The client
  SHOULD log the fact that the packet was discarded (with rate limits)".
  → Constraint (Section 4.3): the server flag "MUST NOT be consulted for other
  types of request packets", so the BlastRADIUS flags govern Access-Request
  only, never CoA-Request or Disconnect-Request.
  → Constraint (Section 4.7): Status-Server answers follow the same rules; Ze
  sends no Status-Server (no `CodeStatusServer` anywhere under `internal/`), so
  nothing there changes.

### RFC Summaries
- [ ] `rfc/short/rfc2869.md` - Section 5.14, Message-Authenticator verification
  on a response; rows stay, the verifier is reused.
- [ ] `rfc/short/rfc3579.md` - Section 3.1 (EAP answer without MA discarded),
  3.2 (signing), 3.3 (no MA in Accounting-Request); all three stay true.
- [ ] `rfc/short/rfc5176.md` - Section 3.4, Message-Authenticator on CoA is a
  MAY; its text names the `require-message-authenticator` leaf this spec
  renames.
  → Constraint: no summary is written for the draft. No release profile lists
  it (profiles are declared by each summary's `Use cases` and `Support` fields,
  and no `rfc/short/` summary exists for it), and `ai/META-DECISIONS.md` forbids
  enrolling one no profile lists. Code comments quote the draft by name and
  section.

### Reference implementations
- [ ] accel-ppp-ng `accel-pppd/radius/` commits dbefbdf, 762017f, 09d7b71,
  02c3ceb, 6322e9b (T9351)
  → Decision: accel-ppp signs with the Message-Authenticator placed first
  (`PACKET_SIGNED_OFFSET (20 + 2)` in packet.c) and re-signs with the failover
  server's secret (`refresh_packet_secret`, req.c, 09d7b71). Its two switches
  (`message-authenticator-include-access-request`,
  `message-authenticator-require-access-response`) are global and default 0.
  Ze takes the placement and the per-server secret, and departs on the two
  switches because the draft makes signing non-configurable and asks for a
  per-server require flag.
- [ ] FreeRADIUS v3.2.x `raddb/radiusd.conf.in` and `raddb/proxy.conf`
  → Decision: FreeRADIUS ships `require_message_authenticator = yes` as the
  global default (radiusd.conf.in, `security` section), overridable per
  `home_server` with `yes`, `no` or `auto`. Its own text on `auto`: "This
  switch is done for the first packet received from that client or home
  server." Ze ships `true` and no `auto` (see Key Design Decisions).

### Architecture Docs
- [ ] `docs/guide/radius.md` - the admin backend page; its Message-Authenticator
  paragraph (lines 188-196) and its test tables describe EAP-only signing.
- [ ] `docs/guide/l2tp.md` - the subscriber backend page; lines 770-790 document
  the CoA `require-message-authenticator` leaf this spec renames.
- [ ] `docs/architecture/testing/interop.md` - the FreeRADIUS suite and its
  scenarios.
  → Constraint: a new scenario directory is named, no numeric prefix.

**Key insights:**
- Signing already re-derives per server: `SendToServers` calls `Exchange` with
  that server's `SharedKey`, and `encodeRequest` signs with the secret it is
  handed. accel-ppp's failover bug does not exist in Ze, and a test keeps it so.
- `dispatchResponse` serves Access and Accounting answers alike, so the waiter
  must know the request code to keep the flag off Accounting-Response.
- The relay `test/interop-radius/unsigned-relay.pl` already strips type 80 from
  any answer, EAP or not, so the PAP negative interop scenario needs no new
  tool.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/radius/client.go` (584L) - `Server{Address,
  SharedKey}`; `Exchange(ctx, pkt, secret, serverAddr)` encodes once, replays an
  Access-Request byte for byte, re-encodes an Accounting-Request per attempt;
  `encodeRequest` refuses EAP-Message/MA in Accounting-Request, signs only when a
  placeholder is present, refuses an unsigned EAP request; `dispatchResponse`
  runs `VerifyResponseAuth` first, then `verifyResponseMessageAuthenticator`,
  logs every discard at Warn with no rate limit; `SendToServers` fails over with
  a fresh Identifier and Request Authenticator per server.
- [ ] `internal/component/radius/packet.go` (541L) -
  `verifyResponseMessageAuthenticator` returns Accepted for a response with no MA
  unless it carries EAP-Message; `SignMessageAuthenticator` signs a placeholder
  wherever it sits; `messageAuthenticatorValueOffset` walks the attributes.
- [ ] `internal/component/radius/authenticator_eap.go` line 239 - the EAP builder
  appends the MA placeholder LAST.
- [ ] `internal/component/radius/metrics.go` - one counter,
  `ze_radius_unsigned_eap_discards_total`.
- [ ] `internal/component/radius/config.go`, `yang/ze-radius-conf.yang` -
  `system authentication radius server <address>` with `port`, `key`.
- [ ] `internal/component/radius/doctor.go` - `radiusAdminReachable` probes with
  `Exchange`; code `doctor-radius-admin-unreachable`.
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `admitRequest`
  verifies the Request Authenticator, then discards a request without MA only
  when `RequireMessageAuthenticator`; verifies an MA when present.
  `sendResponse` sends ACK/NAK without MA.
- [ ] `internal/component/l2tp/plugins/authradius/config.go`, `register.go`,
  `doctor.go`, `yang/ze-l2tp-auth-radius-conf.yang` - container-level leaf
  `require-message-authenticator` (default false) governs CoA only; `list server
  <name>` holds `address`, `port`, `accounting-port`, `shared-key`; the doctor
  probe calls `Exchange`.
- [ ] `internal/test/mock/radius/radius.go`, `eap.go` - `buildResponse` signs no
  non-EAP answer; only EAP answers carry an MA.
- [ ] `test/interop-radius/clients.conf`, `test/interop-radius-subscriber/clients.conf`
  - both set `require_message_authenticator = no`, the first with a comment that
  "ze does not send one on the admin path".

**Behavior to preserve:**
- An Access-Request retransmission to the same server is byte-identical (RFC
  2865 Section 2.5; `TestAccessRequestRetransmitIsByteIdentical`).
- Failover gets a new Identifier, a new Request Authenticator, and the
  User-Password and Message-Authenticator under that server's own secret.
- Accounting-Request carries no Message-Authenticator (RFC 3579 Section 3.3),
  and an Accounting-Response is never judged by the new flag.
- An EAP answer without MA is discarded and counted on
  `ze_radius_unsigned_eap_discards_total` whatever the flag says.
- A present MA that does not verify is discarded (RFC 2869 Section 5.14).
- Admin login falls through to local accounts when no server gives a usable
  answer.
- CoA/Disconnect admission order and verdicts.

**Behavior to change:**
- Every Access-Request carries a Message-Authenticator as its first attribute.
- A response to an Access-Request without MA is discarded when the server's
  `require-message-authenticator` is true, which is the default.
- The CoA leaf `l2tp auth radius require-message-authenticator` is renamed
  `coa-require-message-authenticator`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Outbound: an operator SSH/web login (`internal/component/radius/authenticator.go`
  via `SendToServers`) or a PPP credential (`authradius/handler.go` via
  `SendToServers`), and the two doctor probes (via `Exchange`).
- Inbound: a UDP datagram on the client socket, read by `readAnswers`, handed to
  `dispatchResponse` keyed by server and Identifier.
- Config: `system authentication radius server <address>
  require-message-authenticator` and `l2tp auth radius server <name>
  require-message-authenticator`.

### Transformation Path
1. Config extraction (`radius.ExtractConfig`, `authradius` config) builds each
   `radius.Server` with its require flag.
2. `Exchange` takes the `Server`; `encodeRequest` writes the header, the
   18-octet Message-Authenticator placeholder at offset 20 for an
   Access-Request, then the attributes, signs with that server's secret.
3. `registerWaiter` records the request code and the server's require flag.
4. `dispatchResponse`, per waiter: when the request was an Access-Request and
   the flag is true, a datagram with no MA is discarded BEFORE
   `VerifyResponseAuth`; then the existing Response Authenticator and MA checks
   run unchanged.
5. A discard increments a counter, logs through a per-server rate limiter, and
   marks the exchange so a final timeout is reported as "answered unsigned"
   rather than "unreachable".

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree to radius client | `ExtractConfig` / authradius config read the new leaf into `Server` | No |
| Ze to RADIUS server | UDP datagram, MA first | No |
| Client to doctor | sentinel error `ErrUnsignedResponse` wrapped by `Exchange`, read by both doctor checks | No |

### Integration Points
- `radius.Server` gains `AllowUnsignedResponse` (zero value requires the MA);
  `Exchange` takes a `Server` instead of `(secret, serverAddr)`, so every caller
  passes the flag with the secret it already passes.
- `SignMessageAuthenticator` and `verifyResponseMessageAuthenticator` are reused,
  not duplicated.
- `internal/core/diagnostic/codes.go` registers the two new doctor codes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | all three exits (`SendToServers`, two doctor probes) reach the socket through `Exchange` and `encodeRequest` |
| No unintended coupling | Yes | authradius already imports `radius`; nothing new crosses |
| No duplicated functionality | Yes | the EAP builder stops adding its own placeholder; `encodeRequest` is the one place that adds it |
| Zero-copy preserved where applicable | Yes | the placeholder is written into the pooled `Bufs` buffer at offset 20; no attribute slice is copied to prepend it |
| Registration over hardcoding, outbound | Yes | new doctor codes register in `codes.go` the way the existing radius codes do |
| Registration over hardcoding, inbound | Yes | the flag is read from `Server`; no code switch learns a server name. Searched: `CodeAccessRequest` consumers in `client.go`, `encodeRequest`, `dispatchResponse` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | FreeRADIUS 3.2.10 signs every Access-Accept, Access-Reject and Access-Challenge over UDP | `internal/le/interoplab/radius/check_unsigned_eap.go` header (rad_encode in release_3_2_10) | the default-true leaf breaks every existing interop scenario | existing PAP/CHAP interop scenarios pass with the leaf at its default | unvalidated |
| A-2 | FreeRADIUS 3.2.10 with `require_message_authenticator = yes` drops an Access-Request without MA | radiusd.conf.in text, "yes" | the clients.conf flip proves nothing | revert signing, run `radius-admin-pap-freeradius`, see it fail | unvalidated |
| A-3 | `unsigned-relay.pl` in strip mode handles a PAP Access-Accept | relay walks any attribute list and drops type 80 (lines 69-90) | new scenario needs a relay change | the passthrough leg of the new scenario completes | unvalidated |
| A-4 | Ze sends no Status-Server | grep `CodeStatusServer` / `Status-Server` under `internal/`: no producer | Section 4.7 owes work | the same grep at implementation (2026-10-28: no non-test hit) | confirmed |
| A-5 | No operator config uses the CoA leaf name outside the repo | Ze is pre-release (`ai/META-DECISIONS.md`) | none | - | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Unit and functional tests whose fake server answers unsigned turn red under the default | package tests fail with timeouts | fake servers sign like an updated server (MA first); tests that prove the legacy path set the leaf false explicitly; RFC-tagged test edits go to `ze-test-judge` |
| R-2 | An operator with a pre-2024 server sees every login fail | doctor and log | distinct log line and doctor code naming the exact leaf to set; `docs/guide/radius.md` and `docs/guide/l2tp.md` say it first |
| R-3 | The log line floods under attack | discard rate | per-server limiter: one Warn per 60 s per server, next line reports the suppressed count |
| R-4 | MA-first breaks the EAP path's existing signing tests that assumed the last position | `rfc3579_*` tests red | tests assert presence and validity, not position, unless the AC is the position |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | RADIUS logins fail (admin falls through to local; subscribers refused), or the hole stays open |
| How is it reverted? | single commit revert; the leaf is new, the renamed CoA leaf reverts with it |
| Who else touches this path? | `spec-eap-tls-identity-authorization.md` (EAP over the same client); check `./le spec current` holders before editing `authenticator_eap.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `system authentication radius server X` PAP login | → | `encodeRequest` MA first, `dispatchResponse` presence check | `test/plugin/aaa-radius-unsigned-accept-refused.ci` |
| `system authentication radius server X require-message-authenticator false` | → | `ExtractConfig` → `Server.AllowUnsignedResponse` | `test/plugin/aaa-radius-unsigned-accept-legacy.ci` |
| `l2tp auth radius server X require-message-authenticator` | → | authradius config → `Server` → `SendToServers` | `TestL2TPRadiusConfigRequireMessageAuthenticator` |
| `l2tp auth radius coa-require-message-authenticator true` | → | `coaListener.admitRequest` | `TestCoARequireMessageAuthenticatorLeafRenamed` |
| `ze doctor` with an unsigned server | → | `radiusAdminReachable` / `udpServerReachable` read `ErrUnsignedResponse` | `TestDoctorRadiusAdminUnsignedResponse`, `TestDoctorRadiusUnsignedResponse` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any Access-Request Ze sends (PAP, CHAP, MS-CHAP, EAP, doctor probe) | Octets 20-21 are type 80, length 18, and the HMAC-MD5 over the packet with the value zeroed, keyed by that server's secret, equals octets 22-37 |
| AC-2 | A caller hands `Exchange` an Access-Request that already carries a Message-Authenticator | Refused with an error naming RFC 2869 Section 5.14 "more than one"; nothing sent |
| AC-3 | Accounting-Request | Carries no Message-Authenticator (unchanged, RFC 3579 Section 3.3) |
| AC-4 | Failover from server A (secret a) to server B (secret b) | B receives an MA that verifies under b and not under a; A's retransmissions are byte-identical |
| AC-5 | require true (default), Access-Accept with valid Response Authenticator, no MA | Discarded before `VerifyResponseAuth` runs; login does not succeed; `ze_radius_unsigned_response_discards_total` +1; one Warn naming the server, the reason and the leaf |
| AC-6 | require true, unsigned Access-Reject, Access-Challenge, or Protocol-Error (code 52) | Each discarded the same way |
| AC-7 | require true, unsigned forged Accept followed by the real signed answer inside the timeout | Real answer accepted; the retransmission timer was not reset by the discard (the request is not resent earlier or later than without the forged datagram) |
| AC-8 | require false, unsigned non-EAP Access-Accept with valid Response Authenticator | Accepted (legacy behavior) |
| AC-9 | require false or true, unsigned answer carrying EAP-Message | Discarded and counted on `ze_radius_unsigned_eap_discards_total` (unchanged) |
| AC-10 | Signed answer whose MA is not the first attribute | Accepted when it verifies (draft Section 4.6, no discard on location) |
| AC-11 | Signed answer whose MA does not verify | Discarded (unchanged) |
| AC-12 | Accounting-Response without MA, require true | Accepted (flag not consulted for other request types) |
| AC-13 | 1000 unsigned answers from one server in under 60 s | At most one Warn line in that window; the counter reads 1000 |
| AC-14 | Every retry to a server exhausted and at least one answer was discarded as unsigned | `Exchange` returns an error wrapping `ErrUnsignedResponse`, whose text names the server, says the answer carried no Message-Authenticator, and gives the leaf to set |
| AC-15 | `ze doctor` against a server that answers only unsigned | Admin: `doctor-radius-admin-unsigned-response`; l2tp: `doctor-radius-unsigned-response`; each message carries the exact `set` command for that server's leaf |
| AC-16 | Config omits the leaf | `require-message-authenticator` reads true in both trees |
| AC-17 | `l2tp auth radius require-message-authenticator` in a config | Refused as unknown; `coa-require-message-authenticator` behaves as the old leaf did |
| AC-18 | FreeRADIUS 3.2.10 with `require_message_authenticator = yes` | PAP, CHAP, EAP admin logins and the subscriber PPP logins complete |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator logs in over SSH with RADIUS PAP against a server that requires MA | SSH → aaa chain → `Authenticator` → `SendToServers` → FreeRADIUS → Accept | `radius-admin-pap-freeradius` with clients.conf `yes` |
| 2 | Attacker strips/forges an unsigned Accept | relay strips MA → `dispatchResponse` discards → login refused, log, counter | `radius-admin-pap-unsigned-freeradius` |
| 3 | Operator with a legacy server sets the leaf false | config → `Server` → unsigned Accept accepted | `aaa-radius-unsigned-accept-legacy.ci` |
| 4 | Subscriber PPP login against FreeRADIUS requiring MA | PPP → authradius handler → `SendToServers` → Accept | radius subscriber lab with its clients.conf `yes` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAccessRequestCarriesMessageAuthenticatorFirst` | `internal/component/radius/blastradius_client_test.go` | AC-1, for PAP, CHAP, MS-CHAP and EAP builders | |
| `TestAccessRequestRefusesCallerMessageAuthenticator` | same | AC-2 | |
| `TestFailoverSignsWithEachServerSecret` | same | AC-4 | |
| `TestUnsignedAcceptDiscardedByDefault` | same | AC-5: forged unsigned Accept with a correct Response Authenticator is refused; the negative test the owner asked for | |
| `TestUnsignedResponseCodesDiscarded` | same | AC-6, table over Reject, Challenge, Protocol-Error | |
| `TestPresenceCheckRunsBeforeResponseAuth` | same | AC-5 ordering: an unsigned datagram with a WRONG Response Authenticator is counted as unsigned, not as a bad authenticator | |
| `TestDiscardDoesNotMoveRetransmitTimer` | same | AC-7 | |
| `TestUnsignedAcceptAcceptedWhenNotRequired` | same | AC-8 | |
| `TestMessageAuthenticatorLocationIgnored` | same | AC-10 | |
| `TestAccountingResponseNotJudgedByRequireFlag` | same | AC-12 | |
| `TestUnsignedDiscardLogIsRateLimited` | same | AC-13 | |
| `TestExchangeReportsUnsignedResponse` | same | AC-14, `errors.Is(err, ErrUnsignedResponse)` and message text | |
| `TestExtractConfigRequireMessageAuthenticatorDefault` | `internal/component/radius/rfc2865_config_test.go` | AC-16 admin tree | |
| `TestL2TPRadiusConfigRequireMessageAuthenticator` | `internal/component/l2tp/plugins/authradius/config_test.go` | AC-16 l2tp tree, per server | |
| `TestCoARequireMessageAuthenticatorLeafRenamed` | `internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go` | AC-17 | |
| `TestDoctorRadiusAdminUnsignedResponse` | `internal/component/radius/doctor_test.go` | AC-15 admin | |
| `TestDoctorRadiusUnsignedResponse` | `internal/component/l2tp/plugins/authradius/doctor_test.go` | AC-15 l2tp | |
| existing EAP and signing tests | `internal/component/radius/rfc3579_*_test.go`, `authenticator_eap_test.go`, `rfc2869_message_authenticator*_test.go` | AC-9, AC-11 still hold with MA first | |

Fake servers in existing unit tests that answer an Access-Request unsigned are
changed to sign as an updated server does (MA first). A test whose point is the
legacy path sets `AllowUnsignedResponse: true` explicitly. Any change to
an RFC-tagged test goes to an independent `ze-test-judge` before it lands, and
each tagged unit whose claim changes gets a fresh `./le rfc record`.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none: the feature adds a boolean leaf only | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-unsigned-accept-refused` | `test/plugin/aaa-radius-unsigned-accept-refused.ci` | mock in legacy mode answers Accept unsigned; Ze refuses the RADIUS login, falls to local, logs the leaf name | |
| `aaa-radius-unsigned-accept-legacy` | `test/plugin/aaa-radius-unsigned-accept-legacy.ci` | same mock, leaf false; login succeeds | |
| existing `aaa-radius-admin`, `aaa-radius-chap`, `aaa-radius-eap`, `aaa-radius-fallback`, `reload-aaa-radius-secret-rotation`, l2tp radius `.ci` | `test/plugin/`, `test/reload/`, `test/l2tp/` | pass against a mock that now requires MA on every Access-Request and signs every answer | |

The mock (`le mock radius`, `internal/test/mock/radius`) gains two flags:
`-require-message-authenticator` (default true: discard an Access-Request with
no valid MA, as FreeRADIUS's default does) and `-sign-responses` (default true:
MA first in every Access-* answer; false is the legacy server). The in-process
servers in `internal/test/fixture/tunnel_fixture_l2tp_radius.go` and
`plugin_shell_extra_1_auth.go` sign the same way.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| all existing `radius-admin-*-freeradius` | `test/interop-radius/scenarios/` | FreeRADIUS 3.2.10, clients.conf `require_message_authenticator = yes` | AC-1, AC-18: Ze signs every Access-Request in a form FreeRADIUS verifies. Red phase: revert the signing, scenario fails | |
| `radius-admin-pap-unsigned-freeradius` | `test/interop-radius/scenarios/` | FreeRADIUS 3.2.10 behind `unsigned-relay.pl` | AC-5: passthrough leg completes the PAP login; strip leg is refused, logged, counted; third leg with leaf false completes. Red phase: make the presence check a no-op, strip leg goes green login | |
| radius subscriber lab scenarios | `test/interop-radius-subscriber/` | FreeRADIUS 3.2.10, clients.conf `yes` | AC-18 on the l2tp path | |

## Files to Modify
- `internal/component/radius/client.go` - `Server.AllowUnsignedResponse`; `Exchange(ctx, pkt, srv Server)`; `encodeRequest` writes the MA placeholder first for every Access-Request and refuses a caller-supplied one; waiter records request code and flag; `dispatchResponse` presence check first; rate-limited discard log; `ErrUnsignedResponse`
- `internal/component/radius/packet.go` - comment of `verifyResponseMessageAuthenticator` (the "passes unless EAP" sentence becomes "the caller's require flag decides first")
- `internal/component/radius/authenticator_eap.go` - drop the appended placeholder; the comment points at `encodeRequest`
- `internal/component/radius/metrics.go` - `ze_radius_unsigned_response_discards_total`
- `internal/component/radius/config.go`, `yang/ze-radius-conf.yang` - per-server leaf, default true
- `internal/component/radius/doctor.go` - unsigned-response diagnostic
- `internal/component/l2tp/plugins/authradius/config.go`, `register.go`, `doctor.go`, `coa.go`, `yang/ze-l2tp-auth-radius-conf.yang` - per-server leaf; CoA leaf and field renamed
- `internal/core/diagnostic/codes.go` - `doctor-radius-admin-unsigned-response`, `doctor-radius-unsigned-response`
- `internal/test/mock/radius/radius.go`, `eap.go` - sign answers MA first, two flags
- `internal/test/fixture/tunnel_fixture_l2tp_radius.go`, `internal/test/fixture/plugin_shell_extra_1_auth.go` - sign answers
- `test/interop-radius/clients.conf`, `test/interop-radius-subscriber/clients.conf` - `require_message_authenticator = yes`, comment rewritten
- `internal/le/interoplab/radius/checkers.go`, `register_catalog.go` - checker for the new scenario
- every `*_test.go` calling `Exchange` (53 call sites) - new signature
- `rfc/short/rfc5176.md` - leaf name
- `internal/le/site/testdata/published-configuration.md`, `published-yang-config-tree.json` - regenerated

## Files to Create
- `internal/component/radius/blastradius_client_test.go`
- `test/plugin/aaa-radius-unsigned-accept-refused.ci`, `test/plugin/aaa-radius-unsigned-accept-legacy.ci`
- `test/interop-radius/scenarios/radius-admin-pap-unsigned-freeradius/` (`ze.conf`, `users`)
- `internal/le/interoplab/radius/check_unsigned_accept.go`
- `rfc/drafts/draft-ietf-radext-deprecating-radius-10.txt` (already fetched by this spec, untracked)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | both YANG modules, leaf in `list server` |
| YANG validation constraints | Yes | `type boolean; default true;` |
| YANG custom validators | No | boolean needs none |
| CLI commands/flags | No | config leaf only |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | Yes | automatic for boolean |
| Functional test for new RPC/API | N-A | no RPC; config covered by the two `.ci` |
| Pipe completeness | N-A | no output command |
| Env var registration | No | per-server leaf; not an env-tunable (`ai/rules/config.md` exceptions do not apply) |
| Doctor check for runtime dependencies | Yes | two codes in `codes.go`, unit tests above |
| Prometheus counters/metrics | Yes | `ze_radius_unsigned_response_discards_total{server}` |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature or scope changed? | Yes | `features/` RADIUS entry, if one names Message-Authenticator scope (`./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/guide/radius.md`, `docs/guide/l2tp.md`, `docs/config-reference.md` |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it lists the CoA leaf |
| 6 | Has a user guide page? | Yes | `docs/guide/radius.md` (lines 129, 188-196, test tables 349-390), `docs/guide/l2tp.md` (lines 689, 770-790), `docs/guide/pppoe.md` if it names the leaf |
| 7 | Wire format changed? | Yes | Access-Request attribute order in `docs/guide/radius.md` |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior changed or newly proven? | Yes | `rfc/short/rfc5176.md` leaf name; no summary for the draft (not profile-listed) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (mock flags), `docs/architecture/testing/ci-format.md` (`le mock radius`), `docs/architecture/testing/interop.md` (new scenario, clients.conf) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` RADIUS row, if it states signing |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys | No | - |
| 14 | Prometheus counters added? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered inventory changed? | No | - |
| 16 | Source anchors | DERIVED | `./le spec citation anchors spec plan/release/fixes/spec-radius-blastradius-message-authenticator.md`. Design docs declared by the changed files' `// Design:` headers, each read and unaffected because none mentions Message-Authenticator, the response check or the leaf: `docs/research/l2tpv2-ze-integration.md` (radius and authradius files), `docs/features/ai-first.md` and `ai/rules/repo-maintenance.md` (declared by `internal/core/diagnostic/codes.go` and authradius files; the new codes follow the existing radius code pattern) |
| 17 | Existing examples | Yes | every `require-message-authenticator` example in `docs/guide/l2tp.md` |

## Implementation Steps

1. **Phase: Wiring** - `Server.AllowUnsignedResponse`, the two YANG leaves,
   both config readers, the CoA rename, `Exchange(ctx, pkt, srv)` across all
   callers; wiring tests red on behavior, green on reach.
2. **Phase: Sign every Access-Request** - MA first in `encodeRequest`, EAP builder
   placeholder removed; mock and fixtures verify and sign; AC-1..AC-4.
3. **Phase: Require on responses** - waiter carries code and flag, presence check
   first, counter, limiter, `ErrUnsignedResponse`; AC-5..AC-14.
4. **Phase: Doctor** - two codes; AC-15.
5. **Phase: Functional and interop** - two `.ci`, clients.conf flips, new scenario
   with recorded red phase; AC-17, AC-18.
6. **Phase: Docs** - every row of the Documentation checklist.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the presence check precedes `VerifyResponseAuth`; Accounting-Response never judged; one MA per packet |
| Correctness | a discard leaves the waiter's timer untouched |
| Naming | leaf `require-message-authenticator` per server in both trees; `coa-require-message-authenticator` beside `coa-port`; Go field names match |
| Rule: user-facing-errors | log, error and doctor text each say what, why, and the exact `set` command |
| Rule: rfc-compliance | each MUST quoted from the draft by name and section, byte-exact |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| MA first on every Access-Request | `TestAccessRequestCarriesMessageAuthenticatorFirst` |
| Forged unsigned Accept refused | `TestUnsignedAcceptDiscardedByDefault`, `radius-admin-pap-unsigned-freeradius` |
| Leaf in both trees, default true | `grep -n "require-message-authenticator" internal/component/radius/yang internal/component/l2tp/plugins/authradius/yang` |
| FreeRADIUS requiring MA accepts Ze | `./le test integration` radius scenarios with clients.conf `yes` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | an unparseable attribute list is a discard, never an accept |
| Ordering | no code path hands an unsigned answer to the waiter before the check |
| Flood | log limiter per server, counter unbounded only in value |
| Secret | the MA is keyed by the secret of the server the datagram goes to |
| Fallback | admin fall-through to local on refusal grants nothing without local credentials |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Blocker

Closure (/ze-close, 2026-10-28) stopped before the Review Gate record and the
closing commits. One deliverable has no evidence:

| What blocks | Producer | What clears it |
|-------------|----------|----------------|
| Interop Tests row 1 owes a red phase: "revert the signing, scenario fails". It was never run, so A-2 (FreeRADIUS 3.2.10 with `require_message_authenticator = yes` drops an Access-Request without a Message-Authenticator) is still `unvalidated`, and the claim in `docs/architecture/testing/interop.md` that every scenario "also proves that FreeRADIUS verifies the signature ze computed" has no recorded red | `internal/component/radius/client.go` `encodeRequest`, the `insertMessageAuthenticatorFirst` call under `pkt.Code == CodeAccessRequest` | Skip that call, let the harness rebuild the lab image, run `./le test integration interop-radius` scenario `radius-admin-pap-freeradius`: expect red with FreeRADIUS dropping the request; restore, green. Record it in interop.md beside the clients.conf paragraph, set A-2 confirmed, then rerun /ze-close |

Closure review round 1, already landed (the next closure re-reads, it does not redo):
- ISSUE, fixed in 3dd07256c9: the `ze-test l2tp scale` responder (`internal/test/cli/cmd_l2tp_scale.go` `handlePacket`, also driven by `internal/le/test/deployment/l2tpscale.go`) answered Access-Request unsigned, so every scale-run login was discarded. It now signs; `TestMockRADIUSAnswersZesDefaultClient` is red without the signing and green restored.
- ISSUE, fixed in 3dd07256c9: `docs/guide/l2tp.md` never named the per-server `require-message-authenticator` leaf or the BlastRADIUS discard (risk R-2).
- ISSUE, fixed in 3fa8b278a9 and 8bc5e1c3c3 (verdicts f6a0a90142, b607fceb53): the RFC2865-4.2-1 negative lost isolation once mock answers were signed.
- NOTE: `SendToServers` ends with "radius: all servers unreachable" when every server answered unsigned; the per-server warning carries the real cause.
- NOTE: the rate-limited discard warning names the leaf but not the full `set <row>` command, because the waiter knows only the address; `ErrUnsignedResponse` and the doctor text carry it.
- NOTE: the authradius doctor reports an unparsable `require-message-authenticator` under `doctor-radius-unreachable`; YANG refuses such a value at commit.
- Environment, not code: `radius-coa-disconnect-freeradius` fails on this host because the running kernel lacks CONFIG_NET_ACT_POLICE, which `gokrazy/kernel/runtime.config` carries since 80175d768d.
- Fresh evidence 2026-10-28: `go test -race` of radius, authradius and mock/radius ok; `./le rfc check` shows nothing on rfc2865, rfc2866, rfc3579, rfc5176; `./le repo compiles check` OK after 3dd07256c9; `./le doc check all` red only on a foreign `show policy test peer` wiki drift.

## Design Insights

- The draft scopes every BlastRADIUS flag to Access-Request and its responses
  (Sections 4.3 and 4.6, "MUST NOT be consulted for other types"), so the CoA
  listener owes nothing new here: RFC 5176 Section 3.4 keeps MA a MAY, and Ze
  already offers the stricter leaf.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

- **The `Server` field is `AllowUnsignedResponse`, not `RequireMessageAuthenticator`.**
  A `Server` built in code without the flag must get the secure behavior, so the
  zero value requires the Message-Authenticator. The config readers set
  `AllowUnsignedResponse = !require-message-authenticator`.
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Sign every Access-Request, not configurable | accel-ppp's `include-access-request` switch, default off | Draft Section 4.2: "This behavior MUST NOT be configurable." |
| MA placed first | append last (current EAP builder) | Draft Section 4.2 SHOULD first; accel-ppp `PACKET_SIGNED_OFFSET (20 + 2)`; one rule for EAP and non-EAP |
| `encodeRequest` owns the placeholder; caller-supplied MA refused | builders add it | one declaration; RFC 2869 Section 5.14 allows one MA per packet |
| Per-server boolean leaf `require-message-authenticator` | global only; global plus per-server; enum yes/no/auto | Draft Section 4.1 MUST is per-server boolean; the global setting is a RECOMMENDED convenience with nothing a per-server default does not cover (`ai/rules/simplicity.md`); name matches the draft and FreeRADIUS `home_server` |
| Default true | draft SHOULD false | Draft Section 4.1 "MAY choose to set the default value ... to "true""; FreeRADIUS ships `require_message_authenticator = yes`; Ze is pre-release with no legacy deployment, and default false leaves the hole the spec exists to close |
| No `auto` learning mode | FreeRADIUS `auto` | not in the draft; it learns from the first answer, which is the attacker's forged one when the attack runs first, and it is per-process state that does not survive restart (FreeRADIUS's own warning) |
| Presence check per waiter, before `VerifyResponseAuth` | after both verifications | Draft Section 4.6: "This check MUST be done before the Response Authenticator or Message- Authenticator has been verified." |
| No separate failover re-sign step | accel-ppp's `refresh_packet_secret` | Ze already re-encodes per server through `Exchange(srv)`; AC-4 holds it |
| `Exchange(ctx, pkt, srv Server)` | extra bool parameter; new method | the flag belongs with the address and secret it describes; no-layering forbids keeping the old signature beside a new one |
| Rename CoA leaf to `coa-require-message-authenticator` | keep both under one name at two levels | `l2tp auth radius require-message-authenticator` would read as the response protection and leave it off; the CoA leaf sits beside `coa-port`; pre-release, no migration |
| CoA server: no change in behavior | require MA on CoA by default; sign ACK/NAK | Draft Section 4.3 confines the server flag to Access-Request; RFC 5176 Section 3.4 is a MAY; the leaf exists for operators who want it |
| No draft summary enrolled | write `rfc/short/draft-ietf-radext-deprecating-radius.md` | no profile lists it; `ai/META-DECISIONS.md` forbids growing the validation scope |

## Known Limitations
- The draft's RECOMMENDED global default flag is not offered; each server row
  carries its own leaf, defaulting to the secure value.
- Ze's CoA-ACK/NAK carry no Message-Authenticator; RFC 5176 Section 3.4 makes it
  a MAY and the draft does not touch CoA.
- RADIUS/TLS and RADIUS/DTLS (the draft's real fix) are separate transports not
  implemented in Ze; this spec covers RADIUS/UDP only.

## Pending judge record

The independent ze-test-judge approved the fixture change in 1bd7f3ce49 after
it landed. No `./le` approve route covers an interop fixture, so the next
commit touching this area MUST carry this body line:

`Test-judge: test/interop-radius/scenarios/radius-admin-eap-unsigned-freeradius/ze.conf: (a) the legacy row isolates the RFC 3579 Section 3.1 unsigned-EAP discard (dispatchResponse, responseIntegrityUnsignedEAP) from the presence-check path, which also bumps the EAP counter and is proven by radius-admin-pap-unsigned-freeradius; checker assertions unchanged.`

## RFC Documentation (Scope: protocol)

Add `// draft-ietf-radext-deprecating-radius-10 Section 4.N: "<quoted text>"`
above the signing in `encodeRequest`, the presence check in `dispatchResponse`,
the location-agnostic verify, the timer-untouched discard and the rate-limited
log. Quotes are byte-exact from `rfc/drafts/draft-ietf-radext-deprecating-radius-10.txt`.
The wire diagram above `encodeRequest` shows the header (0..19), the MA
attribute (20..37) and the remaining attributes (38..Length-1).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: boolean leaf only)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-radius-blastradius-message-authenticator.md` only, in the same `./le commit create` script
