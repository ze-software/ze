// Design: docs/architecture/plugin/feature-gates.md -- the reflection walk's indexed active path answers what a scan of the path answers.
// Related: externalreflectionpath.go -- the path index these tests compare with the linear scan.
package repoinventory

import (
	"go/ast"
	"strconv"
	"strings"
	"testing"
)

// TestExternalReflectionPathIndexMatchesTheScan proves the path index changes
// no answer and no amount of work. Method: walk every reuse fixture from every
// start depth, with reuse on and off, once through the index and once with the
// probe forcing the linear scan, and require the same method union with the
// same sources, the same origins, the same refusal at the same site, and the
// same number of expanded visits.
// MUTATION: (internal/le/repo/inventory/externalreflectionpath.go,
// "return position", "return -1"): a backedge the index finds reads as none,
// so the recursive cluster recurses to the depth refusal the scan never meets.
func TestExternalReflectionPathIndexMatchesTheScan(t *testing.T) {
	fixtures := []struct {
		name string
		body string
	}{
		{"diamond", reflectionReuseDiamond(8)},
		{"recursive cluster", reflectionReuseClusterBody},
		{"incoming depth", reflectionReuseDepthBody},
		{"generic arguments", reflectionReuseGenericBody},
		{"absent ancestor", reflectionReuseAbsentAncestorBody},
		{"propagated ancestor", reflectionReusePropagateBody},
		{"charged skip", reflectionReuseChargeBody},
		{"key cap", reflectionReuseCapBody},
		{"nested generic", reflectionPathNestedGenericBody},
	}
	for _, fixture := range fixtures {
		effects, source := reflectionReuseFixture(t, fixture.body)
		for depth := range 128 {
			for _, reuse := range []bool{true, false} {
				label := fixture.name + " at start depth " + strconv.Itoa(depth) + ", reuse " + strconv.FormatBool(reuse)
				indexed, indexedCount := reflectionReuseWalk(effects, source, depth, &externalReflectionReuseProbe{disabled: !reuse})
				linear, linearCount := reflectionReuseWalk(effects, source, depth, &externalReflectionReuseProbe{disabled: !reuse, linearPath: true})
				reflectionReuseSame(t, label, indexed, linear)
				if indexed.unknownAt != linear.unknownAt {
					t.Fatalf("%s: the index sited its refusal elsewhere than the scan", label)
				}
				if indexedCount != linearCount {
					t.Fatalf("%s: the index expanded %d visits, the scan %d", label, indexedCount, linearCount)
				}
			}
		}
	}
}

// reflectionPathNestedGenericBody puts Box[Near] and then Box[Far] on the path
// together: one definition, two arguments. Each Box's Self field is a
// backedge to its own instance only.
const reflectionPathNestedGenericBody = "type Root struct { A Box[Near] }\n" +
	"type Box[T any] struct { Value T; Self *Box[T] }\n" +
	"type Near struct { Inner Box[Far] }\n" +
	"type Far int\n" +
	"func (Near) TouchNear() {}\nfunc (Far) TouchFar() {}\n"

// TestExternalReflectionPathIndexKeepsArgumentsApart proves two active frames
// that share a definition but differ in arguments are never taken for each
// other. Method: walk the nested generic fixture, where Box[Far] is entered
// while Box[Near] is active and Box[Far]'s Self edge must find Box[Far], the
// second frame of the key; the walk must reach both methods with no refusal,
// as the scan does. Then drive the path directly: a frame equal by structure
// to an active one, through a different scope object, is that frame's
// backedge but not its exact match, and a popped frame is found no more.
// MUTATION: (internal/le/repo/inventory/externalreflectionpath.go,
// "externalReflectionFrameEqual(p.frames[position], frame, depth)",
// "position >= 0"): the index answers the key's first frame for any frame of
// the key, Box[Far] is pruned against Box[Near], and TouchFar is lost.
func TestExternalReflectionPathIndexKeepsArgumentsApart(t *testing.T) {
	effects, source := reflectionReuseFixture(t, reflectionPathNestedGenericBody)
	wanted := methodCensusExpectedNodes(t, effects, source.pkg, []string{"TouchNear", "TouchFar"})
	for _, reuse := range []bool{true, false} {
		indexed, _ := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{disabled: !reuse})
		linear, _ := reflectionReuseWalk(effects, source, 0, &externalReflectionReuseProbe{disabled: !reuse, linearPath: true})
		if indexed.unknown != "" {
			t.Fatalf("reuse %t: the nested generic walk refused: %s", reuse, indexed.unknown)
		}
		methodCensusCompareNodes(t, indexed.methods, wanted)
		reflectionReuseSame(t, "nested generic, reuse "+strconv.FormatBool(reuse), indexed, linear)
	}

	box := &ast.TypeSpec{Name: ast.NewIdent("Box")}
	other := &ast.TypeSpec{Name: ast.NewIdent("Other")}
	near, far := ast.NewIdent("Near"), ast.NewIdent("Far")
	first, second := &externalTypeScope{}, &externalTypeScope{}
	frame := func(value ast.Expr, scope *externalTypeScope) externalReflectionFrame {
		return externalReflectionFrame{definition: box, arguments: []externalTypeArgument{{source: sourceExpr{value: value}, scope: scope}}, methods: true}
	}
	var path externalReflectionActivePath
	path.push(frame(near, first))
	path.push(externalReflectionFrame{definition: other, methods: true})
	path.push(frame(far, first))
	if got := path.backedge(frame(far, first), 0); got != 2 {
		t.Fatalf("Box[Far] backedge at %d, want 2", got)
	}
	if got := path.backedge(frame(near, first), 0); got != 0 {
		t.Fatalf("Box[Near] backedge at %d, want 0", got)
	}
	if got := path.backedge(frame(near, second), 0); got != 0 {
		t.Fatalf("Box[Near] through an equal fresh scope: backedge at %d, want 0", got)
	}
	if got := externalReflectionActiveIndex(&path, frame(near, second)); got != -1 {
		t.Fatalf("Box[Near] through another scope object is exactly active at %d, want -1", got)
	}
	if got := externalReflectionActiveIndex(&path, frame(far, first)); got != 2 {
		t.Fatalf("Box[Far] exactly active at %d, want 2", got)
	}
	path.pop()
	if got := path.backedge(frame(far, first), 0); got != -1 {
		t.Fatalf("popped Box[Far] still found at %d", got)
	}
	if got := path.backedge(frame(near, first), 0); got != 0 {
		t.Fatalf("Box[Near] lost after a pop of its key: at %d, want 0", got)
	}
	path.pop()
	path.pop()
	if len(path.buckets) != 0 {
		t.Fatalf("an empty path still indexes %d keys", len(path.buckets))
	}
}

// BenchmarkExternalReflectionRecursiveCluster times one query over a chain of
// reflectionPathClusterTypes types, each holding two edges to the next and a
// backedge to every type above it, so every visit checks a deep path for
// backedges and every skipped sibling looks up every recorded ancestor. The
// index and linear sub-benchmarks differ only in the path lookup.
func BenchmarkExternalReflectionRecursiveCluster(b *testing.B) {
	effects, source := reflectionReuseFixture(b, reflectionPathClusterBody(reflectionPathClusterTypes))
	for _, linear := range []bool{false, true} {
		name := "index"
		if linear {
			name = "linear"
		}
		b.Run(name, func(b *testing.B) {
			probe := &externalReflectionReuseProbe{linearPath: linear}
			walk := externalReflectionWalk{effects: effects, probe: probe}
			if result := walk.value(source, nil, true, false, 0); result.unknown != "" {
				b.Fatalf("the cluster refused: %s", result.unknown)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				walk.value(source, nil, true, false, 0)
			}
		})
	}
}

// reflectionPathClusterTypes keeps the deepest edge of the cluster under the
// depth guard: each level costs four edges of depth.
const reflectionPathClusterTypes = 24

// reflectionPathClusterBody builds T0 to T<count-1>. Each T<i> holds A and B
// pointing at T<i+1> and Back<j> pointing at every T<j> with j <= i, and
// carries a method Touch<i>.
func reflectionPathClusterBody(count int) string {
	var body strings.Builder
	for index := range count {
		current := strconv.Itoa(index)
		body.WriteString("type T" + current + " struct {")
		if index+1 < count {
			next := strconv.Itoa(index + 1)
			body.WriteString(" A *T" + next + "; B *T" + next + ";")
		}
		for back := 0; back <= index; back++ {
			body.WriteString(" Back" + strconv.Itoa(back) + " *T" + strconv.Itoa(back) + ";")
		}
		body.WriteString(" }\nfunc (T" + current + ") Touch" + current + "() {}\n")
	}
	body.WriteString("type Root = T0\n")
	return body.String()
}
