# Named Service Listeners

## Meta

| Field | Value |
|-------|-------|
| Name | Named Service Listeners |
| Kind | daemon |
| Scope | partial |
| Scope gaps | parse-time conflict check over endpoints left at their YANG default |
| Level | experimental |
| Components | internal/component/config/listener.go, internal/component/config/loader_extract.go, internal/component/doctor/checks_listener.go |
| Real-path tests | test/parse/listener-conflict-wildcard.ci, test/parse/listener-conflict-api.ci, test/parse/listener-no-conflict.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: CollectListeners without defaults and CollectListenersWithDefaults used only by doctor, as the row states |
| Defect review | 2026-10-07: plan/journal/gate-excludes-part-of-its-population.md (2026-08-23) defaulted endpoints, carried as a Scope gap; plan/journal/ipv6-address-built-by-concatenation.md (2026-08-10) ServerEndpoint.Listen bare-colon join, unfixed |
| Extra criteria | supported: an IPv6 listener binds with a bracketed address = test/parse/listener-ipv6-literal.ci |

## Description

Every service that accepts inbound connections (web, ssh, mcp, looking-glass, telemetry, REST, gRPC, plugin hub) models its listen endpoints as a named YANG list. Each entry binds its own listener on the same subsystem; bind is all-or-nothing with rollback on failure. At config parse time `CollectListeners` refuses two configured endpoints that share an `ip:port`, across services. It does not fill in YANG defaults, so an endpoint left at its default is compared only by `ze doctor` (`CollectListenersWithDefaults`), and `ServerEndpoint.Listen` joins an IPv6 host to its port without brackets. <!-- source: internal/component/config/yang/modules/ze-types.yang -- grouping listener --> <!-- source: internal/component/config/yang/modules/ze-extensions.yang -- extension listener --> <!-- source: internal/component/config/listener.go -- CollectListeners, ValidateListenerConflicts, CollectListenersWithDefaults --> <!-- source: internal/component/doctor/checks_listener.go -- CollectListenersWithDefaults caller --> <!-- source: internal/component/config/loader_extract.go -- ServerEndpoint.Listen -->
