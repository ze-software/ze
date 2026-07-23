# 1256 -- fixit-ipsec-verify-siblings

## Context

`plan/learned/1255-fixit-codeql-security-triage.md` closed having fixed the EAP-TLS
trust-anchor defect on the initiator side, and recorded three siblings of the same class
that its scope did not cover: `ValidateInterfaceRef` unwired, remote-access certificate
references unvalidated, and the responder's `if ca != nil` swallow. This spec closed those,
plus two bookkeeping obligations that came with them. Two of the three turned out not to be
what the record said they were, and chasing the third surfaced a considerably larger defect.

## Decisions

- **Read the producer before trusting the record.** 1255 called the responder swallow a
  "fail-open". It is not one. `newTLSMethod` (`ike/eap/eap_tls.go:150-174`) hands crypto/tls
  a non-nil but EMPTY `x509.CertPool` as `ClientCAs` with `RequireAndVerifyClientCert`, and
  an empty non-nil pool REJECTS every chain: only a `nil` Roots falls back to the host root
  store, and that code never passes nil. The responder failed CLOSED, silently and late.
  Measured, not reasoned, then pinned by two tests so the distinction cannot rot. It still
  deserved fixing (`fail-closed-guards.md`: a guard that denies while saying nothing does not
  exist), but the label sent the fix in the wrong direction.

- **Where a check belongs is decided by what kind of fact it asserts.** A config-consistency
  fact (does this name resolve inside the config I am judging?) belongs in the plugin
  verifier. A host-readiness fact (does this interface exist on this box?) belongs in
  `ze doctor`. `ValidateInterfaceRef` is the second kind, so it was wired through a
  plugin-owned doctor check following the `checkDHCPInterfaces` precedent, NOT through
  `OnConfigVerify`. Putting it in the verifier would reject a config-first deployment that
  legitimately names an interface the same commit is about to create -- and the ike plugin's
  `ConfigRoots` (`{"vpn","pki"}`) cannot even see the interfaces section.

- **A silent absence of signal is a defect, not a test problem.** A successful SIGHUP reload
  printed NOTHING (`cmd/ze/hub/main_reload.go`); only failures printed. The daemon's last
  word was identical whether a reload finished instantly, was still running, or had wedged.
  Fixed at the source with a stable `reload complete` marker rather than by making the tests
  sleep longer.

- **Declare the requirement, do not widen the gate.** `option=needs-linux` gained an optional
  `caps=net-admin` rather than making every `needs-linux` test capability-gated. Marking all
  of them would have skipped tests that pass unprivileged today, which deletes coverage.
  Only the seven that genuinely cannot pass were marked; their four siblings still run.

## Consequences

- `ze doctor` now reports `doctor-ipsec-iface`, and `ValidateInterfaceRef` finally has a
  non-test caller. It had been implemented, tested and reachable from nothing -- the same
  shape as the wiring gaps `ai/rules/wiring-completeness.md` exists to catch.

- A successful reload is observable for the first time, by operators and by tests. Every
  reload `.ci` can now fence deterministically on `await=stderr:contains=reload complete`
  instead of racing its own teardown.

- The `bgp reload` suite went from 2 failures + 6 hangs (63s) to 31/31 passing with 7 honest
  skips (10s), and two `plan/known-failures/` shards were archived as genuinely resolved.

- **`vpn ipsec remote-access` is inert**, which is a larger defect than the one this spec set
  out to fix: the virtual IP pool is built then discarded (`ike/engine/register.go:372` is
  `_ = ipPool`), `ra.Auth` and every `eap-user` have no consumer, and `matchResponderPeer`
  admits only configured site-to-site peers, so a road warrior can never establish. An
  operator can write a complete remote-access VPN, have it accepted by `ze config validate`
  AND by the reload transaction, and get a daemon that does nothing with it. Owner chose to
  implement rather than reject; `plan/spec-ipsec-remote-access.md` owns that work.

## Gotchas

- **A hypothesis recorded in a shard reads as fact to the next agent.** The known-failures
  entry blamed "the plugin connection closes before verify is dispatched". The first real
  stress run disproved it: that signature appears nowhere in the capture, and the true cause
  was a test-harness race. Label unverified causes as hypotheses.

- **A tool that has never been run on the failure it targets may not work.**
  `stress-repro.py` crashed on EVERY timeout (`bytes + str` from `TimeoutExpired`, which
  carries undecoded streams even under `text=True`), and its 120s default guarantees a
  timeout for `bgp reload` under load. With `--any-failure` it also reported a usage error as
  a reproduction: `stress-repro.py reload` printed `*** REPRODUCED ***` for
  `unknown command: reload`. There is no `reload` suite -- the reload tests live under `bgp`,
  and a sub-suite is passed as ONE argument (`"bgp reload"`).

- **`option=skip-os:value=darwin` on a test that APPLIES Linux config is the wrong marker.**
  `ai/rules/qemu-testing.md` prescribes `needs-linux`, which also enrols the test in the QEMU
  run. Unprivileged, those tests do not fail -- the interface plugin dies mid-handshake, the
  daemon never reaches the asserted state, and the test HANGS to the suite timeout. That is
  why they looked load-sensitive.

- **Gate a capability, not a uid.** `hasNetAdmin` reads `CapEff` from `/proc/self/status`
  because a setcap'd binary holds CAP_NET_ADMIN without being root, and a restricted
  container can be root without it.

- **A bare `go run` of a registry-importing tool fakes reds.** `commit_helper.py` ran
  `scripts/docvalid/doc_drift.go` with no build tags, so every feature-gated NLRI plugin was
  compiled out and all 11 address-family claims in `docs/comparison.md` and `docs/DESIGN.md`
  were reported as drift on a tree whose `make ze-doc-test` was green. `doc_drift.go:21`
  blank-imports `plugin/all`; the sibling tools (`plugin_imports.go`, `inert_tests.go`) are
  stdlib-only source scanners and correctly need no tags.

- **`EAPUser.Certificate` has no runtime consumer.** Validating it against the PKI store
  would have been an invented requirement, and a client certificate does not normally live in
  the gateway's own store anyway.

## Files

- `internal/component/ike/engine/`: `responder_eap.go` (trust-anchor refusal), `doctor.go` (new)
- `internal/component/ike/eap/eap_tls_trust_anchor_test.go` (new): pins the empty-pool semantics
- `internal/core/diagnostic/codes.go`: `doctor-ipsec-iface`
- `cmd/ze/hub/main_reload.go`: `reload complete` marker
- `internal/test/runner/caps_linux.go`, `caps_other.go` (new), `record_parse.go`: `caps=net-admin`
- `scripts/dev/stress-repro.py`: timeout crash + usage false-positive
- `scripts/dev/commit_helper.py`: feature tags for the drift checker
- `plan/known-failures/`: two shards archived to `RESOLVED.md`, one rewritten
