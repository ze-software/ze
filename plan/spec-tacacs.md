# Spec: tacacs -- TACACS+ authentication, authorization, and accounting

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/8 |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/authz/auth.go` -- current bcrypt-only password auth
4. `internal/component/ssh/ssh.go` -- wish-based SSH server, password auth middleware
5. `internal/component/ssh/schema/ze-ssh-conf.yang` -- SSH/user config schema
6. `cmd/ze/hub/main.go` -- user loading from zefs and config

## Task

Add TACACS+ client support to ze for SSH authentication, command authorization, and
accounting. TACACS+ is the standard AAA protocol for ISP network equipment management.

This authenticates SSH logins against a central TACACS+ server, allowing
centralised user management across the Exa network fleet. Without TACACS+, each device
needs local user accounts maintained individually.

### What TACACS+ provides

| Function | Description | Exa usage |
|----------|-------------|-----------|
| **Authentication** | Verify username + password against central server | SSH login for all Exa engineers |
| **Authorization** | Check if user is allowed to run a specific command | Per-user privilege levels |
| **Accounting** | Log command execution to central server | Audit trail of who did what |

### Current state of ze auth

- Password auth only (bcrypt hashes), no public key auth
- No pluggable auth backend (hardcoded bcrypt in authz/auth.go)
- Users from YANG config or zefs blob store
- Profile-based RBAC exists (authz/authz.go) but only with local profiles
- wish SSH server supports password auth callback

### What needs to change

1. **Auth backend abstraction**: Replace hardcoded bcrypt with pluggable Authenticator interface
2. **TACACS+ client**: Connect to TACACS+ servers, perform authen/author/acct exchanges
3. **Auth chain**: Try TACACS+ first, fall back to local users if server unreachable
4. **Authorization integration**: Map TACACS+ privilege levels to ze authz profiles
5. **Accounting**: Send command start/stop records to TACACS+ server

## Required Reading

### Architecture Docs
- [ ] `internal/component/authz/auth.go` -- current auth implementation
  --> Constraint: CheckPassword does bcrypt comparison. AuthenticateUser iterates user list
  --> Constraint: timing-safe with dummy hash for unknown users
- [ ] `internal/component/authz/authz.go` -- profile-based RBAC
  --> Constraint: profiles have Run (operational) and Edit (config) rule sets
  --> Constraint: users assigned profiles via config
- [ ] `internal/component/ssh/ssh.go` -- wish SSH server
  --> Constraint: wish.WithPasswordAuth callback for password verification
  --> Constraint: auth set up at server creation, not hot-reloadable
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` -- SSH config schema
  --> Constraint: system.authentication.user list with name, password, profile
- [ ] `cmd/ze/hub/main.go` -- user loading from zefs and config
  --> Constraint: loadZefsUsers returns []authz.UserConfig

### RFC Summaries (MUST for protocol work)
- [ ] RFC 8907 -- The TACACS+ Protocol (2020, formalises the Cisco draft)
  --> Constraint: TCP port 49, packet encryption with shared secret
  --> Constraint: three services: authentication, authorization, accounting
  --> Constraint: authentication types: ASCII (interactive), PAP, CHAP, MS-CHAP
  --> Constraint: single-connect mode (persistent TCP) vs per-session TCP
  --> Constraint: 12-byte fixed header, body encrypted with MD5 pseudo-pad XOR
  --> Constraint: sequence numbers start at 1, client odd, server even, max 0xFE
  --> Constraint: version byte 0xC0 default, 0xC1 for PAP/CHAP/MSCHAP authen
  --> Decision: ze summary at rfc/short/rfc8907.md

### Reference implementations (studied, not vendored)
- github.com/nwaples/tacplus -- Go client/server, unmaintained (5 years), known issues with buffer allocation, no failover, no graceful shutdown, legacy logging. Clean packet marshal/unmarshal pattern worth following.
- github.com/facebookincubator/tacquito -- Facebook Go server, dependency injection, handler composition. Good testing patterns.
- Both cloned to ~/Code/github.com/ for reference during implementation.

**Key insights:**
- TACACS+ uses TCP (port 49), not UDP like RADIUS
- Packet body is XOR-obfuscated with MD5 pseudo-pad (NOT true encryption per RFC 8907 Section 10.5)
- Pseudo-pad: MD5(session_id + key + version + seq_no), chained for subsequent 16-byte blocks
- Secret validation after decryption: field lengths must sum to header body length
- Authentication flow: START -> REPLY (pass/fail/getdata/getuser/error), optionally CONTINUE
- Authorization flow: REQUEST -> RESPONSE (pass-add/pass-repl/fail/error), single round-trip
- Accounting flow: REQUEST -> REPLY (success/error), single round-trip
- Authorization arguments use = (mandatory, client must handle) or * (optional, may ignore)
- wish SSH password callback receives (ctx, password) and returns bool
- ze's authz profiles map naturally to TACACS+ privilege levels (priv-lvl=15 -> admin, priv-lvl=1 -> read-only)
- Privilege levels 0-15: 0=minimum, 1=user default, 15=admin. 2-14 site-defined.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/authz/auth.go` -- AuthenticateUser takes []UserConfig, username, credential. Returns bool. No interface, no extensibility. Bcrypt only.
  --> Constraint: timing-safe comparison even for unknown users
- [ ] `internal/component/authz/authz.go` -- Authorization struct with Profiles map. Authorize(profiles, section, path) returns bool.
  --> Constraint: profiles loaded from config at startup
- [ ] `internal/component/ssh/ssh.go` -- SSH server calls authz.AuthenticateUser in password callback. Users passed in at server creation.
  --> Constraint: user list is static after server start
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` -- user list under system.authentication
  --> Constraint: YANG schema defines system.authentication.user list with name, password, profile leaves
- [ ] `cmd/ze/hub/main.go` -- merges zefs users and config users into single list
  --> Constraint: user loading happens at startup in hub, passed to SSH server constructor

**Behavior to preserve:**
- Local user authentication continues to work (bcrypt password)
- Profile-based authorization unchanged
- SSH server startup and middleware stack unchanged
- Timing-safe auth (no user enumeration via timing)
- zefs user loading unchanged

**Behavior to change:**
- Introduce Authenticator interface in authz package
- Local bcrypt auth becomes one implementation of Authenticator
- TACACS+ auth becomes another implementation
- Auth chain: TACACS+ -> local fallback (configurable)
- SSH password callback uses Authenticator interface instead of direct bcrypt
- Add TACACS+ accounting for command execution
- Add TACACS+ authorization (optional, maps to ze profiles)

## Data Flow (MANDATORY)

### Authentication flow

| Step | Actor | Action |
|------|-------|--------|
| 1 | SSH client | Connects, provides username + password |
| 2 | wish server | Invokes password callback with (username, password) |
| 3 | ChainAuthenticator | Calls TacacsAuthenticator.Authenticate(username, password) |
| 4 | TacacsAuthenticator | TCP connect to first configured server (e.g., 82.219.1.113:49) |
| 5a | TACACS+ server reachable | Send AUTHEN START (PAP, username, password). Receive AUTHEN REPLY. |
| 5b | AUTHEN REPLY = PASS | Extract priv-lvl from server response, map to ze profile. Return success. |
| 5c | AUTHEN REPLY = FAIL | Return rejection immediately. Chain does NOT try local. Authentication fails. |
| 5d | Server unreachable / timeout | Try next TACACS+ server. If all servers exhausted, return connection error. |
| 6 | ChainAuthenticator | On connection error only: call LocalAuthenticator.Authenticate(username, password) |
| 7 | LocalAuthenticator | Bcrypt compare against configured user list (existing logic). Return success or rejection. |

### Authorization flow (per command)

| Step | Condition | Action |
|------|-----------|--------|
| 1 | User types command in CLI | |
| 2 | TACACS+ authorization disabled (default) | Use local profiles only: existing RBAC check via authz.Authorize(profiles, section, path) |
| 2 | TACACS+ authorization enabled | Send AUTHOR REQUEST (service=shell, cmd=X, cmd-arg=Y) to TACACS+ server |
| 3 | AUTHOR RESPONSE = PASS_ADD or PASS_REPL | Command proceeds |
| 3 | AUTHOR RESPONSE = FAIL | Command blocked |
| 3 | Server unreachable | Fall back to local profile RBAC check |

### Accounting flow

| Step | Event | Action |
|------|-------|--------|
| 1 | User executes command | Send ACCT REQUEST with START flag (username, service=shell, cmd, task_id, start_time) |
| 2 | Command completes | Send ACCT REQUEST with STOP flag (same task_id, elapsed_time, stop_time) |
| 3 | Server unreachable | Log locally, do not block command execution |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SSH callback --> Authenticator | Interface method call | [ ] |
| Authenticator --> TACACS+ server | TCP connection, encrypted TACACS+ packets | [ ] |
| CLI command --> Accounting | Dispatcher.Dispatch() hook in command.go:336 (START after auth, STOP after handler) | [ ] |

### Integration Points
- `internal/component/authz/auth.go` -- new Authenticator interface
- `internal/component/ssh/ssh.go` -- password callback uses Authenticator
- `internal/component/authz/authz.go` -- TACACS+ priv-lvl to profile mapping
- `internal/component/tacacs/` -- native TACACS+ client (RFC 8907, no external dependency)

### Architectural Verification
- [ ] No bypassed layers (SSH -> Authenticator -> TACACS+/local)
- [ ] No unintended coupling (TACACS+ client is a separate package)
- [ ] No duplicated functionality (extends existing auth, does not recreate)

## Data Model

### TacacsConfig

| Field | Type | Description |
|-------|------|-------------|
| Servers | []TacacsServer | TACACS+ servers (tried in order) |
| Timeout | uint16 | Connection timeout in seconds (default 5) |
| SourceAddress | string | Source IP for TACACS+ connections (optional) |
| Authorization | bool | Enable TACACS+ command authorization (default false) |
| Accounting | bool | Enable TACACS+ command accounting (default false) |
| PrivilegeLevelMapping | map[int]string | priv-lvl -> ze profile name mapping |

### TacacsServer

| Field | Type | Description |
|-------|------|-------------|
| Address | string | Server IP address or hostname |
| Port | uint16 | TCP port (default 49) |
| Secret | string | Shared encryption key |

### Authenticator interface (in authz package)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| Authenticate | username string, password string | AuthResult, error | Attempt authentication against this backend |

### AuthResult

| Field | Type | Description |
|-------|------|-------------|
| Authenticated | bool | Whether authentication succeeded |
| Profiles | []string | Ze authz profile names for this user |
| Source | string | Backend identifier ("tacacs", "local") |

### Auth chain

The ChainAuthenticator holds an ordered list of Authenticator backends.
It tries each in order. The chain distinguishes two failure modes:

| Failure mode | Behavior |
|-------------|----------|
| Explicit rejection (wrong password) | Stop immediately. Do NOT try next backend. Authentication fails. |
| Connection error (server unreachable, timeout) | Try next backend in chain. |

Default chain when TACACS+ configured: TacacsAuthenticator, then LocalAuthenticator.
When no TACACS+ configured: LocalAuthenticator only (current behaviour, unchanged).

## Config Syntax

Config path: `system.authentication.tacacs`

### TACACS+ server list

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `system.authentication.tacacs.server <ip>` | list, keyed by IP | - | TACACS+ servers, tried in configured order |
| `system.authentication.tacacs.server <ip>.secret` | string | (required) | Shared encryption key ($9$ encrypted format) |
| `system.authentication.tacacs.server <ip>.port` | uint16 | 49 | TCP port |
| `system.authentication.tacacs.timeout` | uint16 | 5 | Per-server connection timeout in seconds (max 300) |
| `system.authentication.tacacs.source-address` | ip-address | (none) | Source IP for outbound TACACS+ TCP connections |
| `system.authentication.tacacs.authorization` | empty leaf (presence) | disabled | Enable per-command TACACS+ authorization |
| `system.authentication.tacacs.accounting` | empty leaf (presence) | disabled | Enable command execution accounting |

### Privilege level to profile mapping

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `system.authentication.tacacs-profile.level <N>` | list, keyed by uint8 (0-15) | - | Maps TACACS+ priv-lvl to ze authz profile |
| `system.authentication.tacacs-profile.level <N>.profile` | string | (required) | Ze authorization profile name |

### Local users (existing, unchanged)

| Path | Type | Description |
|------|------|-------------|
| `system.authentication.user <name>` | list, keyed by name | Local user accounts |
| `system.authentication.user <name>.password` | string | Bcrypt hash ($2a$ format) |
| `system.authentication.user <name>.profile` | leaf-list | Authorization profile names |

### Example set commands

`set system authentication tacacs server 82.219.1.113 secret "$9$encrypted-key"`
`set system authentication tacacs server 82.219.1.114 secret "$9$encrypted-key"`
`set system authentication tacacs timeout 5`
`set system authentication tacacs source-address 82.219.0.154`
`set system authentication tacacs authorization`
`set system authentication tacacs accounting`
`set system authentication tacacs-profile level 15 profile admin`
`set system authentication tacacs-profile level 5 profile operator`
`set system authentication tacacs-profile level 1 profile read-only`
`set system authentication user exa password "$2a$10$..."`
`set system authentication user exa profile admin`

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|-----|--------------|------|
| SSH login with TACACS+ configured | --> | ChainAuthenticator -> TacacsAuthenticator -> TACACS+ server | `test/ssh/010-tacacs-auth.ci` |
| SSH login, TACACS+ server down | --> | ChainAuthenticator -> TacacsAuthenticator fails -> LocalAuthenticator | `test/ssh/011-tacacs-fallback.ci` |
| SSH login, no TACACS+ configured | --> | ChainAuthenticator -> LocalAuthenticator only (backwards compat) | `test/ssh/012-local-only.ci` |
| Command execution with TACACS+ accounting | --> | Accounting hook -> ACCT REQUEST | `test/ssh/013-tacacs-acct.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | TACACS+ server configured, valid credentials | User authenticated, priv-lvl mapped to ze profile |
| AC-2 | TACACS+ server configured, invalid credentials | TACACS+ rejects (FAIL status), authentication fails immediately. Local auth NOT attempted. |
| AC-3 | TACACS+ server unreachable, valid local credentials | Local auth succeeds (fallback) |
| AC-4 | TACACS+ server unreachable, no local user | Authentication fails |
| AC-5 | No TACACS+ configured | Local-only auth, identical to current behavior |
| AC-6 | TACACS+ priv-lvl 15 | Mapped to "admin" profile (or configured mapping) |
| AC-7 | TACACS+ priv-lvl 1 | Mapped to "read-only" profile (or configured mapping) |
| AC-8 | TACACS+ accounting enabled, command executed | ACCT start+stop records sent to server |
| AC-9 | TACACS+ authorization enabled, permitted command | AUTHOR request sent, command proceeds |
| AC-10 | TACACS+ authorization enabled, denied command | AUTHOR request sent, command blocked |
| AC-11 | Multiple TACACS+ servers, first down | Second server tried |
| AC-12 | TACACS+ auth timing | Timing-safe: failed TACACS+ auth takes similar time regardless of failure reason |
| AC-13 | `ze show tacacs` | Displays server status (reachable/unreachable), auth stats |
| AC-14 | Shared secret with $9$ encoding | Secret decrypted before use (ze's encrypted password format) |
| AC-15 | TACACS+ server returns ERROR status (not FAIL) | Treat as server unavailable, try next server or fall back to local |
| AC-16 | Single-connect negotiation | Client sets flag 0x04 on first packet. If server echoes it, reuse TCP for subsequent sessions. If not, one connection per session. |
| AC-17 | Wrong shared secret | Decryption produces body length mismatch. Treat as server error, try next server. |
| AC-18 | Unmapped priv-lvl (no matching tacacs-profile entry) | Authentication rejected. TACACS+ users with unmapped priv-lvl are denied access. Log warning naming the priv-lvl so admin can add mapping. |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthenticatorInterface` | `internal/component/authz/auth_test.go` | Authenticator interface, AuthResult struct | |
| `TestChainAuthenticator` | `internal/component/authz/auth_test.go` | Chain tries backends in order, first success wins | |
| `TestChainFallback` | `internal/component/authz/auth_test.go` | First backend fails (error), second succeeds | |
| `TestChainAllFail` | `internal/component/authz/auth_test.go` | All backends fail, authentication rejected | |
| `TestLocalAuthenticatorCompat` | `internal/component/authz/auth_test.go` | LocalAuthenticator wraps existing bcrypt logic unchanged | |
| `TestTacacsPacketEncode` | `internal/component/tacacs/packet_test.go` | TACACS+ packet header and body encoding | |
| `TestTacacsPacketEncrypt` | `internal/component/tacacs/packet_test.go` | Body encryption with shared secret (MD5 pseudo-pad) | |
| `TestTacacsAuthenStart` | `internal/component/tacacs/authen_test.go` | Authentication START packet construction | |
| `TestTacacsAuthenReply` | `internal/component/tacacs/authen_test.go` | Authentication REPLY parsing (pass/fail) | |
| `TestTacacsAuthorRequest` | `internal/component/tacacs/author_test.go` | Authorization REQUEST construction | |
| `TestTacacsAcctRequest` | `internal/component/tacacs/acct_test.go` | Accounting REQUEST construction (start/stop) | |
| `TestTacacsServerFailover` | `internal/component/tacacs/client_test.go` | Multiple servers, failover on connection error | |
| `TestTacacsTimeout` | `internal/component/tacacs/client_test.go` | Connection timeout behaviour | |
| `TestPrivLevelMapping` | `internal/component/tacacs/mapping_test.go` | priv-lvl to ze profile mapping | |
| `TestParseTacacsConfig` | `internal/component/tacacs/config_test.go` | Config JSON to TacacsConfig | |
| `TestTacacsRegistration` | `internal/component/tacacs/register_test.go` | Component registration | |
| `TestChainRejectNoFallback` | `internal/component/authz/auth_test.go` | TACACS+ explicit rejection does NOT fall through to local (AC-2) | |
| `TestTacacsSecretValidation` | `internal/component/tacacs/packet_test.go` | Wrong shared secret detected by body length mismatch (AC-17) | |
| `TestTacacsErrorStatusFallback` | `internal/component/tacacs/client_test.go` | ERROR status treated as server unavailable, try next (AC-15) | |
| `TestTacacsSingleConnect` | `internal/component/tacacs/client_test.go` | Single-connect flag negotiation (AC-16) | |
| `TestUnmappedPrivLevel` | `internal/component/tacacs/mapping_test.go` | priv-lvl with no config entry rejects auth, logs warning (AC-18) | |
| `TestTacacsPacketRoundTrip` | `internal/component/tacacs/packet_test.go` | Marshal then unmarshal produces identical packet for all types | |
| `FuzzTacacsPacketUnmarshal` | `internal/component/tacacs/packet_test.go` | Fuzz: random bytes to unmarshal, must not panic | |
| `FuzzTacacsEncryptDecrypt` | `internal/component/tacacs/packet_test.go` | Fuzz: encrypt then decrypt round-trip with random keys and bodies | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port | 1-65535 | 65535 | 0 (invalid) | 65536 (parse error) |
| Timeout | 1-300 | 300 | 0 (invalid) | 301 (rejected, too long) |
| Privilege level | 0-15 | 15 | N/A (0 is valid) | 16 (invalid per TACACS+) |
| Shared secret length | 1-256 | 256 | 0 (empty, rejected) | N/A (string) |
| Sequence number | 1-254 | 254 (0xFE) | 0 (invalid) | 255 (overflow, abort session) |
| Body length | 0-65535 | 65535 | N/A (0 is valid, e.g. empty body) | Values above 65535 rejected |
| Session ID | 1-4294967295 | full uint32 range | 0 (valid but unusual) | N/A (uint32) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TACACS+ auth | `test/ssh/010-tacacs-auth.ci` | SSH login authenticated via TACACS+ | |
| TACACS+ fallback | `test/ssh/011-tacacs-fallback.ci` | TACACS+ server down, local auth works | |
| Local only | `test/ssh/012-local-only.ci` | No TACACS+ config, existing auth unchanged | |
| Accounting | `test/ssh/013-tacacs-acct.ci` | Command execution logged to TACACS+ | |

### Test Infrastructure

Functional tests require a TACACS+ server. Implement a minimal Go test server
(`internal/component/tacacs/testserver_test.go`) that handles AUTHEN START/REPLY
for PAP, accepts/rejects based on configured credentials, and responds to AUTHOR
and ACCT requests. This is internal test code, not production. Allows all .ci
tests to run without external infrastructure.

### Future (if deferring any tests)
- TACACS+ authorization per-command tests (AC-9, AC-10) may be deferred if test server complexity is high.

## Files to Modify

- `internal/component/authz/auth.go` -- extract Authenticator interface, wrap existing bcrypt as LocalAuthenticator
- `internal/component/ssh/ssh.go` -- use Authenticator interface in password callback instead of direct authz.AuthenticateUser
- `internal/component/ssh/schema/ze-ssh-conf.yang` -- add tacacs container to system.authentication
- `cmd/ze/hub/main.go` -- build ChainAuthenticator from config (TACACS+ + local)
- `internal/component/plugin/server/command.go` -- add accounting hook interface to Dispatcher, add RemoteAddr to CommandContext, insert START/STOP calls around handler execution
- `cmd/ze/hub/infra_setup.go` -- populate RemoteAddr in CommandContext from SSH session

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/ssh/schema/ze-ssh-conf.yang` (tacacs config) |
| CLI commands | Yes | `ze show tacacs` for server status |
| Functional test | Yes | `test/ssh/01*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- TACACS+ AAA |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- tacacs config section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze show tacacs` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- TACACS+ component |
| 6 | Has a user guide page? | Yes | `docs/guide/tacacs.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | RFC 8907 |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- TACACS+ support |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- auth backend abstraction |

## Files to Create

- `internal/component/tacacs/client.go` -- TACACS+ TCP client, server failover
- `internal/component/tacacs/packet.go` -- TACACS+ packet encode/decode, encryption
- `internal/component/tacacs/authen.go` -- authentication START/REPLY handling
- `internal/component/tacacs/author.go` -- authorization REQUEST/RESPONSE handling
- `internal/component/tacacs/acct.go` -- accounting REQUEST/REPLY handling
- `internal/component/tacacs/config.go` -- config parsing
- `internal/component/tacacs/mapping.go` -- priv-lvl to profile mapping
- `internal/component/tacacs/register.go` -- component registration
- `internal/component/tacacs/authenticator.go` -- Authenticator implementation wrapping client
- `internal/component/tacacs/packet_test.go` -- packet encoding tests
- `internal/component/tacacs/authen_test.go` -- authentication tests
- `internal/component/tacacs/author_test.go` -- authorization tests
- `internal/component/tacacs/acct_test.go` -- accounting tests
- `internal/component/tacacs/client_test.go` -- client tests (failover, timeout)
- `internal/component/tacacs/mapping_test.go` -- mapping tests
- `internal/component/tacacs/config_test.go` -- config parsing tests
- `internal/component/tacacs/register_test.go` -- registration tests
- `rfc/short/rfc8907.md` -- RFC summary for TACACS+
- `test/ssh/010-tacacs-auth.ci` -- functional test
- `test/ssh/011-tacacs-fallback.ci` -- functional test
- `test/ssh/012-local-only.ci` -- functional test
- `test/ssh/013-tacacs-acct.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4-12 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Auth backend abstraction** -- Authenticator interface, ChainAuthenticator, LocalAuthenticator
   - Tests: TestAuthenticatorInterface, TestChainAuthenticator, TestChainFallback, TestChainAllFail, TestLocalAuthenticatorCompat
   - Files: authz/auth.go (refactor), ssh/ssh.go (use interface)
   - Verify: existing auth behaviour unchanged, tests pass

2. **Phase: TACACS+ wire protocol** -- packet encode/decode, encryption, authen/author/acct
   - Tests: TestTacacsPacketEncode, TestTacacsPacketEncrypt, TestTacacsAuthenStart, TestTacacsAuthenReply, TestTacacsAuthorRequest, TestTacacsAcctRequest
   - Files: tacacs/packet.go, tacacs/authen.go, tacacs/author.go, tacacs/acct.go
   - Verify: packets match RFC 8907 format

3. **Phase: TACACS+ client** -- TCP connection, server failover, timeout
   - Tests: TestTacacsServerFailover, TestTacacsTimeout
   - Files: tacacs/client.go, tacacs/authenticator.go
   - Verify: client connects, authenticates, handles failure

4. **Phase: Config + integration** -- YANG, config parsing, registration, priv-lvl mapping
   - Tests: TestParseTacacsConfig, TestPrivLevelMapping, TestTacacsRegistration
   - Files: tacacs/config.go, tacacs/mapping.go, tacacs/register.go, ze-ssh-conf.yang, hub/main.go
   - Verify: end-to-end config to authentication chain

5. **Phase: Accounting** -- command start/stop logging via Dispatcher hook
   - Hook point: `Dispatcher.Dispatch()` in `internal/component/plugin/server/command.go:336`
     All commands (SSH exec, interactive TUI, local CLI) converge at this single function.
     Insert accounting START after authorization check (line 386), STOP after handler returns (line 427).
   - Add RemoteAddr field to CommandContext (`internal/component/plugin/server/command.go`).
     Populate from SSH session in `cmd/ze/hub/infra_setup.go:133` where CommandContext is created.
   - Accounting hook interface on Dispatcher: optional, nil when accounting disabled.
     Mirrors existing authorization pattern (Dispatcher already has isAuthorized).
   - Files: command.go (hook interface + calls), infra_setup.go (RemoteAddr population),
     tacacs/acct.go (implement hook using TACACS+ ACCT REQUEST/REPLY)
   - Tests: TestAcctStartStop, TestAcctServerUnreachable (log locally, never block command)
   - Verify: accounting records sent on command execution, commands never blocked by accounting failure

6. **Functional tests** --> All .ci tests
7. **Full verification** --> `make ze-verify`
8. **Complete spec** --> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-18) has implementation |
| Correctness | TACACS+ packet format matches RFC 8907 (rfc/short/rfc8907.md); encryption correct |
| Backwards compat | No TACACS+ config = identical behaviour to today |
| Timing safety | Auth timing does not leak user existence |
| Secret handling | Shared secrets never logged, never in CLI output |
| Fallback | Local auth works when TACACS+ unreachable |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Authenticator interface | `grep "type Authenticator interface" internal/component/authz/auth.go` |
| TACACS+ client | `ls internal/component/tacacs/client.go` |
| Config in YANG | `grep "tacacs" internal/component/ssh/schema/ze-ssh-conf.yang` |
| RFC summary | `ls rfc/short/rfc8907.md` |
| Functional tests | `ls test/ssh/01*.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Secret storage | Shared secrets in config use $9$ encryption, never stored plaintext |
| Secret in logs | Shared secrets excluded from log output, CLI show, debug dumps |
| Packet encryption | TACACS+ body encryption uses MD5 pseudo-pad per RFC 8907 Section 4.6 |
| Timing attack | Authentication timing constant regardless of failure reason |
| User enumeration | No difference in response between "user not found" and "wrong password" |
| TCP security | TACACS+ connection from configured source address only |
| Fallback safety | Local fallback does not weaken security (TACACS+ FAIL is final; only unreachability/ERROR triggers fallback) |
| Connection reuse | Single-connect mode: persistent TCP must handle connection loss gracefully |
| Fuzz testing | All packet unmarshal paths fuzzed, no panics on malformed input |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| RFC compliance issue | Re-read RFC 8907 section, fix implementation |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Native TACACS+ implementation, not a Go library dependency | TACACS+ wire protocol is simple (RFC 8907 is 60 pages, mostly examples). A native implementation avoids dependency risk and gives full control. nwaples/tacplus is unmaintained (5 years, unfixed bugs); tacquito is server-focused. Study both, implement our own with ze patterns (pooled buffers, slog, context-based lifecycle). |
| 2 | Authenticator interface in authz package, not tacacs package | The interface belongs to the consumer (authz), not the provider (tacacs). Other auth backends (RADIUS, LDAP, OIDC) can implement the same interface without importing tacacs. |
| 3 | Chain authenticator with ordered fallback | Standard ISP pattern: try TACACS+ first, fall back to local if server unreachable. TACACS+ rejection (wrong password) does NOT fall through to local. Only connection failure triggers fallback. |
| 4 | priv-lvl to profile mapping via config | Different ISPs map privilege levels differently. Hardcoding 15=admin would be wrong. Config-driven mapping (level 15 -> admin, level 5 -> operator, etc.) is flexible. |
| 5 | Accounting as optional | Not all deployments need accounting. Config flag enables/disables it. When disabled, no accounting packets sent, no overhead. |
| 6 | PAP authentication type | SSH already verifies the transport (encrypted channel). TACACS+ over SSH is double-encrypted. PAP (plaintext password in TACACS+ packet, encrypted by TACACS+ body encryption) is sufficient and simplest. CHAP would require challenge-response which doesn't fit the SSH password callback model. |
| 7 | VRF-aware TACACS+ connections | TACACS+ TCP connections should use the VRF context of the SSH server. If SSH is in the management VRF, TACACS+ connections go through the management VRF too. Uses vrfnet.Dial when VRF support is available. |
| 8 | Unmapped priv-lvl denies access | TACACS+ users with a priv-lvl not in the tacacs-profile mapping are rejected. This differs from local users (no profile = admin). Rationale: an unmapped priv-lvl means the admin hasn't configured this level, so denying is safer than granting unexpected admin access. Warning logged with the priv-lvl value. |
| 9 | Accounting hooks in Dispatcher.Dispatch() | All commands converge at Dispatcher.Dispatch() in command.go:336. Accounting START inserted after authorization check (line 386), STOP after handler returns (line 427). Single hook point covers SSH exec, interactive TUI, and local CLI. Accounting failures are logged locally, never block command execution. |

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

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-tacacs.md`
- [ ] Summary included in commit
