# Spec: local-ca

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/local-ca.md` |
| Handoff | - |
| Updated | 2026-09-03 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze authenticates its own components by pinning a fingerprint, and the pin has no
way to fail safe. `TLSConfigWithFingerprint` (`internal/component/plugin/ipc/tls.go`)
sets `InsecureSkipVerify` and compares a SHA-256 of the peer's leaf inside a
`VerifyConnection` closure. An EMPTY fingerprint compares nothing, so the
connection is accepted blind. `dialAndAuth` (`pkg/plugin/sdk/sdk.go`) is the
caller that can still reach that state.

The certificate being pinned is minted per acceptor by `GenerateSelfSignedCert`,
lives 24 hours, is never persisted, and carries one SAN of 127.0.0.1. The
fingerprint reaches the far end in the environment as `ZE_PLUGIN_CERT_FP`. The
managed client takes the same anchor through `clientTLSConfig`
(`internal/component/managed/tls.go`). Appliance push has the same shape one
layer up: `loadDeviceTLS` (`internal/appliance/cmd_push.go`) builds a pool
holding the device's own leaf, so trust is a copy of one certificate rather than
an issuer that can issue another.

A pin cannot rotate. It cannot express "this certificate is no longer ours", and
it breaks the moment the certificate it names is replaced, which is why the
managed hub's certificate had to be made stable before a pin on it was
meaningful.

Ze cannot issue a certificate. `internal/component/pki/` is load-validate-serve:
`parseCACert` reads a `certificate` and no key, the only key-bearing entry is a
leaf, and nothing in the package calls `x509.CreateCertificate`. No CSR path
exists in first-party code. The config store cannot hold a CA key either,
because the `$9$` encoding is obfuscation rather than encryption and its own
package doc says the encoded value always decodes back to plaintext.

The goal is a certificate authority inside Ze, in the shape Caddy's `tls internal`
uses: a root generated once and kept, leaves issued from it, and the root
distributed so a far end trusts an ISSUER instead of a copy of one certificate.
The root replaces the pin rather than joining it, so `certificate-fingerprint`
and every mechanism that carried it are deleted.

`plan/spec-managed-server-hardening.md` is live over the managed half of this
rail and wanted exactly this. It rejected persisting a generated pair because
that "leaves the operator with a certificate no CA issued and no way to rotate it
through config", and kept the pin as a stand-in for "what a private fleet CA
produces". This spec removes both costs it named. It does not undo that spec's
work: `plugin/hub/server/certificate` stays and still wins when set.

Two decisions are taken and are not open. The root private key lives in zefs at
mode 0600, where every TLS private key in Ze lives today, and file permissions
are its only protection. And `github.com/smallstep/certificates`, which is what
Caddy uses, is NOT taken as a dependency: it is a CA server with a database,
ACME provisioners and SSH certificates, where the part Ze needs is issuance over
`crypto/x509` and the store that already exists.

Out of scope, named rather than forgotten. The operator-facing listeners keep
their present path: web, the looking glass and the DoT and DoH fallback still
mint a self-signed leaf when no certificate is named, because a CA-issued leaf
is exactly as untrusted as a self-signed one until a human installs the root,
and Ze cannot install a root in a stranger's browser. Mutual TLS is out of
scope: `handleConn` authenticates by shared secret and `StartListeners` sets no
`ClientAuth`, so client certificates are a further step this spec makes possible
and does not take. SSH host certificates are out of scope, though
`hostKeyWithCertOption` (`internal/component/ssh/ssh.go`) already reads a host
certificate that nothing in the tree writes, which is the clearest single sign
that issuance is the missing capability.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/pki/pki-store.md` - what the store holds and what it cannot
  → Constraint: the store is load-validate-serve, driven by parsed config. A runtime-generated root is not config, so it cannot live in a `pki` block and must not pretend to.
  → Decision: `pki ca <name> certificate` already exists and holds a CA certificate with no key. That is exactly the shape a CLIENT needs to trust a hub's root, so the client side of this feature needs no new storage.
- [ ] `docs/architecture/pki/tls-listeners.md` - the fail-closed contract
  → Constraint: a configured certificate name that does not resolve is an error and no material, never a fallback. Issuance must obey the same rule: a named certificate wins, and a failure to resolve it is not answered by issuing one.
- [ ] `docs/architecture/fleet-config.md` - how a client is told which hub to trust
  → Constraint: the page documents `certificate-fingerprint` as the anchor for a hub certificate no system CA issued, and says the operator copies it from the hub's log line. That paragraph is replaced, not amended: the root is what gets copied now.
- [ ] `docs/architecture/plugin/plugin-system.md` - the plugin process boundary
  → Constraint: an external plugin process cannot import `internal/`, so whatever anchors it must arrive through the environment or the wire. The fingerprint arrives as `ZE_PLUGIN_CERT_FP` today, which is the slot the root PEM takes over.
- [ ] `ai/rules/no-layering.md` - replacement, not coexistence
  → Constraint: the root and the pin answer one question, so the pin is deleted before the root is written. A bootstrap-only pin beside a root is the hybrid this rule refuses.
- [ ] `ai/patterns/cli-command.md` - the structural template for the export verb
  → Constraint: keyword before value, and the response is structured data so `| json`, `| yaml` and `| table` each render it.

### RFC Summaries (Scope: protocol)

N-A. Scope is `plugin`. The feature changes which certificate Ze presents to its
own components and what they verify it against. It changes no wire format. TLS
itself is `crypto/tls`, and X.509 path validation is `crypto/x509`.

**Key insights:** (minimal context to resume after compaction)
- The pin's failure mode is an empty anchor accepted as a pass. `clientTLSConfig` guards it; `dialAndAuth` does not.
- `loadDeviceTLS` already loops over every CERTIFICATE block in the device file and adds each to the pool, so appliance push trusts an issuer with no code change once the root is written into that file.
- A serial ledger is not needed. 128 random bits per certificate is the standard answer, and `selfcert.GenerateWebCertWithNames` already draws exactly that.
- An intermediate is not needed. Its purpose is to keep the root offline, and Ze's root lives on the same box as the issuer, so the intermediate would add a layer that protects nothing.
- `plan/spec-managed-server-hardening.md` marks three rows NOT DONE that are landed in HEAD. Only its doctor check and its two-daemon functional test are outstanding, and its Review Gate was never run.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/plugin/ipc/tls.go` - `GenerateSelfSignedCert` mints an ECDSA P-256 self-signed leaf, SAN 127.0.0.1, 24 hours, never persisted. `TLSConfigWithFingerprint` returns `InsecureSkipVerify` with a `VerifyConnection` closure comparing a hex SHA-256, and returns the same config with no closure when the fingerprint is empty. `CertFingerprint` produces the hex.
- [ ] `internal/component/plugin/acceptor.go` - `NewHubAcceptor` mints that certificate and stores its fingerprint as `certFP`.
- [ ] `internal/component/plugin/process/process.go` - sets `ZE_PLUGIN_CERT_FP` in the child environment from the acceptor's fingerprint.
- [ ] `pkg/plugin/sdk/sdk.go` - `dialAndAuth` reads `ze.plugin.cert.fp` and passes it to `TLSConfigWithFingerprint`, including when it is empty.
- [ ] `internal/component/managed/tls.go` - `clientTLSConfig` pins when a fingerprint is present, falls back to `TLSInsecure`, then to the system CA pool. The fingerprint also arrives from the env var `ze.managed.tls.certificate-fingerprint`.
- [ ] `internal/component/plugin/server/managed_serve.go` - `managedCertificate` resolves `cfg.Certificate` through the injected resolver, and returns `GenerateSelfSignedCert()` when the name is empty. `handleConn` authenticates by shared-name secret; the listener sets no `ClientAuth`.
- [ ] `internal/component/pki/config.go` - `parseCACert` reads only `certificate`; `parseDeviceCert` is the only entry carrying a private key.
- [ ] `internal/component/pki/store.go` - `Load`, `Validate`, `GetCA`, `CAPool`, `GetCertificate`, `ExportPEM`. Nothing issues.
- [ ] `internal/appliance/cmd_push.go` - `loadDeviceTLS` reads the device's `cert.pem` and adds every CERTIFICATE block it finds to a pool.
- [ ] `internal/appliance/cmd_init.go` - `writeTLSSecrets` mints the appliance's web leaf on the build host through `selfcert.GenerateWebCertWithNames`, written by `writeTLSPair`.
- [ ] `pkg/zefs/keys.go` - every stored key registers a pattern, a description and a `Private` flag that hides it from listing. `KeyWebCert` and `KeyWebKey` are the model.
- [ ] `internal/component/config/secret/secret.go` - the package doc states `$9$` is obfuscation, not encryption, and always decodes back.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `plugin/hub/server/certificate` wins when set. An operator-named PKI certificate is served, and a name that does not resolve is an error rather than a fallback.
- `checkManagedCertificateAgreement` keeps refusing two serving blocks that name different certificates.
- Shared-secret authentication in `handleConn` is unchanged. TLS says who the peer is; the secret says whether it may talk.
- The web, looking-glass, DoT and DoH self-signed paths, exactly as they are.
- `loadDeviceTLS` reads the same file and builds the same pool. Only what is written into that file changes.

**Behavior to change:** (only what the user asked for)
- A root is generated once, persisted, and reused. Leaves are issued from it.
- Plugin IPC, the managed hub and the managed client validate a chain instead of comparing a fingerprint.
- An empty trust anchor refuses the connection rather than accepting it.
- `certificate-fingerprint`, `ze.managed.tls.certificate-fingerprint`, `ZE_PLUGIN_CERT_FP`, `TLSConfigWithFingerprint`, `CertFingerprint` and `GenerateSelfSignedCert` are deleted.
- The appliance's build-host certificate becomes a leaf issued by an appliance root, and the root is what the device file offers as a trust anchor.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- First need for a certificate on a daemon with no root: the acceptor or the managed listener starting.
- Operator config on a managed CLIENT: `pki ca <name> certificate <base64>` holding the hub's root, named by `plugin hub client ca <name>`.
- Operator command: the export verb that prints the root PEM for distribution.
- `ze appliance init` on the build host, for the appliance path.

### Transformation Path
1. `LoadOrGenerateRoot` reads `meta/ca/cert` and `meta/ca/key` from zefs, or generates the pair once and writes it at mode 0600.
2. `IssueLeaf` signs a leaf for a named purpose with the SANs that purpose needs, drawing a 128-bit random serial.
3. The hub acceptor and the managed listener each take a leaf at start.
4. The root PEM reaches an external plugin process in the environment, replacing the fingerprint in the same slot.
5. A managed client resolves its configured `pki ca` name into a pool through the store, and validates the hub's chain against it.
6. `ze appliance init` generates a root on the build host, issues the web leaf from it, and writes leaf followed by root into `cert.pem`, which `loadDeviceTLS` already reads as a pool.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| pki component ↔ zefs | root read and written through the storage interface at 0600 | No |
| Hub ↔ external plugin process | root PEM in the child environment | No |
| Managed client ↔ pki store | a configured CA name resolved to a pool | No |
| Build host ↔ appliance image | root written into the device certificate file | No |
| pki component ↔ plugin ipc | issuance called across two components | No |

### Integration Points
- `pki.CAPool` - already builds a pool from configured CA entries, which is what the managed client needs.
- `pki.ServerTLSMaterial` - unchanged, and still outranks issuance when a name is configured.
- `zefs` key registry - the root joins it as two registered keys.
- `diagnostic.RegisterDoctorCheck` - the root's health check registers the way every other check does.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A root PEM fits in an environment variable on every platform Ze runs a plugin process on | An ECDSA P-256 root is about 600 bytes of PEM; the slot already carries a 64-character fingerprint | The root needs a file or a wire exchange, and the plugin handshake gains a step | Measure the encoded size, and start a plugin process on Linux and macOS | unvalidated |
| A-2 | `loadDeviceTLS` trusts a device whose file holds leaf followed by root, with no code change | It loops `pem.Decode` and adds every CERTIFICATE block to the pool | Appliance push needs its own change and the zero-code claim is void | A push against an appliance whose leaf was reissued after the file was written | unvalidated |
| A-3 | An operator can be given the root by the same route that gives them a fingerprint today | `docs/architecture/fleet-config.md` says the hub logs the fingerprint and the operator copies it | The export verb is not enough and distribution needs its own mechanism | The functional test copies the exported root into a client config and connects | unvalidated |
| A-4 | `internal/component/plugin/ipc` may import `internal/component/pki` | `internal/component/web/doctor.go` imports pki for the same kind of reason | Issuance needs an injected resolver, as `internal/core/dnsserver` takes for the same tier reason | `./le tier check` after the issuance phase | unvalidated |
| A-5 | Deleting `certificate-fingerprint` breaks no deployment, because Ze is pre-release and the leaf is four days old | `ai/rules/pre-release.md`; the leaf landed 2026-08-29 | A migration path is owed for a config nobody has | Grep the tree and the test corpus for the leaf, and confirm no fixture depends on it | unvalidated |
| A-6 | The managed hub's certificate may become ephemeral again once the pin is gone | `plan/spec-managed-server-hardening.md` made it stable only so a pin on it would survive a restart | The hub leaf must be persisted too, and the root is not sufficient | A client reconnects across a hub restart with no config change | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The root is regenerated on a restart, so every previously distributed copy stops working | A client that connected yesterday is refused today | `LoadOrGenerateRoot` reads before it writes, and AC-1 asserts the same root survives a restart |
| R-2 | An empty anchor keeps being accepted somewhere the enumeration missed | A connection succeeds in a test that supplies no anchor at all | AC-4 drives the refusal from each caller, and the deletion of `TLSConfigWithFingerprint` removes the branch rather than guarding it |
| R-3 | The root private key reaches a log, the CLI, or a config dump | The key appears in test output or in `show` | AC-10 asserts its absence from each surface; the zefs entry registers `Private`, and the export verb prints the certificate only |
| R-4 | Issuance is reachable when a certificate name is configured, so a fail-closed reference silently becomes a working self-issued one | A broken name produces a working listener | The named branch returns before issuance is considered, and a test drives a broken name to an error |
| R-5 | The appliance file now holds two certificates and something downstream expects one | The image builds and the device serves, but a consumer of `cert.pem` parses only the first block | Enumerate every reader of that file, not only `loadDeviceTLS`, before writing the root into it |
| R-6 | Clock skew between hub and client makes a freshly issued leaf not yet valid | A client refuses a leaf the hub just issued | Backdate `NotBefore` by a small margin, as is standard, and state the margin in the code |
| R-7 | Two hub processes on one box each generate a root and overwrite the other | A client trusts one and is refused by the other | The generate path writes once and re-reads; a second writer finds the first root and uses it |
| R-8 | The spec deletes surfaces belonging to a spec still in progress, and that session rebases onto a hole | `spec-managed-server-hardening` edits a leaf this spec removed | Its outstanding work is a doctor check and a functional test, neither touching the fingerprint. Tell that session before the deletion lands |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every plugin process fails to connect to its hub, and a managed fleet stops fetching config. Plugin IPC is how the daemon talks to its own features, so a wrong landing is not a degraded feature, it is a daemon with nothing registered |
| How is it reverted? | Single commit revert, and the generated root becomes two unread zefs keys. No peer, wire, or on-disk format outside those two keys changes. A deployed client config naming a `pki ca` entry keeps parsing after a revert; it is simply unused |
| Who else touches this path? | `plan/spec-managed-server-hardening.md` is in progress over the managed half and created `internal/component/managed/tls.go`. Its remaining work is a doctor check and a functional test |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A daemon starting with no root in zefs | → | `pki.LoadOrGenerateRoot` | `TestRootIsGeneratedOnceAndReused` |
| The hub acceptor starting | → | `pki.IssueLeaf` through `NewHubAcceptor` | `TestHubAcceptorServesAnIssuedLeaf` |
| An external plugin process dialing its hub | → | `dialAndAuth` validating against the root from the environment | `TestPluginDialValidatesTheChain` |
| A managed client with a configured `pki ca` name | → | `clientTLSConfig` building a pool from the store | `TestManagedClientValidatesAgainstConfiguredRoot` |
| An operator exporting the root | → | the export command handler | `TestExportRootPrintsTheCertificateOnly` |
| `ze appliance push` against a device whose leaf was reissued | → | `loadDeviceTLS` over a file holding leaf and root | `TestPushTrustsAReissuedLeaf` |
| `ze doctor` on a daemon whose root is near expiry | → | the CA doctor check | `TestCARootDoctorCheckRegistered` |
| An operator running the whole fleet path | → | root, issuance, export, client trust | `test/managed/managed-hub-ca-trust.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A daemon starts with no root, then restarts | The same root certificate is presented after the restart, and the key file is mode 0600 |
| AC-2 | A plugin process connects to its hub | The connection succeeds by validating the hub's leaf against the root, and no fingerprint is read or compared anywhere on the path |
| AC-3 | A peer presents a certificate the root did not issue | The connection is refused, with an error naming the verification failure |
| AC-4 | A client is given no trust anchor at all | The connection is refused. There is no configuration under which an absent anchor produces a successful connection |
| AC-5 | A config still carries `certificate-fingerprint` | The config is refused with an error naming the certificate authority replacement, rather than being silently ignored |
| AC-6 | `plugin/hub/server/certificate` names a store certificate | That certificate is served and no leaf is issued. A name that does not resolve is an error, and issuance is not reached |
| AC-7 | A leaf expires and the component restarts | A fresh leaf is issued from the same unchanged root, and a client holding the root connects without any operator action |
| AC-8 | The operator runs the export command | The root certificate is printed in PEM, the private key appears nowhere in the output, and the text is directly usable as a client trust anchor |
| AC-9 | `ze appliance push` targets a device whose leaf was reissued since the trust file was written | The push succeeds, because the file's anchor is the issuer rather than the old leaf |
| AC-10 | Any log line, CLI output, or config dump is inspected after issuance | The root private key appears in none of them |
| AC-11 | `ze doctor` runs on a daemon whose root is missing, unreadable, or within 90 days of expiry | A diagnostic is reported for each case, with distinct codes for absence and expiry |
| AC-12 | A managed client and hub run as two real daemons | The client fetches config over a chain it validated against the configured root |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Starts a daemon and watches its plugins register | zefs -> `LoadOrGenerateRoot` -> `IssueLeaf` -> acceptor -> child environment -> `dialAndAuth` | `TestPluginDialValidatesTheChain` |
| 2 | Exports the hub root and pastes it into a client's `pki ca` block | export command -> operator -> client config -> `pki.CAPool` -> `clientTLSConfig` | `test/managed/managed-hub-ca-trust.ci` |
| 3 | Restarts the hub and expects the client to keep working | the same root, a new leaf | `TestManagedClientSurvivesAHubRestart` |
| 4 | Pushes a new image to an appliance whose certificate was reissued | `loadDeviceTLS` -> pool holding the root -> handshake | `TestPushTrustsAReissuedLeaf` |
| 5 | Runs `ze doctor` before the root expires | parsed state -> CA check -> diagnostic | `TestCARootDoctorCheck` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRootIsGeneratedOnceAndReused` | `internal/component/pki/ca_test.go` | Generation happens once; a second call reads what the first wrote (AC-1, R-1) | |
| `TestRootKeyIsWrittenPrivate` | `internal/component/pki/ca_test.go` | Mode 0600 and the registered `Private` flag (AC-1, R-3) | |
| `TestIssueLeafDrawsAUniqueSerial` | `internal/component/pki/ca_test.go` | 128 random bits, and two issuances differ | |
| `TestIssueLeafBackdatesNotBefore` | `internal/component/pki/ca_test.go` | The skew margin exists and is the stated value (R-6) | |
| `TestConcurrentRootGenerationAgrees` | `internal/component/pki/ca_test.go` | Two callers racing to generate end with one root (R-7) | |
| `TestHubAcceptorServesAnIssuedLeaf` | `internal/component/plugin/acceptor_test.go` | A real handshake presents a leaf the root issued (AC-2) | |
| `TestPluginDialValidatesTheChain` | `pkg/plugin/sdk/sdk_test.go` | Chain validation against the root from the environment (AC-2) | |
| `TestPluginDialRefusesAnUnknownIssuer` | `pkg/plugin/sdk/sdk_test.go` | A foreign certificate is refused (AC-3) | |
| `TestPluginDialRefusesWithNoAnchor` | `pkg/plugin/sdk/sdk_test.go` | An empty anchor refuses, which is the defect this replaces (AC-4, R-2) | |
| `TestManagedClientValidatesAgainstConfiguredRoot` | `internal/component/managed/client_tls_test.go` | The configured `pki ca` name becomes the pool (AC-12) | |
| `TestManagedClientRefusesWithNoAnchor` | `internal/component/managed/client_tls_test.go` | Same refusal on the fleet rail (AC-4) | |
| `TestManagedClientSurvivesAHubRestart` | `internal/component/managed/client_tls_test.go` | A new leaf under the same root is accepted (AC-7, A-6) | |
| `TestNamedCertificateOutranksIssuance` | `internal/component/plugin/server/managed_cert_test.go` | A configured name is served and issuance is not reached; a broken name errors (AC-6, R-4) | |
| `TestFingerprintConfigIsRefused` | `internal/component/config/hub_extract_test.go` | The retired leaf is an error naming its replacement (AC-5) | |
| `TestExportRootPrintsTheCertificateOnly` | `internal/component/pki/show_test.go` | PEM out, no key, and the pipe operators render it (AC-8) | |
| `TestPushTrustsAReissuedLeaf` | `internal/appliance/cmd_push_test.go` | The pool holds the issuer, so a new leaf validates (AC-9, A-2) | |
| `TestCARootDoctorCheck` | `internal/component/pki/doctor_test.go` | Missing, unreadable and near-expiry each report (AC-11) | |
| `TestCARootDoctorCheckRegistered` | `internal/component/pki/doctor_test.go` | The check is in the registry with its codes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Root validity | 10 years from issue | the final second before `NotAfter` | `NotBefore` minus the skew margin | one second past `NotAfter` |
| Leaf validity | 24 hours from issue | the final second before `NotAfter` | `NotBefore` minus the skew margin | one second past `NotAfter` |
| Doctor expiry warning | 90 days | 90 days and one second remaining, which does not warn | expired, which is an error rather than a warning | N/A |
| Serial entropy | 128 bits | N/A, it is fixed | fewer bits is a defect, not an input | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-ca-root-export` | `test/plugin/pki-ca-root-export.ci` | The operator exports the root and gets PEM with no key (AC-8, AC-10) | |
| `pki-ca-fingerprint-retired` | `test/parse/pki-ca-fingerprint-retired.ci` | A config carrying the retired leaf is refused with a message naming the replacement (AC-5) | |
| `managed-hub-ca-trust` | `test/managed/managed-hub-ca-trust.ci` | Two daemons: the client fetches config over a validated chain (AC-12). This is also the two-daemon test `spec-managed-server-hardening` left outstanding | |
| `plugin-dial-no-anchor` | `test/plugin/plugin-dial-no-anchor.ci` | A plugin given no anchor fails to register rather than connecting blind (AC-4) | |

### Interop Tests (Scope: protocol)

N-A. Scope is `plugin` and no wire format changes. The nearest interop assertion
is a real `crypto/tls` client completing a handshake and validating a chain with
`crypto/x509`, which the SDK and managed client tests both do against a root
they did not mint themselves.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/plugin/ipc/tls.go` - delete `GenerateSelfSignedCert`, `TLSConfigWithFingerprint` and `CertFingerprint`; the replacement config validates against a root
- `internal/component/plugin/acceptor.go` - `NewHubAcceptor` takes an issued leaf; `certFP` is deleted
- `internal/component/plugin/process/process.go` - the child environment carries the root PEM instead of the fingerprint
- `pkg/plugin/sdk/sdk.go` - `dialAndAuth` validates the chain and refuses an absent anchor
- `internal/component/managed/tls.go` - `clientTLSConfig` builds a pool from the configured CA name; the fingerprint branch and its env registration are deleted
- `internal/component/plugin/server/managed_serve.go` - `managedCertificate` issues from the root when no name is configured
- `internal/component/plugin/yang/` - the hub client's `certificate-fingerprint` leaf is replaced by a `ca` leaf naming a store entry
- `internal/component/config/loader_extract.go` - `HubClientConfig` carries the CA name; the fingerprint field is deleted
- `internal/component/pki/show.go` - the root export verb
- `internal/component/pki/register.go` - the doctor check registration
- `internal/core/diagnostic/codes.go` - the two CA diagnostic codes
- `pkg/zefs/keys.go` - `meta/ca/cert` and `meta/ca/key`, the key marked `Private`
- `internal/appliance/cmd_init.go` - the build host generates a root, issues the web leaf from it, and writes leaf followed by root
- `docs/architecture/pki/pki-store.md` - the store gains an issuer, and where its key lives
- `docs/architecture/fleet-config.md` - the hub certificate and client trust section, where the fingerprint paragraph is replaced
- `docs/architecture/plugin/plugin-system.md` - what a plugin process is given to trust its hub
- `docs/guide/command-reference.md` - the export verb
- `docs/features.md` - the capability
- `docs/architecture/zefs-format.md` - declared by the key registry this spec adds two entries to
- `docs/architecture/config/syntax.md` - declared by the config loader this spec changes; a leaf is removed and another added
- `docs/architecture/appliance/builder.md` - declared by the appliance init path, which stops self-signing and starts issuing
- `docs/features/ai-first.md` - declared by a file this spec touches, and named here as UNAFFECTED: the page describes the agent-facing command contract, and no command envelope, exit code or JSON shape changes. The new export verb is additive and row 3 of the documentation checklist covers it

## Files to Create
- `internal/component/pki/ca.go` - `LoadOrGenerateRoot` and `IssueLeaf`
- `internal/component/pki/ca_test.go` - generation, reuse, serials, skew, concurrency
- `internal/component/pki/doctor.go` - the root health check
- `internal/component/pki/doctor_test.go` - both codes and the warning window
- `test/plugin/pki-ca-root-export.ci` - the export path
- `test/parse/pki-ca-fingerprint-retired.ci` - the retired leaf is refused
- `test/managed/managed-hub-ca-trust.ci` - two daemons over a validated chain
- `test/plugin/plugin-dial-no-anchor.ci` - an absent anchor refuses
- `plan/deferrals/local-ca.md` - the deferral shard named in the metadata table

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/`: the hub client's `ca` leaf replaces `certificate-fingerprint` |
| YANG validation constraints | Yes | The `ca` leaf takes the same `length "1..255"` and `pattern '[A-Za-z0-9._-]+'` the certificate-name leaves carry, since it names a store entry |
| YANG custom validators | No | Existence of a named CA entry is a cross-root reference a per-leaf validator cannot see. It is enforced at start, and by the doctor check, exactly as the certificate name is |
| CLI commands/flags | Yes | The root export verb, structured per `ai/patterns/cli-command.md` |
| CLI grammar (keyword before value) | Yes | The verb takes no positional value; `ai/rules/cli.md` governs it |
| Editor autocomplete | No | The `ca` leaf names a runtime store entry, so it has no enum. Same position as every other certificate-name leaf |
| Functional test for new RPC/API | Yes | `test/plugin/pki-ca-root-export.ci` for the new command |
| Pipe completeness | Yes | The export response is structured data, so `\| json`, `\| yaml` and `\| table` each render it |
| Env var registration | No | The retired `ze.managed.tls.certificate-fingerprint` is deleted and nothing replaces it. The root reaches a plugin process through the process environment the hub sets, which is not an operator-facing `environment/` leaf |
| Doctor check for runtime dependencies | Yes | A root is a runtime dependency: `internal/component/pki/doctor.go`, two new codes in `internal/core/diagnostic/codes.go`, unit and functional coverage |
| Prometheus counters/metrics | No | Issuance is once per component start, not a rate worth a counter. Expiry is a doctor concern, matching how certificate expiry is already surfaced |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/config-reference.md`: a leaf is removed and another added |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the export verb |
| 4 | API/RPC added/changed? | No | No command JSON envelope outside the new verb, which row 3 covers |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: what a plugin process is given to trust its hub |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` if present, and `docs/architecture/fleet-config.md`, whose fingerprint paragraph is replaced |
| 7 | Wire format changed? | No | TLS and X.509 are unchanged; only which certificate is presented and what validates it |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md` and `docs/architecture/api/process-protocol.md`: the environment slot changes meaning |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement moves. Path validation is `crypto/x509` and no `rfc/short/` row changes |
| 10 | Test infrastructure changed? | No | New tests use the existing `.ci` and Go harnesses |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about how Ze authenticates its own components |
| 12 | Internal architecture changed? | Yes | `docs/architecture/pki/pki-store.md` and `docs/architecture/plugin/plugin-system.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata is involved |
| 14 | Prometheus counters added/changed? | No | None added, per the Integration Checklist row |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A registered command and a registered doctor check: `docs/plugin-overview.md` and `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-local-ca.md` is run at implementation and every named page is answered |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every example carrying `certificate-fingerprint` is stale the moment the leaf is deleted, and each is checked against the YANG |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- a root exists and something reaches it
   - Tests: `TestRootIsGeneratedOnceAndReused`, `TestRootKeyIsWrittenPrivate`, `TestHubAcceptorServesAnIssuedLeaf`
   - Files: `pkg/zefs/keys.go`, `internal/component/pki/ca.go`, `internal/component/plugin/acceptor.go`
   - Verify: the acceptor presents a leaf the root issued. The wiring test fails first because the acceptor still self-signs
2. **Phase: Issuance correctness** -- the properties a certificate authority owes
   - Tests: `TestIssueLeafDrawsAUniqueSerial`, `TestIssueLeafBackdatesNotBefore`, `TestConcurrentRootGenerationAgrees`
   - Files: `internal/component/pki/ca.go`
   - Verify: serials differ, the skew margin is the stated value, and two racing callers end with one root
3. **Phase: Delete the pin** -- the replacement, in the order the no-layering rule requires
   - Tests: `TestPluginDialValidatesTheChain`, `TestPluginDialRefusesAnUnknownIssuer`, `TestPluginDialRefusesWithNoAnchor`
   - Files: `internal/component/plugin/ipc/tls.go`, `process.go`, `pkg/plugin/sdk/sdk.go`
   - Verify: the fingerprint symbols are gone rather than unused, and an absent anchor refuses. Grep proves the deletion
4. **Phase: The fleet rail** -- the managed client trusts an issuer
   - Tests: `TestManagedClientValidatesAgainstConfiguredRoot`, `TestManagedClientRefusesWithNoAnchor`, `TestManagedClientSurvivesAHubRestart`, `TestNamedCertificateOutranksIssuance`, `TestFingerprintConfigIsRefused`
   - Files: `internal/component/managed/tls.go`, `managed_serve.go`, the plugin YANG, `loader_extract.go`
   - Verify: a configured root validates the hub across a restart, and the retired leaf is an error naming its replacement
5. **Phase: Export and doctor** -- the operator can distribute it and learn it is failing
   - Tests: `TestExportRootPrintsTheCertificateOnly`, `TestCARootDoctorCheck`, `TestCARootDoctorCheckRegistered`
   - Files: `internal/component/pki/show.go`, `doctor.go`, `register.go`, `internal/core/diagnostic/codes.go`
   - Verify: PEM out with no key, and each doctor case reports its own code
6. **Phase: Appliance** -- the build host issues rather than self-signs
   - Tests: `TestPushTrustsAReissuedLeaf`
   - Files: `internal/appliance/cmd_init.go`
   - Verify: enumerate every reader of the device certificate file before writing a second block into it (R-5), then prove a reissued leaf still pushes
7. **Phase: Functional tests and documentation**
   - Tests: the four `.ci` files
   - Files: the `.ci` files, plus the pages named in the Documentation checklist
   - Verify: the two-daemon scenario fetches config over a validated chain, and no page still tells an operator to copy a fingerprint

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No branch anywhere produces a usable TLS config from an absent anchor. Enumerate the callers rather than trusting the deletion |
| Correctness | A configured certificate name is resolved before issuance is considered, so a fail-closed reference cannot become a working self-issued certificate |
| Naming | The YANG leaf, the Go field and the store entry agree: `ca`, `CA`, `pki ca <name>` |
| Data flow | The root private key leaves zefs only into a signing operation. No path carries it to a log, a command response, or the process environment |
| Rule: `ai/rules/no-layering.md` | The fingerprint symbols are DELETED, not deprecated. A grep for each name returns nothing outside history |
| Rule: `ai/rules/principles.md` | The empty anchor no longer reads as a valid answer, and its replacement says so rather than returning a permissive default |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The root is registered in zefs | `grep 'meta/ca/' pkg/zefs/keys.go` |
| Issuance exists and is called | `grep -rn 'IssueLeaf' internal/ pkg/` names a non-test caller |
| The pin is gone | `grep -rn 'TLSConfigWithFingerprint\|CertFingerprint\|ZE_PLUGIN_CERT_FP\|certificate-fingerprint' internal/ pkg/ docs/` returns nothing |
| No blind accept survives | `grep -rn 'InsecureSkipVerify' internal/ pkg/` and every hit is justified in place |
| The doctor check is registered | `./le run -- ze doctor list` names the CA root check |
| The functional tests pass | `./le functional plugin`, `./le functional parse`, `./le functional managed` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Fail-open | The defect this spec exists to remove: an absent anchor must never yield a usable config. Check every caller, not only the function |
| Key material | The root key reaches a signer and nothing else. Not a log, not a command response, not the child environment, not a config dump |
| Key at rest | Mode 0600 and the `Private` registry flag. This spec does not encrypt it, and says so rather than implying protection it does not provide |
| Trust scope | The root signs leaves for Ze's own components only. It must not become a general-purpose CA an operator can ask to sign arbitrary names |
| Validity | A leaf must not outlive its purpose, and the root must not be so long-lived that a compromise is unbounded. Both lifetimes are stated and tested |
| Error leakage | A verification failure names that verification failed. It does not print the peer's certificate or the expected issuer's key |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- A pin and a root answer one question, so keeping both would be the hybrid the no-layering rule refuses. What made that hard to see is that the pin looks like a bootstrap mechanism rather than a competing answer. It is not: the operator copies a fingerprint today and copies a root tomorrow, by the same route and at the same moment. Only the durability of what they copied changes.
- The strongest argument for this work was written by the spec that could not do it. `spec-managed-server-hardening` rejected an alternative because it produced "a certificate no CA issued and no way to rotate it through config", and kept the pin as a stand-in for "what a private fleet CA produces". A rejected alternative names the missing capability more precisely than a feature request does.
- Two things this spec was expected to build turned out not to be needed, and both were assumptions carried over from how a public CA works. A serial ledger is unnecessary because 128 random bits is the standard answer and `selfcert` already draws it. An intermediate is unnecessary because its purpose is to keep the root offline, and Ze's root lives on the machine that signs with it.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Internal surfaces only: plugin IPC, managed, appliance push | Every TLS consumer including web and the looking glass; the CA alone with no consumers | A CA pays where the root can be distributed, and Ze can distribute it only where Ze is both ends. On a public listener a CA-issued leaf is exactly as untrusted as a self-signed one. A CA with no consumers is not a feature by this repository's own definition |
| The root replaces the pin, and `certificate-fingerprint` is deleted | Keep the pin for bootstrap; keep both with no boundary | Two anchors answering one question is what the no-layering rule refuses, and nothing tells a reader which wins when they disagree. The leaf is four days old and Ze is pre-release, so there is no deployment to migrate |
| The root key lives in zefs at 0600, unencrypted | A `pki ca` private-key leaf; passphrase-encrypted as the appliance store does | The config route is disqualified outright: `$9$` always decodes, so the root would sit recoverable in `show configuration` and in every backup. A passphrase needs an operator present at every start, which a router that reboots at 3am does not have. zefs at 0600 is where every TLS key in Ze already lives, so this is the existing posture rather than a new weakening |
| The root signs leaves directly | Root plus intermediate, as Caddy does | An intermediate exists so the root can be kept offline. Ze's root is on the box that signs with it, so the intermediate would add a layer that protects nothing and a second expiry to manage |
| Random 128-bit serials | An issuance ledger with a counter | Uniqueness per issuer is the requirement, and 128 random bits meets it without persistent state. `selfcert` already draws exactly that |
| The client trusts a root through the existing `pki ca` store slot | A new store, a file path leaf, or a root shipped over the wire | `pki ca <name> certificate` already holds a CA certificate with no key, which is precisely a trust anchor, and `pki.CAPool` already turns those into a pool. The client side needs no new storage at all |
| The root PEM reaches a plugin process in the environment slot the fingerprint used | A file path; a wire exchange before authentication | It is public material, it is about 600 bytes, and the slot already exists for exactly this purpose. A wire exchange before authentication would be trusting what it is meant to verify |
| Do not take `github.com/smallstep/certificates` | Take it, as Caddy does | It is a CA server with a database, ACME provisioners and SSH certificates. Caddy carries that because HTTPS is its job. Ze needs issuance over `crypto/x509` and a store it already has |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- The root key is protected by file permissions and nothing else. No passphrase, no hardware key store, no encryption at rest beyond what the filesystem gives. This is the posture every TLS key in Ze already has, and naming it here is deliberate so that a later decision to change it starts from a stated position.
- No mutual TLS. `handleConn` still authenticates by shared secret and the listener sets no `ClientAuth`. A CA is what makes client certificates possible, and issuing them is a separate spec.
- No revocation. A leaf that must be distrusted before it expires cannot be, short of rotating the root. Leaf lifetimes are short enough that this is a bounded exposure rather than an open one, and a CRL or OCSP responder is a separate spec.
- The operator-facing listeners are untouched. Web, the looking glass and the DoT and DoH fallback keep minting a self-signed leaf when no certificate is named. `plan/spec-lg-pki-certificate.md` gives the looking glass the named-certificate path the web listener already has.
- SSH host certificates are not issued, though `hostKeyWithCertOption` reads one that nothing writes. That is the next consumer a later spec should take, because the code to consume it already exists.
- Distribution of the root is manual, exactly as fingerprint distribution is today. The export verb prints it; the operator carries it.

## RFC Documentation (Scope: protocol)

N-A. Scope is `plugin`. No RFC-governed protocol behavior is implemented or
changed. X.509 path validation is performed by `crypto/x509`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
