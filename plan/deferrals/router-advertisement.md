# Deferrals -- spec-router-advertisement

Source: `plan/spec-router-advertisement.md`. Format: `ai/rules/deferral-tracking.md`.

Created 2026-08-02 by a session sweeping the shared working tree, NOT by the spec's
author. The spec's 2026-07-10 scoping decision excludes two RA options, and the commit gate
reads that exclusion as a deferral with no shard. The row below transcribes what the spec
itself says and points at the Known Limitations section it already cites. It adds no
judgement of its own: the author should correct it if the transcription is wrong.

Reference lists in this file are written as bullets, never as tables. Every pipe-delimited
line here is read by the deferral gate as a six-cell row.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-02 | spec-router-advertisement | The DNSSL option and the MTU option in Router Advertisements. RDNSS stays in scope | Scoped out on 2026-07-10 by the spec's own decision, which cites its Known Limitations section for the reasoning. Both are RA options a peer may omit, so excluding them narrows the feature rather than breaking the ones that ship | `plan/spec-router-advertisement.md` | deferred |
