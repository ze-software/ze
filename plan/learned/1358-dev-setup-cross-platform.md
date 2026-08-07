# 1358 -- dev-setup-cross-platform

## Context

`make ze-setup` installed on macOS and produced homework on Linux: the apt branch
of `install_tool` printed `sudo apt-get install ...` and returned False, so the
run ended at "Run the install commands above" and exit 1. Separately, every
Homebrew path in the repository was written as the Apple Silicon literal
`/opt/homebrew`, which is absent on an Intel Mac, where the prefix is
`/usr/local`. Both halves of "works on Linux and macOS" were broken, in opposite
directions.

## Decisions

- apt INSTALLS rather than printing, matching brew, because a setup script that
  hands back a list of commands is not a setup script. The old printing branch is
  kept as the fallback for a host with no route to root.
- Privilege is decided BEFORE a command runs (`privilege_mode`), and sudo is
  always given `-n`, over calling sudo and letting it prompt. sudo reads its
  prompt from the stdin it inherits, so an agent or CI run with no terminal waits
  forever with no output. A run that prints the command and exits nonzero is
  recoverable; a hung one is not.
- One `run_privileged` for all three root users (apt, the sysctl drop-in, the kvm
  usermod) over three call sites with their own `sudo` argv, because the drop-in
  writer pipes its content on the same stdin sudo would prompt on: `sudo tee` fed
  a config line hands that line to the prompt.
- `uv` installs through `pipx` on BOTH platforms over `brew` on macOS and the
  curl installer on Linux, because Debian has no uv package. Its `apt=None` meant
  `_has_package` said False and the loop printed `[skipped]`. Check mode then
  reported "All required tools present" on a box with no uv.
- An install whose tool the probe still cannot find is reported as pending and
  exits nonzero, over the `[installed] (not yet on PATH)` line that counted it as
  a success. That line ended the run "Setup complete" with exit 0 while `--check`
  on the same box exited 1, so the two modes disagreed permanently and the
  install one was the mode reading green on an unusable tool.
- The two default prefixes are offered on macOS ONLY, over offering them
  everywhere. `/usr/local` is a directory on essentially every Linux host and it
  is not Homebrew's there. `HOMEBREW_PREFIX` and the `brew` binary are still read
  on any platform, so a Linuxbrew install is found by being asked for.
- The GRUB apt package follows the HOST architecture (`grub_apt_package`) over one
  amd64 name. Measured in debian:stable-slim on both arches: an arm64 host answers
  "has no installation candidate" for `grub-efi-amd64-bin` and installs nothing,
  `grub-mkstandalone` included.
- The Homebrew prefix is RESOLVED (`HOMEBREW_PREFIX`, then the `brew` binary's own
  location, then the two documented defaults) over any literal, in all three
  languages: `brewPrefixes` (Go), `brew_prefixes` (Python), `BREW_PREFIX` (Make).
- The `brew` binary is NOT resolved through its symlinks, deliberately. On Intel
  `/usr/local/bin/brew` links into `/usr/local/Homebrew/bin/brew`, so following it
  answers `/usr/local/Homebrew`: the wrong prefix, on exactly the machines the
  resolver exists for.
- `scripts/dev/dev-setup.py` keeps its own copy of `brew_prefixes` rather than
  importing `scripts/evidence/homebrew.py`, because it is what a contributor runs
  against a machine where nothing is installed. The copy's cost is paid by
  `scripts/dev/homebrew_prefix_test.py`, which asserts both answer the same for
  the same machine.

## Consequences

- `make ze-setup` on Debian installs 13 apt packages, 3 go tools and 3 pipx tools
  in one run. Proven in a container as root, as a user with passwordless sudo, and
  as a user whose sudo wants a password with no terminal (prints, does not hang).
- A hardcoded `/opt/homebrew` anywhere under a source suffix now fails
  `scripts/dev/homebrew_prefix_test.py` unless the file also knows about
  `/usr/local` or defers to a resolver.
- `internal/appliance/homebrew.go` is the Go home for anything Homebrew-shaped.
  Add a third path there rather than a fourth literal.

## Gotchas

- **A grep for hardcoded paths with `--include=*.mk` skips the root `Makefile`.**
  It has no extension. The tree-walking guard test found `Makefile`'s tinygo
  `go@1.26` PATH line that the hand search had missed. Extension filters are how a
  sweep reports itself complete while a site sits in the file everything includes.
- **A probe test on a developer's own machine can be satisfied BY that machine.**
  `_probe_e2fsprogs_macos` was tested with `HOMEBREW_PREFIX` set to a temp dir. The
  resolver still offers `/opt/homebrew`, so the probe found the real e2fsprogs and
  answered True whatever the fixture held, and deleting either search branch left
  it green. A mutation run caught it, and the test looked correct at every reading.
  **Injecting the prefix list was not enough.** The probe ends its search at
  `/usr/sbin` and `/sbin`, so the same test is vacuous on any Linux host that has
  e2fsprogs, which is where CI runs it. The fix that holds is asserting over
  `e2fsprogs_dirs`, the list of places it looks, rather than over the boolean.
- **One test seam for two lookups makes two branches indistinguishable.**
  `brewLookPathFn` answers for `brew` and for `qemu-system-aarch64`. The fake
  returned one path whatever it was asked, so both binaries sat at one prefix,
  the beside-the-binary branch and the brew-prefix branch produced the same
  answer, and deleting either kept the test green. The fake now answers per name.
- **A scanner that reads whole files cannot tell a hardcoded path from a sentence
  about one.** The first version of the repo-wide guard accepted any file that also
  mentioned `/usr/local`, and the three files likeliest to grow a new literal are
  the resolvers, which all carry that string already. It now reads code lines only
  (comments stripped, Python docstrings located through the AST rather than by
  counting quote characters, which desynced on this very file) and holds an EXACT
  set of files allowed to spell the prefix, checked in both directions.
- **`/usr/local` exists on every Linux host.** A resolver offering it as a default
  put `/usr/local/sbin` ahead of `/usr/sbin`, and `/usr/local/share/qemu` ahead of
  `/usr/share/OVMF`, on machines that have never seen Homebrew. Two reviewers found
  this independently, and the comments asserting "on Linux this returns nothing"
  were the reason it was easy to miss.
- **A test that pins a MECHANISM is green on the wrong answer.** The Make copy of
  the Cellar search was "fixed" with `ls -dr` and a test asserting that `ls -dr`
  was present. Both passed, and the answer stayed wrong: `-r` reverses the
  SPELLING, so over `{1.47.4, 1.47.9, 1.47.10}` it yields 1.47.9 first and the
  loop breaks two releases behind the other copies. The test now RUNS the
  snippet against a planted Cellar and compares the directory it picks.
  `sort -V -r` is the fix, and it is present on both macOS and Debian.
- **PATH is the wrong question for a tool whose consumers name directories.**
  `probe_tool` asked `shutil.which` for e2fsprogs everywhere but macOS. Homebrew
  links none of a keg-only formula onto PATH, and Debian keeps /usr/sbin off a
  non-root user's PATH, while `E2FS` (mk/gokrazy.mk) and `e2fsSearchDirs`
  (internal/appliance) both name the directories outright. So the probe reported
  missing what the build then used happily, and once "installed but unusable"
  became a nonzero exit, it would have failed such a box on every run.
- **GNU make exits 2 when a recipe fails, whatever the recipe returned.**
  `make ze-setup CHECK=1` reporting "exit 2" is make's code for "Error 1" from the
  script, not a second failure mode.
- e2fsprogs is keg-only: Homebrew links none of it onto PATH, so `which` answers
  nothing however well it is installed. Both `<prefix>/opt/<formula>/sbin` and
  `<prefix>/Cellar/<formula>/<version>/sbin` must be searched; an interrupted
  upgrade leaves the second with no link into it.
- `DEBIAN_FRONTEND=noninteractive` must ride in the argv (`env VAR=val apt-get`),
  not in the caller's environment: sudoers defaults to `env_reset`, so an exported
  value never reaches apt-get.

## Files

- Created: `internal/appliance/homebrew.go`, `internal/appliance/homebrew_test.go`,
  `scripts/evidence/homebrew.py`, `scripts/dev/homebrew_prefix_test.py`
- Modified: `scripts/dev/dev-setup.py`, `scripts/dev/dev_setup_test.py`,
  `internal/appliance/cmd_build.go`, `internal/appliance/cmd_run.go`, `Makefile`,
  `mk/gokrazy.mk`, `scripts/evidence/effective-install-qemu.py`,
  `scripts/evidence/effective-install-iso-qemu.py`, `scripts/evidence/qemu-run.py`,
  `tools/kernel-builder/qemu-build.py`, `docs/guide/developer-setup.md`,
  `docs/guide/ubuntu-build-install.md`
