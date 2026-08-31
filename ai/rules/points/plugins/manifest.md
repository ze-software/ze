---
title: Plugins
when: "creating or changing a plugin: its registration, placement, transport, command surface, process boundary, dispatch table, or a feature gate"
severity: blocking
related: repo-maintenance, cli, evidence
---
directives ## Directives
  all-plugins-must-follow-these-patterns
  a-plugin-owns-its-entire-feature-surface
  give-every-rpc-a-yang-registration
  cross-a-boundary-with-value-types-only
  bound-and-clean-every-declared-text
registration-based-dispatch ## Registration-Based Dispatch
  dispatch-subcommands-by-registration-not-switch
