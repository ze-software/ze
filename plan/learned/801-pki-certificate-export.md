# 801 -- PKI Certificate Export Commands

## Context

Ze's `show pki certificate <name>` returned structured JSON detail but had no way to export raw PEM text, bundled cert+key, or fingerprints. Operators need these during troubleshooting ("is this the right cert?") and for feeding PEM blobs to external clients (e.g. OpenConnect). VyOS PR #5225 added similar capability; this adapts the concept to Ze's architecture.

## Decisions

- Chose sub-command routing inside the single `handleShowPKICertificate` handler over separate RPC handlers per output format, because Ze's dispatcher uses longest-prefix matching and a positional `<name>` argument between the command prefix and sub-command breaks the prefix chain. Three separate wire methods (`ze-show:pki-certificate-pem`, etc.) were initially implemented and registered in YANG sub-containers, but review proved they were dead code: `show pki certificate dev-1 pem` matches `show pki certificate` (the shorter prefix), not `show pki certificate pem`.
- Chose SHA-256/384/512 for fingerprint over including SHA-1, because SHA-1 is deprecated for certificate fingerprinting and including it would invite misuse.
- Chose colon-separated hex for fingerprint format over raw hex, matching OpenSSL convention that operators expect.
- Chose to reject `bundle pem` for CA certificates over silently returning cert-only, because CAs do not have private keys in the store and silent partial output would confuse operators.

## Consequences

- The dispatcher's prefix-matching model means any CLI command that takes a positional argument cannot have YANG sub-container children that add keywords after the argument. Sub-commands must be handled inside the handler via arg inspection. This is a general constraint, not PKI-specific.
- Future export formats (PKCS#12, JKS) would follow the same pattern: add a case to the switch in `handleShowPKICertificate`, no new wire method needed.
- The `marshalPrivateKeyPEM` and `certRawDER` helpers are available for any future handler that needs PEM encoding of stored keys/certs.

## Gotchas

- **YANG sub-containers do not work for positional-argument commands.** This was the critical discovery. The initial implementation registered three separate wire methods with YANG `container` children under `certificate`. The YANG tree builder created paths like `show pki certificate pem`, but the dispatcher registered them as prefix-match keys. With a positional `<name>` between `certificate` and `pem`, the input `show pki certificate dev-1 pem` never matched the deeper key. Two review passes were needed to catch this: the first pass missed it because wiring verification checked that wire methods were registered (they were) but did not trace through to the prefix-match dispatch logic.
- The `nilerr` linter flags any `if err != nil { return response, nil }` pattern where `nil` is the error return. Extracting a helper that returns `(*plugin.Response)` instead of `error` avoids the lint without losing the error information.
- The `goconst` linter triggers on 3+ occurrences of the same string literal, even when the string is a short keyword like `"sha256"`. Constants are the right fix regardless.

## Files

- `internal/component/pki/show.go` -- sub-command routing, PEM/bundle/fingerprint logic
- `internal/component/pki/store_test.go` -- 16 new tests covering all sub-commands and error paths
- `internal/component/pki/yang/ze-pki-api.yang` -- updated RPC description
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- updated command description
- `test/plugin/show-pki-certificate-export.ci` -- functional test (5 assertions)
- `docs/guide/command-reference.md` -- new sub-commands documented
- `docs/architecture/api/commands.md` -- wire method and response formats
