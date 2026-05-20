# 733 -- PKI Certificate Store

## Context

Ze had no PKI infrastructure. IPsec VPN (spec-ipsec-0) needs X.509 certificate authentication, and TLS/SSH could also benefit from a shared certificate store. The production config at `../home.conf` uses a `pki {}` block with CA certificates and device certificates (ECDSA, base64-encoded DER, `$9$` private keys) issued by the SurfProtect CPE Management CA. The goal was to build shared PKI infrastructure that IPsec, TLS, and future mutual-auth features can consume.

## Decisions

- Chose base64-encoded DER over PEM for YANG leaves because the VyOS convention stores raw DER in config; PEM headers add no value inside a YANG leaf
- Chose multi-format key detection (PKCS8 then SEC1 then PKCS1) over requiring a specific format because real-world keys come in all three encodings depending on the tool that generated them
- Chose atomic pointer swap over mutex-guarded store because readers (show commands, IPsec consumer) never block writers (config reload)
- Chose to validate chain and expiry at Load time over lazy validation because failing early at config load produces better error messages and prevents the daemon from running with broken certificates
- Chose PEM export to `/tmp/ze-ipsec/` over passing DER directly to strongSwan because strongSwan's `swanctl.conf` expects PEM file paths, not inline certificate data
- Chose a standalone `pki` component over embedding in `ipsec` because PKI is shared infrastructure (web TLS, SSH host certs, managed device auth can all use it)

## Consequences

- The IKE engine (spec-ipsec-7) calls `pki.Load()` after config parse, then uses `pki.CAPool()`, `pki.GetCertificate()` for X.509 authentication. `pki.ExportPEM()` is available if PEM files are needed on disk
- The `pki` YANG module is registered via schema init() and blank-imported in hub; no explicit wiring code needed in `main.go`
- Show commands are RPC handlers in the `pki` package, not in `cmd/show/`; this keeps the pki package self-contained
- `ze:sensitive` on the private key leaf means the config parser auto-decodes `$9$` before the PKI parser sees it; the PKI parser receives plaintext base64
- Health check registered as "pki": reports degraded when any cert expires within 30 days, down when expired. Checked on demand by health registry
- Report bus warnings raised via `pki.RaiseExpiryWarnings()` after each `Load()`: warns at 30 days, marks expired certs. Warnings clear automatically when certs are renewed

## Gotchas

- `config.Tree` stores YANG lists via `AddListEntry`/`GetList`, not via nested containers. Tests that use `GetOrCreateContainer` to build list entries produce trees where `GetList` returns nil. Always use `AddListEntry` in test tree construction.
- The `keySize` function needs explicit type switches for `*rsa.PublicKey`, `*ecdsa.PublicKey`, and `ed25519.PublicKey`; a generic `Size()` interface only works for RSA. ECDSA uses `Curve.Params().BitSize`.
- Ed25519 private keys in Go are 64-byte seeds. `ed25519.PrivateKey[32:]` is the public key portion. Do not call `.Public().(ed25519.PublicKey)` for comparison; use the slice directly.

## Files

- `internal/component/pki/types.go` -- CACertEntry, CertificateEntry, PKIConfig, CertSummary, CertDetail
- `internal/component/pki/config.go` -- ParseConfig, parseCACert, parseDeviceCert, parsePrivateKey, verifyKeyMatchesCert
- `internal/component/pki/store.go` -- Load, GetCA, GetCertificate, CertCN, CAPool, IntermediatePool, ExportPEM, CleanupPEM
- `internal/component/pki/show.go` -- handleShowPKICertificates, handleShowPKICertificate
- `internal/component/pki/register.go` -- blank-import of schema package
- `internal/component/pki/schema/ze-pki-conf.yang` -- pki {} YANG module
- `internal/component/pki/schema/ze-pki-api.yang` -- show pki RPCs
- `internal/component/pki/schema/embed.go` -- go:embed for YANG files
- `internal/component/pki/schema/register.go` -- yang.RegisterModule init()
- `internal/component/pki/health.go` -- health check (30d warn, 7d critical, expired=down) and report bus expiry warnings
- `internal/component/pki/health_test.go` -- 7 health/report tests
- `internal/component/pki/config_test.go` -- 10 config parser tests
- `internal/component/pki/store_test.go` -- 20 store + show tests
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- show pki dispatch entries
- `cmd/ze/hub/main.go` -- blank-import of pki package
- `docs/features.md` -- PKI feature entry
- `docs/guide/command-reference.md` -- show pki commands
- `docs/guide/configuration.md` -- pki {} config syntax
