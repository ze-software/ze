// Fixture for RuleErrorDroppedBelowWarn.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
)

type mobike struct {
	local   *net.UDPAddr
	pending bool
	drops   int
}

type sa struct {
	peer   string
	mobike mobike
	failed error
}

var errNoSource = errors.New("no source")

func routeSource(s *sa) (*net.UDPAddr, error) {
	if s.mobike.local == nil {
		return nil, errNoSource
	}
	return s.mobike.local, nil
}

// checkMobike mirrors the MOBIKE defect before d6ef7093a8: the route-source
// lookup failed with EPERM under the SPD catch-all, the error went to a debug
// line, and the function went on as if no address had moved, so
// UPDATE_SA_ADDRESSES was never sent.
func checkMobike(s *sa, log *slog.Logger) {
	local, err := routeSource(s)
	if err != nil { // want: error-dropped-below-warn
		log.Debug("ike: MOBIKE source unavailable", "peer", s.peer, "error", err)
	} else {
		s.mobike.local = local
	}
	if s.mobike.pending {
		s.mobike.pending = false
	}
}

// keepalive drops a send error at debug inside an if-init.
func keepalive(s *sa, log *slog.Logger, send func() error) {
	if err := send(); err != nil { // want: error-dropped-below-warn
		log.Debug("ike: keepalive failed", "peer", s.peer, "error", err)
	}
}

// loadAll skips an entry at info level and continues.
func loadAll(names []string, log *slog.Logger, load func(string) error) {
	for _, name := range names {
		if err := load(name); err != nil { // want: error-dropped-below-warn
			log.Info("skipping entry", "name", name, "error", err)
			continue
		}
	}
}

// apply answers the count read so far after a debug line: the caller reads
// a partial count as the whole.
func apply(log *slog.Logger, write func() (int, error)) int {
	count := 0
	written, err := write()
	if err != nil { // want: error-dropped-below-warn
		log.DebugContext(context.Background(), "write failed", "error", err)
		return count
	}
	return count + written
}

// probe answers false after an info line, in the inverted form.
func probe(log *slog.Logger, dial func() error) bool {
	err := dial()
	if err == nil {
		return true
	} else { // want: error-dropped-below-warn
		log.Log(context.Background(), slog.LevelDebug, "dial failed", "error", err)
	}
	return false
}

// propagates hands the error on: no finding.
func propagates(log *slog.Logger, write func() error) error {
	if err := write(); err != nil {
		log.Debug("write failed", "error", err)
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// replaces answers another error: the failure still reaches the caller.
func replaces(log *slog.Logger, write func() error) error {
	if err := write(); err != nil {
		log.Debug("write failed", "error", err)
		return errNoSource
	}
	return nil
}

// warns logs at WARN: the operator sees it, no finding.
func warns(log *slog.Logger, write func() error) {
	if err := write(); err != nil {
		log.Warn("write failed", "error", err)
	}
}

// records keeps the error on the SA: no finding.
func records(s *sa, log *slog.Logger, write func() error) {
	if err := write(); err != nil {
		log.Debug("write failed", "error", err)
		s.failed = err
	}
}

// counts steps a counter: a counted discard, no finding.
func counts(s *sa, log *slog.Logger, write func() error) {
	if err := write(); err != nil {
		log.Debug("write failed", "error", err)
		s.mobike.drops++
	}
}

// hands passes the error to another call: no finding.
func hands(log *slog.Logger, write func() error, fail func(error)) {
	if err := write(); err != nil {
		log.Debug("write failed", "error", err)
		fail(err)
	}
}

// silent has no log at all: another rule's business, no finding here.
func silent(write func() error) {
	if err := write(); err != nil {
		return
	}
}

// panics stops: no finding.
func panics(log *slog.Logger, write func() error) {
	if err := write(); err != nil {
		log.Debug("write failed", "error", err)
		panic("BUG: write cannot fail here")
	}
}

// notAnError tests a non-error value against nil: no finding.
func notAnError(s *sa, log *slog.Logger) {
	local := s.mobike.local
	if local != nil {
		log.Debug("have a source", "local", local)
	}
}
