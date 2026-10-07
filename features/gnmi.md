# gNMI

## Meta

| Field | Value |
|-------|-------|
| Name | gNMI |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/gnmi |
| Real-path tests | test/plugin/gnmi-show.ci |
| Docs | docs/guide/gnmi.md |
| Doc review | 2026-10-07: default port 9339 in ze-gnmi-conf.yang, subtle.ConstantTimeCompare at server.go, the three named metrics in internal/component/gnmi; Get, Set and Subscribe are unproven at the daemon rather than known false |
| Defect review | 2026-10-07: plan/immediate/spec-login-identity-for-looking-glass-and-gnmi.md is design scope (caller identity), not a defect in the stated behavior; no journal row names internal/component/gnmi |
| Extra criteria | supported: Capabilities, Get, Set and Subscribe STREAM against a running daemon = test/plugin/gnmi-get-set-subscribe.ci; supported: gnmic drives the daemon = test/interop/gnmi-gnmic.md |

## Description

Industry-standard gRPC Network Management Interface for YANG-modeled config. Capabilities, Get, Set (via segment-based paths preserving IP list keys), Subscribe ONCE and STREAM modes. Bearer token auth with constant-time comparison, optional TLS. YANG config schema under `environment { gnmi {} }`, `show gnmi` CLI command, Prometheus counters (`ze_gnmi_requests_total`, `ze_gnmi_subscribe_active`, `ze_gnmi_errors_total`). External config commits (web, CLI, managed) notify STREAM subscribers. Env-var gated (`ze.gnmi.enabled`), default port 9339. The service is default-on but compile-out-able with the `ze_gnmi` build tag: normal and appliance builds include it, while `ze-stripped` and bare `ze_core` builds drop `internal/component/gnmi`, its schema, and its `show gnmi` RPC. <!-- source: internal/component/gnmi/server.go -- gNMI gRPC server --> <!-- source: internal/component/gnmi/yang/ze-gnmi-conf.yang -- YANG config --> <!-- source: feature-gates.txt -- ze_gnmi --> <!-- source: cmd/ze/hub/gnmi_infra.go -- gnmiBuild --> <!-- source: internal/component/plugin/all/all_ze_gnmi.go -- gated gNMI imports -->
