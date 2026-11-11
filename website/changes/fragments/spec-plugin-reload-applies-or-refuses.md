---
spec: spec-plugin-reload-applies-or-refuses
topic: Config
found-by: review
---

## Before

A reload could report success while a plugin kept its startup configuration.
Some listener failures were logged without refusing the change. A failed
rollback could also say a plugin still ran merely because its restart was
refused.

## After

Plugins apply supported changes at commit and restore their previous
configuration on rollback, or refuse unsupported changes with restart
instructions. A plugin without an apply handler must explicitly declare that
it has nothing to apply; otherwise verification refuses the reload.

VLAN QoS map changes replace the existing map, including removal of omitted
entries and clearing an empty map. Rollback errors distinguish restart
outcomes and direct operators to current process status without inferring
liveness from a refusal. Filter changes affect subsequent updates; this does
not re-evaluate routes already stored or advertised.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
