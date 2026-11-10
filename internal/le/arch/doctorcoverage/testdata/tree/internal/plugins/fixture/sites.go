// Package fixture is the doctor-coverage scan's fixture: one call of every
// shape the scan reads, each tagged by its message so a test finds it.
package fixture

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// localCode names the covered code through a package constant.
const localCode = diagnostic.CodeDoctorFixtureCovered

// literalMessage is a message held in a constant.
const literalMessage = "fixture: constant message"

// Sites holds one call of every shape.
func Sites(ctx context.Context, log *slog.Logger, w http.ResponseWriter, err error, dynamic string, peer string) {
	// Covered: a key-value code naming an exported constant.
	log.Warn("fixture: covered constant", "peer", peer, "code", diagnostic.CodeDoctorFixtureCovered)
	// Covered: a slog attribute naming a package constant.
	log.Error("fixture: covered attr", slog.String("code", localCode))
	// Covered: a literal code.
	log.Warn("fixture: covered literal", "code", "doctor-fixture-covered")
	// Covered: the context form.
	log.WarnContext(ctx, "fixture: covered context", "code", diagnostic.CodeDoctorFixtureCovered)
	// Covered: the level form.
	log.Log(ctx, slog.LevelError, "fixture: covered level", "code", diagnostic.CodeDoctorFixtureCovered)
	// Covered: an operator error's log attributes carrying the caller's code.
	refusal := opererr.New("fixture: refused", "why", "ze show doctor")
	log.Warn(refusal.Problem(), refusal.LogAttrs(slog.String("code", diagnostic.CodeDoctorFixtureCovered))...)

	// Uncovered: no code at all.
	log.Warn("fixture: no code", "peer", peer)
	// Uncovered: the package-level logger, no code.
	slog.Error("fixture: package logger")
	// Uncovered: a message held in a constant, no code.
	log.Warn(literalMessage)
	// Uncovered: a code built at run time.
	log.Warn("fixture: dynamic code", "code", dynamic)
	// Uncovered: a code no registered doctor check declares.
	log.Error("fixture: orphan code", "code", diagnostic.CodeDoctorFixtureOrphan)

	// Report bus: one covered raise, one whose code no check declares.
	report.RaiseError("fixture", diagnostic.CodeDoctorFixtureCovered, peer, "fixture: raise covered")
	report.RaiseWarning("fixture", "fixture-unregistered", peer, "fixture: raise uncovered")

	// A health row: it carries a name and no code.
	health.Register("fixture-row", nil)

	// Not problem sites: lower levels, other packages' Error, an error's text.
	log.Info("fixture: info", "peer", peer)
	log.Debug("fixture: debug", "peer", peer)
	http.Error(w, "fixture: http error", http.StatusInternalServerError)
	_ = err.Error()
	log.Log(ctx, slog.LevelInfo, "fixture: info level")
}
