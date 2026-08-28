---
kind: note
level: MUST
stage:
---
> A config-driven engine (`sdk.NewWithConn`) at a top-level subsystem MUST be in
> `internal/component/` if a feature depends on it, else in `internal/plugins/`.
>
> A non-engine package outside `internal/core/` MUST either be classified by the
> existing registration mechanics or have a manifest row in
> `internal/le/tier_non_engine_categories.txt`.
