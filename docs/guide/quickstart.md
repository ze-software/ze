# Quick Start

Run two Ze peers on one Linux host and verify that each receives a route. The
supplied lab uses loopback addresses and ports above 1024. It needs no external
router or development test peer. Ze is pre-release software; use an isolated lab.
<!-- source: examples/quickstart/r1.conf -- local lab endpoints and originated prefix -->
<!-- source: examples/quickstart/r2.conf -- local lab endpoints and originated prefix -->

## Build

```bash
git clone https://github.com/ze-software/ze.git
cd ze
./ze version
```

Install Go and Git first. The Go version in `go.mod` is the supported toolchain;
the launcher selects that version and builds `bin/ze` with Ze's patched standard
library when the binary is absent. A first build takes longer than starting an
existing binary. Windows is not a supported runtime platform.
<!-- source: ze -- first-build bootstrap, toolchain pin and patched GOROOT -->
<!-- source: go.mod -- Go version -->

### Optional: install the built command on PATH

From the same checkout, select the patched toolchain before using `go install`.
This installs the daemon on your Go binary path; it does not create a system
service. The default feature tags below are derived from `feature-gates.txt`.
<!-- source: internal/le/repo/featuretags/daemontags.go -- DaemonTags -->
<!-- source: internal/le/go/stdpatch/gen/stdpatch.go -- patched toolchain generation -->

```bash
GOROOT="$(go run ./internal/le/go/stdpatch/goroot)"
export GOROOT GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH"
CGO_ENABLED=0 go install -tags 'ze_core ze_distro ze_anomaly ze_as112 ze_bfd ze_bgp ze_bmp ze_copp ze_cos ze_ddos ze_dhcpserver ze_exabgp ze_flowexport ze_geodns ze_gnmi ze_grpc ze_ike ze_isis ze_l2tp ze_ldp ze_lg ze_mcp ze_mpls ze_ntp ze_ospf ze_policyroute ze_pxe ze_radius ze_rest ze_rsvpte ze_ssh ze_tacacs ze_telemetry ze_trafficusage ze_vpp ze_vrrp ze_web' ./cmd/ze
```

The remaining commands use `./ze` from the checkout. For a Linux service, follow
[Linux installation](ubuntu-build-install.md). For a container, use
[Docker](docker.md). For an immutable VM or hardware image, use
[VM Appliance](appliance.md).
<!-- source: ze -- checkout launcher and executable path -->

## Local two-peer lab

The files `examples/quickstart/r1.conf` and `r2.conf` declare both ends of the
session, explicit import/export policy, and a BGP RIB. Each announces one prefix:
`192.0.2.0/24` from r1 and `198.51.100.0/24` from r2. SSH listens on
`127.0.0.1:2221` and `127.0.0.1:2222`. Both peers use AS 65001 for iBGP.
The BGP transport uses `127.0.0.1:1179` and `127.0.0.2:2179`; advertised
next hops are the documentation addresses `192.0.2.1` and `192.0.2.2`.
These next hops support route inspection in this control-plane lab; they do
not establish forwarding connectivity.
<!-- source: examples/quickstart/r1.conf -- r1 endpoints, policy and update -->
<!-- source: examples/quickstart/r2.conf -- r2 endpoints, policy and update -->

These files contain the known lab username and password `lab` / `lab`. They bind
management to loopback and disable resolver file output. Do not use these
credentials on a production system.
Explicit-file startup creates the private store beside each file, so each peer
must have its own directory. No `ze init` step is needed for this configured-user
lab; the later initialization section covers a real installation.
<!-- source: examples/quickstart/r1.conf -- lab authentication, management binding and disabled resolver output -->
<!-- source: cmd/ze/ze_core_start.go -- explicit configuration path -->

Run the following from the repository root in one shell:

```bash
mkdir -p tmp
ZE_LAB_DIR="$(mktemp -d "$PWD/tmp/quickstart.XXXXXX")"
mkdir "$ZE_LAB_DIR/r1" "$ZE_LAB_DIR/r2"
cp examples/quickstart/r1.conf "$ZE_LAB_DIR/r1/ze.conf"
cp examples/quickstart/r2.conf "$ZE_LAB_DIR/r2/ze.conf"
./ze config validate "$ZE_LAB_DIR/r1/ze.conf"
./ze config validate "$ZE_LAB_DIR/r2/ze.conf"

./ze start "$ZE_LAB_DIR/r1/ze.conf" >"$ZE_LAB_DIR/r1.log" 2>&1 &
R1_PID=$!
./ze start "$ZE_LAB_DIR/r2/ze.conf" >"$ZE_LAB_DIR/r2.log" 2>&1 &
R2_PID=$!
```
<!-- source: examples/quickstart/r1.conf -- first peer configuration -->
<!-- source: examples/quickstart/r2.conf -- second peer configuration -->
<!-- source: internal/component/config/cli/cmd_validate.go -- cmdValidate -->

On Linux, both loopback addresses are available. On macOS, add the second
address once before starting the peers: `sudo ifconfig lo0 alias 127.0.0.2`.
<!-- source: internal/le/setup/system.go -- loopbackAddArgv -->

### Verify a session and received routes

Run these commands in the same shell. If a listener is still starting, inspect
the log files and retry the command after it is ready.

```bash
ZE_SSH_PASSWORD=lab ./ze cli --remote 127.0.0.1:2221 --user lab -c "show bgp peer list"
ZE_SSH_PASSWORD=lab ./ze cli --remote 127.0.0.1:2221 --user lab -c "show bgp rib | received"
ZE_SSH_PASSWORD=lab ./ze cli --remote 127.0.0.1:2222 --user lab -c "show bgp rib | received"
```
<!-- source: internal/component/cli/client/main.go -- remote SSH command execution -->
<!-- source: internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang -- show bgp rib -->

The peer list must show an established session. The r1 received RIB must contain
`198.51.100.0/24`; r2 must contain `192.0.2.0/24`. A configured peer or an open
management port alone is not this success check. This lab verifies control-plane
route exchange; it does not install forwarding routes on the host.
<!-- source: examples/quickstart/r1.conf -- r1 advertised documentation prefix -->
<!-- source: examples/quickstart/r2.conf -- r2 advertised documentation prefix -->

### Stop and keep or remove the lab

```bash
kill -TERM "$R1_PID" "$R2_PID"
wait "$R1_PID" "$R2_PID"
printf 'Lab files and stores: %s\n' "$ZE_LAB_DIR"
```

SIGTERM uses Ze's normal shutdown path. Keep the directory if you want to inspect
the stores and logs or restart the lab. To discard this lab after both processes
stop, remove the directory printed above. It contains only this run's copies.
<!-- source: cmd/ze/hub/main.go -- daemon shutdown signal handling -->

## Use an existing router as the peer

The remainder shows initialization and a configuration for an external peer.
Replace the addresses and ASNs with your network's values and configure the
other router's BGP session as well. The local lab above supplies both ends when
you do not have another router.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer connection and session settings -->

## Initialize

Ze runs an SSH server on localhost for CLI access (`ze cli`, `ze show`, `ze signal`). This keeps the control plane authenticated even in multi-user environments. Initialize the folder that will hold the example config:

```bash
export ZE_CONFIG_DIR="$PWD"
./ze init
```

This prompts for username, password, SSH host (default `127.0.0.1`), port (default `2222`), and node name (default: hostname). Credentials are stored locally with bcrypt-hashed passwords. For scripting (later fields fall back to their defaults):
<!-- source: internal/plugins/init/main.go -- Run, defaultHost, defaultPort, bcrypt hashing -->

```bash
printf 'admin\nREPLACE_WITH_A_STRONG_PASSWORD\n' | ./ze init
```

Initialization creates a private `database/` tree in the selected folder.
Every directory under the store is 0700 and every frame file 0600, owned by the
caller. Running `ze init` a second time refuses either an existing tree or a
`database.zefs` seed. To reinitialize, use `--force`; this preserves the old tree
as `database.replaced-<stamp>` before publishing the fully seeded replacement.

```bash
./ze signal stop             # stop daemon first
./ze init --force            # prompts for confirmation, then backs up and reinitializes
```
<!-- source: internal/plugins/init/main.go -- forceFlag -->

`--force --yes` confirms replacement in a script. Replacement refuses while any
process owns the store, regardless of the selected SSH target. Ordinary startup
refuses an existing blob without converting it. The diagnostic names
`ze init --from <blob>`, which imports a local blob; the URL form is storage-2.
Appliance first boot already imports its seed. `ze init --seed` writes an
explicit blob artifact for appliance builders.
<!-- source: internal/plugins/init/main.go -- Run -->
<!-- source: internal/component/config/storage/open.go -- detect -->
<!-- source: cmd/ze/ze_core_autoinit.go -- gokrazyAutoInit -->

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

        filter {
            export {
                final accept
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
  `update {}` block, sent when the session establishes. The automatic End-of-RIB
  marker follows Ze's own routes and the separate peer-up registration barrier,
  not a plugin's session-ready route report (owner ruling, 2026-09-18).
  `behavior { manual-eor true }` suppresses that automatic marker, not the routes.
  <!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendInitialRoutes, the marker follows the routes the peer owns -->

The `filter { export { final accept } }` block is required: `test-peer` is an EBGP peer
(AS 65001), and with no export policy Ze sends an EBGP peer no routes at all (RFC 8212
Section 3), originated ones included. See [BGP policy](bgp-policy.md).
<!-- source: internal/component/bgp/reactor/peer_run.go -- egressFinalAccepts -->

> Advanced: a `process` binding attaches a plugin or your own external program to a peer (the
> ExaBGP-style event API), including making a peer RIB-backed so it stores and re-advertises
> received routes. It is not needed for the config above; see [Plugins](plugins.md).

## Validate

```bash
./ze config validate example.conf
```
<!-- source: internal/component/config/cli/cmd_validate.go -- cmdValidate -->

Expected output:

```
configuration valid: example.conf
```

## Start

```bash
./ze start example.conf
```
<!-- source: cmd/ze/ze_core_dispatch.go -- registerLocalCommands, "start" root handler; cmd/ze/ze_core_start.go -- cmdStart, startConfigPath -->

With no file, `ze start` uses the config the store names, and refuses with exit 1 if the store's instance name cannot be read; `ze doctor` checks the store.

The config path goes behind the `start` keyword. A bare `./ze example.conf` is rejected with `unknown command: example.conf` (exit 1); global flags such as `-d` are consumed before the keyword, so they stay ahead of it.

An explicit path selects the store in that file's folder, before
`ZE_CONFIG_DIR` or the default path. If neither a tree nor a seed exists there,
startup creates a private tree. Every explicit-file start reads the supplied file;
daemon commits update that file and stored history. Bare `ze start` uses the
stored active config instead. Keep `ZE_CONFIG_DIR` set in the second terminal so
CLI commands read the credentials from the same folder.

Ze logs to stderr. You should see something like:

```
level=INFO  msg="hub ready" subsystem=hub plugins=1 peers=1 listen=":179"
level=INFO  msg="peer connecting" subsystem=bgp.reactor peer=test-peer address=10.0.0.2
```

Silence means the default log level (`warn`) has nothing to report -- that's normal. To see all activity:

```bash
./ze -d start example.conf  # debug logging
```
<!-- source: cmd/ze/main.go -- "-d" debug flag sets ze.log=debug -->

## Verify

In another terminal:

```bash
# Check daemon is running
./ze status

# List peers
./ze cli -c "show bgp peer list"

# Show peer details
./ze cli -c "show bgp peer test-peer detail"

# Watch live events (streams until Ctrl-C)
./ze cli -c "monitor event"
```
<!-- source: internal/component/cli/client/main.go -- Execute, StreamMonitor -->

## Test Without a Real Peer

Use the local two-peer lab above for route verification. Developers who only need
a protocol sink can start the repository's test peer through the `le` launcher:

```bash
# Terminal 1: start a sink peer (accepts sessions, replies keepalive)
./le test peer --mode sink --port 1179 --asn 65001
```
<!-- source: internal/test/cli/cmd_peer.go -- le test peer command -->

For this optional sink, copy the external-peer configuration to its own directory
and change its peer's `connection` block to dial `127.0.0.1:1179`:

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
alias once with `sudo ifconfig lo0 alias 127.0.0.2`, which `./le setup install` also
does. The refusal is `originatedNextHopIsPeerOwn`
(`internal/component/bgp/reactor/forward_next_hop.go`), and `precomputeNextHop`
(`internal/component/bgp/reactor/peer_forward_facts.go`) is what resolves
`next-hop self` to the local address.

The sink accepts whatever it is sent, so this demo works either way until you
announce a route; the address split is what keeps it working when you do.

## Stop

```bash
./ze signal stop             # graceful shutdown
./ze signal restart          # graceful restart (preserves routes via GR)
```
<!-- source: internal/plugins/signal/main.go -- Run -->

## Next Steps

- [Configuration](configuration.md) -- peer groups, capabilities, static routes
- [Plugins](plugins.md) -- RIB, route server, RPKI, graceful restart
- [CLI Reference](cli.md) -- interactive CLI, route injection, monitoring
- [Logging](logging.md) -- log levels, backends, per-subsystem tuning
- [Operations](operations.md) -- SSH setup, signals, health checks, troubleshooting
