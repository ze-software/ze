// Design: docs/architecture/plugin/feature-gates.md -- one reflection query skips a named visit only on proof of coverage.
// Related: externalreflectionreuse.go -- the per-query table these tests probe.
package repoinventory

import (
	"strconv"
	"strings"
	"testing"
)

// TestExternalReflectionReuseDiamond proves reuse shrinks a shared subgraph's
// work without changing the answer. Method: walk a diamond whose single Touch
// method is reachable through 2^8 paths, with reuse and with reuse disabled,
// and require the same union and strictly fewer expanded visits with reuse.
// MUTATION: make covered return false on its first line, and the counts tie.
func TestExternalReflectionReuseDiamond(t *testing.T) {
	effects, source := reflectionReuseFixture(t, reflectionReuseDiamond(8))
	on, onCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{})
	off, offCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{disabled: true})
	reflectionReuseSame(t, "diamond", on, off)
	if off.unknown != "" {
		t.Fatalf("the diamond refused: %s", off.unknown)
	}
	methodCensusCompareNodes(t, off.methods, methodCensusExpectedNodes(t, effects, source.pkg, []string{"Touch"}))
	if onCount >= offCount {
		t.Fatalf("reuse expanded %d visits, the full walk %d: nothing was skipped", onCount, offCount)
	}
}

// TestExternalReflectionReuseRecursiveCluster proves the pruned-ancestor rule,
// not only the clean-subtree rule. Method: Shared is reached through Left and
// then Right, and points back at Top, so its first visit prunes against Top.
// Top is on the path at the second visit, which reuse must skip.
// MUTATION: in store, return when pruned is non-zero, and the counts tie.
func TestExternalReflectionReuseRecursiveCluster(t *testing.T) {
	body := "type Top struct { Left *Left; Right *Right }\n" +
		"type Left struct { Next *Shared }\n" +
		"type Right struct { Next *Shared }\n" +
		"type Shared struct { Back *Top }\n" +
		"func (Left) TouchLeft() {}\nfunc (Right) TouchRight() {}\nfunc (Shared) TouchShared() {}\n" +
		"type Root = Top\n"
	effects, source := reflectionReuseFixture(t, body)
	on, onCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{})
	off, offCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{disabled: true})
	reflectionReuseSame(t, "recursive cluster", on, off)
	if off.unknown != "" {
		t.Fatalf("the cluster refused: %s", off.unknown)
	}
	methodCensusCompareNodes(t, off.methods, methodCensusExpectedNodes(t, effects, source.pkg, []string{"TouchLeft", "TouchRight", "TouchShared"}))
	if onCount >= offCount {
		t.Fatalf("reuse expanded %d visits, the full walk %d: the second Shared visit was not skipped", onCount, offCount)
	}
}

// TestExternalReflectionReuseKeepsIncomingDepth proves a deeper visit of a
// type is never answered by a shallower one. Method: Shared is reached through
// A, then three edges deeper through Wrap. For every starting depth, the walk
// with reuse must answer what the full walk answers, and the range must cross
// the depth limit, so some start refuses only through the deeper route.
// MUTATION: build the key with depth 0 in externalReflectionVisitKeyOf, and the
// boundary start loses its depth refusal.
func TestExternalReflectionReuseKeepsIncomingDepth(t *testing.T) {
	body := "type Root struct { A Shared; B Wrap }\n" +
		"type Wrap struct { X Shared }\n" +
		"type Shared struct { Next Chain1 }\n" +
		reflectionReuseChain(6, "Touch")
	reflectionReuseAcrossDepths(t, body)
}

// TestExternalReflectionReuseKeepsGenericArguments proves Box[Near] does not
// answer for Box[Far]. Method: both have the same definition, owner and depth.
// Far's chain is reached through the body's T a few edges deeper than through
// the argument itself, so near the limit only the body route refuses.
// MUTATION: make externalReflectionArgumentsSame return true, and the boundary
// start loses its depth refusal.
func TestExternalReflectionReuseKeepsGenericArguments(t *testing.T) {
	body := "type Root struct { A Box[Near]; B Box[Far] }\n" +
		"type Box[T any] struct { Value T }\n" +
		"type Near int\nfunc (Near) TouchNear() {}\n" +
		"type Far struct { Next Chain1 }\n" +
		reflectionReuseChain(4, "TouchFar")
	reflectionReuseAcrossDepths(t, body)
}

// TestExternalReflectionReuseNeedsPrunedAncestorsOnPath proves a visit that
// pruned against an ancestor proves nothing where that ancestor is absent.
// Method: Shared is reached under Holder, where its Back edge to Holder is
// pruned, then at the same depth under Other, where Holder is not active. The
// second visit must traverse into Holder, whose chain is then deep enough to
// refuse near the limit.
// MUTATION: make externalReflectionActiveIndex return 0, so every recorded
// ancestor reads as active, and the boundary start loses its depth refusal.
func TestExternalReflectionReuseNeedsPrunedAncestorsOnPath(t *testing.T) {
	body := "type Root struct { A *Holder; B *Other }\n" +
		"type Holder struct { S *Shared; C Chain1 }\n" +
		"type Other struct { S *Shared; C Plain }\n" +
		"type Shared struct { Back *Holder }\n" +
		"type Plain int\n" +
		reflectionReuseChain(4, "Touch")
	reflectionReuseAcrossDepths(t, body)
}

// TestExternalReflectionReusePropagatesPrunedAncestors proves a visit's record
// holds what its children pruned against, not only its own backedges. Method:
// Mid reaches Holder only through Shared, so Mid's record names Holder only
// when Shared's completion hands it up. Mid is then reached at the same depth
// under Other, where Holder is not active, and must traverse into Holder,
// whose chain is deep enough to refuse near the limit.
// MUTATION: in complete, delete the externalReflectionEachIndex call that
// hands the pruned set to the parent, and the boundary start loses its depth
// refusal.
func TestExternalReflectionReusePropagatesPrunedAncestors(t *testing.T) {
	body := "type Root struct { A *Holder; B *Other }\n" +
		"type Holder struct { M *Mid; C Chain1 }\n" +
		"type Other struct { M *Mid; C Plain }\n" +
		"type Mid struct { S *Shared }\n" +
		"type Shared struct { Back *Holder }\n" +
		"type Plain int\n" +
		reflectionReuseChain(4, "Touch")
	reflectionReuseAcrossDepths(t, body)
}

// TestExternalReflectionReuseChargesSkippedVisits proves a skipped visit's
// pruned set is charged to the visit that encloses it. Method: under Holder,
// Shared is first reached through Pad, then at the same depth through Mid,
// where it is skipped. Mid's record names Holder only through that charge.
// Mid is then reached at the same depth under Other, where Holder is not
// active, and must traverse into Holder, whose chain refuses near the limit.
// MUTATION: in covered, delete the r.pruned call in the ancestors loop, and the
// boundary start loses its depth refusal.
func TestExternalReflectionReuseChargesSkippedVisits(t *testing.T) {
	body := "type Root struct { A *Holder; B *Other }\n" +
		"type Holder struct { X *Pad; M *Mid; C Chain1 }\n" +
		"type Other struct { M *Mid; C Plain }\n" +
		"type Pad struct { S *Shared }\n" +
		"type Mid struct { S *Shared }\n" +
		"type Shared struct { Back *Holder }\n" +
		"type Plain int\n" +
		reflectionReuseChain(4, "Touch")
	reflectionReuseAcrossDepths(t, body)
}

// TestExternalReflectionReuseFullTableTraverses proves a full table falls back
// to the original traversal with the same answer. Method: a table that keeps
// one visit must answer what the full walk answers, and expand more visits
// than the default table does.
// MUTATION: delete the visitsMax check in store, and the tiny table expands as
// few visits as the default one.
func TestExternalReflectionReuseFullTableTraverses(t *testing.T) {
	effects, source := reflectionReuseFixture(t, reflectionReuseDiamond(8))
	normal, normalCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{})
	tiny, tinyCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{visits: 1})
	off, offCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{disabled: true})
	reflectionReuseSame(t, "default table", normal, off)
	reflectionReuseSame(t, "one-visit table", tiny, off)
	if tinyCount <= normalCount {
		t.Fatalf("a one-visit table expanded %d visits, the default table %d: the bound was not applied", tinyCount, normalCount)
	}
	if tinyCount > offCount {
		t.Fatalf("a one-visit table expanded %d visits, more than the full walk's %d", tinyCount, offCount)
	}
}

// TestExternalReflectionReuseFullKeyTraverses proves a key that holds its cap
// of visits falls back to traversal with the same answer. Method: Shared is
// reached at one depth under Left, with record Left, then under Right, with
// record Right, and again through Right's second field. With one visit kept
// per key the second record is dropped, so the third visit must expand again.
// MUTATION: delete the visitsPerKeyMax check in store, and the counts tie.
func TestExternalReflectionReuseFullKeyTraverses(t *testing.T) {
	reflectionReuseCapTraverses(t, "one-visit key", &externalReflectionReuseProbe{visitsPerKey: 1})
}

// TestExternalReflectionReuseFullFramesTraverses proves a table whose ancestor
// frame budget is spent falls back to traversal with the same answer. Method:
// the fixture of TestExternalReflectionReuseFullKeyTraverses, with a budget of
// one frame, so records that name an ancestor are dropped once it is spent.
// MUTATION: delete the framesMax check in store, and the counts tie.
func TestExternalReflectionReuseFullFramesTraverses(t *testing.T) {
	reflectionReuseCapTraverses(t, "one-frame table", &externalReflectionReuseProbe{frames: 1})
}

// reflectionReuseCapTraverses walks a graph whose Shared visits need two
// records under one key, with the default table, with the capped probe and
// with reuse disabled. It requires one answer, and a capped count above the
// default count and no higher than the full walk's.
func reflectionReuseCapTraverses(t *testing.T, label string, capped *externalReflectionReuseProbe) {
	t.Helper()
	body := "type Root struct { A *Left; B *Right }\n" +
		"type Left struct { S *Shared }\n" +
		"type Right struct { S *Shared; T *Shared }\n" +
		"type Shared struct { One *Left; Two *Right }\n" +
		"func (Left) TouchLeft() {}\nfunc (Right) TouchRight() {}\nfunc (Shared) TouchShared() {}\n"
	effects, source := reflectionReuseFixture(t, body)
	normal, normalCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{})
	small, smallCount := reflectionReuseWalk(effects, source, 0, capped)
	off, offCount := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{disabled: true})
	if off.unknown != "" {
		t.Fatalf("the graph refused: %s", off.unknown)
	}
	methodCensusCompareNodes(t, off.methods, methodCensusExpectedNodes(t, effects, source.pkg, []string{"TouchLeft", "TouchRight", "TouchShared"}))
	reflectionReuseSame(t, "default table", normal, off)
	reflectionReuseSame(t, label, small, off)
	if smallCount <= normalCount {
		t.Fatalf("a %s expanded %d visits, the default table %d: the bound was not applied", label, smallCount, normalCount)
	}
	if smallCount > offCount {
		t.Fatalf("a %s expanded %d visits, more than the full walk's %d", label, smallCount, offCount)
	}
}

// reflectionReuseAcrossDepths compares the walk with and without reuse at
// every starting depth, and requires the range to hold both a clean start and
// a start refused at the depth limit.
func reflectionReuseAcrossDepths(t *testing.T, body string) {
	t.Helper()
	effects, source := reflectionReuseFixture(t, body)
	clean, refused := false, false
	for depth := range 128 {
		on, _ := reflectionReuseWalk(effects, source, depth, &externalReflectionReuseProbe{})
		off, _ := reflectionReuseWalk(effects, source, depth, &externalReflectionReuseProbe{disabled: true})
		reflectionReuseSame(t, "start depth "+strconv.Itoa(depth), on, off)
		if off.unknown == "" {
			clean = true
		}
		if strings.Contains(off.unknown, "depth exceeds 128") {
			refused = true
		}
	}
	if !clean {
		t.Fatal("no starting depth walked the graph without a refusal")
	}
	if !refused {
		t.Fatal("no starting depth reached the depth limit")
	}
}

func reflectionReuseFixture(t *testing.T, body string) (*externalEffects, sourceExpr) {
	t.Helper()
	effects, definition, owner := methodCensusFixture(t, body)
	return effects, sourceExpr{value: definition.Name, file: owner.file, pkg: owner.pkg}
}

// reflectionReuseWalk runs one query on a fresh walker carrying probe, and
// answers the union and the visits it expanded.
func reflectionReuseWalk(effects *externalEffects, source sourceExpr, depth int, probe *externalReflectionReuseProbe) (externalReflection, int) {
	walk := externalReflectionWalk{effects: effects, probe: probe}
	result := walk.value(source, nil, true, false, depth)
	return result, probe.traversals
}

// reflectionReuseSame requires the same methods with the same sources, the
// same external origins and the same refusal.
func reflectionReuseSame(t *testing.T, label string, got, wanted externalReflection) {
	t.Helper()
	if got.unknown != wanted.unknown {
		t.Fatalf("%s: reuse refused %q, the full walk %q", label, got.unknown, wanted.unknown)
	}
	methodCensusCompareNodes(t, got.methods, wanted.methods)
	if len(got.origins) != len(wanted.origins) {
		t.Fatalf("%s: reuse found origins %v, the full walk %v", label, got.origins, wanted.origins)
	}
	for origin := range wanted.origins {
		if !got.origins[origin] {
			t.Fatalf("%s: reuse lost origin %s", label, origin)
		}
	}
}

// reflectionReuseDiamond builds depth levels of two fields to the level below,
// so the single Touch method is reachable through 2^depth paths.
func reflectionReuseDiamond(depth int) string {
	var body strings.Builder
	body.WriteString("type Leaf int\nfunc (Leaf) Touch() {}\ntype Node0 = Leaf\n")
	for index := 1; index <= depth; index++ {
		current, previous := strconv.Itoa(index), strconv.Itoa(index-1)
		body.WriteString("type Node" + current + " struct { First Node" + previous + "; Second Node" + previous + " }\n")
	}
	body.WriteString("type Root = Node" + strconv.Itoa(depth) + "\n")
	return body.String()
}

// reflectionReuseChain builds Chain1 to Chain<length>, each a struct holding
// the next, ending in an int that carries method.
func reflectionReuseChain(length int, method string) string {
	const prefix = "Chain"
	var body strings.Builder
	for index := 1; index < length; index++ {
		body.WriteString("type " + prefix + strconv.Itoa(index) + " struct { Next " + prefix + strconv.Itoa(index+1) + " }\n")
	}
	last := prefix + strconv.Itoa(length)
	body.WriteString("type " + last + " int\nfunc (" + last + ") " + method + "() {}\n")
	return body.String()
}
