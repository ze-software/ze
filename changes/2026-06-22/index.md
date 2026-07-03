# Week of 2026-06-22

Focused on trimming attack surface and rounding out OSPF.

## 🧩 Feature gates

SSH and Looking Glass can now be compiled out at build time, continuing the work toward smaller images with less attack surface for deployments that don't need them.

## 🛰️ Routing

- A unified OSPFv2/OSPFv3 engine, with IPv6 interop coverage
- BGP redistribution now has a dedicated producer exporting RIB best-paths
- Live SSE views for OSPF and IS-IS state in the web UI

## 📊 Observability

eBPF-based per-port, per-IP traffic accounting (TCX), validated against the runtime kernel.

## 🛠️ Under the hood

A Fintek Super-IO serial console fix for Alder Lake-N hardware, and the installer now probes for a reachable install server before trusting the default route.
