# Independent agent-fixture judgment and credential-repair source review

Parent SID c43b4620-2510-44dd-8936-84100b444bb5; namespace79a29bf3.
Retained reviewer deployment_complete_source_review. No formal CLEAN.

## Agent client fixture — APPROVE(a), with sequencing disclosure

Exact candidate SHA25664c4a55724b74abbbd3cf968db2d73ea25a6066b59f853f15e5cc0f0e1a3d801:
native-config-push-agent-client.candidate.patch. Read complete current test and
configPushIdentity/agent owner, exact patch and diagnosis, and actual vendored
NewClient/newPipeline/readLoop. Each NewClient on an io.Closer owns a background
pipeline reader. Multiple clients on one connection compete for response bytes.
Reusing one client preserves every real Sign/Verify/exact-key/mode/idle-stop/
worker-join/socket-removal/environment assertion and all request operations.

Important: the first source read already showed the one-client executable hunk
present, although the request described a before-edit judgment. Only the new
Upstream comment remained absent. ROOT was told immediately; this report does
not fabricate pre-edit timing. The carrier is untracked and absent from HEAD,
so no historical native body row was invented. ROOT-reported old failure was
actual response-too-large at Sign, with the other three helper parents passing.

Test-judge: internal/le/test/qemu/install_config_push_agent_test.go: APPROVE(a): exact64c4a557 candidate gives the owned connection one agent pipeline reader while retaining List, Signers, actual signing/verification and every identity/lifetime/environment assertion.

Read test hash before the Upstream comment:
7a1dce9feb0619d0ea39eaf6a2be4a231610599323ac90ddb0a131d3fc0d4da1.
No test execution or source edit by reviewer.

## Certificate refusal — source-resolved

Read exact new Parse certificate guard, both complete new certificate test
files, and matching authentication/builder prose. *ssh.Certificate is refused
before append; failure returns nil rather than keys already read. Assembly
continues validating before publication. Real SSH tests cover expired, critical
force-command/source-address and otherwise-valid signed certificates, followed
by useful plain-key authentication. Assembly also prefixes a valid plain key to
certificate input and requires no database publication, with its own useful
plain-key assembly positive. Thus rejection is atomic and does not create a CA
implementation or change configured-user key handling. ROOT reports both
certificate parents PASS03acd7d2; this reviewer did not execute them.

## Bounded doctor delta — source-clear

Reviewed new bootstrap_credentials.go, register_bootstrap_credentials.go,
bootstrap_credential_diagnostic_test.go and bootstrap_credential_probe_test.go,
exact main.go bootPowerUsers log/report replacement, and code/metadata hunks
in diagnostic/codes.go. Reached canonical credential reader,
actual doctor runner, report RaiseWarningFix/ClearWarning, opererr LogAttrs and
offline cmdWrite were read. This report names actual read paths; final immutable
freeze population reconciliation remains required before the formal gate.

The registered pre-config check receives the owned Store and uses usersFromStore,
never a second parser or a runtime borrowed handle. Actual runRegisteredChecks
filters only platform, so Dependencies metadata does not suppress the nil-Store
probe: that path explicitly warns unknown. Errors are sanitized by the canonical
key parser; no submitted key line appears in new diagnostic/log text. The coded
Warn carries actionable fix, and report-bus issue has stable hub/code/local-admin
identity independent of logger presence. Successful read and explicit disable
clear only that owned issue. Metadata and event registration make the code
explainable/discoverable through existing doctor surfaces. Offline repair syntax
matches cmdWrite's key/file contract; docs correctly require stopping the daemon,
selecting the actual database, restoring the affected frame and restarting.

New tests drive registered canonical-reader cases, missing password, malformed
and disabled/optional controls, coded sanitized Warn, active issue dedup, repair
clear, and unavailable-store explanation. ROOT's doctor GREEN was pending at
brief; no unreturned runtime result is credited here.

Read-source hashes:

| Path | SHA256 |
|---|---|
| cmd/ze/hub/bootstrap_credentials.go | 8eae3db09efcbc0b0b121abc4c73869fd6e593142cc5236697559eca8342cb23 |
| cmd/ze/hub/register_bootstrap_credentials.go | f3eac17e61e615c55d0fbe9683f27e387bfc0facd3ff0f4e55cace345f83bb96 |
| cmd/ze/hub/bootstrap_credential_probe_test.go | 066d74fb7983b3172d417e6dec6660d737c83cbe0143599219cb191c5ad6d632 |
| cmd/ze/hub/bootstrap_credential_diagnostic_test.go | 49f144b8e3427d7449aef3fbe0501e649436bca266ddcd5264ba631a49be85fe |
| cmd/ze/hub/bootstrap_certificate_login_test.go | 149188d24e1a401b9ccd3562ef2278b205510cce4302a50ba755c71a2f34828e |
| internal/appliance/bootstrap_certificates_test.go | 25ada828f968b1e598f46fd286c5000f93c7c6bf5bfd7ccb0cec5a485f922bc8 |
| internal/core/authorizedkeys/authorizedkeys.go | b40b4e21d73e9412e31e9b43af9ada462bd871862e08558e221221ca155fde0c |
| internal/core/diagnostic/codes.go | b7e6eeb2138bb79ac4aea605bfb5a057802b9da6947334fc960986a15f255b21 |

No new source issue from these bounded repairs. Full retained historical431
population including foreign IKE remains; later immutable census and ROOT
runtime/native installed-appliance/delivery proofs still govern formal closure.
No tests/builds/formatters/product execution, tracked edits, commits or children
by reviewer in this continuation.

## Follow-on port-allocation fixture — APPROVE(a), before edit

Exact candidate SHA256:
440eadb7f9078342b4fed9ad97eae94c387b26eee588d3b691d737588c1cee6f.
Current newConfigPushProtocolFixture was still the old three SSH allocations
when read, so this judgment precedes that exact edit.

Read configPushOne's cfg.SSH.Port dial, resolveSeedConfig and
appendListenerOverrides installing that same port, private executeApplianceConfigPush
allocation/named-init and configPushSeed producer, and shared launchOTAGuestMode
patch. The required named SSH port is now allocated once before build and
retained in the initial guest listener. All three boots forward host p to guest
p; each consumes only its updater/web allocations. The old OTA wrapper still
passes zero and allocates a host port forwarding to guest22. This is fixture
scheduling against the real producer contract, not a NAT feature or an extra
management listener. Existing sequence/build/boot/payload/state/refusal/login/
cleanup assertions remain byte-for-byte; only the allocation list changes.

Test-judge: internal/le/test/qemu/install_config_push_protocol_test.go: APPROVE(a): exact440eadb7 fixture follows one pre-build named SSH port across all three boots while preserving every orchestration and refusal assertion.

No historical native unit row was invented for the new untracked helper.
This is not a whole native-action source review or a native VM proof. ROOT owns
application and execution; the runner source was still held private for its
scheduled source release. No source edits or execution by this reviewer.

ROOT clarified the seven formatted Go paths were the six doctor paths above
plus the separately reviewed authorizedkeys certificate guard: no missing
seventh doctor file. ROOT now reports all3doctor parents GREEN6bc651c7 and
actual agent Sign/Verify/cleanup GREEN33bd91e2. Native VM proof remains owed.
