# Spec: ntp-nts-client

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | NTS is a new capability; the unauthenticated NTP client ships without it, and RFC 8915 is in no release profile |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-ntp-per-server-poll-interval.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze sets the system clock from unauthenticated NTPv4 answers. An on-path
attacker can move the clock within `max-step` (default 3600 s), which breaks
certificate validity checks, RPKI, IKE and TACACS+ timestamps. Network Time
Security (RFC 8915) authenticates each NTP answer: a TLS 1.3 key exchange
(NTS-KE, TCP 4460) gives the client keys and cookies, then each NTPv4 packet
carries an authenticator extension field.

This spec adds an NTS client: a server entry can require NTS, and Ze then
never takes time from that server without a valid authenticator.

VyOS reference: VyOS configures chrony's `nts` option per server
(`~/Code/github.com/vyos/vyos-1x`, `interface-definitions/service_ntp.xml.in`).
T8882 `44d216875` is the per-server poll work this spec depends on.

**Owner decisions:**

| Decision | What Thomas decided | What it fixes in this spec |
|----------|---------------------|----------------------------|
| D1: release timing | NTS can come after the first release | The spec stays in `plan/post-release/` |
| D2: RFC 8915 enrolment | Not enrolled: no profile lists RFC 8915 and no `rfc/short/rfc8915.md` is written (RFC validation scope does not grow for an RFC no profile lists) | The spec runs with RFC quotes in code but no ledger rows, tags or discrimination records |
| D3: a library for NTS | "Any library we could use": a third-party Go module for NTS is approved, either an AES-SIV-CMAC-256 (RFC 5297) module for the authenticator or a whole NTS client such as `github.com/beevik/nts`. Cloudflare's `circl` is never used for it | Ze writes no RFC 5297 implementation of its own. Choosing the library is design work (below) |

**Design work owed by D3: choose the library.** Go's standard library has no
AES-SIV, and `github.com/beevik/ntp`, already vendored, has no NTS. Candidates
to evaluate at design, judged on RFC 5297 Appendix A vectors, RFC 8915
behaviour, maintenance, licence, dependency footprint (no `circl` anywhere in
the dependency tree) and how well it fits the plugin's per-server state:

| Candidate | Note |
|-----------|------|
| `github.com/secure-io/siv-go` | AES-SIV and AES-GCM-SIV; small, standard-library only |
| `github.com/miscreant/miscreant.go` | AES-SIV per RFC 5297; check maintenance status |
| `github.com/tink-crypto/tink-go` (deterministic AEAD, AES-SIV) | Maintained, but a large module for one primitive |
| `github.com/beevik/nts` | A whole NTS client by the author of the vendored `beevik/ntp`; it brings its own AES-SIV dependency. Evaluated against the in-plugin design; if chosen, its own dependencies are named here |

The chosen module lands in a commit of its own (`go.mod`, `go.sum` and its
`vendor/` tree, nothing else) before the commit that first uses it.

## Required Reading

### RFC texts
- [ ] `rfc/full/rfc8915.txt` - NTS-KE and the NTS extension fields
  → Constraint: Section 3: "Implementations MUST NOT negotiate TLS versions earlier than 1.3 [RFC8446] and MAY refuse to negotiate any TLS version that has been superseded by a later supported version."
  → Constraint: Section 4.1.5: "Server implementations of NTS Extension Fields for NTPv4 (Section 5) MUST support AEAD_AES_SIV_CMAC_256 [RFC5297] (Numeric Identifier 15)." A client that offers only that algorithm reaches every conforming server
- [ ] `rfc/short/rfc5905.md` - the NTPv4 client this extends
- [ ] RFC 5297 (AES-SIV) - fetch to `rfc/full/rfc5297.txt` at design

**Key insights:**
- The PKI store already holds named CA certificates (`internal/component/pki/store.go` `GetCA`), so the NTS-KE server can be verified against a named CA or the system pool.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ntp/ntp.go` - `doSync` queries each server through `ntpQueryFn` (`beevik/ntp`), checks the year and `resp.Validate`, keeps `serverState` per address; no authentication of any kind
- [ ] `internal/plugins/ntp/yang/ze-ntp-conf.yang` - a server entry has only `address`

**Behavior to preserve:**
- A server without NTS works as today

**Behavior to change:**
- A server marked for NTS is used only through NTS

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| NTP sync worker | `internal/plugins/ntp/ntp.go` `syncWorker`, `doSync` | Unauthenticated queries, per-server state | Extends: an NTS server's query goes through the NTS path; state gains cookies and keys |
| Per-server poll | `plan/post-release/features/spec-ntp-per-server-poll-interval.md` | Per-server schedule | Depends: NTS-KE renewal and cookie use follow each server's own timer |
| PKI CA store | `internal/component/pki/store.go` `GetCA` | Named CA certificates | Extends: the NTS-KE server certificate is verified against a named CA when one is set |
| Runtime state store | SDK `StateGet`/`StatePut` (`ai/doors/system.md`) | Plugin state across restart | Extends: cookies survive a restart, so the first sync after boot needs no NTS-KE |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `environment ntp server <name> { address; nts { ... } }` (shape at design)

### Transformation Path
1. NTS-KE: TLS 1.3 to port 4460 with ALPN `ntske/1`; records negotiate NTPv4 and AES-SIV; keys come from the TLS exporter; the server returns cookies and optionally another NTP server and port
2. Each NTP request carries a Unique Identifier, a cookie and an authenticator; the answer must carry a matching Unique Identifier and a valid authenticator, and refills cookies
3. Only an authenticated answer enters selection

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to NTS-KE server | TLS 1.3, RFC 8915 Section 4 records | No |
| Ze to NTP server | NTPv4 with RFC 8915 Section 5 extension fields | No |

### Changed-Wire Census
**Search evidence:** to write at design. New wires only; the existing unauthenticated query is unchanged.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `ntpQueryFn`, `serverState`, the NTP status command, doctor

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | plugin-local; PKI read through `GetCA` |
| No duplicated functionality | No | to fill at design |
| Zero-copy preserved where applicable | No | extension fields written into the request buffer |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze can build its own NTPv4 request with extension fields beside `beevik/ntp` | the library exposes no extension-field hook (to confirm in `vendor/`) | The NTS path needs its own NTPv4 encoder | read `vendor/github.com/beevik/ntp` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A failed NTS-KE silently falls back to plain NTP | test with KE refused shows a sync | AC-3 forbids it |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The clock accepts forged time, or never syncs |
| How is it reverted? | Single commit revert |
| Who else touches this path? | The per-server poll and Kiss-o'-Death specs |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with an NTS server | → | NTS-KE then authenticated query | `TestNTSAgainstChrony` (planned, beside `ntp_chrony_docker_test.go`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Server entry with NTS, chrony serving NTS | NTS-KE over TLS 1.3 succeeds, authenticated answers set the clock, status shows NTS active and the cookie count | None |
| AC-2 | NTS-KE server offers TLS 1.2 only | Handshake refused, naming the TLS version (RFC 8915 Section 3) | None |
| AC-3 | NTS-KE fails or the authenticator does not verify | No time is taken from that server; no fallback to unauthenticated NTP; doctor and the log name the cause and the fix | None |
| AC-4 | Answer with a Unique Identifier that does not match the request | Discarded | None |
| AC-5 | Cookies run out | NTS-KE runs again before the next query | None |
| AC-6 | NTS-KE certificate not trusted by the named CA (or the system pool when none is named) | Refused, naming the CA | None |
| AC-7 | Restart with stored cookies | First sync uses a stored cookie with no NTS-KE | None |
| AC-8 | Status command from every surface | Same NTS fields everywhere | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAESSIVVectors` | `internal/plugins/ntp/`, over the chosen module | RFC 5297 Appendix A vectors | |
| `TestNTSKERecordsRoundTrip` | `internal/plugins/ntp/` | Section 4 record encoding | |
| `TestNTSAuthenticatorRejects` | `internal/plugins/ntp/` | AC-3, AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NTS-KE port | 1-65535 | 1, 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ntp-nts-config` | `test/parse/` | NTS config accepted and refused shapes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ntp-nts-chrony` | `test/interop/scenarios/` | chrony, latest stable, NTS server | AC-1, AC-3 against a real NTS server | |

## Files to Modify
- `internal/plugins/ntp/ntp.go`, `internal/plugins/ntp/yang/ze-ntp-conf.yang`

## Files to Create
- the NTS-KE client and extension-field code in `internal/plugins/ntp/`
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-ntp-conf.yang` |
| YANG validation constraints | Yes | port range |
| YANG custom validators | Yes | CA name resolves |
| CLI commands/flags | Yes | status fields |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | CA name |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | existing command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | AC-3: NTS-KE unreachable, certificate untrusted |
| Prometheus counters/metrics | Yes | NTS-KE results, authenticator failures |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` entry |
| 2 | Config syntax changed | Yes | the NTP guide page |
| 7 | Wire format | Yes | a `docs/architecture/wire/` page for NTS |
| 9 | RFC behavior | No: RFC 8915 is not enrolled (D2), so no `rfc/short/` summary | - |

## Implementation Steps
1. **Phase: Wiring** - config and status, test red
2. **Phase: Library** - choose the library (D3), land its vendor commit alone, then `TestAESSIVVectors` against it
3. **Phase: NTS-KE** - AC-2, AC-5, AC-6
4. **Phase: Authenticated NTP** - AC-1, AC-3, AC-4, AC-7

## RFC Documentation (Scope: protocol)

Add `// RFC 8915 Section X.Y: "<quoted requirement>"` above enforcing code.

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No fallback to plain NTP for an NTS server | Fall back when NTS-KE fails | A fallback hands the attacker the clock by blocking port 4460 |

## Known Limitations
- Ze as an NTS server is not in this spec; Ze has no NTP server.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional parse`, `./le job run label ntp command go test -tags ze_ntp ./internal/plugins/ntp/`, `./le test integration ntp-nts-chrony`
