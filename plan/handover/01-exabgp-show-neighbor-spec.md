# Handoff: spec for the ExaBGP bridge `show neighbor` translation

**Agreed with Thomas (2026-10-31):** write a new spec, in `plan/release/fixes/`,
for the bridge to translate ExaBGP's `show neighbor` (v6 spelling `peer show`)
instead of refusing it. Run `/ze-spec` in a fresh session; this file is the
starting point and is removed when that spec exists.

## What is known

| Fact | Source |
|------|--------|
| ExaBGP 4.2, 5.0 and 6.0 all have `show neighbor` with `summary`, `extensive` and `configuration` modes | `def show_neighbor` in `4.2/lib/exabgp/reactor/api/command/neighbor.py`, `5.0/src/exabgp/reactor/api/command/neighbor.py`, `main/src/exabgp/reactor/api/command/neighbor.py` under `~/Code/github.com/exa-networks/exabgp/` |
| Only 6.0 prints `DISABLED`: an operator-disabled peer whose FSM is Idle | 6.0 `src/exabgp/reactor/peer/peer.py`: `'state': 'DISABLED' if self._disable is not None and self.fsm == FSM.IDLE else self.fsm.name()`. No `DISABLED` in the 4.2 or 5.0 peer code |
| The bridge refuses the line today | `internal/exabgp/bridge/bridge_control_test.go`, refused table (`{"peer show", "show neighbor"}`, `ErrLineNotTranslated`) |
| ExaBGP compatibility is agreed, so the translation is owed work, not a question | `ai/META-DECISIONS.md`, "ExaBGP compatibility" |

## Decisions already made

- Own spec, not folded into `spec-exabgp-bridge-refuses-untranslated` (that spec refuses what is not translated) nor into `spec-bgp-peer-state-fsm-and-reason` (Ze's own state display).
- Depends on `plan/release/fixes/spec-bgp-peer-state-fsm-and-reason.md`: `DISABLED` is printed exactly when that spec's answer has `state` `idle` and `state-reason` `operator-shutdown`, and ExaBGP's `FSM.name()` spelling otherwise (that spec, requirement E-6). A 4.2 or 5.0 script gets the plain FSM name.

## Open for the spec's research

- The exact bytes of each mode's template (`NeighborTemplate.summary`, extensive, configuration) per version, and which ze answer fields feed each column.
- Whether the standalone runner (`ze exabgp plugin`) can answer it, or refuses it by name as it refuses `status`.
