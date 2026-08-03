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
- **Eight rounds in the end, and rounds 6 and 7 repeated the pattern at one remove: the
  weak part was the previous round's PROOF, not its code.** Round 6 fixed four real
  defects and shipped two tests that could not fail. A reviewer that MUTATES finds this;
  a reviewer that reads does not. Every round after the first was cheaper than the bug it
  caught, and the loop only converged once each fix carried a mutation that killed it.

## Gotchas, round 6 to 8

- **A result type with two fields whose consumer branches on one silently loses the
  other.** `MethodResult` carries `Response` and `Err`; `Session.handleMethod` tests `Err`
  first and answers `s.failure()`. A fix that set BOTH looked complete and put nothing on
  the wire: the EAP-TLS fatal alert RFC 5216 Section 2.1.3 exists to deliver was dropped
  for two commits. Where a protocol spends two rounds, the producer has to spend two
  rounds -- park the cause, send the packet, report on the round after. Ze as PEER needed
  the mirror of the same fix, because the authenticator now WAITS for a reply the peer was
  discarding.
- **A guard keyed on inequality is inert when both sides are empty.** `policyOwners.claim`
  refuses on `held != p.Owner`. Delete the owner at either producer and every claim
  compares `"" != ""`, so the two-peer takeover it exists to refuse is admitted in
  silence. The test that "covered" it compared two empties, because its own fixture never
  set the field. Drive a guard from the entry point that PRODUCES its input, never from
  the guard's own helper.
- **Where finding evidence is what PASSES, ambiguity must strip MORE.** The
  `relocated-to-spec` tripwire searched a destination spec's raw bytes, so an id inside an
  HTML comment or a strikethrough span read as a live reservation. The fix removes the
  markup that DISOWNS text, and every fallback consumes the rest of the input rather than
  less: stripping too little leaves an obligation owed by nobody, stripping too much only
  reddens a gate that names the file.
- **Patching a module before importing a test module that re-execs it patches a discarded
  copy.** `rfc_requirements_test._load()` runs `importlib` and overwrites
  `sys.modules["rfc_requirements"]`. The mutation harness reported KILLED for an unrelated
  reason (a stale ledger) while the four tests it was aimed at never moved. Patch the
  object the TEST holds.
- **A floor set below the tree is slack exactly where the new rows are.** `GATED_FLOOR`
  read 218 against 222, so the four MUSTs the extraction walk had just found were the four
  that could vanish unopposed.
- **An assertion of the form `any(x in e for e in errs)` is satisfied by any error.** Every
  denying arm of the tripwire embeds the site id and the requirement id, so four new tests
  asserting those two substrings passed against a stub returning the WRONG arm. Assert the
  phrase only the arm under test emits.
- **Closing a spec without the "design references survive closure" grep leaves debt in
  three gates at once**, and the sweep of 2026-08-02 left all three red at HEAD: two
  `// Design:` lines, 31 spec-to-spec citations, and 8 dead references over the
  learned-staleness ceiling. None was caused by this spec and all three blocked its
  commit, because a structural gate is never known-red. The repointing is mechanical --
  a closed `plan/spec-<stem>.md` becomes `plan/learned/<NNNN>-<stem>.md` -- and it is
  worth running as a closure step rather than leaving for whoever meets the red next.

## Files

- `internal/component/ike/` -- `engine/` (`delete.go`, `bypass.go`, `notify_error.go`,
  `cookie.go`, `ts_narrow.go`, `transport_mode.go`, `certurl.go`, `cert_payload.go`,
  `msgid.go`, `child.go`, `fsm.go`), `eap/eap_tls.go`, `eap/eap_mschapv2.go`,
  `dataplane/` (`espform.go`, `xfrm_linux.go`, `policy_owner.go`), `ipsec/validate.go`
- `scripts/dev/rfc_requirements.py` and its test; `rfc/extraction/rfc7296.json` and
  `README.md`; `rfc/short/rfc7296.md`; `rfc/enrolled.txt`; `docs/features/rfc-status.md`;
  `test/ipsec-interop/` `lab.py`, `lab_test.py`, scenarios 02, 04, 06, 12, 18
- Rounds 6 to 8: `eap/eap_tls.go`, `eap/peer.go`, `eap/eap_tls_alert_flight_test.go`,
  `engine/child_policy_owner_test.go`, `rfc/short/rfc5216.md`
