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
2026-10-14, using the technique in Vincent Bernat's article
(<https://vincent.bernat.ch/en/blog/2026-go-netip-addrto6>).

## The mechanism: `go build -overlay`

Every `go` command that compiles (`build`, `test`, `vet`, `run`, `list -compiled`)
accepts `-overlay=<file>`. The file is JSON with one key, `Replace`, that maps
the absolute path of a source file to the absolute path of the file to compile
in its place. A path that does not exist adds a new file to that package. The
mapping works for standard-library files under GOROOT, so the package is changed
for this build only, and GOROOT stays untouched.
<!-- source: internal/le/rfc/discriminate_observe.go -- writeOverlay -->

Ze already uses an overlay in another place: `./le rfc discriminate-record`
compiles a deliberately broken copy of a repository file through `-overlay`,
and sends the flag to second processes through `GOFLAGS`.
<!-- source: internal/le/rfc/discriminate_observe.go -- setGoFlagsOverlay -->

Two facts about the flag decide the design. Both were tested with go1.27.2 on
2026-10-14:

- An overlay can add a new file to a standard-library package.
- A command takes one overlay. A `-overlay` on the command line replaces the one
  in `GOFLAGS`, so a second overlay must merge the first one's entries.

The article names the cost: the JSON maps absolute GOROOT paths, and every go
command needs the flag. The rest of this page is how Ze pays that cost once.

## The layout

| Part | Location | Role |
|------|----------|------|
| The patch | `internal/le/go/stdpatch/patches/<name>.patch` | A git unified diff with paths relative to GOROOT (`a/src/crypto/tls/handshake_server.go`). Its leading text names this page and quotes the requirement |
| The generator | `internal/le/go/stdpatch` | Makes the patched copies and the overlay JSON for one GOROOT. It imports only the standard library, so it compiles before any overlay exists |
| The output | `cache/stdpatch/<key>/` | The patched copies and `overlay.json`, keyed by a SHA-256 over the `go version`, the patch, and each original GOROOT file |
| The command | `./le go stdpatch overlay` | Prints the overlay path for the `go` on PATH, for gopls and plain `go` commands |

The patch is stored as a diff, not as a patched copy of the file. A copy belongs
to one Go version, so a toolchain bump would silently compile old upstream code.
A diff applies to the GOROOT file of the toolchain in use, or refuses.

The generator does this, each time:

1. Resolve GOROOT with `go env GOROOT`, under the same environment as the build.
   The `GOTOOLCHAIN` pin can select a downloaded toolchain whose GOROOT is under
   `GOMODCACHE`, so GOROOT read in another environment can be the wrong one.
2. When `cache/stdpatch/<key>/overlay.json` exists, answer it.
3. Otherwise, copy each target file to a new directory under `cache/stdpatch/`,
   run `git apply` there with `GIT_CEILING_DIRECTORIES` set so that git does not
   find the Ze repository, write `overlay.json`, and rename the directory to the
   key. The rename makes two sessions that race produce one result.
4. When `git apply` refuses, stop with an error that names the Go version, the
   patch file, git's output, and the section "When the Go version changes". No
   caller builds without the overlay after that error.

## Every build gets the overlay

The overlay reaches each Go command that compiles Ze from one producer:

- `./le` actions take their environment from `gotoolchain`
  (`internal/le/go/toolchain`), which adds `-overlay=<path>` to `GOFLAGS`. It
  appends to an inherited `GOFLAGS` and never replaces it.
- The `./le` launcher script builds `cmd/ze` with the `ze_le` tag, and that
  binary links the EAP-TLS authenticator. The script runs the generator with
  `go run` first.
- Sites that build their own environment (the tracked build gate, which refuses
  ambient `GOFLAGS` on purpose; the gokrazy build; QEMU guests; the Docker images;
  the CI workflows) each call the generator. A QEMU guest or a container has its
  own GOROOT, so it runs the generator itself.
<!-- source: internal/le/go/toolchain/gotoolchain.go -- Toolchain.Environment -->
<!-- source: le -- build_le -->

The spec lists every site. A site that is missed cannot ship an unpatched
binary, because of the guard below.

## The guard: an unpatched build does not compile

The patch also adds a file to the package. That file declares one exported
constant. For the `crypto/tls` patch the spec names it
`tls.ZeServerChoosesSessionID`. The Ze code that depends
on the patch references the constant. A build without the overlay then stops
with:

```
undefined: tls.ZeServerChoosesSessionID
```

The comment at the reference names this page. So a binary that carries the
dependent code is built with the patch, or it is not built.

Three checks cover what the constant cannot:

| Check | Catches |
|-------|---------|
| The constant | Any build without the overlay: CI, a Dockerfile, a developer, a missed site |
| A test of the patched behavior | An overlay that adds the constant but not the change |
| The generator's refusal | A Go version whose file no longer matches the patch |

Two other guards were rejected. A test alone proves its own build, never the
binary that ships. A runtime self-check makes plain builds work, but it ships a
broken binary that fails at the operator's site instead of at build time.

## Plain `go` and gopls

A plain `go build` or `go test` of a package that links the dependent code fails
with the error above. To use plain commands, put the overlay in `GOFLAGS`:

```
export GOFLAGS="-overlay=$(./le go stdpatch overlay)"
```

For gopls, put the same `-overlay=<path>` in the `build.buildFlags` setting of
your editor, or start the editor with that `GOFLAGS`. Queries that only parse,
such as `gopls symbols`, work without it.

## When the Go version changes

1. Change the Go version in `go.mod` as usual.
2. Build. When the patch still applies, the generator writes a new overlay, and
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
| Fork the Go toolchain | Every contributor, CI job, container and the gokrazy build must install the fork, and each Go release must be rebased by hand |
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
