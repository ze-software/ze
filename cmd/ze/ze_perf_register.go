// Design: docs/architecture/system-architecture.md -- ze-perf feature wiring

//go:build ze_perf

package main

import perfcli "github.com/ze-software/ze/internal/perf/cli"

// The ze-perf personality registers the `perf` root. internal/perf/cli leaves
// it to this file, because le links the same package and registers no tool
// root.
func init() { perfcli.RegisterRoot() }
