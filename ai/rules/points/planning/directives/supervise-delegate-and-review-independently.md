---
kind: directive
level: MUST
stage:
---
- **The main thread supervises. It does not perform the spec work itself.** Each phase runs in a subagent through its `ze-*` skill, except the four the `Runs in` column names ("Spec Work Runs in Subagents", below).
- **Each phase of Ze work runs on a specific model.** The model is chosen by phase, never by convenience, and never by "the session I happen to be in" ("Model Selection by Work Phase", below).
- **Before closing a spec or claiming a substantive change is done -- review is INDEPENDENT (subagents / fresh session), never the author's own inline reasoning, and is enforced by `commit_helper.py`.**
- **Obligation on you (not a hard gate):** Every decision to not perform in-scope work MUST be recorded AND land in a destination spec.
- **A spec that passes its Review Gate is not done until it is deleted from `plan/`,** and the completed spec MUST be committed to git first so it is preserved in history.
- **When the user asks how to continue, start with a short rationale section, then output exact edits.**
