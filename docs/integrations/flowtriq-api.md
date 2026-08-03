# Flowtriq Cloud API Reference (Reverse-Engineered)

<!-- source: ~/Code/github.com/Flowtriq/ftagent/ftagent/agent.py -->

This document describes the Flowtriq cloud API as consumed by the `ftagent`
agent (v1.x). Reverse-engineered from `ftagent/agent.py`.

## Base URL and Authentication

| Setting | Value |
|---------|-------|
| Base URL | `https://flowtriq.com/api/v1` (configurable via `api_base`) |
| Auth | Bearer token via `Authorization: Bearer {api_key}` |
| Node ID | `X-Node-UUID: {node_uuid}` header on every request |
| Content-Type | `application/json` (except PCAP upload) |
| User-Agent | `ftagent/{VERSION}` |

Both `api_key` and `node_uuid` are configured during `ftagent --setup` and
stored in `/etc/ftagent.conf`.

## Circuit Breaker

| Parameter | Value | Source |
|-----------|-------|--------|
| Failure threshold | 5 consecutive failures | `agent.py` |
| Recovery timeout | 60 seconds | `agent.py` |
| Half-open probes | 1 | `agent.py` |
| Retry queue | bounded deque, maxlen=2000 | `agent.py` |
| Retry backoff | exponential, capped at 10s (16s for 503) | `agent.py,448` |
| 503 handling | honors `Retry-After` header | `agent.py` |

When tripped, requests are queued. The queue is flushed after a successful heartbeat (`agent.py`).

## Endpoints

### POST `/agent/heartbeat`

Every 30 seconds (`agent.py`). Flushes retry queue on success.

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `version` | string | Agent version (e.g. "1.2.3") | `agent.py` |
| `baseline_ready` | bool | Rolling baseline has enough samples | `agent.py` |
| `baseline_avg_pps` | float | Current baseline average PPS | `agent.py` |
| `baseline_p99_pps` | float | Current baseline p99 PPS | `agent.py` |
| `baseline_hourly_ready` | bool | Hourly baseline bucket populated | `agent.py` |
| `baseline_current_hour_p99` | float | Current hour's p99 PPS | `agent.py` |
| `circuit_breaker` | string | "closed", "open", or "half-open" | `agent.py` |
| `retry_queue_size` | int | Pending retries in queue | `agent.py` |
| `gre_dedup_enabled` | bool | GRE deduplication active | `agent.py` |
| `hypervisor_mode` | bool | Per-VM tracking active | `agent.py` |
| `pcap_active` | bool | PCAP capture enabled | `agent.py` |
| `flow_active` | bool | Flow collector running | `agent.py` |
| `src_ip_overflow` | int | Source IP tracking overflows | `agent.py` |
| `src_ip_count` | int | Tracked source IPs | `agent.py` |
| `pkt_samples_count` | int | Packet samples in analyser | `agent.py` |
| `vm_count` | int | Tracked VMs (hypervisor mode) | `agent.py` |
| `flow_collector` | object | Flow collector stats | `agent.py` |

### POST `/agent/metrics`

Aggregated traffic metrics, sent periodically (~30s window). `agent.py`.

| Field | Type | Description |
|-------|------|-------------|
| `pps` | float | Peak PPS in window |
| `bps` | float | Peak BPS in window |
| `tcp_pct` | float | TCP percentage of latest sample |
| `udp_pct` | float | UDP percentage |
| `icmp_pct` | float | ICMP percentage |
| `conn_count` | int | Connection count |
| `threshold` | float | Current detection threshold |
| `avg_pps` | float | Average PPS across window |
| `samples` | int | Number of samples in window |

### POST `/agent/incidents` (Open Incident)

Called at attack onset (`agent.py`). Returns the incident UUID.

**Request:**

| Field | Type | Description |
|-------|------|-------------|
| `peak_pps` | float | Peak PPS at detection |
| `peak_bps` | float | Peak BPS at detection |
| `started_at` | string | ISO 8601 UTC timestamp |
| `attack_family` | string | Classification (see families below) |
| `attack_subtype` | string? | Sub-classification (e.g. "dns_amplification") |
| `attack_tool` | string? | Detected tool if IOC matched |
| `baseline_pps` | float | Pre-attack baseline average |
| `duration` | int | Always 0 at open |
| `gre_dedup_active` | bool | GRE dedup state |
| `inner_ip` | string? | Target VM IP (hypervisor mode) |
| `inner_ip_label` | string? | VM label from config |
| `vm_breakdown` | array? | Per-VM stats (hypervisor mode) |
| `top_src_ips` | array? | `[{"ip": "1.2.3.4", "count": 5000}]` |
| `top_dst_ports` | array? | `[{"port": 53, "count": 4000}]` |
| `source_ip_count` | int? | Unique source IPs observed |

**Response:**

```json
{
  "uuid": "incident-uuid-string",
  "pending_commands": [...]
}
```

The `pending_commands` array, if present, contains server-side mitigation rules
to apply immediately (same format as the config poll).

### POST `/agent/incidents/{uuid}` (Update Incident)

Called every ~5 seconds during an active attack (`agent.py`).

| Field | Type | Description |
|-------|------|-------------|
| `peak_pps` | float | Running peak PPS |
| `peak_bps` | float | Running peak BPS |
| `attack_family` | string | Current classification |
| `attack_subtype` | string? | Current sub-classification |
| `attack_tool` | string? | Tool if IOC matched |
| `protocol_breakdown` | object | `{"tcp": 5.0, "udp": 92.0, "icmp": 3.0}` |
| `source_ip_count` | int | Unique source IPs |
| `total_packets` | int | Total packets since onset |
| `top_src_ips` | array | Top 20 source IPs with counts |
| `top_dst_ports` | array | Top 20 destination ports with counts |
| `ioc_hits` | array | IOC pattern matches (strings) |
| `spoofing_detected` | bool | Source IP spoofing indicators |
| `botnet_detected` | bool | Botnet indicators |
| `fragment_count` | int | IP fragment count |
| `fragment_pct` | float | Fragment percentage |
| `inner_ip` | string? | Top attacked VM (hypervisor) |
| `inner_ip_label` | string? | VM label |
| `vm_breakdown` | array? | Per-VM stats |

### POST `/agent/incidents/{uuid}/resolve` (Resolve Incident)

Called when the attack ends (`agent.py`). Full forensic record.

| Field | Type | Description |
|-------|------|-------------|
| `duration_seconds` | float | Attack duration in seconds |
| `peak_pps` | float | Overall peak PPS |
| `peak_bps` | float | Overall peak BPS |
| `attack_family` | string | Final classification |
| `attack_subtype` | string? | Final sub-classification |
| `attack_tool` | string? | Tool attribution |
| `protocol_breakdown` | object | `{"tcp": %, "udp": %, "icmp": %}` |
| `ioc_hits` | array | IOC matches |
| `spoofing_detected` | bool | Spoofing indicators |
| `botnet_detected` | bool | Botnet indicators |
| `total_packets` | int | Total packets during attack |
| `source_ip_count` | int | Total unique source IPs |
| `src_ip_entropy` | float | Shannon entropy of source IPs |
| `tcp_flag_breakdown` | object | `{"SYN": 100, "ACK": 50, ...}` |
| `dns_query_stats` | object | DNS query analysis if DNS flood |
| `pkt_length_histogram` | object | `{"64": 5000, "128": 3000, ...}` |
| `ttl_distribution` | object | `{"64": 8000, "128": 2000, ...}` |
| `velocity_curve` | array | `[{"t": 0, "pps": 50000}, {"t": 5, "pps": 75000}]` |
| `top_src_ips` | array | Top source IPs with counts |
| `top_dst_ports` | array | Top destination ports with counts |
| `avg_pkt_length` | float | Average packet length |
| `fragment_count` | int | Total IP fragments |
| `fragment_pct` | float | Fragment percentage |

### POST `/agent/incidents/{uuid}/pcap` (Upload PCAP)

Forensic packet capture upload (`agent.py`).

**Small files (<=2MB):** multipart form upload, field name `pcap`.

**Large files (>2MB):** chunked upload at 2MB boundaries:

| Header | Value |
|--------|-------|
| `Content-Type` | `application/octet-stream` |
| `X-Chunk-Index` | 0-based chunk index |
| `X-Chunk-Total` | total chunks |
| `X-Upload-Id` | unique 16-char hex ID per upload |

Maximum file size: 500MB (truncated if larger, `agent.py`).

### GET `/agent/config` (Fetch Config)

Polled every 300s idle, 10s during attack (`agent.py`).

**Response fields:**

| Field | Type | Description |
|-------|------|-------------|
| `suspended` | bool | Workspace suspended (billing) |
| `force_update` | string? | Version to force-upgrade to |
| `pps_threshold` | float? | Server-set static threshold |
| `dynamic_threshold` | bool | Use dynamic baseline |
| `ioc_patterns` | array? | IOC regex patterns for threat intel |
| `pcap_enabled` | bool? | Enable/disable PCAP |
| `ip_blocklist` | array? | `[{"indicator": "1.2.3.4"}]` |
| `gre_mode` | string? | "auto", "enabled", "disabled" |
| `gre_max_depth` | int? | Max GRE nesting depth |
| `hypervisor_mode` | bool? | Enable per-VM tracking |
| `vm_labels` | object? | `{"10.0.0.5": "Customer A"}` |
| `pending_commands` | array? | Mitigation commands (see below) |
| `flow` | object? | Flow collector config |

### POST `/agent/commands/ack` (Acknowledge Command)

Sent after executing a server-pushed command (`agent.py`).

| Field | Type | Description |
|-------|------|-------------|
| `command_id` | int | Command ID from the pending command |
| `status` | string | "applied" or "failed" |
| `error` | string? | Error message if failed |

### POST `/agent/sp/metrics` (Service Port Metrics)

Split service/non-service traffic metrics (`agent.py`).

| Field | Type | Description |
|-------|------|-------------|
| `service_pps` | float | Service traffic PPS |
| `service_bps` | float | Service traffic BPS |
| `non_service_pps` | float | Non-service traffic PPS |
| `non_service_bps` | float | Non-service traffic BPS |
| `blocked_pps` | float | Blocked traffic PPS |
| `active_blocks` | int | Active iptables block rules |

### POST `/agent/sp/blocks` (Service Port Block Report)

Reports per-source-IP block actions (`agent.py`). Payload varies.

### POST `/agent/vm-stats` (VM Stats)

Per-VM inner IP statistics (`agent.py`).

| Field | Type | Description |
|-------|------|-------------|
| `vms` | array | Per-VM breakdown objects |
| `gre_dedup_active` | bool | GRE dedup state |

### POST `/agent/mirror-metrics` (Mirror Mode Metrics)

SPAN/mirror port per-IP stats (`agent.py`).

| Field | Type | Description |
|-------|------|-------------|
| `ips` | array | Per-IP stat objects |
| `total_pps` | float | Aggregate PPS |
| `total_bps` | float | Aggregate BPS |
| `tracked_ip_count` | int | Number of tracked IPs |
| `active_attacks` | int | Active attacks |
| `mirror_engine` | object | Engine stats |

### POST `/agent/gre-tunnels` (GRE Tunnel Report)

Reports discovered GRE tunnel endpoints (`agent.py`).

### POST `/agent/l7/detect` (L7 Web Server Detection)

Reports web server auto-detection results (`agent.py`).

| Field | Type | Description |
|-------|------|-------------|
| `web_server` | string | Detected server (e.g. "nginx") |
| `server_version` | string? | Version if detected |
| `detected_paths` | array | Log file paths found |
| `active_log_path` | string? | Currently monitored path |

### POST `/agent/l7/metrics` (L7 Metrics)

HTTP request metrics (`agent.py`). Payload varies by implementation.

## Pending Commands Format

Commands arrive in `pending_commands` arrays in incident responses and config
polls. Deduplicated by `id` (the agent tracks executed IDs, `agent.py`).

### Standard Commands (iptables/sysctl)

```json
{
  "id": 123,
  "command_type": "iptables",
  "command_text": "iptables -A INPUT -s 1.2.3.4 -j DROP",
  "title": "Block source 1.2.3.4"
}
```

`command_text` may contain multiple newline-separated commands.

Allowed prefixes (`agent.py`):
`iptables`, `ip6tables`, `ipset`, `sysctl`, `nft`, `ufw`, `firewall-cmd`,
`tc`, `ip route`, `fail2ban-client`, `nginx`, `apache2ctl`.

### XDP Commands

```json
{
  "id": 456,
  "command_type": "xdp",
  "command_text": "{\"type\": \"xdp_filter\", \"target\": \"192.0.2.1\", \"proto\": \"udp\", \"dport\": 53, \"action\": \"drop\", \"rate_pps\": 10000}",
  "title": "XDP filter 192.0.2.1"
}
```

XDP spec fields (`agent.py`):

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"xdp_filter"` or `"xdp_filter_remove"` |
| `target` | string | Target IP address |
| `proto` | string | `"udp"`, `"tcp"`, `"icmp"`, `"any"` |
| `dport` | int? | Destination port (optional) |
| `action` | string | `"drop"` or `"pass"` |
| `rate_pps` | int? | Rate limit in PPS (optional) |

## Attack Families

Classification values for `attack_family` (`agent.py`):

| Family | Condition |
|--------|-----------|
| `udp_flood` | UDP > 60% |
| `syn_flood` | TCP dominant + SYN > 50% |
| `tcp_flood` | TCP > 60% (non-SYN) |
| `icmp_flood` | ICMP > 40% |
| `dns_amplification` | UDP + DNS evidence |
| `ntp_amplification` | UDP + port 123 dominant |
| `fragment_flood` | Fragment % > threshold |
| `http_flood` | L7 detection (RPS-based) |
| `unknown` | No clear pattern |

Subtypes include: `dns_amplification`, `ntp_amplification`, `memcached_amplification`,
`chargen_amplification`, `ssdp_amplification`, `ldap_amplification`,
`syn_flood`, `ack_flood`, `rst_flood`, `fin_flood`, `xmas_flood`,
`null_flood`, `l7_flood`, `l7_slowloris`, `l7_rudy`.
