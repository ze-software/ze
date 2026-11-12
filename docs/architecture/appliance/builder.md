# Appliance builder

The packer terminates both boot command-line files with a newline after their
reserved whitespace. The runtime's testboot edit inserts its inactive-root flag
at that newline. Without it, the update endpoint can accept testboot while the
next kernel receives no testboot flag and starts the original root again.
<!-- source: vendor/github.com/gokrazy/tools/internal/packer/write.go -- writeCmdline -->
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go -- enableTestboot -->

The runtime consumes the reserved trailing whitespace when it adds testboot.
Repeated requests keep one flag. In-place boot-file edits retain the FAT file
length, pad only the final line, and refuse an edit that exceeds that extent
before writing. Loader titles and kernel filenames remain unchanged.
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go -- testbootCmdline, modifyCmdlineFile -->


`ze appliance` is the build-host tooling for gokrazy-based Ze appliance images.
It replaced a retired workflow that exposed credentials on the command line.
`ze appliance build <name>` reads them without putting them in process arguments.

The build host is Linux or macOS. gokrazy's packer reads block device sizes
only on those two systems, so on any other host `ze appliance build` and
`le build gokrazy` refuse with that reason rather than failing to compile.

A named image build reads its existing `update.token` and TLS certificate/key.
It puts the token into the prepared gokrazy instance's `Update` object and
sets `Update.UseTLS` to private temporary certificate/key paths. The device
therefore uses the same credentials and HTTPS port as `ze appliance push`.
The prepared configuration and TLS files use mode `0600`. Normal builder
return, including failure, removes the prepared instance and TLS files. The tracked
instance configuration is unchanged. These settings are for the named appliance
build; the lower-level gokrazy development builder retains its supplied instance
configuration.
<!-- source: internal/appliance/build_update.go -- readGokUpdate, writeGokUpdate -->
<!-- source: internal/appliance/cmd_build.go -- buildOne, runGokBuild -->

The updater token stays unchanged in the image configuration. URL construction
escapes it as user information, so reserved characters cannot change the
destination. Named-build errors and the connection hint redact the token.
<!-- source: vendor/github.com/gokrazy/internal/updateflag/updateflag.go -- Value.BaseURL -->
<!-- source: internal/appliance/build_update.go -- gokBuildFailure -->
<!-- source: vendor/github.com/gokrazy/tools/internal/packer/packerwrite.go -- Pack.printHowToInteract -->

The embedded gokrazy host packer returns errors through `gok.Context.Execute`.
Ze carries that change as the stamped `gokrazy-packer-return-errors.patch`,
including ELF validation and the privileged partition-helper path. An ordinary
packing failure therefore reaches deferred working-directory restoration and
prepared-credential cleanup. The cleanup regression invokes the actual packer
in a child test process and checks cleanup before the test framework can remove
its temporary directory.
<!-- source: internal/appliance/gok_execute.go -- ExecuteGok, keepWorkingDir -->
<!-- source: internal/appliance/gok_failure_cleanup_test.go -- TestGokPackingFailureReturnsAndCleansIdentity -->
<!-- source: internal/le/vendorpatch/patches/gokrazy-packer-return-errors.patch -- embedded packer error boundary -->

A successful privileged partitioning helper returns a distinct typed
completion through the public embedded API. It is not a completed image.
`runGokBuild`, `buildOne` and `buildAll` preserve that distinction, so helper
completion cannot trigger manifest generation, persistent-partition injection
or another batch build. Only `runBuild` converts it to process exit success,
after the cleanup owners have returned. The `le build gokrazy` command applies
the same rule at its boundary; a VM build propagates the result before QEMU can
start.
<!-- source: internal/appliance/cmd_build.go -- buildOutcome, buildCommandExit, runGokBuild, buildOne, buildAll -->
<!-- source: internal/appliance/gok_execute.go -- IsGokHelperCompletion -->
<!-- source: internal/le/build/gokrazy/gokrazy.go -- Run -->
<!-- source: vendor/github.com/gokrazy/tools/gok/helper.go -- HelperCompletion, IsHelperCompletion -->

gok's go subprocesses compile the image's ze, which links the EAP-TLS
authenticator and compiles only against Ze's patched `crypto/tls`. So
`SetGokGoEnv` first refuses, before it sets `GOMODCACHE`, `GOPROXY` or `GOFLAGS`,
unless the environment already selects the patched GOROOT: `GOROOT` set,
`GOTOOLCHAIN=local`, the first `go` on `PATH` being `$GOROOT/bin/go`, and the
marker file the patch adds present under `$GOROOT`. It sets no `GOTOOLCHAIN`
itself, so gok inherits all three from the caller. `le build gokrazy` passes
because le selects the tree at startup; `ze appliance build` ships in the
ze-setup binary, which links no `internal/le` package, so its refusal names the
command line that selects the tree
(`docs/contributing/patching-the-standard-library.md`).
<!-- source: internal/appliance/cmd_build.go -- SetGokGoEnv, requirePatchedTree -->

gok's commands change into the instance directory and never change back, and
`le build gokrazy` builds from a prepared instance it removes afterwards. Every
gok run goes through `ExecuteGok`, which returns the process to the directory it
started in, so a caller that keeps working after the build (`./le setup
docker host start` runs Colima next) never hands its children a removed working
directory.
<!-- source: internal/appliance/gok_execute.go -- ExecuteGok, keepWorkingDir -->

The runtime-kernel and `le build gokrazy` library entry points take the selected
source root explicitly. Docker-host image construction carries that same root
through its initial kernel/cache-key lookup and its nested gok preparation,
including default module-cache and toolchain inputs. A library call does not
rediscover another checkout from the caller's current directory. The registered
le command discovers the root before entering that chain. Explicit relative
`--parent_dir` and `GOMODCACHE` operands still refer to the invocation directory.
<!-- source: internal/le/build/gokrazy/register.go -- Answer -->
<!-- source: internal/le/build/gokrazy/gokrazy.go -- Run, prepareArgs -->
<!-- source: internal/le/setup/docker/host/build.go -- builder.build, runGok, runGokWith -->

The named appliance's `image.kernel-profile` selects its installer kernel.
The disk-image build resolves the shared `runtime` profile for `image.arch`,
so selecting the installer `qemu` profile leaves the runtime driver set
unchanged. Both kernel purposes start from Linux `defconfig` and merge their
resolved fragments; a fragment's driver requests describe additions to that
baseline. The emitted configuration records the resulting driver set.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->
<!-- source: internal/appliance/kernelargs.go -- resolveBuildParentDir -->
<!-- source: internal/appliance/runtimekernel.go -- RuntimeKernelTree -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

The appliance CLI resolves its base directory to an absolute path once, before
dispatch. This pins relative `--dir`, `ZE_APPLIANCE_DIR` and XDG-default paths to
the invocation's working directory even while gok temporarily changes it.
If the relative path cannot be resolved because the working directory is gone,
the CLI refuses before dispatch and asks for an existing working directory or an
absolute `--dir`; it does not guess another appliance directory.
<!-- source: internal/appliance/main.go -- Run, baseDir -->

<!-- source: internal/appliance/main.go -- command dispatch, built at call time -->
<!-- source: internal/appliance/gok_execute.go -- ExecuteGok, the one gok entry point -->
<!-- source: internal/appliance/gok_execute_other.go -- the refusal on any other host -->
<!-- source: internal/appliance/config.go -- applianceConfig, Validate, LoadConfig, saveConfig -->
<!-- source: internal/appliance/resolve.go -- ResolveDir and the per-appliance path layout -->
<!-- source: internal/appliance/cmd_init.go -- init wizard, interactive and config-file modes -->
<!-- source: internal/appliance/cmd_assemble.go -- ZeFS assembly with config layering -->
<!-- source: internal/appliance/cmd_build.go -- full image build -->
<!-- source: internal/appliance/manifest.go -- build manifest and image checksums -->
<!-- source: internal/appliance/kernelargs.go -- the one kernel-cmdline assembly seam -->

Image injection takes an explicit context and command executor through
formatting, database writes and verification. The native appliance proof supplies
its existing checked process-group executor, so cancellation during `mkfs.ext4`,
`debugfs`, `e2fsck` or mount verification joins the command before returning and
starts no later phase. Every injection command retains the existing ten-minute
bound. The ordinary appliance build supplies its existing executor at its own
root; this does not establish a new signal root for that separate command.
The appliance package imports no development-tool runner to achieve this.
<!-- source: internal/appliance/cmd_build.go -- InjectDatabase, injectZeFS, runExternal -->
<!-- source: internal/appliance/inject_command.go -- runInjectionCommand -->
<!-- source: internal/le/test/deployment/gokrazyimage.go -- runNativeImageCommand -->

## Kernel command line

`image.hugepages`, `image.isolated-cpus` and `image.crash-dump` are the three
appliance-config fields that reach the built image's `/cmdline.txt`. Each is a
pure function of the config in `kernelargs.go`, and each answers nil when its
field is unset, which leaves the cmdline unchanged. `resolveBuildParentDir`
concatenates them at the one seam and hands the result to `instance.Prepare`.

`image.crash-dump.reserve` renders two tokens, and both are required:
`reserve_mem=<N>M:4096:zecrash` carves a named region and
`ramoops.mem_name=zecrash` binds the kernel's pstore backend to it. The region
name is `crashlog.ReserveRegionName`, which the daemon reads back off
`/proc/cmdline` to answer whether the running kernel is armed.
`docs/guide/appliance.md` carries the operator half.

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
- Generated serving certificates include `device.address` as an additional DNS
  or IP subject alternative name. Push verifies the configured destination with
  the normal TLS hostname check, including when it uses a numeric address.
  After changing the destination of an existing appliance, use
  `ze appliance replace-cert <name>` and rebuild before deploying that material.
  <!-- source: internal/appliance/ca.go -- issueWebLeaf -->
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
