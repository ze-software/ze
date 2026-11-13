# Bounded source review: window repair and bootstrap credentials

Independent reviewer deployment_complete_source_review, parent SID
c43b4620-2510-44dd-8936-84100b444bb5. Source preparation only, not CLEAN.

## Window repair: source-clear

Read the incremental repair and reached Window constructor/do/Stop/open
producers, all three new window-test parents and their fixture, main startup
and shutdown, all explicit bind sites, the exact applied DNS fixture changes,
all five new host-driver repair strings, and the two architecture paragraphs.

configMutationRuntime now stores its explicit window and commit passes r.window,
so public withdrawal cannot turn confirmed-window exclusion into WriteOutside(nil).
Main constructs the empty window before binding runtime. NewWindow does not load
pending records; boot recovery is a separate earlier synchronous operation. The
worker remains available for revert reloads until Stop completes; only then does
main close the owner's shared reload admission. A stopped retained window refuses
new writes. Existing admission still joins before store release.

The new stopped-window regression uses Stop's actual worker join, then exercises
both Upload and source Commit; accepted/active bytes, candidate absence, reload
count and the original durable rollback are checked. Separate no-pending useful
upload and actual Abort/reload/promotion controls prevent blanket refusal from
satisfying it. The real SSH fixture still covers actual daemon application.
Both configs explicitly disable resolver output; exact four-MiB calculations,
all rejection assertions and the fixed 96-byte incomplete declaration remain.
All five new remediation strings now name ./ze-host appliance. Source anchors
in remote-operations.md and hub-architecture.md match these producers.

ROOT reports semantic RED f1e44291; failing-selector GREEN90612 was active at
brief time. This reviewer has not executed or credited an unreturned GREEN.

## Bootstrap keys: eight Go paths reviewed through three lenses

Reviewed all changed hunks of main_servers.go, cmd_assemble.go,
cmd_assemble_test.go and pkg/zefs/keys.go; full new core authorizedkeys parser
and fuzz file, bootstrap_key_login_test.go, and appliance/bootstrap_keys_test.go.
Read own documentation hunks in authentication.md, builder.md, zefs-format.md,
aaa-auth.md and the nine-line appliance-guide addition. Applied production and
security, tests/wiring/edge, and style/documentation lenses.

Useful wiring is actual initialization key file -> assembler validation before
artifact staging -> registered meta/auth/local/authorized-keys -> usersFromStore
-> existing boot/merge -> accepted generation -> existing real SSH key matcher.
No SSH-only store fallback or outbound namespace grant is introduced. Admin
disable precedes key parsing. Same-name configured users replace the complete
bootstrap user and recovery profile. Key Marshal bytes become the normal typed
public-key fields, preserving the existing accepted-generation/RBAC mechanism.
Real SSH tests exercise two keys, wrong username/key, password recovery,
disable/config override, and outbound-only refusal. Parsing each complete line
prevents upstream whole-file bad-line skipping; malformed or textual option-
bearing entries return no partial identity. Assembly validates before artifact
publication. Missing/comment-only files preserve password recovery. Fuzzing
checks input ownership, atomic refusal and returned-key blob validity.

### One new source-bounded PLAUSIBLE security issue

authorizedkeys.Parse accepts certificate-valued ssh.PublicKey results with no
textual authorized_keys options. usersFromStore stores their complete blob, and
the existing parseConfiguredKey/matchPublicKey authentication path merely compares
Marshal bytes. Actual charm.land/ssh callback forwards that bool without a
CertChecker. x/crypto server.go delegates authorization to PublicKeyCallback;
its signature verification reaches Certificate.Verify, which calls only the
embedded Key.Verify. Thus a pinned expired, wrong-principal, or critical-option-
restricted SSH certificate can receive unrestricted bootstrap recovery authority
without the certificate constraints being checked.

Sent to ROOT before fixes. ROOT must reproduce with actual signed certificate
input and a useful ordinary-key positive. Minimal bounded repair is refusal of
certificate-valued bootstrap entries, maintaining a plain-key-only recovery
contract; no CA/certificate feature is requested. Existing textual from=/command=
options are correctly rejected and are not claimed broken. The known doctor
visibility gap remains separately author-owned and unresolved at this review.

Current read-source hashes (before any certificate/doctor repair):

| Path | SHA256 |
|---|---|
| internal/core/authorizedkeys/authorizedkeys.go | 10d34018bb1a9ae7354e1132f13578d410fbe6f7ea16ae4d407bcd41e6b03050 |
| internal/core/authorizedkeys/authorizedkeys_test.go | 299007536abd8606933980b6e35229c8ee5c5d5799217c240aae8ef718c180a3 |
| cmd/ze/hub/main_servers.go | 9fd3333adbda0269fcd1c071d54890a3ae02941e847f10b9e0cbcbd030508f87 |
| internal/appliance/cmd_assemble.go | 60613853c907044cdfa907b576463d462d2061a0f304d941106a9887b17915a6 |
| internal/appliance/cmd_assemble_test.go | 9bfbf01fbdbb80959b8f2632feaff401667395886c3bd40c9f8de71383940014 |
| pkg/zefs/keys.go | 35d0b26be9bb36057fcbd5f36c25788cf299428db86000e74022c4ee076b96b8 |
| cmd/ze/hub/bootstrap_key_login_test.go | 1ea3592039a8b11a327de007069e37294d629184ecd465dec7bdeb5d254aef1a |
| internal/appliance/bootstrap_keys_test.go | de3b739a53288e2e4294f370e70ba29b109b556e1d9246916f1b9378bf2ef504 |
| cmd/ze/hub/config_upload.go | 8b0ae750730127ed0e82be799574a6cd1c2566e8f7775653fc6c161c839e5263 |
| cmd/ze/hub/config_upload_window_test.go | 899b46944b1b3a1095d4a9751826247870307a18e69e70a259ed2d664030d5e7 |

## Bounded LDP routing for ROOT to record

Owner is existing in-progress
plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md; its owned
table at line537 has no formatter row. No matching ldp-backoff-clear-frr row was
found in the two specifically considered journal classes. Suggested single row
under parser-refuses-a-form-its-producer-emits:

| 2026-11-13 | spec-single-owner-lifecycle-5-session-end-locals | TestFormatCorpusRoundTrip / ldp-backoff-clear-frr/ze.conf -> Serialize | Actual541c2f18: fixture environment/log { ldp info } formats as log ldp info and hierarchical reparsing refuses line17; unrelated to SetParser | open: owning LDP fixture must use the current logging schema; preserve corpus round-trip assertions |

This is a fixture correction/disposition, not a generic formatter expansion.
No tracked journal or owning-spec row was authored by this independent phase.

## Limits and continuation

No tests, builds, formatters, product execution, commits, tracked edits or children
by reviewer. Historical431 plus foreign IKE scope retained; final immutable
committed-delta census is still owed. Resume on exact certificate/doctor repairs
and returned ROOT proofs. Parser9b and CR750 all-five passes are ROOT-reported;
native installed-appliance and final delivery proofs remain ROOT-owned.
