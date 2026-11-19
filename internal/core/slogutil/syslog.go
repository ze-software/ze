// Design: docs/architecture/config/environment.md — structured logging utilities
// Related: slogutil.go — createHandler selects this handler for backend syslog

package slogutil

import (
	"context"
	"fmt"
	"log/slog"
	"log/syslog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// envKeySyslogFacility is the facility every syslog record goes out on unless
// a ze.log.syslog.<subsystem> key names another one for its subsystem.
const envKeySyslogFacility = "ze.log.syslog.facility"

// envPrefixSyslog is the prefix of the per-subsystem facility keys. The
// config container environment/log/syslog maps onto it: the facility leaf to
// envKeySyslogFacility, and any other name to ze.log.syslog.<subsystem>.
const envPrefixSyslog = "ze.log.syslog."

// syslogFacilityDefault is the facility used while no key names one.
const syslogFacilityDefault = "daemon"

var (
	_ = env.MustRegister(env.EnvEntry{Key: envKeySyslogFacility, Type: envTypeString, Default: syslogFacilityDefault, Description: "Syslog facility for every subsystem no ze.log.syslog.<subsystem> key maps, one of " + strings.Join(FacilityNames(), ", ")})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.log.syslog.<subsystem>", Type: envTypeString, Description: "Syslog facility for one subsystem and its children (e.g. ze.log.syslog.bgp=local7)", Private: true})
)

// syslogFacilities is the one place Go names the syslog facilities an
// operator can choose: each word selects the facility bits of the priority
// the syslog writer sends. The environment/log/syslog/facility leaf of
// ze-hub-conf.yang offers the same words, and internal/component/config holds
// the two together in TestLogSyslogFacilityLeafMatchesSlogutil, because this
// package sits under the model and cannot read it (ai/rules/principles.md).
var syslogFacilities = map[string]syslog.Priority{
	"kern":     syslog.LOG_KERN,
	"user":     syslog.LOG_USER,
	"mail":     syslog.LOG_MAIL,
	"daemon":   syslog.LOG_DAEMON,
	"auth":     syslog.LOG_AUTH,
	"syslog":   syslog.LOG_SYSLOG,
	"lpr":      syslog.LOG_LPR,
	"news":     syslog.LOG_NEWS,
	"uucp":     syslog.LOG_UUCP,
	"cron":     syslog.LOG_CRON,
	"authpriv": syslog.LOG_AUTHPRIV,
	"ftp":      syslog.LOG_FTP,
	"local0":   syslog.LOG_LOCAL0,
	"local1":   syslog.LOG_LOCAL1,
	"local2":   syslog.LOG_LOCAL2,
	"local3":   syslog.LOG_LOCAL3,
	"local4":   syslog.LOG_LOCAL4,
	"local5":   syslog.LOG_LOCAL5,
	"local6":   syslog.LOG_LOCAL6,
	"local7":   syslog.LOG_LOCAL7,
}

// FacilityNames answers every syslog facility name, sorted: the words a
// refusal names, and the words a description of ze.log.syslog.facility lists.
func FacilityNames() []string {
	return slices.Sorted(maps.Keys(syslogFacilities))
}

// parseFacility parses a facility word, case-insensitive. ok is false for a
// word that names no facility.
func parseFacility(s string) (syslog.Priority, bool) {
	facility, ok := syslogFacilities[strings.ToLower(s)]
	return facility, ok
}

// validateFacility refuses a word that names no syslog facility.
func validateFacility(value string) error {
	if _, ok := parseFacility(value); ok {
		return nil
	}
	return fmt.Errorf("invalid syslog facility %q (must be one of %s)", value, strings.Join(FacilityNames(), "/"))
}

// facilityFor answers the facility a subsystem's records go out on. It walks
// ze.log.syslog.<subsystem> from the most specific name to the shortest, the
// way getLogEnv walks levels, so bgp covers bgp.server until bgp.server is
// named too, and answers ze.log.syslog.facility when no name matches. A word
// ApplyLogConfig would refuse can still arrive from the OS environment: the
// warning names it and the walk goes on to the next, shorter key.
func facilityFor(subsystem string) syslog.Priority {
	parts := strings.Split(subsystem, ".")
	var tb textbuf.Buffer
	for i := len(parts); i > 0; i-- {
		key := tb.Reset().Str(envPrefixSyslog).Join(parts[:i], ".").String()
		if facility, ok := facilityFromEnv(key); ok {
			return facility
		}
	}
	if facility, ok := facilityFromEnv(envKeySyslogFacility); ok {
		return facility
	}
	return syslog.LOG_DAEMON
}

// facilityFromEnv reads one facility key. ok is false while the key is unset
// or holds a word that names no facility, which is warned about on stderr.
func facilityFromEnv(key string) (syslog.Priority, bool) {
	value := env.Get(key)
	if value == "" {
		return 0, false
	}
	facility, ok := parseFacility(value)
	if !ok {
		writeWarn(os.Stderr, "warning: %s=%q names no syslog facility (must be one of %s), ignoring it\n",
			key, value, strings.Join(FacilityNames(), "/"))
	}
	return facility, ok
}

// newSyslogHandler creates the syslog handler for one subsystem. It reads
// ze.log.destination for the address and facilityFor for the facility, and
// gives each record the syslog severity of its slog level. It falls back to
// stderr when the syslog connection fails.
func newSyslogHandler(opts *slog.HandlerOptions, subsystem string) slog.Handler {
	addr := getSpecialEnv("destination")

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

	w, err := syslog.Dial(network, raddr, syslog.LOG_INFO|facilityFor(subsystem), "ze")
	if err != nil {
		// Fall back to stderr on error
		return slog.NewTextHandler(os.Stderr, opts)
	}

	return newSeverityHandler(w, opts)
}

// severityHandler sends each record at the syslog severity of its slog level.
// A syslog writer's Write sends at the one priority it was dialed with, so a
// handler that wrote every record through it reported an error as info, and a
// collector that selects by severity could not tell them apart. It holds one
// text handler per severity, each writing through the writer method of that
// severity, and Handle picks the one for the record's level. Safe for
// concurrent use: each text handler serializes its own writes, and the
// syslog writer serializes the connection.
type severityHandler struct {
	debug slog.Handler
	info  slog.Handler
	warn  slog.Handler
	err   slog.Handler
}

// newSeverityHandler builds the four text handlers over w.
func newSeverityHandler(w *syslog.Writer, opts *slog.HandlerOptions) severityHandler {
	return severityHandler{
		debug: slog.NewTextHandler(severityWriter(w.Debug), opts),
		info:  slog.NewTextHandler(severityWriter(w.Info), opts),
		warn:  slog.NewTextHandler(severityWriter(w.Warning), opts),
		err:   slog.NewTextHandler(severityWriter(w.Err), opts),
	}
}

// severityWriter adapts one syslog writer method to io.Writer.
type severityWriter func(m string) error

func (f severityWriter) Write(p []byte) (int, error) {
	if err := f(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// handlerFor answers the text handler of the syslog severity a slog level
// maps to: below info is debug, below warn is info, below error is warning,
// and error or above is err.
func (h severityHandler) handlerFor(level slog.Level) slog.Handler {
	if level < slog.LevelInfo {
		return h.debug
	}
	if level < slog.LevelWarn {
		return h.info
	}
	if level < slog.LevelError {
		return h.warn
	}
	return h.err
}

func (h severityHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handlerFor(level).Enabled(ctx, level)
}

func (h severityHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handlerFor(r.Level).Handle(ctx, r)
}

func (h severityHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return severityHandler{
		debug: h.debug.WithAttrs(attrs),
		info:  h.info.WithAttrs(attrs),
		warn:  h.warn.WithAttrs(attrs),
		err:   h.err.WithAttrs(attrs),
	}
}

func (h severityHandler) WithGroup(name string) slog.Handler {
	return severityHandler{
		debug: h.debug.WithGroup(name),
		info:  h.info.WithGroup(name),
		warn:  h.warn.WithGroup(name),
		err:   h.err.WithGroup(name),
	}
}
