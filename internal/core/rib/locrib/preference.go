// Design: docs/architecture/rib/unified-locrib.md -- shared source preference.
// Related: entry.go -- stable first-seen selection after equal preference.
package locrib

import ribdistance "github.com/ze-software/ze/internal/core/rib/distance"

// Preference is the already-resolved source rank. Every uint8 distance and
// uint32 metric, including zero, is valid. Equal values preserve caller order.
type Preference struct {
	Distance uint8
	Metric   uint32
}

// Before reports a strict improvement. It never breaks equal rank, so the
// admitting table retains its own stable first-seen order.
func (p Preference) Before(other Preference) bool {
	if p.Distance < other.Distance {
		return true
	}
	if p.Distance > other.Distance {
		return false
	}
	return p.Metric < other.Metric
}

// PreferenceSource carries one admitted source's rank inputs without retaining
// a forwarding Path. Zero names an undeclared protocol and no override; it uses
// the named worst-distance guard. A present zero override remains meaningful.
type PreferenceSource struct {
	Protocol            string
	Metric              uint32
	DistanceOverride    uint8
	HasDistanceOverride bool
}

// PreferenceFor resolves an override before the supplied current declaration.
// The caller MUST supply its authoritative lookup. It applies no import/FIB gate.
func PreferenceFor(source PreferenceSource, lookup ribdistance.Func) Preference {
	if source.HasDistanceOverride {
		return Preference{Distance: source.DistanceOverride, Metric: source.Metric}
	}
	if value, found := lookup(source.Protocol); found {
		return Preference{Distance: value, Metric: source.Metric}
	}
	return Preference{Distance: UndeclaredDistance, Metric: source.Metric}
}
