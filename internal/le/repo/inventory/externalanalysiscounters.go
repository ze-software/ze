// Design: docs/architecture/plugin/feature-gates.md -- test-only counters of the reflection and type walks' work.
// Related: externalreflectiontypes.go -- collectNamed counts its visits and frame computations here.
// Related: externaleffecttypes.go -- the type walk counts its frames here.
package repoinventory

// externalAnalysisCounters is set only by tests, on the effects. It counts the
// work of the reflection and type walks, so a bounded run of the actual root
// shows whether the work follows the number of distinct frames. Production
// leaves it nil, so no walk counts anything and no answer depends on it:
// every hook only adds to a counter.
//
// report, when set, is called on the walking goroutine once every reportEvery
// ticks (a named reflection visit or a type frame expansion), so a harness
// reads the counters without racing the walk. Not safe for concurrent use.
type externalAnalysisCounters struct {
	// reflectionQueries counts externalReflectionWalk.value calls.
	reflectionQueries int
	// reflectedValues counts reflectedValue calls past the ambient check, and
	// memoHits the ones the analysis-wide memo answered.
	reflectedValues int
	memoHits        int
	// reflectedKeys counts reflectedValue calls per source and scope at any
	// depth: its length is the number of distinct queries ignoring depth.
	reflectedKeys map[externalReflectionMemoKey]int
	// namedVisits counts collectNamed visits that interned a frame, and
	// reflectionFrames the frame walks they started: once per exact frame in
	// an analysis whose walks never nest.
	namedVisits      int
	reflectionFrames int
	// Type walk: frames expanded, provisional answers (a frame that read an
	// outer estimate), settled answers reused, and rounds run.
	typeExpansions  int
	typeProvisional int
	typeSettledHits int
	typeRounds      int
	// objectEvaluations counts readObject calls that evaluated an object's
	// definitions, past the round's read cache.
	objectEvaluations int

	ticks       int
	reportEvery int
	report      func(*externalAnalysisCounters)
}

// tick counts one unit of walk work and calls report every reportEvery ticks.
func (c *externalAnalysisCounters) tick() {
	c.ticks++
	if c.report == nil {
		return
	}
	if c.reportEvery <= 0 {
		return
	}
	if c.ticks%c.reportEvery != 0 {
		return
	}
	c.report(c)
}

// reflected counts one reflectedValue call past the ambient check.
func (c *externalAnalysisCounters) reflected(key externalReflectionMemoKey) {
	c.reflectedValues++
	if c.reflectedKeys == nil {
		c.reflectedKeys = make(map[externalReflectionMemoKey]int)
	}
	c.reflectedKeys[key]++
}
