# Quick Start

Get Ze running with two BGP peers in under 5 minutes.

## Build

```bash
git clone https://github.com/ze-software/ze.git
cd ze
make build    # produces bin/ze, bin/ze-test, bin/ze-chaos
```

Requires **Go 1.26+** on a macOS or Linux development host. Windows is not a supported development platform.

### Or: go install

To just get a `ze` binary without cloning the repo, `go install` with the same
build tags `make build` uses:
<!-- source: Makefile -- build target, "ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)" -->

```bash
go install -tags 'ze_core ze_distro ze_anomaly ze_as112 ze_bfd ze_bgp ze_bmp ze_copp ze_cos ze_ddos ze_dhcpserver ze_exabgp ze_flowexport ze_geodns ze_gnmi ze_grpc ze_ike ze_isis ze_l2tp ze_ldp ze_lg ze_mcp ze_mpls ze_mrt ze_ntp ze_ospf ze_policyroute ze_pxe ze_radius ze_rest ze_rsvpte ze_ssh ze_tacacs ze_telemetry ze_trafficusage ze_vpp ze_vrrp ze_web' github.com/ze-software/ze/cmd/ze@latest
```

This tracks the module's default branch (development version), not a
tagged release -- there are no tagged releases yet.

## Initialize

Ze runs an SSH server on localhost for CLI access (`ze cli`, `ze show`, `ze signal`). This keeps the control plane authenticated even in multi-user environments. Set up credentials once:

```bash
bin/ze init
```

This prompts for username, password, SSH host (default `127.0.0.1`), port (default `2222`), and node name (default: hostname). Credentials are stored locally with bcrypt-hashed passwords. For scripting (later fields fall back to their defaults):
<!-- source: internal/plugins/init/main.go -- Run, defaultHost, defaultPort, bcrypt hashing -->

```bash
echo -e "admin\nsecret" | bin/ze init
```

Running `ze init` a second time will refuse with `error: database already exists`. To reinitialize, use `--force` -- this backs up the old database as `database.zefs.replaced-<date>` before creating a new one:

```bash
bin/ze signal stop             # stop daemon first
bin/ze init --force            # prompts for confirmation, then backs up and reinitializes
```
<!-- source: internal/plugins/init/main.go -- forceFlag -->

<!-- terminal-demo: zefs-config -->

## Minimal Config

Save as `example.conf`:

```
static {
    table default {
        route 172.16.0.0/24 {
            next {
                hop 10.0.0.1 {
                }
            }
        }
    }
}

redistribute {
    destination bgp {
        import static
    }
}

bgp {
    router-id 10.0.0.1
    session {
        asn {
            local 65000
        }
    }

    peer test-peer {
        connection {
            remote {
                ip 10.0.0.2
            }
            local {
                ip 10.0.0.1
            }
        }

        session {
            asn {
                local 65000
                remote 65001
            }
            family {
                ipv4/unicast {
                    prefix {
                        maximum 1000000
                    }
                }
            }
        }

        update {
            attribute {
                origin igp
                next-hop 10.0.0.1
            }
            nlri {
                ipv4/unicast add 192.168.1.0/24
            }
        }
    }
}
```

This advertises two prefixes to `test-peer`, one by each of the routes Ze gives you to get
a prefix into BGP:

- **Redistribution** (`172.16.0.0/24`): a `static` route pulled into BGP by the top-level
  `redistribute { destination bgp { import static } }` block. Every source protocol
  (`static`, `connected`, `kernel`, `ospf`, `isis`, ...) redistributes into any destination
  the same way -- this is how interface and interior-gateway routes reach your peers.
  <!-- source: internal/plugins/static/register.go -- registerStaticSources; internal/component/config/loader_redistribute.go -- ExtractRedistributeRules -->
- **Direct announcement** (`192.168.1.0/24`): a prefix declared inline on the peer in its
  `update {}` block, sent as soon as the session establishes.
  <!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendInitialRoutes -->

> Advanced: a `process` binding attaches a plugin or your own external program to a peer (the
> ExaBGP-style event API), including making a peer RIB-backed so it stores and re-advertises
> received routes. It is not needed for the config above; see [Plugins](plugins.md).

## Validate

```bash
bin/ze config validate example.conf
```
<!-- source: internal/component/config/cli/cmd_validate.go -- cmdValidate -->

Expected output:

```
configuration valid: example.conf
```

## Start

```bash
bin/ze example.conf
```

Ze logs to stderr. You should see something like:

```
level=INFO  msg="hub ready" subsystem=hub plugins=1 peers=1 listen=":179"
level=INFO  msg="peer connecting" subsystem=bgp.reactor peer=test-peer address=10.0.0.2
```

Silence means the default log level (`warn`) has nothing to report -- that's normal. To see all activity:

```bash
bin/ze -d example.conf        # debug logging
```
<!-- source: cmd/ze/main.go -- "-d" debug flag sets ze.log=debug -->

## Verify

In another terminal:

```bash
# Check daemon is running
bin/ze status

# List peers
bin/ze cli -c "show bgp peer list"

# Show peer details
bin/ze cli -c "show bgp peer test-peer detail"

# Watch live events (streams until Ctrl-C)
bin/ze cli -c "monitor event"
```
<!-- source: internal/component/cli/client/main.go -- Execute, StreamMonitor -->

## Test Without a Real Peer

Use the built-in test peer to accept any BGP session:

```bash
# Terminal 1: start a sink peer (accepts sessions, replies keepalive)
bin/ze-test peer --mode sink --port 1179 --asn 65001

# Terminal 2: start ze with config pointing to localhost:1179
bin/ze example-local.conf
```
<!-- source: internal/test/cli/cmd_peer.go -- ze-test peer command -->

Where `example-local.conf` is the config above with the peer's `connection`
block pointed at the local sink, so ze dials `127.0.0.1:1179` instead of
`10.0.0.2`:

```
        connection {
            remote {
                ip 127.0.0.1
                port 1179
            }
            local {
                ip 127.0.0.1
            }
        }
```

The sink's `--asn 65001` matches the peer's `session { asn { remote 65001 } }`.

## Stop

```bash
bin/ze signal stop             # graceful shutdown
bin/ze signal restart          # graceful restart (preserves routes via GR)
```
<!-- source: internal/plugins/signal/main.go -- Run -->

## Next Steps

- [Configuration](configuration.md) -- peer groups, capabilities, static routes
- [Plugins](plugins.md) -- RIB, route server, RPKI, graceful restart
- [CLI Reference](cli.md) -- interactive CLI, route injection, monitoring
- [Logging](logging.md) -- log levels, backends, per-subsystem tuning
- [Operations](operations.md) -- SSH setup, signals, health checks, troubleshooting
