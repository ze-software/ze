// Design: docs/architecture/core-design.md -- nftables firewall backend plugin

// Package firewallnft implements the firewall backend using google/nftables.
// It translates ze's abstract expression types to nftables register operations,
// reconciles desired state against the kernel (create/replace/delete ze_* tables),
// and provides read methods (ListTables, GetCounters) for CLI.
//
// All kernel operations are atomic via nftables.Conn.Flush().
//
// Only ze_* tables are touched, with one bounded exception: the tables an
// earlier ze build wrote WITHOUT the prefix are deleted once, because nothing
// else can reach them after an upgrade. The exception is bounded twice over.
// It is decided on three facts together -- the name, the address family, and
// every chain the table holds -- so a table another tool wrote under one of
// those names is left alone. And it runs on the FIRST reconcile of the process
// and on no later one, so it is a migration with an end rather than a standing
// deletion policy. The list, the decision and the gate live in
// internal/component/firewall/legacy_tables.go.
package firewallnft
