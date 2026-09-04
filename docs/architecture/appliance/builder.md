# Appliance builder

`ze appliance` is the build-host tooling for gokrazy-based Ze appliance images.
It replaced a retired workflow that exposed credentials on the command line.
`ze appliance build <name>` reads them without putting them in process arguments.

<!-- source: internal/appliance/main.go -- command dispatch, built at call time -->
<!-- source: internal/appliance/config.go -- applianceConfig, Validate, LoadConfig, saveConfig -->
<!-- source: internal/appliance/resolve.go -- ResolveDir and the per-appliance path layout -->
<!-- source: internal/appliance/cmd_init.go -- init wizard, interactive and config-file modes -->
<!-- source: internal/appliance/cmd_assemble.go -- ZeFS assembly with config layering -->
<!-- source: internal/appliance/cmd_build.go -- full image build -->
<!-- source: internal/appliance/manifest.go -- build manifest and image checksums -->

## Secrets at rest

Secrets are encrypted with a key derived by Argon2id and sealed with
XChaCha20-Poly1305. Files are written atomically and keys are zeroed after use.

<!-- source: internal/appliance/crypto.go -- DeriveKey, Encrypt, Decrypt, ZeroBytes, ResolvePassphrase -->

The passphrase agent hands out the KEY over a Unix domain socket, and does not
decrypt on the caller's behalf. The trust boundary is the same UID, so a
key-on-socket protocol is simpler than a decrypt-on-socket one and has no
serialization bottleneck. The socket is created under a restrictive umask and
the agent expires after a bounded duration.

<!-- source: internal/appliance/agent.go -- RunAgent, StopAgent -->
<!-- source: internal/appliance/cmd_unlock.go -- agent lifecycle from the CLI -->

## Decisions

- JSON is kebab-case, matching the CLI rule. The original spec claimed
  snake_case and was wrong.
- The `config-base` path is validated at assemble time, not inside `Validate()`.
  The base directory is resolved at run time, so path validation needs the real
  appliance directory.
- Day-2 operations are separate commands rather than flags on build: `passwd`,
  `cert`, `rekey` (both directions between plaintext and encrypted), and
  `clone`, which copies config and never secrets.
- `cert.pem` and `key.pem` have one write path, `writeTLSSecrets`. Both
  `ze appliance init` and `ze appliance replace-cert` reach the two files
  through it, so both get the same guarantees.
- Each appliance has its own certificate authority, in `ca-cert.pem` and
  `ca-key.pem` beside the material it signs. `ze appliance init` generates the
  root once and every later `ze appliance replace-cert` issues from that same
  root, so a device certificate can change and the trust an operator already
  distributed keeps working. The root key goes through the appliance
  passphrase, exactly as `key.pem` does, and `ze appliance rekey` re-encrypts
  both keys.
- `cert.pem` holds the device certificate FIRST and the root SECOND. Four
  readers take the first block only: `certExpiry`, `validateTLSPair`,
  `checkCertExpiry` and `selfcert.NewTLSConfig` on the device itself. The
  serving certificate is the answer each of them wants. `loadDeviceTLS` reads
  the whole file and trusts the root it finds there (`ota-push.md`).
- Operator-supplied material (`--cert` and `--key`) is stored as it arrives. An
  appliance given a certificate from an external authority grows no local one.
- The root lives one year longer than the certificate it signs, because a chain
  stops verifying the moment its issuer expires. `tls.validity-years` stays the
  life of the SERVING certificate, which is what an operator sets it for.
- The material is validated before either file is touched. The certificate and
  the key must parse, and they must be a pair. This is the check the web server
  makes at boot, so material the command accepts is material the listener
  accepts. An expired certificate is refused. A certificate whose validity
  starts in the future is accepted, because a staged renewal is copied into an
  image that boots later.
- Both halves are written with `WriteSecret`, which writes a temp file and
  renames it. An interrupted run leaves neither file truncated. The certificate
  is written first, and a failed key write puts the previous certificate back.
  The error names the write that failed and the outcome of the restore.

<!-- source: internal/appliance/cmd_passwd.go -- password rotation -->
<!-- source: internal/appliance/cmd_cert.go -- TLS certificate replacement, validateTLSPair, writeTLSPair -->
<!-- source: internal/appliance/ca.go -- applianceRootStore, issueWebLeaf -->
<!-- source: internal/appliance/cmd_rekey.go -- passphrase change -->
<!-- source: internal/appliance/cmd_clone.go -- config-only clone -->
<!-- source: internal/appliance/cmd_list.go -- fleet listing -->
<!-- source: internal/appliance/cmd_show.go -- config and certificate expiry -->
<!-- source: internal/appliance/cmd_run.go -- QEMU boot with port conflict detection -->
<!-- source: internal/appliance/homebrew.go -- Homebrew prefix resolution on a macOS build host -->

## Constraint the code does not state

The dispatch table is built when it is called, not at package load. The
`cmd_*.go` files install their handlers by assigning package-level variables in
`init()`, so the map must be built after every `init()` has run. A handler added
without its variable declared in `main.go` fails to compile, and a variable
declared without a handler map entry trips the unused-variable lint.

## Related

- `remote-operations.md` for push, config preview, and fleet operations
- `disaster-recovery.md` for export and import of a bastion
- `device-config.md` for what the device does with a pushed config
- `command-provider.md` for how this surface registers itself
