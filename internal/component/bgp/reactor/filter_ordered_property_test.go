// Property tests for the unified stage-ordered filter pass (spec
// followup-test-infra L96). Post-d7d925cc6 the reactor runs one stage-ordered
// ingress/egress pass; its payload-emitting core is buildModifiedPayload
// (forward_build.go) and its ordering primitive is filterapi.LessOrder.
//
// Engine: stdlib testing/quick, fixed seed (deterministic CI; R-1). Standing up
// the full external policy chain (RPC/text filters) in a unit property test is
// impractical, so the property targets the two units the pass is built on:
//  1. buildModifiedPayload: random UPDATE payloads (well-formed AND garbage) put
//     through random attribute-modification ops must never panic and must only
//     ever emit a structurally well-formed UPDATE body.
//  2. LessOrder: the stage/priority/name ordering the pass sorts steps by is a
//     strict total order (irreflexive, asymmetric, transitive) -- the "stage
//     order invariants hold" guarantee.
package reactor

import (
	"encoding/binary"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// opSpec is one generated attribute modification.
type opSpec struct {
	code   uint8
	action uint8
	val    []byte
}

// genBuildInput is a testing/quick generator: a payload (half well-formed, half
// garbage) plus a set of modification ops and an optional NLRI override.
type genBuildInput struct {
	payload     []byte
	ops         []opSpec
	nlriPresent bool
	nlriData    []byte
}

// Generate implements quick.Generator.
func (genBuildInput) Generate(r *rand.Rand, _ int) reflect.Value {
	in := genBuildInput{}
	if r.Intn(2) == 0 {
		in.payload = randWellFormedBody(r)
	} else {
		in.payload = randBytes(r, r.Intn(48)) // may be truncated/malformed
	}
	n := r.Intn(6)
	for range n {
		in.ops = append(in.ops, opSpec{
			code:   uint8(r.Intn(256)), //nolint:gosec // bounded
			action: uint8(r.Intn(5)),   //nolint:gosec // 5 AttrMod* actions
			val:    randBytes(r, r.Intn(8)),
		})
	}
	// Occasionally exercise the legacy-NLRI override path (nil / empty / bytes).
	switch r.Intn(3) {
	case 0:
		in.nlriPresent = false
	case 1:
		in.nlriPresent = true
		in.nlriData = nil // "drop all legacy NLRI"
	default:
		in.nlriPresent = true
		in.nlriData = randBytes(r, r.Intn(8))
	}
	return reflect.ValueOf(in)
}

func randBytes(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Intn(256)) //nolint:gosec // bounded
	}
	return b
}

// randWellFormedBody builds a structurally valid UPDATE body:
// withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri.
func randWellFormedBody(r *rand.Rand) []byte {
	withdrawn := randBytes(r, r.Intn(4))
	// A few well-formed non-extended attributes.
	var attrs []byte
	for range r.Intn(4) {
		val := randBytes(r, r.Intn(6))
		attrs = append(attrs, 0x40, uint8(r.Intn(256)), byte(len(val))) //nolint:gosec // bounded
		attrs = append(attrs, val...)
	}
	nlri := randBytes(r, r.Intn(4))

	body := make([]byte, 0, 4+len(withdrawn)+len(attrs)+len(nlri))
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(withdrawn))) //nolint:gosec // bounded
	body = append(body, hdr[:]...)
	body = append(body, withdrawn...)
	binary.BigEndian.PutUint16(hdr[:], uint16(len(attrs))) //nolint:gosec // bounded
	body = append(body, hdr[:]...)
	body = append(body, attrs...)
	body = append(body, nlri...)
	return body
}

// testAttrHandlers returns a handler for every attribute code that writes a
// single well-formed non-extended attribute (or no-ops when the buffer is
// full). Registering all codes forces the handler path for any generated op.
func testAttrHandlers() map[uint8]filterapi.AttrModHandler {
	h := make(map[uint8]filterapi.AttrModHandler, 256)
	for c := range 256 {
		code := uint8(c) //nolint:gosec // 0..255
		h[code] = func(_ []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
			var val []byte
			if len(ops) > 0 {
				val = ops[0].Buf
			}
			if len(val) > 255 {
				val = val[:255]
			}
			need := 3 + len(val)
			if off+need > len(buf) {
				return off // no room: leave offset unchanged (valid contract)
			}
			buf[off] = 0x80
			buf[off+1] = code
			buf[off+2] = byte(len(val))
			copy(buf[off+3:], val)
			return off + need
		}
	}
	return h
}

func (in genBuildInput) accumulator() *filterapi.ModAccumulator {
	var mods filterapi.ModAccumulator
	for _, op := range in.ops {
		mods.Op(op.code, op.action, op.val)
	}
	return &mods
}

// isWellFormedUpdateBody reports whether b parses as withdrawn_len + withdrawn +
// attr_len + attrs + nlri with all length fields in bounds.
func isWellFormedUpdateBody(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(b[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(b) < attrOffset+2 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(b[attrOffset : attrOffset+2]))
	attrEnd := attrOffset + 2 + attrLen
	return attrEnd <= len(b)
}

// TestFilterChainRandomUpdatesProperty is the L96 property.
//
// VALIDATES: AC-1 / L96 -- the stage-ordered pass's payload builder never
// panics on random (even malformed) UPDATE bytes + random ops, and only ever
// emits a structurally well-formed UPDATE body; the ordering primitive is a
// strict total order.
// PREVENTS: an attribute-modification change that emits a malformed UPDATE onto
// the wire, panics on adversarial input, or breaks stage-order determinism.
func TestFilterChainRandomUpdatesProperty(t *testing.T) {
	t.Parallel()

	handlers := testAttrHandlers()

	// Property 1 -- buildModifiedPayload robustness + well-formed output.
	t.Run("buildModifiedPayload_wellformed", func(t *testing.T) {
		t.Parallel()
		f := func(in genBuildInput) bool {
			var nlriOverride []byte
			if in.nlriPresent {
				nlriOverride = in.nlriData
				if nlriOverride == nil {
					nlriOverride = []byte{} // non-nil empty = "drop legacy NLRI"
				}
			}
			// Must not panic (a panic aborts quick.Check and fails the test).
			result, _ := buildModifiedPayload(in.payload, in.accumulator(), handlers, nil, nlriOverride)
			if result == nil {
				return true // no modification / rejected malformed input
			}
			return isWellFormedUpdateBody(result)
		}
		if err := quick.Check(f, propertyQuickConfig(97)); err != nil {
			t.Fatalf("buildModifiedPayload emitted malformed output or panicked: %v", err)
		}
	})

	// Property 2 -- LessOrder is a strict total order.
	t.Run("lessorder_strict_total_order", func(t *testing.T) {
		t.Parallel()
		f := func(a, b, c orderKey) bool {
			less := func(x, y orderKey) bool {
				return filterapi.LessOrder(x.name, x.stage, x.priority, y.name, y.stage, y.priority)
			}
			// Irreflexive.
			if less(a, a) {
				return false
			}
			// Asymmetric.
			if less(a, b) && less(b, a) {
				return false
			}
			// Transitive: a<b && b<c => a<c.
			if less(a, b) && less(b, c) && !less(a, c) {
				return false
			}
			return true
		}
		if err := quick.Check(f, propertyQuickConfig(98)); err != nil {
			t.Fatalf("LessOrder is not a strict total order: %v", err)
		}
	})
}

// orderKey is a generated filter ordering key with small ranges so ties on
// stage/priority/name are common.
type orderKey struct {
	name     string
	stage    int
	priority int
}

// Generate implements quick.Generator.
func (orderKey) Generate(r *rand.Rand, _ int) reflect.Value {
	names := []string{"a", "b", "c"}
	stages := []int{
		filterapi.FilterStageProtocol,
		filterapi.FilterStagePolicy,
		filterapi.FilterStageAnnotation,
		filterapi.FilterStagePeerChain,
	}
	return reflect.ValueOf(orderKey{
		name:     names[r.Intn(len(names))],
		stage:    stages[r.Intn(len(stages))],
		priority: r.Intn(3),
	})
}
