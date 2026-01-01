# Environment Variables

**Source:** ExaBGP `environment/config.py`
**Purpose:** Document all environment variables for compatibility

---

## ZeBGP Enhancement

ZeBGP uses `zebgp.` prefix instead of `exabgp.` and adds:

1. **Config block support:** Set environment in config file via `environment { }` block
2. **Strict validation:** Invalid values cause startup failure (not silent defaults)
3. **Migration helper:** `zebgp config check --env` validates before upgrade

See [ENVIRONMENT_BLOCK.md](ENVIRONMENT_BLOCK.md) for the config block syntax.

**ZeBGP priority:** `zebgp.x.y` > `zebgp_x_y` > config block > defaults

---

## Overview

ExaBGP configuration uses environment variables with the format:

```
exabgp.<section>.<option>=<value>
```

Or with underscores (for shell compatibility):

```
exabgp_<section>_<option>=<value>
```

Priority: `exabgp.x.y` > `exabgp_x_y` > INI file > default

---

## Configuration Sections

### profile

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.profile.enable | bool | false | Toggle profiling of the code |
| exabgp.profile.file | string | "" | Profiling result file (empty = stdout) |

### pdb

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.pdb.enable | bool | false | Start pdb on program fault |

### daemon

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.daemon.pid | string | "" | PID file location |
| exabgp.daemon.user | string | "nobody" | User to run as |
| exabgp.daemon.daemonize | bool | false | Run in background |
| exabgp.daemon.drop | bool | true | Drop privileges before forking |
| exabgp.daemon.umask | octal | 0137 | Umask for files |

### log

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.log.enable | bool | true | Enable logging |
| exabgp.log.level | string | "INFO" | Syslog level |
| exabgp.log.destination | string | "stdout" | Log destination |
| exabgp.log.all | bool | false | Debug everything |
| exabgp.log.configuration | bool | true | Log config parsing |
| exabgp.log.reactor | bool | true | Log signals, reloads |
| exabgp.log.daemon | bool | true | Log pid, forking |
| exabgp.log.processes | bool | true | Log process handling |
| exabgp.log.network | bool | true | Log TCP/IP, network state |
| exabgp.log.statistics | bool | true | Log packet statistics |
| exabgp.log.packets | bool | false | Log BGP packets |
| exabgp.log.rib | bool | false | Log local route changes |
| exabgp.log.message | bool | false | Log route announcements |
| exabgp.log.timers | bool | false | Log keepalive timers |
| exabgp.log.routes | bool | false | Log received routes |
| exabgp.log.parser | bool | false | Log message parsing |
| exabgp.log.short | bool | true | Short log format |

**Log destination values:**
- `syslog` - Local syslog
- `host:<location>` - Remote syslog
- `stdout` - Standard output
- `stderr` - Standard error
- `<filename>` - File path

### tcp

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.tcp.once | bool | false | One connection attempt (deprecated) |
| exabgp.tcp.attempts | int | 0 | Max connection attempts (0 = unlimited) |
| exabgp.tcp.delay | int | 0 | Delay announcements by N minutes |
| exabgp.tcp.bind | list | [] | IPs to bind when listening |
| exabgp.tcp.port | int | 179 | Port to bind |
| exabgp.tcp.acl | bool | false | Experimental ACL |

### bgp

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.bgp.passive | bool | false | Make all peers passive |
| exabgp.bgp.openwait | int | 60 | Seconds to wait for OPEN |

### cache

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.cache.attributes | bool | true | Cache attributes |
| exabgp.cache.nexthops | bool | true | Cache nexthops (deprecated) |

### api

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.api.version | int | 6 | API version (4 or 6) |
| exabgp.api.ack | bool | true | Acknowledge API commands |
| exabgp.api.chunk | int | 1 | Max lines before yield |
| exabgp.api.encoder | string | "json" | Encoder for v4 (json/text) |
| exabgp.api.compact | bool | false | Compact JSON for INET |
| exabgp.api.respawn | bool | true | Respawn dead processes |
| exabgp.api.terminate | bool | false | Terminate if process dies |
| exabgp.api.cli | bool | true | Create CLI named pipe |
| exabgp.api.pipename | string | "exabgp" | Name for CLI pipe |
| exabgp.api.socketname | string | "exabgp" | Name for Unix socket |

### reactor

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.reactor.speed | float | 1.0 | Reactor loop time |

### debug

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| exabgp.debug.pdb | bool | false | Enable pdb on errors |
| exabgp.debug.memory | bool | false | Memory debug (--memory) |
| exabgp.debug.configuration | bool | false | Raise on config errors |
| exabgp.debug.selfcheck | bool | false | Self-check config |
| exabgp.debug.route | string | "" | Decode route from config |
| exabgp.debug.defensive | bool | false | Generate random faults |
| exabgp.debug.rotate | bool | false | Rotate config on reload |
| exabgp.debug.timing | bool | false | Reactor timing analysis |

---

## INI File Format

Location: `~/.exabgp/exabgp.env` or `/etc/exabgp/exabgp.env`

```ini
[exabgp.log]
level = DEBUG
destination = /var/log/exabgp.log

[exabgp.api]
version = 6
encoder = json

[exabgp.tcp]
bind = 0.0.0.0
port = 179
```

---

## Type Parsing

### Boolean

```python
def boolean(value: str) -> bool:
    return value.lower() in ('1', 'true', 'yes', 'on', 'enable')
```

### Integer

```python
def integer(value: str) -> int:
    return int(value)
```

### Umask

```python
def umask_read(value: str) -> int:
    return int(value, 8)

def umask_write(value: int) -> str:
    return f'0{value:o}'
```

### IP List

```python
def ip_list(value: str) -> list[IP]:
    return [IP.create(v) for v in value.split()]
```

### Syslog Level

```python
SYSLOG_LEVELS = {
    'DEBUG': 10, 'INFO': 20, 'NOTICE': 25,
    'WARNING': 30, 'ERR': 40, 'CRITICAL': 50
}

def syslog_value(value: str) -> str:
    return value.upper() if value.upper() in SYSLOG_LEVELS else 'INFO'
```

---

## Backward Compatibility

### tcp.once → tcp.attempts

```python
if env.tcp.once and not env.tcp.attempts:
    env.tcp.attempts = 1
```

### tcp.connections → tcp.attempts

```python
connections = os.environ.get('exabgp.tcp.connections')
if connections:
    env.tcp.attempts = int(connections)
```

---

## ZeBGP Implementation Notes

### Go Configuration

```go
type Config struct {
    Profile  ProfileConfig
    Daemon   DaemonConfig
    Log      LogConfig
    TCP      TCPConfig
    BGP      BGPConfig
    Cache    CacheConfig
    API      APIConfig
    Reactor  ReactorConfig
    Debug    DebugConfig
}

type LogConfig struct {
    Enable        bool   `env:"exabgp.log.enable" default:"true"`
    Level         string `env:"exabgp.log.level" default:"INFO"`
    Destination   string `env:"exabgp.log.destination" default:"stdout"`
    // ...
}
```

### Environment Loading

```go
func LoadConfig() (*Config, error) {
    cfg := &Config{}

    // For each field:
    // 1. Check env var with dots: exabgp.log.level
    // 2. Check env var with underscores: exabgp_log_level
    // 3. Check INI file
    // 4. Use default

    return cfg, nil
}
```

### Priority

```go
func getValue(section, option string, def string) string {
    // Dot notation first
    dotKey := fmt.Sprintf("exabgp.%s.%s", section, option)
    if v := os.Getenv(dotKey); v != "" {
        return v
    }

    // Underscore notation
    underKey := strings.ReplaceAll(dotKey, ".", "_")
    if v := os.Getenv(underKey); v != "" {
        return v
    }

    // INI file
    if v := ini.Get(dotKey); v != "" {
        return v
    }

    return def
}
```

---

**Last Updated:** 2025-12-19
