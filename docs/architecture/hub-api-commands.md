# Hub API Commands Reference

This page once specified the hub's text-line protocol: `declare schema ...`,
`declare cmd ...` and `declare done` at Stage 1, `config verify`,
`config apply`, `query config live|edit` and `config done` at Stage 2, then
`capability hex ...`, `registry cmd ...` and `ready` for Stages 3 to 5. That
protocol is retired. The daemon reads none of those lines, and the last Go code
that wrote them, the `Plugin` type of `pkg/plugin`, was removed once nothing
called it. The `ze <subsystem> schema show|validate|complete` commands the page
proposed were never built.

The page now says where each subject it covered is documented today.

## Where each subject lives

| Subject | Page |
|---------|------|
| Plugin startup: the five stages, each a YANG RPC, and the answer to each | `docs/architecture/api/process-protocol.md`, "5-Stage Startup Protocol" |
| How a plugin brings its YANG modules (Stage 1 `schemas`) | `docs/plugin-development/schema.md` |
| How request and answer lines are framed | `docs/architecture/api/process-protocol.md`, "Wire Format", and `docs/architecture/api/ipc_protocol.md` |
| A config change after start: verify, apply and rollback for each plugin's own roots | `docs/architecture/config/transaction-protocol.md` |
| Command dispatch by YANG path, and where each command is served | `docs/architecture/api/commands.md` |
| Event subscription | `docs/architecture/api/process-protocol.md`, "Event Subscription" |
| The hub's own startup | `docs/architecture/hub-architecture.md` |

The loaded schemas are inspected with `show schema` (`list`, `methods`,
`events`, `handlers`, `protocol`), which the config schema plugin owns.
<!-- source: internal/plugins/config-schema/yang/ze-schema-cli-cmd.yang -- show schema commands -->

## Cross-subsystem references

This section is design only: no Ze module carries the leaf below. Ze checks
a leafref value against the referred leaf's instances in the whole config
(`Validator.ValidateReferences`), so a module that declared it would have the
reference checked at validate and commit.
<!-- source: internal/component/config/yang/validator_instance.go -- ValidateReferences -->

When BGP references system interfaces (leafref to another subsystem):

```yang
// In ze-bgp-conf.yang
leaf update-source {
  type leafref {
    path "/system/interface/name";
  }
}
```
