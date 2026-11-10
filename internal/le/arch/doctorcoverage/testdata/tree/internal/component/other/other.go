// Package other is a fixture: a second owner package, so the report groups.
package other

import (
	"log/slog"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Watch logs one covered and one uncovered warning.
func Watch(log *slog.Logger) {
	log.Warn("other: covered", "code", diagnostic.CodeDoctorFixtureCovered)
	log.Error("other: uncovered")
}
