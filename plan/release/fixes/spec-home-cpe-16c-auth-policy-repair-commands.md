# Spec: Make PPP authentication policy repair commands executable

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli, config, docs |
| Priority | P1 |
| Release reason | Printed repair commands cannot be accepted by the configuration editor. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/1 |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-3 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original review and owner direction. Make refusal and PPPoE configuration errors print repair commands the real editor accepts. Print only the transport path that owns the refused session. Preserve authentication policy and explicit operator choice before broadening allowed methods.

## Required Reading

- [x] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`.
  → Constraint: follow the existing command/editor grammar and keep shared PPP independent of the editor component.
- [x] `docs/guide/l2tp.md`, authentication; `docs/guide/pppoe.md`, subscriber authentication; `docs/guide/config-editor.md`.
  → Constraint: distinguish configuration-file leaf-list syntax from executable editor commands.
- [x] `ai/rules/user-facing-errors.md` and `ai/rules/cli.md`.
  → Constraint: errors name the failed policy and a precise applicable repair; broadening policy remains intentional.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/component/l2tp/ppp/auth.go`, `pppSession.refuseAuthMethod` | Builds bracketed `set` commands for both L2TP and PPPoE after a recognized method suggestion. |
| `internal/component/l2tp/pppoe/config.go`, `ExtractParameters` | Policy resolution error prints `set pppoe authentication-allow [chap-md5]`. |
| Review finding F-3 | Reports that the editor rejects these bracketed forms. This split has not rerun the editor workflow. |

Preserve the reason's peer suggestion and allowed-method list, fail-closed auth behavior and the warning that permitting another method is an operator policy decision.

## Existing Mechanisms

| Mechanism | Location | Current role | Disposition |
|-----------|----------|--------------|-------------|
| Shared refusal builder | `pppSession.refuseAuthMethod` | Produces final session reason and warning. | Extend existing transport knowledge to choose one applicable repair path. |
| PPPoE extraction errors | `pppoe.ExtractParameters` | Rejects an invalid explicit auth policy. | Correct its repair syntax without changing policy resolution. |
| Configuration editor | Existing `set`/leaf-list handler, to verify during design | Applies operator commands. | Exercise it directly; do not add grammar support only to accept an erroneous message. |

## Data Flow

Peer auth suggestion or invalid configuration → policy refusal/error producer → displayed repair command → real editor command parser → candidate configuration → policy validation → permitted-method session. Trace existing transport identity through the shared driver rather than importing the editor into PPP.

## Draft Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | L2TP required-auth refusal with recognized disallowed method | Error states the policy and prints only executable L2TP repair command(s); applying the intentional allow-list edit produces the stated ordered list. | Existing permitted methods survive because a repair must not silently remove them. |
| AC-2 | Equivalent PPPoE refusal | Print only executable PPPoE repair command(s), with the same explicit-policy semantics. | L2TP configuration remains unrelated and unchanged because PPPoE owns this session. |
| AC-3 | Invalid PPPoE explicit allow-list | Both advertised repair alternatives execute successfully from their intended starting state and yield valid policy. | Restoring the method-derived default remains available without enabling no-auth. |
| AC-4 | Unknown suggestion or Auth-Protocol Reject | Refusal still gives an actionable peer-policy explanation without inventing a method name or bypassing required authentication. | A valid permitted-method peer still establishes a session. |

## 🧪 Draft Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestAuthRefusalRepairTransport` | Shared driver refusal reasons | L2TP and PPPoE each contain only their applicable transport repair. |
| `TestPPPoEAuthPolicyRepairCommands` | Config extraction error | Every offered alternative can be extracted and executed through the real editor. |
| Editor functional regression | Name exact `.et`/`.ci` carrier during design | Trigger actual message, execute the printed commands, inspect candidate list and validate resulting policy; the old bracketed message must fail. |

Read and use `ze-test` before editing any `.ci` file. Independent judgment is required for protected expectation changes. No editor or protocol tests ran for this split.

## Files to Modify and Documentation

- `internal/component/l2tp/ppp/auth.go`: transport-specific executable repair text.
- `internal/component/l2tp/pppoe/config.go`: valid configuration-error repair alternatives.
- Existing driver/config tests and editor functional carriers: prove the actual message-to-command workflow.
- `docs/guide/l2tp.md`, `docs/guide/pppoe.md` and corresponding wiki examples: verify command examples against the real editor and preserve valid file-format examples separately.

## Risks & Assumptions

| ID | Risk or assumption | Signal / validation | Mitigation / impact |
|----|--------------------|---------------------|---------------------|
| R-1 | A syntactically valid repair replaces rather than extends the intended list. | Candidate list differs from displayed repair intent. | Assert full ordered list and retain existing permitted methods. |
| R-2 | Selecting transport text adds component coupling. | PPP imports config/editor or names a global feature switch. | Use existing session transport information at the refusal producer. |
| A-1 | Confirmed: existing editor operations express the intended list without bracketed tokens. | `Model.cmdSet`, `Editor.SetValue`, `setLeafListMember`; both transport variants of `TestAuthRefusalRepairCommands` pass through the real editor. | One command per member retains the full ordered policy; no editor grammar change. |

## Research Remaining Before Ready

Read the real editor's leaf-list producer, reproduce rejection and validate the replacement spelling. Complete source-aware command/error census, wiring test names and integration/documentation checklists. This spec changes messages, not authentication admission policy.

## Implementation Handoff

The owner directed implementation with all compilation and execution centralized
in the main thread. The agent inspected sources and edited code, tests and
documentation without compiling or running code. Earlier skeleton research
instructions remain as history; the following source checks and test carriers
now define the authored change.

| Surface | Source-supported decision |
|---------|---------------------------|
| Editor grammar | `Model.cmdSet` takes its last token as the value; `Editor.SetValue` calls `setLeafListMember` for these leaf-lists. Print one command per member, without brackets. |
| Transport | `pppSession.pppoe` already carries the transport identity. Use it to choose the existing L2TP or PPPoE path without importing the editor into production PPP. |
| Refusal repair | Retain each permitted method, then add the recognized peer suggestion. The displayed policy warning remains explicit; no admission logic changes. |
| Config repair | The reset alternative deletes the old explicit list before selecting CHAP-MD5 only, so invalid members cannot survive the repair. |

| AC | Authored carrier | Execution status |
|----|------------------|------------------|
| AC-1, AC-2 | `TestAuthRefusalRepairCommands`: actual refusal producer for both transports, then every printed command through the real headless editor, with the full ordered list checked. | Not run; main thread owns compilation and execution. |
| AC-3 | `TestPPPoEAuthPolicyRepairCommands`: obtain the actual extraction error, execute both command alternatives through editor dispatch, then re-extract and check the resulting policy. | Not run; main thread owns compilation and execution. |
| AC-4 | `TestAuthRefusalUnknownMethodRepair`: preserve the peer-policy explanation without an invented method for unknown protocols or Reject. Existing allowed-method and termination tests remain unchanged. | Not run; main thread owns compilation and execution. |

New tests are in `internal/component/l2tp/ppp/auth_repair_commands_test.go`
and `internal/component/cli/pppoe_repair_commands_test.go`, both guarded by
`ze_l2tp`. The PPP refusal test imports only test/editor infrastructure and
schema registration, keeping production PPP independent of the editor.

Update the corresponding wiki authentication sections with the guide paragraphs
added here. No sibling-checkout edit was attempted. The main thread must run
the new tests, existing auth-policy regression tests, scoped race checks and
documentation/source-anchor checks before claiming success. No protected test
expectation was changed by this task.

## Central Verification (2026-11-05)

This result supersedes the author's unrun handoff above. The coordinator ran
all commands; the issue agent did not compile or execute code.

`TestAuthRefusalRepairCommands`, `TestAuthRefusalUnknownMethodRepair` and
`TestPPPoEAuthPolicyRepairCommands` passed through the actual editor/command
dispatcher. Both transport variants of `TestAuthMethodRefusalCounter` passed
under race with their original refusal count and peer-policy assertions,
valid per-member repair commands and rejection of the other transport's
commands. The coordinator independently approved that protected expectation
migration before applying it. All L2TP packages passed the final race run.

The runtime change is committed in `18ea5e830a`; the matching wiki correction
is committed in `312639d`. Evidence is retained in the coordinator's
`job-cpe16-first-auth-image-c7fe6d80.log` and
`job-cpe16-postreview-runtime-race-f3e7163f.log`, under
`tmp/session/2026-11-04-222204/scratch/`. No broader CLI-package green is
claimed. This spec remains available for the separate closure review.

## Closure Review (2026-11-05)

The independent closure agent applied `ze-close`, `ze-review` and
`ze-review-spec` inline. It read the committed implementation in `18ea5e830a`
and the current producers, tests and guides. The coordinator owns every
execution and commit. The following current record supersedes the skeleton's
research tasks and unrun implementation handoff above.

### Executive Summary

The editor accepts each printed method separately. Refusals retain permitted
methods, name only the affected transport and require an explicit operator
decision before permitting the suggested method. Both PPPoE configuration-error
alternatives leave a valid authenticated policy. Unknown protocols and Reject
print the peer-policy explanation without a guessed command.

Risks and observations: R-1 is covered by exact ordered-list readback; R-2 is
covered by the existing `pppSession.pppoe` field and unchanged production
imports. A repair is a candidate edit; the operator still commits configuration
through the normal editor workflow. This change adds no wire behavior.

### Wiring and Source Census

| Writer / reader | Producing source | Verified contract |
|-----------------|------------------|-------------------|
| Session transport | `internal/component/l2tp/ppp/manager.go`, `Driver.spawnSession` | Copies `StartSession.PPPoE` into the existing session field. |
| Policy refusal entry | `internal/component/l2tp/ppp/session_run.go`, `handleLCPPacket`, `applyTransition` | Required-auth exhaustion calls the shared refusal before publishing Opened. |
| Refusal command writer | `internal/component/l2tp/ppp/auth.go`, `refuseAuthMethod` | Recognized enum suggestion selects one transport path; retained methods precede the suggestion. |
| Config-error command writer | `internal/component/l2tp/pppoe/config.go`, `ExtractParameters` | Policy-resolution errors offer default restoration or explicit CHAP-MD5 reset. Existing malformed-type errors already offer deletion. |
| Command consumer | `internal/component/cli/model_commands_edit.go`, `Model.cmdSet` | Last token is the value; preceding tokens are the schema-validated path. |
| Leaf-list consumer | `internal/component/cli/editor_commands.go`, `Editor.SetValue`, `setLeafListMember`, `DeleteValue` | Adds one unique member or deletes the old explicit list. |
| Policy consumer | `internal/component/l2tp/ppp/auth_policy.go`, `ResolveAuthAllow` | Retains ordered unique known methods and requires the configured method. No admission-policy change. |
| Published examples | `docs/guide/l2tp.md`, `docs/guide/pppoe.md`; wiki `l2tp.md`, `pppoe.md` | Editor examples use one member per command; bracket syntax remains configuration-file syntax. |

The census searched both transport sources and guides for the old bracketed
`set` form and used the remaining valid bracketed file examples as positive
controls. No production refusal/error writer retaining the old command was
found. No external SDK, JSON field, YANG declaration or shell/Python consumer
changes are needed for these plain diagnostic strings.

## Deliverables Checklist

| Deliverable | Verification | Result |
|-------------|--------------|--------|
| Executable L2TP and PPPoE refusal commands | `refuseAuthMethod`; `TestAuthRefusalRepairCommands` | Done: both real editor variants pass, retaining CHAP-MD5 and MS-CHAPv2 before PAP. |
| Both PPPoE error repairs | `ExtractParameters`; `TestPPPoEAuthPolicyRepairCommands` | Done: each alternative executes through `Model.dispatchCommand`, re-extracts CHAP-MD5 only and leaves `AllowNoAuth` false. |
| Unknown/Reject and required-auth preservation | `TestAuthRefusalUnknownMethodRepair`, `TestAuthMethodRefusalCounter`, `TestAuthRefusalTerminationCleanup`, `TestAuthAllowSelectionMatrix` | Done: policy explanation, exactly one refusal counter, normal termination and permitted alternative remain covered. |
| Source-aware guides and wiki | `18ea5e830a`, wiki `312639d` | Done: actual command grammar and intentional policy change explained. |

## Security Review Checklist

| Concern | Source-aware result |
|---------|---------------------|
| Accidental authentication downgrade | `refuseAuthMethod` only formats text before the unchanged Close transition. Its warning requires intent. Both reset alternatives preserve authenticated CHAP-MD5 and never set `allow-no-auth`. |
| Wrong transport or untrusted command injection | The path is one of two constants selected by the admitted transport. `authMethodFromAuthProto` maps the peer protocol to a closed enum; only `AuthMethod.String()` values enter commands. No peer-provided text enters command syntax. |
| Malformed, unknown or missing input | Parse failure, unknown protocol and Reject do not enter the suggestion-command branch. Unknown/Reject regression asserts no quoted command and retains the peer-policy repair. |
| Bounds, allocation and denial of service | No allocation is sized from a peer count in the changed code. The policy resolves to at most three distinct wire methods; the existing LCP packet/options parser bounds packet access. Diagnostic construction uses `textbuf.Buffer` on the refusal path. |
| Authorization, secrets, paths and cryptography | Diagnostics do not execute commands or mutate policy. No credentials, file paths, external process, socket, crypto primitive or authorization handler is added or changed. |
| Race and lifecycle | Text is built on the existing session path; no new goroutine, shared map or timer is introduced. Refusal/metrics and all L2TP packages passed the coordinator's race run. |

## Documentation Update Checklist

| Category | Update? | Evidence |
|----------|---------|----------|
| Feature declaration/list | No | No capability, registration or feature level changes; only existing refusal/error messages. Source-anchor census of `docs/` finds the transport guides as the changed claims. |
| User guide | Yes | `18ea5e830a`: authentication sections of `docs/guide/l2tp.md` and `docs/guide/pppoe.md`; anchors name `refuseAuthMethod`, `ExtractParameters`, `cmdSet`, `SetValue` and `setLeafListMember`. Wiki matching sections landed in `312639d`. |
| Config syntax | No | No parser or schema changes. `docs/guide/configuration.md` still correctly uses file leaf-list brackets; `docs/guide/config-editor.md` documents member edits. |
| CLI reference | No | No command declaration, completion or grammar change. Existing `Model.cmdSet` is the authority used by the new tests. The transport guides carry the changed diagnostic examples. |
| API/RPC | No | No registered method, result shape or API type changes; refusal remains the existing reason string. |
| Plugin SDK | No | No `pkg/` change or cross-process contract change. |
| Wire format | No | Changed hunks only format error/repair text. Required-auth transition and packet writers are preserved. |
| RFC compliance | No | No new or changed RFC-tagged unit belongs to 16c. Existing policy/termination RFC tests are preservation evidence. The broader changed-producer proof refresh belongs to child 16a; no global RFC-green claim is made here. |
| Comparison table | No | No capability scope or support level changes. |
| Test infrastructure | No | `RunETContentIn`, `Model.dispatchCommand` and existing editor infrastructure are reused, with no harness contract change. |
| Architecture / doctor | No | Existing session transport field selects the path; production PPP gains no editor import or runtime dependency. Existing architecture/source-anchor claims remain accurate. |
| Declared PPPoE design page | No | `docs/architecture/l2tp/bng-5-pppoe.md`, Decisions/reload: `Subsystem.Reload` calls `parametersFromProvider`, which still calls `ExtractParameters`. The changed hunk alters only the error's repair text; live reload and policy parsing claims remain accurate. |
| Declared PPP integration design page | No | `docs/research/l2tpv2-ze-integration.md`, userspace control plane and YANG Schema Design: `ResolveAuthAllow` policy resolution and shared authentication dispatch are unchanged. The page declares no bracketed editor-repair syntax, so no sentence becomes stale. |
| Related subscriber architecture page | No | `docs/architecture/l2tp/subscriber-session-model.md`, accepted subscriber identity: `awaitAuthDecision` still assigns `authenticatedUsername` only after `decision.accept`. The refusal-text hunk changes neither this function nor its event contract. |

## Implementation Summary

### What Was Implemented

`refuseAuthMethod` prints one member command per retained method followed by
the recognized suggestion, for one transport. `ExtractParameters` prints a
valid default-restoration alternative and a CHAP-MD5-only reset that removes
the old list. Runtime/tests/guides are committed in `18ea5e830a`; wiki examples
are committed in `312639d`.

### Bugs Found/Fixed

F-3's bracketed editor commands and unrelated transport alternative are fixed
by the two production message writers. `TestAuthRefusalRepairCommands` and
`TestPPPoEAuthPolicyRepairCommands` check the actual produced strings through
the real command consumer. The protected metrics assertion migration was
independently approved and is retained in the implementation commit trailer.

### Documentation Updates

The two guide authentication paragraphs carry the source anchors listed above.
The matching wiki paragraphs distinguish member commands from file syntax.
The coordinator's `./le doc check all` reported broader generated catalog drift
and unrelated stale pages; it did not identify a stale 16c repair paragraph.
No whole-documentation-green claim is made. Those wider corrections are being
handled centrally and do not change this message-only implementation.

### Deviations from Plan

Draft `TestAuthRefusalRepairTransport` became
`TestAuthRefusalRepairCommands`, which adds full editor execution rather than
only string checks. The editor functional carrier is generated ET content in
that test and run by `RunETContentIn`; no separate tracked `.et` was added.
The draft's unknown-method test is `TestAuthRefusalUnknownMethodRepair`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation handoff retained skeleton research prose and omitted closure checklists. | Sources, tests and guide updates had already landed. | Independent closure compared `18ea5e830a` and current producers with this spec. | Added the three checklists and current audit here; retained the dated handoff as history. |
| approach | Initial coordinator evidence had only passing editor runs. | User-workflow proof also owes an observed semantic break of the real producer. | Closure checked the recorded command/output against the discrimination rule. | Coordinator restored both old message producers through a Go overlay: both PPP transport subtests failed the command-count guard and the CLI test failed the alternatives-shape guard. Restoring current producers passed both editor variants and both alternatives under race. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Repair commands execute in the real editor | Done | `refuseAuthMethod`, `ExtractParameters`; both repair test files | Parsed commands and resulting policy are checked. |
| Only session transport path is printed | Done | `refuseAuthMethod`; `TestAuthMethodRefusalCounter` | Both transports prohibit the other's repair path. |
| Preserve auth policy and explicit operator choice | Done | `refuseAuthMethod`, `ResolveAuthAllow`; policy matrix/termination controls | Warning, original methods and required-auth behavior survive. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAuthRefusalRepairCommands/l2tp` | Full ordered CHAP-MD5/MS-CHAPv2/PAP candidate readback. |
| AC-2 | Done | `TestAuthRefusalRepairCommands/pppoe`, `TestAuthMethodRefusalCounter/PPPoE` | Same ordered list; no L2TP command. |
| AC-3 | Done | Both `TestPPPoEAuthPolicyRepairCommands` subtests | Actual error alternatives execute and re-extract valid authenticated policy. |
| AC-4 | Done | `TestAuthRefusalUnknownMethodRepair`, `TestAuthRefusalTerminationCleanup`, `TestAuthAllowSelectionMatrix` | No invented method; required refusal and permitted-alternative control remain. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Draft transport refusal test | Changed | `internal/component/l2tp/ppp/auth_repair_commands_test.go`, `TestAuthRefusalRepairCommands` | Actual name and stronger editor carrier; coordinator PASS both transports. |
| PPPoE repair alternatives | Done | `internal/component/cli/pppoe_repair_commands_test.go`, `TestPPPoEAuthPolicyRepairCommands` | Coordinator PASS both alternatives. |
| Editor functional regression | Done | `TestAuthRefusalRepairCommands`, `RunETContentIn` | Actual generated ET input executes printed commands and checks full rendered list. Both old-producer transport subtests fail the command-count guard; current producers pass the complete editor workflow under race. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/ppp/auth.go` | Done | Transport/member message hunk in `18ea5e830a`. |
| `internal/component/l2tp/pppoe/config.go` | Done | Executable alternatives hunk in `18ea5e830a`. |
| Driver/config and editor regression tests | Done | New `auth_repair_commands_test.go`, `pppoe_repair_commands_test.go`; existing `auth_policy_metrics_test.go` updated with independent approval. |
| `docs/guide/l2tp.md`, `docs/guide/pppoe.md` | Done | Both source-anchored editor examples in `18ea5e830a`. |
| Wiki authentication sections | Done | `l2tp.md`, `pppoe.md` in wiki `312639d`. |

### Audit Summary

15 items: 14 Done, 1 Changed (the stronger renamed refusal test); none Partial
or Skipped. The functional carrier's location is explained under Deviations.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Refusal repairs accepted by the real editor | Functional ET | `TestAuthRefusalRepairCommands` invokes `RunETContentIn` with every produced command and checks ordered readback; both transport subtests PASS in the central first-auth and race jobs. |
| PPPoE error repairs accepted and valid | Real command dispatcher | `TestPPPoEAuthPolicyRepairCommands` extracts the error, dispatches each alternative and re-extracts CHAP-MD5 only with `AllowNoAuth=false`; central first-auth PASS. |
| Keep transport scope and operator policy choice | Negative controls | Both transport metrics subtests PASS with only the correct path, exactly one refusal and original peer/policy text; unknown/Reject offers no invented command; required refusal and permitted-method controls PASS in the race job. |

The coordinator's `cpe16c-old-repair-producers-red` run compiled an overlay of
the two real production message writers, restoring the pre-fix strings. Both
`TestAuthRefusalRepairCommands` transport subtests failed because the old text
contained two bracketed commands instead of one command per retained/added
method. `TestPPPoEAuthPolicyRepairCommands` failed because the old error did
not identify both executable alternatives. These are command-count/shape
failures before editor dispatch; this run does not claim that the old bracketed
text reached the editor parser. The restored-current-producer race run passed
both actual editor variants (`TestAuthRefusalRepairCommands`, 1.29s) and both
PPPoE alternatives (`TestPPPoEAuthPolicyRepairCommands`, 0.91s), with package
results PPP `ok` (2.341s) and CLI `ok` (1.986s).

No new interop scenario is required for this diagnostic/config-only change:
the changed hunks alter neither wire decisions nor session admission. Existing
policy tests are retained solely as preservation controls.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All F-3 implementation and documentation deliverables are implemented and centrally tested. Wider MRU behavior and RFC record refresh belong to their existing child scopes. | Not applicable |

## Review Gate

Round 1 scope, declared before the review: the complete 16c production message
hunks, both added test files, the changed metrics assertions, source-aware
guide/wiki claims, and the parser/policy/session consumers they reach. The
three lenses were logic+wiring, security+edge cases, and completeness+style.
No source fix was required. No review round is added for record corrections.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-16c-auth-policy-repair-commands-3a525723-d369-4547-9124-fe35463dc0f5.md`; coordinator recorded CLEAN over the ten-file reviewed population. |
| `./le spec review check` | PASS: coordinator observed `review_gate OK` for five code files, verdict clean and matching hashes; the recorded ten-file population includes guides and closure/citation prose. |
| Rounds | 1 |
| Reviewer lenses used | Logic/wiring; security/edge cases; completeness/Go style. |
| Source review | 0 BLOCKER, 0 ISSUE; coordinator's real old-producer semantic red and restored-current-producer race green are observed. |

The style pass covered all five 16c Go files. No new panic, unbounded loop,
goroutine, lifecycle resource, identity wrapper or pre-validation mutation was
introduced. Command values are enum spellings and path constants; production
PPP remains independent of the editor. Refusal formatting uses `textbuf` on
the existing error path. Existing cleanup obligations remain intact.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| None | None | No runtime issue or missing implementation found by the independent source pass. | 16c message producers and consumers | No source fix needed. |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/ppp/auth_repair_commands_test.go` | Yes | Read full file; contains both refusal repair tests and generated ET carrier. |
| `internal/component/cli/pppoe_repair_commands_test.go` | Yes | Read full file; contains both real-dispatch alternatives. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Executable L2TP ordered repair | Source read of actual `refuseAuthMethod` and complete `TestAuthRefusalRepairCommands`; central race output contains PASS `/l2tp`. |
| AC-2 | Executable PPPoE ordered repair | Same source/test read; race PASS `/pppoe` and metrics `/PPPoE`, with other-transport rejection. |
| AC-3 | Both invalid-policy repairs succeed | Read `ExtractParameters` and full `TestPPPoEAuthPolicyRepairCommands`; first-auth PASS both alternatives. |
| AC-4 | Unknown/Reject stays actionable and required auth remains closed | Race PASS `TestAuthRefusalUnknownMethodRepair`, `TestAuthRefusalTerminationCleanup` and policy matrix permitted controls. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Actual refusal reason to command parser and candidate list | Generated ET in `TestAuthRefusalRepairCommands` | `RunETContentIn` runs each extracted command through the real editor; complete rendered list checked in both transport subtests. |
| Actual PPPoE configuration error to valid policy | `TestPPPoEAuthPolicyRepairCommands` real dispatcher carrier | Both alternatives execute in a real editor then call `ExtractParameters` over its candidate tree; authenticated result asserted. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Actual `cmdSet` consumes one final value; `setLeafListMember` adds one unique member. Printed commands pass the real editor and ordered readback in both transport variants. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| One member command and one transport path | `refuseAuthMethod`, `Model.cmdSet`, `setLeafListMember` | Both guide paragraphs and wiki paragraphs read against the producers; correct. |
| PPPoE default restoration | `ExtractParameters`, `ResolveAuthAllow`, `Editor.DeleteValue` | Delete removes the explicit override; method-derived CHAP-MD5 restored in the actual repair test. |
| Unchanged categories | `rg` source-anchor census includes the guides, configuration reference, editor guide and subscriber architecture. | No grammar/schema/API/SDK/wire/capability changes in 16c; bracketed file examples remain valid. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | `Subsystem.Reload`, `parametersFromProvider`, `ExtractParameters` | Declared Design page read: boot/reload still share the same parser; changing repair text leaves the described configuration behavior accurate. |
| `docs/research/l2tpv2-ze-integration.md` | `ResolveAuthAllow`, shared PPP auth dispatch | Declared Design page read: policy resolution and authentication dispatch unchanged; page carries no old editor-repair syntax. |
| `docs/architecture/l2tp/subscriber-session-model.md` | `awaitAuthDecision`, `decision.accept` branch | Related source-anchor page read: only accepted credentials replace the subscriber username; unchanged by the refusal-text hunk. |

### Central Checks and Limitations

| Check | Observed result and scope |
|-------|---------------------------|
| Focused editor/auth race run | Coordinator first-auth job PASS: both refusal editor subtests, unknown/Reject, both PPPoE alternatives, policy matrix and termination controls. No broader CLI-package green is claimed. |
| Runtime race | Coordinator postreview runtime job PASS for every L2TP package and L2TP/PPPoE checker packages; includes both refusal metric subtests and the editor refusal workflow. |
| Broader prechecks | Coordinator reports repo-check foreign unused exports and weakened-check foreign findings. The 16c protected metrics expectation was approved in `18ea5e830a`; no remaining 16c weakened test is identified. |
| Documentation | Coordinator `./le doc check all` reported generated published catalog drift and unrelated pages; the two repair-guide paragraphs match source. Wider corrections remain centrally tracked. |
| RFC | No 16c-added test contains an RFC requirement tag. The coordinator's corpus check has broader findings and 16a-owned stale producer records; no RFC-green result is claimed for this closure. |
| Semantic discrimination | Actual old-producer overlay RED: both PPP transport tests reject command count/shape, CLI test rejects alternatives shape. Restored current producers race GREEN: both editor transport variants and both config-repair alternatives pass. No old-text editor-parser execution is claimed. |

### Retained Command and Observed Output

| Centrally executed command | Directly observed result |
|---------------------------|--------------------------|
| `./le --name cpe16 job run label cpe16-first-auth-image quiet command go test -race -tags ze_core,ze_anomaly,ze_as112,ze_bfd,ze_bgp,ze_bmp,ze_copp,ze_cos,ze_ddos,ze_dhcpserver,ze_exabgp,ze_flowexport,ze_geodns,ze_gnmi,ze_grpc,ze_ike,ze_isis,ze_l2tp,ze_ldp,ze_lg,ze_mcp,ze_mpls,ze_ntp,ze_ospf,ze_policyroute,ze_pxe,ze_radius,ze_rest,ze_rsvpte,ze_ssh,ze_tacacs,ze_telemetry,ze_trafficusage,ze_vpp,ze_vrrp,ze_web ./internal/component/l2tp/ppp ./internal/component/cli ./internal/le/interoplab -run 'Test(AuthRefusalRepairCommands\|AuthRefusalUnknownMethodRepair\|PPPoEAuthPolicyRepairCommands\|AuthAllowSelectionMatrix\|AuthRefusalTerminationCleanup\|LACPeerReleasePinsAgree)$' -v -count=1` | PASS: both refusal/editor transport variants, unknown/Reject, both PPPoE repair alternatives, policy matrix, termination controls and release-pin check. PPP `ok` (2.383s), CLI `ok` (1.971s), interoplab `ok` (1.068s). |

All feature tags from `feature-gates.txt` were enabled, matching the
coordinator's actual run. The canonical retained evidence is the named tests
and source committed in `18ea5e830a`; session logs, including
`job-cpe16c-old-repair-producers-red-18f3f145.log` and
`job-cpe16c-repair-producers-restored-green-1cc32b69.log`, are transient
debugging aids rather than retained proof files.
