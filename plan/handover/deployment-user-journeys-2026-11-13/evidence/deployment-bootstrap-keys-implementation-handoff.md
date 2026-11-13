# Bootstrap keys: implementation and diagnostic continuation

Parent session c43b4620-2510-44dd-8936-84100b444bb5, root namespace79a29bf3.
Agent appliance_seed_keys. Parent owns deployment spec. All execution belongs
to ROOT. No tests, builds, formatters, commits, children or approvals by me.

## Current implementation, frozen

ROOT's actual e7e47c81 run first established semantic RED through the real
standalone SSH listener: the supplied valid bootstrap key failed admin auth.
Malformed/restricted local-key files were accepted by the canonical reader.
Explicit disable, configured same-name replacement plus its own key success,
outbound-namespace refusal plus password success, and absent/comment-only key
password controls passed. Initial f1e44291 real-SSH failures were only fixture
ownership failures, not authentication evidence; the new fixture now uses a
store-relative host-key name and keeps strict host pinning.

Eight Go files are frozen for ROOT formatting and focused GREEN:

- cmd/ze/hub/main_servers.go: reads the new local key frame after existing
  disabled/username/password checks. Missing key means password-only; other
  read errors are actionable opererr. One shared parser returns validated keys,
  attached to the same recovery UserCredential before existing accepted identity.
- cmd/ze/hub/bootstrap_key_login_test.go: six real-entry parents, corrected
  store-relative host pin; missing/comments cases now named subtests whose
  owned SSH and identity cleanup finishes before the next case. Added invalid
  line before a valid key to prevent upstream skip behavior hiding it.
- internal/appliance/cmd_assemble.go: validates optional named public-key file
  before staging/publishing. Only fs.ErrNotExist means absent; unreadable,
  malformed or restricted material refuses. Stores the exact original public
  bytes under new local credential frame, no outbound compatibility writer.
  New read-error repair uses the actual ./ze-host appliance assemble command.
- internal/appliance/cmd_assemble_test.go: exact independently APPROVED(b)
  historical candidate fd110ae674673fdcdfeb60ece11add697a8217ab9683ea664065db420fd3a861
  applied only after retained reviewer recorded native root79a approval.
- internal/appliance/bootstrap_keys_test.go: actual assembly/blob namespace and
  complete payload proof, malformed/restricted refusal before publication.
- pkg/zefs/keys.go: KeyLocalAdminAuthorizedKeys at
  meta/auth/local/authorized-keys replaces unused KeySSHAuthorizedKeys. No
  Go reference to the old symbol remained in cmd/internal/pkg after replacement.
- internal/core/authorizedkeys/authorizedkeys.go: shared per-complete-line
  x/crypto ParseAuthorizedKey use. A whole-file upstream parse can skip bad
  earlier lines, so each noncomment line is parsed alone. Any options are
  refused rather than silently stripped. Error text carries no submitted line.
  Empty/comment-only input means no keys; any error returns no partial keys.
- internal/core/authorizedkeys/authorizedkeys_test.go: FuzzParse seeds valid,
  comments, multiple keys, invalid-before/after-valid and restricted inputs;
  checks atomic refusal, input ownership and returned encoded-key validity.

Formatting/testing has not been done by this agent. ROOT may already have
formatted or observed results after this handoff; use its directly read log.

Five documentation paths updated:
docs/guide/authentication.md; docs/architecture/appliance/builder.md;
docs/architecture/zefs-format.md; ai/digests/aaa-auth.md; ONLY the nine-line
public-key usage paragraph after named-config JSON in shared docs/guide/appliance.md.
The digest also corrects stale usersFromZefsDB names and distinguishes absent
username keys from present empty/broken credentials. No native VM success is
claimed in these edits. Parent owns wiki and public site evidence.

## Diagnostic gap remains, do not claim covered

ROOT explicitly authorized an exact bootPowerUsers log/report hunk in main.go,
plus an owning registered doctor check and code. That function currently emits
an uncoded Warn for every genuine usersFromStore failure, with no report bus
issue. doctor-store-integrity checks frame integrity; a well-framed malformed
key is a semantic credential refusal it cannot discover. No fitting registered
local credential problem code was found. No main.go diagnostic edit was made.

Private tests-first candidate (NOT APPLIED):
deployment-bootstrap-credential-doctor-tests.candidate.patch
SHA256294a76c39343abc5befb7a3681c55c1d5494f45ee94a2ebd2a167901af852d44
Adds cmd/ze/hub/bootstrap_credential_diagnostic_test.go, build tag ze_ssh.
Selector ^TestBootstrapCredential, two parents:

1. TestBootstrapCredentialDoctorUsesCanonicalReader requires a registered
   pre-config hub-local-admin-credentials check, code
   doctor-local-admin-credentials with explanation metadata, and actual
   canonical reader refusal for malformed keys/missing password. Check must
   return exactly one Warning with repair command and sanitized message.
   Password-only/comments and explicitly disabled malformed keys yield none.
2. TestBootstrapCredentialFailureRaisesCodedWarningAndClears drives existing
   bootPowerUsers twice over a malformed file. It requires coded/fix-bearing
   sanitized WARN plus exactly one deduplicated report issue under source hub,
   subject local-admin. Removing optional malformed key restores password
   recovery and clears that owned issue. Test clears only that exact warning
   on entry/cleanup, not unrelated report state.

ROOT first applies/formats/runs this candidate after the current key GREEN job,
observes actual registry/code/report RED, then releases diagnostic implementation.
The new test names only existing APIs/literal planned code and registry name;
it should compile without new production symbols.

## Concrete diagnostic implementation route after RED

- Register one pre-config check from a new hub register_*.go, with storage
  dependency and explanation metadata. It must call usersFromStore; no second
  key parser, no separate SSH-only metadata reader. Runtime report code also
  registers through diagnostic.RegisterEventCode for recent-events coverage.
- Fix the existing bootPowerUsers warning at its producer to carry the code
  and actual repair command; raise one report.RaiseWarningFix with stable
  source/code/subject independently of whether a logger was supplied. Clear
  that exact issue on successful canonical read or explicit admin-disabled.
  Preserve existing silent disable semantics and absence-is-uninitialized.
- Existing core/report API supports RaiseWarningFix, ClearWarning, Warnings
  and SourceWarnings. Diagnostic.DoctorCheck uses PhasePreConfig, dependencies,
  Codes and Check. DoctorCheckContext.Store is the supplied owned handle.
  Runtime-phase contexts intentionally have no Store, so this storage check
  belongs pre-config; do not mark it runtime then read a nil borrowed handle.
- Reuse or register code metadata at its proper owner. Existing metadata is
  centralized through diagnostic builtin tables; new files must join that
  registry without hand-replicating a list. Inspect its actual registration.
- ze data check is verified actual offline grammar: config/storage/cli/register.go
  registers data root and check, main.go handlers maps check to cmdCheck. It
  diagnoses unreadable/corrupt frames, not semantic key text. ze data write
  <key> <file> also exists and requires exclusive offline ownership; determine
  correct remediation/lifecycle before using it as Fix. Malformed public-key
  material already has safe ssh-keygen -y -f <recovery-private-key> advice.
- Add a repair/disable clear control if needed, keep payload/option text out
  of logs/diagnostics, and run targeted RED→GREEN, then affected package/report
  tests and coverage from ROOT. No whole-tree checks or generic AAA rewrite.

ROOT owns native appliance config-push proof, final source review, signed
landing and all-flavor compile checks. Do not close the deployment spec here.
