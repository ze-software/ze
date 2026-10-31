# Spec: tacacs-chap-mschap-login

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P3 |
| Release reason | RFC 8907 Sections 5.4.2.3 to 5.4.2.5 are optional login flows Ze does not yet offer; owner release timing lets a missing RFC feature wait until after the first release. The authen_method/authen_type correction in Phase 3 rides here because it is the same path and CHAP makes the current hard-coded value wrong in more cases |
| RFC gaps | RFC8907-5.4.2.3-1, RFC8907-5.4.2.4-1, RFC8907-5.4.2.5-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's TACACS+ client authenticates an operator login (SSH, web, MCP, API) with PAP
only. RFC 8907 defines three challenge/response login flows Ze does not offer:
CHAP (Section 5.4.2.3), MS-CHAP v1 (Section 5.4.2.4) and MS-CHAP v2 (Section
5.4.2.5). A TACACS+ server configured under Section 10.5.3 to accept
challenge/response options only refuses every Ze login today.

Goals:

| # | Goal |
|---|------|
| G-1 | The operator chooses the TACACS+ login type (`pap`, `chap`, `mschap-v1`, `mschap-v2`) for the `system authentication tacacs` block; Ze sends a START for that type that matches RFC 8907 byte for byte and a real TACACS+ server (tac_plus-ng) accepts |
| G-2 | Ze holds the operator's plaintext password at login, so Ze draws the challenge itself and computes the response; each exchange stays a single START and a single REPLY |
| G-3 | Authorization and accounting requests report how the session was really authenticated: authen_method and authen_type come from the login that produced the session, never from a constant. Today they always claim TACACS+ with ASCII, which is wrong for every PAP login and for every session the local backend authenticated |
| G-4 | With no `login-type` configured, behaviour is PAP exactly as today |

Scope is TACACS+ only. `plan/post-release/features/spec-radius-chap-eap-admin.md`
carries the same question for RADIUS admin login (its O-1). This spec answers it
for TACACS+ (yes: Ze has the plaintext password, so it plays both CHAP roles) and
names the setting `login-type`, so the RADIUS spec can follow the same shape.

Owner decisions taken at the /ze-spec gates (2026-10-31):

| Decision | Chosen | Rejected |
|----------|--------|----------|
| Scope | TACACS+ only, all three types | Merge with the RADIUS spec; drop MS-CHAP v1 |
| Setting placement | One `login-type` leaf for the whole tacacs block | Per-server leaf |
| Crypto location | Export the existing primitives from `internal/core/eap` | New `internal/core/mschap` package |
| authen_type in authorization and accounting | Carry per-session login provenance through the AAA result | Send the configured type and journal the provenance defect |
| Phasing | One spec, three phases, provenance landing as its own commit | Split provenance into its own spec |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/aaa-tacacs.md` - TACACS+ AAA design: backend chain, session authorization after PAP, text rules for usernames and PAP passwords
  → Constraint: a PASS is always followed by session authorization (`service=shell`, `cmd=`) before profiles are granted; authentication reply data never supplies privileges. This holds for every login type.
  → Constraint: usernames that are not plain text are sent as `~ze~u:<base64url>` in the user field; the MS-CHAP v2 ChallengeHash must be computed over exactly the user field bytes Ze sends, not the operator's typed name.
- [ ] `docs/guide/tacacs.md` - operator page; its capability table says "PAP login followed by shell-session authorization"
  → Constraint: this page and its config example are rewritten in Phase 2 to name `login-type`.
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - design doc declared by `internal/core/eap/mschapv2.go`
  → Constraint: exporting primitives from `internal/core/eap` must not change IKEv2 EAP behaviour; the doc's description of MS-CHAPv2 stays true.

- [ ] `ai/patterns/registration.md` - declared by files this spec changes (AAA backend registration)
  → Constraint: backends keep self-registering through `aaa` `Register`; the login record is filled by each backend in its own package, no central switch over backend names.
- [ ] `docs/architecture/system-architecture.md` - declared by files this spec changes
  → Constraint: transports carry session state they are handed; the bound accountant travels the same way the bound authorizer does, no transport reaches into a backend.
- [ ] `docs/architecture/testing/ci-format.md` - declared by the `le mock tacacs` harness
  → Constraint: the new `.ci` tests assert on the mock's logged authen fields through the existing `.ci` expectation syntax; the mock's new options are documented there if the page lists mock options.
- [ ] `docs/research/l2tpv2-ze-integration.md` - declared by a RADIUS file this spec touches
  → Decision: unaffected. The L2TP subscriber RADIUS path is not changed; only the admin RADIUS result gains a login record.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8907.md` - rows RFC8907-5.4.2.3-1, -5.4.2.4-1, -5.4.2.5-1 are recorded as gaps citing `client.go::Authenticate` builds only a PAP START
  → Constraint: each row closes only with a tagged test and a discrimination record (`./le rfc record`); the gap annotation is replaced, not kept beside the tag.
- [ ] `rfc/full/rfc8907.txt` Section 5.4.1 (versioning table), 5.4.2.3, 5.4.2.4, 5.4.2.5, 6.1 (authen_method, authen_type), 7.1 (accounting reuses the same fields), 10.5.3
  → Constraint: "PAP, CHAP, and MS-CHAP login use minor_version 1." (Section 5.4.1). `AuthenStart.Version` returns minor 1 for PAP only today and must cover all three.
  → Constraint: CHAP data is "a concatenation of the PPP id, the challenge, and the response"; response always 16 octets (Section 5.4.2.3). Challenge "minimum recommended is 8 bytes".
  → Constraint: MS-CHAP v1 response "always 49 octets"; "The TACACS+ server MUST reject authentications where the challenge deviates from 8 bytes" (Section 5.4.2.4).
  → Constraint: MS-CHAP v2 response "always 49 octets"; challenge must be 16 bytes (Section 5.4.2.5).
  → Constraint: Section 6.1 authen_type "indicates the type of authentication that was performed. If this information is not available, then the client will set authen_type to TAC_PLUS_AUTHEN_TYPE_NOT_SET := 0x00."
- [ ] `rfc/full/rfc2433.txt` (fetched 2026-10-31; no summary, because no release profile lists RFC 2433 and META-DECISIONS forbids enrolling one) - MS-CHAP v1 response layout
  → Decision: LM response is zero-filled and the "use Windows NT compatible challenge response" flag is 1: "this flag SHOULD always be set (1) and the LAN Manager compatible challenge response field SHOULD be zero-filled." Ze therefore needs no LM hash.
- [ ] `rfc/short/rfc2759.md`, `rfc/full/rfc2759.txt` Section 4 and 8 - MS-CHAP v2 response layout and GenerateNTResponse
  → Constraint: Response = Peer-Challenge (16) + Reserved (8, zero) + NT-Response (24) + Flags (1, zero).
- [ ] `rfc/short/rfc1994.md` - CHAP MD5 response = MD5(Identifier, secret, Challenge)

**Key insights:**
- All cryptography exists in `internal/core/eap`: `md5ChallengeResponse` (RFC 1994 CHAP via EAP-MD5), `ntPasswordHash`, `challengeResponse` (DES, RFC 2759 Section 8, also RFC 2433's NT response), and exported `GenerateNTResponse`. MS-CHAP v1 for TACACS+ is `challengeResponse(challenge8, ntPasswordHash(password))`.
- tac_plus-ng (pinned commit ed3c66a8, `tac_plus-ng/authen.c`) dispatches CHAP, MSCHAP and MSCHAPV2 START packets at minor version 1; MS-CHAP is compiled only `#ifdef WITH_CRYPTO` and verifies only the NT response (it reads from data offset 1 + challenge + 24). Its v2 ChallengeHash uses `session->username.txt` with no domain stripping.
- tac_plus-ng's DES uses `EVP_des_ecb()`; on OpenSSL 3 single DES lives in the legacy provider (assumption A-3).
- The per-session authorizer already exists: `aaa.AuthorizerForResult` binds `AuthResult` to a session authorizer through `BindProfiles`, and transports carry `result.Authorizer`. Accounting has no per-session view: `Accountant.CommandStart(username, remoteAddr, command)` is called on a shared accountant.
- FRR and BIRD do not implement TACACS+, so the interop reference is tac_plus-ng (the RFC is not silent on any layout this spec needs).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/tacacs/authen.go` - `AuthenStart` and `MarshalBinaryInto` (validates a PAP password as text), `Version` (minor 1 only for PAP login), `NewPAPAuthenStart`; constant `authenTypePAP = 0x02` is the only type constant
- [ ] `internal/component/tacacs/client.go` - `TacacsClient.Authenticate(username, password, port, remAddr)` builds a PAP START, clears its data after use, and `sendToServers` re-runs the marshal closure for every server attempt so in-place encryption never corrupts a retry
- [ ] `internal/component/tacacs/authenticator.go` - `tacacsAuthenticator.Authenticate` maps PASS/FAIL/RESTART/FOLLOW/unknown; `handlePass` sends session authorization with `AuthenType: authenTypePAP`
- [ ] `internal/component/tacacs/authorizer.go` - `authorize` builds every command AuthorRequest with `AuthenMethod: AuthenMethodTACACS`, `AuthenType: 0x01` (ASCII); `BindProfiles` and `BindLocalFallback` produce session views
- [ ] `internal/component/tacacs/accounting.go` - START and STOP records carry `AuthenMethod: 0x06`, `AuthenType: 0x01` (ASCII) as constants
- [ ] `internal/component/tacacs/config.go` - `ExtractConfig` reads servers, timeout, source-address, authorization, strict-fallback, accounting; no login type
- [ ] `internal/component/tacacs/yang/ze-tacacs-conf.yang` - `system authentication tacacs` container with `list server` (ordered-by user, port, key)
- [ ] `internal/component/aaa/aaa.go`, `types.go`, `login_profiles.go` - `AuthResult{Authenticated, Profiles, Source, LocalGeneration, Authorizer}`; `AuthorizerForResult` calls `BindProfiles`; `Accountant` and `CommandArgsAccountant` are username/remoteAddr based
- [ ] `internal/component/plugin/server/command.go` - `CommandContext` carries `Authorizer` per request; `beginAccounting` uses the dispatcher's shared `d.accountant`
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `liveAAABundleAuthorizer`, `liveLocalAuthorizer`, `acceptedLocalGenerationAuthorizer` implement `BindProfiles`; `liveAAABundleAccountant` forwards to the bundle's accountant
- [ ] `internal/core/eap/mschapv2.go`, `md4.go`, `eap_md5challenge.go` - the primitives named in Key insights
- [ ] `internal/test/mock/tacacs/tacacs.go` - mock server; authen compares the START data to the password (PAP only)
- [ ] `internal/component/tacacs/tacplusng_docker_test.go` - `TestTacPlusNGInterop` runs a PAP login, session authorization and command authorization against tac_plus-ng built from `test/interop-tacacs/Dockerfile.tacplusng`

**Behavior to preserve:**
- A config without `login-type` sends exactly today's PAP START (minor version 1, data = password) and today's validation errors.
- PASS is followed by session authorization before any profile is granted; FAIL, RESTART and FOLLOW are rejections that stop the chain; connection failure lets the chain try the next backend.
- Credentials and decrypted scratch are cleared before the pooled buffer is reused (`releaseBuffer`, `clear(start.Data)`).
- Server order and failover in `sendToServers`.
- IKEv2 EAP-MSCHAPv2 and EAP-MD5 behaviour and their RFC-tagged tests.

**Behavior to change:**
- `login-type` selects CHAP, MS-CHAP v1 or MS-CHAP v2 START bodies.
- `Version` returns minor version 1 for every PAP, CHAP and MS-CHAP login.
- Session authorization reports the login type that was used.
- Command authorization and accounting report authen_method and authen_type from the session's own login record instead of TACACS+/ASCII constants.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator login on SSH (password), web, MCP or API: `aaa.AuthRequest{Username, Password, RemoteAddr, Service}`.
- Config: `system authentication tacacs login-type <type>`.

### Transformation Path
1. Config load: `ExtractConfig` reads `login-type` into `ExtractedConfig`; `register.go` passes it to `TacacsClientConfig`.
2. `tacacsAuthenticator.Authenticate` calls the client with the prepared username and password.
3. `TacacsClient.Authenticate` builds the START for the configured type. The marshal closure draws a fresh PPP id and challenge from `crypto/rand` on every server attempt and computes the response with the `internal/core/eap` primitives over the wire user field.
4. REPLY handling is unchanged; on PASS, session authorization carries authen_type = the login type.
5. The authenticator returns `AuthResult` with a login record (backend, login type). `aaa.AuthorizerForResult` binds the session authorizer from the whole result; a new accountant binding does the same for accounting.
6. Transports (SSH, web, API, MCP, plugin dispatch) carry the bound accountant beside the bound authorizer; command authorization and accounting read authen_method/authen_type from the record.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ TACACS+ server | RFC 8907 START over TCP, body obfuscated with the shared key | No |
| tacacs component ↔ core/eap | Direct import of exported primitives (radius and ike already import core/eap) | No |
| AAA backend ↔ transports | `aaa.AuthResult` fields carried with the session | No |

### Integration Points
- `aaa.AuthorizerForResult` - replaced binding takes the result, so TACACS+ sees the login record
- `CommandContext` / `plugin.CallerIdentity` / web and SSH session state - gain the bound accountant beside `Authorizer`
- `liveAAABundleAccountant` - binds per session against the live bundle the same way `liveAAABundleAuthorizer` does

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze always has the operator's plaintext password when TACACS+ authenticates | `aaa.AuthRequest.Password` is the only credential `tacacsAuthenticator` receives; public-key SSH never reaches TACACS+ authentication | CHAP cannot be computed; design collapses | Read every `Authenticate` caller (ssh passwordauth, web auth, api) | unvalidated |
| A-2 | tac_plus-ng accepts all three types at minor version 1 | `tac_plus-ng/authen.c` dispatch at the pinned commit | Interop proof needs another server | `TestTacPlusNGInterop` subtests | unvalidated |
| A-3 | The tac_plus-ng image computes single DES on OpenSSL 3 | `mschap_desencrypt` uses `EVP_des_ecb()`; OpenSSL 3 moves single DES to the legacy provider | MS-CHAP subtests FAIL for a server-side reason | Run the MS-CHAP subtest; if it fails, check the server log and enable the legacy provider in the Dockerfile with `OPENSSL_CONF` | unvalidated |
| A-4 | `configure` enables WITH_CRYPTO when openssl-dev is present | `configure` sets the define; the Dockerfile installs openssl-dev | MS-CHAP START hits no handler | `tac_plus-ng -v` output or a failing subtest | unvalidated |
| A-5 | MS-CHAP v2 ChallengeHash over the exact wire user field matches tac_plus-ng | tac_plus-ng hashes `session->username.txt` without domain stripping | Logins with a `~ze~u:` or `DOMAIN\` name fail | Interop subtest with a reserved-form username | unvalidated |
| A-6 | MS-CHAP passwords may hold any Unicode; NtPasswordHash takes UTF-16LE | RFC 2759 Section 8 NtPasswordHash | Non-ASCII password fails | Unit test with a non-ASCII password against RFC 2759 vectors | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reusing one challenge across servers lets a hostile server replay the response to the next | A test sees two attempts with the same challenge | Draw id and challenge inside the marshal closure, per attempt; unit test asserts two attempts differ |
| R-2 | Exporting core/eap functions changes the bodies of RFC-tagged tests, staling their discrimination records | `./le rfc check` refuses the commit | Re-record each affected record with `./le rfc record` in Phase 1 |
| R-3 | Phase 3 changes a shared AAA interface; a transport that does not carry the bound accountant silently falls back to the shared one and reports wrong provenance | A transport test sees NOT_SET where TACACS+ logged in | Replace `BindProfiles` (no hybrid); every transport carries the bound accountant; one functional test per transport |
| R-4 | A session with no login record (internal plugin identity, internal RPC) reaches TACACS+ authorization | Authorization claims TACACS+ for an internal caller | Missing record means authen_method NOT_SET and authen_type NOT_SET, stated in code and tested |
| R-5 | Password bytes linger in heap copies (UTF-16 buffer, MD5 input) | Code review | Clear every intermediate buffer holding password material; the response, not the password, enters the START |
| R-6 | `crypto/rand` failure | Error from the reader | Return an error before sending; the chain treats it as an infrastructure failure, never a PASS |
| R-7 | MS-CHAP v1 is weak (DES over the NT hash) | Operator picks it without knowing | YANG description and guide say so, and recommend mschap-v2 or chap |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Operator logins through TACACS+ fail (wrong START), or authorization and accounting records misreport the login method to the TACACS+ server's logs and policy |
| How is it reverted? | Phases 1 and 2: single commit revert; default `pap` keeps existing configs working. Phase 3: revert of its own commit restores BindProfiles |
| Who else touches this path? | RADIUS admin AAA (shares `aaa.AuthResult` and binding), IKEv2 EAP (shares core/eap), `spec-radius-chap-eap-admin` (same setting shape) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `login-type chap` in config, SSH password login | → | `TacacsClient.Authenticate` CHAP START | `test/plugin/tacacs-login-chap.ci` |
| `login-type mschap-v1` in config, SSH password login | → | MS-CHAP v1 START | `test/plugin/tacacs-login-mschap-v1.ci` |
| `login-type mschap-v2` in config, SSH password login | → | MS-CHAP v2 START | `test/plugin/tacacs-login-mschap-v2.ci` |
| SSH login via TACACS+, then a command with accounting on | → | bound accountant reports TACACS+ and the login type | `test/plugin/tacacs-acct-login-provenance.ci` |
| Local-fallback login, then a command with TACACS+ authorization | → | bound authorizer reports LOCAL / NOT_SET | `test/plugin/tacacs-author-local-provenance.ci` |
| `ze config validate` with an unknown `login-type` value | → | YANG enumeration refusal | `test/parse/tacacs-login-type-invalid.ci` |
| Web login by local fallback, then a command | → | bound authorizer and accountant on the web session | `test/plugin/tacacs-web-fallback-bound.ci` |
| Web editor commit with accounting on | → | session accountant on the editor mutation path | `test/plugin/tacacs-acct-web-editor.ci` |
| gNMI Set with accounting on | → | NOT_SET accounting record | `test/plugin/tacacs-acct-gnmi-set.ci` |
| tac_plus-ng real server, each type | → | full START, PASS, session authorization | `TestTacPlusNGInterop` subtests `chap`, `mschap-v1`, `mschap-v2` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No `login-type` configured | START is PAP at minor version 1 with data = password, byte-identical to before; existing PAP tests unchanged |
| AC-2 | `login-type chap` | START: action LOGIN, authen_type 0x03, minor version 1, user = wire user field, data = 1-byte id + 16-byte challenge + 16-byte MD5(id, password, challenge) |
| AC-3 | `login-type mschap-v1` | START: authen_type 0x05, minor version 1, data = 1-byte id + 8-byte challenge + 24 zero bytes + 24-byte NT response + flag byte 0x01 |
| AC-4 | `login-type mschap-v2` | START: authen_type 0x06, minor version 1, data = 1-byte id + 16-byte authenticator challenge + 16-byte peer challenge + 8 zero bytes + 24-byte NT response + flag byte 0x00; the NT response is GenerateNTResponse over the wire user field |
| AC-5 | Two server attempts in one login (first server unreachable) | The second START carries a different id-and-challenge from the first |
| AC-6 | A code path asks for an MS-CHAP v1 challenge that is not 8 bytes or a v2 challenge that is not 16 bytes | Marshal refuses with an error; nothing is sent |
| AC-7 | tac_plus-ng with alice's correct password, for each of chap, mschap-v1, mschap-v2 | PASS, session authorization returns priv-lvl 15, Ze grants the mapped profile |
| AC-8 | tac_plus-ng with a wrong password, each type | FAIL; the chain stops with a rejection |
| AC-9 | PASS under any login type | The session authorization request carries authen_method TACACS+ (0x06) and authen_type equal to the login type |
| AC-10 | A session TACACS+ authenticated runs a command with TACACS+ command authorization and accounting on | Both requests carry authen_method 0x06 and authen_type of that session's login type |
| AC-11 | A session the local backend authenticated (TACACS+ unreachable, local fallback) runs a command with TACACS+ command authorization and accounting on | Both requests carry authen_method LOCAL (0x05) and authen_type NOT_SET (0x00) |
| AC-12 | An internal identity (plugin or internal RPC) reaches TACACS+ authorization or accounting | authen_method NOT_SET (0x00) and authen_type NOT_SET (0x00) |
| AC-13 | `login-type` set to a value outside the enumeration | `ze config validate` refuses it naming the valid values |
| AC-14 | CHAP or MS-CHAP with a non-ASCII password | The response follows the RFC (CHAP: password bytes; MS-CHAP: UTF-16LE); no printable-ASCII refusal applies, which stays PAP-only |
| AC-15 | crypto/rand fails while drawing a challenge | Authenticate returns an infrastructure error; no START is sent; no PASS |
| AC-16 | A web login admitted by local fallback (TACACS+ rejected or unreachable) | The session carries a bound authorizer and accountant exactly as an SSH session does; a recovery-profile session stops being authorized once a newer local generation is accepted |
| AC-17 | An operator edits and commits config in the web editor with TACACS+ accounting on | A TACACS+ accounting record is sent for the mutation carrying the session's login record; if a test shows one was already sent, the journal row is closed as not-a-defect with that test as evidence |
| AC-18 | A gNMI Set with TACACS+ accounting on | An accounting record is sent with authen_method and authen_type NOT_SET; if a test shows gNMI writes are already accounted, the row is closed with that evidence |
| AC-19 | Any transport that authenticates without AAA (MCP bearer/OAuth, API token, local CLI, looking glass) reaches command accounting | The record carries authen_method and authen_type NOT_SET, never TACACS+ |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `login-type mschap-v2`, logs in over SSH with a TACACS+ account | SSH password → aaa chain → tacacs authenticator → MS-CHAP v2 START → PASS → session authorization → profile | `test/plugin/tacacs-login-mschap-v2.ci`, `TestTacPlusNGInterop/mschap-v2` |
| 2 | Logs in over SSH with CHAP and runs a command with accounting on | login → bound accountant → ACCT START with authen_type CHAP | `test/plugin/tacacs-acct-login-provenance.ci` |
| 3 | Logs in while TACACS+ is down (local fallback), runs a command | local backend → bound authorizer → AUTHOR carries LOCAL / NOT_SET | `test/plugin/tacacs-author-local-provenance.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCHAPResponseRFC1994` | `internal/core/eap/chap_export_test.go` | Exported CHAP-MD5 response matches a fixed vector | |
| `TestChallengeResponseRFC2759Vector` | `internal/core/eap/rfc2759_mschapv2_test.go` (existing, renamed calls) | Exported NtPasswordHash and ChallengeResponse still match RFC 2759 Section 9.2 vectors | |
| `TestRFC8907CHAPStartLayout` | `internal/component/tacacs/rfc8907_chap_test.go` | AC-2 byte layout and minor version | |
| `TestRFC8907MSCHAPv1StartLayout` | `internal/component/tacacs/rfc8907_chap_test.go` | AC-3 layout, zero LM field, flag 1 | |
| `TestRFC8907MSCHAPv2StartLayout` | `internal/component/tacacs/rfc8907_chap_test.go` | AC-4 layout, reserved zero, flag 0, NT response over wire user field (including `~ze~u:` form) | |
| `TestRFC8907MSCHAPChallengeLength` | `internal/component/tacacs/rfc8907_chap_test.go` | AC-6 refusal | |
| `TestTacacsFreshChallengePerAttempt` | `internal/component/tacacs/client_test.go` | AC-5 | |
| `TestTacacsChallengeRandFailure` | `internal/component/tacacs/client_test.go` | AC-15 | |
| `TestTacacsPAPUnchanged` | `internal/component/tacacs/authen_test.go` | AC-1 golden bytes | |
| `TestExtractConfigLoginType` | `internal/component/tacacs/config_test.go` | Default pap; each enum value parsed | |
| `TestRFC8907SessionAuthorAuthenType` | `internal/component/tacacs/rfc8907_authorization_test.go` | AC-9 | |
| `TestRFC8907CommandAuthorProvenance` | `internal/component/tacacs/rfc8907_authorization_test.go` | AC-10, AC-11, AC-12 | |
| `TestRFC8907AcctProvenance` | `internal/component/tacacs/rfc8907_accounting_test.go` | AC-10, AC-11, AC-12 for START and STOP | |
| `TestAuthorizerForResultBindsResult` | `internal/component/aaa/login_profiles_test.go` | Binding receives the whole result, profiles and login record | |
| `TestAccountantForResult` | `internal/component/aaa/login_profiles_test.go` | Accountant binding; no binder returns the shared accountant unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MS-CHAP v1 challenge length | 8 exactly | 8 | 7 | 9 |
| MS-CHAP v2 challenge length | 16 exactly | 16 | 15 | 17 |
| START data length (uint8) | 0-255 | 66 (v2: 1+16+49) | N/A | 256 |
| MS-CHAP password length (RFC 2759: 256 Unicode chars) | 0-256 | 256 | N/A | 257 refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tacacs-login-chap` | `test/plugin/tacacs-login-chap.ci` | SSH login against `le mock tacacs` set to verify CHAP succeeds; wrong password fails | |
| `tacacs-login-mschap-v1` | `test/plugin/tacacs-login-mschap-v1.ci` | Same for MS-CHAP v1 | |
| `tacacs-login-mschap-v2` | `test/plugin/tacacs-login-mschap-v2.ci` | Same for MS-CHAP v2 | |
| `tacacs-acct-login-provenance` | `test/plugin/tacacs-acct-login-provenance.ci` | Mock logs authen_method and authen_type of ACCT START/STOP after a CHAP login | |
| `tacacs-author-local-provenance` | `test/plugin/tacacs-author-local-provenance.ci` | Mock logs LOCAL / NOT_SET on command authorization after local fallback | |
| `tacacs-login-type-invalid` | `test/parse/tacacs-login-type-invalid.ci` | Config with `login-type foo` refused | |
| `tacacs-web-fallback-bound` | `test/plugin/tacacs-web-fallback-bound.ci` | AC-16: web fallback session authorized and accounted with its own LOCAL record | |
| `tacacs-acct-web-editor` | `test/plugin/tacacs-acct-web-editor.ci` | AC-17: web editor commit produces an accounting record | |
| `tacacs-acct-gnmi-set` | `test/plugin/tacacs-acct-gnmi-set.ci` | AC-18: gNMI Set produces a NOT_SET accounting record | |

The mock (`internal/test/mock/tacacs/tacacs.go`) gains verification of CHAP, MS-CHAP v1 and v2 START bodies (using the core/eap primitives) and logs authen_method and authen_type of every AUTHOR and ACCT request, so the `.ci` tests assert on its log.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestTacPlusNGInterop` subtests `chap`, `mschap-v1`, `mschap-v2`, each with a wrong-password case | `internal/component/tacacs/tacplusng_docker_test.go`, config in the same file (`password { chap = clear ...; mschap = clear ... }`) | tac_plus-ng (pinned) | A real server accepts each START and rejects a wrong password | |

Each tagged test gets a recorded break (`./le rfc record`): corrupt one response byte, swap the challenge length, drop the minor version, and confirm red.

## Files to Modify
- `internal/core/eap/eap_md5challenge.go` - export the CHAP-MD5 response (rename, callers updated)
- `internal/core/eap/mschapv2.go` - export `NtPasswordHash` and `ChallengeResponse` (rename, callers updated)
- `internal/core/eap/*_test.go` - follow the renames; re-record affected discrimination records
- `internal/component/tacacs/authen.go` - type constants for CHAP, MSCHAP, MSCHAPV2, NOT_SET, LOCAL; `Version` covers all login types; START builders per type; MS-CHAP challenge-length guard
- `internal/component/tacacs/client.go` - `Authenticate` builds the configured type, fresh challenge per attempt, clears intermediates
- `internal/component/tacacs/config.go` - `LoginType` in `ExtractedConfig`
- `internal/component/tacacs/register.go` - pass login type to client and authenticator
- `internal/component/tacacs/yang/ze-tacacs-conf.yang` - `leaf login-type` enumeration, default pap, description warning about mschap-v1
- `internal/component/tacacs/authenticator.go` - session authorization authen_type; `AuthResult` login record
- `internal/component/tacacs/authorizer.go` - bind from result; authen_method/authen_type from the record
- `internal/component/tacacs/accounting.go` - bind from result; authen_method/authen_type from the record
- `internal/component/aaa/aaa.go`, `types.go`, `login_profiles.go` - login record on `AuthResult`; replace `BindProfiles` binder with a result binder; accountant binder and `AuthResult.Accountant`
- `internal/component/authz/register.go`, `cmd/ze/hub/aaa_lifecycle.go` - implement the replaced binder; `liveAAABundleAccountant` binds per result
- `internal/component/radius/authenticator.go` - set its login record (RADIUS, PAP) so the record is never silent
- `internal/component/authz/auth.go` (local password login) and `internal/component/ssh/pubkey.go` (local public-key login) - set the LOCAL record, authen_type NOT_SET
- Carriers of the bound accountant, traced 2026-10-31 (every site that carries `result.Authorizer` today):

| File | Transport | Change |
|------|-----------|--------|
| `internal/component/plugin/server/command.go` | the single accounting sink | `CommandContext.Accountant`; `beginAccounting` reads it the way `isAuthorized` reads `ctx.Authorizer`, falling back to `d.accountant` only for a context with no session |
| `internal/component/plugin/dispatch.go` | all | `CallerIdentity.Accountant`, context key and getter beside `WithCallerAuthorizer` |
| `cmd/ze/hub/main_servers.go` | web, MCP, CLI, LG, REST, gRPC, no-BGP SSH | copy caller accountant into `CommandContext`, with the same context fallback as the authorizer |
| `cmd/ze/hub/service_ssh.go` | SSH exec and streaming, no-BGP `CallerIdentity` | set `CommandContext.Accountant`; the streaming `BeginAccounting` call uses it |
| `internal/component/ssh/ssh.go` | SSH password and public key | store the bound accountant per session; the public-key branch binds from the result like the password branch |
| `internal/component/ssh/session.go`, `cmd/ze/hub/session_factory.go` | SSH TUI | pass the bound accountant to `ExecutorForUser` |
| `internal/component/web/auth.go` | web cookie and Basic | `webSession.Accountant`; carried through the request context |
| `cmd/ze/hub/aaa_lifecycle.go` | SSH, web | `liveAAABundleAuthenticator` binds every successful result, fallback returns included (journal row); `liveAAABundleAccountant` binds per result |
| `cmd/ze/hub/service_web.go` | web | the fallback authenticator's result is bound by the change above; no local wrapping left unbound |
| `cmd/ze/hub/api.go` | REST, gRPC, API streaming | password results carry the bound accountant; token mode carries a NOT_SET accountant view |
| `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go` | REST, gRPC | unchanged if the accountant rides on `CallerIdentity`, which they copy whole; verified by test |
| `internal/component/web/rbac.go` and editor handlers | web config editor | account mutations through the session accountant (AC-17) |
| `internal/component/gnmi/set.go` | gNMI | account Set through the dispatcher's NOT_SET view (AC-18) |
| `internal/component/mcp/tools.go` | MCP | no AAA login; carries no accountant, so the sink's NOT_SET fallback applies (AC-19) |
- `internal/test/mock/tacacs/tacacs.go` - verify CHAP and MS-CHAP; log authen fields
- `internal/component/tacacs/tacplusng_docker_test.go` - three subtests; `test/interop-tacacs/Dockerfile.tacplusng` only if A-3 or A-4 fails
- `rfc/short/rfc8907.md` - replace the three gap annotations with test tags
- `docs/guide/tacacs.md`, `docs/architecture/aaa-tacacs.md`, `features/tacacs-aaa.md` - login types, provenance

## Files to Create
- `internal/component/tacacs/rfc8907_chap_test.go` - START layout tests
- `internal/core/eap/chap_export_test.go` - CHAP export vector
- `test/plugin/tacacs-login-chap.ci`, `tacacs-login-mschap-v1.ci`, `tacacs-login-mschap-v2.ci`, `tacacs-acct-login-provenance.ci`, `tacacs-author-local-provenance.ci`
- `test/parse/tacacs-login-type-invalid.ci`
- `rfc/full/rfc2433.txt` - fetched 2026-10-31 (text only, no summary)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/tacacs/yang/ze-tacacs-conf.yang`, `leaf login-type` |
| YANG validation constraints | Yes | enumeration pap, chap, mschap-v1, mschap-v2; default pap |
| YANG custom validators | N-A | enumeration is fully native |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | Yes | `login-type <value>` follows keyword-before-value |
| Editor autocomplete | Yes | automatic from the enumeration; covered by an `.et` row in `test/editor/completion/` if the existing tacacs completion test lists leaves |
| Functional test for new RPC/API | N-A | no new RPC; login and accounting `.ci` listed above |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | not under `environment/` |
| Doctor check for runtime dependencies | N-A | no new file, socket, port, module or binary |
| Prometheus counters/metrics | N-A | existing TACACS+ counters, if any, are per request and unchanged |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/tacacs-aaa.md` (login types, provenance) |
| 2 | Config syntax changed? | Yes | `docs/guide/tacacs.md` config example; `docs/guide/configuration.md` if it lists tacacs leaves |
| 3 | CLI command added/changed? | No | no command |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | component, not plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/tacacs.md` capability table and authentication flow |
| 7 | Wire format changed? | Yes | `docs/architecture/aaa-tacacs.md` START bodies per type and authen fields in AUTHOR/ACCT |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8907.md` rows 5.4.2.3-1, 5.4.2.4-1, 5.4.2.5-1 and the `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | mock tacacs gains CHAP/MS-CHAP verification: `docs/functional-tests.md` if it documents mock options |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists TACACS+ login types |
| 12 | Internal architecture changed? | Yes | `docs/architecture/aaa-tacacs.md` and `docs/guide/authorization.md`: result binding replaces BindProfiles, per-session accountant |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/post-release/features/spec-tacacs-chap-mschap-login.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/tacacs.md` example checked against the YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - YANG leaf, `ExtractConfig`, client config field, empty START builders returning an error; the three login `.ci` files and the parse `.ci` written and failing
   - Tests: wiring table rows 1-3 and 6
   - Files: YANG, `config.go`, `register.go`, `client.go`
   - Verify: config accepted; login `.ci` fails because the builder is a stub
2. **Phase: Crypto export** - export the three core/eap primitives, update callers and tests, re-record discrimination records the rename stales
   - Tests: `TestCHAPResponseRFC1994`, existing RFC 2759 vectors
   - Files: `internal/core/eap/*`
   - Verify: IKE and RADIUS EAP tests unchanged and green; `./le rfc check` clean
3. **Phase: START bodies** - CHAP, MS-CHAP v1, v2 builders, minor version, fresh challenge per attempt, length guards, buffer clearing; mock verifies each type
   - Tests: AC-1 to AC-8, AC-14, AC-15 unit and functional tests; `TestTacPlusNGInterop` subtests
   - Files: `authen.go`, `client.go`, `authenticator.go`, mock, docker test
   - Verify: red-then-green; interop subtests pass; discrimination records for the three RFC rows
   - Commit after this phase
4. **Phase: Login provenance** - login record on `AuthResult`; result binder replaces `BindProfiles` in every implementer; accountant binding and transport carriage; TACACS+ author/acct read the record
   - Tests: AC-9 to AC-12 and AC-16 to AC-19 unit tests, provenance `.ci` files
   - Files: aaa, authz, tacacs authorizer/accounting, radius and local authenticators, and every carrier in the traced table under Files to Modify
   - Verify: every transport in that table has a functional or unit test; no remaining `BindProfiles` symbol; the three owned journal rows are closed `fixed:` (or closed with the test that shows no defect)
   - Commit after this phase
5. **Phase: Docs and RFC rows** - each phase updates its pages as it lands (`ai/rules/documentation.md`); this step checks the RFC summary rows and the feature level

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | MS-CHAP v2 NT response hashed over the wire user field, not the typed username; challenge drawn per attempt; minor version 1 for all four login types |
| Naming | YANG enum values `mschap-v1`/`mschap-v2` kebab-case; Go constants mirror RFC names |
| Data flow | Login record set by the backend that authenticated; authorizer and accountant never infer it |
| Rule: no-layering | `BindProfiles` deleted, not kept beside the result binder |
| Rule: rfc-compliance | RFC 8907 quotes above each builder; RFC 2433 and 2759 quotes above the v1 and v2 response assembly |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three login types selectable | `grep -n "login-type" internal/component/tacacs/yang/ze-tacacs-conf.yang` |
| Three RFC rows tagged | `./le rfc check` and `grep -n "5.4.2.[345]-1" rfc/short/rfc8907.md` shows tags, no gap |
| Interop proof | `TestTacPlusNGInterop` output pasted with the three subtests |
| No constant authen fields | `grep -n "AuthenType: *0x01" internal/component/tacacs/*.go` returns nothing |
| BindProfiles gone | `grep -rn "BindProfiles" internal cmd` returns nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Credential lifetime | Password, UTF-16 buffer, NT hash and MD5 input cleared after use; only the response enters the START |
| Challenge quality | `crypto/rand`, per attempt; failure is an error, never a weak challenge |
| Weak method warning | YANG description and guide state mschap-v1 is DES-based and not recommended |
| Fail closed | Missing login record yields NOT_SET, never TACACS+; no new path grants a profile without session authorization |
| Log hygiene | No response, challenge or password in logs |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| MS-CHAP interop fails while unit vectors pass | Check A-3/A-4 against the tac_plus-ng log before touching Ze |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-31 | AAA fallback login binding (web session left unbound) |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-10-31 | web config-editor mutations not accounted |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-10-31 | gNMI Set neither session-authorized nor accounted |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze draws the challenge and computes the response | Pass a client-supplied challenge through | No client on SSH/web/API speaks CHAP; Ze holds the plaintext password, so it plays both roles in one START |
| One `login-type` per tacacs block | Per-server leaf | One login is one type; authorization and accounting report one type without tracking which server answered |
| Export from core/eap | New core/mschap package | One copy of each algorithm, no move of RFC-tagged tests; radius and ike already import core/eap |
| MS-CHAP v1 sends zero LM field, flag 1 | Compute the LM response | RFC 2433 says SHOULD zero-fill and SHOULD set the flag; tac_plus-ng reads only the NT response |
| Fresh challenge per server attempt | One challenge per login | A response shown to one server cannot be replayed to the next |
| Login record on `AuthResult`, bound per session for both authorizer and accountant | Configured type sent by a shared authorizer | The shared authorizer cannot know which backend authenticated a session; only the result knows |
| Missing record means NOT_SET | Default to TACACS+ | RFC 8907 Section 6.1 names NOT_SET for "information is not available"; claiming TACACS+ would be a silent wrong value |

## Known Limitations
- RADIUS admin CHAP stays in `plan/post-release/features/spec-radius-chap-eap-admin.md`; this spec only gives RADIUS results a login record (RADIUS, PAP) so provenance is never silent.
- ASCII login (Section 5.4.2.1) and ENABLE requests (RFC8907-5.4.2.6-1) stay out of scope; ENABLE remains a recorded gap in `rfc/short/rfc8907.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
Every START builder quotes its RFC 8907 Section 5.4.2.x sentence; the MS-CHAP
v1 and v2 response assembly quotes RFC 2433 Section 3.2 / RFC 2759 Section 4;
the authen_type choice quotes RFC 8907 Section 6.1. Each builder carries an
ASCII diagram of the data field with byte offsets.

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
- [ ] AC-1..AC-19 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-tacacs-chap-mschap-login.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
