// Design: docs/architecture/core-design.md -- VPP traffic control backend
// Related: trafficvpp.go -- package-level loggerPtr initialized here's counterpart

//go:build linux

package trafficvpp

import "log/slog"

// logger returns the package-level slog.Logger, never nil.
//
// Lives in a linux-tagged file because backend_linux.go is the only caller.
// On darwin/other GOOS, backend_linux.go is excluded so a non-tagged logger()
// would be flagged as unused. Keeping this helper next to its caller also
// makes the coupling obvious.
func logger() *slog.Logger { return loggerPtr.Load() }
