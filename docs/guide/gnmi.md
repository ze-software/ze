# gNMI Guide

Ze includes a gNMI (gRPC Network Management Interface) server that exposes YANG-modeled configuration over the industry-standard gNMI protocol. This allows management by tools like gnmic, Ansible, and network controllers that speak gNMI.

Build flavor: gNMI is compiled into default `make ze` and `make ze-appliance-build` builds through `ze_gnmi`. Minimal `ze_core` and `ze-stripped-build` builds omit the gNMI server, config schema, and `show gnmi`; custom minimal builds that need gNMI must include `-tags "ze_core ze_gnmi"`.
<!-- source: feature-gates.txt -- ze_gnmi -->
<!-- source: Makefile -- ZE_FEATURES, ze-stripped-build -->
<!-- source: cmd/ze/hub/gnmi_infra.go -- gnmiBuild -->
<!-- source: internal/component/plugin/all/all_ze_gnmi.go -- gated gNMI imports -->

## Enabling gNMI

gNMI is disabled by default. Enable it via environment variables or YANG config.

Environment variables:

    ze.gnmi.enabled=true
    ze.gnmi.listen=0.0.0.0:9339
    ze.gnmi.token=your-secret-token

YANG config (in ze.conf):

    environment {
        gnmi {
            enabled true;
            token your-secret-token;
            server default {
                ip 0.0.0.0;
                port 9339;
            }
        }
    }

## Authentication

When `ze.gnmi.token` is set, every gNMI request must include a `Bearer` token in the `authorization` gRPC metadata header. The token is compared using constant-time comparison (SHA-256 hash with `subtle.ConstantTimeCompare`) to prevent timing attacks.

When no token is configured, the server accepts unauthenticated requests. This is only appropriate for loopback or trusted-network deployments.

## TLS

To enable TLS on the gNMI gRPC transport:

    ze.gnmi.tls.cert=/path/to/cert.pem
    ze.gnmi.tls.key=/path/to/key.pem

Or in YANG config:

    environment {
        gnmi {
            tls {
                cert /path/to/cert.pem;
                key /path/to/key.pem;
            }
        }
    }

TLS certificates are loaded once at startup. Certificate rotation requires restarting the gNMI server.

## Supported RPCs

| RPC | Description |
|-----|-------------|
| Capabilities | Returns supported YANG models, encodings (JSON_IETF), and gNMI version |
| Get | Reads config state from the running tree by path |
| Set | Applies config modifications atomically (update, replace, delete) |
| Subscribe ONCE | Snapshot of current config for requested paths |
| Subscribe STREAM | Continuous change notifications (from gNMI Set and external commits) |

## CLI Status

Check gNMI server status from the SSH CLI when `ze_gnmi` is compiled in:

    show gnmi

Returns JSON with: enabled, listen address, token configured, TLS configured, and active subscriber count.
<!-- source: internal/component/gnmi/show.go -- handleShowGNMI -->
<!-- source: internal/component/plugin/all/all_ze_gnmi.go -- gated gNMI RPC import -->

## Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_gnmi_requests_total` | counter | rpc | Total gNMI RPC requests |
| `ze_gnmi_subscribe_active` | gauge | -- | Active Subscribe STREAM connections |
| `ze_gnmi_errors_total` | counter | rpc, code | gNMI RPC errors by gRPC status code |

## Usage with gnmic

Example using gnmic to get the full config tree:

    gnmic -a 192.0.2.1:9339 --token your-secret-token get --path /

Example to subscribe to config changes:

    gnmic -a 192.0.2.1:9339 --token your-secret-token subscribe --path / --mode stream

## Limitations

- Subscribe POLL mode is not implemented
- Telemetry SAMPLE mode (periodic counter streaming) is not supported
- gNMI path target field (multi-target) is not supported
- TLS certificates are loaded at startup only; no hot rotation
- External config commits (web/CLI) send a generic config-changed notification, not per-path diffs
