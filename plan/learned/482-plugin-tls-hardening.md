# 482 -- Plugin TLS Hardening

## Context

External plugins all shared a single auth token, meaning any plugin could impersonate any other by sending a different name during the auth handshake. The SDK used `InsecureSkipVerify` for TLS, accepting any certificate without verification. The token sat in the process environment for the plugin's entire lifetime, visible via `/proc/<pid>/environ`. These three weaknesses were addressed together.

## Decisions

- Chose per-plugin token generation in `PluginAcceptor.TokenForPlugin()` over modifying `HubServerConfig` to hold per-plugin secrets, because tokens are ephemeral (regenerated each engine restart) and don't need to be in config.
- Used `combinedLookup` in `handleConn` (wrapping `AuthenticateWithLookup`) over adding a new `AuthenticateWithNameAndToken` path, because the existing lookup-based auth already provides natural name binding: the lookup is by the name the client claims, so impersonation attempts get the wrong expected token.
- Added `Secret bool` to `EnvEntry` over calling `os.Unsetenv` in `NewFromTLSEnv` directly, because the mechanism applies to any future sensitive env var without per-callsite code.
- Used `tls.Config.VerifyConnection` callback for cert fingerprint verification over custom `VerifyPeerCertificate`, because `VerifyConnection` receives parsed certificates and runs after the TLS handshake completes but before application data.
- Kept `InsecureSkipVerify: true` even with fingerprint pinning, because Go's TLS requires it to skip chain validation when using self-signed certs; `VerifyConnection` provides the actual verification.

## Consequences

- Plugin impersonation is blocked: each plugin gets a unique token, and the engine validates the name matches the token.
- MITM on non-localhost is blocked when `ZE_PLUGIN_CERT_FP` is set (always set for engine-forked plugins).
- Token exposure window is minimal: cleared from OS env after first SDK read.
- `NewPluginAcceptor` signature changed to require `certFP` parameter -- all callers must be updated.
- `AuthenticateWithName` is available as a public API for callers who want explicit name binding without the acceptor.

## Gotchas

- `secretCleared` map in `env.go` must be protected by `cacheMu` -- initial implementation had a data race (caught by deep review agent).
- Fingerprint mismatch error originally included the actual cert fingerprint, leaking it to attackers via plugin stderr relay -- removed from error message.
- `TLSConfigWithFingerprint` must `strings.TrimSpace` the fingerprint before checking emptiness, or whitespace-only values bypass pinning.
- `handleConn` now always uses `AuthenticateWithLookup` (previously used `Authenticate` when no lookup was set). Functionally equivalent because lookup returning false falls back to shared secret.

## Files

- `internal/component/plugin/ipc/tls.go` -- `AuthenticateWithName`, `CertFingerprint`, `TLSConfigWithFingerprint`, `TokenForPlugin`, `CertFP`, `combinedLookup`; `PluginAcceptor` extended with per-plugin tokens and cert fingerprint
- `internal/core/env/registry.go` -- `Secret bool` on `EnvEntry`, `IsSecret()`
- `internal/core/env/env.go` -- `Get()` auto-clears `Secret` vars from OS env; `clearSecretFromEnv`
- `pkg/plugin/sdk/sdk.go` -- token registration `Secret: true`; `ze.plugin.cert.fp` registered; `NewFromTLSEnv` uses `TLSConfigWithFingerprint`
- `internal/component/plugin/process/process.go` -- `startExternal` uses `TokenForPlugin` + passes `ZE_PLUGIN_CERT_FP`
- `internal/component/plugin/manager/manager.go` -- passes `CertFingerprint(cert)` to `NewPluginAcceptor`
