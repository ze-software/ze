# 1255 -- fixit-codeql-security-triage

## Context

GitHub code scanning showed 86 open CodeQL alerts on `ze-software/ze` and nobody
had triaged them. An untriaged scanning page is worse than no scanner: real
findings become indistinguishable from noise, so everyone learns to ignore all of
it. The task was to judge every alert against the producing source, fix what was
real, and leave the page in a state where a future alert means something.

Six alerts were real. The rest split into protocol-mandated cryptography (6), a
guard living upstream of the flagged line (49), a guard local to it (18), and
test-only tooling (8). Two more defects surfaced that CodeQL never flagged, and an
independent review pass found eleven more the author had missed.

## Decisions

- **Judged every alert against the producing function, never the description.**
  49 of 86 were one pattern: a config-tree narrowing whose bound lives three
  layers up in the config parser. Proving that meant reading
  `schema.go ValidateLeafValue` and confirming empirically that
  `ze config validate` rejects an out-of-range leaf. Without that, the honest
  answer would have been "probably fine".

- **Replaced the rescue credential rather than strengthening it.** The installer
  published an unsalted `sha256(adminPassword)` on a kernel cmdline, inside an
  iPXE script served unauthenticated over plain HTTP. Salting it would have left
  a digest that still commits to the admin password. It now commits to a
  dedicated random token (`internal/core/rescueauth`), so a full break costs
  rescue-shell access on a machine that has already failed to install. Chose an
  independent token over a salted admin-password KDF because the digest is public
  by construction, not merely exposed.

- **Hex `salt:digest` over a PHC or bcrypt string.** `$` risks interpolation in
  the generated iPXE script; hex plus `:` is safe in both iPXE and
  `/proc/cmdline`.

- **Bounded the OSPF/IS-IS narrowing by the target type's own maximum**, not a
  per-leaf maximum. That is what made a 42-site mechanical conversion safe: every
  YANG leaf declares the same width as the Go field it feeds, so no config the
  parser accepts can newly be refused. The one leaf that did not
  (`srms-preference`, `type uint8` with no `range`) was caught by review.

- **Dismissed per alert number with individual comments**, never a query-level
  suppression. A filter would blind the repo to the next real instance.

## Consequences

- `ai/rules/critical-review.md`'s "a different context" requirement is not a
  formality. The author's own review pass found one real BLOCKER. Three
  independent subagents then found five more BLOCKERs and six ISSUEs in the same
  diff, including a config-verify path that mutated live process state and four
  tests that provably gated nothing. Self-review found roughly a sixth of what
  independent review found, on code the author had just written and believed
  correct.

- `ze-installer-unit-test` now exists and is a prerequisite of `ze-unit-test`.
  Four test files guarded by `ze_installer` had never run: no `go test` supplied
  the tag. Anything behind a build tag that no test target passes is invisible,
  and `go test` reports success for a package whose files were silently excluded.
  `test/health/latest.json` tracks these as `tag-orphan`; consult it before
  trusting that a tagged file runs.

- Config verification must be side-effect free, and the IPsec verifier was not:
  it called `pki.Load`, swapping the process-wide store, so a REJECTED commit left
  the daemon holding the config it refused. A verifier that must resolve names
  parses the candidate into a throwaway lookup set; it never installs it.

- `ValidateInterfaceRef` is still unwired, `remote-access` gateway certificate
  references are still unvalidated, and `eapTLSServerConfig` (responder side)
  keeps the `if ca != nil` swallow the initiator side lost. Same class, out of
  scope here, recorded in the spec's NOTEs before closure.

  **Correction (2026-07-23, spec-fixit-ipsec-verify-siblings).** This bullet
  originally called the `eapTLSServerConfig` swallow a "fail-open". It is not one,
  and the error matters because it points the fix in the wrong direction.
  `newTLSMethod` (`ike/eap/eap_tls.go:150-174`) passes crypto/tls a **non-nil but
  empty** `x509.CertPool` as `ClientCAs` together with
  `ClientAuth: tls.RequireAndVerifyClientCert`, and an empty non-nil pool REJECTS
  every chain -- only a `nil` Roots falls back to the host root store. The
  responder therefore fails CLOSED; the defect is that it does so *silently and
  late*, refusing every client at handshake with an opaque "certificate signed by
  unknown authority" that names neither the peer nor the CA that failed to load.
  A guard that denies correctly while saying nothing is still a bug
  (`ai/rules/fail-closed-guards.md`, "or say something"), which is why it was worth
  fixing -- but it was never an authentication bypass. Now pinned by
  `TestEAPTLSAuthenticatorWithoutCARejectsEveryClient` and `TestEmptyClientCAPoolRejects`
  (`ike/eap/eap_tls_trust_anchor_test.go`), so the distinction cannot rot again.

  The same follow-up work found that `vpn ipsec remote-access` is **inert**, which is
  a much larger defect than the unvalidated references named above: the virtual IP
  pool is built and then discarded (`ike/engine/register.go:372` is `_ = ipPool`),
  `ra.Auth` and every `eap-user` have no consumer at all, and the responder admits
  only sources that `matchResponderPeer` resolves to a configured site-to-site peer,
  so a road-warrior client can never establish. Being implemented under its own spec.

## Gotchas

- **A bounded helper does not satisfy CodeQL.** `vrrp/groups.go` has bounded
  through `asUint(v, max)` all along and all four of its conversions are still
  flagged: the query cannot follow a `max` parameter. The spec claimed the
  OSPF/IS-IS helper would "silence the family as a side effect". It does not.
  Verify a suppression claim against the live alert list before writing it down.

- **YANG `uint8` maps to schema `TypeUint16`.** A `uint8` leaf with no explicit
  `range` accepts 256..65535 at parse time. `srms-preference` had none, so 300
  passed validation, `configUint8` refused it, and the RFC 8665 SRMS TLV silently
  stopped being advertised. Bare `type uintN` is not a bound.

- **A test asserting an error message can match the wrong error.**
  `contains=rescue-auth` also matches the unknown-field message, so the `.ci`
  passed with the leaf deleted from the YANG entirely. Pin the needle to text only
  the intended producer emits.

- **`ze config validate` does not invoke plugin `OnConfigVerify`.**
  `SendConfigVerify` is reached only from the reload transaction bridge, so a
  plugin cross-reference check fires at SIGHUP and nowhere else. The functional
  test belongs in `test/reload/`, not `test/parse/`.

- **Bare `go test` fake-reds feature-gated packages.** Two web tests failed
  because `policyFilterAddActions` derives from a YANG schema that is empty
  without the feature tags. Always pass
  `awk '!/^#/ && NF {print $1}' feature-gates.txt`.

- **`go mod vendor` reverts the gokrazy/updater fork patches.** Re-run
  `scripts/dev/reapply-updater-fixes.py` after any vendor refresh.

- **Plain `goimports -w` undoes the project import grouping** (golangci uses
  `local-prefixes`, so the local group comes last, after a blank line).

- An ENOSPC during a mid-flight write truncated the spec, and the truncated file
  was committed. Near a full disk, confirm a generated or edited file is intact
  before committing it.

## Files

- `internal/core/rescueauth/` (new): credential encoding, verification, memory bound
- `internal/install/disk/`: cmdline, validate, run, rescue console, fatal policy
- `internal/plugins/provision/`, `internal/plugins/imageserver/` (+ YANG): mint and publish
- `internal/appliance/cmd_config_push.go`: known_hosts verification
- `internal/component/ike/`: EAP-TLS trust anchor at runtime, config build, and verify
- `internal/component/web/`: `isSameOriginPath` and its two consumers
- `internal/plugins/ospf/`, `internal/plugins/isis/`: `configUint8/16/32`
- `mk/test-unit.mk`: `ze-installer-unit-test`
- `plan/known-failures/`: two new entries with attribution evidence
