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
go install -tags 'ze_core ze_distro ze_gnmi ze_grpc ze_isis ze_ldp ze_lg ze_mcp ze_ospf ze_rest ze_rsvpte ze_ssh ze_telemetry ze_vrrp ze_web' codeberg.org/thomas-mangin/ze/cmd/ze@latest
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

### Demo: Create ZeFS and commit over SSH

Create the ZeFS database, edit the active configuration through Ze's SSH management plane, and verify the committed setting.

[Play the WebM recording](../../../assets/demos/zefs-config.webm?v=644b96eda1) · [View the poster](../../../assets/demos/zefs-config.png?v=26fc4f1939) · [Plain-text transcript](../../../assets/demos/zefs-config.txt?v=4253c65985)

Recorded with Ze 26.07.18 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 55 seconds.

```console
$ ze init < "$ZE_INIT_INPUT"
$ ze config ls
ze.conf
$ ze data check

$ ssh ze-demo
ze# run show bgp summary
ze# set environment cli format default table
ze# show | compare
ze# commit
Session committed
ze# run show bgp summary

`ze init` creates `database.zefs`. The first BGP summary uses the default text format. The SSH editor commits the table-format setting back to ZeFS, not to a second flat file, and the same operational command immediately renders as a box-drawing table.
```


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
> received routes. It is not needed for the config above; see [Plugins](../plugins/index.md).

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

- [Configuration](https://github.com/ze-software/ze/blob/main/docs/guide/configuration.md) -- peer groups, capabilities, static routes
- [Plugins](../plugins/index.md) -- RIB, route server, RPKI, graceful restart
- [CLI Reference](https://github.com/ze-software/ze/blob/main/docs/guide/cli.md) -- interactive CLI, route injection, monitoring
- [Logging](https://github.com/ze-software/ze/blob/main/docs/guide/logging.md) -- log levels, backends, per-subsystem tuning
- [Operations](https://github.com/ze-software/ze/blob/main/docs/guide/operations.md) -- SSH setup, signals, health checks, troubleshooting
