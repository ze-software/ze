# structural-review-fixes

Full structural review (dev-system, architecture, testing, rule adherence)
followed by a repo-wide fix pass, executed with six parallel subagents on
disjoint file sets.

## Core lesson

Enforcement works; hand-maintained text rots. Hook-enforced rules measured
99%+ adherence while honor-system rules leaked, and the auto-loaded
instruction layer itself had drifted (components that no longer exist,
renamed abstractions, dead pointers). Every fix that matters got a generator
or a checker, not a correction: architecture lists in `ai/INSTRUCTIONS.md`
are generated (`scripts/dev/arch_map.py`), corpus path references and
`// Design:` targets are checked (`scripts/dev/check_doc_links.py`, wired
into `ze-regen-check`/`ze-doc-links`), generated agent files get content
comparison (`skill_sync.sh --check`, session-start warning) because
`git diff` can never see gitignored drift.

## Mechanisms added

- `commit_helper.py learned-next <slug>`: collision-free learned numbering
  (max(existing, counter)+1, file created at allocation).
- Verify status records `mode=` and `skipped=`; `verify-status.sh check`
  reports `FRESH(<mode>)` and treats skipped-suite passes as STALE.
- `changed-pkgs.sh`: reverse-dependency expansion (importers of changed
  packages are retested) and buildable-package filter (`//go:build ignore`
  tool dirs broke `ze-lint-changed`). Go 1.26 trap: `go list -m` outside a
  module prints `command-line-arguments` and exits 0.
- Web suite fails hard (not skips) when agent-browser is missing under
  ZE_VERIFY_MODE; CI installs agent-browser.
- `.ci` sleep ratchet (`test/.ci-sleep-baseline`) and a functional-test diff
  advisory in `verify_wiring_docs.py`.
- Mutation history appended per run to committed `test/mutation/history.ndjson`.
- QEMU integration package list derived from `//go:build integration && linux`
  (the hardcoded list had already rotted: one dead path, two missing packages).

## Architecture corrections

- Peer-address maps in rib/adj_rib_in/weight-tracker converted to
  `map[netip.Addr]` with parse-once-at-boundary; BMP composite keys
  (`router:peerIP`) stay strings by design -- check actual producers before
  converting a "string key".
- Health checks and the radius doctor check moved to owning components
  (`report.HealthProbeDegraded/Down` helpers); BGP filter types extracted
  from generic plugin registry into `bgp/filterapi`; default event namespace
  is now a registration, not a hardcoded import.
- `cli/client`'s hand-maintained blank-import block was fully redundant with
  generated `plugin/all` -- check transitive registration coverage before
  maintaining import lists by hand.
- Spec closure now requires rewriting `// Design: plan/spec-*` references to
  the learned summary (74 of 153 design targets were dead because closure
  deleted specs without remapping).

## Process notes

- Six write-capable subagents on disjoint file sets worked; the two
  interaction bugs (a fixture expecting a now-derived sorted list, transient
  compile breaks while the rib conversion was mid-flight) were caught by the
  agents themselves polling for a compiling tree.
- The test-deletion hook cannot distinguish moved tests from deleted ones;
  moves need the new-location copy created first.

## Files

None recorded.
