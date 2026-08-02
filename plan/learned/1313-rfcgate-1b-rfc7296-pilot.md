# 1313 -- rfcgate-1b: the RFC 7296 pilot, and what a green gate was not measuring

## Context

`rfc/short/rfc7296.md` carried 23 requirement rows over an RFC with 289 MUST-level keyword
lines, and every gate judges only what a summary lists, so `make ze-rfc-check` had been green
over a 92% blind spot. The pilot extracted the missing obligations, implemented what was
absent, put a mutation-verified tagged pair behind each, and signed
`rfc/extraction/rfc7296.json`. The summary now carries 227 rows, 222 gated. Those rows are
the smaller half: the larger half is that signals green for months were measuring nothing,
found only by asking whether the EVIDENCE was real instead of looking for failing tests.

## Decisions

- **A tier is honest only when something EXECUTES the test AND the test CAN fail.** Wiring
  the interop trees into a pipeline before fixing their fail-open checks would have granted
  evidence status to scenarios that passed with ESP dead.
- **Ze ships fail-closed on EAP-TLS over TLS 1.2 without RFC 7627.** A `//go:debug
  tlsunsafeekm=1` in `cmd/ze/main.go` weakened the export rule for every user to suit one
  peer version. Owner ruling removed it; the lab opts in per scenario.
- **Every fix raised proof rather than lowering the claim.** `RFC7296-1.4-1`'s gap was
  cleared because the behavior landed, not by reclassifying a keyword-free sentence.
- **The sign-off landed before this spec closed** (owner ruling, 2026-08-02), rather than
  moving to a successor spec. The sign-off IS the pilot's subject: closing without it would
  have shipped the extraction and dropped the thing being piloted.

## Consequences

- **The extraction sign-off paid for itself on its first real run.** rfc7296 was chosen
  because it was the worst-measured input in the corpus. The walk (261 keyword sites in 104
  sections, 230 mapped, 31 excluded, 26 `unsourced-ids`) found **four MUSTs the summary
  never carried**: `RFC7296-3.2-5` and `-3.2-6` (the Critical-bit SENDER obligations of
  §3.2), `-1.3.3-2` (REKEY_SA when a CREATE_CHILD_SA replaces an ESP or AH SA), and `-4-5`
  (the four-message capability). **No gate in the repo could have asked for one of them**:
  every gate judges what a summary already lists. Each landed with a tagged pair.
- **The same reading found five rows whose level or anchor misquotes its RFC**, three of them
  RFC 7296's: `2.8-1` states MUST where the text says SHOULD, `3.3.6-1` cites a section
  stating no such obligation, `1.7-2` cites a second section carrying no matching sentence.
  Gated, proven, green, wrong. Homed in `plan/spec-fixit-rfc-row-level-and-anchor-drift.md`.
- **A sixth exclusion kind, `relocated-to-spec`, was added** to `rfc_requirements.py`. The
  other five say a sentence binds nobody; this one says somebody is bound, over there. Twelve
  sites moved to `plan/spec-ipsec-remote-access.md` and `plan/spec-ipsec-ipcomp.md` by owner
  ruling, and `binds-another-role` would have asserted Ze plays no IRAS or IPComp role while
  two specs exist to build exactly those roles. It fails closed: the check re-reads the named
  spec every run and refuses if the spec is gone, dropped the reserved id, or the summary
  took the id back.
- Interop went from two scenarios reporting false passes to eight green, plus a new
  `06-eap-tls13`. The XFRM model gained an IKE bypass, `UPDPOLICY` so a rekey survives
  EEXIST, and a shared-selector check.

## Gotchas

- **Five defects were invisible to same-implementation testing:** a bare OID where RFC 7427
  wants an `AlgorithmIdentifier` SEQUENCE, which ze's own verifier accepts; RFC 2759's
  one-octet Success Response, of which ze demanded four; a discarded EAP-TLS closing flight;
  an MSK derived as `sha256(TLSUnique)`, which no implementation computes; a client
  certificate with no subjectAltName. **EAP-TLS had never once interoperated while its suite
  was green.** A self-test suite cannot find a defect both ends share.
- **Two interop `check.py` files wrapped their ESP assertion in
  `except (AssertionError, Exception)`**, one calling `log_pass` in the handler. Removing
  those swallows is what made everything above visible.
- **A green carrier can be green for a reason unrelated to its subject.**
  `test/ipsec/ipsec-child-rekey.ci` passed while the rekey it tests was broken, because it
  sets `ze_test_ike_dataplane=noop`. AC-18 passed by grepping `{gap}` while every annotation
  on disk reads `{gap: <reason>}`: it found zero of something it could not see.
- **A spelled-out count is a guard, and adding the thing it counts costs one edit, not zero.**
  Scenario `06-eap-tls13` was added and `test/ipsec-interop/lab_test.py` still asserted three
  PKI scenarios and nine leaves. The totals stay spelled out on purpose: a scenario that
  silently stops carrying PKI material must red.
- **Five review rounds, none empty, and two found the previous round's fixes INOPERATIVE.**
  The shared cause was tests asserting that a call HAPPENED, not what reached the wire. One
  fix was worse than the defect it closed: the `errSADead` exit returned above the only
  writer of `RetransmitCount`, collapsing the reconnect backoff from 60s to 1s.

## Files

- `internal/component/ike/` -- `engine/` (`delete.go`, `bypass.go`, `notify_error.go`,
  `cookie.go`, `ts_narrow.go`, `transport_mode.go`, `certurl.go`, `cert_payload.go`,
  `msgid.go`, `child.go`, `fsm.go`), `eap/eap_tls.go`, `eap/eap_mschapv2.go`,
  `dataplane/` (`espform.go`, `xfrm_linux.go`, `policy_owner.go`), `ipsec/validate.go`
- `scripts/dev/rfc_requirements.py` and its test; `rfc/extraction/rfc7296.json` and
  `README.md`; `rfc/short/rfc7296.md`; `rfc/enrolled.txt`; `docs/features/rfc-status.md`;
  `test/ipsec-interop/` `lab.py`, `lab_test.py`, scenarios 02, 04, 06, 12, 18
