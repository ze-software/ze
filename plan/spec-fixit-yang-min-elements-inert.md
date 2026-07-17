# Spec: fixit-yang-min-elements-inert

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

> **Status history:** `skeleton` (2026-07-16) → `ready` (2026-07-17, this design fill).
>
> **Design fill (2026-07-17):** Option (a) chosen as the autonomous default (see the
> resolution appended under `## Options`). Status promoted `skeleton` → `ready`. All
> `(fill during design)` placeholders filled from source, grounded in a live re-probe of
> the real `ze config validate` binary (see `### Ground-Truth Re-Verification (2026-07-17)`
> under `## Current Behavior`). The re-probe corrected two material facts the original
> skeleton missed: VRRP cardinality is already backstopped by the VRRP plugin verifier at
> `ze config validate`, and a committed functional test for the iface leaf-list bound
> already exists and does not hold against current code. R-5 is honored explicitly: this
> fix touches ONLY `ze config validate`; the daemon load path still runs no cardinality
> validation.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/config-surface.md`, `ai/patterns/config-option.md` - YANG constraint expectations
4. `internal/component/config/yang/validator.go` (`walkTree`, `checkCardinality`), `internal/component/config/tree.go` (`ToMap`), `internal/component/config/cli/cmd_validate.go` (`runValidation`)

## Task

YANG leaf-list cardinality (`min-elements` AND `max-elements`) is declared in the
schema but never enforced. `min-elements` can never reject anything; `max-elements`
can never reject anything on a leaf-list. `list` cardinality does enforce correctly,
which is why the gap was invisible.

This spec RECORDS the verified defect and its blast radius. The fix approach is
deliberately undecided: closing the gap would newly reject configs that load today,
so an operator upgrading Ze could find their box refusing its own running config.
Thomas approved recording now and deciding later. Do NOT implement from this
skeleton without an explicit decision on the Options table below.

Scope of this spec: the `min-elements`/`max-elements` enforcement gap in the YANG
validator only. The two live escalations it contributed to (the TACACS and RADIUS
empty-profile-mapping defects) were tracked and resolved in their own specs, since
closed and git-rm'd, and are NOT re-litigated here.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/config-surface.md` - YANG vs env var; what operators are promised by a YANG constraint
  → Constraint: "Default answer: YANG config." Config leaves are expected to be "validated by YANG constraints" and part of commit/rollback. A declared-but-inert constraint breaks that promise.
- [ ] `ai/patterns/config-option.md` - structural template for a config leaf
  → Constraint: every leaf must carry maximum native YANG validation. This spec records that one class of native constraint (leaf-list cardinality) is silently inert, so "it has a YANG constraint" is not evidence of enforcement.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - implement missing behavior at the source
  → Constraint: relevant to Option (b); deleting a declaration to make the schema honest is not the same as implementing the missing check.

### RFC Summaries (MUST for protocol work)
Not applicable: this is config-validation infrastructure, not protocol work. The
affected VRRP declarations trace to RFC 9568 Section 5.2.9, but this spec changes
no wire behavior.

**Key insights:**
- A YANG constraint in this repo is a claim, not a guarantee. Leaf-list cardinality is declared in 6 places and enforced in 0.
- `list` cardinality works; `leaf-list` cardinality does not. The two share `checkCardinality` but reach it by different paths.
- The gap is invisible to unit tests because the helper is correct in isolation. Nothing tested that `walkTree` ever calls it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/validator.go` - `walkTree` (:616), leaf-list branch (:668-693), list branch (:644-658), `checkCardinality` (:782)
  → Constraint: `checkCardinality` (:782-806) is CORRECT. Both the `MinElements` (:786) and `MaxElements` (:796) comparisons are right. The defect is entirely in reachability, not in the helper.
  → Constraint: `walkTree` (:632) is `for key, value := range data` — it iterates only keys PRESENT in the config data map. An absent leaf-list is never visited.
  → Constraint: the mandatory-child loop (:619-629) cannot compensate: it fires only on `child.Mandatory == yang.TSTrue`, and goyang never derives `Mandatory` from `min-elements`.
  → Constraint: the leaf-list guard (:669) is `if str, ok := value.(string); ok && str != ""`. Both the type assertion and the emptiness test skip the branch, `continue` at :692.
  → Constraint: the comment at :667-668 ("Bracket leaf-lists are stored as space-separated strings") is STALE. That is not the shape `Tree.ToMap` produces for a multi-member leaf-list.
- [ ] `internal/component/config/tree.go` - `ToMap` (:884), leaf-list rendering (:901-911)
  → Constraint: THIS is the producer of the shape `walkTree` consumes, and it renders a leaf-list three different ways by member count: 0 active members → key OMITTED (:904-905); 1 → `result[k] = active[0]`, a bare `string` (:906-907); 2+ → `result[k] = active`, a `[]string` (:908-909).
  → Decision: the three-way shape is what makes cardinality unreachable. The only count that survives the `value.(string)` assertion at validator.go:669 is exactly 1.
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation` (:240), `yangSectionsToValidate` (:50-54), the only production caller of the YANG tree validator (:277)
  → Constraint: `ValidateTreeAllModules` has exactly ONE non-test caller in the tree. YANG tree validation runs on the `ze config validate` path ONLY.
  → Constraint: `yangSectionsToValidate` (:51-53) lists `interface, sysctl, fib, plugin, web, ssh, dns, telemetry, looking-glass, mcp, managed, vpp, vpn, pki, l2tp, isis, ospf`. It does NOT list `tacacs`, `as112`, or `geodns`, so those sections get no YANG tree validation on any path.
- [ ] `internal/component/config/loader.go` - `ParseTreeWithYANG` (:76), `parseTreeWithYANG` (:89-131)
  → Constraint: the config LOAD path does schema build → parse → `PruneInactive` and returns. It NEVER calls the YANG tree validator. No cardinality check of any kind runs at config load, for lists or leaf-lists.
- [ ] `internal/component/config/yang/validator_test.go` - `TestCheckCardinality` (:35-74)
  → Constraint: the table includes `{"exactly one but zero", 1, 1, 0, true, "too few"}` (:51) and it PASSES. The helper provably handles count==0. The test constructs `&gyang.Entry{ListAttr: ...}` by hand (:57-62) and calls `checkCardinality` directly (:64) — it never goes through `walkTree`, so it cannot observe that the call site is unreachable.
- [ ] `github.com/openconfig/goyang@v1.6.3/pkg/yang/entry.go` - `Mandatory` assignment (:613), `LeafList` case (:616-653)
  → Constraint: `e.Mandatory` is set from `s.Mandatory` for a `*Leaf` only (:613). The `*LeafList` case (:616-653) synthesizes a leaf carrying Name/Type/Config/Description but NO Mandatory, then sets `ListAttr` (:636-646). `Entry.Mandatory` for a leaf-list is therefore always `TSUnset`, regardless of `min-elements`.

**Behavior to preserve:**
- `checkCardinality` semantics: `MinElements > 0 && count < MinElements` → too few; `MaxElements > 0 && count > MaxElements` → too many. The helper is correct; a fix must not touch it.
- `list` cardinality enforcement, which works today (verified below) and must keep working.
- `Tree.ToMap`'s three-way leaf-list shape is consumed by many readers beyond the validator. Changing `ToMap` to normalize leaf-lists is NOT a local change and is out of scope for a fix that only closes the validation gap.
- Every config that loads today must keep loading, unless a chosen option explicitly and knowingly changes that (see Upgrade Risk).

**Behavior to change:**
- ~~None. This is a skeleton. The defect is recorded; no fix is chosen.~~
- Superseded 2026-07-17 (Option (a) chosen): `walkTree`'s leaf-list branch (`validator.go:668-693`)
  gains the ability to count members from BOTH shapes `Tree.ToMap` emits — a bare `string`
  (1 member) and a `[]string` (2+ members, `tree.go:908-909`) — and to visit a declared
  leaf-list that is absent or empty when `MinElements > 0`, so `checkCardinality` (:782) fires.
  The stale comment at `:667-668` is corrected. Net effect: `ze config validate` newly rejects
  a leaf-list whose member count violates `min-elements`/`max-elements`, for any leaf-list that
  lives under a section in `yangSectionsToValidate`. Nothing else changes: `checkCardinality`
  is untouched (A-1), the LIST branch is untouched (A-5), `Tree.ToMap` is untouched (R-4), and
  the daemon load path (`loader.go:89-131`) still runs no validation (R-5).

### Verification Evidence (probe, 2026-07-16)

A temporary probe drove the REAL validator path (`config.YANGSchema` → `NewParser.Parse`
→ `PruneInactive` → `YANGValidatorWithPlugins(nil).ValidateTreeAllModules`), i.e. the
same call sequence as `runValidation` (`cmd_validate.go:240-277`). The probe was deleted
after use; a deletion script is at `tmp/delete-yang-min-elements-spec.sh` because the
`block-test-deletion` hook requires user approval to remove a `_test.go`.

| Probe case | VRRP declares | Result | Reading |
|-----------|---------------|--------|---------|
| `virtual-address [ 192.0.2.1 ]` (1 member) | min 1, max 16 | 0 errors | Correct, but only by accident: this is the ONLY shape that reaches `checkCardinality`, and count==1 trivially satisfies both bounds |
| `virtual-address` absent | min 1 | 0 errors | Mechanism 1: key omitted by `ToMap` (:904-905), never visited by `walkTree` (:632) |
| `virtual-address [ ]` (empty brackets) | min 1 | 0 errors | Mechanism 2: `ToMap` yields `""`; skipped by the `str != ""` guard (:669) |
| `virtual-address [ ...17 addrs... ]` | **max 16** | **0 errors** | Mechanism 3: `ToMap` yields `[]string`; the `value.(string)` assertion (:669) FAILS, branch skipped at :692 |
| **Contrast:** `sysctl` with 51 `profile` entries | list, max 50 | **1 error:** `sysctl/profile cardinality too many entries: 51 (maximum 50)` | The LIST branch (:658) reaches `checkCardinality` correctly. The helper and the error plumbing both work |

The tree map shape was dumped directly to confirm the producer:
17 members rendered as `"virtual-address":[]string{"192.0.2.1", ...}`;
1 member as `"virtual-address":"192.0.2.1"`; empty brackets as `"virtual-address":""`;
absent as no key at all.

**Correction to the reported characterisation.** The originally reported framing
(two mechanisms; `checkCardinality` only ever invoked with count >= 1) is directionally
right and its conclusion holds, but it is incomplete and understates the defect:

| Reported | Verified |
|----------|----------|
| Mechanism 1: `walkTree:632` iterates only present keys, so an absent leaf-list never reaches `checkCardinality` | CONFIRMED at `validator.go:632`, with the producing shape at `tree.go:904-905` |
| Mechanism 2: the guard at `:668` is `if str, ok := value.(string); ok && str != ""`, so an empty string is skipped | CONFIRMED in effect, at `:669` (`:668` is the enclosing `if child.IsLeafList()`) |
| "`checkCardinality` is only ever invoked with a count >= 1" | Understated. It is only ever invoked with a count of EXACTLY 1. A leaf-list of 2+ members renders as `[]string` (`tree.go:908-909`), fails the `value.(string)` assertion at `:669`, and is skipped |
| "`min-elements 1` can never reject anything" | CONFIRMED, and **`max-elements` on a leaf-list can never reject anything either** — proven by the 17-vs-16 probe. The defect is leaf-list cardinality entirely, not just `min-elements` |
| The earlier sibling-agent probe (`PROBE absent leaf-list err=<nil>`, `PROBE empty brackets err=<nil>`) via `config.ParseTreeWithYANG` | Results REPRODUCED but MISATTRIBUTED. `ParseTreeWithYANG` (`loader.go:76-131`) never invokes the YANG tree validator at all, so that probe could not have exercised `walkTree`. It shows a real and broader fact (the config LOAD path runs no cardinality validation), not evidence for the `walkTree` mechanisms. `TestCheckCardinality` passing plus this probe is a coincidence of two different gaps. |

The now-closed TACACS empty-profile-mapping spec's A-3 row recorded the same two
mechanisms and reached the same conclusion by the same misattributed probe. Its
conclusion (the code guard is the only load-bearing defence) is correct and
unaffected.

### Ground-Truth Re-Verification (2026-07-17, design fill)

Before filling from the skeleton, a host `ze` binary (`go build -tags 'ze_core ze_setup'`)
was built from current source and the REAL `ze config validate` entry point was driven
directly (not the isolated validator the 2026-07-16 probe used). Two facts the skeleton
missed changed the AC/Wiring shape and are recorded here APPEND-ONLY. The original
Verification Evidence stands; it isolated the YANG validator correctly. What it did NOT
run is `config.VerifyPluginConfig` (`cmd_validate.go:323`), which the full command does.

| Case (real `ze config validate`) | Exit | Producer of the error | Reading |
|-----------------------------------|------|-----------------------|---------|
| iface `sysctl-profile [ 11 members ]` vs `max-elements 10` | **0, "configuration valid"** | none | Leaf-list cardinality is INERT and has NO backstop. The committed test `test/parse/sysctl-profile-max-elements.ci` asserts exit 1 + "too many entries" for this exact input, so that assertion does NOT hold against current code. This is the clean, un-backstopped, in-section proof of the defect. |
| iface `sysctl-profile [ 10 members ]` | 0, valid | — | Boundary: 10 == max is accepted. Correct. |
| VRRP IPv4 group, `virtual-address` absent (`min-elements 1`) | 1 | **VRRP plugin verifier** `groups.go:496` "at least one virtual-address is required" | Already rejected TODAY at `ze config validate`, via the plugin verifier — NOT via YANG. The walkTree fix adds NO new rejection here. |
| VRRP IPv4 group, 17 addresses (`max-elements 16`) | 1 | **VRRP plugin verifier** `groups.go:499` "17 virtual-address entries exceed the maximum of 16" | Same: already rejected via the plugin verifier, not YANG. |
| VRRP IPv4 group, 2 addresses | 0, valid | — | Accepted (within bounds). |
| `sysctl` LIST, 51 `profile` entries vs `max-elements 50` | 1 | YANG walkTree LIST branch (`validator.go:658`) | "sysctl: sysctl/profile: too many entries: 51 (maximum 50)". LIST branch enforces (A-5 reconfirmed). |

**Consequences for this spec (all reflected in the fills below):**
1. **VRRP is a poor AC/Wiring target.** Its cardinality is already enforced by the VRRP plugin
   verifier (`register.go:64` → `plugin_verify.go:86-97` → `groups.go:492-500`) which runs at
   `ze config validate` (`cmd_validate.go:323`). AC-1..AC-5 as originally written already pass
   today via that path, so they cannot prove the walkTree fix. The walkTree-proving AC/Wiring
   target is **iface `sysctl-profile`** (`ze-iface-conf.yang:253-261`, `max-elements 10`), which
   is in `yangSectionsToValidate` (`interface`) and has NO plugin-verifier backstop for its count.
2. **The TDD red test already exists**, committed: `test/parse/sysctl-profile-max-elements.ci`.
   It was added by `1fa167072 (2026-04-13) feat(config): enforce YANG max-elements and
   min-elements in validator`, whose leaf-list count relied on `strings.Fields` over a
   space-separated string. A later change to `Tree.ToMap` (rendering 2+ members as `[]string`,
   `tree.go:908-909`) silently re-broke the leaf-list path and the test with it. The stale
   comment at `validator.go:667-668` is the fossil of that April shape.
3. **The upgrade risk (R-1) does NOT materialize at the `ze config validate` boundary.** The
   two shapes the skeleton feared (no-vip / 17-vip VRRP groups) are already rejected there. The
   only NEW `ze config validate` rejection the fix introduces, in the reachable in-section set,
   is iface > 10 `sysctl-profile` — and the committed test already demands exactly that. A repo
   grep found no other config/fixture with a > 10 `sysctl-profile` leaf-list.
4. **Potential duplicate VRRP reporting (design note).** `ze-vrrp-conf.yang:216` augments
   `/iface:interface/.../ipv4`, so the `virtual-address` leaf-list is grafted under the
   `interface` section that `walkTree` validates. IF `walkTree` reaches the augmented leaf-list
   (unverified — the parser rejected a bad-IP probe before `walkTree` ran), the fix will emit a
   second (YANG) cardinality error alongside the plugin verifier's existing one. The implementer
   MUST confirm reachability and decide dedup vs. accept (both are errors; the config is rejected
   either way). Recorded as R-7 below.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator config text (hierarchical or set format), via `ze config validate <file>` (the only path that reaches the YANG tree validator) or via `ParseTreeWithYANG` at daemon load (which does not).

### Transformation Path
1. `config.YANGSchema()` builds the schema from modules registered by `configyang.RegisterModule` in each owner's `yang/register.go` (VRRP: `internal/plugins/vrrp/yang/register.go:10-11`).
2. `NewParser(schema).Parse(input)` → `*Tree`. Leaf-list members land in `Tree.multiValues`.
3. `PruneInactive(tree, schema)` drops `inactive:` subtrees.
4. **Load path forks here and stops** (`loader.go:128-130`): returns the tree. No validation.
5. **Validate path only** (`cmd_validate.go:271-277`): `YANGValidatorWithPlugins(nil)`, then per section in `yangSectionsToValidate`, `tree.GetContainer(section).ToMap()`.
6. `Tree.ToMap` (`tree.go:884`) renders leaf-lists three ways by active-member count (:901-911). **This is where cardinality information is destroyed:** a 0-member leaf-list becomes indistinguishable from an absent one, and count is no longer recoverable by a `string`-typed reader.
7. `Validator.ValidateTreeAllModules` → `walkTree` (:616) → leaf-list branch (:668) → `checkCardinality` (:782), reached only when the value is a non-empty `string`, i.e. exactly one member.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Parser ↔ Tree | leaf-list members stored in `Tree.multiValues` | [ ] |
| Tree ↔ Validator | `Tree.ToMap()` → `map[string]any`; leaf-list count collapsed to a 3-way shape (`tree.go:901-911`) | [ ] |
| Validator ↔ goyang | `*yang.Entry.ListAttr.{Min,Max}Elements`; `Entry.Mandatory` never set for a leaf-list | [ ] |
| CLI ↔ Validator | `ze config validate` only; the daemon load path never crosses this boundary | [ ] |

### Integration Points
- `checkCardinality` (`validator.go:782`) - already correct, already called correctly from the LIST branch (:658). A fix wires the leaf-list branch to it, it does not add a checker.
- `Tree.ToMap` (`tree.go:884`) - the shape producer. Any fix must read count from a shape that still carries it.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — no new per-feature field, switch case, or factory in a core/shared package

## Blast Radius

Every leaf-list cardinality declaration in the repo
(`grep -rn "min-elements\|max-elements" --include="*.yang" internal/`).
All 6 leaf-list rows are inert. Both `list` rows enforce.

### `min-elements` (3 declarations, all inert)

| # | Declaration | Kind | Intends to constrain | What actually happens today |
|---|-------------|------|---------------------|----------------------------|
| 1 | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang:56` — `leaf-list virtual-address` (IPv4), `min-elements 1`, `max-elements 16` | leaf-list | A VRRP IPv4 group MUST advertise at least one virtual address (RFC 9568 Section 5.2.9); at most 16 | Neither bound fires. A group with no `virtual-address`, or `[ ]`, validates clean (probed). 17+ addresses validate clean (probed). |
| 2 | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang:165` — `leaf-list virtual-address` (IPv6), `min-elements 1`, `max-elements 16` | leaf-list | Same for IPv6; the first address MUST be link-local (enforced separately by the plugin verifier, not by YANG) | Same as #1. Inert on both bounds. |
| 3 | `internal/component/tacacs/yang/ze-tacacs-conf.yang:91` — `leaf-list profile`, `min-elements 1` | leaf-list | A TACACS+ privilege level mapped to zero profiles is meaningless: it authenticates the user while granting nothing | Inert twice over. Not only is the leaf-list branch unreachable, `tacacs` is not in `yangSectionsToValidate` (`cmd_validate.go:51-53`), so the section is never handed to the validator on any path. Defence is the code guard in `handlePass` (recorded in the now-closed TACACS empty-profile-mapping spec). |

### `max-elements` (5 declarations: 3 inert leaf-lists, 2 working lists)

| # | Declaration | Kind | Intends to constrain | What actually happens today |
|---|-------------|------|---------------------|----------------------------|
| 4 | `internal/plugins/as112/yang/ze-as112-conf.yang:119` — `leaf-list community`, `max-elements 32` | leaf-list | At most 32 communities | Inert. Also: the as112 section is not in `yangSectionsToValidate`. |
| 5 | `internal/plugins/geodns/yang/ze-geodns-conf.yang:90` — `leaf-list nameserver`, `max-elements 9` | leaf-list | At most 9 nameservers | Inert. Also not in `yangSectionsToValidate`. |
| 6 | `internal/component/iface/yang/ze-iface-conf.yang:255` — `leaf-list sysctl-profile`, `max-elements 10` | leaf-list | At most 10 sysctl profiles per unit | Inert, despite `interface` BEING in `yangSectionsToValidate`. This is the cleanest proof that section coverage is not the cause: the section is validated, the leaf-list is not. |
| 7 | `internal/component/sysctl/yang/ze-sysctl-conf.yang:40` — `list profile`, `max-elements 50` | **list** | At most 50 sysctl profiles | **ENFORCES.** Probed: 51 profiles → `sysctl/profile cardinality too many entries: 51 (maximum 50)`. |
| 8 | `internal/component/sysctl/yang/ze-sysctl-conf.yang:53` — `list setting`, `max-elements 50` | **list** | At most 50 settings per profile | Enforces (same list branch, :658). Not separately probed. |

Rows 7-8 are why this survived: cardinality visibly works, so "YANG checks cardinality"
is true enough to pass a spot check and false exactly where it matters.

## Upgrade Risk (why the fix was deferred)

Closing the gap makes previously-accepted config newly invalid. Ze's config is the
appliance's running state; an operator who upgrades and reboots could have the box
refuse its own saved config. Concretely, per declaration:

| # | Declaration | Config shape that loads today and would newly FAIL | Realistic? |
|---|-------------|--------------------------------------------------|------------|
| 1 | vrrp IPv4 `virtual-address` min 1 | `vrrp { group lan { vrid 1; } }` — a group with a VRID and no virtual address, or with `virtual-address [ ]` | **Yes.** Named as the concrete example. A half-configured group is a natural intermediate state an operator can save and commit. Today it is accepted. |
| 2 | vrrp IPv6 `virtual-address` min 1 | Same shape under `ipv6` | Yes, same reasoning |
| 1,2 | vrrp `virtual-address` max 16 | A group with 17+ virtual addresses | Unlikely but possible; would newly fail |
| 3 | tacacs `profile` min 1 | `tacacs-profile { level 9; }` — a level mapped to no profiles | **Yes**, and this is exactly the escalation the now-closed TACACS empty-profile-mapping spec addressed. Note the code guard now DENIES this at runtime, so enforcing it at load turns a silent runtime denial into a loud load failure. That is arguably the desired outcome, but it is still a newly-rejected config. |
| 4 | as112 `community` max 32 | 33+ communities | Unlikely |
| 5 | geodns `nameserver` max 9 | 10+ nameservers | Plausible on a large deployment |
| 6 | iface `sysctl-profile` max 10 | 11+ sysctl profiles on one unit | Plausible |

Rows 1 and 2 carry the real risk: a VRRP group with no virtual address is both the
most likely shape in the wild and the one an operator is most likely to have saved
while mid-edit. A fix that lands without a migration story converts that into a
boot-time config rejection.

Aggravating factor: because the LOAD path never validates (`loader.go:89-131`), a
fix confined to `walkTree` changes only `ze config validate`, not daemon load. That
narrows the upgrade risk substantially — but it also means the fix does not actually
protect the daemon, which is the thing the escalations cared about. Any option that
claims to "close the gap" must state which of the two paths it closes.

## Options (NOT decided — for Thomas)

| Option | What it does | For | Against |
|--------|-------------|-----|---------|
| **(a) Fix `walkTree` so cardinality enforces** | Make the leaf-list branch read count from a shape that preserves it, and visit declared-but-absent leaf-lists with `MinElements > 0`. Both bounds start firing for all 6 leaf-lists. | The constraint means what it says. `max-elements` starts working too (currently a silent, unreported hole). Uses `checkCardinality` as-is; the LIST branch already proves the pattern. Every future `min-elements` is live for free. | Newly rejects config that loads today (see Upgrade Risk). Needs a migration story. The count-preserving read is not free: `ToMap`'s 3-way shape (`tree.go:901-911`) cannot distinguish 0 members from absent, so a faithful fix likely needs a count source other than `ToMap`, which widens the change beyond the validator. |
| **(b) Delete the declarations** | Remove all 6 leaf-list cardinality declarations; rely on code guards (TACACS `handlePass`, the VRRP plugin verifier). | The schema stops lying. Nothing looks like a constraint it isn't. Zero upgrade risk. Honest today. | Loses declared intent and the operator-facing `description` rationale. Contradicts `ai/patterns/config-option.md` ("maximum native validation"). Directly reverses the TACACS decision from days ago. Discards `max-elements` semantics that a future validator fix would have honoured, and leaves nothing to re-enable. Arguably a workaround for missing behavior (`ai/rules/no-workarounds-for-missing-behavior.md`). |
| **(c) Enforce only newly-added declarations** | Gate enforcement per declaration (e.g. a `ze:enforce-cardinality` extension, or an allowlist), so new constraints bite and existing ones stay advisory. | No upgrade risk for existing config. Lets new leaves get real validation immediately. | A YANG constraint whose enforcement depends on a side-channel is worse than either honest state: two classes of `min-elements` that look identical. Permanently institutionalizes the bug. Needs new extension machinery + registration. Nobody will ever migrate the grandfathered set, so the split is forever. |

**Recommended: (a), staged — but the recommendation is conditional and the decision is Thomas's.**

Reasoning:
- (b) is the wrong direction. The precedent set days ago by the now-closed TACACS
  empty-profile-mapping fix deliberately KEPT `min-elements 1`
  as declared intent, documented it as non-enforcing, and made the code guard load-bearing,
  explicitly so that it "becomes live for free if the `walkTree` gap is ever closed."
  Option (b) throws away the asset that decision was designed to preserve, and would
  require reversing a reviewed decision with no new argument beyond "it still does not work."
- (c) is the worst outcome: it makes the schema's meaning depend on a declaration's age.
- (a) is the only option that ends with the schema telling the truth. The `max-elements`
  finding strengthens it: this is not only about rejecting bad config, it is a live,
  unreported hole in bounds that already exist and that nobody knows are open.

Staging, because the upgrade risk is real and separable:
1. Fix `max-elements` on leaf-lists first. Lower risk (over-limit config is rare), no new rejection of the common half-configured shape, and it is a pure bug fix with no policy content.
2. Fix `min-elements` behind a warn-then-reject transition: emit a validation WARNING for a deficient leaf-list for one release, then promote to an error. `runValidation` already has a warning channel (`result.addWarning`, `cmd_validate.go:260-262`), so the mechanism exists.
3. Decide separately, and explicitly, whether the daemon LOAD path should validate at all (`loader.go:89-131`). That is the bigger latent question this defect exposed and it deserves its own spec, not a rider on this one.

The staging is a proposal, not a decision. Steps 1-3 are independently approvable.

→ **AUTONOMOUS DEFAULT (2026-07-17): Option (a), single-stage, error semantics, `ze config
validate` scope only. Thomas: override if wrong.**

**Decision.** Fix `walkTree`'s leaf-list branch (`validator.go:668-693`) so `checkCardinality`
(:782) is actually reached for leaf-lists, using the count `Tree.ToMap` already carries (bare
`string` → 1 member; `[]string` → N members; absent/`""` → 0 members), plus a declared-but-absent
scan for `MinElements > 0`. Both bounds enforce as **errors** (the LIST branch and the committed
`sysctl-profile-max-elements.ci` test both use error/exit-1 semantics). `checkCardinality`,
`Tree.ToMap`, and the LIST branch are all untouched (A-1, R-4, A-5).

**Why not the staged min-elements warn-then-reject (spec's own step 2):** the transition existed
to protect the daemon boot path from newly rejecting a saved config. But `walkTree` runs ONLY
under `ze config validate` (A-6), never at daemon load (`loader.go:89-131`), so no boot-time
rejection is introduced and the warn-phase protects nothing. The live re-probe (2026-07-17)
further shows the two feared shapes are already rejected at `ze config validate` by the VRRP
plugin verifier, so min-elements enforcement adds ZERO new `ze config validate` rejections for
the reachable, in-section set. Error semantics are therefore both simpler and safe.

**Why not (b):** discards the declared intent the now-closed TACACS fix deliberately preserved;
reverses a reviewed decision with no new argument; loses `max-elements` semantics. **Why not (c):**
institutionalizes the bug via a side-channel; needs new extension machinery.

**Explicitly OUT of scope for this spec (deferred, each to its own spec):**
- **Daemon load-path validation** (staging step 3; `loader.go:89-131`). R-5 honored: this fix does
  NOT make config load enforce cardinality. Any doc/claim to the contrary is false. Deferred.
- **Section coverage** (R-6): `tacacs`, `as112`, `geodns` are NOT in `yangSectionsToValidate`
  (`cmd_validate.go:50-54`) and this spec does NOT add them. Consequences: TACACS `min-elements 1`
  stays code-guarded only (its `handlePass` guard remains load-bearing); as112 `community` max 32
  and geodns `nameserver` max 9 stay unenforced. The walkTree fix makes them *ready* to enforce the
  instant their section is added, but adding the section is a separate decision.
- **`Tree.ToMap` normalization** (R-4): not changed.

**In scope (the deliverable):** the walkTree leaf-list fix + its unit test driven through `walkTree`
(AC-7) + turning the committed red `test/parse/sysctl-profile-max-elements.ci` green + a new
`.ci` case proving the iface leaf-list bound and (regression) the LIST bound.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `checkCardinality` is correct and needs no change | Read `validator.go:782-806`; `TestCheckCardinality` (`validator_test.go:35-74`) passes incl. the count-0 row (:51) | A fix would have to change the helper too, widening scope | Read + existing unit test | **confirmed** |
| A-2 | `walkTree:632` never visits an absent leaf-list, and no other check compensates | Read `validator.go:632`; mandatory loop at :619-629 keys off `child.Mandatory`; goyang `entry.go:613` sets `Mandatory` for `*Leaf` only, `:616-653` (LeafList case) never sets it | The gap would be narrower than recorded | Read of both producer and goyang; probe "absent" → 0 errors | **confirmed** |
| A-3 | A present-but-empty leaf-list is skipped by the `str != ""` guard at `:669` | Read `validator.go:669`; probe shows `virtual-address [ ]` → `ToMap` yields `""` | — | Probe "empty brackets" → 0 errors | **confirmed** |
| A-4 | A leaf-list of 2+ members is skipped because `ToMap` yields `[]string` and the `value.(string)` assertion fails | Read `tree.go:908-909` (producer) and `validator.go:669` (consumer); tree map dumped in probe | The `max-elements` finding would be wrong and Option (a) staging step 1 would be unnecessary | Probe: 17 addrs vs `max-elements 16` → 0 errors; map dumped as `[]string{...}` | **confirmed** |
| A-5 | `list` cardinality DOES enforce, so the defect is leaf-list-specific | Read `validator.go:658` (list branch calls `checkCardinality` with `len(subMap)`) | The defect would be all of `walkTree`, changing the fix shape | Probe: 51 sysctl profiles vs `max-elements 50` → error raised | **confirmed** |
| A-6 | The YANG tree validator runs ONLY on `ze config validate`, never at daemon load | `grep` for callers: `ValidateTreeAllModules` has one non-test caller, `cmd_validate.go:277`; `parseTreeWithYANG` (`loader.go:89-131`) returns after `PruneInactive` | The upgrade risk would be far larger (daemon would reject config at boot), and would change the recommendation | Grep of all callers + read of the load path | **confirmed** |
| A-7 | The comment at `validator.go:667-668` ("stored as space-separated strings") describes a shape `Tree.ToMap` never produces for multi-member leaf-lists | Read `tree.go:901-911`; probe dump shows `string` for 1 member and `[]string` for 17 | Some other producer may deliver that shape on another path, meaning the branch is not wholly dead | Probe dump of the production tree map. NOT exhaustively traced: the set-format parser (`parseSetWithMigration`, `loader.go:134`) and the web/gNMI tree readers were not probed | ~~**unvalidated**~~ → **resolved for Option (a) (2026-07-17): moot.** Option (a) RETAINS the single-member `string` branch (a bare string is a live 1-member shape) and ADDS a `[]string` branch; it does NOT delete the string branch, so "confirm no caller delivers a space-separated string" is no longer a precondition. The comment is corrected, not the branch removed |
| A-8 | Enforcing `min-elements` would newly reject real operator config | Probe shows `vrrp { group lan { vrid 1; } }` validates clean today; TACACS escalation documents `tacacs-profile { level 9; }` in the wild | The upgrade risk is theoretical and the fix could land unstaged | Probe of the absent-leaf-list shape. No survey of actual deployed configs was done | ~~**unvalidated**~~ → **resolved for Option (a) scope (2026-07-17).** Re-verification: the only min-elements leaf-lists reachable in a validated section are VRRP `virtual-address` (already plugin-backstopped, so no NEW rejection) and `tacacs profile` (section not validated, R-6). So enforcing min-elements adds ZERO new `ze config validate` rejections. The one new rejection is iface `max-elements` (> 10 sysctl-profile). Field configs remain unsurveyed, but they reach only `ze config validate`, never boot (A-6) — so a survey is not a precondition for this scope |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fix lands and an operator's saved config is rejected at upgrade | Functional tests break on configs that previously passed; field reports after release | ~~Staged warn-then-reject (Option (a) step 2).~~ **Superseded 2026-07-17:** re-verification shows this risk does NOT reach the daemon — `walkTree` runs only under `ze config validate`, never at boot (A-6, R-5). No warn-then-reject needed; enforce as error. The only newly-rejected `ze config validate` shape is > 10 `sysctl-profile` (AC-10), which a committed test already demands |
| R-2 | A future session reads `min-elements 1` in VRRP or TACACS YANG and assumes it is enforced, then removes the load-bearing code guard | A diff deletes a `len(...) == 0` guard citing the YANG constraint | This spec (and the R-2 row of the now-closed TACACS empty-profile-mapping spec) record the non-enforcement. The guards have direct tests that would fail |
| R-3 | The `max-elements` hole is silently relied upon: some config in the wild already exceeds a declared max and works | Enforcing `max-elements` breaks a config nobody knew was over-limit | ~~Staging step 1 should ship with a warning first.~~ **Superseded 2026-07-17:** in-repo grep found no config with a > 10 `sysctl-profile` leaf-list (the only reachable, un-backstopped max in a validated section); VRRP over-16 is already rejected by the plugin verifier. The reachable blast radius is empty beyond the committed test. Enforce as error. Field configs remain unsurveyed (A-8) — but they hit only `ze config validate`, not boot |
| R-4 | A fix to `Tree.ToMap`'s leaf-list shape to preserve count breaks unrelated consumers | Compile errors, or silent behavior change in web/gNMI/plugin readers that type-assert `string` | Do NOT change `ToMap`. Source the count elsewhere (e.g. `Tree.multiValues` / `activeMembersLocked`, `tree.go:901-902`) or pass the tree, not the map, to the validator |
| R-5 | Fixing `walkTree` creates false confidence that the daemon now validates cardinality, when only `ze config validate` does | A spec or doc claims config load enforces `min-elements` | Option (a) step 3 exists precisely to force that question to be answered separately and explicitly |
| R-6 | Sections not in `yangSectionsToValidate` (`tacacs`, `as112`, `geodns`) stay unvalidated even after a perfect `walkTree` fix | A fix closes the gap but the TACACS constraint still never fires | Recorded here. Any fix claiming to make TACACS `min-elements 1` live MUST also add `tacacs` to `yangSectionsToValidate` (`cmd_validate.go:50-54`), or say plainly that it does not. This spec chooses "does not" (see Options resolution). |
| R-7 | The fix emits a DUPLICATE cardinality error for VRRP: `walkTree` (YANG) fires on the augmented `virtual-address` leaf-list under `interface`, on top of the VRRP plugin verifier that already reports the same violation (`groups.go:496`/`:499`) | `ze config validate` on a no-vip or 17-vip VRRP group prints two errors for one defect | First CONFIRM reachability: `ze-vrrp-conf.yang:216` augments `interface`, so the leaf-list is in-section, but the 2026-07-17 bad-IP probe was caught by the parser before `walkTree` ran, so reachability is unproven. If reachable, either accept the duplicate (both are errors, config rejected either way — defense in depth) or suppress the YANG cardinality error where a plugin verifier owns the same leaf-list. Decide at implementation; do NOT let a duplicate-error surprise fail a VRRP `.ci` that asserts a single message |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- Filled 2026-07-17 for Option (a). The wiring proof MUST use a leaf-list with no
     plugin-verifier backstop, or it proves the plugin verifier, not walkTree. iface
     sysctl-profile is that target (in-section, un-backstopped). -->

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate -` on a unit with 11 `sysctl-profile` entries (`max-elements 10`) | → | `walkTree` leaf-list branch (`validator.go:668-693`) counts `[]string` members → `checkCardinality` (:782) | `test/parse/sysctl-profile-max-elements.ci` (already committed; currently RED — turns green on the fix) |
| `ze config validate -` on a unit with 10 `sysctl-profile` entries (boundary) | → | same branch, no error at count == max | `test/parse/sysctl-profile-max-elements-ok.ci` (new — boundary regression guard) |
| `TestWalkTreeLeafListCardinality` in-process on an iface `sysctl-profile` tree of 11 | → | `Validator.walkTree` → `checkCardinality`, driven from the tree entry (NOT a direct `checkCardinality` call) | `internal/component/config/yang/validator_test.go::TestWalkTreeLeafListCardinality` (new — AC-7) |
| `ze config validate -` on 51 `sysctl` `profile` list entries (`max-elements 50`, regression) | → | `walkTree` LIST branch (`validator.go:658`) → `checkCardinality` | `test/parse/sysctl-profile-max-elements.ci` (add a LIST case) or existing `sysctl` parse coverage |

~~The one row that is already known, whichever option is chosen:~~
Superseded 2026-07-17: the VRRP wiring row below is NOT a valid walkTree proof — VRRP
cardinality is already enforced at `ze config validate` by the VRRP plugin verifier
(`groups.go:496`/`:499`), so this row passes today regardless of the walkTree fix. Kept for
history; the load-bearing wiring is the iface `sysctl-profile` rows above.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ~~`ze config validate` on a VRRP group with no `virtual-address`~~ (already rejected by the plugin verifier, not walkTree) | → | VRRP plugin verifier `groups.go:496`, NOT `walkTree` | `test/vrrp/vrrp-config-invalid.ci` (existing; already exercises the plugin verifier) |

## Acceptance Criteria

~~**Provisional.** These describe the end state of Option (a). They are NOT approved
and must be revisited once an option is chosen.~~

**Resolved for Option (a) (2026-07-17).** The AC set is now approved with one reframe grounded
in the live re-probe: **AC-1..AC-5 are VRRP-based and are already satisfied TODAY by the VRRP
plugin verifier, NOT by the walkTree fix.** They are retained as regression guards (the fix must
not break them, and must not introduce a confusing duplicate — R-7), but the walkTree fix is
PROVEN by the iface-based AC-11..AC-13 below, because iface `sysctl-profile` has no plugin-verifier
backstop. AC-6 (LIST regression), AC-7 (walkTree-driven test), AC-8, AC-10 stand as written; AC-9
is answered "TACACS stays code-guarded only" (out of scope, R-6).

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config validate` on a VRRP group with `virtual-address` absent | Reports an error naming the group and "at least one virtual-address is required". **NOTE: already produced today by the VRRP plugin verifier (`groups.go:496`), not by the walkTree fix.** Regression guard only |
| AC-2 | `ze config validate` on a VRRP group with `virtual-address [ ]` | Same as AC-1 (already rejected by the plugin verifier). Regression guard only |
| AC-3 | `ze config validate` on a VRRP group with 17 addresses vs `max-elements 16` | Reports the max violation. **Already produced today by `groups.go:499`.** Regression guard only. After the fix, verify no confusing duplicate (R-7) |
| AC-4 | `ze config validate` on a VRRP group with 1 address | No cardinality error (regression guard) |
| AC-5 | `ze config validate` on a VRRP group with 2..16 addresses | No cardinality error (regression guard) |
| AC-6 | `ze config validate` on 51 `sysctl` profiles vs `max-elements 50` | Still reports `too many entries: 51 (maximum 50)` (LIST-branch regression guard; verified still firing 2026-07-17) |
| AC-7 | A test drives cardinality through `walkTree`, not by calling `checkCardinality` directly | Exists and fails against pre-fix code, passes after. This is the coverage gap that hid the defect (`TestWalkTreeLeafListCardinality`) |
| AC-8 | Every leaf-list `min-elements`/`max-elements` declaration in the tree | Has a test proving the bound fires (iface, AC-11..13), or an explicit recorded reason why it cannot (VRRP: plugin-backstopped; tacacs/as112/geodns: section not in `yangSectionsToValidate`, R-6) |
| AC-9 | TACACS `min-elements 1` | **Answered: TACACS remains code-guarded ONLY.** `tacacs` is deliberately NOT added to `yangSectionsToValidate` in this spec (R-6). The `handlePass` guard stays load-bearing. The spec states this plainly |
| AC-10 | Config that loads today and would newly fail at `ze config validate` | **Enumerated: exactly one shape** — a unit with > 10 `sysctl-profile` entries. Disposition: **reject** (the committed `sysctl-profile-max-elements.ci` already demands it; a repo grep found no other config with a > 10 `sysctl-profile` leaf-list). No migration needed: `walkTree` runs only under `ze config validate`, never at daemon boot (R-5) |
| AC-11 | `ze config validate -` on a unit with 11 `sysctl-profile` entries (`max-elements 10`) | **NEW walkTree-proving AC.** Reports `too many entries: 11 (maximum 10)`; exit 1. Fails against pre-fix code (probed 2026-07-17: exit 0, valid) |
| AC-12 | `ze config validate -` on a unit with exactly 10 `sysctl-profile` entries | No cardinality error; exit 0 (boundary, walkTree path) |
| AC-13 | `ze config validate -` on a unit with 1 `sysctl-profile` entry (bare-string shape) | No cardinality error; the single-member (`string`) shape still validates and still type-checks each item |

## End-to-End User Stories (MANDATORY for new features)

<!-- Filled 2026-07-17. Story 1 reframed to the iface leaf-list (the un-backstopped, walkTree-
     proving path). Story 2 records the daemon-load boundary honestly (R-5). -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze config validate` on a unit that lists 11 `sysctl-profile` names (over the declared max of 10), expecting to be told | parser → `Tree` → `ToMap` (`[]string`, `tree.go:908`) → `walkTree` leaf-list branch (`validator.go:668`) → `checkCardinality` (:782) → diagnostic (`cmd_validate.go:271-311`) | `test/parse/sysctl-profile-max-elements.ci` (turns green) |
| 2 | Boots the daemon from a saved config that violates a leaf-list bound | **Unchanged by this fix.** The daemon load path (`loader.go:89-131`) runs NO YANG tree validation, so the box still boots (R-5, A-6). Closing that gap is a SEPARATE deferred spec | N/A — daemon-load validation is explicitly out of scope (R-5). Story 2 documents the boundary, it is not a deliverable here |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWalkTreeLeafListCardinality` | `internal/component/config/yang/validator_test.go` | Builds an iface-shaped `map[string]any` with an 11-member `[]string` `sysctl-profile` and drives `Validator.walkTree` (NOT `checkCardinality` directly), asserting a `too many entries: 11 (maximum 10)` error — see AC-7, AC-11 | new |
| `TestWalkTreeLeafListCardinalityMin` | `internal/component/config/yang/validator_test.go` | Drives `walkTree` on a declared leaf-list absent from data with `MinElements > 0`, asserting a `too few entries` error (min-elements reachability) | new |
| `TestWalkTreeLeafListBoundaryAndSingle` | `internal/component/config/yang/validator_test.go` | 10-member `[]string` (boundary, no error) and 1-member bare `string` (no error, still type-checked) — AC-12, AC-13 | new |
| `TestCheckCardinality` (existing) | `internal/component/config/yang/validator_test.go:35` | Unchanged; keep as the helper's isolation test (supplement to, never substitute for, the walkTree-driven tests) | keep |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| iface `sysctl-profile` member count (walkTree, un-backstopped — the load-bearing case) | 0..10 | 10 | N/A (no min-elements) | 11 |
| vrrp `virtual-address` member count (plugin-backstopped; regression only) | 1..16 | 16 | 0 (incl. absent and `[ ]`) | 17 |
| tacacs `profile` member count (out of scope — section not validated, R-6) | 1..unbounded | n/a | 0 | N/A |
| sysctl `profile` LIST count (regression) | 0..50 | 50 | N/A | 51 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| iface sysctl-profile over max | `test/parse/sysctl-profile-max-elements.ci` (committed; currently RED — asserts exit 1 + "too many entries", but the current binary returns exit 0/valid) | Operator lists too many sysctl profiles on one unit and `ze config validate` rejects it naming the max | fix turns it green |
| iface sysctl-profile at boundary | `test/parse/sysctl-profile-max-elements-ok.ci` (new) | Exactly 10 profiles validates clean; 1 profile validates clean (bare-string shape) | new |
| sysctl LIST regression | `test/parse/sysctl-profile-max-elements.ci` (add a 51-entry LIST case) or existing sysctl parse coverage | The LIST `max-elements 50` still fires (must not regress) | new/extend |
| VRRP unchanged (regression) | `test/vrrp/vrrp-config-invalid.ci` (existing) | The VRRP plugin verifier's virtual-address rejections still fire and read cleanly (no confusing duplicate, R-7) | verify unchanged |

### Interop Tests (MANDATORY for protocol features)
Not applicable: config validation infrastructure, no wire protocol behavior changes.

### Future (if deferring any tests)
- **Daemon load-path cardinality validation** — deferred to its own spec (R-5, A-6). Not a test debt of this spec.
- **Section coverage for `tacacs`/`as112`/`geodns`** — deferred (R-6). When their section is added to `yangSectionsToValidate`, each gains a `.ci` proving its leaf-list bound fires. Not in this spec.

## Files to Modify

<!-- Resolved 2026-07-17 for Option (a). -->
- `internal/component/config/yang/validator.go` — **the fix.** Extend the leaf-list branch (`:668-693`) to count members from a `[]string` value (2+ members) as well as the current bare-`string` (1 member) and empty-`string`/absent (0 members) shapes, so `checkCardinality` (:782) is reached; add a declared-but-absent leaf-list scan for `MinElements > 0` (mirroring the mandatory-child loop at `:619-629`); correct/remove the stale comment (`:667-668`). Do NOT touch `checkCardinality` (A-1) or the LIST branch (A-5).
- `internal/component/config/yang/validator_test.go` — new `TestWalkTreeLeafListCardinality` (+ Min/Boundary variants) driving cardinality through `walkTree`, not a direct `checkCardinality` call (AC-7). Keep `TestCheckCardinality`.
- `test/parse/sysctl-profile-max-elements.ci` — **committed, currently RED.** The fix turns it green (no edit needed unless a LIST regression case is appended here).
- `test/parse/sysctl-profile-max-elements-ok.ci` — **new**, boundary + single-member regression guard (see Files to Create).
- ~~`internal/component/config/cli/cmd_validate.go` - `yangSectionsToValidate` (:50-54) if TACACS/as112/geodns coverage is in scope (R-6).~~ NOT modified: section coverage is out of scope (R-6, Options resolution).
- ~~`internal/plugins/vrrp/yang/...`, `.../tacacs/...`, `.../as112/...`, `.../geodns/...`, `.../iface/...` YANG - Option (b) only.~~ NOT modified: Option (b) rejected; the declarations stay as declared intent (they become live under `ze config validate` for any in-section leaf-list once `walkTree` is fixed).
- ~~`test/vrrp/vrrp-config-invalid.ci` - functional coverage.~~ Only re-verified (regression), not the primary wiring; VRRP is plugin-backstopped (see Wiring Test).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No — no new config surface; this fixes enforcement of existing declarations | - |
| YANG validation constraints | [ ] Yes — this spec is entirely about them | `internal/component/config/yang/validator.go` |
| YANG custom validators | [ ] No — Option (c) rejected; no `ze:enforce-cardinality` extension. Native `checkCardinality` is used as-is | - |
| CLI commands/flags | [ ] No — `ze config validate` exists | - |
| Functional test for new RPC/API | [ ] Yes | `test/parse/sysctl-profile-max-elements.ci` (green after fix) + `test/parse/sysctl-profile-max-elements-ok.ci` (new) |
| Env var registration | [ ] No | - |
| Doctor check for runtime dependencies | [ ] No new runtime dependency | - |
| Prometheus counters/metrics | [ ] No | - |

### Documentation Update Checklist (BLOCKING)
<!-- Answered 2026-07-17 for Option (a). -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No — enforcement of an existing declared constraint, not a new surface | - |
| 2 | Config syntax changed? | No — no syntax change. A newly-rejected shape (> 10 `sysctl-profile`) was already invalid per the declared `max-elements`; behavior now matches the declaration | - (verify `docs/guide/configuration.md` does not claim leaf-list bounds are unenforced) |
| 3 | CLI command added/changed? | No — `ze config validate` behavior sharpens; no flag/command change | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No dedicated page for leaf-list cardinality | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Tangential — VRRP `virtual-address` min-elements traces to RFC 9568 5.2.9, but VRRP stays plugin-verifier-enforced; this fix changes no VRRP behavior. No RFC doc update | - |
| 10 | Test infrastructure changed? | No — uses existing `.ci` parse suite and Go unit tests | - |
| 11 | Affects daemon comparison? | No — `walkTree` runs only under `ze config validate`, never at daemon load (R-5) | - |
| 12 | Internal architecture changed? | Yes — the validator's leaf-list reach changes. Update the validator design doc | `docs/architecture/config/yang-config-design.md` (add: leaf-list cardinality now enforced at `ze config validate`; NOT at daemon load) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin/event/command/capability changed? | No | - |
| 16 | Any changed source file referenced by doc source anchors? | Check — `validator.go` carries `// Design: docs/architecture/config/yang-config-design.md`; verify anchored line ranges after the edit | `docs/architecture/config/yang-config-design.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Check `docs/` for any example asserting an over-limit leaf-list validates clean (none expected) | - |

## Files to Create
- `test/parse/sysctl-profile-max-elements-ok.ci` — boundary/positive guard: 10 `sysctl-profile` entries validate clean (exit 0), and a 1-entry (bare-string shape) unit validates clean. Complements the committed over-limit `sysctl-profile-max-elements.ci`.
- No new production files: the fix lives entirely in `internal/component/config/yang/validator.go` and its existing test file.

## Implementation Steps

<!-- Filled 2026-07-17 for Option (a). Prerequisites in the old "requires" list are resolved:
     option = (a); no staging (single-stage error semantics); daemon load OUT of scope (R-5);
     section coverage OUT of scope (R-6); migration = none needed (walkTree never runs at boot). -->

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1 Audit / read source | `## Current Behavior` + `### Ground-Truth Re-Verification` |
| 2 TDD red | `### Unit Tests` (`TestWalkTreeLeafListCardinality`) + the committed RED `sysctl-profile-max-elements.ci` |
| 3 Implement | `## Files to Modify` (`validator.go` leaf-list branch) |
| 4 Wire / functional | `## Wiring Test` + `### Functional Tests` |
| 5 Regression | AC-4..AC-6 + VRRP unchanged (R-7) |
| 6 Critical review | `### Critical Review Checklist` |
| 10 Deliverables | `### Deliverables Checklist` |
| 11 Security review | `### Security Review Checklist` |

### Implementation Phases

**Phase 1 — Reproduce the red.** Run `test/parse/sysctl-profile-max-elements.ci` (11 profiles) and
confirm it fails against current code (probed 2026-07-17: `ze config validate` returns exit 0/valid).
Add `TestWalkTreeLeafListCardinality` and confirm it fails. This is the AC-7 anchor: it must drive
`walkTree`, not call `checkCardinality` directly.

**Phase 2 — Fix `walkTree`'s leaf-list branch** (`validator.go:668-693`). Two changes:
1. Count members from a `[]string` value (2+ members) in addition to the existing bare-`string`
   (1 member) path. Retain the `string` path — a single member legitimately renders as a bare
   string (`tree.go:906-907`), so this is NOT dead code and must not be deleted (nuances the
   no-layering note vs A-7: A-7's concern was a *space-separated* multi-member string, which
   `ToMap` does not produce; the single-member string branch is live and stays).
2. Reach `checkCardinality` for 0-member cases so `MinElements > 0` fires: add a scan over
   `entry.Dir` for children that are leaf-lists with `MinElements > 0` and are absent from `data`
   (or present as `""`), mirroring the mandatory-child loop at `:619-629`.
Correct the stale comment at `:667-668`.

**Phase 3 — Green.** `sysctl-profile-max-elements.ci` and the new unit tests pass. Add
`test/parse/sysctl-profile-max-elements-ok.ci` (boundary 10 + single-member 1).

**Phase 4 — Regression + duplicate check.** Confirm the LIST branch still fires (51 sysctl profiles,
AC-6), the boundary cases pass (AC-4, AC-5, AC-12, AC-13), and re-run `test/vrrp/vrrp-config-invalid.ci`.
Resolve R-7: confirm whether `walkTree` now emits a second (YANG) cardinality error for VRRP on top
of the plugin verifier; if so, decide dedup vs. accept and adjust the VRRP `.ci` expectations to match.

**Phase 5 — Honesty pass.** No doc or comment may claim daemon load now validates cardinality
(R-5). Update `docs/architecture/config/yang-config-design.md` to state the enforcement is at
`ze config validate` only.

**Explicitly NOT done here (deferred to their own specs):** daemon-load validation (R-5, A-6);
adding `tacacs`/`as112`/`geodns` to `yangSectionsToValidate` (R-6); any `Tree.ToMap` change (R-4).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | `checkCardinality` unchanged (A-1); the LIST branch unregressed (AC-6) |
| Data flow | Count is sourced from a shape that preserves it; `Tree.ToMap` NOT changed (R-4) |
| Coverage shape | The new test drives `walkTree`, not `checkCardinality` directly (AC-7). This is the whole lesson |
| Rule: no-layering | The single-member `string` branch is LIVE (`tree.go:906-907`), not dead — retain it; ADD a `[]string` branch beside it. A-7's "space-separated string" shape is what `ToMap` does not produce; do not conflate the two. Do not leave a truly dead path if one is proven |
| Honesty | Any claim that a constraint "now enforces" names the path it enforces on: `ze config validate` YES, daemon load NO (R-5); `interface`/in-section sections YES, `tacacs`/`as112`/`geodns` NO (R-6) |
| Registration over hardcoding | No new per-feature field/switch/factory: the fix reuses the existing `checkCardinality` and the generic `walkTree`. No plugin-specific spelling enters the validator |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `walkTree` leaf-list branch counts `[]string` + absent/empty members | `go test ./internal/component/config/yang/` — `TestWalkTreeLeafListCardinality` (+Min/Boundary) pass |
| Committed red `.ci` turns green | `bin/ze-test bgp parse --all` (or `make ze-parse-test`) — `sysctl-profile-max-elements.ci` passes |
| Boundary/positive guard added | `sysctl-profile-max-elements-ok.ci` passes (10 and 1 member validate clean) |
| LIST branch unregressed | 51 sysctl profiles still report `too many entries: 51 (maximum 50)` (AC-6) |
| VRRP unchanged, no confusing duplicate | `test/vrrp/vrrp-config-invalid.ci` passes; R-7 resolved |
| No daemon-load claim | grep the diff + touched docs for any statement that config load now enforces cardinality (must find none, R-5) |
| Lint clean | `make ze-lint-changed` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Cardinality bounds are themselves an input-validation control; enforcing them is the security-positive direction |
| Availability | Originally framed as the dominant risk (a too-eager fix rejecting running config at boot, R-1). **Re-verified 2026-07-17: this does NOT apply to this fix** — `walkTree` runs only under `ze config validate`, never at daemon boot (A-6, R-5), and the two feared VRRP shapes are already rejected at `ze config validate` today. Availability is not affected; the only new rejection is an operator-invoked lint result for > 10 sysctl-profiles |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `TestWalkTreeLeafListCardinality` / AC-11 does not fail against pre-fix code | The test is not driving `walkTree` (it may be hitting a backstop or calling `checkCardinality` directly). Re-read AC-7; use iface `sysctl-profile`, which has no plugin-verifier backstop |
| A regression guard (AC-4, AC-5, AC-6, AC-12, AC-13) fails | Stop. The fix changed working behavior. Re-read `tree.go:901-911` and the `[]string` vs bare-`string` split |
| VRRP `.ci` now shows two cardinality errors for one group | R-7 made visible. Decide dedup vs. accept; adjust the `.ci` expectation. Do NOT suppress the plugin verifier |
| A previously-passing config now fails on cardinality | Only `sysctl-profile > 10` should newly fail (AC-10). If anything else does, an over-limit config existed unknown-to-us — enumerate it and decide reject vs. fixture-fix. Do NOT weaken the test |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The `PROBE ... err=<nil>` evidence via `config.ParseTreeWithYANG` demonstrated the `walkTree` mechanisms | `ParseTreeWithYANG` (`loader.go:76-131`) never calls the YANG tree validator at all. The probe results were real and reproducible but could not have exercised `walkTree`; they evidenced a different, broader gap | Grepping callers of `ValidateTreeAllModules` after a control probe (17 addrs vs `max-elements 16`) unexpectedly returned 0 errors, which the two stated mechanisms could not explain | The conclusion survived, but for a partly different reason. Recorded in the now-closed TACACS empty-profile-mapping spec too, where its A-3 row cites the same misattributed probe |
| `checkCardinality` is invoked with count >= 1, so only `min-elements` is inert | It is invoked with count EXACTLY 1. `max-elements` on a leaf-list is equally inert — a 17-member leaf-list renders as `[]string` and is skipped | Control probe: 17 addresses against `max-elements 16` produced 0 errors | Doubled the blast radius: 6 inert declarations, not 3. Changed the recommendation (staging step 1 exists only because of this) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Probing via `config.ParseTreeWithYANG` (reproducing the sibling agent's method) | Proved nothing about `walkTree`: that path runs no validation. Its `err=<nil>` results are consistent with the validator being perfect | Probe the real `runValidation` sequence: `YANGSchema` → `Parse` → `PruneInactive` → `ValidateTreeAllModules`, plus a LIST control to prove the harness can observe a failure |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A unit test proves a helper correct in isolation; nothing proves the helper is ever CALLED for the case it guards. The test passes, the guard is dead, and the green test is what stops anyone looking | Twice in this area alone (`TestCheckCardinality` here; the same shape underpins the TACACS/RADIUS escalations) | A guard's test must drive it from the entry point that is supposed to trigger it, at least once. A direct-call table test is a supplement, never the only coverage. This is `ai/rules/wiring-completeness.md` applied to VALIDATION: an exported symbol having a non-test caller is not enough if the caller cannot reach the branch | Propose to Thomas. Candidate home: `ai/rules/testing.md` or `ai/rules/wiring-completeness.md`. NOT actioned in this spec |
| A probe that exercises the wrong path returns the expected result and is accepted as proof | Once, propagated across two specs | When a probe confirms a hypothesis, add a control case that MUST fail. A probe harness that cannot produce a failure has proven nothing. The LIST control (51 vs 50) is what exposed this | Propose to Thomas alongside the row above |

## Design Insights

- **The gap is reachability, not logic.** `checkCardinality` is correct and well-tested. The LIST branch reaches it correctly. Only the leaf-list branch cannot. A fix that "adds validation" has misdiagnosed the problem.
- **`Tree.ToMap` destroys the information the validator needs.** Rendering by member count (0 → absent, 1 → `string`, 2+ → `[]string`, `tree.go:901-911`) is reasonable for readers that want values, and fatal for a reader that wants a count. The validator is the only consumer that needs cardinality, and it is the one consuming the lossy shape.
- **A stale comment documented the bug as the design.** `validator.go:667-668` says leaf-lists are "stored as space-separated strings." That shape is not what `ToMap` produces. The code matches the comment; both are wrong; `strings.Fields` on a single-member string is why one count works.
- **Two independent gaps produced one consistent-looking story.** The load path does not validate; the leaf-list branch cannot validate. Either alone explains `err=<nil>`. This is exactly the `ai/rules/no-fabrication.md` failure mode: a coherent narrative that survives because no control case was run.

## Core Insight

A passing unit test on a correct helper is what hid a dead call site for the entire
life of the feature. `TestCheckCardinality` proves `checkCardinality` rejects count 0
(`validator_test.go:51`). It constructs its own `gyang.Entry` and calls the helper
directly (:57-64). Nothing in the suite proves `walkTree` ever hands it a 0. The test
is not wrong — it is testing the one part that was never broken, and its green status
is precisely what stopped anyone from asking whether the call happens.

The generalisable lesson: **a guard needs a test that drives it from the entry point
that is supposed to trigger it.** Isolation tests verify the guard's logic; only
entry-point tests verify the guard exists in the path. When the two are conflated, a
constraint can be declared, implemented, unit-tested, and completely inert — which is
the exact state of 6 declarations in this repo today.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record the defect as a skeleton; do not fix now | Fix immediately | Thomas's call: enforcing would newly reject config that loads today (Upgrade Risk). The fix needs a migration decision, not just code |
| Widen the spec from `min-elements` to all leaf-list cardinality | Keep the reported `min-elements` scope | The `max-elements` control probe proved the same defect makes 3 more declarations inert. A `min-elements`-only fix would leave the identical hole open and re-diagnose it later |
| Verify by driving `runValidation`'s sequence + a LIST control | Reproduce the sibling probe via `ParseTreeWithYANG` | That path never validates, so it cannot distinguish "validator broken" from "validator not called". The LIST control proves the harness can observe a real failure |
| Do NOT change `Tree.ToMap` | Normalize leaf-lists to `[]string` always | `ToMap` has many consumers (web, gNMI, plugins) that type-assert `string`; a shape change is a wide blast radius for a validator-local problem (R-4) |

## Known Limitations
- No fix. By design: status is `skeleton`, the option is undecided.
- A-7 (no caller delivers a space-separated leaf-list) is unvalidated: the set-format parser and web/gNMI readers were not probed. A fix must settle this before deleting the `string` branch.
- A-8 (real operator config would newly fail) is reasoned from probes and the TACACS escalation, not measured against deployed configs. No field config was surveyed.
- The daemon load path running no validation at all (A-6) is a strictly larger finding that this spec records but does not address. It deserves its own spec.
- Sections absent from `yangSectionsToValidate` (`tacacs`, `as112`, `geodns`) are a separate, compounding gap (R-6), recorded but not fixed.

## RFC Documentation

Not applicable. The VRRP `min-elements 1` declarations encode RFC 9568 Section 5.2.9
(a virtual router advertises at least one virtual address), but this spec changes no
protocol behavior. RFC comments belong with a fix, if one lands.

## Implementation Summary

### What Was Implemented
Nothing. This spec is a `skeleton` recording a verified defect. No production code,
YANG, or test was changed.

### Bugs Found/Fixed
Found, not fixed:
1. `min-elements` is inert on all 3 leaf-list declarations (`vrrp:56`, `vrrp:165`, `tacacs:91`).
2. `max-elements` is inert on all 3 leaf-list declarations (`as112:119`, `geodns:90`, `iface:255`) and on the two VRRP `max-elements 16` bounds. **Not previously reported.**
3. The daemon config LOAD path runs no YANG tree validation at all (`loader.go:89-131`). **Not previously reported.**
4. Stale comment at `validator.go:667-668` describing a shape `Tree.ToMap` does not produce.

### Documentation Updates
None. No behavior changed.

### Deviations from Plan
- The task described 2 mechanisms and 3 affected declarations. Verification found a third mechanism (the `[]string` shape, `tree.go:908-909` vs `validator.go:669`) and 6 affected declarations. The spec records the wider finding; see Mistake Log.
- The task's supplied probe evidence was reproduced but found to be misattributed. Recorded rather than papered over, per `ai/rules/no-fabrication.md`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Verify both mechanisms at the producing function, cite file:line | Done | Current Behavior; Verification Evidence | Both confirmed; a third mechanism found; characterisation corrected |
| Enumerate every `min-elements` declaration | Done | Blast Radius | 3 `min-elements`, plus 5 `max-elements` (6 inert leaf-lists, 2 working lists) |
| Establish upgrade risk concretely | Done | Upgrade Risk | Per-declaration; VRRP group with no virtual-address confirmed by probe |
| Lay out options, recommend, do not decide | Done | Options | (a) staged recommended; decision left to Thomas |
| Record related escalations without duplicating | Done | Task; Blast Radius #3; Options | TACACS (fixed), RADIUS (concurrent) referenced only |
| Fill Task, Required Reading, Current Behavior, Risks & Assumptions, provisional ACs | Done | Respective sections | |
| Leave Implementation Steps thin | Done | Implementation Steps | Deliberately unwritten |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-10 | Provisional | - | Not approved; describe Option (a)'s end state only |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| - | Not written | - | Skeleton; no implementation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-fixit-yang-min-elements-inert.md` | Created | This file; the only file created |

### Audit Summary
- **Total items:** 7 task requirements
- **Done:** 7
- **Partial:** 0
- **Skipped:** 0
- **Changed:** Scope widened from `min-elements` to all leaf-list cardinality (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| The defect is verified, not assumed | Probe through the real validator path + source read of every producer | `validator.go:632`, `:669`, `:692`, `:782`; `tree.go:901-911`; goyang `entry.go:613`, `:616-653`. Probe: absent / `[ ]` / 17-addrs all → 0 errors |
| The probe harness can observe a real failure | Control case | 51 sysctl profiles vs `max-elements 50` → `too many entries: 51 (maximum 50)`. Without this control the leaf-list results are unfalsifiable |
| Blast radius is complete | Exhaustive grep | `grep -rn "min-elements\|max-elements" --include="*.yang" internal/` → 10 hits, 8 declarations, all classified list vs leaf-list |
| No production code changed | git status | Only `plan/spec-fixit-yang-min-elements-inert.md` (new) and `tmp/delete-yang-min-elements-spec.sh` (probe cleanup script) |

## Review Gate

<!-- Not run. Status is `skeleton`; there is no diff to review. -->
<!-- The Review Gate applies when an option is chosen and implemented. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Not applicable — skeleton spec, no implementation diff | - | - |

### Fixes applied
- None. No implementation.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | - | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- Scoped to what a skeleton spec can verify: the recorded claims, not an implementation. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-fixit-yang-min-elements-inert.md` | Yes | This file |
| `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` | Yes | `grep -rn "min-elements" --include="*.yang" internal/` → `:56`, `:165` |
| `internal/component/tacacs/yang/ze-tacacs-conf.yang` | Yes | same grep → `:91` |
| TACACS empty-profile-mapping spec (referenced) | Closed | Existed at write time (`ls` → 33K, 2026-07-16 11:52); since closed and git-rm'd, referenced as history only |
| RADIUS empty-profile-mapping spec (referenced) | Closed | Existed at write time (`ls` → 41K, 2026-07-16 12:11); since closed and git-rm'd, referenced as history only |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| - | ACs are provisional and describe unimplemented behavior | Not verifiable at skeleton status. AC-1..AC-3 are verified to FAIL today (probe: 0 errors for all three shapes), which is the defect |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| - | - | No implementation to wire |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `validator.go:782-806` read; `TestCheckCardinality` count-0 row (`validator_test.go:51`) passes |
| A-2 | confirmed | `validator.go:632`, `:619-629`; goyang `entry.go:613` (Leaf only), `:616-653` (LeafList sets ListAttr, not Mandatory); probe "absent" → 0 errors |
| A-3 | confirmed | `validator.go:669`; probe "empty brackets" → `ToMap` yields `""` → 0 errors |
| A-4 | confirmed | `tree.go:908-909`; probe dump `"virtual-address":[]string{...}`; 17 vs max 16 → 0 errors |
| A-5 | confirmed | `validator.go:658`; probe 51 vs max 50 → `too many entries: 51 (maximum 50)` |
| A-6 | confirmed | `ValidateTreeAllModules` non-test callers = 1 (`cmd_validate.go:277`); `loader.go:89-131` returns after `PruneInactive` |
| A-7 | **unvalidated** | Set-format parser and web/gNMI readers not probed. BLOCKING for any fix that deletes the `string` branch. Carried forward deliberately: a skeleton does not need it settled, an implementation does |
| A-8 | **unvalidated** | No deployed config surveyed. Reasoned from the probe + the TACACS escalation. Carried forward: settling it needs field data, not code |

Per `ai/rules/planning.md`, no assumption may be `unvalidated` at Pre-Commit
Verification. A-7 and A-8 are recorded as knowingly unvalidated because this spec
commits no implementation: A-7 requires a fix approach to exist before it can be
tested, and A-8 requires field data. Both are BLOCKING for the implementation spec
that supersedes this skeleton. Flagged to Thomas rather than silently marked confirmed.

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No docs updated | No behavior changed; nothing to document | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
