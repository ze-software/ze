# 803 -- gNMI Component

## Context

Ze had YANG-modeled config, a gRPC server for its custom API, and a ConfigSessionManager
for transactional edits, but no standardized network management interface. Network
automation platforms (gnmic, Ansible, controllers) use gNMI as the industry-standard
protocol for YANG-modeled device management. Adding gNMI makes Ze addressable by the
same tooling used for Cisco, Juniper, and Arista devices.

## Decisions

- Chose separate component (`internal/component/gnmi/`) over adding gNMI as a third
  transport under `api/`, because gNMI is a standardized protocol with its own proto
  service definition and subscription semantics, not another view of Ze's custom API.
- Chose own gRPC server on a separate port (default 9339) over sharing the Ze API
  gRPC server, because automation tools expect gNMI on a known port and independent
  TLS/auth keeps configuration clean.
- Chose reusing ConfigSessionManager for Set over direct tree manipulation, because
  it provides atomicity, validation hooks, and commit/discard semantics for free.
- Chose JSON_IETF as primary encoding over PROTO, because it is human-readable and
  what gnmic, Ansible, and most gNMI clients use by default.
- Chose `default:` (non-blocking drop) for Subscribe notify over blocking send,
  because a slow subscriber must not block config commits. `time.After(0)` was
  tried first but races with the send in select (both cases are immediately ready).

## Consequences

- Ze can now be managed by any standard gNMI client when `ze.gnmi.enabled=true`.
- YANG path stability matters more: gNMI clients hardcode paths, so renames become
  breaking changes. Consider YANG module versioning before exposing unstable paths.
- Telemetry SAMPLE mode (periodic counter streaming) is not included; it needs deeper
  telemetry collector integration and should be a separate spec.
- The `github.com/openconfig/gnmi` dependency is vendored but only linked when the
  gnmi package is imported (optional component).

## Gotchas

- `time.After(0)` in a select with a channel send is not equivalent to `default:`.
  Both cases are immediately ready, so the runtime picks randomly, causing dropped
  notifications ~50% of the time. Use `default:` for non-blocking sends.
- `yang.Module.Organization` and `Revision` are pointer/slice types (not strings),
  requiring nil checks before accessing `.Name`.
- Ze's config tree uses positional list keys (the key value is a direct child name
  in the list map), not named keys. gNMI PathElem keys must be mapped to this model.
- `ConfigSessionManager.splitPath` splits on **dots**, not spaces. Joining gNMI path
  segments with spaces produces an unparseable single-segment path. Always `strings.Join(segments, ".")`.
- Token comparison must use constant-time (`sha256` + `subtle.ConstantTimeCompare`)
  to match the existing Ze API pattern and avoid timing side-channels.

## Files

- `internal/component/gnmi/` -- server, capabilities, get, set, subscribe, path, errors
- `internal/component/config/environment.go` -- env var registration
- `cmd/ze/hub/main.go` -- gNMI startup wiring
- `cmd/ze/hub/main_servers.go` -- serveGNMI helper
