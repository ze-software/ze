# 1296 -- rfcgate-2 -- wire-level evidence for RFC requirements

## Context

Ze binds RFC MUST-level requirements to tests with an `RFC requirement: <id> <polarity>` comment,
and `make ze-rfc-check` treats the tag as proof. Before this spec that proof was ~99.8% in-process:
2582 tag lines in Go unit tests against 4 in `.ci` files, with zero in interop scenarios and zero
in editor tests. So Ze's public compliance claims rested almost entirely on evidence a peer never sees,
which `ai/rules/integration-completeness.md` says outright cannot prove a feature is reachable.
The naive fix -- tag the interop scenarios -- would have been strictly worse than the unit bindings
it replaced: the interop runner failed OPEN on a missing Docker (`sys.exit(0)`) and no automated
pipeline ran it at all, so a tag there would have satisfied the gate with evidence nothing executes.
The goal was therefore ordered: make non-unit evidence execute first, only then let the scanner see
it, then make the ledger show how strong each piece is, then ratchet it so it can only rise.

## Decisions

- Execution (Phase 1) strictly before scanning (Phase 2), over landing the scanner first and wiring CI after: a tag in an unexecuted carrier retires real proof in favor of a claim.
- Interop lands as an ADVISORY nightly job, over a blocking `ze-verify` stage: blocking would put Docker on every developer machine and slow the merge gate (owner decision).
- A tag in an `unrun` carrier (ipsec/l2tp/pppoe) is a hard error, over accepting it and marking it "unrun" in the ledger: a marker is a note, a refusal is a guard (`ai/rules/fail-closed-guards.md`).
- Evidence carries two independent axes, **kind** (which layer it exercises) and **tier** (whether anything runs it), over one flat "proven" cell: flattening them is how "we have interop coverage" becomes true and worthless in the same sentence.
- Two monotonic counters, verify-tier and nightly-tier, over one non-unit counter: a single counter lets a `.ci` binding be swapped for a nightly interop binding at equal count, which is a silent downgrade.
- `check.py` is scanned by Python **tokenization**, over a regex line scan: only the tokenizer can tell a comment from a `#` inside the quoted protocol text these files are full of.
- `.et` reuses `scan_ci_tags`, over a third scanner: `.et` genuinely has `terminator=` blocks (163 of 164 files), so reuse keeps one implementation of the trap.
- `.et` ships as a fully supported carrier with NO seed binding, over manufacturing an editor-visible obligation to exercise the plumbing: a test written for the gate rather than for the requirement is the exact vacuity the spec spends R-3 and AC-17 guarding against.
- The wire-visible classifier (at least 76% of 2720 gated MUSTs) is a sizing input only and gates nothing, over building a gate on it: ~97% precision but poor recall would manufacture obligations for the mis-classified minority.
- Prefer `.ci` bindings over interop bindings when a behavior is reachable from both: a `.ci` runs inside `ze-verify` on every push, interop is nightly and advisory (owner decision, AC-18).

## Consequences

- The carrier table `CARRIERS` is the single source for the tree scan, the git-HEAD baseline, the tolerant scan, the ledger and the ratchet; no literal suffix check survives in the module, so extending one filter alone can no longer corrupt the ratchet.
- `ai/RFC-REQUIREMENTS.md` now labels every test link `kind/tier` and carries a `nightly-only` subset column, so a reader can tell merge-gate proof from scheduled-advisory proof without opening the pipeline.
- **Tier is asserted, not derived.** `CARRIERS` calls `.ci` verify-tier because a human read `stagesForMode`; nothing re-derives it, and the ratchet baseline is re-labeled with the same table it is meant to police, so a one-word tier edit moves every claim in the repo without reddening anything.
- The ledger states a pipeline CLASS, not a RESULT: `interop/nightly` says the proof is scheduled and advisory, never whether the last nightly ran or was green.
- The three other interop trees (ipsec 10, l2tp 3, pppoe 1 scenarios) stay `TIER_UNRUN` and refuse tags until someone wires them into CI (`plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`).
- The ~2571 unit-only requirements are untouched; the back-fill remainder is tracked, not implied (`plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md`).
- Interop is now executable-by-default: `make ze-interop-test` exits non-zero without Docker, so a laptop run can no longer report success over an absence.

## Gotchas

- **Closing a spec breaks a gate that says nothing during closure.** The two-commit model `git rm`s the spec while sibling specs still cite it, and `make ze-spec-citation-check` then fails with "dangling `plan/spec-*.md` references" -- 35 of them here, all pointing at `plan/spec-rfcgate-1-extraction.md` from the umbrella and from child 4. Nothing in the closure path warns about this: it surfaced two children later, inside an unrelated `ze-verify-changed`, and reads as a mystery structural red in whatever work happens to be in flight. The sanctioned fix is `python3 scripts/dev/spec-citation-check.py --write-baseline` (the tool's own help documents exactly this case), and it belongs in the CLOSURE commit that removes the spec, not in the next session's. Check `plan/.citation-baseline` in the same breath as `git rm plan/spec-*.md`.
- **A test's green can depend on a bug.** Interop scenario 47 passed on 2026-07-20 because the announce rail copied attribute blocks through verbatim -- itself an RFC 4271 Section 5.1.2 defect, since the receiver's loop detection could not see Ze behind it. `8bb55e509` fixed that on 2026-07-25, the scenario went red, and the red was read as a NEW route-server transparency defect. Two agents wrote a pre-fix architecture into the scenario as current fact before a third traced it.
- **The real cause was config, not code:** `rs-client` defaults to false (`internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang`) and NO interop scenario set it, so Ze correctly prepended for what was, as configured, a plain eBGP peer. Both rails share ONE prepend gate, `if facts.isEBGP && !facts.rsClient` (`internal/component/bgp/reactor/reactor_api_forward.go:711`) -- there is no replay-vs-forward split to fix.
- **Read the producing gate, not the symptom.** A whole destination spec, two deferral rows and a "fix RS AS-path transparency on the replay rail" task were filed against a defect that does not exist. One grep of the gate would have refuted all of it.
- **Evidence tier claimed by file extension is a guard that fails open.** `.ci` and `.et` originally earned `functional/verify` from their suffix alone, which credited `test/draft/` -- the mandated incubator for unfinished tests -- and ~59 `.ci` files in suites `ze-verify` never runs, with three demonstrated moves that let a tagged test leave a running suite while the ratchet stayed silent. The fix derives the tier from `mk/test-functional.mk`'s own `all_suites=` line.
- **A ratchet that re-labels its own baseline cannot detect a downgrade.** The HEAD baseline was being labeled with TODAY's carrier table, so flipping a tier from `verify` to `nightly` rewrote both sides symmetrically and no gate reddened.
- **"Sibling audit complete" was written before the audit.** The launch-form fix covered four Docker labs and claimed in-comment that it covered every invocation site; eight executable sites remained on the removed `ze <config>` form, including the QEMU twins of the very labs it fixed and the shipped `docker/compose.yaml`.
- **A NOTIFICATION left no trace in the log.** Ze emitted a counter, a Prometheus label and a report-bus entry, but nothing on stderr, so an operator saw a session drop with no reason and a `.ci` rule asserting "ze did not answer with a NOTIFICATION" could only fail by timeout. The fix gave the daemon the log line the operator was already missing, rather than retuning the assertion to whatever the code happened to print.
- **Coverage of a dead path reads exactly like coverage of a live one.** The incidental SIGTERM/Cease finding: `Session.Close()` builds a Cease NOTIFICATION with no reachable production caller, so a peer observes a bare TCP FIN. Homed in `plan/spec-fixit-bgp-shutdown-cease-notification.md`.
- **A wrapped bullet does not survive condensation.** `scripts/dev/rules_condensed.py` keeps only a list item's FIRST physical line; five `ai/rules/testing.md` directives reached `CONDENSED.md` truncated, and the unrun-carrier directive lost its verb entirely, so the digest said a tag in those trees *is*, and stopped. Regenerating is not verifying -- read your section in the regenerated digest.
- **`verdict_is_fresh` is exact equality on the whole `tests` map**, so ADDING a tag stales an audit verdict about assertions nobody touched. `rfc/audit/rfc7606.json` was hand re-stamped for the sixth time (F18 in `plan/learned/HOOK-FRICTION.md`); `plan/spec-rfcgate-3-audit-teeth.md` owns the pattern.
- **One deferral row held two separable items and only one landed.** Row 7 read "give `RFC7947-x-1` non-unit evidence AND add an rs-client relay test"; the evidence half closed here, the relay test does not exist (`reactor_api_relay_test.go` has no `rsClient` reference). Marking the row done wholesale would have silently dropped a real test -- write one item per row.
- **Closure `file:line` citations rot inside the closure pass itself.** `scripts/dev/rfc_requirements.py` was uncommitted and grew 3418 -> 4180 lines while the spec was being closed, moving `CARRIERS` from `:641` to `:720` and `render_ledger` from `:1977` to `:2239`. Every cited symbol and all 29 cited tests still existed; only the numbers moved. Anchor on the symbol name and re-verify at closure.

## Files

- `scripts/dev/rfc_requirements.py` -- `CARRIERS` table, tier constants, `carrier_for`, `scan_python_tags`, `.et` routing, both extension filters, `check_evidence_ratchet`, ledger `kind/tier` cell and `nightly-only` marker.
- `scripts/dev/rfc_requirements_test.py` -- created; the carrier, scanner, ledger and ratchet unit tests (361 selftest cases total).
- `test/interop/run.py` -- Docker probe fails closed; `test/interop/run_test.go` created to pin it.
- `.github/workflows/evidence-nightly.yml` -- advisory `interop` job; pinned by `TestEvidenceNightlyRunsInterop` in `scripts/dev/github_workflows_test.go`.
- `ai/RFC-REQUIREMENTS.md` -- regenerated with the tier column and rollup; `rfc/audit/rfc7606.json` re-stamped.
- Seed bindings: `test/plugin/rfc7606-relay-one-field.ci` (`RFC7606-5.1-2`, `RFC7606-5.1-3`), `test/interop/scenarios/14-route-server-frr/check.py` (`RFC7947-x-1`), `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py` (`RFC7606-5.1-3`), plus `session/rs-client true` in both scenarios' `ze.conf`.
- `internal/component/bgp/reactor/session_write.go` -- logs `notification sent` at WARN, the missing operator signal.
- Discovery surfaces: `ai/rules/testing.md` (+ regenerated `ai/rules/CONDENSED.md`), `ai/rules/rfc-compliance.md`, `docs/features/rfc-status.md`, `docs/functional-tests.md`.
