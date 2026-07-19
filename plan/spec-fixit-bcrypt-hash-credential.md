# Spec: fixit-bcrypt-hash-credential

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/9 |
| Updated | 2026-07-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/fail-closed-guards.md` - credential handling
4. Source files in Current Behavior below (start with `internal/component/authz/auth.go` and `internal/component/aaa/types.go`)

## Task

**[HIGH]** The stored bcrypt password hash is itself accepted as a valid credential over
remote SSH/web AND is exported unmasked, so any config read becomes a remote
read-only-to-admin escalation. Verified by the 2026-07-16 audit (verifier V2);
all citations re-verified against the working tree on 2026-07-16 (design pass, see
Current Behavior).

**Chosen resolution (Thomas, 2026-07-16): restrict + mask.** Keep hash-as-token only for the
local CLI path (loopback / unix socket), reject the hash as a credential over remote SSH/web,
mask `ze:bcrypt` leaves on display/export, and gate `GET /config/download` behind edit-authz.
This direction is fixed; the design below implements it.

Producing code (all verified 2026-07-16):
- `CheckPassword` (`internal/component/authz/auth.go:81-93`) accepts the plaintext password
  (branch 2, bcrypt, `:92`) OR the stored bcrypt hash string itself (branch 1, constant-time
  compare of `sha256(hash)` vs `sha256(credential)`, `:85-89`). The hash-as-token branch is
  intentional (doc comment `:73-80`: the `ze` CLI sends the zefs-stored hash), but is reached
  over remote paths: SSH password auth (`internal/component/ssh/ssh.go:433-453`, `Authenticate`
  call `:437`) -> `LocalAuthenticator.Authenticate` (`auth.go:46-71`, `CheckPassword` call `:57`),
  web Basic auth (`internal/component/web/auth.go:218`) and web form login (`:263`), and the
  REST/gRPC API bearer path (`cmd/ze/hub/api.go:77-100`, `Authenticate` call `:91`). On success
  `Authenticate` returns the user's full profiles (`auth.go:60`), full impersonation.
- Export: the password leaf is `ze:bcrypt`, not `ze:sensitive`
  (`internal/component/ssh/yang/ze-ssh-conf.yang:29-35`, extension at `:31`); `ze:bcrypt` sets
  `LeafNode.Bcrypt` (`internal/component/config/yang_schema.go:572`) while `Sensitive` is set
  only by `ze:sensitive` (`:571`); `SensitiveKeys` collects only `Sensitive` leaves
  (`internal/component/config/schema.go:94-98`, check at `:107`). No serializer masks anything
  (zero `Sensitive`/`Bcrypt` references in `serialize.go`, `serialize_set.go`,
  `serialize_annotated.go`), so `show config` prints the hash raw. `GET /config/download`
  streams the raw committed config (`internal/component/web/handler_config_transfer.go:50-60`;
  `EditorManager.CommittedConfig`, `internal/component/web/editor.go:198-204`) behind `authWrap`,
  any authenticated session including read-only (`cmd/ze/hub/service_web.go:544`).

Also fold in the adjacent credential-in-logs leak:
- **[MEDIUM] SSH exec logs the full command at Info.** `execMiddleware` logs every exec command
  (`internal/component/ssh/ssh.go:682`, truncated at 256 bytes by `truncateForLog`, `:652-664`);
  a one-shot `ze config set ... password <hash>` is written verbatim. Redact credential-bearing
  tokens before logging.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/fail-closed-guards.md` - credential comparison and guard discipline
  → Constraint: a credential path must not accept a value that is only supposed to protect data at rest.
  → Constraint: the new transport flag's zero value MUST be the restrictive one (remote); a caller that forgets to set it gets hash-as-token rejected, never accepted.
- [ ] `ai/rules/config-surface.md` - how config display/export masks sensitive leaves
  → Constraint: masking must not break config round-trip (download -> edit -> upload) for real secrets.
- [ ] `docs/architecture/config/syntax.md` - password hashing on commit (Design anchor of `password_hash.go`)
  → Constraint: the `plaintext-<name>` sibling is the operator-facing write path; the canonical leaf holds only the hash.

**Key insights (verified during design):**
- `aaa.AuthRequest` already carries `RemoteAddr` and `Service` (`internal/component/aaa/types.go:95-100`); adding a transport-class field is a natural extension of the existing request shape, and the chain (`ChainAuthenticator`, `types.go:107-115`) passes the request through unmodified, so RADIUS/TACACS backends are unaffected.
- The `ze:sensitive` round-trip precedent is JunOS-style `$9$` reversible obfuscation: encoded on dump display (`internal/component/config/cli/cmd_dump.go:129-137`), decoded back on parse (`internal/component/config/parser.go:257-263`). Bcrypt leaves are explicitly excluded from `$9$` decode (`parser.go:254-257`). `$9$` is reversible by anyone, so it is NOT a suitable mask for the bcrypt hash; masking must use the strip placeholder plus a fail-closed guard (see Key Design Decisions).
- There is NO existing upload-side "unchanged" sentinel: `SecretDataPlaceholder` (`schema.go:91`) appears only in display paths (`cmd_dump.go:130,135`, `internal/component/cli/model_search.go:68`). The skeleton's R-1 mitigation ("existing sentinel") was wrong; the guard is new machinery.
- The local `ze` CLI connects over TCP SSH only (`internal/core/ssh/client/client.go:83,141,218`), default `127.0.0.1:2222` (`:40-41`); the hash-as-token is resolved from zefs for the super-admin (`resolvePassword`, `client.go:408-419`, store read `:413`). No unix-socket transport exists anywhere in the ssh component or hub today (grep: zero hits).

## Current Behavior (MANDATORY)

**Source files read:** (all re-verified against the working tree 2026-07-16; no citation drift
found except the `CheckPassword` doc comment, which spans `:73-80` not `:74-78`)
- [ ] `internal/component/authz/auth.go` - `CheckPassword` (`:81-93`; hash-as-token `:85-89`, bcrypt `:92`), `LocalAuthenticator.Authenticate` (`:46-71`; call `:57`, profiles returned `:58-62`). Sibling `AuthenticateUser` (`:100-118`, `CheckPassword` call `:108`) has NO non-test caller (grep over internal/, cmd/, pkg/); it must get the same restriction to stay consistent, and its unwired status is flagged for review.
  → Constraint: timing safety (dummy bcrypt for unknown users, `:66-69`; constant-time compare `:88`) must be preserved by the change.
- [ ] `internal/component/aaa/types.go` - `AuthRequest` struct (`:95-100`: Username, Password, RemoteAddr, Service); `ChainAuthenticator.Authenticate` (`:112+`) forwards the request unchanged.
- [ ] `internal/component/ssh/ssh.go` - password auth callback (`:433-453`) builds `AuthRequest` with `RemoteAddr: ctx.RemoteAddr().String()` and `Service: "ssh"` (`:437-442`); public-key auth (`:419-432`) is untouched by this spec. `execMiddleware` logs the command (`:682`) via `truncateForLog` (`:652-664`). Listeners are TCP only (`lc.Listen(ctx, "tcp", ...)`, `:463`, extra listeners `:485`).
  → Constraint: `ssh.Context.RemoteAddr()` is the real socket peer address, set from `conn.RemoteAddr()` (charmbracelet/ssh `context.go:137-138` at the pinned version); it is not client-controllable data.
- [ ] `internal/component/web/auth.go` - Basic auth `Authenticate` call (`:218-222`) and form login (`:263-267`); both set `RemoteAddr: r.RemoteAddr`, neither sets `Service`. Sessions cache profiles (`:125-152`).
- [ ] `cmd/ze/hub/api.go` - `buildUserAuthenticator` (`:77-100`): REST/gRPC bearer `user:password` -> `LocalAuthenticator.Authenticate` (`:91-94`) with no RemoteAddr at all. A remote surface; must never allow hash-as-token.
- [ ] `internal/component/config/schema.go` - `SensitiveKeys` (`:94-98`, `Sensitive`-only check `:107`); `DisplayMode` (`:78-88`), `SecretDataPlaceholder` (`:91`); `LeafNode.Sensitive`/`.Bcrypt` (`:135-136`).
- [ ] `internal/component/config/yang_schema.go` - `yangToLeaf` sets `Sensitive` (`:571`) and `Bcrypt` (`:572`).
- [ ] `internal/component/config/parser.go` - `parseLeaf` decodes `$9$` only for `Sensitive && !Bcrypt` (`:257-263`); a bcrypt leaf value is preserved verbatim.
- [ ] `internal/component/config/password_hash.go` - `IsBcryptHash` regex (`:17-24`), `CheckBcryptLeaves` warning walk (`:30-65`, the walk pattern the mask transform will mirror), `ApplyPasswordHashing` (`:88`) invoked from `internal/component/cli/editor_commit.go:152,312` and `internal/component/cli/editor_commands.go:985`.
- [ ] `internal/component/config/serialize.go` / `serialize_set.go` / `serialize_annotated.go` - shared by display AND persistence (`Serialize` `serialize.go:171`, `SerializeSubtree` `:185`, `SerializeSet` `serialize_set.go:89`, `SerializeSetWithMeta` `:492`, annotated `serialize_annotated.go:43,57,173,181`); none consults `Sensitive`/`Bcrypt`. Masking must therefore happen on a display copy of the tree, not inside the serializers (`Tree.Clone` exists, `tree.go:272`, already used by `internal/component/cli/editor.go:390,408`).
- [ ] `internal/component/cli/model_commands_show.go` - `renderTreeAtPath` (`:171-185`) is the CLI `show` choke point (config/set formats); annotated views go through `internal/component/cli/editor_annotated.go:42-66`.
- [ ] `internal/component/web/handler_config_transfer.go` - `HandleConfigDownload` (`:38-64`, raw stream `:50-60`, comment `:33-37` documents the current any-authenticated policy); `HandleConfigUpload` (`:73-115`) is already edit-gated (route `editMutationWrap`, `service_web.go:545`) and validates via `zeconfigcmd.ValidateContent` (`:96-103`) before `ApplyCommittedContent` (`internal/component/web/editor.go:212-233`).
- [ ] `cmd/ze/hub/service_web.go` - `authWrap` (`:467-474`), `editWrap` = authWrap + `RequireEditAuthz` (`:484-486`), `editMutationWrap` (`:487-489`); download route `:544` uses `authWrap`, upload route `:545` uses `editMutationWrap`. `RequireEditAuthz` is `internal/component/web/rbac.go:36-44`.
- [ ] `internal/core/ssh/client/client.go` - CLI credential resolution (`readCredentials` `:302-349`, `resolvePassword` `:408-419`): env `ze.ssh.password` wins, super-admin falls back to the zefs hash-as-token, others prompt. Dial is always TCP (`:83,141,218`).
- [ ] `pkg/zefs/keys.go` - `KeySSHPassword` (`:10`, "meta/ssh/{host}/{port}/password"). Writers: `internal/plugins/init/main.go:249` (host defaults to 127.0.0.1, `main.go:179,209-210`), `internal/appliance/cmd_assemble.go:113` (`cfg.SSH.Host`, image-local zefs), `internal/plugins/imageserver/handler.go:133` (hardcoded 127.0.0.1:2222, `:20-21`), `internal/plugins/connect/main.go:202,297` (`ze connect add` hashes the supplied password with a FRESH salt, `:187`, so its stored hash can never byte-match the remote daemon's hash; remote hash-as-token via `ze connect` credentials cannot work today).

**Behavior to preserve:**
- The local `ze` CLI still authenticates with the zefs-stored hash over its actual transport (TCP to 127.0.0.1:2222 today; unix socket if later added).
- Real (plaintext) password authentication over SSH/web/API continues to work from anywhere, including `ze.ssh.password` env used by test infra (many `test/plugin/*.ci` set it).
- SSH public-key auth (`ssh.go:419-432`) unchanged.
- Config round-trip (edit-gated download -> edit -> upload) preserves real passwords byte-exactly.
- `$9$` handling for `ze:sensitive` leaves unchanged (encode on dump, decode on parse).
- Timing safety: constant-time compare within `CheckPassword`, dummy bcrypt for unknown users.
- Persistence serialization unchanged: the on-disk config file keeps the real hash.

**Behavior to change:**
- Reject the bcrypt hash as a credential when the connection is not local (SSH from non-loopback, all web, all API).
- Mask `ze:bcrypt` leaf values with `SecretDataPlaceholder` on display paths (CLI `show` in all formats, web editor views, `ze config dump`, CLI search); reject the placeholder as a bcrypt leaf value on commit/upload (fail closed, clear error) so a masked artifact can never silently clobber a stored hash.
- Gate `GET /config/download` behind edit-authz (`editWrap`).
- Redact credential-bearing tokens from the SSH exec command log line.
- Known consequence (accepted by the fixed direction, flagged below as R-3): any flow that presents the stored hash to a daemon over a non-loopback connection stops authenticating and must supply the real password (env `ze.ssh.password` or prompt). Affected candidates: host-driven provisioned appliances (imageserver/assemble images driven over forwarded or LAN SSH, where the guest sees a non-loopback peer such as 10.0.2.2 under QEMU user networking) and QEMU install test tooling. This must be audited and migrated in the same change (Phase 2 exit criterion).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A login credential arrives via: SSH password callback (`ssh.go:433`), web Basic auth (`web/auth.go:218`), web form login (`web/auth.go:263`), API bearer header (`hub/api.go:91`). Separately: a config tree is rendered for display, the committed config is downloaded, a config is uploaded/committed, or an exec command line is logged.

### Transformation Path
1. The SSH entry point classifies the accepted socket peer (`ctx.RemoteAddr()`): unix-socket address or loopback TCP address => local; anything else => remote. It sets the new `AuthRequest` transport field accordingly. Web and API entry points never set the field (zero value = remote), because the local CLI never speaks to them with the hash.
2. `LocalAuthenticator.Authenticate` passes the transport class to `CheckPassword`; the hash-as-token branch (`auth.go:85-89`) runs only when the request is local. The plaintext bcrypt branch (`:92`) runs unconditionally. Unknown-user dummy bcrypt (`:66-69`) unchanged.
3. On display, the rendering path clones the tree (`Tree.Clone`, `tree.go:272`) and applies a new `config` transform that replaces every `Bcrypt` leaf value with `SecretDataPlaceholder` (walk mirrors `CheckBcryptLeaves`, `password_hash.go:39-65`); the masked clone feeds the existing serializers unchanged.
4. On commit/upload/validate, a new fail-closed guard rejects a `Bcrypt` leaf whose value is exactly `SecretDataPlaceholder` (error, not warning; message points at the raw download and `plaintext-<name>`). The raw committed file and the edit-gated download remain byte-exact, so the sanctioned round-trip never sees the placeholder.
5. `GET /config/download` route wrapping changes from `authWrap` to `editWrap` (`service_web.go:544`).
6. `execMiddleware` redacts credential-bearing tokens (bcrypt-shaped tokens anywhere; the value token following a `password`/`plaintext-password`-suffixed key) before `truncateForLog` (`ssh.go:678-682`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Transport ↔ auth | new transport-class field on `aaa.AuthRequest` (`types.go:95-100`), set only from the accepted socket peer at the SSH entry; zero value = remote (fail closed) | [ ] |
| Auth chain ↔ backends | `ChainAuthenticator` forwards `AuthRequest` unchanged (`types.go:112+`); only `LocalAuthenticator` consumes the flag | [ ] |
| Config store ↔ display | mask applied to a display clone of the tree before serialization; persistence serializers untouched | [ ] |
| Display ↔ re-import | placeholder rejected on commit/upload by a guard next to `CheckBcryptLeaves` | [ ] |
| Web route ↔ authz | `/config/download` moves to `editWrap` (`service_web.go:484-486`) | [ ] |
| Exec ↔ log | redaction helper applied at `ssh.go:682` call site | [ ] |

### Integration Points
- `aaa.AuthRequest` (new field), `LocalAuthenticator.Authenticate` + `CheckPassword` (+ sibling `AuthenticateUser`), SSH password callback (`ssh.go:433-453`).
- `config` package: new mask transform + placeholder guard, both walking like `CheckBcryptLeaves`; `SecretDataPlaceholder` reused.
- CLI show choke points: `model_commands_show.go:171-185`, `editor_annotated.go:42-66`, diff/compare (`internal/component/cli/diff_tree.go:266-267,440-451,529`), search (`model_search.go:35,68`).
- Web: `handler_config_walk.go` / `handler_config_leaf.go` / `handler_config_form.go` leaf-value rendering; `service_web.go:544` route wrap.
- `ze config dump`: `cmd_dump.go:85-140` extends its masking key set with bcrypt keys (placeholder always, never `$9$`, because `$9$` on a bcrypt leaf corrupts: parser preserves it verbatim and validation then fails).
- `execMiddleware` (`ssh.go:669+`) + a small credential-redaction helper in a core leaf package (see Key Design Decisions), which `config.IsBcryptHash` also uses so the bcrypt-shape regex has one home.

### Architectural Verification
- [ ] No bypassed layers (transport context reaches the auth decision explicitly as request data; no IP parsing inside authz)
- [ ] No duplicated functionality (one mask transform, one placeholder guard, one bcrypt-shape regex; serializers untouched)
- [ ] Registration over hardcoding — N/A (auth/config internals), stated for completeness (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The transport (loopback / unix socket vs remote) is knowable at the `CheckPassword` decision point | `ssh.Context.RemoteAddr()` is the socket peer set from `conn.RemoteAddr()` (charmbracelet/ssh `context.go:137-138`); `AuthRequest` already plumbs per-request data to the backend (`aaa/types.go:95-100`) | Cannot restrict by transport; fall back to "remove entirely" | design pass 2026-07-16: traced producer; re-confirm with `TestCheckPasswordRejectsHashOverRemote` | confirmed (design pass) |
| A-2 | Masking `ze:bcrypt` on display does not break upload round-trip | The sanctioned round-trip uses the edit-gated RAW download (`handler_config_transfer.go:50-60`), which stays unmasked; display-only masking plus a placeholder-reject guard at commit/upload prevents silent clobber. NOTE: the skeleton's basis ("existing sensitive-leaf sentinel") was wrong; no upload-side sentinel exists (`SecretDataPlaceholder` grep: display-only) and `$9$` cannot be used for bcrypt (`parser.go:254-257`, and it is reversible so it would not protect anyway) | Upload clobbers real passwords | round-trip functional test + `TestCommitRejectsMaskedBcryptValue` | confirmed (design pass, mechanism revised) |
| A-3 | The local `ze` CLI always connects over a transport classifiable as local | CLI defaults to TCP 127.0.0.1:2222 (`client.go:40-41`, `:272`); `ze init` defaults host to 127.0.0.1 (`init/main.go:179,209-210`); imageserver zefs is hardcoded 127.0.0.1:2222 (`imageserver/handler.go:20-21`). EDGE: `ze appliance assemble` keys zefs by `cfg.SSH.Host` (`cmd_assemble.go:113`), which an operator may set non-loopback; and host-driven access to provisioned appliances is genuinely remote | Local CLI login breaks on such appliances; those flows need `ze.ssh.password` (already the test-infra pattern) | audit every `KeySSHPassword` writer + QEMU install flow during Phase 2; `test/plugin/cli-credential-resolution.ci` guards the resolution order | partially confirmed (loopback default confirmed; provisioning flows to audit) |
| A-4 | A remote client cannot manufacture a loopback source through ze's own SSH server | charmbracelet/ssh denies port forwarding by default (`LocalPortForwardingCallback ... denies all if nil`, `server.go:60`; `DefaultChannelHandlers` registers only "session", `server.go:29-30`); ze sets neither `ChannelHandlers` nor a forwarding callback (grep `internal/component/ssh/`: zero hits), and wish v2.0.1 does not either (grep module: zero hits) | Loopback classification is forgeable via ze itself; would force unix-socket-only | pinned-version module source read 2026-07-16; add a regression test asserting direct-tcpip is rejected if feasible | confirmed (design pass) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A masked `show config` re-uploaded clobbers the password with the mask | round-trip test loses the password | Placeholder-reject guard at commit/upload (fail closed, clear error); raw edit-gated download stays byte-exact for the sanctioned round-trip |
| R-2 | Transport classification is spoofable (e.g. a local proxy forwards remote traffic to loopback) | security review | Web/API never accept hash-as-token regardless of source (flag never set there); SSH loopback cannot be reached via ze's own forwarding (A-4). An operator-installed local TCP proxy in front of port 2222 remains out of ze's control: documented residual risk; the dedicated unix-socket listener (open question Q-1) would close it |
| R-3 | Flows that present the stored hash over a non-loopback connection break: host-driven provisioned appliances, QEMU install tooling (guest sees 10.0.2.2), any `ze --remote`-style use against `ze init`-seeded targets | install/managed test suites fail after Phase 2 | Audit all `KeySSHPassword` writers and consumers in Phase 2; migrate affected flows to `ze.ssh.password` (plaintext) which already takes precedence (`client.go:409-411`) and is the established test-infra pattern; `ze connect add` remote hash-as-token is already non-functional (fresh salt, `connect/main.go:187`) so it cannot regress |
| R-4 | Web editor leaf form prefills the masked value and saves it back | form round-trip test | Mask the leaf VIEW; the form for a bcrypt leaf must not prefill the hash and should steer to `plaintext-<name>`; the commit-time placeholder guard is the backstop |
| R-5 | Sibling call sites drift: `AuthenticateUser` (`auth.go:100-118`) keeps the permissive behavior | grep in review | Same restriction applied in the same commit (sibling call-site audit row in Critical Review); its lack of non-test callers is flagged to Thomas |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| remote SSH login presenting a user's bcrypt hash as the password | -> | SSH callback classifies peer as remote; `CheckPassword` refuses hash-as-token | `TestSSHPasswordCallbackRejectsHashFromRemotePeer` (unit, drives the wish callback with a non-loopback peer) + `test/plugin/ssh-remote-hash-rejected.ci` |
| local CLI login presenting the zefs hash over loopback | -> | SSH callback classifies peer as local; hash-as-token accepted | `TestSSHPasswordCallbackAcceptsHashFromLoopback` + existing `test/plugin/cli-credential-resolution.ci` stays green |
| web Basic/form login presenting the hash | -> | web never sets the local flag; `CheckPassword` refuses hash-as-token | `TestWebBasicAuthRejectsHashAsToken` (`internal/component/web/auth_test.go`) |
| read-only user requests `GET /config/download` | -> | route now wrapped by `editWrap`; `RequireEditAuthz` returns 403 | `test/web/config-download-edit-gated.wb` |
| `show config` as any user | -> | display clone masked; serializer prints placeholder for the password leaf | `test/editor/commands/show-config-masks-bcrypt.et` |
| commit/upload of a config carrying the placeholder on a bcrypt leaf | -> | placeholder guard rejects with a clear error | `test/parse/bcrypt-placeholder-rejected.ci` |
| SSH exec `config set ... password <hash>` | -> | redaction helper scrubs the token before the Info log | `TestExecLogRedactsPasswordToken` (`internal/component/ssh/ssh_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Remote (non-loopback) SSH, or any web/API login, with a valid user's bcrypt hash as the password | Authentication fails |
| AC-2 | Local CLI login with the zefs-stored hash over loopback TCP (or unix socket if present) | Authentication succeeds (unchanged) |
| AC-3 | Remote/local login with the correct plaintext password (interactive or `ze.ssh.password`) | Succeeds (unchanged) |
| AC-4 | Read-only user requests `GET /config/download` | 403 via `RequireEditAuthz`; edit-authorized user still gets the raw file |
| AC-5 | `show config` (config/set/annotated formats), web editor views, `ze config dump`, CLI search results | `ze:bcrypt` leaf values shown as `SecretDataPlaceholder`, never the hash and never `$9$` |
| AC-6 | Download (edit-authz) -> edit -> upload | Round-trip preserves the real password byte-exactly (raw download is unmasked) |
| AC-7 | `ze config set ... password <bcrypt-hash>` over SSH exec | The credential token is redacted in the operational log; the command still executes with the full value |
| AC-8 | Commit or upload where a `ze:bcrypt` leaf value is exactly the placeholder | Rejected with an error naming the leaf and pointing at `plaintext-<name>` / the raw download; nothing applied |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | attacker who read a config backup tries the hash as an SSH/web password from another machine | transport classified remote -> hash-as-token branch skipped -> bcrypt compare fails -> reject + audit log | `test/plugin/ssh-remote-hash-rejected.ci`, `TestWebBasicAuthRejectsHashAsToken` |
| 2 | operator on the box runs `ze` commands; CLI sends the zefs hash to 127.0.0.1:2222 | loopback peer -> local class -> hash-as-token accepted -> profiles returned | `test/plugin/cli-credential-resolution.ci` (existing, stays green) |
| 3 | read-only web user tries to fetch the raw config | `editWrap` -> `RequireEditAuthz` 403 | `test/web/config-download-edit-gated.wb` |
| 4 | edit-authorized operator downloads, edits an unrelated leaf, uploads | raw file -> edit -> validate -> apply; password leaf untouched | round-trip assertion inside `test/web/config-download-edit-gated.wb` or a dedicated `.wb` step |
| 5 | operator pastes masked `show config` output into an upload/commit | placeholder guard rejects with actionable error | `test/parse/bcrypt-placeholder-rejected.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckPasswordRejectsHashOverRemote` | `internal/component/authz/auth_test.go` | AC-1 (flag false => hash-as-token off, plaintext still on) | |
| `TestCheckPasswordAcceptsHashOverLocal` | `internal/component/authz/auth_test.go` | AC-2 | |
| `TestAuthenticateDefaultsToRemote` | `internal/component/authz/auth_test.go` | fail-closed zero value (a caller that never sets the flag gets no hash-as-token) | |
| `TestSSHPasswordCallbackRejectsHashFromRemotePeer` | `internal/component/ssh/ssh_test.go` | transport classification from a non-loopback TCP peer | |
| `TestSSHPasswordCallbackAcceptsHashFromLoopback` | `internal/component/ssh/ssh_test.go` | loopback and unix peers classify local | |
| `TestWebBasicAuthRejectsHashAsToken` | `internal/component/web/auth_test.go` | web path never enables hash-as-token | |
| `TestMaskBcryptLeavesForDisplay` | `internal/component/config/schema_test.go` (or a new `mask_test.go`) | AC-5 transform: bcrypt leaves masked, sensitive/other leaves untouched, original tree unmodified | |
| `TestCommitRejectsMaskedBcryptValue` | `internal/component/config/password_hash_test.go` | AC-8 guard (driven from the commit/validate entry point, not the helper alone, per `ai/rules/fail-closed-guards.md`) | |
| `TestExecLogRedactsPasswordToken` | `internal/component/ssh/ssh_test.go` | AC-7 (bcrypt-shaped token and password-key value both scrubbed; non-credential commands logged unchanged) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| transport class | unix / loopback TCP / non-loopback TCP | unix and 127.0.0.1 / ::1 accept hash | N/A | first non-loopback address (e.g. 10.0.2.2) rejects hash |
| redaction truncation interplay | command length 0..>256 bytes | 256-byte redacted command intact | N/A | redaction applied BEFORE `truncateForLog` so a hash straddling the cut cannot half-leak |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ssh-remote-hash-rejected` | `test/plugin/ssh-remote-hash-rejected.ci` | presenting a stored hash over a non-loopback SSH connection fails to authenticate (needs a non-loopback source; if the runner cannot provide one, drive the daemon bound on a non-loopback local address and document the source-address evidence) | |
| `config-download-edit-gated` | `test/web/config-download-edit-gated.wb` | read-only web user gets 403 on `/config/download`; edit user gets the raw file containing the real hash (AC-4, AC-6) | |
| `show-config-masks-bcrypt` | `test/editor/commands/show-config-masks-bcrypt.et` | `show` hides the hash in config, set, and annotated formats (AC-5) | |
| `bcrypt-placeholder-rejected` | `test/parse/bcrypt-placeholder-rejected.ci` | validating/committing a config with the placeholder on the password leaf errors out (AC-8) | |
| `cli-credential-resolution` | `test/plugin/cli-credential-resolution.ci` (existing) | local CLI hash-as-token login still works (AC-2 regression guard) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local authentication and config display; no wire-protocol change | - | - | not a protocol feature | - |

## Files to Modify
- `internal/component/aaa/types.go` - add the transport-class field to `AuthRequest` (zero value = remote)
- `internal/component/authz/auth.go` - gate the hash-as-token branch on the request's transport class; same change in `AuthenticateUser`; preserve constant-time compare and dummy-bcrypt behavior
- `internal/component/ssh/ssh.go` - classify `ctx.RemoteAddr()` in the password callback and set the flag; redact credentials in `execMiddleware` before `truncateForLog`
- `internal/component/config/password_hash.go` (or sibling file) - placeholder-reject guard beside `CheckBcryptLeaves`; wire into commit (`editor_commit.go` call sites) and `ValidateContent` path used by web upload
- `internal/component/config/` (new small file, e.g. `mask.go`) - `MaskBcryptLeavesForDisplay`-style transform over a `Tree.Clone()`
- `internal/component/cli/model_commands_show.go`, `internal/component/cli/editor_annotated.go`, `internal/component/cli/diff_tree.go`, `internal/component/cli/model_search.go` - render masked display clone / extend masked key set
- `internal/component/config/cli/cmd_dump.go` - mask bcrypt keys with the placeholder in both text and JSON dumps (never `$9$` for bcrypt)
- `internal/component/web/handler_config_walk.go`, `handler_config_leaf.go`, `handler_config_form.go` - mask bcrypt leaf values in views; no hash prefill in forms
- `internal/component/web/handler_config_transfer.go` - update the download handler's policy comment (`:33-37`)
- `cmd/ze/hub/service_web.go` - `:544` `authWrap` -> `editWrap`
- `internal/core/` (new leaf helper, e.g. `internal/core/redact/`) - credential-token redaction + the bcrypt-shape regex; `config.IsBcryptHash` delegates to it (single source)
- docs: `docs/features/web-interface.md` / `docs/guide/config-editor.md` where download policy and masking are described (audit `source:` anchors for the changed files)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema change | no | masking keys off the existing `ze:bcrypt` extension; no new leaves |
| Env var | no | `ze.ssh.password` already exists and is the migration path for R-3 flows |
| CLI command change | no | display-only behavior change |
| Doctor check | no | no new runtime dependency |
| Audit log | yes (existing) | failed hash-as-token attempts flow through the existing SSH/web auth-failure recording (`ssh.go:450-451`, `web/auth.go:231-232`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features/web-interface.md` (download gate), config editor guide (masking) |
| 2 | Config syntax changed? | [ ] no | placeholder is display-only; file syntax unchanged |
| 3 | CLI command added/changed? | [ ] no | output masking only; note in `docs/guide/config-editor.md` |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/config-editor.md`; SSH/auth guide section on hash-as-token being local-only |
| 16 | Changed files referenced by doc source anchors? | [ ] check | grep `docs/` for anchors on `auth.go`, `ssh.go`, `handler_config_transfer.go`, `service_web.go` |

## Files to Create
- `internal/component/config/mask.go` (+ test) - display mask transform
- `internal/core/redact/` (+ test) - credential-token redaction helper
- `test/plugin/ssh-remote-hash-rejected.ci`, `test/web/config-download-edit-gated.wb`, `test/editor/commands/show-config-masks-bcrypt.et`, `test/parse/bcrypt-placeholder-rejected.ci` - functional tests

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add failing tests: remote hash-as-token rejected (unit + callback level); local accepted; web rejects hash; read-only download denied; bcrypt masked in show; placeholder rejected on commit; exec log redacted.
2. **Phase: transport context** — add the `AuthRequest` field (zero = remote); classify the socket peer in the SSH password callback; gate `CheckPassword` (and `AuthenticateUser`) on it. Sibling audit: every `Authenticate`/`CheckPassword` caller checked (ssh `:437`, web `:218,263`, api `:91`, authz register `:58`, hub api `:81`). Exit criterion: audit of every `KeySSHPassword` writer/consumer and the QEMU install flow for R-3; migrate any hash-over-remote flow to `ze.ssh.password` in the same phase.
3. **Phase: export masking** — mask transform on a display clone; wire CLI show (all formats), diff/compare, search, web views/forms, `ze config dump`. Verify the persistence path and raw download are byte-identical before/after.
4. **Phase: placeholder guard** — reject `SecretDataPlaceholder` on `Bcrypt` leaves at commit and upload-validate entry points (drive tests from those entry points, not the helper).
5. **Phase: download gate** — `service_web.go:544` to `editWrap`; update handler comment; `.wb` test.
6. **Phase: log redaction** — core redaction helper; apply in `execMiddleware` before truncation; `config.IsBcryptHash` delegates to the shared regex.
7. **Functional tests** — all `.ci`/`.wb`/`.et` rows above.
8. **Full verification** — `make ze-verify`.
9. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Transport spoofing | Local classification derives ONLY from the accepted socket peer address, never from headers or request data; web/API can never enable hash-as-token even from loopback; charmbracelet/ssh forwarding stays disabled (no `ChannelHandlers`/forwarding callback introduced) |
| Fail-closed default | The transport flag's zero value rejects hash-as-token; a new future caller that forgets to set it cannot fail open |
| Round-trip safety | Masking never clobbers a stored password: raw gated download byte-exact; placeholder rejected at every commit/upload entry point (editor commit `editor_commit.go:152,312`, `editor_commands.go:985`, web upload validate) |
| Residual disclosure | Audit remaining read surfaces for the raw hash: gNMI get, REST/gRPC API config reads, config-archive plugin, show-compare/diff output, crash/support bundles, web SSE fragments; each masked or justified in the audit table |
| Timing safety | Constant-time compare retained; dummy bcrypt for unknown users retained; rejection path does not create a user-enumeration oracle |
| Log redaction completeness | Redaction runs before truncation; covers bcrypt-shaped tokens anywhere in the command and values following password-like keys; web/API request logging checked for the same leak |
| Session invalidation | A hash-authenticated web session cannot exist after the change (web never accepts the hash); no stale sessions grandfathered |

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-8 each have code + test |
| Correctness | plaintext auth everywhere and local CLI hash login both still work; `$9$` sensitive handling untouched; persistence serialization byte-identical |
| Sibling call-site audit | every caller of `CheckPassword`/`Authenticate` passes or inherits correct transport context; `AuthenticateUser` updated in the same commit |
| R-3 migration | every flow that presented the hash over non-loopback is identified and migrated (install/managed/imageserver tooling), with test evidence |
| Registration over hardcoding | N/A — auth/config internals (`ai/rules/plugin-self-containment.md`) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Transport signal = explicit field on `aaa.AuthRequest`, set only by the SSH entry point from the accepted socket peer; zero value = remote | (a) parse `RemoteAddr` string inside `LocalAuthenticator`; (b) global mode flag; (c) separate local-only authenticator type | (a) makes authz guess at strings every surface formats differently and would accidentally enable hash-as-token for loopback WEB requests (reverse-proxy loopback is common; the CLI never uses web); (b) is not per-connection; (c) duplicates the user-list logic. The field keeps the decision where the socket is known and fails closed by construction |
| "Local" = unix-socket peer OR loopback TCP peer, decided at the SSH callback | unix-socket-only (add a dedicated listener now) | The CLI dials TCP 127.0.0.1:2222 today (`client.go:40-41,83`); no unix listener exists anywhere; loopback cannot be reached through ze's own SSH server (forwarding denied by library default, verified at the pinned version) and cannot be spoofed across the network (TCP handshake). A dedicated unix socket adds client+server+packaging surface and is raised as open question Q-1 rather than smuggled into this fix |
| Mask = `SecretDataPlaceholder` on a display CLONE of the tree; serializers and persistence untouched | (a) `$9$`-encode bcrypt like sensitive leaves; (b) mask inside `Serialize*`; (c) post-process serialized text | (a) `$9$` is reversible obfuscation, worthless against offline cracking of a value that remains a local credential, and the parser deliberately refuses `$9$` on bcrypt (`parser.go:254-257`); (b) the same serializers write the config file, masking there corrupts persistence; (c) text post-processing is fragile. `Tree.Clone` (`tree.go:272`) is cheap and already an editor pattern |
| Placeholder guard: commit/upload REJECTS a bcrypt leaf holding the placeholder (error, not warning) | silently resolve the placeholder to the committed value | Resolution needs the committed baseline at every entry point and silently "fixes" pasted configs (implicit behavior); rejection is fail-closed, one shared check next to `CheckBcryptLeaves`, and the sanctioned raw download makes the error recoverable. `ai/rules/fail-closed-guards.md` |
| Raw download stays unmasked but moves behind `editWrap` | mask the download too | A masked download can never round-trip (bcrypt is one-way); backup/restore is the download's purpose. Gating to edit-authz matches upload (`:545`) and closes the read-only escalation |
| Redaction helper lives in a core leaf package shared with `config.IsBcryptHash` | duplicate the regex in `ssh` | ssh must not import the config component for one regex; duplication violates single-source. A core leaf package is tier-correct (`ai/rules/module-tiers.md`) |

## Open Questions (for Thomas at approval)

1. **Q-1 unix-socket listener:** accept loopback TCP as the local signal (this design), or also add a dedicated unix-socket SSH listener (per-UID filesystem gating, closes the operator-installed-local-proxy residual R-2) in this spec? It touches client dial, server listen, paths, and packaging; my recommendation is a follow-up spec.
   → AUTONOMOUS DEFAULT (2026-07-17) [STAKES: security]: Keep loopback TCP as the local signal for THIS fix; do NOT add a dedicated unix-socket SSH listener here. Rationale: this is the design's own recommendation (Key Design Decisions "Local" row); a unix-socket listener touches client dial, server listen, filesystem paths, and packaging, so it is a separate follow-up spec, not smuggled into this fix. Fail-closed stays intact: the transport flag's zero value remains remote (reject hash-as-token), web/API never set it, and A-4 (verified at the pinned charmbracelet/ssh version, forwarding denied by default) shows loopback cannot be forged through ze's own SSH server. R-2's residual proxy risk (an operator-installed local TCP proxy in front of 127.0.0.1:2222, outside ze's control) is DOCUMENTED and ACCEPTED for this fix; the unix-socket-listener follow-up would close it. Thomas: override if wrong.
2. **Q-2 R-3 blast radius:** confirm that breaking hash-as-token for host-driven provisioned appliances and QEMU install tooling (migrating them to `ze.ssh.password`) is acceptable within this fix; the Phase 2 exit criterion treats the migration as in-scope.
   → AUTONOMOUS DEFAULT (2026-07-17) [STAKES: security]: Accept in scope. Breaking hash-as-token for host-driven provisioned appliances and QEMU install tooling, and migrating those flows to `ze.ssh.password` (plaintext, which already takes precedence at `internal/core/ssh/client/client.go:409-411`), is accepted within this fix. Rationale: R-3 and the Phase 2 exit criterion already treat the migration as in-scope; carving out an exception would preserve a permissive hash-as-token path over a non-loopback connection, which `ai/rules/fail-closed-guards.md` forbids. The Phase 2 audit of every `KeySSHPassword` writer/consumer plus the QEMU install flow stays a hard exit criterion. Thomas: override if wrong.
3. **Q-3 `AuthenticateUser`:** it has no non-test caller (grep 2026-07-16). It gets the same restriction here for consistency; flag whether it should instead be deleted (separate cleanup).
   → AUTONOMOUS DEFAULT (2026-07-17) [STAKES: security]: Apply the SAME transport restriction to `AuthenticateUser` (`internal/component/authz/auth.go:100-118`) for consistency; do NOT delete it in this fix. Rationale: although it has no non-test caller today (grep 2026-07-16), leaving a permissive sibling is a fail-open hazard the moment any future caller wires it, so it must inherit the restriction now; deletion is a separate cleanup and stays out of scope here. Its unwired status remains flagged (R-5 and the Critical Review sibling call-site audit row). Thomas: override if wrong (delete instead).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Round-trip (download -> edit -> upload) preserves real passwords
- [ ] R-3 audit complete: every `KeySSHPassword` consumer classified local/remote with migration where needed
- [ ] Registration over hardcoding respected (N/A stated)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Security review checklist complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for transport class and redaction/truncation interplay
- [ ] Functional `.ci`/`.wb`/`.et` tests for remote-hash-reject, download-gate, mask, placeholder-reject

## Implementation Result (2026-07-19)

All AC-1..AC-8 implemented with code + test:

| AC | Evidence |
|----|----------|
| AC-1 remote hash rejected | `authz.TestCheckPasswordRejectsHashOverRemote`, `ssh.TestSSHPasswordCallbackRejectsHashFromRemotePeer`, `web.TestWebBasicAuthRejectsHashAsToken`, `authz.TestAuthenticateDefaultsToRemote`; functional `test/plugin/ssh-remote-hash-rejected.ci` (written; daemon-based, not run under the no-server constraint) |
| AC-2 local hash accepted | `authz.TestCheckPasswordAcceptsHashOverLocal`, `ssh.TestSSHPasswordCallbackAcceptsHashFromLoopback` (loopback v4/v6 + unix) |
| AC-3 plaintext everywhere | `authz.TestCheckPassword` (local+remote), `web.TestWebBasicAuthRejectsHashAsToken` (plaintext branch) |
| AC-4 download edit-gated | `web.TestConfigDownloadRouteGatedByEditAuthz` (read-only 403, edit 200 raw); `service_web.go` `authWrap`->`editWrap` |
| AC-5 display masked | `config.TestMaskBcryptLeavesForDisplay`, `cli.TestDisplayContentAtPathMasksBcrypt`, functional `test/parse/config-dump-masks-bcrypt.ci` (placeholder, never hash, never $9$) |
| AC-6 round-trip byte-exact | `web.TestConfigDownloadRouteGatedByEditAuthz` (edit download == raw committed config); download handler streams unmasked |
| AC-7 exec log redacted | `redact.TestCommandRedacts*`, `ssh.TestExecLogRedactsPasswordToken`, `ssh.TestExecLogRedactsBeforeTruncation` |
| AC-8 placeholder rejected | `config.TestRejectMaskedBcryptLeaves`, functional `test/parse/bcrypt-placeholder-rejected.ci`; guard wired at all commit/validate entry points |

Assumptions A-1..A-4 all confirmed (transport knowable at decision point; masking preserves
round-trip via raw gated download + reject guard; local CLI is loopback; forwarding disabled).
R-3 audit: NO existing flow breaks (see learned 1181 / DECISION.md).

### Review Gate

Independent reviewer (subagent, read-only diff review) run 2026-07-19:
- **HIGH**: web `/cli` show verb (`handleCLIShow` -> `EditorManager.ContentAtPath`) rendered
  config UNMASKED and was reachable by any authenticated (incl. read-only) session, leaking the
  raw hash. **FIXED**: `EditorManager.ContentAtPath` now routes through the masked
  `DisplayContentAtPath` (added to `contract.Editor` + all implementers); regression
  `cli.TestDisplayContentAtPathMasksBcrypt`. Web serialize sweep confirms no other unmasked
  display path.
- NOTE (documented residual, not fixed here): gNMI `Get` returns raw leaf values (separate
  component, own auth); redaction scoped to password-family+bcrypt to avoid false-positives on
  BGP community / host-key path tokens; `cmd_dump` resolves BGP before masking (safe: no bcrypt
  leaf under bgp). Recorded in DECISION.md / learned 1181.

Verification: scoped unit tests green for `authz`, `ssh`, `web`, `config` (core), `config/cli`
(dump), `aaa`, `core/redact`. Three pre-existing `config/cli` failures (listener-conflict,
fix-plan repair IDs) are in the shared BGP/web validation pipeline, caused by concurrent agents'
uncommitted rpki/vrrp schema changes (my `cmd_validate.go` change is additive and inert for
those non-bcrypt configs). Full `make ze-verify` intentionally NOT run (would kill live servers
per operator instruction).

## Notes
- Skeleton captured from the 2026-07-16 repository audit; hash-as-token remote reachability and the unmasked export chain were verified by verifier V2. Resolution direction (restrict + mask) chosen by Thomas.
- Design pass 2026-07-16: every skeleton citation re-verified against the working tree; only drift is the `CheckPassword` doc comment span (`:73-80`, skeleton said `:74-78`). New producer citations added throughout (aaa request shape, CLI dial path, zefs writers, library forwarding defaults, serializer sharing, `$9$` machinery, `Tree.Clone`).
- Sibling spec: `plan/spec-password-weakness-warning.md` (status ready) also touches password-set paths in `internal/component/config/password_hash.go` and expects `auth.go` area stability; coordinate merges. A concurrent mgmt-listener spec touches `cmd/ze/hub/service_web.go`; the `:544` one-line wrap change here should rebase trivially but must be re-verified after either lands.
- The web CLI terminal (`internal/component/web/cli_terminal.go`) drives the same CLI model, so masking at the show/render choke points covers it; confirm during Phase 3.
