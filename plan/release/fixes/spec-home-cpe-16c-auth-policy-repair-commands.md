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

- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`.
  → Constraint: follow the existing command/editor grammar and keep shared PPP independent of the editor component.
- [ ] `docs/guide/l2tp.md`, authentication; `docs/guide/pppoe.md`, subscriber authentication; `docs/guide/config-editor.md`.
  → Constraint: distinguish configuration-file leaf-list syntax from executable editor commands.
- [ ] `ai/rules/user-facing-errors.md` and `ai/rules/cli.md`.
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
| A-1 | Existing editor operations can express the intended list without bracketed tokens. | Read the producing editor handler and run generated commands. | Unvalidated; derive the repair from actual grammar, never a guessed replacement. |

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
