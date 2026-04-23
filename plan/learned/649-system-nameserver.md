# 649 -- Static DNS Name-Servers and Resolver Unification

## Context

Ze's DNS configuration was split across two disconnected locations: `environment { dns { server; timeout; cache-size; cache-ttl; } }` (registered but never wired into the resolver) and per-interface `resolv-conf-path` in the iface config. Operators had no way to configure static DNS servers. The goal was to add a VyOS-style `system { name-server }` leaf-list and unify all DNS configuration under `system {}`, retiring the unused `environment { dns {} }` YANG module.

## Decisions

- Moved DNS config from `environment { dns {} }` to `system { name-server; dns { timeout; cache-size; cache-ttl; resolv-conf-path; } }` as a clean break, over a migration shim, because the old config was never functional (env vars registered but never consumed).
- `name-server` is a leaf-list at the system level (not nested under `dns {}`) to match VyOS syntax and keep the common case simple.
- DNS tuning lives in `system { dns {} }` container, separate from the `name-server` leaf-list, to group resolver internals away from the user-visible server list.
- `resolv-conf-path` moved from per-interface iface config to `system { dns {} }` because static name-servers need a single resolv.conf, not per-interface files.
- Static name-servers take priority over DHCP-discovered servers: when static servers are configured, DHCP skips resolv.conf writes.
- Retired `ze.dns.*` env vars and removed the `resolve/dns/schema` YANG module entirely.

## Consequences

- Single source of truth for DNS: `system { name-server }` feeds both resolv.conf and ze's internal resolver.
- DHCP and static DNS coexist cleanly: DHCP writes resolv.conf only when no static servers are configured.
- `environment { dns {} }` config is now a validation error, forcing operators to migrate.
- The `resolv-conf-path` default (`/tmp/resolv.conf`) targets gokrazy deployments where `/etc/resolv.conf` is read-only.

## Gotchas

- The `environment { dns {} }` YANG existed with registered env vars but was never wired into the resolver. Removing it was safe but required verifying no code path consumed those env vars.
- `resolv.conf` writes use atomic rename (write to temp, rename) on Linux; the `resolv_other.go` stub is a no-op for non-Linux platforms.
- The hub creates resolvers after config load, so `ExtractSystemConfig` has access to the full config tree.

## Files

- `internal/component/config/system/schema/ze-system-conf.yang` - `name-server` leaf-list + `dns {}` container
- `internal/component/config/system/system.go` - `ExtractSystemConfig` with DNS fields
- `internal/component/config/system/resolv_linux.go` - atomic resolv.conf writer
- `cmd/ze/hub/main.go` - hub wiring for resolver config
- `internal/plugins/iface/dhcp/` - DHCP coordination (skip resolv.conf when static)
- `docs/features/dns-resolver.md`, `docs/guide/configuration.md` - updated docs
