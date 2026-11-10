package fixture

import "log/slog"

// Marked carries marker comments that claim an exemption. No marker is a
// cover: both lines are refused like any line with no doctor code.
func Marked(log *slog.Logger, peer string) {
	// doctor-coverage: transient -- the peer recovers on its own.
	log.Warn("fixture: marked transient", "peer", peer)
	log.Error("fixture: marked exempt", "peer", peer) // doctor-coverage: exempt
}
