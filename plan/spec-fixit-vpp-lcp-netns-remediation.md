# Spec: fixit-vpp-lcp-netns-remediation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - (shares `internal/plugins/iface/vpp/doctor.go` with `plan/spec-fixit-vpp-lcp-reachability.md` and `plan/spec-bgp-netns.md`; neither blocks this, and this blocks neither) |
| Phase | 2/2 |
| Updated | 2026-07-16 |

**Approved by Thomas 2026-07-16 as a standalone fix.** Thomas was offered "fix the message
AND drop the check to a note" and chose "fix the message now, standalone". Severity stays
`SeverityWarning`; detection is unchanged.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/error-messages.md` (what/why/next contract), `ai/rules/no-fabrication.md`
   (comment-as-belief, binding-stub traps), `ai/rules/fail-closed-guards.md`
   (Evidence corollary), `ai/rules/doctor-checks.md`
4. `internal/plugins/iface/vpp/doctor.go`, `internal/core/diagnostic/codes.go`,
   `internal/plugins/iface/vpp/lcp.go`
5. `docs/guide/vpp.md` (corrected in `c49d36524`; its wording is the model for what is TRUE)

## Task

`ze doctor` recommends a configuration that breaks the VPP dataplane.

`checkVPPLCPNetns` (`internal/plugins/iface/vpp/doctor.go:100-131`) emits, at `:124-125`:

> bgp is enabled and vpp.lcp.netns="dataplane" is not root-reachable; BGP cannot bind on an
> LCP-shadowed interface in a separate namespace. Set vpp.lcp.netns to host or root, or run
> BGP in that namespace.

The **detection is correct**. The **remediation is false and destructive**: to VPP, `host`
and `root` are ordinary namespace *names*, not "the host namespace". An operator who follows
the advice makes VPP open `/var/run/netns/host`; absent a namespace of that literal name, LCP
pair creation fails and the dataplane's kernel shadow interfaces never appear. The same false
premise is asserted by `lcpNetnsIsRootReachable`'s doc comment (`doctor.go:133-135`), by
`lcpPairNetns`'s doc comment (`lcp.go:105-108`), and by the registered diagnostic code's
description (`codes.go:297`), which `ze explain doctor-vpp-lcp-netns` prints verbatim.

This is a textbook `ai/rules/error-messages.md` violation: the what and the why are right,
the **what-to-do-next is wrong**. Fix the message so it names only the remedy that works,
and correct the false premise wherever it is asserted in prose.

**Scope boundaries (from Thomas, 2026-07-16):** do NOT change the `vpp.lcp.netns` default
(stays `dataplane`; settled, not a bug), the YANG, `lcpPairNetns`'s *behaviour*, or the
check's detection/severity. Message and doc-comment correctness only. Do NOT edit
`docs/guide/vpp.md`, `plan/spec-bgp-netns.md`, `plan/spec-fixit-vpp-lcp-reachability.md`,
`plan/deferrals.md`, or `plan/learned/*`.

**Cross-references (read-only, not edited by this spec):**

| File | Relationship |
|------|--------------|
| `plan/deferrals.md` row dated 2026-07-16 (search `doctor-vpp-lcp-netns`) | Records this exact defect and routes it to the doctor half. This spec discharges that row. |
| `plan/spec-fixit-vpp-lcp-reachability.md` | Owns the `doctor-vpp-lcp-*` checks after the split; owns AC-12 (`test/ui/doctor-vpp-lcp-netns.ci`, still absent) and the new `doctor-vpp-lcp-plugin` check. This spec touches only the message/prose of the DELIVERED check and does not create that `.ci`. |
| `plan/spec-bgp-netns.md` | A-13 records the VPP-source verification this spec relies on. Its AC-3 later NARROWS `checkVPPLCPNetns` to a mismatch check; that rewrite supersedes this message. Whichever lands second re-reads the file rather than trusting line numbers (R-12 there). |

## Required Reading

### Architecture Docs

- [ ] `ai/rules/error-messages.md` - the contract this bug violates
  → Constraint: an error must answer what failed, why (with the offending value, `%q`), and
    what to do next. Leg 3 is MANDATORY on machine-facing surfaces, doctor explicitly named.
  → Constraint: if the next step needs more than one line, attach a diagnostic code rather
    than truncating the guidance. `doctor-vpp-lcp-netns` is that code; `ze explain` expands it.
  → Decision: "A user-facing failure with no diagnostic code or remediation" is banned. So
    deleting the remediation and saying nothing is NOT an option; the honest remedy must be named.
- [ ] `ai/rules/no-fabrication.md` - how the bug was authored
  → Constraint: a comment states what its author BELIEVED, not the decision record.
    `doctor.go:133-135` and `lcp.go:105-108` are beliefs, and they are wrong.
  → Constraint: a generated binding stub states that a field exists, never what the foreign
    system does with it. `vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go:354` documents "optional
    tap netns" and cannot express VPP's resolution rule. Read VPP's C, not the stub.
- [ ] `ai/rules/fail-closed-guards.md` - Evidence corollary
  → Constraint: a doc or comment asserting a safety property is not evidence the property
    holds. Read the producing function. Exactly how three prior sessions passed this claim on.
- [ ] `ai/rules/doctor-checks.md` - the check's contract
  → Constraint: the plugin that owns the dependency owns the registration, check function,
    and unit test. `ifacevpp` owns all three here.
  → Constraint: every registered code must be explainable via `ze explain`, so `codes.go`'s
    Description is an operator-facing surface and carries the same what/why/next duty.
- [ ] `ai/rules/discovery-updates.md`
  → Constraint: a changed operator-facing diagnostic must update the discovery path.
    `ze explain doctor-vpp-lcp-netns` is a listed discovery surface; its text is in `codes.go`.
- [ ] `docs/guide/vpp.md:88-101, :213` (corrected `c49d36524`) - the model for TRUE wording
  → Decision: "Setting `vpp.lcp.netns` to `host` or `root` does **not** work around this ...
    To VPP those are ordinary namespace *names*."
  → Decision: "Until BGP learns to bind inside a named namespace (specced, not implemented),
    the remedy is to run ze's BGP in the same namespace as the TAPs." This spec's message
    must match this and must not contradict it.

### RFC Summaries (MUST for protocol work)

N/A - no protocol behavior. This is a diagnostic-text and doc-comment correction.

**Key insights:**
- The remedy is not a config value. It is a deployment placement (run ze where the TAPs are)
  or a code change that does not exist yet (`plan/spec-bgp-netns.md`).
- A diagnostic that admits "there is no config-only fix, here is the one workaround" is
  honest and useful; one that invents a fix is the bug being removed.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)

- [ ] `internal/plugins/iface/vpp/doctor.go` - registers `vpp-lcp-netns`
  (Phase PostConfig, Order 741, code `doctor-vpp-lcp-netns`). `checkVPPLCPNetns:100-131`
  returns nil unless a `bgp` container exists AND `vpp/lcp` exists AND `lcp.enabled != "false"`;
  defaults `netns` to `"dataplane"` when the leaf is omitted (`:115-119`, mirroring the YANG
  default); returns nil when `lcpNetnsIsRootReachable(netns)`; else emits ONE
  `SeverityWarning` diagnostic built with `textbuf.Buffer` (`:123-130`).
  `lcpNetnsIsRootReachable:136-143` returns true for `""`, `"host"`, `"root"`.
  → Constraint: detection, severity, registration, and the `""`/`host`/`root` predicate are
    all OUT of scope. Only the message string (`:124-125`) and the doc comment (`:133-135`).
- [ ] `internal/plugins/iface/vpp/lcp.go` - `lcpPairNetns:109-114` maps a root-reachable name
  to `""` for the per-pair `Netns` field of `lcp_itf_pair_add_del` (`lcpItfPair:87-103`).
  Its doc comment `:105-108` asserts `""` makes "VPP places the TAP in its own (host)
  namespace". The file header `:8-13` repeats the claim.
  → Constraint: `lcpPairNetns`'s BEHAVIOUR is frozen by Thomas. Its false comment is prose.
- [ ] `internal/core/diagnostic/codes.go:294-299` - registers `doctor-vpp-lcp-netns` with
  Title "LCP netns unreachable by BGP" and a Description ending "Set vpp.lcp.netns to a
  root-reachable namespace (host/root) or run BGP in that namespace." `RegisterBuiltinCodes:7`
  loads it; `Lookup:39` (`registry.go`) serves `ze explain`.
  → Decision: the registry row DOES carry the false advice. It is in scope (same defect, same
    operator-facing surface, reached by `ze explain`).
- [ ] `internal/component/vpp/startupconf.go:98-112` - when `s.LCP.Enabled`, unconditionally
  writes `default netns <s.LCP.Netns>` into startup.conf's `linux-cp` section (`:106`).
  → Constraint: ze ALWAYS sets VPP's global default netns from the same leaf. This is the
    keystone: it is why an empty per-pair netns does not mean "VPP's own namespace".
- [ ] `internal/core/network/network.go:164-190` - `RealListenerFactory.Listen` binds with a
  bare `net.ListenConfig`; the only `Control` hook is for TCP-MD5 and TTL. No netns awareness.
  → Constraint: DETECTION IS CORRECT. BGP genuinely cannot bind in a non-root netns. Do not
    weaken or delete the check.
- [ ] `internal/plugins/iface/vpp/register_test.go:100-158` - existing coverage:
  `TestDoctorLCPNetnsIsolated`, `...DefaultIsolated`, `...RootReachable`, `...NoBGP`,
  `...Disabled`. All assert diagnostic COUNT and CODE. **None asserts message content.**
  → Decision: that gap is why the false advice shipped and survived. The new test closes it.

**VPP's own source (the foreign system's producer, not a binding stub).** Fetched during the
`spec-bgp-netns` investigation and cached at `tmp/vpp-lcp/`; no in-tree copy exists yet
(`find` for `linux-cp`/`lcp_interface.c` outside `tmp/` returns nothing; another agent is
vendoring it). Verified independently by this session against the cache:

| Producer | Line | What it produces |
|----------|------|------------------|
| `lcp_set_default_ns` (`src/plugins/linux-cp/lcp.c`) | `:67` | `s = format (0, "/var/run/netns/%s%c", lcpm->default_namespace, 0); ... open(s, O_RDONLY)`. The leaf is a namespace NAME under `/var/run/netns/`. `host` and `root` are not special. |
| `lcp_get_default_ns` (`lcp.c`) | `:22-30` | Returns `lcpm->default_namespace`, NULL only when unset or empty. |
| `lcp_itf_pair_create` (`lcp_interface.c`) | `:856-861` | `/* Use interface-specific netns if supplied. Otherwise, use netns if defined, otherwise use the OS default. */ if (ns == 0 \|\| ns[0] == 0) ns = lcp_get_default_ns ();` An EMPTY per-pair netns resolves to the GLOBAL default. |
| `lcp_itf_pair_create` (`lcp_interface.c`) | `:1061-1062` | `if (ns && ns[0] != 0) args.host_namespace = ns;` The POST-fallback `ns` is what reaches `tap_create_if`. |
| `lcp_itf_pair_config` (`lcp_interface.c`) | `:576-579`, `:608` | `unformat (input, "default netns %v")` -> `lcp_set_default_ns`. This is the startup.conf line ze writes. |
| `lcp_interface_stable_2306.c` | `:821` | Identical fallback. Longstanding, not master-only. |
| `lcp_interface_stable_2402.c` | `:830` | Identical fallback. |

**Behavior to preserve:**
- Detection: the check fires on exactly the same configs as today. `bgp` present + `vpp/lcp`
  present + `lcp.enabled != "false"` + `!lcpNetnsIsRootReachable(netns)`.
- Severity `SeverityWarning`, diagnostic code `doctor-vpp-lcp-netns`, exactly ONE diagnostic.
- Registration metadata: Name `vpp-lcp-netns`, Phase PostConfig, Order 741, Component `vpp`,
  Dependencies `["vpp-lcp"]`, Platforms `[Any]`.
- `lcpNetnsIsRootReachable`'s return values (`""`, `host`, `root` -> true).
- `lcpPairNetns`'s mapping behaviour.
- The `vpp.lcp.netns` YANG default `dataplane`, and the message's naming of the offending
  value with `%q`-style quoting (`textbuf.Quoted`).
- All five existing tests in `register_test.go:115-158` pass unchanged.

**Behavior to change:** (user explicitly requested)
1. The diagnostic message (`doctor.go:124-125`) stops recommending `vpp.lcp.netns host|root`
   and names the remedy that actually works.
2. `lcpNetnsIsRootReachable`'s doc comment (`doctor.go:133-135`) stops asserting that VPP maps
   these names to the host netns.
3. The registered code Description (`codes.go:297`) stops repeating the false advice.
4. `lcpPairNetns`'s doc comment (`lcp.go:105-108`) and the `lcp.go:8-13` header stop asserting
   the same false premise. **Prose only; the mapping is untouched.**
   → Decision: included per `ai/rules/before-writing-code.md` "Sibling call-site audit". The
     false claim has four assertion sites; fixing one and leaving three is how it survived
     three sessions. `plan/spec-bgp-netns.md` A-13 names `lcp.go:105-108` as a co-equal site.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator runs `ze doctor` (or `ze doctor --json`) with a config containing a `bgp` stanza
  and `vpp { lcp { netns dataplane } }`. Entry format: the parsed `*config.Tree`.
- Operator runs `ze explain doctor-vpp-lcp-netns`. Entry format: the code string.

### Transformation Path
1. `ze doctor` builds `diagnostic.DoctorCheckContext{Tree: *config.Tree}` and runs registered
   checks by Phase/Order. `vpp-lcp-netns` is PostConfig, Order 741.
2. `checkVPPLCPNetns` (`doctor.go:100`) reads `bgp`, `vpp/lcp`, `vpp/lcp/enabled`,
   `vpp/lcp/netns` from the Tree; defaults netns to `dataplane`.
3. `lcpNetnsIsRootReachable` (`doctor.go:136`) gates: root-reachable -> no diagnostic.
4. Otherwise `textbuf.Buffer` builds the message (`doctor.go:123-125`) -> ONE
   `diagnostic.Diagnostic{Code, Severity, Message}`. **This message is the defect.**
5. The runner prints Message to the operator (text) or emits it in `--json`.
6. Separately, `RegisterBuiltinCodes` (`codes.go:7`) loads `builtinCodes` into the registry;
   `ze explain doctor-vpp-lcp-netns` reads it via `diagnostic.Lookup` (`registry.go:39`) and
   prints Title + Description. **This Description repeats the defect.**

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ diagnostic registry | `diagnostic.RegisterDoctorCheck` from `registerDoctorChecks` (`doctor.go:27`), called by `init()` in `register.go` | [ ] |
| Config tree ↔ check | `ctx.Tree.(*config.Tree)` type assertion (`doctor.go:101`); nil/wrong-type -> nil | [ ] |
| Check ↔ operator | Diagnostic Message, printed by the doctor runner / `--json` | [ ] |
| Code registry ↔ operator | `ze explain <code>` -> `diagnostic.Lookup` -> `CodeMeta.Description` | [ ] |
| ze ↔ VPP (informational, not crossed by this change) | `vpp.lcp.netns` -> `startupconf.go:106` `default netns` -> `lcp_set_default_ns` -> `/var/run/netns/<name>` | [ ] |

### Integration Points
- `internal/core/textbuf` - `Buffer.Str/.Quoted/.String`, the existing message builder. Reused.
- `internal/core/diagnostic` - `Diagnostic`, `CodeMeta`, `Lookup`, `RegisterBuiltinCodes`. Reused.
- No new registration, no new code, no new check.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — no new registry entry, field, switch case, or factory is
      added. The check and code already exist; only their text changes.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VPP resolves `vpp.lcp.netns` as a namespace NAME under `/var/run/netns/`, so `host`/`root` are not "the host namespace" | `lcp.c:67` (`format (0, "/var/run/netns/%s%c", ...)` then `open`) | The old advice might work and this whole spec is wrong | Read of VPP's C at the producer (`tmp/vpp-lcp/lcp.c`), independent of the briefing | confirmed |
| A-2 | An EMPTY per-pair netns does NOT mean "VPP's own namespace"; it falls back to the global default | `lcp_interface.c:856-861`; same at `stable/2306:821`, `stable/2402:830` | `lcpPairNetns`'s comment would be right and only the message would be wrong | Read of VPP's C at the producer | confirmed |
| A-3 | ze always sets that global default from the same leaf when LCP is enabled | `startupconf.go:106` `b.kv("default netns", s.LCP.Netns)` inside `if s.LCP.Enabled` | The empty-netns fallback might land on an unset default (= VPP's own ns), making the comment true in ze's deployment | Read of the producing function | confirmed |
| A-4 | BGP cannot bind in a non-root netns today, so DETECTION is correct and must not be weakened | `RealListenerFactory.Listen` (`network.go:167`) uses a bare `net.ListenConfig`; `Control` only sets MD5/TTL | Deleting the check would be right; keeping it would warn about a non-problem | Read of the producing function; `docs/guide/vpp.md:304` states the same | confirmed |
| A-5 | The registered code Description repeats the false advice and is operator-reachable | `codes.go:297` text; `ze explain` path via `Lookup` (`registry.go:39`) | Scope would shrink to the message alone | `grep` + read; `Lookup` read | confirmed |
| A-6 | There is NO config-only remedy today: no value of `vpp.lcp.netns` lets a root-netns ze bind on the TAPs | A-1..A-4 combined; `docs/guide/vpp.md:88-101` states the same after `c49d36524` | The message would be lying in the opposite direction (claiming no fix when one exists) | Reasoning over A-1..A-4 + the corrected guide. **Limit:** a host-side `mount --bind /proc/1/ns/net /var/run/netns/host` would give the literal name a meaning, but ze cannot do it, ze does not document it, and VPP's behaviour on such a bind-mount is NOT verified here. It is therefore NOT named as a remedy (`no-fabrication`). | confirmed |
| A-7 | The five existing netns tests assert only count and code, so none breaks on a message rewrite | `register_test.go:115-158` read in full | The rewrite would need coordinated test edits | Read; then `go test` after the change | confirmed - all five PASS unchanged after the rewrite (`tmp/lcp-netns-green.log:1-10`) |
| A-8 | No `.ci`, doc test, or golden file asserts the old message text | `grep -rn doctor-vpp-lcp-netns internal/ test/ docs/ ai/ plan/` returns no `test/` hit; `test/ui/doctor-vpp-lcp-netns.ci` is confirmed ABSENT and is AC-12 of `spec-fixit-vpp-lcp-reachability` | A functional test would go red and would need its owner's spec touched | grep; then package tests | confirmed - grep has no `test/` hit; `internal/core/diagnostic`, `internal/component/doctor{,/cmd,/yang}`, `internal/plugins/iface/vpp{,/yang}` all green (`tmp/lcp-netns-pkgs.log`). **Limit:** `make ze-verify` was NOT run (other agents hold live uncommitted work in this tree; the caller asked for `ze-lint-changed` + `ze`, both clean). The three packages that can see this text are covered |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new message replaces bad advice with different bad advice (invents a remedy) | Review asks "did you verify that remedy at a producer?" | Name ONLY what A-1..A-6 support: run ze in that namespace. Explicitly refuse the bind-mount trick (A-6 limit). |
| R-2 | A test asserting exact message text is brittle and blocks the later `spec-bgp-netns` AC-3 rewrite | The next session edits the message and the test fails for a cosmetic reason | Assert SUBSTANTIVE properties via regexp (no "set the leaf to X" directive; names run-ze-in-namespace), never the full string. |
| R-3 | File collision: `spec-fixit-vpp-lcp-reachability` (new plugin check) and `spec-bgp-netns` (AC-3 narrowing) both touch `doctor.go` | Merge conflict in `checkVPPLCPNetns` | This change is small and localized to the message + one doc comment. Whichever lands next re-reads the file (their R-12 says the same). The property test survives a narrowing rewrite; a text test would not. |
| R-4 | ~~`docs/guide/vpp.md:97-101` and `:213` say the doctor advice "does not work" / "should not be followed". After this fix those sentences are STALE~~ | A doc reader is told to distrust a message that is now correct | **RESOLVED 2026-07-16, same session.** Both passages were corrected once this fix landed: `:97-101` now says the doctor agrees no `vpp.lcp.netns` value fixes it and points at `ze explain`, and the leaf row's "though its suggested fix does not work" clause is gone. The agent was right to flag the staleness rather than leave it: the guide was the ORIGINAL source of the false advice (`c49d36524`), so a stale correction there would have re-seeded the bug. |
| R-5 | The check still stays SILENT for `netns host`, a config that breaks LCP pair creation, because `lcpNetnsIsRootReachable` returns true for it | An operator sets `host`, gets no warning, and LCP fails at apply | OUT OF SCOPE: detection and `lcpPairNetns` behaviour are frozen by Thomas. This is a detection gap, not a false remedy. Raised in the return as a follow-up candidate for the owning spec. |
| R-6 | The message grows long enough to be unreadable | Reviewer skims past it | Keep the deep explanation in `codes.go` (what `ze explain` prints), per `error-messages.md` "if the next step needs more than one line, attach a diagnostic code". Message stays ~3 clauses and points at `ze explain`. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor` with `bgp` + `vpp { lcp { netns dataplane } }` (config tree) | → | `checkVPPLCPNetns` (`doctor.go:100`) message construction (`:123-130`) | `TestDoctorLCPNetnsRemediation` (`internal/plugins/iface/vpp/register_test.go`) |
| `ze explain doctor-vpp-lcp-netns` | → | `RegisterBuiltinCodes` (`codes.go:7`) -> `diagnostic.Lookup` -> `CodeMeta.Description` (`codes.go:297`) | `TestDoctorLCPNetnsCodeDescription` (`internal/plugins/iface/vpp/register_test.go`) |

Both entry points already exist and are already registered; this is a text fix behind live
wiring, so there is no new wiring phase. The two tests drive the real check function and the
real registry, not a copy of the string.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze doctor` on `bgp` + `vpp { lcp { netns dataplane } }` | The emitted `doctor-vpp-lcp-netns` message does NOT direct the operator to set `vpp.lcp.netns` to any value, and does NOT offer `host`/`root` as the fix |
| AC-2 | Same | The message names the remedy that works today: run ze in the configured namespace so BGP binds where the TAPs are. It still names the leaf and quotes the offending value, and points at `ze explain doctor-vpp-lcp-netns` for the rest |
| AC-3 | `ze explain doctor-vpp-lcp-netns` (registry Description, `codes.go`) | Carries neither the "set the leaf to host/root" directive nor the "root-reachable namespace (host/root)" phrasing; names the same true remedy; states that netns-aware BGP binding is specced but not implemented |
| AC-4 | Read `lcpNetnsIsRootReachable`'s doc comment (`doctor.go:133-135`) | It no longer asserts that VPP maps `""`/`host`/`root` to the host netns. It states what the predicate actually decides (ze's own marker set) and cites VPP's producing lines for the true resolution rule |
| AC-5 | Read `lcpPairNetns`'s doc comment and the `lcp.go` header | Neither asserts that an empty per-pair netns places the TAP in VPP's own namespace. The mapping's CODE is byte-for-byte unchanged |
| AC-6 | Run the five pre-existing netns tests (`register_test.go:115-158`) | All pass unchanged: detection, severity, code, and the `""`/`host`/`root` predicate are untouched |
| AC-7 | `make ze-lint-changed`, `make ze` | Both clean |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze doctor` on a `bgp` + `vpp.lcp.netns dataplane` config and reads the warning | config tree -> `checkVPPLCPNetns` -> `textbuf` message -> doctor runner output | `TestDoctorLCPNetnsRemediation` |
| 2 | Runs `ze explain doctor-vpp-lcp-netns` to get the long form | code string -> `diagnostic.Lookup` -> `CodeMeta.Description` | `TestDoctorLCPNetnsCodeDescription` |
| 3 | Acts on the advice and their dataplane KEEPS WORKING (the anti-story: today, acting on it breaks LCP pair creation) | operator -> `vpp.lcp.netns` -> `startupconf.go:106` -> `lcp_set_default_ns` -> `open("/var/run/netns/host")` | Negative assertions in `TestDoctorLCPNetnsRemediation` + `TestDoctorLCPNetnsCodeDescription`: no reachable text tells them to do this |

Story 3 is proven by absence *of a directive*, not by absence of an action, so it does not
trip the `ai/rules/tdd.md` red flag: the AC's expected behavior IS "the text does not say X".

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorLCPNetnsRemediation` | `internal/plugins/iface/vpp/register_test.go` | AC-1, AC-2: drives the real `checkVPPLCPNetns`; asserts the message matches NO banned-advice pattern and DOES match the required remedy + subject patterns | |
| `TestDoctorLCPNetnsCodeDescription` | `internal/plugins/iface/vpp/register_test.go` | AC-3: `RegisterBuiltinCodes` + `Lookup("doctor-vpp-lcp-netns")`; same banned/required patterns over `CodeMeta.Description` | |
| `TestDoctorLCPNetnsIsolated`, `...DefaultIsolated`, `...RootReachable`, `...NoBGP`, `...Disabled` (existing, unchanged) | `internal/plugins/iface/vpp/register_test.go:115-158` | AC-6: detection and severity preserved | |

Assertion strategy (R-2): regexp over substantive properties, never the full string.

| Kind | Property | Rationale |
|------|----------|-----------|
| Banned | `(set\|use\|change\|switch) ... vpp.lcp.netns ... (to\|=)` (within one clause) | The directive shape. Catches today's message AND today's `codes.go` Description. Value-agnostic, so it also catches a future "set it to X" regression. |
| Banned | `host or root` / `root or host` | The offered pair. The new text may still MENTION `host`/`root` to say they do not work, so a bare "must not contain 'host'" would be wrong. |
| Required | mentions `vpp.lcp.netns` | `error-messages.md` leg 1: name the subject. |
| Required | `run (ze\|bgp) ... namespace` in one clause | `error-messages.md` leg 3: the one true next step. |
| Required (message only) | contains the quoted offending value (`"dataplane"`) | `error-messages.md` leg 2: the evidence. |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | no numeric input is added or changed | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-vpp-lcp-netns` | `test/ui/doctor-vpp-lcp-netns.ci` | An operator running `ze doctor` on a non-root-reachable LCP netns is warned | NOT created here: owned by `plan/spec-fixit-vpp-lcp-reachability.md` AC-12, which ships it for the check as it exists today. Confirmed absent. |

→ Decision: no functional test is added by this spec. `ai/rules/doctor-checks.md` requires a
functional test for every NEW doctor check; this check is not new, is already registered, and
its `.ci` is an explicit, dated, already-tracked AC of a sibling spec that Thomas forbade
editing. Creating it here would collide with that AC. The two unit tests drive the real
producing function and the real registry, which is what the ACs assert. **This is a scope
boundary set by the task instructions, not a self-authorized reduction**
(`ai/rules/no-partial-completion.md`), and it is named in the return.

### Interop Tests (MANDATORY for protocol features)

N/A - no wire protocol behavior is added or changed.

### Future (if deferring any tests)

None. Both ACs that this spec owns have a unit test in this change.

## Files to Modify

- `internal/plugins/iface/vpp/doctor.go` - the message (`:124-125`) and
  `lcpNetnsIsRootReachable`'s doc comment (`:133-135`); the file-header bullet at `:13-15`
  that repeats "root-reachable namespace" framing
- `internal/plugins/iface/vpp/lcp.go` - `lcpPairNetns`'s doc comment (`:105-108`) and the
  header's netns paragraph (`:8-13`). **Comments only; no statement changes.**
- `internal/core/diagnostic/codes.go` - the `doctor-vpp-lcp-netns` Description (`:297`)
- `internal/plugins/iface/vpp/register_test.go` - two new tests

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No - no leaf added or changed; `vpp.lcp.netns` default stays `dataplane` | - |
| YANG validation constraints | [ ] No | - |
| YANG custom validators | [ ] No | - |
| CLI commands/flags | [ ] No | - |
| CLI grammar (action before identifier) | [ ] No | - |
| Editor autocomplete | [ ] No | - |
| Functional test for new RPC/API | [ ] No - no new RPC/API | - |
| Pipe completeness | [ ] No | - |
| Env var registration | [ ] No | - |
| Doctor check for runtime dependencies | [ ] No new dependency. The check, its code, and its unit test already exist; this corrects their TEXT. Registry row updated per `ai/rules/doctor-checks.md` "must be explainable via `ze explain`" | `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | [ ] No | - |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No - existing check, corrected text | - |
| 2 | Config syntax changed? | [ ] No - no YANG, no leaf, no default changed | - |
| 3 | CLI command added/changed? | [ ] No - `ze doctor` / `ze explain` output text only, no grammar change | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | - |
| 6 | Has a user guide page? | [ ] Yes, `docs/guide/vpp.md` - **but EXPLICITLY OUT OF SCOPE** (Thomas: do not edit; corrected in `c49d36524`). Its `:97-101` and `:213` sentences that the doctor advice "does not work"/"should not be followed" go stale on this change. Raised in the return; see Known Limitations and R-4 | `docs/guide/vpp.md` (NOT edited) |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] No | - |
| 10 | Test infrastructure changed? | [ ] No - two ordinary unit tests in an existing file | - |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [ ] No - no structure, no flow, no registration changed | - |
| 13 | Route metadata keys added/changed? | [ ] No | - |
| 14 | Prometheus counters added/changed? | [ ] No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] No - the code `doctor-vpp-lcp-netns` and the check `vpp-lcp-netns` keep their identity; only Description text changes | - |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] Yes - `docs/guide/vpp.md:220` anchors `internal/plugins/iface/vpp/doctor.go` and `:217`/`:99` anchor the same file and `startupconf.go`. The anchored CLAIMS stay true (VPP resolves names under `/var/run/netns/`; `host`/`root` are not the host namespace) except the two stale sentences in R-4. Not edited per instruction | `docs/guide/vpp.md` (NOT edited) |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] No - `docs/guide/vpp.md` shows no `ze doctor` transcript for this code (grep: only prose references) | - |

## Files to Create

None. Two tests are added to the existing `internal/plugins/iface/vpp/register_test.go`; no
new source file, no new `.ci`.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; confirm `doctor.go:124-125`, `codes.go:297`, `lcp.go:105-108` still read as described |
| 3. Wiring phase | N/A - both entry points are already registered and live (see Wiring Test) |
| 4. Implement (TDD) | Phases 1-2 below |
| 5. Full verification | `make ze-lint-changed`, `make ze` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | - |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Return to the caller; closure per `ai/rules/planning.md` when the user commits |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Failing tests (TDD red)** — assert the substantive properties of both surfaces
   - Tests: `TestDoctorLCPNetnsRemediation`, `TestDoctorLCPNetnsCodeDescription`
   - Files: `internal/plugins/iface/vpp/register_test.go`
   - Verify: BOTH fail against the shipped text, and fail for the RIGHT reason: the message
     matches the banned "set vpp.lcp.netns to" directive and "host or root", and matches
     neither required remedy pattern. Paste the red output.
2. **Phase: Correct the text (TDD green)** — message, registry Description, doc comments
   - Files: `doctor.go` (message `:124-125`, doc comment `:133-135`, header bullet),
     `codes.go` (`:297`), `lcp.go` (comments only)
   - Verify: both new tests pass; the five existing netns tests still pass (AC-6);
     `git diff` on `lcp.go` shows comment lines only (AC-5).
3. **Full verification** → `make ze-lint-changed`, then `make ze`.
4. **Complete spec** → fill the audit tables. Learned summary and the two-commit closure are
   the USER's call: this session is forbidden from committing (see Known Limitations).

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation or a test with file:line |
| Feature completeness | Both operator surfaces (`ze doctor` message, `ze explain` Description) are corrected. Fixing one and leaving the other is the exact shape of this bug |
| Correctness | Every factual clause in the new text traces to a producer: `/var/run/netns/` -> `lcp.c:67`; global-default fallback -> `lcp_interface.c:856-861`; ze sets the default -> `startupconf.go:106`; BGP cannot bind -> `network.go:167`. No clause without one |
| Naming | The message keeps the stable leading phrase `bgp is enabled and vpp.lcp.netns=` so log scanners and any existing grep still match (`error-messages.md`: do not reword a stable phrase) |
| Data flow | The message is still built with `textbuf.Buffer` in the check; no formatting moves to the runner; no `fmt.Sprintf` introduced |
| Registration over hardcoding | No new code, check, field, or switch case |
| Doctor checks | Code stays registered and `ze explain`-able; `TestDoctorCoverageCodesRegistered` still passes |
| Rule: no-fabrication | The new text asserts nothing about VPP that was not read in VPP's C. The bind-mount workaround (A-6 limit) is NOT named |
| Rule: error-messages | The new text still answers what / why / next. "Next" is a real action, not an invented config |
| Rule: no-workarounds-for-missing-behavior | The detection is NOT weakened to make the problem go away; the missing behavior (netns-aware BGP) stays specced in `plan/spec-bgp-netns.md` |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| The message no longer recommends `host`/`root` | `go test -run TestDoctorLCPNetnsRemediation ./internal/plugins/iface/vpp/` (with feature tags) |
| The `ze explain` Description no longer recommends `host`/`root` | `go test -run TestDoctorLCPNetnsCodeDescription ./internal/plugins/iface/vpp/` |
| Detection unchanged | `go test -run 'TestDoctorLCPNetns' ./internal/plugins/iface/vpp/` (all 7 tests) |
| `lcpPairNetns` behaviour unchanged | `git diff internal/plugins/iface/vpp/lcp.go` shows only `//` lines |
| No false premise left in prose | `grep -rn "own (host) namespace\|maps these to the host netns" internal/` returns nothing |
| Lint + build clean | `make ze-lint-changed`, `make ze` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | None added. The netns value flows from an already-validated YANG leaf into `textbuf.Quoted`, which quotes it; no new sink |
| Error leakage | The message names a config value the operator already wrote. No secret, no path outside `/var/run/netns/`, no host detail is disclosed that the config does not already contain |
| Resource exhaustion | The message is a bounded constant plus one quoted leaf value; built once per doctor run |
| Advice safety | The PRIMARY security-adjacent concern: the current advice causes an operator to break their own dataplane. Removing it is the fix |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| New test passes immediately (red phase) | INVALID test - the assertion does not target the shipped text. Re-read `doctor.go:124-125` and tighten |
| An existing netns test fails | Detection was changed - REVERT that part; only text is in scope |
| `make ze` fails in an unrelated package | Check feature tags (`ai/rules/bash-output.md`); a bare `go test` drops them |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (Pre-existing, by the code's authors) An empty per-pair netns places the TAP in VPP's own (host) namespace, so `netns host`/`root` is a working escape hatch | VPP's netns model is TWO-LEVEL: an empty per-pair netns falls back to the GLOBAL default (`lcp_interface.c:856-861`), which ze itself sets from the same leaf (`startupconf.go:106`). `host`/`root` are ordinary names resolved to `/var/run/netns/<name>` (`lcp.c:67`) | Read of VPP's C source at the producer during `spec-bgp-netns` (A-13), re-verified independently by this session against `tmp/vpp-lcp/` | `ze doctor` shipped a remediation that BREAKS LCP pair creation. It survived three sessions because the belief was asserted in two code comments and repeated as settled background |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Name a config-only remedy (any `vpp.lcp.netns` value) | A-1..A-4: no value works while BGP has no netns awareness. Naming one would repeat the bug in a new costume | Say there is no config-only fix and name the one workaround: run ze in that namespace |
| Name the `mount --bind /proc/1/ns/net /var/run/netns/host` trick as the fix | `ai/rules/no-fabrication.md`: VPP's behaviour on a bind-mounted root netns was NOT read at a producer. Also a host action ze neither performs nor documents | Omit it. A-6 records the limit explicitly |
| Delete the remediation and state only the problem | `ai/rules/error-messages.md` bans a user-facing failure with no remediation; leg 3 is mandatory on doctor | Keep leg 3, make it true, push the long form to `ze explain` |
| Assert the exact new message string in the test | Brittle (R-2); would block `spec-bgp-netns` AC-3's later narrowing for a cosmetic reason | Regexp property assertions over banned/required patterns |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A doctor check's remediation text is never asserted by any test, so wrong advice ships and survives | 1 known (this); the sibling `doctor-vpp-wireguard` message is equally unasserted | `ai/rules/doctor-checks.md` "Test Requirement" could require the unit test to assert the diagnostic's REMEDIATION, not only its code. Today it says "emits the registered code", which is exactly what the five existing tests do, and it was not enough | Raise in the return. Do NOT edit the rule in this spec (`ai/rules/canonical-sources.md`; not approved by Thomas) |

## Design Insights

- The false claim had FOUR assertion sites (`doctor.go:124-125` message, `doctor.go:133-135`
  comment, `lcp.go:105-108` comment, `codes.go:297` registry) and ZERO verification sites.
  Comments and a generated binding stub (`binapi/lcp/lcp.ba.go:354`, "optional tap netns")
  were the entire evidence base. `ai/rules/no-fabrication.md`'s two newest banned rows (comment
  as intent, binding stub as foreign semantics) name this exact pair, and both fired here.
- A test that asserts a diagnostic's CODE proves the check fires. It says nothing about
  whether the check's advice is survivable. `ai/rules/fail-closed-guards.md`'s test corollary
  ("test the shape that should be rejected") generalizes: for a diagnostic, the shape that
  should be rejected is BAD ADVICE, and only a message-content assertion can reject it.
- `ze doctor`'s remediation is not documentation. It is executed by operators. It deserves the
  same producer-verification bar as code, which is why `error-messages.md` leg 3 is BLOCKING.

## Core Insight

An error message's "what to do next" is a behavioral claim about a foreign system, and it
must be verified at that system's producer like any other claim. This one was verified at a
Go comment instead, and the comment was its author's belief. The result was a diagnostic that
told operators to break their dataplane, printed by the very tool whose job is to stop them.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Say "no config value fixes this", then name running ze in the namespace | Name a config remedy; name the bind-mount trick; say nothing | Only the namespace-placement remedy is verified (A-1..A-6). `error-messages.md` requires a next step; `no-fabrication.md` forbids inventing one. The honest intersection is exactly this sentence |
| Keep the "host/root do not work" clause in the message instead of only in `ze explain` | Move it entirely to the code Description | The old advice is in the wild: in operators' notes, in three sessions of spec prose, and it was in `docs/guide/vpp.md` until `c49d36524`. A reader who already believes it will not run `ze explain`. Actively contradicting it is worth the extra clause |
| Correct `lcp.go`'s comments too, though the task named only `doctor.go:133-135` | Fix only the named comment | `ai/rules/before-writing-code.md` sibling audit; `spec-bgp-netns` A-13 names `lcp.go:105-108` as a co-equal assertion site. Fixing one of four sites is how this survived. Comments only; behaviour frozen per instruction |
| Assert properties by regexp, not exact text | Golden string; substring checks | R-2: `spec-bgp-netns` AC-3 will rewrite this message. A property test survives that rewrite and keeps guarding the invariant; a text test would be deleted with the first cosmetic edit |
| Keep `SeverityWarning` | Drop to a note; raise to Error | Thomas chose "fix the message now, standalone" over "fix the message AND drop the check to a note". No concrete reason to change severity was found: the hazard is real and BGP silently fails to bind, which is not a note. Recommendation raised in the return, not acted on |

## Known Limitations

- **No functional `.ci`.** `test/ui/doctor-vpp-lcp-netns.ci` remains absent. It is AC-12 of
  `plan/spec-fixit-vpp-lcp-reachability.md`, which this session must not edit.
- **`docs/guide/vpp.md` goes partly stale** (R-4): `:97-101` and `:213` tell readers the
  doctor advice does not work / should not be followed. After this change the advice is
  correct and those sentences should be trimmed. Not editable per instruction.
- **The check is still silent for `netns host`** (R-5): `lcpNetnsIsRootReachable` returns true
  for a name that, per A-1, would break LCP pair creation. Detection is frozen by instruction.
- **No commit.** This session is forbidden from `git commit`/`git add` (CLAUDE.md ABSOLUTE
  PROHIBITIONS; other agents and a human session are live). The learned summary and the
  two-commit closure are therefore the user's call, not this session's.

## RFC Documentation

N/A - no RFC-governed behavior. The foreign-system citations that play the equivalent role
are VPP's linux-cp C sources, quoted with file:line in Current Behavior and repeated in the
corrected doc comments.

## Implementation Summary

### What Was Implemented

- `internal/plugins/iface/vpp/doctor.go`: the `doctor-vpp-lcp-netns` message no longer tells
  the operator to set `vpp.lcp.netns` to `host`/`root`; it states that no value of the leaf
  fixes it, names running ze in the configured namespace, and points at `ze explain`.
  `lcpNetnsIsRootReachable`'s doc comment now describes what the predicate actually decides
  (ze's marker set) and cites VPP's producing lines for the true rule. The file-header bullet
  is aligned.
- `internal/core/diagnostic/codes.go`: the `doctor-vpp-lcp-netns` Description carries the same
  true remedy and drops "(host/root)".
- `internal/plugins/iface/vpp/lcp.go`: `lcpPairNetns`'s doc comment and the header paragraph
  no longer assert that an empty per-pair netns means VPP's own namespace. Code unchanged.
- `internal/plugins/iface/vpp/register_test.go`: `TestDoctorLCPNetnsRemediation` and
  `TestDoctorLCPNetnsCodeDescription`.

### Bugs Found/Fixed

- The subject of this spec. Both new tests fail against the shipped text and pass after.

### Documentation Updates

- None in `docs/`. Every row of the Documentation Update Checklist is answered above; the two
  Yes rows (#6, #16) both name `docs/guide/vpp.md`, which is explicitly out of scope by
  instruction and already carries the true wording from `c49d36524`.

### Deviations from Plan

- Extended the task's named `doctor.go:133-135` doc-comment fix to `lcp.go`'s two comments and
  the `codes.go` registry Description. Both are the same false claim on the same defect; the
  task explicitly directed checking `codes.go`, and the sibling-audit rule directed `lcp.go`.
  No behaviour changed at either site.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Confirm the verified chain at each producer | Done | Current Behavior | All 4 links re-verified independently against `tmp/vpp-lcp/` and ze's source |
| Correct the message so it stops recommending a broken config | Done | `doctor.go:123-135` | AC-1 |
| Message still tells the operator what to do next | Done | `doctor.go:133-135` | AC-2 |
| Correct `lcpNetnsIsRootReachable`'s doc comment | Done | `doctor.go:143-155` | AC-4 |
| Check and fix `codes.go`'s registry row if it repeats the advice | Done | `codes.go:297` | AC-3; it DID carry the false advice |
| Do not weaken or delete the detection | Done | `doctor.go` | AC-6; 5 pre-existing tests pass unchanged |
| Failing test first, asserting properties not exact text | Done | `register_test.go:160-250` | Red: `tmp/lcp-netns-red.log`; green: `tmp/lcp-netns-green.log` |
| Spec from `plan/TEMPLATE.md` with citations, ACs, assumptions | Done | This file | Validator (JSON stdin) exit 0 |
| Severity recommendation raised, not acted on | Done | Key Design Decisions | `SeverityWarning` unchanged |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDoctorLCPNetnsRemediation` | Failed on both banned patterns before, passes after |
| AC-2 | Done | `TestDoctorLCPNetnsRemediation` | Required patterns + `"dataplane"` quoted |
| AC-3 | Done | `TestDoctorLCPNetnsCodeDescription` + `./bin/ze explain doctor-vpp-lcp-netns` | Real binary output inspected |
| AC-4 | Done | `doctor.go:143-155`; `grep "own (host) namespace\|maps these to the host netns\|VPP's host namespace" internal/` returns nothing | |
| AC-5 | Done | `git diff internal/plugins/iface/vpp/lcp.go` | Comment lines only; `lcpPairNetns` body untouched |
| AC-6 | Done | 5 pre-existing tests | `tmp/lcp-netns-green.log:1-10` |
| AC-7 | Done | `make ze-lint-changed` (0 issues), `make ze` (exit 0) | `tmp/lcp-netns-lint.log:203`, `tmp/lcp-netns-build.log` |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDoctorLCPNetnsRemediation` | Done | `register_test.go:213-234` | Red then green |
| `TestDoctorLCPNetnsCodeDescription` | Done | `register_test.go:236-250` | Red then green |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/iface/vpp/doctor.go` | Done | Message + `lcpNetnsIsRootReachable` doc comment. Header bullet re-read and left unchanged: it describes ze's detection accurately and asserts nothing false |
| `internal/plugins/iface/vpp/lcp.go` | Done | Comments only |
| `internal/core/diagnostic/codes.go` | Done | Description only; Code, Title, Examples unchanged |
| `internal/plugins/iface/vpp/register_test.go` | Done | Two tests + shared banned/required pattern tables |

### Audit Summary

- **Total items:** 9 requirements, 7 ACs, 2 new tests, 4 files
- **Done:** all of the above
- **Partial:** none
- **Skipped:** none in scope. `test/ui/doctor-vpp-lcp-netns.ci` is NOT this spec's (AC-12 of `plan/spec-fixit-vpp-lcp-reachability.md`, which the caller forbade editing); the four out-of-scope items are in Known Limitations
- **Changed:** the doc-comment fix was extended to `lcp.go` and `codes.go` (see Deviations); the `doctor.go` header bullet was NOT changed after re-reading (no false claim in it)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `ze doctor` stops recommending a configuration that breaks the VPP dataplane | Unit test driving the real check function | `TestDoctorLCPNetnsRemediation`: banned-pattern assertions over the message produced by `checkVPPLCPNetns` |
| `ze explain doctor-vpp-lcp-netns` stops repeating it | Unit test driving the real registry | `TestDoctorLCPNetnsCodeDescription`: same assertions over `Lookup(...).Description` |
| The operator is still told what to do next | Unit test (required patterns) | Both tests assert a `run (ze\|bgp) ... namespace` clause and the named leaf |
| The false premise is gone from prose | grep | `grep -rn "own (host) namespace\|maps these to the host netns" internal/` returns nothing |
| Detection is not weakened | Pre-existing tests, unchanged | The five `TestDoctorLCPNetns*` tests pass |

## Review Gate

### Run 1 (initial)

Run 1 reviewed the state as COMMITTED in `287aa411e` (the implementation had already
landed; this session found the spec still open and un-gated). Both findings are in the
prose shipped by that commit. Neither breaks behaviour: the CLAIMS are true and were
verified at a producer. What is wrong is where the CITATIONS point, which is the exact
defect class this spec exists to remove.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | **Citation resolves against a git-ignored scratch copy.** Three comments cite `lcp_interface.c:856-861` for the empty-per-pair -> global-default fallback. That range is correct ONLY in `tmp/vpp-lcp/lcp_interface.c`, the author's scratch fetch, which `.gitignore:12` excludes. The SAME commit vendored a DIFFERENT copy into `third_party/vpp-linux-cp/src/`, where the fallback is at `:850-855` and `:856-861` is the sub-interface block (`vnet_sw_interface_is_sub`). A reader following the citation in the only in-tree copy lands on unrelated code | `doctor.go:148`, `lcp.go:110`, `register_test.go:170` | FIXED: all three now cite `third_party/vpp-linux-cp/src/lcp_interface.c:850-855`, the vendored copy, with the producing function named (`lcp_itf_pair_create`) |
| 2 | ISSUE | **Producer misattributed.** Three comments credit `lcp_get_default_ns` with formatting and opening `/var/run/netns/<name>`. It does not: `lcp.c:28-36` only returns `lcpm->default_namespace`. `lcp_set_default_ns` (`lcp.c:50-78`) formats the path (`:73`) and `open`s it (`:74`). The spec's own A-1 row carries the same misattribution (`lcp.c:67`, the scratch copy's line for `:73`) | `doctor.go:125`, `doctor.go:147`, `lcp.go:113`, `register_test.go:168` | FIXED: all sites now name `lcp_set_default_ns` and cite `third_party/vpp-linux-cp/src/lcp.c:73-74` |
| 3 | NOTE | The path substitution that rewrote `tmp/vpp-lcp/` -> `third_party/vpp-linux-cp/src/` when the sources were vendored left three comment lines ~125 chars and visibly unreflowed, which is the visible trace of finding 1: the path was substituted, the LINE NUMBERS were not | `doctor.go:147`, `lcp.go:113`, `register_test.go:168` | FIXED incidentally: comment blocks reflowed as part of findings 1 and 2 |
| 4 | NOTE | `plan/spec-bgp-netns.md` (`:202`, `:332`, `:448`, `:738`) also cites `lcp_interface.c:856-861`. NOT a defect and NOT touched: that spec explicitly labels its citation "not vendored; fetched from FDio/vpp" master, so the range is honest against the copy it names. Now that `third_party/vpp-linux-cp/` exists in-tree, that spec may want to re-anchor when it next lands | `plan/spec-bgp-netns.md` | Raised in the return; another spec's file, not edited |

### Fixes applied

- `internal/plugins/iface/vpp/doctor.go`: the remediation comment in `checkVPPLCPNetns`
  and `lcpNetnsIsRootReachable`'s doc comment now name `lcp_set_default_ns` /
  `lcp_itf_pair_create` and cite the VENDORED copy at its real lines.
- `internal/plugins/iface/vpp/lcp.go`: same correction in `lcpPairNetns`'s CAUTION block.
  **Comments only** -- `git diff` shows no non-`//` line, so AC-5 still holds.
- `internal/plugins/iface/vpp/register_test.go`: same correction in the
  `lcpNetnsBannedAdvice` header comment.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No BLOCKER, no ISSUE. `grep -rn "856-861" internal/ docs/` and `grep -rn "lcp_get_default_ns" internal/ docs/` both return nothing. All 7 `TestDoctorLCPNetns*` PASS (`tmp/lcp-close/netns2.log`). `make ze-lint-changed` "0 issues." (`tmp/lcp-close/lint.log`) | - | - |

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE — Run 2 clean; the two Run-1 ISSUEs are fixed and re-verified by grep + tests + lint
- [ ] All NOTEs recorded above (or explicitly "none") — NOTE 3 fixed incidentally; NOTE 4 is another spec's file, raised in the return, deliberately not edited

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| (no files created; all four targets are pre-existing) | | |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Message names no `vpp.lcp.netns` value as the fix | `tmp/lcp-netns-red.log:12-17` (both banned patterns matched the shipped text) -> `tmp/lcp-netns-green.log:12` PASS |
| AC-2 | Message names the working remedy and quotes the value | Same test asserts `run (ze\|bgp) ... namespace` and `"dataplane"`; PASS |
| AC-3 | `ze explain` carries the true remedy | `tmp/lcp-netns-red.log:20-25` -> `tmp/lcp-netns-green.log:14` PASS; `./bin/ze explain doctor-vpp-lcp-netns` prints the corrected text |
| AC-4 | No false premise in `lcpNetnsIsRootReachable`'s doc | `grep -rn "own (host) namespace\|maps these to the host netns\|VPP's host namespace" internal/` -> NONE |
| AC-5 | `lcpPairNetns` behaviour untouched | `git diff -- internal/plugins/iface/vpp/lcp.go` shows only `//` lines |
| AC-6 | Detection unchanged | 5 pre-existing tests PASS (`tmp/lcp-netns-green.log:1-10`) |
| AC-7 | Lint + build clean | `tmp/lcp-netns-lint.log:203` "0 issues."; `make ze` exit 0 |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze doctor` -> `checkVPPLCPNetns` | none (AC-12 of `spec-fixit-vpp-lcp-reachability`) | Unit test drives the real check function |
| `ze explain` -> `Lookup` | none | Unit test drives the real registry |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `tmp/vpp-lcp/lcp.c:67` |
| A-2 | confirmed | `tmp/vpp-lcp/lcp_interface.c:856-861`, `_stable_2306.c:821`, `_stable_2402.c:830` |
| A-3 | confirmed | `internal/component/vpp/startupconf.go:106` |
| A-4 | confirmed | `internal/core/network/network.go:167-190` |
| A-5 | confirmed | `internal/core/diagnostic/codes.go:297`, `registry.go:39` |
| A-6 | confirmed | A-1..A-4 + `docs/guide/vpp.md:88-101`; bind-mount trick explicitly NOT claimed |
| A-7 | confirmed | 5 pre-existing tests PASS unchanged (`tmp/lcp-netns-green.log:1-10`) |
| A-8 | confirmed | No `test/` grep hit; `internal/core/diagnostic`, `internal/component/doctor{,/cmd,/yang}`, `internal/plugins/iface/vpp{,/yang}` all green. `make ze-verify` NOT run (live concurrent work in tree) |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No `docs/` update is made | `grep -rn doctor-vpp-lcp-netns docs/` hits only `docs/guide/vpp.md` (`:97`, `:213`, `:217`, `:304`); the file is out of scope by instruction | Yes |
| The corrected text does not contradict `docs/guide/vpp.md` | The guide (`:88-101`, corrected in `c49d36524`) says `host`/`root` are ordinary namespace names and "the remedy is to run ze's BGP in the same namespace as the TAPs". The new message and Description say exactly that | Yes |
| Two guide sentences go stale | `docs/guide/vpp.md:97-101` ("its suggested fix still names `host`/`root` and should not be followed") and `:213` ("though its suggested fix (`host`/`root`) does not work") describe the OLD message | Yes - R-4, raised in the return, not edited |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-7 all demonstrated
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

- [ ] RFC constraint comments added (N/A)
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
- [ ] Boundary tests for all numeric inputs (N/A - no numeric input)
- [ ] Functional tests for end-to-end behavior (owned by `spec-fixit-vpp-lcp-reachability` AC-12)
- [ ] Interop tests for protocol features (N/A - not a protocol change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)

- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
