# Spec: local-ca

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 7/7 |
| Deferral shard | `plan/deferrals/local-ca.md` |
| Handoff | - |
| Updated | 2026-09-04 |

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
| pki component ↔ zefs | root read and written through the storage interface at 0600 | Yes -- `TestRootKeyIsWrittenPrivate` asserts the blob file's mode and the `Private` registry flag |
| Hub ↔ external plugin process | root PEM in the child environment | Yes -- `test/plugin/plugin-dial-no-anchor.ci`, which removes `ZE_PLUGIN_CA_PEM` and restores it in one process |
| Managed client ↔ pki store | a configured CA name resolved to a pool | Yes -- `TestManagedClientValidatesAgainstConfiguredRoot`, and `managed-hub-ca-trust` end to end |
| Build host ↔ appliance image | root written into the device certificate file | Yes -- `TestInitWritesLeafBeforeRoot` and `TestPushTrustsAReissuedLeaf` |
| pki component ↔ plugin ipc | issuance called across two components | Yes -- INJECTED, not imported: `plugin.Authority` and `ManagedServerConfig.Authority`, met at `cmd/ze/hub`. A-4's cycle is what forced it |

### Integration Points
- `pki.CAPool` - already builds a pool from configured CA entries, which is what the managed client needs.
- `pki.ServerTLSMaterial` - unchanged, and still outranks issuance when a name is configured.
- `zefs` key registry - the root joins it as two registered keys.
- `diagnostic.RegisterDoctorCheck` - the root's health check registers the way every other check does.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | A configured `plugin/hub/server/certificate` is resolved through `TLSMaterialResolver` and returns BEFORE issuance is considered (`managedCertificate`); `TestNamedCertificateOutranksIssuance/broken-name-errors-rather-than-issuing` proves the fail-closed reference never becomes a self-issued one |
| No unintended coupling (components stay isolated) | Yes | `internal/component/plugin` declares `Authority` and imports no pki. `go list` shows no new edge: the production `*pki.Root` is supplied at `cmd/ze/hub`, the composition root |
| No duplicated functionality (extends existing, does not recreate) | Yes | The client trust anchor reuses `pki ca <name>`; `selfcert.WebCertHosts` became the ONE declaration of the web SAN set, taken by both the self-signed and the issued path; `codeCARootInvalid` reuses `doctor-tls-invalid` rather than spelling a third name for one finding |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Nothing here is on a wire hot path. Issuance runs once per component start and `ServingLeaf.Certificate` runs once per handshake, which is the control plane |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The doctor check registers through `diagnostic.RegisterDoctorCheck` in `pki/register.go`; the export verb registers as an `RPCRegistration` and a YANG command node; the two zefs keys register through `MustRegister`. No switch, factory or central list was edited |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A root PEM fits in an environment variable on every platform Ze runs a plugin process on | An ECDSA P-256 root is about 600 bytes of PEM; the slot already carries a 64-character fingerprint | The root needs a file or a wire exchange, and the plugin handshake gains a step | Measure the encoded size, and start a plugin process on Linux and macOS | confirmed: a 10-year P-256 root is 369 bytes of DER and 554 of PEM; the child environment in `internal/component/plugin/process/process.go` appends to `os.Environ()` and sets no size limit, so the bound is the OS `ARG_MAX` |
| A-2 | `loadDeviceTLS` trusts a device whose file holds leaf followed by root, with no code change | It loops `pem.Decode` and adds every CERTIFICATE block to the pool | Appliance push needs its own change and the zero-code claim is void | A push against an appliance whose leaf was reissued after the file was written | confirmed: `loadDeviceTLS` (`internal/appliance/cmd_push.go`) was not edited, and `TestPushTrustsAReissuedLeaf` pushes to a device serving a leaf the trust file never held |
| A-3 | An operator can be given the root by the same route that gives them a fingerprint today | `docs/architecture/fleet-config.md` says the hub logs the fingerprint and the operator copies it | The export verb is not enough and distribution needs its own mechanism | The functional test copies the exported root into a client config and connects | confirmed: `test/managed/managed-hub-ca-trust.ci` PASS 97.4s. `caTrustExportRoot` runs `show pki local-ca pem` inside the hub, writes the PEM, and `caTrustClientConfig` pastes those exact bytes into the client's `pki ca` block |
| A-4 | `internal/component/plugin/ipc` may import `internal/component/pki` | `internal/component/web/doctor.go` imports pki for the same kind of reason | Issuance needs an injected resolver, as `internal/core/dnsserver` takes for the same tier reason | `./le verify lint run` after the issuance phase, which compiles the tree | BROKEN: `internal/component/pki` already reaches `plugin/ipc` (`pki/show.go` imports `plugin/server`, which imports `pluginipc` in `managed_serve.go`), so the edge is an import cycle rather than a tier verdict. Issuance is injected, per this row's own If-wrong column |
| A-5 | Deleting `certificate-fingerprint` breaks no deployment, because Ze is pre-release and the leaf is four days old | `ai/rules/pre-release.md`; the leaf landed 2026-08-29 | A migration path is owed for a config nobody has | Grep the tree and the test corpus for the leaf, and confirm no fixture depends on it | confirmed, with additions the Files list now carries: `internal/component/managed/client.go`, `internal/component/plugin/types.go`, `cmd/ze/ze_core_start.go`, `internal/component/config/hub_certificate_test.go`, `docs/plugin-development/README.md`, `docs/architecture/api/process-protocol.md`, `test/plugin/pki-certificate-export-show.ci` |
| A-6 | The managed hub's certificate may become ephemeral again once the pin is gone | `plan/spec-managed-server-hardening.md` made it stable only so a pin on it would survive a restart | The hub leaf must be persisted too, and the root is not sufficient | A client reconnects across a hub restart with no config change | BROKEN as stated: `managedCertificate` (`internal/component/plugin/server/managed_serve.go`) is called once from `NewManagedServer`, and its no-name branch already returns a fresh 24-hour self-signed leaf on every start. The hub certificate is ALREADY ephemeral across restarts, so issuance from a persisted root is what first makes a restart survivable |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The root is regenerated on a restart, so every previously distributed copy stops working | A client that connected yesterday is refused today | `LoadOrGenerateRoot` reads before it writes, and AC-1 asserts the same root survives a restart |
| R-2 | An empty anchor keeps being accepted somewhere the enumeration missed | A connection succeeds in a test that supplies no anchor at all | AC-4 drives the refusal from each caller, and the deletion of `TLSConfigWithFingerprint` removes the branch rather than guarding it |
| R-3 | The root private key reaches a log, the CLI, or a config dump | The key appears in test output or in `show` | AC-10 asserts its absence from each surface; the zefs entry registers `Private`, and the export verb prints the certificate only |
| R-4 | Issuance is reachable when a certificate name is configured, so a fail-closed reference silently becomes a working self-issued one | A broken name produces a working listener | The named branch returns before issuance is considered, and a test drives a broken name to an error |
| R-5 | The appliance file now holds two certificates and something downstream expects one | The image builds and the device serves, but a consumer of `cert.pem` parses only the first block | Enumerated: `loadDeviceTLS` loops, and `certExpiry` (`cmd_show.go`), `validateTLSPair` (`cmd_cert.go`) and `checkCertExpiry` (`internal/component/doctor/checks_tls.go`) each read the FIRST block only. Leaf first, root second keeps all four correct, and the ordering is asserted rather than assumed |
| R-6 | Clock skew between hub and client makes a freshly issued leaf not yet valid | A client refuses a leaf the hub just issued | Backdate `NotBefore` by a small margin, as is standard, and state the margin in the code |
| R-7 | Two goroutines in one daemon each generate a root | A client trusts one and is refused by the other | Generation is serialized inside the process, and `TestConcurrentRootGenerationAgrees` proves one root survives. The CROSS-PROCESS half is out of reach and is not this spec's: `BlobStore` takes no file lock, which `TestBlobStoreNoFlock` (`pkg/zefs/store_test.go`) asserts deliberately, so a second daemon sharing one blob already replaces arbitrary state and not only the root. It gets one journal row (owner decision, 2026-09-03) |
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
| AC-1 | A daemon starts with no root, then restarts | The same root certificate is presented after the restart. The private key is held in the zefs blob, whose on-disk file is mode 0600, and its key entry is registered `Private` so no listing shows it |
| AC-2 | A plugin process connects to its hub | The connection succeeds by validating the hub's leaf against the root, and no fingerprint is read or compared anywhere on the path |
| AC-3 | A peer presents a certificate the root did not issue | The connection is refused, with an error naming the verification failure |
| AC-4 | A client is given no trust anchor at all | The connection is refused. No anchor that is SUPPOSED to be checked can be silently empty and pass, which is the defect this spec exists to remove. `ze.managed.tls.insecure` survives and is not that defect: it is a named, logged operator choice to check nothing, so the operator who set it knows what they have (owner decision, 2026-09-03) |
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
| `TestRootIsGeneratedOnceAndReused` | `internal/component/pki/ca_test.go` | Generation happens once; a second call reads what the first wrote (AC-1, R-1) | PASS |
| `TestRootKeyIsWrittenPrivate` | `internal/component/pki/ca_test.go` | The key entry is registered `Private`, so it is absent from a listing, and the backing blob file is mode 0600 (AC-1, R-3) | PASS |
| `TestIssueLeafDrawsAUniqueSerial` | `internal/component/pki/ca_test.go` | 128 random bits, and two issuances differ | PASS |
| `TestIssueLeafBackdatesNotBefore` | `internal/component/pki/ca_test.go` | The skew margin exists and is the stated value (R-6) | PASS |
| `TestIssueLeafRefusesALeafThatIdentifiesNothing` | `internal/component/pki/ca_test.go` | A leaf with no SAN or no common name is refused at issuance, not at the far end's handshake | PASS |
| `TestConcurrentRootGenerationAgrees` | `internal/component/pki/ca_test.go` | Two callers racing to generate end with one root (R-7) | PASS |
| `TestIssuanceRefusesANonPositiveValidity` | `internal/component/pki/ca_test.go` | Both lifetime-naming entry points refuse a certificate that is expired when issued | PASS |
| `TestIssueLeafForHonoursTheLifetimeItIsGiven` | `internal/component/pki/ca_test.go` | A leaf minted once lives as long as the caller asked, which is what the appliance depends on | PASS |
| `TestHubAcceptorServesAnIssuedLeaf` | `internal/component/plugin/acceptor_test.go` | A real handshake presents a leaf the root issued (AC-2) | PASS |
| `TestHubAcceptorRefusesWithoutAnIssuer` | `internal/component/plugin/acceptor_test.go` | A nil issuer is an error at construction, not a self-signed fallback (AC-4, R-2) | PASS |
| `TestPluginDialValidatesTheChain` | `pkg/plugin/sdk/sdk_test.go` | Chain validation against the root from the environment (AC-2) | |
| `TestPluginDialRefusesAnUnknownIssuer` | `pkg/plugin/sdk/sdk_test.go` | A foreign certificate is refused (AC-3) | |
| `TestPluginDialRefusesWithNoAnchor` | `pkg/plugin/sdk/sdk_test.go` | An empty anchor refuses, which is the defect this replaces (AC-4, R-2) | |
| `TestManagedClientValidatesAgainstConfiguredRoot` | `internal/component/managed/client_tls_test.go` | The configured `pki ca` name becomes the pool (AC-12) | |
| `TestManagedClientRefusesWithNoAnchor` | `internal/component/managed/client_tls_test.go` | Same refusal on the fleet rail (AC-4) | |
| `TestManagedClientSurvivesAHubRestart` | `internal/component/managed/client_tls_test.go` | A new leaf under the same root is accepted (AC-7, A-6) | |
| `TestNamedCertificateOutranksIssuance` | `internal/component/plugin/server/managed_cert_test.go` | A configured name is served and issuance is not reached; a broken name errors (AC-6, R-4) | |
| `TestFingerprintConfigIsRefused` | `internal/component/config/hub_extract_test.go` | The retired leaf is an error naming its replacement (AC-5) | |
| `TestExportRootPrintsTheCertificateOnly` | `internal/component/pki/show_test.go` | PEM out, no key, and the pipe operators render it (AC-8) | |
| `TestPushTrustsAReissuedLeaf` | `internal/appliance/cmd_push_test.go` | The pool holds the issuer, so a new leaf validates (AC-9, A-2) | PASS |
| `TestPushRefusesALeafFromAnotherAuthority` | `internal/appliance/cmd_push_test.go` | Trusting an issuer is not trusting anybody: a leaf another appliance's root signed is refused (AC-9) | PASS |
| `TestInitWritesLeafBeforeRoot` | `internal/appliance/ca_test.go` | cert.pem holds the leaf then the root, the leaf verifies against it, and certExpiry reports the leaf (R-5) | PASS |
| `TestApplianceRootOutlivesItsLeaf` | `internal/appliance/ca_test.go` | The appliance root takes `tls.validity-years` plus a one-year margin, so a leaf never outlives its issuer | PASS |
| `TestRekeyReEncryptsTheCertificateAuthorityKey` | `internal/appliance/ca_test.go` | `ze appliance rekey` re-encrypts the CA key, and the next `replace-cert` reaches the SAME root | PASS |
| `TestApplianceRootStoreRefusesAnUnknownKey` | `internal/appliance/ca_test.go` | The build-host root store answers only for the two registered CA keys; anything else is an error, not a path | PASS |
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
- `internal/component/plugin/types.go` - `HubClientConfig.CertificateFingerprint` becomes `CA`
- `internal/component/managed/client.go` - `ClientConfig.CertificateFingerprint` becomes `CA`
- `cmd/ze/ze_core_start.go` - the two call sites that carry the fingerprint
- `cmd/ze/hub/main.go` - CORRECTED at implementation: `runYANGConfig` is where the store handle and the plugin manager meet, so it is where the root is loaded and the issuer injected, not `ze_core_start.go`
- `internal/component/plugin/manager/manager.go` - `Manager` carries the injected issuer and passes it to `NewHubAcceptor`
- `internal/component/plugin/manager/manager_hub_test.go` - the acceptor test supplies an issuer
- `pkg/zefs/registry_test.go` - `TestPrivateKeysMarked` holds the list of Private patterns and gains the root key
- `internal/component/config/hub_certificate_test.go` - asserts the retired leaf parses; it now asserts the CA name parses and the retired leaf is refused
- `internal/component/plugin/ipc/tls_test.go` - every test over the three deleted symbols
- `internal/component/plugin/server/managed_cert_test.go` - the fingerprint assertions
- `internal/component/managed/client_tls_test.go` - the fingerprint assertions
- `test/plugin/pki-certificate-export-show.ci` - carries the retired leaf
- `internal/component/pki/show.go` - the root export verb
- `internal/component/pki/register.go` - the doctor check registration
- `internal/core/diagnostic/codes.go` - the two CA diagnostic codes
- `pkg/zefs/keys.go` - `meta/ca/cert` and `meta/ca/key`, the key marked `Private`
- `internal/appliance/cmd_init.go` - the build host generates a root, issues the web leaf from it, and writes leaf followed by root
- `internal/appliance/cmd_rekey.go` - ADDED at implementation: the CA key file joins the re-encrypted list, or it survives a rekey under the old passphrase and every later `replace-cert` fails to load the root
- `internal/appliance/cmd_cert.go` - ADDED at implementation: `replace-cert` no longer says "self-signed"; it reissues from the appliance authority
- `internal/core/selfcert/selfcert.go` - ADDED at implementation: `WebCertHosts` is the one declaration of the web SAN set, taken by the self-signed path and by the appliance's issuance
- `docs/architecture/pki/pki-store.md` - the store gains an issuer, and where its key lives
- `docs/architecture/fleet-config.md` - the hub certificate and client trust section, where the fingerprint paragraph is replaced
- `docs/architecture/plugin/plugin-system.md` - what a plugin process is given to trust its hub
- `docs/architecture/api/process-protocol.md` - the child environment table and the fork description, both naming the fingerprint slot
- `docs/plugin-development/README.md` - the environment table and the pinning paragraph
- `docs/guide/command-reference.md` - the export verb
- `docs/features.md` - the capability
- `docs/architecture/zefs-format.md` - declared by the key registry this spec adds two entries to
- `docs/architecture/config/syntax.md` - declared by the config loader this spec changes; a leaf is removed and another added
- `docs/architecture/appliance/builder.md` - declared by the appliance init path, which stops self-signing and starts issuing
- `docs/architecture/appliance/ota-push.md` - ADDED at implementation: its trust row said devices carry self-signed certificates, which the change makes wrong
- `docs/guide/appliance.md` - ADDED at implementation: the tls directory listing, what `replace-cert` does, and the push trust paragraph
- `docs/architecture/web-interface.md`, `docs/guide/web-interface.md` - ADDED at implementation: both carry a source anchor on `selfcert.go`, where the SAN set became `WebCertHosts`
- `docs/architecture/hub-architecture.md` - declared by `cmd/ze/hub/main.go`, and named here as UNAFFECTED: the page makes no claim about TLS material or about which certificate any listener serves
- `docs/architecture/plugin/component-boundaries.md` - declared by `internal/component/plugin/manager/manager.go`, and named here as UNAFFECTED: it draws the component boundary and says nothing about the acceptor's certificate
- `docs/architecture/plugin-manager-wiring.md` - declared by `internal/component/plugin/manager/manager.go`, and named here as UNAFFECTED: its Phase 1 list says PluginManager "sets up TLS acceptor for external plugin connect-back", which stays true when the certificate is issued rather than self-signed
- `docs/architecture/system-architecture.md` - declared by `internal/component/plugin/acceptor.go`, and named here as UNAFFECTED: its "Plugin TLS Transport" paragraph describes the listener, the three `ZE_PLUGIN_HUB_*` environment variables and the MuxConn connection, and names neither the certificate's provenance nor the fingerprint slot. What signs the leaf changes; nothing that paragraph asserts does
- `docs/features/ai-first.md` - declared by a file this spec touches, and named here as UNAFFECTED: the page describes the agent-facing command contract, and no command envelope, exit code or JSON shape changes. The new export verb is additive and row 3 of the documentation checklist covers it

## Files to Create
- `internal/component/pki/ca.go` - `LoadOrGenerateRoot` and `IssueLeaf`, plus `LoadOrGenerateRootFor` and `IssueLeafFor`, which name the lifetime at the call site for a caller that mints once instead of at every start
- `internal/component/pki/ca_test.go` - generation, reuse, serials, skew, concurrency
- `internal/appliance/ca.go` - ADDED at implementation: `applianceRootStore` over the two files beside cert.pem, and `issueWebLeaf`
- `internal/appliance/ca_test.go` - ADDED at implementation: block order, the lifetime rule, the rekey, and the store's refusals
- `internal/component/plugin/acceptor_test.go` - the acceptor's handshake against the root, and its refusal with no issuer
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
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | ANSWERED. `docs/guide/status.md` carries the doctor check. `docs/plugin-overview.md` is UNAFFECTED and was checked rather than assumed: its inventory lists plugins that have an implementation, and `internal/plugins/pki-cmd` holds a `yang/` directory and nothing else, so the page never named it and gains no row |
| 16 | Any changed source file referenced by existing doc source anchors? | ANSWERED, clean. `./le spec citation anchors spec plan/spec-local-ca.md` exits 0 with no broken or unresolved anchor. Its one note is advisory: 34 pages MENTION a file this spec touched without being named in Files to Modify, which is the same population mismatch `plan/journal/gate-fires-outside-its-population.md` records for `./le doc wiring`. Mentioning a file is not carrying a claim about it. The claims were found by subject instead, and three pages the spec had NOT named were repaired for it: `docs/architecture/system-architecture.md` listed the child environment without the trust anchor, `docs/features.md` said the root is kept "at mode 0600" when ZeFS applies no per-key mode, and `internal/component/plugin/cli/cli.go`'s own path had set `InsecureSkipVerify` |
| 17 | Existing docs show config/CLI/API examples for this area? | ANSWERED. Every hit of the retired names across `docs/`, `test/`, `internal/`, `pkg/` and `cmd/` was read: 25 in total, and every one is either the unrelated `ze-show:pki-certificate-fingerprint` command, the `retiredKeywords` mechanism that refuses the leaf BY NAME for AC-5, or prose describing the retirement so a reader with an old config can find the new leaf. No example still shows a working pin |

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
| The pin is gone | `grep -rn 'TLSConfigWithFingerprint\|CertFingerprint\|ZE_PLUGIN_CERT_FP\|ze\.plugin\.cert\.fp\|hub/client/certificate-fingerprint' internal/ pkg/ docs/ test/` returns only the survivors named here, each read and each legitimate. No live pin remains. The survivors are of three kinds. `ze-show:pki-certificate-fingerprint` is an unrelated command printing the digest of a STORED operator certificate (`handleShowPKICertificateFingerprint`, `internal/component/pki/show.go`), and it carries its YANG row, its five `store_test.go` cases and its wire-method snapshot line with it. `retiredKeywords` (`internal/component/config/retired.go`) and its test name the leaf deliberately, because refusing it BY NAME is AC-5. And four documentation lines plus three code comments describe the retirement, which is the reader's only route from an old config to the new one. A grep that returned literally nothing would mean the retirement was silent |
| No blind accept survives | `grep -rn 'InsecureSkipVerify' internal/ pkg/` and every hit is justified in place |
| The doctor check is registered | `TestCARootDoctorCheckRegistered` asserts `diagnostic.Lookup` answers for both codes, which is stronger than a listing: it fails when the code is spelled but never registered, the loss `plan/journal/concurrent-session-corruption.md` records for this same file. There is no `ze doctor list` subcommand; the earlier wording named one that does not exist |
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
| Issuance is INJECTED into the acceptor and the managed listener, not imported | `plugin/ipc` imports `pki` directly; move issuance to `internal/core` | `internal/component/pki` already reaches `plugin/ipc` through `pki/show.go` -> `plugin/server` -> `pluginipc`, so the direct import is a cycle. `ManagedServerConfig.TLSMaterialResolver` is the same problem already solved in this tree, and `cmd/ze/ze_core_start.go` is where both halves already meet |
| AC-1 asserts the blob's mode, not a per-key mode | Give `KeyEntry` a mode and make zefs enforce it | zefs documents `WriteFile`'s perm as accepted and ignored, and `storeFileInfo.Mode()` returns a constant `0o444`. The 0600 is real and lives on the blob file `atomicWrite` creates through `os.CreateTemp`. Teaching zefs per-key modes changes the storage contract for every consumer inside a CA spec (owner decision, 2026-09-03) |
| Do not take `github.com/smallstep/certificates` | Take it, as Caddy does | It is a CA server with a database, ACME provisioners and SSH certificates. Caddy carries that because HTTPS is its job. Ze needs issuance over `crypto/x509` and a store it already has |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- The root key is protected by file permissions and nothing else. No passphrase, no hardware key store, no encryption at rest beyond what the filesystem gives. This is the posture every TLS key in Ze already has, and naming it here is deliberate so that a later decision to change it starts from a stated position.
- No mutual TLS. `handleConn` still authenticates by shared secret and the listener sets no `ClientAuth`. A CA is what makes client certificates possible, and issuing them is a separate spec.
- No revocation. A leaf that must be distrusted before it expires cannot be, short of rotating the root. Leaf lifetimes are short enough that this is a bounded exposure rather than an open one, and a CRL or OCSP responder is a separate spec.
- The operator-facing listeners are untouched. Web, the looking glass and the DoT and DoH fallback keep minting a self-signed leaf when no certificate is named. The looking glass has the named-certificate path the web listener already has: spec-lg-pki-certificate closed 2026-09-04, and its design is `docs/architecture/pki/tls-listeners.md`.
- SSH host certificates are not issued, though `hostKeyWithCertOption` reads one that nothing writes. That is the next consumer a later spec should take, because the code to consume it already exists.
- Distribution of the root is manual, exactly as fingerprint distribution is today. The export verb prints it; the operator carries it.
- First boot on a managed client has no configurable private anchor. `ze.managed.tls.certificate-fingerprint` is deleted and nothing replaces it, and a client fetching its FIRST config has no `pki ca` entry yet because that entry arrives in the config it is fetching. So `fetchInitialConfig` validates against the system pool, or the operator sets `ze.managed.tls.insecure` for that one exchange. This is a real capability the fingerprint provided and the root does not, and it is named rather than hidden: closing it means handing a client a root before any config exists, which is a bootstrap surface this spec does not design.
- `ze doctor` reports an unloadable stored root as `doctor-tls-invalid` rather than a code of its own, because `dnsserver` and `as112` already report that code for the same finding. Only absence and expiry get new codes.

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

## Implementation Summary

### What Was Implemented
- `internal/component/pki/ca.go`: the authority. `RootStore` (three methods, satisfied by `storage.Storage` with no adapter), `Root`, `LoadOrGenerateRoot` / `LoadOrGenerateRootFor`, `IssueLeaf` / `IssueLeafFor`, `Certificate`, `CertificatePEM`, `loadedRoot`. Root 10 years, leaf 24 hours, 5-minute skew backdate, 128-bit random serial, no key accessor.
- `internal/component/plugin/leaf.go`: `ServingLeaf` is `tls.Config.GetCertificate` and reissues once two thirds of the held leaf's life is spent, with the deadline READ from the issued certificate.
- `internal/component/plugin/ipc/tls.go`: `TLSConfigWithRoot` replaces `TLSConfigWithFingerprint`, `CertFingerprint` and `GenerateSelfSignedCert`, all three deleted. `StartListeners` takes a `GetCertificate` func and refuses nil. `PluginAcceptor.RootPEM` replaces `CertFP`.
- `pkg/plugin/sdk/sdk.go` and `internal/component/plugin/cli/cli.go`: both dial paths validate the chain against `ze.plugin.ca.pem` and refuse an absent anchor. The CLI path set `InsecureSkipVerify` until this change.
- `internal/component/managed/tls.go`: `clientTLSConfig` builds the pool from the configured `pki ca` name and returns an error, so first boot and steady state take one rule.
- `internal/component/plugin/server/managed_serve.go`: a named certificate is served unchanged at every handshake; with no name the injected `Authority` issues through `ServingLeaf`.
- `internal/appliance/ca.go`: `applianceRootStore` over two files beside `cert.pem`, key through the appliance passphrase, and `issueWebLeaf` writing leaf then root.
- `internal/component/pki/doctor.go` + `register.go`: the `pki-ca-root` check, three codes, 90-day window.
- `internal/component/pki/show.go`: `show pki local-ca pem`, plus the `local-ca` command tree in `internal/plugins/pki-cmd/yang/`.
- Deleted: `certificate-fingerprint`, `ze.managed.tls.certificate-fingerprint`, `ZE_PLUGIN_CERT_FP`, `ManagedServer.CertificateFingerprint`.

### Bugs Found/Fixed
- The exported root could not be pasted into the `pki ca` block it exists for: `show pki local-ca pem` printed PEM and `parseCACert` took base64 DER only. `certificateDER` / `privateKeyDER` (`internal/component/pki/config.go`) now take either form. `TestExportedRootPastesIntoAPKICABlock`.
- A leaf expired with no restart. Both callers issued once at construction and neither listener set `GetCertificate`, so a hub up for more than 24 hours served an expired certificate. `ServingLeaf` fixes it; `TestHubAcceptorReissuesBeforeTheLeafExpires` drives a real handshake 25 hours on.
- `internal/component/plugin/cli/cli.go` `connFromEnv` set `InsecureSkipVerify` and wrote the plugin's own token to whatever answered. Found by the pin-is-gone grep, not by an AC.
- A certificate leaf holding two PEM blocks silently took the first, so an operator pasting the appliance `cert.pem` (leaf then root) would have stored the LEAF as their trust anchor. Found at the Review Gate; refused now, `TestPKILeafNamesBothFormsWhenItRefuses/leaf_and_root_pasted_together`.
- `internal/appliance/cmd_rekey.go` re-encrypted a hardcoded list that would have left `ca-key.pem` under the old passphrase.

### Documentation Updates
- `docs/architecture/pki/pki-store.md` (`<!-- source: internal/component/pki/ca.go -- ... loadedRoot -->`, plus `plugin/leaf.go` and `pki/config.go` anchors), `docs/architecture/fleet-config.md` (`<!-- source: cmd/ze/ze_core_start.go -- fetchInitialConfig, extractManagedClientConfig -->`), `docs/architecture/api/process-protocol.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/system-architecture.md`, `docs/architecture/config/syntax.md`, `docs/architecture/zefs-format.md`, `docs/architecture/appliance/builder.md`, `docs/architecture/appliance/ota-push.md`, `docs/architecture/web-interface.md`, `docs/architecture/cli/plugin-modes.md`.
- `docs/features.md` (four anchors added at the Review Gate: `pki/ca.go`, `plugin/leaf.go`, `pki/show.go`, `appliance/ca.go`), `docs/config-reference.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/guide/plugins.md`, `docs/guide/appliance.md`, `docs/guide/status.md`, `docs/guide/web-interface.md`, `docs/plugin-development.md`, `docs/plugin-development/README.md`.
- `./le doc check verify`: red, and no finding names this spec's YANG or anchors after the Review Gate fixes. The summary-rule count fell 14 -> 11 and the three `ze-pki-conf` rows are gone; the 11 that remain are `bgp/policy/reject-asn`, another session's. Two `docs/features.md:44` anchor findings are the pre-existing introspection row and name `command.go`/`node.go`, neither touched here. One finding IS this spec's and is not repairable from this branch: the published `../gh-pages/reference/command-equivalents/show-pki-local-ca-pem/` surface does not exist, which is the site publish step, and 22 other commands (`announce`, `withdraw`, `show-plugins`, every `show-pki-certificate-*` export form) sit in the same state before this change.

### Deviations from Plan
- The spec named `pki.CAPool` as the client's integration point. `clientTLSConfig` uses `pki.GetCA(name)` and `pool.AddCert` instead: the client names ONE entry, and `CAPool` returns every configured CA, which is a wider anchor than the operator asked for.
- `cmd/ze/hub/main.go` rather than `cmd/ze/ze_core_start.go` is where the store and the plugin manager meet, so the root is loaded there. Recorded in Files to Modify during implementation.
- `IssueLeafFor` and `LoadOrGenerateRootFor` were added for the appliance, which mints once on the build host instead of reissuing at every start.
- `internal/core/selfcert/selfcert.go` gained `WebCertHosts` so the self-signed and the issued web leaf carry the same SAN set from one declaration.

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 assumed `plugin/ipc` may import `pki` | `pki/show.go` -> `plugin/server` -> `pluginipc` already closes the edge, so the import is a cycle | The implementation phase's first compile | Issuance is INJECTED, as `ManagedServerConfig.TLSMaterialResolver` already was |
| assumption | A-6 assumed the managed hub certificate had become stable | `managedCertificate` already returned a fresh 24-hour self-signed leaf on every start | Reading `managed_serve.go` at design time | Issuance from a persisted root is what first makes a restart survivable; recorded in the A-6 cell |
| approach | The AC set covered expiry THEN restart, which is a developer's case | A router runs for months, so the leaf expires with NO restart and the operator's config never changed | The documentation phase, writing the reissue sentence honestly and failing to find code that reissues | `ServingLeaf`, and `TestHubAcceptorReissuesBeforeTheLeafExpires` |
| approach | AC-8 said the export is "directly usable as a client trust anchor" | It printed PEM and the parser took base64 DER only, so the paste was refused | The same documentation phase, writing the paste instruction | Both forms accepted, and `TestExportedRootPastesIntoAPKICABlock` walks export -> config text -> pool |
| escalation | Two foreign commits carried this spec's uncommitted files, and one left `main` unable to compile for eleven hours | `557f401028` committed `process.go` calling `RootPEM` while `ipc/tls.go` stayed in the working tree | Building a clean worktree at HEAD, to establish whether nine functional failures predated this spec | Row in `plan/journal/concurrent-session-corruption.md`; the guard is `renderBlock` and its window is already recorded there |
| approach | Driving `managed/hub-ca-trust` by hand read as 1-in-3 flaky | A hand-run daemon gets the 5s product stage barrier; the runner derives 10s or more from the test budget | Three direct runs against three `.ci` runs | Row in `plan/journal/gate-verdict-depends-on-the-machine.md`; the test itself is unchanged and passes in 97s |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A root generated once and kept | Done | `pki.LoadOrGenerateRootFor` (`internal/component/pki/ca.go`) | Reads before it writes; `TestRootIsGeneratedOnceAndReused` |
| Leaves issued from it | Done | `(*pki.Root).IssueLeafFor` (`ca.go`) | `TestIssueLeafDrawsAUniqueSerial`, `TestIssueLeafBackdatesNotBefore` |
| The root distributed so a far end trusts an ISSUER | Done | `handleShowPKILocalCAPEM` (`internal/component/pki/show.go`), `PluginAcceptor.RootPEM` (`ipc/tls.go`), `issueWebLeaf` (`internal/appliance/ca.go`) | Three distribution routes: the command, the child environment, the device file |
| An empty anchor refuses rather than accepting | Done | `ipc.TLSConfigWithRoot`, `managed.clientTLSConfig`, `plugin.NewHubAcceptor` | `TestPluginDialRefusesWithNoAnchor`, `TestManagedClientRefusesWithNoAnchor`, `test/plugin/plugin-dial-no-anchor.ci` |
| The pin DELETED rather than kept alongside | Done | `internal/component/plugin/ipc/tls.go`, `internal/component/config/retired.go` | Three symbols and two config surfaces gone; 8 grep survivors, each read |
| Issuance not imported across a cycle | Done | `plugin.Authority` (`acceptor.go`), `ManagedServerConfig.Authority` | Injected at `cmd/ze/hub` |
| `github.com/smallstep/certificates` NOT taken | Done | `go.mod` unchanged | Issuance is `crypto/x509` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRootIsGeneratedOnceAndReused`, `TestRootKeyIsWrittenPrivate` | The store is closed and reopened; the blob file is 0600 and `KeyCAKey` is `Private` |
| AC-2 | Done | `TestHubAcceptorServesAnIssuedLeaf`, `TestPluginDialValidatesTheChain`, `TestTLSConfigWithRootValidatesTheChain` | A real `tls.Dial` whose only anchor is the stored root |
| AC-3 | Done | `TestPluginDialRefusesAnUnknownIssuer`, `TestManagedClientRefusesAnotherIssuer`, `TestPushRefusesALeafFromAnotherAuthority` | Each refuses a well-formed leaf from another root |
| AC-4 | Done | `TestPluginDialRefusesWithNoAnchor`, `TestManagedClientRefusesWithNoAnchor`, `TestHubAcceptorRefusesWithoutAnIssuer`, `test/plugin/plugin-dial-no-anchor.ci` | The `.ci` asserts a non-nil `*sdk.Plugin` never appears |
| AC-5 | Done | `TestFingerprintConfigIsRefused`, `test/parse/pki-ca-fingerprint-retired.ci` | The `.ci` asserts both halves: the retired leaf is refused BY NAME and `ca` parses |
| AC-6 | Done | `TestNamedCertificateOutranksIssuance` (4 subtests) | Including `a-named-certificate-is-never-reissued` after 400 days |
| AC-7 | Done | `TestManagedClientSurvivesAHubRestart`, `TestHubAcceptorReissuesBeforeTheLeafExpires` | The second covers expiry with NO restart, which the AC did not ask for |
| AC-8 | Done | `TestExportRootPrintsTheCertificateOnly`, `TestExportedRootPastesIntoAPKICABlock`, `test/plugin/pki-ca-root-export.ci` | The paste walks export -> config text -> `CAPool` -> chain verify |
| AC-9 | Done | `TestPushTrustsAReissuedLeaf` | `loadDeviceTLS` was not edited; the device serves a leaf the file never held |
| AC-10 | Done | `Root` exposes no key accessor; `test/plugin/pki-ca-root-export.ci` marshals the WHOLE response and refuses `PRIVATE KEY` | The grep for the key's exit from pki returns only the two signing call sites |
| AC-11 | Done | `TestCARootDoctorCheck`, `TestCARootDoctorCheckRegistered` | `doctor-pki-ca-root-missing` and `doctor-pki-ca-root-expiry`; an unloadable root reports `doctor-tls-invalid` (Known Limitations) |
| AC-12 | Done | `test/managed/managed-hub-ca-trust.ci`, PASS in 97.4s | Two real `ze start` daemons, plus a `foreign` control that must NOT fetch |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 12 `internal/component/pki` cases | PASS | `ca_test.go`, `doctor_test.go`, `show_test.go`, `config_pem_test.go` | Verbose run at closure, `ok internal/component/pki 1.252s` |
| The 6 acceptor and serving-leaf cases | PASS | `acceptor_test.go`, `leaf_test.go` | `ok internal/component/plugin 0.951s` |
| The 6 `TLSConfigWithRoot` and listener cases | PASS | `ipc/tls_root_test.go`, `ipc/tls_test.go` | `ok internal/component/plugin/ipc 1.228s`; two more than the plan named |
| `TestNamedCertificateOutranksIssuance` | PASS | `server/managed_cert_test.go` | 4 subtests where the plan named 1 |
| The 4 managed client cases | PASS | `managed/client_tls_test.go` | `ok internal/component/managed 4.151s` |
| `TestFingerprintConfigIsRefused` | PASS | `config/hub_certificate_test.go` | `ok internal/component/config 1.819s` |
| The 6 appliance cases | PASS | `appliance/ca_test.go`, `cmd_push_test.go` | `ok internal/appliance 11.074s` |
| The 3 SDK dial cases | PASS | `pkg/plugin/sdk/sdk_test.go` | `ok pkg/plugin/sdk 3.485s` |
| `TestPrivateKeysMarked` | PASS | `pkg/zefs/registry_test.go` | `ok pkg/zefs 3.202s` |
| `pki-ca-root-export` | PASS | `test/plugin/` | 7.1s |
| `plugin-dial-no-anchor` | PASS | `test/plugin/` | 7.5s |
| `pki-ca-fingerprint-retired` | PASS | `test/parse/` | 5.3s |
| `managed-hub-ca-trust` | PASS | `test/managed/` | 97.4s, and the runner marks it slow against a 28.4s suite average |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify and Files to Create | Done | Each exists and each carries the change the row named |
| `plan/deferrals/local-ca.md` | Changed | Never created: no deferral was taken. Creating an empty shard would be a file with nothing in it for a later closure to remove |
| `cmd/ze/hub/main.go` | Done, and committed by another session | It was entangled with the concurrent looking-glass certificate work for most of this closure. That session committed first (`87c9de85d9`), carrying this spec's `textbuf` import, `zepki.LoadOrGenerateRoot` and `pm.SetHubAuthority` lines with its own. The wiring is at HEAD and `git status` on the file is clean |
| `internal/component/config/loader_extract.go` | Done, and committed by another session | Same commit, same reason: it holds this spec's `extractHubClientConfig` `ca` hunk beside that session's `LGListenConfig.Certificate` |
| `internal/component/pki/store.go` | Included, and it is NOT this spec's change | Its 17 lines are `Snapshot`, orphaned by the looking-glass closure: `cmd/ze/hub/main_reload.go` at HEAD calls `zepki.Snapshot()` and the function stayed in the working tree, so `main` does not compile without it. Carried here because that spec is closed and nobody is coming back for it |

### Audit Summary
- **Total items:** 12 acceptance criteria, 7 task requirements, 13 test rows, 4 file rows
- **Done:** 12 / 12 acceptance criteria, 7 / 7 task requirements, 13 / 13 test rows, 2 / 4 file rows
- **Partial:** none
- **Skipped:** none
- **Changed:** 4 file rows, none of them a reduction. The deferral shard was never taken; `cmd/ze/hub/main.go` and `internal/component/config/loader_extract.go` reached HEAD inside another session's commit; and `internal/component/pki/store.go` is that session's orphaned `Snapshot`, carried here because HEAD calls it and does not compile without it

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An empty trust anchor can no longer be accepted as a pass | functional | `test/plugin/plugin-dial-no-anchor.ci` PASS 7.5s. The compiled driver asserts a non-nil `*sdk.Plugin` never comes back with `ZE_PLUGIN_CA_PEM` removed, and that the SAME process registers when the root is restored. A log line could not stand for this: the registration IS a Go value |
| A far end trusts an ISSUER rather than a copy of one certificate | functional, two daemons | `test/managed/managed-hub-ca-trust.ci` PASS 97.4s. The hub is restarted between the export and the fetch, so the leaf the client validates was issued AFTER the operator copied the root. The `foreign` half is the control: a well-formed root the hub never used, everything else equal, and the client must not fetch |
| The exported root is usable by the operator it exists for | functional + unit | `test/plugin/pki-ca-root-export.ci` PASS 7.1s (the whole response is marshalled and refused if it names a private key), and `TestExportedRootPastesIntoAPKICABlock`, which puts the export's exact bytes into config TEXT, parses it with the real schema, and verifies a leaf the root issued against the resulting pool |
| A component that outlives its leaf keeps serving a valid certificate | unit, real handshake | `TestHubAcceptorReissuesBeforeTheLeafExpires`. A client dials with `tls.Config.Time` set to a fake clock 25 hours on; with the renewal check removed the handshake fails `x509: certificate has expired`, which was observed |
| The appliance stays pushable after its certificate is reissued | unit, real handshake | `TestPushTrustsAReissuedLeaf`: the device serves a leaf the trust file never held and the push succeeds. `TestPushRefusesALeafFromAnotherAuthority` proves the pool is an anchor and not a pass |
| The root private key reaches no operator-visible surface | negative | `Root` declares no key accessor; `grep -rn 'InsecureSkipVerify' internal/ pkg/` over this spec's packages returns two comments about the retirement and one justified `ze.managed.tls.insecure` opt-in; the export `.ci` marshals the whole payload rather than the `pem` field alone |
| Interop | N-A | Scope is `plugin` and no wire format changes. The nearest assertion is a real `crypto/tls` client completing a handshake and validating a chain with `crypto/x509` against a root it did not mint, which the SDK, managed-client and appliance-push tests each do |

## Deferrals Resolved

<!-- Closure must leave no dangling row: deferral_unassigned_problems in
     internal/le/commit/prepare.go WARNS (it does not block) on a live row with no
     destination -- act on the warning here, because nothing else will.
     The spec's own shard is git rm'd at closure ONLY when every row in it is
     terminal; a shard still holding a live row outlives its source spec and
     deferral_shard_removal_problems blocks its removal
     (ai/rules/planning.md). Account for every row here.
     If resolving a row empties a FOREIGN shard (its last live row becomes
     terminal), that shard is now residue and this closure removes it too. -->
| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| None. `plan/deferrals/local-ca.md` was never created | cancelled | `ls plan/deferrals/ \| grep local-ca` returns nothing. The metadata table reserved the shard and no phase took a deferral, so there is no file to remove and no foreign shard emptied by this closure |
| R-7, the cross-process half of root generation | done, elsewhere | `plan/journal/store-serializes-in-process-only.md`, by owner decision 2026-09-03. `BlobStore` takes no file lock, which `TestBlobStoreNoFlock` asserts deliberately, so a second daemon on one blob already replaces arbitrary state |
| The six `./le doc wiring` drift rows on `cmd/ze/hub/main.go runYANGConfig` | done, elsewhere | `plan/journal/gate-fires-outside-its-population.md`. No page the rows name is made wrong by the 13 added lines; the check asks only whether the symbol moved |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     internal/le/spec/session/review.go record --spec <spec> --rounds <N> ... then check.
     --rounds is the pass count and is required; more than five needs
     --rounds-reason naming the PRODUCT defect a later round found, AND
     --owner-authorised carrying Thomas's word, because more than five passes
     is his decision (owner ruling 2026-08-17). At the cap you stop and ask him;
     you never set that flag on your own initiative. A false statement in this
     record is a NOTE, never a reason for another round (ai/rules/planning.md).
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/local-ca-0af7cb7e-0de7-4426-a8dd-9fdbf3640778.md` (78 files, verdict=clean) |
| `./le spec session review check` | `review_gate: OK (56 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 found six findings over the whole uncommitted diff; round 2 re-ran `./le repository check` and `./le doc check verify` against the fixes and found nothing new of this spec's |
| Reviewer lenses used | wiring and reachability (every new symbol traced to a non-test caller); guard and security (every fail-closed branch driven from its entry point, `InsecureSkipVerify` enumerated, key-material exit paths traced); Go style pass over every changed Go file (`docs/contributing/ze-go-style.md`, step 18); documentation drift (source anchors and YANG summary rules); removed-behaviour audit over `./le commit audit` |

### Findings fixed
<!-- Only BLOCKER and ISSUE. NOTEs do not block: record them and proceed.
     Every fix is new code that needs a fresh pass, so re-run until clean. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `managedCertificate`'s doc names `hub.listenerTLSMaterial`, a symbol declared only in a concurrent session's uncommitted `cmd/ze/hub/service_tls.go`. A committed comment would have pointed at nothing | `internal/component/plugin/server/managed_serve.go` | Reworded to `pki.ServerTLSMaterial`, the resolver actually injected here, which is the fail-closed function the sentence is about |
| 2 | ISSUE | `CurrentRoot` is exported and its only caller is `handleShowPKILocalCAPEM` in the same package. `./le repository check` reports it: an export with no cross-package caller | `internal/component/pki/ca.go` | Unexported to `loadedRoot`, with the reason in its doc comment. `show.go`, the `pki-store.md` source anchor and its prose all followed. Re-run: the finding is gone |
| 3 | ISSUE | A certificate leaf holding TWO PEM blocks silently took the first, so an operator pasting the appliance's own `cert.pem` (leaf then root) would have stored the LEAF as their trust anchor and met the failure at a later handshake | `certificateDER` and `privateKeyDER`, `internal/component/pki/config.go` | Refused, naming the second block's type. `TestPKILeafNamesBothFormsWhenItRefuses/leaf_and_root_pasted_together`, whose RED was observed by removing the check and restoring the file byte-identical |
| 4 | ISSUE | Three YANG declarations broke the summary rules the doc gate enforces: `pki/ca/certificate` and `pki/certificate/certificate` carried a summary and no `ze:help`, and `pki/certificate/intermediate` ran to 113 characters against a 96 cap. The PEM rewording introduced all three | `internal/component/pki/yang/ze-pki-conf.yang` | `ze:help` written for both leaves and the intermediate summary shortened. The gate's broken-rule count fell 14 -> 11 and no `ze-pki-conf` row remains |
| 5 | ISSUE | `docs/architecture/fleet-config.md` anchored `cmd/ze/ze_core_start.go -- managedClientConfig`, a symbol that does not exist. The function is `extractManagedClientConfig` | `docs/architecture/fleet-config.md:318` | Anchor corrected |
| 6 | ISSUE | The `docs/features.md` PKI row makes claims about `ca.go`, `leaf.go`, `show.go` and `appliance/ca.go` (lifetimes, the renewal fraction, the export, the appliance root) and carried no anchor for any of them | `docs/features.md` | Four source anchors added |
| NOTE | NOTE | `ServingLeaf.Certificate` returns an error when a renewal issuance fails, even though the held leaf is still valid for a third of its life. Fail-closed, and the only realistic cause is an RNG failure, so it is recorded rather than changed | `internal/component/plugin/leaf.go` | not changed |
| NOTE | NOTE | The published `../gh-pages/reference/command-equivalents/show-pki-local-ca-pem/` surface does not exist for the new command. 22 other commands are in the same state before this change, and the site publish step on the `gh-pages` branch is what writes them | `../gh-pages/` | not changed; out of this branch's reach |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in internal/le/commit/prepare.go checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/pki/ca.go` | yes | `ls -1` 2026-09-04 09:00, 17K |
| `internal/component/pki/ca_test.go` | yes | `ls -1` 2026-09-03 23:36, 11K |
| `internal/component/pki/doctor.go` | yes | `ls -1` 2026-09-03 22:03, 6.6K |
| `internal/component/pki/doctor_test.go` | yes | `ls -1` 2026-09-03 21:54, 5.7K |
| `internal/component/pki/show_test.go` | yes | `ls -1` 2026-09-03 21:59, 4.7K |
| `internal/component/pki/config_pem_test.go` | yes | `ls -1` 2026-09-04 09:11, 8.7K |
| `internal/appliance/ca.go` | yes | `ls -1` 2026-09-03 23:07, 6.6K |
| `internal/appliance/ca_test.go` | yes | `ls -1` 2026-09-03 23:04, 7.5K |
| `internal/component/plugin/acceptor_test.go` | yes | `ls -1` 2026-09-03 23:38, 5.6K |
| `internal/component/plugin/leaf.go` | yes | `ls -1` 2026-09-04 00:27, 5.6K |
| `internal/component/plugin/leaf_test.go` | yes | `ls -1` 2026-09-04 00:27, 9.0K |
| `internal/component/plugin/ipc/tls_root_test.go` | yes | `ls -1` 2026-09-03 22:05, 7.1K |
| `test/plugin/pki-ca-root-export.ci` | yes | `ls -1` 2026-09-03 22:06, 1.7K |
| `test/plugin/plugin-dial-no-anchor.ci` | yes | `ls -1` 2026-09-04 00:56, 2.2K |
| `test/parse/pki-ca-fingerprint-retired.ci` | yes | `ls -1` 2026-09-04 00:53, 1.7K |
| `test/managed/managed-hub-ca-trust.ci` | yes | `ls -1` 2026-09-04 01:15, 1.6K |
| `plan/deferrals/local-ca.md` | no, deliberately | `ls plan/deferrals/ \| grep local-ca` returns nothing; no deferral was taken |

### AC Verified (grep/test)
<!-- Every AC-N, re-checked. Acceptable: test name + pass output, grep showing
     the call, ls showing the file. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Same root after a restart; key entry `Private`; blob 0600 | `--- PASS: TestRootIsGeneratedOnceAndReused (0.07s)`, `--- PASS: TestRootKeyIsWrittenPrivate (0.03s)`; `grep 'meta/ca/' pkg/zefs/keys.go` shows `KeyCAKey ... Private: true` |
| AC-2 | The chain validates and no fingerprint is compared | `--- PASS: TestTLSConfigWithRootValidatesTheChain`, `--- PASS: TestHubAcceptorServesAnIssuedLeaf (0.06s)`, `--- PASS: TestPluginDialValidatesTheChain (0.06s)` |
| AC-3 | A foreign certificate is refused | `--- PASS: TestTLSConfigWithRootRefusesAnotherIssuer`, `--- PASS: TestPluginDialRefusesAnUnknownIssuer (0.06s)`, `--- PASS: TestManagedClientRefusesAnotherIssuer (0.16s)` |
| AC-4 | No anchor refuses | `--- PASS: TestPluginDialRefusesWithNoAnchor (0.53s)`, `--- PASS: TestManagedClientRefusesWithNoAnchor (1.07s)`, `--- PASS: TestHubAcceptorRefusesWithoutAnIssuer (0.00s)`, and `plugin-dial-no-anchor` PASS 7.5s |
| AC-5 | The retired leaf errors and names its replacement | `--- PASS: TestFingerprintConfigIsRefused (0.04s)`; `pki-ca-fingerprint-retired` PASS 5.3s asserting `certificate-fingerprint is retired` and `write ca <pki-ca-name>` |
| AC-6 | A named certificate is served and issuance is not reached | `--- PASS: TestNamedCertificateOutranksIssuance (0.01s)`, four subtests including `broken-name-errors-rather-than-issuing` |
| AC-7 | A fresh leaf from the same root, with no operator action | `--- PASS: TestManagedClientSurvivesAHubRestart (0.06s)`, `--- PASS: TestHubAcceptorReissuesBeforeTheLeafExpires (0.00s)` |
| AC-8 | PEM out, no key, directly usable | `--- PASS: TestExportRootPrintsTheCertificateOnly (0.03s)`, `--- PASS: TestExportedRootPastesIntoAPKICABlock (0.04s)`, `pki-ca-root-export` PASS 7.1s |
| AC-9 | A reissued leaf still pushes | `--- PASS: TestPushTrustsAReissuedLeaf (0.49s)`, `--- PASS: TestPushRefusesALeafFromAnotherAuthority (1.06s)` |
| AC-10 | The root key appears in no log, response or dump | `grep -rn 'InsecureSkipVerify' internal/component/plugin/ internal/component/managed/ internal/component/pki/ internal/appliance/ pkg/plugin/` over non-test files returns 3 hits: two comments about the retirement and the `ze.managed.tls.insecure` opt-in. `Root` declares `Certificate` and `CertificatePEM` and no key accessor |
| AC-11 | Missing, unreadable and near-expiry each report, with distinct codes | `--- PASS: TestCARootDoctorCheck (0.29s)`, `--- PASS: TestCARootDoctorCheckRegistered (0.00s)` |
| AC-12 | Two real daemons over a validated chain | `managed-hub-ca-trust` PASS 97.4s, `pass 1/1 100.0%` |

### Wiring Verified (end-to-end)
<!-- Every Wiring Test row: does the .ci exist AND exercise the claimed path?
     Read the file; do not infer it from its name. -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A daemon starting with no root | (unit) `TestRootIsGeneratedOnceAndReused` | yes: a real `storage.NewBlob` in `t.TempDir()`, closed and reopened |
| The hub acceptor starting | (unit) `TestHubAcceptorServesAnIssuedLeaf` | yes: a real `tls.Dialer.DialContext` whose only anchor is the stored root |
| An external plugin process dialing its hub | `test/plugin/plugin-dial-no-anchor.ci` | yes, read: the assertion lives in `internal/test/fixture/plugin_fixture_dial_no_anchor.go` because a non-nil `*sdk.Plugin` IS the registration, and it drives the same process twice, with and without the anchor |
| A managed client with a configured `pki ca` name | `test/managed/managed-hub-ca-trust.ci` | yes, read: two `ze start` daemons, the hub restarted between export and fetch, and a `foreign` control that must not fetch |
| An operator exporting the root | `test/plugin/pki-ca-root-export.ci` | yes, read: `fixture10PKICARootExport` marshals the WHOLE response, refuses `PRIVATE KEY`, parses the PEM, refuses a second block, and checks `IsCA` |
| `ze appliance push` against a reissued leaf | (unit) `TestPushTrustsAReissuedLeaf` | yes: `loadDeviceTLS` unedited, the device serves a leaf the file never held |
| `ze doctor` near root expiry | (unit) `TestCARootDoctorCheckRegistered` | yes: `diagnostic.Lookup` answers for both codes, which a listing would not prove |
| The whole fleet path | `test/managed/managed-hub-ca-trust.ci` | yes, PASS 97.4s |

### Assumptions Resolved
<!-- Every A-N. `unvalidated` is not a valid final status. A broken assumption
     needs a Mistake Log row and a Deviations entry. -->
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | A 10-year P-256 root is 369 bytes of DER and 554 of PEM; `startExternal` appends to `os.Environ()` and sets no size limit |
| A-2 | confirmed | `internal/appliance/cmd_push.go` `loadDeviceTLS` was not edited except for one constant rename, and `TestPushTrustsAReissuedLeaf` passes |
| A-3 | confirmed | `managed-hub-ca-trust` PASS 97.4s: the export command's bytes are what the client's `pki ca` block holds |
| A-4 | BROKEN | `pki/show.go` -> `plugin/server` -> `pluginipc` closes the edge. Issuance is injected as `plugin.Authority`. Mistake Log row |
| A-5 | confirmed | The pin grep returns 8 hits, all read: 5 are the unrelated `ze-show:pki-certificate-fingerprint` tests, 2 are SDK test comments about the retirement, 1 is a `.ci` comment |
| A-6 | BROKEN as stated | `managedCertificate` already returned a fresh self-signed leaf at every start, so the hub certificate was ALREADY ephemeral. Mistake Log row |

### Documentation Verified
<!-- Every Yes in the Documentation checklist: verify the edited claim against
     source. Every No: paste the grep that proves no update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 1, new user-facing feature (`docs/features.md`) | The PKI row's lifetimes, renewal fraction and consumers each read from `ca.go` and `leaf.go`; four source anchors added at the Review Gate | yes |
| Row 2, config syntax (`docs/guide/configuration.md`, `docs/config-reference.md`) | The `ca central-hub-root;` example parses: `pki-ca-fingerprint-retired.ci` runs the same shape through `ze config validate` and exits 0 | yes |
| Row 3, CLI (`docs/guide/command-reference.md`) | `show pki local-ca pem` is registered in `pki/show.go`, in `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang`, and in `plugin/all/testdata/wire-methods.snapshot` | yes |
| Row 5 / row 8, plugin SDK and process protocol | `docs/architecture/api/process-protocol.md` names `ZE_PLUGIN_CA_PEM`, which is what `process.go` sets; its anchor names `TLSConfigWithRoot`, `PluginAcceptor.RootPEM`, `StartListeners` and `leaf.go` | yes |
| Row 6, fleet config | `docs/architecture/fleet-config.md` first-boot paragraph checked against `fetchInitialConfig`; the anchor named a symbol that does not exist and was corrected to `extractManagedClientConfig` at the Review Gate | yes |
| Row 12, internal architecture | `docs/architecture/pki/pki-store.md` and `docs/architecture/plugin/plugin-system.md`; the `CurrentRoot` anchor and prose followed the rename to `loadedRoot` | yes |
| Rows 4, 7, 9, 10, 11, 14 answered No | `grep -rn 'rfc-status\|comparison' docs/features/rfc-status.md docs/comparison.md` for this spec's surfaces returns nothing; no wire format, no RFC row, no metric, no test infrastructure changed | yes |
| Row 13 N-A | No route metadata is involved | yes |
| `./le doc check verify` | Red tree-wide. No remaining finding names this spec's YANG, its anchors or its pages. The one that IS this spec's is the unpublished `../gh-pages/.../show-pki-local-ca-pem/` surface, which the site publish step writes and which 22 other commands already lack | yes, with that exception stated |

## Core Insight
<!-- Optional: the single most important design revelation from this work.
     Not every spec has one. Delete the section if nothing qualifies.
     Feeds the Decisions section of the learned summary. -->

Two of this spec's four real defects were found by WRITING THE PAGE, not by an
acceptance criterion. The AC set said "a leaf expires and the component
restarts" and "the export is directly usable"; both read as covered until an
agent had to write, for an operator, the sentence that says what happens next.
There is no code that reissues on expiry without a restart, and the parser
refused the bytes the export command prints. Neither gap sits on a changed line,
so no review of the diff would have found either.

The lesson generalises past this spec. An acceptance criterion is written by the
person who knows how the feature works, so it inherits their model, including
the case they did not think of. Documentation is written for someone who does
not have that model, and the sentence either has a producer behind it or it does
not. That makes the documentation phase a CORRECTNESS check and not a reporting
one, which is the argument for `ai/rules/documentation.md` putting the page edit
in the same work as the code rather than at closure.

Second, smaller: the root's replacement of the pin was easy to see and the
RENEWAL was not. A pin is compared at every handshake, so it cannot go stale
silently; an issued leaf is minted once and then merely presented. Replacing a
mechanism with a better one moves where the state lives, and the new place needs
its own liveness answer. `ServingLeaf` is that answer, and `tls.Config`'s own
`GetCertificate` seam is where it belongs, because the stdlib already asks the
question once per connection.
