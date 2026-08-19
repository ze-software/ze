# Spec: fixit-appliance-cert-replace-unvalidated

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze appliance replace-cert` destroys the working TLS material before it knows the
replacement is usable.

`runReplaceCert` reads the operator's certificate and key, writes `cert.pem` with
a plain truncating write, then writes `key.pem` through the atomic secret writer.
Nothing between the read and the writes parses the PEM, checks that the
certificate and the key are a pair, or looks at the validity dates. Nothing takes
a backup, so there is no rollback: a failure after the certificate write leaves a
mismatched pair with the previous material gone, and a crash during the
certificate write leaves a truncated file.

The self-signed regeneration branch has the same two-write shape, and so does
`writeTLSSecrets` in the appliance initialisation path. The problem is the shape,
not one function.

The material lives on the operator's machine and is copied into the image at
assemble time, so the damage surfaces later and in three different places: the
next OTA push refuses because the certificate will not parse, the assemble step
copies the broken pair into the image without checking it, and the installed
appliance boots, loads the stored material without validating it, fails to build
its TLS configuration, prints one warning line, and runs with the web listener
silently disabled.

The repository states the correct rule elsewhere: material that does not parse is
refused, and the previously installed certificate keeps serving. This command
does the opposite.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/builder.md` - the day-2 command set and where certificate replacement sits
  → Constraint: the page anchors the certificate replacement source, so a behavior change here must update it
- [ ] `docs/guide/appliance.md` - the operator-facing replace-cert usage
  → Decision: the command is documented as a routine day-2 operation, so its failure mode must be recoverable without rebuilding the appliance
- [ ] `ai/rules/never-destroy-work.md` - overwriting material the operator supplied
  → Constraint: the previous certificate is user-visible work; overwriting it before the replacement is proven good is the failure this rule names
- [ ] `ai/patterns/cli-command.md` - the command surface conventions this must keep
  → Constraint: exit codes and error text are part of the contract; a refusal must be a clean non-zero exit with a message naming the reason

**Key insights:**
- The secret writer is already atomic, temp file plus rename. The certificate write is not, which is why the pair can diverge inside one command.
- The configuration storage layer has the better template: create a temp file, set the mode, sync, then rename.
- The standard library's key-pair loader is the validation this needs, and it is already used at two other places in the tree.
- The existing test writes a fake certificate body and asserts the command succeeds, so it pins the absence of validation and must be corrected with the fix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/cmd_cert.go` - `runReplaceCert` parses the flags, loads the appliance config, resolves the passphrase when the store is encrypted, then writes the certificate with a truncating write and the key with the atomic secret writer; the self-signed branch has the same shape; the certificate and key bytes are never zeroed
- [ ] `internal/appliance/cmd_init.go` - `writeTLSSecrets` repeats the same two-write shape at initialisation
- [ ] `internal/appliance/crypto.go` - `WriteSecret` encrypts when a passphrase is set, writes a temp file, renames, and removes the temp file on a rename failure
- [ ] `internal/appliance/resolve.go` - `tLSDir` places the material under the appliance's secrets directory on the operator's machine
- [ ] `internal/core/cliio/cliio.go` - the file reader, including the single-use standard input path; it has no PEM awareness
- [ ] `internal/core/selfcert/selfcert.go` - `GenerateWebCertWithNames` returns a PEM pair in memory; `LoadOrGenerateCert` returns stored material without validating it; `NewTLSConfig` is where the pair is finally parsed
- [ ] `internal/appliance/cmd_push.go` - `loadDeviceTLS` parses the certificate file and refuses the push when it holds none
- [ ] `internal/appliance/cmd_assemble.go` - the assemble step copies both files into the image database with no validation
- [ ] `internal/appliance/cmd_show.go` - `certExpiry` decodes the first PEM block and prints nothing when it fails
- [ ] `internal/component/doctor/checks_tls.go` - the TLS check notices a certificate with no key, and never checks that the two match
- [ ] `internal/component/config/storage/storage.go` - the atomic write helper this should follow: temp file, mode, sync, rename
- [ ] `internal/appliance/cmd_day2_test.go` - the existing replace-cert tests write a fake body and assert success, pinning the missing validation

**Behavior to preserve:**
- The passphrase handling and the encrypted-secret path stay exactly as they are, including zeroing the passphrase.
- The single-use standard input path keeps failing closed when it is claimed twice.
- The self-signed regeneration keeps producing the same certificate content and SANs.
- The file mode stays owner-read-write only.
- A successful replacement keeps printing the same confirmation line.

**Behavior to change:**
- Operator-supplied material is validated before anything is written.
- Neither file is replaced unless both can be replaced.
- A failure leaves the previous material in place, and says so.
- The initialisation path gets the same treatment.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator runs the appliance certificate replacement command with a certificate and key path, or with no paths for self-signed regeneration.
- Format at entry: PEM files on the operator's filesystem, or standard input.

### Transformation Path
1. `runReplaceCert` in `internal/appliance/cmd_cert.go` parses the flags and loads the appliance configuration
2. The passphrase is resolved when the secret store is encrypted
3. `cliio.ReadFile` reads the certificate and the key, or generates the pair through `selfcert.GenerateWebCertWithNames`
4. The certificate is written with a truncating write, then the key through `WriteSecret`
5. Later, `cmd_assemble.go` copies both into the image database, and the installed appliance loads them at boot through `selfcert.LoadOrGenerateCert`
6. `NewTLSConfig` parses the pair; on failure the web listener is disabled with a warning

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator ↔ command | file paths or standard input | No |
| Command ↔ secret store | plain write for the certificate, encrypted atomic write for the key | No |
| Builder ↔ image | the assemble step copies both files into the image database | No |
| Image ↔ running daemon | the web server loads the stored pair at boot | No |

### Integration Points
- `WriteSecret` - the existing atomic writer for the key half
- the configuration storage atomic write helper - the template for the certificate half
- the standard library key-pair loader - the validation to reuse rather than reinvent
- `loadDeviceTLS` - an existing PEM parser in the same package that already refuses an empty certificate file

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `runReplaceCert` no longer writes files itself; both entry points go through `writeTLSSecrets` (`internal/appliance/cmd_init.go`) |
| No unintended coupling (components stay isolated) | Yes | the validation uses `crypto/tls` and `crypto/x509` directly; no new dependency on `internal/core/selfcert` or on the doctor component |
| No duplicated functionality (extends existing, does not recreate) | Yes | the certificate half reuses `WriteSecret` (`internal/appliance/crypto.go`) with no passphrase; no second atomic writer exists |
| Zero-copy preserved where applicable (refs, not copies) | Yes | one backup copy of the previous certificate, held only until the key write returns; not a hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | no command added; `cmdReplaceCert` keeps its existing `init()` registration (`internal/appliance/cmd_cert.go`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The secrets directory is an ordinary writable directory on the operator's machine, so a same-directory temp file and rename works | the directory resolver places it under the operator's configuration directory | A staging directory or a different swap mechanism is needed | A test that performs the replacement in a temporary directory and asserts the temp files are cleaned up | confirmed -- `TestReplaceCertLeavesNoTempFile` |
| A-2 | The standard library key-pair loader is sufficient validation for what the appliance will later serve | the same loader is what the web server uses at boot, so agreeing with it is exactly the property wanted | A certificate that loads here still fails at boot, and the validation gives false confidence | A test that feeds a mismatched pair and asserts the refusal, plus one that feeds the self-signed output and asserts acceptance | confirmed -- `TestReplaceCertRefusesMismatchedPair` refuses, `TestReplaceCertRegenerates` accepts the generated pair; `validateTLSPair` calls `tls.X509KeyPair`, the same call `selfcert.NewTLSConfig` makes at boot |
| A-3 | An expired certificate should be refused rather than warned about | the operator is replacing material precisely to keep the listener working | A legitimate workflow that stages a not-yet-valid certificate is blocked | Owner decision recorded here; the chosen answer is pinned by a boundary test | ANSWERED, owner confirmation owed -- a certificate past its not-after date is REFUSED, and the message names both dates. A certificate whose not-before is in the future is ACCEPTED: R-3 names blocking a staged renewal as the risk, and the material is copied into an image that boots later, so the not-before check belongs at boot (`checkCertExpiry`, `internal/component/doctor/checks_tls.go`) rather than at replacement. Pinned by `TestReplaceCertExpiredCertificate`, which also asserts the last second before not-after is accepted |
| A-4 | The initialisation path can take the same helper with no ordering change | it writes the same two files in the same directory | Initialisation has a constraint the replacement path does not, and the helper needs a variant | Read the initialisation ordering in Phase 2 and record the answer | confirmed -- `runInit` creates the TLS directory and writes `password.hash` before it calls `writeTLSSecrets` (`internal/appliance/cmd_init.go`), and the directory is empty at that point. `readForRestore` treats a missing file as absent, so the restore for the initialisation case is a delete. No variant is needed; the direction inverted instead, and `runReplaceCert` now calls `writeTLSSecrets` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two renames are not one atomic commit, so a crash between them still leaves a mismatched pair | A crash test leaves divergent files | Validate first, which removes the realistic cause; then keep a backup of both files and restore on any error, reporting a failed restore rather than swallowing it |
| R-2 | Validation rejects material an operator previously managed to install, making an upgrade look like a regression | An operator reports that a command that used to work now refuses | The refusal names the exact reason. Material that fails this check was never going to serve, so the refusal is the earlier report of an existing failure |
| R-3 | The expiry decision blocks a staged renewal workflow | An operator with a future-dated certificate is refused | Settle A-3 before implementing; a warning plus an explicit override flag is the fallback |
| R-4 | Zeroing the certificate and key buffers changes behavior if any caller retains them | A test asserting buffer contents after the call fails | Only the private key material needs zeroing, and only after the write completes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The operator's ability to replace an appliance certificate, and in the worst case the material for an appliance they are about to build. A refusal is recoverable; a wrong write is what this spec exists to stop |
| How is it reverted? | Single commit revert. No format change on disk, so a revert leaves whatever material is present working |
| Who else touches this path? | The initialisation path writes the same files; the assemble and push paths read them; `plan/journal/rollback-forgets-partial-apply.md` records the same class from another surface |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The operator supplies a mismatched certificate and key | → | the validation step in `internal/appliance/cmd_cert.go` | `TestReplaceCertRefusesMismatchedPair` |
| The operator supplies unparseable material | → | the same validation step | `TestReplaceCertRefusesUnparseablePEM` |
| The key write fails after the certificate write | → | the backup and restore path | `TestReplaceCertRestoresOnKeyWriteFailure` |
| The operator regenerates a self-signed certificate | → | the same write path | `TestReplaceCertRegenerates` |
| An operator replaces a certificate on a built appliance | → | the whole command | `test/appliance/appliance-replace-cert.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A certificate and key that are not a pair | The command refuses with a message naming the mismatch, and both existing files are unchanged |
| AC-2 | Material that does not parse as PEM | The command refuses with a message naming the file, and both existing files are unchanged |
| AC-3 | Valid material, but the key write fails | Both files are restored to their previous content, and the command reports both the original failure and the restore outcome |
| AC-4 | Valid material and a successful run | Both files are replaced, with the same modes as today, and no temporary file is left behind |
| AC-5 | An expired certificate | The behavior recorded in A-3 is applied, and the message names the validity dates |
| AC-6 | The appliance initialisation path | It uses the same validated, restorable write |
| AC-7 | A partially written certificate from an interrupted run | No such state is reachable: the certificate is written through a temp file and a rename |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Replaces an appliance certificate with a CA-signed pair | command → validation → atomic writes → secrets directory | `test/appliance/appliance-replace-cert.ci` |
| 2 | Mistypes a path and supplies a key that belongs to another certificate | command → validation → refusal, nothing written | `TestReplaceCertRefusesMismatchedPair` |
| 3 | Hits a disk error partway through | command → write failure → restore → reported | `TestReplaceCertRestoresOnKeyWriteFailure` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReplaceCertRefusesMismatchedPair` | `internal/appliance/cmd_day2_test.go` | AC-1 | RED at HEAD, GREEN with the fix |
| `TestReplaceCertRefusesUnparseablePEM` | `internal/appliance/cmd_day2_test.go` | AC-2; replaces the current test's fake body assertion | RED at HEAD, GREEN with the fix |
| `TestReplaceCertRestoresOnKeyWriteFailure` | `internal/appliance/cmd_day2_test.go` | AC-3, with an injected write failure | RED at HEAD, GREEN with the fix |
| `TestReplaceCertUpdatesSecrets` | `internal/appliance/cmd_day2_test.go` | existing test, corrected to supply a real pair | GREEN; corrected, and no longer discriminating on its own |
| `TestReplaceCertRegenerates` | `internal/appliance/cmd_day2_test.go` | existing test stays green through the new write path | GREEN, unchanged |
| `TestReplaceCertLeavesNoTempFile` | `internal/appliance/cmd_day2_test.go` | AC-4 and A-1 | GREEN; `TestReplaceCertWritesCertificateThroughTempFile` is the discriminating half (RED at HEAD) |
| `TestReplaceCertExpiredCertificate` | `internal/appliance/cmd_day2_test.go` | AC-5, pinning the A-3 answer | RED at HEAD, GREEN with the fix |
| `TestInitWritesValidatedTLSSecrets` | `internal/appliance/cmd_init_test.go` | AC-6 | RED at HEAD, GREEN with the fix |
| `TestReplaceCertWritesCertificateThroughTempFile` | `internal/appliance/cmd_day2_test.go` | AC-7 -- a directory at `cert.pem.tmp` proves the write goes through a temp file and a rename | RED at HEAD, GREEN with the fix |
| `TestReplaceCertRefusesEmptyFile` | `internal/appliance/cmd_day2_test.go` | the zero-PEM-blocks boundary | RED at HEAD, GREEN with the fix |
| `TestReplaceCertRequiresBothFlags` | `internal/appliance/cmd_day2_test.go` | `--cert` alone used to fall through to self-signed regeneration and destroy the material the operator meant to keep | RED at HEAD, GREEN with the fix |
| `TestCheckWebTLS_MismatchedPair` | `internal/component/doctor/doctor_test.go` | the doctor half: a stored pair that does not load is reported, and the message carries no key material | RED without the pair check, GREEN with it |
| `TestCheckWebTLS_UnreadableKey` | `internal/component/doctor/doctor_test.go` | a key that exists and fails to read is not reported as missing | RED without the pair check, and RED again when the read failure is reported as `doctor-tls-missing` |
| `TestCheckWebTLS_MatchingPair` | `internal/component/doctor/doctor_test.go` | a matching pair is not a finding | GREEN; the companion that catches a check reporting every pair |
| `TestRunChecksReportsUnusableWebTLSPair` | `internal/component/doctor/doctor_test.go` | `runChecks` reaches the pair check, so the finding is one an operator sees | RED without the pair check, GREEN with it |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| certificate validity window | up to not-after | the last second before not-after (`TestReplaceCertExpiredCertificate`) | N/A -- a not-before in the future is ACCEPTED per the A-3 answer, and the test asserts it | a certificate already past not-after (`TestReplaceCertExpiredCertificate`) |
| PEM blocks in the certificate file | 1 or more | a leaf plus its chain | zero blocks (`TestReplaceCertRefusesEmptyFile`) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `appliance-replace-cert` | `test/appliance/appliance-replace-cert.ci` | replacing a certificate with a valid pair succeeds, and replacing it with a mismatched pair refuses and leaves the appliance's material intact | GREEN (`make ze-functional-appliance-test`, 9/12); RED at HEAD with "replace-cert accepted a mismatched certificate and key" |
| `doctor-web-tls-pair` | `test/ui/doctor-web-tls-pair.ci` | `ze doctor --json` reports a stored web certificate and key from two different pairs as `doctor-tls-invalid` | GREEN (`make ze-functional-ui-test`, 144/182); RED with the pair check removed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No protocol behavior changes; this is a builder-side command | |

## Files to Modify
- `internal/appliance/cmd_cert.go` - validate before writing; write the certificate atomically; back up both files and restore on any failure; report a failed restore
- `internal/appliance/cmd_init.go` - use the same helper for the initialisation write
- `internal/appliance/crypto.go` - if the certificate half needs a sibling of the secret writer, it belongs here beside it
- `internal/appliance/cmd_day2_test.go` - the existing tests assert success for a fake certificate body and must be corrected
- `docs/architecture/appliance/builder.md` - state the validation and rollback guarantee
- `docs/guide/appliance.md` - document what the command refuses and what it leaves behind on failure
- `internal/component/doctor/checks_tls.go` - report a stored web certificate and key that do not load as a pair
- `internal/component/doctor/doctor_test.go` - unit and entry-point tests for the pair check
- `internal/core/diagnostic/codes.go` - `doctor-tls-invalid` now also covers a pair that does not load

## Files to Create
- `test/appliance/appliance-replace-cert.ci` - functional proof for AC-1 and AC-4
- `test/ui/doctor-web-tls-pair.ci` - functional proof that `ze doctor` reports a stored pair that does not load

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | a builder-side command, no daemon config |
| YANG validation constraints | N-A | no YANG leaf |
| YANG custom validators | N-A | no YANG leaf |
| CLI commands/flags | Yes | `internal/appliance/cmd_cert.go`; a possible override flag if A-3 lands on warn-plus-override |
| CLI grammar (keyword before value) | Yes | any new flag follows the existing appliance flag style |
| Editor autocomplete | N-A | not a config editor surface |
| Functional test for new RPC/API | Yes | `test/appliance/appliance-replace-cert.ci`, and `test/ui/doctor-web-tls-pair.ci` for the doctor half |
| Pipe completeness | N-A | the command prints one confirmation line, not a pipeable table |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | Yes -- done | The owner authorised this item on 2026-08-19, after the rest of the spec was implemented. `checkWebTLSPair` (`internal/component/doctor/checks_tls.go`) reads the key and calls `tls.X509KeyPair`, under the existing `doctor-tls-invalid` code. `checkWebTLS` calls it when the certificate reads and the key exists, so a pair an older `ze` stored without validation is now reported. A key that exists and fails to read gets its own message and is never reported as missing |
| Prometheus counters/metrics | N-A | a builder-side command emits no daemon metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the command exists; its guarantees change |
| 2 | Config syntax changed? | No | no config syntax |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md` carries the refusal cases and the `replace-cert <name>` row. `docs/guide/command-reference.md` is NOT updated: it documents no `ze appliance` subcommand at all, and it states that the per-command list lives in `ze help command`. One appliance command added there would document the family in two places |
| 4 | API/RPC added/changed? | No | no API surface |
| 5 | Plugin added/changed? | No | no plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | no wire format |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC requirement involved |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` carries one row for `appliance-replace-cert.ci`. The new `test/ui/doctor-web-tls-pair.ci` gets no row: that page describes the ui suite as a whole and lists no individual `test/ui/*.ci` |
| 11 | Affects daemon comparison? | No | no comparison claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/builder.md` |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | No | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/appliance/builder.md` anchors the certificate replacement source; `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` carry the mapping rows |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the appliance guide's replace-cert example must show what a refusal looks like |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the missing validation visible
   - Tests: `TestReplaceCertRefusesMismatchedPair`, `TestReplaceCertRefusesUnparseablePEM`
   - Files: `internal/appliance/cmd_day2_test.go`
   - Verify: both fail today, and the existing fake-body test is shown to pin the defect
2. **Phase: validate first** -- parse and pair-check before any write
   - Tests: the two above, plus `TestReplaceCertUpdatesSecrets` corrected to a real pair
   - Files: `internal/appliance/cmd_cert.go`
   - Verify: nothing is written when validation fails; A-4 is answered by reading the initialisation ordering
3. **Phase: write atomically** -- temp file and rename for the certificate half
   - Tests: `TestReplaceCertLeavesNoTempFile`
   - Files: `internal/appliance/cmd_cert.go`, `internal/appliance/crypto.go`
   - Verify: no truncating write remains on either path, validating A-1
4. **Phase: restore on failure** -- back up both files, restore on any error, report the restore
   - Tests: `TestReplaceCertRestoresOnKeyWriteFailure`
   - Files: `internal/appliance/cmd_cert.go`
   - Verify: an injected key-write failure leaves the previous material in place and the failure is reported, not swallowed
5. **Phase: same treatment at initialisation, and the expiry decision**
   - Tests: `TestInitWritesValidatedTLSSecrets`, `TestReplaceCertExpiredCertificate`
   - Files: `internal/appliance/cmd_init.go`
   - Verify: the A-3 answer is recorded in this spec before the code lands
6. **Phase: close the boot-side half and prove it functionally**
   - Tests: `test/appliance/appliance-replace-cert.ci`
   - Files: the doctor TLS check, `test/appliance/`
   - Verify: a pair that fails to load is reported by the doctor check rather than only by a warning line at boot

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Both write sites are fixed, not only the replacement command |
| Correctness | A refusal leaves both files byte-identical to their previous content |
| Naming | The error messages name the file and the reason, in the project's US English style |
| Data flow | Validation happens before the first write, not between the two |
| Rule: `ai/rules/never-destroy-work.md` | No path overwrites the previous material before the replacement is proven good |
| Rule: `ai/rules/evidence.md` | The restore failure is reported rather than discarded |
| Registration over hardcoding | The doctor check extension registers through the existing diagnostic registry |

### Deliverables Checklist
| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| No truncating write of certificate material | grep both files for a direct write of the certificate path | met -- `grep -n os.WriteFile internal/appliance/cmd_cert.go internal/appliance/cmd_init.go` returns only the two `authorized_keys` writes |
| Validation before write | `TestReplaceCertRefusesMismatchedPair` passes | met |
| Restore on failure | `TestReplaceCertRestoresOnKeyWriteFailure` passes | met |
| Functional proof | `make ze-functional-appliance-test` runs the new `.ci` green, or the suite target that owns `test/appliance` | met -- `appliance-replace-cert` PASS (9/12). `vpp-hugepages-qemu` fails in the same suite at HEAD too, on an unpopulated gokrazy modcache; recorded in `plan/journal/gate-verdict-depends-on-the-machine.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Certificate and key bytes are operator input read from a file or standard input; both must parse and match before anything is written |
| Secret handling | The private key buffer is zeroed after use, as the passphrase already is; error messages never echo key material |
| File permissions | Temp files are created with the owner-only mode before any content is written, never widened and narrowed afterwards |
| Fail closed | A failure leaves the previous, working material in place; an appliance must never be left with material that cannot serve |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-3 unanswered when Phase 5 starts | STOP and put the expiry question to the owner; implementing either answer silently is the failure this row prevents |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Validate, then write both through temp files and renames | Back up and restore only; stage in a separate directory and swap | Validation removes the realistic cause of a mismatched pair, and same-directory renames need no new layout. A staging directory adds machinery for a two-file write. The backup remains as the residual guard for a write that fails for reasons validation cannot predict |
| Reuse the standard library key-pair loader | Write a bespoke certificate and key comparison | Agreeing with the loader the web server actually uses at boot is the property wanted; a bespoke check could pass here and fail there |
| Fix the initialisation path in the same spec | Fix only the replacement command | It is the same defect at a second entry point, and leaving it means the very first write an appliance gets is still unvalidated |

## Known Limitations
- Two renames are not one atomic commit. After validation the residual window is two adjacent renames, and the backup covers it; a genuinely atomic two-file swap would need a directory swap, which the layout does not currently support.
- This spec does not change what the appliance does at boot with material that fails to load beyond making it reportable; whether a boot should regenerate rather than disable the listener is a separate question and is named in Known Limitations rather than assumed.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
