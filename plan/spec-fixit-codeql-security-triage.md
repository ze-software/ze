# Spec: fixit-codeql-security-triage

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/fail-closed-guards.md`, `ai/rules/exact-or-reject.md`, `ai/rules/no-layering.md`
4. `internal/plugins/provision/main.go`, `internal/install/disk/rescue_linux.go`,
   `internal/appliance/cmd_config_push.go`, `internal/component/ike/eap/peer.go`,
   `internal/component/web/handler_config_commit.go`

## Task

GitHub code scanning (`ze-software/ze`, CodeQL Advanced, `.github/workflows/codeql.yml`)
reports 86 open alerts. A full triage against the producing source classified them:

| Class | Count | Disposition |
|-------|-------|-------------|
| Real defect, fix in this spec | 5 | code change |
| Found during triage, CodeQL missed it | 1 | code change |
| Protocol-mandated cryptography | 6 | dismiss `won't fix` |
| Config-tree narrowing, guard is upstream in the parser | 49 | harden locally, then dismiss `false positive` |
| False positive, guard is local | 18 | dismiss `false positive` |
| Test / build tooling, not linked into `ze` | 8 | dismiss `used in tests` |

Goals:

1. Remove the unsalted digest of an operator secret from the PXE install network.
2. Make the appliance config push authenticate the host it talks to.
3. Close the EAP-TLS fail-open path when no trust anchor is configured.
4. Close the `HX-Current-URL` open redirect.
5. Reject rather than truncate out-of-range operator input in two CLI paths.
6. Give the OSPF and IS-IS config parsers a bounded numeric helper so the
   narrowing guard is local, matching what `vrrp` already does.
7. Leave the scanning page in a fully triaged state: every remaining alert
   dismissed with a reason and a comment naming the guard or the RFC.

Non-goals: changing which authentication protocols Ze offers (MS-CHAPv2, CHAP-MD5
and RADIUS MD5 stay, the RFCs mandate those primitives); rewriting the config
validator; suppressing CodeQL queries wholesale.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/fail-closed-guards.md` - three of the six fixes are guards that fail open
  → Constraint: on a miss, an unmapped input, an empty set, or an error, deny; a guard that cannot deny MUST say so
  → Constraint: drive the guard's test from the entry point that triggers it, never the helper alone
- [ ] `ai/rules/exact-or-reject.md` - truncation of operator input is silent approximation
  → Constraint: if the implementation cannot deliver exactly what the config asks for, verify/commit fails with a clear error
- [ ] `ai/rules/no-layering.md` - the rescue credential changes shape
  → Constraint: DELETE the old `shell-auth-sha256` path first, then implement the replacement; no alias, no fallback
- [ ] `ai/rules/compatibility.md` - whether the config leaf may be renamed
  → Decision: Ze is pre-release and this leaf is not part of the plugin SDK surface, so a clean rename is correct
- [ ] `ai/rules/initrd-no-external-tools.md` - what the installer initrd can link
  → Constraint: the initrd is a Go binary (`cmd/ze-installer/main.go:7` imports `internal/install/disk`), not busybox; the "no bcrypt in busybox" rationale at `internal/plugins/provision/main.go:347-351` is stale

### RFC Summaries
- [ ] `rfc/short/rfc2759.md` - MS-CHAPv2 mandates SHA1 and DES
  → Constraint: alerts #60/#61/#62 are conformance, not defects
- [ ] `rfc/short/rfc5216.md` - EAP-TLS peer certificate validation
  → Constraint: §5.3 the peer validates the authenticator chain against its configured trust anchor; with no anchor there is nothing to validate against, so the config is the thing that must be rejected

**Key insights:**
- The 49 config-tree integer alerts are guarded three layers up, at the config
  file parser (`internal/component/config/parser.go:266` →
  `internal/component/config/schema.go:787-805`), not in the plugin. Verified
  empirically: `bin/ze config validate` rejects `tag 4294967296` with
  `line 4: invalid value for tag: invalid uint32: "4294967296"` (exit 1).
- goyang gives every builtin integer type its natural range even with no explicit
  `range` statement (`vendor/github.com/openconfig/goyang/pkg/yang/yangtype.go:98-118`),
  so a bare `type uint32` leaf is still bounded.
- The YANG declared width equals the Go narrowing width at every checked site
  (hello-interval/dead-interval/transmit-delay `uint16`, priority `uint8`), so no
  valid config is truncated today. The exposure is that the guard is remote.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/provision/main.go` - `shellAuthHash` (`:340-356`) returns unsalted `sha256(adminPassword)`; called at `:103`, emitted into the generated config at `:206` as `shell-auth-sha256`
- [ ] `internal/plugins/imageserver/config.go` - `ShellAuthSHA256` field (`:24`), parsed at `:85`, validated as 64 lowercase hex at `:110-112`
- [ ] `internal/plugins/imageserver/handler.go` - `shellAuth` field (`:30`, set `:40`); appended to the iPXE kernel cmdline as `ze.shell-auth=` at `:196-198`; the iPXE script is served over unauthenticated plain HTTP by design (`:251-255`)
- [ ] `internal/plugins/imageserver/yang/ze-image-server-conf.yang:56-59` - `leaf shell-auth-sha256 { type string; }`, no pattern
- [ ] `internal/install/disk/cmdline.go:20,57-58` - `InstallConfig.ShellAuth` parsed from `ze.shell-auth`
- [ ] `internal/install/disk/validate.go:108-118` - `validateShellAuth` requires exactly 64 lowercase hex chars
- [ ] `internal/install/disk/run.go:85-88` - validation call site
- [ ] `internal/install/disk/rescue_linux.go` - `selectFatalBranch` (`:49-57`) three-branch policy; `checkPassword` (`:59-63`) compares `sha256(typed)`; `gateWithPassword` (`:144-160`) prompts `admin password:` up to `rescueMaxAttempts`
- [ ] `internal/appliance/cmd_config_push.go:146-198` - `sshExecReal` dials with agent pubkey auth and `HostKeyCallback: ssh.InsecureIgnoreHostKey()` (`:169`), unconditionally, to a remote appliance
- [ ] `internal/core/ssh/client/client.go:491-501` - `hostKeyCallback` returns the insecure callback only for `127.0.0.1`/`::1`/`localhost` or under `ze.ssh.insecure`, else errors. This is the shape to match
- [ ] `internal/component/ike/eap/peer.go:355-392` - EAP-TLS client config; `InsecureSkipVerify: true` at `:378`, explicit chain verification attached only `if rootCAs != nil` (`:383-386`)
- [ ] `internal/component/web/handler_config_commit.go:257-280` - `parentFromCurrentURL` reads `HX-Current-URL` (falls back to `Referer`), strips scheme+host, trims the last segment, returns the result unvalidated; used as a redirect target at `:233`
- [ ] `internal/component/web/auth.go:330-335` - `sanitizeReturnTo` rejects a target that is empty, does not start with `/`, starts with `//`, or starts with `/\`
- [ ] `internal/component/bgp/plugins/filter_irr/command.go:212-225` - `updateASN` parses 64-bit via `readUint`, then `uint32(v)` at `:219`
- [ ] `internal/component/bgp/plugins/cmd/announce/announce.go:154-163` - `parseDuration` returns `time.Duration(secs) * time.Second` with no bound
- [ ] `internal/plugins/vrrp/groups.go:655-676` - `asUint(v any, max uint64)` parses then enforces `n > max`; the reference shape for the OSPF/IS-IS helper
- [ ] `internal/plugins/ospf/config.go:1682-1698` and `internal/plugins/isis/config.go:207-223` - `configNumber(v any) (uint64, bool)`, unbounded

**Behavior to preserve:**
- The installer's three-branch fatal policy (`gated` / `ungated` / `reboot`) and
  which branch each `(shellAuth, source)` pair selects.
- `rescueMaxAttempts`, the echo-off terminal handling, and the marker strings the
  QEMU evidence harness matches on (`admin password:` becomes a different prompt,
  see Behavior to change; `authenticated` / `incorrect` / `too many attempts` stay).
- Appliance config push semantics: agent-based pubkey auth, parallel push, dry-run.
- Every EAP-TLS session that today configures a CA keeps working unchanged.
- Every currently valid OSPF/IS-IS config parses to the same values.

**Behavior to change:**
- The rescue credential is a token generated at provision time, not the admin
  password. The prompt becomes `rescue token:`. The cmdline value becomes
  `<saltHex>:<argon2idHex>`, not a bare sha256.
- `ze appliance config push` to a host absent from `~/.ssh/known_hosts` now fails
  with a remediation message instead of connecting.
- An EAP-TLS peer config with no CA certificate is now rejected at config verify.
- `update bgp irr asn <asn>` above 2^32-1 is rejected instead of truncated.

## Data Flow (MANDATORY)

### Entry Point
Four independent entry points, one per fix area:
1. `ze plugin provision ...` (operator, generates the install-server config)
2. `ze appliance config push <name>` (operator, over SSH to a remote appliance)
3. `ike { ... eap { tls { ... } } }` config (operator, no CA leaf set)
4. `POST /config/discard/` with an `HX-Current-URL` header (web client)

### Transformation Path
**Rescue credential (1):**
1. `provision.Run` generates a random rescue token and a random 16-byte salt
2. `rescueAuthValue(token, salt)` returns `hex(salt) + ":" + hex(argon2id(token, salt))`
3. Emitted into the generated config as `image-server { rescue-auth "<value>"; }`
4. `imageserver` config parse validates the shape, stores it
5. `imageserver` handler appends `ze.rescue-auth=<value>` to the iPXE kernel cmdline
6. Installer parses it into `InstallConfig.RescueAuth`, `validateRescueAuth` checks shape
7. On a fatal install error, `gateWithPassword` prompts and compares
   `argon2id(typed, salt)` to the stored digest in constant time

**Host key (2):** `sshExecReal` → `knownhosts.New(~/.ssh/known_hosts)` → `ssh.Dial`;
unknown host returns an error naming `ssh-keyscan`, never dials further.

**EAP-TLS (3):** config verify → reject when `eap.tls` is configured with an empty
`CACertPEM`; `peer.go` keeps `InsecureSkipVerify` for the hostname check only, and
its `rootCAs == nil` branch becomes unreachable by construction.

**Web redirect (4):** `parentFromCurrentURL` → `sanitizeReturnTo` → `htmxRedirect`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Install network ↔ installer | iPXE kernel cmdline over unauthenticated HTTP | [ ] |
| Operator host ↔ appliance | SSH, agent pubkey auth | [ ] |
| Config parser ↔ plugin | validated tree delivered as strings | [ ] |
| Browser ↔ web handler | `HX-Current-URL` / `Referer` request headers | [ ] |

### Integration Points
- `internal/core/diagnostic/codes.go` - a new code for the EAP-TLS rejection so
  `ze explain` can carry the remediation (`ai/rules/error-messages.md`).
- `scripts/evidence/effective-install-scenarios-qemu.py:385-395` - the AC-7 rescue
  scenarios drive the real prompt and must move to the token.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (n/a, none of these are hot paths)
- [ ] Registration over hardcoding - the new diagnostic code registers in the existing registry

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The installer initrd can link argon2 | `cmd/ze-installer/main.go:7` imports `internal/install/disk`; `vendor/golang.org/x/crypto/argon2/` present; `vendor/modules.txt:370` | Fall back to bcrypt (also vendored) | `GOOS=linux go vet -tags 'linux ze_installer' ./internal/install/disk/` exit 0; whole `install/disk` suite green | confirmed |
| A-2 | argon2id at m=64MB is safe in the initrd | initrd runs as PID 1 with the machine's full RAM as a ramdisk | OOM on small appliances at the rescue prompt | QEMU rescue scenario (AC-7) at the harness default RAM | unvalidated -- NOT YET RUN, see R-2 |
| A-3 | `golang.org/x/crypto/ssh/knownhosts` needs no new module | `golang.org/x/crypto` already direct in `go.mod`; only the subpackage is missing from `vendor/` | Ask the user before adding a module | after `go mod vendor`: `go.mod` byte-identical, `vendor/modules.txt` gained exactly one subpackage line | confirmed |
| A-4 | No shipped code besides the flagged sites reads `shell-auth-sha256` | grep across `internal/`, `cmd/`, `test/`, `docs/` | A consumer breaks on the rename | full-tree grep after the rename leaves only two `test-relax:` provenance comments and one assertion that the leaf is gone | confirmed |
| A-5 | The config parser validates every entry point that reaches a plugin parser, not just the file path | `parser.go:266`; the agent-tooling rule lists the five validation boundaries | The 49 dismissals would be wrong | file path proven empirically (`ze config validate` rejects an out-of-range `tag`); hub-push and web-commit paths NOT traced | unvalidated -- blocks the Phase 3 dismissals, not the fixes that landed |
| A-6 | Dismissing an alert on GitHub is reversible | GitHub code scanning API exposes `state: open` to reopen | A wrong dismissal is permanent | reopen one test alert and confirm | unvalidated -- Phase 9 not started |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rescue token rename desynchronises the QEMU install evidence harness, and AC-7/7b/7c go red | `make ze-qemu-install-scenarios` fails at the rescue prompt | LIVE. Harness updated in the same change (`RESCUE_TOKEN`/`RESCUE_AUTH`/`TOKEN_PROMPT`), and `TestValuePinnedVector` fails loudly if the argon2 parameters drift from the harness constant. The QEMU run itself has NOT been executed yet |
| R-2 | argon2 in the installer inflates the initrd past a boot-time size limit | `bin/ze-installer-<arch>` grows sharply | Measure before and after; bcrypt is the smaller fallback |
| R-3 | `knownhosts` refusal breaks an operator's existing push workflow with no warning | first push after upgrade fails | The error names the exact `ssh-keyscan` line; document in the appliance guide |
| R-4 | The OSPF/IS-IS `configUint` refactor silently changes a default when a value is now rejected | OSPF/IS-IS functional tests fail | Bound each call at the leaf's YANG maximum, not an invented one; the parser already rejects anything larger, so no currently valid config can newly fail |
| R-5 | Bulk dismissal hides a future real alert because the fingerprint is reused | a new alert appears already-dismissed | Dismiss per alert number with an individual comment, never a query-level suppression |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| iPXE cmdline `ze.rescue-auth=` on install failure | → | `gateWithPassword` / `checkRescueToken` (`rescue_linux.go`) | `scripts/evidence/effective-install-scenarios-qemu.py` AC-7 (gated shell accepts the token, rejects a wrong one) |
| `ze appliance config push <name>` against an unknown host | → | `sshExecReal` knownhosts callback | `TestSSHExecRefusesUnknownHost` (`internal/appliance/cmd_config_push_test.go`) |
| `ike { eap { tls { } } }` config with no CA | → | EAP-TLS config verify | `test/parse/eap-tls-requires-ca.ci` (`expect=exit:code=1`) |
| `POST /config/discard/` with `HX-Current-URL: //evil.com/a/b` | → | `parentFromCurrentURL` → `sanitizeReturnTo` | `TestConfigDiscardRedirectIsSameOrigin` (`internal/component/web/handler_config_commit_test.go`) |
| `update bgp irr asn 4294967296` | → | `irrPlugin.updateASN` | `test/plugin/irr-asn-range.ci` (`expect=stdout:contains=invalid ASN`) |
| OSPF config with an at-maximum leaf value | → | `configUint` | `TestConfigUintRejectsAboveMax` (`internal/plugins/ospf/config_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze plugin provision` run | Generated config carries `rescue-auth "<saltHex>:<digestHex>"`; the admin password is not recoverable from it; the rescue token is printed once to the operator |
| AC-2 | Installer fatal error with `ze.rescue-auth` set, correct token typed | Rescue shell opens, `authenticated` printed |
| AC-3 | Installer fatal error, wrong token typed `rescueMaxAttempts` times | `too many attempts`, no shell |
| AC-4 | Grep the shipped tree for `shell-auth-sha256` / `shellAuthHash` after the change | Zero matches outside `plan/learned/` (old path deleted, not aliased) |
| AC-5 | `ze appliance config push` to a host not in `~/.ssh/known_hosts` | Exits non-zero, message names the host, the file, and the `ssh-keyscan` remediation; no TCP session established |
| AC-6 | `ze appliance config push` to a host present in `~/.ssh/known_hosts` with a matching key | Push succeeds exactly as before |
| AC-7 | `ze appliance config push` where the presented key differs from the pinned one | Exits non-zero naming a host key mismatch |
| AC-8 | EAP-TLS peer configured with no CA certificate | `ze config verify` fails with a registered diagnostic code and a remediation naming the CA leaf |
| AC-9 | EAP-TLS peer configured with a CA certificate | Handshake behaves exactly as today; chain verified by `verifyServerChain` |
| AC-10 | `POST /config/discard/` with `HX-Current-URL: //evil.com/a/b` | Redirect target starts with a single `/`; never `//` or a scheme |
| AC-11 | `update bgp irr asn 4294967296` | Error `invalid ASN`; no lookup performed with a truncated value |
| AC-12 | `announce ... 99999999999999999999s`-scale seconds | Rejected with a range error, never a negative duration |
| AC-13 | OSPF/IS-IS config value above the leaf's YANG maximum reaching `configUint` directly | `configUint` returns `false`; the field keeps its default; no truncated value is stored |
| AC-14 | Every currently valid OSPF/IS-IS config in `test/` | Parses to identical values (no behavior change for valid input) |
| AC-15 | GitHub code scanning after the work | Zero open alerts; every dismissal carries a reason and a comment naming the guard `file:line` or the RFC section |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Provisions an install server, notes the rescue token, PXE-boots a client that fails, opens a rescue shell | `provision` → generated config → `imageserver` cmdline → installer `run.go` → `fatalInitrd` → `gateWithPassword` | `effective-install-scenarios-qemu.py` AC-7 |
| 2 | Pushes config to a known appliance, then to an unknown one | `ze appliance config push` → `sshExecReal` → knownhosts → `ssh.Dial` | `TestSSHExecRefusesUnknownHost` + existing push tests |
| 3 | Configures EAP-TLS without a CA and runs `ze config verify` | config parse → verify → diagnostic code | `test/parse/eap-tls-requires-ca.ci` |
| 4 | Uses the web config editor and discards a change | `POST /config/discard/` → `parentFromCurrentURL` → `sanitizeReturnTo` → `htmxRedirect` | `TestConfigDiscardRedirectIsSameOrigin` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRescueAuthValueRoundTrip` | `internal/plugins/provision/main_test.go` | salt+digest encodes and re-verifies | |
| `TestRescueAuthValueIsSalted` | `internal/plugins/provision/main_test.go` | two calls with the same token differ | |
| `TestCheckRescueTokenRejectsWrong` | `internal/install/disk/rescue_test.go` | wrong token fails, constant-time path used | |
| `TestValidateRescueAuthShape` | `internal/install/disk/validate_test.go` | boundary table below | |
| `TestSSHExecRefusesUnknownHost` | `internal/appliance/cmd_config_push_test.go` | AC-5, no dial attempted | |
| `TestSSHExecRejectsChangedHostKey` | `internal/appliance/cmd_config_push_test.go` | AC-7 | |
| `TestEAPTLSConfigRequiresCA` | `internal/component/ike/eap/config_test.go` | AC-8, registered code returned | |
| `TestParentFromCurrentURLRejectsProtocolRelative` | `internal/component/web/handler_config_commit_test.go` | AC-10, table of hostile header values | |
| `TestConfigDiscardRedirectIsSameOrigin` | `internal/component/web/handler_config_commit_test.go` | AC-10 from the handler entry point, per `fail-closed-guards.md` | |
| `TestUpdateASNRejectsOutOfRange` | `internal/component/bgp/plugins/filter_irr/command_test.go` | AC-11 | |
| `TestParseDurationRejectsOverflow` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | AC-12 | |
| `TestConfigUintRejectsAboveMax` | `internal/plugins/ospf/config_test.go`, `internal/plugins/isis/config_test.go` | AC-13 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `rescue-auth` salt hex length | 32 chars | 32 | 31 | 33 |
| `rescue-auth` digest hex length | 64 chars | 64 | 63 | 65 |
| `update bgp irr asn` | 0..4294967295 | 4294967295 | N/A | 4294967296 |
| `announce` duration seconds | 0..(MaxInt64/1e9) | 9223372036 | N/A | 9223372037 |
| OSPF `priority` (`configUint` max 255) | 0..255 | 255 | N/A | 256 |
| OSPF `hello-interval` (max 65535) | 1..65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `eap-tls-requires-ca.ci` | `test/parse/eap-tls-requires-ca.ci` | EAP-TLS without a CA is rejected at parse | |
| `irr-asn-range.ci` | `test/plugin/irr-asn-range.ci` | `update bgp irr asn` above 2^32 errors | |
| AC-7 / AC-7b / AC-7c | `scripts/evidence/effective-install-scenarios-qemu.py` | rescue token gates the shell (updated, not new) | |

### Interop Tests
Not applicable. No wire-visible protocol behavior changes: the MS-CHAPv2, CHAP-MD5
and RADIUS MD5 primitives that CodeQL flags stay exactly as the RFCs require, and
the EAP-TLS change is a config-verify rejection, not a handshake change.

### Future
None. No test is deferred.

## Files to Modify

- `internal/plugins/provision/main.go` - replace `shellAuthHash` with rescue-token generation, print the token
- `internal/plugins/imageserver/config.go` - `ShellAuthSHA256` → `RescueAuth`, new shape validation
- `internal/plugins/imageserver/handler.go` - cmdline arg rename, comment update
- `internal/plugins/imageserver/yang/ze-image-server-conf.yang` - leaf rename + `pattern` constraint (currently bare `type string`)
- `internal/install/disk/cmdline.go` - `ShellAuth` → `RescueAuth`, key rename
- `internal/install/disk/validate.go` - `validateShellAuth` → `validateRescueAuth`
- `internal/install/disk/run.go` - validation call site
- `internal/install/disk/rescue_linux.go` - `checkPassword` → `checkRescueToken` (argon2id), prompt string
- `internal/install/disk/initrd_linux.go` - log field rename
- `internal/appliance/cmd_config_push.go` - knownhosts callback
- `internal/component/ike/eap/peer.go` - comment update once the config guard exists
- `internal/component/ike/` (config verify site) - reject EAP-TLS with no CA
- `internal/core/diagnostic/codes.go` - register the EAP-TLS code
- `internal/component/web/handler_config_commit.go` - sanitize the redirect target
- `internal/component/bgp/plugins/filter_irr/command.go` - bound the ASN
- `internal/component/bgp/plugins/cmd/announce/announce.go` - bound the duration
- `internal/plugins/ospf/config.go`, `internal/plugins/ospf/sr_config.go` - `configUint(v, max)` at each narrowing site
- `internal/plugins/isis/config.go` - same
- `scripts/evidence/effective-install-scenarios-qemu.py` - rescue token instead of password
- `docs/` - see Documentation Update Checklist

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] yes | `internal/plugins/imageserver/yang/ze-image-server-conf.yang` leaf rename |
| YANG validation constraints | [ ] yes | add `pattern` for `<32 hex>:<64 hex>`; the leaf is bare `type string` today |
| YANG custom validators | [ ] no | native `pattern` is sufficient |
| CLI commands/flags | [ ] no | no new commands |
| CLI grammar | [ ] no | no new commands |
| Editor autocomplete | [ ] no | opaque credential value |
| Functional test for new RPC/API | [ ] yes | `test/parse/eap-tls-requires-ca.ci`, `test/plugin/irr-asn-range.ci` |
| Pipe completeness | [ ] no | no new output-producing command |
| Env var registration | [ ] no | no new `environment/` leaves |
| Doctor check for runtime dependencies | [ ] no | no new file path, socket, port, or binary; `~/.ssh/known_hosts` is read on demand by an operator CLI, not a daemon dependency |
| Prometheus counters/metrics | [ ] no | no new observable daemon state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` if the rescue token is listed |
| 2 | Config syntax changed? | [ ] yes | image-server `rescue-auth` leaf |
| 3 | CLI command added/changed? | [ ] no | none added |
| 4 | API/RPC added/changed? | [ ] no | none |
| 5 | Plugin added/changed? | [ ] yes | imageserver, provision |
| 6 | Has a user guide page? | [ ] yes | appliance push (known_hosts requirement), PXE install (rescue token) |
| 7 | Wire format changed? | [ ] no | none |
| 8 | Plugin SDK/protocol changed? | [ ] no | none |
| 9 | RFC behavior implemented or newly proven? | [ ] no | EAP-TLS change is config rejection, not protocol behavior |
| 10 | Test infrastructure changed? | [ ] yes | the QEMU install scenarios harness |
| 11 | Affects daemon comparison? | [ ] no | |
| 12 | Internal architecture changed? | [ ] no | |
| 13 | Route metadata keys? | [ ] no | |
| 14 | Prometheus counters? | [ ] no | |
| 15 | Registered inventory changed? | [ ] yes | new diagnostic code appears in `ze explain` inventory |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors naming every file in Files to Modify |
| 17 | Existing docs show examples for this area? | [ ] check | any `shell-auth-sha256` example in `docs/` |

## Files to Create
- `internal/component/web/handler_config_commit_test.go` - if absent
- `internal/appliance/cmd_config_push_test.go` - if absent
- `test/parse/eap-tls-requires-ca.ci`
- `test/plugin/irr-asn-range.ci`
- `tmp/codeql-dismiss-<SESSION>.py` - the dismissal driver (scratch, not committed)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify / Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | |
| 8. Re-verify | |
| 9. Repeat 6-8 | |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - create every entry point and a failing test
   - Tests: all six rows of the Wiring Test table, written to fail
   - Files: test files from Files to Create, plus stub signatures
   - Verify: each test fails because the guard is absent, not because it cannot compile

2. **Phase: Cheap guards** - the four one-liner fixes, lowest risk first
   - Tests: `TestParentFromCurrentURLRejectsProtocolRelative`, `TestConfigDiscardRedirectIsSameOrigin`, `TestUpdateASNRejectsOutOfRange`, `TestParseDurationRejectsOverflow`
   - Files: `handler_config_commit.go`, `filter_irr/command.go`, `announce.go`
   - Verify: tests go green; `make ze-lint-changed` clean

3. **Phase: OSPF/IS-IS `configUint`** - local bound at every narrowing site
   - Tests: `TestConfigUintRejectsAboveMax`; existing OSPF/IS-IS suites prove AC-14
   - Files: `ospf/config.go`, `ospf/sr_config.go`, `isis/config.go`
   - Verify: 49 alert sites now carry a local bound; no valid config changes value

4. **Phase: EAP-TLS trust anchor** - reject at config verify, register the code
   - Tests: `TestEAPTLSConfigRequiresCA`, `test/parse/eap-tls-requires-ca.ci`
   - Files: ike config verify, `diagnostic/codes.go`, `eap/peer.go` comment
   - Verify: `ze explain <code>` returns the remediation

5. **Phase: appliance known_hosts** - `go mod vendor` for the subpackage, then the callback
   - Tests: `TestSSHExecRefusesUnknownHost`, `TestSSHExecRejectsChangedHostKey`
   - Files: `cmd_config_push.go`, `vendor/`
   - Verify: A-3 confirmed (no new module in `vendor/modules.txt`)

6. **Phase: rescue token** - the largest change; delete the sha256 path first per `no-layering.md`
   - Tests: provision, validate, and rescue unit tests; then the QEMU scenarios
   - Files: provision, imageserver (+YANG), install/disk, evidence harness
   - Verify: AC-4 grep is clean; `make ze-qemu-install-scenarios` green (R-1)

7. **Functional tests** - both `.ci` files green, mutation-verified per `ai/rules/functional-test-gate.md`

8. **Full verification** - `make ze-verify`

9. **Dismissal pass** - only after the code fixes land, so the fixed alerts close on
   their own rather than being dismissed. Drive the GitHub API per alert number with
   an individual reason and comment (R-5). Confirm AC-15.

10. **Complete spec** - audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every user story path works end to end |
| Fail-closed | Each of the four guards denies on miss/error/empty, and none returns a valid-looking zero value (`ai/rules/fail-closed-guards.md`) |
| Guard test altitude | Each guard test drives the entry point, not only the helper |
| Correctness | argon2id parameters are explicit constants, salt is `crypto/rand`, comparison is `subtle.ConstantTimeCompare` |
| Naming | `rescue-auth` kebab-case in YANG and cmdline; Go field `RescueAuth`; no `*Str`-style names |
| Data flow | The admin password never reaches the cmdline, in any form, at any layer |
| Rule: no-layering | `shell-auth-sha256`, `shellAuthHash`, `ShellAuth`, `validateShellAuth` fully deleted, no alias |
| Rule: exact-or-reject | Out-of-range operator input errors, never truncates or clamps silently |
| Rule: stale-comments | The "busybox has no bcrypt" rationale and the `InsecureIgnoreHostKey` comments are rewritten, not left describing the old behavior |
| YANG validation | `rescue-auth` has a `pattern`, not bare `type string` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Old rescue path deleted | `git grep -n 'shell-auth\|shellAuthHash\|ShellAuth' -- internal cmd test scripts docs` returns nothing |
| Rescue token verified end to end | QEMU scenario output showing `authenticated` after typing the token |
| Appliance push fails closed | test output for `TestSSHExecRefusesUnknownHost` |
| EAP-TLS code explainable | `bin/ze explain <code>` output pasted |
| Redirect sanitized | test output for the hostile-header table |
| 49 sites locally bounded | `git grep -c 'configUint' internal/plugins/ospf internal/plugins/isis` |
| Scanning page triaged | `gh api /repos/ze-software/ze/code-scanning/alerts --paginate` shows zero `open` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | cmdline `ze.rescue-auth` is attacker-supplied on a PXE network: shape-validate before use, never let a malformed value open an ungated shell |
| Fail-open on parse error | A malformed `rescue-auth` MUST select the reboot branch, not the ungated branch. VERIFIED and FIXED: `selectFatalBranch` keyed on `rescueAuth != ""`, so a malformed value selected `branchGated` and prompted for a token `rescueauth.Check` can never accept, hanging an unattended install at a console forever. It now returns `branchReboot` on either medium: never a shell on a bad credential, never a hang. `TestFatalPolicyBranchRejectsMalformedCredential` |
| Secret in logs | The rescue token must never reach `slog`; `initrd_linux.go:43` logs only a boolean today, keep it that way |
| Timing | Token comparison stays constant-time |
| Resource exhaustion | argon2 memory parameter is bounded and fixed, not derived from the cmdline |
| Error leakage | The known_hosts error names the host and file, never the key material |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| QEMU rescue scenario fails | R-1: harness and product must change together |
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

- CodeQL's signal-to-noise on this codebase is dominated by one pattern: a guard
  that exists but is not local to the flagged conversion. 49 of 86 alerts are the
  config parser guarding a plugin three layers down. Moving the bound next to the
  narrowing is both the fix and the suppression.
- The three genuinely valuable alerts all share a shape the scanner cannot see:
  a *documented* insecure choice whose stated justification has expired
  (`busybox has no bcrypt` when the initrd is now Go; `appliance uses self-signed
  host keys` when nothing pins them). A `//nolint:gosec` comment is a claim, and
  claims rot.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Independent rescue token over admin password | keep the admin password with a salted KDF; drop the cmdline digest entirely | A digest that commits to the admin password stays a standing offline-attack target even with a strong KDF. An independent token means a full break costs only rescue-shell access on a machine that is already failing to install. User decision, 2026-07-22 |
| argon2id with hex `salt:digest` on the cmdline | bcrypt `$2a$...`; scrypt | The `$` in a bcrypt/PHC string risks iPXE variable interpolation in the generated script; hex plus `:` is safe in both iPXE and `/proc/cmdline`. argon2 is already vendored |
| `~/.ssh/known_hosts` over TOFU-in-basedir | pin on first push under `getBaseDir()`; gate behind `ze.ssh.insecure` | Operators already have these hosts in known_hosts from ordinary SSH use, so it needs no new state and no unauthenticated first contact. User decision, 2026-07-22 |
| Reject EAP-TLS with no CA at config verify | verify the chain some other way; leave as documented | EAP carries no server hostname, so with no trust anchor there is nothing to check against; the config is the only place the problem can be reported (`exact-or-reject.md`) |
| Bound OSPF/IS-IS locally rather than suppress the query | add a CodeQL query filter; dismiss 49 alerts as-is | A query filter would hide future real cases. CORRECTED 2026-07-22: the local bound does NOT silence the alerts. `vrrp/groups.go` already bounds via `asUint(v, max)` (`groups.go:672-674`) and CodeQL still flags all four of its conversions (#163-#166), because the query cannot follow a `max` parameter. Phase 3 is therefore defense-in-depth only (`ai/rules/fail-closed-guards.md`: make the guard local to the narrowing); the 49 alerts need dismissing either way |
| Dismiss per alert number with individual comments | one query-level suppression | R-5: a query filter blinds the repo to the next real instance |

## Known Limitations
- The PXE install network stays the trust boundary: kernel, initrd and image are
  still fetched unauthenticated (`imageserver/handler.go:251-255`). This spec removes
  the operator *secret* from that network; it does not authenticate the network.
- MS-CHAPv2, CHAP-MD5 and RADIUS MD5 remain available. Their primitives are weak by
  modern standards but mandated by RFC 2759, RFC 1994 and RFC 2865. Whether to offer
  those methods at all is a product decision, out of scope here.
- The SSH client's loopback exemption (`core/ssh/client/client.go:492-494`) is not
  changed. A local user who wins the race to bind `127.0.0.1:2222` can still harvest
  the password sent after the no-op host key check. Recorded, not fixed.

## RFC Documentation

`// RFC 5216 Section 5.3` above the EAP-TLS trust-anchor rejection. The existing
RFC 2759 / RFC 1994 / RFC 2865 annotations on the flagged crypto sites stay as they
are; they already cite the mandating section.

## Implementation Summary

### What Was Implemented

| # | Fix | Where | Proof |
|---|-----|-------|-------|
| 1 | Rescue credential is a dedicated random token behind salted argon2id, replacing the unsalted sha256 of the admin password | new `internal/core/rescueauth`; `provision/main.go`, `imageserver/{config,handler}.go` + YANG, `install/disk/{cmdline,validate,run,rescue_linux,initrd_linux}.go` | `TestValuePinnedVector`, `TestCheckFailsClosedOnMalformedValue` (15 malformed forms), `TestPrintRescueToken`, `test/parse/image-server-invalid-rescue-auth.ci` PASS, `ze-test install 12` PASS |
| 2 | Appliance config push verifies the host key against `~/.ssh/known_hosts`, fails closed | `internal/appliance/cmd_config_push.go` | `TestSSHExecRefusesUnknownHost`, `TestSSHExecRejectsChangedHostKey`, `TestSSHExecAcceptsPinnedHostKey`, `TestSSHExecRefusesMissingKnownHosts` |
| 3 | EAP-TLS peer refuses to start without a resolvable trust anchor (producer side) | `internal/component/ike/engine/fsm.go` `buildPeerTLSConfig` | `sa.State = StateDead` path at `fsm.go:612-616`; ike suite green |
| 4 | `HX-Current-URL` / `Referer` open redirect closed at the source | `internal/component/web/{auth.go,handler_config_commit.go}` | `TestParentFromCurrentURLRejectsProtocolRelative` + 3 more, 9 hostile header forms each |
| 5 | `update bgp irr asn` rejects above 2^32-1 instead of truncating | `filter_irr/command.go` | `TestUpdateASNRejectsOutOfRange` |
| 6 | `announce` duration rejects values that overflow `time.Duration` | `cmd/announce/announce.go` | `TestParseDurationRejectsOverflow` |

### Bugs Found/Fixed
- **Open redirect CodeQL did not flag** (`parentFromCurrentURL`): the red run proved
  `HX-Current-URL: //evil.example/a/b` came back out as `//evil.example/a/`, and
  `https://evil.example` (no path) came out as the malformed `https://`. Both now
  fall back to `configEditPath`.
- **Silent ASN truncation** proved by the red run: `4294967296` became ASN 0 and
  `18446744073709551615` became ASN 4294967295, each producing an error naming an
  ASN the operator never typed.
- **EAP-TLS trust anchor lost silently**: `buildPeerTLSConfig` skipped `CACertPEM`
  not only when no CA was configured but also when a configured CA failed to
  resolve in the PKI store, downgrading to no verification with no log line.

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- **Phase order.** Phases were executed 2, 5, 4, 6 rather than 3-6, ordering by
  security value rather than by the spec's listing. No dependency between them.
- **Phase 3 (OSPF/IS-IS `configUint`) not started.** The 49 alerts it addresses are
  all guarded upstream (proven empirically), so this is hardening, not a fix.
- **`rescueauth` placed in `internal/core/`, not `internal/install/disk/`.** The
  image-server plugin needs the same encoding; a plugin importing the installer
  package would be a tier violation (`ai/rules/module-tiers.md`). It is a pure
  library with no config-driven lifecycle, so `internal/core/` is its tier.
- **AC-8 delivered, but at the reload transaction rather than `ze config validate`.**
  The three IPsecConfig validators had no non-test caller, so they were wired into
  a new ike `OnConfigVerify`. `ze config validate` does not invoke plugin verify
  callbacks (confirmed by the producer: `SendConfigVerify` is reached only from
  `internal/component/plugin/server/config_tx_bridge.go:206`, the reload transaction
  path), so the functional test is a `test/reload/` case, matching the existing
  `test-tx-iface-wireguard-invalid-no-private-key` precedent.
- **The `peer.go` guard WAS applied, with user approval** (d067ae22d). Thomas asked
  that the code and the tests do what the RFC requires, which authorised replacing
  `TestEAPTLSPeerWithoutCASkipsServerValidation` -- a test tagged
  `RFC5216-5.3-1 positive` while asserting a violation of that very MUST. Both test
  edits carry `rfc-test-change-approved:` markers.

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| No operator secret digest on the install network | grep + unit | `git grep 'shell-auth\|shellAuthHash\|ShellAuth'` returns only two `test-relax:` provenance comments and one assertion that the leaf is gone. The cmdline value now commits to a random token, not the admin password (`internal/core/rescueauth`), proven by `TestValueIsCmdlineSafe` (no plaintext leak) and `TestValueIsSalted` |
| Appliance push authenticates its peer | unit, from the command entry point | `TestSSHExecRefusesUnknownHost`, `TestSSHExecRejectsChangedHostKey`, `TestSSHExecRefusesMissingKnownHosts`, `TestSSHExecAcceptsPinnedHostKey` |
| EAP-TLS never accepts an unverified chain | unit + mutation | `TestEAPTLSPeerWithoutCARefusesToStart` (tagged `RFC5216-5.3-1 negative`). Mutation-verified: disabling the guard yields `startTLSClient() = eap-tls: failed to parse peer CA certificate, want errNoPeerTrustAnchor`. `make ze-rfc-check` green: 2716 gated requirements, 2544 tags resolved |
| Web redirect cannot leave the origin | unit over a hostile-header table | `TestParentFromCurrentURLRejectsProtocolRelative`, `TestParentFromCurrentURLSanitizesReferer`, `TestConfigDiscardRedirectIsSameOrigin`, `TestConfigDiscardHXRedirectIsSameOrigin`, 9 hostile forms each. The red run proved the hole: `//evil.example/a/b` returned `//evil.example/a/` |
| Operator input is rejected, not truncated | functional + boundary | `TestUpdateASNRejectsOutOfRange` (red run proved `4294967296` became ASN 0), `TestParseDurationRejectsOverflow`, `test/parse/image-server-invalid-rescue-auth.ci` PASS |
| Scanning page fully triaged | `gh api` listing | DONE 2026-07-22: 169 alerts, **0 open**. 39 fixed (four by commit 7457a0fcf, auto-closed by the next scan), 130 dismissed: 93 false positive, 26 won't fix, 11 used in tests. Every dismissal carries an individual comment naming the producing guard `file:line` or the RFC section; no query-level suppression was added (R-5) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). NOT YET RUN: the spec is still
     open (Phase 3 outstanding), so the independent review pass belongs with the
     closure commit, over the complete diff. -->

### Run 1 (2026-07-22, `/ze-review` single pass)

**Independence caveat, stated plainly:** this pass ran in the SAME context that
wrote the code. `ai/rules/critical-review.md` requires a DIFFERENT context
(independent reviewer subagents or a fresh session), so this does NOT satisfy the
Review Gate on its own. It is recorded because it found and fixed a real defect,
not as a substitute for the independent pass.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `selectFatalBranch` treated ANY non-empty `rescue-auth` as usable, so a malformed value selected `branchGated` and prompted for a token `rescueauth.Check` can never accept. An unattended network install would wait at a console forever instead of rebooting to retry. The spec's own Security Review row asserted the opposite ("MUST select the reboot branch"), making it a false safety claim in the diff | `internal/install/disk/rescue_linux.go:48-56` | FIXED: malformed now returns `branchReboot` on either medium. `TestFatalPolicyBranchRejectsMalformedCredential` added (red first, 14 cases) |
| 2 | ISSUE | The existing `TestFatalPolicyBranch` table used `"abcd1234"` as its "credential present" fixture, which is not a valid credential in the argon2id encoding. It was exercising the malformed path while claiming to cover the present path | `internal/install/disk/rescue_linux_test.go:59-60` | FIXED: fixture replaced with a valid pinned credential. No assertion removed; the rows still assert "usable credential -> gated" |
| 3 | NOTE | `rescueauth.Value` documents "Callers MUST pass a salt of SaltLen bytes" but does not enforce it. A wrong-length salt silently yields a credential that can never verify, because `split` rejects the salt hex length | `internal/core/rescueauth/rescueauth.go` | Accepted. Only `NewValue` (which always passes `SaltLen`) and the pinned-vector test call it; no production caller can get it wrong |
| 4 | NOTE | `docs/features.md` Installation row describes the installer initrd at length but does not mention the rescue token | `docs/features.md:20` | Accepted. It makes no stale claim, and the guide it links (`docs/guide/ze-install.md`) documents the token fully |

### Pre-checks
- `python3 scripts/dev/audit-test-relaxation.py 7457a0fcf~1`: 4 findings in this
  work, all verified legitimate. Three are documented relaxations for DELETED
  functions (`checkPassword`, `validateShellAuth`, `shellAuthHash`) whose coverage
  moved to `internal/core/rescueauth`; one `[DELETED]` is the
  `image-server-invalid-shell-auth.ci` -> `image-server-invalid-rescue-auth.ci`
  rename, confirmed by the commit stat (20 added / 20 removed) and the replacement
  passing as parse entry 150.
- `make ze-validate`: 8 unwired-export issues, none in this work (BGP reactor and
  test runner belong to a concurrent session; `host/inventory.go` is pre-existing).
  No symbol introduced here was flagged.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- Filled at closure. The two commits so far were made under an explicit Thomas
     owner override of the verify requirement (ai/rules/git-safety.md), recorded
     in each commit message. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/rescueauth/rescueauth.go` | yes | committed in 7457a0fcf (`create mode 100644`) |
| `internal/core/rescueauth/rescueauth_test.go` | yes | committed in 7457a0fcf |
| `internal/appliance/cmd_config_push_hostkey_test.go` | yes | committed in 7457a0fcf |
| `internal/component/web/handler_config_commit_test.go` | yes | committed in 7457a0fcf |
| `internal/component/bgp/plugins/filter_irr/command_test.go` | yes | committed in 7457a0fcf |
| `test/parse/image-server-invalid-rescue-auth.ci` | yes | committed in 7457a0fcf; `ze-test` parse suite entry 150 PASS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | rescue token minted, printed, verified; wrong token refused | `internal/core/rescueauth` suite green; `TestPrintRescueToken` |
| AC-4 | old sha256 path deleted, not aliased | full-tree grep: only `test-relax:` provenance comments remain |
| AC-5..AC-7 | appliance push fails closed / accepts pinned / rejects changed | four `TestSSHExec*` tests green |
| AC-8 | reject EAP-TLS with no CA at config verification | DELIVERED. `ValidateIPsecSections` (`internal/component/ike/engine/config.go`) is the ike plugin's `OnConfigVerify` body and runs all three previously-orphaned validators. `test/reload/test-tx-ipsec-eap-tls-requires-ca.ci` PASS; mutation-verified (no-op the callback and it FAILs). Note the enforcement point is the reload transaction, not offline `ze config validate`, which does not invoke plugin verify callbacks |
| AC-9 | EAP-TLS with a CA behaves as before | full `ike/...` suite green after the change |
| AC-10 | redirect stays same-origin | four web tests, 9 hostile header forms each |
| AC-11, AC-12 | irr ASN and announce duration rejected, not truncated | `TestUpdateASNRejectsOutOfRange`, `TestParseDurationRejectsOverflow` |
| AC-13, AC-14 | OSPF/IS-IS local bound; valid config unchanged | DELIVERED in 752051f3e. `TestConfigUintRejectsAboveMax` (mutation-verified) and the full ospf/... + isis/... suites, 28 packages, green before and after |
| AC-15 | scanning page fully triaged | 0 open alerts, verified by `gh api` listing |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `GOOS=linux go vet -tags 'linux ze_installer' ./internal/install/disk/` exit 0; suite green |
| A-2 | unvalidated -- BLOCKED | `make ze-install-scenarios-qemu-test` SKIPs without `ZE_INSTALL_KERNEL` (needs a vmlinuz with IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in). The installer QEMU harness runs the VM at 1024 MiB (`scripts/evidence/effective-install-qemu.py:352`), against which a 64 MiB argon2id arena is 6%, but that is the harness, not a real appliance floor, and no minimum-RAM figure for the installer is documented anywhere in `docs/`. Genuinely unproven |
| A-3 | confirmed | `go.mod` byte-identical after `go mod vendor`; one subpackage line added to `vendor/modules.txt` |
| A-4 | confirmed | full-tree grep after the rename |
| A-5 | partially validated -- and now moot for correctness | the config-file path is proven empirically; the hub-push and web-commit paths were NOT traced. Phase 3 (752051f3e) removed the dependence: the OSPF/IS-IS narrowing is now bounded locally, so an unvalidated entry point can no longer produce a truncated value |
| A-6 | confirmed | dismissed alert #168, PATCHed it back to `state=open`, re-read it as `open` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated (AC-8 and AC-13/14 outstanding, see above)
- [ ] End-to-End User Stories: story 3 (EAP-TLS config verify) has no passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (A-2 and A-5 outstanding)

### Quality Gates
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (justified in TDD Test Plan)
- [ ] Goal Validation table filled

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fixit-codeql-security-triage.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-codeql-security-triage.md` only
