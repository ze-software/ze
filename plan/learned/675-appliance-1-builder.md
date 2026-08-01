# 675: appliance-1-builder

Build-time tooling for gokrazy-based Ze appliance images.

## What Was Built

`cmd/ze/appliance/` package (16 files) replacing the `make ze-gokrazy USER=x PASS=y` workflow:

- **Config types** (`config.go`): `ApplianceConfig` struct with kebab-case JSON, validation (name length, ports, arch, image size, cert validity), dir resolution (--dir > env > XDG).
- **Crypto** (`crypto.go`): Argon2id KDF + XChaCha20-Poly1305 AEAD for secrets at rest, atomic file writes, key zeroing.
- **Passphrase agent** (`agent.go`, `cmd_unlock.go`): key-on-socket protocol via Unix domain socket, umask-protected creation, duration-limited auto-expiry.
- **Init wizard** (`cmd_init.go`): interactive + config-file modes, bcrypt password hashing, self-signed TLS cert with configurable validity, update token generation.
- **Assemble** (`cmd_assemble.go`): ZeFS database from config + encrypted secrets, config layering (base + overlay), auto-delete of database.zefs.
- **Day-2 ops**: passwd, replace-cert, rekey (plaintext-to-encrypted and reverse), clone (config only).
- **List/Show** (`cmd_list.go`, `cmd_show.go`): tabwriter columns, cert expiry display, managed mode explanation.
- **Build/Run**: manifest generation, SHA-256 checksums, port conflict detection. gok/ext4/QEMU invocation are stubs pending binary availability.

Also: added `validity` parameter to `GenerateWebCertWithNames`, added `KeySSHAuthorizedKeys` and `KeyInstanceAdminDisabled` to ZeFS key registry.

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Key-on-socket agent protocol | Same-UID trust boundary; simpler than decrypt-on-socket; no serialization bottleneck |
| Kebab-case JSON (not snake_case) | Project rule `json-format.md`; spec's snake_case claim was wrong |
| config-base validation at assemble time, not Validate() | Base dir is resolved at runtime; path validation needs the actual appliance dir |
| Spec split into 4 specs | 74 ACs was 3-4x typical; builder (45 ACs), remote (18), recovery (6), device-config (5) |

## What Remains

- gok invocation + ext4 inject (requires external binary)
- QEMU launch (requires qemu binary)
- Functional `.ci` tests
- Specs: appliance-2-remote, appliance-3-recovery, appliance-4-device-config

## Files

None recorded.
