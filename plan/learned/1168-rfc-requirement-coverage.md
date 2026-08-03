# 1168 -- rfc-requirement-coverage

## Context
Built a two-layer system binding every RFC 2119 MUST in `rfc/short/*.md` to the tests that
enforce it: a mechanical gate (`make ze-rfc-check`) that proves a positive AND a negative
tagged test exist per gated requirement, and a semantic audit (`/ze-rfc-audit` ->
`rfc/audit/<rfc>.json`) that reads the RFC against each tagged test and judges whether it
would fail on non-compliance. RFC 7606 is the pilot. See also
[[1166-rfc-clause-map-needs-producers]].

## Decisions
- The gate proves a LINK; the audit proves SUBSTANCE. Keep them separate. The pilot audit's
  first pass turned up 6 `weak` verdicts on requirements the gate reported green: §5.3-3/4/5
  (MP NLRI length/overrun/flags), §7.8-1 (zero-length COMMUNITY), §2-5 (Adj-RIB-In removal).
  Each was a real hole the mechanical gate structurally cannot see.
- Fix `weak` at the source, do not record it as debt (user direction for this pilot). Two
  shapes: (a) the CODE did not enforce it -> implement the check (§5.3); (b) the TEST did not
  isolate it -> rewrite the test to pin the rule (§7.8, §2-5, and the §5.3 negatives).
- Scope a partial enforcement honestly. The new MP NLRI check (`validateMPNLRISyntax`) runs
  only for plain-prefix families (IPv4/IPv6 unicast/multicast) and is permissive for
  typed/labeled NLRI, mirroring `validateMPReachNextHop`'s permissive-on-unknown pattern --
  a naive prefix walk would misread EVPN/VPN NLRI and reject valid routes.
- The generated ledger's freshness gate must live where verify runs. `run_check` (in both
  `stagesForMode` branches) got `check_ledger_fresh`; `ze-doc-test` got a lighter
  `--check-fresh`. `ze-doc-test` alone was not enough because it is NOT in `stagesForMode`.

## Consequences
- Final pilot: 52 gated MUSTs, 44 `enforced`, 8 `unimplemented` (the disclosed `{gap}`s),
  0 `weak`, 0 `wrong`. `validateMPReachAttr`/`validateMPUnreachAttr` now enforce §5.3-3/4/5;
  `validateAttributeFlags` rejects MP attributes that are not RFC 4760 optional non-transitive.
- `ai/RFC-REQUIREMENTS.md` renders deterministically (citations sorted by file,line; walk
  sorted) so the freshness gate cannot false-flag across machines.
- A new genuine §2-5 test observes the Adj-RIB-In directly
  (`adj_rib_in.handleReceivedStructured` + `message.SynthesizeWithdraw`), not the dispatch
  shape the reactor tests already prove for §2-1.

## Gotchas
- **A negative test that trips a NEIGHBORING rule proves nothing.** The §5.3-4/5.3-5
  negatives set next-hop length 0, which tripped the §7.11 next-hop rule and returned
  SessionReset -- the right action for the wrong reason. Isolation test: flip the one defect
  (make it conforming); the verdict must flip to `none`. If it stays failing, the buffer is
  cascade-confounded. Give the negative a VALID everything-else (here a valid 16-octet IPv6
  next hop) so the rule under test is the only thing wrong.
- **The audit fingerprint is a whole-FILE sha, so it over-triggers.** Editing any test in
  `rfc7606_test.go` restales EVERY requirement tagged in that file -- a batch of test edits
  produced 40 stale-verdict gate errors at once. This is correct (over-trigger is safe,
  under-trigger ships a rotted verdict); re-issue the whole audit via the assembler, which
  recomputes shas live from the current tree. Keep verdicts you did not substantively change
  as-is; only the fingerprint refreshes.
- **A `weak` that is really `unimplemented`.** `validateMPReachAttr` "checked" §5.3 by only
  testing `length < 5` + next hop -- it never parsed the NLRI. `validateAttributeFlags`
  returned nil for any code not in `wellKnownAttrs`, and MP codes 14/15 are optional, so MP
  flags were never checked. Both read as "weak test" but were code gaps. Read the producer
  before deciding which.
- **`ze-doc-test` is not in `stagesForMode()`.** A staleness check placed only there never
  fires at commit-time verify -- which is exactly how this ledger drifted once (two commits
  re-tagged tests without regenerating it). Put freshness where verify actually runs.
- **A byte-walking NLRI validator must be ADD-PATH aware.** The §5.3 NLRI-syntax walk read
  a bare `(len, prefix)` list, but under RFC 7911 ADD-PATH each NLRI is prefixed with a
  4-byte Path Identifier. Without skipping it, a path-id octet is read as a prefix length and
  a VALID UPDATE is session-reset -- a valid-input reset (DoS-adjacent). The receive context
  answers `AddPathFor(family)`; thread it in (`ValidateUpdateRFC7606AddPath`, nil = no
  add-path so existing test callers are unchanged). The pre-existing IPv4 NLRI check had the
  same blindness -- when you add a byte-walk over NLRI, check every negotiated capability that
  changes the byte layout, not just the family's prefix maximum.
- **Shared working tree.** A second session's uncommitted changes (ospf/isis/traffic/CLI)
  polluted `make ze-lint-changed` and `make ze-verify`. Lint/test your OWN packages directly
  (`golangci-lint run ./internal/component/bgp/message/... ...`) and commit only your files
  via a scoped commit script; never `git add -A`.
- **Editing an RFC-tagged test needs the approval token.** `// rfc-test-change-approved:
  <date> <what>` (the hook's `_RFC_APPROVED`). Comment/tag-only edits are allowed without it;
  adding an assertion or changing bytes is not. Removing a tag is a tag edit (allowed), but
  the GATE then fails if the requirement loses a polarity -- so add the replacement first.

## Files
- Modified: `scripts/dev/rfc_requirements.py` (sort render + `scan_tree`; `check_ledger_fresh`
  + `--check-fresh`; `_collect_for_check`; dropped dead `load_all`)
- Modified: `internal/component/bgp/message/rfc7606.go` (§5.3-3/4/5 MP NLRI + flag
  enforcement; `validateMPNLRISyntax`; §7.8 clause naming; immediate flag session-reset)
- Modified: RFC 7606 tests -- `rfc7606_test.go`, `rfc7606_withdraw_test.go`,
  `rfc7606_structural_test.go`, `reactor/session_validate_test.go`,
  `reactor/session_test.go`, `reactor/session_rfc7606_structural_test.go`
- Created: `internal/component/bgp/plugins/adj_rib_in/rib_test.go`
  `TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute` (genuine §2-5)
- Created: `rfc/audit/rfc7606.json` (52 verdicts)
- Modified: `mk/inventory.mk` (`ze-doc-test` ledger freshness); docs
  (`rfc-implementation-guide.md` §9.7, `functional-tests.md`, `repo-maintenance.md`)
