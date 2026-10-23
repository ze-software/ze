package types

import (
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// unregisteredContextID returns a non-zero context id the global registry does
// not hold, searching down from the top of the id space.
func unregisteredContextID(t *testing.T) bgpctx.ContextID {
	t.Helper()
	for id := ^bgpctx.ContextID(0); id > 0; id-- {
		if bgpctx.Registry.Get(id) == nil {
			return id
		}
	}
	t.Fatal("every context id is registered")
	return 0
}

// TestSourceEncodingContextNamesTheUnknownID asks SourceEncodingContext for
// each kind of message it can be handed.
//
// VALIDATES: a non-UPDATE and an UPDATE with context id zero answer nil and
// known (no ADD-PATH negotiated); a registered id answers its context; a
// non-zero id the registry does not hold answers unknown.
// PREVENTS: an unknown id answering a bare nil that every caller reads as "no
// ADD-PATH", so an ADD-PATH section is walked from the wrong octet.
func TestSourceEncodingContextNamesTheUnknownID(t *testing.T) {
	plain := RawMessage{}
	if ctx, known := plain.SourceEncodingContext(); ctx != nil || !known {
		t.Fatalf("non-UPDATE: got %v, %v; want nil, true", ctx, known)
	}

	unset := RawMessage{WireUpdate: wireu.NewWireUpdate([]byte{0, 0, 0, 0}, 0)}
	if ctx, known := unset.SourceEncodingContext(); ctx != nil || !known {
		t.Fatalf("context id zero: got %v, %v; want nil, true", ctx, known)
	}

	want := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	id, err := bgpctx.Registry.Register(want)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	registered := RawMessage{WireUpdate: wireu.NewWireUpdate([]byte{0, 0, 0, 0}, id)}
	ctx, known := registered.SourceEncodingContext()
	if !known || ctx == nil || !ctx.AddPath(family.IPv4Unicast) {
		t.Fatalf("registered id %d: got %v, %v; want its ADD-PATH context", id, ctx, known)
	}

	unknown := RawMessage{WireUpdate: wireu.NewWireUpdate([]byte{0, 0, 0, 0}, unregisteredContextID(t))}
	if ctx, known := unknown.SourceEncodingContext(); ctx != nil || known {
		t.Fatalf("unregistered id: got %v, %v; want nil, false", ctx, known)
	}
}
