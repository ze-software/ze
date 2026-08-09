# Host hardware architecture

What Ze knows about the machine it runs on, and what it writes back. Code lives
in `internal/component/host/`, with the command surfaces in
`internal/plugins/host/` and `internal/plugins/host-cmd/`.

| Document | Subject |
|----------|---------|
| `inventory.md` | the detection library, its units, and its fixtures |
| `smart.md` | disk health through `smartctl` |
| `observability.md` | cache, Prometheus gauges, hardware-change events |
| `tuning.md` | the write side: governor, IRQ affinity, ethtool rings |

Read `inventory.md` first. The other three build on it.
