# Environment Configuration Block

## TL;DR

Ze-specific feature to set environment variables from the config file:

```
environment {
    log { level DEBUG; }
    tcp { port 1179; }
    api { encoder text; }
}
```

Priority: **OS env > config block > defaults**
<!-- source: internal/component/config/environment.go -- LoadEnvironmentWithConfig priority order -->

## Syntax

```
environment {
    <section> {
        <option> <value>;
    }
}
```

### Available Sections and Options

<!-- source: internal/component/config/environment.go -- loadDefaults(), envOptions -->

| Section | Option | Type | Range/Values | Default |
|---------|--------|------|--------------|---------|
| **daemon** | pid | string | - | "" |
| | user | string | - | "zeuser" |
| | daemonize | bool | - | false |
| | drop | bool | - | true |
| | umask | octal | - | 0o137 |
| **log** | level | enum | DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL | INFO |
| | enable | bool | - | true |
| | destination | string | - | "stdout" |
| | all | bool | - | false |
| | configuration | bool | - | true |
| | reactor | bool | - | true |
| | daemon | bool | - | true |
| | processes | bool | - | true |
| | network | bool | - | true |
| | statistics | bool | - | true |
| | packets | bool | - | false |
| | rib | bool | - | false |
| | message | bool | - | false |
| | timers | bool | - | false |
| | routes | bool | - | false |
| | parser | bool | - | false |
| | short | bool | - | true |
| **tcp** | port | int | 179 or 1025-65535 | 179 |
| | attempts | int | 0-1000 | 0 |
| | delay | int | - | 0 |
| | acl | bool | - | false |
| | once | bool | - | false (legacy alias) |
| | connections | int | 0-1000 | (legacy alias for attempts) |
| **bgp** | connection | enum | both, passive, active | "" |
| | openwait | int | 1-3600 | 60 |
| **cache** | attributes | bool | - | true |
| **api** | ack | bool | - | true |
| | chunk | int | - | 1 |
| | encoder | enum | json, text | "json" |
| | compact | bool | - | false |
| | respawn | bool | - | true |
| | terminate | bool | - | false |
| | cli | bool | - | true |
| **reactor** | speed | float | 0.1-10.0 | 1.0 |
| | cache-ttl | int | 0-3600 | 60 |
| | cache-max | int | 0+ | 1000000 |
| **debug** | pdb | bool | - | false |
| | memory | bool | - | false |
| | configuration | bool | - | false |
| | selfcheck | bool | - | false |
| | route | string | - | "" |
| | defensive | bool | - | false |
| | rotate | bool | - | false |
| | timing | bool | - | false |
| | pprof | string | - | "" |
| **chaos** | seed | int64 | - | 0 |
| | rate | float | 0.0-1.0 | 0.1 |
<!-- source: internal/component/config/environment.go -- loadDefaults, envOptions -->

### Value Types

| Type | Valid Values |
|------|-------------|
| bool | `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off`, `enable`, `disable` |
| enum | Case-insensitive (e.g., `DEBUG`, `debug`, `Debug` all valid) |
| int | Decimal integer |
| float | Decimal number (e.g., `1.5`, `0.1`) |
| octal | Octal number (with or without leading 0) |
| string | Quoted for spaces (e.g., `"/path/with spaces/file"`) |
<!-- source: internal/component/config/environment.go -- parseBoolStrict, parseIntStrict, parseFloatStrict, parseOctalStrict -->

## Priority Order

1. **OS environment variable** (dot notation): `ze.bgp.log.level=DEBUG`
2. **OS environment variable** (underscore notation): `ze_bgp_log_level=DEBUG`
3. **Config file** environment block
4. **Defaults**
<!-- source: internal/component/config/environment.go -- LoadEnvironmentWithConfig -->

## Strict Validation

Ze uses **strict validation** - invalid values cause startup failure:

```bash
# These will cause Ze to refuse to start:
ze.bgp.tcp.port=abc          # Invalid: not a number
ze.bgp.tcp.port=99999        # Invalid: out of range (1-65535)
ze.bgp.log.level=BOGUS       # Invalid: unknown level
ze.bgp.bgp.connection=maybe   # Invalid: must be both, passive, or active
```
<!-- source: internal/component/config/environment.go -- validatePort, validateLogLevel, validateEncoder -->

### Migration Helper

Before upgrading, validate your environment variables:

```bash
ze config validate --limit environment
```

This will report any invalid environment variables that would cause startup failure.

## Examples

### Basic Configuration

```
environment {
    log {
        level DEBUG;
        short true;
    }
    tcp {
        port 1179;
    }
}

bgp {
    router-id 192.0.2.1;
    local { as 65000; }

    peer my-peer {
        remote { ip 192.0.2.2; as 65001; }
    }
}
```

### Full Configuration

```
environment {
    daemon {
        user ze;
        daemonize true;
    }
    log {
        level INFO;
        destination /var/log/ze.bgp.log;
    }
    tcp {
        port 179;
        attempts 3;
    }
    bgp {
        connection passive;
        openwait 120;
    }
    api {
        encoder json;
        cli true;
    }
    reactor {
        speed 1.0;
    }
}
```

### OS Environment Override

```bash
# Config file sets port 1179, but OS env overrides to 179
export ze.bgp.tcp.port=179
ze bgp run config.conf
```

## Differences from ExaBGP

| Feature | ExaBGP | Ze |
|---------|--------|-------|
| Environment source | Separate INI file (`exabgp.env`) | Config block + OS env |
| Invalid values | Silent fallback to default | Strict error, refuse to start |
| Validation | None | Enums and ranges validated |
| Migration helper | None | `ze config validate --limit environment` |
<!-- source: internal/component/config/environment.go -- SetConfigValue error handling -->

## Multiple Environment Blocks

Multiple environment blocks are **merged** (not overwritten):

```
environment {
    log { level DEBUG; }
}
environment {
    tcp { port 1179; }
}
# Result: log.level=DEBUG AND tcp.port=1179
```

However, for clarity, use a single environment block.

## Related

- [ENVIRONMENT.md](ENVIRONMENT.md) - OS environment variable reference
- [SYNTAX.md](SYNTAX.md) - Config file syntax overview
