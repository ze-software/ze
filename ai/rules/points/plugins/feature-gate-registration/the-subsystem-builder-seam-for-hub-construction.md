---
kind: directive
level: MUST
stage:
---
**Subsystem-builder seam (`ze_l2tp` hub construction).** When the hub CONSTRUCTS a feature (parses params, `eng.RegisterSubsystem`) rather than blank-importing it, a hub-local nil-able hook MUST be used (`bng_infra.go` `bngRegister`, filled by the gated `register_l2tp.go` init): the ssh/gnmi seam shape carrying only generic values (config trees, engine handle, portal entries) across the boundary.
