# Spec: Make PPP authentication policy repair commands executable

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli, config, docs |
| Priority | P1 |
| Release reason | Printed repair commands cannot be accepted by the configuration editor. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

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
