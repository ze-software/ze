# Bootstrap key diagnostics source handoff

Bounded source-only continuation by bootstrap_key_diagnostics. ROOT owns every
build, test, formatter, review and commit. No execution evidence was produced
by this child and no formal CLEAN is claimed. All live Go edits are frozen.

## ROOT evidence already received

- Doctor RED 6398878a: missing pre-config registration, uncoded warning without
  fix, no report-bus issue. This was a semantic RED, not a compiler failure.
- Certificate RED 7e0c86f7: actual SSH authenticated expired, force-command,
  source-address and otherwise-valid certificates as unrestricted recovery
  admin. Canonical reader and actual assembly accepted them. Ordinary keys
  passed as controls.
- Certificate GREEN 03acd7d2: both actual entrypoint parents passed all four
  negatives plus ordinary-key controls after the canonical parser refusal.
  The same run still gave all three doctor parents semantic RED, including
  the newly added unavailable-store probe. ROOT then released doctor edits.
- Doctor production source is now applied; this child has not yet received
  ROOT's subsequent three-parent GREEN. Keep that verification outstanding.

## Applied production and tests

- internal/core/authorizedkeys/authorizedkeys.go: refuse *ssh.Certificate,
  keeping the existing plain-public-key contract. No CA or certificate feature.
- internal/core/diagnostic/codes.go: central code and metadata for
  doctor-local-admin-credentials, explicit offline repair command.
- cmd/ze/hub/bootstrap_credentials.go: canonical usersFromStore check;
  missing store reports unchecked; disabled/healthy is clear; coded boot log
  and one deduplicated report warning with fix, independently of logger.
- cmd/ze/hub/register_bootstrap_credentials.go: registered pre-config check,
  storage dependency, any platform, owned event code.
- cmd/ze/hub/main.go: ONLY bootPowerUsers hunk belongs to this phase; preserve
  ROOT's separate configuration-owner/window edits elsewhere in the file.
- cmd/ze/hub/bootstrap_credential_diagnostic_test.go: original candidate
  294a76c3 applied by ROOT; helper now calls RegisterBuiltinCodes, matching the
  shipped entrypoint's post-init setup. All original assertions retained.
- cmd/ze/hub/bootstrap_credential_probe_test.go: new nil-store refusal parent.
- cmd/ze/hub/bootstrap_certificate_login_test.go and
  internal/appliance/bootstrap_certificates_test.go: ROOT-applied/formatted
  candidate 2796c311, unchanged assertions; actual signed certificate cases.

Doctor production candidate SHA fd70c210514c9fff9ca8e0cd88351b5714f33cc06ec1ec99bad8c2660efbb2c2
was applied exactly after RED. Parent formatting may change live file hashes.
The seven newly edited Go paths requiring ROOT formatter were already sent;
the two certificate test files were formatted separately by ROOT ecf108c0.

Docs updated in this phase: docs/guide/authentication.md (plain-key contract,
canonical doctor, console/working-login stop/restore/restart instructions),
docs/guide/appliance.md and docs/architecture/appliance/builder.md (certificate
refusal in existing bootstrap-key paragraphs). Shared pages contain other
owners' edits. No runtime-success claims were added.

## Functional proof prepared, deliberately UNAPPLIED

scratch/deployment-bootstrap-credential-doctor-functional.candidate.patch
SHA 51728083b9d36a541bb19c9f818c11b0e368c38c03eaed93a365db951ed8cb39
supersedes prior dd916e12 candidate. It adds exactly:

- internal/test/fixture/register_bootstrap_credential_doctor.go
- test/draft/plugin/bootstrap-credential-doctor.ci

The fixture uses existing createDaemonStorePopulated, storageCommand,
storageRequireCommand and openDaemonStoreReadOnly helpers. It seeds an owned
store with a proper bcrypt hash and a marked malformed key, invokes the
runner-selected shipped Ze CLI, checks strict current-schema JSON and exact
one Warning/code/explicit-path write fix/private-input absence, proves the
read-only check preserved the key, repairs through actual ze data write,
checks the owned row clears, then restores malformed material and explicitly
disables admin through actual ze data write to prove the disabled control.
It tolerates other host findings, but only normal doctor exit 0 or 1 and exact
owned findings; process/setup failures and malformed/trailing JSON fail.

Crucial isolation: doctor uses `--json config -` with configuration on stdin.
doctor/local_route.go::askDaemonDoctor refuses to forward stdin before any
SSH address probe. Thus the result cannot come from another session's daemon.
doctor.go::loadDoctorConfig reads stdin; core/resolve.StoreDir("-") uses the
test's ze.config.dir=. so it still checks precisely the seeded owned store.
No daemon, listener, port, VM, sleep or new runner is added.

ROOT next: inspect/apply candidate, format fixture, build freshly named le and
Ze so registration is actually compiled, baseline draft, inspect cost row,
stress80 any-failure, promote the exact tested .ci bytes and run required
promoted scope per ze-test. No functional run has been attempted by this child.

## ROOT remaining verification/review

1. Three doctor parents GREEN (^TestBootstrapCredential), then scoped affected
   hub/appliance/authorizedkeys tests and already planned frame/fuzz coverage.
2. Functional candidate baseline/load/promotion as above.
3. Retained independent reviewer review of exact certificate/doctor deltas and
   immutable final census; review has not yet declared CLEAN.
4. ROOT-controlled signed commits/closure, alongside the other deployment work.

This child did not edit metadata, specs, signing state, CI live tests, or any
configuration-owner, QEMU, IKE or website files. No children were spawned.
