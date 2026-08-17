// Design: docs/architecture/appliance/self-update.md -- self-update configuration validation

package system

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/slogutil"
)

type hhmm struct {
	hour   int
	minute int
}

func parseHHMM(s string) (hhmm, error) {
	if len(s) != 5 || s[2] != ':' {
		return hhmm{}, fmt.Errorf("invalid HH:MM format: %q", s)
	}
	var h, m int
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil {
		return hhmm{}, fmt.Errorf("invalid HH:MM format: %q", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return hhmm{}, fmt.Errorf("HH:MM out of range: %q", s)
	}
	return hhmm{hour: h, minute: m}, nil
}

// validateSelfUpdateConfig checks for config errors.
func validateSelfUpdateConfig(cfg SelfUpdateConfig) error {
	if cfg.RestartImmediate && cfg.RestartTime != "" {
		return errors.New("restart { immediate } and restart { time } are mutually exclusive")
	}
	if cfg.RestartTime != "" {
		if _, err := parseHHMM(cfg.RestartTime); err != nil {
			return fmt.Errorf("restart time: %w", err)
		}
	}
	if cfg.MaintenanceStart != "" {
		if _, err := parseHHMM(cfg.MaintenanceStart); err != nil {
			return fmt.Errorf("maintenance-window start: %w", err)
		}
	}
	if cfg.MaintenanceEnd != "" {
		if _, err := parseHHMM(cfg.MaintenanceEnd); err != nil {
			return fmt.Errorf("maintenance-window end: %w", err)
		}
	}
	return nil
}

// warnConfigConflicts logs warnings for non-error config conflicts.
func warnConfigConflicts(cfg SelfUpdateConfig) {
	if cfg.RestartTime == "" || cfg.MaintenanceStart == "" || cfg.MaintenanceEnd == "" {
		return
	}
	start, startErr := parseHHMM(cfg.MaintenanceStart)
	end, endErr := parseHHMM(cfg.MaintenanceEnd)
	restartT, restartErr := parseHHMM(cfg.RestartTime)
	if startErr != nil || endErr != nil || restartErr != nil {
		return
	}

	restartMin := restartT.hour*60 + restartT.minute
	startMin := start.hour*60 + start.minute
	endMin := end.hour*60 + end.minute

	var inWindow bool
	if startMin <= endMin {
		inWindow = restartMin >= startMin && restartMin < endMin
	} else {
		inWindow = restartMin >= startMin || restartMin < endMin
	}
	if !inWindow {
		slogutil.Logger("self-update").Warn(
			"restart time is outside maintenance-window; binary will be staged during window but restart will happen after it closes",
			"restart", cfg.RestartTime, "window", cfg.MaintenanceStart+"-"+cfg.MaintenanceEnd)
	}
}
