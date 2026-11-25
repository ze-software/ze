---
kind: directive
level: MUST NOT
stage:
---
**Secrets, private keys, and local home-directory paths MUST NOT be committed.** API keys, credential files (`.env`, `.netrc`, `.npmrc`, `id_rsa`, `id_ed25519`, `settings.local.json`, `credentials.json`), private keys outside test fixtures, live secret-named environment values, and the committer's own home directory path are refused at commit time by `commit secrets check` (`internal/le/commit/secrets`). A deliberate test fixture carrying a synthetic token or path marks that line with `secret-scan: allow`.
