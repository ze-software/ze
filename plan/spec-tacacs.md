# Spec: tacacs -- TACACS+ authentication, authorization, and accounting

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
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

The VyOS LNS config:
```
set system login tacacs server 82.219.1.113 key 'EXA-TACACS-KEY'
```

This authenticates SSH logins against the TACACS+ server at 82.219.1.113, allowing
centralised user management across the Exa network fleet. Without TACACS+, each device
needs local user accounts maintained individually.

### What TACACS+ provides

| Function | Description | VyOS LNS usage |
|----------|-------------|----------------|
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

**Key insights:**
- TACACS+ uses TCP (port 49), not UDP like RADIUS
- Packet body is encrypted with MD5-based stream cipher using shared secret
- Authentication flow: START -> REPLY (pass/fail/getdata/getuser/error)
- Authorization flow: REQUEST -> RESPONSE (pass-add/pass-repl/fail/error)
- Accounting flow: REQUEST -> REPLY (success/error)
- Go TACACS+ libraries: github.com/facebookincubator/tacquito (Facebook), github.com/nwaples/tacplus
- wish SSH password callback receives (ctx, password) and returns bool
- ze's authz profiles map naturally to TACACS+ privilege levels (priv-lvl=15 -> admin, priv-lvl=1 -> read-only)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/authz/auth.go` -- AuthenticateUser takes []UserConfig, username, credential. Returns bool. No interface, no extensibility. Bcrypt only.
  --> Constraint: timing-safe comparison even for unknown users
- [ ] `internal/component/authz/authz.go` -- Authorization struct with Profiles map. Authorize(profiles, section, path) returns bool.
  --> Constraint: profiles loaded from config at startup
- [ ] `internal/component/ssh/ssh.go` -- SSH server calls authz.AuthenticateUser in password callback. Users passed in at server creation.
  --> Constraint: user list is static after server start
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` -- user list under system.authentication
- [ ] `cmd/ze/hub/main.go` -- merges zefs users and config users into single list

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

```
SSH client connects
    |
    v
wish password callback(username, password)
    |
    v
Authenticator.Authenticate(username, password)
    |
    ├── TacacsAuthenticator: TCP connect to 82.219.1.113:49
    │   ├── AUTHEN START (username, PAP password)
    │   ├── AUTHEN REPLY (pass/fail)
    │   ├── If pass: extract priv-lvl, map to ze profile
    │   └── If fail or timeout: fall through
    │
    └── LocalAuthenticator: bcrypt compare (existing)
        ├── If match: use local user's profile
        └── If no match: reject
```

### Authorization flow (per command)

```
User types command in CLI
    |
    v
authz.Authorize(user.profiles, section, path)
    |
    ├── Local profiles: existing RBAC check
    │
    └── If TACACS+ authorization enabled:
        AUTHOR REQUEST(username, cmd, cmd-arg)
        AUTHOR RESPONSE(pass-add/pass-repl/fail)
```

### Accounting flow

```
User executes command
    |
    v
ACCT REQUEST(username, cmd, start, task_id)
    ... command runs ...
ACCT REQUEST(username, cmd, stop, task_id, elapsed)
```

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SSH callback --> Authenticator | Interface method call | [ ] |
| Authenticator --> TACACS+ server | TCP connection, encrypted TACACS+ packets | [ ] |
| CLI command --> Accounting | Hook after command execution | [ ] |

### Integration Points
- `internal/component/authz/auth.go` -- new Authenticator interface
- `internal/component/ssh/ssh.go` -- password callback uses Authenticator
- `internal/component/authz/authz.go` -- TACACS+ priv-lvl to profile mapping
- New TACACS+ client library (dependency)

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

### Authenticator interface

```
type Authenticator interface {
    Authenticate(username, password string) (AuthResult, error)
}

type AuthResult struct {
    Authenticated bool
    Profiles      []string  // ze authz profiles for this user
    Source        string    // "tacacs", "local"
}
```

### Auth chain

The auth chain tries backends in order. First success wins. If all fail, reject:

```
type ChainAuthenticator struct {
    Backends []Authenticator
}
```

Default chain: [TacacsAuthenticator, LocalAuthenticator]
If no TACACS+ configured: [LocalAuthenticator] (current behaviour)

## Config Syntax

```
system {
    authentication {
        # TACACS+ servers (tried in order)
        tacacs {
            server 82.219.1.113 {
                secret "$9$encrypted-shared-key"
                port 49
            }
            server 82.219.1.114 {
                secret "$9$encrypted-shared-key"
                port 49
            }
            timeout 5
            source-address 82.219.0.154
            authorization
            accounting
        }

        # Privilege level to profile mapping
        tacacs-profile {
            level 15 { profile admin; }
            level 5 { profile operator; }
            level 1 { profile read-only; }
        }

        # Local users (fallback)
        user exa {
            password "$2a$10$..."
            profile admin
        }
    }
}
```

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
| AC-2 | TACACS+ server configured, invalid credentials | TACACS+ rejects, local auth attempted, both fail -> rejected |
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

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port | 1-65535 | 65535 | 0 (invalid) | 65536 (parse error) |
| Timeout | 1-300 | 300 | 0 (invalid) | 301 (rejected, too long) |
| Privilege level | 0-15 | 15 | N/A (0 is valid) | 16 (invalid per TACACS+) |
| Shared secret length | 1-256 | 256 | 0 (empty, rejected) | N/A (string) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TACACS+ auth | `test/ssh/010-tacacs-auth.ci` | SSH login authenticated via TACACS+ | |
| TACACS+ fallback | `test/ssh/011-tacacs-fallback.ci` | TACACS+ server down, local auth works | |
| Local only | `test/ssh/012-local-only.ci` | No TACACS+ config, existing auth unchanged | |
| Accounting | `test/ssh/013-tacacs-acct.ci` | Command execution logged to TACACS+ | |

### Future (if deferring any tests)
- TACACS+ authorization tests require a running TACACS+ server in CI. May use a Go-based test server.

## Files to Modify

- `internal/component/authz/auth.go` -- extract Authenticator interface, wrap existing bcrypt as LocalAuthenticator
- `internal/component/ssh/ssh.go` -- use Authenticator interface in password callback instead of direct authz.AuthenticateUser
- `internal/component/ssh/schema/ze-ssh-conf.yang` -- add tacacs container to system.authentication
- `cmd/ze/hub/main.go` -- build ChainAuthenticator from config (TACACS+ + local)

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

5. **Phase: Accounting** -- command start/stop logging
   - Files: tacacs/acct.go (extend), ssh/ssh.go or CLI hooks
   - Verify: accounting records sent on command execution

6. **Functional tests** --> All .ci tests
7. **Full verification** --> `make ze-verify`
8. **Complete spec** --> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-14) has implementation |
| Correctness | TACACS+ packet format matches RFC 8907; encryption correct |
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
| Fallback safety | Local fallback does not weaken security (TACACS+ rejection is final; only unreachability triggers fallback) |
| Connection reuse | Single-connect mode: persistent TCP must handle connection loss gracefully |

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
| 1 | Native TACACS+ implementation, not a Go library dependency | TACACS+ wire protocol is simple (RFC 8907 is 60 pages, mostly examples). A native implementation avoids dependency risk and gives full control. Packet format is header + encrypted body, three message types. |
| 2 | Authenticator interface in authz package, not tacacs package | The interface belongs to the consumer (authz), not the provider (tacacs). Other auth backends (RADIUS, LDAP, OIDC) can implement the same interface without importing tacacs. |
| 3 | Chain authenticator with ordered fallback | Standard ISP pattern: try TACACS+ first, fall back to local if server unreachable. TACACS+ rejection (wrong password) does NOT fall through to local. Only connection failure triggers fallback. |
| 4 | priv-lvl to profile mapping via config | Different ISPs map privilege levels differently. Hardcoding 15=admin would be wrong. Config-driven mapping (level 15 -> admin, level 5 -> operator, etc.) is flexible. |
| 5 | Accounting as optional | Not all deployments need accounting. Config flag enables/disables it. When disabled, no accounting packets sent, no overhead. |
| 6 | PAP authentication type | SSH already verifies the transport (encrypted channel). TACACS+ over SSH is double-encrypted. PAP (plaintext password in TACACS+ packet, encrypted by TACACS+ body encryption) is sufficient and simplest. CHAP would require challenge-response which doesn't fit the SSH password callback model. |
| 7 | VRF-aware TACACS+ connections | TACACS+ TCP connections should use the VRF context of the SSH server. If SSH is in the management VRF, TACACS+ connections go through the management VRF too. Uses vrfnet.Dial when VRF support is available. |

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
- [ ] AC-1..AC-14 all demonstrated
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
