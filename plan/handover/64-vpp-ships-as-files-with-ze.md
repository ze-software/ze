# Handover: VPP ships as files with Ze (design reopened 2026-11-28)

Written from the Mac session that reopened `plan/release/delivery/spec-vpp-embedded-runtime.md`.
The owner ordered the work moved to this server's main checkout. The three edited
files were applied here by patch, byte-identical to the Mac copies, uncommitted.

## State of the tree

| File | State |
|------|-------|
| `plan/release/delivery/spec-vpp-embedded-runtime.md` | Rewritten whole for the files-in-a-Ze-folder design. Status `design`. `./le spec design check` printed only "no judge has judged this design" on the Mac |
| `docs/research/vpp-deployment-reference.md` | One row appended to `## Design decisions`, dated 2026-11-28. The owner has not yet been shown its wording |
| `docs/guide/vpp.md` | Sidecar bullet removed; new subsection "Where the VPP program comes from" |

Nothing is committed. No spec is claimed. The file name of the spec stays, about ten specs cite it.

## Owner decisions of 2026-11-28 (voice-transcribed, read for meaning)

1. "on the appliance we should manage the VPP binary. Can we generate it and add it to the file system ?"
2. "So we should always install our VPP with the software."
3. "VPP bundle placed in the image's file system"
4. "for linux it should be installed in a ze folder for binaries following the LSB"
5. "ok for having a .deb for installation too follow the usal debian packaging method in repos"
6. "One package" and "you can do rpm too"
7. "install following the FSB on a linux FS (probably /usr/local/) and then add a script in /usr/local/bin so we can have multiple version installed and change based on a env variable"
8. "same for vpp install it in the same folder as ze and make sure ze will scan that folder"
9. "old version are kept the ze script providing an uninstall option a bit like we do with ./le"
10. Approved ("lgtm") as an operator-facing shape: `ze version list` shows installed versions and the selected one; `ze version uninstall 26.11.2` removes that version's directory, ze and VPP together.
11. "we want to do this work in the main tree on the claude linux server"

Decisions 7, 9 and 10 (the /usr/local versioned install, the launcher, kept versions, list and uninstall) are NOT recorded in any page or spec yet.

## Open questions for the owner, one at a time, each with an example

| Id | Question | Recommendation |
|----|----------|----------------|
| C-1 | Is a VPP at /usr/bin/vpp ever started when Ze finds none in its folder? Asked on 2026-11-28, not yet answered | No: the 2026-11-19 fallback is withdrawn, VPP stays down, doctor gives the fix |
| C-2 | Where is the Ze folder? Debian Policy forbids subdirectories in /usr/bin, so "the same folder as ze" cannot be literal on a package install | ze stays at <prefix>/bin/ze, folder <prefix>/lib/ze |
| C-3 | What does an in-place self-updated Ze do about VPP until version directories replace that updater? | Nothing new: the start check refuses the old bundle |
| C-4 | Which VPP is ours? Ze holds two patched VPPs: v25.10 with the PPPoE series (`third_party/vpp-patches/`) and v26.06 with the IPsec patch (`third_party/vpp-patches-v26.06/`). No one build carries both | v26.06 for the first release |
| C-5 | Does every image, test images included, carry VPP? | Yes, cached like the kernel |
| L-1 | Which version runs when the variable is unset | A current link that install, update and rollback move |
| L-2 | What the unit from `ze install systemd` executes | The launcher |
| L-3 | Whether a DEB or RPM install uses the launcher | No, ze runs directly |
| L-4 | Uninstalling the running or selected version | Refuse, with the command that works |
| L-5 | Uninstalling the last version | Remove the launcher too |
| L-6 | Command spelling: `version` is already a root handler, and `ai/rules/cli.md` wants a keyword before a value and `list` for enumeration. Read the rule whole before putting a respelling to him | - |
| L-7 | Whether "usual Debian method" means debhelper in place of nFPM, which `spec-release-build.md` uses | - |

## Conflicts found with what the owner was told

- "In repos": `spec-release-build.md` says no APT or DNF repository in that spec; repositories are post-release. The first release ships a downloaded .deb.
- The package-managed update backend that refuses self-update does not exist in code, only in the two release specs.
- A natively built C bundle can enter the release build only as a digest-pinned cached input: `spec-release-build.md` requires byte-identical artifacts from a macOS host and Linux CI.
- With one directory per version under /usr/local the bundle path varies, and an ELF file cannot name its loader by a relative path, so the bundled loader is run directly with a library directory.

## Verified at the producer

- `internal/component/vpp/register.go` `runConfiguredVPP` passes the constants `defaultVPPBinary` and `defaultConfDir`; `vpp.go` `runOnce` runs that path with `-c`.
- gokrazy packer (vendored, used in process through `internal/appliance/gok_execute.go`): `extraFilesArchiveCandidates` tries P_<GOARCH>.tar.zst, P_<GOARCH>.tar, P.tar.zst, P.tar, then directory P. An archive keeps modes and symlinks. A directory keeps exec bits and FOLLOWS symlinks. An empty regular file or a hard link in an archive becomes an empty directory. A destination under `perm` is refused.
- Partition table: boot 100 MB, two roots of 500 MB each, /perm after. No size check when the root is written. Current root use not measured.
- gokrazy puts ze at /user/ze, so the bundle cannot be assumed beside it: the design tells the appliance its Ze folder through the `Environment` list of `gokrazy/ze/config.json`.
- `internal/appliance/instance/kernelpkg.go` `addLinuxNotice` rewrites only `ExtraFileContents`; placing a directory or archive is new wiring.
- `selfupdate.go` `resolveTarget` uses os.Executable then EvalSymlinks; `stageBinary` renames one file.
- `internal/plugins/local/native.go` `packageOwnership` finds the install marker only when the binary's directory is named bin: a hazard if the binary moves.

## Not verified

- What the plugin server does with a failed managed start (the spec's assumption A-14). Trace it before trusting AC-4 and AC-5.
- How extra-file roots merge into the image root (A-5), and whether gokrazy reacts to extra files under /user or /usr/lib/ze (A-13).
- The Debian Policy section numbers written in the spec come from the rendered page: check them. The LSB text was not read.
- The `./le` script was only grepped; its mapping to the ze launcher is not done.

## Remaining work

1. Show the owner the decision row wording, then an independent `ze-design-judge` run, then his approval from his terminal.
2. The /usr/local versioned-install design (launcher, version variable, list, uninstall, self-update installing a version directory), homed in `plan/release/delivery/spec-release-build.md`. Read first: the `le` script, `docs/contributing/running-commands.md`, `ai/rules/cli.md`, `ai/patterns/cli-command.md`, the `version` root handler in `cmd/ze/ze_core_dispatch.go`, `selfupdate.go`, `docs/architecture/appliance/self-update.md`, `plan/release/fixes/spec-selfupdate-manifest-authenticity.md`.
3. Amendments owed and not made: `spec-release-build.md` AC-14, its Artifacts row and A-1; `spec-release-distribution.md` payload rule and AC-10, plus the LGPL source archive; `spec-vpp-failure-lifecycle.md` R-2, its "Embedded runtime start" row and its `ze.vpp.binary` notes; `spec-pppoe-vpp-dataplane.md` per C-4.
4. Seven specs quote the 2026-11-23 decision row and owe a verdict on the new row at their next edit: `spec-config-bound-authority`, `spec-bgp-netns`, `spec-vpp-reply-deadline-iface-fib-static`, `spec-fault-investigation-0-umbrella`, `spec-lifecycle-invariants`, `spec-vpp-interface-in-use`, `spec-vpp-failure-lifecycle`.
5. Package size before and after is an estimate (120 MB development ze, about 63 MB bundle without DPDK): the first phase measures it.
6. `plan/release/roadmap.md` still carries the old release reason: regenerate it with the roadmap tool.

## Owed commands

- `./le doc check all`: anchors and links after the two docs edits.
- `./le doc consistency`: the new design-decisions row is well formed.
- `./le spec design check spec plan/release/delivery/spec-vpp-embedded-runtime.md`: expect only the missing-judge line.
