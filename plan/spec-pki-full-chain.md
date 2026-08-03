# Spec: pki-full-chain

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

Anchor refresh (2026-07-22 plan review, design unchanged and implementable;
citations below updated in-body): `startWebServer` now `service_web.go`,
`LoadOrGenerateCert` `:282` (persist block `:278-286`, `NewWebServer`
`:307-311`). `NewTLSConfig` (`selfcert.go`), `Intermediate`
(`pki/types.go`), `LoadTLSMaterial`, `CheckCertMaterial`
verified exact.

Design filled 2026-07-10; user instruction 2026-07-10 authorized conversion to ready.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. ~~`internal/component/web/service_web.go` - TLS listener setup~~ (stale path: the file is `cmd/ze/hub/service_web.go`; the listener itself is `internal/component/web/server.go`)
4. `internal/component/pki/` - certificate storage and chain handling
5. `cmd/ze/hub/service_web.go` - hub web TLS bootstrap (`startWebServer`)
6. `internal/component/web/server.go` - `WebServer` TLS listener
7. `internal/core/dnsserver/secure.go` + `tlsmaterial.go` - DoT/DoH TLS material path
8. `cmd/ze/hub/main_reload.go` - reload ordering (`doReload`, `reloadAfterCommit`)

## Task

Ze's web/API server always uses self-signed certificates via `selfcert.LoadOrGenerateCert`
~~(`service_web.go`)~~ (stale citation; verified producer: `cmd/ze/hub/service_web.go`).
There is no path for operators to use PKI-stored certificates
with their intermediate chain for the web/API HTTPS endpoint. The PKI component correctly
stores intermediates (`CertificateEntry.Intermediate`, `types.go`) and includes them
in PEM output (`show.go`, `show.go`), but the web TLS listener never
consumes PKI-stored certs.

This is a two-part gap:
1. Web server cannot use operator-provided certificates from PKI store
2. When #1 is added, the full chain (leaf + intermediates) must be served

Design update 2026-07-10 (review-followup finding, `tmp/review-followup/result-batch-03.md`):
the followup wave added a SECOND TLS-listener surface with the same limitation. The
DoT/DoH listeners (`internal/core/dnsserver/tlsmaterial.go`, consumed by the
as112 and geodns plugins) serve operator file-PEM or an ephemeral self-signed
certificate and never consult the PKI store. This spec generalizes: **TLS listeners
can serve a PKI-stored certificate plus its full chain**, across BOTH consumers
(web/API HTTPS, and the dnsserver-based DoT/DoH listeners of as112 and geodns), with
per-listener YANG config referencing a PKI store entry, chain assembly in the pki
component, unchanged self-signed fallback semantics, doctor checks, and reload/rotation
behavior.

## Required Reading

### Architecture Docs
- [ ] `internal/component/web/` - web server implementation
  → Constraint: `WebServer` builds one `tls.Config` at construction (`server.go`) and wraps every listener with it (`server.go`, `:337`); reload only migrates listen addresses (`cmd/ze/hub/listener_migrate.go`), never TLS material.
- [ ] `internal/component/pki/` - PKI certificate management
  → Decision: chain assembly already exists twice in `show.go` (`certPEM` :171-173, `certBundlePEM` :196-199); the new loader reuses that leaf+intermediate PEM concatenation, in the same package.
  → Constraint: `pki.Load` validates expiry + chain against the stored CA pool before installing (`store.go`), so a loadable store entry is always verifiable up to a stored CA.
- [ ] `ai/rules/config.md` - YANG vs env var decision
  → Decision: operator-facing certificate selection is YANG config (visible, validated, commit/rollback); the `environment/` placement of the web leaf mandates a matching `ze.web.certificate` env var (`ai/patterns/config-option.md` step 3).
- [ ] `ai/rules/architecture.md` - tier placement for the shared loader
  → Constraint: `internal/core/` MUST NOT import `internal/component/` (core import-direction rule, enforced by `scripts/dev/dep_audit.py --check`); so `internal/core/dnsserver` and `internal/core/selfcert` can never call the pki store directly. PKI is explicitly "shared certificate infrastructure for IPsec and future TLS users" (`ai/rules/architecture.md`) -- the loader lives there.
- [ ] `ai/rules/config.md` - leaf naming
  → Decision: reuse the ipsec precedent `leaf certificate { type string; description "Name of the ... certificate in the PKI store." }` (`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`).
- [ ] `ai/rules/plugins.md` - ownership of doctor checks and YANG
  → Constraint: as112/geodns own their YANG leaves and doctor checks; the shared reference-check helper lives in pki (owner of certificate semantics), mirroring how `dnsserver.CheckCertMaterial` is shared today (`internal/core/dnsserver/certcheck.go`).
- [ ] `ai/rules/repo-maintenance.md` - doctor requirements
  → Constraint: "Config leaf that references a file path (cert, key, ...)" and "New service with TLS" both require registered checks; plugins use `registry.Registration.DoctorChecks`, the web component uses `diagnostic.RegisterDoctorCheck()` (Tree is available in both contexts: `internal/core/diagnostic/doctor_registry.go`, `internal/component/plugin/registry` DoctorCheckContext).

### RFC Summaries (MUST for protocol work)
N/A - standard TLS behavior, not protocol extension. (DoT/DoH transports were proven by
`rfc/short/rfc7858.md` / `rfc/short/rfc8484.md` in spec-followup-subsystem; this spec
only changes where their certificate material comes from.)

**Key insights:**
- ~~`service_web.go`: `selfcert.LoadOrGenerateCert` is the only TLS path~~ (stale path) `cmd/ze/hub/service_web.go`: `selfcert.LoadOrGenerateCert` is the only web TLS path; `cmd/ze/hub/service_lg.go` is the same pattern for the looking glass
- ~~`selfcert.go`~~ `internal/core/selfcert/selfcert.go`: `NewTLSConfig` builds `tls.Config` via `tls.X509KeyPair` -- NOTE: `tls.X509KeyPair` parses EVERY CERTIFICATE block in the cert PEM into `tls.Certificate.Certificate`, so a leaf+intermediate PEM concatenation already serves the full chain; no selfcert change is needed, only chain-shaped input (validated by A-3 / `TestNewTLSConfigServesChain`)
- `pki/types.go`: `CertificateEntry` has `Intermediate` field (chain is stored); `PrivateKey` is OPTIONAL (`pki/config.go`) -- a TLS server reference needs a key, so "no private key" is a validation error
- `pki/show.go,197-198`: PEM output includes intermediates
- No wiring exists between PKI store and web server TLS config
- NEW: `internal/core/dnsserver/tlsmaterial.go` (`LoadTLSMaterial`) is the DoT/DoH sibling: file-PEM or ephemeral self-signed, never PKI; `secure.go` (`buildSecureTLS`) is its only caller; `secure.go` folds a leaf-cert fingerprint into the listener signature so cert rotation forces a rebind
- `cmd/ze/hub/main.go`: `zepki.Load` at startup runs BEFORE plugin coordinator creation (:385) and service construction -- good; `cmd/ze/hub/main_reload.go`: on reload `zepki.Load` runs AFTER plugin apply (`s.ReloadConfig` :151, `eng.Reload` :163) -- plugins resolving PKI references at OnConfigure would see the OLD store; the reload ordering must change (AC-10)
- `internal/plugins/as112/register.go`: as112 refuses to run as an external plugin (in-process registries); geodns has no such guard, so an external geodns resolving the process-local pki store must fail loudly, not silently

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] ~~`internal/component/web/service_web.go` - web server TLS setup~~ (file does not exist; superseded by the two rows below)
- [ ] `cmd/ze/hub/service_web.go` - hub web bootstrap: `startWebServer` (:237) loads/creates self-signed PEM from blob storage (:278-286, `LoadOrGenerateCert` :282) and passes `CertPEM`/`KeyPEM` into `zeweb.NewWebServer` (:307-311)
- [ ] `internal/component/web/server.go` - `NewWebServer` (:94) requires PEM material (:107-109), builds one `tls.Config` via `selfcert.NewTLSConfig` (:111), wraps every bound listener (:198, :337); no TLS reload path
- [ ] ~~`internal/component/web/selfcert.go` - self-signed cert generation~~ (stale path; the package is `internal/core/selfcert`)
- [ ] `internal/core/selfcert/selfcert.go` - `NewTLSConfig` (:176-184, `tls.X509KeyPair` + TLS 1.2 floor), `LoadOrGenerateCert` (:190-222, blob-store persistence of the self-signed pair)
- [ ] `internal/component/pki/types.go` - certificate storage types (`CertificateEntry` :23-30: `Raw`, `PrivateKey`, `Intermediate`, `RawInter`)
- [ ] `internal/component/pki/show.go` - certificate display with chain (`certPEM` :167-179, `certBundlePEM` :181-210, `marshalPrivateKeyPEM` same package)
- [ ] `internal/component/pki/store.go` - atomic store: `Validate` (:38-79, expiry + chain verification against stored CAs), `Load` (:82-98), `GetCertificate` (:114)
- [ ] `internal/component/pki/config.go` - `ParseConfig` from config tree (:54-100); `parseDeviceCert` makes `intermediate` and `private.key` optional (:147-173); `validateName` charset/length (:30-50)
- [ ] `internal/component/pki/health.go` - expiry health check + report-bus warnings + Prometheus gauges (`RaiseExpiryWarnings` :101-114, `updateMetrics` :143-167)
- [ ] `internal/core/dnsserver/tlsmaterial.go` - `LoadTLSMaterial` (:26-59): operator file-PEM pair or ephemeral self-signed via selfcert; half-configured pair is an error
- [ ] `internal/core/dnsserver/secure.go` - `SecureConfig` (:135-143: `CertFile`/`KeyFile` only), `ParseSecureLeaves` (:161-196), `ApplyWithSecure` (:204-226), `buildSecureTLS` (:232-245: files re-read per apply so rotation is picked up; self-signed cached), listener signature folds cert fingerprint (:274-290); TLS load failure disables ONLY the secure listeners (:213-215), cleartext stays up (:89-97)
- [ ] `internal/core/dnsserver/certcheck.go` - `CheckCertMaterial` (:34-84): shared doctor helper for file-based material (missing/invalid/expired + 30-day warning window)
- [ ] `internal/plugins/as112/config.go` - `Secure dnsserver.SecureConfig` (:80), `ParseSecureLeaves` call (:180)
- [ ] `internal/plugins/as112/doctor.go` - `checkAS112TLSCert` (:87-124) delegates to `dnsserver.CheckCertMaterial`; codes `doctor-tls-missing|expired|invalid` registered (`register.go`)
- [ ] `internal/plugins/as112/register.go` - in-process-only guard (:223-226); DoctorChecks declaration (:138-157)
- [ ] `internal/plugins/geodns/config.go` - same `Secure` field (:91) and shared parse (:206 area); `internal/plugins/geodns/doctor.go` same cert check; codes at `register.go`
- [ ] `cmd/ze/hub/main.go` - startup: `preparePKIConfig` + `zepki.Load` (:359-367) precede coordinator creation (:385) and service construction
- [ ] `cmd/ze/hub/main_reload.go` - `doReload`: `preparePKIConfig` early (:131-137), plugin apply `s.ReloadConfig` (:151), `eng.Reload` (:163), `lm.ReloadListeners` (:178), `zepki.Load` LAST (:192-203)
- [ ] `cmd/ze/hub/main_pki.go` - `preparePKIConfig` (:10-19): parse + side-effect-free `Validate`
- [ ] `internal/component/config/loader_extract.go` - `WebListenConfig`/`ExtractWebConfig` (:85-130): `environment.web` leaves `enabled`, `server` list, `insecure`, `ui-mode` (with env override precedent at :115)
- [ ] `internal/component/config/environment.go` - existing `ze.web.*` env registrations (:49-60)
- [ ] `cmd/ze/hub/listener_migrate.go` - `Reconfigurable` seam keeps always-on hub code from importing the compile-out-able web package (:53-58)
- [ ] `cmd/ze/hub/service_lg.go` - looking glass TLS uses the identical self-signed-only path (:73-84); OUT OF SCOPE here (Known Limitations)

**Behavior to preserve:**
- Self-signed cert generation as fallback when no PKI cert configured
  (web: `LoadOrGenerateCert` persisted in blob storage, `cmd/ze/hub/service_web.go`;
  DoT/DoH: ephemeral self-signed with SAN fan-out, `tlsmaterial.go`, cached per manager `secure.go`)
- DoT/DoH operator `cert-file`/`key-file` path keeps working exactly as today (file re-read per apply, half-pair error, `tlsmaterial.go`)
- TLS material failure on DoT/DoH disables only the secure listeners, cleartext DNS stays up (`secure.go`, `:89-97`)
- Cert-fingerprint rebind semantics of the dnsserver listener signature (`secure.go`)
- PKI intermediate storage and PEM display (`show.go`)
- `pki.Load` validation semantics (expiry + chain, `store.go`) and expiry health/report/metrics (`health.go`)
- Web server functionality, listener migration on reload (`listener_migrate.go`), TLS 1.2 floor (`selfcert.go`)
- Compile-out rules: always-on hub code must not import `internal/component/web` directly (module-tiers disable-ability; the existing `Reconfigurable` seam pattern)

**Behavior to change:**
- Add YANG config option to specify PKI certificate name for web server (`environment.web.certificate` + env override `ze.web.certificate`)
- Add YANG `certificate` leaf to the as112 and geodns `tls` containers (shared by DoT and DoH), mutually exclusive with `cert-file`/`key-file`
- When configured, load cert + intermediate from PKI store; build TLS material with full certificate chain (new `pki.ServerTLSMaterial`)
- Fall back to self-signed when not configured (unchanged); when configured but unresolvable, fail loudly (web: startup error / commit rejection; DoT/DoH: error log + secure listeners not started, mirroring today's file-material failure semantics)
- Web TLS certificate becomes hot-rotatable on config reload (GetCertificate indirection; today it is fixed at construction)
- Reload ordering: install the new PKI store BEFORE plugin apply so consumers resolving references at OnConfigure see the new material (today `main_reload.go` runs after `:151`/`:163`)
- Doctor checks for a configured-but-broken reference (missing entry, no private key, expired/expiring, incomplete chain)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- YANG config: web server certificate reference to PKI store
  (`environment.web.certificate` leaf, extracted by `ExtractWebConfig` into `WebListenConfig.Certificate`; env override `ze.web.certificate`)
- YANG config: `service.as112.tls.certificate` / `service.geodns.tls.certificate` (parsed by `dnsserver.ParseSecureLeaves` into `SecureConfig.Certificate`)
- PKI material itself enters via the existing `pki { certificate <name> { certificate; intermediate; private { key } } }` config block (`ze-pki-conf.yang`)

### Transformation Path
1. Config specifies PKI certificate name for web server
2. Web server queries PKI store for certificate entry
3. `CertificateEntry` provides leaf cert + `Intermediate` chain
4. `tls.Config.Certificates` built with full chain
5. HTTPS listener serves leaf + intermediates in TLS handshake

Design elaboration (both consumers):
1. Commit/startup parses the tree; `preparePKIConfig` validates the pki block (`main_pki.go`); `zepki.Load` installs the store (startup `main.go`; reload moved before plugin apply, see AC-10)
2. New `pki.ServerTLSMaterial(name)` (component pki, new `tls.go`) resolves the store entry and assembles PEM: leaf + intermediate CERTIFICATE blocks concatenated (same shape as `show.go`) + private-key PEM; errors for unknown name and missing private key
3. Web: hub `startWebServer` picks `pki.ServerTLSMaterial` when `WebListenConfig.Certificate` is set, else `selfcert.LoadOrGenerateCert`; PEM flows into `WebServer`, whose `tls.Config` now uses a GetCertificate indirection over an atomically swappable `tls.Certificate` (chain populated by `tls.X509KeyPair` multi-block parse, `selfcert.go`)
4. DoT/DoH: `SecureConfig.Certificate` set -> `buildSecureTLS` calls an injected TLS-material resolver (consumer plugins inject `pki.ServerTLSMaterial`; core dnsserver stays PKI-free per module tiers); resolved chain PEM feeds the existing `selfcert.NewTLSConfig` path; the listener-signature fingerprint (`secure.go`) rebinds on rotation
5. Reload: new store installed first; hub calls the web cert-updater seam so the served web certificate rotates without rebind; plugins re-resolve during their apply and rebind if the fingerprint changed
6. TLS handshake serves leaf + intermediate to clients (both consumers)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> web server | YANG tree resolution | [ ] |
| Web server -> PKI store | PKI component query | [ ] |
| Web server -> TLS listener | `tls.Config` with chain | [ ] |
| Config -> as112/geodns plugin | JSON config section -> `ParseSecureLeaves` (`secure.go`) | [ ] |
| Plugin -> PKI store | in-process call to `pki.ServerTLSMaterial` (injected into dnsserver as resolver func; external-process geodns fails loudly, see R-4) | [ ] |
| core dnsserver <- component pki | ONLY via injected resolver func; no core->component import (module tiers) | [ ] |
| Hub reload -> web TLS | cert-updater seam next to `Reconfigurable` (`listener_migrate.go` pattern, keeps compile-out) | [ ] |

### Integration Points
- PKI component certificate lookup (`pki.GetCertificate`, `store.go`; new `pki.ServerTLSMaterial` wraps it)
- Web server TLS configuration (`web/server.go,131,198`; hub glue `cmd/ze/hub/service_web.go`)
- YANG config for certificate reference (`ze-web-conf.yang`, `ze-as112-conf.yang` tls container :147, `ze-geodns-conf.yang` tls container; `ExtractWebConfig` `loader_extract.go`)
- dnsserver secure listener path (`secure.go`)
- Reload pipeline (`main_reload.go`) and expiry health/reporting (`pki/health.go`) which already covers any store entry, so served certs inherit expiry metrics for free

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PKI store is available before web server starts | Component startup order | Would need lazy cert loading | Check component dependency graph | confirmed -- `zepki.Load` at `cmd/ze/hub/main.go` precedes coordinator creation (:385) and service construction; web/lg/mcp services are built after the engine exists |
| A-2 | Certificate rotation needs graceful reload | Standard practice | Would need server restart | Check if web server supports TLS cert reload | confirmed-as-gap -- `web/server.go,131` fixes the `tls.Config` at construction and `listener_migrate.go` only migrates addresses; this spec ADDS rotation via GetCertificate indirection (AC-9) |
| A-3 | `tls.X509KeyPair` on a multi-block PEM yields a served chain (leaf + intermediates) with no selfcert change | Go stdlib documented behavior; `selfcert.go` | Would need a dedicated chain builder in selfcert | Unit test `TestNewTLSConfigServesChain` asserting `len(tls.Certificate.Certificate) == 2` | confirmed -- the test also asserts leaf-first ordering; selfcert is unchanged |
| A-4 | as112 and geodns doctor checks receive the FULL config tree, so a pki-reference check can parse the pki block offline | `internal/plugins/as112/doctor.go` (`ctx.Tree.(*config.Tree)` then reads `service`); `diagnostic/doctor_registry.go` | Doctor check would need the live store and could not run offline | Unit test feeding a tree containing both `pki` and `service` roots | confirmed -- `pki.ParseConfig(tree)` runs inside all three checks; `TestWebTLSDoctorCheck` drives a tree carrying both roots |
| A-5 | Plugin OnConfigure during reload runs before `zepki.Load` today (ordering hazard is real) | `main_reload.go` (`s.ReloadConfig`, plugin verify/apply) and `:163` (`eng.Reload`) precede `:192` (`zepki.Load`) | Ordering fix unnecessary; drop AC-10 | Read of `doReload`; regression test `test/reload/pki-reference-reload.ci` proving one-commit add-cert+reference works after the fix | confirmed -- and MEASURED: restoring the old ordering makes the .ci fail with `pki: certificate lan-next not found (available: lan)` |
| A-6 | as112/geodns run in the hub process so `pki.ServerTLSMaterial` reads the live store | as112 refuses external (`register.go`); geodns MAY be external, handled by R-4 | Resolution returns not-found in external process | as112: existing guard; geodns: loud-failure path unit test | confirmed -- `TestBuildSecureTLSResolverFailureIsLoud` proves an external process (empty store) leaves the secure listeners down and never reaches the self-signed fallback |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PKI cert expires without auto-renewal | HTTPS stops working | Fall back to self-signed on cert load failure |
| R-2 | Circular dependency: PKI needs web, web needs PKI | Startup deadlock | Lazy cert loading after both components are up |
| R-3 | Reload rollback leaves store/config drift once `zepki.Load` moves before plugin apply | Rollback path in `doReload` restores provider but serves new certs | Re-install the prior PKI config in `rollbackReload` (re-prepare from the prior provider snapshot's `pki` root) |
| R-4 | geodns configured `external` with `tls.certificate` set silently serves nothing | DoT/DoH listeners absent, only a log line | Loud failure: resolution error logged at error level + secure listeners not started (existing `secure.go` semantics); doctor check flags the reference; document in geodns YANG description |
| R-5 | Web configured with a broken reference silently downgrades to self-signed (operator believes real cert is served) | Browser shows self-signed warning in production | Fail CLOSED for web: startup returns error (like pki config errors `main.go`); reload rejects the commit; never silent fallback when a name WAS configured |
| R-6 | ~~`intermediate` supports a single certificate~~ VOID at implementation | -- | `CertificateEntry.Intermediates` is a SLICE (RFC 7296 Section 3.6 work landed since this spec was written), so deeper chains already express and `ServerTLSMaterial` emits every intermediate. No follow-up owed |

Notes on skeleton rows (append-only): R-1 is refined by R-5 -- the self-signed fallback applies only when NO reference is configured; a configured-but-expired cert is caught by `pki.Validate` at commit (`store.go`) and by expiry warnings 30 days ahead (`health.go`). R-2 did not materialize: pki is a passive in-process store loaded from the config tree (`main.go`) with no dependency on web; no lazy loading needed.

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config web cert reference | -> | TLS listener uses PKI cert with chain | `TestStartWebServerUsesPKIMaterial` (cmd/ze/hub) + `test/parse/web-pki-certificate.ci` |
| No cert configured | -> | Self-signed fallback (unchanged) | `TestWebServerSelfSignedFallbackUnchanged` (cmd/ze/hub, asserts `LoadOrGenerateCert` path) |
| `environment.web.certificate` leaf | -> | `ExtractWebConfig` -> `WebListenConfig.Certificate` | `TestExtractWebConfigCertificate` (internal/component/config) |
| `service.as112.tls.certificate` | -> | `ParseSecureLeaves` -> `SecureConfig.Certificate` -> resolver -> `bindDoT` | `TestParseSecureLeavesCertificate` (internal/core/dnsserver) + `test/plugin/as112-dot-pki.ci` |
| `service.geodns.tls.certificate` | -> | same shared parse + resolver injection in geodns server apply | `TestGeoDNSAppliesPKICertificate` (internal/plugins/geodns) + `test/plugin/geodns-dot-pki.ci` |
| Config reload with changed pki material | -> | store installed before plugin apply; web cert updater invoked | `test/reload/pki-reference-reload.ci` |
| `ze doctor` with broken reference | -> | pki reference check emits `doctor-tls-reference` | `TestCheckCertReferenceDiagnostics` (internal/component/pki) + `TestAS112TLSDiagnosticPKIReference` (internal/plugins/as112) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PKI certificate name configured for web server | HTTPS serves leaf + intermediate chain |
| AC-2 | No PKI certificate configured | Self-signed cert used (backward compatible) |
| AC-3 | Referenced PKI certificate does not exist | Config validation error |
| AC-4 | `openssl s_client` against configured web server | Full chain visible in handshake |
| AC-5 | `tls { certificate <name> }` on as112 (or geodns) with DoT/DoH enabled | DoT and DoH listeners serve leaf + intermediate chain from the PKI store |
| AC-6 | `certificate` set together with `cert-file` or `key-file` in a tls container | Parse/verify error (mutually exclusive), commit rejected by the plugin verifier |
| AC-7 | Referenced PKI certificate exists but has no `private { key }` | Web: startup/commit error; DoT/DoH: loud apply error, secure listeners not started, cleartext unaffected; doctor reports `doctor-tls-reference` |
| AC-8 | `ze doctor` on a config whose tls/web reference is missing, keyless, expired, or has an AKI/SKI-mismatched intermediate | Diagnostics emitted: `doctor-tls-reference` (missing/keyless/mismatch) or `doctor-tls-expired` (expiry, 30-day warning window), from the owning plugin/component check |
| AC-9 | Config reload replaces the referenced certificate's material | Web serves the new chain without listener rebind (GetCertificate swap); DoT/DoH rebind via existing fingerprint signature; no daemon restart |
| AC-10 | Single commit adds a new pki certificate AND references it (web or DoT/DoH) | Reference resolves during that same commit: reload installs the PKI store before plugin apply; rollback restores the prior store |
| AC-11 | Referenced entry lacks `intermediate` and its leaf is issued directly by a stored root CA | Served chain is just the leaf; no error, no incomplete-chain diagnostic |

AC-3 scope note: "config validation error" means commit rejection on the hub-owned web path
(startup error / reload error). For as112/geodns the same condition is AC-7/AC-8 behavior
(loud apply failure + doctor), because their verifier only receives the `service` root
(`register.go` ConfigRoots) and must not have the pki root (private keys) delivered
to a possibly-external plugin process.
AC-4 is proven with a Go `crypto/tls` client asserting two `PeerCertificates` (the .ci
observer sandbox cannot drive TLS handshakes, see `test/plugin/as112-dot.ci`);
`openssl s_client` remains the manual operator check.

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures web server to use PKI cert | YANG -> web server loads cert + chain from PKI -> HTTPS serves full chain | `TestWebServerServesPKIChain` (internal/component/web) + `test/parse/web-pki-certificate.ci` |
| 2 | Does not configure web cert | Web server auto-generates self-signed cert (existing behavior) | `TestWebServerSelfSignedFallbackUnchanged` + existing web .ci suite stays green |
| 3 | Enables DoT+DoH on as112 with `tls { certificate lan }` | JSON config -> `ParseSecureLeaves` -> `ApplyWithSecure` -> resolver (`pki.ServerTLSMaterial`) -> `bindDoT`/`bindDoH` serve the chain | `TestBuildSecureTLSFromResolver` + `test/plugin/as112-dot-pki.ci` |
| 4 | Rotates the certificate content in one commit | reload -> `zepki.Load` (moved early) -> web cert updater swap + dnsserver fingerprint rebind | `TestWebServerUpdateTLSCertificate` + `test/reload/pki-reference-reload.ci` |
| 5 | Runs `ze doctor` before deploying a config with a typo'd reference | doctor -> owner check -> `pki.ParseConfig(tree)` + `pki.CheckCertReference` -> `doctor-tls-reference` | `TestAS112TLSDiagnosticPKIReference`, `TestWebTLSDoctorCheck` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestServerTLSMaterialAssemblesChain` | `internal/component/pki/tls_test.go` | leaf+intermediate PEM concatenation + key PEM from a store entry (mirrors `certBundlePEM` shape) | |
| `TestServerTLSMaterialLeafOnly` | `internal/component/pki/tls_test.go` | entry without intermediate yields single-block cert PEM (AC-11) | |
| `TestServerTLSMaterialNotFound` | `internal/component/pki/tls_test.go` | unknown name -> error naming the reference | |
| `TestServerTLSMaterialNoPrivateKey` | `internal/component/pki/tls_test.go` | keyless entry -> error (AC-7) | |
| `TestCheckCertReferenceDiagnostics` | `internal/component/pki/tls_test.go` | doctor helper: missing ref, keyless, expired, AKI/SKI mismatch, happy path (AC-8, AC-11) | |
| `TestNewTLSConfigServesChain` | `internal/core/selfcert/selfcert_test.go` | A-3: multi-block PEM -> `len(Certificates[0].Certificate) == 2` | |
| `TestWebServerServesPKIChain` | `internal/component/web/server_test.go` | real handshake via `crypto/tls` client sees 2 `PeerCertificates` (AC-1/AC-4) | |
| `TestWebServerUpdateTLSCertificate` | `internal/component/web/server_test.go` | GetCertificate indirection swaps the served cert without rebind (AC-9) | |
| `TestExtractWebConfigCertificate` | `internal/component/config/loader_extract_test.go` | leaf extraction + `ze.web.certificate` env override precedence | |
| `TestStartWebServerUsesPKIMaterial` | `cmd/ze/hub/service_web_test.go` | hub glue picks PKI path when configured, errors (fail closed) on broken reference (AC-3, R-5) | |
| `TestWebServerSelfSignedFallbackUnchanged` | `cmd/ze/hub/service_web_test.go` | no reference -> `LoadOrGenerateCert` path untouched (AC-2) | |
| `TestParseSecureLeavesCertificate` | `internal/core/dnsserver/secure_test.go` | `certificate` leaf parsed into `SecureConfig.Certificate` | |
| `TestParseSecureLeavesCertificateConflict` | `internal/core/dnsserver/secure_test.go` | certificate + cert-file/key-file -> error (AC-6) | |
| `TestBuildSecureTLSFromResolver` | `internal/core/dnsserver/secure_test.go` | resolver injected -> chain served; resolver error -> secure listeners not started, cleartext untouched (AC-5, AC-7) | |
| `TestBuildSecureTLSResolverMissing` | `internal/core/dnsserver/secure_test.go` | `Certificate` set but no resolver injected -> loud error (guards misuse by future consumers) | |
| `TestAS112TLSDiagnosticPKIReference` | `internal/plugins/as112/doctor_test.go` | doctor emits `doctor-tls-reference`/`doctor-tls-expired` from a tree with pki + service roots (A-4, AC-8) | |
| `TestGeoDNSTLSDiagnosticPKIReference` | `internal/plugins/geodns/doctor_test.go` | same for geodns | |
| `TestGeoDNSAppliesPKICertificate` | `internal/plugins/geodns/server_test.go` | apply path injects `pki.ServerTLSMaterial` resolver | |
| `TestWebTLSDoctorCheck` | `internal/component/web/doctor_test.go` | web component doctor check (diagnostic registry) for `environment.web.certificate` | |
| `TestReloadInstallsPKIBeforePluginApply` | `cmd/ze/hub/main_reload_test.go` | AC-10 ordering + R-3 rollback reinstalls prior store | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `certificate` leaf length (all three YANG modules) | 1..255 chars (mirrors `pki/config.go` maxNameLen + `validateName`) | 255-char name | empty string | 256-char name |

No new numeric leaves; ports/paths of the tls/doh containers are unchanged.

Implementation note: enforcing this row required implementing YANG `length` in the
config schema. It had never been enforced (`schema.go` carried `Patterns` and `Ranges`
only), so the ten pre-existing `length "1..255"` declarations were decorative. See
Implementation Summary, Bugs Found/Fixed. Covered by `TestValidateLeafValueLength` and
`test/parse/web-pki-certificate-name-too-long.ci` (255 accepted, 256 rejected, empty
rejected, path separator rejected by the pattern).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-pki-certificate` | `test/parse/web-pki-certificate.ci` | config with pki block + `environment.web.certificate` accepted; daemon starts serving the referenced cert (log assertion) | |
| `web-pki-certificate-missing` | `test/parse/web-pki-certificate-missing.ci` | reference to a nonexistent pki entry rejected at startup (AC-3) | |
| `as112-dot-pki` | `test/plugin/as112-dot-pki.ci` | as112 DoT bound and serving with `tls { certificate }` (modeled on `test/plugin/as112-dot.ci`) | |
| `as112-tls-certificate-conflict` | `test/parse/as112-tls-certificate-conflict.ci` | certificate + cert-file in one tls container rejected (AC-6) | |
| `geodns-dot-pki` | `test/plugin/geodns-dot-pki.ci` | geodns DoT with PKI-referenced cert | |
| `pki-reference-reload` | `test/reload/pki-reference-reload.ci` | one commit adds cert + reference; reload applies cleanly; second reload rotates material (AC-9, AC-10) | |

### Interop Tests (MANDATORY for protocol features)
N/A -- no wire-protocol change. TLS serving uses stock `crypto/tls`; DoT/DoH transports
and their interop runbook were proven in spec-followup-subsystem (`test/plugin/as112-dot.ci`
cites `TestDoTListener` + runbook). The chain content is asserted by Go handshake tests above.

### Future (if deferring any tests)
None deferred.

## Files to Modify
- `internal/component/web/yang/ze-web-conf.yang` - add `leaf certificate` (type string, length 1..255, pattern matching pki name charset, description naming the PKI store + the `ze.web.certificate` env override)
- `internal/component/config/loader_extract.go` - `WebListenConfig.Certificate` field + extraction with env override (mirror `ui-mode` precedent at :115)
- `internal/component/config/environment.go` - `env.MustRegister` for `ze.web.certificate` (config-option pattern step 3; environment/ leaf rule)
- `internal/component/web/server.go` - GetCertificate indirection over an atomic `tls.Certificate`; new `UpdateTLSCertificate(certPEM, keyPEM)` method; keep `NewWebServer` signature
- `cmd/ze/hub/service_web.go` - resolve `pki.ServerTLSMaterial` when configured (fail closed), else existing fallback; wire the cert-updater seam
- `cmd/ze/hub/main.go` - startup validation of `environment.web.certificate` against the prepared PKI config (error out like pki errors at :360-367)
- `cmd/ze/hub/main_reload.go` - move `zepki.Load` before `s.ReloadConfig`; reject commit on broken web reference; call web cert updater after install; rollback reinstalls prior PKI config (R-3)
- `cmd/ze/hub/listener_migrate.go` (or sibling hub file) - TLS-updatable seam alongside `Reconfigurable` so always-on code stays free of the gated web import
- `internal/core/dnsserver/secure.go` - `SecureConfig.Certificate`; `ParseSecureLeaves` parses it + mutual-exclusion error; `buildSecureTLS` resolver branch; resolver injection point on `Manager` (option or `ApplyWithSecure` companion)
- `internal/plugins/as112/server.go` - inject `pki.ServerTLSMaterial` as the manager's TLS material resolver
- `internal/plugins/as112/doctor.go` - extend `checkAS112TLSCert`: when `tls.certificate` set, run `pki.ParseConfig(tree)` + `pki.CheckCertReference`
- `internal/plugins/as112/register.go` - add `doctor-tls-reference` to the check's `Codes`
- `internal/plugins/as112/yang/ze-as112-conf.yang` - `leaf certificate` in the tls container (:147) with mutual-exclusion description
- `internal/plugins/geodns/server.go`, `doctor.go`, `register.go`, `yang/ze-geodns-conf.yang` - same four changes as as112
- `internal/core/diagnostic/codes.go` - register `doctor-tls-reference` (title, description, examples; `ze explain` requirement)
- `internal/component/web/register.go` (or new `doctor.go`) - `diagnostic.RegisterDoctorCheck` for the web reference check

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] yes | `internal/component/web/yang/ze-web-conf.yang`, `internal/plugins/as112/yang/ze-as112-conf.yang`, `internal/plugins/geodns/yang/ze-geodns-conf.yang` (existing modules extended; no new module registration) |
| YANG validation constraints | [ ] yes | `certificate` leaves get `length "1..255"` + `pattern` for the pki name charset (`pki/config.go`); booleans/ports unchanged |
| YANG custom validators | [ ] no | cross-root reference existence cannot be a per-leaf `ValidateFn` (no tree access); enforced at hub startup/reload (web) and doctor + apply (plugins). Optional follow-up: `CompleteFn` completing live store names |
| CLI commands/flags | [ ] no | config-only feature; no new verbs |
| CLI grammar (action before identifier) | [ ] no | no CLI change |
| Editor autocomplete | [ ] partial | native for the new leaves; live cert-name completion deferred (see custom validators row) |
| Functional test for new RPC/API | [ ] yes | `.ci` rows in Functional Tests table (test/parse, test/plugin, test/reload) |
| Pipe completeness | [ ] no | no new output-producing command |
| Env var registration | [ ] yes | `ze.web.certificate` in `internal/component/config/environment.go` (environment/ leaf rule); plugin `service.*` leaves need none |
| Doctor check for runtime dependencies | [ ] yes | `pki.CheckCertReference` helper (pki owns cert semantics); as112/geodns extend existing tls checks; web registers via `diagnostic.RegisterDoctorCheck`; new code `doctor-tls-reference` in `internal/core/diagnostic/codes.go` + unit tests + functional doctor coverage test (`TestDoctorCoverageCodesRegistered`) |
| Prometheus counters/metrics | [ ] no new | store entries already emit `ze_pki_certificate_expiry_seconds` / `ze_pki_certificate_near_expiry` (`pki/health.go`); serving certs are store entries, so expiry observability is inherited |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` (PKI-backed TLS for web + DoT/DoH) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` (three new `certificate` leaves + env override) |
| 3 | CLI command added/changed? | [ ] no | - |
| 4 | API/RPC added/changed? | [ ] no | - |
| 5 | Plugin added/changed? | [ ] yes | as112/geodns guide sections for the tls container (grep `docs/` for cert-file anchors) |
| 6 | Has a user guide page? | [ ] yes | web/services guide page describing HTTPS certificate selection and rotation |
| 7 | Wire format changed? | [ ] no | - |
| 8 | Plugin SDK/protocol changed? | [ ] no | resolver injection is in-process Go API, not the JSON protocol |
| 9 | RFC behavior implemented/changed? | [ ] no | standard TLS |
| 10 | Test infrastructure changed? | [ ] no | - |
| 11 | Affects daemon comparison? | [ ] maybe | `docs/comparison.md` if it lists TLS cert management |
| 12 | Internal architecture changed? | [ ] yes | reload ordering note where `doReload` sequencing is documented (`main_reload.go` header comment is the source anchor) |
| 13 | Route metadata keys? | [ ] no | - |
| 14 | Prometheus counters added/changed? | [ ] no | none added |
| 15 | Registered inventory changed? | [ ] yes | new doctor code `doctor-tls-reference` (explainable via `ze explain`) |
| 16 | Changed files referenced by doc source anchors? | [ ] check | grep `docs/` for `service_web.go`, `secure.go`, `tlsmaterial.go` anchors |
| 17 | Existing config/CLI examples in docs for this area? | [ ] check | DoT/DoH examples showing cert-file must mention the certificate alternative |

## Files to Create
- `internal/component/pki/tls.go` - `ServerTLSMaterial(name) (certPEM, keyPEM []byte, err error)` + `CheckCertReference(cfg *PKIConfig, name string, now time.Time)` doctor helper (reuses `marshalPrivateKeyPEM`, chain shape of `show.go`)
- `internal/component/pki/tls_test.go` - unit tests listed above
- `internal/component/web/doctor.go` + `doctor_test.go` - web reference doctor check (if not folded into register.go)
- `cmd/ze/hub/service_web_test.go` additions or new test file for the hub glue tests
- `test/parse/web-pki-certificate.ci`, `test/parse/web-pki-certificate-missing.ci`, `test/parse/as112-tls-certificate-conflict.ci`
- `test/plugin/as112-dot-pki.ci`, `test/plugin/geodns-dot-pki.ci`
- `test/reload/pki-reference-reload.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- config surface reaches consumer structs
   - Tests: `TestExtractWebConfigCertificate`, `TestParseSecureLeavesCertificate`, `TestParseSecureLeavesCertificateConflict`; failing `.ci` skeletons `test/parse/web-pki-certificate.ci`, `test/parse/as112-tls-certificate-conflict.ci`
   - Files: three YANG modules, `loader_extract.go`, `environment.go`, `secure.go` (parse only)
   - Verify: leaves parse into `WebListenConfig.Certificate` / `SecureConfig.Certificate`; conflict rejected; downstream still ignores the values (wiring tests for serving fail)
2. **Phase: PKI chain loader** -- `pki.ServerTLSMaterial` + `pki.CheckCertReference`
   - Tests: the five `internal/component/pki/tls_test.go` tests + `TestNewTLSConfigServesChain` (A-3 gate: validate stdlib chain behavior BEFORE building on it)
   - Files: `internal/component/pki/tls.go`
   - Verify: chain PEM shape identical to `certBundlePEM` output; keyless/missing errors
3. **Phase: Web consumer** -- hub resolution, fail-closed validation, rotation
   - Tests: `TestWebServerServesPKIChain`, `TestWebServerUpdateTLSCertificate`, `TestStartWebServerUsesPKIMaterial`, `TestWebServerSelfSignedFallbackUnchanged`, `TestReloadInstallsPKIBeforePluginApply`
   - Files: `web/server.go`, `cmd/ze/hub/service_web.go`, `main.go`, `main_reload.go`, seam file
   - Verify: handshake serves 2 certs; broken reference fails startup and rejects commit; reload ordering moved with rollback reinstall (R-3); fallback untouched
4. **Phase: dnsserver consumers** -- resolver injection, as112 + geodns
   - Tests: `TestBuildSecureTLSFromResolver`, `TestBuildSecureTLSResolverMissing`, `TestGeoDNSAppliesPKICertificate`
   - Files: `secure.go` (buildSecureTLS + injection point), `as112/server.go`, `geodns/server.go`
   - Verify: chain served over DoT/DoH; resolution failure keeps cleartext up and logs at error level; fingerprint rebind on rotation
5. **Phase: Doctor checks** -- reference diagnostics on all three surfaces
   - Tests: `TestCheckCertReferenceDiagnostics`, `TestAS112TLSDiagnosticPKIReference`, `TestGeoDNSTLSDiagnosticPKIReference`, `TestWebTLSDoctorCheck`; `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'`
   - Files: `pki/tls.go` (helper), plugin doctor files + registrations, web doctor registration, `diagnostic/codes.go`
   - Verify: `doctor-tls-reference` registered and explainable; checks fire only when the leaf is set
6. **Functional tests** -- fill the `.ci` files (`test/plugin/as112-dot-pki.ci`, `geodns-dot-pki.ci`, `test/reload/pki-reference-reload.ci`, complete the parse ones)
7. **Full verification** -- `make ze-verify` (respect `scripts/dev/verify-status.sh check` freshness)
8. **Complete spec** -- audit tables, learned summary `plan/learned/NNN-pki-full-chain.md`, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 has implementation with file:line |
| Feature completeness | Every End-to-End User Story path works; web and BOTH dns consumers covered |
| Correctness | Chain order leaf-first; fail-closed on web (R-5) vs degrade-loud on DoT/DoH (existing semantics preserved); rollback reinstalls prior store (R-3) |
| Naming | `certificate` leaf on all three surfaces; env `ze.web.certificate` leaf segment matches YANG leaf |
| Data flow | core dnsserver never imports component pki (run `scripts/dev/dep_audit.py --check`); resolution only via injected resolver |
| CLI grammar | N/A (no CLI change) |
| Registration over hardcoding | doctor checks registered via existing registries; no plugin spelling added to central packages |
| Doctor checks | `doctor-tls-reference` in `diagnostic/codes.go`, owner-registered, unit + functional coverage |
| YANG validation | new leaves have `length` + `pattern`; no bare `type string` |
| Prometheus counters | no new metrics; confirm expiry gauges cover the referenced certs (they are store entries) |
| Rule: no-workarounds | reload ordering fixed at the source (`doReload`), not worked around in consumers |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `pki.ServerTLSMaterial` with non-test callers in hub + both plugins | grep -rn "ServerTLSMaterial" cmd/ internal/ (wiring-completeness: every exported symbol has a non-test caller) |
| Three YANG `certificate` leaves | grep "leaf certificate" in the three -conf.yang files |
| Web GetCertificate rotation | `go test ./internal/component/web -run TestWebServerUpdateTLSCertificate` |
| Reload ordering + rollback | `go test ./cmd/ze/hub -run TestReloadInstallsPKIBeforePluginApply`; read `doReload` order |
| Doctor code registered | `go test ./internal/component/doctor -run TestDoctorCoverageCodesRegistered` |
| Six .ci files exist and pass | `ls test/parse/web-pki-* test/plugin/*-dot-pki.ci test/reload/pki-reference-reload.ci` + functional run |
| Tier rule intact | `scripts/dev/dep_audit.py --check` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Private key handling | key PEM never logged, never written to disk on these paths (unlike ipsec `ExportPEM`); keys stay in-memory PEM/DER; error messages name the certificate, never key material |
| Fail-closed web | configured-but-broken reference must NEVER silently serve self-signed (R-5); verify both startup and reload paths |
| Reference name as input | name flows into map lookup only; `validateName` charset already prevents traversal (`pki/config.go`); no filesystem use |
| External plugin boundary | pki root is NOT added to plugin `ConfigRoots`/`WantsConfig` (would ship private keys over the plugin transport); geodns-external resolution fails loudly (R-4) |
| Downgrade on rotation failure | a reload that breaks the reference must reject the commit (web) or keep serving the previous material until rebind (DoT/DoH), never fall back to self-signed mid-flight |
| TLS floor | TLS 1.2 minimum preserved (`selfcert.go`) on all new paths |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Spec skeleton cited `internal/component/web/service_web.go` and `web/selfcert.go` | Producers are `cmd/ze/hub/service_web.go` and `internal/core/selfcert/selfcert.go` | 2026-07-10 design research read the tree | Citations corrected; design unchanged |
| The `certificate` leaf's `length "1..255"` would be enforced by the YANG schema | `length` was parsed and discarded; nothing in the config validator enforced string length | Ran `ze config validate` on a 256-character name: it passed | Implemented `LeafNode.Lengths` + `validateLengths`; ten pre-existing leaves gained real enforcement |
| A pki store certificate needs no CA to load | `pki.Validate` verifies every device certificate against the stored CA pool, so a self-signed test fixture is refused | Reload test failed with `certificate chain validation failed: x509: certificate signed by unknown authority` | Test fixtures build a CA and sign the leaf with it |
| One `.ci` could drive three reloads with fixed sleeps | Several SIGHUPs race the daemon's reload queue; a HUP arriving mid-reload is queued and replayed | The test passed and failed alternately on the same code | Split into one reload per file; 5/5 parallel rounds clean |
| `daemon.ready` is the right fence for a signal script | The runner waits for `daemon.ready` and only THEN writes `daemon.pid`, so pid is the later file | Script broke out on ready, read pid, and got "daemon.pid not found" under parallel load | Fence on `daemon.pid` |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- `tls.X509KeyPair` already turns concatenated PEM into a served chain, so "serve the full chain" is a data-shaping problem (produce leaf+intermediate PEM in pki), not a TLS-stack problem. The whole feature reduces to: one loader in pki, config plumbing per consumer, and lifecycle (validation, rotation, ordering, doctor).
- The reload pipeline order (`main_reload.go`) is itself config surface: a reference-style config (name -> store) only works if the store install precedes every consumer apply in the same commit.

## Core Insight
A cross-cutting capability ("serve PKI cert + chain") should be one producer in the
owning component (pki) plus thin per-consumer seams, never a shared loader in core:
the tier rule (core cannot import component) makes the injection seam the ONLY correct
shape for core-hosted listeners like dnsserver.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Chain assembly lives in `internal/component/pki` (`ServerTLSMaterial`, new `tls.go`) | (a) shared loader in `internal/core` (selfcert or dnsserver); (b) each consumer assembles from `CertificateEntry` | (a) violates the core import-direction rule (`ai/rules/architecture.md`, dep_audit gate) since core may not import component pki; (b) duplicates the `show.go` chain logic in 3+ places. architecture.md names pki the home for shared cert infrastructure for TLS users |
| Loader returns PEM bytes (chain + key), not `*tls.Certificate` | returning `tls.Certificate` or `*tls.Config` | PEM feeds the EXISTING `selfcert.NewTLSConfig` path both consumers already use (`web/server.go`, `tlsmaterial.go`), so TLS policy (1.2 floor) stays in one place and consumers change minimally |
| dnsserver gets the material via an injected resolver func; `SecureConfig` carries only the `Certificate` name | (a) core dnsserver imports pki; (b) plugins pre-resolve PEM into new SecureConfig byte fields | (a) tier violation; (b) splits parse-time struct with apply-time material and would re-resolve stale material between OnConfigure runs; the resolver runs inside `buildSecureTLS` per apply, matching the existing per-apply file re-read that makes rotation work (`secure.go`) |
| Web rotation via `tls.Config.GetCertificate` over an atomic cert + hub updater called after `zepki.Load` on reload | (a) restart/rebind web listeners on cert change; (b) no rotation (restart required) | web holds long-lived sessions (SSE); GetCertificate swap is race-free per handshake and needs no listener churn; dnsserver keeps its existing rebind-by-fingerprint because that machinery already exists (`secure.go`) |
| Move `zepki.Load` before plugin apply in `doReload`; rollback reinstalls prior store | validate-only early + late install (today); double-load | the store is already validated side-effect-free early (`main_reload.go`); consumers resolving names during apply must see the same commit's material (AC-10); rollback path re-prepares the prior pki root so drift (R-3) is impossible |
| Web fail-closed, DoT/DoH degrade-loud on broken references | uniform silent fallback to self-signed | a configured web reference silently downgrading is an operator trap (R-5); DoT/DoH already have a defined loud-degrade contract that keeps cleartext DNS up (`secure.go`) and their commit path must not depend on cross-root data a possibly-external plugin cannot see |
| Plugin-side commit rejection for missing references NOT attempted; doctor + loud apply instead | deliver `pki` root to plugin verifiers via ConfigRoots | delivering the pki root would ship private keys over the plugin transport to possibly-external processes; doctor checks get the full tree in the hub process (A-4) and cover the pre-flight need |
| One new doctor code `doctor-tls-reference`; reuse `doctor-tls-expired` for expiry | reuse `doctor-tls-missing`/`invalid` for reference problems | missing-file and missing-store-entry have different operator fixes; a dedicated code keeps `ze explain` actionable, while expiry semantics are identical to the file case |
| Leaf named `certificate` on all three surfaces | `pki-certificate`, `certificate-name` | matches the established ipsec reference precedent (`ze-ipsec-conf.yang`: "Name of the server certificate in the PKI store") and config-naming (noun, no redundancy) |

## Known Limitations
- No current bug: self-signed certs have no chain to serve
- Gap is architectural: no path exists to use operator certs even if desired
- ~~Certificate rotation / reload not covered in this skeleton~~ (now in scope: AC-9/AC-10 cover rotation and reload ordering)
- Looking glass TLS (`cmd/ze/hub/service_lg.go`) keeps the self-signed-only path; it is the same `LoadOrGenerateCert` + PEM-in pattern, so extending it is a small follow-up consuming `pki.ServerTLSMaterial` (out of scope to keep this spec bounded; same for MCP/REST if they grow TLS)
- ~~Single intermediate only~~ no longer true: `pki` stores a slice of intermediates and the served chain carries all of them (R-6 void)
- Client-certificate authentication (mTLS) on these listeners is not in scope; this spec covers the server-side chain only
- Live CLI completion of store certificate names (CompleteFn) is a UX follow-up, not required for correctness
- geodns run as an external plugin cannot resolve the in-process store; behavior is loud failure + doctor diagnostic (R-4), not support

## Implementation Summary

### What Was Implemented
- `pki.ServerTLSMaterial(name)` (`internal/component/pki/tls.go`): leaf + every stored
  intermediate as one PEM document, leaf first, plus the PKCS#8 key. Errors on unknown
  name, keyless entry, and empty name; returns no material alongside an error.
- `pki.CheckCertReference(cfg, name, now)` (same file): offline doctor helper over a
  PARSED config, so a broken reference is reported before the config is committed.
  Emits `doctor-tls-reference` and `doctor-tls-expired`.
- `certificate` leaf on three YANG surfaces: `environment.web`, `service.as112.tls`,
  `service.geodns.tls`. Type `string { length "1..255"; pattern '[A-Za-z0-9._-]+'; }`.
- `WebListenConfig.Certificate` plus the `ExtractWebSettings` / `ExtractWebConfig`
  split (`loader_extract.go`), and `ze.web.certificate`.
- `WebServer.UpdateTLSCertificate` + `tls.Config.GetCertificate` indirection over an
  atomic `tls.Certificate` (`internal/component/web/server.go`): rotation with no rebind.
- Hub: `webTLSMaterial` (fail-closed selection, `service_web.go`), a startup gate in
  `main.go`, and the `TLSUpdatable` / `SetWebTLS` / `UpdateWebCertificate` seam in
  `listener_migrate.go` so always-on code never imports the gated web package.
- Reload: `zepki.Load` moved BEFORE plugin apply, a web-reference gate before that apply,
  and `restorePKI` / `restorePKIAfter` so every failure path reinstalls the prior store.
- dnsserver: `SecureConfig.Certificate`, mutual-exclusion error in `ParseSecureLeaves`,
  a resolver branch in `buildSecureTLS`, and `Options.TLSMaterialResolver`. as112 and
  geodns inject `pki.ServerTLSMaterial`; core never imports component pki.
- Doctor: `doctor-tls-reference` registered in `diagnostic/codes.go`; checks on all
  three surfaces (`web/register.go` + `web/doctor.go`, as112/geodns `doctor.go`).

### Bugs Found/Fixed
- **YANG `length` was never enforced.** `internal/component/config/schema.go` carried
  `Patterns` and `Ranges` but no length, and `yang_schema.go` discarded the parsed
  restriction, so every `length "1..255"` in the tree was decorative: a 256-character
  value validated clean. Ten leaves predating this spec were affected (mrt, geodns,
  ddos/flowtriq, exabgp bridge). Fixed at the source: `LeafNode.Lengths`,
  `lengthRangesFromType`, and `validateLengths` (characters, not bytes, per RFC 7950
  Section 9.4.4). This spec's boundary row depends on it, so it was in scope.
- **The `enabled` gate would have discarded the web certificate.** `cmd/ze/hub/main.go`
  starts the web server from `--web`, `ze.web.listen`, and `ze.web.enabled`, none of
  which consult the config block. Parsing `certificate` behind the block's `enabled`
  leaf would have served a self-signed certificate to an operator who named their own.
  This is the third instance of
  `plan/learned/1327-enabled-gate-discards-service-settings.md`; the extractor is split
  the same way MCP and looking-glass are.

### Documentation Updates
- `docs/guide/configuration.md`: new "TLS Certificates From the PKI Store" section.
- `docs/features/web-interface.md`, `docs/features.md`: PKI-backed TLS rows.

### Deviations from Plan
- **R-6 is void.** `CertificateEntry.Intermediates` is a SLICE now (RFC 7296 Section 3.6
  work), so deeper chains already express. `ServerTLSMaterial` emits every intermediate,
  not one.
- **AC-3 landed as a startup/reload gate, not `ze config validate`.** Cross-root
  reference resolution has no per-leaf validator hook, exactly as the spec's Integration
  Checklist predicted. Verified: `ze start` exits 1 with
  `error: environment.web.certificate: pki: certificate no-such-cert not found (available: lan)`.
- **The reload `.ci` is two files, not one.** Several SIGHUPs in one script race the
  daemon's reload queue (a HUP arriving mid-reload is queued and replayed), which made a
  three-reload script pass or fail on load rather than on behavior. One reload per file
  is deterministic: 5/5 parallel rounds clean.
- **`TestReloadInstallsPKIBeforePluginApply` observes the gate, not a plugin OnConfigure.**
  `pluginserver.Server.ReloadConfig` needs a full `ReactorLifecycle`. The gate runs
  immediately before plugin apply, so where the reload stops is the ordering evidence;
  the `.ci` covers the same AC through the real daemon.
- **`internal/component/web/register.go` is new.** The doctor registration needs
  `os.Exit` on a failed registration, which the hook allowlist permits only in
  `register.go`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Web server can use a PKI-stored certificate | done | `cmd/ze/hub/service_web.go` `webTLSMaterial` | selects store or self-signed |
| The full chain (leaf + intermediates) is served | done | `internal/component/pki/tls.go` `chainPEM` | every stored intermediate, leaf first |
| Same capability on the DoT/DoH listeners | done | `internal/core/dnsserver/secure.go` `buildSecureTLS` | resolver injected by as112/geodns |
| Self-signed fallback semantics unchanged | done | `webTLSMaterial`, `LoadTLSMaterial` | only when NO name is configured |
| Doctor checks for a broken reference | done | `pki.CheckCertReference` + three owner checks | `doctor-tls-reference` |
| Reload / rotation behavior | done | `runReload`, `UpdateWebCertificate` | store first, then rotate |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestServerTLSMaterialAssemblesChain`, `TestWebServerServesPKIChain`, `TestStartWebServerUsesPKIMaterial` | handshake shows 2 PeerCertificates |
| AC-2 | done | `TestWebServerSelfSignedFallbackUnchanged` | persisted pair reused, not regenerated |
| AC-3 | done | `TestStartWebServerFailsClosedOnBrokenReference`, `test/reload/pki-reference-reload-broken.ci`, live `ze start` exit 1 | startup + reload rejection (not `config validate`, see Deviations) |
| AC-4 | done | `TestWebServerServesPKIChain` | real `crypto/tls` client, 2 peer certs, leaf first |
| AC-5 | done | `TestBuildSecureTLSFromResolver`, `test/plugin/as112-dot-pki.ci`, `test/plugin/geodns-dot-pki.ci` | DoT bound with the store certificate |
| AC-6 | done | `TestParseSecureLeavesCertificateConflict`, `test/parse/as112-tls-certificate-conflict.ci` | commit rejected |
| AC-7 | done | `TestServerTLSMaterialNoPrivateKey`, `TestStartWebServerFailsClosedOnBrokenReference` (keyless), `TestBuildSecureTLSResolverFailureIsLoud` | web errors; DoT/DoH leave cleartext up |
| AC-8 | done | `TestCheckCertReferenceDiagnostics`, `TestWebTLSDoctorCheck`, `TestAS112TLSDiagnosticPKIReference`, `TestGeoDNSTLSDiagnosticPKIReference` | missing / keyless / expired / chain mismatch |
| AC-9 | done | `TestWebServerUpdateTLSCertificate`, `TestListenerMigratorUpdateWebCertificate`, `test/reload/pki-reference-reload.ci` | listeners unchanged across rotation |
| AC-10 | done | `TestReloadInstallsPKIBeforePluginApply`, `test/reload/pki-reference-reload.ci` | one commit adds cert + reference |
| AC-11 | done | `TestServerTLSMaterialLeafOnly`, `TestCheckCertReferenceDiagnostics` leaf-only subtest | single block, no diagnostic |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestServerTLSMaterialAssemblesChain` | done | `internal/component/pki/tls_test.go` | |
| `TestServerTLSMaterialLeafOnly` | done | same | |
| `TestServerTLSMaterialNotFound` | done | same | error lists available names |
| `TestServerTLSMaterialNoPrivateKey` | done | same | |
| `TestCheckCertReferenceDiagnostics` | done | same | 9 subtests |
| `TestNewTLSConfigServesChain` | done | `internal/core/selfcert/selfcert_test.go` | A-3 gate |
| `TestWebServerServesPKIChain` | done | `internal/component/web/server_tls_test.go` | |
| `TestWebServerUpdateTLSCertificate` | done | same | |
| `TestExtractWebConfigCertificate` | done | `internal/component/config/web_extract_test.go` | |
| `TestStartWebServerUsesPKIMaterial` | done | `cmd/ze/hub/service_web_tls_test.go` | |
| `TestWebServerSelfSignedFallbackUnchanged` | done | same | |
| `TestParseSecureLeavesCertificate` | done | `internal/core/dnsserver/secure_pki_test.go` | |
| `TestParseSecureLeavesCertificateConflict` | done | same | |
| `TestBuildSecureTLSFromResolver` | done | same | |
| `TestBuildSecureTLSResolverMissing` | done | same | |
| `TestAS112TLSDiagnosticPKIReference` | done | `internal/plugins/as112/doctor_test.go` | |
| `TestGeoDNSTLSDiagnosticPKIReference` | done | `internal/plugins/geodns/doctor_test.go` | |
| `TestGeoDNSAppliesPKICertificate` | changed | covered by `TestBuildSecureTLSFromResolver` + `geodns-dot-pki.ci` | the injection is one struct field on `dnsserver.New`; the resolver BRANCH is where behavior lives and is tested at the seam |
| `TestWebTLSDoctorCheck` | done | `internal/component/web/doctor_test.go` | |
| `TestReloadInstallsPKIBeforePluginApply` | done | `cmd/ze/hub/main_reload_pki_test.go` | see Deviations for the observation point |
| ADDED `TestValidateLeafValueLength` | done | `internal/component/config/leaf_length_test.go` | boundary row; found the length gap |
| ADDED `TestCertificateLeafLengthFromYANG` | done | same | constraint reaches the built schema |
| ADDED `TestExtractWebSettingsSurviveDisabledBlock` | done | `internal/component/config/web_extract_test.go` | learned 1327 |
| ADDED `TestRollbackReloadRestoresPriorPKIStore` | done | `cmd/ze/hub/main_reload_pki_test.go` | R-3 |
| ADDED `TestReloadRejectsBrokenWebCertificateReference` | done | same | R-5 on reload |
| ADDED `TestUpdateTLSCertificateRejectsBadMaterial` | done | `internal/component/web/server_tls_test.go` | fail-closed rotation |
| ADDED `TestBuildSecureTLSResolverFailureIsLoud` | done | `internal/core/dnsserver/secure_pki_test.go` | no self-signed fallback |
| ADDED `TestBuildSecureTLSFileAndSelfSignedUnchanged` | done | same | regression guard |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/pki/tls.go` + `tls_test.go` | created | |
| `internal/component/web/doctor.go` + `doctor_test.go` | created | |
| `internal/component/web/register.go` | created | registration split out (hook allowlist) |
| `cmd/ze/hub/service_web_tls_test.go`, `main_reload_pki_test.go` | created | |
| `internal/component/config/web_extract_test.go`, `leaf_length_test.go` | created | |
| `internal/core/dnsserver/secure_pki_test.go`, `internal/component/web/server_tls_test.go` | created | |
| three `*-conf.yang` modules | modified | `certificate` leaf |
| `loader_extract.go`, `environment.go`, `schema.go`, `yang_schema.go` | modified | extraction split, env var, length enforcement |
| `web/server.go`, `hub/service_web.go`, `main.go`, `main_reload.go`, `listener_migrate.go`, `service_registry.go`, `register_web.go` | modified | |
| `dnsserver/secure.go`, `manager.go`, `as112/{server,doctor,register}.go`, `geodns/{server,doctor,register}.go`, `diagnostic/codes.go` | modified | |
| `test/parse/web-pki-certificate.ci`, `web-pki-certificate-name-too-long.ci`, `as112-tls-certificate-conflict.ci` | created | `-missing` renamed: the reference check is a startup gate, not a parse one |
| `test/plugin/as112-dot-pki.ci`, `geodns-dot-pki.ci` | created | |
| `test/reload/pki-reference-reload.ci`, `pki-reference-reload-broken.ci` | created | split, see Deviations |

### Audit Summary
- **Total items:** 11 ACs, 20 planned tests, 8 added tests, 13 planned files
- **Done:** all 11 ACs; 19 of 20 planned tests as written
- **Partial:** none
- **Skipped:** none
- **Changed:** `TestGeoDNSAppliesPKICertificate` folded into the seam test; AC-3 enforced at startup/reload; reload `.ci` split in two (all in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Web HTTPS serves operator PKI cert + full chain | unit handshake + functional | `TestWebServerServesPKIChain`: real `crypto/tls` client sees 2 `PeerCertificates`, leaf first. `test/parse/web-pki-certificate.ci` PASS. Mutation: dropping the intermediate from `chainPEM` reddens `TestServerTLSMaterialAssemblesChain` |
| DoT/DoH serve PKI cert + full chain on as112 and geodns | unit + functional | `TestBuildSecureTLSFromResolver` asserts `len(Certificates[0].Certificate) == 2` and the resolved leaf CN. `test/plugin/as112-dot-pki.ci` and `geodns-dot-pki.ci` PASS in the 585-test plugin suite |
| Self-signed fallback unchanged | regression | `TestWebServerSelfSignedFallbackUnchanged` (generates, persists, then REUSES). `TestBuildSecureTLSFileAndSelfSignedUnchanged` (resolver never consulted; cached config identical across applies). Full `web`, `as112`, `geodns`, `dnsserver` suites green |
| Rotation without restart | unit + reload functional | `TestWebServerUpdateTLSCertificate`: `Addresses()` identical across the swap, new handshakes serve the new leaf. Mutation: removing the `GetCertificate` indirection reddens it. `test/reload/pki-reference-reload.ci` PASS |
| Broken references caught before/at deploy | doctor + startup + reload | `TestCheckCertReferenceDiagnostics` (9 cases), `TestWebTLSDoctorCheck`. Live daemon: `ze start` exits 1 with `error: environment.web.certificate: pki: certificate no-such-cert not found (available: lan)`. `test/reload/pki-reference-reload-broken.ci` PASS |
| The reference is never silently downgraded to self-signed | negative test at every entry point | `TestStartWebServerFailsClosedOnBrokenReference` asserts the self-signed store was NOT written. `TestBuildSecureTLSResolverFailureIsLoud` asserts `m.selfSigned` stays nil. `TestServerTLSMaterialNotFound` asserts no material accompanies the error |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (N/A expected: no protocol change)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?) -- yes: one pki loader, three consumers (web now, DoT/DoH now, LG later)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-pki-full-chain.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-pki-full-chain.md` only
