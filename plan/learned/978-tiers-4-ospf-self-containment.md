# 978 - tiers-4: OSPF self-containment (nest ospfv3 under ospf/v3)

## Context

After the module-tier umbrella's Phase 3 (`plan/spec-tiers-0-umbrella.md`), the
OSPF/OSPFv3 feature landed (`plan/learned/955-975`). The Phase-1 placement gate
(`scripts/dev/dep_audit.py --check`, in `make ze-verify`) then went RED: it flagged
`internal/plugins/ospf` (an `sdk.NewWithConn` engine) to move to
`internal/component/`, because something under `internal/plugins/` imported the
`ospf` engine subtree -- `internal/plugins/ospfv3/transport` imported
`internal/plugins/ospf/wire` (the shared `RawPacket`). The gate's axis B is
path-based: any `.go` file outside a dir's own subtree importing into it counts as
"a feature depends on it." This was the umbrella's AC-8 (regression catch) firing.

## Decisions

- Chose to make the OSPF plugin self-contained (nest the v6 wire leaves under it)
  over relabelling `ospf` a `component`, because `ospfv3` is NOT a peer plugin: no
  `sdk.NewWithConn`, no top-level files -- just three leaf packages (`types`,
  `packet`, `transport`) plus one doctor-check registration, all consumed by the
  single unified `ospf` engine (learned 972). OSPF is one edge plugin whose code was
  split across two top-level dirs by history; self-containment is the fix.
- Chose `internal/plugins/ospf/v3/{types,packet,transport}` (keep three distinct
  packages) over folding into `ospf`'s existing v4 `packet`/`transport`/`types`
  (name collisions) or a single shared blob. Preserves learned 972's discipline
  (separate guarded leaves + one-way engine->leaf dependency); 972 chose package
  *separation* and dependency *direction*, NOT a top-level *location*, so nesting
  does not reverse it.
- Chose a deterministic relocation (FS move + boundary-safe quoted-path rewrite over
  `.go/.ci/.sh/.md`, skipping `plan/` historical specs and the generated `ai/`
  index) over `git mv` (forbidden from tooling) or hand edits.
- Chose to leave `ike` in `internal/component/`: it is an `sdk.NewWithConn("ike")`
  engine with two feature dependents (`component/web/page_vpn_ipsec.go`,
  `component/cmd/clear/doc.go`), so the engine rule keeps it a component (no move).

## Consequences

- The OSPF `RawPacket` back-edge is now internal to the `ospf` tree, so axis B finds
  no external feature depending on `ospf`; it correctly stays an edge plugin and the
  gate is GREEN (baseline empty, full engine enforcement, zero exceptions).
- A library that only the engine consumes belongs *inside* the plugin subtree, never
  as a top-level sibling under `internal/plugins/`. A top-level sibling that the
  engine imports trips the placement gate and reads as an independent plugin in the
  arch inventory.
- `ai/INSTRUCTIONS.md` arch lists regenerate with `ospfv3` dropped from the top-level
  `internal/plugins/` enumeration -- the inventory now matches reality (one OSPF).

## Gotchas

- Import-guard tests that forbid by SUBSTRING (`strings.Contains(p, "internal/plugins/ospf/")`)
  BREAK when you nest the guarded package under that very prefix: after the move the
  `packet` codec's legitimate `ospf/v3/types` import matched the forbidden
  `internal/plugins/ospf/` string (false positive). Rewrite such guards to an
  allow-list form: forbid the OSPF tree EXCEPT the one permitted leaf
  (`Contains(p, "internal/plugins/ospf/") && !HasSuffix(p, allowedLeaf)`).
- The generator has no hardcoded `ospfv3`; it discovers the doctor-registering
  `transport` package via the whole-tree `internal/plugins` scan, so a nest under
  `internal/plugins/ospf/v3/transport` is rediscovered and `all.go`'s blank-import
  set is preserved (0 dropped) -- but always regenerate + diff to prove it.
- The move surfaced a PRE-EXISTING, unrelated `test/.ci-sleep-baseline` drift: the
  committed count was 426 (the ratchet's own measure) but the baseline read 423 and
  had even been mistakenly decremented 424->423 by `a6338e2ec` without removing a
  sleep (`git log -S "time.sleep(" a6338e2ec^..HEAD` is empty). Corrected to 426 with
  user approval -- reconciling a stale baseline, not raising a ceiling for a new sleep.
- A functional test directory (`test/ospfv3/`) need NOT mirror the code path; it is
  organized by feature and keys on stable identifiers (the doctor code
  `doctor-ospfv3-raw-socket`), so only a path comment changed there.

## Files

- `internal/plugins/ospf/v3/{types,packet,transport}/` (moved from `internal/plugins/ospf/v3/`)
- `internal/plugins/ospf/v3/{packet,types}/imports_test.go` (guards rewritten for the nested layout)
- `internal/plugins/ospf/*_v6*.go`, `nssa.go`, `register.go`, `transport_iface.go` (import-path rewrites)
- `internal/component/plugin/all/all.go` (regenerated)
- `ai/INSTRUCTIONS.md`, `ai/CODE-TO-DOCS.md` (regenerated arch + doc index)
- `docs/architecture/{core-design,wire/ospfv3}.md`, `docs/guide/ospf.md`, `docs/research/ospf-implementation-guide.md`, `scripts/evidence/qemu-all-tests.sh`, `test/ospfv3/ospfv3-doctor-raw-socket.ci` (path refs)
- `test/.ci-sleep-baseline` (423->426 reconciliation), `plan/spec-tiers-0-umbrella.md` (Phase 4 record)
