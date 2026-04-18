// Design: docs/architecture/core-design.md -- nftables firewall backend plugin

// Package firewallnft implements the firewall backend using google/nftables.
// It translates ze's abstract expression types to nftables register operations,
// reconciles desired state against the kernel (create/replace/delete ze_* tables),
// and provides read methods (ListTables, GetCounters) for CLI.
//
// All kernel operations are atomic via nftables.Conn.Flush().
// Only ze_* tables are touched; non-ze_* tables are never modified.
package firewallnft
