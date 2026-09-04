# Spec: lg-pki-certificate

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1/7 |
| Deferral shard | `plan/deferrals/lg-pki-certificate.md` |
| Handoff | - |
| Updated | 2026-09-03 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The looking glass is the only TLS listener in Ze an operator cannot point at a
certificate. When TLS is on, `buildLGService` (`cmd/ze/hub/service_lg.go`) calls
`selfcert.LoadOrGenerateCert` unconditionally, so the server always presents a
leaf it signed for itself. The web and API listener reads a name from
`environment.web.certificate` and serves the PKI store chain through
`webTLSMaterial` (`cmd/ze/hub/service_web.go`), and the DoT and DoH listeners of
geodns and as112 do the same through an injected `pki.ServerTLSMaterial`
resolver.

The symptom reaches the public. The looking glass binds `0.0.0.0`, is read-only,
and is open unless a token is set, because it is meant for people outside the
operator's organization. It is the surface most likely to be visited by a
stranger, and the only one that cannot present a chain that stranger's browser
accepts. An operator today either terminates TLS in a proxy and turns Ze's own
TLS off, or publishes a URL that trains every visitor to click through a
certificate warning.

The goal is to give the looking glass the same certificate reference the web
listener has, with the same fail-closed rule and the same rotation behavior: a
configured name that does not resolve is an error and no material, never a
silent fall back to the self-signed path, and a rotated certificate reaches a
running listener without a rebind.

This closes the deferral recorded in the Known Limitations of
`spec-pki-full-chain`, which sized it as "a small follow-up consuming
`pki.ServerTLSMaterial`" and left `cmd/ze/hub/service_lg.go` out of scope in its
Current Behavior list. The research phase found that sizing wrong: the
looking-glass server cannot rotate a certificate at all, so the follow-up owes
server code, not only wiring.

Out of scope: a local certificate authority that would let Ze issue a chain of
its own rather than an issuer-less leaf. That is a separate spec. It does not
help this surface, because a stranger visiting a public looking glass will never
have installed Ze's root.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/pki/tls-listeners.md` - the contract every PKI-serving listener follows
  → Decision: `ServerTLSMaterial` returns the leaf CERTIFICATE block first, then one block per intermediate, because `tls.X509KeyPair` requires the sender's own certificate at index 0. The looking glass inherits that ordering by calling the same producer, and MUST NOT re-assemble the chain itself.
  → Constraint: a configured name that does not resolve returns an error and no material. There is no fallback to self-signed. An EMPTY name is the different case, and it takes the established self-signed path unchanged.
  → Constraint: the page opens by saying two consumers use the material, the web and API HTTPS listener and the DoT and DoH listeners. This spec makes it three, so the page changes in the same work as the code.
  → Decision: rotation does not rebind the listener. The new chain is installed on the running server and the next handshake serves it.
- [ ] `docs/architecture/pki/pki-store.md` - what the store holds and what it does not
  → Constraint: the store is load-validate-serve. `pki.Load` takes a parsed `PKIConfig` and nothing in it depends on blob storage, so a named certificate is reachable on a deployment that has no blob store.
- [ ] `docs/guide/looking-glass.md` - the operator's view of the surface
  → Constraint: the page's TLS paragraph states the looking glass uses the same certificate infrastructure as the web UI. That is false today and is a page defect repaired in this work: the web listener resolves a PKI name, the looking glass never does.
  → Decision: the looking-glass listener defaults to TLS on and port 8443, and is open unless `token` is set. The certificate leaf must not change either default.
- [ ] `ai/patterns/config-option.md` - the structural template for a new YANG leaf
  → Constraint: every YANG leaf under `environment/` MUST have a matching `env.MustRegister()` entry, and the env key mirrors the YANG path with the final segment exact: `ze.looking-glass.certificate`.
  → Constraint: the leaf takes maximum native validation. `length "1..255"` and `pattern '[A-Za-z0-9._-]+'` are what the web leaf carries, and the same constraints apply to the same kind of value.
  → Decision: this is YANG config rather than an env-only knob, because an operator changes it during normal operation, it must appear in `show configuration` and in a config backup, and it takes part in commit and rollback.
- [ ] `ai/rules/config.md` - naming and listener conventions
  → Constraint: YANG leaf names are kebab-case with no abbreviations, and a single leaf is singular. `certificate` matches the web leaf exactly, which is what makes the two surfaces learnable together.

### RFC Summaries (Scope: protocol)

N-A. Scope is `config`. The feature changes which certificate a TLS listener
presents; it changes no wire format and no protocol behavior. TLS itself is
served by `crypto/tls`, whose conformance is not altered by this change.

**Key insights:** (minimal context to resume after compaction)
- The looking-glass server builds one `tls.Config` at construction and hands it to `tls.NewListener`. There is no `GetCertificate` callback, so the served certificate cannot change without rebinding. The web server sets `GetCertificate`, clears `Certificates` so the two can never disagree, and swaps the pair under an atomic pointer.
- `webTLSMaterial` is eight lines and holds the whole precedence rule: a set name resolves from the PKI store or fails, an empty name takes the self-signed path. Copying it would declare that rule twice.
- `buildLGService` refuses TLS when the store is not blob storage. That precondition exists to persist a self-signed certificate. It does not apply to a named PKI certificate, and left where it is it would refuse a valid deployment for the wrong reason.
- The lg package has no root `register.go` and no `doctor.go`. It is compiled only under the `ze_lg` build tag, and `cmd/ze/hub/register_lg.go` is the existing seam that keeps always-on hub code free of lg types.
- `spec-pki-full-chain` built the web half and closed on 2026-09-03. All eleven of its acceptance criteria have live implementations, and its design is now `docs/architecture/pki/tls-listeners.md`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `cmd/ze/hub/service_lg.go` - builds the looking-glass service. When TLS is on it requires blob storage, then calls `selfcert.LoadOrGenerateCert` and puts the PEM into `LGConfig.CertPEM` / `KeyPEM`. No certificate name is read anywhere on this path.
- [ ] `cmd/ze/hub/service_web.go` - `webTLSMaterial` resolves a configured name through `zepki.ServerTLSMaterial` and falls back to `selfcert.LoadOrGenerateCert` only when the name is empty. `startWebServer` calls it with the operator's `environment.web.certificate` value.
- [ ] `internal/component/lg/server.go` - `NewLGServer` builds `tls.Config` once with a fixed `Certificates` slice and a TLS 1.2 minimum, stores it, and uses it at both `tls.NewListener` sites. No rotation path exists.
- [ ] `internal/component/web/server.go` - `getCertificate` reads an `atomic.Pointer[tls.Certificate]` per handshake and refuses rather than serving nothing when it is nil. `UpdateTLSCertificate` parses the new pair, refuses unparseable material so the previous certificate keeps serving, and stores it.
- [ ] `internal/component/pki/tls.go` - `ServerTLSMaterial` returns leaf-then-intermediates plus the PKCS#8 key, errors on an unknown name listing the available ones, and errors on an entry with no private key. `CheckCertReference` is the doctor-side producer of `doctor-tls-reference` and `doctor-tls-expired`, including the 30-day warning.
- [ ] `internal/component/lg/yang/ze-lg-conf.yang` - declares `environment / looking-glass` with `enabled`, a `server` list carrying `ip` and `port`, `tls` defaulting true, and a `ze:sensitive` `token`. No certificate leaf.
- [ ] `internal/component/web/yang/ze-web-conf.yang` - declares the `certificate` leaf with `length "1..255"`, `pattern '[A-Za-z0-9._-]+'`, and help text stating the fail-closed rule and naming the env override.
- [ ] `cmd/ze/hub/register_lg.go` - registers the lg service factory under `ze_lg` and hands the service to `lm.setLG`. It does not make the `tlsUpdatable` assertion that `register_web.go` makes.
- [ ] `cmd/ze/hub/listener_migrate.go` - holds `tlsUpdatable`, `setWebTLS` and `updateWebCertificate`. The lg has an address-migration path only.
- [ ] `cmd/ze/hub/main_reload.go` - `reloadWebCertificate` re-resolves the web name against the just-installed store and rejects the commit through `restorePKIAfter` before any consumer applies. There is no lg reference anywhere in the file.
- [ ] `internal/component/web/doctor.go` - `checkWebTLSCertificate` reads the parsed tree offline and delegates to `pki.CheckCertReference`. Registered from `internal/component/web/register.go` through `diagnostic.RegisterDoctorCheck`.
- [ ] `internal/component/config/loader_extract.go` - `LGListenConfig` and `extractLGBlock` carry the lg leaves; `WebListenConfig` and `extractWebBlock` carry the web ones including `Certificate`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- TLS on by default for the looking glass, port 8443, `0.0.0.0` bind.
- The empty-name path: with no certificate configured, the self-signed certificate is loaded or generated from blob storage exactly as today, and persisted so browsers do not re-accept on every restart.
- The blob-storage warning path: an operator who only inherited the TLS default, on a deployment without blob storage, gets plaintext plus a warning naming the remedy. An operator who asked for TLS explicitly gets an error. Both stay, for the empty-name case only.
- The bearer-token gate, and the looking glass being open when no token is set.
- Every existing looking-glass listener address behavior, including multi-listener and the web/lg port-conflict rejection.

**Behavior to change:** (only what the user asked for)
- A `certificate` name may be configured for the looking glass, and when set it is served from the PKI store with its full chain.
- A configured name that does not resolve refuses the listener at startup and refuses the commit at reload, rather than falling back.
- The looking-glass server can have its certificate replaced without rebinding its listeners.
- The blob-storage precondition applies to the empty-name case only.
- `webTLSMaterial` becomes a listener-agnostic selector serving both the web and the looking glass, so the precedence rule is declared once.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config: `environment { looking-glass { certificate <name> } }` in the config file, or `set environment looking-glass certificate <name>` in the editor.
- Environment override: `ze.looking-glass.certificate`, which wins over the file value on both the startup and the reload path, matching the web leaf.
- Format at entry: a string of 1 to 255 characters matching `[A-Za-z0-9._-]+`, naming an entry in the `pki` config block.

### Transformation Path
1. YANG validation in `internal/component/config` applies the leaf's length and pattern constraints.
2. `extractLGBlock` (`internal/component/config/loader_extract.go`) reads the leaf into `LGListenConfig.Certificate`, and `ExtractLGSettings` exposes it independently of `enabled`, so a flag-started listener still receives the operator certificate.
3. `cmd/ze/hub/main.go` resolves precedence: the env value first, the config value as fallback, and refuses to start when a non-empty name does not resolve against the loaded PKI store.
4. The value reaches `serviceDeps.LGCertificate` and then `buildLGService`.
5. `buildLGService` calls the shared listener material selector, which resolves the name through `pki.ServerTLSMaterial` or takes the self-signed path for an empty name.
6. `NewLGServer` installs the parsed pair in an atomic pointer and serves it from a `GetCertificate` callback.
7. On reload, `reloadLGCertificate` re-resolves against the just-installed store, refuses the commit on failure, and otherwise hands the new material to `updateLGCertificate`, which calls `UpdateTLSCertificate` on the running server.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ hub | `ExtractLGSettings` returns `LGListenConfig`; the hub reads `Certificate` | No |
| Hub ↔ pki component | `pki.ServerTLSMaterial` returns PEM material or an error | No |
| Hub ↔ lg component | `serviceDeps.LGCertificate` into `buildLGService`; `tlsUpdatable` back out for rotation | No |
| Always-on hub ↔ `ze_lg`-gated code | `cmd/ze/hub/register_lg.go` `init()`, which does not exist without the tag | No |
| Doctor registry ↔ lg component | `diagnostic.RegisterDoctorCheck` from a new lg root `register.go` | No |

### Integration Points
- `pki.ServerTLSMaterial` - the chain producer, called rather than re-implemented.
- `pki.CheckCertReference` - the doctor producer, called by the new lg check exactly as the web check calls it.
- `listenerMigrator` - already carries the per-service seams; this adds the lg TLS handle beside the web one.
- `registerService` in `cmd/ze/hub/register_lg.go` - the existing hook where the `tlsUpdatable` assertion belongs.

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
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The PKI store is reachable without blob storage, so a named certificate can be served on a deployment that has no blob store | `pki.Load` takes a parsed `PKIConfig`; nothing in `internal/component/pki/store.go` references blob storage | The blob-storage guard must stay where it is, and AC-7 is void | Functional test: a no-blob-storage deployment with a named certificate serves that chain | confirmed |
| A-2 | The looking glass wants its own leaf rather than a share of `environment.web.certificate` | The lg has its own container, its own default port 8443, and plausibly its own hostname; user decision at the design gate | One leaf serves both and this spec's YANG half disappears | User confirmation, recorded at the design gate | confirmed |
| A-3 | `ExtractLGSettings` returns on block presence rather than on `enabled`, so a flag-started looking glass still receives the operator certificate | `internal/component/config/lg_extract_test.go` asserts settings survive a disabled block | The certificate is dropped for exactly the deployments most likely to set it | Existing test re-read at implementation, plus a new case naming `Certificate` | confirmed |
| A-4 | Adding `GetCertificate` and clearing `Certificates` on the lg `tls.Config` changes no negotiated parameter other than which certificate is served | `internal/component/web/server.go` does exactly this and its handshake tests pass | The lg handshake changes in some way the tests do not cover | The existing `test/plugin/lg-tls-default-on.ci` handshake assertion must still pass unchanged | confirmed |
| A-5 | The `ze_lg` build tag keeps the new doctor check out of a build without the looking glass | `cmd/ze/hub/register_lg.go` uses this pattern for the service factory | A build without lg either fails to compile or registers a check for a component it does not have | Build both tag configurations; assert the check is absent in one and present in the other | confirmed |
| A-6 | The lg package may import `internal/component/config` and `internal/component/pki`, which it does not today, without failing the tier rule | `internal/component/web/doctor.go` imports both for exactly this purpose | The doctor check needs a different home, most likely an injected resolver like the one `internal/core/dnsserver` takes | `./le tier check` after the doctor phase | confirmed: with `internal/component/lg/doctor.go` importing both, `./le tier check` exits 0 ("engine placement clean", "non-engine placement categories clean; 28 manifest row(s)", "core import direction clean"). No injected resolver is needed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rotation path installs the web certificate onto the looking glass, or the reverse, because the two handles live on the same migrator | A rotation test that passes while asserting the wrong service's chain | Each migrator method takes its own handle and its own name; a unit test rotates one service and asserts the other is untouched |
| R-2 | `GetCertificate` is set on the lg config while `Certificates` is left populated, so `crypto/tls` and the atomic pointer can disagree about what is served | A rotation that appears to succeed while handshakes keep the old chain | Clear `Certificates` at the same statement that sets `GetCertificate`, as the web server does, and assert the rotated chain over a real handshake |
| R-3 | The blob-storage precondition is left ahead of the name branch, so a store-less deployment naming a valid certificate is refused | A functional test on a no-blob-storage deployment fails with a storage error rather than serving | Move the guard inside the empty-name branch; AC-7 and AC-8 pin both sides |
| R-4 | The lg doctor registration escapes the `ze_lg` tag and a build without the looking glass registers a check for it | Compilation failure, or `ze doctor` naming a component that is not in the binary | The registration lives in the lg package, which the tag already excludes; A-5 validates both builds |
| R-5 | The reload checks the certificate reference against the store that is being replaced rather than the one just installed, so a valid new reference is rejected or an invalid one is accepted | A reload test that passes for the wrong reason: the old store happened to hold the same name | Hook `reloadLGCertificate` at the same point `reloadWebCertificate` runs, after the store install and before any consumer applies, and restore the prior store on failure |
| R-6 | A handshake in flight during rotation reads a partially written certificate | Data race under `-race` in the rotation test | `atomic.Pointer[tls.Certificate]` holds a whole pointer; the web server proves the shape. Run the rotation test under `-race` |
| R-7 | An operator sets both a token and a certificate and one path clears the other | The token gate stops working when a certificate is configured | The two are independent fields; a functional test sets both and asserts the gate and the chain together |
| R-8 | Strengthening the web rotation test to hold a connection open surfaces a real defect in the web listener rather than only in the test | The strengthened web test goes red on unchanged web code | That red is a product defect and is fixed here, not weakened away. It is the reason the strengthening is in scope |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The looking-glass listener fails to start, or serves the wrong certificate, on deployments that configure one. A mistake in the shared selector reaches the web and API listener too: that listener carries the config editor and the operator's own session. WIDENED AT CLOSURE, and this is now the largest surface: the work also changed `preparePKIConfig` (`cmd/ze/hub/main_pki.go`), which every daemon runs at startup and at every reload, whatever listeners it carries. A mistake there refuses a `pki {}` block that used to load, or loads one it should refuse, on every deployment |
| How is it reverted? | Single commit revert. No config migration: the leaf is new and absent config behaves exactly as today. No peer or wire state is involved. The `preparePKIConfig` half reverts with it, and reverting restores the defect it fixed: no config-file `pki certificate <name> intermediate` reached the store, so a deployment naming a CA-issued chain exited at the `pki config` stage |
| Who else touches this path? | `spec-pki-full-chain` built the web half and closed on 2026-09-03; its design is `docs/architecture/pki/tls-listeners.md`. Any session working `cmd/ze/hub/service_web.go`, `listener_migrate.go` or `main_reload.go` shares these files |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `environment.looking-glass.certificate` in a config file | → | `extractLGBlock` into `LGListenConfig.Certificate` | `TestExtractLGBlockReadsCertificate` |
| `serviceDeps.LGCertificate` at hub startup | → | `buildLGService` calling the shared selector | `TestBuildLGServiceResolvesNamedCertificate` |
| A TLS handshake against the looking-glass listener | → | `lg.LGServer.getCertificate` | `TestLGServerServesPKIChain` |
| `ze.looking-glass.certificate` environment variable | → | the precedence resolution in `cmd/ze/hub/main.go` | `TestLGCertificateEnvWins` |
| A config reload that changes the certificate name | → | `reloadLGCertificate` then `updateLGCertificate` | `TestReloadRotatesLGCertificate` |
| `ze doctor` over a config naming an undefined lg certificate | → | the lg doctor check calling `pki.CheckCertReference` | `TestLGTLSDoctorCheckRegistered` |
| An operator starting the looking glass over TLS end to end | → | the whole path from config file to served chain | `test/plugin/lg-pki-certificate.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `certificate` names a store entry holding a leaf and one intermediate | A TLS client handshaking with the looking glass receives both certificates, leaf first |
| AC-2 | `certificate` is unset and blob storage is present | The looking glass serves the self-signed certificate from blob storage, and serves the same certificate again after a restart rather than generating a new one |
| AC-3 | `certificate` names an entry the PKI config does not define | Ze exits non-zero at startup naming the missing certificate and the available names; no looking-glass listener binds |
| AC-4 | `certificate` names an entry that holds no private key | Same refusal as AC-3, with an error naming the missing key |
| AC-5 | A reload changes `certificate` to a name the new store does not define | The commit is rejected, the prior PKI store is restored, and the running looking glass keeps serving its previous chain |
| AC-6 | A reload changes `certificate` to a different valid name while a client holds an open connection | The open connection continues to carry data, the listener address is unchanged, and the next handshake receives the new chain |
| AC-7 | `certificate` is set on a deployment with no blob storage | The looking glass starts with TLS and serves the named chain |
| AC-8 | `certificate` is unset on a deployment with no blob storage | Unchanged: an explicit `tls true` is an error, an inherited default warns and serves plaintext |
| AC-9 | `ze doctor` runs over a config whose lg certificate name is undefined, and separately over one whose certificate expired | `doctor-tls-reference` for the first, `doctor-tls-expired` for the second, and a warning within 30 days of expiry |
| AC-10 | `ze.looking-glass.certificate` is set and the config file names a different certificate | The environment value is served |
| AC-11 | Both `token` and `certificate` are set | The bearer gate refuses an unauthenticated request over the named chain; neither disables the other |
| AC-12 | A build without the `ze_lg` tag | Compiles, runs, and registers no looking-glass doctor check |
| AC-13 | A reload rotates the WEB certificate while a client holds an open connection | The open connection continues to carry data and the next handshake receives the new chain, closing the gap between what the existing web rotation test claims and what it asserts |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Loads a CA-issued certificate into the PKI store and names it on the looking glass | config file -> `extractLGBlock` -> `serviceDeps` -> `buildLGService` -> selector -> `ServerTLSMaterial` -> `NewLGServer` -> handshake | `test/plugin/lg-pki-certificate.ci` |
| 2 | Renews the certificate and reloads the config without dropping a viewer's session | reload -> `reloadLGCertificate` -> `updateLGCertificate` -> `UpdateTLSCertificate` -> next handshake | `test/reload/lg-pki-reference-reload.ci`, `TestLGServerUpdateTLSCertificate` |
| 3 | Mistypes the certificate name and reloads | reload -> `reloadLGCertificate` -> commit refused -> prior store restored | `test/reload/lg-pki-reference-reload-broken.ci` |
| 4 | Runs `ze doctor` before committing a config that names a certificate the store lacks | parsed tree -> lg doctor check -> `pki.CheckCertReference` -> `doctor-tls-reference` | `TestLGTLSDoctorCheck` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractLGBlockReadsCertificate` | `internal/component/config/lg_extract_test.go` | The leaf reaches `LGListenConfig.Certificate`, and survives a disabled block (A-3) | |
| `TestCertificateLeafLengthFromYANGLG` | `internal/component/config/leaf_length_test.go` | The 1..255 length and the pattern come from YANG, not from Go | |
| `TestLGServerServesPKIChain` | `internal/component/lg/server_tls_test.go` | A real `crypto/tls` client receives leaf then intermediate (AC-1) | |
| `TestLGServerUpdateTLSCertificate` | `internal/component/lg/server_tls_test.go` | An open connection survives the swap, the address is unchanged, and the next handshake gets the new chain (AC-6) | |
| `TestLGUpdateTLSCertificateRejectsBadMaterial` | `internal/component/lg/server_tls_test.go` | Unparseable material is refused and the previous certificate keeps serving | |
| `TestBuildLGServiceResolvesNamedCertificate` | `cmd/ze/hub/service_lg_tls_test.go` | A set name resolves from the store; an unresolvable one is an error and no service (AC-3, AC-4) | |
| `TestBuildLGServiceNamedCertificateWithoutBlobStorage` | `cmd/ze/hub/service_lg_tls_test.go` | The blob-storage guard does not apply to a named certificate (AC-7, R-3) | |
| `TestBuildLGServiceEmptyNameKeepsStorageRules` | `cmd/ze/hub/service_lg_tls_test.go` | Explicit TLS without blob storage still errors; an inherited default still warns and serves plaintext (AC-8) | |
| `TestLGCertificateEnvWins` | `cmd/ze/hub/main_reload_pki_test.go` | The env value beats the config file on both paths (AC-10) | |
| `TestReloadRejectsBrokenLGCertificateReference` | `cmd/ze/hub/main_reload_pki_test.go` | The commit is refused and the prior store restored (AC-5, R-5) | |
| `TestReloadRotatesLGCertificate` | `cmd/ze/hub/main_reload_pki_test.go` | The running listener receives the new material (AC-6) | |
| `TestListenerMigratorUpdateLGCertificate` | `cmd/ze/hub/service_lg_tls_test.go` | Rotating the looking glass leaves the web certificate untouched, and the reverse (R-1) | |
| `TestLGTLSDoctorCheck` | `internal/component/lg/doctor_test.go` | Both diagnostic codes and the 30-day warning window (AC-9) | |
| `TestLGTLSDoctorCheckRegistered` | `internal/component/lg/doctor_test.go` | The check is present in the registry with its component and codes | |
| `TestWebServerUpdateTLSCertificate` | `internal/component/web/server_tls_test.go` | Extended: an open connection survives the swap, which the test claims today and does not assert (AC-13, R-8) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `certificate` name length | 1-255 characters | 255 | empty string, which is the valid "unset" case rather than an error | 256 |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-pki-certificate` | `test/parse/lg-pki-certificate.ci` | The certificate leaf parses and validates in an lg block | |
| `lg-pki-certificate-name-too-long` | `test/parse/lg-pki-certificate-name-too-long.ci` | A 256-character name is refused by the YANG length constraint | |
| `lg-pki-certificate-served` | `test/plugin/lg-pki-certificate.ci` | An operator names a store certificate and a client receives the full chain, with the token gate still refusing an unauthenticated request (AC-1, AC-11) | |
| `lg-pki-reference-reload` | `test/reload/lg-pki-reference-reload.ci` | A reload rotates the certificate and the listener keeps serving (AC-6) | |
| `lg-pki-reference-reload-broken` | `test/reload/lg-pki-reference-reload-broken.ci` | A reload naming an undefined certificate is refused and the prior store restored (AC-5) | |

### Interop Tests (Scope: protocol)

N-A. Scope is `config` and no wire format changes. The nearest thing to an
interop assertion is a real `crypto/tls` client completing a handshake and
reading the served chain, which `TestLGServerServesPKIChain` and
`test/plugin/lg-pki-certificate.ci` both do.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/lg/yang/ze-lg-conf.yang` - the `certificate` leaf, with the web leaf's constraints and help text adapted to this listener
- `internal/component/config/environment.go` - `env.MustRegister` for `ze.looking-glass.certificate`
- `internal/component/config/loader_extract.go` - `LGListenConfig.Certificate` and its read in `extractLGBlock`
- `cmd/ze/hub/main.go` - env-over-config precedence, the startup fail-closed gate, and passing the value into `serviceDeps`
- `cmd/ze/hub/service_registry.go` - the `LGCertificate` field
- `cmd/ze/hub/service_lg.go` - call the shared selector, and move the blob-storage precondition inside the empty-name branch
- `cmd/ze/hub/service_web.go` - generalize `webTLSMaterial` into the listener-agnostic selector both services call
- `cmd/ze/hub/register_lg.go` - the `tlsUpdatable` assertion beside `lm.setLG`
- `cmd/ze/hub/listener_migrate.go` - `setLGTLS` and `updateLGCertificate`
- `cmd/ze/hub/main_reload.go` - `reloadLGCertificate`, its refusal block, and the rotation call
- `internal/component/lg/server.go` - the atomic certificate pointer, the `GetCertificate` callback, `UpdateTLSCertificate`, and `ServesTLS`
- `cmd/ze/hub/main_pki.go` - take the config TREE rather than the plugin-facing map, and delete the lossy rebuild that was dropping every stored intermediate
- `cmd/ze/hub/service_registry.go` - `serviceDeps` reaches every factory by pointer, because the new field pushed it past the linter's value-parameter threshold
- `internal/component/web/server_tls_test.go` - hold a connection open across the rotation (AC-13)
- `docs/architecture/pki/tls-listeners.md` - three consumers, not two, and the looking-glass rows
- `docs/architecture/web-interface.md` - declared by the `// Design:` header of `internal/component/lg/server.go`, which this spec changes
- `docs/architecture/config/syntax.md` - declared by the config loader this spec changes; the looking-glass block gains a leaf, which is config syntax
- `docs/architecture/hub-architecture.md` - declared by the hub files this spec changes; the hub gains a service dependency, a startup gate and a reload rotation hook
- `docs/guide/looking-glass.md` - its TLS paragraph is wrong today and becomes true; document the leaf and the fail-closed rule
- `docs/guide/configuration.md` - the new leaf beside the web one
- `docs/config-reference.md` - the generated reference row for the leaf
- `docs/guide/environment-variables.md` - the new `ze.looking-glass.certificate` override
- `docs/features/looking-glass.md` - the feature page for the surface that gains the capability

## Files to Create
- `cmd/ze/hub/service_tls.go` - the one listener-agnostic TLS material selector, tagged `ze_lg || ze_web` because its two callers carry independent gates
- `internal/component/lg/register.go` - the lg package's root registration file, holding the doctor-check registration
- `internal/component/lg/doctor.go` - the looking-glass certificate reference check, delegating to `pki.CheckCertReference`
- `internal/component/lg/doctor_test.go` - both diagnostic codes, the warning window, and the registration assertion
- `internal/component/lg/server_tls_test.go` - chain serving, rotation with an open connection, and fail-closed rotation
- `cmd/ze/hub/service_lg_tls_test.go` - selector resolution, the storage-guard cases, and per-service rotation isolation
- `cmd/ze/hub/listener_migrate_lg_test.go` - per-service rotation isolation in both directions (R-1)
- `cmd/ze/hub/main_pki_test.go` - a chain of one, two and three intermediates reaching the store from config text, on the startup and the reload path
- `test/parse/lg-pki-certificate.ci` - the leaf parses
- `test/parse/lg-pki-certificate-name-too-long.ci` - the length constraint refuses
- `test/plugin/lg-pki-certificate.ci` - end-to-end chain plus token gate
- `test/reload/lg-pki-reference-reload.ci` - rotation over a reload
- `test/reload/lg-pki-reference-reload-broken.ci` - a broken reference refuses the commit
- `plan/deferrals/lg-pki-certificate.md` - the deferral shard named in the metadata table

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/lg/yang/ze-lg-conf.yang`, a `certificate` leaf in the existing `looking-glass` container |
| YANG validation constraints | Yes | `length "1..255"` and `pattern '[A-Za-z0-9._-]+'`, matching `internal/component/web/yang/ze-web-conf.yang` |
| YANG custom validators | No | The web leaf carries none. Existence of a named entry is a cross-root reference a per-leaf `ValidateFn` cannot see, so it is enforced at startup, at reload and by the doctor check instead. Live completion of store names is inherited as a deferral from the Known Limitations of `spec-pki-full-chain` |
| CLI commands/flags | N-A | No new verb. The leaf is reached through the config editor and the config file |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | No | Automatic completion needs an enum or a `CompleteFn`; the value is a runtime store name. Same position as the web leaf, and the same inherited deferral |
| Functional test for new RPC/API | N-A | No new RPC. Functional coverage is the `.ci` set in the Functional Tests table |
| Pipe completeness | N-A | No command output is added |
| Env var registration | Yes | `ze.looking-glass.certificate` in `internal/component/config/environment.go`, mandatory for an `environment/` leaf |
| Doctor check for runtime dependencies | Yes | A certificate is a runtime dependency. New `internal/component/lg/doctor.go` and `internal/component/lg/register.go`; codes `doctor-tls-reference` and `doctor-tls-expired` already exist in `internal/core/diagnostic/codes.go`, so no new code is declared. Unit test `TestLGTLSDoctorCheck`, functional coverage in `test/reload/lg-pki-reference-reload-broken.ci` |
| Prometheus counters/metrics | No | The feature changes which certificate is served, not any observable rate or state worth a counter. Certificate expiry is surfaced by the doctor check rather than a gauge, matching the web listener |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, beside the web certificate entry |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, and `docs/config-reference.md` for the generated row |
| 3 | CLI command added/changed? | No | No verb is added; the leaf is set through the config editor, whose grammar is unchanged |
| 4 | API/RPC added/changed? | No | No command or RPC surface changes |
| 5 | Plugin added/changed? | No | The looking glass is a component, not a plugin, and no plugin registration changes |
| 6 | Has a user guide page? | Yes | `docs/guide/looking-glass.md`, whose TLS paragraph is false today |
| 7 | Wire format changed? | No | No wire format is touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK or process-protocol surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement changes. TLS is served by `crypto/tls` and no `rfc/short/` row moves |
| 10 | Test infrastructure changed? | No | New tests use the existing `.ci`, `.et` and Go harnesses; no runner or fixture mechanism changes |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about which certificate the looking glass serves |
| 12 | Internal architecture changed? | Yes | `docs/architecture/pki/tls-listeners.md`, which names two consumers and gains a third, and gains the looking-glass rotation seam |
| 13 | Route metadata keys added/changed? | N-A | No route metadata is involved |
| 14 | Prometheus counters added/changed? | No | No counter is added, per the Integration Checklist row |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A new doctor check is a registered inventory item: `docs/guide/status.md` and the doctor check listing |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-lg-pki-certificate.md` is run at implementation and every named page is answered. `internal/component/lg/server.go` declares `docs/architecture/web-interface.md` in its `// Design:` header, so that page BLOCKS and is named in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/looking-glass.md` and `docs/guide/looking-glass-howto.md` carry looking-glass config examples; each is checked against the YANG after the leaf lands |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- the leaf exists and reaches the service builder
   - Tests: `TestExtractLGBlockReadsCertificate`, `TestCertificateLeafLengthFromYANGLG`, `TestBuildLGServiceResolvesNamedCertificate`
   - Files: `ze-lg-conf.yang`, `environment.go`, `loader_extract.go`, `service_registry.go`, `main.go`, `service_lg.go`
   - Verify: the config value reaches `buildLGService`. The wiring test fails first because the builder ignores it
2. **Phase: Shared selector** -- one precedence rule for both listeners
   - Tests: `TestBuildLGServiceNamedCertificateWithoutBlobStorage`, `TestBuildLGServiceEmptyNameKeepsStorageRules`, plus the existing web selector tests unchanged
   - Files: `service_web.go`, `service_lg.go`
   - Verify: the web tests still pass against the renamed function, and the blob-storage guard now applies only to the empty-name branch
3. **Phase: Server rotation** -- the looking glass can be handed new material
   - Tests: `TestLGServerServesPKIChain`, `TestLGServerUpdateTLSCertificate`, `TestLGUpdateTLSCertificateRejectsBadMaterial`
   - Files: `internal/component/lg/server.go`
   - Verify: an open connection survives a rotation and the next handshake serves the new chain, under `-race`
4. **Phase: Startup and reload gates** -- fail closed on both paths
   - Tests: `TestLGCertificateEnvWins`, `TestReloadRejectsBrokenLGCertificateReference`, `TestReloadRotatesLGCertificate`, `TestListenerMigratorUpdateLGCertificate`
   - Files: `main.go`, `main_reload.go`, `listener_migrate.go`, `register_lg.go`
   - Verify: a broken reference refuses at startup and at commit, a valid change rotates, and rotating one service leaves the other alone
5. **Phase: Doctor** -- the operator learns before committing
   - Tests: `TestLGTLSDoctorCheck`, `TestLGTLSDoctorCheckRegistered`
   - Files: `internal/component/lg/doctor.go`, `internal/component/lg/register.go`
   - Verify: both codes fire, the 30-day window matches the web check, and a build without `ze_lg` registers nothing
6. **Phase: Web rotation test strengthening** -- close the claim the web test does not assert
   - Tests: `TestWebServerUpdateTLSCertificate` extended
   - Files: `internal/component/web/server_tls_test.go`
   - Verify: an open connection survives the web rotation. A red here is a product defect in the web listener and is fixed rather than weakened (R-8)
7. **Phase: Functional tests and documentation** -- the operator's path and the pages that describe it
   - Tests: the five `.ci` files in the Functional Tests table
   - Files: the five `.ci` files, plus `tls-listeners.md`, `looking-glass.md`, `configuration.md`, `config-reference.md`, `features.md`, `status.md`
   - Verify: the end-to-end scenario serves the named chain, and no page still says the looking glass shares the web UI's certificate infrastructure

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The empty name and the unresolvable name are distinguishable at every branch: one takes the self-signed path, the other refuses. Neither ever produces a listener serving material the config did not name |
| Correctness | `Certificates` is cleared wherever `GetCertificate` is set, so `crypto/tls` and the atomic pointer cannot disagree |
| Naming | The YANG leaf, the Go field and the env key agree word for word: `certificate`, `Certificate`, `ze.looking-glass.certificate` |
| Data flow | The blob-storage precondition sits inside the empty-name branch only, and nothing on the named path consults the blob store |
| Rule: `ai/rules/principles.md` | The precedence rule is declared once. Grep for a second place deciding name-versus-self-signed |
| Rule: `ai/rules/plugins.md` | The lg doctor check lives in the lg package and registers itself. No lg spelling appears in a central package |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The YANG leaf exists with its constraints | `grep -A6 'leaf certificate' internal/component/lg/yang/ze-lg-conf.yang` |
| The env var is registered | `grep 'ze.looking-glass.certificate' internal/component/config/environment.go` |
| One selector serves both listeners | `grep -rn 'ServerTLSMaterial' cmd/ze/hub/` returns one MATERIAL-SELECTION call site, in `service_tls.go`. The startup gate, the reload refusal and the rotation path resolve a reference too and are not second declarations of the rule: an empty name there means "do not rotate", never "serve self-signed" |
| The looking glass can rotate | `grep -n 'UpdateTLSCertificate\|GetCertificate' internal/component/lg/server.go` |
| The doctor check is registered | `go test ./internal/component/lg/ -run TestLGTLSDoctorCheckRegistered` finds `lg-tls-certificate` in the post-config phase with both codes. There is no `ze doctor list` verb and no `le run` action, so the registry is read through the test rather than a CLI |
| Every functional test exists and passes | `./le test functional` over the five named `.ci` files |
| No page still claims shared certificate infrastructure | `grep -rn 'same certificate infrastructure' docs/` returns nothing |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The certificate name is operator config, not untrusted input, but it indexes a store. The YANG pattern restricts it to `[A-Za-z0-9._-]+`, so it cannot carry a path separator |
| Fail-open | The one failure that must never be silent: an unresolvable name serving a self-signed certificate while the config names a real one. Every branch that could reach `selfcert.LoadOrGenerateCert` is reachable only from an EMPTY name |
| Error leakage | The not-found error lists the available certificate names. That is existing `ServerTLSMaterial` behavior on an operator-facing path, and the looking glass must not surface it to an HTTP client: the failure happens before any listener binds |
| Key material | The served private key reaches `tls.X509KeyPair` and the atomic pointer only. It is never logged, and the rotation log line names no material |
| Authorization | The bearer-token gate and the certificate are independent. A configured certificate must not make the token gate optional, which AC-11 pins |

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
- A deferral written as "the same pattern, so extending it is a small follow-up" measured the code it could see and not the code it could not. The looking-glass server had no rotation path at all, which no reading of `service_lg.go` alone would show. The lesson is about where a deferral's sizing comes from: naming the CONSUMER as identical says nothing about whether the consumer can accept what the producer now offers.
- A value lost between two representations asks one question before any other: was the second representation needed at all? The stored chain was dropped because the hub lowered its config tree to the plugin-facing map and rebuilt a tree from it, and the two obvious repairs were both to the conversion. Every caller already held the tree, so the conversion was pure loss. The cheapest fix was the one that deleted code rather than adding a case to it.
- A functional test earns its place by reaching a surface no unit test does. Three phases of unit tests passed over this feature and none of them failed, because they all built the store entry through the slice setter. The first test that wrote the operator's own config text found that the operator's route had never worked.
- A test comment that claims more than the test body asserts is the same defect as a claim wider than its evidence, and it survives longer because a green bar hides it. The web rotation test says long-lived SSE sessions survive and asserts only that the listener address did not change.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Generalize `webTLSMaterial` into one selector both listeners call | Copy the shape into `service_lg.go`; build a listener-certificate registry every TLS service registers into | Copying declares one precedence rule twice, with nothing to arbitrate a future disagreement. The registry is the right end state at four consumers and buys an abstraction at two |
| The looking glass gets its own `certificate` leaf | Share `environment.web.certificate` | The looking glass has its own container, its own default port and plausibly its own hostname. One name for two listeners would force an operator to choose which surface gets the right certificate |
| The blob-storage precondition moves inside the empty-name branch | Leave the guard ahead of both branches | The guard exists to persist a self-signed certificate. `pki.Load` takes a parsed config and touches no blob store, so a named certificate on a store-less deployment would be refused for a reason that does not apply to it |
| Rotation is in scope rather than deferred | Fail-closed only, with a restart to pick up new material; a deferral shard row | Without it a reload validates the new name, reports success, and keeps serving the old chain until restart. That is the silently-wrong-value failure the fail-closed rule exists to prevent, wearing a green bar |
| The web rotation test is strengthened in this spec | Mirror its current shape for the looking glass; strengthen only the looking-glass test | Its comment already claims what it does not assert, and leaving two tests of one behavior at different strengths invites the weaker to be read as the standard |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- No live CLI completion of store certificate names for the new leaf. The Known Limitations of `spec-pki-full-chain` deferred it for the web leaf and this spec inherits that position rather than diverging from it. `pki.certificateNames` is the natural body of a future `Complete()`.
- No mutual TLS or client-certificate authentication for the looking glass. Server-side chain only, matching the web listener.
- No local certificate authority. Ze still mints an issuer-less self-signed leaf when no certificate is named, which no third party can trust. A separate spec covers a local CA, and it does not help this surface: a stranger visiting a public looking glass will never have installed Ze's root.
- MCP and REST TLS are untouched. They carry the same self-signed-only shape and inherit the same follow-up, named in the Known Limitations of `spec-pki-full-chain`.
- No `.ci` drives a DAEMON through the plaintext-downgrade sequence: a deployment with no blob store, TLS inherited, an operator who then adds a certificate name and reloads. What is proven in-process, through the real registration hook and the real `runReload`, is every discriminating step of it: `TestPlaintextLGHoldsNoRotationHandle` builds that deployment with `storage.NewFilesystem()`, asserts the rotation handle is withheld, asserts the reload is accepted, and asserts the four things the warning must say; `TestReloadPlaintextLGKeepsCertificateInert` asserts `lgCertificateName` reports no name for `tls false`. What no test reads is that warning arriving on a real daemon's stderr. Closing that needs a fixture that starts a daemon with no blob store and greps its output, and it would assert the same producer, `(*listenerMigrator).updateLGCertificate`, one process boundary further out.
- The WEB listener has no `ServesTLS` equivalent. `web.(*WebServer).UpdateTLSCertificate` does not ask whether the server serves TLS, so `--insecure-web` plus a named certificate would accept a rotation nothing serves. No AC covers it and no phase touched it. Row: `plan/journal/guard-added-to-one-half-of-a-pair.md`.

## RFC Documentation (Scope: protocol)

N-A. Scope is `config`. No RFC-governed behavior is implemented or changed.

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
- The `certificate` leaf in the `looking-glass` container (`internal/component/lg/yang/ze-lg-conf.yang`), `length "1..255"` and `pattern '[A-Za-z0-9._-]+'`, matching the web leaf.
- `ze.looking-glass.certificate` (`internal/component/config/environment.go`), and `LGListenConfig.Certificate` read in `extractLGBlock` on the SETTINGS path (`internal/component/config/loader_extract.go`), so a flag-started listener still receives the operator certificate.
- `listenerTLSMaterial` (`cmd/ze/hub/service_tls.go`, NEW, `//go:build ze_lg || ze_web`): ONE precedence rule for every hub TLS listener. `webTLSMaterial` and `lgTLSMaterial` are both DELETED, and `startWebServer` and `buildLGService` call the shared selector.
- The blob-storage precondition moved inside the empty-name branch of `buildLGService` (`cmd/ze/hub/service_lg.go`), so a named certificate serves on a deployment that never ran `ze init`.
- Rotation on the running server: `LGServer.cert atomic.Pointer[tls.Certificate]`, `getCertificate`, `UpdateTLSCertificate` and `ServesTLS` (`internal/component/lg/server.go`). `NewLGServer` clears `tls.Config.Certificates` at the statement that sets `GetCertificate`.
- Fail-closed on both paths: the startup gate in `runYANGConfig` (`cmd/ze/hub/main.go`) exits non-zero, and the reload refusal through `restorePKIAfter` (`cmd/ze/hub/main_reload.go`). Both read one name through `lgCertificateName`, so a restart and a reload cannot disagree.
- The rotation seam: `setLGTLS` and `updateLGCertificate` (`cmd/ze/hub/listener_migrate.go`), installed by `register_lg.go` only for a TLS-serving looking glass.
- The doctor check `lg-tls-certificate` (`internal/component/lg/doctor.go`, `internal/component/lg/register.go`), delegating to `pki.CheckCertReference`.
- `serviceDeps` reaches every factory by POINTER (`cmd/ze/hub/service_registry.go`): the new field took the struct to 288 bytes, the linter's value-parameter threshold. `buildWebService` no longer writes `deps.WebAddrs`, because one struct now reaches every later factory.

### Bugs Found/Fixed
- **`pki certificate <name> intermediate` never reached the store, on every deployment.** `(*Tree).toMap` lowers a one-member leaf-list to a bare string and a longer one to `[]string`; the hub rebuilt a tree from that map with a `case string` arm calling `Set` and a slice arm matching only `[]any`, so `GetSlice("intermediate")` answered nil for every count, `pki.Validate` built an empty intermediate pool, and the daemon exited at the `pki config` stage with `x509: certificate signed by unknown authority`. Fixed by DELETING the round trip: `preparePKIConfig` (`cmd/ze/hub/main_pki.go`) takes the `*zeconfig.Tree` its callers already hold, and `configTreeFromMap` and `mapValuesAreMaps` are gone. Covered by `TestPreparePKIConfigKeepsEveryIntermediate` and `TestReloadInstallsEveryIntermediate` (`cmd/ze/hub/main_pki_test.go`), both building the chain from config TEXT through `ParseTreeWithYANG`.
- **The same defect at a third site.** The reload's rollback rebuilt the prior PKI config from the PROVIDER snapshot, which holds the same plugin-facing maps, so a rejected reload restored a store with its intermediates stripped. `runReloadContext` now takes `zepki.Snapshot()`.
- **`test/plugin/lg-pki-certificate.ci` was RED at promotion on that defect** and is GREEN now. It is the first test anywhere that wrote an `intermediate` leaf in operator config text.
- **A plaintext looking glass refused the whole reload** over a certificate no listener reads. `ServesTLS` plus the gate in `register_lg.go` make the leaf inert instead; `updateLGCertificate` logs why. `TestPlaintextLGHoldsNoRotationHandle` asserts both the withheld handle and that log line.
- **`listenerMigrator.logger` was nil on the zero-value migrator** tests build directly, so a log line added to a path a test enters would panic. `(*listenerMigrator).log()` falls back to the subsystem logger.

### Documentation Updates
- `docs/architecture/pki/tls-listeners.md`: three consumers with a per-consumer leaf table; the one-rule statement and its disjunction build tag; the looking glass's three-row blob-storage table; the startup gate; the reload refusal and `restorePKIAfter`; the new `## Decision: a listener that serves no TLS makes the leaf inert`; the rotation section now names both hub listeners and both `UpdateTLSCertificate` and `getCertificate` pairs; the doctor section gained `internal/component/lg/doctor.go`.
- `docs/guide/looking-glass.md`: the `certificate` row, the env override, and a `### Serve your own certificate` section whose table carries Default, Fail closed, Rotation, TLS off, Plaintext by downgrade, No blob storage needed, Name, Own leaf and Env override, with five source anchors. Its closing sentence no longer claims the same certificate infrastructure as the web UI.
- `docs/features/looking-glass.md` (three new rows and four anchors), `docs/features/web-interface.md` (the anchor follows `webTLSMaterial` to `listenerTLSMaterial`; the fail-closed sentence says the daemon exits), `docs/guide/looking-glass-howto.md`, `docs/guide/config-reload.md` (the lg reload row and two anchors), `docs/guide/environment-variables.md` (both certificate env vars), `docs/architecture/hub-architecture.md` (the startup-refusal count was already wrong and is now uncounted).
- `docs/features.md` and `internal/component/lg/yang/ze-lg-conf.yang` carry this spec's edits ALREADY COMMITTED: a concurrent session staged those two whole files. Verified at `git show HEAD:docs/features.md` lines 42 and 72, and `git show HEAD:internal/component/lg/yang/ze-lg-conf.yang` line 68.
- FOUR pages carry this spec's edits UNCOMMITTED, because a concurrent session's unshipped work sits in the same files and git stages whole files: `docs/config-reference.md` (the four-listener TLS Listeners section and the lg leaf row), `docs/guide/configuration.md` (four listeners, the Plaintext row, the Blob storage row, the rotation and env-override rows, three anchors), `docs/architecture/web-interface.md` (the `### LG TLS` section), `docs/architecture/config/syntax.md` (the one-way lowering paragraph naming `preparePKIConfig`). Carrying them would commit that session's PEM-parsing, retired-keyword and hub-CA prose, which describes behavior no commit holds yet.

### Deviations from Plan
- `cmd/ze/hub/main_pki.go` and `cmd/ze/hub/main_pki_test.go` were not in the original Files lists. The chain defect was found by the first functional test that wrote operator config text, and it blocks the spec's own headline claim, so it was fixed here (`ai/rules/completion.md`).
- `plan/deferrals/lg-pki-certificate.md` was named in Files to Create and never created. No deferral was recorded by any phase, so the shard has no rows and nothing to remove.
- The Deliverables row naming `./le run -- ze doctor list` was corrected: neither the action nor the verb exists. The registry is read through `TestLGTLSDoctorCheckRegistered` and through a two-build `ze doctor --json` run.
- The Deliverables row demanding ONE `ServerTLSMaterial` call site was corrected to name the MATERIAL-SELECTION site: the startup gate, the reload refusal and the rotation each must resolve a reference too.
- `serviceDeps` by pointer was not planned. Phase 1's 16-byte field crossed `gocritic.hugeParam.sizeThreshold: 288`; `.golangci.yml` was not edited.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Three phases of unit tests passed over the chain-serving path and none of them failed | Every one built the store entry with `SetSlice`, so none entered the operator's config route, where the chain was dropped | The first `.ci` that wrote an `intermediate` leaf in config text | Both new tests in `cmd/ze/hub/main_pki_test.go` build the tree from config text through `ParseTreeWithYANG` |
| approach | A deferral in `spec-pki-full-chain` sized this work as "a small follow-up consuming `pki.ServerTLSMaterial`" | The looking-glass server could not rotate a certificate at all, so the follow-up owed server code | The research phase read `internal/component/lg/server.go` | Row in `plan/journal/deferral-sized-from-the-visible-half.md` |
| approach | Three `.ci` headers stated the chain defect as unfixed and one called its own test RED | The defect was fixed inside this spec and the test is green | The closure review read the files | All three headers corrected |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The looking glass serves a named PKI certificate with its full chain | Done | `buildLGService` calls `listenerTLSMaterial` (`cmd/ze/hub/service_tls.go`), which calls `zepki.ServerTLSMaterial` | `test/plugin/lg-pki-certificate.ci` reads both certificates off a real handshake |
| The same fail-closed rule as the web listener | Done | `runYANGConfig` (`cmd/ze/hub/main.go`), the lg refusal block in `runReloadContext` (`cmd/ze/hub/main_reload.go`) | Never falls back to self-signed for a configured name |
| The same rotation behavior: no rebind | Done | `updateLGCertificate` (`cmd/ze/hub/listener_migrate.go`) into `(*LGServer).UpdateTLSCertificate` | `test/reload/lg-pki-reference-reload.ci` holds a connection across the rotation |
| The precedence rule is declared once | Done | `listenerTLSMaterial`, `cmd/ze/hub/service_tls.go` | `webTLSMaterial` and `lgTLSMaterial` are both deleted |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLGServerServesPKIChain`, `TestBuildLGServiceResolvesNamedCertificate`, `test/plugin/lg-pki-certificate.ci` | The `.ci` client trusts only the operator root, so a completed handshake is itself the chain proof |
| AC-2 | Done | `TestBuildLGServiceEmptyNameKeepsStorageRules`, `TestBuildLGService_DefaultTLSWithoutBlobStorageServesPlaintext` | The self-signed path is unchanged; `selfcert.LoadOrGenerateCert` still persists into zefs |
| AC-3 | Done | `TestBuildLGServiceResolvesNamedCertificate/typo-cert`, the startup gate in `runYANGConfig` | No unit test drives `runYANGConfig`: `runHub` is not callable from a test. The gate shares `lgCertificateName` and `zepki.ServerTLSMaterial` with the reload path, which IS tested |
| AC-4 | Done | `TestBuildLGServiceResolvesNamedCertificate/lg-keyless` | The keyless-entry error comes from `ServerTLSMaterial` |
| AC-5 | Done | `TestReloadRejectsBrokenLGCertificateReference`, `test/reload/lg-pki-reference-reload-broken.ci` | Both stores define the same name with DIFFERENT leaves, so the restore cannot pass by the name surviving |
| AC-6 | Done | `TestLGServerUpdateTLSCertificate`, `TestReloadRotatesLGCertificate`, `test/reload/lg-pki-reference-reload.ci` | The `.ci` holds one connection across the rotation and counts `sighup reload complete` rather than searching for it |
| AC-7 | Done | `TestBuildLGServiceNamedCertificateWithoutBlobStorage`, `test/plugin/lg-pki-certificate.ci` | The `.ci` fixture runs no `ze init`, so the deployment has no blob storage at all |
| AC-8 | Done | `TestBuildLGServiceEmptyNameKeepsStorageRules`, `TestBuildLGService_ExplicitTLSWithoutBlobStorageFails` | One observable change: the error message no longer carries a doubled prefix |
| AC-9 | Done | `TestLGTLSDoctorCheck` | Both codes and the 30-day window |
| AC-10 | Done | `TestLGCertificateEnvWins` | Two subtests: the shared producer, and a real `runReload` refused over the env name |
| AC-11 | Done | `test/plugin/lg-pki-certificate.ci` | One deployment sets both; the gate answers 401 over the named chain |
| AC-12 | Done | `TestLGTLSDoctorCheckRegistered`, plus a two-build `ze doctor --json` run | The build without `ze_lg` names no looking-glass check |
| AC-13 | Done | `TestWebServerUpdateTLSCertificate` | The held connection answers before and after, and still names the pre-rotation leaf |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestExtractLGBlockReadsCertificate` | Done | `internal/component/config/lg_extract_test.go` | Three subtests, one of them A-3 |
| `TestCertificateLeafLengthFromYANGLG` | Done | `internal/component/config/leaf_length_test.go` | Length and pattern read from the built schema |
| `TestLGServerServesPKIChain` | Done | `internal/component/lg/server_tls_test.go` | |
| `TestLGServerUpdateTLSCertificate` | Done | `internal/component/lg/server_tls_test.go` | |
| `TestLGUpdateTLSCertificateRejectsBadMaterial` | Done | `internal/component/lg/server_tls_test.go` | `TestLGUpdateTLSCertificateWithoutTLS` was added beside it |
| `TestBuildLGServiceResolvesNamedCertificate` | Done | `cmd/ze/hub/service_lg_tls_test.go` | |
| `TestBuildLGServiceNamedCertificateWithoutBlobStorage` | Done | `cmd/ze/hub/service_lg_tls_test.go` | |
| `TestBuildLGServiceEmptyNameKeepsStorageRules` | Done | `cmd/ze/hub/service_lg_tls_test.go` | |
| `TestLGCertificateEnvWins` | Done | `cmd/ze/hub/main_reload_pki_test.go` | |
| `TestReloadRejectsBrokenLGCertificateReference` | Done | `cmd/ze/hub/main_reload_pki_test.go` | |
| `TestReloadRotatesLGCertificate` | Done | `cmd/ze/hub/main_reload_pki_test.go` | |
| `TestListenerMigratorUpdateLGCertificate` | Changed | `cmd/ze/hub/listener_migrate_lg_test.go` | The plan named `service_lg_tls_test.go`; it lives in its own file beside two siblings |
| `TestLGTLSDoctorCheck` | Done | `internal/component/lg/doctor_test.go` | |
| `TestLGTLSDoctorCheckRegistered` | Done | `internal/component/lg/doctor_test.go` | |
| `TestWebServerUpdateTLSCertificate` | Done | `internal/component/web/server_tls_test.go` | Extended to hold a connection open |
| `TestPreparePKIConfigKeepsEveryIntermediate` | Changed | `cmd/ze/hub/main_pki_test.go` | Not in the plan: the chain defect the work found |
| `TestReloadInstallsEveryIntermediate` | Changed | `cmd/ze/hub/main_pki_test.go` | Not in the plan, same reason |
| `TestPlaintextLGHoldsNoRotationHandle` | Changed | `cmd/ze/hub/service_lg_tls_test.go` | Not in the plan: the inert-leaf decision |
| `TestReloadPlaintextLGKeepsCertificateInert` | Changed | `cmd/ze/hub/main_reload_pki_test.go` | Not in the plan, same reason |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | `internal/component/lg/yang/ze-lg-conf.yang` and `docs/features.md` were carried into HEAD by a concurrent session's whole-file commit |
| Every file in Files to Create | Done | Except `plan/deferrals/lg-pki-certificate.md`, which no phase needed |

### Audit Summary
- **Total items:** 13 AC, 4 requirements, 19 tests
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5, four tests added beyond the plan and one file moved, each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A visitor's browser accepts the looking glass's certificate, so the public surface stops training people to click through a warning | functional | `test/plugin/lg-pki-certificate.ci`: the fixture's client trusts ONLY the operator's generated root and verifies against it, so a completed handshake proves the listener sent a buildable path. `./le functional plugin` reports it PASSING |
| The same fail-closed rule as the web listener: a configured name that does not resolve is an error, never a silent self-signed fallback | functional | `test/reload/lg-pki-reference-reload-broken.ci`: the refusal names `environment.looking-glass.certificate`, the listener keeps its previous chain, and the corrected reload then commits. Discrimination: deleting the refusal block moves the failure LATER and drops the leaf name from the message |
| The same rotation behavior: a rotated certificate reaches a running listener without a rebind | functional | `test/reload/lg-pki-reference-reload.ci`: one held connection carries data across the rotation, the address is unchanged, the next handshake gets the new leaf. Discrimination: dropping `UpdateTLSCertificate` from `updateLGCertificate` gives `the served leaf CN is "ze looking glass leaf one", want ... "leaf two"` |
| The precedence rule is declared once, so the two surfaces cannot disagree | grep | `grep -rn 'ServerTLSMaterial' cmd/ze/hub/` returns ONE material-selection call site, `cmd/ze/hub/service_tls.go`. The others are reference VALIDATION (`main.go`, `main_reload.go`), ROTATION (`listener_migrate.go`) and one injected resolver (`managed_server.go`) |
| A named certificate serves where the self-signed one cannot: a deployment with no blob storage | functional | `test/plugin/lg-pki-certificate.ci` runs no `ze init`, so no blob store exists, and the named chain serves |
| Every stored intermediate reaches the listener from a config FILE | unit, entered by the operator's own route | `TestPreparePKIConfigKeepsEveryIntermediate` (one, two and three intermediates) and `TestReloadInstallsEveryIntermediate`, both parsing config text with `ParseTreeWithYANG`. Discrimination walked 2026-09-04: routing `preparePKIConfig` back through a lowering round trip turns all four cases red with the production error |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard exists | done | `plan/deferrals/lg-pki-certificate.md` was named in the metadata table and in Files to Create, and no phase recorded a deferral into it. `ls` reports no such file, so there is no shard to remove and no live row to home |
| Live CLI completion of store certificate names, inherited from the Known Limitations of `spec-pki-full-chain` | deferred | Unchanged position, recorded in Known Limitations above. `pki.certificateNames` is the natural body of a future `Complete()` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lg-pki-certificate-8c387580-991c-4941-9e76-f341b6dd780e.md`, 52 files, verdict clean |
| `./le spec session review check` | `review_gate: OK (37 code files, clean, hashes match ...)` |
| Rounds | 1 |
| Reviewer lenses used | automated pre-checks (`./le repository check`, `./le commit audit`), wiring, functional-test coverage, documentation drift, removed-behavior audit, logic and guard audit, the Go style pass of `docs/contributing/ze-go-style.md`, simplicity and altitude, project-rule cross-check |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Three `.ci` headers stated the intermediate-lowering defect as live and one declared its own test RED. The defect was fixed inside this spec, so each header taught a reader the opposite of the tree | `test/plugin/lg-pki-certificate.ci`, `test/reload/lg-pki-reference-reload.ci`, `test/reload/lg-pki-reference-reload-broken.ci` | Headers rewritten to state the fix, name `preparePKIConfig` as the repair, and forbid taking the intermediate out of the plugin fixture |
| 2 | ISSUE | The plaintext-downgrade reload is ACCEPTED and rotates nothing, and the log line is the only thing that tells the operator so. No test asserted that line, so a silent no-op would have passed | `updateLGCertificate` (`cmd/ze/hub/listener_migrate.go`), `TestPlaintextLGHoldsNoRotationHandle` (`cmd/ze/hub/service_lg_tls_test.go`) | `captureMigratorLog` plus four substring assertions. Discrimination: with the `Warn` call deleted the subtest fails on the first missing substring |
| 3 | ISSUE | The Blast Radius table described the looking glass alone, while the work also changed `preparePKIConfig`, which every daemon runs at startup and at every reload | Blast Radius, this spec | Both rows rewritten to name the wider surface and what a revert restores |
| 4 | NOTE | `./le commit audit` reports `internal/component/web/server_tls_test.go` `handshakePeerCerts` as weakened, 2 assertions to 0 | `internal/component/web/server_tls_test.go` | Not a weakening: both assertions moved into `dialHeld`, which `handshakePeerCerts` now calls. Recorded as an accepted row on commit A |
| 5 | ISSUE | `TestCertificateLeafLengthFromYANGLG` opened with `t.Skipf` on a schema that fails to build, copied from its web sibling. A schema that does not build is a defect, and the skip retires the check with no test red and nothing said | `internal/component/config/leaf_length_test.go` | Both tests now `t.Fatalf`. Found by `./le commit create`, which refused the file for adding a `t.Skip` with no ledger row; the fix removed the need for the row rather than writing one |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/service_tls.go` | Yes | `ls -1`: 2.4K, 2026-09-04 |
| `internal/component/lg/register.go`, `doctor.go`, `doctor_test.go`, `server_tls_test.go` | Yes | `ls -1`: 1.8K, 2.3K, 8.5K, 9.6K |
| `cmd/ze/hub/service_lg_tls_test.go`, `listener_migrate_lg_test.go`, `main_pki_test.go` | Yes | `ls -1`: 14K, 4.5K, 7.0K |
| `test/parse/lg-pki-certificate.ci`, `test/parse/lg-pki-certificate-name-too-long.ci` | Yes | `ls -1`: 2.6K, 1.6K |
| `test/plugin/lg-pki-certificate.ci` | Yes | `ls -1`: 2.6K |
| `test/reload/lg-pki-reference-reload.ci`, `test/reload/lg-pki-reference-reload-broken.ci` | Yes | `ls -1`: 1.8K, 2.0K |
| `internal/test/fixture/lg_pki_fixture.go`, `register_lg_pki.go` | Yes | `ls -1`: 26K, 493 bytes |
| `plan/deferrals/lg-pki-certificate.md` | No | Never created; no phase recorded a deferral. Recorded in Deviations |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-11 | The named chain serves and the token gate still refuses | `./le functional plugin`: `lg-pki-certificate` PASSES, 718 of 727 overall; the nine failures name neither `pki` nor `looking-glass` |
| AC-3, AC-7, AC-8 | Startup resolution, the storage rules, and the plaintext-downgrade decision | `go test -race -run 'TestPlaintextLG\|TestBuildLGService\|TestListenerMigratorUpdateLGCertificate' ./cmd/ze/hub` under the full gate set: `ok github.com/ze-software/ze/cmd/ze/hub 4.764s` |
| AC-5, AC-6 | The refusal, the restore and the rotation | `./le functional reload`: 59 pass, both new tests among them |
| AC-13 | The web rotation keeps an open connection | `TestWebServerUpdateTLSCertificate` holds one `*tls.Conn` and one `*bufio.Reader` across the swap and probes over both |
| Chain, the defect this work found | Every intermediate reaches the store from config text | `TestPreparePKIConfigKeepsEveryIntermediate` and `TestReloadInstallsEveryIntermediate` pass, and both go red under a restored round trip with the production error |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `environment.looking-glass.certificate` in a config file | `test/parse/lg-pki-certificate.ci` | Yes: the leaf parses and validates, and the length sibling refuses 256 characters |
| `serviceDeps.LGCertificate` at hub startup | `test/plugin/lg-pki-certificate.ci` | Yes: read the file. It drives a daemon with no `ze init` and dials it with `crypto/tls` |
| A TLS handshake against the looking glass | `test/plugin/lg-pki-certificate.ci` | Yes: the fixture reads `PeerCertificates` and asserts leaf then intermediate |
| `ze.looking-glass.certificate` | `TestLGCertificateEnvWins` | Yes: the reload subtest runs a real `runReload` refused over the env name |
| A reload that changes the name | `test/reload/lg-pki-reference-reload.ci` | Yes: the fixture counts `sighup reload complete` and re-dials |
| `ze doctor` over a broken lg reference | `test/reload/lg-pki-reference-reload-broken.ci`, `TestLGTLSDoctorCheck` | Yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestBuildLGServiceNamedCertificateWithoutBlobStorage`, and `test/plugin/lg-pki-certificate.ci` running no `ze init` |
| A-2 | confirmed | User decision at the design gate. The leaf is `environment.looking-glass.certificate`, its own |
| A-3 | confirmed | `TestExtractLGBlockReadsCertificate/the_name_survives_enabled_false`; `extractLGBlock` applies no `enabled` gate |
| A-4 | confirmed | `test/plugin/lg-tls-default-on.ci` unchanged and passing; `MinVersion` and both `tls.NewListener` sites untouched |
| A-5 | confirmed | Two `cmd/ze` builds, with and without `ze_lg`, run as `ze doctor --json`: only the `ze_lg` build reports `doctor-tls-reference` |
| A-6 | confirmed | `./le tier check` exits 0 with `internal/component/lg/doctor.go` importing `internal/component/config` and `internal/component/pki` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| New user-facing feature (`docs/features.md`) | The PKI row names three consumers and anchors `cmd/ze/hub/service_tls.go -- listenerTLSMaterial`; the Looking Glass row names the leaf | Yes, already at HEAD |
| User guide (`docs/guide/looking-glass.md`) | The `### Serve your own certificate` table matches `listenerTLSMaterial`, `lgCertificateName`, `ServesTLS` and `updateLGCertificate` | Yes |
| Internal architecture (`docs/architecture/pki/tls-listeners.md`) | Every anchor resolves: `./le repository check` reports 0 source-anchor issues | Yes |
| Doctor checks | `lg-tls-certificate` registered with both existing codes. No page enumerates check NAMES, and `docs/guide/health-checks.md` already covers both codes | Yes |
| RFC behavior | N-A: scope is `config`, no wire format changes and no `rfc/short/` row moves | Yes |
| Config syntax (`docs/config-reference.md`, `docs/guide/configuration.md`) | Written, and held out of commit A because a concurrent session's unshipped work shares those files | Partial, named in Documentation Updates |

## Core Insight

A value lost between two representations asks one question before any other: was the second representation needed at all? The intermediate chain was dropped because the hub lowered its config tree to the plugin-facing map and rebuilt a tree from it, and both obvious repairs were to the conversion. Every caller already held the tree, so the conversion was pure loss. The fix that DELETED code was the correct one, and it was the one nobody had looked for.
