// Design: docs/architecture/plugin/feature-gates.md -- settled recursive type frames are reused across one analysis.
// Related: externaleffecttypes.go -- the walk and its fixpoint rounds.
package repoinventory

// externalTypeSettled is the final value of one exact frame
// (externalTypeFrame.id), computed by a frame whose rounds read no estimate of
// a frame outside it. The walk treats the type graph by reachability, so no
// guard inside it depends on how deep the frame was met, and a value carrying
// an unknown holds from every visit as one without does. references records
// whether the computation met a reference type. ambientRead records whether it
// read the ambient pool: one that did holds only for the pool it read
// (ambient), one that did not holds for every pool, because the pool is the
// only input of a type walk that grows.
type externalTypeSettled struct {
	ambient     int
	ambientRead bool
	value       externalValue
	references  bool
}

// externalTypeProbe is set only by tests: it turns the settled table or the
// reflectedValue memo off, as before each existed, and counts frame
// expansions. Production leaves it nil.
type externalTypeProbe struct {
	unsettled  bool
	unmemoized bool
	expansions int
}

// estimate answers frame's estimate slot, creating it empty. Estimates live
// for the whole walk: every estimate only grows from below the least
// fixpoint, so a later component starting from it still settles on the same
// fixpoint.
func (w *externalTypeWalk) estimate(frame *externalTypeFrame) *externalValue {
	if estimate, found := w.estimates[frame.id]; found {
		return estimate
	}
	if w.estimates == nil {
		w.estimates = make(map[int32]*externalValue)
	}
	estimate := &externalValue{}
	w.estimates[frame.id] = estimate
	return estimate
}

// settledValue answers frame's settled value when one was computed over the
// same ambient pool, or over any pool when that computation never read it.
// The frame id fixes the definition, the arguments, the method flag and the
// reflection setting, and the pool is everything else a walk reads that
// changes: type declarations, scopes and method censuses are fixed for the
// analysis, so the table belongs to the whole analysis and every typeMethods
// call shares it. The caller joins the answer, which copies it, so the slot
// stays unshared.
func (w *externalTypeWalk) settledValue(frame *externalTypeFrame) (externalValue, bool) {
	if probe := w.effects.typeProbe; probe != nil {
		if probe.unsettled {
			return externalValue{}, false
		}
	}
	slot, found := w.effects.settledTypes[frame.id]
	if !found {
		return externalValue{}, false
	}
	if slot.ambientRead {
		if slot.ambient != w.effects.ambient {
			return externalValue{}, false
		}
	}
	if slot.references {
		w.references = true
	}
	w.ambientRead = w.ambientRead || slot.ambientRead
	return slot.value, true
}

// settle keeps a final frame value. A slot from a smaller pool that read it
// is dead, because the pool only grows, so it is replaced rather than kept
// beside the new one.
func (w *externalTypeWalk) settle(frame *externalTypeFrame, value externalValue) {
	if probe := w.effects.typeProbe; probe != nil {
		if probe.unsettled {
			return
		}
	}
	var kept externalValue
	kept.join(value)
	if w.effects.settledTypes == nil {
		w.effects.settledTypes = make(map[int32]externalTypeSettled)
	}
	w.effects.settledTypes[frame.id] = externalTypeSettled{ambient: w.effects.ambient, ambientRead: w.ambientRead, value: kept, references: w.references}
}
