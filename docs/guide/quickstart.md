# Quick Start

Get Ze running with two BGP peers in under 5 minutes.

## Build

```bash
git clone https://codeberg.org/thomas-mangin/ze.git
cd ze
make build    # produces bin/ze, bin/ze-test, bin/ze-chaos
```

Requires **Go 1.25+**.

## Initialize

Ze runs an SSH server on localhost for CLI access (`ze cli`, `ze show`, `ze signal`). This keeps the control plane authenticated even in multi-user environments. Set up credentials once:

```bash
bin/ze init
```

This prompts for username, password, SSH host (default `127.0.0.1`), and port (default `2222`). Credentials are stored locally with bcrypt-hashed passwords. For scripting:

```bash
echo -e "admin\nsecret" | bin/ze init
```

Running `ze init` a second time will refuse with `error: database already exists`. To reinitialize, use `--force` -- this backs up the old database as `database.zefs.replaced-<date>` before creating a new one:

```bash
bin/ze signal stop             # stop daemon first
bin/ze init --force            # prompts for confirmation, then backs up and reinitializes
```

## Minimal Config

Save as `example.conf`:

```
plugin {
    external rib {
        run "ze plugin bgp-rib"
        encoder json
    }
}

bgp {
    router-id 10.0.0.1
    local {
        as 65000
    }

    peer test-peer {
        remote {
            ip 10.0.0.2
            as 65001
        }
        local {
            ip 10.0.0.1
        }

        family {
            ipv4/unicast
        }

        process rib {
            receive [ state ]
            send [ update ]
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

## Validate

```bash
bin/ze config validate example.conf
```

Expected output:

```
configuration valid (1 peer, 1 plugin)
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

## Verify

In another terminal:

```bash
# Check daemon is running
bin/ze status

# List peers
bin/ze cli --run "peer list"

# Show peer details
bin/ze cli --run "peer test-peer detail"

# Watch live events
bin/ze cli --run "bgp monitor"
```

## Test Without a Real Peer

Use the built-in test peer to accept any BGP session:

```bash
# Terminal 1: start a sink peer (accepts sessions, replies keepalive)
bin/ze-test peer --mode sink --port 1179 --asn 65001

# Terminal 2: start ze with config pointing to localhost:1179
bin/ze example-local.conf
```

Where `example-local.conf` uses `remote { ip 127.0.0.1; }` and `port 1179`.

## Stop

```bash
bin/ze signal stop             # graceful shutdown
bin/ze signal restart          # graceful restart (preserves routes via GR)
```

## Next Steps

- [Configuration](configuration.md) -- peer groups, capabilities, static routes
- [Plugins](plugins.md) -- RIB, route server, RPKI, graceful restart
- [CLI Reference](cli.md) -- interactive CLI, route injection, monitoring
- [Logging](logging.md) -- log levels, backends, per-subsystem tuning
- [Operations](operations.md) -- SSH setup, signals, health checks, troubleshooting
