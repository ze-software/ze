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
CGO_ENABLED=0 go install -tags 'ze_core ze_distro ze_anomaly ze_as112 ze_bfd ze_bgp ze_bmp ze_copp ze_cos ze_ddos ze_dhcpserver ze_exabgp ze_flowexport ze_geodns ze_gnmi ze_grpc ze_ike ze_isis ze_l2tp ze_ldp ze_lg ze_mcp ze_mpls ze_mrt ze_ntp ze_ospf ze_policyroute ze_pxe ze_radius ze_rest ze_rsvpte ze_ssh ze_tacacs ze_telemetry ze_trafficusage ze_vpp ze_vrrp ze_web' github.com/ze-software/ze/cmd/ze@latest
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

[Play the WebM recording](../../assets/demos/zefs-config.webm?v=9651a7e3df) · [View the poster](../../assets/demos/zefs-config.png?v=9e82166aac) · [Plain-text transcript](../../assets/demos/zefs-config.txt?v=cf435951c8)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 2 minutes 21 seconds.

```console
$ cat "$ZE_INIT_INPUT"
admin
secret123
127.0.0.1
2222
ze-demo
$ ze init < "$ZE_INIT_INPUT"
$ ze config ls
ze.conf
$ ze data check

$ ssh ze-demo show bgp summary

$ ssh ze-demo
ze# set environment cli format default table
ze# show | compare
ze# commit
Session committed
ze# exit
ze# exit
$ ze cli -c 'show bgp summary'
$ ze cli -c 'show bgp summary | text'
$ ze cli -c 'show bgp summary | raw' | head -14
$ ze cli -c 'show bgp summary | raw' | ze pipe text

The five lines answer `ze init`'s prompts in order: username, password, host, port, and name. It reads them from a file here so the recording is reproducible, and it prints nothing when its input is not a terminal, so the file is shown first rather than left as an unexplained redirection. `ze init` creates `database.zefs`. The first BGP summary uses the default text format. The SSH editor commits the format setting back to ZeFS, not to a second flat file, and the same operational command immediately uses the committed default. The last commands show the two ways to override that default, and they are different pipes. `show bgp summary | text` is Ze's own operator, inside the quoted command, and it wins over the committed setting. Then `| raw` on its own shows what every one of these renderings is made from: the payload as the daemon holds it, unrendered. The last command sends that same payload across a real shell pipe, and `ze pipe text` formats it on this side, which is how output captured earlier is formatted later. The command is `ze pipe` rather than `ze format` because the operator language also carries `match`, `count`, `first`, `last` and `resolve`, so `format` would name one clause of it. Every command answers with structured data, so `text`, `table`, `json`, `yaml` and `ndjson` all render the same payload.
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
bin/ze start example.conf
```
<!-- source: cmd/ze/ze_core_dispatch.go -- registerLocalCommands, "start" root handler; cmd/ze/ze_core_start.go -- cmdStart, startConfigPath -->

The config path goes behind the `start` keyword. A bare `bin/ze example.conf` is rejected with `unknown command: example.conf` (exit 1); global flags such as `-d` are consumed before the keyword, so they stay ahead of it.

Ze logs to stderr. You should see something like:

```
level=INFO  msg="hub ready" subsystem=hub plugins=1 peers=1 listen=":179"
level=INFO  msg="peer connecting" subsystem=bgp.reactor peer=test-peer address=10.0.0.2
```

Silence means the default log level (`warn`) has nothing to report -- that's normal. To see all activity:

```bash
bin/ze -d start example.conf  # debug logging
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
bin/ze start example-local.conf
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
                ip 127.0.0.2
            }
        }
```

The sink's `--asn 65001` matches the peer's `session { asn { remote 65001 } }`.

**The two ends carry different addresses on purpose, even here.** One machine can
hold both, so a loopback demo is the one place a session can be written with one
address at each end, and BGP does not work that way: `next-hop self` then puts the
peer's OWN address on the wire, and Ze refuses to advertise that, because RFC 4271
Section 5.1.3 forbids telling a peer to reach a destination through itself. The
session still establishes, so the symptom is routes that never arrive rather than
an error. On Linux the whole 127.0.0.0/8 range is available; on macOS add the
alias once with `sudo ifconfig lo0 alias 127.0.0.2`, which `make ze-dev-setup` also
does. The refusal is `originatedNextHopIsPeerOwn`
(`internal/component/bgp/reactor/forward_next_hop.go`), and `precomputeNextHop`
(`internal/component/bgp/reactor/peer_forward_facts.go`) is what resolves
`next-hop self` to the local address.

The sink accepts whatever it is sent, so this demo works either way until you
announce a route; the address split is what keeps it working when you do.

## Stop

```bash
bin/ze signal stop             # graceful shutdown
bin/ze signal restart          # graceful restart (preserves routes via GR)
```
<!-- source: internal/plugins/signal/main.go -- Run -->

## Next Steps

- [Configuration](../configuration-model/index.md) -- peer groups, capabilities, static routes
- [Plugins](../plugins/index.md) -- RIB, route server, RPKI, graceful restart
- [CLI Reference](../cli/index.md) -- interactive CLI, route injection, monitoring
- [Logging](../logging/index.md) -- log levels, backends, per-subsystem tuning
- [Operations](../operations/index.md) -- SSH setup, signals, health checks, troubleshooting
