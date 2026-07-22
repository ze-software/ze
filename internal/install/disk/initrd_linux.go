// Design: plan/learned/1024-installer-initrd-pure-go.md -- PID-1 entry point for installer initrd

//go:build linux && ze_installer

package disk

import (
	"log/slog"

	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// RunInitrd is the PID-1 entry point for the installer initrd binary.
// It bootstraps proc/sys/dev, sets up console fan-out, then runs the
// install logic. It never returns; all exit paths end in reboot,
// poweroff, or the recovery console.
func RunInitrd() {
	cfg := InstallConfig{Source: sourceHTTP}
	var tb textbuf.Buffer

	defer func() {
		if r := recover(); r != nil {
			slog.Error("FATAL: panic in installer", "panic", r)
			fatalInitrd(cfg, "panic in installer")
		}
	}()

	bootstrap()
	slog.Info("ze installer initrd starting")

	slog.Info("parsing kernel cmdline")
	cfg = parseCmdline()
	slog.Info("cmdline parsed",
		"source", cfg.Source,
		"server", cfg.Server,
		"image", cfg.Image,
		"port", cfg.Port,
		"target", cfg.Target,
		"wait", cfg.Wait,
		"mac", cfg.Mac,
		"rescue-auth-set", cfg.RescueAuth != "",
	)

	slog.Info("validating configuration")
	if err := validateConfig(cfg); err != nil {
		msg := tb.Reset().Str("invalid cmdline: ").Err(err).String()
		fatalInitrd(cfg, msg)
	}

	// R-6 fault-injection evidence hook. No-op in the shipping initrd; the real
	// hook is compiled in only under the ze_installer_fault build tag.
	maybeInjectFault(cfg)

	slog.Info("starting install", "source", cfg.Source)
	var code int
	switch cfg.Source {
	case sourceHTTP:
		code = runHTTP(cfg)
	case sourceISO:
		code = runISO(cfg)
	default:
		msg := tb.Reset().Str("unsupported source: ").Str(cfg.Source).String()
		fatalInitrd(cfg, msg)
	}

	if code != 0 {
		fatalInitrd(cfg, "installation failed")
	}

	// runHTTP/runISO called doReboot/doPoweroff. Safety valve.
	slog.Warn("install completed but process still alive, rebooting")
	unix.Sync()
	_ = unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	select {}
}
