# Patching the Go Standard Library

How Ze changes one file of a Go standard-library package at build time,
without a fork of the toolchain and without a copy of the package in the tree.

**Status: designed, not built.** Nothing on this page exists in the tree yet.
`plan/immediate/spec-ike-eap-rfc-defects.md` ("D2 design") builds it. Until that
spec lands, every build uses the unmodified standard library. When it lands, this
page is restated as a description of the built mechanism.

## Why Ze needs it

RFC 5216 Section 2.1.1 says: "If the peer's sessionId is null or unrecognized by
the server, the server MUST choose the sessionId to establish a new session."
Ze's EAP-TLS authenticator runs on `crypto/tls`. In go1.27.2, `crypto/tls` sets
the ServerHello session_id only when it resumes a session, where it echoes the
client's id. A TLS 1.2 full handshake therefore sends an empty session_id, and
the server chooses no sessionId.
<!-- source: internal/core/eap/eap_tls.go -- newTLSMethod -->

The public API of `crypto/tls` has no hook for the ServerHello session_id. The
value is a field of an unexported struct, set inside an unexported method. Only
a change to the package source can set it. The owner chose that route on
2026-10-14, pointing to Vincent Bernat's article
(<https://vincent.bernat.ch/en/blog/2026-go-netip-addrto6>) as the way to do it.

## The idea, and where Ze departs from the article

The article changes the standard library for one build with `go build -overlay`:
a JSON file maps a GOROOT source file to a patched copy, and the go command
compiles the copy in its place. Ze keeps the idea: the change is a diff applied
at build time to the toolchain in use, with no fork and no copy in the tree, and
a Go release that moves the patched code makes the build refuse.

Ze does not keep the delivery. An overlay is read by the go command only.
Tools built on `golang.org/x/tools/go/packages` ask `go list` for the file names
and then open each file from disk. Ze runs two of them as gates
(golangci-lint through `./le go lint`, govulncheck through
`./le verify deps vulnerability`). Probed with go1.27.2 on 2026-10-14, with the
overlay in `GOFLAGS`:

| Tool | Result |
|------|--------|
| `go vet`, `go build`, `go list -compiled` | the overlay is read |
| golangci-lint 2.14.0 | `open /usr/local/go/src/crypto/tls/zz_ze_server_session_id.go: no such file or directory (typecheck)` |
| staticcheck | the same error |
| govulncheck | the same error, while loading packages |

Moving the patch's new constant into the replaced file does not help: these
tools read the unpatched copy on disk, where it is not defined either. So Ze
puts the patched files on disk, in a GOROOT of its own.

## The mechanism: a patched GOROOT

The generator builds a directory that is the toolchain's GOROOT with the patched
files changed, and every Go command that compiles Ze runs with `GOROOT` set to
it. Every tool reads that directory as it reads any GOROOT.

The directory is made of links, so it costs little:

- Every top-level entry other than `src` (`bin`, `pkg`, `lib`, `VERSION`, and the
  rest) is a symlink to the real one. The tree runs the real go binary and the
  real prebuilt tools.
- Under `src`, every directory is a real directory and every file is a symlink
  to the real file. A symlinked directory would not do: the go command does not
  follow one when it expands `std` or `...`, and a tree that linked whole
  directories answered `go list std` with 2 packages instead of 383.
- Some files are real copies. The go command refuses to embed a symlink
  ("cannot embed irregular file"), so every file a `//go:embed` pattern in `std`
  or `cmd` names is copied (44 files, 2.9 MiB on go1.27.2). `cmd` counts because
  the go command builds some of its tools from `src/cmd` on demand. The patched
  file and the file the patch adds are real too.

On go1.27.2 the tree is 30 MiB. Symlinks are used rather than hard links, because
Linux refuses a hard link to a root-owned GOROOT file that the user does not own.

## The layout

| Part | Location | Role |
|------|----------|------|
| The patch | `internal/le/go/stdpatch/patches/<name>.patch` | A git unified diff with paths relative to GOROOT (`a/src/crypto/tls/handshake_server.go`). Its leading text names this page and quotes the requirement |
| The generator | `internal/le/go/stdpatch` | Builds the patched GOROOT for one toolchain. It imports only the standard library, so it compiles under the unpatched GOROOT, before any tree exists | <!-- doc-links: ignore (designed, not built: spec-ike-eap-rfc-defects creates this package) -->
| The output | `cache/stdpatch/<key>/goroot` | The tree, keyed by a SHA-256 over a layout version, the `go version`, the real GOROOT path, the patch, and each original GOROOT file. The path is in the key because every symlink names an absolute path into that GOROOT and `cache/` is shared by every checkout |
| The command | `./le go stdpatch goroot` | Prints the tree path for this checkout's toolchain, for plain `go` commands and editors |

The patch is stored as a diff, not as a patched copy of the file. A copy belongs
to one Go version, so a toolchain bump would silently compile old upstream code.
A diff applies to the GOROOT file of the toolchain in use, or refuses.

The generator does this, each time:

1. Find the real GOROOT with `go env GOROOT`, under the same environment as the
   build, with the toolchain pin from `go.mod`. When the pin names another
   version than the go on PATH, the go command runs the pinned toolchain, whose
   GOROOT is under `GOMODCACHE`. An inherited `GOROOT` that points into
   `cache/stdpatch/` is ignored, so the generator never depends on its own output.
2. When `cache/stdpatch/<key>/goroot` exists, answer it.
3. Otherwise build the tree in a new directory under `cache/stdpatch/`, run
   `git apply` in it with `GIT_CEILING_DIRECTORIES` set so that git does not find
   the Ze repository, and check that `go list std cmd` under the new tree answers
   the same packages as under the real GOROOT. Then rename the directory to the
   key. The rename makes two sessions that race produce one result: the session
   that loses deletes its own directory and answers the winner's. The generator
   needs `git` wherever it runs.
4. When `git apply` refuses, stop with an error that names the Go version, the
   patch file, git's output, and the section "When the Go version changes". No
   caller builds without the tree after that error.

`./le scratch cache-clean` leaves `cache/stdpatch/` alone and reports the size of
each tree, because a build in another session may be running under one of them.

## What a build sets

Three settings select the tree, and each one is needed:

| Setting | Why |
|---------|-----|
| `GOROOT=<tree>` | the patched source on disk, for the go command and every tool |
| `GOTOOLCHAIN=local` | a toolchain switch sets `GOROOT` to the toolchain it switches to, which would drop the tree. The pin still holds, because the tree was built from the pinned toolchain |
| `<tree>/bin` first on `PATH` | a tool that starts `go` itself (golangci-lint, staticcheck, govulncheck, CodeQL) must find the same go |

Under the tree, `go version` reports the real toolchain's version, so
govulncheck matches standard-library advisories against the right release.

## Every build gets the tree

The tree reaches each Go command that compiles Ze from one producer:

- `./le` actions take their environment from `gotoolchain`
  (`internal/le/go/toolchain`), which sets `GOROOT` and `GOTOOLCHAIN=local`,
  puts `<tree>/bin` first on `PATH`, and starts every Go command with the tree's
  go binary.
- The `./le` launcher script builds `cmd/ze` with the `ze_le` tag, and that
  binary links the EAP-TLS authenticator. The script runs the generator with
  `go run` first, then builds under the tree.
- Sites that build their own environment (the tracked build gate, which refuses
  ambient settings on purpose; the gokrazy build; QEMU guests; the Docker images;
  the CI workflows) each call the generator and set the three values. A QEMU
  guest or a container has its own GOROOT, so it runs the generator itself.
<!-- source: internal/le/go/toolchain/gotoolchain.go -- Toolchain.Overrides -->
<!-- source: le -- build_le -->

`./le rfc discriminate-record` still compiles a deliberately broken copy of a
repository file through `-overlay`. That overlay names only the repository file,
and the standard library comes from `GOROOT`, so the two never meet.
<!-- source: internal/le/rfc/discriminate_observe.go -- writeOverlay -->

The spec lists every site. A missed site cannot ship an unpatched binary that
links the EAP-TLS authenticator (`internal/core/eap`), because of the guard
below. A binary that does not link it builds without error and without the
patch.

## The guard: an unpatched build does not compile

The patch also adds a file to the package. That file declares one exported
constant. For the `crypto/tls` patch the spec names it
`tls.ZeServerChoosesSessionID`. The Ze code that depends on the patch references
the constant. A build under an unpatched GOROOT then stops with:

```
undefined: tls.ZeServerChoosesSessionID
```

The comment at the reference names this page. So a binary that carries the
dependent code is built with the patch, or it is not built. A lint or
vulnerability run under an unpatched GOROOT fails the same way, so no gate
passes over code it did not see.

Three checks cover what the constant cannot:

| Check | Catches |
|-------|---------|
| The constant | Any build without the tree: CI, a Dockerfile, a developer, a missed site |
| A test of the patched behavior | A tree that adds the constant but not the change |
| The generator's refusal | A Go version whose file no longer matches the patch |

Two other guards were rejected. A test alone proves its own build, never the
binary that ships. A runtime self-check makes plain builds work, but it ships a
broken binary that fails at the operator's site instead of at build time.

## Plain `go` and gopls

A plain `go build` or `go test` of a package that links the dependent code fails
with the error above. To use plain commands, set the three values in the shell:

```
export GOROOT="$(./le go stdpatch goroot)"
export GOTOOLCHAIN=local
export PATH="$GOROOT/bin:$PATH"
```

gopls sees the patch when it starts with that environment: gopls v0.23.0, run
under the tree, reported no error at the constant. A gopls daemon shared between
editors keeps the environment of whichever client started it. Started from a
shell without the tree, it reports `undefined: tls.ZeServerChoosesSessionID` at
the reference, while navigation, `gopls symbols`, definitions and references
still work. Start the daemon from a shell that has the three exports, or expect
that one diagnostic.

## When the Go version changes

1. Change the Go version in `go.mod` as usual.
2. Build. When the patch still applies, the generator builds a new tree, and
   the patched-behavior tests decide whether the change still holds.
3. When the generator refuses, read the new upstream function. If upstream now
   does what the patch did, delete the patch, the constant, the reference to it,
   and the generator if no patch remains (`ai/rules/no-layering.md`).
4. Otherwise, make the patch again from the new GOROOT file: copy the file,
   apply the change by hand, and write the diff with
   `git diff --no-index <original> <patched>`, with the paths changed to
   `a/src/...` and `b/src/...`. Keep the leading text. Run the patched-behavior
   tests.

## Alternatives considered

| Alternative | Why it was rejected |
|-------------|---------------------|
| `go build -overlay`, the article's delivery | golangci-lint, staticcheck and govulncheck cannot open a file an overlay adds (see above), and two of them are Ze gates |
| Fork the Go toolchain | Every contributor, CI job, container and the gokrazy build must install the fork, and each Go release must be rebased by hand |
| A full copy of GOROOT per toolchain | Hundreds of megabytes per key, for two changed files |
| A copy of `crypto/tls` in the tree (vendored or `internal/`) | `crypto/tls` uses internal standard-library packages that code outside GOROOT cannot import. The copy also stays on old upstream code after each Go release, silently |
| Mirror the unexported struct layout with `unsafe` | The article measures this route for `net/netip`, and it needs tests to catch layout changes. Here it cannot work at all: the value is set inside an unexported method during the handshake, and the callbacks that run then (`GetConfigForClient`, `GetCertificate`) receive no pointer to the handshake state that holds the ServerHello |
| Ask upstream for an API | Correct in the long term, and it does not fix the current release. If upstream adds the behavior, step 3 above deletes the patch |

Ze changes vendored modules with a related mechanism: patches under
`internal/le/vendorpatch/patches/`, with a guard test each. The difference is
that a vendored file is in the tree, so the patch is applied once and committed.
A GOROOT file is not in the tree, so the patch is applied at build time.
<!-- source: internal/le/vendorpatch/vendor_patch_test.go -- assertVendorPatchApplied -->

## Reference

- Vincent Bernat's article on `netip.Addr` and IPv4-mapped addresses, the
  `-overlay` update in its section "Modifying the Go standard library":
  <https://vincent.bernat.ch/en/blog/2026-go-netip-addrto6>
- `go help build`, the `-overlay` flag
- `go help environment`, `GOROOT` and `GOTOOLCHAIN`
