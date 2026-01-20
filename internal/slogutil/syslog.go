package slogutil

import (
	"log/slog"
	"log/syslog"
	"os"

	"codeberg.org/thomas-mangin/zebgp/internal/env"
)

// newSyslogHandler creates a syslog handler.
// Reads zebgp.log.destination for syslog address.
// Falls back to stderr if syslog connection fails.
func newSyslogHandler(opts *slog.HandlerOptions) slog.Handler {
	addr := env.Get("log", "destination")

	// Determine network and address based on addr format
	var network, raddr string
	switch {
	case addr == "":
		// Use local syslog (empty network/raddr = default)
		network = ""
		raddr = ""
	case addr[0] == '/':
		// Unix socket path
		network = "unix"
		raddr = addr
	default:
		// UDP address (host:port)
		network = "udp"
		raddr = addr
	}

	w, err := syslog.Dial(network, raddr, syslog.LOG_INFO|syslog.LOG_DAEMON, "zebgp")
	if err != nil {
		// Fall back to stderr on error
		return slog.NewTextHandler(os.Stderr, opts)
	}

	// Use syslog writer with text handler
	return slog.NewTextHandler(w, opts)
}
